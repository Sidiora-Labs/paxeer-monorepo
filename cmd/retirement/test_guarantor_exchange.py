from pathlib import Path
import subprocess


ROOT = Path(__file__).resolve().parents[2]
RETIRED_PATHS = (
    "cmd/layerx-guarantor/exchange.c",
    "cmd/layerx-guarantor/exchange.h",
    "cmd/layerx-guarantor/lni.c",
    "cmd/layerx-guarantor/lni.h",
)


def test_guarantor_exchange_leaves_the_build():
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
    for relative_path in RETIRED_PATHS:
        path = ROOT / relative_path
        assert not path.exists(), f"Retired path still exists: {relative_path}"
        assert not path.is_symlink(), f"Retired path is a symlink: {relative_path}"
        assert relative_path not in result.stdout, (
            f"Build recipe still names {relative_path}:\n{result.stdout}"
        )
