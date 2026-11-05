/* Proves: lxp_kernel.h:lxp_kernel_execute_activity -> lxp_storage.h:lxp_log_append/lxp_log_recover -> lxp_receipt.h:lxp_verified_receipt_index_add_height_authority/lxp_verified_receipt_index_lookup_authority -> lxp_ctx_verified_receipt_authority_facts / Programs balance and arbiter consumers.
 * Repairs src/protocol/lxp_kernel.c:lxp_kernel_batch_snapshot_create, which
 * copied an unchecked authority_count, and shares the receipt index bounds
 * check with clone, commit and authority lookup in src/protocol/lxp_receipt.c.
 * The producer is the existing real signed asset SEND harness. Its state and
 * signed activities and canonical receipts survive destruction and reopening
 * of the state store and receipt log. Normal log recovery requires matching
 * activity/receipt pairs (src/storage/lxp_log_recover.c:lxp_log_recover).
 * The index is rebuilt from recovered log records, never
 * from constructed facts or a fallback callback.
 */
#define _POSIX_C_SOURCE 200809L
#include "../codec/lxp_kernel_case.h"
#include "layerx/lxp_storage.h"
#include <unistd.h>

/* Reuse the real account, signer, ledger and kernel setup, without running its
 * unrelated sequence cases. No producer or transport is substituted. */
#define main receipt_h_sequence_case_main
#include "../codec/lxp_test_global_sequence.c"
#undef main

/* The native call consumer uses this private call-state type, as in
 * tests/codec/lxp_test_receipt_authority_v2.c. */
#include "../../src/modules/programs/call.c"

enum { RECEIPT_H_PRODUCERS = 3 };

typedef struct receipt_h_fixture {
    sequence_fixture producer;
    lxp_verified_receipt_index index;
    lx_programs_transfer_runtime programs_runtime;
    lxp_module_ctx consumer;
    lxp_programs_call_activity call;
    uint8_t digests[RECEIPT_H_PRODUCERS][32];
    lxp_log receipt_log;
    lxp_arena log_arena;
    uint8_t log_storage[2U * LXP_MAX_ACTIVITY_BYTES];
    uint64_t receipt_offsets[RECEIPT_H_PRODUCERS];
    size_t receipt_count;
    uint8_t replay_activity_id[32];
    bool replay_activity_pending;
    char directory[96];
    char log_path[128];
} receipt_h_fixture;

static lxp_result receipt_h_publish(void *context, const lxp_kernel *kernel,
                                     const lxp_activity *activity,
                                     const lxp_receipt *receipt)
{
    receipt_h_fixture *f = context;
    lxp_byte_span canonical;
    lxp_result status;
    if (f->receipt_count == RECEIPT_H_PRODUCERS ||
        activity->activity_type != LX_ASSET_SEND ||
        memcmp(kernel->current_state_root, receipt->resulting_state_root, 32U) != 0)
        return LXP_FATAL_INVARIANT;
    status = lxp_arena_reset(&f->log_arena, 0U);
    if (status == LXP_OK)
        status = lxp_activity_encode(activity, &f->log_arena, &canonical);
    if (status == LXP_OK && canonical.length > UINT32_MAX)
        status = LXP_ERR_LENGTH_LIMIT;
    if (status == LXP_OK)
        status = lxp_log_append(&f->receipt_log, LXP_LOG_ACTIVITY,
            receipt->global_sequence, canonical.bytes, (uint32_t)canonical.length,
            NULL);
    if (status == LXP_OK) status = lxp_arena_reset(&f->log_arena, 0U);
    if (status == LXP_OK)
        status = lxp_receipt_encode(receipt, true, &f->log_arena, &canonical);
    if (status == LXP_OK && canonical.length > UINT32_MAX)
        status = LXP_ERR_LENGTH_LIMIT;
    if (status == LXP_OK)
        status = lxp_log_append(&f->receipt_log, LXP_LOG_RECEIPT,
            receipt->global_sequence, canonical.bytes, (uint32_t)canonical.length,
            &f->receipt_offsets[f->receipt_count]);
    if (status == LXP_OK) status = lxp_log_write_boundary(&f->receipt_log);
    if (status == LXP_OK) ++f->receipt_count;
    return status;
}

