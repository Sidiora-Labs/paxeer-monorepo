package migrate_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/ceremonyfixture"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/migrate"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/testsupport"
)

func TestWaitingIdentityDeliveryPreservesOtherWallets(t *testing.T) {
	ctx := context.Background()
	f := ceremonyfixture.NewIdentityStates(t, true)
	plan, err := migrate.PlanMigration(ctx, f.DB.DB)
	if err != nil {
		t.Fatal(err)
	}
	f.AssertPlan(t, plan)
	retained := map[string]string{}
	for _, id := range []string{f.Waiting, f.External, f.Funded, f.Migrated} {
		retained[id] = f.WalletState(t, id)
	}
	path := filepath.Join(t.TempDir(), "inventory.json")
	if count, err := plan.WriteInventoryRequests(ctx, f.DB.DB, f.Options.MasterKey, path); err != nil || count != 2 {
		t.Fatalf("deliverable inventory: count=%d error=%v", count, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document migrate.InventoryRequests
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Requests) != 2 || document.Requests[0].KeyID != migrate.SecpKeyID(f.Deliverable) || document.Requests[1].KeyID != plan.Deliverable[0].OriginalIdentityKeyID {
		t.Fatal("inventory includes keys outside the deliverable set")
	}
	f.Rehearse(t)
	client := f.Client(t)
	report, err := migrate.Deliver(ctx, f.DB.DB, client, f.Options.MasterKey, plan, f.DeliveryOptions())
	want := migrate.Report{Read: 1, Verified: 1, Imported: 1, Refreshed: 1, TestSigned: 1, HeldIdentity: 1, Matched: 1, WaitingIdentity: 1, ExternalIdentity: 1}
	if err != nil || report != want {
		t.Fatalf("delivery counts: %+v, %v", report, err)
	}
	f.AssertMigrated(t, []string{f.Deliverable})
	for id, state := range retained {
		if f.WalletState(t, id) != state {
			t.Fatal("delivery changed a retained wallet row or provisioning record")
		}
	}
	f.AssertNoJournal(t, f.Waiting, f.External, f.Funded, f.Migrated)
	for _, id := range []string{f.Waiting, f.External, f.Funded, f.Migrated} {
		f.Delivery.AssertNoKeyRequests(t, migrate.SecpKeyID(id), migrate.IdentityKeyID(id))
		f.Isolated.AssertNoKeyRequests(t, migrate.SecpKeyID(id), migrate.IdentityKeyID(id))
	}
	incomplete := *plan
	incomplete.Eligible = append(append([]migrate.Wallet{}, plan.Eligible...), plan.WaitingIdentity[0])
	if err := migrate.WriteRehearsalReceipt(*f.Options.CeremonyOptions, &incomplete, client, report); !errors.Is(err, migrate.ErrRehearsal) {
		t.Fatalf("incomplete partition accepted for receipt: %v", err)
	}
	report.HeldIdentity = 0
	if err := migrate.WriteRehearsalReceipt(*f.Options.CeremonyOptions, plan, client, report); !errors.Is(err, migrate.ErrRehearsal) {
		t.Fatalf("incomplete identity custody accepted for receipt: %v", err)
	}
}

func TestWaitingIdentityReceiptAndLaterCeremony(t *testing.T) {
	ctx := context.Background()
	f := ceremonyfixture.NewIdentityStates(t, true)
	f.Rehearse(t)
	client := f.Client(t)
	plan, err := migrate.PlanMigration(ctx, f.DB.DB)
	if err != nil {
		t.Fatal(err)
	}
	if report, err := migrate.Deliver(ctx, f.DB.DB, client, f.Options.MasterKey, plan, f.DeliveryOptions()); err != nil || report.Matched != 1 {
		t.Fatalf("first delivery: %+v, %v", report, err)
	}
	firstState := f.WalletState(t, f.Deliverable)
	f.Provision(t, f.Waiting)
	client.Close()
	client = f.Client(t)
	plan, err = migrate.PlanMigration(ctx, f.DB.DB)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Deliverable) != 1 || plan.Deliverable[0].ID != f.Waiting || len(plan.WaitingIdentity) != 0 || plan.AlreadyMigrated != 2 {
		t.Fatalf("new identity plan: %+v", plan)
	}
	before := f.Delivery.ReceivedBytes()
	state := f.WalletState(t, f.Waiting)
	if _, err = migrate.Deliver(ctx, f.DB.DB, client, f.Options.MasterKey, plan, f.DeliveryOptions()); !errors.Is(err, migrate.ErrRehearsal) {
		t.Fatalf("earlier receipt accepted a different deliverable set: %v", err)
	}
	if !reflect.DeepEqual(before, f.Delivery.ReceivedBytes()) || f.WalletState(t, f.Waiting) != state {
		t.Fatal("receipt refusal contacted an attestor or changed a wallet")
	}
	f.AssertNoJournal(t, f.Waiting, f.External)
	earlierID := f.Options.CeremonyOptions.CeremonyID
	earlierReceipt := f.Options.CeremonyOptions.RehearsalReceipt
	f.NextCeremony(t, earlierID+"-next")
	if f.Options.CeremonyOptions.CeremonyID == earlierID || f.Options.CeremonyOptions.RehearsalReceipt == earlierReceipt {
		t.Fatal("later ceremony reused its identifier or receipt")
	}
	f.Rehearse(t)
	report, err := migrate.Deliver(ctx, f.DB.DB, client, f.Options.MasterKey, plan, f.DeliveryOptions())
	want := migrate.Report{Read: 1, Verified: 1, Imported: 1, Refreshed: 1, TestSigned: 1, HeldIdentity: 1, Matched: 1, ExternalIdentity: 1}
	if err != nil || report != want {
		t.Fatalf("later delivery: %+v, %v", report, err)
	}
	f.AssertMigrated(t, []string{f.Deliverable, f.Waiting})
	if f.WalletState(t, f.Deliverable) != firstState {
		t.Fatal("later ceremony changed the earlier migrated wallet")
	}
	remaining, err := migrate.PlanMigration(ctx, f.DB.DB)
	if err != nil || remaining.AlreadyMigrated != 3 || len(remaining.Deliverable) != 0 || len(remaining.WaitingIdentity) != 0 || len(remaining.ExternalIdentity) != 1 {
		t.Fatalf("later plan counts: %+v, %v", remaining, err)
	}
	f.AssertNoJournal(t, f.Deliverable, f.External)
}

