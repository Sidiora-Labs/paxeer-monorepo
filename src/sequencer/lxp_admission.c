#include "layerx/lxp_admission.h"

static lxp_admission_result rejected(lxp_result result)
{
    return (lxp_admission_result){ result, false, false, false };
}

lxp_result lxp_activity_check_timestamp_bound(lxp_timestamp_bound bound,
                                              uint64_t batch_timestamp,
                                              uint64_t maximum_window)
{
    if (bound.not_after < bound.not_before ||
        bound.not_after - bound.not_before > maximum_window)
        return LXP_ERR_MALFORMED_ENVELOPE;
    if (batch_timestamp < bound.not_before) return LXP_ERR_NOT_YET_VALID;
    if (batch_timestamp > bound.not_after) return LXP_ERR_EXPIRED;
    return LXP_OK;
}

lxp_admission_result lxp_admit_activity(const lxp_activity *activity,
                                        const lxp_admission_context *context)
{
    lxp_result status;
    if (activity == NULL || context == NULL)
        return rejected(LXP_ERR_MALFORMED_ENVELOPE);
    status = lxp_activity_check_envelope(activity, context->network_id);
    if (status != LXP_OK) return rejected(status);
    status = lxp_activity_check_timestamp_bound(
        activity->timestamp_bound, context->batch_timestamp,
        context->maximum_timestamp_window);
    if (status != LXP_OK) return rejected(status);
    if (!context->signature_valid) return rejected(LXP_ERR_BAD_SIGNATURE);
    if (activity->account_sequence != context->next_account_sequence)
        return rejected(activity->account_sequence < context->next_account_sequence ?
                        LXP_ERR_SEQUENCE_REUSED : LXP_ERR_SEQUENCE_GAP);
    if (context->idempotency_key_exists)
        return rejected(LXP_ERR_IDEMPOTENT_REPLAY);
    if (!context->fee_limit_spendable) return rejected(LXP_ERR_FEE_UNPAYABLE);
    return (lxp_admission_result){ LXP_OK, true, true, true };
}
