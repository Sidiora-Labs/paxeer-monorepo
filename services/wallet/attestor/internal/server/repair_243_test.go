package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/transport"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/tss/dealershare"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

func repair243Totals(t *testing.T, c *testCluster, account string, amount int64, requests int) {
	t.Helper()
	for _, node := range c.nodes {
		ledger := node.server.spends
		now, err := ledger.Now()
		if err != nil {
			t.Fatal(err)
		}
		spent, err := ledger.Spent(policy.AccountKey(account), policy.AssetNative, now.Add(-policy.SpendWindow))
		if err != nil {
			t.Fatal(err)
		}
		count, err := ledger.Requests(policy.AccountKey(account), now.Add(-policy.RateWindow))
		if err != nil || spent.Cmp(big.NewInt(amount)) != 0 || count != requests {
			t.Fatalf("%s: durable totals = %s / %d (%v), want %d / %d", node.id, spent, count, err, amount, requests)
		}
	}
}

func TestRepair_243_RestartRecovery(t *testing.T) {
	doc := testPolicy()
	doc.Defaults.Caps[policy.AssetNative] = policy.Cap{PerTransaction: "1000", Daily: "2000"}
	// A fixed real ledger clock keeps the rate-window assertion meaningful while
	// the test waits for actual transport expiry and performs a full restart.
	ledgerTime := time.Now()
	c := newTestClusterWith(t, 5, true, doc, func(o *Options) {
		ledger, err := policy.NewSpendLedger(o.Store, func() time.Time { return ledgerTime })
		if err != nil {
			t.Fatal(err)
		}
		o.Ledger = ledger
	})
	seed := sha256.Sum256([]byte("repair 243 immutable EVM signing intent"))
	key, err := gethcrypto.ToECDSA(seed[:])
	if err != nil {
		t.Fatal(err)
	}
	account := gethcrypto.PubkeyToAddress(key.PublicKey).Hex()
	const keyID = "repair-243-evm"
	const session = "repair-243-original"
	importKey(t, c, keyID, dealer.Secp256k1, new(big.Int).SetBytes(seed[:]), "")
	decodeOK[KeyResponse](t, "refresh EVM shares", c.callAll(t, c.nodes, PathRefresh, func(*testNode) any {
		return RefreshRequest{SessionID: "repair-243-refresh", KeyID: keyID, CeremonyID: "metrics-ceremony", ImportSessionID: "import-" + keyID, ExpectedEpoch: new(uint64)}
	}, ""))
	c.publishImportEpoch(t, keyID, 1)
	signers := []string{"node-1", "node-2", "node-3"}
	to := common.HexToAddress("0x1111111111111111111111111111111111111111")
	other := common.HexToAddress("0x2222222222222222222222222222222222222222")
	makeRequest := func(id string, nonce uint64, recipient common.Address) (SignRequest, *types.Transaction) {
		tx := types.NewTx(&types.DynamicFeeTx{ChainID: big.NewInt(testChainID), Nonce: nonce, GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(2), Gas: 21000, To: &recipient, Value: big.NewInt(1000)})
		raw, err := tx.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		return SignRequest{SessionID: id, KeyID: keyID, Kind: KindEVMTransaction, Signers: signers, Transaction: hex.EncodeToString(raw)}, tx
	}
	request, tx := makeRequest(session, 0, to)
	originalToken := c.idp.mint(t, c.idp.key, testOwner)
	sign := func(req SignRequest, token string) []apiResult {
		return c.callAll(t, c.byID(signers...), PathSign, func(*testNode) any { return req }, token)
	}
	responses := decodeOK[SignResponse](t, "first cap-sized transaction", sign(request, originalToken))
	verify := func(tx *types.Transaction, responses []SignResponse) {
		t.Helper()
		for _, response := range responses {
			sig, err := hex.DecodeString(response.Signature)
			if err != nil {
				t.Fatal(err)
			}
			signer := types.LatestSignerForChainID(big.NewInt(testChainID))
			signed, err := tx.WithSignature(signer, sig)
			if err != nil {
				t.Fatal(err)
			}
			from, err := types.Sender(signer, signed)
			if err != nil || from.Hex() != account {
				t.Fatalf("%s: invalid real threshold signature: %s %v", response.NodeID, from, err)
			}
		}
	}
	verify(tx, responses)
	repair243Totals(t, c, account, 1000, 1)
	expected := signingIntent(request, 1, types.LatestSignerForChainID(big.NewInt(testChainID)).Hash(tx).Bytes())
	for _, node := range c.nodes {
		state, err := node.server.loadSignIntent(keyID, session)
		if err != nil || state.Binding != expected {
			t.Fatalf("%s: producer intent not durably received: %+v %v", node.id, state, err)
		}
		if contains(signers, node.id) != (state.Response != nil) {
			t.Fatalf("%s: completed result differs from actual signer membership", node.id)
		}
	}
	assertConflicts := func(label string) {
		t.Helper()
		for _, change := range []struct {
			name  string
			nonce uint64
			to    common.Address
		}{{"nonce", 1, to}, {"recipient", 0, other}, {"both", 1, other}} {
			changed, _ := makeRequest(session, change.nonce, change.to)
			changed.Signers = c.ids
			for _, token := range []string{originalToken, c.idp.mint(t, c.idp.key, testOwner)} {
				// All nodes, including non-signers, must enforce the peer binding.
				for _, result := range c.callAll(t, c.nodes, PathSign, func(*testNode) any { return changed }, token) {
					expectError(t, label+" changed "+change.name, result, CodeSessionBadRequest)
				}
			}
		}
		repair243Totals(t, c, account, 1000, 1)
	}
	assertConflicts("before transport expiry")
	time.Sleep(transport.DefaultLimits().PendingTTL + time.Second)
	// Prove the old transport tombstone has actually expired by opening and
	// closing the original transport session without executing another signature.
	for _, node := range c.byID(signers...) {
		probe, err := node.server.opts.Transport.OpenKind(sessionName(session, "sign"), signers, "repair-243-expiry-probe", transport.KindSignSecp256k1)
		if err != nil {
			t.Fatalf("%s: transport tombstone has not expired: %v", node.id, err)
		}
		probe.Close()
	}
	// Fresh credentials permit the transport-independent retry and conflict path.
	for _, node := range c.nodes {
		changed, _ := makeRequest(session, 2, other)
		changed.Signers = c.ids
		expectError(t, "changed after transport expiry", c.call(t, node, PathSign, changed, c.idp.mint(t, c.idp.key, testOwner)), CodeSessionBadRequest)
	}
	for i, node := range c.byID(signers...) {
		got := decodeOK[SignResponse](t, "exact retry after transport expiry", []apiResult{c.call(t, node, PathSign, request, c.idp.mint(t, c.idp.key, testOwner))})[0]
		if !reflect.DeepEqual(got, responses[i]) {
			t.Fatalf("%s: expired transport changed durable result", node.id)
		}
	}
	repair243Totals(t, c, account, 1000, 1)
	for _, node := range c.nodes {
		node.stop()
	}
	for _, node := range append([]*testNode(nil), c.nodes...) {
		c.restart(t, node)
	}
	repair243Totals(t, c, account, 1000, 1)
	for _, node := range c.nodes {
		changed, _ := makeRequest(session, 3, other)
		changed.Signers = c.ids
		for _, token := range []string{originalToken, c.idp.mint(t, c.idp.key, testOwner)} {
			expectError(t, "changed after quorum restart", c.call(t, node, PathSign, changed, token), CodeSessionBadRequest)
		}
	}
	for i, node := range c.byID(signers...) {
		// A single recovered signer returns its exact durable result without TSS.
		retry := request
		retry.Transaction = "0x" + retry.Transaction
		retry.Signers = []string{"node-3", "node-2", "node-1"}
		got := decodeOK[SignResponse](t, "canonical exact retry after restart", []apiResult{c.call(t, node, PathSign, retry, c.idp.mint(t, c.idp.key, testOwner))})[0]
		if !reflect.DeepEqual(got, responses[i]) {
			t.Fatalf("%s: retry changed durable result: %+v != %+v", node.id, got, responses[i])
		}
	}
	repair243Totals(t, c, account, 1000, 1)
	second, tx2 := makeRequest("repair-243-distinct", 1, other)
	verify(tx2, decodeOK[SignResponse](t, "distinct transaction consumes another budget", sign(second, c.idp.mint(t, c.idp.key, testOwner))))
	repair243Totals(t, c, account, 2000, 2)
	third, _ := makeRequest("repair-243-over-daily", 2, to)
	expectPolicyCode(t, "third transaction exceeds daily cap", sign(third, c.idp.mint(t, c.idp.key, testOwner)), policy.CodeDailyCap)
	// Even a policy refusal reserves its immutable intent before evaluating caps.
	for _, node := range c.byID(signers...) {
		state, err := node.server.loadSignIntent(keyID, third.SessionID)
		if err != nil || state.Binding.KeyID != keyID || state.Response != nil {
			t.Fatalf("%s: policy evaluated before durable binding: %+v %v", node.id, state, err)
		}
	}
}

