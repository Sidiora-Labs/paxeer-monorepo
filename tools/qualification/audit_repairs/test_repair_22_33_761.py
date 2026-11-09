"""Offline qualification of real upgrade runtime paths with a durable HTTP peer.

Only Python's standard library, pytest and the declared bash parser are needed.
The committed peer is a protocol/storage fixture; native paxd is not simulated
as an upgrade implementation and no native upgrade coverage is claimed.
"""

import ast
import copy
import importlib.util
import json
import os
from pathlib import Path
import sqlite3
import subprocess
import sys
import time
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
SCRIPTS = ROOT / "tests/integration/upgrade_module/scripts"
RUNTIME = SCRIPTS / "upgrade_runtime.py"
SPEC = importlib.util.spec_from_file_location("upgrade_runtime_761", RUNTIME)
U = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(U)
SOURCE = RUNTIME.read_text()
TREE = ast.parse(SOURCE)
PEER = SCRIPTS / "fixtures/committed_peer.py"
ACTIONS = (
    "launch_cluster", "paxd_downgrade", "paxd_upgrade", "proposal_target_height",
    "proposal_vote", "verify_panic", "verify_running", "verify_upgrade_needed_log",
)


def function(name):
    matches = [node for node in ast.walk(TREE) if isinstance(node, ast.FunctionDef) and node.name == name]
    assert len(matches) == 1
    return ast.get_source_segment(SOURCE, matches[0])


def ordered(text, *facts):
    cursor = 0
    for fact in facts:
        cursor = text.index(fact, cursor) + len(fact)


@pytest.mark.parametrize("action", ACTIONS)
def test_shell_entry_points_parse_and_dispatch_real_runtime(action):
    path = SCRIPTS / f"{action}.sh"
    parsed = subprocess.run(["bash", "-n", str(path)], capture_output=True, text=True, timeout=5)
    assert parsed.returncode == 0, parsed.stderr
    assert path.read_text() == (
        '#!/bin/bash\n\nSCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"\n'
        f'exec python3 "$SCRIPT_DIR/upgrade_runtime.py" {action} "$@"\n'
    )
    assert f'"{action}"' in function("main")
    assert os.access(path, os.X_OK)


class Peer:
    """Owns a committed fixture process and its real SQLite and HTTP resources."""

    def __init__(self, path, *, stalled=False, identity="fixture-node-0"):
        self.path = path
        path.mkdir()
        self.store = path / "store.sqlite"
        self.address = path / "address"
        self.command = [sys.executable, "-I", "-S", str(PEER), "--store", str(self.store),
                        "--address", str(self.address), "--identity", identity]
        if stalled:
            self.command.append("--stalled")
        self.children = []
        self.logs = path / "logs"
        self.logs.mkdir()
        self.initial_log = self.logs / f"paxd-{os.environ.get('ID', '0')}.log"
        with self.initial_log.open("wb") as log:
            self.child = subprocess.Popen(
                self.command + ["start", "--chain-id", "pax", "--inv-check-period", "0"],
                stdout=log, stderr=log, env=dict(os.environ, UPGRADE_VERSION_LIST="v1.0.0"),
            )
        self.children.append(self.child)
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline:
            assert self.child.poll() is None, self.initial_log.read_text()
            if self.address.exists() and self.address.read_text():
                self.rpc = U.RPC(self.address.read_text(), timeout=0.2)
                try:
                    checkpoint = self.rpc.status("pax")
                    break
                except U.TRANSIENT:
                    pass
            time.sleep(0.01)
        else:
            raise AssertionError("loopback peer did not open")
        self.state = U.RestartState(path / "restart.json")
        self.state.data = {"process": U.process_record(self.child.pid), "version": "committed-peer-1",
                           "versions": "v1.0.0", "checkpoint": checkpoint}
        self.state.save()

    def reopen(self):
        self.state.close()
        self.state = U.RestartState(self.path / "restart.json")
        return U.ManagedNode(self.state, self.rpc, self.command, timeout=0.3,
                             interval=0.01, attempts=2)

    def close(self):
        if self.state.data and "process" in self.state.data:
            U.stop_process(self.state.data["process"], 3)
        for child in self.children:
            if child.poll() is None:
                child.terminate()
            child.wait(timeout=3)


@pytest.fixture
def peers(tmp_path):
    opened = []

    def open_peer(**kwargs):
        peer = Peer(tmp_path / f"peer-{len(opened)}", **kwargs)
        opened.append(peer)
        return peer

    yield open_peer
    for peer in reversed(opened):
        peer.close()


