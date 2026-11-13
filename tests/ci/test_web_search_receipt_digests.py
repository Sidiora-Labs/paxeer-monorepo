"""Replay recorded web-search payments with the canonical receipt digest."""

from __future__ import annotations

import base64
import copy
import json
import sys
from dataclasses import asdict
from hashlib import sha256
from pathlib import Path
from urllib.parse import parse_qs, urlsplit

import pytest

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))
sys.path.insert(0, str(ROOT / "tests/agent/sdk/python"))

from test_repair_22_31_147 import WEB_FIXTURES, http_server, raw

from layerx_sdk.verifier import ReceiptKey, verify_sequencer_receipt
from layerx_sdk.web_search import (
    WebSearchAssetTerms,
    WebSearchClient,
    WebSearchError,
    WebSearchExactPayment,
    WebSearchPreference,
)
from layerx_sdk.x402_http import decode_header, encode_header


EXCHANGE = json.loads((WEB_FIXTURES / "client-exchange.json").read_text())["exchange"]
EXACT = json.loads((WEB_FIXTURES / "gateway/exact.json").read_text())
BUYER = json.loads((WEB_FIXTURES / "gateway/buyer.json").read_text())
RECEIPT_KEY = ReceiptKey(
    raw(BUYER["sequencerId"]),
    raw(BUYER["sequencerPublicKey"]),
    BUYER["firstBatchNumber"],
    BUYER["lastBatchNumber"],
)


@pytest.mark.parametrize(
    "index,currency,expected_digest",
    (
        (3, "SID", "f0592bfff13daab5a4e84608ce89e1116e9968d50fdd7bc2fdd50b28967c36a6"),
        (5, "PAX", "4cd761dedbeec7e06b571a458c18d7e4a892c59a19cc5d2f12874fd3ddcd749a"),
        (7, "USDC", "b27f1014cf967addb57bd2fce7ccb96e41ec8113e2da96c93314ccaa4699c398"),
        (9, "USDL", "33bda9005ca7fec26d97474f9766b9fa12e39847baec2b71cef667e09a3f00bd"),
    ),
)
def test_recorded_exact_receipt_digests_match_the_verifier(
    index, currency, expected_digest
):
    entry = EXCHANGE[index]
    payment = entry["request"]["headers"]["PAYMENT-SIGNATURE"]
    assert payment["accepted"]["extra"]["layerx"]["currency"] == currency
    recorded = EXACT["payments"][currency]
    layerx = entry["response"]["headers"]["PAYMENT-RESPONSE"]["extensions"]["layerx"]
    receipt = base64.b64decode(layerx["receipt"], validate=True)
    verified = verify_sequencer_receipt(
        receipt, raw(recorded["rawTransaction"]), (RECEIPT_KEY,)
    )
    digest = verified.receipt_digest.hex()
    assert digest == expected_digest
    assert payment["payload"]["receiptDigest"] == digest
    assert layerx["receiptDigest"] == digest
    assert recorded["receiptDigest"] == digest
    assert BUYER["exact"][currency]["receiptDigest"] == digest
    assert digest != sha256(b"LXP/v1/merkle-leaf\0" + receipt).hexdigest()


@pytest.mark.parametrize("response_digest", ("canonical", "merkle-leaf", "incorrect"))
def test_exact_payment_payload_matches_recorded_loopback_exchange(response_digest):
    unpaid, paid = EXCHANGE[4:6]
    expected = paid["request"]["headers"]["PAYMENT-SIGNATURE"]
    accepted = expected["accepted"]
    settlement = copy.deepcopy(paid["response"]["headers"]["PAYMENT-RESPONSE"])
    layerx = settlement["extensions"]["layerx"]
    receipt = base64.b64decode(layerx["receipt"], validate=True)
    transaction = raw(EXACT["payments"]["PAX"]["rawTransaction"])
    if response_digest == "merkle-leaf":
        layerx["receiptDigest"] = sha256(b"LXP/v1/merkle-leaf\0" + receipt).hexdigest()
    elif response_digest == "incorrect":
        layerx["receiptDigest"] = "00" * 32
    requests = []
    payments = []

    class RecordedPayer:
        did = None
        preferences = (WebSearchPreference("PAX", "exact"),)

        def exact(self, offer, target):
            assert target == paid["request"]["target"]
            assert offer.scheme == accepted["scheme"]
            assert offer.network == accepted["network"]
            assert offer.amount == accepted["amount"]
            assert offer.asset == accepted["asset"]
            assert offer.pay_to == accepted["payTo"]
            payments.append(target)
            return WebSearchExactPayment(receipt, transaction)

    def dispatch(method, target, headers, body):
        assert method == "GET" and body == b""
        assert target == paid["request"]["target"]
        header = headers.get("PAYMENT-SIGNATURE")
        requests.append(header)
        if len(requests) == 1:
            assert header is None
            response = unpaid["response"]
            return (
                response["status"],
                {name: encode_header(value) for name, value in response["headers"].items()},
                response["body"],
            )
        assert len(requests) == 2
        assert decode_header(header) == expected
        return (
            paid["response"]["status"],
            {"PAYMENT-RESPONSE": encode_header(settlement)},
            paid["response"]["body"],
        )

    with http_server(dispatch) as endpoint:
        client = WebSearchClient(
            endpoint,
            accepted["network"],
            accepted["asset"],
            {"PAX": WebSearchAssetTerms(accepted["asset"], int(accepted["amount"]))},
            RecordedPayer(),
            (RECEIPT_KEY,),
        )
        url = parse_qs(urlsplit(paid["request"]["target"]).query)["url"][0]
        if response_digest == "canonical":
            result = client.fetch(url)
            expected_body = paid["response"]["body"]
            for name in ("url", "final_url", "media_type", "digest", "text"):
                assert getattr(result, name) == expected_body[name]
            assert len(result.text.encode("utf-8")) == expected_body["length"]
            assert asdict(result.settlement) == {
                "scheme": accepted["scheme"],
                "currency": "PAX",
                "network": settlement["network"],
                "asset": accepted["asset"],
                "amount": settlement["amount"],
                "payer": settlement["payer"],
                "receipt_digest": expected["payload"]["receiptDigest"],
                "transaction": settlement["transaction"],
            }
        else:
            with pytest.raises(WebSearchError) as failure:
                client.fetch(url)
            assert failure.value.code == "settlement-receipt-mismatch"
    assert len(requests) == 2
    assert payments == [paid["request"]["target"]]
