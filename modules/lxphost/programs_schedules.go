//go:build cgo

package lxphost

/*
#include <stdlib.h>
#include <string.h>
#include "layerx/programs.h"
#include "layerx/lxp_genesis.h"
#include "layerx/lxp_hash.h"
#include "layerx/lxp_snapshot.h"

enum { LXPHOST_FEE_SCHEDULE_RECORD_BYTES = 217 };

typedef struct lxphost_schedule_rows {
    uint8_t fee_active[LXPHOST_FEE_SCHEDULE_RECORD_BYTES];
    uint8_t fee_history[LXPHOST_FEE_SCHEDULE_RECORD_BYTES];
    uint8_t metering_active[LX_PROGRAMS_METERING_RECORD_BYTES];
    uint8_t metering_history[LX_PROGRAMS_METERING_RECORD_BYTES];
} lxphost_schedule_rows;

static int32_t lxphost_produce_programs_schedules(
    lxphost_schedule_rows *rows, const lx_programs_fee_genesis_parameters *fees,
    const lx_programs_metering_schedule *metering, const uint8_t signer[32])
{
    lxp_kernel *kernel = calloc(1, sizeof(*kernel));
    lxp_genesis_manifest *manifest = calloc(1, sizeof(*manifest));
    lxp_state_store *state = calloc(1, sizeof(*state));
    lxp_state_journal *journal = calloc(1, sizeof(*journal));
    uint64_t parameters = 1;
    uint8_t found = 0;
    int32_t status = LXP_ERR_ARENA_EXHAUSTED;
    if (rows == NULL || kernel == NULL || manifest == NULL || state == NULL || journal == NULL) goto done;
    memcpy(manifest->signer_public_key, signer, 32);
    status = lxp_state_store_init(state, 1);
    if (status == LXP_OK) status = lxp_kernel_create(kernel, state, journal, &parameters, 0);
    if (status == LXP_OK) status = lxp_kernel_register_module(kernel, programs_module_registration_v4_storage_retirement());
    if (status == LXP_OK) status = lxp_programs_fee_genesis_append(manifest, fees);
    if (status == LXP_OK) status = lxp_programs_fee_genesis_validate(manifest);
    if (status == LXP_OK) status = lxp_programs_fee_genesis_materialize(manifest, kernel);
    if (status == LXP_OK) status = lxp_programs_metering_genesis_append(manifest, metering);
    if (status == LXP_OK) status = lxp_programs_metering_genesis_validate(manifest);
    if (status == LXP_OK) status = lxp_programs_metering_genesis_materialize(manifest, kernel);
    if (status == LXP_OK) {
        for (size_t i = 0; i < kernel->module_kv_count; ++i) {
            const lxp_module_kv_entry *entry = &kernel->module_kv[i];
            if (entry->module_id != LXP_MODULE_PROGRAMS) continue;
            if (entry->key_length == sizeof("progfee/active/v1") - 1 &&
                memcmp(entry->key, "progfee/active/v1", entry->key_length) == 0 &&
                entry->value_length == sizeof(rows->fee_active)) {
                memcpy(rows->fee_active, entry->value, sizeof(rows->fee_active));
                found |= 1;
            } else if (entry->key_length == sizeof("progfee/history/v1/\0\0\0\1") - 1 &&
                memcmp(entry->key, "progfee/history/v1/\0\0\0\1", entry->key_length) == 0 &&
                entry->value_length == sizeof(rows->fee_history)) {
                memcpy(rows->fee_history, entry->value, sizeof(rows->fee_history));
                found |= 2;
            } else if (entry->key_length == sizeof("progmet/active/v1") - 1 &&
                memcmp(entry->key, "progmet/active/v1", entry->key_length) == 0 &&
                entry->value_length == sizeof(rows->metering_active)) {
                memcpy(rows->metering_active, entry->value, sizeof(rows->metering_active));
                found |= 4;
            } else if (entry->key_length == sizeof("progmet/history/v1/\0\0\0\1") - 1 &&
                memcmp(entry->key, "progmet/history/v1/\0\0\0\1", entry->key_length) == 0 &&
                entry->value_length == sizeof(rows->metering_history)) {
                memcpy(rows->metering_history, entry->value, sizeof(rows->metering_history));
                found |= 8;
            }
        }
        if (found != 15) status = LXP_ERR_UNKNOWN_FIELD;
    }
done:
    if (state != NULL) (void)lxp_state_store_destroy(state);
    free(kernel);
    free(manifest);
    free(state);
    free(journal);
    return status;
}
*/
import "C"

