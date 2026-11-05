import shlex
import subprocess
import sys
from pathlib import Path

import pytest

from programme.evidence import evidence_line
from programme.kvx import KvxValue, load
from programme.rehearsal import run_steps, to_finding


@pytest.mark.parametrize("stop_on_failure", [True, False])
@pytest.mark.parametrize("failed_step", [1, 2])
def test_failed_step_yields_finding_block(
    tmp_path: Path, stop_on_failure: bool, failed_step: int
) -> None:
    root = tmp_path / "repository with spaces"
    root.mkdir()
    subprocess.run(["git", "init", "-q", str(root)], check=True)
    subprocess.run(
        [
            "git",
            "-c",
            "user.name=Rehearsal test",
            "-c",
            "user.email=rehearsal@example.invalid",
            "commit",
            "--allow-empty",
            "-qm",
            "Initialize rehearsal repository",
        ],
        cwd=root,
        check=True,
    )
    revision = subprocess.run(
        ["git", "rev-parse", "HEAD"],
        cwd=root,
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()
    steps: list[dict[str, KvxValue]] = []
    for number in range(1, 4):
        exit_code = 23 if number == failed_step else 0
        steps.append(
            {
                "command": shlex.join(
                    [
                        sys.executable,
                        "-c",
                        "import sys; print(sys.argv[1]); "
                        f"sys.exit({exit_code})",
                        r'literal "quotes" # backslash\path command= exit=0 log=',
                    ]
                ),
                "stop_on_failure": stop_on_failure,
            }
        )
    log_dir = root / 'private "logs" # records\\archive'
    result = run_steps(steps, root, log_dir)
    assert result.passed is False
    assert result.failed_step == failed_step
    assert len(result.evidence) == (failed_step if stop_on_failure else len(steps))
    failed_evidence = evidence_line(
        revision,
        steps[failed_step - 1]["command"],
        23,
        log_dir / f"step-{failed_step}.log",
    )
    assert result.evidence[failed_step - 1] == failed_evidence
    block = to_finding(result, "17.3.3")
    assert block.splitlines()[0] == "[finding.17.3.3]"
    assert len(block.splitlines()) == 7
    ledger = tmp_path / "qualification.kvx"
    ledger.write_text(block, encoding="utf-8")
    sections = load(ledger)
    assert list(sections) == ["finding.17.3.3"]
    finding = sections["finding.17.3.3"]
    assert list(finding) == [
        "task",
        "file",
        "symbol",
        "observed",
        "assumption",
        "severity",
    ]
    assert finding == {
        "task": "17.3.3",
        "file": "spec/layerx-merge/programme/rehearsal.py",
        "symbol": "run_steps",
        "observed": f"Rehearsal step {failed_step} failed: {failed_evidence}.",
        "assumption": (
            "The failed rehearsal step is outside this task's scope and remains unresolved."
        ),
        "severity": "blocker",
    }
    assert (log_dir / f"step-{failed_step}.log").read_text(encoding="utf-8") == (
        'literal "quotes" # backslash\\path command= exit=0 log=\n'
    )

    passed = run_steps(
        [
            {
                "command": shlex.join([sys.executable, "-c", "print('passed')"]),
                "stop_on_failure": True,
            }
        ],
        root,
        root / "passed logs",
    )
    assert passed.passed is True
    assert passed.failed_step is None
    assert to_finding(passed, "17.3.3") == ""
    assert to_finding(run_steps([], root, root / "empty logs"), "17.3.3") == ""
