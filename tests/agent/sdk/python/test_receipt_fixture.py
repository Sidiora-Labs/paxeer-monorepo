from __future__ import annotations

import importlib.util
import json
import unittest
from hashlib import sha256
from pathlib import Path

from layerx_sdk import AuthorizedReceiptBatch, PlatformSdkError, ReceiptVerificationError, verify_receipt
from layerx_sdk.verifier import (
    ReceiptKey,
    SequencerReceiptVerificationError,
    decode_program_receipt_outcome,
    verify_sequencer_receipt,
)
from layerx_sdk.revert import RevertReasonError, raise_for_receipt_status

_PROGRAM_OUTCOME_V3 = "505247330100000000000100010000000700000001000000000000000b000000000000000c000000000000000d000000000000000e00000001000000000000000f0000000000000000000000000000000000000000000000000000000000000000000000000000000100000000000000020000000000000003000000000000000400000000000000050000000000000006000000000000000700000020000000000000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010000000201111111111111111111111111111111111111111111111111111111111111111000000202222222222222222222222222222222222222222222222222222222222222222000000200000000000000000000000000000000000000000000000000000000000000000"

_REPO_ROOT = Path(__file__).resolve().parents[4]
_SIGNATURES_PATH = _REPO_ROOT / "sdk/integrations/fastapi/layerx_fastapi/signatures.py"
_FIXTURE_PATH = _REPO_ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
_NATIVE = _REPO_ROOT / "tests/fixtures/programs/emulator-response-code"


