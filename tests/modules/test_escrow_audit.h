#ifndef TEST_ESCROW_AUDIT_H
#define TEST_ESCROW_AUDIT_H

#include "../../src/modules/escrow/lx_escrow_internal.h"
#include "layerx/lxp_activity.h"
#include "layerx/lxp_snapshot.h"

#include <openssl/evp.h>
#include <stdio.h>

#define ESCROW_CHECK(condition) do { \
    if (!(condition)) { \
        (void)fprintf(stderr, "%s:%d: %s\n", __func__, __LINE__, #condition); \
        return 1; \
    } \
} while (0)

typedef struct escrow_audit_fixture {
    lx_account_registry accounts;
    lx_asset_registry assets;
    lx_asset_record asset;
    lxp_transfer_asset_state asset_state;
    lx_asset_runtime asset_runtime;
    lx_escrow_runtime runtime;
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_kernel kernel;
    lxp_module_ctx ctx;
    lxp_effect_buffer effects;
    lxp_arena arena;
    uint8_t arena_bytes[LXP_MAX_ACTIVITY_BYTES + 262144U];
    lx_account *owner;
    lx_account *beneficiary;
    uint64_t parameters;
} escrow_audit_fixture;

static escrow_audit_fixture escrow_audit;

static int escrow_audit_context(uint64_t timestamp, uint16_t version)
{
    escrow_audit_fixture *f = &escrow_audit;
    ESCROW_CHECK(lxp_arena_init(&f->arena, f->arena_bytes,
                                sizeof(f->arena_bytes)) == LXP_OK);
    ESCROW_CHECK(lxp_effect_buffer_init(&f->effects) == LXP_OK);
    ESCROW_CHECK(lxp_module_ctx_init(&f->ctx, &f->kernel, LXP_MODULE_ESCROW,
        timestamp, f->kernel.epoch, f->state.next_sequence, 2000000U,
        &f->arena, true) == LXP_OK);
    f->ctx.protocol_version = version;
    ESCROW_CHECK(lxp_module_ctx_bind_effects(&f->ctx, &f->effects) == LXP_OK);
    return 0;
}

static int escrow_audit_account(const char *name, uint64_t balance,
                                 lx_account **account)
{
    escrow_audit_fixture *f = &escrow_audit;
    ESCROW_CHECK(lx_asset_account_open(&f->assets, &f->accounts,
        f->asset.asset_id, (const uint8_t *)name, strlen(name),
        f->accounts.count + 1U, LX_ACCOUNT_OPEN_CREDIT, NULL, account) == LXP_OK);
    ESCROW_CHECK(lxp_ledger_bootstrap_balance(*account, f->asset.asset_id,
        (lxp_u128){0U, balance}, 0U) == LXP_OK);
    return 0;
}

static int escrow_audit_bind(void)
{
    escrow_audit_fixture *f = &escrow_audit;
    ESCROW_CHECK(lxp_kernel_register_module(&f->kernel,
        lx_asset_module_iface()) == LXP_OK);
    ESCROW_CHECK(lxp_kernel_set_capabilities(&f->kernel, NULL,
        lxp_kernel_canonical_ledger_apply) == LXP_OK);
    ESCROW_CHECK(lx_asset_transfer_state(&f->asset, &f->asset_state) == LXP_OK);
    f->asset_runtime.accounts = &f->accounts;
    f->asset_runtime.assets = &f->asset;
    f->asset_runtime.asset_count = 1U;
    f->asset_runtime.transfer_assets = &f->asset_state;
    f->asset_runtime.transfer_asset_count = 1U;
    f->asset_runtime.network_id = 7U;
    f->asset_runtime.protocol_version = 3U;
    ESCROW_CHECK(lxp_kernel_bind_module_runtime(&f->kernel,
        LXP_MODULE_ASSET, &f->asset_runtime) == LXP_OK);
    ESCROW_CHECK(lxp_kernel_register_module(&f->kernel,
        lx_escrow_module_iface()) == LXP_OK);
    f->runtime.accounts = &f->accounts;
    f->runtime.assets = &f->assets;
    ESCROW_CHECK(lxp_kernel_bind_module_runtime(&f->kernel,
        LXP_MODULE_ESCROW, &f->runtime) == LXP_OK);
    return 0;
}

