package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/health"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/tss/dealer"
)

func gatheredMetric(t *testing.T, registry *prometheus.Registry, name, label, value string) *dto.Metric {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			for _, pair := range metric.GetLabel() {
				if pair.GetName() == label && pair.GetValue() == value {
					return metric
				}
			}
		}
	}
	t.Fatalf("missing %s{%s=%q}", name, label, value)
	return nil
}

func metricOptions(t *testing.T, registries map[string]*prometheus.Registry) func(*Options) {
	t.Helper()
	return func(opts *Options) {
		registry := prometheus.NewRegistry()
		codes := ErrorCodes()
		refusalCodes := make([]string, 0, len(codes))
		for code := range codes {
			refusalCodes = append(refusalCodes, code)
		}
		metrics, err := health.NewMetrics(registry, refusalCodes)
		if err != nil {
			t.Fatal(err)
		}
		registries[opts.NodeID] = registry
		opts.Metrics = metrics
	}
}

func metricsTransaction(t *testing.T, value *big.Int) (*types.Transaction, string) {
	t.Helper()
	to := common.HexToAddress("0x1111111111111111111111111111111111111111")
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID: big.NewInt(testChainID), Nonce: 7, GasTipCap: big.NewInt(1_000_000_000),
		GasFeeCap: big.NewInt(2_000_000_000), Gas: 21000, To: &to, Value: value,
	})
	raw, err := tx.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return tx, hex.EncodeToString(raw)
}

