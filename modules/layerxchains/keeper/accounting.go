package keeper

import (
	"encoding/binary"
	"math"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

func (k Keeper) RecordDomainTx(ctx sdk.Context, id uint32, height uint64, txHash common.Hash, gasUsed uint64) {
	d, ok := k.GetDomain(ctx, id)
	if !ok {
		panic(types.ErrUnknownDomain)
	}
	if d.Status != types.StatusActive {
		panic(types.ErrDomainNotActive)
	}
	previous := k.DomainGasUsed(ctx, id, height)
	if gasUsed > math.MaxUint64-previous {
		panic("domain gas used overflow")
	}
	store := ctx.KVStore(k.storeKey)
	key := types.HeightKey(types.DomainTxPrefix, id, height)
	hashes := append(append([]byte{}, store.Get(key)...), txHash.Bytes()...)
	used := make([]byte, 8)
	binary.BigEndian.PutUint64(used, previous+gasUsed)
	store.Set(key, hashes)
	store.Set(types.HeightKey(types.DomainGasUsedPrefix, id, height), used)
}
func (k Keeper) DomainTxs(ctx sdk.Context, id uint32, height uint64) []common.Hash {
	bz := ctx.KVStore(k.storeKey).Get(types.HeightKey(types.DomainTxPrefix, id, height))
	hashes := make([]common.Hash, 0, len(bz)/common.HashLength)
	for offset := 0; offset < len(bz); offset += common.HashLength {
		hashes = append(hashes, common.BytesToHash(bz[offset:offset+common.HashLength]))
	}
	return hashes
}
func (k Keeper) DomainGasUsed(ctx sdk.Context, id uint32, height uint64) uint64 {
	bz := ctx.KVStore(k.storeKey).Get(types.HeightKey(types.DomainGasUsedPrefix, id, height))
	if bz == nil {
		return 0
	}
	return binary.BigEndian.Uint64(bz)
}
func (k Keeper) NextBaseFee(ctx sdk.Context, id uint32) sdk.Dec {
	bz := ctx.KVStore(k.storeKey).Get(types.IDKey(types.DomainNextBaseFeePrefix, id))
	if bz == nil {
		panic(types.ErrUnknownDomain)
	}
	var fee sdk.Dec
	readJSON(bz, &fee)
	return fee
}
func (k Keeper) SetNextBaseFee(ctx sdk.Context, id uint32, fee sdk.Dec) {
	if _, ok := k.GetDomain(ctx, id); !ok {
		panic(types.ErrUnknownDomain)
	}
	if fee.IsNil() || fee.IsNegative() {
		panic("invalid base fee")
	}
	putJSON(ctx, k.storeKey, types.IDKey(types.DomainNextBaseFeePrefix, id), fee)
}
func (k Keeper) AdjustDomainBaseFee(ctx sdk.Context, id uint32, gasUsed uint64, minimum, maximum, upward, downward sdk.Dec) sdk.Dec {
	d, ok := k.GetDomain(ctx, id)
	if !ok {
		panic(types.ErrUnknownDomain)
	}
	if minimum.IsNil() || maximum.IsNil() || upward.IsNil() || downward.IsNil() || minimum.IsNegative() || maximum.LT(minimum) || upward.IsNegative() || upward.GT(sdk.OneDec()) || downward.IsNegative() || downward.GT(sdk.OneDec()) {
		panic("invalid base fee bounds or adjustments")
	}
	fee := k.NextBaseFee(ctx, id)
	if d.TargetGasPerBlock == 0 {
		return fee
	}
	if gasUsed > d.GasAllowance {
		gasUsed = d.GasAllowance
	}
	limit := sdk.NewDecFromInt(sdk.NewIntFromUint64(d.GasAllowance))
	target := sdk.NewDecFromInt(sdk.NewIntFromUint64(d.TargetGasPerBlock))
	used := sdk.NewDecFromInt(sdk.NewIntFromUint64(gasUsed))
	if used.GT(target) {
		fee = fee.Mul(sdk.OneDec().Add(upward.Mul(used.Sub(target).Quo(limit.Sub(target)))))
	} else {
		fee = fee.Mul(sdk.OneDec().Sub(downward.Mul(target.Sub(used).Quo(target))))
	}
	if fee.LT(minimum) {
		fee = minimum
	}
	if fee.GT(maximum) {
		fee = maximum
	}
	k.SetNextBaseFee(ctx, id, fee)
	return fee
}
