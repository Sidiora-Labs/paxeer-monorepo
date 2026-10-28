#include "layerx/lxp_bridge.h"

#ifdef LAYERX_LXP_GUARANTOR_H
#define BRIDGE_HEADER_INCLUDES_GUARANTOR 1
#else
#define BRIDGE_HEADER_INCLUDES_GUARANTOR 0
#endif

#include "lxp_kernel_case.h"

#include <stdlib.h>

/* Opaque legacy parameter types keep the weak probes' original signatures. */
typedef struct lxp_withdrawal_claim lxp_withdrawal_claim;
typedef struct lxp_exit_claim lxp_exit_claim;
struct lxp_guarantor_cert;
struct lxp_guarantor_key_record;

extern lxp_result lxp_bridge_withdraw_finalize(
    lxp_module_ctx *, lx_account *, lx_account *, const lx_asset_record *,
    const lx_withdrawal_request *, lx_withdrawal_store *,
    const lxp_withdrawal_claim *, lxp_transfer_context, lxp_receipt *);
extern lxp_result lxp_exit_claim_build(
    const lx_finalized_checkpoint *, const struct lxp_guarantor_cert *,
    const lxp_exit_balance_record *, const lxp_merkle_proof *, lxp_arena *,
    lxp_exit_claim *);
extern lxp_result lxp_exit_verify_balance_proof(
    const lxp_exit_claim *, const struct lxp_guarantor_key_record *, size_t,
    lxp_arena *);
extern lxp_result lx_checkpoint_registry_register_deposit_root(
    lx_checkpoint_registry *, const lx_paxeer_deposit_root_registration *);

#pragma weak lxp_bridge_withdraw_finalize
#pragma weak lxp_exit_claim_build
#pragma weak lxp_exit_verify_balance_proof
#pragma weak lx_checkpoint_registry_register_deposit_root

static int header_lacks(const char *path, const char *const *names, size_t count)
{
    FILE *header = fopen(path, "rb");
    long length;
    char *contents;
    size_t bytes;
    size_t i;
    int read_ok;
    int close_ok;
    int absent = 1;

    LXP_CASE_CHECK(header != NULL);
    if (fseek(header, 0L, SEEK_END) != 0 || (length = ftell(header)) <= 0L ||
        fseek(header, 0L, SEEK_SET) != 0) {
        (void)fclose(header);
        LXP_CASE_CHECK(0);
    }
    bytes = (size_t)length;
    if (bytes == SIZE_MAX) {
        (void)fclose(header);
        LXP_CASE_CHECK(0);
    }
    contents = malloc(bytes + 1U);
    if (contents == NULL) {
        (void)fclose(header);
        LXP_CASE_CHECK(0);
    }
    read_ok = fread(contents, 1U, bytes, header) == bytes && !ferror(header);
    close_ok = fclose(header) == 0;
    contents[bytes] = '\0';
    if (read_ok) {
        for (i = 0U; i < count; ++i) {
            if (strstr(contents, names[i]) != NULL) {
                fprintf(stderr, "%s still declares %s\n", path, names[i]);
                absent = 0;
            }
        }
    }
    free(contents);
    LXP_CASE_CHECK(read_ok);
    LXP_CASE_CHECK(close_ok);
    LXP_CASE_CHECK(absent);
    return 0;
}

