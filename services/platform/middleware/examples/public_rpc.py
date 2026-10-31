import argparse
import hashlib
import json
import os
import re
import urllib.error
import urllib.request
import urllib.parse
from pathlib import Path

from layerx_sdk.account_derivation import keccak256
from layerx_sdk.evm_transaction import decode_signed
from layerx_sdk.x402_http import decode_header
from layerx_sdk.x402_rpc import (
    PaymentRpc,
    PaymentRpcError,
    rpc_hex,
    _NoRedirect,
)
from layerx_sdk.px import decode_px_resolved_identities
from layerx_sdk.verifier import ReceiptKey, verify_sequencer_receipt


class PublicRpc(PaymentRpc):
    def call(self, method, params):
        self._id += 1
        request_id = self._id
        request = urllib.request.Request(
            self.endpoint,
            method="POST",
            headers={**self.headers, "Content-Type": "application/json"},
            data=json.dumps(
                {"jsonrpc": "2.0", "id": request_id, "method": method, "params": params}
            ).encode(),
        )
        try:
            response = self._opener.open(request, timeout=30)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            raw = response.read(8388609)
            status = response.status
        if len(raw) > 8388608:
            raise ValueError("rpc-body-too-large")
        try:
            body = json.loads(raw)
        except (ValueError, UnicodeError) as error:
            if not 200 <= status < 300:
                raise ValueError(f"rpc-http-{status}") from error
            raise
        if (
            not isinstance(body, dict)
            or body.get("jsonrpc") != "2.0"
            or type(body.get("id")) is not int
            or body["id"] != request_id
            or ("error" in body) == ("result" in body)
        ):
            raise ValueError("invalid-rpc-envelope")
        if "error" in body:
            failure = body["error"]
            if (
                not isinstance(failure, dict)
                or type(failure.get("code")) is not int
                or not isinstance(failure.get("message"), str)
            ):
                raise ValueError("invalid-rpc-error")
            error = PaymentRpcError(failure["code"], failure.get("data"))
            error.args = (f"{method}: JSON-RPC error {failure['code']}",)
            raise error
        if not 200 <= status < 300:
            raise ValueError(f"rpc-http-{status}")
        result = body["result"]
        if method == "eth_getTransactionCount":
            if (
                not isinstance(result, str)
                or re.fullmatch(r"0x(?:0|[1-9a-f][0-9a-f]{0,15})", result) is None
            ):
                raise ValueError("invalid-rpc-nonce")
            return result
        if method == "eth_sendRawTransaction":
            return result
        if method in ("pax_sequencerReceipt", "eth_getTransactionReceipt") and result is None:
            return None
        if not isinstance(result, dict):
            raise ValueError("invalid-rpc-result")
        return result


def receipt_hex(value, size=None):
    if not isinstance(value, str):
        raise ValueError("invalid-rpc-hex")
    return rpc_hex(value.removeprefix("0x"), size)


def receipt_height(value):
    if type(value) is int:
        height = value
    elif isinstance(value, str) and re.fullmatch(
        r"(?:0|[1-9][0-9]*|0x(?:0|[1-9a-f][0-9a-f]*))", value
    ):
        height = int(value, 16 if value.startswith("0x") else 10)
    else:
        raise ValueError("invalid-receipt-key-height")
    if not 0 <= height < 1 << 64:
        raise ValueError("invalid-receipt-key-height")
    return height


def load_receipt_keys(path):
    configured = json.loads(Path(path).read_text())
    if not isinstance(configured, list) or not configured:
        raise ValueError("invalid-receipt-keys")
    keys = []
    for entry in configured:
        if not isinstance(entry, dict) or not {
            "keyId", "publicKey", "firstHeight", "lastHeight"
        } <= entry.keys():
            raise ValueError("receipt-key-id-range-and-public-key-required")
        first = receipt_height(entry["firstHeight"])
        last = receipt_height(entry["lastHeight"])
        if first > last:
            raise ValueError("invalid-receipt-key-range")
        keys.append(ReceiptKey(
            receipt_hex(entry["keyId"], 32),
            receipt_hex(entry["publicKey"], 32), first, last,
        ))
    return keys


