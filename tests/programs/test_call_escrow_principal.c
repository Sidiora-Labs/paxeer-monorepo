#include "../../src/modules/programs/call.c"

#include "layerx/lxp_identity.h"

#include <stdio.h>

#define CHECK(value) do { if (!(value)) { \
    (void)fprintf(stderr, "escrow principal check failed at %d\n", __LINE__); \
    return 1; \
} } while (0)

static int escrow_case(bool unknown_principal)
{
    static const uint8_t owner[] = "did:lxp:escrow-owner";
    static const uint8_t unknown[] = "did:lxp:escrow-unknown";
    static const char *const names[] = {
        "agent:did:lxp:escrow-other:main",
        "agent:did:lxp:escrow-owner:escrow:primary",
        "agent:did:lxp:escrow-owner:main"
    };
    static uint8_t arena_bytes[LXP_MAX_ACTIVITY_BYTES + 4096U];
    lx_account_registry accounts;
    lx_account *opened[3];
    uint8_t ids[3][32];
    lxp_transfer_asset_state asset = {{1U}, true, false};
    lx_programs_transfer_runtime runtime;
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_kernel kernel;
    lxp_module_ctx ctx;
    lxp_arena arena;
    lxp_authority_resolved authority = {0};
    lxp_programs_call_activity call = {0};
    lxp_programs_call_catalog_entry catalog = {.program_id = {0x77U}};
    lxp_transfer_leg *leg;
    uint64_t parameters = 1U;
    uint64_t token = (uint64_t)(uintptr_t)&call;
    lxp_result status;
    bool settled = !unknown_principal;

    CHECK(lx_account_registry_init(&accounts) == LXP_OK);
    for (size_t i = 0U; i < 3U; ++i) {
        CHECK(lx_account_id_from_string((const uint8_t *)names[i],
            strlen(names[i]), ids[i]) == LXP_OK);
        CHECK(lx_account_open(&accounts, (const uint8_t *)names[i],
            strlen(names[i]), ids[i], 1U, LX_ACCOUNT_OPEN_GENESIS, NULL,
            &opened[i]) == LXP_OK);
        CHECK(lxp_ledger_bootstrap_balance(opened[i], asset.asset_id,
            (lxp_u128){0U, i == 1U ? 0U : 10U},
            i == 2U ? 7U : 0U) == LXP_OK);
    }
    CHECK(opened[1]->kind == LX_ACCOUNT_AGENT_ESCROW);
    CHECK(opened[2]->kind == LX_ACCOUNT_AGENT_MAIN);
    CHECK(lxp_did_id_derive(unknown_principal ? unknown : owner,
        unknown_principal ? sizeof(unknown) - 1U : sizeof(owner) - 1U,
        authority.principal) == LXP_OK);
    CHECK(account_by_id(&accounts, authority.principal) == NULL);
    (void)memcpy(authority.actor, authority.principal, 32U);
    authority.kind = LXP_AUTHORITY_OWNER;
    runtime = (lx_programs_transfer_runtime){
        .accounts = &accounts,
        .assets = &asset,
        .asset_count = 1U
    };
    CHECK(lxp_state_store_init(&state, 1U) == LXP_OK);
    CHECK(lxp_kernel_create(&kernel, &state, &journal, &parameters, 0U) == LXP_OK);
    CHECK(lxp_kernel_register_module(&kernel, programs_module_registration()) == LXP_OK);
    CHECK(lxp_kernel_set_capabilities(&kernel, NULL,
        lxp_kernel_canonical_ledger_apply) == LXP_OK);
    CHECK(lxp_kernel_bind_module_runtime(&kernel, LXP_MODULE_PROGRAMS,
        &runtime) == LXP_OK);
    CHECK(lxp_arena_init(&arena, arena_bytes, sizeof(arena_bytes)) == LXP_OK);
    CHECK(lxp_module_ctx_init(&ctx, &kernel, LXP_MODULE_PROGRAMS, 10U, 0U,
        7U, 100000U, &arena, true) == LXP_OK);
    ctx.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    ctx.ledger_admission.activity_type = LX_PROGRAMS_SANDBOX;
    CHECK(!ctx.ledger_admission.bound);
    call.ctx = &ctx;
    call.authority = &authority;
    call.catalog = &catalog;
    call.catalog_count = 1U;
    call.abi_version = LX_PROGRAMS_ABI_VERSION;
    CHECK(layerx_programs_call_transfer_begin(token, 0U, 1U) == LXP_OK);
    leg = &call.transfer_set->legs[0];
    leg->from = opened[2];
    leg->to = opened[1];
    (void)memcpy(leg->asset_id, asset.asset_id, 32U);
    leg->amount = (lxp_u128){0U, 4U};
    leg->reason = LXP_REASON_PAYMENT;
    leg->supply_mode = LXP_TRANSFER_CONSERVED;
    call.transfer_sources[0].kind = PROGRAM_TRANSFER_SOURCE_PRINCIPAL;
    (void)memcpy(call.transfer_sources[0].staging_program,
        catalog.program_id, 32U);
    call.transfer_leg_written[0] = 1U;
    call.transfer_leg_count = 1U;

    status = layerx_programs_call_transfer_apply(token);
    if (status != (settled ? LXP_OK : LXP_ERR_UNKNOWN_ACCOUNT_NAMESPACE)) {
        (void)fprintf(stderr, "escrow unknown=%d apply=%d\n",
            (int)unknown_principal, (int)status);
        return 1;
    }
    CHECK(call.transfer_applied == settled);
    CHECK(ctx.transfer_applied == settled);
    CHECK(opened[2]->balance.hi == 0U);
    CHECK(opened[2]->balance.lo == (settled ? 6U : 10U));
    CHECK(opened[2]->next_sequence == (settled ? 8U : 7U));
    CHECK(opened[1]->balance.hi == 0U);
    CHECK(opened[1]->balance.lo == (settled ? 4U : 0U));
    CHECK(opened[1]->next_sequence == 0U);
    CHECK(opened[0]->balance.hi == 0U && opened[0]->balance.lo == 10U);
    CHECK(opened[0]->next_sequence == 0U);
    CHECK(ctx.applied_transfers.count == (settled ? 1U : 0U));
    CHECK(lxp_ct_is_zero(call.transfer_receipt.transfer_set_root, 32U) != settled);
    if (settled) {
        CHECK(call.transfer_set->context.sequence_account == opened[2]);
        CHECK(memcmp(call.transfer_set->context.authorized_from, ids[2], 32U) == 0);
        CHECK(memcmp(ctx.applied_transfers.legs[0].from, ids[2], 32U) == 0);
        CHECK(memcmp(ctx.applied_transfers.legs[0].to, ids[1], 32U) == 0);
        CHECK(ctx.applied_transfers.legs[0].amount.hi == 0U);
        CHECK(ctx.applied_transfers.legs[0].amount.lo == 4U);
    } else {
        CHECK(ctx.transfer_snapshot_count == 0U);
    }
    lxp_module_ctx_rollback(&ctx);
    lx_account_registry_release(&accounts);
    CHECK(lxp_state_store_destroy(&state) == LXP_OK);
    return 0;
}

static int escrow_from_owned_main_account(void)
{
    return escrow_case(false);
}

static int escrow_from_unknown_principal_refused(void)
{
    return escrow_case(true);
}

int main(void)
{
    int owned = escrow_from_owned_main_account();
    int unknown = escrow_from_unknown_principal_refused();
    (void)printf("escrow_from_owned_main_account: %s\n", owned ? "FAIL" : "PASS");
    (void)printf("escrow_from_unknown_principal_refused: %s\n",
        unknown ? "FAIL" : "PASS");
    return owned || unknown;
}
