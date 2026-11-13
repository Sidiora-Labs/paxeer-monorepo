package l16

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestInventoryProviderReadAvailability(t *testing.T) {
	ctx := context.Background()
	origin := repair63RecordedRPC(t)
	for _, action := range []string{"ordinary", "archive", "live", "baseline", "receipt", "live receipt", "mark"} {
		t.Run(action, func(t *testing.T) {
			service, pool := recordedRebuildService(t, ctx)
			var raw []byte
			if err := pool.QueryRow(ctx, `SELECT payload FROM indexer.indexer_accounting_fills_v1 ORDER BY log_index LIMIT 1`).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			fill, err := accounting.DecodeFillV1(raw)
			if err != nil {
				t.Fatal(err)
			}
			var mu sync.Mutex
			target := 0
			methods := []string{}
			client, hits := inventoryProviderVariant(t, origin, func(r inventoryRPCRequest) bool {
				mu.Lock()
				defer mu.Unlock()
				methods = append(methods, r.Method)
				return target > 0 && len(methods) == target
			}, func(out map[string]any) {
				delete(out, "result")
				out["error"] = map[string]any{"code": -32000, "message": "provider unavailable"}
			})
			authority := *service.authority
			authority.rpc = client
			variant := *service
			variant.authority = &authority
			operation := func() error {
				switch action {
				case "ordinary", "archive":
					snapshot, err := variant.rebuild(ctx, realMarketCreator, realMarketToken, 33963733, action == "archive")
					if err == nil && (snapshot.Height != 33963733 || snapshot.State.Known == "0") {
						t.Fatal(snapshot)
					}
					return err
				case "baseline":
					digest, err := authority.Capture(ctx, "0x"+strings.Repeat("e", 40), realMarketToken)
					if err == nil && len(digest) != 64 {
						t.Fatal(digest)
					}
					return err
				default:
					tx, err := pool.Begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					defer inventoryRollback(tx)
					switch action {
					case "live":
						snapshot, err := variant.liveRebuildTx(ctx, tx, realMarketCreator, realMarketToken, 33963733)
						if err == nil && (snapshot.Height != 33963733 || snapshot.State.Known == "0") {
							t.Fatal(snapshot)
						}
						return err
					case "mark":
						price, err := authority.markPriceTx(ctx, tx, realMarketToken, 33963733, fill.Position.BlockHash)
						if err == nil && price == nil {
							t.Fatal("missing canonical price")
						}
						return err
					default:
						var settlement receiptSettlement
						if action == "receipt" {
							settlement, err = authority.receiptSettlement(ctx, tx, fill)
						} else {
							settlement, err = authority.liveReceiptSettlement(ctx, tx, fill)
						}
						if err == nil && settlement.fill.SourceFillID != fill.SourceFillID {
							t.Fatal(settlement)
						}
						return err
					}
				}
			}
			if err := operation(); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			queries := append([]string(nil), methods...)
			mu.Unlock()
			if len(queries) == 0 {
				t.Fatal("missing provider reads")
			}
			for i, method := range queries {
				if method == "eth_getLogs" {
					continue
				}
				t.Run(fmt.Sprintf("%03d_%s", i+1, method), func(t *testing.T) {
					before := hits.Load()
					mu.Lock()
					target = i + 1
					methods = nil
					mu.Unlock()
					err := operation()
					mu.Lock()
					observed := append([]string(nil), methods...)
					mu.Unlock()
					if err == nil || hits.Load() != before+1 || len(observed) != i+1 || observed[i] != method {
						t.Fatal(observed, method, err)
					}
				})
			}
		})
	}
}
