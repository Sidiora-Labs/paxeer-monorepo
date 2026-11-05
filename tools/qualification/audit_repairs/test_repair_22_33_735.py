"""Finding 019: sleeping refusal and confirmed readiness through the real CLI."""

import ast
import importlib.util
import json
import re
import shlex
import tomllib
from pathlib import Path

import pytest


ROOT = Path(__file__).resolve().parents[3]
TESTS = ROOT / "tools/deploy/tests"
RECORD = Path(__file__).with_name("repair_22_33_735.kvx")


@pytest.fixture
def readiness():
    spec = importlib.util.spec_from_file_location(
        "repair_22_33_735_readiness", TESTS / "railway-perps-readiness.test.py",
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def test_finding_019_successful_wake_readiness_before_later_wave(readiness):
    readiness.check_successful_wake_readiness()


def test_finding_019_unavailable_sleeping_service_blocks_later_wave(readiness):
    readiness.check_sleeping_unavailable_refusal()


def test_finding_019_health_without_deployment_confirmation_still_blocks(readiness):
    readiness.check_health_without_deployment_confirmation_refused()


def test_finding_019_shell_registers_guarded_readiness_cases():
    shell = (TESTS / "railway-perps-project.test.sh").read_text()
    assert shell.splitlines()[1] == "set -euo pipefail"
    command = 'python3 "$here/railway-perps-readiness.test.py"'
    assert shell.splitlines().count(command) == 1
    assert shlex.split(command) == ["python3", "$here/railway-perps-readiness.test.py"]
    assert shell.index(command) < shell.index('tmp="$(mktemp -d)"')
    assert 'echo "wave 5 explorer SUCCESS"' in shell
    assert 'echo "wave 5 explorer SLEEPING"' not in shell
    assert "grep -q '^wave 5 explorer SLEEPING$'" not in shell
    assert ('grep -q \'^wave 5 explorer SUCCESS$\' "$tmp/out" || '
            'fail "ready explorer did not pass wave 5"') in shell
    programs = re.findall(r"<<'PY2'\n(.*?)\nPY2", shell, re.S)
    assert len(programs) == 3
    latest = []
    for program in programs:
        tree = ast.parse(program)
        for node in ast.walk(tree):
            if isinstance(node, ast.Assign) and any(
                isinstance(target, ast.Name) and target.id == "latest" for target in node.targets
            ):
                latest.append(ast.literal_eval(node.value))
    assert len(latest) == 1
    assert latest[0]["explorer"] == "SUCCESS"
    assert latest[0]["indexer"] == "CRASHED"


def test_finding_019_record_maps_proof_and_qualification_limits():
    record = tomllib.loads(RECORD.read_text())
    finding = record["finding"]["019"]
    audit = (ROOT / "spec/layerx-merge/programme/audit/records/22.30.171.kvx").read_text()
    section = re.search(r"(?ms)^\[finding\.019\]\n(.*?)(?=^\[|\Z)", audit)
    assert section
    original = tomllib.loads(section.group(1))
    for key in ("file", "class", "severity", "observed", "expected", "proposal"):
        assert finding[key] == original[key]
    assert finding["tests"] == [
        "test_finding_019_successful_wake_readiness_before_later_wave",
        "test_finding_019_unavailable_sleeping_service_blocks_later_wave",
        "test_finding_019_health_without_deployment_confirmation_still_blocks",
        "test_finding_019_shell_registers_guarded_readiness_cases",
    ]
    tree = ast.parse(Path(__file__).read_text())
    declared = {node.name for node in tree.body if isinstance(node, ast.FunctionDef)}
    assert set(finding["tests"]) <= declared
    fixture = json.loads((TESTS / "fixtures/railway-perps-readiness.json").read_text())
    assert fixture["scenarios"] == {
        "wake": {"health_statuses": [503, 200], "confirm_deployment": True},
        "unavailable": {"health_statuses": [503, 503], "confirm_deployment": True},
        "unconfirmed": {"health_statuses": [200], "confirm_deployment": False},
    }
    task = record["task"]["22"]["33"]["735"]
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_735.py"
    assert task["build_cmd"] == "bash -n tools/deploy/tests/*-perps-project.test.sh"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert task["wiring_requests"] == []
    assert "external wake" in task["runtime_proof"]
    assert "does not issue a health request" in task["review_only"]
    assert "No baseline failures are claimed" in task["baseline"]
