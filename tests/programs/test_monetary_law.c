#define main monetary_call_fixture_reference_main
#define write_u16 monetary_fixture_write_u16
#define write_u64 monetary_fixture_write_u64
int monetary_call_fixture_reference_main(int argc, char **argv);
#include "test_call_activity.c"
#undef write_u64
#undef write_u16
#undef main

#include "layerx/programs.h"

#include "layerx/lxp_kernel.h"
#include "layerx/lxp_receipt.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_crypto.h"

#include <stdio.h>
#include <string.h>

static size_t charged_fees;
static lxp_u128 last_fee;

static lxp_result apply_transfer_set(lxp_kernel *kernel,
                                     const lxp_transfer_set *set,
                                     lxp_receipt *receipt)
{
    lxp_transfer_set_result result;
    lxp_transfer_context context = set->context;
    lxp_result status;
    (void)kernel;
    status = lxp_apply_transfer_set((lxp_transfer_leg *)set->legs,
                                    set->leg_count, &context, &result);
    if (status == LXP_OK)
        (void)memcpy(receipt->transfer_set_root, result.transfer_set_root, 32U);
    return status;
}

static lxp_result prepare_fee(lxp_kernel *kernel,
                              const lxp_activity *activity,
                              const lxp_authority_resolved *authority,
                              lxp_u128 fee,
                              void **transaction)
{
    (void)kernel;
    (void)activity;
    (void)authority;
    ++charged_fees;
    last_fee = fee;
    *transaction = &last_fee;
    return LXP_OK;
}

static void commit_fee(lxp_kernel *kernel, void *transaction)
{ (void)kernel; (void)transaction; }

static void rollback_fee(lxp_kernel *kernel, void *transaction)
{ (void)kernel; (void)transaction; --charged_fees; }

static void write_u16(uint8_t *bytes, uint16_t value)
{
    bytes[0] = (uint8_t)(value >> 8U);
    bytes[1] = (uint8_t)value;
}

static void write_u64(uint8_t *bytes, uint64_t value)
{
    size_t i;
    for (i = 0U; i < 8U; ++i)
        bytes[i] = (uint8_t)(value >> ((7U - i) * 8U));
}

static void leg(uint8_t *out, const uint8_t from[32], const uint8_t asset[32],
                const uint8_t to[32], uint64_t amount)
{
    (void)memcpy(out, from, 32U);
    (void)memcpy(out + 32U, asset, 32U);
    (void)memcpy(out + 64U, to, 32U);
    write_u64(out + 104U, amount);
}

static int dispatch(lxp_kernel *kernel, lxp_authority_resolved *authority,
                    uint8_t *payload, size_t payload_length,
                    uint64_t account_sequence,
                    lxp_result expected)
{
    uint8_t arena_bytes[8192];
    lxp_arena arena;
    lxp_module_ctx ctx;
    lxp_effect_buffer effects;
    lxp_activity activity;
    const lxp_module_registration *registration;
    lxp_result result = LXP_OK;
    (void)memset(&activity, 0, sizeof(activity));
    activity.activity_type = LX_PROGRAMS_TRANSFER;
    activity.account_sequence = account_sequence;
    activity.payload = (lxp_byte_span){payload, payload_length};
    if (lxp_arena_init(&arena, arena_bytes, sizeof(arena_bytes)) != LXP_OK ||
        lxp_kernel_module_for_activity(kernel, LX_PROGRAMS_TRANSFER, 0U,
                                       &registration) != LXP_OK ||
        lxp_module_ctx_init(&ctx, kernel, LXP_MODULE_PROGRAMS, 10U, 0U, 1U,
                            100000U, &arena, true) != LXP_OK ||
        lxp_effect_buffer_init(&effects) != LXP_OK ||
        lxp_module_ctx_bind_effects(&ctx, &effects) != LXP_OK ||
        lxp_kernel_dispatch(registration, &ctx, &activity, authority,
                            &effects, &result) != LXP_OK ||
        result != expected)
        return 1;
    if (result == LXP_OK &&
        (effects.count != 1U || effects.effects[0].module_id != LXP_MODULE_PROGRAMS ||
         effects.effects[0].event_type != LX_PROGRAMS_EVENT_TRANSFERRED ||
         effects.effects[0].body_length != 32U))
        return 1;
    return 0;
}

static int mixed_source_kernel_law(
    lx_account *principal, lx_account *program_account, lx_account *recipient,
    const lxp_transfer_asset_state *asset)
{
    lxp_transfer_leg legs[2];
    lxp_transfer_context context;
    lxp_transfer_source_authority authorities[2];
    lxp_transfer_set_result result;
    lxp_u128 principal_before = principal->balance;
    lxp_u128 program_before = program_account->balance;
    lxp_u128 recipient_before = recipient->balance;
    uint64_t sequence_before = principal->next_sequence;
    size_t index;
    (void)memset(legs, 0, sizeof(legs));
    for (index = 0U; index < 2U; ++index) {
        legs[index].from = index == 0U ? principal : program_account;
        legs[index].to = recipient;
        (void)memcpy(legs[index].asset_id, asset->asset_id, 32U);
        legs[index].amount = (lxp_u128){0U, index == 0U ? 5U : 7U};
        legs[index].reason = LXP_REASON_PAYMENT;
        legs[index].supply_mode = LXP_TRANSFER_CONSERVED;
        (void)memcpy(authorities[index].authorized_from,
                     legs[index].from->id, 32U);
        authorities[index].debit_authority_kind = LXP_AUTH_OWNER;
        authorities[index].protocol_system_capability = false;
    }
    (void)memset(&context, 0, sizeof(context));
    context.assets = asset;
    context.asset_count = 1U;
    context.actor_sequence = sequence_before;
    context.sequence_account = principal;
    context.debit_authority_kind = LXP_AUTH_OWNER;
    context.source_authorities = authorities;
    context.source_authority_count = 2U;
    if (lxp_apply_transfer_set(legs, 2U, &context, &result) != LXP_OK ||
        result.leg_count != 2U || !result.receipt_emitted ||
        lxp_ct_is_zero(result.transfer_set_root, 32U) ||
        principal->balance.lo != principal_before.lo - 5U ||
        program_account->balance.lo != program_before.lo - 7U ||
        recipient->balance.lo != recipient_before.lo + 12U ||
        principal->next_sequence != sequence_before + 1U ||
        program_account->next_sequence != 0U)
        return 1;

    principal_before = principal->balance;
    program_before = program_account->balance;
    recipient_before = recipient->balance;
    sequence_before = principal->next_sequence;
    context.actor_sequence = sequence_before;
    context.inject_failure = true;
    context.failure_after_leg = 1U;
    if (lxp_apply_transfer_set(legs, 2U, &context, &result) != LXP_ERR_IO ||
        principal->balance.lo != principal_before.lo ||
        program_account->balance.lo != program_before.lo ||
        recipient->balance.lo != recipient_before.lo ||
        principal->next_sequence != sequence_before ||
        program_account->next_sequence != 0U)
        return 1;

    context.inject_failure = false;
    context.source_authority_count = 1U;
    if (lxp_apply_transfer_set(legs, 2U, &context, &result) !=
            LXP_ERR_UNAUTHORIZED_DEBIT ||
        principal->balance.lo != principal_before.lo ||
        program_account->balance.lo != program_before.lo ||
        recipient->balance.lo != recipient_before.lo ||
        principal->next_sequence != sequence_before)
        return 1;
    return 0;
}

static size_t passed_cases;

static int report_case(const char *name, int failed)
{
    if (failed != 0) {
        (void)printf("CASE %s FAILED result=%d\n", name, failed);
        return 1;
    }
    (void)printf("CASE %s ok\n", name);
    ++passed_cases;
    return 0;
}

/* The ledger context a redeemed program-spend permit hands the kernel's
 * canonical applier: the principal leg under its owner root and the
 * program-derived account under the root the kernel rebinds after
 * redemption. */
