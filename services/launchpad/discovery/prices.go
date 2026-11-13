package discovery

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

// MetricFromMarketPrice accepts only a committed, complete, pool-bound price.
// Asset identities come from the price authority, never from the saved rule.
func MetricFromMarketPrice(pool string, raw []byte) (Metric, error) {
	if !address.MatchString(pool) || len(raw) > 256*1024 || v2.ValidateJSON("MarketPriceEvent", raw) != nil {
		return Metric{}, ErrInvalid
	}
	var frame v2.MarketPriceEvent
	if json.Unmarshal(raw, &frame) != nil {
		return Metric{}, ErrInvalid
	}
	if frame.Topic != "market.price."+strings.ToLower(pool) || frame.Coalesced || frame.SourceSequenceFirst != nil || frame.SourceSequenceLast != nil || frame.Watermark == nil || frame.Watermark.Integrity != "healthy" || string(frame.Revision) != string(frame.Watermark.ProjectionRevision) {
		return Metric{}, ErrInvalid
	}
	payload, e := json.Marshal(frame.Payload)
	if e != nil {
		return Metric{}, e
	}
	var price v2.InputPrice
	if json.Unmarshal(payload, &price) != nil {
		return Metric{}, ErrInvalid
	}
	if price.Source.Kind != "gross_execution" || price.Source.AmountBasis != "paid_input" || price.Source.Position == nil || price.Source.Position.ChainId != "125" || price.Source.Position.Height != frame.Watermark.Height || price.Source.Position.BlockHash != frame.Watermark.BlockHash || price.Source.ObservedAt != frame.OccurredAt {
		return Metric{}, ErrInvalid
	}
	revision, e := strconv.ParseUint(string(frame.Revision), 10, 64)
	if e != nil || revision == 0 {
		return Metric{}, ErrInvalid
	}
	observed, e := time.Parse(time.RFC3339Nano, price.Source.ObservedAt)
	if e != nil {
		return Metric{}, ErrInvalid
	}
	value := string(price.Numerator) + "/" + string(price.Denominator)
	if validatePrice(price, value) != nil {
		return Metric{}, ErrInvalid
	}
	return Metric{PoolAddress: strings.ToLower(pool), Price: &price, Value: value, Revision: revision, ObservedAt: observed, Quality: qualityComplete}, nil
}

// CanonicalPrice reads the existing owner API. No foreign table or derived
// decimal/display value is treated as a pricing authority.
func (s Store) CanonicalPrice(ctx context.Context, pool string) (Metric, error) {
	if s.DB == nil {
		return Metric{}, ErrUnavailable
	}
	return canonicalPrice(ctx, s.DB, pool)
}

type priceReader interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func canonicalPrice(ctx context.Context, reader priceReader, pool string) (Metric, error) {
	if !address.MatchString(pool) {
		return Metric{}, ErrInvalid
	}
	var raw []byte
	if e := reader.QueryRowContext(ctx, `SELECT api_stream.market_projection_head($1)`, "market.price."+strings.ToLower(pool)).Scan(&raw); e != nil {
		return Metric{}, e
	}
	if len(raw) == 0 {
		return Metric{}, ErrUnavailable
	}
	return MetricFromMarketPrice(pool, raw)
}
