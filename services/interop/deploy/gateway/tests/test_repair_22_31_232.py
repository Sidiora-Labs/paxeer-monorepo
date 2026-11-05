import contextlib
import copy
import http.client
import http.server
import json
import pathlib
import socket
import subprocess
import sys
import threading
import time

import pytest


GATEWAY = pathlib.Path(__file__).resolve().parents[1]
ROOT = GATEWAY.parents[3]
RENDERER = GATEWAY / "render.py"
FIXTURES = ROOT / "sdk/platform/conformance/fixtures"
OLD_RECORDING = FIXTURES / "sequencer-receipt-v1.json"
NEW_RECORDING = FIXTURES / "receipt-sequencer-v1.json"


def rotation_keys():
    old = json.loads(OLD_RECORDING.read_text())["cases"][0]["pinnedKey"]
    new = json.loads(NEW_RECORDING.read_text())["vectors"][0]["receiptKeys"][0]
    return [
        {**old, "lastHeight": 1199},
        {**new, "firstHeight": 1200},
    ]


def root_environment():
    example = json.loads((GATEWAY / "config.example.json").read_text())
    return {
        "LAYERX_BETA_INTEROP_" + name.upper(): json.dumps(example[name])
        for name in ("ap2_keys", "ap2_assets", "visa_agents", "visa_targets", "fiat_providers")
    }


def render(tmp_path, keys, extra=None):
    key_file = tmp_path / "receipt-keys.json"
    key_file.write_text(json.dumps(keys))
    key_file.chmod(0o600)
    output = tmp_path / "config.json"
    result = subprocess.run(
        [
            sys.executable, str(RENDERER), "--network-id", "eip155-125",
            "--receipt-keys-file", str(key_file), "--out", str(output),
        ],
        env={**root_environment(), **(extra or {})},
        capture_output=True,
        text=True,
        timeout=30,
    )
    return result, output


def expected_signers(keys):
    return {"eip155:125": sorted({"did:layerx:" + key["publicKey"][2:] for key in keys})}


def test_gateway_rotation_preserves_every_distinct_signer(tmp_path):
    keys = rotation_keys()
    repeated = {**keys[0], "keyId": "0x" + "cc" * 32, "firstHeight": 2000, "lastHeight": 2999}
    declared = [repeated, keys[1], keys[0]]
    result, output = render(tmp_path, declared)
    assert result.returncode == 0, result.stderr
    manifest = json.loads(output.read_text())
    assert manifest["x402_supported"]["signers"] == expected_signers(keys)
    assert manifest["x402_supported"]["kinds"] == [
        {"x402Version": 2, "scheme": "exact", "network": "eip155:125"}
    ]
    assert output.stat().st_mode & 0o777 == 0o600
    assert json.loads((tmp_path / "receipt-keys.json").read_text()) == declared


@pytest.mark.parametrize("source", ["environment", "manifest"])
def test_gateway_rotation_accepts_exact_signer_set_in_either_order(tmp_path, source):
    keys = rotation_keys()
    supported = {
        "kinds": [{"x402Version": 2, "scheme": "exact", "network": "eip155:125"}],
        "extensions": [],
        "signers": {"eip155:125": expected_signers(keys)["eip155:125"][::-1]},
    }
    if source == "environment":
        extra = {"LAYERX_BETA_INTEROP_X402_SUPPORTED": json.dumps(supported)}
    else:
        override = tmp_path / "override.json"
        override.write_text(json.dumps({"x402_supported": supported}))
        extra = {"LAYERX_BETA_INTEROP_MANIFEST_FILE": str(override)}
    result, output = render(tmp_path, keys, extra)
    assert result.returncode == 0, result.stderr
    assert json.loads(output.read_text())["x402_supported"] == supported


@pytest.mark.parametrize("change", ["missing", "extra", "duplicate", "network", "zero", "malformed"])
def test_gateway_rotation_refuses_unauthorized_signer_overrides(tmp_path, change):
    keys = rotation_keys()
    signers = expected_signers(keys)
    supported = {
        "kinds": [{"x402Version": 2, "scheme": "exact", "network": "eip155:125"}],
        "extensions": [],
        "signers": signers,
    }
    if change == "missing":
        signers["eip155:125"].pop()
    elif change == "extra":
        signers["eip155:125"].append("did:layerx:" + "aa" * 32)
    elif change == "duplicate":
        signers["eip155:125"].append(signers["eip155:125"][0])
    elif change == "network":
        supported["kinds"][0]["network"] = "eip155:126"
        signers["eip155:126"] = signers.pop("eip155:125")
    elif change == "zero":
        signers["eip155:125"][0] = "did:layerx:" + "00" * 32
    else:
        supported["signers"] = []
    result, output = render(
        tmp_path, keys, {"LAYERX_BETA_INTEROP_X402_SUPPORTED": json.dumps(supported)}
    )
    assert result.returncode == 2, result.stderr
    assert "x402" in result.stderr.lower()
    assert not output.exists()


