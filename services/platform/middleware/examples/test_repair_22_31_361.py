import copy
import json
import os
import shlex
import subprocess
import sys
import threading
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[4]
EXAMPLES = Path(__file__).resolve().parent
sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))
from layerx_sdk.x402_http import decode_header, encode_header

VECTORS = json.loads(
    (ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text()
)
PAYMENT = next(row for row in VECTORS["cases"] if row["id"] == "V1-layerxledger-pay-exact")
PAYER = PAYMENT["receiptJson"]["from"][2:].zfill(64)
DID = "did:layerx:" + PAYER
GATEWAY_TOKEN = "example-gateway-bearer"
NODE_TOKEN = "example-node-bearer"
FAUCET_TOKEN = "example-faucet-bearer"


@pytest.fixture(scope="module")
def node_environment(tmp_path_factory):
    # Interpret the real workspace TypeScript sources at import time. This gate
    # has no TypeScript build target and must not depend on stale dist artifacts.
    loader = tmp_path_factory.mktemp("source-loader") / "loader.mjs"
    loader.write_text(
        '''import { registerHooks, stripTypeScriptTypes } from "node:module";
import { existsSync, readFileSync } from "node:fs";
import { fileURLToPath, pathToFileURL } from "node:url";
const root = pathToFileURL(process.env.EXAMPLE_REPO_ROOT + "/");
const seller = new URL("services/platform/middleware/seller/dist/index.js", root).href;
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "@sidiora/layerx-sdk") {
      return { url: new URL("services/agent/sdk/typescript/src/index.ts", root).href, shortCircuit: true };
    }
    if (specifier.startsWith(".") && context.parentURL?.startsWith("file:")) {
      const target = new URL(specifier, context.parentURL);
      if (target.href === seller) {
        return { url: new URL("services/platform/middleware/seller/src/index.ts", root).href, shortCircuit: true };
      }
      if (target.pathname.endsWith(".js")) {
        target.pathname = target.pathname.slice(0, -3) + ".ts";
        if (existsSync(fileURLToPath(target))) return { url: target.href, shortCircuit: true };
      }
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (url.startsWith(root.href) && url.endsWith(".ts")) {
      return { format: "module", source: stripTypeScriptTypes(readFileSync(new URL(url), "utf8"),
        { mode: "transform", sourceUrl: url }), shortCircuit: true };
    }
    return nextLoad(url, context);
  }
});
'''
    )
    return {
        "NODE_OPTIONS": f"--import={loader}",
        "NODE_NO_WARNINGS": "1",
        "EXAMPLE_REPO_ROOT": str(ROOT),
    }


def environment(node_environment):
    env = {key: value for key, value in os.environ.items() if not key.startswith("LAYERX_")}
    env.update(node_environment)
    env.update(
        PYTHONPATH=str(ROOT / "services/agent/sdk/python"),
        LAYERX_GATEWAY_TOKEN=GATEWAY_TOKEN,
        LAYERX_RPC_TOKEN=NODE_TOKEN,
        LAYERX_FAUCET_TOKEN=FAUCET_TOKEN,
    )
    return env


@contextmanager
def recorded_service(role, receipt=None):
    requests = []
    failures = []
    token = {"gateway": GATEWAY_TOKEN, "node": NODE_TOKEN, "faucet": FAUCET_TOKEN}[role]
    expected_path = {"gateway": "/rpc", "node": "/", "faucet": "/v1/faucet/claims"}[role]
    allowed = {
        "px_resolveAccount": [DID],
        "eth_getTransactionCount": [PAYMENT["receiptJson"]["from"], "pending"],
        "eth_sendRawTransaction": [PAYMENT["rawTransaction"]],
        "pax_sequencerReceipt": [PAYMENT["receiptJson"]["txHash"]],
    }

    class RecordedService(BaseHTTPRequestHandler):
        def do_POST(self):
            body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            requests.append({
                "path": self.path,
                "host": self.headers["Host"],
                "authorization": self.headers.get("Authorization"),
                "body": body,
            })
            if self.headers.get("Authorization") != f"Bearer {token}":
                self.send_error(401, "bearer required for this origin")
                return
            try:
                assert self.path == expected_path
                assert self.headers.get_content_type() == "application/json"
                assert self.headers["Host"] == f"127.0.0.1:{self.server.server_port}"
                if role == "faucet":
                    assert body == {"did": DID, "public_key": "11" * 32}
                    assert self.headers.get("Idempotency-Key", "").startswith("examples-python-")
                    payload = {"funded": True}
                else:
                    assert body["jsonrpc"] == "2.0"
                    assert type(body["id"]) is int
                    assert body["params"] == allowed[body["method"]]
                    if role == "gateway":
                        assert body["method"] == "px_resolveAccount"
                        result = {
                            "evm_address": PAYMENT["receiptJson"]["from"],
                            "pax_address": None,
                            "layerx_did": DID,
                            "layerx_account": PAYER,
                            "bound": True,
                        }
                    else:
                        assert body["method"] != "px_resolveAccount"
                        result = {
                            "eth_getTransactionCount": PAYMENT["receiptJson"]["nonce"],
                            "eth_sendRawTransaction": PAYMENT["receiptJson"]["txHash"],
                            "pax_sequencerReceipt": receipt,
                        }[body["method"]]
                    payload = {"jsonrpc": "2.0", "id": body["id"], "result": result}
            except (AssertionError, KeyError) as error:
                failures.append(repr(error))
                self.send_error(400, "request violates producer contract")
                return
            raw = json.dumps(payload).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)

        def log_message(self, format, *args):
            pass

    with ThreadingHTTPServer(("127.0.0.1", 0), RecordedService) as server:
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            yield f"http://127.0.0.1:{server.server_port}{expected_path}", requests
        finally:
            server.shutdown()
            thread.join(timeout=5)
            assert not thread.is_alive()
            assert failures == []


