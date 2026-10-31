from __future__ import annotations

import base64
import json
import sqlite3
from dataclasses import replace
from hashlib import sha256

import pytest

from layerx_sdk.production import PlatformSdkError, SdkErrorCode
from layerx_sdk.verifier import SequencerReceiptVerificationError
from layerx_sdk.x402_draw import PreparedGrantDraws
from layerx_sdk.x402_http import (
    BuyerMiddleware,
    ConfiguredReceiptAuthority,
    FulfillmentStore,
    PaymentEvidence,
    SellerMiddleware,
    decode_header,
    encode_header,
    grant_payment_header,
)
from test_sequencer_receipt import VECTORS, _bytes, _key
from test_x402_draw_sequencer import _ReceiptRpc, _grant_inputs, _payment_terms


def _required(vector):
    terms = _payment_terms(vector)
    return {
        "x402Version": 2,
        "resource": {"url": "https://seller.example/resource"},
        "accepts": [
            {
                "scheme": "exact",
                "network": "layerx:125",
                "amount": terms["amount"],
                "asset": terms["asset"],
                "payTo": terms["pay_to"],
                "maxTimeoutSeconds": 30,
            }
        ],
    }


def _payment_header(required, signed):
    return encode_header(
        {
            "x402Version": 2,
            "resource": required["resource"],
            "accepted": required["accepts"][0],
            "payload": {
                "receipt": base64.b64encode(signed).decode(),
                "receiptDigest": sha256(b"LXP/v1/merkle-leaf\0" + signed).hexdigest(),
                "verificationLevel": "sequencer-signed",
            },
        }
    )


def test_seller_verifies_a_sequencer_receipt_through_receipt_keys(tmp_path):
    vector = VECTORS[0]
    signed = _bytes(vector["signedReceipt"])
    raw = _bytes(vector["rawTransaction"])
    key = _key(vector)
    terms = _payment_terms(vector)
    required = _required(vector)
    payment_header = _payment_header(required, signed)
    evidence = PaymentEvidence(signed, raw)
    resolutions = []

    def resolve_receipt(receipt, offer):
        assert receipt == signed
        assert offer == required["accepts"][0]
        resolutions.append(receipt)
        return evidence

    releases = []

    def release():
        releases.append("released")
        return b"paid resource"

    path = tmp_path / "fulfillments.sqlite"
    seller = SellerMiddleware(required, [key], resolve_receipt, FulfillmentStore(path))
    status, headers, body = seller.handle("buyer", None, release)
    assert status == 402
    assert decode_header(headers["PAYMENT-REQUIRED"]) == required
    assert json.loads(body) == required
    assert releases == []

    paid = seller.handle("buyer", payment_header, release)
    assert paid[0] == 200
    assert paid[2] == b"paid resource"
    settlement = decode_header(paid[1]["PAYMENT-RESPONSE"])
    receipt_digest = sha256(b"LXP/v1/merkle-leaf\0" + signed).hexdigest()
    assert settlement == {
        "success": True,
        "transaction": "lxp:" + receipt_digest,
        "network": "layerx:125",
        "amount": terms["amount"],
        "payer": terms["payer"],
        "extensions": {"layerx": decode_header(payment_header)["payload"]},
    }
    payload = decode_header(payment_header)
    request_digest = sha256(
        json.dumps(payload, sort_keys=True, separators=(",", ":")).encode()
    ).hexdigest()
    request_key = sha256(
        b"LayerX/middleware/x402/idempotency\0"
        + b"buyer"
        + bytes.fromhex(request_digest)
    ).hexdigest()
    with sqlite3.connect(path) as db:
        assert db.execute("SELECT * FROM payment_fulfillments").fetchall() == [
            (request_key, request_digest, receipt_digest, b"paid resource")
        ]
    recovered = SellerMiddleware(
        required, [key], resolve_receipt, FulfillmentStore(path)
    )
    assert recovered.handle("buyer", payment_header, release) == paid
    assert releases == ["released"]
    assert resolutions == [signed, signed]

    for name, changed_required, keys, error_type, check in (
        (
            "wrong-public-key",
            required,
            [replace(key, public_key=b"\x2a" * 32)],
            SequencerReceiptVerificationError,
            "bad_signature",
        ),
        (
            "unknown-key",
            required,
            [replace(key, key_id=b"\x2a" * 32)],
            SequencerReceiptVerificationError,
            "unpinned_key",
        ),
        (
            "altered-amount",
            {
                **required,
                "accepts": [
                    {**required["accepts"][0], "amount": str(int(terms["amount"]) + 1)}
                ],
            },
            [key],
            PlatformSdkError,
            None,
        ),
    ):
        refused_path = tmp_path / (name + ".sqlite")

        def resolve_refused(receipt, offer):
            assert receipt == signed
            assert offer == changed_required["accepts"][0]
            return evidence

        refused = SellerMiddleware(
            changed_required, keys, resolve_refused, FulfillmentStore(refused_path)
        )
        with pytest.raises(error_type) as raised:
            refused.handle("buyer", _payment_header(changed_required, signed), release)
        assert raised.value.code == SdkErrorCode.VERIFICATION_FAILURE
        if check is not None:
            assert raised.value.check == check
        with sqlite3.connect(refused_path) as db:
            assert db.execute("SELECT * FROM payment_fulfillments").fetchall() == []
    assert releases == ["released"]


