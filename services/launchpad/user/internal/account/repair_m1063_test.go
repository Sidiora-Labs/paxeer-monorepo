package account

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/migrations"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// This persistence peer stores opaque production records. All authorization,
// claim, fencing, expiry and consumption decisions run in exportDownloads.
type repairM1063Records struct {
	mu           sync.Mutex
	file         *os.File
	job, account uuid.UUID
}

func repairM1063OpenRecords(t *testing.T, path string, job, account uuid.UUID) *repairM1063Records {
	t.Helper()
	file, err := os.OpenFile(path, os.O_RDWR, 0600)
	must(t, err)
	return &repairM1063Records{file: file, job: job, account: account}
}

func (r *repairM1063Records) read() (exportDownloadRecord, error) {
	var record exportDownloadRecord
	if _, err := r.file.Seek(0, io.SeekStart); err != nil {
		return record, err
	}
	err := json.NewDecoder(r.file).Decode(&record)
	return record, err
}

func (r *repairM1063Records) update(ctx context.Context, job, account uuid.UUID, mutate func(*exportDownloadRecord) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if job != r.job || account != r.account {
		return ErrDenied
	}
	record, err := r.read()
	if err != nil {
		return err
	}
	if err = mutate(&record); err != nil {
		return err
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if _, err = r.file.WriteAt(raw, 0); err != nil {
		return err
	}
	if err = r.file.Truncate(int64(len(raw))); err != nil {
		return err
	}
	return r.file.Sync()
}

func (r *repairM1063Records) snapshot(t *testing.T) exportDownloadRecord {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	record, err := r.read()
	must(t, err)
	return record
}

// The loopback object peer speaks the HTTP operations used by storage.Client;
// it retains the actual encrypted bytes emitted by the production worker.
type repairM1063Objects struct {
	mu              sync.Mutex
	objects         map[string][]byte
	outage, corrupt bool
	reads           int
}

func (o *repairM1063Objects) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	o.mu.Lock()
	defer o.mu.Unlock()
	switch r.Method {
	case http.MethodPut:
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		o.objects[r.URL.Path] = raw
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		o.reads++
		if o.outage {
			http.Error(w, "object unavailable", http.StatusServiceUnavailable)
			return
		}
		raw, ok := o.objects[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if o.corrupt {
			raw = []byte("corrupt ciphertext")
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprint(len(raw)))
		_, _ = w.Write(raw)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (o *repairM1063Objects) failure(outage, corrupt bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.outage, o.corrupt = outage, corrupt
}

type repairM1063Export struct {
	records              *repairM1063Records
	objects              *repairM1063Objects
	objectServer, server *httptest.Server
	runtime              *Runtime
	job                  Job
	now                  time.Time
	path                 string
	snapshot             json.RawMessage
}

func repairM1063NewExport(t *testing.T) *repairM1063Export {
	t.Helper()
	f := &repairM1063Export{now: time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC), path: filepath.Join(t.TempDir(), "export.json"), objects: &repairM1063Objects{objects: map[string][]byte{}}}
	f.objectServer = httptest.NewServer(f.objects)
	t.Cleanup(f.close)
	client := s3.NewFromConfig(aws.Config{Region: "test", Credentials: credentials.NewStaticCredentialsProvider("fixture", "fixture", ""), HTTPClient: f.objectServer.Client(), RetryMaxAttempts: 1}, func(o *s3.Options) { o.BaseEndpoint = aws.String(f.objectServer.URL); o.UsePathStyle = true })
	f.runtime = &Runtime{Objects: storage.NewFromS3(client, "exports"), ExportKey: bytes.Repeat([]byte{63}, 32)}
	f.job = Job{ID: uuid.New(), AccountID: uuid.New(), Kind: "export", State: "ready", RequestedAt: f.now}
	// These are the actual account export document and worker publication paths.
	var err error
	f.snapshot, err = exportSnapshotDocument(Profile{PublicProfile: PublicProfile{Wallet: fmt.Sprintf("0x%040x", 1063), Revision: "0"}, SchemaVersion: 1, UpdatedAt: f.now}, initialPreferences(f.now), []WatchEntry{}, []map[string]any{})
	must(t, err)
	worker := Worker{Runtime: f.runtime}
	object, err := worker.writeExportObject(context.Background(), jobLease{Job: f.job, Fence: 1}, f.snapshot)
	must(t, err)
	payload, hash, err := newExportMetadata(f.runtime.ExportKey, f.job.ID, object, f.now)
	must(t, err)
	expires := f.now.Add(15 * time.Minute)
	raw, err := json.Marshal(exportDownloadRecord{Payload: payload, Hash: hash[:], ExpiresAt: &expires, State: "ready"})
	must(t, err)
	must(t, os.WriteFile(f.path, raw, 0600))
	f.records = repairM1063OpenRecords(t, f.path, f.job.ID, f.job.AccountID)
	return f
}

func (f *repairM1063Export) downloads() exportDownloads {
	return exportDownloads{records: f.records, now: func() time.Time { return f.now }}
}

func (f *repairM1063Export) start(t *testing.T) (*http.Client, string) {
	t.Helper()
	if f.server != nil {
		t.Fatal("service already running")
	}
	h := HTTP{Store: Store{Clock: func() time.Time { return f.now }}, Runtime: f.runtime}
	mux := http.NewServeMux()
	mux.Handle("GET /v2/me/exports/{jobId}/download", h.downloadHandler(f.downloads()))
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		record := f.records.snapshot(t)
		serveV2Owner(w, r, func(w http.ResponseWriter, r *http.Request) {
			h.respondExportStatus(w, r, f.job, record.ExpiresAt, record.Payload)
		}, func(raw []byte) ([]byte, error) { return AdaptV2AccountJob(raw, "export", true) })
	})
	f.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := Principal{AccountID: f.job.AccountID, ProofAt: f.now}
		if r.Header.Get("X-Test-Other-Account") != "" {
			p.AccountID = uuid.New()
		}
		mux.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), p)))
	}))
	f.runtime.DownloadBaseURL = f.server.URL
	return f.server.Client(), f.server.URL
}

