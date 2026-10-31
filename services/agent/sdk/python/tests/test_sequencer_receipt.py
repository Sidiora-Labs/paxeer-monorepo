from __future__ import annotations

import json
from dataclasses import FrozenInstanceError, replace
from hashlib import sha256
from pathlib import Path

import pytest
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

from layerx_sdk.production import SdkErrorCode
from layerx_sdk.verifier import (
    ReceiptKey,
    SequencerReceiptVerificationError,
    verify_sequencer_receipt,
)
from layerx_sdk.x402_rpc import verify_rpc_payment

ROOT = Path(__file__).resolve().parents[5]
VECTORS = json.loads(
    (ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text()
)["cases"]


def _bytes(value: str) -> bytes:
    return bytes.fromhex(value.removeprefix("0x"))


def _key(vector: dict) -> ReceiptKey:
    key = vector["pinnedKey"]
    return ReceiptKey(
        _bytes(key["keyId"]),
        _bytes(key["publicKey"]),
        key["firstHeight"],
        key["lastHeight"],
    )


def test_verify_sequencer_receipt_pinned_key():
    for vector in VECTORS:
        if vector["vector"] not in ("V1", "V2"):
            continue
        signed = _bytes(vector["signedReceipt"])
        raw = _bytes(vector["rawTransaction"])
        verified = verify_sequencer_receipt(signed, raw, [_key(vector)])
        assert verified.level == "sequencer-signed"
        assert verified.height == int(vector["receiptJson"]["height"], 16)
        assert verified.canonical_bytes == signed
        assert verified.receipt_digest == _bytes(vector["receiptDigest"])
        assert verified.receipt.activity_id == _bytes(vector["receiptJson"]["txHash"])

    vector = VECTORS[0]
    signed = _bytes(vector["signedReceipt"])
    raw = _bytes(vector["rawTransaction"])
    key = _key(vector)
    assert (
        verify_sequencer_receipt(
            signed, raw, [replace(key, first_height=1000, last_height=1000)]
        ).height
        == 1000
    )
    with pytest.raises(FrozenInstanceError):
        key.first_height = 0

    refusals = [
        (signed, raw, [], "unpinned_key"),
        (signed, raw, [replace(key, key_id=b"\x2a" * 32)], "unpinned_key"),
        (signed, raw, [replace(key, first_height=1001)], "key_out_of_range"),
        (signed, raw, [replace(key, last_height=999)], "key_out_of_range"),
        (signed, raw, [replace(key, first_height=-1)], "key_out_of_range"),
        (signed, raw, [replace(key, last_height=1 << 64)], "key_out_of_range"),
        (
            signed,
            raw,
            [replace(key, first_height=1001, last_height=999)],
            "key_out_of_range",
        ),
        (signed, raw[:-1] + bytes([raw[-1] ^ 1]), [key], "hash_mismatch"),
        (signed[:-1] + bytes([signed[-1] ^ 1]), raw, [key], "bad_signature"),
        (signed, raw, [replace(key, public_key=b"\x2a" * 32)], "bad_signature"),
        (signed, raw, [replace(key, public_key=b"\x2a" * 31)], "bad_signature"),
        (signed[:20], raw, [key], "bad_signature"),
        (signed + b"\0", raw, [key], "bad_signature"),
    ]
    for canonical, transaction, keys, check in refusals:
        with pytest.raises(SequencerReceiptVerificationError) as raised:
            verify_sequencer_receipt(canonical, transaction, keys)
        assert raised.value.check == check
        assert raised.value.code == SdkErrorCode.VERIFICATION_FAILURE

    assert (
        verify_sequencer_receipt(
            signed,
            raw,
            [replace(key, public_key=b"\x2a" * 32), key],
        ).canonical_bytes
        == signed
    )
    for field_offset, replacement in ((0, b"\0\x03"), (4, b"\0\x03")):
        changed = signed[:field_offset] + replacement + signed[field_offset + 2 :]
        with pytest.raises(SequencerReceiptVerificationError) as raised:
            verify_sequencer_receipt(changed, raw, [key])
        assert raised.value.check == "bad_signature"


def test_verify_rpc_payment_uses_receipt_keys():
    vector = next(vector for vector in VECTORS if vector["vector"] == "V1")
    raw = _bytes(vector["rawTransaction"])
    fixture_receipt = verify_sequencer_receipt(
        _bytes(vector["signedReceipt"]), raw, [_key(vector)]
    ).receipt
    payment_secret = Ed25519PrivateKey.from_private_bytes(b"\x31" * 32)
    key = ReceiptKey(
        b"k" * 32,
        payment_secret.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw),
        1000,
        1999,
    )
    canonical = bytearray(_bytes(vector["signedReceipt"]))
    canonical[-109:-77] = key.key_id
    digest = sha256(b"LXP/v1/receipt\0" + bytes(canonical[:-69]) + b"\0").digest()
    canonical[-64:] = payment_secret.sign(digest)
    signed = bytes(canonical)
    receipt = verify_sequencer_receipt(signed, raw, [key]).receipt
    result = {
        "state": "completed",
        "activity_id": receipt.activity_id.hex(),
        "receipt": signed.hex(),
    }
    terms = {
        "raw_transaction": raw,
        "amount": str(fixture_receipt.amount),
        "asset": fixture_receipt.asset.hex(),
        "pay_to": fixture_receipt.to_account.hex(),
    }

    def verify(row, keys=(key,), payer=fixture_receipt.from_account.hex(), **changes):
        return verify_rpc_payment(
            row,
            receipt.activity_id.hex(),
            payer,
            receipt_keys=keys,
            **{**terms, **changes},
        )

    payment = verify(result)
    assert payment.receipt == receipt
    assert payment.canonical_bytes == signed
    assert payment.receipt_digest == digest
    assert payment.receipt.module_id == fixture_receipt.module_id
    assert payment.receipt.operation == fixture_receipt.operation
    assert verify({**result, "returnData": "0x"}) == payment
    with pytest.raises(SequencerReceiptVerificationError) as raised:
        verify(result, keys=())
    assert raised.value.check == "unpinned_key"

    secret = Ed25519PrivateKey.from_private_bytes(b"\x2a" * 32)
    public_key = secret.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw)
    unpinned = bytearray(signed)
    unpinned[-109:-77] = b"\x2a" * 32
    unpinned[-64:] = secret.sign(
        sha256(b"LXP/v1/receipt\0" + bytes(unpinned[:-69]) + b"\0").digest()
    )
    unpinned_result = {**result, "receipt": bytes(unpinned).hex()}
    with pytest.raises(SequencerReceiptVerificationError) as raised:
        verify(unpinned_result)
    assert raised.value.check == "unpinned_key"
    other_key = ReceiptKey(b"\x2a" * 32, public_key, 1000, 1999)
    assert (
        verify(unpinned_result, keys=(other_key,)).canonical_bytes
        == bytes(unpinned)
    )

    with pytest.raises(SequencerReceiptVerificationError) as raised:
        verify(result, keys=(replace(key, first_height=1001),))
    assert raised.value.check == "key_out_of_range"
    with pytest.raises(SequencerReceiptVerificationError) as raised:
        verify(result, raw_transaction=raw + b"\0")
    assert raised.value.check == "hash_mismatch"
    for changes in (
        {"amount": str(receipt.amount + 1)},
        {"asset": "2a" * 32},
        {"pay_to": "2a" * 32},
    ):
        with pytest.raises(ValueError, match="^payment-binding-mismatch$"):
            verify(result, **changes)
    with pytest.raises(ValueError, match="^payment-binding-mismatch$"):
        verify({**result, "returnData": "0x2a"})
    with pytest.raises(ValueError, match="^payment-binding-mismatch$"):
        verify(result, payer="2a" * 32)
    altered_root = signed.replace(receipt.resulting_state_root, b"\x2a" * 32, 1)
    assert altered_root != signed
    with pytest.raises(SequencerReceiptVerificationError) as raised:
        verify({**result, "receipt": altered_root.hex()})
    assert raised.value.check == "bad_signature"
    assert verify({**result, "state": "pending"}) is None
