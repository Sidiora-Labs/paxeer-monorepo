package keeper

import (
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
)

// matchUnit is 1e18, the fixed-point scale of prices and base sizes.
var matchUnit = uint256.NewInt(1_000_000_000_000_000_000)

// FillResult is one maker-taker match of contracts/perps.md section 6: SizeBase
// at the maker's Price, the fill notional SizeUsd = SizeBase x Price / 1e18
// rounded down and booked once to both parties, the maker fee (negative for a
// rebate) and the taker fee of section 10, and the positions the fill settled.
type FillResult struct {
	MarketID        uint64
	MakerOrderID    uint256.Int
	TakerOrderID    uint256.Int
	Maker           common.Address
	Taker           common.Address
	TakerIsLong     bool
	Price           uint256.Int
	SizeBase        uint256.Int
	SizeUsd         uint256.Int
	MakerFeeUsd     *big.Int
	TakerFeeUsd     uint256.Int
	MakerPositionID uint256.Int
	TakerPositionID uint256.Int
}

// MatchResult is the outcome of one taker order: its final record, the fills
// in match order, the ids of the orders cancelled (resting orders of the same
// account first, then the taker's own remainder), whether the remainder rests
// in the book, and the event records in emission order.
type MatchResult struct {
	Taker     types.Order
	Fills     []FillResult
	Cancelled []uint256.Int
	Rested    bool
	Events    []types.EventRecord
}

// Match runs a taker order against the opposite side of its market's book in
// price-time priority, filling at the maker's price while the maker price
// crosses the order's slippage bound, and applies every fill through
// CheckOpenInterest, ApplyFill and the maker and taker fees. A resting order of
// the taker's own account is cancelled and matching continues; at most
// Params.MaxFillsPerOrder fills are made. A GTC or POST_ONLY limit remainder
// rests unless the fill cap stopped matching against a crossing order; every
// other remainder is cancelled. POST_ONLY orders that would cross fail with
// ErrPostOnlyWouldCross, FOK orders that cannot fill completely with
// ErrFokNotFilled, non-resting orders meeting an empty opposite side with
// ErrNoLiquidity and non-resting orders filling nothing with
// ErrSlippageExceeded. A failed match writes nothing.
func (k *Keeper) Match(ctx sdk.Context, market types.Market, taker types.Order) (MatchResult, error) {
	return k.matchKind(ctx, market, taker, FillKindTrade)
}

// matchKind is Match with the taker's fills applied as kind; makers always
// fill as FillKindTrade.
func (k *Keeper) matchKind(ctx sdk.Context, market types.Market, taker types.Order, kind FillKind) (MatchResult, error) {
	cacheCtx, write := ctx.CacheContext()
	result, err := k.match(cacheCtx, market, taker, kind)
	if err != nil {
		return MatchResult{}, err
	}
	write()
	return result, nil
}

