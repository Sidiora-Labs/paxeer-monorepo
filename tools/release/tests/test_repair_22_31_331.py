"""Real snapshot/rollback processes and the rehearsal producer/consumer contract.

The local RPC server serves protocol-shaped state, not a replacement for either
release component. These tests qualify release orchestration, not a paxd build.
"""

import base64
import contextlib
import hashlib
import http.server
import json
from pathlib import Path
import shutil
import sqlite3
import subprocess
import sys
import threading

import pytest

RELEASE = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(RELEASE))
import fleet_check as fleet
from rehearsal import pool_parity


def run(*args, success=True):
    result = subprocess.run(
        [str(arg) for arg in args], capture_output=True, text=True, timeout=30
    )
    if success:
        assert result.returncode == 0, result.stdout + result.stderr
    else:
        assert result.returncode != 0, result.stdout + result.stderr
    return result


def dump(path, value):
    path.write_text(json.dumps(value))


def signing_state(height, round_=0, step=3):
    return {"height": str(height), "round": round_, "step": step,
            "signature": base64.b64encode(bytes(64)).decode(), "signbytes": "0820"}


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


class RollbackHome:
    def __init__(self, root):
        self.root = root
        self.home = root / "validator"
        self.bin = root / "bin"
        self.snapshots = root / "snapshots"
        for directory in (self.home / "config", self.home / "data/committer.db",
                          self.bin, self.snapshots):
            directory.mkdir(parents=True)
        self.state = self.home / "data/priv_validator_state.json"
        dump(self.state, signing_state(20))
        self.store = self.home / "data/committer.db/history.sqlite"
        with sqlite3.connect(self.store) as db:
            db.execute("CREATE TABLE commits(height INTEGER PRIMARY KEY, hash TEXT NOT NULL)")
            db.execute("INSERT INTO commits VALUES(20, 'halt-commit')")
        self.app = self.home / "config/app.toml"
        self.app.write_text("halt-height = 20\n")
        (self.home / "config/config.toml").write_text(
            '[rpc]\nladdr = "tcp://127.0.0.1:36000"\n'
            '[p2p]\nladdr = "tcp://127.0.0.1:36001"\npex = false\n'
            'persistent-peers = ""\nexternal-address = ""\nbootstrap-peers = ""\nseeds = ""\n'
        )
        self.key = base64.b64encode(bytes(range(32))).decode()
        self.source_key = base64.b64encode(bytes(range(32, 64))).decode()
        self.genesis = self.home / "config/genesis.json"
        dump(self.genesis, {"chain_id": "hyperpax_125-1",
                            "validators": [{"pub_key": {"value": self.key}}]})
        dump(self.home / "config/priv_validator_key.json",
             {"pub_key": {"type": "tendermint/PubKeyEd25519", "value": self.key}})
        self.source = root / "source-genesis.json"
        dump(self.source, {"chain_id": "hyperpax_125-1", "app_state": {"staking": {
            "validators": [{"consensus_pubkey": {"key": self.source_key}}]}}})
        # Real ELF files exercise the release binary checksum/loader checks. They
        # are never used as substitute paxd implementations or launched as nodes.
        shutil.copy2(sys.executable, self.bin / "paxd.pre-r1")
        shutil.copy2(sys.executable, self.bin / "paxd")
        self.recovery = root / "recovery-paxd"
        shutil.copy2(sys.executable, self.recovery)
        self.env = root / "rollout.env"
        self.extra = {}
        self.write_env()
        run("bash", RELEASE / "snapshot.sh", "--env-file", self.env)
        dump(self.state, signing_state(31, 2))
        with sqlite3.connect(self.store) as db:
            db.execute("INSERT INTO commits VALUES(31, 'candidate-commit')")
        dump(self.home / "data/upgrade-info.json", {"name": "v7.0", "height": 21})

    def write_env(self):
        fields = {
            "RELEASE_ID": "r1", "BIN_DIR": self.bin, "SNAPSHOT_DIR": self.snapshots,
            "HALT_HEIGHT": 20, "VALIDATOR_HOMES": self.home,
            "VALIDATOR_UNITS": "validator.service", "FULLNODE_HOMES": "",
            "FULLNODE_UNITS": "", "RECOVERY_BIN": self.recovery,
            "RECOVERY_SHA256": digest(self.recovery), **self.extra,
        }
        self.env.write_text("".join(f'{key}="{value}"\n' for key, value in fields.items()))

    def invoke(self, *args, success=True):
        return run("bash", RELEASE / "rollback.sh", "--env-file", self.env,
                   "--local", "--service-manager", "foreground", *args, success=success)

    def commits(self):
        with sqlite3.connect(self.store) as db:
            return db.execute("SELECT * FROM commits ORDER BY height").fetchall()

    def isolate(self):
        genesis = json.loads(self.genesis.read_text())
        genesis["chain_id"] = "rehearsal_hyperpax_125-1"
        dump(self.genesis, genesis)
        self.extra.update(ROLLBACK_ISOLATED_FORK="true", ROLLBACK_SOURCE_GENESIS=self.source)
        self.write_env()


