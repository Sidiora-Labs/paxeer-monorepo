from contextlib import contextmanager
import http.server
import json
import os
import pathlib
import subprocess
import threading
import unittest
import urllib.error
import urllib.request

import pytest

ROOT = pathlib.Path(__file__).resolve().parents[3]
ACCOUNT_ADDRESS = "0x000000000000000000000000000000000000dEaD"


def network_answer(consensus_available=True):
    result = {
        "network_id": "paxeer-125",
        "paxeer": {"chain_id": "0x7d", "latest_block": "0x2a"},
        "consensus": None,
    }
    if consensus_available:
        result["consensus"] = {
            "latest_height": "42",
            "latest_block_hash": "AB" * 32,
            "latest_app_hash": "CD" * 32,
            "catching_up": False,
        }
    else:
        result["reason"] = "consensus_unavailable"
    return {"result": result}


@contextmanager
def endpoint_server(account_answer, consensus_available=True):
    requests = []

    class EndpointHandler(http.server.BaseHTTPRequestHandler):
        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            requests.append(
                (self.path, request, {key.lower(): value for key, value in self.headers.items()})
            )
            if self.path != "/":
                self.send_error(404)
                return
            answers = {
                "eth_chainId": {"result": "0x7d"},
                "px_resolveAccount": {"result": {"bound": True}},
                "px_getNetwork": network_answer(consensus_available),
                "px_getAccount": account_answer,
            }
            answer = json.dumps(
                {"jsonrpc": "2.0", "id": request["id"], **answers[request["method"]]}
            ).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(answer)))
            self.end_headers()
            self.wfile.write(answer)

    with http.server.HTTPServer(("127.0.0.1", 0), EndpointHandler) as server:
        thread = threading.Thread(target=server.serve_forever)
        thread.start()
        try:
            yield f"http://127.0.0.1:{server.server_port}", requests
        finally:
            server.shutdown()
            thread.join()


def run_endpoint_account_probe(account_answer, consensus_available=True):
    with endpoint_server(account_answer, consensus_available) as (base, requests):
        checked = subprocess.run(
            ["bash", "scripts/wallet/check-live.sh", "endpoint"],
            cwd=ROOT,
            env={
                **os.environ,
                "CHECK_LIVE_ENDPOINT_BASE": base,
                "CHECK_LIVE_TIMEOUT": "2",
            },
            capture_output=True,
            text=True,
            check=False,
            timeout=10,
        )
    assert [path for path, _, _ in requests] == ["/"] * 4
    assert [request["method"] for _, request, _ in requests] == [
        "eth_chainId",
        "px_resolveAccount",
        "px_getNetwork",
        "px_getAccount",
    ]
    assert [request["params"] for _, request, _ in requests] == [
        [],
        [ACCOUNT_ADDRESS],
        [],
        [ACCOUNT_ADDRESS],
    ]
    for _, request, headers in requests:
        assert request["jsonrpc"] == "2.0"
        assert request["id"] == 1
        assert headers["content-type"] == "application/json"
    return checked


@pytest.mark.parametrize("path", ["/rpc", "/rpc/evm"])
def test_endpoint_fixture_refuses_retired_paths(path):
    with endpoint_server({"result": {}}) as (base, requests):
        request = urllib.request.Request(
            base + path,
            data=json.dumps(
                {"jsonrpc": "2.0", "id": 1, "method": "px_getNetwork", "params": []}
            ).encode(),
            headers={"Content-Type": "application/json"},
        )
        with pytest.raises(urllib.error.HTTPError) as refused:
            urllib.request.urlopen(request, timeout=2)
        assert refused.value.code == 404
    assert requests[0][0] == path


@pytest.mark.parametrize("consensus_available", [True, False])
@pytest.mark.parametrize("bound,height", [(True, 42), (False, 0)])
def test_endpoint_account_probe_passes_a_resolved_account(
    bound, height, consensus_available
):
    checked = run_endpoint_account_probe(
        {
            "result": {
                "account": {
                    "bound": bound,
                    "evm_address": ACCOUNT_ADDRESS.lower(),
                },
                "paxeer": {
                    "address": ACCOUNT_ADDRESS.upper(),
                    "balance": "0x1aF",
                    "nonce": "0x0",
                    "height": height,
                },
            }
        },
        consensus_available,
    )
    assert checked.returncode == 0, checked.stdout + checked.stderr
    assert (
        f"pass px_getAccount bound={str(bound).lower()} height={height}"
        in checked.stdout.splitlines()
    )
    consensus = "42" if consensus_available else "unavailable"
    assert f"pass px_getNetwork consensus={consensus}" in checked.stdout.splitlines()
    assert "check-live: all checks passed" in checked.stdout.splitlines()


