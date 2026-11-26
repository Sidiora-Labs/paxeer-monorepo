package l16

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/consumerhealth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
)

type InventoryCatchUp struct {
	Consumer *AccountingConsumer
	Period   time.Duration
	MinAge   time.Duration
	Limit    int
	Now      func() time.Time
}

func NewInventoryCatchUp(consumer *AccountingConsumer) InventoryCatchUp {
	return InventoryCatchUp{Consumer: consumer, Period: 30 * time.Second, MinAge: 120 * time.Second, Limit: 16, Now: time.Now}
}

type inventoryRecordDelivery struct{}

func (inventoryRecordDelivery) Ack(context.Context) error { return nil }

func (c *AccountingConsumer) recordProgress(ctx context.Context, result error, wire []byte) {
	class := consumerhealth.ErrorClass(result)
	if errors.Is(result, ErrBaselineUnavailable) {
		class = consumerhealth.SourceUnavailableClass
	}
	c.Progress.RecordDelivery(ctx, result, class, wire)
}

func (c *AccountingConsumer) inventoryRecordWire(ctx context.Context, raw []byte) ([]byte, error) {
	publication, err := accounting.DecodePublicationV1(raw)
	if err != nil || publication.Facet != sourceInventory || publication.Enrichment == nil || publication.Positions[0].ProjectionVersion != c.projectionVersion {
		return nil, delivery.ErrGap
	}
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer inventoryRollback(tx)
	fill, err := ReadSourceFillTx(ctx, tx, publication.SourceFillID, publication.SourceFactDigest)
	if err != nil {
		return nil, err
	}
	if fill.Position != publication.Positions[0].SourceOrder {
		return nil, delivery.ErrIntegrity
	}
	payload, err := accounting.CanonicalPublicationV1(publication)
	if err != nil {
		return nil, delivery.ErrGap
	}
	seconds, err := strconv.ParseInt(fill.BlockTimestampSeconds, 10, 64)
	if err != nil || seconds <= 0 || seconds > math.MaxInt64/1000 {
		return nil, delivery.ErrIntegrity
	}
	envelope := delivery.Envelope{
		EventID: publication.PublicationID, EventType: accounting.InventorySubject,
		SchemaVersion: 1, ChainID: 125,
		SourceOrder: delivery.SourceOrder{Height: fill.Position.Height, TransactionIndex: fill.Position.TransactionIndex, LogIndex: fill.Position.LogIndex},
		EmittedAt:   time.Unix(seconds, 0).UTC(), TraceID: publication.PublicationID,
		PayloadHash: delivery.Hash(payload), Payload: json.RawMessage(payload),
	}
	policy := delivery.Policy{Subjects: map[string]bool{accounting.InventorySubject: true}, EventTypes: map[string]bool{accounting.InventorySubject: true}}
	return envelope.Encode(policy, accounting.InventorySubject)
}

func (c *AccountingConsumer) catchUpBefore(ctx context.Context, position accounting.PartitionPosition) error {
	sequence, err := strconv.ParseInt(position.Sequence, 10, 64)
	if err != nil {
		return delivery.ErrGap
	}
	var next int64
	err = c.pool.QueryRow(ctx, "SELECT next_sequence FROM "+c.schema+".delivery_partition_offsets WHERE consumer_id=$1 AND projection_generation=$2 AND partition_kind='inventory' AND partition_key=$3", c.consumerID, c.generation, position.PartitionKey).Scan(&next)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	for ; next < sequence; next++ {
		var raw []byte
		err = c.pool.QueryRow(ctx, `SELECT payload FROM indexer.indexer_accounting_publications_v1 WHERE payload->>'facet'='inventory' AND payload->'positions'->0->>'partitionKey'=$1 AND payload->'positions'->0->>'sequence'=$2 AND payload->'positions'->0->>'projectionVersion'=$3`, position.PartitionKey, strconv.FormatInt(next, 10), c.projectionVersion).Scan(&raw)
		if errors.Is(err, pgx.ErrNoRows) {
			return delivery.ErrGap
		}
		if err != nil {
			return err
		}
		wire, err := c.inventoryRecordWire(ctx, raw)
		if err != nil {
			return err
		}
		err = c.consumeOrdered(ctx, accounting.InventorySubject, wire, inventoryRecordDelivery{})
		c.recordProgress(ctx, err, wire)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p InventoryCatchUp) RunOnce(ctx context.Context) (int, error) {
	if p.Consumer == nil || p.MinAge < 0 || p.Limit <= 0 || p.Now == nil {
		return 0, errors.New("invalid inventory catch up configuration")
	}
	c := p.Consumer
	cutoff := p.Now().Add(-p.MinAge).Unix()
	for applied := 0; applied < p.Limit; applied++ {
		var raw []byte
		err := c.pool.QueryRow(ctx, `SELECT p.payload FROM indexer.indexer_accounting_publications_v1 p
JOIN indexer.indexer_accounting_fills_v1 f ON f.source_fill_id=p.payload->>'sourceFillId'
LEFT JOIN `+c.schema+`.delivery_partition_offsets o ON o.consumer_id=$1 AND o.projection_generation=$2 AND o.partition_kind='inventory' AND o.partition_key=p.payload->'positions'->0->>'partitionKey'
WHERE p.payload->>'facet'='inventory' AND p.payload->'positions'->0->>'projectionVersion'=$3
AND (p.payload->'positions'->0->>'sequence')::numeric=COALESCE(o.next_sequence,1)
AND (f.payload->>'blockTimestampSeconds')::numeric<$4
ORDER BY (p.payload->'positions'->0->'sourceOrder'->>'height')::numeric,(p.payload->'positions'->0->'sourceOrder'->>'transactionIndex')::numeric,(p.payload->'positions'->0->'sourceOrder'->>'logIndex')::numeric,p.publication_id
LIMIT 1`, c.consumerID, c.generation, c.projectionVersion, cutoff).Scan(&raw)
		if errors.Is(err, pgx.ErrNoRows) {
			return applied, nil
		}
		if err != nil {
			c.recordProgress(ctx, err, nil)
			return applied, err
		}
		wire, err := c.inventoryRecordWire(ctx, raw)
		if err != nil {
			c.recordProgress(ctx, err, nil)
			return applied, err
		}
		if err = c.PrepareAndConsume(ctx, accounting.InventorySubject, wire, inventoryRecordDelivery{}); err != nil {
			return applied, err
		}
	}
	return p.Limit, nil
}
