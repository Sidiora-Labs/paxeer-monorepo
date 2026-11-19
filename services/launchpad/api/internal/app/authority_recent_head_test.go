package app

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/capabilities"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-chi/chi/v5"
)

type authorityAttestationCounter struct {
	deployment.Reader
	attestations atomic.Int64
}

func (r *authorityAttestationCounter) ChainID(ctx context.Context) (*big.Int, error) {
	r.attestations.Add(1)
	return r.Reader.ChainID(ctx)
}

func productionAuthorityRouter(t *testing.T, a *networkAuthority) http.Handler {
	t.Helper()
	admission, err := rest.NewAdmission(64, 64, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	a.Register(router, admission)
	return router
}

func productionAuthorityHeads(t *testing.T, a *networkAuthority) func(*types.Header, string) {
	t.Helper()
	a.pool = internaltest.NewPostgres(t)
	internaltest.ApplyReadMigrations(t, a.pool, "60001")
	a.config.SourceSchema = "indexer"
	return func(header *types.Header, integrity string) {
		t.Helper()
		_, err := a.pool.Exec(t.Context(), "INSERT INTO indexer.indexer_committed_heads(chain_id,stream_id,height,block_hash,integrity,source_id) VALUES(125,'authority',$1,$2,$3,'chain') ON CONFLICT(chain_id,stream_id,height) DO UPDATE SET block_hash=excluded.block_hash,integrity=excluded.integrity", header.Number.String(), strings.ToLower(header.Hash().Hex()), integrity)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func productionPreflightRequest(t *testing.T, a *networkAuthority, path string, family capabilities.Family, hash common.Hash) *http.Request {
	t.Helper()
	trader := common.HexToAddress("0xa1")
	body, err := json.Marshal(capabilities.PreflightRequest{Family: family, ExpectedRevision: a.engine.Revision().RevisionSHA256, BlockHash: hash, Trader: trader, Recipient: trader})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "https://api.example/v2/preflight/"+path, bytes.NewReader(body))
	r.Host = strings.TrimPrefix(webOrigin, "https://")
	r.Header.Set("Origin", webOrigin)
	return r
}

func assertProductionPreflight(t *testing.T, router http.Handler, request *http.Request, hash common.Hash) {
	t.Helper()
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	var answer interfaces.ConnectedCapabilitiesPreflight
	if response.Code != http.StatusOK || interfaces.ValidateJSON("ConnectedCapabilitiesPreflight", response.Body.Bytes()) != nil || json.Unmarshal(response.Body.Bytes(), &answer) != nil || !strings.EqualFold(answer.BlockHash, hash.Hex()) {
		t.Fatalf("preflight = %d %s", response.Code, response.Body)
	}
}

func TestProductionAuthorityRewriteOrigin(t *testing.T) {
	a, reader, _ := newAuthorityReadinessState(t)
	commit := productionAuthorityHeads(t, a)
	commit(reader.header, "healthy")
	a.origins = map[string]bool{webOrigin: true}
	a.proxies = []string{"10.0.0.0/8", "fd12::/16"}
	router := productionAuthorityRouter(t, a)
	for path, family := range map[string]capabilities.Family{"create": capabilities.Creation, "quote": capabilities.PlainQuote, "trade": capabilities.Trading, "claim": capabilities.NFTFees} {
		t.Run(path, func(t *testing.T) {
			for _, tc := range []struct {
				name, remote, proto, host string
				proxies                   []string
				allowed                   bool
			}{
				{"trusted rewrite", "10.1.2.3:5000", "https", strings.TrimPrefix(webOrigin, "https://"), []string{"10.0.0.0/8"}, true},
				{"first forwarded values", "[fd12:aa::7]:5000", "https, http", strings.TrimPrefix(webOrigin, "https://") + ", api.example", []string{"fd12::/16"}, true},
				{"untrusted peer", "203.0.113.9:5000", "https", strings.TrimPrefix(webOrigin, "https://"), []string{"10.0.0.0/8"}, false},
				{"plain scheme", "10.1.2.3:5000", "http", strings.TrimPrefix(webOrigin, "https://"), []string{"10.0.0.0/8"}, false},
				{"different host", "10.1.2.3:5000", "https", "other.example", []string{"10.0.0.0/8"}, false},
				{"no trusted ranges", "10.1.2.3:5000", "https", strings.TrimPrefix(webOrigin, "https://"), nil, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					a.proxies = tc.proxies
					r := productionPreflightRequest(t, a, path, family, reader.header.Hash())
					r.Host, r.RemoteAddr = "api.example", tc.remote
					r.Header.Set("X-Forwarded-Proto", tc.proto)
					r.Header.Set("X-Forwarded-Host", tc.host)
					if tc.allowed {
						assertProductionPreflight(t, router, r, reader.header.Hash())
					} else {
						response := httptest.NewRecorder()
						router.ServeHTTP(response, r)
						assertAPIError(t, response.Code, response.Body.Bytes(), 403, "SCOPE_DENIED")
					}
				})
			}
		})
	}
}

func TestProductionAuthorityKeptEnvelope(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a, reader, head := newAuthorityReadinessState(t)
		counter := &authorityAttestationCounter{Reader: reader}
		a.reader = counter
		var hold chan struct{}
		headReads := 0
		a.readHead = func(ctx context.Context) (committedHead, error) {
			headReads++
			if hold != nil {
				select {
				case <-hold:
				case <-ctx.Done():
					return committedHead{}, ctx.Err()
				}
			}
			return head, nil
		}
		if err := a.refresh(t.Context()); err != nil {
			t.Fatal(err)
		}
		router := productionAuthorityRouter(t, a)
		get := func() *httptest.ResponseRecorder {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/network/authority", nil))
			return response
		}
		expected, err := json.Marshal(a.lastEnvelope)
		if err != nil {
			t.Fatal(err)
		}
		if err := interfaces.ValidateJSON("ConnectedNetworkAuthorityResponse", expected); err != nil {
			t.Fatal(err)
		}
		expected = append(expected, '\n')
		before := reader.reads.Load()
		for range 8 {
			response := get()
			if response.Code != 200 || !bytes.Equal(response.Body.Bytes(), expected) {
				t.Fatalf("kept envelope = %d %s", response.Code, response.Body)
			}
		}
		time.Sleep(authorityEnvelopeMaxAge)
		if response := get(); response.Code != 200 || !bytes.Equal(response.Body.Bytes(), expected) {
			t.Fatal("envelope at cache bound differs")
		}
		if counter.attestations.Load() != 1 || headReads != 1 || reader.reads.Load() != before {
			t.Fatal("kept envelope read an authority dependency")
		}
		time.Sleep(time.Nanosecond)
		hold = make(chan struct{})
		answers := make([]*httptest.ResponseRecorder, 8)
		var finished atomic.Int64
		for i := range answers {
			go func() { answers[i] = get(); finished.Add(1) }()
		}
		synctest.Wait()
		if headReads != 2 || finished.Load() != 0 {
			t.Fatal("expired requests did not share the pending attestation")
		}
		close(hold)
		synctest.Wait()
		refreshed, err := json.Marshal(a.lastEnvelope)
		if err != nil {
			t.Fatal(err)
		}
		refreshed = append(refreshed, '\n')
		if bytes.Equal(refreshed, expected) || counter.attestations.Load() != 2 || headReads != 2 || finished.Load() != int64(len(answers)) {
			t.Fatal("expired requests did not attest once and keep the result")
		}
		for _, response := range answers {
			if response.Code != 200 || !bytes.Equal(response.Body.Bytes(), refreshed) {
				t.Fatalf("refreshed envelope = %d %s", response.Code, response.Body)
			}
		}
	})
}

