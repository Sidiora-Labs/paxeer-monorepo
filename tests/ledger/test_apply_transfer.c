#include "test_values.h"

#include "layerx/lxp_module.h"
#include "layerx/lxp_snapshot.h"
#include "layerx/lxp_transfer.h"

#include <stdlib.h>
#include <string.h>

static int unchanged(const lx_account *from, const lx_account *to,
                     uint64_t from_value, uint64_t to_value)
{
    return TEST_EQ(from->balance.hi, 0U) &&
           TEST_EQ(from->balance.lo, from_value) &&
           TEST_EQ(to->balance.hi, 0U) &&
           TEST_EQ(to->balance.lo, to_value);
}

/* One debit leg drawn under a resolved grant. A delegated debit that presents
 * no grant is refused; the grant it does present is bound to the leg, and its
 * scope is charged before any balance moves. */
static int allowance_checks(void)
{
    lx_account_registry *registry;
    lx_account *from;
    lx_account *to;
    const char *from_name = "agent:did:key:carol:main";
    const char *to_name = "agent:did:key:dave:main";
    uint8_t from_id[32];
    uint8_t to_id[32];
    uint8_t grant_asset[32] = { 3U };
    lxp_transfer_asset_state assets[1];
    lxp_authority_scope scope;
    lxp_authority_scope before;
    lxp_transfer_allowance allowance;
    lxp_transfer_leg leg = { 0 };
    lxp_transfer_context context;
    lxp_transfer_result result;
    int failed = 1;

    registry = malloc(sizeof(*registry));
    if (!TEST_EQ(!!(registry == NULL), 0)) return 1;
    if (!TEST_EQ(lx_account_registry_init(registry), LXP_OK) ||
        !TEST_EQ(lx_account_id_from_string((const uint8_t *)from_name, strlen(from_name),
                                  from_id), LXP_OK) ||
        !TEST_EQ(lx_account_id_from_string((const uint8_t *)to_name, strlen(to_name),
                                  to_id), LXP_OK) ||
        !TEST_EQ(lx_account_open(registry, (const uint8_t *)from_name, strlen(from_name),
                        from_id, 1U, LX_ACCOUNT_OPEN_CREDIT, NULL, &from), LXP_OK) ||
        !TEST_EQ(lx_account_open(registry, (const uint8_t *)to_name, strlen(to_name),
                        to_id, 1U, LX_ACCOUNT_OPEN_CREDIT, NULL, &to), LXP_OK) ||
        !TEST_EQ(lxp_ledger_bootstrap_balance(from, grant_asset, (lxp_u128){ 0U, 100U },
                                     0U), LXP_OK) ||
        !TEST_EQ(lxp_ledger_bootstrap_balance(to, grant_asset, (lxp_u128){ 0U, 0U },
                                     0U), LXP_OK)) goto done;
    (void)memset(assets, 0, sizeof(assets));
    (void)memcpy(assets[0].asset_id, grant_asset, 32U);
    assets[0].registered = true;
    (void)memset(&scope, 0, sizeof(scope));
    scope.module_mask = UINT64_C(1) << LXP_MODULE_ASSET;
    scope.activity_ordinal_min = 1U;
    scope.activity_ordinal_max = 2U;
    (void)memcpy(scope.asset_id, grant_asset, 32U);
    scope.maximum_per_activity = (lxp_u128){ 0U, 40U };
    scope.maximum_total = (lxp_u128){ 0U, 60U };
    scope.purpose_hash[0] = 0x77U;
    (void)memset(&allowance, 0, sizeof(allowance));
    allowance.scope = &scope;
    allowance.kind = LXP_AUTHORITY_DELEGATED_CAPABILITY;
    (void)memcpy(allowance.grantor, from_id, 32U);
    (void)memset(&context, 0, sizeof(context));
    context.assets = assets;
    context.asset_count = 1U;
    (void)memcpy(context.authorized_from, from_id, 32U);
    context.batch_timestamp = 100U;
    context.origin_module_id = LXP_MODULE_ASSET;
    context.debit_authority_kind = LXP_AUTH_DELEGATED_CAPABILITY;
    (void)memset(&leg, 0, sizeof(leg));
    leg.from = from;
    leg.to = to;
    (void)memcpy(leg.asset_id, grant_asset, 32U);
    leg.amount = (lxp_u128){ 0U, 10U };
    leg.reason = LXP_REASON_PAYMENT;
    leg.supply_mode = LXP_TRANSFER_CONSERVED;

    /* A delegated debit that hands the ledger no grant cannot spend. */
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_AUTH_ALLOWANCE) ||
        !TEST_EQ(unchanged(from, to, 100U, 0U), 1) ||
        !TEST_EQ(from->next_sequence, 0U)) goto done;
    context.allowance = &allowance;
    before = scope;
    /* The presented grant kind must be the kind the debit claims. */
    allowance.kind = LXP_AUTHORITY_SESSION_KEY;
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_AUTH_SCOPE) ||
        !TEST_EQ(unchanged(from, to, 100U, 0U), 1)) goto done;
    allowance.kind = LXP_AUTHORITY_DELEGATED_CAPABILITY;
    /* The debited account must be the account the grant was written against. */
    (void)memcpy(allowance.grantor, to_id, 32U);
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_UNAUTHORIZED_DEBIT)) goto done;
    (void)memcpy(allowance.grantor, from_id, 32U);
    /* The emitting module must be inside the grant scope. */
    context.origin_module_id = LXP_MODULE_PERPS;
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_AUTH_SCOPE))
        goto done;
    context.origin_module_id = LXP_MODULE_ASSET;
    /* The moved asset must be the asset the grant meters. */
    scope.asset_id[0] = 0x99U;
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_ASSET_MISMATCH))
        goto done;
    scope.asset_id[0] = 3U;
    /* Above the per-activity cap the refusal is typed and nothing moves: not
     * the balances, not the sequence, not the recorded spend. */
    leg.amount = (lxp_u128){ 0U, 41U };
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_GRANT_EXHAUSTED) ||
        !TEST_EQ(unchanged(from, to, 100U, 0U), 1) ||
        !TEST_EQ(from->next_sequence, 0U) ||
        !TEST_EQ(memcmp(&scope, &before, sizeof(scope)), 0)) goto done;
    /* A permitted draw charges the grant and moves the balance together. */
    leg.amount = (lxp_u128){ 0U, 40U };
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_OK) ||
        !TEST_EQ(unchanged(from, to, 60U, 40U), 1) ||
        !TEST_EQ(from->next_sequence, 1U) ||
        !TEST_EQ(scope.spent_total.lo, 40U) ||
        !TEST_EQ(scope.spent_this_period.lo, 40U))
        goto done;
    /* The lifetime cap binds across activities, not only within one. */
    before = scope;
    context.actor_sequence = 1U;
    leg.amount = (lxp_u128){ 0U, 21U };
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_GRANT_EXHAUSTED) ||
        !TEST_EQ(unchanged(from, to, 60U, 40U), 1) ||
        !TEST_EQ(from->next_sequence, 1U) ||
        !TEST_EQ(memcmp(&scope, &before, sizeof(scope)), 0)) goto done;
    leg.amount = (lxp_u128){ 0U, 20U };
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_OK) ||
        !TEST_EQ(unchanged(from, to, 40U, 60U), 1) ||
        !TEST_EQ(from->next_sequence, 2U) ||
        !TEST_EQ(scope.spent_total.lo, 60U)) goto done;
    context.actor_sequence = 2U;
    leg.amount = (lxp_u128){ 0U, 1U };
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_GRANT_EXHAUSTED) ||
        !TEST_EQ(unchanged(from, to, 40U, 60U), 1))
        goto done;
    /* A metered grant cannot be drawn as an unmetered kind. */
    context.debit_authority_kind = LXP_AUTH_OWNER;
    allowance.kind = LXP_AUTHORITY_OWNER;
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_AUTH_SCOPE) ||
        !TEST_EQ(unchanged(from, to, 40U, 60U), 1)) goto done;
    /* The Budget module charges its own record before it emits the set, so it
     * is the one origin that carries a budget authorization without a grant
     * scope; every other origin has to present one. */
    context.allowance = NULL;
    context.debit_authority_kind = LXP_AUTH_BUDGET_ALLOWANCE;
    context.origin_module_id = LXP_MODULE_ASSET;
    leg.amount = (lxp_u128){ 0U, 10U };
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_AUTH_ALLOWANCE) ||
        !TEST_EQ(unchanged(from, to, 40U, 60U), 1) ||
        !TEST_EQ(from->next_sequence, 2U)) goto done;
    context.origin_module_id = LXP_MODULE_BUDGET;
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_OK) ||
        !TEST_EQ(unchanged(from, to, 30U, 70U), 1) ||
        !TEST_EQ(from->next_sequence, 3U)) goto done;
    failed = 0;
