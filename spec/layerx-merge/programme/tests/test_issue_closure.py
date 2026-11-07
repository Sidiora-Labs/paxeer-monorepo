import ast
import runpy
import subprocess
import sys
from pathlib import Path


CHECKER = Path(__file__).resolve().parents[1] / "issue_closure.py"
issue_closure = runpy.run_path(str(CHECKER))["issue_closure"]


def test_issue_closure_passes_when_every_issue_task_is_done(tmp_path: Path) -> None:
    spec_path = tmp_path / "spec.kvx"
    spec_path.write_text(
        '[task.18.0.1]\nstatus = "in_progress"\n'
        '[task.18.7.3] # completed issue\nstatus = "done" # qualified\n'
        '[task.18.1.1]\nstatus = "done"\n'
        '[task.19.1.1]\nstatus = "pending"\n'
        '[task.180.1.1]\nstatus = "pending"\n'
        '[req.18001.1]\nstatus = "pending"\n',
        encoding="utf-8",
    )

    assert issue_closure(spec_path) == []
    result = subprocess.run(
        [sys.executable, str(CHECKER), str(spec_path)],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    assert ast.literal_eval(result.stdout) == []


def test_issue_closure_names_the_open_task(tmp_path: Path) -> None:
    spec_path = tmp_path / "spec.kvx"
    spec_path.write_text(
        '[task.18.0.1]\nstatus = "in_progress"\n'
        '[task.18.1.2]\nstatus = "done"\n'
        '[task.18.1.1]\nstatus = "pending"\n'
        '[task.19.1.1]\nstatus = "pending"\n',
        encoding="utf-8",
    )

    assert issue_closure(spec_path) == ["18.1.1"]
    result = subprocess.run(
        [sys.executable, str(CHECKER), str(spec_path)],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 1, result.stderr
    assert ast.literal_eval(result.stdout) == ["18.1.1"]

    with spec_path.open("a", encoding="utf-8") as spec:
        spec.write(
            '[task.18.7.3]\nstatus = "blocked"\n'
            '[task.18.2.1]\nstatus = "in_progress"\n'
            '[task.18.3.1]\nstatus = "superseded"\n'
            '[task.18.4.1]\ntitle = "No status declared"\n'
            '[task.18.1.3]\nstatus = "pending"\n'
        )
    expected = ["18.1.1", "18.1.3", "18.2.1", "18.4.1", "18.7.3"]
    assert issue_closure(spec_path) == expected
    result = subprocess.run(
        [sys.executable, str(CHECKER), str(spec_path)],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 1, result.stderr
    assert ast.literal_eval(result.stdout) == expected


def test_issue_closure_treats_a_superseded_task_as_closed(tmp_path: Path) -> None:
    spec_path = tmp_path / "spec.kvx"
    spec_path.write_text(
        '[task.18.1.1]\nstatus = "done"\n'
        '[task.18.1.2]\nstatus = "superseded"\n',
        encoding="utf-8",
    )

    assert issue_closure(spec_path) == []
    result = subprocess.run(
        [sys.executable, str(CHECKER), str(spec_path)],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    assert ast.literal_eval(result.stdout) == []