def example_inputs(tmp_path, purpose="matching", keys=None):
    log = PAYMENT["receiptJson"]["logs"][0]
    terms = {
        "commitment": "receipted",
        "payer": log["topics"][2][2:],
    }
    if purpose != "absent":
        terms["purposeHash"] = log["topics"][1][2:] if purpose == "matching" else "2a" * 32
    offer = {
        "x402Version": 2,
        "resource": {"url": "https://seller.example/paid"},
        "accepts": [{
            "scheme": "exact", "network": "eip155:125",
            "amount": str(int(log["data"][66:], 16)),
            "asset": log["data"][:66], "payTo": log["topics"][3],
            "maxTimeoutSeconds": 120, "extra": {"layerx": terms},
        }],
    }
    activity = tmp_path / "transaction.hex"
    required = tmp_path / "offer.header"
    pins = tmp_path / "receipt-keys.json"
    activity.write_text(PAYMENT["rawTransaction"] + "\n")
    required.write_text(encode_header(offer))
    assert decode_header(required.read_text()) == offer
    pins.write_text(json.dumps([PAYMENT["pinnedKey"]] if keys is None else keys))
    return ["--activity", str(activity), "--offer", str(required), "--receipt-keys", str(pins), "--payer", PAYER]


def run_example(language, gateway, node, arguments, env, use_environment=False):
    command = ["node", str(EXAMPLES / "public-rpc.mjs")] if language == "js" else [sys.executable, str(EXAMPLES / "public_rpc.py")]
    if use_environment:
        env.update(LAYERX_GATEWAY_URL=gateway, LAYERX_RPC_URL=node, LAYERX_DID=DID)
    else:
        command += ["--gateway", gateway, "--rpc", node, "--did", DID]
    return subprocess.run(command + arguments, env=env, cwd=ROOT, capture_output=True, text=True, timeout=30)


def assert_payment(result):
    assert result.returncode == 0, result.stderr
    assert result.stderr == ""
    assert [json.loads(line) for line in result.stdout.splitlines()] == [
        {"sequence": PAYMENT["receiptJson"]["nonce"]},
        {"transaction": PAYMENT["receiptJson"]["txHash"], "commitment": "receipted"},
    ]


def assert_route(gateway, node):
    assert [item["body"]["method"] for item in gateway] == ["px_resolveAccount"]
    assert [item["body"]["method"] for item in node] == [
        "eth_getTransactionCount", "eth_sendRawTransaction", "pax_sequencerReceipt",
    ]
    assert [item["body"]["id"] for item in gateway] == [1]
    assert [item["body"]["id"] for item in node] == [1, 2, 3]
    assert gateway[0]["host"] != node[0]["host"]
    assert all(item["path"] == "/rpc" and item["authorization"] == f"Bearer {GATEWAY_TOKEN}" for item in gateway)
    assert all(item["path"] == "/" and item["authorization"] == f"Bearer {NODE_TOKEN}" for item in node)


