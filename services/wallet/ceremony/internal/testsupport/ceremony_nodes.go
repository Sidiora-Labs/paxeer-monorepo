package testsupport

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/dealer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/attestor"
)

// CeremonyNodes runs independent production attestors; it never combines shares.
type CeremonyNodes struct {
	TLS             *TLSFiles
	gateway         *TLSFiles
	nodes           []ceremonyNode
	authority       *ecdsa.PrivateKey
	sequence        uint64
	dir, publicFile string
}

type ceremonyNode struct {
	id, certFile, keyFile, storeKeyFile, inventoryFile, dataDir string
	peerAddr, apiAddr                                           string
	pin                                                         [32]byte
	close                                                       func() error
	fingerprint                                                 func(string) ([32]byte, error)
	epoch                                                       func(string) (uint64, error)
	interrupt                                                   func()
}

func StartCeremonyNodes(t *testing.T) *CeremonyNodes {
	t.Helper()
	n := &CeremonyNodes{TLS: NewTLSFiles(t), gateway: NewTLSFiles(t), dir: t.TempDir()}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	n.authority = key
	raw, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	n.publicFile = filepath.Join(n.dir, "inventory-public.pem")
	writePEM(t, n.publicFile, "PUBLIC KEY", raw)
	peers, apis := make([]net.Listener, attestor.NodeCount), make([]net.Listener, attestor.NodeCount)
	for i := 0; i < attestor.NodeCount; i++ {
		id := fmt.Sprintf("n%d", i+1)
		cert, key := n.TLS.issue(t, id, x509.ExtKeyUsageServerAuth)
		// The same pinned node identity authenticates both sides of peer transport.
		tmpl := *cert
		tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
		der, err := x509.CreateCertificate(rand.Reader, &tmpl, n.TLS.ca, &key.PublicKey, n.TLS.caKey)
		if err != nil {
			t.Fatal(err)
		}
		cert, err = x509.ParseCertificate(der)
		if err != nil {
			t.Fatal(err)
		}
		cfg := ceremonyNode{id: id, certFile: filepath.Join(n.dir, id+".pem"), keyFile: filepath.Join(n.dir, id+"-tls.pem"), storeKeyFile: filepath.Join(n.dir, id+"-store.key"), inventoryFile: filepath.Join(n.dir, id+"-inventory.jwt"), dataDir: filepath.Join(n.dir, id), pin: attestor.SPKIHash(cert)}
		writePEM(t, cfg.certFile, "CERTIFICATE", der)
		keyDER, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			t.Fatal(err)
		}
		writePEM(t, cfg.keyFile, "EC PRIVATE KEY", keyDER)
		storeKey := make([]byte, 32)
		if _, err = rand.Read(storeKey); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(cfg.storeKeyFile, []byte(hex.EncodeToString(storeKey)), 0600); err != nil {
			t.Fatal(err)
		}
		for j := range storeKey {
			storeKey[j] = 0
		}
		peers[i] = ceremonyListener(t, "")
		apis[i] = ceremonyListener(t, "")
		cfg.peerAddr, cfg.apiAddr = peers[i].Addr().String(), apis[i].Addr().String()
		n.nodes = append(n.nodes, cfg)
	}
	n.PublishInventory(t, nil)
	for i := range n.nodes {
		n.open(t, i, peers[i], apis[i])
	}
	t.Cleanup(func() {
		for i := range n.nodes {
			if err := n.nodes[i].close(); err != nil {
				t.Error(err)
			}
		}
	})
	return n
}

