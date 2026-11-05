package app

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain/profile"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain/readpolicy"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"github.com/go-chi/chi/v5"
)

type repair27Chain struct {
	backend  *simulated.Backend
	endpoint string
	key      *ecdsa.PrivateKey
	chainID  int64
	nonce    uint64
}

func repair27Node(t *testing.T, chainID int64) *repair27Chain {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return repair27NodeWithKey(t, chainID, key)
}

func repair27NodeWithKey(t *testing.T, chainID int64, key *ecdsa.PrivateKey) *repair27Chain {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	cfg := *params.AllDevChainProtocolChanges
	cfg.ChainID = big.NewInt(chainID)
	backend := simulated.NewBackend(types.GenesisAlloc{
		crypto.PubkeyToAddress(key.PublicKey): {Balance: new(big.Int).Mul(big.NewInt(1000), big.NewInt(params.Ether))},
	}, func(n *node.Config, e *ethconfig.Config) {
		n.HTTPHost, n.HTTPPort = "127.0.0.1", port
		n.HTTPModules = []string{"eth"}
		e.Genesis.Config = &cfg
		e.NetworkId = uint64(chainID)
	})
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	backend.Commit()
	return &repair27Chain{backend: backend, endpoint: "http://127.0.0.1:" + strconv.Itoa(port), key: key, chainID: chainID}
}

