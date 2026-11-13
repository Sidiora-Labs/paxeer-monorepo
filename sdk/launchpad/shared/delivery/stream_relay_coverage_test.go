package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

func TestStreamOutboxRelayPublishesCommittedFrame(t *testing.T) {
	dsn := os.Getenv("DELIVERY_TEST_DSN")
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Hostname() != "127.0.0.1" || parsed.Port() != "25432" || parsed.Path != "/kindle_fixture" {
		t.Fatal("private PostgreSQL fixture required")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	database := fmt.Sprintf("stream_relay_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{database}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := admin.Exec(cleanup, "DROP DATABASE "+quoted+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
	})
	parsed.Path = "/" + database
	pool, err := pgxpool.New(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	_, err = pool.Exec(ctx, `CREATE SCHEMA api_stream;
CREATE TABLE api_stream.stream_projection(partition_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,topic text NOT NULL UNIQUE,sequence bigint NOT NULL DEFAULT 0 CHECK(sequence>=0),frame bytea,updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),CHECK(sequence=0 OR frame IS NOT NULL));
CREATE TABLE api_stream.stream_delivery_outbox(partition_id bigint NOT NULL REFERENCES api_stream.stream_projection(partition_id),sequence bigint NOT NULL CHECK(sequence>0),event_id text NOT NULL UNIQUE,frame bytea NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),PRIMARY KEY(partition_id,sequence));`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, StreamDeliveryDDL); err != nil {
		t.Fatal(err)
	}
	frame := committedPriceFrame(t)
	wire, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseCommittedStreamFrame(wire); err != nil {
		t.Fatalf("committed price frame: %v", err)
	}
	var partition int64
	if err := pool.QueryRow(ctx, "INSERT INTO api_stream.stream_projection(topic,sequence,frame) VALUES($1,1,$2) RETURNING partition_id", frame["topic"], wire).Scan(&partition); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO api_stream.stream_delivery_outbox(partition_id,sequence,event_id,frame) VALUES($1,1,$2,$3)", partition, frame["eventId"], wire); err != nil {
		t.Fatal(err)
	}
	connection := orderedBroker(t)
	broker, err := connection.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := broker.AddStream(&nats.StreamConfig{Name: "STREAM_RELAY", Subjects: []string{"market.price.*"}}); err != nil {
		t.Fatal(err)
	}
	budget, err := NewWorkBudget(1, 1000, 100)
	if err != nil {
		t.Fatal(err)
	}
	guard := func(ctx context.Context, tx pgx.Tx) error {
		var actual string
		if err := tx.QueryRow(ctx, "SELECT current_database()").Scan(&actual); err != nil {
			return err
		}
		if actual != database {
			return ErrIntegrity
		}
		return nil
	}
	relay, err := NewStreamOutboxRelay(pool, connection, StreamOutboxConfig{Stream: "STREAM_RELAY", Owner: "relay", Batch: 1, MaxAttempts: 3, LeaseDuration: 5 * time.Second, PublishTimeout: 2 * time.Second, RetryDelay: time.Second}, guard, budget)
	if err != nil {
		t.Fatal(err)
	}
	if err := relay.RunOnce(ctx); err != nil {
		t.Fatalf("publish committed frame: %v", err)
	}
	var state, stream string
	var sequence, next int64
	if err := pool.QueryRow(ctx, "SELECT p.state,p.broker_stream,p.broker_sequence,c.next_sequence FROM api_stream.stream_delivery_publications p JOIN api_stream.stream_delivery_offsets c USING(partition_id) WHERE p.partition_id=$1", partition).Scan(&state, &stream, &sequence, &next); err != nil || state != "published" || stream != "STREAM_RELAY" || sequence != 1 || next != 2 {
		t.Fatalf("durable publication: %s, %s, %d, %d, %v", state, stream, sequence, next, err)
	}
	message, err := broker.GetMsg("STREAM_RELAY", uint64(sequence))
	if err != nil || message.Subject != frame["topic"] || !bytes.Equal(message.Data, wire) {
		t.Fatalf("broker publication: %+v, %v", message, err)
	}
	if err := relay.RunOnce(ctx); err != nil {
		t.Fatalf("empty relay pass: %v", err)
	}
	stats, err := relay.Stats(ctx)
	if err != nil || stats.PendingRows != 0 || stats.QuarantinedRows != 0 || stats.GapPartitions != 0 {
		t.Fatalf("published relay stats: %+v, %v", stats, err)
	}
	if err := relay.Ready(ctx, time.Minute); err != nil {
		t.Fatalf("published stream readiness: %v", err)
	}
	if err := relay.Ready(ctx, 0); err == nil || !strings.Contains(err.Error(), "lag budget") {
		t.Fatalf("missing readiness budget: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE api_stream.stream_projection SET sequence=2 WHERE partition_id=$1", partition); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO api_stream.stream_delivery_outbox(partition_id,sequence,event_id,frame) VALUES($1,2,$2,$3)", partition, "changed_event", wire); err != nil {
		t.Fatal(err)
	}
	if err := relay.RunOnce(ctx); err == nil {
		t.Fatal("changed committed publication identity was delivered")
	}
	var failure string
	if err := pool.QueryRow(ctx, "SELECT state,failure_class FROM api_stream.stream_delivery_publications WHERE partition_id=$1 AND sequence=2", partition).Scan(&state, &failure); err != nil || state != "quarantined" || failure != string(IntegrityConflict) {
		t.Fatalf("corrupt publication quarantine: %s, %s, %v", state, failure, err)
	}
	if err := relay.Ready(ctx, time.Minute); err == nil {
		t.Fatal("quarantined committed publication reported ready")
	}
	_, err = pool.Exec(ctx, `CREATE TABLE api_stream.delivery_consumer_watermarks(
consumer_id text NOT NULL,generation text NOT NULL,partition_kind text NOT NULL,partition_key text NOT NULL,
broker_stream text NOT NULL,broker_sequence bigint NOT NULL CHECK(broker_sequence>0),source_order jsonb NOT NULL,
position_sequence bigint NOT NULL CHECK(position_sequence>0),event_id text NOT NULL,envelope_hash text NOT NULL,
committed_at timestamptz NOT NULL DEFAULT clock_timestamp(),PRIMARY KEY(consumer_id,generation,partition_kind,partition_key));
CREATE SCHEMA indexer;
CREATE TABLE indexer.indexer_ingest_watermarks_v1(chain_id bigint NOT NULL,stream_id text NOT NULL,typed_complete_height bigint NOT NULL,PRIMARY KEY(chain_id,stream_id));
CREATE TABLE indexer.indexer_block_completeness_v1(chain_id bigint NOT NULL,stream_id text NOT NULL,height bigint NOT NULL,fetch_complete boolean NOT NULL,required_decode_complete boolean NOT NULL,PRIMARY KEY(chain_id,stream_id,height));`)
	if err != nil {
		t.Fatal(err)
	}
	key := ConsumerKey{ConsumerID: "stream_consumer", ProjectionGeneration: "g1", PartitionKind: "market", PartitionKey: "125:pool"}
	sourceOrder := SourceOrder{Height: "1", TransactionIndex: "0", LogIndex: "0"}
	if _, err := pool.Exec(ctx, `INSERT INTO api_stream.delivery_consumer_watermarks(consumer_id,generation,partition_kind,partition_key,broker_stream,broker_sequence,source_order,position_sequence,event_id,envelope_hash) VALUES($1,$2,$3,$4,'STREAM_RELAY',1,$5,1,'price_1',$6)`, key.ConsumerID, key.ProjectionGeneration, key.PartitionKind, key.PartitionKey, sourceOrder, Hash(wire)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks_v1 VALUES(125,'stream_coverage',2);
INSERT INTO indexer.indexer_block_completeness_v1 VALUES(125,'stream_coverage',1,true,true),(125,'stream_coverage',2,true,true)`); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(pool, "api_stream", fixturePolicy())
	if err != nil {
		t.Fatal(err)
	}
	status, err := store.ConsumerReplayStatus(ctx, key, "stream_coverage", 1)
	if err != nil || !status.Ready || status.Gap || status.SourceHeight != 2 || status.ManifestHeight != 2 || status.BrokerSequence != 1 || status.Committed != sourceOrder {
		t.Fatalf("complete replay status: %+v, %v", status, err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM indexer.indexer_block_completeness_v1 WHERE height=2"); err != nil {
		t.Fatal(err)
	}
	status, err = store.ConsumerReplayStatus(ctx, key, "stream_coverage", 1)
	if err != nil || status.Ready || !status.Gap || status.ManifestHeight != 1 {
		t.Fatalf("incomplete replay status: %+v, %v", status, err)
	}
	if _, err := store.ConsumerReplayStatus(ctx, key, "stream_coverage", 10001); !errors.Is(err, ErrSchema) {
		t.Fatalf("unbounded replay lag accepted: %v", err)
	}

	for _, mode := range []string{"broker identity", "malformed order", "noncanonical order", "order overflow", "future order", "source behind", "unbounded source", "source unavailable", "completeness unavailable"} {
		t.Run("readiness/"+mode, func(t *testing.T) {
			switch mode {
			case "broker identity":
				_, err = pool.Exec(ctx, "UPDATE api_stream.delivery_consumer_watermarks SET broker_stream=''")
			case "malformed order":
				_, err = pool.Exec(ctx, "UPDATE api_stream.delivery_consumer_watermarks SET source_order='[]'")
			case "noncanonical order":
				_, err = pool.Exec(ctx, "UPDATE api_stream.delivery_consumer_watermarks SET source_order='{}'")
			case "order overflow":
				_, err = pool.Exec(ctx, `UPDATE api_stream.delivery_consumer_watermarks SET source_order='{"height":"18446744073709551616","transaction_index":"0","log_index":"0"}'`)
			case "future order":
				_, err = pool.Exec(ctx, `UPDATE api_stream.delivery_consumer_watermarks SET source_order='{"height":"3","transaction_index":"0","log_index":"0"}'`)
			case "source behind":
				_, err = pool.Exec(ctx, "UPDATE indexer.indexer_ingest_watermarks_v1 SET typed_complete_height=-1")
			case "unbounded source":
				_, err = pool.Exec(ctx, "UPDATE indexer.indexer_ingest_watermarks_v1 SET typed_complete_height=10002")
			case "source unavailable":
				_, err = pool.Exec(ctx, "ALTER TABLE indexer.indexer_ingest_watermarks_v1 RENAME COLUMN typed_complete_height TO unavailable_height")
			case "completeness unavailable":
				_, err = pool.Exec(ctx, "ALTER TABLE indexer.indexer_block_completeness_v1 RENAME COLUMN height TO unavailable_height")
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.ConsumerReplayStatus(ctx, key, "stream_coverage", 1); err == nil {
				t.Fatal("unbound consumer readiness admitted")
			}
			if mode == "source unavailable" {
				if _, err := pool.Exec(ctx, "ALTER TABLE indexer.indexer_ingest_watermarks_v1 RENAME COLUMN unavailable_height TO typed_complete_height"); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "completeness unavailable" {
				if _, err := pool.Exec(ctx, "ALTER TABLE indexer.indexer_block_completeness_v1 RENAME COLUMN unavailable_height TO height"); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := pool.Exec(ctx, "UPDATE api_stream.delivery_consumer_watermarks SET source_order=$1,broker_stream='STREAM_RELAY'", sourceOrder); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, "UPDATE indexer.indexer_ingest_watermarks_v1 SET typed_complete_height=2"); err != nil {
				t.Fatal(err)
			}
		})
	}
	// Catalog changes cannot authorize a publication or advance its offset.
	for _, mode := range []string{"missing offsets", "missing outbox", "missing publication", "unavailable projection", "invalid frame", "unrouted topic", "unavailable stream", "attempt bound"} {
		t.Run(mode, func(t *testing.T) {
			if _, err := pool.Exec(ctx, "TRUNCATE api_stream.stream_delivery_publications,api_stream.stream_delivery_offsets,api_stream.stream_delivery_outbox"); err != nil {
				t.Fatal(err)
			}
			data := append([]byte(nil), wire...)
			if mode == "invalid frame" {
				data = []byte(`{}`)
			}
			if _, err := pool.Exec(ctx, "INSERT INTO api_stream.stream_delivery_outbox(partition_id,sequence,event_id,frame) VALUES($1,1,$2,$3)", partition, frame["eventId"], data); err != nil {
				t.Fatal(err)
			}
			candidate := *relay
			switch mode {
			case "missing offsets", "missing outbox", "missing publication", "unavailable projection":
				table := map[string]string{"missing offsets": "stream_delivery_offsets", "missing outbox": "stream_delivery_outbox", "missing publication": "stream_delivery_publications", "unavailable projection": "stream_projection"}[mode]
				if _, err := pool.Exec(ctx, "ALTER TABLE api_stream."+table+" RENAME TO unavailable_catalog"); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if _, err := pool.Exec(ctx, "ALTER TABLE api_stream.unavailable_catalog RENAME TO "+table); err != nil {
						t.Error(err)
					}
				}()
			case "unrouted topic":
				candidate.config.Routes = []StreamRoute{{Subjects: []string{"market.trade.*"}, Stream: "STREAM_RELAY"}}
			case "unavailable stream":
				candidate.config.Stream = "UNAVAILABLE"
			case "attempt bound":
				if _, err := pool.Exec(ctx, "INSERT INTO api_stream.stream_delivery_offsets(partition_id) VALUES($1)", partition); err != nil {
					t.Fatal(err)
				}
				if _, err := pool.Exec(ctx, "INSERT INTO api_stream.stream_delivery_publications(partition_id,sequence,event_id,frame_sha256,attempts) VALUES($1,1,$2,$3,3)", partition, frame["eventId"], Hash(data)); err != nil {
					t.Fatal(err)
				}
			}
			err := candidate.RunOnce(ctx)
			if mode == "attempt bound" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("unavailable publication authority admitted")
			}
			if mode == "invalid frame" || mode == "attempt bound" || mode == "unrouted topic" || mode == "unavailable stream" {
				var recordedState, class string
				if err := pool.QueryRow(ctx, "SELECT state,failure_class FROM api_stream.stream_delivery_publications WHERE partition_id=$1 AND sequence=1", partition).Scan(&recordedState, &class); err != nil {
					t.Fatal(err)
				}
				expectedState, expectedClass := "pending", "transient"
				if mode == "invalid frame" {
					expectedState, expectedClass = "quarantined", "schema_violation"
				}
				if mode == "attempt bound" {
					expectedState = "quarantined"
				}
				if recordedState != expectedState || class != expectedClass {
					t.Fatalf("publication disposition: %s %s", recordedState, class)
				}
				var next int64
				if err := pool.QueryRow(ctx, "SELECT next_sequence FROM api_stream.stream_delivery_offsets WHERE partition_id=$1", partition).Scan(&next); err != nil || next != 1 {
					t.Fatalf("refused publication offset: %d %v", next, err)
				}
			}
		})
	}
	// A real persistence acknowledgement remains bound to the leased frame.
	if _, err := pool.Exec(ctx, "TRUNCATE api_stream.stream_delivery_publications,api_stream.stream_delivery_offsets,api_stream.stream_delivery_outbox"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO api_stream.stream_delivery_outbox(partition_id,sequence,event_id,frame) VALUES($1,1,$2,$3)", partition, frame["eventId"], wire); err != nil {
		t.Fatal(err)
	}
	lease, err := relay.claim(ctx)
	if err != nil || lease == nil {
		t.Fatalf("frame lease: %+v %v", lease, err)
	}
	ack, err := relay.js.Publish(ctx, lease.topic, lease.frame)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"missing acknowledgement", "wrong stream", "changed frame", "changed topic", "expired lease", "offset gap"} {
		t.Run(mode, func(t *testing.T) {
			candidate := *lease
			actualAck := ack
			expectedStream := ack.Stream
			switch mode {
			case "missing acknowledgement":
				actualAck = nil
			case "wrong stream":
				expectedStream = "OTHER"
			case "changed frame":
				candidate.frame = []byte("changed")
			case "changed topic":
				candidate.topic = "market.price.other"
			case "expired lease":
				candidate.epoch++
			case "offset gap":
				if _, err := pool.Exec(ctx, "UPDATE api_stream.stream_delivery_offsets SET next_sequence=2 WHERE partition_id=$1", partition); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if _, err := pool.Exec(ctx, "UPDATE api_stream.stream_delivery_offsets SET next_sequence=1 WHERE partition_id=$1", partition); err != nil {
						t.Error(err)
					}
				}()
			}
			if err := relay.markPublished(ctx, &candidate, expectedStream, actualAck); err == nil {
				t.Fatal("unbound acknowledgement advanced publication")
			}
			var state string
			if err := pool.QueryRow(ctx, "SELECT state FROM api_stream.stream_delivery_publications WHERE partition_id=$1", partition).Scan(&state); err != nil || state != "pending" {
				t.Fatalf("refused publication: %s %v", state, err)
			}
		})
	}
	if err := relay.markPublished(ctx, lease, ack.Stream, ack); err != nil {
		t.Fatalf("bound acknowledgement: %v", err)
	}
	if err := relay.fail(ctx, lease, Transient); !errors.Is(err, ErrLease) {
		t.Fatalf("published lease failure: %v", err)
	}

}
