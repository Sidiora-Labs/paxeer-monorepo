package notifications

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/discovery"
)

func releaseCoverageDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func releaseCoverageConfig(t *testing.T) ReleaseConfig {
	t.Helper()
	root := t.TempDir()
	write := func(name string, raw []byte) string {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	const sourceName = "candidate.txt"
	source := []byte("candidate")
	write(sourceName, source)
	sourceDigest := releaseCoverageDigest(source)
	manifest := []byte(sourceName + "\x00" + sourceDigest + "\n")
	topology := []byte("topology")
	iface := []byte("interface")
	triggers := []byte("trigger dataset")
	c := ReleaseConfig{
		Binding:    ReleaseBinding{CandidateDigest: releaseCoverageDigest(manifest), TopologyDigest: releaseCoverageDigest(topology), InterfaceDigest: releaseCoverageDigest(iface), DatasetDigest: releaseCoverageDigest(triggers), SchemaVersion: "1", PolicyVersion: "1"},
		SourceRoot: root, TopologyFile: write("topology.json", topology), InterfaceFile: write("interface.json", iface),
		TriggerFile: write("triggers.jsonl", triggers), SourceFiles: map[string]string{sourceName: sourceDigest}, OutputDir: filepath.Join(root, "capture"),
		AllowedAccounts: []string{"account"}, AllowedOrigins: []string{"https://example.test"}, IsolatedSandbox: true,
		Concurrency: 1, QueueCapacity: 1, MaxCaptureBytes: 1024,
		DeviceReceiptFile: write("devices.json", []byte("[]")), DeviceCaptureKeys: map[string]string{"device": "key"},
		AppBuildDigests: map[string]string{providerAPNS: releaseCoverageDigest([]byte("apns")), providerFCM: releaseCoverageDigest([]byte("fcm"))},
	}
	for _, name := range []string{"pool_launch", stepCalloutPublish, "follow_author", "squad_join", stepThresholdAlert, "inbox", "feed", "leaderboard"} {
		step := ReleaseStep{Name: name, Method: http.MethodGet, URL: "https://example.test/" + name, Status: http.StatusOK, Assertions: []ReleaseAssertion{{Pointer: "/id", Equals: json.RawMessage(`"pool"`)}}}
		switch name {
		case stepCalloutPublish, stepThresholdAlert:
			step.Assertions = []ReleaseAssertion{{Pointer: "/id", FromStep: "pool_launch", FromPointer: "/id"}}
		case "inbox":
			step.Assertions = []ReleaseAssertion{{Pointer: "/id", FromStep: stepThresholdAlert, FromPointer: "/id"}}
		case "feed":
			step.Assertions = []ReleaseAssertion{{Pointer: "/id", FromStep: stepCalloutPublish, FromPointer: "/id"}}
		}
		c.Steps = append(c.Steps, step)
	}
	return c
}

func TestReleaseConfigBindsSourceFilesAndJourneyIdentities(t *testing.T) {
	c := releaseCoverageConfig(t)
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := c.bindSource(); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected journey request: %s %v", r.Method, r.Header)
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, `{"id":"pool"}`); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	for i := range c.Steps {
		c.Steps[i].URL = server.URL + "/" + c.Steps[i].Name
	}
	path := filepath.Join(t.TempDir(), "journey.jsonl")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	capture := &releaseCapture{file: file, max: 1 << 20}
	result := ReleaseResult{HTTPLatency: map[string]ReleaseLatency{}}
	if err := releaseJourney(context.Background(), c, server.Client(), capture, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.HTTPLatency) != len(c.Steps) {
		t.Fatalf("journey samples: %d", len(result.HTTPLatency))
	}
	raw, err := os.ReadFile(path)
	if err != nil || bytes.Count(raw, []byte(`"kind":"journey"`)) != len(c.Steps) {
		t.Fatalf("journey capture: %s, %v", raw, err)
	}
	c.Steps[0].Assertions[0].Equals = json.RawMessage(`"different"`)
	if err := releaseJourney(context.Background(), c, server.Client(), capture, &result); err == nil || !strings.Contains(err.Error(), "identity assertion") {
		t.Fatalf("changed identity accepted: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseSourceBindingRejectsChangedInputs(t *testing.T) {
	c := releaseCoverageConfig(t)
	if err := c.bindSource(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c.SourceRoot, "candidate.txt"), []byte("different"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.bindSource(); err == nil || !strings.Contains(err.Error(), "candidate file drift") {
		t.Fatalf("changed candidate accepted: %v", err)
	}
	c = releaseCoverageConfig(t)
	c.SourceFiles = map[string]string{"../candidate.txt": c.SourceFiles["candidate.txt"]}
	if err := c.bindSource(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("parent path accepted: %v", err)
	}
	c = releaseCoverageConfig(t)
	if err := os.WriteFile(c.TriggerFile, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.bindSource(); err == nil || !strings.Contains(err.Error(), "dataset digest") {
		t.Fatalf("changed dataset accepted: %v", err)
	}
	c = releaseCoverageConfig(t)
	c.Binding.CandidateDigest = releaseCoverageDigest([]byte("another manifest"))
	if err := c.bindSource(); err == nil || !strings.Contains(err.Error(), "manifest digest") {
		t.Fatalf("changed candidate manifest accepted: %v", err)
	}
	c = releaseCoverageConfig(t)
	if err := os.WriteFile(c.TopologyFile, []byte("changed topology"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.bindSource(); err == nil || !strings.Contains(err.Error(), "authority artifact digest") {
		t.Fatalf("changed topology accepted: %v", err)
	}
	if _, err := releaseLimitedFile(c.TopologyFile, 4); !errors.Is(err, ErrInvalid) {
		t.Fatalf("oversized input accepted: %v", err)
	}
	c = releaseCoverageConfig(t)
	if err := os.Remove(c.TriggerFile); err != nil {
		t.Fatal(err)
	}
	if err := c.bindSource(); err == nil {
		t.Fatal("missing trigger dataset accepted")
	}
	if _, err := releaseLimitedFile(c.TriggerFile, 1024); err == nil {
		t.Fatal("missing input file accepted")
	}
	if _, err := releasePointer(json.RawMessage(`{"id":"pool"}`), "id"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("non-pointer path accepted: %v", err)
	}
	if _, err := releasePointer(json.RawMessage(`{"id":"pool"}`), "/id/next"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("scalar traversal accepted: %v", err)
	}
}

func TestReleaseConfigChecksIdentityBindingsAndLimits(t *testing.T) {
	base := releaseCoverageConfig(t)
	for _, change := range []func(*ReleaseConfig){
		func(c *ReleaseConfig) { c.Binding.CandidateDigest = "not-a-digest" },
		func(c *ReleaseConfig) { c.IsolatedSandbox = false },
		func(c *ReleaseConfig) { c.AllowedAccounts = []string{"account", "account"} },
		func(c *ReleaseConfig) { c.AllowedOrigins = []string{"http://example.test"} },
		func(c *ReleaseConfig) { c.QueueCapacity = 16001 },
		func(c *ReleaseConfig) { c.Concurrency = 257 },
		func(c *ReleaseConfig) { c.Steps[0].Method = http.MethodDelete },
		func(c *ReleaseConfig) { c.Steps[0].Status = http.StatusForbidden },
		func(c *ReleaseConfig) { c.Steps[0].URL = "https://another.example/pool" },
		func(c *ReleaseConfig) { c.Steps[0].Assertions[0] = ReleaseAssertion{Pointer: "/id"} },
		func(c *ReleaseConfig) {
			c.Steps[1].Assertions[0] = ReleaseAssertion{Pointer: "/id", Equals: json.RawMessage(`"pool"`)}
		},
		func(c *ReleaseConfig) { c.Steps[5].Assertions[0].FromStep = "feed" },
		func(c *ReleaseConfig) { c.Steps = c.Steps[:7] },
		func(c *ReleaseConfig) { c.DeviceReceiptFile = "" },
	} {
		c := base
		c.Steps = append([]ReleaseStep(nil), base.Steps...)
		for i := range c.Steps {
			c.Steps[i].Assertions = append([]ReleaseAssertion(nil), base.Steps[i].Assertions...)
		}
		change(&c)
		if err := c.Validate(); err == nil {
			t.Fatal("invalid release configuration accepted")
		}
	}
}

func TestReleaseRawGlobalVerifiesSignedSocketAndRecoveryObservations(t *testing.T) {
	c := releaseCoverageConfig(t)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	c.GlobalCaptureKeys = map[string]string{"capture": base64.StdEncoding.EncodeToString(public)}
	c.MaxSocketQueueMessages = 10
	start := time.Now().UTC().Add(-25 * time.Hour)
	writeObservation := func(gate string, observations []ReleaseGlobalObservation) ReleaseGlobalArtifact {
		t.Helper()
		path := filepath.Join(t.TempDir(), "observations.jsonl")
		var out bytes.Buffer
		for _, observation := range observations {
			observation.Binding = c.Binding
			observation.Gate = gate
			observation.CaptureKeyID = "capture"
			message, e := json.Marshal(observation)
			if e != nil {
				t.Fatal(e)
			}
			observation.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, message))
			if e = json.NewEncoder(&out).Encode(observation); e != nil {
				t.Fatal(e)
			}
		}
		if e := os.WriteFile(path, out.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		return ReleaseGlobalArtifact{Binding: c.Binding, Gate: gate, StartedAt: start, FinishedAt: start.Add(25 * time.Hour), RawFiles: map[string]string{path: releaseCoverageDigest(out.Bytes())}}
	}
	sockets := writeObservation(gateSocketSurge, []ReleaseGlobalObservation{
		{At: start, Kind: "sockets", ActiveSockets: 200000, MaxQueuedMessages: 4},
		{At: start.Add(11 * time.Minute), Kind: "sockets", ActiveSockets: 210000, MaxQueuedMessages: 5},
	})
	totals, err := releaseRawGlobal(c, sockets)
	if err != nil || totals.peak != 210000 || totals.surgeLast.Sub(totals.surgeFirst) != 11*time.Minute {
		t.Fatalf("socket observations: %+v, %v", totals, err)
	}
	recovery := writeObservation(gateAZLoss, []ReleaseGlobalObservation{{At: start, Kind: "recovery", LostAZCount: 1, CatchupNS: int64(time.Second)}})
	totals, err = releaseRawGlobal(c, recovery)
	if err != nil || !totals.observedAZ {
		t.Fatalf("recovery observation: %+v, %v", totals, err)
	}
	mixed := writeObservation(gateMixedSoak, []ReleaseGlobalObservation{
		{At: start, Kind: "origin", HTTPRequestsTotal: 1, OriginHistogram: map[string]int64{"100000000": 1}},
		{At: start.Add(time.Hour), Kind: "channel", Channel: channelPush, ChannelLatencyNS: int64(time.Millisecond)},
		{At: start.Add(25 * time.Hour), Kind: "origin", HTTPRequestsTotal: 1000000000, OriginHistogram: map[string]int64{"100000000": 999999999}},
	})
	totals, err = releaseRawGlobal(c, mixed)
	if err != nil || totals.origin != 1000000000 || totals.channelSamples != 1 || totals.maxRPS < 10000 {
		t.Fatalf("mixed observations: %+v, %v", totals, err)
	}
	c.GlobalArtifacts = map[string]string{}
	for _, artifact := range []ReleaseGlobalArtifact{sockets, recovery, mixed} {
		path := filepath.Join(t.TempDir(), artifact.Gate+".json")
		raw, e := json.Marshal(artifact)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(path, raw, 0600); e != nil {
			t.Fatal(e)
		}
		c.GlobalArtifacts[artifact.Gate] = path
	}
	result := ReleaseResult{Files: map[string]string{}}
	if err := releaseGlobals(c, &result); err != nil || result.GlobalStatus != "imported_actual_serial_evidence" || len(result.Files) != 6 {
		t.Fatalf("signed global artifacts: %+v, %v", result, err)
	}
	badSocket := writeObservation(gateSocketSurge, []ReleaseGlobalObservation{{At: start, Kind: "sockets", ActiveSockets: 1, MaxQueuedMessages: 1}})
	if _, err := releaseRawGlobal(c, badSocket); err == nil || !strings.Contains(err.Error(), "surge missing") {
		t.Fatalf("short socket surge accepted: %v", err)
	}
	badRecovery := writeObservation(gateAZLoss, []ReleaseGlobalObservation{{At: start, Kind: "recovery", LostAZCount: 0}})
	if _, err := releaseRawGlobal(c, badRecovery); err == nil || !strings.Contains(err.Error(), "AZ loss") {
		t.Fatalf("missing recovery accepted: %v", err)
	}
	duplicate := writeObservation(gateSocketSurge, []ReleaseGlobalObservation{{At: start, Kind: "sockets", ActiveSockets: 200000}, {At: start, Kind: "sockets", ActiveSockets: 200000}})
	if _, err := releaseRawGlobal(c, duplicate); err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Fatalf("duplicate observation accepted: %v", err)
	}
	badHistogram := writeObservation(gateMixedSoak, []ReleaseGlobalObservation{{At: start, Kind: "origin", HTTPRequestsTotal: 2, OriginHistogram: map[string]int64{"100000000": 1}}})
	if _, err := releaseRawGlobal(c, badHistogram); err == nil || !strings.Contains(err.Error(), "histogram") {
		t.Fatalf("incomplete histogram accepted: %v", err)
	}
	badSocketKind := writeObservation(gateSocketSurge, []ReleaseGlobalObservation{{At: start, Kind: "channel", ActiveSockets: 200000}})
	if _, err := releaseRawGlobal(c, badSocketKind); err == nil || !strings.Contains(err.Error(), "socket recovery") {
		t.Fatalf("wrong socket observation accepted: %v", err)
	}
	badRecoveryIntegrity := writeObservation(gateAZLoss, []ReleaseGlobalObservation{{At: start, Kind: "recovery", LostAZCount: 1, MissingAcknowledgedInbox: 1}})
	if _, err := releaseRawGlobal(c, badRecoveryIntegrity); err == nil || !strings.Contains(err.Error(), "AZ recovery integrity") {
		t.Fatalf("missing acknowledged inbox accepted: %v", err)
	}
	badChannel := writeObservation(gateMixedSoak, []ReleaseGlobalObservation{{At: start, Kind: "channel"}})
	if _, err := releaseRawGlobal(c, badChannel); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unbound channel accepted: %v", err)
	}
	badHistogramBound := writeObservation(gateMixedSoak, []ReleaseGlobalObservation{{At: start, Kind: "origin", HTTPRequestsTotal: 1, OriginHistogram: map[string]int64{"invalid": 1}}})
	if _, err := releaseRawGlobal(c, badHistogramBound); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid histogram bound accepted: %v", err)
	}
	emptyCapture := writeObservation(gateAZLoss, nil)
	if _, err := releaseRawGlobal(c, emptyCapture); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("empty signed capture accepted: %v", err)
	}
	outOfWindow := writeObservation(gateAZLoss, []ReleaseGlobalObservation{{At: start.Add(-time.Second), Kind: "recovery", LostAZCount: 1}})
	if _, err := releaseRawGlobal(c, outOfWindow); !errors.Is(err, ErrInvalid) {
		t.Fatalf("out-of-window observation accepted: %v", err)
	}
	malformedArtifact := filepath.Join(t.TempDir(), "artifact.json")
	if err := os.WriteFile(malformedArtifact, []byte(`{"gate":"unexpected"}`), 0600); err != nil {
		t.Fatal(err)
	}
	c.GlobalArtifacts[gateSocketSurge] = malformedArtifact
	if err := releaseGlobals(c, &ReleaseResult{Files: map[string]string{}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unbound global artifact accepted: %v", err)
	}
	c.MaxSocketQueueMessages = 3
	if _, err := releaseRawGlobal(c, sockets); err == nil {
		t.Fatal("queue bound exceeded without rejection")
	}
	for path := range sockets.RawFiles {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var observation ReleaseGlobalObservation
		if err := json.Unmarshal(bytes.Split(raw, []byte{'\n'})[0], &observation); err != nil {
			t.Fatal(err)
		}
		observation.ActiveSockets++
		modified, err := json.Marshal(observation)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(modified, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := releaseRawGlobal(c, sockets); !errors.Is(err, ErrForbidden) {
			t.Fatalf("changed signed socket observation accepted: %v", err)
		}
	}
}

func TestReleaseDeviceReceiptsMatchAcceptedDelivery(t *testing.T) {
	ctx := context.Background()
	db, _ := repair958Open(t, t.TempDir())
	repair958Schema(t, db)
	store := Store{DB: db}
	started := time.Now().UTC().Add(-time.Minute)
	item, created, err := store.CommitTrigger(ctx, InboxItem{Identity: Identity{AccountID: "release-account", TriggerEventID: "release-event", TemplateVersion: "v2"}, Category: categoryWatchlist, Summary: "Threshold crossed", CreatedAt: started.Add(time.Second)}, []DeliveryAttempt{{Channel: channelPush, TargetID: "installation"}})
	if err != nil || !created {
		t.Fatalf("commit delivery: %+v %v %v", item, created, err)
	}
	claims, err := store.Claim(ctx, "worker", time.Now(), 1)
	if err != nil || len(claims) != 1 {
		t.Fatalf("claim delivery: %+v %v", claims, err)
	}
	attempt := claims[0].Deliveries[0]
	attempt.Attempt = 1
	attempt.State = stateAccepted
	attempt.ProviderReceipt = "provider-receipt"
	if err := store.Complete(ctx, "worker", claims[0].ID, attempt); err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	c := releaseCoverageConfig(t)
	c.DeviceCaptureKeys = map[string]string{"capture": base64.StdEncoding.EncodeToString(public)}
	receipt := ReleaseDeviceReceipt{NotificationID: item.ID, Provider: providerAPNS, ProviderReceipt: attempt.ProviderReceipt, DeviceInstallationID: attempt.TargetID, AppBuildDigest: c.AppBuildDigests[providerAPNS], Sandbox: true, ReceivedAt: time.Now().UTC(), CaptureKeyID: "capture"}
	message, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	receipt.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, message))
	write := func(receipts ...ReleaseDeviceReceipt) {
		t.Helper()
		raw, e := json.Marshal(receipts)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(c.DeviceReceiptFile, raw, 0600); e != nil {
			t.Fatal(e)
		}
	}
	write(receipt)
	file, err := os.Create(filepath.Join(t.TempDir(), "receipts.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	capture := &releaseCapture{file: file, max: 1 << 20}
	result := ReleaseResult{DeviceReceiveLatency: map[string]ReleaseLatency{}}
	if err := releaseDevices(ctx, db, c, started, capture, &result); err == nil || !strings.Contains(err.Error(), "both actual signed") {
		t.Fatalf("single provider accepted: %v", err)
	}
	if result.DeviceReceiveLatency[providerAPNS].Count != 0 {
		t.Fatal("incomplete device set produced latency")
	}
	secondItem, secondCreated, err := store.CommitTrigger(ctx, InboxItem{Identity: Identity{AccountID: "release-account", TriggerEventID: "release-fcm", TemplateVersion: "v2"}, Category: categoryWatchlist, Summary: "Threshold crossed", CreatedAt: started.Add(2 * time.Second)}, []DeliveryAttempt{{Channel: channelPush, TargetID: "installation-fcm"}})
	if err != nil || !secondCreated {
		t.Fatalf("FCM delivery commit: %+v %v %v", secondItem, secondCreated, err)
	}
	claims, err = store.Claim(ctx, "worker", time.Now(), 1)
	if err != nil || len(claims) != 1 {
		t.Fatalf("FCM delivery claim: %+v %v", claims, err)
	}
	secondAttempt := claims[0].Deliveries[0]
	secondAttempt.Attempt = 1
	secondAttempt.State = stateAccepted
	secondAttempt.ProviderReceipt = "fcm-receipt"
	if err := store.Complete(ctx, "worker", claims[0].ID, secondAttempt); err != nil {
		t.Fatal(err)
	}
	secondReceipt := ReleaseDeviceReceipt{NotificationID: secondItem.ID, Provider: providerFCM, ProviderReceipt: secondAttempt.ProviderReceipt, DeviceInstallationID: secondAttempt.TargetID, AppBuildDigest: c.AppBuildDigests[providerFCM], Sandbox: true, ReceivedAt: time.Now().UTC(), CaptureKeyID: "capture"}
	message, err = json.Marshal(secondReceipt)
	if err != nil {
		t.Fatal(err)
	}
	secondReceipt.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, message))
	write(receipt, secondReceipt)
	if err := releaseDevices(ctx, db, c, started, capture, &result); err != nil || result.DeviceReceiveLatency[providerAPNS].Count != 1 || result.DeviceReceiveLatency[providerFCM].Count != 1 {
		t.Fatalf("signed device receipts: %+v %v", result.DeviceReceiveLatency, err)
	}
	wrongReceipt := secondReceipt
	wrongReceipt.ProviderReceipt = "unmatched-receipt"
	wrongReceipt.Signature = ""
	message, err = json.Marshal(wrongReceipt)
	if err != nil {
		t.Fatal(err)
	}
	wrongReceipt.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, message))
	write(receipt, wrongReceipt)
	if err := releaseDevices(ctx, db, c, started, capture, &result); err == nil || !strings.Contains(err.Error(), "matching accepted provider effect") {
		t.Fatalf("unmatched provider receipt accepted: %v", err)
	}
	wrongReceipt = secondReceipt
	wrongReceipt.Provider = "unknown"
	write(receipt, wrongReceipt)
	if err := releaseDevices(ctx, db, c, started, capture, &result); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown provider accepted: %v", err)
	}
	futureReceipt := receipt
	futureReceipt.ReceivedAt = time.Now().UTC().Add(time.Hour)
	write(futureReceipt)
	if err := releaseDevices(ctx, db, c, started, capture, &result); !errors.Is(err, ErrInvalid) {
		t.Fatalf("future device receipt accepted: %v", err)
	}
	missingKeyReceipt := receipt
	missingKeyReceipt.CaptureKeyID = "missing"
	write(missingKeyReceipt)
	if err := releaseDevices(ctx, db, c, started, capture, &result); !errors.Is(err, ErrForbidden) {
		t.Fatalf("untrusted capture key accepted: %v", err)
	}
	if err := os.WriteFile(c.DeviceReceiptFile, []byte(`[] {}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := releaseDevices(ctx, db, c, started, capture, &result); !errors.Is(err, ErrInvalid) {
		t.Fatalf("trailing receipt data accepted: %v", err)
	}
	receipt.Signature = "invalid"
	write(receipt)
	if err := releaseDevices(ctx, db, c, started, capture, &result); !errors.Is(err, ErrForbidden) {
		t.Fatalf("unsigned receipt accepted: %v", err)
	}
}

func releaseCoverageTrigger(now time.Time) discovery.AlertTrigger {
	rule := discovery.AlertRule{ID: "release-rule", OwnerAccountID: "release-account", PoolAddress: "0x3333333333333333333333333333333333333333", Metric: "price", Threshold: "10", Direction: "crosses_above", Hysteresis: "1", Cooldown: time.Minute, Channels: []string{channelPush}, Revision: 1}
	metric := discovery.Metric{PoolAddress: rule.PoolAddress, Value: "11", Revision: 1, ObservedAt: now.Add(-time.Second), Quality: "complete"}
	return discovery.AlertTrigger{ID: "release-rule:1:1", Rule: rule, Metric: metric, CreatedAt: now}
}

func TestReleaseTriggerCohortAndBurstUseCommittedConsumer(t *testing.T) {
	ctx := context.Background()
	db, _ := repair958Open(t, t.TempDir())
	repair958Schema(t, db)
	peer := repair958Authorities(t)
	options := repair958Credentials(t)
	credentials, err := NewCredentials(ctx, options, "REPAIR958_D11")
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewAccountClient(peer.server.URL, peer.client, credentials, "REPAIR958_D11")
	if err != nil {
		t.Fatal(err)
	}
	c := releaseCoverageConfig(t)
	c.AllowedAccounts = []string{peer.account.AccountID}
	trigger := releaseCoverageTrigger(time.Now().UTC())
	trigger.Rule.OwnerAccountID = peer.account.AccountID
	raw, err := json.Marshal(trigger)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.TriggerFile, append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := releaseReadTriggers(c.TriggerFile, func(got discovery.AlertTrigger, gotRaw json.RawMessage) error {
		count++
		if got.ID != trigger.ID || !bytes.Equal(gotRaw, raw) {
			t.Fatalf("trigger changed: %+v %s", got, gotRaw)
		}
		return nil
	}); err != nil || count != 1 {
		t.Fatalf("trigger reader: count=%d err=%v", count, err)
	}
	if _, err := releasePrepareCohort(ctx, db, c); err == nil || !strings.Contains(err.Error(), "1200000") {
		t.Fatalf("short cohort accepted: %v", err)
	}
	file, err := os.Create(filepath.Join(t.TempDir(), "burst.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	capture := &releaseCapture{file: file, max: 1 << 20}
	result := ReleaseResult{}
	if err := releaseBurst(ctx, c, TriggerConsumer{Store: Store{DB: db}, Authority: authority}, capture, &result); err == nil || !strings.Contains(err.Error(), "incomplete actual burst") {
		t.Fatalf("short burst accepted: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if result.Scheduled != 1 || result.Completed != 1 || result.Succeeded != 1 || result.AdmissionFailures != 0 || result.TriggerLatency.Count != 1 {
		t.Fatalf("burst did not commit one trigger: %+v", result)
	}
	items, err := (Store{DB: db}).List(ctx, peer.account.AccountID, 10)
	if err != nil || len(items) != 1 || items[0].Identity.TriggerEventID != trigger.ID {
		t.Fatalf("committed trigger: %+v %v", items, err)
	}
	shortFile, err := os.Create(filepath.Join(t.TempDir(), "bounded.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	shortResult := ReleaseResult{}
	shortCapture := &releaseCapture{file: shortFile, max: 1}
	if err := releaseBurst(ctx, c, TriggerConsumer{Store: Store{DB: db}, Authority: authority}, shortCapture, &shortResult); !errors.Is(err, ErrUnavailable) || shortResult.Completed != 1 {
		t.Fatalf("bounded capture: %+v %v", shortResult, err)
	}
	if err := shortFile.Close(); err != nil {
		t.Fatal(err)
	}
	consumer := TriggerConsumer{Store: Store{DB: db}, Authority: authority}
	if err := consumer.Publish(ctx, "unknown", trigger.ID, raw); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown trigger kind accepted: %v", err)
	}
	if err := consumer.Publish(ctx, "threshold-crossing-v2", trigger.ID, json.RawMessage(`not-json`)); err == nil {
		t.Fatal("malformed trigger accepted")
	}
	if err := consumer.Publish(ctx, "threshold-crossing-v2", "other-event", raw); !errors.Is(err, ErrInvalid) {
		t.Fatalf("mismatched trigger identity accepted: %v", err)
	}
	unsupported := trigger
	unsupported.Rule.Channels = []string{"email"}
	unsupportedRaw, err := json.Marshal(unsupported)
	if err != nil {
		t.Fatal(err)
	}
	if err := consumer.Publish(ctx, "threshold-crossing-v2", unsupported.ID, unsupportedRaw); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unsupported delivery channel accepted: %v", err)
	}
	var many bytes.Buffer
	for i := 2; i <= 129; i++ {
		queued := trigger
		queued.Metric.Revision = uint64(i)
		queued.ID = fmt.Sprintf("release-rule:1:%d", i)
		if err := json.NewEncoder(&many).Encode(queued); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(c.TriggerFile, many.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	boundedFile, err := os.Create(filepath.Join(t.TempDir(), "admission.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	boundedResult := ReleaseResult{}
	err = releaseBurst(ctx, c, consumer, &releaseCapture{file: boundedFile, max: 1 << 20}, &boundedResult)
	if err == nil || boundedResult.AdmissionFailures == 0 || !strings.Contains(err.Error(), "bounded trigger admission") {
		t.Fatalf("bounded trigger queue: %+v %v", boundedResult, err)
	}
	if err := boundedFile.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.TriggerFile, []byte(`{"ID":"different"}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := releaseReadTriggers(c.TriggerFile, func(discovery.AlertTrigger, json.RawMessage) error { return nil }); !errors.Is(err, ErrInvalid) {
		t.Fatalf("incomplete trigger admitted: %v", err)
	}
	trigger.Rule.OwnerAccountID = "outside-account"
	raw, err = json.Marshal(trigger)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.TriggerFile, append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := releasePrepareCohort(ctx, db, c); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign cohort accepted: %v", err)
	}
	trigger.Rule.OwnerAccountID = peer.account.AccountID
	trigger.ID = "unbound-event"
	raw, err = json.Marshal(trigger)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.TriggerFile, append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := releasePrepareCohort(ctx, db, c); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unbound cohort event accepted: %v", err)
	}
}