func (f *repairM1063Export) reopen(t *testing.T) {
	t.Helper()
	if f.server != nil {
		f.server.Close()
		f.server = nil
	}
	must(t, f.records.file.Close())
	f.records = repairM1063OpenRecords(t, f.path, f.job.ID, f.job.AccountID)
}

func (f *repairM1063Export) close() {
	if f.server != nil {
		f.server.Close()
		f.server = nil
	}
	if f.objectServer != nil {
		f.objectServer.Close()
		f.objectServer = nil
	}
	if f.records != nil {
		_ = f.records.file.Close()
	}
}

func repairM1063Request(t *testing.T, client *http.Client, target string, headers map[string]string, status int) []byte {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, target, nil)
	must(t, err)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	response, err := client.Do(req)
	must(t, err)
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	must(t, err)
	if response.StatusCode != status {
		t.Fatalf("%s: status %d, want %d: %s", req.URL.Path, response.StatusCode, status, raw)
	}
	if response.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatal("cacheable export response")
	}
	return raw
}

func repairM1063DownloadURL(t *testing.T, f *repairM1063Export, client *http.Client, base string) *url.URL {
	t.Helper()
	raw := repairM1063Request(t, client, base+"/status", nil, http.StatusOK)
	response, err := v2.DecodeAccountExportStatusResponse(raw)
	must(t, err)
	if response.Data.DownloadUrl == nil {
		t.Fatal("ready URL missing")
	}
	target, err := url.Parse(*response.Data.DownloadUrl)
	must(t, err)
	return target
}

