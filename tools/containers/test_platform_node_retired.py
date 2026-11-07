import os
from pathlib import Path
import subprocess
import tempfile
import unittest


class PlatformNodeRetiredTest(unittest.TestCase):
    def test_check_refuses_retired_service_name(self):
        root = Path(__file__).resolve().parents[2]
        script = root / "tools/containers/check.sh"
        with tempfile.TemporaryDirectory(prefix="platform-node-retired-") as log_dir:
            env = dict(os.environ, CONTAINERS_CHECK_LOG_DIR=log_dir)
            retired = subprocess.run(
                ["bash", str(script), "platform-node"],
                cwd=root,
                env=env,
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(retired.returncode, 2, retired.stdout + retired.stderr)
            self.assertIn("unknown service platform-node", retired.stderr)

            kernel = subprocess.run(
                ["bash", str(script), "kernel"],
                cwd=root,
                env=env,
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(kernel.returncode, 0, kernel.stdout + kernel.stderr)


if __name__ == "__main__":
    unittest.main()
