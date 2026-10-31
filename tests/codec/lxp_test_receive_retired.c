#define _POSIX_C_SOURCE 200809L

#include "lxp_kernel_case.h"

#include "layerx/lx_asset.h"
#include "layerx/lxp_crypto.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_kernel.h"

#include <openssl/evp.h>
#include <stdlib.h>

struct lxp_receive;
struct lxp_receive_environment;
struct lxp_grant_store;
struct lxp_grant_state;
struct lxp_payer_grant;

extern lxp_result lxp_receive_validate(const struct lxp_receive *,
    const struct lxp_receive_environment *);
extern lxp_result lxp_receive_execute(const struct lxp_receive *,
    struct lxp_receive_environment *, lxp_send_receipt_projection *);
extern lxp_result lxp_grant_store_put(struct lxp_grant_store *,
    const struct lxp_payer_grant *, const lx_account *);
extern lxp_result lxp_grant_revoke(struct lxp_grant_store *,
    const uint8_t[32], uint64_t);
extern lxp_result lxp_grant_draw_record(struct lxp_grant_state *, lxp_u128,
    uint64_t);
extern lxp_result lxp_grant_authorization_message(const struct lxp_payer_grant *,
    uint8_t *, size_t, size_t *);
extern lxp_result lxp_verify_payer_grant(const struct lxp_payer_grant *,
    const lx_account *);
extern lxp_result lx_asset_receive_execute(lxp_module_ctx *,
    const lx_asset_transfer_request *, lxp_receipt *);
extern lxp_result lx_asset_payer_grant_read(const lxp_kernel *,
    const uint8_t[32], struct lxp_grant_state *);
extern lxp_result lx_budget_grant_draws_read(const lxp_kernel *,
    const uint8_t[32], struct lxp_grant_state *);

/* Task 0.62 planned these entry points, but neither was ever built. */
extern void lx_asset_payer_grant_encode(void);
extern void lxp_ctx_asset_grant_state_stage(void);

#pragma weak lxp_receive_validate
#pragma weak lxp_receive_execute
#pragma weak lxp_grant_store_put
#pragma weak lxp_grant_revoke
#pragma weak lxp_grant_draw_record
#pragma weak lxp_grant_authorization_message
#pragma weak lxp_verify_payer_grant
#pragma weak lx_asset_receive_execute
#pragma weak lx_asset_payer_grant_read
#pragma weak lx_budget_grant_draws_read
#pragma weak lx_asset_payer_grant_encode
#pragma weak lxp_ctx_asset_grant_state_stage

static int sign_activity(lxp_activity *activity, uint8_t public_key[32],
                         uint8_t signature[64])
{
    static const uint8_t seed[32] = {1U};
    uint8_t digest[32];
    size_t public_key_length = 32U;
    size_t signature_length = 64U;
    EVP_PKEY *key = EVP_PKEY_new_raw_private_key(EVP_PKEY_ED25519, NULL,
                                                seed, sizeof(seed));
    EVP_MD_CTX *ctx = EVP_MD_CTX_new();
    int ok = key != NULL && ctx != NULL &&
        EVP_PKEY_get_raw_public_key(key, public_key, &public_key_length) == 1 &&
        public_key_length == 32U;
    activity->authority = (lxp_byte_span){public_key, 32U};
    activity->signature = (lxp_byte_span){signature, 64U};
    if (ok) ok = lxp_activity_signing_preimage(activity, digest) == LXP_OK &&
        EVP_DigestSignInit(ctx, NULL, NULL, NULL, key) == 1 &&
        EVP_DigestSign(ctx, signature, &signature_length, digest,
                       sizeof(digest)) == 1 && signature_length == 64U;
    EVP_MD_CTX_free(ctx);
    EVP_PKEY_free(key);
    LXP_CASE_CHECK(ok);
    return 0;
}

