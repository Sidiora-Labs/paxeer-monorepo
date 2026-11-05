"""Offline source and loopback-HTTP proofs for the production RPC startup scripts."""

from contextlib import contextmanager
from copy import deepcopy
import importlib.util
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
from pathlib import Path
import re
import shlex
import subprocess
import threading
import time
import tomllib
import urllib.error

import pytest


ROOT = Path(__file__).resolve().parents[3]
SCRIPTS = ROOT / "docker/rpcnode/scripts"
SPEC = importlib.util.spec_from_file_location("rpcnode_state_sync", SCRIPTS / "state_sync.py")
STATE_SYNC = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(STATE_SYNC)
HEIGHT = "12500"
BLOCK_HASH = "AB" * 32
PEER_A = "1" * 40
PEER_B = "2" * 40
# Recorded wire shapes: consensus/rpc/coretypes/responses.go ResultBlock,
# ResultNetInfo and Peer, carried in the JSON-RPC result envelope.
BLOCK_TRANSCRIPT = {
    "jsonrpc": "2.0",
    "id": -1,
    "result": {
        "block_id": {"hash": BLOCK_HASH},
        "block": {"header": {"chain_id": "pax", "height": HEIGHT}},
    },
}
PEERS_TRANSCRIPT = {
    "jsonrpc": "2.0",
    "id": -1,
    "result": {
        "listening": True,
        "listeners": [],
        "n_peers": "1",
        "peers": [{"node_id": PEER_A, "url": f"mconn://{PEER_A}@127.0.0.1:26656"}],
    },
}


def wire_response(payload, status=200, headers=None, mode="normal"):
    data = payload if isinstance(payload, bytes) else json.dumps(payload).encode()
    return data, status, headers or {}, mode


@contextmanager
def provider(routes=None):
    routes = routes if routes is not None else {
        f"/block?height={HEIGHT}": wire_response(BLOCK_TRANSCRIPT),
        "/net_info": wire_response(PEERS_TRANSCRIPT),
    }
    requests = []

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def do_GET(self):
            requests.append(self.path)
            data, status, headers, mode = routes.get(
                self.path, wire_response(b"unexpected route", status=404)
            )
            if mode == "stall":
                time.sleep(STATE_SYNC.IO_TIMEOUT_SECONDS + 0.5)
            self.send_response(status)
            self.send_header("Content-Type", headers.get("Content-Type", "application/json"))
            if mode != "unbounded":
                self.send_header("Content-Length", headers.get("Content-Length", str(len(data))))
            for key, value in headers.items():
                if key not in {"Content-Type", "Content-Length"}:
                    self.send_header(key, value)
            self.end_headers()
            try:
                if mode == "trickle":
                    for byte in data:
                        self.wfile.write(bytes([byte]))
                        self.wfile.flush()
                        time.sleep(0.1)
                else:
                    self.wfile.write(data)
            except (BrokenPipeError, ConnectionResetError):
                pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    server.daemon_threads = True
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_port}", requests
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=2)
        assert not thread.is_alive()


def inputs(tmp_path, origins):
    genesis = tmp_path / "genesis.json"
    genesis.write_text(json.dumps({"chain_id": "pax"}))
    config = ROOT / "docker/rpcnode/config/config.toml"
    output = tmp_path / "config.toml"
    output.write_bytes(config.read_bytes())
    peers = tmp_path / "PEERS"
    peers.write_text("previous peer\n")
    env = {
        "STATE_SYNC_RPC_SERVERS": ",".join(origins),
        "STATE_SYNC_TRUST_HEIGHT": HEIGHT,
        "STATE_SYNC_TRUST_HASH": BLOCK_HASH.lower(),
    }
    return config, genesis, output, peers, env


def shell_commands(name):
    source = (SCRIPTS / name).read_text()
    return source, [line.strip() for line in source.splitlines() if line.strip() and not line.startswith("#")]


@pytest.mark.parametrize("name", ["deploy.sh", "step1_configure_init.sh", "step2_start_pax.sh"])
def test_scripts_parse_and_fail_fast(name):
    result = subprocess.run(
        ["bash", "-n", str(SCRIPTS / name)], capture_output=True, text=True, timeout=5
    )
    assert result.returncode == 0, result.stderr
    source, commands = shell_commands(name)
    assert commands[0] == "set -eu"
    assert "|| true" not in source


