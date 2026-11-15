package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/auth/agent"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/store"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

type inventoryFixture struct {
	dir, private, public, inventory string
	common, members                 []string
	pins                            map[string]string
	key                             *ecdsa.PrivateKey
}

func writeFile(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func newFixture(t *testing.T) *inventoryFixture {
	t.Helper()
	dir := t.TempDir()
	f := &inventoryFixture{dir: dir, private: filepath.Join(dir, "owner.pem"), public: filepath.Join(dir, "owner-public.pem"), inventory: filepath.Join(dir, "inventory.jwt"), pins: map[string]string{}}
	var err error
	f.key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	private, err := x509.MarshalPKCS8PrivateKey(f.key)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, f.private, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}))
	public, err := x509.MarshalPKIXPublicKey(&f.key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, f.public, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: public}))
	f.common = []string{"-private-key", f.private, "-issuer", "wallet-inventory-owner", "-tenant", "wallet-tenant", "-lifetime", "1h"}
	for i := 1; i <= 5; i++ {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		id := fmt.Sprintf("member-%d", i)
		template := &x509.Certificate{SerialNumber: big.NewInt(int64(i)), Subject: pkix.Name{CommonName: id}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
		der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, id+".pem")
		writeFile(t, path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
		spki, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
		if err != nil {
			t.Fatal(err)
		}
		pin := sha256.Sum256(spki)
		f.pins[id] = hex.EncodeToString(pin[:])
		f.members = append(f.members, "-member", id+"="+path)
	}
	return f
}

func (f *inventoryFixture) args(command string, extra ...string) []string {
	args := append([]string{command}, f.common...)
	return append(args, extra...)
}

func (f *inventoryFixture) init(t *testing.T) {
	t.Helper()
	args := append(f.args("init", "-output", f.inventory), f.members...)
	if err := run(args, io.Discard); err != nil {
		t.Fatal(err)
	}
}

func (f *inventoryFixture) decode(t *testing.T) server.WalletInventory {
	t.Helper()
	v, err := agent.NewSignedVerifier(f.public, "wallet-inventory-owner", "wallet-tenant")
	if err != nil {
		t.Fatal(err)
	}
	var document server.WalletInventory
	if err := v.Decode(strings.TrimSpace(string(readFile(t, f.inventory))), agent.InventoryAudience, maxLifetime, &document); err != nil {
		t.Fatal(err)
	}
	return document
}

func (f *inventoryFixture) requests(t *testing.T, keys []server.InventoryKey) string {
	t.Helper()
	path := filepath.Join(f.dir, "requests.json")
	raw, err := json.Marshal(struct {
		Version   int                   `json:"version"`
		RunID     string                `json:"run_id"`
		Owner     string                `json:"owner"`
		ChainID   uint64                `json:"chain_id"`
		Endpoints []string              `json:"endpoints"`
		Keys      []string              `json:"keys"`
		Inventory []server.InventoryKey `json:"inventory_requests"`
	}{1, "wallet-run", "wallet-owner", 125, []string{}, []string{}, keys})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, raw)
	return path
}

func (f *inventoryFixture) approve(t *testing.T, keys ...server.InventoryKey) {
	t.Helper()
	path := f.requests(t, keys)
	if err := run(f.args("approve", "-inventory", f.inventory, "-requests", path, "-output", f.inventory), io.Discard); err != nil {
		t.Fatal(err)
	}
}

