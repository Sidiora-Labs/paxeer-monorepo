package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/jackc/pgx/v5"
)

// These are the persisted qualified-exact-v2 fields, in their producer's wire
// order. Hashing the typed snapshot preserves the canonical snapshot identity.
type rebuildOrder struct {
	Height           uint64 `json:"height"`
	TransactionIndex uint64 `json:"transactionIndex"`
	LogIndex         uint64 `json:"logIndex"`
}
type rebuildPrice struct {
	Numerator   string `json:"numerator"`
	Denominator string `json:"denominator"`
}
type rebuildPriceSource struct {
	SourceFillID  string                    `json:"sourceFillId"`
	Position      accounting.SourcePosition `json:"position"`
	ActualEmitter string                    `json:"actualEmitter"`
	Provenance    accounting.Provenance     `json:"provenance"`
}
type rebuildSnapshot struct {
	PoolAddress              string             `json:"poolAddress"`
	Interval                 string             `json:"interval"`
	Start                    int64              `json:"start"`
	Open                     rebuildPrice       `json:"open"`
	High                     rebuildPrice       `json:"high"`
	Low                      rebuildPrice       `json:"low"`
	Close                    rebuildPrice       `json:"close"`
	FirstPosition            rebuildOrder       `json:"firstPosition"`
	LastPosition             rebuildOrder       `json:"lastPosition"`
	VolumeUsdlRaw            string             `json:"volumeUsdlRaw"`
	VolumeTokenRaw           string             `json:"volumeTokenRaw"`
	BuyVolumeUsdlRaw         string             `json:"buyVolumeUsdlRaw"`
	SellVolumeUsdlRaw        string             `json:"sellVolumeUsdlRaw"`
	Trades                   uint64             `json:"trades"`
	Revision                 uint64             `json:"revision"`
	UniqueTraders            uint64             `json:"uniqueTraderCount"`
	Token                    accounting.Asset   `json:"token"`
	Settlement               accounting.Asset   `json:"settlement"`
	OpenSource               rebuildPriceSource `json:"openSource"`
	HighSource               rebuildPriceSource `json:"highSource"`
	LowSource                rebuildPriceSource `json:"lowSource"`
	CloseSource              rebuildPriceSource `json:"closeSource"`
	UnknownActorTradeCount   uint64             `json:"unknownActorTradeCount"`
	ActorSemantics           string             `json:"actorSemantics"`
	AlgorithmVersion         string             `json:"algorithmVersion"`
	SourceWatermarkHeight    uint64             `json:"sourceWatermarkHeight"`
	SourceWatermarkTimestamp int64              `json:"sourceWatermarkTimestamp"`
	IntervalEnd              int64              `json:"intervalEnd"`
	CompletenessStatus       string             `json:"completenessStatus"`
	Synthetic                bool               `json:"synthetic"`
	Provenance               string             `json:"provenance"`
	DisplayRevision          uint64             `json:"displayRevision"`
	CorrectionReason         string             `json:"correctionReason,omitempty"`
}

