package control

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresWorkSelectionAndDurableBackoff(t *testing.T) {
	s, _ := fixture(t, nil)
	ctx := context.Background()
	authority := NewLocalCredentialAuthority()
	credential := readySecret(t)
	principal := Principal{ID: id(), CredentialID: id(), Role: "finance", Scopes: []string{"service"}}
	must(t, authority.Grant(credential, principal, time.Now().Add(time.Hour)))
	s.identity = authority.Reader()
	put := func(kind, key string, value any) {
		t.Helper()
		body, err := json.Marshal(value)
		must(t, err)
		_, err = s.pool.Exec(ctx, "INSERT INTO "+s.schema+".objects(kind,id,body) VALUES($1,$2,$3) ON CONFLICT(kind,id) DO UPDATE SET body=EXCLUDED.body", kind, key, body)
		must(t, err)
	}
	for _, key := range []string{"alpha", "beta", "later", "finished"} {
		state := controlPending
		if key == "finished" {
			state = controlSettled
		}
		put(controlSettlement, key, Obligation{ID: id(), Scope: "service", IdempotencyKey: key, State: state})
	}
	put("work_schedule", controlSettlement+".beta", workSchedule{Attempts: 5, Last: time.Now().Add(-time.Hour), Next: time.Now().Add(-time.Minute)})
	put("work_schedule", controlSettlement+".later", workSchedule{Next: time.Now().Add(time.Hour)})
	items, err := s.pendingWork(ctx, controlSettlement)
	must(t, err)
	if len(items) != 2 || items[0].Key != "alpha" || items[1].Key != "beta" || items[0].Scope != "service" {
		t.Fatalf("eligible settlement order: %+v", items)
	}
	claimed, err := s.claimWork(ctx, credential, controlSettlement, controlSettlement)
	must(t, err)
	if len(claimed) != 2 || claimed[0].Key != "alpha" || claimed[1].Key != "beta" {
		t.Fatalf("claimed settlements: %+v", claimed)
	}
	for _, sample := range []struct {
		key      string
		attempts uint
		delay    time.Duration
	}{{"alpha", 1, 30 * time.Second}, {"beta", 5, 5 * time.Minute}} {
		schedule := load[workSchedule](t, s, "work_schedule", controlSettlement+"."+sample.key)
		if schedule.Attempts != sample.attempts || schedule.Next.Sub(schedule.Last) != sample.delay {
			t.Fatalf("settlement backoff %s: %+v", sample.key, schedule)
		}
	}
	items, err = s.pendingWork(ctx, controlSettlement)
	must(t, err)
	if len(items) != 0 {
		t.Fatalf("claimed settlements still eligible: %+v", items)
	}
	for _, key := range []string{"contained", "completed"} {
		put(controlIncident, key, Incident{ID: key, Scope: "service", State: controlContained})
		state := controlPending
		if key == "completed" {
			state = controlCompleted
		}
		put("incident_outbox", key, map[string]string{controlState: state})
	}
	items, err = s.pendingWork(ctx, controlIncident)
	must(t, err)
	if len(items) != 1 || items[0].Key != "contained" || items[0].Scope != "service" {
		t.Fatalf("eligible incidents: %+v", items)
	}
	for _, sample := range []struct{ key, kind, state string }{
		{"queued", controlReplay, "queued"}, {"running", controlReplay, "running"},
		{"done", controlReplay, controlSucceeded}, {"other-kind", controlCapChange, "queued"},
	} {
		put(controlJob, sample.key, ReplayJob{ActionKey: sample.key, Scope: "service", Kind: sample.kind, State: sample.state})
	}
	items, err = s.pendingWork(ctx, controlJob)
	must(t, err)
	if len(items) != 2 || items[0].Key != "queued" || items[1].Key != "running" || items[1].Scope != "service" {
		t.Fatalf("eligible replay jobs: %+v", items)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.pendingWork(cancelled, controlSettlement); err == nil {
		t.Fatal("cancelled database work selection succeeded")
	}
}

func TestLocalSnapshotWriteErrorsKeepMutationAtomic(t *testing.T) {
	for _, obstacle := range []string{"missing parent", "directory destination"} {
		t.Run(obstacle, func(t *testing.T) {
			env := workflowRecoveryEnvironmentFor(t, nil)
			parent := filepath.Dir(env.path)
			if obstacle == "missing parent" {
				must(t, os.Rename(parent, parent+".saved"))
			} else {
				must(t, os.Mkdir(env.path, 0700))
			}
			propose := func() (Action, error) {
				return env.store.Propose(context.Background(), env.credentials["infrastructure"], "atomic-action", controlReplay, "service", "restore projection", strings.Repeat("a", 64), time.Now().Add(time.Hour))
			}
			if _, err := propose(); err == nil {
				t.Fatal("action accepted without a durable snapshot")
			}
			if len(env.store.local.objects) != 0 || len(env.store.local.audits) != 0 {
				t.Fatal("failed snapshot changed objects or audits")
			}
			if obstacle == "missing parent" {
				must(t, os.Rename(parent+".saved", parent))
			} else {
				must(t, os.Remove(env.path))
				files, err := filepath.Glob(filepath.Join(parent, ".control-objects-*"))
				must(t, err)
				if len(files) != 0 {
					t.Fatalf("uncommitted snapshots remain: %v", files)
				}
			}
			action, err := propose()
			must(t, err)
			env.reopen(t)
			got := workflowRecoveryRead[Action](t, env.store, "action", "atomic-action")
			if got.ID != action.ID || got.State != controlProposed || len(env.store.local.audits) != 1 {
				t.Fatalf("durable action after storage recovery: %+v", got)
			}
		})
	}
}

func TestLocalTransactionRejectsUnencodableAndOversizedObjects(t *testing.T) {
	env := workflowRecoveryEnvironmentFor(t, nil)
	ctx := context.Background()
	for _, sample := range []struct {
		name   string
		value  any
		fault  error
		audits int
	}{
		{"unencodable object", make(chan int), nil, 0},
		{"oversized object", strings.Repeat("a", 65536), Invalid, 1},
	} {
		t.Run(sample.name, func(t *testing.T) {
			err := env.store.run(ctx, env.credentials["infrastructure"], "action", "service", "save bounded object", func(tx *transaction) error {
				return tx.put("action", "bounded", sample.value)
			})
			if err == nil || (sample.fault != nil && !errors.Is(err, sample.fault)) {
				t.Fatalf("bounded transaction error: %v", err)
			}
			if len(env.store.local.objects) != 0 || len(env.store.local.audits) != sample.audits {
				t.Fatal("rejected object changed durable data")
			}
		})
	}
	env.reopen(t)
	if len(env.store.local.objects) != 0 || len(env.store.local.audits) != 1 || env.store.local.audits[0].Result != string(Invalid) {
		t.Fatal("object rejection audit did not survive reopening")
	}
	var output Action
	if err := env.store.readObject(ctx, "action", "bounded", &output); !errors.Is(err, Missing) {
		t.Fatalf("rejected object became readable: %v", err)
	}
}

func TestLocalStoredDocumentsRequireExpectedTypes(t *testing.T) {
	env := workflowRecoveryEnvironmentFor(t, nil)
	ctx := context.Background()
	for _, sample := range []struct {
		name, kind, key string
		body            json.RawMessage
	}{
		{"settlement type", controlSettlement, "typed", json.RawMessage(`[]`)},
		{"incident outbox type", "incident_outbox", "typed", json.RawMessage(`[]`)},
		{"schedule type", "work_schedule", controlSettlement + ".typed", json.RawMessage(`[]`)},
	} {
		t.Run(sample.name, func(t *testing.T) {
			objects := map[string]json.RawMessage{
				controlSettlement + "/typed": json.RawMessage(`{"Scope":"service","State":"pending"}`),
				controlIncident + "/typed":   json.RawMessage(`{"Scope":"service","State":"contained"}`),
				"incident_outbox/typed":      json.RawMessage(`{"state":"pending"}`),
			}
			objects[sample.kind+"/"+sample.key] = sample.body
			must(t, env.store.local.save(objects, nil))
			kind := controlSettlement
			if sample.kind == "incident_outbox" {
				kind = controlIncident
			}
			if _, err := env.store.pendingWork(ctx, kind); err == nil {
				t.Fatal("incompatible durable work document admitted")
			}
		})
	}
	before := Action{ID: "unchanged"}
	must(t, env.store.local.save(map[string]json.RawMessage{"action/typed": json.RawMessage(`[]`)}, nil))
	if err := env.store.Read(ctx, env.credentials["infrastructure"], "service", "read typed action", "action", "typed", &before); err == nil || before.ID != "unchanged" || len(env.store.local.audits) != 0 {
		t.Fatalf("incompatible action read: %+v, %v", before, err)
	}
	objects := map[string]json.RawMessage{"quarantine/" + env.actors["infrastructure"]: json.RawMessage(`[]`)}
	must(t, env.store.local.save(objects, nil))
	if err := env.store.Access(ctx, env.credentials["infrastructure"], "service", "inspect quarantine", "support_public", "", false); err == nil {
		t.Fatal("incompatible quarantine admitted")
	}
	if !reflect.DeepEqual(env.store.local.objects, objects) || len(env.store.local.audits) != 0 {
		t.Fatal("incompatible quarantine changed durable data")
	}
}

func TestPostgresIdentityOutageFailsClosedAcrossHTTPRoutes(t *testing.T) {
	env := workflowRecoveryEnvironmentFor(t, nil)
	pool, err := pgxpool.New(context.Background(), "postgres://localhost/control")
	must(t, err)
	pool.Close()
	identity, err := NewPostgresIdentityV2(pool, "control", "identity")
	must(t, err)
	credential := env.credentials["infrastructure"]
	if _, err := identity.Resolve(context.Background(), credential); !errors.Is(err, Unavailable) {
		t.Fatalf("closed identity pool: %v", err)
	}
	if _, err := identity.Resolve(context.Background(), "short"); !errors.Is(err, Denied) {
		t.Fatalf("short identity credential: %v", err)
	}
	if identity.Active(context.Background(), Principal{ID: id(), CredentialID: id()}) || identity.Active(context.Background(), Principal{}) {
		t.Fatal("identity outage admitted active authority")
	}
	if err := identity.Wallet(context.Background(), id(), beneficiary); !errors.Is(err, Unavailable) {
		t.Fatalf("closed wallet authority: %v", err)
	}
	env.store.identity = identity
	for _, sample := range []struct{ method, path string }{
		{http.MethodPost, "actions"}, {http.MethodGet, "campaigns/absent"}, {http.MethodGet, "recommendations/absent"},
	} {
		request := httptest.NewRequest(sample.method, "/v2/control/"+sample.path, strings.NewReader(`{}`))
		request.Header.Set("Authorization", "Bearer "+credential)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		service, err := NewHTTPService(env.store, nil)
		must(t, err)
		service.Handler().ServeHTTP(response, request)
		var fault WorkflowError
		must(t, json.Unmarshal(response.Body.Bytes(), &fault))
		if response.Code != http.StatusServiceUnavailable || fault.Code != "DEPENDENCY_UNAVAILABLE" || !fault.Retryable {
			t.Fatalf("identity outage response %s: %d %+v", sample.path, response.Code, fault)
		}
	}
}

func TestControlJSONValidatesNestedNamesAndDepth(t *testing.T) {
	for _, body := range []string{
		`{"nested":[{"Key":"one","key":"two"}]}`, `{"outer":{"name":1,"NAME":2}}`,
		`{"nested":[}`, `{"nested":{"key":}}`, `{"key"`, `{"key":1`, `{"key":[]`,
		`{"key":1} false`, strings.Repeat("[", 18) + "0" + strings.Repeat("]", 18),
	} {
		if err := rejectDuplicateFields([]byte(body)); err == nil {
			t.Fatalf("ambiguous nested document admitted: %s", body)
		}
	}
	for _, body := range []string{`{"nested":[{"key":1},{"key":2}],"empty":[]}`, `{"nested":{"key":1},"key":2}`, strings.Repeat("[", 16) + "0" + strings.Repeat("]", 16)} {
		if err := rejectDuplicateFields([]byte(body)); err != nil {
			t.Fatalf("distinct nested names refused: %s: %v", body, err)
		}
	}
}

func TestPublicViewsRequireBoundAssetsAndSettlement(t *testing.T) {
	p := policy(id())
	p.Owner, p.State = id(), controlDraft
	for _, edit := range []func(*Policy){
		func(p *Policy) { p.ID = "invalid" },
		func(p *Policy) { p.Instruments = []string{"invalid"} },
		func(p *Policy) { p.Asset = Asset{} },
		func(p *Policy) { p.Mode = "unknown" },
		func(p *Policy) { p.Disclosure = "" },
	} {
		candidate := p
		edit(&candidate)
		if _, err := PublicCampaign(candidate); !errors.Is(err, Invalid) {
			t.Fatalf("invalid public campaign: %v", err)
		}
	}
	p.Asset = pax
	view, err := PublicCampaign(p)
	must(t, err)
	view.AssetAllowlist[0] = beneficiary
	if view.BudgetAsset.Kind != "native" || view.BudgetAsset.Address != nil || view.BudgetAsset.ChainID == nil || *view.BudgetAsset.ChainID != "125" || p.Instruments[0] != USDLAddress {
		t.Fatalf("native campaign view: %+v", view)
	}
	b := bounty(id(), usdl, "1000")
	b.Scope, b.State = "service", controlAwarded
	o := Obligation{BountyID: b.ID, Scope: b.Scope, Asset: b.Asset, Amount: b.Amount, State: controlSettled, SettlementReference: "0x" + strings.Repeat("a", 64)}
	paid, err := PublicBounty(b, beneficiary, &o)
	must(t, err)
	if paid.State != "paid" || paid.SettlementState != controlSettled || paid.AssetAddress != USDLAddress || paid.CreatorWallet != beneficiary {
		t.Fatalf("settled public bounty: %+v", paid)
	}
	for _, edit := range []func(*Bounty, *Obligation){
		func(_ *Bounty, o *Obligation) { o.Amount = "2000" },
		func(_ *Bounty, o *Obligation) { o.SettlementReference = "" },
		func(b *Bounty, _ *Obligation) { b.State = controlActive },
	} {
		candidate, settlement := b, o
		edit(&candidate, &settlement)
		if _, err := PublicBounty(candidate, beneficiary, &settlement); !errors.Is(err, Conflict) {
			t.Fatalf("unbound public settlement: %v", err)
		}
	}
	if _, err := PublicBounty(Bounty{}, beneficiary, nil); !errors.Is(err, Invalid) {
		t.Fatalf("invalid public bounty identifier: %v", err)
	}
	b.Asset = pax
	if _, err := PublicBounty(b, beneficiary, nil); !errors.Is(err, Unavailable) {
		t.Fatalf("native reward public asset: %v", err)
	}
}
