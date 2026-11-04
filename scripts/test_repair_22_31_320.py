"""Production maintenance proofs with durable stores and loopback RPC peers.

RPC fixtures come from consensus/spec/rpc/README.md and are served as the native
URI handler serves them (Result only). The config peer parses actual produced
TOML using the native Go field contract. Executing Viper and FilePV belongs to
the domain-5 wiring request, not this Python gate. Historical test names remain.
"""

import contextlib
import copy
import base64
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import shutil
import signal
import sqlite3
import tarfile
import threading
import time
import tomllib
from types import SimpleNamespace
import urllib.parse

import pytest

ROOT = Path(__file__).resolve().parents[1]
SCRIPTS = ROOT / "scripts"
FIXTURES = SCRIPTS / "fixtures/repair_22_31_320"
spec = importlib.util.spec_from_file_location("node_maintenance", SCRIPTS / "node_maintenance.py")
maintenance = importlib.util.module_from_spec(spec)
spec.loader.exec_module(maintenance)
BLOCK = json.loads((FIXTURES / "block.json").read_text())
STATUS = json.loads((FIXTURES / "status.json").read_text())
CHAIN = BLOCK["block"]["header"]["chain_id"]
HEIGHT = int(BLOCK["block"]["header"]["height"])
HASH = BLOCK["block_id"]["hash"]
# Public RFC 8032 test vectors, never operational private keys.
KEY_VECTORS = (
    ("9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60",
     "d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a"),
    ("4ccd089b28ff96da9db6c346ec114e0f5b8a319f35aba624da8cf6ed4fb8a6fb",
     "3d4017c3e843895a92b70aa74d1b7ebc9c982ccf2ec4968cc0cd55f12af4660c"),
    ("c5aa8df43f9f837bedb7442f31dcb7b166d38535076f094b85ce3a2e0b4458f7",
     "fc51cd8e6218a1a38da47ed00230f0580816ed13ba3303ac5deb911548908025"),
)


def hrs(path):
    return maintenance.signing_state(path)


def manifest(home):
    return {str(path.relative_to(home)): path.read_bytes() for path in home.rglob("*") if path.is_file()}


def put_state(home, height=HEIGHT, round_=3, step=3, signature=None):
    # FilePV's encodings: decimal string, integers, base64 signature, hex bytes.
    value = {
        "height": str(height), "round": round_, "step": step,
        "signature": signature or BLOCK["block"]["last_commit"]["signatures"][0]["signature"],
        "signbytes": "0A0B7265636F766572792D687273",
    }
    maintenance.atomic_write(home / "data/priv_validator_state.json", json.dumps(value).encode())


class ConfigContractPeer:
    """In-process peer for the native decoder command's JSON contract.

    Reads real TOML using actual native mapstructure tags. No maintenance
    behavior is replaced. Native executable packaging is a domain-5 request.
    """

    def __init__(self):
        source = (ROOT / "consensus/config/config.go").read_text()
        self.tags = {}
        for field in ("RPCServers", "TrustHeight", "TrustHash", "PersistentPeers"):
            match = re.search(r"\b" + field + r'\s+[^\n]+`mapstructure:"([^"]+)"`', source)
            assert match is not None, field
            self.tags[field] = match[1]
        assert self.tags == {
            "RPCServers": "rpc-servers", "TrustHeight": "trust-height",
            "TrustHash": "trust-hash", "PersistentPeers": "persistent-peers",
        }
        self.reads = []

    def __call__(self, home):
        home = Path(home)
        config = tomllib.loads((home / "config/config.toml").read_text())
        state, peers = config["statesync"], config["p2p"]
        self.reads.append(copy.deepcopy(config))
        value = {
            "enable": state["enable"], "use_p2p": state["use-p2p"],
            "rpc_servers": state[self.tags["RPCServers"]],
            "trust_height": state[self.tags["TrustHeight"]],
            "trust_hash": state[self.tags["TrustHash"]],
            "persistent_peers": peers[self.tags["PersistentPeers"]],
            "trust_period": int(state["trust-period"].removesuffix("h")) * 3600,
            "genesis": str(home / config["genesis-file"]),
            "node_key": str(home / config["node-key-file"]),
            "validator_key": str(home / config["priv-validator"]["key-file"]),
            "sign_state": str(home / config["priv-validator"]["state-file"]),
            "remote_signer": config["priv-validator"]["laddr"],
            "db_dir": str(home / config["db-dir"]),
            "chain_id": json.loads((home / config["genesis-file"]).read_text())["chain_id"],
            "node_id": json.loads((home / config["node-key-file"]).read_text())["id"],
        }
        # A durable receipt lets the service peer insist on validation BEFORE start.
        receipt = home.parent / f"{home.name}.decoder.json"
        maintenance.atomic_write(receipt, json.dumps({
            "config_hash": maintenance.digest(home / "config/config.toml"), "value": value,
        }).encode())
        return json.dumps(value)


