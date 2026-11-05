from __future__ import annotations

import base64
import json
from dataclasses import dataclass, replace
from hashlib import sha256
from pathlib import Path

import pytest
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

from layerx_sdk.account_derivation import layerx_did_call
from layerx_sdk.payment_transaction import decode_payment_transaction
from layerx_sdk.revert import RevertReasonError
from layerx_sdk.x402 import PaymentBindingError
from receipt_transport import recorded_payment_rpc
from test_sequencer_receipt import _key, _payment_terms, _payment_vector

from layerx_sdk.verifier import (
    ReceiptKey,
    _decode_sequencer_receipt,
    verify_sequencer_receipt,
)
from layerx_sdk.web_search import (
    WebSearchAssetTerms, WebSearchClient, WebSearchError, WebSearchExactPayment,
    WebSearchMeteredPayment, WebSearchOffer, WebSearchPreference, _PreparedPayment,
    verify_sequencer_receipt as verify_web_payment,
)
from layerx_sdk.x402_rpc import PaymentRpc

ROOT = Path(__file__).resolve().parents[5]
WEB_REFUSAL_ORDER_FINDING = "web-payment-test-fixture.refusal-order"
VECTOR = _payment_vector()
SIGNED_RECEIPT = bytes.fromhex(VECTOR["signedReceipt"][2:])
RAW_TRANSACTION = bytes.fromhex(VECTOR["rawTransaction"][2:])
RECEIPT_KEY = _key(VECTOR)
TERMS = _payment_terms(VECTOR)
ASSET = TERMS["asset"]
PAYER = TERMS["payer"]
OFFER = WebSearchOffer(
    scheme="exact",
    network="eip155:125",
    amount=TERMS["amount"],
    asset=ASSET,
    pay_to=TERMS["pay_to"],
    max_timeout_seconds=30,
    currency="PAX",
    account="agent:did:layerx:seller:main",
    payer=PAYER,
)
RECEIPT_DIGEST = verify_sequencer_receipt(
    SIGNED_RECEIPT, RAW_TRANSACTION, (RECEIPT_KEY,)
).receipt_digest.hex()
SETTLEMENT = {
    "success": True, "network": OFFER.network, "amount": OFFER.amount, "payer": PAYER,
    "transaction": VECTOR["receiptJson"]["txHash"],
    "extensions": {
        "layerx": {
            "receipt": base64.b64encode(SIGNED_RECEIPT).decode("ascii"),
            "receiptDigest": RECEIPT_DIGEST,
            "verificationLevel": "receipted",
        }
    },
}


@dataclass(frozen=True)
class _ReceiptPayer:
    payment: WebSearchExactPayment
    did: str | None = None
    preferences: tuple[WebSearchPreference, ...] = (WebSearchPreference(currency="PAX", scheme="exact"),)

    def exact(self, offer: WebSearchOffer, target: str) -> WebSearchExactPayment:
        assert offer == OFFER and target == "/search?q=paxeer"
        return self.payment

    def metered(self, offer: WebSearchOffer, target: str) -> WebSearchMeteredPayment:
        raise WebSearchError("metered-payer-unavailable")


def _client(receipt_keys: tuple[ReceiptKey, ...], *, payer_rpc=None) -> WebSearchClient:
    return WebSearchClient(
        endpoint="http://127.0.0.1", network=OFFER.network, native_asset=ASSET,
        assets={"PAX": WebSearchAssetTerms(ASSET, int(OFFER.amount))},
        payer=_ReceiptPayer(WebSearchExactPayment(SIGNED_RECEIPT, RAW_TRANSACTION)),
        receipt_keys=receipt_keys,
        payer_rpc=payer_rpc,
        payer_address=VECTOR["receiptJson"]["from"] if payer_rpc is not None else None,
    )


def _header(value: dict) -> str:
    return base64.b64encode(json.dumps(value).encode()).decode()


