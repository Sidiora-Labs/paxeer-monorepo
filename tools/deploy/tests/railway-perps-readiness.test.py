"""Exercise the production apply against an in-memory Railway peer over HTTP."""

import importlib.util
import io
import json
import os
import re
import sys
import tempfile
import threading
import urllib.error
import urllib.request
from contextlib import contextmanager, redirect_stderr, redirect_stdout
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path


HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
FIXTURE = HERE / "fixtures/railway-perps-readiness.json"
DIGEST = "sha256:" + "a" * 64
TOKEN = "readiness-test-token"


class RailwayPeer:
    def __init__(self, fixture, scenario):
        self.fixture = fixture
        self.scenario = scenario
        self.services = {service["name"]: dict(service) for service in fixture["services"]}
        self.events = []
        self.errors = []
        self.health_index = 0
        self.domain = None

    def graphql(self, request):
        match = re.match(r"\s*(query|mutation)\s+(\w+)\(", request["query"])
        assert match, request
        kind, operation = match.groups()
        variables = request["variables"]
        self.events.append((kind, operation, variables))
        if operation == "Projects":
            assert variables == {"w": self.fixture["workspace"], "a": None}
            return {"projects": {
                "pageInfo": {"hasNextPage": False, "endCursor": None},
                "edges": [{"node": {
                    "id": "p1", "name": self.fixture["project"],
                    "environments": {"edges": [{"node": {
                        "id": "e1", "name": self.fixture["environment"],
                    }}]},
                    "services": {"edges": [{"node": {"id": name, "name": name}}
                                            for name in self.services]},
                    "volumes": {"edges": []},
                }}],
            }}
        if operation in ("Instance", "Variables", "Domains", "Deploy"):
            name = variables.get("s")
            assert name in self.services, variables
            expected = {"s": name, "e": "e1"}
            if operation in ("Variables", "Domains"):
                expected["p"] = "p1"
            assert variables == expected
            service = self.services[name]
            if operation == "Instance":
                assert "healthcheckPath" in request["query"]
                return {"serviceInstance": {
                    "source": {"image": f"ghcr.io/sidiora-labs/{service['image']}@{DIGEST}"},
                    "healthcheckPath": service["health"], "healthcheckTimeout": 300,
                    "restartPolicyType": service["restart"], "numReplicas": service["replicas"],
                    "latestDeployment": {"id": f"dep-{name}", "status": service["status"]},
                }}
            if operation == "Variables":
                return {"variables": {}}
            if operation == "Domains":
                return {"domains": {"serviceDomains": [{"domain": self.domain}]}}
            assert operation == "Deploy" and kind == "mutation" and name == "web"
            service["status"] = "SUCCESS"
            return {"serviceInstanceDeployV2": "dep-web"}
        assert operation == "DeploymentStatus" and kind == "query", request
        name = variables["id"].removeprefix("dep-")
        assert variables == {"id": f"dep-{name}"} and name in self.services
        status = self.services[name]["status"]
        self.events.append(("deployment", name, status))
        return {"deployment": {"status": status}}

    def health(self, path):
        assert path == self.services["explorer"]["health"], path
        statuses = self.scenario["health_statuses"]
        assert self.health_index < len(statuses), "unexpected extra health request"
        status = statuses[self.health_index]
        self.health_index += 1
        self.events.append(("health", path, status))
        if status == 200 and self.scenario["confirm_deployment"]:
            self.services["explorer"]["status"] = "SUCCESS"
        return status


@contextmanager
def serve(peer):
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def do_POST(self):
            try:
                assert self.path == "/graphql/v2"
                assert self.headers["Authorization"] == f"Bearer {TOKEN}"
                assert self.headers["Content-Type"] == "application/json"
                request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                assert set(request) == {"query", "variables"}
                response = {"data": peer.graphql(request)}
                status = 200
            except Exception as error:
                peer.errors.append(repr(error))
                response, status = {"errors": [{"message": repr(error)}]}, 400
            self.respond(status, json.dumps(response).encode())

        def do_GET(self):
            try:
                status = peer.health(self.path)
            except Exception as error:
                peer.errors.append(repr(error))
                status = 500
            self.respond(status, b"ready" if status == 200 else b"unavailable")

        def respond(self, status, body):
            self.send_response(status)
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, kwargs={"poll_interval": 0.01})
    peer.domain = f"127.0.0.1:{server.server_port}"
    thread.start()
    try:
        yield f"http://{peer.domain}"
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
        assert not thread.is_alive()
        assert not peer.errors, peer.errors


