import ast
import runpy
import subprocess
import sys
from pathlib import Path


CHECKER = Path(__file__).resolve().parents[1] / "names_closure.py"
names_closure = runpy.run_path(str(CHECKER))["names_closure"]


def test_names_closure_passes_when_every_name_task_is_done(tmp_path: Path) -> None:
    spec_path = tmp_path / "spec.kvx"
    spec_path.write_text(
        '[task.19.0.1]\nstatus = "in_progress"\n'
        '[task.19.7.3]\nstatus = "done"\n'
        '[task.19.1.1]\nstatus = "done"\n'
        '[task.18.1.1]\nstatus = "pending"\n'
        '[task.190.1.1]\nstatus = "pending"\n'
        '[req.19009]\nstatus = "pending"\n',
        encoding="utf-8",
    )

    assert names_closure(spec_path) == []
    result = subprocess.run(
        [sys.executable, str(CHECKER), str(spec_path)],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    assert ast.literal_eval(result.stdout) == []


def test_names_closure_names_the_open_task(tmp_path: Path) -> None:
    spec_path = tmp_path / "spec.kvx"
    spec_path.write_text(
        '[task.19.0.1]\nstatus = "in_progress"\n'
        '[task.19.1.2]\nstatus = "done"\n'
        '[task.19.1.1]\nstatus = "pending"\n'
        '[task.18.1.1]\nstatus = "pending"\n',
        encoding="utf-8",
    )

    assert names_closure(spec_path) == ["19.1.1"]
    result = subprocess.run(
        [sys.executable, str(CHECKER), str(spec_path)],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 1, result.stderr
    assert ast.literal_eval(result.stdout) == ["19.1.1"]

    with spec_path.open("a", encoding="utf-8") as spec:
        spec.write(
            '[task.19.7.3]\nstatus = "blocked"\n'
            '[task.19.2.1]\nstatus = "in_progress"\n'
            '[task.19.3.1]\nstatus = "superseded"\n'
            '[task.19.4.1]\ntitle = "No status declared"\n'
            '[task.19.1.3]\nstatus = "pending"\n'
        )
    expected = ["19.1.1", "19.1.3", "19.2.1", "19.3.1", "19.4.1", "19.7.3"]
    assert names_closure(spec_path) == expected
    result = subprocess.run(
        [sys.executable, str(CHECKER), str(spec_path)],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 1, result.stderr
    assert ast.literal_eval(result.stdout) == expected
