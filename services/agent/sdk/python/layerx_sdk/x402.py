from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass
from hashlib import sha256
from typing import Literal, NoReturn

from . import verifier
from .account_derivation import keccak256
from .evm_transaction import EvmTransactionError, decode_signed_transaction, encode_call
from .generated.receipt import ReceiptFailureCode
from .production import PlatformSdkError, SdkErrorCode
from .revert import decode_revert_reason
from .verifier import (
    MerkleProof,
    ReceiptKey,
    ReceiptVerificationError,
    SequencerReceiptVerification,
)

PaymentCommitment = Literal["submitted", "receipted", "committed"]
PAYMENT_RECEIPT_PROTOCOL_VERSIONS = (2, 3)
PAYMENT_RUNGS = ("submitted", "receipted", "committed")
LAYERX_LEDGER_ADDRESS = bytes.fromhex("000000000000000000000000000000000000101a")


class PaymentCommitmentError(PlatformSdkError):
    def __init__(self) -> None:
        super().__init__(SdkErrorCode.VERIFICATION_FAILURE, "never")
        self.args = (
            "payment commitment must follow the rung ladder: submitted -> receipted -> committed",
        )


@dataclass(frozen=True)
class _SequencerAuthorization:
    sequencer_id: bytes
    public_key: bytes
    first_batch_number: int
    last_batch_number: int


@dataclass(frozen=True)
class PaymentCommitmentEvidence:
    network_id: int
    canonical_header: bytes
    header_signature: bytes
    authorization: _SequencerAuthorization
    proof: MerkleProof


def _failure() -> NoReturn:
    raise PlatformSdkError(SdkErrorCode.VERIFICATION_FAILURE, "never")


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
    layerx = extra.get("layerx")
    if layerx is None:
        return None
    if not isinstance(layerx, Mapping):
        _failure()
    return layerx


def payment_payer(extra: object = None, *, required: bool = False) -> str | None:
    payer = (_layerx_terms(extra) or {}).get("payer")
    if payer is None and not required:
        return None
    if (
        not isinstance(payer, str)
        or len(payer) != 64
        or any(c not in "0123456789abcdef" for c in payer)
        or payer == "0" * 64
    ):
        _failure()
    return payer


def payment_purpose(extra: object = None, *, required: bool = False) -> str | None:
    purpose = (_layerx_terms(extra) or {}).get("purposeHash")
    if purpose is None and not required:
        return None
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


def verify_payment_receipt(
    signed_receipt: bytes,
    raw_transaction: bytes,
    receipt_keys: tuple[ReceiptKey, ...] | list[ReceiptKey],
    *,
    amount: str,
    asset: str,
    pay_to: str,
    payer: str | None = None,
    return_data: bytes | None = None,
    commitment: PaymentCommitment = "receipted",
) -> SequencerReceiptVerification:
    payment_commitment({"layerx": {"commitment": commitment}})
    if commitment == "committed":
        _failure()
    if (
        not isinstance(amount, str)
        or not 0 < len(amount) <= 39
        or any(c not in "0123456789" for c in amount)
        or amount[0] == "0"
        or not 0 < int(amount) < 1 << 128
    ):
        _failure()
    for identifier in (asset, pay_to) + (() if payer is None else (payer,)):
        if (
            not isinstance(identifier, str)
            or len(identifier) != 64
            or any(c not in "0123456789abcdef" for c in identifier)
        ):
            _failure()
    if payer == "0" * 64:
        _failure()
    verified = verify_sequencer_receipt(
        signed_receipt,
        raw_transaction,
        receipt_keys,
        return_data=return_data,
    )
    receipt = verified.receipt
    try:
        transaction, _ = decode_signed_transaction(raw_transaction)
    except EvmTransactionError as error:
        raise PlatformSdkError(SdkErrorCode.VERIFICATION_FAILURE, "never") from error
    if receipt.module_id <= 11 or not receipt.effects:
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
        or (payer is not None and receipt.from_account.hex() != payer)
    ):
        _failure()
    if transaction.to != LAYERX_LEDGER_ADDRESS or transaction.value != 0:
        _failure()
    calldata = transaction.data
    selector = encode_call(
        "payExact",
        ("bytes32", "bytes32", "bytes32", "uint128", "uint64", "bytes32"),
        (bytes(32), bytes(32), bytes(32), 0, 0, bytes(32)),
    )[:4]
    if (
        len(calldata) != 4 + 6 * 32
        or calldata[:4] != selector
        or calldata[36:68].hex() != pay_to
        or calldata[68:100].hex() != asset
        or int.from_bytes(calldata[100:132], "big") != int(amount)
        or int.from_bytes(calldata[132:164], "big") >= 1 << 64
    ):
        _failure()
    topic = keccak256(b"PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)")
    data_digest = sha256(bytes.fromhex(asset) + int(amount).to_bytes(32, "big")).digest()
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
        if (
            body[20] != 4
            or body[-32:] != data_digest
            or body[53:85] != calldata[164:196]
            or body[85:117] != calldata[4:36]
            or body[117:149].hex() != pay_to
        ):
            _failure()
        payments += 1
    if payments != 1:
        _failure()
    return verified
