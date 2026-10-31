from __future__ import annotations

import asyncio
import json
from dataclasses import asdict, replace
from hashlib import sha256
from pathlib import Path
from typing import Awaitable, Callable

import pytest
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat
from layerx_sdk.account_derivation import keccak256
from layerx_sdk.px import PxReceiptKeyEntry
from layerx_sdk.verifier import ReceiptKey, verify_sequencer_receipt

from layerx_fastapi.config import DeclaredConfig, read_declared_config
from layerx_fastapi.protocol import (
    PAYMENT_RESPONSE_HEADER,
    MiddlewareError,
    MiddlewareErrorCode,
    PaymentPayload,
    decode_settlement_header,
    encode_base64,
    encode_payment_payload_header,
    parse_layerx_evidence,
    verify_payment_receipt,
)
from layerx_fastapi.seller import (
    ProposedFulfillment,
    ReceiptPayloadAuthority,
    ReleasedDecision,
    SellerMiddleware,
    SellerSettlementRequest,
    SettlementSettled,
    StoredFulfillment,
)
from layerx_fastapi.signatures import LayerXSignatureVerifier

ROOT = Path(__file__).resolve().parents[4]


class FileFulfillments:
    def __init__(self, directory: Path) -> None:
        self.directory = directory

    async def fulfill(
        self, proposed: ProposedFulfillment, release: Callable[[], Awaitable[bytes]]
    ) -> StoredFulfillment[bytes]:
        path = self.directory / (proposed.idempotency_key + ".json")
        if not path.exists():
            resource = await release()
            with path.open("x", encoding="utf-8") as handle:
                json.dump(
                    {
                        "idempotency_key": proposed.idempotency_key,
                        "request_digest": proposed.request_digest,
                        "raw_transaction": proposed.raw_transaction.hex(),
                        "signed_receipt": proposed.signed_receipt.hex(),
                        "receipt_key": asdict(proposed.receipt_key),
                        "resource": resource.hex(),
                    },
                    handle,
                )
        stored = json.loads(path.read_text(encoding="utf-8"))
        return StoredFulfillment(
            idempotency_key=stored["idempotency_key"],
            request_digest=stored["request_digest"],
            raw_transaction=bytes.fromhex(stored["raw_transaction"]),
            signed_receipt=bytes.fromhex(stored["signed_receipt"]),
            receipt_key=PxReceiptKeyEntry(**stored["receipt_key"]),
            resource=bytes.fromhex(stored["resource"]),
        )


