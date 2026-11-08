#!/usr/bin/env python3

import base64
from copy import deepcopy
from datetime import datetime, timezone
from hashlib import sha256
import json
from pathlib import Path
import runpy
import sys


ROOT = Path(__file__).resolve().parents[2]
SOURCE = "daemon/committed-fixture/vectors/vector-v4-committed.json"
SDK = "sdk/platform/conformance/fixtures/vector-v4-committed.json"
CONVERTER = "sdk/platform/conformance/fixtures/refresh_receipt_vectors.py"
EXPLORER = (
    "services/explorer/backend/apps/explorer/test/support/fixture/"
    "paxeer_x/committed_vectors.json"
)
COMET = (
    "services/explorer/backend/apps/explorer/test/support/fixture/"
    "paxeer_x/comet_commit.json"
)
INTEROP = "services/interop/specs/conformance/committed/vector-v4-committed.json"
X402 = "services/interop/specs/conformance/x402/sequenced-settlement.json"
RETAINED = (
    "vector-v1-receipted-positive.json",
    "vector-v2-receipted-revert.json",
    "vector-v3-receipt-refusals.json",
)
RETAINED_SOURCE = "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"


def load(path, root=ROOT):
    with (root / path).open(encoding="utf-8") as stream:
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


def hex_base64(value):
    return "0x" + base64.b64decode(value, validate=True).hex()


def timestamp(value):
    seconds, fraction = value[:-1].split(".") if "." in value else (value[:-1], "")
    epoch = datetime(1970, 1, 1, tzinfo=timezone.utc)
    date = datetime.fromisoformat(seconds).replace(tzinfo=timezone.utc)
    return {"seconds": (date - epoch).days * 86400 + (date - epoch).seconds,
            "nanos": int(fraction.ljust(9, "0") or "0")}


def block_id(value):
    return {
        "hash": "0x" + value["hash"].lower(),
        "parts": {"total": value["parts"]["total"],
                  "hash": "0x" + value["parts"]["hash"].lower()},
    }


def header(value):
    result = {
        "version": {key: int(number) for key, number in value["version"].items()},
        "chain_id": value["chain_id"], "height": int(value["height"]),
        "time": timestamp(value["time"]), "last_block_id": block_id(value["last_block_id"]),
    }
    for field in ("last_commit_hash", "data_hash", "validators_hash", "next_validators_hash",
                  "consensus_hash", "app_hash", "last_results_hash", "evidence_hash", "proposer_address"):
        result[field] = "0x" + value[field].lower()
    return result


def commit(value):
    return {
        "height": int(value["height"]), "round": value["round"],
        "block_id": block_id(value["block_id"]),
        "signatures": [{
            "block_id_flag": signature["block_id_flag"],
            "validator_address": "0x" + signature["validator_address"].lower(),
            "timestamp": timestamp(signature["timestamp"]),
            "signature": (hex_base64(signature["signature"])
                          if signature["signature"] is not None else None),
        } for signature in value["signatures"]],
    }


def proof(value, data, root_hash, encoding):
    return {
        "rootHash": "0x" + root_hash.lower(), "data": data,
        "total": int(value["total"]), "index": int(value["index"]),
        "aunts": [hex_base64(aunt) for aunt in value["aunts"]], "leafEncoding": encoding,
    }


def interop_cases(source, sdk):
    settlement = source["settlement"]
    power = sum(int(row["voting_power"]) for row in settlement["validators"])
    case = {
        "id": "committed-payExact", "rawTransaction": settlement["raw_transaction"],
        "receiptJson": settlement["receipt"], "signedReceipt": settlement["receipt"]["receipt"],
        "prevSeqHash": sdk["prevSeqHash"], "pinnedKey": sdk["pinnedKey"],
        "header": header(settlement["header"]), "executionHeader": header(settlement["execution_header"]),
        "validators": [{"pub_key": hex_base64(row["pub_key"]["value"]),
                        "voting_power": int(row["voting_power"])} for row in settlement["validators"]],
        "commit": commit(settlement["commit"]),
        "expected": {"rung": "committed", "power": power, "commitPower": power},
        "receiptHeader": source["positive"]["header"],
        "receiptHeaderSignature": source["positive"]["header_signature"],
        "receiptProof": source["positive"]["proof"],
    }
    for field, recorded in (("batchProof", "sequence_batch_proof"), ("transactionProof", "transaction_proof")):
        wire = settlement[recorded]
        case[field] = proof(wire["proof"], hex_base64(wire["data"]), wire["root_hash"], "sha256-tx")
    case["execTxResultProof"] = proof(
        settlement["exec_tx_result_proof"], settlement["exec_tx_result_bytes"],
        settlement["header"]["last_results_hash"], "exec-tx-result",
    )
    under_power = deepcopy(case)
    under_power.update(id="commit-under-two-thirds", commit=commit(settlement["commit_at_most_two_thirds"]),
                       expected={"rung": "refused", "error": "InsufficientVotingPower",
                                 "commitError": "InsufficientVotingPower"})
    wrong_validators = deepcopy(case)
    wrong_validators["id"] = "wrong-validators-hash"
    wrong_validators["validators"][0]["voting_power"] += 1
    wrong_validators["expected"] = {"rung": "refused", "error": "ValidatorsHashMismatch", "commitPower": power + 1}
    flipped = deepcopy(case)
    flipped["id"] = "flipped-proof-aunt"
    aunt = bytearray.fromhex(flipped["batchProof"]["aunts"][0][2:])
    aunt[0] ^= 1
    flipped["batchProof"]["aunts"][0] = "0x" + aunt.hex()
    flipped["expected"] = {"rung": "refused", "error": "RootMismatch", "commitPower": power}
    return [case, under_power, wrong_validators, flipped]


