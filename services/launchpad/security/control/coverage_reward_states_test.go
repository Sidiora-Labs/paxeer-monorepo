package control

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestBountyFundingAndCancellationPreserveInventoryBounds(t *testing.T) {
	verifier := chain(t, nil)
	env := repairM1020EnvironmentFor(t, verifier)
	ctx := context.Background()
	credential := env.credentials["finance"]
	base := bounty("bounded-funding", pax, "100")
	base.Scope, base.State, base.Revision = "service", controlDraft, 1
	base.Deadline = time.Now().Add(time.Hour)
	inventory := Inventory{Asset: pax, Confirmed: "1000", Pending: "100", Reserved: "100", Settled: "0", Reversed: "0", Source: custody, Manifest: fixtureManifest, Reference: "confirmed", CheckedAt: time.Now(), Revision: 1}
	put := func(b Bounty, v *Inventory) {
		t.Helper()
		objects := map[string]json.RawMessage{}
		bountyWire, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		objects["bounty/"+b.ID] = bountyWire
		if v != nil {
			inventoryWire, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			objects[controlInventory+"/"+pax.Key()] = inventoryWire
		}
		if err := env.store.local.save(objects, nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, sample := range []struct {
		name, next string
		change     func(*Bounty, *Inventory) bool
		want       error
	}{
		{"missing funding inventory", controlFunded, func(*Bounty, *Inventory) bool { return false }, FundingRequired},
		{"already funded", controlFunded, func(b *Bounty, _ *Inventory) bool { b.State = controlFunded; return true }, Denied},
		{"reserved balance exceeds confirmed", controlFunded, func(_ *Bounty, v *Inventory) bool { v.Reserved = "1100"; return true }, FundingRequired},
		{"inventory revision ceiling", controlFunded, func(_ *Bounty, v *Inventory) bool {
			v.Pending = "0"
			v.Reserved = "0"
			v.Revision = ^uint64(0)
			return true
		}, Conflict},
		{"active without inventory", controlActive, func(b *Bounty, _ *Inventory) bool { b.State = controlFunded; return false }, Missing},
		{"active with frozen inventory", controlActive, func(b *Bounty, v *Inventory) bool { b.State = controlFunded; v.Frozen = true; return true }, FundingRequired},
		{"adjudication before submission", controlAdjudicating, func(*Bounty, *Inventory) bool { return true }, Denied},
		{"award without inventory", controlAwarded, func(b *Bounty, _ *Inventory) bool { b.State = controlAdjudicating; return false }, Missing},
		{"award with frozen inventory", controlAwarded, func(b *Bounty, v *Inventory) bool { b.State = controlAdjudicating; v.Frozen = true; return true }, FundingRequired},
		{"expiry before deadline", controlExpired, func(*Bounty, *Inventory) bool { return true }, Denied},
		{"cancel without inventory", controlCancelled, func(b *Bounty, _ *Inventory) bool { b.State = controlFunded; return false }, Missing},
		{"cancel without reserved amount", controlCancelled, func(b *Bounty, v *Inventory) bool { b.State = controlFunded; v.Reserved = "0"; return true }, BudgetExceeded},
		{"cancel without pending amount", controlCancelled, func(b *Bounty, v *Inventory) bool { b.State = controlFunded; v.Pending = "0"; return true }, BudgetExceeded},
		{"cancel with reversed ceiling", controlCancelled, func(b *Bounty, v *Inventory) bool {
			b.State = controlFunded
			v.Reversed = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)).String()
			return true
		}, Invalid},
		{"cancel at revision ceiling", controlCancelled, func(b *Bounty, v *Inventory) bool { b.State = controlFunded; v.Revision = ^uint64(0); return true }, Conflict},
	} {
		t.Run(sample.name, func(t *testing.T) {
			candidate, current := base, inventory
			include := sample.change(&candidate, &current)
			if include {
				put(candidate, &current)
			} else {
				put(candidate, nil)
			}
			if err := env.store.BountyState(ctx, credential, "service", candidate.ID, "review funding state", sample.next, "reviewed evidence", beneficiary, candidate.Revision); !errors.Is(err, sample.want) {
				t.Fatalf("%s: %v", sample.name, err)
			}
		})
	}
	invalidDeadline := base
	invalidDeadline.ID = "expired-draft"
	invalidDeadline.Deadline = time.Now().Add(-time.Minute)
	if err := env.store.CreateBounty(ctx, credential, "service", "create expired draft", invalidDeadline); !errors.Is(err, Invalid) {
		t.Fatalf("expired bounty draft: %v", err)
	}
	if err := env.store.Reconcile(ctx, credential, controlPlatform, "invalid inventory asset", Asset{}); !errors.Is(err, Invalid) {
		t.Fatalf("invalid inventory asset: %v", err)
	}
	if err := env.store.Reconcile(ctx, credential, controlPlatform, "unbound token inventory", usdl); !errors.Is(err, Unavailable) {
		t.Fatalf("unverified token inventory: %v", err)
	}
	env.store.verifier = nil
	if err := env.store.Reconcile(ctx, credential, controlPlatform, "missing inventory authority", pax); !errors.Is(err, Unavailable) {
		t.Fatalf("missing inventory authority: %v", err)
	}
}

func TestSettlementObservationKeepsBoundReferences(t *testing.T) {
	verifier := chain(t, nil)
	env := repairM1020EnvironmentFor(t, verifier)
	ctx := context.Background()
	credential := env.credentials["finance"]
	key := "bounty.observed"
	if err := env.store.ObserveSettlement(ctx, credential, "service", "bad key", "inspect reference", "reference"); !errors.Is(err, Invalid) {
		t.Fatalf("invalid obligation key: %v", err)
	}
	if err := env.store.ObserveSettlement(ctx, credential, "service", key, "inspect absent obligation", "reference"); !errors.Is(err, Missing) {
		t.Fatalf("absent obligation: %v", err)
	}
	obligation := Obligation{ID: id(), Scope: "other", IdempotencyKey: key, Amount: "100", Destination: beneficiary, AuthorityReference: custody, State: controlSettled, SettlementReference: "0x" + strings.Repeat("a", 64), Asset: pax, Revision: 1}
	put := func(o Obligation) {
		t.Helper()
		wire, err := json.Marshal(o)
		if err != nil {
			t.Fatal(err)
		}
		if err := env.store.local.save(map[string]json.RawMessage{controlSettlement + "/" + key: wire}, nil); err != nil {
			t.Fatal(err)
		}
	}
	put(obligation)
	if err := env.store.ObserveSettlement(ctx, credential, "service", key, "different obligation scope", obligation.SettlementReference); !errors.Is(err, Denied) {
		t.Fatalf("unbound obligation scope: %v", err)
	}
	obligation.Scope = "service"
	put(obligation)
	if err := env.store.ObserveSettlement(ctx, credential, "service", key, "different settlement reference", "0x"+strings.Repeat("b", 64)); !errors.Is(err, Conflict) {
		t.Fatalf("different settled reference: %v", err)
	}
	if err := env.store.ObserveSettlement(ctx, credential, "service", key, "repeat settlement observation", obligation.SettlementReference); err != nil {
		t.Fatalf("matching settled reference: %v", err)
	}
}
