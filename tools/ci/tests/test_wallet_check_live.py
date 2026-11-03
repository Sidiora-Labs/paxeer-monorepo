import http.server
import json
import os
import pathlib
import subprocess
import threading
import unittest

import pytest

ROOT = pathlib.Path(__file__).resolve().parents[3]
ACCOUNT_ADDRESS = "0x000000000000000000000000000000000000dEaD"


def run_endpoint_account_probe(account_answer):
    requests = []

    class EndpointHandler(http.server.BaseHTTPRequestHandler):
        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            requests.append((self.path, request))
            answers = {
                "eth_chainId": {"result": "0x7d"},
                "px_resolveAccount": {"result": {"bound": True}},
                "px_getNetwork": {
                    "result": {
                        "kernel": {"available": False, "reason": "not_configured"}
                    }
                },
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
            checked = subprocess.run(
                ["bash", "scripts/wallet/check-live.sh", "endpoint"],
                cwd=ROOT,
                env={
                    **os.environ,
                    "CHECK_LIVE_ENDPOINT_BASE": f"http://127.0.0.1:{server.server_port}",
                    "CHECK_LIVE_TIMEOUT": "2",
                },
                capture_output=True,
                text=True,
                check=False,
                timeout=10,
            )
        finally:
            server.shutdown()
            thread.join()
    assert [path for path, _ in requests] == ["/rpc"] * 4
    assert [request["method"] for _, request in requests] == [
        "eth_chainId",
        "px_resolveAccount",
        "px_getNetwork",
        "px_getAccount",
    ]
    assert requests[-1][1]["params"] == [ACCOUNT_ADDRESS]
    return checked


@pytest.mark.parametrize("bound,height", [(True, 42), (False, 0)])
def test_endpoint_account_probe_passes_a_resolved_account(bound, height):
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
        }
    )
    assert checked.returncode == 0, checked.stdout + checked.stderr
    assert (
        f"pass px_getAccount bound={str(bound).lower()} height={height}"
        in checked.stdout.splitlines()
    )
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
            "cutover | human | human-session\n",
            usage.stderr,
        )


if __name__ == "__main__":
    unittest.main()
