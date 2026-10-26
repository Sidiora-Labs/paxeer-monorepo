import hashlib
import json
import subprocess
from pathlib import Path


def test_fixture_output_matches_golden(tmp_path: Path) -> None:
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
    take(5 * 32)  # Public key, batch ID, asset and the two state roots.
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
    assert {
        "batch_header_hash_hex": hashlib.sha256(
            b"LXP/v1/batch-header\0" + header
        ).hexdigest(),
        "batch_header_signature_hex": signature.hex(),
    } == golden
