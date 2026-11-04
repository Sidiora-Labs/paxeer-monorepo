#include "layerx/lx_perps.h"
#include "lx_perps_internal.h"
#include "lx_perps_codec.h"

#include "layerx/lxp_kernel.h"

#include <string.h>

static lxp_result position_notional(const lx_perps_position *position,
                                    lxp_u128 mark_price,
                                    lxp_u128 price_scale,
                                    lxp_u128 *notional)
{
    lxp_u128 remainder;
    lxp_result status;
    if (position == NULL || notional == NULL ||
        lxp_u128_is_zero(mark_price) || lxp_u128_is_zero(price_scale) ||
        lxp_u128_is_zero(position->size))
        return LXP_ERR_NON_CANONICAL;
    status = lxp_u128_mul_div_floor(mark_price, position->size, price_scale,
                                    notional, &remainder);
    return status == LXP_OK ? LXP_OK : LXP_ERR_OVERFLOW;
}

lxp_result lx_perps_position_value(const lx_perps_position *position,
    lxp_u128 price, lxp_u128 scale, lxp_i128 *pnl)
{
    lxp_u128 marked;
    lxp_u128 remainder;
    lxp_i128 entry;
    lxp_i128 exit_value;
    if (position == NULL || pnl == NULL ||
        (position->side != LX_PERPS_SIDE_BUY &&
         position->side != LX_PERPS_SIDE_SELL) ||
        lxp_u128_is_zero(position->size) || lxp_u128_is_zero(price) ||
        lxp_u128_is_zero(scale))
        return LXP_ERR_NON_CANONICAL;
    if (lxp_u128_mul_div_floor(price, position->size, scale, &marked,
                               &remainder) != LXP_OK)
        return LXP_ERR_OVERFLOW;
    /* A short buys back at the rounded-up quote cost. */
    if (position->side == LX_PERPS_SIDE_SELL &&
        !lxp_u128_is_zero(remainder) &&
        lxp_u128_add(marked, (lxp_u128){0U, 1U}, &marked) != LXP_OK)
        return LXP_ERR_OVERFLOW;
    entry = (lxp_i128){false, position->entry_notional};
    exit_value = (lxp_i128){false, marked};
    return position->side == LX_PERPS_SIDE_BUY ?
        lxp_i128_sub(exit_value, entry, pnl) :
        lxp_i128_sub(entry, exit_value, pnl);
}

lxp_result lx_perps_equity_check(const lx_perps_market *market,
    const lx_perps_position *position, lxp_u128 price, lxp_u128 scale,
    lxp_u128 cash, lxp_i128 funding_index, lxp_i128 *pnl,
    lxp_i128 *owed, lxp_i128 *equity, bool *liquidatable)
{
    lxp_u128 notional;
    lxp_u128 required;
    lxp_result status;
    if (market == NULL || position == NULL || pnl == NULL || owed == NULL ||
        equity == NULL || liquidatable == NULL ||
        market->maintenance_margin_ratio_bps == 0U ||
        market->maintenance_margin_ratio_bps > LXP_BASIS_POINTS_ONE)
        return LXP_ERR_NON_CANONICAL;
    status = position_notional(position, price, scale, &notional);
    if (status == LXP_OK)
        status = lxp_u128_mul_bps_ceil(notional,
            market->maintenance_margin_ratio_bps, &required);
    if (status == LXP_OK)
        status = lx_perps_position_value(position, price, scale, pnl);
    if (status == LXP_OK)
        status = lx_perps_funding_owed(position, funding_index, owed);
    if (status == LXP_OK)
        status = lxp_i128_add((lxp_i128){false, cash}, *pnl, equity);
    if (status == LXP_OK) status = lxp_i128_sub(*equity, *owed, equity);
    if (status != LXP_OK) return status;
    *liquidatable = equity->negative ||
        lxp_u128_cmp(equity->magnitude, required) < 0;
    return LXP_OK;
}

lxp_result lx_perps_maintenance_check(const lx_perps_market *market,
    const lx_perps_position *position, lxp_u128 mark_price,
    lxp_u128 price_scale, lxp_u128 margin_balance, bool *liquidatable)
{
    lxp_i128 pnl;
    lxp_i128 owed;
    lxp_i128 equity;
    if (position == NULL) return LXP_ERR_NON_CANONICAL;
    return lx_perps_equity_check(market, position, mark_price, price_scale,
        margin_balance, position->funding_index_at_entry, &pnl, &owed,
        &equity, liquidatable);
}

