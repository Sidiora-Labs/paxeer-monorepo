import copy
import hashlib
import json
from pathlib import Path
import re
import sys

from Crypto.Hash import keccak
from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey


MAX_BYTES = 1_048_576
KEY_FIELDS = {"keyId", "publicKey", "firstHeight", "lastHeight"}
FIELDS = {
    "format", "verificationLevel", "signedReceipt", "receiptDigest",
    "receiptKey", "inclusionHeight",
}


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate JSON member")
        result[key] = value
    return result


def canonical_hex(value, length=None):
    if not isinstance(value, str) or re.fullmatch(r"0x(?:[0-9a-f]{2})+", value) is None:
        raise ValueError("noncanonical hex")
    decoded = bytes.fromhex(value[2:])
    if len(decoded) > MAX_BYTES or (length is not None and len(decoded) != length):
        raise ValueError("wrong field length")
    return decoded


def quantity(value):
    if not isinstance(value, str) or re.fullmatch(r"0x(?:0|[1-9a-f][0-9a-f]{0,15})", value) is None:
        raise ValueError("noncanonical uint64 quantity")
    return int(value, 16)


class ReceiptReader:
    def __init__(self, data):
        self.data = data
        self.offset = 0

    def take(self, length):
        end = self.offset + length
        if end > len(self.data):
            raise ValueError("truncated receipt")
        result = self.data[self.offset:end]
        self.offset = end
        return result

    def integer(self, length):
        return int.from_bytes(self.take(length), "big")

    def blob(self, length=None):
        size = self.integer(4)
        if size > MAX_BYTES or (length is not None and size != length):
            raise ValueError("noncanonical receipt field length")
        return self.take(size)


def decode_receipt(signed):
    reader = ReceiptReader(signed)
    if reader.take(6) != bytes.fromhex("000252010002"):
        raise ValueError("not a ProtocolReceipt v2")
    activity_id = reader.blob(32)
    if activity_id == bytes(32):
        raise ValueError("zero activity id")
    reader.take(8)  # global sequence
    for _ in range(3):
        reader.blob(32)  # previous, resulting and activity roots
    result_code = int.from_bytes(reader.take(4), "big", signed=True)
    count = reader.integer(4)
    if count > MAX_BYTES // 48:
        raise ValueError("effect count limit")
    for _ in range(count):
        reader.take(6)  # module id, ordinal, event type
        kind = reader.integer(1)
        monetary = reader.integer(1)
        if kind not in (1, 2, 3) or monetary not in (0, 1) or (monetary and kind != 2):
            raise ValueError("noncanonical effect")
        reader.blob(32)
        if len(reader.blob()) > 256:
            raise ValueError("effect body limit")
    reader.take(16)  # fee charged
    batch_id = reader.blob(32)
    if batch_id[:24] != bytes(24):
        raise ValueError("noncanonical signed height")
    height = int.from_bytes(batch_id[24:], "big")
    if reader.integer(2) <= 11 or reader.integer(4) != 1:
        raise ValueError("unsupported sequencer layout")
    reader.take(5)  # parameter version and operation
    reader.blob(32)  # asset
    reader.take(16)  # amount
    reader.blob(32)  # from
    reader.take(40)  # balances and sequence
    reader.blob(32)  # to
    reader.take(32)  # balances
    reader.blob(32)  # transfer set root
    reader.blob(32)  # authorization hash
    key_id = reader.blob(32)
    reader.take(8)  # timestamp
    prefix_end = reader.offset
    if reader.take(5) != bytes.fromhex("0100000040"):
        raise ValueError("noncanonical signature framing")
    signature = reader.take(64)
    if reader.offset != len(signed):
        raise ValueError("trailing receipt bytes")
    return activity_id, height, key_id, signature, signed[:prefix_end] + b"\0", result_code


def verify(document, pinned_keys):
    if not isinstance(document, dict) or set(document) not in (FIELDS, FIELDS | {"rawTransaction"}):
        raise ValueError("unexpected portable fields")
    if document["format"] != "layerx-receipt-proof-v2" or document["verificationLevel"] != "receipted":
        raise ValueError("unsupported portable evidence")
    carried = document["receiptKey"]
    if not isinstance(carried, dict) or set(carried) != KEY_FIELDS:
        raise ValueError("unexpected key fields")
    canonical_hex(carried["keyId"], 32)
    canonical_hex(carried["publicKey"], 32)
    quantity(carried["firstHeight"])
    quantity(carried["lastHeight"])
    signed = canonical_hex(document["signedReceipt"])
    activity_id, height, key_id, signature, unsigned, _ = decode_receipt(signed)
    matching = [key for key in pinned_keys if canonical_hex(key["keyId"], 32) == key_id]
    if len(matching) != 1:
        raise ValueError("unpinned or ambiguous receipt key")
    pinned = matching[0]
    if not quantity(pinned["firstHeight"]) <= height <= quantity(pinned["lastHeight"]):
        raise ValueError("signed height outside pinned key window")
    if carried != pinned:
        raise ValueError("carried key differs from pinned entry")
    if quantity(document["inclusionHeight"]) < height:
        raise ValueError("inclusion precedes signed height")
    digest = hashlib.sha256(b"LXP/v1/receipt\0" + unsigned).digest()
    if digest != canonical_hex(document["receiptDigest"], 32):
        raise ValueError("portable digest mismatch")
    try:
        Ed25519PublicKey.from_public_bytes(canonical_hex(pinned["publicKey"], 32)).verify(signature, digest)
    except InvalidSignature as error:
        raise ValueError("invalid receipt signature") from error
    if "rawTransaction" in document:
        raw = canonical_hex(document["rawTransaction"])
        if keccak.new(digest_bits=256, data=raw).digest() != activity_id:
            raise ValueError("raw transaction does not match activity id")
    return "0x" + digest.hex(), "0x" + activity_id.hex()


