from __future__ import annotations

import asyncio
import json
import runpy
from base64 import b64encode
from dataclasses import fields, replace
from pathlib import Path

import pytest
from fastapi import Request
from layerx_sdk.verifier import ReceiptKey, verify_sequencer_receipt

import layerx_fastapi
from layerx_fastapi import (
    LayerXMountOptions,
    MiddlewareError,
    MiddlewareErrorCode,
    PaymentPayload,
    ReceiptPayloadAuthority,
    ReleasedDecision,
    SellerSettlementRequest,
    encode_payment_payload_header,
    parse_receipt_keys,
)


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
    vectors = json.loads(
        (root / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text()
    )["cases"]
    vector = next(row for row in vectors if row["id"] == "V1-pax-transfer")
    raw = bytes.fromhex(vector["rawTransaction"][2:])
    signed = bytes.fromhex(vector["signedReceipt"][2:])
    pin = vector["pinnedKey"]
    key_id = bytes.fromhex(pin["keyId"][2:])
    public_key = bytes.fromhex(pin["publicKey"][2:])
    receipt = verify_sequencer_receipt(
        signed, raw, (ReceiptKey(key_id, public_key, pin["firstHeight"], pin["lastHeight"]),)
    ).receipt
    keys_json = json.dumps(
        [
            {
                "keyId": b64encode(key_id).decode("ascii"),
                "publicKey": b64encode(public_key).decode("ascii"),
                "firstHeight": pin["firstHeight"],
                "lastHeight": pin["lastHeight"],
            }
        ]
    )
    resource_file = tmp_path / "resource.json"
    resource_file.write_bytes(b'{"paid":true}')
    environment = {
        "LAYERX_PRINCIPAL": "merchant",
        "LAYERX_PROTECTED_PATH": "/paid",
        "LAYERX_RESOURCE_URL": "https://merchant.example/paid",
        "LAYERX_X402_SCHEME": "exact",
        "LAYERX_X402_NETWORK": "layerx:paxeer",
        "LAYERX_PRICE": str(receipt.amount),
        "LAYERX_ASSET": receipt.asset.hex(),
        "LAYERX_PAY_TO": receipt.to_account.hex(),
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
    example = runpy.run_path(str(root / "sdk/integrations/fastapi/examples/paid_api.py"))
    mount = example["mount"]
    authority = mount.seller._authority
    assert isinstance(authority, ReceiptPayloadAuthority)
    assert authority._receipt_keys is mount.config.receipt_keys
    assert mount.config.receipt_keys == parse_receipt_keys(keys_json)
    key = mount.config.receipt_keys[0]
    payload = PaymentPayload(
        accepted=mount.config.requirements,
        payload={
            "rawTransaction": b64encode(raw).decode("ascii"),
            "signedReceipt": b64encode(signed).decode("ascii"),
            "receiptDigest": vector["receiptDigest"][2:],
            "receiptKey": {
                "keyId": key.key_id,
                "publicKey": key.public_key,
                "firstHeight": key.first_height,
                "lastHeight": key.last_height,
            },
            "verificationLevel": "sequencer-signed",
        },
    )

    async def release() -> layerx_fastapi.LayerXResource:
        return await mount.runtime.resources.release(
            Request({"type": "http", "method": "GET", "path": "/paid", "headers": []})
        )

    decision = asyncio.run(
        mount.seller.handle(mount.config.principal, encode_payment_payload_header(payload), release)
    )
    assert isinstance(decision, ReleasedDecision)
    assert decision.resource.body == resource_file.read_bytes()
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
            "receiptKey": {**payload.payload["receiptKey"], "lastHeight": key.last_height + 1},
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
