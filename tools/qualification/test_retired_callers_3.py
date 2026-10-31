import subprocess
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
PRODUCERS = (
    "tools/qualification/paxeer-x/authenticated-replay-step.py",
    "tools/qualification/paxeer-x/market-sandbox-capture.py",
    "tools/qualification/paxeer-x/market-sandbox-profile.py",
)


class RetiredCallers3Test(unittest.TestCase):
    def test_producers_name_no_retired_construct(self):
        command = [
            "python3",
            "tools/paxeer-x/tests/retired_callers_test.py",
            "--component",
            "wire-codec",
            "--component",
            "batch-seal",
        ]
        for producer in PRODUCERS:
            command.extend(["--path", producer])
        result = subprocess.run(
            command, cwd=ROOT, capture_output=True, text=True, timeout=60
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        for producer in PRODUCERS:
            with self.subTest(producer=producer):
                result = subprocess.run(
                    ["python3", producer, "--help"],
                    cwd=ROOT,
                    capture_output=True,
                    text=True,
                    timeout=30,
                )
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