static int escrow_audit_init(void)
{
    escrow_audit_fixture *f = &escrow_audit;
    uint8_t bytes[512];
    size_t length;
    (void)memset(f, 0, sizeof(*f));
    f->parameters = 1U;
    f->asset.asset_id[0] = 0x91U;
    f->asset.symbol_length = 3U;
    (void)memcpy(f->asset.symbol, "TST", 4U);
    f->asset.name_length = 5U;
    (void)memcpy(f->asset.name, "Asset", 5U);
    f->asset.issuer_kind = 1U;
    f->asset.issuer_did32[0] = 1U;
    f->asset.custody_kind = LX_ASSET_CUSTODY_NATIVE;
    f->asset.total_units.lo = 10000U;
    ESCROW_CHECK(lx_asset_registry_init(&f->assets, 0U) == LXP_OK);
    ESCROW_CHECK(lx_asset_register(&f->assets, &f->asset, 0U,
        (lxp_u128){0U, 0U}) == LXP_OK);
    ESCROW_CHECK(lx_account_registry_init(&f->accounts) == LXP_OK);
    ESCROW_CHECK(escrow_audit_account("agent:did:key:audit:main", 10000U,
        &f->owner) == 0);
    ESCROW_CHECK(escrow_audit_account("agent:did:key:provider:main", 0U,
        &f->beneficiary) == 0);
    ESCROW_CHECK(lxp_state_store_init(&f->state, 0U) == LXP_OK);
    ESCROW_CHECK(lxp_state_store_bind_accounts(&f->state, &f->accounts) == LXP_OK);
    ESCROW_CHECK(lxp_state_store_require_account_root(&f->state) == LXP_OK);
    ESCROW_CHECK(lxp_kernel_create(&f->kernel, &f->state, &f->journal,
        &f->parameters, 0U) == LXP_OK);
    ESCROW_CHECK(escrow_audit_bind() == 0);
    ESCROW_CHECK(escrow_audit_context(100U, 2U) == 0);
    f->ctx.module_id = LXP_MODULE_ASSET;
    ESCROW_CHECK(lx_asset_record_encode(&f->asset, bytes, sizeof(bytes),
        &length) == LXP_OK);
    ESCROW_CHECK(lxp_ctx_kv_put(&f->ctx, f->asset.asset_id, 32U, bytes,
        length) == LXP_OK);
    ESCROW_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    return 0;
}

static int escrow_audit_destroy(void)
{
    ESCROW_CHECK(lxp_state_store_destroy(&escrow_audit.state) == LXP_OK);
    lx_account_registry_release(&escrow_audit.accounts);
    return 0;
}

static void escrow_audit_record(lx_escrow_record *record, uint8_t id,
                                 const lx_account *custody)
{
    (void)memset(record, 0, sizeof(*record));
    record->escrow_id[0] = id;
    (void)memcpy(record->owner, escrow_audit.owner->id, 32U);
    (void)memcpy(record->beneficiary, escrow_audit.beneficiary->id, 32U);
    (void)memcpy(record->arbiter, escrow_audit.owner->id, 32U);
    (void)memcpy(record->escrow_account, custody->id, 32U);
    (void)memcpy(record->asset_id, escrow_audit.asset.asset_id, 32U);
    record->locked_amount.lo = 40U;
    record->state = LX_ESCROW_STATE_OPEN;
    record->expiry = 1000U;
    record->dispute_window = 2000U;
}

static void escrow_audit_open_payload(const lx_escrow_record *record,
    uint8_t payload[LX_ESCROW_OPEN_PAYLOAD_BYTES])
{
    (void)memset(payload, 0, LX_ESCROW_OPEN_PAYLOAD_BYTES);
    (void)memcpy(payload, record->escrow_id, 32U);
    (void)memcpy(payload + 32U, record->owner, 32U);
    (void)memcpy(payload + 64U, record->escrow_account, 32U);
    (void)memcpy(payload + 96U, record->beneficiary, 32U);
    (void)memcpy(payload + 128U, record->arbiter, 32U);
    (void)memcpy(payload + 160U, record->asset_id, 32U);
    (void)lxp_u128_to_be(record->locked_amount, payload + 192U);
    lx_escrow_write_u64(payload + 208U, record->expiry);
    lx_escrow_write_u64(payload + 216U, record->dispute_window);
}