static void program_set(lxp_transfer_leg legs[2],
                        lxp_transfer_source_authority authorities[2],
                        lxp_transfer_context *context,
                        lx_account *first, lx_account *second,
                        lx_account *recipient,
                        const lxp_transfer_asset_state *asset,
                        uint64_t first_amount, uint64_t second_amount)
{
    size_t index;
    (void)memset(legs, 0, 2U * sizeof(legs[0]));
    (void)memset(authorities, 0, 2U * sizeof(authorities[0]));
    for (index = 0U; index < 2U; ++index) {
        legs[index].from = index == 0U ? first : second;
        legs[index].to = recipient;
        (void)memcpy(legs[index].asset_id, asset->asset_id, 32U);
        legs[index].amount =
            (lxp_u128){0U, index == 0U ? first_amount : second_amount};
        legs[index].reason = LXP_REASON_PAYMENT;
        legs[index].supply_mode = LXP_TRANSFER_CONSERVED;
        (void)memcpy(authorities[index].authorized_from,
                     legs[index].from->id, 32U);
        authorities[index].debit_authority_kind = LXP_AUTH_OWNER;
    }
    (void)memset(context, 0, sizeof(*context));
    context->assets = asset;
    context->asset_count = 1U;
    context->actor_sequence = first->next_sequence;
    context->sequence_account = first;
    context->debit_authority_kind = LXP_AUTH_OWNER;
    context->source_authorities = authorities;
    context->source_authority_count = first == second ? 1U : 2U;
    context->origin_module_id = LXP_MODULE_PROGRAMS;
}

static int program_leg_atomic_rollback_after_first_leg(
    lx_account *principal, lx_account *program_account, lx_account *recipient,
    const lxp_transfer_asset_state *asset)
{
    lxp_transfer_leg legs[2];
    lxp_transfer_source_authority authorities[2];
    lxp_transfer_context context;
    lxp_transfer_set_result result;
    uint64_t principal_before = principal->balance.lo;
    uint64_t program_before = program_account->balance.lo;
    uint64_t recipient_before = recipient->balance.lo;
    uint64_t sequence_before = principal->next_sequence;
    program_set(legs, authorities, &context, principal, program_account,
                recipient, asset, 3U, 4U);
    context.inject_failure = true;
    context.failure_after_leg = 1U;
    if (lxp_apply_transfer_set(legs, 2U, &context, &result) != LXP_ERR_IO ||
        principal->balance.lo != principal_before ||
        program_account->balance.lo != program_before ||
        recipient->balance.lo != recipient_before ||
        principal->next_sequence != sequence_before ||
        program_account->next_sequence != 0U)
        return 1;
    return 0;
}

static int program_leg_insufficient_balance_refusal(
    lx_account *principal, lx_account *program_account, lx_account *recipient,
    const lxp_transfer_asset_state *asset)
{
    lxp_transfer_leg legs[2];
    lxp_transfer_source_authority authorities[2];
    lxp_transfer_context context;
    lxp_transfer_set_result result;
    uint64_t principal_before = principal->balance.lo;
    uint64_t program_before = program_account->balance.lo;
    uint64_t recipient_before = recipient->balance.lo;
    uint64_t sequence_before = principal->next_sequence;
    program_set(legs, authorities, &context, principal, program_account,
                recipient, asset, 1U, program_before + 1U);
    if (lxp_apply_transfer_set(legs, 2U, &context, &result) !=
            LXP_ERR_INSUFFICIENT_BALANCE ||
        principal->balance.lo != principal_before ||
        program_account->balance.lo != program_before ||
        recipient->balance.lo != recipient_before ||
        principal->next_sequence != sequence_before ||
        program_account->next_sequence != 0U)
        return 1;
    return 0;
}

static int program_leg_cumulative_bound_refusal(
    lx_account *principal, lx_account *program_account, lx_account *recipient,
    const lxp_transfer_asset_state *asset)
{
    lxp_transfer_leg legs[2];
    lxp_transfer_source_authority authorities[2];
    lxp_transfer_context context;
    lxp_transfer_set_result result;
    uint64_t principal_before = principal->balance.lo;
    uint64_t program_before = program_account->balance.lo;
    uint64_t recipient_before = recipient->balance.lo;
    uint64_t sequence_before = principal->next_sequence;
    /* Each leg alone fits the program account; together they exceed it. */
    if (program_before < 2U) return 1;
    program_set(legs, authorities, &context, program_account,
                program_account, recipient, asset, program_before - 1U,
                program_before - 1U);
    if (lxp_apply_transfer_set(legs, 2U, &context, &result) !=
            LXP_ERR_INSUFFICIENT_BALANCE ||
        principal->balance.lo != principal_before ||
        program_account->balance.lo != program_before ||
        recipient->balance.lo != recipient_before ||
        principal->next_sequence != sequence_before ||
        program_account->next_sequence != 0U)
        return 1;
    return 0;
}

static int balances_hold(const lx_account *a, uint64_t a_lo,
                         const lx_account *b, uint64_t b_lo,
                         const lx_account *c, uint64_t c_lo)
{
    return a->balance.lo == a_lo && b->balance.lo == b_lo &&
           c->balance.lo == c_lo;
}

static int program_spend_kernel_law(
    lxp_kernel *kernel, lx_account *principal, lx_account *program_account,
    lx_account *recipient, const lxp_transfer_asset_state *asset)
{
    lxp_transfer_leg legs[2];
    lxp_transfer_source_authority authorities[2];
    lxp_transfer_set set;
    lxp_receipt receipt;
    uint64_t principal_before = principal->balance.lo;
    uint64_t program_before = program_account->balance.lo;
    uint64_t recipient_before = recipient->balance.lo;
    uint64_t sequence_before = principal->next_sequence;
    size_t index;
    (void)memset(legs, 0, sizeof(legs));
    (void)memset(authorities, 0, sizeof(authorities));
    for (index = 0U; index < 2U; ++index) {
        legs[index].from = index == 0U ? principal : program_account;
        legs[index].to = recipient;
        (void)memcpy(legs[index].asset_id, asset->asset_id, 32U);
        legs[index].amount = (lxp_u128){0U, index == 0U ? 1U : 2U};
        legs[index].reason = LXP_REASON_PAYMENT;
        legs[index].supply_mode = LXP_TRANSFER_CONSERVED;
        (void)memcpy(authorities[index].authorized_from,
                     legs[index].from->id, 32U);
        authorities[index].debit_authority_kind =
            index == 0U ? LXP_AUTH_OWNER : LXP_AUTH_PROGRAM_SPEND;
    }
    (void)memset(&set, 0, sizeof(set));
    (void)memcpy(set.legs, legs, sizeof(legs));
    set.leg_count = 2U;
    set.context.assets = asset;
    set.context.asset_count = 1U;
    set.context.actor_sequence = sequence_before;
    set.context.sequence_account = principal;
    set.context.debit_authority_kind = LXP_AUTH_OWNER;
    set.context.source_authorities = authorities;
    set.context.source_authority_count = 2U;
    set.context.origin_module_id = LXP_MODULE_PROGRAMS;
    set.context.program_spend_token = 0x5eedU;

    /* A program-authorised leg never reaches a balance writer other than the
     * kernel's canonical ledger primitive. */
    (void)memset(&receipt, 0, sizeof(receipt));
    if (lxp_kernel_apply_transfer_set(kernel, &set, &receipt) !=
            LXP_ERR_BALANCE_BYPASS ||
        !balances_hold(principal, principal_before, program_account,
                       program_before, recipient, recipient_before) ||
        principal->next_sequence != sequence_before)
        return 1;
    if (lxp_kernel_set_capabilities(kernel, NULL,
                                    lxp_kernel_canonical_ledger_apply) != LXP_OK)
        return 1;

    /* The ledger itself refuses a program-spend authority presented without
     * the kernel's redeemed permit. */
    {
        lxp_transfer_set_result result;
        lxp_transfer_context context = set.context;
        if (lxp_apply_transfer_set(legs, 2U, &context, &result) !=
                LXP_ERR_UNAUTHORIZED_DEBIT ||
            !balances_hold(principal, principal_before, program_account,
                           program_before, recipient, recipient_before) ||
            principal->next_sequence != sequence_before)
            return 1;
    }

    /* A forged permit the deriving program never staged is refused for the
     * whole mixed set: the principal leg settles neither. */
    if (lxp_kernel_apply_transfer_set(kernel, &set, &receipt) !=
            LXP_ERR_UNAUTHORIZED_DEBIT ||
        !balances_hold(principal, principal_before, program_account,
                       program_before, recipient, recipient_before) ||
        principal->next_sequence != sequence_before ||
        program_account->next_sequence != 0U)
        return 1;

    /* No permit at all. */
    set.context.program_spend_token = 0U;
    if (lxp_kernel_apply_transfer_set(kernel, &set, &receipt) !=
            LXP_ERR_UNAUTHORIZED_DEBIT ||
        !balances_hold(principal, principal_before, program_account,
                       program_before, recipient, recipient_before))
        return 1;

    /* A program-spend leg staged on behalf of any module other than
     * Programs is refused. */
    set.context.program_spend_token = 0x5eedU;
    set.context.origin_module_id = (uint16_t)(LXP_MODULE_PROGRAMS + 1U);
    if (lxp_kernel_apply_transfer_set(kernel, &set, &receipt) !=
            LXP_ERR_UNAUTHORIZED_DEBIT ||
        !balances_hold(principal, principal_before, program_account,
                       program_before, recipient, recipient_before))
        return 1;
    set.context.origin_module_id = LXP_MODULE_PROGRAMS;

    /* One account cannot be bound to two authorization roots. */
    (void)memcpy(authorities[1].authorized_from, principal->id, 32U);
    if (lxp_kernel_apply_transfer_set(kernel, &set, &receipt) !=
            LXP_ERR_UNAUTHORIZED_DEBIT ||
        !balances_hold(principal, principal_before, program_account,
                       program_before, recipient, recipient_before))
        return 1;
    (void)memcpy(authorities[1].authorized_from, program_account->id, 32U);

    /* A program-spend authority must name a staged source leg. */
    set.leg_count = 1U;
    set.context.source_authority_count = 1U;
    authorities[0].debit_authority_kind = LXP_AUTH_PROGRAM_SPEND;
    (void)memcpy(authorities[0].authorized_from, program_account->id, 32U);
    if (lxp_kernel_apply_transfer_set(kernel, &set, &receipt) !=
            LXP_ERR_UNAUTHORIZED_DEBIT ||
        !balances_hold(principal, principal_before, program_account,
                       program_before, recipient, recipient_before) ||
        principal->next_sequence != sequence_before)
        return 1;
    return lxp_kernel_set_capabilities(kernel, NULL, apply_transfer_set) ==
                   LXP_OK ? 0 : 1;
}

