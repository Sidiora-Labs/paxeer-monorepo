package evmrpc_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	tmutils "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	tmnode "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	rpclocal "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	rpctest "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/test"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

type pxRootAccountHarness struct {
	app      *app.App
	tm       *rpclocal.Local
	provider func(int64) sdk.Context
	height   int64
	address  common.Address
	pax      string
	did      string
	account  string
	huge     string
	http, ws *evmrpc.HTTPServer
	clients  []*rpc.Client
}

func newPxRootAccountHarness(t *testing.T) *pxRootAccountHarness {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	config, err := rpctest.CreateConfig(t, t.Name())
	require.NoError(t, err)
	genesis, err := tmtypes.GenesisDocFromFile(config.GenesisFile())
	require.NoError(t, err)
	genesis.ChainID = "hyperpax_125-1"
	require.NoError(t, genesis.SaveAs(config.GenesisFile()))
	consensusApp := kvstore.NewApplication()
	consensusApp.SetValidators(genesis.ValidatorUpdates())
	node, err := tmnode.New(ctx, config, func() {}, consensusApp, nil, nil, tmnode.NoOpMetricsProvider(), tmtypes.DefaultConsensusPolicy())
	require.NoError(t, err)
	consensus, ok := node.RPCEnvironment().ConsensusState.Get()
	require.True(t, ok)
	signing, ok := consensus.(interface {
		SetPrivValidator(context.Context, tmutils.Option[tmtypes.PrivValidator])
	})
	require.True(t, ok)
	validator, err := privval.LoadFilePV(config.PrivValidator.KeyFile(), config.PrivValidator.StateFile())
	require.NoError(t, err)
	signing.SetPrivValidator(ctx, tmutils.Some[tmtypes.PrivValidator](validator))
	require.NoError(t, node.Start(ctx))
	t.Cleanup(func() { cancel(); node.Wait(); require.NoError(t, consensusApp.Close()) })
	require.NoError(t, rpctest.WaitForRPC(ctx, config))
	tm, err := rpclocal.New(node)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		status, err := tm.Status(ctx)
		return err == nil && status.SyncInfo.LatestBlockHeight >= 2
	}, 20*time.Second, 20*time.Millisecond)
	application := app.Setup(t, false, true, false)
	t.Cleanup(func() { require.NoError(t, application.Close()) })
	h := &pxRootAccountHarness{app: application, tm: tm, height: 2, address: common.HexToAddress("0x223402")}
	state := application.GetContextForDeliverTx(nil).WithChainID(genesis.ChainID).WithBlockHeight(1).WithClosestUpgradeName(LatestCtxUpgradeName)
	pax := sdk.AccAddress(h.address.Bytes())
	h.pax = pax.String()
	application.EvmKeeper.SetAddressMapping(state, pax, h.address)
	application.EvmKeeper.SetNonce(state, h.address, 7)
	h.huge = new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 200), big.NewInt(234)).String()
	base := application.EvmKeeper.GetBaseDenom(state)
	coins := sdk.NewCoins(sdk.NewCoin(base, sdk.NewInt(987654321)), sdk.NewCoin("uxroot", sdk.NewIntFromBigInt(new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 200), big.NewInt(234)))))
	require.NoError(t, application.BankKeeper.MintCoins(state, "evm", coins))
	require.NoError(t, application.BankKeeper.SendCoinsFromModuleToAccount(state, "evm", pax, coins))
	application.BankKeeper.SetDenomMetaData(state, banktypes.Metadata{Base: "uxroot", Display: "XROOT", Name: "Root Asset", Symbol: "XROOT", DenomUnits: []*banktypes.DenomUnit{{Denom: "uxroot", Exponent: 0}, {Denom: "XROOT", Exponent: 8}}})
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	var did [32]byte
	copy(did[:], public)
	message, err := evmtypes.LayerXBindMessage(application.EvmKeeper.ChainID(state), h.address, 0)
	require.NoError(t, err)
	var signature [64]byte
	copy(signature[:], ed25519.Sign(private, message))
	_, err = application.EvmKeeper.BindLayerX(state, h.address, did, signature)
	require.NoError(t, err)
	h.did = evmtypes.LayerXDid(did)
	account, err := evmtypes.LayerXMainAccountID(did)
	require.NoError(t, err)
	h.account = hex.EncodeToString(account[:])
	// Commit the producer's signed binding, bank writes and nonce, then read only
	// versioned cache stores. No RPC fixture mutates the committed read context.
	application.SetDeliverStateToCommit()
	_, err = application.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), application.LastCommitID().Version)
	require.Equal(t, h.height, application.CommitMultiStore().Commit(true).Version)
	h.provider = func(height int64) sdk.Context {
		if height <= 0 {
			height = h.height
		}
		store, err := application.CommitMultiStore().CacheMultiStoreWithVersion(height)
		if err != nil {
			panic(err)
		}
		read := sdk.NewContext(store, tmproto.Header{ChainID: genesis.ChainID, Height: height}, false).WithClosestUpgradeName(LatestCtxUpgradeName).WithIsTracing(true)
		return read.WithConsensusParams(application.GetConsensusParams(read))
	}
	if receipts := application.EvmKeeper.ReceiptStore(); receipts != nil {
		require.NoError(t, receipts.SetLatestVersion(h.height))
		require.NoError(t, receipts.SetEarliestVersion(1))
	}
	h.start(t)
	return h
}
func (h *pxRootAccountHarness) start(t *testing.T, opts ...evmrpc.ServerOption) {
	t.Helper()
	config := evmrpcconfig.DefaultConfig
	config.HTTPAddress = "127.0.0.1"
	config.HTTPPort = 0
	config.WSPort = 0
	config.IPRateLimitRPS = 0
	config.FilterTimeout = time.Minute
	config.TraceBakeEnabled = false
	txConfig := h.app.GetTxConfig()
	txProvider := func(int64) client.TxConfig { return txConfig }
	httpService, err := evmrpc.NewEVMHTTPServer(config, h.tm, &h.app.EvmKeeper, h.app.BeginBlockKeepers, h.app.BaseApp, h.app.TracerAnteHandler, h.provider, txProvider, "", nil, opts...)
	require.NoError(t, err)
	h.http = httpService.(*evmrpc.HTTPServer)
	require.NoError(t, h.http.Start())
	t.Cleanup(h.http.Stop)
	wsService, err := evmrpc.NewEVMWebSocketServer(config, h.tm, &h.app.EvmKeeper, h.app.BeginBlockKeepers, h.app.BaseApp, h.app.TracerAnteHandler, h.provider, txProvider, "", nil, evmrpc.NewBlockHeaderNotifier(4), opts...)
	require.NoError(t, err)
	h.ws = wsService.(*evmrpc.HTTPServer)
	require.NoError(t, h.ws.Start())
	t.Cleanup(h.ws.Stop)
	httpClient, err := rpc.DialHTTP("http://" + h.http.ListenAddr() + "/")
	require.NoError(t, err)
	t.Cleanup(httpClient.Close)
	wsClient, err := rpc.DialWebsocket(t.Context(), "ws://"+h.ws.ListenAddr()+"/", "")
	require.NoError(t, err)
	t.Cleanup(wsClient.Close)
	h.clients = []*rpc.Client{httpClient, wsClient}
}
func pxRootCall(t *testing.T, c *rpc.Client, method string, args ...interface{}) map[string]interface{} {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	var result map[string]interface{}
	err := c.CallContext(ctx, &result, method, args...)
	if dataError, ok := err.(rpc.DataError); ok {
		t.Logf("%s error data: %v", method, dataError.ErrorData())
	}
	require.NoError(t, err)
	require.NotNil(t, result)
	pxRootAssertSchema(t, method, result)
	return result
}
func pxRootError(t *testing.T, c *rpc.Client, code int, dataCode, method string, args ...interface{}) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	var result interface{}
	err := c.CallContext(ctx, &result, method, args...)
	require.Error(t, err)
	var rpcErr rpc.Error
	require.ErrorAs(t, err, &rpcErr)
	require.Equal(t, code, rpcErr.ErrorCode(), "%s: %v", method, err)
	if code == -32602 {
		return
	}
	var dataErr rpc.DataError
	require.ErrorAs(t, err, &dataErr)
	data, ok := dataErr.ErrorData().(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, dataCode, data["code"])
	if code == -32004 {
		require.Equal(t, "Hosted gateway service operation", err.Error())
		require.Equal(t, method, data["method"])
	} else {
		require.Equal(t, "Paxeer read unavailable", err.Error())
	}
}

