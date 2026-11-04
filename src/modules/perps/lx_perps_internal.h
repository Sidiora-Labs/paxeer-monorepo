#ifndef LX_PERPS_INTERNAL_H
#define LX_PERPS_INTERNAL_H

#include "layerx/lx_perps.h"
#include "layerx/lx_oracle.h"

lxp_result lx_perps_position_value(const lx_perps_position *position,
    lxp_u128 price, lxp_u128 scale, lxp_i128 *pnl);
lxp_result lx_perps_equity_check(const lx_perps_market *market,
    const lx_perps_position *position, lxp_u128 price, lxp_u128 scale,
    lxp_u128 cash, lxp_i128 funding_index, lxp_i128 *pnl,
    lxp_i128 *owed, lxp_i128 *equity, bool *liquidatable);
lxp_result lx_perps_liquidation_settle(lxp_module_ctx *ctx,
    const lx_perps_liquidation_request *request, lxp_i128 funding_index,
    lxp_u128 *covered, lxp_receipt *receipt);
lxp_result lx_perps_margin_encumbered(lxp_module_ctx *ctx,
    const uint8_t margin_id[32], const uint8_t *closing_market_id,
    bool *encumbered);

lxp_result lx_perps_oracle_transport_encode(const lx_perps_oracle_command *command,
    uint8_t bytes[LX_ORACLE_TRANSPORT_BYTES]);
lxp_result lx_perps_oracle_admit(lxp_module_ctx *ctx,
    const lx_perps_oracle_command *command, const uint8_t *payload,
    size_t payload_length);
lxp_result lx_perps_oracle_accept(lxp_module_ctx *ctx,
    const lx_perps_oracle_state *state);
lxp_result lx_perps_oracle_halt_put(lxp_module_ctx *ctx,
    const uint8_t market_id[32], bool halted);
lxp_result lx_perps_oracle_halt_read(lxp_module_ctx *ctx,
    const uint8_t market_id[32], bool *halted);
lxp_result lx_perps_oracle_action_check(lxp_module_ctx *ctx,
    const lx_perps_market *market, lx_oracle_market_action action);
lxp_result lx_perps_oracle_epoch(lxp_module_ctx *ctx, uint64_t timestamp);

#endif
