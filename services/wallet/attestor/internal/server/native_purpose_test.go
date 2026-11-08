package server

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/auth/agent"
	nativepolicy "github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy/native"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/tss/dealershare"
	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

func nativeText(out *bytes.Buffer, value string, width int) {
	if width == 4 {
		_ = binary.Write(out, binary.BigEndian, uint32(len(value)))
	} else {
		_ = binary.Write(out, binary.BigEndian, uint16(len(value)))
	}
	out.WriteString(value)
}

func nativePurposeBytes(expiry uint64) []byte {
	var out bytes.Buffer
	out.WriteString(nativepolicy.PurposeDomain)
	out.WriteByte(1)
	nativeText(&out, "tenant-a", 4)
	nativeText(&out, "did:layerx:alice", 4)
	out.Write(bytes.Repeat([]byte{0x11}, 32))
	_ = binary.Write(&out, binary.BigEndian, uint64(1))
	_ = binary.Write(&out, binary.BigEndian, expiry)
	for _, value := range []byte{0x22, 3, 4, 5} {
		out.Write(bytes.Repeat([]byte{value}, 32))
	}
	return out.Bytes()
}

func nativeGrantRecords(owner []byte, expiry uint64) ([]byte, []byte) {
	return nativeGrantRecordsForAgent(owner, expiry, "did:layerx:alice")
}

func nativeGrantRecordsForAgent(owner []byte, expiry uint64, agentDID string) ([]byte, []byte) {
	var record bytes.Buffer
	record.WriteByte(1)
	record.Write(bytes.Repeat([]byte{0x22}, 32))
	record.WriteByte(0)
	nativeText(&record, "tenant-a", 2)
	nativeText(&record, agentDID, 2)
	record.WriteByte(1)
	record.Write(owner)
	for i := 0; i < 6; i++ {
		_ = binary.Write(&record, binary.BigEndian, uint16(0))
	}
	for _, value := range []uint64{expiry / 1000, expiry, expiry - 600000, 1} {
		_ = binary.Write(&record, binary.BigEndian, value)
	}
	record.WriteByte(0)
	var capability bytes.Buffer
	capability.WriteString("LXNC")
	capability.WriteByte(1)
	_ = binary.Write(&capability, binary.BigEndian, uint32(record.Len()))
	capability.Write(record.Bytes())
	_ = binary.Write(&capability, binary.BigEndian, uint16(1))
	capability.Write([]byte{1, 0, 9, 1, 2})
	_ = binary.Write(&capability, binary.BigEndian, uint16(0))
	_ = binary.Write(&capability, binary.BigEndian, uint16(0))
	var session bytes.Buffer
	session.WriteString("LXNS01")
	nativeText(&session, "tenant-a", 2)
	nativeText(&session, agentDID, 2)
	session.Write(bytes.Repeat([]byte{0x11}, 32))
	_ = binary.Write(&session, binary.BigEndian, uint64(1))
	_ = binary.Write(&session, binary.BigEndian, uint16(1))
	session.Write([]byte{1, 0, 9, 1, 2})
	return capability.Bytes(), session.Bytes()
}

type nativeInventoryAuthority struct {
	key          *ecdsa.PrivateKey
	path, public string
	cluster      *testCluster
	sequence     uint64
	keys         []InventoryKey
	members      []string
}

func (a *nativeInventoryAuthority) publish(t *testing.T) {
	t.Helper()
	a.sequence++
	now := time.Now().Unix()
	members := make([]InventoryMember, 0, 5)
	for _, peer := range a.cluster.peers {
		if a.members == nil || slices.Contains(a.members, peer.ID) {
			members = append(members, InventoryMember{ID: peer.ID, Pin: peer.SPKISHA256})
		}
	}
	document := WalletInventory{SignedClaims: agent.SignedClaims{Version: 1, Issuer: "native-corpus", Audience: agent.InventoryAudience, Tenant: "tenant-a", Sequence: strconv.FormatUint(a.sequence, 10), IssuedAt: now - 1, ExpiresAt: now + 3600}, Protocol: "wallet", Threshold: 3, Members: members, Keys: a.keys}
	header, _ := json.Marshal(map[string]string{"alg": "ES256", "typ": agent.InventoryAudience + "+jwt"})
	body, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	input := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	digest := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, a.key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	if err = os.WriteFile(a.path, []byte(input+"."+base64.RawURLEncoding.EncodeToString(signature)), 0o600); err != nil {
		t.Fatal(err)
	}
	pins := map[string]string{}
	for _, member := range members {
		pins[member.ID] = member.Pin
	}
	for _, node := range a.cluster.nodes {
		if node.stopped.Load() {
			continue
		}
		inventory, err := NewInventory(a.path, a.public, "native-corpus", "tenant-a", node.store, pins)
		if err != nil {
			t.Fatal(err)
		}
		node.server.opts.Inventory = inventory
		if a.members != nil {
			node.server.opts.Participants = append([]string(nil), a.members...)
		}
	}
}