func importMetricsKey(t *testing.T, c *testCluster, keyID string) ([]KeyResponse, *nativeInventoryAuthority) {
	t.Helper()
	bundles, publicKey, err := dealer.Split(dealer.Secp256k1, big.NewInt(424242), c.ids)
	if err != nil {
		t.Fatal(err)
	}
	publicBytes, err := publicKeyBytes(dealer.Secp256k1, publicKey)
	if err != nil {
		t.Fatal(err)
	}
	authorityKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	authorityPublic, err := x509.MarshalPKIXPublicKey(&authorityKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	authority := &nativeInventoryAuthority{
		key: authorityKey, path: filepath.Join(dir, "inventory.jwt"), public: filepath.Join(dir, "authority.pem"), cluster: c,
		keys: []InventoryKey{{
			KeyID: keyID, Curve: "secp256k1", PublicKey: hex.EncodeToString(publicBytes), Owner: testOwner,
			Operations: []string{"import", "refresh", "sign"},
		}},
	}
	writeTestPEM(t, authority.public, "PUBLIC KEY", authorityPublic)
	authority.publish(t)
	byNode := make(map[string]ShareBundleJSON, len(bundles))
	for _, bundle := range bundles {
		byNode[bundle.ParticipantID] = EncodeBundle(bundle)
	}
	return decodeOK[KeyResponse](t, "import metrics key", c.callAll(t, c.nodes, PathImport, func(node *testNode) any {
		return ImportRequest{
			CeremonyID: "metrics-ceremony", SessionID: "import-" + keyID, KeyID: keyID, Owner: testOwner,
			PublicKey: hex.EncodeToString(publicBytes), Participants: c.ids, Threshold: dealer.Threshold,
			Share: byNode[node.id],
		}
	}, "")), authority
}

func TestSigningSessionRecordsSessionMetrics(t *testing.T) {
	registries := make(map[string]*prometheus.Registry)
	tune := metricOptions(t, registries)
	c := newTestClusterWith(t, 5, true, testPolicy(), func(opts *Options) {
		if opts.NodeID != "node-5" {
			tune(opts)
		}
	})
	const keyID = "metrics-evm"
	imported, authority := importMetricsKey(t, c, keyID)
	epoch := uint64(0)
	decodeOK[KeyResponse](t, "refresh metrics key", c.callAll(t, c.nodes, PathRefresh, func(*testNode) any {
		return RefreshRequest{
			SessionID: "metrics-refresh", KeyID: keyID, CeremonyID: "metrics-ceremony",
			ImportSessionID: "import-" + keyID, ExpectedEpoch: &epoch,
		}
	}, ""))
	authority.keys[0].Epoch = 1
	authority.publish(t)

	signers := []string{"node-1", "node-3", "node-5"}
	tx, raw := metricsTransaction(t, big.NewInt(1_000_000_000_000_000))
	token := c.idp.mint(t, c.idp.key, testOwner)
	results := decodeOK[SignResponse](t, "metrics EVM signing", c.callAll(t, c.byID(signers...), PathSign, func(*testNode) any {
		return SignRequest{SessionID: "metrics-sign", KeyID: keyID, Kind: KindEVMTransaction, Signers: signers, Transaction: raw}
	}, token))
	txSigner := types.LatestSignerForChainID(big.NewInt(testChainID))
	for _, result := range results {
		sig, err := hex.DecodeString(result.Signature)
		if err != nil {
			t.Fatal(err)
		}
		signed, err := tx.WithSignature(txSigner, sig)
		if err != nil {
			t.Fatal(err)
		}
		from, err := types.Sender(txSigner, signed)
		if err != nil || from != common.HexToAddress(imported[0].Address) {
			t.Fatalf("%s: invalid transaction signature: sender %s, error %v", result.NodeID, from, err)
		}
	}
	for _, id := range []string{"node-1", "node-3"} {
		registry := registries[id]
		for _, name := range []string{"attestor_sessions_started_total", "attestor_sessions_completed_total"} {
			metric := gatheredMetric(t, registry, name, "protocol", protocolSignSecp)
			if metric.Counter == nil || metric.GetCounter().GetValue() != 1 {
				t.Fatalf("%s: %s = %v, want 1", id, name, metric)
			}
		}
		metric := gatheredMetric(t, registry, "attestor_signing_latency_seconds", "protocol", protocolSignSecp)
		if metric.Histogram == nil || metric.GetHistogram().GetSampleCount() != 1 || metric.GetHistogram().GetSampleSum() <= 0 {
			t.Fatalf("%s: signing latency = %v, want one positive observation", id, metric)
		}
	}
	if c.byID("node-5")[0].server.opts.Metrics != nil {
		t.Fatal("expected signing participant without metrics")
	}
}

func TestRefusalRecordsReasonCode(t *testing.T) {
	registries := make(map[string]*prometheus.Registry)
	c := newTestClusterWith(t, 5, true, testPolicy(), metricOptions(t, registries))
	const keyID = "metrics-refused"
	importMetricsKey(t, c, keyID)
	_, raw := metricsTransaction(t, new(big.Int).Exp(big.NewInt(10), big.NewInt(19), nil))
	node := c.nodes[0]
	for i, direct := range []bool{false, true} {
		body, err := json.Marshal(SignRequest{
			SessionID: []string{"refuse-route", "refuse-direct"}[i], KeyID: keyID,
			Kind: KindEVMTransaction, Signers: []string{"node-1", "node-2", "node-3"}, Transaction: raw,
		})
		if err != nil {
			t.Fatal(err)
		}
		r := verifiedRequest(c.ca.cert, http.MethodPost, PathSign, body)
		r.Header.Set("Authorization", "Bearer "+c.idp.mint(t, c.idp.key, testOwner))
		w := httptest.NewRecorder()
		if direct {
			node.server.HandleSign(w, r)
		} else {
			node.server.Handler().ServeHTTP(w, r)
		}
		if w.Code != http.StatusForbidden {
			t.Fatalf("policy refusal status = %d, body %s", w.Code, w.Body)
		}
		e := expectError(t, "metrics policy refusal", apiResult{status: w.Code, body: w.Body.Bytes()}, CodePolicyDenied)
		if e.PolicyCode != policy.CodeValueCap {
			t.Fatalf("policy code = %s, want %s", e.PolicyCode, policy.CodeValueCap)
		}
		metric := gatheredMetric(t, registries[node.id], "attestor_refusals_total", "reason", CodePolicyDenied)
		if metric.Counter == nil || metric.GetCounter().GetValue() != float64(i+1) {
			t.Fatalf("policy refusal count = %v, want %d", metric, i+1)
		}
	}

	w := httptest.NewRecorder()
	node.server.Handler().ServeHTTP(w, verifiedRequest(c.ca.cert, http.MethodGet, PathSign, nil))
	expectError(t, "method refusal", apiResult{status: w.Code, body: w.Body.Bytes()}, CodeSessionBadRequest)
	metric := gatheredMetric(t, registries[node.id], "attestor_refusals_total", "reason", CodeSessionBadRequest)
	if metric.Counter == nil || metric.GetCounter().GetValue() != 1 {
		t.Fatalf("method refusal count = %v, want 1", metric)
	}

	w = httptest.NewRecorder()
	node.server.Handler().ServeHTTP(w, verifiedRequest(c.ca.cert, http.MethodPost, PathAuthority, []byte(`{"unexpected":true}`)))
	expectError(t, "authority handler refusal", apiResult{status: w.Code, body: w.Body.Bytes()}, CodeSessionBadRequest)
	metric = gatheredMetric(t, registries[node.id], "attestor_refusals_total", "reason", CodeSessionBadRequest)
	if metric.Counter == nil || metric.GetCounter().GetValue() != 2 {
		t.Fatalf("handler refusal count = %v, want 2", metric)
	}
}
