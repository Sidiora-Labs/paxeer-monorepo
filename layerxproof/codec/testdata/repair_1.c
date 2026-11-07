#include "lxphost_native.h"
#include "layerx/lxp_state_proof.h"

#include <assert.h>
#include <stdio.h>

static void hex(const uint8_t *bytes, size_t length)
{
    putchar('"');
    for (size_t i = 0U; i < length; ++i) printf("%02x", bytes[i]);
    putchar('"');
}

static void path(const lxp_state_proof *proof)
{
    printf("{\"index\":%u,\"count\":%u,\"siblings\":[",
           proof->leaf_index, proof->leaf_count);
    for (size_t i = 0U; i < proof->depth; ++i) {
        if (i != 0U) putchar(',');
        hex(proof->siblings[i], 32U);
    }
    printf("]}");
}

static void emit_witness(const lxp_state_witness *proof, const uint8_t root[32],
                         const char *name, bool *first)
{
    lxp_state_witness *decoded = malloc(sizeof(*decoded));
    uint8_t *wire = malloc(LXP_STATE_WITNESS_MAX_BYTES);
    uint8_t leaf[32], lengths[8];
    size_t length = 0U, tag_length = 0U;
    lxp_hash_context hash;
    const uint8_t *tag = lxp_domain_tag(LXP_DOMAIN_STATE_LEAF, &tag_length);
    assert(decoded != NULL && wire != NULL && tag != NULL);
    assert(lxp_state_proof_verify(proof, root) == LXP_OK);
    assert(lxp_state_proof_encode(proof, wire,
        LXP_STATE_WITNESS_MAX_BYTES, &length) == LXP_OK);
    assert(lxp_state_proof_decode(wire, length, decoded) == LXP_OK);
    assert(lxp_state_proof_verify(decoded, root) == LXP_OK);
    for (size_t i = 0U; i < 4U; ++i) {
        lengths[i] = (uint8_t)(proof->key_length >> (8U * (3U - i)));
        lengths[i + 4U] = (uint8_t)(proof->value_length >> (8U * (3U - i)));
    }
    lxp_hash_init(&hash);
    assert(lxp_hash_update(&hash, tag, tag_length) == LXP_OK);
    assert(lxp_hash_update(&hash, lengths, sizeof(lengths)) == LXP_OK);
    assert(lxp_hash_update(&hash, proof->key, proof->key_length) == LXP_OK);
    assert(lxp_hash_update(&hash, proof->value, proof->value_length) == LXP_OK);
    assert(lxp_hash_final(&hash, leaf) == LXP_OK);
    printf("%s{\"name\":\"%s\",\"module\":%u,\"key\":",
           *first ? "" : ",", name, proof->module_id);
    hex(proof->key, proof->key_length);
    printf(",\"value\":");
    hex(proof->value, proof->value_length);
    printf(",\"leaf\":");
    hex(leaf, sizeof(leaf));
    printf(",\"root\":");
    hex(root, 32U);
    printf(",\"proof\":");
    hex(wire, length);
    printf(",\"account\":");
    path(&proof->account_path);
    printf(",\"a\":");
    path(&proof->layer_a);
    printf(",\"b\":");
    path(&proof->layer_b);
    putchar('}');
    *first = false;
    free(wire);
    free(decoded);
}

static void emit(const lxp_kernel *kernel, const char *name, uint16_t module,
                 const uint8_t *key, size_t key_length, bool *first)
{
    lxp_state_witness *proof = malloc(sizeof(*proof));
    uint8_t root[32];
    assert(proof != NULL);
    assert(lxp_state_root(kernel, root) == LXP_OK);
    assert(lxp_state_proof_build(kernel, module,
        (lxp_byte_span){key, key_length}, proof) == LXP_OK);
    emit_witness(proof, root, name, first);
    free(proof);
}

