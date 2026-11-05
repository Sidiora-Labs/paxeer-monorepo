"""Offline source and real filesystem/loopback protocol proofs for localnode repairs.

No Pax build, native daemon, credentials, live endpoint or toolchain is required.
Bash is invoked only for the explicitly required shell syntax parsing.
"""

import ast
from contextlib import contextmanager
import importlib.util
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
from pathlib import Path
import shlex
import stat
import subprocess
import threading
import time
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
SCRIPTS = ROOT / "docker/localnode/scripts"
SPEC = importlib.util.spec_from_file_location("localnode_runtime", SCRIPTS / "localnode_runtime.py")
RUNTIME = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(RUNTIME)
RECORD = Path(__file__).with_name("repair_22_33_801.kvx")


def source(name):
    return (SCRIPTS / name).read_text()


def function_source(name):
    text = source("localnode_runtime.py")
    matches = [node for node in ast.walk(ast.parse(text))
               if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)) and node.name == name]
    assert len(matches) == 1
    return ast.get_source_segment(text, matches[0])


def environment(node=0, size=1, timeout=3, run_id="qualification-run-801"):
    return {"LOCALNODE_DISPOSABLE": "1", "LOCALNODE_RUN_ID": run_id,
            "ID": str(node), "CLUSTER_SIZE": str(size),
            "LOCALNODE_STARTUP_TIMEOUT": str(timeout)}


def joined(root, node=0, size=1):
    run = RUNTIME.Run(root, environment(node, size))
    run.join()
    return run


@pytest.mark.parametrize("name", [
    "deploy.sh", "localnode_common.sh", "step0_build.sh", "step1_configure_init.sh",
    "step2_genesis.sh", "step3_add_validator_to_genesis.sh", "step4_config_override.sh",
    "step5_start_pax.sh",
])
def test_shell_sources_parse(name):
    result = subprocess.run(["bash", "-n", str(SCRIPTS / name)], capture_output=True, text=True)
    assert result.returncode == 0, result.stderr


def test_finding7_real_coordinator_late_join_and_reopen(tmp_path):
    coordinator = joined(tmp_path, 0, 2)
    coordinator.complete_init()
    before = (coordinator.directory / "init.0.complete").read_bytes()
    late_node = joined(tmp_path, 1, 2)
    assert late_node.directory == coordinator.directory
    assert (late_node.directory / "init.0.complete").read_bytes() == before
    late_node.complete_init()
    coordinator.wait_init()
    # Reopen the production consumer and recover the producer's actual persisted result.
    del late_node
    reopened = RUNTIME.Run(tmp_path, environment(1, 2))
    reopened.env["PAX_RUN_DIR"] = str(reopened.directory)
    reopened.validate_stage()
    reopened.wait_init()
    assert json.loads((reopened.directory / "init.0.complete").read_text()) == {"node": 0}
    with pytest.raises(FileExistsError):
        RUNTIME.Run(tmp_path, environment(0, 2)).join()
    assert (coordinator.directory / "init.0.complete").read_bytes() == before
    assert stat.S_IMODE((coordinator.directory / "exported_keys").stat().st_mode) == 0o700


def test_finding7_atomic_publication_and_failure_transport(tmp_path):
    runs = [joined(tmp_path, node, 3) for node in range(3)]
    path = runs[0].directory / "atomic.complete"
    runs[0].publish(path.name, {"sequence": 0, "payload": "x" * 10000})
    finished = threading.Event()
    errors = []
    observations = []

    def read_transport():
        try:
            while not finished.is_set():
                document = json.loads(path.read_text())
                assert document["payload"] == "x" * 10000
                observations.append(document["sequence"])
        except BaseException as error:
            errors.append(error)

    reader = threading.Thread(target=read_transport)
    reader.start()
    try:
        for sequence in range(1, 30):
            runs[0].publish(path.name, {"sequence": sequence, "payload": "x" * 10000})
    finally:
        finished.set()
        reader.join(timeout=2)
    assert not reader.is_alive()
    assert not errors
    assert observations
    assert json.loads(path.read_text())["sequence"] == 29
    runs[1].stage = "init"
    runs[1].fail("initialization exited 23")
    for consumer in runs:
        with pytest.raises(RUNTIME.StageFailure, match="init.*initialization exited 23"):
            consumer.wait(lambda: False)
    assert not list(runs[0].directory.glob(".failed.*"))


