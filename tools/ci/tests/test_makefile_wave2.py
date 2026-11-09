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
            "lxp_replica",
            "lxp_batch_seal",
            "lxp_da_kernel",
            "cmd/layerx-module-registry/node.c",
            "lxp_guarantor",
            "src/network/lxp_da_bundle.c",
            "src/network/lxp_da_service.c",
            "lxp_da_",
        ):
            with self.subTest(retired=retired):
                self.assertNotIn(retired, result.stdout)

    def test_library_exclusions_exact(self):
        result = subprocess.run(
            [
                "make",
                "-s",
                "--no-print-directory",
                "--eval",
                "lx-excluded: ; @printf '%s\\n' "
                "'$(sort $(LIB_EXCLUDED_SOURCES))' '$(sort $(LIB_SOURCES))'",
                "lx-excluded",
            ],
            cwd=ROOT,
            env={**os.environ, "LC_ALL": "C"},
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        declared, included = (set(line.split()) for line in result.stdout.splitlines())
        # Projection stays out of the native library; the node owns projection.
        # Oracle adapter/root are retired: native pricefeed/oracle reads replace them.
        # DA challenges belong to the retired guarantor/checkpoint execution domain.
        expected = {
            "src/storage/lxp_projection.c",
            "src/network/lx_oracle_adapter.c",
            "src/sequencer/lx_oracle_root.c",
            "src/paxeer/lxp_da_challenge_record.c",
            "src/guarantor/lxp_da_challenge.c",
        }
        self.assertEqual(declared, expected)
        sources = {str(path.relative_to(ROOT)) for path in (ROOT / "src").rglob("*.c")}
        self.assertEqual(included, sources - declared)
        self.assertEqual(sources - included, sources & declared)
        self.assertTrue((ROOT / "modules/pricefeed").is_dir())
        self.assertTrue((ROOT / "src/storage/lxp_projection.c").is_file())


if __name__ == "__main__":
    unittest.main()
