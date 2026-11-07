#!/usr/bin/env python3
"""Capture terminal availability from a trusted paxd committed-chain endpoint."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[3]
SOURCE = (
    "rpc/sequencer_receipt.go",
    "precompiles/layerxprograms/events_call.go",
    "tools/qualification/paxeer-x/native-terminal-outcome.py",
    "tools/qualification/paxeer-x/router-authority-schema.py",
    "tools/qualification/paxeer-x/paid-web-contract.py",
    "services/agent/sdk/python/layerx_sdk/verifier.py",
    "services/agent/sdk/python/layerx_sdk/production.py",
)
SCHEMA = "paxeer-x.committed-terminal-outcomes.v1"
REQUIRED = {"serial-empty-0", "scheduled-0", "scheduled-1", "terminal-0"}
PROGRAMS_ADDRESS = "0x000000000000000000000000000000000000101d"
OUTCOME_TOPIC = "0xcfe5297f03a2951065e463e9e0c2e3686bc0204f727eeec1a52357b5dd4a27a0"
definition = importlib.util.spec_from_file_location(
    "node_schema_transport", Path(__file__).with_name("router-authority-schema.py"))
transport = importlib.util.module_from_spec(definition)
definition.loader.exec_module(transport)
require = transport.require
definition = importlib.util.spec_from_file_location(
    "committed_execution", Path(__file__).with_name("paid-web-contract.py"))
execution = importlib.util.module_from_spec(definition)
definition.loader.exec_module(execution)
sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))
from layerx_sdk.production import PlatformSdkError


def digest(path):
    with Path(path).open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def source_inputs():
    return {name: digest(ROOT / name) for name in SOURCE}


def hex_bytes(value, size=None):
    require(isinstance(value, str) and value.startswith("0x") and len(value) % 2 == 0,
            "canonical hexadecimal bytes required")
    raw = bytes.fromhex(value[2:])
    require(value == "0x" + raw.hex() and (size is None or len(raw) == size),
            "canonical hexadecimal width required")
    return raw


def quantity(value):
    require(isinstance(value, str) and value.startswith("0x"), "hexadecimal quantity required")
    number = int(value, 16)
    require(value == hex(number), "canonical hexadecimal quantity required")
    return number


def terminal_bytes(return_data):
    raw = hex_bytes(return_data)
    require(len(raw) >= 128 and int.from_bytes(raw[64:96]) == 96,
            "real call result ABI required")
    size = int.from_bytes(raw[96:128])
    require(0 < size <= 1048576 and len(raw) == 128 + ((size + 31) // 32) * 32
            and not any(raw[128 + size:]), "canonical bounded terminal ABI required")
    terminal = raw[128:128 + size]

    def split(value, domain):
        require(value.startswith(domain), "retained terminal domain required")
        offset = len(domain)
        parts = []
        for _ in range(2):
            require(len(value) >= offset + 4, "truncated terminal length")
            length = int.from_bytes(value[offset:offset + 4])
            offset += 4
            require(length <= len(value) - offset, "truncated terminal evidence")
            parts.append(value[offset:offset + length])
            offset += length
        require(offset == len(value), "trailing terminal evidence")
        return parts

    detail, legs = split(terminal, b"LXP/programs/terminal-applied-legs/v1\0")
    execution, settlement = split(detail, b"LXP/program-execution-with-occupancy/v1\0")
    require(execution.startswith(b"LXP/program-execution/v2\0")
            and settlement.startswith(b"LXP/storage-occupancy-settlement/v3\0"),
            "retained execution and occupancy evidence required")
    require(len(execution) > len(b"LXP/program-execution/v2\0")
            and len(settlement) > len(b"LXP/storage-occupancy-settlement/v3\0"),
            "empty terminal components")
    return terminal


def committed_outcome(name, receipt, block, raw_transaction, receipt_keys):
    require(name in REQUIRED, "unknown terminal outcome case")
    require(isinstance(receipt, dict) and receipt.get("rung") == "committed",
            "committed chain receipt required")
    try:
        execution.verified_execution(receipt, hex_bytes(raw_transaction), {"receipt_keys": receipt_keys})
    except (execution.Refusal, PlatformSdkError) as error:
        raise RuntimeError("committed chain execution refused: " + str(error)) from error
    tx_hash = receipt["txHash"]
    hex_bytes(tx_hash, 32)
    block_hash = receipt["blockHash"]
    require(any(hex_bytes(block_hash, 32)), "nonzero committed block hash required")
    require(isinstance(block, dict) and block.get("hash") == block_hash
            and block.get("number") == receipt["blockNumber"]
            and quantity(receipt["blockNumber"]) >= quantity(receipt["height"]),
            "committed block identity differs from receipt")
    require(block["transactions"].count(tx_hash) == 1, "transaction absent or duplicated in committed block")
    require(bool(hex_bytes(receipt["receipt"])), "retained chain receipt bytes required")
    status = quantity(receipt["status"])
    require(status in {0, 1}, "invalid chain execution status")
    if name == "terminal-0":
        require(status == 0 and receipt["returnData"] == "0x" and not receipt["logs"]
                and isinstance(receipt.get("vmError"), str) and receipt["vmError"],
                "preexecution refusal must preserve empty execution evidence")
        terminal = b""
    else:
        terminal = terminal_bytes(receipt["returnData"])
        outcomes = [item for item in receipt["logs"] if item.get("address") == PROGRAMS_ADDRESS
                    and isinstance(item.get("topics"), list) and len(item["topics"]) == 2
                    and item["topics"][0] == OUTCOME_TOPIC]
        require(len(outcomes) == 1, "one committed ProgramCallOutcome required")
        data = hex_bytes(outcomes[0]["data"], 13 * 32)
        require(data[11 * 32:12 * 32] == hashlib.sha256(terminal).digest(),
                "terminal bytes differ from committed outcome digest")
        raw = hex_bytes(receipt["returnData"])
        require(raw[:32] == data[8 * 32:9 * 32], "call result differs from committed outcome status")
    return {"name": name, "receipt": receipt, "block": block,
            "terminal_sha256": hashlib.sha256(terminal).hexdigest(),
            "raw_transaction": raw_transaction, "receipt_keys": receipt_keys}


def capture_outcomes(endpoint, transactions, receipt_keys):
    require(set(transactions) == REQUIRED, "all four retained outcome cases required")
    require(transport.rpc(endpoint, "eth_chainId", []).get("result") == "0x7d", "chain 125 required")
    sources = source_inputs()
    captures = []
    for name, tx_hash in sorted(transactions.items()):
        hex_bytes(tx_hash, 32)
        receipt = transport.rpc(endpoint, "pax_sequencerReceipt", [tx_hash]).get("result")
        require(isinstance(receipt, dict) and receipt.get("txHash") == tx_hash,
                "requested committed receipt unavailable")
        block = transport.rpc(endpoint, "eth_getBlockByHash", [receipt["blockHash"], False]).get("result")
        raw = transport.rpc(endpoint, "eth_getRawTransactionByHash", [tx_hash]).get("result")
        captures.append(committed_outcome(name, receipt, block, raw, receipt_keys))
    require(sources == source_inputs(), "terminal producer source changed during capture")
    return {"schema": SCHEMA, "chain_id": 125, "captures": captures, "sources": sources}


def admit_captures(value):
    require(value.get("schema") == SCHEMA and value.get("chain_id") == 125,
            "committed terminal manifest required")
    require(value.get("sources") == source_inputs(), "terminal capture source binding differs")
    captures = value["captures"]
    require(len(captures) == 4 and {item["name"] for item in captures} == REQUIRED,
            "all four retained outcome cases required")
    for item in captures:
        require(committed_outcome(item["name"], item["receipt"], item["block"],
                                  item["raw_transaction"], item["receipt_keys"]) == item,
                "changed committed terminal evidence")
    return captures


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--node-rpc", required=True)
    parser.add_argument("--transactions", type=Path, required=True)
    parser.add_argument("--receipt-keys", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    directory = args.output.parent.resolve(strict=True)
    info = directory.stat()
    require(ROOT != directory and ROOT not in directory.parents and info.st_uid == os.geteuid()
            and not info.st_mode & 0o077, "private owned evidence outside checkout required")
    value = capture_outcomes(args.node_rpc, json.loads(args.transactions.read_text()),
                             json.loads(args.receipt_keys.read_text()))
    admit_captures(value)
    fd = os.open(args.output, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, "w") as output:
        json.dump(value, output, indent=2, sort_keys=True)
        output.write("\n")
    print("PAXEER_X_GATE tests=4 skipped=0")


if __name__ == "__main__":
    try:
        main()
    except (KeyError, OSError, ValueError, RuntimeError) as error:
        print("FAILED", str(error), file=sys.stderr)
        sys.exit(1)
