//go:build cgo

package lxphost

/*
#include <string.h>
#include "layerx/lxp_kernel.h"
#include "layerx/lxp_snapshot.h"

static int32_t lxphost_genesis_account_validate(const uint8_t id[32],
                                                const uint8_t *bytes, size_t length)
{
    lx_account account;
    int32_t status = lx_account_record_decode(bytes, length, &account);
    if (status != LXP_OK) return status;
    if (memcmp(account.id, id, 32U) != 0) return LXP_FATAL_INVARIANT;
    return LXP_OK;
}
*/
import "C"

import (
	"bytes"
	"encoding/binary"
	"fmt"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
)

type StoreGenesis struct {
	LastSequence uint64    `json:"last_sequence"`
	Records      []kv.Pair `json:"records"`
}

func DefaultStoreGenesis() *StoreGenesis { return &StoreGenesis{} }

func (gs StoreGenesis) Validate() error {
	ledgerRows := 0
	for i, record := range gs.Records {
		invalid := func(reason string) error { return fmt.Errorf("lxphost: record %d: %s", i, reason) }
		if i > 0 && bytes.Compare(gs.Records[i-1].Key, record.Key) >= 0 {
			return invalid("keys must be strictly ascending")
		}
		if len(record.Key) == 0 {
			return invalid("empty key")
		}
		switch record.Key[0] {
		case moduleKVPrefix:
			if len(record.Key) < 4 || len(record.Key) > 3+C.LXP_MODULE_MAX_KEY_BYTES {
				return invalid("invalid ledger key length")
			}
			if binary.BigEndian.Uint16(record.Key[1:3]) != C.LXP_MODULE_ASSET {
				return invalid("foreign module row")
			}
			if len(record.Value) > C.LXP_MODULE_MAX_VALUE_BYTES {
				return invalid("ledger value too large")
			}
			ledgerRows++
			if ledgerRows > C.LXP_KERNEL_MAX_MODULE_KV {
				return invalid("too many ledger rows")
			}
		case identitySequencePrefix:
			if len(record.Key) != 33 || len(record.Value) != 8 {
				return invalid("invalid identity sequence")
			}
		case accountPrefix:
			if len(record.Key) != 33 {
				return invalid("invalid account key length")
			}
			if err := result(C.lxphost_genesis_account_validate(bytesPtr(record.Key[1:]), bytesPtr(record.Value), C.size_t(len(record.Value)))); err != nil {
				return fmt.Errorf("lxphost: record %d: invalid account: %w", i, err)
			}
		default:
			return invalid("foreign or reserved prefix")
		}
	}
	return nil
}

func ExportStoreGenesis(ctx sdk.Context, key sdk.StoreKey) (*StoreGenesis, error) {
	branch := &branchCtx{store: ctx.KVStore(key)}
	last, err := branch.lastSequence()
	if err != nil {
		return nil, err
	}
	gs := &StoreGenesis{LastSequence: last}
	for _, prefix := range [][]byte{{moduleKVPrefix, 0, C.LXP_MODULE_ASSET}, {identitySequencePrefix}, {accountPrefix}} {
		iterator := sdk.KVStorePrefixIterator(branch.store, prefix)
		for ; iterator.Valid(); iterator.Next() {
			gs.Records = append(gs.Records, kv.Pair{Key: bytes.Clone(iterator.Key()), Value: bytes.Clone(iterator.Value())})
		}
		if err := iterator.Close(); err != nil {
			return nil, err
		}
	}
	return gs, nil
}

func InitStoreGenesis(ctx sdk.Context, key sdk.StoreKey, gs StoreGenesis) error {
	if err := gs.Validate(); err != nil {
		return err
	}
	store := ctx.KVStore(key)
	if gs.LastSequence > 0 {
		store.Set([]byte{lastSequenceKey}, binary.BigEndian.AppendUint64(nil, gs.LastSequence))
	}
	for _, record := range gs.Records {
		store.Set(record.Key, append([]byte{}, record.Value...))
	}
	return nil
}