lxp_result lx_perps_liquidation_fee_split(lxp_u128 total_fee,
                                          uint32_t liquidator_share_bps,
                                          lxp_u128 *liquidator_fee,
                                          lxp_u128 *insurance_fee)
{
    lxp_result status;
    if (liquidator_fee == NULL || insurance_fee == NULL ||
        liquidator_share_bps > LXP_BASIS_POINTS_ONE)
        return LXP_ERR_NON_CANONICAL;
    status = lxp_u128_mul_bps_ceil(
        total_fee, LXP_BASIS_POINTS_ONE - liquidator_share_bps,
        insurance_fee);
    if (status != LXP_OK) return LXP_ERR_OVERFLOW;
    status = lxp_u128_sub(total_fee, *insurance_fee, liquidator_fee);
    return status == LXP_OK ? LXP_OK : LXP_ERR_OVERFLOW;
}

static lxp_result append_leg(lxp_transfer_set *set, lx_account *from,
                             lx_account *to, const uint8_t asset_id[32],
                             lxp_u128 amount, uint16_t reason)
{
    lxp_transfer_leg *leg;
    if (lxp_u128_is_zero(amount)) return LXP_OK;
    if (set->leg_count == LXP_MAX_TRANSFER_SET_LEGS)
        return LXP_ERR_TOO_MANY_LEGS;
    leg = &set->legs[set->leg_count++];
    (void)memset(leg, 0, sizeof(*leg));
    leg->from = from;
    leg->to = to;
    (void)memcpy(leg->asset_id, asset_id, 32U);
    leg->amount = amount;
    leg->reason = reason;
    leg->supply_mode = LXP_TRANSFER_CONSERVED;
    return LXP_OK;
}

lxp_result lx_perps_liquidation_legs_build(
    const lx_perps_liquidation_request *request, lxp_transfer_set *set,
    lxp_transfer_source_authority *authorities, size_t capacity,
    size_t *authority_count)
{
    size_t index;
    lxp_u128 notional;
    lxp_u128 total_fee;
    lxp_u128 loss_from_margin;
    lxp_u128 deficit;
    lxp_u128 remaining;
    lxp_u128 fee_from_margin;
    lxp_u128 liquidator_fee;
    lxp_u128 insurance_fee;
    lxp_result status;
    if (request == NULL || set == NULL || authorities == NULL ||
        authority_count == NULL || capacity == 0U ||
        request->position == NULL ||
        request->market == NULL || request->margin_account == NULL ||
        request->market_liquidity_account == NULL ||
        request->liquidator_main_account == NULL ||
        request->insurance_account == NULL ||
        request->owner_main_account == NULL || request->asset == NULL ||
        request->liquidation_fee_bps > LXP_BASIS_POINTS_ONE)
        return LXP_ERR_NON_CANONICAL;
    status = position_notional(request->position, request->mark_price,
                               request->price_scale, &notional);
    if (status != LXP_OK) return status;
    status = lxp_u128_mul_bps_ceil(notional, request->liquidation_fee_bps,
                                   &total_fee);
    if (status != LXP_OK) return LXP_ERR_OVERFLOW;
    loss_from_margin = lxp_u128_cmp(request->trading_loss,
                                    request->margin_account->balance) < 0 ?
        request->trading_loss : request->margin_account->balance;
    status = lxp_u128_sub(request->margin_account->balance, loss_from_margin,
                          &remaining);
    if (status != LXP_OK) return LXP_FATAL_INVARIANT;
    status = lxp_u128_sub(request->trading_loss, loss_from_margin, &deficit);
    if (status != LXP_OK) return LXP_FATAL_INVARIANT;
    if (lxp_u128_cmp(deficit, request->insurance_account->balance) > 0)
        return LXP_ERR_INSUFFICIENT_BALANCE;
    fee_from_margin = lxp_u128_cmp(total_fee, remaining) < 0 ?
        total_fee : remaining;
    status = lxp_u128_sub(remaining, fee_from_margin, &remaining);
    if (status != LXP_OK) return LXP_FATAL_INVARIANT;
    status = lx_perps_liquidation_fee_split(
        fee_from_margin, request->liquidator_share_bps,
        &liquidator_fee, &insurance_fee);
    if (status != LXP_OK) return status;
    (void)memset(set, 0, sizeof(*set));
    status = append_leg(set, request->margin_account,
                        request->market_liquidity_account,
                        request->asset->asset_id, loss_from_margin,
                        LXP_REASON_TRADING_LOSS);
    if (status == LXP_OK)
        status = append_leg(set, request->margin_account,
                            request->liquidator_main_account,
                            request->asset->asset_id, liquidator_fee,
                            LXP_REASON_LIQUIDATION_FEE);
    if (status == LXP_OK)
        status = append_leg(set, request->margin_account,
                            request->insurance_account,
                            request->asset->asset_id, insurance_fee,
                            LXP_REASON_LIQUIDATION_FEE);
    if (status == LXP_OK)
        status = append_leg(set, request->insurance_account,
                            request->market_liquidity_account,
                            request->asset->asset_id, deficit,
                            LXP_REASON_INSURANCE);
    if (status == LXP_OK)
        status = append_leg(set, request->margin_account,
                            request->owner_main_account,
                            request->asset->asset_id, remaining,
                            LXP_REASON_MARGIN_RELEASE);
    if (status != LXP_OK) return status;
    if (set->leg_count == 0U) return LXP_ERR_ZERO_AMOUNT;
    set->context = request->context;
    set->context.assets = request->asset;
    set->context.asset_count = 1U;
    set->context.protocol_system_capability = true;
    set->context.debit_authority_kind = LXP_AUTH_PROTOCOL_MODULE;
    (void)memcpy(set->context.authorized_from,
                 request->margin_account->id, 32U);
    *authority_count = 0U;
    for (index = 0U; index < set->leg_count; ++index) {
        status = lx_perps_source_authority_add(
            authorities, capacity, authority_count,
            set->legs[index].from->id, LXP_AUTH_PROTOCOL_MODULE, true);
        if (status != LXP_OK) return status;
    }
    set->context.source_authorities = authorities;
    set->context.source_authority_count = *authority_count;
    return LXP_OK;
}

