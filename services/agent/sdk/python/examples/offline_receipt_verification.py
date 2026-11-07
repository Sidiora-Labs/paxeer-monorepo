from layerx_sdk import (
    ReceiptKey,
    SequencerReceiptVerification,
    verify_sequencer_receipt,
)


def offline_receipt_verification(
    signed_receipt: bytes,
    raw_transaction: bytes,
    receipt_keys: tuple[ReceiptKey, ...] | list[ReceiptKey],
) -> SequencerReceiptVerification:
    """Verify offline using independently pinned receipt keys.

    Pass the original signed EVM transaction bytes, not its transaction hash.
    Each ReceiptKey(key_id, public_key, first_height, last_height) pins a key
    to an inclusive height range. The result proves sequencer signing only;
    committed finality requires a separate consensus proof.
    """
    return verify_sequencer_receipt(signed_receipt, raw_transaction, receipt_keys)