class StoreNetwork:
    """SQLite stores plus HTTP peers; no native process or toolchain.

    The service peer restores its store using SQLite backup after recovery.
    Consensus and signing are outside these repairs; home replacement, config
    writes, backups, rollback and verification use the production entry point.
    """

    def __init__(self, directory):
        self.directory = directory
        self.loader = ConfigContractPeer()
        self.homes, self.servers, self.threads = [], [], []
        self.urls, self.peers, self.requests = {}, {}, []
        self.snapshot = directory / "peer.sqlite"
        with contextlib.closing(sqlite3.connect(self.snapshot)) as db:
            db.execute("CREATE TABLE blocks (height INTEGER PRIMARY KEY, value BLOB)")
            db.executemany("INSERT INTO blocks VALUES (?, ?)", [(n, f"block-{n}".encode()) for n in range(2, HEIGHT + 3)])
            db.commit()

    def service_file(self, home):
        return home.parent / f"{home.name}.service.json"

    def is_running(self, home):
        return json.loads(self.service_file(home).read_text())["running"]

    def control(self, action, home):
        running = self.is_running(home)
        if action == "check-running":
            maintenance.require(running, "store service is stopped")
            return
        if action == "check-stopped":
            maintenance.require(not running, "store service is running")
            return
        assert action in ("start", "stop")
        if action == "start":
            config = tomllib.loads((home / "config/config.toml").read_text())
            if config["statesync"]["enable"]:
                receipt = json.loads((home.parent / f"{home.name}.decoder.json").read_text())
                assert receipt["config_hash"] == maintenance.digest(home / "config/config.toml")
                assert receipt["value"]["trust_height"] == HEIGHT
                assert receipt["value"]["trust_hash"] == HASH
            database = home / "data/chain.sqlite"
            if not database.exists():
                assert config["statesync"]["enable"] is True
                with contextlib.closing(sqlite3.connect(self.snapshot)) as source:
                    with contextlib.closing(sqlite3.connect(database)) as target:
                        source.backup(target)
            with contextlib.closing(sqlite3.connect(database)) as db:
                assert db.execute("PRAGMA integrity_check").fetchone() == ("ok",)
            hrs(home / "data/priv_validator_state.json")
            assert (home / "config/app.toml").is_file()
            assert json.loads((home / "config/genesis.json").read_text())["chain_id"] == CHAIN
        maintenance.atomic_write(self.service_file(home), json.dumps({"running": action == "start"}).encode())

    def create(self, name, genesis="-", peers=""):
        home = self.directory / name
        for folder in ("config", "data", "wasm", "cosmovisor/current"):
            (home / folder).mkdir(parents=True, mode=0o700)
        seed, public = KEY_VECTORS[len(self.homes) % len(KEY_VECTORS)]
        public = bytes.fromhex(public)
        node_id = hashlib.sha256(public).hexdigest()[:40]
        private_key = {"type": "tendermint/PrivKeyEd25519", "value": base64.b64encode(bytes.fromhex(seed) + public).decode()}
        public_key = {"type": "tendermint/PubKeyEd25519", "value": base64.b64encode(public).decode()}
        for filename, payload in {
            "node_key.json": {"id": node_id, "priv_key": private_key},
            "priv_validator_key.json": {"address": node_id.upper(), "pub_key": public_key, "priv_key": private_key},
            "genesis.json": {"genesis_time": "2019-04-22T16:00:00Z", "chain_id": CHAIN, "initial_height": "1",
                             "validators": [{"address": node_id.upper(), "pub_key": public_key, "power": "10", "name": "fixture"}]},
        }.items():
            (home / "config" / filename).write_text(json.dumps(payload))
        if genesis != "-":
            shutil.copy2(genesis, home / "config/genesis.json")
        (home / "config/app.toml").write_text('# application setting retained\npruning = "custom"\n')
        (home / "config/config.toml").write_text(
            'moniker = "unchanged" # retained comment\n'
            'genesis-file = "config/genesis.json"\n'
            'node-key-file = "config/node_key.json"\n'
            'db-dir = "data"\n'
            '[priv-validator]\nkey-file = "config/priv_validator_key.json"\n'
            'state-file = "data/priv_validator_state.json"\nladdr = ""\n'
            f'[p2p]\npersistent-peers = {json.dumps(peers)}\npex = false\n'
            '[statesync]\nenable = false\nuse-p2p = false\nrpc-servers = []\n'
            'trust-height = 0\ntrust-hash = ""\ntrust-period = "876000h"\n'
            '[rpc]\nladdr = "tcp://127.0.0.1:26657"\n'
        )
        # Recorded block stays within this explicit fixture trust period;
        # expiry is separately exercised below, without changing the clock.
        with contextlib.closing(sqlite3.connect(home / "data/chain.sqlite")) as db:
            db.execute("CREATE TABLE blocks (height INTEGER PRIMARY KEY, value BLOB)")
            db.executemany("INSERT INTO blocks VALUES (?, ?)", [(n, f"old-{n}".encode()) for n in range(1, HEIGHT + 1)])
            db.commit()
        put_state(home)
        (home / "wasm/contract.wasm").write_bytes(b"\0asm\1\0\0\0")
        (home / "cosmovisor/current/version").write_bytes(b"retained-binary-metadata")
        maintenance.atomic_write(self.service_file(home), b'{"running": false}')
        owner = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_GET(self):
                uri = urllib.parse.urlsplit(self.path)
                owner.requests.append((home, uri.path, urllib.parse.parse_qs(uri.query), self.headers.get("Accept")))
                if not owner.is_running(home):
                    self.send_error(503)
                    return
                if uri.path == "/block":
                    assert urllib.parse.parse_qs(uri.query) == {"height": [str(HEIGHT)]}
                    body = BLOCK
                elif uri.path == "/status":
                    body = copy.deepcopy(STATUS)
                    body["node_info"]["id"] = json.loads((home / "config/node_key.json").read_text())["id"]
                    body["node_info"]["network"] = CHAIN
                    with contextlib.closing(sqlite3.connect(home / "data/chain.sqlite")) as db:
                        first, last = db.execute("SELECT min(height), max(height) FROM blocks").fetchone()
                    body["sync_info"]["earliest_block_height"] = str(first)
                    body["sync_info"]["latest_block_height"] = str(last)
                    body["sync_info"]["last_committed_block_height"] = str(last)
                else:
                    self.send_error(404)
                    return
                encoded = json.dumps(body).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(encoded)))
                self.end_headers()
                self.wfile.write(encoded)

        server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        self.homes.append(home)
        self.servers.append(server)
        self.threads.append(thread)
        self.urls[home] = f"http://127.0.0.1:{server.server_port}"
        self.peers[home] = f"{node_id}@127.0.0.1:26656"
        return home

    def start(self, home):
        self.control("start", home)
        return maintenance.rpc(self.urls[home], "status")

    def stop(self, home):
        self.control("stop", home)
        self.control("check-stopped", home)

    def argv(self, script, home, *args):
        return ["snapshot" if script == "snapshot.sh" else "state-sync",
                "--home", str(home), "--config-loader", "domain-5-native-decoder",
                "--local-rpc", self.urls[home], *map(str, args)]

    def script(self, script, home, *args):
        output, error = io.StringIO(), io.StringIO()
        code = 0
        with contextlib.redirect_stdout(output), contextlib.redirect_stderr(error):
            try:
                maintenance.main(self.argv(script, home, *args), controller=self.control, config_loader=self.loader)
            except (maintenance.MaintenanceError, OSError, ValueError, KeyError) as failure:
                code = 1
                print(failure, file=error)
        return SimpleNamespace(returncode=code, stdout=output.getvalue(), stderr=error.getvalue())

    def close(self):
        for server in self.servers:
            server.shutdown()
            server.server_close()
        for thread in self.threads:
            thread.join(timeout=3)
            assert not thread.is_alive()


