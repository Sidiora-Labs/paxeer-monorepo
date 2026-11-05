package testsupport

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/dealer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/attestor"
)

// CeremonyNodes runs independent production attestors; it never combines shares.
type CeremonyNodes struct {
	TLS                           *TLSFiles
	gateway                       *TLSFiles
	nodes                         []ceremonyNode
	authority                     *ecdsa.PrivateKey
	sequence                      uint64
	dir, publicFile, producerFile string
	rpcURL, issuerURL, tokensFile string
	issuerKey                     *rsa.PrivateKey
	approved                      [][]CeremonyInventoryKey
	ownerTokens                   map[string]string
	inventoryMu                   sync.Mutex
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

func StartCeremonyNodes(t *testing.T) *CeremonyNodes { return startCeremonyNodes(t, "") }

func StartWalletNodes(t *testing.T, rpcURL string) *CeremonyNodes {
	return startCeremonyNodes(t, rpcURL)
}

func startCeremonyNodes(t *testing.T, rpcURL string) *CeremonyNodes {
	t.Helper()
	n := &CeremonyNodes{TLS: NewTLSFiles(t), gateway: NewTLSFiles(t), dir: t.TempDir(), rpcURL: rpcURL}
	if rpcURL != "" {
		n.startIssuer(t)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	n.authority = key
	n.producerFile = filepath.Join(n.dir, "authority-private.pem")
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	writePEM(t, n.producerFile, "PRIVATE KEY", privateDER)
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
	cfg.Issuer, cfg.Tenant, cfg.ChainID = n.inventoryIssuer(), "ceremony-fixture", 125
	if n.rpcURL != "" {
		cfg.JWKSURL, cfg.TokenIssuer, cfg.TokenAudience = n.issuerURL+"/jwks", n.issuerURL, "authenticated"
		cfg.RPCURL = n.rpcURL
		cfg.KernelPolicyJSON = []byte(`{"version":1,"defaults":{"modules":{"asset":[5]}}}`)
		cfg.ApproveRefresh = func(keyID string, epoch uint64) { n.approveEpoch(t, i, keyID, epoch) }
	}
	cfg.PolicyJSON = []byte(`{"version":1,"defaults":{"chain_id":125,"kinds":["operator_verification","lx_bind"],"caps":{"native":{"per_transaction":"1000000000000000000","daily":"10000000000000000000"}},"rate_per_minute":1000}}`)
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
	if n.rpcURL != "" {
		cfg.GatewayCertFile, cfg.GatewayKeyFile, cfg.OwnerTokensFile = n.gateway.CertFile, n.gateway.KeyFile, n.tokensFile
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
	n.inventoryMu.Lock()
	defer n.inventoryMu.Unlock()
	n.approved = make([][]CeremonyInventoryKey, len(n.nodes))
	for i := range n.nodes {
		if keys != nil {
			n.approved[i] = append([]CeremonyInventoryKey(nil), keys[i]...)
		}
	}
	n.publishInventory(t)
}

func (n *CeremonyNodes) approveEpoch(t *testing.T, node int, keyID string, epoch uint64) {
	n.inventoryMu.Lock()
	defer n.inventoryMu.Unlock()
	found := false
	for i := range n.approved[node] {
		if n.approved[node][i].KeyID == keyID {
			n.approved[node][i].Epoch = epoch
			found = true
		}
	}
	if !found {
		t.Errorf("no approved refresh for %s", keyID)
		return
	}
	n.publishInventory(t)
}

func (n *CeremonyNodes) publishInventory(t *testing.T) {
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
		if n.approved != nil {
			approved = n.approved[i]
		}
		raw, err := json.Marshal(map[string]any{"version": 1, "iss": n.inventoryIssuer(), "aud": "wallet-custody-inventory", "tenant": "ceremony-fixture", "sequence": fmt.Sprint(n.sequence), "iat": time.Now().Unix() - 1, "exp": time.Now().Unix() + 3600, "protocol": "wallet", "threshold": 3, "members": members, "keys": approved})
		if err != nil {
			t.Error(err)
			return
		}
		input := header + "." + encode(raw)
		digest := sha256.Sum256([]byte(input))
		r, s, err := ecdsa.Sign(rand.Reader, n.authority, digest[:])
		if err != nil {
			t.Error(err)
			return
		}
		signature := append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)
		// Atomic replacement keeps concurrent real handlers from reading partial JWTs.
		tmp := p.inventoryFile + ".next"
		if err = os.WriteFile(tmp, []byte(input+"."+encode(signature)), 0600); err != nil {
			t.Error(err)
			return
		}
		if err = os.Rename(tmp, p.inventoryFile); err != nil {
			t.Error(err)
			return
		}
	}
}

func (n *CeremonyNodes) Approve(t *testing.T, keys []CeremonyInventoryKey) {
	t.Helper()
	all := make([][]CeremonyInventoryKey, len(n.nodes))
	for i := range all {
		all[i] = keys
	}
	n.PublishInventory(t, all)
}

func (n *CeremonyNodes) startIssuer(t *testing.T) {
	t.Helper()
	var err error
	n.issuerKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	encode := base64.RawURLEncoding.EncodeToString
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jwks" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{"kty": "RSA", "kid": "ceremony-owner", "alg": "RS256", "use": "sig", "n": encode(n.issuerKey.N.Bytes()), "e": encode(big.NewInt(int64(n.issuerKey.E)).Bytes())}}})
	}))
	t.Cleanup(srv.Close)
	n.issuerURL = srv.URL
	n.tokensFile = filepath.Join(n.dir, "owner-tokens.json")
	n.ownerTokens = map[string]string{}
	if err = os.WriteFile(n.tokensFile, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
}