func TestRepairM_1063_export_failed_read_consumes_token(t *testing.T) {
	for _, failure := range []string{"storage outage", "corrupt ciphertext"} {
		t.Run(failure, func(t *testing.T) {
			f := repairM1063NewExport(t)
			client, base := f.start(t)
			target := repairM1063DownloadURL(t, f, client, base)
			f.objects.failure(failure == "storage outage", failure == "corrupt ciphertext")
			raw := repairM1063Request(t, client, target.String(), nil, http.StatusServiceUnavailable)
			if bytes.Contains(raw, []byte(`"preferences"`)) {
				t.Fatal("failed retrieval produced export bytes")
			}
			record := f.records.snapshot(t)
			if record.ConsumedAt != nil || record.Owner != nil || record.Until != nil {
				t.Fatal("failed retrieval consumed or retained claim", record)
			}
			f.reopen(t)
			f.objects.failure(false, false)
			client, base = f.start(t)
			plain := repairM1063Request(t, client, base+target.RequestURI(), nil, http.StatusOK)
			if !bytes.Equal(plain, f.snapshot) {
				t.Fatal("wrong recovered export", string(plain))
			}
			f.reopen(t)
			client, base = f.start(t)
			repairM1063Request(t, client, base+target.RequestURI(), nil, http.StatusForbidden)
			if f.records.snapshot(t).ConsumedAt == nil {
				t.Fatal("consumption was not durable")
			}
		})
	}
}

func TestRepairM_1063_abandoned_claim_recovery_and_fencing(t *testing.T) {
	f := repairM1063NewExport(t)
	client, base := f.start(t)
	target := repairM1063DownloadURL(t, f, client, base)
	p := Principal{AccountID: f.job.AccountID}
	old, err := f.downloads().claim(context.Background(), p, f.job.ID, target.Query().Get("authorization"))
	must(t, err)
	record := f.records.snapshot(t)
	if record.Until == nil || !record.Until.Equal(f.now.Add(time.Minute)) || record.Fence != old.Fence || record.ConsumedAt != nil {
		t.Fatal("claim is not durable and bounded", record)
	}
	f.reopen(t)
	client, base = f.start(t)
	repairM1063Request(t, client, base+target.RequestURI(), nil, http.StatusForbidden)
	f.server.Close()
	f.server = nil
	f.now = f.now.Add(exportDownloadLease)
	newer, err := f.downloads().claim(context.Background(), p, f.job.ID, target.Query().Get("authorization"))
	must(t, err)
	if newer.Fence <= old.Fence {
		t.Fatal("recovery reused fence")
	}
	if err = f.downloads().consume(context.Background(), old); !errors.Is(err, ErrDenied) {
		t.Fatal("stale claim consumed export", err)
	}
	must(t, f.downloads().release(context.Background(), old))
	record = f.records.snapshot(t)
	if record.Owner == nil || *record.Owner != newer.Owner {
		t.Fatal("stale cleanup released new claim")
	}
	must(t, f.downloads().release(context.Background(), newer))
	client, base = f.start(t)
	repairM1063Request(t, client, base+target.RequestURI(), nil, http.StatusOK)
	f.reopen(t)
	client, base = f.start(t)
	repairM1063Request(t, client, base+target.RequestURI(), nil, http.StatusForbidden)
}

func TestRepairM_1063_claim_refusals_and_concurrent_delivery(t *testing.T) {
	f := repairM1063NewExport(t)
	client, base := f.start(t)
	target := repairM1063DownloadURL(t, f, client, base)
	const count = 12
	codes := make(chan int, count)
	for i := 0; i < count; i++ {
		go func() {
			response, err := client.Get(target.String())
			if err != nil {
				codes <- 0
				return
			}
			_, err = io.Copy(io.Discard, response.Body)
			response.Body.Close()
			if err != nil {
				codes <- 0
				return
			}
			codes <- response.StatusCode
		}()
	}
	delivered := 0
	for i := 0; i < count; i++ {
		switch code := <-codes; code {
		case http.StatusOK:
			delivered++
		case http.StatusForbidden:
		default:
			t.Fatalf("unexpected concurrent status %d", code)
		}
	}
	if delivered != 1 {
		t.Fatalf("successful deliveries %d, want 1", delivered)
	}
	f.reopen(t)
	client, base = f.start(t)
	repairM1063Request(t, client, base+target.RequestURI(), nil, http.StatusForbidden)
	for _, reason := range []string{"expired", "deleted", "wrong state", "wrong token", "cancelled", "nil account"} {
		t.Run(reason, func(t *testing.T) {
			f := repairM1063NewExport(t)
			record := f.records.snapshot(t)
			h := HTTP{Store: Store{Clock: func() time.Time { return f.now }}, Runtime: f.runtime}
			h.Runtime.DownloadBaseURL = "https://example.com"
			projection, err := h.exportStatusProjection(f.job, record.ExpiresAt, record.Payload)
			must(t, err)
			authorization := *projection.DownloadAuthorization
			ctx := context.Background()
			p := Principal{AccountID: f.job.AccountID}
			switch reason {
			case "expired":
				f.now = *record.ExpiresAt
			case "deleted", "wrong state":
				must(t, f.records.update(ctx, f.job.ID, p.AccountID, func(r *exportDownloadRecord) error {
					if reason == "deleted" {
						r.Deleted = true
					} else {
						r.State = "failed"
					}
					return nil
				}))
			case "wrong token":
				authorization = strings.Repeat("0", 64)
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "nil account":
				p.AccountID = uuid.Nil
			}
			if _, err = f.downloads().claim(ctx, p, f.job.ID, authorization); err == nil {
				t.Fatal("invalid claim allowed")
			}
			if f.records.snapshot(t).ConsumedAt != nil {
				t.Fatal("refusal consumed export")
			}
		})
	}
}