def explorer_cases(source, sdk, negative):
    settlement = source["settlement"]
    receipt = settlement["receipt"]
    batch = settlement["sequence_batch"]
    wire = settlement["sequence_batch_proof"]
    validators = [{key: value for key, value in row.items() if key != "proposer_priority"}
                  for row in settlement["validators"]]
    count = str(len(validators))
    case = {
        "id": "committed-ok", "rawTransaction": settlement["raw_transaction"],
        "receiptJson": receipt, "pinnedKey": sdk["pinnedKey"], "prevSeqHash": sdk["prevSeqHash"],
        "eth_receipt": {"transactionHash": receipt["txHash"], "blockNumber": receipt["blockNumber"],
                        "blockHash": receipt["blockHash"], "transactionIndex": receipt["index"],
                        **{field: receipt[field] for field in ("status", "gasUsed", "logs")}},
        "batch": {"from_seq": batch["FromSeq"], "to_seq": batch["ToSeq"], "height": str(batch["Height"]),
                  "tx": hex_base64(wire["data"]), "proof": {
                      "total": int(wire["proof"]["total"]), "index": int(wire["proof"]["index"]),
                      "leafHash": hex_base64(wire["proof"]["leaf_hash"]),
                      "aunts": [hex_base64(aunt) for aunt in wire["proof"]["aunts"]]}},
        "chain_id": settlement["execution_header"]["chain_id"],
        "commit": {"signed_header": {"header": settlement["execution_header"], "commit": settlement["execution_commit"]}},
        "validators": {"block_height": settlement["execution_header"]["height"],
                       "validators": validators, "count": count, "total": count},
        "receiptInclusion": sdk["receiptInclusion"], "settlement": settlement,
        "results_commit": {"signed_header": {"header": settlement["header"], "commit": settlement["commit"]}},
        "results_validators": {"block_height": settlement["header"]["height"],
                               "validators": settlement["validators"], "count": count, "total": count},
    }
    below = deepcopy(case)
    below["id"] = "r7-height-below-receipt"
    below["eth_receipt"]["blockNumber"] = hex(int(receipt["height"], 16) - 1)
    mismatch = deepcopy(case)
    mismatch.update(id="r8-commit-height-mismatch", chain_id=negative["chain_id"], **negative["four_of_four"])
    unrelated = deepcopy(mismatch)
    unrelated["id"] = "unrelated-valid-header"
    unrelated["receiptJson"]["rung"] = "receipted"
    del unrelated["receiptInclusion"]
    del unrelated["settlement"]
    return [case, below, mismatch], [unrelated]


def refresh_explorer_copy(root=ROOT):
    source_bytes = (root / SOURCE).read_bytes()
    source = json.loads(source_bytes)
    converter = runpy.run_path(str(root / CONVERTER))
    sdk = converter["_committed"](source)
    document = load(EXPLORER, root)
    document["source"]["positive"] = SOURCE
    document["source"]["sha256"] = sha256(source_bytes).hexdigest()
    document["cases"], document["refusals"] = explorer_cases(source, sdk, load(COMET, root))
    (root / EXPLORER).write_text(json.dumps(document, indent=2) + "\n", encoding="utf-8")


