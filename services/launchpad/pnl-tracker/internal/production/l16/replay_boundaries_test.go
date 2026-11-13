package l16

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestLiveTransferRangeSplitsAndReceiptRecovery(t *testing.T) {
	ctx := context.Background()
	origin := repair63RecordedRPC(t)
	reference, err := ethclient.DialContext(ctx, origin)
	if err != nil {
		t.Fatal(err)
	}
	defer reference.Close()
	authority := &BaselineAuthority{rpc: reference}
	expected, err := authority.liveTransferLogs(ctx, realMarketToken, 33963716, 33963733)
	if err != nil || len(expected) == 0 {
		t.Fatal(expected, err)
	}
	for _, receipts := range []bool{false, true} {
		t.Run(map[bool]string{false: "split ranges", true: "receipt recovery"}[receipts], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
					return
				}
				var request struct {
					ID     json.RawMessage `json:"id"`
					Method string          `json:"method"`
					Params []struct {
						From string `json:"fromBlock"`
						To   string `json:"toBlock"`
					} `json:"params"`
				}
				// Only log queries carry object parameters; other calls retain their recorded bytes.
				var method struct {
					Method string `json:"method"`
				}
				if err = json.Unmarshal(raw, &method); err != nil {
					t.Error(err)
					return
				}
				if method.Method == "eth_getLogs" {
					if err = json.Unmarshal(raw, &request); err != nil {
						t.Error(err)
						return
					}
					if receipts || request.Params[0].From != request.Params[0].To {
						if err = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "error": map[string]any{"code": -32005, "message": "query exceeds provider result limit"}}); err != nil {
							t.Error(err)
						}
						return
					}
				}
				response, err := http.Post(origin, "application/json", bytes.NewReader(raw))
				if err != nil {
					t.Error(err)
					return
				}
				defer response.Body.Close()
				if _, err = io.Copy(w, response.Body); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			client, err := ethclient.DialContext(ctx, server.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			actual, err := (&BaselineAuthority{rpc: client}).liveTransferLogs(ctx, realMarketToken, 33963716, 33963733)
			if err != nil || !reflect.DeepEqual(actual, expected) {
				t.Fatalf("logs=%d want=%d error=%v", len(actual), len(expected), err)
			}
		})
	}
}

func TestCheckpointRejectsInvalidArchiveFrontiers(t *testing.T) {
	ctx := context.Background()
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	leg := testLeg(1)
	leg.Payer = leg.Wallet
	first, err := RebuildDay(ctx, Empty(), []TimedLeg{{leg, day}}, day, true)
	if err != nil || first.State.Known != "10" {
		t.Fatal(first, err)
	}
	for _, change := range []func(*Checkpoint){
		func(c *Checkpoint) { c.InputDigest = strings.Repeat("z", 64) },
		func(c *Checkpoint) { c.State.Known = "bad" },
		func(c *Checkpoint) { c.Partition = "bad" },
		func(c *Checkpoint) { c.Partition = "" },
		func(c *Checkpoint) { c.LastHeight = 0 },
		func(c *Checkpoint) { c.LastIdentity = "bad" },
	} {
		bad := first
		change(&bad)
		if _, err := ContinueDay(ctx, bad, nil, day.AddDate(0, 0, 1)); err == nil {
			t.Fatal("invalid continuation accepted", bad)
		}
		if _, err := RebuildDayPages(ctx, bad, nil, day.AddDate(0, 0, 1)); err == nil {
			t.Fatal("invalid paged continuation accepted", bad)
		}
	}
	for _, change := range []func(*TimedLeg){
		func(l *TimedLeg) { l.Leg.Identity.Chain = "bad" },
		func(l *TimedLeg) { l.Leg.Quantity = "bad" },
		func(l *TimedLeg) { l.Leg.Predecessor = "missing" },
	} {
		bad := TimedLeg{leg, day}
		change(&bad)
		if _, err := RebuildDay(ctx, Empty(), []TimedLeg{bad}, day, true); err == nil {
			t.Fatal("invalid archive accepted", bad)
		}
		if _, err := RebuildDayPages(ctx, Checkpoint{}, []TimedLeg{bad}, day); err == nil {
			t.Fatal("invalid paged archive accepted", bad)
		}
	}
	conflicting := leg
	conflicting.Quantity = "11"
	for _, legs := range [][]TimedLeg{{{leg, day}, {conflicting, day}}, {{leg, day}, {leg, day}}} {
		cp, err := RebuildDay(ctx, Empty(), legs, day, true)
		if legs[1].Leg.Quantity == "11" {
			if err == nil {
				t.Fatal("conflicting identity accepted")
			}
		} else if err != nil || cp.State.Known != "10" {
			t.Fatal(cp, err)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := RebuildDay(canceled, Empty(), []TimedLeg{{leg, day}}, day, true); err != context.Canceled {
		t.Fatal(err)
	}
	if _, err := RebuildDayPages(canceled, Checkpoint{}, []TimedLeg{{leg, day}}, day); err != context.Canceled {
		t.Fatal(err)
	}
}

func inventoryDisagreeingNode(t *testing.T) *ethclient.Client {
	t.Helper()
	origin := repair63RecordedRPC(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err = json.Unmarshal(raw, &request); err != nil {
			t.Error(err)
			return
		}
		if request.Method == "eth_call" {
			if err = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": "0x" + strings.Repeat("0", 64)}); err != nil {
				t.Error(err)
			}
			return
		}
		response, err := http.Post(origin, "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Error(err)
			return
		}
		defer response.Body.Close()
		if _, err = io.Copy(w, response.Body); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	client, err := ethclient.DialContext(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client
}

func TestInventoryQuarantinesCommittedNodeDisagreement(t *testing.T) {
	for _, live := range []bool{false, true} {
		t.Run(map[bool]string{false: "archive", true: "enrolled wallet"}[live], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			service, pool := recordedRebuildService(t, ctx)
			service.second = inventoryDisagreeingNode(t)
			if live {
				inventoryCommittedHead(t, ctx, pool)
				identity, principal := inventoryIdentityFixture(t, realMarketCreator)
				worker, err := NewLiveTransferWorker(service, l17.Repository{Pool: pool}, identity, 1)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = worker.RebuildOwned(ctx, principal); !errors.Is(err, ErrParity) {
					t.Fatal(err)
				}
				if _, err = worker.processWallet(ctx, principal.Wallet); !errors.Is(err, ErrParity) {
					t.Fatal(err)
				}
			} else {
				snapshot, err := service.Rebuild(ctx, realMarketCreator, realMarketToken, 33963733)
				if !errors.Is(err, ErrParity) || snapshot.NodeBalances[0] == snapshot.NodeBalances[1] {
					t.Fatal(snapshot, err)
				}
			}
			if _, err := service.RebuildArchive(ctx, realMarketCreator, realMarketToken, 33963749); !errors.Is(err, ErrParity) {
				t.Fatal(err)
			}
			var count int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM pnl.inventory_l16_quarantine_v2`).Scan(&count); err != nil || count != 1 {
				t.Fatal(count, err)
			}
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM pnl.inventory_l16_active_v2`).Scan(&count); err != nil || count != 0 {
				t.Fatal(count, err)
			}
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM pnl.inventory_l16_live_commits_v1`).Scan(&count); err != nil || count != 0 {
				t.Fatal(count, err)
			}
		})
	}
}
