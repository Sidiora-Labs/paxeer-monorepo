"""Bounded native snapshot restore probe used by the chain-operation suites.

Only the CLI launches paxd. Evidence validation is also exercised in-process by
qualification; that does not execute or certify the native snapshot importer.
"""

import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import socket
import subprocess
import tempfile
import time
import tomllib
from urllib.error import URLError
from urllib.request import urlopen


DONOR_RPC = "http://192.168.10.10:26657"
WITNESS_RPC = "http://192.168.10.11:26657"
BUDGET_SECONDS = 90


def require(condition, reason):
    if not condition:
        raise ValueError(reason)


def positive(value):
    require(not isinstance(value, bool) and re.fullmatch(r"[1-9][0-9]*", str(value)),
            "expected positive integer")
    return int(value)


def digest(value):
    """slog JSON encodes native []byte fields as base64."""
    raw = base64.b64decode(value, validate=True)
    require(len(raw) == 32, "expected SHA-256 digest")
    return raw.hex()


def snapshot_metadata(directory, height, snapshot_format):
    height, snapshot_format = positive(height), positive(snapshot_format)
    path = Path(directory) / str(height) / str(snapshot_format)
    chunks = sorted(path.iterdir(), key=lambda p: int(p.name))
    require(chunks and [p.name for p in chunks] == [str(i) for i in range(len(chunks))],
            "empty or noncontiguous snapshot chunks")
    total = hashlib.sha256()
    for chunk in chunks:
        require(chunk.is_file() and not chunk.is_symlink() and chunk.stat().st_size > 0,
                "missing or empty snapshot chunk")
        with chunk.open("rb") as stream:
            for block in iter(lambda: stream.read(1024 * 1024), b""):
                total.update(block)
    return {"height": height, "format": snapshot_format, "chunks": len(chunks),
            "hash": total.hexdigest()}


def import_outcome(log):
    """Require one ordered, complete import; replay/acceptance alone cannot pass."""
    events = []
    for line in log.splitlines():
        if line.startswith("{"):
            event = json.loads(line)
            if event.get("logger") == "tendermint/internal/statesync":
                events.append(event)
    restored = [e for e in events if e.get("msg") == "Snapshot restored"]
    require(len(restored) == 1, "missing or ambiguous completed import")
    final = restored[0]
    height, fmt = positive(final["height"]), positive(final["format"])
    snapshot_hash = digest(final["hash"])
    before = events[:events.index(final)]
    discovered = [e for e in before
                  if e.get("msg") == "discovered and added new snapshot"
                  and e.get("height") == height and e.get("format") == fmt
                  and digest(e["hash"]) == snapshot_hash]
    require(len(discovered) == 1, "missing advertised snapshot identity")
    accepted = [i for i, e in enumerate(before)
                if e.get("msg") == "Snapshot accepted, restoring"
                and e.get("height") == height and e.get("format") == fmt
                and digest(e["hash"]) == snapshot_hash]
    require(len(accepted) == 1 and before.index(discovered[0]) < accepted[0],
            "missing accepted snapshot identity")
    applied = [e for e in before[accepted[0] + 1:]
               if e.get("msg") == "Applied snapshot chunk to ABCI app"
               and e.get("height") == height and e.get("format") == fmt]
    require(applied, "no imported chunks")
    count = positive(applied[0]["total"])
    require(all(positive(e["total"]) == count for e in applied)
            and {e["chunk"] for e in applied} == set(range(count)),
            "incomplete imported chunks")
    verified = [e for e in before[accepted[0] + 1:]
                if e.get("msg") == "Verified ABCI app" and e.get("height") == height]
    require(len(verified) == 1 and before.index(verified[0]) > before.index(applied[-1]),
            "missing final ABCI verification")
    return {"height": height, "format": fmt, "chunks": count, "hash": snapshot_hash,
            "app_hash": digest(verified[0]["appHash"]), "peer": discovered[0]["peer"]}


def validate_comparison(bootstrap, evidence, metadata, donor_state):
    for key in ("height", "format", "chunks", "hash"):
        require(evidence[key] == metadata[key], "snapshot metadata mismatch: " + key)
    require(evidence["chain_id"] == bootstrap["chain_id"] == donor_state["chain_id"],
            "chain identity mismatch")
    require(evidence["address"] == bootstrap["address"] == donor_state["address"],
            "known account mismatch")
    require(positive(evidence["height"]) == positive(donor_state["height"]),
            "state compared at different heights")
    require(positive(evidence["amount"]) == positive(donor_state["amount"]),
            "known application state mismatch")
    require(evidence["denom"] == donor_state["denom"] == "uhpx", "denom mismatch")
    require(re.fullmatch(r"[0-9a-f]{64}", evidence["app_hash"])
            and evidence["app_hash"] == evidence["header_app_hash"]
            == donor_state["header_app_hash"], "committed application hash mismatch")
    require(evidence["peer"] == bootstrap["peer"].split("@", 1)[0],
            "snapshot came from a different donor")
    require(evidence["fresh_home"] is True, "receiver was not clean")
    return "verified"


