"""CometBFT commit and transaction inclusion verification with caller-pinned validators."""
from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
from hashlib import sha256
import re

from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey

from .account_derivation import keccak256


def _varint(value: int) -> bytes:
    value &= (1 << 64) - 1
    out = bytearray()
    while value >= 128:
        out.append((value & 127) | 128)
        value >>= 7
    return bytes(out) + bytes([value])


def _bytes(field: int, value: bytes) -> bytes:
    return _varint(field * 8 + 2) + _varint(len(value)) + value


def _int(field: int, value: int) -> bytes:
    return b"" if value == 0 else _varint(field * 8) + _varint(value)


def _hex(value: str, size: int | None = None) -> bytes:
    result = bytes.fromhex(value)
    if size is not None and len(result) != size:
        raise ValueError("invalid committed evidence digest")
    return result


def _timestamp(value: str) -> bytes:
    match = re.fullmatch(r"(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d)(?:\.(\d{1,9}))?Z", value)
    if match is None:
        raise ValueError("invalid Comet timestamp")
    seconds = int(datetime.fromisoformat(match[1]).replace(tzinfo=timezone.utc).timestamp())
    nanos = int((match[2] or "").ljust(9, "0"))
    return _int(1, seconds) + _int(2, nanos)


def _block_id(value: dict) -> bytes:
    parts = value["parts"]
    total = parts["total"]
    if type(total) is not int or not 0 < total < 1 << 32:
        raise ValueError("invalid part set")
    return _bytes(1, _hex(value["hash"], 32)) + _bytes(2, _int(1, total) + _bytes(2, _hex(parts["hash"], 32)))


def _root(leaves: tuple[bytes, ...] | list[bytes]) -> bytes:
    if not leaves:
        return sha256(b"").digest()
    if len(leaves) == 1:
        return sha256(b"\0" + leaves[0]).digest()
    split = 1 << ((len(leaves) - 1).bit_length() - 1)
    return sha256(b"\1" + _root(leaves[:split]) + _root(leaves[split:])).digest()


def _header_hash(header: dict) -> bytes:
    version = header["version"]
    last = header["last_block_id"]
    last_id = b"" if not last["hash"] else _block_id(last)
    leaves = [
        _int(1, int(version["block"])) + _int(2, int(version["app"])),
        _bytes(1, header["chain_id"].encode()), _int(1, int(header["height"])),
        _timestamp(header["time"]), last_id,
    ]
    leaves.extend(_bytes(1, _hex(header[key])) if header[key] else b"" for key in (
        "last_commit_hash", "data_hash", "validators_hash", "next_validators_hash",
        "consensus_hash", "app_hash", "last_results_hash", "evidence_hash", "proposer_address"))
    return _root(leaves)


def _vote_bytes(chain_id: str, commit: dict, signature: dict) -> bytes:
    def fixed(field, value):
        return b"" if value == 0 else bytes([field * 8 + 1]) + value.to_bytes(8, "little", signed=True)
    vote = (b"\x08\x02" + fixed(2, commit["height"]) + fixed(3, commit["round"])
            + _bytes(4, _block_id(commit["block_id"]))
            + _bytes(5, _timestamp(signature["timestamp"])) + _bytes(6, chain_id.encode()))
    return _varint(len(vote)) + vote


@dataclass(frozen=True)
class VerifiedCommit:
    height: int
    block_hash: bytes
    transaction_hash: bytes
    app_hash: bytes


