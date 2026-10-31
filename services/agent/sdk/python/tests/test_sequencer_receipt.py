from __future__ import annotations

import importlib.util
import json
from dataclasses import FrozenInstanceError, replace
from hashlib import sha256
from pathlib import Path

import pytest
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

from layerx_sdk.account_derivation import keccak256
from layerx_sdk.production import SdkErrorCode
from layerx_sdk.verifier import (
    AuthorizedReceiptBatch,
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


def _signatures():
    spec = importlib.util.spec_from_file_location(
        "sequencer_receipt_signatures",
        ROOT / "sdk/integrations/fastapi/layerx_fastapi/signatures.py",
    )
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module.LayerXSignatureVerifier()


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
    vector = VECTORS[0]
    raw = _bytes(vector["rawTransaction"])
    fixture = json.loads(
        (ROOT / "sdk/platform/conformance/fixtures/receipt-positive-v2.json").read_text()
    )
    batch = fixture["authorized_batch"]
    expected = fixture["expected"]
    payment_secret = Ed25519PrivateKey.from_private_bytes(b"\x31" * 32)
    key = ReceiptKey(
        b"k" * 32,
        payment_secret.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw),
        1000,
        1999,
    )
    canonical = bytearray.fromhex(fixture["canonical_receipt_hex"])
    batch_id = bytes.fromhex(batch["batch_id_hex"])
    assert canonical.count(batch_id) == 1
    batch_at = canonical.index(batch_id)
    canonical[10:42] = keccak256(raw)
    canonical[batch_at : batch_at + 32] = (1000).to_bytes(32, "big")
    canonical[batch_at + 38 : batch_at + 42] = bytes(4)
    canonical[-109:-77] = key.key_id
    digest = sha256(b"LXP/v1/receipt\0" + bytes(canonical[:-69]) + b"\0").digest()
    canonical[-64:] = payment_secret.sign(digest)
    signed = bytes(canonical)
    receipt = verify_sequencer_receipt(signed, raw, [key]).receipt
    authorized = AuthorizedReceiptBatch(
        (1000).to_bytes(32, "big"),
        bytes.fromhex(batch["asset_hex"]),
        bytes.fromhex(batch["previous_state_root_hex"]),
        bytes.fromhex(batch["resulting_state_root_hex"]),
        key.public_key,
    )
    result = {
        "state": "completed",
        "activity_id": receipt.activity_id.hex(),
        "receipt": signed.hex(),
    }
    terms = {
        "raw_transaction": raw,
        "amount": expected["amount"],
        "asset": batch["asset_hex"],
        "pay_to": expected["to_hex"],
    }
    signatures = _signatures()

    def verify(row, authority=authorized, keys=(key,), **changes):
        return verify_rpc_payment(
            row,
            receipt.activity_id.hex(),
            receipt.from_account.hex(),
            authority,
            signatures,
            receipt_keys=keys,
            **{**terms, **changes},
        )

    payment = verify(result)
    assert payment.receipt == receipt
    assert payment.canonical_bytes == signed
    assert payment.receipt_digest == digest
    assert payment.receipt.module_id == 1
    assert payment.receipt.operation == expected["operation"]
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
    unpinned_authority = replace(authorized, sequencer_public_key=public_key)
    with pytest.raises(SequencerReceiptVerificationError) as raised:
        verify(unpinned_result, authority=unpinned_authority)
    assert raised.value.check == "unpinned_key"
    other_key = ReceiptKey(b"\x2a" * 32, public_key, 1000, 1999)
    assert (
        verify(
            unpinned_result, authority=unpinned_authority, keys=(other_key,)
        ).canonical_bytes
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
        verify(result, authority=replace(authorized, resulting_state_root=b"\x2a" * 32))
    assert verify({**result, "state": "pending"}) is None