func TestRepairM_1063_provisioned_privacy_default(t *testing.T) {
	at := time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC)
	gateway := initialPreferences(at)
	backfill := initialPreferences(at)
	if !reflect.DeepEqual(gateway, backfill) || !gateway.PrivateActivity || gateway.QuietHours != (QuietHours{Timezone: "UTC", Start: "22:00", End: "07:00"}) || !gateway.UpdatedAt.Equal(at) {
		t.Fatal("provisioning defaults drifted", gateway, backfill)
	}
	raw, err := json.Marshal(gateway)
	must(t, err)
	var reopened Preferences
	must(t, json.Unmarshal(raw, &reopened))
	if !reflect.DeepEqual(reopened, gateway) {
		t.Fatal("preference persistence changed defaults")
	}
	wallet := fmt.Sprintf("0x%040x", 1063)
	original := state{ID: uuid.New(), Wallet: wallet, Profile: Profile{PublicProfile: PublicProfile{Wallet: wallet, Revision: "0"}, SchemaVersion: 1, UpdatedAt: at}, Preferences: initialPreferences(at.Add(time.Microsecond))}
	original.Preferences.PrivateActivity = false
	original.Preferences.QuietHours = QuietHours{Timezone: "UTC", Start: "00:00", End: "00:00"}
	recovered := original
	if !reconcileProvisionedPreferences(&recovered, false, at.Add(time.Hour)) || !recovered.Preferences.PrivateActivity || recovered.Preferences.QuietHours != gateway.QuietHours || !recovered.Preferences.UpdatedAt.Equal(at.Add(time.Hour)) {
		t.Fatal("untouched auto-provisioned defaults not repaired", recovered)
	}
	if reconcileProvisionedPreferences(&recovered, false, at.Add(2*time.Hour)) {
		t.Fatal("reconciliation was not idempotent")
	}
	for _, choice := range []string{"revision", "locale", "timezone", "currency", "chart", "channels", "categories", "analytics", "quiet hours", "privacy", "timestamp", "profile", "deleted", "consent history"} {
		t.Run(choice, func(t *testing.T) {
			a := original
			switch choice {
			case "revision":
				a.Revision = 1
				a.Preferences.Revision = "1"
			case "locale":
				a.Preferences.Locale = "fr"
			case "timezone":
				a.Preferences.Timezone = "Europe/Paris"
			case "currency":
				a.Preferences.Currency = "EUR"
			case "chart":
				a.Preferences.ChartInterval = "4h"
			case "channels":
				a.Preferences.Channels = []string{"push"}
			case "categories":
				a.Preferences.NotificationCategories = map[string]bool{"trade": true}
			case "analytics":
				a.Preferences.AnalyticsConsent = true
			case "quiet hours":
				a.Preferences.QuietHours.Start = "01:00"
			case "privacy":
				a.Preferences.PrivateActivity = true
			case "timestamp":
				a.Preferences.UpdatedAt = time.Time{}
			case "profile":
				a.Profile.DisplayName = "Chosen name"
			case "deleted":
				a.Deleted = true
			}
			before, err := json.Marshal(a)
			must(t, err)
			if reconcileProvisionedPreferences(&a, choice == "consent history", at.Add(time.Hour)) {
				t.Fatal("explicit choice overwritten")
			}
			after, err := json.Marshal(a)
			must(t, err)
			if !bytes.Equal(before, after) {
				t.Fatal("reconciliation mutated choices")
			}
		})
	}
	// Both transactional entry points must use the shared initializer, rather
	// than testing two calls that could mask drift in a provisioning caller.
	for _, source := range []struct{ file, function string }{{"preferences_http.go", "ensureIdentityAccount"}, {"backfill.go", "BackfillLegacyRange"}, {"privacy.go", "Backfill"}, {"store.go", "Create"}} {
		file, err := parser.ParseFile(token.NewFileSet(), source.file, nil, 0)
		must(t, err)
		calls := 0
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == source.function {
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					if call, ok := node.(*ast.CallExpr); ok {
						if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "initialPreferences" {
							calls++
						}
					}
					return true
				})
			}
		}
		if calls != 1 {
			t.Fatalf("%s.%s uses initializer %d times", source.file, source.function, calls)
		}
	}
}

