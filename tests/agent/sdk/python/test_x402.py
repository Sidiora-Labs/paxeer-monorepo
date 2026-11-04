from __future__ import annotations

import copy
import json
import unittest
from hashlib import sha256

from layerx_sdk import (
    PlatformSdkError,
    ReceiptFailureCode,
    ReceiptVerificationError,
    verify_payment_receipt,
)
from layerx_sdk.account_derivation import (
    BindFees,
    LayerXBindState,
    derive_from_mnemonic,
    keccak256,
    plan_layerx_bind,
    sign_bind_transaction,
)
from layerx_sdk.production import SdkErrorCode
from layerx_sdk.revert import (
    LXP_RETRYABLE, REVERT_SELECTOR, RevertReasonError, decode_revert_reason,
)
from layerx_sdk.verifier import SequencerReceiptVerificationError
from layerx_sdk.x402 import PAYMENT_RECEIPT_PROTOCOL_VERSIONS, receipt_protocol_version
from layerx_sdk.x402_rpc import (
    PaymentRpcError,
    _committed_result,
    _decode_rpc_response,
    _return_data,
    _send_result,
    _sequencer_result,
    _submission_error,
    _transaction_bytes,
    verify_rpc_payment,
)

from payment_receipt_evidence import ROOT, payment_terms, receipt_key, receipt_vector, unhex

_FIXTURES = ROOT / "sdk/platform/conformance/fixtures"


def _verify(vector, canonical=None, **changes):
    terms = {**payment_terms(vector), **changes}
    return verify_payment_receipt(
        unhex(vector["signedReceipt"]) if canonical is None else canonical,
        unhex(vector["rawTransaction"]),
        (receipt_key(vector),),
        **terms,
    )


def _revert_data(code: str) -> bytes:
    # ABI codec vectors; these are never signed or presented as node evidence.
    text = f"{code}: payment refused; ledger".encode()
    return (REVERT_SELECTOR + (32).to_bytes(32, "big")
            + len(text).to_bytes(32, "big") + text + bytes(-len(text) % 32))


class VerifyPaymentReceiptTest(unittest.TestCase):
    def test_supported_versions(self) -> None:
        # Historical version inspection remains distinct from payment authority.
        self.assertEqual(PAYMENT_RECEIPT_PROTOCOL_VERSIONS, (2, 3))
        vector = receipt_vector()
        verified = _verify(vector)
        self.assertEqual(verified.receipt.protocol_version, 2)
        self.assertEqual(verified.level, "sequencer-signed")

    def test_sequencer_receipt_verifies_with_raw_transaction_and_governed_key(self) -> None:
        vector = receipt_vector()
        verified = _verify(vector)
        self.assertEqual(verified.canonical_bytes, unhex(vector["signedReceipt"]))
        self.assertEqual(verified.receipt_digest, unhex(vector["receiptDigest"]))
        self.assertEqual(verified.height, int(vector["receiptJson"]["height"], 16))
        self.assertEqual(verified.receipt.activity_id, keccak256(unhex(vector["rawTransaction"])))
        self.assertEqual(verified.receipt.context_hash, receipt_key(vector).key_id)

    def test_version_three_layout_is_only_historical_and_refused_for_payment(self) -> None:
        vector = receipt_vector()
        canonical = unhex(vector["signedReceipt"])
        historical = b"\x00\x03" + canonical[2:4] + b"\x00\x03" + canonical[6:]
        self.assertEqual(receipt_protocol_version(historical), 3)
        with self.assertRaises(SequencerReceiptVerificationError) as raised:
            _verify(vector, historical)
        self.assertEqual(raised.exception.check, "bad_signature")

    def test_unsupported_version_is_refused_by_name(self) -> None:
        vector = receipt_vector()
        canonical = unhex(vector["signedReceipt"])
        for changed in (
            b"\x00\x01" + canonical[2:4] + b"\x00\x01" + canonical[6:],
            b"\x00\x04" + canonical[2:4] + b"\x00\x04" + canonical[6:],
            b"\x00\x03" + canonical[2:4] + b"\x00\x02" + canonical[6:],
        ):
            with self.assertRaises(ReceiptVerificationError) as raised:
                receipt_protocol_version(changed)
            self.assertIs(raised.exception.check, ReceiptFailureCode.RECEIPT_VERSION)
            with self.assertRaises(SequencerReceiptVerificationError) as refused:
                _verify(vector, changed)
            self.assertEqual(refused.exception.check, "bad_signature")
        with self.assertRaises(ReceiptVerificationError) as raised:
            receipt_protocol_version(canonical[:5])
        self.assertIs(raised.exception.check, ReceiptFailureCode.DECODE)

    def test_carried_version_does_not_bypass_the_signature(self) -> None:
        vector = receipt_vector()
        canonical = unhex(vector["signedReceipt"])
        for changed in (
            canonical[:-1] + bytes([canonical[-1] ^ 1]),
            canonical[:126] + bytes([canonical[126] ^ 1]) + canonical[127:],
        ):
            with self.assertRaises(SequencerReceiptVerificationError) as raised:
                _verify(vector, changed)
            self.assertEqual(raised.exception.check, "bad_signature")
        wrong = copy.deepcopy(vector)
        raw = unhex(vector["rawTransaction"])
        wrong["rawTransaction"] = "0x" + (raw[:-1] + bytes([raw[-1] ^ 1])).hex()
        with self.assertRaises(SequencerReceiptVerificationError) as raised:
            _verify(wrong)
        self.assertEqual(raised.exception.check, "hash_mismatch")

    def test_payment_terms_still_bind(self) -> None:
        vector = receipt_vector()
        terms = payment_terms(vector)
        for changed in (
            {"payer": "ab" * 32}, {"asset": "ab" * 32},
            {"pay_to": "ab" * 32}, {"amount": str(int(terms["amount"]) + 1)},
        ):
            with self.subTest(changed=changed), self.assertRaises(PlatformSdkError) as raised:
                _verify(vector, **changed)
            self.assertIs(raised.exception.code, SdkErrorCode.VERIFICATION_FAILURE)

class PaymentRpcProtocolTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        fixture = json.loads((_FIXTURES / "account-derivation-v1.json").read_text())
        vector = fixture["mnemonic_vectors"][0]
        account = vector["accounts"][0]
        bind = account["bind"]
        derived = derive_from_mnemonic(
            vector["mnemonic"], vector["passphrase"], account["index"]
        )
        plan = plan_layerx_bind(
            derived, bind["chain_id"], LayerXBindState(None, bind["nonce"])
        )
        if plan.call is None:
            raise AssertionError("bind transaction unavailable")
        cls.raw = _transaction_bytes(
            sign_bind_transaction(
                derived, bind["chain_id"], plan.call, BindFees(0, 1, 2, 100000)
            )
        )
        cls.transaction_hash = keccak256(cls.raw)
        cls.hash_hex = "0x" + cls.transaction_hash.hex()

    def test_send_result_binds_the_signed_evm_transaction(self) -> None:
        self.assertEqual(self.raw[0], 2)
        self.assertEqual(_send_result(self.hash_hex, self.raw), self.hash_hex)
        self.assertEqual(
            _send_result("0x" + self.transaction_hash.hex().upper(), self.raw),
            self.hash_hex,
        )
        with self.assertRaisesRegex(ValueError, "^transaction-hash-mismatch$"):
            _send_result("0x" + bytes(32).hex(), self.raw)

    def test_send_result_refuses_legacy_and_malformed_hashes(self) -> None:
        for result in (
            None,
            {},
            self.hash_hex[2:],
            "0x1",
            "0x" + "gg" * 32,
            self.hash_hex + "00",
        ):
            with self.subTest(result=result):
                with self.assertRaisesRegex(ValueError, "^invalid-rpc-hex$"):
                    _send_result(result, self.raw)

    def test_rpc_envelope_allows_transaction_hash_and_unknown_receipt(self) -> None:
        for result in (self.hash_hex, None, {"rung": "submitted"}):
            with self.subTest(result=result):
                self.assertEqual(
                    _decode_rpc_response(
                        {"jsonrpc": "2.0", "id": 1, "result": result}, 1
                    ),
                    result,
                )

    def test_rpc_envelope_refuses_ambiguous_or_mismatched_responses(self) -> None:
        for body in (
            [],
            {"jsonrpc": "1.0", "id": 1, "result": None},
            {"jsonrpc": "2.0", "id": True, "result": None},
            {"jsonrpc": "2.0", "id": 2, "result": None},
            {"jsonrpc": "2.0", "id": 1},
            {"jsonrpc": "2.0", "id": 1, "result": None, "error": {}},
        ):
            with self.subTest(body=body):
                with self.assertRaisesRegex(ValueError, "^invalid-rpc-envelope$"):
                    _decode_rpc_response(body, 1)
        for result in (0, True, [], 1.0):
            with self.subTest(result=result):
                with self.assertRaisesRegex(ValueError, "^invalid-rpc-result$"):
                    _decode_rpc_response(
                        {"jsonrpc": "2.0", "id": 1, "result": result}, 1
                    )

    def test_rpc_error_keeps_its_code_and_data(self) -> None:
        data = {"nonce": "0x1"}
        with self.assertRaises(PaymentRpcError) as raised:
            _decode_rpc_response(
                {
                    "jsonrpc": "2.0",
                    "id": 1,
                    "error": {"code": -32000, "message": "refused", "data": data},
                },
                1,
            )
        self.assertEqual(raised.exception.code, -32000)
        self.assertEqual(raised.exception.data, data)
        for error in (
            None,
            {"code": True, "message": "refused"},
            {"code": -32000, "message": None},
        ):
            with self.subTest(error=error):
                with self.assertRaisesRegex(ValueError, "^invalid-rpc-error$"):
                    _decode_rpc_response({"jsonrpc": "2.0", "id": 1, "error": error}, 1)

    def test_unknown_and_submitted_receipts_stay_unexecuted(self) -> None:
        self.assertIsNone(_sequencer_result(None, self.transaction_hash))
        submitted = {"txHash": self.hash_hex, "nonce": "0x0", "rung": "submitted"}
        self.assertEqual(_sequencer_result(submitted, self.transaction_hash), submitted)
        for changed in (
            {**submitted, "txHash": "0x" + bytes(32).hex()},
            {**submitted, "rung": "completed"},
            {**submitted, "status": "0x1"},
            {**submitted, "nonce": "0x00"},
            {**submitted, "nonce": "0x" + "f" * 17},
        ):
            with self.subTest(changed=changed):
                with self.assertRaisesRegex(ValueError, "^invalid-sequencer-receipt$"):
                    _sequencer_result(changed, self.transaction_hash)

    def test_committed_rpc_requires_a_block_hash_and_matching_transaction(self) -> None:
        self.assertIsNone(_committed_result(None, self.transaction_hash))
        receipt = {
            "transactionHash": self.hash_hex,
            "blockHash": "0x" + sha256(self.raw).hexdigest(),
            "status": "0x1",
        }
        self.assertEqual(_committed_result(receipt, self.transaction_hash), receipt)
        for changed in (
            {**receipt, "transactionHash": "0x" + bytes(32).hex()},
            {**receipt, "status": 1},
            {**receipt, "status": "0x2"},
        ):
            with self.subTest(changed=changed):
                with self.assertRaisesRegex(
                    ValueError, "^invalid-transaction-receipt$"
                ):
                    _committed_result(changed, self.transaction_hash)
        with self.assertRaisesRegex(ValueError, "^invalid-rpc-hex$"):
            _committed_result({**receipt, "blockHash": None}, self.transaction_hash)

    def test_submission_reverts_keep_registry_codes_and_retry_classes(self) -> None:
        for code, retryable in LXP_RETRYABLE.items():
            for data in (
                "0x" + _revert_data(code).hex(),
                {"data": "0x" + _revert_data(code).hex()},
            ):
                with self.subTest(code=code, nested=isinstance(data, dict)):
                    error = PaymentRpcError(3, data)
                    with self.assertRaises(RevertReasonError) as raised:
                        _submission_error(error)
                    self.assertEqual(
                        (
                            raised.exception.code,
                            raised.exception.retryable,
                            raised.exception.detail,
                        ),
                        (code, retryable, "ledger"),
                    )
                    self.assertIs(raised.exception.__cause__, error)

    def test_non_revert_rpc_errors_keep_the_existing_mapping(self) -> None:
        for data in (None, {"state": "pending"}, "unavailable"):
            with self.subTest(data=data):
                self.assertIsNone(_submission_error(PaymentRpcError(-32000, data)))

    def test_malformed_submission_revert_is_refused(self) -> None:
        for data in ("0x", "0x00", "0x0", "0xgg"):
            with self.subTest(data=data):
                with self.assertRaises(PlatformSdkError) as raised:
                    _submission_error(PaymentRpcError(3, data))
                self.assertIs(raised.exception.code, SdkErrorCode.DECODE_FAILURE)
                self.assertEqual(raised.exception.retry, "never")

    def test_return_data_accepts_empty_evm_results(self) -> None:
        self.assertEqual(_return_data("0x"), b"")
        self.assertEqual(_return_data("0xAa"), b"\xaa")
        for data in (None, "", "aa", "0x1", "0xgg"):
            with self.subTest(data=data):
                with self.assertRaisesRegex(ValueError, "^invalid-rpc-hex$"):
                    _return_data(data)


