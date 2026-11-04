import importlib.util
import json
import re
import threading
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import pytest


API_TOKEN = 'repair-railway-token-"quoted"'
REGISTRY_USER = "repair-registry-user"
REGISTRY_PASSWORD = 'repair-registry-password-"quoted"\\slash'
AUTH_SECRET = 'repair-auth-secret-"quoted"\nsecond-line'
RPC_URLS = "repair-private-rpc-addresses"
SECRETS = (API_TOKEN, REGISTRY_USER, REGISTRY_PASSWORD, AUTH_SECRET, RPC_URLS)
DIGEST = "sha256:" + "a" * 64


class RailwayTranscript:
    def __init__(self, fail_op=None, error_shape="messages", status=200):
        self.fail_op = fail_op
        self.error_shape = error_shape
        self.status = status
        self.requests = []
        self.responses = []

    def respond(self, request):
        op = re.match(r"\s*(?:query|mutation)\s+(\w+)", request["query"]).group(1)
        self.requests.append((op, request))
        status, reason = 200, "OK"
        if op == self.fail_op:
            message = "deployment refused: " + json.dumps(request["variables"])
            message += " credentials=" + json.dumps(SECRETS)
            errors = [{"message": message, "extensions": {"input": request["variables"]}}]
            if self.error_shape == "malformed":
                errors = {"nested": {"echo": message}}
            response = {"errors": errors}
            status = self.status
            if status != 200:
                reason = f"refused {REGISTRY_PASSWORD} {RPC_URLS}"
        else:
            data = {
                "Projects": {"projects": {
                    "pageInfo": {"hasNextPage": False, "endCursor": None},
                    "edges": [{"node": {
                        "id": "project-1", "name": "repair-project",
                        "environments": {"edges": [{"node": {"id": "env-1", "name": "production"}}]},
                        "services": {"edges": [{"node": {"id": "service-1", "name": "gateway"}}]},
                        "volumes": {"edges": []},
                    }}],
                }},
                "Instance": {"serviceInstance": {"source": {"image": "registry.invalid/previous:old"}}},
                "Variables": {"variables": {}},
                "InstanceUpdate": {"serviceInstanceUpdate": True},
                "VariablesUpsert": {"variableCollectionUpsert": True},
                "Deploy": {"serviceInstanceDeployV2": "deployment-1"},
                "DeploymentStatus": {"deployment": {"status": "SUCCESS"}},
            }
            response = {"data": data[op]}
        self.responses.append(response)
        return status, reason, response


@contextmanager
def railway_server(transcript):
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            status, reason, response = transcript.respond(request)
            transcript.requests[-1][1]["headers"] = dict(self.headers)
            transcript.requests[-1][1]["path"] = self.path
            body = json.dumps(response).encode()
            self.send_response(status, reason)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, kwargs={"poll_interval": 0.01})
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_port}/graphql/v2"
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
        assert not thread.is_alive()