func TestReleaseMeasurementsRejectsUntrustedRuntimeBeforeCapture(t *testing.T) {
	c := releaseCoverageConfig(t)
	ctx := context.Background()
	if _, err := RunReleaseMeasurements(ctx, ReleaseConfig{}, nil, nil, nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unbound measurement accepted: %v", err)
	}
	result, err := RunReleaseMeasurements(ctx, c, nil, nil, nil)
	if !errors.Is(err, ErrUnavailable) || result.Qualified || result.GlobalStatus != "not_run" {
		t.Fatalf("missing peers: %+v %v", result, err)
	}
}

func TestReleaseQueueReconcilesCommittedInbox(t *testing.T) {
	ctx := context.Background()
	db, _ := repair958Open(t, t.TempDir())
	repair958Schema(t, db)
	store := Store{DB: db}
	item, created, err := store.CommitTrigger(ctx, InboxItem{Identity: Identity{AccountID: "queue-account", TriggerEventID: "queue-event", TemplateVersion: "market-crossing-v2"}, Category: categoryWatchlist, Summary: "Threshold crossed", CreatedAt: time.Now().UTC()}, []DeliveryAttempt{})
	if err != nil || !created {
		t.Fatalf("queue inbox: %+v %v %v", item, created, err)
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `CREATE TEMP TABLE release_notification_cohort(id text,trigger_id text,account_id text,document jsonb)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO release_notification_cohort VALUES($1,$2,$3,'{}')`, item.ID, item.Identity.TriggerEventID, item.Identity.AccountID); err != nil {
		t.Fatal(err)
	}
	sample, err := releaseQueue(ctx, conn)
	if err != nil || sample.Expected != 1 || sample.Inbox != 1 || sample.Missing != 0 || sample.Pending != 0 || sample.Duplicates != 0 {
		t.Fatalf("queue reconciliation: %+v %v", sample, err)
	}
}

