from layerx_sdk import (
    Client,
    IdempotentMutation,
    SubmissionCommitted,
    SubmissionReceipted,
    SubmissionReverted,
    SubmissionState,
    SubmissionSubmitted,
)


def settle_402(
    client: Client, request: IdempotentMutation[bytes]
) -> SubmissionReceipted | SubmissionCommitted:
    state = client.call("submit", request)
    if isinstance(state, (SubmissionReceipted, SubmissionCommitted)):
        return state
    if isinstance(state, SubmissionSubmitted):
        raise RuntimeError(f"settlement_not_receipted:{state.tx_hash}")
    if isinstance(state, SubmissionReverted):
        raise RuntimeError(f"settlement_reverted:{state.revert_reason}")
    kind = state.kind if isinstance(state, SubmissionState.__args__) else "Invalid"
    raise RuntimeError(f"settlement_failed:{kind}")