func TestWaitingIdentityInvalidOriginalStopsDelivery(t *testing.T) {
	ctx := context.Background()
	f := ceremonyfixture.NewIdentityStates(t, true)
	client := f.Client(t)
	plan, err := migrate.PlanMigration(ctx, f.DB.DB)
	if err != nil {
		t.Fatal(err)
	}
	original := plan.Deliverable[0]
	for _, state := range []string{"inactive", "active_without_row_identity", "missing_record"} {
		t.Run(state, func(t *testing.T) {
			switch state {
			case "inactive":
				_, err = f.DB.DB.Exec(`update account_provisioning set state='bind_signed' where wallet_id=$1::uuid`, f.Deliverable)
			case "active_without_row_identity":
				_, err = f.DB.DB.Exec(`update account_provisioning set state='active' where wallet_id=$1::uuid`, f.Deliverable)
				if err == nil {
					_, err = f.DB.DB.Exec(`update wallets set did=null,layerx_key_id=null where id=$1::uuid`, f.Deliverable)
				}
			case "missing_record":
				_, err = f.DB.DB.Exec(`update wallets set did=$2,layerx_key_id=$3 where id=$1::uuid`, f.Deliverable, original.DID, original.OriginalIdentityKeyID)
				if err == nil {
					_, err = f.DB.DB.Exec(`delete from account_provisioning where wallet_id=$1::uuid`, f.Deliverable)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			plan, err := migrate.PlanMigration(ctx, f.DB.DB)
			if err != nil {
				t.Fatal(err)
			}
			f.AssertPlan(t, plan)
			before := f.Delivery.ReceivedBytes()
			row := f.WalletState(t, f.Deliverable)
			options := f.DeliveryOptions()
			options.Rehearsal = true
			report, err := migrate.Deliver(ctx, f.DB.DB, client, f.Options.MasterKey, plan, options)
			var walletError *migrate.WalletError
			if !errors.Is(err, migrate.ErrIdentityRecovery) || !errors.As(err, &walletError) || walletError.WalletID != f.Deliverable || walletError.Stage != "original identity" {
				t.Fatalf("invalid original identity: %v", err)
			}
			if report != (migrate.Report{Read: 1, WaitingIdentity: 1, ExternalIdentity: 1}) || f.WalletState(t, f.Deliverable) != row || !reflect.DeepEqual(before, f.Delivery.ReceivedBytes()) {
				t.Fatal("invalid identity delivery changed custody, counts or a wallet row")
			}
			f.AssertNoJournal(t, f.Waiting, f.External)
		})
	}
}

func TestWaitingIdentityPlanProvisioningStates(t *testing.T) {
	ctx := context.Background()
	pg := testsupport.StartPostgres(t)
	testsupport.ApplyGatewayMigrations(t, pg.DB)
	_, master := testsupport.MasterKey(t)
	vectors := testsupport.GatewayVectors(t, master, 6)
	ids := make([]string, len(vectors))
	for i, vector := range vectors {
		kind := "standard"
		if i == 4 {
			kind = "agent"
		}
		ids[i] = testsupport.InsertWallet(t, pg.DB, vector, kind)
	}
	wallets, err := migrate.ReadWallets(ctx, pg.DB)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range []struct {
		owner, wallet int
		state         string
	}{{0, 0, "identity"}, {1, 1, "active"}, {2, 1, "active"}, {3, 2, "refused"}} {
		if _, err := pg.DB.Exec(`insert into account_provisioning(user_id,kind,wallet_id,state) values($1::uuid,'standard',$2::uuid,$3)`, wallets[record.owner].UserID, ids[record.wallet], record.state); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pg.DB.Exec(`update wallets set did=$2 where id=$1::uuid`, ids[2], "did:layerx:"+ids[2]); err != nil {
		t.Fatal(err)
	}
	if _, err := pg.DB.Exec(`update wallets set layerx_key_id=$2 where id=$1::uuid`, ids[3], migrate.IdentityKeyID(ids[3])); err != nil {
		t.Fatal(err)
	}
	plan, err := migrate.PlanMigration(ctx, pg.DB)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Total != 6 || len(plan.Eligible) != 6 || len(plan.Deliverable) != 3 || len(plan.WaitingIdentity) != 2 || len(plan.ExternalIdentity) != 1 {
		t.Fatalf("provisioning state plan: %+v", plan)
	}
	for i, id := range []string{ids[1], ids[2], ids[3]} {
		if plan.Deliverable[i].ID != id {
			t.Fatal("identity metadata was passed over or reordered")
		}
	}
	if plan.WaitingIdentity[0].ID != ids[0] || plan.WaitingIdentity[1].ID != ids[5] || plan.ExternalIdentity[0].ID != ids[4] {
		t.Fatal("waiting or external identity set changed")
	}
}
