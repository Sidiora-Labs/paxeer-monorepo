from __future__ import annotations

import base64
import hashlib
import json
import re
import sqlite3
from collections.abc import Callable
from urllib.parse import urlsplit

from .account_derivation import keccak256
from .production import PlatformSdkError
from .x402 import (
    PaymentEvidence,
    ledger_payment_payer,
    payment_commitment,
    payment_identifier,
    payment_payer,
    payment_purpose,
    verify_payment_receipt,
    verify_sequencer_receipt,
)
from .x402_rpc import _transaction_bytes, rpc_hex, verify_rpc_payment


def _object(value):
    if isinstance(value, dict):
        return value
    raise ValueError("invalid-payment-object")


def _keys(value, required, optional=()):
    _object(value)
    if not set(required) <= value.keys() or value.keys() - set(required) - set(
        optional
    ):
        raise ValueError("invalid-payment-fields")


def _text(value, limit):
    if not isinstance(value, str) or not 0 < len(value) <= limit or "\0" in value:
        raise ValueError("invalid-payment-text")
    return value


def _url(value):
    _text(value, 2048)
    url = urlsplit(value)
    if (
        url.scheme not in ("http", "https")
        or not url.netloc
        or any(c in value for c in "\r\n\0")
    ):
        raise ValueError("invalid-payment-url")


def _canonical(value):
    return json.dumps(
        value,
        sort_keys=True,
        separators=(",", ":"),
        ensure_ascii=False,
        allow_nan=False,
    )


def _resource(value):
    _keys(
        value, ("url",), ("description", "mimeType", "serviceName", "tags", "iconUrl")
    )
    _url(value["url"])
    for key, limit in (("description", 512), ("mimeType", 32), ("serviceName", 32)):
        if key in value:
            _text(value[key], limit)
    if "iconUrl" in value:
        _url(value["iconUrl"])
    if "tags" in value:
        if not isinstance(value["tags"], list) or len(value["tags"]) > 5:
            raise ValueError("invalid-payment-tags")
        for tag in value["tags"]:
            if re.fullmatch(r"[\x20-\x7e]{1,32}", _text(tag, 32)) is None:
                raise ValueError("invalid-payment-tags")


def validate_requirements(value):
    _keys(
        value,
        ("scheme", "network", "amount", "asset", "payTo", "maxTimeoutSeconds"),
        ("extra",),
    )
    if value["scheme"] not in ("exact", "metered", "subscription"):
        raise ValueError("unsupported-payment")
    network = re.fullmatch(r"eip155:([1-9][0-9]{0,9})", _text(value["network"], 17))
    if network is None or int(network[1]) > 0xFFFFFFFF:
        raise ValueError("unsupported-network")
    amount = value["amount"]
    if (
        not isinstance(amount, str)
        or re.fullmatch(r"[1-9][0-9]{0,38}", amount) is None
        or int(amount) >= 1 << 128
    ):
        raise ValueError("invalid-payment-amount")
    timeout = value["maxTimeoutSeconds"]
    if type(timeout) is not int or not 0 < timeout < 1 << 32:
        raise ValueError("invalid-payment-timeout")
    try:
        payment_identifier(value["asset"])
        payment_identifier(value["payTo"])
        payment_commitment(value.get("extra"))
        payment_payer(value.get("extra"), required=value["scheme"] != "exact")
        payment_purpose(value.get("extra"), required=value["scheme"] != "exact")
    except PlatformSdkError as error:
        raise ValueError("invalid-payment-terms") from error
    if value["scheme"] != "exact":
        terms = _object(_object(value.get("extra")).get("layerx"))
        window = terms.get("windowSeconds")
        if value["scheme"] == "subscription":
            if (
                not isinstance(window, str)
                or re.fullmatch(r"[1-9][0-9]{0,19}", window) is None
                or int(window) >= 1 << 64
            ):
                raise ValueError("invalid-payment-window")
        elif "windowSeconds" in terms:
            raise ValueError("invalid-payment-window")
    return value


