from pathlib import Path
import subprocess
import unittest


ROOT = Path(__file__).resolve().parents[2]


class KernelInitTest(unittest.TestCase):
    def test_init_lists_no_retired_service(self):
        syntax = subprocess.run(
            ["bash", "-n", "docker/kernel/init.sh"],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(syntax.returncode, 0, syntax.stdout + syntax.stderr)

        retired = subprocess.run(
            [
                "python3",
                "tools/paxeer-x/tests/retired_callers_test.py",
                "--component",
                "guarantor-runtime",
                "--component",
                "lni",
                "--path",
                "docker/kernel/init.sh",
                "--path",
                "docker/kernel/kernel.env.example",
                "--path",
                "docker/kernel/layerx-kernel.service",
            ],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(retired.returncode, 0, retired.stdout + retired.stderr)

        script = (ROOT / "docker/kernel/init.sh").read_text(encoding="utf-8")
        self.assertNotRegex(
            script,
            r"(?m)^\s*(?:service\s+(?:layerxd(?:-authority)?|guarantor(?:-[12])?|relay-archive)\b"
            r"|(?:guarantor|guarantor_prepare|relay_archive_prepare)\s*\(\)"
            r"|guarantor\s+[12]\b)",
        )
