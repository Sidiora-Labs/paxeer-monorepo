// Proves: Server.HandleGenerate/HandleVerification/HandleLXReview/HandleNativeSign/HandleAuthority -> Server.Serve mTLS HTTP and encrypted store -> ceremony.Client, KMS AttestorClient, gateway parsers/action validator and loadTester.
//
// Repairs consumer response identity/audit checks in loadtest, KMS and gateway,
// and replica agreement in ceremony.Client.DescribeCeremony. The action validator
// is shared with orchestrator.broadcastLeg so the proof checks retained bytes at
// the production consumer boundary. Ceremony's ExternalAttestorConfig avoids its
// testsupport.Nodes simulator; every protocol participant here is a real Server.
// authority.go inventoryRoute recovers an existing generation idempotently; the
// proof checks that routed behavior and reads durable state through HandleDescribe.
// authority.go Inventory.Check refuses loadtest's unapproved random key identifiers;
// the real loadTester must surface that refusal rather than bypass inventory.
package server

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/auth/agent"
	nativepolicy "github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy/native"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/tss/dealershare"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
	prommodel "github.com/prometheus/common/model"
)

func proofS28Write(t *testing.T, path string, value []byte) {
	t.Helper()
	if err := os.WriteFile(path, value, 0600); err != nil {
		t.Fatal(err)
	}
}

func proofS28JSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func proofS28Run(t *testing.T, dir string, env []string, command string, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("consumer %s: %v\n%s", command, err, out)
	}
	return out
}

func proofS28Signed(t *testing.T, key *ecdsa.PrivateKey, audience string, value any) string {
	t.Helper()
	header := b64(proofS28JSON(t, map[string]string{"alg": "ES256", "typ": audience + "+jwt"}))
	input := header + "." + b64(proofS28JSON(t, value))
	digest := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	return input + "." + b64(signature)
}