func rebuildTimestamp(seconds int64) (string, error) {
	stamp := time.Unix(seconds, 0).UTC()
	if seconds < 0 || stamp.Year() < 1 || stamp.Year() > 9999 {
		return "", ErrIntegrity
	}
	return stamp.Format("2006-01-02T15:04:05.000Z"), nil
}
func rebuildPosition(p accounting.SourcePosition) interfaces.ChainPosition {
	return interfaces.ChainPosition{BlockHash: p.BlockHash, ChainId: p.ChainID, Height: interfaces.UInt256(p.Height), TransactionHash: p.TransactionHash, TransactionIndex: interfaces.UInt256(p.TransactionIndex), LogIndex: interfaces.UInt256(p.LogIndex)}
}
func rebuildAsset(a accounting.Asset) interfaces.AssetRef {
	chain, address, decimals := "125", a.Address, interfaces.SafeInteger(a.Decimals)
	return interfaces.AssetRef{Kind: "erc20", ChainId: &chain, Address: &address, Decimals: &decimals}
}
func rebuildOrderMatches(p accounting.SourcePosition, o rebuildOrder) bool {
	return p.Height == strconv.FormatUint(o.Height, 10) && p.TransactionIndex == strconv.FormatUint(o.TransactionIndex, 10) && p.LogIndex == strconv.FormatUint(o.LogIndex, 10)
}
func rebuildCanonicalPayload(ctx context.Context, tx pgx.Tx, schema string, bar rebuildSnapshot) ([]byte, error) {
	durations := map[string]int64{"1m": 60, "5m": 300, "15m": 900, "1h": 3600, "4h": 14400, "1d": 86400, "1w": 604800}
	duration := durations[bar.Interval]
	if bar.AlgorithmVersion != "qualified-exact-v2" || bar.Synthetic || bar.Provenance != "actual_trade" || bar.Trades == 0 || bar.Revision == 0 || bar.DisplayRevision == 0 || bar.ActorSemantics != "canonical_swap_actor_v1" || duration == 0 || bar.IntervalEnd-bar.Start != duration || !rebuildOrderMatches(bar.OpenSource.Position, bar.FirstPosition) || !rebuildOrderMatches(bar.CloseSource.Position, bar.LastPosition) {
		return nil, ErrIntegrity
	}
	start, err := rebuildTimestamp(bar.Start)
	if err != nil {
		return nil, err
	}
	end, err := rebuildTimestamp(bar.IntervalEnd)
	if err != nil {
		return nil, err
	}
	coverage := "partial"
	switch bar.CompletenessStatus {
	case "complete_through_watermark":
		coverage = "complete"
	case "live_interval", "interval_elapsed", statusBackfillIncomplete:
	default:
		return nil, ErrIntegrity
	}
	prices := []rebuildPrice{bar.Open, bar.High, bar.Low, bar.Close}
	refs := []rebuildPriceSource{bar.OpenSource, bar.HighSource, bar.LowSource, bar.CloseSource}
	mapped := make([]interfaces.Price, 4)
	for i, ref := range refs {
		var raw []byte
		// Authorize every price independently of the archive and staged snapshot.
		err = tx.QueryRow(ctx, `SELECT i.fill FROM `+schema+`.chart_inbox i JOIN indexer.indexer_accounting_fills_v1 f ON f.source_fill_id=$6 AND f.pool_address=i.pool AND f.payload=i.fill AND f.source_fact_digest=encode(i.digest,'hex') WHERE i.chain_id='125' AND i.pool=$1 AND i.emitter=$2 AND i.code_epoch=$3 AND i.tx_hash=$4 AND i.log_index=$5::numeric`, bar.PoolAddress, ref.ActualEmitter, ref.Provenance.EmitterCodeEpoch, ref.Position.TransactionHash, ref.Position.LogIndex, ref.SourceFillID).Scan(&raw)
		if err != nil {
			return nil, err
		}
		fill, err := accounting.DecodeFillV1(raw)
		if err != nil {
			return nil, err
		}
		if fill.SourceFillID != ref.SourceFillID || fill.Position != ref.Position || fill.ActualEmitter != ref.ActualEmitter || fill.Provenance != ref.Provenance || fill.PoolAddress != bar.PoolAddress || fill.Token != bar.Token || fill.Settlement != bar.Settlement {
			return nil, ErrIntegrity
		}
		n, ok := new(big.Int).SetString(fill.USDLAmountRaw, 10)
		if !ok {
			return nil, ErrIntegrity
		}
		d, ok := new(big.Int).SetString(fill.TokenAmountRaw, 10)
		if !ok || d.Sign() <= 0 {
			return nil, ErrIntegrity
		}
		n.Mul(n, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(fill.Token.Decimals)), nil))
		d.Mul(d, big.NewInt(1000000))
		price := new(big.Rat).SetFrac(n, d)
		if prices[i].Numerator != price.Num().String() || prices[i].Denominator != price.Denom().String() {
			return nil, ErrIntegrity
		}
		seconds, err := strconv.ParseInt(fill.BlockTimestampSeconds, 10, 64)
		if err != nil {
			return nil, err
		}
		observed, err := rebuildTimestamp(seconds)
		if err != nil {
			return nil, err
		}
		position, identity, receipt := rebuildPosition(ref.Position), ref.SourceFillID, fill.Provenance.ReceiptSHA256
		mapped[i] = interfaces.Price{Numerator: interfaces.PriceComponent(prices[i].Numerator), Denominator: interfaces.PositivePriceComponent(prices[i].Denominator), BaseAsset: rebuildAsset(bar.Token), QuoteAsset: rebuildAsset(bar.Settlement), Source: interfaces.PriceSource{Kind: "gross_execution", AmountBasis: "paid_input", SourceEventId: &identity, Position: &position, ObservedAt: observed, SourceReference: &receipt}}
	}
	raw, err := json.Marshal(bar)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(raw)
	hash[6] = (hash[6] & 0x0f) | 0x50
	hash[8] = (hash[8] & 0x3f) | 0x80
	snapshot := fmt.Sprintf("%x-%x-%x-%x-%x", hash[:4], hash[4:6], hash[6:8], hash[8:10], hash[10:16])
	unique := interfaces.UInt256(strconv.FormatUint(bar.UniqueTraders, 10))
	candle := interfaces.Candle{PoolAddress: bar.PoolAddress, Interval: bar.Interval, Start: start, Open: mapped[0], High: mapped[1], Low: mapped[2], Close: mapped[3], VolumeUsdlRaw: interfaces.AggregateUInt(bar.VolumeUsdlRaw), VolumeTokenRaw: interfaces.AggregateUInt(bar.VolumeTokenRaw), BuyVolumeUsdlRaw: interfaces.AggregateUInt(bar.BuyVolumeUsdlRaw), SellVolumeUsdlRaw: interfaces.AggregateUInt(bar.SellVolumeUsdlRaw), Trades: interfaces.PositiveUInt256(strconv.FormatUint(bar.Trades, 10)), Revision: interfaces.PositiveUInt256(strconv.FormatUint(bar.Revision, 10)), Coverage: coverage, FirstPosition: rebuildPosition(bar.OpenSource.Position), LastPosition: rebuildPosition(bar.CloseSource.Position), SeriesModelVersion: bar.AlgorithmVersion, SnapshotId: snapshot, UniqueTraderCount: &unique}
	payload, err := json.Marshal(candle)
	if err != nil {
		return nil, err
	}
	var fields map[string]any
	if err = json.Unmarshal(payload, &fields); err != nil {
		return nil, err
	}
	fields["algorithmVersion"] = bar.AlgorithmVersion
	fields["sourceWatermarkHeight"] = strconv.FormatUint(bar.SourceWatermarkHeight, 10)
	fields["intervalEnd"] = end
	fields["completenessStatus"] = bar.CompletenessStatus
	fields["provenance"] = bar.Provenance
	fields["displayRevision"] = strconv.FormatUint(bar.DisplayRevision, 10)
	fields["unknownActorTradeCount"] = strconv.FormatUint(bar.UnknownActorTradeCount, 10)
	fields["actorSemantics"] = bar.ActorSemantics
	if bar.CorrectionReason != "" {
		fields["correctionReason"] = bar.CorrectionReason
	}
	payload, err = json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	if interfaces.ValidateJSON("Candle", payload) != nil {
		return nil, ErrIntegrity
	}
	return payload, nil
}