def _fixture() -> tuple[bytes, bytes, PxReceiptKeyEntry, str, DeclaredConfig]:
    vectors = json.loads(
        (ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text()
    )["cases"]
    vector = next(row for row in vectors if row["id"] == "V1-pax-transfer")
    raw = bytes.fromhex(vector["rawTransaction"][2:])
    signed = bytes.fromhex(vector["signedReceipt"][2:])
    pin = vector["pinnedKey"]
    key = PxReceiptKeyEntry(
        pin["keyId"][2:], pin["publicKey"][2:], pin["firstHeight"], pin["lastHeight"]
    )
    receipt = verify_sequencer_receipt(
        signed,
        raw,
        (
            ReceiptKey(
                bytes.fromhex(key.key_id), bytes.fromhex(key.public_key),
                key.first_height, key.last_height,
            ),
        ),
    ).receipt
    config = read_declared_config(
        {
            "LAYERX_PRINCIPAL": "merchant",
            "LAYERX_PROTECTED_PATH": "/paid",
            "LAYERX_RESOURCE_URL": "https://merchant.example/paid",
            "LAYERX_X402_SCHEME": "exact",
            "LAYERX_X402_NETWORK": "layerx:paxeer",
            "LAYERX_PRICE": str(receipt.amount),
            "LAYERX_ASSET": receipt.asset.hex(),
            "LAYERX_PAY_TO": receipt.to_account.hex(),
            "LAYERX_PAYMENT_TIMEOUT_SECONDS": "60",
            "LAYERX_RECEIPT_KEYS_JSON": json.dumps(
                [
                    {
                        "keyId": encode_base64(bytes.fromhex(key.key_id)),
                        "publicKey": encode_base64(bytes.fromhex(key.public_key)),
                        "firstHeight": key.first_height,
                        "lastHeight": key.last_height,
                    }
                ]
            ),
            "LAYERX_WEBHOOK_PATH": "/webhook",
            "LAYERX_WEBHOOK_PUBLIC_KEYS_JSON": json.dumps(
                {"webhook": encode_base64(bytes.fromhex(key.public_key))}
            ),
        }
    )
    return raw, signed, key, vector["receiptDigest"][2:], config


def _payload(
    config: DeclaredConfig, raw: bytes, signed: bytes, key: PxReceiptKeyEntry, digest: str
) -> PaymentPayload:
    return PaymentPayload(
        accepted=config.requirements,
        payload={
            "rawTransaction": encode_base64(raw),
            "signedReceipt": encode_base64(signed),
            "receiptDigest": digest,
            "receiptKey": {
                "keyId": key.key_id,
                "publicKey": key.public_key,
                "firstHeight": key.first_height,
                "lastHeight": key.last_height,
            },
            "verificationLevel": "sequencer-signed",
        },
    )


def test_seller_settles_signed_sequencer_receipt(tmp_path: Path) -> None:
    raw, signed, key, digest, config = _fixture()
    payload = _payload(config, raw, signed, key, digest)
    authority = ReceiptPayloadAuthority(config.receipt_keys)
    request = SellerSettlementRequest(
        principal=config.principal,
        payload=payload,
        requirements=config.requirements,
        idempotency_key="payment",
        request_digest="request",
    )
    settled = asyncio.run(authority.settle(request))
    assert isinstance(settled, SettlementSettled)
    assert settled.raw_transaction == raw
    assert settled.signed_receipt == signed
    assert settled.receipt_key == key
    assert not hasattr(settled, "authorized_batch")
    verified = verify_payment_receipt(
        settled.signed_receipt, settled.raw_transaction, (settled.receipt_key,), config.requirements
    )
    assert verified.receipt.activity_id == keccak256(raw)
    assert verified.receipt_digest.hex() == digest
    assert verified.height == 1000

    releases: list[bytes] = []

    async def release() -> bytes:
        resource = b"paid resource"
        releases.append(resource)
        return resource

    seller = SellerMiddleware(
        config.payment_required, authority, FileFulfillments(tmp_path), LayerXSignatureVerifier()
    )
    header = encode_payment_payload_header(payload)
    decision = asyncio.run(seller.handle(config.principal, header, release))
    assert isinstance(decision, ReleasedDecision)
    assert decision.resource == b"paid resource"
    assert decision.verification == verified
    assert decision.settlement.success
    assert decision.settlement.transaction == "0x" + keccak256(raw).hex()
    assert decision.settlement.amount == config.requirements.amount
    assert decode_settlement_header(decision.headers[PAYMENT_RESPONSE_HEADER]) == decision.settlement
    evidence = parse_layerx_evidence(decision.settlement.extensions["layerx"])
    assert evidence == parse_layerx_evidence(payload.payload)

    reopened = SellerMiddleware(
        config.payment_required, authority, FileFulfillments(tmp_path), LayerXSignatureVerifier()
    )
    assert asyncio.run(reopened.handle(config.principal, header, release)) == decision
    assert releases == [b"paid resource"]
    records = list(tmp_path.glob("*.json"))
    assert len(records) == 1
    stored = json.loads(records[0].read_text())
    assert stored["raw_transaction"] == raw.hex()
    assert stored["signed_receipt"] == signed.hex()
    assert stored["receipt_key"] == asdict(key)
    assert "authorized_batch" not in stored

    for changes in (
        {"rawTransaction": encode_base64(raw + b"\0")},
        {"signedReceipt": encode_base64(signed[:-1] + bytes([signed[-1] ^ 1]))},
        {"receiptDigest": "00" * 32},
        {"receiptKey": {**payload.payload["receiptKey"], "lastHeight": key.last_height + 1}},
    ):
        invalid = replace(payload, payload={**payload.payload, **changes})
        with pytest.raises(MiddlewareError) as raised:
            asyncio.run(seller.handle(config.principal, encode_payment_payload_header(invalid), release))
        assert raised.value.code is MiddlewareErrorCode.VERIFICATION_FAILURE
    assert releases == [b"paid resource"]
    assert list(tmp_path.glob("*.json")) == records

    stored["receipt_key"]["last_height"] += 1
    records[0].write_text(json.dumps(stored))
    with pytest.raises(MiddlewareError) as raised:
        asyncio.run(reopened.handle(config.principal, header, release))
    assert raised.value.code is MiddlewareErrorCode.FULFILLMENT_CONFLICT
    assert releases == [b"paid resource"]


def test_verify_payment_receipt_refuses_unpinned_key(tmp_path: Path) -> None:
    raw, signed, key, _, config = _fixture()
    secret = Ed25519PrivateKey.from_private_bytes(b"\x2a" * 32)
    unpinned_key = replace(
        key,
        key_id="2a" * 32,
        public_key=secret.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw).hex(),
    )
    changed = bytearray(signed)
    changed[-109:-77] = bytes.fromhex(unpinned_key.key_id)
    digest = sha256(b"LXP/v1/receipt\0" + bytes(changed[:-69]) + b"\0").digest()
    changed[-64:] = secret.sign(digest)
    unpinned = bytes(changed)
    assert verify_sequencer_receipt(
        unpinned,
        raw,
        (
            ReceiptKey(
                bytes.fromhex(unpinned_key.key_id), bytes.fromhex(unpinned_key.public_key),
                unpinned_key.first_height, unpinned_key.last_height,
            ),
        ),
    ).receipt_digest == digest
    with pytest.raises(MiddlewareError) as raised:
        verify_payment_receipt(unpinned, raw, config.receipt_keys, config.requirements)
    assert raised.value.code is MiddlewareErrorCode.VERIFICATION_FAILURE
    assert raised.value.__cause__.check == "unpinned_key"

    for keys, check in (
        ((), "unpinned_key"),
        ((replace(key, first_height=1001),), "key_out_of_range"),
        ((replace(key, last_height=999),), "key_out_of_range"),
        ((replace(key, public_key=unpinned_key.public_key),), "bad_signature"),
    ):
        with pytest.raises(MiddlewareError) as raised:
            verify_payment_receipt(signed, raw, keys, config.requirements)
        assert raised.value.code is MiddlewareErrorCode.VERIFICATION_FAILURE
        assert raised.value.__cause__.check == check
    for requirements in (
        replace(config.requirements, amount=str(int(config.requirements.amount) + 1)),
        replace(config.requirements, asset="2a" * 32),
        replace(config.requirements, pay_to="2a" * 32),
    ):
        with pytest.raises(MiddlewareError) as raised:
            verify_payment_receipt(signed, raw, config.receipt_keys, requirements)
        assert raised.value.code is MiddlewareErrorCode.VERIFICATION_FAILURE

    releases: list[bytes] = []

    async def release() -> bytes:
        releases.append(b"protected")
        return releases[-1]

    seller = SellerMiddleware(
        config.payment_required,
        ReceiptPayloadAuthority(config.receipt_keys),
        FileFulfillments(tmp_path),
        LayerXSignatureVerifier(),
    )
    payload = _payload(config, raw, unpinned, unpinned_key, digest.hex())
    with pytest.raises(MiddlewareError) as raised:
        asyncio.run(seller.handle(config.principal, encode_payment_payload_header(payload), release))
    assert raised.value.code is MiddlewareErrorCode.VERIFICATION_FAILURE
    assert releases == []
    assert list(tmp_path.iterdir()) == []
