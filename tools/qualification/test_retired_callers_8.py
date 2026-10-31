from pathlib import Path
import subprocess
import unittest


ROOT = Path(__file__).resolve().parents[2]
PRODUCERS = (
    'tools/qualification/paxeer-x/storage-recovery-producer.py',
    'tools/qualification/paxeer-x/explorer-receipt-reader.py',
)


class RetiredCallers8Test(unittest.TestCase):
    def test_producers_name_no_retired_construct(self):
        command = [
            'python3',
            'tools/paxeer-x/tests/retired_callers_test.py',
            '--component', 'finality-authority',
            '--component', 'replica',
            '--component', 'handover-client',
        ]
        for path in PRODUCERS:
            command.extend(['--path', path])
        result = subprocess.run(
            command, cwd=ROOT, stdin=subprocess.DEVNULL,
            capture_output=True, text=True, timeout=60, check=False,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

        for relative in PRODUCERS:
            with self.subTest(producer=relative):
                path = ROOT / relative
                self.assertNotIn(
                    'tests/storage/check_daemon_recovery_order.sh',
                    path.read_text(), relative,
                )
                result = subprocess.run(
                    ['python3', str(path), '--help'], cwd=ROOT,
                    stdin=subprocess.DEVNULL, capture_output=True,
                    text=True, timeout=30, check=False,
                )
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn('usage:', result.stdout)

        result = subprocess.run(
            ['python3', PRODUCERS[1]], cwd=ROOT, stdin=subprocess.DEVNULL,
            capture_output=True, text=True, timeout=30, check=False,
        )
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        self.assertIn('receipt evidence production is retired', result.stderr)
        self.assertNotIn('PAXEER_X_GATE', result.stdout)


if __name__ == '__main__':
    unittest.main()
