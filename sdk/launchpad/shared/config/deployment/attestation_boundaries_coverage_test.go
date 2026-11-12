package deployment

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

func TestAttestationValidationBindsFamilyPolicyAndEveryIdentity(t *testing.T) {
	manifest, reader, block, policy := newEVM(t, nil, 125)
	attestation, err := Attest(t.Context(), reader, manifest, block, policy)
	if err != nil {
		t.Fatal(err)
	}
	if err := attestation.ValidateFor(manifest, policy); err != nil {
		t.Fatal(err)
	}
	for name, edit := range map[string]func(*ActivationAttestation, *Manifest, *Policy){
		"policy":           func(_ *ActivationAttestation, _ *Manifest, p *Policy) { p.MaxAge = 0 },
		"revision":         func(a *ActivationAttestation, _ *Manifest, _ *Policy) { a.ManifestHash = strings.Repeat("0", 64) },
		"clock":            func(a *ActivationAttestation, _ *Manifest, p *Policy) { a.ObservedAt = p.Now.Add(time.Second) },
		"missing identity": func(a *ActivationAttestation, _ *Manifest, _ *Policy) { delete(a.Identities, "Router") },
		"implementation": func(a *ActivationAttestation, _ *Manifest, _ *Policy) {
			id := a.Identities["Router"]
			id.ImplementationAddress = common.Address{1}.Hex()
			a.Identities["Router"] = id
		},
		"beacon": func(a *ActivationAttestation, _ *Manifest, _ *Policy) {
			id := a.Identities["FixturePool"]
			id.BeaconAddress = common.Address{1}.Hex()
			a.Identities["FixturePool"] = id
		},
		"owner":      func(a *ActivationAttestation, _ *Manifest, _ *Policy) { a.ActualOwner = common.Address{1}.Hex() },
		"settlement": func(a *ActivationAttestation, _ *Manifest, _ *Policy) { a.SettlementAddress = common.Address{1}.Hex() },
		"fee":        func(a *ActivationAttestation, _ *Manifest, _ *Policy) { a.CreationFeeRaw = "2" },
		"eligible": func(a *ActivationAttestation, _ *Manifest, _ *Policy) {
			a.Eligible = false
			a.Issues = []Issue{{Code: "configuration_policy_read_failed", Role: "Router"}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			a, m, p := attestation, manifest, policy
			a.Identities = map[string]Identity{}
			for role, id := range attestation.Identities {
				a.Identities[role] = id
			}
			edit(&a, &m, &p)
			if err := a.ValidateFor(m, p); err == nil {
				t.Fatal("changed complete attestation admitted")
			}
			if name != "eligible" {
				if err := a.ValidateFamilyFor(m, p, []string{"Router", "FixturePool", "USDL"}, true); err == nil {
					t.Fatal("changed family attestation admitted")
				}
			}
		})
	}
	for _, status := range []string{"recorded", "pending-beacon-acceptance"} {
		candidate := manifest
		candidate.Status = status
		hash, err := candidate.Hash()
		if err != nil {
			t.Fatal(err)
		}
		a := attestation
		a.ManifestHash = hash
		if err := a.ValidateFor(candidate, policy); err == nil {
			t.Fatal("unactivated deployment admitted")
		}
		if err := a.ValidateFamilyFor(candidate, policy, []string{"Router"}, false); err == nil {
			t.Fatal("unactivated family admitted")
		}
	}
	a := attestation
	a.Issues = []Issue{{Code: "configuration_policy_read_failed", Role: "Router"}}
	if err := a.ValidateFamilyFor(manifest, policy, []string{"Router"}, false); err == nil {
		t.Fatal("unavailable family policy admitted")
	}
	if err := a.ValidateFamilyFor(manifest, policy, []string{"unknown"}, false); err == nil {
		t.Fatal("unknown role admitted")
	}
	invalid := manifest
	invalid.ChainID = 1
	if err := a.ValidateFor(invalid, policy); err == nil {
		t.Fatal("foreign chain admitted")
	}
	if err := a.ValidateFamilyFor(invalid, policy, nil, false); err == nil {
		t.Fatal("foreign family chain admitted")
	}
}

func TestDeploymentJSONAndManifestRejectAmbiguousDocuments(t *testing.T) {
	for _, wire := range []string{`{"a":1,"a":2}`, `{"a":{"b":1,"b":2}}`, `[1,]`, `{"a":}`, `{"a":[`, `"unterminated`, strings.Repeat("[", 66) + "0" + strings.Repeat("]", 66)} {
		if err := uniqueJSON([]byte(wire)); err == nil {
			t.Fatalf("ambiguous document admitted: %.32s", wire)
		}
	}
	for _, wire := range []string{`{"a":[1,true,null,{"b":2}]}`, `[1]`} {
		if err := uniqueJSON([]byte(wire)); err != nil {
			t.Fatal(err)
		}
	}
	for _, mode := range []string{"inventory", "name", "direct", "precompile", "status", "staged"} {
		candidate := Current()
		switch mode {
		case "inventory":
			for i := 0; i < 65; i++ {
				candidate.Roles[fmt.Sprintf("Role%d", i)] = Role{Address: fmt.Sprintf("0x%040x", 100+i), Kind: "direct", RuntimeHash: "0x" + strings.Repeat("a", 64)}
			}
		case "name":
			candidate.Roles["invalid role"] = candidate.Roles["Router"]
		case "direct":
			r := candidate.Roles["USDL"]
			r.BeaconAddress = candidate.Roles["Router"].Address
			candidate.Roles["USDL"] = r
		case "precompile":
			r := candidate.Roles["USDL"]
			r.Kind = "precompile"
			candidate.Roles["Other"] = r
		case "status":
			candidate.Status = "unknown"
		case "staged":
			r := candidate.Roles["USDL"]
			r.StagedRuntimeHash = r.RuntimeHash
			r.StagedUpgrade = "stage"
			candidate.Roles["USDL"] = r
		}
		if err := candidate.Validate(); err == nil {
			t.Fatalf("invalid manifest %s admitted", mode)
		}
		if _, err := candidate.Hash(); err == nil {
			t.Fatal("invalid manifest hashed")
		}
		if got := candidate.String(); got != "deployment: invalid" {
			t.Fatalf("invalid manifest description: %s", got)
		}
	}
	if _, err := Parse([]byte(`{} {}`)); err == nil {
		t.Fatal("multiple manifest documents admitted")
	}
	valid := Current()
	if valid.String() == "" {
		t.Fatal("deployment description missing")
	}
	if _, ok := Approved(0); ok {
		t.Fatal("unknown activation approved")
	}
	if _, ok := NextActivationHeight(^uint64(0)); ok {
		t.Fatal("unbounded next activation")
	}
	if next, ok := NextActivationHeight(0); !ok || next == 0 {
		t.Fatal("first activation missing")
	}
	if _, err := NewRevision(Manifest{}, strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64)); err == nil {
		t.Fatal("invalid deployment revision admitted")
	}
}
