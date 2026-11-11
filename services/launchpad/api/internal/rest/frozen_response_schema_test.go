package rest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
)

// frozenV2Operation resolves a frozen v2 catalog operation and returns the
// response schema name embedded by the generated Go validator.
func frozenV2Operation(t *testing.T, path, operationID string, expected ...internaltest.FrozenOperation) string {
	t.Helper()
	want := internaltest.FrozenOperation{Method: http.MethodGet, Authenticated: false}
	if len(expected) > 1 {
		t.Fatal("at most one operation expectation is allowed")
	}
	if len(expected) == 1 {
		want = expected[0]
	}
	for _, operation := range internaltest.FrozenV2Operations(t) {
		if operation.Path != path || operation.ID != operationID {
			continue
		}
		if operation.Method != want.Method || operation.Authenticated != want.Authenticated {
			t.Fatalf("%s frozen ID %s method=%s authenticated=%t, want method=%s authenticated=%t", path, operationID, operation.Method, operation.Authenticated, want.Method, want.Authenticated)
		}
		if operation.Response == "" {
			t.Fatalf("GET %s frozen ID %s has no response schema", path, operationID)
		}
		return operation.Response
	}
	t.Fatalf("GET %s lacks frozen ID %s in the catalog", path, operationID)
	return ""
}

