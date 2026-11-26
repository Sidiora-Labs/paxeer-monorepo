package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
)

func streamRetryFrame(t *testing.T, ctx context.Context, pool *pgxpool.Pool, topic string, sequence int, defect ...string) int64 {
	t.Helper()
	frame := committedPriceFrame(t)
	frame["topic"], frame["sequence"], frame["revision"], frame["eventId"] = topic, fmt.Sprint(sequence), fmt.Sprint(sequence), fmt.Sprintf("retry_%s_%d", Hash([]byte(topic))[:12], sequence)
	wire, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	if len(defect) > 0 {
		switch defect[0] {
		case "schema":
			wire = []byte(`{}`)
		case "integrity":
			frame["eventId"] = "changed_identity"
		}
	}
	var partition int64
	if err = pool.QueryRow(ctx, "INSERT INTO api_stream.stream_projection(topic,sequence,frame) VALUES($1,$2,$3) ON CONFLICT(topic) DO UPDATE SET sequence=excluded.sequence,frame=excluded.frame RETURNING partition_id", topic, sequence, wire).Scan(&partition); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "INSERT INTO api_stream.stream_delivery_outbox(partition_id,sequence,event_id,frame) VALUES($1,$2,$3,$4)", partition, sequence, frame["eventId"], wire); err != nil {
		t.Fatal(err)
	}
	return partition
}

func TestStreamRelayRetryUntilBrokerAccepts(t *testing.T) {
	ctx, pool, relay := boundedOutbox(t)
	relay.config.Batch = 1
	const topic = "market.price.0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	partition := streamRetryFrame(t, ctx, pool, topic, 1)
	streamRetryFrame(t, ctx, pool, topic, 2)
	for attempt, delay := range []float64{1, 2, 4, 5, 5, 5} {
		err := relay.RunOnce(ctx)
		var pass *StreamRetryPassError
		if !errors.As(err, &pass) || !pass.RetriedOnly || pass.Quarantined || pass.Attempts != attempt+1 {
			t.Fatalf("attempt %d disposition: %+v, %v", attempt+1, pass, err)
		}
		if !errors.Is(err, jetstream.ErrNoStreamResponse) {
			t.Fatalf("attempt %d lost publication cause: %v", attempt+1, err)
		}
		for _, detail := range []string{"stream: persistence publication unavailable", topic, fmt.Sprintf("partition=%d", partition), "sequence=1", fmt.Sprintf("attempts=%d", attempt+1)} {
			if !strings.Contains(err.Error(), detail) {
				t.Fatalf("publication error lacks %q: %v", detail, err)
			}
		}
		if strings.Contains(err.Error(), "providerId") || strings.Contains(err.Error(), "numerator") {
			t.Fatalf("publication error contains frame payload: %v", err)
		}
		var state, class string
		var attempts, next int
		var remaining float64
		if err = pool.QueryRow(ctx, "SELECT p.state,p.failure_class,p.attempts,EXTRACT(EPOCH FROM(p.available_at-clock_timestamp()))::double precision,c.next_sequence FROM api_stream.stream_delivery_publications p JOIN api_stream.stream_delivery_offsets c USING(partition_id) WHERE p.partition_id=$1 AND p.sequence=1", partition).Scan(&state, &class, &attempts, &remaining, &next); err != nil {
			t.Fatal(err)
		}
		if state != "pending" || class != string(Transient) || attempts != attempt+1 || next != 1 || remaining < delay-0.5 || remaining > delay+0.5 {
			t.Fatalf("attempt %d: %s %s attempts=%d next=%d delay=%f, want %f", attempt+1, state, class, attempts, next, remaining, delay)
		}
		if _, err = pool.Exec(ctx, "UPDATE api_stream.stream_delivery_publications SET available_at=clock_timestamp() WHERE partition_id=$1", partition); err != nil {
			t.Fatal(err)
		}
	}
	stream, err := relay.js.CreateStream(ctx, jetstream.StreamConfig{Name: "BOUNDED", Subjects: []string{topic}})
	if err != nil {
		t.Fatal(err)
	}
	for sequence := 1; sequence <= 2; sequence++ {
		if err = relay.RunOnce(ctx); err != nil {
			t.Fatalf("publish sequence %d: %v", sequence, err)
		}
		var state string
		var next int
		var wire []byte
		if err = pool.QueryRow(ctx, "SELECT p.state,c.next_sequence,o.frame FROM api_stream.stream_delivery_publications p JOIN api_stream.stream_delivery_offsets c USING(partition_id) JOIN api_stream.stream_delivery_outbox o USING(partition_id,sequence) WHERE p.partition_id=$1 AND p.sequence=$2", partition, sequence).Scan(&state, &next, &wire); err != nil || state != "published" || next != sequence+1 {
			t.Fatalf("published sequence %d: state=%s next=%d: %v", sequence, state, next, err)
		}
		message, err := stream.GetMsg(ctx, uint64(sequence))
		if err != nil || message.Subject != topic || !bytes.Equal(message.Data, wire) {
			t.Fatalf("ordered broker frame %d: %+v: %v", sequence, message, err)
		}
	}
	if err = relay.Ready(ctx, time.Minute); err != nil {
		t.Fatalf("recovered relay readiness: %v", err)
	}
}