func TestReleaseMeasurementCapturesJourneyRefusal(t *testing.T) {
	ctx := context.Background()
	c := releaseCoverageConfig(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, `{"id":"other"}`); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	c.AllowedOrigins = []string{server.URL}
	for i := range c.Steps {
		c.Steps[i].URL = server.URL + "/" + c.Steps[i].Name
	}
	client := server.Client()
	client.Timeout = 5 * time.Second
	client.Transport.(*http.Transport).TLSClientConfig.MinVersion = tls.VersionTLS12
	db, _ := repair958Open(t, t.TempDir())
	peer := repair958Authorities(t)
	options := repair958Credentials(t)
	credentials, err := NewCredentials(ctx, options, "REPAIR958_D11")
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewAccountClient(peer.server.URL, peer.client, credentials, "REPAIR958_D11")
	if err != nil {
		t.Fatal(err)
	}
	result, err := RunReleaseMeasurements(ctx, c, db, authority, client)
	if err == nil || !strings.Contains(err.Error(), "identity assertion") || result.LanePassed || result.Qualified || len(result.Failures) != 1 {
		t.Fatalf("journey refusal: %+v %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(c.OutputDir, "result.json")); err != nil {
		t.Fatalf("measurement result absent: %v", err)
	}
	if digest := result.Files[filepath.Join(c.OutputDir, "raw.jsonl")]; !releaseDigest(digest) {
		t.Fatalf("capture digest: %s", digest)
	}
	if _, err := RunReleaseMeasurements(ctx, c, db, authority, client); err == nil || !os.IsExist(err) {
		t.Fatalf("existing capture directory reused: %v", err)
	}
}

func TestReleaseJourneyRejectsUnboundResponses(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"status", http.StatusForbidden, `{"id":"pool"}`, "actual status"},
		{"non-json", http.StatusOK, `not-json`, "non-JSON"},
		{"missing-id", http.StatusOK, `{"other":"pool"}`, "missing assertion pointer"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(scenario.status)
				if _, err := io.WriteString(w, scenario.body); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			c := releaseCoverageConfig(t)
			c.Steps = []ReleaseStep{{Name: "pool_launch", Method: http.MethodGet, URL: server.URL, Status: http.StatusOK, Assertions: []ReleaseAssertion{{Pointer: "/id", Equals: json.RawMessage(`"pool"`)}}}}
			file, err := os.Create(filepath.Join(t.TempDir(), "journey.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			result := ReleaseResult{HTTPLatency: map[string]ReleaseLatency{}}
			err = releaseJourney(context.Background(), c, server.Client(), &releaseCapture{file: file, max: 1024}, &result)
			if err == nil || !strings.Contains(err.Error(), scenario.want) {
				t.Fatalf("unbound response accepted: %v", err)
			}
		})
	}
}

