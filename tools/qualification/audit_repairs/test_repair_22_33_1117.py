"""Offline source and real result-consumer proof for accountability-local-harness."""

import ast
import csv
import hashlib
import importlib.util
from pathlib import Path
import re
import shlex
import subprocess
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
MODEL = ROOT / "consensus/spec/light-client/accountability"
BENCHMARK = MODEL / "001indinv-apalache.csv"
HISTORICAL = MODEL / "results/001indinv-apalache-unstable.csv"
SCRIPT = (MODEL / "run.sh").read_text(encoding="utf-8")
PIN = "1b76b8e70549f6cd71a758b4014e7f7682d7fd44"
SPEC = importlib.util.spec_from_file_location("accountability_experiment", MODEL / "experiment.py")
EXPERIMENT = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(EXPERIMENT)


def result_rows():
    with HISTORICAL.open(encoding="utf-8", newline="") as stream:
        reader = csv.DictReader(stream, quotechar="'")
        return reader.fieldnames, list(reader)


def write_rows(path, fields, rows):
    with path.open("w", encoding="utf-8", newline="") as stream:
        writer = csv.DictWriter(stream, fieldnames=fields, quotechar="'")
        writer.writeheader()
        writer.writerows(rows)


def test_runner_shell_syntax_and_explicit_pinned_locations():
    # The task explicitly requires bash parsing; no experiment/toolchain is run.
    parsed = subprocess.run(
        ["bash", "-n", str(MODEL / "run.sh")], capture_output=True, text=True, check=False
    )
    assert parsed.returncode == 0, parsed.stderr
    assert SCRIPT.startswith("#!/bin/sh\n")
    assert "set -eu\n" in SCRIPT
    assert f"HARNESS_REVISION={PIN}\n" in SCRIPT
    for variable in ("APALACHE_TESTS_DIR", "APALACHE_DIR", "APALACHE_REVISION", "RESULTS_ROOT"):
        assert re.search(r'^: "\$\{' + variable + r':\?[^\n]+\}"$', SCRIPT, re.M)
    assert "SCRIPTS_DIR=$APALACHE_TESTS_DIR/scripts\n" in SCRIPT
    assert "~/devl" not in SCRIPT
    assert "unstable" not in SCRIPT
    assert "make -e" not in SCRIPT


def test_prerequisites_guard_generation_and_archive_creation():
    before_archive = SCRIPT.split("ARCHIVE=$(mktemp", 1)[0]
    for fact in (
        'for prerequisite in git python3 bash java mktemp; do',
        'command -v "$prerequisite" >/dev/null 2>&1 || fail',
        'for prerequisite in /usr/bin/time /usr/bin/timeout; do',
        '[ -x "$prerequisite" ] || fail',
        '[ "${#APALACHE_REVISION}" -eq 40 ] || fail',
        '*[!0-9a-f]*) fail',
        '*[!a-zA-Z0-9_./:-]*) fail',
        '"$APALACHE_TESTS_DIR:$HARNESS_REVISION" "$APALACHE_DIR:$APALACHE_REVISION"',
        '[ "$(git -C "$checkout" rev-parse --show-toplevel)" = "$checkout" ] || fail',
        '[ "$(git -C "$checkout" rev-parse HEAD)" = "$revision" ] || fail',
        'checkout_status=$(git -C "$checkout" status --porcelain --untracked-files=all)',
        '[ -z "$checkout_status" ] || fail',
        'for script in mk-run.py parse-logs.py; do',
        '[ -r "$SCRIPTS_DIR/$script" ] || fail',
        '[ -x "$APALACHE_DIR/bin/apalache-mc" ] || fail',
        '[ -w "$RESULTS_ROOT" ] || fail',
        'model_status=$(git -C "$MODEL_DIR" status --porcelain --untracked-files=all -- .)',
        '[ -z "$model_status" ] || fail',
        'python3 "$MODEL_DIR/experiment.py" validate-benchmarks "$MODEL_DIR/$BENCHMARK.csv"',
    ):
        assert fact in before_archive, fact
    assert 'exit 1\n}' in SCRIPT
    for variable in ("MODEL_DIR", "APALACHE_TESTS_DIR", "APALACHE_DIR", "RESULTS_ROOT"):
        assert re.search(r'^' + variable + r'=\$\(CDPATH= cd -- .*&& pwd -P\)$', SCRIPT, re.M)