def _signature_verifier_class() -> type:
    spec = importlib.util.spec_from_file_location("layerx_fastapi_signatures", _SIGNATURES_PATH)
    if spec is None or spec.loader is None:
        raise AssertionError(f"cannot load {_SIGNATURES_PATH}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module.LayerXSignatureVerifier


LayerXSignatureVerifier = _signature_verifier_class()


def _load_fixture() -> dict:
    return json.loads(_FIXTURE_PATH.read_text(encoding="utf-8"))


def _hex(value: str) -> bytes:
    return bytes.fromhex(value.removeprefix("0x"))


def _pinned_key(vector: dict) -> ReceiptKey:
    key = vector["pinnedKey"]
    return ReceiptKey(_hex(key["keyId"]), _hex(key["publicKey"]), key["firstHeight"], key["lastHeight"])


def _verify_vector(vector: dict):
    return verify_sequencer_receipt(_hex(vector["signedReceipt"]), _hex(vector["rawTransaction"]), [_pinned_key(vector)])


def _sequence_hash(vector: dict, receipt) -> bytes:
    return sha256(b"PAXEER/v1/sequence\0" + _hex(vector["prevSeqHash"]) + receipt.global_sequence.to_bytes(8, "big") + receipt.activity_id).digest()


def _log_bodies(vector: dict) -> tuple[bytes, ...]:
    return tuple(_hex(log["address"]) + bytes([len(log["topics"])]) + b"".join(_hex(topic) for topic in log["topics"]) + sha256(_hex(log["data"])).digest() for log in vector["receiptJson"]["logs"])


def _native_execution() -> tuple[dict, AuthorizedReceiptBatch]:
    document = json.loads((_NATIVE / "execution.json").read_text(encoding="utf-8"))
    batch = document["authority"]
    return document, AuthorizedReceiptBatch(*(bytes.fromhex(batch[field]) for field in ("batch_id", "asset", "previous_state_root", "resulting_state_root", "sequencer_public_key")))


class ReceiptFixtureTest(unittest.TestCase):
    def test_explicit_protocol_three(self) -> None:
        document, authority = _native_execution()
        canonical = bytes.fromhex(document["receipt"])
        with self.assertRaises(ReceiptVerificationError):
            verify_receipt(canonical, authority, LayerXSignatureVerifier())
        verified = verify_receipt(canonical, authority, LayerXSignatureVerifier(), protocol_version=3)
        self.assertEqual(verified.receipt.protocol_version, 3)
        self.assertEqual(verified.receipt_digest.hex(), document["receipt_digest"])
        for damaged, protocol in ((canonical[:-1] + bytes([canonical[-1] ^ 1]), 3), (canonical, 4)):
            with self.assertRaises(ReceiptVerificationError):
                verify_receipt(damaged, authority, LayerXSignatureVerifier(), protocol_version=protocol)

    def test_protocol_three_refusal_taxonomy(self) -> None:
        expected = {
            "V3-key-not-pinned": "unpinned_key",
            "V3-height-outside-key-range": "key_out_of_range",
            "V3-activity-id-not-raw-hash": "hash_mismatch",
            "V3-non-canonical-bytes": "bad_signature",
        }
        seen = set()
        for vector in _load_fixture()["cases"]:
            if vector["id"] in expected:
                with self.subTest(vector=vector["id"]):
                    self.assertEqual(vector["vector"], "V3")
                    with self.assertRaises(SequencerReceiptVerificationError) as refused:
                        _verify_vector(vector)
                    self.assertEqual(refused.exception.check, expected[vector["id"]])
                    seen.add(vector["id"])
        self.assertEqual(seen, set(expected))

    def test_program_outcome_v3_vector(self) -> None:
        outcome = decode_program_receipt_outcome(bytes.fromhex(_PROGRAM_OUTCOME_V3), 1)
        self.assertEqual(outcome.encoding_version, 3)
        self.assertEqual(outcome.abi_version, 1)
        self.assertEqual(outcome.fee_units, 16)
        self.assertEqual(outcome.call_graph_root, bytes.fromhex("11" * 32))
        self.assertEqual(outcome.terminal_payload_root, bytes.fromhex("22" * 32))

    def test_core_fixture_receipt_verifies_positively(self) -> None:
        fixture = _load_fixture()
        seen = set()
        for vector in fixture["cases"]:
            if vector["vector"] not in ("V1", "V2"):
                continue
            with self.subTest(vector=vector["id"]):
                verified = _verify_vector(vector)
                view = vector["receiptJson"]
                receipt = verified.receipt
                self.assertEqual(verified.level, "sequencer-signed")
                self.assertEqual(vector["expected"], {"rung": "receipted", "verifier": "accepts"})
                self.assertEqual(verified.canonical_bytes, _hex(vector["signedReceipt"]))
                self.assertEqual(verified.receipt_digest, _hex(vector["receiptDigest"]))
                self.assertEqual(verified.height, int(view["height"], 16))
                self.assertEqual((receipt.protocol_version, receipt.module_id, receipt.module_version), (2, fixture["moduleId"], 1))
                self.assertEqual(receipt.global_sequence, int(view["seq"], 16))
                self.assertEqual(receipt.timestamp, int(view["timestamp"], 16))
                self.assertEqual(receipt.result_code, 0 if vector["vector"] == "V1" else 1)
                self.assertEqual(receipt.fee_charged, int(view["feeAmount"], 16))
                self.assertEqual(receipt.activity_id, _hex(view["txHash"]))
                self.assertEqual(receipt.from_account, _hex(view["from"]).rjust(32, b"\0"))
                self.assertEqual(receipt.to_account, _hex(view["to"]).rjust(32, b"\0"))
                self.assertEqual(receipt.previous_state_root, _hex(vector["prevSeqHash"]))
                self.assertEqual(receipt.resulting_state_root, _sequence_hash(vector, receipt))
                self.assertEqual(receipt.activity_root, sha256(_hex(view["returnData"])).digest())
                self.assertEqual(tuple(effect.body for effect in receipt.effects[1:]), _log_bodies(vector))
                self.assertIsNone(receipt.program_outcome)
                if vector["vector"] == "V2":
                    with self.assertRaises(RevertReasonError) as refused:
                        raise_for_receipt_status(view)
                    self.assertEqual(refused.exception.code, vector["lxpCode"])
                seen.add(vector["vector"])
        self.assertEqual(seen, {"V1", "V2"})

    def test_core_fixture_receipt_byte_flip_fails(self) -> None:
        vectors = [vector for vector in _load_fixture()["cases"] if vector["vector"] in ("V1", "V2")]
        self.assertTrue(vectors)
        for vector in vectors:
            canonical = _hex(vector["signedReceipt"])
            with self.subTest(vector=vector["id"]), self.assertRaises(PlatformSdkError):
                verify_sequencer_receipt(canonical[:-1] + bytes([canonical[-1] ^ 1]), _hex(vector["rawTransaction"]), [_pinned_key(vector)])

    def test_core_programs_fixture_preserves_optional_outcome(self) -> None:
        document, authority = _native_execution()
        verified = verify_receipt(bytes.fromhex(document["receipt"]), authority, LayerXSignatureVerifier(), protocol_version=3)
        outcome = verified.receipt.program_outcome
        self.assertIsNotNone(outcome)
        assert outcome is not None
        self.assertEqual((outcome.encoding_version, outcome.runtime_version, outcome.abi_version), (4, 1, 2))
        self.assertEqual(outcome.fee_units, int(document["usage"]["fee_units"]))
        self.assertEqual(outcome.cpu_fuel, int(document["usage"]["cpu_fuel"]))
        self.assertEqual(outcome.terminal_payload_root, sha256(bytes.fromhex(document["terminal_payload"])).digest())
        self.assertEqual(outcome.call_graph_root, sha256(bytes.fromhex(document["call_graph"])).digest())

    def test_core_refusal_vectors_expose_shared_taxonomy(self) -> None:
        seen = set()
        for vector in _load_fixture()["cases"]:
            if vector["id"] == "V3-seq-hash-mismatch":
                receipt = _verify_vector(vector).receipt
                self.assertEqual(vector["expected"]["refusedAt"], "R6")
                self.assertNotEqual(receipt.previous_state_root, _hex(vector["prevSeqHash"]))
                self.assertNotEqual(receipt.resulting_state_root, _sequence_hash(vector, receipt))
                seen.add(vector["id"])
            elif vector["id"] == "V3-tampered-log":
                receipt = _verify_vector(vector).receipt
                self.assertEqual(vector["expected"]["refusedAt"], "R5")
                self.assertNotEqual(tuple(effect.body for effect in receipt.effects[1:]), _log_bodies(vector))
                seen.add(vector["id"])
        self.assertEqual(seen, {"V3-seq-hash-mismatch", "V3-tampered-log"})


if __name__ == "__main__":
    unittest.main()
