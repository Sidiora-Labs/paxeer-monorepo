package rest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/node"
	"github.com/go-chi/chi/v5"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain/profile"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestRepairM_1008_CursorParity(t *testing.T) {
	admission, err := rest.NewAdmission(16, 16, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{"production", "development"} {
		t.Run(profile, func(t *testing.T) {
			router := chi.NewRouter()
			if profile == "production" {
				err = rest.RegisterProductionReads(router, rest.ProductionReadOptions{Model: &store.AccountingReadModel{}, Admission: admission})
			} else {
				err = rest.RegisterDevelopmentMarketReads(context.Background(), router, rest.DevelopmentMarketOptions{Store: store.New(nil, nil), Admission: admission})
			}
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(router)
			defer server.Close()
			for _, category := range []string{"trending", "breakout", "volume", "gainers", "losers", "movers", "unusual", "new"} {
				for _, cursor := range []string{"opaque", "eyJvZmZzZXQiOjUwfQ", strings.Repeat("x", 512)} {
					got := getV2(t, server.URL+"/v2/rankings/"+category+"?limit=3&cursor="+cursor)
					assertV2Error(t, got, 400, "CURSOR_INVALID")
					decoded, err := interfaces.DecodeError(got.body)
					if err != nil || decoded.Retryable || decoded.RetryAfterMs != nil || got.header.Get("Retry-After") != "" || got.header.Get("Cache-Control") != "no-store" || got.header.Get("X-Schema-Version") != "2" {
						t.Fatalf("%s cursor metadata: %v %s", profile, err, got.body)
					}
				}
			}
			// Empty cursors remain invalid input; they are never a first page.
			assertV2Error(t, getV2(t, server.URL+"/v2/rankings/trending?cursor="), 422, "FIELD_INVALID")
		})
	}
}

// A real geth node on its default signing domain is the mismatched peer.
// No RPC result is rewritten to manufacture the mismatch.
func repair1008WrongNode(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	backend := simulated.NewBackend(types.GenesisAlloc{}, func(n *node.Config, _ *ethconfig.Config) {
		n.HTTPHost, n.HTTPPort, n.HTTPModules = "127.0.0.1", port, []string{"eth"}
	})
	t.Cleanup(func() { _ = backend.Close() })
	backend.Commit()
	origin := "http://127.0.0.1:" + strconv.Itoa(port)
	client, err := ethclient.Dial(origin)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	id, err := client.ChainID(context.Background())
	if err != nil || id.Cmp(big.NewInt(1337)) != 0 {
		t.Fatalf("mismatched node's real signing domain: %v %v", id, err)
	}
	return origin
}

// Switching a network hop preserves the actual nodes' JSON-RPC responses.
func repair1008Switch(t *testing.T, initial string) (*httptest.Server, *atomic.Value) {
	t.Helper()
	upstream := &atomic.Value{}
	upstream.Store(initial)
	proxy := &httputil.ReverseProxy{Director: func(r *http.Request) {
		target, err := url.Parse(upstream.Load().(string))
		if err != nil {
			panic(err)
		}
		r.URL.Scheme, r.URL.Host, r.Host = target.Scheme, target.Host, target.Host
	}}
	server := httptest.NewServer(proxy)
	t.Cleanup(server.Close)
	return server, upstream
}

