/* Fixture recorder only; the Go gate reads its committed JSON output. */
#include "lxphost_native.h"
#include "layerx/lxp_snapshot.h"
#include "layerx/lxp_state_proof.h"
#include "../../../src/state/lxp_state_internal.h"

#include <inttypes.h>
#include <stdio.h>

#define CHECK(expression) do { \
    if (!(expression)) { \
        fprintf(stderr, "%s:%d: %s\n", __FILE__, __LINE__, #expression); \
        exit(1); \
    } \
} while (0)

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

static void emit(lxphost_node *node)
{
    uint8_t root[32];
    size_t width;
    CHECK(lxp_state_root(&node->kernel, root) == LXP_OK);
    CHECK(lxp_state_module_root_count(&node->kernel, &width) == LXP_OK);
    printf("{\"root\":");
    hex(root, sizeof(root));
    printf(",\"width\":%zu,\"next_sequence\":%" PRIu64 ",\"modules\":[",
           width, node->store.next_sequence);
    for (size_t i = 0U; i < node->kernel.module_count; ++i)
        printf("%s%u", i == 0U ? "" : ",", node->kernel.modules[i].module_id);
    printf("],\"accounts\":[");
    for (size_t i = 0U; i < node->accounts.count; ++i) {
        const lx_account *account = &node->accounts.accounts[i];
        uint8_t key[33] = {4U};
        uint8_t *wire = malloc(LXP_STATE_WITNESS_MAX_BYTES);
        lxp_state_witness *witness = malloc(sizeof(*witness));
        size_t length;
        CHECK(wire != NULL && witness != NULL);
        memcpy(key + 1U, account->id, 32U);
        CHECK(lxp_state_proof_build(&node->kernel, 0U,
              (lxp_byte_span){key, sizeof(key)}, witness) == LXP_OK);
        CHECK(lxp_state_proof_verify(witness, root) == LXP_OK);
        CHECK(lxp_state_proof_encode(witness, wire,
              LXP_STATE_WITNESS_MAX_BYTES, &length) == LXP_OK);
        printf("%s{\"id\":", i == 0U ? "" : ",");
        hex(account->id, 32U);
        printf(",\"name\":");
        hex(account->name, account->name_length);
        printf(",\"kind\":%u,\"asset\":", account->kind);
        hex(account->asset_id, 32U);
        printf(",\"balance\":{\"Hi\":%" PRIu64 ",\"Lo\":%" PRIu64 "},"
               "\"proof\":", account->balance.hi, account->balance.lo);
        hex(wire, length);
        printf(",\"key\":");
        hex(witness->key, witness->key_length);
        printf(",\"value\":");
        hex(witness->value, witness->value_length);
        printf(",\"account_path\":");
        path(&witness->account_path);
        printf(",\"module_path\":");
        path(&witness->layer_a);
        printf(",\"composite_path\":");
        path(&witness->layer_b);
        putchar('}');
        free(witness);
        free(wire);
    }
    printf("]}");
}

