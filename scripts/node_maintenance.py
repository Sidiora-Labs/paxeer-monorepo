#!/usr/bin/env python3
"""Offline snapshots and journaled state-sync recovery for a locally signed paxd.

Pass a packaged native config decoder as --config-loader. It receives HOME and
returns the validated config contract as JSON. A controller receives ACTION HOME and
must stop all writers and suppress automatic restarts until explicitly started.
The default controller manages PAXD_SERVICE through systemd. Interrupted recovery
requires --rollback before any further maintenance. Backups are private and are
retained even after startup verification; remove them only after operator review.
"""

import argparse
import copy
import datetime
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import stat
import subprocess
import sys
import tarfile
import tempfile
import time
import tomllib
import urllib.parse
import urllib.request


class MaintenanceError(RuntimeError):
    pass


class CommandController:
    def __init__(self, executable):
        self.executable = executable

    def __call__(self, action, home):
        subprocess.run([self.executable, action, str(home)], check=True, timeout=60)


class NativeConfigLoader:
    def __init__(self, executable):
        self.executable = executable

    def __call__(self, home):
        result = subprocess.run(
            [self.executable, str(home)],
            check=True, capture_output=True, text=True, timeout=60,
        )
        return result.stdout


def require(condition, message):
    if not condition:
        raise MaintenanceError(message)


def sync_dir(path):
    fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def durable_tree(root):
    for path in root.rglob("*"):
        if path.is_file():
            with path.open("rb") as stream:
                os.fsync(stream.fileno())
    for path in sorted(root.rglob("*"), reverse=True):
        if path.is_dir():
            sync_dir(path)
    sync_dir(root)


def atomic_write(path, data):
    fd, temporary = tempfile.mkstemp(dir=path.parent, prefix=".maintenance-")
    try:
        with os.fdopen(fd, "wb") as stream:
            stream.write(data)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
        sync_dir(path.parent)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def regular_tree(root):
    require(root.is_dir() and not root.is_symlink(), f"not a real directory: {root}")
    for path in root.rglob("*"):
        mode = path.lstat().st_mode
        require(stat.S_ISDIR(mode) or stat.S_ISREG(mode), f"unsafe home entry: {path}")


def run_controller(args, action):
    args.controller(action, args.home)


def stop(args):
    run_controller(args, "stop")
    run_controller(args, "check-stopped")


def load_config(args, home=None):
    root = home or args.home
    cfg = json.loads(args.config_loader(root))
    require(isinstance(cfg, dict), "native decoder must return a JSON object")
    require(type(cfg["enable"]) is bool and type(cfg["use_p2p"]) is bool, "invalid native state-sync flags")
    require(isinstance(cfg["rpc_servers"], list) and all(isinstance(url, str) for url in cfg["rpc_servers"]), "invalid native RPC servers")
    require(type(cfg["trust_height"]) is int and cfg["trust_height"] >= 0, "invalid native trust height")
    require(isinstance(cfg["trust_hash"], str) and isinstance(cfg["persistent_peers"], str), "invalid native config strings")
    require(type(cfg["trust_period"]) in (int, float) and cfg["trust_period"] > 0, "invalid native trust period")
    require(isinstance(cfg["chain_id"], str) and cfg["chain_id"], "missing native chain identity")
    require(isinstance(cfg["node_id"], str) and re.fullmatch(r"[0-9a-f]{40}", cfg["node_id"]), "invalid native node identity")
    require(not cfg["remote_signer"], "external signers require their own stopped-node procedure")
    for key in ("genesis", "node_key", "validator_key", "sign_state", "db_dir"):
        require(Path(cfg[key]).is_absolute(), f"native {key} path must be absolute")
        path = Path(cfg[key]).resolve()
        require(path.is_relative_to(root), f"{key} must be inside the home")
        cfg[key] = str(path)
    require(Path(cfg["db_dir"]) == root / "data", "only the standard data directory is supported")
    require(Path(cfg["sign_state"]).is_relative_to(root / "data"), "signing state must be inside data")
    for key in ("node_key", "validator_key", "genesis"):
        require(Path(cfg[key]).is_relative_to(root / "config"), f"{key} must be inside config")
    for name in ("config/config.toml", "config/app.toml", "data"):
        require((root / name).exists(), f"incomplete home: missing {name}")
    for key in ("validator_key", "node_key", "genesis"):
        require(Path(cfg[key]).is_file(), f"missing {key}")
    signing_state(Path(cfg["sign_state"]))
    return cfg


