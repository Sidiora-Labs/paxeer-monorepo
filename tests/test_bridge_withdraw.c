#define _POSIX_C_SOURCE 200809L

#include "layerx/lxp_bridge.h"
#include "layerx/lxp_kernel.h"
#include "layerx/lxp_receipt.h"
#include "layerx/lxp_snapshot.h"
#include "layerx/lxp_state.h"
#include "layerx/lxp_storage.h"
#include "../src/modules/asset/lx_asset_history.h"

#include <unistd.h>

#define LXP_NATIVE_LEDGER_SUPPORT_ONLY
#include "test_bridge_deposit.c"
#undef LXP_NATIVE_LEDGER_SUPPORT_ONLY

static void native_withdraw_direction(native_ledger_fixture *f)
{
    lx_account *account = f->sender;
    f->sender = f->recipient;
    f->recipient = account;
    (void)memcpy(f->authority.principal, f->sender->id, 32U);
}

typedef struct native_withdraw_consumer {
    native_ledger_fixture *fixture;
    lxp_receipt emitted;
    size_t activities;
    size_t receipts;
} native_withdraw_consumer;

static lxp_result native_withdraw_replay(void *opaque,
    const lxp_log_record_header *header, const uint8_t *body)
{
    native_withdraw_consumer *consumer = opaque;
    native_ledger_fixture *f = consumer->fixture;
    lxp_result status;
    if (header->record_kind == (uint8_t)LXP_LOG_ACTIVITY) {
        if (consumer->activities != consumer->receipts ||
            consumer->activities >= 2U ||
            header->global_sequence != f->state.next_sequence)
            return LXP_FATAL_REPLAY_DIVERGENCE;
        if (consumer->activities == 1U) native_withdraw_direction(f);
        status = native_ledger_execute(f, body, header->body_length,
                                       &consumer->emitted);
        if (status == LXP_OK) ++consumer->activities;
        return status;
    }
    if (header->record_kind == (uint8_t)LXP_LOG_RECEIPT) {
        lxp_receipt persisted;
        uint8_t bytes[LXP_MAX_ACTIVITY_BYTES];
        lxp_arena arena;
        lxp_byte_span encoded;
        if (consumer->activities != consumer->receipts + 1U ||
            header->global_sequence != consumer->emitted.global_sequence)
            return LXP_FATAL_REPLAY_DIVERGENCE;
        status = lxp_receipt_decode(body, header->body_length, true, &persisted);
        if (status == LXP_OK) status = lxp_arena_init(&arena, bytes, sizeof(bytes));
        if (status == LXP_OK) status = lxp_receipt_verify(&persisted, f->public_key, &arena);
        if (status == LXP_OK) status = lxp_arena_reset(&arena, 0U);
        if (status == LXP_OK)
            status = lxp_receipt_encode(&consumer->emitted, true, &arena, &encoded);
        if (status == LXP_OK && (encoded.length != header->body_length ||
            memcmp(encoded.bytes, body, encoded.length) != 0))
            status = LXP_FATAL_REPLAY_DIVERGENCE;
        if (status == LXP_OK) ++consumer->receipts;
        return status;
    }
    return LXP_FATAL_REPLAY_DIVERGENCE;
}

/* Supported native ledger withdrawal: a signed SEND from the asset account
 * into the main account is final when the kernel commits. The Go bank and
 * bridgeOut producers require the separate wiring recorded in the task record. */
