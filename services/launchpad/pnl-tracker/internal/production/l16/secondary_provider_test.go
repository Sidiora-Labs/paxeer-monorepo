package l16

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestInventoryRequiresIndependentArchiveAgreement(t *testing.T) {
	ctx := context.Background()
	service, pool := recordedRebuildService(t, ctx)
	origin := repair63RecordedRPC(t)
	for _, tc := range []struct {
		name, method string
		result       any
		unavailable  bool
		want         error
	}{
		{name: "chain unavailable", method: "eth_chainId", unavailable: true},
		{name: "chain identity", method: "eth_chainId", result: "0x1", want: delivery.ErrIntegrity},
		{name: "header unavailable", method: "eth_getBlockByNumber", unavailable: true},
		{name: "balance unavailable", method: "eth_call", unavailable: true},
		{name: "balance shape", method: "eth_call", result: "0x01", want: delivery.ErrIntegrity},
		{name: "balance disagreement", method: "eth_call", result: "0x" + strings.Repeat("0", 64), want: ErrParity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, hits := inventoryProviderVariant(t, origin, func(r inventoryRPCRequest) bool { return r.Method == tc.method }, func(out map[string]any) {
				if tc.unavailable {
					delete(out, "result")
					out["error"] = map[string]any{"code": -32000, "message": "archive unavailable"}
				} else {
					delete(out, "error")
					out["result"] = tc.result
				}
			})
			candidate := *service
			candidate.second = client
			for _, live := range []bool{false, true} {
				t.Run(map[bool]string{false: "ordinary", true: "live"}[live], func(t *testing.T) {
					var err error
					if live {
						tx, e := pool.Begin(ctx)
						if e != nil {
							t.Fatal(e)
						}
						defer inventoryRollback(tx)
						_, err = candidate.liveRebuildTx(ctx, tx, realMarketCreator, realMarketToken, 33963733)
					} else {
						_, err = candidate.Rebuild(ctx, realMarketCreator, realMarketToken, 33963733)
					}
					if err == nil || tc.want != nil && !errors.Is(err, tc.want) {
						t.Fatal(err, tc.want)
					}
				})
			}
			if hits.Load() != 2 {
				t.Fatal(hits.Load())
			}
		})
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pnl.inventory_l16_active_v2`).Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}