@pytest.mark.parametrize(
    "code,data",
    [
        (-32001, {"code": "native_account_reverse_resolution_unsupported"}),
        (-32001, {"code": "paxeer_rpc_not_configured"}),
        (
            -32010,
            {"code": "kernel_unavailable", "backend": "kernel", "reason": "unconfigured"},
        ),
    ],
)
def test_endpoint_account_probe_fails_a_read_refusal(code, data):
    checked = run_endpoint_account_probe(
        {"error": {"code": code, "message": "Paxeer read unavailable", "data": data}}
    )
    assert checked.returncode == 1, checked.stdout + checked.stderr
    failure = next(
        line
        for line in checked.stdout.splitlines()
        if line.startswith("fail px_getAccount ")
    )
    assert f'"code":{code}' in failure
    assert data["code"] in failure
    assert "pass px_getAccount" not in checked.stdout
    assert "check-live: 1 check(s) failed" in checked.stdout.splitlines()


@contextmanager
def ramp_server(readiness_scope="local-journal", refusal_status=401):
    requests = []

    class RampHandler(http.server.BaseHTTPRequestHandler):
        def respond(self, status, document):
            body = json.dumps(document).encode()
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def do_GET(self):
            requests.append(
                ("GET", self.path, None, {key.lower(): value for key, value in self.headers.items()})
            )
            if self.path != "/readyz":
                self.send_error(404)
                return
            self.respond(
                200,
                {
                    "ready": True,
                    "readiness_scope": readiness_scope,
                    "external_custody": True,
                    "chain_id": 125,
                    "journal": {"ready": True},
                },
            )

        def do_POST(self):
            body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            requests.append(
                ("POST", self.path, body, {key.lower(): value for key, value in self.headers.items()})
            )
            if self.path not in ("/internal/v1/work", "/internal/v1/rebalances"):
                self.send_error(404)
                return
            self.respond(refusal_status, {"error": "operator_authentication_required"})

    with http.server.HTTPServer(("127.0.0.1", 0), RampHandler) as server:
        thread = threading.Thread(target=server.serve_forever)
        thread.start()
        try:
            yield f"http://127.0.0.1:{server.server_port}", requests
        finally:
            server.shutdown()
            thread.join()


def test_ramp_mode_routes_readiness_and_operator_refusals():
    with ramp_server() as (base, requests):
        checked = subprocess.run(
            ["bash", "scripts/wallet/check-live.sh", "ramp"],
            cwd=ROOT,
            env={**os.environ, "CHECK_LIVE_RAMP_BASE": base, "CHECK_LIVE_TIMEOUT": "2"},
            capture_output=True,
            text=True,
            check=False,
            timeout=10,
        )
    assert checked.returncode == 0, checked.stdout + checked.stderr
    assert checked.stdout.splitlines() == [
        "pass ramp /readyz",
        "pass ramp /internal/v1/work",
        "pass ramp /internal/v1/rebalances",
    ]
    assert [(method, path, body) for method, path, body, _ in requests] == [
        ("GET", "/readyz", None),
        ("POST", "/internal/v1/work", {}),
        ("POST", "/internal/v1/rebalances", {}),
    ]
    assert requests[0][3]["cache-control"] == "no-cache"
    for _, _, _, headers in requests[1:]:
        assert headers["content-type"] == "application/json"
        assert "authorization" not in headers


class WalletCheckLiveTest(unittest.TestCase):
    def test_check_live_names_no_lx_method(self):
        scan = subprocess.run(
            [
                "python3",
                "tools/paxeer-x/tests/retired_callers_test.py",
                "--component",
                "kernel-web",
                "--component",
                "layerxctl-port",
                "--path",
                "scripts/wallet/check-live.sh",
            ],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(scan.returncode, 0, scan.stdout + scan.stderr)

        usage = subprocess.run(
            ["bash", "scripts/wallet/check-live.sh"],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(usage.returncode, 2, usage.stdout + usage.stderr)
        self.assertIn(
            "usage: scripts/wallet/check-live.sh endpoint | attestors | gateway | "
            "cutover | human | human-session | ramp\n",
            usage.stderr,
        )


if __name__ == "__main__":
    unittest.main()