static int receive_and_grant_types_refused(void)
{
    static const uint32_t retired_types[] = {
        0x00010006U, 0x00010007U, 0x00010008U
    };
    static const uint8_t did[] = "did:key:alice";
    static const uint8_t payload[] = {0U, 1U};
    static lxp_kernel kernel;
    static lxp_state_store state;
    static lxp_state_journal journal;
    static lxp_identity_store identities;
    static lxp_receipt receipt;
    static uint8_t arena_bytes[LXP_MAX_ACTIVITY_BYTES];
    static uint8_t wire_bytes[LXP_MAX_ACTIVITY_BYTES];
    const lxp_module_iface *iface = lx_asset_module_iface();
    const uint64_t parameters = 1U;
    lxp_authority_resolved authority = {0};
    lxp_fee_params fees = {0};
    lxp_kernel_execution execution = {0};
    lxp_arena arena;
    lxp_arena wire_arena;
    uint8_t before[32];
    uint8_t after[32];

    LXP_CASE_CHECK(lxp_state_store_init(&state, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&kernel, &state, &journal,
                                    &parameters, 1U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&kernel, iface) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&arena, arena_bytes,
                                 sizeof(arena_bytes)) == LXP_OK);
    LXP_CASE_CHECK(iface->activity_type_count == 8U);
    execution.network_id = 7U;
    execution.batch_number = 1U;
    execution.batch_timestamp_ms = 10U;
    execution.maximum_timestamp_window = 100U;
    execution.epoch = 1U;
    execution.global_sequence = 1U;
    execution.recorded_module_version = 1U;
    execution.parameter_version = 1U;
    execution.signature_valid = true;
    execution.identities = &identities;
    execution.authority = &authority;
    execution.fee_parameters = &fees;
    execution.arena = &arena;
    LXP_CASE_CHECK(lxp_state_root(&kernel, before) == LXP_OK);

    for (size_t i = 0U; i < sizeof(retired_types) / sizeof(retired_types[0]); ++i) {
        const lxp_module_registration *registration = NULL;
        lxp_activity activity = {0};
        lxp_activity received;
        lxp_byte_span wire;
        uint8_t public_key[32];
        uint8_t signature[64];
        lxp_module_ctx ctx;
        void *decoded = NULL;
        size_t mark = lxp_arena_mark(&arena);

        for (size_t j = 0U; j < iface->activity_type_count; ++j)
            LXP_CASE_CHECK(iface->activity_types[j] != retired_types[i]);
        LXP_CASE_CHECK(lxp_kernel_module_for_activity(&kernel, retired_types[i],
            1U, &registration) == LXP_ERR_UNKNOWN_ACTIVITY);
        LXP_CASE_CHECK(registration == NULL);
        activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
        activity.network_id = 7U;
        activity.activity_type = retired_types[i];
        activity.actor_did = (lxp_byte_span){did, sizeof(did) - 1U};
        activity.payload = (lxp_byte_span){payload, sizeof(payload)};
        activity.timestamp_bound = (lxp_timestamp_bound){1U, 100U};
        activity.idempotency_key[0] = (uint8_t)(i + 1U);
        LXP_CASE_CHECK(lxp_hash_payload(payload, sizeof(payload),
                                        activity.payload_hash) == LXP_OK);
        LXP_CASE_CHECK(sign_activity(&activity, public_key, signature) == 0);
        LXP_CASE_CHECK(lxp_arena_init(&wire_arena, wire_bytes,
                                     sizeof(wire_bytes)) == LXP_OK);
        LXP_CASE_CHECK(lxp_activity_encode(&activity, &wire_arena, &wire) == LXP_OK);
        LXP_CASE_CHECK(lxp_activity_decode(wire.bytes, wire.length,
                                           &received) == LXP_OK);
        LXP_CASE_CHECK(received.activity_type == retired_types[i]);
        LXP_CASE_CHECK(lxp_activity_verify_signature(&received) == LXP_OK);
        LXP_CASE_CHECK(lxp_kernel_execute_activity(&kernel, &received,
            &execution, &receipt) == LXP_ERR_UNKNOWN_ACTIVITY);
        LXP_CASE_CHECK(lxp_arena_mark(&arena) == mark);
        LXP_CASE_CHECK(lxp_state_root(&kernel, after) == LXP_OK);
        LXP_CASE_CHECK(memcmp(before, after, sizeof(before)) == 0);
        LXP_CASE_CHECK(state.next_sequence == 1U && !journal.open);
        LXP_CASE_CHECK(kernel.module_kv_count == 0U);
        LXP_CASE_CHECK(lxp_module_ctx_init(&ctx, &kernel, LXP_MODULE_ASSET,
            10U, execution.epoch, 1U, 1000000U, &arena, true) == LXP_OK);
        LXP_CASE_CHECK(iface->decode(&ctx,
            lxp_activity_type_ordinal(retired_types[i]), payload,
            sizeof(payload), &decoded) == LXP_ERR_UNKNOWN_ACTIVITY);
        LXP_CASE_CHECK(decoded == NULL && ctx.staged_count == 0U);
        lxp_module_ctx_rollback(&ctx);
    }
    LXP_CASE_CHECK(lxp_state_store_destroy(&state) == LXP_OK);
    (void)puts("receive_and_grant_types_refused passed");
    return 0;
}