def test_upgrade_launch_unbounded(peers, tmp_path):
    nodes = [peers(identity=f"node-{i}") for i in range(4)]
    marker = tmp_path / "launch.complete"
    started = time.time_ns()
    child = subprocess.Popen([
        sys.executable, "-c",
        "import sys, time; from pathlib import Path; "
        "time.sleep(0.01); Path(sys.argv[1]).write_text('Done\\n' * 4); time.sleep(3)",
        str(marker),
    ])
    try:
        statuses = [lambda peer=peer: peer.rpc.status("pax") for peer in nodes]
        U.await_launch(child, marker, started, statuses, timeout=1, interval=0.01)
        assert marker.stat().st_mtime_ns >= started
        duplicate = [statuses[0]] * 4
        with pytest.raises(U.CheckFailed, match="deadline"):
            U.await_launch(child, marker, started, duplicate, timeout=0.05, interval=0.01)
        marker.write_text("Done\n" * 3)
        with pytest.raises(U.CheckFailed, match="deadline"):
            U.await_launch(child, marker, started, statuses, timeout=0.05, interval=0.01)
        os.utime(marker, ns=(started - 1_000_000, started - 1_000_000))
        with pytest.raises(U.CheckFailed, match="stale"):
            U.await_launch(child, marker, started, statuses, timeout=0.05, interval=0.01)
    finally:
        child.terminate()
        child.wait(timeout=3)
    failed = subprocess.Popen([sys.executable, "-c", "raise SystemExit(23)"])
    failed.wait(timeout=3)
    with pytest.raises(U.CheckFailed, match="exited 23"):
        U.await_launch(failed, marker, started, [], timeout=0.1, interval=0.01)
    launch = function("launch")
    ordered(launch, '"clean"', 'marker.unlink(missing_ok=True)', 'started = time.time_ns()',
            'child = subprocess.Popen', 'await_launch(', '"run-rpc-node-skipbuild"',
            '"tests/integration/startup/startup_test.yaml"')
    assert 'f"pax-node-{index}"' in launch
    assert 'timeout=timeout' in launch and 'os.killpg(child.pid, signal.SIGTERM)' in launch


@pytest.mark.parametrize("versions", ["v1.0.0,v2.0.0", "v1.0.0"])
def test_restart_recovery_retains_process_version_blocks_and_application_state(peers, versions):
    peer = peers()
    before = copy.deepcopy(peer.state.data)
    # Change real durable application state before stopping the component.
    with sqlite3.connect(peer.store) as db:
        db.execute("UPDATE application SET value='941' WHERE key='balance'")
    node = peer.reopen()
    assert node.state.data == before
    current = node.restart(versions, peer.logs)
    peer.children.append(node.child)
    peer.child.wait(timeout=3)
    assert not U.same_process(before["process"])
    assert node.state.data["process"]["pid"] != before["process"]["pid"]
    assert node.state.data["versions"] == versions
    assert U.process_record(node.child.pid)["versions"] == versions
    assert node.state.data["version"] == "committed-peer-1"
    assert current["height"] > before["checkpoint"]["height"]
    peer.rpc.retained(before["checkpoint"])
    assert peer.rpc.get("/application") == {"balance": "941"}
    assert Path(node.state.data["log"]).read_text().startswith("opened durable peer")
    assert any(path.read_text().startswith("opened durable peer")
               for path in peer.logs.glob("*.before-*"))
    node.state.close()
    recovered = U.RestartState(peer.path / "restart.json")
    assert recovered.data["checkpoint"] == current
    assert U.same_process(recovered.data["process"])
    peer.state = recovered
    restart = function("restart")
    ordered(restart, 'stop_process(previous', 'for attempt in range', 'log.open("xb")',
            'subprocess.Popen(', 'process_record(self.child.pid)', 'self.state.save()',
            'await_progress(', 'self.state.data["checkpoint"] = current')
    assert 'temporary.replace(stable)' in restart and 'stable.rename(' in restart
    assert 'self.child.wait(timeout=self.timeout)' in restart


def test_restart_failure_preserves_every_attempt_and_rejects_living_stall(peers):
    peer = peers(stalled=True)
    before = copy.deepcopy(peer.state.data["checkpoint"])
    node = peer.reopen()
    with pytest.raises(U.CheckFailed, match="attempts exhausted"):
        node.restart("v1.0.0,v2.0.0", peer.logs)
    peer.children.append(node.child)
    attempts = list(peer.logs.glob("*-attempt-*.log"))
    assert len(attempts) == 2
    assert all("opened durable peer" in log.read_text() for log in attempts)
    assert peer.state.data["checkpoint"] == before
    assert not U.same_process(peer.state.data["process"])
    assert not list(peer.logs.glob("*.link"))


def test_restart_rejects_lost_committed_state(peers):
    peer = peers()
    peer.state.data["checkpoint"]["hash"] = "F" * 64
    node = peer.reopen()
    with pytest.raises(U.CheckFailed, match="attempts exhausted"):
        node.restart("v1.0.0", peer.logs)
    peer.children.append(node.child)
    assert peer.state.data["checkpoint"]["hash"] == "F" * 64