func TestRepairM_1008_ChainAdmissionAndReadiness(t *testing.T) {
	ctx := context.Background()
	wrong := repair1008WrongNode(t)
	if err := rest.RegisterDevelopmentMarketReads(ctx, chi.NewRouter(), rest.DevelopmentMarketOptions{ChainRPCURL: wrong}); err == nil {
		t.Fatal("development startup admitted chain 1337 as chain 125")
	}
	supported := newMktChain(t)
	hop, upstream := repair1008Switch(t, supported.url)
	var readiness func(context.Context) error
	router := chi.NewRouter()
	err := rest.RegisterDevelopmentMarketReads(ctx, router, rest.DevelopmentMarketOptions{
		ChainRPCURL: hop.URL, Store: store.New(nil, nil),
		BindChainReadiness: func(check func(context.Context) error) { readiness = check },
	})
	if err != nil || readiness == nil {
		t.Fatalf("supported startup/readiness binding: %v", err)
	}
	if err := readiness(ctx); err != nil {
		t.Fatalf("chain 125 readiness: %v", err)
	}
	server := httptest.NewServer(router)
	defer server.Close()
	upstream.Store(wrong)
	if err := readiness(ctx); err == nil {
		t.Fatal("readiness reused the startup chain binding")
	}
	// Refusal happens before a database read, including the optional-watermark
	// routes that otherwise could label cached results as chain 125.
	for _, path := range []string{"/v2/pools", "/v2/rankings/trending", "/v2/pools/" + mktPoolA, "/v2/platform/metrics"} {
		got := getV2(t, server.URL+path)
		assertV2Error(t, got, 503, "DEPENDENCY_UNAVAILABLE")
		assertRetryable(t, got, true)
	}
	upstream.Store(supported.url)
	if err := readiness(ctx); err != nil {
		t.Fatalf("readiness did not recover on the supported node: %v", err)
	}
}

func TestRepairM_1008_RealCreationAndRestart(t *testing.T) {
	chain := newMktChain(t)
	const optical = "0x0000000000000000000000000000000000000000"
	receipt := chain.emitMarketCreated(t, mktPoolID(mktPoolA), mktTokenA, mktCreator, mktPoolA, optical)
	event, err := chain.emitter.ParseMarketCreated(*receipt.Logs[0])
	if err != nil {
		t.Fatal(err)
	}
	fee := 100
	// The row peer retains the actual generated event's fields. No topic or
	// event payload is constructed by this test.
	row := store.DevelopmentPool{
		Pool: strings.ToLower(event.Pool.Hex()), Token: strings.ToLower(event.Token.Hex()), Creator: strings.ToLower(event.Creator.Hex()), Optical: strings.ToLower(event.Optical.Hex()), PoolID: common.Hash(event.PoolId).Hex(),
		CreatedHeight: event.BlockNumber.Int64(), CreatedAt: event.Timestamp.Int64(), CreationTransaction: strings.ToLower(receipt.TxHash.Hex()),
		FeeBps: &fee, Reserves: &store.PoolReserves{VirtualUsdl: "100000000000000000000000001", RealUsdl: "9999999999999999999999999", Token: "800000000000000000000000000"},
	}
	// A disk-backed fixture peer preserves the indexed row across a complete
	// consumer stop/reopen. It does not stand in for the indexer processor.
	path := filepath.Join(t.TempDir(), "indexed-row.json")
	raw, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	hop, upstream := repair1008Switch(t, chain.url)
	var first interfaces.Pool
	for restart := 0; restart < 2; restart++ {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var recovered store.DevelopmentPool
		if err := json.Unmarshal(raw, &recovered); err != nil {
			t.Fatal(err)
		}
		handler, closeChain, err := rest.RepairM1008CreationHandler(context.Background(), hop.URL, recovered)
		if err != nil {
			t.Fatal(err)
		}
		server := httptest.NewServer(handler)
		got := getV2(t, server.URL+"/v2/pools?sort=new")
		if got.status != 200 {
			t.Fatalf("creation consumer after restart %d: %d %s", restart, got.status, got.body)
		}
		decoded, err := interfaces.DecodePoolListResponse(got.body)
		if err != nil || len(decoded.Data.Items) != 1 {
			t.Fatalf("generated consumer: %v %s", err, got.body)
		}
		pool := decoded.Data.Items[0]
		wantPosition := interfaces.ChainPosition{ChainId: "125", Height: interfaces.UInt256(receipt.BlockNumber.String()), TransactionIndex: interfaces.UInt256(strconv.FormatUint(uint64(receipt.TransactionIndex), 10)), LogIndex: interfaces.UInt256(strconv.FormatUint(uint64(event.Raw.Index), 10)), TransactionHash: strings.ToLower(receipt.TxHash.Hex()), BlockHash: strings.ToLower(receipt.BlockHash.Hex())}
		if pool.CreatedPosition != wantPosition || pool.PoolAddress != row.Pool || pool.TokenAddress != row.Token || pool.Creator != row.Creator || pool.ChainId != "125" || string(pool.VirtualUsdlReserveRaw) != string(row.Reserves.VirtualUsdl) || string(pool.RealUsdlBalanceRaw) != string(row.Reserves.RealUsdl) || string(pool.TokenReserveRaw) != string(row.Reserves.Token) || string(pool.EffectivePricingUsdlReserveRaw) != "110000000000000000000000000" || pool.FeeBps != 100 || pool.QuoteAssetDecimals != 6 || pool.QuoteAssetAddress != strings.ToLower(profile.USDLAddress) || !pool.CreatedAt.Present || pool.CreatedAt.Value != time.Unix(event.Timestamp.Int64(), 0).UTC().Format("2006-01-02T15:04:05.000Z") {
			t.Fatalf("creation fields lost in HTTP/generated decoder: %+v", pool)
		}
		if decoded.Watermark == nil || string(decoded.Watermark.Height) != receipt.BlockNumber.String() || decoded.Watermark.BlockHash != wantPosition.BlockHash || decoded.NextCursor != nil {
			t.Fatalf("creation watermark: %+v", decoded)
		}
		if restart == 0 {
			first = pool
			// Even a warm creation/head cache cannot cross an endpoint drift.
			upstream.Store(repair1008WrongNode(t))
			assertV2Error(t, getV2(t, server.URL+"/v2/pools?sort=new"), 503, "DEPENDENCY_UNAVAILABLE")
			upstream.Store(chain.url)
		} else if !reflect.DeepEqual(pool, first) {
			t.Fatalf("cold consumer did not recover the original creation: %+v / %+v", pool, first)
		}
		server.Close()
		closeChain()
	}
}