def _extensions(value):
    _object(value)
    if len(value) > 32:
        raise ValueError("invalid-payment-extensions")
    for name, extension in value.items():
        if re.fullmatch(r"[A-Za-z0-9._-]{1,32}", name) is None:
            raise ValueError("invalid-payment-extension")
        _keys(extension, ("info", "schema"))


def validate_required(value):
    _keys(value, ("x402Version", "resource", "accepts"), ("error", "extensions"))
    if type(value["x402Version"]) is not int or value["x402Version"] != 2:
        raise ValueError("invalid-payment-version")
    _resource(value["resource"])
    if not isinstance(value["accepts"], list) or not 1 <= len(value["accepts"]) <= 32:
        raise ValueError("invalid-payment-offers")
    for offer in value["accepts"]:
        validate_requirements(offer)
    if "error" in value:
        _text(value["error"], 512)
    if "extensions" in value:
        _extensions(value["extensions"])
    return value


def validate_payload(value):
    _keys(value, ("x402Version", "payload", "accepted"), ("resource", "extensions"))
    if type(value["x402Version"]) is not int or value["x402Version"] != 2:
        raise ValueError("invalid-payment-version")
    _object(value["payload"])
    validate_requirements(value["accepted"])
    if "resource" in value:
        _resource(value["resource"])
    if "extensions" in value:
        _extensions(value["extensions"])
    return value


def encode_header(value):
    raw = json.dumps(
        value, separators=(",", ":"), ensure_ascii=False, allow_nan=False
    ).encode()
    if len(raw) > 65536:
        raise ValueError("payment-header-too-large")
    return base64.b64encode(raw).decode()


def decode_header(value):
    if not isinstance(value, str) or not 0 < len(value) <= 131072:
        raise ValueError("invalid-payment-header")
    raw = base64.b64decode(value, validate=True)
    if len(raw) > 65536 or base64.b64encode(raw).decode() != value:
        raise ValueError("invalid-payment-header")

    def pairs(items):
        result = {}
        for key, item in items:
            if key in result:
                raise ValueError("duplicate-payment-field")
            result[key] = item
        return result

    return json.loads(
        raw.decode("utf-8"),
        object_pairs_hook=pairs,
        parse_constant=lambda _: (_ for _ in ()).throw(
            ValueError("invalid-json-number")
        ),
    )


def _proof_bytes(value, size=None, *, empty=False):
    if (
        not isinstance(value, str)
        or len(value) > 131072
        or re.fullmatch(r"0x(?:[0-9a-f]{2})*", value) is None
        or not empty and value == "0x"
        or size is not None and len(value) != 2 + size * 2
    ):
        raise ValueError("invalid-receipt-proof")
    return bytes.fromhex(value[2:])


def _quantity(value, bits=None):
    if (
        not isinstance(value, str)
        or re.fullmatch(r"0x(?:0|[1-9a-f][0-9a-f]*)", value) is None
        or len(value) > (18 if bits == 64 else 66)
    ):
        raise ValueError("invalid-receipt-quantity")
    number = int(value, 16)
    if bits is not None and number >= 1 << bits:
        raise ValueError("invalid-receipt-quantity")
    return number


def _parse_proof(value):
    _keys(value, ("format", "verificationLevel", "rawTransaction", "signedReceipt",
                  "receiptDigest", "receiptKey"), ("receiptJson",))
    if value["format"] != "layerx-receipt-proof-v2" or value["verificationLevel"] not in ("receipted", "committed"):
        raise ValueError("invalid-receipt-proof")
    _proof_bytes(value["rawTransaction"])
    _proof_bytes(value["signedReceipt"])
    _proof_bytes(value["receiptDigest"], 32)
    key = value["receiptKey"]
    _keys(key, ("keyId", "publicKey", "firstHeight", "lastHeight"))
    _proof_bytes(key["keyId"], 32)
    _proof_bytes(key["publicKey"], 32)
    if _quantity(key["firstHeight"], 64) > _quantity(key["lastHeight"], 64):
        raise ValueError("invalid-receipt-key-range")
    if "receiptJson" in value:
        _object(value["receiptJson"])
    return value