def test_finding7_missing_coordinator_has_bounded_deadline(tmp_path):
    run = RUNTIME.Run(tmp_path, environment(1, 2, timeout=0.08))
    start = time.monotonic()
    with pytest.raises(RUNTIME.StageFailure, match="deadline"):
        run.join()
    assert time.monotonic() - start < 1
    assert not run.directory.exists()


def test_finding7_run_and_context_mismatch_refused(tmp_path):
    coordinator = joined(tmp_path, 0, 2)
    with pytest.raises(RUNTIME.StageFailure, match="context mismatch"):
        RUNTIME.Run(tmp_path, environment(1, 3)).join()
    assert not (coordinator.directory / "node_1").exists()
    assert "rm -rf" not in source("deploy.sh")
    deploy = shlex.split(source("deploy.sh").splitlines()[-1])
    assert deploy == ["exec", "python3", "docker/localnode/scripts/localnode_runtime.py", "deploy"]
    implementation = function_source("deploy")
    assert 'run.execute("build", "step0_build.sh")' in implementation
    assert 'run.execute("init", "step1_configure_init.sh")' in implementation
    assert 'run.execute("genesis", "step2_genesis.sh")' in implementation
    assert 'run.execute("config", "step4_config_override.sh")' in implementation
    assert '"step5_start_pax.sh"' in implementation
    assert "run.wait(all_launched)" in implementation
    assert implementation.index("run.wait(all_launched)") < implementation.index("Nodes started successfully")
    execute = function_source("execute")
    assert "self.check()" in execute and "start_new_session=True" in execute
    assert "stop_process(process)" in execute


def test_finding8_build_guard_commands_and_atomic_snapshot(tmp_path):
    script = source("step0_build.sh")
    assert "set -euo pipefail" in script
    assert 'trap \'rc=$?' in script
    assert 'localnode_runtime.py fail build "$rc"' in script
    clean = script.index("make -f chain.mk clean")
    absent = script.index("[[ ! -e build/paxd ]]")
    build = script.index("make -f chain.mk build-linux")
    regular = script.index("[[ -f build/paxd && -x build/paxd && ! -L build/paxd ]]")
    version = script.index("build/paxd version")
    published = script.index("localnode_runtime.py build-complete")
    assert clean < absent < build < regular < version < published
    assert 'make -f chain.mk build-linux BUILD_TAGS="mock_balances"' in script
    assert "echo \"DONE\"" not in script
    assert 'with run.build_lock():' in function_source("deploy")
    run = joined(tmp_path)
    with run.build_lock():
        with pytest.raises(FileExistsError):
            with run.build_lock():
                pytest.fail("a second build acquired the exclusive lock")
    assert not (tmp_path / "build/localnode-build.lock").exists()
    with pytest.raises(RUNTIME.StageFailure, match="regular executable"):
        run.build_complete()
    assert not (run.directory / "build.complete").exists()
    code = function_source("build_complete")
    assert code.index("source.is_symlink()") < code.index("shutil.copyfile")
    assert code.index("shutil.copyfile") < code.index("os.replace") < code.index('self.publish("build.complete"')
    assert 'self.node != 0' in code


@pytest.mark.parametrize("override", [
    {"LOCALNODE_DISPOSABLE": "0"}, {"LOCALNODE_DISPOSABLE": ""},
    {"LOCALNODE_RUN_ID": "../operator-home"}, {"LOCALNODE_RUN_ID": ""},
    {"ID": "-1"}, {"CLUSTER_SIZE": "0"}, {"LOCALNODE_STARTUP_TIMEOUT": "nan"},
])
def test_finding9_disposable_context_required(tmp_path, override):
    env = environment()
    env.update(override)
    with pytest.raises((RUNTIME.StageFailure, ValueError)):
        RUNTIME.Run(tmp_path, env)
    assert list(tmp_path.iterdir()) == []


@pytest.mark.parametrize("fail", [False, True])
def test_finding9_private_home_checked_cleanup(tmp_path, fail):
    run = joined(tmp_path)
    operator = tmp_path / "operator-home"
    operator.mkdir()
    sentinel = operator / "operator-key"
    sentinel.write_text("untouched operator state")
    private_path = None

    def use_private_home():
        nonlocal private_path
        with RUNTIME.private_home(run) as private:
            private_path = private
            assert stat.S_IMODE(private.stat().st_mode) == 0o700
            assert run.env["PAX_HOME"] == str(private / ".pax")
            assert run.env["PAX_PRIVATE_DIR"] == str(private)
            (private / "test_accounts").mkdir(mode=0o700)
            (private / "test_accounts" / "seed.txt").write_text("disposable inventory")
            if fail:
                raise RUNTIME.StageFailure("init failure")

    if fail:
        with pytest.raises(RUNTIME.StageFailure, match="init failure"):
            use_private_home()
    else:
        use_private_home()
    assert private_path is not None and not private_path.exists()
    assert sentinel.read_text() == "untouched operator state"
    assert list((run.directory / "exported_keys").iterdir()) == []