@pytest.mark.parametrize("language", ["js", "py"])
@pytest.mark.parametrize("use_environment", [False, True])
def test_gateway_node_origins_and_authentication(tmp_path, node_environment, language, use_environment):
    args = example_inputs(tmp_path)
    with recorded_service("gateway") as (gateway, gateway_requests), recorded_service("node", PAYMENT["receiptJson"]) as (node, node_requests):
        result = run_example(language, gateway, node, args, environment(node_environment), use_environment)
    assert_payment(result)
    assert_route(gateway_requests, node_requests)


@pytest.mark.parametrize("language", ["js", "py"])
@pytest.mark.parametrize("origin", ["gateway", "node"])
def test_origin_authentication_is_required(tmp_path, node_environment, language, origin):
    args = example_inputs(tmp_path)
    env = environment(node_environment)
    env["LAYERX_GATEWAY_TOKEN" if origin == "gateway" else "LAYERX_RPC_TOKEN"] = "wrong-origin-bearer"
    with recorded_service("gateway") as (gateway, gateway_requests), recorded_service("node", PAYMENT["receiptJson"]) as (node, node_requests):
        result = run_example(language, gateway, node, args, env)
    assert result.returncode == 1, result.stderr
    assert "rpc-http-401" in result.stderr
    assert '"transaction"' not in result.stdout
    assert len(gateway_requests) == 1
    assert [item["body"]["method"] for item in node_requests] == ([] if origin == "gateway" else ["eth_getTransactionCount"])


@pytest.mark.parametrize("language", ["js", "py"])
@pytest.mark.parametrize("purpose", ["absent", "mismatch"])
def test_exact_purpose_optional_and_bound_when_present(tmp_path, node_environment, language, purpose):
    args = example_inputs(tmp_path, purpose)
    with recorded_service("gateway") as (gateway, gateway_requests), recorded_service("node", PAYMENT["receiptJson"]) as (node, node_requests):
        result = run_example(language, gateway, node, args, environment(node_environment))
    assert_route(gateway_requests, node_requests)
    if purpose == "absent":
        assert_payment(result)
    else:
        assert result.returncode == 1, result.stderr
        assert "payment-binding-mismatch" in result.stderr if language == "py" else "verification-failure" in result.stderr
        assert [json.loads(line) for line in result.stdout.splitlines()] == [{"sequence": PAYMENT["receiptJson"]["nonce"]}]


@pytest.mark.parametrize("language", ["js", "py"])
@pytest.mark.parametrize("invalid", ["signature", "missing-range", "expired-key"])
def test_receipt_pins_and_signature_remain_required(tmp_path, node_environment, language, invalid):
    receipt = copy.deepcopy(PAYMENT["receiptJson"])
    keys = [copy.deepcopy(PAYMENT["pinnedKey"])]
    if invalid == "signature":
        signed = bytearray.fromhex(receipt["receipt"][2:])
        signed[-1] ^= 1
        receipt["receipt"] = "0x" + signed.hex()
    elif invalid == "missing-range":
        del keys[0]["lastHeight"]
    else:
        keys[0]["lastHeight"] = int(receipt["height"], 16) - 1
    args = example_inputs(tmp_path, "absent", keys)
    with recorded_service("gateway") as (gateway, gateway_requests), recorded_service("node", receipt) as (node, node_requests):
        result = run_example(language, gateway, node, args, environment(node_environment))
    assert result.returncode == 1, result.stderr
    assert '"transaction"' not in result.stdout
    if invalid == "missing-range":
        assert "receipt-key-id-range-and-public-key-required" in result.stderr
        assert [item["body"]["method"] for item in node_requests] == ["eth_getTransactionCount"]
    else:
        assert_route(gateway_requests, node_requests)
        assert "SequencerReceiptVerificationError" in result.stderr if language == "py" else "verification-failure" in result.stderr