func TestProofS28WalletAttestorsTss(t *testing.T) {
	root, err := filepath.Abs("../../../../../")
	if err != nil {
		t.Fatal(err)
	}
	wallet := filepath.Join(root, "services/wallet")
	registries := map[string]*prometheus.Registry{}
	const ownerKey = "proof-s28-owner"
	const label = "proof s28 real owner"
	const account = "0x5555555555555555555555555555555555555555"
	seed := sha256.Sum256([]byte(label))
	pub := ed25519.NewKeyFromSeed(seed[:]).Public().(ed25519.PublicKey)
	native := &nativepolicy.Document{Version: 2, Rules: []nativepolicy.Rule{{KeyID: ownerKey, Owner: testOwner, Tenant: "tenant-a", AgentDID: "did:layerx:alice", OwnerPublicKey: hex.EncodeToString(pub), Operations: []string{NativePreparationPurpose}, Capabilities: []string{strings.Repeat("22", 32)}, MaximumValidityMS: 700000, RatePerMinute: 1000}}}
	tuneMetrics := metricOptions(t, registries)
	doc := testPolicy()
	doc.Defaults.Kinds = append(doc.Defaults.Kinds, KindOperatorVerification)
	c := newTestClusterWith(t, 5, true, doc, func(o *Options) { tuneMetrics(o); o.NativePolicy = native })
	importKernelKey(t, c, ownerKey, label, account)
	inventory := c.importInventory
	// Admit generation with no preselected public key; the actual five-party DKG
	// produces the material later consumed by both Go and Rust clients.
	for _, id := range []string{"proof-s28-generated", "proof-s28-kms"} {
		inventory.keys = append(inventory.keys, InventoryKey{KeyID: id, Curve: "ed25519", Owner: testOwner, Operations: []string{"generate"}})
	}
	inventory.publish(t)
	generate := GenerateRequest{SessionID: "proof-s28-dkg", KeyID: "proof-s28-generated", Curve: "ed25519", Owner: testOwner, Account: account}
	generated := decodeOK[KeyResponse](t, "distributed generation", c.callAll(t, c.nodes, PathGenerate, func(*testNode) any { return generate }, ""))
	for i, r := range generated {
		if r.NodeID != c.ids[i] || r.PublicKey != generated[0].PublicKey || len(r.PublicKey) != 64 || r.AuditSequence == 0 || !reflect.DeepEqual(r.Participants, c.ids) {
			t.Fatalf("generated identity: %+v", r)
		}
	}
	expectError(t, "operator cannot generate", c.callAs(t, c.opClient, c.nodes[0], PathGenerate, generate, ""), CodeOperatorRequired)
	badGenerate := generate
	badGenerate.KeyID = "unknown-generation"
	expectError(t, "inventory refuses unknown key", c.call(t, c.nodes[0], PathGenerate, badGenerate, ""), CodeKeyInvalidShare)

	// Authority snapshots are real ES256 producer records admitted over the route.
	previousTune := c.tune
	c.tune = func(o *Options) {
		previousTune(o)
		authority, e := agent.NewAuthority(agent.AuthorityConfig{PublicKeyFile: inventory.public, Issuer: "native-corpus", Tenant: "tenant-a", Store: o.Store, ChainID: testChainID})
		if e != nil {
			t.Fatal(e)
		}
		o.Authority = authority
	}
	for _, n := range c.nodes {
		o := n.server.opts
		c.tune(&o)
		n.server.opts.Authority = o.Authority
	}
	now := time.Now().Unix()
	snapshot := agent.AuthoritySnapshot{SignedClaims: agent.SignedClaims{Version: 1, Issuer: "native-corpus", Audience: agent.AuthorityAudience, Tenant: "tenant-a", Sequence: "1", IssuedAt: now - 1, ExpiresAt: now + 59}, Principals: []agent.AuthorityPrincipal{}}
	authorityToken := proofS28Signed(t, inventory.key, agent.AuthorityAudience, snapshot)
	type ack struct {
		NodeID   string `json:"node_id"`
		Sequence string `json:"sequence"`
	}
	admit := func() []ack {
		return decodeOK[ack](t, "custody authority", c.callAll(t, c.nodes, PathAuthority, func(*testNode) any { return map[string]string{"token": authorityToken} }, ""))
	}
	for i, r := range admit() {
		if r.NodeID != c.ids[i] || r.Sequence != "1" {
			t.Fatalf("authority ack: %+v", r)
		}
	}
	expectError(t, "operator cannot publish authority", c.callAs(t, c.opClient, c.nodes[0], PathAuthority, map[string]string{"token": authorityToken}, ""), CodeOperatorRequired)
	expectError(t, "invalid authority signature", c.call(t, c.nodes[0], PathAuthority, map[string]string{"token": authorityToken + "x"}, ""), CodeAgentInvalid)

	for _, n := range append([]*testNode(nil), c.nodes...) {
		c.restart(t, n)
	}
	// Replay after the API, peer transport, audit log and encrypted store all closed.
	replay := decodeOK[KeyResponse](t, "generation after restart", c.callAll(t, c.nodes, PathGenerate, func(*testNode) any { return generate }, ""))
	for i, r := range replay {
		if r.PublicKey != generated[i].PublicKey || r.Epoch != generated[i].Epoch || r.AuditSequence <= generated[i].AuditSequence {
			t.Fatalf("durable DKG replay: %+v", r)
		}
	}
	for _, n := range c.nodes {
		if e := n.server.opts.Authority.RequireSequence("1"); e != nil {
			t.Fatalf("authority lost on restart: %v", e)
		}
	}
	for _, r := range admit() {
		if r.Sequence != "1" {
			t.Fatal("authority replay changed sequence")
		}
	}

	// Native-v2 signing produces an actual 3-of-5 Ed25519 signature and persists
	// the exact response, including its audit sequence, across a process restart.
	signers := c.ids[:3]
	purpose := nativePurposeBytes(uint64(time.Now().Add(10 * time.Minute).UnixMilli()))
	digest, _, err := nativepolicy.PurposeDigest(purpose)
	if err != nil {
		t.Fatal(err)
	}
	request := NativeSignRequest{Profile: 2, SessionID: "proof-s28-native", KeyID: ownerKey, Operation: NativePreparationPurpose, Signers: signers, Purpose: hex.EncodeToString(purpose)}
	signNative := func(req NativeSignRequest) []apiResult {
		return c.callAll(t, c.byID(signers...), PathNativeSign, func(*testNode) any { return req }, c.idp.mint(t, c.idp.key, testOwner))
	}
	nativeResponses := decodeOK[NativeSignResponse](t, "native signing", signNative(request))
	verifyNativeResponses(t, pub, digest, request.Operation, nativeResponses)
	c.restart(t, c.nodes[1])
	nativeReplay := decodeOK[NativeSignResponse](t, "native durable replay", signNative(request))
	if !reflect.DeepEqual(nativeReplay, nativeResponses) {
		t.Fatal("native evidence changed after close and reopen")
	}
	malformed := request
	malformed.Purpose = "00"
	expectError(t, "malformed native purpose", signNative(malformed)[0], CodeSessionBadRequest)
	expectError(t, "operator cannot sign native", c.callAs(t, c.opClient, c.nodes[0], PathNativeSign, request, ""), CodeOperatorRequired)

	pending := newPendingSend(t, pub, nil, 7, 5000, uint64(time.Now().Unix()-60), uint64(time.Now().Unix()+600))
	unsigned := pending.envelope(t)
	reviewRequest := LXReviewRequest{SessionID: "proof-s28-review", KeyID: ownerKey, Kind: KindLXSendAuth, Activity: hex.EncodeToString(unsigned)}
	review := decodeOK[LXReviewResponse](t, "owner review", []apiResult{c.call(t, c.nodes[0], PathLXReview, reviewRequest, c.idp.mint(t, c.idp.key, testOwner))})[0]
	if review.Kind != KindLXSendAuth || review.SigningPreimage != hex.EncodeToString(pending.digest(t)) || review.PublicKey != hex.EncodeToString(pub) || review.AuditSequence == 0 || review.Activity != reviewRequest.Activity {
		t.Fatalf("review differs: %+v", review)
	}
	// Regression: a held key retired from signed inventory used to remain reviewable.
	saved := append([]InventoryKey(nil), inventory.keys...)
	inventory.keys = inventory.keys[1:]
	inventory.publish(t)
	expectError(t, "retired key cannot be reviewed", c.call(t, c.nodes[0], PathLXReview, reviewRequest, c.idp.mint(t, c.idp.key, testOwner)), CodeKeyInvalidShare)
	inventory.keys = saved
	inventory.publish(t)
	signedSend := decodeOK[SignResponse](t, "review to signing", authorizeSend(t, c, ownerKey, "proof-s28-send", signers, pending))
	for _, r := range signedSend {
		sig, e := hex.DecodeString(r.Signature)
		if e != nil || !ed25519.Verify(pub, pending.digest(t), sig) || r.SignedBytes != review.SigningPreimage {
			t.Fatal("consumer observed a different send signature")
		}
	}
	refused := SignRequest{SessionID: "proof-s28-wrong-signer", KeyID: ownerKey, Kind: KindLXSendAuth, Signers: []string{c.ids[0], c.ids[1], "unknown-node"}}
	expectError(t, "unknown signer", c.call(t, c.nodes[0], PathSign, refused, c.idp.mint(t, c.idp.key, testOwner)), CodeQuorumNotMember)
	if got := SignKinds(); !reflect.DeepEqual(got, []string{KindEVMTransaction, KindTypedData, KindPersonalMessage, KindEthSignDigest, KindLXBind, KindLXGrant, KindLXSendAuth}) {
		t.Fatalf("sign kinds: %v", got)
	}

	// The ceremony consumer signs and re-reads a real imported/refreshed key.
	verifyPub := importKernelKey(t, c, "proof-s28-verify", "proof s28 verification", account)
	decodeOK[KeyResponse](t, "ceremony refresh", c.callAll(t, c.nodes, PathRefresh, func(*testNode) any {
		return RefreshRequest{SessionID: "proof-s28-refresh", KeyID: "proof-s28-verify", CeremonyID: "metrics-ceremony", ImportSessionID: "import-proof-s28-verify", ExpectedEpoch: new(uint64)}
	}, ""))
	c.publishImportEpoch(t, "proof-s28-verify", 1)
	proofS28Ceremony(t, c, root, wallet, verifyPub)
	for _, n := range append([]*testNode(nil), c.nodes...) {
		c.restart(t, n)
	}
	proofS28Ceremony(t, c, root, wallet, verifyPub)

	proofS28KMS(t, c, root, GenerateRequest{KeyID: "proof-s28-kms", Owner: testOwner, Account: account})

	// The action consumer checks an actual EVM threshold signature against its draft.
	evmKey := importKey(t, c, "proof-s28-evm", dealer.Secp256k1, big.NewInt(424242), "")[0]
	decodeOK[KeyResponse](t, "EVM refresh", c.callAll(t, c.nodes, PathRefresh, func(*testNode) any {
		return RefreshRequest{SessionID: "proof-s28-evm-refresh", KeyID: evmKey.KeyID, CeremonyID: "metrics-ceremony", ImportSessionID: "import-" + evmKey.KeyID, ExpectedEpoch: new(uint64)}
	}, ""))
	c.publishImportEpoch(t, evmKey.KeyID, 1)
	tx, raw := metricsTransaction(t, big.NewInt(1000))
	evmResponses := decodeOK[SignResponse](t, "EVM signing", c.callAll(t, c.byID(signers...), PathSign, func(*testNode) any {
		return SignRequest{SessionID: "proof-s28-evm-sign", KeyID: evmKey.KeyID, Kind: KindEVMTransaction, Signers: signers, Transaction: raw}
	}, c.idp.mint(t, c.idp.key, testOwner)))
	sig, err := hex.DecodeString(evmResponses[0].Signature)
	if err != nil {
		t.Fatal(err)
	}
	signedTx, err := tx.WithSignature(types.LatestSignerForChainID(big.NewInt(testChainID)), sig)
	if err != nil {
		t.Fatal(err)
	}
	signedRaw, err := signedTx.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	proofS28Gateway(t, root, wallet, review, generated[0], evmResponses[0], "0x"+hex.EncodeToString(signedRaw), evmKey.Address)
	proofS28Loadtest(t, c, wallet)

	// Scrape the served operator endpoint and parse Prometheus text, rather than
	// reading the registry directly. Gateways are refused by the same real route.
	resp, err := c.opClient.Get("https://" + c.nodes[0].apiAddr + PathMetrics)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("metrics %d: %v", resp.StatusCode, err)
	}
	parser := expfmt.NewTextParser(prommodel.LegacyValidation)
	metrics, err := parser.TextToMetricFamilies(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	family := metrics["attestor_sessions_completed_total"]
	if family == nil {
		t.Fatal("served completion metric absent")
	}
	var completed float64
	for _, m := range family.Metric {
		completed += m.GetCounter().GetValue()
	}
	if completed < 1 {
		t.Fatal("served completion metric did not observe EVM session")
	}
	denied, err := c.client.Get("https://" + c.nodes[0].apiAddr + PathMetrics)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(denied.Body)
	denied.Body.Close()
	if denied.StatusCode != http.StatusForbidden {
		t.Fatalf("gateway metrics status %d", denied.StatusCode)
	}
	expectError(t, "gateway metrics", apiResult{status: denied.StatusCode, body: body}, CodeOperatorRequired)
}

