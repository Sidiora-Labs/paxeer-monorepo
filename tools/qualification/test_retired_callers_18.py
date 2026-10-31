from pathlib import Path
import subprocess
import unittest


ROOT = Path(__file__).resolve().parents[2]
PRODUCERS = (
    "tools/qualification/paxeer-x/program_spend_composition.py",
    "tools/qualification/paxeer-x/program_transfer_authority.py",
    "tools/qualification/paxeer-x/wallet-402-caps.py",
)


class RetiredCallers18Test(unittest.TestCase):
    def test_producers_name_no_retired_construct(self):
        command = [
            "python3",
            "tools/paxeer-x/tests/retired_callers_test.py",
            "--component",
            "anchor",
            "--component",
            "checkpoint-evidence",
        ]
        for path in PRODUCERS:
            command.extend(["--path", path])
        result = subprocess.run(
            command, cwd=ROOT, capture_output=True, text=True, timeout=60
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        for path in PRODUCERS:
            with self.subTest(producer=path):
                result = subprocess.run(
                    ["python3", path, "--help"],
                    cwd=ROOT,
                    capture_output=True,
                    text=True,
                    timeout=60,
                )
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
