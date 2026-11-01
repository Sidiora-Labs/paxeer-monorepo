import os
import pathlib
import re
import subprocess
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[3]
LIBRARY_LINE = re.compile(
    r"^\S+ .* -c src/\S+\.c -o build(?:-repro-[ab]|/(?:crypto-)?(?:a|t|ub)san)?/(?:obj|test-obj)/src/\S+\.o$|^\S+ rcsD build(?:-repro-[ab]|/(?:crypto-)?(?:a|t|ub)san)?/liblayerx(?:-testing)?\.a |^rm -f build(?:-repro-[ab]|/(?:crypto-)?(?:a|t|ub)san)?/liblayerx(?:-testing)?\.a$"
)


class MakefileWaveTwoTest(unittest.TestCase):
    def test_retired_targets_have_no_rule(self):
        for target in (
            "test-da",
            "test-da-possession",
            "test-da-verified",
            "test-da-retrieval",
            "test-da-challenge",
            "test-da-unavailable",
            "test-replica",
            "test-replay",
            "test-replica-divergence",
            "test-batch-seal",
            "test-batch-distribute",
            "test-gateway-receive",
            "test-guarantor",
            "test-guarantor-cert",
            "test-guarantor-bond",
            "test-guarantor-disagreement",
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
                "test-wave-10",
                "test-wave-12",
                "layerx-module-registry",
            ],
            cwd=ROOT,
            env={**os.environ, "LC_ALL": "C"},
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        for retired in (
            "test_batch_publish",
            "test_gateway_receive",
            "cmd/layerx-module-registry/node.c",
        ):
            with self.subTest(retired=retired):
                self.assertNotIn(retired, result.stdout)
        non_library_stdout = "\n".join(
            line for line in result.stdout.splitlines() if not LIBRARY_LINE.match(line)
        )
        for retired in (
            "lxp_da_",
            "lxp_replica",
            "lxp_batch_seal",
            "lxp_guarantor",
        ):
            with self.subTest(retired=retired):
                self.assertNotIn(retired, non_library_stdout)

    def test_library_exclusions_exact(self):
        result = subprocess.run(
            [
                "make",
                "-s",
                "--no-print-directory",
                "--eval",
                'lx-excluded: ; @printf "%s\\n" $(sort $(filter-out $(LIB_SOURCES),$(shell find src -type f -name "*.c" -print)))',
                "lx-excluded",
            ],
            cwd=ROOT,
            env={**os.environ, "LC_ALL": "C"},
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(
            set(result.stdout.splitlines()),
            {
                "src/storage/lxp_projection.c",
                "src/network/lx_oracle_adapter.c",
                "src/sequencer/lx_oracle_root.c",
                "src/paxeer/lxp_da_challenge_record.c",
                "src/guarantor/lxp_da_challenge.c",
            },
        )


if __name__ == "__main__":
    unittest.main()
