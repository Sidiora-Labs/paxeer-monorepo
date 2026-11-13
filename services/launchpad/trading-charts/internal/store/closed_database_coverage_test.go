package store

import (
	"context"
	"crypto/sha256"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/jackc/pgx/v5"
)

func TestClosedOwnedDatabaseRefusesDurableChartOperations(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	exact, err := NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	address := "0x" + strings.Repeat("a", 40)
	digest := strings.Repeat("b", 64)
	claimDigest := sha256.Sum256([]byte(`{}`))
	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"retained range", func() error { _, err := exact.DeleteRetainedRange(ctx, address, "1m", 0, 60); return err }},
		{"replay hold", func() error { return exact.HoldReplayRange(ctx, "owned", address, 0, 60, time.Minute) }},
		{"recoverability incident", func() error { return exact.RecordRecoverabilityIncident(ctx, address, "archive_unavailable", digest) }},
		{"series snapshots", func() error { _, _, _, err := exact.SeriesSnapshots(ctx, address, "1m", 0, 60, 1); return err }},
		{"delivery attempt", func() error {
			return exact.RecordAttempt(ctx, "charts", "local", "publication", digest, 1, "received", false)
		}},
		{"reprocess authority", func() error {
			return exact.AuthorizeReprocess(ctx, "charts", "reprocess_local", "publication", digest, strings.Repeat("c", 64), "decoder-v2", "operator")
		}},
		{"delivery result", func() error { return exact.RecordAttemptResult(ctx, "charts", "local", "publication", "retry", false) }},
		{"retry backoff", func() error { return exact.WaitRetryBackoff(ctx, "charts", "local", "publication") }},
		{"reprocess check", func() error { return exact.CheckReprocess(ctx, "charts", "reprocess_local", "publication", digest) }},
		{"staged reconstruction", func() error {
			return exact.StageRebuild(ctx, "owned", address, 0, 60, digest, digest, digest, 1, nil, nil, nil)
		}},
		{"reconstruction promotion", func() error { _, err := exact.PromoteRebuild(ctx, "owned"); return err }},
		{"canonical claim", func() error {
			return exact.Transaction(ctx, Claim{ChainID: "125", Pool: address, Emitter: address, CodeEpoch: "epoch-1", TxHash: "0xowned", LogIndex: "0", Epoch: 1, Digest: claimDigest[:], Fill: []byte(`{}`)}, func(pgx.Tx, string) error { return nil })
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil || !strings.Contains(err.Error(), "closed pool") {
				t.Fatalf("closed database operation: %v", err)
			}
		})
	}
}
