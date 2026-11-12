package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrivateManifestRejectsAmbiguousAndIncompleteNestedDocuments(t *testing.T) {
	for _, wire := range []string{"", `{"a":1,"a":2}`, `{"a":{"b":1,"b":2}}`, `[1,]`, `{"a":}`, `{"a":[`, `"unterminated`, strings.Repeat("[", 18) + "0" + strings.Repeat("]", 18), `{} {}`, "[" + strings.Repeat("0,", 256) + "0]"} {
		if err := fixtureUniqueJSON([]byte(wire)); err == nil {
			t.Fatalf("ambiguous document admitted: %.32s", wire)
		}
	}
	if err := fixtureUniqueJSON([]byte(`{"array":[1,true,null,{"b":2}]}`)); err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	for i := 0; i < 257; i++ {
		keys = append(keys, fmt.Sprintf("%q:0", fmt.Sprint(i)))
	}
	if err := fixtureUniqueJSON([]byte("{" + strings.Join(keys, ",") + "}")); err == nil {
		t.Fatal("unbounded manifest object admitted")
	}
	proof := privateIdentityFixture()
	wire, err := json.Marshal(proof)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"deployment", "native", "settlement", "governance", "required key", "null asset"} {
		t.Run(field, func(t *testing.T) {
			var doc map[string]any
			if err := json.Unmarshal(wire, &doc); err != nil {
				t.Fatal(err)
			}
			deployment := doc["deployment"].(map[string]any)
			switch field {
			case "deployment":
				deployment["unknown"] = true
			case "required key":
				deployment["unknown"] = deployment["status"]
				delete(deployment, "status")
			case "null asset":
				deployment["native"] = nil
			default:
				deployment[field] = map[string]any{}
			}
			candidate, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			if err := fixtureRequiredFields(candidate); err == nil {
				t.Fatal("incomplete nested private identity admitted")
			}
		})
	}
	path := filepath.Join(t.TempDir(), "manifest.json")
	write := func(raw []byte) BaseEnv {
		t.Helper()
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(raw)
		return BaseEnv{DeploymentRealm: valuePrivateFixture, RPCURL: proof.RPCURL, PrivateFixtureManifestFile: path, PrivateFixtureManifestSHA256: hex.EncodeToString(hash[:])}
	}
	for _, mode := range []string{"unavailable", "directory", "malformed type", "wrong realm", "wrong schema", "wrong rpc", "wrong genesis", "invalid identity"} {
		t.Run(mode, func(t *testing.T) {
			var doc map[string]any
			if err := json.Unmarshal(wire, &doc); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "malformed type":
				doc["schemaVersion"] = "one"
			case "wrong realm":
				doc["realm"] = "public"
			case "wrong schema":
				doc["schemaVersion"] = 2
			case "wrong rpc":
				doc["rpcUrl"] = "http://127.0.0.1:9545"
			case "wrong genesis":
				doc["genesisHash"] = "invalid"
			case "invalid identity":
				doc["deployment"].(map[string]any)["chainId"] = 1
			}
			raw, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			env := write(raw)
			if mode == "unavailable" {
				env.PrivateFixtureManifestFile = filepath.Join(t.TempDir(), "absent.json")
			}
			if mode == "directory" {
				env.PrivateFixtureManifestFile = t.TempDir()
			}
			if _, err := env.PrivateFixtureEvidence(); err == nil {
				t.Fatal("unbound private identity admitted")
			}
			if _, err := env.PrivateFixtureDeployment(); err == nil {
				t.Fatal("unbound private deployment admitted")
			}
		})
	}
}
