#define main web_program_path_reference_main
int web_program_path_reference_main(int argc, char **argv);
#include "programs/test_call_activity.c"
#undef main

#include "lxp_test_harness.c"

/* Task 22.33.892: this suite proves the guest/kernel paid-request boundary
 * with the current registration and retains ABI-v4 compatibility explicitly.
 * Native xweb fee escrow, attestation and committed answer consumption require
 * the Go owner wiring recorded in test_lxgb_producers.py; no kernel web module
 * is registered here and legacy web_read remains refusal coverage. */

#define PATH_CHECK(condition) do { if (!(condition)) { \
    (void)fprintf(stderr, "web program path check failed at line %d\n", \
                  __LINE__); \
    return 1; } } while (0)

enum {
    PATH_NETWORK = 7,
    PATH_WASM_CAPACITY = 262144,
    PATH_CALLDATA_BYTES = 512,
    PATH_PENDING_KEY_BYTES = 11 + 32 + 8,
    PATH_KIND_FETCH = 1,
    PATH_FEE = 100,
    PATH_OP_REQUEST = 1,
    PATH_OP_READ = 2
};

static const uint8_t path_request_topic[] = "PAXEERX_WEB_REQUEST_V1";
static const uint8_t path_response[] = "Paxeer X Network";
static const uint8_t path_payload[] = "https://paxeer.app/status";
static const uint64_t path_request = UINT64_C(0x0102030405060708);
static const uint64_t path_unpaid_request = UINT64_C(0x0102030405060709);

enum {
    PATH_RESPONSE_BYTES = sizeof(path_response) - 1U,
    PATH_PAYLOAD_BYTES = sizeof(path_payload) - 1U
};

static int path_read_file(const char *path, uint8_t *out, size_t capacity,
                          size_t *length)
{
    FILE *file = fopen(path, "rb");
    size_t read;
    if (file == NULL) return 1;
    read = fread(out, 1U, capacity, file);
    if (ferror(file) || !feof(file)) {
        (void)fclose(file);
        return 1;
    }
    (void)fclose(file);
    *length = read;
    return read == 0U ? 1 : 0;
}

static void path_guest_const(uint8_t *out, size_t *cursor, uint32_t value)
{
    uint8_t byte;
    out[(*cursor)++] = 0x41U; /* i32.const, signed LEB128 */
    do {
        byte = (uint8_t)(value & 0x7fU);
        value >>= 7U;
        out[(*cursor)++] = value != 0U || (byte & 0x40U) != 0U ?
            (uint8_t)(byte | 0x80U) : byte;
    } while (value != 0U || (byte & 0x40U) != 0U);
}

static void path_guest_byte(uint8_t *out, size_t *cursor, uint32_t offset)
{
    out[(*cursor)++] = 0x20U; out[(*cursor)++] = 0U;
    out[(*cursor)++] = 0x2dU; out[(*cursor)++] = 0U;
    append_u32_leb(out, cursor, offset); /* input[offset] */
}

static void path_guest_u64(uint8_t *out, size_t *cursor, uint32_t offset)
{
    uint32_t index;
    out[(*cursor)++] = 0x42U; out[(*cursor)++] = 0U;
    for (index = 0U; index < 8U; ++index) {
        out[(*cursor)++] = 0x42U; out[(*cursor)++] = 8U;
        out[(*cursor)++] = 0x86U; /* i64.shl */
        path_guest_byte(out, cursor, offset + index);
        out[(*cursor)++] = 0xadU; /* i64.extend_i32_u */
        out[(*cursor)++] = 0x84U; /* i64.or */
    }
}

/* A complete byte-entry guest fixture: calldata supplies both transfer legs
 * and the request record. The production runtime performs payment and emits
 * the envelope; this guest has no host replacement or precomputed receipt. */
