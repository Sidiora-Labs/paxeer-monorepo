package migrate_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/attestor"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/ceremonyfixture"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/envelope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/migrate"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/testsupport"
)

func TestPlanInventoryRequestsDelivery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	pg := testsupport.StartPostgres(t)
	testsupport.ApplyGatewayMigrations(t, pg.DB)
	master, masterB64 := testsupport.MasterKey(t)
	vectors := testsupport.GatewayVectors(t, masterB64, 5)
	first := testsupport.InsertWallet(t, pg.DB, vectors[0], "standard")
	second := testsupport.InsertWallet(t, pg.DB, vectors[1], "standard")
	testsupport.InsertWallet(t, pg.DB, vectors[2], "funded")
	funded := testsupport.InsertWallet(t, pg.DB, vectors[3], "standard")
	testsupport.InsertFundedAccount(t, pg.DB, funded)
	migrated := testsupport.InsertWallet(t, pg.DB, vectors[4], "standard")
	testsupport.MarkMigrated(t, pg.DB, migrated, migrate.SecpKeyID(migrated))
	fixture := ceremonyfixture.New(t, pg, master, vectors)
	fixture.Delivery.PublishInventory(t, nil)
	fixture.Isolated.PublishInventory(t, nil)
	plan, err := migrate.PlanMigration(ctx, pg.DB)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Total != 5 || len(plan.Eligible) != 2 || len(plan.Funded) != 2 || plan.AlreadyMigrated != 1 {
		t.Fatalf("unexpected plan counts: %+v", plan)
	}
	path := filepath.Join(t.TempDir(), "requests.json")
	count, err := plan.WriteInventoryRequests(ctx, pg.DB, master, path)
	if err != nil || count != 4 {
		t.Fatalf("write requests: count=%d err=%v", count, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("requests permissions: %v, %v", info, err)
	}
	var document struct {
		Requests []testsupport.CeremonyInventoryKey `json:"inventory_requests"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&document); err != nil {
		t.Fatal(err)
	}
	want := map[string]testsupport.CeremonyInventoryKey{}
	for _, w := range plan.Eligible {
		var vector testsupport.Vector
		switch w.ID {
		case first:
			vector = vectors[0]
		case second:
			vector = vectors[1]
		default:
			t.Fatalf("unexpected eligible wallet %s", w.ID)
		}
		private, err := crypto.HexToECDSA(vector.Key)
		if err != nil {
			t.Fatal(err)
		}
		public := hex.EncodeToString(crypto.FromECDSAPub(&private.PublicKey))
		secpID := migrate.SecpKeyID(w.ID)
		want[secpID] = testsupport.CeremonyInventoryKey{KeyID: secpID, Epoch: 0, Curve: attestor.CurveSecp256k1, PublicKey: public, Owner: w.UserID, Operations: []string{"import", "refresh", "sign"}}
		var identity, identityPublic string
		if err = pg.DB.QueryRowContext(ctx, `select ed_key_id,ed_public_key from account_provisioning where wallet_id=$1::uuid`, w.ID).Scan(&identity, &identityPublic); err != nil {
			t.Fatal(err)
		}
		want[identity] = testsupport.CeremonyInventoryKey{KeyID: identity, Epoch: 0, Curve: attestor.CurveEd25519, PublicKey: identityPublic, Owner: w.UserID, Operations: []string{"sign", "refresh"}}
		if bytes.Contains(raw, []byte(vector.Key)) || bytes.Contains(raw, []byte(vector.Envelope)) {
			t.Fatal("requests contain private key material")
		}
	}
	if len(document.Requests) != len(want) {
		t.Fatalf("got %d requests, want %d", len(document.Requests), len(want))
	}
	for _, request := range document.Requests {
		if expected, ok := want[request.KeyID]; !ok || !reflect.DeepEqual(request, expected) {
			t.Fatalf("unexpected inventory request: %+v; want %+v", request, expected)
		}
		delete(want, request.KeyID)
	}
	if len(want) != 0 {
		t.Fatalf("missing requests: %+v", want)
	}
	var fields struct {
		Requests []map[string]json.RawMessage `json:"inventory_requests"`
	}
	if err = json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for _, entry := range fields.Requests {
		for _, name := range []string{"key_id", "epoch", "curve", "public_key", "owner", "operations"} {
			if _, ok := entry[name]; !ok {
				t.Fatalf("request omits %s", name)
			}
		}
		if len(entry) != 6 {
			t.Fatalf("request has %d fields, want 6", len(entry))
		}
	}

	t.Run("invalid wallet preserves document", func(t *testing.T) {
		invalid := *plan
		invalid.Eligible = append([]migrate.Wallet(nil), plan.Eligible...)
		invalid.Eligible[1].EncryptedPrivateKey = "invalid"
		if n, err := invalid.WriteInventoryRequests(ctx, pg.DB, master, path); n != 0 || !errors.Is(err, envelope.ErrFormat) {
			t.Fatalf("invalid envelope: count=%d err=%v", n, err)
		}
		preserved, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(preserved, raw) {
			t.Fatalf("invalid wallet changed requests: %v", err)
		}
	})

	approved := make([][]testsupport.CeremonyInventoryKey, attestor.NodeCount)
	for i := range approved {
		approved[i] = document.Requests
	}
	fixture.Delivery.PublishInventory(t, approved)
	fixture.Isolated.PublishInventory(t, approved)
	fixture.Rehearse(t)
	client, err := attestor.New(fixture.Delivery.Config(t))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	options := *fixture.Options.CeremonyOptions
	options.Rehearsal = false
	options.JournalDir = fixture.Options.DeliveryJournalDir
	report, err := migrate.Deliver(ctx, pg.DB, client, master, plan, options)
	if err != nil {
		t.Fatalf("delivery: %v (%+v)", err, report)
	}
	if report.Read != 2 || report.Verified != 2 || report.Imported != 2 || report.Refreshed != 2 || report.TestSigned != 2 || report.HeldIdentity != 2 || report.Matched != 2 {
		t.Fatalf("incomplete delivery: %+v", report)
	}
	for i := 0; i < attestor.NodeCount; i++ {
		fixture.Delivery.Reopen(t, i)
	}
	fixture.AssertMigrated(t, []string{first, second})
	remaining, err := migrate.PlanMigration(ctx, pg.DB)
	if err != nil || len(remaining.Eligible) != 0 || len(remaining.Funded) != 2 || remaining.AlreadyMigrated != 3 {
		t.Fatalf("plan after delivery: %+v, %v", remaining, err)
	}
}

func TestPlanInventoryRequestsPublication(t *testing.T) {
	ctx := context.Background()
	plan := &migrate.Plan{}
	if _, err := plan.WriteInventoryRequests(ctx, nil, nil, "requests.json"); err == nil {
		t.Fatal("relative requests path accepted")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "requests.json")
	if err := os.WriteFile(path, []byte("previous document"), 0644); err != nil {
		t.Fatal(err)
	}
	previous := filepath.Join(dir, "previous.json")
	if err := os.Link(path, previous); err != nil {
		t.Fatal(err)
	}
	count, err := plan.WriteInventoryRequests(ctx, nil, nil, path)
	if err != nil || count != 0 {
		t.Fatalf("empty requests: count=%d err=%v", count, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "{\n  \"inventory_requests\": []\n}\n" {
		t.Fatalf("empty requests document: %q, %v", raw, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("requests permissions: %v, %v", info, err)
	}
	old, err := os.ReadFile(previous)
	if err != nil || string(old) != "previous document" {
		t.Fatalf("previous inode changed: %q, %v", old, err)
	}
	if _, err = plan.WriteInventoryRequests(ctx, nil, nil, dir); err == nil {
		t.Fatal("directory accepted as requests file")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("unexpected publication files: %v, %v", entries, err)
	}
}
