//go:build cgo

// Package m909fixture produces native Programs bootstrap records for host path tests.
package m909fixture

/*
#cgo CFLAGS: -I${SRCDIR}/../../../../include -I${SRCDIR}/../../../../build/generated
#cgo LDFLAGS: -L${SRCDIR}/../../../../build -L${SRCDIR}/../../../../programs/target/release -L${SRCDIR}/../../../../programs/target/debug -llayerx -llayerx_programs_sandbox -llayerx -lssl -lcrypto -pthread -ldl -lm
#include <stdlib.h>
#include <string.h>
#include "layerx/programs.h"
#include "layerx/lxp_genesis.h"
#include "layerx/lxp_module_ctx.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_snapshot.h"

static int32_t account_record(const uint8_t *name, size_t name_length,
    const uint8_t *asset, const uint8_t *key, uint64_t balance, uint64_t sequence,
    uint8_t *out, size_t *length)
{
    lx_account_registry registry;
    lx_account *account = NULL;
    uint8_t id[32];
    int32_t status = lx_account_registry_init(&registry);
    if (status != LXP_OK) return status;
    status = lx_account_id_from_string(name, name_length, id);
    if (status == LXP_OK) status = lx_account_open(&registry, name, name_length,
        id, 1, LX_ACCOUNT_OPEN_GENESIS, NULL, &account);
    if (status == LXP_OK) status = lxp_ledger_bootstrap_balance(account, asset,
        (lxp_u128){0, balance}, sequence);
    if (status == LXP_OK && key != NULL) {
        memcpy(account->authority_key, key, 32);
        account->has_authority_key = true;
    }
    if (status == LXP_OK) status = lx_account_validate_canonical(account);
    if (status == LXP_OK) status = lx_account_record_encode(account, out, *length, length);
    lx_account_registry_release(&registry);
    return status;
}

static int32_t records(lxp_kernel *kernel, const uint8_t *asset,
    const uint8_t *controller, const uint8_t *key_a, const uint8_t *key_b,
    const uint8_t *program)
{
    lxp_genesis_manifest *manifest = calloc(1, sizeof(*manifest));
    lxp_state_store *state = calloc(1, sizeof(*state));
    lxp_state_journal *journal = calloc(1, sizeof(*journal));
    lxp_module_ctx ctx;
    lxp_arena arena;
    uint8_t arena_bytes[16384];
    uint64_t parameters = 1;
    lx_programs_fee_genesis_parameters fees = {0};
    lxp_programs_retirement_profile profile = {0};
    lx_programs_metering_schedule metering = {0};
    int32_t status = LXP_ERR_ARENA_EXHAUSTED;
    if (!manifest || !state || !journal) goto done;
    fees.schedule = (lx_programs_fee_schedule){1,1,1,1,1,1,1,1};
    memcpy(fees.occupancy_asset_id, asset, 32);
    fees.target_occupancy_byte_batches = 1000;
    fees.response_denominator = 8;
    fees.maximum_change_numerator = 1;
    fees.maximum_change_denominator = 8;
    fees.minimum_fee_units_per_occupancy_byte_batch = 1;
    fees.maximum_fee_units_per_occupancy_byte_batch = 100;
    memcpy(manifest->signer_public_key, key_a, 32);
    status = lxp_state_store_init(state, 1);
    if (status == LXP_OK) status = lxp_kernel_create(kernel, state, journal, &parameters, 0);
    if (status == LXP_OK) status = lxp_kernel_register_module(kernel, programs_module_registration_v4_storage_retirement());
    if (status == LXP_OK) status = lxp_programs_fee_genesis_append(manifest, &fees);
    if (status == LXP_OK) status = lxp_programs_fee_genesis_materialize(manifest, kernel);
    metering.version = 1;
    metering.activation_batch = 1;
    metering.authority_kind = LX_PROGRAMS_METERING_AUTHORITY_GENESIS;
    const uint64_t coefficients[9] = {1,1,1,1,1,8,8,64,8};
    memcpy(metering.coefficients, coefficients, sizeof(coefficients));
    if (status == LXP_OK) status = lxp_hash_payload(key_a, 32, metering.authority_digest);
    if (status == LXP_OK) status = lxp_programs_metering_genesis_append(manifest, &metering);
    if (status == LXP_OK) status = lxp_programs_metering_genesis_materialize(manifest, kernel);
    profile.version = 1;
    profile.network_id = 7;
    memcpy(profile.controller, controller, 32);
    memcpy(profile.archive_keys[0], key_a, 32);
    memcpy(profile.archive_keys[1], key_b, 32);
    memcpy(profile.paxai_program_id, program, 32);
    profile.hot_horizon = 1;
    if (status == LXP_OK) status = lxp_arena_init(&arena, arena_bytes, sizeof(arena_bytes));
    if (status == LXP_OK) status = lxp_module_ctx_init(&ctx, kernel, LXP_MODULE_PROGRAMS, 10, 0, 1, 1000000, &arena, true);
    if (status == LXP_OK) {
        status = lxp_programs_retirement_profile_stage(&ctx, &profile);
        if (status == LXP_OK) status = lxp_module_ctx_commit(&ctx);
        else lxp_module_ctx_rollback(&ctx);
    }
    (void)lxp_state_store_destroy(state);
done:
    free(manifest);
    free(state);
    free(journal);
    return status;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func Account(name string, asset [32]byte, key *[32]byte, balance, sequence uint64) ([]byte, error) {
	encoded := make([]byte, 256+C.LX_ACCOUNT_NAME_MAX)
	length := C.size_t(len(encoded))
	var authority *C.uint8_t
	if key != nil {
		authority = (*C.uint8_t)(unsafe.Pointer(&key[0]))
	}
	nameBytes := []byte(name)
	status := C.account_record((*C.uint8_t)(unsafe.Pointer(&nameBytes[0])), C.size_t(len(nameBytes)),
		(*C.uint8_t)(unsafe.Pointer(&asset[0])), authority, C.uint64_t(balance), C.uint64_t(sequence),
		(*C.uint8_t)(unsafe.Pointer(&encoded[0])), &length)
	if status != 0 {
		return nil, fmt.Errorf("native account producer: %d", int32(status))
	}
	return encoded[:int(length)], nil
}

func Records(asset, controller, keyA, keyB, program [32]byte) (map[string][]byte, error) {
	kernel := (*C.lxp_kernel)(C.calloc(1, C.size_t(unsafe.Sizeof(C.lxp_kernel{}))))
	if kernel == nil {
		return nil, fmt.Errorf("kernel allocation")
	}
	defer C.free(unsafe.Pointer(kernel))
	status := C.records(kernel, (*C.uint8_t)(unsafe.Pointer(&asset[0])), (*C.uint8_t)(unsafe.Pointer(&controller[0])), (*C.uint8_t)(unsafe.Pointer(&keyA[0])), (*C.uint8_t)(unsafe.Pointer(&keyB[0])), (*C.uint8_t)(unsafe.Pointer(&program[0])))
	if status != 0 {
		return nil, fmt.Errorf("native Programs bootstrap: %d", int32(status))
	}
	out := make(map[string][]byte)
	for _, entry := range unsafe.Slice(&kernel.module_kv[0], int(kernel.module_kv_count)) {
		if entry.module_id == 9 {
			out[string(C.GoBytes(unsafe.Pointer(&entry.key[0]), C.int(entry.key_length)))] = C.GoBytes(unsafe.Pointer(&entry.value[0]), C.int(entry.value_length))
		}
	}
	return out, nil
}
