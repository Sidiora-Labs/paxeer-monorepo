from pathlib import Path
import subprocess
import sys
import unittest


ROOT = Path(__file__).resolve().parents[2]
PRODUCERS = (
    "tools/qualification/paxeer-x/native-programs-link.py",
    "tools/qualification/paxeer-x/build-caps-discovery.sh",
)
COMPONENTS = ("finality-authority", "custody-credit", "replica", "batch-seal")


class RetiredCallers5Test(unittest.TestCase):
    def test_producers_name_no_retired_construct(self):
        command = [sys.executable, "tools/paxeer-x/tests/retired_callers_test.py"]
        for component in COMPONENTS:
            command.extend(("--component", component))
        for producer in PRODUCERS:
            command.extend(("--path", producer))
        result = subprocess.run(
            command, cwd=ROOT, capture_output=True, text=True, timeout=60
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

        for producer in PRODUCERS:
            if Path(producer).suffix == ".py":
                with self.subTest(producer=producer):
                    result = subprocess.run(
                        [sys.executable, producer, "--help"],
                        cwd=ROOT,
                        capture_output=True,
                        text=True,
                        timeout=60,
                    )
                    self.assertEqual(result.returncode, 0, result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