def test_web_search_receipt_verifies_through_receipt_keys():
    client = _client((RECEIPT_KEY,))
    payment = client._prepare(OFFER, {}, "/search?q=paxeer")
    payload = json.loads(base64.b64decode(payment.header))["payload"]
    assert payload == {
        "txHash": VECTOR["receiptJson"]["txHash"],
        "receiptDigest": RECEIPT_DIGEST,
        "verificationLevel": "receipted",
        "idempotencyKey": VECTOR["receiptJson"]["logs"][1]["topics"][1][2:],
    }
    assert payment.receipt == SIGNED_RECEIPT and payment.raw_transaction == RAW_TRANSACTION
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
        (None, "settlement-transaction-unavailable"),
        ("0x", "settlement-transaction-unavailable"),
        ("0x0", "settlement-transaction-unavailable"),
        ("0xGG", "settlement-transaction-unavailable"),
        ("0x" + (RAW_TRANSACTION + b"\0").hex(), "receipt-unverified"),
    ):
        with recorded_payment_rpc(VECTOR, raw_transaction=raw) as (rpc, exchanges):
            lookup_client = _client((RECEIPT_KEY,), payer_rpc=rpc)
            with pytest.raises(WebSearchError) as raised:
                lookup_client._settle(
                    _header(SETTLEMENT), OFFER, replace(payment, raw_transaction=None)
                )
            assert raised.value.code == code
            assert exchanges == [
                ("eth_getRawTransactionByHash", [VECTOR["receiptJson"]["txHash"]])
            ]

    with recorded_payment_rpc(
        VECTOR, raw_transaction=VECTOR["rawTransaction"]
    ) as (rpc, exchanges):
        fetched = _client((RECEIPT_KEY,), payer_rpc=rpc)._settle(
            _header(SETTLEMENT), OFFER, replace(payment, raw_transaction=None)
        )
        assert fetched == settlement
        assert exchanges == [
            ("eth_getRawTransactionByHash", [VECTOR["receiptJson"]["txHash"]])
        ]
    with pytest.raises(WebSearchError) as raised:
        client._settle(_header(SETTLEMENT), OFFER, replace(payment, raw_transaction=None))
    assert raised.value.code == "settlement-transaction-unavailable"
    for raw in (b"", b"\0", RAW_TRANSACTION + b"\0"):
        with pytest.raises(WebSearchError) as raised:
            client._settle(
                _header(SETTLEMENT), OFFER, replace(payment, raw_transaction=raw)
            )
        assert raised.value.code == "receipt-unverified"
    with pytest.raises(WebSearchError) as raised:
        client._settle(_header({**SETTLEMENT, "payer": "11" * 32}), OFFER, payment)
    assert raised.value.code == "settlement-payer-mismatch"
    with pytest.raises(WebSearchError) as raised:
        client._settle(
            _header({**SETTLEMENT, "transaction": "0x" + "2a" * 32}), OFFER, payment
        )
    assert raised.value.code == "settlement-receipt-mismatch"


def test_web_search_refuses_a_receipt_under_an_unknown_key():
    payment = _client((RECEIPT_KEY,))._prepare(OFFER, {}, "/search?q=paxeer")
    unknown = replace(RECEIPT_KEY, key_id=b"\x2a" * 32)
    assert unknown.public_key == RECEIPT_KEY.public_key and unknown.key_id != RECEIPT_KEY.key_id
    for pins in ((unknown,), (), (replace(RECEIPT_KEY, public_key=b"\x2a" * 32),)):
        client = _client(pins)
        with pytest.raises(WebSearchError) as raised:
            client._prepare(OFFER, {}, "/search?q=paxeer")
        assert raised.value.code == "receipt-unverified"
        with pytest.raises(WebSearchError) as raised:
            client._settle(_header(SETTLEMENT), OFFER, payment)
        assert raised.value.code == "receipt-unverified"


