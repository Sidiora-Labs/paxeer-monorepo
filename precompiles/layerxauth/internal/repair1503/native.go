// Package repair1503 supplies in-process native fixtures for the auth integration tests.
package repair1503

/*
#cgo CFLAGS: -I${SRCDIR}/../../../../include -I${SRCDIR}/../../../../build/generated
#cgo LDFLAGS: -L${SRCDIR}/../../../../build -L${SRCDIR}/../../../../programs/target/release -L${SRCDIR}/../../../../programs/target/debug -llayerx -llayerx_programs_sandbox -llayerx -lssl -lcrypto -pthread -ldl -lm
#include <stdlib.h>
#include <string.h>
#include "layerx/lxp_genesis.h"
#include "layerx/lxp_activity.h"
#include "layerx/lxp_crypto.h"
#include "layerx/lxp_kernel.h"
#include "layerx/lxp_authority.h"
#include "layerx/lxp_governance.h"
#include "layerx/lxp_module_ctx.h"
#include "layerx/programs.h"

static int fixture_governed(lxp_genesis_manifest *manifest, lxp_kernel *kernel,
    const uint8_t *did, size_t did_length, const uint8_t signer[32],
    const uint8_t payload[149], const uint8_t signature[64], uint64_t timestamp, int *failed_line) {
    lxp_state_store state;
    lxp_state_journal journal;
    lxp_identity_store identities = {0};
    lxp_identity *identity = NULL;
    lx_account_registry accounts;
    lxp_fee_params parameters = {0};
    lxp_kernel_execution execution = {0};
    lxp_authority_resolved authority = {0};
    lxp_activity activity = {0};
    lxp_receipt receipt = {0};
    lxp_verified_receipt_index *index = calloc(1, sizeof(*index));
    uint8_t *arena_bytes = malloc(4194304);
    lxp_arena arena;
    lxp_module_ctx ctx;
    lx_programs_fee_schedule proposed = {0,1,1,1,1,1,1,1};
    lx_programs_metering_schedule metering = {0};
    const uint64_t coefficients[9] = {1,1,1,1,1,8,8,64,8};
    parameters.version = 1;
    parameters.multiplier_basis_points = 10000;
    int status = lxp_state_store_init(&state, 1);
    if (status != LXP_OK) { free(index); free(arena_bytes); return status; }
    status = lx_account_registry_init(&accounts);
#define STEP(expr) do { if (status == LXP_OK) { status = (expr); if (status != LXP_OK) *failed_line = __LINE__; } } while (0)
    STEP(lxp_state_store_bind_accounts(&state, &accounts));
    STEP(lxp_state_store_require_account_root(&state));
    STEP(lxp_kernel_create(kernel, &state, &journal, &parameters, 1));
    STEP(lxp_kernel_register_module(kernel, lxp_governance_module_iface()));
    STEP(lxp_kernel_register_module(kernel, programs_module_registration()));
    STEP(lxp_arena_init(&arena, arena_bytes, 4194304));
    STEP(lxp_identity_register(&identities, did, did_length, signer, &identity));
    if (status == LXP_OK) {
        kernel->governance_authority.enabled = true;
        kernel->governance_authority.network_id = 7;
        memcpy(kernel->governance_authority.governance_public_key, signer, 32);
        authority.kind = LXP_AUTHORITY_OWNER;
        memcpy(authority.actor, identity->did_id, 32);
        memcpy(authority.principal, identity->did_id, 32);
        memcpy(authority.verified_key, signer, 32);
        activity.protocol_version = 3;
        activity.network_id = 7;
        activity.activity_type = LXP_GOVERNANCE_PROGRAMS_FEE;
        activity.actor_did = (lxp_byte_span){did, did_length};
        activity.authority = (lxp_byte_span){signer, 32};
        activity.timestamp_bound = (lxp_timestamp_bound){timestamp, timestamp};
        activity.payload = (lxp_byte_span){payload, 149};
        activity.signature = (lxp_byte_span){signature, 64};
        activity.idempotency_key[0] = 150;
        status = lxp_hash_payload(payload, 149, activity.payload_hash);
        const lxp_module_registration *registration = NULL;
        STEP(lxp_kernel_module_for_activity(kernel, activity.activity_type, 1, &registration));
        if (status == LXP_OK) execution.recorded_module_version = registration->abi_version;
        execution.network_id = 7;
        execution.batch_number = 1;
        execution.batch_timestamp_ms = timestamp;
        execution.maximum_timestamp_window = 60000;
        execution.epoch = 1;
        execution.global_sequence = 1;
        execution.parameter_version = 1;
        execution.signature_valid = lxp_activity_verify_signature(&activity) == LXP_OK;
        execution.identities = &identities;
        execution.authority = &authority;
        execution.fee_parameters = &parameters;
        execution.gas_limit = 1000000;
        execution.arena = &arena;
    }
    STEP(lxp_state_root(kernel, kernel->current_state_root));
    STEP(lxp_kernel_execute_activity(kernel, &activity, &execution, &receipt));
    if (status == LXP_OK) { status = receipt.result_code; if (status != LXP_OK) *failed_line = __LINE__; }
    STEP(lxp_verified_receipt_index_init(index));
    STEP(lxp_verified_receipt_index_bind_authority_network(index, 7));
    uint8_t sequencer_seed[32];
    memset(sequencer_seed, 150, 32);
    STEP(lxp_receipt_sign(&receipt, sequencer_seed, &arena));
    STEP(lxp_verified_receipt_index_add(index, &receipt, signer, &arena));
    STEP(lxp_module_ctx_init(&ctx, kernel, LXP_MODULE_PROGRAMS, timestamp, 1, 2, 1000000, &arena, true));
    if (status == LXP_OK) {
        ctx.batch_number = 1;
        ctx.protocol_version = 3;
        ctx.verified_receipts = index;
    }
    STEP(lxp_programs_fee_governance_stage(&ctx, &proposed, payload + 61,
        1000, 8, 1, 8, 1, 100, 2, &receipt));
    if (status == LXP_OK) ctx.batch_number = 2;
    STEP(lxp_programs_fee_governance_activate(&ctx, 2));
    STEP(lxp_module_ctx_commit(&ctx));
    memcpy(manifest->signer_public_key, signer, 32);
    metering.version = 1;
    metering.activation_batch = 1;
    metering.authority_kind = LX_PROGRAMS_METERING_AUTHORITY_GENESIS;
    memcpy(metering.coefficients, coefficients, sizeof(coefficients));
    STEP(lxp_hash_payload(signer, 32, metering.authority_digest));
    STEP(lxp_programs_metering_genesis_append(manifest, &metering));
    STEP(lxp_programs_metering_genesis_validate(manifest));
    lx_account_registry_release(&accounts);
    lxp_state_store_destroy(&state);
    free(index);
    free(arena_bytes);
    kernel->state = NULL;
    kernel->journal = NULL;
    return status;
#undef STEP
}

static int fixture_fee_admission(lxp_kernel *kernel, const uint8_t grant_id[32], uint64_t timestamp, uint64_t amount) {
    lxp_authority_grant grant;
    lxp_authority_resolved authority = {0};
    lxp_activity activity = {0};
    lx_programs_transfer_runtime runtime = {0};
    int status = lxp_authority_grant_load(kernel, grant_id, &grant);
    if (status != LXP_OK) return status;
    authority.kind = grant.kind;
    memcpy(authority.grant_id, grant_id, 32);
    memcpy(authority.actor, grant.grantee, 32);
    memcpy(authority.principal, grant.grantor, 32);
    memcpy(authority.verified_key, grant.key, 32);
    status = lxp_authority_hash(grant.kind, grant_id, grant.key, authority.authority_hash);
    runtime.resolve_occupancy_parameters = lxp_programs_fee_governance_resolve_runtime;
    runtime.occupancy_parameter_context = kernel;
    kernel->module_runtime[LXP_MODULE_PROGRAMS] = &runtime;
    if (status == LXP_OK) status = lxp_authority_fee_resolve(kernel, &authority, &activity, timestamp, 0, (lxp_u128){0, amount}, &grant);
    kernel->module_runtime[LXP_MODULE_PROGRAMS] = NULL;
    return status;
}
*/
import "C"

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"fmt"
	"unsafe"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
)