def signing_state(path):
    value = json.loads(path.read_text())
    require(isinstance(value["height"], str) and value["height"].isdigit(), "invalid signing height")
    require(type(value["round"]) is int and value["round"] >= 0, "invalid signing round")
    require(type(value["step"]) is int and 0 <= value["step"] <= 3, "invalid signing step")
    if int(value["height"]) > 0:
        require(bool(value.get("signature")) and bool(value.get("signbytes")), "incomplete signing state")
    return (int(value["height"]), value["round"], value["step"])


def rpc(url, method, **params):
    query = urllib.parse.urlencode(params)
    request = urllib.request.Request(f"{url.rstrip('/')}/{method}?{query}", headers={"Accept": "application/json"})
    with urllib.request.urlopen(request, timeout=5) as response:
        require(response.status == 200, "RPC did not return HTTP 200")
        require(response.headers.get_content_type() == "application/json", "RPC did not return application/json")
        payload = json.load(response)
    # Native URI handlers return Result directly (writeHTTPResponse), whereas
    # POST JSON-RPC handlers return an envelope. This client only sends URI GETs.
    require(isinstance(payload, dict) and not {"error", "code", "jsonrpc", "result"}.intersection(payload), "invalid native RPC response")
    require(method in ("block", "status"), "unsupported native RPC method")
    require(("block" if method == "block" else "sync_info") in payload, "incomplete native RPC response")
    return payload


def rpc_url(value):
    parsed = urllib.parse.urlsplit(value)
    require(parsed.scheme in ("http", "https") and parsed.hostname and parsed.port, "RPC requires http(s)://host:port")
    require(not parsed.username and not parsed.password and not parsed.query and not parsed.fragment, "invalid RPC URL")
    require(parsed.path in ("", "/"), "RPC must use its root URL")
    return value.rstrip("/")


def validate_trust(args, cfg):
    require(args.trust_height > 0, "trust height must be positive")
    require(re.fullmatch(r"[0-9A-Fa-f]{64}", args.trust_hash) is not None, "trust hash must be 32 bytes of hex")
    args.rpc_server = [rpc_url(url) for url in args.rpc_server]
    require(len(args.rpc_server) >= 2 and len(set(args.rpc_server)) == len(args.rpc_server), "provide at least two distinct RPC servers")
    for url in args.rpc_server:
        block = rpc(url, "block", height=args.trust_height)
        header = block["block"]["header"]
        require(header["chain_id"] == cfg["chain_id"], "trust block belongs to another chain")
        require(int(header["height"]) == args.trust_height, "RPC returned the wrong trust height")
        require(block["block_id"]["hash"].upper() == args.trust_hash.upper(), "trust hash disagrees with RPC")
        stamp = datetime.datetime.fromisoformat(header["time"].replace("Z", "+00:00"))
        age = (datetime.datetime.now(datetime.timezone.utc) - stamp).total_seconds()
        require(0 <= age < cfg["trust_period"], "trust block is expired or in the future")
    require(args.peer, "at least one persistent peer is required")
    for peer in args.peer.split(","):
        require(re.fullmatch(r"[0-9a-fA-F]{40}@[A-Za-z0-9.\-]+:[0-9]+", peer) is not None, "invalid persistent peer")
        require(0 < int(peer.rsplit(":", 1)[1]) <= 65535, "invalid peer port")


