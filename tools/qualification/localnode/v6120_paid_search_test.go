package localnode

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	ledger "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const paidSearchLedger = "0x000000000000000000000000000000000000101a"

const paidSearchKernelNetworkID uint32 = 125

type paidSearchFixture struct {
	n          *followerNetwork
	client     *ethclient.Client
	issuer     *ecdsaPrivateKey
	receiver   *ecdsaPrivateKey
	buyer      *ecdsaPrivateKey
	asset      [32]byte
	price      *big.Int
	receiverID [32]byte
	buyerID    [32]byte
	config     string
	listen     string
	binary     string
	seed       *httptest.Server
}

// ecdsaPrivateKey keeps the fixture fields readable while retaining the concrete
// key type required by the existing follower helpers.
type ecdsaPrivateKey = ecdsa.PrivateKey

func TestLocalNodeV6120PaidSearch(t *testing.T) {
	n := newFollowerNetwork(t)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	waitForKernelNetworkID(t, n.primary, paidSearchKernelNetworkID)

	fixture := setupPaidSearch(t, n)
	defer fixture.client.Close()
	defer fixture.seed.Close()

	server := startPaidSearch(t, ctx, fixture)
	defer server.stop(t)

	t.Run("unpaid_search_offers_receiver_asset", func(t *testing.T) {
		response := paidSearchRequest(t, server.url, "/search?q=layerx", nil)
		if response.StatusCode != http.StatusPaymentRequired {
			t.Fatalf("unpaid search status = %d, want 402; body=%s", response.StatusCode, response.Body)
		}
		var required struct {
			Accepts []struct {
				Scheme string `json:"scheme"`
				Asset  string `json:"asset"`
				Amount string `json:"amount"`
				PayTo  string `json:"payTo"`
			} `json:"accepts"`
		}
		decodeHeaderJSON(t, response.Headers.Get("PAYMENT-REQUIRED"), &required)
		var offer *struct {
			Scheme string `json:"scheme"`
			Asset  string `json:"asset"`
			Amount string `json:"amount"`
			PayTo  string `json:"payTo"`
		}
		for index := range required.Accepts {
			if required.Accepts[index].Scheme == "exact" && sameHex(required.Accepts[index].Asset, fixture.asset[:]) {
				offer = &required.Accepts[index]
				break
			}
		}
		if offer == nil {
			t.Fatalf("payment offer does not include the registered asset: %+v", required.Accepts)
		}
		if offer.Amount != fixture.price.String() {
			t.Fatalf("offer amount = %q, want %s", offer.Amount, fixture.price)
		}
		if !sameHex(offer.PayTo, fixture.receiverID[:]) {
			t.Fatalf("offer payTo = %q, want receiver account 0x%x", offer.PayTo, fixture.receiverID)
		}
	})

	var paymentHeader string
	var paymentKey [32]byte
	t.Run("pay_exact_releases_search_and_moves_balances", func(t *testing.T) {
		challenge := paidSearchRequest(t, server.url, "/search?q=layerx", nil)
		var required map[string]interface{}
		decodeHeaderJSON(t, challenge.Headers.Get("PAYMENT-REQUIRED"), &required)
		accepts, ok := required["accepts"].([]interface{})
		if !ok {
			t.Fatalf("payment offer accepts = %#v", required["accepts"])
		}
		var accepted map[string]interface{}
		for _, candidate := range accepts {
			offer, ok := candidate.(map[string]interface{})
			asset, assetOK := offer["asset"].(string)
			if ok && assetOK && offer["scheme"] == "exact" && sameHex(asset, fixture.asset[:]) {
				accepted = offer
				break
			}
		}
		if accepted == nil {
			t.Fatal("exact offer for registered asset is missing")
		}
		paymentKey = crypto.Keccak256Hash([]byte("paid-search-layerx"))
		payData, err := ledger.GetABI().Pack("payExact", fixture.buyerID, fixture.receiverID, fixture.asset, fixture.price, uint64(time.Now().Add(2*time.Minute).UnixMilli()), paymentKey)
		if err != nil {
			t.Fatalf("pack payExact: %v", err)
		}
		payTx := fixture.n.sign(t, fixture.buyer, common.HexToAddress(paidSearchLedger), big.NewInt(0), payData)
		beforeBuyer := ledgerBalance(t, fixture.client, fixture.buyerID, fixture.asset)
		beforeReceiver := ledgerBalance(t, fixture.client, fixture.receiverID, fixture.asset)
		fixture.n.compare(t, payTx, types.ReceiptStatusSuccessful, true)
		afterBuyer := ledgerBalance(t, fixture.client, fixture.buyerID, fixture.asset)
		afterReceiver := ledgerBalance(t, fixture.client, fixture.receiverID, fixture.asset)
		if new(big.Int).Sub(beforeBuyer, afterBuyer).Cmp(fixture.price) != 0 {
			t.Fatalf("buyer balance moved from %s to %s, want decrease by %s", beforeBuyer, afterBuyer, fixture.price)
		}
		if new(big.Int).Sub(afterReceiver, beforeReceiver).Cmp(fixture.price) != 0 {
			t.Fatalf("receiver balance moved from %s to %s, want increase by %s", beforeReceiver, afterReceiver, fixture.price)
		}

		receipt := sequencerReceipt(t, fixture.client, payTx.Hash())
		if receipt == nil || (receipt.Rung != evmrpc.RungReceipted && receipt.Rung != evmrpc.RungCommitted) {
			t.Fatalf("payExact sequencer receipt = %#v, want receipted or committed", receipt)
		}
		if len(receipt.Receipt) == 0 {
			t.Fatal("payExact sequencer receipt has no signed receipt")
		}
		paymentHeader = exactPaymentHeader(t, accepted, payTx.Hash(), receipt.Receipt, paymentKey)
		response := paidSearchRequest(t, server.url, "/search?q=layerx", map[string]string{"PAYMENT-SIGNATURE": paymentHeader})
		if response.StatusCode != http.StatusOK {
			t.Fatalf("paid search status = %d, want 200; body=%s", response.StatusCode, response.Body)
		}
		if !strings.Contains(response.Body, `"query":"layerx"`) || response.Headers.Get("PAYMENT-RESPONSE") == "" {
			t.Fatalf("paid search response lacks successful content or payment response: headers=%v body=%s", response.Headers, response.Body)
		}
	})

	t.Run("same_payment_for_different_query_is_refused", func(t *testing.T) {
		response := paidSearchRequest(t, server.url, "/search?q=different", map[string]string{"PAYMENT-SIGNATURE": paymentHeader})
		if response.StatusCode == http.StatusOK {
			t.Fatalf("reused payment for a different query returned content: %s", response.Body)
		}
		if strings.Contains(response.Body, `"results"`) || response.Headers.Get("PAYMENT-RESPONSE") != "" {
			t.Fatalf("reused payment returned content or a successful payment response: status=%d body=%s", response.StatusCode, response.Body)
		}
	})

	t.Run("below_price_payment_is_refused", func(t *testing.T) {
		challenge := paidSearchRequest(t, server.url, "/search?q=below", nil)
		var required map[string]interface{}
		decodeHeaderJSON(t, challenge.Headers.Get("PAYMENT-REQUIRED"), &required)
		accepts, ok := required["accepts"].([]interface{})
		if !ok {
			t.Fatalf("payment offer accepts = %#v", required["accepts"])
		}
		var accepted map[string]interface{}
		for _, candidate := range accepts {
			offer, ok := candidate.(map[string]interface{})
			asset, assetOK := offer["asset"].(string)
			if ok && assetOK && offer["scheme"] == "exact" && sameHex(asset, fixture.asset[:]) {
				accepted = offer
				break
			}
		}
		if accepted == nil {
			t.Fatal("exact offer for registered asset is missing")
		}
		below := new(big.Int).Sub(fixture.price, big.NewInt(1))
		belowKey := crypto.Keccak256Hash([]byte("paid-search-below"))
		payData, err := ledger.GetABI().Pack("payExact", fixture.buyerID, fixture.receiverID, fixture.asset, below, uint64(time.Now().Add(2*time.Minute).UnixMilli()), belowKey)
		if err != nil {
			t.Fatalf("pack below-price payExact: %v", err)
		}
		payTx := fixture.n.sign(t, fixture.buyer, common.HexToAddress(paidSearchLedger), big.NewInt(0), payData)
		fixture.n.compare(t, payTx, types.ReceiptStatusSuccessful, true)
		receipt := sequencerReceipt(t, fixture.client, payTx.Hash())
		if receipt == nil || len(receipt.Receipt) == 0 {
			t.Fatal("below-price payExact has no signed sequencer receipt")
		}
		header := exactPaymentHeader(t, accepted, payTx.Hash(), receipt.Receipt, belowKey)
		response := paidSearchRequest(t, server.url, "/search?q=below", map[string]string{"PAYMENT-SIGNATURE": header})
		if response.StatusCode == http.StatusOK || strings.Contains(response.Body, `"results"`) {
			t.Fatalf("below-price payment released content: status=%d body=%s", response.StatusCode, response.Body)
		}
	})
}

