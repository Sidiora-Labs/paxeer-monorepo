package broker

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestPostgresReplayKeepsNumericSequenceAcrossResume(t *testing.T) {
	ctx, stream := postgresStreamStorage(t)
	first := publicPriceFrame(t, "1")
	commitStreamFrame(t, ctx, stream, first)
	token := snapshotCursor(t, ctx, stream, []string{first.Topic})
	for sequence := 2; sequence <= 12; sequence++ {
		commitStreamFrame(t, ctx, stream, publicPriceFrame(t, fmt.Sprint(sequence)))
	}
	replay, err := stream.PublicResume(ctx, []string{first.Topic}, token)
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Close()
	written := replay.WrittenCheckpoints()
	batch, err := replay.Batch(ctx)
	if err != nil || len(batch) != 11 {
		t.Fatalf("numeric replay: frames=%d error=%v", len(batch), err)
	}
	for i, raw := range batch {
		var frame Frame
		if err := json.Unmarshal(raw, &frame); err != nil || frame.Sequence != fmt.Sprint(i+2) {
			t.Fatalf("frame %d: sequence=%s error=%v", i, frame.Sequence, err)
		}
	}
	token, err = written.Advance(batch[0])
	if err != nil {
		t.Fatal(err)
	}
	replay.Close()
	resumed, err := stream.PublicResume(ctx, []string{first.Topic}, token)
	if err != nil {
		t.Fatal(err)
	}
	defer resumed.Close()
	batch, err = resumed.Batch(ctx)
	if err != nil || len(batch) != 10 || resumed.Machine.State != Live {
		t.Fatalf("resumed numeric replay: state=%v frames=%d error=%v", resumed.Machine.State, len(batch), err)
	}
	for i, raw := range batch {
		var frame Frame
		if err := json.Unmarshal(raw, &frame); err != nil || frame.Sequence != fmt.Sprint(i+3) {
			t.Fatalf("resumed frame %d: sequence=%s error=%v", i, frame.Sequence, err)
		}
	}
}
