from dataclasses import dataclass
from typing import Literal, Protocol, TypeAlias

from .generated.receipt import ReceiptFailureCode
from .production import PlatformSdkError

class ReceiptVerificationError(PlatformSdkError):
    check: ReceiptFailureCode
    def __init__(self, check: ReceiptFailureCode) -> None: ...

@dataclass(frozen=True)
class MerkleProof:
    leaf_index: int
    leaf_count: int
    siblings: tuple[bytes, ...]

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

SequencerReceiptRefusal: TypeAlias = Literal[
    "unpinned_key", "key_out_of_range", "bad_signature", "hash_mismatch"
]

class SequencerReceiptVerificationError(PlatformSdkError):
    check: SequencerReceiptRefusal
    def __init__(self, check: SequencerReceiptRefusal) -> None: ...

@dataclass(frozen=True)
class SequencerReceiptVerification(ReceiptVerification):
    height: int

def verify_sequencer_receipt(
    signed_receipt: bytes,
    raw_transaction: bytes,
    receipt_keys: tuple[ReceiptKey, ...] | list[ReceiptKey],
) -> SequencerReceiptVerification: ...