static lxp_result escrow_audit_dispatch(uint32_t type, const uint8_t *payload,
    size_t length, const uint8_t principal[32], uint64_t timestamp,
    uint16_t version)
{
    escrow_audit_fixture *f = &escrow_audit;
    const lxp_module_registration *registration;
    lxp_activity activity = {0};
    lxp_activity transported;
    lxp_authority_resolved authority = {0};
    lxp_arena wire_arena;
    lxp_byte_span wire;
    static uint8_t wire_bytes[LXP_MAX_ACTIVITY_BYTES];
    uint8_t seed[32] = {0x34U};
    uint8_t signature[64];
    uint8_t public_key[32];
    uint8_t digest[32];
    size_t key_length = sizeof(public_key);
    size_t signature_length = sizeof(signature);
    EVP_PKEY *key;
    EVP_MD_CTX *signer;
    lxp_result result = LXP_FATAL_INVARIANT;
    lxp_result status;
    if (escrow_audit_context(timestamp, version) != 0)
        return LXP_FATAL_INVARIANT;
    key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL, seed, sizeof(seed));
    signer = EVP_MD_CTX_new();
    if (key == NULL || signer == NULL ||
        EVP_PKEY_get_raw_public_key(key, public_key, &key_length) != 1) {
        EVP_MD_CTX_free(signer);
        EVP_PKEY_free(key);
        return LXP_FATAL_INVARIANT;
    }
    activity.protocol_version = version;
    activity.network_id = 7U;
    activity.activity_type = type;
    activity.actor_did = (lxp_byte_span){(const uint8_t *)"did:key:audit", 13U};
    activity.authority = (lxp_byte_span){public_key, sizeof(public_key)};
    activity.signature = (lxp_byte_span){signature, sizeof(signature)};
    activity.payload = (lxp_byte_span){payload, length};
    activity.timestamp_bound.not_after = timestamp + 1000U;
    activity.idempotency_key[0] = (uint8_t)(f->state.next_sequence + 1U);
    status = lxp_hash_payload(payload, length, activity.payload_hash);
    if (status == LXP_OK)
        status = lxp_activity_signing_preimage(&activity, digest);
    if (status == LXP_OK &&
        (EVP_DigestSignInit(signer, NULL, NULL, NULL, key) != 1 ||
         EVP_DigestSign(signer, signature, &signature_length, digest,
                        sizeof(digest)) != 1 || signature_length != 64U))
        status = LXP_FATAL_INVARIANT;
    EVP_MD_CTX_free(signer);
    EVP_PKEY_free(key);
    if (status == LXP_OK)
        status = lxp_arena_init(&wire_arena, wire_bytes, sizeof(wire_bytes));
    if (status == LXP_OK)
        status = lxp_activity_encode(&activity, &wire_arena, &wire);
    if (status == LXP_OK)
        status = lxp_activity_decode(wire.bytes, wire.length, &transported);
    if (status == LXP_OK)
        status = lxp_activity_id(wire.bytes, wire.length, f->ctx.activity_id);
    if (status == LXP_OK)
        status = lxp_kernel_module_for_activity(&f->kernel, type,
            f->kernel.epoch, &registration);
    if (status == LXP_OK)
        status = lxp_state_journal_open(&f->state, f->state.next_sequence,
                                        &f->journal);
    authority.kind = LXP_AUTHORITY_OWNER;
    (void)memcpy(authority.principal, principal, 32U);
    (void)memcpy(authority.verified_key, public_key, 32U);
    if (status == LXP_OK)
        status = lxp_kernel_dispatch(registration, &f->ctx, &transported,
            &authority, &f->effects, &result);
    if (status != LXP_OK || result != LXP_OK) {
        lxp_module_ctx_rollback(&f->ctx);
        if (f->journal.open) (void)lxp_state_journal_rollback(&f->journal);
        return status != LXP_OK ? status : result;
    }
    status = lxp_module_ctx_commit(&f->ctx);
    if (status == LXP_OK) status = lxp_state_journal_commit(&f->journal);
    return status;
}

