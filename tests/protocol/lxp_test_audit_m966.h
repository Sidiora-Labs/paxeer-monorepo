#ifndef LXP_TEST_AUDIT_M966_H
#define LXP_TEST_AUDIT_M966_H

#include "layerx/lxp_transfer.h"
#include "layerx/lxp_snapshot.h"

/* Compile the committed harness and genesis producer into the existing gate
 * executable; neither needs a subprocess or a new Make target. */
#include "../lxp_test_harness.c"
#include "../../cmd/layerx-genesis/lxp_genesis_builder.c"

#define AUDIT_CHECK(condition) \
    do { \
        if (!(condition)) { \
            fprintf(stderr, "%s:%d: %s\n", __FILE__, __LINE__, #condition); \
            return 1; \
        } \
    } while (0)
#define AUDIT_OK(expression) \
    do { \
        if (LXP_ASSERT_RESULT(LXP_OK, (expression)) != 0) return 1; \
    } while (0)

typedef struct audit_kernel_fixture {
    lxp_kernel kernel;
    lxp_state_store store;
    lxp_state_journal journal;
    lx_account_registry accounts;
    lxp_module_ctx ctx;
    lxp_effect_buffer effects;
    lxp_arena arena;
    _Alignas(max_align_t) uint8_t memory[2097153U];
    uint64_t parameters;
} audit_kernel_fixture;

static int audit_context_open(audit_kernel_fixture *f)
{
    AUDIT_OK(lxp_module_ctx_init(&f->ctx, &f->kernel, LXP_MODULE_PROGRAMS,
        10U, f->kernel.epoch, 1U, 1000U, &f->arena, true));
    AUDIT_OK(lxp_effect_buffer_init(&f->effects));
    AUDIT_OK(lxp_module_ctx_bind_effects(&f->ctx, &f->effects));
    return 0;
}

static int audit_kernel_open(audit_kernel_fixture *f, bool register_programs)
{
    f->parameters = 1U;
    AUDIT_OK(lxp_state_store_init(&f->store, 1U));
    AUDIT_OK(lx_account_registry_init(&f->accounts));
    AUDIT_OK(lxp_state_store_bind_accounts(&f->store, &f->accounts));
    AUDIT_OK(lxp_kernel_create(&f->kernel, &f->store, &f->journal,
        &f->parameters, 1U));
    if (register_programs)
        AUDIT_OK(lxp_kernel_register_module(&f->kernel,
            programs_module_registration_v4()));
    AUDIT_OK(lxp_kernel_set_capabilities(&f->kernel, NULL,
        lxp_kernel_canonical_ledger_apply));
    AUDIT_OK(lxp_arena_init(&f->arena, f->memory + 1U,
        sizeof(f->memory) - 1U));
    return register_programs ? audit_context_open(f) : 0;
}

static int audit_kernel_close(audit_kernel_fixture *f)
{
    lxp_module_ctx_rollback(&f->ctx);
    for (size_t i = 0U; i < f->kernel.blob_count; ++i)
        free(f->kernel.blobs[i].bytes);
    lx_account_registry_release(&f->accounts);
    AUDIT_OK(lxp_state_store_destroy(&f->store));
    free(f);
    return 0;
}

static int audit_arena_base_alignment(void)
{
    audit_kernel_fixture *f = calloc(1U, sizeof(*f));
    static const size_t sizes[] = {
        sizeof(size_t), sizeof(lxp_module_kv_change),
        sizeof(lx_account_registration), sizeof(lxp_module_blob)
    };
    static const size_t alignments[] = {
        _Alignof(size_t), _Alignof(lxp_module_kv_change),
        _Alignof(lx_account_registration), _Alignof(lxp_module_blob)
    };
    void *allocation = NULL;
    lxp_arena exhausted;
    size_t mark;
    size_t end;
    AUDIT_CHECK(f != NULL && audit_kernel_open(f, true) == 0);
    AUDIT_CHECK((uintptr_t)f->arena.buffer % _Alignof(size_t) != 0U);
    mark = lxp_arena_mark(&f->arena);
    for (size_t i = 0U; i < sizeof(sizes) / sizeof(sizes[0]); ++i) {
        size_t previous = lxp_arena_mark(&f->arena);
        size_t padding = (size_t)((0U -
            (uintptr_t)(f->arena.buffer + previous)) & (alignments[i] - 1U));
        AUDIT_OK(lxp_ctx_arena_alloc(&f->ctx, sizes[i], alignments[i],
            &allocation));
        AUDIT_CHECK((uintptr_t)allocation % alignments[i] == 0U);
        AUDIT_CHECK(allocation == f->arena.buffer + previous + padding);
        AUDIT_CHECK(f->arena.offset == previous + padding + sizes[i]);
        (void)memset(allocation, 0xA5, sizes[i]);
    }
    end = lxp_arena_mark(&f->arena);
    AUDIT_OK(lxp_arena_reset(&f->arena, mark));
    for (size_t i = mark; i < end; ++i)
        AUDIT_CHECK(f->arena.buffer[i] == 0U);
    AUDIT_OK(lxp_arena_init(&exhausted, f->memory + 1U,
        _Alignof(size_t) - 1U));
    AUDIT_CHECK(lxp_arena_alloc(&exhausted, sizeof(size_t),
        _Alignof(size_t), &allocation) == LXP_ERR_ARENA_EXHAUSTED);
    AUDIT_CHECK(allocation == NULL && exhausted.offset == 0U);
    exhausted = f->arena;
    exhausted.offset = SIZE_MAX;
    exhausted.capacity = SIZE_MAX;
    AUDIT_CHECK(lxp_arena_alloc(&exhausted, 1U, _Alignof(size_t),
        &allocation) == LXP_ERR_ARENA_EXHAUSTED);
    AUDIT_CHECK(allocation == NULL && exhausted.offset == SIZE_MAX);
    AUDIT_CHECK(lxp_arena_alloc(&f->arena, SIZE_MAX, 1U,
        &allocation) == LXP_ERR_ARENA_EXHAUSTED);
    AUDIT_CHECK(allocation == NULL && f->arena.offset == mark);
    return audit_kernel_close(f);
}

static int audit_registration_refusal_atomic(void)
{
    audit_kernel_fixture *f = calloc(1U, sizeof(*f));
    lxp_module_iface interfaces[LXP_KERNEL_MAX_MODULE_REGISTRATIONS + 1U];
    lxp_module_registration before[LXP_KERNEL_MAX_MODULE_REGISTRATIONS];
    const lxp_module_registration *r = NULL;
    uint8_t root[32], after[32];
    uint8_t payload[32] = {1U}, key[32] = {2U}, signature[64] = {3U};
    static const uint8_t actor[] = "did:key:audit";
    lxp_activity activity = {0}, decoded_activity;
    lxp_authority_resolved authority = {0};
    lxp_byte_span wire;
    lxp_result module_result = LXP_FATAL_INVARIANT;
    AUDIT_CHECK(f != NULL && audit_kernel_open(f, false) == 0);
    for (size_t i = 0U; i < LXP_KERNEL_MAX_MODULE_REGISTRATIONS; ++i) {
        interfaces[i] = *programs_module_registration_v4();
        interfaces[i].abi_version += (uint32_t)i;
        AUDIT_OK(lxp_kernel_set_epoch(&f->kernel, i + 1U));
        AUDIT_OK(lxp_kernel_register_module(&f->kernel, &interfaces[i]));
    }
    (void)memcpy(before, f->kernel.modules, sizeof(before));
    AUDIT_OK(lxp_state_root(&f->kernel, root));
    interfaces[LXP_KERNEL_MAX_MODULE_REGISTRATIONS] =
        interfaces[LXP_KERNEL_MAX_MODULE_REGISTRATIONS - 1U];
    ++interfaces[LXP_KERNEL_MAX_MODULE_REGISTRATIONS].abi_version;
    AUDIT_CHECK(lxp_kernel_register_module(&f->kernel,
        &interfaces[LXP_KERNEL_MAX_MODULE_REGISTRATIONS]) ==
            LXP_ERR_ARENA_EXHAUSTED);
    AUDIT_CHECK(f->kernel.module_count == LXP_KERNEL_MAX_MODULE_REGISTRATIONS);
    AUDIT_CHECK(memcmp(before, f->kernel.modules, sizeof(before)) == 0);
    AUDIT_CHECK(lxp_kernel_register_module(&f->kernel,
        &interfaces[LXP_KERNEL_MAX_MODULE_REGISTRATIONS - 1U]) ==
            LXP_ERR_VERSION_UNSUPPORTED);
    interfaces[LXP_KERNEL_MAX_MODULE_REGISTRATIONS].execute = NULL;
    AUDIT_CHECK(lxp_kernel_register_module(&f->kernel,
        &interfaces[LXP_KERNEL_MAX_MODULE_REGISTRATIONS]) ==
            LXP_ERR_UNKNOWN_MODULE);
    AUDIT_CHECK(memcmp(before, f->kernel.modules, sizeof(before)) == 0);
    AUDIT_OK(lxp_state_root(&f->kernel, after));
    AUDIT_CHECK(memcmp(root, after, 32U) == 0);
    AUDIT_OK(lxp_kernel_module_for_activity(&f->kernel, LX_PROGRAMS_REGISTRY,
        f->kernel.epoch, &r));
    AUDIT_CHECK(r->iface == &interfaces[LXP_KERNEL_MAX_MODULE_REGISTRATIONS - 1U]);
    AUDIT_CHECK(r->abi_version == interfaces[LXP_KERNEL_MAX_MODULE_REGISTRATIONS - 1U].abi_version);
    AUDIT_CHECK(audit_context_open(f) == 0);
    activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    activity.network_id = 7U;
    activity.activity_type = LX_PROGRAMS_REGISTRY;
    activity.actor_did = (lxp_byte_span){actor, sizeof(actor) - 1U};
    activity.authority = (lxp_byte_span){key, sizeof(key)};
    activity.signature = (lxp_byte_span){signature, sizeof(signature)};
    activity.payload = (lxp_byte_span){payload, sizeof(payload)};
    activity.timestamp_bound = (lxp_timestamp_bound){1U, 100U};
    activity.idempotency_key[0] = 1U;
    activity.fee_limit = (lxp_u128){0U, 100U};
    AUDIT_OK(lxp_hash_payload(payload, sizeof(payload), activity.payload_hash));
    AUDIT_OK(lxp_activity_encode(&activity, &f->arena, &wire));
    AUDIT_OK(lxp_activity_decode(wire.bytes, wire.length, &decoded_activity));
    authority.kind = LXP_AUTHORITY_OWNER;
    authority.principal[0] = 4U;
    AUDIT_OK(lxp_kernel_dispatch(r, &f->ctx, &decoded_activity, &authority,
        &f->effects, &module_result));
    AUDIT_CHECK(module_result == LXP_OK && f->effects.count == 1U);
    AUDIT_CHECK(f->effects.effects[0].body_length == sizeof(payload));
    AUDIT_CHECK(memcmp(f->effects.effects[0].body, payload, sizeof(payload)) == 0);
    AUDIT_CHECK(f->ctx.gas_used == 75U);
    AUDIT_OK(lxp_state_root(&f->kernel, after));
    AUDIT_CHECK(memcmp(root, after, 32U) == 0);
    return audit_kernel_close(f);
}

static int audit_transfer_shape_refusals(void)
{
    audit_kernel_fixture *f = calloc(1U, sizeof(*f));
    static const char *const names[] = {
        "agent:did:key:audit:main", "agent:did:key:recipient:main"
    };
    lx_account *accounts[2];
    lx_account before[2];
    lxp_transfer_asset_state asset = {{1U}, true, false};
    lxp_transfer_source_authority source = {{0}, LXP_AUTH_OWNER, false};
    lxp_transfer_set valid = {0}, malformed;
    lxp_receipt receipt = {0}, receipt_before;
    lxp_module_account_snapshot snapshots[LXP_MAX_TRANSFER_SET_LEGS * 2U + 1U];
    uint8_t id[32], root[32], after[32];
    AUDIT_CHECK(f != NULL && audit_kernel_open(f, true) == 0);
    for (size_t i = 0U; i < 2U; ++i) {
        AUDIT_OK(lx_account_id_from_string((const uint8_t *)names[i],
            strlen(names[i]), id));
        AUDIT_OK(lx_account_open(&f->accounts, (const uint8_t *)names[i],
            strlen(names[i]), id, 1U, LX_ACCOUNT_OPEN_GENESIS, NULL,
            &accounts[i]));
    }
    /* Opening can grow the registry; resolve both stable account addresses. */
    for (size_t i = 0U; i < 2U; ++i) {
        AUDIT_OK(lx_account_id_from_string((const uint8_t *)names[i],
            strlen(names[i]), id));
        AUDIT_OK(lxp_ctx_account_find(&f->ctx, id, &accounts[i]));
        AUDIT_OK(lxp_ledger_bootstrap_balance(accounts[i], asset.asset_id,
            (lxp_u128){0U, i == 0U ? 100U : 0U}, 1U));
        before[i] = *accounts[i];
    }
    (void)memcpy(source.authorized_from, accounts[0]->id, 32U);
    valid.leg_count = 1U;
    valid.legs[0].from = accounts[0];
    valid.legs[0].to = accounts[1];
    valid.legs[0].amount = (lxp_u128){0U, 7U};
    valid.legs[0].reason = LXP_REASON_PAYMENT;
    (void)memcpy(valid.legs[0].asset_id, asset.asset_id, 32U);
    valid.context.assets = &asset;
    valid.context.asset_count = 1U;
    valid.context.source_authorities = &source;
    valid.context.source_authority_count = 1U;
    valid.context.debit_authority_kind = LXP_AUTH_OWNER;
    valid.context.batch_timestamp = 10U;
    valid.context.expires_at = 100U;
    valid.context.actor_sequence = 1U;
    valid.context.sequence_account = accounts[0];
    (void)memcpy(valid.context.authorized_from, accounts[0]->id, 32U);
    (void)memcpy(snapshots, f->ctx.transfer_snapshots, sizeof(snapshots));
    receipt_before = receipt;
    AUDIT_OK(lxp_state_root(&f->kernel, root));
    for (size_t i = 0U; i < 8U; ++i) {
        malformed = valid;
        switch (i) {
        case 0U: malformed.leg_count = 0U; break;
        case 1U: malformed.leg_count = LXP_MAX_TRANSFER_SET_LEGS + 1U; break;
        case 2U: malformed.leg_count = SIZE_MAX; break;
        case 3U: malformed.context.source_authorities = NULL; break;
        case 4U: malformed.context.source_authority_count = 0U; break;
        case 5U: malformed.context.source_authority_count = 2U; break;
        case 6U: malformed.context.source_authority_count = LXP_MAX_TRANSFER_SET_LEGS + 1U; break;
        default: malformed.context.source_authority_count = SIZE_MAX; break;
        }
        AUDIT_CHECK(lxp_ctx_emit_monetary_transfer_set(&f->ctx, &malformed,
            &receipt) == LXP_ERR_NON_CANONICAL);
        AUDIT_CHECK(memcmp(&receipt, &receipt_before, sizeof(receipt)) == 0);
        AUDIT_CHECK(f->ctx.transfer_snapshot_count == 0U &&
            !f->ctx.transfer_applied && f->effects.count == 0U);
        AUDIT_CHECK(memcmp(snapshots, f->ctx.transfer_snapshots,
            sizeof(snapshots)) == 0);
        for (size_t side = 0U; side < 2U; ++side)
            AUDIT_CHECK(memcmp(&before[side], accounts[side],
                sizeof(before[side])) == 0);
        AUDIT_OK(lxp_state_root(&f->kernel, after));
        AUDIT_CHECK(memcmp(root, after, 32U) == 0);
    }
    AUDIT_OK(lxp_ctx_emit_monetary_transfer_set(&f->ctx, &valid, &receipt));
    AUDIT_CHECK(accounts[0]->balance.lo == 93U && accounts[1]->balance.lo == 7U);
    AUDIT_CHECK(f->ctx.transfer_applied && f->effects.count == 1U);
    lxp_module_ctx_rollback(&f->ctx);
    for (size_t side = 0U; side < 2U; ++side)
        AUDIT_CHECK(memcmp(&before[side], accounts[side], sizeof(before[side])) == 0);
    return audit_kernel_close(f);
}

static int audit_value(lxp_module_ctx *ctx, uint8_t key, uint8_t expected)
{
    const uint8_t *value = NULL;
    size_t length = 0U;
    AUDIT_OK(lxp_ctx_kv_get(ctx, &key, 1U, &value, &length));
    AUDIT_CHECK(length == 1U && value[0] == expected);
    return 0;
}

static int audit_blob(lxp_module_ctx *ctx, const uint8_t key[32], uint8_t expected)
{
    const uint8_t *bytes = NULL;
    size_t length = 0U;
    AUDIT_OK(lxp_ctx_blob_get(ctx, key, &bytes, &length));
    AUDIT_CHECK(length == 1U && bytes[0] == expected);
    return 0;
}

static int audit_savepoint_nested_before_images(void)
{
    audit_kernel_fixture *f = calloc(1U, sizeof(*f));
    lxp_module_savepoint outer = {0}, inner = {0}, accepted = {0};
    uint8_t keys[] = {'k', 'd', 'r'};
    uint8_t values[] = {'A', 'B', 'C', 'D'};
    uint8_t blob_keys[4][32];
    const uint8_t *bytes = NULL;
    size_t length = 0U, mark, end;
    AUDIT_CHECK(f != NULL && audit_kernel_open(f, true) == 0);
    for (size_t i = 0U; i < 3U; ++i)
        AUDIT_OK(lxp_ctx_kv_put(&f->ctx, &keys[i], 1U, &values[0], 1U));
    for (size_t i = 0U; i < 4U; ++i)
        AUDIT_OK(lxp_hash_sha256(&values[i], 1U, blob_keys[i]));
    for (size_t i = 0U; i < 2U; ++i)
        AUDIT_OK(lxp_ctx_blob_put(&f->ctx, blob_keys[i], &values[i], 1U));
    mark = lxp_arena_mark(&f->arena);
    AUDIT_OK(lxp_module_savepoint_begin(&f->ctx, &outer));
    AUDIT_OK(lxp_ctx_kv_put(&f->ctx, &keys[0], 1U, &values[1], 1U));
    AUDIT_OK(lxp_ctx_kv_del(&f->ctx, &keys[1], 1U));
    AUDIT_OK(lxp_ctx_blob_del(&f->ctx, blob_keys[0]));
    AUDIT_OK(lxp_ctx_blob_put(&f->ctx, blob_keys[2], &values[2], 1U));
    AUDIT_OK(lxp_ctx_emit_event(&f->ctx, 1U, &values[1], 1U));
    AUDIT_OK(lxp_ctx_charge_gas(&f->ctx, 7U));
    AUDIT_OK(lxp_module_savepoint_begin(&f->ctx, &inner));
    AUDIT_OK(lxp_ctx_kv_put(&f->ctx, &keys[0], 1U, &values[2], 1U));
    AUDIT_OK(lxp_ctx_kv_del(&f->ctx, &keys[2], 1U));
    AUDIT_OK(lxp_ctx_blob_del(&f->ctx, blob_keys[1]));
    AUDIT_OK(lxp_ctx_blob_put(&f->ctx, blob_keys[3], &values[3], 1U));
    AUDIT_OK(lxp_ctx_emit_event(&f->ctx, 2U, &values[2], 1U));
    AUDIT_OK(lxp_ctx_charge_gas(&f->ctx, 11U));
    AUDIT_OK(lxp_module_savepoint_discard(&f->ctx, &inner));
    AUDIT_CHECK(!inner.active && outer.active && f->ctx.gas_used == 18U);
    AUDIT_CHECK(audit_value(&f->ctx, keys[0], values[1]) == 0);
    AUDIT_CHECK(audit_value(&f->ctx, keys[2], values[0]) == 0);
    AUDIT_CHECK(lxp_ctx_kv_get(&f->ctx, &keys[1], 1U, &bytes, &length) ==
        LXP_ERR_UNKNOWN_FIELD);
    AUDIT_CHECK(audit_blob(&f->ctx, blob_keys[1], values[1]) == 0);
    AUDIT_CHECK(audit_blob(&f->ctx, blob_keys[2], values[2]) == 0);
    AUDIT_CHECK(lxp_ctx_blob_get(&f->ctx, blob_keys[0], &bytes, &length) ==
        LXP_ERR_UNKNOWN_FIELD);
    AUDIT_CHECK(f->effects.count == 1U && f->ctx.next_effect_ordinal == 1U);
    AUDIT_OK(lxp_module_savepoint_begin(&f->ctx, &accepted));
    AUDIT_OK(lxp_ctx_kv_put(&f->ctx, &keys[0], 1U, &values[3], 1U));
    AUDIT_OK(lxp_ctx_blob_del(&f->ctx, blob_keys[2]));
    AUDIT_OK(lxp_ctx_blob_put(&f->ctx, blob_keys[3], &values[3], 1U));
    AUDIT_OK(lxp_module_savepoint_accept(&f->ctx, &accepted));
    AUDIT_CHECK(!accepted.active && audit_value(&f->ctx, keys[0], values[3]) == 0);
    end = lxp_arena_mark(&f->arena);
    AUDIT_OK(lxp_module_savepoint_discard(&f->ctx, &outer));
    AUDIT_CHECK(!outer.active && f->ctx.gas_used == 18U);
    AUDIT_CHECK(f->arena.offset == mark && f->effects.count == 0U &&
        f->ctx.next_effect_ordinal == 0U);
    for (size_t i = mark; i < end; ++i) AUDIT_CHECK(f->arena.buffer[i] == 0U);
    for (size_t i = 0U; i < 3U; ++i)
        AUDIT_CHECK(audit_value(&f->ctx, keys[i], values[0]) == 0);
    for (size_t i = 0U; i < 2U; ++i)
        AUDIT_CHECK(audit_blob(&f->ctx, blob_keys[i], values[i]) == 0);
    for (size_t i = 2U; i < 4U; ++i)
        AUDIT_CHECK(lxp_ctx_blob_get(&f->ctx, blob_keys[i], &bytes, &length) ==
            LXP_ERR_UNKNOWN_FIELD);
    AUDIT_OK(lxp_module_ctx_commit(&f->ctx));
    AUDIT_CHECK(f->kernel.module_kv_count == 3U && f->kernel.blob_count == 2U);
    AUDIT_CHECK(audit_context_open(f) == 0);
    for (size_t i = 0U; i < 3U; ++i)
        AUDIT_CHECK(audit_value(&f->ctx, keys[i], values[0]) == 0);
    for (size_t i = 0U; i < 2U; ++i)
        AUDIT_CHECK(audit_blob(&f->ctx, blob_keys[i], values[i]) == 0);
    AUDIT_OK(lxp_ctx_blob_del(&f->ctx, blob_keys[0]));
    AUDIT_OK(lxp_module_savepoint_begin(&f->ctx, &outer));
    AUDIT_OK(lxp_ctx_blob_del(&f->ctx, blob_keys[1]));
    AUDIT_OK(lxp_module_savepoint_discard(&f->ctx, &outer));
    AUDIT_CHECK(lxp_ctx_blob_get(&f->ctx, blob_keys[0], &bytes, &length) ==
        LXP_ERR_UNKNOWN_FIELD);
    AUDIT_CHECK(audit_blob(&f->ctx, blob_keys[1], values[1]) == 0);
    AUDIT_OK(lxp_module_ctx_commit(&f->ctx));
    AUDIT_CHECK(f->kernel.blob_count == 1U);
    return audit_kernel_close(f);
}

static int audit_savepoint_capacity_refusal(void)
{
    audit_kernel_fixture *f = calloc(1U, sizeof(*f));
    lxp_module_savepoint savepoint = {0};
    const uint8_t key = 'k', value = 'A';
    uint8_t blob_key[32];
    size_t mark;
    void *allocation = NULL;
    AUDIT_CHECK(f != NULL && audit_kernel_open(f, true) == 0);
    AUDIT_OK(lxp_arena_init(&f->arena, f->memory + 1U, 512U));
    AUDIT_OK(lxp_ctx_arena_alloc(&f->ctx, 1U, 1U, &allocation));
    *(uint8_t *)allocation = 0xA5U;
    mark = lxp_arena_mark(&f->arena);
    AUDIT_OK(lxp_ctx_kv_put(&f->ctx, &key, 1U, &value, 1U));
    AUDIT_OK(lxp_hash_sha256(&value, 1U, blob_key));
    AUDIT_OK(lxp_ctx_blob_put(&f->ctx, blob_key, &value, 1U));
    AUDIT_OK(lxp_ctx_charge_gas(&f->ctx, 5U));
    AUDIT_CHECK(lxp_module_savepoint_begin(&f->ctx, &savepoint) ==
        LXP_ERR_ARENA_EXHAUSTED);
    AUDIT_CHECK(!savepoint.active && f->arena.offset == mark &&
        f->ctx.staged_count == 1U && f->ctx.staged_blob_count == 1U &&
        f->ctx.gas_used == 5U && f->effects.count == 0U);
    AUDIT_CHECK(*(const uint8_t *)allocation == 0xA5U);
    AUDIT_CHECK(audit_value(&f->ctx, key, value) == 0);
    AUDIT_CHECK(audit_blob(&f->ctx, blob_key, value) == 0);
    for (size_t i = mark; i < 512U; ++i)
        AUDIT_CHECK(f->arena.buffer[i] == 0U);
    return audit_kernel_close(f);
}

static int audit_genesis_fresh_merged_bootstrap(void)
{
    static const uint8_t private_key[32] = {7U};
    static const uint8_t asset_id[32] = {0x85U};
    static uint8_t memory[8388609U];
    static lxp_genesis_manifest draft, manifest, parsed, changed;
    lxp_genesis_module_plan plan;
    lx_programs_metering_schedule metering = {0};
    lx_programs_fee_genesis_parameters fees = {0};
    lxp_snapshot_manifest_record snapshot_manifest, wrong_snapshot;
    lxp_byte_span encoded, snapshot;
    lxp_arena arena;
    bool enabled = false;
    (void)memset(&draft, 0, sizeof(draft));
    draft.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    draft.network_id = 42U;
    draft.genesis_timestamp_ms = UINT64_C(1700000000000);
    draft.parameter_count = 1U;
    draft.parameters[0].module_id = LXP_MODULE_GOVERNANCE;
    (void)memcpy(draft.parameters[0].key, "parameter-version", 17U);
    draft.parameters[0].value[31] = 1U;
    metering.version = 1U;
    metering.activation_batch = 1U;
    metering.authority_kind = LX_PROGRAMS_METERING_AUTHORITY_GENESIS;
    for (size_t i = 0U; i < 9U; ++i)
        metering.coefficients[i] = i < 5U ? 1U : (i == 7U ? 64U : 8U);
    fees.schedule = (lx_programs_fee_schedule){1U, 1U, 1U, 2U, 4U, 1U, 1U, 100U};
    (void)memcpy(fees.occupancy_asset_id, asset_id, 32U);
    fees.target_occupancy_byte_batches = 100U;
    fees.response_denominator = 1U;
    fees.maximum_change_numerator = 1U;
    fees.maximum_change_denominator = 10U;
    fees.minimum_fee_units_per_occupancy_byte_batch = 1U;
    fees.maximum_fee_units_per_occupancy_byte_batch = 1000U;
    AUDIT_OK(lxp_arena_init(&arena, memory + 1U, sizeof(memory) - 1U));
    AUDIT_OK(lxp_genesis_build_fresh_empty(&draft, asset_id, &metering, &fees,
        private_key, &arena, &manifest, &snapshot_manifest, &encoded, &snapshot));
    AUDIT_CHECK(manifest.guarantor_count == 0U);
    AUDIT_OK(lxp_genesis_parse(encoded.bytes, encoded.length,
        LXP_GENESIS_INPUT_MANIFEST, &parsed));
    AUDIT_CHECK(memcmp(&parsed, &manifest, sizeof(parsed)) == 0);
    AUDIT_OK(lxp_genesis_verify_signature(&parsed, &arena));
    AUDIT_OK(lxp_genesis_accept(&parsed, NULL, true, &arena, &enabled));
    AUDIT_CHECK(enabled);
    AUDIT_OK(lxp_genesis_module_plan_resolve(&parsed, &plan));
    for (size_t restart = 0U; restart < 2U; ++restart) {
        audit_kernel_fixture *f = calloc(1U, sizeof(*f));
        AUDIT_CHECK(f != NULL && audit_kernel_open(f, false) == 0);
        AUDIT_OK(lxp_genesis_module_plan_register(&plan, &f->kernel));
        AUDIT_OK(lxp_snapshot_load(snapshot.bytes, snapshot.length,
            &snapshot_manifest, &f->kernel));
        if (restart == 0U)
            AUDIT_OK(lxp_genesis_bootstrap_verify(&parsed, NULL, 42U, true,
                &snapshot_manifest, &f->kernel, &arena, &enabled));
        else
            AUDIT_OK(lxp_genesis_initialized_verify(&parsed, NULL, 42U,
                &snapshot_manifest, &f->kernel, &arena, &enabled));
        AUDIT_CHECK(enabled && f->accounts.count ==
            LXP_GENESIS_FRESH_SYSTEM_ACCOUNT_COUNT);
        AUDIT_CHECK(memcmp(f->kernel.current_state_root,
            parsed.genesis_receipt_state_root, 32U) == 0);
        AUDIT_CHECK(lxp_genesis_bootstrap_verify(&parsed, NULL, 43U, true,
            &snapshot_manifest, &f->kernel, &arena, &enabled) ==
                LXP_ERR_ROOT_MISMATCH && !enabled);
        AUDIT_CHECK(lxp_genesis_bootstrap_verify(&parsed, NULL, 42U, false,
            &snapshot_manifest, &f->kernel, &arena, &enabled) ==
                LXP_ERR_ROOT_MISMATCH && !enabled);
        wrong_snapshot = snapshot_manifest;
        wrong_snapshot.receipt_state_root[0] ^= 1U;
        AUDIT_CHECK(lxp_genesis_initialized_verify(&parsed, NULL, 42U,
            &wrong_snapshot, &f->kernel, &arena, &enabled) ==
                LXP_ERR_ROOT_MISMATCH && !enabled);
        changed = parsed;
        changed.signature[0] ^= 1U;
        AUDIT_CHECK(lxp_genesis_initialized_verify(&changed, NULL, 42U,
            &snapshot_manifest, &f->kernel, &arena, &enabled) ==
                LXP_ERR_BAD_SIGNATURE && !enabled);
        AUDIT_CHECK(audit_kernel_close(f) == 0);
    }
    changed = parsed;
    changed.protocol_version = LXP_PROTOCOL_VERSION;
    AUDIT_CHECK(lxp_genesis_encode(&changed, true, &arena, &encoded) ==
        LXP_ERR_NON_CANONICAL);
    return 0;
}

static int audit_read_fixture(const char *path, uint8_t *bytes,
                               size_t capacity, size_t *length)
{
    FILE *file = fopen(path, "rb");
    AUDIT_CHECK(file != NULL);
    *length = fread(bytes, 1U, capacity, file);
    AUDIT_CHECK(*length != 0U && !ferror(file) && fgetc(file) == EOF);
    AUDIT_CHECK(fclose(file) == 0);
    return 0;
}

static int audit_genesis_historical_import_hashes(void)
{
    static const char *const directories[] = {
        "tests/fixtures/public-testnet-genesis",
        "tests/fixtures/public-testnet-genesis-v3"
    };
    /* Pinned by each committed expected.json, not by this implementation. */
    static const char *const hashes[] = {
        "a82eb5201c4246a178aeb5fe44ad102235fa963d58e0149e8c543b8050c2767b",
        "ffa553bb4e1359f15ac06a908c5ee3cd7efba06be6fe4756b7d97e7825c90df6"
    };
    static uint8_t memory[8388608U], bytes[LXP_GENESIS_MAX_ENCODED_BYTES];
    static lxp_genesis_manifest manifest;
    lxp_genesis_module_plan plan;
    lxp_genesis_bootstrap_registration registration, parsed_registration;
    uint8_t registration_wire[LXP_GENESIS_REGISTRATION_BYTES];
    uint8_t digest[32];
    char hex[65], path[256];
    size_t length;
    lxp_arena arena;
    lxp_byte_span encoded, snapshot;
    lxp_snapshot_manifest_record snapshot_manifest;
    bool enabled;
    for (size_t i = 0U; i < sizeof(directories) / sizeof(directories[0]); ++i) {
        audit_kernel_fixture *f = calloc(1U, sizeof(*f));
        int count = snprintf(path, sizeof(path), "%s/genesis.manifest", directories[i]);
        AUDIT_CHECK(f != NULL && count > 0 && (size_t)count < sizeof(path));
        AUDIT_CHECK(audit_read_fixture(path, bytes, sizeof(bytes), &length) == 0);
        AUDIT_OK(lxp_hash_sha256(bytes, length, digest));
        for (size_t j = 0U; j < sizeof(digest); ++j)
            (void)snprintf(hex + 2U * j, 3U, "%02x", (unsigned int)digest[j]);
        AUDIT_CHECK(strcmp(hex, hashes[i]) == 0);
        AUDIT_OK(lxp_genesis_parse(bytes, length, LXP_GENESIS_INPUT_MANIFEST,
            &manifest));
        AUDIT_CHECK(manifest.guarantor_count != 0U);
        AUDIT_OK(lxp_arena_init(&arena, memory, sizeof(memory)));
        AUDIT_OK(lxp_genesis_encode(&manifest, true, &arena, &encoded));
        AUDIT_CHECK(encoded.length == length && memcmp(encoded.bytes, bytes, length) == 0);
        AUDIT_OK(lxp_genesis_verify_signature(&manifest, &arena));
        AUDIT_CHECK(lxp_genesis_accept(&manifest, NULL, true, &arena,
            &enabled) == LXP_ERR_ROOT_MISMATCH && !enabled);
        count = snprintf(path, sizeof(path), "%s/00000000000000000000.lxs", directories[i]);
        AUDIT_CHECK(count > 0 && (size_t)count < sizeof(path));
        AUDIT_OK(lxp_snapshot_store_read(path, &arena, &snapshot_manifest, &snapshot));
        AUDIT_CHECK(audit_kernel_open(f, false) == 0);
        AUDIT_OK(lxp_genesis_module_plan_resolve(&manifest, &plan));
        AUDIT_OK(lxp_genesis_module_plan_register(&plan, &f->kernel));
        AUDIT_OK(lxp_snapshot_load(snapshot.bytes, snapshot.length,
            &snapshot_manifest, &f->kernel));
        (void)memset(&registration, 0, sizeof(registration));
        registration.network_id = manifest.network_id;
        registration.finalised = true;
        (void)memcpy(registration.state_root, manifest.genesis_receipt_state_root, 32U);
        (void)memcpy(registration.settlement_anchor, manifest.genesis_receipt_state_root, 32U);
        AUDIT_OK(lxp_genesis_registration_encode(&registration, registration_wire));
        AUDIT_OK(lxp_genesis_registration_parse(registration_wire,
            sizeof(registration_wire), &parsed_registration));
        AUDIT_OK(lxp_genesis_bootstrap_verify(&manifest, &parsed_registration,
            manifest.network_id, true, &snapshot_manifest, &f->kernel,
            &arena, &enabled));
        AUDIT_CHECK(enabled);
        parsed_registration.finalised = false;
        AUDIT_CHECK(lxp_genesis_initialized_verify(&manifest, &parsed_registration,
            manifest.network_id, &snapshot_manifest, &f->kernel, &arena,
            &enabled) == LXP_ERR_ROOT_MISMATCH && !enabled);
        parsed_registration = registration;
        parsed_registration.settlement_anchor[0] ^= 1U;
        AUDIT_CHECK(lxp_genesis_initialized_verify(&manifest, &parsed_registration,
            manifest.network_id, &snapshot_manifest, &f->kernel, &arena,
            &enabled) == LXP_ERR_ROOT_MISMATCH && !enabled);
        AUDIT_CHECK(audit_kernel_close(f) == 0);
    }
    return 0;
}

static int audit_m966_tests(void)
{
    AUDIT_CHECK(lxp_test_register("audit_arena_base_alignment",
        audit_arena_base_alignment) == 0);
    AUDIT_CHECK(lxp_test_register("audit_genesis_fresh_merged_bootstrap",
        audit_genesis_fresh_merged_bootstrap) == 0);
    AUDIT_CHECK(lxp_test_register("audit_genesis_historical_import_hashes",
        audit_genesis_historical_import_hashes) == 0);
    AUDIT_CHECK(lxp_test_register("audit_registration_refusal_atomic",
        audit_registration_refusal_atomic) == 0);
    AUDIT_CHECK(lxp_test_register("audit_transfer_shape_refusals",
        audit_transfer_shape_refusals) == 0);
    AUDIT_CHECK(lxp_test_register("audit_savepoint_nested_before_images",
        audit_savepoint_nested_before_images) == 0);
    AUDIT_CHECK(lxp_test_register("audit_savepoint_capacity_refusal",
        audit_savepoint_capacity_refusal) == 0);
    return lxp_test_run_all(0);
}

#endif