func nativeInventory(t *testing.T, c *testCluster, keyID string, pub []byte) *nativeInventoryAuthority {
	t.Helper()
	if c.importInventory != nil {
		c.importMu.Lock()
		defer c.importMu.Unlock()
		key := InventoryKey{KeyID: keyID, Epoch: 0, Curve: "ed25519", PublicKey: hex.EncodeToString(pub), Owner: testOwner, Operations: []string{"sign"}}
		replaced := false
		for i := range c.importInventory.keys {
			if c.importInventory.keys[i].KeyID == keyID {
				c.importInventory.keys[i] = key
				replaced = true
				break
			}
		}
		if !replaced {
			c.importInventory.keys = append(c.importInventory.keys, key)
		}
		c.importInventory.publish(t)
		return c.importInventory
	}
	dir := t.TempDir()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	public, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	a := &nativeInventoryAuthority{key: key, path: filepath.Join(dir, "inventory.jwt"), public: filepath.Join(dir, "authority.pem"), cluster: c,
		keys: []InventoryKey{{KeyID: keyID, Epoch: 0, Curve: "ed25519", PublicKey: hex.EncodeToString(pub), Owner: testOwner, Operations: []string{"sign"}}}}
	writeTestPEM(t, a.public, "PUBLIC KEY", public)
	a.publish(t)
	previous := c.tune
	c.tune = func(o *Options) {
		if previous != nil {
			previous(o)
		}
		pins := map[string]string{}
		for _, p := range c.peers {
			pins[p.ID] = p.SPKISHA256
		}
		i, err := NewInventory(a.path, a.public, "native-corpus", "tenant-a", o.Store, pins)
		if err != nil {
			t.Fatal(err)
		}
		o.Inventory = i
	}
	return a
}

func verifyNativeResponses(t *testing.T, pub []byte, digest [32]byte, operation string, responses []NativeSignResponse) {
	t.Helper()
	if len(responses) != 3 {
		t.Fatal("actual threshold response count")
	}
	for _, row := range responses {
		signature, err := hex.DecodeString(row.Signature)
		if err != nil || row.Profile != 2 || row.Operation != operation || row.SignedBytes != hex.EncodeToString(digest[:]) || !ed25519.Verify(pub, digest[:], signature) || row.AuditSequence == 0 {
			t.Fatalf("native consent verification failed for %s", row.NodeID)
		}
		if row.Signature != responses[0].Signature {
			t.Fatal("actual quorum signature agreement")
		}
	}
}

func verifyNativeInventoryRequired(t *testing.T, c *testCluster, request NativeSignRequest) {
	t.Helper()
	node := c.byID(request.Signers[0])[0]
	inventory := node.server.opts.Inventory
	if inventory == nil {
		t.Fatal("real signed owner inventory must be installed before native signing")
	}
	node.server.opts.Inventory = nil
	defer func() { node.server.opts.Inventory = inventory }()
	request.SessionID += "-missing-inventory"
	result := c.call(t, node, PathNativeSign, request, c.idp.mint(t, c.idp.key, testOwner))
	refusal := expectError(t, "native consent without owner inventory", result, CodeTokenUnavailable)
	if result.status != http.StatusUnauthorized || refusal.Message != "owner-approved key inventory unavailable for native consent" {
		t.Fatalf("unexpected native inventory refusal: status %d error %+v", result.status, refusal)
	}
	t.Logf("native inventory refusal: %s", refusal.Code)
}

func nativeRustCommand(t *testing.T, binary, test string) *exec.Cmd {
	t.Helper()
	supervisor := filepath.Join("..", "..", "..", "..", "..", "tools", "runtime", "run-with-clock.sh")
	command := exec.Command("sh", supervisor, binary, "--exact", test, "--nocapture", "--test-threads=1")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if evidence := os.Getenv("HF_OUT_DIR"); evidence != "" {
		path := filepath.Join(evidence, "logs", test+".log")
		log, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = log.Close() })
		command.Stdout = io.MultiWriter(os.Stdout, log)
		command.Stderr = io.MultiWriter(os.Stderr, log)
		t.Logf("private Rust consumer evidence: %s", path)
	}
	return command
}