func openStore(t *testing.T, path string, key []byte) *store.Store {
	t.Helper()
	s, err := store.Open(path, key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestInventoryLifecycle(t *testing.T) {
	f := newFixture(t)
	f.init(t)
	initial := f.decode(t)
	if initial.Sequence != "1" || initial.Version != 1 || initial.Protocol != "wallet" || initial.Threshold != 3 || len(initial.Keys) != 0 || len(initial.Members) != 5 {
		t.Fatalf("unexpected initial inventory: %+v", initial)
	}
	for _, member := range initial.Members {
		if member.Pin != f.pins[member.ID] {
			t.Fatalf("SPKI pin differs for %s", member.ID)
		}
	}
	parts := strings.Split(strings.TrimSpace(string(readFile(t, f.inventory))), ".")
	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(header) != `{"alg":"ES256","typ":"wallet-custody-inventory+jwt"}` {
		t.Fatalf("unexpected header: %s", header)
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(signature) != 64 {
		t.Fatal("ES256 signature must have fixed width")
	}
	storeKey := make([]byte, store.KeySize)
	if _, err := rand.Read(storeKey); err != nil {
		t.Fatal(err)
	}
	storePath := filepath.Join(f.dir, "store")
	s := openStore(t, storePath, storeKey)
	t.Cleanup(func() {
		if s != nil {
			s.Close()
		}
	})
	consumer, err := server.NewInventory(f.inventory, f.public, "wallet-inventory-owner", "wallet-tenant", s, f.pins)
	if err != nil {
		t.Fatal(err)
	}
	keys := []server.InventoryKey{
		{KeyID: "wallet-evm", Curve: "secp256k1", Owner: "wallet-owner", Operations: []string{"generate"}},
		{KeyID: "wallet-ed25519", Curve: "ed25519", Owner: "wallet-owner", Operations: []string{"generate"}},
	}
	f.approve(t, keys...)
	if got := f.decode(t); got.Sequence != "2" {
		t.Fatalf("sequence = %s", got.Sequence)
	}
	for _, key := range keys {
		if err := consumer.Check(key.KeyID, "generate", key.Owner, key.Curve, nil); err != nil {
			t.Fatal(err)
		}
		if err := consumer.Check(key.KeyID, "sign", key.Owner, key.Curve, nil); err == nil {
			t.Fatal("generation admission allowed signing")
		}
	}
	evmKey, err := gethcrypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	edPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicKeys := [][]byte{gethcrypto.FromECDSAPub(&evmKey.PublicKey), edPublic}
	for i := range keys {
		keys[i].PublicKey = hex.EncodeToString(publicKeys[i])
		keys[i].Epoch = 1
		keys[i].Operations = []string{"sign"}
	}
	f.approve(t, keys...)
	if got := f.decode(t); got.Sequence != "3" {
		t.Fatalf("sequence = %s", got.Sequence)
	}
	memberIDs := []string{"member-1", "member-2", "member-3", "member-4", "member-5"}
	for i, key := range keys {
		record := &store.ShareRecord{KeyID: key.KeyID, Curve: key.Curve, PublicKey: publicKeys[i], Epoch: key.Epoch, Participants: memberIDs}
		if err := consumer.Check(key.KeyID, "sign", key.Owner, key.Curve, record); err != nil {
			t.Fatal(err)
		}
		if err := consumer.Check(key.KeyID, "generate", key.Owner, key.Curve, nil); err == nil {
			t.Fatal("signing admission allowed generation")
		}
		record.Epoch++
		if err := consumer.Check(key.KeyID, "sign", key.Owner, key.Curve, record); err == nil {
			t.Fatal("different share epoch admitted")
		}
	}
	var shown bytes.Buffer
	if err := run(f.args("show", "-inventory", f.inventory), &shown); err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"sequence: 3\n", "issued: ", "expires: ", "members: 5\n", "keys: 2\n", "generate: 0\n", "sign: 2\n", "import: 0\n", "refresh: 0\n"} {
		if !strings.Contains(shown.String(), line) {
			t.Fatalf("summary missing %q: %s", line, shown.String())
		}
	}
	if strings.Contains(shown.String(), "wallet-evm") {
		t.Fatal("summary printed key detail without -keys")
	}
	shown.Reset()
	if err := run(f.args("show", "-inventory", f.inventory, "-keys"), &shown); err != nil {
		t.Fatal(err)
	}
	if strings.Count(shown.String(), "\nkey ") != 2 {
		t.Fatalf("expected two key lines: %s", shown.String())
	}
	if strings.Contains(shown.String(), "PRIVATE KEY") {
		t.Fatal("private key appeared in output")
	}
	prior := readFile(t, f.inventory)
	before := f.decode(t)
	if err := run(f.args("renew", "-inventory", f.inventory, "-output", f.inventory, "-lifetime", "2h"), io.Discard); err != nil {
		t.Fatal(err)
	}
	after := f.decode(t)
	if after.Sequence != "4" || after.IssuedAt < before.IssuedAt || after.ExpiresAt <= before.ExpiresAt || after.ExpiresAt-after.IssuedAt != 7200 || !reflect.DeepEqual(after.Members, before.Members) || !reflect.DeepEqual(after.Keys, before.Keys) {
		t.Fatalf("renewal changed inventory content or invalid times: %+v", after)
	}
	if _, err := server.NewInventory(f.inventory, f.public, "wallet-inventory-owner", "wallet-tenant", s, f.pins); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = nil
	s = openStore(t, storePath, storeKey)
	if _, err := server.NewInventory(f.inventory, f.public, "wallet-inventory-owner", "wallet-tenant", s, f.pins); err != nil {
		t.Fatalf("renewed token rejected after restart: %v", err)
	}
	writeFile(t, f.inventory, prior)
	if _, err := server.NewInventory(f.inventory, f.public, "wallet-inventory-owner", "wallet-tenant", s, f.pins); err == nil {
		t.Fatal("earlier token admitted after renewal and restart")
	}
}

func TestApproveRefusesIncompatibleKeysAtomically(t *testing.T) {
	f := newFixture(t)
	f.init(t)
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key := server.InventoryKey{KeyID: "approved-key", Curve: "ed25519", Owner: "wallet-owner", PublicKey: hex.EncodeToString(public), Epoch: 2, Operations: []string{"sign"}}
	f.approve(t, key)
	before := readFile(t, f.inventory)
	cases := []struct {
		name   string
		change func(*server.InventoryKey)
	}{
		{"changed owner", func(k *server.InventoryKey) { k.Owner = "different-owner" }},
		{"changed public key", func(k *server.InventoryKey) {
			k.PublicKey = strings.Repeat("ab", 32)
			if k.PublicKey == key.PublicKey {
				k.PublicKey = strings.Repeat("cd", 32)
			}
		}},
		{"lower epoch", func(k *server.InventoryKey) { k.Epoch-- }},
		{"changed curve", func(k *server.InventoryKey) { k.Curve = "secp256k1" }},
		{"generation reversal", func(k *server.InventoryKey) { k.PublicKey = ""; k.Epoch = 0; k.Operations = []string{"generate"} }},
		{"unsafe epoch", func(k *server.InventoryKey) { k.Epoch = maxSafeInteger + 1 }},
		{"duplicate operations", func(k *server.InventoryKey) { k.Operations = []string{"sign", "sign"} }},
		{"unknown operation", func(k *server.InventoryKey) { k.Operations = []string{"delete"} }},
		{"uppercase public key", func(k *server.InventoryKey) { k.PublicKey = strings.ToUpper(k.PublicKey) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := key
			tc.change(&request)
			newKey := server.InventoryKey{KeyID: "new-key", Owner: key.Owner, Curve: "ed25519", Operations: []string{"generate"}}
			path := f.requests(t, []server.InventoryKey{newKey, request})
			err := run(f.args("approve", "-inventory", f.inventory, "-requests", path, "-output", f.inventory), io.Discard)
			if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), key.KeyID) {
				t.Fatalf("expected refusal naming file and key, got %v", err)
			}
			if strings.Contains(err.Error(), request.PublicKey) && request.PublicKey != "" {
				t.Fatal("refusal included public key contents")
			}
			if !bytes.Equal(before, readFile(t, f.inventory)) {
				t.Fatal("refused document changed output")
			}
		})
	}
	key.Epoch++
	key.Operations = []string{"sign", "refresh"}
	f.approve(t, key)
	got := f.decode(t)
	if got.Sequence != "3" || !reflect.DeepEqual(got.Keys, []server.InventoryKey{key}) {
		t.Fatalf("compatible operation and epoch change refused: %+v", got)
	}
}

