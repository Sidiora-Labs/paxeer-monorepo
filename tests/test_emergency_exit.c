#include "layerx/lxp_bridge.h"

#include <string.h>

int main(void)
{
    lxp_exit_state exit_state;
    bool eligible = false;

    (void)memset(&exit_state, 0, sizeof(exit_state));
    exit_state.now_ms = 150U;
    exit_state.last_finalised_at_ms = 100U;
    exit_state.liveness_bound_ms = 100U;
    exit_state.last_finalised_sequence = 900U;
    if (lxp_exit_eligibility(&exit_state, &eligible) != LXP_OK || eligible ||
        lxp_exit_declare(&exit_state) != LXP_ERR_DEPOSIT_PROOF_NOT_FINAL)
        return 1;
    exit_state.now_ms = 200U;
    if (lxp_exit_declare(&exit_state) != LXP_OK || !exit_state.declared ||
        exit_state.discard_after_sequence != 900U) return 1;
    exit_state.declared = false;
    exit_state.now_ms = 101U;
    exit_state.governance_emergency = true;
    if (lxp_exit_declare(&exit_state) != LXP_OK) return 1;
    exit_state.governance_emergency = false;
    exit_state.latest_checkpoint_fraud_accepted = true;
    if (lxp_exit_eligibility(&exit_state, &eligible) != LXP_OK || !eligible)
        return 1;

    return 0;
}
