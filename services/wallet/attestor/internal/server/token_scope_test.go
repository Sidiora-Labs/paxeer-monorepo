package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/audit"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/lxwire"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/transport"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/tss/dealer"
)

func replayAudits(t *testing.T, c *testCluster, node *testNode) int {
	t.Helper()
	logged, err := os.ReadFile(filepath.Join(c.configs[node.index].nodeDir, "audit", audit.FileName))
	if err != nil {
		t.Fatalf("%s: read audit log: %v", node.id, err)
	}
	return bytes.Count(logged, []byte(auditReasonTokenReplayed))
}

func expectReplay(t *testing.T, label string, r apiResult) {
	t.Helper()
	e := expectError(t, label, r, CodeTokenInvalid)
	if !strings.Contains(e.Message, "already authorised") {
		t.Fatalf("%s: %s", label, e.Message)
	}
}

func TestOneTokenAuthorisesDistinctRequestsAndRefusesARepeat(t *testing.T) {
	c := newTestCluster(t, 5, true)
	pub, account := importEd25519(t, c, "lx-key", "attestor request-scoped token ed25519 key")
	signers := []string{"node-1", "node-2", "node-3", "node-4", "node-5"}
	token := c.idp.mint(t, c.idp.key, testOwner)

	verifyBind(t, "first request under the token", pub, account, signBind(t, c, "lx-key", "token-scope-first", signers, account, token))
	verifyBind(t, "second request under the same token", pub, account, signBind(t, c, "lx-key", "token-scope-second", signers, account, token))

	for i, r := range signBind(t, c, "lx-key", "token-scope-first", signers, account, token) {
		expectReplay(t, "the first request repeated on "+signers[i], r)
	}
	for _, node := range c.nodes {
		if n := replayAudits(t, c, node); n != 1 {
			t.Fatalf("%s: %d audit entries name the replay, want 1", node.id, n)
		}
	}

	restarted := c.restart(t, c.byID("node-3")[0])
	bind := lxwire.BindMessage(testChainID, common.HexToAddress(account), 1)
	for _, session := range []string{"token-scope-first", "token-scope-second"} {
		r := c.call(t, restarted, PathSign, SignRequest{SessionID: session, KeyID: "lx-key", Kind: KindLXBind, Signers: signers, Message: hex.EncodeToString(bind)}, token)
		expectReplay(t, session+" repeated after the node restarted", r)
	}
	if n := replayAudits(t, c, restarted); n != 3 {
		t.Fatalf("%s: %d audit entries name the replay after restart, want 3", restarted.id, n)
	}

	verifyBind(t, "a third request under the same token after a restart", pub, account, signBind(t, c, "lx-key", "token-scope-third", signers, account, token))
}