func TestGenerationApprovalPreservesIdentity(t *testing.T) {
	f := newFixture(t)
	f.init(t)
	key := server.InventoryKey{KeyID: "generation-key", Curve: "ed25519", Owner: "wallet-owner", Operations: []string{"generate"}}
	f.approve(t, key)
	f.approve(t, key)
	for _, change := range []func(*server.InventoryKey){func(k *server.InventoryKey) { k.Owner = "other-owner" }, func(k *server.InventoryKey) { k.Curve = "secp256k1" }} {
		request := key
		change(&request)
		path := f.requests(t, []server.InventoryKey{request})
		if err := run(f.args("approve", "-inventory", f.inventory, "-requests", path, "-output", f.inventory), io.Discard); err == nil || !strings.Contains(err.Error(), key.KeyID) {
			t.Fatalf("expected identity refusal: %v", err)
		}
	}
}

func TestInitRefusesInvalidMembersAndLifetime(t *testing.T) {
	f := newFixture(t)
	cases := []struct {
		name           string
		members, extra []string
	}{
		{"sixth member", append(append([]string{}, f.members...), "-member", "sixth="+filepath.Join(f.dir, "member-1.pem")), nil},
		{"four members", f.members[:8], nil},
		{"duplicate member", append(append([]string{}, f.members[:8]...), f.members[:2]...), nil},
		{"duplicate pin", append(append([]string{}, f.members[:8]...), "-member", "member-5="+filepath.Join(f.dir, "member-1.pem")), nil},
		{"long lifetime", f.members, []string{"-lifetime", "720h1s"}},
		{"zero lifetime", f.members, []string{"-lifetime", "0s"}},
		{"negative lifetime", f.members, []string{"-lifetime", "-1h"}},
		{"fractional second", f.members, []string{"-lifetime", "1.5s"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append(f.args("init", "-output", f.inventory), tc.members...)
			args = append(args, tc.extra...)
			if err := run(args, io.Discard); err == nil {
				t.Fatal("invalid init input accepted")
			}
			if _, err := os.Stat(f.inventory); !os.IsNotExist(err) {
				t.Fatal("refused init wrote output")
			}
		})
	}
	args := append(f.args("init", "-output", f.inventory, "-lifetime", "720h"), f.members...)
	if err := run(args, io.Discard); err != nil {
		t.Fatal(err)
	}
	got := f.decode(t)
	if got.ExpiresAt-got.IssuedAt != 2592000 {
		t.Fatal("maximum lifetime differs")
	}
}

