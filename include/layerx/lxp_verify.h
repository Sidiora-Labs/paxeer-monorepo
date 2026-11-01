#ifndef LAYERX_LXP_VERIFY_H
#define LAYERX_LXP_VERIFY_H

#include "layerx/lxp_gateway.h"

lxp_result lxp_receipt_verify_offline(
    const lxp_receipt *receipt,
    const uint8_t sequencer_public_key[32],
    lxp_arena *arena);
lxp_result lxp_receipt_match_requirement(
    const lxp_receipt *receipt,
    const lxp_payment_requirement *requirement,
    uint32_t receipt_network_id);
lxp_result lxp_verify_receipt_against_requirement(
    const lxp_receipt *receipt,
    const lxp_payment_requirement *requirement,
    uint32_t receipt_network_id,
    const uint8_t sequencer_public_key[32],
    lxp_arena *arena);

#endif