@pytest.mark.parametrize("change", ["overlap", "reversed", "zero", "boolean"])
def test_gateway_rotation_keeps_receipt_window_validation(tmp_path, change):
    keys = rotation_keys()
    if change == "overlap":
        keys[1]["firstHeight"] = keys[0]["lastHeight"]
    elif change == "reversed":
        keys[1]["firstHeight"] = keys[1]["lastHeight"] + 1
    elif change == "zero":
        keys[1]["publicKey"] = "0x" + "00" * 32
    else:
        keys[1]["firstHeight"] = True
    result, output = render(tmp_path, keys)
    assert result.returncode == 2, result.stderr
    assert "receipt keys file" in result.stderr
    assert not output.exists()


@pytest.fixture(scope="module")
def real_binaries(tmp_path_factory):
    target = tmp_path_factory.mktemp("gateway-rotation-binaries")
    log = target / "cargo-build.log"
    command = [
        "cargo", "build", "--manifest-path", str(GATEWAY / "tests/rotation/Cargo.toml"),
        "--bins", "--target-dir", str(target),
    ]
    with log.open("w") as evidence:
        result = subprocess.run(command, cwd=ROOT, stdout=evidence, stderr=subprocess.STDOUT, timeout=1200)
    assert result.returncode == 0, f"real service preparation exited {result.returncode}: {log}\n{log.read_text()[-10000:]}"
    return target / "debug/rotation-gateway", target / "debug/rotation-probe"


@contextlib.contextmanager
def recorded_sequencer(keys):
    state = {"keys": copy.deepcopy(keys), "calls": []}

    class RecordedServer(http.server.BaseHTTPRequestHandler):
        protocol_version = "HTTP/1.1"

        def log_message(self, *args):
            pass

        def send_document(self, body):
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.send_header("Connection", "close")
            self.end_headers()
            self.wfile.write(body)
            self.close_connection = True

        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            assert self.path == "/"
            assert self.headers["Content-Type"].startswith("application/json")
            assert request["jsonrpc"] == "2.0"
            assert request["method"] == "px_getReceiptKeys"
            assert request["params"] == []
            state["calls"].append(request["method"])
            self.send_document(json.dumps({
                "jsonrpc": "2.0", "id": request["id"], "result": {"keys": state["keys"]},
            }).encode())

        def do_GET(self):
            name = self.path.removeprefix("/recorded/")
            assert name in ("old", "new", "outside", "unpinned")
            state["calls"].append(name)
            source = OLD_RECORDING if name == "old" else NEW_RECORDING
            self.send_document(source.read_bytes())

    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), RecordedServer)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield server.server_port, state
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
        assert not thread.is_alive()


def openssl(tmp_path, *arguments):
    result = subprocess.run(["openssl", *arguments], cwd=tmp_path, capture_output=True, timeout=30)
    assert result.returncode == 0, result.stderr.decode()
    return result.stdout


def service_environment(tmp_path, port):
    openssl(
        tmp_path, "req", "-x509", "-newkey", "ec", "-pkeyopt", "ec_paramgen_curve:prime256v1",
        "-nodes", "-days", "1", "-subj", "/CN=localhost", "-keyout", "key.pem", "-out", "cert.pem",
    )
    openssl(tmp_path, "x509", "-in", "cert.pem", "-outform", "DER", "-out", "cert.der")
    openssl(
        tmp_path, "pkcs12", "-export", "-in", "cert.pem", "-inkey", "key.pem",
        "-passout", "pass:rotation-test-identity", "-out", "client.p12",
    )
    public_key = openssl(tmp_path, "pkey", "-in", "key.pem", "-pubout", "-outform", "DER")[-65:].hex()
    trust = root_environment()
    ap2 = json.loads(trust["LAYERX_BETA_INTEROP_AP2_KEYS"])
    for key in ap2:
        key["public_key_sec1"] = public_key
    trust["LAYERX_BETA_INTEROP_AP2_KEYS"] = json.dumps(ap2)
    signer = rotation_keys()[0]["publicKey"][2:]
    for name, field in (("VISA_AGENTS", "public_key"), ("FIAT_PROVIDERS", "public_key_ed25519")):
        values = json.loads(trust["LAYERX_BETA_INTEROP_" + name])
        for value in values:
            value[field] = signer
        trust["LAYERX_BETA_INTEROP_" + name] = json.dumps(values)
    for filename, content in (
        ("identity-password", "rotation-test-identity"), ("redis-username", "interop"),
        ("redis-password", "rotation-test-redis-password"),
        ("registry.json", '{"modules":[{"module":1,"ordinals":[1]}]}'),
    ):
        path = tmp_path / filename
        path.write_text(content)
        path.chmod(0o600)
    environ = {
        "LAYERX_INTEROP_" + variable: str(tmp_path / filename)
        for variable, filename in (
            ("CONFIG", "config.json"), ("OUTBOUND_CA_DER", "cert.der"),
            ("CLIENT_IDENTITY_PKCS12", "client.p12"),
            ("CLIENT_IDENTITY_PASSWORD_FILE", "identity-password"),
            ("REDIS_USERNAME_FILE", "redis-username"), ("REDIS_PASSWORD_FILE", "redis-password"),
            ("RECEIPT_KEYS_FILE", "receipt-keys.json"), ("MODULE_REGISTRY_FILE", "registry.json"),
        )
    }
    environ.update({
        "LAYERX_INTEROP_LISTENER": "plain", "LAYERX_INTEROP_REDIS_URL": "rediss://localhost:1",
        "LAYERX_INTEROP_NETWORK_ID": "eip155-125", "LAYERX_INTEROP_WIRE_VERSION": "3",
        "LAYERX_INTEROP_PROTOCOL_NETWORK_ID": "402", "LAYERX_INTEROP_CHAIN_ID": "125",
        "LAYERX_INTEROP_READINESS_MAX_AGE_MS": "30000", "LAYERX_INTEROP_TAP_CLOCK_SKEW_SECONDS": "60",
        "LAYERX_INTEROP_SEQUENCER_URL": f"http://127.0.0.1:{port}",
        "ROTATION_RECORDING_PORT": str(port),
    })
    return environ, trust


