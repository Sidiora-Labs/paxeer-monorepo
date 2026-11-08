"""Keep the public qualification ledger parseable and its real checker green."""

from collections import Counter
import os
from pathlib import Path
import re
import subprocess
import tempfile


ROOT = Path(__file__).resolve().parents[2]
LEDGER = ROOT / "spec/layerx-merge/qualification.kvx"


def test_ledger_headers_unique():
    headers = re.findall(r"^\[([^\]\n]+)\]\s*$", LEDGER.read_text(), re.MULTILINE)
    assert headers, "The qualification ledger must contain records"
    duplicates = {name: count for name, count in Counter(headers).items() if count > 1}
    assert not duplicates, duplicates


def test_ledger_strings_terminated():
    parsed = subprocess.run(
        [
            "bash",
            "-c",
            'source scripts/ci/beta-ledger-check.sh; awk "$KVX_PARSER" "$1"',
            "ledger-parser",
            str(LEDGER),
        ],
        cwd=ROOT,
        capture_output=True,
        text=True,
        timeout=60,
    )
    assert parsed.returncode == 0, parsed.stdout + parsed.stderr
    rows = [line.split("\x1e") for line in parsed.stdout.split("\n") if line]
    assert rows, "The real KVX parser must read the ledger"
    errors = [row for row in rows if row[0] == "error"]
    assert not errors, errors
    assert any(row[0] == "pair" and row[3] == "string" for row in rows)
    invalid_strings = [
        row
        for row in rows
        if row[0] == "pair" and (row[3] != "string" or not row[4].strip())
    ]
    assert not invalid_strings, invalid_strings


def test_beta_ledger_check_passes():
    environment = os.environ.copy()
    for name in (
        "PAXEER_X_BETA_LEDGER_FILE",
        "PAXEER_X_BETA_SPEC_FILE",
        "PAXEER_X_BETA_CANDIDATE",
    ):
        environment.pop(name, None)
    with tempfile.TemporaryDirectory(prefix="qualification-ledger-") as evidence:
        result = subprocess.run(
            ["bash", "scripts/ci/beta-ledger-check.sh", "--evidence-root", evidence],
            cwd=ROOT,
            env=environment,
            capture_output=True,
            text=True,
            timeout=60,
        )
    assert result.returncode == 0, result.stdout + result.stderr
    assert "spec/layerx-merge/qualification.kvx" in result.stdout, result.stdout
    assert re.search(r"\b[1-9][0-9]* finding record\(s\)", result.stdout), result.stdout
    assert "1 observation record(s)" in result.stdout, result.stdout
    assert "violation(s)" not in result.stdout + result.stderr