func (s *ExactStore) appendRebuildStreamsTx(ctx context.Context, tx pgx.Tx, id, pool, manifest, output string, epoch int64) error {
	rows, err := tx.Query(ctx, `SELECT interval,start,snapshot FROM `+s.Schema+`.chart_rebuild_candles WHERE run_id=$1 ORDER BY interval,start`, id)
	if err != nil {
		return err
	}
	bars := []StagedCandle{}
	for rows.Next() {
		var bar StagedCandle
		if err = rows.Scan(&bar.Interval, &bar.Start, &bar.Snapshot); err != nil {
			rows.Close()
			return err
		}
		bars = append(bars, bar)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, staged := range bars {
		var bar rebuildSnapshot
		if err = json.Unmarshal(staged.Snapshot, &bar); err != nil {
			return err
		}
		if bar.PoolAddress != pool || bar.Interval != staged.Interval || bar.Start != staged.Start || bar.CorrectionReason != "archive_reconstruction" {
			return ErrIntegrity
		}
		payload, err := rebuildCanonicalPayload(ctx, tx, s.Schema, bar)
		if err != nil {
			return err
		}
		var rawFill, rawPosition []byte
		// The publication join and the SQL append contract both verify the canonical
		// market position; a chart revision never invents a new market fill sequence.
		err = tx.QueryRow(ctx, `SELECT f.payload,p.position FROM indexer.indexer_accounting_fills_v1 f JOIN indexer.indexer_accounting_publications_v1 a ON a.payload->>'sourceFillId'=f.source_fill_id AND a.payload->>'sourceFactDigest'=f.source_fact_digest AND a.payload->'fill'=f.payload AND a.payload->>'facet'='market' JOIN indexer.indexer_market_partition_positions_v1 p ON a.payload->'positions'->0=convert_from(p.position,'UTF8')::jsonb WHERE f.source_fill_id=$1 AND f.pool_address=$2 AND p.partition_kind='market' AND p.partition_key='125:'||$2 ORDER BY p.projection_version,p.sequence LIMIT 1`, bar.CloseSource.SourceFillID, pool).Scan(&rawFill, &rawPosition)
		if err != nil {
			return err
		}
		source, err := accounting.DecodeFillV1(rawFill)
		if err != nil {
			return err
		}
		var position accounting.PartitionPosition
		if err = json.Unmarshal(rawPosition, &position); err != nil {
			return err
		}
		if _, err = accounting.NewMarketPublication(source, []accounting.PartitionPosition{position}); err != nil {
			return err
		}
		digest, err := accounting.FillDigest(source)
		if err != nil {
			return err
		}
		sequence, err := strconv.ParseInt(position.Sequence, 10, 64)
		if err != nil || sequence < 1 {
			return ErrIntegrity
		}
		var now time.Time
		if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return err
		}
		occurred := now.UTC().Format("2006-01-02T15:04:05.000Z")
		var watermark *interfaces.Watermark
		if bar.SourceWatermarkHeight > 0 {
			var blockHash string
			if err = tx.QueryRow(ctx, `SELECT min(block_hash) FROM indexer.indexer_block_completeness_v1 WHERE chain_id=125 AND height=$1 AND fetch_complete AND required_decode_complete HAVING count(DISTINCT block_hash)=1`, bar.SourceWatermarkHeight).Scan(&blockHash); err != nil {
				return err
			}
			observed, err := rebuildTimestamp(bar.SourceWatermarkTimestamp)
			if err != nil {
				return err
			}
			watermark = &interfaces.Watermark{ChainId: "125", Height: interfaces.UInt256(strconv.FormatUint(bar.SourceWatermarkHeight, 10)), BlockHash: blockHash, ProjectionRevision: interfaces.PositiveUInt256(strconv.FormatUint(bar.DisplayRevision, 10)), ObservedAt: observed, Integrity: "healthy"}
		} else if bar.CompletenessStatus != statusBackfillIncomplete {
			return ErrIntegrity
		}
		watermarkWire, err := json.Marshal(watermark)
		if err != nil {
			return err
		}
		revision, start := strconv.FormatUint(bar.Revision, 10), strconv.FormatInt(bar.Start, 10)
		identity := "candle:" + accounting.HashTuple(bar.AlgorithmVersion, pool, bar.Interval, start, revision, position.ProjectionVersion, position.Sequence, string(digest))
		if _, err = tx.Exec(ctx, `INSERT INTO `+s.Schema+`.chart_stream_sources(chain_id,pool,interval,start,revision,event_id,snapshot,payload,watermark,occurred_at,projection_version,source_sequence,source_fill_id,source_fact_digest) VALUES('125',$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, pool, bar.Interval, bar.Start, revision, identity, staged.Snapshot, payload, watermarkWire, occurred, position.ProjectionVersion, sequence, source.SourceFillID, string(digest)); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO `+s.Schema+`.chart_rebuild_stream_sources(event_id,run_id,manifest_digest,output_digest,owner_epoch) VALUES($1,$2,$3,$4,$5)`, identity, id, manifest, output, epoch); err != nil {
			return err
		}
		writer := delivery.StreamWriter{Schema: "api_stream"}
		if err = writer.AppendTx(ctx, tx, delivery.StreamUpdate{EventID: identity, Topic: "market.candles." + pool + "." + bar.Interval, Revision: revision, OccurredAt: occurred, Watermark: watermarkWire, Payload: payload, Source: delivery.StreamSourceProof{Kind: "candle_commit", Pool: pool, ProjectionVersion: position.ProjectionVersion, Sequence: position.Sequence, SourceFillID: source.SourceFillID, SourceFactDigest: string(digest), Interval: bar.Interval, BucketStart: start, SourceID: id, SourceDigest: manifest, OwnerEpoch: strconv.FormatInt(epoch, 10)}}); err != nil {
			return err
		}
	}
	return nil
}

