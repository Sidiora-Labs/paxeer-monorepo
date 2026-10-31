from pathlib import Path
import subprocess
import unittest


class SolidityStateSurfaceTest(unittest.TestCase):
    def test_surface_check_passes_on_kept_contracts(self):
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