def test_rpc_deploy_failure_prerequisites_and_supervision():
    source, commands = shell_commands("deploy.sh")
    assert "umask 077" in commands
    assert 'for script in /usr/bin/configure_init.sh /usr/bin/start_pax.sh; do' in source
    assert '[ -x "$script" ] ||' in source and 'exit 1;' in source
    assert '[ -r /usr/bin/build.sh ] ||' in source
    assert 'sh -e /usr/bin/build.sh' in commands
    assert '[ -x build/paxd ] ||' in source
    build = commands.index("sh -e /usr/bin/build.sh")
    copy = commands.index('cp build/paxd "$GOBIN"/')
    configure = commands.index("/usr/bin/configure_init.sh")
    assert build < copy < configure < len(commands) - 1
    assert commands[-1] == "exec /usr/bin/start_pax.sh"
    assert "tail" not in source and "/dev/null" not in source
    assert not re.search(r"(?<!&)\s&\s*$", source, re.M)


def test_rpc_background_start_foreground_node_and_no_completion_marker():
    source, commands = shell_commands("step2_start_pax.sh")
    assert 'command -v paxd >/dev/null 2>&1 ||' in source
    assert 'for file in app.toml config.toml genesis.json; do' in source
    assert '[ -s "$HOME/.pax/config/$file" ] && [ -r "$HOME/.pax/config/$file" ] ||' in source
    assert "exit 1" in source
    assert shlex.split(commands[-1]) == ["exec", "paxd", "start", "--chain-id", "pax"]
    assert not re.search(r"(?<!&)\s&\s*$", source, re.M)
    assert "rpc-launch.complete" not in source
    assert "rpc-node.log" not in source


def test_state_sync_shell_stages_before_any_node_mutation():
    source, _ = shell_commands("step1_configure_init.sh")
    assert 'command -v "$command" >/dev/null 2>&1 ||' in source
    assert '[ -s "$file" ] && [ -r "$file" ] ||' in source
    assert 'INIT_DIR=$(mktemp -d build/generated/rpc-init.XXXXXX)' in source
    assert 'trap \'rm -rf "$INIT_DIR"\' EXIT' in source
    assert 'trap \'exit 1\' HUP INT TERM' in source
    validate = source.index("python3 docker/rpcnode/scripts/state_sync.py")
    initialize = source.index('paxd init --chain-id pax "$MONIKER"')
    copy = source.index('cp docker/rpcnode/config/app.toml')
    assert validate < initialize < copy
    assert '--config docker/rpcnode/config/config.toml' in source
    assert '--genesis build/generated/genesis.json' in source
    assert '--output "$INIT_DIR/config.toml" --peers "$INIT_DIR/PEERS"' in source
    assert 'cp "$INIT_DIR/config.toml" "$HOME/.pax/config/config.toml"' in source
    assert 'mv "$INIT_DIR/PEERS" build/generated/PEERS' in source
    assert not re.search(r"curl|STATE_SYNC_RPC=|>> build/generated/PEERS|LATEST_HEIGHT", source)
    assert "GIGA_STORAGE=${GIGA_STORAGE:-false}" in source
    assert "GIGA_FLATKV_ONLY=${GIGA_FLATKV_ONLY:-false}" in source
    assert "RECEIPT_BACKEND=${RECEIPT_BACKEND:-}" in source


def test_state_sync_real_http_to_toml_and_atomic_peer_replacement(tmp_path):
    with provider() as (first, first_requests), provider() as (second, second_requests):
        second = second.replace("127.0.0.1", "localhost")
        args = inputs(tmp_path, [first, second])
        config, genesis, output, peers, env = args
        original = tomllib.loads(config.read_text())
        old_inode = peers.stat().st_ino
        STATE_SYNC.configure(*args)
        configured = tomllib.loads(output.read_text())
        assert configured["statesync"] == {
            **original["statesync"],
            "enable": True,
            "rpc-servers": f"{first},{second}",
            "trust-height": int(HEIGHT),
            "trust-hash": BLOCK_HASH,
        }
        original["p2p"]["persistent-peers"] = f"{PEER_A}@127.0.0.1:26656"
        original["statesync"] = configured["statesync"]
        assert configured == original
        assert peers.read_text() == f"{PEER_A}@127.0.0.1:26656\n"
        assert peers.stat().st_ino != old_inode
        assert peers.stat().st_mode & 0o777 == 0o600
        assert first_requests == second_requests == [f"/block?height={HEIGHT}", "/net_info"]
    changed = deepcopy(PEERS_TRANSCRIPT)
    changed["result"]["peers"] = [
        {"node_id": PEER_B, "url": f"mconn://{PEER_B}@[::1]:26656"}
    ]
    routes = {
        f"/block?height={HEIGHT}": wire_response(BLOCK_TRANSCRIPT),
        "/net_info": wire_response(changed),
    }
    with provider(routes) as (first, _), provider(routes) as (second, _):
        env["STATE_SYNC_RPC_SERVERS"] = f"{first},{second.replace('127.0.0.1', 'localhost')}"
        STATE_SYNC.configure(config, genesis, output, peers, env)
    assert peers.read_text() == f"{PEER_B}@[::1]:26656\n"
    assert tomllib.loads(output.read_text())["p2p"]["persistent-peers"] == f"{PEER_B}@[::1]:26656"
    assert not list(tmp_path.glob(".*"))