func (k *Keeper) match(ctx sdk.Context, market types.Market, taker types.Order, kind FillKind) (MatchResult, error) {
	if err := orderDeadline(ctx, taker); err != nil {
		return MatchResult{}, err
	}
	remaining, err := validateTaker(market, taker)
	if err != nil {
		return MatchResult{}, err
	}
	params, err := k.GetParams(ctx)
	if err != nil {
		return MatchResult{}, err
	}
	bound, err := k.slippageBound(ctx, market, taker, params.DefaultSlippagePpm)
	if err != nil {
		return MatchResult{}, err
	}
	best, hasBest, err := k.bestOpposite(ctx, market.ID, taker.IsLong)
	if err != nil {
		return MatchResult{}, err
	}
	resting := restsRemainder(taker)
	if taker.TimeInForce == types.TimeInForcePostOnly && hasBest && crossesBound(taker.IsLong, best, bound) {
		return MatchResult{}, types.ErrPostOnlyWouldCross.Wrapf("order %s crosses best price %s", taker.ID.Dec(), best.Dec())
	}
	if !resting && !hasBest {
		return MatchResult{}, types.ErrNoLiquidity.Wrapf("market %d opposite side of order %s is empty", market.ID, taker.ID.Dec())
	}

	var candidates []types.Order
	if hasBest && crossesBound(taker.IsLong, best, bound) {
		if candidates, err = k.crossingMakers(ctx, market.ID, taker, bound, remaining, params.MaxFillsPerOrder); err != nil {
			return MatchResult{}, err
		}
	}

	var (
		result         MatchResult
		filledBase     uint256.Int
		filledUsd      uint256.Int
		capped         bool
		takerPosition  uint256.Int
		lastTradePrice uint256.Int
		takerOpened    bool
	)
	for _, candidate := range candidates {
		if remaining.IsZero() {
			break
		}
		if uint64(len(result.Fills)) == params.MaxFillsPerOrder {
			capped = true
			break
		}
		// An earlier fill's ReconcileReduceOnly may have cancelled or shrunk a
		// reduce-only candidate of its account.
		maker, err := k.GetOrder(ctx, candidate.ID)
		if err != nil {
			return MatchResult{}, err
		}
		if maker.Status != types.OrderStatusResting || !maker.FilledBase.Lt(&maker.SizeBase) {
			continue
		}
		if err := orderDeadline(ctx, maker); err != nil {
			if err := k.withdrawOrder(ctx, maker, types.OrderStatusExpired); err != nil {
				return MatchResult{}, err
			}
			result.Events = append(result.Events, types.EventRecord{Name: types.EventOrderExpired, Args: []any{maker.ID.ToBig(), maker.Account, new(big.Int).SetUint64(maker.MarketID)}})
			continue
		}
		if maker.Account == taker.Account {
			event, err := k.cancelResting(ctx, maker)
			if err != nil {
				return MatchResult{}, err
			}
			result.Cancelled = append(result.Cancelled, maker.ID)
			result.Events = append(result.Events, event)
			continue
		}
		opens, err := k.openingBase(ctx, taker.Account, market.ID, taker.IsLong, remaining)
		if err != nil {
			return MatchResult{}, err
		}
		if !opens.IsZero() {
			takerOpened = true
		}
		fill, events, err := k.fillMaker(ctx, market.ID, maker, taker, remaining, kind)
		if err != nil {
			return MatchResult{}, err
		}
		result.Fills = append(result.Fills, fill)
		result.Events = append(result.Events, events...)
		remaining.Sub(&remaining, &fill.SizeBase)
		filledBase.Add(&filledBase, &fill.SizeBase)
		if _, overflow := filledUsd.AddOverflow(&filledUsd, &fill.SizeUsd); overflow {
			return MatchResult{}, types.ErrOverflow.Wrapf("filled notional of order %s", taker.ID.Dec())
		}
		takerPosition = fill.TakerPositionID
		lastTradePrice = fill.Price
	}

	if taker.TimeInForce == types.TimeInForceFOK && !remaining.IsZero() {
		return MatchResult{}, types.ErrFokNotFilled.Wrapf("order %s leaves %s base unfilled", taker.ID.Dec(), remaining.Dec())
	}
	if !resting && len(result.Fills) == 0 {
		return MatchResult{}, types.ErrSlippageExceeded.Wrapf("order %s fills nothing within bound %s", taker.ID.Dec(), bound.Dec())
	}

	taker.FilledBase.Add(&taker.FilledBase, &filledBase)
	if len(result.Fills) > 0 {
		vwap, overflow := new(uint256.Int).MulDivOverflow(&filledUsd, matchUnit, &filledBase)
		if overflow {
			return MatchResult{}, types.ErrOverflow.Wrapf("execution price of order %s", taker.ID.Dec())
		}
		result.Events = append(result.Events, orderExecuted(taker.ID, takerPosition, *vwap))
		attached, err := k.attachOrderTpSl(ctx, taker)
		if err != nil {
			return MatchResult{}, err
		}
		result.Events = append(result.Events, attached...)
		if err := k.recordFillVolume(ctx, result.Fills); err != nil {
			return MatchResult{}, err
		}
		if err := k.setLastTradePrice(ctx, market.ID, lastTradePrice); err != nil {
			return MatchResult{}, err
		}
	}
	switch {
	case remaining.IsZero():
		if err := k.closeOrder(ctx, taker, types.OrderStatusFilled); err != nil {
			return MatchResult{}, err
		}
		taker.Status = types.OrderStatusFilled
	case resting && !capped:
		taker.Status = types.OrderStatusResting
		k.store(ctx).Delete(types.OrderKey(taker.ID))
		if err := k.BookInsert(ctx, taker); err != nil {
			return MatchResult{}, err
		}
		result.Rested = true
	default:
		if err := k.closeOrder(ctx, taker, types.OrderStatusCancelled); err != nil {
			return MatchResult{}, err
		}
		taker.Status = types.OrderStatusCancelled
		result.Cancelled = append(result.Cancelled, taker.ID)
		result.Events = append(result.Events, orderCancelled(taker))
	}
	if len(result.Fills) > 0 {
		reconciled, err := k.ReconcileReduceOnly(ctx, taker.Account, market.ID)
		if err != nil {
			return MatchResult{}, err
		}
		result.Events = append(result.Events, reconciled...)
	}
	if takerOpened && kind == FillKindTrade {
		if err := k.checkFilledMargin(ctx, market, taker.Account); err != nil {
			return MatchResult{}, err
		}
	}
	result.Taker = taker
	return result, nil
}