func setupPaidSearch(t *testing.T, n *followerNetwork) *paidSearchFixture {
	t.Helper()
	binary := xWebsearchBinary(t)
	client := n.primary
	issuer := n.freshKey(t)
	n.bind(t, issuer)
	receiver := n.freshKey(t)
	buyer := n.freshKey(t)
	n.bind(t, buyer)

	issuerID := boundIdentity(t, client, crypto.PubkeyToAddress(issuer.PublicKey))
	var salt [32]byte
	copy(salt[:], crypto.Keccak256([]byte(fmt.Sprintf("local-paid-search-%d", time.Now().UnixNano()))))
	issuerActor := layerXDIDID(issuerID)
	assetHash := sha256.Sum256(append(append([]byte("LX:ASSET:v1"), issuerActor[:]...), salt[:]...))
	asset := assetHash
	price := big.NewInt(2)
	root := t.TempDir()
	configPath := filepath.Join(root, "config.json")
	receiverKeyPath := filepath.Join(root, "receiver.key")
	identityKeyPath := filepath.Join(root, "identity.key")
	identityPublic, identityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate receiver identity: %v", err)
	}
	if err := os.WriteFile(receiverKeyPath, []byte(fmt.Sprintf("%064x", receiver.D.Bytes())), 0600); err != nil {
		t.Fatalf("write receiver key: %v", err)
	}
	if err := os.WriteFile(identityKeyPath, []byte(hex.EncodeToString(identityPrivate.Seed())), 0600); err != nil {
		t.Fatalf("write receiver identity key: %v", err)
	}
	_ = identityPublic
	seed := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(writer, "<html><title>LayerX</title><body>paid search fixture</body></html>")
	}))
	t.Cleanup(seed.Close)
	keyInfo, err := os.ReadFile(filepath.Join(n.home, "receipt-public.json"))
	if err != nil {
		t.Fatalf("read local receipt key: %v", err)
	}
	var public struct {
		PublicKey string `json:"public_key"`
	}
	if err := json.Unmarshal(keyInfo, &public); err != nil || len(public.PublicKey) != ed25519.PublicKeySize*2 {
		t.Fatalf("decode local receipt key: %v", err)
	}
	publicBytes, err := hex.DecodeString(public.PublicKey)
	if err != nil {
		t.Fatalf("decode receipt public key: %v", err)
	}
	keyID := sha256.Sum256(publicBytes)
	listen := freeLoopbackAddress(t)
	other := [][32]byte{}
	for i := 1; i < 4; i++ {
		var id [32]byte
		copy(id[:], crypto.Keccak256([]byte(fmt.Sprintf("local-paid-search-other-%d", i))))
		other = append(other, id)
	}
	config := map[string]interface{}{
		"listen":                 listen,
		"data_dir":               filepath.Join(root, "data"),
		"seeds":                  []string{seed.URL + "/"},
		"crawl_interval_seconds": 1,
		"crawl": map[string]interface{}{
			"pages_per_cycle": 1, "pages_per_host": 1, "max_depth": 1, "politeness_delay_ms": 1,
		},
		"fetch": map[string]interface{}{
			"connect_timeout_ms": 100, "total_timeout_ms": 500, "max_body_bytes": 4096, "max_redirects": 0, "allow_loopback": true,
		},
		"assets": map[string]interface{}{
			"SID":  map[string]interface{}{"asset_id": "0x" + hex.EncodeToString(asset[:]), "price": price.String()},
			"PAX":  map[string]interface{}{"asset_id": "0x" + hex.EncodeToString(other[0][:]), "price": price.String()},
			"USDC": map[string]interface{}{"asset_id": "0x" + hex.EncodeToString(other[1][:]), "price": price.String()},
			"USDL": map[string]interface{}{"asset_id": "0x" + hex.EncodeToString(other[2][:]), "price": price.String()},
		},
		"gateway": map[string]interface{}{
			"endpoint": n.env("LOCAL_NODE_SEQUENCER_RPC"),
			"receipt_keys": []interface{}{map[string]interface{}{
				"key_id": "0x" + hex.EncodeToString(keyID[:]), "public_key": "0x" + public.PublicKey,
				"first_height": 1, "last_height": int64(^uint64(0) >> 1),
			}},
		},
		"evm":               map[string]interface{}{"endpoint": n.env("LOCAL_NODE_SEQUENCER_RPC"), "chain_id": 125, "confirmations": 1},
		"kernel_network_id": paidSearchKernelNetworkID,
		"peers":             []string{},
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("encode sidecar config: %v", err)
	}
	if err := os.WriteFile(configPath, encoded, 0600); err != nil {
		t.Fatalf("write sidecar config: %v", err)
	}
	bindOutput := runXWebsearch(t, binary, configPath, receiverKeyPath, "bind-receiver", "--identity-key-file", identityKeyPath)
	var binding struct {
		TransactionHash string `json:"transaction_hash"`
		Bound           bool   `json:"bound"`
	}
	if err := json.Unmarshal(bindOutput, &binding); err != nil {
		t.Fatalf("decode bind-receiver output: %v: %s", err, bindOutput)
	}
	if !binding.Bound || len(strings.TrimPrefix(binding.TransactionHash, "0x")) != 64 {
		t.Fatalf("bind-receiver result = %s", bindOutput)
	}
	boundTx, _, err := client.TransactionByHash(context.Background(), common.HexToHash(binding.TransactionHash))
	if err != nil {
		t.Fatalf("read bind-receiver transaction: %v", err)
	}
	n.compare(t, boundTx, types.ReceiptStatusSuccessful, false)

	ledgerAddress := common.HexToAddress(paidSearchLedger)
	register, err := ledger.GetABI().Pack("register", asset, salt, "SID", "Local paid search", uint8(6), big.NewInt(1000000), uint8(1), []byte{})
	if err != nil {
		t.Fatalf("pack register: %v", err)
	}
	n.compare(t, n.sign(t, issuer, ledgerAddress, big.NewInt(0), register), types.ReceiptStatusSuccessful, true)
	fixture := &paidSearchFixture{n: n, client: client, issuer: issuer, receiver: receiver, buyer: buyer, asset: asset, price: price, config: configPath, listen: listen, binary: binary, seed: seed}
	for name, key := range map[string]*ecdsa.PrivateKey{"issuer": issuer, "receiver": receiver, "buyer": buyer} {
		data, err := ledger.GetABI().Pack("openAccount", asset)
		if err != nil {
			t.Fatalf("pack %s openAccount: %v", name, err)
		}
		n.compare(t, n.sign(t, key, ledgerAddress, big.NewInt(0), data), types.ReceiptStatusSuccessful, true)
	}
	fixture.receiverID = ledgerAccountID(t, client, boundIdentity(t, client, crypto.PubkeyToAddress(receiver.PublicKey)), asset)
	fixture.buyerID = ledgerAccountID(t, client, boundIdentity(t, client, crypto.PubkeyToAddress(buyer.PublicKey)), asset)
	mint, err := ledger.GetABI().Pack("mint", asset, fixture.buyerID, big.NewInt(100))
	if err != nil {
		t.Fatalf("pack mint: %v", err)
	}
	n.compare(t, n.sign(t, issuer, ledgerAddress, big.NewInt(0), mint), types.ReceiptStatusSuccessful, true)
	return fixture
}

