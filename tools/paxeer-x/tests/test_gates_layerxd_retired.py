#!/usr/bin/env python3
from pathlib import Path
import subprocess
import unittest

ROOT = Path(__file__).resolve().parents[3]


class GatesLayerxdRetiredTest(unittest.TestCase):
    def test_gates_name_no_layerxd_or_replica(self):
        result = subprocess.run(
            [
                "python3",
                "tools/paxeer-x/tests/retired_callers_test.py",
                "--component",
                "replica",
                "--component",
                "guarantor-runtime",
                "--component",
                "finality-authority",
                "--path",
                "tools/paxeer-x/gates/104.30.5.sh",
                "--path",
                "tools/paxeer-x/gates/104.32.6.sh",
                "--path",
                "tools/paxeer-x/gates/111.6.11.sh",
                "--path",
                "tools/paxeer-x/build/104.35.35.mk",
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