// validateTaker checks the fields matching relies on and returns the unfilled
// base size of the order.
func validateTaker(market types.Market, taker types.Order) (uint256.Int, error) {
	if taker.MarketID != market.ID {
		return uint256.Int{}, types.ErrUnknownMarket.Wrapf("order %s of market %d matched in market %d", taker.ID.Dec(), taker.MarketID, market.ID)
	}
	if taker.ID.IsZero() {
		return uint256.Int{}, types.ErrUnknownOrder.Wrapf("order id 0")
	}
	if taker.Type > types.OrderTypeStopMarket {
		return uint256.Int{}, types.ErrInvalidOrderType.Wrapf("%d", taker.Type)
	}
	if taker.TimeInForce > types.TimeInForcePostOnly {
		return uint256.Int{}, types.ErrInvalidTimeInForce.Wrapf("%d", taker.TimeInForce)
	}
	if taker.TimeInForce == types.TimeInForcePostOnly && !isLimitType(taker.Type) {
		return uint256.Int{}, types.ErrInvalidTimeInForce.Wrapf("POST_ONLY order %s of type %d", taker.ID.Dec(), taker.Type)
	}
	if isLimitType(taker.Type) && taker.LimitPrice.IsZero() {
		return uint256.Int{}, types.ErrTickSize.Wrapf("limit order %s has no limit price", taker.ID.Dec())
	}
	if taker.FilledBase.Gt(&taker.SizeBase) {
		return uint256.Int{}, types.ErrInvalidParams.Wrapf("order %s filled %s above its size %s", taker.ID.Dec(), taker.FilledBase.Dec(), taker.SizeBase.Dec())
	}
	var remaining uint256.Int
	remaining.Sub(&taker.SizeBase, &taker.FilledBase)
	if remaining.IsZero() {
		return uint256.Int{}, types.ErrZeroAmount.Wrapf("order %s has nothing left to fill", taker.ID.Dec())
	}
	return remaining, nil
}

// isLimitType reports the order types that carry a limit price: LIMIT and a
// triggered STOP_LIMIT.
func isLimitType(orderType types.OrderType) bool {
	return orderType == types.OrderTypeLimit || orderType == types.OrderTypeStopLimit
}

// restsRemainder reports whether the unfilled remainder of the order rests:
// GTC and POST_ONLY limit orders rest, MARKET and triggered market orders are
// IOC whatever their time in force.
func restsRemainder(taker types.Order) bool {
	return isLimitType(taker.Type) &&
		(taker.TimeInForce == types.TimeInForceGTC || taker.TimeInForce == types.TimeInForcePostOnly)
}

