//go:build cgo

package lxphost

/*
#cgo CFLAGS: -I${SRCDIR}/../../include -I${SRCDIR}/../../build/generated
#cgo LDFLAGS: -L${SRCDIR}/../../build -L${SRCDIR}/../../programs/target/release -L${SRCDIR}/../../programs/target/debug -llayerx -llayerx_programs_sandbox -llayerx -lssl -lcrypto -pthread -ldl -lm
#include <stdlib.h>
#include <string.h>
#include "layerx/lx_asset.h"
#include "layerx/lx_escrow.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_module_ctx.h"
#include "layerx/lxp_protocol.h"
#include "layerx/programs.h"

enum { LXPHOST_ARENA_BYTES = 4194304 };

typedef struct lxphost_envelope {
    lxp_activity activity;
    uint8_t did[LXP_MAX_DID_LENGTH];
    uint8_t authority[32];
    uint8_t signature[128];
    uint8_t payload[LXP_MAX_PAYLOAD_BYTES];
} lxphost_envelope;

typedef struct lxphost_node {
    lxp_kernel kernel;
    lxp_state_store store;
    lxp_state_journal journal;
    lxp_identity_store identities;
    lxp_fee_params fees;
    lxp_authority_resolved authority;
    lxp_authority_scope authority_scope;
    bool authority_bound;
    lxp_kernel_execution execution;
    lxp_receipt receipt;
    lxp_arena arena;
    lxphost_envelope envelope;
    lx_account_registry accounts;
    lx_asset_registry assets;
    lxp_transfer_asset_state transfer_assets[LX_ASSET_REGISTRY_CAPACITY];
    lx_asset_runtime asset_runtime;
    lx_escrow_runtime escrow_runtime;
    lx_programs_transfer_runtime programs_runtime;
    uint64_t parameters;
    uint64_t epoch;
    int store_ready;
    uint8_t arena_bytes[LXPHOST_ARENA_BYTES];
} lxphost_node;

static lxphost_envelope *lxphost_node_envelope(lxphost_node *node)
{
    return &node->envelope;
}

static lxp_identity *lxphost_identity_find(lxphost_node *node,
                                           const uint8_t did_id[32])
{
    size_t i;
    for (i = 0U; i < node->identities.count &&
                 i < (size_t)LXP_IDENTITY_STORE_CAPACITY; ++i)
        if (memcmp(node->identities.identities[i].did_id, did_id, 32U) == 0)
            return &node->identities.identities[i];
    return NULL;
}

static int32_t lxphost_actor_sequence_set(lxphost_node *node,
                                          const uint8_t did_id[32],
                                          uint64_t sequence)
{
    lxp_identity *identity = lxphost_identity_find(node, did_id);
    if (identity == NULL) return LXP_ERR_UNKNOWN_DID;
    identity->next_sequence = sequence;
    return LXP_OK;
}

static int32_t lxphost_simulate(lxphost_node *node, uint32_t network_id,
                                uint64_t batch_number, uint64_t timestamp_ms,
                                uint64_t timestamp_window, uint64_t sequence,
                                uint32_t parameter_version, uint64_t gas_limit,
                                bool binding_authority, lxp_u128 fee_balance,
                                lxp_kernel_prepared_batch **batch_out)
{
    const lxp_module_registration *registration;
    lxp_kernel_execution *execution = &node->execution;
    lxp_kernel_batch_snapshot *snapshot = NULL;
    lxp_kernel_prepared_batch *batch = NULL;
    const lxp_receipt *receipts;
    int32_t status = lxp_kernel_module_for_activity(
        &node->kernel, node->envelope.activity.activity_type,
        node->kernel.epoch, &registration);
    if (status == LXP_OK)
        status = lxp_state_root(&node->kernel, node->kernel.current_state_root);
    if (status != LXP_OK) return status;
    (void)memset(execution, 0, sizeof(*execution));
    execution->network_id = network_id;
    execution->batch_number = batch_number;
    execution->batch_timestamp_ms = timestamp_ms;
    execution->maximum_timestamp_window = timestamp_window;
    execution->epoch = node->kernel.epoch;
    execution->global_sequence = sequence;
    execution->recorded_module_version = registration->abi_version;
    execution->parameter_version = parameter_version;
    execution->signature_valid = node->envelope.activity.signature.length != 0U ?
        lxp_activity_verify_signature(&node->envelope.activity) == LXP_OK :
        binding_authority;
    execution->binding_authority =
        node->envelope.activity.signature.length == 0U && binding_authority;
    execution->identities = &node->identities;
    execution->authority = &node->authority;
    execution->fee_parameters = &node->fees;
    execution->fee_balance = fee_balance;
    execution->gas_limit = gas_limit;
    execution->arena = &node->arena;
    if (node->envelope.activity.activity_type != LX_PROGRAMS_CALL)
        return lxp_kernel_execute_activity(&node->kernel,
            &node->envelope.activity, execution, &node->receipt);
    status = lxp_kernel_batch_snapshot_create(
        &node->kernel, &node->identities, execution->verified_receipts,
        execution, &snapshot);
    if (status == LXP_OK)
        status = lxp_kernel_simulate_activity(
            snapshot, &node->envelope.activity, execution, &batch);
    if (status == LXP_OK) {
        receipts = lxp_kernel_prepared_batch_receipts(batch);
        if (lxp_kernel_prepared_batch_count(batch) != 1U || receipts == NULL) {
            status = LXP_FATAL_INVARIANT;
        } else {
            node->receipt = receipts[0];
            *batch_out = batch;
            batch = NULL;
        }
    }
    lxp_kernel_prepared_batch_destroy(batch);
    lxp_kernel_batch_snapshot_destroy(snapshot);
    return status;
}
*/
import "C"

