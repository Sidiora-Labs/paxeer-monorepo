#include "lxp_kernel_case.h"

#include "layerx/lx_asset.h"
#include "layerx/lx_service.h"
#include "layerx/lx_stream.h"
#include "layerx/lxp_genesis.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_snapshot.h"
#include "layerx/programs.h"

#include <openssl/evp.h>
#include <stdlib.h>

lxp_result lxp_snapshot_write(const lxp_kernel *kernel,
    uint64_t global_sequence, lxp_arena *arena, lxp_byte_span *snapshot);
lxp_result lxp_snapshot_load(const uint8_t *snapshot, size_t snapshot_length,
    const lxp_snapshot_manifest_record *manifest, lxp_kernel *kernel);

enum { GOLDEN_ACTIVITIES = 24, GOLDEN_ACTORS = 8 };

static const uint8_t golden_seed[32] = {0x61U};
static const uint8_t golden_digest[32] = {
    0x8cU, 0xccU, 0x86U, 0x2cU, 0x70U, 0x51U, 0x53U, 0x3eU,
    0x04U, 0xd1U, 0xabU, 0x1dU, 0x6eU, 0xe3U, 0x4dU, 0xf1U,
    0xa4U, 0x0fU, 0xaeU, 0xe2U, 0xbcU, 0xa5U, 0x7bU, 0x39U,
    0xbfU, 0xd3U, 0x4aU, 0xb4U, 0x29U, 0x59U, 0x5dU, 0x04U
};

typedef struct golden_fixture {
    lxp_genesis_manifest genesis;
    lxp_kernel kernel;
    lxp_state_store state;
    lxp_state_journal journal;
    lx_account_registry accounts;
    lxp_identity_store identities;
    lx_asset_record records[16];
    lx_asset_runtime asset_runtime;
    lx_programs_transfer_runtime runtime;
    lxp_transfer_asset_state transfer_asset;
    lxp_fee_params fees;
    uint32_t parameter_version;
    lxp_arena arena;
    uint8_t storage[8388608U];
    uint8_t public_key[32];
    uint8_t asset_id[32];
    uint8_t principals[GOLDEN_ACTORS][32];
    char dids[GOLDEN_ACTORS][32];
    lxp_receipt last_receipt;
} golden_fixture;

static int golden_sign(const uint8_t digest[32], uint8_t signature[64],
                       uint8_t public_key[32])
{
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL,
        golden_seed, sizeof(golden_seed));
    EVP_MD_CTX *ctx = EVP_MD_CTX_new();
    size_t public_length = 32U;
    size_t signature_length = 64U;
    int ok = key != NULL && ctx != NULL &&
        EVP_PKEY_get_raw_public_key(key, public_key, &public_length) == 1 &&
        public_length == 32U &&
        EVP_DigestSignInit(ctx, NULL, NULL, NULL, key) == 1 &&
        EVP_DigestSign(ctx, signature, &signature_length, digest, 32U) == 1 &&
        signature_length == 64U;

    EVP_MD_CTX_free(ctx);
    EVP_PKEY_free(key);
    LXP_CASE_CHECK(ok);
    return 0;
}

static void golden_hex(const uint8_t bytes[32], char hex[65])
{
    static const char digits[] = "0123456789abcdef";

    for (size_t i = 0U; i < 32U; ++i) {
        hex[2U * i] = digits[bytes[i] >> 4U];
        hex[2U * i + 1U] = digits[bytes[i] & 15U];
    }
    hex[64] = '\0';
}

static void golden_u64(uint8_t *bytes, uint64_t value)
{
    for (size_t i = 0U; i < 8U; ++i)
        bytes[i] = (uint8_t)(value >> (56U - 8U * i));
}