done:
    free(registry);
    return failed;
}

static int historical_namespace_record_recovery(void)
{
    static const struct {
        const char *name;
        lx_account_kind kind;
    } historical_vectors[] = {
        { "system:paxeer-reserve", LX_ACCOUNT_SYSTEM_PAXEER_RESERVE },
        { "system:paxeer-reserve:pax", LX_ACCOUNT_SYSTEM_PAXEER_RESERVE },
        { "system:paxeer-reserve:sid", LX_ACCOUNT_SYSTEM_PAXEER_RESERVE },
        { "system:paxeer-reserve:usdc", LX_ACCOUNT_SYSTEM_PAXEER_RESERVE },
        { "system:paxeer-reserve:usdl", LX_ACCOUNT_SYSTEM_PAXEER_RESERVE },
        { "system:paxeer-withdrawals", LX_ACCOUNT_SYSTEM_PAXEER_WITHDRAWALS }
    };
    enum { count = sizeof(historical_vectors) / sizeof(historical_vectors[0]) };
    lx_account_registry registry;
    uint8_t records[count][1024];
    size_t lengths[count];
    uint8_t root_before[32];
    uint8_t root_after[32];
    const uint8_t asset_id[32] = { 3U };
    int failed = 1;

    if (!TEST_EQ(lx_account_registry_init(&registry), LXP_OK)) return 1;
    for (size_t i = 0U; i < count; ++i) {
        const uint8_t *name = (const uint8_t *)historical_vectors[i].name;
        size_t name_length = strlen(historical_vectors[i].name);
        uint8_t id[32];
        lx_account *account = NULL;

        if (!TEST_EQ(lx_account_id_from_string(name, name_length, id), LXP_OK))
            goto done;
        if (!TEST_EQ(lx_account_registry_root(&registry, root_before), LXP_OK) ||
            !TEST_EQ(lx_account_open(&registry, name, name_length, id, 7U,
                LX_ACCOUNT_OPEN_CREDIT, NULL, &account),
                LXP_ERR_UNAUTHORIZED_DEBIT) ||
            !TEST_EQ(account == NULL, true) ||
            !TEST_EQ(registry.count, i) ||
            !TEST_EQ(lx_account_registry_root(&registry, root_after), LXP_OK) ||
            !TEST_EQ(memcmp(root_before, root_after, 32U), 0) ||
            !TEST_EQ(lx_account_open(&registry, name, name_length, id, 7U,
                LX_ACCOUNT_OPEN_GENESIS, NULL, &account), LXP_OK) ||
            !TEST_EQ(lxp_ledger_bootstrap_balance(account, asset_id,
                (lxp_u128){ 1U, 17U + i }, 42U), LXP_OK)) goto done;
        account->frozen = true;
        account->has_open_reference = true;
        if (!TEST_EQ(lx_account_validate_canonical(account), LXP_OK) ||
            !TEST_EQ(lx_account_record_encode(account, records[i],
                sizeof(records[i]), &lengths[i]), LXP_OK)) goto done;
    }
    if (!TEST_EQ(lx_account_registry_root(&registry, root_before), LXP_OK))
        goto done;
    lx_account_registry_release(&registry);
    if (!TEST_EQ(lx_account_registry_init(&registry), LXP_OK)) return 1;
    for (size_t i = 0U; i < count; ++i) {
        const uint8_t *name = (const uint8_t *)historical_vectors[i].name;
        size_t name_length = strlen(historical_vectors[i].name);
        uint8_t id[32];
        lx_account restored;
        lx_account *account = NULL;
        size_t slot;

        if (!TEST_EQ(lx_account_id_from_string(name, name_length, id), LXP_OK))
            goto done;
        if (!TEST_EQ(lx_account_record_decode(records[i], lengths[i],
                &restored), LXP_OK) ||
            !TEST_EQ(lx_account_validate_canonical(&restored), LXP_OK) ||
            !TEST_EQ(lx_account_registry_slot_insert(&registry, &restored,
                &slot), LXP_OK) ||
            !TEST_EQ(lx_account_lookup(&registry, name, name_length, id,
                &account), LXP_OK) ||
            !TEST_EQ(account == &registry.accounts[slot], true) ||
            !TEST_EQ(account->kind, historical_vectors[i].kind) ||
            !TEST_EQ(account->balance.hi, 1U) ||
            !TEST_EQ(account->balance.lo, 17U + i) ||
            !TEST_EQ(account->next_sequence, 42U) ||
            !TEST_EQ(account->created_at_sequence, 7U) ||
            !TEST_EQ(account->has_asset, true) ||
            !TEST_EQ(memcmp(account->asset_id, asset_id, 32U), 0) ||
            !TEST_EQ(account->frozen, true) ||
            !TEST_EQ(account->has_open_reference, true) ||
            !TEST_EQ(lx_account_open(&registry, name, name_length, id, 99U,
                LX_ACCOUNT_OPEN_CREDIT, NULL, &account), LXP_OK) ||
            !TEST_EQ(account == &registry.accounts[slot], true)) goto done;
    }
    if (!TEST_EQ(registry.count, count) ||
        !TEST_EQ(lx_account_registry_index_validate(&registry), LXP_OK) ||
        !TEST_EQ(lx_account_registry_root(&registry, root_after), LXP_OK) ||
        !TEST_EQ(memcmp(root_before, root_after, 32U), 0)) goto done;
    failed = 0;
done:
    lx_account_registry_release(&registry);
    return failed;
}

