import os
import pathlib
import re
import subprocess
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[3]


class MakefileWaveFourTest(unittest.TestCase):
    def test_retired_targets_have_no_rule(self):
        for target in (
            "test-daemon-lni-admission",
            "test-daemon-lni-account",
            "test-daemon-lni-module",
            "test-daemon-lni-head-attestation",
            "test-paxeer",
            "test-emergency-exit",
            "test-receipt-offline",
            "build/contracts/.paxeer-built",
            "build/tests/test_daemon_lni_admission",
            "build/tests/lxp_test_lni_account",
            "build/tests/lxp_test_lni_module",
            "build/tests/lxp_test_lni_head_attestation",
            "build/tests/test_paxeer_custody",
            "build/tests/test_emergency_exit",
            "build/tests/test_receipt_offline",
            "build/tests/lxp_test_module_custody",
        ):
            with self.subTest(target=target):
                result = subprocess.run(
                    ["make", "-n", target],
                    cwd=ROOT,
                    env={**os.environ, "LC_ALL": "C"},
                    capture_output=True,
                    text=True,
                    check=False,
                )
                self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
                self.assertIn(f"No rule to make target '{target}'", result.stderr)

    def test_aggregates_dry_run_clean(self):
        result = subprocess.run(
            [
                "make",
                "-n",
                "test",
                "core-test-all",
                "test-paxeer-bond",
                "test-genesis-builder-cli",
            ],
            cwd=ROOT,
            env={**os.environ, "LC_ALL": "C"},
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        for retired in (
            "contracts/LayerXCustody.sol",
            "contracts/CheckpointRegistry.sol",
            "contracts/GuarantorBond.sol",
            "lxp_daemon_modules.h",
        ):
            with self.subTest(retired=retired):
                self.assertNotIn(retired, result.stdout)

        commands = result.stdout.replace("\\\n", " ").splitlines()
        genesis_commands = [
            line for line in commands if "tests/test_genesis_builder_cli.c" in line
        ]
        self.assertEqual(len(genesis_commands), 1, result.stdout)
        self.assertIn("tests/lxp_test_harness.c", genesis_commands[0].split())
        self.assertIn("-Itests", genesis_commands[0].split())
        makefile = (ROOT / "Makefile").read_text()
        genesis_rule = re.search(
            r"^\$\(BUILD_DIR\)/tests/test_genesis_builder_cli:(.*?)\n\t@mkdir",
            makefile,
            re.M | re.S,
        )
        self.assertIsNotNone(genesis_rule)
        self.assertIn("tests/lxp_test_harness.c", genesis_rule.group(1))


if __name__ == "__main__":
    unittest.main()
