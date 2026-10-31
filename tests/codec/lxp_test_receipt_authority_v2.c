#include "lxp_kernel_case.h"

#include "../../src/modules/programs/call.c"

extern lxp_result lxp_verified_receipt_index_add_authority(
    lxp_verified_receipt_index *index, lxp_byte_span canonical_receipt,
    lxp_byte_span canonical_header, const uint8_t header_signature[64],
    const lxp_merkle_proof *receipt_proof,
    const lxp_sequencer_authorization *authorization,
    lxp_arena *arena) __attribute__((weak));

enum { AUTHORITY_V2_NETWORK = 0x01020304 };
static const uint64_t authority_height = UINT64_C(0x0102030405060708);

static lxp_verified_receipt_index authority_index;
static lxp_kernel authority_kernel;
static lxp_state_store authority_state;
static lxp_state_journal authority_journal;
static lxp_module_ctx authority_ctx;
static lxp_authority_resolved call_authority;
static lxp_programs_call_activity authority_call;
static lxp_receipt source_receipt;
static uint8_t receipt_storage[LXP_MAX_ACTIVITY_BYTES + 1U];
static uint8_t work_storage[2U * LXP_MAX_ACTIVITY_BYTES];
static lxp_byte_span canonical_receipt;
static lxp_arena authority_arena;
static uint8_t receipt_digest[32];

static int produce_receipt(bool signed_receipt)
{
    static const uint8_t key[32] = {0x57U};
    lxp_ledger_receipt_input input = {0};
    lxp_byte_span encoded;
    size_t i;

    input.operation = 0x39U;
    input.global_sequence = UINT64_MAX;
    input.timestamp = 19U;
    input.leg_count = 1U;
    input.amount.lo = 17U;
    input.from_balance_before.lo = 100U;
    input.from_balance_after.lo = 83U;
    input.to_balance_before.lo = 1U;
    input.to_balance_after.lo = 18U;
    for (i = 0U; i < 32U; ++i) {
        input.transaction_id[i] = (uint8_t)(i + 1U);
        input.previous_state_root[i] = (uint8_t)(i + 33U);
        input.resulting_state_root[i] = (uint8_t)(i + 65U);
        input.transfer_set_root[i] = (uint8_t)(i + 97U);
        input.asset[i] = (uint8_t)(i + 129U);
    }
    LXP_CASE_CHECK(lxp_ledger_receipt_build(&source_receipt, &input) == LXP_OK);
    LXP_CASE_CHECK(lxp_arena_init(&authority_arena, work_storage,
                                  sizeof(work_storage)) == LXP_OK);
    LXP_CASE_CHECK(lxp_receipt_digest(&source_receipt, &authority_arena,
                                     receipt_digest) == LXP_OK);
    if (signed_receipt)
        LXP_CASE_CHECK(lxp_receipt_sign(&source_receipt, key,
                                        &authority_arena) == LXP_OK);
    LXP_CASE_CHECK(lxp_receipt_encode(&source_receipt, signed_receipt,
                                     &authority_arena, &encoded) == LXP_OK);
    LXP_CASE_CHECK(encoded.length < sizeof(receipt_storage));
    (void)memcpy(receipt_storage, encoded.bytes, encoded.length);
    canonical_receipt = (lxp_byte_span){receipt_storage, encoded.length};
    LXP_CASE_CHECK(lxp_arena_reset(&authority_arena, 0U) == LXP_OK);
    return 0;
}

