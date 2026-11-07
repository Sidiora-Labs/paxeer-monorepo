"""Repository-root command contracts and an in-process native telemetry path."""

import ast
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
from pathlib import Path
import re
import runpy
import shlex
import sys
import threading

import pytest


ROOT = Path(__file__).resolve().parents[5]
OBSERVABILITY = ROOT / "deploy/launchpad/production/observability"
PHASE = OBSERVABILITY / "phase2"
LOAD = ROOT / "tools/launchpad/production/load"


def documented_commands():
    readme = (OBSERVABILITY / "README.md").read_text()
    return [
        shlex.split(block.replace("\\\n", " "))
        for block in re.findall(r"```sh\n(.*?)```", readme, re.DOTALL)
    ]


@pytest.fixture
def phase4(monkeypatch):
    monkeypatch.syspath_prepend(str(PHASE))
    return runpy.run_path(str(PHASE / "phase4.py"))


def test_observability_command_paths_match_real_cli(monkeypatch, capsys, phase4):
    commands = documented_commands()
    assert len(commands) == 3
    assert commands[0] == [
        "python3", "deploy/launchpad/production/observability/check.py",
        "--promtool", "/reviewed/path/to/promtool",
        "--output", "/private/evidence/checks.json",
    ]
    shared = [
        "--plan", "<private-plan>", "--binding", "<runtime-binding>",
        "--config", "<runtime-config>", "--root", "<repository-root>",
    ]
    script = "deploy/launchpad/production/observability/phase2/phase4.py"
    assert commands[1] == [
        "python3", script, "collect", *shared,
        "--output", "<fresh-private-collection-directory>",
    ]
    assert commands[2] == [
        "python3", script, "evaluate", *shared,
        "--output", "<fresh-private-evaluation-directory>",
        "--measurement", "<measurement-manifest>",
        "--measurement-sha256", "<measurement-digest>",
        "--shared-capture", "<shared-capture>",
        "--shared-capture-sha256", "<capture-digest>",
        "--scenario-evidence", "<phase3-evidence>",
        "--scenario-evidence-sha256", "<evidence-digest>",
        "--attestation-key-file", "<private-verifier-key>",
    ]
    references = [
        ["python3", *shlex.split(reference)]
        for reference in re.findall(
            r"`(deploy/launchpad/production/observability/[^`]+\.py[^`]*)`",
            (OBSERVABILITY / "README.md").read_text(),
        )
    ]
    assert len(references) == 4
    for command in [*commands, *references]:
        path = ROOT / command[1]
        assert path.is_file()
        declarations = {
            arg.value
            for node in ast.walk(ast.parse(path.read_text()))
            if isinstance(node, ast.Call)
            and isinstance(node.func, ast.Attribute)
            and node.func.attr == "add_argument"
            for arg in node.args
            if isinstance(arg, ast.Constant) and isinstance(arg.value, str)
        }
        assert {arg for arg in command if arg.startswith("--")} <= declarations
        monkeypatch.setattr(sys, "argv", [str(path), *command[2:], "--help"])
        with pytest.raises(SystemExit) as result:
            runpy.run_path(str(path), run_name="__main__")
        assert result.value.code == 0
        assert "usage:" in capsys.readouterr().out


def test_observability_source_root_and_safety_conditions(phase4):
    readme = (OBSERVABILITY / "README.md").read_text()
    assert "deploy/production/" not in readme
    assert "tools/production/" not in readme
    assert "`phase2/" not in readme
    assert phase4["ROOT"] == ROOT
    assert phase4["LOAD"] == LOAD
    for symbol, filename in (
        ("read_manifest", "capture.py"),
        ("bound_document", "phase2_types.py"),
    ):
        assert Path(phase4[symbol].__code__.co_filename).resolve() == LOAD / filename
        assert f"tools/launchpad/production/load/{filename}" in readme
    for condition in (
        "--phase 3 --operator-authority-file /private/operator-authority.json",
        "OS-owned private authority mount",
        "Native promtool 3.2.1 required",
        "verify its binary digest",
        "Runtime activation intentionally fails closed",
        "capacity, monthly availability and production qualification remain false",
        "Final shared 24-hour soak and all burst/restore acceptance belong to gate 9.1",
    ):
        assert condition in readme


@pytest.mark.parametrize("command_index", [1, 2], ids=["collect", "evaluate"])
def test_documented_measurement_cli_fails_closed_without_private_inputs(
    command_index, tmp_path, monkeypatch, capsys, phase4,
):
    command = documented_commands()[command_index]
    output = tmp_path / "private-result"
    arguments = [
        str(ROOT) if token == "<repository-root>"
        else str(output) if token.startswith("<fresh-private-")
        else str(tmp_path / token[1:-1]) if token.startswith("<")
        else token
        for token in command[2:]
    ]
    monkeypatch.setattr(sys, "argv", [command[1], *arguments])
    with pytest.raises(SystemExit) as result:
        phase4["main"]()
    assert result.value.code == 1
    diagnostic = json.loads(capsys.readouterr().out)
    assert diagnostic["passed"] is False
    assert diagnostic["reason"] == "missing_or_invalid_native_identity_or_plan"
    evidence = json.loads((output / "result.json").read_bytes())
    assert evidence["passed"] is False
    assert evidence["capacity_qualified"] is False
    assert evidence["monthly_availability_qualified"] is False
    assert evidence["production_qualified"] is False
    assert len(evidence["results"]) == 12
    assert all(row["passed"] is False for row in evidence["results"])


def test_documented_collector_imports_real_producer_transport_and_codec(phase4):
    from probes import exposition
    from runtime import fetch_metrics

    config = json.loads((OBSERVABILITY / "config-v2.json").read_bytes())
    binding = {
        "environment": "qualification", "probes": config["probes"],
        "synthetic": {"zone": "zone-a"},
    }
    observations = [
        {"journey": probe["journey"], "environment": "qualification",
         "success": 1, "observed_at": 1700000000}
        for probe in binding["probes"]
    ]
    raw = exposition(binding, observations)
    requests = []

    class MetricsOrigin(BaseHTTPRequestHandler):
        def do_GET(self):
            requests.append((self.path, self.headers.get("Accept")))
            self.send_response(200 if self.path == "/metrics" else 404)
            self.send_header("Content-Type", "text/plain; version=0.0.4")
            self.send_header("Content-Length", str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)

        def log_message(self, *_args):
            pass

    with ThreadingHTTPServer(("127.0.0.1", 0), MetricsOrigin) as server:
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            status, received, duration = fetch_metrics(
                f"http://127.0.0.1:{server.server_port}/metrics"
            )
            assert status == 200
            assert received == raw
            assert 0 <= duration < 5
            samples = phase4["prometheus_samples"](received, config)
        finally:
            server.shutdown()
            thread.join(timeout=3)
            assert not thread.is_alive()
    assert requests == [("/metrics", "text/plain")]
    assert len(samples) == 15
    for observation in observations:
        labels = tuple(sorted({
            "journey": observation["journey"],
            "environment": "qualification", "zone": "zone-a",
        }.items()))
        assert samples[("synthetic_journey_expected", labels)] == 1
        assert samples[("synthetic_journey_success", labels)] == 1
        assert samples[("synthetic_journey_last_observed_timestamp_seconds", labels)] == 1700000000