@pytest.mark.parametrize("setting,value", [
    ("STATE_SYNC_RPC_SERVERS", ""),
    ("STATE_SYNC_RPC_SERVERS", "https://one.example"),
    ("STATE_SYNC_RPC_SERVERS", "https://one.example,https://ONE.example.:443"),
    ("STATE_SYNC_RPC_SERVERS", "http://127.0.0.1:1,http://[::ffff:127.0.0.1]:2"),
    ("STATE_SYNC_RPC_SERVERS", "file:///etc/passwd,https://two.example"),
    ("STATE_SYNC_RPC_SERVERS", "https://user:password@one.example,https://two.example"),
    ("STATE_SYNC_RPC_SERVERS", "https://one.example/path,https://two.example"),
    ("STATE_SYNC_RPC_SERVERS", "https://one.example?x=1,https://two.example"),
    ("STATE_SYNC_RPC_SERVERS", "https://one.example:0,https://two.example"),
    ("STATE_SYNC_RPC_SERVERS", "https://one.example,https://two.example,https://three.example"),
    ("STATE_SYNC_TRUST_HEIGHT", ""),
    ("STATE_SYNC_TRUST_HEIGHT", "0"),
    ("STATE_SYNC_TRUST_HEIGHT", "-1"),
    ("STATE_SYNC_TRUST_HEIGHT", "01"),
    ("STATE_SYNC_TRUST_HEIGHT", str(2**63)),
    ("STATE_SYNC_TRUST_HASH", "null"),
    ("STATE_SYNC_TRUST_HASH", "G" * 64),
    ("STATE_SYNC_TRUST_HASH", '"; enable = true'),
])
def test_operator_inputs_rejected_before_publication(tmp_path, setting, value):
    args = inputs(tmp_path, ["https://one.example", "https://two.example"])
    args[-1][setting] = value
    before = (args[2].read_bytes(), args[3].read_bytes())
    with pytest.raises(ValueError):
        STATE_SYNC.configure(*args)
    assert (args[2].read_bytes(), args[3].read_bytes()) == before


@pytest.mark.parametrize("failure", ["chain", "height", "hash", "missing", "null", "rpc-error"])
def test_disagreeing_provider_cannot_publish_checkpoint(tmp_path, failure):
    bad = deepcopy(BLOCK_TRANSCRIPT)
    if failure == "chain":
        bad["result"]["block"]["header"]["chain_id"] = "another-chain"
    elif failure == "height":
        bad["result"]["block"]["header"]["height"] = "12501"
    elif failure == "hash":
        bad["result"]["block_id"]["hash"] = "CD" * 32
    elif failure == "missing":
        del bad["result"]["block_id"]
    elif failure == "null":
        bad["result"] = None
    else:
        bad["error"] = {"code": -32603, "message": "unavailable"}
    routes = {f"/block?height={HEIGHT}": wire_response(bad)}
    with provider() as (first, first_requests), provider(routes) as (second, second_requests):
        args = inputs(tmp_path, [first, second.replace("127.0.0.1", "localhost")])
        before = (args[2].read_bytes(), args[3].read_bytes())
        with pytest.raises(ValueError):
            STATE_SYNC.configure(*args)
        assert (args[2].read_bytes(), args[3].read_bytes()) == before
        assert first_requests == second_requests == [f"/block?height={HEIGHT}"]


@pytest.mark.parametrize("peer", [
    {"node_id": PEER_A, "url": f"mconn://{PEER_B}@127.0.0.1:26656"},
    {"node_id": PEER_A, "url": f"http://{PEER_A}@127.0.0.1:26656"},
    {"node_id": PEER_A, "url": f"mconn://{PEER_A}@0.0.0.0:26656"},
    {"node_id": PEER_A, "url": f"mconn://{PEER_A}@224.0.0.1:26656"},
    {"node_id": PEER_A, "url": f"mconn://{PEER_A}@127.0.0.1:0"},
    {"node_id": PEER_A, "url": f"mconn://{PEER_A}@127.0.0.1:26656/path"},
    {"node_id": PEER_A, "url": f"mconn://{PEER_A}@127.0.0.1:26656?bad=1"},
    {"node_id": PEER_A, "url": f"mconn://{PEER_A}@bad\"host:26656"},
    {"node_id": PEER_A, "url": None},
    "invalid peer",
])
def test_invalid_discovery_leaves_both_files_unchanged(tmp_path, peer):
    bad = deepcopy(PEERS_TRANSCRIPT)
    bad["result"]["peers"] = [peer]
    routes = {
        f"/block?height={HEIGHT}": wire_response(BLOCK_TRANSCRIPT),
        "/net_info": wire_response(bad),
    }
    with provider() as (first, _), provider(routes) as (second, _):
        args = inputs(tmp_path, [first, second.replace("127.0.0.1", "localhost")])
        before = (args[2].read_bytes(), args[3].read_bytes())
        with pytest.raises(ValueError):
            STATE_SYNC.configure(*args)
        assert (args[2].read_bytes(), args[3].read_bytes()) == before
        assert not list(tmp_path.glob(".*"))


