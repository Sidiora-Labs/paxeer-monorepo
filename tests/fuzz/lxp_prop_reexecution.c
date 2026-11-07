#define main reexecution_integrated_fixture_main
#include "lxp_prop_kernel_integrated.c"
#undef main

extern const unsigned char lxp_replay_record __attribute__((weak));
extern lxp_result lxp_determinism_guard_check(void) __attribute__((weak));
extern lxp_result lxp_determinism_guard_trip(const char *symbol)
    __attribute__((weak));
extern void lxp_determinism_guard_reset(void) __attribute__((weak));
extern lxp_result lxp_kernel_replay(lxp_kernel *kernel, const void *records,
    const uint8_t (*expected_roots)[32], size_t record_count,
    size_t worker_threads, uint8_t terminal_root[32]) __attribute__((weak));
extern lxp_result lxp_replay_compare_roots(const uint8_t expected[32],
    const uint8_t produced[32]) __attribute__((weak));
extern lxp_result lxp_replay_golden_run(const void *records, size_t record_count,
    const uint8_t (*roots)[32], size_t worker_threads, uint8_t digest[32])
    __attribute__((weak));

typedef struct reexecution_record {
    lxp_byte_span activity;
    lxp_byte_span receipt;
    uint8_t root[32];
} reexecution_record;

static int reexecution_copy(lxp_byte_span source, lxp_byte_span *recorded)
{
    uint8_t *bytes = malloc(source.length);
    LXP_CASE_CHECK(bytes != NULL);
    (void)memcpy(bytes, source.bytes, source.length);
    *recorded = (lxp_byte_span){bytes, source.length};
    return 0;
}

static void reexecution_execution(integrated_fixture *fixture, size_t index,
    const lxp_fee_params *fees, lxp_authority_resolved *authority,
    lxp_kernel_execution *execution)
{
    (void)memset(execution, 0, sizeof(*execution));
    (void)memset(authority, 0, sizeof(*authority));
    execution->network_id = 7U;
    execution->batch_number = 1U + index / 8U;
    execution->batch_timestamp_ms = 1000U + index * 37U;
    execution->maximum_timestamp_window = 100U;
    execution->global_sequence = index + 1U;
    execution->recorded_module_version = 1U;
    execution->parameter_version = 1U;
    execution->signature_valid = true;
    execution->fee_parameters = fees;
    execution->gas_limit = 1000000U;
    execution->sequencer_private_key = integrated_seed;
    execution->batch_id[31] = (uint8_t)execution->batch_number;
    execution->identities = &fixture->identities;
    execution->arena = &fixture->arena;
    authority->kind = LXP_AUTHORITY_OWNER;
    (void)memcpy(authority->actor, fixture->identity->did_id, 32U);
    (void)memcpy(authority->verified_key, fixture->public_key, 32U);
    (void)memcpy(authority->principal,
        fixture->account_ids[index % 2U == 0U ? 0U : 2U], 32U);
    execution->authority = authority;
}