@contextlib.contextmanager
def running_gateway(binary, environ, tmp_path):
    with socket.socket() as reserved:
        reserved.bind(("127.0.0.1", 0))
        port = reserved.getsockname()[1]
    log = tmp_path / "gateway-stderr.log"
    with log.open("ab") as stderr:
        process = subprocess.Popen(
            [str(binary)], env={**environ, "LAYERX_INTEROP_LISTEN": f"127.0.0.1:{port}"},
            stdout=subprocess.DEVNULL, stderr=stderr,
        )
        try:
            deadline = time.monotonic() + 15
            while True:
                assert process.poll() is None, log.read_text()
                connection = http.client.HTTPConnection("127.0.0.1", port, timeout=1)
                try:
                    connection.request("GET", "/livez")
                    response = connection.getresponse()
                    body = json.loads(response.read())
                    assert response.status == 200
                    assert body["status"] == "live"
                    break
                except (ConnectionError, TimeoutError):
                    assert time.monotonic() < deadline, log.read_text()
                    time.sleep(0.05)
                finally:
                    connection.close()
            yield process, {**environ, "LAYERX_INTEROP_LISTEN": f"127.0.0.1:{port}"}
        finally:
            if process.poll() is None:
                process.terminate()
            process.wait(timeout=10)


def probe_receipts(binary, environ, *names):
    result = subprocess.run([str(binary), *names], env=environ, capture_output=True, text=True, timeout=30)
    assert result.returncode == 0, result.stderr
    checked = json.loads(result.stdout)["verified"]
    assert [item["recording"] for item in checked] == list(names)
    for item in checked:
        if item["recording"] in ("old", "new"):
            assert item["height"] == {"old": 1000, "new": 1200}[item["recording"]]
        else:
            assert item["refused"] == "SequencerSignature"


def test_gateway_rotation_real_service_and_receipts_survive_restart(tmp_path, real_binaries):
    gateway, probe = real_binaries
    keys = rotation_keys()
    with recorded_sequencer(keys[:1]) as (port, state):
        environ, trust = service_environment(tmp_path, port)
        before, output = render(tmp_path, keys[:1], trust)
        assert before.returncode == 0, before.stderr
        assert json.loads(output.read_text())["x402_supported"]["signers"] == expected_signers(keys[:1])
        with running_gateway(gateway, environ, tmp_path) as (_, live_environ):
            probe_receipts(probe, live_environ, "old")
        state["keys"] = keys
        rotated, output = render(tmp_path, keys[::-1], trust)
        assert rotated.returncode == 0, rotated.stderr
        assert json.loads(output.read_text())["x402_supported"]["signers"] == expected_signers(keys)
        manifest_bytes = output.read_bytes()
        key_bytes = (tmp_path / "receipt-keys.json").read_bytes()
        for _ in range(2):
            with running_gateway(gateway, environ, tmp_path) as (_, live_environ):
                probe_receipts(probe, live_environ, "old", "new", "outside", "unpinned")
            assert output.read_bytes() == manifest_bytes
            assert (tmp_path / "receipt-keys.json").read_bytes() == key_bytes
        assert state["calls"].count("px_getReceiptKeys") == 6
        assert state["calls"].count("old") == 3
        assert state["calls"].count("new") == 2
        assert state["calls"].count("outside") == 2
        assert state["calls"].count("unpinned") == 2
