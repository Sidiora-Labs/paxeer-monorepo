"""Offline source qualification of the production Ivy proof harness.

Bash is used only for the required syntax parse. Ivy and its legacy toolchain
are not executed; producer, logging and result-consumer wiring is asserted in
the production source, without substituting a checker or fabricating proof logs.
"""

import ast
from pathlib import Path
import re
import shlex
import subprocess
import tomllib


ROOT = Path(__file__).resolve().parents[3]
PROOF_DIR = ROOT / "consensus/spec/ivy-proofs"
SCRIPT = PROOF_DIR / "check_proofs.sh"
RECORD = Path(__file__).with_name("repair_22_33_1116.kvx")


def script_parts():
    source = SCRIPT.read_text(encoding="utf-8")
    match = re.search(r"^check_proof\(\) \{\n(.*?)^\}\n", source, re.M | re.S)
    assert match is not None, "the shared production proof runner must exist"
    assert source.count("check_proof()") == 1
    return source[: match.start()], match[1], source[match.end() :]


def statements(text):
    return [
        line.strip()
        for line in text.splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    ]


def test_ivy_harness_bash_syntax():
    result = subprocess.run(
        ["bash", "-n", str(SCRIPT)], capture_output=True, text=True, timeout=10
    )
    assert result.returncode == 0, result.stderr


def test_ivy_all_three_production_invocations_and_seed_are_preserved():
    setup, body, final = script_parts()
    assert 'cmd=(ivy_check "seed=$RANDOM")' in statements(setup)
    calls = [shlex.split(line) for line in statements(final) if line.startswith("check_proof ")]
    assert calls == [
        ["check_proof", "classic safety", "classic_safety.ivy"],
        ["check_proof", "accountable safety 1", "accountable_safety_1.ivy"],
        ["check_proof", "accountable safety 2", "accountable_safety_2.ivy", "complete=fo"],
    ]
    assert statements(body)[:3] == [
        "local label=$1 proof=$2",
        "shift 2",
        'local log_file="$log_dir/${proof%.ivy}.txt"',
    ]
    assert '"${cmd[@]}" "$@" "$proof" 2>&1 | tee -- "$log_file"' in statements(body)
    assert SCRIPT.read_text().count("ivy_check") == 2  # command and final log notice
    for call in calls:
        assert (PROOF_DIR / call[2]).is_file()


def test_ivy_checker_and_logger_statuses_are_captured_before_any_command():
    _, body, _ = script_parts()
    lines = statements(body)
    pipeline = '"${cmd[@]}" "$@" "$proof" 2>&1 | tee -- "$log_file"'
    index = lines.index(pipeline)
    assert lines[index + 1] == 'pipeline_status=("${PIPESTATUS[@]}")'
    assert "local -a pipeline_status" in lines[:index]
    assert body.count("PIPESTATUS") == 1
    assert body.count("pipeline_status=") == 1
    assert "tail" not in pipeline
    assert "2>&1 | tee" in pipeline


def test_ivy_success_requires_both_statuses_read_success_and_exact_final_ok():
    _, body, _ = script_parts()
    condition = re.search(r"^  if (.*?); then\n", body, re.M | re.S)
    assert condition is not None
    predicates = [predicate.strip() for predicate in condition[1].split("&&")]
    assert predicates == [
        '[ "${pipeline_status[0]}" -eq 0 ]',
        '[ "${pipeline_status[1]}" -eq 0 ]',
        'res=$(tail -n 1 -- "$log_file")',
        '[ "$res" = "OK" ]',
    ]
    assert statements(body[condition.end() :]) == [
        'echo "OK"', "else", 'echo "FAILED"', "success=1", "fi"
    ]
    assert body.count("res=") == 1
    assert "|| true" not in SCRIPT.read_text()


def test_ivy_failure_is_sticky_and_does_not_skip_later_proofs():
    setup, body, final = script_parts()
    source = SCRIPT.read_text()
    assert statements(setup)[0] == "success=0"
    assert re.findall(r"^\s*success=(.*)$", source, re.M) == ["0", "1"]
    assert "success=1" in statements(body)
    assert "exit" not in body and "return" not in body
    assert not re.search(r"\bset\s+-\w*e", source)
    assert statements(final) == [
        'check_proof "classic safety" classic_safety.ivy',
        'check_proof "accountable safety 1" accountable_safety_1.ivy',
        'check_proof "accountable safety 2" accountable_safety_2.ivy complete=fo',
        "echo",
        'echo "See ivy_check output in $log_dir"',
        'exit "$success"',
    ]


def test_ivy_complete_logs_have_private_unique_paths_and_guarded_setup():
    setup, body, final = script_parts()
    assert statements(setup) == [
        "success=0",
        "umask 077 || exit 1",
        "mkdir -p output || exit 1",
        "if ! log_dir=$(mktemp -d output/proofs-XXXXXX); then",
        "exit 1",
        "fi",
        'cmd=(ivy_check "seed=$RANDOM")',
    ]
    assert 'local log_file="$log_dir/${proof%.ivy}.txt"' in statements(body)
    assert 'tee -- "$log_file"' in body
    assert 'res=$(tail -n 1 -- "$log_file")' in body
    assert "rm " not in SCRIPT.read_text() and "trap " not in SCRIPT.read_text()
    assert 'echo "See ivy_check output in $log_dir"' in statements(final)
    readme = (PROOF_DIR / "README.md").read_text(encoding="utf-8")
    assert "preserves each checker and `tee` exit status" in readme
    assert "captures stdout and\nstderr in a unique private directory" in readme
    assert "All three proofs run" in readme
    assert "This documentation does not repair the harness" not in readme


def test_ivy_repair_record_maps_finding_tests_and_evidence_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["1116"]
    finding = record["finding"]["ivy-exit-status-lost"]
    audit = tomllib.loads(
        (ROOT / "spec/layerx-merge/programme/audit/records/22.30.47.kvx").read_text()
    )["finding"]["ivy-exit-status-lost"]
    assert finding["audit"] == "22.30.47"
    for key in ("file", "line", "class", "severity"):
        assert finding[key] == audit[key]
    declared = {
        node.name
        for node in ast.parse(Path(__file__).read_text()).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    assert len(finding["tests"]) == len(declared) == 7
    assert task["base_revision"] == "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    assert task["build_cmd"] == "bash -n consensus/spec/ivy-proofs/check_proofs.sh"
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1116.py"
    )
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert task["wiring_requests"] == []