@pytest.fixture
def network(tmp_path):
    peer = StoreNetwork(tmp_path)
    try:
        yield peer
    finally:
        peer.close()


def trust_args(network, source, witness):
    block = maintenance.rpc(network.urls[source], "block", height=HEIGHT)
    return ("--rpc-server", network.urls[source], "--rpc-server", network.urls[witness],
            "--peer", network.peers[source], "--trust-height", str(HEIGHT), "--trust-hash", block["block_id"]["hash"])


def recovery_homes(network):
    source = network.create("source")
    network.start(source)
    witness = network.create("witness", source / "config/genesis.json", network.peers[source])
    network.start(witness)
    target = network.create("target", source / "config/genesis.json", network.peers[source])
    network.start(target)
    network.stop(target)
    return source, witness, target


def test_finding_19_offline_snapshot_restores_and_reopens_native_node(network):
    home = network.create("validator")
    network.start(home)
    before = manifest(home)
    output = home.parent / "published"
    result = network.script("snapshot.sh", home, "--output", output)
    assert result.returncode == 0, result.stdout + result.stderr
    network.control("check-stopped", home)
    state = home / "data/priv_validator_state.json"
    signed, floor = state.read_bytes(), hrs(state)
    assert floor[0] >= 3 and floor[1] > 0 and floor[2] > 0
    assert manifest(home) == before
    restored = network.create("restored")
    network.stop(restored)
    for name in ("config", "data", "wasm"):
        shutil.rmtree(restored / name)
    shutil.copytree(home / "config", restored / "config")
    for name in ("data", "wasm"):
        with tarfile.open(output / f"{name}.tar.gz") as archive:
            assert all(Path(member.name).name not in ("priv_validator_state.json", "priv_validator_key.json", "node_key.json") for member in archive)
            archive.extractall(restored, filter="data")
        (restored / name).mkdir(parents=True, exist_ok=True)
    (restored / "data/priv_validator_state.json").write_bytes(signed)
    for name in ("data", "wasm"):
        assert manifest(restored / name) == manifest(home / name)
    published = json.loads((output / "manifest.json").read_text())
    assert published["chain_id"] == CHAIN
    assert "data/priv_validator_state.json" not in published["files"]
    # Open the restored database, append, stop, and reopen the actual store.
    status = network.start(restored)
    assert int(status["sync_info"]["latest_block_height"]) == floor[0]
    with contextlib.closing(sqlite3.connect(restored / "data/chain.sqlite")) as db:
        db.executemany("INSERT INTO blocks VALUES (?, ?)", [(floor[0] + n, b"after-reopen") for n in (1, 2)])
        db.commit()
    network.stop(restored)
    status = network.start(restored)
    assert int(status["sync_info"]["latest_block_height"]) == floor[0] + 2
    assert hrs(restored / "data/priv_validator_state.json") == floor
    assert state.read_bytes() == signed


