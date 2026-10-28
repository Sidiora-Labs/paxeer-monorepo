#include "layerx/programs.h"

#include "layerx/lxp_authority.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_identity.h"
#include "layerx/lxp_kernel.h"
#include "layerx/lxp_receipt.h"

#include <stdio.h>
#include <string.h>

enum transfer_source {
    SOURCE_MAIN,
    SOURCE_ASSET,
    SOURCE_FOREIGN,
    SOURCE_UNOWNED,
    SOURCE_PRINCIPAL
};

static lxp_result apply_transfer_set(lxp_kernel *kernel,
                                     const lxp_transfer_set *set,
                                     lxp_receipt *receipt)
{
    lxp_transfer_set_result result;
    lxp_transfer_context context = set->context;
    lxp_result status;
    (void)kernel;
    status = lxp_apply_transfer_set((lxp_transfer_leg *)set->legs,
                                    set->leg_count, &context, &result);
    if (status == LXP_OK)
        (void)memcpy(receipt->transfer_set_root, result.transfer_set_root, 32U);
    return status;
}

static int transfer_case(enum transfer_source source, bool zero_program,
                          bool zero_authority, lxp_result expected)
{
    static const uint8_t did[] = "did:lxp:transfer-owner";
    static const char *const names[] = {
        "agent:did:lxp:transfer-owner:main",
        "agent:did:lxp:transfer-owner:asset:"
        "0100000000000000000000000000000000000000000000000000000000000000",
        "agent:did:lxp:transfer-other:main",
        "system:fees"
    };
    static uint8_t arena_bytes[LXP_MAX_ACTIVITY_BYTES + 4096U];
    const uint8_t primary_key[32] = {1U};
    const uint8_t grant_id[32] = {0};
    const uint8_t program_id[32] = {0x77U};
    const uint8_t seed[] = "primary";
    uint8_t value_id[32];
    uint8_t ids[4][32];
    uint8_t payload[146] = {0};
    lx_account_registry accounts;
    lx_account *opened[4];
    lx_account *value;
    lx_account_registration registration;
    lxp_transfer_asset_state asset = {{1U}, true, false};
    lx_programs_transfer_runtime runtime;
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_kernel kernel;
    lxp_authority_resolved authority = {0};
    lxp_identity_store identities = {0};
    lxp_identity *identity;
    lxp_arena arena;
    lxp_kernel_execution execution = {0};
    lxp_fee_params fee_parameters = {
        .version = 1U,
        .multiplier_basis_points = 10000U
    };
    lxp_activity activity = {0};
    lxp_receipt receipt = {0};
    uint64_t parameters = 1U;
    size_t source_index = source == SOURCE_PRINCIPAL ? 0U : (size_t)source;
    bool created = false;
    int failed;
    lxp_result status;
    if (lx_account_registry_init(&accounts) != LXP_OK) return 1;
    for (size_t i = 0U; i < 4U; ++i) {
        if (lx_account_id_from_string((const uint8_t *)names[i], strlen(names[i]),
                                      ids[i]) != LXP_OK ||
            lx_account_open(&accounts, (const uint8_t *)names[i], strlen(names[i]),
                            ids[i], 1U, LX_ACCOUNT_OPEN_GENESIS, NULL,
                            &opened[i]) != LXP_OK ||
            lxp_ledger_bootstrap_balance(opened[i], asset.asset_id,
                                         (lxp_u128){0U, 10U}, 0U) != LXP_OK)
            return fprintf(stderr, "Account setup failed at %zu\n", i), 1;
    }
    if (lxp_programs_account_derive(program_id, seed, sizeof(seed) - 1U,
                                    value_id) != LXP_OK ||
        lx_account_module_value_prepare(&accounts, (const uint8_t *)"programs",
            8U, value_id, asset.asset_id, 1U, &registration, &value,
            &created) != LXP_OK || !created ||
        lx_account_registration_commit(&accounts, &registration, &value) != LXP_OK ||
        lxp_ledger_bootstrap_balance(value, asset.asset_id,
                                     (lxp_u128){0U, 0U}, 0U) != LXP_OK ||
        lxp_did_id_derive(did, sizeof(did) - 1U, authority.principal) != LXP_OK)
        return fprintf(stderr, "Program value account setup failed\n"), 1;
    if (source == SOURCE_PRINCIPAL)
        (void)memcpy(authority.principal, ids[0], 32U);
    (void)memcpy(authority.actor, authority.principal, 32U);
    (void)memcpy(authority.verified_key, primary_key, 32U);
    authority.kind = LXP_AUTHORITY_OWNER;
    if (lxp_authority_hash(authority.kind, grant_id, primary_key,
                           authority.authority_hash) != LXP_OK)
        return 1;
    if (zero_authority)
        (void)memset(authority.authority_hash, 0, 32U);
    runtime = (lx_programs_transfer_runtime){
        .accounts = &accounts,
        .assets = &asset,
        .asset_count = 1U
    };
    if (!zero_program) (void)memcpy(payload, program_id, 32U);
    payload[33] = 1U;
    (void)memcpy(payload + 34U, ids[source_index], 32U);
    (void)memcpy(payload + 66U, asset.asset_id, 32U);
    (void)memcpy(payload + 98U, value_id, 32U);
    payload[145] = 4U;
    if (lxp_state_store_init(&state, 1U) != LXP_OK ||
        lxp_identity_register(&identities, did, sizeof(did) - 1U, primary_key,
                              &identity) != LXP_OK ||
        lxp_kernel_create(&kernel, &state, &journal, &parameters, 0U) != LXP_OK ||
        lxp_kernel_register_module(&kernel, programs_module_registration()) != LXP_OK ||
        lxp_kernel_set_capabilities(&kernel, NULL, apply_transfer_set) != LXP_OK ||
        lxp_kernel_bind_module_runtime(&kernel, LXP_MODULE_PROGRAMS, &runtime) != LXP_OK ||
        lxp_arena_init(&arena, arena_bytes, sizeof(arena_bytes)) != LXP_OK)
        return fprintf(stderr, "Kernel setup failed\n"), 1;
    activity.protocol_version = LXP_PROTOCOL_VERSION;
    activity.network_id = 7U;
    activity.activity_type = 0x00090005U;
    activity.actor_did = (lxp_byte_span){did, sizeof(did) - 1U};
    activity.authority = (lxp_byte_span){primary_key, sizeof(primary_key)};
    activity.timestamp_bound = (lxp_timestamp_bound){1U, 100U};
    activity.idempotency_key[31] = 1U;
    activity.payload = (lxp_byte_span){payload, sizeof(payload)};
    if (lxp_hash_payload(payload, sizeof(payload), activity.payload_hash) != LXP_OK)
        return 1;
    execution.network_id = 7U;
    execution.batch_number = 1U;
    execution.batch_timestamp_ms = 10U;
    execution.maximum_timestamp_window = 100U;
    execution.global_sequence = 1U;
    execution.recorded_module_version = LX_PROGRAMS_ABI_VERSION;
    execution.parameter_version = 1U;
    execution.signature_valid = true;
    execution.identities = &identities;
    execution.authority = &authority;
    execution.fee_parameters = &fee_parameters;
    execution.gas_limit = 100000U;
    execution.arena = &arena;
    status = lxp_kernel_execute_activity(&kernel, &activity, &execution, &receipt);
    failed = status != LXP_OK || receipt.result_code != expected ||
        receipt.module_id != LXP_MODULE_PROGRAMS ||
        value->balance.hi != 0U ||
        value->balance.lo != (expected == LXP_OK ? 4U : 0U) ||
        opened[0]->next_sequence != (expected == LXP_OK ? 1U : 0U) ||
        opened[1]->next_sequence != 0U || opened[2]->next_sequence != 0U ||
        opened[3]->next_sequence != 0U ||
        receipt.effects.count != (expected == LXP_OK ? 1U : 0U);
    for (size_t i = 0U; i < 4U; ++i)
        if (opened[i]->balance.hi != 0U ||
            opened[i]->balance.lo !=
                (expected == LXP_OK && i == source_index ? 6U : 10U))
            failed = 1;
    if (failed)
        (void)fprintf(stderr, "Transfer source=%d program_zero=%d authority_zero=%d "
                     "dispatch=%d receipt=%d expected=%d\n", (int)source,
                     (int)zero_program, (int)zero_authority, (int)status,
                     (int)receipt.result_code, (int)expected);
    lx_account_registry_release(&accounts);
    if (lxp_state_store_destroy(&state) != LXP_OK) failed = 1;
    return failed;
}

static int transfer_from_owned_main_account(void)
{
    return transfer_case(SOURCE_MAIN, false, false, LXP_OK) ||
        transfer_case(SOURCE_ASSET, false, false, LXP_OK) ||
        transfer_case(SOURCE_PRINCIPAL, false, false, LXP_OK);
}

static int transfer_from_foreign_account_refused(void)
{
    return transfer_case(SOURCE_FOREIGN, false, false, LXP_ERR_AUTH_SCOPE) ||
        transfer_case(SOURCE_UNOWNED, false, false, LXP_ERR_AUTH_SCOPE) ||
        transfer_case(SOURCE_MAIN, true, false, LXP_ERR_NON_CANONICAL) ||
        transfer_case(SOURCE_MAIN, false, true, LXP_ERR_NON_CANONICAL);
}

int main(void)
{
    int owned = transfer_from_owned_main_account();
    int foreign = transfer_from_foreign_account_refused();
    (void)printf("transfer_from_owned_main_account: %s\n", owned ? "FAIL" : "PASS");
    (void)printf("transfer_from_foreign_account_refused: %s\n",
                 foreign ? "FAIL" : "PASS");
    return owned || foreign;
}
