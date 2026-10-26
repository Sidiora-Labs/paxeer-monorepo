import pathlib
import subprocess
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[3]


class ExchangeNativeAggregateTest(unittest.TestCase):
    def test_aggregate_runs_every_member(self):
        result = subprocess.run(
            ["make", "-n", "test-exchange-native"],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        commands = {line.strip() for line in result.stdout.splitlines()}
        for member in (
            "test_perps_market",
            "test_perps_book",
            "test_spot_book",
            "test_perps_position",
            "test_perps_funding",
            "test_perps_liquidate",
            "test_perps_insurance",
        ):
            with self.subTest(member=member):
                self.assertIn(f"build/tests/{member}", commands, result.stdout)


if __name__ == "__main__":
    unittest.main()