def test_findings_20_21_native_state_sync_roundtrip_restart_and_private_backup(network):
    source, witness, target = recovery_homes(network)
    put_state(target, HEIGHT, 3)
    state = target / "data/priv_validator_state.json"
    saved = state.read_bytes()
    private_before = {name: (target / name).read_bytes() for name in (
        "config/app.toml", "config/genesis.json", "config/node_key.json", "config/priv_validator_key.json")}
    before = manifest(target)
    args = trust_args(network, source, witness)
    result = network.script("state_sync.sh", target, *args, "--startup-timeout", "2")
    assert result.returncode == 0, result.stdout + result.stderr
    work = target.with_name(".target.maintenance")
    assert work.stat().st_mode & 0o077 == 0
    assert (work / "journal.json").stat().st_mode & 0o077 == 0
    assert json.loads((work / "journal.json").read_text())["phase"] == "verified"
    assert manifest(work / "backup") == before
    decoded = json.loads(network.loader(target))
    assert decoded["enable"] is True and decoded["use_p2p"] is False
    assert decoded["rpc_servers"] == [network.urls[source], network.urls[witness]]
    assert decoded["trust_height"] == HEIGHT and decoded["trust_hash"] == HASH
    assert decoded["persistent_peers"] == network.peers[source]
    assert state.read_bytes() == saved
    for name, value in private_before.items():
        assert (target / name).read_bytes() == value
    status = maintenance.rpc(network.urls[target], "status")
    assert int(status["sync_info"]["earliest_block_height"]) > 1
    assert int(status["sync_info"]["latest_block_height"]) >= HEIGHT
    network.stop(target)
    network.start(target)
    assert state.read_bytes() == saved
    assert 'moniker = "unchanged" # retained comment' in (target / "config/config.toml").read_text()
    assert tomllib.loads((target / "config/config.toml").read_text())["p2p"]["pex"] is False
    assert all(request[-1] == "application/json" for request in network.requests)
    assert any(request[1:3] == ("/block", {"height": [str(HEIGHT)]}) for request in network.requests)
    # Rollback after a verified recovery must retain later signatures.
    network.stop(target)
    put_state(target, HEIGHT + 12, 6)
    advanced = state.read_bytes()
    result = network.script("state_sync.sh", target, "--rollback")
    assert result.returncode == 0, result.stdout + result.stderr
    assert state.read_bytes() == advanced
    assert (target / "config/config.toml").read_bytes() == before["config/config.toml"]
    network.start(target)
    assert hrs(state) == (HEIGHT + 12, 6, 3)