def update_config(path, sections):
    # Retain every unrelated setting and comment. Reject obsolete spellings rather
    # than relying on Viper to ignore them and silently take defaults.
    original = path.read_text()
    parsed = tomllib.loads(original)
    expected = copy.deepcopy(parsed)
    for section, values in sections.items():
        require(section in parsed, f"missing [{section}] section")
        expected[section].update(values)
        for key in values:
            require(key.replace("-", "_") not in parsed[section] or "-" not in key, f"obsolete key in [{section}]: {key}")
    current = None
    seen = set()
    output = []

    def missing():
        if current in sections:
            for key, value in sections[current].items():
                if (current, key) not in seen:
                    output.append(f"{key} = {json.dumps(value)}\n")

    for line in original.splitlines(keepends=True):
        match = re.match(r"^\s*\[([^\]]+)\]\s*(?:#.*)?$", line.strip())
        if match:
            missing()
            current = match[1]
        assignment = re.match(r"^\s*([\w-]+)\s*=", line)
        if current in sections and assignment and assignment[1] in sections[current]:
            key = assignment[1]
            output.append(f"{key} = {json.dumps(sections[current][key])}\n")
            seen.add((current, key))
        else:
            output.append(line if line.endswith("\n") else line + "\n")
    missing()
    encoded = "".join(output).encode()
    require(tomllib.loads(encoded.decode()) == expected, "config edit changed unrelated settings")
    atomic_write(path, encoded)


def startup(args, cfg, minimum):
    run_controller(args, "start")
    deadline = time.monotonic() + args.startup_timeout
    last = "no status response"
    while time.monotonic() < deadline:
        try:
            run_controller(args, "check-running")
            status = rpc(args.local_rpc, "status")
            require(status["node_info"]["network"] == cfg["chain_id"], "startup chain identity mismatch")
            require(status["node_info"]["id"] == cfg["node_id"], "startup node identity mismatch")
            info = status["sync_info"]
            require(info["catching_up"] is False, "node is still catching up")
            require(int(info["latest_block_height"]) >= minimum, "node has not recovered required height")
            live_state = Path(cfg["sign_state"])
            live_hrs = signing_state(live_state)
            require(live_hrs >= args.signed_floor, "signing state regressed")
            if live_hrs == args.signed_floor:
                require(json.loads(live_state.read_bytes()) == args.signed_state, "signing state changed at same height/round/step")
            return
        except (OSError, ValueError, KeyError, MaintenanceError, subprocess.SubprocessError) as error:
            last = str(error)
            time.sleep(0.2)
    raise MaintenanceError(f"startup verification failed: {last}")


def journal_write(work, record):
    atomic_write(work / "journal.json", json.dumps(record, sort_keys=True).encode())


def rollback(args, work):
    require(not work.is_symlink() and work.is_dir(), "invalid private recovery directory")
    record = json.loads((work / "journal.json").read_text())
    require(record["home"] == str(args.home) and record["phase"] in ("prepared", "replaced", "verified", "restoring", "rolled-back"), "invalid recovery journal")
    relative_state = Path(record["sign_state"])
    require(not relative_state.is_absolute() and ".." not in relative_state.parts and relative_state.is_relative_to("data"), "invalid journal signing-state path")
    stop(args)
    backup = work / "backup"
    regular_tree(backup)
    require(tree_manifest(backup) == record["backup_manifest"], "backup failed integrity verification")
    state = backup / relative_state
    chosen = state.read_bytes()
    failed = work / "failed-home"
    for candidate_home in (failed, args.home):
        if candidate_home.exists():
            regular_tree(candidate_home)
        live_state = candidate_home / relative_state
        if not live_state.exists():
            continue
        live_hrs, saved_hrs = signing_state(live_state), signing_state(state)
        require(live_hrs >= saved_hrs, "live signing state regressed; manual safety review required")
        chosen_state = json.loads(chosen)
        chosen_hrs = (int(chosen_state["height"]), chosen_state["round"], chosen_state["step"])
        if live_hrs == chosen_hrs:
            require(json.loads(live_state.read_bytes()) == chosen_state, "conflicting signing state at same height/round/step")
        if live_hrs >= chosen_hrs:
            chosen = live_state.read_bytes()
    if record["phase"] == "rolled-back":
        require(args.home.is_dir(), "rolled-back home is missing")
        require(json.loads((args.home / relative_state).read_bytes()) == json.loads(chosen), "rolled-back signing state regressed; manual safety review required")
        print(f"Rollback already restored a stopped home. Private backup retained: {work}", flush=True)
        return
    stage = work / "restore"
    if stage.exists():
        shutil.rmtree(stage)
    shutil.copytree(backup, stage)
    atomic_write(stage / relative_state, chosen)
    durable_tree(stage)
    expected = tree_manifest(stage)
    if failed.exists() and args.home.exists():
        require(tree_manifest(args.home) == expected, "interrupted rollback home failed verification")
        shutil.rmtree(stage)
        record["phase"] = "rolled-back"
        journal_write(work, record)
        return
    record["phase"] = "restoring"
    journal_write(work, record)
    if args.home.exists():
        os.rename(args.home, failed)
        sync_dir(args.home.parent)
        sync_dir(work)
    os.rename(stage, args.home)
    sync_dir(args.home.parent)
    sync_dir(work)
    record["phase"] = "rolled-back"
    journal_write(work, record)
    print(f"Rollback restored a stopped home. Private backup retained: {work}", flush=True)