def test_web_search_metered_settlement_accepts_the_sequencer_receipt():
    offer = replace(OFFER, scheme="metered", purpose_hash="2a" * 32)
    payment = _PreparedPayment(
        header="", receipt=SIGNED_RECEIPT, raw_transaction=RAW_TRANSACTION
    )
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
    settlement = client._settle(_header(response), OFFER, payment)
    assert settlement.scheme == "exact"
    assert settlement.currency == offer.currency
    assert settlement.network == offer.network
    assert settlement.asset == offer.asset
    assert settlement.amount == offer.amount
    assert settlement.payer == offer.payer
    assert settlement.receipt_digest == RECEIPT_DIGEST
    assert settlement.transaction == VECTOR["receiptJson"]["txHash"]

    for field, value in (
        ("payer", "2b" * 32),
        ("purpose_hash", "2b" * 32),
        ("amount", str(int(offer.amount) + 1)),
        ("asset", "2b" * 32),
        ("pay_to", "2b" * 32),
    ):
        changed_offer = replace(offer, **{field: value})
        changed_response = {**response, "amount": changed_offer.amount}
        # The retired scheme refuses every variant before receipt authority.
        with pytest.raises(WebSearchError) as raised:
            client._settle(_header(changed_response), changed_offer, payment)
        assert raised.value.code == "receipt-unverified"
        assert isinstance(raised.value.__cause__, RevertReasonError)
        assert raised.value.__cause__.code == "LXP_NO_PAYER_GRANT"
        assert raised.value.__cause__.retryable is False
        if field != "purpose_hash":
            with pytest.raises(WebSearchError) as raised:
                client._settle(
                    _header(changed_response), replace(changed_offer, scheme="exact"), payment
                )
            assert raised.value.code == "receipt-unverified"
            assert isinstance(raised.value.__cause__, PaymentBindingError)
            assert raised.value.__cause__.field == "payment"

    with pytest.raises(WebSearchError) as raised:
        client._settle(_header(response), offer, payment)
    assert raised.value.code == "receipt-unverified"
    assert isinstance(raised.value.__cause__, RevertReasonError)
    assert raised.value.__cause__.code == "LXP_NO_PAYER_GRANT"
    assert raised.value.__cause__.retryable is False
    with pytest.raises(WebSearchError) as raised:
        client._settle(_header({**response, "payer": "2b" * 32}), OFFER, payment)
    assert raised.value.code == "settlement-payer-mismatch"


@pytest.mark.parametrize("scheme", ("exact", "metered"))
@pytest.mark.parametrize("module_id,operation", ((1, 6), (1, 2), (13, 2), (12, 6)))
def test_web_search_refuses_other_receipt_operations(scheme, module_id, operation):
    body, _ = _decode_sequencer_receipt(SIGNED_RECEIPT)
    prefix = (body.module_id.to_bytes(2, "big") + body.module_version.to_bytes(4, "big")
              + body.parameter_version.to_bytes(4, "big") + bytes([body.operation])
              + len(body.asset).to_bytes(4, "big") + body.asset)
    assert SIGNED_RECEIPT.count(prefix) == 1
    offset = SIGNED_RECEIPT.index(prefix)
    receipt = bytearray(SIGNED_RECEIPT)
    receipt[offset:offset + 2] = module_id.to_bytes(2, "big")
    receipt[offset + 10] = operation
    _, unsigned = _decode_sequencer_receipt(bytes(receipt))
    signer = Ed25519PrivateKey.from_private_bytes(b"\x2a" * 32)
    receipt_key = replace(RECEIPT_KEY, public_key=signer.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw))
    receipt[-64:] = signer.sign(sha256(b"LXP/v1/receipt\0" + unsigned).digest())
    signed = bytes(receipt)
    verified = verify_sequencer_receipt(signed, RAW_TRANSACTION, (receipt_key,))
    assert verified.receipt.module_id == module_id
    assert verified.receipt.operation == operation
    original = verify_sequencer_receipt(SIGNED_RECEIPT, RAW_TRANSACTION, (RECEIPT_KEY,))
    assert verified.receipt.amount == original.receipt.amount
    assert verified.receipt.asset == original.receipt.asset
    assert verified.receipt.from_account == original.receipt.from_account
    assert verified.receipt.to_account == original.receipt.to_account
    assert _payment_terms(VECTOR) == TERMS

    offer = replace(OFFER, scheme=scheme, purpose_hash="2a" * 32)
    response = {
        **SETTLEMENT,
        "extensions": {
            "layerx": {
                **SETTLEMENT["extensions"]["layerx"],
                "receipt": base64.b64encode(signed).decode("ascii"),
                "receiptDigest": verified.receipt_digest.hex(),
                "purposeHash": offer.purpose_hash,
            }
        },
    }
    payment = _PreparedPayment(
        header="",
        receipt=signed if scheme == "exact" else None,
        raw_transaction=RAW_TRANSACTION,
    )
    with pytest.raises(WebSearchError) as raised:
        _client((receipt_key,))._settle(_header(response), offer, payment)
    assert raised.value.code == "receipt-unverified"
    if scheme == "exact":
        assert isinstance(raised.value.__cause__, PaymentBindingError)
        assert raised.value.__cause__.field == "payment"
    else:
        assert isinstance(raised.value.__cause__, RevertReasonError)
        assert raised.value.__cause__.code == "LXP_NO_PAYER_GRANT"
        assert raised.value.__cause__.retryable is False


