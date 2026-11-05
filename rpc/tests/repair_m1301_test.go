package tests

import (
	"bytes"
	"fmt"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/merkle"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1301_FixtureBlockMetadata(t *testing.T) {
	for _, appendBlocks := range []bool{false, true} {
		t.Run(fmt.Sprintf("append_%t", appendBlocks), func(t *testing.T) {
			txData := send(0)
			tx := signTxWithMnemonic(txData, mnemonic1)
			txBytes := encodeEvmTx(txData, tx)
			blocks := [][][]byte{{txBytes}}
			if appendBlocks {
				blocks = nil
			}
			ts := SetupTestServer(t, blocks, mnemonicInitializer(mnemonic1))
			if appendBlocks {
				previousCommit := bytes.Clone(ts.app.LastCommitID().Hash)
				ts.SetupBlocks([][][]byte{{txBytes}})
				require.Equal(t, previousCommit, []byte(ts.mockClient.headers[1].AppHash))
			}

			producer := ts.app.GetCheckCtx().BlockHeader()
			producerHash := bytes.Clone(ts.app.LastBlockID().Hash)
			committedHash := bytes.Clone(ts.app.LastCommitID().Hash)
			require.Equal(t, int64(2), producer.Height)
			require.Equal(t, "pax-test", producer.ChainID)
			require.NotEmpty(t, producerHash)
			require.NotEmpty(t, producer.AppHash)
			require.Equal(t, committedHash, ts.mockClient.committedAppHashes[1])
			results, err := abci.MarshalTxResults(ts.mockClient.txResults[1])
			require.NoError(t, err)
			resultHash := merkle.HashFromByteSlices(results)
			require.Equal(t, resultHash, ts.mockClient.committedResultsHashes[1])
			storedReceipt, err := ts.app.EvmKeeper.GetReceipt(ts.app.GetCheckCtx(), tx.Hash())
			require.NoError(t, err)
			require.Equal(t, uint64(2), storedReceipt.BlockNumber)
			require.Equal(t, uint32(1), storedReceipt.Status)

			// Advance the real producer so historical reads must retain block 2's metadata.
			ts.SetupBlocks([][][]byte{{}})
			nextProducer := ts.app.GetCheckCtx().BlockHeader()
			require.Equal(t, committedHash, nextProducer.AppHash)
			require.Equal(t, resultHash, nextProducer.LastResultsHash)
			require.Equal(t, producerHash, nextProducer.LastBlockId.Hash)
			for _, height := range []int64{1, 2, 3} {
				block, err := ts.mockClient.Block(t.Context(), &height)
				require.NoError(t, err)
				require.NotNil(t, block)
				header, err := ts.mockClient.Header(t.Context(), &height)
				require.NoError(t, err)
				byHash, err := ts.mockClient.BlockByHash(t.Context(), block.BlockID.Hash)
				require.NoError(t, err)
				require.Equal(t, block.Block.Header, *header.Header)
				require.Equal(t, block.Block.Header, byHash.Block.Header)
				require.Equal(t, block.BlockID.Hash, header.Header.Hash())
				if height == 2 {
					require.Equal(t, &producer, header.Header.ToProto())
					require.Equal(t, producerHash, []byte(block.BlockID.Hash))
					require.Equal(t, tmtypes.Txs{txBytes}, block.Block.Txs)
				}
			}
			latest, err := ts.mockClient.Header(t.Context(), nil)
			require.NoError(t, err)
			require.Equal(t, &nextProducer, latest.Header.ToProto())

			require.NoError(t, ts.Start())
			defer ts.Stop()
			rpcBlockHash := common.BytesToHash(producerHash).Hex()
			for _, lookup := range []struct {
				method string
				arg    string
			}{{"getBlockByNumber", "0x2"}, {"getBlockByHash", rpcBlockHash}} {
				response := sendRequestWithNamespace("eth", ts.port, lookup.method, lookup.arg, true)
				require.NotContains(t, response, "error")
				block, ok := response["result"].(map[string]interface{})
				require.True(t, ok, "%v", response)
				require.Equal(t, rpcBlockHash, block["hash"])
				require.Equal(t, common.BytesToHash(producer.AppHash).Hex(), block["stateRoot"])
				require.Equal(t, hexutil.EncodeUint64(uint64(producer.Time.Unix())), block["timestamp"])
				require.Equal(t, common.BytesToHash(producer.LastBlockId.Hash).Hex(), block["parentHash"])
				txs, ok := block["transactions"].([]interface{})
				require.True(t, ok)
				require.Len(t, txs, 1)
				transaction := txs[0].(map[string]interface{})
				require.Equal(t, tx.Hash().Hex(), transaction["hash"])
				require.Equal(t, rpcBlockHash, transaction["blockHash"])
				require.Equal(t, hexutil.EncodeBig(ts.app.EvmKeeper.ChainID(ts.app.GetCheckCtx())), transaction["chainId"])
			}
			response := sendRequestWithNamespace("eth", ts.port, "getTransactionReceipt", tx.Hash().Hex())
			require.NotContains(t, response, "error")
			receipt, ok := response["result"].(map[string]interface{})
			require.True(t, ok, "%v", response)
			require.Equal(t, rpcBlockHash, receipt["blockHash"])
			require.Equal(t, hexutil.EncodeUint64(storedReceipt.BlockNumber), receipt["blockNumber"])
			require.Equal(t, "0x1", receipt["status"])
			chain := sendRequestWithNamespace("eth", ts.port, "chainId")
			require.NotContains(t, chain, "error")
			require.Equal(t, hexutil.EncodeBig(ts.app.EvmKeeper.ChainID(ts.app.GetCheckCtx())), chain["result"])
		})
	}
}