def test_complete_csv_flows_through_pinned_generator_parser_and_consumer():
    assert "BENCHMARK=001indinv-apalache\n" in SCRIPT
    assert "RUN_SCRIPT=./run-all.sh\n" in SCRIPT
    calls = [shlex.split(line.strip()) for line in SCRIPT.splitlines() if line.strip().startswith("python3 ")]
    assert calls == [
        ["python3", "$MODEL_DIR/experiment.py", "validate-benchmarks", "$MODEL_DIR/$BENCHMARK.csv"],
        ["python3", "$SCRIPTS_DIR/mk-run.py", "$MODEL_DIR/$BENCHMARK.csv", "$APALACHE_DIR", "$MODEL_DIR", "$RUN_DIR"],
        ["python3", "$SCRIPTS_DIR/parse-logs.py", "$RUN_DIR", "--output", "$RESULT_CSV"],
        ["python3", "$MODEL_DIR/experiment.py", "validate-results", "$MODEL_DIR/$BENCHMARK.csv", "$RESULT_CSV"],
    ]
    assert '(cd "$RUN_DIR" && bash "$RUN_SCRIPT")' in SCRIPT
    assert SCRIPT.index('"$SCRIPTS_DIR/mk-run.py"') < SCRIPT.index('(cd "$RUN_DIR"')
    assert SCRIPT.index('(cd "$RUN_DIR"') < SCRIPT.index('"$SCRIPTS_DIR/parse-logs.py"')
    assert SCRIPT.index('"$SCRIPTS_DIR/parse-logs.py"') < SCRIPT.index(' validate-results ')
    rows = EXPERIMENT.validate_benchmarks(BENCHMARK)
    assert len(rows) == 12
    assert {row["filename"] for row in rows} == {"MC_n4_f1.tla", "MC_n4_f2.tla", "MC_n5_f1.tla", "MC_n5_f2.tla"}
    assert all((MODEL / row["filename"]).is_file() for row in rows)
    assert EXPERIMENT.main(["validate-benchmarks", str(BENCHMARK)]) is None
    assert EXPERIMENT.main(["validate-results", str(BENCHMARK), str(HISTORICAL)]) is None
    assert len(EXPERIMENT.validate_results(BENCHMARK, HISTORICAL)) == 12


@pytest.mark.parametrize("damage", ["missing", "duplicate", "extra", "wrong_id", "wrong_tool", "Fail", "Timeout", "Error", "unknown", "missing_status"])
def test_real_consumer_rejects_incomplete_or_unsuccessful_results(tmp_path, damage):
    fields, rows = result_rows()
    if damage == "missing":
        rows.pop(0)
    elif damage == "duplicate":
        rows[-1] = dict(rows[0])
    elif damage == "extra":
        rows.append(dict(rows[0]))
    elif damage == "wrong_id":
        rows[0]["01:no"] = "13"
    elif damage == "wrong_tool":
        rows[0]["02:tool"] = "tlc"
    elif damage == "missing_status":
        fields.remove("03:status")
        for row in rows:
            del row["03:status"]
    else:
        rows[0]["03:status"] = damage
    path = tmp_path / "results.csv"
    write_rows(path, fields, rows)
    with pytest.raises(ValueError):
        EXPERIMENT.main(["validate-results", str(BENCHMARK), str(path)])


@pytest.mark.parametrize("field", ["no", "filename", "tool", "timeout", "init", "inv", "next", "args", "missing_row"])
def test_real_consumer_rejects_benchmark_drift(tmp_path, field):
    with BENCHMARK.open(encoding="utf-8", newline="") as stream:
        reader = csv.DictReader(stream)
        fields, rows = reader.fieldnames, list(reader)
    if field == "missing_row":
        rows.pop()
    else:
        rows[0][field] = "changed"
    path = tmp_path / "benchmark.csv"
    write_rows(path, fields, rows)
    with pytest.raises(ValueError):
        EXPERIMENT.main(["validate-benchmarks", str(path)])
    with pytest.raises(ValueError):
        EXPERIMENT.main(["validate-results", str(path), str(HISTORICAL)])


