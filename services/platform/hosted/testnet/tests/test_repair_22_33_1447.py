"""Production smoke parser, signer and TLS transports; loopback peers need no services."""

import ast
import copy
from contextlib import contextmanager
from datetime import datetime, timedelta, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
from pathlib import Path
import shlex
import ssl
from string import Template
import sys
import threading
import urllib.error
import urllib.request

import pytest
from cryptography import x509
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import ed25519
from cryptography.x509.oid import NameOID

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[4]
sys.path.insert(0, str(HERE))
from journey_common import JourneyRpc, RpcError, protected, root_endpoint, write
import smoke_accounts
from layerx_sdk.account_derivation import layerx_bind_message
from layerx_sdk.evm_transaction import decode_signed
from layerx_sdk.verifier import SequencerReceiptVerificationError, verify_sequencer_receipt
from layerx_sdk.x402_rpc import decode_receipt_keys


def load_module(name, filename):
    spec = importlib.util.spec_from_file_location(name, HERE / filename)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


payment = load_module("payment_m1447", "payment-journey.py")
wallet = load_module("wallet_m1447", "wallet-receipt-journey.py")


@pytest.fixture
def tls(tmp_path):
    key = ed25519.Ed25519PrivateKey.generate()
    subject = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, "localhost")])
    now = datetime.now(timezone.utc)
    cert = (
        x509.CertificateBuilder().subject_name(subject).issuer_name(subject)
        .public_key(key.public_key()).serial_number(x509.random_serial_number())
        .not_valid_before(now - timedelta(minutes=1)).not_valid_after(now + timedelta(hours=1))
        .add_extension(x509.SubjectAlternativeName([x509.DNSName("localhost")]), critical=False)
        .add_extension(x509.BasicConstraints(ca=True, path_length=None), critical=True)
        .sign(key, None)
    )
    ca = tmp_path / "ca.pem"
    ca.write_bytes(cert.public_bytes(serialization.Encoding.PEM))
    private = write(tmp_path / "tls.key", key.private_bytes(
        serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8,
        serialization.NoEncryption(),
    ))
    return ca, private


@contextmanager
def listener(tls, reply, expected_auth=None):
    """A strict TLS fixture peer; all routing and authentication is production code."""
    ca, private = tls
    requests, failures = [], []

    class Listener(BaseHTTPRequestHandler):
        def log_message(self, *_args):
            pass

        def do_POST(self):
            try:
                assert self.path == "/", self.path
                assert self.headers.get("Content-Type") == "application/json"
                body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                assert body["jsonrpc"] == "2.0" and type(body["id"]) is int
                auth = self.headers.get("Authorization")
                required = expected_auth(body) if callable(expected_auth) else expected_auth
                assert auth == required, (auth, required)
                requests.append((self.path, auth, body))
                value = reply(body)
                result = json.dumps({"jsonrpc": "2.0", "id": body["id"], **value}).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(result)))
                self.end_headers()
                self.wfile.write(result)
            except Exception as error:
                failures.append(error)
                self.send_error(400)

    server = ThreadingHTTPServer(("127.0.0.1", 0), Listener)
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.load_cert_chain(ca, private)
    server.socket = context.wrap_socket(server.socket, server_side=True)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield f"https://localhost:{server.server_port}/", requests
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
        assert not thread.is_alive()
        assert not failures, failures


@pytest.fixture
def material(tmp_path):
    directory = tmp_path / "private accounts"
    directory.mkdir(mode=0o700)
    for name in ("source", "destination"):
        key = ed25519.Ed25519PrivateKey.generate()
        write(directory / f"test-{name}-signer.key", key.private_bytes(
            serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8,
            serialization.NoEncryption(),
        ))
    document = smoke_accounts.generate_accounts(directory)
    gateway = write(directory / "gateway.curl", 'header = "Authorization: LayerX-Key fixture:lxp_live_1234"\n')
    source = write(directory / "source.curl", 'header = "Authorization: Bearer source_session"\n')
    destination = write(directory / "destination.curl", 'header = "Authorization: Bearer destination_session"\n')
    return directory, document, gateway, source, destination


