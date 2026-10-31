from __future__ import annotations

import sqlite3
from dataclasses import replace
from hashlib import sha256

import pytest
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

from layerx_sdk.account_derivation import keccak256
from layerx_sdk.generated.receipt import ReceiptFailureCode
from layerx_sdk.production import PlatformSdkError, SdkErrorCode
from layerx_sdk.revert import RevertReasonError
from layerx_sdk.verifier import (
    ReceiptVerificationError,
    SequencerReceiptVerificationError,
)
from layerx_sdk.x402 import verify_payment_receipt, verify_sequencer_receipt
from layerx_sdk.x402_draw import PreparedGrantDraws
from layerx_sdk.x402_http import ConfiguredReceiptAuthority, PaymentEvidence
from layerx_sdk.x402_receive import (
    encode_receive,
    grant_authorization_message,
    receive_authorization_message,
)
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


class _ReceiptRpc:
    def __init__(self, vector):
        self.result = {
            "activity_id": _bytes(vector["receiptJson"]["txHash"]).hex(),
            "state": "completed",
            "commitment": "executed",
            "receipt": _bytes(vector["signedReceipt"]).hex(),
            "raw_transaction": _bytes(vector["rawTransaction"]).hex(),
            "returnData": vector["receiptJson"]["returnData"],
        }
        self.raw_transaction = _bytes(vector["rawTransaction"])
        self.submissions = []
        self.lookups = []

    def send(self, canonical_hex, commitment):
        assert bytes.fromhex(canonical_hex) == self.raw_transaction
        assert commitment == "executed"
        self.submissions.append((canonical_hex, commitment))
        return dict(self.result)

    def receipt(self, activity_id):
        assert activity_id == self.result["activity_id"]
        self.lookups.append(activity_id)
        return dict(self.result)


def _grant_inputs(vector):
    terms = _payment_terms(vector)
    key = "11" * 32
    purpose = sha256(b"sequencer-grant-purpose").hexdigest()
    context = sha256(b"sequencer-grant-context").hexdigest()
    grant_signer = Ed25519PrivateKey.generate()
    receiver_signer = Ed25519PrivateKey.generate()
    grant = {
        "grant_id": "22" * 32,
        "from": terms["payer"],
        "recipient": terms["pay_to"],
        "asset": terms["asset"],
        "per_draw_maximum": terms["amount"],
        "allowance": terms["amount"],
        "recurring": False,
        "window_length": "0",
        "expiration": str((1 << 64) - 1),
        "purpose_hash": purpose,
        "has_reference": False,
        "reference_hash": "00" * 32,
        "revocation_sequence": "0",
        "public_key": grant_signer.public_key().public_bytes_raw().hex(),
        "signature": "00" * 64,
    }
    grant["signature"] = grant_signer.sign(grant_authorization_message(grant)).hex()
    receive = {
        "from": terms["payer"],
        "to": terms["pay_to"],
        "asset": terms["asset"],
        "amount": terms["amount"],
        "grant_id": grant["grant_id"],
        "receiver_sequence": "0",
        "idempotency_key": key,
        "context_hash": context,
        "payer_grant": grant,
        "receiver_authorization": {
            "kind": 1,
            "controller": terms["pay_to"],
            "public_key": receiver_signer.public_key().public_bytes_raw().hex(),
            "signature": "00" * 64,
            "signed_context_hash": context,
            "network_id": 125,
            "protocol_version": 2,
        },
    }
    receive["receiver_authorization"]["signature"] = receiver_signer.sign(
        receive_authorization_message(receive)
    ).hex()
    offer = {
        "scheme": "metered",
        "network": "layerx:125",
        "amount": terms["amount"],
        "asset": terms["asset"],
        "payTo": terms["pay_to"],
        "maxTimeoutSeconds": 30,
        "extra": {
            "layerx": {
                "commitment": "executed",
                "payer": terms["payer"],
                "purposeHash": purpose,
            }
        },
    }
    wire = encode_receive(receive)
    return wire, {"receive": wire.hex(), "idempotencyKey": key}, offer


def test_grant_draws_verify_through_receipt_keys(tmp_path):
    vector = VECTORS[0]
    terms = _payment_terms(vector)
    signed = _bytes(vector["signedReceipt"])
    raw = _bytes(vector["rawTransaction"])
    key = _key(vector)
    activity = _bytes(vector["receiptJson"]["txHash"]).hex()
    request_digest = sha256(b"grant-draw-request").hexdigest()
    receive, body, offer = _grant_inputs(vector)
    rpc = _ReceiptRpc(vector)
    authority = ConfiguredReceiptAuthority(raw)
    path = tmp_path / "draws.sqlite"
    draws = PreparedGrantDraws(path, terms["pay_to"], 125, rpc, authority, [key])
    with sqlite3.connect(path) as db:
        db.execute(
            "INSERT INTO grant_draws (idempotency_key, principal, request_digest, canonical, receive, activity_id) VALUES (?, ?, ?, ?, ?, ?)",
            (body["idempotencyKey"], "buyer", request_digest, raw, receive, activity),
        )

    evidence = draws("buyer", request_digest, body, offer)
    assert evidence == PaymentEvidence(signed, raw)
    verified = verify_payment_receipt(
        evidence.canonical_receipt, evidence.raw_transaction, [key], **terms
    )
    assert verified.level == "sequencer-signed"
    assert verified.receipt.activity_id.hex() == activity
    assert verified.receipt_digest == _bytes(vector["receiptDigest"])
    assert rpc.submissions == [(raw.hex(), "executed")]
    with sqlite3.connect(path) as db:
        attempted, attempted_at = db.execute(
            "SELECT attempted, attempted_at FROM grant_draws WHERE idempotency_key = ?",
            (body["idempotencyKey"],),
        ).fetchone()
    assert attempted == 1
    assert type(attempted_at) is int

    recovered = PreparedGrantDraws(path, terms["pay_to"], 125, rpc, authority, [key])
    assert recovered("buyer", request_digest, body, offer) == evidence
    assert rpc.lookups == [activity]
    assert len(rpc.submissions) == 1
    for keys, check in (
        ([replace(key, key_id=b"\x2a" * 32)], "unpinned_key"),
        ([replace(key, public_key=b"\x2a" * 32)], "bad_signature"),
    ):
        refused = PreparedGrantDraws(path, terms["pay_to"], 125, rpc, authority, keys)
        with pytest.raises(SequencerReceiptVerificationError) as raised:
            refused("buyer", request_digest, body, offer)
        assert raised.value.check == check
        assert raised.value.code == SdkErrorCode.VERIFICATION_FAILURE
    assert len(rpc.submissions) == 1
