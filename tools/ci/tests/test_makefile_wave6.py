import os
import pathlib
import subprocess
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[3]


class MakefileWaveSixTest(unittest.TestCase):
    def test_retired_targets_have_no_rule(self):
        for target in (
            "layerxd",
            "layerx-handover",
            "layerx-guarantor",
            "layerx-verify",
            "layerxctl",
            "test-layerxctl",
            "test-layerxd",
            "test-tools",
            "test-verify-cli",
            "test-lni-version-parity",
            "test-daemon-metered-allowance",
            "test-daemon-native-onboarding",
            "test-daemon-module-maintenance",
            "test-daemon-handover",
            "test-daemon-handover-crash",
            "test-daemon-handover-peers",
            "test-daemon-guarantor-feedback",
            "test-daemon-handover-consumers",
            "test-program-admission",
            "test-daemon-withdrawal",
            "test-daemon-withdrawal-replica-recovery",
            "test-daemon-paid-withdrawal",
            "test-program-simulate",
            "test-daemon-maintenance-publication",
            "test-daemon-maintenance-crash",
            "test-bridge-maintenance-publication",
            "test-daemon-allowance",
            "test-daemon-availability",
            "test-daemon-guarantor",
            "test-daemon-guarantor-unit",
            "test-daemon-guarantor-integration",
            "test-daemon-guarantor-sanitize",
            "test-daemon-guarantor-settlement",
            "platform-hosted-guarantor-topology-check",
            "test-guarantor-receipt",
            "test-guarantor-terminal-rejection",
            "test-daemon-grant-issuance",
            "authority-grant-fixtures",
            "authority-grant-fixtures-check",
            "test-daemon-owner-rotation",
            "relay-archive-build",
            "relay-archive-e2e",
            "paxeer-x-finality-recordings-build",
            "test-sequencer",
            "test-sequencer-recovery",
            "test-batch-time",
            "test-replay-crossarch",
            "test-paxeer-bond",
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
        self.assertFalse((ROOT / "scripts/ci/lni-version-parity.sh").exists())

    def test_aggregates_dry_run_clean(self):
        result = subprocess.run(
            [
                "make",
                "-n",
                "build",
                "test",
                "core-test-all",
                "core-qualification-environment",
                "beta-qualify-focused",
                "human-test",
                "test-tests-python",
            ],
            cwd=ROOT,
            env={**os.environ, "LC_ALL": "C"},
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        for retired in (
            "cmd/layerxd",
            "cmd/layerx-guarantor",
            "cmd/layerx-handover",
            "cmd/layerx-verify",
            "tests/daemon",
            "layerx-archive-codec",
            "layerx-human-movement-provider",
        ):
            with self.subTest(retired=retired):
                offending = [line for line in result.stdout.splitlines() if retired in line]
                self.assertEqual(offending, [], "\n".join(offending))
        self.assertIn("'make build layerx-genesis-build'", result.stdout)
        commands = result.stdout.replace("\\\n", " ").splitlines()
        python_suites = (
            "tests/ci/test_real_node_build_ownership.py",
            "tests/ci/test_runner_routing_container_runtime.py",
            "tests/test_lxgb_producers.py",
            "tests/test_beta_withdrawal_genesis.py",
        )
        suite_commands = [line for line in commands if python_suites[0] in line]
        self.assertEqual(len(suite_commands), 1, result.stdout)
        self.assertIn("python3 -m pytest", suite_commands[0])
        for suite in python_suites:
            self.assertIn(suite, suite_commands[0].split())
        makefile = (ROOT / "Makefile").read_text()
        harness_end = makefile.index("\n\n", makefile.index("\ntest-harness:"))
        next_rule = makefile[harness_end:].lstrip().splitlines()
        self.assertEqual(next_rule[0], ".PHONY: test-tests-python")
        self.assertEqual(next_rule[1], "test-tests-python:")


if __name__ == "__main__":
    unittest.main()
