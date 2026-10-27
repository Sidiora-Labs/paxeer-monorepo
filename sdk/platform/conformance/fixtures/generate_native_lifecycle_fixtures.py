import argparse
import json
import subprocess
from pathlib import Path


WIND_DOWN_MODES = {
    "wind-down-route": 1,
    "wind-down-deprecate": 2,
    "wind-down-tombstone": 3,
    "wind-down-exit": 4,
}


def _word(value):
    return value.to_bytes(32, "big")


def _calldata(selector, head, dynamic):
    tail = b""
    for index, value in dynamic:
        head[index] = _word(32 * len(head) + len(tail))
        tail += _word(len(value)) + value + bytes(-len(value) % 32)
    return "0x" + selector + (b"".join(head) + tail).hex()


def _lifecycle_parts(payload, upgrading):
    header = 106 if upgrading else 104
    if len(payload) < header or payload[35] != 0:
        raise ValueError("invalid native lifecycle header")
    hook_length = int.from_bytes(payload[100:102], "big") if upgrading else 0
    wasm_length = int.from_bytes(payload[header - 4:header], "big")
    framed = len(payload) != header + hook_length + wasm_length
    if upgrading and payload[34] & 2:
        framed = True
    interface_length = 0
    if framed:
        if len(payload) < header + 4:
            raise ValueError("truncated native lifecycle interface length")
        interface_length = int.from_bytes(payload[header:header + 4], "big")
        header += 4
    if (
        not wasm_length
        or len(payload) != header + hook_length + interface_length + wasm_length
    ):
        raise ValueError("invalid native lifecycle payload lengths")
    hook = payload[header:header + hook_length]
    interface = payload[header + hook_length:header + hook_length + interface_length]
    wasm = payload[header + hook_length + interface_length:]
    return hook, interface, wasm


def _lifecycle_head(payload):
    return [
        payload[:32],
        _word(int.from_bytes(payload[32:34], "big")),
        _word(payload[34]),
        payload[36:68],
        payload[68:100],
    ]


def deploy(payload):
    _, interface, wasm = _lifecycle_parts(payload, False)
    return _calldata(
        "bde3ff66",
        _lifecycle_head(payload) + [b"", b""],
        [(5, interface), (6, wasm)],
    )


def upgrade(payload, fee_limit):
    if not 0 <= fee_limit < 1 << 128:
        raise ValueError("upgrade fee limit exceeds uint128")
    hook, interface, wasm = _lifecycle_parts(payload, True)
    return _calldata(
        "a860929f",
        _lifecycle_head(payload) + [b"", b"", b"", _word(fee_limit)],
        [(5, hook), (6, interface), (7, wasm)],
    )


def wind_down(payload):
    if len(payload) < 33:
        raise ValueError("truncated native wind-down header")
    program = payload[:32]
    mode = payload[32]
    if mode == 1:
        if len(payload) < 131:
            raise ValueError("truncated native wind-down route")
        seed_length = int.from_bytes(payload[129:131], "big")
        if seed_length > 128 or len(payload) != 131 + seed_length:
            raise ValueError("invalid native wind-down route seed length")
        return _calldata(
            "23330dd5",
            [program, payload[33:65], payload[65:97], payload[97:129], b""],
            [(4, payload[131:])],
        )
    if mode == 2:
        if len(payload) != 73:
            raise ValueError("invalid native wind-down deprecate length")
        return _calldata(
            "86ba1a99",
            [program, payload[33:65], _word(int.from_bytes(payload[65:73], "big"))],
            [],
        )
    if mode == 3:
        if len(payload) != 33:
            raise ValueError("invalid native wind-down tombstone length")
        return _calldata("a512f1a9", [program], [])
    if mode == 4:
        if len(payload) != 65:
            raise ValueError("invalid native wind-down exit length")
        return _calldata("4dc50730", [program, payload[33:65]], [])
    raise ValueError("unknown native wind-down mode")


def evm(document):
    payload = bytes.fromhex(document["payload_hex"])
    if not payload or payload.hex() != document["payload_hex"]:
        raise ValueError("native lifecycle payload is not canonical hexadecimal")
    if (document["protocol_version"], document["module"]) != (3, 9):
        raise ValueError("invalid native lifecycle operation binding")
    if document["ordinal"] == 1:
        calldata = deploy(payload)
    elif document["ordinal"] == 2:
        # dump_lifecycle_payload in the native encoder fixes the fee limit at 1000.
        calldata = upgrade(payload, 1000)
    elif document["ordinal"] == 7:
        calldata = wind_down(payload)
    else:
        raise ValueError("expected native deploy, upgrade or wind-down vector")
    return {"to": "0x101D", "chainId": 125, "calldata": calldata}