static int grant_store_symbols_absent(void)
{
    static const char *const retired[] = {
        "lxp_receive_validate", "lxp_receive_execute", "lxp_grant_store_put",
        "lxp_grant_revoke", "lxp_grant_draw_record",
        "lxp_grant_authorization_message", "lx_asset_payer_grant_read",
        "lx_budget_grant_draws_read",
        "lx_asset_payer_grant_encode", "lxp_ctx_asset_grant_state_stage",
        "lxp_payer_grant_encode", "lxp_payer_grant_decode",
        "lxp_receive_encode", "lxp_receive_decode",
        "lxp_receive_authorization_message", "lxp_verify_payer_grant",
        "lx_asset_receive_execute"
    };
    FILE *symbols;
    char line[1024];
    int absent = 1;
    int read_ok;
    int close_status;

    LXP_CASE_CHECK(lxp_receive_validate == NULL);
    LXP_CASE_CHECK(lxp_receive_execute == NULL);
    LXP_CASE_CHECK(lxp_grant_store_put == NULL);
    LXP_CASE_CHECK(lxp_grant_revoke == NULL);
    LXP_CASE_CHECK(lxp_grant_draw_record == NULL);
    LXP_CASE_CHECK(lxp_grant_authorization_message == NULL);
    LXP_CASE_CHECK(lxp_verify_payer_grant == NULL);
    LXP_CASE_CHECK(lx_asset_receive_execute == NULL);
    LXP_CASE_CHECK(lx_asset_payer_grant_read == NULL);
    LXP_CASE_CHECK(lx_budget_grant_draws_read == NULL);
    LXP_CASE_CHECK(lx_asset_payer_grant_encode == NULL);
    LXP_CASE_CHECK(lxp_ctx_asset_grant_state_stage == NULL);
    LXP_CASE_CHECK(lx_asset_module_iface() != NULL);
    symbols = popen("nm -g --defined-only build/liblayerx.a", "r");
    LXP_CASE_CHECK(symbols != NULL);
    while (fgets(line, (int)sizeof(line), symbols) != NULL) {
        size_t length = strlen(line);
        char *name;
        if (length == 0U || line[length - 1U] != '\n') {
            absent = 0;
            continue;
        }
        line[length - 1U] = '\0';
        name = strrchr(line, ' ');
        if (name == NULL) continue;
        ++name;
        for (size_t i = 0U; i < sizeof(retired) / sizeof(retired[0]); ++i) {
            if (strcmp(name, retired[i]) == 0) {
                (void)fprintf(stderr, "archive still defines %s\n", name);
                absent = 0;
            }
        }
    }
    read_ok = !ferror(symbols);
    close_status = pclose(symbols);
    LXP_CASE_CHECK(read_ok && close_status == 0 && absent);
    (void)puts("grant_store_symbols_absent passed");
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        {"receive_and_grant_types_refused", receive_and_grant_types_refused},
        {"grant_store_symbols_absent", grant_store_symbols_absent}
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
