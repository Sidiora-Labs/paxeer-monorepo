from __future__ import annotations

from dataclasses import dataclass
from hashlib import sha256
from typing import Literal, Protocol, cast

from .account_derivation import keccak256
from .generated.receipt import ReceiptFailureCode
from .production import PlatformSdkError, SdkErrorCode

_RECEIPT_DOMAIN = b"LXP/v1/receipt\0"
_MAX_MESSAGE_BYTES = 1_048_576
_MAX_EFFECTS = 512
_MAX_EFFECT_BODY = 256
_MAX_U128 = 0xFFFF_FFFF_FFFF_FFFF_FFFF_FFFF_FFFF_FFFF


class ReceiptVerificationError(PlatformSdkError):
    __slots__ = ("check",)

    def __init__(self, check: ReceiptFailureCode) -> None:
        super().__init__(SdkErrorCode.VERIFICATION_FAILURE, "never")
        self.check = check


def _receipt_failure(check: ReceiptFailureCode) -> None:
    raise ReceiptVerificationError(check)


def _failure() -> None:
    raise PlatformSdkError(SdkErrorCode.VERIFICATION_FAILURE, "never")


def _exact(value: bytes, length: int) -> bytes:
    if len(value) != length:
        _failure()
    return value


def _equal(left: bytes, right: bytes) -> bool:
    if len(left) != len(right):
        return False
    difference = 0
    for left_byte, right_byte in zip(left, right, strict=True):
        difference |= left_byte ^ right_byte
    return difference == 0


def _digest(domain: bytes, *values: bytes) -> bytes:
    digest = sha256()
    digest.update(domain)
    for value in values:
        digest.update(value)
    return digest.digest()


@dataclass(frozen=True)
class MerkleProof:
    leaf_index: int
    leaf_count: int
    siblings: tuple[bytes, ...]


class _Decoder:
    def __init__(self, value: bytes) -> None:
        self._value = value
        self._offset = 0

    def fixed(self, length: int) -> bytes:
        end = self._offset + length
        if length < 0 or end > len(self._value):
            _failure()
        result = self._value[self._offset:end]
        self._offset = end
        return result

    def integer(self, length: int) -> int:
        return int.from_bytes(self.fixed(length), "big", signed=False)

    def u8(self) -> int:
        return self.integer(1)

    def u16(self) -> int:
        return self.integer(2)

    def u32(self) -> int:
        return self.integer(4)

    def u64(self) -> int:
        return self.integer(8)

    def u128(self) -> int:
        return self.integer(16)

    def i32(self) -> int:
        return int.from_bytes(self.fixed(4), "big", signed=True)

    def position(self) -> int:
        return self._offset

    def remaining(self) -> int:
        return len(self._value) - self._offset

    def bounded(self, length: int) -> bytes:
        if self.u32() != length:
            _failure()
        return self.fixed(length)

    def bounded_at_most(self, maximum: int) -> bytes:
        length = self.u32()
        if length > maximum:
            _failure()
        return self.fixed(length)

    def finish(self) -> None:
        if self._offset != len(self._value):
            _failure()


class LocalSignatureVerifier(Protocol):
    def verify_ed25519(self, public_key: bytes, signature: bytes, digest: bytes) -> bool: ...
    def verify_recoverable_secp256k1(
        self,
        public_key: bytes,
        signature: bytes,
        signature_v: int,
        signer: bytes,
        digest: bytes,
    ) -> bool: ...


@dataclass(frozen=True)
class _SequencerEffect:
    module_id: int
    ordinal: int
    event_type: int
    kind: Literal[1, 2, 3]
    monetary: bool
    transfer_set_root: bytes
    body: bytes


@dataclass(frozen=True)
class _SequencerReceipt:
    protocol_version: int
    activity_id: bytes
    global_sequence: int
    previous_state_root: bytes
    resulting_state_root: bytes
    activity_root: bytes
    result_code: int
    effects: tuple[_SequencerEffect, ...]
    fee_charged: int
    batch_id: bytes
    module_id: int
    module_version: int
    parameter_version: int
    operation: int
    asset: bytes
    amount: int
    from_account: bytes
    from_balance_before: int
    from_balance_after: int
    from_sequence: int
    to_account: bytes
    to_balance_before: int
    to_balance_after: int
    transfer_set_root: bytes
    authorization_hash: bytes
    context_hash: bytes
    timestamp: int
    sequencer_signature: bytes
    total_units: tuple[int, int] | None = None