def caller_arguments(environment):
    source = (HERE / "hosted-smoke.sh").read_text()
    block = source.split('/payment-journey.py" ', 1)[1].split('\nreceipt_id=', 1)[0]
    block = block.replace('"$(jq -er \'.evm_chain_id\' "$work/parameters.json")"', '"125"')
    return shlex.split(Template(block.replace('\\\n', '')).substitute(environment))


def test_TestRepairM_1447_smoke_removed_option(material, tls, tmp_path, capsys):
    directory, document, _, _, _ = material
    # Invoke the exact producer entry point used by beta-cluster, then expand the real caller.
    smoke_accounts.main(["env", str(directory)])
    exports = {}
    for line in capsys.readouterr().out.splitlines():
        command, assignment = shlex.split(line)
        assert command == "export"
        key, value = assignment.split("=", 1)
        exports[key] = value
    exports.update({
        "LAYERX_GATEWAY_URL": "https://localhost:19444",
        "LAYERX_TEST_NODE_URL": "https://localhost:19446",
        "LAYERX_TEST_CA_FILE": str(tls[0]),
        "auth_config": str(directory / "gateway.curl"),
        "LAYERX_TEST_SOURCE_DID": document["accounts"]["source"]["did"],
        "LAYERX_TEST_DESTINATION_DID": document["accounts"]["destination"]["did"],
        "LAYERX_TEST_AMOUNT": "1",
        "LAYERX_TEST_SEQUENCER_PUBLIC_KEY": "03" * 32,
        "work": str(tmp_path),
    })
    arguments = caller_arguments(exports)
    args = payment.parse_args(arguments)
    assert args.asset == payment.NATIVE_DENOM == "uhpx"
    assert args.node == exports["LAYERX_TEST_NODE_URL"]
    assert args.gateway == exports["LAYERX_GATEWAY_URL"]
    assert args.signer == directory / "test-source-evm.key"
    assert args.accounts == directory / "smoke-accounts.json"
    assert args.evm_chain_id == 125 and args.amount == "1"
    assert "--encoder" not in arguments
    with pytest.raises(SystemExit) as error:
        payment.parse_args(arguments + ["--encoder", "retired"])
    assert error.value.code == 2
    # Payment preconditions are reached, not replaced with parser-only success.
    wrong = arguments.copy()
    wrong[wrong.index("--asset") + 1] = "00" * 32
    with pytest.raises(AssertionError, match="native bank denomination required"):
        payment.main(wrong)
    producer = (ROOT / "services/platform/hosted/tests/beta-cluster.sh").read_text()
    assert 'smoke_accounts.py" generate "$d"' in producer
    assert 'smoke_accounts.py" env "$SECRETS_DIR"' in producer
    assert "LAYERX_TEST_SEND_ENCODER" not in producer
    assert "--encoder" not in (HERE / "hosted-smoke.sh").read_text()


