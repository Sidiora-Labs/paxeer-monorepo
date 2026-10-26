import hashlib
import importlib.util
import json
import shutil
import subprocess
import sys
from pathlib import Path

import pytest


FIXTURES = Path(__file__).with_name("fixtures")
GENERATOR = FIXTURES / "generate_native_lifecycle_fixtures.py"


def test_wind_down_vectors_are_layerxprograms_calls(tmp_path, request):
    assert request.config.getoption("--import-mode") == "importlib"
    originals = {}
    payloads = []
    for operation, mode, selector in (
        ("route", 1, "23330dd5"),
        ("deprecate", 2, "86ba1a99"),
        ("tombstone", 3, "a512f1a9"),
        ("exit", 4, "4dc50730"),
    ):
        native_documents = []
        for version in (3, 4):
            path = FIXTURES / f"native-program-wind-down-{operation}-v{version}.json"
            document = json.loads(path.read_text())
            assert document["name"] == path.stem
            assert (
                document["protocol_version"], document["module"], document["ordinal"]
            ) == (3, 9, 7)
            payload = bytes.fromhex(document["payload_hex"])
            assert document["payload_hex"] == payload.hex()
            assert payload[32] == mode
            signed = bytes.fromhex(document["signed_activity_hex"])
            assert b"\x0b" + len(payload).to_bytes(4, "big") + payload in signed
            assert (
                hashlib.sha256(b"LXP/v1/activity-id\0" + signed).hexdigest()
                == document["activity_id_hex"]
            )
            wrapper = document["evm"]
            assert wrapper["to"] == "0x101D"
            assert wrapper["chainId"] == 125
            calldata = bytes.fromhex(wrapper["calldata"][2:])
            assert wrapper["calldata"] == "0x" + calldata.hex()
            assert calldata[:4].hex() == selector
            arguments = calldata[4:]
            assert len(arguments) % 32 == 0
            assert arguments[:32] == payload[:32]
            reconstructed = arguments[:32] + bytes([mode])
            if mode == 1:
                offset = int.from_bytes(arguments[128:160], "big")
                assert offset == 160
                seed_length = int.from_bytes(arguments[offset:offset + 32], "big")
                assert seed_length <= 128
                start = offset + 32
                end = start + seed_length
                padded_end = end + (-seed_length % 32)
                assert len(arguments) == padded_end
                assert arguments[end:] == bytes(-seed_length % 32)
                reconstructed += (
                    arguments[32:128]
                    + seed_length.to_bytes(2, "big")
                    + arguments[start:end]
                )
            elif mode == 2:
                assert len(arguments) == 96
                deadline = int.from_bytes(arguments[64:96], "big")
                assert 0 <= deadline < 1 << 64
                reconstructed += arguments[32:64] + deadline.to_bytes(8, "big")
            elif mode == 3:
                assert len(arguments) == 32
            else:
                assert len(arguments) == 64
                reconstructed += arguments[32:64]
            assert reconstructed == payload
            payloads.append(payload)
            native_documents.append({
                key: value for key, value in document.items() if key != "name"
            })
            destination = tmp_path / path.name
            shutil.copyfile(path, destination)
            originals[destination] = destination.read_bytes()
        assert native_documents[0] == native_documents[1]

    generator = tmp_path / GENERATOR.name
    shutil.copyfile(GENERATOR, generator)
    command = [sys.executable, str(generator), "--wind-down-only"]
    baseline = subprocess.run(
        command + ["--check"], capture_output=True, text=True, timeout=30
    )
    assert baseline.returncode == 0, baseline.stderr
    for destination, original in originals.items():
        for field, value in (("to", "0x101A"), ("chainId", 1), ("calldata", None)):
            document = json.loads(original)
            if field == "calldata":
                calldata = bytearray.fromhex(document["evm"][field][2:])
                calldata[4] ^= 1
                value = "0x" + calldata.hex()
            document["evm"][field] = value
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
    assert all(path.read_bytes() == original for path, original in originals.items())

    spec = importlib.util.spec_from_file_location("native_lifecycle", GENERATOR)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    for payload in payloads:
        for malformed in (payload[:32], payload[:-1], payload + b"\0"):
            with pytest.raises(ValueError):
                module.wind_down(malformed)
    route = payloads[0]
    for malformed in (
        route[:32] + b"\0" + route[33:],
        route[:129] + (129).to_bytes(2, "big") + bytes(129),
        route[:129] + (5).to_bytes(2, "big") + route[131:],
    ):
        with pytest.raises(ValueError):
            module.wind_down(malformed)
