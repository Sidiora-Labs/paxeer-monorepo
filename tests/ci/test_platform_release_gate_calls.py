import re
import shlex
from pathlib import Path

import pytest
import yaml

from tools.ci.tests.workflow_graph import release_gate_job


ROOT = Path(__file__).resolve().parents[2]
RELEASE_GATES = (
    ("programs-acceptance", "programs-test"),
    ("agent-sanitizers", "agent-test-sanitize"),
    ("agent-fuzz-corpus", "agent-fuzz-long"),
)


def _workflow():
    return yaml.safe_load(
        (ROOT / ".github/workflows/platform.yml").read_text(encoding="utf-8")
    )


def test_platform_workflow_does_not_pass_ordinal_to_release_gate():
    workflow = _workflow()
    for job in workflow["jobs"].values():
        for step in job.get("steps", []):
            assert "--ordinal" not in step.get("run", "")
    for job_name, _ in RELEASE_GATES:
        _, job = release_gate_job(ROOT, job_name)
        for step in job["steps"]:
            assert "--ordinal" not in step.get("run", "")
            assert "release-gate.sh" not in step.get("run", "")


@pytest.mark.parametrize("job_name,target", RELEASE_GATES)
def test_platform_release_gate_upload_paths_are_runner_temp(job_name, target):
    _, job = release_gate_job(ROOT, job_name)
    steps = job["steps"]
    uploads = [
        step
        for step in steps
        if step.get("uses", "").startswith("actions/upload-artifact@")
    ]
    assert len(uploads) == 1
    upload = uploads[0]
    assert upload["with"]["name"] == f"release-gate-{job_name}"
    assert upload["if"] == "${{ always() }}"
    assert upload["with"]["path"] == f"${{{{ runner.temp }}}}/{job_name}.log"
    assert upload["with"]["if-no-files-found"] == "error"
    producers = [step for step in steps if f"make {target}" in step.get("run", "")]
    assert len(producers) == 1
    producer = producers[0]
    assert steps.index(producer) < steps.index(upload)
    assert re.search(
        rf"make {re.escape(target)} 2>&1 \| tee (?:-a )?"
        rf'"\$RUNNER_TEMP/{re.escape(job_name)}\.log"',
        producer["run"],
    )
    assert producer["shell"] == "bash"
    assert producer["run"].splitlines()[0] == "set -euo pipefail"
    assert not producer.get("continue-on-error", False)
    assert "if" not in producer


@pytest.mark.parametrize("job_name,target", RELEASE_GATES)
def test_platform_release_gate_jobs_run_existing_make_targets(job_name, target):
    _, job = release_gate_job(ROOT, job_name)
    steps = job["steps"]
    commands = [step["run"] for step in steps if "run" in step]
    invocations = [
        shlex.split(line)
        for command in commands
        for line in command.replace("\\\n", " ").splitlines()
        if re.search(rf"\bmake {re.escape(target)}\b", line)
    ]
    assert len(invocations) == 1
    tokens = invocations[0]
    make_index = tokens.index("make")
    assert tokens[make_index + 1] == target
    assert tokens[0] == "make" or tokens[:2] == ["sudo", "env"]
    assert tokens[make_index + 2 : make_index + 5] == ["2>&1", "|", "tee"]

    makefile = (ROOT / "Makefile").read_text(encoding="utf-8")
    rules = [
        line.partition(":")[0].split()
        for line in makefile.splitlines()
        if line
        and not line[0].isspace()
        and ":" in line
        and not re.match(r"\s*\w+\s*[:+?]?=", line.partition(":")[2])
    ]
    assert any(target in names for names in rules), f"No Makefile rule for {target}"