func ceremonyListener(t *testing.T, address string) net.Listener {
	t.Helper()
	if address == "" {
		address = "127.0.0.1:0"
	}
	l, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func (n *CeremonyNodes) open(t *testing.T, i int, peer, api net.Listener) {
	t.Helper()
	node := &n.nodes[i]
	cfg := (dealer.ShareBundle{}).CeremonyNodeConfig()
	cfg.NodeID, cfg.DataDir, cfg.NodeKeyFile = node.id, node.dataDir, node.storeKeyFile
	cfg.CertFile, cfg.KeyFile, cfg.CAFile = node.certFile, node.keyFile, n.TLS.CAFile
	cfg.GatewayCAFile, cfg.OperatorCAFile = n.gateway.CAFile, n.TLS.CAFile
	cfg.InventoryFile, cfg.InventoryPublicKeyFile = node.inventoryFile, n.publicFile
	cfg.Issuer, cfg.Tenant, cfg.ChainID = "ceremony-fixture", "ceremony-fixture", 125
	cfg.PolicyJSON = []byte(`{"version":1,"defaults":{"chain_id":125,"kinds":["operator_verification"],"caps":{"native":{"per_transaction":"1000000000000000000","daily":"10000000000000000000"}},"rate_per_minute":1000}}`)
	for _, p := range n.nodes {
		cfg.AddPeer(p.id, p.peerAddr, hex.EncodeToString(p.pin[:]))
	}
	var closeStore func() error
	var armed atomic.Bool
	cfg.BeforeRefreshCommit = func() {
		if armed.Swap(false) {
			if err := closeStore(); err != nil {
				t.Error(err)
			}
		}
	}
	real, err := cfg.Open(peer, api)
	if err != nil {
		t.Fatal(err)
	}
	closeStore = real.Store.Close
	node.close = real.Close
	node.interrupt = func() { armed.Store(true) }
	node.epoch = func(keyID string) (uint64, error) { rec, err := real.Store.Get(keyID); return rec.Epoch, err }
	node.fingerprint = func(keyID string) ([32]byte, error) {
		var fingerprint [32]byte
		err := real.Store.WithShare(keyID, func(raw []byte) error {
			// Compare the scalar inside either production encoding, without returning it.
			var held struct {
				Bundle *struct {
					Share string `json:"share"`
				} `json:"bundle"`
				ECDSA json.RawMessage `json:"ecdsa"`
			}
			if err := json.Unmarshal(raw, &held); err != nil {
				return err
			}
			scalar := ""
			if held.Bundle != nil {
				scalar = held.Bundle.Share
			} else {
				var share struct {
					Share string `json:"share"`
				}
				if err := json.Unmarshal(held.ECDSA, &share); err != nil {
					return err
				}
				scalar = share.Share
			}
			if scalar == "" {
				return errors.New("production record has no scalar")
			}
			rawScalar, err := hex.DecodeString(scalar)
			if err != nil {
				return err
			}
			fingerprint = sha256.Sum256(rawScalar)
			for i := range rawScalar {
				rawScalar[i] = 0
			}
			return nil
		})
		return fingerprint, err
	}
}

func (n *CeremonyNodes) Config(t *testing.T) attestor.Config {
	t.Helper()
	members := make([]attestor.NodeConfig, len(n.nodes))
	for i, p := range n.nodes {
		members[i] = attestor.NodeConfig{ID: p.id, URL: "https://" + p.apiAddr, Pin: p.pin}
	}
	cfg, err := ExternalAttestorConfig(n.TLS.CertFile, n.TLS.KeyFile, n.TLS.CAFile, members)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func (n *CeremonyNodes) Reopen(t *testing.T, i int) {
	t.Helper()
	if err := n.nodes[i].close(); err != nil {
		t.Fatal(err)
	}
	n.open(t, i, ceremonyListener(t, n.nodes[i].peerAddr), ceremonyListener(t, n.nodes[i].apiAddr))
}

type CeremonyInventoryKey struct {
	KeyID      string   `json:"key_id"`
	Epoch      uint64   `json:"epoch"`
	Curve      string   `json:"curve"`
	PublicKey  string   `json:"public_key"`
	Owner      string   `json:"owner"`
	Operations []string `json:"operations"`
}

func (n *CeremonyNodes) PublishInventory(t *testing.T, keys [][]CeremonyInventoryKey) {
	t.Helper()
	n.sequence++
	members := make([]map[string]string, len(n.nodes))
	for i, p := range n.nodes {
		members[i] = map[string]string{"id": p.id, "spki_sha256": hex.EncodeToString(p.pin[:])}
	}
	encode := base64.RawURLEncoding.EncodeToString
	header := encode([]byte(`{"alg":"ES256","typ":"wallet-custody-inventory+jwt"}`))
	for i, p := range n.nodes {
		var approved []CeremonyInventoryKey
		if keys != nil {
			approved = keys[i]
		}
		raw, err := json.Marshal(map[string]any{"version": 1, "iss": "ceremony-fixture", "aud": "wallet-custody-inventory", "tenant": "ceremony-fixture", "sequence": fmt.Sprint(n.sequence), "iat": time.Now().Unix() - 1, "exp": time.Now().Unix() + 3600, "protocol": "wallet", "threshold": 3, "members": members, "keys": approved})
		if err != nil {
			t.Fatal(err)
		}
		input := header + "." + encode(raw)
		digest := sha256.Sum256([]byte(input))
		r, s, err := ecdsa.Sign(rand.Reader, n.authority, digest[:])
		if err != nil {
			t.Fatal(err)
		}
		signature := make([]byte, 64)
		r.FillBytes(signature[:32])
		s.FillBytes(signature[32:])
		if err = os.WriteFile(p.inventoryFile, []byte(input+"."+encode(signature)), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
