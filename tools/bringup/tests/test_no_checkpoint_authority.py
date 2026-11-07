import subprocess
from pathlib import Path


def test_bringup_names_no_checkpoint_authority():
    bringup = Path(__file__).resolve().parents[1]
    retired = tuple(f"{name}.py" for name in ("checkpoint-authority", "publication-policy"))
    sources = sorted(path for path in bringup.rglob("*") if path.suffix in {".sh", ".py"})
    assert bringup / "kernel-genesis.sh" in sources
    violations = [
        f"{path.relative_to(bringup)}:{number}: {name}"
        for path in sources
        for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1)
        for name in retired
        if name in line
    ]
    assert not violations, "Retired bringup dependencies:\n" + "\n".join(violations)

    result = subprocess.run(
        ["bash", "-n", str(bringup / "kernel-genesis.sh")],
        capture_output=True,
        text=True,
        timeout=30,
        check=False,
    )
    assert result.returncode == 0, result.stdout + result.stderr