func TestRepairM_1063_deletion_fixture_schema_source(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "v2_owner_http_test.go", nil, 0)
	must(t, err)
	found := false
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "v2OwnerDeletionFixture" {
			for _, stmt := range fn.Body.List {
				if ret, ok := stmt.(*ast.ReturnStmt); ok && len(ret.Results) == 1 {
					if call, ok := ret.Results[0].(*ast.CallExpr); ok {
						if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "connectedLegacyFixture" {
							found = true
						}
					}
				}
			}
		}
	}
	if !found {
		t.Fatal("deletion fixture must use the connected tracked fixture")
	}
	ddl, err := migrations.FS.ReadFile("00002_account_store.sql")
	must(t, err)
	for _, table := range []string{"account_state", "account_job"} {
		if !bytes.Contains(ddl, []byte("CREATE TABLE "+table+" (")) {
			t.Fatal("tracked user schema missing", table)
		}
	}
	identity := connectedIdentityDDL(t)
	for _, table := range []string{"identity_accounts", "identity_sessions", "identity_reauth"} {
		if !strings.Contains(identity, "CREATE TABLE "+table+"(") {
			t.Fatal("tracked identity schema missing", table)
		}
	}
	h := HTTP{Store: Store{Clock: func() time.Time { return time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC) }}, Runtime: &Runtime{EndpointKey: bytes.Repeat([]byte{63}, 32)}}
	job := Job{ID: uuid.New(), AccountID: uuid.New(), Kind: "deletion", State: "processing", RequestedAt: h.Store.now()}
	projection := deletionProjection(job)
	receipt, err := h.recoveryReceipt(projection.Job)
	must(t, err)
	raw, err := json.Marshal(struct {
		Job     Job
		Receipt string
	}{job, receipt})
	must(t, err)
	path := filepath.Join(t.TempDir(), "deletion.json")
	must(t, os.WriteFile(path, raw, 0600))
	reopened, err := os.Open(path)
	must(t, err)
	var recovered struct {
		Job     Job
		Receipt string
	}
	must(t, json.NewDecoder(reopened).Decode(&recovered))
	must(t, reopened.Close())
	recovery := httptest.NewRequest(http.MethodGet, "/v2/me/deletion-requests/"+job.ID.String(), nil)
	recovery.SetPathValue("jobId", recovered.Job.ID.String())
	recovery.Header.Set("X-Recovery-Receipt", recovered.Receipt)
	principal, err := h.recoveryPrincipal(recovery)
	must(t, err)
	if principal.AccountID != job.AccountID {
		t.Fatal("recovery lost owner")
	}
	response := v2OwnerTestEnvelope(t, deletionProjection(recovered.Job), "0")
	adapted, err := AdaptV2AccountJob(response, "deletion", false)
	must(t, err)
	decoded, err := v2.DecodeAccountDeleteResponse(adapted)
	must(t, err)
	if decoded.Data.Job.JobId != job.ID.String() || len(decoded.Data.RetentionSummary) != 2 || len(decoded.Data.Job.DomainResults) != 4 {
		t.Fatal("deletion consumer lost recovery state", decoded)
	}
	recovery.SetPathValue("jobId", uuid.NewString())
	if _, err = h.recoveryPrincipal(recovery); !errors.Is(err, ErrDenied) {
		t.Fatal("recovery accepted other job", err)
	}
}

