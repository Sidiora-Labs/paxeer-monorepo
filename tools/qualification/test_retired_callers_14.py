from pathlib import Path
import subprocess
import unittest


ROOT = Path(__file__).resolve().parents[2]
PRODUCERS = (
    'tools/qualification/paxeer-x/native_send_profile.py',
    'tools/qualification/paxeer-x/programs_source_abi.py',
)


class RetiredCallers14Test(unittest.TestCase):
    def test_producers_name_no_retired_construct(self):
        command = [
            'python3',
            'tools/paxeer-x/tests/retired_callers_test.py',
            '--component',
            'lni',
        ]
        for producer in PRODUCERS:
            command.extend(['--path', producer])
        result = subprocess.run(
            command, cwd=ROOT, capture_output=True, text=True, timeout=60,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        for producer in PRODUCERS:
            with self.subTest(producer=producer):
                result = subprocess.run(
                    ['python3', producer, '--help'],
                    cwd=ROOT, capture_output=True, text=True, timeout=60,
                )
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn('usage:', result.stdout)
