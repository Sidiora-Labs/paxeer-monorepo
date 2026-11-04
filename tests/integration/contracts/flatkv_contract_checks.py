"""Offline evidence consumers used by the FlatKV integration scripts."""

import argparse
import json
import hashlib
import os
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



def dump_records(path, bucket, version):
    lines = Path(path).read_text().splitlines()
    if not lines or lines[0] != f"Bucket {bucket} at version {version}" or version <= 0:
        raise ValueError("dump is not at the selected committed version")
    records = {}
    for line in lines[1:]:
        row = re.fullmatch(r"Key: ([0-9A-Fa-f]+), Value: ([0-9A-Fa-f]+)", line)
        if row is None:
            raise ValueError("malformed dump row")
        key, value = (bytes.fromhex(part) for part in row.groups())
        if key in records:
            raise ValueError("duplicate physical key")
        records[key] = value
    return records


def restored_fixture(directory, fixtures, version):
    directory, fixtures = Path(directory), Path(fixtures)
    def fixture(name):
        return (fixtures / f"flatkv_evm_{name}.txt").read_text().strip()
    receipt = json.loads((fixtures / "flatkv_evm_deploy_receipt.json").read_text())
    if receipt["result"]["status"] != "0x1":
        raise ValueError("fixture deployment was not committed successfully")
    deployed = int(receipt["result"]["blockNumber"], 16)
    if not 0 < deployed <= version:
        raise ValueError("selected version predates the fixture")
    address = hex_bytes(fixture("contract_addr"), 20)
    account = dump_records(directory / "account", "account", version).get(b"evm/\x0a" + address, b"")
    if (len(account) not in (49, 81) or account[0] != 0
            or not deployed <= int.from_bytes(account[1:9], "big") <= version
            or int.from_bytes(account[41:49], "big") != 1):
        raise ValueError("missing or invalid fixture AccountData")
    storage = dump_records(directory / "storage", "storage", version)
    # storage_row also checks the exact receipt modification height and typed value.
    storage_row((directory / "storage").read_text(), fixture("contract_addr"),
                fixture("storage_slot"), fixture("storage_expected"), deployed)
    code = dump_records(directory / "code", "code", version).get(b"evm/\x07" + address, b"")
    expected_code = bytes.fromhex(fixture("code_expected").removeprefix("0x"))
    if code != b"\x00" + deployed.to_bytes(8, "big") + expected_code or not expected_code:
        raise ValueError("fixture CodeData differs at the exact contract key")
    return {"contract": fixture("contract_addr"), "storage": fixture("storage_expected"),
            "code": fixture("code_expected"), "version": version,
            "storage_key": next(key.hex() for key in storage
                                if key == b"evm/\x03" + address + hex_bytes(fixture("storage_slot"), 32))}


def log_fields(line):
    return dict(re.findall(r"\b(height|format|hash|chunk|total|blocks_synced|state_synced)=([A-Za-z0-9]+)\b", line))


def partial_import(logfile, chunk_root, flatkv, output):
    """Called with paxd stopped: retain actual non-final persisted import progress."""
    lines = Path(logfile).read_text().splitlines()
    accepted = [i for i, line in enumerate(lines) if 'msg="Snapshot accepted, restoring"' in line]
    if not accepted:
        raise ValueError("no accepted snapshot")
    start = accepted[-1]
    identity = log_fields(lines[start])
    height, fmt, snap_hash = int(identity["height"]), int(identity["format"]), identity["hash"]
    if height <= 0 or fmt <= 0 or not re.fullmatch(r"[0-9A-Fa-f]+", snap_hash):
        raise ValueError("invalid partial snapshot identity")
    if any('msg="Snapshot restored"' in line or 'state_synced=' in line for line in lines[start:]):
        raise ValueError("snapshot already completed before kill")
    chunks = [log_fields(line) for line in lines[start:]
              if 'msg="Applied snapshot chunk to ABCI app"' in line]
    if not chunks:
        raise ValueError("no applied chunk")
    chunk = chunks[-1]
    index, total = int(chunk["chunk"]), int(chunk["total"])
    if (chunk["height"] != str(height) or chunk["format"] != str(fmt)
            or not 0 <= index < total - 1):
        raise ValueError("no non-final chunk in the accepted snapshot")
    files = [p for p in Path(chunk_root).glob(f"tm-statesync*/{index}")
             if p.is_file() and p.stat().st_size > 0]
    working = Path(flatkv) / "working"
    persisted = [p for p in working.rglob("*")
                 if p.is_file() and p.suffix in (".log", ".sst") and p.stat().st_size > 0]
    if len(files) != 1 or not persisted or (working / "SNAPSHOT_BASE").exists():
        raise ValueError("partial chunk and unpublished FlatKV writes must be persisted")
    proof = {"snapshot_height": height, "snapshot_format": fmt, "snapshot_hash": snap_hash.upper(),
             "chunk": index, "total": total, "chunk_file": str(files[0]),
             "chunk_sha256": hashlib.sha256(files[0].read_bytes()).hexdigest()}
    with Path(output).open("x", encoding="utf-8") as stream:
        json.dump(proof, stream, sort_keys=True)
        stream.write("\n")
        stream.flush()
        os.fsync(stream.fileno())
    return proof


