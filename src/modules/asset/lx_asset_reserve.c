#include "lx_asset_history.h"
#include "lx_asset_native.h"

#include "layerx/lxp_crypto.h"

#include <string.h>

static lxp_result add(lxp_u128 *sum, lxp_u128 value)
{
    lxp_u128 next;
    lxp_result status = lxp_u128_add(*sum, value, &next);
    if (status == LXP_OK) *sum = next;
    return status == LXP_OK ? LXP_OK : LXP_FATAL_SUPPLY_MISMATCH;
}

static lxp_u128 *bucket(lx_asset_reserve_report_record *report,
                        lx_account_kind kind)
{
    switch (kind) {
    case LX_ACCOUNT_AGENT_MAIN:
    case LX_ACCOUNT_AGENT_ASSET: return &report->agent_main;
    case LX_ACCOUNT_AGENT_ESCROW: return &report->escrow;
    case LX_ACCOUNT_AGENT_BUDGET: return &report->budget;
    case LX_ACCOUNT_AGENT_STREAM: return &report->stream;
    case LX_ACCOUNT_AGENT_MARGIN: return &report->margin;
    case LX_ACCOUNT_SYSTEM_LIQUIDITY:
    case LX_ACCOUNT_SYSTEM_FUNDING_LONG:
    case LX_ACCOUNT_SYSTEM_FUNDING_SHORT: return &report->liquidity;
    case LX_ACCOUNT_SYSTEM_INSURANCE: return &report->insurance;
    case LX_ACCOUNT_SYSTEM_FEES: return &report->fees;
    case LX_ACCOUNT_SYSTEM_PAXEER_WITHDRAWALS: return &report->withdrawals;
    case LX_ACCOUNT_SYSTEM_PAXEER_RESERVE: return &report->reserve;
    default: return &report->other_system;
    }
}

lxp_result lx_asset_historical_reserve_report_v1(
    const lx_account_registry *accounts,
    const lx_asset_custody_attestation *attestation,
    lx_asset_reserve_report_record *report)
{
    size_t i;
    lxp_result status = LXP_OK;
    if (accounts == NULL || attestation == NULL || report == NULL ||
        accounts->count > LX_ACCOUNT_REGISTRY_CAPACITY ||
        !attestation->finalized) return LXP_FATAL_SUPPLY_MISMATCH;
    (void)memset(report, 0, sizeof(*report));
    (void)memcpy(report->asset_id, attestation->asset_id, 32U);
    for (i = 0U; status == LXP_OK && i < accounts->count; ++i) {
        const lx_account *account = &accounts->accounts[i];
        if (!account->has_asset ||
            memcmp(account->asset_id, attestation->asset_id, 32U) != 0) continue;
        if (account->kind == LX_ACCOUNT_AGENT_ESCROW) {
            size_t line = report->escrow_line_count;
            if (line == LX_ASSET_RESERVE_LINE_CAPACITY)
                return LXP_FATAL_SUPPLY_MISMATCH;
            (void)memcpy(report->escrow_lines[line].account_id,
                         account->id, 32U);
            report->escrow_lines[line].kind = account->kind;
            report->escrow_lines[line].balance = account->balance;
            ++report->escrow_line_count;
        }
        status = add(bucket(report, account->kind), account->balance);
        if (status == LXP_OK) status = add(&report->raw_total, account->balance);
        if (status == LXP_OK &&
            account->kind != LX_ACCOUNT_SYSTEM_PAXEER_RESERVE)
            status = add(&report->circulating, account->balance);
    }
    if (status == LXP_OK)
        status = lxp_u128_sub(report->raw_total, attestation->settled_out,
                              &report->effective_total);
    if (status == LXP_OK)
        status = lxp_u128_sub(attestation->custody_amount,
                              attestation->settled_out,
                              &report->expected_backing);
    return status == LXP_OK ? LXP_OK : LXP_FATAL_SUPPLY_MISMATCH;
}