def test_finding9_no_plaintext_export_and_real_consumers_use_private_home():
    init = source("step1_configure_init.sh")
    assert 'source docker/localnode/scripts/localnode_common.sh' in init
    assert 'mkdir -m 700 -p "$NODE_DIR/gentx" "$SHARED_DIR/exported_keys"' in init
    assert 'chmod 700 "$SHARED_DIR/exported_keys"' in init
    assert "keys export" not in init and "PRIV_KEY" not in init
    assert '--accounts "$PAX_PRIVATE_DIR/test_accounts"' in init
    assert 'paxd init "$MONIKER" --chain-id pax' in init
    common = source("localnode_common.sh")
    assert 'command "$PAXD_BIN" --home "$PAX_HOME" "$@"' in common
    assert common.index("localnode_runtime.py check-stage") < common.index("paxd()")
    for name in ("step1_configure_init.sh", "step2_genesis.sh",
                 "step3_add_validator_to_genesis.sh", "step4_config_override.sh"):
        script = source(name)
        assert "source docker/localnode/scripts/localnode_common.sh" in script
        assert "~/.pax" not in script and "$HOME/.pax" not in script
        # Historical cleanup paths may appear in comments only.
        commands = "\n".join(line for line in script.splitlines() if not line.lstrip().startswith("#"))
        assert "build/generated" not in commands
    assert "exported_keys" not in source("step2_genesis.sh")
    assert "with private_home(run):" in function_source("deploy")


# CometBFT /status protocol fixture served by an actual loopback HTTP peer.
STATUS = {"jsonrpc": "2.0", "id": -1, "result": {
    "node_info": {"network": "pax"},
    "sync_info": {"catching_up": False, "latest_block_height": "12"},
}}


@contextmanager
def status_peer(document):
    class StatusHandler(BaseHTTPRequestHandler):
        def do_GET(self):
            assert self.path == "/status"
            payload = json.dumps(self.server.document).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)

        def log_message(self, format, *arguments):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), StatusHandler)
    server.document = document
    thread = threading.Thread(target=server.serve_forever, kwargs={"poll_interval": 0.02})
    thread.start()
    try:
        yield server, thread, f"http://127.0.0.1:{server.server_port}/status"
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=2)
        assert not thread.is_alive()


def test_finding10_readiness_drives_chain_and_sync_consumer():
    with status_peer(STATUS) as (_, _, url):
        assert RUNTIME.ready_status(url, time.time() + 1) is True
    for field, value in (("catching_up", True), ("latest_block_height", "0")):
        document = json.loads(json.dumps(STATUS))
        document["result"]["sync_info"][field] = value
        with status_peer(document) as (_, _, url):
            assert RUNTIME.ready_status(url, time.time() + 1) is False
    document = json.loads(json.dumps(STATUS))
    document["result"]["node_info"]["network"] = "operator-chain"
    with status_peer(document) as (_, _, url):
        with pytest.raises(RUNTIME.StageFailure, match="chain identity"):
            RUNTIME.ready_status(url, time.time() + 1)


def test_finding10_supervision_revokes_launch_and_notifies_deploy(tmp_path):
    run = joined(tmp_path)
    observed = []
    marker = run.directory / "launch.0.complete"
    with status_peer(STATUS) as (server, server_thread, url):
        # A real loopback peer thread supplies liveness as well as the RPC transport.
        def supervise_peer():
            try:
                RUNTIME.monitor_daemon(run, server_thread.is_alive, url)
            except RUNTIME.StageFailure as error:
                observed.append(str(error))

        monitor = threading.Thread(target=supervise_peer)
        monitor.start()
        try:
            end = time.monotonic() + 2
            while not marker.exists() and monitor.is_alive() and time.monotonic() < end:
                time.sleep(0.01)
            assert marker.exists()
            assert json.loads(marker.read_text()) == {"chain_id": "pax", "node": 0}
            # Reopening the deploy consumer sees the real supervisor's atomic marker.
            consumer = RUNTIME.Run(tmp_path, environment())
            consumer.wait(lambda: marker.is_file())
            server.shutdown()
            server_thread.join(timeout=2)
            monitor.join(timeout=2)
            assert not monitor.is_alive()
            assert observed and "paxd exited" in observed[0]
            assert not marker.exists()
            with pytest.raises(RUNTIME.StageFailure, match="paxd exited"):
                consumer.wait(lambda: marker.is_file())
        finally:
            server.shutdown()
            monitor.join(timeout=2)


