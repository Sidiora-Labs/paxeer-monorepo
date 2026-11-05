package testsupport

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/dealer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/attestor"
)

func repairM175Config(t *testing.T) (*TLSFiles, []attestor.NodeConfig) {
	t.Helper()
	files := NewTLSFiles(t)
	nodes := make([]attestor.NodeConfig, attestor.NodeCount)
	for i := range nodes {
		id := fmt.Sprintf("member-%d", i+1)
		_, pin := files.ServerCertificate(t, id)
		nodes[i] = attestor.NodeConfig{ID: id, URL: fmt.Sprintf("https://127.0.0.1:%d", 20000+i), Pin: pin}
	}
	return files, nodes
}

func TestRepairM_175_ExternalConfigProductionClient(t *testing.T) {
	files, nodes := repairM175Config(t)
	want := append([]attestor.NodeConfig(nil), nodes...)
	cfg, err := ExternalAttestorConfig(files.CertFile, files.KeyFile, files.CAFile, nodes)
	if err != nil {
		t.Fatal(err)
	}
	nodes[0].ID = "changed-by-caller"
	if !reflect.DeepEqual(cfg.Nodes, want) {
		t.Fatal("external configuration aliases caller-owned membership")
	}
	client, err := attestor.New(cfg)
	if err != nil {
		t.Fatalf("production client rejected external configuration: %v", err)
	}
	defer client.Close()
	for i, member := range client.Membership() {
		if member.ID != want[i].ID || member.Pin != want[i].Pin {
			t.Fatalf("production client changed pinned member %d: %+v", i, member)
		}
	}
}

func TestRepairM_175_ExternalConfigIndependentMembers(t *testing.T) {
	files, nodes := repairM175Config(t)
	for _, tc := range []struct {
		name string
		edit func([]attestor.NodeConfig) []attestor.NodeConfig
	}{
		{"missing-member", func(n []attestor.NodeConfig) []attestor.NodeConfig { return n[:4] }},
		{"duplicate-id", func(n []attestor.NodeConfig) []attestor.NodeConfig { n[1].ID = n[0].ID; return n }},
		{"missing-id", func(n []attestor.NodeConfig) []attestor.NodeConfig { n[0].ID = ""; return n }},
		{"duplicate-endpoint", func(n []attestor.NodeConfig) []attestor.NodeConfig { n[1].URL = n[0].URL; return n }},
		{"duplicate-identity", func(n []attestor.NodeConfig) []attestor.NodeConfig { n[1].Pin = n[0].Pin; return n }},
		{"missing-pin", func(n []attestor.NodeConfig) []attestor.NodeConfig { n[0].Pin = [32]byte{}; return n }},
		{"plaintext", func(n []attestor.NodeConfig) []attestor.NodeConfig { n[0].URL = "http://127.0.0.1:20000"; return n }},
		{"userinfo", func(n []attestor.NodeConfig) []attestor.NodeConfig {
			n[0].URL = "https://user@127.0.0.1:20000"
			return n
		}},
		{"path", func(n []attestor.NodeConfig) []attestor.NodeConfig { n[0].URL += "/simulator"; return n }},
		{"query", func(n []attestor.NodeConfig) []attestor.NodeConfig { n[0].URL += "?node=other"; return n }},
		{"fragment", func(n []attestor.NodeConfig) []attestor.NodeConfig { n[0].URL += "#other"; return n }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := tc.edit(append([]attestor.NodeConfig(nil), nodes...))
			if _, err := ExternalAttestorConfig(files.CertFile, files.KeyFile, files.CAFile, changed); !errors.Is(err, attestor.ErrConfig) {
				t.Fatalf("invalid member boundary admitted: %v", err)
			}
		})
	}
	for _, tc := range []struct{ name, cert, key, ca string }{
		{"missing-certificate", "", files.KeyFile, files.CAFile},
		{"unreadable-certificate", filepath.Join(t.TempDir(), "absent.pem"), files.KeyFile, files.CAFile},
		{"missing-key", files.CertFile, "", files.CAFile},
		{"wrong-key-file", files.CertFile, files.CAFile, files.CAFile},
		{"missing-ca", files.CertFile, files.KeyFile, ""},
		{"invalid-ca", files.CertFile, files.KeyFile, files.KeyFile},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ExternalAttestorConfig(tc.cert, tc.key, tc.ca, nodes); !errors.Is(err, attestor.ErrConfig) {
				t.Fatalf("invalid production TLS material admitted: %v", err)
			}
		})
	}
}

