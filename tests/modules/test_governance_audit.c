#define _POSIX_C_SOURCE 200809L

#include "../../src/modules/governance/lxp_emergency_store.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_module_ctx.h"
#include "layerx/lxp_snapshot.h"
#include "layerx/lxp_storage.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#define CHECK(condition) do { \
    if (!(condition)) { \
        fprintf(stderr, "%s:%d: %s\n", __func__, __LINE__, #condition); \
        return 1; \
    } \
} while (0)

static const uint8_t trigger[32] = {1U};
static const uint8_t exit_conditions[32] = {2U};
static const uint8_t market[32] = {3U};

static int audit_emergency_resume_validation(void)
{
    lxp_gov_emergency_state state;
    lxp_gov_emergency_state before;
    const uint8_t zero[32] = {0U};
    struct invalid_scope {
        lxp_pause_scope scope;
        uint16_t module_id;
        const uint8_t *market_id;
    } invalid[] = {
        {LXP_PAUSE_MARKET, 7U, NULL},
        {LXP_PAUSE_MARKET, 7U, zero},
        {LXP_PAUSE_MARKET, 0U, market},
        {LXP_PAUSE_MARKET, 256U, market},
        {LXP_PAUSE_MODULE, 0U, NULL},
        {LXP_PAUSE_MODULE, 256U, NULL},
        {LXP_PAUSE_NETWORK, 7U, NULL},
        {(lxp_pause_scope)0, 7U, market},
        {(lxp_pause_scope)4, 7U, market}
    };
    size_t i;
    CHECK(lxp_gov_emergency_state_init(&state) == LXP_OK);
    CHECK(lxp_gov_emergency_halt(&state, LXP_PAUSE_MARKET, 7U, market,
        trigger, exit_conditions, 1U, 1U, true, true) == LXP_OK);
    CHECK(lxp_gov_emergency_halt(&state, LXP_PAUSE_MODULE, 8U, NULL,
        trigger, exit_conditions, 1U, 2U, true, true) == LXP_OK);
    CHECK(lxp_gov_emergency_halt(&state, LXP_PAUSE_NETWORK, 0U, NULL,
        trigger, exit_conditions, 1U, 3U, true, true) == LXP_OK);
    before = state;
    for (i = 0U; i < sizeof(invalid) / sizeof(invalid[0]); ++i) {
        CHECK(lxp_gov_emergency_resume(&state, invalid[i].scope,
            invalid[i].module_id, invalid[i].market_id, 4U, true, true) ==
            LXP_ERR_NON_CANONICAL);
        CHECK(memcmp(&state, &before, sizeof(state)) == 0);
        CHECK(lxp_gov_emergency_halt(&state, invalid[i].scope,
            invalid[i].module_id, invalid[i].market_id, trigger,
            exit_conditions, 1U, 4U, true, true) == LXP_ERR_NON_CANONICAL);
        CHECK(memcmp(&state, &before, sizeof(state)) == 0);
    }
    CHECK(lxp_gov_emergency_resume(&state, LXP_PAUSE_MARKET, 7U, market,
        4U, false, true) == LXP_ERR_AUTH_SCOPE);
    CHECK(memcmp(&state, &before, sizeof(state)) == 0);
    CHECK(lxp_gov_emergency_resume(&state, LXP_PAUSE_MARKET, 7U, market,
        5U, true, true) == LXP_ERR_SEQUENCE_MISMATCH);
    CHECK(memcmp(&state, &before, sizeof(state)) == 0);
    CHECK(lxp_gov_emergency_resume(&state, LXP_PAUSE_MARKET, 7U, market,
        4U, true, true) == LXP_OK);
    CHECK(!state.pauses[0].active && state.pauses[1].active && state.pauses[2].active);
    CHECK(lxp_gov_emergency_resume(&state, LXP_PAUSE_NETWORK, 0U, NULL,
        5U, true, true) == LXP_OK);
    CHECK(lxp_pause_scope_check(&state, 7U, market, false) == LXP_OK);
    CHECK(lxp_pause_scope_check(&state, 8U, NULL, false) == LXP_ERR_PAUSED_SCOPE);
    return 0;
}

