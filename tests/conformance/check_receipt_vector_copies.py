#!/usr/bin/env python3

import base64
import json
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SOURCE = "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
EXPLORER = (
    "services/explorer/backend/apps/explorer/test/support/fixture/"
    "paxeer_x/receipt_vectors.json"
)
RELAYER = (
    "services/interop/crates/layerx-bridge-relayer/tests/fixtures/"
    "sequencer_receipts.json"
)
GAS_STATION = (
    "services/interop/crates/layerx-gas-station/tests/fixtures/"
    "sequencer_receipts.json"
)
X402 = "services/interop/specs/conformance/x402/settlement-response.json"


def load(path):
    with (ROOT / path).open(encoding="utf-8") as stream:
        return json.load(stream)


def require(condition, location, message):
    if not condition:
        raise ValueError(f"{location}: {message}")


def compare(actual, expected, location):
    if isinstance(expected, dict):
        require(isinstance(actual, dict), location, "expected an object")
        for field, value in expected.items():
            require(field in actual, location, f"missing {field}")
            compare(actual[field], value, f"{location}.{field}")
        require(actual.keys() == expected.keys(), location, "unexpected fields")
    elif isinstance(expected, list):
        require(isinstance(actual, list), location, "expected an array")
        require(len(actual) == len(expected), location, "array length differs")
        for index, value in enumerate(expected):
            compare(actual[index], value, f"{location}[{index}]")
    else:
        require(
            type(actual) is type(expected) and actual == expected,
            location,
            "differs from conformance source",
        )


def source_case(by_hash, tx_hash, location, case_id=None):
    require(tx_hash in by_hash, location, f"source lacks transaction hash {tx_hash}")
    candidates = by_hash[tx_hash]
    if case_id is None:
        candidates = [
            case for case in candidates if "refusedAt" not in case["expected"]
        ]
    else:
        candidates = [case for case in candidates if case["id"] == case_id]
    require(len(candidates) == 1, location, "source case is missing or ambiguous")
    return candidates[0]


def compare_cases(cases, by_hash, path):
    require(bool(cases), path, "no copied cases")
    seen = set()
    for case in cases:
        location = f"{path}:{case['id']}"
        require(case["id"] not in seen, location, "duplicate copied case")
        seen.add(case["id"])
        expected = source_case(
            by_hash, case["receiptJson"]["txHash"], location, case["id"]
        )
        compare(case, expected, location)
    return len(cases)


def compare_envelopes(path, by_hash):
    document = load(path)
    count = 0
    for name, receipt in document.items():
        if name in ("provenance", "key", "tls"):
            continue
        if name == "unknown" and receipt is None:
            continue
        location = f"{path}:{name}"
        expected = source_case(by_hash, receipt["txHash"], location)
        view = dict(expected["receiptJson"])
        if name == "submitted":
            view = {
                "rung": "submitted",
                "txHash": view["txHash"],
                "nonce": view["nonce"],
            }
        elif name == "committed":
            view["rung"] = "committed"
            view["blockNumber"] = view["height"]
            require("blockHash" in receipt, location, "missing committed block hash")
            view["blockHash"] = receipt["blockHash"]
        compare(receipt, view, location)
        if "key" in document:
            compare(document["key"], expected["pinnedKey"], f"{location}.key")
        count += 1
    require(count >= 4, path, "missing receipt envelopes")
    for name in ("submitted", "receipted", "committed", "reverted"):
        require(name in document, path, f"missing {name} envelope")
    return count


def compare_settlements(path, by_hash, chain_id):
    vectors = load(path)
    require(bool(vectors), path, "no settlement vectors")
    count = 0
    receipts = 0
    for vector in vectors:
        document = vector["document"]
        location = f"{path}:{vector['name']}"
        if not document["transaction"]:
            continue
        expected = source_case(by_hash, document["transaction"], location)
        compare(document["network"], f"eip155:{chain_id}", f"{location}.network")
        if "payer" in document:
            compare(
                document["payer"],
                expected["receiptJson"]["from"],
                f"{location}.payer",
            )
        extension = document.get("extensions", {}).get("layerx", {})
        if document["success"] and vector["valid"]:
            require("receipt" in extension, location, "missing settlement receipt")
        if "receipt" in extension:
            receipt = base64.b64decode(extension["receipt"], validate=True)
            compare(
                receipt,
                bytes.fromhex(expected["signedReceipt"][2:]),
                f"{location}.receipt",
            )
            compare(
                extension["receiptDigest"],
                expected["receiptDigest"][2:],
                f"{location}.receiptDigest",
            )
            compare(
                extension["verificationLevel"],
                expected["receiptJson"]["rung"],
                f"{location}.verificationLevel",
            )
            receipts += 1
        count += 1
    require(receipts > 0, path, "no copied settlement receipts")
    return count


def check_receipt_vector_copies():
    source = load(SOURCE)
    require(bool(source["cases"]), SOURCE, "no source cases")
    by_hash = {}
    for case in source["cases"]:
        by_hash.setdefault(case["receiptJson"]["txHash"], []).append(case)

    explorer = load(EXPLORER)["sequencer"]
    count = compare_cases(explorer["cases"], by_hash, EXPLORER)
    for field in ("chainId", "moduleId", "feeDenom"):
        compare(explorer[field], source[field], f"{EXPLORER}:{field}")
    count += compare_envelopes(RELAYER, by_hash)
    count += compare_envelopes(GAS_STATION, by_hash)
    count += compare_settlements(X402, by_hash, source["chainId"])
    print(f"Compared {count} receipt vector copies with {SOURCE}")


if __name__ == "__main__":
    try:
        check_receipt_vector_copies()
    except (OSError, ValueError, KeyError, TypeError) as error:
        print(f"receipt vector copy check failed: {error}", file=sys.stderr)
        sys.exit(1)