static lxp_u128 minimum(lxp_u128 left, lxp_u128 right)
{
    return lxp_u128_cmp(left, right) < 0 ? left : right;
}

static lxp_result liquidation_debt_record(lxp_module_ctx *ctx,
    const lx_perps_liquidation_request *request, lx_account *pool,
    lxp_u128 trading_debt, lxp_u128 funding_debt)
{
    lxp_result status;
    if (!lxp_u128_is_zero(trading_debt)) {
        lx_perps_deficit deficit;
        status = lx_perps_deficit_lookup(ctx, request->market->market_id,
                                         &deficit);
        if (status == LXP_ERR_UNKNOWN_FIELD) {
            (void)memset(&deficit, 0, sizeof(deficit));
            (void)memcpy(deficit.market_id, request->market->market_id, 32U);
            (void)memcpy(deficit.insurance_account_id,
                         request->insurance_account->id, 32U);
        } else if (status != LXP_OK) return status;
        if (memcmp(deficit.insurance_account_id,
                   request->insurance_account->id, 32U) != 0)
            return LXP_ERR_NON_CANONICAL;
        if (lxp_u128_add(deficit.amount, trading_debt, &deficit.amount) != LXP_OK)
            return LXP_ERR_OVERFLOW;
        deficit.recorded_at_sequence = lxp_ctx_global_sequence(ctx);
        status = lx_perps_deficit_put(ctx, &deficit);
        if (status != LXP_OK) return status;
    }
    if (!lxp_u128_is_zero(funding_debt)) {
        static const uint8_t prefix[] = "funding-debt:";
        uint8_t key[sizeof(prefix) - 1U + 64U];
        uint8_t value[16];
        const uint8_t *previous;
        size_t length;
        lxp_u128 amount = {0U, 0U};
        (void)memcpy(key, prefix, sizeof(prefix) - 1U);
        (void)memcpy(key + sizeof(prefix) - 1U,
                     request->market->market_id, 32U);
        (void)memcpy(key + sizeof(prefix) - 1U + 32U, pool->id, 32U);
        status = lxp_ctx_kv_get(ctx, key, sizeof(key), &previous, &length);
        if (status == LXP_OK) {
            if (length != sizeof(value)) return LXP_ERR_NON_CANONICAL;
            status = lxp_u128_from_be(previous, &amount);
        } else if (status == LXP_ERR_UNKNOWN_FIELD) status = LXP_OK;
        if (status != LXP_OK) return status;
        if (lxp_u128_add(amount, funding_debt, &amount) != LXP_OK)
            return LXP_ERR_OVERFLOW;
        status = lxp_u128_to_be(amount, value);
        if (status == LXP_OK)
            status = lxp_ctx_kv_put(ctx, key, sizeof(key), value, sizeof(value));
        if (status != LXP_OK) return status;
    }
    return LXP_OK;
}

