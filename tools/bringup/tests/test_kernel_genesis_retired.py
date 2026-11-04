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
        self.assertTrue({"human-event-client", "human-kms", "human-kms-client", "human-kms-executor", "registry", "gateway-client"} <= names)
        self.assertEqual(len(names), 30)
        self.assertNotIn("receipt-authority", names)
        self.assertNotIn("guarantor-1", names)
        self.assertNotIn("guarantor-2", names)
        self.assertNotIn("layerxd", names)
        self.assertNotIn("guarantor", names)
        self.assertNotIn("relay-archive", names)

        value_loop = (root / "tools/bringup/value-loop.sh").read_text()
        self.assertIn('committed-transaction "$rpc" "0x$activity"', value_loop)
        self.assertIn("latest_committed_height", value_loop)
        self.assertNotIn("--socket", value_loop)
