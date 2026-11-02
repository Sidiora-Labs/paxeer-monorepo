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
	"github.com/ethereum/go-ethereum/crypto"
)

const payCallOrdinalPrefix = "layerxpay/ordinal/v1/"

func (k Keeper) BudgetSpend(ctx sdk.Context, caller common.Address, budgetID, recipient [32]byte, amount codec.U128) (lxphost.Receipt, error) {
	payload, err := types.EncodeBudgetSpend(budgetID, recipient, amount)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "budgetSpend", payload, [32]byte{})
}

func (k Keeper) BudgetDelegateAdd(ctx sdk.Context, caller common.Address, budgetID, delegate [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeBudgetDelegate(budgetID, delegate)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "budgetDelegateAdd", payload, [32]byte{})
}

func (k Keeper) BudgetDelegateRemove(ctx sdk.Context, caller common.Address, budgetID, delegate [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeBudgetDelegate(budgetID, delegate)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "budgetDelegateRemove", payload, [32]byte{})
}

func (k Keeper) BudgetDelegateSpend(ctx sdk.Context, caller common.Address, budgetID, recipient [32]byte, amount codec.U128) (lxphost.Receipt, error) {
	key, err := nextPayIdempotencyKey(ctx, [4]byte(crypto.Keccak256([]byte("budgetDelegateSpend(bytes32,bytes32,uint128)"))[:4]))
	if err != nil {
		return lxphost.Receipt{}, err
	}
	payload, err := types.EncodeBudgetDelegateSpend(budgetID, recipient, amount, key)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "budgetDelegateSpend", payload, key)
}

func nextPayIdempotencyKey(ctx sdk.Context, selector [4]byte) ([32]byte, error) {
	txHash := ctx.EVMTxHash()
	if txHash == (common.Hash{}) {
		return [32]byte{}, fmt.Errorf("LXP_NON_CANONICAL: layerxpay missing transaction hash")
	}
	var transientKey *sdk.TransientStoreKey
	for _, key := range ctx.MultiStore().StoreKeys() {
		if key.Name() == evmtypes.TransientStoreKey {
			transientKey, _ = key.(*sdk.TransientStoreKey)
			break
		}
	}
	if transientKey == nil {
		return [32]byte{}, fmt.Errorf("LXP_NON_CANONICAL: layerxpay EVM transient store is not mounted")
	}
	store := ctx.TransientStore(transientKey)
	key := append([]byte(payCallOrdinalPrefix), txHash[:]...)
	var ordinal uint32
	if previous := store.Get(key); previous != nil {
		if len(previous) != 4 {
			return [32]byte{}, fmt.Errorf("LXP_NON_CANONICAL: layerxpay invalid call ordinal")
		}
		last := binary.BigEndian.Uint32(previous)
		if last == math.MaxUint32 {
			return [32]byte{}, fmt.Errorf("LXP_OVERFLOW: layerxpay call ordinal exhausted")
		}
		ordinal = last + 1
	}
	var encoded [4]byte
	binary.BigEndian.PutUint32(encoded[:], ordinal)
	store.Set(key, encoded[:])
	return types.DeriveIdempotencyKey(txHash, ordinal, selector), nil
}
