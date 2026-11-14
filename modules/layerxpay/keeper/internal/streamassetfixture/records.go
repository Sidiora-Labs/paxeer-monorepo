// Package streamassetfixture encodes native asset and funded account genesis records.
package streamassetfixture

/*
#cgo CFLAGS: -I${SRCDIR}/../../../../../include -I${SRCDIR}/../../../../../build/generated
#cgo LDFLAGS: -L${SRCDIR}/../../../../../build -llayerx
#include <string.h>
#include "layerx/lx_asset.h"
#include "layerx/lxp_genesis.h"
#include "layerx/lxp_snapshot.h"

static int32_t native_asset(const uint8_t *issuer, uint8_t *id,
    uint8_t *out, size_t capacity, size_t *length)
{
    lx_asset_record record = {0};
    int32_t status = lxp_genesis_custody_asset_id(0, id);
    if (status != LXP_OK) return status;
    memcpy(record.asset_id, id, 32);
    memcpy(record.symbol, "PAX", 4);
    record.symbol_length = 3;
    memcpy(record.name, "PAX", 3);
    record.name_length = 3;
    record.decimals = 6;
    record.issuer_kind = 2;
    record.custody_kind = LX_ASSET_CUSTODY_PAXEER;
    memcpy(record.issuer_did32, issuer, 32);
    memcpy(record.custody_reference, "uhpx", 4);
    record.custody_reference_length = 4;
    record.supply_cap = (lxp_u128){0, 200};
    record.total_units = (lxp_u128){0, 200};
    return lx_asset_record_encode(&record, out, capacity, length);
}

static int32_t funded_account(const uint8_t *name, size_t name_length,
    const uint8_t *id, const uint8_t *asset, const uint8_t *key,
    uint64_t balance, uint8_t *out, size_t capacity, size_t *length)
{
    lx_account_registry registry;
    lx_account *account = NULL;
    int32_t status = lx_account_registry_init(&registry);
    if (status != LXP_OK) return status;
    status = lx_account_open(&registry, name, name_length, id, 1,
        LX_ACCOUNT_OPEN_GENESIS, NULL, &account);
    if (status == LXP_OK) status = lxp_ledger_bootstrap_balance(account,
        asset, (lxp_u128){0, balance}, 0);
    if (status == LXP_OK) {
        memcpy(account->authority_key, key, 32);
        account->has_authority_key = true;
        status = lx_account_validate_canonical(account);
    }
    if (status == LXP_OK) status = lx_account_record_encode(account,
        out, capacity, length);
    lx_account_registry_release(&registry);
    return status;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func NativeAsset(issuer [32]byte) ([32]byte, []byte, error) {
	var id [32]byte
	out := make([]byte, 384)
	var length C.size_t
	status := C.native_asset((*C.uint8_t)(unsafe.Pointer(&issuer[0])),
		(*C.uint8_t)(unsafe.Pointer(&id[0])), (*C.uint8_t)(unsafe.Pointer(&out[0])), C.size_t(len(out)), &length)
	if status != 0 {
		return id, nil, fmt.Errorf("native asset record: %d", int32(status))
	}
	return id, out[:int(length)], nil
}

func Account(name string, id, asset, key [32]byte, balance uint64) ([]byte, error) {
	nameBytes := []byte(name)
	if len(nameBytes) == 0 {
		return nil, fmt.Errorf("empty account name")
	}
	out := make([]byte, 512)
	var length C.size_t
	status := C.funded_account((*C.uint8_t)(unsafe.Pointer(&nameBytes[0])), C.size_t(len(nameBytes)),
		(*C.uint8_t)(unsafe.Pointer(&id[0])), (*C.uint8_t)(unsafe.Pointer(&asset[0])),
		(*C.uint8_t)(unsafe.Pointer(&key[0])), C.uint64_t(balance),
		(*C.uint8_t)(unsafe.Pointer(&out[0])), C.size_t(len(out)), &length)
	if status != 0 {
		return nil, fmt.Errorf("funded account record: %d", int32(status))
	}
	return out[:int(length)], nil
}
