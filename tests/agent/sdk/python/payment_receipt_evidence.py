"""Unmodified node-produced transactions and sequencer-receipt-v1 evidence."""

import json
from copy import deepcopy
from pathlib import Path

from layerx_sdk.account_derivation import keccak256
from layerx_sdk.evm_transaction import decode_signed_transaction
from layerx_sdk.verifier import ReceiptKey

ROOT = Path(__file__).resolve().parents[4]
CORPUS_PATH = ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"


def receipt_vector(name="V1-layerxledger-pay-exact"):
    corpus = json.loads(CORPUS_PATH.read_text())
    if corpus["provenance"]["generator"] != "rpc/sequencer_receipt_vectors_test.go":
        raise AssertionError("payment evidence must come from the node producer")
    return deepcopy(next(case for case in corpus["cases"] if case["id"] == name))


def unhex(value):
    return bytes.fromhex(value.removeprefix("0x"))


def receipt_key(vector):
    pin = vector["pinnedKey"]
    return ReceiptKey(
        unhex(pin["keyId"]), unhex(pin["publicKey"]),
        pin["firstHeight"], pin["lastHeight"],
    )


def payment_terms(vector):
    transaction, signature = decode_signed_transaction(unhex(vector["rawTransaction"]))
    # payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32): compare
    # independent transaction arguments and emitted PaymentExact log fields.
    calldata = transaction.data
    if len(calldata) != 196 or transaction.to != unhex("00" * 18 + "101a"):
        raise AssertionError("expected the recorded ledger payment transaction")
    terms = {
        "payer": transaction.recover_signer(signature)[2:].lower().zfill(64),
        "pay_to": calldata[36:68].hex(),
        "asset": calldata[68:100].hex(),
        "amount": str(int.from_bytes(calldata[100:132], "big")),
    }
    if vector["receiptJson"]["status"] == "0x1":
        topic = keccak256(b"PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)")
        payments = [
            log for log in vector["receiptJson"]["logs"]
            if unhex(log["address"]) == transaction.to
            and log["topics"] and unhex(log["topics"][0]) == topic
        ]
        if len(payments) != 1:
            raise AssertionError("expected exactly one recorded PaymentExact log")
        log = payments[0]
        if (
            len(log["topics"]) != 4
            or unhex(log["topics"][1]) != calldata[164:196]
            or unhex(log["topics"][2]) != calldata[4:36]
            or unhex(log["topics"][3]).hex() != terms["pay_to"]
            or unhex(log["data"])
            != unhex(terms["asset"]) + int(terms["amount"]).to_bytes(32, "big")
        ):
            raise AssertionError("payment log differs from signed calldata")
    return terms