def _verify_node_receipt(proof, verified, key):
    if "receiptJson" not in proof:
        return
    node = proof["receiptJson"]
    _keys(node, ("rung", "txHash", "nonce", "seq", "height", "index", "timestamp",
                 "status", "returnData", "logs", "gasUsed", "cumulativeGasUsed",
                 "effectiveGasPrice", "paymentDenom", "feeDenom", "feeAmount",
                 "sponsor", "from", "to", "contractAddress", "vmError", "keyId",
                 "signingKey", "receipt"), ("blockNumber", "blockHash"))
    from .payment_transaction import decode_payment_transaction
    transaction = decode_payment_transaction(_proof_bytes(proof["rawTransaction"]))
    receipt = verified.receipt
    body = receipt.effects[0].body
    if (
        node["rung"] not in ("receipted", "committed")
        or proof["verificationLevel"] == "committed" and node["rung"] != "committed"
        or node["txHash"] != "0x" + receipt.activity_id.hex()
        or node["receipt"] != proof["signedReceipt"]
        or node["keyId"] != "0x" + receipt.context_hash.hex()
        or node["signingKey"] != "0x" + key.public_key.hex()
        or _quantity(node["nonce"]) != transaction.nonce
        or transaction.nonce != receipt.from_sequence
        or _quantity(node["seq"]) != receipt.global_sequence
        or _quantity(node["height"]) != verified.height
        or _quantity(node["timestamp"]) != receipt.timestamp
        or _quantity(node["index"]) != int.from_bytes(body[24:28], "big")
        or _quantity(node["status"]) != (1 if receipt.result_code == 0 else 0)
        or _quantity(node["feeAmount"]) != receipt.fee_charged
        or _proof_bytes(node["from"], 20) != receipt.from_account[12:]
        or _proof_bytes(node["to"], 20) != receipt.to_account[12:]
        or _proof_bytes(node["contractAddress"], 20) != body[28:48]
        or _proof_bytes(node["sponsor"], 20) != body[48:68]
        or len(_proof_bytes(node["returnData"], empty=True)) != int.from_bytes(body[100:104], "big")
        or hashlib.sha256(_proof_bytes(node["returnData"], empty=True)).digest() != receipt.activity_root
        or not isinstance(node["vmError"], str)
        or hashlib.sha256(node["vmError"].encode()).digest() != body[68:100]
        or not isinstance(node["paymentDenom"], str) or not node["paymentDenom"]
        or hashlib.sha256(node["paymentDenom"].encode()).digest() != receipt.asset
        or not isinstance(node["feeDenom"], str) or node["feeDenom"].encode() != body[109:]
    ):
        raise ValueError("node-receipt-mismatch")
    for name, offset in (("gasUsed", 0), ("cumulativeGasUsed", 8), ("effectiveGasPrice", 16)):
        if _quantity(node[name]) != int.from_bytes(body[offset:offset + 8], "big"):
            raise ValueError("node-receipt-mismatch")
    if "blockNumber" in node or "blockHash" in node:
        if node["rung"] != "committed" or _quantity(node.get("blockNumber")) < verified.height:
            raise ValueError("node-receipt-mismatch")
        _proof_bytes(node.get("blockHash"), 32)
    logs = node["logs"]
    if not isinstance(logs, list) or len(logs) != len(receipt.effects) - 1:
        raise ValueError("node-receipt-mismatch")
    for index, (log, effect) in enumerate(zip(logs, receipt.effects[1:])):
        _keys(log, ("address", "topics", "data", "logIndex"))
        topics = log["topics"]
        if (not isinstance(topics, list) or len(topics) != effect.body[20]
                or _quantity(log["logIndex"]) != index
                or _proof_bytes(log["address"], 20) != effect.body[:20]
                or hashlib.sha256(_proof_bytes(log["data"], empty=True)).digest() != effect.body[-32:]):
            raise ValueError("node-receipt-mismatch")
        for topic_index, topic in enumerate(topics):
            if _proof_bytes(topic, 32) != effect.body[21 + topic_index * 32:53 + topic_index * 32]:
                raise ValueError("node-receipt-mismatch")


