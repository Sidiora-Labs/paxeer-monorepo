package attestor_test

import (
	"context"
	"encoding/json"
	"errors"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/dealer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/attestor"
)

// The overlay lets this test reuse the real server's cluster harness across Go's
// internal-package boundary without exporting test hooks or changing other paths.
func TestRepair_247_RefreshEpochRecovery(t *testing.T) {
	if raw := os.Getenv("REPAIR247_CONFIG"); raw != "" {
		repair247Consumer(t, raw, os.Getenv("REPAIR247_PHASE"))
		return
	}
	dir := t.TempDir()
	source, err := format.Source([]byte(repair247ServerTest))
	if err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(dir, "server_test.go")
	if err := os.WriteFile(helper, source, 0600); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../../../attestor")
	if err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{
		filepath.Join(root, "internal/server/repair_247_integration_test.go"): helper,
	}})
	if err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0600); err != nil {
		t.Fatal(err)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-overlay", overlayPath, "-count=1", "-run", "^TestRepair247RealServers$", "./internal/server")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "REPAIR247_CONSUMER="+binary)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("real attestors: %v\n%s", err, out)
	}
	t.Logf("%s", out)
}

func repair247Consumer(t *testing.T, raw, phase string) {
	t.Helper()
	var cfg attestor.Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatal(err)
	}
	client, err := attestor.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	const ceremony, imported, session, keyID = "repair247", "repair247-import", "repair247-refresh", "repair247-key"
	bundles, pub, err := dealer.Split(dealer.Ed25519, edSecret(t), client.NodeIDs())
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := attestor.PublicKeyHex(pub)
	if err != nil {
		t.Fatal(err)
	}
	inspect := func() ([]attestor.DescribeResponse, error) {
		return client.InspectRefreshRecovery(ctx, ceremony, imported, session, keyID, publicKey, attestor.CurveEd25519, 0)
	}
	switch phase {
	case "import":
		if _, err := client.ImportCeremony(ctx, ceremony, imported, keyID, "user-0001", "0x4444444444444444444444444444444444444444", bundles, pub); err != nil {
			t.Fatal(err)
		}
		rows, err := inspect()
		if err != nil || len(rows) != 5 {
			t.Fatalf("initial inspection: %v %v", rows, err)
		}
		for _, row := range rows {
			if row.Refresh != nil || row.Epoch != 0 {
				t.Fatalf("unexpected initial state: %+v", row)
			}
		}
	case "interrupt":
		if _, err := client.RefreshCeremony(ctx, ceremony, imported, session, "", keyID, pub, 0); err == nil {
			t.Fatal("interrupted refresh succeeded")
		}
	case "recover", "complete":
		if phase == "recover" {
			if _, err := client.DescribeCeremony(ctx, keyID); !errors.Is(err, attestor.ErrDisagree) {
				t.Fatalf("strict description admitted split epochs: %v", err)
			}
			if err := client.ReconcileCeremony(ctx, ceremony, imported, session, keyID, publicKey, attestor.CurveEd25519, 1); !errors.Is(err, attestor.ErrDisagree) {
				t.Fatalf("reconciled before recovery: %v", err)
			}
		}
		rows, err := inspect()
		if err != nil || len(rows) != 5 {
			t.Fatalf("recovery inspection: %+v %v", rows, err)
		}
		epochs := map[uint64]int{}
		for _, row := range rows {
			epochs[row.Epoch]++
			if row.Refresh == nil || row.Refresh.Decision != "commit" || row.Refresh.BaseEpoch != 0 || row.Refresh.SessionID != session {
				t.Fatalf("durable identity missing: %+v", row)
			}
		}
		if phase == "recover" && (epochs[0] != 1 || epochs[1] != 4) {
			t.Fatalf("not a partial durable commit: %v", epochs)
		}
		if phase == "complete" && epochs[1] != 5 {
			t.Fatalf("recovered epoch lost on restart: %v", epochs)
		}
		type expectation struct {
			name, ceremony, imported, session, publicKey, curve string
			base                                                uint64
		}
		cases := []expectation{
			{"ceremony", "other", imported, session, publicKey, attestor.CurveEd25519, 0},
			{"import", ceremony, "other", session, publicKey, attestor.CurveEd25519, 0},
			{"refresh", ceremony, imported, "other", publicKey, attestor.CurveEd25519, 0},
			{"existing-key", ceremony, "", session, publicKey, attestor.CurveEd25519, 0},
			{"epoch", ceremony, imported, session, publicKey, attestor.CurveEd25519, 1},
			{"overflow", ceremony, imported, session, publicKey, attestor.CurveEd25519, ^uint64(0)},
			{"curve", ceremony, imported, session, publicKey, attestor.CurveSecp256k1, 0},
			{"public-key", ceremony, imported, session, "", attestor.CurveEd25519, 0},
		}
		for _, tc := range cases {
			if _, err := client.InspectRefreshRecovery(ctx, tc.ceremony, tc.imported, tc.session, keyID, tc.publicKey, tc.curve, tc.base); err == nil {
				t.Fatalf("accepted conflicting %s", tc.name)
			}
		}
		wrong := cfg
		wrong.Nodes = append([]attestor.NodeConfig(nil), cfg.Nodes...)
		wrong.Nodes[0].Pin = cfg.Nodes[1].Pin
		pinned, err := attestor.New(wrong)
		if err != nil {
			t.Fatal(err)
		}
		defer pinned.Close()
		if _, err := pinned.InspectRefreshRecovery(ctx, ceremony, imported, session, keyID, publicKey, attestor.CurveEd25519, 0); !errors.Is(err, attestor.ErrPinMismatch) {
			t.Fatalf("inspection accepted wrong pin: %v", err)
		}
		if phase == "recover" {
			if _, err := client.RefreshCeremony(ctx, ceremony, imported, session, "", keyID, pub, 0); err != nil {
				t.Fatal(err)
			}
		}
		if err := client.ReconcileCeremony(ctx, ceremony, imported, session, keyID, publicKey, attestor.CurveEd25519, 1); err != nil {
			t.Fatalf("strict final reconciliation: %v", err)
		}
	case "reject":
		if _, err := inspect(); !errors.Is(err, attestor.ErrDisagree) {
			t.Fatalf("accepted inconsistent durable member identity: %v", err)
		}
	default:
		t.Fatalf("unknown phase %q", phase)
	}
}