// crossesBound is crosses of src/modules/perps/lx_perps_book.c:63 against the
// slippage bound: a buy takes asks at or below the bound, a sell bids at or
// above it.
func crossesBound(isBuy bool, makerPrice, bound uint256.Int) bool {
	if isBuy {
		return !makerPrice.Gt(&bound)
	}
	return !makerPrice.Lt(&bound)
}

// slippageBound is the one bound of contracts/perps.md section 6: the limit
// price of a limit order; otherwise acceptablePrice when non-zero, else mark x
// (1 +- DefaultSlippagePpm) rounded to the tick toward mark, down for a buy and
// up for a sell.
func (k *Keeper) slippageBound(ctx sdk.Context, market types.Market, taker types.Order, slippagePpm uint64) (uint256.Int, error) {
	if isLimitType(taker.Type) {
		return taker.LimitPrice, nil
	}
	if !taker.AcceptablePrice.IsZero() {
		return taker.AcceptablePrice, nil
	}
	if market.PxDecimals > types.USDDecimals {
		return uint256.Int{}, types.ErrInvalidParams.Wrapf("market %d pxDecimals %d above %d", market.ID,
			market.PxDecimals, types.USDDecimals)
	}
	if slippagePpm >= types.PpmDenominator {
		return uint256.Int{}, types.ErrInvalidParams.Wrapf("default slippage %d ppm", slippagePpm)
	}
	state, err := k.GetMarketState(ctx, market.ID)
	if err != nil {
		return uint256.Int{}, err
	}
	if state.MarkPrice.IsZero() {
		return uint256.Int{}, types.ErrPriceStale.Wrapf("market %d has no mark price", market.ID)
	}
	tick := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(types.USDDecimals-market.PxDecimals)), nil)
	factor := new(big.Int).SetUint64(types.PpmDenominator - slippagePpm)
	if taker.IsLong {
		factor.SetUint64(types.PpmDenominator + slippagePpm)
	}
	numerator := new(big.Int).Mul(state.MarkPrice.ToBig(), factor)
	denominator := new(big.Int).Mul(big.NewInt(types.PpmDenominator), tick)
	ticks, remainder := new(big.Int).QuoRem(numerator, denominator, new(big.Int))
	if !taker.IsLong && remainder.Sign() != 0 {
		ticks.Add(ticks, big.NewInt(1))
	}
	bound, overflow := uint256.FromBig(ticks.Mul(ticks, tick))
	if overflow {
		return uint256.Int{}, types.ErrOverflow.Wrapf("slippage bound of market %d", market.ID)
	}
	return *bound, nil
}

// bestOpposite is the best price of the side a taker order takes from.
func (k *Keeper) bestOpposite(ctx sdk.Context, marketID uint64, isBuy bool) (uint256.Int, bool, error) {
	if isBuy {
		return k.BestAsk(ctx, marketID)
	}
	return k.BestBid(ctx, marketID)
}

// crossingMakers collects the resting orders of the opposite side in book
// order up to the first price beyond the bound, stopping once the orders of
// other accounts cover the remainder or exceed the fill cap by one, the order
// that tells a capped match from an exhausted side.
func (k *Keeper) crossingMakers(ctx sdk.Context, marketID uint64, taker types.Order, bound, remaining uint256.Int,
	maxFills uint64) ([]types.Order, error) {
	opposite := types.SideBuy
	if taker.IsLong {
		opposite = types.SideSell
	}
	var (
		makers    []types.Order
		reach     uint256.Int
		others    uint64
		failure   error
		makerLeft uint256.Int
	)
	err := k.IterateBook(ctx, marketID, opposite, func(maker types.Order) bool {
		if !crossesBound(taker.IsLong, maker.LimitPrice, bound) {
			return true
		}
		if maker.MarketID != marketID || maker.IsLong == taker.IsLong || maker.Status != types.OrderStatusResting ||
			!maker.FilledBase.Lt(&maker.SizeBase) {
			failure = types.ErrInvalidParams.Wrapf("book entry of market %d holds order %s that is not resting there", marketID, maker.ID.Dec())
			return true
		}
		makers = append(makers, maker)
		if orderDeadline(ctx, maker) != nil {
			return false
		}
		if maker.Account == taker.Account {
			return false
		}
		others++
		makerLeft.Sub(&maker.SizeBase, &maker.FilledBase)
		if _, overflow := reach.AddOverflow(&reach, &makerLeft); overflow {
			failure = types.ErrOverflow.Wrapf("resting size of market %d", marketID)
			return true
		}
		return others > maxFills || !reach.Lt(&remaining)
	})
	if err != nil {
		return nil, err
	}
	if failure != nil {
		return nil, failure
	}
	return makers, nil
}