static int receipt_h_open(receipt_h_fixture *f)
{
    LXP_CASE_CHECK(sequence_fixture_init(&f->producer) == 0);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->producer.kernel,
        programs_module_registration()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_set_epoch(&f->producer.kernel, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&f->producer.kernel,
        programs_module_registration_v2()) == LXP_OK);
    f->producer.execution.epoch = 1U;
    f->programs_runtime.accounts = &f->producer.accounts;
    f->programs_runtime.assets = &f->producer.transfer_asset;
    f->programs_runtime.asset_count = 1U;
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&f->producer.kernel,
        LXP_MODULE_PROGRAMS, &f->programs_runtime) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_root(&f->producer.kernel,
        f->producer.kernel.current_state_root) == LXP_OK);
    strcpy(f->directory, "/tmp/lxp-receipt-h-XXXXXX");
    LXP_CASE_CHECK(mkdtemp(f->directory) != NULL);
    LXP_CASE_CHECK(snprintf(f->log_path, sizeof(f->log_path), "%s/receipts.log",
        f->directory) > 0);
    LXP_CASE_CHECK(lxp_log_open_or_create(&f->receipt_log, f->log_path,
        4U * LXP_MAX_ACTIVITY_BYTES) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&f->log_arena, f->log_storage,
        sizeof(f->log_storage)) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_set_commit_observer(&f->producer.kernel,
        receipt_h_publish, f) == LXP_OK);
    LXP_CASE_CHECK(lxp_verified_receipt_index_init(&f->index) == LXP_OK);
    LXP_CASE_CHECK(lxp_verified_receipt_index_bind_authority_network(
        &f->index, 7U) == LXP_OK);
    return 0;
}

static int receipt_h_index(receipt_h_fixture *f, const lxp_receipt *receipt,
                            lxp_byte_span canonical, uint64_t height)
{
    lxp_arena *arena = &f->producer.arena;
    LXP_CASE_CHECK(lxp_verified_receipt_index_add(&f->index, receipt,
        f->producer.public_key, arena) == LXP_OK);
    LXP_CASE_CHECK(lxp_verified_receipt_index_add_height_authority(&f->index,
        canonical, height, arena) == LXP_OK);
    LXP_CASE_CHECK(lxp_verified_receipt_index_validate(&f->index) == LXP_OK);
    return 0;
}

static int receipt_h_produce(receipt_h_fixture *f)
{
    sequence_fixture *p = &f->producer;
    for (size_t i = 0U; i < RECEIPT_H_PRODUCERS; ++i) {
        lxp_byte_span canonical;
        LXP_CASE_CHECK(lxp_arena_reset(&p->arena, 0U) == LXP_OK);
        LXP_CASE_CHECK(sequence_prepare_send(p, (uint8_t)(i + 1U)) == 0);
        p->execution.global_sequence = p->state.next_sequence;
        LXP_CASE_CHECK(lxp_kernel_execute_activity(&p->kernel, &p->activity,
            &p->execution, &p->receipt) == LXP_OK);
        LXP_CASE_CHECK(p->receipt.result_code == LXP_OK);
        LXP_CASE_CHECK(p->receipt.global_sequence == 7U + i);
        LXP_CASE_CHECK(lxp_receipt_verify(&p->receipt, p->public_key,
            &p->arena) == LXP_OK);
        LXP_CASE_CHECK(lxp_receipt_digest(&p->receipt, &p->arena,
            f->digests[i]) == LXP_OK);
        LXP_CASE_CHECK(lxp_receipt_encode(&p->receipt, true, &p->arena,
            &canonical) == LXP_OK);
        LXP_CASE_CHECK(receipt_h_index(f, &p->receipt, canonical, 1U) == 0);
        LXP_CASE_CHECK(receipt_h_index(f, &p->receipt, canonical, 1U) == 0);
        LXP_CASE_CHECK(f->index.count == i + 1U);
        LXP_CASE_CHECK(f->index.authority_count == i + 1U);
    }
    LXP_CASE_CHECK(p->state.next_sequence == 10U);
    LXP_CASE_CHECK(p->accounts.accounts[0].balance.lo == 7U);
    LXP_CASE_CHECK(p->accounts.accounts[1].balance.lo == 3U);
    return 0;
}

