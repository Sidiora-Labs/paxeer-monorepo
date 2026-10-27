import argparse
import json
from pathlib import Path
import struct


PROGRAMS_ADDRESS = "0x000000000000000000000000000000000000101d"
CALL_SELECTOR = "0x54bd2b48"
CALL_HEADER = struct.Struct(">32sHHIHII7Q")


def calldata(payload, fee_limit):
    if not 0 <= fee_limit < 2**128:
        raise ValueError("fee limit exceeds uint128")
    if len(payload) < CALL_HEADER.size:
        raise ValueError("truncated native call header")
    program, abi, entry_size, data_size, caps_size, access_size, capacity, *budget = (
        CALL_HEADER.unpack_from(payload)
    )
    lengths = (entry_size, data_size, caps_size, access_size)
    if len(payload) != CALL_HEADER.size + sum(lengths):
        raise ValueError("native call payload length mismatch")
    cursor = CALL_HEADER.size
    dynamic = []
    for length in lengths:
        value = payload[cursor : cursor + length]
        dynamic.append(value)
        cursor += length
    dynamic[0].decode("utf-8")
    head_size = 15 * 32
    offsets = []
    tail = bytearray()
    for value in dynamic:
        offsets.append(head_size + len(tail))
        tail.extend(len(value).to_bytes(32, "big"))
        tail.extend(value)
        tail.extend(bytes(-len(value) % 32))
    words = [abi, *offsets, capacity, *budget, fee_limit]
    head = program + b"".join(value.to_bytes(32, "big") for value in words)
    return CALL_SELECTOR + (head + tail).hex()


def evm(fixture):
    return {
        "to": PROGRAMS_ADDRESS,
        "chainId": 125,
        "selector": CALL_SELECTOR,
        "calldata": calldata(
            bytes.fromhex(fixture["payload_hex"]), int(fixture["fee_limit"])
        ),
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    arguments = parser.parse_args()
    for version in (3, 4):
        path = Path(__file__).with_name(f"native-program-call-v{version}.json")
        fixture = json.loads(path.read_text())
        expected = evm(fixture)
        if arguments.check:
            if fixture.get("evm") != expected:
                raise ValueError(f"EVM call fixture drift: {path}")
        else:
            fixture["evm"] = expected
            path.write_text(json.dumps(fixture, indent=2) + "\n")


if __name__ == "__main__":
    main()
