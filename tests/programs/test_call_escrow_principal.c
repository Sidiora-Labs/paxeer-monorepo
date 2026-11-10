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

enum transfer_destination {
    DESTINATION_ACCOUNT,
    DESTINATION_PRINCIPAL,
    DESTINATION_NO_ASSET_ACCOUNT,
    DESTINATION_AMBIGUOUS
};

static int transfer_destination_case(enum transfer_destination destination)
{
    static const uint8_t payer[] = "did:lxp:destination-payer";
    static const uint8_t recipient[] = "did:lxp:destination-recipient";
    static const char *const names[] = {
        "agent:did:lxp:destination-payer:main",
        "agent:did:lxp:destination-recipient:main",
        "agent:did:lxp:destination-recipient:escrow:primary",
        "agent:did:lxp:destination-recipient:asset:"
        "0100000000000000000000000000000000000000000000000000000000000000"
    };
    static const uint64_t balances[] = {10U, 3U, 5U, 7U};
    static uint8_t arena_bytes[LXP_MAX_ACTIVITY_BYTES + 4096U];
    const uint8_t other_asset[32] = {2U};
    lx_account_registry accounts;
    lx_account *opened[4];
    uint8_t ids[4][32];
    uint8_t principal[32];
    const uint8_t *to;
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
    uint64_t parameters = 1U;
    uint64_t token = (uint64_t)(uintptr_t)&call;
    size_t account_count = destination == DESTINATION_AMBIGUOUS ? 4U : 3U;
    size_t paid_index = destination == DESTINATION_ACCOUNT ? 2U : 1U;
    bool settled = destination == DESTINATION_ACCOUNT ||
        destination == DESTINATION_PRINCIPAL;
    lxp_result expected = settled ? LXP_OK :
        destination == DESTINATION_AMBIGUOUS ? LXP_ERR_CONTEXT_MISMATCH :
        LXP_ERR_UNKNOWN_ACCOUNT_NAMESPACE;
    lxp_result status;
    lx_account *resolved = NULL;

    CHECK(lx_account_registry_init(&accounts) == LXP_OK);
    for (size_t i = 0U; i < account_count; ++i) {
        const uint8_t *account_asset =
            destination == DESTINATION_NO_ASSET_ACCOUNT && i == 1U ?
                other_asset : asset.asset_id;
        CHECK(lx_account_id_from_string((const uint8_t *)names[i],
            strlen(names[i]), ids[i]) == LXP_OK);
        CHECK(lx_account_open(&accounts, (const uint8_t *)names[i],
            strlen(names[i]), ids[i], 1U, LX_ACCOUNT_OPEN_GENESIS, NULL,
            &opened[i]) == LXP_OK);
        CHECK(lxp_ledger_bootstrap_balance(opened[i], account_asset,
            (lxp_u128){0U, balances[i]}, i == 0U ? 7U : 0U) == LXP_OK);
    }
    CHECK(lxp_did_id_derive(payer, sizeof(payer) - 1U,
        authority.principal) == LXP_OK);
    CHECK(lxp_did_id_derive(recipient, sizeof(recipient) - 1U,
        principal) == LXP_OK);
    CHECK(account_by_id(&accounts, authority.principal) == NULL);
    CHECK(account_by_id(&accounts, principal) == NULL);
    CHECK(opened[2]->kind == LX_ACCOUNT_AGENT_ESCROW);
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
    call.ctx = &ctx;
    call.authority = &authority;
    call.catalog = &catalog;
    call.catalog_count = 1U;
    call.abi_version = LX_PROGRAMS_ABI_VERSION;
    to = destination == DESTINATION_ACCOUNT ? ids[paid_index] : principal;
    if (destination != DESTINATION_ACCOUNT) {
        CHECK(lxp_kernel_program_payment_account(&accounts, principal,
            asset.asset_id, ctx.protocol_version, &resolved) == expected);
        if (settled) CHECK(resolved == opened[paid_index]);
    }
    {
        uint8_t fee_account[32];
        lxp_result length = layerx_programs_call_destination_name_byte(token,
            read_u64(to), read_u64(to + 8U), read_u64(to + 16U), read_u64(to + 24U),
            read_u64(asset.asset_id), read_u64(asset.asset_id + 8U),
            read_u64(asset.asset_id + 16U), read_u64(asset.asset_id + 24U), UINT32_MAX);
        CHECK(length == (destination == DESTINATION_ACCOUNT ? 0 : settled ?
            (lxp_result)strlen(names[paid_index]) : expected));
        if (destination == DESTINATION_PRINCIPAL) {
            for (uint32_t offset = 0U; offset <= (uint32_t)length; ++offset) {
                CHECK(layerx_programs_call_destination_name_byte(token,
                    read_u64(to), read_u64(to + 8U), read_u64(to + 16U), read_u64(to + 24U),
                    read_u64(asset.asset_id), read_u64(asset.asset_id + 8U),
                    read_u64(asset.asset_id + 16U), read_u64(asset.asset_id + 24U), offset) ==
                    (offset == (uint32_t)length ? LXP_ERR_TRUNCATED :
                        (lxp_result)(uint8_t)names[paid_index][offset]));
            }
        }
        CHECK(web_fee_account_id(asset.asset_id, fee_account) == LXP_OK);
        CHECK(account_by_id(&accounts, fee_account) == NULL);
        for (size_t i = 0U; i < 2U; ++i) {
            CHECK(layerx_programs_call_destination_name_byte(token,
                read_u64(fee_account), read_u64(fee_account + 8U),
                read_u64(fee_account + 16U), read_u64(fee_account + 24U),
                read_u64(asset.asset_id), read_u64(asset.asset_id + 8U),
                read_u64(asset.asset_id + 16U), read_u64(asset.asset_id + 24U),
                i == 0U ? UINT32_MAX : 0U) == (i == 0U ? 0 : LXP_ERR_TRUNCATED));
        }
        CHECK(accounts.count == account_count);
    }
    CHECK(layerx_programs_call_transfer_begin(token, 0U, 1U) == LXP_OK);
    status = layerx_programs_call_transfer_leg(token, 0U,
        PROGRAM_TRANSFER_SOURCE_PRINCIPAL,
        read_u64(authority.principal), read_u64(authority.principal + 8U),
        read_u64(authority.principal + 16U), read_u64(authority.principal + 24U),
        0U, 0U, 0U, 0U,
        read_u64(catalog.program_id), read_u64(catalog.program_id + 8U),
        read_u64(catalog.program_id + 16U), read_u64(catalog.program_id + 24U),
        0U, 0U, 0U,
        read_u64(to), read_u64(to + 8U), read_u64(to + 16U), read_u64(to + 24U),
        read_u64(asset.asset_id), read_u64(asset.asset_id + 8U),
        read_u64(asset.asset_id + 16U), read_u64(asset.asset_id + 24U),
        0U, 4U);
    CHECK(status == expected);
    CHECK(call.transfer_leg_written[0] == (settled ? 1U : 0U));
    CHECK(call.transfer_leg_count == (settled ? 1U : 0U));
    CHECK(!call.transfer_applied && !ctx.transfer_applied);
    CHECK(ctx.transfer_snapshot_count == 0U);
    CHECK(ctx.applied_transfers.count == 0U);
    CHECK(accounts.count == account_count);
    for (size_t i = 0U; i < account_count; ++i)
        CHECK(opened[i]->balance.hi == 0U && opened[i]->balance.lo == balances[i]);
    if (settled) {
        CHECK(call.transfer_set->legs[0].from == opened[0]);
        CHECK(call.transfer_set->legs[0].to == opened[paid_index]);
    }
    CHECK(layerx_programs_call_transfer_apply(token) ==
        (settled ? LXP_OK : LXP_ERR_NON_CANONICAL));
    CHECK(call.transfer_applied == settled && ctx.transfer_applied == settled);
    CHECK(ctx.applied_transfers.count == (settled ? 1U : 0U));
    CHECK(lxp_ct_is_zero(call.transfer_receipt.transfer_set_root, 32U) != settled);
    for (size_t i = 0U; i < account_count; ++i) {
        uint64_t expected_balance = balances[i];
        if (settled && i == 0U) expected_balance -= 4U;
        if (settled && i == paid_index) expected_balance += 4U;
        CHECK(opened[i]->balance.hi == 0U);
        CHECK(opened[i]->balance.lo == expected_balance);
        CHECK(opened[i]->next_sequence == (i == 0U ? (settled ? 8U : 7U) : 0U));
    }
    if (settled) {
        CHECK(memcmp(ctx.applied_transfers.legs[0].from, ids[0], 32U) == 0);
        CHECK(memcmp(ctx.applied_transfers.legs[0].to, ids[paid_index], 32U) == 0);
        CHECK(memcmp(ctx.applied_transfers.legs[0].asset_id, asset.asset_id, 32U) == 0);
        CHECK(ctx.applied_transfers.legs[0].amount.hi == 0U);
        CHECK(ctx.applied_transfers.legs[0].amount.lo == 4U);
    } else {
        CHECK(ctx.transfer_snapshot_count == 0U);
        CHECK(call.transfer_sources[0].kind == 0U);
    }
    lxp_module_ctx_rollback(&ctx);
    lx_account_registry_release(&accounts);
    CHECK(lxp_state_store_destroy(&state) == LXP_OK);
    return 0;
}

int main(void)
{
    int owned = escrow_from_owned_main_account();
    int unknown = escrow_from_unknown_principal_refused();
    int account = transfer_destination_case(DESTINATION_ACCOUNT);
    int principal = transfer_destination_case(DESTINATION_PRINCIPAL);
    int missing = transfer_destination_case(DESTINATION_NO_ASSET_ACCOUNT);
    int ambiguous = transfer_destination_case(DESTINATION_AMBIGUOUS);
    (void)printf("escrow_from_owned_main_account: %s\n", owned ? "FAIL" : "PASS");
    (void)printf("escrow_from_unknown_principal_refused: %s\n",
        unknown ? "FAIL" : "PASS");
    (void)printf("transfer_destination_account: %s\n", account ? "FAIL" : "PASS");
    (void)printf("transfer_destination_principal: %s\n", principal ? "FAIL" : "PASS");
    (void)printf("transfer_destination_no_asset_account: %s\n", missing ? "FAIL" : "PASS");
    (void)printf("transfer_destination_ambiguous: %s\n", ambiguous ? "FAIL" : "PASS");
    return owned || unknown || account || principal || missing || ambiguous;
}
