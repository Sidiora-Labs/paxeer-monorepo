package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/server"
)

func TestRepairM_172_HelperUsesCLI(t *testing.T) {
	TestLoadTestAgainstFiveHardenedDaemons(t)
}

func TestRepairM_172_InventoryLifecycle(t *testing.T) {
	nw := startNetwork(t, 5)
	token := nw.issuer.mint(t, nw.issuer.key, testSubject)
	env := nw.env(token)
	env[EnvDuration], env[EnvConcurrency] = "2s", "2"
	path := filepath.Join(t.TempDir(), "run.json")
	invoke := func(phase string, extra ...string) (Summary, []byte, error) {
		t.Helper()
		var out, stderr bytes.Buffer
		args := append([]string{"-manifest", path, "-phase", phase}, extra...)
		summary, err := run(context.Background(), args, func(k string) string { return env[k] }, &out, &stderr)
		return summary, out.Bytes(), err
	}
	read := func() RunManifest {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var m RunManifest
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("manifest permissions: %v, %v", info, err)
		}
		return m
	}
	pins := make([]string, 0, 5)
	for _, ep := range nw.endpoints {
		id, _, _ := strings.Cut(ep, "=")
		pins = append(pins, id+"="+nw.pins[id])
	}
	summary, raw, err := invoke("export", "-pins", strings.Join(pins, ","))
	if err != nil || summary.Sessions != 0 {
		t.Fatalf("export: %+v %v", summary, err)
	}
	initial := read()
	var printed RunManifest
	if err := json.Unmarshal(raw, &printed); err != nil || !reflect.DeepEqual(printed, initial) {
		t.Fatalf("exported manifest mismatch: %v", err)
	}
	if len(initial.Inventory) != 2 || initial.RunID == "" {
		t.Fatalf("missing approval handoff: %+v", initial)
	}
	for i, key := range initial.Inventory {
		if key.Owner != testSubject || key.PublicKey != "" || key.KeyID != initial.Keys[i].KeyID || !reflect.DeepEqual(key.Operations, []string{"generate"}) {
			t.Fatalf("generation handoff: %+v", key)
		}
	}
	if _, _, err := invoke("export", "-pins", strings.Join(pins, ",")); err == nil {
		t.Fatal("export overwrote an existing manifest")
	}
	if _, _, err := invoke("sign"); err == nil || !strings.Contains(err.Error(), "have not been generated") {
		t.Fatalf("unprovisioned resume: %v", err)
	}
	if _, _, err := invoke("generate"); err == nil || !strings.Contains(err.Error(), server.CodeKeyInvalidShare) {
		t.Fatalf("unapproved generation: %v", err)
	}
	if got := read(); !reflect.DeepEqual(got, initial) {
		t.Fatal("refused generation changed the manifest")
	}

	// A trusted CA alone must not admit a certificate different from the approved pin.
	bad := initial
	bad.Endpoints = append([]endpoint(nil), initial.Endpoints...)
	bad.Endpoints[0].Pin = strings.Repeat("0", 64)
	if err := saveManifest(path, &bad, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := invoke("generate"); err == nil || !strings.Contains(err.Error(), "SPKI pin mismatch") {
		t.Fatalf("unpinned attestor admitted: %v", err)
	}
	if err := saveManifest(path, &initial, false); err != nil {
		t.Fatal(err)
	}

	nw.inventory.Keys = append(nw.inventory.Keys, initial.Inventory...)
	nw.publishInventory(t)
	summary, raw, err = invoke("generate")
	if err != nil || summary.Sessions != 0 {
		t.Fatalf("generation must pause before signing: %+v %v", summary, err)
	}
	generated := read()
	if err := json.Unmarshal(raw, &printed); err != nil || !reflect.DeepEqual(printed, generated) {
		t.Fatalf("generated handoff mismatch: %v", err)
	}
	if generated.RunID != initial.RunID {
		t.Fatal("generation changed approved run identity")
	}
	for i, key := range generated.Inventory {
		if key.KeyID != initial.Inventory[i].KeyID || key.PublicKey == "" || key.PublicKey != generated.Keys[i].PublicKey || !reflect.DeepEqual(key.Operations, []string{"sign"}) {
			t.Fatalf("signing handoff: %+v", key)
		}
	}
	// Each invocation reopens the durable handoff, without retaining a loadTester or HTTP client.
	if _, _, err := invoke("generate"); err != nil {
		t.Fatalf("reopen generated manifest: %v", err)
	}
	if got := read(); !reflect.DeepEqual(got, generated) {
		t.Fatal("resuming generation replaced generated keys")
	}
	refused, raw, err := invoke("sign")
	if err != nil {
		t.Fatal(err)
	}
	checkShape(t, "before signing approval", refused, raw, 2, 2*time.Second)
	if refused.Successes != 0 || len(refused.Refusals) != 1 || refused.Refusals[server.CodeKeyInvalidShare] != refused.Sessions {
		t.Fatalf("generation approval admitted signing: %+v", refused)
	}
	nw.inventory.Keys = generated.Inventory
	nw.publishInventory(t)
	for pass := 0; pass < 2; pass++ {
		signed, raw, err := invoke("sign")
		if err != nil {
			t.Fatalf("resume %d: %v", pass, err)
		}
		checkShape(t, "approved resume", signed, raw, 2, 2*time.Second)
		if signed.Successes == 0 || signed.Successes != signed.Sessions || len(signed.Refusals) != 0 || signed.ThroughputPerSecond <= 0 {
			t.Fatalf("approved resume %d: %+v", pass, signed)
		}
		for curve, c := range signed.Curves {
			if c.Successes == 0 || c.ThroughputPerSecond <= 0 {
				t.Fatalf("resume curve %s: %+v", curve, c)
			}
		}
		if !reflect.DeepEqual(signed.Keys, generated.Keys) {
			t.Fatal("resume generated new keys")
		}
	}
	if got := read(); !reflect.DeepEqual(got, generated) {
		t.Fatal("signing changed the durable handoff")
	}
	env[EnvChainID] = "126"
	if _, _, err := invoke("sign"); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("cross-chain resume admitted: %v", err)
	}
	env[EnvChainID] = "125"
	env[EnvToken] = nw.issuer.mint(t, nw.issuer.key, "other-owner")
	if _, _, err := invoke("sign"); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("cross-owner resume admitted: %v", err)
	}
}
