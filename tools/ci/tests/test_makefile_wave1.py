import os
import pathlib
import re
import subprocess
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[3]
LIBRARY_LINE = re.compile(
    r"^\S+ .* -c src/\S+\.c -o build(?:-repro-[ab]|/(?:crypto-)?(?:a|t|ub)san)?/(?:obj|test-obj)/src/\S+\.o$|^\S+ rcsD build(?:-repro-[ab]|/(?:crypto-)?(?:a|t|ub)san)?/liblayerx(?:-testing)?\.a |^rm -f build(?:-repro-[ab]|/(?:crypto-)?(?:a|t|ub)san)?/liblayerx(?:-testing)?\.a$"
)


class MakefileWaveOneTest(unittest.TestCase):
    def test_retired_targets_have_no_rule(self):
        for target in (
            "test-replay-golden",
            "test-replay-golden-local",
            "test-oracle-adapter",
            "test-oracle-root",
            "test-web-adapter",
            "test-web-intake",
            "test-web-root",
            "test-web-program-path",
            "test-idempotency",
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
                "test-wave-8",
                "test-journal",
                "test-oracle-intake",
                "test-oracle-bounds",
                "test-oracle-failclosed",
            ],
            cwd=ROOT,
            env={**os.environ, "LC_ALL": "C"},
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        for retired in (
            "lx_oracle_adapter",
            "lx_oracle_root",
            "replay-golden",
        ):
            with self.subTest(retired=retired):
                self.assertNotIn(retired, result.stdout)
        non_library_stdout = "\n".join(
            line for line in result.stdout.splitlines() if not LIBRARY_LINE.match(line)
        )
        for retired in (
            "lx_web_adapter",
            "lx_web_root",
            "lxp_idempotency",
            "lxp_checkpoint_cert",
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
        wave_one_sources = {
            "src/storage/lxp_projection.c",
            "src/network/lx_oracle_adapter.c",
            "src/network/lx_web_adapter.c",
            "src/sequencer/lx_oracle_root.c",
            "src/sequencer/lx_web_root.c",
            "src/state/lxp_idempotency.c",
            "src/paxeer/lxp_checkpoint_cert.c",
        }
        self.assertEqual(
            set(result.stdout.splitlines()) & wave_one_sources,
            {
                "src/storage/lxp_projection.c",
                "src/network/lx_oracle_adapter.c",
                "src/sequencer/lx_oracle_root.c",
            },
        )


if __name__ == "__main__":
    unittest.main()