def _verify_proof(proof, offer, receipt_keys, resolve_receipt):
    proof = _parse_proof(proof)
    receipt = _proof_bytes(proof["signedReceipt"])
    raw = _proof_bytes(proof["rawTransaction"])
    evidence = resolve_receipt(receipt, offer)
    if evidence.canonical_receipt != receipt or evidence.raw_transaction != raw:
        raise ValueError("receipt-mismatch")
    required = payment_commitment(offer.get("extra"))
    if required == "committed" and proof["verificationLevel"] != "committed":
        raise ValueError("commitment-mismatch")
    if offer["network"] != "eip155:125":
        raise ValueError("unsupported-network")
    verified = verify_payment_receipt(
        receipt, raw, receipt_keys, amount=offer["amount"], asset=offer["asset"],
        pay_to=offer["payTo"], ledger_payer=payment_payer(offer.get("extra")),
        purpose_hash=payment_purpose(offer.get("extra")),
        commitment=proof["verificationLevel"], finality=evidence.finality,
        scheme=offer["scheme"], grant_binding=evidence.grant_binding,
    )
    if proof["receiptDigest"] != "0x" + verified.receipt_digest.hex():
        raise ValueError("receipt-digest-mismatch")
    advertised = proof["receiptKey"]
    matches = [key for key in receipt_keys if (
        key.key_id == _proof_bytes(advertised["keyId"], 32) == verified.receipt.context_hash
        and key.public_key == _proof_bytes(advertised["publicKey"], 32)
        and key.first_height == _quantity(advertised["firstHeight"], 64)
        and key.last_height == _quantity(advertised["lastHeight"], 64)
    )]
    if not matches:
        raise ValueError("receipt-key-mismatch")
    # Verify with the advertised pin as well as the caller's complete trust set.
    verify_sequencer_receipt(receipt, raw, matches)
    _verify_node_receipt(proof, verified, matches[0])
    return verified, evidence


def _make_proof(evidence, receipt_keys):
    verified = verify_sequencer_receipt(evidence.canonical_receipt, evidence.raw_transaction, receipt_keys)
    for key in receipt_keys:
        if key.key_id != verified.receipt.context_hash or not key.first_height <= verified.height <= key.last_height:
            continue
        try:
            verify_sequencer_receipt(evidence.canonical_receipt, evidence.raw_transaction, (key,))
        except PlatformSdkError:
            continue
        return {
            "format": "layerx-receipt-proof-v2",
            "verificationLevel": "committed" if evidence.finality is not None else "receipted",
            "rawTransaction": "0x" + evidence.raw_transaction.hex(),
            "signedReceipt": "0x" + evidence.canonical_receipt.hex(),
            "receiptDigest": "0x" + verified.receipt_digest.hex(),
            "receiptKey": {"keyId": "0x" + key.key_id.hex(), "publicKey": "0x" + key.public_key.hex(),
                           "firstHeight": hex(key.first_height), "lastHeight": hex(key.last_height)},
        }
    raise ValueError("receipt-key-mismatch")


class FulfillmentStore:
    def __init__(self, path: str):
        self.path = path
        with sqlite3.connect(path) as db:
            db.execute("PRAGMA journal_mode=WAL")
            db.execute(
                "CREATE TABLE IF NOT EXISTS payment_fulfillments (request_key TEXT PRIMARY KEY, request_digest TEXT NOT NULL, receipt_digest TEXT NOT NULL UNIQUE, resource BLOB NOT NULL)"
            )

    def fulfill(
        self, key, request_digest, receipt_digest, release: Callable[[], bytes]
    ):
        with sqlite3.connect(self.path, timeout=30) as db:
            db.execute("BEGIN IMMEDIATE")
            previous = db.execute(
                "SELECT request_digest, receipt_digest, resource FROM payment_fulfillments WHERE request_key = ?",
                (key,),
            ).fetchone()
            if previous:
                if previous[:2] != (request_digest, receipt_digest):
                    raise ValueError("fulfillment-conflict")
                return previous[2]
            if db.execute(
                "SELECT 1 FROM payment_fulfillments WHERE receipt_digest = ?",
                (receipt_digest,),
            ).fetchone():
                raise ValueError("receipt-replay")
            resource = release()
            if type(resource) is not bytes:
                raise ValueError("resource-must-be-bytes")
            db.execute(
                "INSERT INTO payment_fulfillments VALUES (?, ?, ?, ?)",
                (key, request_digest, receipt_digest, resource),
            )
            return resource