static int audit_emergency_reuses_inactive_slots(void)
{
    lxp_gov_emergency_state state;
    lxp_gov_emergency_state before;
    size_t i;
    CHECK(lxp_gov_emergency_state_init(&state) == LXP_OK);
    for (i = 0U; i < LXP_MAX_GOV_PAUSES; ++i)
        CHECK(lxp_gov_emergency_halt(&state, LXP_PAUSE_MODULE,
            (uint16_t)(i + 1U), NULL, trigger, exit_conditions,
            1U, i + 1U, true, true) == LXP_OK);
    before = state;
    CHECK(lxp_gov_emergency_halt(&state, LXP_PAUSE_MODULE, 65U, NULL,
        trigger, exit_conditions, 1U, 65U, true, true) == LXP_ERR_NON_CANONICAL);
    CHECK(memcmp(&state, &before, sizeof(state)) == 0);
    CHECK(lxp_gov_emergency_resume(&state, LXP_PAUSE_MODULE, 31U, NULL,
        65U, true, true) == LXP_OK);
    CHECK(lxp_gov_emergency_halt(&state, LXP_PAUSE_MODULE, 65U, NULL,
        trigger, exit_conditions, 2U, 66U, true, true) == LXP_OK);
    CHECK(state.pause_count == LXP_MAX_GOV_PAUSES);
    CHECK(state.pauses[30].module_id == 65U && state.pauses[30].active);
    for (i = 1U; i <= LXP_MAX_GOV_PAUSES; ++i)
        CHECK(lxp_pause_scope_check(&state, (uint16_t)i, NULL, false) ==
            (i == 31U ? LXP_OK : LXP_ERR_PAUSED_SCOPE));
    before = state;
    CHECK(lxp_gov_emergency_halt(&state, LXP_PAUSE_MODULE, 65U, NULL,
        trigger, exit_conditions, 2U, 67U, true, true) == LXP_ERR_PAUSED_SCOPE);
    CHECK(memcmp(&state, &before, sizeof(state)) == 0);
    CHECK(state.events[30].pause.module_id == 31U && state.events[30].entered);
    CHECK(state.events[64].pause.module_id == 31U && !state.events[64].entered);
    return 0;
}