static size_t monetary_mixed_module(uint8_t *out, const uint8_t *seed,
                                   size_t seed_length,
                                   const uint8_t source[32],
                                   const uint8_t asset[32],
                                   const uint8_t destination[32])
{
    static const uint8_t header[] = {0U, 0x61U, 0x73U, 0x6dU, 1U, 0U, 0U, 0U};
    static const uint8_t types[] = {
        4U,
        0x60U, 10U, 0x7eU, 0x7eU, 0x7fU, 0x7fU, 0x7fU, 0x7fU, 0x7fU, 0x7fU,
        0x7fU, 0x7fU, 1U, 0x7fU,
        0x60U, 1U, 0x7fU, 1U, 0x7fU,
        0x60U, 2U, 0x7fU, 0x7fU, 1U, 0x7fU,
        0x60U, 6U, 0x7eU, 0x7eU, 0x7fU, 0x7fU, 0x7fU, 0x7fU, 1U, 0x7fU
    };
    static const uint8_t functions[] = {2U, 1U, 2U};
    static const uint8_t memory[] = {1U, 1U, 1U, 1U};
    uint8_t section[256];
    uint8_t body[64];
    size_t cursor = 0U;
    size_t length = 0U;
    size_t body_length = 0U;
    append_bytes(out, &cursor, header, sizeof(header));
    append_section(out, &cursor, 1U, types, sizeof(types));
    section[length++] = 2U;
    append_name(section, &length, "layerx_v2");
    append_name(section, &length, "transfer_program_402");
    section[length++] = 0U;
    section[length++] = 0U;
    append_name(section, &length, "layerx_v1");
    append_name(section, &length, "transfer_402");
    section[length++] = 0U;
    section[length++] = 3U;
    append_section(out, &cursor, 2U, section, length);
    append_section(out, &cursor, 3U, functions, sizeof(functions));
    append_section(out, &cursor, 5U, memory, sizeof(memory));
    length = 0U;
    section[length++] = 3U;
    append_name(section, &length, "layerx_reserve");
    section[length++] = 0U; section[length++] = 2U;
    append_name(section, &length, "layerx_call");
    section[length++] = 0U; section[length++] = 3U;
    append_name(section, &length, "memory");
    section[length++] = 2U; section[length++] = 0U;
    append_section(out, &cursor, 7U, section, length);
    body[body_length++] = 0U;
    body[body_length++] = 0x42U; body[body_length++] = 0U;
    body[body_length++] = 0x42U; body[body_length++] = 3U;
    body[body_length++] = 0x41U; body[body_length++] = 0xa0U; body[body_length++] = 1U;
    body[body_length++] = 0x41U; body[body_length++] = 32U;
    body[body_length++] = 0x41U; body[body_length++] = 0xc0U; body[body_length++] = 1U;
    body[body_length++] = 0x41U; body[body_length++] = 32U;
    body[body_length++] = 0x10U; body[body_length++] = 1U;
    body[body_length++] = 0x1aU;
    body[body_length++] = 0x42U; body[body_length++] = 0U;
    body[body_length++] = 0x42U;
    body[body_length++] = (uint8_t)PROGRAM_SPEND_AMOUNT;
    body[body_length++] = 0x41U; body[body_length++] = 0U;
    body[body_length++] = 0x41U; body[body_length++] = (uint8_t)seed_length;
    body[body_length++] = 0x41U; body[body_length++] = 0x80U;
    body[body_length++] = 1U;
    body[body_length++] = 0x41U; body[body_length++] = 32U;
    body[body_length++] = 0x41U; body[body_length++] = 0xa0U;
    body[body_length++] = 1U;
    body[body_length++] = 0x41U; body[body_length++] = 32U;
    body[body_length++] = 0x41U; body[body_length++] = 0xc0U;
    body[body_length++] = 1U;
    body[body_length++] = 0x41U; body[body_length++] = 32U;
    body[body_length++] = 0x10U; body[body_length++] = 0U;
    body[body_length++] = 0x0bU;
    length = 0U;
    section[length++] = 2U;
    section[length++] = 4U;
    section[length++] = 0U; section[length++] = 0x41U;
    section[length++] = 0U; section[length++] = 0x0bU;
    append_u32_leb(section, &length, (uint32_t)body_length);
    append_bytes(section, &length, body, body_length);
    append_section(out, &cursor, 10U, section, length);
    length = 0U;
    section[length++] = 4U;
    section[length++] = 0U;
    section[length++] = 0x41U; section[length++] = 0U;
    section[length++] = 0x0bU;
    append_u32_leb(section, &length, (uint32_t)seed_length);
    append_bytes(section, &length, seed, seed_length);
    section[length++] = 0U;
    section[length++] = 0x41U; section[length++] = 0x80U;
    section[length++] = 1U; section[length++] = 0x0bU;
    section[length++] = 32U;
    append_bytes(section, &length, source, 32U);
    section[length++] = 0U;
    section[length++] = 0x41U; section[length++] = 0xa0U;
    section[length++] = 1U; section[length++] = 0x0bU;
    section[length++] = 32U;
    append_bytes(section, &length, asset, 32U);
    section[length++] = 0U;
    section[length++] = 0x41U; section[length++] = 0xc0U;
    section[length++] = 1U; section[length++] = 0x0bU;
    section[length++] = 32U;
    append_bytes(section, &length, destination, 32U);
    append_section(out, &cursor, 11U, section, length);
    return cursor;
}

static size_t monetary_mixed_capabilities(
    uint8_t *out, const uint8_t owner[32], const uint8_t *seed, size_t seed_length,
    const uint8_t source[32], const uint8_t asset[32], const uint8_t to[32],
    uint64_t maximum_amount)
{
    size_t length = program_spend_capabilities(out + 81U, owner, seed,
        seed_length, source, asset, to, maximum_amount);
    write_u16(out, 2U);
    out[2] = 5U;
    (void)memcpy(out + 3U, asset, 32U);
    (void)memcpy(out + 35U, to, 32U);
    (void)memset(out + 67U, 0, 16U);
    write_u64(out + 75U, 3U);
    return length + 81U;
}

