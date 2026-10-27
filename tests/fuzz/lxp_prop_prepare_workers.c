#define main prepare_workers_call_fixture_main
#include "../programs/test_call_activity.c"
#undef main

#include "lxp_kernel_case.h"

enum {
    PREPARE_WORKERS_BATCH_SIZE = 16,
    PREPARE_WORKERS_SAMPLES = 50,
    PREPARE_WORKERS_ARENA_BYTES = 2 * LXP_MAX_ACTIVITY_BYTES + 65536
};

lxp_result lxp_kernel_prepare_activity_batch(
    lxp_kernel *kernel, const lxp_activity *activities,
    const lxp_kernel_execution *executions, size_t offered_count,
    uint32_t maximum_workers, lxp_kernel_prepared_batch **batch,
    size_t *retry_prefix_count);

typedef struct prepare_workers_fixture {
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_kernel kernel;
    lx_account_registry accounts;
    lxp_identity_store identities;
    lx_programs_transfer_runtime runtime;
    lxp_transfer_asset_state asset;
    lxp_fee_params fees;
    lxp_authority_resolved authorities[PREPARE_WORKERS_BATCH_SIZE];
    lxp_authority_scope scopes[PREPARE_WORKERS_BATCH_SIZE];
    uint8_t dids[PREPARE_WORKERS_BATCH_SIZE][40];
    size_t did_lengths[PREPARE_WORKERS_BATCH_SIZE];
    uint8_t payers[PREPARE_WORKERS_BATCH_SIZE][32];
    uint8_t programs[PREPARE_WORKERS_BATCH_SIZE][32];
    uint8_t public_key[32];
} prepare_workers_fixture;

static const uint8_t prepare_workers_asset[32] = {9U};
static const uint8_t prepare_workers_seed[32] = {0x33U};

static int prepare_workers_genesis(lxp_genesis_manifest *manifest)
{
    lx_programs_metering_schedule metering = {0};
    lx_programs_fee_genesis_parameters fees = {0};
    size_t index;

    manifest->protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    manifest->network_id = 7U;
    manifest->genesis_timestamp_ms = 1U;
    manifest->parameter_count = 1U;
    manifest->parameters[0].module_id = LXP_MODULE_GOVERNANCE;
    (void)memcpy(manifest->parameters[0].key, "parameter-version", 17U);
    manifest->parameters[0].value[31] = 1U;
    manifest->guarantor_count = 1U;
    manifest->guarantors[0].guarantor_id[0] = 1U;
    (void)memset(manifest->guarantors[0].public_key, 1, 33U);
    LXP_CASE_CHECK(executed_public_key(prepare_workers_seed,
                                      manifest->signer_public_key) == 0);
    metering.version = 1U;
    for (index = 0U; index < 5U; ++index)
        metering.coefficients[index] = 1U;
    metering.coefficients[5] = 8U;
    metering.coefficients[6] = 8U;
    metering.coefficients[7] = 64U;
    metering.coefficients[8] = 8U;
    metering.activation_batch = 1U;
    metering.authority_kind = LX_PROGRAMS_METERING_AUTHORITY_GENESIS;
    LXP_CASE_CHECK(lxp_hash_payload(manifest->signer_public_key, 32U,
                                    metering.authority_digest) == LXP_OK);
    fees.schedule = (lx_programs_fee_schedule){1U, 1U, 1U, 2U, 4U, 1U, 1U, 1U};
    (void)memcpy(fees.occupancy_asset_id, prepare_workers_asset, 32U);
    fees.target_occupancy_byte_batches = 100U;
    fees.response_denominator = 1U;
    fees.maximum_change_numerator = 1U;
    fees.maximum_change_denominator = 10U;
    fees.minimum_fee_units_per_occupancy_byte_batch = 1U;
    fees.maximum_fee_units_per_occupancy_byte_batch = 1000U;
    LXP_CASE_CHECK(lxp_genesis_fresh_empty_accounts(manifest,
                                                  prepare_workers_asset) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_metering_genesis_append(manifest,
                                                       &metering) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_fee_genesis_append(manifest, &fees) == LXP_OK);
    return 0;
}

