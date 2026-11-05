/* Proves: programs.h / differential_fixture_init_balanced + pay5_account_payload
 * -> lxp_kernel_execute_activity / snapshot -> lxp_programs_interface_read,
 * lxp_programs_state_record_encode and layerx-types' generated Programs tables.
 * Fixes accounts.c:lxp_programs_account_register accepting duplicate bindings
 * with mismatched primary/reverse versions or provenance. Also separates module
 * ABI from guest ABI in the differential producer for state-commitment receipts.
 * tests/codec/lxp_test_programs_retired.c requires lxp_programs_replay_finalize
 * and lxp_programs_replay_engine_bind absent: this inventory predates retirement.
 * The refusal case preserves that requirement, including retired ordinal eight.
 * LX_PROGRAMS_FEE_PRICE_FIELDS remains seven; the retired proposal-size constant
 * and relay runtime member are deliberately absent from the current header.
 */
#define _POSIX_C_SOURCE 200809L
#define LXP_PROGRAMS_DIFFERENTIAL_EMBEDDED
#include "../../programs/tests/differential/parallel.c"
#define LXP_PAY5_ACCOUNT_PAYLOAD_ONLY
#include "../../programs/fixtures/pay5/native_roundtrip.h"
#include "../codec/lxp_kernel_case.h"
#include <sys/wait.h>
#include <unistd.h>

extern lxp_result lxp_programs_replay_finalize(
    void *, const lxp_batch_header *, uint32_t, uint64_t, const uint8_t *,
    lxp_arena *, lxp_replay_activity_output *);
extern lxp_result lxp_programs_replay_engine_bind(lxp_replay_engine *, lxp_kernel *);
#pragma weak lxp_programs_replay_finalize
#pragma weak lxp_programs_replay_engine_bind

static differential_fixture proof_fixture;
static uint8_t proof_bytes[4U * LXP_MAX_ACTIVITY_BYTES];
static uint8_t snapshot_bytes[4U * LXP_MAX_ACTIVITY_BYTES];
static uint8_t record_bytes[4096U * sizeof(lx_programs_value_account_view) +
                            2U * LXP_MAX_ACTIVITY_BYTES];
static uint8_t saved_record[2U * LXP_MAX_ACTIVITY_BYTES];
static lxp_verified_receipt_index proof_receipts;
static const uint8_t proof_seed[] = "programs-h-proof";
static bool proof_ready;

static int proof_context(lxp_module_ctx *ctx, lxp_arena *arena, bool mutable)
{
    LXP_CASE_CHECK(lxp_arena_init(arena, proof_bytes, sizeof(proof_bytes)) == LXP_OK);
    if (mutable) {
        LXP_CASE_CHECK(lxp_state_journal_open(&proof_fixture.state,
            proof_fixture.state.next_sequence, &proof_fixture.journal) == LXP_OK);
    }
    LXP_CASE_CHECK(lxp_module_ctx_init(ctx, &proof_fixture.kernel,
        LXP_MODULE_PROGRAMS, 20U, 1U, proof_fixture.state.next_sequence,
        1000000U, arena, mutable) == LXP_OK);
    ctx->protocol_version = proof_fixture.protocol_version;
    ctx->batch_number = 2U;
    ctx->verified_receipts = &proof_receipts;
    return 0;
}

