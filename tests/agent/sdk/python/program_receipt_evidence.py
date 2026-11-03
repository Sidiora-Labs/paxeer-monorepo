import importlib.util
import json
from copy import deepcopy
from pathlib import Path

from layerx_sdk.programs import (
    ProgramTrustContext,
    _submission,
    _verify_execution_receipt,
    decode_signed,
)
from layerx_sdk.verifier import ReceiptKey, verify_sequencer_receipt


class ProgramReceiptEvidence:
    def __init__(self):
        root = Path(__file__).resolve().parents[4]
        corpus = json.loads(
            (root / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text()
        )
        self.vector = next(
            case for case in corpus["cases"] if case["id"] == "V1-layerxprograms-call"
        )
        pin = self.vector["pinnedKey"]
        self.key = ReceiptKey(
            bytes.fromhex(pin["keyId"][2:]),
            bytes.fromhex(pin["publicKey"][2:]),
            pin["firstHeight"],
            pin["lastHeight"],
        )
        self.signed = bytes.fromhex(self.vector["rawTransaction"][2:])
        self.encoded = bytes.fromhex(self.vector["signedReceipt"][2:])
        self.verified = verify_sequencer_receipt(self.encoded, self.signed, (self.key,))
        call, _ = decode_signed(self.signed)
        self.trust = ProgramTrustContext(
            self.key.public_key, network_id=call.chain_id, receipt_keys=(self.key,)
        )
        spec = importlib.util.spec_from_file_location(
            "inventory_program_signatures",
            root / "sdk/integrations/fastapi/layerx_fastapi/signatures.py",
        )
        if spec is None or spec.loader is None:
            raise AssertionError("cannot load signature verifier")
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        self.signatures = module.LayerXSignatureVerifier()
        receipt = self.verified.receipt
        self.execution = {
            "state": "executed",
            "receipt": self.encoded.hex(),
            "retained_signed_activity": self.signed.hex(),
            "program_id": call.data[4:36].hex(),
            "activity_id": receipt.activity_id.hex(),
            "batch_id": receipt.batch_id.hex(),
            "global_sequence": str(receipt.global_sequence),
            "module_version": receipt.module_version,
            "result_code": receipt.result_code,
            "state_root": receipt.resulting_state_root.hex(),
            "receipt_digest": self.verified.receipt_digest.hex(),
            "authority": {
                "batch_id": receipt.batch_id.hex(),
                "asset": receipt.asset.hex(),
                "previous_state_root": receipt.previous_state_root.hex(),
                "resulting_state_root": receipt.resulting_state_root.hex(),
                "sequencer_public_key": self.key.public_key.hex(),
            },
        }

    def verify(self, execution=None, *, trust=None, signed=None):
        return _verify_execution_receipt(
            self.execution if execution is None else execution,
            self.trust if trust is None else trust,
            expected_signed_activity=self.signed if signed is None else signed,
        )

    def submission(self, document=None, **changes):
        return _submission(
            deepcopy(self.vector["receiptJson"]) if document is None else document,
            self.signatures,
            self.trust,
            **{
                "program_id": self.execution["program_id"],
                "activity_id": self.execution["activity_id"],
                "retained_signed_activity": self.signed.hex(),
                **changes,
            },
        )