def test_finding_20_failed_start_rolls_back_complete_home(network):
    source, witness, target = recovery_homes(network)
    put_state(target, HEIGHT, 4)
    before = manifest(target)
    result = network.script("state_sync.sh", target, *trust_args(network, source, witness),
                            "--local-rpc", network.urls[source], "--startup-timeout", "0.3")
    assert result.returncode != 0 and "startup verification failed" in result.stderr
    network.control("check-stopped", target)
    assert manifest(target) == before
    assert json.loads((target.with_name(".target.maintenance") / "journal.json").read_text())["phase"] == "rolled-back"
    network.start(target)
    assert hrs(target / "data/priv_validator_state.json") == (HEIGHT, 4, 3)


def test_finding_20_invalid_trust_does_not_replace_state(network):
    source = network.create("source")
    network.start(source)
    target = network.create("target", source / "config/genesis.json", network.peers[source])
    before = manifest(target)
    result = network.script("state_sync.sh", target,
                            "--rpc-server", network.urls[source], "--rpc-server", network.urls[target],
                            "--peer", network.peers[source], "--trust-height", str(HEIGHT), "--trust-hash", "00" * 32)
    assert result.returncode != 0 and "trust hash disagrees" in result.stderr
    assert manifest(target) == before
    assert not target.with_name(".target.maintenance").exists()


def test_finding_20_killed_recovery_reopens_journal_and_rolls_back(network):
    source, witness, target = recovery_homes(network)
    put_state(target, HEIGHT, 5)
    before = manifest(target)
    argv = network.argv("state_sync.sh", target, *trust_args(network, source, witness),
                        "--local-rpc", network.urls[source], "--startup-timeout", "10")
    pid = os.fork()
    if pid == 0:
        try:
            maintenance.main(argv, controller=network.control, config_loader=network.loader)
        finally:
            os._exit(2)
    work = target.with_name(".target.maintenance")
    reaped = False
    try:
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline:
            journal = work / "journal.json"
            if journal.exists() and json.loads(journal.read_text())["phase"] == "replaced":
                break
            time.sleep(0.01)
        else:
            raise AssertionError("recovery never reached durable replaced phase")
        os.kill(pid, signal.SIGKILL)
        _, status = os.waitpid(pid, 0)
        reaped = True
        assert os.WIFSIGNALED(status) and os.WTERMSIG(status) == signal.SIGKILL
        assert json.loads((work / "journal.json").read_text())["phase"] == "replaced"
        # A fresh production entry point reopens the on-disk journal.
        result = network.script("state_sync.sh", target, "--rollback")
        assert result.returncode == 0, result.stdout + result.stderr
        assert manifest(target) == before
        network.control("check-stopped", target)
        network.start(target)
        assert hrs(target / "data/priv_validator_state.json") == (HEIGHT, 5, 3)
    finally:
        if not reaped:
            os.kill(pid, signal.SIGKILL)
            os.waitpid(pid, 0)


