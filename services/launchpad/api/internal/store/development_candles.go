package store

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
)

// DevelopmentCandlePage reads the same committed projection the development
// candle topic uses. A populated scope is never served partly from legacy rows.
func (s *Store) DevelopmentCandlePage(ctx context.Context, r AccountingPageRequest) (*AccountingPage, error) {
	ctx, cancel, tx, err := s.beginLegacyRead(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()
	defer rollbackRead(tx)
	var installed bool
	if err = tx.QueryRow(ctx, `SELECT to_regclass('api_stream_dev.candle_projection') IS NOT NULL`).Scan(&installed); err != nil {
		return nil, err
	}
	if !installed {
		return nil, ErrDataIntegrity
	}
	var count int64
	var coherent bool
	var sequence int64
	var fingerprint string
	err = tx.QueryRow(ctx, `SELECT count(*),COALESCE(bool_and(p.source_revision IS NOT NULL AND p.source_revision=c.trade_count),true),COALESCE(max(p.topic_sequence),0),COALESCE(md5(string_agg(c.candle_start::text||':'||c.trade_count::text||':'||COALESCE(md5(p.payload),''),',' ORDER BY c.candle_start)),'')
        FROM candles.candles c LEFT JOIN api_stream_dev.candle_projection p USING(pool_address,timeframe,candle_start)
        WHERE c.pool_address=$1 AND c.timeframe=$2 AND c.candle_start*1000 >= $3 AND c.candle_start*1000 < $4`, r.Pool, r.Interval, r.From, r.To).Scan(&count, &coherent, &sequence, &fingerprint)
	if err != nil {
		return nil, err
	}
	if !coherent {
		// The chart writer commits before the stream projector acknowledges
		// that source revision. A real durable source is a moving snapshot;
		// legacy candles without that source still fail closed.
		var sourceInstalled bool
		if err = tx.QueryRow(ctx, `SELECT to_regclass('candles.development_state') IS NOT NULL`).Scan(&sourceInstalled); err != nil {
			return nil, err
		}
		if !sourceInstalled {
			return nil, ErrDataIntegrity
		}
		var legacy bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM candles.candles c
 LEFT JOIN api_stream_dev.candle_projection p USING(pool_address,timeframe,candle_start)
 WHERE c.pool_address=$1 AND c.timeframe=$2 AND c.candle_start*1000 >= $3 AND c.candle_start*1000 < $4
 AND (p.source_revision IS NULL OR p.source_revision<>c.trade_count)
 AND NOT EXISTS (SELECT 1 FROM candles.development_state s
 WHERE s.key='candle/["'||c.pool_address||'","'||c.timeframe||'","'||c.candle_start::text||'"]'
 AND (convert_from(s.payload,'UTF8')::jsonb->'Row'->>'TradeCount')::bigint=c.trade_count))`, r.Pool, r.Interval, r.From, r.To).Scan(&legacy)
		if err != nil {
			return nil, err
		}
		if legacy {
			return nil, ErrDataIntegrity
		}
		return nil, ErrDevelopmentCandleSnapshotChanged
	}
	if count == 0 {
		return nil, ErrDevelopmentCandleSnapshotChanged
	}
	snapshot := uuid.NewSHA1(uuid.NameSpaceOID, []byte(r.Pool+"|"+r.Interval+"|"+strconv.FormatInt(r.From, 10)+"|"+strconv.FormatInt(r.To, 10)+"|"+fingerprint)).String()
	if r.ExpectedSnapshot != "" && r.ExpectedSnapshot != snapshot {
		return nil, ErrDevelopmentCandleSnapshotChanged
	}
	page := &AccountingPage{SnapshotID: snapshot, Items: []json.RawMessage{}, Revision: strconv.FormatInt(sequence, 10)}
	rows, err := tx.Query(ctx, `SELECT candle_start,payload FROM api_stream_dev.candle_projection WHERE pool_address=$1 AND timeframe=$2 AND candle_start*1000 >= $3 AND candle_start*1000 < $4 AND ($5::bigint IS NULL OR candle_start>$5) ORDER BY candle_start LIMIT $6`, r.Pool, r.Interval, r.From, r.To, r.AfterStart, r.Limit+1)
	if err != nil {
		return nil, err
	}
	// Drain the bounded SQL result and watermark and release the database
	// transaction before validating or rewriting contract JSON. Validation
	// must not spend the read transaction's deadline before its final query.
	var last int64
	var payloads [][]byte
	for rows.Next() {
		var start int64
		var payload []byte
		if err = rows.Scan(&start, &payload); err != nil {
			rows.Close()
			return nil, err
		}
		if len(payloads) == r.Limit {
			page.NextStart = &last
			break
		}
		payloads = append(payloads, payload)
		last = start
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	var watermark []byte
	if err = tx.QueryRow(ctx, `SELECT watermark FROM api_stream_dev.candle_projection WHERE pool_address=$1 AND timeframe=$2 AND candle_start*1000 >= $3 AND candle_start*1000 < $4 ORDER BY topic_sequence DESC LIMIT 1`, r.Pool, r.Interval, r.From, r.To).Scan(&watermark); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	for _, payload := range payloads {
		if interfaces.ValidateJSON("Candle", payload) != nil {
			return nil, ErrDataIntegrity
		}
		var doc struct {
			SeriesModelVersion string `json:"seriesModelVersion"`
		}
		if err = json.Unmarshal(payload, &doc); err != nil {
			return nil, err
		}
		if page.SeriesModelVersion != "" && page.SeriesModelVersion != doc.SeriesModelVersion {
			return nil, ErrDataIntegrity
		}
		page.SeriesModelVersion = doc.SeriesModelVersion
		var item map[string]json.RawMessage
		if err = json.Unmarshal(payload, &item); err != nil {
			return nil, err
		}
		item["snapshotId"] = json.RawMessage(strconv.Quote(snapshot))
		payload, err = json.Marshal(item)
		if err != nil {
			return nil, err
		}
		if interfaces.ValidateJSON("Candle", payload) != nil {
			return nil, ErrDataIntegrity
		}
		page.Items = append(page.Items, json.RawMessage(payload))
	}

	var wm ChainWatermark
	if err = json.Unmarshal(watermark, &wm); err != nil {
		return nil, err
	}
	wm.ProjectionRevision = page.Revision
	page.Watermark, err = json.Marshal(wm)
	if err != nil {
		return nil, err
	}
	return page, nil
}

var ErrDevelopmentCandleSnapshotChanged = errors.New("development candle snapshot changed")
