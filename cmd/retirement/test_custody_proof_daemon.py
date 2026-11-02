import subprocess
from pathlib import Path

import pytest


ROOT = Path(__file__).resolve().parents[2]
RETIRED_PATHS = (
    "daemon/layerx-custody-proof/main.go",
    "daemon/layerx-custody-proof/light.go",
)
RETIRED_RECIPE_NAMES = (
    *RETIRED_PATHS,
    "daemon/layerx-custody-proof",
    "layerx-custody-proof",
)


def assert_custody_proof_daemon_absent_from_recipe(output: str):
    for name in RETIRED_RECIPE_NAMES:
        assert name not in output, f"Build recipe still names {name}:\n{output}"


def test_custody_proof_daemon_leaves_the_build():
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
    assert_custody_proof_daemon_absent_from_recipe(result.stdout)


@pytest.mark.parametrize(
    "recipe",
    (
        "go build ./daemon/layerx-custody-proof/main.go",
        "go build ./daemon/layerx-custody-proof/light.go",
        "go build ./daemon/layerx-custody-proof",
        "go install daemon/layerx-custody-proof",
        "go build -o layerx-custody-proof ./cmd/proof",
    ),
)
def test_custody_proof_daemon_rejects_retired_build_recipes(recipe):
    with pytest.raises(AssertionError, match="Build recipe still names"):
        assert_custody_proof_daemon_absent_from_recipe(recipe)