func TestProductionAuthorityRecentHeadRefusals(t *testing.T) {
	a, reader, _ := newAuthorityReadinessState(t)
	commit := productionAuthorityHeads(t, a)
	a.origins = map[string]bool{webOrigin: true}
	router := productionAuthorityRouter(t, a)
	newest := types.CopyHeader(reader.header)
	newest.Number.Add(newest.Number, big.NewInt(5))
	commit(newest, "healthy")
	for _, tc := range []struct {
		name    string
		prepare func() common.Hash
	}{
		{"unknown chain hash", func() common.Hash { return common.HexToHash("0xab") }},
		{"no committed row", func() common.Hash { return reader.header.Hash() }},
		{"unhealthy committed row", func() common.Hash { commit(reader.header, "quarantined"); return reader.header.Hash() }},
		{"different committed hash", func() common.Hash {
			h := types.CopyHeader(reader.header)
			h.Extra = []byte{1}
			commit(h, "healthy")
			return reader.header.Hash()
		}},
		{"different committed height", func() common.Hash {
			if _, err := a.pool.Exec(t.Context(), "INSERT INTO indexer.indexer_committed_heads(chain_id,stream_id,height,block_hash,integrity,source_id) VALUES(125,'authority',$1,$2,'healthy','chain')", new(big.Int).Sub(reader.header.Number, big.NewInt(1)).String(), strings.ToLower(reader.header.Hash().Hex())); err != nil {
				t.Fatal(err)
			}
			return reader.header.Hash()
		}},
		{"above newest committed head", func() common.Hash {
			if _, err := a.pool.Exec(t.Context(), "DELETE FROM indexer.indexer_committed_heads"); err != nil {
				t.Fatal(err)
			}
			h := types.CopyHeader(reader.header)
			h.Number.Sub(h.Number, big.NewInt(1))
			commit(h, "healthy")
			return reader.header.Hash()
		}},
		{"committed block beyond age bound", func() common.Hash {
			reader.header.Time = uint64(time.Now().Add(-authorityRecentHeadMaxAge - time.Second).Unix())
			commit(reader.header, "healthy")
			return reader.header.Hash()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := a.pool.Exec(t.Context(), "DELETE FROM indexer.indexer_committed_heads"); err != nil {
				t.Fatal(err)
			}
			commit(newest, "healthy")
			hash := tc.prepare()
			response := httptest.NewRecorder()
			router.ServeHTTP(response, productionPreflightRequest(t, a, "trade", capabilities.Trading, hash))
			assertAPIError(t, response.Code, response.Body.Bytes(), 409, "REVISION_CONFLICT")
		})
	}
}