class SellerMiddleware:
    def __init__(
        self,
        required,
        receipt_keys,
        resolve_receipt,
        fulfillments: FulfillmentStore,
        draw=None,
    ):
        self.required = validate_required(json.loads(_canonical(required)))
        self.receipt_keys = receipt_keys
        self.resolve_receipt = resolve_receipt
        self.fulfillments = fulfillments
        self.draw = draw

    def handle(self, principal, payment_header, release):
        if payment_header is None:
            return (
                402,
                {"PAYMENT-REQUIRED": encode_header(self.required)},
                json.dumps(self.required).encode(),
            )
        _text(principal, 512)
        payload = validate_payload(decode_header(payment_header))
        offer = payload["accepted"]
        if (
            offer not in self.required["accepts"]
            or payload.get("resource") != self.required["resource"]
        ):
            raise ValueError("requirements-mismatch")
        for name, value in self.required.get("extensions", {}).items():
            if payload.get("extensions", {}).get(name) != value:
                raise ValueError("extensions-mismatch")
        digest = hashlib.sha256(_canonical(payload).encode()).hexdigest()
        key = hashlib.sha256(
            b"LayerX/middleware/x402/idempotency\0"
            + principal.encode()
            + bytes.fromhex(digest)
        ).hexdigest()
        body = payload["payload"]
        if offer["scheme"] == "exact":
            verified, evidence = _verify_proof(body, offer, self.receipt_keys, self.resolve_receipt)
        else:
            _keys(body, ("receive", "idempotencyKey"))
            rpc_hex(body["receive"], 733)
            rpc_hex(body["idempotencyKey"], 32)
            if self.draw is None:
                raise ValueError("grant-draw-unavailable")
            evidence = self.draw(principal, digest, body, offer)
            if evidence is None:
                return 202, {}, b""
        if offer["scheme"] != "exact":
            from .x402 import refuse_payer_grant
            refuse_payer_grant()
        receipt_digest = verified.receipt_digest.hex()
        resource = self.fulfillments.fulfill(key, digest, receipt_digest, release)
        settlement = {
            "success": True,
            "transaction": "0x" + verified.receipt.activity_id.hex(),
            "network": offer["network"],
            "amount": offer["amount"],
            "payer": ledger_payment_payer(verified, evidence.raw_transaction, offer["scheme"]),
            "extensions": {"layerx": body},
        }
        return 200, {"PAYMENT-RESPONSE": encode_header(settlement)}, resource


