#include "lxp_emergency_store.h"

#include <stdlib.h>
#include <string.h>

enum { PAUSE_BYTES = 108, HEADER_BYTES = 270 };
static const uint8_t header_key[] = "gov/emergency/v1/state";
static const uint8_t pause_prefix[] = "gov/emergency/v1/pause/";

static void put64(uint8_t *bytes, uint64_t value)
{
    size_t i;
    for (i = 0U; i < 8U; ++i)
        bytes[i] = (uint8_t)(value >> (56U - i * 8U));
}

static uint64_t get64(const uint8_t *bytes)
{
    uint64_t value = 0U;
    size_t i;
    for (i = 0U; i < 8U; ++i) value = (value << 8U) | bytes[i];
    return value;
}

static int nonzero(const uint8_t *bytes, size_t length)
{
    size_t i;
    uint8_t combined = 0U;
    for (i = 0U; i < length; ++i) combined |= bytes[i];
    return combined != 0U;
}

static void pause_key(size_t slot, uint8_t key[sizeof(pause_prefix)])
{
    (void)memcpy(key, pause_prefix, sizeof(pause_prefix) - 1U);
    key[sizeof(pause_prefix) - 1U] = (uint8_t)slot;
}

static void pause_encode(const lxp_gov_pause_record *pause,
                          uint8_t bytes[PAUSE_BYTES])
{
    bytes[0] = (uint8_t)pause->scope;
    bytes[1] = (uint8_t)(pause->module_id >> 8U);
    bytes[2] = (uint8_t)pause->module_id;
    (void)memcpy(bytes + 3U, pause->market_id, 32U);
    (void)memcpy(bytes + 35U, pause->trigger, 32U);
    (void)memcpy(bytes + 67U, pause->exit_conditions, 32U);
    put64(bytes + 99U, pause->entry_epoch);
    bytes[107] = pause->active ? 1U : 0U;
}

static lxp_result pause_decode(const uint8_t bytes[PAUSE_BYTES],
                               bool module_control,
                               lxp_gov_pause_record *pause)
{
    lxp_gov_pause_record decoded;
    (void)memset(&decoded, 0, sizeof(decoded));
    decoded.scope = (lxp_pause_scope)bytes[0];
    decoded.module_id = (uint16_t)((uint16_t)bytes[1] << 8U | bytes[2]);
    (void)memcpy(decoded.market_id, bytes + 3U, 32U);
    (void)memcpy(decoded.trigger, bytes + 35U, 32U);
    (void)memcpy(decoded.exit_conditions, bytes + 67U, 32U);
    decoded.entry_epoch = get64(bytes + 99U);
    decoded.active = bytes[107] != 0U;
    if (bytes[107] > 1U || decoded.scope < LXP_PAUSE_MODULE ||
        decoded.scope > LXP_PAUSE_NETWORK ||
        (decoded.scope == LXP_PAUSE_NETWORK ? decoded.module_id != 0U :
         decoded.module_id == 0U || decoded.module_id > LXP_MAX_GOV_MODULE_ID) ||
        (decoded.scope == LXP_PAUSE_MARKET ? !nonzero(decoded.market_id, 32U) :
         nonzero(decoded.market_id, 32U)))
        return LXP_ERR_NON_CANONICAL;
    if (module_control) {
        if (decoded.scope != LXP_PAUSE_MODULE || decoded.entry_epoch != 0U ||
            nonzero(decoded.trigger, 32U) || nonzero(decoded.exit_conditions, 32U))
            return LXP_ERR_NON_CANONICAL;
    } else if (decoded.entry_epoch == 0U) return LXP_ERR_NON_CANONICAL;
    *pause = decoded;
    return LXP_OK;
}

