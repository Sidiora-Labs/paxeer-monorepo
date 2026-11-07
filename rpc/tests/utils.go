package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/merkle"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	precompiletest "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/testutil"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	paxutils "github.com/Sidiora-Labs/Paxeer-X-Network/utils"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/gogo/protobuf/proto"
)

const testAddr = "127.0.0.1"

var portProvider = atomic.Int32{}

func init() {
	portProvider.Store(7800)
}

type TestServer struct {
	evmrpc.EVMServer
	port int

	mockClient *MockClient
	app        *app.App
}

func (ts TestServer) Run(r func(port int)) {
	_ = ts.Start()
	defer ts.Stop()
	r(ts.port)
}

func (ts TestServer) SetupBlocks(blocks [][][]byte, initializer ...func(sdk.Context, *app.App)) {
	for _, block := range blocks {
		finalizeFixtureBlock(ts.app, ts.mockClient, block)
	}
}

func finalizeFixtureBlock(a *app.App, c *MockClient, txs [][]byte, initializer ...func(sdk.Context, *app.App)) *abci.ResponseFinalizeBlock {
	height := int64(len(c.blocks) + 1)
	validator := tmtypes.NewValidator(ed25519.TestSecretKey([]byte("rpc-test-validator")).Public(), 1)
	block := tmtypes.MakeBlock(height, paxutils.Map(txs, func(tx []byte) tmtypes.Tx { return tmtypes.Tx(bytes.Clone(tx)) }), &tmtypes.Commit{Height: height - 1}, nil)
	block.ChainID = a.ChainID
	block.Time = time.Now().UTC()
	block.ProposerAddress = validator.Address
	block.ValidatorsHash = tmtypes.NewValidatorSet([]*tmtypes.Validator{validator}).Hash()
	block.NextValidatorsHash = bytes.Clone(block.ValidatorsHash)
	block.AppHash = bytes.Clone(a.LastCommitID().Hash)
	if height > 1 {
		block.LastBlockID = tmtypes.BlockID{Hash: bytes.Clone(c.headers[height-2].Hash())}
		block.LastResultsHash = bytes.Clone(c.committedResultsHashes[height-2])
	}
	request := &abci.RequestFinalizeBlock{Txs: txs, Hash: block.Hash(), Header: block.Header.ToProto()}
	res, err := a.FinalizeBlock(context.Background(), request)
	if err != nil {
		panic(err)
	}
	ctx := a.GetContextForDeliverTx(nil)
	for _, initialize := range initializer {
		initialize(ctx, a)
	}
	if _, err := a.Commit(context.Background()); err != nil {
		panic(err)
	}
	results, err := abci.MarshalTxResults(res.TxResults)
	if err != nil {
		panic(err)
	}
	c.blocks = append(c.blocks, paxutils.Map(block.Txs, func(tx tmtypes.Tx) []byte { return bytes.Clone(tx) }))
	c.headers = append(c.headers, &block.Header)
	c.committedAppHashes = append(c.committedAppHashes, bytes.Clone(a.LastCommitID().Hash))
	c.committedResultsHashes = append(c.committedResultsHashes, merkle.HashFromByteSlices(results))
	c.recordBlockResult(res.TxResults, res.ConsensusParamUpdates, res.Events)
	return res
}

func initializeApp(
	t *testing.T,
	chainID string,
	initializer ...func(sdk.Context, *app.App),
) (*app.App, *MockClient) {
	a := app.Setup(t, false, true, chainID == "pacific-1")
	if err := precompiletest.Install(a); err != nil {
		t.Fatal(err)
	}
	a.ChainID = chainID
	c := &MockClient{}
	finalizeFixtureBlock(a, c, [][]byte{}, initializer...)
	return a, c
}

func SetupTestServer(
	t *testing.T,
	blocks [][][]byte,
	initializer ...func(sdk.Context, *app.App),
) TestServer {
	a, mockClient := initializeApp(t, "pax-test", initializer...)
	for _, block := range blocks {
		finalizeFixtureBlock(a, mockClient, block)
	}
	return setupTestServer(a, a.RPCContextProvider, mockClient)
}

func setupTestServer(
	a *app.App,
	ctxProvider func(int64) sdk.Context,
	mockClient *MockClient,
) TestServer {
	port := int(portProvider.Add(1))
	cfg := evmrpcconfig.DefaultConfig
	cfg.HTTPEnabled = true
	cfg.HTTPPort = port
	cfg.EnabledLegacyPaxApis = evmrpc.PaxLegacyAllGatedMethodNames()
	s, err := evmrpc.NewEVMHTTPServer(
		cfg,
		mockClient,
		&a.EvmKeeper,
		a.BeginBlockKeepers,
		a.BaseApp,
		a.TracerAnteHandler,
		ctxProvider,
		func(int64) client.TxConfig { return a.GetTxConfig() },
		"",
		nil,
	)
	if err != nil {
		panic(err)
	}
	if store := a.EvmKeeper.ReceiptStore(); store != nil {
		latest := int64(math.MaxInt64)
		if err := store.SetLatestVersion(latest); err != nil {
			panic(err)
		}
		_ = store.SetEarliestVersion(1)
	}
	return TestServer{EVMServer: s, port: port, mockClient: mockClient, app: a}
}