func (n *CeremonyNodes) OwnerToken(t *testing.T, owner string) string {
	t.Helper()
	if token := n.ownerTokens[owner]; token != "" {
		return token
	}
	encode := base64.RawURLEncoding.EncodeToString
	header := encode([]byte(`{"alg":"RS256","kid":"ceremony-owner","typ":"JWT"}`))
	now := time.Now().Unix()
	raw, err := json.Marshal(map[string]any{"iss": n.issuerURL, "sub": owner, "aud": "authenticated", "iat": now, "exp": now + 3600})
	if err != nil {
		t.Fatal(err)
	}
	input := header + "." + encode(raw)
	digest := sha256.Sum256([]byte(input))
	signature, err := rsa.SignPKCS1v15(rand.Reader, n.issuerKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	token := input + "." + encode(signature)
	n.ownerTokens[owner] = token
	raw, err = json.Marshal(n.ownerTokens)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(n.tokensFile, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return token
}

// Clone quiescent encrypted stores with the same pinned identities and separate
// listeners. Both groups retain their real protocol, auth and inventory checks.
func (n *CeremonyNodes) Clone(t *testing.T) *CeremonyNodes {
	t.Helper()
	clone := &CeremonyNodes{TLS: n.TLS, gateway: n.gateway, authority: n.authority, publicFile: n.publicFile, producerFile: n.producerFile, dir: t.TempDir(), rpcURL: n.rpcURL, issuerURL: n.issuerURL, issuerKey: n.issuerKey, tokensFile: n.tokensFile, ownerTokens: n.ownerTokens, sequence: n.sequence}
	for i, original := range n.nodes {
		if err := n.nodes[i].close(); err != nil {
			t.Fatal(err)
		}
		copied := original
		copied.dataDir = filepath.Join(clone.dir, original.id)
		if err := os.CopyFS(copied.dataDir, os.DirFS(original.dataDir)); err != nil {
			t.Fatal(err)
		}
		copied.inventoryFile = filepath.Join(clone.dir, original.id+"-inventory.jwt")
		peer, api := ceremonyListener(t, ""), ceremonyListener(t, "")
		copied.peerAddr, copied.apiAddr = peer.Addr().String(), api.Addr().String()
		copied.close = nil
		clone.nodes = append(clone.nodes, copied)
		peer.Close()
		api.Close()
	}
	clone.PublishInventory(t, n.approved)
	for i := range n.nodes {
		n.open(t, i, ceremonyListener(t, n.nodes[i].peerAddr), ceremonyListener(t, n.nodes[i].apiAddr))
	}
	for i := range clone.nodes {
		clone.open(t, i, ceremonyListener(t, clone.nodes[i].peerAddr), ceremonyListener(t, clone.nodes[i].apiAddr))
	}
	t.Cleanup(func() {
		for i := range clone.nodes {
			if err := clone.nodes[i].close(); err != nil {
				t.Error(err)
			}
		}
	})
	return clone
}

func (n *CeremonyNodes) Fingerprint(t *testing.T) [32]byte {
	t.Helper()
	hash := sha256.New()
	for _, node := range n.nodes {
		if err := filepath.WalkDir(node.dataDir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(n.dir, path)
			if err != nil {
				return err
			}
			fmt.Fprintf(hash, "%s:%d:", relative, len(raw))
			hash.Write(raw)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result
}

func (n *CeremonyNodes) inventoryIssuer() string {
	if n.rpcURL != "" {
		return "http://127.0.0.1:1/auth/v1"
	}
	return "ceremony-fixture"
}

// GatewayAuthority supplies the same real signed inventory consumed by the nodes.
func (n *CeremonyNodes) GatewayAuthority() map[string]string {
	return map[string]string{"WALLET_CUSTODY_INVENTORY_FILE": n.nodes[0].inventoryFile, "WALLET_CUSTODY_INVENTORY_PUBLIC_KEY_FILE": n.publicFile, "WALLET_IDENTITY_BINDING_TENANT": "ceremony-fixture", "WALLET_IDENTITY_BINDING_PRIVATE_KEY_FILE": n.producerFile}
}
