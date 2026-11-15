package rehearsal_test

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/ceremonyfixture"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/migrate"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/rehearsal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/testsupport"
)

func TestWaitingIdentityCountsOnlyRehearsal(t *testing.T) {
	f := ceremonyfixture.NewIdentityStates(t, true)
	before := testsupport.Fingerprint(t, f.DB.DB)
	traffic := f.Delivery.ReceivedBytes()
	options := f.Options
	options.CountsOnly = true
	report, err := rehearsal.Rehearse(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	want := "wallets=5 eligible=3 funded_archived=1 already_migrated=1 deliverable=1 waiting_identity=1 external_identity=1 read=1 verified=1 imported=1 refreshed=1 test_signed=1 matched=1"
	if report.String() != want || report.HeldIdentity != 1 {
		t.Fatalf("counts-only rehearsal: %s", report)
	}
	if info, err := os.Stat(options.CeremonyOptions.RehearsalReceipt); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("protected receipt: %v", err)
	}
	if !reflect.DeepEqual(before, testsupport.Fingerprint(t, f.DB.DB)) || !reflect.DeepEqual(traffic, f.Delivery.ReceivedBytes()) {
		t.Fatal("rehearsal changed its source or contacted delivery attestors")
	}
	f.AssertNoJournal(t, f.Waiting, f.External, f.Funded, f.Migrated)
	f.Isolated.AssertNoKeyRequests(t, migrate.SecpKeyID(f.Waiting), migrate.IdentityKeyID(f.Waiting), migrate.SecpKeyID(f.External), migrate.IdentityKeyID(f.External))
	if databases(t, f.DB) != 0 {
		t.Fatal("rehearsal retained its temporary database")
	}
}

func TestWaitingIdentityNoDeliverableRehearsalAndDelivery(t *testing.T) {
	f := ceremonyfixture.NewIdentityStates(t, false)
	before := testsupport.Fingerprint(t, f.DB.DB)
	deliveryTraffic, rehearsalTraffic := f.Delivery.ReceivedBytes(), f.Isolated.ReceivedBytes()
	options := f.Options
	options.CountsOnly = true
	report, err := rehearsal.Rehearse(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	want := "wallets=5 eligible=3 funded_archived=1 already_migrated=1 deliverable=0 waiting_identity=2 external_identity=1 read=0 verified=0 imported=0 refreshed=0 test_signed=0 matched=0"
	if report.String() != want || report.HeldIdentity != 0 {
		t.Fatalf("empty rehearsal counts: %s", report)
	}
	if info, err := os.Stat(options.CeremonyOptions.RehearsalReceipt); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("empty rehearsal receipt: %v", err)
	}
	plan, err := migrate.PlanMigration(context.Background(), f.DB.DB)
	if err != nil {
		t.Fatal(err)
	}
	counts, err := migrate.Deliver(context.Background(), f.DB.DB, f.Client(t), options.MasterKey, plan, f.DeliveryOptions())
	if err != nil || counts != (migrate.Report{WaitingIdentity: 2, ExternalIdentity: 1}) {
		t.Fatalf("empty delivery counts: %+v, %v", counts, err)
	}
	if !reflect.DeepEqual(deliveryTraffic, f.Delivery.ReceivedBytes()) || !reflect.DeepEqual(rehearsalTraffic, f.Isolated.ReceivedBytes()) {
		t.Fatal("empty ceremony contacted an attestor")
	}
	if !reflect.DeepEqual(before, testsupport.Fingerprint(t, f.DB.DB)) {
		t.Fatal("empty ceremony changed its source")
	}
	f.AssertNoJournal(t, f.Deliverable, f.Waiting, f.External, f.Funded, f.Migrated)
}