class BuyerMiddleware:
    def __init__(self, rpc, receipt_keys, resolve_receipt, supported, *, journal_path=":memory:"):
        self.rpc = rpc
        self.receipt_keys = receipt_keys
        self.resolve_receipt = resolve_receipt
        self.supported = frozenset(supported)
        self._submissions = sqlite3.connect(journal_path)
        self._submissions.execute("CREATE TABLE IF NOT EXISTS buyer_submissions (tx_hash TEXT PRIMARY KEY, canonical BLOB NOT NULL, offer TEXT NOT NULL)")
        self._submissions.commit()

    def parse_offer(self, header):
        required = validate_required(decode_header(header))
        for offer in required["accepts"]:
            if (offer["scheme"], offer["network"]) in self.supported:
                return required, offer
        raise ValueError("unsupported-payment")

    def prepare(self, header, canonical_hex, activity_id, payer):
        required, offer = self.parse_offer(header)
        if offer["scheme"] != "exact":
            raise ValueError("grant-requires-receive-payload")
        raw = _transaction_bytes(canonical_hex)
        from .payment_transaction import decode_payment_transaction
        transaction = decode_payment_transaction(raw)
        if activity_id != "0x" + keccak256(raw).hex() or transaction.chain_id != 125:
            raise ValueError("activity-mismatch")
        with self._submissions:
            prior = self._submissions.execute("SELECT canonical, offer FROM buyer_submissions WHERE tx_hash = ?", (activity_id,)).fetchone()
            if prior is not None and prior != (raw, _canonical(offer)):
                raise ValueError("buyer-submission-conflict")
            claimed = self._submissions.execute("INSERT OR IGNORE INTO buyer_submissions VALUES (?, ?, ?)", (activity_id, raw, _canonical(offer))).rowcount
        if claimed:
            sent = self.rpc.send("0x" + raw.hex())
            if sent != activity_id:
                raise ValueError("activity-mismatch")
        result = self.rpc.receipt(activity_id)
        if result is None or result.get("rung") == "submitted":
            return None
        if payment_commitment(offer.get("extra")) == "committed" and result.get("rung") != "committed":
            return None
        receipt = _transaction_bytes(result.get("receipt"))
        evidence = self.resolve_receipt(receipt, offer)
        verified = verify_rpc_payment(
            result,
            activity_id,
            payer,
            receipt_keys=self.receipt_keys,
            raw_transaction=evidence.raw_transaction,
            finality=evidence.finality,
            amount=offer["amount"],
            asset=offer["asset"],
            pay_to=offer["payTo"],
            commitment=payment_commitment(offer.get("extra")),
        )
        if verified["rung"] == "pending" or evidence.canonical_receipt != receipt:
            raise ValueError("receipt-mismatch")
        proof = _make_proof(evidence, self.receipt_keys)
        _verify_proof(proof, offer, self.receipt_keys, self.resolve_receipt)
        return encode_header(
            {
                "x402Version": 2,
                "resource": required["resource"],
                "accepted": offer,
                "extensions": required.get("extensions", {}),
                "payload": proof,
            }
        )

    def fetch(
        self,
        url,
        payment_header,
        *,
        method="GET",
        data=None,
        headers=None,
        expected_activity=None,
    ):
        import urllib.error
        import urllib.request

        from .x402_rpc import _NoRedirect

        _url(url)
        opener = urllib.request.build_opener(_NoRedirect())

        def request(extra):
            try:
                response = opener.open(
                    urllib.request.Request(
                        url,
                        data=data,
                        headers={**(headers or {}), **extra},
                        method=method,
                    ),
                    timeout=30,
                )
            except urllib.error.HTTPError as error:
                response = error
            with response:
                body = response.read(8388609)
                if len(body) > 8388608:
                    raise ValueError("http-body-too-large")
                return response.status, dict(response.headers), body

        initial = request({})
        if initial[0] != 402:
            return initial
        required_header = next(
            (
                value
                for name, value in initial[1].items()
                if name.lower() == "payment-required"
            ),
            None,
        )
        required, offer = self.parse_offer(required_header)
        payment = validate_payload(decode_header(payment_header))
        if (
            payment["accepted"] != offer
            or payment.get("resource") != required["resource"]
        ):
            raise ValueError("requirements-mismatch")
        if offer["scheme"] != "exact" and expected_activity is None:
            raise ValueError("expected-activity-required")
        paid = request({"PAYMENT-SIGNATURE": payment_header})
        if paid[0] == 202:
            return paid
        settlement = next(
            (
                value
                for name, value in paid[1].items()
                if name.lower() == "payment-response"
            ),
            None,
        )
        if settlement is None:
            raise ValueError("missing-payment-response")
        if offer["scheme"] == "exact":
            self.capture_settlement(settlement, payment_header)
        else:
            self.capture_grant_settlement(settlement, payment_header, expected_activity)
        return paid

    def grant_header(self, header, receive_hex):
        required, offer = self.parse_offer(header)
        return grant_payment_header(required, offer, receive_hex)

    def capture_grant_settlement(self, header, payment_header, expected_activity):
        from .x402 import refuse_payer_grant
        expected_hash = _transaction_bytes(expected_activity, 32)
        payment = validate_payload(decode_header(payment_header))
        if payment["accepted"]["scheme"] not in ("metered", "subscription"):
            raise ValueError("grant-offer-required")
        settlement = _object(decode_header(header))
        proof = _parse_proof(_object(settlement.get("extensions")).get("layerx"))
        if keccak256(_proof_bytes(proof["rawTransaction"])) != expected_hash:
            raise ValueError("transaction-hash-mismatch")
        refuse_payer_grant()

    def capture_settlement(self, header, payment_header):
        settlement = _object(decode_header(header))
        payment = validate_payload(decode_header(payment_header))
        offer = payment["accepted"]
        evidence_body = _object(_object(settlement.get("extensions")).get("layerx"))
        if (
            settlement.get("success") is not True
            or settlement.get("network") != offer["network"]
            or settlement.get("amount") != offer["amount"]
        ):
            raise ValueError("settlement-mismatch")
        payment_proof = _parse_proof(payment["payload"])
        proof = _parse_proof(evidence_body)
        for name in ("rawTransaction", "signedReceipt", "receiptDigest", "receiptKey"):
            if proof[name] != payment_proof[name]:
                raise ValueError("settlement-receipt-mismatch")
        if payment_proof["verificationLevel"] == "committed" and proof["verificationLevel"] != "committed":
            raise ValueError("commitment-mismatch")
        # Authenticate the submitted proof too, including optional node JSON.
        _verify_proof(payment_proof, offer, self.receipt_keys, self.resolve_receipt)
        verified, evidence = _verify_proof(proof, offer, self.receipt_keys, self.resolve_receipt)
        if settlement.get("transaction") != "0x" + verified.receipt.activity_id.hex():
            raise ValueError("settlement-receipt-mismatch")
        if settlement.get("payer") != ledger_payment_payer(verified, evidence.raw_transaction, offer["scheme"]):
            raise ValueError("settlement-payer-mismatch")
        return verified



