package broker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPublicResumeRejectsMalformedAndForeignCursors(t *testing.T) {
	ctx, stream := postgresStreamStorage(t)
	first := publicPriceFrame(t, "1")
	commitStreamFrame(t, ctx, stream, first)
	valid := snapshotCursor(t, ctx, stream, []string{first.Topic})
	for _, check := range []struct {
		name, cursor string
	}{
		{"empty", ""},
		{"unframed", "invalid"},
		{"bad base64", "!.signature"},
		{"oversized", strings.Repeat("x", 2049)},
		{"changed signature", valid[:len(valid)-1] + "!"},
	} {
		t.Run(check.name, func(t *testing.T) {
			if replay, err := stream.PublicResume(ctx, []string{first.Topic}, check.cursor); !errors.Is(err, ErrCursor) || replay != nil {
				t.Fatalf("cursor %q accepted: replay=%v error=%v", check.name, replay, err)
			}
		})
	}
	if _, err := stream.pool.Exec(ctx, "UPDATE "+stream.table("stream_generation")+" SET generation=gen_random_uuid() WHERE singleton"); err != nil {
		t.Fatal(err)
	}
	if replay, err := stream.PublicResume(ctx, []string{first.Topic}, valid); !errors.Is(err, ErrCursor) || replay != nil {
		t.Fatalf("previous generation cursor accepted: replay=%v error=%v", replay, err)
	}
}

func TestCommittedReplayDetectsChangedOutboxAndMissingSequence(t *testing.T) {
	for _, check := range []struct {
		name   string
		change func(context.Context, *PostgresStream) error
		want   error
	}{
		{"missing sequence", func(ctx context.Context, stream *PostgresStream) error {
			_, err := stream.pool.Exec(ctx, "DELETE FROM "+stream.table("stream_delivery_outbox")+" WHERE sequence=2")
			return err
		}, ErrSequence},
		{"changed frame", func(ctx context.Context, stream *PostgresStream) error {
			if _, err := stream.pool.Exec(ctx, "ALTER TABLE "+stream.table("stream_delivery_outbox")+" DISABLE TRIGGER stream_delivery_outbox_immutable"); err != nil {
				return err
			}
			_, err := stream.pool.Exec(ctx, "UPDATE "+stream.table("stream_delivery_outbox")+" SET frame=$1 WHERE sequence=2", []byte(`{}`))
			return err
		}, ErrFrame},
	} {
		t.Run(check.name, func(t *testing.T) {
			ctx, stream := postgresStreamStorage(t)
			first := publicPriceFrame(t, "1")
			commitStreamFrame(t, ctx, stream, first)
			cursor := snapshotCursor(t, ctx, stream, []string{first.Topic})
			for _, sequence := range []string{"2", "3"} {
				commitStreamFrame(t, ctx, stream, publicPriceFrame(t, sequence))
			}
			replay, err := stream.PublicResume(ctx, []string{first.Topic}, cursor)
			if err != nil {
				t.Fatal(err)
			}
			defer replay.Close()
			if err := check.change(ctx, stream); err != nil {
				t.Fatal(err)
			}
			frames, err := replay.Batch(ctx)
			if !errors.Is(err, check.want) || len(frames) != 0 {
				t.Fatalf("changed outbox: frames=%d error=%v, want %v", len(frames), err, check.want)
			}
			if check.want == ErrSequence && replay.Machine.State != ResyncRequired {
				t.Fatalf("missing sequence left replay state %v", replay.Machine.State)
			}
		})
	}
}

func TestCommittedReplayCheckpointRequiresLiveState(t *testing.T) {
	ctx, stream := postgresStreamStorage(t)
	first := publicPriceFrame(t, "1")
	commitStreamFrame(t, ctx, stream, first)
	cursor := snapshotCursor(t, ctx, stream, []string{first.Topic})
	replay, err := stream.PublicResume(ctx, []string{first.Topic}, cursor)
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Close()
	if token, err := replay.Checkpoint(ctx); !errors.Is(err, ErrCursor) || token != "" {
		t.Fatalf("checkpoint before replay: %q, %v", token, err)
	}
	frames, err := replay.Batch(ctx)
	if err != nil || len(frames) != 0 || replay.Machine.State != Live {
		t.Fatalf("empty replay barrier: state=%v frames=%d error=%v", replay.Machine.State, len(frames), err)
	}
	token, err := replay.Checkpoint(ctx)
	if err != nil || token == "" {
		t.Fatalf("live checkpoint: %q, %v", token, err)
	}
	checkpoint, err := VerifyCursorV2(token, stream.key, replay.Cursor.Scope, time.Now())
	if err != nil || len(checkpoint.Offsets) != 1 {
		t.Fatalf("live checkpoint offsets: %+v, %v", checkpoint.Offsets, err)
	}
}