func TestNativeCustodyRealProfiles(t *testing.T) {
	seed := sha256.Sum256([]byte("native custody disposable owner"))
	pub := ed25519.NewKeyFromSeed(seed[:]).Public().(ed25519.PublicKey)
	document := &nativepolicy.Document{Version: 2, Rules: []nativepolicy.Rule{{KeyID: "native-owner", Owner: testOwner, Tenant: "tenant-a", AgentDID: "did:layerx:alice", OwnerPublicKey: hex.EncodeToString(pub), Operations: []string{NativePreparationPurpose, NativeLocalGrantConsent}, Capabilities: []string{strings.Repeat("22", 32)}, MaximumValidityMS: 700000, RatePerMinute: 1000}}}
	c := newTestClusterWith(t, 5, true, testPolicy(), func(o *Options) { o.NativePolicy = document })
	imported := importKernelKey(t, c, "native-owner", "native custody disposable owner", "0x9999999999999999999999999999999999999999")
	if !bytes.Equal(imported, pub) {
		t.Fatal("real imported owner key")
	}
	authority := nativeInventory(t, c, "native-owner", pub)
	signers := []string{"node-1", "node-2", "node-3"}
	expiry := uint64(time.Now().Add(10 * time.Minute).UnixMilli())
	purpose := nativePurposeBytes(expiry)
	digest, _, err := nativepolicy.PurposeDigest(purpose)
	if err != nil {
		t.Fatal(err)
	}
	request := NativeSignRequest{Profile: 2, SessionID: "native-purpose", KeyID: "native-owner", Operation: NativePreparationPurpose, Signers: signers, Purpose: hex.EncodeToString(purpose)}
	verifyNativeInventoryRequired(t, c, request)
	sign := func(req NativeSignRequest, subject string) []apiResult {
		return c.callAll(t, c.byID(req.Signers...), PathNativeSign, func(*testNode) any { return req }, c.idp.mint(t, c.idp.key, subject))
	}
	first := decodeOK[NativeSignResponse](t, "native purpose", sign(request, testOwner))
	verifyNativeResponses(t, pub, digest, request.Operation, first)
	replay := decodeOK[NativeSignResponse](t, "exact native purpose replay", sign(request, testOwner))
	for i := range replay {
		if replay[i] != first[i] {
			t.Fatal("exact durable native replay")
		}
	}
	restarted := c.restart(t, c.byID("node-2")[0])
	_ = restarted
	recovered := decodeOK[NativeSignResponse](t, "native restart replay", sign(request, testOwner))
	for i := range recovered {
		if recovered[i] != first[i] {
			t.Fatal("native replay changed after encrypted-store restart")
		}
	}
	altered := request
	changed := append([]byte(nil), purpose...)
	changed[len(changed)-1] ^= 1
	altered.Purpose = hex.EncodeToString(changed)
	for _, result := range sign(altered, testOwner) {
		expectError(t, "same session changed purpose", result, CodeSessionBadRequest)
	}
	foreign := request
	foreign.SessionID = "native-foreign-owner"
	for _, result := range sign(foreign, "user-0002") {
		expectError(t, "foreign original owner", result, CodeTokenNotOwner)
	}
	malformed := request
	malformed.SessionID = "native-malformed"
	malformed.Purpose = hex.EncodeToString(append(purpose, 0))
	for _, result := range sign(malformed, testOwner) {
		expectError(t, "canonical trailing bytes", result, CodeSessionBadRequest)
	}
	expired := request
	expired.SessionID = "native-expired"
	expired.Purpose = hex.EncodeToString(nativePurposeBytes(uint64(time.Now().Add(-time.Second).UnixMilli())))
	expectPolicyCode(t, "expired owner consent", sign(expired, testOwner), "native_consent_expired")
	tenant := request
	tenant.SessionID = "native-foreign-tenant"
	raw := append([]byte(nil), purpose...)
	raw[len(nativepolicy.PurposeDomain)+5] ^= 1
	tenant.Purpose = hex.EncodeToString(raw)
	expectPolicyCode(t, "foreign tenant coordinates", sign(tenant, testOwner), "destination_denied")
	few := request
	few.SessionID = "native-short-quorum"
	few.Signers = signers[:2]
	for _, result := range sign(few, testOwner) {
		expectError(t, "insufficient quorum", result, CodeQuorumTooFew)
	}
	capability, session := nativeGrantRecords(pub, expiry)
	grantDigest, _, err := nativepolicy.GrantDigest(capability, session, expiry, pub)
	if err != nil {
		t.Fatal(err)
	}
	grant := NativeSignRequest{Profile: 2, SessionID: "native-grant", KeyID: "native-owner", Operation: NativeLocalGrantConsent, Signers: signers, Capability: hex.EncodeToString(capability), NativeSession: hex.EncodeToString(session), ExpiryMS: strconv.FormatUint(expiry, 10)}
	verifyNativeResponses(t, pub, grantDigest, grant.Operation, decodeOK[NativeSignResponse](t, "genuine local grant consent", sign(grant, testOwner)))
	badGrant := grant
	badGrant.SessionID = "native-malformed-full-ordinal"
	bad := append([]byte(nil), session...)
	bad[len(bad)-5] = 0
	badGrant.NativeSession = hex.EncodeToString(bad)
	for _, result := range sign(badGrant, testOwner) {
		expectError(t, "closed native activity version", result, CodeSessionBadRequest)
	}
	unknown := c.call(t, c.nodes[0], PathSign, SignRequest{SessionID: "v1-native-kind", KeyID: "native-owner", Kind: NativePreparationPurpose, Signers: signers}, c.idp.mint(t, c.idp.key, testOwner))
	expectError(t, "v1 unknown native kind unchanged", unknown, CodeSessionKind)
	runNativeRustConsumer(t, c, authority, document)
	for _, node := range c.nodes {
		if err := node.audit.Verify(); err != nil {
			t.Fatal(err)
		}
	}
}