func TestRepair_243_RequestRateAndPeerBinding(t *testing.T) {
	doc := testPolicy()
	rate := uint32(2)
	doc.Defaults.RatePerMinute = &rate
	c := newTestClusterWith(t, 5, true, doc, nil)
	pub, account := importEd25519(t, c, "repair-243-rate", "repair 243 durable rate key")
	signers := []string{"node-1", "node-2", "node-3"}
	for _, session := range []string{"repair-243-rate-first", "repair-243-rate-second"} {
		verifyBind(t, "distinct sign consumes request rate", pub, account, signBind(t, c, "repair-243-rate", session, signers, account, c.idp.mint(t, c.idp.key, testOwner)))
	}
	for _, node := range c.nodes {
		node.stop()
	}
	for _, node := range append([]*testNode(nil), c.nodes...) {
		c.restart(t, node)
	}
	repair243Totals(t, c, account, 0, 2)
	expectPolicyCode(t, "third request exceeds recovered request rate", signBind(t, c, "repair-243-rate", "repair-243-rate-third", []string{"node-3", "node-4", "node-5"}, account, c.idp.mint(t, c.idp.key, testOwner)), policy.CodeRateLimited)
	sender := c.nodes[0]
	receiver := c.nodes[4]
	state, err := sender.server.loadSignIntent("repair-243-rate", "repair-243-rate-first")
	if err != nil {
		t.Fatal(err)
	}
	// Exercise the real announcement producer, mTLS transport and consumer again.
	if acked, silent := sender.server.Announce(context.Background(), state.Binding.KeyID, state.Binding.SessionID, account, c.ids, nil); acked != len(c.ids) {
		t.Fatalf("bound announcement replay: acked %d, silent %v", acked, silent)
	}
	// Negative wire mutations supplement that producer-to-consumer proof.
	for _, mutate := range []func(*Announcement){
		func(a *Announcement) { a.Intent.Digest = hex.EncodeToString(make([]byte, 32)) },
		func(a *Announcement) { a.Intent.Epoch++ },
		func(a *Announcement) { a.Intent.SessionID = "different-session" },
		func(a *Announcement) { a.Intent.KeyID = "different-key" },
		func(a *Announcement) { a.Intent.Kind = KindPersonalMessage },
		func(a *Announcement) { a.Intent = nil },
	} {
		binding := state.Binding
		a := Announcement{SessionID: binding.SessionID, KeyID: binding.KeyID, Account: account, Intent: &binding, Spends: []SpendJSON{{Asset: policy.AssetNative, Amount: "999"}}}
		mutate(&a)
		body, err := json.Marshal(a)
		if err != nil {
			t.Fatal(err)
		}
		status, raw, err := sender.server.opts.Transport.Request(context.Background(), receiver.id, PathPeerAnnounce, body)
		if err != nil || status != http.StatusBadRequest {
			t.Fatalf("conflicting peer intent accepted: status %d, %s, %v", status, raw, err)
		}
		expectError(t, "conflicting peer intent", apiResult{status: status, body: raw}, CodeSessionBadRequest)
	}
	repair243Totals(t, c, account, 0, 2)
}