func TestPrivateKeyFileRequirements(t *testing.T) {
	f := newFixture(t)
	for _, mode := range []os.FileMode{0644, 0640, 0400, 0700} {
		t.Run(fmt.Sprintf("mode %o", mode), func(t *testing.T) {
			if err := os.Chmod(f.private, mode); err != nil {
				t.Fatal(err)
			}
			args := append(f.args("init", "-output", f.inventory), f.members...)
			if err := run(args, io.Discard); err == nil || !strings.Contains(err.Error(), f.private) {
				t.Fatalf("expected private file refusal, got %v", err)
			}
		})
	}
	if err := os.Chmod(f.private, 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(f.dir, "owner-link.pem")
	if err := os.Symlink(f.private, link); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPrivateKey(link); err == nil {
		t.Fatal("symlink accepted as private key")
	}
	if _, err := loadPrivateKey(f.dir); err == nil {
		t.Fatal("directory accepted as private key")
	}
	if os.Geteuid() == 0 {
		if err := os.Chown(f.private, 65534, -1); err != nil {
			t.Fatal(err)
		}
		if _, err := loadPrivateKey(f.private); err == nil {
			t.Fatal("key owned by another user accepted")
		}
		if err := os.Chown(f.private, os.Geteuid(), -1); err != nil {
			t.Fatal(err)
		}
	}
	p384, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalECPrivateKey(p384)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, f.private, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}))
	if _, err := loadPrivateKey(f.private); err == nil {
		t.Fatal("P-384 owner key accepted")
	}
	der, err = x509.MarshalECPrivateKey(f.key)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, f.private, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}))
	if _, err := loadPrivateKey(f.private); err != nil {
		t.Fatal(err)
	}
	writeFile(t, f.private, []byte("private-material-marker"))
	if _, err := loadPrivateKey(f.private); err == nil || strings.Contains(err.Error(), "private-material-marker") || !strings.Contains(err.Error(), f.private) {
		t.Fatalf("unsafe key refusal: %v", err)
	}
}

