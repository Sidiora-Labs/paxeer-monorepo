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


def _checkpoint(source, source_path, source_digest):
    settlement = source["settlement"]
    header = settlement["execution_header"]
    commit = settlement["execution_commit"]
    height = int(commit["height"])
    block_hash = commit["block_id"]["hash"].lower()
    if height != int(header["height"]) or len(bytes.fromhex(block_hash)) != 32:
        raise ValueError("invalid recorded execution checkpoint")
    if settlement["receipt"]["blockHash"].removeprefix("0x").lower() != block_hash:
        raise ValueError("recorded receipt differs from execution checkpoint")
    return {
        "provenance": "Out-of-band test-network checkpoint captured from the producer's execution_commit, independently of the portable document being verified. This is a fixture-network pin, not a Paxeer production-network pin.",
        "source": source_path,
        "source_sha256": source_digest,
        "capture_command": "python3 sdk/platform/conformance/fixtures/refresh_receipt_vectors.py",
        "chain_id": header["chain_id"],
        "height": height,
        "block_hash": block_hash,
    }


def _receipted_copies(fixtures):
    source_text = (fixtures / "sequencer-receipt-v1.json").read_text()
    source = json.loads(source_text)
    if (source["chainId"] != 125 or source["provenance"]["generator"]
            != "rpc/sequencer_receipt_vectors_test.go"):
        raise ValueError("receipted copies require the retained chain-125 producer output")
    decoder = json.JSONDecoder()
    marker = '"cases": ['
    position = source_text.index(marker) + len(marker)
    cases = []
    while True:
        position += len(source_text[position:]) - len(source_text[position:].lstrip())
        if source_text[position] == "]":
            break
        case, end = decoder.raw_decode(source_text, position)
        cases.append((case, source_text[position:end]))
        position = end
        if source_text[position] == ",":
            position += 1
    for name, vector, count in (
        ("vector-v1-receipted-positive", "V1", 5),
        ("vector-v2-receipted-revert", "V2", 2),
        ("vector-v3-receipt-refusals", "V3", 6),
    ):
        selected = [(case, raw) for case, raw in cases if case["vector"] == vector]
        if len(selected) != count or len({case["id"] for case, _ in selected}) != count:
            raise ValueError(f"missing or duplicate producer cases for {vector}")
        path = fixtures / (name + ".json")
        text = path.read_text()
        if json.loads(text)["name"] != name:
            raise ValueError(f"unexpected conformance copy identity: {name}")
        start = text.index(marker) + len(marker) - 1
        _, end = decoder.raw_decode(text, start)
        # Preserve the producer's raw case bytes, including JSON escapes.
        array = "[\n    " + ",\n    ".join(raw for _, raw in selected) + "\n  ]"
        text = text[:start] + array + text[end:]
        if vector == "V3":
            provenance = {
                "source": "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json",
                "section": "cases",
                "generator": source["provenance"]["generator"],
                "sourceSha256": hashlib.sha256(source_text.encode()).hexdigest(),
            }
            start = text.index('"provenance": ') + len('"provenance": ')
            _, end = decoder.raw_decode(text, start)
            encoded = json.dumps(provenance, indent=2, sort_keys=True).replace("\n", "\n  ")
            text = text[:start] + encoded + text[end:]
        path.write_text(text)


def main():
    fixtures = Path(__file__).resolve().parent
    root = fixtures.parents[3]
    _receipted_copies(fixtures)
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
    receipted_bytes = (root / source_path).read_bytes()
    cases = json.loads(receipted_bytes)["cases"]
    receipted = next(case for case in cases if case["id"] == case_id)
    committed_path, section = v5["provenance"]["committedSource"].split("#")
    if committed_path != v4["provenance"]["source"] or section != "settlement":
        raise ValueError("portable committed provenance differs from V4")
    v5["provenance"]["receiptedSha256"] = hashlib.sha256(receipted_bytes).hexdigest()
    v5["provenance"]["committedSha256"] = hashlib.sha256(source_bytes).hexdigest()
    bundles = [_portable(receipted, "receipted"), _portable(v4["positive"], "committed")]
    compact = lambda value: json.dumps(value, separators=(",", ":"))
    v4_path.write_text(json.dumps(v4, indent=2) + "\n")
    metadata = {key: value for key, value in v5.items() if key != "bundles"}
    prefix = json.dumps(metadata, indent=2).removesuffix("\n}") + ",\n"
    v5_path.write_text(prefix + '  "bundles": [\n    ' + ",\n    ".join(map(compact, bundles)) + "\n  ]\n}\n")
    checkpoint_path = root / "sdk/platform/go/testdata/repair_m1384/checkpoint.json"
    checkpoint = _checkpoint(source, v4["provenance"]["source"], v4["provenance"]["sha256"])
    checkpoint_path.write_text(json.dumps(checkpoint, indent=2) + "\n")


if __name__ == "__main__":
    main()