func TestRepair_243_AtomicBindingRecovery(t *testing.T) {
	dir := t.TempDir()
	nodeKey := sha256.Sum256([]byte("repair 243 real encrypted store key"))
	db, err := store.Open(dir, nodeKey[:])
	if err != nil {
		t.Fatal(err)
	}
	srv := &Server{opts: Options{Store: db}}
	req := SignRequest{KeyID: "repair-243-atomic", SessionID: "same-accounting-identity", Kind: KindEVMTransaction}
	intents := []signIntentBinding{signingIntent(req, 4, []byte("first signing digest")), signingIntent(req, 4, []byte("second signing digest"))}
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan signIntentBinding, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(intent signIntentBinding) {
			defer wg.Done()
			<-start
			state, e := srv.bindSignIntent(intent)
			if e == nil {
				results <- state.Binding
			} else if e.Code != CodeSessionBadRequest {
				t.Errorf("unexpected durable binding error: %v", e)
			}
		}(intents[i%2])
	}
	close(start)
	wg.Wait()
	close(results)
	winner, err := srv.loadSignIntent(req.KeyID, req.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	accepted := 0
	for binding := range results {
		accepted++
		if binding != winner.Binding {
			t.Fatal("two different signing intents acquired the same accounting identity")
		}
	}
	if accepted != 8 {
		t.Fatalf("%d bindings accepted, want every exact retry of the sole winner (8)", accepted)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = store.Open(dir, nodeKey[:])
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	srv = &Server{opts: Options{Store: db}}
	for _, intent := range intents {
		state, e := srv.bindSignIntent(intent)
		if intent == winner.Binding {
			if e != nil || state.Binding != winner.Binding || state.Response != nil {
				t.Fatalf("pending intent lost after reopen: %+v %v", state, e)
			}
		} else if e == nil || e.Code != CodeSessionBadRequest {
			t.Fatal("conflicting intent admitted after reopen", e)
		}
	}
	newEpoch := winner.Binding
	newEpoch.Epoch++
	if _, e := srv.bindSignIntent(newEpoch); e == nil || e.Code != CodeSessionBadRequest {
		t.Fatal("another epoch reused an existing accounting identity", e)
	}
}
