from __future__ import annotations

import sys
from pathlib import Path

import pytest

from layerx_sdk.evm_transaction import decode_signed_transaction
from layerx_sdk.verifier import ReceiptKey, verify_sequencer_receipt
from layerx_sdk.x402_rpc import (
    HostedGatewayRpc,
    PaymentRpc,
    PaymentRpcError,
    decode_receipt_keys,
    rpc_payment_evidence,
    verify_rpc_payment,
)

ROOT = Path(__file__).resolve().parents[5]
sys.path.insert(0, str(ROOT / "services/platform/middleware/conformance/pay6"))
from node_peer import node_peer


def test_rpc_surface_uses_the_public_producer_names():
    for producer in (HostedGatewayRpc, PaymentRpc, PaymentRpcError):
        assert producer.__module__ == "layerx_sdk.x402_rpc"
    assert HostedGatewayRpc.__name__ == "HostedGatewayRpc"
    assert PaymentRpc.__name__ == "PaymentRpc"
    assert PaymentRpcError.__name__ == "PaymentRpcError"
    error = PaymentRpcError(-32000, {"reason": "refused"})
    assert str(error) == "JSON-RPC error -32000"
    assert error.code == -32000
    assert error.data == {"reason": "refused"}


def test_rpc_submission_and_receipt_use_the_typescript_producer():
    with node_peer() as (endpoint, produced):
        proof = produced["proof"]
        assert set(proof) == {
            "format", "verificationLevel", "rawTransaction", "signedReceipt",
            "receiptDigest", "receiptKey",
        }
        assert proof["format"] == "layerx-receipt-proof-v2"
        raw = bytes.fromhex(proof["rawTransaction"][2:])
        transaction, _ = decode_signed_transaction(raw)
        assert transaction.chain_id == 125
        pin = proof["receiptKey"]
        keys = (ReceiptKey(
            bytes.fromhex(pin["keyId"][2:]),
            bytes.fromhex(pin["publicKey"][2:]),
            int(pin["firstHeight"], 16),
            int(pin["lastHeight"], 16),
        ),)
        rpc = PaymentRpc(endpoint)
        assert rpc.send(proof["rawTransaction"]) == produced["txHash"]
        result = rpc.receipt(produced["txHash"])
        assert result == produced["result"]
        verified = verify_sequencer_receipt(
            bytes.fromhex(proof["signedReceipt"][2:]), raw, keys,
        )
        assert proof["receiptDigest"] == "0x" + verified.receipt_digest.hex()
        assert result["receipt"] == proof["signedReceipt"]
        assert result["keyId"] == pin["keyId"]
        assert result["rung"] == proof["verificationLevel"] == "receipted"
        offer = produced["required"]["accepts"][0]
        payer = "0x" + result["from"][2:].zfill(64)
        assert verify_rpc_payment(
            result, produced["txHash"], payer, receipt_keys=keys,
            raw_transaction=raw, amount=offer["amount"], asset=offer["asset"],
            pay_to=offer["payTo"],
        ) == {
            "txHash": produced["txHash"], "rung": "receipted",
            "height": verified.height, "keyId": pin["keyId"],
        }
        for commitment, error_name in (
            ("receipted", "missing-committed-evidence"),
            ("committed", "payment-binding-mismatch"),
        ):
            with pytest.raises(ValueError, match=f"^{error_name}$"):
                verify_rpc_payment(
                    result | {"rung": "committed", "blockHash": "0x" + "12" * 32},
                    produced["txHash"], payer, receipt_keys=keys,
                    raw_transaction=raw, amount=offer["amount"], asset=offer["asset"],
                    pay_to=offer["payTo"], commitment=commitment,
                )
        evidence = rpc_payment_evidence(result, raw, keys)
        assert evidence.canonical_receipt == verified.canonical_bytes
        assert evidence.raw_transaction == raw
        assert evidence.finality is None
        assert decode_receipt_keys({"keys": [{
            **pin, "firstHeight": keys[0].first_height,
            "lastHeight": keys[0].last_height,
        }]}) == keys
        assert rpc._id == 2


def test_malformed_submissions_are_refused_before_a_request():
    with node_peer() as (endpoint, produced):
        rpc = PaymentRpc(endpoint)
        valid = produced["proof"]["rawTransaction"]
        malformed = (
            None, b"\x02", "", "00", "0x", "0x0", "0xgg", "0x02",
            "0x02c0", valid[:-2], valid + "00", "0x" + "00" * 524289,
        )
        for raw in malformed:
            with pytest.raises(ValueError, match="^invalid-rpc-submit$"):
                rpc.send(raw)
            assert rpc._id == 0
        assert rpc.send(valid) == produced["txHash"]
        assert rpc._id == 1


@pytest.mark.parametrize("method", (
    "px_getReceiptKeys", "px_resolveAccount", "eth_", "pax_", "eth_send-Raw",
    "eth_sendRawTransaction\n", "net_version", "", None,
))
def test_node_methods_are_refused_before_a_request(method):
    rpc = PaymentRpc("http://127.0.0.1:1/")
    with pytest.raises(ValueError, match="^invalid-node-method$"):
        rpc.call(method, [])
    assert rpc._id == 0


@pytest.mark.parametrize("method", (
    "eth_sendRawTransaction", "pax_sequencerReceipt", "px_", "px_get-Keys",
    "px_getReceiptKeys\n", "net_version", "", None,
))
def test_gateway_methods_are_refused_before_a_request(method):
    gateway = HostedGatewayRpc("http://127.0.0.1:1/rpc")
    with pytest.raises(ValueError, match="^invalid-gateway-method$"):
        gateway.call(method, [])
    assert gateway._id == 0


@pytest.mark.parametrize("client,path,wrong_path", (
    (PaymentRpc, "/", "/rpc"),
    (HostedGatewayRpc, "/rpc", "/"),
))
def test_rpc_clients_keep_their_own_endpoint_and_credentials(client, path, wrong_path):
    headers = {"Authorization": "Bearer local-test"}
    rpc = client("https://example.test" + path, headers)
    headers["Authorization"] = "Bearer changed"
    assert rpc.endpoint == "https://example.test" + path
    assert rpc.headers == {"Authorization": "Bearer local-test"}
    for endpoint in (
        "https://example.test" + wrong_path,
        "http://example.test" + path,
        "https://user:password@example.test" + path,
        "https://@example.test" + path,
        "https://example.test" + path + "?token=1",
        "https://example.test" + path + "#fragment",
        "https://example.test:65536" + path,
        "https://example.test\\other" + path,
        " https://example.test" + path,
        "https://example.test" + path + "\n",
    ):
        with pytest.raises(ValueError):
            client(endpoint, headers)
    for origin in ("http://localhost:8080", "http://127.0.0.1:8080", "http://[::1]:8080"):
        assert client(origin + path).endpoint == origin + path


def test_node_and_hosted_credentials_have_independent_owners():
    node = PaymentRpc("https://node.example.test/", {"Authorization": "Bearer node"})
    gateway = HostedGatewayRpc(
        "https://gateway.example.test/rpc", {"Authorization": "Bearer gateway"},
    )
    assert node.endpoint != gateway.endpoint
    assert node.headers == {"Authorization": "Bearer node"}
    assert gateway.headers == {"Authorization": "Bearer gateway"}
    assert node._opener is not gateway._opener
    assert node._id == gateway._id == 0