class PaymentRevertBindingTest(unittest.TestCase):
    def verify(self, vector, canonical=None, return_data=None):
        return _verify(vector, canonical, return_data=return_data)

    def test_signed_revert_reports_retriable_and_terminal_codes(self) -> None:
        vector = receipt_vector("V2-layerxledger-revert")
        data = unhex(vector["receiptJson"]["returnData"])
        with self.assertRaises(RevertReasonError) as raised:
            self.verify(vector, return_data=data)
        self.assertEqual((raised.exception.code, raised.exception.retryable),
                         ("LXP_INSUFFICIENT_BALANCE", True))
        # Other registry codes remain codec assertions, not fabricated receipts.
        for code in ("LXP_ACCOUNT_FROZEN", "LXP_IDEMPOTENT_REPLAY", "LXP_UNKNOWN"):
            decoded = decode_revert_reason(_revert_data(code))
            self.assertEqual((decoded.code, decoded.retryable), (code, LXP_RETRYABLE[code]))
            self.assertEqual(decoded.detail, "ledger")

    def test_unbound_return_data_is_refused_before_decoding_the_revert(self) -> None:
        vector = receipt_vector("V2-layerxledger-revert")
        with self.assertRaises(PlatformSdkError) as raised:
            self.verify(vector, return_data=_revert_data("LXP_IDEMPOTENT_REPLAY"))
        self.assertIs(raised.exception.code, SdkErrorCode.VERIFICATION_FAILURE)
        self.assertEqual(raised.exception.retry, "never")

    def test_revert_signature_is_required_before_return_data_is_decoded(self) -> None:
        vector = receipt_vector("V2-layerxledger-revert")
        canonical = unhex(vector["signedReceipt"])
        corrupt = canonical[:-1] + bytes([canonical[-1] ^ 1])
        with self.assertRaises(SequencerReceiptVerificationError) as raised:
            self.verify(vector, corrupt, unhex(vector["receiptJson"]["returnData"]))
        self.assertEqual(raised.exception.check, "bad_signature")

    def test_revert_without_return_data_keeps_the_result_code_refusal(self) -> None:
        with self.assertRaises(ReceiptVerificationError) as raised:
            self.verify(receipt_vector("V2-layerxledger-revert"))
        self.assertIs(raised.exception.check, ReceiptFailureCode.FIELDS)

    def test_sequencer_transport_propagates_the_revert_registry_class(self) -> None:
        vector = receipt_vector("V2-layerxledger-revert")
        with self.assertRaises(RevertReasonError) as raised:
            _sequencer_result(vector["receiptJson"], keccak256(unhex(vector["rawTransaction"])))
        self.assertEqual((raised.exception.code, raised.exception.retryable),
                         ("LXP_INSUFFICIENT_BALANCE", True))

    def test_rpc_payment_propagates_the_signed_revert_code(self) -> None:
        vector = receipt_vector("V2-layerxledger-revert")
        terms = payment_terms(vector)
        with self.assertRaises(RevertReasonError) as raised:
            verify_rpc_payment(
                vector["receiptJson"], vector["receiptJson"]["txHash"],
                "0x" + terms.pop("payer"), receipt_keys=(receipt_key(vector),),
                raw_transaction=unhex(vector["rawTransaction"]), **terms,
            )
        self.assertEqual((raised.exception.code, raised.exception.retryable),
                         ("LXP_INSUFFICIENT_BALANCE", True))


if __name__ == "__main__":
    unittest.main()
