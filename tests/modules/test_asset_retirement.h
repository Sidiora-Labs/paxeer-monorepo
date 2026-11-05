#ifndef LAYERX_TEST_ASSET_RETIREMENT_H
#define LAYERX_TEST_ASSET_RETIREMENT_H

#include "../ledger/test_values.h"
#include "../../src/modules/asset/lx_asset_history.h"
#include "../../src/modules/asset/lx_asset_native.h"
#include "layerx/lxp_kernel.h"
#include "layerx/lxp_snapshot.h"
#include "layerx/lxp_state_proof.h"

#include <stdlib.h>
#include <string.h>

static inline int asset_history_reopen(lxp_kernel *kernel, lxp_state_store *state,
    lxp_state_journal *journal, lx_account_registry *accounts,
    const void *parameters, uint64_t sequence)
{
    uint8_t *bytes = malloc(1048576U);
    lxp_arena arena;
    lxp_byte_span snapshot;
    lxp_snapshot_manifest_record manifest;
    uint8_t before[32], after[32];
    FILE *file = NULL;
    size_t length = 0U;
    int failed = 1;
    if (!TEST_NE(bytes, NULL) ||
        !TEST_EQ(lxp_state_root(kernel, before), LXP_OK) ||
        !TEST_EQ(lxp_arena_init(&arena, bytes, 1048576U), LXP_OK) ||
        !TEST_EQ(lxp_snapshot_write(kernel, sequence, &arena, &snapshot), LXP_OK) ||
        !TEST_EQ(lxp_snapshot_manifest_build(snapshot.bytes, snapshot.length,
            sequence, before, kernel->current_state_root, &manifest), LXP_OK)) goto finish;
    file = tmpfile();
    if (!TEST_NE(file, NULL)) goto finish;
    length = snapshot.length;
    if (!TEST_EQ(fwrite(snapshot.bytes, 1U, length, file), length) ||
        !TEST_EQ(fflush(file), 0) || !TEST_EQ(fseek(file, 0L, SEEK_SET), 0) ||
        !TEST_EQ(lxp_state_store_destroy(state), LXP_OK)) goto finish;
    memset(kernel, 0, sizeof(*kernel));
    lx_account_registry_release(accounts);
    memset(accounts, 0, sizeof(*accounts));
    memset(journal, 0, sizeof(*journal));
    if (!TEST_EQ(lx_account_registry_init(accounts), LXP_OK) ||
        !TEST_EQ(lxp_state_store_init(state, 0U), LXP_OK) ||
        !TEST_EQ(lxp_state_store_bind_accounts(state, accounts), LXP_OK) ||
        !TEST_EQ(lxp_state_store_require_account_root(state), LXP_OK) ||
        !TEST_EQ(lxp_kernel_create(kernel, state, journal, parameters, 0U), LXP_OK) ||
        !TEST_EQ(lxp_kernel_register_module(kernel, lx_asset_module_iface()), LXP_OK) ||
        !TEST_EQ(lxp_kernel_set_capabilities(kernel, NULL,
            lxp_kernel_canonical_ledger_apply), LXP_OK)) goto finish;
    memset(bytes, 0, 1048576U);
    if (!TEST_EQ(fread(bytes, 1U, length, file), length) ||
        !TEST_EQ(lxp_snapshot_load(bytes, length, &manifest, kernel), LXP_OK) ||
        !TEST_EQ(lxp_snapshot_verify_root(kernel, &manifest), LXP_OK) ||
        !TEST_EQ(lxp_state_root(kernel, after), LXP_OK) ||
        !TEST_EQ(memcmp(before, after, 32U), 0)) goto finish;
    failed = 0;
finish:
    if (file != NULL && fclose(file) != 0) failed = 1;
    free(bytes);
    return failed;
}

static inline int asset_history_proof(lxp_kernel *kernel,
    const lx_withdrawal_request *request, int vectors)
{
    uint8_t key[LX_WITHDRAWAL_STATE_KEY_BYTES];
    uint8_t value[LX_WITHDRAWAL_STATE_VALUE_BYTES];
    uint8_t root[32];
    lx_withdrawal_record record;
    lxp_state_witness *proof = malloc(sizeof(*proof));
    int failed = 1;
    if (!TEST_NE(proof, NULL) ||
        !TEST_EQ(lx_asset_historical_withdrawal_encode_v2(request, key, value), LXP_OK) ||
        !TEST_EQ(lxp_state_root(kernel, root), LXP_OK) ||
        !TEST_EQ(lxp_state_proof_build(kernel, LXP_MODULE_ASSET,
            (lxp_byte_span){key, sizeof(key)}, proof), LXP_OK) ||
        !TEST_EQ(lxp_state_proof_verify(proof, root), LXP_OK) ||
        !TEST_EQ(lx_asset_historical_withdrawal_decode_v2(proof->key,
            proof->key_length, proof->value, proof->value_length, &record), LXP_OK) ||
        !TEST_EQ(memcmp(value, proof->value, sizeof(value)), 0) ||
        !TEST_EQ(memcmp(record.nullifier, key + 11U, 32U), 0) ||
        !TEST_EQ(lxp_u128_cmp(record.request.amount, request->amount), 0) ||
        !TEST_EQ(memcmp(record.request.payout_recipient, request->payout_recipient, 32U), 0) ||
        !TEST_EQ(record.settled, false)) goto finish;
    if (vectors != 0) {
        uint8_t *encoded = malloc(LXP_STATE_WITNESS_MAX_BYTES);
        size_t length = 0U;
        if (!TEST_NE(encoded, NULL)) goto finish;
        if (lxp_state_proof_encode(proof, encoded, LXP_STATE_WITNESS_MAX_BYTES,
                                  &length) != LXP_OK) {
            free(encoded);
            goto finish;
        }
        printf("{\"historical_version\":2,\"root\":\"0x");
        for (size_t i = 0U; i < 32U; ++i) printf("%02x", root[i]);
        printf("\",\"proof\":\"0x");
        for (size_t i = 0U; i < length; ++i) printf("%02x", encoded[i]);
        printf("\"}\n");
        free(encoded);
    }
    failed = 0;
finish:
    free(proof);
    return failed;
}

#endif