static int open_consumer(uint32_t network_id)
{
    static const uint64_t parameters = 1U;

    LXP_CASE_CHECK(lxp_state_store_init(&authority_state, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_create(&authority_kernel, &authority_state,
        &authority_journal, &parameters, 0U) == LXP_OK);
    LXP_CASE_CHECK(lxp_kernel_register_module(&authority_kernel,
        programs_module_registration()) == LXP_OK);
    LXP_CASE_CHECK(lxp_module_ctx_init(&authority_ctx, &authority_kernel,
        LXP_MODULE_PROGRAMS, 19U, 0U, 1U, UINT64_MAX,
        &authority_arena, false) == LXP_OK);
    authority_ctx.batch_number = authority_height + 1U;
    authority_ctx.verified_receipts = &authority_index;
    (void)memset(&call_authority, 0, sizeof(call_authority));
    call_authority.principal[0] = 1U;
    call_authority.authority_hash[0] = 2U;
    (void)memset(&authority_call, 0, sizeof(authority_call));
    authority_call.ctx = &authority_ctx;
    authority_call.authority = &call_authority;
    authority_call.network_id = network_id;
    return 0;
}

static int close_consumer(void)
{
    lxp_module_ctx_rollback(&authority_ctx);
    (void)memset(&authority_call, 0, sizeof(authority_call));
    return lxp_state_store_destroy(&authority_state) == LXP_OK ? 0 : 1;
}

static lxp_result begin_authority(uint64_t expected_height)
{
    return layerx_programs_call_arbiter_authority_begin(
        (uint64_t)(uintptr_t)&authority_call,
        read_u64(receipt_digest), read_u64(receipt_digest + 8U),
        read_u64(receipt_digest + 16U), read_u64(receipt_digest + 24U),
        expected_height);
}

static int authority_v2_record_bytes(void)
{
    static const uint8_t prefix[] = {
        0U, 2U, 1U, 2U, 3U, 4U, 0U, LXP_PROTOCOL_VERSION,
        0U, LXP_LEDGER_RECEIPT_MODULE_ID, 0x39U, 0U, 0U, 0U, 0U,
        1U, 2U, 3U, 4U, 5U, 6U, 7U, 8U
    };
    uint8_t expected[151];
    lxp_verified_receipt_authority_facts facts;
    unsigned int restart;
    uint32_t offset;
    size_t mark;

    LXP_CASE_CHECK(ARBITER_AUTHORITY_BYTES == 151);
    LXP_CASE_CHECK(produce_receipt(true) == 0);
    (void)memcpy(expected, prefix, sizeof(prefix));
    (void)memcpy(expected + 23U, receipt_digest, 32U);
    (void)memcpy(expected + 55U, source_receipt.activity_id, 32U);
    (void)memcpy(expected + 87U, source_receipt.previous_state_root, 32U);
    (void)memcpy(expected + 119U, source_receipt.resulting_state_root, 32U);
    for (restart = 0U; restart < 2U; ++restart) {
        LXP_CASE_CHECK(lxp_verified_receipt_index_init(&authority_index) == LXP_OK);
        LXP_CASE_CHECK(lxp_verified_receipt_index_bind_authority_network(
            &authority_index, AUTHORITY_V2_NETWORK) == LXP_OK);
        mark = lxp_arena_mark(&authority_arena);
        LXP_CASE_CHECK(lxp_verified_receipt_index_add_height_authority(
            &authority_index, canonical_receipt, authority_height,
            &authority_arena) == LXP_OK);
        LXP_CASE_CHECK(lxp_arena_mark(&authority_arena) == mark);
        LXP_CASE_CHECK(open_consumer(AUTHORITY_V2_NETWORK) == 0);
        LXP_CASE_CHECK(lxp_ctx_verified_receipt_authority_facts(
            &authority_ctx, receipt_digest, &facts) == LXP_OK);
        LXP_CASE_CHECK(facts.version == 2U);
        LXP_CASE_CHECK(facts.network_id == AUTHORITY_V2_NETWORK);
        LXP_CASE_CHECK(facts.protocol_version == source_receipt.protocol_version);
        LXP_CASE_CHECK(facts.module_id == source_receipt.module_id);
        LXP_CASE_CHECK(facts.operation == source_receipt.operation);
        LXP_CASE_CHECK(facts.result_code == source_receipt.result_code);
        LXP_CASE_CHECK(facts.block_height == authority_height);
        LXP_CASE_CHECK(begin_authority(authority_height) == LXP_OK);
        for (offset = 0U; offset < sizeof(expected); ++offset)
            LXP_CASE_CHECK(layerx_programs_call_arbiter_authority_byte(
                (uint64_t)(uintptr_t)&authority_call, offset) ==
                (lxp_result)expected[offset]);
        LXP_CASE_CHECK(layerx_programs_call_arbiter_authority_byte(
            (uint64_t)(uintptr_t)&authority_call, 151U) == LXP_ERR_TRUNCATED);
        LXP_CASE_CHECK(layerx_programs_call_arbiter_authority_byte(
            (uint64_t)(uintptr_t)&authority_call, UINT32_MAX) == LXP_ERR_TRUNCATED);
        LXP_CASE_CHECK(close_consumer() == 0);
    }
    return 0;
}

static int authority_v2_height_checks(void)
{
    uint32_t offset;

    LXP_CASE_CHECK(produce_receipt(false) == 0);
    LXP_CASE_CHECK(lxp_verified_receipt_index_init(&authority_index) == LXP_OK);
    LXP_CASE_CHECK(lxp_verified_receipt_index_bind_authority_network(
        &authority_index, AUTHORITY_V2_NETWORK) == LXP_OK);
    LXP_CASE_CHECK(lxp_verified_receipt_index_add_height_authority(
        &authority_index, canonical_receipt, authority_height,
        &authority_arena) == LXP_OK);
    LXP_CASE_CHECK(open_consumer(AUTHORITY_V2_NETWORK) == 0);
    LXP_CASE_CHECK(begin_authority(authority_height) == LXP_OK);
    LXP_CASE_CHECK(begin_authority(authority_height + 1U) == LXP_ERR_ROOT_MISMATCH);
    LXP_CASE_CHECK(layerx_programs_call_arbiter_authority_byte(
        (uint64_t)(uintptr_t)&authority_call, 0U) == LXP_ERR_UNKNOWN_FIELD);
    for (offset = 0U; offset < ARBITER_AUTHORITY_BYTES; ++offset)
        LXP_CASE_CHECK(authority_call.arbiter_authority[offset] == 0U);
    authority_ctx.batch_number = authority_height;
    LXP_CASE_CHECK(begin_authority(authority_height) == LXP_ERR_NOT_YET_VALID);
    authority_ctx.batch_number = authority_height - 1U;
    LXP_CASE_CHECK(begin_authority(authority_height) == LXP_ERR_NOT_YET_VALID);
    authority_ctx.batch_number = authority_height + 1U;
    authority_ctx.global_sequence = 0U;
    LXP_CASE_CHECK(begin_authority(authority_height) == LXP_OK);
    authority_call.network_id = AUTHORITY_V2_NETWORK + 1U;
    LXP_CASE_CHECK(begin_authority(authority_height) == LXP_ERR_WRONG_NETWORK);
    LXP_CASE_CHECK(layerx_programs_call_arbiter_authority_byte(
        (uint64_t)(uintptr_t)&authority_call, 0U) == LXP_ERR_UNKNOWN_FIELD);
    authority_call.network_id = AUTHORITY_V2_NETWORK;
    LXP_CASE_CHECK(begin_authority(0U) == LXP_ERR_NON_CANONICAL);
    authority_index.authority_entries[0].version = 1U;
    LXP_CASE_CHECK(begin_authority(authority_height) == LXP_ERR_VERSION_UNSUPPORTED);
    LXP_CASE_CHECK(close_consumer() == 0);
    return 0;
}

static int height_authority_add_refusals(void)
{
    lxp_verified_receipt_authority_facts facts;
    lxp_byte_span invalid;
    size_t mark;

    LXP_CASE_CHECK(lxp_verified_receipt_index_add_authority == NULL);
    LXP_CASE_CHECK(produce_receipt(true) == 0);
    LXP_CASE_CHECK(lxp_verified_receipt_index_init(&authority_index) == LXP_OK);
    LXP_CASE_CHECK(lxp_verified_receipt_index_bind_authority_network(
        NULL, AUTHORITY_V2_NETWORK) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_verified_receipt_index_bind_authority_network(
        &authority_index, 0U) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_verified_receipt_index_add_height_authority(
        &authority_index, canonical_receipt, authority_height,
        &authority_arena) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_verified_receipt_index_bind_authority_network(
        &authority_index, AUTHORITY_V2_NETWORK) == LXP_OK);
    LXP_CASE_CHECK(lxp_verified_receipt_index_add_height_authority(
        &authority_index, canonical_receipt, 0U,
        &authority_arena) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_verified_receipt_index_add_height_authority(
        NULL, canonical_receipt, authority_height,
        &authority_arena) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_verified_receipt_index_add_height_authority(
        &authority_index, canonical_receipt, authority_height,
        NULL) == LXP_ERR_NON_CANONICAL);
    invalid = (lxp_byte_span){NULL, canonical_receipt.length};
    LXP_CASE_CHECK(lxp_verified_receipt_index_add_height_authority(
        &authority_index, invalid, authority_height,
        &authority_arena) == LXP_ERR_NON_CANONICAL);
    invalid = (lxp_byte_span){receipt_storage, canonical_receipt.length - 1U};
    LXP_CASE_CHECK(lxp_verified_receipt_index_add_height_authority(
        &authority_index, invalid, authority_height,
        &authority_arena) == LXP_ERR_NON_CANONICAL);
    receipt_storage[canonical_receipt.length] = 0U;
    invalid.length = canonical_receipt.length + 1U;
    LXP_CASE_CHECK(lxp_verified_receipt_index_add_height_authority(
        &authority_index, invalid, authority_height,
        &authority_arena) == LXP_ERR_NON_CANONICAL);
    receipt_storage[0] ^= 0x80U;
    LXP_CASE_CHECK(lxp_verified_receipt_index_add_height_authority(
        &authority_index, canonical_receipt, authority_height,
        &authority_arena) == LXP_ERR_NON_CANONICAL);
    receipt_storage[0] ^= 0x80U;
    LXP_CASE_CHECK(authority_index.authority_count == 0U);
    mark = lxp_arena_mark(&authority_arena);
    LXP_CASE_CHECK(lxp_verified_receipt_index_add_height_authority(
        &authority_index, canonical_receipt, authority_height,
        &authority_arena) == LXP_OK);
    LXP_CASE_CHECK(lxp_verified_receipt_index_add_height_authority(
        &authority_index, canonical_receipt, authority_height,
        &authority_arena) == LXP_OK);
    LXP_CASE_CHECK(authority_index.authority_count == 1U);
    LXP_CASE_CHECK(lxp_verified_receipt_index_add_height_authority(
        &authority_index, canonical_receipt, authority_height + 1U,
        &authority_arena) == LXP_ERR_ROOT_MISMATCH);
    LXP_CASE_CHECK(authority_index.authority_count == 1U);
    LXP_CASE_CHECK(lxp_arena_mark(&authority_arena) == mark);
    LXP_CASE_CHECK(lxp_verified_receipt_index_lookup_authority(
        &authority_index, receipt_digest, &facts) == LXP_OK);
    LXP_CASE_CHECK(facts.block_height == authority_height);
    LXP_CASE_CHECK(lxp_verified_receipt_index_bind_authority_network(
        &authority_index, AUTHORITY_V2_NETWORK) == LXP_OK);
    LXP_CASE_CHECK(lxp_verified_receipt_index_bind_authority_network(
        &authority_index, AUTHORITY_V2_NETWORK + 1U) == LXP_ERR_WRONG_NETWORK);
    LXP_CASE_CHECK(produce_receipt(false) == 0);
    LXP_CASE_CHECK(lxp_verified_receipt_index_add_height_authority(
        &authority_index, canonical_receipt, authority_height,
        &authority_arena) == LXP_OK);
    LXP_CASE_CHECK(authority_index.authority_count == 1U);
    authority_index.authority_count = LXP_VERIFIED_RECEIPT_INDEX_MAX + 1U;
    LXP_CASE_CHECK(lxp_verified_receipt_index_add_height_authority(
        &authority_index, canonical_receipt, authority_height,
        &authority_arena) == LXP_ERR_NON_CANONICAL);
    LXP_CASE_CHECK(lxp_verified_receipt_index_init(&authority_index) == LXP_OK);
    return 0;
}

int main(int argc, char **argv)
{
    const lxp_kernel_case cases[] = {
        {"authority_v2_record_bytes", authority_v2_record_bytes},
        {"authority_v2_height_checks", authority_v2_height_checks},
        {"height_authority_add_refusals", height_authority_add_refusals}
    };
    return lxp_kernel_case_main(cases, sizeof(cases) / sizeof(cases[0]), argc, argv);
}