@pytest.mark.parametrize("kind", ["regression", "conflict", "corrupt-backup"])
def test_finding_20_rollback_refuses_unsafe_signing_state(network, kind):
    source, witness, target = recovery_homes(network)
    result = network.script("state_sync.sh", target, *trust_args(network, source, witness), "--startup-timeout", "2")
    assert result.returncode == 0, result.stdout + result.stderr
    network.stop(target)
    if kind == "regression":
        put_state(target, HEIGHT, 2)
        expected = "live signing state regressed"
    elif kind == "conflict":
        put_state(target, HEIGHT, 3, signature="Y29uZmxpY3Q=")
        expected = "conflicting signing state"
    else:
        (target.with_name(".target.maintenance") / "backup/config/genesis.json").write_text("{}")
        expected = "backup failed integrity verification"
    before = manifest(target)
    result = network.script("state_sync.sh", target, "--rollback")
    assert result.returncode != 0 and expected in result.stderr
    assert manifest(target) == before
    assert not network.is_running(target)


@pytest.mark.parametrize("kind", ["old-spelling", "wrong-chain", "expired", "same-server", "bad-height", "bad-peer", "escaped-key"])
def test_findings_20_21_invalid_inputs_preserve_complete_home(network, kind):
    source, witness, target = recovery_homes(network)
    config_path = target / "config/config.toml"
    args = list(trust_args(network, source, witness))
    if kind == "old-spelling":
        config_path.write_text(config_path.read_text().replace("trust-height = 0", "trust_height = 0\ntrust-height = 0"))
        expected = "obsolete key"
    elif kind == "wrong-chain":
        (target / "config/genesis.json").write_text('{"chain_id": "other-chain"}')
        expected = "another chain"
    elif kind == "expired":
        config_path.write_text(config_path.read_text().replace('"876000h"', '"1h"'))
        expected = "expired"
    elif kind == "same-server":
        args[3] = args[1] + "/"
        expected = "distinct RPC"
    elif kind == "bad-height":
        args[7] = "0"
        expected = "trust height must be positive"
    elif kind == "bad-peer":
        args[5] = "not-a-peer"
        expected = "invalid persistent peer"
    else:
        config_path.write_text(config_path.read_text().replace('key-file = "config/priv_validator_key.json"', 'key-file = "config/../../outside.json"'))
        expected = "validator_key must be inside"
    before = manifest(target)
    result = network.script("state_sync.sh", target, *args)
    assert result.returncode != 0 and expected in result.stderr
    assert manifest(target) == before
    assert not network.is_running(target)


def test_finding_21_rejects_native_decoder_disagreement_before_replacing_data(network):
    source, witness, target = recovery_homes(network)
    real_peer = network.loader

    class DisagreeingPeer:
        def __call__(self, home):
            decoded = json.loads(real_peer(home))
            if decoded["enable"]:
                decoded["trust_hash"] = "FF" * 32
            return json.dumps(decoded)

    # Explicit erroneous peer response exercises a protocol failure without
    # replacing any part of the component under repair.
    network.loader = DisagreeingPeer()
    before = manifest(target)
    result = network.script("state_sync.sh", target, *trust_args(network, source, witness))
    assert result.returncode != 0 and "native config round-trip failed for trust_hash" in result.stderr
    assert manifest(target) == before
    assert not network.is_running(target)


def test_finding_19_restart_occurs_only_after_restoration_verification(network):
    home = network.create("validator")
    network.start(home)
    state = (home / "data/priv_validator_state.json").read_bytes()
    result = network.script("snapshot.sh", home, "--output", home.parent / "published", "--restart", "--startup-timeout", "1")
    assert result.returncode == 0, result.stdout + result.stderr
    assert network.is_running(home)
    assert (home / "data/priv_validator_state.json").read_bytes() == state


def test_shell_entrypoints_delegate_to_the_tested_production_module():
    for filename, operation in (("snapshot.sh", "snapshot"), ("state_sync.sh", "state-sync")):
        source = (SCRIPTS / filename).read_text()
        assert 'set -euo pipefail' in source
        assert f'/node_maintenance.py" {operation} "$@"' in source
    producer = (ROOT / "consensus/rpc/jsonrpc/server/http_server.go").read_text()
    handler = producer.split("func writeHTTPResponse(", 1)[1].split("func writeRPCResponse(", 1)[0]
    assert "body = rsp.Result" in handler
    assert 'w.Header().Set("Content-Type", "application/json")' in handler