lxp_result lxp_gov_emergency_load(lxp_module_ctx *ctx,
                                 lxp_gov_emergency_state *state)
{
    lxp_gov_emergency_state next;
    const uint8_t *bytes;
    size_t length;
    size_t i;
    lxp_result status;
    if (ctx == NULL || ctx->kernel == NULL || state == NULL ||
        ctx->module_id != LXP_MODULE_GOVERNANCE ||
        ctx->staged_count > LXP_MODULE_MAX_STAGED_WRITES ||
        ctx->kernel->module_kv_count > LXP_KERNEL_MAX_MODULE_KV)
        return LXP_ERR_NON_CANONICAL;
    status = lxp_gov_emergency_state_init(&next);
    if (status != LXP_OK) return status;
    status = lxp_ctx_kv_get(ctx, header_key, sizeof(header_key) - 1U,
                            &bytes, &length);
    if (status == LXP_ERR_UNKNOWN_FIELD) {
        *state = next;
        return LXP_OK;
    }
    if (status != LXP_OK) return status;
    if (length != HEADER_BYTES || memcmp(bytes, "LXGE1", 5U) != 0 ||
        bytes[13] > LXP_MAX_GOV_PAUSES)
        return LXP_ERR_NON_CANONICAL;
    next.last_ordered_sequence = get64(bytes + 5U);
    next.pause_count = bytes[13];
    if (next.last_ordered_sequence == 0U) return LXP_ERR_NON_CANONICAL;
    for (i = 0U; i <= LXP_MAX_GOV_MODULE_ID; ++i) {
        if (bytes[14U + i] > 1U) return LXP_ERR_NON_CANONICAL;
        next.module_enabled[i] = bytes[14U + i] != 0U;
    }
    for (i = 0U; i < next.pause_count; ++i) {
        uint8_t key[sizeof(pause_prefix)];
        size_t j;
        pause_key(i, key);
        status = lxp_ctx_kv_get(ctx, key, sizeof(key), &bytes, &length);
        if (status != LXP_OK) return status;
        if (length != PAUSE_BYTES) return LXP_ERR_NON_CANONICAL;
        status = pause_decode(bytes, false, &next.pauses[i]);
        if (status != LXP_OK) return status;
        for (j = 0U; next.pauses[i].active && j < i; ++j)
            if (next.pauses[j].active &&
                next.pauses[j].scope == next.pauses[i].scope &&
                next.pauses[j].module_id == next.pauses[i].module_id &&
                memcmp(next.pauses[j].market_id, next.pauses[i].market_id, 32U) == 0)
                return LXP_ERR_NON_CANONICAL;
    }
    *state = next;
    return LXP_OK;
}

lxp_result lxp_gov_emergency_event_decode(
    const uint8_t *bytes, size_t length,
    lxp_gov_emergency_event *event, uint8_t *operation_kind)
{
    lxp_gov_emergency_event decoded;
    lxp_result status;
    if (bytes == NULL || event == NULL || operation_kind == NULL ||
        length != LXP_GOV_EMERGENCY_EVENT_BYTES ||
        memcmp(bytes, "LXGE1", 5U) != 0 ||
        bytes[5] < LXP_GOV_EMERGENCY_HALT ||
        bytes[5] > LXP_GOV_EMERGENCY_MODULE_ENABLE || bytes[122] > 1U)
        return LXP_ERR_NON_CANONICAL;
    (void)memset(&decoded, 0, sizeof(decoded));
    decoded.ordered_sequence = get64(bytes + 6U);
    if (decoded.ordered_sequence == 0U) return LXP_ERR_NON_CANONICAL;
    status = pause_decode(bytes + 14U,
        bytes[5] == LXP_GOV_EMERGENCY_MODULE_ENABLE, &decoded.pause);
    if (status != LXP_OK) return status;
    decoded.entered = bytes[122] != 0U;
    if ((bytes[5] == LXP_GOV_EMERGENCY_HALT &&
         (!decoded.pause.active || !decoded.entered)) ||
        (bytes[5] == LXP_GOV_EMERGENCY_RESUME &&
         (!decoded.pause.active || decoded.entered)) ||
        (bytes[5] == LXP_GOV_EMERGENCY_MODULE_ENABLE &&
         decoded.pause.active != decoded.entered))
        return LXP_ERR_NON_CANONICAL;
    *event = decoded;
    *operation_kind = bytes[5];
    return LXP_OK;
}

