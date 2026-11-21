package rest_test

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestCoinTextPoolPageAndDetails(t *testing.T) {
	f := newPoolReadsFixture(t, "")
	cases := []struct {
		label, pool, token, name, symbol, wantName, wantSymbol string
		metadata                                               bool
	}{
		{"ordinary", f.first, "0x" + strings.Repeat("3", 40), "Sidiora Fixture", "SIDF", "Sidiora Fixture", "SIDF", true},
		{"without metadata", poolReadsSecond, "0x" + strings.Repeat("4", 40), "", "", "", "", false},
		{"long symbol", "0x" + strings.Repeat("71", 20), "0x" + strings.Repeat("81", 20), "Symbol coin", strings.Repeat("S", 17), "Symbol coin", strings.Repeat("S", 16), true},
		{"long name", "0x" + strings.Repeat("72", 20), "0x" + strings.Repeat("82", 20), strings.Repeat("N", 101), "NAME", strings.Repeat("N", 100), "NAME", true},
		{"empty text", "0x" + strings.Repeat("73", 20), "0x" + strings.Repeat("83", 20), "", "", "", "", true},
		{"multibyte", "0x" + strings.Repeat("74", 20), "0x" + strings.Repeat("84", 20), strings.Repeat("界", 99) + "🪙x", strings.Repeat("é", 15) + "🚀x", strings.Repeat("界", 99) + "🪙", strings.Repeat("é", 15) + "🚀", true},
	}
	values, err := f.model.PublicProjections(t.Context(), []string{"market.stats." + f.first})
	if err != nil || len(values) != 1 {
		t.Fatalf("pool stats: %v %v", values, err)
	}
	var stats map[string]json.RawMessage
	if err = json.Unmarshal(values[0].Payload, &stats); err != nil {
		t.Fatal(err)
	}
	pools := make([]string, 0, len(cases))
	for i, tc := range cases {
		pools = append(pools, tc.pool)
		if i >= 2 {
			insertRegistryPool(t, f.pool, tc.pool, tc.token, int64(30+i), tc.metadata)
			insertPoolState(t, f.pool, tc.pool, tc.token)
			insertQualifiedFill(t, f.pool, tc.pool, 1, 100)
			text, marshalErr := json.Marshal(map[string]string{"name": tc.name, "symbol": tc.symbol})
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			if _, err = f.pool.Exec(t.Context(), `UPDATE indexer.indexer_canonical_creations SET payload=jsonb_set(payload,'{metadata}',(payload->'metadata') || $1::jsonb) WHERE pool=$2`, text, tc.pool); err != nil {
				t.Fatal(err)
			}
		}
		if i == 0 {
			continue
		}
		stats["poolAddress"], err = json.Marshal(tc.pool)
		if err != nil {
			t.Fatal(err)
		}
		payload, marshalErr := json.Marshal(stats)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		tx, beginErr := f.pool.Begin(t.Context())
		if beginErr != nil {
			t.Fatal(beginErr)
		}
		frame := broker.Frame{EventID: "stats_" + tc.pool, Topic: "market.stats." + tc.pool, Sequence: "1", Revision: "1", OccurredAt: "2026-10-12T00:00:00.000Z", Watermark: stats["watermark"], Payload: payload}
		if err = f.stream.AppendProjectionTx(t.Context(), tx, frame); err != nil {
			_ = tx.Rollback(t.Context())
			t.Fatal(err)
		}
		if err = tx.Commit(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	appendRankingFrame(t, f.pool, f.stream, "volume", uuid.NewString(), pools)
	page := assertV2Success(t, getV2(t, f.srv.URL+"/v2/pools?sort=volume"), "PoolListResponse")
	items := page["data"].(map[string]any)["items"].([]any)
	if len(items) != len(cases) || page["nextCursor"] != nil {
		t.Fatalf("pool page: %v", page)
	}
	for i, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			item := items[i].(map[string]any)
			if item["poolAddress"] != tc.pool || item["tokenAddress"] != tc.token {
				t.Fatalf("pool list item: %v", item)
			}
			detail := assertV2Success(t, getV2(t, f.srv.URL+"/v2/pools/"+tc.pool), "PoolDetailResponse")
			data := detail["data"].(map[string]any)
			token := data["token"].(map[string]any)
			if data["pool"].(map[string]any)["poolAddress"] != tc.pool || data["stats"].(map[string]any)["poolAddress"] != tc.pool || token["address"] != tc.token || token["name"] != tc.wantName || token["symbol"] != tc.wantSymbol {
				t.Fatalf("pool detail: %v", data)
			}
			if tc.metadata {
				if token["decimalsQuality"] != "verified" || token["supplyQuality"] != "verified" || token["sourcePosition"] == nil {
					t.Fatalf("token metadata quality: %v", token)
				}
			} else if token["decimalsQuality"] != "unavailable" || token["supplyQuality"] != "unavailable" || token["decimals"] != nil || token["totalSupplyRaw"] != nil || token["sourcePosition"] != nil {
				t.Fatalf("token without metadata: %v", token)
			}
		})
	}
}

func TestCoinTextSearchRoutes(t *testing.T) {
	f := newPoolReadsFixture(t, "")
	if _, err := f.pool.Exec(t.Context(), `CREATE SCHEMA metadata; CREATE TABLE metadata.token_metadata(token_address text PRIMARY KEY, name text, symbol text)`); err != nil {
		t.Fatal(err)
	}
	name := strings.Repeat("界", 100)
	symbol := strings.Repeat("🪙", 16)
	if _, err := f.pool.Exec(t.Context(), `INSERT INTO metadata.token_metadata(token_address,name,symbol) VALUES($1,$2,$3),($4,'',$3)`, "0x"+strings.Repeat("3", 40), name+"x", symbol+"x", "0x"+strings.Repeat("4", 40)); err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	if err := rest.RegisterDevelopmentHistorySearchDiscovery(t.Context(), router, rest.DevelopmentHistoryOptions{Store: store.New(f.pool, f.rdb)}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	for _, kind := range []string{"pool", "token"} {
		for _, expanded := range []bool{false, true} {
			path, contract := "/v2/search", "SearchResponse"
			if expanded {
				path, contract = path+"/expanded", "ConnectedExpandedSearchResponse"
			}
			for _, tc := range []struct{ pool, want string }{{f.first, name}, {poolReadsSecond, symbol}} {
				page := assertV2Success(t, getV2(t, srv.URL+path+"?q="+tc.pool+"&type="+kind), contract)
				items := page["data"].(map[string]any)["items"].([]any)
				if len(items) != 1 {
					t.Fatalf("search items: %v", items)
				}
				item := items[0].(map[string]any)
				if item["label"] != tc.want || item["poolAddress"] != tc.pool || (expanded && item["ticker"] != symbol) {
					t.Fatalf("search result: %v", item)
				}
			}
		}
	}
}
