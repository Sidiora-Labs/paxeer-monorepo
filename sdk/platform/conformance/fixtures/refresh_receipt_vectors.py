import base64
import hashlib
import json
from pathlib import Path


def _base64_hex(value):
    return base64.b64encode(bytes.fromhex(value.removeprefix("0x"))).decode()


def _block_id(value):
    return {
        "Hash": _base64_hex(value["hash"]),
        "PartSetTotal": value["parts"]["total"],
        "PartSetHash": _base64_hex(value["parts"]["hash"]),
    }


def _header(value):
    result = {
        "VersionBlock": int(value["version"]["block"]),
        "VersionApp": int(value["version"]["app"]),
        "ChainID": value["chain_id"],
        "Height": int(value["height"]),
        "Time": value["time"],
        "LastBlockID": _block_id(value["last_block_id"]),
    }
    for field in (
        "last_commit_hash", "data_hash", "validators_hash", "next_validators_hash",
        "consensus_hash", "app_hash", "last_results_hash", "evidence_hash",
        "proposer_address",
    ):
        result["".join(part.title() for part in field.split("_"))] = _base64_hex(value[field])
    return result


def _commit(value):
    return {
        "Height": int(value["height"]),
        "Round": value["round"],
        "BlockID": _block_id(value["block_id"]),
        "Signatures": [{
            "BlockIDFlag": signature["block_id_flag"],
            "ValidatorAddress": _base64_hex(signature["validator_address"]),
            "Timestamp": signature["timestamp"],
            "Signature": signature["signature"],
        } for signature in value["signatures"]],
    }


def _proof(value):
    return {
        "Total": int(value["total"]),
        "Index": int(value["index"]),
        "Aunts": [list(base64.b64decode(aunt)) for aunt in value["aunts"]],
    }


def _committed(source):
    settlement = source["settlement"]
    receipt = settlement["receipt"].copy()
    receipt["rung"] = "receipted"
    del receipt["blockNumber"]
    del receipt["blockHash"]
    signed = bytes.fromhex(receipt["receipt"][2:])
    if signed[-69:-64] != b"\x01\x00\x00\x00\x40":
        raise ValueError("invalid sequencer signature framing")
    digest = hashlib.sha256(b"LXP/v1/receipt\x00" + signed[:-69] + b"\x00").hexdigest()
    validators = [{
        "PublicKey": list(base64.b64decode(validator["pub_key"]["value"])),
        "VotingPower": int(validator["voting_power"]),
    } for validator in settlement["validators"]]
    result = settlement["exec_tx_result"]
    return {
        "id": "committed-payExact",
        "rawTransaction": settlement["raw_transaction"],
        "receiptJson": receipt,
        "signedReceipt": receipt["receipt"],
        "receiptDigest": "0x" + digest,
        "prevSeqHash": "0x" + base64.b64decode(settlement["sequence_batch"]["PrevSeqHash"]).hex(),
        "pinnedKey": {
            "keyId": receipt["keyId"],
            "publicKey": "0x" + bytes(source["authorization"]["PublicKey"]).hex(),
            "firstHeight": source["authorization"]["FirstBatchNumber"],
            "lastHeight": source["authorization"]["LastBatchNumber"],
        },
        "blockHeader": _header(settlement["execution_header"]),
        "receiptInclusion": {
            "Transaction": settlement["transaction_proof"]["data"],
            "TransactionProof": _proof(settlement["transaction_proof"]["proof"]),
            "Result": {
                "Code": result.get("code", 0), "Data": result["data"],
                "GasWanted": int(result["gas_wanted"]), "GasUsed": int(result["gas_used"]),
            },
            "ResultProof": _proof(settlement["exec_tx_result_proof"]),
            "BatchTransaction": settlement["sequence_batch_proof"]["data"],
            "BatchProof": _proof(settlement["sequence_batch_proof"]["proof"]),
            "NextHeader": _header(settlement["header"]),
            "NextCommit": _commit(settlement["commit"]),
            "NextValidators": validators,
        },
        "commit": _commit(settlement["execution_commit"]),
        "validatorSet": validators,
        "expected": {"rung": "committed"},
    }


def _portable(case, level):
    pin = case["pinnedKey"]
    result = {
        "format": "layerx-receipt-proof-v2", "verificationLevel": level,
        "rawTransaction": case["rawTransaction"], "signedReceipt": case["signedReceipt"],
        "receiptDigest": case["receiptDigest"],
        "receiptKey": {
            "keyId": pin["keyId"], "publicKey": pin["publicKey"],
            "firstHeight": hex(pin["firstHeight"]), "lastHeight": hex(pin["lastHeight"]),
        },
        "receiptJson": case["receiptJson"],
    }
    if level == "committed":
        for field in ("blockHeader", "receiptInclusion", "commit", "validatorSet"):
            result[field] = case[field]
    return result


def main():
    fixtures = Path(__file__).resolve().parent
    root = fixtures.parents[3]
    v4_path = fixtures / "vector-v4-committed.json"
    v4 = json.loads(v4_path.read_text())
    source_bytes = (root / v4["provenance"]["source"]).read_bytes()
    source = json.loads(source_bytes)
    v4["provenance"]["sha256"] = hashlib.sha256(source_bytes).hexdigest()
    v4["positive"] = _committed(source)
    v4["refusals"] = [{
        "id": "insufficient-power",
        "nextCommit": _commit(source["settlement"]["commit_at_most_two_thirds"]),
        "expectedCheck": "committed-next-commit",
    }]
    v5_path = fixtures / "vector-v5-portable.json"
    v5 = json.loads(v5_path.read_text())
    source_path, case_id = v5["provenance"]["receiptedSource"].split("#")
    cases = json.loads((root / source_path).read_text())["cases"]
    receipted = next(case for case in cases if case["id"] == case_id)
    committed_path, section = v5["provenance"]["committedSource"].split("#")
    if committed_path != v4["provenance"]["source"] or section != "settlement":
        raise ValueError("portable committed provenance differs from V4")
    bundles = [_portable(receipted, "receipted"), _portable(v4["positive"], "committed")]
    compact = lambda value: json.dumps(value, separators=(",", ":"))
    v4_path.write_text(json.dumps(v4, indent=2) + "\n")
    prefix = v5_path.read_text().split('  "bundles": [', 1)[0]
    v5_path.write_text(prefix + '  "bundles": [\n    ' + ",\n    ".join(map(compact, bundles)) + "\n  ]\n}\n")


if __name__ == "__main__":
    main()
