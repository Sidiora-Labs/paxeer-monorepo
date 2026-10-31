from __future__ import annotations

from dataclasses import replace
from hashlib import sha256

import pytest

from layerx_sdk.account_derivation import keccak256
from layerx_sdk.generated.receipt import ReceiptFailureCode
from layerx_sdk.production import PlatformSdkError, SdkErrorCode
from layerx_sdk.revert import RevertReasonError
from layerx_sdk.verifier import (
    ReceiptVerificationError,
    SequencerReceiptVerificationError,
)
from layerx_sdk.x402 import verify_payment_receipt, verify_sequencer_receipt
from test_sequencer_receipt import VECTORS, _bytes, _key


def _payment_terms(vector: dict) -> dict[str, str]:
    return {
        "amount": str(10**18),
        "asset": sha256(b"uhpx").hexdigest(),
        "pay_to": _bytes(vector["receiptJson"]["to"]).rjust(32, b"\0").hex(),
        "payer": _bytes(vector["receiptJson"]["from"]).rjust(32, b"\0").hex(),
    }


def test_x402_draw_verifies_the_sequencer_receipt():
    for vector in VECTORS:
        if vector["vector"] != "V1":
            continue
        signed = _bytes(vector["signedReceipt"])
        raw = _bytes(vector["rawTransaction"])
        verified = verify_sequencer_receipt(signed, raw, [_key(vector)])
        assert verified.level == "sequencer-signed"
        assert verified.receipt.activity_id == keccak256(raw)
        assert verified.receipt.activity_id == _bytes(vector["receiptJson"]["txHash"])
        assert verified.canonical_bytes == signed
        assert verified.receipt_digest == _bytes(vector["receiptDigest"])
        assert verified.height == int(vector["receiptJson"]["height"], 16)
        assert verified.receipt.result_code == 0

    vector = VECTORS[0]
    signed = _bytes(vector["signedReceipt"])
    raw = _bytes(vector["rawTransaction"])
    key = _key(vector)
    terms = _payment_terms(vector)
    payment = verify_payment_receipt(signed, raw, (key,), return_data=b"", **terms)
    assert payment == verify_sequencer_receipt(signed, raw, [key])
    assert payment.receipt.amount == int(terms["amount"])
    assert payment.receipt.asset.hex() == terms["asset"]
    assert payment.receipt.to_account.hex() == terms["pay_to"]
    assert payment.receipt.from_account.hex() == terms["payer"]
    assert raw == _bytes(vector["rawTransaction"])

    for changes in (
        {"amount": str(10**18 + 1)},
        {"amount": "01"},
        {"asset": "2a" * 32},
        {"pay_to": "2a" * 32},
        {"payer": "2a" * 32},
        {"return_data": b"altered"},
    ):
        with pytest.raises(PlatformSdkError) as raised:
            verify_payment_receipt(signed, raw, [key], **{**terms, **changes})
        assert raised.value.code == SdkErrorCode.VERIFICATION_FAILURE

    for canonical, transaction, keys, check in (
        (signed, raw + b"\0", [key], "hash_mismatch"),
        (signed[:-1] + bytes([signed[-1] ^ 1]), raw, [key], "bad_signature"),
        (signed, raw, [replace(key, first_height=1001)], "key_out_of_range"),
    ):
        with pytest.raises(SequencerReceiptVerificationError) as raised:
            verify_payment_receipt(canonical, transaction, keys, **terms)
        assert raised.value.check == check

    reverted = next(vector for vector in VECTORS if vector["vector"] == "V2")
    signed = _bytes(reverted["signedReceipt"])
    raw = _bytes(reverted["rawTransaction"])
    with pytest.raises(ReceiptVerificationError) as raised:
        verify_sequencer_receipt(signed, raw, [_key(reverted)])
    assert raised.value.check == ReceiptFailureCode.RESULT_CODE
    with pytest.raises(RevertReasonError) as raised:
        verify_sequencer_receipt(
            signed,
            raw,
            [_key(reverted)],
            return_data=_bytes(reverted["receiptJson"]["returnData"]),
        )
    assert raised.value.code == "LXP_INSUFFICIENT_BALANCE"


def test_x402_draw_refuses_a_receipt_under_an_unknown_key():
    vector = VECTORS[0]
    signed = _bytes(vector["signedReceipt"])
    raw = _bytes(vector["rawTransaction"])
    key = _key(vector)
    unknown = replace(key, key_id=b"\x2a" * 32)
    assert unknown.public_key == key.public_key
    for keys in ([unknown], []):
        with pytest.raises(SequencerReceiptVerificationError) as raised:
            verify_sequencer_receipt(signed, raw, keys)
        assert raised.value.check == "unpinned_key"
        assert raised.value.code == SdkErrorCode.VERIFICATION_FAILURE
        with pytest.raises(SequencerReceiptVerificationError) as raised:
            verify_payment_receipt(signed, raw, keys, **_payment_terms(vector))
        assert raised.value.check == "unpinned_key"
        assert raised.value.code == SdkErrorCode.VERIFICATION_FAILURE