def tree_manifest(root, excluded=()):
    return {str(path.relative_to(root)): digest(path) for path in sorted(root.rglob("*")) if path.is_file() and path.relative_to(root) not in excluded}


def state_sync(args, work):
    if args.rollback:
        rollback(args, work)
        return
    require(not work.exists(), f"unfinished or retained recovery at {work}; use --rollback or archive after review")
    regular_tree(args.home)
    stop(args)
    cfg = load_config(args)
    args.signed_floor = signing_state(Path(cfg["sign_state"]))
    args.signed_state = json.loads(Path(cfg["sign_state"]).read_bytes())
    validate_trust(args, cfg)
    # The complete stopped home is copied and fsynced before any live state edit.
    work.mkdir(mode=0o700)
    backup = work / "backup"
    shutil.copytree(args.home, backup)
    durable_tree(backup)
    require(tree_manifest(backup) == tree_manifest(args.home), "private backup verification failed")
    relative_state = str(Path(cfg["sign_state"]).relative_to(args.home))
    record = {"home": str(args.home), "phase": "prepared", "sign_state": relative_state, "backup_manifest": tree_manifest(backup)}
    journal_write(work, record)
    sync_dir(args.home.parent)
    try:
        update_config(args.home / "config/config.toml", {
            "statesync": {"enable": True, "use-p2p": False, "rpc-servers": args.rpc_server,
                          "trust-height": args.trust_height, "trust-hash": args.trust_hash.upper()},
            "p2p": {"persistent-peers": args.peer},
        })
        decoded = load_config(args)
        require(decoded["enable"] is True and decoded["use_p2p"] is False, "native decoder did not enable RPC state sync")
        for key, expected in (("rpc_servers", args.rpc_server), ("trust_height", args.trust_height),
                              ("trust_hash", args.trust_hash.upper()), ("persistent_peers", args.peer)):
            require(decoded[key] == expected, f"native config round-trip failed for {key}")
        for key in ("genesis", "node_key", "validator_key", "sign_state", "remote_signer", "db_dir", "chain_id", "node_id", "trust_period"):
            require(decoded[key] == cfg[key], f"native config changed recovery identity: {key}")
        run_controller(args, "check-stopped")
        shutil.rmtree(args.home / "data")
        (args.home / "data").mkdir(mode=0o700)
        live_state = args.home / relative_state
        live_state.parent.mkdir(parents=True, exist_ok=True)
        atomic_write(live_state, (backup / relative_state).read_bytes())
        if (args.home / "wasm").exists():
            shutil.rmtree(args.home / "wasm")
        (args.home / "wasm").mkdir(mode=0o700)
        durable_tree(args.home)
        record["phase"] = "replaced"
        journal_write(work, record)
        startup(args, cfg, max(args.trust_height, args.signed_floor[0]))
        record["phase"] = "verified"
        journal_write(work, record)
        print(f"Recovery startup verified. Private backup retained: {work}", flush=True)
    except BaseException:
        # A failed startup may already have signed. Never restore an older H/R/S.
        rollback(args, work)
        raise


