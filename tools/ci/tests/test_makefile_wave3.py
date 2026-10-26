import os
import pathlib
import re
import subprocess
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[3]


class MakefileWaveThreeTest(unittest.TestCase):
    def test_retired_targets_have_no_rule(self):
        for target in (
            "agent-test-lni-schema",
            "agent-test-lni-transport",
            "agent-test-lni-handshake",
            "agent-test-lni-abi",
            "agent-test-wire-parity",
            "agent-qualify-wire",
            "agent-test-wire-primitives",
            "agent-test-wire-structures",
            "agent-test-wire-rejection",
            "agent-test-wire-hashing",
            "build/agent/layerxd-lni",
            "build/agent/lni-preparation-state",
            "build/agent-wire-reference",
            "build/tests/programs_fee_governance",
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

        database = subprocess.run(
            ["make", "-np", "agent-test"],
            cwd=ROOT,
            env={**os.environ, "LC_ALL": "C"},
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(database.returncode, 0, database.stdout + database.stderr)
        self.assertNotRegex(
            database.stdout, re.compile(r"^AGENT_WIRE_HARNESS\s*[:?+!]?=", re.M)
        )

    def test_aggregates_dry_run_clean(self):
        rpc_url = "http://127.0.0.1:18545"
        result = subprocess.run(
            [
                "make",
                "-n",
                f"AGENT_NODE_RPC_URL={rpc_url}",
                "agent-test",
                "agent-qualify-boundary",
                "programs-core-test",
                "programs-protocol-regression",
                "test-genesis-module-table",
            ],
            cwd=ROOT,
            env={**os.environ, "LC_ALL": "C"},
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn(f'"{rpc_url}"', result.stdout)
        makefile = (ROOT / "Makefile").read_text()
        recipes = "\n".join(
            line for line in makefile.splitlines() if line.startswith("\t")
        )
        for retired in ("layerx-wire", "layerxd-lni", "programs_fee_governance"):
            with self.subTest(retired=retired):
                self.assertNotIn(retired, result.stdout)
                self.assertNotIn(retired, recipes)

        for target in (
            "agent-test-boundary",
            "agent-qualify-boundary",
            "agent-test-sdk-parity",
        ):
            with self.subTest(target=target):
                rule = re.search(rf"^{target}:.*?(?=\n\n)", makefile, re.M | re.S)
                self.assertIsNotNone(rule, f"missing {target} rule")
                self.assertIn('"$(AGENT_NODE_RPC_URL)"', rule.group())

        commands = result.stdout.replace("\\\n", " ").splitlines()
        genesis_commands = [
            line
            for line in commands
            if "tests/test_genesis_module_table.c" in line
        ]
        self.assertEqual(len(genesis_commands), 1, result.stdout)
        self.assertIn("tests/lxp_test_harness.c", genesis_commands[0])
        self.assertIn("-Itests", genesis_commands[0].split())
        genesis_rule = re.search(
            r"^\$\(BUILD_DIR\)/tests/test_genesis_module_table:(.*?)\n\t@mkdir",
            makefile,
            re.M | re.S,
        )
        self.assertIsNotNone(genesis_rule)
        self.assertIn("tests/lxp_test_harness.c", genesis_rule.group(1))


if __name__ == "__main__":
    unittest.main()
