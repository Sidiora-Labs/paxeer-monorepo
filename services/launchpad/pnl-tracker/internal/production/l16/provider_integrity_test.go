package l16

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum/ethclient"
)

type inventoryRPCRequest struct {
	Method string            `json:"method"`
	Params []json.RawMessage `json:"params"`
}

func inventoryProviderVariant(t *testing.T, origin string, match func(inventoryRPCRequest) bool, transform func(map[string]any)) (*ethclient.Client, *atomic.Int64) {
	t.Helper()
	hits := new(atomic.Int64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		var request inventoryRPCRequest
		if err = json.Unmarshal(raw, &request); err != nil {
			t.Error(err)
			return
		}
		response, err := http.Post(origin, "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Error(err)
			return
		}
		defer response.Body.Close()
		var envelope map[string]any
		if err = json.NewDecoder(response.Body).Decode(&envelope); err != nil {
			t.Error(err)
			return
		}
		if match(request) {
			hits.Add(1)
			transform(envelope)
		}
		if err = json.NewEncoder(w).Encode(envelope); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	client, err := ethclient.DialContext(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client, hits
}

func TestRecordedInventoryRejectsProviderInconsistency(t *testing.T) {
	ctx := context.Background()
	service, pool := recordedRebuildService(t, ctx)
	origin := repair63RecordedRPC(t)
	rpcError := func(out map[string]any) {
		delete(out, "result")
		out["error"] = map[string]any{"code": -32000, "message": "provider unavailable"}
	}
	result := func(value any) func(map[string]any) {
		return func(out map[string]any) { out["result"] = value; delete(out, "error") }
	}
	method := func(name string) func(inventoryRPCRequest) bool {
		return func(r inventoryRPCRequest) bool { return r.Method == name }
	}
	code := func(before bool) func(inventoryRPCRequest) bool {
		return func(r inventoryRPCRequest) bool {
			return r.Method == "eth_getCode" && len(r.Params) > 1 && (len(r.Params[1]) > 0 && r.Params[1][0] == '"') == before
		}
	}
	call := func(selector string) func(inventoryRPCRequest) bool {
		return func(r inventoryRPCRequest) bool {
			return r.Method == "eth_call" && len(r.Params) > 0 && strings.Contains(string(r.Params[0]), selector)
		}
	}
	tests := []struct {
		name    string
		match   func(inventoryRPCRequest) bool
		mutate  func(map[string]any)
		capture bool
		want    error
	}{
		{"chain unavailable", method("eth_chainId"), rpcError, true, nil},
		{"wrong chain", method("eth_chainId"), result("0x1"), true, delivery.ErrIntegrity},
		{"code history unavailable", code(true), rpcError, true, nil},
		{"preexisting code", code(true), result("0x6000"), true, ErrBaselineUnavailable},
		{"creation code unavailable", code(false), rpcError, true, nil},
		{"missing creation code", code(false), result("0x"), true, ErrBaselineUnavailable},
		{"header unavailable", method("eth_getBlockByNumber"), rpcError, true, nil},
		{"missing header", method("eth_getBlockByNumber"), result(nil), true, nil},
		{"wrong header hash", method("eth_getBlockByNumber"), func(out map[string]any) { out["result"].(map[string]any)["hash"] = "0x" + strings.Repeat("e", 64) }, true, delivery.ErrIntegrity},
		{"logs unavailable", method("eth_getLogs"), rpcError, true, nil},
		{"missing logs", method("eth_getLogs"), result([]any{}), true, nil},
		{"receipt unavailable", method("eth_getTransactionReceipt"), rpcError, false, nil},
		{"receipt reverted", method("eth_getTransactionReceipt"), func(out map[string]any) { out["result"].(map[string]any)["status"] = "0x0" }, false, delivery.ErrIntegrity},
		{"receipt provenance changed", method("eth_getTransactionReceipt"), func(out map[string]any) { out["result"].(map[string]any)["gasUsed"] = "0x1" }, false, delivery.ErrIntegrity},
		{"receipt header unavailable", method("eth_getBlockByHash"), rpcError, false, nil},
		{"receipt timestamp changed", method("eth_getBlockByHash"), func(out map[string]any) { out["result"].(map[string]any)["timestamp"] = "0x1" }, false, delivery.ErrIntegrity},
		{"receipt header provenance changed", method("eth_getBlockByHash"), func(out map[string]any) { out["result"].(map[string]any)["extraData"] = "0x00" }, false, delivery.ErrIntegrity},
		{"decimals unavailable", call("313ce567"), rpcError, false, nil},
		{"decimals malformed", call("313ce567"), result("0x01"), false, delivery.ErrIntegrity},
		{"decimals overflow", call("313ce567"), result("0x" + strings.Repeat("f", 64)), false, delivery.ErrIntegrity},
		{"balance unavailable", call("70a08231"), rpcError, true, nil},
		{"balance malformed", call("70a08231"), result("0x"), true, delivery.ErrIntegrity},
		{"supply unavailable", call("18160ddd"), rpcError, true, nil},
		{"supply malformed", call("18160ddd"), result("0x"), true, delivery.ErrIntegrity},
		{"supply mismatch", call("18160ddd"), result("0x" + strings.Repeat("0", 64)), true, ErrBaselineUnavailable},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, hits := inventoryProviderVariant(t, origin, tc.match, tc.mutate)
			authority := *service.authority
			authority.rpc = client
			variant := *service
			variant.authority = &authority
			modes := []string{"ordinary", "live"}
			if tc.capture {
				modes = append(modes, "baseline")
			}
			if strings.HasPrefix(tc.name, "supply ") {
				modes = []string{"baseline"}
			}
			for _, mode := range modes {
				t.Run(mode, func(t *testing.T) {
					var err error
					var snapshot RebuildSnapshot
					switch mode {
					case "ordinary":
						_, err = variant.Rebuild(ctx, realMarketCreator, realMarketToken, 33963733)
					case "live":
						tx, e := pool.Begin(ctx)
						if e != nil {
							t.Fatal(e)
						}
						defer inventoryRollback(tx)
						snapshot, err = variant.liveRebuildTx(ctx, tx, realMarketCreator, realMarketToken, 33963733)
					case "baseline":
						_, err = authority.Capture(ctx, "0x"+strings.Repeat("e", 40), realMarketToken)
					}
					if tc.name == "logs unavailable" && mode == "live" {
						if err != nil || snapshot.Height != 33963733 || snapshot.State.Known == "0" {
							t.Fatalf("receipt recovery snapshot=%+v error=%v", snapshot, err)
						}
						return
					}
					if err == nil || (tc.want != nil && !errors.Is(err, tc.want)) {
						t.Fatalf("error=%v want=%v", err, tc.want)
					}
				})
			}
			if hits.Load() == 0 {
				t.Fatal("provider variant was not exercised")
			}
		})
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM pnl.inventory_l16_active_v2").Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

func TestBaselineAheadDistinguishesRefusalAndArchiveRecovery(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []string{"archive recovery", "source gap", "invalid supply", "integrity disagreement", "both unavailable"} {
		t.Run(mode, func(t *testing.T) {
			service, pool := recordedRebuildService(t, ctx)
			primary, archive := *service.authority, *service.authority
			archive.rpc = service.second
			origin := repair63RecordedRPC(t)
			match := func(r inventoryRPCRequest) bool { return r.Method == "eth_chainId" }
			unavailable := func(out map[string]any) {
				delete(out, "result")
				out["error"] = map[string]any{"code": -32000, "message": "provider unavailable"}
			}
			if mode == "archive recovery" || mode == "both unavailable" {
				primary.rpc, _ = inventoryProviderVariant(t, origin, match, unavailable)
			}
			if mode == "both unavailable" {
				archive.rpc, _ = inventoryProviderVariant(t, origin, match, unavailable)
			}
			if mode == "integrity disagreement" {
				mismatch := func(out map[string]any) { out["result"] = "0x1" }
				primary.rpc, _ = inventoryProviderVariant(t, origin, match, mismatch)
				archive.rpc, _ = inventoryProviderVariant(t, origin, match, mismatch)
			}
			if mode == "invalid supply" {
				primary.rpc, _ = inventoryProviderVariant(t, origin, func(r inventoryRPCRequest) bool {
					return r.Method == "eth_call" && strings.Contains(string(r.Params[0]), "18160ddd")
				}, func(out map[string]any) { out["result"] = "0x" + strings.Repeat("0", 64) })
			}
			for _, digit := range []string{"d", "e"} {
				identity, principal := inventoryIdentityFixture(t, "0x"+strings.Repeat(digit, 40))
				worker, err := NewLiveTransferWorker(service, l17.Repository{Pool: pool}, identity, 1)
				if err != nil {
					t.Fatal(err)
				}
				if err = worker.EnqueueOwned(ctx, principal); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "source gap" {
				if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_block_completeness SET fetch_complete=false`); err != nil {
					t.Fatal(err)
				}
			}
			ahead, err := NewBaselineAhead(&primary, &archive, 32)
			if err != nil {
				t.Fatal(err)
			}
			issued, err := ahead.RunOnce(ctx)
			wantIssued, wantRefused, wantError := 0, 0, false
			switch mode {
			case "archive recovery":
				wantIssued = 2
			case "invalid supply":
				wantRefused = 2
			case "integrity disagreement":
				wantRefused = 2
				wantError = true
			case "both unavailable":
				wantError = true
			}
			if issued != wantIssued || len(ahead.refused) != wantRefused || (err != nil) != wantError {
				t.Fatal(issued, len(ahead.refused), err)
			}
			if wantRefused > 0 {
				pending, err := ahead.candidates(ctx)
				if err != nil || len(pending) != 0 {
					t.Fatal(pending, err)
				}
			}
		})
	}
}