func proofS28Ceremony(t *testing.T, c *testCluster, root, wallet string, pub []byte) {
	t.Helper()
	dir := t.TempDir()
	cert := c.operator.issue(t, dir, "proof-operator", x509.ExtKeyUsageClientAuth)
	type node struct {
		ID  string
		URL string
		Pin [32]byte
	}
	nodes := make([]node, len(c.nodes))
	for i, n := range c.nodes {
		pin, err := hex.DecodeString(c.peers[i].SPKISHA256)
		if err != nil {
			t.Fatal(err)
		}
		nodes[i] = node{ID: n.id, URL: "https://" + n.apiAddr}
		copy(nodes[i].Pin[:], pin)
	}
	proofS28Write(t, filepath.Join(dir, "config.json"), proofS28JSON(t, map[string]any{"CertFile": cert.certPath, "KeyFile": cert.keyPath, "CAFile": c.ca.pemPath, "Nodes": nodes}))
	mod := fmt.Sprintf("module github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/proof\n\ngo 1.25.6\nrequire github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony v0.0.0\nreplace github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony => %s/ceremony\nreplace github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor => %s/attestor\nreplace github.com/Sidiora-Labs/Paxeer-X-Network => %s\n", wallet, wallet, root)
	proofS28Write(t, filepath.Join(dir, "go.mod"), []byte(mod))
	proofS28Write(t, filepath.Join(dir, "main.go"), []byte(proofS28CeremonyProgram))
	out := proofS28Run(t, dir, []string{"GOWORK=off"}, "go", "run", "-mod=mod", ".", filepath.Join(dir, "config.json"), hex.EncodeToString(pub))
	if !bytes.Contains(out, []byte("ceremony-consumer-ok")) {
		t.Fatalf("ceremony output: %s", out)
	}
}

