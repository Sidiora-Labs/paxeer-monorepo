package l17

import (
	"bytes"
	"context"
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/go-chi/chi/v5"
)

func portfolioIdentity(t *testing.T) (auth.HTTPIdentityAuthority, auth.IdentityPrincipal) {
	t.Helper()
	now := time.Now().UTC()
	account, err := opaqueID()
	if err != nil {
		t.Fatal(err)
	}
	session, err := opaqueID()
	if err != nil {
		t.Fatal(err)
	}
	p := auth.IdentityPrincipal{AccountID: account, SessionID: session, Wallet: wallet, AuthEpoch: 1, ProofAt: now, AccessExpiresAt: now.Add(time.Hour)}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("s", 32) {
			http.Error(w, "service credentials required", 401)
			return
		}
		var body auth.IdentityAuthenticationRequest
		if r.URL.Path != "/internal/v2/identity/request:authenticate" || json.NewDecoder(r.Body).Decode(&body) != nil || body.Authorization != "Bearer portfolio-owner" {
			http.Error(w, "owner credentials required", 401)
			return
		}
		if err := json.NewEncoder(w).Encode(p); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	return auth.HTTPIdentityAuthority{Endpoint: server.URL, ServiceToken: strings.Repeat("s", 32), Client: server.Client()}, p
}

func portfolioRequest(t *testing.T, router http.Handler, method, path string, body any, want int) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	var err error
	if body != nil {
		raw, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer portfolio-owner")
	req.Header.Set("Idempotency-Key", "http-operation")
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != want {
		t.Fatalf("%s %s: status=%d want=%d body=%s", method, path, response.Code, want, response.Body.String())
	}
	return response
}

func TestOwnerHTTPPortfolioHistoryExportAndCard(t *testing.T) {
	ctx := context.Background()
	repository := phase2Repository(t)
	summary := phase2Commit(t, repository, "owner-http-source", "1", 2)
	six := 6
	record := HistoryRecord{EventID: "http-deposit", Kind: "deposit", Asset: AssetRef{Kind: "erc20", ChainID: ptr("125"), Address: ptr(token), Decimals: &six}, AmountRaw: "1000000", BasisQuality: "unknown", Position: ChainPosition{ChainID: "125", Height: "99", BlockHash: summary.Watermark.BlockHash, TransactionHash: "0x" + strings.Repeat("b", 64), TransactionIndex: "0", LogIndex: "0"}}
	if err := repository.CommitHistory(ctx, wallet, summary.SnapshotID, "owner-http-history", []HistoryRecord{record}); err != nil {
		t.Fatal(err)
	}
	metadataTx, err := repository.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err = repository.CommitHistoryMetadataTx(ctx, metadataTx, wallet, summary.SnapshotID, SnapshotHistoryMetadata{SourceEvent: "http-history-range", Start: now.Add(-time.Hour), End: now, Quote: fixture().MarkPrice.QuoteAsset}); err != nil {
		t.Fatal(err)
	}
	if err = metadataTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	authority, _ := portfolioIdentity(t)
	service := Service{Repository: repository, Authority: authority, CursorKey: []byte(strings.Repeat("k", 32)), ExportsDir: t.TempDir()}
	if err := os.Chmod(service.ExportsDir, 0700); err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	service.RegisterHTTP(router)
	router.Get("/v3/me/portfolio", service.PortfolioV3HTTP)
	router.Get("/v3/wallets/{wallet}/portfolio", service.PublicPortfolioV3HTTP)
	for _, path := range []string{"/v2/me/portfolio", "/v2/wallets/" + wallet + "/portfolio", "/v3/me/portfolio", "/v3/wallets/" + wallet + "/portfolio"} {
		out := portfolioRequest(t, router, "GET", path, nil, 200)
		if out.Header().Get("Server-Timing") == "" || !bytes.Contains(out.Body.Bytes(), []byte(summary.SnapshotID)) {
			t.Fatal(out.Header(), out.Body.String())
		}
	}
	from, err := time.Parse(time.RFC3339Nano, summary.Watermark.ObservedAt)
	if err != nil {
		t.Fatal(err)
	}
	query := url.Values{"from": {from.Add(-time.Hour).Format("2006-01-02T15:04:05.000Z")}, "to": {from.Add(time.Hour).Format("2006-01-02T15:04:05.000Z")}, "interval": {"1h"}, "limit": {"10"}}
	out := portfolioRequest(t, router, "GET", "/v2/me/portfolio/history?"+query.Encode(), nil, 200)
	if !bytes.Contains(out.Body.Bytes(), []byte(`"historicalReturn"`)) || !bytes.Contains(out.Body.Bytes(), []byte(`"items"`)) {
		t.Fatal(out.Body.String())
	}
	for _, path := range []string{"/v2/me/portfolio?limit=x", "/v2/me/portfolio?limit=1&limit=2", "/v2/me/portfolio?unknown=1", "/v3/me/portfolio?limit=x", "/v3/me/portfolio?unknown=1", "/v2/me/portfolio/history?from=bad", "/v2/me/portfolio/history?unknown=1"} {
		portfolioRequest(t, router, "GET", path, nil, 400)
	}
	out = portfolioRequest(t, router, "POST", "/v2/me/portfolio/exports", map[string]any{"snapshotId": summary.SnapshotID, "format": "jsonl"}, 202)
	var job ExportJob
	if err = json.Unmarshal(out.Body.Bytes(), &job); err != nil || job.State != "queued" {
		t.Fatal(job, err)
	}
	portfolioRequest(t, router, "GET", "/v2/me/portfolio/exports/"+job.ID, nil, 200)
	if err = (ExportWorker{Repository: repository, Directory: service.ExportsDir, Owner: "http-export"}).RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	out = portfolioRequest(t, router, "GET", "/v2/me/portfolio/exports/"+job.ID+"/download", nil, 200)
	if out.Header().Get("Content-Type") != "application/x-ndjson" || !bytes.Contains(out.Body.Bytes(), []byte(record.EventID)) {
		t.Fatal(out.Header(), out.Body.String())
	}
	portfolioRequest(t, router, "DELETE", "/v2/me/portfolio/exports/"+job.ID, nil, 202)
	out = portfolioRequest(t, router, "POST", "/v2/me/portfolio/cards", map[string]any{"snapshotId": summary.SnapshotID, "includeWallet": false}, 202)
	var card StoredCard
	if err = json.Unmarshal(out.Body.Bytes(), &card); err != nil || card.ID == "" {
		t.Fatal(card, err)
	}
	portfolioRequest(t, router, "GET", "/v2/me/portfolio/cards/"+card.ID, nil, 200)
	worker := CardWorker{Repository: repository, Owner: "http-card", FontDir: filepath.Join("..", "..", "ogrender", "fonts")}
	if err = worker.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	out = portfolioRequest(t, router, "GET", "/v2/portfolio/cards/"+card.ID+"/image.png", nil, 200)
	image, err := png.DecodeConfig(bytes.NewReader(out.Body.Bytes()))
	if err != nil || image.Width != 1200 || image.Height != 630 {
		t.Fatal(image, err)
	}
	portfolioRequest(t, router, "DELETE", "/v2/me/portfolio/cards/"+card.ID, nil, 202)
	portfolioRequest(t, router, "GET", "/v2/portfolio/cards/"+card.ID+"/image.png", nil, 410)
	for _, path := range []string{"/v2/me/portfolio/exports/invalid", "/v2/me/portfolio/cards/invalid", "/v2/portfolio/cards/invalid/image.png"} {
		portfolioRequest(t, router, "GET", path, nil, 404)
	}
	for _, path := range []string{"/v2/me/portfolio/exports", "/v2/me/portfolio/cards"} {
		portfolioRequest(t, router, "POST", path, map[string]any{"unknown": true}, 400)
	}
}