func (c *repair27Chain) send(t *testing.T, to *common.Address, data []byte) common.Hash {
	t.Helper()
	tx, err := types.SignNewTx(c.key, types.LatestSignerForChainID(big.NewInt(c.chainID)), &types.DynamicFeeTx{
		ChainID: big.NewInt(c.chainID), Nonce: c.nonce, GasTipCap: big.NewInt(params.GWei), GasFeeCap: big.NewInt(10 * params.GWei), Gas: 100000, To: to, Value: big.NewInt(1), Data: data,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.backend.Client().SendTransaction(context.Background(), tx); err != nil {
		t.Fatal(err)
	}
	c.nonce++
	return tx.Hash()
}

// The EVM executes LOG0 during contract creation. Two creations in one block
// give the second receipt a genuine nonzero block-global event index.
func (c *repair27Chain) events(t *testing.T) common.Hash {
	t.Helper()
	code := common.FromHex("0x60006000a060006000f3")
	c.send(t, nil, code)
	hash := c.send(t, nil, code)
	c.backend.Commit()
	return hash
}

func repair27HTTP(t *testing.T, endpoint string) *httptest.Server {
	t.Helper()
	rpcClient, err := dialNetworkRPC(context.Background(), endpoint, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(rpcClient.Close)
	// Exercise the production registration without relying on preflight engine
	// readiness: transaction reads must independently attest their evidence.
	a := &networkAuthority{rpc: rpcClient, config: networkAuthorityConfig{Manifest: deployment.Current()}}
	admission, err := rest.NewAdmission(100, 100, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	a.RegisterNetworkReads(router, admission)
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv
}

func repair27SDK(t *testing.T, c *repair27Chain) *chain.Client {
	t.Helper()
	header, err := c.backend.Client().HeaderByNumber(context.Background(), big.NewInt(0))
	if err != nil {
		t.Fatal(err)
	}
	client, err := chain.NewVerifiedClient(context.Background(), c.endpoint, "", chain.VerifiedConfig{
		Profile: profile.Paxeer("repair-27", "repair-27"), AnchorHeight: 0, AnchorHash: header.Hash(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client
}

func repair27Submitted(t *testing.T, hash common.Hash) readpolicy.TransactionState {
	t.Helper()
	state, err := readpolicy.Submitted(hash, "repair-27")
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func repair27Refused(t *testing.T, srv *httptest.Server, hash common.Hash) {
	t.Helper()
	status, body, _ := netCall(t, http.MethodGet, srv.URL+"/v2/transactions/"+hash.Hex())
	if status != 503 || interfaces.ValidateJSON("Error", body) != nil {
		t.Fatalf("conflicting evidence published: %d %s", status, body)
	}
	var got struct {
		Code string          `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Code != "INTEGRITY_QUARANTINED" || len(got.Data) != 0 {
		t.Fatalf("receipt was not quarantined: %s", body)
	}
}

func TestRepair_27_receipt_event_position(t *testing.T) {
	c := repair27Node(t, 125)
	hash := c.events(t)
	receipt, err := c.backend.Client().TransactionReceipt(context.Background(), hash)
	if err != nil {
		t.Fatal(err)
	}
	if len(receipt.Logs) != 1 || receipt.Logs[0].Index != 1 || receipt.TransactionIndex != 1 {
		t.Fatalf("real nonzero event fixture not produced: %+v", receipt)
	}
	sdk := repair27SDK(t, c)
	next, expected, err := sdk.ReceiptState(context.Background(), repair27Submitted(t, hash))
	if err != nil {
		t.Fatal(err)
	}
	if next.State != "included_success" || expected.Receipt == nil || expected.Receipt.Position.LogIndex != "1" {
		t.Fatalf("SDK lost event identity: %+v %+v", next, expected)
	}
	servers := []*httptest.Server{repair27HTTP(t, c.endpoint), networkReadsServer(t, c.endpoint)}
	for _, srv := range servers {
		status, body, header := netCall(t, http.MethodGet, srv.URL+"/v2/transactions/"+hash.Hex())
		if status != 200 || interfaces.ValidateJSON("TxStatusResponse", body) != nil || header.Get("Cache-Control") != "no-store" {
			t.Fatalf("receipt HTTP contract: %d %s", status, body)
		}
		var got struct {
			Data    interfaces.TxStatusData `json:"data"`
			Quality string                  `json:"quality"`
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if got.Quality != "complete" || !reflect.DeepEqual(got.Data, expected) {
			t.Fatalf("API/SDK receipt mismatch: %s; SDK %+v", body, expected.Receipt)
		}
		position := got.Data.Receipt.Position
		if position.BlockHash != receipt.BlockHash.Hex() || position.TransactionHash != hash.Hex() || position.Height != interfaces.UInt256(receipt.BlockNumber.String()) || position.TransactionIndex != "1" || position.LogIndex != "1" || position.ChainId != "125" {
			t.Fatalf("consumer event identity: %+v", position)
		}
	}
}

func TestRepair_27_log_free_receipts(t *testing.T) {
	c := repair27Node(t, 125)
	to := common.HexToAddress("0x1234")
	transfer := c.send(t, &to, nil)
	revert := c.send(t, nil, common.FromHex("0x60006000fd"))
	c.backend.Commit()
	sdk := repair27SDK(t, c)
	servers := []*httptest.Server{repair27HTTP(t, c.endpoint), networkReadsServer(t, c.endpoint)}
	for _, tc := range []struct {
		hash          common.Hash
		success       bool
		state, status string
	}{
		{transfer, true, "included_success", "included_success"},
		{revert, false, "included_failure", "included_reverted"},
	} {
		receipt, err := c.backend.Client().TransactionReceipt(context.Background(), tc.hash)
		if err != nil || receipt == nil {
			t.Fatalf("receipt missing: %v", err)
		}
		if len(receipt.Logs) != 0 || (receipt.Status == 1) != tc.success {
			t.Fatalf("log-free fixture: %+v", receipt)
		}
		next, view, err := sdk.ReceiptState(context.Background(), repair27Submitted(t, tc.hash))
		if !errors.Is(err, chain.ErrReceiptPosition) || view.Receipt != nil || view.Status != tc.status || next.State != tc.state || next.BlockHash == nil || *next.BlockHash != receipt.BlockHash {
			t.Fatalf("SDK log-free evidence/position refusal: %+v %+v %v", next, view, err)
		}
		for _, srv := range servers {
			repair27Refused(t, srv, tc.hash)
		}
	}
}

func TestRepair_27_wrong_chain(t *testing.T) {
	foreign := repair27Node(t, 1337)
	hash := foreign.events(t)
	receipt, err := foreign.backend.Client().TransactionReceipt(context.Background(), hash)
	if err != nil || receipt == nil || len(receipt.Logs) == 0 {
		t.Fatalf("foreign receipt not produced: %v", err)
	}
	repair27Refused(t, repair27HTTP(t, foreign.endpoint), hash)
	repair27Refused(t, networkReadsServer(t, foreign.endpoint), hash)
}

// Route selected RPC methods to a second real node. Both nodes produce their
// own signed transactions, blocks and receipts; no response payload is invented
// or modified. This proves refusal of conflicting evidence over real HTTP RPC.
func repair27SplitRPC(t *testing.T, primary, other string, methods map[string]bool) *httptest.Server {
	t.Helper()
	first, err := url.Parse(primary)
	if err != nil {
		t.Fatal(err)
	}
	second, err := url.Parse(other)
	if err != nil {
		t.Fatal(err)
	}
	p, q := httputil.NewSingleHostReverseProxy(first), httputil.NewSingleHostReverseProxy(second)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			http.Error(w, "request read failed", 500)
			return
		}
		_ = r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(body))
		var request struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			t.Error(err)
			http.Error(w, "invalid RPC request", 400)
			return
		}
		if methods[request.Method] {
			q.ServeHTTP(w, r)
		} else {
			p.ServeHTTP(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestRepair_27_conflicting_block(t *testing.T) {
	c, other := repair27Node(t, 125), repair27Node(t, 125)
	hash := c.events(t)
	other.events(t)
	a, err := c.backend.Client().HeaderByNumber(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := other.backend.Client().HeaderByNumber(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.Number.Cmp(b.Number) != 0 || a.Hash() == b.Hash() {
		t.Fatal("distinct committed blocks required")
	}
	proxy := repair27SplitRPC(t, c.endpoint, other.endpoint, map[string]bool{"eth_getBlockByNumber": true})
	repair27Refused(t, repair27HTTP(t, proxy.URL), hash)
}

func TestRepair_27_conflicting_inclusion(t *testing.T) {
	c := repair27Node(t, 125)
	other := repair27NodeWithKey(t, 125, c.key)
	hash := c.events(t)
	// Identical signed transactions are mined one height later on the other
	// node; its real transaction lookup conflicts with the first receipt.
	other.backend.Commit()
	if other.events(t) != hash {
		t.Fatal("expected the same signed transaction on both histories")
	}
	first, err := c.backend.Client().TransactionReceipt(context.Background(), hash)
	if err != nil {
		t.Fatal(err)
	}
	second, err := other.backend.Client().TransactionReceipt(context.Background(), hash)
	if err != nil {
		t.Fatal(err)
	}
	if first.BlockNumber.Cmp(second.BlockNumber) == 0 || first.BlockHash == second.BlockHash {
		t.Fatal("conflicting inclusion not produced")
	}
	proxy := repair27SplitRPC(t, c.endpoint, other.endpoint, map[string]bool{"eth_getTransactionByHash": true})
	repair27Refused(t, repair27HTTP(t, proxy.URL), hash)
}

func TestRepair_27_missing_receipt(t *testing.T) {
	c, empty := repair27Node(t, 125), repair27Node(t, 125)
	hash := c.events(t)
	proxy := repair27SplitRPC(t, c.endpoint, empty.endpoint, map[string]bool{"eth_getTransactionReceipt": true})
	repair27Refused(t, repair27HTTP(t, proxy.URL), hash)
}

func TestRepair_27_pending_and_unknown(t *testing.T) {
	c := repair27Node(t, 125)
	to := common.HexToAddress("0x1234")
	pending := c.send(t, &to, nil)
	srv := repair27HTTP(t, c.endpoint)
	for _, tc := range []struct {
		hash   common.Hash
		status string
	}{
		{pending, "submitted"}, {common.HexToHash("0x" + strings.Repeat("ab", 32)), "unknown"},
	} {
		status, body, _ := netCall(t, http.MethodGet, srv.URL+"/v2/transactions/"+tc.hash.Hex())
		if status != 200 || interfaces.ValidateJSON("TxStatusResponse", body) != nil {
			t.Fatalf("pending/unknown: %d %s", status, body)
		}
		var got struct {
			Data interfaces.TxStatusData `json:"data"`
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if got.Data.Status != tc.status || got.Data.Receipt != nil {
			t.Fatalf("missing receipt became included: %s", body)
		}
	}
}
