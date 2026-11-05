import pathlib
import subprocess
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[3]


class ProgramHttpProducerTest(unittest.TestCase):
    def test_producer_uses_no_lni(self):
        scan = subprocess.run(
            [
                "python3",
                "tools/paxeer-x/tests/retired_callers_test.py",
                "--component",
                "lni",
                "--path",
                "scripts/qualification/paxeer-x/program-http-producer.py",
            ],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(scan.returncode, 0, scan.stdout + scan.stderr)

        help_result = subprocess.run(
            ["python3", "scripts/qualification/paxeer-x/program-http-producer.py", "--help"],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(
            help_result.returncode, 0, help_result.stdout + help_result.stderr
        )


if __name__ == "__main__":
    unittest.main()