static int escrow_audit_reopen(void)
{
    escrow_audit_fixture *f = &escrow_audit;
    static uint8_t bytes[1048576];
    lxp_snapshot_manifest_record manifest;
    lxp_arena arena;
    lxp_byte_span snapshot;
    uint8_t root[32];
    uint8_t after[32];
    uint8_t owner_id[32];
    uint8_t beneficiary_id[32];
    size_t length;
    FILE *file;
    (void)memcpy(owner_id, f->owner->id, 32U);
    (void)memcpy(beneficiary_id, f->beneficiary->id, 32U);
    ESCROW_CHECK(lxp_state_journal_open(&f->state, f->state.next_sequence,
        &f->journal) == LXP_OK);
    ESCROW_CHECK(lxp_state_journal_commit(&f->journal) == LXP_OK);
    ESCROW_CHECK(f->state.next_sequence != 0U);
    ESCROW_CHECK(lxp_state_root(&f->kernel, root) == LXP_OK);
    ESCROW_CHECK(lxp_arena_init(&arena, bytes, sizeof(bytes)) == LXP_OK);
    ESCROW_CHECK(lxp_snapshot_write(&f->kernel, f->state.next_sequence - 1U,
        &arena, &snapshot) == LXP_OK);
    ESCROW_CHECK(lxp_snapshot_manifest(snapshot.bytes, snapshot.length,
        f->state.next_sequence - 1U, root, root, &manifest) == LXP_OK);
    file = tmpfile();
    ESCROW_CHECK(file != NULL);
    length = snapshot.length;
    ESCROW_CHECK(fwrite(snapshot.bytes, 1U, length, file) == length);
    ESCROW_CHECK(fflush(file) == 0 && fseek(file, 0L, SEEK_SET) == 0);
    ESCROW_CHECK(lxp_state_store_destroy(&f->state) == LXP_OK);
    lx_account_registry_release(&f->accounts);
    (void)memset(&f->kernel, 0, sizeof(f->kernel));
    (void)memset(&f->journal, 0, sizeof(f->journal));
    ESCROW_CHECK(lx_account_registry_init(&f->accounts) == LXP_OK);
    ESCROW_CHECK(lxp_state_store_init(&f->state, 0U) == LXP_OK);
    ESCROW_CHECK(lxp_state_store_bind_accounts(&f->state, &f->accounts) == LXP_OK);
    ESCROW_CHECK(lxp_state_store_require_account_root(&f->state) == LXP_OK);
    ESCROW_CHECK(lxp_kernel_create(&f->kernel, &f->state, &f->journal,
        &f->parameters, 0U) == LXP_OK);
    ESCROW_CHECK(escrow_audit_bind() == 0);
    (void)memset(bytes, 0, sizeof(bytes));
    ESCROW_CHECK(fread(bytes, 1U, length, file) == length && fclose(file) == 0);
    ESCROW_CHECK(lxp_snapshot_load(bytes, length, &manifest, &f->kernel) == LXP_OK);
    ESCROW_CHECK(lxp_snapshot_verify_root(&f->kernel, &manifest) == LXP_OK);
    ESCROW_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK);
    ESCROW_CHECK(memcmp(root, after, 32U) == 0);
    ESCROW_CHECK(lx_escrow_resolve_account(&f->runtime, owner_id,
        &f->owner) == LXP_OK);
    ESCROW_CHECK(lx_escrow_resolve_account(&f->runtime, beneficiary_id,
        &f->beneficiary) == LXP_OK);
    return 0;
}

static int audit_escrow_dedicated_custody(void)
{
    escrow_audit_fixture *f = &escrow_audit;
    lx_account *custody;
    lx_escrow_record record;
    lx_escrow_record stored;
    uint8_t payload[LX_ESCROW_OPEN_PAYLOAD_BYTES];
    uint8_t before[32];
    uint8_t after[32];
    uint16_t version;
    for (version = 1U; version <= 3U; ++version) {
        const char *name = version == 3U ?
            "agent:did:key:audit:escrow:0100000000000000000000000000000000000000000000000000000000000000" :
            "agent:did:key:audit:escrow:one";
        ESCROW_CHECK(escrow_audit_init() == 0);
        ESCROW_CHECK(escrow_audit_account(name, 0U, &custody) == 0);
        escrow_audit_record(&record, 1U, custody);
        escrow_audit_open_payload(&record, payload);
        ESCROW_CHECK(escrow_audit_dispatch(LX_ESCROW_OPEN, payload,
            sizeof(payload), f->owner->id, 100U, version) == LXP_OK);
        ESCROW_CHECK(custody->balance.lo == 40U && f->owner->balance.lo == 9960U);
        record.escrow_id[0] = 2U;
        escrow_audit_open_payload(&record, payload);
        ESCROW_CHECK(lxp_state_root(&f->kernel, before) == LXP_OK);
        ESCROW_CHECK(escrow_audit_dispatch(LX_ESCROW_OPEN, payload,
            sizeof(payload), f->owner->id, 100U, version) ==
            (version == 3U ? LXP_ERR_ACCOUNT_ID_MISMATCH : LXP_ERR_ESCROW_STATE));
        ESCROW_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK);
        ESCROW_CHECK(memcmp(before, after, 32U) == 0 && f->effects.count == 0U);
        ESCROW_CHECK(custody->balance.lo == 40U && f->owner->balance.lo == 9960U);
        ESCROW_CHECK(lx_escrow_lookup(&f->ctx, record.escrow_id, &stored) ==
            LXP_ERR_ESCROW_STATE);
        ESCROW_CHECK(escrow_audit_destroy() == 0);
    }
    return 0;
}