class ConfiguredReceiptAuthority:
    def __init__(self, raw_transaction: bytes, *, rpc=None, receipt_keys=(),
                 commit_trust=None, committed_evidence=None):
        self.raw_transaction = raw_transaction
        self.rpc = rpc
        self.receipt_keys = receipt_keys
        self.commit_trust = commit_trust
        self.committed_evidence = committed_evidence

    def __call__(self, receipt, offer):
        validate_requirements(offer)
        if self.rpc is None:
            return PaymentEvidence(receipt, self.raw_transaction)
        from .x402_rpc import rpc_payment_evidence
        result = self.rpc.receipt("0x" + keccak256(self.raw_transaction).hex())
        evidence = rpc_payment_evidence(
            result, self.raw_transaction, self.receipt_keys,
            commit_trust=self.commit_trust, committed_evidence=self.committed_evidence,
        )
        if evidence is None or evidence.canonical_receipt != receipt:
            raise ValueError("receipt-mismatch")
        if payment_commitment(offer.get("extra")) == "committed" and evidence.finality is None:
            raise ValueError("missing-committed-evidence")
        return evidence


def grant_payment_header(required, accepted, receive_hex):
    from .x402_receive import decode_receive

    validate_required(required)
    validate_requirements(accepted)
    receive = decode_receive(rpc_hex(receive_hex, 733))
    terms = accepted["extra"]["layerx"]
    grant = receive["payer_grant"]
    if (
        accepted["scheme"] not in ("metered", "subscription")
        or accepted not in required["accepts"]
        or receive["to"] != accepted["payTo"]
        or receive["asset"] != accepted["asset"]
        or receive["amount"] != accepted["amount"]
        or receive["from"] != terms["payer"]
        or receive["from"] != grant["from"]
        or receive["grant_id"] != grant["grant_id"]
        or receive["to"] != grant["recipient"]
        or receive["asset"] != grant["asset"]
        or grant["purpose_hash"] != terms["purposeHash"]
    ):
        raise ValueError("requirements-mismatch")
    return encode_header(
        {
            "x402Version": 2,
            "resource": required["resource"],
            "accepted": accepted,
            "extensions": required.get("extensions", {}),
            "payload": {
                "receive": receive_hex,
                "idempotencyKey": receive["idempotency_key"],
            },
        }
    )