func sendRequestWithNamespace(namespace string, port int, method string, params ...interface{}) map[string]interface{} {
	paramsFormatted := ""
	if len(params) > 0 {
		paramsFormatted = strings.Join(paxutils.Map(params, formatParam), ",")
	}
	body := fmt.Sprintf("{\"jsonrpc\": \"2.0\",\"method\": \"%s_%s\",\"params\":[%s],\"id\":\"test\"}", namespace, method, paramsFormatted)
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s:%d", testAddr, port), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer func() { _ = res.Body.Close() }()
	resBody, _ := io.ReadAll(res.Body)
	resObj := map[string]interface{}{}
	_ = json.Unmarshal(resBody, &resObj)
	return resObj
}

func formatParam(p interface{}) string {
	if p == nil {
		return "null"
	}
	switch v := p.(type) {
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%f", v)
	case string:
		return fmt.Sprintf("\"%s\"", v)
	case common.Address:
		return fmt.Sprintf("\"%s\"", v)
	case []common.Address:
		wrapper := func(i common.Address) string {
			return formatParam(i)
		}
		return fmt.Sprintf("[%s]", strings.Join(paxutils.Map(v, wrapper), ","))
	case common.Hash:
		return fmt.Sprintf("\"%s\"", v)
	case []common.Hash:
		wrapper := func(i common.Hash) string {
			return formatParam(i)
		}
		return fmt.Sprintf("[%s]", strings.Join(paxutils.Map(v, wrapper), ","))
	case [][]common.Hash:
		wrapper := func(i []common.Hash) string {
			return formatParam(i)
		}
		return fmt.Sprintf("[%s]", strings.Join(paxutils.Map(v, wrapper), ","))
	case []string:
		return fmt.Sprintf("[%s]", strings.Join(v, ","))
	case []float64:
		return fmt.Sprintf("[%s]", strings.Join(paxutils.Map(v, func(s float64) string { return fmt.Sprintf("%f", s) }), ","))
	case []interface{}:
		return fmt.Sprintf("[%s]", strings.Join(paxutils.Map(v, formatParam), ","))
	case map[string]interface{}:
		kvs := []string{}
		for k, v := range v {
			kvs = append(kvs, fmt.Sprintf("\"%s\":%s", k, formatParam(v)))
		}
		return fmt.Sprintf("{%s}", strings.Join(kvs, ","))
	case map[string]map[string]interface{}:
		kvs := []string{}
		for k, v := range v {
			kvs = append(kvs, fmt.Sprintf("\"%s\":%s", k, formatParam(v)))
		}
		return fmt.Sprintf("{%s}", strings.Join(kvs, ","))
	default:
		return fmt.Sprintf("%s", p)
	}
}

func signAndEncodeTx(txData ethtypes.TxData, mnemonic string) []byte {
	signed := signTxWithMnemonic(txData, mnemonic)
	return encodeEvmTx(txData, signed)
}

func encodeEvmTx(txData ethtypes.TxData, signed *ethtypes.Transaction) []byte {
	var typedTx proto.Message
	switch txData.(type) {
	case *ethtypes.LegacyTx:
		typedTx, _ = ethtx.NewLegacyTx(signed)
	case *ethtypes.AccessListTx:
		typedTx, _ = ethtx.NewAccessListTx(signed)
	case *ethtypes.DynamicFeeTx:
		typedTx, _ = ethtx.NewDynamicFeeTx(signed)
	case *ethtypes.BlobTx:
		typedTx, _ = ethtx.NewBlobTx(signed)
	case *ethtypes.SetCodeTx:
		typedTx, _ = ethtx.NewSetCodeTx(signed)
	default:
		panic("invalid tx type")
	}
	msg, _ := types.NewMsgEVMTransaction(typedTx)
	builder := testkeeper.EVMTestApp.GetTxConfig().NewTxBuilder()
	_ = builder.SetMsgs(msg)
	tx := builder.GetTx()
	txBz, _ := testkeeper.EVMTestApp.GetTxConfig().TxEncoder()(tx)
	return txBz
}

func signAndEncodeCosmosTx(msg sdk.Msg, mnemonic string, acctN uint64, seq uint64) []byte {
	tx, err := signCosmosTxWithMnemonic(msg, mnemonic, acctN, seq)
	if err != nil {
		// TODO: pass in testing.T and assert no error instead
		panic(err)
	}
	return encodeCosmosTx(tx)
}

func encodeCosmosTx(tx sdk.Tx) []byte {
	txBz, _ := testkeeper.EVMTestApp.GetTxConfig().TxEncoder()(tx)
	return txBz
}
