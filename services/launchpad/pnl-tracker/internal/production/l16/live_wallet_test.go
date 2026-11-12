package l16

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5/pgxpool"
)

func inventoryIdentityFixture(t *testing.T, wallet string) (auth.HTTPIdentityAuthority, auth.IdentityPrincipal) {
	t.Helper()
	p := auth.IdentityPrincipal{AccountID: "11111111-1111-4111-8111-111111111111", SessionID: "22222222-2222-4222-8222-222222222222", Wallet: wallet, AuthEpoch: 1, ProofAt: time.Now().UTC(), AccessExpiresAt: time.Now().UTC().Add(time.Hour)}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("s", 32) {
			http.Error(w, "service identity required", 401)
			return
		}
		switch r.URL.Path {
		case "/internal/v2/identity/request:authenticate":
			var input auth.IdentityAuthenticationRequest
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Authorization != "Bearer owner-access" {
				http.Error(w, "owner required", 401)
				return
			}
			if err := json.NewEncoder(w).Encode(p); err != nil {
				t.Error(err)
			}
		case "/internal/v2/identity/session:check":
			var input auth.StreamPrincipal
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input != p.StreamPrincipal() {
				http.Error(w, "session required", 403)
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return auth.HTTPIdentityAuthority{Endpoint: server.URL, ServiceToken: strings.Repeat("s", 32), Client: server.Client()}, p
}

func TestLiveWalletEnrollmentReplayAndCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	service, pool := recordedRebuildService(t, ctx)
	inventoryCommittedHead(t, ctx, pool)
	identity, p := inventoryIdentityFixture(t, realMarketCreator)
	worker, err := NewLiveTransferWorker(service, l17.Repository{Pool: pool}, identity, 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, batch := range []int{0, 33} {
		if _, err = NewLiveTransferWorker(service, l17.Repository{Pool: pool}, identity, batch); !errors.Is(err, ErrBaselineUnavailable) {
			t.Fatal(err)
		}
	}
	if _, err = NewLiveTransferWorker(nil, l17.Repository{}, identity, 1); !errors.Is(err, ErrBaselineUnavailable) {
		t.Fatal(err)
	}
	if count, err := worker.RunOnce(ctx); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	if err = worker.EnqueueOwned(ctx, auth.IdentityPrincipal{}); !errors.Is(err, auth.ErrScopeDenied) {
		t.Fatal(err)
	}
	stale := p
	stale.ProofAt = time.Now().Add(-16 * time.Minute)
	if err = worker.EnqueueOwned(ctx, stale); !errors.Is(err, auth.ErrScopeDenied) {
		t.Fatal(err)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	for _, tc := range []struct {
		bearer string
		want   int
	}{{"", 401}, {"Bearer wrong", 401}, {"Bearer owner-access", 204}} {
		req := httptest.NewRequest(http.MethodGet, "/v3/me/portfolio", nil)
		req.Header.Set("Authorization", tc.bearer)
		out := httptest.NewRecorder()
		worker.EnrollmentMiddleware(next).ServeHTTP(out, req)
		if out.Code != tc.want {
			t.Fatal(out.Code, out.Body.String())
		}
	}
	summary, err := worker.RebuildOwned(ctx, p)
	if err != nil || summary.Wallet != p.Wallet || summary.SnapshotID == "" {
		t.Fatal(summary, err)
	}
	active, err := service.Active(ctx, p.Wallet, realMarketToken)
	if err != nil || active.State.Known == "0" || active.Height == 0 {
		t.Fatal(active, err)
	}
	again, err := worker.RebuildOwned(ctx, p)
	if err != nil || again.SnapshotID != summary.SnapshotID {
		t.Fatal(again, err)
	}
	var commits int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM pnl.inventory_l16_live_commits_v1 WHERE wallet=$1", p.Wallet).Scan(&commits); err != nil || commits != 1 {
		t.Fatal(commits, err)
	}
	wrong := p
	wrong.AccountID = "33333333-3333-4333-8333-333333333333"
	if err = worker.CancelOwned(ctx, wrong); err == nil {
		t.Fatal("different account cancelled enrollment")
	}
	if err = worker.CancelOwned(ctx, p); err != nil {
		t.Fatal(err)
	}
	if changed, err := worker.processWallet(ctx, p.Wallet); err != nil || changed {
		t.Fatal(changed, err)
	}
	if err = worker.EnqueueOwned(ctx, p); err != nil {
		t.Fatal(err)
	}
	if changed, err := worker.processWallet(ctx, p.Wallet); err != nil || !changed {
		t.Fatal(changed, err)
	}
}

func TestInventoryMeasurementRunsPortfolioAndActiveReads(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	service, pool := recordedRebuildService(t, ctx)
	inventoryCommittedHead(t, ctx, pool)
	identity, p := inventoryIdentityFixture(t, realMarketCreator)
	worker, err := NewLiveTransferWorker(service, l17.Repository{Pool: pool}, identity, 4)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := worker.RebuildOwned(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	active, err := service.Active(ctx, p.Wallet, realMarketToken)
	if err != nil {
		t.Fatal(err)
	}
	config := pool.Config()
	config.ConnConfig.RuntimeParams["search_path"] = "pnl,pg_catalog"
	reads, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reads.Close)
	api := l17.Service{Repository: l17.Repository{Pool: reads}, Authority: identity, CursorKey: []byte(strings.Repeat("k", 32))}
	server := httptest.NewServer(http.HandlerFunc(api.PortfolioV3HTTP))
	defer server.Close()
	_, plan := measurementPlanFixture(t)
	plan.APIOrigin = server.URL
	plan.Concurrency = 4
	plan.RequestsPerSecond = 10
	plan.MaxDurationSeconds = 30
	for i := range plan.Cases {
		c := &plan.Cases[i]
		c.Wallet = p.Wallet
		c.Token = realMarketToken
		c.Height = active.Height
		c.BlockHash = active.BlockHash
		c.ExpectedState = &active.State
		c.ExpectedNormalizedAverage = nil
		if c.Scenario == 3 {
			decimals := uint8(6)
			average, e := Average(active.State, &decimals)
			if e != nil || average == nil {
				t.Fatal(average, e)
			}
			c.ExpectedNormalizedAverage = str(average.RatString())
		}
		if c.Scenario == 10 {
			response, e := api.Portfolio(ctx, p.Wallet, "", 100, time.Now())
			if e != nil || len(response.Page.Positions) == 0 {
				t.Fatal(response, e)
			}
			c.PricingQuality = response.Page.Positions[0].PricingQuality
			c.ExpectedUnrealized = response.Page.Positions[0].UnrealizedPnlRaw
		}
	}
	plan = bindMeasurementPlan(t, plan)
	output := filepath.Join(t.TempDir(), "measurement")
	manifest, err := RunInventoryMeasurement(ctx, worker, &http.Client{Timeout: 2 * time.Second}, plan, output)
	if err != nil || manifest.Offered != 110 || manifest.Completed != 110 || manifest.Errors != 0 || manifest.UnexpectedErrors != 0 || manifest.GeneratorSaturated != 0 || manifest.OriginSampleCount != 1 || manifest.ServiceSampleCount != 109 || manifest.GlobalAcceptanceQualified || len(manifest.Failures) != 0 {
		t.Fatalf("manifest=%+v error=%v", manifest, err)
	}
	if manifest.ServiceP95NS == nil || manifest.OriginP95NS == nil || manifest.CorrectedP99NS == nil || manifest.PeakHeapAllocBytes == 0 || manifest.InputPlanSHA256 != plan.InputPlanSHA256 {
		t.Fatal(manifest)
	}
	for _, file := range manifest.Artifacts {
		raw, e := measurementReadFile(file, output)
		if e != nil || len(raw) == 0 {
			t.Fatal(file, e)
		}
	}
	raw, err := os.ReadFile(filepath.Join(output, "samples.jsonl"))
	if err != nil || len(strings.Split(strings.TrimSpace(string(raw)), "\n")) != 110 {
		t.Fatal(len(raw), err)
	}

	for _, client := range []*http.Client{nil, {}, {Timeout: 91 * time.Second}} {
		if _, err := RunInventoryMeasurement(ctx, worker, client, plan, filepath.Join(t.TempDir(), "client")); err == nil {
			t.Fatal("unbounded measurement client accepted")
		}
	}
	resourcePlan := plan
	resourcePlan.RunID = "resource-budget"
	resourcePlan.RequestsPerSecond = 800
	resourcePlan.MaxHeapAllocBytes = 1
	resourcePlan = bindMeasurementPlan(t, resourcePlan)
	resourceManifest, err := RunInventoryMeasurement(ctx, worker, &http.Client{Timeout: time.Second}, resourcePlan, filepath.Join(t.TempDir(), "resources"))
	if err == nil || resourceManifest.PeakHeapAllocBytes <= 1 || !strings.Contains(strings.Join(resourceManifest.Failures, ";"), "resource high-water exceeded") {
		t.Fatal(resourceManifest, err)
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	cancelManifest, err := RunInventoryMeasurement(canceled, worker, &http.Client{Timeout: time.Second}, plan, filepath.Join(t.TempDir(), "cancelled"))
	if err == nil || cancelManifest.Offered != 0 || !strings.Contains(strings.Join(cancelManifest.Failures, ";"), "bounded workload did not finish") {
		t.Fatal(cancelManifest, err)
	}
	c := plan.Cases[0]
	c.ExpectedError = expectedParity
	if raw, e := captureInventoryMeasurement(ctx, worker, c, plan.ReleaseDigest, plan.MaxArchiveRecords); !errors.Is(e, delivery.ErrIntegrity) || len(raw) == 0 {
		t.Fatal(string(raw), e)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO pnl.inventory_l16_quarantine_v2(generation,partition,height,block_hash,reason,payload) VALUES($1,$2,$3,$4,'dual_node_or_inventory_parity',$5)`, service.authority.generation, active.Partition, active.Height, active.BlockHash, []byte(`{"parity":"quarantined"}`)); err != nil {
		t.Fatal(err)
	}
	quarantined, e := captureInventoryMeasurement(ctx, worker, c, plan.ReleaseDigest, plan.MaxArchiveRecords)
	var evidence struct {
		CaseID     string `json:"caseId"`
		Quarantine []struct {
			Height  int64           `json:"height"`
			Payload json.RawMessage `json:"payload"`
			SHA256  string          `json:"sha256"`
		} `json:"quarantine"`
		ReleaseQualified bool `json:"releaseQualified"`
	}
	if !errors.Is(e, ErrParity) || json.Unmarshal(quarantined, &evidence) != nil || len(evidence.Quarantine) != 1 || evidence.Quarantine[0].SHA256 != delivery.Hash(evidence.Quarantine[0].Payload) || evidence.ReleaseQualified {
		t.Fatal(string(quarantined), e)
	}
	for _, tc := range []struct {
		name   string
		change func(*InventoryMeasurementCase)
	}{
		{"historical date", func(c *InventoryMeasurementCase) { c.Operation = periodHistoricalDay; c.Day = "invalid" }},
		{"historical partial day", func(c *InventoryMeasurementCase) {
			c.Operation = periodHistoricalDay
			c.Day = active.ObservedThrough.Format("2006-01-02")
		}},
		{"state mismatch", func(c *InventoryMeasurementCase) { state := Empty(); c.ExpectedState = &state }},
		{"absent credentials", func(c *InventoryMeasurementCase) { c.AccessTokenEnv = "INVENTORY_ABSENT_ACCESS" }},
		{"different owner", func(c *InventoryMeasurementCase) { c.Wallet = "0x" + strings.Repeat("e", 40) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := plan.Cases[0]
			tc.change(&c)
			var err error
			if tc.name == "state mismatch" {
				_, err = captureInventoryMeasurement(ctx, worker, c, plan.ReleaseDigest, plan.MaxArchiveRecords)
			} else {
				_, _, _, err = inventoryMeasurementOperation(ctx, worker, &http.Client{Timeout: time.Second}, plan, c)
			}
			if err == nil {
				t.Fatal("inconsistent measurement operation accepted")
			}
		})
	}
	for _, output := range []string{"relative", output} {
		if _, err := RunInventoryMeasurement(ctx, worker, &http.Client{Timeout: time.Second}, plan, output); err == nil {
			t.Fatal("invalid measurement output accepted")
		}
	}
	tiny := plan
	tiny.MaxArtifactBytes = 1
	tiny = bindMeasurementPlan(t, tiny)
	if _, err = RunInventoryMeasurement(ctx, worker, &http.Client{Timeout: time.Second}, tiny, filepath.Join(t.TempDir(), "tiny")); err == nil {
		t.Fatal("exhausted artifact budget accepted")
	}
	if summary.Watermark.Height != strconv.FormatInt(plan.Cases[0].Height, 10) {
		t.Fatal("measurement selected a different committed height")
	}
}

func inventoryCommittedHead(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	tag, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_committed_heads(chain_id,stream_id,height,block_hash,integrity,source_id) SELECT c.chain_id,c.stream_id,c.height,c.block_hash,'healthy','recorded-canonical' FROM indexer.indexer_block_completeness c JOIN indexer.indexer_ingest_watermarks w ON w.chain_id=c.chain_id AND w.stream_id=c.stream_id WHERE c.height=LEAST(w.raw_complete_height,w.typed_complete_height) AND c.fetch_complete AND c.required_decode_complete`)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatal(tag, err)
	}
}
