package keeper

import (
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/holiman/uint256"
)

// openInterestScale is 1e18, the fixed-point scale of prices and base sizes.
var openInterestScale = uint256.NewInt(1_000_000_000_000_000_000)

// MarketOpenInterest is the open interest of one market: the long and short
// sides in 18-decimal base units and in USD at the mark price. Every fill adds
// or removes the same size on both sides, so the long and short values are
// equal. The venue keeps no borrowing fee and no borrowing index; funding is
// the only carry.
type MarketOpenInterest struct {
	LongBase  uint256.Int
	ShortBase uint256.Int
	LongUsd   uint256.Int
	ShortUsd  uint256.Int
}

// OpenInterestFill is the open-interest effect of one fill of SizeBase at
// PriceUsd between a buyer and a seller: BuyerOpensBase is the part that opens
// or increases the buyer's long and SellerOpensBase the part that opens or
// increases the seller's short; the rest of each party's size closes its
// opposite position. Both sides change by BuyerOpensBase + SellerOpensBase -
// SizeBase.
type OpenInterestFill struct {
	MarketID        uint64
	SizeBase        uint256.Int
	PriceUsd        uint256.Int
	BuyerOpensBase  uint256.Int
	SellerOpensBase uint256.Int
}

// OpenInterest returns the market's open interest in base units and in USD at
// the mark of its market state.
func (k *Keeper) OpenInterest(ctx sdk.Context, marketID uint64) (MarketOpenInterest, error) {
	if _, err := k.GetMarket(ctx, marketID); err != nil {
		return MarketOpenInterest{}, err
	}
	state, err := k.openInterestState(ctx, marketID)
	if err != nil {
		return MarketOpenInterest{}, err
	}
	return openInterestAtMark(state)
}

// OpenInterestCap is the per-side open-interest cap in force at the block
// time: the fixed cap of classes A to D and F, or the listing-age row of
// Params.ClassEOiCaps for class E.
func (k *Keeper) OpenInterestCap(ctx sdk.Context, marketID uint64) (sdk.Int, error) {
	market, err := k.GetMarket(ctx, marketID)
	if err != nil {
		return sdk.Int{}, err
	}
	params, err := k.GetParams(ctx)
	if err != nil {
		return sdk.Int{}, err
	}
	return types.OiCapInForce(market, params, ctx.BlockTime())
}

// CheckOpenInterest books the open-interest change of a fill. A fill that
// raises open interest is refused with ErrOpenInterestCap when the new side,
// in USD at mark, would exceed the cap in force; a fill that leaves open
// interest unchanged or lowers it always passes, which covers every
// reduce-only, liquidation, backstop and ADL fill because one party of each
// only closes. It returns the OpenInterestChanged record of the change, with
// both sides after the fill in USD at mark and deltaUsd the changed size at
// the fill price, or nil when the fill leaves open interest unchanged. A
// refused fill writes nothing.
func (k *Keeper) CheckOpenInterest(ctx sdk.Context, fill OpenInterestFill) (*types.EventRecord, error) {
	if fill.SizeBase.IsZero() || fill.PriceUsd.IsZero() {
		return nil, types.ErrZeroAmount.Wrapf("fill of market %d needs a size and a price", fill.MarketID)
	}
	if fill.BuyerOpensBase.Gt(&fill.SizeBase) || fill.SellerOpensBase.Gt(&fill.SizeBase) {
		return nil, types.ErrInvalidParams.Wrapf("opening size above fill size %s in market %d", fill.SizeBase.Dec(), fill.MarketID)
	}
	var opens uint256.Int
	if _, overflow := opens.AddOverflow(&fill.BuyerOpensBase, &fill.SellerOpensBase); overflow {
		return nil, types.ErrOverflow.Wrapf("opening size of market %d", fill.MarketID)
	}
	if opens.Eq(&fill.SizeBase) {
		return nil, nil
	}
	isIncrease := opens.Gt(&fill.SizeBase)
	var delta uint256.Int
	if isIncrease {
		delta.Sub(&opens, &fill.SizeBase)
	} else {
		delta.Sub(&fill.SizeBase, &opens)
	}

	if _, err := k.GetMarket(ctx, fill.MarketID); err != nil {
		return nil, err
	}
	state, err := k.openInterestState(ctx, fill.MarketID)
	if err != nil {
		return nil, err
	}
	var side uint256.Int
	if isIncrease {
		if _, overflow := side.AddOverflow(&state.LongOpenInterest, &delta); overflow {
			return nil, types.ErrOverflow.Wrapf("open interest of market %d", fill.MarketID)
		}
		if err := k.checkOpenInterestCap(ctx, state, side); err != nil {
			return nil, err
		}
	} else {
		if delta.Gt(&state.LongOpenInterest) {
			return nil, types.ErrOverflow.Wrapf("fill removes %s base above the open interest %s of market %d", delta.Dec(), state.LongOpenInterest.Dec(), fill.MarketID)
		}
		side.Sub(&state.LongOpenInterest, &delta)
	}

	deltaUsd, overflow := new(uint256.Int).MulDivOverflow(&delta, &fill.PriceUsd, openInterestScale)
	if overflow {
		return nil, types.ErrOverflow.Wrapf("open interest change of market %d in USD", fill.MarketID)
	}
	state.LongOpenInterest = side
	state.ShortOpenInterest = side
	after, err := openInterestAtMark(state)
	if err != nil {
		return nil, err
	}
	ctx.KVStore(k.storeKey).Set(types.MarketStateKey(fill.MarketID), state.Marshal())
	return &types.EventRecord{
		Name: types.EventOpenInterestChanged,
		Args: []any{
			new(big.Int).SetUint64(fill.MarketID), after.LongUsd.ToBig(), after.ShortUsd.ToBig(),
			deltaUsd.ToBig(), isIncrease,
		},
	}, nil
}

