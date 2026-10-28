import shlex
import subprocess
import sys
from pathlib import Path

import pytest

from programme.evidence import evidence_line
from programme.kvx import KvxValue
from programme.rehearsal import run_steps


def _repository(root: Path) -> str:
    root.mkdir()
    subprocess.run(["git", "init", "-q", str(root)], check=True)
    subprocess.run(
        [
            "git",
            "-c",
            "user.name=Rehearsal tests",
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
    return subprocess.run(
        ["git", "rev-parse", "HEAD"],
        cwd=root,
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()


def _steps(exit_codes: list[int], stop_on_failure: bool) -> list[dict[str, KvxValue]]:
    steps: list[dict[str, KvxValue]] = []
    for number, exit_code in enumerate(exit_codes, start=1):
        previous = "".join(f"{index}\n" for index in range(1, number))
        marker = f"{number}\n"
        script = (
            "import sys; from pathlib import Path; "
            "trace = Path('order.txt'); "
            "previous = trace.read_text() if trace.exists() else ''; "
            f"assert previous == {previous!r}; "
            f"trace.write_text(previous + {marker!r}); "
            f"print('stdout step {number}', flush=True); "
            f"print('stderr step {number}', file=sys.stderr, flush=True); "
            "print(sys.argv[1], flush=True); "
            f"sys.exit({exit_code})"
        )
        steps.append(
            {
                "command": shlex.join(
                    [sys.executable, "-c", script, "literal ; $(untouched) # argument"]
                ),
                "stop_on_failure": stop_on_failure,
            }
        )
    return steps


@pytest.mark.parametrize("stop_on_failure", [True, False])
@pytest.mark.parametrize("failed_step", [1, 2])
def test_runner_stops_at_first_failure(
    tmp_path: Path,
    capsys: pytest.CaptureFixture[str],
    stop_on_failure: bool,
    failed_step: int,
) -> None:
    root = tmp_path / "repository with spaces"
    revision = _repository(root)
    exits = [0, 0, 0]
    exits[failed_step - 1] = 1
    steps = _steps(exits, stop_on_failure)
    log_dir = root / "private logs"

    result = run_steps(steps, root, log_dir)

    executed = failed_step if stop_on_failure else len(steps)
    assert result.passed is False
    assert result.failed_step == failed_step
    assert result.exit_code == 1
    assert (root / "order.txt").read_text() == "".join(
        f"{number}\n" for number in range(1, executed + 1)
    )
    assert len(result.evidence) == executed
    assert (log_dir / "evidence.log").read_text().splitlines() == result.evidence
    assert capsys.readouterr().out.splitlines() == result.evidence
    for number, step in enumerate(steps[:executed], start=1):
        log_path = log_dir / f"step-{number}.log"
        assert result.evidence[number - 1] == evidence_line(
            revision, step["command"], exits[number - 1], log_path
        )
        assert log_path.read_text() == (
            f"stdout step {number}\nstderr step {number}\n"
            "literal ; $(untouched) # argument\n"
        )
    for number in range(executed + 1, len(steps) + 1):
        assert not (log_dir / f"step-{number}.log").exists()


def test_runner_passes_when_all_steps_pass(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    root = tmp_path / "repository with spaces"
    revision = _repository(root)
    steps = _steps([0, 0], stop_on_failure=True)
    log_dir = root / "private logs"

    result = run_steps(steps, root, "private logs")

    assert result.passed is True
    assert result.failed_step is None
    assert result.exit_code == 0
    assert (root / "order.txt").read_text() == "1\n2\n"
    assert len(result.evidence) == 2
    assert (log_dir / "evidence.log").read_text().splitlines() == result.evidence
    assert capsys.readouterr().out.splitlines() == result.evidence
    for number, step in enumerate(steps, start=1):
        log_path = log_dir / f"step-{number}.log"
        assert result.evidence[number - 1] == evidence_line(
            revision, step["command"], 0, log_path
        )
        assert log_path.read_text() == (
            f"stdout step {number}\nstderr step {number}\n"
            "literal ; $(untouched) # argument\n"
        )