static int test_native_ledger_withdrawal_reopen(void)
{
    static native_ledger_fixture f;
    uint8_t canonical[2048], withdrawal[2048];
    uint8_t before[32], committed[32], recovered[32], receipt_root[32];
    size_t length, withdrawal_length = 0U;
    lxp_receipt receipt;
    lxp_log log;
    char directory[] = "/tmp/lxp-native-withdraw-XXXXXX";
    char path[128];
    NATIVE_CHECK(native_ledger_init(&f) == 0);
    NATIVE_CHECK(mkdtemp(directory) != NULL);
    NATIVE_CHECK(lxp_log_segment_create(&log, directory, 0U, 65536U) == LXP_OK);
    for (uint64_t index = 0U; index < 2U; ++index) {
        uint8_t bytes[LXP_MAX_ACTIVITY_BYTES];
        lxp_arena arena;
        lxp_byte_span encoded;
        if (index == 1U) {
            native_withdraw_direction(&f);
            NATIVE_CHECK(f.sender->kind == LX_ACCOUNT_AGENT_ASSET);
            NATIVE_CHECK(f.recipient->kind == LX_ACCOUNT_AGENT_MAIN);
        }
        NATIVE_CHECK(native_ledger_activity(&f, index, index == 0U ? 25U : 10U,
            canonical, sizeof(canonical), &length) == 0);
        NATIVE_CHECK(lxp_state_root(&f.kernel, before) == LXP_OK);
        NATIVE_CHECK(native_ledger_execute(&f, canonical, length, &receipt) == LXP_OK);
        NATIVE_CHECK(receipt.result_code == LXP_OK && receipt.global_sequence == index + 1U);
        NATIVE_CHECK(receipt.fee_charged.hi == 0U && receipt.fee_charged.lo == 7U);
        NATIVE_CHECK(lxp_state_root(&f.kernel, committed) == LXP_OK &&
            memcmp(before, committed, 32U) != 0);
        NATIVE_CHECK(lxp_arena_init(&arena, bytes, sizeof(bytes)) == LXP_OK);
        NATIVE_CHECK(lxp_receipt_verify(&receipt, f.public_key, &arena) == LXP_OK);
        NATIVE_CHECK(lxp_arena_reset(&arena, 0U) == LXP_OK);
        NATIVE_CHECK(lxp_receipt_encode(&receipt, true, &arena, &encoded) == LXP_OK);
        NATIVE_CHECK(encoded.length <= LXP_STATE_MAX_RECEIPT_BYTES);
        NATIVE_CHECK(lxp_log_append(&log, LXP_LOG_ACTIVITY, receipt.global_sequence,
            canonical, (uint32_t)length, NULL) == LXP_OK);
        NATIVE_CHECK(lxp_log_append(&log, LXP_LOG_RECEIPT, receipt.global_sequence,
            encoded.bytes, (uint32_t)encoded.length, NULL) == LXP_OK);
        if (index == 1U) {
            withdrawal_length = length;
            (void)memcpy(withdrawal, canonical, length);
        }
    }
    NATIVE_CHECK(native_ledger_balance_proof(&f, f.sender, 8U) == 0);
    NATIVE_CHECK(native_ledger_balance_proof(&f, f.recipient, 978U) == 0);
    NATIVE_CHECK(native_ledger_balance_proof(&f, f.treasury, 14U) == 0);
    (void)memcpy(receipt_root, f.kernel.current_state_root, 32U);
    NATIVE_CHECK(native_ledger_execute(&f, withdrawal, withdrawal_length,
        &receipt) == LXP_ERR_SEQUENCE_REUSED);
    NATIVE_CHECK(lxp_state_root(&f.kernel, recovered) == LXP_OK &&
        memcmp(committed, recovered, 32U) == 0);
    NATIVE_CHECK(lxp_log_write_boundary(&log) == LXP_OK);
    NATIVE_CHECK(lxp_log_close(&log) == LXP_OK);
    NATIVE_CHECK(native_ledger_destroy(&f) == 0);
    int path_length = snprintf(path, sizeof(path), "%s/%020u.lxp", directory, 0U);
    NATIVE_CHECK(path_length > 0 && (size_t)path_length < sizeof(path));
    for (unsigned restart = 0U; restart < 2U; ++restart) {
        native_withdraw_consumer consumer = {0};
        NATIVE_CHECK(native_ledger_init(&f) == 0);
        consumer.fixture = &f;
        NATIVE_CHECK(lxp_log_open(&log, path) == LXP_OK);
        NATIVE_CHECK(lxp_log_recover(&log, native_withdraw_replay, &consumer) == LXP_OK);
        NATIVE_CHECK(consumer.activities == 2U && consumer.receipts == 2U);
        NATIVE_CHECK(lxp_state_root(&f.kernel, recovered) == LXP_OK &&
            memcmp(committed, recovered, 32U) == 0);
        NATIVE_CHECK(memcmp(receipt_root, f.kernel.current_state_root, 32U) == 0);
        NATIVE_CHECK(native_ledger_balance_proof(&f, f.sender, 8U) == 0);
        NATIVE_CHECK(native_ledger_balance_proof(&f, f.recipient, 978U) == 0);
        NATIVE_CHECK(native_ledger_balance_proof(&f, f.treasury, 14U) == 0);
        NATIVE_CHECK(f.sender->next_sequence == 1U && f.recipient->next_sequence == 1U);
        NATIVE_CHECK(native_ledger_execute(&f, withdrawal, withdrawal_length,
            &receipt) == LXP_ERR_SEQUENCE_REUSED);
        NATIVE_CHECK(lxp_state_root(&f.kernel, recovered) == LXP_OK &&
            memcmp(committed, recovered, 32U) == 0);
        NATIVE_CHECK(native_ledger_balance_proof(&f, f.sender, 8U) == 0);
        NATIVE_CHECK(native_ledger_balance_proof(&f, f.recipient, 978U) == 0);
        NATIVE_CHECK(native_ledger_balance_proof(&f, f.treasury, 14U) == 0);
        NATIVE_CHECK(lxp_log_close(&log) == LXP_OK);
        NATIVE_CHECK(native_ledger_destroy(&f) == 0);
    }
    NATIVE_CHECK(unlink(path) == 0 && rmdir(directory) == 0);
    return 0;
}