def test_revision_tagged_private_evidence_and_failure_propagation():
    assert "umask 077\n" in SCRIPT
    assert 'REPOSITORY_REVISION=$(git -C "$MODEL_DIR" rev-parse HEAD)' in SCRIPT
    assert '$BENCHMARK-repo-$REPOSITORY_REVISION-harness-$HARNESS_REVISION-apalache-$APALACHE_REVISION.XXXXXX' in SCRIPT
    assert "RUN_DIR=$ARCHIVE/runs\n" in SCRIPT
    assert "RESULT_CSV=$ARCHIVE/$BENCHMARK-$APALACHE_REVISION.csv\n" in SCRIPT
    for key, value in (
        ("repository_revision", "REPOSITORY_REVISION"),
        ("harness_revision", "HARNESS_REVISION"),
        ("apalache_revision", "APALACHE_REVISION"),
        ("harness_directory", "APALACHE_TESTS_DIR"),
        ("apalache_directory", "APALACHE_DIR"),
        ("model_directory", "MODEL_DIR"),
        ("run_script", "RUN_SCRIPT"),
    ):
        assert f'"{key}=${value}"' in SCRIPT
    assert '> "$ARCHIVE/revisions.txt"' in SCRIPT
    assert 'cp "$MODEL_DIR/$BENCHMARK.csv" "$ARCHIVE/$BENCHMARK.csv"' in SCRIPT
    assert 'set +e\n(\n    set -ex\n' in SCRIPT
    assert ') > "$ARCHIVE/runner.log" 2>&1\nstatus=$?\nset -e\n' in SCRIPT
    assert "printf 'exit=%s\\n' \"$status\" >> \"$ARCHIVE/revisions.txt\"" in SCRIPT
    assert SCRIPT.endswith('exit "$status"\n')
    assert "rm " not in SCRIPT
    for path, digest in (
        (BENCHMARK, "fb1bb84f0d30ec3c3f03507a48c4d635e7a91e05f9b369289e5640034f3808de"),
        (HISTORICAL, "6a9e7a012756fa8902bb9a39fa74a7d0bc04f91a4f31ca1e231be135dd223ccb"),
        (MODEL / "results/001indinv-apalache-report.md", "afe99894ae7e661192a0b55c0d139cbdb3be38d46448f80c40755eae792e560a"),
    ):
        assert hashlib.sha256(path.read_bytes()).hexdigest() == digest


def test_synopsis_documents_the_runner_contract_and_proof_limits():
    synopsis = (MODEL / "Synopsis.md").read_text(encoding="utf-8")
    for fact in (
        PIN, "APALACHE_TESTS_DIR", "APALACHE_DIR", "APALACHE_REVISION", "RESULTS_ROOT",
        "full lowercase 40-character commit", "mk-run.py", "parse-logs.py", "--iniFile",
        "Python 3 (standard library)", "GNU `/usr/bin/time`", "GNU",
        "`/usr/bin/timeout`", "all twelve rows", "exactly one `NoError`", "revisions.txt",
        "runner.log", "all per-case raw logs", "return nonzero", "historical files",
        "No model checker was", "cannot prove", "ignored build artifacts",
        "No known compatible", "not reproduce", "must be explicitly ignored",
    ):
        assert fact in synopsis, fact
    assert "not a portable reproduction command" not in synopsis
    assert "classic Tendermint" in synopsis


def test_repair_record_maps_finding_tests_and_declares_evidence_boundary():
    path = Path(__file__).with_name("repair_22_33_1117.kvx")
    record = tomllib.loads(path.read_text(encoding="utf-8"))
    finding = record["finding"]["accountability-local-harness"]
    assert finding["audit"] == "22.30.47"
    assert finding["file"] == "consensus/spec/light-client/accountability/run.sh"
    assert finding["class"] == "config-drift"
    assert finding["severity"] == "medium"
    declared = {node.name for node in ast.parse(Path(__file__).read_text()).body if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")}
    assert set(finding["tests"]) == declared
    repair = record["repair"]
    assert repair["task"] == "22.33.1117"
    assert repair["requirement"] == "22998.1"
    assert repair["harness_revision"] == PIN
    assert repair["wiring_requests"] == []
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in repair["evidence"]
    assert "No baseline verification was run" in repair["baseline"]
    assert "not executed" in repair["review_only"]
    assert repair["build_cmd"] == "bash -n consensus/spec/light-client/accountability/run.sh"
    assert repair["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1117.py"