import (
	"runtime"
	"unsafe"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func lxphost_simulate(node *C.lxphost_node, cfg Config, branch *branchCtx, bindingAuthority bool, batch **C.lxp_kernel_prepared_batch) error {
	balance, err := fee_balance(node)
	if err != nil {
		return err
	}
	return result(C.lxphost_simulate(node, C.uint32_t(cfg.NetworkID), C.uint64_t(branch.batchNumber),
		C.uint64_t(branch.timestampMs), C.uint64_t(cfg.TimestampWindowMs), C.uint64_t(branch.sequence),
		C.uint32_t(cfg.ParameterVersion), C.uint64_t(cfg.GasLimit), C.bool(bindingAuthority), balance, batch))
}

// Simulate returns the kernel's hypothetical receipt without publishing state.
func (h *Host) Simulate(ctx sdk.Context, activity Activity, authority KernelAuthority) (Receipt, error) {
	bindingAuthority := binding_authority(activity, authority)
	if len(activity.Signature) == 0 && !bindingAuthority {
		return Receipt{}, Code(C.LXP_ERR_BAD_SIGNATURE)
	}
	cached, _ := ctx.CacheContext()
	branch, err := newBranchCtx(cached, h.cfg.StoreKey)
	if err != nil {
		return Receipt{}, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.node == nil {
		return Receipt{}, codeNonCanonical
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	private, err := NewHost(h.cfg)
	if err != nil {
		return Receipt{}, err
	}
	defer private.Close()
	if err := lxphost_reset(private.node, branch.sequence); err != nil {
		return Receipt{}, err
	}
	if err := writeEnvelope(C.lxphost_node_envelope(private.node), h.cfg.NetworkID, activity); err != nil {
		return Receipt{}, err
	}
	module, err := lxphost_activity_module(private.node)
	if err != nil {
		return Receipt{}, err
	}
	modules := activityModules(module)
	if err := lxphost_authority_write(private.node, authority); err != nil {
		return Receipt{}, err
	}
	if err := branch.load(private, modules); err != nil {
		return Receipt{}, err
	}
	if _, err := private.loadBlobs(branch.store, modules); err != nil {
		return Receipt{}, err
	}
	if err := lxphost_bind_assets(private.node, h.cfg.NetworkID, bindingAuthority); err != nil {
		return Receipt{}, err
	}
	did, err := private.bindActor()
	if err != nil {
		return Receipt{}, err
	}
	next, err := branch.identitySequence(did)
	if err != nil {
		return Receipt{}, err
	}
	if err := result(C.lxphost_actor_sequence_set(private.node, bytes32(&did), C.uint64_t(next))); err != nil {
		return Receipt{}, err
	}
	var batch *C.lxp_kernel_prepared_batch
	defer func() { C.lxp_kernel_prepared_batch_destroy(batch) }()
	if err := lxphost_simulate(private.node, h.cfg, branch, bindingAuthority, &batch); err != nil {
		return Receipt{}, err
	}
	var outcome lxphost_outcome
	defer func() {
		C.free(unsafe.Pointer(outcome.program_outcome.terminal_payload.bytes))
		C.free(unsafe.Pointer(outcome.program_outcome.call_graph_payload.bytes))
		C.free(unsafe.Pointer(outcome.program_outcome.event_envelope_payload.bytes))
	}()
	if err := lxphost_outcome_read(private.node, &outcome); err != nil {
		return Receipt{}, err
	}
	receipt := Receipt{
		FeeCharged:         readAmount(C.struct_lxp_u128{hi: outcome.fee_hi, lo: outcome.fee_lo}),
		ResultCode:         Code(outcome.result_code),
		GlobalSequence:     uint64(outcome.global_sequence),
		Timestamp:          uint64(outcome.timestamp),
		PreviousStateRoot:  *(*[32]byte)(unsafe.Pointer(&outcome.previous_state_root[0])),
		ResultingStateRoot: *(*[32]byte)(unsafe.Pointer(&outcome.resulting_state_root[0])),
		Encoded:            C.GoBytes(unsafe.Pointer(outcome.encoded), C.int(outcome.encoded_length)),
	}
	for _, effect := range unsafe.Slice(outcome.effects, int(outcome.effect_count)) {
		if effect.kind == C.LXP_EFFECT_EVENT {
			receipt.Events = append(receipt.Events, ReceiptEvent{
				Code: uint16(effect.event_type),
				Body: C.GoBytes(unsafe.Pointer(&effect.body[0]), C.int(effect.body_length)),
			})
		}
	}
	if outcome.amount.hi != 0 || outcome.amount.lo != 0 {
		receipt.Transfers = []TransferLeg{{
			Asset:  *(*[32]byte)(unsafe.Pointer(&outcome.asset[0])),
			From:   *(*[32]byte)(unsafe.Pointer(&outcome.from[0])),
			To:     *(*[32]byte)(unsafe.Pointer(&outcome.to[0])),
			Amount: readAmount(outcome.amount),
		}}
	}
	if program := &outcome.program_outcome; program.present != C.bool(false) {
		receipt.ProgramOutcome = &ProgramOutcome{
			Status:                  int32(program.result_code),
			Kind:                    uint8(program.terminal_kind),
			Response:                C.GoBytes(unsafe.Pointer(program.terminal_payload.bytes), C.int(program.terminal_payload.length)),
			FeeScheduleVersion:      uint64(program.fee_schedule_version),
			MeteringScheduleVersion: uint64(program.metering_schedule_version),
			Events:                  C.GoBytes(unsafe.Pointer(program.event_envelope_payload.bytes), C.int(program.event_envelope_payload.length)),
		}
	}
	return receipt, nil
}
