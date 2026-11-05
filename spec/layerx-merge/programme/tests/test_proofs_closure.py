import ast
import runpy
import subprocess
import sys
from pathlib import Path


CHECKER = Path(__file__).resolve().parents[1] / "proofs_closure.py"
assert CHECKER.is_file(), f"Missing closure checker: {CHECKER}"
proofs_closure = runpy.run_path(str(CHECKER))["proofs_closure"]


def test_proofs_closure_passes_when_every_proof_task_is_done(tmp_path: Path) -> None:
    spec_path = tmp_path / "spec.kvx"
    spec_path.write_text(
        '[task.21.0.1]\nstatus = "in_progress"\n'
        '[task.21.7.3]\nstatus = "done"\n'
        '[task.21.1.1]\nstatus = "done"\n'
        '[task.20.1.1]\nstatus = "pending"\n'
        '[task.210.1.1]\nstatus = "pending"\n'
        '[req.21500]\nstatus = "pending"\n',
        encoding="utf-8",
    )

    assert proofs_closure(spec_path) == []
    result = subprocess.run(
        [sys.executable, str(CHECKER), str(spec_path)],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stderr
    assert ast.literal_eval(result.stdout) == []


def test_proofs_closure_names_the_open_task(tmp_path: Path) -> None:
    spec_path = tmp_path / "spec.kvx"
    spec_path.write_text(
        '[task.21.0.1]\nstatus = "in_progress"\n'
        '[task.21.1.2]\nstatus = "done"\n'
        '[task.21.1.1]\nstatus = "pending"\n'
        '[task.20.1.1]\nstatus = "pending"\n',
        encoding="utf-8",
    )

    assert proofs_closure(spec_path) == ["21.1.1"]
    result = subprocess.run(
        [sys.executable, str(CHECKER), str(spec_path)],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 1, result.stderr
    assert ast.literal_eval(result.stdout) == ["21.1.1"]

    with spec_path.open("a", encoding="utf-8") as spec:
        spec.write(
            '[task.21.7.3]\nstatus = "blocked"\n'
            '[task.21.2.1]\nstatus = "in_progress"\n'
            '[task.21.3.1]\nstatus = "superseded"\n'
            '[task.21.4.1]\ntitle = "No status declared"\n'
            '[task.21.1.3]\nstatus = "pending"\n'
        )
    expected = ["21.1.1", "21.1.3", "21.2.1", "21.3.1", "21.4.1", "21.7.3"]
    assert proofs_closure(spec_path) == expected
    result = subprocess.run(
        [sys.executable, str(CHECKER), str(spec_path)],
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 1, result.stderr
    assert ast.literal_eval(result.stdout) == expected
