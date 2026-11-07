#include "lx_asset_history.h"
#include "layerx/lxp_bridge.h"

#include <string.h>

static void store_u32(uint8_t out[4], uint32_t value)
{
    out[0] = (uint8_t)(value >> 24U);
    out[1] = (uint8_t)(value >> 16U);
    out[2] = (uint8_t)(value >> 8U);
    out[3] = (uint8_t)value;
}

lxp_result lx_asset_historical_withdrawal_nullifier_v1(
    const lx_withdrawal_request *request, uint8_t nullifier[32])
{
    return lxp_withdrawal_nullifier(request, nullifier);
}

lxp_result lx_asset_historical_deposit_nullifier_v1(
    const lx_deposit_proof *proof, uint8_t nullifier[32])
{
    return lxp_deposit_nullifier(proof, nullifier);
}

bool lx_asset_nullifier_seen(const lx_withdrawal_store *store,
                             const uint8_t nullifier[32])
{
    size_t i;
    if (store == NULL || nullifier == NULL) return false;
    if (store->count > LX_DEPOSIT_NULLIFIER_CAPACITY) return true;
    for (i = 0U; i < store->count; ++i)
        if (memcmp(store->records[i].nullifier, nullifier, 32U) == 0)
            return true;
    return false;
}

lxp_result lx_asset_historical_withdrawal_encode_v2(const lx_withdrawal_request *request,
    uint8_t key[LX_WITHDRAWAL_STATE_KEY_BYTES],
    uint8_t value[LX_WITHDRAWAL_STATE_VALUE_BYTES])
{
    uint8_t nullifier[32];
    if (key == NULL || value == NULL ||
        lx_asset_historical_withdrawal_nullifier_v1(request, nullifier) != LXP_OK)
        return LXP_ERR_NON_CANONICAL;
    memcpy(key, "withdrawal:", 11U);
    memcpy(key + 11U, nullifier, 32U);
    value[0] = 0U;
    value[1] = 2U;
    store_u32(value + 2U, request->network_id);
    memcpy(value + 6U, request->withdrawal_id, 32U);
    memcpy(value + 38U, request->account_id, 32U);
    memcpy(value + 70U, request->asset_id, 32U);
    if (lxp_u128_to_be(request->amount, value + 102U) != LXP_OK)
        return LXP_ERR_NON_CANONICAL;
    memcpy(value + 118U, request->payout_recipient, 32U);
    memcpy(value + 150U, request->checkpoint_id, 32U);
    return LXP_OK;
}

lxp_result lx_asset_historical_withdrawal_decode_v2(const uint8_t *key, size_t key_length,
    const uint8_t *value, size_t value_length, lx_withdrawal_record *record)
{
    lx_withdrawal_record candidate = {0};
    uint8_t canonical_key[LX_WITHDRAWAL_STATE_KEY_BYTES];
    uint8_t canonical_value[LX_WITHDRAWAL_STATE_VALUE_BYTES];
    if (key == NULL || value == NULL || record == NULL ||
        key_length != LX_WITHDRAWAL_STATE_KEY_BYTES ||
        value_length != LX_WITHDRAWAL_STATE_VALUE_BYTES ||
        value[0] != 0U || value[1] != 2U)
        return LXP_ERR_NON_CANONICAL;
    candidate.request.network_id = ((uint32_t)value[2] << 24U) |
        ((uint32_t)value[3] << 16U) | ((uint32_t)value[4] << 8U) | value[5];
    memcpy(candidate.request.withdrawal_id, value + 6U, 32U);
    memcpy(candidate.request.account_id, value + 38U, 32U);
    memcpy(candidate.request.asset_id, value + 70U, 32U);
    if (lxp_u128_from_be(value + 102U, &candidate.request.amount) != LXP_OK)
        return LXP_ERR_NON_CANONICAL;
    memcpy(candidate.request.payout_recipient, value + 118U, 32U);
    memcpy(candidate.request.checkpoint_id, value + 150U, 32U);
    if (lx_asset_historical_withdrawal_encode_v2(&candidate.request, canonical_key,
            canonical_value) != LXP_OK ||
        memcmp(key, canonical_key, sizeof(canonical_key)) != 0 ||
        memcmp(value, canonical_value, sizeof(canonical_value)) != 0)
        return LXP_ERR_NON_CANONICAL;
    memcpy(candidate.nullifier, key + 11U, 32U);
    *record = candidate;
    return LXP_OK;
}