static size_t path_guest_module(uint8_t *out)
{
    static const uint8_t header[] = {0U, 0x61U, 0x73U, 0x6dU, 1U, 0U, 0U, 0U};
    static const uint8_t types[] = {
        4U,
        0x60U, 6U, 0x7eU, 0x7eU, 0x7fU, 0x7fU, 0x7fU, 0x7fU, 1U, 0x7fU,
        0x60U, 4U, 0x7fU, 0x7fU, 0x7fU, 0x7fU, 1U, 0x7fU,
        0x60U, 1U, 0x7fU, 1U, 0x7fU,
        0x60U, 2U, 0x7fU, 0x7fU, 1U, 0x7fU
    };
    static const uint8_t functions[] = {2U, 2U, 3U};
    static const uint8_t memory[] = {1U, 1U, 1U, 1U};
    uint8_t section[2048];
    uint8_t body[1536];
    size_t cursor = 0U, length = 0U, body_length = 0U;
    uint32_t index;
    append_bytes(out, &cursor, header, sizeof(header));
    append_section(out, &cursor, 1U, types, sizeof(types));
    section[length++] = 3U;
    append_name(section, &length, "layerx_v1");
    append_name(section, &length, "transfer_402");
    section[length++] = 0U; section[length++] = 0U;
    append_name(section, &length, "layerx_v1");
    append_name(section, &length, "event_emit");
    section[length++] = 0U; section[length++] = 1U;
    append_name(section, &length, "layerx_v4");
    append_name(section, &length, "web_read");
    section[length++] = 0U; section[length++] = 1U;
    append_section(out, &cursor, 2U, section, length);
    append_section(out, &cursor, 3U, functions, sizeof(functions));
    append_section(out, &cursor, 5U, memory, sizeof(memory));
    length = 0U; section[length++] = 3U;
    append_name(section, &length, "layerx_reserve");
    section[length++] = 0U; section[length++] = 3U;
    append_name(section, &length, "layerx_call");
    section[length++] = 0U; section[length++] = 4U;
    append_name(section, &length, "memory");
    section[length++] = 2U; section[length++] = 0U;
    append_section(out, &cursor, 7U, section, length);

    /* reserve returns an input buffer at 1024, clear of topic and output. */
    length = 0U; section[length++] = 2U;
    section[length++] = 5U; section[length++] = 0U;
    path_guest_const(section, &length, 1024U); section[length++] = 0x0bU;
    body[body_length++] = 1U; body[body_length++] = 1U;
    body[body_length++] = 0x7fU; /* local 2: host status */
    path_guest_byte(body, &body_length, 1U);
    path_guest_const(body, &body_length, PATH_OP_READ);
    body[body_length++] = 0x46U; /* i32.eq */
    body[body_length++] = 0x04U; body[body_length++] = 0x40U;
    /* The compatibility read reaches the real disabled web_read import. */
    path_guest_const(body, &body_length, 0x01020304U);
    path_guest_const(body, &body_length, 0x05060708U);
    path_guest_const(body, &body_length, 8192U);
    path_guest_const(body, &body_length, 4136U);
    body[body_length++] = 0x10U; body[body_length++] = 2U;
    body[body_length++] = 0x0fU; body[body_length++] = 0x0bU;
    path_guest_u64(body, &body_length, 75U);
    path_guest_u64(body, &body_length, 83U);
    body[body_length++] = 0x20U; body[body_length++] = 0U;
    path_guest_const(body, &body_length, 11U); body[body_length++] = 0x6aU;
    path_guest_const(body, &body_length, 32U);
    body[body_length++] = 0x20U; body[body_length++] = 0U;
    path_guest_const(body, &body_length, 43U); body[body_length++] = 0x6aU;
    path_guest_const(body, &body_length, 32U);
    body[body_length++] = 0x10U; body[body_length++] = 0U;
    body[body_length++] = 0x22U; body[body_length++] = 2U;
    body[body_length++] = 0x04U; body[body_length++] = 0x40U;
    body[body_length++] = 0x20U; body[body_length++] = 2U;
    body[body_length++] = 0x0fU; body[body_length++] = 0x0bU;
    for (index = 0U; index < 9U; ++index) {
        path_guest_const(body, &body_length, 4096U + index);
        path_guest_byte(body, &body_length, 2U + index);
        body[body_length++] = 0x3aU;
        body[body_length++] = 0U; body[body_length++] = 0U;
    }
    for (index = 0U; index < 4U; ++index) {
        path_guest_const(body, &body_length, 4105U + index);
        path_guest_const(body, &body_length, PATH_PAYLOAD_BYTES);
        path_guest_const(body, &body_length, (3U - index) * 8U);
        body[body_length++] = 0x76U; /* i32.shr_u */
        body[body_length++] = 0x3aU;
        body[body_length++] = 0U; body[body_length++] = 0U;
    }
    for (index = 0U; index < PATH_PAYLOAD_BYTES; ++index) {
        path_guest_const(body, &body_length, 4109U + index);
        path_guest_byte(body, &body_length, 91U + index);
        body[body_length++] = 0x3aU;
        body[body_length++] = 0U; body[body_length++] = 0U;
    }
    path_guest_const(body, &body_length, 0U);
    path_guest_const(body, &body_length, sizeof(path_request_topic) - 1U);
    path_guest_const(body, &body_length, 4096U);
    path_guest_const(body, &body_length, 13U + PATH_PAYLOAD_BYTES);
    body[body_length++] = 0x10U; body[body_length++] = 1U;
    body[body_length++] = 0x0bU;
    append_u32_leb(section, &length, (uint32_t)body_length);
    append_bytes(section, &length, body, body_length);
    append_section(out, &cursor, 10U, section, length);
    length = 0U; section[length++] = 1U; section[length++] = 0U;
    path_guest_const(section, &length, 0U); section[length++] = 0x0bU;
    append_u32_leb(section, &length, sizeof(path_request_topic) - 1U);
    append_bytes(section, &length, path_request_topic,
                  sizeof(path_request_topic) - 1U);
    append_section(out, &cursor, 11U, section, length);
    return cursor;
}