static lxp_result liquidation_debit(lxp_transfer_set *set,
    const lx_perps_liquidation_request *request, lx_account *recipient,
    uint16_t reason, lxp_u128 debt, lxp_u128 *cash,
    lxp_u128 *insurance, lxp_u128 *unpaid)
{
    lxp_u128 margin_part = minimum(debt, *cash);
    lxp_u128 insurance_part;
    lxp_result status;
    if (lxp_u128_sub(debt, margin_part, &debt) != LXP_OK ||
        lxp_u128_sub(*cash, margin_part, cash) != LXP_OK)
        return LXP_FATAL_INVARIANT;
    insurance_part = minimum(debt, *insurance);
    if (lxp_u128_sub(debt, insurance_part, unpaid) != LXP_OK ||
        lxp_u128_sub(*insurance, insurance_part, insurance) != LXP_OK)
        return LXP_FATAL_INVARIANT;
    status = append_leg(set, request->margin_account, recipient,
        request->asset->asset_id, margin_part, reason);
    if (status == LXP_OK)
        status = append_leg(set, request->insurance_account, recipient,
            request->asset->asset_id, insurance_part, LXP_REASON_INSURANCE);
    return status;
}

lxp_result lx_perps_liquidation_settle(lxp_module_ctx *ctx,
    const lx_perps_liquidation_request *request, lxp_i128 funding_index,
    lxp_u128 *covered, lxp_receipt *receipt)
{
    lxp_transfer_set set;
    lxp_transfer_source_authority authorities[4];
    size_t authority_count = 0U;
    size_t i;
    lx_account *pool;
    lxp_i128 pnl;
    lxp_i128 owed;
    lxp_i128 equity;
    lxp_u128 cash;
    lxp_u128 insurance;
    lxp_u128 trading_debt = {0U, 0U};
    lxp_u128 funding_debt = {0U, 0U};
    lxp_u128 notional;
    lxp_u128 fee;
    lxp_u128 liquidator_fee;
    lxp_u128 insurance_fee;
    bool liquidatable;
    lxp_result status;
    if (ctx == NULL || request == NULL || request->position == NULL ||
        request->market == NULL || request->margin_account == NULL ||
        request->market_liquidity_account == NULL ||
        request->insurance_account == NULL || request->asset == NULL ||
        request->owner_main_account == NULL ||
        request->liquidator_main_account == NULL || covered == NULL ||
        receipt == NULL || ctx->module_id != LXP_MODULE_PERPS)
        return LXP_ERR_NON_CANONICAL;
    if (request->market->halted) return LXP_ERR_MARKET_HALTED;
    if (!request->position->open) return LXP_ERR_AGREEMENT_STATE;
    status = lx_perps_equity_check(request->market, request->position,
        request->mark_price, request->price_scale,
        request->margin_account->balance, funding_index, &pnl, &owed,
        &equity, &liquidatable);
    if (status != LXP_OK) return status;
    if (!liquidatable) return LXP_ERR_MARGIN_INSUFFICIENT;
    status = lxp_ctx_account_find(ctx,
        request->position->side == LX_PERPS_SIDE_BUY ?
        request->market->long_funding_account_id :
        request->market->short_funding_account_id, &pool);
    if (status != LXP_OK) return status;
    if (pool->kind != (request->position->side == LX_PERPS_SIDE_BUY ?
        LX_ACCOUNT_SYSTEM_FUNDING_LONG : LX_ACCOUNT_SYSTEM_FUNDING_SHORT))
        return LXP_ERR_UNKNOWN_ACCOUNT_NAMESPACE;
    (void)memset(&set, 0, sizeof(set));
    (void)memset(authorities, 0, sizeof(authorities));
    cash = request->margin_account->balance;
    insurance = request->insurance_account->balance;
    if (!pnl.negative && !lxp_u128_is_zero(pnl.magnitude)) {
        status = append_leg(&set, request->market_liquidity_account,
            request->margin_account, request->asset->asset_id,
            pnl.magnitude, LXP_REASON_TRADING_PROFIT);
        if (status == LXP_OK) status = lxp_u128_add(cash, pnl.magnitude, &cash);
        if (status != LXP_OK) return status;
    }
    if (owed.negative) {
        status = append_leg(&set, pool, request->margin_account,
            request->asset->asset_id, owed.magnitude, LXP_REASON_FUNDING);
        if (status == LXP_OK) status = lxp_u128_add(cash, owed.magnitude, &cash);
        if (status != LXP_OK) return status;
    } else {
        status = liquidation_debit(&set, request, pool, LXP_REASON_FUNDING,
            owed.magnitude, &cash, &insurance, &funding_debt);
        if (status != LXP_OK) return status;
    }
    if (pnl.negative) {
        status = liquidation_debit(&set, request,
            request->market_liquidity_account, LXP_REASON_TRADING_LOSS,
            pnl.magnitude, &cash, &insurance, &trading_debt);
        if (status != LXP_OK) return status;
    }
    status = position_notional(request->position, request->mark_price,
                                request->price_scale, &notional);
    if (status == LXP_OK)
        status = lxp_u128_mul_bps_ceil(notional,
            request->liquidation_fee_bps, &fee);
    if (status != LXP_OK) return status;
    fee = minimum(fee, cash);
    status = lx_perps_liquidation_fee_split(fee,
        request->liquidator_share_bps, &liquidator_fee, &insurance_fee);
    if (status == LXP_OK) status = lxp_u128_sub(cash, fee, &cash);
    if (status == LXP_OK)
        status = append_leg(&set, request->margin_account,
            request->liquidator_main_account, request->asset->asset_id,
            liquidator_fee, LXP_REASON_LIQUIDATION_FEE);
    if (status == LXP_OK)
        status = append_leg(&set, request->margin_account,
            request->insurance_account, request->asset->asset_id,
            insurance_fee, LXP_REASON_LIQUIDATION_FEE);
    if (status == LXP_OK)
        status = append_leg(&set, request->margin_account,
            request->owner_main_account, request->asset->asset_id,
            cash, LXP_REASON_MARGIN_RELEASE);
    if (status != LXP_OK) return status;
    set.context = request->context;
    set.context.assets = request->asset;
    set.context.asset_count = 1U;
    set.context.protocol_system_capability = true;
    set.context.debit_authority_kind = LXP_AUTH_PROTOCOL_MODULE;
    for (i = 0U; i < set.leg_count; ++i) {
        status = lx_perps_source_authority_add(authorities, 4U,
            &authority_count, set.legs[i].from->id,
            LXP_AUTH_PROTOCOL_MODULE, true);
        if (status != LXP_OK) return status;
    }
    set.context.source_authorities = authorities;
    set.context.source_authority_count = authority_count;
    if (set.leg_count != 0U) {
        (void)memcpy(set.context.authorized_from, set.legs[0].from->id, 32U);
        status = lxp_ctx_emit_transfer_set(ctx, &set, receipt);
        if (status != LXP_OK) return status;
    }
    if (!lxp_u128_is_zero(request->margin_account->balance))
        return LXP_FATAL_INVARIANT;
    status = liquidation_debt_record(ctx, request, pool, trading_debt,
                                     funding_debt);
    if (status != LXP_OK) return status;
    *covered = (lxp_u128){0U, 0U};
    if (pnl.negative &&
        lxp_u128_sub(pnl.magnitude, trading_debt, covered) != LXP_OK)
        return LXP_FATAL_INVARIANT;
    request->position->open = false;
    request->margin_account->has_open_reference = false;
    return LXP_OK;
}

lxp_result lx_perps_liquidate_execute(lxp_module_ctx *ctx,
    const lx_perps_liquidation_request *request, lxp_receipt *receipt)
{
    lx_perps_funding_state funding;
    lxp_u128 covered;
    lxp_result status;
    if (request == NULL || request->market == NULL)
        return LXP_ERR_NON_CANONICAL;
    status = lx_perps_funding_state_lookup(ctx, request->market->market_id,
                                           &funding);
    if (status != LXP_OK) return status;
    return lx_perps_liquidation_settle(ctx, request, funding.funding_index,
                                       &covered, receipt);
}
