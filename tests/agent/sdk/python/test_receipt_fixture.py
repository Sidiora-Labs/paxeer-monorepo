from __future__ import annotations

import json
import sys
import unittest
from hashlib import sha256
from pathlib import Path

ROOT = Path(__file__).resolve().parents[4]
sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))

from layerx_sdk import PlatformSdkError  # noqa: E402
from layerx_sdk.verifier import (  # noqa: E402
    ReceiptKey,
    SequencerReceiptVerificationError,
    verify_sequencer_receipt,
)
from layerx_sdk.revert import RevertReasonError, raise_for_receipt_status  # noqa: E402

_FIXTURE_PATH = ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"


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


class ReceiptFixtureTest(unittest.TestCase):
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
                self.assertFalse(hasattr(receipt, "program_outcome"))
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
