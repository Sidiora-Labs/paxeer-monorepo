#ifndef LAYERX_LXP_EMERGENCY_STORE_H
#define LAYERX_LXP_EMERGENCY_STORE_H

#include "layerx/lxp_governance.h"
#include "layerx/lxp_kernel.h"

enum {
    LXP_GOV_EMERGENCY_HALT = 1,
    LXP_GOV_EMERGENCY_RESUME = 2,
    LXP_GOV_EMERGENCY_MODULE_ENABLE = 3,
    LXP_GOV_EMERGENCY_EVENT = 0x7150,
    LXP_GOV_EMERGENCY_EVENT_BYTES = 123
};

typedef struct lxp_gov_emergency_operation {
    uint8_t kind;
    lxp_pause_scope scope;
    uint16_t module_id;
    const uint8_t *market_id;
    const uint8_t *trigger;
    const uint8_t *exit_conditions;
    uint64_t entry_epoch;
    uint64_t ordered_sequence;
    bool enabled;
    uint32_t attempted_effect_mask;
} lxp_gov_emergency_operation;

lxp_result lxp_gov_emergency_load(lxp_module_ctx *ctx,
                                 lxp_gov_emergency_state *state);
lxp_result lxp_gov_emergency_execute(
    lxp_module_ctx *ctx, const lxp_gov_emergency_operation *operation,
    bool governance_authorized, bool ordered_governance_activity);
lxp_result lxp_gov_emergency_event_decode(
    const uint8_t *bytes, size_t length,
    lxp_gov_emergency_event *event, uint8_t *operation_kind);

#endif
