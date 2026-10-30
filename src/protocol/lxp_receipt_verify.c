#include "layerx/lxp_verify.h"

#include "layerx/lxp_crypto.h"
#include "layerx/lxp_hash.h"

#include <string.h>

lxp_result lxp_receipt_verify_offline(
    const lxp_receipt *receipt,
    const uint8_t sequencer_public_key[32],
    lxp_arena *arena)
{
    if (receipt == NULL || sequencer_public_key == NULL || arena == NULL ||
        !lxp_protocol_version_supported(receipt->protocol_version) ||
        receipt->result_code != LXP_OK || receipt->operation == 0U ||
        lxp_ct_is_zero(receipt->activity_id, 32U) ||
        lxp_ct_is_zero(receipt->sequencer_signature, 64U))
        return LXP_ERR_NON_CANONICAL;
    lxp_result status = lxp_receipt_validate_supply(receipt);
    return status == LXP_OK ?
        lxp_receipt_verify(receipt, sequencer_public_key, arena) : status;
}

lxp_result lxp_receipt_match_requirement(
    const lxp_receipt *receipt,
    const lxp_payment_requirement *requirement,
    uint32_t receipt_network_id)
{
    uint8_t receive_context_preimage[64];
    uint8_t expected_context[32];
    lxp_result status;
    if (receipt == NULL || requirement == NULL || receipt_network_id == 0U ||
        receipt_network_id != requirement->network_id)
        return LXP_ERR_WRONG_NETWORK;
    if (lxp_ct_memcmp(receipt->asset, requirement->asset, 32U) != 0 ||
        lxp_ct_memcmp(receipt->to, requirement->recipient, 32U) != 0 ||
        lxp_u128_cmp(receipt->amount, requirement->amount) != 0)
        return LXP_ERR_GRANT_SCOPE_VIOLATION;
    if (receipt->operation == (uint8_t)LX_ASSET_SEND)
        (void)memcpy(expected_context, requirement->purpose_hash, 32U);
    else if (receipt->operation == (uint8_t)LX_ASSET_RECEIVE) {
        (void)memcpy(receive_context_preimage,
                     requirement->purpose_hash, 32U);
        (void)memcpy(receive_context_preimage + 32U,
                     requirement->invoice_id, 32U);
        status = lxp_hash_context_value(
            receive_context_preimage, sizeof(receive_context_preimage),
            expected_context);
        if (status != LXP_OK) return status;
    } else {
        return LXP_ERR_UNKNOWN_ACTIVITY;
    }
    return lxp_ct_memcmp(
        receipt->context_hash, expected_context, 32U) == 0 ?
        LXP_OK : LXP_ERR_CONTEXT_MISMATCH;
}