static int audit_governance_proposal_admission_bounds(void)
{
    static lxp_param_table table;
    static lxp_param_table before;
    lxp_gov_param_proposal valid;
    lxp_gov_param_proposal invalid;
    uint8_t key[LXP_MAX_PARAMETER_KEY_BYTES];
    uint8_t cohort[LXP_MAX_GOV_COHORT_MEMBERS][32] = {{0U}};
    uint8_t outsider[32] = {33U};
    lxp_byte_span parameter = {key, sizeof(key)};
    uint64_t value;
    uint32_t version;
    size_t i;
    (void)memset(key, 'k', sizeof(key));
    CHECK(lxp_param_table_init(&table) == LXP_OK);
    CHECK(lxp_param_set_bounds(&table, parameter, 7U,
        1U, 100U, 10U, 1U) == LXP_OK);
    (void)memset(&valid, 0, sizeof(valid));
    valid.proposal_id[0] = 1U;
    valid.target_module = 7U;
    (void)memcpy(valid.parameter_key, key, sizeof(key));
    valid.parameter_key_length = sizeof(key);
    valid.proposed_value = 25U;
    valid.activation_epoch = 5U;
    valid.ordered_sequence = 1U;
    for (i = 0U; i < LXP_MAX_GOV_COHORT_MEMBERS; ++i)
        cohort[i][0] = (uint8_t)(i + 1U);
    CHECK(lxp_gov_stage_cohort(&valid, LXP_GOV_ROLLOUT_MARKET,
        (const uint8_t (*)[32])cohort, LXP_MAX_GOV_COHORT_MEMBERS) == LXP_OK);
    before = table;
    invalid = valid;
    invalid.cohort_count = 33U;
    CHECK(lxp_gov_param_propose(&table, &invalid, 2U, 2U, true, true) ==
        LXP_ERR_PARAMETER_BOUNDS);
    CHECK(memcmp(&table, &before, sizeof(table)) == 0);
    invalid.cohort_count = SIZE_MAX;
    CHECK(lxp_gov_param_propose(&table, &invalid, 2U, 2U, true, true) ==
        LXP_ERR_PARAMETER_BOUNDS);
    CHECK(memcmp(&table, &before, sizeof(table)) == 0);
    for (i = 0U; i < 3U; ++i) {
        invalid = valid;
        invalid.parameter_key_length = i == 0U ? 0U :
            i == 1U ? LXP_MAX_PARAMETER_KEY_BYTES + 1U : SIZE_MAX;
        CHECK(lxp_gov_param_propose(&table, &invalid, 2U, 2U, true, true) ==
            LXP_ERR_PARAMETER_BOUNDS);
        CHECK(memcmp(&table, &before, sizeof(table)) == 0);
    }
    for (i = 0U; i < LXP_MAX_GOV_COHORT_MEMBERS; ++i) {
        invalid = valid;
        (void)memset(invalid.cohort[i], 0, 32U);
        CHECK(lxp_gov_param_propose(&table, &invalid, 2U, 2U, true, true) ==
            LXP_ERR_UNSORTED_SEQUENCE);
        CHECK(memcmp(&table, &before, sizeof(table)) == 0);
    }
    invalid = valid;
    (void)memcpy(invalid.cohort[1], invalid.cohort[0], 32U);
    CHECK(lxp_gov_param_propose(&table, &invalid, 2U, 2U, true, true) ==
        LXP_ERR_UNSORTED_SEQUENCE);
    CHECK(memcmp(&table, &before, sizeof(table)) == 0);
    invalid = valid;
    invalid.cohort[1][0] = 34U;
    CHECK(lxp_gov_param_propose(&table, &invalid, 2U, 2U, true, true) ==
        LXP_ERR_UNSORTED_SEQUENCE);
    CHECK(memcmp(&table, &before, sizeof(table)) == 0);
    invalid = valid;
    invalid.rollout_scope = LXP_GOV_ROLLOUT_ALL;
    CHECK(lxp_gov_param_propose(&table, &invalid, 2U, 2U, true, true) ==
        LXP_ERR_PARAMETER_BOUNDS);
    CHECK(memcmp(&table, &before, sizeof(table)) == 0);
    invalid = valid;
    invalid.cohort_count = 0U;
    CHECK(lxp_gov_param_propose(&table, &invalid, 2U, 2U, true, true) ==
        LXP_ERR_PARAMETER_BOUNDS);
    CHECK(memcmp(&table, &before, sizeof(table)) == 0);
    CHECK(lxp_gov_param_propose(&table, &valid, 2U, 2U, true, true) == LXP_OK);
    CHECK(table.proposal_count == 1U && table.proposals[0].cohort_count == 32U);
    CHECK(lxp_gov_activation_apply(&table, 5U, true) == LXP_OK);
    for (i = 0U; i < LXP_MAX_GOV_COHORT_MEMBERS; ++i) {
        CHECK(lxp_gov_param_enact(&table, parameter, 5U, cohort[i],
            &value, &version) == LXP_OK);
        CHECK(value == 25U && version == table.current_version);
    }
    CHECK(lxp_gov_param_enact(&table, parameter, 5U, outsider,
        &value, &version) == LXP_OK);
    CHECK(value == 10U);
    return 0;
}

typedef struct emergency_env {
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_kernel kernel;
    lxp_module_ctx ctx;
    lxp_effect_buffer effects;
    lxp_arena arena;
    uint8_t arena_bytes[65536];
    uint64_t parameters;
    lx_account_registry accounts;
} emergency_env;

static emergency_env env;
static lxp_effect history[512];
static lxp_module_ctx context_before;
static lxp_effect_buffer effects_before;
static uint64_t committed_sequence;
static lxp_log receipts;

