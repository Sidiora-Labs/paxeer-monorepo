import base64
import copy
import datetime
import hashlib
import json
from pathlib import Path


def base64_hex(value):
    return "0x" + base64.b64decode(value, validate=True).hex()


def recorded_hex(value):
    return "0x" + bytes.fromhex(value).hex()


def timestamp(value):
    whole, _, fraction = value.removesuffix("Z").partition(".")
    instant = datetime.datetime.fromisoformat(whole)
    delta = instant - datetime.datetime(1970, 1, 1)
    return {
        "seconds": delta.days * 86400 + delta.seconds,
        "nanos": int(fraction.ljust(9, "0")),
    }


def block_id(value):
    return {
        "hash": recorded_hex(value["hash"]),
        "parts": {
            "total": int(value["parts"]["total"]),
            "hash": recorded_hex(value["parts"]["hash"]),
        },
    }


def header(value):
    result = {
        "version": {key: int(number) for key, number in value["version"].items()},
        "chain_id": value["chain_id"],
        "height": int(value["height"]),
        "time": timestamp(value["time"]),
        "last_block_id": block_id(value["last_block_id"]),
    }
    for field in (
        "last_commit_hash",
        "data_hash",
        "validators_hash",
        "next_validators_hash",
        "consensus_hash",
        "app_hash",
        "last_results_hash",
        "evidence_hash",
        "proposer_address",
    ):
        result[field] = recorded_hex(value[field])
    return result


def commit(value):
    return {
        "height": int(value["height"]),
        "round": int(value["round"]),
        "block_id": block_id(value["block_id"]),
        "signatures": [
            {
                "block_id_flag": vote["block_id_flag"],
                "validator_address": recorded_hex(vote["validator_address"]),
                "timestamp": timestamp(vote["timestamp"]),
                "signature": (
                    base64_hex(vote["signature"])
                    if vote["signature"] is not None
                    else None
                ),
            }
            for vote in value["signatures"]
        ],
    }


def proof(root, data, value, encoding):
    return {
        "rootHash": root,
        "data": data,
        "total": int(value["total"]),
        "index": int(value["index"]),
        "aunts": [base64_hex(aunt) for aunt in value["aunts"]],
        "leafEncoding": encoding,
    }


def record_committed_vectors(recording, source):
    recorded = json.loads(recording)
    settlement = recorded["settlement"]
    receipt = recorded["positive"]
    authorization = recorded["authorization"]
    validators = [
        {
            "pub_key": base64_hex(validator["pub_key"]["value"]),
            "voting_power": int(validator["voting_power"]),
        }
        for validator in settlement["validators"]
    ]
    power = sum(validator["voting_power"] for validator in validators)
    positive = {
        "id": "committed-payExact",
        "rawTransaction": settlement["raw_transaction"],
        "receiptJson": settlement["receipt"],
        "signedReceipt": receipt["receipt"],
        "prevSeqHash": base64_hex(settlement["sequence_batch"]["PrevSeqHash"]),
        "pinnedKey": {
            "keyId": base64_hex(settlement["sequence_batch"]["KeyId"]),
            "publicKey": "0x" + bytes(authorization["PublicKey"]).hex(),
            "firstHeight": authorization["FirstBatchNumber"],
            "lastHeight": authorization["LastBatchNumber"],
        },
        "header": header(settlement["header"]),
        "executionHeader": header(settlement["execution_header"]),
        "validators": validators,
        "commit": commit(settlement["commit"]),
    }
    for field, source_field in (
        ("batchProof", "sequence_batch_proof"),
        ("transactionProof", "transaction_proof"),
    ):
        recorded_proof = settlement[source_field]
        positive[field] = proof(
            recorded_hex(recorded_proof["root_hash"]),
            base64_hex(recorded_proof["data"]),
            recorded_proof["proof"],
            "sha256-tx",
        )
    positive["execTxResultProof"] = proof(
        recorded_hex(settlement["header"]["last_results_hash"]),
        settlement["exec_tx_result_bytes"],
        settlement["exec_tx_result_proof"],
        "exec-tx-result",
    )
    positive["expected"] = {"rung": "committed", "power": power, "commitPower": power}
    positive["receiptHeader"] = receipt["header"]
    positive["receiptHeaderSignature"] = receipt["header_signature"]
    positive["receiptProof"] = receipt["proof"]

    insufficient = copy.deepcopy(positive)
    insufficient["id"] = "commit-under-two-thirds"
    insufficient["commit"] = commit(settlement["commit_at_most_two_thirds"])
    insufficient["expected"] = {
        "rung": "refused",
        "error": "InsufficientVotingPower",
        "commitError": "InsufficientVotingPower",
    }
    wrong_validators = copy.deepcopy(positive)
    wrong_validators["id"] = "wrong-validators-hash"
    wrong_validators["validators"][0]["voting_power"] += 1
    wrong_validators["expected"] = {
        "rung": "refused",
        "error": "ValidatorsHashMismatch",
        "commitPower": power + 1,
    }
    flipped = copy.deepcopy(positive)
    flipped["id"] = "flipped-proof-aunt"
    aunt = bytearray.fromhex(flipped["batchProof"]["aunts"][0].removeprefix("0x"))
    aunt[0] ^= 1
    flipped["batchProof"]["aunts"][0] = "0x" + aunt.hex()
    flipped["expected"] = {
        "rung": "refused",
        "error": "RootMismatch",
        "commitPower": power,
    }
    return {
        "name": recorded["name"],
        "version": 4,
        "provenance": {
            "recorder": "daemon/committed-fixture/vector_v4.go::recordV4",
            "source": source,
            "sourceSha256": hashlib.sha256(recording).hexdigest(),
            "description": recorded["provenance"],
            "receiptScenario": "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json::V1-layerxledger-pay-exact",
            "chainId": settlement["header"]["chain_id"],
            "executionHeight": int(settlement["execution_header"]["height"]),
            "commitHeight": int(settlement["header"]["height"]),
        },
        "cases": [positive, insufficient, wrong_validators, flipped],
        "positive": receipt,
        "settlement": settlement,
        "authorization": authorization,
    }


def main():
    root = Path(__file__).resolve().parents[5]
    source = "daemon/committed-fixture/vectors/vector-v4-committed.json"
    target = root / "services/interop/specs/conformance/committed/vector-v4-committed.json"
    vectors = record_committed_vectors((root / source).read_bytes(), source)
    target.write_text(json.dumps(vectors, indent=2) + "\n")


if __name__ == "__main__":
    main()
