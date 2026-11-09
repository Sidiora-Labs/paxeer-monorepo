"""CometBFT commit and transaction inclusion verification with caller-pinned validators."""
from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
from hashlib import sha256
import re

from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey

from .account_derivation import keccak256
from .evm_transaction import _rlp


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
class _TransactionField:
    tag: int
    wire: int
    data: bytes = b""
    value: int = 0


def _transaction_fields(data: bytes) -> list[_TransactionField]:
    if len(data) > 1048576:
        raise ValueError("invalid committed transaction envelope")
    offset = 0

    def read():
        nonlocal offset
        value = 0
        for shift in range(0, 70, 7):
            if offset >= len(data):
                raise ValueError("truncated committed transaction envelope")
            next_byte = data[offset]
            offset += 1
            if shift == 63 and next_byte > 1:
                raise ValueError("invalid committed transaction varint")
            value |= (next_byte & 127) << shift
            if next_byte < 128:
                if shift and next_byte == 0:
                    raise ValueError("noncanonical committed transaction varint")
                return value
        raise ValueError("invalid committed transaction varint")

    fields = []
    while offset < len(data):
        key = read()
        tag, wire = key >> 3, key & 7
        if not 0 < tag < 1 << 29 or wire not in (0, 1, 2, 5):
            raise ValueError("invalid committed transaction field")
        if wire == 0:
            fields.append(_TransactionField(tag, wire, value=read()))
        else:
            size = read() if wire == 2 else 8 if wire == 1 else 4
            if size > len(data) - offset:
                raise ValueError("truncated committed transaction field")
            fields.append(_TransactionField(tag, wire, data[offset:offset + size]))
            offset += size
    return fields


def _transaction_field(fields: list[_TransactionField], tag: int, wire: int = 2) -> _TransactionField:
    matches = [field for field in fields if field.tag == tag]
    if len(matches) > 1 or matches and matches[0].wire != wire:
        raise ValueError("ambiguous committed transaction field")
    return matches[0] if matches else _TransactionField(tag, wire)


def _committed_evm_transaction(data: bytes) -> bytes:
    transaction = _transaction_fields(data)
    body = _transaction_fields(_transaction_field(transaction, 1).data)
    message = _transaction_fields(_transaction_field(body, 1).data)
    if _transaction_field(message, 1).data not in (
        b"/paxprotocol.paxchain.evm.MsgEVMTransaction", b"/ethermint.evm.v1.MsgEthereumTx",
    ):
        raise ValueError("invalid committed EVM message")
    wrapped = _transaction_fields(_transaction_field(message, 2).data)
    payload = _transaction_field(wrapped, 1).data
    if not payload:
        raise ValueError("missing committed EVM transaction")
    if payload[0] >= 192 or 1 <= payload[0] <= 4:
        return payload
    encoded = _transaction_fields(payload)
    layouts = {
        b"/paxprotocol.paxchain.eth.LegacyTx": (0, "uduhdrbbb"),
        b"/paxprotocol.paxchain.eth.AccessListTx": (1, "duduhdrabbb"),
        b"/paxprotocol.paxchain.eth.DynamicFeeTx": (2, "dudduhdrabbb"),
        b"/paxprotocol.paxchain.eth.BlobTx": (3, "dudduhdradx_bbb"),
        b"/paxprotocol.paxchain.eth.SetCodeTx": (4, "dudduhdraobbb"),
    }
    layout = layouts.get(_transaction_field(encoded, 1).data)
    if layout is None:
        raise ValueError("unsupported committed EVM transaction")
    fields = _transaction_fields(_transaction_field(encoded, 2).data)

    def unsigned(number):
        if not 0 <= number < 1 << 256:
            raise ValueError("invalid committed EVM integer")
        return number.to_bytes((number.bit_length() + 7) // 8, "big")

    def hex_bytes(value, size):
        if re.fullmatch(rb"0x[0-9a-fA-F]{" + str(size * 2).encode() + rb"}", value) is None:
            raise ValueError("invalid committed EVM address or hash")
        return bytes.fromhex(value[2:].decode("ascii"))

    def scalar(values, tag, kind):
        field = _transaction_field(values, tag, 0 if kind == "u" else 2)
        value = field.data
        if kind == "u":
            return unsigned(field.value)
        if kind == "d" and value:
            if re.fullmatch(rb"(?:0|[1-9][0-9]{0,77})", value) is None:
                raise ValueError("invalid committed EVM integer")
            return unsigned(int(value))
        if kind == "h" and value:
            return hex_bytes(value, 20)
        if kind == "b":
            if len(value) > 32:
                raise ValueError("invalid committed EVM signature")
            return value.lstrip(b"\0")
        return value

    def repeated(tag, kind):
        result = []
        for field in fields:
            if field.tag != tag:
                continue
            if field.wire != 2:
                raise ValueError("invalid committed EVM list")
            if kind == "x":
                if len(field.data) != 32:
                    raise ValueError("invalid committed EVM blob hash")
                result.append(field.data)
                continue
            members = _transaction_fields(field.data)
            if kind == "o":
                result.append(tuple(scalar(members, i, fmt) for i, fmt in enumerate("dhubbb", 1)))
            else:
                address = hex_bytes(_transaction_field(members, 1).data, 20)
                keys = []
                for member in members:
                    if member.tag == 2:
                        if member.wire != 2:
                            raise ValueError("invalid committed EVM storage key")
                        keys.append(hex_bytes(member.data, 32))
                result.append((address, tuple(keys)))
        return tuple(result)

    tx_type, formats = layout
    values = tuple(
        repeated(tag, kind) if kind in "axo" else scalar(fields, tag, kind)
        for tag, kind in enumerate(formats, 1) if kind != "_"
    )
    return (bytes([tx_type]) if tx_type else b"") + _rlp(values)


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
    if data != raw_transaction and _committed_evm_transaction(data) != raw_transaction:
        raise ValueError("transaction inclusion identity mismatch")
    inclusion_header = header
    if "execution_header" in proof:
        execution = proof["execution_header"]
        if (not isinstance(execution, dict) or execution["chain_id"] != chain_id
                or int(execution["height"]) != receipt_height
                or int(execution["height"]) + 1 != commit["height"]
                or _header_hash(execution) != _hex(header["last_block_id"]["hash"], 32)
                or _hex(execution["next_validators_hash"], 32) != _hex(header["validators_hash"], 32)):
            raise ValueError("execution header binding mismatch")
        inclusion_header = execution
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
    if reconstruct(index, total) != _hex(inclusion_header["data_hash"], 32) or aunts:
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