def snapshot(args, work):
    require(not work.exists(), f"recovery journal exists: {work}")
    require(not args.output.exists() and not args.output.is_relative_to(args.home), "snapshot output must be new and outside the home")
    regular_tree(args.home)
    stop(args)
    cfg = load_config(args)
    state = Path(cfg["sign_state"])
    state_bytes = state.read_bytes()
    args.signed_floor = signing_state(state)
    args.signed_state = json.loads(state_bytes)
    before = tree_manifest(args.home)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    stage = Path(tempfile.mkdtemp(prefix=".snapshot-", dir=args.output.parent))
    try:
        for name in ("data", "wasm"):
            source = args.home / name
            require(source.is_dir(), f"missing snapshot directory: {name}")
            excluded = {state.relative_to(source)} if state.is_relative_to(source) else set()
            manifest = tree_manifest(source, excluded)
            archive = stage / f"{name}.tar.gz"
            with tarfile.open(archive, "w:gz") as packed:
                for relative in manifest:
                    require(Path(relative).name not in ("priv_validator_key.json", "node_key.json", "priv_validator_state.json"), "private identity found in snapshot data")
                    packed.add(source / relative, arcname=f"{name}/{relative}", recursive=False)
            with tempfile.TemporaryDirectory(prefix=".snapshot-restore-", dir=args.output.parent) as directory:
                restored = Path(directory)
                with tarfile.open(archive, "r:gz") as packed:
                    packed.extractall(restored, filter="data")
                (restored / name).mkdir(exist_ok=True)
                require(tree_manifest(restored / name) == manifest, "snapshot restoration verification failed")
        require(state.read_bytes() == state_bytes and tree_manifest(args.home) == before, "home changed while stopped")
        atomic_write(stage / "manifest.json", json.dumps({"chain_id": cfg["chain_id"], "files": before_without_identity(before, cfg, args.home)}, sort_keys=True).encode())
        durable_tree(stage)
        os.rename(stage, args.output)
        sync_dir(args.output.parent)
        if args.restart:
            try:
                startup(args, cfg, args.signed_floor[0])
            except BaseException:
                stop(args)
                raise
        print(f"Offline snapshot restored and verified: {args.output}", flush=True)
    finally:
        if stage.exists():
            shutil.rmtree(stage)


def before_without_identity(manifest, cfg, home):
    state = str(Path(cfg["sign_state"]).relative_to(home))
    return {key: value for key, value in manifest.items() if key.startswith(("data/", "wasm/")) and key != state}


def main(argv=None, *, controller=None, config_loader=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("operation", choices=("snapshot", "state-sync"))
    parser.add_argument("--home", type=Path, default=Path.home() / ".pax")
    parser.add_argument("--controller", default=str(Path(__file__).with_name("paxd-service.sh")))
    parser.add_argument("--config-loader", required=True)
    parser.add_argument("--local-rpc", default="http://127.0.0.1:26657")
    parser.add_argument("--startup-timeout", type=float, default=120)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--restart", action="store_true")
    parser.add_argument("--rpc-server", action="append", default=[])
    parser.add_argument("--peer")
    parser.add_argument("--trust-height", type=int, default=0)
    parser.add_argument("--trust-hash", default="")
    parser.add_argument("--rollback", action="store_true")
    args = parser.parse_args(argv)
    args.controller = CommandController(args.controller) if controller is None else controller
    args.config_loader = NativeConfigLoader(args.config_loader) if config_loader is None else config_loader
    require(not args.home.is_symlink(), "home must not be a symlink")
    args.home = args.home.resolve()
    require(args.home != Path("/") and args.home.parent.is_dir(), "invalid home path")
    require(args.startup_timeout > 0, "startup timeout must be positive")
    rpc_url(args.local_rpc)
    work = args.home.with_name(f".{args.home.name}.maintenance")
    if args.operation == "snapshot":
        require(args.output is not None, "snapshot requires --output")
        args.output = args.output.resolve()
    lock = args.home.with_name(f".{args.home.name}.maintenance.lock")
    previous_mask = os.umask(0o077)
    previous_handler = signal.getsignal(signal.SIGTERM)
    try:
        fd = os.open(lock, os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
        with os.fdopen(fd, "w"):
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
            signal.signal(signal.SIGTERM, lambda *_: sys.exit("maintenance interrupted"))
            if args.operation == "snapshot":
                snapshot(args, work)
            else:
                state_sync(args, work)
    finally:
        signal.signal(signal.SIGTERM, previous_handler)
        os.umask(previous_mask)


if __name__ == "__main__":
    try:
        main()
    except (MaintenanceError, OSError, ValueError, KeyError, subprocess.SubprocessError) as error:
        print(f"maintenance refused: {error}", file=sys.stderr)
        sys.exit(1)
