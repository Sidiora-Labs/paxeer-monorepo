from __future__ import annotations

import asyncio
import json
import runpy
from base64 import b64encode
from dataclasses import fields, replace
from pathlib import Path

import pytest
from fastapi import FastAPI, Request
from fastapi.testclient import TestClient
from layerx_sdk.evm_transaction import decode_signed
from layerx_sdk.verifier import ReceiptKey, verify_sequencer_receipt

import layerx_fastapi
from layerx_fastapi import (
    LayerXMountOptions,
    LayerXReceiptEvidence,
    MiddlewareError,
    MiddlewareErrorCode,
    PaymentPayload,
    ReceiptPayloadAuthority,
    ReleasedDecision,
    SellerSettlementRequest,
    SingleProcessWebhookDeliveryStore,
    decode_payment_required_header,
    decode_settlement_header,
    encode_payment_payload_header,
    mount_layerx,
    parse_receipt_keys,
)

from layerx_fastapi.seller import layerx_evidence_wire


def mount_example_offer(
    example_path: Path, environment: dict[str, str], purpose_hash: str | None
) -> dict:
    example = runpy.run_path(str(example_path))
    app = FastAPI()
    mount = mount_layerx(
        app,
        LayerXMountOptions(
            resources=example["StaticResourceHandler"](
                Path(environment["LAYERX_RESOURCE_FILE"]).read_bytes()
            ),
            fulfillments=example["FileFulfillmentRepository"](
                Path(environment["LAYERX_FULFILLMENT_DIR"])
            ),
            deliveries=SingleProcessWebhookDeliveryStore(),
            events=example["RecordingEventHandler"](),
            environment=environment,
            offer_purpose_hash=purpose_hash,
        ),
    )
    return {**example, "app": app, "mount": mount}


def test_package_exports_no_authorized_batch_names() -> None:
    for name in ("AuthorizedBatchResolver", "StaticAuthorizedBatches"):
        assert name not in layerx_fastapi.__all__
        assert not hasattr(layerx_fastapi, name)
    assert "authorized_batches" not in {field.name for field in fields(LayerXMountOptions)}
    assert "parse_receipt_keys" in layerx_fastapi.__all__
    assert callable(parse_receipt_keys)
    assert "create_integration" in layerx_fastapi.__all__
    assert callable(layerx_fastapi.create_integration)


