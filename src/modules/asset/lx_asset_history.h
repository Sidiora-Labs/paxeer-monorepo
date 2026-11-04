#ifndef LAYERX_LX_ASSET_HISTORY_H
#define LAYERX_LX_ASSET_HISTORY_H

#include "layerx/lx_asset.h"

/* Historical custody codecs and reports only. These never authorize execution. */
lxp_result lx_asset_historical_deposit_nullifier_v1(
    const lx_deposit_proof *proof, uint8_t nullifier[32]);
lxp_result lx_asset_historical_withdrawal_nullifier_v1(
    const lx_withdrawal_request *request, uint8_t nullifier[32]);
lxp_result lx_asset_historical_withdrawal_encode_v2(
    const lx_withdrawal_request *request,
    uint8_t key[LX_WITHDRAWAL_STATE_KEY_BYTES],
    uint8_t value[LX_WITHDRAWAL_STATE_VALUE_BYTES]);
lxp_result lx_asset_historical_withdrawal_decode_v2(
    const uint8_t *key, size_t key_length, const uint8_t *value,
    size_t value_length, lx_withdrawal_record *record);
lxp_result lx_asset_historical_reserve_report_v1(
    const lx_account_registry *accounts,
    const lx_asset_custody_attestation *attestation,
    lx_asset_reserve_report_record *report);
lxp_result lx_asset_historical_reserve_reconcile_v1(
    const lx_account_registry *accounts,
    const lx_asset_custody_attestation *attestation,
    lx_asset_reserve_report_record *report);
/* Original untagged v1 wire report; use only with an explicitly v1 archive. */
lxp_result lx_asset_historical_reserve_encode_v1(
    const lx_asset_reserve_report_record *report,
    uint8_t *bytes, size_t capacity, size_t *length);

#endif