func TestProductionAuthorityRecentCommittedHead(t *testing.T) {
	a, reader, head := newAuthorityReadinessState(t)
	commit := productionAuthorityHeads(t, a)
	commit(reader.header, "healthy")
	a.origins = map[string]bool{webOrigin: true}
	router := productionAuthorityRouter(t, a)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/network/authority", nil))
	var envelope authorityEnvelope
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &envelope) != nil || envelope.CommittedHead != head {
		t.Fatalf("authority = %d %s", response.Code, response.Body)
	}
	newest := types.CopyHeader(reader.header)
	newest.Number.Add(newest.Number, big.NewInt(7))
	commit(newest, "healthy")
	for range 2 {
		other := &networkAuthority{pool: a.pool, config: a.config, reader: a.reader, engine: a.engine, origins: a.origins}
		assertProductionPreflight(t, productionAuthorityRouter(t, other), productionPreflightRequest(t, other, "trade", capabilities.Trading, common.HexToHash(envelope.CommittedHead.Hash)), reader.header.Hash())
	}
	if _, err := a.pool.Exec(t.Context(), "DROP VIEW indexer.indexer_committed_heads_v2 CASCADE"); err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, productionPreflightRequest(t, a, "trade", capabilities.Trading, reader.header.Hash()))
	assertAPIError(t, response.Code, response.Body.Bytes(), 503, "DEPENDENCY_UNAVAILABLE")
}

func TestProductionAuthorityMovingHead(t *testing.T) {
	source, manifest := authorityEVMSource(t)
	_, client := forkChain(t, source)
	reader, err := capabilities.NewPinnedRPCReader(client)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := capabilities.NewPreflightEngine(reader, manifest, developmentPoolRuntime, developmentSerialInterface, authorityMaxAge, authorityAttestationTimeout)
	if err != nil {
		t.Fatal(err)
	}
	a := &networkAuthority{reader: reader, engine: engine, origins: map[string]bool{webOrigin: true}, config: networkAuthorityConfig{
		SchemaVersion: 2, Manifest: manifest, GoAddresses: manifest.Addresses(), FrontendAddresses: manifest.Addresses(),
		Endpoints: deployment.PublicEndpoints{RPC: liveRPC, Explorer: developmentExplorer, AllowedRPCs: []string{liveRPC}, AllowedExplorers: []string{developmentExplorer}},
	}}
	commit := productionAuthorityHeads(t, a)
	indexerCursors(t, a.pool, 1)
	development := &developmentAuthority{store: store.New(a.pool, nil), rpc: client, reader: reader, engine: engine, origins: a.origins, logger: slog.Default()}
	admission, err := rest.NewAdmission(64, 64, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	developmentRouter := chi.NewRouter()
	development.Register(developmentRouter, admission)
	headSource := &networkAuthority{pool: a.pool, config: a.config, engine: engine}
	a.readHead = func(ctx context.Context) (committedHead, error) {
		if err := client.CallContext(ctx, nil, "evm_mine"); err != nil {
			return committedHead{}, err
		}
		header, err := reader.HeaderByNumber(ctx, nil)
		if err != nil {
			return committedHead{}, err
		}
		if wait := time.Until(time.Unix(int64(header.Time), 0)); wait > 0 {
			time.Sleep(wait)
		}
		commit(header, "healthy")
		if _, err := a.pool.Exec(ctx, "UPDATE indexer.cursors SET last_processed_block=$1 WHERE chain_id=125", header.Number.Uint64()); err != nil {
			return committedHead{}, err
		}
		return headSource.head(ctx)
	}
	router := productionAuthorityRouter(t, a)
	for round := range 20 {
		a.mu.Lock()
		a.builtAt = time.Time{}
		a.mu.Unlock()
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/network/authority", nil))
		var envelope authorityEnvelope
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &envelope) != nil {
			t.Fatalf("round %d authority = %d %s", round, response.Code, response.Body)
		}
		newest, err := a.head(t.Context())
		if err != nil || newest.Number == envelope.CommittedHead.Number {
			t.Fatalf("moving head = %+v, %v", newest, err)
		}
		assertProductionPreflight(t, router, productionPreflightRequest(t, a, "trade", capabilities.Trading, common.HexToHash(envelope.CommittedHead.Hash)), common.HexToHash(envelope.CommittedHead.Hash))
		assertProductionPreflight(t, developmentRouter, productionPreflightRequest(t, a, "trade", capabilities.Trading, common.HexToHash(envelope.CommittedHead.Hash)), common.HexToHash(envelope.CommittedHead.Hash))
	}
}
