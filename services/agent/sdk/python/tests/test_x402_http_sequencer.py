from __future__ import annotations

from copy import deepcopy
import json
import sqlite3
from dataclasses import replace
from hashlib import sha256

import pytest

from layerx_sdk.account_derivation import keccak256
from layerx_sdk.production import PlatformSdkError, SdkErrorCode
from layerx_sdk.revert import RevertReasonError
from layerx_sdk.verifier import SequencerReceiptVerificationError
from layerx_sdk.x402 import PaymentBindingError
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
from test_sequencer_receipt import _bytes, _key, _payment_vector
from test_x402_draw_sequencer import _receipt_rpc, _grant_inputs, _payment_terms
from test_ai_market_commit import commit_producer
from receipt_transport import recorded_payment_rpc


def _required(vector):
    terms = _payment_terms(vector)
    return {
        "x402Version": 2,
        "resource": {"url": "https://seller.example/resource"},
        "accepts": [
            {
                "scheme": "exact",
                "network": "eip155:125",
                "amount": terms["amount"],
                "asset": "0x" + terms["asset"],
                "payTo": "0x" + terms["pay_to"],
                "maxTimeoutSeconds": 30,
            }
        ],
    }


def _payment_header(required, signed):
    vector = _payment_vector()
    key = vector["pinnedKey"]
    return encode_header(
        {
            "x402Version": 2,
            "resource": required["resource"],
            "accepted": required["accepts"][0],
            "payload": {
                "format": "layerx-receipt-proof-v2",
                "verificationLevel": "receipted",
                "rawTransaction": vector["rawTransaction"],
                "signedReceipt": "0x" + signed.hex(),
                "receiptDigest": vector["receiptDigest"],
                "receiptKey": {
                    "keyId": key["keyId"],
                    "publicKey": key["publicKey"],
                    "firstHeight": hex(key["firstHeight"]),
                    "lastHeight": hex(key["lastHeight"]),
                },
            },
        }
    )