func nativeWaitFile(t *testing.T, path string, done <-chan error) []byte {
	t.Helper()
	until := time.Now().Add(2 * time.Minute)
	for time.Now().Before(until) {
		raw, err := os.ReadFile(path)
		if err == nil {
			return raw
		}
		select {
		case err := <-done:
			t.Fatalf("real Rust consumer exited before handshake: %v", err)
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("bounded genuine provider handshake timed out")
	return nil
}

func runNativeRustConsumer(t *testing.T, c *testCluster, authority *nativeInventoryAuthority, document *nativepolicy.Document) {
	t.Helper()
	binary := os.Getenv("PAXEER_X_NATIVE_CUSTODY_RUST_TEST")
	if binary == "" {
		t.Fatal("prebuilt real Rust custody consumer is required")
	}
	dir := t.TempDir()
	pair := c.client.Transport.(*http.Transport).TLSClientConfig.Certificates[0]
	private, err := x509.MarshalPKCS8PrivateKey(pair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	nodes := make([]map[string]string, 0, 5)
	for _, node := range c.nodes {
		nodes = append(nodes, map[string]string{"id": node.id, "address": node.apiAddr})
	}
	fixture := map[string]any{"schema": "layerx-human-native-custody-live.v1", "directory": dir, "nodes": nodes, "signers": []string{"node-1", "node-2", "node-3"},
		"root_der": c.ca.cert.Raw, "client_der": pair.Certificate[0], "client_pkcs8": private, "owner": testOwner, "account": "0x9999999999999999999999999999999999999999",
		"owner_assertion": c.idp.mint(t, c.idp.key, testOwner), "second_assertion": c.idp.mint(t, c.idp.key, testOwner), "foreign_assertion": c.idp.mint(t, c.idp.key, "user-0002")}
	path := filepath.Join(dir, "fixture.json")
	raw, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	command := nativeRustCommand(t, binary, "native_provider_and_sdk_real_cluster")
	command.Env = append(os.Environ(), "PAXEER_X_NATIVE_CUSTODY_LIVE_FIXTURE="+path)
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill() })
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	var provision struct {
		KeyID string `json:"key_id"`
	}
	if err = json.Unmarshal(nativeWaitFile(t, filepath.Join(dir, "provision.json"), done), &provision); err != nil || provision.KeyID == "" {
		t.Fatal("authentic Rust provider binding request")
	}
	authority.keys = append(authority.keys, InventoryKey{KeyID: provision.KeyID, Epoch: 0, Curve: "ed25519", Owner: testOwner, Operations: []string{"generate"}})
	authority.publish(t)
	if err = os.WriteFile(filepath.Join(dir, "provision-ready"), []byte("ready"), 0o600); err != nil {
		t.Fatal(err)
	}
	var generated struct {
		Public []byte `json:"public"`
	}
	if err = json.Unmarshal(nativeWaitFile(t, filepath.Join(dir, "generated.json"), done), &generated); err != nil || len(generated.Public) != 32 {
		t.Fatal("actual Rust production-provider generated key")
	}
	for _, node := range c.nodes {
		record, payload, e := node.server.loadShare(provision.KeyID)
		if e != nil || record.Epoch != 0 || payload.Owner != testOwner || !bytes.Equal(record.PublicKey, generated.Public) {
			t.Fatal("real stored quorum key lineage")
		}
	}
	authority.keys[len(authority.keys)-1] = InventoryKey{KeyID: provision.KeyID, Epoch: 0, Curve: "ed25519", PublicKey: hex.EncodeToString(generated.Public), Owner: testOwner, Operations: []string{"sign"}}
	authority.publish(t)
	document.Rules = append(document.Rules, nativepolicy.Rule{KeyID: provision.KeyID, Owner: testOwner, Tenant: "tenant-a", AgentDID: "did:layerx:alice", OwnerPublicKey: hex.EncodeToString(generated.Public), Operations: []string{NativePreparationPurpose, NativeLocalGrantConsent}, Capabilities: []string{strings.Repeat("22", 32)}, MaximumValidityMS: 700000, RatePerMinute: 1000})
	if err = document.Validate(); err != nil {
		t.Fatal(err)
	}
	expiry := uint64(time.Now().Add(10 * time.Minute).UnixMilli())
	capability, session := nativeGrantRecords(generated.Public, expiry)
	material := map[string]any{"purpose": nativePurposeBytes(expiry), "capability": capability, "native_session": session, "expiry_ms": expiry}
	raw, err = json.Marshal(material)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "signing.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("real Rust provider consumer failed: %v", err)
		}
	case <-time.After(5 * time.Minute):
		t.Fatal("actual Rust provider consumer deadline")
	}
	fmt.Println("native custody real provider handshake complete")
}

func nativeSendPurposeBytes(owner []byte, expiry uint64) []byte {
	return nativeSendPurposeBytesForSender(owner, expiry, common.HexToAddress("0x9999999999999999999999999999999999999999"))
}

