import json
import unittest
from copy import deepcopy
from pathlib import Path

from layerx_sdk.production import PlatformSdkError
from layerx_sdk.programs import _execution
from layerx_sdk.verifier import SequencerReceiptVerificationError

from program_receipt_evidence import ProgramReceiptEvidence


class ProgramExecutedV3Test(unittest.TestCase):
    def test_actual_kernel_runtime_receipt_and_module_binding(self):
        evidence = ProgramReceiptEvidence()
        verified = evidence.verify()
        self.assertEqual(verified, evidence.verified)
        self.assertEqual(
            (verified.receipt.module_id, verified.receipt.module_version, verified.receipt.operation),
            (12, 1, 2),
        )
        self.assertEqual(verified.receipt_digest.hex(), evidence.vector["receiptDigest"][2:])
        self.assertEqual(evidence.submission(), evidence.vector["receiptJson"])
        with self.assertRaisesRegex(ValueError, "receipt binding failed"):
            evidence.verify({**evidence.execution, "module_version": 3})
        document = json.loads(
            (Path(__file__).resolve().parents[4]
             / "tests/fixtures/programs/emulator-response-code/execution.json").read_text()
        )
        parsed = _execution(document, "executed")
        self.assertEqual(parsed["module_version"], 4)
        self.assertEqual(parsed["authority"], document["authority"])
        for field in document["authority"]:
            changed = deepcopy(document)
            del changed["authority"][field]
            with self.subTest(missing_authority=field), self.assertRaises(ValueError):
                _execution(changed, "executed")
        for field in ("module_version", "guest_abi_version", "result_code"):
            with self.subTest(field=field), self.assertRaises(ValueError):
                _execution({**document, field: True}, "executed")
        for value in (0, 6, True):
            with self.subTest(module=value), self.assertRaises(ValueError):
                _execution({**document, "module_version": value}, "executed")
        corrupted = evidence.encoded[:-1] + bytes([evidence.encoded[-1] ^ 1])
        changed = deepcopy(evidence.execution)
        changed["receipt"] = corrupted.hex()
        with self.assertRaises(SequencerReceiptVerificationError) as raised:
            evidence.verify(changed)
        self.assertEqual(raised.exception.check, "bad_signature")
        with self.assertRaises(PlatformSdkError):
            evidence.verify({**evidence.execution, "receipt": document["receipt"]})