// fillMaker fills min(maker remainder, taker remainder) at the maker's price:
// the open-interest change is checked first against the positions before the
// fill, then both positions are settled at the fill price and the maker and
// taker fees are booked on the one fill notional.
func (k *Keeper) fillMaker(ctx sdk.Context, marketID uint64, maker, taker types.Order, takerLeft uint256.Int, kind FillKind) (FillResult, []types.EventRecord, error) {
	var makerLeft, size uint256.Int
	makerLeft.Sub(&maker.SizeBase, &maker.FilledBase)
	size = takerLeft
	if makerLeft.Lt(&takerLeft) {
		size = makerLeft
	}
	for _, order := range []types.Order{maker, taker} {
		if !order.ReduceOnly {
			continue
		}
		position, exists, err := k.orderPosition(ctx, order.Account, marketID)
		if err != nil {
			return FillResult{}, nil, err
		}
		if !exists || position.IsLong == order.IsLong || size.Gt(&position.SizeBase) {
			return FillResult{}, nil, types.ErrReduceOnlyViolation.Wrapf("fill %s exceeds live closing exposure of order %s", size.Dec(), order.ID.Dec())
		}
	}
	price := maker.LimitPrice
	notional, overflow := new(uint256.Int).MulDivOverflow(&size, &price, matchUnit)
	if overflow {
		return FillResult{}, nil, types.ErrOverflow.Wrapf("notional of fill of order %s", maker.ID.Dec())
	}
	if notional.IsZero() {
		return FillResult{}, nil, types.ErrZeroAmount.Wrapf("fill of order %s rounds to zero notional", maker.ID.Dec())
	}

	buyer, seller := taker.Account, maker.Account
	if !taker.IsLong {
		buyer, seller = maker.Account, taker.Account
	}
	buyerOpens, err := k.openingBase(ctx, buyer, marketID, true, size)
	if err != nil {
		return FillResult{}, nil, err
	}
	sellerOpens, err := k.openingBase(ctx, seller, marketID, false, size)
	if err != nil {
		return FillResult{}, nil, err
	}
	oiEvent, err := k.CheckOpenInterest(ctx, OpenInterestFill{
		MarketID: marketID, SizeBase: size, PriceUsd: price, BuyerOpensBase: buyerOpens, SellerOpensBase: sellerOpens,
	})
	if err != nil {
		return FillResult{}, nil, err
	}

	makerApplied, err := k.ApplyFill(ctx, PositionFill{
		Account: maker.Account, MarketID: marketID, IsLong: maker.IsLong, SizeBase: size, Price: price,
		Leverage: maker.Leverage, Kind: FillKindTrade, Fee: FeeRoleMaker,
	})
	if err != nil {
		return FillResult{}, nil, err
	}
	takerApplied, err := k.ApplyFill(ctx, PositionFill{
		Account: taker.Account, MarketID: marketID, IsLong: taker.IsLong, SizeBase: size, Price: price,
		Leverage: taker.Leverage, Kind: kind, Fee: FeeRoleTaker,
	})
	if err != nil {
		return FillResult{}, nil, err
	}
	makerPosition, takerPosition := makerApplied.PositionID, takerApplied.PositionID
	makerFee, takerFee := makerApplied.Fee, takerApplied.Fee
	makerFeeUsd := makerFee.FeeUsd.ToBig()
	makerFeeTokens := makerFee.Tokens.ToBig()
	if makerFee.FeeType == FeeTypeMakerRebate {
		makerFeeUsd.Neg(makerFeeUsd)
		makerFeeTokens.Neg(makerFeeTokens)
	}

	firstFill := maker.FilledBase.IsZero()
	maker.FilledBase.Add(&maker.FilledBase, &size)
	if maker.FilledBase.Eq(&maker.SizeBase) {
		maker.Status = types.OrderStatusFilled
		if err := k.BookRemove(ctx, maker); err != nil {
			return FillResult{}, nil, err
		}
		if err := k.closeOrder(ctx, maker, types.OrderStatusFilled); err != nil {
			return FillResult{}, nil, err
		}
	} else {
		k.setOrder(ctx, maker)
	}
	makerOpened := buyerOpens
	if !maker.IsLong {
		makerOpened = sellerOpens
	}
	if !makerOpened.IsZero() {
		market, err := k.GetMarket(ctx, marketID)
		if err != nil {
			return FillResult{}, nil, err
		}
		if err := k.checkFilledMargin(ctx, market, maker.Account); err != nil {
			return FillResult{}, nil, err
		}
	}
	reconciled, err := k.ReconcileReduceOnly(ctx, maker.Account, marketID)
	if err != nil {
		return FillResult{}, nil, err
	}

	fill := FillResult{
		MarketID:        marketID,
		MakerOrderID:    maker.ID,
		TakerOrderID:    taker.ID,
		Maker:           maker.Account,
		Taker:           taker.Account,
		TakerIsLong:     taker.IsLong,
		Price:           price,
		SizeBase:        size,
		SizeUsd:         *notional,
		MakerFeeUsd:     makerFeeUsd,
		TakerFeeUsd:     takerFee.FeeUsd,
		MakerPositionID: makerPosition,
		TakerPositionID: takerPosition,
	}
	events := []types.EventRecord{fillEvent(fill)}
	if oiEvent != nil {
		events = append(events, *oiEvent)
	}
	events = append(events, makerApplied.Events...)
	events = append(events, takerApplied.Events...)
	events = append(events, makerFee.Events...)
	events = append(events, takerFee.Events...)
	events = append(events, makerApplied.TradeSettledEvents(makerFeeUsd, makerFeeTokens)...)
	events = append(events, takerApplied.TradeSettledEvents(takerFee.FeeUsd.ToBig(), takerFee.Tokens.ToBig())...)
	events = append(events, reconciled...)
	events = append(events, orderExecuted(maker.ID, makerPosition, price))
	if firstFill {
		attached, err := k.attachOrderTpSl(ctx, maker)
		if err != nil {
			return FillResult{}, nil, err
		}
		events = append(events, attached...)
	}
	return fill, events, nil
}

