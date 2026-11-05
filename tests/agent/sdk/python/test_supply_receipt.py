from __future__ import annotations

import unittest

from test_receipt_fixture import _load_fixture, _verify_vector


class SupplyReceiptTests(unittest.TestCase):
    def test_sequencer_vectors_bind_supply_and_fee_fields(self):
        seen = set()
        for vector in _load_fixture()["cases"]:
            if vector["vector"] not in ("V1", "V2"):
                continue
            with self.subTest(vector=vector["id"]):
                receipt = _verify_vector(vector).receipt
                self.assertIsNone(receipt.total_units)
                self.assertEqual(receipt.fee_charged, int(vector["receiptJson"]["feeAmount"], 16))
                if vector["id"] == "V1-pax-transfer":
                    self.assertEqual(receipt.amount, 10**18)
                    self.assertEqual(receipt.from_balance_before - receipt.from_balance_after,
                                     receipt.amount + receipt.fee_charged)
                    self.assertEqual(receipt.to_balance_after - receipt.to_balance_before, receipt.amount)
                seen.add(vector["vector"])
        self.assertEqual(seen, {"V1", "V2"})