def test_finding10_live_peer_health_loss_revokes_success(tmp_path):
    run = joined(tmp_path)
    marker = run.directory / "launch.0.complete"
    observed = []
    with status_peer(STATUS) as (server, server_thread, url):
        def monitor_peer():
            try:
                RUNTIME.monitor_daemon(run, server_thread.is_alive, url)
            except RUNTIME.StageFailure as error:
                observed.append(str(error))

        monitor = threading.Thread(target=monitor_peer)
        monitor.start()
        try:
            end = time.monotonic() + 2
            while not marker.exists() and monitor.is_alive() and time.monotonic() < end:
                time.sleep(0.01)
            assert marker.exists()
            unhealthy = json.loads(json.dumps(STATUS))
            unhealthy["result"]["sync_info"]["catching_up"] = True
            server.document = unhealthy
            monitor.join(timeout=2)
            assert not monitor.is_alive()
            assert server_thread.is_alive()
            assert observed == ["paxd readiness lost"]
            assert not marker.exists()
            with pytest.raises(RUNTIME.StageFailure, match="readiness lost"):
                run.check(startup=False)
        finally:
            server.shutdown()
            monitor.join(timeout=2)


def test_finding10_not_ready_is_bounded_and_early_exit_is_failure(tmp_path):
    run = joined(tmp_path)
    document = json.loads(json.dumps(STATUS))
    document["result"]["sync_info"]["catching_up"] = True
    with status_peer(document) as (_, server_thread, url):
        run.deadline = time.time() + 0.12
        # HTTP peer liveness is a real resource; no native binary is substituted.
        with pytest.raises(RUNTIME.StageFailure, match="deadline"):
            RUNTIME.monitor_daemon(run, server_thread.is_alive, url)
    assert not (run.directory / "launch.0.complete").exists()
    assert "deadline" in (run.directory / "failed.0.json").read_text()
    other = RUNTIME.Run(tmp_path, environment(run_id="qualification-early-exit"))
    other.join()
    departed = threading.Thread(target=lambda: None)
    departed.start()
    departed.join()
    with pytest.raises(RUNTIME.StageFailure, match="paxd exited"):
        RUNTIME.monitor_daemon(other, departed.is_alive)
    assert not (other.directory / "launch.0.complete").exists()


def test_finding10_shell_supervisor_and_rpc_config_are_wired():
    command = shlex.split(source("step5_start_pax.sh").splitlines()[-1])
    assert command == ["exec", "python3", "docker/localnode/scripts/localnode_runtime.py", "supervise"]
    supervisor = function_source("supervise")
    assert 'run.env["PAXD_BIN"], "start", "--home", run.env["PAX_HOME"]' in supervisor
    assert '"--chain-id", "pax"' in supervisor
    assert "monitor_daemon(run, lambda: process.poll() is None)" in supervisor
    assert "stop_process(process)" in supervisor
    app = tomllib.loads((ROOT / "docker/localnode/config/config.toml").read_text())
    assert app["rpc"]["laddr"] == "tcp://0.0.0.0:26657"
    assert 'http://127.0.0.1:26657/status' in source("localnode_runtime.py")


def test_record_names_all_findings_tests_and_limits():
    record = tomllib.loads(RECORD.read_text())
    assert record["task"]["22"]["33"]["801"]["audit"] == "22.30.178"
    functions = {node.name for node in ast.walk(ast.parse(Path(__file__).read_text()))
                 if isinstance(node, ast.FunctionDef)}
    assert set(record["finding"]) == {"7", "8", "9", "10"}
    for finding in record["finding"].values():
        assert finding["tests"] and set(finding["tests"]) <= functions
    assert record["evidence"]["ledger"] == "/out/run/gates.jsonl"
    assert "not run" in record["baseline"]["verification"].lower()
    assert "native" in record["proof"]["review_only"].lower()
    assert "docker/docker-compose.yml" in record["wiring"]["compose"]["request"]
