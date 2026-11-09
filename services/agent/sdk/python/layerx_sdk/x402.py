from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass
from hashlib import sha256
from typing import TYPE_CHECKING, Literal, NoReturn

from . import verifier
from .account_derivation import keccak256
from .evm_transaction import EvmTransactionError, decode_signed_transaction, encode_call
from .generated.receipt import ReceiptFailureCode
from .production import PlatformSdkError, SdkErrorCode
from .revert import LXP_RETRYABLE, RevertReasonError, decode_revert_reason
from .verifier import (
    ReceiptKey,
    ReceiptVerificationError,
    SequencerReceiptVerification,
)

if TYPE_CHECKING:
    from .committed import CommitTrust


PaymentCommitment = Literal["receipted", "committed"]
PAYMENT_RECEIPT_PROTOCOL_VERSIONS = (2, 3)
PAYMENT_RUNGS = ("receipted", "committed")
LAYERX_LEDGER_ADDRESS = bytes.fromhex("000000000000000000000000000000000000101a")
LEDGER_DRAW_TYPES = ("bytes32", "bytes32", "uint128", "bytes32", "uint64", "bytes32", "bytes32")


class PaymentCommitmentError(PlatformSdkError):
    def __init__(self) -> None:
        super().__init__(SdkErrorCode.VERIFICATION_FAILURE, "never")
        self.args = (
            "payment commitment must follow the rung ladder: submitted -> receipted -> committed",
        )


@dataclass(frozen=True)
class PaymentEvidence:
    canonical_receipt: bytes
    raw_transaction: bytes
    finality: tuple[CommitTrust, dict] | None = None
    grant_binding: dict | None = None


@dataclass(frozen=True)
class LedgerDrawTerms:
    grant_id: str
    purpose_hash: str
    idempotency_key: str
    context_hash: str


class PaymentBindingError(PlatformSdkError):
    __slots__ = ("field",)

    def __init__(self, field: str) -> None:
        super().__init__(SdkErrorCode.VERIFICATION_FAILURE, "never")
        self.field = field
        self.args = (f"Local verification failed: payment binding {field} mismatch.",)


def _failure(field: str = "payment") -> NoReturn:
    raise PaymentBindingError(field)


def refuse_payer_grant() -> NoReturn:
    raise RevertReasonError(
        "LXP_NO_PAYER_GRANT", "no payer grant", None, LXP_RETRYABLE["LXP_NO_PAYER_GRANT"]
    )


def receipt_protocol_version(canonical_receipt: bytes) -> int:
    if not isinstance(canonical_receipt, bytes) or len(canonical_receipt) < 6:
        raise ReceiptVerificationError(ReceiptFailureCode.DECODE)
    envelope = int.from_bytes(canonical_receipt[0:2], "big")
    version = int.from_bytes(canonical_receipt[4:6], "big")
    if envelope != version or version not in PAYMENT_RECEIPT_PROTOCOL_VERSIONS:
        raise ReceiptVerificationError(ReceiptFailureCode.RECEIPT_VERSION)
    return version


def payment_commitment(extra: object = None) -> PaymentCommitment:
    if not isinstance(extra, Mapping) or "layerx" not in extra:
        return "receipted"
    layerx = extra["layerx"]
    if not isinstance(layerx, Mapping):
        raise PaymentCommitmentError()
    commitment = layerx.get("commitment")
    if commitment not in PAYMENT_RUNGS:
        raise PaymentCommitmentError()
    return commitment


def _layerx_terms(extra: object) -> Mapping | None:
    if not isinstance(extra, Mapping):
        return None
    if "layerx" not in extra:
        return None
    layerx = extra["layerx"]
    if not isinstance(layerx, Mapping):
        _failure()
    return layerx


def payment_payer(extra: object = None, *, required: bool = False) -> str | None:
    terms = _layerx_terms(extra) or {}
    if "payer" not in terms and not required:
        return None
    payer = terms.get("payer")
    if (
        not isinstance(payer, str)
        or len(payer) != 64
        or any(c not in "0123456789abcdef" for c in payer)
        or payer == "0" * 64
    ):
        _failure()
    return payer


def payment_purpose(extra: object = None, *, required: bool = False) -> str | None:
    terms = _layerx_terms(extra) or {}
    if "purposeHash" not in terms and not required:
        return None
    purpose = terms.get("purposeHash")
    if (
        not isinstance(purpose, str)
        or len(purpose) != 64
        or any(c not in "0123456789abcdef" for c in purpose)
        or purpose == "0" * 64
    ):
        _failure()
    return purpose


def verify_sequencer_receipt(
    signed_receipt: bytes,
    raw_transaction: bytes,
    receipt_keys: tuple[ReceiptKey, ...] | list[ReceiptKey],
    *,
    return_data: bytes | None = None,
) -> SequencerReceiptVerification:
    verified = verifier.verify_sequencer_receipt(
        signed_receipt, raw_transaction, receipt_keys
    )
    if return_data is not None and (
        not isinstance(return_data, bytes)
        or sha256(return_data).digest() != verified.receipt.activity_root
    ):
        _failure()
    if verified.receipt.result_code != 0:
        if verified.receipt.result_code == 1 and return_data is not None:
            raise decode_revert_reason(return_data)
        raise ReceiptVerificationError(ReceiptFailureCode.FIELDS)
    return verified


def payment_identifier(value: object) -> bytes:
    if not isinstance(value, str):
        _failure()
    digits = value[2:] if value.startswith("0x") else value
    if len(digits) != 64 or any(c not in "0123456789abcdefABCDEF" for c in digits):
        _failure()
    return bytes.fromhex(digits)


