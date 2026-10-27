import struct
import subprocess
from pathlib import Path


BUILDER = Path(__file__).resolve().parents[2] / "build/bin/layerx-genesis-build"


def test_custody_profile_argument_is_refused(tmp_path):
    result = subprocess.run(
        [BUILDER, "manifest", "key", "out", "--custody-profile", "profile"],
        cwd=tmp_path,
        capture_output=True,
        timeout=10,
        check=False,
    )

    assert result.returncode == 2
    assert result.stdout == b""
    assert result.stderr == b""
    assert list(tmp_path.iterdir()) == []


def test_custody_registry_argument_is_refused(tmp_path):
    output_directory = tmp_path / "out"
    output_directory.mkdir()
    result = subprocess.run(
        [BUILDER, "manifest", "key", "out", "--custody-registry", "registry"],
        cwd=tmp_path,
        capture_output=True,
        timeout=10,
        check=False,
    )

    assert result.returncode == 2
    assert result.stdout == b""
    assert result.stderr == b""
    assert list(output_directory.iterdir()) == []
    assert list(tmp_path.iterdir()) == [output_directory]


def test_fresh_build_writes_no_handover_trust(tmp_path):
    governance_public_key = bytes.fromhex(
        "d75a980182b10ab7d54bfed3c964073a"
        "0ee172f3daa62325af021a68f707511a"
    )
    governance_parameter = struct.pack(
        ">H32s32s", 7, b"handover-authority", governance_public_key
    )
    request = b"".join(
        [
            struct.pack(">4sBHIQH", b"LXGB", 1, 3, 42, 1700000000000, 2),
            governance_parameter,
            struct.pack(
                ">H32s32s", 7, b"parameter-version", (1).to_bytes(32, "big")
            ),
            struct.pack(
                ">H32s33s16s",
                1,
                b"\x01" + bytes(31),
                b"\x02" + bytes(31) + b"\x03",
                bytes(16),
            ),
            bytes([0xA1]) * 32,
            struct.pack(">I9QQB", 1, 1, 1, 1, 1, 1, 8, 8, 64, 8, 1, 1),
            struct.pack(
                ">I7Q6Q", 1, 1, 1, 2, 4, 1, 1, 100, 100, 1, 1, 10, 1, 1000
            ),
        ]
    )
    request_path = tmp_path / "request.lxgb"
    request_path.write_bytes(request)
    signer_path = tmp_path / "signer.key"
    signer_path.write_bytes(bytes([0x22]) * 32)
    signer_path.chmod(0o600)
    output_directory = tmp_path / "out"
    assert not output_directory.exists()

    result = subprocess.run(
        [BUILDER, request_path, signer_path, output_directory],
        cwd=tmp_path,
        capture_output=True,
        timeout=10,
        check=False,
    )

    assert result.returncode == 0, (result.stdout, result.stderr)
    encoded_governance_parameter = struct.pack(
        ">HI32sI32s", 7, 32, b"handover-authority", 32, governance_public_key
    )
    assert (
        encoded_governance_parameter
        in (output_directory / "genesis.manifest").read_bytes()
    )
    assert (output_directory / "00000000000000000000.lxs").stat().st_size > 0
    assert (output_directory / "paxeer-registration-request.lxrr").stat().st_size == 73
    assert (output_directory / "paxeer-deployment-descriptor.lxgd").stat().st_size == 105
    assert not (output_directory / "genesis-handover-trust.lxt").exists()