static int reexecution_equal_roots(void)
{
    lxp_genesis_manifest *manifest = calloc(1U, sizeof(*manifest));
    integrated_fixture *fixtures = calloc(2U, sizeof(*fixtures));
    reexecution_record records[INTEGRATED_ACTIVITIES] = {0};
    lxp_fee_params fees = {.version = 1U, .multiplier_basis_points = 10000U};
    uint8_t genesis_root[32];
    size_t side, index;

    LXP_CASE_CHECK(manifest != NULL && fixtures != NULL);
    LXP_CASE_CHECK(integrated_genesis(manifest) == 0);
    for (side = 0U; side < 2U; ++side)
        LXP_CASE_CHECK(integrated_fixture_init(&fixtures[side], manifest) == 0);
    LXP_CASE_CHECK(integrated_load(&fixtures[0], &fixtures[1],
        UINT64_C(0x726565786563)) == 0);
    (void)memcpy(genesis_root, fixtures[0].kernel.current_state_root, 32U);

    for (side = 0U; side < 2U; ++side) {
        integrated_fixture *fixture = &fixtures[side];
        LXP_CASE_CHECK(memcmp(fixture->kernel.current_state_root,
            genesis_root, 32U) == 0);
        LXP_CASE_CHECK(fixture->identity->next_sequence == 0U);
        for (index = 0U; index < INTEGRATED_ACTIVITIES; ++index) {
            lxp_kernel_execution execution;
            lxp_authority_resolved authority;
            lxp_activity activity;
            lxp_receipt receipt;
            lxp_result status;
            lxp_byte_span encoded;
            uint8_t payload[512], signature[64], before[32], root[32];

            LXP_CASE_CHECK(lxp_arena_reset(&fixture->arena, 0U) == LXP_OK);
            reexecution_execution(fixture, index, &fees, &authority, &execution);
            if (side == 0U) {
                LXP_CASE_CHECK(integrated_activity(fixture, index, &execution,
                    &activity, payload, signature) == 0);
                LXP_CASE_CHECK(lxp_activity_encode(&activity, &fixture->arena,
                    &encoded) == LXP_OK);
                LXP_CASE_CHECK(reexecution_copy(encoded,
                    &records[index].activity) == 0);
            }
            LXP_CASE_CHECK(lxp_activity_decode(records[index].activity.bytes,
                records[index].activity.length, &activity) == LXP_OK);
            LXP_CASE_CHECK(lxp_activity_verify_signature(&activity) == LXP_OK);
            LXP_CASE_CHECK(lxp_activity_encode(&activity, &fixture->arena,
                &encoded) == LXP_OK);
            LXP_CASE_CHECK(encoded.length == records[index].activity.length);
            LXP_CASE_CHECK(memcmp(encoded.bytes, records[index].activity.bytes,
                encoded.length) == 0);
            LXP_CASE_CHECK(lxp_arena_reset(&fixture->arena, 0U) == LXP_OK);
            (void)memcpy(before, fixture->kernel.current_state_root, 32U);
            status = lxp_kernel_execute_activity(&fixture->kernel,
                &activity, &execution, &receipt);
            if (status != LXP_OK)
                (void)fprintf(stderr, "side %zu, activity %zu: execution %d\n",
                    side, index, (int)status);
            LXP_CASE_CHECK(status == LXP_OK);
            LXP_CASE_CHECK(receipt.result_code == LXP_OK);
            LXP_CASE_CHECK(receipt.global_sequence == index + 1U);
            LXP_CASE_CHECK(fixture->state.next_sequence == index + 2U);
            LXP_CASE_CHECK(fixture->identity->next_sequence == index + 1U);
            LXP_CASE_CHECK(!fixture->journal.open);
            LXP_CASE_CHECK(memcmp(receipt.previous_state_root, before, 32U) == 0);
            LXP_CASE_CHECK(lxp_state_root(&fixture->kernel, root) == LXP_OK);
            LXP_CASE_CHECK(memcmp(root, before, 32U) != 0);
            LXP_CASE_CHECK(memcmp(root,
                fixture->kernel.current_state_root, 32U) == 0);
            LXP_CASE_CHECK(memcmp(root,
                receipt.resulting_state_root, 32U) == 0);
            LXP_CASE_CHECK(lxp_receipt_encode(&receipt, true, &fixture->arena,
                &encoded) == LXP_OK);
            LXP_CASE_CHECK(lxp_receipt_verify(&receipt, fixture->public_key,
                &fixture->arena) == LXP_OK);
            if (side == 0U) {
                LXP_CASE_CHECK(reexecution_copy(encoded,
                    &records[index].receipt) == 0);
                (void)memcpy(records[index].root, root, 32U);
            } else {
                LXP_CASE_CHECK(encoded.length == records[index].receipt.length);
                LXP_CASE_CHECK(memcmp(encoded.bytes,
                    records[index].receipt.bytes, encoded.length) == 0);
                LXP_CASE_CHECK(memcmp(root, records[index].root, 32U) == 0);
            }
        }
        {
            const lx_account *from, *to;
            size_t from_slot, to_slot;
            LXP_CASE_CHECK(lx_account_registry_index_lookup(&fixture->accounts,
                fixture->account_ids[0], &from_slot) == LXP_OK);
            LXP_CASE_CHECK(lx_account_registry_index_lookup(&fixture->accounts,
                fixture->account_ids[1], &to_slot) == LXP_OK);
            from = &fixture->accounts.accounts[from_slot];
            to = &fixture->accounts.accounts[to_slot];
            LXP_CASE_CHECK(from->balance.hi == 0U && from->balance.lo == 9968U);
            LXP_CASE_CHECK(to->balance.hi == 0U && to->balance.lo == 32U);
            LXP_CASE_CHECK(from->next_sequence == 32U);
        }
    }
    for (index = 0U; index < INTEGRATED_ACTIVITIES; ++index) {
        free((void *)records[index].activity.bytes);
        free((void *)records[index].receipt.bytes);
    }
    for (side = 0U; side < 2U; ++side) {
        LXP_CASE_CHECK(lxp_state_store_destroy(&fixtures[side].state) == LXP_OK);
        lx_account_registry_release(&fixtures[side].accounts);
    }
    free(fixtures);
    free(manifest);
    (void)printf("reexecution_equal_roots: 64 recorded activities, receipts and roots match\n");
    return 0;
}

static int determinism_guard_removed(void)
{
    LXP_CASE_CHECK(&lxp_replay_record == NULL);
    LXP_CASE_CHECK(lxp_determinism_guard_check == NULL);
    LXP_CASE_CHECK(lxp_determinism_guard_trip == NULL);
    LXP_CASE_CHECK(lxp_determinism_guard_reset == NULL);
    LXP_CASE_CHECK(lxp_kernel_replay == NULL);
    LXP_CASE_CHECK(lxp_replay_compare_roots == NULL);
    LXP_CASE_CHECK(lxp_replay_golden_run == NULL);
    (void)printf("determinism_guard_removed: all retired weak symbols are absent\n");
    return 0;
}

int main(int argc, char **argv)
{
    static const lxp_kernel_case cases[] = {
        {"reexecution_equal_roots", reexecution_equal_roots},
        {"determinism_guard_removed", determinism_guard_removed}
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
