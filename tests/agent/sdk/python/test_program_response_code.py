import json
import unittest
from copy import deepcopy
from pathlib import Path

from layerx_sdk.programs import _execution

from program_receipt_evidence import ProgramReceiptEvidence


class ProgramResponseCodeTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.evidence = ProgramReceiptEvidence()
        fixture = Path(__file__).resolve().parents[4] / "tests/fixtures/programs/emulator-response-code"
        cls.document = json.loads((fixture / "execution.json").read_text())
        cls.expected = json.loads((fixture / "expected.json").read_text())

    def execution(self):
        return deepcopy(self.document)

    def test_original_executed_guest_response_seven_has_protocol_success_zero(self):
        execution = _execution(self.execution(), "executed")
        self.assertEqual(execution["result_code"], 0)
        self.assertEqual(execution["outcome"]["kind"], "completed")
        self.assertEqual(execution["outcome"]["code"], 7)
        self.assertEqual(execution["authority"]["previous_state_root"], self.expected["previous_state_root"])
        verified = self.evidence.verify()
        self.assertEqual(verified.receipt.result_code, 0)
        self.assertEqual(self.evidence.submission()["status"], "0x1")

    def test_guest_response_and_protocol_result_remain_independently_authenticated(self):
        evidence = self.evidence
        self.assertEqual(evidence.submission(), evidence.vector["receiptJson"])
        for field, value in (("returnData", "0x00"), ("status", "0x0")):
            changed = deepcopy(evidence.vector["receiptJson"])
            changed[field] = value
            with self.subTest(field=field), self.assertRaisesRegex(ValueError, "binding failed"):
                evidence.submission(changed)
        changed = deepcopy(evidence.vector["receiptJson"])
        changed["logs"][0]["data"] = "0x00"
        with self.assertRaisesRegex(ValueError, "program log binding failed"):
            evidence.submission(changed)
        with self.assertRaisesRegex(ValueError, "receipt binding failed"):
            evidence.verify({**evidence.execution, "result_code": 1})