/* Existing codec names remain read-only compatibility entry points. */
lxp_result lx_withdrawal_nullifier(const lx_withdrawal_request *request,
                                   uint8_t nullifier[32])
{
    return lx_asset_historical_withdrawal_nullifier_v1(request, nullifier);
}

lxp_result lx_withdrawal_state_encode(const lx_withdrawal_request *request,
    uint8_t key[LX_WITHDRAWAL_STATE_KEY_BYTES],
    uint8_t value[LX_WITHDRAWAL_STATE_VALUE_BYTES])
{
    return lx_asset_historical_withdrawal_encode_v2(request, key, value);
}

lxp_result lx_withdrawal_state_decode(const uint8_t *key, size_t key_length,
    const uint8_t *value, size_t value_length, lx_withdrawal_record *record)
{
    return lx_asset_historical_withdrawal_decode_v2(
        key, key_length, value, value_length, record);
}

/* Retired ABI boundaries: no allocation, proof admission, writes or transfers.
 * External callers are removed by the owning bridge/header tasks. */
lxp_result lx_paxeer_deposit_root_message(
    const lx_paxeer_deposit_root_registration *registration,
    uint8_t *message, size_t capacity, size_t *message_length)
{
    (void)registration;
    (void)message;
    (void)capacity;
    (void)message_length;
    return LXP_ERR_UNKNOWN_ACTIVITY;
}

lxp_result lx_checkpoint_registry_create(
    const uint8_t paxeer_checkpoint_authority[32],
    uint32_t network_id, uint16_t protocol_version,
    lx_checkpoint_registry **registry)
{
    (void)paxeer_checkpoint_authority;
    (void)network_id;
    (void)protocol_version;
    (void)registry;
    return LXP_ERR_UNKNOWN_ACTIVITY;
}

lxp_result lx_checkpoint_registry_destroy(lx_checkpoint_registry **registry)
{
    (void)registry;
    return LXP_ERR_UNKNOWN_ACTIVITY;
}

lxp_result lx_paxeer_deposit_leaf_hash(const lx_deposit_proof *proof,
                                       uint8_t leaf_hash[32])
{
    (void)proof;
    (void)leaf_hash;
    return LXP_ERR_UNKNOWN_ACTIVITY;
}

lxp_result lx_bridge_verify_deposit(const lx_deposit_proof *proof,
                                    const lx_checkpoint_registry *checkpoints,
                                    uint32_t network_id,
                                    uint16_t protocol_version)
{
    (void)proof;
    (void)checkpoints;
    (void)network_id;
    (void)protocol_version;
    return LXP_ERR_UNKNOWN_ACTIVITY;
}

lxp_result lx_asset_deposit_credit(lxp_module_ctx *ctx,
                                   const lx_asset_transfer_request *request,
                                   const lx_deposit_proof *proof,
                                   const lx_checkpoint_registry *checkpoints,
                                   uint32_t network_id,
                                   uint16_t protocol_version,
                                   lxp_receipt *receipt)
{
    (void)ctx;
    (void)request;
    (void)proof;
    (void)checkpoints;
    (void)network_id;
    (void)protocol_version;
    (void)receipt;
    return LXP_ERR_UNKNOWN_ACTIVITY;
}

lxp_result lx_asset_withdraw_request(lxp_module_ctx *ctx,
                                     const lx_asset_transfer_request *transfer,
                                     const lx_withdrawal_request *withdrawal,
                                     lx_withdrawal_store *store,
                                     lxp_receipt *receipt)
{
    (void)ctx;
    (void)transfer;
    (void)withdrawal;
    (void)store;
    (void)receipt;
    return LXP_ERR_UNKNOWN_ACTIVITY;
}

lxp_result lx_asset_withdraw_settle(lxp_module_ctx *ctx,
                                    lx_account *withdrawals,
                                    lx_account *reserve,
                                    const lx_asset_record *asset,
                                    const lx_finalized_checkpoint *checkpoint,
                                    const uint8_t nullifier[32],
                                    lx_withdrawal_store *store,
                                    lxp_transfer_context context,
                                    lxp_receipt *receipt)
{
    (void)ctx;
    (void)withdrawals;
    (void)reserve;
    (void)asset;
    (void)checkpoint;
    (void)nullifier;
    (void)store;
    (void)context;
    (void)receipt;
    return LXP_ERR_UNKNOWN_ACTIVITY;
}