/* Historical ABI/codec refusal only. Native bank withdrawal and bridgeOut are
 * node operations; the C ledger SEND below must not stand in for either one. */
static int bridge_withdraw_regression(bool refusal_only)
{
    static native_ledger_fixture f;
    static lxp_module_ctx ctx;
    static uint8_t snapshot_bytes[1048576];
    static lx_withdrawal_store store;
    static lx_withdrawal_store saved_store;
    lxp_arena snapshot_arena;
    lxp_snapshot_manifest_record manifest;
    lxp_byte_span snapshot;
    lxp_receipt receipt, saved_receipt;
    lx_withdrawal_request request = {0}, malformed;
    lx_withdrawal_record decoded;
    lx_asset_transfer_request transfer = {0};
    lx_account *withdrawals, *reserve;
    uint8_t withdrawal_id[32], reserve_id[32], sender_id[32], recipient_id[32], treasury_id[32];
    uint8_t before[32], after[32], key[LX_WITHDRAWAL_STATE_KEY_BYTES];
    uint8_t value[LX_WITHDRAWAL_STATE_VALUE_BYTES], leaf[32], nullifier[32];
    uint8_t canonical[2048];
    size_t canonical_length;
    const uint8_t *stored;
    size_t stored_length;
    const char *withdrawal_name = "system:paxeer-withdrawals";
    const char *reserve_name = "system:paxeer-reserve";
    NATIVE_CHECK(native_ledger_init(&f) == 0);
    NATIVE_CHECK(lx_account_id_from_string((const uint8_t *)withdrawal_name,
        strlen(withdrawal_name), withdrawal_id) == LXP_OK);
    NATIVE_CHECK(lx_account_id_from_string((const uint8_t *)reserve_name,
        strlen(reserve_name), reserve_id) == LXP_OK);
    NATIVE_CHECK(lx_account_open(&f.accounts, (const uint8_t *)withdrawal_name,
        strlen(withdrawal_name), withdrawal_id, 1U, LX_ACCOUNT_OPEN_GENESIS, NULL, &withdrawals) == LXP_OK);
    NATIVE_CHECK(lx_account_open(&f.accounts, (const uint8_t *)reserve_name,
        strlen(reserve_name), reserve_id, 1U, LX_ACCOUNT_OPEN_GENESIS, NULL, &reserve) == LXP_OK);
    NATIVE_CHECK(lxp_ledger_bootstrap_balance(withdrawals, f.asset.asset_id, (lxp_u128){0U, 0U}, 0U) == LXP_OK);
    NATIVE_CHECK(lxp_ledger_bootstrap_balance(reserve, f.asset.asset_id, (lxp_u128){0U, 0U}, 0U) == LXP_OK);
    (void)memcpy(sender_id, f.sender->id, 32U);
    (void)memcpy(recipient_id, f.recipient->id, 32U);
    (void)memcpy(treasury_id, f.treasury->id, 32U);
    NATIVE_CHECK(native_ledger_activity(&f, 0U, 40U, canonical,
        sizeof(canonical), &canonical_length) == 0);
    NATIVE_CHECK(native_ledger_execute(&f, canonical, canonical_length, &receipt) == LXP_OK);
    NATIVE_CHECK(receipt.result_code == LXP_OK);
    NATIVE_CHECK(native_ledger_balance_proof(&f, f.sender, 953U) == 0);
    NATIVE_CHECK(native_ledger_balance_proof(&f, f.recipient, 40U) == 0);
    request.network_id = 125U;
    request.withdrawal_id[0] = 2U;
    (void)memcpy(request.account_id, sender_id, 32U);
    (void)memcpy(request.asset_id, f.asset.asset_id, 32U);
    request.amount.lo = 40U;
    request.payout_recipient[31] = 0xaaU;
    request.checkpoint_id[0] = 3U;
    NATIVE_CHECK(lxp_withdrawal_leaf(&request, leaf) == LXP_OK);
    NATIVE_CHECK(lxp_withdrawal_nullifier(&request, nullifier) == LXP_OK);
    NATIVE_CHECK(lx_asset_historical_withdrawal_encode_v2(&request, key, value) == LXP_OK);
    NATIVE_CHECK(lx_asset_historical_withdrawal_decode_v2(key, sizeof(key), value,
        sizeof(value), &decoded) == LXP_OK);
    NATIVE_CHECK(memcmp(decoded.nullifier, nullifier, 32U) == 0 && !decoded.settled);
    for (size_t length = 0U; length < sizeof(value); ++length)
        NATIVE_CHECK(lx_asset_historical_withdrawal_decode_v2(key, sizeof(key), value,
            length, &decoded) == LXP_ERR_NON_CANONICAL);
    value[1] ^= 1U;
    NATIVE_CHECK(lx_asset_historical_withdrawal_decode_v2(key, sizeof(key), value,
        sizeof(value), &decoded) == LXP_ERR_NON_CANONICAL);
    value[1] ^= 1U;

    /* Archive bytes are produced by the explicit historical codec. They are
     * readable after recovery but can never authorize a second payout. */
    NATIVE_CHECK(lxp_arena_reset(&f.arena, 0U) == LXP_OK);
    NATIVE_CHECK(lxp_module_ctx_init(&ctx, &f.kernel, LXP_MODULE_ASSET, 10U,
        0U, f.state.next_sequence, 1000U, &f.arena, true) == LXP_OK);
    NATIVE_CHECK(lxp_state_journal_open(&f.state, f.state.next_sequence, &f.journal) == LXP_OK);
    NATIVE_CHECK(lxp_ctx_kv_put(&ctx, key, sizeof(key), value, sizeof(value)) == LXP_OK);
    NATIVE_CHECK(lxp_state_journal_commit(&f.journal) == LXP_OK);
    NATIVE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
    for (unsigned restart = 0U; restart < 2U; ++restart) {
        const lxp_module_registration *registration;
        NATIVE_CHECK(lxp_arena_reset(&f.arena, 0U) == LXP_OK);
        NATIVE_CHECK(lxp_module_ctx_init(&ctx, &f.kernel, LXP_MODULE_ASSET,
            10U, 0U, f.state.next_sequence, 1000U, &f.arena, true) == LXP_OK);
        NATIVE_CHECK(lxp_kernel_module_for_activity(&f.kernel, LX_ASSET_WITHDRAW,
            0U, &registration) == LXP_ERR_UNKNOWN_ACTIVITY);
        NATIVE_CHECK(lxp_state_root(&f.kernel, before) == LXP_OK);
        transfer.from = f.sender;
        transfer.to = withdrawals;
        transfer.asset = &f.asset;
        transfer.amount = request.amount;
        for (unsigned mode = 0U; mode < 4U; ++mode) {
            (void)memset(&store, 0, sizeof(store));
            if (mode != 0U) {
                store.count = mode == 3U ? LX_DEPOSIT_NULLIFIER_CAPACITY : 1U;
                store.records[0].request = request;
                (void)memcpy(store.records[0].nullifier, nullifier, 32U);
                store.records[0].settled = mode == 2U;
            }
            saved_store = store;
            (void)memset(&receipt, 0x5a, sizeof(receipt));
            saved_receipt = receipt;
            NATIVE_CHECK(lxp_bridge_withdraw_request(&ctx, &transfer, &request,
                &store, &receipt) == LXP_ERR_UNKNOWN_ACTIVITY);
            NATIVE_CHECK(lx_asset_withdraw_settle(&ctx, withdrawals, reserve,
                &f.asset, NULL, nullifier, &store, (lxp_transfer_context){0},
                &receipt) == LXP_ERR_UNKNOWN_ACTIVITY);
            malformed = request;
            (void)memset(malformed.payout_recipient, 0, 32U);
            NATIVE_CHECK(lxp_withdrawal_nullifier(&malformed, leaf) == LXP_ERR_NON_CANONICAL);
            NATIVE_CHECK(lxp_withdrawal_leaf(&malformed, leaf) == LXP_ERR_NON_CANONICAL);
            NATIVE_CHECK(lxp_bridge_withdraw_request(&ctx, &transfer, &malformed,
                &store, &receipt) == LXP_ERR_GRANT_SCOPE_VIOLATION);
            NATIVE_CHECK(memcmp(&receipt, &saved_receipt, sizeof(receipt)) == 0);
            NATIVE_CHECK(memcmp(&store, &saved_store, sizeof(store)) == 0);
            NATIVE_CHECK(!ctx.transfer_applied && ctx.staged_count == 0U);
            NATIVE_CHECK(lxp_state_root(&f.kernel, after) == LXP_OK && memcmp(before, after, 32U) == 0);
            NATIVE_CHECK(f.sender->balance.lo == 953U && f.recipient->balance.lo == 40U);
            NATIVE_CHECK(lxp_u128_is_zero(withdrawals->balance) && lxp_u128_is_zero(reserve->balance));
        }
        NATIVE_CHECK(lxp_ctx_kv_get(&ctx, key, sizeof(key), &stored, &stored_length) == LXP_OK);
        NATIVE_CHECK(stored_length == sizeof(value) && memcmp(stored, value, sizeof(value)) == 0);
        NATIVE_CHECK(lx_asset_historical_withdrawal_decode_v2(key, sizeof(key), stored,
            stored_length, &decoded) == LXP_OK && memcmp(decoded.nullifier, nullifier, 32U) == 0);
        NATIVE_CHECK(lxp_arena_init(&snapshot_arena, snapshot_bytes, sizeof(snapshot_bytes)) == LXP_OK);
        NATIVE_CHECK(lxp_snapshot_write(&f.kernel, f.state.next_sequence - 1U,
            &snapshot_arena, &snapshot) == LXP_OK);
        NATIVE_CHECK(lxp_snapshot_manifest_build(snapshot.bytes, snapshot.length,
            f.state.next_sequence - 1U, before, f.kernel.current_state_root, &manifest) == LXP_OK);
        NATIVE_CHECK(native_ledger_destroy(&f) == 0);
        (void)memset(&store, 0, sizeof(store));
        NATIVE_CHECK(lx_account_registry_init(&f.accounts) == LXP_OK);
        NATIVE_CHECK(lxp_state_store_init(&f.state, 1U) == LXP_OK);
        NATIVE_CHECK(lxp_state_store_bind_accounts(&f.state, &f.accounts) == LXP_OK);
        NATIVE_CHECK(lxp_state_store_require_account_root(&f.state) == LXP_OK);
        NATIVE_CHECK(lxp_kernel_create(&f.kernel, &f.state, &f.journal, &f.parameters, 0U) == LXP_OK);
        NATIVE_CHECK(native_ledger_bind(&f) == 0);
        NATIVE_CHECK(lxp_snapshot_load(snapshot.bytes, snapshot.length, &manifest, &f.kernel) == LXP_OK);
        NATIVE_CHECK(lxp_snapshot_verify_root(&f.kernel, &manifest) == LXP_OK);
        uint8_t *ids[] = {sender_id, recipient_id, treasury_id, withdrawal_id, reserve_id};
        lx_account **accounts[] = {&f.sender, &f.recipient, &f.treasury, &withdrawals, &reserve};
        for (size_t i = 0U; i < 5U; ++i) {
            size_t slot;
            NATIVE_CHECK(lx_account_registry_index_lookup(&f.accounts, ids[i], &slot) == LXP_OK);
            *accounts[i] = &f.accounts.accounts[slot];
        }
        NATIVE_CHECK(native_ledger_balance_proof(&f, f.sender, 953U) == 0);
        NATIVE_CHECK(native_ledger_balance_proof(&f, f.recipient, 40U) == 0);
        NATIVE_CHECK(native_ledger_balance_proof(&f, f.treasury, 7U) == 0);
        NATIVE_CHECK(lxp_state_root(&f.kernel, after) == LXP_OK && memcmp(before, after, 32U) == 0);
    }
    if (refusal_only) {
        NATIVE_CHECK(lxp_bridge_withdraw_request(NULL, &transfer, &request,
            &store, &receipt) == LXP_ERR_GRANT_SCOPE_VIOLATION);
    }
    NATIVE_CHECK(native_ledger_destroy(&f) == 0);
    return 0;
}

#ifndef LXP_BRIDGE_WITHDRAW_PROOF
#include "lxp_test_harness.c"

static int test_historical_withdrawal_recovery(void)
{
    return bridge_withdraw_regression(false);
}

static int test_historical_withdrawal_malformed_recipient(void)
{
    return bridge_withdraw_regression(true);
}

int main(int argc, char **argv)
{
    NATIVE_CHECK(lxp_test_register("bridge.native-ledger-withdrawal-reopen",
        test_native_ledger_withdrawal_reopen) == 0);
    NATIVE_CHECK(lxp_test_register("bridge.historical-withdrawal-recovery",
        test_historical_withdrawal_recovery) == 0);
    NATIVE_CHECK(lxp_test_register("bridge.historical-withdrawal-malformed-recipient",
        test_historical_withdrawal_malformed_recipient) == 0);
    return lxp_test_main(argc, argv);
}
#endif