static int occupancy_asset_account_checks(void)
{
    lx_account_registry registry;
    lx_account *matching;
    lx_account *different;
    lx_account *third_kind;
    lx_account *to;
    const char *names[] = {
        "agent:did:key:alice:asset:0100000000000000000000000000000000000000000000000000000000000000",
        "agent:did:key:alice:asset:0200000000000000000000000000000000000000000000000000000000000000",
        "agent:did:key:alice:budget:daily",
        "agent:did:key:bob:main"
    };
    lx_account **accounts[] = { &matching, &different, &third_kind, &to };
    uint8_t ids[4][32];
    uint8_t asset_id[32] = { 1U };
    uint8_t other_asset[32] = { 2U };
    lxp_transfer_asset_state assets[1] = { 0 };
    lxp_transfer_leg leg = { 0 };
    lxp_transfer_context context = { 0 };
    lxp_transfer_result result;
    size_t index;

    if (!TEST_EQ(lx_account_registry_init(&registry), LXP_OK)) return 1;
    for (index = 0U; index < 4U; ++index) {
        size_t length = strlen(names[index]);
        if (!TEST_EQ(lx_account_id_from_string((const uint8_t *)names[index],
                length, ids[index]), LXP_OK) ||
            !TEST_EQ(lx_account_open(&registry, (const uint8_t *)names[index],
                length, ids[index], 1U, LX_ACCOUNT_OPEN_CREDIT, NULL,
                accounts[index]), LXP_OK)) return 1;
    }
    if (!TEST_EQ(lxp_ledger_bootstrap_balance(matching, asset_id,
            (lxp_u128){ 0U, 100U }, 7U), LXP_OK) ||
        !TEST_EQ(lxp_ledger_bootstrap_balance(different, other_asset,
            (lxp_u128){ 0U, 100U }, 7U), LXP_OK) ||
        !TEST_EQ(lxp_ledger_bootstrap_balance(third_kind, asset_id,
            (lxp_u128){ 0U, 100U }, 7U), LXP_OK) ||
        !TEST_EQ(lxp_ledger_bootstrap_balance(to, asset_id,
            (lxp_u128){ 0U, 0U }, 0U), LXP_OK)) return 1;
    (void)memcpy(assets[0].asset_id, asset_id, 32U);
    assets[0].registered = true;
    context.assets = assets;
    context.asset_count = 1U;
    context.protocol_system_capability = true;
    context.origin_module_id = LXP_MODULE_PROGRAMS;
    context.debit_authority_kind = LXP_AUTH_OCCUPANCY_RESPONSIBILITY;
    leg.from = matching;
    leg.to = to;
    leg.reason = LXP_REASON_STORAGE_OCCUPANCY;
    leg.amount = (lxp_u128){ 0U, 10U };
    (void)memcpy(leg.asset_id, asset_id, 32U);
    (void)memcpy(context.authorized_from, matching->id, 32U);
    if (!TEST_EQ(matching->kind, LX_ACCOUNT_AGENT_ASSET) ||
        !TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_OK) ||
        !TEST_EQ(unchanged(matching, to, 90U, 10U), 1) ||
        !TEST_EQ(matching->next_sequence, 7U)) return 1;

    leg.from = different;
    (void)memcpy(context.authorized_from, different->id, 32U);
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_ASSET_MISMATCH) ||
        !TEST_EQ(unchanged(different, to, 100U, 10U), 1) ||
        !TEST_EQ(different->next_sequence, 7U)) return 1;

    leg.from = third_kind;
    (void)memcpy(context.authorized_from, third_kind->id, 32U);
    if (!TEST_EQ(third_kind->kind, LX_ACCOUNT_AGENT_BUDGET) ||
        !TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_UNAUTHORIZED_DEBIT) ||
        !TEST_EQ(unchanged(third_kind, to, 100U, 10U), 1) ||
        !TEST_EQ(third_kind->next_sequence, 7U)) return 1;

    leg.from = matching;
    (void)memcpy(context.authorized_from, matching->id, 32U);
    context.origin_module_id = LXP_MODULE_ASSET;
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_UNAUTHORIZED_DEBIT) ||
        !TEST_EQ(unchanged(matching, to, 90U, 10U), 1) ||
        !TEST_EQ(matching->next_sequence, 7U)) return 1;
    lx_account_registry_release(&registry);
    return 0;
}