func startPaidSearch(t *testing.T, ctx context.Context, fixture *paidSearchFixture) *paidSearchServer {
	t.Helper()
	logPath := filepath.Join(filepath.Dir(fixture.config), "x-websearch.log")
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		t.Fatalf("open sidecar log: %v", err)
	}
	cmd := exec.CommandContext(ctx, fixture.binary, "--config", fixture.config)
	cmd.Env = append(os.Environ(), "X_WEBSEARCH_RECEIVER_KEY_FILE="+filepath.Join(filepath.Dir(fixture.config), "receiver.key"))
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		log.Close()
		t.Fatalf("start x-websearch: %v", err)
	}
	server := &paidSearchServer{cmd: cmd, log: log, url: "http://" + fixture.listen}
	t.Cleanup(func() { server.stop(t) })
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(server.url + "/readyz")
		if err == nil {
			var ready struct {
				Payment struct {
					State string `json:"state"`
				} `json:"payment"`
			}
			data, readErr := io.ReadAll(response.Body)
			response.Body.Close()
			if response.StatusCode == http.StatusOK && readErr == nil && json.Unmarshal(data, &ready) == nil && ready.Payment.State == "ready" {
				return server
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for x-websearch readiness: %v; see %s", ctx.Err(), logPath)
		case <-time.After(250 * time.Millisecond):
		}
	}
	t.Fatalf("x-websearch payment readiness did not become ready; see %s", logPath)
	return nil
}

