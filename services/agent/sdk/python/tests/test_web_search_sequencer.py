from __future__ import annotations

import base64
import json
from dataclasses import dataclass, replace
from hashlib import sha256
from pathlib import Path

import pytest
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

from layerx_sdk.verifier import (
    ReceiptKey,
    _decode_sequencer_receipt,
    verify_sequencer_receipt,
)
from layerx_sdk.web_search import (
    WebSearchAssetTerms,
    WebSearchClient,
    WebSearchError,
    WebSearchExactPayment,
    WebSearchMeteredPayment,
    WebSearchOffer,
    WebSearchPreference,
    _PreparedPayment,
)

ROOT = Path(__file__).resolve().parents[5]
VECTOR = json.loads(
    (ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text()
)["cases"][0]
SIGNED_RECEIPT = bytes.fromhex(VECTOR["signedReceipt"][2:])
RAW_TRANSACTION = bytes.fromhex(VECTOR["rawTransaction"][2:])
PIN = VECTOR["pinnedKey"]
RECEIPT_KEY = ReceiptKey(
    bytes.fromhex(PIN["keyId"][2:]),
    bytes.fromhex(PIN["publicKey"][2:]),
    PIN["firstHeight"],
    PIN["lastHeight"],
)
ASSET = "0e237584332fc3e1f1672d80a167a8d027597a64b34f4be18951e60babdd4f73"
PAYER = VECTOR["receiptJson"]["from"][2:].zfill(64)
OFFER = WebSearchOffer(
    scheme="exact",
    network="eip155:125",
    amount="1000000000000000000",
    asset=ASSET,
    pay_to=VECTOR["receiptJson"]["to"][2:].zfill(64),
    max_timeout_seconds=30,
    currency="PAX",
    account="agent:did:layerx:seller:main",
    payer=PAYER,
)
RECEIPT_DIGEST = sha256(b"LXP/v1/merkle-leaf\0" + SIGNED_RECEIPT).hexdigest()
SETTLEMENT = {
    "success": True,
    "network": OFFER.network,
    "amount": OFFER.amount,
    "payer": PAYER,
    "transaction": VECTOR["receiptJson"]["txHash"],
    "extensions": {
        "layerx": {
            "receipt": base64.b64encode(SIGNED_RECEIPT).decode("ascii"),
            "receiptDigest": RECEIPT_DIGEST,
            "verificationLevel": "receipted",
            "rawTransaction": VECTOR["rawTransaction"],
        }
    },
}


@dataclass(frozen=True)
class _ReceiptPayer:
    payment: WebSearchExactPayment
    did: str | None = None
    preferences: tuple[WebSearchPreference, ...] = (
        WebSearchPreference(currency="PAX", scheme="exact"),
    )

    def exact(self, offer: WebSearchOffer, target: str) -> WebSearchExactPayment:
        assert offer == OFFER
        assert target == "/search?q=paxeer"
        return self.payment

    def metered(self, offer: WebSearchOffer, target: str) -> WebSearchMeteredPayment:
        raise WebSearchError("metered-payer-unavailable")


def _client(receipt_keys: tuple[ReceiptKey, ...]) -> WebSearchClient:
    return WebSearchClient(
        endpoint="http://127.0.0.1",
        network=OFFER.network,
        native_asset=ASSET,
        assets={"PAX": WebSearchAssetTerms(ASSET, int(OFFER.amount))},
        payer=_ReceiptPayer(WebSearchExactPayment(SIGNED_RECEIPT, RAW_TRANSACTION)),
        receipt_keys=receipt_keys,
    )


def _header(settlement: dict) -> str:
    return base64.b64encode(json.dumps(settlement).encode("utf-8")).decode("ascii")


def test_web_search_receipt_verifies_through_receipt_keys():
    client = _client((RECEIPT_KEY,))
    payment = client._prepare(OFFER, {}, "/search?q=paxeer")
    payload = json.loads(base64.b64decode(payment.header))["payload"]
    assert payload == {
        "receipt": base64.b64encode(SIGNED_RECEIPT).decode("ascii"),
        "receiptDigest": RECEIPT_DIGEST,
        "verificationLevel": "sequencer-signed",
        "rawTransaction": VECTOR["rawTransaction"],
    }
    assert payment.receipt == SIGNED_RECEIPT
    assert payment.raw_transaction == RAW_TRANSACTION

    settlement = client._settle(_header(SETTLEMENT), OFFER, payment)
    assert settlement.transaction == VECTOR["receiptJson"]["txHash"]
    assert settlement.receipt_digest == RECEIPT_DIGEST
    assert settlement.payer == PAYER
    assert settlement.amount == OFFER.amount
    assert settlement.asset == OFFER.asset
    assert settlement.network == OFFER.network
    assert settlement.currency == "PAX"
    assert settlement.scheme == "exact"

    for raw, code in (
        (None, "settlement-receipt-mismatch"),
        ("0x", "settlement-receipt-mismatch"),
        ("0x0", "settlement-receipt-mismatch"),
        ("0xGG", "settlement-receipt-mismatch"),
        ("0x" + (RAW_TRANSACTION + b"\0").hex(), "receipt-unverified"),
    ):
        changed = {
            **SETTLEMENT,
            "extensions": {
                "layerx": {**SETTLEMENT["extensions"]["layerx"], "rawTransaction": raw}
            },
        }
        with pytest.raises(WebSearchError) as raised:
            client._settle(
                _header(changed), OFFER, replace(payment, raw_transaction=None)
            )
        assert raised.value.code == code

    changed = {
        **SETTLEMENT,
        "extensions": {
            "layerx": {
                **SETTLEMENT["extensions"]["layerx"],
                "rawTransaction": "0x" + (RAW_TRANSACTION + b"\0").hex(),
            }
        },
    }
    with pytest.raises(WebSearchError) as raised:
        client._settle(_header(changed), OFFER, payment)
    assert raised.value.code == "settlement-receipt-mismatch"


def test_web_search_refuses_a_receipt_under_an_unknown_key():
    payment = _client((RECEIPT_KEY,))._prepare(OFFER, {}, "/search?q=paxeer")
    unknown_key = replace(RECEIPT_KEY, key_id=b"\x2a" * 32)
    assert unknown_key.key_id != RECEIPT_KEY.key_id
    assert unknown_key.public_key == RECEIPT_KEY.public_key
    for pins in ((unknown_key,), ()):
        client = _client(pins)
        with pytest.raises(WebSearchError) as raised:
            client._prepare(OFFER, {}, "/search?q=paxeer")
        assert raised.value.code == "receipt-unverified"
        with pytest.raises(WebSearchError) as raised:
            client._settle(_header(SETTLEMENT), OFFER, payment)
        assert raised.value.code == "receipt-unverified"


def test_web_search_metered_settlement_accepts_the_sequencer_receipt():
    offer = replace(OFFER, scheme="metered", purpose_hash="2a" * 32)
    payment = _PreparedPayment(header="", receipt=None)
    response = {
        **SETTLEMENT,
        "extensions": {
            "layerx": {
                **SETTLEMENT["extensions"]["layerx"],
                "purposeHash": offer.purpose_hash,
            }
        },
    }
    verified = verify_sequencer_receipt(SIGNED_RECEIPT, RAW_TRANSACTION, (RECEIPT_KEY,))
    assert verified.receipt.module_id == 12
    assert verified.receipt.operation == 2

    client = _client((RECEIPT_KEY,))
    settlement = client._settle(_header(response), offer, payment)
    assert settlement.scheme == "metered"
    assert settlement.currency == offer.currency
    assert settlement.network == offer.network
    assert settlement.asset == offer.asset
    assert settlement.amount == offer.amount
    assert settlement.payer == offer.payer
    assert settlement.receipt_digest == RECEIPT_DIGEST
    assert settlement.transaction == VECTOR["receiptJson"]["txHash"]

    for field, value, code in (
        ("payer", "2b" * 32, "settlement-payer-mismatch"),
        ("purpose_hash", "2b" * 32, "settlement-purpose-mismatch"),
        ("amount", str(int(offer.amount) + 1), "receipt-offer-mismatch"),
        ("asset", "2b" * 32, "receipt-offer-mismatch"),
        ("pay_to", "2b" * 32, "receipt-offer-mismatch"),
    ):
        changed_offer = replace(offer, **{field: value})
        changed_response = {**response, "amount": changed_offer.amount}
        with pytest.raises(WebSearchError) as raised:
            client._settle(_header(changed_response), changed_offer, payment)
        assert raised.value.code == code


@pytest.mark.parametrize("scheme", ("exact", "metered"))
@pytest.mark.parametrize("module_id,operation", ((1, 6), (1, 2), (13, 2), (12, 6)))
def test_web_search_refuses_other_receipt_operations(scheme, module_id, operation):
    body, _ = _decode_sequencer_receipt(SIGNED_RECEIPT)
    summary_prefix = b"".join(
        (
            body.module_id.to_bytes(2, "big"),
            body.module_version.to_bytes(4, "big"),
            body.parameter_version.to_bytes(4, "big"),
            body.operation.to_bytes(1, "big"),
            len(body.asset).to_bytes(4, "big"),
            body.asset,
        )
    )
    assert SIGNED_RECEIPT.count(summary_prefix) == 1
    offset = SIGNED_RECEIPT.index(summary_prefix)
    receipt = bytearray(SIGNED_RECEIPT)
    receipt[offset : offset + 2] = module_id.to_bytes(2, "big")
    receipt[offset + 10] = operation
    _, unsigned = _decode_sequencer_receipt(bytes(receipt))
    signer = Ed25519PrivateKey.from_private_bytes(b"\x2a" * 32)
    receipt_key = replace(
        RECEIPT_KEY,
        public_key=signer.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw),
    )
    receipt[-64:] = signer.sign(sha256(b"LXP/v1/receipt\0" + unsigned).digest())
    signed = bytes(receipt)
    verified = verify_sequencer_receipt(signed, RAW_TRANSACTION, (receipt_key,))
    assert verified.receipt.module_id == module_id
    assert verified.receipt.operation == operation
    assert verified.receipt.amount == int(OFFER.amount)
    assert verified.receipt.asset.hex() == OFFER.asset
    assert verified.receipt.from_account.hex() == OFFER.payer
    assert verified.receipt.to_account.hex() == OFFER.pay_to

    offer = replace(OFFER, scheme=scheme, purpose_hash="2a" * 32)
    response = {
        **SETTLEMENT,
        "extensions": {
            "layerx": {
                **SETTLEMENT["extensions"]["layerx"],
                "receipt": base64.b64encode(signed).decode("ascii"),
                "receiptDigest": sha256(b"LXP/v1/merkle-leaf\0" + signed).hexdigest(),
                "purposeHash": offer.purpose_hash,
            }
        },
    }
    payment = _PreparedPayment(
        header="",
        receipt=signed if scheme == "exact" else None,
        raw_transaction=RAW_TRANSACTION if scheme == "exact" else None,
    )
    with pytest.raises(WebSearchError) as raised:
        _client((receipt_key,))._settle(_header(response), offer, payment)
    assert raised.value.code == "settlement-operation-mismatch"