static lxp_result path_fee_account(const uint8_t asset[32], uint8_t id[32])
{
    static const uint8_t domain[] = "PAXEERX_WEB_FEES_V1";
    uint8_t preimage[sizeof(domain) - 1U + 32U];
    (void)memcpy(preimage, domain, sizeof(domain) - 1U);
    (void)memcpy(preimage + sizeof(domain) - 1U, asset, 32U);
    return lxp_hash_sha256(preimage, sizeof(preimage), id);
}

static void path_pending_key(const uint8_t program_id[32],
                             uint64_t request_id,
                             uint8_t key[PATH_PENDING_KEY_BYTES])
{
    (void)memcpy(key, "web/pending", 11U);
    (void)memcpy(key + 11U, program_id, 32U);
    write_u64(key + 43U, request_id);
}

static const lxp_module_kv_entry *path_committed(const lxp_kernel *kernel,
                                                 const uint8_t *key,
                                                 size_t key_length)
{
    size_t index;
    for (index = 0U; index < kernel->module_kv_count; ++index) {
        const lxp_module_kv_entry *entry = &kernel->module_kv[index];
        if (entry->module_id == LXP_MODULE_PROGRAMS &&
            entry->key_length == key_length &&
            memcmp(entry->key, key, key_length) == 0)
            return entry;
    }
    return NULL;
}

static uint32_t path_read_u32(const uint8_t *bytes)
{
    return ((uint32_t)bytes[0] << 24U) | ((uint32_t)bytes[1] << 16U) |
           ((uint32_t)bytes[2] << 8U) | (uint32_t)bytes[3];
}

/* Reads the web request record back from the call's full event list after
 * checking the list hashes to the call outcome event's envelope digest. */
static int path_event_list_request(const lxp_receipt *receipt,
                                   uint64_t *request_id, uint8_t *kind,
                                   lxp_byte_span *payload)
{
    static const uint8_t domain[] = "LayerX/programs/events/v1";
    const lxp_byte_span list =
        receipt->program_outcome.event_envelope_payload;
    const uint8_t *digest = NULL;
    uint8_t list_root[32];
    size_t cursor = sizeof(domain);
    size_t found = 0U;
    size_t index;
    uint32_t count;
    PATH_CHECK(list.bytes != NULL && list.length >= sizeof(domain) + 4U);
    for (index = 0U; index < receipt->effects.count; ++index) {
        const lxp_effect *effect = &receipt->effects.effects[index];
        if (effect->module_id != LXP_MODULE_PROGRAMS ||
            effect->kind != LXP_EFFECT_EVENT ||
            effect->event_type != LX_PROGRAMS_EVENT_CALL_OUTCOME)
            continue;
        PATH_CHECK(digest == NULL && effect->body_length == 255U);
        digest = effect->body + effect->body_length - 32U;
    }
    PATH_CHECK(digest != NULL);
    PATH_CHECK(lxp_hash_sha256(list.bytes, list.length, list_root) == LXP_OK &&
               memcmp(list_root, digest, 32U) == 0);
    PATH_CHECK(memcmp(list.bytes, domain, sizeof(domain)) == 0);
    count = path_read_u32(list.bytes + cursor);
    cursor += 4U;
    for (index = 0U; index < count; ++index) {
        uint32_t topic_length;
        uint32_t data_length;
        const uint8_t *topic;
        PATH_CHECK(list.length - cursor >= 32U + 32U + 8U + 1U + 4U);
        cursor += 32U + 32U + 8U + 1U;
        topic_length = path_read_u32(list.bytes + cursor);
        cursor += 4U;
        PATH_CHECK(list.length - cursor >= (size_t)topic_length + 4U);
        topic = list.bytes + cursor;
        cursor += topic_length;
        data_length = path_read_u32(list.bytes + cursor);
        cursor += 4U;
        PATH_CHECK(list.length - cursor >= data_length);
        if (topic_length == sizeof(path_request_topic) - 1U &&
            memcmp(topic, path_request_topic,
                   sizeof(path_request_topic) - 1U) == 0) {
            size_t byte;
            const uint8_t *record = list.bytes + cursor;
            PATH_CHECK(data_length >= 13U);
            PATH_CHECK(path_read_u32(record + 9U) == data_length - 13U);
            *request_id = 0U;
            for (byte = 0U; byte < 8U; ++byte)
                *request_id = (*request_id << 8U) | record[byte];
            *kind = record[8];
            *payload = (lxp_byte_span){record + 13U, data_length - 13U};
            ++found;
        }
        cursor += data_length;
    }
    PATH_CHECK(cursor == list.length && found == 1U);
    return 0;
}