@pytest.fixture
def client(tmp_path, monkeypatch):
    path = Path(__file__).resolve().parents[1] / "railway-perps-project.py"
    spec = importlib.util.spec_from_file_location("repair_perps_project", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    rollout = __import__("rollout")
    monkeypatch.setattr(rollout, "MASKED", set())
    for name, value in {
        "RAILWAY_API_TOKEN": API_TOKEN,
        "RAILWAY_WORKSPACE_ID": "workspace-1",
        "GHCR_USERNAME": REGISTRY_USER,
        "GHCR_TOKEN": REGISTRY_PASSWORD,
        "REPAIR_AUTH_VALUE": AUTH_SECRET,
        "REPAIR_RPC_VALUE": RPC_URLS,
    }.items():
        monkeypatch.setenv(name, value)
    manifest = tmp_path / "project.kvx"
    manifest.write_text(
        '[meta]\nkind = "railway-project"\nproject = "repair-project"\nenvironment = "production"\n'
        '[service.gateway]\nkind = "image"\nimage = "repair-gateway"\nrailway_service = "gateway"\n'
        'order = 1\nhealth = "/healthz"\nrestart = "ALWAYS"\nreplicas = 1\n'
        '[vars.gateway]\nAUTH_SECRET = "env:REPAIR_AUTH_VALUE"\nRPC_URLS = "env:REPAIR_RPC_VALUE"\n'
    )
    digests = tmp_path / "digests.json"
    digests.write_text(json.dumps({"repair-gateway": DIGEST}))
    argv = ["--apply", "--manifest", str(manifest), "--digests", str(digests), "--start-height", "123"]
    return module, argv


def assert_private(output):
    for secret in SECRETS:
        for form in (secret, json.dumps(secret)[1:-1], json.dumps(json.dumps(secret)[1:-1])[1:-1]):
            assert form not in output


def assert_wire(transcript):
    assert transcript.requests
    for op, request in transcript.requests:
        assert request["path"] == "/graphql/v2"
        assert request["headers"]["Content-Type"] == "application/json"
        assert request["headers"]["Authorization"] == f"Bearer {API_TOKEN}"
        assert set(request) == {"query", "variables", "headers", "path"}
        assert re.match(r"\s*(?:query|mutation) " + op + r"\(", request["query"])
    requests = dict(transcript.requests)
    if "InstanceUpdate" in requests:
        request = requests["InstanceUpdate"]
        assert "serviceInstanceUpdate(serviceId: $s, environmentId: $e, input: $i)" in request["query"]
        variables = request["variables"]
        assert variables["s"] == "service-1"
        assert variables["e"] == "env-1"
        assert variables["i"]["registryCredentials"] == {
            "username": REGISTRY_USER, "password": REGISTRY_PASSWORD,
        }
    if "VariablesUpsert" in requests:
        request = requests["VariablesUpsert"]
        assert "variableCollectionUpsert(input: $i)" in request["query"]
        assert request["variables"] == {"i": {
            "projectId": "project-1", "environmentId": "env-1", "serviceId": "service-1",
            "variables": {"AUTH_SECRET": AUTH_SECRET, "RPC_URLS": RPC_URLS}, "skipDeploys": True,
        }}


@pytest.mark.parametrize("operation", ["Projects", "InstanceUpdate", "VariablesUpsert"])
@pytest.mark.parametrize("error_shape", ["messages", "malformed"])
def test_finding_005_echoed_credentials_and_variables_are_redacted(
    client, monkeypatch, capsys, operation, error_shape,
):
    module, argv = client
    transcript = RailwayTranscript(operation, error_shape)
    with railway_server(transcript) as endpoint:
        monkeypatch.setattr(module, "API", endpoint)
        assert module.main(argv) == 1
    captured = capsys.readouterr()
    assert_private(captured.out + captured.err)
    assert "[redacted]" in captured.err
    assert "Railway GraphQL error:" in captured.err
    assert "deployment refused:" in captured.err
    assert ("railway query failed:" if operation == "Projects" else "railway write failed:") in captured.err
    assert transcript.requests[-1][0] == operation
    assert "Deploy" not in dict(transcript.requests)
    assert_wire(transcript)
    request = transcript.requests[-1][1]
    with railway_server(RailwayTranscript(operation, error_shape)) as endpoint:
        monkeypatch.setattr(module, "API", endpoint)
        with pytest.raises(RuntimeError) as raised:
            module.http_transport(API_TOKEN)(request["query"], request["variables"])
    assert_private(str(raised.value))
    assert "Railway GraphQL error:" in str(raised.value)
    assert "[redacted]" in str(raised.value)


def test_finding_005_success_preserves_real_mutation_payloads(client, monkeypatch, capsys):
    module, argv = client
    transcript = RailwayTranscript()
    with railway_server(transcript) as endpoint:
        monkeypatch.setattr(module, "API", endpoint)
        assert module.main(argv) == 0
    captured = capsys.readouterr()
    assert_private(captured.out + captured.err)
    assert captured.err == ""
    assert "wave 1 gateway SUCCESS" in captured.out
    assert [op for op, _ in transcript.requests] == [
        "Projects", "Instance", "Variables", "InstanceUpdate", "VariablesUpsert", "Deploy", "DeploymentStatus",
    ]
    assert_wire(transcript)


@pytest.mark.parametrize("operation", ["Projects", "VariablesUpsert"])
def test_finding_005_http_exception_diagnostics_are_redacted(client, monkeypatch, capsys, operation):
    module, argv = client
    transcript = RailwayTranscript(operation, status=403)
    with railway_server(transcript) as endpoint:
        monkeypatch.setattr(module, "API", endpoint)
        assert module.main(argv) == 1
    captured = capsys.readouterr()
    assert_private(captured.out + captured.err)
    assert "HTTP Error 403: refused [redacted] [redacted]" in captured.err
    assert transcript.requests[-1][0] == operation
    assert_wire(transcript)


@pytest.mark.parametrize("missing", [False, True], ids=["recorded-error", "missing-response"])
def test_finding_005_replay_sanitizes_producer_generated_payloads(
    client, monkeypatch, capsys, tmp_path, missing,
):
    module, argv = client
    transcript = RailwayTranscript("VariablesUpsert")
    with railway_server(transcript) as endpoint:
        monkeypatch.setattr(module, "API", endpoint)
        assert module.main(argv) == 1
    assert_wire(transcript)
    capsys.readouterr()
    replay = tmp_path / "recorded.json"
    entries = [
        dict(op=op, variables=request["variables"], **response)
        for (op, request), response in zip(transcript.requests, transcript.responses)
    ]
    replay.write_text(json.dumps(entries[:-1] if missing else entries))
    __import__("rollout").MASKED.clear()
    assert module.main(argv + ["--replay", str(replay)]) == 1
    captured = capsys.readouterr()
    assert_private(captured.out + captured.err)
    assert "railway write failed: Railway GraphQL error:" in captured.err
    assert "[redacted]" in captured.err
    assert ("replay has no response for VariablesUpsert" if missing else "deployment refused:") in captured.err
