from pathlib import Path
import shutil
import subprocess
import unittest


class SolidityStateSurfaceTest(unittest.TestCase):
    def test_surface_check_passes_on_kept_contracts(self):
        if shutil.which("forge") is None:
            self.skipTest("forge is absent from PATH; command: sh scripts/ci/solidity-state-surface.sh")
        root = Path(__file__).resolve().parents[3]
        result = subprocess.run(
            ["sh", "scripts/ci/solidity-state-surface.sh"],
            cwd=root,
            capture_output=True,
            text=True,
            timeout=120,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