static int escrow_audit_credit(lx_account *custody, uint64_t amount)
{
    escrow_audit_fixture *f = &escrow_audit;
    lxp_transfer_asset_state asset;
    lxp_transfer_set set = {0};
    lxp_transfer_source_authority source = {0};
    lxp_receipt receipt;
    ESCROW_CHECK(lx_asset_transfer_state(&f->asset, &asset) == LXP_OK);
    set.leg_count = 1U;
    set.legs[0].from = f->owner;
    set.legs[0].to = custody;
    set.legs[0].amount.lo = amount;
    set.legs[0].reason = LXP_REASON_PAYMENT;
    (void)memcpy(set.legs[0].asset_id, f->asset.asset_id, 32U);
    (void)memcpy(source.authorized_from, f->owner->id, 32U);
    source.debit_authority_kind = LXP_AUTH_OWNER;
    set.context.source_authorities = &source;
    set.context.source_authority_count = 1U;
    set.context.assets = &asset;
    set.context.asset_count = 1U;
    set.context.sequence_account = f->owner;
    set.context.actor_sequence = f->owner->next_sequence;
    set.context.debit_authority_kind = LXP_AUTH_OWNER;
    (void)memcpy(set.context.authorized_from, f->owner->id, 32U);
    ESCROW_CHECK(lxp_kernel_apply_transfer_set(&f->kernel, &set, &receipt) == LXP_OK);
    return 0;
}

