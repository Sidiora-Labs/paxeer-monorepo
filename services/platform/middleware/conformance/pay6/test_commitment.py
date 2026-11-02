import json
import sys
from dataclasses import replace
from pathlib import Path
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[4] / "agent/sdk/python"))

from layerx_sdk.x402 import payment_commitment, verify_payment_receipt
from layerx_sdk.verifier import ReceiptKey, verify_sequencer_receipt
from layerx_sdk.production import PlatformSdkError

FIXTURE = json.loads(Path(__file__).with_name("evm-receipt.json").read_text())


class CommitmentTests(unittest.TestCase):
    def setUp(self):
        proof = FIXTURE["receipt"]
        self.wire = bytes.fromhex(proof["signedReceipt"][2:])
        self.raw_transaction = bytes.fromhex(proof["rawTransaction"][2:])
        key = proof["receiptKey"]
        self.receipt_keys = (
            ReceiptKey(
                key_id=bytes.fromhex(key["keyId"][2:]),
                public_key=bytes.fromhex(key["publicKey"][2:]),
                first_height=int(key["firstHeight"], 16),
                last_height=int(key["lastHeight"], 16),
            ),
        )
        self.offer = dict(
            amount=FIXTURE["offer"]["amount"],
            asset=FIXTURE["offer"]["asset"],
            pay_to=FIXTURE["offer"]["payTo"],
        )

    def verify(self, **changes):
        return verify_payment_receipt(
            self.wire, self.raw_transaction, self.receipt_keys,
            **(self.offer | changes),
        )

    def test_evm_receipt_key_and_transaction_binding(self):
        verified = verify_sequencer_receipt(
            self.wire, self.raw_transaction, self.receipt_keys
        )
        self.assertEqual(verified.receipt.activity_id.hex(), FIXTURE["txHash"][2:])
        self.assertEqual(verified.height, int(FIXTURE["rpcResult"]["height"], 16))
        expired = replace(self.receipt_keys[0], last_height=verified.height - 1)
        with self.assertRaises(PlatformSdkError):
            verify_sequencer_receipt(self.wire, self.raw_transaction, (expired,))
        with self.assertRaises(PlatformSdkError):
            verify_sequencer_receipt(self.wire, self.raw_transaction, ())
        with self.assertRaises(PlatformSdkError):
            verify_sequencer_receipt(
                self.wire, self.raw_transaction[:-1] + bytes([self.raw_transaction[-1] ^ 1]),
                self.receipt_keys,
            )

    def test_executed_payment_binding(self):
        verified = self.verify()
        payer = verified.receipt.from_account.hex()
        self.assertEqual(verified.canonical_bytes, self.wire)
        self.assertNotEqual(verified.receipt.amount, int(self.offer["amount"]))
        self.assertNotEqual(verified.receipt.asset.hex(), self.offer["asset"])
        self.assertNotEqual(verified.receipt.to_account.hex(), self.offer["pay_to"])
        self.verify(payer=payer)
        for change in (
            {"amount": "25001"},
            {"asset": "01" * 32},
            {"pay_to": "01" * 32},
        ):
            with self.assertRaises(PlatformSdkError):
                self.verify(**change)
        for invalid_payer in ("01" * 32, "00" * 32):
            with self.assertRaises(PlatformSdkError):
                self.verify(payer=invalid_payer)
        with self.assertRaises(PlatformSdkError):
            verify_payment_receipt(
                self.wire[:-1] + bytes([self.wire[-1] ^ 1]),
                self.raw_transaction,
                self.receipt_keys,
                **self.offer,
            )

    def test_no_commitment_downgrade(self):
        self.verify()
        for commitment in (
            "committed",
            "executed",
            "batched",
            "finalised",
            "finalized",
            "acknowledged",
            None,
            1,
        ):
            with self.assertRaises(PlatformSdkError):
                self.verify(commitment=commitment)
        self.assertEqual(payment_commitment(), "receipted")
        self.assertEqual(payment_commitment({}), "receipted")
        for rung in ("submitted", "receipted", "committed"):
            self.assertEqual(payment_commitment({"layerx": {"commitment": rung}}), rung)
        for rung in ("submitted", "receipted"):
            self.assertEqual(self.verify(commitment=rung).canonical_bytes, self.wire)
        for retired in ("executed", "batched", "finalised"):
            with self.assertRaisesRegex(PlatformSdkError, "rung ladder.*submitted.*receipted.*committed"):
                payment_commitment({"layerx": {"commitment": retired}})
        for value in (None, [], {}, "executed"):
            with self.assertRaises(PlatformSdkError):
                payment_commitment({"layerx": value})


if __name__ == "__main__":
    unittest.main()