@pytest.mark.parametrize("live", [signing_state(31, 2), signing_state(20, 2), signing_state(20, 0, 3)])
def test_validator_signing_rewind_refused_before_any_move(tmp_path, live):
    node = RollbackHome(tmp_path)
    if live["height"] == "20" and live["round"] == 0:
        live["signbytes"] = "0821"
    dump(node.state, live)
    before = node.state.read_bytes(), node.store.read_bytes(), node.app.read_bytes()
    result = node.invoke("--execute", "--phase", "restore", success=False)
    assert "post-halt signing" in result.stderr
    assert "same-chain snapshot restore refused" in result.stderr
    assert before == (node.state.read_bytes(), node.store.read_bytes(), node.app.read_bytes())
    assert not (node.home / "data.v7.0.0-r1").exists()
    assert not (node.home / ".rollback-r1").exists()
    assert node.commits() == [(20, "halt-commit"), (31, "candidate-commit")]


def test_forward_recovery_reopens_journal_without_rewinding(tmp_path):
    node = RollbackHome(tmp_path)
    before = node.state.read_bytes(), node.store.read_bytes()
    inode = (node.home / "data").stat().st_ino
    node.invoke("--forward", "--execute", "--phase", "stop")
    record = node.home / ".forward-recovery-r1"
    pending = node.home / ".forward-recovery-r1.tmp"
    pending.mkdir()
    (pending / "material").write_text("interrupted unpublished journal")
    (node.bin / "paxd.new").write_bytes(b"interrupted executable copy")
    node.invoke("--forward", "--execute", "--phase", "prepare")
    node.invoke("--forward", "--execute", "--phase", "restore")
    assert not pending.exists()
    assert (record / "restored").is_file()
    assert (record / "priv_validator_state.json").read_bytes() == before[0]
    # The first rollback process has exited. Reopen recovery from its durable
    # journal, including a crash after staging but before a start receipt exists.
    (record / "restored").unlink()
    node.invoke("--forward", "--execute", "--phase", "restore")
    assert (record / "restored").is_file()
    assert before == (node.state.read_bytes(), node.store.read_bytes())
    assert (node.home / "data").stat().st_ino == inode
    assert node.commits() == [(20, "halt-commit"), (31, "candidate-commit")]
    assert (node.home / "data/upgrade-info.json").is_file()
    assert node.app.read_text() == "halt-height = 0\n"
    assert digest(node.bin / "paxd") == digest(node.recovery)
    assert not (node.home / "data.v7.0.0-r1").exists()
    dump(node.state, signing_state(30))
    result = node.invoke("--forward", "--execute", "--phase", "restore", success=False)
    assert "high-water mark regressed" in result.stderr


def test_isolated_fork_rewind_requires_fresh_keys_and_network(tmp_path):
    node = RollbackHome(tmp_path)
    node.isolate()
    genesis = json.loads(node.genesis.read_text())
    genesis["validators"][0]["pub_key"]["value"] = node.source_key
    dump(node.genesis, genesis)
    result = node.invoke("--execute", "--phase", "restore", success=False)
    assert "fresh consensus identities" in result.stderr
    assert node.commits()[-1][0] == 31
    genesis["validators"][0]["pub_key"]["value"] = node.key
    dump(node.genesis, genesis)
    config = node.home / "config/config.toml"
    isolated = config.read_text()
    config.write_text(isolated.replace("127.0.0.1:36001", "0.0.0.0:36001"))
    result = node.invoke("--execute", "--phase", "restore", success=False)
    assert "listeners must be loopback" in result.stderr
    assert node.commits()[-1][0] == 31
    config.write_text(isolated)
    node.invoke("--execute", "--phase", "restore")
    assert json.loads(node.state.read_text())["height"] == "20"
    assert node.commits() == [(20, "halt-commit")]
    retained = node.home / "data.v7.0.0-r1/committer.db/history.sqlite"
    with sqlite3.connect(retained) as db:
        assert db.execute("SELECT MAX(height) FROM commits").fetchone() == (31,)
    # Reopen the retained state with a new rollback process's readiness check.
    node.invoke("--readiness")
    assert not (node.home / "data/upgrade-info.json").exists()