int main(void)
{
    lx_account_registry registry;
    lx_account *from;
    lx_account *to;
    const char *from_name = "agent:did:key:alice:main";
    const char *to_name = "agent:did:key:bob:main";
    uint8_t from_id[32];
    uint8_t to_id[32];
    uint8_t asset_id[32] = { 1U };
    uint8_t other_asset[32] = { 2U };
    lxp_transfer_asset_state assets[1];
    lxp_transfer_leg leg = { 0 };
    lxp_transfer_context context;
    lxp_transfer_result result;
    lxp_u128 maximum = { UINT64_MAX, UINT64_MAX };

    if (historical_namespace_record_recovery() != 0) return 1;
    if (!TEST_EQ(lx_account_registry_init(&registry), LXP_OK) ||
        !TEST_EQ(lx_account_id_from_string((const uint8_t *)from_name, strlen(from_name),
                                  from_id), LXP_OK) ||
        !TEST_EQ(lx_account_id_from_string((const uint8_t *)to_name, strlen(to_name),
                                  to_id), LXP_OK) ||
        !TEST_EQ(lx_account_open(&registry, (const uint8_t *)from_name, strlen(from_name),
                        from_id, 1U, LX_ACCOUNT_OPEN_CREDIT, NULL, &from), LXP_OK) ||
        !TEST_EQ(lx_account_open(&registry, (const uint8_t *)to_name, strlen(to_name),
                        to_id, 1U, LX_ACCOUNT_OPEN_CREDIT, NULL, &to), LXP_OK) ||
        !TEST_EQ(lxp_ledger_bootstrap_balance(from, asset_id, (lxp_u128){ 0U, 100U },
                                     4U), LXP_OK) ||
        !TEST_EQ(lxp_ledger_bootstrap_balance(to, asset_id, (lxp_u128){ 0U, 5U }, 0U), LXP_OK)) return 1;
    (void)memset(&assets, 0, sizeof(assets));
    (void)memcpy(assets[0].asset_id, asset_id, 32U);
    assets[0].registered = true;
    (void)memset(&context, 0, sizeof(context));
    context.assets = assets;
    context.asset_count = 1U;
    (void)memcpy(context.authorized_from, from_id, 32U);
    context.actor_sequence = 4U;
    context.batch_timestamp = 10U;
    context.expires_at = 10U;
    leg.from = from;
    leg.to = to;
    (void)memcpy(leg.asset_id, asset_id, 32U);

    leg.amount = (lxp_u128){ 0U, 0U };
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_ZERO_AMOUNT) ||
        !TEST_EQ(unchanged(from, to, 100U, 5U), 1)) return 1;
    (void)memcpy(leg.asset_id, other_asset, 32U);
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_ZERO_AMOUNT))
        return 1;
    leg.amount = (lxp_u128){ 0U, 1U };
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_ASSET_MISMATCH) ||
        !TEST_EQ(unchanged(from, to, 100U, 5U), 1)) return 1;
    (void)memcpy(leg.asset_id, asset_id, 32U);
    assets[0].registered = false;
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_ASSET_MISMATCH))
        return 1;
    assets[0].registered = true;
    assets[0].paused = true;
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_ASSET_PAUSED))
        return 1;
    assets[0].paused = false;
    from->frozen = true;
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_ACCOUNT_FROZEN))
        return 1;
    from->frozen = false;
    leg.amount = (lxp_u128){ 0U, 101U };
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_INSUFFICIENT_BALANCE) ||
        !TEST_EQ(unchanged(from, to, 100U, 5U), 1))
        return 1;
    if (!TEST_EQ(lxp_ledger_bootstrap_balance(to, asset_id, maximum, 0U), LXP_OK))
        return 1;
    leg.amount = (lxp_u128){ 0U, 1U };
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_OVERFLOW) ||
        !TEST_EQ(from->balance.lo, 100U) ||
        !TEST_EQ(to->balance.hi, UINT64_MAX) ||
        !TEST_EQ(to->balance.lo, UINT64_MAX)) return 1;
    if (!TEST_EQ(lxp_ledger_bootstrap_balance(to, asset_id, (lxp_u128){ 0U, 0U }, 0U), LXP_OK)) return 1;
    context.has_client_balance = true;
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_CLIENT_SUPPLIED_BALANCE)) return 1;
    context.has_client_balance = false;
    leg.amount = (lxp_u128){ 0U, 100U };
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_OK) ||
        !TEST_EQ(unchanged(from, to, 0U, 100U), 1) ||
        !TEST_EQ(result.from_balance_before.lo, 100U) ||
        !TEST_EQ(result.from_balance_after.lo, 0U) ||
        !TEST_EQ(result.to_balance_before.lo, 0U) ||
        !TEST_EQ(result.to_balance_after.lo, 100U) ||
        !TEST_EQ(from->next_sequence, 5U))
        return 1;
    leg.reason = LXP_REASON_PAYMENT;
    leg.supply_mode = LXP_TRANSFER_CONSERVED;
    if (!TEST_EQ(lxp_ledger_bootstrap_balance(from, asset_id, (lxp_u128){ 0U, 100U },
                                     UINT64_MAX), LXP_OK) ||
        !TEST_EQ(lxp_ledger_bootstrap_balance(to, asset_id, (lxp_u128){ 0U, 40U }, 0U), LXP_OK)) return 1;
    context.actor_sequence = UINT64_MAX;
    leg.amount = (lxp_u128){ 0U, 10U };
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_SEQUENCE_EXHAUSTED) ||
        !TEST_EQ(unchanged(from, to, 100U, 40U), 1) ||
        !TEST_EQ(from->next_sequence, UINT64_MAX) ||
        !TEST_EQ(to->next_sequence, 0U)) return 1;
    leg.from = to;
    leg.to = from;
    (void)memcpy(context.authorized_from, to_id, 32U);
    context.actor_sequence = 0U;
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_OK) ||
        !TEST_EQ(unchanged(to, from, 30U, 110U), 1) ||
        !TEST_EQ(to->next_sequence, 1U) ||
        !TEST_EQ(from->next_sequence, UINT64_MAX)) return 1;
    leg.from = from;
    leg.to = to;
    (void)memcpy(context.authorized_from, from_id, 32U);
    if (!TEST_EQ(lxp_ledger_bootstrap_balance(from, asset_id, (lxp_u128){ 0U, 110U },
                                     3U), LXP_OK) ||
        !TEST_EQ(lxp_ledger_bootstrap_balance(to, asset_id, (lxp_u128){ 0U, 30U },
                                     UINT64_MAX), LXP_OK)) return 1;
    context.sequence_account = to;
    context.actor_sequence = 3U;
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_SEQUENCE_EXHAUSTED) ||
        !TEST_EQ(unchanged(from, to, 110U, 30U), 1) ||
        !TEST_EQ(from->next_sequence, 3U) ||
        !TEST_EQ(to->next_sequence, UINT64_MAX)) return 1;
    context.sequence_account = NULL;
    context.protocol_system_capability = true;
    context.origin_module_id = LXP_MODULE_PROGRAMS;
    context.debit_authority_kind = LXP_AUTH_OCCUPANCY_RESPONSIBILITY;
    leg.reason = LXP_REASON_STORAGE_OCCUPANCY;
    if (!TEST_EQ(from->kind, LX_ACCOUNT_AGENT_MAIN) ||
        !TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_OK) ||
        !TEST_EQ(unchanged(from, to, 100U, 40U), 1) ||
        !TEST_EQ(from->next_sequence, 3U) ||
        !TEST_EQ(to->next_sequence, UINT64_MAX)) return 1;
    if (!TEST_EQ(lxp_ledger_bootstrap_balance(from, asset_id, (lxp_u128){ 0U, 100U },
                                     UINT64_MAX), LXP_OK)) return 1;
    if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_ERR_SEQUENCE_EXHAUSTED) ||
        !TEST_EQ(unchanged(from, to, 100U, 40U), 1) ||
        !TEST_EQ(from->next_sequence, UINT64_MAX)) return 1;
    {
        lxp_transfer_source_authority source;
        (void)memset(&source, 0, sizeof(source));
        (void)memcpy(source.authorized_from, from_id, 32U);
        source.debit_authority_kind = LXP_AUTH_OWNER;
        context.source_authorities = &source;
        context.source_authority_count = 1U;
        context.debit_authority_kind = LXP_AUTH_OCCUPANCY_RESPONSIBILITY;
        context.actor_sequence = 7U;
        leg.reason = LXP_REASON_PAYMENT;
        if (!TEST_EQ(lxp_ledger_bootstrap_balance(from, asset_id,
                 (lxp_u128){0U, 100U}, 7U), LXP_OK) ||
        !TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_OK) ||
        !TEST_EQ(from->next_sequence, 8U)) return 1;
        source.debit_authority_kind = LXP_AUTH_OCCUPANCY_RESPONSIBILITY;
        source.protocol_system_capability = true;
        context.debit_authority_kind = LXP_AUTH_OWNER;
        context.actor_sequence = 8U;
        leg.reason = LXP_REASON_STORAGE_OCCUPANCY;
        if (!TEST_EQ(lxp_apply_transfer(&leg, &context, &result), LXP_OK) ||
        !TEST_EQ(from->next_sequence, 8U)) return 1;
    }
    if (!TEST_EQ(allowance_checks(), 0)) return 1;
    if (!TEST_EQ(occupancy_asset_account_checks(), 0)) return 1;
    return 0;
}
