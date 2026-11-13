package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/account"
	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/internaltest"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
)

const notificationEligibilityToken = "repair-77-private-service-token-with-32-characters"

type notificationEligibilityResult struct {
	Preference accountDeliveryPreference
	Target     accountPushTarget
	Targets    []accountPushTarget
	Decision   struct {
		Send            bool
		Reason          string
		ConsentRevision uint64
	}
	Resolved       bool
	NotificationID string
	Deliveries     []struct {
		State, Reason string
		Attempt       int
	}
}

type notificationEligibilityFixture struct {
	t                               *testing.T
	store                           account.Store
	principal                       account.Principal
	session, device                 uuid.UUID
	runtime                         *account.Runtime
	pki                             *testpki.PKI
	serverLeaf, clientLeaf          testpki.Leaf
	server                          *httptest.Server
	consumerPath, notificationsPath string
}

func mustNotificationEligibility(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newNotificationEligibilityFixture(t *testing.T) *notificationEligibilityFixture {
	t.Helper()
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	// Apply the real identity migration, using its own goose version table.
	db := stdlib.OpenDB(*pool.Config().ConnConfig)
	versions, err := database.NewStore(database.DialectPostgres, "repair_77_identity_version")
	mustNotificationEligibility(t, err)
	provider, err := goose.NewProvider("", db, os.DirFS("../../../gateway/internal/devidentity/migrations"), goose.WithStore(versions))
	mustNotificationEligibility(t, err)
	_, err = provider.Up(ctx)
	mustNotificationEligibility(t, err)
	mustNotificationEligibility(t, db.Close())
	f := &notificationEligibilityFixture{t: t, store: account.Store{DB: pool}, principal: account.Principal{AccountID: uuid.New(), ProofAt: time.Now()}, session: uuid.New(), device: uuid.New(), pki: testpki.New(t)}
	f.runtime = &account.Runtime{EndpointKey: bytes.Repeat([]byte{77}, 32), ExtendedV3: true}
	ddl, err := delivery.SchemaDDL("public")
	mustNotificationEligibility(t, err)
	_, err = pool.Exec(ctx, ddl)
	mustNotificationEligibility(t, err)
	f.wireDelivery()
	f.serverLeaf = f.pki.Leaf("user", "user.test")
	f.clientLeaf = f.pki.Leaf("notifications", "notifications.test")
	f.consumerPath, err = filepath.Abs("testdata/notification_consumer/main.go")
	mustNotificationEligibility(t, err)
	f.notificationsPath, err = filepath.Abs("../../../notifications")
	mustNotificationEligibility(t, err)
	mustNotificationEligibility(t, f.store.Create(ctx, f.principal.AccountID, "0x7777777777777777777777777777777777777777"))
	_, err = pool.Exec(ctx, `INSERT INTO identity_accounts VALUES($1,1,clock_timestamp())`, f.principal.AccountID)
	mustNotificationEligibility(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO identity_wallet_links VALUES($1,$2,125,'0x7777777777777777777777777777777777777777','private','verified',clock_timestamp())`, uuid.New(), f.principal.AccountID)
	mustNotificationEligibility(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO identity_sessions(session_id,account_id,wallet,auth_epoch,access_hash,csrf_hash,issued_at,access_expires_at,refresh_expires_at,reauthenticated_at,last_seen_at,device_label) VALUES($1,$2,'0x7777777777777777777777777777777777777777',1,$3,$4,clock_timestamp(),clock_timestamp()+interval '1 hour',clock_timestamp()+interval '1 day',clock_timestamp(),clock_timestamp(),'repair77')`, f.session, f.principal.AccountID, []byte(uuid.NewString()), []byte(uuid.NewString()))
	mustNotificationEligibility(t, err)
	// Real HTTP preference mutations persist every category's consent. The
	// notification client below consumes only the internal authority's output.
	consents := []account.ConsentPatch{}
	for _, category := range []string{"trade", "watchlist", "creator", "chat", "security", "marketing"} {
		for _, channel := range []string{"push", "inapp", "email"} {
			consents = append(consents, account.ConsentPatch{Category: category, Channel: channel, Enabled: true})
		}
	}
	f.patch("/v3/me/preferences", map[string]any{"expectedRevision": "0", "channels": map[string]bool{"push": true, "inapp": true, "email": true}, "consents": consents, "quietHours": account.QuietHours{Timezone: "UTC", Start: "00:00", End: "00:00"}})
	// Persist an already registered installation with the real endpoint codec;
	// identity mutations intentionally leave these endpoint rows untouched.
	sealed, err := account.Seal(f.runtime.EndpointKey, []byte("repair-77-device-token"), []byte(f.principal.AccountID.String()))
	mustNotificationEligibility(t, err)
	fingerprint := sha256.Sum256([]byte("repair-77-device-token"))
	_, err = pool.Exec(ctx, `INSERT INTO account_device_installation VALUES($1,$2,$3,'fcm','android',clock_timestamp(),false)`, f.device, f.principal.AccountID, f.session)
	mustNotificationEligibility(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO account_endpoint VALUES($1,$2,$3,'android',1,clock_timestamp(),false,$4,'fcm',$5)`, fingerprint[:], f.principal.AccountID, sealed, f.device, f.session)
	mustNotificationEligibility(t, err)
	f.open()
	t.Cleanup(func() { f.server.Close(); f.store.DB.Close() })
	return f
}

func (f *notificationEligibilityFixture) open() {
	f.server = httptest.NewUnstartedServer(accountNotificationAuthority(f.store, f.runtime, notificationEligibilityToken, f.clientLeaf.SHA256))
	f.server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{f.serverLeaf.Pair}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: f.pki.Roots()}
	f.server.StartTLS()
}

func (f *notificationEligibilityFixture) wireDelivery() {
	f.t.Helper()
	f.runtime.Subjects = map[string]string{"preferences.updated": "repair.account.preferences.updated", "consent.updated": "repair.account.consent.updated"}
	store, err := delivery.NewStore(f.store.DB, "public", delivery.Policy{Subjects: map[string]bool{"repair.account.preferences.updated": true, "repair.account.consent.updated": true}, EventTypes: map[string]bool{"preferences.updated": true, "consent.updated": true}})
	mustNotificationEligibility(f.t, err)
	f.runtime.Delivery = store
}

func (f *notificationEligibilityFixture) reopen() {
	f.t.Helper()
	f.server.Close()
	cfg := f.store.DB.Config().Copy()
	f.store.DB.Close()
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	mustNotificationEligibility(f.t, err)
	f.store = account.Store{DB: pool}
	f.wireDelivery()
	f.open()
}

func (f *notificationEligibilityFixture) patch(path string, body any) {
	f.t.Helper()
	raw, err := json.Marshal(body)
	mustNotificationEligibility(f.t, err)
	h := (account.HTTP{Store: f.store, Runtime: f.runtime}).Handler()
	request := httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(raw))
	request = request.WithContext(account.WithPrincipal(request.Context(), f.principal))
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		f.t.Fatalf("PATCH %s: %d %s", path, response.Code, response.Body)
	}
}

func (f *notificationEligibilityFixture) consume(action, notification string, stale accountPushTarget) notificationEligibilityResult {
	f.t.Helper()
	in := map[string]any{"DSN": f.store.DB.Config().ConnString(), "Base": f.server.URL, "CAFile": f.pki.CAFile, "CertFile": f.clientLeaf.CertFile, "KeyFile": f.clientLeaf.KeyFile, "AccountID": f.principal.AccountID.String(), "DeviceID": f.device.String(), "Action": action, "NotificationID": notification, "Target": stale}
	raw, err := json.Marshal(in)
	mustNotificationEligibility(f.t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "run", "-mod=readonly", f.consumerPath)
	command.Dir = f.notificationsPath
	command.Env = append(os.Environ(), "GOWORK=off", "REPAIR_77_SERVICE_TOKEN="+notificationEligibilityToken)
	command.Stdin = bytes.NewReader(raw)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err = command.Run(); err != nil {
		f.t.Fatalf("real notification consumer: %v\n%s", err, &stderr)
	}
	var out notificationEligibilityResult
	mustNotificationEligibility(f.t, json.Unmarshal(stdout.Bytes(), &out))
	return out
}

func (f *notificationEligibilityFixture) enqueue() notificationEligibilityResult {
	f.t.Helper()
	out := f.consume("enqueue", uuid.NewString(), accountPushTarget{})
	if !out.Decision.Send || !out.Resolved || out.Target.Status != "active" || len(out.Targets) != 1 || out.Targets[0].Status != "active" || len(out.Deliveries) != 1 || out.Deliveries[0].State != "pending" {
		f.t.Fatalf("eligible notification did not reach the real outbox: %+v", out)
	}
	return out
}

func assertNotificationSuppressed(t *testing.T, out notificationEligibilityResult, reason string) {
	t.Helper()
	if len(out.Deliveries) != 1 || out.Deliveries[0].State != "suppressed" || out.Deliveries[0].Reason != reason || out.Deliveries[0].Attempt != 1 {
		t.Fatalf("worker did not persist %s refusal: %+v", reason, out)
	}
}

func TestNotificationSessionRevocation(t *testing.T) {
	cases := []struct{ name, sql string }{
		{"logout", `UPDATE identity_sessions SET revoked_at=clock_timestamp() WHERE session_id=$1`},
		{"epoch_change", `UPDATE identity_accounts SET auth_epoch=auth_epoch+1 WHERE account_id=(SELECT account_id FROM identity_sessions WHERE session_id=$1)`},
		{"session_replacement", `UPDATE account_device_installation SET session_id=$2 WHERE session_id=$1`},
		{"access_expiry", `UPDATE identity_sessions SET access_expires_at=clock_timestamp()-interval '1 second' WHERE session_id=$1`},
		{"wallet_revoked", `UPDATE identity_wallet_links SET state='revoked' WHERE account_id=(SELECT account_id FROM identity_sessions WHERE session_id=$1) AND wallet=(SELECT wallet FROM identity_sessions WHERE session_id=$1)`},
		{"installation_expiry", `UPDATE account_device_installation SET verified_at=clock_timestamp()-interval '31 days' WHERE session_id=$1`},
		{"account_logout", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newNotificationEligibilityFixture(t)
			if tc.name == "wallet_revoked" {
				// Keep account-level linkage valid so the worker must reach the
				// target check for the session's specific wallet, even while
				// another verified wallet still belongs to the account.
				_, err := f.store.DB.Exec(context.Background(), `INSERT INTO identity_wallet_links VALUES($1,$2,125,'0x8888888888888888888888888888888888888888','private','verified',clock_timestamp())`, uuid.New(), f.principal.AccountID)
				mustNotificationEligibility(t, err)
			}
			first, second := f.enqueue(), f.enqueue()
			if tc.name == "account_logout" {
				tx, err := f.store.DB.Begin(context.Background())
				mustNotificationEligibility(t, err)
				defer tx.Rollback(context.Background())
				mustNotificationEligibility(t, sharedauth.RevokeIdentityAccountTx(context.Background(), tx, f.principal.AccountID.String(), "logout"))
				mustNotificationEligibility(t, tx.Commit(context.Background()))
			} else {
				args := []any{f.session}
				if strings.Contains(tc.sql, "$2") {
					args = append(args, uuid.New())
				}
				_, err := f.store.DB.Exec(context.Background(), tc.sql, args...)
				mustNotificationEligibility(t, err)
			}
			for i, pending := range []notificationEligibilityResult{first, second} {
				if i == 1 {
					f.reopen()
				}
				out := f.consume("work", pending.NotificationID, pending.Target)
				assertNotificationSuppressed(t, out, "target_revoked")
				if out.Target.Status != "revoked" || len(out.Targets) != 1 || out.Targets[0].Status != "revoked" || out.Resolved || !out.Decision.Send {
					t.Fatalf("session invalidation not enforced by target read/list/resolve: %+v", out)
				}
				eligible, err := f.store.EndpointEligible(context.Background(), f.principal.AccountID, f.device, "chat", "push", 1)
				mustNotificationEligibility(t, err)
				if eligible {
					t.Fatal("target eligibility disagrees with account predicate")
				}
			}
		})
	}
}

func TestNotificationChannelOptOut(t *testing.T) {
	for _, path := range []string{"/v1/me/preferences", "/v2/me/preferences", "/v3/me/preferences"} {
		t.Run(path, func(t *testing.T) {
			f := newNotificationEligibilityFixture(t)
			first, second := f.enqueue(), f.enqueue()
			f.patch(path, map[string]any{"expectedRevision": "1", "channels": map[string]bool{"push": false, "inapp": true, "email": true}})
			for i, pending := range []notificationEligibilityResult{first, second} {
				if i == 1 {
					f.reopen()
				}
				out := f.consume("work", pending.NotificationID, accountPushTarget{})
				assertNotificationSuppressed(t, out, "opted_out")
				if out.Decision.Send || out.Decision.Reason != "opted_out" || out.Preference.Revision != 2 || out.Preference.Channels["push"] || !out.Preference.Channels["email"] || !out.Preference.Channels["inapp"] {
					t.Fatalf("global push opt-out not carried to participant: %+v", out)
				}
				for category, channels := range out.Preference.CategoryChannels {
					if channels["push"] || !channels["email"] || !channels["inapp"] {
						t.Fatalf("effective channels for %s: %v", category, channels)
					}
				}
				var count int
				mustNotificationEligibility(t, f.store.DB.QueryRow(context.Background(), `SELECT count(*) FROM account_consent WHERE account_id=$1 AND channel='push' AND enabled`, f.principal.AccountID).Scan(&count))
				if count != 6 {
					t.Fatalf("category consents changed: %d", count)
				}
				preferences, err := f.store.ReadPreferences(context.Background(), f.principal, f.principal.AccountID)
				mustNotificationEligibility(t, err)
				if preferences.Revision != strconv.FormatUint(out.Preference.Revision, 10) {
					t.Fatal("HTTP preference revision differs from persisted revision")
				}
			}
		})
	}
}