int main(void)
{
    int32_t status = LXP_OK;
    lxphost_node *node = lxphost_node_new(0U, LX_PROGRAMS_GUEST_ABI_V5_VERSION, &status);
    lxp_module_ctx *ctx = calloc(1U, sizeof(*ctx));
    uint8_t balance_key[32] = {0xa5U};
    uint8_t asset[32] = {1U};
    bool first = true;
    assert(node != NULL && ctx != NULL && status == LXP_OK);
    assert(lxphost_reset(node, 1U) == LXP_OK);
    for (size_t i = 0U; i < 3U; ++i) {
        const char *names[] = {"agent:did:key:a:main", "agent:did:key:b:main",
                               "agent:did:key:c:main"};
        uint8_t id[32];
        lx_account *account = NULL;
        assert(lx_account_id_from_string((const uint8_t *)names[i],
            strlen(names[i]), id) == LXP_OK);
        assert(lx_account_open(&node->accounts, (const uint8_t *)names[i],
            strlen(names[i]), id, 1U, LX_ACCOUNT_OPEN_CREDIT, NULL,
            &account) == LXP_OK);
        assert(lxp_ledger_bootstrap_balance(account, asset,
            (lxp_u128){0U, 100U + i}, 0U) == LXP_OK);
    }
    assert(lxp_state_journal_open(&node->store, 1U, &node->journal) == LXP_OK);
    assert(lxp_state_journal_set(&node->journal, balance_key,
        (lxp_u128){1U, 2U}) == LXP_OK);
    assert(lxp_state_journal_commit(&node->journal) == LXP_OK);
    for (size_t i = 0U; i < node->kernel.module_count; ++i) {
        uint16_t module = node->kernel.modules[i].module_id;
        assert(lxp_arena_reset(&node->arena, 0U) == LXP_OK);
        assert(lxp_module_ctx_init(ctx, &node->kernel, module, 2U, 0U, 0U,
            100000U, &node->arena, true) == LXP_OK);
        for (uint8_t j = 1U; j <= 3U; ++j) {
            uint8_t value[] = {(uint8_t)module, j, 0xa5U};
            assert(lxp_ctx_kv_put(ctx, &j, 1U, value, sizeof(value)) == LXP_OK);
        }
        assert(lxp_module_ctx_commit(ctx) == LXP_OK);
    }
    printf("{\"max_module\":%u,\"spot\":%u,\"service\":%u,\"modules\":[",
           LXP_MODULE_RESERVED_COUNT, LXP_MODULE_SPOT, LXP_MODULE_SERVICE);
    for (size_t i = 0U; i < node->kernel.module_count; ++i)
        printf("%s%u", i == 0U ? "" : ",", node->kernel.modules[i].module_id);
    printf("],\"vectors\":[");
    for (size_t i = 0U; i < node->accounts.count; ++i) {
        uint8_t key[33] = {4U};
        char name[32];
        memcpy(key + 1U, node->accounts.accounts[i].id, 32U);
        assert(snprintf(name, sizeof(name), "account_%zu", i) > 0);
        emit(&node->kernel, name, 0U, key, sizeof(key), &first);
    }
    {
        uint8_t key[33] = {1U};
        memcpy(key + 1U, balance_key, 32U);
        emit(&node->kernel, "balance", 0U, key, sizeof(key), &first);
    }
    emit(&node->kernel, "sequence", 0U, (const uint8_t *)"sequence", 8U, &first);
    for (size_t i = 0U; i < node->kernel.module_count; ++i) {
        uint16_t module = node->kernel.modules[i].module_id;
        for (uint8_t j = 1U; j <= 3U; ++j) {
            char name[32];
            assert(snprintf(name, sizeof(name), "module_%u_leaf_%u", module, j) > 0);
            emit(&node->kernel, name, module, &j, 1U, &first);
        }
    }
    printf("]}\n");
    free(ctx);
    lxphost_node_free(node);
    return 0;
}
