import json
import hashlib
import unittest
from dataclasses import replace
from pathlib import Path

from layerx_sdk.ai_market import _decode_programs_call
from layerx_sdk.account_derivation import keccak256

from layerx_sdk.native_program_call import (
    decode_native_program_call,
    encode_native_program_call,
)
from layerx_sdk.program_wire import decode_historical_program_call, decode_signed_program_call
from layerx_sdk.programs import NativeProgramRequest, _wire
from layerx_sdk.ai_market import _encode_programs_call
from layerx_sdk.evm_transaction import Type2Transaction, activity_id, decode_signed


class NativeProgramBindingTest(unittest.TestCase):
    def test_real_signed_native_binding_and_mismatches(self):
        fixture = json.loads((Path(__file__).resolve().parents[4] / "sdk/platform/conformance/fixtures/native-program-call-v4.json").read_text())
        native = decode_native_program_call(bytes.fromhex(fixture["payload_hex"]))
        fee = int(fixture["fee_limit"])
        self.assertEqual(_encode_programs_call(native, fee).hex(), fixture["evm"]["calldata"][2:])
        transaction = Type2Transaction(chain_id=fixture["evm"]["chainId"], nonce=9,
            max_priority_fee_per_gas=1_000_000_000, max_fee_per_gas=3_000_000_000,
            to=bytes.fromhex(fixture["evm"]["to"][2:]), gas_limit=2_000_000,
            value=0,
            data=bytes.fromhex(fixture["evm"]["calldata"][2:]))
        fixtures = Path(__file__).resolve().parents[4] / "sdk/platform/conformance/fixtures"
        recording = json.loads((fixtures / "native-program-binding-v4.json").read_text())
        self.assertEqual(recording["provenance"]["sourceSha256"],
            hashlib.sha256((fixtures / "native-program-call-v4.json").read_bytes()).hexdigest())
        self.assertEqual(recording["provenance"]["signer"],
            "tests/agent/sdk/python/test_evm_transaction.py:signed_raw")
        raw = bytes.fromhex(recording["raw_transaction"])
        request = NativeProgramRequest(native, fee, raw)
        binding = decode_signed_program_call(request)
        self.assertEqual(binding.activity_id, activity_id(raw).hex())
        self.assertEqual(binding.idempotency_key, binding.activity_id)
        self.assertEqual(binding.canonical_bytes, raw)
        self.assertEqual(decode_signed(raw)[0], transaction)
        historical = replace(request, signed_activity=bytes.fromhex(fixture["signed_activity_hex"]))
        self.assertEqual(decode_historical_program_call(historical).activity_id, fixture["activity_id_hex"])
        with self.assertRaises(ValueError):
            decode_signed_program_call(historical)
        self.assertEqual(_wire(request)["payload_encoding"], "native-v1")
        program_id = bytearray(native.program_id)
        index = next(i for i, value in enumerate(program_id) if value)
        program_id[index] = 1 if program_id[index] != 1 else 2
        mutate_bytes = lambda value: bytes([value[0] ^ 1]) + value[1:] if value else b"\1"
        for field, value in {
            "program_id": bytes(program_id),
            "guest_abi": 2 if native.guest_abi == 1 else 1,
            "entrypoint": ("a" if native.entrypoint[0] != "a" else "b") + native.entrypoint[1:],
            "calldata": mutate_bytes(native.calldata),
            "capabilities": mutate_bytes(native.capabilities),
            "access_declaration": mutate_bytes(native.access_declaration),
            "response_capacity": (native.response_capacity + 1) % 1_048_577,
            "resources": (native.resources[0] ^ 1,) + native.resources[1:],
        }.items():
            with self.subTest(field=field):
                self.assertNotEqual(value, getattr(native, field))
                changed = replace(native, **{field: value})
                encode_native_program_call(changed)
                with self.assertRaises(ValueError):
                    decode_signed_program_call(replace(request, native_call=changed))
        with self.assertRaises(ValueError):
            decode_signed_program_call(replace(request, fee_limit=request.fee_limit - 1))

        corpus = json.loads((Path(__file__).resolve().parents[4] / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text())
        self.assertEqual(corpus["provenance"]["generator"], "rpc/sequencer_receipt_vectors_test.go")
        fixture = next(case for case in corpus["cases"] if case["id"] == "V1-layerxprograms-call")
        raw = bytes.fromhex(fixture["rawTransaction"][2:])
        transaction, signature = decode_signed(raw)
        native, fee_limit = _decode_programs_call(transaction.data)
        self.assertEqual(transaction.chain_id, 125)
        self.assertEqual(transaction.recover_signer(signature).lower(), fixture["receiptJson"]["from"].lower())
        self.assertEqual(decode_native_program_call(encode_native_program_call(native)), native)
        request = NativeProgramRequest(native, fee_limit, raw)
        self.assertEqual(decode_signed_program_call(request).activity_id, keccak256(raw).hex())
        self.assertEqual(_wire(request)["payload_encoding"], "native-v1")