const proofS28CeremonyProgram = `package main
import("context";"crypto/ed25519";"encoding/hex";"encoding/json";"errors";"fmt";"os";a "github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/attestor";s "github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/testsupport";"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/dealer")
func main(){var cfg a.Config;raw,e:=os.ReadFile(os.Args[1]);must(e);must(json.Unmarshal(raw,&cfg));cfg,e=s.ExternalAttestorConfig(cfg.CertFile,cfg.KeyFile,cfg.CAFile,cfg.Nodes);must(e);c,e:=a.New(cfg);must(e);defer c.Close();ctx:=context.Background();key,e:=a.DecodePoint(dealer.Ed25519,os.Args[2]);must(e);v,e:=c.SignVerificationCeremony(ctx,"metrics-ceremony","proof-s28-verification","","proof-s28-verify","import-proof-s28-verify",key,1);must(e);pub,e:=hex.DecodeString(os.Args[2]);must(e);if !ed25519.Verify(pub,v.Message,v.Signature)||len(v.AuditSeqs)!=3{panic("verification consumer evidence")};rows,e:=c.DescribeCeremony(ctx,"proof-s28-verify");must(e);for _,r:=range rows{if r.PublicKey!=os.Args[2]||r.Epoch!=1{panic("describe state")}};_,e=c.SignVerificationCeremony(ctx,"metrics-ceremony","proof-s28-duplicate","","proof-s28-verify","import-proof-s28-verify",key,1);var refusal *a.APIError;if !errors.As(e,&refusal)||refusal.Code!="verification_used"{panic(fmt.Sprintf("duplicate: %v",e))};_,e=c.DescribeCeremony(ctx,"unknown-key");if !errors.As(e,&refusal)||refusal.Code!="key_not_found"{panic(fmt.Sprintf("unknown: %v",e))};fmt.Println("ceremony-consumer-ok")}
func must(e error){if e!=nil{panic(e)}}
`