func TestSelectedShareOwnerHTTPPublicationAndPrivacy(t *testing.T) {
	service, _, summary := sharingV3Fixture(t)
	authority, principal := portfolioIdentity(t)
	service.authority = authority
	router := chi.NewRouter()
	service.RegisterHTTP(router)
	expiry := time.Now().UTC().Add(time.Hour)
	selection := ShareSelectionV3{SnapshotID: summary.SnapshotID, Fields: []string{"knownRealizedSubtotalUsdlRaw", "realizedPnlRaw", "completeValueUsdlRaw", "historicalReturn"}, IncludeWallet: true, ExpiresAt: expiry}
	out := portfolioRequest(t, router, "POST", "/v3/me/portfolio/shares/preview", selection, 200)
	var preview SharePreviewV3
	if err := json.Unmarshal(out.Body.Bytes(), &preview); err != nil || preview.Share.Wallet == nil || *preview.Share.Wallet != principal.Wallet {
		t.Fatal(preview, err)
	}
	portfolioRequest(t, router, "POST", "/v3/me/portfolio/shares", SharePublishV3{ID: preview.ID, Intent: preview.Intent}, 202)
	portfolioRequest(t, router, "GET", "/v3/me/portfolio/shares/"+preview.ID, nil, 200)
	worker := ShareWorkerV3{Repository: service.repository, Owner: "http-share", FontDir: filepath.Join("..", "..", "ogrender", "fonts")}
	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	out = portfolioRequest(t, router, "GET", "/v3/portfolio/shares/"+preview.ID, nil, 200)
	if bytes.Contains(out.Body.Bytes(), []byte(principal.AccountID)) || bytes.Contains(out.Body.Bytes(), []byte(principal.SessionID)) {
		t.Fatal("private identity exposed", out.Body.String())
	}
	image := portfolioRequest(t, router, "GET", "/v3/portfolio/shares/"+preview.ID+"/image.png", nil, 200)
	if config, err := png.DecodeConfig(bytes.NewReader(image.Body.Bytes())); err != nil || config.Width != 1200 {
		t.Fatal(config, err)
	}
	portfolioRequest(t, router, "DELETE", "/v3/me/portfolio/shares/"+preview.ID, nil, 202)
	portfolioRequest(t, router, "GET", "/v3/portfolio/shares/"+preview.ID, nil, 410)
	for _, body := range []map[string]any{
		{"snapshotId": summary.SnapshotID, "fields": selection.Fields, "expiresAt": nil},
		{"snapshotId": summary.SnapshotID, "fields": selection.Fields, "expiresAt": expiry, "includeWallet": nil},
		{"snapshotId": summary.SnapshotID, "fields": selection.Fields, "expiresAt": "invalid"},
	} {
		portfolioRequest(t, router, "POST", "/v3/me/portfolio/shares/preview", body, 400)
	}
	for _, path := range []string{"/v3/me/portfolio/shares/preview?unknown=1", "/v3/portfolio/shares/" + preview.ID + "?unknown=1", "/v3/portfolio/shares/" + preview.ID + "/image.png?unknown=1"} {
		method := "GET"
		if strings.Contains(path, "preview") {
			method = "POST"
		}
		portfolioRequest(t, router, method, path, selection, 400)
	}
}
