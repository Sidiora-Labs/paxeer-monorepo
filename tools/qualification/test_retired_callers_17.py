from pathlib import Path
import subprocess
import unittest


ROOT = Path(__file__).resolve().parents[2]
PRODUCERS = (
    'tools/qualification/paxeer-x/program_balance_read.py',
    'tools/qualification/paxeer-x/program_cache_native_equivalence.py',
    'tools/qualification/paxeer-x/program_market_attestation.py',
)


class RetiredCallers17Test(unittest.TestCase):
    def test_producers_name_no_retired_construct(self):
        command = [
            'python3', 'tools/paxeer-x/tests/retired_callers_test.py',
            '--component', 'anchor', '--component', 'checkpoint-evidence',
        ]
        for producer in PRODUCERS:
            command.extend(['--path', producer])
        result = subprocess.run(
            command, cwd=ROOT, stdin=subprocess.DEVNULL,
            capture_output=True, text=True, timeout=30, check=False)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

        for producer in PRODUCERS:
            with self.subTest(producer=producer):
                source = (ROOT / producer).read_text()
                self.assertNotIn('contracts/config/checkpoint-settlement.json', source)
                if producer == PRODUCERS[0]:
                    self.assertNotIn('tests/programs/test_balance_read.c', source)
                result = subprocess.run(
                    ['python3', producer, '--help'], cwd=ROOT,
                    stdin=subprocess.DEVNULL, capture_output=True, text=True,
                    timeout=30, check=False)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn('usage:', result.stdout)


if __name__ == '__main__':
    unittest.main()