def flip_byte(value, index):
    changed = bytearray(canonical_hex(value))
    changed[index] ^= 1
    return "0x" + changed.hex()


def main():
    fixture = json.loads(Path(sys.argv[1]).read_text(), object_pairs_hook=unique_object)
    pinned_keys = []
    for case in fixture["cases"]:
        key = copy.deepcopy(case["pinnedKey"])
        for field in ("firstHeight", "lastHeight"):
            key[field] = hex(key[field])
        if key not in pinned_keys:
            pinned_keys.append(key)
    encoded = sys.stdin.buffer.read(1_500_001)
    if len(encoded) > 1_500_000:
        raise ValueError("portable length limit")
    document = json.loads(encoded, object_pairs_hook=unique_object)
    digest, activity_id = verify(document, pinned_keys)
    _, height, key_id, _, _, _ = decode_receipt(canonical_hex(document["signedReceipt"]))
    pinned = next(key for key in pinned_keys if canonical_hex(key["keyId"], 32) == key_id)
    later_inclusion = copy.deepcopy(document)
    later_inclusion["inclusionHeight"] = hex(quantity(pinned["lastHeight"]) + 1)
    assert verify(later_inclusion, pinned_keys) == (digest, activity_id)
    boundary_key = dict(pinned, firstHeight=hex(height), lastHeight=hex(height))
    boundary_proof = copy.deepcopy(later_inclusion)
    boundary_proof["receiptKey"] = boundary_key
    assert verify(boundary_proof, [boundary_key]) == (digest, activity_id)
    for first, last in ((height + 1, quantity(later_inclusion["inclusionHeight"])), (0, height - 1)):
        outside_key = dict(pinned, firstHeight=hex(first), lastHeight=hex(last))
        outside_proof = copy.deepcopy(later_inclusion)
        outside_proof["receiptKey"] = outside_key
        try:
            verify(outside_proof, [outside_key])
        except ValueError:
            continue
        raise AssertionError("independent verifier selected a window using inclusion height")
    mutations = []
    signed = canonical_hex(document["signedReceipt"])
    for index in (0, len(signed) // 2, len(signed) - 1):
        altered = copy.deepcopy(document)
        altered["signedReceipt"] = flip_byte(document["signedReceipt"], index)
        mutations.append(altered)
    for field in ("receiptDigest", "rawTransaction"):
        altered = copy.deepcopy(document)
        altered[field] = flip_byte(document[field], 0)
        mutations.append(altered)
    for field in ("keyId", "publicKey", "firstHeight", "lastHeight"):
        altered = copy.deepcopy(document)
        value = document["receiptKey"][field]
        altered["receiptKey"][field] = hex(quantity(value) ^ 1) if field.endswith("Height") else flip_byte(value, 0)
        mutations.append(altered)
    for field, value in (("format", "unknown"), ("verificationLevel", "checkpoint"),
                         ("signedReceipt", 42), ("unrecognised", True),
                         ("inclusionHeight", hex(height - 1))):
        altered = copy.deepcopy(document)
        altered[field] = value
        mutations.append(altered)
    for altered in mutations:
        try:
            verify(altered, pinned_keys)
        except (ValueError, TypeError):
            continue
        raise AssertionError("independent verifier accepted a substitution")
    wrong_key = copy.deepcopy(pinned)
    wrong_key["publicKey"] = flip_byte(wrong_key["publicKey"], 0)
    future_key = dict(pinned, firstHeight=hex(height + 1))
    expired_key = dict(pinned, lastHeight=hex(height - 1))
    key_lists = ([], [wrong_key], [future_key], [expired_key], pinned_keys * 2)
    for untrusted in key_lists:
        try:
            verify(document, untrusted)
        except ValueError:
            continue
        raise AssertionError("independent verifier trusted an invalid key list")
    print(json.dumps({"receipt_digest": digest, "activity_id": activity_id,
                      "mutations_refused": len(mutations), "key_lists_refused": len(key_lists)}))


if __name__ == "__main__":
    main()