const repair247ServerTest = `package server
import (
 "context"
 "crypto/x509"
 "encoding/json"
 "errors"
 "os"
 "os/exec"
 "path/filepath"
 "testing"
 "time"

 "github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/store"
 "github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/transport"
)

func TestRepair247RealServers(t *testing.T) {
 c := newTestClusterWith(t, 5, true, testPolicy(), func(o *Options) { o.PeerTimeout = 5*time.Second })
 const keyID, session = "repair247-key", "repair247-refresh"
 approveTestKeys(t, c, c.ids, InventoryKey{KeyID:keyID, Curve:"ed25519", PublicKey:"d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a", Owner:testOwner, Operations:[]string{"import","refresh","sign"}})
 operator := c.operator.issue(t, t.TempDir(), "repair247-operator", x509.ExtKeyUsageClientAuth)
 nodes := []map[string]any{}
 for i, cfg := range c.configs { nodes = append(nodes, map[string]any{"ID":cfg.id,"URL":"https://"+cfg.apiAddr,"Pin":transport.SPKIHash(c.configs[i].cert.cert)}) }
 config, err := json.Marshal(map[string]any{"CertFile":operator.certPath,"KeyFile":operator.keyPath,"CAFile":c.ca.pemPath,"Nodes":nodes})
 if err != nil { t.Fatal(err) }
 consume := func(phase string) {
  t.Helper()
  cmd := exec.Command(os.Getenv("REPAIR247_CONSUMER"), "-test.run=^TestRepair_247_RefreshEpochRecovery$", "-test.v")
  cmd.Env = append(os.Environ(), "REPAIR247_CONFIG="+string(config), "REPAIR247_PHASE="+phase)
  out, err := cmd.CombinedOutput()
  if err != nil { t.Fatalf("consumer %s: %v\n%s", phase, err, out) }
  t.Logf("%s", out)
 }
 consume("import")
 stopped := c.nodes[4]
 interrupted := make(chan error, 1)
 stopped.server.afterStage = func(key string) {
  committed, err := stopped.server.decideRefresh(context.Background(), key, session, 1, c.ids)
  if err == nil && !committed { err = errors.New("real coordinator did not commit") }
  stopped.stop()
  interrupted <- err
 }
 consume("interrupt")
 select { case err := <-interrupted: if err != nil { t.Fatal(err) }; case <-time.After(time.Minute): t.Fatal("no interruption") }
 // Reopen every real store and server: neither HTTP results nor in-memory votes
 // may supply the epoch transition recovered by the ceremony consumer.
 for _, node := range append([]*testNode(nil), c.nodes...) { c.restart(t, node) }
 stage, err := c.nodes[4].store.GetStaged(keyID)
 if err != nil || stage.Epoch != 1 { t.Fatalf("retained real protocol stage: %+v %v", stage, err) }
 for i, node := range c.nodes {
  held, err := node.store.Get(keyID)
  if err != nil { t.Fatal(err) }
  want := uint64(1); if i == 4 { want = 0 }
  if held.Epoch != want { t.Fatalf("%s epoch %d want %d", node.id, held.Epoch, want) }
 }
 // Each member's signed inventory must authorize its held epoch. Issue the
 // updated epoch to advanced members while the interrupted member retains base.
 authority := c.importInventory
 current, err := os.ReadFile(authority.path); if err != nil { t.Fatal(err) }
 base := filepath.Join(t.TempDir(), "base.jwt")
 if err := os.WriteFile(base, current, 0600); err != nil { t.Fatal(err) }
 allNodes := c.nodes
 c.nodes = allNodes[:4]
 c.publishImportEpoch(t, keyID, 1)
 c.nodes = allNodes
 pins := map[string]string{}; for _, peer := range c.peers { pins[peer.ID] = peer.SPKISHA256 }
 inventory, err := NewInventory(base, authority.public, "native-corpus", "tenant-a", c.nodes[4].store, pins)
 if err != nil { t.Fatal(err) }
 c.nodes[4].server.opts.Inventory = inventory
 consume("recover")
 c.publishImportEpoch(t, keyID, 1)
 for _, node := range append([]*testNode(nil), c.nodes...) { c.restart(t, node) }
 consume("complete")
 // Corrupt durable metadata through the real encrypted record codec; descriptions
 // still come from the authenticated handler, never from a fabricated response.
 node := c.nodes[2]
 original, err := node.store.CeremonyRefresh(keyID); if err != nil { t.Fatal(err) }
 mutations := []func(*store.CeremonyRefresh){
  func(s *store.CeremonyRefresh){s.KeyID="other"},
  func(s *store.CeremonyRefresh){s.Curve="secp256k1"},
  func(s *store.CeremonyRefresh){s.CeremonyID="other"},
  func(s *store.CeremonyRefresh){s.ImportSessionID="other"},
  func(s *store.CeremonyRefresh){s.SessionID="other"},
  func(s *store.CeremonyRefresh){s.BaseEpoch=1},
  func(s *store.CeremonyRefresh){s.Decision="abort"},
  func(s *store.CeremonyRefresh){s.Participants=[]string{"node-1"}},
  func(s *store.CeremonyRefresh){s.PublicKey=[]byte{1}},
  func(s *store.CeremonyRefresh){s.ExistingKey=true},
  func(s *store.CeremonyRefresh){s.AuditSequence=0},
 }
 for _, mutate := range mutations {
  changed := original; mutate(&changed)
  if err := node.store.UpdateRecord(store.RecordRefresh, keyID, func([]byte)([]byte,error){return json.Marshal(changed)}); err != nil {t.Fatal(err)}
  consume("reject")
  if err := node.store.UpdateRecord(store.RecordRefresh, keyID, func([]byte)([]byte,error){return json.Marshal(original)}); err != nil {t.Fatal(err)}
 }
}
`
