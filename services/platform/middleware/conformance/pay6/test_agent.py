import concurrent.futures
import sqlite3
import tempfile
import unittest
from pathlib import Path
from urllib.error import URLError

from test_commitment import CommitmentTests, FIXTURE
from layerx_sdk.verifier import verify_sequencer_receipt
from layerx_sdk.x402_agent import PaymentBudget
from layerx_sdk.account_derivation import _N, _recover, keccak256
from layerx_sdk.evm_transaction import Type2Transaction, decode_signed, encode_call
from layerx_sdk.production import PlatformSdkError, SdkErrorCode
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec, utils
from layerx_sdk.x402_rpc import PaymentRpc
from layerx_sdk.x402_http import PaymentEvidence
from recorded_rpc import recorded_rpc, result as rpc_result
from node_peer import node_peer, exchange


class AgentBudgetTests(CommitmentTests):
    def test_agent_receipt_uses_pinned_evm_key(self):
        verified = verify_sequencer_receipt(
            self.wire, self.raw_transaction, self.receipt_keys
        )
        self.assertEqual(verified.canonical_bytes, self.wire)
        self.assertEqual(verified.receipt.activity_id.hex(), FIXTURE["txHash"][2:])
        self.assertEqual(verified.height, int(FIXTURE["rpcResult"]["height"], 16))
        self.assertEqual(verified.receipt.from_account.hex(), FIXTURE["payer"][2:])

    def test_budget_receipt_commit_and_replay(self):
        offer = {
            "scheme": "exact",
            "network": "eip155:125",
            "amount": self.offer["amount"],
            "asset": FIXTURE["offer"]["asset"],
            "payTo": FIXTURE["offer"]["payTo"],
            "maxTimeoutSeconds": 30,
        }
        endpoint, produced = self.enterContext(node_peer())
        self.assertEqual(produced["proof"], FIXTURE["receipt"])
        typescript = exchange(endpoint + "agent", {
            "proof": produced["proof"], "spend": produced["agentSpend"],
        })
        self.assertEqual(typescript["network"], offer["network"])
        self.assertEqual(typescript["amount"], offer["amount"])
        self.assertEqual(typescript["asset"], offer["asset"])
        self.assertEqual(typescript["payTo"], offer["payTo"])
        self.assertEqual(
            typescript["receiptDigest"],
            "0x" + verify_sequencer_receipt(
                self.wire, self.raw_transaction, self.receipt_keys
            ).receipt_digest.hex(),
        )
        evidence = PaymentEvidence(self.wire, self.raw_transaction)
        with tempfile.TemporaryDirectory() as directory:
            path = str(Path(directory) / "budgets.sqlite")
            budget = PaymentBudget(
                path, "tenant", offer["asset"], int(offer["amount"]) * 2
            )
            key, digest = "01" * 32, "02" * 32
            with concurrent.futures.ThreadPoolExecutor() as pool:
                list(pool.map(lambda _: budget.reserve(key, digest, offer), range(8)))
            for _ in range(2):
                budget = PaymentBudget(
                    path, "tenant", offer["asset"], int(offer["amount"]) * 2
                )
                verified = budget.verify_and_commit(
                    key, digest, offer, evidence, self.receipt_keys
                )
                self.assertEqual(verified.canonical_bytes, evidence.canonical_receipt)
            with self.assertRaises(ValueError):
                budget.reserve(key, "03" * 32, offer)
            second = "04" * 32
            budget.reserve(second, digest, offer)
            with self.assertRaises(sqlite3.IntegrityError):
                budget.verify_and_commit(
                    second, digest, offer, evidence, self.receipt_keys
                )
            with self.assertRaises(ValueError):
                budget.reserve("05" * 32, digest, offer)
            with self.assertRaises(ValueError):
                PaymentBudget(path, "tenant", offer["asset"], 1)
            with sqlite3.connect(path) as db:
                self.assertEqual(
                    db.execute(
                        "SELECT count(*) FROM payment_reservations WHERE receipt_digest IS NOT NULL"
                    ).fetchone(),
                    (1,),
                )
                self.assertEqual(
                    db.execute(
                        "SELECT receipt_digest FROM payment_reservations WHERE request_key = ?",
                        (key,),
                    ).fetchone(),
                    (
                        typescript["receiptDigest"][2:],
                    ),
                )

    def test_agent_draw_preserves_unknown_reservation(self):
        words = (
            bytes.fromhex("33" * 32),
            bytes.fromhex("11" * 32),
            (40).to_bytes(32, "big"),
            bytes.fromhex("44" * 32),
            (0x0102030405060708).to_bytes(32, "big"),
            bytes.fromhex("22" * 32),
            bytes.fromhex("55" * 32),
        )
        calldata = encode_call(
            "draw",
            ("bytes32", "bytes32", "uint128", "bytes32", "uint64", "bytes32", "bytes32"),
            (words[0], words[1], 40, words[3], 0x0102030405060708, words[5], words[6]),
        )
        self.assertEqual(calldata, bytes.fromhex("2e89490f") + b"".join(words))
        transaction = Type2Transaction(
            chain_id=125, nonce=7, max_priority_fee_per_gas=1,
            max_fee_per_gas=2, gas_limit=200000,
            to=bytes.fromhex("000000000000000000000000000000000000101a"),
            value=0, data=calldata,
        )
        signing_key = ec.derive_private_key(int.from_bytes(bytes([1]) * 32, "big"), ec.SECP256K1())
        public = signing_key.public_key().public_numbers()
        public_xy = public.x.to_bytes(32, "big") + public.y.to_bytes(32, "big")
        signing_digest = transaction.signing_digest()
        r, s = utils.decode_dss_signature(signing_key.sign(
            signing_digest, ec.ECDSA(utils.Prehashed(hashes.SHA256()))
        ))
        s = min(s, _N - s)
        signature = None
        for parity in (0, 1):
            if _recover(signing_digest, r, s, parity) == public_xy:
                signature = r.to_bytes(32, "big") + s.to_bytes(32, "big") + bytes([27 + parity])
                break
        self.assertIsNotNone(signature)
        signer = "0x1a642f0e3c3af545e7acbd38b07251b3990914f1"
        canonical = transaction.attach_signature(signature, signer)
        self.assertEqual(decode_signed(canonical)[0], transaction)
        self.assertEqual(transaction.recover_signer(signature).lower(), signer)
        key, digest = words[3].hex(), words[6].hex()
        offer = {
            "scheme": "exact",
            "network": "eip155:125",
            "amount": "40",
            "asset": "0x" + "66" * 32,
            "payTo": "0x" + "11" * 32,
            "maxTimeoutSeconds": 30,
            "extra": {"layerx": {"commitment": "receipted"}},
        }
        with tempfile.TemporaryDirectory() as directory:
            path = str(Path(directory) / "budget.sqlite")
            budget = PaymentBudget(path, "tenant", offer["asset"], 40)
            budget.reserve(key, digest, offer)
            transaction_hash = "0x" + keccak256(canonical).hex()
            answers = [None, rpc_result(None), rpc_result(None)]
            with recorded_rpc(answers) as (endpoint, requests):
                rpc = PaymentRpc(endpoint)
                with self.assertRaises(PlatformSdkError) as raised:
                    rpc.send("0x" + canonical.hex())
                self.assertEqual(raised.exception.code, SdkErrorCode.UNKNOWN_OUTCOME)
                for _ in range(2):
                    budget = PaymentBudget(path, "tenant", offer["asset"], 40)
                    budget.reserve(key, digest, offer)
                    self.assertIsNone(rpc.receipt(transaction_hash))
                self.assertEqual([entry["body"]["method"] for entry in requests],
                    ["eth_sendRawTransaction", "pax_sequencerReceipt", "pax_sequencerReceipt"])
                self.assertEqual(requests[0]["body"]["params"], ["0x" + canonical.hex()])
            with self.assertRaises(ValueError):
                budget.reserve("66" * 32, digest, offer)
            with sqlite3.connect(path) as db:
                self.assertEqual(
                    db.execute(
                        "SELECT amount, receipt_digest FROM payment_reservations"
                    ).fetchall(),
                    [(offer["amount"], None)],
                )

    def test_budget_requires_requested_commitment(self):
        offer = {
            "scheme": "exact",
            "network": "eip155:125",
            "amount": self.offer["amount"],
            "asset": FIXTURE["offer"]["asset"],
            "payTo": FIXTURE["offer"]["payTo"],
            "maxTimeoutSeconds": 30,
            "extra": {"layerx": {"commitment": "committed"}},
        }
        with tempfile.TemporaryDirectory() as directory:
            path = str(Path(directory) / "budgets.sqlite")
            budget = PaymentBudget(path, "tenant", offer["asset"], int(offer["amount"]))
            key, digest = "01" * 32, "02" * 32
            budget.reserve(key, digest, offer)
            with self.assertRaises(PlatformSdkError):
                budget.verify_and_commit(
                    key,
                    digest,
                    offer,
                    PaymentEvidence(self.wire, self.raw_transaction),
                    self.receipt_keys,
                )
            with sqlite3.connect(path) as db:
                self.assertEqual(
                    db.execute(
                        "SELECT receipt_digest FROM payment_reservations"
                    ).fetchone(),
                    (None,),
                )

    def test_submitted_cannot_reserve_or_commit_a_payment(self):
        offer = FIXTURE["offer"] | {"extra": {"layerx": {"commitment": "submitted"}}}
        with tempfile.TemporaryDirectory() as directory:
            path = str(Path(directory) / "budget.sqlite")
            budget = PaymentBudget(path, "tenant", offer["asset"], int(offer["amount"]))
            with self.assertRaises((ValueError, PlatformSdkError)):
                budget.reserve("01" * 32, "02" * 32, offer)
            with sqlite3.connect(path) as db:
                self.assertEqual(db.execute("SELECT count(*) FROM payment_reservations").fetchone(), (0,))


if __name__ == "__main__":
    unittest.main()