@contextmanager
def deployment(scenario):
    fixture = json.loads(FIXTURE.read_text())
    spec = importlib.util.spec_from_file_location(
        "readiness_perps_project", ROOT / "tools/deploy/railway-perps-project.py",
    )
    module = importlib.util.module_from_spec(spec)
    original_path = sys.path[:]
    spec.loader.exec_module(module)
    masked = set(module.mask.__globals__["MASKED"])
    env = {
        "RAILWAY_API_TOKEN": TOKEN, "RAILWAY_WORKSPACE_ID": fixture["workspace"],
        "GHCR_USERNAME": "readiness-registry-user", "GHCR_TOKEN": "readiness-registry-token",
        "ROLLOUT_TIMEOUT": "0", "ROLLOUT_POLL": "0",
    }
    previous = {key: os.environ.get(key) for key in env}
    peer = RailwayPeer(fixture, fixture["scenarios"][scenario])
    try:
        os.environ.update(env)
        with tempfile.TemporaryDirectory() as directory, serve(peer) as endpoint:
            directory = Path(directory)
            lines = [
                '[meta]', 'kind = "railway-project"', f'project = "{fixture["project"]}"',
                f'environment = "{fixture["environment"]}"',
            ]
            _, declared = module.load(module.MANIFEST)
            for service in fixture["services"]:
                actual = next(item for item in declared if item["name"] == service["name"])
                for key in ("image", "order", "health", "restart", "replicas"):
                    assert actual[key] == service[key], (key, actual, service)
                lines.extend([
                    f'[service.{service["name"]}]', 'kind = "image"',
                    f'image = "{service["image"]}"', f'railway_service = "{service["name"]}"',
                    f'order = {service["order"]}', f'health = "{service["health"]}"',
                    f'restart = "{service["restart"]}"', f'replicas = {service["replicas"]}',
                    'public = 1',
                ])
            manifest = directory / "project.kvx"
            manifest.write_text("\n".join(lines) + "\n")
            digests = directory / "digests.json"
            digests.write_text(json.dumps({service["image"]: DIGEST for service in fixture["services"]}))
            module.API = endpoint + "/graphql/v2"
            argv = ["--apply", "--manifest", str(manifest), "--digests", str(digests),
                    "--start-height", "123"]
            yield module, argv, peer, endpoint
    finally:
        for key, value in previous.items():
            if value is None:
                os.environ.pop(key, None)
            else:
                os.environ[key] = value
        module.mask.__globals__["MASKED"].clear()
        module.mask.__globals__["MASKED"].update(masked)
        sys.path[:] = original_path


def apply(module, argv):
    stdout, stderr = io.StringIO(), io.StringIO()
    with redirect_stdout(stdout), redirect_stderr(stderr):
        code = module.main(list(argv))
    return code, stdout.getvalue(), stderr.getvalue()


def wake(endpoint, path):
    try:
        with urllib.request.urlopen(endpoint + path, timeout=2) as response:
            return response.status, response.read()
    except urllib.error.HTTPError as error:
        with error:
            return error.code, error.read()


def assert_blocked(result, events):
    code, stdout, stderr = result
    assert code == 1, (code, stdout, stderr)
    assert "wave 5 explorer SLEEPING\n" in stdout
    assert ("wave 5 failed: explorer deployment dep-explorer is SLEEPING; "
            "later waves were not applied") in stderr
    assert "wave 6 " not in stdout
    assert "applied web " not in stdout and "deployed web " not in stdout
    assert not [event for event in events if event[0] == "mutation"], events
    assert not [event for event in events if event[:2] == ("deployment", "web")], events


def check_successful_wake_readiness():
    with deployment("wake") as (module, argv, peer, endpoint):
        health = peer.services["explorer"]["health"]
        assert wake(endpoint, health) == (503, b"unavailable")
        assert_blocked(apply(module, argv), peer.events)
        assert wake(endpoint, health) == (200, b"ready")
        code, stdout, stderr = apply(module, argv)
        assert code == 0 and stderr == "", (code, stdout, stderr)
        assert "wave 5 explorer SUCCESS\n" in stdout
        assert "applied web Deploy\n" in stdout
        assert "wave 6 web SUCCESS\n" in stdout
        assert stdout.index("wave 5 explorer SUCCESS\n") < stdout.index("applied web Deploy\n")
        ready = peer.events.index(("health", health, 200))
        confirmed = peer.events.index(("deployment", "explorer", "SUCCESS"))
        mutations = [event for event in peer.events if event[0] == "mutation"]
        assert mutations == [("mutation", "Deploy", {"s": "web", "e": "e1"})]
        assert ready < confirmed < peer.events.index(mutations[0])
        assert peer.events[-1] == ("deployment", "web", "SUCCESS")


def check_sleeping_unavailable_refusal():
    with deployment("unavailable") as (module, argv, peer, endpoint):
        health = peer.services["explorer"]["health"]
        for _ in range(2):
            assert wake(endpoint, health) == (503, b"unavailable")
            assert_blocked(apply(module, argv), peer.events)
        assert peer.health_index == 2
        assert peer.services["explorer"]["status"] == "SLEEPING"


def check_health_without_deployment_confirmation_refused():
    with deployment("unconfirmed") as (module, argv, peer, endpoint):
        health = peer.services["explorer"]["health"]
        assert wake(endpoint, health) == (200, b"ready")
        assert_blocked(apply(module, argv), peer.events)
        assert peer.services["explorer"]["status"] == "SLEEPING"


if __name__ == "__main__":
    check_successful_wake_readiness()
    check_sleeping_unavailable_refusal()
    check_health_without_deployment_confirmation_refused()
    print("railway-perps-readiness.test.py: ok")