static int emergency_env_open(void)
{
    (void)memset(&env, 0, sizeof(env));
    env.parameters = 1U;
    CHECK(lx_account_registry_init(&env.accounts) == LXP_OK);
    CHECK(lxp_state_store_init(&env.state, 0U) == LXP_OK);
    CHECK(lxp_state_store_bind_accounts(&env.state, &env.accounts) == LXP_OK);
    CHECK(lxp_kernel_create(&env.kernel, &env.state, &env.journal,
        &env.parameters, 0U) == LXP_OK);
    CHECK(lxp_kernel_register_module(&env.kernel, lxp_governance_module_iface()) == LXP_OK);
    return 0;
}

static int emergency_context_open(void)
{
    CHECK(lxp_arena_init(&env.arena, env.arena_bytes, sizeof(env.arena_bytes)) == LXP_OK);
    CHECK(lxp_effect_buffer_init(&env.effects) == LXP_OK);
    CHECK(lxp_module_ctx_init(&env.ctx, &env.kernel, LXP_MODULE_GOVERNANCE,
        100U, 1U, committed_sequence + 1U, 100000U, &env.arena, true) == LXP_OK);
    CHECK(lxp_module_ctx_bind_effects(&env.ctx, &env.effects) == LXP_OK);
    return 0;
}

static int emergency_commit(lxp_gov_emergency_operation *operation)
{
    static lxp_receipt receipt;
    uint8_t previous[32];
    uint8_t resulting[32];
    uint8_t activity_id[32];
    lxp_byte_span encoded;
    lxp_gov_emergency_event event;
    uint8_t kind;
    CHECK(emergency_context_open() == 0);
    CHECK(lxp_state_root(&env.kernel, previous) == LXP_OK);
    operation->ordered_sequence = committed_sequence + 1U;
    CHECK(lxp_gov_emergency_execute(&env.ctx, operation, true, true) == LXP_OK);
    CHECK(env.effects.count == 1U);
    CHECK(lxp_gov_emergency_event_decode(env.effects.effects[0].body,
        env.effects.effects[0].body_length, &event, &kind) == LXP_OK);
    CHECK(kind == operation->kind && event.ordered_sequence == operation->ordered_sequence);
    CHECK(lxp_module_ctx_commit(&env.ctx) == LXP_OK);
    CHECK(lxp_state_journal_open(&env.state, env.state.next_sequence,
        &env.journal) == LXP_OK);
    CHECK(lxp_state_journal_commit(&env.journal) == LXP_OK);
    CHECK(lxp_state_root(&env.kernel, resulting) == LXP_OK);
    CHECK(lxp_hash_sha256(env.effects.effects[0].body,
        env.effects.effects[0].body_length, activity_id) == LXP_OK);
    (void)memset(&receipt, 0, sizeof(receipt));
    CHECK(lxp_receipt_build(&receipt, activity_id, operation->ordered_sequence,
        previous, resulting, activity_id, LXP_OK, &env.effects,
        (lxp_u128){0U, 0U}, activity_id, LXP_MODULE_GOVERNANCE, 1U, 1U) == LXP_OK);
    CHECK(lxp_receipt_encode(&receipt, false, &env.arena, &encoded) == LXP_OK);
    CHECK(encoded.length <= UINT32_MAX);
    CHECK(lxp_log_append(&receipts, LXP_LOG_RECEIPT, operation->ordered_sequence,
        encoded.bytes, (uint32_t)encoded.length, NULL) == LXP_OK);
    CHECK(lxp_log_sync(&receipts) == LXP_OK);
    history[committed_sequence] = env.effects.effects[0];
    ++committed_sequence;
    return 0;
}