lxp_result lx_asset_historical_reserve_reconcile_v1(
    const lx_account_registry *accounts,
    const lx_asset_custody_attestation *attestation,
    lx_asset_reserve_report_record *report)
{
    lxp_result status = lx_asset_historical_reserve_report_v1(accounts, attestation, report);
    if (status != LXP_OK ||
        lxp_u128_cmp(report->circulating, report->expected_backing) > 0 ||
        lxp_u128_cmp(report->effective_total, report->expected_backing) != 0)
        return LXP_FATAL_SUPPLY_MISMATCH;
    return LXP_OK;
}

lxp_result lx_asset_historical_reserve_encode_v1(
    const lx_asset_reserve_report_record *report,
    uint8_t *bytes, size_t capacity, size_t *length)
{
    const lxp_u128 *fields[15];
    size_t cursor = 0U;
    size_t i;
    if (report == NULL || bytes == NULL || length == NULL ||
        report->escrow_line_count > LX_ASSET_RESERVE_LINE_CAPACITY ||
        capacity < 274U + report->escrow_line_count * 48U)
        return LXP_ERR_LENGTH_LIMIT;
    fields[0] = &report->agent_main;
    fields[1] = &report->escrow;
    fields[2] = &report->budget;
    fields[3] = &report->stream;
    fields[4] = &report->margin;
    fields[5] = &report->liquidity;
    fields[6] = &report->insurance;
    fields[7] = &report->fees;
    fields[8] = &report->withdrawals;
    fields[9] = &report->other_system;
    fields[10] = &report->reserve;
    fields[11] = &report->raw_total;
    fields[12] = &report->circulating;
    fields[13] = &report->effective_total;
    fields[14] = &report->expected_backing;
    (void)memcpy(bytes, report->asset_id, 32U);
    cursor = 32U;
    for (i = 0U; i < 15U; ++i) {
        lxp_result status = lxp_u128_to_be(*fields[i], bytes + cursor);
        if (status != LXP_OK) return status;
        cursor += 16U;
    }
    bytes[cursor++] = (uint8_t)(report->escrow_line_count >> 8U);
    bytes[cursor++] = (uint8_t)report->escrow_line_count;
    for (i = 0U; i < report->escrow_line_count; ++i) {
        lxp_result status;
        (void)memcpy(bytes + cursor, report->escrow_lines[i].account_id, 32U);
        cursor += 32U;
        status = lxp_u128_to_be(report->escrow_lines[i].balance,
                                bytes + cursor);
        if (status != LXP_OK) return status;
        cursor += 16U;
    }
    *length = cursor;
    return LXP_OK;
}

lxp_result lx_asset_native_reconcile(const lx_asset_record *asset,
    const lx_account_registry *accounts, lx_asset_native_supply_report *report)
{
    lx_asset_native_supply_report candidate = {0};
    uint8_t issuance_name[LX_ASSET_ISSUANCE_NAME_BYTES];
    uint8_t issuance_id[32];
    lxp_u128 limit;
    lxp_u128 total;
    if (asset == NULL || accounts == NULL || report == NULL ||
        accounts->count > LX_ACCOUNT_REGISTRY_CAPACITY ||
        lx_account_registry_index_validate(accounts) != LXP_OK ||
        lxp_ct_is_zero(asset->asset_id, 32U) ||
        lx_asset_issuance_name(asset->asset_id, issuance_name, issuance_id) != LXP_OK)
        return LXP_FATAL_SUPPLY_MISMATCH;
    memcpy(candidate.asset_id, asset->asset_id, 32U);
    candidate.registered_units = asset->total_units;
    limit = lxp_u128_is_zero(asset->supply_cap) ?
        (lxp_u128){UINT64_MAX, UINT64_MAX} : asset->supply_cap;
    if (lxp_u128_cmp(asset->total_units, limit) > 0)
        return LXP_FATAL_SUPPLY_MISMATCH;
    for (size_t i = 0U; i < accounts->count; ++i) {
        const lx_account *account = &accounts->accounts[i];
        if (!account->has_asset ||
            memcmp(account->asset_id, asset->asset_id, 32U) != 0) continue;
        if (memcmp(account->id, issuance_id, 32U) == 0) {
            if (candidate.issuance_present || account->kind != LX_ACCOUNT_MODULE_VALUE ||
                account->name_length != sizeof(issuance_name) ||
                memcmp(account->name, issuance_name, sizeof(issuance_name)) != 0)
                return LXP_FATAL_SUPPLY_MISMATCH;
            candidate.issuance_present = true;
            candidate.issuance_remaining = account->balance;
        } else if (add(&candidate.bank_units, account->balance) != LXP_OK) {
            return LXP_FATAL_SUPPLY_MISMATCH;
        }
    }
    if ((asset->issuer_kind == 1U && !candidate.issuance_present) ||
        lxp_u128_cmp(candidate.bank_units, candidate.registered_units) != 0)
        return LXP_FATAL_SUPPLY_MISMATCH;
    if (candidate.issuance_present &&
        (lxp_u128_add(candidate.bank_units, candidate.issuance_remaining, &total) != LXP_OK ||
         lxp_u128_cmp(total, limit) != 0))
        return LXP_FATAL_SUPPLY_MISMATCH;
    *report = candidate;
    return LXP_OK;
}