static lx_account *path_account(lx_account_registry *accounts,
                                const uint8_t id[32])
{
    size_t index;
    for (index = 0U; index < accounts->count; ++index)
        if (memcmp(accounts->accounts[index].id, id, 32U) == 0)
            return &accounts->accounts[index];
    return NULL;
}

static size_t path_capabilities(uint8_t *out, const uint8_t asset[32],
                                const uint8_t to[32])
{
    size_t cursor = 0U;
    out[cursor++] = 0U;
    out[cursor++] = 2U;
    out[cursor++] = 3U;
    out[cursor++] = 5U;
    (void)memcpy(out + cursor, asset, 32U);
    cursor += 32U;
    (void)memcpy(out + cursor, to, 32U);
    cursor += 32U;
    (void)memset(out + cursor, 0, 16U);
    out[cursor + 15U] = PATH_FEE;
    return cursor + 16U;
}

static size_t path_request_calldata(uint8_t *out, uint64_t request_id,
                                    const uint8_t asset[32],
                                    const uint8_t to[32])
{
    size_t cursor = 0U;
    out[cursor++] = 1U;
    out[cursor++] = PATH_OP_REQUEST;
    write_u64(out + cursor, request_id);
    cursor += 8U;
    out[cursor++] = PATH_KIND_FETCH;
    (void)memcpy(out + cursor, asset, 32U);
    cursor += 32U;
    (void)memcpy(out + cursor, to, 32U);
    cursor += 32U;
    (void)memset(out + cursor, 0, 16U);
    out[cursor + 15U] = PATH_FEE;
    cursor += 16U;
    (void)memcpy(out + cursor, path_payload, PATH_PAYLOAD_BYTES);
    return cursor + PATH_PAYLOAD_BYTES;
}

static size_t path_read_calldata(uint8_t *out, uint64_t request_id,
                                 const uint8_t digest[32])
{
    size_t cursor = 0U;
    out[cursor++] = 1U;
    out[cursor++] = PATH_OP_READ;
    write_u64(out + cursor, request_id);
    cursor += 8U;
    (void)memcpy(out + cursor, digest, 32U);
    cursor += 32U;
    write_u32(out + cursor, PATH_RESPONSE_BYTES);
    cursor += 4U;
    (void)memcpy(out + cursor, path_response, PATH_RESPONSE_BYTES);
    return cursor + PATH_RESPONSE_BYTES;
}

typedef struct path_chain {
    lxp_kernel kernel;
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_kernel_execution execution;
    lxp_identity_store identities;
    lxp_identity *identity;
    lxp_authority_resolved authority;
    lxp_authority_scope scope;
    lxp_fee_params fees;
    lx_account_registry accounts;
    lxp_transfer_asset_state asset_state;
    lx_programs_transfer_runtime runtime;
    lxp_arena arena;
    uint8_t arena_bytes[2U * LXP_MAX_ACTIVITY_BYTES + 4096U];
    uint8_t primary_key[32];
    uint8_t payer_id[32];
    uint8_t activity_ordinal;
    uint16_t guest_abi;
} path_chain;

static const uint8_t path_did[] = "did:lxp:web-program-path";

