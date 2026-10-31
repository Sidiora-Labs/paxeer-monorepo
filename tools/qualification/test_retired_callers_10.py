from pathlib import Path
import subprocess
import unittest


ROOT = Path(__file__).resolve().parents[2]
PRODUCERS = (
    "tools/qualification/paxeer-x/fixtures/xweb_fixture.py",
    "tools/qualification/paxeer-x/programs_all_form_validation.py",
)


class RetiredCallers10Test(unittest.TestCase):
    def test_producers_name_no_retired_construct(self):
        command = ["python3", "tools/paxeer-x/tests/retired_callers_test.py"]
        for component in ("custody-credit", "finality-authority", "custody-deposit"):
            command.extend(("--component", component))
        for path in PRODUCERS:
            command.extend(("--path", path))
        result = subprocess.run(command, cwd=ROOT, capture_output=True, text=True, timeout=60)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        for path in PRODUCERS:
            with self.subTest(producer=path):
                result = subprocess.run(
                    ["python3", path, "--help"], cwd=ROOT,
                    capture_output=True, text=True, timeout=30,
                )
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
