package l16

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestQuarantineCaptureValidatesEvidenceAndCancellation(t *testing.T) {
	ctx := context.Background()
	s, trace := inventoryCancelableService(t)
	pool := s.authority.pool
	worker := &LiveTransferWorker{service: s}
	for i, tc := range []struct {
		name, payload, block string
		count                int
		want                 error
	}{
		{name: "canonical evidence", payload: `{"reason":"parity"}`, block: "0x" + strings.Repeat("a", 64), count: 1, want: ErrParity},
		{name: "block binding", payload: `{}`, block: "invalid", count: 1, want: delivery.ErrIntegrity},
		{name: "payload schema", payload: `{`, block: "0x" + strings.Repeat("a", 64), count: 1, want: delivery.ErrIntegrity},
		{name: "capture bound", payload: `{}`, block: "0x" + strings.Repeat("a", 64), count: 1001},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wallet := fmt.Sprintf("0x%040x", i+100)
			partition := "125:" + wallet + ":" + realMarketToken
			if _, err := pool.Exec(ctx, `INSERT INTO pnl.inventory_l16_quarantine_v2(generation,partition,height,block_hash,reason,payload) SELECT 'g1',$1,n,$2,'dual_node_or_inventory_parity',$3 FROM generate_series(1,$4::integer) n`, partition, tc.block, []byte(tc.payload), tc.count); err != nil {
				t.Fatal(err)
			}
			input := InventoryMeasurementCase{ID: "canonical-quarantine", Wallet: wallet, Token: realMarketToken, Height: int64(tc.count), ExpectedError: expectedParity}
			raw, err := captureInventoryQuarantine(ctx, worker, input)
			if err == nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatal(string(raw), err, tc.want)
			}
			if tc.name == "capture bound" && !strings.Contains(err.Error(), "capture bound") {
				t.Fatal(err)
			}
			if tc.name == "canonical evidence" {
				var result struct {
					Quarantine []struct {
						Payload json.RawMessage
						SHA256  string
					}
					ReleaseQualified bool
				}
				if err := json.Unmarshal(raw, &result); err != nil || len(result.Quarantine) != 1 || result.Quarantine[0].SHA256 != delivery.Hash(result.Quarantine[0].Payload) || result.ReleaseQualified {
					t.Fatal(string(raw), err)
				}
				trace.Start(0)
				_, err = captureInventoryQuarantine(ctx, worker, input)
				queries := trace.Stop()
				if !errors.Is(err, ErrParity) || len(queries) == 0 {
					t.Fatal(queries, err)
				}
				for j, query := range queries {
					if strings.EqualFold(query, "rollback") {
						continue
					}
					t.Run(fmt.Sprintf("cancel_%02d", j+1), func(t *testing.T) {
						trace.Start(j + 1)
						_, err := captureInventoryQuarantine(ctx, worker, input)
						observed := trace.Stop()
						if !errors.Is(err, context.Canceled) || len(observed) != j+1 || observed[j] != query {
							t.Fatal(observed, query, err)
						}
					})
				}
			}
		})
	}
}