// GovernedRecords executes a signed governance proposal, authenticates its real
// receipt, and stages and activates the resulting production fee schedule.
func GovernedRecords(asset [32]byte, timestamp uint64) ([]kv.Pair, error) {
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{150}, ed25519.SeedSize))
	var signer [32]byte
	copy(signer[:], private.Public().(ed25519.PublicKey))
	did := []byte(evmtypes.LayerXDid(signer))
	payload := []byte("LXFG1")
	for i := 0; i < 7; i++ {
		payload = binary.BigEndian.AppendUint64(payload, 1)
	}
	payload = append(payload, asset[:]...)
	for _, n := range []uint64{1000, 8, 1, 8, 1, 100, 2} {
		payload = binary.BigEndian.AppendUint64(payload, n)
	}
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: sdk.NewKVStoreKey("native-governance-fixture"), NetworkID: 7, ParameterVersion: 1, GasLimit: 1000000})
	if err != nil {
		return nil, err
	}
	defer host.Close()
	activity := lxphost.Activity{ProtocolVersion: 3, Type: 0x0007000b, ActorDID: did, AuthorityKey: signer,
		NotBefore: timestamp, NotAfter: timestamp, IdempotencyKey: [32]byte{150}, Payload: payload}
	digest, err := host.SigningHash(activity)
	if err != nil {
		return nil, err
	}
	signature := ed25519.Sign(private, digest[:])
	manifest := (*C.lxp_genesis_manifest)(C.calloc(1, C.size_t(C.sizeof_lxp_genesis_manifest)))
	kernel := (*C.lxp_kernel)(C.calloc(1, C.size_t(C.sizeof_lxp_kernel)))
	defer C.free(unsafe.Pointer(manifest))
	defer C.free(unsafe.Pointer(kernel))
	var failedLine C.int
	if status := C.fixture_governed(manifest, kernel, (*C.uint8_t)(unsafe.Pointer(&did[0])), C.size_t(len(did)),
		(*C.uint8_t)(unsafe.Pointer(&signer[0])), (*C.uint8_t)(unsafe.Pointer(&payload[0])),
		(*C.uint8_t)(unsafe.Pointer(&signature[0])), C.uint64_t(timestamp), &failedLine); status != 0 {
		return nil, fmt.Errorf("governance fee fixture at native C line %d: %w", int(failedLine), lxphost.Code(status))
	}
	var rows []kv.Pair
	for _, value := range manifest.module_values[:int(manifest.module_value_count)] {
		key := bytes.TrimRight(C.GoBytes(unsafe.Pointer(&value.key[0]), 32), "\x00")
		rows = append(rows, kv.Pair{Key: append([]byte{1, 0, byte(value.module_id)}, key...), Value: C.GoBytes(unsafe.Pointer(&value.value[0]), C.int(value.value_length))})
	}
	for _, value := range kernel.module_kv[:int(kernel.module_kv_count)] {
		key := C.GoBytes(unsafe.Pointer(&value.key[0]), C.int(value.key_length))
		rows = append(rows, kv.Pair{Key: append([]byte{1, 0, byte(value.module_id)}, key...), Value: C.GoBytes(unsafe.Pointer(&value.value[0]), C.int(value.value_length))})
	}
	// Fixed genesis trust policy; no grant counters or fee schedules are encoded here.
	var key [32]byte
	copy(key[:], "native-fee-authority-version")
	value := make([]byte, 32)
	value[31] = 2
	rows = append(rows, kv.Pair{Key: append([]byte{1, 0, 7}, key[:]...), Value: value})
	return rows, nil
}

