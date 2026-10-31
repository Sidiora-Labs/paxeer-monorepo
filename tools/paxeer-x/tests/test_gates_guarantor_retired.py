#!/usr/bin/env python3
from pathlib import Path
import subprocess
import unittest

ROOT = Path(__file__).resolve().parents[3]


class GatesGuarantorRetiredTest(unittest.TestCase):
    def test_gates_name_no_retired_daemon(self):
        result = subprocess.run(
            [
                "python3",
                "tools/paxeer-x/tests/retired_callers_test.py",
                "--component",
                "guarantor-runtime",
                "--component",
                "finality-authority",
                "--path",
                "tools/paxeer-x/gates/24.14.sh",
            ],
            cwd=ROOT,
            stdin=subprocess.DEVNULL,
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        retired_gate = ROOT / "tools/paxeer-x/gates/2.2.sh"
        self.assertFalse(retired_gate.exists() or retired_gate.is_symlink())
        result = subprocess.run(
            ["bash", "-n", "tools/paxeer-x/gates/24.14.sh"],
            cwd=ROOT,
            stdin=subprocess.DEVNULL,
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main(verbosity=2)