def test_upgrade_target_stalled_sample(peers):
    peer = peers()
    start = peer.rpc.status("pax")["height"]
    target = U.sample_target(peer.rpc, 60, samples=2, window=0.05)
    assert target > start + 60
    stalled = peers(stalled=True)
    with pytest.raises(U.CheckFailed, match="stalled"):
        U.sample_target(stalled.rpc, 60, samples=2, window=0.01)
    for value in ("0", "-1", "1; exit 0", "1.5", "null", str(2**63)):
        with pytest.raises(U.CheckFailed):
            U.sample_target(peer.rpc, value, samples=2, window=0.01)
    for window in (0, -1):
        with pytest.raises(U.CheckFailed):
            U.sample_target(peer.rpc, 60, samples=2, window=window)
    assert "BLOCK_MS=400" not in SOURCE
    sample = function("sample_target")
    assert 'for _ in range(samples)' in sample
    assert 'seconds_ahead * 1_000_000_000 * blocks' in sample
    assert 'positive(target, "target height")' in sample


def test_upgrade_vote_rejection_success():
    accepted = {"code": 0, "txhash": "A" * 64}
    U.check_tx(accepted)
    for rejected in ({"code": 7, "raw_log": "unauthorized"}, {}, {"code": "0"},
                     {"code": False, "txhash": "A" * 64}, {"code": 0}):
        with pytest.raises(U.CheckFailed):
            U.check_tx(rejected)
    committed = {"proposal_id": "7", "voter": "pax1voter", "options": [
        {"option": "VOTE_OPTION_YES", "weight": "1.000000000000000000"}]}
    U.yes_vote(committed, 7, "pax1voter")
    U.yes_vote({"vote": committed}, 7, "pax1voter")
    U.yes_vote({"proposal_id": "7", "voter": "pax1voter", "option": "VOTE_OPTION_YES"}, 7, "pax1voter")
    for change in ({"proposal_id": "8"}, {"voter": "pax1other"}, {"options": []},
                   {"options": [{"option": "VOTE_OPTION_NO", "weight": "1"}]},
                   {"options": [{"option": "VOTE_OPTION_YES", "weight": "0.5"}]}):
        with pytest.raises(U.CheckFailed):
            U.yes_vote(dict(committed, **change), 7, "pax1voter")
    body = function("vote")
    ordered(body, '"keys", "show", "node_admin", "-a"', '"tx", "gov", "vote"',
            'check_tx(response)', '"query", "gov", "vote", str(proposal_id), voter',
            'yes_vote(committed, proposal_id, voter)', 'return')
    assert 'time.monotonic() + timeout' in body
    assert '_get_account_sequence' not in SOURCE


def panic_state():
    return {"plan": {"name": "v2.0.0", "height": 100, "info": ""},
            "checkpoint": {"height": 99}, "versions": "v1.0.0"}


def test_upgrade_panic_false_positive():
    state = panic_state()
    event = 'UPGRADE "v2.0.0" NEEDED at height: 100: test major release'
    U.panic_evidence(state, 100, event, state["plan"], False)
    for text, halt, alive in (("database failure", {}, False), ("", {}, False),
                              (event, state["plan"], True), (event, {}, False),
                              (event.replace("v2.0.0", "v3.0.0"), state["plan"], False),
                              (event.replace("100:", "1000:"), state["plan"], False)):
        with pytest.raises(U.CheckFailed):
            U.panic_evidence(state, 100, text, halt, alive)
    with pytest.raises(U.CheckFailed, match="target mismatch"):
        U.panic_evidence(state, 101, event, state["plan"], False)
    early = 'BINARY UPDATED BEFORE TRIGGER! UPGRADE "v2.0.0" - in binary but not executed on chain'
    state["checkpoint"]["height"] = 50
    state["versions"] += ",v2.0.0"
    U.panic_evidence(state, 100, early, {}, False)
    state["plan"]["info"] = '{"upgradeType":"minor"}'
    with pytest.raises(U.CheckFailed, match="minor"):
        U.panic_evidence(state, 100, early, {}, False)
    main = function("main")
    assert 'panic_evidence(state.data, args[0], text, halt, False)' in main
    assert '"stale halt file"' in main
    assert 'query_plan(binary)' in main


