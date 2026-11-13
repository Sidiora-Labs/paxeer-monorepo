"""Real binding, controller scheduling, subprocess and loopback transport proof."""

import asyncio
from copy import deepcopy
from datetime import datetime, timedelta, timezone
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
import os
from pathlib import Path
import sys
import threading

import pytest


DIRECTORY = Path(__file__).resolve().parent
KINDS = ("workload", "replica_loss", "checkpoint_restore_replay")
SPEC = importlib.util.spec_from_file_location("charts_phase4_repair", DIRECTORY / "phase4.py")
PHASE4 = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PHASE4)


def write_document(path, value):
    path.write_text(json.dumps(value, sort_keys=True))
    return PHASE4.digest_file(path)


@pytest.fixture
def bound_plan(tmp_path):
    state = {"begun": {}, "ended": {}, "instructions": {}, "errors": []}

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def reply(self, value):
            raw = json.dumps(value).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)

        def do_POST(self):
            if self.headers.get_content_type() != "application/json":
                state["errors"].append("incorrect content type")
                self.send_error(415)
                return
            row = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            if self.path == "/begin":
                state["begun"][row["kind"]] = row
                instruction = {"stdoutBytes": 32, "stderrBytes": 16, "sleepSeconds": 0}
                instruction.update(state["instructions"].get(row["kind"], {}))
                self.reply(instruction)
            elif self.path == "/end":
                state["ended"][row["kind"]] = row
                self.reply({"accepted": True})
            else:
                state["errors"].append(self.path)
                self.send_error(404)

        def do_GET(self):
            if self.path != "/finished":
                state["errors"].append(self.path)
                self.send_error(404)
                return
            self.reply({"finished": set(state["ended"]) == set(KINDS)})

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    worker = threading.Thread(target=server.serve_forever, daemon=True)
    worker.start()
    executable = tmp_path / "command-peer"
    executable.write_bytes(
        f"#!{sys.executable}\n".encode()
        + (DIRECTORY / "fixtures" / "command_peer.py").read_bytes()
    )
    executable.chmod(0o700)
    executable_digest = PHASE4.digest_file(executable)
    environment = {
        "CHART_PEER_URL": f"http://127.0.0.1:{server.server_port}",
        "CHART_PEER_TOKEN": "explicit-diagnostic-environment",
    }
    # Bind the actual repository tree; do not replace ROOT or bypass validators.
    sources = set(PHASE4.REQUIRED_SOURCES)
    for package in (
        "services/launchpad/trading-charts",
        "sdk/launchpad/shared/accounting",
        "sdk/launchpad/shared/delivery",
        "sdk/launchpad/shared/storage",
        "sdk/launchpad/shared/interfaces/v2",
    ):
        sources.update(str(path.relative_to(PHASE4.ROOT)) for path in (PHASE4.ROOT / package).rglob("*.go"))
    for module in ("services/launchpad/trading-charts", "sdk/launchpad/shared"):
        sources.update(f"{module}/{filename}" for filename in ("go.mod", "go.sum"))
    sources.add(str((DIRECTORY / "fixtures" / "command_peer.py").relative_to(PHASE4.ROOT)))
    files = {name: PHASE4.digest_file(PHASE4.ROOT / name) for name in sorted(sources)}
    candidate = hashlib.sha256(json.dumps(files, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
    config_path = tmp_path / "measurement.json"
    config_digest = write_document(config_path, {
        "candidateDigest": candidate,
        "startAt": "2030-01-01T00:00:00Z",
        "burstOffsetSeconds": 1,
        "rawArtifactBudgetBytes": 1048576,
    })
    commands = []
    for kind in KINDS:
        input_path = tmp_path / f"{kind}.json"
        input_digest = write_document(input_path, {"kind": kind})
        commands.append({
            "kind": kind,
            "offsetSeconds": 0,
            "argv": [str(executable), "--input", str(input_path), "--input-sha256", input_digest],
            "executableSha256": executable_digest,
            "environment": dict(environment),
            "timeoutSeconds": 5,
            "boundFiles": {str(input_path): input_digest},
        })
    plan = {
        "measureConfig": str(config_path),
        "measureConfigSha256": config_digest,
        "binary": str(executable),
        "binarySha256": executable_digest,
        "sourceFiles": files,
        "environment": environment,
        "releaseCommands": commands,
        "maximumCommandLogBytes": 1048576,
    }
    try:
        yield plan, state, tmp_path
    finally:
        server.shutdown()
        server.server_close()
        worker.join(timeout=5)
        assert not worker.is_alive()
        assert not state["errors"]


def execute_release(plan, directory):
    # Set a fresh coordinated launch only after validating the complete binding.
    config, _ = PHASE4.validate_binding(plan)
    config["startAt"] = (datetime.now(timezone.utc) + timedelta(seconds=0.2)).isoformat()
    plan["measureConfigSha256"] = write_document(Path(plan["measureConfig"]), config)
    config, _ = PHASE4.validate_binding(plan)
    output = directory / "capture"
    output.mkdir(mode=0o700)
    asyncio.run(PHASE4.run(plan, config, output, release=True))


def assert_reaped(state):
    assert state["begun"]
    for receipt in state["begun"].values():
        with pytest.raises(ProcessLookupError):
            os.kill(receipt["pid"], 0)


def test_chart_controller_timeout_valid_binding_reaches_execution(bound_plan):
    plan, state, directory = bound_plan
    # The diagnostic measurement ends with 7 only after all real controllers
    # report completion. No invented chart samples enter the production summary.
    with pytest.raises(RuntimeError, match="actual provider command failed"):
        execute_release(plan, directory)
    assert set(state["begun"]) == {*KINDS, "measurement"}
    assert set(state["ended"]) == set(KINDS)
    for index, command in enumerate(plan["releaseCommands"]):
        kind = command["kind"]
        receipt = state["ended"][kind]
        assert receipt["environment"] == plan["environment"]["CHART_PEER_TOKEN"]
        assert receipt["inputSha256"] == command["argv"][-1]
        raw = (directory / "capture" / "commands" / f"{index}-{kind}.log").read_bytes()
        header, payload = raw.split(b"\n", 1)
        assert json.loads(header) == receipt
        assert payload == b"o" * 32 + b"e" * 16
    measurement = state["begun"]["measurement"]
    assert measurement["release"] is True
    assert measurement["inputSha256"] == plan["measureConfigSha256"]
    assert measurement["output"] == str(directory / "capture" / "measurement")
    assert b"controllers finished; no provider measurement" in (directory / "capture" / "measure.log").read_bytes()
    assert_reaped(state)


@pytest.mark.parametrize("timeout", [True, False, 0, -1, 86401, 1.0, "1", None])
def test_chart_controller_timeout_rejects_invalid(bound_plan, timeout):
    plan, state, _ = bound_plan
    plan["releaseCommands"][0]["timeoutSeconds"] = timeout
    with pytest.raises(ValueError, match="bounded controller runtime required"):
        PHASE4.validate_binding(plan)
    assert not state["begun"]


@pytest.mark.parametrize("timeout", [1, 86400])
def test_chart_controller_timeout_accepts_integer_boundaries(bound_plan, timeout):
    plan, _, _ = bound_plan
    for command in plan["releaseCommands"]:
        command["timeoutSeconds"] = timeout
    config, digest = PHASE4.validate_binding(plan)
    assert config["candidateDigest"] == digest


@pytest.mark.parametrize("kind", KINDS)
def test_chart_controller_runtime_limit_terminates_real_process(bound_plan, kind):
    plan, state, directory = bound_plan
    state["instructions"][kind] = {"sleepSeconds": 30}
    for command in plan["releaseCommands"]:
        if command["kind"] == kind:
            command["timeoutSeconds"] = 1
    with pytest.raises(TimeoutError):
        execute_release(plan, directory)
    assert kind in state["begun"]
    assert kind not in state["ended"]
    assert_reaped(state)


@pytest.mark.parametrize("kind", (*KINDS, "measurement"))
@pytest.mark.parametrize("stream,budget", [("stdoutBytes", 1048576), ("stderrBytes", 1114112)])
def test_chart_controller_log_budget_bounds_real_capture(bound_plan, kind, stream, budget):
    plan, state, directory = bound_plan
    plan["maximumCommandLogBytes"] = budget
    state["instructions"][kind] = {stream: budget + 65536, "sleepSeconds": 30}
    with pytest.raises(ValueError, match="actual controller log byte budget exceeded"):
        execute_release(plan, directory)
    assert kind in state["begun"]
    if kind == "measurement":
        log = directory / "capture" / "measure.log"
    else:
        log = directory / "capture" / "commands" / f"{KINDS.index(kind)}-{kind}.log"
    assert 0 < log.stat().st_size <= budget
    for capture in (directory / "capture").rglob("*.log"):
        assert capture.stat().st_size <= budget
    assert_reaped(state)


@pytest.mark.parametrize("binding,message", [
    ("source", "current candidate source digest differs"),
    ("missing_source", "all actual measurement and provider source/module paths must be bound"),
    ("candidate", "candidate digest must name exact executable source manifest"),
    ("config", "bounded digest-bound actual input required"),
    ("measurement_executable", "actual built measurement executable differs"),
    ("controller_executable", "actual controller executable binding required"),
    ("controller_input", "actual workload/checkpoint command input differs"),
    ("empty_inputs", "actual controller inputs/checkpoint manifest must be bound"),
])
def test_chart_controller_retains_digest_checks(bound_plan, binding, message):
    original, state, _ = bound_plan
    plan = deepcopy(original)
    command = plan["releaseCommands"][0]
    if binding == "source":
        plan["sourceFiles"]["tools/launchpad/production/charts-engine/phase4.py"] = "0" * 64
    elif binding == "missing_source":
        del plan["sourceFiles"]["tools/launchpad/production/charts-engine/phase4.py"]
    elif binding == "candidate":
        config = json.loads(Path(plan["measureConfig"]).read_text())
        config["candidateDigest"] = "0" * 64
        plan["measureConfigSha256"] = write_document(Path(plan["measureConfig"]), config)
    elif binding == "config":
        with Path(plan["measureConfig"]).open("ab") as output:
            output.write(b" ")
    elif binding == "measurement_executable":
        plan["binarySha256"] = "0" * 64
    elif binding == "controller_executable":
        command["executableSha256"] = "0" * 64
    elif binding == "controller_input":
        with Path(command["argv"][2]).open("ab") as output:
            output.write(b" ")
    elif binding == "empty_inputs":
        command["boundFiles"] = {}
    with pytest.raises(ValueError, match=message):
        PHASE4.validate_binding(plan)
    assert not state["begun"]