func TestReleaseMeasurementRequiresCanonicalCohortAfterJourney(t *testing.T) {
	ctx := context.Background()
	c := releaseCoverageConfig(t)
	c.MaxCaptureBytes = 1 << 20
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, `{"id":"pool"}`); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	c.AllowedOrigins = []string{server.URL}
	for i := range c.Steps {
		c.Steps[i].URL = server.URL + "/" + c.Steps[i].Name
	}
	client := server.Client()
	client.Timeout = 5 * time.Second
	client.Transport.(*http.Transport).TLSClientConfig.MinVersion = tls.VersionTLS12
	db, _ := repair958Open(t, t.TempDir())
	repair958Schema(t, db)
	peer := repair958Authorities(t)
	options := repair958Credentials(t)
	credentials, err := NewCredentials(ctx, options, "REPAIR958_D11")
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewAccountClient(peer.server.URL, peer.client, credentials, "REPAIR958_D11")
	if err != nil {
		t.Fatal(err)
	}
	result, err := RunReleaseMeasurements(ctx, c, db, authority, client)
	if err == nil || result.LanePassed || result.Qualified || len(result.HTTPLatency) != len(c.Steps) || len(result.Failures) != 1 {
		t.Fatalf("cohort admission: %+v %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(c.OutputDir, "result.json")); err != nil {
		t.Fatalf("cohort result absent: %v", err)
	}
}

