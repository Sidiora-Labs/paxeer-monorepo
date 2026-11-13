package canonicaltest

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestCommitResponseLossPreservesDatabaseCommit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dsn, pool := New(t)
	if _, err := pool.Exec(ctx, `CREATE TABLE public.commit_observations(value text PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	proxy, lost := LostCommitResponse(t, dsn)
	cfg, err := pgx.ParseConfig(proxy)
	if err != nil {
		t.Fatal(err)
	}
	cfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.commit_observations VALUES('committed')`); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err == nil {
		t.Fatal("lost commit response acknowledged")
	}
	select {
	case <-lost:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	var value string
	if err = pool.QueryRow(ctx, `SELECT value FROM public.commit_observations`).Scan(&value); err != nil || value != "committed" {
		t.Fatalf("durable transaction %q %v", value, err)
	}
	next, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close(context.Background())
	if _, err = next.Exec(ctx, "BEGIN; SELECT 1; COMMIT"); err != nil {
		t.Fatalf("subsequent connection unavailable: %v", err)
	}
}

func TestPostgresFramesEnforceWireLengthAndCompleteness(t *testing.T) {
	for _, tc := range []struct {
		name    string
		length  uint32
		body    []byte
		invalid bool
	}{
		{"complete", 7, []byte{'O', 'K', 0}, false},
		{"empty", 4, nil, false},
		{"short length", 3, nil, true},
		{"oversize", 32<<20 + 1, nil, true},
		{"truncated", 7, []byte{'O'}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader, writer := net.Pipe()
			defer reader.Close()
			go func() {
				defer writer.Close()
				header := make([]byte, 5)
				header[0] = 'C'
				binary.BigEndian.PutUint32(header[1:], tc.length)
				_, _ = writer.Write(append(header, tc.body...))
			}()
			frame, err := postgresFrame(reader)
			if tc.invalid {
				if err == nil {
					t.Fatal("invalid frame accepted")
				}
				return
			}
			if err != nil || len(frame) != int(tc.length)+1 || frame[0] != 'C' {
				t.Fatalf("frame %x %v", frame, err)
			}
		})
	}
	reader, writer := net.Pipe()
	writer.Close()
	defer reader.Close()
	if _, err := postgresFrame(reader); !errors.Is(err, io.EOF) {
		t.Fatalf("closed wire %v", err)
	}
}