// FeeAdmission invokes the production kernel admission consumer on persisted module rows.
func FeeAdmission(rows map[string][]byte, id [32]byte, timestamp, amount uint64) error {
	kernel := (*C.lxp_kernel)(C.calloc(1, C.size_t(C.sizeof_lxp_kernel)))
	defer C.free(unsafe.Pointer(kernel))
	for rawKey, value := range rows {
		key := []byte(rawKey)
		if len(key) < 4 || key[0] != 1 {
			continue
		}
		if int(kernel.module_kv_count) >= len(kernel.module_kv) {
			return lxphost.Code(-5)
		}
		entry := &kernel.module_kv[int(kernel.module_kv_count)]
		entry.module_id = C.uint16_t(binary.BigEndian.Uint16(key[1:3]))
		entry.key_length = C.uint16_t(len(key) - 3)
		entry.value_length = C.uint32_t(len(value))
		if len(key)-3 > len(entry.key) || len(value) > len(entry.value) {
			return lxphost.Code(-5)
		}
		copy(unsafe.Slice((*byte)(unsafe.Pointer(&entry.key[0])), len(key)-3), key[3:])
		copy(unsafe.Slice((*byte)(unsafe.Pointer(&entry.value[0])), len(value)), value)
		kernel.module_kv_count++
	}
	status := C.fixture_fee_admission(kernel, (*C.uint8_t)(unsafe.Pointer(&id[0])), C.uint64_t(timestamp), C.uint64_t(amount))
	if status != 0 {
		return lxphost.Code(status)
	}
	return nil
}