func TestPxNodeRoot_Account_CommittedReadsAndParity(t *testing.T) {
	h := newPxRootAccountHarness(t)
	expectedAccount := map[string]interface{}{"evm_address": strings.ToLower(h.address.Hex()), "pax_address": h.pax, "layerx_did": h.did, "layerx_account": h.account, "bound": true}
	for _, method := range []string{"px_resolveAccount", "px_getAccount", "px_getBalances", "px_listAssets", "px_getNetwork", "px_getCapabilities", "px_getRouteCatalogue", "px_register", "px_requestFunds"} {
		require.Contains(t, h.http.HTTPMethods(), method)
		require.Contains(t, h.ws.WSMethods(), method)
	}
	var previous map[string]interface{}
	for i, c := range h.clients {
		var unified hexutil.Bytes
		payload := append(crypto.Keccak256([]byte("getUnifiedAccount(address)"))[:4], common.LeftPadBytes(h.address.Bytes(), 32)...)
		require.NoError(t, c.CallContext(t.Context(), &unified, "eth_call", map[string]interface{}{"to": "0x0000000000000000000000000000000000001004", "data": hexutil.Encode(payload)}, "0x2"))
		require.NotEmpty(t, unified, "the committed fixture must serve the real addr view")
		for _, selector := range []string{strings.ToLower(h.address.Hex()), h.did} {
			resolved := pxRootCall(t, c, "px_resolveAccount", selector)
			require.Equal(t, expectedAccount, resolved)
			account := pxRootCall(t, c, "px_getAccount", selector)
			require.Equal(t, expectedAccount, account["account"])
			paxeer := account["paxeer"].(map[string]interface{})
			require.Equal(t, float64(h.height), paxeer["height"])
			require.Equal(t, "0x7", paxeer["nonce"])
			var expected hexutil.Big
			tag := "0x" + strconv.FormatInt(h.height, 16)
			require.NoError(t, c.CallContext(t.Context(), &expected, "eth_getBalance", h.address, tag))
			require.Equal(t, expected.String(), paxeer["balance"])
			require.Equal(t, hexutil.EncodeBig(new(big.Int).Mul(big.NewInt(987654321), big.NewInt(1000000000000))), paxeer["balance"])
			balances := pxRootCall(t, c, "px_getBalances", selector)
			require.Equal(t, expectedAccount, balances["account"])
			require.Equal(t, float64(h.height), balances["height"])
			found := false
			for _, entry := range balances["balances"].([]interface{}) {
				coin := entry.(map[string]interface{})
				if coin["denom"] == "uxroot" {
					require.Equal(t, h.huge, coin["amount"])
					found = true
				}
			}
			require.True(t, found, "bank coin list lost the full-width balance")
			if i == 0 {
				previous = account
			} else {
				require.Equal(t, previous, account)
			}
		}
		network := pxRootCall(t, c, "px_getNetwork")
		require.Equal(t, "hyperpax_125-1", network["network_id"])
		require.Equal(t, map[string]interface{}{"chain_id": "0x7d", "latest_block": "0x2"}, network["paxeer"])
		require.NotNil(t, network["consensus"])
		require.NotContains(t, network, "reason")
		capabilities := pxRootCall(t, c, "px_getCapabilities")
		require.Equal(t, "2", capabilities["rpc_height"])
		require.InDelta(t, float64(time.Now().Unix()), capabilities["probed_at"], 10)
		require.Equal(t, capabilities, pxRootCall(t, c, "px_getCapabilities"), "cache must retain the complete probe snapshot")
		require.True(t, capabilities["ledger"].(bool), "ledger accountId must decode through the real EVM call path")
		probeNames := []string{"exchange", "bridge", "launchpad", "ledger", "pay", "auth", "programs"}
		probeSignatures := []string{"totalMarkets()", "isPaused()", "getPoolCount()", "accountId(bytes32,bytes32)", "budgetRemaining(bytes32)", "isLive(bytes32,uint64,uint64)", "getCurrentFeeSchedule()"}
		probeWords := []int{0, 0, 0, 2, 1, 3, 0}
		for index, name := range probeNames {
			data := append(crypto.Keccak256([]byte(probeSignatures[index]))[:4], make([]byte, probeWords[index]*32)...)
			var answer hexutil.Bytes
			err := c.CallContext(t.Context(), &answer, "eth_call", map[string]interface{}{"to": capabilities["addresses"].(map[string]interface{})[name], "data": hexutil.Encode(data)}, "0x2")
			switch name {
			case "auth":
				if err == nil && len(answer) >= 96 {
					require.True(t, capabilities[name].(bool))
				} else {
					require.False(t, capabilities[name].(bool))
				}
			case "programs":
				require.Equal(t, err == nil && len(answer) == 288, capabilities[name])
			default:
				require.Equal(t, err == nil && len(answer) == 32, capabilities[name])
			}
		}
		catalogue := pxRootCall(t, c, "px_getRouteCatalogue")
		require.Equal(t, []interface{}{"/"}, catalogue["rpc_paths"])
		require.NotEmpty(t, catalogue["services"])
		require.NotEmpty(t, catalogue["routes"])
		sourceBytes, err := os.ReadFile("../tools/paxeer-x/route-catalogue.json")
		require.NoError(t, err)
		var source map[string]interface{}
		require.NoError(t, json.Unmarshal(sourceBytes, &source))
		serviceIDs := map[string]bool{}
		for _, entry := range catalogue["services"].([]interface{}) {
			serviceIDs[entry.(map[string]interface{})["id"].(string)] = true
		}
		expectedRoutes := []interface{}{}
		for _, entry := range source["routes"].([]interface{}) {
			route := entry.(map[string]interface{})
			if !serviceIDs[route["service"].(string)] {
				continue
			}
			for _, key := range []string{"upstream", "upstream_path", "source", "owner_source", "forwarded_headers", "attestor_authorization_headers"} {
				delete(route, key)
			}
			expectedRoutes = append(expectedRoutes, route)
		}
		require.Equal(t, expectedRoutes, catalogue["routes"])
		readiness := catalogue["readiness"].(map[string]interface{})
		require.Equal(t, true, readiness["node_rpc"].(map[string]interface{})["ready"])
		for key, value := range readiness {
			if key != "node_rpc" {
				require.Equal(t, false, value.(map[string]interface{})["ready"])
				require.Equal(t, "unknown", value.(map[string]interface{})["status"])
			}
		}
		for _, entry := range catalogue["services"].([]interface{}) {
			service := entry.(map[string]interface{})
			require.Equal(t, "product", service["exposure"])
			require.NotContains(t, service, "upstream")
		}
		for _, entry := range catalogue["routes"].([]interface{}) {
			route := entry.(map[string]interface{})
			require.NotContains(t, route, "upstream")
			require.NotContains(t, route, "upstream_path")
		}
		pxRootError(t, c, -32001, "indexer_not_configured", "px_listAssets")
	}
}