func proofS28Gateway(t *testing.T, root, wallet string, review LXReviewResponse, key KeyResponse, signature SignResponse, raw, address string) {
	t.Helper()
	dir := t.TempDir()
	proofS28Write(t, filepath.Join(dir, "producer.json"), proofS28JSON(t, map[string]any{"review": review, "key": key, "signature": signature, "raw": raw, "address": address}))
	program := strings.ReplaceAll(proofS28GatewayProgram, "__GATEWAY__", filepath.ToSlash(filepath.Join(wallet, "gateway/src")))
	proofS28Write(t, filepath.Join(dir, "consumer.mts"), []byte(program))
	env := []string{"NODE_ENV=test", "CORS_ORIGINS=http://127.0.0.1", "SUPABASE_URL=http://127.0.0.1:1", "DATABASE_URL=postgresql://localhost/proof", "WALLET_MASTER_KEY=" + base64.StdEncoding.EncodeToString(make([]byte, 32))}
	out := proofS28Run(t, filepath.Join(wallet, "gateway"), env, "pnpm", "exec", "tsx", filepath.Join(dir, "consumer.mts"), filepath.Join(dir, "producer.json"))
	if !bytes.Contains(out, []byte("gateway-consumer-ok")) {
		t.Fatalf("gateway output: %s", out)
	}
}