static int monetary_exact_fee(lxp_u128 actor_before, lxp_u128 treasury_before,
    const lx_account *actor, const lx_account *treasury, lxp_u128 fee,
    uint64_t principal_amount)
{
    lxp_u128 principal_debit = {0U, principal_amount};
    lxp_u128 after_principal;
    return lxp_u128_sub(actor_before, principal_debit, &after_principal) == LXP_OK &&
        exact_fee_applied(after_principal, treasury_before, actor, treasury, fee) == 0 ? 0 : 1;
}

static int monetary_failure_detail(const lxp_receipt *receipt,
                                   lxp_byte_span *detail)
{
    static const uint8_t domain[] = "LXP/programs/terminal-applied-legs/v1";
    static const uint8_t no_applied_legs[4] = {0};
    const lxp_program_outcome *outcome = &receipt->program_outcome;
    const lxp_byte_span terminal = outcome->terminal_payload;
    const size_t prefix = sizeof(domain) + 4U;
    uint32_t length;
    if (receipt->result_code != LXP_ERR_PROGRAM_REFUSED || !outcome->present ||
        outcome->terminal_kind != LXP_PROGRAM_TERMINAL_FAILURE ||
        outcome->result_code != LXP_ERR_PROGRAM_REFUSED ||
        outcome->abi_version != LX_PROGRAMS_GUEST_ABI_V5_VERSION ||
        terminal.bytes == NULL || terminal.length < prefix + 4U ||
        memcmp(terminal.bytes, domain, sizeof(domain)) != 0)
        return 1;
    length = ((uint32_t)terminal.bytes[sizeof(domain)] << 24U) |
             ((uint32_t)terminal.bytes[sizeof(domain) + 1U] << 16U) |
             ((uint32_t)terminal.bytes[sizeof(domain) + 2U] << 8U) |
             terminal.bytes[sizeof(domain) + 3U];
    if ((size_t)length != terminal.length - prefix - 4U ||
        memcmp(terminal.bytes + prefix + length, no_applied_legs,
               sizeof(no_applied_legs)) != 0)
        return 1;
    *detail = (lxp_byte_span){terminal.bytes + prefix, length};
    return 0;
}

static int monetary_authority_refusal(const lxp_receipt *receipt,
                                     const uint8_t program[32],
                                     const uint8_t principal[32])
{
    static const uint8_t execution_domain[] = "LXP/program-execution/v5";
    static const uint8_t graph_domain[] = "LayerX/programs/call-graph/v1";
    static const uint8_t reason[] = "LXP/programs/authority-refusal/v1\0\x05";
    const lxp_byte_span graph = receipt->program_outcome.call_graph_payload;
    lxp_byte_span detail;
    uint8_t expected[32U + 2U + 1U + 8U + 32U + 4U + 4U +
                     sizeof(reason) - 1U + 8U];
    const size_t failure_length = 32U + 4U + 4U + sizeof(reason) - 1U;
    size_t cursor = 0U;
    if (monetary_failure_detail(receipt, &detail) != 0 ||
        graph.bytes == NULL || graph.length < sizeof(graph_domain) + 32U ||
        memcmp(graph.bytes, graph_domain, sizeof(graph_domain)) != 0 ||
        memcmp(graph.bytes + sizeof(graph_domain), principal, 32U) != 0 ||
        detail.length < sizeof(execution_domain) + sizeof(expected) + graph.length ||
        memcmp(detail.bytes, execution_domain, sizeof(execution_domain)) != 0)
        return 1;
    (void)memcpy(expected + cursor, program, 32U);
    cursor += 32U;
    write_u16(expected + cursor, LX_PROGRAMS_GUEST_ABI_V5_VERSION);
    cursor += 2U;
    expected[cursor++] = 1U;
    write_u64(expected + cursor, failure_length);
    cursor += 8U;
    (void)memcpy(expected + cursor, program, 32U);
    cursor += 32U;
    write_u32(expected + cursor, 3U);
    cursor += 4U;
    write_u32(expected + cursor, (uint32_t)(sizeof(reason) - 1U));
    cursor += 4U;
    (void)memcpy(expected + cursor, reason, sizeof(reason) - 1U);
    cursor += sizeof(reason) - 1U;
    write_u64(expected + cursor, graph.length);
    cursor += 8U;
    return cursor != sizeof(expected) ||
        memcmp(detail.bytes + detail.length - graph.length - sizeof(expected),
               expected, sizeof(expected)) != 0 ||
        memcmp(detail.bytes + detail.length - graph.length,
               graph.bytes, graph.length) != 0 ? 1 : 0;
}

static int monetary_kernel_refusal(const lxp_receipt *receipt)
{
    static const uint8_t domain[] = "LXP/programs/settlement-failure/v1";
    lxp_byte_span detail;
    return monetary_failure_detail(receipt, &detail) != 0 ||
        detail.length != sizeof(domain) + 1U ||
        memcmp(detail.bytes, domain, sizeof(domain)) != 0 ||
        detail.bytes[sizeof(domain)] != 7U ? 1 : 0;
}

static int monetary_real_failure(int line, lxp_result dispatch_result,
                                 const lxp_receipt *receipt)
{
    const lxp_program_outcome *outcome = &receipt->program_outcome;
    (void)fprintf(stderr,
                  "Real mixed program transfer failed line=%d dispatch=%d receipt=%d terminal=%u abi=%u detail_bytes=%zu detail=",
                  line, (int)dispatch_result, (int)receipt->result_code,
                  (unsigned)outcome->terminal_kind, (unsigned)outcome->abi_version,
                  outcome->terminal_payload.length);
    for (size_t index = 0U; index < outcome->terminal_payload.length; ++index)
        (void)fprintf(stderr, "%02x",
                      (unsigned)outcome->terminal_payload.bytes[index]);
    (void)fputc('\n', stderr);
    return 1;
}

static int monetary_fixture_step(const char *step, lxp_result status)
{
    if (status == LXP_OK) return 0;
    (void)fprintf(stderr, "Real mixed program fixture step=%s status=%d\n",
                  step, (int)status);
    return 1;
}

