"""Run tools/ci with the declared ledger deferral and check every settlement."""

import re
import subprocess
import sys
from pathlib import Path
import tomllib


ROOT = Path(__file__).resolve().parents[2]
RECORD = ROOT / "tools/ci/tests/repair_22_39_54.kvx"
DESELECT_FILE = ROOT / "tests/ci/data/tools_ci_deselect.txt"
LEDGER_TEST = (
    "tools/ci/tests/test_beta_ledger_paths.py::BetaLedgerPathsTest::"
    "test_ledger_check_reads_layerx_merge_ledger"
)


def tools_ci_deselections():
    deselections = DESELECT_FILE.read_text(encoding="utf-8").splitlines()
    assert len(deselections) == len(set(deselections)), deselections
    assert set(deselections) <= {LEDGER_TEST}, deselections
    return deselections


def test_tools_ci_suite_collects_and_passes():
    command = [sys.executable, "-m", "pytest", "-q", "tools/ci/tests"]
    deselections = tools_ci_deselections()
    for test_id in deselections:
        command.extend(["--deselect", test_id])
    result = subprocess.run(
        command,
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=False,
        timeout=600,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    assert re.search(r"\b[1-9][0-9]* passed\b", result.stdout), result.stdout
    assert "no tests ran" not in result.stdout
    if deselections:
        assert "1 deselected" in result.stdout, result.stdout


def test_repair_record_settles_every_baseline_entry():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    meta = record["meta"]
    assert meta["task"] == "22.39.54"
    assert re.fullmatch(r"[0-9a-f]{40}", meta["baseline_revision"])
    assert meta["baseline_command"] == "python3 -m pytest -q tools/ci/tests"
    assert meta["baseline_exit"] == 1
    baseline = meta["baseline_entries"]
    assert len(baseline) == 19
    assert len(set(baseline)) == len(baseline)
    settlements = list(record["baseline"].values())
    assert {entry["test"] for entry in settlements} == set(baseline)
    for entry in settlements:
        assert entry["reason"].strip(), entry
        if entry["test"] == LEDGER_TEST:
            assert entry["settlement"] == "deferred to task 22.39.87", entry
            assert entry["status"] == "deferred", entry
            assert entry["owner_task"] == "22.39.87", entry
            assert entry["decision"] == 2800 and entry["gotcha"] == 3445, entry
            continue
        assert entry["settlement"] in {
            "stale test rewritten to the live rule",
            "producer repaired",
            "skip with tool name",
        }, entry
        assert entry["status"] == "settled", entry
        if entry["settlement"] == "skip with tool name":
            assert entry["tool"] and entry["command"], entry
    assert sum(entry["status"] == "settled" for entry in settlements) == 18
    classification = record["ledger_classification"]
    assert classification["exit"] == 1
    assert classification["violation_count"] == 6448
    assert sum(
        classification[rule]
        for rule in (
            "duplicate_keys",
            "duplicate_sections",
            "invalid_record_headers",
            "invalid_severity",
            "invalid_value_type",
            "missing_keys",
            "unterminated_quoted_values",
        )
    ) == classification["violation_count"]
    continuation = record["continuation"]
    assert continuation["owner_task"] == "22.39.87"
    assert continuation["deselect_file"] == str(DESELECT_FILE.relative_to(ROOT))
    assert continuation["deferred_test"] == LEDGER_TEST
    assert re.fullmatch(r"[0-9a-f]{40}", continuation["prior_revision"])
    assert "6448 violation(s)" in continuation["prior_output_summary"]
    evidence = meta["qualification_evidence"]
    assert "/out/run/gates.jsonl" in evidence
    assert "revision" in evidence and "hf-done" in evidence
