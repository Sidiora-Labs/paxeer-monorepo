import importlib.util
import json
import unittest
from pathlib import Path

from layerx_sdk.production import PlatformSdkError
from layerx_sdk.program_wire import decode_and_verify_program_terminal
from layerx_sdk.programs import ProgramTrustContext, _execution, verify_program_receipt
from layerx_sdk.verifier import AuthorizedReceiptBatch, verify_receipt_outcome


class ProgramExecutedV3Test(unittest.TestCase):
    def test_actual_kernel_runtime_receipt_and_module_binding(self):
        root = Path(__file__).resolve().parents[4]
        fixture = root / "tests/fixtures/programs/emulator-response-code"
        document = json.loads((fixture / "execution.json").read_text())
        spec = importlib.util.spec_from_file_location("executed_v3_signatures", root / "sdk/integrations/fastapi/layerx_fastapi/signatures.py")
        self.assertIsNotNone(spec)
        self.assertIsNotNone(spec.loader)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        signatures = module.LayerXSignatureVerifier()
        source = document["authority"]
        authority = AuthorizedReceiptBatch(**{field: bytes.fromhex(source[field]) for field in ("batch_id", "asset", "previous_state_root", "resulting_state_root", "sequencer_public_key")})
        canonical = bytes.fromhex((fixture / "receipt.hex").read_text())
        receipt = verify_receipt_outcome(canonical, authority, signatures, protocol_version=3).receipt
        self.assertEqual((receipt.protocol_version, receipt.module_version, receipt.operation), (3, 4, 3))
        self.assertIsNotNone(receipt.program_outcome)
        terminal = decode_and_verify_program_terminal(bytes.fromhex(document["terminal_payload"]), bytes.fromhex(document["call_graph"]), document["program_id"], receipt.program_outcome, 3)
        self.assertEqual(document["usage"], terminal.usage)
        self.assertEqual(document["outcome"], terminal.outcome)
        parsed = _execution(document, "executed")
        self.assertEqual(parsed["module_version"], 4)
        trust = ProgramTrustContext(authority.sequencer_public_key, protocol_version=3)
        verified = verify_program_receipt(parsed, authority, signatures, trust)
        self.assertEqual(verified.verification.receipt_digest.hex(), document["receipt_digest"])
        with self.assertRaises((ValueError, PlatformSdkError)):
            verify_program_receipt(_execution({**document, "module_version": 3}, "executed"), authority, signatures, trust)
        with self.assertRaises((ValueError, PlatformSdkError)):
            verify_program_receipt(parsed, authority, signatures, ProgramTrustContext(authority.sequencer_public_key, protocol_version=2))
        for field in ("module_version", "guest_abi_version", "result_code"):
            with self.assertRaises((ValueError, PlatformSdkError)):
                verify_program_receipt({**document, field: True}, authority, signatures, trust)
        for value in (0, 5, True):
            with self.assertRaises(ValueError): _execution({**document, "module_version": value}, "executed")
        corrupted = canonical[:-1] + bytes([canonical[-1] ^ 1])
        with self.assertRaises((ValueError, PlatformSdkError)):
            verify_program_receipt(_execution({**document, "receipt": corrupted.hex()}, "executed"), authority, signatures, trust)
