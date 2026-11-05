#!/usr/bin/env python3
from pathlib import Path
import subprocess
import unittest

ROOT = Path(__file__).resolve().parents[3]


class GatesCustodyRetiredTest(unittest.TestCase):
    def test_gates_name_no_root_contract_or_credit(self):
        result = subprocess.run(
            [
                "python3",
                "tools/paxeer-x/tests/retired_callers_test.py",
                "--component",
                "anchor",
                "--component",
                "custody-claims",
                "--component",
                "custody-credit",
                "--path",
                "tools/paxeer-x/gates/24.16.sh",
                "--path",
                "tools/paxeer-x/gates/5.1.sh",
            ],
            cwd=ROOT,
            stdin=subprocess.DEVNULL,
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main(verbosity=2)
