#include "layerx/lxp_bridge.h"


lxp_result lxp_exit_eligibility(const lxp_exit_state *state, bool *eligible)
{
    bool liveness_expired;
    if (state == NULL || eligible == NULL || state->liveness_bound_ms == 0U ||
        state->last_finalised_at_ms == 0U ||
        state->now_ms < state->last_finalised_at_ms)
        return LXP_ERR_NON_CANONICAL;
    liveness_expired =
        state->now_ms - state->last_finalised_at_ms >=
            state->liveness_bound_ms;
    *eligible = liveness_expired || state->governance_emergency ||
        state->latest_checkpoint_fraud_accepted;
    return LXP_OK;
}

lxp_result lxp_exit_declare(lxp_exit_state *state)
{
    bool eligible = false;
    lxp_result status = lxp_exit_eligibility(state, &eligible);
    if (status != LXP_OK) return status;
    if (!eligible) return LXP_ERR_DEPOSIT_PROOF_NOT_FINAL;
    state->declared = true;
    state->discard_after_sequence = state->last_finalised_sequence;
    return LXP_OK;
}

