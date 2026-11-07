package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/audit"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy"
)

func evmReviewTransaction(t *testing.T, tx *types.Transaction) string {
	t.Helper()
	raw, err := tx.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return "0x" + hex.EncodeToString(raw)
}

func TestEVMReviewDisclosesDecodedCall(t *testing.T) {
	tokenAddress := common.HexToAddress("0x2222222222222222222222222222222222222222")
	recipient := common.HexToAddress("0x3333333333333333333333333333333333333333")
	doc := testPolicy()
	doc.Defaults.Caps[tokenAddress.Hex()] = policy.Cap{PerTransaction: "1000", Daily: "1000"}
	registries := make(map[string]*prometheus.Registry)
	c := newTestClusterWith(t, 5, true, doc, metricOptions(t, registries))
	const keyID = "review-token"
	importMetricsKey(t, c, keyID)
	data := append(common.FromHex("0xa9059cbb"), common.LeftPadBytes(recipient.Bytes(), 32)...)
	data = append(data, common.LeftPadBytes(big.NewInt(600).Bytes(), 32)...)
	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID: big.NewInt(testChainID), Nonce: 7, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(2),
		Gas: 80000, To: &tokenAddress, Value: big.NewInt(0), Data: data,
	})
	req := EVMReviewRequest{SessionID: "review-erc20", KeyID: keyID, Transaction: evmReviewTransaction(t, tx)}
	node := c.nodes[0]
	before, err := node.store.Get(keyID)
	if err != nil {
		t.Fatal(err)
	}
	var shareHash [32]byte
	if err := node.store.WithShare(keyID, func(plain []byte) error {
		shareHash = sha256.Sum256(plain)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sequence, _ := node.audit.Head()
	result := c.call(t, node, PathEVMReview, req, c.idp.mint(t, c.idp.key, testOwner))
	response := decodeOK[EVMReviewResponse](t, "ERC-20 review", []apiResult{result})[0]
	wantHash := types.LatestSignerForChainID(big.NewInt(testChainID)).Hash(tx)
	if response.NodeID != node.id || response.KeyID != keyID || response.To != tokenAddress.Hex() ||
		response.Value != "0" || response.Data != "0x"+hex.EncodeToString(data) || response.Method != "transfer" ||
		response.Destination != "erc20" || response.SigningHash != wantHash.Hex() ||
		response.ChainID != testChainID || response.Decision != policy.CodeAllowed {
		t.Fatalf("decoded review = %+v, want transfer to %s with digest %s", response, tokenAddress, wantHash)
	}
	signed, _, _, e := node.server.prepare(SignRequest{Kind: KindEVMTransaction, Transaction: req.Transaction}, before.PublicKey, testOwner)
	if e != nil || !bytes.Equal(signed, wantHash.Bytes()) || response.SigningHash != "0x"+hex.EncodeToString(signed) {
		t.Fatalf("review hash %s differs from signing digest %x: %v", response.SigningHash, signed, e)
	}
	afterSequence, _ := node.audit.Head()
	if response.AuditSequence != sequence+1 || afterSequence != sequence+1 {
		t.Fatalf("review wrote %d audit records, response sequence %d", afterSequence-sequence, response.AuditSequence)
	}
	logged, err := os.ReadFile(filepath.Join(c.configs[node.index].nodeDir, "audit", audit.FileName))
	if err != nil || bytes.Count(logged, []byte("evm_review")) != 1 {
		t.Fatalf("expected one evm_review audit record: %v", err)
	}
	if err := node.audit.Verify(); err != nil {
		t.Fatal(err)
	}
	after, err := node.store.Get(keyID)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("review changed the held share: %v", err)
	}
	if err := node.store.WithShare(keyID, func(plain []byte) error {
		if sha256.Sum256(plain) != shareHash {
			t.Fatal("review changed the private share payload")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for id, registry := range registries {
		families, err := registry.Gather()
		if err != nil {
			t.Fatal(err)
		}
		for _, family := range families {
			if family.GetName() == "attestor_sessions_started_total" {
				for _, metric := range family.Metric {
					if metric.GetCounter().GetValue() != 0 {
						t.Fatalf("review started a protocol session on %s", id)
					}
				}
			}
		}
	}

	t.Run("requires gateway and owner authentication", func(t *testing.T) {
		expectError(t, "missing token", c.call(t, node, PathEVMReview, req, ""), CodeTokenMissing)
		expectError(t, "invalid token", c.call(t, node, PathEVMReview, req, "invalid"), CodeTokenInvalid)
		expectError(t, "wrong owner", c.call(t, node, PathEVMReview, req, c.idp.mint(t, c.idp.key, "someone-else")), CodeTokenNotOwner)
		expectError(t, "operator certificate", c.callAs(t, c.opClient, node, PathEVMReview, req, c.idp.mint(t, c.idp.key, testOwner)), CodeOperatorRequired)
	})
	t.Run("policy refusal carries the decision code", func(t *testing.T) {
		overCap := types.NewTx(&types.DynamicFeeTx{
			ChainID: big.NewInt(testChainID), GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(2),
			Gas: 21000, To: &recipient, Value: big.NewInt(2_000_000_000_000_000_000),
		})
		denied := c.call(t, node, PathEVMReview, EVMReviewRequest{SessionID: "review-over-cap", KeyID: keyID, Transaction: evmReviewTransaction(t, overCap)}, c.idp.mint(t, c.idp.key, testOwner))
		if denied.status != http.StatusForbidden {
			t.Fatalf("policy refusal status = %d, body %s", denied.status, denied.body)
		}
		if e := expectError(t, "over cap", denied, CodePolicyDenied); e.PolicyCode != policy.CodeValueCap {
			t.Fatalf("policy code = %s, want %s", e.PolicyCode, policy.CodeValueCap)
		}
	})
}

func TestEVMReviewRecordsNoSpend(t *testing.T) {
	doc := testPolicy()
	doc.Defaults.Caps[policy.AssetNative] = policy.Cap{PerTransaction: "1000000000000000000", Daily: "1000000000000000000"}
	c := newTestClusterWith(t, 5, true, doc, nil)
	const keyID = "review-native"
	imported, authority := importMetricsKey(t, c, keyID)
	epoch := uint64(0)
	decodeOK[KeyResponse](t, "refresh review key", c.callAll(t, c.nodes, PathRefresh, func(*testNode) any {
		return RefreshRequest{SessionID: "review-refresh", KeyID: keyID, CeremonyID: "metrics-ceremony", ImportSessionID: "import-" + keyID, ExpectedEpoch: &epoch}
	}, ""))
	authority.keys[0].Epoch = 1
	authority.publish(t)
	amount := big.NewInt(600_000_000_000_000_000)
	tx, raw := metricsTransaction(t, amount)
	signers := []string{"node-1", "node-3", "node-5"}
	nodes := c.byID(signers...)
	account := policy.AccountKey(imported[0].Address)
	for _, session := range []string{"review-native-first", "review-native-second"} {
		answers := decodeOK[EVMReviewResponse](t, session, c.callAll(t, nodes, PathEVMReview, func(*testNode) any {
			return EVMReviewRequest{SessionID: session, KeyID: keyID, Transaction: raw}
		}, c.idp.mint(t, c.idp.key, testOwner)))
		for _, answer := range answers {
			if answer.Decision != policy.CodeAllowed || answer.Value != amount.String() || answer.Data != "0x" {
				t.Fatalf("native review = %+v", answer)
			}
		}
	}
	for _, node := range c.nodes {
		spent, err := node.server.spends.Spent(account, policy.AssetNative, time.Now().Add(-policy.SpendWindow))
		if err != nil || spent.Sign() != 0 {
			t.Fatalf("%s: reviews spent %v: %v", node.id, spent, err)
		}
		requests, err := node.server.spends.Requests(account, time.Now().Add(-policy.RateWindow))
		if err != nil || requests != 0 {
			t.Fatalf("%s: reviews recorded %d requests: %v", node.id, requests, err)
		}
	}
	results := decodeOK[SignResponse](t, "sign after two reviews", c.callAll(t, nodes, PathSign, func(*testNode) any {
		return SignRequest{SessionID: "sign-after-reviews", KeyID: keyID, Kind: KindEVMTransaction, Signers: signers, Transaction: raw}
	}, c.idp.mint(t, c.idp.key, testOwner)))
	txSigner := types.LatestSignerForChainID(big.NewInt(testChainID))
	for _, result := range results {
		if result.SignedBytes != hex.EncodeToString(txSigner.Hash(tx).Bytes()) {
			t.Fatalf("%s: signed a different digest: %s", result.NodeID, result.SignedBytes)
		}
		signature, err := hex.DecodeString(result.Signature)
		if err != nil {
			t.Fatal(err)
		}
		signed, err := tx.WithSignature(txSigner, signature)
		if err != nil {
			t.Fatal(err)
		}
		sender, err := types.Sender(txSigner, signed)
		if err != nil || sender != common.HexToAddress(imported[0].Address) {
			t.Fatalf("%s: invalid signature, sender %s: %v", result.NodeID, sender, err)
		}
	}
	for _, node := range c.nodes {
		spent, err := node.server.spends.Spent(account, policy.AssetNative, time.Now().Add(-policy.SpendWindow))
		if err != nil || spent.Cmp(amount) != 0 {
			t.Fatalf("%s: spent %v after signing, want %s: %v", node.id, spent, amount, err)
		}
	}
	denied := c.call(t, nodes[0], PathEVMReview, EVMReviewRequest{SessionID: "review-after-signing", KeyID: keyID, Transaction: raw}, c.idp.mint(t, c.idp.key, testOwner))
	if denied.status != http.StatusForbidden {
		t.Fatalf("review exceeding the remaining cap: status %d body %s", denied.status, denied.body)
	}
	if e := expectError(t, "review reads existing spend", denied, CodePolicyDenied); e.PolicyCode != policy.CodeDailyCap {
		t.Fatalf("policy code = %s, want %s", e.PolicyCode, policy.CodeDailyCap)
	}
}

func TestEVMReviewRefusesWrongChain(t *testing.T) {
	c := newTestCluster(t, 5, true)
	const keyID = "review-chain"
	importMetricsKey(t, c, keyID)
	to := common.HexToAddress("0x1111111111111111111111111111111111111111")
	foreignChain := big.NewInt(testChainID + 1)
	transactions := []*types.Transaction{
		types.NewTx(&types.DynamicFeeTx{ChainID: foreignChain, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(2), Gas: 21000, To: &to, Value: big.NewInt(1)}),
		types.NewTx(&types.AccessListTx{ChainID: foreignChain, GasPrice: big.NewInt(1), Gas: 21000, To: &to, Value: big.NewInt(1)}),
		types.NewTx(&types.LegacyTx{V: foreignChain, GasPrice: big.NewInt(1), Gas: 21000, To: &to, Value: big.NewInt(1)}),
	}
	for _, tx := range transactions {
		node := c.nodes[0]
		sequence, head := node.audit.Head()
		req := EVMReviewRequest{SessionID: "review-foreign-chain", KeyID: keyID, Transaction: evmReviewTransaction(t, tx)}
		result := c.call(t, node, PathEVMReview, req, c.idp.mint(t, c.idp.key, testOwner))
		if result.status != http.StatusForbidden {
			t.Fatalf("type %d: wrong chain status = %d, body %s", tx.Type(), result.status, result.body)
		}
		if e := expectError(t, "wrong chain", result, CodePolicyDenied); e.PolicyCode != policy.CodeChainMismatch {
			t.Fatalf("type %d: policy code = %s, want %s", tx.Type(), e.PolicyCode, policy.CodeChainMismatch)
		}
		if afterSequence, afterHead := node.audit.Head(); afterSequence != sequence || afterHead != head {
			t.Fatalf("type %d: wrong chain wrote an audit record", tx.Type())
		}
	}
}