def verify_payment_receipt(
    signed_receipt: bytes,
    raw_transaction: bytes,
    receipt_keys: tuple[ReceiptKey, ...] | list[ReceiptKey],
    *,
    amount: str,
    asset: str,
    pay_to: str,
    payer: str | None = None,
    ledger_payer: str | None = None,
    purpose_hash: str | None = None,
    return_data: bytes | None = None,
    commitment: PaymentCommitment = "receipted",
    finality=None,
    scheme: str = "exact",
    grant_binding: Mapping | LedgerDrawTerms | None = None,
) -> SequencerReceiptVerification:
    if grant_binding is not None or scheme in ("metered", "subscription"):
        refuse_payer_grant()
    payment_commitment({"layerx": {"commitment": commitment}})
    if (
        not isinstance(amount, str)
        or not 0 < len(amount) <= 39
        or any(c not in "0123456789" for c in amount)
        or amount[0] == "0"
        or not 0 < int(amount) < 1 << 128
    ):
        _failure()
    asset_bytes, recipient = payment_identifier(asset), payment_identifier(pay_to)
    sender = None if payer is None else payment_identifier(payer)
    ledger_from = None if ledger_payer is None else payment_identifier(ledger_payer)
    reference = None if purpose_hash is None else payment_identifier(purpose_hash)
    if sender == bytes(32) or ledger_from == bytes(32) or reference == bytes(32):
        _failure()
    verified = verify_sequencer_receipt(
        signed_receipt,
        raw_transaction,
        receipt_keys,
        return_data=return_data,
    )
    receipt = verified.receipt
    try:
        transaction, signature = decode_signed_transaction(raw_transaction)
        recovered = bytes.fromhex(transaction.recover_signer(signature)[2:]).rjust(32, b"\0")
    except EvmTransactionError as error:
        raise PlatformSdkError(SdkErrorCode.VERIFICATION_FAILURE, "never") from error
    if (verified.receipt.from_account != recovered
            or verified.receipt.to_account != transaction.to.rjust(32, b"\0")
            or sender is not None and recovered != sender):
        _failure("payer")
    transaction_operation = 2 if raw_transaction[0] == 2 else 0
    if receipt.module_id != 12 or receipt.operation != transaction_operation or not receipt.effects:
        _failure()
    summary = receipt.effects[0]
    body = summary.body
    if (
        summary.module_id != receipt.module_id
        or summary.ordinal != 0
        or summary.kind != 1
        or summary.monetary
        or summary.event_type != 0
        or len(body) < 110
        or not 1 <= body[108] <= 64
        or len(body) != 109 + body[108]
        or int.from_bytes(body[104:108], "big") != len(receipt.effects) - 1
    ):
        _failure()
    if transaction.chain_id != 125 or transaction.to != LAYERX_LEDGER_ADDRESS or transaction.value != 0:
        _failure()
    calldata = transaction.data
    if scheme == "exact":
        selector = encode_call("payExact", ("bytes32", "bytes32", "bytes32", "uint128", "uint64", "bytes32"),
                               (bytes(32), bytes(32), bytes(32), 0, 0, bytes(32)))[:4]
        if (len(calldata) != 196 or calldata[:4] != selector
                or calldata[36:68] != recipient or calldata[68:100] != asset_bytes
                or int.from_bytes(calldata[100:132], "big") != int(amount)
                or int.from_bytes(calldata[132:164], "big") >= 1 << 64
                or ledger_from is not None and calldata[4:36] != ledger_from
                or reference is not None and calldata[164:196] != reference):
            _failure()
        topic = keccak256(b"PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)")
        data_digest = sha256(asset_bytes + int(amount).to_bytes(32, "big")).digest()
        indexed = (calldata[164:196], calldata[4:36], recipient)
    else:
        _failure()
    payments = 0
    for index, effect in enumerate(receipt.effects[1:], 1):
        body = effect.body
        if (
            effect.module_id != receipt.module_id
            or effect.ordinal != index
            or effect.kind != 3
            or effect.monetary
            or effect.event_type != 0
            or len(body) < 53
            or body[20] > 4
            or len(body) != 53 + body[20] * 32
        ):
            _failure()
        if body[:20] != LAYERX_LEDGER_ADDRESS or body[21:53] != topic:
            continue
        if body[20] != 4:
            _failure("event.topic_count")
        for field, actual, required in (
            ("idempotency_key", body[53:85], indexed[0]),
            ("payer", body[85:117], indexed[1]),
            ("pay_to", body[117:149], indexed[2]),
            ("data_digest", body[-32:], data_digest),
        ):
            if actual != required:
                _failure("event." + field)
        payments += 1
    if payments != 1:
        _failure()
    if commitment == "committed":
        if finality is None:
            _failure()
        if not isinstance(finality, tuple) or len(finality) != 2:
            _failure()
        trust, evidence = finality
        from .committed import CommitTrust
        if not isinstance(trust, CommitTrust):
            _failure()
        trust.verify(raw_transaction, verified.height, evidence)
    return verified


def ledger_payment_payer(verified: SequencerReceiptVerification, raw_transaction: bytes, scheme: str = "exact") -> str:
    from .payment_transaction import decode_payment_transaction
    if scheme in ("metered", "subscription"):
        refuse_payer_grant()
    if scheme != "exact":
        _failure()
    transaction = decode_payment_transaction(raw_transaction)
    if transaction.sender.rjust(32, b"\0") != verified.receipt.from_account:
        _failure("payer")
    return verified.receipt.from_account.hex()