def remaining(deadline):
    seconds = deadline - time.monotonic()
    if seconds <= 0:
        raise TimeoutError("snapshot probe exceeded bounded deadline")
    return min(seconds, 5)


def rpc(url, method, deadline):
    with urlopen(url + "/" + method, timeout=remaining(deadline)) as response:
        data = json.load(response)
    require("error" not in data, "RPC error: " + str(data.get("error")))
    return data["result"]


def command(argv, deadline):
    return subprocess.run(argv, check=True, text=True, capture_output=True,
                          timeout=remaining(deadline)).stdout.strip()


def bank_state(url, address, height, home, deadline):
    # Header H+1 commits the application hash after executing block H.
    block = rpc(url, "block?height=" + str(positive(height) + 1), deadline)["block"]
    header = block["header"]
    require(positive(header["height"]) == height + 1, "wrong committed header height")
    balance = json.loads(command([
        "paxd", "q", "bank", "balances", address, "--denom", "uhpx",
        "--height", str(height), "--node", url, "--home", str(home), "--output", "json",
    ], deadline))
    require(balance["denom"] == "uhpx", "wrong known-state denomination")
    return {"chain_id": header["chain_id"], "address": address, "height": height,
            "denom": balance["denom"], "amount": str(positive(balance["amount"])),
            "header_app_hash": header["app_hash"].lower()}


def prepare(directory, home, deadline):
    # Wait for actual nonempty exported chunks in the configured custom location.
    while True:
        exports = list(Path(directory).glob("[0-9]*/[0-9]*"))
        if exports:
            newest = max(exports, key=lambda p: (int(p.parent.name), int(p.name)))
            snapshot_metadata(directory, newest.parent.name, newest.name)
            break
        time.sleep(min(1, remaining(deadline)))
    genesis = json.loads((home / "config/genesis.json").read_text())
    accounts = genesis["app_state"]["bank"]["balances"]
    address = next(a["address"] for a in accounts
                   if any(c["denom"] == "uhpx" and int(c["amount"]) > 0 for c in a["coins"]))
    status = rpc(DONOR_RPC, "status", deadline)
    height = positive(status["sync_info"]["latest_block_height"])
    block = rpc(DONOR_RPC, "block?height=" + str(height), deadline)
    witness = rpc(WITNESS_RPC, "block?height=" + str(height), deadline)
    require(block["block_id"]["hash"] == witness["block_id"]["hash"],
            "donor and witness disagree on trust block")
    require(block["block"]["header"]["chain_id"] == genesis["chain_id"], "wrong donor chain")
    return {"chain_id": genesis["chain_id"], "address": address, "trust_height": height,
            "trust_hash": block["block_id"]["hash"],
            "peer": status["node_info"]["id"] + "@192.168.10.10:26656"}


def set_toml(text, section, values):
    """Replace a section's keys, preserving unrelated configuration and arrays."""
    headers = list(re.finditer(r"(?m)^\[([^]\n]+)\]\s*$", text))
    start, end = 0, headers[0].start() if headers else len(text)
    if section:
        match = next((i for i, h in enumerate(headers) if h[1] == section), None)
        require(match is not None, "missing TOML section: " + section)
        start = headers[match].end()
        end = headers[match + 1].start() if match + 1 < len(headers) else len(text)
    body = text[start:end]
    for key, value in values.items():
        encoded = json.dumps(value)
        pattern = r"(?m)^" + re.escape(key) + r"\s*=.*$"
        if re.search(pattern, body):
            body = re.sub(pattern, lambda _: key + " = " + encoded, body)
        else:
            body += "\n" + key + " = " + encoded + "\n"
    result = text[:start] + body + "\n" + text[end:]
    parsed = tomllib.loads(result)
    target = parsed[section] if section else parsed
    require(all(target[k] == v for k, v in values.items()), "TOML update failed")
    return result