const proofS28GatewayProgram = `
import { readFileSync } from 'node:fs';
import { parseKeyResponse, parseSignResponse, parseLxReview, lxSigningPreimage, struct } from '__GATEWAY__/attestor/client.ts';
import { assertSignedActionTransaction } from '__GATEWAY__/agent/actions/orchestrator.ts';
const p = JSON.parse(readFileSync(process.argv[2]!, 'utf8'));
const check = (v: unknown, message: string) => { if (!v) throw new Error(message); };
const refuses = async (f: () => unknown) => { let refused = false; try { await f(); } catch { refused = true; } check(refused, 'malformed producer accepted'); };
check(struct(p.review) && !struct([]), 'wire struct');
const review = parseLxReview(p.review, p.review.node_id);
check(lxSigningPreimage(review.activity, review.kind) === review.signing_preimage, 'consumer send digest differs');
check(parseKeyResponse(p.key, p.key.node_id).public_key === p.key.public_key, 'key consumer');
check(parseSignResponse(p.signature, p.signature.node_id, 'secp256k1').signature === p.signature.signature, 'sign consumer');
await refuses(() => parseKeyResponse(p.key, 'wrong-node'));
await refuses(() => parseSignResponse(p.signature, 'wrong-node', 'secp256k1'));
await refuses(() => parseLxReview(p.review, 'wrong-node'));
await refuses(() => lxSigningPreimage(review.activity + '00', review.kind));
const draft = {nonce:7,chainId:125,to:'0x1111111111111111111111111111111111111111',data:'0x',value:'1000',gas:'21000',maxFeePerGas:'2000000000',maxPriorityFeePerGas:'1000000000'};
await assertSignedActionTransaction(p.raw, p.address, draft);
await refuses(() => assertSignedActionTransaction(p.raw, '0x2222222222222222222222222222222222222222', draft));
await refuses(() => assertSignedActionTransaction(p.raw, p.address, {...draft, nonce:8}));
console.log('gateway-consumer-ok');
`

func proofS28KMS(t *testing.T, c *testCluster, root string, generate GenerateRequest) {
	t.Helper()
	dir := t.TempDir()
	cert := c.ca.issue(t, dir, "proof-kms", x509.ExtKeyUsageClientAuth)
	private, err := x509.MarshalPKCS8PrivateKey(cert.pair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	proofS28Write(t, filepath.Join(dir, "client.der"), cert.cert.Raw)
	proofS28Write(t, filepath.Join(dir, "private.der"), private)
	proofS28Write(t, filepath.Join(dir, "ca.der"), c.ca.cert.Raw)
	var nodes []string
	for _, n := range c.nodes {
		nodes = append(nodes, n.id+"="+n.apiAddr)
	}
	manifest := fmt.Sprintf("[package]\nname = \"s28-kms-consumer\"\nversion = \"0.0.0\"\nedition = \"2024\"\n[workspace]\n[dependencies]\nlayerx-human-kms = { path = %q }\n", filepath.Join(root, "services/human/crates/layerx-human-kms"))
	proofS28Write(t, filepath.Join(dir, "Cargo.toml"), []byte(manifest))
	if err := os.Mkdir(filepath.Join(dir, "src"), 0700); err != nil {
		t.Fatal(err)
	}
	proofS28Write(t, filepath.Join(dir, "src/main.rs"), []byte(proofS28KMSProgram))
	out := proofS28Run(t, dir, []string{"CARGO_TARGET_DIR=" + filepath.Join(os.TempDir(), "s28-kms-target")}, "cargo", "run", "--quiet", "--manifest-path", filepath.Join(dir, "Cargo.toml"), "--", dir, strings.Join(nodes, ","), generate.KeyID, generate.Owner, generate.Account)
	if !bytes.Contains(out, []byte("kms-consumer-ok")) {
		t.Fatalf("KMS output: %s", out)
	}
	var public string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "kms-public=") {
			public = strings.TrimPrefix(line, "kms-public=")
		}
	}
	if len(public) != 64 {
		t.Fatalf("KMS did not report generated public key: %s", out)
	}
	for _, n := range append([]*testNode(nil), c.nodes...) {
		c.restart(t, n)
	}
	persisted := decodeOK[KeyResponse](t, "KMS generation after restart", c.callAll(t, c.nodes, PathDescribe, func(*testNode) any {
		return DescribeRequest{SessionID: "proof-s28-kms-describe", KeyID: generate.KeyID}
	}, ""))
	for i, response := range persisted {
		if response.NodeID != c.ids[i] || response.PublicKey != public || response.AuditSequence == 0 {
			t.Fatalf("KMS durable consumer state: %+v", response)
		}
	}
}