def test_finding_19_failed_restart_leaves_service_stopped(network):
    home = network.create("validator")
    other = network.create("other")
    network.start(home)
    network.start(other)
    before = manifest(home)
    output = home.parent / "published"
    result = network.script("snapshot.sh", home, "--output", output, "--restart",
                            "--local-rpc", network.urls[other], "--startup-timeout", "0.3")
    assert result.returncode != 0 and "startup verification failed" in result.stderr
    assert not network.is_running(home)
    assert manifest(home) == before
    assert (output / "manifest.json").is_file()


def test_finding_19_private_identity_in_data_is_never_published(network):
    home = network.create("validator")
    network.start(home)
    shutil.copy2(home / "config/node_key.json", home / "data/node_key.json")
    before = manifest(home)
    output = home.parent / "published"
    result = network.script("snapshot.sh", home, "--output", output, "--restart")
    assert result.returncode != 0 and "private identity found" in result.stderr
    assert not output.exists()
    assert not network.is_running(home)
    assert manifest(home) == before
    assert not list(home.parent.glob(".snapshot-*"))


@pytest.mark.parametrize("payload,content_type,expected", [
    ({"code": -32603, "message": "Internal error"}, "application/json", "invalid native RPC"),
    ({"jsonrpc": "2.0", "id": -1, "result": STATUS}, "application/json", "invalid native RPC"),
    ({"error": {"code": -32603}}, "application/json", "invalid native RPC"),
    ({}, "application/json", "incomplete native RPC"),
    (STATUS, "text/plain", "application/json"),
])
def test_native_uri_reader_rejects_errors_envelopes_and_wrong_headers(payload, content_type, expected):
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def do_GET(self):
            assert urllib.parse.urlsplit(self.path).path == "/status"
            assert urllib.parse.urlsplit(self.path).query == ""
            assert self.headers.get("Accept") == "application/json"
            body = json.dumps(payload).encode()
            self.send_response(200)
            self.send_header("Content-Type", content_type)
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        with pytest.raises(maintenance.MaintenanceError, match=expected):
            maintenance.rpc(f"http://127.0.0.1:{server.server_port}", "status")
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=3)
        assert not thread.is_alive()


def test_finding_20_reopened_rollback_preserves_latest_signature_and_rejects_regression(network):
    source, witness, target = recovery_homes(network)
    result = network.script("state_sync.sh", target, *trust_args(network, source, witness), "--startup-timeout", "2")
    assert result.returncode == 0, result.stdout + result.stderr
    network.stop(target)
    put_state(target, HEIGHT + 12, 6)
    advanced = (target / "data/priv_validator_state.json").read_bytes()
    result = network.script("state_sync.sh", target, "--rollback")
    assert result.returncode == 0, result.stdout + result.stderr
    before = manifest(target)
    result = network.script("state_sync.sh", target, "--rollback")
    assert result.returncode == 0, result.stdout + result.stderr
    assert manifest(target) == before
    assert (target / "data/priv_validator_state.json").read_bytes() == advanced
    # The original failed-home still proves that this height has been signed.
    put_state(target, HEIGHT, 3)
    regressed = manifest(target)
    result = network.script("state_sync.sh", target, "--rollback")
    assert result.returncode != 0 and "rolled-back signing state regressed" in result.stderr
    assert manifest(target) == regressed
    assert not network.is_running(target)


def test_finding_21_config_editor_retains_unrelated_multiline_values(tmp_path):
    path = tmp_path / "config.toml"
    original = ('moniker = """description\n[statesync]\ntrust-height = 99\n"""\n'
                '[p2p]\npersistent-peers = ""\n'
                '[statesync]\nenable = false\ntrust-height = 0\n')
    path.write_text(original)
    # TOML-looking text inside a multiline string must never be edited as keys.
    with pytest.raises(maintenance.MaintenanceError, match="unrelated settings"):
        maintenance.update_config(path, {"statesync": {"trust-height": HEIGHT}})
    assert path.read_text() == original