// checkOpenInterestCap refuses a side of newSideBase whose USD value at mark
// exceeds the cap in force; a side exactly at the cap passes.
func (k *Keeper) checkOpenInterestCap(ctx sdk.Context, state types.MarketState, newSideBase uint256.Int) error {
	if state.MarkPrice.IsZero() {
		return types.ErrPriceStale.Wrapf("market %d has no mark price", state.MarketID)
	}
	capUsd, err := k.OpenInterestCap(ctx, state.MarketID)
	if err != nil {
		return err
	}
	sideUsd, overflow := new(uint256.Int).MulDivOverflow(&newSideBase, &state.MarkPrice, openInterestScale)
	if overflow || sideUsd.ToBig().Cmp(capUsd.BigInt()) > 0 {
		return types.ErrOpenInterestCap.Wrapf("market %d side above cap %s USD", state.MarketID, capUsd)
	}
	return nil
}

// openInterestState reads the market state, a zero state before the first
// write, and refuses a record whose sides differ.
func (k *Keeper) openInterestState(ctx sdk.Context, marketID uint64) (types.MarketState, error) {
	state := types.MarketState{MarketID: marketID}
	bz := ctx.KVStore(k.storeKey).Get(types.MarketStateKey(marketID))
	if bz == nil {
		return state, nil
	}
	if err := state.Unmarshal(bz); err != nil {
		return types.MarketState{}, types.ErrInvalidParams.Wrapf("market state %d: %v", marketID, err)
	}
	if state.MarketID != marketID || !state.LongOpenInterest.Eq(&state.ShortOpenInterest) {
		return types.MarketState{}, types.ErrInvalidParams.Wrapf("market state %d holds market %d with long %s and short %s", marketID, state.MarketID, state.LongOpenInterest.Dec(), state.ShortOpenInterest.Dec())
	}
	return state, nil
}

func openInterestAtMark(state types.MarketState) (MarketOpenInterest, error) {
	out := MarketOpenInterest{LongBase: state.LongOpenInterest, ShortBase: state.ShortOpenInterest}
	if _, overflow := out.LongUsd.MulDivOverflow(&state.LongOpenInterest, &state.MarkPrice, openInterestScale); overflow {
		return MarketOpenInterest{}, types.ErrOverflow.Wrapf("long open interest of market %d in USD", state.MarketID)
	}
	if _, overflow := out.ShortUsd.MulDivOverflow(&state.ShortOpenInterest, &state.MarkPrice, openInterestScale); overflow {
		return MarketOpenInterest{}, types.ErrOverflow.Wrapf("short open interest of market %d in USD", state.MarketID)
	}
	return out, nil
}