static int proof_register(lxp_receipt *receipt)
{
    static uint8_t payload[73U + LX_PROGRAMS_ACCOUNT_MAX_SEED_BYTES];
    uint8_t signature[64];
    size_t length;
    lxp_activity activity;
    lxp_arena arena;
    lxp_kernel_execution execution = {0};
    LXP_CASE_CHECK(pay5_account_payload(payload, sizeof(payload),
        proof_fixture.program_id, proof_fixture.fee_asset, proof_seed,
        sizeof(proof_seed) - 1U, &length) == LXP_OK);
    fill_activity(&activity, LX_PROGRAMS_ACCOUNT, payload, length,
        proof_fixture.dids[0], proof_fixture.did_lengths[0], proof_fixture.keys[0]);
    activity.protocol_version = proof_fixture.protocol_version;
    activity.account_sequence = proof_fixture.identities.identities[0].next_sequence;
    write_u64(activity.idempotency_key + 24U, proof_fixture.state.next_sequence);
    LXP_CASE_CHECK(sign_activity(&activity, proof_fixture.private_keys[0], signature) == 0);
    activity.signature = (lxp_byte_span){signature, sizeof(signature)};
    LXP_CASE_CHECK(lxp_activity_verify_signature(&activity) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&arena, proof_bytes, sizeof(proof_bytes)) == LXP_OK);
    execution.network_id = 7U;
    execution.batch_number = 2U;
    execution.batch_timestamp_ms = 20U;
    execution.maximum_timestamp_window = 100U;
    execution.global_sequence = proof_fixture.state.next_sequence;
    execution.epoch = 1U;
    execution.recorded_module_version = proof_fixture.module_version;
    execution.parameter_version = 1U;
    execution.signature_valid = true;
    execution.identities = &proof_fixture.identities;
    execution.authority = &proof_fixture.authorities[0];
    execution.fee_parameters = &proof_fixture.fees;
    execution.gas_limit = 1000000U;
    execution.arena = &arena;
    execution.sequencer_private_key = proof_fixture.sequencer_private_key;
    {
        lxp_byte_span canonical;
        lxp_batch_roots roots;
        size_t mark = lxp_arena_mark(&arena);
        LXP_CASE_CHECK(lxp_activity_encode(&activity, &arena, &canonical) == LXP_OK);
        LXP_CASE_CHECK(bind_differential_prefix(&canonical, 1U,
            proof_fixture.kernel.current_state_root, execution.global_sequence,
            execution.batch_number, &arena, &execution, &roots,
            execution.batch_id) == LXP_OK);
        LXP_CASE_CHECK(lxp_arena_reset(&arena, mark) == LXP_OK);
    }
    {
        lxp_result status = lxp_kernel_execute_activity(&proof_fixture.kernel,
            &activity, &execution, receipt);
        if (status != LXP_OK) (void)fprintf(stderr, "account producer status=%d\n", (int)status);
        LXP_CASE_CHECK(status == LXP_OK);
    }
    LXP_CASE_CHECK(receipt->result_code == LXP_OK);
    return 0;
}

static int proof_run(char *const arguments[], const char *crate, const char *output)
{
    pid_t child = fork();
    int status;
    LXP_CASE_CHECK(child >= 0);
    if (child == 0) {
        if (crate != NULL && (setenv("CARGO_MANIFEST_DIR", crate, 1) != 0 ||
                              setenv("OUT_DIR", output, 1) != 0)) _exit(126);
        execvp(arguments[0], arguments);
        perror(arguments[0]);
        _exit(127);
    }
    LXP_CASE_CHECK(waitpid(child, &status, 0) == child);
    LXP_CASE_CHECK(WIFEXITED(status) && WEXITSTATUS(status) == 0);
    return 0;
}

/* Execute the actual build.rs and dependency-free crate modules. The producer
 * uses local paths created here; no service or operator configuration is read.
 * The consumer receives the actual kernel receipt's version and activity type. */
static int proof_agent_consumer(const lxp_receipt *receipt)
{
    char root[4096], crate[4352], directory[] = "/tmp/lxp-programs-h-XXXXXX";
    char producer[256], harness[256], consumer[256], version[16], activity[16];
    FILE *source;
    LXP_CASE_CHECK(getcwd(root, sizeof(root)) != NULL);
    LXP_CASE_CHECK(mkdtemp(directory) != NULL);
    LXP_CASE_CHECK(snprintf(crate, sizeof(crate),
        "%s/services/agent/crates/layerx-types", root) > 0);
    LXP_CASE_CHECK(snprintf(producer, sizeof(producer), "%s/producer", directory) > 0);
    LXP_CASE_CHECK(snprintf(harness, sizeof(harness), "%s/consumer.rs", directory) > 0);
    LXP_CASE_CHECK(snprintf(consumer, sizeof(consumer), "%s/consumer", directory) > 0);
    LXP_CASE_CHECK(snprintf(version, sizeof(version), "%u", receipt->module_version) > 0);
    LXP_CASE_CHECK(snprintf(activity, sizeof(activity), "%u", (unsigned)LX_PROGRAMS_ACCOUNT) > 0);
    {
        char build_script[4608];
        LXP_CASE_CHECK(snprintf(build_script, sizeof(build_script), "%s/build.rs", crate) > 0);
        char *compile[] = {"rustc", "--edition=2021", build_script, "-o", producer, NULL};
        char *generate[] = {producer, NULL};
        LXP_CASE_CHECK(proof_run(compile, NULL, NULL) == 0);
        LXP_CASE_CHECK(proof_run(generate, crate, directory) == 0);
    }
    source = fopen(harness, "wb");
    LXP_CASE_CHECK(source != NULL);
    LXP_CASE_CHECK(fprintf(source,
        "#[path = \"%s/src/limits.rs\"] mod limits;\n"
        "#[path = \"%s/src/payload.rs\"] mod payload;\n"
        "#[path = \"%s/src/programs_activity.rs\"] mod programs_activity;\n"
        "#[path = \"%s/src/programs_module_abi.rs\"] mod programs_module_abi;\n"
        "fn main() {\n"
        " let args: Vec<String> = std::env::args().collect();\n"
        " let version: u32 = args[1].parse().unwrap();\n"
        " let packed: u32 = args[2].parse().unwrap();\n"
        " let ordinal = programs_activity::ACCOUNT_ORDINAL;\n"
        " assert!(programs_activity::allocated(ordinal));\n"
        " assert_eq!(programs_activity::activity_type(ordinal).unwrap().value(), packed);\n"
        " assert_eq!(programs_activity::receipt_operation(ordinal), 0);\n"
        " assert!(programs_module_abi::supported(version));\n"
        " assert_eq!(version, programs_module_abi::SANDBOX_DESTROY);\n"
        " assert!(!programs_activity::allocated(8));\n"
        " assert!(!programs_activity::allocated(0));\n"
        " assert!(programs_activity::activity_type(0).is_err());\n"
        " assert!(!programs_module_abi::supported(0));\n"
        " assert!(!programs_module_abi::supported(programs_module_abi::CURRENT + 1));\n"
        "}\n", crate, crate, crate, crate) > 0);
    LXP_CASE_CHECK(fclose(source) == 0);
    {
        char *compile[] = {"rustc", "--edition=2021", harness, "-o", consumer, NULL};
        char *consume[] = {consumer, version, activity, NULL};
        LXP_CASE_CHECK(proof_run(compile, crate, directory) == 0);
        LXP_CASE_CHECK(proof_run(consume, NULL, NULL) == 0);
    }
    return 0;
}