@pytest.mark.parametrize("language", ["js", "py"])
@pytest.mark.parametrize("endpoint", ["gateway", "node"])
@pytest.mark.parametrize("invalid", ["credentials", "remote-http", "wrong-path", "query", "fragment"])
def test_endpoint_security_precedes_network(tmp_path, node_environment, language, endpoint, invalid):
    args = example_inputs(tmp_path)
    with recorded_service("gateway") as (gateway, gateway_requests), recorded_service("node", PAYMENT["receiptJson"]) as (node, node_requests):
        target = gateway if endpoint == "gateway" else node
        if invalid == "credentials":
            target = target.replace("http://", "http://user:password@")
        elif invalid == "remote-http":
            target = "http://node.example/" + ("rpc" if endpoint == "gateway" else "")
        elif invalid == "wrong-path":
            target = target.removesuffix("/rpc") + "/" if endpoint == "gateway" else target + "rpc"
        elif invalid == "query":
            target += "?token=value"
        else:
            target += "#fragment"
        result = run_example(language, target if endpoint == "gateway" else gateway, target if endpoint == "node" else node, args, environment(node_environment))
    assert result.returncode == 1
    assert "invalid-rpc-endpoint" in result.stderr
    assert gateway_requests == node_requests == []


def test_live_python_receipt_keys_invocation(tmp_path, node_environment):
    args = example_inputs(tmp_path, "absent")
    inputs = dict(zip(args[::2], args[1::2]))
    # Execute the exact shell invocation, including its variable expansion and
    # env contract, without running the live gate's unrelated workspace builds.
    script = (EXAMPLES / "examples-live-check.sh").read_text()
    invocation = script.split("run_example python ", 1)[1].split("\n\nprintf", 1)[0]
    assert '--receipt-keys "$LAYERX_EXAMPLE_PYTHON_RECEIPT_KEYS_FILE"' in invocation
    assert "--authority" not in invocation
    checker = (EXAMPLES / "examples-check.sh").read_text()
    assert "LAYERX_EXAMPLE_PYTHON_RECEIPT_KEYS_FILE" in checker
    assert "LAYERX_EXAMPLE_PYTHON_AUTHORITY_FILE" not in checker
    env = environment(node_environment)
    env.update(
        LAYERX_TEST_CA_FILE="/etc/ssl/certs/ca-certificates.crt",
        LAYERX_TEST_SOURCE_DID=DID,
        LAYERX_TEST_SOURCE_PUBLIC_KEY="11" * 32,
        LAYERX_EXAMPLE_PYTHON_ACTIVITY_FILE=inputs["--activity"],
        LAYERX_EXAMPLE_OFFER_FILE=inputs["--offer"],
        LAYERX_EXAMPLE_PYTHON_RECEIPT_KEYS_FILE=inputs["--receipt-keys"],
        LAYERX_EXAMPLE_PAYER=PAYER,
        stamp="qualification",
    )
    with recorded_service("gateway") as (gateway, gateway_requests), recorded_service("node", PAYMENT["receiptJson"]) as (node, node_requests), recorded_service("faucet") as (faucet, faucet_requests):
        env.update(gateway_url=gateway, rpc_url=node, faucet_url=faucet)
        result = subprocess.run(["sh", "-eu", "-c", invocation], env=env, cwd=ROOT, capture_output=True, text=True, timeout=30)
    assert result.returncode == 0, result.stderr
    assert result.stderr == ""
    assert result.stdout.splitlines()[0] == "Faucet HTTP status: 200; confirm the account balance before spending."
    payment_result = subprocess.CompletedProcess(result.args, result.returncode, "\n".join(result.stdout.splitlines()[1:]), result.stderr)
    assert_payment(payment_result)
    assert_route(gateway_requests, node_requests)
    assert len(faucet_requests) == 1


def test_live_environment_requires_python_receipt_pin_file(node_environment):
    script = (EXAMPLES / "examples-live-check.sh").read_text()
    required = [shlex.split(line)[1] for line in script.splitlines() if line.startswith("require LAYERX_")]
    assert "LAYERX_EXAMPLE_PYTHON_RECEIPT_KEYS_FILE" in required
    assert "LAYERX_EXAMPLE_PYTHON_AUTHORITY_FILE" not in required
    assert "LAYERX_RPC_URL" in required
    assert "LAYERX_TEST_NODE_AUTH_TOKEN_FILE" in required
    env = environment(node_environment)
    env["PLATFORM_BETA_CLUSTER_ENV"] = "/dev/null"
    result = subprocess.run(["sh", str(EXAMPLES / "examples-live-check.sh")], env=env, cwd=ROOT, capture_output=True, text=True, timeout=10)
    assert result.returncode == 2
    for name in required:
        assert f"missing input {name}:" in result.stderr
    assert f"{len(required)} missing input(s)" in result.stderr
