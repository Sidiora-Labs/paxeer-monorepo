import json
import sys
import unittest
from dataclasses import replace
from hashlib import sha256
from pathlib import Path

ROOT = Path(__file__).resolve().parents[4]
sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))

from layerx_sdk.program_wire import (  # noqa: E402
    _ACCOUNT_BOUND_SET, _AUTHORITY, _Reader, _historical_activity_envelope, _verify_authorization_root,
    bind_historical_program_call, decode_historical_program_call,
)
from layerx_sdk.native_program_call import decode_native_program_call  # noqa: E402
from layerx_sdk.programs import NativeProgramRequest  # noqa: E402


class ProgramAccountBoundTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        root = Path(__file__).resolve().parents[4]
        fixture = root / "tests/fixtures/programs/account-bound-call"
        cls.execution = json.loads((fixture / "execution.json").read_text())
        cls.signed = bytes.fromhex((fixture / "signed-activity.hex").read_text())
        envelope = _historical_activity_envelope(cls.signed)
        cls.payload_hash = envelope.payload_hash
        cls.payload = envelope.payload
        cls.request = NativeProgramRequest(
            decode_native_program_call(cls.payload), envelope.fee_limit, cls.signed)

    def test_real_native_call_verifies_with_principal_account_binding(self):
        """Historical Activity codec coverage; chain identity is tested separately."""
        self.assertEqual(sha256(b"LXP/v1/activity-id\0" + self.signed).hexdigest(), self.execution["activity_id"])
        binding = decode_historical_program_call(self.request, self.execution["idempotency_key"])
        self.assertEqual(binding.activity_id, self.execution["activity_id"])
        self.assertEqual(binding.canonical_bytes, self.signed)
        self.assertEqual(bind_historical_program_call(
            self.signed, binding.activity_id, self.execution["program_id"], 3),
            (self.payload_hash, self.execution["guest_abi_version"], self.execution["idempotency_key"]))
        self.assertEqual(self.payload_hash, sha256(b"LXP/v1/payload-hash\0" + self.payload).digest())
        self.assertEqual(self.request.program_id, self.execution["program_id"])

    def test_current_transaction_binds_hash_sender_and_precompile(self):
        from program_receipt_evidence import ProgramReceiptEvidence
        from layerx_sdk.account_derivation import keccak256
        from layerx_sdk.evm_transaction import activity_id, decode_signed

        evidence = ProgramReceiptEvidence()
        transaction, signature = decode_signed(evidence.signed)
        verified = evidence.verify()
        receipt = verified.receipt
        self.assertEqual(transaction.to.hex(), "000000000000000000000000000000000000101d")
        self.assertEqual(activity_id(evidence.signed), keccak256(evidence.signed))
        self.assertEqual(receipt.activity_id, keccak256(evidence.signed))
        self.assertEqual(receipt.from_account, bytes(12) + bytes.fromhex(transaction.recover_signer(signature)[2:]))
        self.assertEqual(receipt.to_account, bytes(12) + transaction.to)
        self.assertEqual(receipt.from_sequence, transaction.nonce)
        self.assertEqual(receipt.module_id, 12)
        self.assertEqual(receipt.operation, 2)
        self.assertEqual(receipt.result_code, 0)
        self.assertEqual(evidence.submission(), evidence.vector["receiptJson"])

    def test_account_bound_authority_rejects_changed_names_and_shape(self):
        terminal = bytes.fromhex(self.execution["terminal_payload"])
        domain = b"LXP/programs/terminal-applied-legs/v1\0"
        wrapper = _Reader(terminal)
        self.assertEqual(wrapper.fixed(len(domain)), domain)
        detail = wrapper.sized_u32(1_048_576)
        wrapper.sized_u32(256 * 115)
        wrapper.end()
        self.assertTrue(detail.startswith(_AUTHORITY))
        wrapper = _Reader(detail[len(_AUTHORITY):])
        wrapper.sized_u32(1_048_576)
        authorization = wrapper.sized_u32(1_048_576)
        root = wrapper.fixed(32)
        wrapper.end()
        self.assertTrue(authorization.startswith(_ACCOUNT_BOUND_SET))
        wrapped = _Reader(authorization[len(_ACCOUNT_BOUND_SET):])
        original = wrapped.sized_u32(1_048_576)
        name = wrapped.fixed(wrapped.u16())
        wrapped.end()
        self.assertTrue(name.startswith(b"agent:did:layerx:"))
        _verify_authorization_root(authorization, root, require_v2=True)
        prefix = _ACCOUNT_BOUND_SET + len(original).to_bytes(4, "big") + original
        changed_owner = name.replace(b"did:layerx:", b"did:layerz:", 1)
        cases = [
            authorization + b"\0", authorization[:-1], original,
            _ACCOUNT_BOUND_SET + len(authorization).to_bytes(4, "big") + authorization,
            prefix + b"\0\0",
            prefix + len(name).to_bytes(2, "big") + name.upper(),
            prefix + len(changed_owner).to_bytes(2, "big") + changed_owner,
            authorization + len(name).to_bytes(2, "big") + name,
            _ACCOUNT_BOUND_SET + len(original).to_bytes(4, "big") + original.replace(b"transfer-set/v2\0", b"transfer-set/v1\0", 1) + len(name).to_bytes(2, "big") + name,
        ]
        for index, changed in enumerate(cases):
            with self.subTest(index=index), self.assertRaises(ValueError):
                _verify_authorization_root(changed, root, require_v2=True)
        with self.assertRaises(ValueError):
            _verify_authorization_root(authorization, bytes(32), require_v2=True)

    def test_signed_native_call_commitments_remain_bound(self):
        for activity, program, protocol in (
            ("00" * 32, self.execution["program_id"], 3),
            (self.execution["activity_id"], "00" * 32, 3),
            (self.execution["activity_id"], self.execution["program_id"], 2),
        ):
            with self.subTest(activity=activity, program=program, protocol=protocol), self.assertRaises(ValueError):
                bind_historical_program_call(self.signed, activity, program, protocol)
        native = self.request.native_call
        for changed in (
            replace(self.request, fee_limit=self.request.fee_limit + 1),
            replace(self.request, native_call=replace(native, calldata=native.calldata + b"\0")),
            replace(self.request, native_call=replace(native, guest_abi=5)),
            replace(self.request, signed_activity=self.signed[:-1]),
            replace(self.request, signed_activity=self.signed + b"\0"),
        ):
            with self.subTest(request=changed), self.assertRaises(ValueError):
                decode_historical_program_call(changed)
        with self.assertRaisesRegex(ValueError, "signed activity idempotency"):
            decode_historical_program_call(self.request, "00" * 32)