def test_TestRepairM_1447_journey_router_path(material, tls, tmp_path):
    directory, document, gateway_auth, source_auth, destination_auth = material
    output = tmp_path / "routing"
    output.mkdir()
    receipt_document = json.loads((ROOT / "daemon/committed-fixture/vectors/vector-v4-committed.json").read_text())
    receipt = receipt_document["settlement"]["receipt"]
    node_results = {
        "eth_chainId": "0x7d", "eth_getBalance": "0x100000000000000",
        "eth_getTransactionCount": "0x0", "eth_gasPrice": "0x1",
        "eth_estimateGas": "0x5208", "eth_sendRawTransaction": receipt["txHash"],
        "pax_sequencerReceipt": receipt,
    }
    node_results["px_getBalances"] = {"account": document["accounts"]["source"]}
    hosted_results = {"px_requestFunds": {"funded": True}, "px_register": {"registered": True}}
    with listener(tls, lambda body: {"result": node_results[body["method"]]}) as (node, node_requests):
        with listener(tls, lambda body: {"result": hosted_results[body["method"]]},
                      "LayerX-Key fixture:lxp_live_1234") as (gateway, hosted_requests):
            client = JourneyRpc(gateway, tls[0], gateway_auth, output, node=node)
            for method, expected in node_results.items():
                assert client.rpc(method, []) == expected
            assert client.rpc("px_requestFunds", [document["accounts"]["source"]["did"]]) == hosted_results["px_requestFunds"]
            assert client.rpc("px_register", []) == hosted_results["px_register"]
            assert len(node_requests) == 8 and len(hosted_requests) == 2
            assert [r[2]["id"] for r in node_requests + hosted_requests] == list(range(1, 11))
            assert all(path == "/" and auth is None for path, auth, _ in node_requests)
            assert all(path == "/" and auth == "LayerX-Key fixture:lxp_live_1234"
                       for path, auth, _ in hosted_requests)
            for method in ("lx_submit", "eth_unknown", "px_unknown", "pax_unknown"):
                with pytest.raises(AssertionError):
                    client.rpc(method, [])
            with pytest.raises(AssertionError, match="hosted credentials"):
                client.rpc("eth_chainId", [], auth_config=source_auth)
            with pytest.raises(AssertionError, match="hosted credentials"):
                client.rpc("px_getBalances", [], auth_config=source_auth)
            assert len(node_requests) == 8 and len(hosted_requests) == 2
    # Optional node credentials remain independent of the gateway key.
    node_auth = write(directory / "node.curl", 'header = "Authorization: Bearer node_only"\n')
    with listener(tls, lambda body: {"result": "0x7d"}, "Bearer node_only") as (node, requests):
        node_output = tmp_path / "authenticated-node"
        node_output.mkdir()
        client = JourneyRpc("https://localhost:1", tls[0], gateway_auth, node_output,
                            node=node, node_auth_config=node_auth)
        assert client.rpc("eth_chainId", []) == "0x7d"
        assert len(requests) == 1
    gateway_source = (ROOT / "services/platform/hosted/gateway/src/main.rs").read_text()
    root_reads = (ROOT / "services/platform/hosted/gateway/src/public_reads.rs").read_text()
    assert 'matches!(request.path.as_str(), "/" | "/openrpc.json")' in root_reads
    assert 'public_reads::route(config, request)' in gateway_source
    locations = json.loads((ROOT / "services/platform/hosted/gateway/route-locations.json").read_text())
    assert locations['rpc_paths'] == ['/'] and locations['node_rpc']['chain_id'] == 125
    node_source = (ROOT / 'rpc/px_account.go').read_text()
    assert 'func (a *pxAccountAPI) GetBalances(' in node_source
    assert 'return nil, pxHosted("px_requestFunds")' in node_source
    assert 'if path == "/rpc"' in gateway_source


@pytest.mark.parametrize("endpoint", [
    "https://localhost/rpc", "https://localhost/?token=x", "https://user:secret@localhost",
    "https://localhost/#fragment", "http://localhost/",
])
def test_TestRepairM_1447_retired_endpoints_are_rejected(endpoint):
    with pytest.raises(AssertionError, match="HTTPS root endpoints"):
        root_endpoint(endpoint)


