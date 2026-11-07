package evmrpc_test

import (
	"context"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func newOptionsWiringServer(s *pendingSequencer, websocket bool, opts ...evmrpc.ServerOption) (evmrpc.EVMServer, error) {
	config := evmrpcconfig.DefaultConfig
	config.HTTPAddress = TestAddr
	config.HTTPPort = 0
	config.WSPort = 0
	config.FilterTimeout = time.Minute
	txConfig := func(int64) client.TxConfig { return s.app.GetTxConfig() }
	if websocket {
		return evmrpc.NewEVMWebSocketServer(config, &MockClient{latestOverride: s.committed}, &s.app.EvmKeeper,
			s.app.BeginBlockKeepers, s.app.BaseApp, s.app.TracerAnteHandler, s.ctxProvider, txConfig, "", nil, nil, opts...)
	}
	return evmrpc.NewEVMHTTPServer(config, &MockClient{latestOverride: s.committed}, &s.app.EvmKeeper,
		s.app.BeginBlockKeepers, s.app.BaseApp, s.app.TracerAnteHandler, s.ctxProvider, txConfig, "", nil, opts...)
}

func startOptionsWiringServer(t *testing.T, s *pendingSequencer, websocket bool, opts ...evmrpc.ServerOption) *rpc.Client {
	t.Helper()
	service, err := newOptionsWiringServer(s, websocket, opts...)
	require.NoError(t, err)
	t.Cleanup(service.Stop)
	require.NoError(t, service.Start())
	address := service.(*evmrpc.HTTPServer).ListenAddr()
	var rpcClient *rpc.Client
	if websocket {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		rpcClient, err = rpc.DialWebsocket(ctx, "ws://"+address, "http://localhost")
	} else {
		rpcClient, err = rpc.DialHTTP("http://" + address)
	}
	require.NoError(t, err)
	t.Cleanup(rpcClient.Close)
	return rpcClient
}

func TestHTTPServerPendingOptionServesPendingBlock(t *testing.T) {
	params := app.DefaultConsensusParams
	configured := *params
	blockParams := *params.Block
	blockParams.MaxGasWanted = pendingTransferGas
	configured.Block = &blockParams
	app.DefaultConsensusParams = &configured
	t.Cleanup(func() { app.DefaultConsensusParams = params })

	sender := utils.NewSigner()
	s := newPendingSequencer(t, 1, sender)
	transaction := pendingTransfer(t, sender, 0, utils.NewSigner().EvmAddress, big.NewInt(pendingOnePaxWei))
	accepted, err := s.exec.Accept(transaction)
	require.NoError(t, err)
	require.Equal(t, abci.CodeTypeOK, accepted.Result.Code, accepted.Result.Log)
	option, err := evmrpc.NewPendingOption(s.exec)
	require.NoError(t, err)
	snapshot, err := s.exec.Snapshot()
	require.NoError(t, err)

	for _, websocket := range []bool{false, true} {
		t.Run(map[bool]string{false: "HTTP", true: "WebSocket"}[websocket], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			withoutOption := startOptionsWiringServer(t, s, websocket)
			var unavailable map[string]interface{}
			require.EqualError(t, withoutOption.CallContext(ctx, &unavailable, "eth_getBlockByNumber", "pending", false),
				evmrpc.ErrPendingServedBySequencer.Error())

			withOption := startOptionsWiringServer(t, s, websocket, option)
			var block struct {
				Number       hexutil.Uint64 `json:"number"`
				Timestamp    hexutil.Uint64 `json:"timestamp"`
				Hash         *common.Hash   `json:"hash"`
				StateRoot    *common.Hash   `json:"stateRoot"`
				Transactions []common.Hash  `json:"transactions"`
			}
			require.NoError(t, withOption.CallContext(ctx, &block, "eth_getBlockByNumber", "pending", false))
			require.Equal(t, hexutil.Uint64(s.committed+1), block.Number)
			require.Equal(t, hexutil.Uint64(snapshot.BlockTime().Unix()), block.Timestamp)
			require.Nil(t, block.Hash)
			require.Nil(t, block.StateRoot)
			require.Equal(t, []common.Hash{pendingBlockEthHash(t, s, transaction)}, block.Transactions)
		})
	}
}

func TestHTTPServerForwardOptionForwardsRawTransaction(t *testing.T) {
	s := newPendingSequencer(t, 1)
	raw, err := multiTxBlockTx1.MarshalBinary()
	require.NoError(t, err)
	wantHash := multiTxBlockTx1.Hash()
	for _, websocket := range []bool{false, true} {
		t.Run(map[bool]string{false: "HTTP", true: "WebSocket"}[websocket], func(t *testing.T) {
			type forwardedRequest struct {
				body        []byte
				err         error
				method      string
				path        string
				contentType string
			}
			requests := make(chan forwardedRequest, 1)
			endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				requests <- forwardedRequest{body: body, err: err, method: r.Method, path: r.URL.RequestURI(), contentType: r.Header.Get("Content-Type")}
				var request struct {
					ID json.RawMessage `json:"id"`
				}
				if err != nil || json.Unmarshal(body, &request) != nil {
					http.Error(w, "invalid forwarding request", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(struct {
					JSONRPC string          `json:"jsonrpc"`
					ID      json.RawMessage `json:"id"`
					Result  common.Hash     `json:"result"`
				}{JSONRPC: "2.0", ID: request.ID, Result: wantHash}); err != nil {
					t.Errorf("write forwarding response: %v", err)
				}
			}))
			t.Cleanup(endpoint.Close)
			option, err := evmrpc.NewSequencerForwardOption(endpoint.URL + "/rpc?network=pax")
			require.NoError(t, err)
			rpcClient := startOptionsWiringServer(t, s, websocket, option)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var hash common.Hash
			require.NoError(t, rpcClient.CallContext(ctx, &hash, "eth_sendRawTransaction", hexutil.Bytes(raw)))
			require.Equal(t, wantHash, hash)
			select {
			case forwarded := <-requests:
				require.NoError(t, forwarded.err)
				require.Equal(t, http.MethodPost, forwarded.method)
				require.Equal(t, "/rpc?network=pax", forwarded.path)
				require.Equal(t, "application/json", forwarded.contentType)
				var request struct {
					JSONRPC string          `json:"jsonrpc"`
					Method  string          `json:"method"`
					Params  []hexutil.Bytes `json:"params"`
				}
				require.NoError(t, json.Unmarshal(forwarded.body, &request))
				require.Equal(t, "2.0", request.JSONRPC)
				require.Equal(t, "eth_sendRawTransaction", request.Method)
				require.Equal(t, []hexutil.Bytes{raw}, request.Params)
			case <-ctx.Done():
				t.Fatal("sequencer endpoint did not receive the raw transaction")
			}
		})
	}

	t.Run("sequencer and forwarder conflict", func(t *testing.T) {
		sequencer := newSubscribeSequencer(t, utils.NewSigner())
		forward, err := evmrpc.NewSequencerForwardOption("http://127.0.0.1:8545")
		require.NoError(t, err)
		for _, websocket := range []bool{false, true} {
			for _, opts := range [][]evmrpc.ServerOption{{sequencer.option, forward}, {forward, sequencer.option}} {
				server, err := newOptionsWiringServer(sequencer.pendingSequencer, websocket, opts...)
				require.EqualError(t, err, "sequencer and sequencer forwarding options are mutually exclusive")
				require.Nil(t, server)
			}
		}
	})
}
