"""Produce adversarial CW721 fixtures from the committed cw721_base contract.

Only the query export is wrapped. Mint, ownership, authorization and transfers
still execute the original contract. Fixtures are gzip-compressed so Go tests
load committed bytecode without a subprocess or a contract compiler.
Run from the repository root: python3 precompiles/solo/testdata/cw721_pages.py
"""
import base64
import gzip
import json
from pathlib import Path


def leb(n):
    result = bytearray()
    while True:
        b = n & 127
        n >>= 7
        result.append(b | (128 if n else 0))
        if not n:
            return bytes(result)


def signed(n):
    result = bytearray()
    while True:
        b = n & 127
        n >>= 7
        more = not ((n == 0 and b < 64) or (n == -1 and b >= 64))
        result.append(b | (128 if more else 0))
        if not more:
            return bytes(result)


def read(data, pos):
    n = shift = 0
    while True:
        b = data[pos]
        pos += 1
        n |= (b & 127) << shift
        if b < 128:
            return n, pos
        shift += 7


def vector(items):
    return leb(len(items)) + b"".join(items)


def const(n):
    return b"\x41" + signed(n)


def get(n):
    return b"\x20" + leb(n)


def set_(n):
    return b"\x21" + leb(n)


def call(n):
    return b"\x10" + leb(n)


def load(offset=0):
    return b"\x28\x02" + leb(offset)


def store(offset=0):
    return b"\x36\x02" + leb(offset)


def load8(offset=0):
    return b"\x2d\x00" + leb(offset)


def store8(offset=0):
    return b"\x3a\x00" + leb(offset)


def emit_bytes(local, data):
    return b"".join(get(local) + const(b) + store8(i) for i, b in enumerate(data))


def wrapped(original, mode, tokens=None):
    sections = []
    pos = 8
    while pos < len(original):
        ident = original[pos]
        size, begin = read(original, pos + 1)
        sections.append((ident, original[begin:begin + size]))
        pos = begin + size
    by_id = dict(sections)
    # All CosmWasm imports in this fixture are functions.
    imports, _ = read(by_id[2], 0)
    functions, pos = read(by_id[3], 0)
    types = []
    for _ in range(functions):
        typ, pos = read(by_id[3], pos)
        types.append(typ)
    count, pos = read(by_id[7], 0)
    exports = []
    named = {}
    for _ in range(count):
        size, pos = read(by_id[7], pos)
        name = by_id[7][pos:pos + size]
        pos += size
        kind = by_id[7][pos]
        index, pos = read(by_id[7], pos + 1)
        exports.append((name, kind, index))
        named[name] = index
    query, allocate = named[b"query"], named[b"allocate"]
    wrapper = imports + functions
    # Locals 2..7 are scratch i32s. Tokens queries start with {"tokens":.
    code = get(1) + load() + set_(2)
    code += get(2) + load8(2) + const(ord("t")) + b"\x46\x04\x40"
    if mode == "repeat":
        # Drop the trailing start_after member and query real ownership from
        # the beginning again. The base contract rejects unknown JSON fields.
        code += get(1) + load(8) + set_(3) + const(0) + set_(4)
        code += b"\x02\x40\x03\x40"  # block / loop
        code += get(4) + const(11) + b"\x6a" + get(3) + b"\x4b\x0d\x01"
        code += const(1)
        for offset, byte in enumerate(b"start_after"):
            code += get(2) + get(4) + b"\x6a" + load8(offset) + const(byte) + b"\x46\x71"
        code += b"\x04\x40"
        # Go's JSON producer orders owner before start_after. Replace the
        # comma and opening quote before this key with the closing braces.
        code += get(2) + get(4) + b"\x6a" + const(2) + b"\x6b" + const(ord("}")) + store8()
        code += get(2) + get(4) + b"\x6a" + const(1) + b"\x6b" + const(ord("}")) + store8()
        code += get(1) + get(4) + store(8)
        code += get(0) + get(1) + call(query) + b"\x0f\x0b"
        code += get(4) + const(1) + b"\x6a" + set_(4) + b"\x0c\x00\x0b\x0b"
    elif mode == "single":
        # Insert an explicit page size into the original tokens query.
        prefix = b'{"tokens":{"limit":1,'
        code += get(1) + load(8) + const(len(prefix) - 11) + b"\x6a" + set_(3)
        code += get(3) + call(allocate) + set_(5)
        code += get(5) + load() + set_(6)
        code += emit_bytes(6, prefix)
        code += const(11) + set_(4)
        code += b"\x02\x40\x03\x40"
        code += get(4) + get(1) + load(8) + b"\x4f\x0d\x01"
        code += get(6) + get(4) + b"\x6a" + const(len(prefix) - 11) + b"\x6a"
        code += get(2) + get(4) + b"\x6a" + load8() + store8()
        code += get(4) + const(1) + b"\x6a" + set_(4) + b"\x0c\x00\x0b\x0b"
        code += get(5) + get(3) + store(8)
        code += get(0) + get(5) + call(query) + b"\x0f"
    else:
        # Deliberately malformed pagination transcript, served by the contract.
        response = json.dumps({"tokens": tokens}, separators=(",", ":")).encode()
        result = json.dumps({"ok": base64.b64encode(response).decode()}, separators=(",", ":")).encode()
        code += const(len(result)) + call(allocate) + set_(5)
        code += get(5) + load() + set_(6) + emit_bytes(6, result)
        code += get(5) + const(len(result)) + store(8) + get(5) + b"\x0f"
    code += b"\x0b" + get(0) + get(1) + call(query) + b"\x0b"
    body = b"\x01\x06\x7f" + code
    new_sections = {
        3: leb(functions + 1) + by_id[3][read(by_id[3], 0)[1]:] + leb(types[query - imports]),
        7: vector([leb(len(name)) + name + bytes([kind]) + leb(wrapper if name == b"query" else idx)
                   for name, kind, idx in exports]),
        10: leb(functions + 1) + by_id[10][read(by_id[10], 0)[1]:] + leb(len(body)) + body,
    }
    return original[:8] + b"".join(bytes([ident]) + leb(len(new_sections.get(ident, data))) + new_sections.get(ident, data)
                                   for ident, data in sections)


if __name__ == "__main__":
    base = Path("contracts/wasm/cw721_base.wasm").read_bytes()
    variants = {
        "repeat": ("repeat", None),
        "single": ("single", None),
        "duplicate": ("transcript", ["0000", "0000"]),
        "backwards": ("transcript", ["0001", "0000"]),
        "empty_id": ("transcript", [""]),
        "oversized": ("transcript", [f"{i:04d}" for i in range(257)]),
    }
    for name, (mode, tokens) in variants.items():
        Path(f"precompiles/solo/testdata/cw721_{name}.wasm.gz").write_bytes(
            gzip.compress(wrapped(base, mode, tokens), mtime=0))
