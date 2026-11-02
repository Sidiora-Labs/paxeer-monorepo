import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def test_movement_proof_and_nullifier_deploy_absent():
    retired_path = ROOT / "tests/daemon/movement_proof.py"
    assert not retired_path.exists(), "Retired movement proof script still exists"
    assert not retired_path.is_symlink(), "Retired movement proof script is a symlink"

    for path in sorted((ROOT / "tests/daemon").rglob("*")):
        if path.is_file():
            assert b"WithdrawalNullifierRegistry" not in path.read_bytes(), (
                f"Daemon file still names WithdrawalNullifierRegistry: "
                f"{path.relative_to(ROOT)}"
            )

    result = subprocess.run(
        ["make", "-n", "build", "test"],
        cwd=ROOT,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        timeout=120,
        check=False,
    )

    assert result.returncode == 0, result.stdout
    assert "movement_proof" not in result.stdout, (
        f"Build recipe still names movement_proof:\n{result.stdout}"
    )