func nativeSendPurposeBytesForSender(owner []byte, expiry uint64, sender common.Address) []byte {
	var out bytes.Buffer
	out.WriteString(nativeSendPurposeV2Domain)
	out.WriteByte(2)
	nativeText(&out, "tenant-a", 4)
	nativeText(&out, "did:layerx:"+strings.Repeat("aa", 32), 4)
	nativeText(&out, "did:layerx:"+strings.Repeat("cc", 32), 4)
	out.Write(owner)
	out.Write(sender[:])
	out.Write(bytes.Repeat([]byte{0x11}, 32))
	_ = binary.Write(&out, binary.BigEndian, uint64(1))
	_ = binary.Write(&out, binary.BigEndian, expiry)
	out.Write(bytes.Repeat([]byte{0x22}, 32))
	out.WriteByte(2)
	_ = binary.Write(&out, binary.BigEndian, uint64(125))
	out.Write([]byte{1, 0, 1, 0, 5})
	for _, value := range []byte{3, 4, 6, 7, 5} {
		out.Write(bytes.Repeat([]byte{value}, 32))
	}
	return out.Bytes()
}
func importNativeSendOwner(t *testing.T, c *testCluster, keyID, label, account string) (ed25519.PublicKey, *nativeInventoryAuthority) {
	t.Helper()
	seed := sha256.Sum256([]byte(label))
	pub := ed25519.NewKeyFromSeed(seed[:]).Public().(ed25519.PublicKey)
	scalar, err := dealer.Ed25519ScalarFromSeed(seed[:])
	if err != nil {
		t.Fatal(err)
	}
	defer dealer.Wipe(scalar)
	bundles, _, err := dealer.Split(dealer.Ed25519, scalar, c.ids)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundles) != dealer.Participants || len(c.nodes) != dealer.Participants {
		t.Fatal("actual five-member ceremony")
	}
	participants := bundles[0].ParticipantIDs()
	shares := make(map[string]ShareBundleJSON, len(bundles))
	for _, bundle := range bundles {
		if bundle.Threshold != dealer.Threshold || !sameIDs(bundle.ParticipantIDs(), participants) {
			t.Fatal("actual ceremony share membership and threshold")
		}
		shares[bundle.ParticipantID] = EncodeBundle(bundle)
	}
	authority := nativeInventory(t, c, keyID, pub)
	authority.keys[0].Operations = []string{"import"}
	authority.publish(t)
	results := c.callAll(t, c.nodes, PathImport, func(node *testNode) any {
		share, ok := shares[node.id]
		if !ok {
			t.Errorf("actual ceremony share absent for %s", node.id)
		}
		return ImportRequest{CeremonyID: "native-send-import-" + keyID, SessionID: "import-" + keyID,
			KeyID: keyID, Epoch: 0, PublicKey: hex.EncodeToString(pub), Participants: participants,
			Threshold: dealer.Threshold, Owner: testOwner, Account: account, Share: share}
	}, "")
	responses := decodeOK[KeyResponse](t, "native send ceremony import", results)
	if len(responses) != dealer.Participants {
		t.Fatal("all actual ceremony import responses")
	}
	for i, node := range c.nodes {
		response := responses[i]
		if response.NodeID != node.id || response.KeyID != keyID || response.Curve != "ed25519" || response.PublicKey != hex.EncodeToString(pub) || response.Epoch != 0 || !sameIDs(response.Participants, participants) || response.AuditSequence == 0 {
			t.Fatal("real imported ceremony response identity and audit")
		}
		record, payload, failure := node.server.loadShare(keyID)
		if failure != nil || record.KeyID != keyID || record.Curve != "ed25519" || record.Epoch != 0 || !bytes.Equal(record.PublicKey, pub) || !sameIDs(record.Participants, participants) || payload.Owner != testOwner || payload.Account != account {
			t.Fatal("actual stored ceremony owner public key account epoch and membership")
		}
	}
	authority.keys[0].Operations = []string{"sign"}
	authority.publish(t)
	return pub, authority
}

