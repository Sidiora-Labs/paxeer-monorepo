package keeper

/*
#cgo CFLAGS: -I${SRCDIR}/../../../include -I${SRCDIR}/../../../build/generated
#include "layerx/lxp_snapshot.h"
#include "layerx/lxp_ledger.h"
*/
import "C"

import (
	"bytes"
	"unsafe"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/ethereum/go-ethereum/common"
)

type exchangeAccountBindings interface {
	GetLayerXDid(sdk.Context, common.Address) ([32]byte, bool)
}

func defaultOrderAccount(key [32]byte) ([32]byte, error) {
	return evmtypes.LayerXMainAccountID(key)
}

func (k *Keeper) requireOrderAccount(ctx sdk.Context, caller common.Address, accountID [32]byte) error {
	bindings, ok := k.evmKeeper.(exchangeAccountBindings)
	if !ok {
		return types.ErrKeeperMissing
	}
	key, bound := bindings.GetLayerXDid(ctx, caller)
	if !bound || accountID == ([32]byte{}) {
		return sdkerrors.Wrap(types.ErrInvalidIntent, "account is not controlled by caller")
	}
	expected, err := lxphost.DIDID([]byte(evmtypes.LayerXDid(key)))
	if err != nil {
		return sdkerrors.Wrap(types.ErrInvalidIntent, "account owner")
	}
	for _, storeKey := range ctx.MultiStore().StoreKeys() {
		if storeKey.Name() != keys.LayerXHostStoreKey {
			continue
		}
		row := ctx.KVStore(storeKey).Get(append([]byte{0x06}, accountID[:]...))
		if len(row) == 0 {
			return sdkerrors.Wrap(types.ErrInvalidIntent, "unknown account")
		}
		var account C.lx_account
		if C.lx_account_record_decode((*C.uint8_t)(unsafe.Pointer(&row[0])), C.size_t(len(row)), &account) != C.LXP_OK ||
			!bytes.Equal(C.GoBytes(unsafe.Pointer(&account.id[0]), 32), accountID[:]) {
			return sdkerrors.Wrap(types.ErrInvalidIntent, "invalid account record")
		}
		var owner [32]byte
		if C.lx_account_owner_did(&account, (*C.uint8_t)(unsafe.Pointer(&owner[0]))) != C.LXP_OK || owner != expected {
			return sdkerrors.Wrap(types.ErrInvalidIntent, "account is not controlled by caller")
		}
		return nil
	}
	return sdkerrors.Wrap(types.ErrKeeperMissing, "account registry")
}
