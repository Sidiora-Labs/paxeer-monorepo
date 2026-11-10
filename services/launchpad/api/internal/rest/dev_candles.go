package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/development"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain/profile"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// projectCandles consumes only the chart builder's durable state, never legacy
// candle rows alone. A source revision is acknowledged in the same transaction
// that appends its history document and stream frame.
func (d *DevelopmentStream) projectCandles(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var installed bool
	if err := d.pool.QueryRow(ctx, `SELECT to_regclass('candles.development_state') IS NOT NULL`).Scan(&installed); err != nil {
		return err
	}
	if !installed {
		return nil
	}
	rows, err := d.pool.Query(ctx, `SELECT s.key,s.payload FROM candles.development_state s
        LEFT JOIN `+streamTable("candle_projection")+` p
        ON p.pool_address=convert_from(s.payload,'UTF8')::jsonb->'Row'->>'PoolAddress' AND p.timeframe=convert_from(s.payload,'UTF8')::jsonb->'Row'->>'Timeframe'
        AND p.candle_start=(convert_from(s.payload,'UTF8')::jsonb->'Row'->>'CandleStart')::bigint
        WHERE starts_with(s.key,'candle/') AND (p.source_revision IS NULL OR p.source_revision < (convert_from(s.payload,'UTF8')::jsonb->'Row'->>'TradeCount')::bigint)
        ORDER BY s.key LIMIT $1`, developmentStreamBatch)
	if err != nil {
		return err
	}
	type committed struct {
		key   string
		raw   []byte
		state development.Candle
	}
	var batch []committed
	for rows.Next() {
		var c committed
		if err = rows.Scan(&c.key, &c.raw); err != nil {
			rows.Close()
			return err
		}
		if err = json.Unmarshal(c.raw, &c.state); err != nil {
			rows.Close()
			return err
		}
		batch = append(batch, c)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, c := range batch {
		doc, wm, err := d.candleDocument(ctx, c.state)
		if err != nil {
			return fmt.Errorf("development candle %s: %w", c.key, err)
		}
		tx, locked, err := d.beginProjection(ctx)
		if err != nil {
			return err
		}
		if !locked {
			return nil
		}
		err = d.appendCandle(ctx, tx, c.key, c.raw, c.state, doc, wm)
		if err == nil {
			err = tx.Commit(ctx)
		}
		_ = tx.Rollback(context.WithoutCancel(ctx))
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *DevelopmentStream) candleDocument(ctx context.Context, c development.Candle) (map[string]any, store.ChainWatermark, error) {
	fail := func() (map[string]any, store.ChainWatermark, error) {
		return nil, store.ChainWatermark{}, store.ErrDataIntegrity
	}
	r := c.Row
	if r.TradeCount <= 0 || c.OpenSource == nil || c.CloseSource == nil || c.HighSource == nil || c.LowSource == nil || c.OpenSource.Position == nil || c.CloseSource.Position == nil || *c.OpenSource.Position != c.First || *c.CloseSource.Position != c.Last {
		return fail()
	}
	var token, optical string
	if err := d.pool.QueryRow(ctx, `SELECT lower(token_address),lower(optical) FROM indexer.pools WHERE pool_address=$1`, r.PoolAddress).Scan(&token, &optical); err != nil {
		return nil, store.ChainWatermark{}, err
	}
	refs := []*development.Swap{c.OpenSource, c.HighSource, c.LowSource, c.CloseSource}
	rawPrices := []string{r.Open, r.High, r.Low, r.Close}
	swaps := make([]developmentSwap, 0, 4)
	for i, s := range refs {
		if s.Position == nil || s.PoolAddress != r.PoolAddress || s.Price != rawPrices[i] || s.BlockTimestamp < r.CandleStart {
			return fail()
		}
		swaps = append(swaps, developmentSwap{id: s.TxHash + ":" + strconv.Itoa(s.LogIndex), poolID: s.PoolID, pool: s.PoolAddress, sender: s.Sender, isBuy: s.IsBuy, amountIn: s.AmountIn, amountOut: s.AmountOut, fee: s.Fee, price: s.Price, txHash: s.TxHash, block: int64(s.Position.Height), timestamp: s.BlockTimestamp, logIndex: int32(s.LogIndex), token: token, optical: optical})
	}
	completed, err := d.complete(ctx, swaps)
	if err != nil {
		return nil, store.ChainWatermark{}, err
	}
	if len(completed) != 4 {
		return fail()
	}
	prices := make([]map[string]any, 4)
	for i, p := range completed {
		s := refs[i]
		if p.position.Height != strconv.FormatUint(s.Position.Height, 10) || p.position.TransactionIndex != strconv.FormatUint(s.Position.TransactionIndex, 10) || p.position.LogIndex != strconv.FormatUint(s.Position.LogIndex, 10) {
			return fail()
		}
		value, err := store.ParseUInt256(s.Price)
		if err != nil {
			return fail()
		}
		num, _ := new(big.Int).SetString(string(value), 10)
		ratio := new(big.Rat).SetFrac(new(big.Int).Mul(num, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(p.tokenDecimals)), nil)), new(big.Int).Exp(big.NewInt(10), big.NewInt(24), nil))
		asset := func(address string, decimals int) map[string]any {
			return map[string]any{"kind": "erc20", "chainId": "125", "address": address, "currencyCode": nil, "decimals": decimals}
		}
		prices[i] = map[string]any{"numerator": ratio.Num().String(), "denominator": ratio.Denom().String(), "baseAsset": asset(token, p.tokenDecimals), "quoteAsset": asset(strings.ToLower(profile.USDLAddress), 6), "source": map[string]any{
			"kind": "post_swap_spot", "amountBasis": "effective_reserves", "sourceEventId": "125:" + p.position.TransactionHash + ":" + p.position.LogIndex, "position": chainPosition(p.position), "providerId": nil, "observedAt": wireTime(time.Unix(s.BlockTimestamp, 0)), "sourceReference": nil,
		}}
	}
	revision := strconv.Itoa(r.TradeCount)
	identity, err := json.Marshal(c)
	if err != nil {
		return nil, store.ChainWatermark{}, err
	}
	doc := map[string]any{
		"poolAddress": r.PoolAddress, "interval": r.Timeframe, "start": wireTime(time.Unix(r.CandleStart, 0)),
		"open": prices[0], "high": prices[1], "low": prices[2], "close": prices[3],
		"volumeUsdlRaw": r.VolumeUsdl, "volumeTokenRaw": r.VolumeToken, "buyVolumeUsdlRaw": r.BuyVolumeUsdl, "sellVolumeUsdlRaw": r.SellVolumeUsdl,
		"trades": revision, "revision": revision, "synthetic": false, "coverage": "partial",
		"firstPosition": chainPosition(completed[0].position), "lastPosition": chainPosition(completed[3].position),
		"seriesModelVersion": DevelopmentSeriesModelVersion, "snapshotId": uuid.NewSHA1(uuid.NameSpaceOID, identity).String(),
		"uniqueTraderCount": strconv.Itoa(r.UniqueTraders), "largeTradeCount": strconv.Itoa(r.LargeTradeCount), "largeTradePolicyId": "development-usdl-500-v1", "largeTradeThresholdUsdlRaw": "500000000",
	}
	raw, err := json.Marshal(doc)
	if err != nil || interfaces.ValidateJSON("Candle", raw) != nil {
		return fail()
	}
	last := completed[3].position
	wm := store.ChainWatermark{ChainID: "125", Height: last.Height, BlockHash: last.BlockHash, ProjectionRevision: revision, ObservedAt: wireTime(time.Now()), Integrity: "healthy"}
	return doc, wm, nil
}

