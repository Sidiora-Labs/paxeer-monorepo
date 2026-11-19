package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/archive"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/ceremonyfixture"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/migrate"
)

func TestWaitingIdentityCommands(t *testing.T) {
	f := ceremonyfixture.NewIdentityStates(t, true)
	env := f.CommandEnvironment(t)
	code, out, errOut := invoke(env, "plan")
	if code != 0 || out != "wallets=5 eligible=3 funded=1 already_migrated=1 deliverable=1 waiting_identity=1 external_identity=1\n" {
		t.Fatalf("plan counts: exit=%d out=%q error=%s", code, out, errOut)
	}
	path := filepath.Join(t.TempDir(), "inventory.json")
	code, out, errOut = invoke(env, "plan", "--inventory-requests", path)
	if code != 0 || out != "wallets=5 eligible=3 funded=1 already_migrated=1 deliverable=1 waiting_identity=1 external_identity=1 inventory_requests=2\n" {
		t.Fatalf("plan inventory counts: exit=%d out=%q error=%s", code, out, errOut)
	}
	code, out, errOut = invoke(env, "deliver", "--report-only-counts")
	if code != 1 || out != "wallets=5 eligible=4 funded=1 migrated=1 deliverable=1 waiting_identity=1 external_identity=1 eligible_not_migrated=3\n" || !strings.Contains(errOut, "1 deliverable wallets") {
		t.Fatalf("pending delivery counts: exit=%d out=%q error=%s", code, out, errOut)
	}
	before := f.Delivery.ReceivedBytes()
	code, out, errOut = invoke(env, "rehearse", "--report-only-counts")
	summary := "wallets=5 eligible=3 funded_archived=1 already_migrated=1 deliverable=1 waiting_identity=1 external_identity=1 read=1 verified=1 imported=1 refreshed=1 test_signed=1 matched=1\n"
	if code != 0 || !strings.HasPrefix(out, summary) || strings.Contains(out, "sha256") {
		t.Fatalf("rehearsal counts: exit=%d out=%q error=%s", code, out, errOut)
	}
	if info, err := os.Stat(env["CEREMONY_REHEARSAL_RECEIPT"]); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("command rehearsal receipt: %v", err)
	}
	if !reflect.DeepEqual(before, f.Delivery.ReceivedBytes()) {
		t.Fatal("rehearsal command contacted a delivery attestor")
	}
	env[archive.EnvPath] = filepath.Join(t.TempDir(), "delivery.archive")
	if code, _, errOut := invoke(env, "archive"); code != 0 {
		t.Fatal(errOut)
	}
	row, external := f.WalletState(t, f.Waiting), f.WalletState(t, f.External)
	code, out, errOut = invoke(env, "deliver")
	if code != 0 || out != "eligible=3 deliverable=1 waiting_identity=1 external_identity=1 funded_archived=1 read=1 verified=1 imported=1 refreshed=1 test_signed=1 matched=1\n" {
		t.Fatalf("delivery command: exit=%d out=%q error=%s", code, out, errOut)
	}
	if f.WalletState(t, f.Waiting) != row || f.WalletState(t, f.External) != external {
		t.Fatal("delivery command changed a retained wallet")
	}
	f.AssertNoJournal(t, f.Waiting, f.External)
	f.Delivery.AssertNoKeyRequests(t, migrate.SecpKeyID(f.Waiting), migrate.IdentityKeyID(f.Waiting), migrate.SecpKeyID(f.External), migrate.IdentityKeyID(f.External))
	code, out, errOut = invoke(env, "deliver", "--report-only-counts")
	if code != 0 || out != "wallets=5 eligible=4 funded=1 migrated=2 deliverable=0 waiting_identity=1 external_identity=1 eligible_not_migrated=2\n" {
		t.Fatalf("completed delivery counts: exit=%d out=%q error=%s", code, out, errOut)
	}
}

func TestWaitingIdentityCommandsWithoutDeliverable(t *testing.T) {
	f := ceremonyfixture.NewIdentityStates(t, false)
	env := f.CommandEnvironment(t)
	delivery, isolated := f.Delivery.ReceivedBytes(), f.Isolated.ReceivedBytes()
	code, out, errOut := invoke(env, "rehearse", "--report-only-counts")
	if code != 0 || !strings.HasPrefix(out, "wallets=5 eligible=3 funded_archived=1 already_migrated=1 deliverable=0 waiting_identity=2 external_identity=1 read=0 verified=0 imported=0 refreshed=0 test_signed=0 matched=0\n") {
		t.Fatalf("empty command rehearsal: exit=%d out=%q error=%s", code, out, errOut)
	}
	env[archive.EnvPath] = filepath.Join(t.TempDir(), "delivery.archive")
	if code, _, errOut := invoke(env, "archive"); code != 0 {
		t.Fatal(errOut)
	}
	code, out, errOut = invoke(env, "deliver")
	if code != 0 || out != "eligible=3 deliverable=0 waiting_identity=2 external_identity=1 funded_archived=1 read=0 verified=0 imported=0 refreshed=0 test_signed=0 matched=0\n" {
		t.Fatalf("empty command delivery: exit=%d out=%q error=%s", code, out, errOut)
	}
	if !reflect.DeepEqual(delivery, f.Delivery.ReceivedBytes()) || !reflect.DeepEqual(isolated, f.Isolated.ReceivedBytes()) {
		t.Fatal("empty command ceremony contacted an attestor")
	}
	f.AssertNoJournal(t, f.Deliverable, f.Waiting, f.External, f.Funded, f.Migrated)
}