def test_seller_verifies_a_sequencer_receipt_through_receipt_keys(tmp_path):
    vector = _payment_vector()
    signed = _bytes(vector["signedReceipt"])
    raw = _bytes(vector["rawTransaction"])
    key = _key(vector)
    terms = _payment_terms(vector)
    assert vector["receiptJson"]["txHash"] == "0x" + keccak256(raw).hex()
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
    receipt_digest = _bytes(vector["receiptDigest"]).hex()
    assert settlement == {
        "success": True,
        "transaction": vector["receiptJson"]["txHash"],
        "network": "eip155:125",
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
    vector = _payment_vector()
    with recorded_payment_rpc(
        vector, raw_transaction=vector["rawTransaction"]
    ) as (rpc, exchanges):
        _verify_buyer_receipt(tmp_path, vector, rpc, exchanges)


def _verify_buyer_receipt(tmp_path, vector, rpc, exchanges):
    signed = _bytes(vector["signedReceipt"])
    raw = _bytes(vector["rawTransaction"])
    key = _key(vector)
    terms = _payment_terms(vector)
    required = _required(vector)
    activity = vector["receiptJson"]["txHash"]
    assert activity == "0x" + keccak256(raw).hex()
    authority = ConfiguredReceiptAuthority(raw)
    buyer = BuyerMiddleware(rpc, [key], authority, [("exact", "eip155:125")])
    payment_header = buyer.prepare(
        encode_header(required), "0x" + raw.hex(), activity, "0x" + terms["payer"]
    )
    assert decode_header(payment_header)["payload"] == decode_header(
        _payment_header(required, signed)
    )["payload"]
    assert exchanges == [
        ("eth_sendRawTransaction", ["0x" + raw.hex()]),
        ("pax_sequencerReceipt", [activity]),
    ]
    seller = SellerMiddleware(
        required, [key], authority, FulfillmentStore(tmp_path / "buyer.sqlite")
    )
    status, headers, resource = seller.handle("buyer", payment_header, lambda: b"paid")
    assert (status, resource) == (200, b"paid")
    verified = buyer.capture_settlement(headers["PAYMENT-RESPONSE"], payment_header)
    assert verified.level == "sequencer-signed"
    assert verified.canonical_bytes == signed
    assert "0x" + verified.receipt.activity_id.hex() == activity
    assert verified.receipt_digest == _bytes(vector["receiptDigest"])

    with pytest.raises(ValueError, match="^activity-mismatch$"):
        buyer.prepare(encode_header(required), vector["rawTransaction"], "0x" + "2a" * 32, "0x" + terms["payer"])
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
    with pytest.raises(RevertReasonError, match="LXP_NO_PAYER_GRANT") as raised:
        draw.register("buyer", grant_digest, raw, receive, body["idempotencyKey"])
    assert raised.value.retryable is False
    grant_seller = SellerMiddleware(
        grant_required,
        [key],
        authority,
        FulfillmentStore(tmp_path / "grant-buyer.sqlite"),
        draw=draw,
    )
    grant_releases = []

    def release_grant():
        grant_releases.append(True)
        return b"grant resource"

    with pytest.raises(RevertReasonError, match="LXP_NO_PAYER_GRANT") as raised:
        grant_seller.handle("buyer", grant_header, release_grant)
    assert raised.value.retryable is False
    assert grant_releases == []
    with sqlite3.connect(draw_path) as db:
        assert db.execute("SELECT * FROM grant_draws").fetchall() == []
    with sqlite3.connect(tmp_path / "grant-buyer.sqlite") as db:
        assert db.execute("SELECT * FROM payment_fulfillments").fetchall() == []
    recovered_draw = PreparedGrantDraws(
        draw_path, terms["pay_to"], 125, rpc, authority, [key]
    )
    with pytest.raises(RevertReasonError, match="LXP_NO_PAYER_GRANT"):
        recovered_draw("buyer", grant_digest, body, offer)
    grant_settlement = decode_header(headers["PAYMENT-RESPONSE"])
    grant_response = encode_header(grant_settlement)
    with pytest.raises(RevertReasonError, match="LXP_NO_PAYER_GRANT") as raised:
        buyer.capture_grant_settlement(grant_response, grant_header, activity)
    assert raised.value.retryable is False
    with pytest.raises(ValueError, match="^transaction-hash-mismatch$"):
        buyer.capture_grant_settlement(
            grant_response, grant_header, "0x" + "2a" * 32
        )
    with pytest.raises(ValueError, match="^invalid-rpc-hex$"):
        buyer.capture_grant_settlement(
            grant_response, grant_header, activity.removeprefix("0x")
        )

    for keys, check in (
        ([replace(key, key_id=b"\x2a" * 32)], "unpinned_key"),
        ([replace(key, public_key=b"\x2a" * 32)], "bad_signature"),
    ):
        refused = BuyerMiddleware(rpc, keys, authority, [("exact", "eip155:125")])
        for verify in (
            lambda: refused.prepare(
                encode_header(required), "0x" + raw.hex(), activity, "0x" + terms["payer"]
            ),
            lambda: refused.capture_settlement(
                headers["PAYMENT-RESPONSE"], payment_header
            ),
        ):
            with pytest.raises(SequencerReceiptVerificationError) as raised:
                verify()
            assert raised.value.check == check
            assert raised.value.code == SdkErrorCode.VERIFICATION_FAILURE
        with pytest.raises(RevertReasonError, match="LXP_NO_PAYER_GRANT") as raised:
            refused.capture_grant_settlement(grant_response, grant_header, activity)
        assert raised.value.retryable is False
    assert exchanges == [
        ("eth_sendRawTransaction", ["0x" + raw.hex()]),
        ("pax_sequencerReceipt", [activity]),
        ("eth_sendRawTransaction", ["0x" + raw.hex()]),
        ("pax_sequencerReceipt", [activity]),
        ("eth_sendRawTransaction", ["0x" + raw.hex()]),
        ("pax_sequencerReceipt", [activity]),
    ]


def test_http_committed_offer_waits_for_verified_inclusion_after_restart(tmp_path, commit_producer):
    from test_repair_22_31_147 import finality_bundle
    vector = _payment_vector()
    raw, signed, key = _bytes(vector["rawTransaction"]), _bytes(vector["signedReceipt"]), _key(vector)
    activity = vector["receiptJson"]["txHash"]
    terms = _payment_terms(vector)
    required = _required(vector)
    required["accepts"][0]["extra"] = {"layerx": {"commitment": "committed"}}
    produced = commit_producer(raw, int(vector["receiptJson"]["height"], 16) + 1)
    trust, evidence = finality_bundle(produced)
    path = tmp_path / "committed-buyer.sqlite"
    fulfilled = tmp_path / "committed-seller.sqlite"
    releases = []
    def release():
        releases.append(True)
        return b"committed resource"
    with _receipt_rpc(vector) as (rpc, node):
        authority = ConfiguredReceiptAuthority(raw, rpc=rpc, receipt_keys=(key,), commit_trust=trust, committed_evidence=evidence)
        # Reopening after each outcome proves only one submission crosses PaymentRpc.
        for rung in ("unknown", "submitted", "receipted"):
            if rung == "unknown":
                node.receipts.pop(activity)
            elif rung == "submitted":
                node.receipts[activity] = {"txHash": activity, "nonce": vector["receiptJson"]["nonce"], "rung": rung}
            else:
                node.add_recording(vector)
                node.receipts[activity]["rung"] = rung
                node.receipts[activity].pop("blockNumber")
                node.receipts[activity].pop("blockHash")
                assert node.receipts[activity]["receipt"] == vector["signedReceipt"]
            buyer = BuyerMiddleware(rpc, (key,), authority, [("exact", "eip155:125")], journal_path=path)
            assert buyer.prepare(encode_header(required), vector["rawTransaction"], activity, "0x" + terms["payer"]) is None
            buyer._submissions.close()
            with sqlite3.connect(path) as db:
                assert db.execute("SELECT tx_hash, canonical FROM buyer_submissions").fetchall() == [(activity, raw)]
        seller = SellerMiddleware(required, (key,), ConfiguredReceiptAuthority(raw), FulfillmentStore(fulfilled))
        committed_payload = decode_header(_payment_header(required, signed))
        committed_payload["payload"]["verificationLevel"] = "committed"
        with pytest.raises(PaymentBindingError):
            seller.handle("buyer", encode_header(committed_payload), release)
        assert releases == []
        with sqlite3.connect(fulfilled) as db:
            assert db.execute("SELECT * FROM payment_fulfillments").fetchall() == []
        node.receipts[activity] = {**vector["receiptJson"], "rung": "committed", "blockHash": "0x" + produced["header_hash"]}
        buyer = BuyerMiddleware(rpc, (key,), authority, [("exact", "eip155:125")], journal_path=path)
        header = buyer.prepare(encode_header(required), vector["rawTransaction"], activity, "0x" + terms["payer"])
        assert header is not None
        for reopen in range(2):
            seller = SellerMiddleware(required, (key,), authority, FulfillmentStore(fulfilled))
            status, headers, resource = seller.handle("buyer", header, release)
            assert (status, resource) == (200, b"committed resource")
            assert buyer.capture_settlement(headers["PAYMENT-RESPONSE"], header).canonical_bytes == signed
        assert releases == [True]
        sends = [request for request in node.requests if request["method"] == "eth_sendRawTransaction"]
        assert len(sends) == 1 and sends[0]["params"] == [vector["rawTransaction"]]
        buyer._submissions.close()


def test_http_emitted_receive_registration_cannot_release_retired_grant(tmp_path):
    vector = _payment_vector()
    required = _required(vector)
    key = _key(vector)
    with _receipt_rpc(vector) as (rpc, node):
        from test_repair_22_33_17 import draw_inputs
        receive, body, offer, values, canonical, actor = draw_inputs()
        grant_required = {**required, "accepts": [offer]}
        grant_header = grant_payment_header(grant_required, offer, receive.hex())
        grant_digest = sha256(json.dumps(decode_header(grant_header), sort_keys=True, separators=(",", ":")).encode()).hexdigest()
        draw_path = tmp_path / "buyer-draws.sqlite"
        draw = PreparedGrantDraws(draw_path, actor, 125, rpc, ConfiguredReceiptAuthority(canonical), [key])
        with pytest.raises(RevertReasonError, match="LXP_NO_PAYER_GRANT"):
            draw.register("buyer", grant_digest, canonical, receive, body["idempotencyKey"])
        grant_seller = SellerMiddleware(grant_required, [key], ConfiguredReceiptAuthority(canonical), FulfillmentStore(tmp_path / "grant-buyer.sqlite"), draw=draw)
        releases = []
        with pytest.raises(RevertReasonError, match="LXP_NO_PAYER_GRANT"):
            grant_seller.handle("buyer", grant_header, lambda: releases.append(True))
        assert releases == []
        with sqlite3.connect(draw_path) as db:
            assert db.execute("SELECT * FROM grant_draws").fetchall() == []
        assert node.requests == []
        with sqlite3.connect(tmp_path / "grant-buyer.sqlite") as db:
            assert db.execute("SELECT * FROM payment_fulfillments").fetchall() == []


@pytest.mark.parametrize("field,value", (
    ("format", "layerx-receipt-proof-v1"),
    ("verificationLevel", "submitted"),
    ("verificationLevel", "executed"),
    ("verificationLevel", "batched"),
    ("verificationLevel", "finalised"),
    ("verificationLevel", "finalized"),
    ("verificationLevel", "committed"),
    ("receiptDigest", "0x" + "2a" * 32),
    ("receiptDigest", "0x" + keccak256(_bytes(_payment_vector()["signedReceipt"])).hex()),
    ("rawTransaction", "0x02"),
    ("signedReceipt", "0x02"),
    ("receiptKey.keyId", "0x" + "2a" * 32),
    ("receiptKey.publicKey", "0x" + "2a" * 32),
    ("receiptKey.firstHeight", "0x0"),
    ("receiptKey.lastHeight", "0x0"),
))
def test_http_portable_proof_mutations_never_release(tmp_path, field, value):
    vector = _payment_vector()
    signed, raw, key = _bytes(vector["signedReceipt"]), _bytes(vector["rawTransaction"]), _key(vector)
    required = _required(vector)
    payment = decode_header(_payment_header(required, signed))
    proof = payment["payload"]
    if "." in field:
        parent, child = field.split(".")
        proof[parent][child] = value
    else:
        proof[field] = value
    path = tmp_path / "refused.sqlite"
    seller = SellerMiddleware(required, (key,), ConfiguredReceiptAuthority(raw), FulfillmentStore(path))
    releases = []
    with pytest.raises((ValueError, PlatformSdkError)):
        seller.handle("buyer", encode_header(payment), lambda: releases.append(True))
    assert releases == []
    with sqlite3.connect(path) as db:
        assert db.execute("SELECT * FROM payment_fulfillments").fetchall() == []


def test_http_offer_ledger_payer_and_reference_are_bound(tmp_path):
    vector = _payment_vector()
    signed, raw, key = _bytes(vector["signedReceipt"]), _bytes(vector["rawTransaction"]), _key(vector)
    required = _required(vector)
    terms = _payment_terms(vector)
    payment_log = next(log for log in vector["receiptJson"]["logs"]
                       if log["topics"][0] == "0x" + keccak256(b"PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)").hex())
    required["accepts"][0]["extra"] = {"layerx": {
        "commitment": "receipted", "payer": terms["ledger_payer"],
        "purposeHash": payment_log["topics"][1][2:],
    }}
    authority = ConfiguredReceiptAuthority(raw)
    for field in ("payer", "purposeHash"):
        changed = deepcopy(required)
        changed["accepts"][0]["extra"]["layerx"][field] = "2a" * 32
        path = tmp_path / (field + ".sqlite")
        seller = SellerMiddleware(changed, (key,), authority, FulfillmentStore(path))
        releases = []
        with pytest.raises(PaymentBindingError):
            seller.handle("buyer", _payment_header(changed, signed), lambda: releases.append(True))
        assert releases == []
        with sqlite3.connect(path) as db:
            assert db.execute("SELECT * FROM payment_fulfillments").fetchall() == []


def test_http_portable_node_json_authenticates_every_producer_field(tmp_path):
    vector = _payment_vector()
    signed, raw, key = _bytes(vector["signedReceipt"]), _bytes(vector["rawTransaction"]), _key(vector)
    required = _required(vector)
    payload = decode_header(_payment_header(required, signed))
    payload["payload"]["receiptJson"] = deepcopy(vector["receiptJson"])
    path = tmp_path / "node-json.sqlite"
    seller = SellerMiddleware(required, (key,), ConfiguredReceiptAuthority(raw), FulfillmentStore(path))
    releases = []

    def release():
        releases.append(True)
        return b"node resource"

    paid = seller.handle("buyer", encode_header(payload), release)
    assert (paid[0], paid[2]) == (200, b"node resource")
    for field, value in (("nonce", "0xffff"), ("from", "0x" + "2a" * 20),
                         ("returnData", "0x01"), ("feeAmount", "0xffff"),
                         ("paymentDenom", "altered"), ("signingKey", "0x" + "2a" * 32)):
        changed = deepcopy(payload)
        changed["payload"]["receiptJson"][field] = value
        with pytest.raises(ValueError, match="node-receipt-mismatch"):
            seller.handle("buyer", encode_header(changed), release)
    changed = deepcopy(payload)
    changed["payload"]["receiptJson"]["logs"][0]["data"] = "0x01"
    with pytest.raises(ValueError, match="node-receipt-mismatch"):
        seller.handle("buyer", encode_header(changed), release)
    assert releases == [True]
    with sqlite3.connect(path) as db:
        assert db.execute("SELECT count(*) FROM payment_fulfillments").fetchone() == (1,)
