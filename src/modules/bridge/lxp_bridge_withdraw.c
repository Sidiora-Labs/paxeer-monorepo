#include "layerx/lxp_bridge.h"

#include "layerx/lxp_crypto.h"
#include "layerx/lxp_hash.h"

#include <string.h>

lxp_result lxp_withdrawal_nullifier(const lx_withdrawal_request *request,
                                    uint8_t nullifier[32])
{
    static const uint8_t tag[] = "LX:WITHDRAWAL:v1";
    uint8_t input[sizeof(tag) - 1U + 4U + 32U * 4U + 16U];
    uint8_t amount[16];
    size_t cursor = 0U;
    lxp_result status;
    if (request == NULL || nullifier == NULL || request->network_id == 0U ||
        lxp_ct_is_zero(request->withdrawal_id, 32U) ||
        lxp_ct_is_zero(request->account_id, 32U) ||
        lxp_ct_is_zero(request->asset_id, 32U) ||
        lxp_ct_is_zero(request->checkpoint_id, 32U) ||
        lxp_u128_is_zero(request->amount)) return LXP_ERR_NON_CANONICAL;
    status = lxp_u128_to_be(request->amount, amount);
    if (status != LXP_OK) return status;
    (void)memcpy(input + cursor, tag, sizeof(tag) - 1U);
    cursor += sizeof(tag) - 1U;
    input[cursor++] = (uint8_t)(request->network_id >> 24U);
    input[cursor++] = (uint8_t)(request->network_id >> 16U);
    input[cursor++] = (uint8_t)(request->network_id >> 8U);
    input[cursor++] = (uint8_t)request->network_id;
    (void)memcpy(input + cursor, request->withdrawal_id, 32U);
    cursor += 32U;
    (void)memcpy(input + cursor, request->account_id, 32U);
    cursor += 32U;
    (void)memcpy(input + cursor, request->asset_id, 32U);
    cursor += 32U;
    (void)memcpy(input + cursor, amount, sizeof(amount));
    cursor += sizeof(amount);
    (void)memcpy(input + cursor, request->checkpoint_id, 32U);
    cursor += 32U;
    return lxp_hash_sha256(input, cursor, nullifier);
}

lxp_result lxp_withdrawal_leaf(const lx_withdrawal_request *request,
                               uint8_t leaf_hash[32])
{
    uint8_t canonical[32U * 4U + 16U];
    uint8_t amount[16];
    size_t cursor = 0U;
    lxp_result status;
    if (request == NULL || leaf_hash == NULL ||
        lxp_ct_is_zero(request->withdrawal_id, 32U) ||
        lxp_ct_is_zero(request->account_id, 32U) ||
        lxp_ct_is_zero(request->asset_id, 32U) ||
        lxp_ct_is_zero(request->payout_recipient, 32U) ||
        lxp_u128_is_zero(request->amount)) return LXP_ERR_NON_CANONICAL;
    status = lxp_u128_to_be(request->amount, amount);
    if (status != LXP_OK) return status;
    (void)memcpy(canonical + cursor, request->withdrawal_id, 32U);
    cursor += 32U;
    (void)memcpy(canonical + cursor, request->account_id, 32U);
    cursor += 32U;
    (void)memcpy(canonical + cursor, request->asset_id, 32U);
    cursor += 32U;
    (void)memcpy(canonical + cursor, amount, sizeof(amount));
    cursor += sizeof(amount);
    (void)memcpy(canonical + cursor, request->payout_recipient, 32U);
    cursor += 32U;
    return lxp_merkle_leaf_hash(canonical, cursor, leaf_hash);
}

lxp_result lxp_bridge_withdraw_request(
    lxp_module_ctx *ctx,
    const lx_asset_transfer_request *transfer,
    const lx_withdrawal_request *withdrawal,
    lx_withdrawal_store *store,
    lxp_receipt *receipt)
{
    if (ctx == NULL || transfer == NULL || withdrawal == NULL ||
        store == NULL || receipt == NULL || transfer->from == NULL ||
        transfer->to == NULL ||
        lxp_ct_is_zero(withdrawal->payout_recipient, 32U) ||
        transfer->from->kind != LX_ACCOUNT_AGENT_MAIN ||
        transfer->to->kind != LX_ACCOUNT_SYSTEM_PAXEER_WITHDRAWALS)
        return LXP_ERR_GRANT_SCOPE_VIOLATION;
    return lx_asset_withdraw_request(ctx, transfer, withdrawal, store, receipt);
}

lxp_result lxp_paxeer_challenge_window(
    lxp_challenge_window_state *window,
    uint64_t now_ms,
    lxp_challenge_outcome resolution,
    size_t attesting_guarantor_count)
{
    if (window == NULL || window->opened_at_ms >= window->closes_at_ms ||
        now_ms < window->opened_at_ms ||
        lxp_ct_is_zero(window->checkpoint_id, 32U) ||
        resolution > LXP_CHALLENGE_FAILED)
        return LXP_ERR_NON_CANONICAL;
    if (resolution == LXP_CHALLENGE_PENDING) {
        if (now_ms > window->closes_at_ms ||
            window->outcome != LXP_CHALLENGE_NONE)
            return LXP_ERR_DISPUTE_WINDOW_CLOSED;
        window->outcome = LXP_CHALLENGE_PENDING;
    } else if (resolution == LXP_CHALLENGE_SUCCEEDED) {
        if (window->outcome != LXP_CHALLENGE_PENDING ||
            attesting_guarantor_count == 0U)
            return LXP_ERR_NON_CANONICAL;
        window->outcome = LXP_CHALLENGE_SUCCEEDED;
        window->payouts_cancelled = true;
        window->slashed_attester_count = attesting_guarantor_count;
    } else if (resolution == LXP_CHALLENGE_FAILED) {
        if (window->outcome != LXP_CHALLENGE_PENDING)
            return LXP_ERR_NON_CANONICAL;
        window->outcome = LXP_CHALLENGE_FAILED;
    }
    if (window->payouts_cancelled ||
        window->outcome == LXP_CHALLENGE_SUCCEEDED)
        return LXP_ERR_WITHDRAWAL_CANCELLED;
    if (window->outcome == LXP_CHALLENGE_PENDING ||
        now_ms < window->closes_at_ms)
        return LXP_ERR_CHALLENGE_WINDOW_OPEN;
    return LXP_OK;
}