func TestRepairM_1063_completed_export_fixture(t *testing.T) {
	testCompletedExportParticipant(t)
	// Preserve all malformed-envelope, field, kind and unsafe-URL rejections.
	TestAdaptV2AccountJobsMatchFrozenSchema(t)
}

// The connected proof keeps the real PostgreSQL producers and mutation-history
// assertions available without adding infrastructure to the focused gate.
func TestConnectedRepairM1063ProvisionedPrivacyDefaults(t *testing.T) {
	s := connectedLegacyFixture(t)
	at := s.now()
	s.Clock = func() time.Time { return at }
	ctx := context.Background()
	p := Principal{AccountID: uuid.New(), ProofAt: at}
	session := uuid.New()
	wallet := fmt.Sprintf("0x%040x", 10631)
	seedConnectedIdentity(t, s, p, session, wallet)
	gateway := sharedauth.Principal{Wallet: wallet, SessionID: session.String(), Roles: []string{"owner"}}
	must(t, s.ensureIdentityAccount(ctx, sharedauth.PostgresIdentityResolver{Pool: s.DB}, gateway))
	gatewayPreferences, err := s.ReadPreferences(ctx, p, p.AccountID)
	must(t, err)
	other := Principal{AccountID: uuid.New(), ProofAt: at}
	otherWallet := fmt.Sprintf("0x%040x", 10632)
	seedConnectedIdentity(t, s, other, uuid.New(), otherWallet)
	_, err = s.DB.Exec(ctx, `INSERT INTO users.user_profiles(wallet_address,display_name,bio,created_at,updated_at) VALUES($1,'Backfilled','Biography',1,1)`, otherWallet)
	must(t, err)
	counts, err := s.BackfillLegacyRange(ctx, BackfillRange{ID: "privacy-defaults", Limit: 100})
	must(t, err)
	if counts.Verified != 1 || counts.Quarantined != 0 {
		t.Fatal(counts)
	}
	backfillPreferences, err := s.ReadPreferences(ctx, other, other.AccountID)
	must(t, err)
	if !reflect.DeepEqual(gatewayPreferences, backfillPreferences) || !gatewayPreferences.PrivateActivity || !gatewayPreferences.UpdatedAt.Equal(at) {
		t.Fatal(gatewayPreferences, backfillPreferences)
	}
	legacy := initialPreferences(at.Add(time.Microsecond))
	legacy.PrivateActivity = false
	legacy.QuietHours = QuietHours{Timezone: "UTC", Start: "00:00", End: "00:00"}
	raw, err := json.Marshal(legacy)
	must(t, err)
	_, err = s.DB.Exec(ctx, `UPDATE account_state SET preferences=$2 WHERE account_id=$1`, p.AccountID, raw)
	must(t, err)
	must(t, s.ensureIdentityAccount(ctx, sharedauth.PostgresIdentityResolver{Pool: s.DB}, gateway))
	repaired, err := s.ReadPreferences(ctx, p, p.AccountID)
	must(t, err)
	if !reflect.DeepEqual(repaired, gatewayPreferences) {
		t.Fatal("untouched row did not reconcile", repaired)
	}
	private := false
	must(t, s.PatchExtendedPreferences(ctx, p, ExtendedPreferencesPatch{PreferencesPatch: PreferencesPatch{ExpectedRevision: repaired.Revision}, PrivateActivity: &private}))
	chosen, err := s.ReadPreferences(ctx, p, p.AccountID)
	must(t, err)
	s = repair76Reopen(t, s)
	must(t, s.ensureIdentityAccount(ctx, sharedauth.PostgresIdentityResolver{Pool: s.DB}, gateway))
	persisted, err := s.ReadPreferences(ctx, p, p.AccountID)
	must(t, err)
	if !reflect.DeepEqual(chosen, persisted) || persisted.PrivateActivity {
		t.Fatal("explicit privacy choice overwritten", persisted)
	}
}
