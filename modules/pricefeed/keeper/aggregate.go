package keeper

import (
	"bytes"
	"sort"
	"strconv"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

// candidate is the stored observation of one bonded validator for a market.
type candidate struct {
	operator sdk.ValAddress
	power    sdk.Int
	record   types.ObservationRecord
}

type bondedOperator struct {
	operator sdk.ValAddress
	power    sdk.Int
}

// Aggregate updates the aggregate of every enabled market in id order from
// the fresh observations of the bonded set. It reads only the store and the
// block time, so the order of the block's transactions does not change it.
// It does nothing while the store holds no params.
func (k Keeper) Aggregate(ctx sdk.Context) {
	var params types.Params
	if !k.get(ctx, types.ParamsKey, &params) {
		return
	}
	bonded := k.bondedOperators(ctx)
	totalPower := sdk.ZeroInt()
	for _, operator := range bonded {
		totalPower = totalPower.Add(operator.power)
	}
	nowMs := ctx.BlockTime().UnixMilli()
	for _, market := range params.Markets {
		if !market.Enabled {
			continue
		}
		var candidates []candidate
		for _, operator := range bonded {
			if record, found := k.GetObservation(ctx, market.ID, operator.operator); found &&
				withinStaleness(nowMs, record.SourceTimestampMs, params.StalenessWindowMs) {
				candidates = append(candidates, candidate{operator: operator.operator, power: operator.power, record: record})
			}
		}
		k.aggregateMarket(ctx, market.ID, candidates, totalPower, params)
	}
}

// bondedOperators walks the power store up to MaxValidators bonded
// validators, the claim map walk of the oracle vote module.
func (k Keeper) bondedOperators(ctx sdk.Context) []bondedOperator {
	maxValidators := k.stakingKeeper.MaxValidators(ctx)
	iterator := k.stakingKeeper.ValidatorsPowerStoreIterator(ctx)
	defer func() { _ = iterator.Close() }()

	var operators []bondedOperator
	for ; iterator.Valid() && uint32(len(operators)) < maxValidators; iterator.Next() {
		validator := k.stakingKeeper.Validator(ctx, iterator.Value())
		if validator != nil && validator.IsBonded() {
			operators = append(operators, bondedOperator{
				operator: validator.GetOperator(),
				power:    sdk.NewInt(validator.GetConsensusPower(sdk.DefaultPowerReduction)),
			})
		}
	}
	return operators
}

// required enforces the bonded-power quorum: reporting power must exceed
// two thirds of total bonded power, and the source count must reach MinSources.
func required(candidates []candidate, minSources uint32, totalPower sdk.Int) bool {
	if uint32(len(candidates)) < minSources {
		return false
	}
	reportingPower := sdk.ZeroInt()
	for _, c := range candidates {
		reportingPower = reportingPower.Add(c.power)
	}
	return reportingPower.MulRaw(3).GT(totalPower.MulRaw(2))
}

// withinStaleness reports block_time_ms - source_timestamp_ms <= window; a
// source timestamp at or after block time has no age.
func withinStaleness(nowMs int64, sourceMs, windowMs uint64) bool {
	if nowMs < 0 || sourceMs >= uint64(nowMs) {
		return true
	}
	return uint64(nowMs)-sourceMs <= windowMs
}

func (k Keeper) aggregateMarket(ctx sdk.Context, marketID uint64, candidates []candidate, totalPower sdk.Int, params types.Params) {
	need := params.MinSources
	if !required(candidates, need, totalPower) {
		emitSkipped(ctx, marketID, len(candidates), need)
		return
	}
	first := median(candidates, indexPrice)
	limit := first.Mul(sdk.NewIntFromUint64(uint64(params.MaxDivergenceBps)))
	bps := sdk.NewIntFromUint64(uint64(types.BpsDenominator))
	agreeing := make([]candidate, 0, len(candidates))
	for _, c := range candidates {
		if c.record.IndexPrice.Sub(first).Abs().Mul(bps).LTE(limit) {
			agreeing = append(agreeing, c)
		}
	}
	if !required(agreeing, need, totalPower) {
		emitSkipped(ctx, marketID, len(agreeing), need)
		return
	}

	index := median(agreeing, indexPrice)
	timestamp := agreeing[0].record.SourceTimestampMs
	for _, c := range agreeing[1:] {
		if c.record.SourceTimestampMs < timestamp {
			timestamp = c.record.SourceTimestampMs
		}
	}
	var prev *types.Aggregate
	var stored types.Aggregate
	if k.get(ctx, types.AggregateKey(marketID), &stored) {
		prev = &stored
	}
	mark, basis := markPrice(prev, index, median(agreeing, markPriceOf), timestamp, params)
	aggregate := types.Aggregate{
		IndexPrice:  index,
		MarkPrice:   mark,
		Basis:       basis,
		TimestampMs: timestamp,
		Height:      ctx.BlockHeight(),
		Sources:     uint32(len(agreeing)),
	}
	k.set(ctx, types.AggregateKey(marketID), aggregate)
	ctx.EventManager().EmitEvent(sdk.NewEvent(types.EventTypeAggregate,
		sdk.NewAttribute(types.AttributeMarketID, strconv.FormatUint(marketID, 10)),
		sdk.NewAttribute(types.AttributeIndexPrice, aggregate.IndexPrice.String()),
		sdk.NewAttribute(types.AttributeMarkPrice, aggregate.MarkPrice.String()),
		sdk.NewAttribute(types.AttributeBasis, aggregate.Basis.String()),
		sdk.NewAttribute(types.AttributeTimestampMs, strconv.FormatUint(aggregate.TimestampMs, 10)),
		sdk.NewAttribute(types.AttributeSources, strconv.FormatUint(uint64(aggregate.Sources), 10)),
		sdk.NewAttribute(types.AttributeRequired, strconv.FormatUint(uint64(need), 10)),
		sdk.NewAttribute(types.AttributeHeight, strconv.FormatInt(aggregate.Height, 10)),
	))
}

func emitSkipped(ctx sdk.Context, marketID uint64, sources int, need uint32) {
	ctx.EventManager().EmitEvent(sdk.NewEvent(types.EventTypeAggregateSkipped,
		sdk.NewAttribute(types.AttributeMarketID, strconv.FormatUint(marketID, 10)),
		sdk.NewAttribute(types.AttributeSources, strconv.Itoa(sources)),
		sdk.NewAttribute(types.AttributeRequired, strconv.FormatUint(uint64(need), 10)),
	))
}

func indexPrice(c candidate) sdk.Int  { return c.record.IndexPrice }
func markPriceOf(c candidate) sdk.Int { return c.record.MarkPrice }

// median sorts a copy of the non-empty candidates by price, then by operator
// bytes, and returns the middle price; an even count takes the floor
// midpoint of the middle pair.
func median(candidates []candidate, price func(candidate) sdk.Int) sdk.Int {
	sorted := append([]candidate(nil), candidates...)
	sort.Slice(sorted, func(i, j int) bool {
		a, b := price(sorted[i]), price(sorted[j])
		if !a.Equal(b) {
			return a.LT(b)
		}
		return bytes.Compare(sorted[i].operator, sorted[j].operator) < 0
	})
	n := len(sorted)
	if n%2 == 1 {
		return price(sorted[n/2])
	}
	return price(sorted[n/2-1]).Add(price(sorted[n/2])).QuoRaw(2)
}
