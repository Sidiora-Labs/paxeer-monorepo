"""Offline evidence consumers used by the FlatKV integration scripts."""

import argparse
import json
from pathlib import Path
import re
import sys


RUNTIME = bytes.fromhex("602a60005260206000f3")
SLOT_VALUE = (42).to_bytes(32, "big")


def hex_bytes(value, length):
    if not isinstance(value, str) or not re.fullmatch(r"(?:0x)?[0-9a-fA-F]+", value):
        raise ValueError("invalid hex value")
    result = bytes.fromhex(value.removeprefix("0x"))
    if len(result) != length:
        raise ValueError(f"expected {length} bytes, got {len(result)}")
    return result


def storage_row(dump, address, slot, expected, height):
    """Bind the physical EVM key and StorageData v0 in a single dump row."""
    key = b"evm/\x03" + hex_bytes(address, 20) + hex_bytes(slot, 32)
    value = hex_bytes(expected, 32)
    if height <= 0:
        raise ValueError("storage modification height must be positive")
    matches = []
    for line in dump.splitlines():
        if not line.startswith("Key:"):
            continue
        row = re.fullmatch(r"Key: ([0-9A-Fa-f]+), Value: ([0-9A-Fa-f]+)\s*", line)
        if row is None:
            raise ValueError("malformed dump row")
        row_key, row_value = (bytes.fromhex(part) for part in row.groups())
        if row_key == key:
            matches.append(row_value)
    want = b"\x00" + height.to_bytes(8, "big") + value
    if matches != [want]:
        raise ValueError("fixture key must have exactly one matching StorageData row")
    return {"key": key.hex(), "value": value.hex(), "height": height, "tag": 0}


def snapshot_import(log, minimum, recovered_height):
    """Require completed ABCI restore followed by a state-sync handoff."""
    restored = []
    outcomes = []
    for index, line in enumerate(log.splitlines()):
        if 'msg="Snapshot restored"' in line:
            height = re.search(r"\bheight=([0-9]+)\b", line)
            snap_hash = re.search(r"\bhash=([0-9A-Fa-f]+)\b", line)
            snap_format = re.search(r"\bformat=([0-9]+)\b", line)
            if not height or not snap_hash or not snap_format:
                raise ValueError("snapshot restore record lacks height, hash or format")
            restored.append((index, int(height[1]), snap_hash[1], int(snap_format[1])))
        if 'msg="switching to consensus reactor"' in line:
            synced = re.search(r"\bstate_synced=(true|false)\b", line)
            height = re.search(r"\bheight=([0-9]+)\b", line)
            if not synced or not height:
                raise ValueError("handoff lacks state-sync outcome or height")
            outcomes.append((index, synced[1], int(height[1])))
    if not restored or not outcomes:
        raise ValueError("snapshot import and handoff evidence are required")
    restore_index, height, snap_hash, snap_format = restored[-1]
    outcome_index, synced, handoff_height = outcomes[-1]
    if (synced != "true" or outcome_index <= restore_index or height < minimum
            or height <= 0 or handoff_height < height or recovered_height < handoff_height):
        raise ValueError("snapshot import did not recover the required committed state")
    return {"snapshot_height": height, "snapshot_hash": snap_hash,
            "snapshot_format": snap_format, "recovered_height": recovered_height}


def validate_restored_state(state):
    hex_bytes(state["contract"], 20)
    if (hex_bytes(state["storage"], 32) != SLOT_VALUE
            or hex_bytes(state["code"], len(RUNTIME)) != RUNTIME):
        raise ValueError("restored fixture differs from the embedded contract")
    return state


def save_recovery(path, proof):
    """Retain the validated import across the node's next restart."""
    validate_restored_state(proof["restored_state"])
    target = Path(path)
    with target.open("x", encoding="utf-8") as stream:
        json.dump(proof, stream, sort_keys=True)
        stream.write("\n")


def reopen_recovery(path, recovered_height, restored_state):
    with Path(path).open(encoding="utf-8") as stream:
        proof = json.load(stream)
    validate_restored_state(restored_state)
    if proof["restored_state"] != restored_state:
        raise ValueError("restored fixture changed after reopening")
    if (proof["snapshot_height"] <= 0 or not proof["snapshot_hash"]
            or recovered_height < proof["recovered_height"]
            or proof["recovered_height"] < proof["snapshot_height"]):
        raise ValueError("reopened recovery state regressed")
    return proof


def missing_state_error(log, version):
    """Accept only the production FlatKV load error for the deleted version."""
    if version <= 0:
        raise ValueError("deleted state version must be positive")
    message = ("failed to load FlatKV version: FlatKV version mismatch: "
               f"requested {version}, reached 0")
    errors = [line for line in log.splitlines()
              if re.search(r"(?:^Error:|^panic:|\blevel=error\b|\blevel=fatal\b)", line)]
    if not errors or any(not re.search(re.escape(message) + r'(?:"|\s|$)', line)
                         for line in errors):
        raise ValueError("startup failure is not the exact missing FlatKV state/version error")
    return version


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("storage", "snapshot", "reopen", "missing"))
    parser.add_argument("args", nargs="+")
    options = parser.parse_args(argv)
    args = options.args
    if options.command == "storage":
        dump, directory = map(Path, args)
        receipt = json.loads((directory / "flatkv_evm_deploy_receipt.json").read_text())
        height = int(receipt["result"]["blockNumber"], 16)
        def fixture(name):
            return (directory / f"flatkv_evm_{name}.txt").read_text().strip()
        storage_row(dump.read_text(), fixture("contract_addr"), fixture("storage_slot"),
                    fixture("storage_expected"), height)
    elif options.command == "snapshot":
        logfile, minimum, height, state, output = args
        proof = snapshot_import(Path(logfile).read_text(), int(minimum), int(height))
        proof["restored_state"] = json.loads(Path(state).read_text())
        save_recovery(output, proof)
        print(proof["snapshot_height"])
    elif options.command == "reopen":
        path, height, state = args
        print(reopen_recovery(path, int(height), json.loads(Path(state).read_text()))["snapshot_height"])
    else:
        logfile, version = args
        missing_state_error(Path(logfile).read_text(), int(version))


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, OSError) as error:
        print(f"FlatKV evidence rejected: {error}", file=sys.stderr)
        raise SystemExit(1)