@pytest.mark.parametrize("bad_key", (False, True))
def test_web_search_public_exact_exchange_replays_server_offer_and_pending(bad_key):
    from test_repair_22_31_147 import http_server, rotated_receipt_key
    fixtures = ROOT / "services/interop/crates/x-websearch/tests/fixtures"
    exchange = json.loads((fixtures / "client-exchange.json").read_text())["exchange"]
    recording = json.loads((fixtures / "gateway/exact.json").read_text())
    ledger = json.loads((ROOT / "services/agent/sdk/typescript/test/recorded-web-ledger.json").read_text())
    address = exchange[0]["request"]["headers"]["LAYERX-PAYER-ADDRESS"]
    did = "did:layerx:" + "00" * 12 + address[2:]
    required = exchange[2]["response"]["headers"]["PAYMENT-REQUIRED"]
    accepted = next(offer for offer in required["accepts"] if offer["extra"]["layerx"]["currency"] == "SID")
    payment = recording["payments"]["SID"]
    result = next(rule["result"] for rule in recording["endpoints"]["gateway"]["*"]
                  if rule["method"] == "pax_sequencerReceipt" and rule["params"] == [payment["txHash"]])
    requests, account_reads = [], []
    class Wallet:
        preferences = (WebSearchPreference("SID", "exact"),)
        def __init__(self):
            self.did = did
        def exact(self, offer, target):
            assert offer.pay_to == accepted["payTo"] and offer.amount == accepted["amount"]
            assert target == exchange[2]["request"]["target"]
            return WebSearchExactPayment(bytes.fromhex(result["receipt"][2:]), bytes.fromhex(payment["rawTransaction"][2:]))
    def rpc_dispatch(method, path, headers, body):
        request = json.loads(body)
        assert method == "POST" and path == "/"
        if request["params"][0].get("data") == layerx_did_call(address):
            encoded = did.encode()
            value = "0x" + (bytes.fromhex(did[11:]) + (64).to_bytes(32, "big") + len(encoded).to_bytes(32, "big") + encoded + bytes(-len(encoded) % 32)).hex()
        else:
            rule = next(rule for rule in ledger["calls"] if rule["method"] == request["method"] and rule["params"] == request["params"])
            account_reads.append(request)
            value = rule["result"]
        return 200, {}, {"jsonrpc": "2.0", "id": request["id"], "result": value}
    def dispatch(method, target, headers, body):
        assert method == "GET" and target == exchange[2]["request"]["target"]
        assert headers["LAYERX-PAYER-ADDRESS"] == address and "LAYERX-PAYER-DID" not in headers
        signature = headers.get("PAYMENT-SIGNATURE")
        requests.append(signature)
        if signature is None:
            return 402, {"PAYMENT-REQUIRED": _header(required)}, exchange[2]["response"]["body"]
        assert json.loads(base64.b64decode(signature)) == exchange[3]["request"]["headers"]["PAYMENT-SIGNATURE"]
        if len(requests) == 2:
            return 503, {"Retry-After": "0"}, {"error": "payment_pending"}
        return 200, {"PAYMENT-RESPONSE": _header(exchange[3]["response"]["headers"]["PAYMENT-RESPONSE"])}, exchange[3]["response"]["body"]
    key = rotated_receipt_key()
    if bad_key:
        key = replace(key, key_id=b"\x2a" * 32)
    with http_server(rpc_dispatch) as rpc_endpoint, http_server(dispatch) as endpoint:
        assets = {offer["extra"]["layerx"]["currency"]: WebSearchAssetTerms(offer["asset"], int(offer["amount"])) for offer in required["accepts"]}
        client = WebSearchClient(endpoint, "eip155:125", assets["PAX"].asset_id, assets, Wallet(), (key,),
                                 payer_rpc=PaymentRpc(rpc_endpoint), payer_address=address, pending_attempts=2)
        if bad_key:
            with pytest.raises(WebSearchError) as error:
                client.fetch("https://paxeer.app/index.html")
            assert error.value.code == "receipt-unverified" and len(requests) == 1
        else:
            response = client.fetch("https://paxeer.app/index.html")
            assert response.digest == exchange[3]["response"]["body"]["digest"]
            assert response.settlement.transaction == payment["txHash"]
            assert response.settlement.payer == decode_payment_transaction(bytes.fromhex(payment["rawTransaction"][2:])).data[4:36].hex()
            assert len(requests) == 3 and requests[1] == requests[2]
        assert len(account_reads) == 2


