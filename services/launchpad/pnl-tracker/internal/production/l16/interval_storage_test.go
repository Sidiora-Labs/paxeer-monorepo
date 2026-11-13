package l16

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/google/uuid"
)

func TestIntervalHistoryReadsBoundDailyBuckets(t *testing.T) {
	ctx := context.Background()
	service, pool := recordedRebuildService(t, ctx)
	start := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	for i, tc := range []struct {
		name               string
		watermark, payload string
		outside, release   bool
		want               error
		valid              bool
	}{
		{name: "daily bucket", valid: true},
		{name: "uncovered range", outside: true, want: ErrBaselineUnavailable},
		{name: "release binding", release: true, want: ErrBaselineUnavailable},
		{name: "watermark schema", watermark: `"invalid"`},
		{name: "interval schema", payload: `"invalid"`},
		{name: "bucket binding", payload: `{"start":"2026-10-11T00:00:00Z","end":"2026-10-12T00:00:00Z"}`, want: delivery.ErrIntegrity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wallet := fmt.Sprintf("0x%040x", i+100)
			snapshot := uuid.NewString()
			release := service.release
			if tc.release {
				release = strings.Repeat("f", 64)
			}
			watermark := tc.watermark
			if watermark == "" {
				watermark = `{"chainId":"125","height":"100"}`
			}
			if _, err := pool.Exec(ctx, `INSERT INTO pnl.inventory_l16_wallet_ranges_v3 VALUES($1,$2,'g1',$3,$4,$5,$6,$7)`, snapshot, wallet, start, start.Add(48*time.Hour), release, strings.Repeat("a", 64), watermark); err != nil {
				t.Fatal(err)
			}
			payload := tc.payload
			if payload == "" {
				gain := "3"
				raw, err := json.Marshal(IntervalPnL{Start: start, End: start.Add(24 * time.Hour), GrossProceedsUsdlRaw: "10", KnownAllocatedBasisUsdlRaw: "7", KnownNetGainUsdlRaw: "3", UnknownProceedsUsdlRaw: "0", RealizedPnlUsdlRaw: &gain, Fees: []IntervalFee{}, GasPaxRaw: "1", AccountingComplete: true})
				if err != nil {
					t.Fatal(err)
				}
				payload = string(raw)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO pnl.inventory_l16_wallet_intervals_v3 VALUES($1,'1d',$2,$3)`, snapshot, start, payload); err != nil {
				t.Fatal(err)
			}
			from := start
			if tc.outside {
				from = from.Add(-24 * time.Hour)
			}
			result, err := service.IntervalHistory(ctx, wallet, from, start.Add(48*time.Hour), "1d", 3)
			if tc.valid {
				if err != nil || result.SnapshotID != snapshot || len(result.Entries) != 2 || result.Entries[0].KnownNetGainUsdlRaw != "3" || result.Entries[1].RealizedPnlUsdlRaw == nil || *result.Entries[1].RealizedPnlUsdlRaw != "0" || !result.Entries[1].AccountingComplete {
					t.Fatal(result, err)
				}
			} else if err == nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatal(result, err, tc.want)
			}
		})
	}
}
