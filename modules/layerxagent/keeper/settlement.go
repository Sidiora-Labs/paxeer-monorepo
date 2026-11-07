package keeper

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/prefix"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

var SettlementKeyPrefix = []byte{0x06}

// Settlement records an executed job's outcome and its two independent payments.
// Recipients are resolved at execution time, so retries cannot redirect a debt.
type Settlement struct {
	JobID           uint64         `json:"job_id"`
	Creator         common.Address `json:"creator"`
	GasUsed         uint64         `json:"gas_used"`
	Reason          string         `json:"reason"`
	FeeRecipient    sdk.AccAddress `json:"fee_recipient"`
	RefundRecipient sdk.AccAddress `json:"refund_recipient"`
	Fee             sdk.Int        `json:"fee"`
	Refund          sdk.Int        `json:"refund"`
	FeePaid         bool           `json:"fee_paid"`
	RefundPaid      bool           `json:"refund_paid"`
}

func settlementKey(id uint64) []byte {
	return binary.BigEndian.AppendUint64(append([]byte{}, SettlementKeyPrefix...), id)
}

func (k Keeper) setSettlement(ctx sdk.Context, obligation Settlement) {
	encoded, err := json.Marshal(obligation)
	if err != nil {
		panic(fmt.Errorf("layerxagent settlement %d: %w", obligation.JobID, err))
	}
	k.store(ctx).Set(settlementKey(obligation.JobID), encoded)
}

func (k Keeper) GetSettlement(ctx sdk.Context, id uint64) (Settlement, bool) {
	encoded := k.store(ctx).Get(settlementKey(id))
	if encoded == nil {
		return Settlement{}, false
	}
	var obligation Settlement
	if err := json.Unmarshal(encoded, &obligation); err != nil {
		panic(fmt.Errorf("layerxagent settlement %d: %w", id, err))
	}
	return obligation, true
}

func (k Keeper) settlementIDs(ctx sdk.Context, limit uint64) []uint64 {
	iterator := prefix.NewStore(k.store(ctx), SettlementKeyPrefix).Iterator(nil, nil)
	defer func() { _ = iterator.Close() }()
	var ids []uint64
	for ; iterator.Valid() && uint64(len(ids)) < limit; iterator.Next() {
		ids = append(ids, binary.BigEndian.Uint64(iterator.Key()))
	}
	return ids
}

// RetrySettlements retries only payments, never EVM execution. A failed oldest
// payment stops this bounded pass so that newer debts cannot consume its escrow.
func (k Keeper) RetrySettlements(ctx sdk.Context, limit uint64) error {
	for _, id := range k.settlementIDs(ctx, limit) {
		obligation, found := k.GetSettlement(ctx, id)
		if !found {
			continue
		}
		if err := k.settleJob(ctx, obligation); err != nil {
			return fmt.Errorf("job %d settlement: %w", id, err)
		}
	}
	return nil
}

func (k Keeper) settleJob(ctx sdk.Context, obligation Settlement) error {
	var errs []error
	// Each transfer and its receipt commit in the same cache. A failed leg
	// leaves both the bank and its outstanding obligation unchanged.
	for _, feeLeg := range []bool{true, false} {
		if feeLeg && obligation.FeePaid || !feeLeg && obligation.RefundPaid {
			continue
		}
		amount, to, name := obligation.Refund, obligation.RefundRecipient, "refund"
		if feeLeg {
			amount, to, name = obligation.Fee, obligation.FeeRecipient, "fee"
		}
		cached, write := ctx.CacheContext()
		if err := k.transfer(cached, k.ModuleAddress(), to, amount); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			continue
		}
		if feeLeg {
			obligation.FeePaid = true
		} else {
			obligation.RefundPaid = true
		}
		k.setSettlement(cached, obligation)
		if obligation.FeePaid && obligation.RefundPaid {
			k.store(cached).Delete(settlementKey(obligation.JobID))
		}
		write()
		ctx.EventManager().EmitEvents(cached.EventManager().Events())
	}
	return errors.Join(errs...)
}
