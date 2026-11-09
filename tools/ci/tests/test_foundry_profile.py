import json
from pathlib import Path
import shutil
import subprocess
import unittest


class FoundryProfileTest(unittest.TestCase):
    def test_root_profile_grants_no_retired_paths(self):
        if shutil.which("forge") is None:
            self.skipTest("forge is absent from PATH; command: forge config --json")
        root = Path(__file__).resolve().parents[3]
        result = subprocess.run(
            ["forge", "config", "--json"],
            cwd=root,
            capture_output=True,
            text=True,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        config = json.loads(result.stdout)
        permissions = {
            (root / permission["path"]).resolve()
            for permission in config["fs_permissions"]
        }
        for retired in ("tests/vectors/checkpoint", "contracts/config"):
            with self.subTest(path=retired):
                self.assertNotIn((root / retired).resolve(), permissions)
        self.assertEqual(
            (root / config["test"]).resolve(), (root / "tests/solidity").resolve()
        )


if __name__ == "__main__":
    unittest.main()