static int golden_open(golden_fixture *f)
{
    uint8_t encoded[65536U];
    lxp_genesis_module_plan plan;
    size_t count = 0U;
    FILE *file = fopen(
        "tests/fixtures/public-testnet-genesis/genesis.manifest", "rb");
    size_t length;
    int failed;

    LXP_CASE_CHECK(file != NULL);
    length = fread(encoded, 1U, sizeof(encoded), file);
    failed = ferror(file) || !feof(file);
    if (fclose(file) != 0) failed = 1;
    LXP_CASE_CHECK(!failed);
    LXP_CASE_CHECK(lxp_genesis_parse(encoded, length,
        LXP_GENESIS_INPUT_MANIFEST, &f->genesis) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&f->arena, f->storage,
        sizeof(f->storage)) == LXP_OK);
    LXP_CASE_CHECK(lx_account_registry_init(&f->accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_init(&f->state, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_bind_accounts(&f->state,
        &f->accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&f->kernel, &f->state, &f->journal,
        &f->genesis, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_genesis_module_plan_resolve(&f->genesis,
        &plan) == LXP_OK);
    LXP_CASE_CHECK(lxp_genesis_module_plan_register(&plan,
        &f->kernel) == LXP_OK);
    LXP_CASE_CHECK(lxp_genesis_materialize(&f->genesis, &f->arena,
        &f->kernel) == LXP_OK);
    LXP_CASE_CHECK(lxp_genesis_parameter_version(&f->genesis,
        &f->parameter_version) == LXP_OK);
    LXP_CASE_CHECK(lxp_fee_committed_schedule(&f->kernel,
        f->parameter_version, &f->fees) == LXP_OK);
    memcpy(f->asset_id, f->genesis.accounts[0].asset_id, 32U);
    LXP_CASE_CHECK(lx_asset_committed_records(&f->kernel, f->records,
        sizeof(f->records) / sizeof(f->records[0]), &count) == LXP_OK);
    for (size_t i = 0U; i < count; ++i) {
        if (memcmp(f->records[i].asset_id, f->asset_id, 32U) == 0) {
            LXP_CASE_CHECK(lx_asset_transfer_state(&f->records[i],
                &f->transfer_asset) == LXP_OK);
            break;
        }
    }
    LXP_CASE_CHECK(f->transfer_asset.registered);
    f->asset_runtime = (lx_asset_runtime){&f->accounts, f->records, count,
        &f->transfer_asset, 1U, f->genesis.network_id,
        f->genesis.protocol_version, false};
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel,
        LXP_MODULE_ASSET, &f->asset_runtime) == LXP_OK);
    f->runtime.accounts = &f->accounts;
    f->runtime.assets = &f->transfer_asset;
    f->runtime.asset_count = 1U;
    memcpy(f->runtime.occupancy_asset_id, f->asset_id, 32U);
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&f->kernel,
        LXP_MODULE_PROGRAMS, &f->runtime) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_set_capabilities(&f->kernel, NULL,
        lxp_kernel_canonical_ledger_apply) == LXP_OK);
    return 0;
}

static int golden_actors(golden_fixture *f)
{
    uint8_t digest[32] = {0U};
    uint8_t signature[64];

    LXP_CASE_CHECK(golden_sign(digest, signature, f->public_key) == 0);
    LXP_CASE_CHECK(lx_account_registry_reserve(&f->accounts, 32U) == LXP_OK);
    for (size_t i = 0U; i < GOLDEN_ACTORS; ++i) {
        char name[80];
        lx_account *account;
        lxp_identity *identity;
        int length = snprintf(f->dids[i], sizeof(f->dids[i]),
            "did:key:snapshot-%u", (unsigned)i);
        LXP_CASE_CHECK(length > 0 && (size_t)length < sizeof(f->dids[i]));
        LXP_CASE_CHECK(lxp_identity_register(&f->identities,
            (const uint8_t *)f->dids[i], (size_t)length, f->public_key,
            &identity) == LXP_OK);
        length = snprintf(name, sizeof(name), "agent:%s:main", f->dids[i]);
        LXP_CASE_CHECK(length > 0 && (size_t)length < sizeof(name));
        LXP_CASE_CHECK(lx_account_id_from_string((const uint8_t *)name,
            (size_t)length, f->principals[i]) == LXP_OK);
        LXP_CASE_CHECK(lx_account_open(&f->accounts, (const uint8_t *)name,
            (size_t)length, f->principals[i], 0U, LX_ACCOUNT_OPEN_CREDIT,
            NULL, &account) == LXP_OK);
        LXP_CASE_CHECK(lxp_ledger_bootstrap_balance(account, f->asset_id,
            (lxp_u128){0U, 10000U}, 0U) == LXP_OK);
        account->has_authority_key = true;
        memcpy(account->authority_key, f->public_key, 32U);
    }
    LXP_CASE_CHECK(lxp_state_root(&f->kernel,
        f->kernel.current_state_root) == LXP_OK);
    return 0;
}

