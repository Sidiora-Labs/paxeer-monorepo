from pathlib import Path
import subprocess
import unittest


class KernelImageTest(unittest.TestCase):
    def test_kernel_dockerfile_passes_build_check(self):
        root = Path(__file__).resolve().parents[2]
        result = subprocess.run(
            ["bash", str(root / "tools/containers/check.sh"), "kernel"],
            cwd=root,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(
            result.returncode,
            0,
            msg=result.stdout + result.stderr,
        )
