package keeper

/*
#cgo CFLAGS: -I${SRCDIR}/../../../include -I${SRCDIR}/../../../build/generated
#include "layerx/lxp_snapshot.h"
*/
import "C"

import (
	"bytes"
	"errors"
	"fmt"
	"unsafe"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
)

// GetAsset reads the shared host state even when ledger writes are paused.
func (k *Keeper) GetAsset(ctx sdk.Context, assetID [32]byte) (lxphost.AssetRecord, error) {
	asset, err := k.host.ReadAsset(ctx, assetID)
	return asset, ledgerReadError(err)
}

func (k *Keeper) BalanceOf(ctx sdk.Context, accountID, assetID [32]byte) (sdk.Int, error) {
	balance, err := k.host.ReadBalance(ctx, accountID, assetID)
	return balance, ledgerReadError(err)
}

func (k *Keeper) TotalUnits(ctx sdk.Context, assetID [32]byte) (sdk.Int, error) {
	total, err := k.host.ReadTotalUnits(ctx, assetID)
	return total, ledgerReadError(err)
}

func (k *Keeper) AccountOwner(ctx sdk.Context, accountID [32]byte) ([32]byte, error) {
	owner, err := k.host.ReadAccountOwner(ctx, accountID)
	return owner, ledgerReadError(err)
}

// NextSequence is the owner-key convenience API used by native payment callers.
// ABI callers supply an account ID and use NextSequenceByAccountID instead.
func (k *Keeper) NextSequence(ctx sdk.Context, didKey, assetID [32]byte) (uint64, error) {
	name, accountID, err := lxphost.AssetAccountID(didKey, assetID)
	if err != nil {
		return 0, ledgerReadError(err)
	}
	account, err := k.host.ReadAccount(ctx, name, accountID)
	if err != nil {
		return 0, ledgerReadError(err)
	}
	return account.NextSequence, nil
}

// NextSequenceByAccountID resolves the persisted account, without deriving another ID.
func (k *Keeper) NextSequenceByAccountID(ctx sdk.Context, accountID, assetID [32]byte) (uint64, error) {
	account, err := k.accountByID(ctx, accountID)
	if err != nil {
		return 0, err
	}
	if !account.HasAsset || account.AssetID != assetID {
		return 0, types.ErrAssetMismatch
	}
	return account.NextSequence, nil
}

func (k *Keeper) accountByID(ctx sdk.Context, accountID [32]byte) (lxphost.AccountRecord, error) {
	// The host's named reader requires the canonical name as well as the ID.
	// Recover that name with the same kernel record codec used by the host.
	for _, key := range ctx.MultiStore().StoreKeys() {
		if key.Name() != keys.LayerXHostStoreKey {
			continue
		}
		row := ctx.KVStore(key).Get(append([]byte{0x06}, accountID[:]...))
		if len(row) == 0 {
			return lxphost.AccountRecord{}, ledgerReadError(lxphost.Code(C.LXP_ERR_UNKNOWN_ACCOUNT_NAMESPACE))
		}
		var decoded C.lx_account
		code := C.lx_account_record_decode((*C.uint8_t)(unsafe.Pointer(&row[0])), C.size_t(len(row)), &decoded)
		if code != C.LXP_OK {
			return lxphost.AccountRecord{}, ledgerReadError(lxphost.Code(code))
		}
		if !bytes.Equal(C.GoBytes(unsafe.Pointer(&decoded.id[0]), 32), accountID[:]) {
			return lxphost.AccountRecord{}, ledgerReadError(lxphost.Code(C.LXP_FATAL_INVARIANT))
		}
		name := string(C.GoBytes(unsafe.Pointer(&decoded.name[0]), C.int(decoded.name_length)))
		account, err := k.host.ReadAccount(ctx, name, accountID)
		return account, ledgerReadError(err)
	}
	return lxphost.AccountRecord{}, fmt.Errorf("layerxledger: lxphost store is not mounted")
}

func ledgerReadError(err error) error {
	var code lxphost.Code
	if errors.As(err, &code) {
		return FromKernel(code)
	}
	return err
}