// openingBase is the part of a fill of size on the given side that opens or
// increases the account's position: all of it without a position or with one
// on the same side, the excess over the opposite position otherwise.
func (k *Keeper) openingBase(ctx sdk.Context, account common.Address, marketID uint64, isLong bool, size uint256.Int) (uint256.Int, error) {
	store := ctx.KVStore(k.storeKey)
	idBytes := store.Get(types.AccountMarketPositionKey(account, marketID))
	if idBytes == nil {
		return size, nil
	}
	if len(idBytes) != 32 {
		return uint256.Int{}, types.ErrInvalidParams.Wrapf("position index of %s in market %d holds %d bytes", account, marketID, len(idBytes))
	}
	var positionID uint256.Int
	positionID.SetBytes32(idBytes)
	bz := store.Get(types.PositionKey(positionID))
	if bz == nil {
		return uint256.Int{}, types.ErrInvalidParams.Wrapf("position %s of %s is indexed but not stored", positionID.Dec(), account)
	}
	var position types.Position
	if err := position.Unmarshal(bz); err != nil {
		return uint256.Int{}, types.ErrInvalidParams.Wrapf("corrupt position %s: %v", positionID.Dec(), err)
	}
	if position.Account != account || position.MarketID != marketID {
		return uint256.Int{}, types.ErrInvalidParams.Wrapf("position %s is not the position of %s in market %d", positionID.Dec(), account, marketID)
	}
	if position.IsLong == isLong {
		return size, nil
	}
	if !size.Gt(&position.SizeBase) {
		return uint256.Int{}, nil
	}
	var opens uint256.Int
	opens.Sub(&size, &position.SizeBase)
	return opens, nil
}

