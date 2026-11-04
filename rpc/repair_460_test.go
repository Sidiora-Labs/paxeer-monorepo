package evmrpc_test

import (
	"context"
	"crypto/sha256"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/merkle"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	rpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	rpcfixtures "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/tests"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestRepair460CommittedHeaderParity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	application, consensus := rpcfixtures.SetupCommittedHeaderFixture(t)
	t.Cleanup(func() { require.NoError(t, application.Close()) })
	bankTx, evmTxs := rpcfixtures.CommittedHeaderTransactions(t, application)
	config := rpcconfig.DefaultConfig
	config.HTTPPort, config.WSPort, config.IPRateLimitRPS = 0, 0, 0
	provider := application.RPCContextProvider
	txConfig := func(int64) client.TxConfig { return application.GetTxConfig() }
	httpService, err := evmrpc.NewEVMHTTPServer(config, consensus, &application.EvmKeeper,
		application.BeginBlockKeepers, application.BaseApp, application.TracerAnteHandler,
		provider, txConfig, "", nil)
	require.NoError(t, err)
	t.Cleanup(httpService.Stop)
	httpServer := httptest.NewServer(httpService.(http.Handler))
	t.Cleanup(httpServer.Close)
	httpClient, err := gethrpc.DialHTTP(httpServer.URL)
	require.NoError(t, err)
	t.Cleanup(httpClient.Close)
	notifier := evmrpc.NewBlockHeaderNotifier(4)
	wsService, err := evmrpc.NewEVMWebSocketServer(config, consensus, &application.EvmKeeper,
		application.BeginBlockKeepers, application.BaseApp, application.TracerAnteHandler,
		provider, txConfig, "", nil, notifier)
	require.NoError(t, err)
	t.Cleanup(wsService.Stop)
	wsServer := httptest.NewServer(wsService.(http.Handler))
	t.Cleanup(wsServer.Close)
	wsClient, err := gethrpc.DialWebsocket(ctx, "ws"+strings.TrimPrefix(wsServer.URL, "http"), "localhost")
	require.NoError(t, err)
	t.Cleanup(wsClient.Close)
	heads := make(chan map[string]interface{}, 4)
	subscription, err := wsClient.EthSubscribe(ctx, heads, "newHeads")
	require.NoError(t, err)
	t.Cleanup(subscription.Unsubscribe)

	parentHash := common.HexToHash("0x01")
	parentState := common.HexToHash("0x02")
	for i, scenario := range []struct {
		name     string
		txs      [][]byte
		evmCount int
	}{
		{name: "empty_consensus_block"},
		{name: "nonempty_consensus_without_EVM", txs: [][]byte{bankTx}},
		{name: "executed_EVM_transactions", txs: evmTxs, evmCount: 2},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			height := int64(i + 2)
			data := tmtypes.Data{}
			for _, tx := range scenario.txs {
				data.Txs = append(data.Txs, tmtypes.Tx(tx))
			}
			header := tmtypes.Header{
				ChainID: application.ChainID, Height: height, Time: time.Now().UTC(),
				LastBlockID: tmtypes.BlockID{Hash: parentHash.Bytes()},
				AppHash:     parentState.Bytes(), DataHash: data.Txs.Hash(),
				LastResultsHash: common.HexToHash("0x03").Bytes(),
				ProposerAddress: common.HexToAddress("0x0405").Bytes(),
				ValidatorsHash:  common.HexToHash("0x06").Bytes(),
			}
			blockHash := common.BigToHash(big.NewInt(height + 100))
			request := &abci.RequestFinalizeBlock{Hash: blockHash.Bytes(), Header: header.ToProto(), Txs: scenario.txs}
			response, err := application.FinalizeBlock(ctx, request)
			require.NoError(t, err)
			require.Len(t, response.TxResults, len(scenario.txs))
			for _, result := range response.TxResults {
				require.Equal(t, abci.CodeTypeOK, result.Code, result.Log)
			}
			notifier.Stash(request, response)
			_, err = application.Commit(ctx)
			require.NoError(t, err)
			block := &coretypes.ResultBlock{
				BlockID: tmtypes.BlockID{Hash: blockHash.Bytes()},
				Block:   &tmtypes.Block{Header: header, Data: data, LastCommit: &tmtypes.Commit{Height: height - 1}},
			}
			consensus.RecordCommittedBlock(block, &coretypes.ResultBlockResults{Height: height, TxsResults: response.TxResults})
			store := application.EvmKeeper.ReceiptStore()
			require.NotNil(t, store)
			require.NoError(t, store.SetLatestVersion(height))
			require.NoError(t, store.SetEarliestVersion(1))
			for _, raw := range scenario.txs {
				tx, err := application.GetTxConfig().TxDecoder()(raw)
				require.NoError(t, err)
				for _, message := range tx.GetMsgs() {
					if evm, ok := message.(*evmtypes.MsgEVMTransaction); ok {
						transaction, _ := evm.AsTransaction()
						require.NotNil(t, transaction)
						require.Eventually(t, func() bool {
							_, err := application.EvmKeeper.GetReceipt(provider(evmrpc.LatestCtxHeight), transaction.Hash())
							return err == nil
						}, 5*time.Second, time.Millisecond)
					}
				}
			}
			require.True(t, notifier.PublishStashed())
			var head map[string]interface{}
			select {
			case head = <-heads:
			case err := <-subscription.Err():
				t.Fatalf("newHeads subscription ended: %v", err)
			case <-ctx.Done():
				t.Fatal("committed header was not delivered:", ctx.Err())
			}
			require.Equal(t, blockHash.Hex(), head["hash"])
			for _, full := range []bool{false, true} {
				var served map[string]interface{}
				require.NoError(t, httpClient.CallContext(ctx, &served, "eth_getBlockByHash", blockHash, full))
				require.NotNil(t, served)
				require.Len(t, served["transactions"], scenario.evmCount)
				delete(served, "transactions")
				delete(served, "uncles")
				delete(served, "totalDifficulty")
				require.Equal(t, served, head, "every HTTP header field must match newHeads (fullTx=%t)", full)
			}
			require.Equal(t, hexutil.EncodeUint64(uint64(block.Block.Size())), head["size"])
			require.Equal(t, parentState.Hex(), head["stateRoot"])
			leaves, err := abci.MarshalTxResults(response.TxResults)
			require.NoError(t, err)
			require.Equal(t, common.BytesToHash(merkle.HashFromByteSlices(leaves)).Hex(), head["receiptsRoot"])
			if scenario.evmCount == 0 {
				require.Equal(t, ethtypes.EmptyTxsHash.Hex(), head["transactionsRoot"])
				require.NotEqual(t, common.BytesToHash(header.DataHash).Hex(), head["transactionsRoot"])
				if len(scenario.txs) == 0 {
					empty := sha256.Sum256(nil)
					require.Equal(t, empty[:], []byte(header.DataHash), "regression: e3b0c442 consensus empty root")
				} else {
					require.NotEmpty(t, block.Block.Txs, "regression: nonempty consensus block with empty Ethereum view")
				}
			} else {
				require.Equal(t, common.BytesToHash(header.DataHash).Hex(), head["transactionsRoot"])
				require.NotEqual(t, ethtypes.EmptyTxsHash.Hex(), head["transactionsRoot"])
				require.NotEqual(t, "0x0", head["gasUsed"])
				require.NotEqual(t, hexutil.Encode(make([]byte, ethtypes.BloomByteLength)), head["logsBloom"])
			}
			parentHash, parentState = blockHash, common.BytesToHash(response.AppHash)
		})
	}
}