static int path_call(path_chain *chain, const uint8_t program_id[32],
                     const uint8_t *capabilities, size_t capabilities_length,
                     const uint8_t *calldata, size_t calldata_length,
                     lxp_receipt *receipt)
{
    static const uint8_t absent_access[] =
        "LayerX/programs/access-declaration/v1\0";
    static uint8_t call[CALL_FIXED_BYTES + 1024U];
    lxp_activity activity;
    lx_account *payer;
    size_t length = call_payload_with_data(call, program_id, capabilities,
                                           capabilities_length, absent_access,
                                           sizeof(absent_access), calldata,
                                           calldata_length);
    write_u16(call + 32U, chain->guest_abi);
    fill_activity(&activity, LX_PROGRAMS_CALL, call, length, path_did,
                  sizeof(path_did) - 1U, chain->primary_key);
    activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    activity.account_sequence = chain->identity->next_sequence;
    activity.idempotency_key[31] = ++chain->activity_ordinal;
    activity.fee_limit = (lxp_u128){0U, 67108864U};
    payer = path_account(&chain->accounts, chain->payer_id);
    PATH_CHECK(payer != NULL);
    chain->execution.fee_balance = payer->balance;
    chain->execution.global_sequence = chain->state.next_sequence;
    PATH_CHECK(lxp_arena_reset(&chain->arena, 0U) == LXP_OK);
    PATH_CHECK(execute_artifact_fixture_activity(&chain->kernel, &activity,
                                                 &chain->execution,
                                                 receipt) == LXP_OK);
    return 0;
}

