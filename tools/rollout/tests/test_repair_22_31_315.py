import base64
import json
import shutil
import subprocess
import time
import uuid
from pathlib import Path
from urllib.parse import parse_qs, urlsplit

import pytest


HERE = Path(__file__).resolve().parents[1]
ROLE = "repair-rollout"
UNIT = f"{ROLE}.service"
IMAGE_FILE = f"/etc/layerx/{ROLE}.image"
OLD_IMAGE = "ghcr.io/sidiora-labs/paxd:" + "1" * 12
RESTART_SHA = "2" * 40
UNHEALTHY_SHA = "3" * 40
HEALTHY_SHA = "4" * 40
TOKEN = "rollout-regression-registry-token"
TOKEN_FILE = "/fixtures/token file;'quoted'"


SERVICE_PROGRAM = r'''
import json
import os
import socket
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

image = Path("/etc/layerx/repair-rollout.image").read_text().strip()
if sys.argv[1] == "check":
    previous = Path("/etc/layerx/repair-rollout.image.prev")
    with open("/fixtures/starts.jsonl", "a") as log:
        log.write(json.dumps({"image": image,
                              "previous": previous.read_text().strip() if previous.exists() else None}) + "\n")
        log.flush()
        os.fsync(log.fileno())
    if image.endswith(":" + "2" * 12):
        raise SystemExit(17)
    if image.endswith(":" + "1" * 12) and Path("/fixtures/refuse-recovery").exists():
        raise SystemExit(18)
    raise SystemExit(0)


class Health(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def do_GET(self):
        status = 503 if image.endswith(":" + "3" * 12) else 200
        if self.path != "/healthz":
            status = 404
        body = json.dumps({"image": image}).encode()
        with open("/fixtures/health.jsonl", "a") as log:
            log.write(json.dumps({"path": self.path, "status": status, "image": image}) + "\n")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


server = ThreadingHTTPServer(("127.0.0.1", 8080), Health)
address = os.environ["NOTIFY_SOCKET"]
if address.startswith("@"):
    address = "\0" + address[1:]
with socket.socket(socket.AF_UNIX, socket.SOCK_DGRAM) as notify:
    notify.connect(address)
    notify.sendall(b"READY=1")
server.serve_forever()
'''


DOCKER_REPLAY_PROGRAM = r'''
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlsplit


class DockerTranscript(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def reply(self, status, body):
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("API-Version", "1.52")
        self.send_header("OSType", "linux")
        self.end_headers()
        if self.command != "HEAD":
            self.wfile.write(body)

    def exchange(self):
        body = self.rfile.read(int(self.headers.get("Content-Length", "0")))
        path = urlsplit(self.path).path
        if path == "/_ping":
            operation, response = "ping", b"OK"
        elif path == "/version":
            operation = "version"
            response = json.dumps({"Version": "29.1.2", "ApiVersion": "1.52",
                                   "MinAPIVersion": "1.44", "Os": "linux", "Arch": "amd64"}).encode()
        elif path.endswith("/auth") and self.command == "POST":
            operation, response = "login", b'{"Status":"Login Succeeded"}'
        elif path.endswith("/images/create") and self.command == "POST":
            operation, response = "pull", b'{"status":"Pull complete"}\n'
        else:
            operation, response = "unexpected", b'{"message":"unrecorded Docker request"}'
        with open("/fixtures/docker.jsonl", "a") as log:
            log.write(json.dumps({"operation": operation, "method": self.command, "path": self.path,
                                  "headers": {k.lower(): v for k, v in self.headers.items()},
                                  "body": body.decode()}) + "\n")
        self.reply(404 if operation == "unexpected" else 200, response)

    do_HEAD = exchange
    do_GET = exchange
    do_POST = exchange


ThreadingHTTPServer(("127.0.0.1", 23750), DockerTranscript).serve_forever()
'''


