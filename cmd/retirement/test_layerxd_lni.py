import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
RETIRED_PATHS = (
    "cmd/layerxd/lxp_daemon_lni.c",
    "cmd/layerxd/lxp_daemon_lni_account.h",
    "cmd/layerxd/lxp_daemon_lni_caps.h",
    "cmd/layerxd/lxp_daemon_lni_head_attestation.h",
    "cmd/layerxd/lxp_daemon_lni_internal.h",
    "cmd/layerxd/lxp_daemon_lni_module.h",
    "cmd/layerxd/lni.env.example",
)


def test_layerxd_lni_leaves_the_build():
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