static int emergency_reopen(const char *directory)
{
    static uint8_t snapshot_bytes[1048576];
    static uint8_t read_bytes[1048576];
    lxp_snapshot_manifest_record manifest;
    lxp_snapshot_manifest_record recovered;
    lxp_byte_span snapshot;
    lxp_byte_span stored;
    lxp_arena arena;
    uint8_t root[32];
    char path[4096];
    int length;
    CHECK(lxp_state_root(&env.kernel, root) == LXP_OK);
    CHECK(lxp_arena_init(&arena, snapshot_bytes, sizeof(snapshot_bytes)) == LXP_OK);
    CHECK(lxp_snapshot_write(&env.kernel, committed_sequence - 1U,
        &arena, &snapshot) == LXP_OK);
    CHECK(lxp_snapshot_manifest(snapshot.bytes, snapshot.length,
        committed_sequence - 1U, root, root, &manifest) == LXP_OK);
    CHECK(lxp_snapshot_store_write(directory, &manifest,
        snapshot.bytes, snapshot.length) == LXP_OK);
    CHECK(lxp_state_store_destroy(&env.state) == LXP_OK);
    lx_account_registry_release(&env.accounts);
    CHECK(emergency_env_open() == 0);
    length = snprintf(path, sizeof(path), "%s/%020llu.lxs", directory,
        (unsigned long long)(committed_sequence - 1U));
    CHECK(length > 0 && (size_t)length < sizeof(path));
    CHECK(lxp_arena_init(&arena, read_bytes, sizeof(read_bytes)) == LXP_OK);
    CHECK(lxp_snapshot_store_read(path, &arena, &recovered, &stored) == LXP_OK);
    CHECK(lxp_snapshot_load(stored.bytes, stored.length, &recovered, &env.kernel) == LXP_OK);
    CHECK(lxp_snapshot_verify_root(&env.kernel, &recovered) == LXP_OK);
    CHECK(unlink(path) == 0);
    CHECK(emergency_context_open() == 0);
    return 0;
}

static lxp_result history_replay(void *context,
                                 const lxp_log_record_header *header,
                                 const uint8_t *body)
{
    static lxp_receipt receipt;
    lxp_gov_emergency_event event;
    uint8_t kind;
    uint64_t *seen = context;
    lxp_result status = lxp_receipt_decode(body, header->body_length, false, &receipt);
    if (status != LXP_OK) return status;
    if (header->record_kind != LXP_LOG_RECEIPT || header->global_sequence != *seen + 1U ||
        receipt.global_sequence != *seen + 1U || receipt.result_code != LXP_OK ||
        receipt.effects.count != 1U || *seen >= committed_sequence ||
        memcmp(&receipt.effects.effects[0], &history[*seen], sizeof(history[0])) != 0)
        return LXP_FATAL_INVARIANT;
    status = lxp_gov_emergency_event_decode(receipt.effects.effects[0].body,
        receipt.effects.effects[0].body_length, &event, &kind);
    if (status != LXP_OK || event.ordered_sequence != *seen + 1U)
        return LXP_FATAL_INVARIANT;
    ++*seen;
    return LXP_OK;
}

