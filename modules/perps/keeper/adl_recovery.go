package keeper

import (
	"bytes"
	"encoding/binary"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/holiman/uint256"
)

var adlBlockBudgetKey = []byte{0x18}

func previousMarkKey(marketID uint64) []byte {
	return binary.BigEndian.AppendUint64([]byte{0x16}, marketID)
}

func previousMarkValue(height int64, mark uint256.Int) []byte {
	word := mark.Bytes32()
	return append(binary.BigEndian.AppendUint64(nil, uint64(height)), word[:]...)
}

func (k *Keeper) snapshotCommittedMark(ctx sdk.Context, state types.MarketState) error {
	if state.MarkPrice.IsZero() {
		return nil
	}
	key := previousMarkKey(state.MarketID)
	raw := k.store(ctx).Get(key)
	if raw != nil && len(raw) != 40 {
		return types.ErrInvalidParams.Wrapf("previous mark of market %d", state.MarketID)
	}
	if raw == nil || binary.BigEndian.Uint64(raw[:8]) != uint64(ctx.BlockHeight()) {
		k.store(ctx).Set(key, previousMarkValue(ctx.BlockHeight(), state.MarkPrice))
	}
	return nil
}

// PreviousCommittedMark is the durable mark from before this block's price
// update. It is distinct from the live mark consumed by margin and matching.
func (k *Keeper) PreviousCommittedMark(ctx sdk.Context, marketID uint64) (uint256.Int, error) {
	raw := k.store(ctx).Get(previousMarkKey(marketID))
	if raw == nil {
		return uint256.Int{}, types.ErrPriceStale.Wrapf("no committed mark for market %d", marketID)
	}
	if len(raw) != 40 {
		return uint256.Int{}, types.ErrInvalidParams.Wrapf("previous mark of market %d", marketID)
	}
	var mark uint256.Int
	mark.SetBytes(raw[8:])
	if mark.IsZero() {
		return mark, types.ErrPriceStale.Wrapf("zero committed mark for market %d", marketID)
	}
	return mark, nil
}

type adlBlockBudget struct{ height, visits, matches uint64 }

func (k *Keeper) loadADLBudget(ctx sdk.Context) (adlBlockBudget, error) {
	budget := adlBlockBudget{height: uint64(ctx.BlockHeight())}
	raw := k.store(ctx).Get(adlBlockBudgetKey)
	if raw == nil {
		return budget, nil
	}
	if len(raw) != 24 {
		return budget, types.ErrInvalidParams.Wrapf("ADL block budget")
	}
	if binary.BigEndian.Uint64(raw[:8]) == budget.height {
		budget.visits = binary.BigEndian.Uint64(raw[8:16])
		budget.matches = binary.BigEndian.Uint64(raw[16:24])
	}
	return budget, nil
}

func (k *Keeper) storeADLBudget(ctx sdk.Context, budget adlBlockBudget) {
	raw := binary.BigEndian.AppendUint64(nil, budget.height)
	raw = binary.BigEndian.AppendUint64(raw, budget.visits)
	raw = binary.BigEndian.AppendUint64(raw, budget.matches)
	k.store(ctx).Set(adlBlockBudgetKey, raw)
}

func adlCursorKey(deficitID uint256.Int) []byte {
	word := deficitID.Bytes32()
	return append([]byte{0x17}, word[:]...)
}

func adlScanStart(store sdk.KVStore, deficitID uint256.Int, prefix []byte) ([]byte, error) {
	cursor := store.Get(adlCursorKey(deficitID))
	if cursor == nil {
		return prefix, nil
	}
	if len(cursor) != len(prefix)+64 || !bytes.HasPrefix(cursor, prefix) {
		return nil, types.ErrInvalidParams.Wrapf("ADL continuation of position %s", deficitID.Dec())
	}
	return append(append([]byte(nil), cursor...), 0), nil
}

var adlDeficitCursorKey = []byte{0x1a}

// Rotate deficits after the last visited position so an ineligible market
// cannot consume every block's shared allowance before another market runs.
func (k *Keeper) adlDeficitOrder(ctx sdk.Context, ids []uint256.Int) ([]uint256.Int, error) {
	raw := k.store(ctx).Get(adlDeficitCursorKey)
	if raw == nil || len(ids) == 0 {
		return ids, nil
	}
	if len(raw) != 32 {
		return nil, types.ErrInvalidParams.Wrapf("ADL deficit continuation")
	}
	var last uint256.Int
	last.SetBytes(raw)
	next := 0
	for next < len(ids) && ids[next].Cmp(&last) <= 0 {
		next++
	}
	ordered := make([]uint256.Int, 0, len(ids))
	ordered = append(ordered, ids[next:]...)
	ordered = append(ordered, ids[:next]...)
	return ordered, nil
}