func TestPxNodeRoot_Account_ValidationRefusalsAndUnbound(t *testing.T) {
	h := newPxRootAccountHarness(t)
	public, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	unbound := "did:layerx:" + hex.EncodeToString(public)
	for _, c := range h.clients {
		for _, method := range []string{"px_resolveAccount", "px_getAccount", "px_getBalances"} {
			for _, args := range [][]interface{}{nil, {nil}, {"garbage"}, {"0x1234"}, {"did:layerx:" + strings.Repeat("0", 64)}, {h.did, "extra"}, {17}} {
				pxRootError(t, c, -32602, "", method, args...)
			}
			pxRootError(t, c, -32001, "native_account_reverse_resolution_unsupported", method, h.account)
			pxRootError(t, c, -32001, "native_account_reverse_resolution_unsupported", method, "0x"+h.account)
		}
		for _, method := range []string{"px_getNetwork", "px_listAssets", "px_getCapabilities", "px_getRouteCatalogue"} {
			pxRootError(t, c, -32602, "", method, "extra")
		}
		pxRootError(t, c, -32001, "identity_binding_unavailable", "px_resolveAccount", unbound)
		account := pxRootCall(t, c, "px_getAccount", unbound)
		require.Nil(t, account["paxeer"])
		resolved := account["account"].(map[string]interface{})
		require.Nil(t, resolved["evm_address"])
		require.Nil(t, resolved["pax_address"])
		require.False(t, resolved["bound"].(bool))
		require.Equal(t, unbound, resolved["layerx_did"])
		pxRootError(t, c, -32001, "identity_binding_unavailable", "px_getBalances", unbound)
		naked := pxRootCall(t, c, "px_resolveAccount", "0x0000000000000000000000000000000000223403")
		require.False(t, naked["bound"].(bool))
		require.Nil(t, naked["layerx_did"])
		require.Nil(t, naked["layerx_account"])
		pxRootError(t, c, -32004, "hosted_gateway_service_required", "px_register", strings.Repeat("ab", 32), strings.Repeat("cd", 64))
		pxRootError(t, c, -32004, "hosted_gateway_service_required", "px_requestFunds", h.did, hex.EncodeToString(public))
		for _, args := range [][]interface{}{nil, {"ab"}, {strings.Repeat("AB", 32), strings.Repeat("cd", 64)}, {strings.Repeat("ab", 32), "zz"}, {strings.Repeat("ab", 32), strings.Repeat("cd", 64), "extra"}} {
			pxRootError(t, c, -32602, "", "px_register", args...)
		}
		for _, args := range [][]interface{}{nil, {h.did}, {"bad/did", hex.EncodeToString(public)}, {h.did, strings.Repeat("00", 32)}, {h.did, strings.Repeat("AB", 32)}, {"did::alice", hex.EncodeToString(public)}} {
			pxRootError(t, c, -32602, "", "px_requestFunds", args...)
		}
	}
}
func TestPxNodeRoot_Account_RestartPreservesCommittedIdentity(t *testing.T) {
	h := newPxRootAccountHarness(t)
	before := pxRootCall(t, h.clients[0], "px_getAccount", h.did)
	beforeBalances := pxRootCall(t, h.clients[1], "px_getBalances", h.did)
	for _, c := range h.clients {
		c.Close()
	}
	h.http.Stop()
	h.ws.Stop()
	h.start(t)
	for _, c := range h.clients {
		require.Equal(t, before, pxRootCall(t, c, "px_getAccount", h.did))
		require.Equal(t, beforeBalances, pxRootCall(t, c, "px_getBalances", h.did))
	}
}

