import re
import subprocess
from pathlib import Path


def test_kernel_image_runs_no_oracle_feeder():
    root = Path(__file__).resolve().parents[3]
    paths = (
        "docker/kernel/init.sh",
        "docker/kernel/kernel.env.example",
        "docker/kernel/Dockerfile",
        "tools/bringup/check-live.sh",
    )
    for path in paths:
        source = (root / path).read_text(encoding="utf-8")
        assert "/usr/local/bin/layerx-oracle-feeder" not in source, path
        assert re.search(r"\bservice\s+oracle-feeder\b", source) is None, path
        assert "LAYERX_FEEDER_" not in source, path
        assert re.search(r"^\s*oracle-feeder(?:\s|$)", source, re.MULTILINE) is None, path

    for path in ("docker/kernel/init.sh", "tools/bringup/check-live.sh"):
        result = subprocess.run(
            ["bash", "-n", str(root / path)],
            cwd=root,
            capture_output=True,
            text=True,
            timeout=30,
            check=False,
        )
        assert result.returncode == 0, f"{path}: {result.stdout}{result.stderr}"
