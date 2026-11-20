package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/ogrender/fonts"
	inventory "github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l16"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l18"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func journeyDocumentsValues(t *testing.T, root string) (string, map[string]any) {
	t.Helper()
	values := t.TempDir()
	for _, pair := range [][2]string{{"common.example.json", "common.json"}, {"pnl.journeys.example.json", "pnl.json"}} {
		raw, err := os.ReadFile(filepath.Join(root, "deploy/launchpad/documents/values", pair[0]))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(values, pair[1]), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(values, "expected-schema"), 0700); err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte("pnl journey schema")))
	schema := production.ExpectedSchema{
		Service: "core/pnl-tracker", RuntimeRole: "pnl_runtime", RegistryDigest: digest,
		InterfaceDigest: digest, ReaderVersion: 1, Migrations: map[int64]string{160001: digest},
		Schemas: map[string]string{"pnl": digest}, Epoch: 1, Region: "mainnet-beta",
		Privileges: []production.Grant{{Schema: "pnl", Object: "positions", Privileges: []string{"SELECT"}}},
	}
	journeyDocumentsWriteJSON(t, filepath.Join(values, "expected-schema/pnl.json"), schema)
	raw, err := os.ReadFile(filepath.Join(values, "pnl.json"))
	if err != nil {
		t.Fatal(err)
	}
	var source map[string]any
	if err = json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	return values, source
}

func journeyDocumentsWriteJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func journeyDocumentsCompose(t *testing.T, root, values string) (map[string]string, []byte, pnlJourneyDocument) {
	t.Helper()
	out := t.TempDir()
	cmd := exec.Command("python3", filepath.Join(root, "deploy/launchpad/documents/compose.py"), "--values", values, "--out", out, "--service", "pnl")
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compose pnl journeys: %v\n%s", err, output)
	}
	path := filepath.Join(out, "pnl.json")
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("private document file: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var composed map[string]string
	if err = json.Unmarshal(raw, &composed); err != nil {
		t.Fatal(err)
	}
	raw, err = base64.StdEncoding.Strict().DecodeString(composed["PNL_JOURNEY_CONFIG_DOCUMENT"])
	if err != nil {
		t.Fatal(err)
	}
	if composed["PNL_JOURNEY_CONFIG_SHA256"] != fmt.Sprintf("%x", sha256.Sum256(raw)) {
		t.Fatal("journey digest does not bind document bytes")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var document pnlJourneyDocument
	if err = decoder.Decode(&document); err != nil {
		t.Fatal(err)
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		t.Fatalf("journey document must contain one object: %v", err)
	}
	return composed, raw, document
}

func TestJourneyDocumentsConstantsAndEnvironment(t *testing.T) {
	root := coordinatortest.RepositoryRoot(t)
	values, source := journeyDocumentsValues(t, root)
	composed, _, document := journeyDocumentsCompose(t, root, values)
	if document.Version != 3 || document.IdentityIssuer != "pnld" || document.IdentityAudience != actorIdentity {
		t.Fatal("journey identity contract differs")
	}
	if document.FontRegularSHA256 != fonts.RegularSHA256 || document.FontBoldSHA256 != fonts.BoldSHA256 || document.FontLicenseSHA256 != fonts.LicenseSHA256 {
		t.Fatal("journey font bindings differ from embedded assets")
	}
	if document.RewardManifestDigest != l18.ManifestDigest {
		t.Fatal("journey reward manifest differs")
	}
	if document.IdentityEndpoint != composed["IDENTITY_SERVICE_URL"] || document.FontDirectory != composed["PNL_FONT_DIR"] {
		t.Fatal("journey environment bindings differ")
	}
	inputs := source["journeys"].(map[string]any)
	if document.IdentityEndpoint != inputs["identityEndpoint"] || document.FontDirectory != inputs["fontDirectory"] {
		t.Fatal("journey environment differs from values")
	}
}

func TestJourneyDocumentsDeliverySubjectsDocumented(t *testing.T) {
	root := coordinatortest.RepositoryRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "deploy/launchpad/documents/README.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, section, found := strings.Cut(string(raw), "## PnL journey configuration\n")
	if !found {
		t.Fatal("PnL journey configuration section is missing")
	}
	section, _, _ = strings.Cut(section, "\n## ")
	policy := delivery.Policy{Subjects: map[string]bool{inventory.RebuildSubject: true, "portfolio.snapshot.v2": true}, EventTypes: map[string]bool{inventory.RebuildSubject: true, "portfolio.snapshot.v2": true}}
	policy = l18.RewardDeliveryPolicy(policy)
	for subject, admitted := range policy.Subjects {
		if admitted && !strings.Contains(section, "`"+subject+"`") {
			t.Errorf("journey delivery subject %s is missing", subject)
		}
	}
}

func TestJourneyDocumentsConstructActualRuntime(t *testing.T) {
	journeyDocumentsConstructActualRuntime(t, false)
}

func journeyDocumentsConstructActualRuntime(t *testing.T, variableKey bool) {
	t.Helper()
	fixture := newJourneyCompositionFixture(t)
	values, source := journeyDocumentsValues(t, fixture.root)
	raw, err := json.Marshal(fixture.document)
	if err != nil {
		t.Fatal(err)
	}
	var inputs map[string]any
	if err = json.Unmarshal(raw, &inputs); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"version", "identityIssuer", "identityAudience", "fontRegularSha256", "fontBoldSha256", "fontLicenseSha256", "rewardManifestDigest"} {
		delete(inputs, name)
	}
	// Empty policies are explicit lists in the values document.
	inputs["approvedCommunityWallets"] = []string{}
	if variableKey {
		fixture.cfg.IdentityMTLSClientKeyPEM = journeyIdentityKey(t, fixture.document.IdentityClientKeyFile)
		delete(inputs, "identityClientKeyFile")
		delete(inputs, "identityClientKeySha256")
	}
	source["journeys"] = inputs
	for name, value := range map[string]any{
		"sourceStream": fixture.runtime.SourceStream, "consumerId": fixture.runtime.ConsumerID,
		"generation": fixture.runtime.Generation, "owner": fixture.runtime.Owner,
		"projectionVersion": fixture.runtime.ProjectionVersion, "epoch": fixture.runtime.Epoch,
		"region": fixture.runtime.Region,
	} {
		source[name] = value
	}
	journeyDocumentsWriteJSON(t, filepath.Join(values, "pnl.json"), source)
	composed, raw, document := journeyDocumentsCompose(t, fixture.root, values)
	if variableKey && (document.IdentityClientKeyFile != "" || document.IdentityClientKeySHA256 != "") {
		t.Fatal("variable key form requires empty client key file members")
	}
	if !reflect.DeepEqual(document.Programs, fixture.document.Programs) || !reflect.DeepEqual(document.RewardRuntimeHashes, fixture.document.RewardRuntimeHashes) {
		t.Fatal("composed reward policy differs from runtime inputs")
	}
	cfg := fixture.cfg
	cfg.IdentityURL, cfg.FontDir = composed["IDENTITY_SERVICE_URL"], composed["PNL_FONT_DIR"]
	cfg.JourneyConfigFile, _ = writeCompositionFile(t, raw)
	cfg.JourneyConfigSHA256 = composed["PNL_JOURNEY_CONFIG_SHA256"]
	deliveryRaw, err := base64.StdEncoding.Strict().DecodeString(composed["DELIVERY_CONFIG_DOCUMENT"])
	if err != nil {
		t.Fatal(err)
	}
	cfg.DeliveryConfigFile, _ = writeCompositionFile(t, deliveryRaw)
	cfg.DeliveryConfigSHA256 = composed["DELIVERY_CONFIG_SHA256"]
	journey, err := newPnLJourneys(fixture.ctx, cfg, fixture.pool, nil, fixture.primary, fixture.nc, fixture.baseline, fixture.consumer)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(journey.second.Close)
	if journey.rewards == nil || journey.live == nil || journey.shares == nil || journey.schema != fixture.runtime.Schema || journey.document.BrokerStream != inputs["brokerStream"] {
		t.Fatal("composed journey runtime is incomplete")
	}
}
