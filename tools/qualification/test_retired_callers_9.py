from pathlib import Path
import subprocess
import unittest


ROOT = Path(__file__).resolve().parents[2]
PRODUCERS = (
    "tools/qualification/paxeer-x/agent_operation_envelope.py",
    "tools/qualification/paxeer-x/event-delivery-contract.py",
)


class RetiredCallers9Test(unittest.TestCase):
    def test_producers_name_no_retired_construct(self):
        command = [
            "python3",
            "tools/paxeer-x/tests/retired_callers_test.py",
            "--component", "custody-credit",
            "--component", "finality-authority",
            "--component", "custody-deposit",
        ]
        for producer in PRODUCERS:
            command.extend(("--path", producer))
        scanned = subprocess.run(command, cwd=ROOT, capture_output=True, text=True, timeout=120)
        self.assertEqual(scanned.returncode, 0, scanned.stdout + scanned.stderr)
        for producer in PRODUCERS:
            with self.subTest(producer=producer):
                self.assertNotIn(
                    "tests/storage/lxp_test_maintenance_publication.c",
                    (ROOT / producer).read_text(),
                )
                helped = subprocess.run(
                    ["python3", producer, "--help"],
                    cwd=ROOT,
                    capture_output=True,
                    text=True,
                    timeout=30,
                )
                self.assertEqual(helped.returncode, 0, helped.stdout + helped.stderr)


if __name__ == "__main__":
    unittest.main()