func TestApproveRefusesMalformedRequests(t *testing.T) {
	f := newFixture(t)
	f.init(t)
	before := readFile(t, f.inventory)
	path := filepath.Join(f.dir, "requests.json")
	for _, raw := range []string{
		`{}`, `{"inventory_requests":null}`, `{"inventory_requests":{}}`, `{"inventory_requests":[]} {}`,
		`{"inventory_requests":[{"key_id":"bad-key","curve":"ed25519","owner":"wallet-owner","operations":["generate"]}]}`,
		`{"inventory_requests":[{"key_id":"bad-key","epoch":null,"curve":"ed25519","public_key":"","owner":"wallet-owner","operations":["generate"]}]}`,
		`{"inventory_requests":[{"key_id":"bad-key","epoch":-1,"curve":"ed25519","public_key":"","owner":"wallet-owner","operations":["generate"]}]}`,
		`{"inventory_requests":[{"key_id":"bad-key","epoch":0,"curve":"ed25519","public_key":"","owner":"wallet-owner","operations":["generate"],"extra":"request-content-marker"}]}`,
	} {
		writeFile(t, path, []byte(raw))
		err := run(f.args("approve", "-inventory", f.inventory, "-requests", path, "-output", f.inventory), io.Discard)
		if err == nil || !strings.Contains(err.Error(), path) || strings.Contains(err.Error(), "request-content-marker") {
			t.Fatalf("unsafe request refusal: %v", err)
		}
		if strings.Contains(raw, "bad-key") && !strings.Contains(err.Error(), "bad-key") {
			t.Fatalf("key id missing: %v", err)
		}
		if !bytes.Equal(before, readFile(t, f.inventory)) {
			t.Fatal("malformed requests changed inventory")
		}
	}
	key := server.InventoryKey{KeyID: "duplicate-key", Curve: "ed25519", Owner: "wallet-owner", Operations: []string{"generate"}}
	f.requests(t, []server.InventoryKey{key, key})
	if err := run(f.args("approve", "-inventory", f.inventory, "-requests", path, "-output", f.inventory), io.Discard); err == nil || !strings.Contains(err.Error(), key.KeyID) {
		t.Fatalf("duplicate request accepted: %v", err)
	}
}

func TestInventoryAuthenticityAndAtomicOutput(t *testing.T) {
	f := newFixture(t)
	f.init(t)
	before := readFile(t, f.inventory)
	for _, extra := range [][]string{{"-issuer", "other-issuer"}, {"-tenant", "other-tenant"}} {
		args := append(f.args("renew", "-inventory", f.inventory, "-output", f.inventory), extra...)
		if err := run(args, io.Discard); err == nil || !strings.Contains(err.Error(), f.inventory) {
			t.Fatalf("scope mismatch accepted: %v", err)
		}
		if !bytes.Equal(before, readFile(t, f.inventory)) {
			t.Fatal("invalid scope changed inventory")
		}
	}
	corrupt := append([]byte{}, before...)
	corrupt[0] = '!'
	writeFile(t, f.inventory, corrupt)
	if err := run(f.args("show", "-inventory", f.inventory), io.Discard); err == nil {
		t.Fatal("invalid token accepted")
	}
	writeFile(t, f.inventory, before)
	privateBefore := readFile(t, f.private)
	if err := run(f.args("renew", "-inventory", f.inventory, "-output", f.private), io.Discard); err == nil {
		t.Fatal("private key overwrite permitted")
	}
	if !bytes.Equal(privateBefore, readFile(t, f.private)) {
		t.Fatal("private key changed")
	}
	output := filepath.Join(f.dir, "published.jwt")
	writeFile(t, output, []byte("previous inventory"))
	if err := run(f.args("renew", "-inventory", f.inventory, "-output", output), io.Discard); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(output)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("output must be mode 0600")
	}
	if len(readFile(t, output)) == 0 || !bytes.Equal(before, readFile(t, f.inventory)) {
		t.Fatal("separate output changed input")
	}
	entries, err := os.ReadDir(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".inventory-") {
			t.Fatal("temporary inventory file retained")
		}
	}
	if err := writeInventory(f.dir, []byte("signed inventory")); err == nil {
		t.Fatal("directory accepted as output")
	}
	entries, err = os.ReadDir(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".inventory-") {
			t.Fatal("temporary inventory file retained after refused write")
		}
	}
}
