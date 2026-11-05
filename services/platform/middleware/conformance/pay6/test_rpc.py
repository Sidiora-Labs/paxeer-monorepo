import unittest
from test_commitment import CommitmentTests, FIXTURE
from layerx_sdk.x402_rpc import verify_rpc_payment, PaymentRpc
from layerx_sdk.verifier import verify_sequencer_receipt
from node_peer import node_peer
from recorded_rpc import recorded_rpc, result as rpc_result


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
        for candidate in (result | {"rung": "committed"}, committed):
            with self.assertRaises(ValueError):
                verify(candidate)
            with self.assertRaises(ValueError):
                verify(candidate, commitment="committed")
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
            "http://example.com/",
            "https://user:password@example.com/",
            "https://example.com/?token=1",
            "https://example.com/#fragment",
            "https://example.com/rpc",
        ):
            with self.assertRaises(ValueError):
                PaymentRpc(endpoint)
        self.assertIsInstance(PaymentRpc("https://example.com/"), PaymentRpc)
        for raw in ("00", "0x02", "0x" + "00" * 524289):
            with self.assertRaises(ValueError):
                PaymentRpc("http://127.0.0.1:1/").send(raw)

    def test_real_node_root_send_and_receipt(self):
        with node_peer() as (endpoint, produced):
            rpc = PaymentRpc(endpoint)
            self.assertEqual(rpc.send(produced["proof"]["rawTransaction"]), produced["txHash"])
            self.assertEqual(rpc.receipt(produced["txHash"]), produced["result"])

    def test_hosted_gateway_has_its_own_origin(self):
        from layerx_sdk.x402_rpc import HostedGatewayRpc

        key = FIXTURE["receipt"]["receiptKey"]
        answer = {"keys": [{**key, "firstHeight": int(key["firstHeight"], 16), "lastHeight": int(key["lastHeight"], 16)}]}
        with recorded_rpc([rpc_result(answer)], path="/rpc", token="Bearer gateway") as (gateway, hosted_requests):
            with recorded_rpc([rpc_result(FIXTURE["txHash"])], token="Bearer node") as (node, node_requests):
                self.assertNotEqual(gateway, node)
                hosted = HostedGatewayRpc(gateway, {"Authorization": "Bearer gateway"})
                rpc = PaymentRpc(node, {"Authorization": "Bearer node"})
                self.assertEqual(hosted.call("px_getReceiptKeys", []), answer)
                self.assertEqual(rpc.send(FIXTURE["receipt"]["rawTransaction"]), FIXTURE["txHash"])
                with self.assertRaises(ValueError):
                    rpc.call("px_getReceiptKeys", [])
                with self.assertRaises(ValueError):
                    hosted.call("eth_sendRawTransaction", [])
        self.assertEqual([entry["path"] for entry in hosted_requests], ["/rpc"])
        self.assertEqual([entry["path"] for entry in node_requests], ["/"])

    def test_redirect_body_and_envelope_bounds(self):
        answers = [
            (307, {"Location": "https://other.example/"}, b""),
            (200, {}, b"x" * 8388609),
            (200, {}, b'{"jsonrpc":"2.0","id":999,"result":null}'),
            rpc_result({"rung": "submitted", "txHash": FIXTURE["txHash"], "nonce": "0x00"}),
            rpc_result({"rung": "receipted", "txHash": "0x" + "ff" * 32}),
        ]
        for answer in answers:
            with self.subTest(answer=answer[:2]), recorded_rpc([answer]) as (endpoint, requests):
                with self.assertRaises(ValueError):
                    PaymentRpc(endpoint).receipt(FIXTURE["txHash"])
                self.assertEqual(len(requests), 1)