func TestPxNodeRoot_Account_ConfigAndIndexerUnavailable(t *testing.T) {
	h := newPxRootAccountHarness(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	endpoint := "http://" + listener.Addr().String()
	t.Cleanup(func() { _ = listener.Close() })
	settings := viper.New()
	settings.Set("evm.px_indexer_url", endpoint)
	config, err := evmrpcconfig.ReadConfig(settings)
	require.NoError(t, err)
	require.Equal(t, endpoint, config.PxIndexerURL)
	for _, c := range h.clients {
		c.Close()
	}
	h.http.Stop()
	h.ws.Stop()
	h.start(t, evmrpc.WithPxIndexerURL(endpoint))
	require.NoError(t, listener.Close())
	for _, c := range h.clients {
		pxRootError(t, c, -32001, "indexer_unreachable", "px_listAssets")
	}
	txProvider := func(int64) client.TxConfig { return h.app.GetTxConfig() }
	for _, endpoint := range []string{"", "file:///tmp/assets", "https://user:password@example.invalid", "https://example.invalid?key=secret"} {
		config.PxIndexerURL = ""
		_, err := evmrpc.NewEVMHTTPServer(config, h.tm, &h.app.EvmKeeper, h.app.BeginBlockKeepers, h.app.BaseApp, h.app.TracerAnteHandler, h.provider, txProvider, "", nil, evmrpc.WithPxIndexerURL(endpoint))
		require.Error(t, err)
		_, err = evmrpc.NewEVMWebSocketServer(config, h.tm, &h.app.EvmKeeper, h.app.BeginBlockKeepers, h.app.BaseApp, h.app.TracerAnteHandler, h.provider, txProvider, "", nil, evmrpc.NewBlockHeaderNotifier(1), evmrpc.WithPxIndexerURL(endpoint))
		require.Error(t, err)
	}
}

func pxRootAssertSchema(t *testing.T, method string, value interface{}) {
	t.Helper()
	document, err := os.ReadFile("../services/platform/hosted/gateway/openrpc.json")
	require.NoError(t, err)
	var openrpc struct {
		Methods []struct {
			Name   string `json:"name"`
			Result struct {
				Schema map[string]interface{} `json:"schema"`
			} `json:"result"`
		} `json:"methods"`
	}
	require.NoError(t, json.Unmarshal(document, &openrpc))
	for _, entry := range openrpc.Methods {
		if entry.Name == method {
			require.NoError(t, pxRootSchema(entry.Result.Schema, value, "result"))
			return
		}
	}
	t.Fatalf("method %s is missing from OpenRPC", method)
}

// Validate the JSON Schema keywords used by these OpenRPC results, including
// their nested null branches and closed objects, without rewriting the schemas.
func pxRootSchema(schema map[string]interface{}, value interface{}, path string) error {
	if branches, ok := schema["anyOf"].([]interface{}); ok {
		matched := false
		for _, branch := range branches {
			matched = matched || pxRootSchema(branch.(map[string]interface{}), value, path) == nil
		}
		if !matched {
			return fmt.Errorf("%s matches no anyOf branch", path)
		}
	}
	if branches, ok := schema["oneOf"].([]interface{}); ok {
		matches := 0
		for _, branch := range branches {
			if pxRootSchema(branch.(map[string]interface{}), value, path) == nil {
				matches++
			}
		}
		if matches != 1 {
			return fmt.Errorf("%s matches %d oneOf branches", path, matches)
		}
	}
	if negated, ok := schema["not"].(map[string]interface{}); ok && pxRootSchema(negated, value, path) == nil {
		return fmt.Errorf("%s matches forbidden schema", path)
	}
	if constant, ok := schema["const"]; ok && !reflect.DeepEqual(value, constant) {
		return fmt.Errorf("%s differs from const", path)
	}
	if enum, ok := schema["enum"].([]interface{}); ok {
		found := false
		for _, item := range enum {
			found = found || reflect.DeepEqual(item, value)
		}
		if !found {
			return fmt.Errorf("%s not in enum", path)
		}
	}
	if types, ok := schema["type"]; ok {
		options, ok := types.([]interface{})
		if !ok {
			options = []interface{}{types}
		}
		matches := false
		for _, candidate := range options {
			switch candidate {
			case "null":
				matches = matches || value == nil
			case "object":
				_, ok := value.(map[string]interface{})
				matches = matches || ok
			case "array":
				_, ok := value.([]interface{})
				matches = matches || ok
			case "string":
				_, ok := value.(string)
				matches = matches || ok
			case "boolean":
				_, ok := value.(bool)
				matches = matches || ok
			case "integer":
				number, ok := value.(float64)
				matches = matches || (ok && number == math.Trunc(number))
			default:
				return fmt.Errorf("unsupported schema type %v", candidate)
			}
		}
		if !matches {
			return fmt.Errorf("%s has wrong type %T", path, value)
		}
	}
	if object, ok := value.(map[string]interface{}); ok {
		if required, ok := schema["required"].([]interface{}); ok {
			for _, key := range required {
				if _, present := object[key.(string)]; !present {
					return fmt.Errorf("%s lacks %s", path, key)
				}
			}
		}
		properties, _ := schema["properties"].(map[string]interface{})
		for key, item := range object {
			if child, ok := properties[key].(map[string]interface{}); ok {
				if err := pxRootSchema(child, item, path+"."+key); err != nil {
					return err
				}
			} else if schema["additionalProperties"] == false {
				return fmt.Errorf("%s has additional property %s", path, key)
			}
		}
	}
	if array, ok := value.([]interface{}); ok {
		if maximum, ok := schema["maxItems"].(float64); ok && float64(len(array)) > maximum {
			return fmt.Errorf("%s exceeds maxItems", path)
		}
		if items, ok := schema["items"].(map[string]interface{}); ok {
			for i, item := range array {
				if err := pxRootSchema(items, item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
					return err
				}
			}
		}
	}
	if str, ok := value.(string); ok {
		if pattern, ok := schema["pattern"].(string); ok && !regexp.MustCompile(pattern).MatchString(str) {
			return fmt.Errorf("%s fails pattern", path)
		}
	}
	if number, ok := value.(float64); ok {
		if minimum, ok := schema["minimum"].(float64); ok && number < minimum {
			return fmt.Errorf("%s below minimum", path)
		}
		if maximum, ok := schema["maximum"].(float64); ok && number > maximum {
			return fmt.Errorf("%s above maximum", path)
		}
	}
	return nil
}
