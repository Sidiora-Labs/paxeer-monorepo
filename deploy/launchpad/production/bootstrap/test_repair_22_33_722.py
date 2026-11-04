"""Regression proof for audit 22.30.170's bootstrap_command_paths finding."""

import ast
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
from pathlib import Path
import re
import shlex
from threading import Thread
from urllib.parse import urlsplit

import pytest


HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[3]
PREFIX = HERE.relative_to(ROOT).as_posix()


def documented_commands():
    text = (HERE / "README.md").read_text()
    return [
        shlex.split(line)
        for block in re.findall(r"```sh\n(.*?)```", text, re.DOTALL)
        for line in block.splitlines()
        if line.strip()
    ]


def test_bootstrap_command_paths():
    commands = documented_commands()
    compose = ["docker", "compose", "-f", f"{PREFIX}/compose.yml"]
    assert commands == [
        [*compose, "config", "--quiet"],
        ["python3", f"{PREFIX}/validate.py"],
        ["python3", f"{PREFIX}/start.py"],
        ["python3", f"{PREFIX}/smoke.py"],
        [*compose, "down"],
    ]
    for command in commands:
        path = command[1] if command[0] == "python3" else command[3]
        assert (ROOT / path).is_file(), path
        assert (ROOT / path).parent == HERE
        if command[0] == "python3":
            tree = ast.parse((ROOT / path).read_text())
            assert any(
                isinstance(node, ast.FunctionDef)
                and node.name == "main"
                and not node.args.args
                for node in tree.body
            ), f"{path} must expose its documented argument-free entry point"
            assert any(
                isinstance(node, ast.If)
                and ast.unparse(node.test) == "__name__ == '__main__'"
                and any(
                    isinstance(child, ast.Call)
                    and isinstance(child.func, ast.Name)
                    and child.func.id == "main"
                    and not child.args
                    and not child.keywords
                    for child in ast.walk(node)
                )
                for node in tree.body
            ), f"{path} must invoke main when run as documented"


def test_bootstrap_command_paths_preserve_safety_conditions():
    text = (HERE / "README.md").read_text()
    normalized = " ".join(text.split())
    for condition in (
        "Never fund these accounts outside this fixture.",
        "All host ports bind to `127.0.0.1`.",
        "No existing service, production volume, `.env`, or chain data is mounted.",
        "The coordinator starts this stack once, after reviewing this configuration:",
        "An initialization failure fails startup.",
        "contract deployment and application address configuration remain explicit coordinator prerequisites.",
        "Verify the pinned images can be obtained on a fresh host before claiming cold-host reproduction.",
        "Deleting fixture volumes is a separate destructive local cleanup decision; the provided scripts never remove volumes.",
        "Concurrent independent fixture stacks require explicitly reviewed project names and host-port overrides; the default stack is coordinator-owned to prevent duplicate starts.",
    ):
        assert condition in normalized
    for link in re.findall(r"\]\(([^)]+)\)", text):
        assert (HERE / link).is_file(), link


class FixtureRpcHandler(BaseHTTPRequestHandler):
    """In-process JSON-RPC peer for the documented fixture's real smoke client."""

    def do_POST(self):
        body = self.rfile.read(int(self.headers["Content-Length"]))
        request = json.loads(body)
        self.server.requests.append(
            (self.path, self.headers["Content-Type"], request)
        )
        response = {"jsonrpc": "2.0", "id": request["id"]}
        if request["method"] == "eth_chainId":
            response["result"] = "0x7d"
        else:
            response["error"] = {"code": -32601, "message": "Method not found"}
        payload = json.dumps(response).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def log_message(self, format, *args):
        pass


def test_bootstrap_command_paths_real_smoke_client():
    smoke_command = documented_commands()[3]
    spec = importlib.util.spec_from_file_location(
        "bootstrap_smoke_repair_22_33_722", ROOT / smoke_command[1]
    )
    smoke = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(smoke)
    assert smoke.HERE == HERE

    environment = dict(
        line.split("=", 1)
        for line in (HERE / "fixture.env.example").read_text().splitlines()
        if line and not line.startswith("#")
    )
    endpoint = urlsplit(environment["RPC_URL"])
    assert endpoint.scheme == "http"
    assert endpoint.hostname == "127.0.0.1"
    assert f'"{endpoint.hostname}:{endpoint.port}:8545"' in (
        HERE / "compose.yml"
    ).read_text()

    with ThreadingHTTPServer(
        (endpoint.hostname, endpoint.port), FixtureRpcHandler
    ) as server:
        server.requests = []
        thread = Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            assert smoke.rpc("eth_chainId", []) == hex(int(environment["CHAIN_ID"]))
            with pytest.raises(RuntimeError, match="-32601.*Method not found"):
                smoke.rpc("fixture_unsupported", [{"probe": "doc-path"}])
            assert server.requests == [
                (
                    "/",
                    "application/json",
                    {"jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []},
                ),
                (
                    "/",
                    "application/json",
                    {
                        "jsonrpc": "2.0",
                        "id": 1,
                        "method": "fixture_unsupported",
                        "params": [{"probe": "doc-path"}],
                    },
                ),
            ]
        finally:
            server.shutdown()
            thread.join(timeout=5)
            assert not thread.is_alive()
