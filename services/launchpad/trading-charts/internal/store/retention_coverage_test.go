package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
)

func TestRetentionGuardsAndReplayHold(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	for _, schema := range []func(string) (string, error){ExactSchemaDDL, SeriesSchemaDDL, RebuildSchemaDDL} {
		ddl, err := schema("candles")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	exact, err := NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	address := "0x" + strings.Repeat("a", 40)
	now := time.Now()
	minuteFloor, err := RetentionFloor(now, "1m")
	if err != nil || !minuteFloor.Before(now.AddDate(0, 0, -364)) {
		t.Fatalf("minute retention floor: %s, %v", minuteFloor, err)
	}
	weeklyFloor, err := RetentionFloor(now, "1w")
	if err != nil || !weeklyFloor.Before(now.AddDate(-4, 0, 0)) {
		t.Fatalf("weekly retention floor: %s, %v", weeklyFloor, err)
	}
	if _, err := RetentionFloor(now, "invalid"); err == nil {
		t.Fatal("unsupported retention interval was accepted")
	}
	if err := exact.HoldReplayRange(ctx, "", address, 0, 1, time.Minute); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("empty reference identity: %v", err)
	}
	if err := exact.HoldReplayRange(ctx, "reference", address, 0, 60, time.Minute); err != nil {
		t.Fatalf("hold replay range: %v", err)
	}
	var held int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_replay_references WHERE reference_id='reference' AND pool=$1`, address).Scan(&held); err != nil || held != 1 {
		t.Fatalf("replay hold: %d, %v", held, err)
	}
	if err := exact.HoldReplayRange(ctx, "reference", address, 0, 60, 2*time.Minute); err != nil {
		t.Fatalf("renew replay range: %v", err)
	}
	if _, err := exact.DeleteRetainedRange(ctx, address, "1m", 0, 60); err == nil || !strings.Contains(err.Error(), "retention_archive_or_reference_guard") {
		t.Fatalf("delete without archive checkpoint: %v", err)
	}
	if err := exact.RecordRecoverabilityIncident(ctx, address, "retention review", strings.Repeat("b", 64)); err != nil {
		t.Fatalf("record recoverability incident: %v", err)
	}
	var incidents int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_recoverability_incidents WHERE pool=$1`, address).Scan(&incidents); err != nil || incidents < 2 {
		t.Fatalf("recoverability incidents: %d, %v", incidents, err)
	}
	retained := "0x" + strings.Repeat("b", 40)
	if _, err := pool.Exec(ctx, `INSERT INTO candles.chart_archive_checkpoints(pool,from_time,to_time,manifest_digest) VALUES($1,0,120,$2)`, retained, strings.Repeat("c", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO candles.chart_exact(chain_id,pool,interval,start,snapshot) VALUES('125',$1,'1m',60,$2)`, retained, `{"interval":"1m","start":60,"revision":"1"}`); err != nil {
		t.Fatal(err)
	}
	removed, err := exact.DeleteRetainedRange(ctx, retained, "1m", 0, 120)
	if err != nil || removed != 1 {
		t.Fatalf("checkpointed retained candle deletion: %d, %v", removed, err)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_exact WHERE pool=$1`, retained).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("retained range remains: %d, %v", remaining, err)
	}
}
