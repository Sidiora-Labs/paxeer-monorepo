from layerx_sdk import (
    Client,
    IdempotentMutation,
    Rung,
    SubmissionCommitted,
    SubmissionReceipted,
    SubmissionReverted,
    SubmissionState,
    VerifiedRead,
    require_verified,
)


def payment_with_verification(
    client: Client,
    request: IdempotentMutation[bytes],
) -> VerifiedRead[SubmissionState]:
    result = client.call("submit", request)
    if not isinstance(result, VerifiedRead):
        raise TypeError("invalid_submit_response")
    verified = require_verified(Rung.RECEIPTED, result)
    if isinstance(verified.value, (SubmissionReceipted, SubmissionCommitted)):
        return verified
    if isinstance(verified.value, SubmissionReverted):
        raise RuntimeError(f"payment_reverted:{verified.value.revert_reason}")
    raise RuntimeError(f"payment_not_receipted:{verified.value.kind}")