static int proof_programs_h_happy_path(void)
{
    lxp_receipt receipt;
    lxp_module_ctx ctx;
    lxp_arena arena, snapshot_arena, record_arena;
    lxp_byte_span snapshot, record;
    lxp_snapshot_manifest_record manifest;
    uint8_t root[32], digest[32], public_key[32], account_id[32];
    uint8_t interface_copy[2048], expected_interface[INTERFACE_MAX_FIXTURE_BYTES];
    uint8_t wasm[256], code_hash[32];
    const uint8_t *interface_bytes;
    size_t interface_length, expected_length, saved_length, account_count;
    lx_programs_account_binding binding;
    lx_account *account;
    bool created = true;

    LXP_CASE_CHECK(LX_PROGRAMS_FEE_PRICE_FIELDS == 7);
    LXP_CASE_CHECK(differential_fixture_init_balanced(&proof_fixture,
        (lxp_u128){0U, UINT64_MAX}, (lxp_u128){0U, 0U},
        LXP_PROTOCOL_VERSION_STATE_COMMITMENT) == 0);
    LXP_CASE_CHECK(proof_fixture.deployment_receipt.result_code == LXP_OK);
    LXP_CASE_CHECK(proof_register(&receipt) == 0);
    LXP_CASE_CHECK(receipt.module_version == LX_PROGRAMS_SANDBOX_DESTROY_ABI_VERSION);
    LXP_CASE_CHECK(public_key_for(proof_fixture.sequencer_private_key, public_key) == 0);
    LXP_CASE_CHECK(lxp_arena_init(&arena, record_bytes, sizeof(record_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_verified_receipt_index_init(&proof_receipts) == LXP_OK);
    LXP_CASE_CHECK(lxp_verified_receipt_index_add(&proof_receipts, &receipt,
        public_key, &arena) == LXP_OK);
    LXP_CASE_CHECK(lxp_receipt_digest(&receipt, &arena, digest) == LXP_OK);
    LXP_CASE_CHECK(proof_agent_consumer(&receipt) == 0);
    LXP_CASE_CHECK(proof_context(&ctx, &arena, false) == 0);
    LXP_CASE_CHECK(lxp_programs_interface_read(&ctx, proof_fixture.program_id,
        &interface_bytes, &interface_length) == LXP_OK);
    LXP_CASE_CHECK(interface_length <= sizeof(interface_copy));
    (void)memcpy(interface_copy, interface_bytes, interface_length);
    /* deploy.c:lxp_programs_interface_read returns the 72-byte registry prefix
     * followed by the interface, not only the embedded declaration. */
    LXP_CASE_CHECK(lxp_hash_sha256(wasm, event_emitting_module(wasm), code_hash) == LXP_OK);
    expected_length = interface_payload(expected_interface, code_hash,
        LX_PROGRAMS_GUEST_ABI_V2_VERSION, INTERFACE_CAPABILITIES_EMIT_EVENT);
    LXP_CASE_CHECK(interface_length == 72U + expected_length);
    LXP_CASE_CHECK(memcmp(interface_bytes, proof_fixture.program_id, 32U) == 0);
    LXP_CASE_CHECK(memcmp(interface_bytes + 72U, expected_interface, expected_length) == 0);
    LXP_CASE_CHECK(lxp_arena_init(&record_arena, record_bytes, sizeof(record_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_state_record_encode(&ctx, proof_fixture.program_id,
        digest, &record_arena, &record) == LXP_OK);
    LXP_CASE_CHECK(record.length > 40U && record.length <= sizeof(saved_record));
    LXP_CASE_CHECK(memcmp(record.bytes, "LXPS1", 5U) == 0);
    LXP_CASE_CHECK(memcmp(record.bytes + 5U, proof_fixture.program_id, 32U) == 0);
    LXP_CASE_CHECK(record.bytes[37U] == LX_PROGRAMS_LIFECYCLE_ACTIVE &&
        record.bytes[38U] == 0U && record.bytes[39U] == 1U);
    saved_length = record.length;
    (void)memcpy(saved_record, record.bytes, saved_length);
    LXP_CASE_CHECK(lxp_programs_account_derive(proof_fixture.program_id,
        proof_seed, sizeof(proof_seed) - 1U, account_id) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_root(&proof_fixture.kernel, root) == LXP_OK);
    account_count = proof_fixture.accounts.count;
    LXP_CASE_CHECK(lxp_arena_init(&snapshot_arena, snapshot_bytes, sizeof(snapshot_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_snapshot_write(&proof_fixture.kernel,
        receipt.global_sequence, &snapshot_arena, &snapshot) == LXP_OK);
    LXP_CASE_CHECK(lxp_snapshot_manifest_build(snapshot.bytes, snapshot.length,
        receipt.global_sequence, root, receipt.resulting_state_root, &manifest) == LXP_OK);
    LXP_CASE_CHECK(differential_fixture_destroy(&proof_fixture) == LXP_OK);
    lx_account_registry_release(&proof_fixture.accounts);
    (void)memset(&proof_fixture.kernel, 0, sizeof(proof_fixture.kernel));
    LXP_CASE_CHECK(lx_account_registry_init(&proof_fixture.accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_init(&proof_fixture.state, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_store_bind_accounts(&proof_fixture.state, &proof_fixture.accounts) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&proof_fixture.kernel, &proof_fixture.state,
        &proof_fixture.journal, &proof_fixture.parameters, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&proof_fixture.kernel,
        programs_module_registration_v4()) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_bind_module_runtime(&proof_fixture.kernel,
        LXP_MODULE_PROGRAMS, &proof_fixture.runtime) == LXP_OK);
    LXP_CASE_CHECK(lxp_snapshot_load(snapshot.bytes, snapshot.length,
        &manifest, &proof_fixture.kernel) == LXP_OK);
    LXP_CASE_CHECK(proof_fixture.accounts.count == account_count);
    LXP_CASE_CHECK(proof_context(&ctx, &arena, false) == 0);
    LXP_CASE_CHECK(lxp_programs_interface_read(&ctx, proof_fixture.program_id,
        &interface_bytes, &expected_length) == LXP_OK);
    LXP_CASE_CHECK(expected_length == interface_length &&
        memcmp(interface_bytes, interface_copy, interface_length) == 0);
    LXP_CASE_CHECK(lxp_programs_account_lookup_id(&ctx, account_id, &binding, &account) == LXP_OK);
    LXP_CASE_CHECK(binding.registered_sequence == receipt.global_sequence &&
        binding.seed_length == sizeof(proof_seed) - 1U && account->balance.lo == 0U);
    LXP_CASE_CHECK(lxp_arena_reset(&record_arena, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_state_record_encode(&ctx, proof_fixture.program_id,
        digest, &record_arena, &record) == LXP_OK);
    LXP_CASE_CHECK(record.length == saved_length && memcmp(record.bytes, saved_record, saved_length) == 0);
    LXP_CASE_CHECK(proof_context(&ctx, &arena, true) == 0);
    LXP_CASE_CHECK(lxp_programs_account_register(&ctx, proof_fixture.program_id,
        proof_seed, sizeof(proof_seed) - 1U, proof_fixture.fee_asset, &account, &created) == LXP_OK);
    LXP_CASE_CHECK(!created && proof_fixture.accounts.count == account_count);
    LXP_CASE_CHECK(ctx.staged_count == 0U);
    lxp_module_ctx_rollback(&ctx);
    LXP_CASE_CHECK(lxp_state_journal_rollback(&proof_fixture.journal) == LXP_OK);
    proof_ready = true;
    return 0;
}

static int proof_programs_h_refusal(void)
{
    static const uint8_t reverse_prefix[] = "program-account\0r";
    lxp_module_ctx ctx;
    lxp_arena arena, record_arena;
    const uint8_t *bytes;
    size_t length;
    uint8_t unknown[32] = {0xf1U}, account_id[32];
    uint8_t key[sizeof(reverse_prefix) - 1U + 32U];
    uint8_t legacy[99U + sizeof(proof_seed) - 1U];
    lxp_byte_span record;
    lx_account *account = NULL;
    bool created = true;
    uint8_t before[32], after[32];
    if (!proof_ready) LXP_CASE_CHECK(proof_programs_h_happy_path() == 0);
    LXP_CASE_CHECK(lxp_programs_replay_finalize == NULL);
    LXP_CASE_CHECK(lxp_programs_replay_engine_bind == NULL);
    LXP_CASE_CHECK(proof_context(&ctx, &arena, false) == 0);
    LXP_CASE_CHECK(lxp_programs_interface_read(&ctx, unknown, &bytes, &length) == LXP_ERR_UNKNOWN_FIELD);
    LXP_CASE_CHECK(lxp_arena_init(&record_arena, record_bytes, sizeof(record_bytes)) == LXP_OK);
    LXP_CASE_CHECK(lxp_programs_state_record_encode(&ctx, proof_fixture.program_id,
        unknown, &record_arena, &record) == LXP_ERR_UNKNOWN_FIELD);
    LXP_CASE_CHECK(record.bytes == NULL && record.length == 0U);
    LXP_CASE_CHECK(lxp_programs_account_derive(proof_fixture.program_id,
        proof_seed, sizeof(proof_seed) - 1U, account_id) == LXP_OK);
    (void)memcpy(key, reverse_prefix, sizeof(reverse_prefix) - 1U);
    (void)memcpy(key + sizeof(reverse_prefix) - 1U, account_id, 32U);
    LXP_CASE_CHECK(lxp_ctx_kv_get(&ctx, key, sizeof(key), &bytes, &length) == LXP_OK);
    LXP_CASE_CHECK(length == 139U + sizeof(proof_seed) - 1U && bytes[0] == 2U);
    /* Convert only the producer's reverse record to its valid legacy format.
     * The primary record still carries v2 provenance and must disagree. */
    (void)memcpy(legacy, bytes, 99U);
    legacy[0] = 1U;
    (void)memcpy(legacy + 99U, bytes + 139U, sizeof(proof_seed) - 1U);
    LXP_CASE_CHECK(proof_context(&ctx, &arena, true) == 0);
    LXP_CASE_CHECK(lxp_ctx_kv_put(&ctx, key, sizeof(key), legacy, sizeof(legacy)) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_prepare_commit(&ctx) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_journal_commit(&proof_fixture.journal) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_commit(&ctx) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_root(&proof_fixture.kernel, before) == LXP_OK);
    LXP_CASE_CHECK(proof_context(&ctx, &arena, true) == 0);
    LXP_CASE_CHECK(lxp_programs_account_register(&ctx, proof_fixture.program_id,
        proof_seed, sizeof(proof_seed) - 1U, proof_fixture.fee_asset, &account, &created) == LXP_ERR_CONTEXT_MISMATCH);
    LXP_CASE_CHECK(ctx.staged_count == 0U && ctx.staged_account_count == 0U);
    lxp_module_ctx_rollback(&ctx);
    LXP_CASE_CHECK(lxp_state_journal_rollback(&proof_fixture.journal) == LXP_OK);
    LXP_CASE_CHECK(lxp_state_root(&proof_fixture.kernel, after) == LXP_OK);
    LXP_CASE_CHECK(memcmp(before, after, 32U) == 0);
    LXP_CASE_CHECK(differential_fixture_destroy(&proof_fixture) == LXP_OK);
    lx_account_registry_release(&proof_fixture.accounts);
    proof_ready = false;
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        {"proof_programs_h_happy_path", proof_programs_h_happy_path},
        {"proof_programs_h_refusal", proof_programs_h_refusal}
    };
    /* lxp_kernel_case_main requires names; supply every case for no arguments. */
    if (argc == 1) {
        char *all[] = {argv[0], "proof_programs_h_happy_path", "proof_programs_h_refusal"};
        return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), 3, all);
    }
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