func TestNativeSendPurposeRealProfiles(t *testing.T) {
	seed := sha256.Sum256([]byte("native-send-disposable-owner"))
	pub := ed25519.NewKeyFromSeed(seed[:]).Public().(ed25519.PublicKey)
	document := &nativepolicy.Document{Version: 2, Rules: []nativepolicy.Rule{{KeyID: "native-owner", Owner: testOwner, Tenant: "tenant-a", AgentDID: "did:layerx:" + strings.Repeat("aa", 32), OwnerPublicKey: hex.EncodeToString(pub), Operations: []string{NativeSendPurpose, NativeLocalGrantConsent}, Capabilities: []string{strings.Repeat("22", 32)}, MaximumValidityMS: 700000, RatePerMinute: 1000}}}
	c := newTestClusterWith(t, 5, true, testPolicy(), func(o *Options) { o.NativePolicy = document })
	imported, authority := importNativeSendOwner(t, c, "native-owner", "native-send-disposable-owner", "0x9999999999999999999999999999999999999999")
	if !bytes.Equal(imported, pub) {
		t.Fatal("real imported owner key")
	}
	signers := []string{"node-1", "node-2", "node-3"}
	expiry := uint64(time.Now().Add(10 * time.Minute).UnixMilli())
	purpose := nativeSendPurposeBytes(pub, expiry)
	digest, _, err := nativeSendPurposeV2Digest(purpose, pub, "0x9999999999999999999999999999999999999999")
	if err != nil {
		t.Fatal(err)
	}
	request := NativeSignRequest{Profile: 2, SessionID: "native-purpose", KeyID: "native-owner", Operation: NativeSendPurpose, Signers: signers, Purpose: hex.EncodeToString(purpose)}
	verifyNativeInventoryRequired(t, c, request)
	sign := func(req NativeSignRequest, subject string) []apiResult {
		return c.callAll(t, c.byID(req.Signers...), PathNativeSign, func(*testNode) any { return req }, c.idp.mint(t, c.idp.key, subject))
	}
	first := decodeOK[NativeSignResponse](t, "native purpose", sign(request, testOwner))
	verifyNativeResponses(t, pub, digest, request.Operation, first)
	replay := decodeOK[NativeSignResponse](t, "exact native purpose replay", sign(request, testOwner))
	for i := range replay {
		if replay[i] != first[i] {
			t.Fatal("exact durable native replay")
		}
	}
	restarted := c.restart(t, c.byID("node-2")[0])
	_ = restarted
	recovered := decodeOK[NativeSignResponse](t, "native restart replay", sign(request, testOwner))
	for i := range recovered {
		if recovered[i] != first[i] {
			t.Fatal("native replay changed after encrypted-store restart")
		}
	}
	altered := request
	changed := append([]byte(nil), purpose...)
	changed[len(changed)-1] ^= 1
	altered.Purpose = hex.EncodeToString(changed)
	for _, result := range sign(altered, testOwner) {
		expectError(t, "same session changed purpose", result, CodeSessionBadRequest)
	}
	parsed, parseErr := parseNativeSendPurposeV2(purpose)
	if parseErr != nil || parsed.OwnerDID != "did:layerx:"+strings.Repeat("cc", 32) || !bytes.Equal(parsed.ConsentPublicKey[:], pub) || !bytes.Equal(parsed.EVMSender[:], bytes.Repeat([]byte{0x99}, 20)) || parsed.TransactionType != 2 || parsed.ChainID != 125 || parsed.ModuleID != 1 || parsed.Ordinal != 5 {
		t.Fatal("actual closed send parser coordinates")
	}
	if _, _, err = nativeSendPurposeV2Digest(purpose, bytes.Repeat([]byte{9}, 32), "0x9999999999999999999999999999999999999999"); err == nil {
		t.Fatal("encoded owner must match real held-share owner")
	}
	if _, err = nativepolicy.ParsePurpose(purpose); err == nil {
		t.Fatal("old preparation profile cannot parse send purpose")
	}
	if _, err = parseNativeSendPurposeV2(nativePurposeBytes(expiry)); err == nil {
		t.Fatal("old preparation purpose cannot parse as send consent")
	}
	oldAsSend := request
	oldAsSend.SessionID = "old-purpose-as-send"
	oldAsSend.Purpose = hex.EncodeToString(nativePurposeBytes(expiry))
	for _, result := range sign(oldAsSend, testOwner) {
		expectError(t, "old purpose under send kind", result, CodeSessionBadRequest)
	}
	sendAsOld := request
	sendAsOld.SessionID = "send-purpose-as-old"
	sendAsOld.Operation = NativePreparationPurpose
	for _, result := range sign(sendAsOld, testOwner) {
		expectError(t, "send purpose under old kind", result, CodeSessionBadRequest)
	}
	if digest != sha256.Sum256(purpose) {
		t.Fatal("V2 signature digest must cover the exact canonical bytes")
	}
	if _, _, err = nativeSendPurposeV2Digest(purpose, pub, "0x8888888888888888888888888888888888888888"); err == nil {
		t.Fatal("encoded EVM sender must match the independently stored account")
	}
	fixed := len(purpose) - 306
	invalid := map[string][]byte{}
	legacy := append([]byte(nil), purpose...)
	copy(legacy, nativepolicy.SendPurposeDomain)
	legacy[len(nativeSendPurposeV2Domain)] = 1
	invalid["legacy-domain"] = legacy
	invalid["oversized"] = append(append([]byte(nil), purpose...), make([]byte, 2049-len(purpose))...)
	for name, span := range map[string][2]int{
		"consent-key": {fixed, 32}, "sender": {fixed + 32, 20},
		"generation": {fixed + 84, 8}, "expiry": {fixed + 92, 8},
		"preparation": {fixed + 146, 32}, "canonical-digest": {fixed + 178, 32},
		"economic-action": {fixed + 210, 32}, "idempotency": {fixed + 242, 32},
		"commitment": {fixed + 274, 32},
	} {
		raw := append([]byte(nil), purpose...)
		clear(raw[span[0] : span[0]+span[1]])
		invalid["zero-"+name] = raw
	}
	for name, offset := range map[string]int{
		"version": len(nativeSendPurposeV2Domain), "transaction-type": fixed + 132,
		"chain": fixed + 140, "activity-version": fixed + 141,
		"module": fixed + 143, "ordinal": fixed + 145,
	} {
		raw := append([]byte(nil), purpose...)
		raw[offset] ^= 1
		invalid[name] = raw
	}
	for name, raw := range invalid {
		if _, err = parseNativeSendPurposeV2(raw); err == nil {
			t.Fatalf("invalid V2 %s accepted", name)
		}
		refused := request
		refused.SessionID = "native-invalid-" + name
		refused.Purpose = hex.EncodeToString(raw)
		for _, result := range sign(refused, testOwner) {
			expectError(t, "invalid V2 "+name, result, CodeSessionBadRequest)
		}
	}
	for _, offset := range []int{len(nativeSendPurposeV2Domain), len(nativeSendPurposeV2Domain) + 5, fixed, fixed + 32, fixed + 52, fixed + 84, fixed + 92, fixed + 100, fixed + 132, fixed + 133, fixed + 141, fixed + 142, fixed + 144, fixed + 146, fixed + 178, fixed + 210, fixed + 242, fixed + 274} {
		changed := append([]byte(nil), purpose...)
		changed[offset] ^= 1
		changedRequest := request
		changedRequest.Purpose = hex.EncodeToString(changed)
		for _, result := range sign(changedRequest, testOwner) {
			expectError(t, "same session changed send field", result, CodeSessionBadRequest)
		}
	}
	for cut := 0; cut < len(purpose); cut++ {
		if _, err = parseNativeSendPurposeV2(purpose[:cut]); err == nil {
			t.Fatal("truncated send purpose accepted")
		}
	}

	foreign := request
	foreign.SessionID = "native-foreign-owner"
	for _, result := range sign(foreign, "user-0002") {
		expectError(t, "foreign original owner", result, CodeTokenNotOwner)
	}
	malformed := request
	malformed.SessionID = "native-malformed"
	malformed.Purpose = hex.EncodeToString(append(purpose, 0))
	for _, result := range sign(malformed, testOwner) {
		expectError(t, "canonical trailing bytes", result, CodeSessionBadRequest)
	}
	expired := request
	expired.SessionID = "native-expired"
	expired.Purpose = hex.EncodeToString(nativeSendPurposeBytes(pub, uint64(time.Now().Add(-time.Second).UnixMilli())))
	expectPolicyCode(t, "expired owner consent", sign(expired, testOwner), "native_consent_expired")
	tenant := request
	tenant.SessionID = "native-foreign-tenant"
	raw := append([]byte(nil), purpose...)
	raw[len(nativeSendPurposeV2Domain)+5] ^= 1
	tenant.Purpose = hex.EncodeToString(raw)
	expectPolicyCode(t, "foreign tenant coordinates", sign(tenant, testOwner), "destination_denied")
	few := request
	few.SessionID = "native-short-quorum"
	few.Signers = signers[:2]
	for _, result := range sign(few, testOwner) {
		expectError(t, "insufficient quorum", result, CodeQuorumTooFew)
	}
	capability, session := nativeGrantRecordsForAgent(pub, expiry, "did:layerx:"+strings.Repeat("aa", 32))
	grantDigest, _, err := nativepolicy.GrantDigest(capability, session, expiry, pub)
	if err != nil {
		t.Fatal(err)
	}
	grant := NativeSignRequest{Profile: 2, SessionID: "native-grant", KeyID: "native-owner", Operation: NativeLocalGrantConsent, Signers: signers, Capability: hex.EncodeToString(capability), NativeSession: hex.EncodeToString(session), ExpiryMS: strconv.FormatUint(expiry, 10)}
	verifyNativeResponses(t, pub, grantDigest, grant.Operation, decodeOK[NativeSignResponse](t, "genuine local grant consent", sign(grant, testOwner)))
	badGrant := grant
	badGrant.SessionID = "native-malformed-full-ordinal"
	bad := append([]byte(nil), session...)
	bad[len(bad)-5] = 0
	badGrant.NativeSession = hex.EncodeToString(bad)
	for _, result := range sign(badGrant, testOwner) {
		expectError(t, "closed native activity version", result, CodeSessionBadRequest)
	}
	unknown := c.call(t, c.nodes[0], PathSign, SignRequest{SessionID: "v1-native-kind", KeyID: "native-owner", Kind: NativeSendPurpose, Signers: signers}, c.idp.mint(t, c.idp.key, testOwner))
	expectError(t, "v1 unknown native kind unchanged", unknown, CodeSessionKind)
	runNativeSendRustConsumer(t, c, authority, document)
	for _, node := range c.nodes {
		if err := node.audit.Verify(); err != nil {
			t.Fatal(err)
		}
	}
}

