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

        def verify(body, expected_payer=payer, commitment="receipted"):
            return verify_rpc_payment(
                body,
                transaction_hash,
                expected_payer,
                receipt_keys=fixture.receipt_keys,
                raw_transaction=fixture.raw_transaction,
                commitment=commitment,
                **fixture.offer,
            )

        expected = {
            "txHash": transaction_hash,
            "rung": "receipted",
            "height": verified.height,
            "keyId": FIXTURE["rpcResult"]["keyId"],
        }
        self.assertEqual(verify(result), expected)
        self.assertEqual(verify({
            "txHash": transaction_hash, "rung": "submitted", "nonce": "0x0"
        }), {"txHash": transaction_hash, "rung": "pending", "height": None, "keyId": None})
        committed = result | {"rung": "committed", "blockHash": "0x" + "12" * 32}
        self.assertEqual(verify(committed), expected | {"rung": "committed"})
        self.assertEqual(verify(committed, commitment="committed"), expected | {"rung": "committed"})
        with self.assertRaises(ValueError):
            verify(result, commitment="committed")
        for change in (
            {"txHash": "0x" + "ab" * 32},
            {"rung": "acknowledged"},
            {"rung": "batched"},
            {"rung": "executed"},
            {"rung": "finalised"},
            {"rung": "committed"},
            {"receipt": "0x00"},
            {"height": "0x8"},
            {"height": "0x07"},
            {"height": "0x10000000000000000"},
            {"status": "0x2"},
            {"keyId": "0x" + "ab" * 32},
        ):
            with self.assertRaises(ValueError):
                verify(result | change)
        with self.assertRaises(ValueError):
            verify(result, "0x" + "ab" * 32)
        with self.assertRaises(ValueError):
            verify({"txHash": transaction_hash})
        with self.assertRaises(ValueError):
            verify({"activity_id": transaction_hash[2:], "state": "pending"})
        with self.assertRaises(ValueError):
            verify({"txHash": transaction_hash, "rung": "submitted", "nonce": "0x00"})
        for retired in ("executed", "batched", "finalised"):
            with self.assertRaisesRegex(Exception, "rung ladder"):
                verify(result, commitment=retired)

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
