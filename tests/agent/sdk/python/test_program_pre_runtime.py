import json
import unittest
from copy import deepcopy
from dataclasses import replace
from hashlib import sha256
from pathlib import Path

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

from layerx_sdk.program_wire import bind_retained_program_call
from layerx_sdk.programs import _execution, _submission, _verify_execution_receipt
from layerx_sdk.verifier import (
    SequencerReceiptVerificationError,
    _RECEIPT_DOMAIN,
    _decode_sequencer_receipt,
    verify_sequencer_receipt,
)

from program_receipt_evidence import ProgramReceiptEvidence


class ProgramPreRuntimeTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        root = Path(__file__).resolve().parents[4]
        fixture = root / "tests/fixtures/programs/pre-runtime-refusal"
        cls.execution = json.loads((fixture / "execution.json").read_text())
        cls.signed = bytes.fromhex((fixture / "signed-activity.hex").read_text())
        cls.payload_hash, cls.abi, cls.key = bind_retained_program_call(
            cls.signed, cls.execution["activity_id"], cls.execution["program_id"], 3
        )
        cls.evidence = ProgramReceiptEvidence()

    def test_original_native_refusal_and_submission_recovery(self):
        parsed = _execution(self.execution, "refused")
        self.assertEqual(parsed["result_code"], -3)
        self.assertEqual(self.abi, 2)
        self.assertEqual(
            parsed["outcome"],
            {"kind": "refused", "failure": {"kind": "guest_refused", "code": -3}},
        )
        evidence = self.evidence
        self.assertEqual(evidence.verify(), evidence.verified)
        self.assertEqual(evidence.submission(), evidence.vector["receiptJson"])
        explicit = deepcopy(evidence.execution)
        del explicit["retained_signed_activity"]
        self.assertEqual(evidence.verify(explicit), evidence.verified)
        with self.assertRaisesRegex(ValueError, "retained signed transaction required"):
            _verify_execution_receipt(explicit, evidence.trust, expected_signed_activity=None)
        unknown = {"state": "unknown", "activity_id": self.execution["activity_id"], "idempotency_key": self.key}
        self.assertEqual(_submission(unknown, evidence.signatures, evidence.trust)["state"], "unknown")

    def test_call_identity_abi_idempotency_and_signature_bindings(self):
        evidence = self.evidence
        self.assertEqual(evidence.verify(), evidence.verified)
        for field in ("activity_id", "program_id"):
            expected = {
                "expected_activity": self.execution["activity_id"],
                "expected_program": self.execution["program_id"],
                "expected_protocol": 3,
            }
            expected["expected_activity" if field == "activity_id" else "expected_program"] = "ab" * 32
            with self.subTest(field=field), self.assertRaises(ValueError):
                bind_retained_program_call(self.signed, **expected)
            with self.subTest(receipt=field), self.assertRaises(ValueError):
                evidence.verify({**evidence.execution, field: "ab" * 32})
        for canonical in (self.signed[:-1], self.signed + b"\0"):
            with self.assertRaises(ValueError):
                bind_retained_program_call(canonical, self.execution["activity_id"], self.execution["program_id"], 3)
        payload_hash, abi, key = bind_retained_program_call(
            self.signed, self.execution["activity_id"], self.execution["program_id"], 3
        )
        self.assertEqual((payload_hash, abi, key), (self.payload_hash, 2, self.key))
        with self.assertRaisesRegex(ValueError, "retained program transaction mismatch"):
            evidence.verify(signed=evidence.signed + b"\0")
        changed = deepcopy(evidence.execution)
        changed["receipt"] = (evidence.encoded[:-1] + bytes([evidence.encoded[-1] ^ 1])).hex()
        with self.assertRaises(SequencerReceiptVerificationError) as raised:
            evidence.verify(changed)
        self.assertEqual(raised.exception.check, "bad_signature")

    def test_every_pre_runtime_receipt_invariant_is_enforced(self):
        evidence = self.evidence
        self.assertEqual(evidence.verify(), evidence.verified)
        for field, value in (
            ("activity_id", "00" * 32), ("program_id", "ab" * 32),
            ("batch_id", "00" * 32), ("global_sequence", "0"),
            ("module_version", 4), ("result_code", 1),
            ("state_root", "00" * 32), ("receipt_digest", "00" * 32),
            ("state", "refused"),
        ):
            with self.subTest(field=field), self.assertRaises(ValueError):
                evidence.verify({**evidence.execution, field: value})
        for field in ("batch_id", "asset", "previous_state_root", "resulting_state_root", "sequencer_public_key"):
            changed = deepcopy(evidence.execution)
            changed["authority"][field] = "ab" * 32
            with self.subTest(authority=field), self.assertRaises(ValueError):
                evidence.verify(changed)
        for changes, check in (
            ({"key_id": b"\x11" * 32}, "unpinned_key"),
            ({"first_height": evidence.verified.height + 1}, "key_out_of_range"),
            ({"last_height": evidence.verified.height - 1}, "key_out_of_range"),
            ({"public_key": b"\x11" * 32}, "bad_signature"),
        ):
            with self.subTest(key=changes), self.assertRaises(SequencerReceiptVerificationError) as raised:
                verify_sequencer_receipt(evidence.encoded, evidence.signed, (replace(evidence.key, **changes),))
            self.assertEqual(raised.exception.check, check)

    def test_original_layout_bounds_and_wrapper_refusal(self):
        evidence = self.evidence
        self.assertEqual(evidence.verify(), evidence.verified)
        for length in range(len(evidence.encoded)):
            with self.subTest(length=length), self.assertRaises(SequencerReceiptVerificationError):
                verify_sequencer_receipt(evidence.encoded[:length], evidence.signed, (evidence.key,))
        with self.assertRaises(SequencerReceiptVerificationError):
            verify_sequencer_receipt(evidence.encoded + b"\0", evidence.signed, (evidence.key,))
        for domain in (b"LXP/programs/terminal-applied-legs/v1\0", b"LXP/program-execution-with-occupancy/v1\0"):
            wrapped = domain + len(evidence.encoded).to_bytes(4, "big") + evidence.encoded + bytes(4)
            with self.subTest(wrapper=domain), self.assertRaises(SequencerReceiptVerificationError):
                verify_sequencer_receipt(wrapped, evidence.signed, (evidence.key,))

    def test_native_protocol_two_codec_preserves_historical_failure_layout(self):
        canonical = bytearray(self.signed)
        canonical[:2] = (2).to_bytes(2, "big")
        canonical[6:8] = (2).to_bytes(2, "big")
        activity = sha256(b"LXP/v1/activity-id\0" + canonical).digest()
        payload_hash, abi, key = bind_retained_program_call(
            bytes(canonical), activity.hex(), self.execution["program_id"], 2
        )
        self.assertEqual((payload_hash, abi, key), (self.payload_hash, 2, self.key))
        self.assertEqual(_execution(self.execution, "refused")["outcome"], self.execution["outcome"])
        with self.assertRaises(ValueError):
            bind_retained_program_call(bytes(canonical), activity.hex(), self.execution["program_id"], 3)

    def test_resigned_terminal_fields_cannot_rebind_refusal(self):
        evidence = self.evidence
        key = Ed25519PrivateKey.from_private_bytes(bytes([3]) * 32)
        pin = key.public_key().public_bytes(serialization.Encoding.Raw, serialization.PublicFormat.Raw)
        _, unsigned = _decode_sequencer_receipt(evidence.encoded)
        digest = sha256(_RECEIPT_DOMAIN + unsigned).digest()
        encoded = evidence.encoded[:-64] + key.sign(digest)
        trust = replace(evidence.trust, sequencer_public_key=pin, receipt_keys=(replace(evidence.key, public_key=pin),))
        execution = deepcopy(evidence.execution)
        execution["receipt"] = encoded.hex()
        execution["authority"]["sequencer_public_key"] = pin.hex()
        self.assertEqual(evidence.verify(execution, trust=trust).receipt_digest, digest)
        for field, value in (("state", "refused"), ("result_code", -3), ("state_root", "ab" * 32)):
            with self.subTest(field=field), self.assertRaisesRegex(ValueError, "receipt binding failed"):
                evidence.verify({**execution, field: value}, trust=trust)
        document = deepcopy(evidence.vector["receiptJson"])
        document["receipt"] = encoded.hex()
        document["signingKey"] = "0x" + pin.hex()
        self.assertEqual(
            _submission(document, evidence.signatures, trust,
                        program_id=execution["program_id"], activity_id=execution["activity_id"],
                        retained_signed_activity=evidence.signed.hex()),
            document,
        )
        document["returnData"] = "0x00"
        with self.assertRaisesRegex(ValueError, "binding failed"):
            _submission(document, evidence.signatures, trust,
                        retained_signed_activity=evidence.signed.hex())
