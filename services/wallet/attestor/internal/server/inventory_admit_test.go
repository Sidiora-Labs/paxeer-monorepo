package server

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/auth/agent"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/store"
)

type inventoryAdmitFixture struct {
	key    *ecdsa.PrivateKey
	path   string
	public string
	store  *store.Store
	pins   map[string]string
	ids    []string
}

func newInventoryAdmitFixture(t *testing.T, sequence uint64, keys []InventoryKey, expiry time.Time) *inventoryAdmitFixture {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	public := filepath.Join(dir, "authority.pem")
	if err := os.WriteFile(public, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	ids := []string{"node-1", "node-2", "node-3", "node-4", "node-5"}
	pins := make(map[string]string, len(ids))
	members := make([]InventoryMember, 0, len(ids))
	for n, id := range ids {
		pin := sha256.Sum256([]byte("inventory-admit-pin-" + strconv.Itoa(n)))
		encoded := hex.EncodeToString(pin[:])
		pins[id] = encoded
		members = append(members, InventoryMember{ID: id, Pin: encoded})
	}
	st, err := store.Open(filepath.Join(dir, "data"), bytes.Repeat([]byte{0x2a}, 32))
	if err != nil {
		t.Fatal(err)
	}
	f := &inventoryAdmitFixture{key: key, path: filepath.Join(dir, "inventory.jwt"), public: public, store: st, pins: pins, ids: ids}
	t.Cleanup(func() { st.Close() })
	if err := f.write(sequence, members, keys, expiry); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *inventoryAdmitFixture) write(sequence uint64, members []InventoryMember, keys []InventoryKey, expiry time.Time) error {
	now := time.Now().Unix()
	document := WalletInventory{
		SignedClaims: agent.SignedClaims{Version: 1, Issuer: "inventory-admit", Audience: agent.InventoryAudience, Tenant: "tenant-a", Sequence: strconv.FormatUint(sequence, 10), IssuedAt: now - 1, ExpiresAt: expiry.Unix()},
		Protocol:     "wallet", Threshold: 3, Members: members, Keys: keys,
	}
	body, err := json.Marshal(document)
	if err != nil {
		return err
	}
	header, err := json.Marshal(map[string]string{"alg": "ES256", "typ": agent.InventoryAudience + "+jwt"})
	if err != nil {
		return err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	digest := sha256.Sum256([]byte(unsigned))
	r, s, err := ecdsa.Sign(rand.Reader, f.key, digest[:])
	if err != nil {
		return err
	}
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	return os.WriteFile(f.path, []byte(unsigned+"."+base64.RawURLEncoding.EncodeToString(signature)), 0o600)
}

func (f *inventoryAdmitFixture) token(t *testing.T, sequence uint64, members []InventoryMember, keys []InventoryKey, expiry time.Time) string {
	t.Helper()
	old, err := os.ReadFile(f.path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.write(sequence, members, keys, expiry); err != nil {
		t.Fatal(err)
	}
	token, err := os.ReadFile(f.path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.path, old, 0o600); err != nil {
		t.Fatal(err)
	}
	return string(token)
}

func (f *inventoryAdmitFixture) inventory(t *testing.T) *Inventory {
	t.Helper()
	inventory, err := NewInventory(f.path, f.public, "inventory-admit", "tenant-a", f.store, f.pins)
	if err != nil {
		t.Fatal(err)
	}
	return inventory
}

func TestInventoryAdmitRaisesSequence(t *testing.T) {
	f := newInventoryAdmitFixture(t, 1, nil, time.Now().Add(time.Hour))
	inventory := f.inventory(t)
	newKey := InventoryKey{KeyID: "new-generation", Curve: "ed25519", Owner: "owner", Operations: []string{"generate"}}
	token := f.token(t, 2, inventory.current.Members, []InventoryKey{newKey}, time.Now().Add(time.Hour))
	sequence, held, err := inventory.Admit(token)
	if err != nil || held || sequence != "2" {
		t.Fatalf("admit = sequence %q held %v err %v", sequence, held, err)
	}
	if err := inventory.Check(newKey.KeyID, "generate", newKey.Owner, newKey.Curve, nil); err != nil {
		t.Fatalf("new inventory was not installed: %v", err)
	}
}

func TestInventoryAdmitNewGenerationWithoutRestart(t *testing.T) {
	f := newInventoryAdmitFixture(t, 1, nil, time.Now().Add(time.Hour))
	inventory := f.inventory(t)
	key := InventoryKey{KeyID: "only-new", Curve: "ed25519", Owner: "owner", Operations: []string{"generate"}}
	if err := inventory.Check(key.KeyID, "generate", key.Owner, key.Curve, nil); err == nil {
		t.Fatal("old inventory admitted a key that it does not contain")
	}
	token := f.token(t, 2, inventory.current.Members, []InventoryKey{key}, time.Now().Add(time.Hour))
	if _, _, err := inventory.Admit(token); err != nil {
		t.Fatal(err)
	}
	if err := inventory.Check(key.KeyID, "generate", key.Owner, key.Curve, nil); err != nil {
		t.Fatalf("live check did not use admitted inventory: %v", err)
	}
}

func TestInventoryAdmitLowerSequenceRefused(t *testing.T) {
	f := newInventoryAdmitFixture(t, 2, nil, time.Now().Add(time.Hour))
	inventory := f.inventory(t)
	token := f.token(t, 1, inventory.current.Members, nil, time.Now().Add(time.Hour))
	if _, _, err := inventory.Admit(token); err == nil {
		t.Fatal("lower inventory sequence was admitted")
	}
}

func TestInventoryAdmitSameTokenHeld(t *testing.T) {
	f := newInventoryAdmitFixture(t, 1, nil, time.Now().Add(time.Hour))
	inventory := f.inventory(t)
	token := f.token(t, 2, inventory.current.Members, nil, time.Now().Add(time.Hour))
	if _, _, err := inventory.Admit(token); err != nil {
		t.Fatal(err)
	}
	sequence, held, err := inventory.Admit(token)
	if err != nil || !held || sequence != "2" {
		t.Fatalf("repeat = sequence %q held %v err %v", sequence, held, err)
	}
	_, retained, err := agent.ReadPersisted(f.store, agent.InventoryAudience)
	if err != nil || retained != token {
		t.Fatalf("repeat changed retained token: %v", err)
	}
}

func TestInventoryAdmitAnotherSignerRefused(t *testing.T) {
	f := newInventoryAdmitFixture(t, 1, nil, time.Now().Add(time.Hour))
	inventory := f.inventory(t)
	other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	old := f.key
	f.key = other
	token := f.token(t, 2, inventory.current.Members, nil, time.Now().Add(time.Hour))
	f.key = old
	if _, _, err := inventory.Admit(token); err == nil {
		t.Fatal("token from another signer was admitted")
	}
}

func TestInventoryAdmitMemberPinMismatchRefused(t *testing.T) {
	f := newInventoryAdmitFixture(t, 1, nil, time.Now().Add(time.Hour))
	inventory := f.inventory(t)
	members := append([]InventoryMember(nil), inventory.current.Members...)
	members[0].Pin = strings.Repeat("0", 64)
	token := f.token(t, 2, members, nil, time.Now().Add(time.Hour))
	if _, _, err := inventory.Admit(token); err == nil {
		t.Fatal("inventory with a mismatched member pin was admitted")
	}
}

func TestInventoryAdmitClientWithoutAuthorityRefused(t *testing.T) {
	c := newTestCluster(t, 5, false)
	approveTestKeys(t, c, c.ids, InventoryKey{KeyID: "inventory-client", Curve: "ed25519", Owner: testOwner, Operations: []string{"generate"}})
	stranger := newTestCA(t, t.TempDir())
	req := verifiedRequest(stranger.cert, http.MethodPost, PathInventory, []byte(`{"token":"invalid"}`))
	recorder := httptest.NewRecorder()
	c.nodes[0].server.Handler().ServeHTTP(recorder, req)
	expectError(t, "client without inventory authority", apiResult{status: recorder.Code, body: recorder.Body.Bytes()}, CodeOperatorRequired)
}

func TestInventoryAdmitRestartUsesRetained(t *testing.T) {
	oldKey := InventoryKey{KeyID: "old", Curve: "ed25519", Owner: "owner", Operations: []string{"generate"}}
	f := newInventoryAdmitFixture(t, 1, []InventoryKey{oldKey}, time.Now().Add(time.Hour))
	inventory := f.inventory(t)
	newKey := InventoryKey{KeyID: "retained", Curve: "ed25519", Owner: "owner", Operations: []string{"generate"}}
	token := f.token(t, 2, inventory.current.Members, []InventoryKey{newKey}, time.Now().Add(time.Hour))
	if _, _, err := inventory.Admit(token); err != nil {
		t.Fatal(err)
	}
	if err := f.write(1, inventory.current.Members, []InventoryKey{oldKey}, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	restarted := f.inventory(t)
	if err := restarted.Check(newKey.KeyID, "generate", newKey.Owner, newKey.Curve, nil); err != nil {
		t.Fatalf("restart lost retained inventory: %v", err)
	}
}

func TestInventoryAdmitExpiredRetainedGivesWayToFile(t *testing.T) {
	oldKey := InventoryKey{KeyID: "file", Curve: "ed25519", Owner: "owner", Operations: []string{"generate"}}
	f := newInventoryAdmitFixture(t, 1, []InventoryKey{oldKey}, time.Now().Add(time.Hour))
	expired := time.Now().Add(-time.Hour)
	expiredToken := f.token(t, 2, f.inventory(t).current.Members, nil, expired)
	if err := agent.PersistSigned(f.store, agent.InventoryAudience, "2", expiredToken); err != nil {
		t.Fatal(err)
	}
	if err := f.write(1, f.inventory(t).current.Members, []InventoryKey{oldKey}, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	inventory := f.inventory(t)
	if err := inventory.Check(oldKey.KeyID, "generate", oldKey.Owner, oldKey.Curve, nil); err != nil {
		t.Fatalf("valid file did not replace expired retained inventory: %v", err)
	}
}