def verify_commit(header: dict, commit: dict, *, trusted_validators: tuple[dict, ...], chain_id: str) -> bytes:
    if (not chain_id or len(chain_id.encode()) > 50 or header["chain_id"] != chain_id
            or type(commit["height"]) is not int or not 0 < commit["height"] < 1 << 63
            or int(header["height"]) != commit["height"]
            or type(commit["round"]) is not int or not 0 <= commit["round"] < 1 << 31):
        raise ValueError("commit chain or height mismatch")
    block_hash = _header_hash(header)
    if block_hash != _hex(commit["block_id"]["hash"], 32):
        raise ValueError("commit header mismatch")
    validators = {}
    validator_leaves = []
    total = 0
    for validator in trusted_validators:
        address = _hex(validator["address"], 20)
        public = _hex(validator["public_key"], 32)
        power = validator["voting_power"]
        if (address in validators or sha256(public).digest()[:20] != address
                or type(power) is not int or not 0 < power < 1 << 63):
            raise ValueError("invalid pinned validator")
        validators[address] = (public, power)
        validator_leaves.append(_bytes(1, _bytes(1, public)) + _int(2, power))
        total += power
    if _root(validator_leaves) != _hex(header["validators_hash"], 32):
        raise ValueError("untrusted validator set")
    signed = 0
    seen = set()
    for signature in commit["signatures"]:
        flag = signature["block_id_flag"]
        if flag in (1, 3):
            continue
        address = _hex(signature["validator_address"], 20)
        if flag != 2 or address in seen or address not in validators:
            raise ValueError("invalid commit signer")
        seen.add(address)
        public, power = validators[address]
        Ed25519PublicKey.from_public_bytes(public).verify(_hex(signature["signature"], 64), _vote_bytes(chain_id, commit, signature))
        signed += power
    if not total or signed * 3 <= total * 2:
        raise ValueError("insufficient committed voting power")
    return block_hash


def verify_committed_transaction(raw_transaction: bytes, receipt_height: int, *, header: dict, commit: dict,
                                 trusted_validators: tuple[dict, ...], chain_id: str, proof: dict) -> VerifiedCommit:
    block_hash = verify_commit(header, commit, trusted_validators=trusted_validators, chain_id=chain_id)
    if commit["height"] < receipt_height:
        raise ValueError("landing height regression")
    # Comet tx proofs commit to SHA256(tx) leaves, not Ethereum transaction hashes.
    data = _hex(proof["data"])
    if data != raw_transaction:
        raise ValueError("transaction inclusion identity mismatch")
    index, total = proof["index"], proof["total"]
    if type(index) is not int or type(total) is not int or not 0 <= index < total <= 1 << 63:
        raise ValueError("transaction proof position")
    aunts = [_hex(value, 32) for value in proof["aunts"]]
    if len(aunts) > 63:
        raise ValueError("transaction proof bounds")
    def reconstruct(position, count):
        if count == 1:
            return sha256(b"\0" + sha256(data).digest()).digest()
        if not aunts:
            raise ValueError("short transaction proof")
        aunt = aunts.pop()
        split = 1 << ((count - 1).bit_length() - 1)
        if position < split:
            return sha256(b"\1" + reconstruct(position, split) + aunt).digest()
        return sha256(b"\1" + aunt + reconstruct(position - split, count - split)).digest()
    if reconstruct(index, total) != _hex(header["data_hash"], 32) or aunts:
        raise ValueError("transaction inclusion proof mismatch")
    return VerifiedCommit(commit["height"], block_hash, keccak256(raw_transaction), _hex(header["app_hash"]))


class CommitTrust:
    """Validator set independently pinned for exactly one chain and landing height."""
    def __init__(self, chain_id: str, height: int, validators: tuple[dict, ...]):
        import copy
        if type(height) is not int or not 0 < height < 1 << 63:
            raise ValueError("invalid trusted height")
        self._chain_id = chain_id
        self._height = height
        self._validators = copy.deepcopy(validators)

    def verify(self, raw_transaction: bytes, receipt_height: int, evidence: dict) -> VerifiedCommit:
        if set(evidence) != {"header", "commit", "proof"} or evidence["commit"]["height"] != self._height:
            raise ValueError("untrusted landing height")
        return verify_committed_transaction(raw_transaction, receipt_height, **evidence,
                                            trusted_validators=self._validators, chain_id=self._chain_id)