static int bridge_header_without_guarantor(void)
{
    static const char *const retired_bridge_names[] = {
        "layerx/lxp_guarantor.h", "lxp_guarantor_",
        "lxp_withdrawal_claim", "lxp_exit_claim",
        "lxp_bridge_withdraw_finalize", "lxp_exit_verify_balance_proof"
    };
    static const char *const retired_asset_names[] = {
        "lx_checkpoint_registry_register_deposit_root"
    };
    static const uint8_t expected_deposit[32] = {
        0xb9, 0x19, 0x76, 0x31, 0x1b, 0x1d, 0xb2, 0xe4,
        0x5c, 0x2a, 0x1a, 0x33, 0xa7, 0x93, 0x6f, 0x0e,
        0xf7, 0x1f, 0xb5, 0x6d, 0xcc, 0xa1, 0x06, 0x21,
        0xc2, 0x1c, 0xdf, 0xed, 0x0c, 0xdf, 0xa8, 0xb0
    };
    static const uint8_t expected_withdrawal[32] = {
        0xd0, 0xad, 0xb1, 0x7b, 0xcb, 0xf6, 0xa2, 0x6d,
        0xe3, 0x2d, 0x94, 0x77, 0x5a, 0x14, 0xb8, 0x0e,
        0x7f, 0xae, 0xb3, 0x7b, 0x84, 0xe4, 0xc2, 0x3e,
        0x5a, 0x72, 0x97, 0x63, 0xda, 0xba, 0x98, 0x98
    };
    lx_deposit_proof deposit = {0};
    lx_withdrawal_request withdrawal = {0};
    uint8_t nullifier[32];
    size_t i;

    LXP_CASE_CHECK(BRIDGE_HEADER_INCLUDES_GUARANTOR == 0);
    LXP_CASE_CHECK(header_lacks("include/layerx/lxp_bridge.h",
        retired_bridge_names,
        sizeof(retired_bridge_names) / sizeof(retired_bridge_names[0])) == 0);
    LXP_CASE_CHECK(header_lacks("include/layerx/lx_asset.h",
        retired_asset_names,
        sizeof(retired_asset_names) / sizeof(retired_asset_names[0])) == 0);

    for (i = 0U; i < 32U; ++i) {
        deposit.deposit_id[i] = (uint8_t)i;
        withdrawal.withdrawal_id[i] = (uint8_t)i;
        withdrawal.account_id[i] = (uint8_t)(i + 32U);
        withdrawal.asset_id[i] = (uint8_t)(i + 64U);
        withdrawal.checkpoint_id[i] = (uint8_t)(i + 96U);
        withdrawal.payout_recipient[i] = (uint8_t)(i + 128U);
    }
    withdrawal.network_id = UINT32_C(0x01020304);
    withdrawal.amount.hi = UINT64_C(0x0102030405060708);
    withdrawal.amount.lo = UINT64_C(0x1112131415161718);
    LXP_CASE_CHECK(lxp_deposit_nullifier(&deposit, nullifier) == LXP_OK);
    LXP_CASE_CHECK(memcmp(nullifier, expected_deposit, sizeof(nullifier)) == 0);
    LXP_CASE_CHECK(lxp_withdrawal_nullifier(&withdrawal, nullifier) == LXP_OK);
    LXP_CASE_CHECK(memcmp(nullifier, expected_withdrawal,
                          sizeof(nullifier)) == 0);
    LXP_CASE_CHECK(lx_withdrawal_nullifier(&withdrawal, nullifier) == LXP_OK);
    LXP_CASE_CHECK(memcmp(nullifier, expected_withdrawal,
                          sizeof(nullifier)) == 0);

    LXP_CASE_CHECK(lxp_deposit_nullifier(NULL, nullifier) ==
                   LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_deposit_nullifier(&deposit, NULL) ==
                   LXP_ERR_NON_CANONICAL);
    (void)memset(deposit.deposit_id, 0, sizeof(deposit.deposit_id));
    LXP_CASE_CHECK(lxp_deposit_nullifier(&deposit, nullifier) ==
                   LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_withdrawal_nullifier(NULL, nullifier) ==
                   LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_withdrawal_nullifier(&withdrawal, NULL) ==
                   LXP_ERR_NON_CANONICAL);
    withdrawal.network_id = 0U;
    LXP_CASE_CHECK(lxp_withdrawal_nullifier(&withdrawal, nullifier) ==
                   LXP_ERR_NON_CANONICAL);
    return 0;
}

static int bridge_claim_symbols_absent(void)
{
    lxp_exit_state state = {0};
    lx_deposit_proof deposit = {0};
    uint8_t nullifier[32];

    /* Strong calls extract every object that used to define a retired symbol. */
    state.now_ms = 2U;
    state.last_finalised_at_ms = 1U;
    state.liveness_bound_ms = 1U;
    state.last_finalised_sequence = 7U;
    LXP_CASE_CHECK(lxp_exit_declare(&state) == LXP_OK);
    LXP_CASE_CHECK(state.declared && state.discard_after_sequence == 7U);
    LXP_CASE_CHECK(lx_withdrawal_nullifier(NULL, nullifier) ==
                   LXP_ERR_NON_CANONICAL);
    deposit.deposit_id[0] = 1U;
    LXP_CASE_CHECK(lxp_deposit_nullifier(&deposit, nullifier) == LXP_OK);

    LXP_CASE_CHECK(lxp_bridge_withdraw_finalize == NULL);
    LXP_CASE_CHECK(lxp_exit_claim_build == NULL);
    LXP_CASE_CHECK(lxp_exit_verify_balance_proof == NULL);
    LXP_CASE_CHECK(lx_checkpoint_registry_register_deposit_root == NULL);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        { "bridge_header_without_guarantor", bridge_header_without_guarantor },
        { "bridge_claim_symbols_absent", bridge_claim_symbols_absent }
    };

    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                                argc, argv);
}