def recovered_import(logfile, minimum, recovered_height, flatkv, partial=None):
    log = Path(logfile).read_text()
    proof = snapshot_import(log, minimum, recovered_height)
    # WriteSnapshot publishes a zero-padded snapshot base only after successful import.
    marker = (Path(flatkv) / "working/SNAPSHOT_BASE").read_text().strip()
    match = re.fullmatch(r"snapshot-([0-9]{20})", marker)
    if not match or not proof["snapshot_height"] <= int(match[1]) <= recovered_height:
        raise ValueError("recovered FlatKV has no published import snapshot")
    if partial is not None:
        with Path(partial).open() as stream:
            previous = json.load(stream)
        retained_chunk = Path(previous["chunk_file"])
        if hashlib.sha256(retained_chunk.read_bytes()).hexdigest() != previous["chunk_sha256"]:
            raise ValueError("persisted partial chunk changed across restart")
        for key in ("snapshot_height", "snapshot_format", "snapshot_hash"):
            if str(proof[key]).upper() != str(previous[key]).upper():
                raise ValueError("restart restored a different snapshot")
        # The real importer resets working state; assert a fresh offer and replay from chunk zero.
        lines = log.splitlines()
        offers = [i for i, line in enumerate(lines) if 'msg="Snapshot accepted, restoring"' in line]
        if not offers:
            raise ValueError("restart did not re-offer the partial snapshot")
        offer = log_fields(lines[offers[-1]])
        if (int(offer["height"]) != previous["snapshot_height"]
                or int(offer["format"]) != previous["snapshot_format"]
                or offer["hash"].upper() != previous["snapshot_hash"]):
            raise ValueError("restart offered a different snapshot")
        chunks = [log_fields(line) for line in lines[offers[-1] + 1:]
                  if 'msg="Applied snapshot chunk to ABCI app"' in line]
        if (not chunks or int(chunks[0]["chunk"]) != 0
                or {int(c["chunk"]) for c in chunks} != set(range(previous["total"]))
                or any(int(c["height"]) != previous["snapshot_height"]
                       or int(c["format"]) != previous["snapshot_format"]
                       or int(c["total"]) != previous["total"] for c in chunks)):
            raise ValueError("restart did not safely reimport every partial-snapshot chunk")
        proof["partial_outcome"] = "discarded-and-reimported-same-snapshot"
    return proof


def replay_recovery(logfile, recovered_height):
    lines = Path(logfile).read_text().splitlines()
    if any('msg="Snapshot restored"' in line for line in lines):
        raise ValueError("replay scenario unexpectedly imported a snapshot")
    outcomes = [log_fields(line) for line in lines if 'msg="switching to consensus reactor"' in line]
    if not outcomes:
        raise ValueError("missing replay handoff")
    outcome = outcomes[-1]
    if (outcome["state_synced"] != "false" or int(outcome["blocks_synced"]) <= 0
            or not 0 < int(outcome["height"]) <= recovered_height):
        raise ValueError("replay did not recover committed state")
    return {"recovery_path": "blocksync-replay", "recovered_height": recovered_height}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("storage", "snapshot", "reopen", "missing", "fixture", "partial", "imported", "replay"))
    parser.add_argument("args", nargs="+")
    options = parser.parse_args(argv)
    args = options.args
    if options.command == "fixture":
        directory, fixtures, version = args
        print(json.dumps(restored_fixture(directory, fixtures, int(version)), sort_keys=True))
    elif options.command == "partial":
        partial_import(*args)
    elif options.command == "imported":
        logfile, minimum, height, flatkv, *partial = args
        print(json.dumps(recovered_import(logfile, int(minimum), int(height), flatkv,
                                          partial[0] if partial else None), sort_keys=True))
    elif options.command == "replay":
        logfile, height = args
        print(json.dumps(replay_recovery(logfile, int(height)), sort_keys=True))
    elif options.command == "storage":
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
    except (ValueError, KeyError, OSError, IndexError) as error:
        print(f"FlatKV evidence rejected: {error}", file=sys.stderr)
        raise SystemExit(1)