def payment_offer(required):
    if (
        not isinstance(required, dict)
        or type(required.get("x402Version")) is not int
        or required["x402Version"] != 2
        or not isinstance(required.get("resource"), dict)
        or not isinstance(required["resource"].get("url"), str)
        or urllib.parse.urlsplit(required["resource"]["url"]).scheme not in ("https", "http")
        or not urllib.parse.urlsplit(required["resource"]["url"]).netloc
        or not isinstance(required.get("accepts"), list)
        or not 1 <= len(required["accepts"]) <= 32
    ):
        raise ValueError("invalid-payment-required")
    selected = None
    for offer in required["accepts"]:
        if not isinstance(offer, dict) or not {
            "scheme", "network", "amount", "asset", "payTo", "maxTimeoutSeconds"
        } <= offer.keys():
            raise ValueError("invalid-payment-offer")
        if offer["scheme"] not in ("exact", "metered", "subscription"):
            raise ValueError("unsupported-payment")
        network = offer["network"]
        if not isinstance(network, str) or re.fullmatch(r"eip155:[1-9][0-9]{0,9}", network) is None:
            raise ValueError("unsupported-network")
        if int(network[7:]) >= 1 << 32:
            raise ValueError("unsupported-network")
        amount = offer["amount"]
        if (
            not isinstance(amount, str)
            or re.fullmatch(r"[1-9][0-9]{0,38}", amount) is None
            or int(amount) >= 1 << 128
        ):
            raise ValueError("invalid-payment-amount")
        receipt_hex(offer["asset"], 32)
        receipt_hex(offer["payTo"], 32)
        if type(offer["maxTimeoutSeconds"]) is not int or not 0 < offer["maxTimeoutSeconds"] < 1 << 32:
            raise ValueError("invalid-payment-timeout")
        extra = offer.get("extra", {})
        if not isinstance(extra, dict) or not isinstance(extra.get("layerx", {}), dict):
            raise ValueError("invalid-payment-terms")
        terms = extra.get("layerx", {})
        for name in ("payer", "purposeHash"):
            if name in terms and receipt_hex(terms[name], 32) == bytes(32):
                raise ValueError("invalid-payment-terms")
        commitment = terms.get("commitment", "receipted")
        if selected is None and commitment in ("receipted", "committed"):
            selected = offer
    if selected is None:
        raise ValueError("receipted-or-committed-offer-required")
    return selected


def verify_rpc_payment(result, raw_transaction, expected_payer, offer, receipt_keys):
    expected_hash = keccak256(raw_transaction)
    if receipt_hex(result.get("txHash"), 32) != expected_hash:
        raise ValueError("transaction-hash-mismatch")
    if result.get("rung") == "submitted":
        return None
    if result.get("rung") not in ("receipted", "committed"):
        raise ValueError("payment-refused")
    verified = verify_sequencer_receipt(
        receipt_hex(result.get("receipt")), raw_transaction, receipt_keys
    )
    receipt = verified.receipt
    if (
        receipt.context_hash != receipt_hex(result.get("keyId"), 32)
        or verified.height != receipt_height(result.get("height"))
        or receipt.result_code != 0
        or result.get("status") != "0x1"
        or receipt.from_account != rpc_hex(expected_payer, 32)
        or receipt.module_id <= 11
        or offer["scheme"] != "exact"
    ):
        raise ValueError("payment-binding-mismatch")
    transaction, _ = decode_signed(raw_transaction)
    if offer["network"] != f"eip155:{transaction.chain_id}":
        raise ValueError("payment-network-mismatch")
    effects = receipt.effects
    if not effects:
        raise ValueError("evm-summary-required")
    summary = effects[0]
    if (
        summary.module_id != receipt.module_id or summary.ordinal != 0
        or summary.kind != 1 or summary.monetary or summary.event_type != 0
        or len(summary.body) < 110 or not 1 <= summary.body[108] <= 64
        or len(summary.body) != 109 + summary.body[108]
        or int.from_bytes(summary.body[104:108], "big") != len(effects) - 1
    ):
        raise ValueError("invalid-evm-summary")
    terms = offer.get("extra", {}).get("layerx", {})
    purpose = receipt_hex(terms.get("purposeHash"), 32)
    data = receipt_hex(offer["asset"], 32) + int(offer["amount"]).to_bytes(32, "big")
    topic = keccak256(b"PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)")
    payments = []
    for index, effect in enumerate(effects[1:], 1):
        body = effect.body
        if (
            effect.module_id != receipt.module_id or effect.ordinal != index
            or effect.kind != 3 or effect.monetary or effect.event_type != 0
            or len(body) < 53 or body[20] > 4
            or len(body) != 53 + body[20] * 32
        ):
            raise ValueError("invalid-evm-log")
        if body[:20] != bytes.fromhex("00" * 18 + "101a") or body[21:53] != topic:
            continue
        if body[20] != 4 or body[-32:] != hashlib.sha256(data).digest():
            raise ValueError("payment-binding-mismatch")
        payments.append(body)
    if len(payments) != 1:
        raise ValueError("single-ledger-payment-required")
    payment = payments[0]
    if (
        payment[53:85] != purpose
        or payment[117:149] != receipt_hex(offer["payTo"], 32)
        or "payer" in terms and payment[85:117] != receipt_hex(terms["payer"], 32)
    ):
        raise ValueError("payment-binding-mismatch")
    if terms.get("commitment", "receipted") == "committed":
        raise ValueError("committed-receipt-proof-verifier-unavailable")
    return verified