static lxp_result receipt_h_replay(void *context,
                                    const lxp_log_record_header *header,
                                    const uint8_t *body)
{
    receipt_h_fixture *f = context;
    uint8_t digest[32];
    lxp_receipt *receipt = &f->producer.receipt;
    lxp_arena *arena = &f->producer.arena;
    if (f->receipt_count == RECEIPT_H_PRODUCERS ||
        header->global_sequence != 7U + f->receipt_count)
        return LXP_ERR_LOG_CORRUPT;
    if (header->record_kind == LXP_LOG_ACTIVITY) {
        lxp_activity activity;
        if (f->replay_activity_pending) return LXP_ERR_LOG_CORRUPT;
        lxp_result status = lxp_activity_decode(body, header->body_length,
            &activity);
        if (status == LXP_OK)
            status = lxp_activity_check_envelope(&activity, 7U);
        if (status == LXP_OK)
            status = lxp_activity_verify_signature(&activity);
        if (status == LXP_OK)
            status = lxp_activity_id(body, header->body_length,
                f->replay_activity_id);
        if (status == LXP_OK) f->replay_activity_pending = true;
        return status;
    }
    if (header->record_kind != LXP_LOG_RECEIPT || !f->replay_activity_pending)
        return LXP_ERR_LOG_CORRUPT;
    lxp_result status = lxp_receipt_decode(body, header->body_length, true, receipt);
    if (status == LXP_OK &&
        (receipt->global_sequence != header->global_sequence ||
         memcmp(receipt->activity_id, f->replay_activity_id, 32U) != 0))
        status = LXP_ERR_LOG_CORRUPT;
    if (status == LXP_OK) status = lxp_arena_reset(arena, 0U);
    if (status == LXP_OK) status = lxp_receipt_digest(receipt, arena, digest);
    if (status == LXP_OK && memcmp(digest, f->digests[f->receipt_count], 32U) != 0)
        status = LXP_ERR_ROOT_MISMATCH;
    if (status == LXP_OK && receipt_h_index(f, receipt,
        (lxp_byte_span){body, header->body_length}, 1U) != 0)
        status = LXP_FATAL_INVARIANT;
    if (status == LXP_OK && receipt_h_index(f, receipt,
        (lxp_byte_span){body, header->body_length}, 1U) != 0)
        status = LXP_FATAL_INVARIANT;
    if (status == LXP_OK) {
        ++f->receipt_count;
        f->replay_activity_pending = false;
    }
    return status;
}