func TestStreamRelayRetryQuarantinesDefects(t *testing.T) {
	for _, mode := range []string{"schema", "integrity"} {
		t.Run(mode, func(t *testing.T) {
			ctx, pool, relay := boundedOutbox(t)
			partition := streamRetryFrame(t, ctx, pool, "market.price.0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 1, mode)
			class := SchemaViolation
			if mode == "integrity" {
				class = IntegrityConflict
			}
			err := relay.RunOnce(ctx)
			var pass *StreamRetryPassError
			if !errors.As(err, &pass) || !pass.Quarantined || pass.RetriedOnly {
				t.Fatalf("defect disposition: %+v: %v", pass, err)
			}
			var state, actualClass string
			var attempts int
			if err = pool.QueryRow(ctx, "SELECT state,failure_class,attempts FROM api_stream.stream_delivery_publications WHERE partition_id=$1", partition).Scan(&state, &actualClass, &attempts); err != nil || state != "quarantined" || actualClass != string(class) || attempts != 1 {
				t.Fatalf("defect state=%s class=%s attempts=%d: %v", state, actualClass, attempts, err)
			}
			if err = relay.Ready(ctx, time.Minute); err == nil {
				t.Fatal("quarantined frame reported ready")
			}
		})
	}
}

func TestStreamRelayRetryAboveAttemptBoundPublishes(t *testing.T) {
	ctx, pool, relay := boundedOutbox(t)
	relay.config.Batch = 1
	const topic = "market.price.0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	partition := streamRetryFrame(t, ctx, pool, topic, 1)
	lease, err := relay.claim(ctx)
	if err != nil || lease == nil {
		t.Fatalf("publication lease: %+v: %v", lease, err)
	}
	if _, err = pool.Exec(ctx, "UPDATE api_stream.stream_delivery_publications SET attempts=10,expires_at=clock_timestamp()-interval '1 second' WHERE partition_id=$1", partition); err != nil {
		t.Fatal(err)
	}
	stream, err := relay.js.CreateStream(ctx, jetstream.StreamConfig{Name: "BOUNDED", Subjects: []string{topic}})
	if err != nil {
		t.Fatal(err)
	}
	if err = relay.RunOnce(ctx); err != nil {
		t.Fatalf("publish reclaimed frame above attempt bound: %v", err)
	}
	var state string
	var attempts, next, expiries int
	var wire []byte
	if err = pool.QueryRow(ctx, "SELECT p.state,p.attempts,p.lease_expiry_count,c.next_sequence,o.frame FROM api_stream.stream_delivery_publications p JOIN api_stream.stream_delivery_offsets c USING(partition_id) JOIN api_stream.stream_delivery_outbox o USING(partition_id,sequence) WHERE p.partition_id=$1", partition).Scan(&state, &attempts, &expiries, &next, &wire); err != nil || state != "published" || attempts != 11 || expiries != 1 || next != 2 {
		t.Fatalf("reclaimed frame: state=%s attempts=%d expiries=%d next=%d: %v", state, attempts, expiries, next, err)
	}
	message, err := stream.GetMsg(ctx, 1)
	if err != nil || message.Subject != topic || !bytes.Equal(message.Data, wire) {
		t.Fatalf("reclaimed broker frame: %+v: %v", message, err)
	}
}

func TestStreamRelayRetryDelayCeiling(t *testing.T) {
	for _, tc := range []struct {
		delay    time.Duration
		attempts int
		want     time.Duration
	}{
		{2 * time.Second, 1, 2 * time.Second},
		{2 * time.Second, 2, 4 * time.Second},
		{2 * time.Second, 3, 5 * time.Second},
		{2 * time.Second, 1000000, 5 * time.Second},
		{10 * time.Second, 1, 10 * time.Second},
		{10 * time.Second, 1000000, 10 * time.Second},
	} {
		if got := streamRetryDelay(tc.delay, tc.attempts); got != tc.want {
			t.Fatalf("delay=%s attempts=%d: got %s, want %s", tc.delay, tc.attempts, got, tc.want)
		}
	}
}

func TestStreamRelayRetryPassCombinesDispositions(t *testing.T) {
	for _, mode := range []string{"retried", "quarantined", "claim failure", "failure recording"} {
		t.Run(mode, func(t *testing.T) {
			ctx, pool, relay := boundedOutbox(t)
			relay.config.Batch = 3
			relay.config.Routes = []StreamRoute{{Stream: "BOUNDED", Subjects: []string{"market.trade.*"}}}
			first := streamRetryFrame(t, ctx, pool, "market.price.0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 1)
			defect := ""
			if mode == "quarantined" {
				defect = "schema"
			}
			second := streamRetryFrame(t, ctx, pool, "market.price.0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 1, defect)
			if _, err := pool.Exec(ctx, "INSERT INTO api_stream.stream_delivery_offsets(partition_id) VALUES($1),($2)", first, second); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, "INSERT INTO api_stream.stream_delivery_publications(partition_id,sequence,event_id,frame_sha256,attempts) SELECT partition_id,sequence,event_id,encode(sha256(frame),'hex'),CASE WHEN partition_id=$1 THEN 1 ELSE 5 END FROM api_stream.stream_delivery_outbox", first); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "claim failure", "failure recording":
				column := "attempts"
				if mode == "failure recording" {
					column = "failure_class"
				}
				if _, err := pool.Exec(ctx, "CREATE FUNCTION api_stream.refuse_publication_update() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.partition_id="+fmt.Sprint(second)+" THEN RAISE EXCEPTION 'publication update unavailable'; END IF; RETURN NEW; END $$; CREATE TRIGGER refuse_publication_update BEFORE UPDATE OF "+column+" ON api_stream.stream_delivery_publications FOR EACH ROW EXECUTE FUNCTION api_stream.refuse_publication_update()"); err != nil {
					t.Fatal(err)
				}
			}
			err := relay.RunOnce(ctx)
			var pass *StreamRetryPassError
			if !errors.As(err, &pass) || pass.RetriedOnly != (mode == "retried") || pass.Quarantined != (mode == "quarantined") {
				t.Fatalf("combined pass disposition: %+v: %v", pass, err)
			}
			wantAttempts := 2
			if mode == "retried" {
				wantAttempts = 6
			}
			if pass.Attempts != wantAttempts {
				t.Fatalf("largest retried attempt: %d, want %d", pass.Attempts, wantAttempts)
			}
			for _, detail := range []string{"no JetStream route for committed topic", fmt.Sprintf("partition=%d", first), "sequence=1", "attempts=2"} {
				if !strings.Contains(err.Error(), detail) {
					t.Fatalf("unrouted error lacks %q: %v", detail, err)
				}
			}
		})
	}
}

func TestStreamRelayRetryAcknowledgementCause(t *testing.T) {
	ctx, pool, relay := boundedOutbox(t)
	relay.config.Batch = 1
	const topic = "market.price.0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	partition := streamRetryFrame(t, ctx, pool, topic, 1)
	stream, err := relay.js.CreateStream(ctx, jetstream.StreamConfig{Name: "BOUNDED", Subjects: []string{topic}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `CREATE FUNCTION api_stream.refuse_offset_advance() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'offset acknowledgement unavailable' USING ERRCODE='40001'; END $$;
CREATE TRIGGER refuse_offset_advance BEFORE UPDATE OF next_sequence ON api_stream.stream_delivery_offsets FOR EACH ROW EXECUTE FUNCTION api_stream.refuse_offset_advance()`); err != nil {
		t.Fatal(err)
	}
	err = relay.RunOnce(ctx)
	var pass *StreamRetryPassError
	var cause *pgconn.PgError
	if !errors.As(err, &pass) || !pass.RetriedOnly || pass.Attempts != 1 || !errors.As(err, &cause) || cause.Code != "40001" {
		t.Fatalf("acknowledgement cause: %+v: %v", pass, err)
	}
	if !strings.Contains(err.Error(), "offset acknowledgement unavailable") || !strings.Contains(err.Error(), topic) {
		t.Fatalf("acknowledgement error lacks cause or identity: %v", err)
	}
	if _, err = pool.Exec(ctx, "DROP TRIGGER refuse_offset_advance ON api_stream.stream_delivery_offsets"); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "UPDATE api_stream.stream_delivery_publications SET available_at=clock_timestamp() WHERE partition_id=$1", partition); err != nil {
		t.Fatal(err)
	}
	if err = relay.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	var state string
	var next int
	if err = pool.QueryRow(ctx, "SELECT p.state,c.next_sequence FROM api_stream.stream_delivery_publications p JOIN api_stream.stream_delivery_offsets c USING(partition_id) WHERE p.partition_id=$1", partition).Scan(&state, &next); err != nil || state != "published" || next != 2 {
		t.Fatalf("recovered acknowledgement: %s next=%d: %v", state, next, err)
	}
	info, err := stream.Info(ctx)
	if err != nil || info.State.Msgs != 1 {
		t.Fatalf("retried acknowledgement duplicated frame: %+v: %v", info, err)
	}
}
