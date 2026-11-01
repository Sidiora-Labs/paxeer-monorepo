from pathlib import Path
import subprocess
import unittest


ROOT = Path(__file__).resolve().parents[2]
PRODUCERS = (
    'tools/qualification/paxeer-x/programs_accounts_winddown.py',
    'tools/qualification/paxeer-x/movement-custody-profile.py',
)


class RetiredCallers11Test(unittest.TestCase):
    def test_producers_name_no_retired_construct(self):
        command = ['python3', 'tools/paxeer-x/tests/retired_callers_test.py']
        for component in ('custody-credit', 'custody-deposit', 'anchor', 'custody-claims'):
            command.extend(['--component', component])
        for producer in PRODUCERS:
            command.extend(['--path', producer])
        result = subprocess.run(command, cwd=ROOT, capture_output=True, text=True,
                                timeout=30)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        for producer in PRODUCERS:
            with self.subTest(producer=producer):
                result = subprocess.run(['python3', producer, '--help'], cwd=ROOT,
                                        capture_output=True, text=True, timeout=30)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)


if __name__ == '__main__':
    unittest.main()