def test_create_integration_uses_receipt_keys(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    root = Path(__file__).resolve().parents[4]
    document = json.loads(
        (root / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text()
    )
    assert document["provenance"]["generator"] == "rpc/sequencer_receipt_vectors_test.go"
    assert document["chainId"] == 125
    vectors = document["cases"]
    vector = next(row for row in vectors if row["id"] == "V1-layerxledger-pay-exact")
    raw = bytes.fromhex(vector["rawTransaction"][2:])
    transaction, _ = decode_signed(raw)
    signed = bytes.fromhex(vector["signedReceipt"][2:])
    pin = vector["pinnedKey"]
    key_id = bytes.fromhex(pin["keyId"][2:])
    public_key = bytes.fromhex(pin["publicKey"][2:])
    receipt = verify_sequencer_receipt(
        signed, raw, (ReceiptKey(key_id, public_key, pin["firstHeight"], pin["lastHeight"]),)
    ).receipt
    keys_json = json.dumps({"keys": [{
        "keyId": pin["keyId"],
        "publicKey": pin["publicKey"],
        "firstHeight": hex(pin["firstHeight"]),
        "lastHeight": hex(pin["lastHeight"]),
    }]})
    resource_file = tmp_path / "resource.json"
    resource_file.write_bytes(b'{"paid":true}')
    environment = {
        "LAYERX_PRINCIPAL": "merchant",
        "LAYERX_PROTECTED_PATH": "/paid",
        "LAYERX_RESOURCE_URL": "https://merchant.example/paid",
        "LAYERX_X402_SCHEME": "exact",
        "LAYERX_X402_NETWORK": "eip155:125",
        "LAYERX_PRICE": str(int.from_bytes(transaction.data[100:132], "big")),
        "LAYERX_ASSET": transaction.data[68:100].hex(),
        "LAYERX_PAY_TO": transaction.data[36:68].hex(),
        "LAYERX_PAYMENT_TIMEOUT_SECONDS": "60",
        "LAYERX_RECEIPT_KEYS_JSON": keys_json,
        "LAYERX_WEBHOOK_PATH": "/webhook",
        "LAYERX_WEBHOOK_PUBLIC_KEYS_JSON": json.dumps(
            {"webhook": b64encode(public_key).decode("ascii")}
        ),
        "LAYERX_RESOURCE_FILE": str(resource_file),
        "LAYERX_FULFILLMENT_DIR": str(tmp_path / "fulfillments"),
    }
    for name, value in environment.items():
        monkeypatch.setenv(name, value)
    example_path = root / "sdk/integrations/fastapi/examples/paid_api.py"
    purpose_hash = transaction.data[164:196].hex()
    example = mount_example_offer(example_path, environment, purpose_hash)
    mount = example["mount"]
    authority = mount.seller._authority
    assert isinstance(authority, ReceiptPayloadAuthority)
    assert authority._receipt_keys is mount.config.receipt_keys
    assert mount.config.receipt_keys == parse_receipt_keys(keys_json)
    key = mount.config.receipt_keys[0]
    payload = PaymentPayload(
        accepted=mount.config.requirements,
        payload=layerx_evidence_wire(LayerXReceiptEvidence(
            raw_transaction=vector["rawTransaction"],
            signed_receipt=vector["signedReceipt"],
            receipt_digest=vector["receiptDigest"],
            receipt_key=key,
        )),
    )

    header = encode_payment_payload_header(payload)
    with TestClient(example["app"]) as client:
        challenge = client.get("/paid")
        assert challenge.status_code == 402
        assert challenge.json()["accepts"] == [mount.config.requirements.to_wire()]
        offer = decode_payment_required_header(challenge.headers["PAYMENT-REQUIRED"])
        assert offer.to_wire() == challenge.json()
        assert offer.accepts[0].extra["layerx"]["purposeHash"] == purpose_hash
        paid = client.get("/paid", headers={"PAYMENT-SIGNATURE": header})
        assert paid.status_code == 200, paid.text
        assert paid.content == resource_file.read_bytes()
        assert paid.headers["content-type"] == "application/json"
        settlement = decode_settlement_header(paid.headers["PAYMENT-RESPONSE"])
        assert settlement.success
        assert settlement.network == "eip155:125"
        assert settlement.transaction == "0x" + receipt.activity_id.hex()
        assert settlement.amount == mount.config.requirements.amount
        assert paid.headers["layerx-transaction"] == settlement.transaction
        assert paid.headers["layerx-receipt-digest"] == vector["receiptDigest"][2:]
        assert settlement.extensions["layerx"] == payload.payload
        replay_response = client.get("/paid", headers={"PAYMENT-SIGNATURE": header})
        assert replay_response.status_code == 200
        assert replay_response.content == paid.content
        assert replay_response.headers["PAYMENT-RESPONSE"] == paid.headers["PAYMENT-RESPONSE"]

    records = list((tmp_path / "fulfillments").glob("*.json"))
    assert len(records) == 1
    original_record = records[0].read_bytes()
    resource_file.write_bytes(b'{"paid":"changed after shutdown"}')
    reopened = mount_example_offer(example_path, environment, purpose_hash)
    assert reopened["mount"].seller is not mount.seller
    assert reopened["mount"].runtime.resources is not mount.runtime.resources
    with TestClient(reopened["app"]) as client:
        recovered = client.get("/paid", headers={"PAYMENT-SIGNATURE": header})
        assert recovered.status_code == 200
        assert recovered.content == paid.content
        assert recovered.headers["PAYMENT-RESPONSE"] == paid.headers["PAYMENT-RESPONSE"]
    assert records[0].read_bytes() == original_record
    assert list((tmp_path / "fulfillments").glob("*.json")) == records

    async def release() -> layerx_fastapi.LayerXResource:
        return await mount.runtime.resources.release(
            Request({"type": "http", "method": "GET", "path": "/paid", "headers": []})
        )

    decision = asyncio.run(
        mount.seller.handle(mount.config.principal, encode_payment_payload_header(payload), release)
    )
    assert isinstance(decision, ReleasedDecision)
    assert decision.resource.body == paid.content
    assert decision.verification.receipt_digest.hex() == vector["receiptDigest"][2:]
    replay = asyncio.run(
        mount.seller.handle(mount.config.principal, encode_payment_payload_header(payload), release)
    )
    assert replay == decision
    assert len(list((tmp_path / "fulfillments").glob("*.json"))) == 1

    unpinned = replace(
        payload,
        payload={
            **payload.payload,
            "receiptKey": {**payload.payload["receiptKey"], "lastHeight": hex(key.last_height + 1)},
        },
    )
    request = SellerSettlementRequest(
        principal=mount.config.principal,
        payload=unpinned,
        requirements=mount.config.requirements,
        idempotency_key="unpinned",
        request_digest="unpinned",
    )
    with pytest.raises(MiddlewareError) as raised:
        asyncio.run(authority.settle(request))
    assert raised.value.code is MiddlewareErrorCode.VERIFICATION_FAILURE