def test_buyer_verifies_sequencer_receipts_through_receipt_keys(tmp_path):
    vector = VECTORS[0]
    signed = _bytes(vector["signedReceipt"])
    raw = _bytes(vector["rawTransaction"])
    key = _key(vector)
    terms = _payment_terms(vector)
    required = _required(vector)
    activity = _bytes(vector["receiptJson"]["txHash"]).hex()
    authority = ConfiguredReceiptAuthority(raw)
    rpc = _ReceiptRpc(vector)
    buyer = BuyerMiddleware(rpc, [key], authority, [("exact", "layerx:125")])
    payment_header = buyer.prepare(
        encode_header(required), raw.hex(), activity, terms["payer"]
    )
    assert decode_header(payment_header)["payload"] == decode_header(
        _payment_header(required, signed)
    )["payload"]
    assert rpc.submissions == [(raw.hex(), "executed")]
    seller = SellerMiddleware(
        required, [key], authority, FulfillmentStore(tmp_path / "buyer.sqlite")
    )
    status, headers, resource = seller.handle("buyer", payment_header, lambda: b"paid")
    assert (status, resource) == (200, b"paid")
    verified = buyer.capture_settlement(headers["PAYMENT-RESPONSE"], payment_header)
    assert verified.level == "sequencer-signed"
    assert verified.canonical_bytes == signed
    assert verified.receipt.activity_id.hex() == activity
    assert verified.receipt_digest == _bytes(vector["receiptDigest"])

    receive, body, offer = _grant_inputs(vector)
    grant_required = {**required, "accepts": [offer]}
    grant_header = grant_payment_header(grant_required, offer, receive.hex())

    draw_path = tmp_path / "buyer-draws.sqlite"
    draw = PreparedGrantDraws(draw_path, terms["pay_to"], 125, rpc, authority, [key])
    grant_digest = sha256(
        json.dumps(
            decode_header(grant_header), sort_keys=True, separators=(",", ":")
        ).encode()
    ).hexdigest()
    with sqlite3.connect(draw_path) as db:
        db.execute(
            "INSERT INTO grant_draws (idempotency_key, principal, request_digest, canonical, receive, activity_id) VALUES (?, ?, ?, ?, ?, ?)",
            (body["idempotencyKey"], "buyer", grant_digest, raw, receive, activity),
        )
    grant_seller = SellerMiddleware(
        grant_required,
        [key],
        authority,
        FulfillmentStore(tmp_path / "grant-buyer.sqlite"),
        draw=draw,
    )
    grant_status, grant_headers, grant_resource = grant_seller.handle(
        "buyer", grant_header, lambda: b"grant resource"
    )
    assert (grant_status, grant_resource) == (200, b"grant resource")
    grant_verified = buyer.capture_grant_settlement(
        grant_headers["PAYMENT-RESPONSE"], grant_header, activity
    )
    assert grant_verified == verified

    for keys, check in (
        ([replace(key, key_id=b"\x2a" * 32)], "unpinned_key"),
        ([replace(key, public_key=b"\x2a" * 32)], "bad_signature"),
    ):
        refused = BuyerMiddleware(rpc, keys, authority, [("exact", "layerx:125")])
        for verify in (
            lambda: refused.prepare(
                encode_header(required), raw.hex(), activity, terms["payer"]
            ),
            lambda: refused.capture_settlement(
                headers["PAYMENT-RESPONSE"], payment_header
            ),
            lambda: refused.capture_grant_settlement(
                grant_headers["PAYMENT-RESPONSE"], grant_header, activity
            ),
        ):
            with pytest.raises(SequencerReceiptVerificationError) as raised:
                verify()
            assert raised.value.check == check
            assert raised.value.code == SdkErrorCode.VERIFICATION_FAILURE