@dataclass(frozen=True)
class AuthorizedReceiptBatch:
    batch_id: bytes
    asset: bytes
    previous_state_root: bytes
    resulting_state_root: bytes
    sequencer_public_key: bytes


@dataclass(frozen=True)
class ReceiptVerification:
    level: Literal["sequencer-signed"]
    receipt: _SequencerReceipt
    canonical_bytes: bytes
    receipt_digest: bytes


@dataclass(frozen=True)
class ReceiptKey:
    key_id: bytes
    public_key: bytes
    first_height: int
    last_height: int


SequencerReceiptRefusal = Literal[
    "unpinned_key", "key_out_of_range", "bad_signature", "hash_mismatch"
]


class SequencerReceiptVerificationError(PlatformSdkError):
    __slots__ = ("check",)

    def __init__(self, check: SequencerReceiptRefusal) -> None:
        super().__init__(SdkErrorCode.VERIFICATION_FAILURE, "never")
        self.check = check


@dataclass(frozen=True)
class SequencerReceiptVerification(ReceiptVerification):
    height: int


def _decode_sequencer_receipt(canonical_receipt: bytes) -> tuple[_SequencerReceipt, bytes]:
    if not canonical_receipt or len(canonical_receipt) > _MAX_MESSAGE_BYTES:
        _receipt_failure(ReceiptFailureCode.RECEIPT_SHAPE)
    decoder = _Decoder(canonical_receipt)
    envelope_version = decoder.u16()
    structure_tag = decoder.u16()
    if envelope_version != 2 or structure_tag not in (0x5201, 0x5202):
        _receipt_failure(ReceiptFailureCode.DECODE)
    protocol_version = decoder.u16()
    if protocol_version != envelope_version:
        _receipt_failure(ReceiptFailureCode.PROTOCOL_VERSION)
    activity_id = decoder.bounded(32)
    global_sequence = decoder.u64()
    previous_state_root = decoder.bounded(32)
    resulting_state_root = decoder.bounded(32)
    activity_root = decoder.bounded(32)
    result_code = decoder.i32()
    effect_count = decoder.u32()
    if effect_count > _MAX_EFFECTS:
        _receipt_failure(ReceiptFailureCode.DECODE)
    effects: list[_SequencerEffect] = []
    for _ in range(effect_count):
        module_id = decoder.u16()
        ordinal = decoder.u16()
        event_type = decoder.u16()
        kind_value = decoder.u8()
        if kind_value < 1 or kind_value > 3:
            _receipt_failure(ReceiptFailureCode.DECODE)
        monetary_value = decoder.u8()
        if monetary_value > 1 or (monetary_value == 1 and kind_value != 2):
            _receipt_failure(ReceiptFailureCode.DECODE)
        effects.append(_SequencerEffect(
            module_id=module_id,
            ordinal=ordinal,
            event_type=event_type,
            kind=cast(Literal[1, 2, 3], kind_value),
            monetary=monetary_value == 1,
            transfer_set_root=decoder.bounded(32),
            body=decoder.bounded_at_most(_MAX_EFFECT_BODY),
        ))
    fee_charged = decoder.u128()
    batch_id = decoder.bounded(32)
    module_id = decoder.u16()
    module_version = decoder.u32()
    parameter_version = decoder.u32()
    operation = decoder.u8()
    asset = decoder.bounded(32)
    amount = decoder.u128()
    from_account = decoder.bounded(32)
    from_balance_before = decoder.u128()
    from_balance_after = decoder.u128()
    from_sequence = decoder.u64()
    to_account = decoder.bounded(32)
    to_balance_before = decoder.u128()
    to_balance_after = decoder.u128()
    transfer_set_root = decoder.bounded(32)
    authorization_hash = decoder.bounded(32)
    context_hash = decoder.bounded(32)
    timestamp = decoder.u64()
    total_units = None
    if structure_tag == 0x5202:
        before, after = decoder.u128(), decoder.u128()
        expected = None
        if operation == 1 and before == 0:
            expected = 0
        elif 2 <= operation <= 8:
            expected = before
        elif operation == 10 and amount > 0 and before <= _MAX_U128 - amount:
            expected = before + amount
        elif operation == 11 and amount > 0 and before >= amount:
            expected = before - amount
        if module_id != 1 or result_code != 0 or not any(asset) or expected != after:
            _receipt_failure(ReceiptFailureCode.DECODE)
        total_units = (before, after)
    if global_sequence == 0:
        _receipt_failure(ReceiptFailureCode.GLOBAL_SEQUENCE)
    if module_id == 0:
        _receipt_failure(ReceiptFailureCode.MODULE_ID)
    if module_version == 0:
        _receipt_failure(ReceiptFailureCode.MODULE_VERSION)
    if timestamp == 0:
        _receipt_failure(ReceiptFailureCode.TIMESTAMP)
    if not any(activity_id):
        _receipt_failure(ReceiptFailureCode.ACTIVITY_ID)
    if not any(resulting_state_root):
        _receipt_failure(ReceiptFailureCode.RESULTING_STATE_ROOT)
    signature_flag_offset = decoder.position()
    if decoder.u8() != 1:
        _receipt_failure(ReceiptFailureCode.MISSING_SIGNATURE)
    sequencer_signature = decoder.bounded(64)
    decoder.finish()
    return (
        _SequencerReceipt(
            protocol_version=protocol_version,
            activity_id=activity_id,
            global_sequence=global_sequence,
            previous_state_root=previous_state_root,
            resulting_state_root=resulting_state_root,
            activity_root=activity_root,
            result_code=result_code,
            effects=tuple(effects),
            fee_charged=fee_charged,
            batch_id=batch_id,
            module_id=module_id,
            module_version=module_version,
            parameter_version=parameter_version,
            operation=operation,
            asset=asset,
            amount=amount,
            from_account=from_account,
            from_balance_before=from_balance_before,
            from_balance_after=from_balance_after,
            from_sequence=from_sequence,
            to_account=to_account,
            to_balance_before=to_balance_before,
            to_balance_after=to_balance_after,
            transfer_set_root=transfer_set_root,
            authorization_hash=authorization_hash,
            context_hash=context_hash,
            timestamp=timestamp,
            sequencer_signature=sequencer_signature,
            total_units=total_units,
        ),
        canonical_receipt[:signature_flag_offset] + b"\0",
    )