func TestFrozenV2OperationLooksUpCatalog(t *testing.T) {
	operations := make(map[string]internaltest.FrozenOperation)
	for _, operation := range internaltest.FrozenV2Operations(t) {
		key := operation.Method + " " + operation.Path
		if _, exists := operations[key]; exists {
			t.Fatalf("duplicate catalog route %s", key)
		}
		operations[key] = operation
	}
	chainRoutes := map[string]struct{ id, response string }{
		"GET /v2/chains":                 {"IF_CHAINS", "ChainsResponse"},
		"GET /v2/chains/{chain}":         {"IF_CHAIN_DETAIL", "ChainDetailResponse"},
		"GET /v2/chains/{chain}/revenue": {"IF_CHAIN_REVENUE", "ChainRevenueResponse"},
	}
	raw, err := os.ReadFile("testdata/contracts/v2/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Paths map[string]map[string]struct {
			OperationID   string `json:"operationId"`
			Authorization string `json:"x-authorization"`
			Responses     map[string]struct {
				Content map[string]struct {
					Schema struct {
						Ref string `json:"$ref"`
					} `json:"schema"`
				} `json:"content"`
			} `json:"responses"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &contract); err != nil {
		t.Fatal(err)
	}
	for route, expected := range chainRoutes {
		_, path, _ := strings.Cut(route, " ")
		operation := contract.Paths[path]["get"]
		if operation.OperationID != expected.id || operation.Authorization != "public" || operation.Responses["200"].Content["application/json"].Schema.Ref != "#/components/schemas/"+expected.response {
			t.Fatalf("chain route %s contract=%+v", route, operation)
		}
	}

	admission, err := rest.NewAdmission(100, 100, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	if err := rest.RegisterProductionReads(router, rest.ProductionReadOptions{
		Model: &store.AccountingReadModel{}, Stream: &broker.PostgresStream{},
		Admission: admission, RegisterSnapshots: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := rest.RegisterDevelopmentHistorySearchDiscovery(context.Background(), router, rest.DevelopmentHistoryOptions{Store: store.New(nil, nil), Admission: admission}); err != nil {
		t.Fatal(err)
	}
	count, authenticated := 0, 0
	seenChain := map[string]bool{}
	err = chi.Walk(router, func(method, path string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		operation, ok := operations[method+" "+path]
		if !ok {
			if _, added := chainRoutes[method+" "+path]; !added {
				t.Errorf("mounted route absent from the v2 catalog: %s %s", method, path)
			} else {
				seenChain[method+" "+path] = true
			}
			count++
			return nil
		}
		t.Run(operation.ID, func(t *testing.T) {
			if got := frozenV2Operation(t, path, operation.ID, operation); got != operation.Response {
				t.Errorf("%s %s response=%q, want %q", method, path, got, operation.Response)
			}
		})
		count++
		if operation.Authenticated {
			authenticated++
		}
		return nil
	})
	if err != nil || count != 24 {
		t.Fatalf("walk = %v, %d catalog operations, want 24", err, count)
	}
	if authenticated != 2 {
		t.Fatalf("mounted authenticated operations = %d, want 2", authenticated)
	}
	for route := range chainRoutes {
		if !seenChain[route] {
			t.Errorf("chain route absent: %s", route)
		}
	}
}

func TestFrozenV2ResponseSchemasMatchHandlerBytes(t *testing.T) {
	t.Run("GET /v2/chains", func(t *testing.T) {
		f := newChainReadsFixture(t)
		list := chainData(t, getV2(t, f.srv.URL+"/v2/chains?limit=2"))
		if len(list["items"].([]any)) != 2 {
			t.Fatalf("chains=%v", list)
		}
		detail := chainData(t, getV2(t, f.srv.URL+"/v2/chains/7"))
		if detail["chain"].(map[string]any)["chainId"] != "1007" {
			t.Fatalf("chain=%v", detail)
		}
		revenue := chainData(t, getV2(t, f.srv.URL+"/v2/chains/7/revenue?holder="+chainHolderA))
		if len(revenue["epochs"].([]any)) != 2 || len(revenue["claims"].([]any)) != 2 {
			t.Fatalf("revenue=%v", revenue)
		}
	})
	t.Run("GET /v2/pools/{poolAddress} homeChain", func(t *testing.T) {
		f := newPoolReadsFixture(t, "")
		response := getV2(t, f.srv.URL+"/v2/pools/"+f.first)
		body := assertV2Success(t, response, "PoolDetailResponse")
		pool := body["data"].(map[string]any)["pool"].(map[string]any)
		if pool["homeChain"] != "0" {
			t.Fatalf("homeChain=%v", pool["homeChain"])
		}
	})
	t.Run("GET /v2/leaderboards/{category}", func(t *testing.T) {
		schema := frozenV2Operation(t, "/v2/leaderboards/{category}", "IF_LEADERBOARDS")
		f := newPoolReadsFixture(t, "")
		if _, err := f.pool.Exec(context.Background(), `CREATE TABLE indexer.indexer_token_transfers_v1(chain_id integer,token text,from_address text,to_address text,height bigint,payload jsonb)`); err != nil {
			t.Fatal(err)
		}
		now := time.Now().Unix()
		insertFrozenLeaderboardFill(t, f, 10, "buy", leaderboardWalletA, "1000000", "500000000000000000000", now-3600)
		insertFrozenLeaderboardFill(t, f, 11, "sell", leaderboardWalletA, "2500000", "400000000000000000000", now-1800)
		insertFrozenLeaderboardFill(t, f, 12, "buy", leaderboardWalletB, "4000000", "900000000000000000000", now-900)
		// The validator is not vacuous: a handler body missing a required
		// envelope field is rejected by the same frozen schema.
		if interfaces.ValidateJSON(schema, []byte(`{"data":{"snapshot":null}}`)) == nil {
			t.Fatal("frozen schema accepted an invalid envelope")
		}
		for _, path := range []string{"/v2/leaderboards/trader?window=24h", "/v2/leaderboards/trader?window=24h&limit=1", "/v2/leaderboards/volume?window=7d", "/v2/leaderboards/creator?window=30d"} {
			got := getV2(t, f.srv.URL+path)
			if got.status != 200 {
				t.Fatalf("%s status=%d body=%s", path, got.status, got.body)
			}
			if err := interfaces.ValidateJSON(schema, got.body); err != nil {
				t.Fatalf("%s handler bytes violate frozen %s: %v body=%s", path, schema, err, got.body)
			}
		}
	})
	t.Run("GET /v2/snapshots", func(t *testing.T) {
		schema := frozenV2Operation(t, "/v2/snapshots", "IF_TOPIC_SNAPSHOT_READ")
		_, model, stream, address, _ := productionReadsFixture(t)
		admission, err := rest.NewAdmission(100, 100, 50*time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}
		router := chi.NewRouter()
		if err = rest.RegisterProductionReads(router, rest.ProductionReadOptions{Model: model, Stream: stream, Admission: admission, RegisterSnapshots: true}); err != nil {
			t.Fatal(err)
		}
		server := httptest.NewServer(router)
		defer server.Close()
		srv := server.URL
		for _, path := range []string{"/v2/snapshots?topics=market.stats." + address, "/v2/snapshots?topics=market.stats." + address + "&normalizedFilters=" + `%7B%22poolAddress%22%3A%22` + address + `%22%7D`} {
			got := getV2(t, srv+path)
			if got.status != 200 {
				t.Fatalf("%s status=%d body=%s", path, got.status, got.body)
			}
			if err := interfaces.ValidateJSON(schema, got.body); err != nil {
				t.Fatalf("%s handler bytes violate frozen %s: %v body=%s", path, schema, err, got.body)
			}
		}
	})
}

func insertFrozenLeaderboardFill(t *testing.T, f poolReadsFixture, sequence int, side, wallet, usdl, tokens string, timestamp int64) {
	t.Helper()
	position := accounting.SourcePosition{ChainID: "125", Height: strconv.Itoa(1000 + sequence), TransactionIndex: "3", LogIndex: "2", TransactionHash: fmt.Sprintf("0x%064x", sequence), BlockHash: fmt.Sprintf("0x%064x", 1000+sequence)}
	logIndex := "1"
	kind, from, to, paid, disposition := "output-transfer", f.first, wallet, usdl, "distributed_usdl"
	if side == "sell" {
		kind, from, to, paid, disposition = "input-transfer", wallet, f.first, tokens, "retained_token_reserve"
	}
	receipt := fmt.Sprintf("%064x", sequence+2000)
	provenance := accounting.Provenance{DeploymentID: "fixture125", ABIRevision: "sidiora-v1", EmitterCodeEpoch: "epoch1", PoolCodeEpoch: "epoch1", ReceiptSHA256: receipt, HeaderSHA256: strings.Repeat("d", 64)}
	fill := accounting.FillV1{
		Version: accounting.FillVersion, Position: position, ActualEmitter: f.first, PoolAddress: f.first,
		Token:      accounting.Asset{Address: "0x" + strings.Repeat("3", 40), Decimals: 18},
		Settlement: accounting.Asset{Address: "0x" + strings.Repeat("4", 40), Decimals: 6},
		Side:       side, TokenAmountRaw: tokens, USDLAmountRaw: usdl, PaidInputRaw: paid, FeeRaw: "0", InputDerivationQuality: "receipt_incomplete", FeeDisposition: disposition,
		Attribution: accounting.Attribution{TransactionSender: wallet, Trader: &wallet, Payer: &wallet, Recipient: &wallet, Quality: "exact_receipt", Evidence: []accounting.EvidenceRef{{Kind: kind, ArtifactSHA256: receipt, ArtifactID: position.TransactionHash, LogIndex: &logIndex}}},
		Execution:   accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
		Provenance:  provenance, BlockTimestampSeconds: strconv.FormatInt(timestamp, 10),
	}
	fill.SourceFillID = accounting.SourceFillIdentity(position, provenance.DeploymentID)
	if err := accounting.ValidateFillV1(fill); err != nil {
		t.Fatal(err)
	}
	movementPosition := position
	movementPosition.LogIndex = logIndex
	movement, err := json.Marshal(map[string]any{"token": fill.Token.Address, "from": from, "to": to, "amountRaw": tokens, "position": movementPosition, "provenance": provenance})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(context.Background(), `INSERT INTO indexer.indexer_token_transfers_v1(chain_id,token,from_address,to_address,height,payload) VALUES(125,$1,$2,$3,$4,$5)`, fill.Token.Address, from, to, 1000+sequence, movement); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(fill)
	if err != nil {
		t.Fatal(err)
	}
	partition, err := json.Marshal(accounting.PartitionPosition{ProjectionVersion: "v1", PartitionKind: "market", PartitionKey: "125:" + f.first, Sequence: strconv.Itoa(sequence), SourceEventID: fill.SourceFillID, SourceOrder: position})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(context.Background(), `INSERT INTO stats.accounting_market_commits(publication_id,source_fill_id,source_fact_digest,projection_version,pool,bucket_start,position,fill,sequence,revision) VALUES($1,$2,$3,'v1',$4,$5,$6,$7,$8,$8)`, "frozen-"+f.first+"-"+strconv.Itoa(sequence), fill.SourceFillID, strings.Repeat("e", 64), f.first, timestamp-timestamp%60, partition, raw, sequence); err != nil {
		t.Fatal(err)
	}
}