static int real_mixed_program_authority_kernel_case(void)
{
    static const uint8_t did[] = "did:lxp:program-spend";
    static const uint8_t actor_name[] = "agent:did:lxp:program-spend:main";
    static const uint8_t treasury_name[] = "system:fees";
    static const uint8_t payee_name[] = "agent:program-spend-payee:main";
    static const uint8_t spend_seed[PROGRAM_SPEND_SEED_BYTES] = {
        'v', 'a', 'u', 'l', 't', '/', 'p', 'r', 'i', 'm', 'a', 'r', 'y'
    };
    static const uint8_t actor_seed[32] = {0x33U};
    static const uint8_t grant_id[32] = {0};
    lxp_authority_scope spend_scope = {0};
    uint8_t program_id[32];
    uint8_t primary_key[32] = {0};
    uint8_t actor_id[32];
    uint8_t treasury_id[32];
    uint8_t payee_id[32];
    uint8_t fee_asset[32] = {9U};
    uint8_t source_id[32];
    uint8_t code_hash[32];
    uint8_t account_profile[33];
    uint16_t recorded_abi = 0U;
    uint8_t wasm[PROGRAM_SPEND_WASM_BYTES];
    uint8_t capabilities[PROGRAM_SPEND_CAPABILITY_BYTES + 81U];
    static uint8_t payload[106U + PROGRAM_SPEND_WASM_BYTES];
    static uint8_t call[CALL_FIXED_BYTES + 64U +
                        PROGRAM_SPEND_CAPABILITY_BYTES + 81U + 64U];
    static uint8_t arena_bytes[2U * LXP_MAX_ACTIVITY_BYTES + 4096U];
    static uint8_t register_arena_bytes[65536];
    lxp_arena arena;
    lxp_arena register_arena;
    lxp_state_store state;
    lxp_state_journal journal;
    static lxp_kernel kernel;
    lxp_module_ctx register_ctx;
    lxp_effect_buffer register_effects;
    lxp_identity_store identities = {0};
    lxp_identity *identity;
    lxp_authority_resolved authority;
    lxp_kernel_execution execution;
    lxp_fee_params fees = {0};
    lx_account_registry accounts;
    lx_account *actor;
    lx_account *treasury;
    lx_account *payee;
    lx_account *source;
    lxp_transfer_asset_state fee_asset_state;
    lx_programs_transfer_runtime runtime;
    lxp_activity activity;
    lxp_receipt receipt = {0};
    uint64_t parameters = 1U;
    uint64_t identity_sequence = 0U;
    bool created = false;
    size_t payload_length;
    size_t wasm_length;
    lxp_u128 source_before;
    lxp_u128 payee_before;
    lxp_u128 actor_before;
    lxp_u128 treasury_before;
    uint64_t actor_sequence_before;
    lxp_result call_result = LXP_OK;
    (void)memset(program_id, 0x37, sizeof(program_id));
    (void)memset(&authority, 0, sizeof(authority));
    if (executed_public_key(actor_seed, primary_key) != 0)
        return monetary_real_failure(__LINE__, call_result, &receipt);
    if (lx_account_registry_init(&accounts) != LXP_OK ||
        lx_account_id_from_string(actor_name, sizeof(actor_name) - 1U,
                                  actor_id) != LXP_OK ||
        lx_account_id_from_string(treasury_name, sizeof(treasury_name) - 1U,
                                  treasury_id) != LXP_OK ||
        lx_account_id_from_string(payee_name, sizeof(payee_name) - 1U,
                                  payee_id) != LXP_OK ||
        lx_account_open(&accounts, actor_name, sizeof(actor_name) - 1U,
                        actor_id, 1U, LX_ACCOUNT_OPEN_GENESIS, NULL,
                        &actor) != LXP_OK ||
        lx_account_open(&accounts, treasury_name, sizeof(treasury_name) - 1U,
                        treasury_id, 2U, LX_ACCOUNT_OPEN_GENESIS, NULL,
                        &treasury) != LXP_OK ||
        lx_account_open(&accounts, payee_name, sizeof(payee_name) - 1U,
                        payee_id, 1U, LX_ACCOUNT_OPEN_GENESIS, NULL,
                        &payee) != LXP_OK ||
        lxp_ledger_bootstrap_balance(actor, fee_asset,
                                     (lxp_u128){0U, UINT64_MAX}, 1U) != LXP_OK ||
        lxp_ledger_bootstrap_balance(treasury, fee_asset,
                                     (lxp_u128){0U, 0U}, 0U) != LXP_OK ||
        lxp_ledger_bootstrap_balance(payee, fee_asset,
                                     (lxp_u128){0U, 0U}, 0U) != LXP_OK ||
        lxp_programs_account_derive(program_id, spend_seed,
                                    sizeof(spend_seed), source_id) != LXP_OK)
        return monetary_real_failure(__LINE__, call_result, &receipt);
    if (lxp_did_id_derive(did, sizeof(did) - 1U, authority.principal) != LXP_OK)
        return monetary_real_failure(__LINE__, call_result, &receipt);
    (void)memcpy(authority.actor, authority.principal, 32U);
    (void)memcpy(authority.verified_key, primary_key, 32U);
    authority.kind = LXP_AUTHORITY_OWNER;
    spend_scope.module_mask = UINT64_C(1) << LXP_MODULE_PROGRAMS;
    spend_scope.activity_ordinal_min = 1U;
    spend_scope.activity_ordinal_max = 7U;
    spend_scope.maximum_per_activity = (lxp_u128){UINT64_MAX, UINT64_MAX};
    spend_scope.maximum_total = spend_scope.maximum_per_activity;
    spend_scope.maximum_per_period = spend_scope.maximum_per_activity;
    authority.scope = &spend_scope;
    if (lxp_authority_hash(authority.kind, grant_id, primary_key,
                           authority.authority_hash) != LXP_OK)
        return monetary_real_failure(__LINE__, call_result, &receipt);
    (void)memset(&fee_asset_state, 0, sizeof(fee_asset_state));
    (void)memcpy(fee_asset_state.asset_id, fee_asset, sizeof(fee_asset));
    fee_asset_state.registered = true;
    (void)memset(&runtime, 0, sizeof(runtime));
    runtime.accounts = &accounts;
    runtime.assets = &fee_asset_state;
    runtime.asset_count = 1U;
    runtime.fee_schedule = (lx_programs_fee_schedule){
        1U, 1U, 1U, 2U, 4U, 1U, 1U, 1U
    };
    runtime.resolve_metering_schedule = lxp_programs_metering_resolve_runtime;
    runtime.metering_schedule_context = &kernel;
    (void)memcpy(runtime.occupancy_asset_id, fee_asset, 32U);
    runtime.resolve_occupancy_parameters = occupancy_parameters;
    runtime.occupancy_parameter_context = &runtime;
    wasm_length = monetary_mixed_module(wasm, spend_seed, sizeof(spend_seed),
                                       source_id, fee_asset, payee_id);
    payload_length = program_spend_deploy_payload(
        payload, program_id, authority.principal, wasm, wasm_length,
        code_hash);
    fees.version = 1U;
    fees.multiplier_basis_points = 10000U;
    if (lxp_state_store_init(&state, 1U) != LXP_OK ||
        lxp_identity_register(&identities, did, sizeof(did) - 1U,
                              primary_key, &identity) != LXP_OK ||
        lxp_kernel_create(&kernel, &state, &journal, &parameters, 0U) != LXP_OK ||
        install_metering_v1(&kernel) != LXP_OK ||
        lxp_kernel_register_module(&kernel,
                                   programs_module_registration_v5()) != LXP_OK ||
        lxp_kernel_bind_module_runtime(&kernel, LXP_MODULE_PROGRAMS,
                                       &runtime) != LXP_OK ||
        lxp_programs_bind_fee_transaction(&kernel) != LXP_OK ||
        lxp_kernel_set_capabilities(&kernel, NULL,
                                    lxp_kernel_canonical_ledger_apply) != LXP_OK ||
        lxp_state_root(&kernel, kernel.current_state_root) != LXP_OK ||
        lxp_arena_init(&arena, arena_bytes, sizeof(arena_bytes)) != LXP_OK)
        return monetary_real_failure(__LINE__, call_result, &receipt);
    (void)memset(&execution, 0, sizeof(execution));
    execution.network_id = 7U;
    execution.batch_number = 1U;
    execution.batch_timestamp_ms = 10U;
    execution.maximum_timestamp_window = 100U;
    execution.recorded_module_version = LX_PROGRAMS_GUEST_ABI_V5_VERSION;
    execution.parameter_version = 1U;
    execution.signature_valid = true;
    execution.binding_authority = true;
    execution.identities = &identities;
    execution.authority = &authority;
    execution.fee_parameters = &fees;
    execution.gas_limit = 1000000U;
    execution.arena = &arena;
    fill_activity(&activity, LX_PROGRAMS_DEPLOY, payload, payload_length,
                  did, sizeof(did) - 1U, primary_key);
    activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    activity.account_sequence = identity_sequence;
    activity.idempotency_key[31] = 0x31U;
    execution.global_sequence = state.next_sequence;
    if ((call_result = execute_artifact_fixture_activity(&kernel, &activity, &execution,
                                          &receipt)) != LXP_OK ||
        receipt.result_code != LXP_OK ||
        identity->next_sequence != ++identity_sequence)
        return monetary_real_failure(__LINE__, call_result, &receipt);

    (void)memset(payload, 0, 73U);
    (void)memcpy(payload, program_id, 32U);
    (void)memcpy(payload + 32U, "LXPA2", 5U);
    (void)memcpy(payload + 37U, fee_asset, 32U);
    fill_activity(&activity, LX_PROGRAMS_ACCOUNT, payload, 73U,
                  did, sizeof(did) - 1U, primary_key);
    activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    activity.account_sequence = identity_sequence;
    activity.idempotency_key[31] = 0x35U;
    execution.fee_balance = actor->balance;
    execution.global_sequence = state.next_sequence;
    if (monetary_fixture_step("profile arena reset", lxp_arena_reset(&arena, 0U)) ||
        monetary_fixture_step("profile execution",
            (call_result = execute_artifact_fixture_activity(
                &kernel, &activity, &execution, &receipt))) ||
        monetary_fixture_step("profile receipt", receipt.result_code) ||
        identity->next_sequence != ++identity_sequence)
        return monetary_real_failure(__LINE__, call_result, &receipt);

    (void)memset(payload, 0, 106U);
    (void)memcpy(payload, program_id, 32U);
    write_u16(payload + 32U, LX_PROGRAMS_GUEST_ABI_V5_VERSION);
    (void)memcpy(payload + 36U, code_hash, 32U);
    (void)memcpy(payload + 68U, code_hash, 32U);
    write_u32(payload + 102U, (uint32_t)wasm_length);
    (void)memcpy(payload + 106U, wasm, wasm_length);
    fill_activity(&activity, LX_PROGRAMS_UPGRADE, payload, 106U + wasm_length,
                  did, sizeof(did) - 1U, primary_key);
    activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    activity.account_sequence = identity_sequence;
    activity.idempotency_key[31] = 0x36U;
    execution.fee_balance = actor->balance;
    execution.global_sequence = state.next_sequence;
    if (monetary_fixture_step("upgrade arena reset", lxp_arena_reset(&arena, 0U)) ||
        monetary_fixture_step("ABI 5 upgrade execution",
            (call_result = execute_artifact_fixture_activity(
                &kernel, &activity, &execution, &receipt))) ||
        monetary_fixture_step("ABI 5 upgrade receipt", receipt.result_code) ||
        identity->next_sequence != ++identity_sequence)
        return monetary_real_failure(__LINE__, call_result, &receipt);

    if (monetary_fixture_step("registration journal open",
            lxp_state_journal_open(&state, state.next_sequence, &journal)) ||
        monetary_fixture_step("registration arena init",
            lxp_arena_init(&register_arena, register_arena_bytes,
                          sizeof(register_arena_bytes))) ||
        monetary_fixture_step("registration context init",
            lxp_module_ctx_init(&register_ctx, &kernel, LXP_MODULE_PROGRAMS, 10U,
                            0U, state.next_sequence, 100000U, &register_arena,
                            true)))
        return monetary_real_failure(__LINE__, call_result, &receipt);
    register_ctx.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    if (monetary_fixture_step("registered program ABI",
            lxp_programs_program_abi(&register_ctx, program_id, &recorded_abi)) ||
        recorded_abi != LX_PROGRAMS_GUEST_ABI_V5_VERSION ||
        monetary_fixture_step("registered program profile",
            lxp_programs_account_profile_read(&register_ctx, program_id,
                                             account_profile)) ||
        account_profile[0] != 2U ||
        memcmp(account_profile + 1U, authority.principal, 32U) != 0 ||
        monetary_fixture_step("registration effects init",
            lxp_effect_buffer_init(&register_effects)) ||
        monetary_fixture_step("registration effects bind",
            lxp_module_ctx_bind_effects(&register_ctx, &register_effects)) ||
        monetary_fixture_step("program account register",
            lxp_programs_account_register(&register_ctx, program_id, spend_seed,
                                      sizeof(spend_seed), fee_asset, &source,
                                      &created)) ||
        !created || source == NULL || register_effects.count != 1U ||
        register_effects.effects[0].event_type !=
            LX_PROGRAMS_EVENT_ACCOUNT_REGISTERED ||
        monetary_fixture_step("registration prepare commit",
            lxp_module_ctx_prepare_commit(&register_ctx)) ||
        monetary_fixture_step("registration journal commit",
            lxp_state_journal_commit(&journal)) ||
        monetary_fixture_step("registration context commit",
            lxp_module_ctx_commit(&register_ctx)))
        return monetary_real_failure(__LINE__, call_result, &receipt);
    source = program_spend_account(&accounts, source_id);
    if (source == NULL || source->kind != LX_ACCOUNT_MODULE_VALUE ||
        lxp_ledger_bootstrap_balance(
            source, fee_asset,
            (lxp_u128){0U, PROGRAM_SPEND_SOURCE_BALANCE}, 0U) != LXP_OK ||
        lxp_state_root(&kernel, kernel.current_state_root) != LXP_OK)
        return monetary_real_failure(__LINE__, call_result, &receipt);
    payload_length = call_payload_with_capabilities(
        call, program_id, capabilities,
        monetary_mixed_capabilities(capabilities, program_id, spend_seed,
                                   sizeof(spend_seed), source_id, fee_asset,
                                   payee_id, PROGRAM_SPEND_GRANT_CEILING));
    write_u16(call + 32U, LX_PROGRAMS_GUEST_ABI_V5_VERSION);
    fill_activity(&activity, LX_PROGRAMS_CALL, call, payload_length,
                  did, sizeof(did) - 1U, primary_key);
    activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    activity.account_sequence = identity_sequence;
    activity.idempotency_key[31] = 0x33U;
    activity.fee_limit = (lxp_u128){0U, 67108864U};
    execution.fee_balance = actor->balance;
    execution.global_sequence = state.next_sequence;
    source_before = source->balance;
    payee_before = payee->balance;
    actor_before = actor->balance;
    actor_sequence_before = actor->next_sequence;
    treasury_before = treasury->balance;
    if (lxp_arena_reset(&arena, 0U) != LXP_OK ||
        (call_result = execute_artifact_fixture_activity(&kernel, &activity, &execution,
                                          &receipt)) != LXP_OK ||
        receipt.result_code != LXP_OK ||
        monetary_exact_fee(actor_before, treasury_before, actor, treasury, receipt.fee_charged, 3U) != 0 ||
        identity->next_sequence != ++identity_sequence ||
        source->balance.hi != source_before.hi ||
        source->balance.lo != source_before.lo - PROGRAM_SPEND_AMOUNT ||
        payee->balance.hi != payee_before.hi ||
        payee->balance.lo != payee_before.lo + PROGRAM_SPEND_AMOUNT + 3U ||
        lxp_ct_is_zero(receipt.program_outcome.transfer_root, 32U) ||
        memcmp(receipt.transfer_set_root,
               receipt.program_outcome.transfer_root, 32U) != 0)
        return monetary_real_failure(__LINE__, call_result, &receipt);
    source_before = source->balance;
    payee_before = payee->balance;
    actor_before = actor->balance;
    actor_sequence_before = actor->next_sequence;
    treasury_before = treasury->balance;
    {
        write_u16(capabilities, 1U);
        payload_length = call_payload_with_capabilities(
            call, program_id, capabilities, 83U);
    }
    write_u16(call + 32U, LX_PROGRAMS_GUEST_ABI_V5_VERSION);
    fill_activity(&activity, LX_PROGRAMS_CALL, call, payload_length,
                  did, sizeof(did) - 1U, primary_key);
    activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    activity.account_sequence = identity_sequence;
    activity.idempotency_key[31] = 0x34U;
    activity.fee_limit = (lxp_u128){0U, 67108864U};
    execution.fee_balance = actor->balance;
    execution.global_sequence = state.next_sequence;
    if (lxp_arena_reset(&arena, 0U) != LXP_OK ||
        (call_result = execute_artifact_fixture_activity(&kernel, &activity, &execution,
                                          &receipt)) != LXP_OK ||
        receipt.result_code != LXP_ERR_PROGRAM_REFUSED ||
        actor->next_sequence != actor_sequence_before ||
        monetary_exact_fee(actor_before, treasury_before, actor, treasury, receipt.fee_charged, 0U) != 0 ||
        !receipt.program_outcome.present ||
        receipt.program_outcome.terminal_kind != LXP_PROGRAM_TERMINAL_FAILURE ||
        receipt.program_outcome.result_code != LXP_ERR_PROGRAM_REFUSED ||
        monetary_authority_refusal(&receipt, program_id, authority.principal) != 0 ||
        !lxp_ct_is_zero(receipt.program_outcome.transfer_root, 32U) ||
        identity->next_sequence != ++identity_sequence ||
        lxp_u128_cmp(source->balance, source_before) != 0 ||
        lxp_u128_cmp(payee->balance, payee_before) != 0)
        return monetary_real_failure(__LINE__, call_result, &receipt);
    payload_length = call_payload_with_capabilities(
        call, program_id, capabilities,
        monetary_mixed_capabilities(capabilities, program_id, spend_seed,
                                   sizeof(spend_seed), source_id, fee_asset,
                                   payee_id, PROGRAM_SPEND_AMOUNT - 1U));
    write_u16(call + 32U, LX_PROGRAMS_GUEST_ABI_V5_VERSION);
    fill_activity(&activity, LX_PROGRAMS_CALL, call, payload_length,
                  did, sizeof(did) - 1U, primary_key);
    activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    activity.account_sequence = identity_sequence;
    activity.idempotency_key[31] = 0x35U;
    actor_before = actor->balance;
    actor_sequence_before = actor->next_sequence;
    treasury_before = treasury->balance;
    activity.fee_limit = (lxp_u128){0U, 67108864U};
    execution.fee_balance = actor->balance;
    execution.global_sequence = state.next_sequence;
    if (lxp_arena_reset(&arena, 0U) != LXP_OK ||
        (call_result = execute_artifact_fixture_activity(&kernel, &activity, &execution,
                                          &receipt)) != LXP_OK ||
        receipt.result_code != LXP_ERR_PROGRAM_REFUSED ||
        actor->next_sequence != actor_sequence_before ||
        monetary_exact_fee(actor_before, treasury_before, actor, treasury, receipt.fee_charged, 0U) != 0 ||
        !receipt.program_outcome.present ||
        receipt.program_outcome.terminal_kind != LXP_PROGRAM_TERMINAL_FAILURE ||
        receipt.program_outcome.result_code != LXP_ERR_PROGRAM_REFUSED ||
        monetary_authority_refusal(&receipt, program_id, authority.principal) != 0 ||
        !lxp_ct_is_zero(receipt.program_outcome.transfer_root, 32U) ||
        identity->next_sequence != ++identity_sequence ||
        lxp_u128_cmp(source->balance, source_before) != 0 ||
        lxp_u128_cmp(payee->balance, payee_before) != 0)
        return monetary_real_failure(__LINE__, call_result, &receipt);
    payload_length = call_payload_with_capabilities(
        call, program_id, capabilities,
        monetary_mixed_capabilities(capabilities, program_id, spend_seed,
            sizeof(spend_seed), source_id, fee_asset, payee_id, PROGRAM_SPEND_GRANT_CEILING));
    write_u16(call + 32U, LX_PROGRAMS_GUEST_ABI_V5_VERSION);
    while (source->balance.lo >= PROGRAM_SPEND_AMOUNT) {
        fill_activity(&activity, LX_PROGRAMS_CALL, call, payload_length,
                      did, sizeof(did) - 1U, primary_key);
        activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
        activity.account_sequence = identity_sequence;
        activity.idempotency_key[31] = (uint8_t)(0x40U + identity_sequence);
        activity.fee_limit = (lxp_u128){0U, 67108864U};
        execution.fee_balance = actor->balance;
        execution.global_sequence = state.next_sequence;
        source_before = source->balance;
        payee_before = payee->balance;
        actor_before = actor->balance;
        actor_sequence_before = actor->next_sequence;
        treasury_before = treasury->balance;
        if (lxp_arena_reset(&arena, 0U) != LXP_OK ||
            (call_result = execute_artifact_fixture_activity(&kernel, &activity, &execution, &receipt)) != LXP_OK ||
            receipt.result_code != LXP_OK ||
            identity->next_sequence != ++identity_sequence ||
            source->balance.hi != source_before.hi ||
            source->balance.lo != source_before.lo - PROGRAM_SPEND_AMOUNT ||
            payee->balance.hi != payee_before.hi ||
            payee->balance.lo != payee_before.lo + PROGRAM_SPEND_AMOUNT + 3U ||
            monetary_exact_fee(actor_before, treasury_before, actor, treasury, receipt.fee_charged, 3U) != 0)
            return monetary_real_failure(__LINE__, call_result, &receipt);
    }
    if (source->balance.hi != 0U || source->balance.lo >= PROGRAM_SPEND_AMOUNT ||
        program_leg_insufficient_balance_refusal(actor, source, payee,
                                                 &fee_asset_state) != 0)
        return monetary_real_failure(__LINE__, call_result, &receipt);
    fill_activity(&activity, LX_PROGRAMS_CALL, call, payload_length,
                  did, sizeof(did) - 1U, primary_key);
    activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    activity.account_sequence = identity_sequence;
    activity.idempotency_key[31] = 0x70U;
    activity.fee_limit = (lxp_u128){0U, 67108864U};
    execution.fee_balance = actor->balance;
    execution.global_sequence = state.next_sequence;
    source_before = source->balance;
    payee_before = payee->balance;
    actor_before = actor->balance;
    actor_sequence_before = actor->next_sequence;
    treasury_before = treasury->balance;
    if (lxp_arena_reset(&arena, 0U) != LXP_OK ||
        (call_result = execute_artifact_fixture_activity(&kernel, &activity, &execution, &receipt)) != LXP_OK ||
        receipt.result_code != LXP_ERR_PROGRAM_REFUSED ||
        monetary_kernel_refusal(&receipt) != 0 ||
        actor->next_sequence != actor_sequence_before ||
        (receipt.fee_charged.hi == 0U && receipt.fee_charged.lo == 0U) ||
        identity->next_sequence != ++identity_sequence ||
        state.next_sequence != execution.global_sequence + 1U ||
        receipt.effects.count != 0U ||
        !lxp_ct_is_zero(receipt.transfer_set_root, 32U) ||
        lxp_u128_cmp(source->balance, source_before) != 0 ||
        lxp_u128_cmp(payee->balance, payee_before) != 0 ||
        monetary_exact_fee(actor_before, treasury_before, actor, treasury, receipt.fee_charged, 0U) != 0)
        return monetary_real_failure(__LINE__, call_result, &receipt);
    if (program_spend_kernel_refusals(&kernel, source, payee, fee_asset) != 0)
        return 1;
    while (kernel.blob_count != 0U)
        free(kernel.blobs[--kernel.blob_count].bytes);
    return lxp_state_store_destroy(&state) == LXP_OK ? 0 : 1;
}