int main(int argc, char **argv)
{
    int32_t status = LXP_OK;
    lxphost_node *node;
    lxp_module_ctx *ctx = calloc(1U, sizeof(*ctx));
    uint8_t *snapshot_memory = malloc(4194304U);
    lxp_arena snapshot_arena;
    lxp_byte_span snapshot;
    lxp_snapshot_manifest_record manifest, loaded_manifest;
    uint8_t root[32], receipt_root[32] = {0x91U};
    char filename[4096];
    CHECK(argc == 2 && ctx != NULL && snapshot_memory != NULL);
    node = lxphost_node_new(0U, 0U, &status);
    CHECK(node != NULL && status == LXP_OK);
    CHECK(lxphost_reset(node, 1U) == LXP_OK);
    for (size_t i = 0U; i < 3U; ++i) {
        const char *names[] = {"agent:did:key:a:main", "agent:did:key:b:main",
                               "agent:did:key:c:main"};
        uint8_t id[32], asset[32] = {(uint8_t)(i + 1U)};
        lx_account *account = NULL;
        CHECK(lx_account_id_from_string((const uint8_t *)names[i],
              strlen(names[i]), id) == LXP_OK);
        CHECK(lx_account_open(&node->accounts, (const uint8_t *)names[i],
              strlen(names[i]), id, 1U, LX_ACCOUNT_OPEN_CREDIT, NULL,
              &account) == LXP_OK);
        CHECK(lxp_ledger_bootstrap_balance(account, asset,
              (lxp_u128){i + 1U, 100U + i}, 0U) == LXP_OK);
    }
    CHECK(lxp_state_journal_open(&node->store, 1U, &node->journal) == LXP_OK);
    CHECK(lxp_state_journal_commit(&node->journal) == LXP_OK);
    for (size_t i = 0U; i < node->kernel.module_count; ++i) {
        uint16_t module = node->kernel.modules[i].module_id;
        CHECK(lxp_arena_reset(&node->arena, 0U) == LXP_OK);
        CHECK(lxp_module_ctx_init(ctx, &node->kernel, module, 2U, 0U, 1U,
              100000U, &node->arena, true) == LXP_OK);
        for (uint8_t j = 1U; j <= 3U; ++j) {
            uint8_t value[] = {(uint8_t)module, j, 0xa5U};
            CHECK(lxp_ctx_kv_put(ctx, &j, 1U, value, sizeof(value)) == LXP_OK);
        }
        CHECK(lxp_module_ctx_commit(ctx) == LXP_OK);
    }
    CHECK(lxp_state_root(&node->kernel, root) == LXP_OK);
    printf("{\"producer\":\"modules/lxphost/host.go:lxphost_reset\","
           "\"global_sequence\":1,\"before\":");
    emit(node);
    CHECK(lxp_arena_init(&snapshot_arena, snapshot_memory, 4194304U) == LXP_OK);
    CHECK(lxp_snapshot_write(&node->kernel, 1U, &snapshot_arena, &snapshot) == LXP_OK);
    CHECK(lxp_snapshot_manifest_build(snapshot.bytes, snapshot.length,
          1U, root, receipt_root, &manifest) == LXP_OK);
    CHECK(lxp_snapshot_store_write(argv[1], &manifest,
          snapshot.bytes, snapshot.length) == LXP_OK);
    printf(",\"snapshot\":");
    hex(snapshot.bytes, snapshot.length);
    printf(",\"snapshot_digest\":");
    hex(manifest.snapshot_digest, 32U);
    printf(",\"receipt_root\":");
    hex(manifest.receipt_state_root, 32U);
    lxphost_node_free(node);
    node = NULL;
    CHECK(lxp_arena_reset(&snapshot_arena, 0U) == LXP_OK);
    CHECK(snprintf(filename, sizeof(filename), "%s/%020u.lxs", argv[1], 1U) > 0);
    CHECK(lxp_snapshot_store_read(filename, &snapshot_arena,
          &loaded_manifest, &snapshot) == LXP_OK);
    node = lxphost_node_new(0U, 0U, &status);
    CHECK(node != NULL && status == LXP_OK);
    CHECK(lxphost_reset(node, 1U) == LXP_OK);
    CHECK(lxp_snapshot_load(snapshot.bytes, snapshot.length,
          &loaded_manifest, &node->kernel) == LXP_OK);
    CHECK(lxp_snapshot_verify_root(&node->kernel, &loaded_manifest) == LXP_OK);
    CHECK(memcmp(root, loaded_manifest.canonical_state_root, 32U) == 0);
    CHECK(memcmp(node->kernel.current_state_root, receipt_root, 32U) == 0);
    printf(",\"after\":");
    emit(node);
    printf("}\n");
    lxphost_node_free(node);
    free(snapshot_memory);
    free(ctx);
    return 0;
}
