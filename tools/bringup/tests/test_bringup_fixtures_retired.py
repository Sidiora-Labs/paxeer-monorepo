import subprocess
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]


class BringupFixturesRetiredTest(unittest.TestCase):
    def test_fixtures_name_no_retired_daemon(self):
        paths = (
            "tools/bringup/check-live.test.sh",
            "tools/bringup/tests/foundation-artifacts.py",
            "tools/bringup/tests/paxeer-x-fixture-foundation.py",
        )
        scanner = [
            "python3",
            "tools/paxeer-x/tests/retired_callers_test.py",
            "--component",
            "custody-credit",
            "--component",
            "finality-authority",
        ]
        for path in paths:
            scanner.extend(("--path", path))
        for command in (
            scanner,
            ["python3", "tools/bringup/tests/foundation-artifacts.py"],
        ):
            with self.subTest(command=command):
                result = subprocess.run(
                    command,
                    cwd=ROOT,
                    capture_output=True,
                    text=True,
                    timeout=30,
                    check=False,
                )
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

        for path in paths:
            with self.subTest(path=path):
                source = (ROOT / path).read_text()
                self.assertNotIn("layerx-custody-proof", source)
                if path.endswith("check-live.test.sh"):
                    for retired in (
                        "tests/daemon/paxeer_x_kernel_readiness.py",
                        "sign-credit",
                        "LAYERX_INDEXER_RELAY_URL",
                        "LAYERX_INDEXER_RELAY_CA_DER",
                    ):
                        self.assertNotIn(retired, source)


if __name__ == "__main__":
    unittest.main()