static int web_program_path(const char *wasm_path, bool genesis_fee_account,
                             uint16_t guest_abi)
{
    static const uint8_t actor_name[] =
        "agent:did:lxp:web-program-path:main";
    static const uint8_t treasury_name[] = "system:fees";
    static const uint8_t actor_seed[32] = {0x33U};
    static const uint8_t grant_id[32] = {0};
    static const uint8_t no_capabilities[] = {0U, 0U};
    static path_chain chain;
    static uint8_t wasm[PATH_WASM_CAPACITY];
    static uint8_t deploy[104U + PATH_WASM_CAPACITY];
    uint8_t capabilities[128];
    uint8_t calldata[PATH_CALLDATA_BYTES];
    uint8_t program_id[32];
    uint8_t code_hash[32];
    uint8_t fee_asset[32] = {9U};
    uint8_t fee_account_id[32];
    uint8_t actor_id[32];
    uint8_t treasury_id[32];
    uint8_t pending_key[PATH_PENDING_KEY_BYTES];
    uint8_t payload_hash[32];
    uint8_t digest[32];
    lx_account *actor;
    lx_account *treasury;
    lx_account *fee_account;
    lxp_activity activity;
    lxp_receipt receipt;
    lxp_u128 actor_before;
    uint64_t parameters = 1U;
    uint64_t paid_sequence;
    size_t accounts_before;
    size_t wasm_length;
    size_t length;

    PATH_CHECK(guest_abi == LX_PROGRAMS_GUEST_ABI_V4_VERSION ||
               guest_abi == LX_PROGRAMS_GUEST_ABI_V5_VERSION);
    if (wasm_path == NULL) {
        wasm_length = path_guest_module(wasm);
        PATH_CHECK(wasm_length != 0U && wasm_length <= sizeof(wasm));
    } else {
        PATH_CHECK(path_read_file(wasm_path, wasm, sizeof(wasm),
                                  &wasm_length) == 0);
    }
    (void)memset(program_id, 0x71, sizeof(program_id));
    (void)memset(&chain, 0, sizeof(chain));
    chain.guest_abi = guest_abi;
    PATH_CHECK(path_fee_account(fee_asset, fee_account_id) == LXP_OK);
    PATH_CHECK(lxp_keccak256(path_payload, PATH_PAYLOAD_BYTES,
                             payload_hash) == LXP_OK);
    PATH_CHECK(executed_public_key(actor_seed, chain.primary_key) == 0);
    PATH_CHECK(lx_account_registry_init(&chain.accounts) == LXP_OK);
    PATH_CHECK(lx_account_id_from_string(actor_name, sizeof(actor_name) - 1U,
                                         actor_id) == LXP_OK);
    (void)memcpy(chain.payer_id, actor_id, 32U);
    PATH_CHECK(lx_account_id_from_string(treasury_name,
                                         sizeof(treasury_name) - 1U,
                                         treasury_id) == LXP_OK);
    PATH_CHECK(lx_account_open(&chain.accounts, actor_name,
        sizeof(actor_name) - 1U, actor_id, 1U, LX_ACCOUNT_OPEN_GENESIS, NULL,
        &actor) == LXP_OK);
    PATH_CHECK(lx_account_open(&chain.accounts, treasury_name,
        sizeof(treasury_name) - 1U, treasury_id, 2U, LX_ACCOUNT_OPEN_GENESIS,
        NULL, &treasury) == LXP_OK);
    PATH_CHECK(lxp_ledger_bootstrap_balance(actor, fee_asset,
        (lxp_u128){0U, UINT64_MAX}, 1U) == LXP_OK);
    PATH_CHECK(lxp_ledger_bootstrap_balance(treasury, fee_asset,
        (lxp_u128){0U, 0U}, 0U) == LXP_OK);
    if (genesis_fee_account) {
        static const uint8_t programs_name[] = "programs";
        lx_account_registration fee_registration;
        bool fee_created = false;
        PATH_CHECK(lx_account_module_value_prepare(&chain.accounts,
                       programs_name, sizeof(programs_name) - 1U,
                       fee_account_id, fee_asset, 3U,
                       &fee_registration, &fee_account,
                       &fee_created) == LXP_OK &&
                   fee_created);
        PATH_CHECK(lx_account_registration_commit(&chain.accounts,
                       &fee_registration, &fee_account) == LXP_OK);
    }
    actor = path_account(&chain.accounts, actor_id);
    treasury = path_account(&chain.accounts, treasury_id);
    PATH_CHECK(actor != NULL && treasury != NULL);
    PATH_CHECK(lxp_did_id_derive(path_did, sizeof(path_did) - 1U,
                                 chain.authority.principal) == LXP_OK);
    (void)memcpy(chain.authority.actor, chain.authority.principal, 32U);
    (void)memcpy(chain.authority.verified_key, chain.primary_key, 32U);
    chain.authority.kind = LXP_AUTHORITY_OWNER;
    chain.scope.module_mask = UINT64_C(1) << LXP_MODULE_PROGRAMS;
    chain.scope.activity_ordinal_min = 1U;
    chain.scope.activity_ordinal_max = 7U;
    chain.scope.maximum_per_activity = (lxp_u128){UINT64_MAX, UINT64_MAX};
    chain.scope.maximum_total = chain.scope.maximum_per_activity;
    chain.scope.maximum_per_period = chain.scope.maximum_per_activity;
    chain.authority.scope = &chain.scope;
    PATH_CHECK(lxp_authority_hash(chain.authority.kind, grant_id,
                                  chain.primary_key,
                                  chain.authority.authority_hash) == LXP_OK);
    (void)memcpy(chain.asset_state.asset_id, fee_asset, 32U);
    chain.asset_state.registered = true;
    chain.runtime.accounts = &chain.accounts;
    chain.runtime.assets = &chain.asset_state;
    chain.runtime.asset_count = 1U;
    chain.runtime.fee_schedule = (lx_programs_fee_schedule){
        1U, 1U, 1U, 2U, 4U, 1U, 1U, 1U
    };
    chain.runtime.resolve_metering_schedule =
        lxp_programs_metering_resolve_runtime;
    chain.runtime.metering_schedule_context = &chain.kernel;
    (void)memcpy(chain.runtime.occupancy_asset_id, fee_asset, 32U);
    chain.runtime.resolve_occupancy_parameters = occupancy_parameters;
    chain.runtime.occupancy_parameter_context = &chain.runtime;
    chain.fees.version = 1U;
    chain.fees.multiplier_basis_points = 10000U;
    PATH_CHECK(lxp_state_store_init(&chain.state, 1U) == LXP_OK);
    PATH_CHECK(lxp_identity_register(&chain.identities, path_did,
                                     sizeof(path_did) - 1U,
                                     chain.primary_key,
                                     &chain.identity) == LXP_OK);
    PATH_CHECK(lxp_kernel_create(&chain.kernel, &chain.state, &chain.journal,
                                 &parameters, 0U) == LXP_OK);
    PATH_CHECK(install_metering_v1(&chain.kernel) == LXP_OK);
    PATH_CHECK(lxp_kernel_register_module(&chain.kernel,
        guest_abi == LX_PROGRAMS_GUEST_ABI_V5_VERSION ?
            programs_module_registration_v5() :
            programs_module_registration_v4()) == LXP_OK);
    PATH_CHECK(lxp_programs_module_version(&chain.kernel) == guest_abi);
    PATH_CHECK(lxp_kernel_bind_module_runtime(&chain.kernel,
        LXP_MODULE_PROGRAMS, &chain.runtime) == LXP_OK);
    PATH_CHECK(lxp_programs_bind_fee_transaction(&chain.kernel) == LXP_OK);
    PATH_CHECK(lxp_kernel_set_capabilities(&chain.kernel, NULL,
        lxp_kernel_canonical_ledger_apply) == LXP_OK);
    PATH_CHECK(lxp_state_root(&chain.kernel,
                              chain.kernel.current_state_root) == LXP_OK);
    PATH_CHECK(lxp_arena_init(&chain.arena, chain.arena_bytes,
                              sizeof(chain.arena_bytes)) == LXP_OK);
    chain.execution.network_id = PATH_NETWORK;
    chain.execution.batch_number = 1U;
    chain.execution.batch_timestamp_ms = 10U;
    chain.execution.maximum_timestamp_window = 100U;
    chain.execution.recorded_module_version = guest_abi;
    chain.execution.parameter_version = 1U;
    chain.execution.signature_valid = true;
    chain.execution.identities = &chain.identities;
    chain.execution.authority = &chain.authority;
    chain.execution.fee_parameters = &chain.fees;
    chain.execution.gas_limit = 1000000U;
    chain.execution.arena = &chain.arena;

    /* Deploy the real web-reader guest at the selected registered ABI. */
    length = program_spend_deploy_payload(deploy, program_id,
                                          chain.authority.principal, wasm,
                                          wasm_length, code_hash);
    write_u16(deploy + 32U, guest_abi);
    fill_activity(&activity, LX_PROGRAMS_DEPLOY, deploy, length, path_did,
                  sizeof(path_did) - 1U, chain.primary_key);
    activity.protocol_version = LXP_PROTOCOL_VERSION_STATE_COMMITMENT;
    activity.account_sequence = chain.identity->next_sequence;
    activity.idempotency_key[31] = ++chain.activity_ordinal;
    chain.execution.global_sequence = chain.state.next_sequence;
    PATH_CHECK(lxp_arena_reset(&chain.arena, 0U) == LXP_OK);
    PATH_CHECK(execute_artifact_fixture_activity(&chain.kernel, &activity,
                                                 &chain.execution,
                                                 &receipt) == LXP_OK);
    PATH_CHECK(receipt.result_code == LXP_OK);
    actor = path_account(&chain.accounts, actor_id);
    PATH_CHECK(actor != NULL);

    /* The retired web intake no longer restricts the event's payment target;
     * the surviving program transfer still pays the named account. */
    length = path_capabilities(capabilities, fee_asset, treasury_id);
    PATH_CHECK(path_call(&chain, program_id, capabilities, length, calldata,
                         path_request_calldata(calldata, path_unpaid_request,
                                               fee_asset, treasury_id),
                         &receipt) == 0);
    PATH_CHECK(receipt.result_code == LXP_OK);
    treasury = path_account(&chain.accounts, treasury_id);
    PATH_CHECK(treasury != NULL && treasury->balance.hi == 0U &&
               treasury->balance.lo >= PATH_FEE);
    path_pending_key(program_id, path_unpaid_request, pending_key);
    PATH_CHECK(path_committed(&chain.kernel, pending_key,
                              sizeof(pending_key)) == NULL);

    /* Reading before any answer is committed refuses the call. */
    (void)memset(digest, 0x5a, sizeof(digest));
    PATH_CHECK(path_call(&chain, program_id, no_capabilities,
                         sizeof(no_capabilities), calldata,
                         path_read_calldata(calldata, path_request, digest),
                         &receipt) == 0);
    PATH_CHECK(receipt.result_code == LXP_ERR_PROGRAM_REFUSED);

    /* Payment and the emitted request survive the retirement of the kernel's
     * pending web store. */
    actor = path_account(&chain.accounts, actor_id);
    PATH_CHECK(actor != NULL);
    actor_before = actor->balance;
    PATH_CHECK((path_account(&chain.accounts, fee_account_id) != NULL) ==
               genesis_fee_account);
    accounts_before = chain.accounts.count;
    paid_sequence = chain.state.next_sequence;
    length = path_capabilities(capabilities, fee_asset, fee_account_id);
    PATH_CHECK(path_call(&chain, program_id, capabilities, length, calldata,
                         path_request_calldata(calldata, path_request,
                                               fee_asset, fee_account_id),
                         &receipt) == 0);
    PATH_CHECK(receipt.result_code == LXP_OK);
    fee_account = path_account(&chain.accounts, fee_account_id);
    PATH_CHECK(fee_account != NULL &&
               fee_account->kind == LX_ACCOUNT_MODULE_VALUE &&
               fee_account->has_asset &&
               memcmp(fee_account->asset_id, fee_asset, 32U) == 0 &&
               fee_account->balance.hi == 0U &&
               fee_account->balance.lo == PATH_FEE);
    if (genesis_fee_account) {
        PATH_CHECK(chain.accounts.count == accounts_before);
    } else {
        PATH_CHECK(chain.accounts.count == accounts_before + 1U &&
                   fee_account->created_at_sequence == paid_sequence);
    }
    actor = path_account(&chain.accounts, actor_id);
    PATH_CHECK(actor != NULL && actor->balance.lo < actor_before.lo);
    path_pending_key(program_id, path_request, pending_key);
    PATH_CHECK(path_committed(&chain.kernel, pending_key,
                              sizeof(pending_key)) == NULL);
    {
        /* The real call's event list carries the exact request and payload. */
        uint64_t listed_request = 0U;
        uint8_t listed_kind = 0U;
        lxp_byte_span listed_payload = {NULL, 0U};
        uint8_t listed_hash[32];
        PATH_CHECK(path_event_list_request(&receipt, &listed_request,
                                           &listed_kind,
                                           &listed_payload) == 0);
        PATH_CHECK(listed_request == path_request &&
                   listed_kind == PATH_KIND_FETCH &&
                   listed_payload.length == PATH_PAYLOAD_BYTES &&
                   memcmp(listed_payload.bytes, path_payload,
                          PATH_PAYLOAD_BYTES) == 0);
        PATH_CHECK(lxp_keccak256(listed_payload.bytes, listed_payload.length,
                                 listed_hash) == LXP_OK &&
                   memcmp(listed_hash, payload_hash, 32U) == 0);
    }
#ifdef LXP_TEST_WEB_REQUEST_OBSERVER
    PATH_CHECK(LXP_TEST_WEB_REQUEST_OBSERVER(&receipt) == 0);
#endif

    /* With kernel intake retired, an event id is no longer a pending key. */
    PATH_CHECK(path_call(&chain, program_id, capabilities, length, calldata,
                         path_request_calldata(calldata, path_request,
                                               fee_asset, fee_account_id),
                         &receipt) == 0);
    PATH_CHECK(receipt.result_code == LXP_OK);
    fee_account = path_account(&chain.accounts, fee_account_id);
    PATH_CHECK(fee_account != NULL && fee_account->balance.hi == 0U &&
               fee_account->balance.lo == 2U * PATH_FEE);
    PATH_CHECK(path_committed(&chain.kernel, pending_key,
                              sizeof(pending_key)) == NULL);

    PATH_CHECK(lxp_state_store_destroy(&chain.state) == LXP_OK);
    return 0;
}