static int audit_emergency_committed_history_reopen(void)
{
    lxp_gov_emergency_state state;
    lxp_gov_emergency_state before;
    lxp_gov_emergency_operation operation;
    char directory[] = "/tmp/lx-governance-audit-XXXXXX";
    char path[4096];
    uint64_t seen = 0U;
    uint64_t offset;
    size_t i;
    int length;
    committed_sequence = 0U;
    CHECK(emergency_env_open() == 0);
    CHECK(mkdtemp(directory) != NULL);
    length = snprintf(path, sizeof(path), "%s/history.lxp", directory);
    CHECK(length > 0 && (size_t)length < sizeof(path));
    CHECK(lxp_log_open_or_create(&receipts, path, 1048576U) == LXP_OK);
    (void)memset(&operation, 0, sizeof(operation));
    operation.kind = LXP_GOV_EMERGENCY_HALT;
    operation.scope = LXP_PAUSE_MARKET;
    operation.module_id = 7U;
    operation.market_id = market;
    operation.trigger = trigger;
    operation.exit_conditions = exit_conditions;
    operation.entry_epoch = 1U;
    CHECK(emergency_commit(&operation) == 0);
    operation.kind = LXP_GOV_EMERGENCY_MODULE_ENABLE;
    operation.module_id = 9U;
    operation.enabled = false;
    CHECK(emergency_commit(&operation) == 0);
    for (i = 0U; i < 130U; ++i) {
        operation.kind = LXP_GOV_EMERGENCY_HALT;
        operation.scope = LXP_PAUSE_MODULE;
        operation.module_id = 8U;
        operation.market_id = NULL;
        operation.entry_epoch = i + 2U;
        CHECK(emergency_commit(&operation) == 0);
        operation.kind = LXP_GOV_EMERGENCY_RESUME;
        CHECK(emergency_commit(&operation) == 0);
        CHECK(lxp_gov_emergency_load(&env.ctx, &state) == LXP_OK);
        CHECK(state.pause_count == 2U && state.event_count == 0U);
        CHECK(env.kernel.module_kv_count == 3U);
        CHECK(lxp_pause_scope_check(&state, 7U, market, false) == LXP_ERR_PAUSED_SCOPE);
        CHECK(lxp_pause_scope_check(&state, 8U, NULL, false) == LXP_OK);
        CHECK(lxp_pause_scope_check(&state, 9U, NULL, false) == LXP_ERR_PAUSED_SCOPE);
        if (i == 69U) {
            before = state;
            CHECK(emergency_reopen(directory) == 0);
            CHECK(lxp_gov_emergency_load(&env.ctx, &state) == LXP_OK);
            CHECK(memcmp(&state, &before, sizeof(state)) == 0);
        }
    }
    CHECK(emergency_context_open() == 0);
    CHECK(lxp_gov_emergency_load(&env.ctx, &before) == LXP_OK);
    operation.kind = LXP_GOV_EMERGENCY_RESUME;
    operation.scope = LXP_PAUSE_MARKET;
    operation.module_id = 7U;
    operation.market_id = NULL;
    operation.ordered_sequence = committed_sequence + 1U;
    context_before = env.ctx;
    effects_before = env.effects;
    offset = receipts.write_offset;
    CHECK(lxp_gov_emergency_execute(&env.ctx, &operation, true, true) == LXP_ERR_NON_CANONICAL);
    CHECK(memcmp(&env.ctx, &context_before, sizeof(env.ctx)) == 0);
    CHECK(memcmp(&env.effects, &effects_before, sizeof(env.effects)) == 0);
    CHECK(receipts.write_offset == offset);
    CHECK(lxp_gov_emergency_load(&env.ctx, &state) == LXP_OK);
    CHECK(memcmp(&state, &before, sizeof(state)) == 0);
    operation.market_id = market;
    CHECK(emergency_commit(&operation) == 0);
    operation.kind = LXP_GOV_EMERGENCY_MODULE_ENABLE;
    operation.module_id = 9U;
    operation.enabled = true;
    CHECK(emergency_commit(&operation) == 0);
    CHECK(lxp_gov_emergency_load(&env.ctx, &before) == LXP_OK);
    CHECK(lxp_pause_scope_check(&before, 7U, market, false) == LXP_OK);
    CHECK(lxp_pause_scope_check(&before, 9U, NULL, false) == LXP_OK);
    CHECK(emergency_reopen(directory) == 0);
    CHECK(lxp_gov_emergency_load(&env.ctx, &state) == LXP_OK);
    CHECK(memcmp(&state, &before, sizeof(state)) == 0);
    CHECK(lxp_log_close(&receipts) == LXP_OK);
    CHECK(lxp_log_open(&receipts, path) == LXP_OK);
    CHECK(lxp_log_recover_complete_records(&receipts, history_replay, &seen) == LXP_OK);
    CHECK(seen == committed_sequence && seen == 264U);
    CHECK(lxp_log_close(&receipts) == LXP_OK);
    CHECK(lxp_state_store_destroy(&env.state) == LXP_OK);
    lx_account_registry_release(&env.accounts);
    CHECK(unlink(path) == 0);
    CHECK(rmdir(directory) == 0);
    return 0;
}

int main(void)
{
    CHECK(audit_emergency_resume_validation() == 0);
    CHECK(audit_emergency_reuses_inactive_slots() == 0);
    CHECK(audit_governance_proposal_admission_bounds() == 0);
    CHECK(audit_emergency_committed_history_reopen() == 0);
    return 0;
}
