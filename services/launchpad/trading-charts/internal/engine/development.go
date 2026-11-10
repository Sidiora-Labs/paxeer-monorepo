package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/constants"
	shareddb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
)

// developmentStore owns the transaction boundary, including the first insertion.
// Implementations serialize the source identity and pool until commit or rollback.
type developmentStore interface {
	Transaction(context.Context, string, string, func(developmentKV) error) error
}
type developmentKV interface {
	Get(string) ([]byte, error)
	Put(string, []byte) error
	Claim(SwapEvent) (bool, error)
}

type DevelopmentCandle struct {
	OpenSource, HighSource, LowSource, CloseSource *SwapEvent
	Row                                            store.CandleRow
	First                                          ChainOrder
	Last                                           ChainOrder
	Traders                                        map[string]bool
}

type developmentCandle = DevelopmentCandle

type developmentDelivery struct {
	ID    string            `json:"id"`
	Event CandleUpdateEvent `json:"event"`
}

func developmentKey(kind string, parts ...string) string {
	wire, _ := json.Marshal(parts)
	return kind + "/" + string(wire)
}
func loadDevelopment(tx developmentKV, key string, target any) (bool, error) {
	raw, err := tx.Get(key)
	if err != nil || len(raw) == 0 {
		return false, err
	}
	if err = json.Unmarshal(raw, target); err != nil {
		return false, err
	}
	return true, nil
}
func saveDevelopment(tx developmentKV, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return tx.Put(key, raw)
}

func (b *Builder) foldSwap(tx developmentKV, swap SwapEvent) error {
	claimed, err := tx.Claim(swap)
	if err != nil || !claimed {
		return err
	}
	volume, tokens := swap.AmountIn, swap.AmountOut
	if !swap.IsBuy {
		volume, tokens = swap.AmountOut, swap.AmountIn
	}
	for _, value := range []string{swap.Price, volume, tokens} {
		if _, err := RawUint256(value); err != nil {
			return err
		}
	}
	mcap, err := shareddb.ComputeMarketCap(swap.Price)
	if err != nil {
		return err
	}
	var pending []developmentDelivery
	outboxKey := developmentKey("outbox", swap.PoolAddress)
	if _, err = loadDevelopment(tx, outboxKey, &pending); err != nil {
		return err
	}
	for _, tf := range constants.TimeframeKeys {
		interval := constants.Timeframes[tf]
		if interval <= 0 {
			return ErrUnsupportedResolution
		}
		start := swap.BlockTimestamp / interval * interval
		key := developmentKey("candle", swap.PoolAddress, tf, strconv.FormatInt(start, 10))
		var candle developmentCandle
		found, err := loadDevelopment(tx, key, &candle)
		if err != nil {
			return err
		}
		cursorKey := developmentKey("cursor", swap.PoolAddress, tf)
		var cursor store.CursorRow
		hasCursor, err := loadDevelopment(tx, cursorKey, &cursor)
		if err != nil {
			return err
		}
		if !found {
			sequence := int64(0)
			if hasCursor {
				sequence = cursor.LastSequenceNum + 1
				if start < cursor.LastCandleStart {
					sequence = cursor.LastSequenceNum - (cursor.LastCandleStart-start)/interval
				}
			}
			candle = developmentCandle{Row: store.CandleRow{
				PoolAddress: swap.PoolAddress, Timeframe: tf, CandleStart: start,
				Open: swap.Price, High: swap.Price, Low: swap.Price, Close: swap.Price,
				VolumeUsdl: "0", VolumeToken: "0", BuyVolumeUsdl: "0", SellVolumeUsdl: "0",
				SequenceNum: sequence, McapOpen: mcap, McapHigh: mcap, McapLow: mcap, McapClose: mcap,
			}, OpenSource: &swap, HighSource: &swap, LowSource: &swap, CloseSource: &swap, First: *swap.Position, Last: *swap.Position, Traders: map[string]bool{}}
		}
		c := &candle.Row
		if candle.Traders == nil || c.TradeCount < 0 || c.TradeCount >= 2147483647 {
			return errors.New("engine: invalid development candle state")
		}
		if swap.Position.Compare(candle.First) < 0 {
			candle.OpenSource = &swap
			candle.First, c.Open, c.McapOpen = *swap.Position, swap.Price, mcap
		}
		if swap.Position.Compare(candle.Last) >= 0 {
			candle.CloseSource = &swap
			candle.Last, c.Close, c.McapClose, c.LastTradeTs = *swap.Position, swap.Price, mcap, swap.BlockTimestamp
		}
		previousHigh, previousLow := c.High, c.Low
		if c.High, err = shareddb.BigintMax(c.High, swap.Price); err != nil {
			return err
		}
		if c.Low, err = shareddb.BigintMin(c.Low, swap.Price); err != nil {
			return err
		}
		if c.High != previousHigh {
			candle.HighSource = &swap
		}
		if c.Low != previousLow {
			candle.LowSource = &swap
		}
		if c.McapHigh, err = shareddb.BigintMax(c.McapHigh, mcap); err != nil {
			return err
		}
		if c.McapLow, err = shareddb.BigintMin(c.McapLow, mcap); err != nil {
			return err
		}
		if c.VolumeUsdl, err = boundedSum(c.VolumeUsdl, volume); err != nil {
			return err
		}
		if c.VolumeToken, err = boundedSum(c.VolumeToken, tokens); err != nil {
			return err
		}
		side := &c.SellVolumeUsdl
		if swap.IsBuy {
			side = &c.BuyVolumeUsdl
		}
		if *side, err = boundedSum(*side, volume); err != nil {
			return err
		}
		amount, _ := RawUint256(volume)
		if amount.Cmp(largeTradeThreshold) >= 0 {
			c.LargeTradeCount++
		}
		c.TradeCount++
		if swap.Sender != "" {
			candle.Traders[swap.Sender] = true
		}
		c.UniqueTraders = len(candle.Traders)
		if err = saveDevelopment(tx, key, candle); err != nil {
			return err
		}
		// Updating an old existing bucket cannot change the current cursor.
		if !hasCursor || start >= cursor.LastCandleStart {
			cursor = store.CursorRow{PoolAddress: swap.PoolAddress, Timeframe: tf, LastClose: c.Close, LastCandleStart: start, LastSequenceNum: c.SequenceNum}
			if err = saveDevelopment(tx, cursorKey, cursor); err != nil {
				return err
			}
		}
		identity, _ := json.Marshal([]any{swap.PoolAddress, tf, start, c.TradeCount})
		digest := sha256.Sum256(identity)
		event := CandleUpdateEvent{PoolAddress: swap.PoolAddress, Timeframe: tf, CandleStart: start,
			Open: c.Open, High: c.High, Low: c.Low, Close: c.Close,
			VolumeUsdl: c.VolumeUsdl, VolumeToken: c.VolumeToken, BuyVolumeUsdl: c.BuyVolumeUsdl, SellVolumeUsdl: c.SellVolumeUsdl,
			TradeCount: c.TradeCount, UniqueTraders: c.UniqueTraders, LargeTradeCount: c.LargeTradeCount,
			McapOpen: c.McapOpen, McapHigh: c.McapHigh, McapLow: c.McapLow, McapClose: c.McapClose,
			EventID: "development_" + hex.EncodeToString(digest[:]), Revision: c.TradeCount}
		pending = append(pending, developmentDelivery{ID: event.EventID, Event: event})
	}
	return saveDevelopment(tx, outboxKey, pending)
}