def test_isolated_flag_cannot_authorize_same_chain_rewind(tmp_path):
    node = RollbackHome(tmp_path)
    node.extra.update(ROLLBACK_ISOLATED_FORK="true", ROLLBACK_SOURCE_GENESIS=node.source)
    node.write_env()
    result = node.invoke("--execute", "--phase", "restore", success=False)
    assert "different consensus chain identity" in result.stderr
    assert node.commits()[-1][0] == 31


@contextlib.contextmanager
def rpc_server():
    requests = []
    pool = "0x" + "11" * 20
    token = "0x" + "22" * 20

    class Handler(http.server.BaseHTTPRequestHandler):
        def log_message(self, *_):
            pass

        def do_POST(self):
            assert self.headers.get_content_type() == "application/json"
            body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            assert set(body) == {"jsonrpc", "id", "method", "params"}
            assert body["jsonrpc"] == "2.0" and body["id"] == 1
            assert body["method"] == "eth_call"
            call, block = body["params"]
            assert set(call) == {"to", "data"}
            assert block in ("0x14", "0x20")
            requests.append(body)
            address, data = call["to"].lower(), call["data"]
            if data == pool_parity.GET_POOL_COUNT:
                value = 0 if address == pool_parity.GENERATION_REGISTRIES[1].lower() else 1
                raw = fleet.word(value)
            elif data.startswith(pool_parity.GET_ALL_POOLS):
                assert data[10:] == (fleet.word(0) + fleet.word(100)).hex()
                raw = fleet.word(32) + fleet.word(1) + fleet.word(int(pool, 16))
            elif data == pool_parity.GET_RESERVES:
                assert address == pool
                raw = b"".join(fleet.word(v) for v in (100, 40, 50))
            elif data == pool_parity.TOKEN_ADDRESS:
                assert address == pool
                raw = fleet.word(int(token, 16))
            elif data.startswith(pool_parity.BALANCE_OF):
                assert data[10:] == fleet.word(int(pool, 16)).hex()
                raw = fleet.word(100)
            elif data in (fleet.call_data("totalMarkets()"), fleet.call_data("getMarketCount()")):
                raw = fleet.word(18)
            else:
                raise AssertionError(body)
            payload = json.dumps({"jsonrpc": "2.0", "id": 1, "result": "0x" + raw.hex()}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)

    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_port}", requests
    finally:
        server.shutdown()
        server.server_close()
        thread.join()


def rehearsal_setup(root, url):
    binary_dir = root / "bin"
    binary_dir.mkdir()
    shutil.copy2(sys.executable, binary_dir / "paxd")
    inventory = root / "inventory.json"
    dump(inventory, {"nodes": [{"role": "validator", "consensus_rpc": url, "evm_rpc": url}]})
    env_file = root / "rehearsal.env"
    env_file.write_text(
        f'CHAIN_ID=rehearsal_hyperpax_125-1\nHALT_HEIGHT=20\nBIN_DIR={binary_dir}\n'
        f'INVENTORY_FILE={inventory}\nRELEASE_SHA256={digest(binary_dir / "paxd")}\n'
        f'RECEIPT_DIR={root / "receipts"}\n'
    )
    record = root / "pools-at-halt.json"
    run(sys.executable, RELEASE / "rehearsal/pool_parity.py", "record",
        "--rpc", url, "--block", 20, "--out", record)
    handoff = root / "rehearsal-handoff.json"
    run("bash", "-euc", 'source "$1"; release=$2; rehearsal_env=$3; pools_file=$4; '
        'write_rehearsal_handoff "$5"', "handoff", RELEASE / "rehearsal/receipt.sh",
        RELEASE, env_file, record, handoff)
    return env_file, record, handoff