def test_TestRepairM_1447_smoke_signer_asset(material, tls, tmp_path):
    directory, document, gateway_auth, source_auth, destination_auth = material
    environment = smoke_accounts.smoke_environment(directory)
    key, source = payment.load_payment_signer(Path(environment["LAYERX_TEST_SOURCE_EVM_KEY_FILE"]))
    accounts = payment.validate_payment_accounts(
        Path(environment["LAYERX_TEST_ACCOUNTS_FILE"]),
        document["accounts"]["source"]["did"], document["accounts"]["destination"]["did"],
        source, environment["LAYERX_TEST_ASSET"], 125,
    )
    assert accounts == document["accounts"]
    for name, account in accounts.items():
        assert account["bind_to"] == "0x0000000000000000000000000000000000001004"
        public = bytes.fromhex(account["did"].removeprefix("did:layerx:"))
        calldata = bytes.fromhex(account["bind_data"][2:])
        assert calldata[:4].hex() == "dd9aa628" and calldata[4:36] == public
        assert account["bind_nonce"] == 0
        ed25519.Ed25519PublicKey.from_public_bytes(public).verify(
            calldata[-64:], layerx_bind_message(125, account["evm_address"], 0)
        )
        assert len(protected(directory / f"test-{name}-evm.key")) > 32
    with pytest.raises(AssertionError):
        payment.load_payment_signer(directory / "test-source-signer.key")
    with pytest.raises(AssertionError):
        payment.validate_payment_accounts(directory / "smoke-accounts.json",
            accounts["source"]["did"], accounts["destination"]["did"],
            accounts["destination"]["evm_address"], "uhpx", 125)

    # Fixture peer implements the declared funding result, with DID/session isolation.
    claims = []
    def funded(body):
        assert body["method"] == "px_requestFunds"
        did, public = body["params"]
        assert did == "did:layerx:" + public
        assert did in {account["did"] for account in accounts.values()}
        claims.append(did)
        return {"result": {"funded": True, "funding_id": "funding-" + public,
                           "transaction_id": "tx-" + public, "amount": "100"}}

    def session(body):
        name = "source" if body["params"][0] == accounts["source"]["did"] else "destination"
        return "Bearer " + name + "_session"

    output = tmp_path / "funding"
    output.mkdir()
    with listener(tls, funded, session) as (gateway, requests):
        client = JourneyRpc(gateway, tls[0], gateway_auth, output, node="https://localhost:1")
        result = payment.fund_accounts(client, [
            (accounts[name]["did"], bytes.fromhex(accounts[name]["did"].split(":")[-1]), auth)
            for name, auth in (("source", source_auth), ("destination", destination_auth))
        ])
        assert len(result) == len(requests) == len(claims) == 2
        assert claims == [accounts[name]["did"] for name in ("source", "destination")]
    data = payment.encode_call("sendNative", ["string"], ["pax1smokedestination"])
    raw = payment.sign_payment(125, 7, 67000000, 200000, payment.NATIVE_SCALE,
                               data, key, source)
    transaction, signature = decode_signed(raw)
    assert transaction.chain_id == 125 and transaction.nonce == 7
    assert transaction.value == 10**12 and transaction.data == data
    assert transaction.to == payment.hex_bytes(payment.BANK, 20)
    assert transaction.recover_signer(signature).lower() == accounts["source"]["evm_address"]
    assert transaction.max_fee_per_gas == 134000000
    assert transaction.max_priority_fee_per_gas == 0
    def accept_payment(body):
        assert body["method"] == "eth_sendRawTransaction"
        assert body["params"] == ["0x" + raw.hex()]
        received, received_signature = decode_signed(bytes.fromhex(body["params"][0][2:]))
        assert received == transaction
        assert received.recover_signer(received_signature).lower() == source
        return {"result": "0x" + payment.keccak256(raw).hex()}

    with listener(tls, accept_payment) as (node, requests):
        submitted = tmp_path / "submission"
        submitted.mkdir()
        client = JourneyRpc("https://localhost:1", tls[0], gateway_auth, submitted, node=node)
        assert client.rpc("eth_sendRawTransaction", ["0x" + raw.hex()]) == "0x" + payment.keccak256(raw).hex()
        assert len(requests) == 1
    # Existing receipt, balance, replay and nonce assertions remain in the real journey.
    main = (HERE / "payment-journey.py").read_text()
    assert 'source_before["account"]["evm_address"].lower() == source' in main
    assert 'verified.from_balance_before == source_wei_before' in main
    assert 'payment replay changed the destination balance' in main