def main():
    parser = argparse.ArgumentParser(
        description="Read public RPC sequence, claim faucet funding, or verify a raw EIP-1559 payment transaction."
    )
    parser.add_argument("--rpc", default=os.environ.get("LAYERX_RPC_URL"))
    parser.add_argument("--did", default=os.environ.get("LAYERX_DID"))
    parser.add_argument("--faucet", default=os.environ.get("LAYERX_FAUCET_URL"))
    parser.add_argument("--public-key")
    parser.add_argument("--claim-key")
    parser.add_argument("--activity")
    parser.add_argument("--offer")
    parser.add_argument(
        "--receipt-keys",
        help="JSON array of keyId, publicKey (hex32), firstHeight and lastHeight pins",
    )
    parser.add_argument("--payer")
    args = parser.parse_args()
    if not args.rpc or not args.did:
        parser.error("RPC URL and DID are required through arguments or environment")
    if urllib.parse.urlsplit(args.rpc).port == 18545 or (
        args.faucet and urllib.parse.urlsplit(args.faucet).port == 18545
    ):
        raise ValueError("persistent-host-chain-forbidden")
    token = os.environ.get("LAYERX_RPC_TOKEN")
    rpc = PublicRpc(args.rpc, {"Authorization": f"Bearer {token}"} if token else {})
    if args.faucet:
        url = urllib.parse.urlsplit(args.faucet)
        if (
            url.username
            or url.password
            or url.query
            or url.fragment
            or url.path != "/v1/faucet/claims"
            or url.scheme != "https"
            and not (
                url.scheme == "http"
                and url.hostname in ("localhost", "127.0.0.1", "::1")
            )
        ):
            raise ValueError("invalid-faucet-url")
        rpc_hex(args.public_key, 32)
        faucet_token = os.environ.get("LAYERX_FAUCET_TOKEN")
        if not args.claim_key or not faucet_token:
            raise ValueError("faucet-claim-key-and-token-required")
        request = urllib.request.Request(
            args.faucet,
            method="POST",
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {faucet_token}",
                "Idempotency-Key": args.claim_key,
            },
            data=json.dumps({"did": args.did, "public_key": args.public_key}).encode(),
        )
        with urllib.request.build_opener(_NoRedirect()).open(
            request, timeout=30
        ) as response:
            print(
                f"Faucet HTTP status: {response.status}; confirm the account balance before spending."
            )
    account = decode_px_resolved_identities(rpc.call("px_resolveAccount", [args.did]))
    if (
        not account.bound
        or account.layerx_did != args.did
        or account.evm_address is None
    ):
        raise ValueError("did-evm-binding-required")
    print(
        json.dumps(
            {
                "sequence": rpc.call(
                    "eth_getTransactionCount", [account.evm_address, "pending"]
                )
            }
        )
    )
    if args.activity:
        if not args.offer or not args.receipt_keys or not args.payer:
            raise ValueError("offer-receipt-keys-and-payer-required")
        offer = payment_offer(decode_header(Path(args.offer).read_text().strip()))
        commitment = offer.get("extra", {}).get("layerx", {}).get("commitment", "receipted")
        canonical = Path(args.activity).read_text().strip()
        if re.fullmatch(r"0x02(?:[0-9a-f]{2})+", canonical) is None:
            raise ValueError("raw-eip1559-transaction-required")
        raw = receipt_hex(canonical)
        decode_signed(raw)
        transaction_hash = "0x" + keccak256(raw).hex()
        receipt_keys = load_receipt_keys(args.receipt_keys)
        rpc_hex(args.payer, 32)
        if rpc.send(canonical) != transaction_hash:
            raise ValueError("transaction-hash-mismatch")
        result = rpc.receipt(transaction_hash)
        verified = None if result is None else verify_rpc_payment(
            result, raw, args.payer, offer, receipt_keys
        )
        if verified is None:
            print(json.dumps({"state": "pending", "transaction": transaction_hash}))
            return 2
        print(json.dumps({"transaction": transaction_hash, "commitment": commitment}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