@pytest.mark.parametrize("peers", [None, [], [{}] * (STATE_SYNC.MAX_PEERS + 1)])
def test_discovery_requires_bounded_nonempty_peers(peers):
    with pytest.raises(ValueError):
        STATE_SYNC.peer_addresses({"peers": peers})


@pytest.mark.parametrize("response", [
    wire_response(b"not JSON"),
    wire_response({"result": {}}, status=500),
    wire_response({"result": {}}, status=204),
    wire_response({"result": {}}, status=302, headers={"Location": "http://unreachable.invalid/"}),
    wire_response({"result": {}}, headers={"Content-Type": "text/html"}),
    wire_response(b"x", headers={"Content-Length": str(STATE_SYNC.MAX_RESPONSE_BYTES + 1)}),
    wire_response(b"x" * (STATE_SYNC.MAX_RESPONSE_BYTES + 1), mode="unbounded"),
    wire_response(b"{}", headers={"Content-Length": "20"}),
    wire_response({"result": {}}, mode="stall"),
])
def test_http_errors_bounds_and_redirects_are_rejected(response):
    with provider({"/net_info": response}) as (origin, requests):
        with pytest.raises((ValueError, OSError, urllib.error.URLError)):
            STATE_SYNC.fetch_result(origin, "/net_info")
        assert requests == ["/net_info"]


def test_http_trickle_cannot_extend_deadline():
    payload = b" " * 200 + b'{"result":{}}'
    start = time.monotonic()
    with provider({"/net_info": wire_response(payload, mode="trickle")}) as (origin, _):
        with pytest.raises(ValueError, match="deadline"):
            STATE_SYNC.fetch_result(origin, "/net_info")
    assert time.monotonic() - start < STATE_SYNC.REQUEST_DEADLINE_SECONDS + STATE_SYNC.IO_TIMEOUT_SECONDS


@pytest.mark.parametrize("kind", ["missing", "duplicate"])
def test_invalid_config_template_cannot_publish(tmp_path, kind):
    with provider() as (first, _), provider() as (second, _):
        config, genesis, output, peers, env = inputs(
            tmp_path, [first, second.replace("127.0.0.1", "localhost")]
        )
        template = tmp_path / "template.toml"
        source = config.read_text()
        if kind == "missing":
            source = source.replace("trust-height = 0", "")
        else:
            source = source.replace("trust-height = 0", "trust-height = 0\ntrust-height = 0")
        template.write_text(source)
        before = (output.read_bytes(), peers.read_bytes())
        with pytest.raises(ValueError, match=kind):
            STATE_SYNC.configure(template, genesis, output, peers, env)
        assert (output.read_bytes(), peers.read_bytes()) == before


def test_genesis_chain_must_match_node_before_discovery(tmp_path):
    args = inputs(tmp_path, ["https://one.example", "https://two.example"])
    args[1].write_text('{"chain_id":"another-chain"}')
    before = (args[2].read_bytes(), args[3].read_bytes())
    with pytest.raises(ValueError, match="genesis chain identity"):
        STATE_SYNC.configure(*args)
    assert (args[2].read_bytes(), args[3].read_bytes()) == before


def test_repair_record_maps_findings_and_evidence():
    record = tomllib.loads(
        (ROOT / "tools/qualification/audit_repairs/repair_22_33_771.kvx").read_text()
    )
    expected = {
        "rpc-deploy-failure": "test_rpc_deploy_failure_prerequisites_and_supervision",
        "rpc-state-sync-trust": "test_state_sync_real_http_to_toml_and_atomic_peer_replacement",
        "rpc-background-start": "test_rpc_background_start_foreground_node_and_no_completion_marker",
    }
    assert set(record["finding"]) == set(expected)
    for finding, test in expected.items():
        assert test in record["finding"][finding]["tests"]
        assert callable(globals()[test])
    task = record["task"]["22"]["33"]["771"]
    assert task["base_revision"] == "76c050242a1edcad6212aa52fac15670ecb5612d"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_771.py"
    assert "No pre-change verification" in task["baseline"]
    assert task["review_only"]
