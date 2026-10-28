import hashlib
import json
import subprocess
from pathlib import Path


def fixture_output_matches_golden(tmp_path: Path) -> tuple[bytes, bytes, bytes]:
    root = Path(__file__).resolve().parents[1]
    build_dir = tmp_path / "build"
    binary = build_dir / "tests" / "explorer_fixture"
    subprocess.run(
        ["make", f"BUILD_DIR={build_dir}", str(binary)],
        cwd=root,
        check=True,
    )
    output = subprocess.run(
        [str(binary)], cwd=root, check=True, capture_output=True
    ).stdout
    assert len(output) <= 1_048_576
    offset = 0

    def take(length: int) -> bytes:
        nonlocal offset
        end = offset + length
        assert end <= len(output), "truncated explorer fixture envelope"
        value = output[offset:end]
        offset = end
        return value

    def sized() -> bytes:
        return take(int.from_bytes(take(4), "big"))

    assert take(5) == b"LXEF\x01"
    sized()  # Receipt.
    public_key = take(32)
    take(4 * 32)  # Batch ID, asset and the two state roots.
    sized()  # Canonical activity.
    sized()  # Activity inclusion proof.
    header = sized()
    assert len(header) == 354
    signature = take(64)
    take(32 + 8 + 8 + 32)  # Sequencer ID, authorized batch range and activity root.
    assert offset == len(output), "trailing explorer fixture envelope bytes"

    golden = json.loads(
        (root / "tests" / "explorer_fixture_golden.json").read_text(encoding="utf-8")
    )
    digest = hashlib.sha256(b"LXP/v1/batch-header\0" + header).digest()
    assert {
        "batch_header_hash_hex": digest.hex(),
        "batch_header_signature_hex": signature.hex(),
    } == golden
    return public_key, digest, signature


def test_fixture_output_matches_golden(tmp_path: Path) -> None:
    fixture_output_matches_golden(tmp_path)


def test_fixture_signature_verifies(tmp_path: Path) -> None:
    public_key, digest, signature = fixture_output_matches_golden(tmp_path)
    public_key_path = tmp_path / "sequencer-public-key.der"
    digest_path = tmp_path / "batch-header-hash.bin"
    signature_path = tmp_path / "batch-header-signature.bin"
    public_key_path.write_bytes(bytes.fromhex("302a300506032b6570032100") + public_key)
    digest_path.write_bytes(digest)
    signature_path.write_bytes(signature)
    verification = subprocess.run(
        [
            "openssl",
            "pkeyutl",
            "-verify",
            "-pubin",
            "-keyform",
            "DER",
            "-inkey",
            str(public_key_path),
            "-rawin",
            "-in",
            str(digest_path),
            "-sigfile",
            str(signature_path),
        ],
        capture_output=True,
        check=False,
    )
    assert verification.returncode == 0, verification.stdout + verification.stderr
