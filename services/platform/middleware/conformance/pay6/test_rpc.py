import unittest
from test_commitment import CommitmentTests, FIXTURE
from layerx_sdk.x402_rpc import verify_rpc_payment, PaymentRpc
from layerx_sdk.verifier import verify_sequencer_receipt


class RpcTests(unittest.TestCase):
    def test_receipt_binding_and_pending(self):
        fixture = CommitmentTests()
        fixture.setUp()
        verified = verify_sequencer_receipt(
            fixture.wire, fixture.raw_transaction, fixture.receipt_keys
        )
        transaction_hash = FIXTURE["txHash"]
        payer = FIXTURE["payer"]
        self.assertEqual(verified.receipt.activity_id.hex(), transaction_hash[2:])
        result = FIXTURE["rpcResult"]

        def verify(body, expected_payer=payer):
            return verify_rpc_payment(
                body,
                transaction_hash,
                expected_payer,
                receipt_keys=fixture.receipt_keys,
                raw_transaction=fixture.raw_transaction,
                **fixture.offer,
            )

        self.assertEqual(verify(result), verified)
        self.assertIsNone(verify({
            "txHash": transaction_hash, "rung": "submitted", "nonce": "0x0"
        }))
        for change in (
            {"txHash": "0x" + "ab" * 32},
            {"rung": "acknowledged"},
            {"rung": "batched"},
            {"rung": "committed"},
            {"receipt": "0x00"},
            {"height": "0x8"},
            {"keyId": "0x" + "ab" * 32},
        ):
            with self.assertRaises(ValueError):
                verify(result | change)
        with self.assertRaises(ValueError):
            verify(result, "0x" + "ab" * 32)
        with self.assertRaises(ValueError):
            verify({"txHash": transaction_hash})

    def test_endpoints(self):
        for endpoint in (
            "http://example.com/rpc",
            "https://user:password@example.com/rpc",
            "https://example.com/rpc?token=1",
            "https://example.com/",
        ):
            with self.assertRaises(ValueError):
                PaymentRpc(endpoint)
        with self.assertRaises(ValueError):
            PaymentRpc("http://127.0.0.1:1/rpc").send("00")