APPLY_PROGRAM = r'''
import json
import os
import sys

sys.path.insert(0, "/fixtures/rollout")
import rollout

os.environ["REGISTRY_TOKEN_FILE"] = "/fixtures/token file;'quoted'"
os.environ["GHCR_ACTOR"] = "registry-user"
with open("/fixtures/apply.log", "w") as log:
    outcome = rollout.apply_box(
        {"box_role": "repair-rollout", "box_unit": "repair-rollout.service",
         "image": "paxd", "health": "/healthz"},
        sys.argv[1], {"REPAIR_ROLLOUT_HOST": "localhost"}, log)
print(json.dumps({"outcome": outcome}))
raise SystemExit(0 if outcome == "pass" else 1)
'''


class Box:
    def __init__(self, name):
        self.name = name

    def execute(self, *args, check=True, input=None):
        result = subprocess.run(
            ["docker", "exec", "-i", self.name, *args],
            input=input, text=True, capture_output=True, timeout=60,
        )
        if check:
            assert result.returncode == 0, result.stdout + result.stderr
        return result

    def read(self, path):
        return self.execute("cat", path).stdout

    def records(self, path):
        return [json.loads(line) for line in self.read(path).splitlines()]

    def write(self, path, value):
        self.execute("tee", path, input=value)

    def health(self):
        result = self.execute("curl", "-fsS", "--max-time", "2", "http://127.0.0.1:8080/healthz")
        return json.loads(result.stdout)

    def apply(self, sha):
        result = self.execute("python3", "/fixtures/apply.py", sha, check=False)
        log = self.read("/fixtures/apply.log")
        assert TOKEN not in result.stdout + result.stderr + log
        expected = "ghcr.io/sidiora-labs/paxd:" + sha[:12]
        calls = self.records("/fixtures/docker.jsonl")
        assert not any(call["operation"] == "unexpected" for call in calls), calls
        operations = [call for call in calls if call["operation"] in {"login", "pull"}]
        assert [call["operation"] for call in operations][-2:] == ["login", "pull"]
        login, pull = operations[-2:]
        auth = json.loads(login["body"])
        assert auth["username"] == "registry-user"
        assert auth["password"] == TOKEN
        assert auth["serveraddress"] == "ghcr.io"
        assert login["headers"]["content-type"] == "application/json"
        assert login["headers"]["user-agent"].startswith("Docker-Client/")
        query = parse_qs(urlsplit(pull["path"]).query)
        assert query["fromImage"] == ["ghcr.io/sidiora-labs/paxd"]
        assert query["tag"] == [sha[:12]]
        encoded = pull["headers"]["x-registry-auth"]
        registry_auth = json.loads(base64.urlsafe_b64decode(encoded + "=" * (-len(encoded) % 4)))
        assert registry_auth["username"] == "registry-user"
        assert registry_auth["password"] == TOKEN
        assert registry_auth["serveraddress"] == "ghcr.io"
        starts = self.records("/fixtures/starts.jsonl")
        assert starts[1]["image"] == expected
        assert starts[1]["previous"] == OLD_IMAGE
        return result, log


