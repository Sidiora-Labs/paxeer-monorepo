import base64
import tempfile
import unittest
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

import test_commitment
from layerx_sdk.x402_http import (
    SellerMiddleware,
    BuyerMiddleware,
    FulfillmentStore,
    ConfiguredReceiptAuthority,
    encode_header,
    decode_header,
    validate_required,
)
from layerx_sdk.x402_rpc import PaymentRpc
from node_peer import node_peer, exchange


class HttpTests(unittest.TestCase):
    def setUp(self):
        self.fixture = test_commitment.CommitmentTests()
        self.fixture.setUp()
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        f = self.fixture
        self.node, produced = self.enterContext(node_peer())
        self.required = produced["required"]
        self.offer = self.required["accepts"][0]
        self.payload = decode_header(produced["header"])
        self.assertEqual(self.payload["payload"]["format"], "layerx-receipt-proof-v2")
        self.assertEqual(self.payload["payload"]["verificationLevel"], "receipted")
        self.assertEqual(self.payload["payload"]["signedReceipt"], test_commitment.FIXTURE["receipt"]["signedReceipt"])
        self.authority = ConfiguredReceiptAuthority(f.raw_transaction)
        self.seller = SellerMiddleware(
            self.required,
            f.receipt_keys,
            self.authority,
            FulfillmentStore(str(Path(self.directory.name) / "payments.sqlite")),
        )

    def test_real_receipt_release_retry_and_buyer_capture(self):
        status, headers, _ = self.seller.handle("payer", None, lambda: b"resource")
        self.assertEqual(status, 402)
        self.assertEqual(decode_header(headers["PAYMENT-REQUIRED"]), self.required)
        header = encode_header(self.payload)
        first = self.seller.handle("payer", header, lambda: b"resource")
        retry = self.seller.handle("payer", header, lambda: b"changed-resource")
        self.assertEqual(first, retry)
        self.assertEqual(first[0], 200)
        buyer = BuyerMiddleware(
            PaymentRpc(self.node),
            self.fixture.receipt_keys,
            self.authority,
            [("exact", "eip155:125")],
        )
        verified = buyer.capture_settlement(first[1]["PAYMENT-RESPONSE"], header)
        self.assertEqual(verified.canonical_bytes, self.fixture.wire)
        self.assertEqual(verified.receipt.activity_id.hex(), test_commitment.FIXTURE["txHash"][2:])
        self.assertEqual(verified.height, int(test_commitment.FIXTURE["rpcResult"]["height"], 16))
        captured = exchange(self.node + "capture", {"settlement": first[1]["PAYMENT-RESPONSE"]})
        self.assertEqual(captured["txHash"], test_commitment.FIXTURE["txHash"])
        self.assertEqual(captured["receiptDigest"], self.payload["payload"]["receiptDigest"])
        settlement = decode_header(first[1]["PAYMENT-RESPONSE"])
        settlement["payer"] = "01" * 32
        with self.assertRaises(ValueError):
            buyer.capture_settlement(encode_header(settlement), header)
        with self.assertRaises(ValueError):
            self.seller.handle("different-payer", header, lambda: b"resource")

    def test_concurrent_retry_persists_one_resource(self):
        header = encode_header(self.payload)
        with ThreadPoolExecutor(max_workers=4) as pool:
            responses = list(
                pool.map(
                    lambda _: self.seller.handle("payer", header, lambda: b"resource"),
                    range(8),
                )
            )
        self.assertTrue(all(response == responses[0] for response in responses))
        self.assertEqual(responses[0][0], 200)
        reopened = SellerMiddleware(self.required, self.fixture.receipt_keys, self.authority,
            FulfillmentStore(str(Path(self.directory.name) / "payments.sqlite")))
        self.assertEqual(reopened.handle("payer", header, lambda: self.fail("second release")), responses[0])

    def test_offer_commitment_uses_rung_ladder(self):
        for rung in ("receipted", "committed"):
            offer = self.offer | {"extra": {"layerx": {"commitment": rung}}}
            required = self.required | {"accepts": [offer]}
            self.assertEqual(validate_required(required), required)
        for retired in ("submitted", "executed", "batched", "finalised"):
            with self.assertRaisesRegex(ValueError, "invalid-payment-terms"):
                validate_required(self.required | {"accepts": [
                    self.offer | {"extra": {"layerx": {"commitment": retired}}}
                ]})

    def test_offer_mismatch_and_invalid_headers(self):
        for header in (
            "!",
            encode_header({}),
            base64.b64encode(b'{"a":1,"a":2}').decode(),
            base64.b64encode(b" " * 65537).decode(),
        ):
            with self.assertRaises((ValueError, KeyError)):
                self.seller.handle("payer", header, lambda: b"resource")
        for change in (
            {"amount": "1"},
            {"asset": "ab" * 32},
            {"payTo": "ab" * 32},
            {"extra": {"layerx": {"commitment": "executed"}}},
            {"extra": {"layerx": {"commitment": "batched"}}},
            {"extra": {"layerx": {"commitment": "finalised"}}},
        ):
            with self.assertRaises(ValueError):
                self.seller.handle(
                    "payer",
                    encode_header(self.payload | {"accepted": self.offer | change}),
                    lambda: b"resource",
                )
        with self.assertRaisesRegex(ValueError, "unsupported-payment"):
            validate_required(
                self.required | {"accepts": [self.offer | {"scheme": "unknown"}]}
            )
        for scheme in ("metered", "subscription"):
            terms = dict(self.offer["extra"]["layerx"])
            if scheme == "subscription":
                terms["windowSeconds"] = "60"
            offer = self.offer | {"scheme": scheme, "extra": {"layerx": terms}}
            required = self.required | {"accepts": [offer]}
            with self.subTest(scheme=scheme):
                self.assertEqual(validate_required(required), required)
        for level in ("executed", "batched", "finalised", "finalized"):
            payload = self.payload | {
                "payload": self.payload["payload"] | {"verificationLevel": level}
            }
            with self.assertRaises(ValueError):
                self.seller.handle("payer", encode_header(payload), lambda: b"resource")

    def test_real_http_402_retry(self):
        from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
        import threading

        seller = self.seller

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                status, headers, body = seller.handle(
                    "payer", self.headers.get("PAYMENT-SIGNATURE"), lambda: b"resource"
                )
                self.send_response(status)
                for name, value in headers.items():
                    self.send_header(name, value)
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, format, *args):
                return

        server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            buyer = BuyerMiddleware(
                PaymentRpc(self.node),
                self.fixture.receipt_keys,
                self.authority,
                [("exact", "eip155:125")],
            )
            response = buyer.fetch(
                f"http://127.0.0.1:{server.server_port}/paid",
                encode_header(self.payload),
            )
            self.assertEqual(response[0], 200)
            self.assertEqual(response[2], b"resource")
        finally:
            server.shutdown()
            server.server_close()
            thread.join()

    def test_python_buyer_captures_typescript_seller(self):
        buyer = BuyerMiddleware(PaymentRpc(self.node), self.fixture.receipt_keys,
            self.authority, [("exact", "eip155:125")])
        status, headers, body = buyer.fetch(self.node + "paid", encode_header(self.payload))
        self.assertEqual((status, body), (200, b"resource"))
        self.assertEqual(exchange(self.node + "metrics")["releases"], 1)
        exchange(self.node + "reopen", {})
        repeated = buyer.fetch(self.node + "paid", encode_header(self.payload))
        self.assertEqual(repeated, (status, headers, body))
        self.assertEqual(exchange(self.node + "metrics")["releases"], 1)

    def test_submitted_offer_never_releases(self):
        required = self.required | {"accepts": [self.offer | {"extra": {"layerx": {"commitment": "submitted"}}}]}
        with self.assertRaises(ValueError):
            SellerMiddleware(required, self.fixture.receipt_keys, self.authority,
                FulfillmentStore(str(Path(self.directory.name) / "submitted.sqlite")))
        self.assertEqual(exchange(self.node + "metrics")["releases"], 0)