@pytest.mark.parametrize("registered_did", (False, True))
def test_web_search_public_metered_offer_binds_payer_and_refuses_retired_payment(registered_did):
    from test_repair_22_31_147 import http_server, rotated_receipt_key
    fixtures = ROOT / "services/interop/crates/x-websearch/tests/fixtures"
    exchange = json.loads((fixtures / "client-exchange.json").read_text())["exchange"]
    gateway = json.loads((fixtures / "gateway/metered.json").read_text())
    ledger = json.loads((ROOT / "services/agent/sdk/typescript/test/recorded-web-ledger.json").read_text())
    required = exchange[0]["response"]["headers"]["PAYMENT-REQUIRED"]
    accepted = next(offer for offer in required["accepts"] if offer["scheme"] == "metered" and offer["extra"]["layerx"]["currency"] == "SID")
    address = exchange[0]["request"]["headers"]["LAYERX-PAYER-ADDRESS"]
    did = "did:layerx:" + ("ab" * 32 if registered_did else "00" * 12 + address[2:])
    grant = next(value for value in gateway["ledger"]["grants"] if value["name"] == "SID")
    transaction_hash = exchange[1]["response"]["headers"]["PAYMENT-RESPONSE"]["transaction"]
    transaction = next(rule["params"][0] for rule in gateway["endpoints"]["gateway"]["*"] if rule["method"] == "eth_sendRawTransaction" and rule["result"] == transaction_hash)
    settlement = exchange[1]["response"]["headers"]["PAYMENT-RESPONSE"]
    receipt = base64.b64decode(settlement["extensions"]["layerx"]["receipt"], validate=True)
    terms = accepted["extra"]["layerx"]
    offer = WebSearchOffer(
        scheme=accepted["scheme"],
        network=accepted["network"],
        amount=accepted["amount"],
        asset=accepted["asset"],
        pay_to=accepted["payTo"],
        max_timeout_seconds=accepted["maxTimeoutSeconds"],
        currency=terms["currency"],
        account=terms["account"],
        payer=terms["payer"],
        purpose_hash=terms["purposeHash"],
    )
    calls, requests = [], []
    class Wallet:
        preferences = (WebSearchPreference("SID", "metered"),)
        def __init__(self):
            self.did = did
        def metered(self, offer, target):
            assert offer.payer == accepted["extra"]["layerx"]["payer"]
            assert offer.pay_to == accepted["payTo"] and target == "/search?q=paxeer"
            return WebSearchMeteredPayment(bytes.fromhex(grant["canonical"]), "27" * 32)
    def rpc_dispatch(method, path, headers, body):
        request = json.loads(body)
        assert method == "POST" and path == "/"
        calls.append(request)
        if request["method"] == "eth_getRawTransactionByHash":
            assert request["params"] == [transaction_hash]
            value = transaction
        elif request["params"][0].get("data") == layerx_did_call(address):
            encoded = did.encode()
            value = "0x" + (bytes.fromhex(did[11:]) + (64).to_bytes(32, "big") + len(encoded).to_bytes(32, "big") + encoded + bytes(-len(encoded) % 32)).hex()
        else:
            rule = next(rule for rule in ledger["calls"] if rule["method"] == request["method"] and rule["params"] == request["params"])
            value = rule["result"]
        return 200, {}, {"jsonrpc": "2.0", "id": request["id"], "result": value}
    def dispatch(method, target, headers, body):
        assert method == "GET" and target == "/search?q=paxeer"
        assert headers["LAYERX-PAYER-ADDRESS"] == address and "LAYERX-PAYER-DID" not in headers
        signature = headers.get("PAYMENT-SIGNATURE")
        requests.append(signature)
        if signature is None:
            return 402, {"PAYMENT-REQUIRED": _header(required)}, exchange[0]["response"]["body"]
        payload = json.loads(base64.b64decode(signature))
        assert payload["accepted"] == accepted
        assert payload["payload"] == {"grantId": grant["grantId"], "idempotencyKey": "27" * 32}
        return 200, {"PAYMENT-RESPONSE": _header(exchange[1]["response"]["headers"]["PAYMENT-RESPONSE"])}, exchange[1]["response"]["body"]
    with http_server(rpc_dispatch) as rpc_endpoint, http_server(dispatch) as endpoint:
        assets = {offer["extra"]["layerx"]["currency"]: WebSearchAssetTerms(offer["asset"], int(offer["amount"])) for offer in required["accepts"]}
        for reopen in range(2):
            rpc = PaymentRpc(rpc_endpoint)
            client = WebSearchClient(endpoint, "eip155:125", assets["PAX"].asset_id, assets, Wallet(), (rotated_receipt_key(),),
                                     payer_rpc=rpc, payer_address=address, now=lambda: 1)
            with pytest.raises(WebSearchError) as error:
                client.search("paxeer")
            assert error.value.code == "receipt-unverified", WEB_REFUSAL_ORDER_FINDING
            assert isinstance(error.value.__cause__, RevertReasonError), WEB_REFUSAL_ORDER_FINDING
            assert error.value.__cause__.code == "LXP_NO_PAYER_GRANT", WEB_REFUSAL_ORDER_FINDING
            assert error.value.__cause__.retryable is False, WEB_REFUSAL_ORDER_FINDING
            assert settlement["extensions"]["layerx"]["receiptDigest"] != sha256(
                b"LXP/v1/merkle-leaf\0" + receipt
            ).hexdigest(), WEB_REFUSAL_ORDER_FINDING
            assert [call["method"] for call in calls[-4:]] == [
                "eth_call", "eth_call", "eth_call", "eth_getRawTransactionByHash"
            ], WEB_REFUSAL_ORDER_FINDING
            # The public consumer and the independent verifier refuse the same retired payment.
            raw = rpc.call("eth_getRawTransactionByHash", [transaction_hash])
            assert raw == transaction
            with pytest.raises(WebSearchError) as retired:
                verify_web_payment(
                    receipt, bytes.fromhex(raw[2:]), offer, (rotated_receipt_key(),)
                )
            assert retired.value.code == "receipt-unverified"
            assert retired.value.__cause__.code == "LXP_NO_PAYER_GRANT"
            del client
        assert len(requests) == 4 and requests[0] is None and requests[2] is None
        assert requests[1] == requests[3]
        assert [call["method"] for call in calls] == ["eth_call", "eth_call", "eth_call", "eth_getRawTransactionByHash", "eth_getRawTransactionByHash"] * 2


