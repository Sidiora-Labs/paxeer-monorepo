import subprocess
from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
PRODUCERS = (
    'tools/qualification/paxeer-x/custody_governance.py',
    'tools/qualification/paxeer-x/caps-codecs.py',
)


class RetiredCallers12Test(unittest.TestCase):
    def test_producers_name_no_retired_construct(self):
        command = ['python3', 'tools/paxeer-x/tests/retired_callers_test.py']
        for component in ('custody-credit', 'custody-deposit', 'anchor', 'custody-claims'):
            command.extend(['--component', component])
        for producer in PRODUCERS:
            command.extend(['--path', producer])
        result = subprocess.run(command, cwd=ROOT, text=True, capture_output=True, timeout=60)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(result.stdout, '')
        for producer in PRODUCERS:
            with self.subTest(producer=producer):
                result = subprocess.run(
                    ['python3', producer, '--help'],
                    cwd=ROOT,
                    text=True,
                    capture_output=True,
                    timeout=30,
                )
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn('usage:', result.stdout)


if __name__ == '__main__':
    unittest.main()
