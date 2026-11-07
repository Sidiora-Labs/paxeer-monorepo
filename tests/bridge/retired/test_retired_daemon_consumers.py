import subprocess
from pathlib import Path


def test_bridge_has_no_retired_daemon_consumer():
    root = Path(__file__).resolve().parents[3]
    retired_files = (
        "test_credit_admission.c",
        "comet_credit.py",
        "paxeer_x_advanced_trust_credit.py",
        "paxeer_x_multiasset_credit.py",
        "owner-custody.py",
    )
    for name in retired_files:
        path = root / "tests" / "bridge" / name
        assert not (path.exists() or path.is_symlink()), f"Retired consumer: {path}"

    result = subprocess.run(
        ["make", "-n", "build", "test"],
        cwd=root,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        check=False,
    )
    assert result.returncode == 0, (
        f"make -n build test exited {result.returncode}:\n{result.stdout}"
    )
    for name in retired_files + (
        "layerx-custody-proof",
        "layerxd",
        "layerx-guarantor",
        "owner_custody",
    ):
        assert name not in result.stdout, (
            f"Retired consumer in make -n build test recipes: {name}\n{result.stdout}"
        )
