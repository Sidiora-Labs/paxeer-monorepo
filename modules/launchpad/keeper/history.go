package keeper

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

// holdingHistoryVersion tags the holding-history marker encoding.
const holdingHistoryVersion = 1

// holdingHistory is a launched denom's marker: balances are checkpointed
// from boundary FirstEpoch on, and CurrentEpoch is the epoch being recorded.
type holdingHistory struct {
	FirstEpoch   uint64
	CurrentEpoch uint64
}

type holdingEntry struct {
	Epoch   uint64
	Balance sdk.Int
}

func (k *Keeper) isLaunchedDenom(denom string) bool {
	return strings.HasPrefix(denom, "factory/"+k.ModuleAddress().String()+"/")
}

func (k *Keeper) getHoldingHistory(ctx sdk.Context, denom string) (holdingHistory, bool, error) {
	bz := k.store(ctx).Get(types.HoldingHistoryKey(denom))
	if bz == nil {
		return holdingHistory{}, false, nil
	}
	if len(bz) != 17 || bz[0] != holdingHistoryVersion {
		return holdingHistory{}, false, fmt.Errorf("%w: malformed marker for %s", types.ErrHoldingHistory, denom)
	}
	history := holdingHistory{FirstEpoch: binary.BigEndian.Uint64(bz[1:9]), CurrentEpoch: binary.BigEndian.Uint64(bz[9:17])}
	if history.FirstEpoch == 0 || history.FirstEpoch > history.CurrentEpoch {
		return holdingHistory{}, false, fmt.Errorf("%w: marker epochs out of order for %s", types.ErrHoldingHistory, denom)
	}
	return history, true, nil
}

func (k *Keeper) setHoldingHistory(ctx sdk.Context, denom string, history holdingHistory) {
	bz := []byte{holdingHistoryVersion}
	bz = binary.BigEndian.AppendUint64(bz, history.FirstEpoch)
	bz = binary.BigEndian.AppendUint64(bz, history.CurrentEpoch)
	k.store(ctx).Set(types.HoldingHistoryKey(denom), bz)
}

// HoldingCheckpointCount is the number of balance checkpoints recorded for
// holder's denom.
func (k *Keeper) HoldingCheckpointCount(ctx sdk.Context, denom string, holder sdk.AccAddress) (uint64, error) {
	bz := k.store(ctx).Get(types.HoldingCountKey(denom, holder))
	if bz == nil {
		return 0, nil
	}
	if len(bz) != 8 {
		return 0, fmt.Errorf("%w: malformed checkpoint count", types.ErrHoldingHistory)
	}
	return binary.BigEndian.Uint64(bz), nil
}

func (k *Keeper) getHoldingEntry(ctx sdk.Context, denom string, holder sdk.AccAddress, index uint64) (holdingEntry, error) {
	bz := k.store(ctx).Get(types.HoldingEntryKey(denom, holder, index))
	if len(bz) < 9 {
		return holdingEntry{}, fmt.Errorf("%w: missing or malformed checkpoint %d", types.ErrHoldingHistory, index)
	}
	var balance sdk.Int
	if err := balance.Unmarshal(bz[8:]); err != nil || balance.IsNil() || balance.IsNegative() {
		return holdingEntry{}, fmt.Errorf("%w: malformed checkpoint %d balance", types.ErrHoldingHistory, index)
	}
	return holdingEntry{Epoch: binary.BigEndian.Uint64(bz[:8]), Balance: balance}, nil
}

// BeforeBalanceChange is the bank balance-change hook. For a launched denom
// with an open history it records the holder's balance before its first
// write in the current epoch, on the caller's metered context.
func (k *Keeper) BeforeBalanceChange(ctx sdk.Context, addr sdk.AccAddress, denom string, before func() sdk.Int) error {
	if !k.isLaunchedDenom(denom) {
		return nil
	}
	history, found, err := k.getHoldingHistory(ctx, denom)
	if err != nil || !found {
		return err
	}
	count, err := k.HoldingCheckpointCount(ctx, denom, addr)
	if err != nil {
		return err
	}
	if count > 0 {
		last, err := k.getHoldingEntry(ctx, denom, addr, count-1)
		if err != nil {
			return err
		}
		if last.Epoch > history.CurrentEpoch {
			return fmt.Errorf("%w: checkpoint epoch %d after recorded epoch %d", types.ErrHoldingHistory, last.Epoch, history.CurrentEpoch)
		}
		if last.Epoch == history.CurrentEpoch {
			return nil
		}
	}
	if count == math.MaxUint64 {
		return fmt.Errorf("%w: checkpoint count overflow", types.ErrHoldingHistory)
	}
	balance := before()
	if balance.IsNil() || balance.IsNegative() {
		return fmt.Errorf("%w: invalid balance before change", types.ErrHoldingHistory)
	}
	amount, err := balance.Marshal()
	if err != nil {
		return err
	}
	store := k.store(ctx)
	store.Set(types.HoldingEntryKey(denom, addr, count), append(binary.BigEndian.AppendUint64(nil, history.CurrentEpoch), amount...))
	store.Set(types.HoldingCountKey(denom, addr), binary.BigEndian.AppendUint64(nil, count+1))
	return nil
}
