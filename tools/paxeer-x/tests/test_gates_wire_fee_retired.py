#!/usr/bin/env python3
from pathlib import Path
import subprocess
import unittest

ROOT = Path(__file__).resolve().parents[3]


class GatesWireFeeRetiredTest(unittest.TestCase):
    def test_gates_name_no_wire_codec_or_fee_relay(self):
        result = subprocess.run(
            [
                "python3",
                "tools/paxeer-x/tests/retired_callers_test.py",
                "--component",
                "wire-codec",
                "--path",
                "tools/paxeer-x/gates/103.6.11.sh",
            ],
            cwd=ROOT,
            stdin=subprocess.DEVNULL,
            capture_output=True,
            text=True,
            timeout=60,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        for path in (
            "tests/programs/test_fee_governance.c",
            "tools/qualification/paxeer-x/program_fee_governance.py",
        ):
            source = (ROOT / path).read_text(encoding="utf-8")
            for name in (
                "LX_PROGRAMS_FEE_GOVERNANCE",
                "native_signed_governance_producer_consumer",
            ):
                with self.subTest(path=path, name=name):
                    self.assertNotIn(name, source)


if __name__ == "__main__":
    unittest.main(verbosity=2)