def compare_settlements(vectors, source, sdk, root):
    receipt = source["settlement"]["receipt"]
    log = receipt["logs"][0]
    data = bytes.fromhex(log["data"][2:])
    expected = {
        "name": "committed_settlement", "valid": True,
        "expected": {"verdict": "accept", "field": "/success"},
        "document": {"success": True, "payer": log["topics"][1][2:],
                     "transaction": receipt["txHash"], "network": "eip155:125",
                     "amount": str(int.from_bytes(data[:32], "big")),
                     "extensions": {"layerx": {"signedReceipt": receipt["receipt"],
                         "receiptDigest": sdk["receiptDigest"], "verificationLevel": "committed",
                         "blockNumber": receipt["blockNumber"], "blockHash": receipt["blockHash"]}}},
    }
    retained = load(RETAINED_SOURCE, root)
    payment = next(case for case in retained["cases"] if case["id"] == "V1-layerxledger-pay-exact")
    receipted = {
        "name": "receipted_settlement", "valid": True,
        "expected": {"verdict": "accept", "field": "/success"},
        "document": {"success": True, "payer": payment["receiptJson"]["from"],
                     "transaction": payment["receiptJson"]["txHash"], "network": "eip155:125",
                     "amount": "1000000", "extensions": {"layerx": {
                         "signedReceipt": payment["signedReceipt"], "receiptDigest": payment["receiptDigest"]}}},
    }
    pending = deepcopy(receipted)
    pending.update(name="submitted_settlement", expected={"verdict": "pending", "field": "/errorReason"})
    pending["document"].update(success=False, errorReason="settlement_pending")
    refused = deepcopy(receipted)
    refused.update(name="committed_without_proof", valid=False,
                   expected={"verdict": "refuse", "field": "/transaction"})
    refused["document"]["transaction"] = ""
    del refused["document"]["extensions"]["layerx"]["signedReceipt"]
    compare(vectors, [receipted, pending, refused, expected], X402)
    return 4


def compare_retained(root):
    recorded = load(RETAINED_SOURCE, root)
    compare(recorded["chainId"], 125, RETAINED_SOURCE + ":chainId")
    compare(recorded["provenance"]["generator"], "rpc/sequencer_receipt_vectors_test.go",
            RETAINED_SOURCE + ":generator")
    count = 0
    for name, group, size in zip(RETAINED, ("V1", "V2", "V3"), (5, 2, 6)):
        path = "sdk/platform/conformance/fixtures/" + name
        copied = load(path, root)
        expected = [case for case in recorded["cases"] if case["vector"] == group]
        require(len(expected) == size, RETAINED_SOURCE, f"{group} coverage differs")
        if group == "V3":
            ids = ["V3-key-not-pinned", "V3-height-outside-key-range", "V3-activity-id-not-raw-hash",
                   "V3-seq-hash-mismatch", "V3-tampered-log", "V3-non-canonical-bytes"]
            compare([case["id"] for case in expected], ids, RETAINED_SOURCE + ":refusal coverage")
            compare([case["expected"]["refusedAt"] for case in expected],
                    ["R3", "R3", "R4", "R6", "R5", "R2"], RETAINED_SOURCE + ":refusedAt")
        compare(copied["cases"], expected, path + ":cases")
        count += len(copied["cases"])
    return count


def check_receipt_vector_copies(root=ROOT):
    source = load(SOURCE, root)
    converter = runpy.run_path(str(root / CONVERTER))
    sdk = converter["_committed"](source)
    copied_sdk = load(SDK, root)
    compare(copied_sdk["positive"], sdk, SDK + ":positive")
    compare(copied_sdk["refusals"], [{"id": "insufficient-power",
            "nextCommit": converter["_commit"](source["settlement"]["commit_at_most_two_thirds"]),
            "expectedCheck": "committed-next-commit"}], SDK + ":refusals")
    digest = sha256((root / SOURCE).read_bytes()).hexdigest()
    compare(copied_sdk["provenance"]["source"], SOURCE, SDK + ":source")
    compare(copied_sdk["provenance"]["sha256"], digest, SDK + ":sha256")
    interop = load(INTEROP, root)
    compare(interop["provenance"]["source"], SOURCE, INTEROP + ":source")
    compare(interop["provenance"]["sourceSha256"], digest, INTEROP + ":sourceSha256")
    for field in ("positive", "settlement", "authorization"):
        compare(interop[field], source[field], INTEROP + ":" + field)
    expected_interop = interop_cases(source, sdk)
    compare(interop["cases"], expected_interop, INTEROP + ":cases")
    explorer = load(EXPLORER, root)
    compare(explorer["source"]["positive"], SOURCE, EXPLORER + ":source")
    compare(explorer["source"]["sha256"], digest, EXPLORER + ":sha256")
    cases, refusals = explorer_cases(source, sdk, load(COMET, root))
    compare(explorer["cases"], cases, EXPLORER + ":cases")
    compare(explorer["refusals"], refusals, EXPLORER + ":refusals")
    count = 2 + len(expected_interop) + len(cases) + len(refusals)
    count += compare_settlements(load(X402, root), source, sdk, root)
    count += compare_retained(root)
    print(f"Compared {count} receipt vector copies with {SOURCE}")


def main():
    try:
        check_receipt_vector_copies()
    except (OSError, ValueError, KeyError, TypeError) as error:
        print(f"receipt vector copy check failed: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