def test_web_search_recorded_producer_digest_is_the_authenticated_unsigned_receipt():
    from test_repair_22_31_147 import rotated_receipt_key
    fixtures = ROOT / "services/interop/crates/x-websearch/tests/fixtures"
    exchange = json.loads((fixtures / "client-exchange.json").read_text())["exchange"]
    exact = json.loads((fixtures / "gateway/exact.json").read_text())
    metered = json.loads((fixtures / "gateway/metered.json").read_text())
    for index, recording in ((1, metered), (3, exact)):
        settlement = exchange[index]["response"]["headers"]["PAYMENT-RESPONSE"]
        evidence = settlement["extensions"]["layerx"]
        if index == 3:
            raw = bytes.fromhex(exact["payments"]["SID"]["rawTransaction"][2:])
        else:
            rule = next(rule for rule in recording["endpoints"]["gateway"]["*"]
                        if rule["method"] == "eth_sendRawTransaction" and rule["result"] == settlement["transaction"])
            raw = bytes.fromhex(rule["params"][0][2:])
        receipt = base64.b64decode(evidence["receipt"], validate=True)
        verified = verify_sequencer_receipt(receipt, raw, (rotated_receipt_key(),))
        # The committed server recording matches the real core verifier, not a stale hash.
        assert evidence["receiptDigest"] == verified.receipt_digest.hex()
        assert settlement["transaction"] == "0x" + verified.receipt.activity_id.hex()
        assert evidence["receiptDigest"] == sha256(b"LXP/v1/receipt\0" + receipt[:-69] + b"\0").hexdigest()
        # Pin the independent signed-envelope digest too; neither hash can stand in for the other.
        leaf = sha256(b"LXP/v1/merkle-leaf\0" + receipt).hexdigest()
        assert leaf != evidence["receiptDigest"]
        if index == 3:
            payload = exchange[index]["request"]["headers"]["PAYMENT-SIGNATURE"]["payload"]
            assert payload["receiptDigest"] == verified.receipt_digest.hex()
            assert payload["txHash"] == settlement["transaction"]