// cancelResting takes a resting order out of the book as cancelled, the step
// of self-trade prevention.
func (k *Keeper) cancelResting(ctx sdk.Context, order types.Order) (types.EventRecord, error) {
	order.Status = types.OrderStatusCancelled
	if err := k.BookRemove(ctx, order); err != nil {
		return types.EventRecord{}, err
	}
	if err := k.closeOrder(ctx, order, types.OrderStatusCancelled); err != nil {
		return types.EventRecord{}, err
	}
	return orderCancelled(order), nil
}

// recordFillVolume books every fill notional to the 14-day buckets once the
// order's fees are charged, so the fills of an order count toward the tier of
// the next order.
func (k *Keeper) recordFillVolume(ctx sdk.Context, fills []FillResult) error {
	for _, fill := range fills {
		if err := k.RecordVolume(ctx, fill.Taker, fill.Maker, fill.SizeUsd); err != nil {
			return err
		}
	}
	return nil
}

// setLastTradePrice stores the price of the last fill of the market, the last
// component of the mark's book median.
func (k *Keeper) setLastTradePrice(ctx sdk.Context, marketID uint64, price uint256.Int) error {
	state, err := k.GetMarketState(ctx, marketID)
	if err != nil {
		return err
	}
	state.LastTradePrice = price
	k.SetMarketState(ctx, state)
	return nil
}

func fillEvent(fill FillResult) types.EventRecord {
	return types.EventRecord{
		Name: types.EventFill,
		Args: []any{
			new(big.Int).SetUint64(fill.MarketID), fill.MakerOrderID.ToBig(), fill.TakerOrderID.ToBig(),
			fill.Maker, fill.Taker, fill.TakerIsLong, fill.Price.ToBig(), fill.SizeUsd.ToBig(),
			new(big.Int).Set(fill.MakerFeeUsd), fill.TakerFeeUsd.ToBig(),
		},
	}
}

func orderCancelled(order types.Order) types.EventRecord {
	return types.EventRecord{Name: types.EventOrderCancelled, Args: []any{order.ID.ToBig(), order.Account}}
}

func orderExecuted(orderID, positionID, executionPrice uint256.Int) types.EventRecord {
	return types.EventRecord{
		Name: types.EventOrderExecuted,
		Args: []any{orderID.ToBig(), positionID.ToBig(), executionPrice.ToBig()},
	}
}

func (k *Keeper) checkFilledMargin(ctx sdk.Context, market types.Market, account common.Address) error {
	summary, err := k.AccountMargin(ctx, account)
	if err != nil {
		return err
	}
	if summary.Equity.LT(summary.InitialMargin) {
		return types.ErrInsufficientMargin.Wrapf("post-fee equity %s below resulting initial margin %s", summary.Equity, summary.InitialMargin)
	}
	position, exists, err := k.orderPosition(ctx, account, market.ID)
	if err != nil || !exists {
		return err
	}
	state, err := k.GetMarketState(ctx, market.ID)
	if err != nil {
		return err
	}
	notional, overflow := new(uint256.Int).MulDivOverflow(&position.SizeBase, &state.MarkPrice, matchUnit)
	if overflow {
		return types.ErrOverflow.Wrap("post-fill position notional")
	}
	return checkOrderLeverage(market, types.Order{SizeUsd: *notional, Leverage: position.Leverage})
}