def test_fresh_rehearsal_driver_handoff_consumer_and_final_receipt(tmp_path):
    with rpc_server() as (url, requests):
        env_file, record, handoff = rehearsal_setup(tmp_path, url)
        env = fleet.read_env(env_file)
        final = tmp_path / "receipts/rehearsal.json"
        assert not final.exists()
        assert "POOL_COUNT" not in env
        internal = json.loads(handoff.read_text())
        assert internal["pools"] == 1
        assert "result" not in internal and "rollback_rehearsed" not in internal
        # Both public CLI parsing and the consumer used by run_phase admit the
        # unmodified producer handoff, with no successful prerequisite receipt.
        run(sys.executable, RELEASE / "fleet_check.py", "--env-file", env_file,
            "--phase", "markets", "--check", "--rehearsal-handoff", handoff)
        nodes, _, _, _, expected_pools = fleet.load_config(
            env, "markets", rehearsal_handoff=handoff
        )
        outcome = fleet.Outcome()
        nodes[0].height = "0x20"
        fleet.check_markets(nodes[0], json.loads((RELEASE / "fleet-views.json").read_text()),
                            expected_pools, outcome, "rehearsal")
        measured = outcome.finish()
        assert measured["result"] == "pass", outcome.errors
        assert measured["pools"] == 1 and measured["markets"] == 18
        assert not final.exists()
        with pytest.raises(fleet.CheckError, match="no rehearsal.json"):
            fleet.load_config(env, "post")
        # Publish with the same final producer called after the driver's rollback
        # leg. No success receipt was seeded to unblock the earlier consumer.
        run("bash", "-euc", 'source "$1"; receipt=$2; plan=v7.0; release_version=v7.0.0; nodes=1; '
            'precompiles_served=8; pools=$3; markets=$4; oracle_fresh=18; feed_keys=1; '
            'new_sha=$5; write_receipt pass', "receipt", RELEASE / "rehearsal/receipt.sh",
            final, measured["pools"], measured["markets"], env["RELEASE_SHA256"])
        published = json.loads(final.read_text())
        fleet.validate_receipt(published)
        assert published["result"] == "pass" and published["rollback_rehearsed"] is True
        assert fleet.load_config(env, "post")[-1] == measured["pools"]
        assert len(requests) >= 9
        assert json.loads(record.read_text())["block"] == 20


def test_handoff_cannot_replace_production_rollback_evidence(tmp_path):
    with rpc_server() as (url, _):
        env_file, _, handoff = rehearsal_setup(tmp_path, url)
        env = fleet.read_env(env_file)
        for phase in ("full", "post"):
            with pytest.raises(fleet.CheckError, match="HANDOFF_MARKETS_ONLY"):
                fleet.load_config(env, phase, rehearsal_handoff=handoff)
            with pytest.raises(fleet.CheckError, match="PRODUCTION_REQUIRES_ROLLBACK_RECEIPT"):
                fleet.load_config(env, phase, no_rehearsal=True)
        with pytest.raises(fleet.CheckError, match="RECEIPT_KEYS"):
            fleet.load_config(env, "post", rehearsal_path=handoff)
        env["CHAIN_ID"] = "hyperpax_125-1"
        with pytest.raises(fleet.CheckError, match="HANDOFF_CHAIN_MISMATCH"):
            fleet.load_config(env, "markets", rehearsal_handoff=handoff)


def test_handoff_binds_candidate_inventory_and_pool_height(tmp_path):
    with rpc_server() as (url, _):
        env_file, record, handoff = rehearsal_setup(tmp_path, url)
        env = fleet.read_env(env_file)
        binary = Path(env["BIN_DIR"]) / "paxd"
        with binary.open("ab") as stream:
            stream.write(b"candidate changed")
        with pytest.raises(fleet.CheckError, match="HANDOFF_BINARY_MISMATCH"):
            fleet.load_config(env, "markets", rehearsal_handoff=handoff)
        shutil.copy2(sys.executable, binary)
        inventory = Path(env["INVENTORY_FILE"])
        inventory.write_text(inventory.read_text() + "\n")
        with pytest.raises(fleet.CheckError, match="HANDOFF_INVENTORY_MISMATCH"):
            fleet.load_config(env, "markets", rehearsal_handoff=handoff)
        book = json.loads(record.read_text())
        book["block"] = 19
        dump(record, book)
        with pytest.raises(fleet.CheckError, match="HANDOFF_POOL_HEIGHT"):
            fleet.write_rehearsal_handoff(env_file, record, handoff)
