import subprocess
import unittest
from pathlib import Path


class KernelGenesisRetiredTest(unittest.TestCase):
    def test_bringup_names_no_guarantor_or_custody_proof(self):
        root = Path(__file__).resolve().parents[3]
        command = [
            "python3",
            "tools/paxeer-x/tests/retired_callers_test.py",
            "--component",
            "guarantor-runtime",
            "--component",
            "custody-credit",
            "--component",
            "replica",
        ]
        for path in (
            "tools/bringup/kernel-genesis.sh",
            "tools/bringup/value-loop.sh",
            "tools/bringup/human-state-preserve.sh",
            "tools/bringup/ca.sh",
        ):
            command.extend(["--path", path])
        scan = subprocess.run(
            command,
            cwd=root,
            capture_output=True,
            text=True,
            timeout=60,
            check=False,
        )
        self.assertEqual(scan.returncode, 0, scan.stdout + scan.stderr)

        services = subprocess.run(
            ["bash", "tools/bringup/ca.sh", "services"],
            cwd=root,
            capture_output=True,
            text=True,
            timeout=60,
            check=False,
        )
        self.assertEqual(services.returncode, 0, services.stdout + services.stderr)
        names = {line.split()[0] for line in services.stdout.splitlines()}
        self.assertIn("receipt-authority", names)
        self.assertNotIn("guarantor", names)
        self.assertNotIn("relay-archive", names)
