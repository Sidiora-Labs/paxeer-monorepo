import base64
import copy
import json
import tempfile
import unittest
from concurrent.futures import ThreadPoolExecutor
from dataclasses import replace
from pathlib import Path
from urllib.error import HTTPError
from urllib.request import Request, urlopen

import test_commitment
from layerx_sdk.committed import CommitTrust
from layerx_sdk.production import PlatformSdkError
from layerx_sdk.verifier import ReceiptKey
from layerx_sdk.x402_http import (
    SellerMiddleware,
    BuyerMiddleware,
    FulfillmentStore,
    ConfiguredReceiptAuthority,
    encode_header,
    decode_header,
    _parse_proof,
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

    def test_typescript_settlement_metadata_round_trip(self):
        fixtures = exchange(self.node + "fixture")
        committed = fixtures["committed"]
        root = Path(__file__).resolve().parents[5]
        source = json.loads((root / "daemon/committed-fixture/vectors/vector-v4-committed.json").read_text())["settlement"]
        vector = json.loads((root / "sdk/platform/conformance/fixtures/vector-v4-committed.json").read_text())["positive"]
        self.assertEqual(committed["proof"]["rawTransaction"], source["raw_transaction"])
        self.assertEqual(committed["proof"]["signedReceipt"], source["receipt"]["receipt"])
        pin = vector["pinnedKey"]
        committed_keys = (ReceiptKey(
            bytes.fromhex(pin["keyId"][2:]), bytes.fromhex(pin["publicKey"][2:]),
            pin["firstHeight"], pin["lastHeight"],
        ),)
        validators = tuple({
            "address": row["address"],
            "public_key": base64.b64decode(row["pub_key"]["value"]).hex(),
            "voting_power": int(row["voting_power"]),
        } for row in source["validators"])
        commit = copy.deepcopy(source["commit"])
        commit["height"] = int(commit["height"])
        for signature in commit["signatures"]:
            signature["signature"] = (
                base64.b64decode(signature["signature"]).hex()
                if signature["signature"] is not None else ""
            )
        transaction_proof = source["transaction_proof"]
        inclusion = vector["receiptInclusion"]
        evidence = {
            "header": source["header"], "commit": commit,
            "proof": {
                "data": base64.b64decode(transaction_proof["data"]).hex(),
                "total": int(transaction_proof["proof"]["total"]),
                "index": int(transaction_proof["proof"]["index"]),
                "aunts": [base64.b64decode(value).hex() for value in transaction_proof["proof"]["aunts"]],
                "execution_header": source["execution_header"],
                "execution_commit": source["execution_commit"],
                "sequence_batch_proof": source["sequence_batch_proof"],
                "result": source["exec_tx_result"],
                "result_proof": source["exec_tx_result_proof"],
            },
        }
        trust = CommitTrust(source["header"]["chain_id"], commit["height"], validators)
        raw = bytes.fromhex(committed["proof"]["rawTransaction"][2:])
        altered_transaction = copy.deepcopy(evidence)
        envelope = bytearray.fromhex(altered_transaction["proof"]["data"])
        signature = raw[-32:]
        self.assertEqual(envelope.count(signature), 1)
        envelope[envelope.index(signature)] ^= 1
        altered_transaction["proof"]["data"] = envelope.hex()
        with self.assertRaisesRegex(ValueError, "transaction inclusion identity mismatch"):
            trust.verify(raw, int(source["execution_header"]["height"]), altered_transaction)
        invalid_inclusion = copy.deepcopy(evidence)
        invalid_inclusion["proof"]["aunts"][0] = "00" * 32
        with self.assertRaisesRegex(ValueError, "transaction inclusion proof mismatch"):
            trust.verify(raw, int(source["execution_header"]["height"]), invalid_inclusion)
        for member, replacement in (("data_hash", "00" * 32), ("height", "9"),
                                    ("chain_id", "wrong-chain")):
            invalid_execution = copy.deepcopy(evidence)
            invalid_execution["proof"]["execution_header"][member] = replacement
            with self.subTest(execution_header=member), self.assertRaisesRegex(ValueError, "execution header binding mismatch"):
                trust.verify(raw, int(source["execution_header"]["height"]), invalid_execution)
        authority = ConfiguredReceiptAuthority(
            raw, rpc=PaymentRpc(self.node), receipt_keys=committed_keys,
            commit_trust=trust, committed_evidence=evidence,
        )
        supported = [("exact", "eip155:125")]
        receipted_buyer = BuyerMiddleware(
            PaymentRpc(self.node), self.fixture.receipt_keys, self.authority, supported,
        )
        committed_buyer = BuyerMiddleware(
            PaymentRpc(self.node), committed_keys, authority, supported,
        )
        self.addCleanup(receipted_buyer._submissions.close)
        self.addCleanup(committed_buyer._submissions.close)
        responses = {}
        for name, fixture, buyer, body in (
            ("receipted", fixtures, receipted_buyer, b"resource"),
            ("committed", committed, committed_buyer, b"committed resource"),
        ):
            url = fixture["required"]["resource"]["url"]
            with self.assertRaises(HTTPError) as challenge:
                urlopen(url, timeout=30)
            with challenge.exception as response:
                self.assertEqual(response.code, 402)
                self.assertEqual(decode_header(response.headers["PAYMENT-REQUIRED"]), fixture["required"])
            # Save only a header emitted by the real seller's paid HTTP route.
            request = Request(url, headers={"PAYMENT-SIGNATURE": fixture["header"]})
            with urlopen(request, timeout=30) as response:
                self.assertEqual(response.status, 200)
                self.assertEqual(response.read(), body)
                header = response.headers["PAYMENT-RESPONSE"]
            responses[name] = header
            path = "/committed" if name == "committed" else "/paid"
            self.assertEqual(exchange(self.node + "metrics")["settlements"][path], header)
            settlement = decode_header(header)
            extension = settlement["extensions"]["layerx"]
            for member in ("rawTransaction", "signedReceipt", "receiptDigest", "receiptKey"):
                self.assertEqual(extension[member], fixture["proof"][member])
            self.assertEqual(settlement["transaction"], fixture["txHash"])
            self.assertEqual(settlement["amount"], fixture["required"]["accepts"][0]["amount"])
            if name == "committed":
                self.assertEqual(extension["landingBlockHeight"], hex(commit["height"]))
                self.assertEqual(extension["landingBlockHash"], "0x" + base64.b64decode(inclusion["NextCommit"]["BlockID"]["Hash"]).hex())
            else:
                self.assertNotIn("landingBlockHeight", extension)
                self.assertNotIn("landingBlockHash", extension)

        def capture(settlement, payment=None, buyer=None):
            return (buyer or committed_buyer).capture_settlement(
                encode_header(settlement), payment or committed["header"],
            )

        settled = decode_header(responses["committed"])
        layerx = settled["extensions"]["layerx"]
        malformed = []
        for member in ("landingBlockHeight", "landingBlockHash"):
            changed = copy.deepcopy(settled)
            del changed["extensions"]["layerx"][member]
            malformed.append(("missing-" + member, changed))
        for height in (None, 11, "11", "0x", "0x00", "0x0b", "0xB", "0x-1", "0x10000000000000000"):
            changed = copy.deepcopy(settled)
            changed["extensions"]["layerx"]["landingBlockHeight"] = height
            malformed.append(("height-" + str(height), changed))
        for block_hash in (None, 11, "00" * 32, "0x" + "00" * 31, "0x" + "00" * 33, "0x" + "GG" * 32):
            changed = copy.deepcopy(settled)
            changed["extensions"]["layerx"]["landingBlockHash"] = block_hash
            malformed.append(("hash-" + str(block_hash), changed))
        for member in ("unknown", "blockHeader", "commit", "validatorSet", "receiptInclusion"):
            changed = copy.deepcopy(settled)
            changed["extensions"]["layerx"][member] = {}
            malformed.append((member, changed))
        for label, changed in malformed:
            with self.subTest(malformed=label), self.assertRaises(ValueError):
                capture(changed)

        for member in ("landingBlockHeight", "landingBlockHash"):
            submitted = copy.deepcopy(decode_header(committed["header"]))
            submitted["payload"][member] = layerx[member]
            with self.subTest(submitted=member), self.assertRaisesRegex(ValueError, "invalid-payment-fields"):
                _parse_proof(submitted["payload"])
            with self.subTest(capture_submitted=member), self.assertRaisesRegex(ValueError, "invalid-payment-fields"):
                capture(settled, encode_header(submitted))
            with self.subTest(seller_submitted=member), self.assertRaisesRegex(ValueError, "invalid-payment-fields"):
                self.seller.handle("payer", encode_header(self.payload | {
                    "payload": self.payload["payload"] | {member: layerx[member]},
                }), lambda: self.fail("invalid submitted proof released"))

        for member in ("rawTransaction", "signedReceipt", "receiptDigest", "receiptKey"):
            changed = copy.deepcopy(settled)
            changed["extensions"]["layerx"][member] = fixtures["proof"][member]
            with self.subTest(substitution=member), self.assertRaisesRegex(ValueError, "settlement-receipt-mismatch"):
                capture(changed)

        downgrade = copy.deepcopy(settled)
        extension = downgrade["extensions"]["layerx"]
        extension["verificationLevel"] = "receipted"
        for member in ("landingBlockHeight", "landingBlockHash"):
            del extension[member]
        for has_metadata in (False, True):
            changed = copy.deepcopy(downgrade)
            if has_metadata:
                changed["extensions"]["layerx"].update({member: layerx[member] for member in ("landingBlockHeight", "landingBlockHash")})
            with self.subTest(downgrade=has_metadata), self.assertRaisesRegex(ValueError, "commitment-mismatch|invalid-settlement-landing"):
                capture(changed)

        # Both independent proofs must authenticate their optional node JSON.
        receipt_settlement = decode_header(responses["receipted"])
        for submitted in (False, True):
            payment = copy.deepcopy(decode_header(fixtures["header"]))
            changed = copy.deepcopy(receipt_settlement)
            proof = payment["payload"] if submitted else changed["extensions"]["layerx"]
            proof["receiptJson"] = copy.deepcopy(fixtures["result"])
            proof["receiptJson"]["seq"] = "0x0"
            with self.subTest(node_json=submitted), self.assertRaisesRegex(ValueError, "node-receipt-mismatch"):
                capture(changed, encode_header(payment), receipted_buyer)

        for member, replacement in (("transaction", "0x" + "00" * 32), ("payer", "00" * 32)):
            changed = receipt_settlement | {member: replacement}
            with self.subTest(binding=member), self.assertRaisesRegex(ValueError, "settlement-.*-mismatch"):
                capture(changed, fixtures["header"], receipted_buyer)

        payment = copy.deepcopy(decode_header(fixtures["header"]))
        signed = bytearray.fromhex(payment["payload"]["signedReceipt"][2:])
        signed[-1] ^= 1
        payment["payload"]["signedReceipt"] = "0x" + signed.hex()
        changed = copy.deepcopy(receipt_settlement)
        changed["extensions"]["layerx"]["signedReceipt"] = payment["payload"]["signedReceipt"]
        with self.assertRaises(PlatformSdkError):
            capture(changed, encode_header(payment), receipted_buyer)
        for keys in ((), (replace(self.fixture.receipt_keys[0], public_key=b"\x01" * 32),),
                     (replace(self.fixture.receipt_keys[0], last_height=0),)):
            buyer = BuyerMiddleware(PaymentRpc(self.node), keys, self.authority, supported)
            self.addCleanup(buyer._submissions.close)
            with self.subTest(pins=keys), self.assertRaises(PlatformSdkError):
                capture(receipt_settlement, fixtures["header"], buyer)

        no_finality = ConfiguredReceiptAuthority(raw)
        buyer = BuyerMiddleware(PaymentRpc(self.node), committed_keys, no_finality, supported)
        self.addCleanup(buyer._submissions.close)
        with self.assertRaises(PlatformSdkError):
            capture(settled, buyer=buyer)
        for bad_trust, bad_evidence in ((None, evidence), (trust, None),
                                       (CommitTrust("wrong-chain", commit["height"], validators), evidence)):
            invalid = ConfiguredReceiptAuthority(raw, rpc=PaymentRpc(self.node), receipt_keys=committed_keys,
                commit_trust=bad_trust, committed_evidence=bad_evidence)
            buyer = BuyerMiddleware(PaymentRpc(self.node), committed_keys, invalid, supported)
            self.addCleanup(buyer._submissions.close)
            with self.subTest(trust=bad_trust), self.assertRaises(ValueError):
                capture(settled, buyer=buyer)

        for member, replacement in (("landingBlockHeight", hex(commit["height"] + 1)),
                                    ("landingBlockHash", "0x" + "00" * 32)):
            changed = copy.deepcopy(settled)
            changed["extensions"]["layerx"][member] = replacement
            with self.subTest(reference=member), self.assertRaisesRegex(ValueError, "settlement-landing-mismatch"):
                capture(changed)

        # Metadata must not manufacture finality for a receipted proof.
        changed = copy.deepcopy(receipt_settlement)
        changed["extensions"]["layerx"].update({member: layerx[member] for member in ("landingBlockHeight", "landingBlockHash")})
        with self.assertRaisesRegex(ValueError, "invalid-settlement-landing"):
            capture(changed, fixtures["header"], receipted_buyer)

        for name, fixture, buyer, body in (
            ("receipted", fixtures, receipted_buyer, b"resource"),
            ("committed", committed, committed_buyer, b"committed resource"),
        ):
            with self.subTest(round_trip=name):
                verified = buyer.capture_settlement(responses[name], fixture["header"])
                self.assertEqual(verified.canonical_bytes, bytes.fromhex(fixture["proof"]["signedReceipt"][2:]))
                self.assertEqual("0x" + verified.receipt_digest.hex(), fixture["proof"]["receiptDigest"])
                self.assertEqual("0x" + verified.receipt.activity_id.hex(), fixture["txHash"])
                response = buyer.fetch(fixture["required"]["resource"]["url"], fixture["header"])
                self.assertEqual((response[0], response[2]), (200, body))
                self.assertEqual(next(value for key, value in response[1].items() if key.lower() == "payment-response"), responses[name])
                metrics = exchange(self.node + "metrics")
                path = "/committed" if name == "committed" else "/paid"
                self.assertEqual(metrics["paidRequests"][-2:], [{"path": path, "status": 402}, {"path": path, "status": 200}])
        self.assertEqual(exchange(self.node + "metrics")["releases"], 2)

    def test_submitted_offer_never_releases(self):
        required = self.required | {"accepts": [self.offer | {"extra": {"layerx": {"commitment": "submitted"}}}]}
        with self.assertRaises(ValueError):
            SellerMiddleware(required, self.fixture.receipt_keys, self.authority,
                FulfillmentStore(str(Path(self.directory.name) / "submitted.sqlite")))
        self.assertEqual(exchange(self.node + "metrics")["releases"], 0)
