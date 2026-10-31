from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass
from hashlib import sha256
from typing import Literal, NoReturn

from . import verifier
from .generated.receipt import ReceiptFailureCode
from .production import PlatformSdkError, SdkErrorCode
from .revert import decode_revert_reason
from .verifier import (
    MerkleProof,
    ReceiptKey,
    ReceiptVerificationError,
    SequencerReceiptVerification,
)

PaymentCommitment = Literal["executed", "batched", "finalised"]
PAYMENT_RECEIPT_PROTOCOL_VERSIONS = (2, 3)


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
        raise ReceiptVerificationError(ReceiptFailureCode.PROTOCOL_VERSION)
    return version


def payment_commitment(extra: object = None) -> PaymentCommitment:
    if not isinstance(extra, Mapping) or "layerx" not in extra:
        return "executed"
    layerx = extra["layerx"]
    if not isinstance(layerx, Mapping):
        _failure()
    commitment = layerx.get("commitment")
    if commitment not in ("executed", "batched", "finalised"):
        _failure()
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
        raise ReceiptVerificationError(ReceiptFailureCode.RESULT_CODE)
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
) -> SequencerReceiptVerification:
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
    if (
        verified.receipt.amount != int(amount)
        or verified.receipt.asset.hex() != asset
        or verified.receipt.to_account.hex() != pay_to
        or (payer is not None and verified.receipt.from_account.hex() != payer)
    ):
        _failure()
    return verified