func (s *ExactStore) refreshRebuildSnapshotsTx(ctx context.Context, tx pgx.Tx, id, pool string) error {
	var height uint64
	var timestamp int64
	var gaps bool
	var now time.Time
	if err := tx.QueryRow(ctx, `SELECT watermark_height,watermark_timestamp,has_gaps,clock_timestamp() FROM `+s.Schema+`.chart_series_clock WHERE chain_id='125' AND pool=$1`, pool).Scan(&height, &timestamp, &gaps, &now); err != nil {
		return err
	}
	unresolved, err := s.HasUnresolvedDeliveriesTx(ctx, tx)
	if err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT c.interval,c.start,c.snapshot,EXISTS(SELECT 1 FROM `+s.Schema+`.chart_rollup_pending p WHERE p.chain_id='125' AND p.pool=$2 AND p.interval=c.interval AND p.start=c.start) FROM `+s.Schema+`.chart_rebuild_candles c WHERE c.run_id=$1 ORDER BY c.interval,c.start`, id, pool)
	if err != nil {
		return err
	}
	bars := []StagedCandle{}
	for rows.Next() {
		var bar StagedCandle
		var pending bool
		if err = rows.Scan(&bar.Interval, &bar.Start, &bar.Snapshot, &pending); err != nil {
			rows.Close()
			return err
		}
		bar.Snapshot, err = refreshSeriesSnapshot(bar.Snapshot, height, timestamp, gaps || unresolved || pending, now)
		if err != nil {
			rows.Close()
			return err
		}
		bars = append(bars, bar)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, bar := range bars {
		if _, err = tx.Exec(ctx, `UPDATE `+s.Schema+`.chart_rebuild_candles SET snapshot=$4 WHERE run_id=$1 AND interval=$2 AND start=$3`, id, bar.Interval, bar.Start, bar.Snapshot); err != nil {
			return err
		}
	}
	return nil
}
