package keeper

import (
	"encoding/binary"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

// retirementHeightKey is the imported custody lifecycle marker. Runtime
// activation continues to come from the upgrade keeper's done height.
var retirementHeightKey = []byte{0x04}

// RetirementGenesis carries activation provenance separately from historical
// custody records. The module genesis adapter must transport both fields.
type RetirementGenesis struct {
	State     types.GenesisState `json:"state"`
	RetiredAt int64              `json:"retired_at,string"`
}

func (gs RetirementGenesis) Validate() error {
	if err := gs.State.Validate(); err != nil {
		return err
	}
	if gs.RetiredAt < 0 {
		return sdkerrors.Wrap(types.ErrInvalidGenesis, "negative retirement height")
	}
	if gs.RetiredAt > 0 {
		for _, claim := range gs.State.Claims {
			if claim.Status == types.ClaimStatus_CLAIM_STATUS_PENDING {
				return sdkerrors.Wrap(types.ErrInvalidGenesis, "retired custody has pending claims")
			}
		}
		for _, totals := range gs.State.Totals {
			custodied, _ := types.ParseAmount(totals.Custodied)
			pending, _ := types.ParseAmount(totals.Pending)
			if !custodied.IsZero() || !pending.IsZero() {
				return sdkerrors.Wrap(types.ErrInvalidGenesis, "retired custody has unreleased totals")
			}
		}
	}
	return nil
}

func (k *Keeper) retirementHeight(ctx sdk.Context) int64 {
	value := k.store(ctx).Get(retirementHeightKey)
	if value == nil {
		return 0
	}
	return int64(binary.BigEndian.Uint64(value)) // stored only after non-negative validation
}

// ExportGenesisWithRetirement retains the activation the upgrade module's
// historical empty genesis omits. It never infers retirement from asset totals.
func (k *Keeper) ExportGenesisWithRetirement(ctx sdk.Context) (*RetirementGenesis, error) {
	gs := &RetirementGenesis{State: *k.ExportGenesis(ctx), RetiredAt: k.retirementHeight(ctx)}
	if gs.RetiredAt == 0 && k.Retired(ctx) {
		reader, ok := k.upgrade.(interface {
			GetDoneHeight(sdk.Context, string) int64
		})
		if !ok {
			return nil, sdkerrors.Wrap(types.ErrInvalidGenesis, "retirement activation height unavailable")
		}
		gs.RetiredAt = reader.GetDoneHeight(ctx, types.MergePlanUpgrade)
		if gs.RetiredAt <= 0 {
			return nil, sdkerrors.Wrap(types.ErrInvalidGenesis, "retirement activation height unavailable")
		}
	}
	if err := gs.Validate(); err != nil {
		return nil, err
	}
	return gs, nil
}

// InitGenesisWithRetirement is an initialization entry point, never a runtime
// mutation API. Validation precedes all writes; importing cannot clear a marker.
func (k *Keeper) InitGenesisWithRetirement(ctx sdk.Context, gs RetirementGenesis) error {
	if err := gs.Validate(); err != nil {
		return err
	}
	if existing := k.retirementHeight(ctx); existing > 0 && existing != gs.RetiredAt {
		return sdkerrors.Wrap(types.ErrInvalidGenesis, "conflicting retirement activation")
	}
	if gs.RetiredAt == 0 && k.Retired(ctx) {
		return sdkerrors.Wrap(types.ErrInvalidGenesis, "cannot clear retirement activation")
	}
	if reader, ok := k.upgrade.(interface {
		GetDoneHeight(sdk.Context, string) int64
	}); ok {
		if height := reader.GetDoneHeight(ctx, types.MergePlanUpgrade); height > 0 && gs.RetiredAt > 0 && height != gs.RetiredAt {
			return sdkerrors.Wrap(types.ErrInvalidGenesis, "conflicting upgrade activation")
		}
	}
	_, err := atomically(ctx, func(ctx sdk.Context) (struct{}, error) {
		if gs.RetiredAt > 0 {
			k.store(ctx).Set(retirementHeightKey, binary.BigEndian.AppendUint64(nil, uint64(gs.RetiredAt)))
		}
		k.InitGenesis(ctx, gs.State)
		return struct{}{}, nil
	})
	return err
}
