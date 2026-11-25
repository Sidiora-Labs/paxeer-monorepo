package broker

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestSnapshotStartedTopicsReplay(t *testing.T) {
	for _, startRow := range []bool{false, true} {
		name := "missing projection"
		if startRow {
			name = "empty projection"
		}
		t.Run(name, func(t *testing.T) {
			ctx, stream := postgresStreamStorage(t)
			first := publicPriceFrame(t, "1")
			empty := "market.price.0x3333333333333333333333333333333333333333"
			missing := "market.price.0x4444444444444444444444444444444444444444"
			topics := []string{first.Topic, empty, missing}
			if _, err := stream.pool.Exec(ctx, "INSERT INTO "+stream.table("stream_projection")+"(topic,sequence) VALUES($1,0)", empty); err != nil {
				t.Fatal(err)
			}
			if raw, err := stream.PublicSnapshot(ctx, topics); !errors.Is(err, ErrTopicNotStarted) || raw != nil {
				t.Fatalf("unstarted topics: %s %v", raw, err)
			}
			commitStreamFrame(t, ctx, stream, first)
			raw, err := stream.PublicSnapshot(ctx, topics)
			if err != nil || interfaces.ValidateJSON("TopicSnapshot", raw) != nil {
				t.Fatalf("snapshot: %s %v", raw, err)
			}
			var snapshot struct {
				ResumeCursor string `json:"resumeCursor"`
				Topics       []struct {
					Topic, Sequence, Revision string
					Payload                   json.RawMessage
				} `json:"topics"`
			}
			if err := json.Unmarshal(raw, &snapshot); err != nil || len(snapshot.Topics) != 1 {
				t.Fatalf("snapshot topics: %s %v", raw, err)
			}
			entry := snapshot.Topics[0]
			if entry.Topic != first.Topic || entry.Sequence != "1" || entry.Revision != first.Revision || string(entry.Payload) != string(first.Payload) {
				t.Fatalf("snapshot entry: %+v", entry)
			}
			second := publicPriceFrame(t, "2")
			commitStreamFrame(t, ctx, stream, second)
			replay, err := stream.PublicResume(ctx, topics, snapshot.ResumeCursor)
			if err != nil {
				t.Fatal(err)
			}
			defer replay.Close()
			if len(replay.Cursor.Offsets) != 1 || len(replay.partitions) != 1 || len(replay.waiting) != 2 {
				t.Fatalf("resume coverage: %+v %+v %v", replay.Cursor.Offsets, replay.partitions, replay.waiting)
			}
			decoded, err := VerifyCursorV2(snapshot.ResumeCursor, stream.key, replay.Cursor.Scope, time.Now())
			if err != nil || len(decoded.Offsets) != 1 {
				t.Fatalf("snapshot offsets: %+v %v", decoded, err)
			}
			for partition, offset := range decoded.Offsets {
				if offset != "1" || replay.partitions[partition] != first.Topic {
					t.Fatalf("snapshot offset: %s %s", partition, offset)
				}
			}
			third := publicPriceFrame(t, "3")
			commitStreamFrame(t, ctx, stream, third)
			batch, err := replay.Batch(ctx)
			if err != nil || len(batch) != 1 || replay.Machine.State != Live {
				t.Fatalf("snapshot barrier: %d %v %v", len(batch), replay.Machine.State, err)
			}
			replayed := batch
			batch, err = replay.Batch(ctx)
			if err != nil || len(batch) != 1 {
				t.Fatalf("live frames: %d %v", len(batch), err)
			}
			batch = append(replayed, batch...)
			for i, expected := range []Frame{second, third} {
				got, err := parseStreamV2(batch[i])
				if err != nil || got.EventID != expected.EventID || got.Sequence != expected.Sequence || got.Topic != first.Topic {
					t.Fatalf("ordered frame: %+v %v", got, err)
				}
			}
			next := publicPriceFrame(t, "1")
			next.Topic = missing
			if startRow {
				next.Topic = empty
			}
			var payload map[string]any
			if err := json.Unmarshal(next.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			payload["baseAsset"].(map[string]any)["address"] = next.Topic[len("market.price."):]
			next.Payload, err = json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			started := time.Now()
			commitStreamFrame(t, ctx, stream, next)
			deadline := started.Add(3 * time.Second)
			ticker := time.NewTicker(25 * time.Millisecond)
			defer ticker.Stop()
			timer := time.NewTimer(time.Until(deadline))
			defer timer.Stop()
			for {
				batch, err = replay.Batch(ctx)
				if errors.Is(err, ErrCursor) {
					break
				}
				if err != nil || len(batch) != 0 {
					t.Fatalf("waiting replay: %d %v", len(batch), err)
				}
				select {
				case <-timer.C:
					t.Fatal("started topic did not require a snapshot within three seconds")
				case <-ticker.C:
				}
			}
			if time.Now().After(deadline) || len(batch) != 0 || StreamFailure(err) != "RESUME_INVALID" || replay.Machine.State != Closed {
				t.Fatalf("started topic control: %v %v", replay.Machine.State, err)
			}
			if replay, err := stream.PublicResume(ctx, topics, snapshot.ResumeCursor); !errors.Is(err, ErrCursor) || replay != nil {
				t.Fatalf("started topic resume: %v %v", replay, err)
			}
			var rows, snapshots int
			if err := stream.pool.QueryRow(ctx, "SELECT count(*) FROM "+stream.table("stream_projection")).Scan(&rows); err != nil {
				t.Fatal(err)
			}
			expectedRows := 3
			if startRow {
				expectedRows = 2
			}
			if rows != expectedRows {
				t.Fatalf("projection rows = %d, want %d", rows, expectedRows)
			}
			if err := stream.pool.QueryRow(ctx, "SELECT count(*) FROM "+stream.table("stream_snapshots")).Scan(&snapshots); err != nil || snapshots != 0 {
				t.Fatalf("anonymous snapshots: %d %v", snapshots, err)
			}
		})
	}
}

func TestSnapshotStartedTopicsAllStarted(t *testing.T) {
	ctx, stream := postgresStreamStorage(t)
	frames := []Frame{publicPriceFrame(t, "1"), publicPriceFrame(t, "1")}
	frames[1].Topic = "market.price.0x3333333333333333333333333333333333333333"
	var payload map[string]any
	if err := json.Unmarshal(frames[1].Payload, &payload); err != nil {
		t.Fatal(err)
	}
	payload["baseAsset"].(map[string]any)["address"] = frames[1].Topic[len("market.price."):]
	var err error
	frames[1].Payload, err = json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	topics := []string{frames[1].Topic, frames[0].Topic}
	for _, frame := range frames {
		commitStreamFrame(t, ctx, stream, frame)
	}
	raw, err := stream.PublicSnapshot(ctx, topics)
	if err != nil || interfaces.ValidateJSON("TopicSnapshot", raw) != nil {
		t.Fatalf("snapshot: %s %v", raw, err)
	}
	var snapshot struct {
		ResumeCursor string `json:"resumeCursor"`
		Topics       []struct {
			Topic, Sequence, Revision string
			Payload                   json.RawMessage
		} `json:"topics"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil || len(snapshot.Topics) != 2 {
		t.Fatalf("snapshot entries: %s %v", raw, err)
	}
	for i, entry := range snapshot.Topics {
		frame := frames[i]
		if entry.Topic != frame.Topic || entry.Sequence != frame.Sequence || entry.Revision != frame.Revision || string(entry.Payload) != string(frame.Payload) {
			t.Fatalf("snapshot entry: %+v", entry)
		}
	}
	replay, err := stream.PublicResume(ctx, topics, snapshot.ResumeCursor)
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Close()
	if len(replay.Cursor.Offsets) != 2 || len(replay.partitions) != 2 || len(replay.waiting) != 0 {
		t.Fatalf("resume coverage: %+v %v", replay.Cursor.Offsets, replay.waiting)
	}
	for _, offset := range replay.Cursor.Offsets {
		if offset != "1" {
			t.Fatalf("snapshot offset = %s", offset)
		}
	}
	if err := replay.Check(ctx); err != nil {
		t.Fatal(err)
	}
}
