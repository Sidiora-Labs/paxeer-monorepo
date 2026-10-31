import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
RETIRED_PATHS = (
    "cmd/layerxd/lxp_daemon_artifact.c",
    "cmd/layerxd/lxp_daemon_artifact.h",
    "cmd/layerxd/lxp_daemon_allowance.c",
    "cmd/layerxd/lxp_daemon_allowance.h",
    "cmd/layerxd/lxp_daemon_deployment.h",
    "cmd/layerxd/lxp_daemon_deployment.inc",
    "cmd/layerxd/lxp_daemon_maintenance_json.h",
    "cmd/layerxd/lxp_daemon_modules.h",
)


def test_layerxd_artifacts_leaves_the_build():
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
