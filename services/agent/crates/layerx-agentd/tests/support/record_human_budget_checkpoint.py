"""Record a signed Comet checkpoint for the native budget fixture's committed tree."""
import base64
import datetime
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parent.parent


def sha(data):
    return hashlib.sha256(data).digest()


def varint(value):
    data = bytearray()
    while value >= 128:
        data.append((value & 127) | 128)
        value >>= 7
    return bytes(data) + bytes([value])


def field(number, data):
    return bytes([(number << 3) | 2]) + varint(len(data)) + data


def integer(number, value):
    return bytes([number << 3]) + varint(value) if value else b""


def sized(data):
    return len(data).to_bytes(4, "big") + data


def leaf(prefix, key, value):
    return sha(prefix + varint(len(key)) + key + bytes([32]) + sha(value))


def tree(rows):
    if len(rows) == 1:
        return leaf(bytes([0, 2, 2]), *rows[0]), 0, 1
    middle = len(rows) // 2
    left, lh, ls = tree(rows[:middle])
    right, rh, rs = tree(rows[middle:])
    height, size = 1 + max(lh, rh), ls + rs
    prefix = varint(height * 2) + varint(size * 2) + bytes([2, 32])
    return sha(prefix + left + bytes([32]) + right), height, size


def merkle(items):
    if len(items) == 1:
        return sha(bytes([0]) + items[0])
    split = 1 << ((len(items) - 1).bit_length() - 1)
    return sha(bytes([1]) + merkle(items[:split]) + merkle(items[split:]))


def block_id(block_hash, parts_hash):
    return field(1, block_hash) + field(2, integer(1, 1) + field(2, parts_hash))


def record():
    source = ROOT / "human_approval_budget_actor.rs"
    text = source.read_text()
    fixture = json.loads(text.split("/* RECORDED_AUTHORITY\n")[1].split("\nEND_RECORDED_AUTHORITY */")[0])
    height = int(fixture["block"]["number"], 16)
    call = bytearray(fixture["budget_call"])
    rows = [(bytes([2]), height.to_bytes(8, "big"))]
    for index, name in [
        (1, f'agent:{fixture["agent_did"]}:main'),
        (2, f'agent:{fixture["agent_did"]}:budget:{bytes(fixture["budget_id"]).hex()}'),
    ]:
        name = name.encode()
        account = sha(b"LX:ACCOUNT:v1" + sized(name))
        call[index * 32:(index + 1) * 32] = account
        value = (sized(account) + sized(name) + bytes([index]) + (77).to_bytes(16, "big")
                 + sized(call[96:128]) + bytes([1]) + (2).to_bytes(8, "big")
                 + (1).to_bytes(8, "big") + bytes([0, 0]) + sized(bytes([0x61]) * 32) + bytes([1]))
        rows.append((bytes([6]) + account, value))
    root, _, _ = tree(sorted(rows))
    app_hash = leaf(bytes([0]), b"lxphost", root)
    seconds = int(fixture["block"]["timestamp"], 16)
    stamp = datetime.datetime.fromtimestamp(seconds, datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    timestamp = integer(1, seconds)
    parent = bytes.fromhex(fixture["block"]["parentHash"][2:])
    data_hash = bytes.fromhex(fixture["block"]["transactionsRoot"][2:])
    results = bytes.fromhex(fixture["block"]["receiptsRoot"][2:])
    with tempfile.TemporaryDirectory(dir=ROOT / "support") as directory:
        directory = Path(directory)
        key = directory / "signer.der"
        key.write_bytes(bytes.fromhex("302e020100300506032b657004220420") + bytes([1]) * 32)
        public = subprocess.check_output(["openssl", "pkey", "-inform", "DER", "-in", str(key), "-pubout", "-outform", "DER"])[-32:]
        address = sha(public)[:20]
        validators_hash = merkle([field(1, field(1, public)) + integer(2, 10)])
        header_parts = [integer(1, 11) + integer(2, 1), field(1, b"125"), integer(1, height), timestamp,
                        block_id(parent, bytes([2]) * 32), field(1, bytes([3]) * 32), field(1, data_hash),
                        field(1, validators_hash), field(1, validators_hash), field(1, bytes([5]) * 32),
                        field(1, app_hash), field(1, results), field(1, bytes([8]) * 32), field(1, address)]
        block_hash = merkle(header_parts)
        commit_id = block_id(block_hash, bytes([9]) * 32)
        vote = bytes([8, 2, 17]) + height.to_bytes(8, "little") + bytes([25]) + (1).to_bytes(8, "little")
        vote += field(4, commit_id) + field(5, timestamp) + field(6, b"125")
        message = directory / "vote.bin"
        message.write_bytes(varint(len(vote)) + vote)
        signature = subprocess.check_output(["openssl", "pkeyutl", "-sign", "-rawin", "-keyform", "DER", "-inkey", str(key), "-in", str(message)])
    bid = lambda h, p: {"hash": h.hex(), "parts": {"total": 1, "hash": p.hex()}}
    signed = {"header": {
        "version": {"block": "11", "app": "1"}, "chain_id": "125", "height": str(height), "time": stamp,
        "last_block_id": bid(parent, bytes([2]) * 32), "last_commit_hash": (bytes([3]) * 32).hex(),
        "data_hash": data_hash.hex(), "validators_hash": validators_hash.hex(), "next_validators_hash": validators_hash.hex(),
        "consensus_hash": (bytes([5]) * 32).hex(), "app_hash": app_hash.hex(), "last_results_hash": results.hex(),
        "evidence_hash": (bytes([8]) * 32).hex(), "proposer_address": address.hex(),
    }, "commit": {"height": str(height), "round": 1, "block_id": bid(block_hash, bytes([9]) * 32), "signatures": [{
        "block_id_flag": 2, "validator_address": address.hex(), "timestamp": stamp,
        "signature": base64.b64encode(signature).decode(),
    }]}}
    answers = [
        {"method": "px_getCommit", "params": [str(height)], "result": {"height": str(height), "signedHeader": signed}},
        {"method": "px_getValidators", "params": [str(height)], "result": {"height": str(height), "total": "1", "validators": [{
            "address": address.hex(), "pubKey": {"type": "tendermint/PubKeyEd25519", "value": base64.b64encode(public).decode()}, "votingPower": "10",
        }]}},
    ]
    (ROOT / "support/human_budget_checkpoint.json").write_text(json.dumps(answers, indent=2) + "\n")
    for name in ["human_approval_budget_actor.rs", "human_projection_facts.rs"]:
        path = ROOT / name
        text = path.read_text()
        start = text.index("/* RECORDED_AUTHORITY\n") + len("/* RECORDED_AUTHORITY\n")
        end = text.index("\nEND_RECORDED_AUTHORITY */", start)
        recorded = json.loads(text[start:end])
        recorded["block"]["hash"] = "0x" + block_hash.hex()
        recorded["block"]["stateRoot"] = "0x" + app_hash.hex()
        recorded["budget_state"]["checkpoint_digest"] = block_hash.hex()
        path.write_text(text[:start] + json.dumps(recorded, indent=2) + text[end:])


if __name__ == "__main__":
    record()