def verify_sequencer_receipt(
    signed_receipt: bytes,
    raw_transaction: bytes,
    receipt_keys: tuple[ReceiptKey, ...] | list[ReceiptKey],
) -> SequencerReceiptVerification:
    from cryptography.exceptions import InvalidSignature
    from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey

    canonical_bytes = bytes(signed_receipt)
    transaction_hash = keccak256(bytes(raw_transaction))
    try:
        receipt, unsigned_bytes = _decode_sequencer_receipt(canonical_bytes)
    except PlatformSdkError as error:
        raise SequencerReceiptVerificationError("bad_signature") from error
    if (
        receipt.protocol_version != 2
        or receipt.module_version != 1
        or receipt.parameter_version != 0
        or any(receipt.batch_id[:24])
    ):
        raise SequencerReceiptVerificationError("bad_signature")
    height = int.from_bytes(receipt.batch_id[24:], "big")
    pinned = [key for key in receipt_keys if _equal(key.key_id, receipt.context_hash)]
    if not pinned:
        raise SequencerReceiptVerificationError("unpinned_key")
    active = [
        key
        for key in pinned
        if type(key.first_height) is int
        and type(key.last_height) is int
        and 0 <= key.first_height <= height <= key.last_height <= 0xFFFF_FFFF_FFFF_FFFF
    ]
    if not active:
        raise SequencerReceiptVerificationError("key_out_of_range")
    receipt_digest = _digest(_RECEIPT_DOMAIN, unsigned_bytes)
    valid = False
    for key in active:
        try:
            Ed25519PublicKey.from_public_bytes(key.public_key).verify(
                receipt.sequencer_signature, receipt_digest
            )
        except (InvalidSignature, ValueError):
            continue
        valid = True
    if not valid:
        raise SequencerReceiptVerificationError("bad_signature")
    if not _equal(receipt.activity_id, transaction_hash):
        raise SequencerReceiptVerificationError("hash_mismatch")
    return SequencerReceiptVerification(
        level="sequencer-signed",
        receipt=receipt,
        canonical_bytes=canonical_bytes,
        receipt_digest=receipt_digest,
        height=height,
    )