static int audit_escrow_overfunded_restore(void)
{
    escrow_audit_fixture *f = &escrow_audit;
    lx_account *custody;
    lx_escrow_record record;
    lx_escrow_record stored;
    lx_escrow_release_request recovery = {0};
    lxp_authority_resolved authority = {0};
    lxp_receipt receipt;
    lxp_receipt replay;
    uint8_t payload[LX_ESCROW_OPEN_PAYLOAD_BYTES];
    uint8_t capture[LX_ESCROW_CAPTURE_PAYLOAD_BYTES] = {0};
    uint8_t release[LX_ESCROW_RELEASE_PAYLOAD_BYTES] = {0};
    uint8_t resolve[LX_ESCROW_DISPUTE_RESOLVE_PAYLOAD_BYTES] = {0};
    uint8_t before[32];
    uint8_t after[32];
    bool found;
    unsigned i;
    ESCROW_CHECK(escrow_audit_init() == 0);
    ESCROW_CHECK(escrow_audit_account("agent:did:key:audit:escrow:one", 0U,
        &custody) == 0);
    escrow_audit_record(&record, 1U, custody);
    escrow_audit_open_payload(&record, payload);
    ESCROW_CHECK(escrow_audit_dispatch(LX_ESCROW_OPEN, payload, sizeof(payload),
        f->owner->id, 100U, 2U) == LXP_OK);
    ESCROW_CHECK(escrow_audit_credit(custody, 7U) == 0);
    ESCROW_CHECK(escrow_audit_reopen() == 0);
    ESCROW_CHECK(lx_escrow_resolve_account(&f->runtime, record.escrow_account,
        &custody) == LXP_OK);
    (void)memcpy(capture, record.escrow_id, 32U);
    capture[47] = 10U;
    capture[48] = 20U;
    (void)memcpy(release, record.escrow_id, 32U);
    release[32] = 21U;
    ESCROW_CHECK(lxp_state_root(&f->kernel, before) == LXP_OK);
    for (i = 0U; i < 5U; ++i) {
        uint32_t type = i == 0U ? LX_ESCROW_CAPTURE :
            i == 1U ? LX_ESCROW_PARTIAL_CAPTURE : i == 2U ? LX_ESCROW_RELEASE :
            i == 3U ? LX_ESCROW_TIMEOUT : LX_ESCROW_DISPUTE_OPEN;
        const uint8_t *body = i < 2U ? capture : release;
        size_t length = i < 2U ? sizeof(capture) :
            i == 4U ? LX_ESCROW_DISPUTE_OPEN_PAYLOAD_BYTES : sizeof(release);
        ESCROW_CHECK(escrow_audit_dispatch(type, body, length,
            i < 2U ? f->beneficiary->id : f->owner->id,
            i == 3U ? 1000U : 100U, 2U) == LXP_FATAL_INVARIANT);
        ESCROW_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK);
        ESCROW_CHECK(memcmp(before, after, 32U) == 0 && f->effects.count == 0U);
        ESCROW_CHECK(custody->balance.lo == 47U && f->owner->balance.lo == 9953U &&
            f->beneficiary->balance.lo == 0U);
        ESCROW_CHECK(lx_escrow_receipt_replay(&f->ctx,
            i < 2U ? capture + 48U : release + 32U, &receipt, &found) == LXP_OK && !found);
    }
    /* A committed legacy disputed hold must also refuse resolution. */
    ESCROW_CHECK(escrow_audit_context(100U, 2U) == 0);
    record.state = LX_ESCROW_STATE_DISPUTED;
    ESCROW_CHECK(lx_escrow_state_update(&f->ctx, &record) == LXP_OK);
    ESCROW_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    (void)memcpy(resolve, record.escrow_id, 32U);
    resolve[34] = 0x13U; resolve[35] = 0x88U; resolve[36] = 22U;
    ESCROW_CHECK(lxp_state_root(&f->kernel, before) == LXP_OK);
    ESCROW_CHECK(escrow_audit_dispatch(LX_ESCROW_DISPUTE_RESOLVE, resolve,
        sizeof(resolve), f->owner->id, 100U, 2U) == LXP_FATAL_INVARIANT);
    ESCROW_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK);
    ESCROW_CHECK(memcmp(before, after, 32U) == 0 && custody->balance.lo == 47U);
    ESCROW_CHECK(escrow_audit_context(100U, 2U) == 0);
    record.state = LX_ESCROW_STATE_OPEN;
    ESCROW_CHECK(lx_escrow_state_update(&f->ctx, &record) == LXP_OK);
    ESCROW_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    recovery.escrow_id = record.escrow_id;
    recovery.escrow_account = custody;
    recovery.owner_account = f->owner;
    recovery.asset = &f->asset;
    recovery.authority = &authority;
    recovery.idempotency_key[0] = 23U;
    authority.kind = LXP_AUTHORITY_OWNER;
    (void)memcpy(authority.principal, f->beneficiary->id, 32U);
    ESCROW_CHECK(lx_escrow_recover_excess_execute(&f->ctx, &recovery, &receipt) ==
        LXP_ERR_UNAUTHORIZED_DEBIT);
    ESCROW_CHECK(custody->balance.lo == 47U && f->owner->balance.lo == 9953U);
    (void)memcpy(authority.principal, f->owner->id, 32U);
    ESCROW_CHECK(lx_escrow_recover_excess_execute(&f->ctx, &recovery, &receipt) == LXP_OK);
    ESCROW_CHECK(receipt.amount.lo == 7U && custody->balance.lo == 40U &&
        f->owner->balance.lo == 9960U && f->beneficiary->balance.lo == 0U);
    ESCROW_CHECK(lx_escrow_lookup(&f->ctx, record.escrow_id, &stored) == LXP_OK);
    ESCROW_CHECK(stored.state == LX_ESCROW_STATE_OPEN && stored.locked_amount.lo == 40U);
    ESCROW_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    ESCROW_CHECK(escrow_audit_context(100U, 2U) == 0);
    ESCROW_CHECK(lx_escrow_recover_excess_execute(&f->ctx, &recovery, &replay) == LXP_OK);
    ESCROW_CHECK(memcmp(&receipt, &replay, sizeof(receipt)) == 0 && custody->balance.lo == 40U);
    ESCROW_CHECK(lx_escrow_release_execute(&f->ctx, &recovery, &replay) == LXP_ERR_CONTEXT_MISMATCH);
    ESCROW_CHECK(escrow_audit_dispatch(LX_ESCROW_CAPTURE, capture, sizeof(capture),
        f->beneficiary->id, 100U, 2U) == LXP_OK);
    ESCROW_CHECK(custody->balance.lo == 0U && f->beneficiary->balance.lo == 40U);
    ESCROW_CHECK(escrow_audit_credit(custody, 3U) == 0);
    ESCROW_CHECK(escrow_audit_context(100U, 2U) == 0);
    recovery.idempotency_key[0] = 26U;
    ESCROW_CHECK(lx_escrow_recover_excess_execute(&f->ctx, &recovery, &receipt) == LXP_OK);
    ESCROW_CHECK(receipt.amount.lo == 3U && custody->balance.lo == 0U &&
        f->owner->balance.lo == 9960U && f->beneficiary->balance.lo == 40U);
    ESCROW_CHECK(lx_escrow_lookup(&f->ctx, record.escrow_id, &stored) == LXP_OK &&
        stored.state == LX_ESCROW_STATE_CAPTURED && lxp_u128_is_zero(stored.locked_amount));
    ESCROW_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    ESCROW_CHECK(escrow_audit_destroy() == 0);
    return 0;
}

