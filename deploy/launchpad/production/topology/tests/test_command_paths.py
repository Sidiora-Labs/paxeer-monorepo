"""Exercise the topology_command_paths examples through their real entry points."""

from contextlib import redirect_stdout
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from io import StringIO
from pathlib import Path
import ast
import json
import re
import runpy
import shlex
import sys
import threading

import pytest

from deploy.launchpad.production.topology import topology
from tools.launchpad.production.integration.adapters import LocalHTTP


ROOT = Path(__file__).resolve().parents[5]
TOPOLOGY = ROOT / "deploy/launchpad/production/topology"
SCRIPT = "deploy/launchpad/production/topology/topology.py"


def documented_commands():
    blocks = re.findall(r"```sh\n(.*?)```", (TOPOLOGY / "README.md").read_text(), re.S)
    return [shlex.split(line) for block in blocks for line in block.splitlines() if line]


def invoke(command, paths):
    """Run the documented Python script in-process, replacing only reviewed files."""
    assert command[:2] == ["python3", SCRIPT]
    argv = [str(ROOT / command[1])] + [str(paths.get(arg, arg)) for arg in command[2:]]
    previous = sys.argv
    output = StringIO()
    try:
        sys.argv = argv
        with redirect_stdout(output), pytest.raises(SystemExit) as stopped:
            runpy.run_path(argv[0], run_name="__main__")
    finally:
        sys.argv = previous
    return stopped.value.code, json.loads(output.getvalue()) if output.getvalue() else None


def test_topology_command_paths_match_canonical_cli_and_source_root():
    commands = documented_commands()
    config = ["--config", "/reviewed/topology.json"]
    assert commands == [
        ["python3", SCRIPT, "validate", *config],
        ["python3", SCRIPT, "render", *config, "--output", "/reviewed/plan.json"],
        ["python3", SCRIPT, "verify-plan", *config, "--plan", "/reviewed/plan.json"],
        ["python3", SCRIPT, "check-evidence", *config, "--plan", "/reviewed/plan.json",
         "--evidence", "/reviewed/measured.json"],
        ["python3", "-m", "unittest", "discover", "-s",
         "deploy/launchpad/production/topology/tests", "-v"],
        ["python3", "deploy/launchpad/production/topology/native_nats_check.py"],
    ]
    for command in commands:
        if command[1] == "-m":
            assert (ROOT / command[command.index("-s") + 1]).is_dir()
        else:
            source = ROOT / command[1]
            assert source.is_file()
            assert any(isinstance(node, ast.FunctionDef) and node.name == "main"
                       for node in ast.parse(source.read_text()).body)
    readme = (TOPOLOGY / "README.md").read_text()
    for directory in ("deploy/launchpad/production", "tools/launchpad/production"):
        assert (ROOT / directory).is_dir()
        assert f"`{directory}`" in readme
    assert "root as `--source-root`" in readme
    assert "deploy/production/" not in readme
    assert "tools/production/" not in readme


def test_topology_command_paths_real_cli_http_plan_roundtrip(tmp_path):
    commands = documented_commands()
    paths = {
        "/reviewed/topology.json": TOPOLOGY / "fixtures/synthetic-config.json",
        "/reviewed/plan.json": tmp_path / "plan.json",
        "/reviewed/measured.json": tmp_path / "measured.json",
    }
    code, validated = invoke(commands[0], paths)
    assert code == 0
    assert validated["status"] == "valid_arithmetic_plan_unqualified"
    assert invoke(commands[1], paths) == (0, None)
    raw = paths["/reviewed/plan.json"].read_bytes()
    produced = topology.decode(raw)
    assert produced == topology.compile_plan(topology.load(paths["/reviewed/topology.json"]))
    assert produced["configurationDigest"] == validated["configurationDigest"]
    requests = []

    class PlanServer(BaseHTTPRequestHandler):
        def do_GET(self):
            requests.append((self.command, self.path, self.headers["Content-Type"]))
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)

        def log_message(self, *args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), PlanServer)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        port = server.server_address[1]
        client = LocalHTTP(f"http://127.0.0.1:{port}", {port})
        status, received = client.request("GET", "/plan.json")
        assert status == 200
        assert received == raw
        assert requests == [("GET", "/plan.json", "application/json")]
        paths["/reviewed/plan.json"].write_bytes(received)
        assert invoke(commands[2], paths) == (0, {"status": "exact_reviewed_plan_unqualified"})
        # Compiler output is a declaration, never a substitute for measured evidence.
        paths["/reviewed/measured.json"].write_bytes(received)
        code, rejected = invoke(commands[3], paths)
        assert code == 1
        assert rejected["status"] == "rejected"
        assert "evidence: missing/unknown keys" in rejected["error"]
    finally:
        server.shutdown()
        thread.join(timeout=5)
        server.server_close()
    assert not thread.is_alive()


def test_topology_command_paths_keep_review_and_isolation_conditions(tmp_path):
    commands = documented_commands()
    code, rejected = invoke(commands[0], {
        "/reviewed/topology.json": TOPOLOGY / "production-input.template.json",
    })
    assert code == 1
    assert rejected["status"] == "rejected"
    paths = {
        "/reviewed/topology.json": TOPOLOGY / "fixtures/synthetic-config.json",
        "/reviewed/plan.json": tmp_path / "plan.json",
    }
    assert invoke(commands[1], paths) == (0, None)
    plan = topology.load(paths["/reviewed/plan.json"])
    service = next(item for item in plan["resources"]["items"] if item["kind"] == "Service")
    service["spec"]["type"] = "LoadBalancer"
    plan["deploymentDigest"] = topology.digest(plan["resources"])
    paths["/reviewed/plan.json"].write_bytes(topology.canonical(plan))
    code, rejected = invoke(commands[2], paths)
    assert code == 1
    assert rejected["status"] == "rejected"
    assert "public service forbidden" in rejected["error"]
    readme = (TOPOLOGY / "README.md").read_text()
    for condition in (
        "it does not establish deployment, throughput or safe failover",
        "require signed measurements for the exact candidate",
        "The compiler never invokes kubectl, fetches secrets, modifies a cluster, signs evidence or promotes a release",
        "must occur in an isolated qualification environment before any deployment",
        "The parser refuses implicit image pulls, uses no network",
        "requires the broker's actual configuration-validity marker",
        "notDeploymentAuthorization: true",
    ):
        assert condition in readme