int main(void)
{
    static const uint8_t actor_did[] = "did:lxp:program-transfer";
    const char *names[3] = {"agent:did:key:a:main", "agent:did:key:b:main",
                            "agent:did:key:c:main"};
    uint8_t ids[3][32];
    lx_account_registry accounts;
    lx_account *opened[3];
    lxp_transfer_asset_state asset = {{1U}, true, false};
    lx_programs_transfer_runtime runtime;
    lxp_state_store state;
    lxp_state_journal journal;
    static lxp_kernel kernel;
    lxp_authority_resolved authority;
    lxp_identity_store identities = {0};
    lxp_identity *identity;
    uint8_t primary_key[32] = {1U};
    static uint8_t execution_arena_bytes[LXP_MAX_ACTIVITY_BYTES + 4096U];
    lxp_arena execution_arena;
    lxp_kernel_execution execution;
    lxp_fee_params fee_parameters = {1U, {0U, 1U}, {0U, 0U}, {0U, 0U},
                                     {0U, 0U}, {0U, 0U}, 10000U, 0U, {{0U, 0U}}, 0U, {{0U, 0U}} };
    lxp_activity activity;
    lxp_receipt receipt;
    uint8_t zero_root[32] = {0};
    uint8_t payload[258] = {0};
    uint8_t oversized_payload[259];
    uint64_t parameters = 1U;
    size_t i;
    if (lx_account_registry_init(&accounts) != LXP_OK) return 1;
    for (i = 0U; i < 3U; ++i)
        if (lx_account_id_from_string((const uint8_t *)names[i], strlen(names[i]),
                                      ids[i]) != LXP_OK ||
            lx_account_open(&accounts, (const uint8_t *)names[i], strlen(names[i]),
                            ids[i], 1U, LX_ACCOUNT_OPEN_CREDIT, NULL,
                            &opened[i]) != LXP_OK)
            return 1;
    if (lxp_ledger_bootstrap_balance(opened[0], asset.asset_id,
                                     (lxp_u128){0U, 100U}, 0U) != LXP_OK ||
        lxp_ledger_bootstrap_balance(opened[1], asset.asset_id,
                                     (lxp_u128){0U, 0U}, 0U) != LXP_OK ||
        lxp_ledger_bootstrap_balance(opened[2], asset.asset_id,
                                     (lxp_u128){0U, 0U}, 0U) != LXP_OK)
        return 1;
    runtime = (lx_programs_transfer_runtime){
        .accounts = &accounts,
        .assets = &asset,
        .asset_count = 1U,
    };
    (void)memset(&authority, 0, sizeof(authority));
    (void)memcpy(authority.principal, ids[0], 32U);
    (void)memset(authority.authority_hash, 0x55, 32U);
    (void)memset(payload, 0x77, 32U);
    write_u16(payload + 32U, 2U);
    leg(payload + 34U, ids[0], asset.asset_id, ids[1], 30U);
    leg(payload + 146U, ids[0], asset.asset_id, ids[2], 80U);
    if (lxp_state_store_init(&state, 1U) != LXP_OK ||
        lxp_identity_register(&identities, actor_did, sizeof(actor_did) - 1U,
                              primary_key, &identity) != LXP_OK ||
        lxp_kernel_create(&kernel, &state, &journal, &parameters, 0U) != LXP_OK ||
        lxp_kernel_register_module(&kernel, programs_module_registration()) != LXP_OK ||
        lxp_kernel_set_fee_transaction(
            &kernel, &(lxp_kernel_fee_transaction){ prepare_fee, commit_fee,
                                                    rollback_fee }) != LXP_OK ||
        lxp_kernel_set_capabilities(&kernel, NULL, apply_transfer_set) != LXP_OK ||
        lxp_kernel_bind_module_runtime(&kernel, LXP_MODULE_PROGRAMS, &runtime) != LXP_OK ||
        dispatch(&kernel, &authority, payload, sizeof(payload), 1U,
                 LXP_ERR_SEQUENCE_GAP) != 0 ||
        opened[0]->balance.lo != 100U || opened[0]->next_sequence != 0U ||
        opened[1]->balance.lo != 0U || opened[2]->balance.lo != 0U ||
        dispatch(&kernel, &authority, payload, sizeof(payload), 0U,
                 LXP_ERR_INSUFFICIENT_BALANCE) != 0 ||
        opened[0]->balance.lo != 100U || opened[0]->next_sequence != 0U ||
        opened[1]->balance.lo != 0U ||
        opened[2]->balance.lo != 0U)
        return 1;
    write_u64(payload + 250U, 20U);
    (void)memset(&activity, 0, sizeof(activity));
    activity.protocol_version = LXP_PROTOCOL_VERSION;
    activity.network_id = 7U;
    activity.activity_type = LX_PROGRAMS_TRANSFER;
    activity.actor_did = (lxp_byte_span){actor_did, sizeof(actor_did) - 1U};
    activity.authority = (lxp_byte_span){primary_key, sizeof(primary_key)};
    activity.timestamp_bound = (lxp_timestamp_bound){1U, 100U};
    activity.account_sequence = 0U;
    activity.idempotency_key[31] = 1U;
    activity.fee_limit = (lxp_u128){0U, 1U};
    activity.payload = (lxp_byte_span){payload, sizeof(payload)};
    if (lxp_hash_payload(payload, sizeof(payload), activity.payload_hash) != LXP_OK ||
        lxp_arena_init(&execution_arena, execution_arena_bytes,
                       sizeof(execution_arena_bytes)) != LXP_OK)
        return 1;
    (void)memset(&execution, 0, sizeof(execution));
    execution.network_id = 7U;
    execution.batch_number = 1U;
    execution.batch_timestamp_ms = 10U;
    execution.maximum_timestamp_window = 100U;
    execution.global_sequence = 1U;
    execution.recorded_module_version = LX_PROGRAMS_ABI_VERSION;
    execution.parameter_version = 1U;
    execution.signature_valid = true;
    execution.identities = &identities;
    execution.authority = &authority;
    execution.fee_parameters = &fee_parameters;
    execution.fee_balance = (lxp_u128){0U, 1U};
    execution.gas_limit = 100000U;
    execution.arena = &execution_arena;
    if (lxp_kernel_execute_activity(&kernel, &activity, &execution, &receipt) != LXP_OK ||
        receipt.result_code != LXP_OK || receipt.module_id != LXP_MODULE_PROGRAMS ||
        receipt.module_version != LX_PROGRAMS_ABI_VERSION ||
        receipt.effects.count != 1U ||
        receipt.effects.effects[0].event_type != LX_PROGRAMS_EVENT_TRANSFERRED ||
        receipt.effects.effects[0].body_length != 32U ||
        memcmp(receipt.effects.effects[0].body, zero_root, 32U) == 0 ||
        charged_fees != 1U || last_fee.hi != 0U || last_fee.lo != 1U ||
        identity->next_sequence != 1U ||
        opened[0]->next_sequence != 1U ||
        opened[0]->balance.lo != 50U || opened[1]->balance.lo != 30U ||
        opened[2]->balance.lo != 20U)
        return 1;
    write_u64(payload + 250U, 80U);
    activity.account_sequence = 1U;
    activity.idempotency_key[31] = 2U;
    activity.payload_hash[0] = 0U;
    if (lxp_hash_payload(payload, sizeof(payload), activity.payload_hash) != LXP_OK)
        return 1;
    execution.global_sequence = 2U;
    if (lxp_kernel_execute_activity(&kernel, &activity, &execution, &receipt) !=
            LXP_OK ||
        receipt.result_code != LXP_ERR_INSUFFICIENT_BALANCE ||
        receipt.module_id != LXP_MODULE_PROGRAMS ||
        receipt.module_version != LX_PROGRAMS_ABI_VERSION ||
        receipt.effects.count != 0U ||
        receipt.fee_charged.hi != 0U || receipt.fee_charged.lo != 1U ||
        charged_fees != 2U || last_fee.hi != 0U || last_fee.lo != 1U ||
        identity->next_sequence != 2U || state.next_sequence != 3U ||
        opened[0]->next_sequence != 1U ||
        opened[0]->balance.lo != 50U || opened[1]->balance.lo != 30U ||
        opened[2]->balance.lo != 20U)
        return 1;
    write_u64(payload + 250U, 20U);
    (void)memcpy(oversized_payload, payload, sizeof(payload));
    oversized_payload[sizeof(payload)] = 0U;
    if (dispatch(&kernel, &authority, oversized_payload,
                 sizeof(oversized_payload), 1U,
                 LXP_ERR_NON_CANONICAL) != 0 ||
        opened[0]->balance.lo != 50U || opened[1]->balance.lo != 30U ||
        opened[2]->balance.lo != 20U)
        return 1;
    payload[34] ^= 1U;
    if (dispatch(&kernel, &authority, payload, sizeof(payload), 1U,
                 LXP_ERR_AUTH_SCOPE) != 0 || opened[0]->balance.lo != 50U)
        return 1;
    if (report_case("program_leg_insufficient_balance_refusal_receipt_fees_sequence",
                    program_leg_insufficient_balance_refusal(
                        opened[0], opened[1], opened[2], &asset)) != 0 ||
        report_case("mixed_source_kernel_law",
                    mixed_source_kernel_law(opened[0], opened[1], opened[2],
                                            &asset)) != 0 ||
        report_case("program_leg_atomic_rollback_after_first_leg",
                    program_leg_atomic_rollback_after_first_leg(
                        opened[0], opened[1], opened[2], &asset)) != 0 ||
        report_case("program_leg_cumulative_bound_refusal",
                    program_leg_cumulative_bound_refusal(
                        opened[0], opened[1], opened[2], &asset)) != 0 ||
        report_case("kernel_primitive_sole_balance_mutation",
                    program_spend_kernel_law(&kernel, opened[0], opened[1],
                                             opened[2], &asset)) != 0)
        return 1;
    if (lxp_state_store_destroy(&state) != LXP_OK) return 1;
    if (report_case("real_mixed_program_authority_kernel_case",
                    real_mixed_program_authority_kernel_case()) != 0) return 1;
    (void)printf("PASSED %zu\n", passed_cases);
    return 0;
}