@pytest.fixture(scope="module")
def systemd_box(tmp_path_factory):
    assert shutil.which("docker"), "The recovery proof requires Docker and a running Docker daemon"
    files = tmp_path_factory.mktemp("rollout-real-box")
    rollout_files = files / "rollout"
    rollout_files.mkdir()
    for name in ("rollout.py", "box-apply.sh"):
        shutil.copy2(HERE / name, rollout_files / name)
    (files / "service.py").write_text(SERVICE_PROGRAM)
    (files / "docker-replay.py").write_text(DOCKER_REPLAY_PROGRAM)
    (files / "apply.py").write_text(APPLY_PROGRAM)
    (files / "token file;'quoted'").write_text(TOKEN + "\n")
    (files / UNIT).write_text(
        "[Service]\nType=notify\nNotifyAccess=main\n"
        "ExecStartPre=/usr/bin/python3 /fixtures/service.py check\n"
        "ExecStart=/usr/bin/python3 /fixtures/service.py serve\n"
        "TimeoutStartSec=10\nTimeoutStopSec=5\n"
    )
    (files / "rollout-docker-replay.service").write_text(
        "[Service]\nType=exec\nExecStart=/usr/bin/python3 /fixtures/docker-replay.py\n"
    )
    name = "rollout-recovery-" + uuid.uuid4().hex
    box = Box(name)
    provision = (
        "apt-get update > /tmp/provision.log 2>&1 && "
        "apt-get install -y --no-install-recommends systemd dbus openssh-server python3 curl "
        ">> /tmp/provision.log 2>&1 && "
        "exec /usr/lib/systemd/systemd --unit=multi-user.target --log-target=console"
    )
    try:
        created = subprocess.run(
            ["docker", "run", "--detach", "--name", name, "--privileged", "--cgroupns=private",
             "--tmpfs", "/run", "--tmpfs", "/tmp", "--env", "container=docker",
             "--env", "DEBIAN_FRONTEND=noninteractive", "ubuntu:26.04", "bash", "-ec", provision],
            text=True, capture_output=True, timeout=180,
        )
        assert created.returncode == 0, created.stdout + created.stderr
        deadline = time.monotonic() + 240
        while time.monotonic() < deadline:
            ready = box.execute("systemctl", "show", "--property=Version", "--value", check=False)
            if ready.returncode == 0:
                break
            state = subprocess.run(
                ["docker", "inspect", "--format", "{{.State.Running}}", name],
                text=True, capture_output=True, timeout=15,
            )
            assert state.returncode == 0 and state.stdout.strip() == "true", (
                subprocess.run(["docker", "logs", name], text=True, capture_output=True, timeout=15).stderr
            )
            time.sleep(0.25)
        else:
            pytest.fail("Real systemd did not start: " + box.read("/tmp/provision.log"))
        box.execute("mkdir", "-p", "/fixtures", "/etc/layerx", "/root/.ssh", "/root/.docker")
        for source, destination in ((str(files) + "/.", "/fixtures"),
                                    (str(Path(shutil.which("docker")).resolve()), "/usr/local/bin/docker")):
            copied = subprocess.run(
                ["docker", "cp", source, f"{name}:{destination}"],
                text=True, capture_output=True, timeout=60,
            )
            assert copied.returncode == 0, copied.stdout + copied.stderr
        box.execute("cp", f"/fixtures/{UNIT}", "/etc/systemd/system/")
        box.execute("cp", "/fixtures/rollout-docker-replay.service", "/etc/systemd/system/")
        box.execute("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", "/root/.ssh/id_ed25519")
        box.execute("cp", "/root/.ssh/id_ed25519.pub", "/root/.ssh/authorized_keys")
        host_key = box.read("/etc/ssh/ssh_host_ed25519_key.pub").split()
        box.write("/root/.ssh/known_hosts", f"localhost {host_key[0]} {host_key[1]}\n")
        box.write("/root/.ssh/config", "Host localhost\n    StrictHostKeyChecking yes\n    LogLevel ERROR\n")
        box.execute("chmod", "700", "/root/.ssh")
        box.execute("chmod", "600", "/root/.ssh/config", "/root/.ssh/authorized_keys", TOKEN_FILE)
        box.write("/etc/environment", "DOCKER_HOST=tcp://127.0.0.1:23750\nROLLOUT_HEALTH_TIMEOUT=1\n")
        box.execute("systemctl", "daemon-reload")
        box.execute("systemctl", "start", "ssh.service", "rollout-docker-replay.service")
        box.execute(
            "python3", "-c",
            "import time, urllib.request\n"
            "for attempt in range(100):\n"
            " try:\n"
            "  urllib.request.urlopen('http://127.0.0.1:23750/_ping', timeout=1).close(); break\n"
            " except OSError:\n"
            "  time.sleep(.05)\n"
            "else: raise RuntimeError('Docker transcript server did not listen')\n",
        )
        yield box
    finally:
        subprocess.run(["docker", "rm", "--force", name], capture_output=True, timeout=60)


@pytest.fixture
def box(systemd_box):
    box = systemd_box
    box.execute("systemctl", "stop", UNIT)
    box.execute("systemctl", "reset-failed")
    box.execute("rm", "-f", IMAGE_FILE + ".prev", IMAGE_FILE + ".tmp", "/fixtures/refuse-recovery")
    box.write(IMAGE_FILE, OLD_IMAGE + "\n")
    box.execute("chmod", "640", IMAGE_FILE)
    for path in ("starts", "health", "docker"):
        box.write(f"/fixtures/{path}.jsonl", "")
    box.write("/root/.docker/config.json", "{}\n")
    box.execute("systemctl", "start", UNIT)
    assert box.health() == {"image": OLD_IMAGE}
    yield box
    box.execute("systemctl", "stop", UNIT)