def receiver_config(config, app, bootstrap, rpc_port, p2p_port, directory):
    config = set_toml(config, "", {"mode": "full"})
    config = set_toml(config, "rpc", {"laddr": f"tcp://127.0.0.1:{rpc_port}", "pprof-laddr": ""})
    config = set_toml(config, "p2p", {"laddr": f"tcp://127.0.0.1:{p2p_port}",
                      "persistent-peers": bootstrap["peer"], "pex": False,
                      "allow-duplicate-ip": True})
    config = set_toml(config, "statesync", {"enable": True, "use-local-snapshot": False,
                      "rpc-servers": DONOR_RPC + "," + WITNESS_RPC,
                      "trust-height": positive(bootstrap["trust_height"]),
                      "trust-hash": bootstrap["trust_hash"], "discovery-time": "5s"})
    config = set_toml(config, "instrumentation", {"prometheus": False})
    for section in ("api", "grpc", "grpc-web", "rosetta"):
        app = set_toml(app, section, {"enable": False})
    app = set_toml(app, "evm", {"http_enabled": False, "ws_enabled": False})
    app = set_toml(app, "state-store", {"ss-enable": True})
    app = set_toml(app, "state-sync", {"snapshot-directory": str(directory / "snapshots"),
                                      "snapshot-interval": 0})
    return config, app


def restore(bootstrap, source_home, deadline):
    # A distinct home avoids touching the running validator/RPC node or its keys.
    with tempfile.TemporaryDirectory(prefix="chain-restore-") as directory:
        home = Path(directory)
        command(["paxd", "init", "snapshot-probe", "--chain-id", bootstrap["chain_id"],
                 "--home", str(home)], deadline)
        allowed = {"priv_validator_state.json"}
        require({p.name for p in (home / "data").iterdir()} <= allowed,
                "fresh receiver already contains application or consensus state")
        shutil.copyfile(source_home / "config/genesis.json", home / "config/genesis.json")
        genesis = json.loads((home / "config/genesis.json").read_text())
        require(genesis["chain_id"] == bootstrap["chain_id"], "receiver genesis mismatch")
        with socket.socket() as rpc_socket, socket.socket() as p2p_socket:
            rpc_socket.bind(("127.0.0.1", 0))
            p2p_socket.bind(("127.0.0.1", 0))
            rpc_port, p2p_port = rpc_socket.getsockname()[1], p2p_socket.getsockname()[1]
        config, app = receiver_config(
            (home / "config/config.toml").read_text(),
            (source_home / "config/app.toml").read_text(), bootstrap, rpc_port, p2p_port, home,
        )
        (home / "config/config.toml").write_text(config)
        (home / "config/app.toml").write_text(app)
        environment = dict(os.environ, PAX_LOG_FORMAT="json", PAX_LOG_OUTPUT="stdout",
                           PAX_LOG_LEVEL="info")
        log_path = home / "restore.log"
        with log_path.open("w") as output:
            process = subprocess.Popen(["paxd", "start", "--home", str(home), "--chain-id",
                                        bootstrap["chain_id"]], stdout=output,
                                       stderr=subprocess.STDOUT, env=environment)
            try:
                while True:
                    remaining(deadline)
                    require(process.poll() is None, "receiver exited: " + log_path.read_text()[-4000:])
                    log = log_path.read_text()
                    if '"msg":"Snapshot restored"' in log or '"msg": "Snapshot restored"' in log:
                        outcome = import_outcome(log)
                        break
                    time.sleep(min(0.5, remaining(deadline)))
                url = f"http://127.0.0.1:{rpc_port}"
                while True:
                    try:
                        status = rpc(url, "status", deadline)
                    except URLError:
                        time.sleep(min(0.5, remaining(deadline)))
                        continue
                    if int(status["sync_info"]["latest_block_height"]) > outcome["height"]:
                        break
                    time.sleep(min(0.5, remaining(deadline)))
                state = bank_state(url, bootstrap["address"], outcome["height"], home, deadline)
                return dict(outcome, **state, fresh_home=True)
            finally:
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=5)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("operation", choices=("prepare", "restore", "compare"))
    parser.add_argument("--snapshot-dir", type=Path)
    parser.add_argument("--bootstrap")
    parser.add_argument("--evidence")
    args = parser.parse_args()
    deadline = time.monotonic() + BUDGET_SECONDS
    home = Path.home() / ".pax"
    if args.operation == "prepare":
        require(args.snapshot_dir is not None, "snapshot directory is required")
        print(json.dumps(prepare(args.snapshot_dir, home, deadline), separators=(",", ":")))
    elif args.operation == "restore":
        print(json.dumps(restore(json.loads(args.bootstrap), home, deadline), separators=(",", ":")))
    else:
        bootstrap, evidence = json.loads(args.bootstrap), json.loads(args.evidence)
        metadata = snapshot_metadata(args.snapshot_dir, evidence["height"], evidence["format"])
        state = bank_state(DONOR_RPC, bootstrap["address"], evidence["height"], home, deadline)
        print(validate_comparison(bootstrap, evidence, metadata, state))


if __name__ == "__main__":
    main()
