import ast
import runpy
import subprocess
import sys
from pathlib import Path


CHECKER = Path(__file__).resolve().parents[1] / "chains_closure.py"
chains_closure = runpy.run_path(str(CHECKER))["chains_closure"]


def test_chains_closure_passes_when_every_chain_task_is_done(tmp_path: Path) -> None:
    spec_path = tmp_path / "spec.kvx"
    spec_path.write_text(
        '[task.20.0.1]\nstatus = "in_progress"\n'
        '[task.20.7.3]\nstatus = "done"\n'
        '[task.20.1.1]\nstatus = "done"\n'
        '[task.19.1.1]\nstatus = "pending"\n'
        '[task.200.1.1]\nstatus = "pending"\n'
        '[req.20010]\nstatus = "pending"\n',
        encoding="utf-8",
    )

    assert chains_closure(spec_path) == []
    result = subprocess.run(
        [sys.executable, str(CHECKER), str(spec_path)],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    assert ast.literal_eval(result.stdout) == []


def test_chains_closure_names_the_open_task(tmp_path: Path) -> None:
    spec_path = tmp_path / "spec.kvx"
    spec_path.write_text(
        '[task.20.0.1]\nstatus = "in_progress"\n'
        '[task.20.1.2]\nstatus = "done"\n'
        '[task.20.1.1]\nstatus = "pending"\n'
        '[task.19.1.1]\nstatus = "pending"\n',
        encoding="utf-8",
    )

    assert chains_closure(spec_path) == ["20.1.1"]
    result = subprocess.run(
        [sys.executable, str(CHECKER), str(spec_path)],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 1, result.stderr
    assert ast.literal_eval(result.stdout) == ["20.1.1"]

    with spec_path.open("a", encoding="utf-8") as spec:
        spec.write(
            '[task.20.7.3]\nstatus = "blocked"\n'
            '[task.20.2.1]\nstatus = "in_progress"\n'
            '[task.20.3.1]\nstatus = "superseded"\n'
            '[task.20.4.1]\ntitle = "No status declared"\n'
            '[task.20.1.3]\nstatus = "pending"\n'
        )
    expected = ["20.1.1", "20.1.3", "20.2.1", "20.3.1", "20.4.1", "20.7.3"]
    assert chains_closure(spec_path) == expected
    result = subprocess.run(
        [sys.executable, str(CHECKER), str(spec_path)],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 1, result.stderr
    assert ast.literal_eval(result.stdout) == expected