lxp_result lx_asset_native_supply_check(const lx_asset_registry *assets,
    const lx_account_registry *accounts)
{
    if (assets == NULL || accounts == NULL ||
        assets->count > LX_ASSET_REGISTRY_CAPACITY ||
        accounts->count > LX_ACCOUNT_REGISTRY_CAPACITY ||
        lx_account_registry_index_validate(accounts) != LXP_OK)
        return LXP_FATAL_SUPPLY_MISMATCH;
    for (size_t i = 0U; i < assets->count; ++i) {
        lx_asset_native_supply_report report;
        for (size_t j = 0U; j < i; ++j)
            if (memcmp(assets->assets[i].asset_id, assets->assets[j].asset_id, 32U) == 0)
                return LXP_FATAL_SUPPLY_MISMATCH;
        if (lx_asset_native_reconcile(&assets->assets[i], accounts, &report) != LXP_OK)
            return LXP_FATAL_SUPPLY_MISMATCH;
    }
    for (size_t i = 0U; i < accounts->count; ++i) {
        const lx_account *account = &accounts->accounts[i];
        size_t asset;
        if (!account->has_asset) {
            if (!lxp_u128_is_zero(account->balance)) return LXP_FATAL_SUPPLY_MISMATCH;
            continue;
        }
        for (asset = 0U; asset < assets->count; ++asset)
            if (memcmp(account->asset_id, assets->assets[asset].asset_id, 32U) == 0) break;
        if (asset == assets->count) return LXP_FATAL_SUPPLY_MISMATCH;
    }
    return LXP_OK;
}

lxp_result lx_asset_supply_check(const lx_asset_registry *assets,
    const lx_account_registry *accounts,
    const lx_asset_custody_attestation *attestations, size_t attestation_count)
{
    /* Old ABI accepts native reconciliation only; custody callers must migrate. */
    if (attestations != NULL || attestation_count != 0U)
        return LXP_FATAL_SUPPLY_MISMATCH;
    return lx_asset_native_supply_check(assets, accounts);
}

lxp_result lx_asset_reserve_report(const lx_account_registry *accounts,
    const lx_asset_custody_attestation *attestation,
    lx_asset_reserve_report_record *report)
{
    (void)accounts;
    (void)attestation;
    (void)report;
    return LXP_ERR_VERSION_UNSUPPORTED;
}

lxp_result lx_asset_reserve_reconcile(const lx_account_registry *accounts,
    const lx_asset_custody_attestation *attestation,
    lx_asset_reserve_report_record *report)
{
    (void)accounts;
    (void)attestation;
    (void)report;
    return LXP_ERR_VERSION_UNSUPPORTED;
}

lxp_result lx_asset_reserve_report_encode(const lx_asset_reserve_report_record *report,
    uint8_t *bytes, size_t capacity, size_t *length)
{
    (void)report;
    (void)bytes;
    (void)capacity;
    (void)length;
    return LXP_ERR_VERSION_UNSUPPORTED;
}
