#include "layerx/lx_oracle.h"
#include "lx_perps_internal.h"
#include "layerx/lxp_kernel.h"

#include <string.h>

lxp_result lx_oracle_market_halt(lx_oracle_market *market)
{
    if (market == NULL) return LXP_ERR_NON_CANONICAL;
    market->halted = true;
    return LXP_OK;
}

bool lx_oracle_market_halted(const lx_oracle_market *market)
{
    return market == NULL || market->halted;
}

lxp_result lx_oracle_market_action_check(
    const lx_oracle_market *market, lx_oracle_market_action action)
{
    if (market == NULL || action < LX_ORACLE_ACTION_ORDER_PLACE ||
        action > LX_ORACLE_ACTION_MARGIN_ADD)
        return LXP_ERR_NON_CANONICAL;
    if (!lx_oracle_market_halted(market)) return LXP_OK;
    return action == LX_ORACLE_ACTION_ORDER_CANCEL ||
           action == LX_ORACLE_ACTION_MARGIN_ADD ?
        LXP_OK : LXP_ERR_MARKET_HALTED;
}

lxp_result lx_oracle_fail_closed_eval(lx_oracle_market *market,
                                      const lx_oracle_store *store,
                                      uint64_t batch_timestamp)
{
    const lx_oracle_accepted *latest = NULL;
    lxp_result status;
    if (market == NULL || store == NULL) return LXP_ERR_NON_CANONICAL;
    status = lx_oracle_store_latest(store, market->market_id, &latest);
    if (status != LXP_OK || latest == NULL ||
        lx_oracle_staleness_check(market, &latest->observation,
                                  batch_timestamp) != LXP_OK) {
        (void)lx_oracle_market_halt(market);
        return LXP_ERR_MARKET_HALTED;
    }
    return market->halted ? LXP_ERR_MARKET_HALTED : LXP_OK;
}

/* Canonical oracle health is separate from an administrator's market halt.
 * A valid observation can recover the former, never the latter. */
static void oracle_halt_key(const uint8_t market_id[32], uint8_t key[44])
{
    (void)memcpy(key, "oracle-halt:", 12U);
    (void)memcpy(key + 12U, market_id, 32U);
}

lxp_result lx_perps_oracle_halt_put(lxp_module_ctx *ctx,
    const uint8_t market_id[32], bool halted)
{
    uint8_t key[44];
    uint8_t value = halted ? 1U : 0U;
    if (ctx == NULL || ctx->module_id != LXP_MODULE_PERPS || market_id == NULL)
        return LXP_ERR_NON_CANONICAL;
    oracle_halt_key(market_id, key);
    return lxp_ctx_kv_put(ctx, key, sizeof(key), &value, 1U);
}

lxp_result lx_perps_oracle_halt_read(lxp_module_ctx *ctx,
    const uint8_t market_id[32], bool *halted)
{
    uint8_t key[44];
    const uint8_t *value;
    size_t length;
    size_t staged_count;
    uint16_t module_id;
    lxp_result status;
    if (ctx == NULL || ctx->kernel == NULL || market_id == NULL || halted == NULL)
        return LXP_ERR_NON_CANONICAL;
    oracle_halt_key(market_id, key);
    module_id = ctx->module_id;
    staged_count = ctx->staged_count;
    if (module_id != LXP_MODULE_PERPS) {
        ctx->module_id = LXP_MODULE_PERPS;
        ctx->staged_count = 0U;
    }
    status = lxp_ctx_kv_get(ctx, key, sizeof(key), &value, &length);
    ctx->module_id = module_id;
    ctx->staged_count = staged_count;
    /* Legacy snapshots have no persisted health halt; trading admission
     * independently requires a fresh committed observation. */
    if (status == LXP_ERR_UNKNOWN_FIELD) {
        *halted = false;
        return LXP_OK;
    }
    if (status != LXP_OK) return status;
    if (length != 1U || value[0] > 1U) return LXP_ERR_NON_CANONICAL;
    *halted = value[0] != 0U;
    return LXP_OK;
}

lxp_result lx_perps_oracle_action_check(lxp_module_ctx *ctx,
    const lx_perps_market *market, lx_oracle_market_action action)
{
    lx_oracle_market projected;
    bool oracle_halted;
    lxp_result status;
    if (market == NULL) return LXP_ERR_NON_CANONICAL;
    status = lx_perps_oracle_halt_read(ctx, market->market_id, &oracle_halted);
    if (status != LXP_OK) return status;
    (void)memset(&projected, 0, sizeof(projected));
    projected.halted = market->halted || oracle_halted;
    return lx_oracle_market_action_check(&projected, action);
}

typedef struct oracle_epoch_context {
    lxp_module_ctx *ctx;
    uint64_t timestamp;
} oracle_epoch_context;

static lxp_result oracle_epoch_market(const lx_perps_market *market, void *user)
{
    oracle_epoch_context *epoch = user;
    lx_perps_oracle_state state;
    lxp_result status = lx_perps_oracle_state_lookup(epoch->ctx,
                                                     market->market_id, &state);
    if (status != LXP_OK && status != LXP_ERR_UNKNOWN_FIELD) return status;
    if (status == LXP_ERR_UNKNOWN_FIELD || state.observed_at > epoch->timestamp ||
        epoch->timestamp - state.observed_at > market->maximum_oracle_staleness_ms)
        return lx_perps_oracle_halt_put(epoch->ctx, market->market_id, true);
    return LXP_OK;
}

lxp_result lx_perps_oracle_epoch(lxp_module_ctx *ctx, uint64_t timestamp)
{
    oracle_epoch_context epoch = { ctx, timestamp };
    return lx_perps_market_iter(ctx, oracle_epoch_market, &epoch);
}