const proofS28KMSProgram = `
use layerx_human_kms::attestor::{AttestorClient, AttestorError};
use std::{env,fs,net::SocketAddr,time::Duration};
fn main() {
 let args: Vec<String> = env::args().collect();
 let read = |name: &str| fs::read(format!("{}/{}",args[1],name)).unwrap();
 let nodes: Vec<(String,SocketAddr)> = args[2].split(',').map(|v| {let (id,addr)=v.split_once('=').unwrap();(id.to_string(),addr.parse().unwrap())}).collect();
 let client=AttestorClient::new(&nodes,&[read("ca.der")],&[read("client.der")],&read("private.der"),Duration::from_secs(20)).unwrap();
 for (id,_) in &nodes {let health=client.health(id).unwrap();assert_eq!(health.node_id,*id);assert!(health.ready);assert!(health.audit_sequence>0);}
 let key=client.generate_ed25519(&args[3],&args[4],&args[5]).unwrap();
 let public:String=key.public_key.iter().map(|b|format!("{b:02x}")).collect();
 assert_eq!(key.audit.len(),5);assert!(key.audit.values().all(|seq|*seq>0));
 println!("kms-public={public}");
 let repeated=client.generate_ed25519(&args[3],&args[4],&args[5]).unwrap();
 assert_eq!(repeated.public_key,key.public_key);assert_eq!(repeated.key_id,key.key_id);
 assert!(repeated.audit.iter().all(|(id,seq)|*seq>key.audit[id]));
 assert!(matches!(client.generate_ed25519("unknown-key",&args[4],&args[5]), Err(AttestorError::Refused{code,..}) if code == "key_invalid_share"));
 println!("kms-consumer-ok");
}
`

func proofS28Loadtest(t *testing.T, c *testCluster, wallet string) {
	t.Helper()
	dir := t.TempDir()
	cert := c.ca.issue(t, dir, "proof-loadtest", x509.ExtKeyUsageClientAuth)
	var nodes []string
	for _, n := range c.nodes {
		nodes = append(nodes, n.id+"="+n.apiAddr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/loadtest", "--endpoints", strings.Join(nodes, ","), "--client-cert", cert.certPath, "--client-key", cert.keyPath, "--ca", c.ca.pemPath, "--chain-id", "125", "--token", c.idp.mint(t, c.idp.key, testOwner), "--duration", "1ms", "--concurrency", "1")
	cmd.Dir = filepath.Join(wallet, "attestor")
	out, err := cmd.CombinedOutput()
	if err == nil || !bytes.Contains(out, []byte(CodeKeyInvalidShare)) {
		t.Fatalf("loadtest must report real inventory refusal: %v\n%s", err, out)
	}
}
