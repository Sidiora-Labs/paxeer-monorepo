#!/usr/bin/env python3
from pathlib import Path
import subprocess
import sys
import unittest

ROOT = Path(__file__).resolve().parents[2]
PRODUCERS = (
    "tools/qualification/paxeer-x/arbiter_prestate.py",
    "tools/qualification/paxeer-x/asset-execution-prestate.py",
    "tools/qualification/paxeer-x/asset-inventory.py",
)


class RetiredCallers2Test(unittest.TestCase):
    def test_producers_name_no_retired_construct(self):
        command = [sys.executable, "tools/paxeer-x/tests/retired_callers_test.py"]
        for component in (
            "wire-codec",
            "batch-seal",
            "finality-authority",
            "guarantor-runtime",
        ):
            command.extend(["--component", component])
        for producer in PRODUCERS:
            command.extend(["--path", producer])
        result = subprocess.run(
            command, cwd=ROOT, capture_output=True, text=True, timeout=30
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        for producer in PRODUCERS:
            with self.subTest(producer=producer):
                path = ROOT / producer
                self.assertNotIn(
                    "tests/storage/lxp_test_maintenance_publication.c",
                    path.read_text(encoding="utf-8"),
                )
                result = subprocess.run(
                    [sys.executable, str(path), "--help"],
                    cwd=ROOT,
                    capture_output=True,
                    text=True,
                    timeout=30,
                )
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn("usage:", result.stdout)


if __name__ == "__main__":
    unittest.main()