func runNativeSendRustConsumer(t *testing.T, c *testCluster, authority *nativeInventoryAuthority, document *nativepolicy.Document) {
	t.Helper()
	binary := os.Getenv("PAXEER_X_NATIVE_SEND_RUST_TEST")
	if binary == "" {
		t.Fatal("prebuilt real Rust custody consumer is required")
	}
	walletSeed := sha256.Sum256([]byte("native send consumer secp256k1 wallet identity"))
	walletKey, err := gethcrypto.ToECDSA(walletSeed[:])
	if err != nil {
		t.Fatal(err)
	}
	sender := gethcrypto.PubkeyToAddress(walletKey.PublicKey)
	walletPublic := gethcrypto.FromECDSAPub(&walletKey.PublicKey)
	const walletKeyID = "native-send-wallet"
	secret := new(big.Int).SetBytes(walletSeed[:])
	defer dealer.Wipe(secret)
	bundles, _, err := dealer.Split(dealer.Secp256k1, secret, c.ids)
	if err != nil {
		t.Fatal(err)
	}
	shares := make(map[string]ShareBundleJSON, len(bundles))
	for _, bundle := range bundles {
		shares[bundle.ParticipantID] = EncodeBundle(bundle)
	}
	authority.keys = append(authority.keys, InventoryKey{KeyID: walletKeyID, Epoch: 0, Curve: "secp256k1", PublicKey: hex.EncodeToString(walletPublic), Owner: testOwner, Operations: []string{"import", "sign"}})
	authority.publish(t)
	responses := decodeOK[KeyResponse](t, "native send wallet ceremony import", c.callAll(t, c.nodes, PathImport, func(node *testNode) any {
		return ImportRequest{CeremonyID: "native-send-wallet-import", SessionID: "import-" + walletKeyID,
			KeyID: walletKeyID, Owner: testOwner, PublicKey: hex.EncodeToString(walletPublic),
			Participants: c.ids, Threshold: dealer.Threshold, Account: sender.Hex(), Share: shares[node.id]}
	}, ""))
	if len(responses) != len(c.nodes) {
		t.Fatal("wallet identity must be held by every node")
	}
	for i, node := range c.nodes {
		record, payload, failure := node.server.loadShare(walletKeyID)
		if failure != nil || record.Curve != "secp256k1" || record.Epoch != 0 || !bytes.Equal(record.PublicKey, walletPublic) ||
			!sameIDs(record.Participants, c.ids) || payload.Owner != testOwner || common.HexToAddress(payload.Account) != sender ||
			responses[i].NodeID != node.id || common.HexToAddress(responses[i].Address) != sender || responses[i].AuditSequence == 0 {
			t.Fatal("held wallet identity must match the real owner, public key, sender and membership")
		}
	}
	authority.keys[len(authority.keys)-1].Operations = []string{"sign"}
	authority.publish(t)
	dir := t.TempDir()
	pair := c.client.Transport.(*http.Transport).TLSClientConfig.Certificates[0]
	private, err := x509.MarshalPKCS8PrivateKey(pair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	nodes := make([]map[string]string, 0, 5)
	for _, node := range c.nodes {
		nodes = append(nodes, map[string]string{"id": node.id, "address": node.apiAddr})
	}
	fixture := map[string]any{"schema": "layerx-human-native-send-live.v1", "directory": dir, "nodes": nodes, "signers": []string{"node-1", "node-2", "node-3"},
		"root_der": c.ca.cert.Raw, "client_der": pair.Certificate[0], "client_pkcs8": private, "owner": testOwner, "account": sender.Hex(),
		"owner_assertion": c.idp.mint(t, c.idp.key, testOwner), "second_assertion": c.idp.mint(t, c.idp.key, testOwner), "foreign_assertion": c.idp.mint(t, c.idp.key, "user-0002")}
	path := filepath.Join(dir, "fixture.json")
	raw, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	command := nativeRustCommand(t, binary, "native_send_provider_and_sdk_real_cluster")
	command.Env = append(os.Environ(), "PAXEER_X_NATIVE_SEND_LIVE_FIXTURE="+path)
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill() })
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	var provision struct {
		KeyID string `json:"key_id"`
	}
	if err = json.Unmarshal(nativeWaitFile(t, filepath.Join(dir, "provision.json"), done), &provision); err != nil || provision.KeyID == "" {
		t.Fatal("authentic Rust provider binding request")
	}
	authority.keys = append(authority.keys, InventoryKey{KeyID: provision.KeyID, Epoch: 0, Curve: "ed25519", Owner: testOwner, Operations: []string{"generate"}})
	authority.publish(t)
	if err = os.WriteFile(filepath.Join(dir, "provision-ready"), []byte("ready"), 0o600); err != nil {
		t.Fatal(err)
	}
	var generated struct {
		Public []byte `json:"public"`
	}
	if err = json.Unmarshal(nativeWaitFile(t, filepath.Join(dir, "generated.json"), done), &generated); err != nil || len(generated.Public) != 32 {
		t.Fatal("actual Rust production-provider generated key")
	}
	for _, node := range c.nodes {
		record, payload, e := node.server.loadShare(provision.KeyID)
		if e != nil || record.Epoch != 0 || payload.Owner != testOwner || !bytes.Equal(record.PublicKey, generated.Public) {
			t.Fatal("real stored quorum key lineage")
		}
	}
	authority.keys[len(authority.keys)-1] = InventoryKey{KeyID: provision.KeyID, Epoch: 0, Curve: "ed25519", PublicKey: hex.EncodeToString(generated.Public), Owner: testOwner, Operations: []string{"sign"}}
	authority.publish(t)
	document.Rules = append(document.Rules, nativepolicy.Rule{KeyID: provision.KeyID, Owner: testOwner, Tenant: "tenant-a", AgentDID: "did:layerx:" + strings.Repeat("aa", 32), OwnerPublicKey: hex.EncodeToString(generated.Public), Operations: []string{NativeSendPurpose, NativeLocalGrantConsent}, Capabilities: []string{strings.Repeat("22", 32)}, MaximumValidityMS: 700000, RatePerMinute: 1000})
	if err = document.Validate(); err != nil {
		t.Fatal(err)
	}
	expiry := uint64(time.Now().Add(10 * time.Minute).UnixMilli())
	capability, session := nativeGrantRecordsForAgent(generated.Public, expiry, "did:layerx:"+strings.Repeat("aa", 32))
	purpose := nativeSendPurposeBytesForSender(generated.Public, expiry, sender)
	parsed, err := parseNativeSendPurposeV2(purpose)
	if err != nil || common.Address(parsed.EVMSender) != sender {
		t.Fatal("canonical send purpose must encode the held wallet sender")
	}
	material := map[string]any{"purpose": purpose, "capability": capability, "native_session": session, "expiry_ms": expiry}
	raw, err = json.Marshal(material)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "signing.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("real Rust provider consumer failed: %v", err)
		}
	case <-time.After(5 * time.Minute):
		t.Fatal("actual Rust provider consumer deadline")
	}
	fmt.Println("native custody real provider handshake complete")
}