func (d *DevelopmentStream) appendCandle(ctx context.Context, tx pgx.Tx, key string, expected []byte, c development.Candle, doc map[string]any, wm store.ChainWatermark) error {
	// Recheck under a row lock so a newer source commit cannot be acknowledged
	// using an older receipt completion.
	var current []byte
	if err := tx.QueryRow(ctx, `SELECT payload FROM candles.development_state WHERE key=$1 FOR SHARE`, key).Scan(&current); err != nil {
		return err
	}
	if !bytes.Equal(current, expected) {
		return nil
	}
	r := c.Row
	var revision int64
	err := tx.QueryRow(ctx, `SELECT source_revision FROM `+streamTable("candle_projection")+` WHERE pool_address=$1 AND timeframe=$2 AND candle_start=$3`, r.PoolAddress, r.Timeframe, r.CandleStart).Scan(&revision)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if revision >= int64(r.TradeCount) {
		return nil
	}
	topic := "market.candles." + r.PoolAddress + "." + r.Timeframe
	head, err := readTopicHead(ctx, tx, topic)
	if err != nil {
		return err
	}
	id := "development-candle:" + r.PoolAddress + ":" + r.Timeframe + ":" + strconv.FormatInt(r.CandleStart, 10) + ":" + strconv.Itoa(r.TradeCount)
	if err = d.appendFrame(ctx, tx, head, id, topic, strconv.Itoa(r.TradeCount), wireTime(time.Unix(r.LastTradeTs, 0)), wm, doc); err != nil {
		return err
	}
	payload, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	watermark, err := json.Marshal(wm)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO `+streamTable("candle_projection")+`(pool_address,timeframe,candle_start,source_revision,topic_sequence,payload,watermark) VALUES($1,$2,$3,$4,$5,$6,$7)
        ON CONFLICT(pool_address,timeframe,candle_start) DO UPDATE SET source_revision=EXCLUDED.source_revision,topic_sequence=EXCLUDED.topic_sequence,payload=EXCLUDED.payload,watermark=EXCLUDED.watermark`, r.PoolAddress, r.Timeframe, r.CandleStart, r.TradeCount, head.sequence+1, payload, watermark)
	return err
}