static int audit_escrow_shared_restored_custody(void)
{
    escrow_audit_fixture *f = &escrow_audit;
    lx_account *custody;
    lx_escrow_record record;
    uint8_t payload[LX_ESCROW_OPEN_PAYLOAD_BYTES];
    uint8_t capture[LX_ESCROW_CAPTURE_PAYLOAD_BYTES] = {0};
    uint8_t before[32];
    uint8_t after[32];
    ESCROW_CHECK(escrow_audit_init() == 0);
    ESCROW_CHECK(escrow_audit_account("agent:did:key:audit:escrow:one", 0U,
        &custody) == 0);
    escrow_audit_record(&record, 1U, custody);
    escrow_audit_open_payload(&record, payload);
    ESCROW_CHECK(escrow_audit_dispatch(LX_ESCROW_OPEN, payload, sizeof(payload),
        f->owner->id, 100U, 2U) == LXP_OK);
    ESCROW_CHECK(escrow_audit_context(100U, 2U) == 0);
    record.escrow_id[0] = 2U;
    ESCROW_CHECK(lx_escrow_state_put(&f->ctx, &record) == LXP_OK);
    ESCROW_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    ESCROW_CHECK(escrow_audit_reopen() == 0);
    ESCROW_CHECK(lxp_state_root(&f->kernel, before) == LXP_OK);
    capture[0] = 1U; capture[48] = 24U;
    ESCROW_CHECK(escrow_audit_dispatch(LX_ESCROW_CAPTURE, capture, sizeof(capture),
        f->beneficiary->id, 100U, 2U) == LXP_ERR_ESCROW_STATE);
    ESCROW_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK);
    ESCROW_CHECK(memcmp(before, after, 32U) == 0 && f->effects.count == 0U);
    ESCROW_CHECK(lx_escrow_resolve_account(&f->runtime, record.escrow_account,
        &custody) == LXP_OK && custody->balance.lo == 40U);
    ESCROW_CHECK(f->owner->balance.lo == 9960U && f->beneficiary->balance.lo == 0U);
    ESCROW_CHECK(escrow_audit_destroy() == 0);
    return 0;
}

static int audit_escrow_post_settlement_rollback(void)
{
    escrow_audit_fixture *f = &escrow_audit;
    lx_account *custody;
    lx_escrow_record record;
    uint8_t payload[LX_ESCROW_OPEN_PAYLOAD_BYTES];
    uint8_t capture[LX_ESCROW_CAPTURE_PAYLOAD_BYTES] = {0};
    uint8_t before[32];
    uint8_t after[32];
    uint64_t sequence;
    ESCROW_CHECK(escrow_audit_init() == 0);
    ESCROW_CHECK(escrow_audit_account("agent:did:key:audit:escrow:one", 0U,
        &custody) == 0);
    escrow_audit_record(&record, 1U, custody);
    (void)memcpy(record.beneficiary, custody->id, 32U);
    escrow_audit_open_payload(&record, payload);
    ESCROW_CHECK(escrow_audit_dispatch(LX_ESCROW_OPEN, payload, sizeof(payload),
        f->owner->id, 100U, 2U) == LXP_OK);
    sequence = custody->next_sequence;
    ESCROW_CHECK(lxp_state_root(&f->kernel, before) == LXP_OK);
    capture[0] = 1U; capture[48] = 25U;
    ESCROW_CHECK(escrow_audit_dispatch(LX_ESCROW_CAPTURE, capture, sizeof(capture),
        custody->id, 100U, 2U) == LXP_FATAL_INVARIANT);
    ESCROW_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK);
    ESCROW_CHECK(memcmp(before, after, 32U) == 0 && f->effects.count == 0U);
    ESCROW_CHECK(custody->balance.lo == 40U && custody->next_sequence == sequence);
    ESCROW_CHECK(lx_escrow_lookup(&f->ctx, record.escrow_id, &record) == LXP_OK);
    ESCROW_CHECK(record.state == LX_ESCROW_STATE_OPEN && record.locked_amount.lo == 40U);
    ESCROW_CHECK(escrow_audit_destroy() == 0);
    return 0;
}

