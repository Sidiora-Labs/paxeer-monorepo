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
	err = tx.QueryRow(ctx, `SELECT count(*),COALESCE(bool_and(p.source_revision IS NOT NULL AND p.source_revision=c.trade_count),true),COALESCE(max(p.topic_sequence),0),COALESCE(md5(string_agg(c.candle_start::text||':'||c.trade_count::text,',' ORDER BY c.candle_start)),'')
        FROM candles.candles c LEFT JOIN api_stream_dev.candle_projection p USING(pool_address,timeframe,candle_start)
        WHERE c.pool_address=$1 AND c.timeframe=$2 AND c.candle_start*1000 >= $3 AND c.candle_start*1000 < $4`, r.Pool, r.Interval, r.From, r.To).Scan(&count, &coherent, &sequence, &fingerprint)
	if err != nil {
		return nil, err
	}
	if !coherent {
		return nil, ErrDataIntegrity
	}
	if count == 0 {
		return nil, ErrAccountingNotReady
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
	var last int64
	for rows.Next() {
		var start int64
		var payload []byte
		if err = rows.Scan(&start, &payload); err != nil {
			rows.Close()
			return nil, err
		}
		if len(page.Items) == r.Limit {
			page.NextStart = &last
			break
		}
		if interfaces.ValidateJSON("Candle", payload) != nil {
			rows.Close()
			return nil, ErrDataIntegrity
		}
		var doc struct {
			SeriesModelVersion string `json:"seriesModelVersion"`
		}
		if err = json.Unmarshal(payload, &doc); err != nil {
			rows.Close()
			return nil, err
		}
		if page.SeriesModelVersion != "" && page.SeriesModelVersion != doc.SeriesModelVersion {
			rows.Close()
			return nil, ErrDataIntegrity
		}
		page.SeriesModelVersion = doc.SeriesModelVersion
		page.Items = append(page.Items, json.RawMessage(payload))
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
	var wm ChainWatermark
	if err = json.Unmarshal(watermark, &wm); err != nil {
		return nil, err
	}
	wm.ProjectionRevision = page.Revision
	page.Watermark, err = json.Marshal(wm)
	if err != nil {
		return nil, err
	}
	return page, tx.Commit(ctx)
}

var ErrDevelopmentCandleSnapshotChanged = errors.New("development candle snapshot changed")