static void prepare_workers_execution(prepare_workers_fixture *fixture,
                                      size_t actor, lxp_arena *arena,
                                      lxp_kernel_execution *execution)
{
    (void)memset(execution, 0, sizeof(*execution));
    execution->network_id = 7U;
    execution->epoch = fixture->kernel.epoch;
    execution->batch_number = 1U;
    execution->batch_timestamp_ms = 20U;
    execution->maximum_timestamp_window = 100U;
    execution->recorded_module_version = LX_PROGRAMS_SANDBOX_DESTROY_ABI_VERSION;
    execution->parameter_version = 1U;
    execution->signature_valid = true;
    execution->identities = &fixture->identities;
    execution->authority = &fixture->authorities[actor];
    execution->fee_parameters = &fixture->fees;
    execution->fee_balance = (lxp_u128){0U, UINT64_MAX};
    execution->gas_limit = 1000000U;
    execution->arena = arena;
    execution->sequencer_private_key = executed_sequencer_seed;
}

static int prepare_workers_fixture_init(prepare_workers_fixture *fixture,
                                        lxp_genesis_manifest *manifest,
                                        lxp_arena *arena)
{
    static const uint8_t success_entry[] = {0x41U, 0U, 0x0bU};
    lxp_genesis_module_plan plan;
    uint8_t wasm[128], payload[2048], code_hash[32];
    size_t wasm_length = candidate_module(wasm, success_entry,
                                          sizeof(success_entry));
    size_t index;

    LXP_CASE_CHECK(lx_account_registry_init(&fixture->accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_init(&fixture->state, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_bind_accounts(&fixture->state,
                                                &fixture->accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&fixture->kernel, &fixture->state,
                                     &fixture->journal, manifest, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_genesis_module_plan_resolve(manifest, &plan) == LXP_OK);
    LXP_CASE_CHECK(lxp_genesis_module_plan_register(&plan,
                                                   &fixture->kernel) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_reset(arena, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_genesis_materialize(manifest, arena,
                                          &fixture->kernel) == LXP_OK);
    LXP_CASE_CHECK(executed_public_key(prepare_workers_seed,
                                      fixture->public_key) == 0);
    for (index = 0U; index < PREPARE_WORKERS_BATCH_SIZE; ++index) {
        uint8_t name[64], actor_id[32];
        lx_account *actor;
        lxp_identity *identity;
        lxp_authority_resolved *authority = &fixture->authorities[index];
        lxp_authority_scope *scope = &fixture->scopes[index];
        int length = snprintf((char *)fixture->dids[index],
                              sizeof(fixture->dids[index]),
                              "did:lxp:prepare-worker-%02zu", index);
        LXP_CASE_CHECK(length > 0 &&
                       (size_t)length < sizeof(fixture->dids[index]));
        fixture->did_lengths[index] = (size_t)length;
        length = snprintf((char *)name, sizeof(name), "agent:%s:main",
                           (const char *)fixture->dids[index]);
        LXP_CASE_CHECK(length > 0 && (size_t)length < sizeof(name));
        LXP_CASE_CHECK(lxp_identity_register(&fixture->identities,
            fixture->dids[index], fixture->did_lengths[index],
            fixture->public_key, &identity) == LXP_OK);
        LXP_CASE_CHECK(lx_account_id_from_string(name, (size_t)length,
                                                actor_id) == LXP_OK);
        LXP_CASE_CHECK(lx_account_open(&fixture->accounts, name, (size_t)length,
            actor_id, index + 1U, LX_ACCOUNT_OPEN_GENESIS, NULL, &actor) == LXP_OK);
        (void)memcpy(fixture->payers[index], actor->id, 32U);
        LXP_CASE_CHECK(lxp_ledger_bootstrap_balance(actor, prepare_workers_asset,
            (lxp_u128){0U, UINT64_MAX}, index + 1U) == LXP_OK);
        scope->module_mask = UINT64_C(1) << LXP_MODULE_PROGRAMS;
        scope->activity_ordinal_min = 1U;
        scope->activity_ordinal_max = 9U;
        scope->maximum_per_activity = (lxp_u128){UINT64_MAX, UINT64_MAX};
        scope->maximum_total = scope->maximum_per_activity;
        scope->maximum_per_period = scope->maximum_per_activity;
        authority->kind = LXP_AUTHORITY_OWNER;
        authority->scope = scope;
        (void)memcpy(authority->actor, identity->did_id, 32U);
        (void)memcpy(authority->principal, identity->did_id, 32U);
        (void)memcpy(authority->verified_key, fixture->public_key, 32U);
        LXP_CASE_CHECK(lxp_authority_hash(authority->kind,
            (const uint8_t[32]){0U}, fixture->public_key,
            authority->authority_hash) == LXP_OK);
        fixture->programs[index][0] = 0x31U;
        fixture->programs[index][31] = (uint8_t)(index + 1U);
    }
    fixture->asset.registered = true;
    (void)memcpy(fixture->asset.asset_id, prepare_workers_asset, 32U);
    fixture->runtime.accounts = &fixture->accounts;
    fixture->runtime.assets = &fixture->asset;
    fixture->runtime.asset_count = 1U;
    fixture->runtime.fee_schedule =
        (lx_programs_fee_schedule){1U, 1U, 1U, 2U, 4U, 1U, 1U, 1U};
    fixture->runtime.resolve_metering_schedule = lxp_programs_metering_resolve_runtime;
    fixture->runtime.metering_schedule_context = &fixture->kernel;
    fixture->runtime.resolve_occupancy_parameters = occupancy_parameters;
    fixture->runtime.occupancy_parameter_context = &fixture->runtime;
    (void)memcpy(fixture->runtime.occupancy_asset_id, prepare_workers_asset, 32U);
    fixture->fees.version = 1U;
    fixture->fees.multiplier_basis_points = 10000U;
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&fixture->kernel,
        LXP_MODULE_PROGRAMS, &fixture->runtime) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_bind_fee_transaction(&fixture->kernel) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_set_capabilities(&fixture->kernel, NULL,
        lxp_kernel_canonical_ledger_apply) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_root(&fixture->kernel,
                                  fixture->kernel.current_state_root) == LXP_OK);
    for (index = 0U; index < PREPARE_WORKERS_BATCH_SIZE; ++index) {
        lxp_activity activity;
        lxp_kernel_execution execution;
        lxp_receipt receipt;
        size_t length = deploy_payload(payload, fixture->programs[index],
            fixture->authorities[index].principal, wasm, wasm_length,
            code_hash, LX_PROGRAMS_ABI_VERSION, INTERFACE_CAPABILITIES_NONE);
        fill_activity(&activity, LX_PROGRAMS_DEPLOY, payload, length,
            fixture->dids[index], fixture->did_lengths[index], fixture->public_key);
        activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
        LXP_CASE_CHECK(lxp_arena_reset(arena, 0U) == LXP_OK);
        prepare_workers_execution(fixture, index, arena, &execution);
        execution.global_sequence = fixture->state.next_sequence;
        LXP_CASE_CHECK(execute_artifact_fixture_activity(&fixture->kernel,
            &activity, &execution, &receipt) == LXP_OK);
        LXP_CASE_CHECK(receipt.result_code == LXP_OK);
        (void)memcpy(fixture->authorities[index].principal,
                     fixture->payers[index], 32U);
    }
    return 0;
}

static uint64_t prepare_workers_random(uint64_t *state)
{
    *state ^= *state << 13U;
    *state ^= *state >> 7U;
    *state ^= *state << 17U;
    return *state;
}

static size_t prepare_workers_payload(uint8_t *payload,
                                      const uint8_t program[32],
                                      const uint8_t *data, size_t length)
{
    static const uint8_t declaration_domain[] =
        "LayerX/programs/access-declaration/v1\0";
    static const uint8_t set_domain[] = "LayerX/programs/access-set/v1\0";
    const uint8_t capabilities[2] = {0U, 0U};
    uint8_t declaration[128];
    size_t offset = 0U;

    (void)memcpy(declaration, declaration_domain, sizeof(declaration_domain) - 1U);
    offset += sizeof(declaration_domain) - 1U;
    declaration[offset++] = 1U;
    write_u32(declaration + offset, (uint32_t)(sizeof(set_domain) - 1U + 6U));
    offset += 4U;
    (void)memcpy(declaration + offset, set_domain, sizeof(set_domain) - 1U);
    offset += sizeof(set_domain) - 1U;
    (void)memset(declaration + offset, 0, 6U);
    offset += 6U;
    return call_payload_with_data(payload, program, capabilities,
        sizeof(capabilities), declaration, offset, data, length);
}

static int prepare_workers_independent(prepare_workers_fixture *fixture,
                                       const lxp_activity *activities,
                                       const lxp_kernel_execution *executions)
{
    lxp_kernel_batch_snapshot *snapshot = NULL;
    lxp_programs_schedule_item *items = calloc(PREPARE_WORKERS_BATCH_SIZE,
                                               sizeof(*items));
    uint16_t levels[PREPARE_WORKERS_BATCH_SIZE] = {0};
    uint16_t maximum_level = UINT16_MAX;
    lxp_result status;
    size_t index;

    if (items == NULL) return 1;
    status = lxp_kernel_batch_snapshot_create(&fixture->kernel,
        &fixture->identities, NULL, &executions[0], &snapshot);
    for (index = 0U; status == LXP_OK && index < PREPARE_WORKERS_BATCH_SIZE;
         ++index) {
        size_t mark = lxp_arena_mark(executions[index].arena);
        status = lxp_kernel_batch_schedule_item(snapshot, &activities[index],
            &executions[index], executions[index].arena, &items[index]);
        if (lxp_arena_reset(executions[index].arena, mark) != LXP_OK)
            status = LXP_FATAL_INVARIANT;
    }
    if (status == LXP_OK)
        status = layerx_programs_schedule_plan(items, PREPARE_WORKERS_BATCH_SIZE,
                                               levels, &maximum_level);
    free(items);
    lxp_kernel_batch_snapshot_destroy(snapshot);
    LXP_CASE_CHECK(status == LXP_OK);
    LXP_CASE_CHECK(maximum_level == 0U);
    for (index = 0U; index < PREPARE_WORKERS_BATCH_SIZE; ++index)
        LXP_CASE_CHECK(levels[index] == 0U);
    return 0;
}

static int prepare_workers_span_equal(lxp_byte_span left, lxp_byte_span right)
{
    LXP_CASE_CHECK(left.length == right.length);
    if (left.length != 0U) {
        LXP_CASE_CHECK(left.bytes != NULL && right.bytes != NULL);
        LXP_CASE_CHECK(memcmp(left.bytes, right.bytes, left.length) == 0);
    }
    return 0;
}

static int prepare_workers_compare(const lxp_kernel_prepared_batch *serial,
                                    const lxp_kernel_prepared_batch *parallel,
                                    lxp_arena *arena)
{
    const lxp_receipt *left = lxp_kernel_prepared_batch_receipts(serial);
    const lxp_receipt *right = lxp_kernel_prepared_batch_receipts(parallel);
    const lxp_byte_span *left_events = lxp_kernel_prepared_batch_events(serial);
    const lxp_byte_span *right_events = lxp_kernel_prepared_batch_events(parallel);
    uint8_t serial_root[32], parallel_root[32];
    size_t index;

    LXP_CASE_CHECK(lxp_kernel_prepared_batch_count(serial) == PREPARE_WORKERS_BATCH_SIZE);
    LXP_CASE_CHECK(lxp_kernel_prepared_batch_count(parallel) == PREPARE_WORKERS_BATCH_SIZE);
    LXP_CASE_CHECK(left != NULL && right != NULL);
    LXP_CASE_CHECK(left_events != NULL && right_events != NULL);
    LXP_CASE_CHECK(memcmp(lxp_kernel_prepared_batch_final_root(serial),
        lxp_kernel_prepared_batch_final_root(parallel), 32U) == 0);
    LXP_CASE_CHECK(lxp_state_root(lxp_kernel_prepared_batch_settled_kernel(serial),
                                  serial_root) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_root(lxp_kernel_prepared_batch_settled_kernel(parallel),
                                  parallel_root) == LXP_OK);
    LXP_CASE_CHECK(memcmp(serial_root, parallel_root, 32U) == 0);
    for (index = 0U; index < PREPARE_WORKERS_BATCH_SIZE; ++index) {
        lxp_byte_span serial_bytes, parallel_bytes;
        LXP_CASE_CHECK(left[index].result_code == LXP_OK);
        LXP_CASE_CHECK(right[index].result_code == LXP_OK);
        LXP_CASE_CHECK(left[index].program_outcome.present);
        LXP_CASE_CHECK(right[index].program_outcome.present);
        LXP_CASE_CHECK(left[index].program_outcome.terminal_kind ==
                        LXP_PROGRAM_TERMINAL_SUCCESS);
        LXP_CASE_CHECK(right[index].program_outcome.terminal_kind ==
                        LXP_PROGRAM_TERMINAL_SUCCESS);
        LXP_CASE_CHECK(memcmp(left[index].previous_state_root,
                              right[index].previous_state_root, 32U) == 0);
        LXP_CASE_CHECK(memcmp(left[index].resulting_state_root,
                              right[index].resulting_state_root, 32U) == 0);
        LXP_CASE_CHECK(lxp_arena_reset(arena, 0U) == LXP_OK);
        LXP_CASE_CHECK(lxp_receipt_encode(&left[index], true, arena,
                                          &serial_bytes) == LXP_OK);
        LXP_CASE_CHECK(lxp_receipt_encode(&right[index], true, arena,
                                          &parallel_bytes) == LXP_OK);
        LXP_CASE_CHECK(prepare_workers_span_equal(serial_bytes, parallel_bytes) == 0);
        LXP_CASE_CHECK(prepare_workers_span_equal(
            left[index].program_outcome.terminal_payload,
            right[index].program_outcome.terminal_payload) == 0);
        LXP_CASE_CHECK(prepare_workers_span_equal(
            left[index].program_outcome.call_graph_payload,
            right[index].program_outcome.call_graph_payload) == 0);
        LXP_CASE_CHECK(prepare_workers_span_equal(left_events[index],
                                                 right_events[index]) == 0);
    }
    return 0;
}

static void prepare_workers_fixture_destroy(prepare_workers_fixture *fixture)
{
    if (fixture == NULL) return;
    while (fixture->kernel.blob_count != 0U)
        free(fixture->kernel.blobs[--fixture->kernel.blob_count].bytes);
    (void)lxp_state_store_destroy(&fixture->state);
    lx_account_registry_release(&fixture->accounts);
    free(fixture);
}

static int prepare_workers_sample(prepare_workers_fixture *serial,
                                  prepare_workers_fixture *parallel,
                                  size_t sample, uint64_t *random,
                                  lxp_arena *encode_arena,
                                  lxp_arena *scratch)
{
    lxp_activity activities[PREPARE_WORKERS_BATCH_SIZE];
    lxp_kernel_execution executions[2][PREPARE_WORKERS_BATCH_SIZE];
    uint8_t payloads[PREPARE_WORKERS_BATCH_SIZE][1024];
    uint8_t signatures[PREPARE_WORKERS_BATCH_SIZE][64];
    lxp_byte_span encoded[PREPARE_WORKERS_BATCH_SIZE];
    lxp_batch_roots roots;
    uint8_t preimage[88], initial_root[32];
    lxp_kernel_prepared_batch *prepared[2] = {NULL, NULL};
    size_t retry[2] = {SIZE_MAX, SIZE_MAX};
    lxp_result status = LXP_OK;
    int result = 1;
    size_t index;

    LXP_CASE_CHECK(lxp_arena_reset(encode_arena, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_reset(scratch, 0U) == LXP_OK);
    (void)memcpy(initial_root, serial->kernel.current_state_root, 32U);
    LXP_CASE_CHECK(memcmp(initial_root, parallel->kernel.current_state_root, 32U) == 0);
    for (index = 0U; index < PREPARE_WORKERS_BATCH_SIZE; ++index) {
        size_t actor = (index + sample) % PREPARE_WORKERS_BATCH_SIZE;
        uint8_t data[64];
        size_t length = 1U + (size_t)(prepare_workers_random(random) % sizeof(data));
        size_t offset;
        uint8_t public_key[32];
        for (offset = 0U; offset < length; ++offset)
            data[offset] = (uint8_t)prepare_workers_random(random);
        length = prepare_workers_payload(payloads[index], serial->programs[actor],
                                          data, length);
        fill_activity(&activities[index], LX_PROGRAMS_CALL, payloads[index], length,
            serial->dids[actor], serial->did_lengths[actor], serial->public_key);
        activities[index].protocol_version = LXP_PROTOCOL_VERSION_LEGACY;
        activities[index].account_sequence = 1U;
        activities[index].fee_limit = (lxp_u128){0U, 67108864U};
        activities[index].idempotency_key[30] = (uint8_t)(sample + 1U);
        activities[index].idempotency_key[31] = (uint8_t)(index + 2U);
        activities[index].signature = (lxp_byte_span){signatures[index], 64U};
        LXP_CASE_CHECK(lifecycle_vector_signature(&activities[index], public_key,
                                                   signatures[index]) == 0);
        LXP_CASE_CHECK(lxp_activity_verify_signature(&activities[index]) == LXP_OK);
        LXP_CASE_CHECK(lxp_activity_encode(&activities[index], encode_arena,
                                           &encoded[index]) == LXP_OK);
        prepare_workers_execution(serial, actor, scratch, &executions[0][index]);
        prepare_workers_execution(parallel, actor, scratch, &executions[1][index]);
        executions[0][index].global_sequence = serial->state.next_sequence + index;
        executions[1][index].global_sequence = parallel->state.next_sequence + index;
        executions[0][index].batch_number = sample + 2U;
        executions[1][index].batch_number = sample + 2U;
    }
    LXP_CASE_CHECK(lxp_batch_roots_compute(
        &(lxp_batch_root_inputs){encoded, PREPARE_WORKERS_BATCH_SIZE,
                                 NULL, 0U, NULL, 0U, NULL, 0U, NULL, 0U},
        encode_arena, &roots) == LXP_OK);
    (void)memcpy(preimage, initial_root, 32U);
    (void)memcpy(preimage + 32U, roots.activity_merkle_root, 32U);
    write_u64(preimage + 64U, executions[0][0].global_sequence);
    write_u64(preimage + 72U,
               executions[0][PREPARE_WORKERS_BATCH_SIZE - 1U].global_sequence);
    write_u64(preimage + 80U, executions[0][0].batch_number);
    for (index = 0U; index < PREPARE_WORKERS_BATCH_SIZE; ++index) {
        size_t run;
        for (run = 0U; run < 2U; ++run) {
            LXP_CASE_CHECK(lxp_hash_context_value(preimage, sizeof(preimage),
                executions[run][index].batch_id) == LXP_OK);
            (void)memcpy(executions[run][index].activity_root,
                         roots.activity_merkle_root, 32U);
        }
    }
    LXP_CASE_CHECK(prepare_workers_independent(serial, activities, executions[0]) == 0);
    status = lxp_kernel_prepare_activity_batch(&serial->kernel, activities,
        executions[0], PREPARE_WORKERS_BATCH_SIZE, 1U, &prepared[0], &retry[0]);
    if (status != LXP_OK) goto done;
    status = lxp_kernel_prepare_activity_batch(&parallel->kernel, activities,
        executions[1], PREPARE_WORKERS_BATCH_SIZE, 4U, &prepared[1], &retry[1]);
    if (status != LXP_OK) goto done;
    if (retry[0] != retry[1] || retry[0] != 0U) goto done;
    if (prepare_workers_compare(prepared[0], prepared[1], scratch) != 0) goto done;
    if (memcmp(initial_root, serial->kernel.current_state_root, 32U) != 0 ||
        memcmp(initial_root, parallel->kernel.current_state_root, 32U) != 0)
        goto done;
    result = 0;
done:
    lxp_kernel_prepared_batch_destroy(prepared[0]);
    lxp_kernel_prepared_batch_destroy(prepared[1]);
    if (result != 0)
        (void)fprintf(stderr,
            "prepare workers sample=%zu status=%d retry=%zu/%zu failed\n",
            sample, (int)status, retry[0], retry[1]);
    return result;
}

static int prepare_one_vs_four_workers_equal(void)
{
    const size_t encode_capacity =
        (PREPARE_WORKERS_BATCH_SIZE + 4U) * (size_t)LXP_MAX_ACTIVITY_BYTES;
    lxp_genesis_manifest *manifest = calloc(1U, sizeof(*manifest));
    prepare_workers_fixture *serial = calloc(1U, sizeof(*serial));
    prepare_workers_fixture *parallel = calloc(1U, sizeof(*parallel));
    uint8_t *encode_bytes = malloc(encode_capacity);
    uint8_t *scratch_bytes = malloc(PREPARE_WORKERS_ARENA_BYTES);
    lxp_arena encode_arena, scratch;
    uint64_t random = UINT64_C(0x1431019cafe51eed);
    int result = 1;
    size_t sample;

    if (manifest == NULL || serial == NULL || parallel == NULL ||
        encode_bytes == NULL || scratch_bytes == NULL) goto done;
    if (lxp_arena_init(&encode_arena, encode_bytes, encode_capacity) != LXP_OK ||
        lxp_arena_init(&scratch, scratch_bytes, PREPARE_WORKERS_ARENA_BYTES) != LXP_OK)
        goto done;
    if (prepare_workers_genesis(manifest) != 0 ||
        prepare_workers_fixture_init(serial, manifest, &scratch) != 0 ||
        prepare_workers_fixture_init(parallel, manifest, &scratch) != 0)
        goto done;
    for (sample = 0U; sample < PREPARE_WORKERS_SAMPLES; ++sample)
        if (prepare_workers_sample(serial, parallel, sample, &random,
                                   &encode_arena, &scratch) != 0)
            goto done;
    (void)printf("prepare_one_vs_four_workers_equal: 50 batches of 16 calls passed\n");
    result = 0;
done:
    prepare_workers_fixture_destroy(serial);
    prepare_workers_fixture_destroy(parallel);
    free(manifest);
    free(encode_bytes);
    free(scratch_bytes);
    return result;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        {"prepare_one_vs_four_workers_equal", prepare_one_vs_four_workers_equal}
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