@pytest.mark.parametrize("sha", [RESTART_SHA, UNHEALTHY_SHA], ids=["restart-failed", "health-failed"])
def test_finding_004_failure_restores_image_and_recovers_after_reopen(box, sha):
    result, log = box.apply(sha)
    assert result.returncode == 1, result.stdout + result.stderr + log
    assert json.loads(result.stdout) == {"outcome": "fail"}
    assert f"fail {ROLE} ghcr.io/sidiora-labs/paxd:{sha[:12]} {UNIT} /healthz" in log
    assert f"restored {ROLE} {OLD_IMAGE}" in log
    assert box.read(IMAGE_FILE) == OLD_IMAGE + "\n"
    assert box.read(IMAGE_FILE + ".prev") == OLD_IMAGE + "\n"
    assert box.execute("stat", "-c", "%a", IMAGE_FILE).stdout.strip() == "640"
    assert [start["image"] for start in box.records("/fixtures/starts.jsonl")] == [
        OLD_IMAGE, f"ghcr.io/sidiora-labs/paxd:{sha[:12]}", OLD_IMAGE,
    ]
    probes = box.records("/fixtures/health.jsonl")
    if sha == UNHEALTHY_SHA:
        assert any(probe["status"] == 503 and probe["image"].endswith(sha[:12]) for probe in probes)
    else:
        assert not any(probe["image"].endswith(sha[:12]) for probe in probes)
    assert box.health() == {"image": OLD_IMAGE}
    box.execute("systemctl", "stop", UNIT)
    assert box.execute("systemctl", "is-active", "--quiet", UNIT, check=False).returncode != 0
    assert box.read(IMAGE_FILE + ".prev") == OLD_IMAGE + "\n"
    box.execute("systemctl", "start", UNIT)
    assert box.health() == {"image": OLD_IMAGE}
    assert box.records("/fixtures/starts.jsonl")[-1]["image"] == OLD_IMAGE


@pytest.mark.parametrize("sha", [RESTART_SHA, UNHEALTHY_SHA], ids=["restart-failed", "health-failed"])
def test_finding_004_failed_recovery_retains_previous_image_and_reports_failure(box, sha):
    box.write("/fixtures/refuse-recovery", "refuse the next old-image startup\n")
    result, log = box.apply(sha)
    assert result.returncode == 1, result.stdout + result.stderr + log
    assert json.loads(result.stdout) == {"outcome": "fail"}
    assert f"restore of {ROLE} {OLD_IMAGE} is unhealthy too" in log
    assert f"restored {ROLE} {OLD_IMAGE}" not in log
    assert box.read(IMAGE_FILE) == OLD_IMAGE + "\n"
    assert box.read(IMAGE_FILE + ".prev") == OLD_IMAGE + "\n"
    assert [start["image"] for start in box.records("/fixtures/starts.jsonl")] == [
        OLD_IMAGE, f"ghcr.io/sidiora-labs/paxd:{sha[:12]}", OLD_IMAGE,
    ]
    assert box.execute("systemctl", "is-active", "--quiet", UNIT, check=False).returncode != 0
    box.execute("rm", "/fixtures/refuse-recovery")
    box.execute("systemctl", "start", UNIT)
    assert box.health() == {"image": OLD_IMAGE}


def test_finding_004_success_keeps_new_image_and_durable_previous_image(box):
    result, log = box.apply(HEALTHY_SHA)
    assert result.returncode == 0, result.stdout + result.stderr + log
    assert json.loads(result.stdout) == {"outcome": "pass"}
    new_image = "ghcr.io/sidiora-labs/paxd:" + HEALTHY_SHA[:12]
    assert f"pass {ROLE} {new_image} {UNIT} /healthz" in log
    assert box.read(IMAGE_FILE) == new_image + "\n"
    assert box.read(IMAGE_FILE + ".prev") == OLD_IMAGE + "\n"
    assert box.health() == {"image": new_image}
    assert [start["image"] for start in box.records("/fixtures/starts.jsonl")] == [OLD_IMAGE, new_image]
