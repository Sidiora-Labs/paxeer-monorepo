import hashlib
import json
import shutil
import subprocess
import sys
from pathlib import Path


FIXTURES = Path(__file__).with_name("fixtures")
GENERATOR = FIXTURES / "generate_native_lifecycle_fixtures.py"


def _dynamic_argument(arguments, index, expected_offset):
    offset = int.from_bytes(arguments[index * 32:(index + 1) * 32], "big")
    assert offset == expected_offset
    length = int.from_bytes(arguments[offset:offset + 32], "big")
    start = offset + 32
    end = start + length
    padded_end = end + (-length % 32)
    assert padded_end <= len(arguments)
    assert arguments[end:padded_end] == bytes(-length % 32)
    return arguments[start:end], padded_end


def test_deploy_and_upgrade_vectors_use_layerxprograms_selectors():
    for operation, selector, ordinal in (
        ("deploy", "bde3ff66", 1),
        ("upgrade", "a860929f", 2),
    ):
        native_documents = []
        for version in (3, 4):
            path = FIXTURES / f"native-program-{operation}-v{version}.json"
            document = json.loads(path.read_text())
            assert document["name"] == path.stem
            assert (
                document["protocol_version"], document["module"], document["ordinal"]
            ) == (3, 9, ordinal)
            wrapper = document["evm"]
            assert wrapper["to"] == "0x101D"
            assert wrapper["chainId"] == 125
            assert wrapper["calldata"].startswith("0x" + selector)
            calldata = bytes.fromhex(wrapper["calldata"][2:])
            assert calldata[:4].hex() == selector
            assert wrapper["calldata"] == "0x" + calldata.hex()
            arguments = calldata[4:]
            assert len(arguments) % 32 == 0
            payload = bytes.fromhex(document["payload_hex"])
            signed = bytes.fromhex(document["signed_activity_hex"])
            assert bytes([11]) + len(payload).to_bytes(4, "big") + payload in signed
            assert (
                hashlib.sha256(b"LXP/v1/activity-id\0" + signed).hexdigest()
                == document["activity_id_hex"]
            )
            fixed = (
                arguments[:32]
                + int.from_bytes(arguments[32:64], "big").to_bytes(2, "big")
                + int.from_bytes(arguments[64:96], "big").to_bytes(1, "big")
                + b"\0"
                + arguments[96:160]
            )
            if operation == "deploy":
                interface, offset = _dynamic_argument(arguments, 5, 7 * 32)
                wasm, end = _dynamic_argument(arguments, 6, offset)
                reconstructed = fixed + len(wasm).to_bytes(4, "big")
                if interface:
                    reconstructed += len(interface).to_bytes(4, "big")
                reconstructed += interface + wasm
            else:
                hook, offset = _dynamic_argument(arguments, 5, 9 * 32)
                interface, offset = _dynamic_argument(arguments, 6, offset)
                wasm, end = _dynamic_argument(arguments, 7, offset)
                fee_limit = int.from_bytes(arguments[256:288], "big")
                assert fee_limit == 1000
                assert b"\x09" + fee_limit.to_bytes(16, "big") in signed
                reconstructed = (
                    fixed + len(hook).to_bytes(2, "big")
                    + len(wasm).to_bytes(4, "big")
                )
                if interface or payload[34] & 2:
                    reconstructed += len(interface).to_bytes(4, "big")
                reconstructed += hook + interface + wasm
            assert end == len(arguments)
            assert reconstructed == payload
            native_documents.append({
                key: value for key, value in document.items() if key != "name"
            })
        assert native_documents[0] == native_documents[1]


def test_lifecycle_generator_check_detects_drift(tmp_path):
    generator = tmp_path / GENERATOR.name
    shutil.copyfile(GENERATOR, generator)
    originals = {}
    for operation in ("deploy", "upgrade"):
        for version in (3, 4):
            source = FIXTURES / f"native-program-{operation}-v{version}.json"
            destination = tmp_path / source.name
            shutil.copyfile(source, destination)
            originals[destination] = destination.read_bytes()

    command = [sys.executable, str(generator), "--evm-only"]
    baseline = subprocess.run(
        command + ["--check"], capture_output=True, text=True, timeout=30
    )
    assert baseline.returncode == 0, baseline.stderr
    for destination, original in originals.items():
        for byte_index in (0, -8):
            document = json.loads(original)
            calldata = bytearray.fromhex(document["evm"]["calldata"][2:])
            calldata[byte_index] ^= 1
            document["evm"]["calldata"] = "0x" + calldata.hex()
            damaged = json.dumps(document, indent=2) + "\n"
            destination.write_text(damaged)
            result = subprocess.run(
                command + ["--check"], capture_output=True, text=True, timeout=30
            )
            assert result.returncode != 0
            assert f"native lifecycle fixture drift: {destination}" in result.stderr
            assert destination.read_text() == damaged
            repaired = subprocess.run(command, capture_output=True, text=True, timeout=30)
            assert repaired.returncode == 0, repaired.stderr
            assert destination.read_bytes() == original
    final = subprocess.run(
        command + ["--check"], capture_output=True, text=True, timeout=30
    )
    assert final.returncode == 0, final.stderr
    assert all(
        destination.read_bytes() == original
        for destination, original in originals.items()
    )