static int receipt_h_restart(receipt_h_fixture *f)
{
    sequence_fixture *p = &f->producer;
    lxp_state_snapshot *saved = NULL;
    uint8_t before[32], after[32];
    size_t receipt_count = f->receipt_count;
    size_t account_count = p->accounts.count;

    LXP_CASE_CHECK(receipt_count == RECEIPT_H_PRODUCERS);
    /* The real publication observer persists canonical receipts in the log. */
    LXP_CASE_CHECK(lxp_log_close(&f->receipt_log) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_root(&p->kernel, before) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_snapshot_create(&p->state, &saved) == LXP_OK);
    LXP_CASE_CHECK(saved != NULL);
    LXP_CASE_CHECK(lxp_state_store_destroy(&p->state) == LXP_OK);
    lx_account_registry_release(&p->accounts);
    LXP_CASE_CHECK(lxp_state_store_init(&p->state, 0U) == LXP_OK);
    LXP_CASE_CHECK(lx_account_registry_init(&p->accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_bind_accounts(&p->state,
        &p->accounts) == LXP_OK);
    LXP_CASE_CHECK(p->state.next_sequence == 0U);
    LXP_CASE_CHECK(p->accounts.count == 0U);
    LXP_CASE_CHECK(lx_account_registry_reserve(&p->accounts,
        account_count) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_snapshot_restore(saved, &p->state) == LXP_OK);
    lxp_state_snapshot_destroy(saved);
    LXP_CASE_CHECK(lxp_state_root(&p->kernel, after) == LXP_OK);
    LXP_CASE_CHECK(memcmp(before, after, 32U) == 0);
    LXP_CASE_CHECK(p->state.next_sequence == 10U);
    LXP_CASE_CHECK(p->accounts.accounts[0].balance.lo == 7U);
    LXP_CASE_CHECK(p->accounts.accounts[1].balance.lo == 3U);

    /* Destroy both caches and rebuild only from the restored producer bytes. */
    LXP_CASE_CHECK(lxp_verified_receipt_index_init(&f->index) == LXP_OK);
    LXP_CASE_CHECK(f->index.count == 0U && f->index.authority_count == 0U);
    LXP_CASE_CHECK(lxp_verified_receipt_index_bind_authority_network(
        &f->index, 7U) == LXP_OK);
    f->receipt_count = 0U;
    LXP_CASE_CHECK(lxp_log_open(&f->receipt_log, f->log_path) == LXP_OK);
    LXP_CASE_CHECK(lxp_log_recover(&f->receipt_log,
        receipt_h_replay, f) == LXP_OK);
    LXP_CASE_CHECK(!f->replay_activity_pending);
    LXP_CASE_CHECK(lxp_log_resume_sequence(&f->receipt_log) == 10U);
    LXP_CASE_CHECK(f->receipt_count == receipt_count);
    LXP_CASE_CHECK(f->receipt_count == RECEIPT_H_PRODUCERS);
    LXP_CASE_CHECK(f->index.count == RECEIPT_H_PRODUCERS);
    LXP_CASE_CHECK(f->index.authority_count == RECEIPT_H_PRODUCERS);
    /* Repeating the last signed producer request cannot debit the ledger again.
     * src/protocol/lxp_kernel.c:lxp_kernel_execute_activity currently refuses
     * its already consumed actor sequence during admission. */
    p->execution.global_sequence = p->state.next_sequence;
    LXP_CASE_CHECK(lxp_kernel_execute_activity(&p->kernel, &p->activity,
        &p->execution, &p->receipt) == LXP_ERR_SEQUENCE_REUSED);
    LXP_CASE_CHECK(p->state.next_sequence == 10U);
    LXP_CASE_CHECK(f->receipt_count == RECEIPT_H_PRODUCERS);
    LXP_CASE_CHECK(p->accounts.accounts[0].balance.lo == 7U);
    LXP_CASE_CHECK(p->accounts.accounts[1].balance.lo == 3U);
    LXP_CASE_CHECK(lxp_state_root(&p->kernel, after) == LXP_OK);
    LXP_CASE_CHECK(memcmp(before, after, 32U) == 0);
    return 0;
}

static int receipt_h_consumer(receipt_h_fixture *f)
{
    sequence_fixture *p = &f->producer;
    LXP_CASE_CHECK(lxp_arena_reset(&p->arena, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&f->consumer, &p->kernel,
        LXP_MODULE_PROGRAMS, 20U, 1U, p->state.next_sequence, UINT64_MAX,
        &p->arena, false) == LXP_OK);
    f->consumer.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    f->consumer.batch_number = 2U;
    f->consumer.verified_receipts = &f->index;
    memset(&f->call, 0, sizeof(f->call));
    f->call.ctx = &f->consumer;
    f->call.authority = &p->authority;
    f->call.network_id = 7U;
    return 0;
}

static lxp_result receipt_h_begin(receipt_h_fixture *f,
                                  const uint8_t digest[32], uint64_t height)
{
    return layerx_programs_call_arbiter_authority_begin(
        (uint64_t)(uintptr_t)&f->call, read_u64(digest), read_u64(digest + 8U),
        read_u64(digest + 16U), read_u64(digest + 24U), height);
}

static int receipt_h_observe(receipt_h_fixture *f)
{
    sequence_fixture *p = &f->producer;
    lx_programs_balance_view view;
    for (size_t i = 0U; i < RECEIPT_H_PRODUCERS; ++i) {
        lxp_verified_receipt_facts facts;
        lxp_verified_receipt_authority_facts authority;
        LXP_CASE_CHECK(lxp_ctx_verified_receipt_facts(&f->consumer,
            f->digests[i], &facts) == LXP_OK);
        LXP_CASE_CHECK(facts.global_sequence == 7U + i);
        LXP_CASE_CHECK(facts.amount.hi == 0U && facts.amount.lo == 1U);
        LXP_CASE_CHECK(lxp_verified_receipt_index_lookup_authority(&f->index,
            f->digests[i], &authority) == LXP_OK);
        LXP_CASE_CHECK(authority.version == 2U && authority.network_id == 7U);
        LXP_CASE_CHECK(authority.block_height == 1U);
        LXP_CASE_CHECK(authority.protocol_version == LXP_PROTOCOL_VERSION_STATE_COMMITMENT);
        LXP_CASE_CHECK(authority.module_id == LXP_MODULE_ASSET);
        LXP_CASE_CHECK(authority.result_code == LXP_OK);
        LXP_CASE_CHECK(memcmp(authority.receipt_digest, f->digests[i], 32U) == 0);
        LXP_CASE_CHECK(memcmp(authority.resulting_state_root,
            facts.resulting_state_root, 32U) == 0);
        LXP_CASE_CHECK(receipt_h_begin(f, f->digests[i], 1U) == LXP_OK);
        LXP_CASE_CHECK(ARBITER_AUTHORITY_BYTES == 151);
        for (uint32_t j = 0U; j < 151U; ++j) {
            uint8_t expected;
            if (j == 0U) expected = 0U;
            else if (j == 1U) expected = 2U;
            else if (j < 6U) expected = (uint8_t)(authority.network_id >> ((5U - j) * 8U));
            else if (j < 8U) expected = (uint8_t)(authority.protocol_version >> ((7U - j) * 8U));
            else if (j < 10U) expected = (uint8_t)(authority.module_id >> ((9U - j) * 8U));
            else if (j == 10U) expected = authority.operation;
            else if (j < 15U) expected = (uint8_t)((uint32_t)authority.result_code >> ((14U - j) * 8U));
            else if (j < 23U) expected = (uint8_t)(authority.block_height >> ((22U - j) * 8U));
            else if (j < 55U) expected = authority.receipt_digest[j - 23U];
            else if (j < 87U) expected = authority.activity_id[j - 55U];
            else if (j < 119U) expected = authority.previous_state_root[j - 87U];
            else expected = authority.resulting_state_root[j - 119U];
            LXP_CASE_CHECK(layerx_programs_call_arbiter_authority_byte(
                (uint64_t)(uintptr_t)&f->call, j) == (lxp_result)expected);
        }
    }
    LXP_CASE_CHECK(lxp_programs_balance_read(&f->consumer,
        p->accounts.accounts[0].id, p->asset.asset_id,
        f->digests[RECEIPT_H_PRODUCERS - 1U], &view) == LXP_OK);
    LXP_CASE_CHECK(view.balance.hi == 0U && view.balance.lo == 7U);
    LXP_CASE_CHECK(view.observed_sequence == 9U && view.observed_at == 10U);
    LXP_CASE_CHECK(memcmp(view.state_root, p->kernel.current_state_root, 32U) == 0);
    LXP_CASE_CHECK(lxp_programs_balance_read(&f->consumer,
        p->accounts.accounts[1].id, p->asset.asset_id,
        f->digests[RECEIPT_H_PRODUCERS - 1U], &view) == LXP_OK);
    LXP_CASE_CHECK(view.balance.hi == 0U && view.balance.lo == 3U);
    return 0;
}

static int receipt_h_close(receipt_h_fixture *f)
{
    lxp_module_ctx_rollback(&f->consumer);
    LXP_CASE_CHECK(lxp_state_store_destroy(&f->producer.state) == LXP_OK);
    lx_account_registry_release(&f->producer.accounts);
    LXP_CASE_CHECK(lxp_kernel_clear_commit_observer(&f->producer.kernel, f) == LXP_OK);
    LXP_CASE_CHECK(lxp_log_close(&f->receipt_log) == LXP_OK);
    LXP_CASE_CHECK(unlink(f->log_path) == 0);
    LXP_CASE_CHECK(rmdir(f->directory) == 0);
    free(f);
    return 0;
}

static int proof_lxp_receipt_h_happy_path(void)
{
    receipt_h_fixture *f = calloc(1U, sizeof(*f));
    LXP_CASE_CHECK(f != NULL);
    LXP_CASE_CHECK(receipt_h_open(f) == 0);
    LXP_CASE_CHECK(receipt_h_produce(f) == 0);
    LXP_CASE_CHECK(receipt_h_consumer(f) == 0);
    LXP_CASE_CHECK(receipt_h_observe(f) == 0);
    lxp_module_ctx_rollback(&f->consumer);
    LXP_CASE_CHECK(receipt_h_restart(f) == 0);
    LXP_CASE_CHECK(receipt_h_consumer(f) == 0);
    LXP_CASE_CHECK(receipt_h_observe(f) == 0);
    LXP_CASE_CHECK(receipt_h_close(f) == 0);
    return 0;
}

static int proof_lxp_receipt_h_refusal(void)
{
    receipt_h_fixture *f = calloc(1U, sizeof(*f));
    uint8_t unknown[32] = {0xeeU};
    lxp_verified_receipt_authority_facts facts;
    lxp_kernel_batch_snapshot *snapshot = NULL;
    lxp_byte_span canonical;
    LXP_CASE_CHECK(f != NULL);
    LXP_CASE_CHECK(receipt_h_open(f) == 0);
    LXP_CASE_CHECK(receipt_h_produce(f) == 0);
    LXP_CASE_CHECK(receipt_h_restart(f) == 0);
    LXP_CASE_CHECK(receipt_h_consumer(f) == 0);
    LXP_CASE_CHECK(receipt_h_begin(f, f->digests[0], 1U) == LXP_OK);
    LXP_CASE_CHECK(receipt_h_begin(f, unknown, 1U) == LXP_ERR_UNKNOWN_FIELD);
    LXP_CASE_CHECK(layerx_programs_call_arbiter_authority_byte(
        (uint64_t)(uintptr_t)&f->call, 0U) == LXP_ERR_UNKNOWN_FIELD);
    LXP_CASE_CHECK(lxp_ctx_verified_receipt_authority_facts(&f->consumer,
        unknown, &facts) == LXP_ERR_UNKNOWN_FIELD);
    LXP_CASE_CHECK(receipt_h_begin(f, f->digests[0], 2U) == LXP_ERR_ROOT_MISMATCH);
    f->call.network_id = 8U;
    LXP_CASE_CHECK(receipt_h_begin(f, f->digests[0], 1U) == LXP_ERR_WRONG_NETWORK);
    f->call.network_id = 7U;
    LXP_CASE_CHECK(receipt_h_begin(f, f->digests[0], 1U) == LXP_OK);

    lxp_log_record_header header;
    LXP_CASE_CHECK(lxp_log_read(&f->receipt_log,
        f->receipt_offsets[RECEIPT_H_PRODUCERS - 1U], &header,
        f->log_storage, sizeof(f->log_storage)) == LXP_OK);
    LXP_CASE_CHECK(header.record_kind == LXP_LOG_RECEIPT);
    canonical = (lxp_byte_span){f->log_storage, header.body_length};
    LXP_CASE_CHECK(lxp_verified_receipt_index_add_height_authority(&f->index,
        (lxp_byte_span){canonical.bytes, canonical.length - 1U}, 1U,
        &f->producer.arena) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(f->index.authority_count == RECEIPT_H_PRODUCERS);
    LXP_CASE_CHECK(lxp_verified_receipt_index_add_height_authority(&f->index,
        canonical, 2U, &f->producer.arena) == LXP_ERR_ROOT_MISMATCH);
    LXP_CASE_CHECK(f->index.authority_count == RECEIPT_H_PRODUCERS);

    /* Regression: kernel snapshot creation must reject malformed authority
     * bounds before borrowing runtimes or invoking any schedule producer. */
    f->index.authority_count = LXP_VERIFIED_RECEIPT_INDEX_MAX + 1U;
    LXP_CASE_CHECK(lxp_kernel_batch_snapshot_create(&f->producer.kernel,
        &f->producer.identities, &f->index, &f->producer.execution,
        &snapshot) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(snapshot == NULL);
    LXP_CASE_CHECK(lxp_verified_receipt_index_lookup_authority(&f->index,
        f->digests[0], &facts) == LXP_ERR_NON_CANONICAL);
    f->index.authority_count = RECEIPT_H_PRODUCERS;
    f->index.count = LXP_VERIFIED_RECEIPT_INDEX_MAX + 1U;
    LXP_CASE_CHECK(lxp_verified_receipt_index_lookup_authority(&f->index,
        f->digests[0], &facts) == LXP_ERR_NON_CANONICAL);
    f->index.count = RECEIPT_H_PRODUCERS;
    f->index.authority_network_id = 0U;
    LXP_CASE_CHECK(lxp_kernel_batch_snapshot_create(&f->producer.kernel,
        &f->producer.identities, &f->index, &f->producer.execution,
        &snapshot) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(snapshot == NULL);
    LXP_CASE_CHECK(lxp_verified_receipt_index_lookup_authority(&f->index,
        f->digests[0], &facts) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(receipt_h_begin(f, f->digests[0], 1U) == LXP_ERR_NON_CANONICAL);
    f->index.authority_network_id = 7U;
    LXP_CASE_CHECK(receipt_h_observe(f) == 0);
    LXP_CASE_CHECK(receipt_h_close(f) == 0);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        {"proof_lxp_receipt_h_happy_path", proof_lxp_receipt_h_happy_path},
        {"proof_lxp_receipt_h_refusal", proof_lxp_receipt_h_refusal}
    };
    /* lxp_kernel_case_main requires explicit case names. Supply every case
     * when the declared gate invokes this proof without arguments. */
    if (argc == 1) {
        char *all[] = {argv[0], "proof_lxp_receipt_h_happy_path",
                       "proof_lxp_receipt_h_refusal"};
        return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                                    3, all);
    }
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                                argc, argv);
}
