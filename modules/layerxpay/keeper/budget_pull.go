package keeper

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

func (k Keeper) BudgetPull(ctx sdk.Context, caller common.Address, budgetID, recipient [32]byte, amount codec.U128, payerGrant []byte) (lxphost.Receipt, error) {
	return lxphost.Receipt{}, types.ErrBudgetPullRetired
}

func nextPayCallOrdinal(ctx sdk.Context, txHash [32]byte) (uint32, error) {
	if txHash == ([32]byte{}) {
		return 0, fmt.Errorf("LXP_NON_CANONICAL: layerxpay missing transaction hash")
	}
	var transientKey *sdk.TransientStoreKey
	for _, key := range ctx.MultiStore().StoreKeys() {
		if key.Name() == evmtypes.TransientStoreKey {
			transientKey, _ = key.(*sdk.TransientStoreKey)
			break
		}
	}
	if transientKey == nil {
		return 0, fmt.Errorf("LXP_NON_CANONICAL: layerxpay EVM transient store is not mounted")
	}
	store := ctx.TransientStore(transientKey)
	key := append([]byte(payCallOrdinalPrefix), txHash[:]...)
	var ordinal uint32
	if previous := store.Get(key); previous != nil {
		if len(previous) != 4 {
			return 0, fmt.Errorf("LXP_NON_CANONICAL: layerxpay call ordinal")
		}
		last := binary.BigEndian.Uint32(previous)
		if last == math.MaxUint32 {
			return 0, fmt.Errorf("LXP_OVERFLOW: layerxpay call ordinal exhausted")
		}
		ordinal = last + 1
	}
	var encoded [4]byte
	binary.BigEndian.PutUint32(encoded[:], ordinal)
	store.Set(key, encoded[:])
	return ordinal, nil
}