@pytest.mark.parametrize("funding", [
    {"funded": False, "funding_id": "id", "transaction_id": "tx", "amount": "100"},
    {"funded": True, "funding_id": "", "transaction_id": "tx", "amount": "100"},
    {"funded": True, "funding_id": "id", "transaction_id": "", "amount": "100"},
    {"funded": True, "funding_id": "id", "transaction_id": "tx", "amount": "0"},
])
def test_TestRepairM_1447_funding_assertions_are_enforced(material, tls, tmp_path, funding):
    directory, document, gateway_auth, source_auth, _ = material
    output = tmp_path / "refused-funding"
    output.mkdir()
    with listener(tls, lambda body: {"result": funding}, "Bearer source_session") as (gateway, requests):
        client = JourneyRpc(gateway, tls[0], gateway_auth, output, node="https://localhost:1")
        did = document["accounts"]["source"]["did"]
        with pytest.raises(AssertionError):
            payment.fund_accounts(client, [(did, bytes.fromhex(did.split(":")[-1]), source_auth)])
        assert len(requests) == 1


def test_TestRepairM_1447_wallet_router_fixture(tls, material, tmp_path):
    _, _, auth, _, _ = material
    recorded = json.loads((ROOT / "daemon/committed-fixture/vectors/vector-v4-committed.json").read_text())
    receipt = recorded["settlement"]["receipt"]
    raw = bytes.fromhex(recorded["settlement"]["raw_transaction"][2:])
    keys = decode_receipt_keys({"keys": [{
        "keyId": receipt["keyId"], "publicKey": "0x" + bytes(recorded["authorization"]["PublicKey"]).hex(),
        "firstHeight": recorded["authorization"]["FirstBatchNumber"],
        "lastHeight": recorded["authorization"]["LastBatchNumber"],
    }]})
    original = copy.deepcopy(receipt)
    def receipt_reply(body):
        assert body["method"] == "pax_sequencerReceipt"
        assert body["params"] == [receipt["txHash"]]
        return {"result": receipt}

    with listener(tls, receipt_reply) as (node, requests):
        with wallet.WalletReceiptProxy(node, tls[0], tls[0], tls[1]) as proxy:
            assert proxy.endpoint.endswith("/") and not proxy.endpoint.endswith("/rpc")
            output = tmp_path / "wallet"
            output.mkdir()
            client = JourneyRpc("https://localhost:1", tls[0], auth, output, node=proxy.endpoint)
            for commitment in ("executed", "batched"):
                proxy.corrupt = False
                valid = client.rpc("pax_sequencerReceipt", [receipt["txHash"]])
                verified = verify_sequencer_receipt(bytes.fromhex(valid["receipt"][2:]), raw, keys)
                assert verified.receipt.result_code == 0
                assert verified.receipt.activity_id.hex() == receipt["txHash"][2:]
                proxy.corrupt = True
                corrupt = client.rpc("pax_sequencerReceipt", [receipt["txHash"]])
                assert {**corrupt, "receipt": valid["receipt"]} == valid
                assert corrupt["receipt"] != valid["receipt"]
                with pytest.raises(SequencerReceiptVerificationError) as error:
                    verify_sequencer_receipt(bytes.fromhex(corrupt["receipt"][2:]), raw, keys)
                assert error.value.check == "bad_signature", commitment
            assert len(proxy.mutations) == 2 and not proxy.rpc_failures
            assert proxy.forwarded_paths == [("/", "/")] * 4
            assert len(requests) == 4 and receipt == original
        assert not proxy.thread.is_alive()
    # The external integration still executes the actual wallet, with both result checks.
    tree = ast.parse((HERE / "wallet-receipt-journey.py").read_text())
    calls = [node for node in ast.walk(tree) if isinstance(node, ast.Call)]
    assert any(isinstance(call.func, ast.Attribute) and isinstance(call.func.value, ast.Name)
               and call.func.value.id == "subprocess" and call.func.attr == "run" for call in calls)
    source = (HERE / "wallet-receipt-journey.py").read_text()
    assert 'assert value["ok"] is (not corrupt), value' in source
    assert 'assert value["result"]["commitment"] == commitment' in source
    with pytest.raises(AssertionError, match="HTTPS root endpoints"):
        wallet.WalletReceiptProxy("https://localhost/rpc", tls[0], tls[0], tls[1])