// Publishing and the Redis revision fence are atomic. Stable event IDs allow
// consumers to deduplicate an ambiguous acknowledgement even after Redis loss.
const developmentPublishScript = `
local old = redis.call('GET', KEYS[1])
if old and tonumber(old) >= tonumber(ARGV[1]) then return 0 end
redis.call('PUBLISH', ARGV[2], ARGV[3])
redis.call('SET', KEYS[1], ARGV[1])
return 1
`

func (b *Builder) RelayCommitted(ctx context.Context, pool string) error {
	// The transaction's pool lock serializes competing relays and producers.
	// Only a later transaction can see outbox records committed by foldSwap.
	return b.storage.Transaction(ctx, pool, "", func(tx developmentKV) error {
		key := developmentKey("outbox", pool)
		var pending []developmentDelivery
		if _, err := loadDevelopment(tx, key, &pending); err != nil {
			return err
		}
		for _, record := range pending {
			if b.redis == nil {
				return errors.New("engine: Redis delivery unavailable")
			}
			raw, err := json.Marshal(record.Event)
			if err != nil {
				return err
			}
			fence := developmentKey("candle:delivered", pool, record.Event.Timeframe, strconv.FormatInt(record.Event.CandleStart, 10))
			result, err := b.redis.Eval(ctx, developmentPublishScript, []string{fence}, record.Event.Revision, constants.ChannelCandleUpdate, raw).Int64()
			if err != nil {
				return fmt.Errorf("engine: committed delivery %s: %w", record.ID, err)
			}
			if result != 0 && result != 1 {
				return errors.New("engine: invalid Redis delivery acknowledgement")
			}
		}
		if len(pending) == 0 {
			return nil
		}
		return saveDevelopment(tx, key, []developmentDelivery{})
	})
}