import (
	"encoding/binary"
	"fmt"
	"unsafe"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

// ProgramsScheduleParameters are the version 1 operating values supplied by a
// chain upgrade. AuthorityKey is the 32-byte identity committed by both native
// genesis producers; the metering record commits its kernel hash.
type ProgramsScheduleParameters struct {
	FeePrices            [7]uint64
	MeteringCoefficients [9]uint64
	OccupancyAssetID     [32]byte
	AuthorityKey         [32]byte
	TargetOccupancy      uint64
	ResponseDenominator  uint64
	MaxChangeNumerator   uint64
	MaxChangeDenominator uint64
	MinFee               uint64
	MaxFee               uint64
}

type ProgramsSchedulesInstalled struct {
	Fee      bool
	Metering bool
}

// InstallProgramsSchedules installs each missing kind from native genesis
// producers. A present active row, including its history, is left untouched.
func InstallProgramsSchedules(ctx sdk.Context, key sdk.StoreKey, p ProgramsScheduleParameters) (ProgramsSchedulesInstalled, error) {
	var installed ProgramsSchedulesInstalled
	store := ctx.KVStore(key)
	feeActive := moduleKVKey(9, []byte("progfee/active/v1"))
	feeHistory := moduleKVKey(9, binary.BigEndian.AppendUint32([]byte("progfee/history/v1/"), 1))
	meteringActive := moduleKVKey(9, []byte("progmet/active/v1"))
	meteringHistory := moduleKVKey(9, binary.BigEndian.AppendUint32([]byte("progmet/history/v1/"), 1))
	needFee, needMetering := !store.Has(feeActive), !store.Has(meteringActive)
	if !needFee && !needMetering {
		return installed, nil
	}
	if needFee && store.Has(feeHistory) || needMetering && store.Has(meteringHistory) {
		return installed, fmt.Errorf("Programs schedule history exists without its active row")
	}

	var fees C.lx_programs_fee_genesis_parameters
	fees.schedule.version = 1
	prices := []*C.uint64_t{&fees.schedule.cpu, &fees.schedule.memory_byte, &fees.schedule.storage_read_byte,
		&fees.schedule.storage_write_byte, &fees.schedule.output_value, &fees.schedule.output_byte,
		&fees.schedule.occupancy_byte_batch}
	for i, price := range p.FeePrices {
		*prices[i] = C.uint64_t(price)
	}
	copy(unsafe.Slice((*byte)(unsafe.Pointer(&fees.occupancy_asset_id[0])), 32), p.OccupancyAssetID[:])
	fees.target_occupancy_byte_batches = C.uint64_t(p.TargetOccupancy)
	fees.response_denominator = C.uint64_t(p.ResponseDenominator)
	fees.maximum_change_numerator = C.uint64_t(p.MaxChangeNumerator)
	fees.maximum_change_denominator = C.uint64_t(p.MaxChangeDenominator)
	fees.minimum_fee_units_per_occupancy_byte_batch = C.uint64_t(p.MinFee)
	fees.maximum_fee_units_per_occupancy_byte_batch = C.uint64_t(p.MaxFee)

	var metering C.lx_programs_metering_schedule
	metering.version = 1
	metering.activation_batch = 1
	metering.authority_kind = C.LX_PROGRAMS_METERING_AUTHORITY_GENESIS
	for i, coefficient := range p.MeteringCoefficients {
		metering.coefficients[i] = C.uint64_t(coefficient)
	}
	if err := result(C.lxp_hash_payload(unsafe.Pointer(bytes32(&p.AuthorityKey)), 32, &metering.authority_digest[0])); err != nil {
		return installed, fmt.Errorf("Programs schedule authority: %w", err)
	}
	var rows C.lxphost_schedule_rows
	if err := result(C.lxphost_produce_programs_schedules(&rows, &fees, &metering, bytes32(&p.AuthorityKey))); err != nil {
		return installed, fmt.Errorf("Programs schedule genesis production: %w", err)
	}
	if needFee {
		store.Set(feeActive, C.GoBytes(unsafe.Pointer(&rows.fee_active[0]), C.LXPHOST_FEE_SCHEDULE_RECORD_BYTES))
		store.Set(feeHistory, C.GoBytes(unsafe.Pointer(&rows.fee_history[0]), C.LXPHOST_FEE_SCHEDULE_RECORD_BYTES))
		installed.Fee = true
	}
	if needMetering {
		store.Set(meteringActive, C.GoBytes(unsafe.Pointer(&rows.metering_active[0]), C.LX_PROGRAMS_METERING_RECORD_BYTES))
		store.Set(meteringHistory, C.GoBytes(unsafe.Pointer(&rows.metering_history[0]), C.LX_PROGRAMS_METERING_RECORD_BYTES))
		installed.Metering = true
	}
	return installed, nil
}