func testChangedTransactionRetainsSessionIdentity(t *testing.T) {
	t.Helper()
	doc := testPolicy()
	doc.Defaults.Caps[policy.AssetNative] = policy.Cap{PerTransaction: "1000", Daily: "1000"}
	ledgerTime := time.Now()
	c := newTestClusterWith(t, 5, true, doc, func(o *Options) {
		ledger, err := policy.NewSpendLedger(o.Store, func() time.Time { return ledgerTime })
		if err != nil {
			t.Fatal(err)
		}
		o.Ledger = ledger
	})
	const keyID, session = "repair-m157-evm", "repair-m157-cap-sized"
	seed := sha256.Sum256([]byte("repair m157 session identity key"))
	key, err := gethcrypto.ToECDSA(seed[:])
	if err != nil {
		t.Fatal(err)
	}
	account := gethcrypto.PubkeyToAddress(key.PublicKey).Hex()
	importKey(t, c, keyID, dealer.Secp256k1, new(big.Int).SetBytes(seed[:]), "")
	decodeOK[KeyResponse](t, "refresh EVM shares", c.callAll(t, c.nodes, PathRefresh, func(*testNode) any {
		return RefreshRequest{SessionID: "repair-m157-refresh", KeyID: keyID, CeremonyID: "metrics-ceremony", ImportSessionID: "import-" + keyID, ExpectedEpoch: new(uint64)}
	}, ""))
	c.publishImportEpoch(t, keyID, 1)
	signers := []string{"node-1", "node-2", "node-3"}
	to := common.HexToAddress("0x1111111111111111111111111111111111111111")
	other := common.HexToAddress("0x2222222222222222222222222222222222222222")
	makeRequest := func(nonce uint64, recipient common.Address) (SignRequest, *types.Transaction) {
		tx := types.NewTx(&types.DynamicFeeTx{ChainID: big.NewInt(testChainID), Nonce: nonce, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(2), Gas: 21000, To: &recipient, Value: big.NewInt(1000)})
		raw, err := tx.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		return SignRequest{SessionID: session, KeyID: keyID, Kind: KindEVMTransaction, Signers: signers, Transaction: hex.EncodeToString(raw)}, tx
	}
	request, tx := makeRequest(0, to)
	originalToken := c.idp.mint(t, c.idp.key, testOwner)
	responses := decodeOK[SignResponse](t, "cap-sized EVM transfer", c.callAll(t, c.byID(signers...), PathSign, func(*testNode) any { return request }, originalToken))
	for _, response := range responses {
		signature, err := hex.DecodeString(response.Signature)
		if err != nil {
			t.Fatal(err)
		}
		signer := types.LatestSignerForChainID(big.NewInt(testChainID))
		signed, err := tx.WithSignature(signer, signature)
		if err != nil {
			t.Fatal(err)
		}
		sender, err := types.Sender(signer, signed)
		if err != nil || sender.Hex() != account {
			t.Fatalf("%s: quorum signature recovered %s: %v", response.NodeID, sender, err)
		}
	}
	repair243Totals(t, c, account, 1000, 1)
	assertChangesRefused := func(label string, round int) {
		t.Helper()
		// Each stage uses unseen request bytes so even the original token must
		// reach durable intent validation rather than the token replay check.
		stageRecipient := common.BigToAddress(new(big.Int).Add(other.Big(), big.NewInt(int64(round))))
		for i, change := range []struct {
			nonce uint64
			to    common.Address
		}{{uint64(round + 1), to}, {0, stageRecipient}, {uint64(round + 1), stageRecipient}} {
			changed, _ := makeRequest(change.nonce, change.to)
			// Drive two different three-member quorums, including original non-signers.
			for _, quorum := range [][]string{{"node-1", "node-3", "node-5"}, {"node-2", "node-4", "node-5"}} {
				changed.Signers = quorum
				for _, token := range []string{originalToken, c.idp.mint(t, c.idp.key, testOwner)} {
					for _, result := range c.callAll(t, c.byID(quorum...), PathSign, func(*testNode) any { return changed }, token) {
						expectError(t, fmt.Sprintf("%s changed transaction %d", label, i), result, CodeSessionBadRequest)
					}
				}
			}
		}
		repair243Totals(t, c, account, 1000, 1)
	}
	assertChangesRefused("before expiry", 0)
	time.Sleep(transport.DefaultLimits().PendingTTL + time.Second)
	for _, node := range c.byID(signers...) {
		probe, err := node.server.opts.Transport.OpenKind(sessionName(session, "sign"), signers, "repair-m157-expiry-probe", transport.KindSignSecp256k1)
		if err != nil {
			t.Fatalf("%s: transport session did not expire: %v", node.id, err)
		}
		probe.Close()
	}
	assertChangesRefused("after transport expiry", 1)
	for _, node := range c.nodes {
		node.stop()
	}
	for _, node := range append([]*testNode(nil), c.nodes...) {
		c.restart(t, node)
	}
	repair243Totals(t, c, account, 1000, 1)
	assertChangesRefused("after full quorum restart", 2)
	repair243Totals(t, c, account, 1000, 1)
	for _, quorum := range [][]string{{"node-1", "node-2", "node-3"}, {"node-3", "node-4", "node-5"}} {
		next, _ := makeRequest(2, other)
		next.SessionID, next.Signers = "repair-m157-over-cap-"+quorum[0], quorum
		expectPolicyCode(t, "fresh session still exceeds the recovered cap", c.callAll(t, c.byID(quorum...), PathSign, func(*testNode) any { return next }, c.idp.mint(t, c.idp.key, testOwner)), policy.CodeDailyCap)
	}
	// Distinct refused requests count toward the rate limit, but never spend.
	for _, node := range c.nodes {
		now, err := node.server.spends.Now()
		if err != nil {
			t.Fatal(err)
		}
		spent, err := node.server.spends.Spent(policy.AccountKey(account), policy.AssetNative, now.Add(-policy.SpendWindow))
		if err != nil || spent.Cmp(big.NewInt(1000)) != 0 {
			t.Fatalf("%s: refused fresh request changed spend: %s (%v)", node.id, spent, err)
		}
		requests, err := node.server.spends.Requests(policy.AccountKey(account), now.Add(-policy.RateWindow))
		wantRequests := 2
		if node.id == "node-3" {
			wantRequests = 3
		}
		if err != nil || requests != wantRequests {
			t.Fatalf("%s: distinct refused requests counted %d (%v), want %d", node.id, requests, err, wantRequests)
		}
	}
}