func waitForKernelNetworkID(t *testing.T, client *ethclient.Client, want uint32) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	selector := crypto.Keccak256([]byte("getKernelNetworkId()"))[:4]
	address := common.HexToAddress("0x0000000000000000000000000000000000001019")
	var lastResult []byte
	var lastErr error
	for ctx.Err() == nil {
		lastResult, lastErr = client.CallContract(ctx, ethereum.CallMsg{To: &address, Data: selector}, nil)
		if lastErr == nil && len(lastResult) == 32 {
			got := new(big.Int).SetBytes(lastResult)
			if got.IsUint64() && got.Uint64() == uint64(want) {
				return
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("getKernelNetworkId did not return %d: result=%x error=%v", want, lastResult, lastErr)
}

type paidSearchServer struct {
	cmd *exec.Cmd
	log *os.File
	url string
}

func (s *paidSearchServer) stop(t *testing.T) {
	t.Helper()
	if s == nil || s.cmd == nil || s.cmd.Process == nil {
		return
	}
	if err := s.cmd.Process.Signal(os.Interrupt); err != nil && !strings.Contains(err.Error(), "already finished") {
		t.Logf("stop x-websearch: %v", err)
	}
	_ = s.cmd.Wait()
	_ = s.log.Close()
}

type paidSearchResponse struct {
	StatusCode int
	Headers    http.Header
	Body       string
}

func paidSearchRequest(t *testing.T, base, path string, headers map[string]string) paidSearchResponse {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, base+path, nil)
	if err != nil {
		t.Fatalf("create paid search request: %v", err)
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		t.Fatalf("paid search request %s: %v", path, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read paid search response: %v", err)
	}
	return paidSearchResponse{StatusCode: response.StatusCode, Headers: response.Header, Body: string(body)}
}

func decodeHeaderJSON(t *testing.T, value string, target interface{}) {
	t.Helper()
	if value == "" {
		t.Fatal("required payment header is missing")
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatalf("decode payment header: %v", err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("decode payment JSON: %v", err)
	}
}

func sameHex(value string, expected []byte) bool {
	return strings.EqualFold(strings.TrimPrefix(value, "0x"), hex.EncodeToString(expected))
}

func exactPaymentHeader(t *testing.T, accepted map[string]interface{}, hash common.Hash, signedReceipt []byte, key [32]byte) string {
	t.Helper()
	unsignedEnd := len(signedReceipt) - 69
	if unsignedEnd < 0 || len(signedReceipt) < 69 || !bytes.Equal(signedReceipt[unsignedEnd:unsignedEnd+5], []byte{1, 0, 0, 0, 64}) {
		t.Fatalf("signed receipt has invalid signature framing")
	}
	digestInput := append([]byte("LXP/v1/receipt\x00"), signedReceipt[:unsignedEnd]...)
	digestInput = append(digestInput, 0)
	digest := sha256.Sum256(digestInput)
	payload := map[string]interface{}{
		"x402Version": 2,
		"accepted":    accepted,
		"payload": map[string]interface{}{
			"txHash":            hash.Hex(),
			"receiptDigest":     hex.EncodeToString(digest[:]),
			"verificationLevel": "receipted",
			"idempotencyKey":    hex.EncodeToString(key[:]),
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode payment payload: %v", err)
	}
	return base64.StdEncoding.EncodeToString(data)
}

func boundIdentity(t *testing.T, client *ethclient.Client, address common.Address) [32]byte {
	t.Helper()
	selector := crypto.Keccak256([]byte("getUnifiedAccount(address)"))[:4]
	data := append(selector, common.LeftPadBytes(address.Bytes(), 32)...)
	precompile := common.HexToAddress("0x0000000000000000000000000000000000001004")
	result, err := client.CallContract(context.Background(), ethereum.CallMsg{To: &precompile, Data: data}, nil)
	if err != nil || len(result) < 96 {
		t.Fatalf("read bound identity for %s: %v (%x)", address, err, result)
	}
	var identity [32]byte
	copy(identity[:], result[64:96])
	if identity == ([32]byte{}) {
		t.Fatalf("address %s has no bound identity", address)
	}
	return identity
}

func layerXDIDID(public [32]byte) [32]byte {
	did := []byte("did:layerx:" + hex.EncodeToString(public[:]))
	canonical := append([]byte{byte(len(did) >> 8), byte(len(did))}, did...)
	return sha256.Sum256(append([]byte("LXP/v1/did-id\x00"), canonical...))
}

func ledgerAccountID(t *testing.T, client *ethclient.Client, owner [32]byte, asset [32]byte) [32]byte {
	t.Helper()
	data, err := ledger.GetABI().Pack("accountId", owner, asset)
	if err != nil {
		t.Fatalf("pack accountId: %v", err)
	}
	address := common.HexToAddress(paidSearchLedger)
	result, err := client.CallContract(context.Background(), ethereum.CallMsg{To: &address, Data: data}, nil)
	if err != nil || len(result) != 32 {
		t.Fatalf("read accountId for %s: %v (%x)", owner, err, result)
	}
	var id [32]byte
	copy(id[:], result)
	return id
}

func ledgerBalance(t *testing.T, client *ethclient.Client, account, asset [32]byte) *big.Int {
	t.Helper()
	data, err := ledger.GetABI().Pack("balanceOf", account, asset)
	if err != nil {
		t.Fatalf("pack balanceOf: %v", err)
	}
	address := common.HexToAddress(paidSearchLedger)
	result, err := client.CallContract(context.Background(), ethereum.CallMsg{To: &address, Data: data}, nil)
	if err != nil {
		t.Fatalf("read ledger balance: %v", err)
	}
	values, err := ledger.GetABI().Unpack("balanceOf", result)
	if err != nil || len(values) != 1 {
		t.Fatalf("decode ledger balance: %v (%x)", err, result)
	}
	balance, ok := values[0].(*big.Int)
	if !ok {
		t.Fatalf("ledger balance type = %T", values[0])
	}
	return new(big.Int).Set(balance)
}

func sequencerReceipt(t *testing.T, client *ethclient.Client, hash common.Hash) *evmrpc.SequencerReceiptResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var result *evmrpc.SequencerReceiptResult
	for ctx.Err() == nil {
		if err := client.Client().CallContext(ctx, &result, "pax_sequencerReceipt", hash); err != nil {
			t.Fatalf("read sequencer receipt %s: %v", hash, err)
		}
		if result != nil && result.Rung != evmrpc.RungSubmitted {
			return result
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("wait for sequencer receipt %s: %v", hash, ctx.Err())
	return nil
}

func runXWebsearch(t *testing.T, binary, config, receiverKey string, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	commandArgs := append([]string{"--config", config}, args...)
	cmd := exec.CommandContext(ctx, binary, commandArgs...)
	cmd.Env = append(os.Environ(), "X_WEBSEARCH_RECEIVER_KEY_FILE="+receiverKey)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("x-websearch %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return output
}

func xWebsearchBinary(t *testing.T) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate localnode test source")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", ".."))
	candidates := []string{filepath.Join(root, "services", "interop", "target", "debug", "x-websearch")}
	if target := os.Getenv("CARGO_TARGET_DIR"); target != "" {
		candidates = append(candidates, filepath.Join(target, "debug", "x-websearch"), filepath.Join(target, "x-websearch"))
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	t.Fatalf("x-websearch binary is absent; checked %s", strings.Join(candidates, ", "))
	return ""
}

func freeLoopbackAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve sidecar loopback port: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release sidecar loopback port: %v", err)
	}
	return address
}

func (n *followerNetwork) env(name string) string {
	value := os.Getenv(name)
	if value == "" {
		panic(fmt.Sprintf("%s is required", name))
	}
	return value
}
