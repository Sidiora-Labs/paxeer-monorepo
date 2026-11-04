package source

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/config"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type repair997Node struct {
	server                                              *httptest.Server
	invalid, wrongChain, conflicting, pruned, holdProbe atomic.Bool
	head                                                atomic.Uint64
	gate                                                atomic.Bool
	entered                                             chan struct{}
	release                                             chan struct{}
	requests                                            atomic.Int64
}

func repair997Serve(t *testing.T) *repair997Node {
	t.Helper()
	n := &repair997Node{entered: make(chan struct{}, 256), release: make(chan struct{})}
	n.head.Store(256)
	n.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			ID     uint64
			Method string
			Params []json.RawMessage
		}
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			http.Error(w, "request", 400)
			return
		}
		n.requests.Add(1)
		if n.invalid.Load() {
			_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":0,"result":"0x7d"}`)
			return
		}
		var result any
		switch q.Method {
		case "eth_chainId":
			if n.holdProbe.Load() {
				n.entered <- struct{}{}
				select {
				case <-n.release:
				case <-r.Context().Done():
					return
				}
			}
			result = "0x7d"
			if n.wrongChain.Load() {
				result = "0x1"
			}
		case "eth_getBlockByNumber":
			var tag string
			_ = json.Unmarshal(q.Params[0], &tag)
			if tag == "latest" {
				tag = hexutil.EncodeUint64(n.head.Load())
			}
			height, err := hexutil.DecodeUint64(tag)
			if err != nil {
				http.Error(w, "height", 400)
				return
			}
			if n.pruned.Load() && height > 0 && height < n.head.Load() {
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": q.ID, "error": map[string]any{"code": -32000, "message": "pruned; earliest available is 256"}})
				return
			}
			if n.gate.Load() && height > 0 {
				n.entered <- struct{}{}
				select {
				case <-n.release:
				case <-r.Context().Done():
					return
				}
			}
			block := committedFixtureBlock(tag, []any{})
			block["logsBloom"] = "0x" + strings.Repeat("0", 512)
			if n.conflicting.Load() && height == 128 {
				block["hash"] = common.HexToHash("0xbad128").Hex()
			}
			result = block
		case "eth_getLogs":
			if n.pruned.Load() {
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": q.ID, "error": map[string]any{"code": -32000, "message": "pruned; earliest available is 256"}})
				return
			}
			result = []any{}
		default:
			http.Error(w, "method", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": q.ID, "result": result})
	}))
	t.Cleanup(n.server.Close)
	return n
}

