/*
Proves: tests/test_bridge_withdraw.c:open_ledger/lxp_bridge_withdraw_request -> include/layerx/lxp_bridge.h and lxp_snapshot_write/lxp_snapshot_load -> lx_withdrawal_state_decode/lxp_bridge_withdraw_request.
 * The real bridge caller produces a withdrawal in an in-process kernel,
 * destroys its state store and account registry, reloads the emitted snapshot,
 * and consumes the stored record with an empty withdrawal cache. The consumer
 * observes the same balances, leaf, nullifier and roots and refuses a duplicate.
 * Repairs: src/modules/bridge/lxp_bridge_withdraw.c:lxp_withdrawal_nullifier
 * accepted an empty payout recipient that its leaf and caller refused;
 * src/modules/bridge/lxp_bridge_exit.c:lxp_exit_declare allowed a duplicate
 * declaration to replace the original discard sequence.
 * Repairs tests/test_bridge_withdraw.c:fresh_ctx, which omitted the monetary
 * effect buffer required by lxp_ctx_emit_monetary_transfer_set. The caller now
 * binds that real buffer and uses lxp_kernel_canonical_ledger_apply directly.
 *
 * Existing interface contradiction: tests/codec/lxp_test_bridge_claims_retired.c
 * requires lxp_exit_claim_build and lxp_exit_verify_balance_proof to be absent,
 * and include/layerx/lxp_bridge.h has neither lxp_exit_claim nor
 * lxp_withdrawal_claim. Their guarantor producer and finalization transport are
 * retired. The weak probes assert that current behavior without replacing or
 * resurrecting those layers. This proof exercises the surviving bridge path;
 * it cannot prove successful consumption through the retired claim functions.
 */
#include "../codec/lxp_kernel_case.h"

/* Share the actual listed caller and its real ledger applier, without a double. */
#define LXP_BRIDGE_WITHDRAW_PROOF
#include "../test_bridge_withdraw.c"

typedef struct lxp_exit_claim lxp_exit_claim;
struct lxp_guarantor_cert;
struct lxp_guarantor_key_record;

typedef lxp_result lxp_exit_claim_build_probe(
    const lx_finalized_checkpoint *, const struct lxp_guarantor_cert *,
    const lxp_exit_balance_record *, const lxp_merkle_proof *, lxp_arena *,
    lxp_exit_claim *);
extern lxp_exit_claim_build_probe lxp_exit_claim_build;
typedef lxp_result lxp_exit_verify_balance_proof_probe(
    const lxp_exit_claim *, const struct lxp_guarantor_key_record *, size_t,
    lxp_arena *);
extern lxp_exit_verify_balance_proof_probe lxp_exit_verify_balance_proof;

#pragma weak lxp_exit_claim_build
#pragma weak lxp_exit_verify_balance_proof

static int proof_lxp_bridge_h_happy_path(void)
{
    lxp_exit_state exit_state = {0};
    bool eligible = false;

    LXP_CASE_CHECK(bridge_withdraw_regression(false) == 0);
    exit_state.now_ms = 200U;
    exit_state.last_finalised_at_ms = 100U;
    exit_state.liveness_bound_ms = 100U;
    exit_state.last_finalised_sequence = 7U;
    LXP_CASE_CHECK(lxp_exit_eligibility(&exit_state, &eligible) == LXP_OK);
    LXP_CASE_CHECK(eligible);
    LXP_CASE_CHECK(lxp_exit_declare(&exit_state) == LXP_OK);
    LXP_CASE_CHECK(exit_state.declared);
    LXP_CASE_CHECK(exit_state.discard_after_sequence == 7U);
    exit_state.last_finalised_sequence = 8U;
    LXP_CASE_CHECK(lxp_exit_declare(&exit_state) == LXP_ERR_IDEMPOTENT_REPLAY);
    LXP_CASE_CHECK(exit_state.discard_after_sequence == 7U);
    return 0;
}

static int proof_lxp_bridge_h_refusal(void)
{
    lxp_exit_state exit_state = {0};
    lxp_exit_state before;
    bool eligible = true;

    LXP_CASE_CHECK(bridge_withdraw_regression(true) == 0);
    exit_state.now_ms = 199U;
    exit_state.last_finalised_at_ms = 100U;
    exit_state.liveness_bound_ms = 100U;
    exit_state.last_finalised_sequence = 7U;
    before = exit_state;
    LXP_CASE_CHECK(lxp_exit_eligibility(&exit_state, &eligible) == LXP_OK);
    LXP_CASE_CHECK(!eligible);
    LXP_CASE_CHECK(lxp_exit_declare(&exit_state) == LXP_ERR_DEPOSIT_PROOF_NOT_FINAL);
    LXP_CASE_CHECK(memcmp(&exit_state, &before, sizeof(before)) == 0);
    LXP_CASE_CHECK(lxp_exit_declare(NULL) == LXP_ERR_NON_CANONICAL);
    /* Expected behavior of the retirement test named above: no claim entry. */
    LXP_CASE_CHECK(lxp_exit_claim_build == NULL);
    LXP_CASE_CHECK(lxp_exit_verify_balance_proof == NULL);
    return 0;
}

int main(int argc, char **argv)
{
    static const lxp_kernel_case cases[] = {
        {"proof_lxp_bridge_h_happy_path", proof_lxp_bridge_h_happy_path},
        {"proof_lxp_bridge_h_refusal", proof_lxp_bridge_h_refusal}
    };
    char *all_cases[] = {
        argv[0], "proof_lxp_bridge_h_happy_path", "proof_lxp_bridge_h_refusal"
    };

    if (argc == 1)
        return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                                    3, all_cases);
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]),
                                argc, argv);
}