static int audit_escrow_epoch_chunks(void)
{
    escrow_audit_fixture *f = &escrow_audit;
    lx_account *custody;
    lx_escrow_record record;
    lx_escrow_economic_result result;
    uint8_t payload[LX_ESCROW_OPEN_PAYLOAD_BYTES];
    uint8_t key[32];
    uint8_t before[32];
    uint8_t after[32];
    char name[80];
    size_t i;
    const size_t count = 2U * (size_t)LX_ESCROW_SWEEP_CAPACITY + 1U;
    bool found;
    uint64_t next_sequence;
    ESCROW_CHECK(escrow_audit_init() == 0);
    for (i = 0U; i < count; ++i) {
        int length = snprintf(name, sizeof(name), "agent:did:key:audit:escrow:h%zu", i);
        ESCROW_CHECK(length > 0 && (size_t)length < sizeof(name));
        ESCROW_CHECK(escrow_audit_account(name, 0U, &custody) == 0);
        escrow_audit_record(&record, (uint8_t)(i + 1U), custody);
        escrow_audit_open_payload(&record, payload);
        ESCROW_CHECK(escrow_audit_dispatch(LX_ESCROW_OPEN, payload, sizeof(payload),
            f->owner->id, 100U, 2U) == LXP_OK);
    }
    /* Fail late in collection; no earlier customer's refund may escape. */
    ESCROW_CHECK(escrow_audit_credit(custody, 1U) == 0);
    ESCROW_CHECK(lxp_state_root(&f->kernel, before) == LXP_OK);
    next_sequence = f->state.next_sequence;
    ESCROW_CHECK(lxp_arena_init(&f->arena, f->arena_bytes,
        sizeof(f->arena_bytes)) == LXP_OK);
    ESCROW_CHECK(lxp_kernel_epoch_transition(&f->kernel, 1U, 1000U,
        &f->arena) == LXP_FATAL_INVARIANT);
    ESCROW_CHECK(lxp_state_root(&f->kernel, after) == LXP_OK);
    ESCROW_CHECK(memcmp(before, after, 32U) == 0 && f->kernel.epoch == 0U &&
        f->state.next_sequence == next_sequence && !f->journal.open);
    {
        lx_escrow_release_request recovery = {0};
        lxp_authority_resolved authority = {0};
        lxp_receipt receipt;
        ESCROW_CHECK(escrow_audit_context(100U, 2U) == 0);
        recovery.escrow_id = record.escrow_id;
        recovery.escrow_account = custody;
        recovery.owner_account = f->owner;
        recovery.asset = &f->asset;
        recovery.authority = &authority;
        recovery.idempotency_key[0] = 240U;
        authority.kind = LXP_AUTHORITY_OWNER;
        (void)memcpy(authority.principal, f->owner->id, 32U);
        ESCROW_CHECK(lx_escrow_recover_excess_execute(&f->ctx, &recovery,
            &receipt) == LXP_OK);
        ESCROW_CHECK(lxp_module_ctx_commit(&f->ctx) == LXP_OK);
    }
    ESCROW_CHECK(lxp_arena_init(&f->arena, f->arena_bytes,
        sizeof(f->arena_bytes)) == LXP_OK);
    ESCROW_CHECK(lxp_kernel_epoch_transition(&f->kernel, 1U, 1000U,
        &f->arena) == LXP_OK);
    ESCROW_CHECK(f->owner->balance.lo == 10000U && f->kernel.epoch == 1U &&
        f->state.next_sequence == next_sequence + 1U && !f->journal.open);
    ESCROW_CHECK(escrow_audit_context(1000U, 2U) == 0);
    for (i = 0U; i < count; ++i) {
        uint8_t id[32] = {0};
        id[0] = (uint8_t)(i + 1U);
        ESCROW_CHECK(lx_escrow_lookup(&f->ctx, id, &record) == LXP_OK);
        ESCROW_CHECK(record.state == LX_ESCROW_STATE_TIMED_OUT &&
            lxp_u128_is_zero(record.locked_amount));
        ESCROW_CHECK(lx_escrow_resolve_account(&f->runtime, record.escrow_account,
            &custody) == LXP_OK && lxp_u128_is_zero(custody->balance));
        ESCROW_CHECK(lx_escrow_timeout_key(&record, key) == LXP_OK);
        ESCROW_CHECK(lx_escrow_result_lookup(&f->ctx, key, &result, &found) ==
            LXP_OK && found && result.ordinal == 5U && result.amount.lo == 40U);
    }
    ESCROW_CHECK(lxp_kernel_epoch_transition(&f->kernel, 2U, 1001U,
        &f->arena) == LXP_OK && f->owner->balance.lo == 10000U);
    ESCROW_CHECK(escrow_audit_destroy() == 0);
    return 0;
}

static int audit_escrow_findings(void)
{
    ESCROW_CHECK(audit_escrow_dedicated_custody() == 0);
    ESCROW_CHECK(audit_escrow_overfunded_restore() == 0);
    ESCROW_CHECK(audit_escrow_shared_restored_custody() == 0);
    ESCROW_CHECK(audit_escrow_post_settlement_rollback() == 0);
    ESCROW_CHECK(audit_escrow_epoch_chunks() == 0);
    return 0;
}

#undef ESCROW_CHECK
#endif