def test_upgrade_running_process_only(peers):
    peer = peers()
    record = peer.state.data["process"]
    checkpoint = peer.state.data["checkpoint"]
    current = U.await_progress(peer.rpc, record, "pax", checkpoint["node_id"],
                               1, checkpoint, interval=0.01)
    assert current["height"] > checkpoint["height"]
    for changed in (dict(record, versions="v8.0.0"), dict(record, ticks="0")):
        with pytest.raises(U.CheckFailed, match="identity changed"):
            U.await_progress(peer.rpc, changed, "pax", checkpoint["node_id"], 0.1, interval=0.01)
    for chain, identity in (("other-chain", checkpoint["node_id"]), ("pax", "other-node")):
        with pytest.raises(U.CheckFailed, match="identity mismatch"):
            U.await_progress(peer.rpc, record, chain, identity, 0.05, interval=0.01)
    stalled = peers(stalled=True)
    with pytest.raises(U.CheckFailed, match="deadline"):
        U.await_progress(stalled.rpc, stalled.state.data["process"], "pax", "fixture-node-0",
                         0.05, interval=0.01)
    main = function("main")
    assert 'run([binary, "tendermint", "show-node-id"])' in main
    assert 'checkpoint = rpc.status("pax", local_id)' in main
    assert 'record["versions"] == state.data["versions"]' in main
    assert 'run([binary, "version"]) == state.data["version"]' in main
    progress = function("await_progress")
    ordered(progress, 'same_process(record)', 'rpc.status(chain, node_id)', 'rpc.retained(checkpoint)',
            'current["height"] > first["height"]', 'return current')


def test_upgrade_log_fallback(tmp_path):
    for text in ('UPGRADE "v2.0.0" NEEDED at height: 100: details',
                 '"message":"UPGRADE \\"v2.0.0\\" NEEDED at height: 100: details"'):
        assert U.upgrade_event(text, "v2.0.0", 100)
    for text in ("", "stalled on target-1", 'UPGRADE "v2.0.0" NEEDED at height: 1000:',
                 'UPGRADE "v2X0X0" NEEDED at height: 100:', 'UPGRADE "v3.0.0" NEEDED at height: 100:'):
        assert not U.upgrade_event(text, "v2.0.0", 100)
    logs = tmp_path / "build/generated/logs"
    logs.mkdir(parents=True)
    log = logs / f"paxd-{os.environ.get('ID', '0')}.log"
    # Real shell -> Python -> exact log consumer, with no paxd available/required.
    log.write_text("stalled on target-1")
    command = ["bash", str(SCRIPTS / "verify_upgrade_needed_log.sh"), "100", "v2.0.0"]
    failed = subprocess.run(command, cwd=tmp_path, text=True, capture_output=True, timeout=5)
    assert failed.returncode == 1 and "PASS" not in failed.stdout
    assert "exact UPGRADE NEEDED event missing" in failed.stderr
    log.write_text('UPGRADE "v2.0.0" NEEDED at height: 100: details\n')
    passed = subprocess.run(command, cwd=tmp_path, text=True, capture_output=True, timeout=5)
    assert passed.returncode == 0 and passed.stdout == "PASS\n"


def test_record_maps_all_findings_to_executed_tests():
    record = tomllib.loads((ROOT / "tools/qualification/audit_repairs/repair_22_33_761.kvx").read_text())
    expected = {"upgrade-launch-unbounded", "downgrade-process-only", "upgrade-process-only",
                "upgrade-target-stalled-sample", "upgrade-vote-rejection-success",
                "upgrade-panic-false-positive", "upgrade-running-process-only", "upgrade-log-fallback"}
    assert set(record["finding"]) == expected
    declared = {node.name for node in ast.walk(ast.parse(Path(__file__).read_text()))
                if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")}
    for finding in record["finding"].values():
        assert finding["test"] in declared
    assert record["meta"]["native_upgrade_proven"] is False
    assert "not run" in record["meta"]["baseline"]
    assert record["wiring"]


def test_target_shell_consumes_committed_peer_rpc(peers, tmp_path):
    peer = peers()
    command = ["bash", str(SCRIPTS / "proposal_target_height.sh"), "60"]
    env = dict(os.environ, PAXD_RPC_URL=peer.address.read_text(),
               PAX_HOME=str(tmp_path / "selected-home"), PAXD_TARGET_SAMPLES="2",
               PAXD_TARGET_SAMPLE_WINDOW_SECONDS="0.05")
    before = peer.rpc.status("pax")["height"]
    result = subprocess.run(command, cwd=tmp_path, env=env, text=True,
                            capture_output=True, timeout=5)
    assert result.returncode == 0, result.stderr
    assert int(result.stdout) > before + 60
    stalled = peers(stalled=True)
    env["PAXD_RPC_URL"] = stalled.address.read_text()
    failed = subprocess.run(command, cwd=tmp_path, env=env, text=True,
                            capture_output=True, timeout=5)
    assert failed.returncode == 1
    assert "stalled" in failed.stderr and not failed.stdout
    native = function("main")
    assert 'os.environ.get("PAXD_BIN", "paxd")' in native
    assert 'state = RestartState(home / "data/upgrade-test-state.json")' in native
    assert '[binary, "--home", str(home)]' in native
    assert 'version_list(record["versions"])' in native
    assert '["--home", home]' in function("run")