func repair1008RankingProjection(t *testing.T) store.PublicProjection {
	t.Helper()
	watermark := json.RawMessage(`{"chainId":"125","height":"33963716","blockHash":"0x367aa367ab921f314429468f9023ba3ec4410a43afd00305f9b2cc3b53d3186e","projectionRevision":"123","observedAt":"2026-10-13T10:00:00.000Z","integrity":"healthy"}`)
	items := make([]map[string]any, 75)
	for i := range items {
		items[i] = map[string]any{"poolAddress": fmt.Sprintf("0x%040x", i+1), "category": "trending", "rank": i + 1, "score": "900719925474099312345" + fmt.Sprintf("%03d", 999-i), "modelVersion": "fixture-projection-v1", "computedAt": "2026-10-13T10:00:00.000Z"}
	}
	payload, err := json.Marshal(map[string]any{"snapshotId": "8c0f8a3e-2b6c-4d0e-9a51-6f4f1d0b7c11", "category": "trending", "modelVersion": "fixture-projection-v1", "computedAt": "2026-10-13T10:00:00.000Z", "items": items, "watermark": watermark, "totalEligible": "125", "truncated": false, "replace": true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := interfaces.DecodeRankingSnapshot(payload); err != nil {
		t.Fatal(err)
	}
	return store.PublicProjection{Topic: "market.rankings.trending", Sequence: "17", Revision: "123", Watermark: watermark, Payload: payload, OccurredAt: "2026-10-13T10:00:00.000Z"}
}

func TestRepairM_1008_RankingsReference(t *testing.T) {
	full := repair1008RankingProjection(t)
	router := chi.NewRouter()
	router.Method("GET", "/v2/rankings/{category}", rest.RepairM1008RankingsHandler(full))
	server := httptest.NewServer(router)
	defer server.Close()
	for _, limit := range []int{0, 1, 3, 50, 74, 75, 100} {
		path := "/v2/rankings/trending"
		count := limit
		if limit == 0 {
			count = 50
		} else {
			path += "?limit=" + strconv.Itoa(limit)
		}
		if count > 75 {
			count = 75
		}
		got := getV2(t, server.URL+path)
		if got.status != 200 {
			t.Fatalf("rankings %s: %d %s", path, got.status, got.body)
		}
		if err := restMeasurementRankingsReference(path, got.body, full); err != nil {
			t.Fatalf("valid bounded response rejected for %s: %v", path, err)
		}
		decoded, err := interfaces.DecodeRankingsResponse(got.body)
		if err != nil || len(decoded.Data.Snapshot.Items) != count || decoded.Data.Snapshot.Truncated != (count < 75) || string(decoded.Data.Snapshot.TotalEligible) != "125" {
			t.Fatalf("bounded response %s: %v %+v", path, err, decoded)
		}
		// Mutate each retained field separately. A reference that checks only
		// lengths, first/last ranks or truncation must fail these assertions.
		for _, field := range []string{"poolAddress", "category", "rank", "score", "modelVersion", "computedAt", "snapshotId", "totalEligible", "truncated", "replace", "watermark", "revision", "generatedAt", "nextCursor"} {
			var document map[string]any
			if err := json.Unmarshal(got.body, &document); err != nil {
				t.Fatal(err)
			}
			snapshot := document["data"].(map[string]any)["snapshot"].(map[string]any)
			entry := snapshot["items"].([]any)[0].(map[string]any)
			switch field {
			case "poolAddress":
				entry[field] = fmt.Sprintf("0x%040x", 1000)
			case "category":
				entry[field] = "volume"
			case "rank":
				entry[field] = 99
			case "score":
				entry[field] = "900719925474099312345998"
			case "modelVersion":
				entry[field] = "other-model"
			case "computedAt":
				entry[field] = "2026-10-13T09:00:00.000Z"
			case "snapshotId":
				snapshot[field] = "7c0f8a3e-2b6c-4d0e-9a51-6f4f1d0b7c11"
			case "totalEligible":
				snapshot[field] = "126"
			case "truncated":
				snapshot[field] = !snapshot[field].(bool)
			case "replace":
				snapshot[field] = false
			case "watermark":
				snapshot[field].(map[string]any)["projectionRevision"] = "124"
			case "revision":
				document[field] = "124"
			case "generatedAt":
				document[field] = "2026-10-13T09:00:00.000Z"
			case "nextCursor":
				document[field] = "invented-continuation"
			}
			mutated, err := json.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
			if err := restMeasurementRankingsReference(path, mutated, full); err == nil {
				t.Fatalf("reference accepted corrupted %s at limit %d", field, limit)
			}
		}
	}
	// Already-truncated projections retain that fact even when all stored
	// entries fit the requested limit; totalEligible must stay independent.
	var snapshot map[string]any
	if err := json.Unmarshal(full.Payload, &snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot["truncated"] = true
	var err error
	full.Payload, err = json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	router = chi.NewRouter()
	router.Method("GET", "/v2/rankings/{category}", rest.RepairM1008RankingsHandler(full))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("GET", "/v2/rankings/trending?limit=100", nil))
	if recorder.Code != 200 || restMeasurementRankingsReference("/v2/rankings/trending?limit=100", recorder.Body.Bytes(), full) != nil {
		t.Fatalf("stored truncation lost: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestRepairM_1008_ServiceContracts(t *testing.T) {
	path := "../../../../../contracts/services.md"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, required := range []string{"## Launchpad API", "## Launchpad metadata", "search errors", "media visibility", "publication recovery", "CURSOR_INVALID", "DEPENDENCY_UNAVAILABLE", "MetadataGetResponse", "ConnectedTokenMediaResponse", "RankingsResponse", "Error", "0x7d"} {
		if !strings.Contains(text, required) {
			t.Errorf("service contract missing %q", required)
		}
	}
	for _, target := range []string{"../sdk/launchpad/shared/interfaces/v2", "../services/launchpad/api/internal/rest/rest.go", "../services/launchpad/metadata/internal/metadata/publication.go"} {
		if !strings.Contains(text, "]("+target+")") {
			t.Errorf("missing maintained authority link %s", target)
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(path), target)); err != nil {
			t.Errorf("broken authority link %s: %v", target, err)
		}
	}
	// The documented search transport error is emitted by the actual proxy.
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	upstream.Close()
	router := chi.NewRouter()
	if err := rest.RegisterV2SearchProxy(router, upstream.URL); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(router)
	defer server.Close()
	response, err := http.Get(server.URL + "/v2/search?q=token")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := interfaces.DecodeError(body)
	if err != nil || response.StatusCode != 503 || decoded.Code != "DEPENDENCY_UNAVAILABLE" || !decoded.Retryable {
		t.Fatalf("documented search error differs from real proxy: %v %s", err, body)
	}
}