func TestRepairM_175_RealProtocolRefreshAndDescribe(t *testing.T) {
	nodes := StartCeremonyNodes(t)
	client, err := attestor.New(nodes.Config(t))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	const ceremonyID, importID, refreshID, keyID = "repair-m175", "repair-m175-import", "repair-m175-refresh", "repair-m175-key"
	bundles, pub, err := dealer.Split(dealer.Secp256k1, big.NewInt(0x5eed), client.NodeIDs())
	if err != nil {
		t.Fatal(err)
	}
	publicBytes, err := attestor.PublicKeyBytes(pub)
	if err != nil {
		t.Fatal(err)
	}
	approval := CeremonyInventoryKey{KeyID: keyID, Curve: "secp256k1", PublicKey: hex.EncodeToString(publicBytes), Owner: "repair-owner", Operations: []string{"import", "refresh", "sign"}}
	repairM175Approve(t, nodes, approval, nil)
	if _, err := client.ImportCeremony(ctx, ceremonyID, importID, keyID, "repair-owner", "", bundles, pub); err != nil {
		t.Fatal(err)
	}
	before := make([][32]byte, len(nodes.nodes))
	for i, n := range nodes.nodes {
		before[i], err = n.fingerprint(keyID)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, bundle := range bundles {
		words := bundle.Share.Bits()
		for i := range words {
			words[i] = 0
		}
		bundle.Share.SetInt64(0)
	}
	t.Run("distributed-refresh", func(t *testing.T) {
		if _, err := client.RefreshCeremony(ctx, ceremonyID, importID, refreshID, "", keyID, pub, 0); err != nil {
			t.Fatal(err)
		}
		for i, bundle := range bundles {
			after, err := nodes.nodes[i].fingerprint(keyID)
			if err != nil {
				t.Fatal(err)
			}
			if after == before[i] {
				t.Errorf("%s advanced its epoch without distributed share refresh", bundle.ParticipantID)
			}
		}
	})
	t.Run("durable-description", func(t *testing.T) {
		for i := range nodes.nodes {
			nodes.Reopen(t, i)
		}
		rows, err := client.DescribeCeremony(ctx, keyID)
		if err != nil {
			t.Fatalf("real ceremony consumer cannot inspect durable commit identity: %v", err)
		}
		if len(rows) != attestor.NodeCount {
			t.Fatalf("described %d members, want %d", len(rows), attestor.NodeCount)
		}
		for _, row := range rows {
			if row.Epoch != 1 || row.CeremonyID != ceremonyID || row.ImportSessionID != importID || row.RefreshSessionID != refreshID || row.RefreshState != "complete" || row.Refresh == nil || row.Refresh.Decision != "commit" {
				t.Errorf("missing durable ceremony commit identity: %+v", row)
			}
		}
	})
}

func repairM175Approve(t *testing.T, nodes *CeremonyNodes, key CeremonyInventoryKey, epochs []uint64) {
	t.Helper()
	approvals := make([][]CeremonyInventoryKey, len(nodes.nodes))
	for i := range approvals {
		entry := key
		if epochs != nil {
			entry.Epoch = epochs[i]
		}
		approvals[i] = []CeremonyInventoryKey{entry}
	}
	nodes.PublishInventory(t, approvals)
}

func TestRepairM_175_PartialCommitAndReopen(t *testing.T) {
	nodes := StartCeremonyNodes(t)
	client, err := attestor.New(nodes.Config(t))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	const ceremonyID, importID, refreshID, keyID = "repair-m175-partial", "repair-m175-partial-import", "repair-m175-partial-refresh", "repair-m175-partial-key"
	seed := sha256.Sum256([]byte("repair m175 disposable ed25519 import"))
	private := ed25519.NewKeyFromSeed(seed[:])
	// Convert the disposable import scalar exactly as the production dealer does.
	digest := sha512.Sum512(seed[:])
	digest[0] &= 248
	digest[31] &= 127
	digest[31] |= 64
	scalarBytes := make([]byte, 32)
	for i := range scalarBytes {
		scalarBytes[31-i] = digest[i]
	}
	scalar := new(big.Int).SetBytes(scalarBytes)
	ec, _ := dealer.Ed25519.Elliptic()
	scalar.Mod(scalar, ec.Params().N)
	bundles, pub, err := dealer.Split(dealer.Ed25519, scalar, client.NodeIDs())
	if err != nil {
		t.Fatal(err)
	}
	publicBytes, err := attestor.PublicKeyBytes(pub)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(publicBytes, private.Public().(ed25519.PublicKey)) {
		t.Fatal("import public identity differs")
	}
	for i := range private {
		private[i] = 0
	}
	for i := range scalarBytes {
		scalarBytes[i] = 0
	}
	for i := range digest {
		digest[i] = 0
	}
	for i := range seed {
		seed[i] = 0
	}
	approval := CeremonyInventoryKey{KeyID: keyID, Curve: "ed25519", PublicKey: hex.EncodeToString(publicBytes), Owner: "repair-owner", Operations: []string{"import", "refresh", "sign"}}
	repairM175Approve(t, nodes, approval, nil)
	if _, err = client.ImportCeremony(ctx, ceremonyID, importID, keyID, "repair-owner", "0x4444444444444444444444444444444444444444", bundles, pub); err != nil {
		t.Fatal(err)
	}
	before := make([][32]byte, len(nodes.nodes))
	for i, n := range nodes.nodes {
		before[i], err = n.fingerprint(keyID)
		if err != nil {
			t.Fatal(err)
		}
	}
	// Destroy all dealer shares before interrupting the production commit path.
	for _, bundle := range bundles {
		words := bundle.Share.Bits()
		for i := range words {
			words[i] = 0
		}
		bundle.Share.SetInt64(0)
	}
	nodes.nodes[4].interrupt()
	if _, err = client.RefreshCeremony(ctx, ceremonyID, importID, refreshID, "", keyID, pub, 0); err == nil {
		t.Fatal("refresh succeeded despite a closed participant store at commit")
	}
	nodes.Reopen(t, 4)
	epochs := make([]uint64, len(nodes.nodes))
	for i, n := range nodes.nodes {
		epochs[i], err = n.epoch(keyID)
		if err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(epochs, []uint64{1, 1, 1, 1, 0}) {
		t.Fatalf("partial commit epochs: %v", epochs)
	}
	if _, err = client.DescribeCeremony(ctx, keyID); !errors.Is(err, attestor.ErrDisagree) {
		t.Fatalf("ordinary description accepted partial epochs: %v", err)
	}
	rows, err := client.InspectRefreshRecovery(ctx, ceremonyID, importID, refreshID, keyID, hex.EncodeToString(publicBytes), "ed25519", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Refresh == nil || row.Refresh.Decision != "commit" {
			t.Fatalf("missing durable commit decision: %+v", row)
		}
	}
	repairM175Approve(t, nodes, approval, epochs)
	if _, err = client.RefreshCeremony(ctx, ceremonyID, importID, refreshID, "", keyID, pub, 0); err != nil {
		t.Fatal(err)
	}
	for i := range nodes.nodes {
		nodes.Reopen(t, i)
	}
	rows, err = client.DescribeCeremony(ctx, keyID)
	if err != nil {
		t.Fatal(err)
	}
	for i, row := range rows {
		if row.Epoch != 1 || row.CeremonyID != ceremonyID || row.ImportSessionID != importID || row.RefreshSessionID != refreshID || row.RefreshState != "complete" || row.Refresh == nil || row.Refresh.Decision != "commit" {
			t.Fatalf("reopened commit identity: %+v", row)
		}
		after, err := nodes.nodes[i].fingerprint(keyID)
		if err != nil {
			t.Fatal(err)
		}
		if after == before[i] {
			t.Fatalf("%s did not refresh its scalar", row.NodeID)
		}
	}
	repairM175Approve(t, nodes, approval, []uint64{1, 1, 1, 1, 1})
	signed, err := client.SignVerificationCeremony(ctx, ceremonyID, "repair-m175-threshold-proof", "", keyID, importID, pub, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(signed.AuditSeqs) != attestor.SignQuorum || !ed25519.Verify(publicBytes, signed.Message, signed.Signature) {
		t.Fatal("real threshold verification did not preserve the original identity")
	}
}