static int golden_execute(golden_fixture *f, size_t actor, uint32_t type,
                          const uint8_t *payload, size_t length)
{
    lxp_activity activity = {0};
    lxp_kernel_execution execution = {0};
    lxp_authority_resolved authority = {0};
    lxp_identity *identity;
    const lxp_module_registration *registration;
    uint8_t signature[64], digest[32];
    uint64_t sequence = f->state.next_sequence;
    lxp_result result;

    LXP_CASE_CHECK(lxp_arena_reset(&f->arena, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_identity_resolve(&f->identities,
        (const uint8_t *)f->dids[actor], strlen(f->dids[actor]),
        &identity) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_module_for_activity(&f->kernel, type,
        f->kernel.epoch, &registration) == LXP_OK);
    activity.protocol_version = f->genesis.protocol_version;
    activity.network_id = f->genesis.network_id;
    activity.activity_type = type;
    activity.actor_did = (lxp_byte_span){(const uint8_t *)f->dids[actor],
        strlen(f->dids[actor])};
    activity.authority = (lxp_byte_span){f->public_key, 32U};
    activity.signature = (lxp_byte_span){signature, sizeof(signature)};
    activity.account_sequence = identity->next_sequence;
    activity.timestamp_bound.not_before = f->genesis.genesis_timestamp_ms;
    activity.timestamp_bound.not_after = f->genesis.genesis_timestamp_ms + 100000U;
    activity.idempotency_key[0] = (uint8_t)sequence;
    activity.fee_limit.lo = 4U;
    activity.payload = (lxp_byte_span){payload, length};
    LXP_CASE_CHECK(lxp_hash_payload(payload, length,
        activity.payload_hash) == LXP_OK);
    LXP_CASE_CHECK(lxp_activity_signing_preimage(&activity, digest) == LXP_OK);
    LXP_CASE_CHECK(golden_sign(digest, signature, f->public_key) == 0);
    LXP_CASE_CHECK(lxp_activity_verify_signature(&activity) == LXP_OK);
    authority.kind = LXP_AUTHORITY_OWNER;
    memcpy(authority.principal, f->principals[actor], 32U);
    memcpy(authority.actor, identity->did_id, 32U);
    memcpy(authority.verified_key, f->public_key, 32U);
    execution.network_id = f->genesis.network_id;
    execution.batch_number = sequence;
    execution.batch_timestamp_ms = f->genesis.genesis_timestamp_ms + sequence * 1000U;
    execution.maximum_timestamp_window = 100000U;
    execution.epoch = f->kernel.epoch;
    execution.global_sequence = sequence;
    execution.recorded_module_version = registration->abi_version;
    execution.parameter_version = f->parameter_version;
    execution.signature_valid = true;
    execution.identities = &f->identities;
    execution.authority = &authority;
    execution.fee_parameters = &f->fees;
    execution.fee_balance.lo = 1000U;
    execution.gas_limit = 1000000U;
    execution.arena = &f->arena;
    execution.sequencer_private_key = golden_seed;
    execution.batch_id[0] = (uint8_t)sequence;
    result = lxp_kernel_execute_activity(&f->kernel, &activity, &execution,
        &f->last_receipt);
    if (result != LXP_OK || f->last_receipt.result_code != LXP_OK)
        fprintf(stderr, "golden activity %u type %08x: execute=%u receipt=%u\n",
            (unsigned)sequence, (unsigned)type, (unsigned)result,
            (unsigned)f->last_receipt.result_code);
    LXP_CASE_CHECK(result == LXP_OK);
    LXP_CASE_CHECK(f->last_receipt.result_code == LXP_OK);
    LXP_CASE_CHECK(f->last_receipt.global_sequence == sequence);
    LXP_CASE_CHECK(f->state.next_sequence == sequence + 1U);
    return 0;
}

static int golden_sequence(golden_fixture *f)
{
    uint8_t payload[512] = {0U, 1U};
    uint8_t stream_id[32] = {0x41U};
    lx_stream_open_payload open = {0};
    char name[160], hex[65];
    size_t length;
    static const uint32_t lifecycle[] = {
        LX_STREAM_PAUSE, LX_STREAM_RESUME, LX_STREAM_PAUSE, LX_STREAM_RESUME,
        LX_STREAM_PAUSE, LX_STREAM_RESUME, LX_STREAM_PAUSE
    };

    LXP_CASE_CHECK(golden_actors(f) == 0);
    memcpy(payload + 2U, f->asset_id, 32U);
    for (size_t i = 0U; i < GOLDEN_ACTORS; ++i)
        LXP_CASE_CHECK(golden_execute(f, i, LX_ASSET_ACCOUNT_OPEN,
            payload, 34U) == 0);

    memcpy(open.record.stream_id, stream_id, 32U);
    golden_hex(stream_id, hex);
    int named = snprintf(name, sizeof(name), "agent:%s:stream:%s", f->dids[0], hex);
    LXP_CASE_CHECK(named > 0 && (size_t)named < sizeof(name));
    LXP_CASE_CHECK(lx_account_id_from_string((const uint8_t *)name,
        (size_t)named, open.record.stream_account) == LXP_OK);
    memcpy(open.record.recipient, f->principals[1], 32U);
    memcpy(open.record.asset_id, f->asset_id, 32U);
    open.record.mode = LX_STREAM_MODE_TIME;
    open.record.rate.lo = 3U;
    open.record.rate_unit = 1000U;
    open.record.start_timestamp = f->genesis.genesis_timestamp_ms + 9000U;
    open.record.total_cap.lo = 500U;
    open.initial_funding.lo = 100U;
    LXP_CASE_CHECK(lx_stream_open_encode(&open, payload, sizeof(payload),
        &length) == LXP_OK);
    LXP_CASE_CHECK(golden_execute(f, 0U, LX_STREAM_OPEN, payload, length) == 0);
    payload[0] = 0U;
    payload[1] = LX_STREAM_PAYLOAD_VERSION;
    memcpy(payload + 2U, stream_id, 32U);
    for (size_t i = 0U; i < sizeof(lifecycle) / sizeof(lifecycle[0]); ++i)
        LXP_CASE_CHECK(golden_execute(f, 0U, lifecycle[i], payload, 34U) == 0);

    for (size_t i = 0U; i < 4U; ++i) {
        memset(payload, 0, sizeof(payload));
        payload[1] = LX_SERVICE_RECORD_VERSION;
        payload[2] = (uint8_t)(0x51U + i);
        memcpy(payload + 34U, f->asset_id, 32U);
        golden_u64(payload + 74U, 25U + i);
        payload[82] = 0x61U;
        payload[114] = 0x71U;
        golden_u64(payload + 146U, f->genesis.genesis_timestamp_ms + 200000U);
        golden_u64(payload + 154U, 20000U);
        golden_u64(payload + 162U, 30000U);
        payload[170] = LX_SERVICE_DEFAULT_ACCEPT;
        golden_u64(payload + 171U, f->genesis.genesis_timestamp_ms + 100000U);
        LXP_CASE_CHECK(golden_execute(f, 0U, LX_SERVICE_OFFER_PUBLISH,
            payload, LX_SERVICE_OFFER_PUBLISH_PAYLOAD_BYTES) == 0);
        LXP_CASE_CHECK(golden_execute(f, 0U, LX_SERVICE_OFFER_WITHDRAW,
            payload, LX_SERVICE_IDENTIFIER_PAYLOAD_BYTES) == 0);
    }
    LXP_CASE_CHECK(f->state.next_sequence == GOLDEN_ACTIVITIES + 1U);
    return 0;
}

static int golden_snapshot(golden_fixture *f, lxp_byte_span *snapshot)
{
    lxp_codec_reader reader;
    uint16_t version;
    uint64_t sequence;
    uint32_t count;

    LXP_CASE_CHECK(lxp_arena_reset(&f->arena, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_snapshot_write(&f->kernel, GOLDEN_ACTIVITIES,
        &f->arena, snapshot) == LXP_OK);
    LXP_CASE_CHECK(lxp_codec_reader_init(&reader, snapshot->bytes,
        snapshot->length) == LXP_OK);
    LXP_CASE_CHECK(lxp_codec_read_struct_header_version(&reader,
        LXP_SNAPSHOT_FORMAT_VERSION, &version) == LXP_OK);
    LXP_CASE_CHECK(version == LXP_PROTOCOL_VERSION_OCCUPANCY);
    LXP_CASE_CHECK(lxp_codec_read_u64(&reader, &sequence) == LXP_OK);
    LXP_CASE_CHECK(sequence == GOLDEN_ACTIVITIES);
    LXP_CASE_CHECK(lxp_codec_read_u32(&reader, &count) == LXP_OK);
    LXP_CASE_CHECK(count == f->state.count);
    for (size_t i = 0U; i < count; ++i) {
        lxp_byte_span key;
        lxp_u128 value;
        LXP_CASE_CHECK(lxp_codec_read_bytes(&reader, &key, 32U) == LXP_OK);
        LXP_CASE_CHECK(key.length == 32U);
        LXP_CASE_CHECK(lxp_codec_read_u128(&reader, &value) == LXP_OK);
    }
    LXP_CASE_CHECK(lxp_codec_read_u32(&reader, &count) == LXP_OK);
    LXP_CASE_CHECK(count == 0U);
    return 0;
}

static int golden_check_digest(lxp_byte_span snapshot)
{
    uint8_t digest[32];
    char hex[65];

    LXP_CASE_CHECK(lxp_hash_sha256(snapshot.bytes, snapshot.length, digest) == LXP_OK);
    if (memcmp(digest, golden_digest, sizeof(digest)) != 0) {
        golden_hex(digest, hex);
        fprintf(stderr, "snapshot SHA-256: %s (%zu bytes)\n", hex, snapshot.length);
    }
    LXP_CASE_CHECK(memcmp(digest, golden_digest, sizeof(digest)) == 0);
    return 0;
}

static int golden_close(golden_fixture *f)
{
    while (f->kernel.blob_count != 0U)
        free(f->kernel.blobs[--f->kernel.blob_count].bytes);
    LXP_CASE_CHECK(lxp_state_store_destroy(&f->state) == LXP_OK);
    lx_account_registry_release(&f->accounts);
    free(f);
    return 0;
}

static int snapshot_golden_digest(void)
{
    golden_fixture *f = calloc(1U, sizeof(*f));
    lxp_byte_span snapshot;

    LXP_CASE_CHECK(f != NULL);
    LXP_CASE_CHECK(golden_open(f) == 0);
    LXP_CASE_CHECK(golden_sequence(f) == 0);
    LXP_CASE_CHECK(golden_snapshot(f, &snapshot) == 0);
    LXP_CASE_CHECK(golden_check_digest(snapshot) == 0);
    LXP_CASE_CHECK(golden_close(f) == 0);
    return 0;
}

static int snapshot_golden_reload_identical(void)
{
    golden_fixture *source = calloc(1U, sizeof(*source));
    golden_fixture *target = calloc(1U, sizeof(*target));
    lxp_byte_span before, after;
    lxp_snapshot_manifest_record manifest;
    uint8_t root[32], loaded_root[32];
    lxp_state_proof proof, loaded_proof;

    LXP_CASE_CHECK(source != NULL && target != NULL);
    LXP_CASE_CHECK(golden_open(source) == 0);
    LXP_CASE_CHECK(golden_sequence(source) == 0);
    LXP_CASE_CHECK(golden_snapshot(source, &before) == 0);
    LXP_CASE_CHECK(golden_check_digest(before) == 0);
    LXP_CASE_CHECK(lxp_state_root_proof(&source->kernel, 0U, root,
        &proof) == LXP_OK);
    LXP_CASE_CHECK(memcmp(root, source->last_receipt.resulting_state_root, 32U) == 0);
    LXP_CASE_CHECK(lxp_snapshot_manifest_build(before.bytes, before.length,
        GOLDEN_ACTIVITIES, root, source->last_receipt.resulting_state_root,
        &manifest) == LXP_OK);
    LXP_CASE_CHECK(golden_open(target) == 0);
    LXP_CASE_CHECK(target->state.next_sequence == 1U);
    LXP_CASE_CHECK(lxp_snapshot_load(before.bytes, before.length, &manifest,
        &target->kernel) == LXP_OK);
    LXP_CASE_CHECK(target->state.next_sequence == GOLDEN_ACTIVITIES + 1U);
    LXP_CASE_CHECK(golden_snapshot(target, &after) == 0);
    LXP_CASE_CHECK(before.length == after.length);
    LXP_CASE_CHECK(memcmp(before.bytes, after.bytes, before.length) == 0);
    uint32_t module_count = proof.leaf_count;
    for (uint16_t module = 0U; module < module_count; ++module) {
        LXP_CASE_CHECK(lxp_state_root_proof(&source->kernel, module, root,
            &proof) == LXP_OK);
        LXP_CASE_CHECK(lxp_state_root_proof(&target->kernel, module, loaded_root,
            &loaded_proof) == LXP_OK);
        LXP_CASE_CHECK(memcmp(root, loaded_root, 32U) == 0);
        LXP_CASE_CHECK(proof.leaf_index == loaded_proof.leaf_index);
        LXP_CASE_CHECK(proof.leaf_count == loaded_proof.leaf_count);
        LXP_CASE_CHECK(proof.depth == loaded_proof.depth);
        LXP_CASE_CHECK(memcmp(proof.siblings, loaded_proof.siblings,
            (size_t)proof.depth * 32U) == 0);
    }
    LXP_CASE_CHECK(golden_close(source) == 0);
    LXP_CASE_CHECK(golden_close(target) == 0);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        {"snapshot_golden_digest", snapshot_golden_digest},
        {"snapshot_golden_reload_identical", snapshot_golden_reload_identical}
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