def write_evm_fixtures(check, wind_down_only=False):
    operations = WIND_DOWN_MODES if wind_down_only else {"deploy": 0, "upgrade": 0}
    for operation, mode in operations.items():
        for version in (3, 4):
            name = f"native-program-{operation}-v{version}"
            destination = Path(__file__).with_name(name + ".json")
            document = json.loads(destination.read_text())
            ordinal = 7 if mode else (1 if operation == "deploy" else 2)
            if (
                document["name"] != name
                or document["ordinal"] != ordinal
                or (mode and document["payload_hex"][64:66] != f"{mode:02x}")
            ):
                raise ValueError(f"invalid native lifecycle operation binding: {destination}")
            document["evm"] = evm(document)
            contents = json.dumps(document, indent=2) + "\n"
            if check:
                if destination.read_text() != contents:
                    raise ValueError(f"native lifecycle fixture drift: {destination}")
            else:
                destination.write_text(contents)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--encoder", type=Path)
    parser.add_argument("--check", action="store_true")
    parser.add_argument(
        "--evm-only",
        action="store_true",
        help="regenerate or check EVM wrappers from retained deploy and upgrade payloads",
    )
    parser.add_argument(
        "--wind-down-only",
        action="store_true",
        help="regenerate or check EVM wrappers from retained wind-down payloads",
    )
    arguments = parser.parse_args()
    if arguments.evm_only and arguments.wind_down_only:
        parser.error("--evm-only and --wind-down-only are mutually exclusive")
    if arguments.wind_down_only:
        write_evm_fixtures(arguments.check, wind_down_only=True)
        return
    if arguments.evm_only:
        write_evm_fixtures(arguments.check)
        return
    if arguments.encoder is None:
        parser.error("--encoder is required unless --evm-only or --wind-down-only is selected")
    encoded = subprocess.run(
        [str(arguments.encoder.resolve()), "--dump-native-lifecycle"],
        check=True,
        capture_output=True,
        text=True,
    )
    expected = {
        "native-program-call-v3": 3,
        "native-program-deploy-v3": 1,
        "native-program-upgrade-v3": 2,
        "native-program-wind-down-route-v3": 7,
        "native-program-wind-down-deprecate-v3": 7,
        "native-program-wind-down-tombstone-v3": 7,
        "native-program-wind-down-exit-v3": 7,
    }
    subprocess.run(
        [str(arguments.encoder.resolve()), "--stored-historical-v3",
         str(Path(__file__).with_name("receipt-programs-executed-v3.json"))],
        check=True, timeout=120,
    )
    for name in expected:
        subprocess.run(
            [str(arguments.encoder.resolve()), "--stored-historical-lifecycle",
             str(Path(__file__).with_name(name + ".json"))],
            check=True, timeout=120,
        )
    documents = {}
    for line in encoded.stdout.splitlines():
        document = json.loads(line)
        name = document["name"]
        if name not in expected or name in documents:
            raise ValueError("native encoder returned an unknown or duplicate vector")
        if (document["protocol_version"], document["module"], document["ordinal"]) != (
            3, 9, expected[name]
        ):
            raise ValueError("native encoder returned a different operation binding")
        payload = bytes.fromhex(document["payload_hex"])
        if not payload or payload.hex() != document["payload_hex"]:
            raise ValueError("native encoder returned non-canonical hexadecimal")
        if document["ordinal"] in (1, 2, 7):
            document["evm"] = evm(document)
        document["name"] = name.removesuffix("-v3") + "-v4"
        documents[name] = json.dumps(document, indent=2) + "\n"
    if set(documents) != set(expected):
        raise ValueError("native encoder omitted a lifecycle vector")
    for name, contents in documents.items():
        destination = Path(__file__).with_name(name.removesuffix("-v3") + "-v4.json")
        if arguments.check:
            if destination.read_text() != contents:
                raise ValueError(f"native lifecycle fixture drift: {destination}")
        else:
            destination.write_text(contents)

    write_evm_fixtures(arguments.check)
    write_evm_fixtures(arguments.check, wind_down_only=True)

    from generate_executed_program_v4_fixture import main as executed_v4
    executed_v4()


if __name__ == "__main__":
    main()