func repair997Open(t *testing.T, options CommittedOptions) *CommittedSource {
	t.Helper()
	s, err := NewCommittedSource(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}

func repair997Seed(t *testing.T, s *CommittedSource) {
	t.Helper()
	result, err := s.FetchCommitted(context.Background(), FetchOptions{FromBlock: 1, ToBlock: 128}, false, nil)
	if err != nil || !result.Complete || len(result.Blocks) != 128 {
		t.Fatalf("seed coverage: blocks=%d complete=%v err=%v", len(result.Blocks), result.Complete, err)
	}
}

func repair997Kind(t *testing.T, err error, kind ErrorKind) {
	t.Helper()
	var failure *SourceError
	if !errors.As(err, &failure) || failure.Kind != kind {
		t.Fatalf("error %v, want %s", err, kind)
	}
}

func TestRepairM_997_ArchiveCapacityBounds(t *testing.T) {
	for _, requested := range []int{0, 24, 96} {
		t.Run(fmt.Sprintf("concurrency_%d", requested), func(t *testing.T) {
			primary, archive := repair997Serve(t), repair997Serve(t)
			primary.pruned.Store(true)
			options := fopts(primary.server.URL)
			options.HistoricalEndpoints = map[string]string{"archive": archive.server.URL}
			options.HistoricalConcurrency = requested
			options.HistoricalReadahead = -1
			s := repair997Open(t, options)
			limit := requested
			if limit == 0 {
				limit = defaultHistoricalConcurrency
			}
			archive.gate.Store(true)
			released := false
			defer func() {
				if !released {
					close(archive.release)
				}
			}()
			results := make(chan error, 1)
			go func() {
				result, err := s.FetchCommitted(context.Background(), FetchOptions{FromBlock: 1, ToBlock: int64(limit), Historical: true}, true, nil)
				if err == nil && (!result.Complete || len(result.Blocks) != limit) {
					err = fmt.Errorf("archive coverage blocks=%d complete=%v", len(result.Blocks), result.Complete)
				}
				results <- err
			}()
			for i := 0; i < limit; i++ {
				select {
				case <-archive.entered:
				case err := <-results:
					t.Fatalf("range exited before saturation: %v", err)
				case <-time.After(3 * time.Second):
					t.Fatalf("only %d/%d archive requests admitted", i, limit)
				}
			}
			sample := s.capacityMeasurement()
			if sample.EndpointInFlight["archive"] != limit || sample.InFlight != int64(limit) || sample.ProcessPermits != 0 || sample.HistoricalPermits != 0 {
				t.Fatalf("independent archive reservation: %+v", sample)
			}
			if err := s.validateCapacityMeasurement(sample, ^uint64(0)); err != nil {
				t.Fatalf("valid configured saturation rejected: %v", err)
			}
			var endpoint *committedEndpoint
			for _, e := range s.endpoints {
				if e.historical {
					endpoint = e
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			var chain string
			err := s.rpcOnce(ctx, endpoint, true, "eth_chainId", []any{}, &chain, 0)
			cancel()
			repair997Kind(t, err, TimeoutError)
			select {
			case <-archive.entered:
				t.Fatal("archive accepted work beyond configured concurrency")
			default:
			}
			head, err := s.ObserveHead(context.Background(), 0)
			if err != nil || head.Height != 256 || head.SourceID != "fixture" {
				t.Fatalf("live reservation lost under archive saturation: %+v %v", head, err)
			}
			for _, field := range []string{"archive", "aggregate", "process", "historical", "primary"} {
				bad := s.capacityMeasurement()
				switch field {
				case "archive":
					bad.EndpointInFlight["archive"] = limit + 1
				case "aggregate":
					bad.InFlight = int64(cap(processPermits) + limit + 1)
				case "process":
					bad.ProcessPermits = int64(cap(processPermits) + 1)
				case "historical":
					bad.HistoricalPermits = int64(cap(historicalPermits) + 1)
				case "primary":
					bad.EndpointInFlight["fixture"] = 17
				}
				repair997Kind(t, s.validateCapacityMeasurement(bad, ^uint64(0)), OverloadError)
			}
			close(archive.release)
			released = true
			select {
			case err := <-results:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("archive range failed to drain")
			}
			if sample := s.capacityMeasurement(); sample.InFlight != 0 || sample.ProcessPermits != 0 || sample.HistoricalPermits != 0 || sample.EndpointInFlight["archive"] != 0 {
				t.Fatalf("leaked reservations: %+v", sample)
			}
		})
	}
}

func TestRepairM_997_QuarantineRuntimeRejoin(t *testing.T) {
	node := repair997Serve(t)
	s := repair997Open(t, fopts(node.server.URL))
	repair997Seed(t, s)
	node.invalid.Store(true)
	for i := 0; i < 3; i++ {
		if _, err := s.ObserveHead(context.Background(), 128); err == nil {
			t.Fatal("invalid response accepted")
		}
	}
	if health := s.EndpointHealth()[0]; health.QuarantineReason != string(InvalidError) {
		t.Fatalf("not quarantined: %+v", health)
	}
	if err := s.RecoverEndpoints(context.Background()); err == nil {
		t.Fatal("dead source recovered")
	}
	node.invalid.Store(false)
	node.wrongChain.Store(true)
	repair997Kind(t, s.RecoverEndpoints(context.Background()), IdentityError)
	node.wrongChain.Store(false)
	node.conflicting.Store(true)
	repair997Kind(t, s.RecoverEndpoints(context.Background()), IntegrityError)
	if s.EndpointHealth()[0].QuarantineReason == "" {
		t.Fatal("identity mismatch bypassed rejoin")
	}
	node.conflicting.Store(false)
	if err := s.RecoverEndpoints(context.Background()); err != nil {
		t.Fatal(err)
	}
	if health := s.EndpointHealth()[0]; health.QuarantineReason != "" || health.RecoveryChecks != 128 {
		t.Fatalf("unverified recovery: %+v", health)
	}
	head, err := s.ObserveHead(context.Background(), 128)
	if err != nil || head.Height != 256 {
		t.Fatalf("head did not resume: %+v %v", head, err)
	}
	result, err := s.FetchCommitted(context.Background(), FetchOptions{FromBlock: 129, ToBlock: 130}, false, nil)
	if err != nil || !result.Complete || len(result.Blocks) != 2 {
		t.Fatalf("ingestion did not resume: %+v %v", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.RecoverEndpoints(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("probe cancellation: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	repair997Kind(t, s.RecoverEndpoints(context.Background()), ClosedError)
}

func TestRepairM_997_RecoveryRotationAndDeadline(t *testing.T) {
	first, second := repair997Serve(t), repair997Serve(t)
	options := fopts(first.server.URL)
	options.Endpoints = map[string]string{"a-primary": first.server.URL, "b-primary": second.server.URL}
	s := repair997Open(t, options)
	repair997Seed(t, s)
	first.invalid.Store(true)
	second.invalid.Store(true)
	for i := 0; i < 3; i++ {
		if _, err := s.ObserveHead(context.Background(), 128); err == nil {
			t.Fatal("failed endpoints admitted")
		}
	}
	if err := s.RecoverEndpoints(context.Background()); err == nil {
		t.Fatal("first failed source recovered")
	}
	second.invalid.Store(false)
	if err := s.RecoverEndpoints(context.Background()); err != nil {
		t.Fatalf("healthy second endpoint starved by first: %v", err)
	}
	if health := s.EndpointHealth(); health[0].QuarantineReason == "" || health[1].QuarantineReason != "" || health[1].RecoveryChecks != 128 {
		t.Fatalf("rotation health: %+v", health)
	}
	if head, err := s.ObserveHead(context.Background(), 128); err != nil || head.SourceID != "b-primary" {
		t.Fatalf("restored endpoint unavailable: %+v %v", head, err)
	}
	first.invalid.Store(false)
	first.holdProbe.Store(true)
	// A caller's shutdown deadline bounds a stalled health probe and releases
	// its transport reservation; the following verified attempt still works.
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	started := time.Now()
	err := s.RecoverEndpoints(ctx)
	cancel()
	repair997Kind(t, err, TimeoutError)
	if time.Since(started) > time.Second {
		t.Fatal("probe exceeded shutdown deadline")
	}
	if health := s.EndpointHealth()[0]; health.QuarantineReason == "" {
		t.Fatal("timed-out probe recovered endpoint")
	}
	if metrics := s.TransportMetrics(); metrics.InFlightRequests != 0 {
		t.Fatalf("probe leaked request: %+v", metrics)
	}
	first.holdProbe.Store(false)
	if err := s.RecoverEndpoints(context.Background()); err != nil {
		t.Fatalf("recovery after bounded probe: %v", err)
	}
}

func repair997Signed(t *testing.T, auth SourceSelectionAuthorization, requester, approver ed25519.PrivateKey) SignedSourceSelection {
	t.Helper()
	wire, err := json.Marshal(auth)
	if err != nil {
		t.Fatal(err)
	}
	return SignedSourceSelection{Authorization: auth, RequestSignature: ed25519.Sign(requester, wire), ApprovalSignature: ed25519.Sign(approver, wire)}
}

func repair997Post(t *testing.T, url string, signed SignedSourceSelection) int {
	t.Helper()
	body, err := json.Marshal(signed)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	return response.StatusCode
}

func TestRepairM_997_AuthorizedIncidentSelection(t *testing.T) {
	requesterKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{1}, ed25519.SeedSize))
	approverKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{2}, ed25519.SeedSize))
	keys := map[string][]byte{"requester": requesterKey.Public().(ed25519.PublicKey), "approver": approverKey.Public().(ed25519.PublicKey)}
	authority, err := NewSelectionAuthority("canonical", keys)
	if err != nil {
		t.Fatal(err)
	}
	first, second := repair997Serve(t), repair997Serve(t)
	options := fopts(first.server.URL)
	options.Endpoints = map[string]string{"a-primary": first.server.URL, "b-primary": second.server.URL}
	options.SelectionAuthority = authority
	s := repair997Open(t, options)
	repair997Seed(t, s)
	second.conflicting.Store(true)
	second.head.Store(128)
	if _, err := s.ObserveHead(context.Background(), 128); err == nil {
		t.Fatal("canonical disagreement did not fence ingestion")
	}
	incidents := s.IntegrityIncidents()
	if len(incidents) != 1 || incidents[0].Height != 128 {
		t.Fatalf("incidents: %+v", incidents)
	}
	second.conflicting.Store(false)
	if err := s.RecoverEndpoints(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.FetchCommitted(context.Background(), FetchOptions{FromBlock: 129, ToBlock: 130}, false, nil); err == nil {
		t.Fatal("transport restoration cleared incident fence")
	}
	api := httptest.NewServer(authority.Handler(s))
	defer api.Close()
	auth := SourceSelectionAuthorization{ID: "incident-128", Kind: "incident_recovery", Scope: "canonical", Reason: "verified canonical history", RequestedBy: "requester", ApprovedBy: "approver", State: "approved", Revision: 1, Expires: time.Now().Add(time.Hour), SourceID: "b-primary", Height: 128, Hash: committedFixtureHash(128)}
	auth.PayloadHash = SourceSelectionPayloadHash(auth.Scope, auth.SourceID, auth.Height, auth.Hash)
	signed := repair997Signed(t, auth, requesterKey, approverKey)
	for _, mode := range []string{"signature", "scope", "expired", "self", "payload", "source", "hash"} {
		bad := signed
		switch mode {
		case "signature":
			bad.ApprovalSignature = bytes.Repeat([]byte{0}, ed25519.SignatureSize)
		case "scope":
			bad.Authorization.Scope = "other"
			bad.Authorization.PayloadHash = SourceSelectionPayloadHash("other", auth.SourceID, auth.Height, auth.Hash)
			bad = repair997Signed(t, bad.Authorization, requesterKey, approverKey)
		case "expired":
			bad.Authorization.Expires = time.Now().Add(-time.Minute)
			bad = repair997Signed(t, bad.Authorization, requesterKey, approverKey)
		case "self":
			bad.Authorization.ApprovedBy = "requester"
			bad = repair997Signed(t, bad.Authorization, requesterKey, requesterKey)
		case "payload":
			bad.Authorization.PayloadHash = strings.Repeat("0", 64)
			bad = repair997Signed(t, bad.Authorization, requesterKey, approverKey)
		case "source":
			bad.Authorization.SourceID = "a-primary"
		case "hash":
			bad.Authorization.Hash = common.HexToHash("0xbad")
		}
		if status := repair997Post(t, api.URL, bad); status != http.StatusForbidden {
			t.Fatalf("%s authorization status=%d", mode, status)
		}
	}
	if health := s.EndpointHealth()[1]; health.QuarantineReason != "canonical_hash_disagreement" {
		t.Fatalf("unauthorized request changed source: %+v", health)
	}
	// Approved selection still cannot rejoin a source serving the wrong chain.
	second.wrongChain.Store(true)
	if status := repair997Post(t, api.URL, signed); status != http.StatusConflict {
		t.Fatalf("wrong-chain rejoin status=%d", status)
	}
	second.wrongChain.Store(false)
	if status := repair997Post(t, api.URL, signed); status != http.StatusNoContent {
		t.Fatalf("approved selection status=%d", status)
	}
	if health := s.EndpointHealth()[1]; health.QuarantineReason != "" || health.RecoveryChecks != 128 {
		t.Fatalf("selected source not verified: %+v", health)
	}
	second.head.Store(256)
	head, err := s.ObserveHead(context.Background(), 128)
	if err != nil || head.SourceID != "b-primary" || head.Height != 256 {
		t.Fatalf("selected head: %+v %v", head, err)
	}
	result, err := s.FetchCommitted(context.Background(), FetchOptions{FromBlock: 129, ToBlock: 130}, false, nil)
	if err != nil || !result.Complete || result.Blocks[129].SourceID != "b-primary" {
		t.Fatalf("selected ingestion: %+v %v", result, err)
	}
	// Expiry cannot silently turn the approved source into ordinary eligibility.
	s.mu.Lock()
	s.endpoints[1].selection.Expires = time.Now().Add(-time.Second)
	s.mu.Unlock()
	if _, err := s.ObserveHead(context.Background(), 128); err == nil {
		t.Fatal("expired selection bypassed incident fence")
	}
	if err := s.RecoverEndpoints(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.EndpointHealth()[1].QuarantineReason != "authorization_expired" {
		t.Fatal("automatic recovery renewed expired approval")
	}
	if _, err := NewSelectionAuthority("canonical", map[string][]byte{"requester": keys["requester"], "alias": keys["requester"]}); err == nil {
		t.Fatal("same signing key admitted as two operators")
	}
}

func TestRepairM_997_MixedRPCSourceSelection(t *testing.T) {
	for _, logMode := range []string{config.SourceRPC, config.SourceRPCGetLogs} {
		t.Run(logMode, func(t *testing.T) {
			var singleLogs, poolLogs, singleHeads, poolHeads atomic.Int64
			serve := func(head int64, logs, heads *atomic.Int64) *httptest.Server {
				node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var request rpcReq
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						http.Error(w, "request", 400)
						return
					}
					var result any
					switch request.Method {
					case "eth_blockNumber":
						heads.Add(1)
						result = fmt.Sprintf("0x%x", head)
					case "eth_getLogs":
						logs.Add(1)
						result = []map[string]any{logObj(common.HexToAddress("0x1"), []common.Hash{common.HexToHash("0x2")}, nil, 5, common.HexToHash("0x3"), 0)}
					case "eth_getBlockByNumber":
						result = blockObj(head)
					default:
						http.Error(w, "method", 400)
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
				}))
				t.Cleanup(node.Close)
				return node
			}
			single := serve(11, &singleLogs, &singleHeads)
			pool := serve(22, &poolLogs, &poolHeads)
			cfg := config.Config{}
			cfg.RPCURL = single.URL
			cfg.RPCURLs = []string{pool.URL}
			cfg.IndexerLogSource = logMode
			cfg.IndexerHeadSource = config.SourceRPCGetLogs
			wantHead := int64(22)
			if logMode == config.SourceRPCGetLogs {
				cfg.IndexerHeadSource = config.SourceRPC
				wantHead = 11
			}
			bundle, err := Create(context.Background(), cfg, slog.Default())
			if err != nil {
				t.Fatal(err)
			}
			defer bundle.Close()
			if bundle.Log.Name() != cfg.IndexerLogSource || bundle.Head.Name() != cfg.IndexerHeadSource || len(bundle.closers) != 2 {
				t.Fatalf("mixed modes collapsed: %s/%s closers=%d", bundle.Log.Name(), bundle.Head.Name(), len(bundle.closers))
			}
			beforeSingle, beforePool := singleHeads.Load(), poolHeads.Load()
			head, err := bundle.Head.Head(context.Background())
			if err != nil || head != wantHead {
				t.Fatalf("head=%d want=%d err=%v", head, wantHead, err)
			}
			if wantHead == 11 && (singleHeads.Load() != beforeSingle+1 || poolHeads.Load() != beforePool) || wantHead == 22 && (poolHeads.Load() != beforePool+1 || singleHeads.Load() != beforeSingle) {
				t.Fatal("head contacted the log backend")
			}
			result, err := bundle.Log.FetchLogs(context.Background(), FetchOptions{FromBlock: 5, ToBlock: 5})
			if err != nil || len(result.Logs) != 1 {
				t.Fatalf("logs=%d err=%v", len(result.Logs), err)
			}
			wantTimestamp := int64(11)
			if logMode == config.SourceRPCGetLogs {
				wantTimestamp = 22
			}
			if result.BlockTimestamps[5] != wantTimestamp {
				t.Fatalf("block read used wrong endpoint: %+v", result.BlockTimestamps)
			}
			if logMode == config.SourceRPC && (singleLogs.Load() != 1 || poolLogs.Load() != 0) || logMode == config.SourceRPCGetLogs && (poolLogs.Load() != 1 || singleLogs.Load() != 0) {
				t.Fatal("logs contacted the head backend")
			}
			cfg.IndexerHeadSource = cfg.IndexerLogSource
			shared, err := Create(context.Background(), cfg, slog.Default())
			if err != nil {
				t.Fatal(err)
			}
			defer shared.Close()
			if len(shared.closers) != 1 {
				t.Fatal("identical effective configuration not shared")
			}
		})
	}
}