static lxp_result operation_apply(lxp_gov_emergency_state *state,
                                  const lxp_gov_emergency_operation *operation,
                                  bool authorized, bool ordered_activity)
{
    if (operation->kind == LXP_GOV_EMERGENCY_HALT)
        return lxp_gov_emergency_halt(state, operation->scope,
            operation->module_id, operation->market_id, operation->trigger,
            operation->exit_conditions, operation->entry_epoch,
            operation->ordered_sequence, authorized, ordered_activity);
    if (operation->kind == LXP_GOV_EMERGENCY_RESUME)
        return lxp_gov_emergency_resume(state, operation->scope,
            operation->module_id, operation->market_id,
            operation->ordered_sequence, authorized, ordered_activity);
    if (operation->kind == LXP_GOV_EMERGENCY_MODULE_ENABLE)
        return lxp_gov_module_enable(state, operation->module_id,
            operation->enabled, operation->attempted_effect_mask,
            operation->ordered_sequence, authorized, ordered_activity);
    return LXP_ERR_NON_CANONICAL;
}

lxp_result lxp_gov_emergency_execute(
    lxp_module_ctx *ctx, const lxp_gov_emergency_operation *operation,
    bool governance_authorized, bool ordered_governance_activity)
{
    lxp_gov_emergency_state before;
    lxp_gov_emergency_state after;
    lxp_module_ctx *staged;
    lxp_effect_buffer *effects;
    lxp_effect_buffer *published;
    uint8_t header[HEADER_BYTES];
    uint8_t event[LXP_GOV_EMERGENCY_EVENT_BYTES];
    size_t i;
    lxp_result status;
    if (ctx == NULL || ctx->kernel == NULL || operation == NULL ||
        !ctx->mutable || ctx->commit_prepared || ctx->effects == NULL ||
        ctx->effects->count > LXP_MAX_EFFECTS ||
        ctx->module_id != LXP_MODULE_GOVERNANCE)
        return LXP_ERR_NON_CANONICAL;
    status = lxp_gov_emergency_load(ctx, &before);
    if (status != LXP_OK) return status;
    after = before;
    status = operation_apply(&after, operation, governance_authorized,
                              ordered_governance_activity);
    if (status != LXP_OK) return status;
    staged = malloc(sizeof(*staged));
    effects = malloc(sizeof(*effects));
    if (staged == NULL || effects == NULL) {
        free(staged);
        free(effects);
        return LXP_ERR_ARENA_EXHAUSTED;
    }
    *staged = *ctx;
    *effects = *ctx->effects;
    staged->effects = effects;
    (void)memcpy(header, "LXGE1", 5U);
    put64(header + 5U, after.last_ordered_sequence);
    header[13] = (uint8_t)after.pause_count;
    for (i = 0U; i <= LXP_MAX_GOV_MODULE_ID; ++i)
        header[14U + i] = after.module_enabled[i] ? 1U : 0U;
    status = lxp_ctx_kv_put(staged, header_key, sizeof(header_key) - 1U,
                            header, sizeof(header));
    for (i = 0U; status == LXP_OK && i < after.pause_count; ++i) {
        uint8_t key[sizeof(pause_prefix)];
        uint8_t bytes[PAUSE_BYTES];
        if (i < before.pause_count &&
            memcmp(&before.pauses[i], &after.pauses[i], sizeof(after.pauses[i])) == 0)
            continue;
        pause_key(i, key);
        pause_encode(&after.pauses[i], bytes);
        status = lxp_ctx_kv_put(staged, key, sizeof(key), bytes, sizeof(bytes));
    }
    (void)memcpy(event, "LXGE1", 5U);
    event[5] = operation->kind;
    put64(event + 6U, after.events[0].ordered_sequence);
    pause_encode(&after.events[0].pause, event + 14U);
    event[122] = after.events[0].entered ? 1U : 0U;
    if (status == LXP_OK)
        status = lxp_ctx_emit_event(staged, LXP_GOV_EMERGENCY_EVENT,
                                     event, sizeof(event));
    if (status == LXP_OK) {
        published = ctx->effects;
        *ctx = *staged;
        ctx->effects = published;
        *published = *effects;
    }
    free(effects);
    free(staged);
    return status;
}