#ifndef LXP_TEST_WEB_PROGRAM_PATH_MAIN
#define LXP_TEST_WEB_PROGRAM_PATH_MAIN main
#endif

static int current_paid_web_request(void)
{
    return web_program_path(NULL, true, LX_PROGRAMS_GUEST_ABI_V5_VERSION);
}

static int current_paid_web_request_account_creation(void)
{
    return web_program_path(NULL, false, LX_PROGRAMS_GUEST_ABI_V5_VERSION);
}

static int historical_paid_web_request(void)
{
    return web_program_path(NULL, true, LX_PROGRAMS_GUEST_ABI_V4_VERSION);
}

static int historical_paid_web_request_account_creation(void)
{
    return web_program_path(NULL, false, LX_PROGRAMS_GUEST_ABI_V4_VERSION);
}

int LXP_TEST_WEB_PROGRAM_PATH_MAIN(int argc, char **argv)
{
    if (lxp_test_register("current_paid_web_request",
            current_paid_web_request) != 0 ||
        lxp_test_register("current_paid_web_request_account_creation",
            current_paid_web_request_account_creation) != 0 ||
        lxp_test_register("historical_paid_web_request",
            historical_paid_web_request) != 0 ||
        lxp_test_register("historical_paid_web_request_account_creation",
            historical_paid_web_request_account_creation) != 0)
        return 1;
    if (argc == 1 || (argc > 1 && argv[1][0] == '-'))
        return lxp_test_main(argc, argv);
    if (argc != 2) {
        (void)fprintf(stderr, "usage: %s web-reader.wasm\n", argv[0]);
        return 2;
    }
    if (web_program_path(argv[1], true,
                         LX_PROGRAMS_GUEST_ABI_V5_VERSION) != 0 ||
        web_program_path(argv[1], false,
                         LX_PROGRAMS_GUEST_ABI_V5_VERSION) != 0)
        return 1;
    /* Explicitly historical ABI-v4 compatibility, with the same assertions. */
    if (web_program_path(argv[1], true,
                         LX_PROGRAMS_GUEST_ABI_V4_VERSION) != 0)
        return 1;
    return web_program_path(argv[1], false,
                             LX_PROGRAMS_GUEST_ABI_V4_VERSION);
}