func TestReleaseJourneyRecordsTransportAndCredentialRefusal(t *testing.T) {
	c := releaseCoverageConfig(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := io.WriteString(w, `{"id":"pool"}`); err != nil {
			t.Error(err)
		}
	}))
	client := server.Client()
	c.Steps = c.Steps[:1]
	c.Steps[0].URL = server.URL
	file, err := os.Create(filepath.Join(t.TempDir(), "journey.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	capture := &releaseCapture{file: file, max: 4096}
	result := ReleaseResult{HTTPLatency: map[string]ReleaseLatency{}}
	c.Steps[0].BearerEnv = "RELEASE_JOURNEY_TOKEN"
	t.Setenv("RELEASE_JOURNEY_TOKEN", "")
	if err := releaseJourney(context.Background(), c, client, capture, &result); err == nil || !strings.Contains(err.Error(), "missing journey credential") {
		t.Fatalf("missing bearer accepted: %v", err)
	}
	c.Steps[0].BearerEnv = ""
	server.Close()
	if err := releaseJourney(context.Background(), c, client, capture, &result); err == nil || !strings.Contains(err.Error(), "transport unavailable") {
		t.Fatalf("closed peer accepted: %v", err)
	}
}

func TestReleaseJourneyRejectsNullCrossStepIdentity(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := io.WriteString(w, `{"id":null}`); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	c := releaseCoverageConfig(t)
	c.Steps = []ReleaseStep{
		{Name: "pool_launch", Method: http.MethodGet, URL: server.URL + "/pool", Status: http.StatusOK, Assertions: []ReleaseAssertion{{Pointer: "/id", Equals: json.RawMessage(`null`)}}},
		{Name: "feed", Method: http.MethodGet, URL: server.URL + "/feed", Status: http.StatusOK, Assertions: []ReleaseAssertion{{Pointer: "/id", FromStep: "pool_launch", FromPointer: "/id"}}},
	}
	file, err := os.Create(filepath.Join(t.TempDir(), "journey.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	result := ReleaseResult{HTTPLatency: map[string]ReleaseLatency{}}
	if err := releaseJourney(context.Background(), c, server.Client(), &releaseCapture{file: file, max: 4096}, &result); err == nil || !strings.Contains(err.Error(), "missing actual identity binding") {
		t.Fatalf("null identity bound across steps: %v", err)
	}
}
