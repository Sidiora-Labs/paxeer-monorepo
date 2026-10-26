package evmrpc_test

import (
	"context"
	"encoding/json"
	"math"
	"math/big"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestPendingBlockCarriesHeaderFields(t *testing.T) {
	sender := utils.NewSigner()
	recipient := utils.NewSigner().EvmAddress
	s := newPendingSequencer(t, 2, sender)
	api := pendingBlockAPI(s)
	ctx := context.Background()
	snapshot, err := s.exec.Snapshot()
	require.NoError(t, err)
	parent, err := (&MockClient{latestOverride: s.committed}).Block(ctx, &s.committed)
	require.NoError(t, err)
	parentHash := common.BytesToHash(parent.BlockID.Hash)
	require.NotEqual(t, common.Hash{}, parentHash)
	miner := common.BytesToAddress(snapshot.BlockHeader().ProposerAddress)
	require.NotEqual(t, common.Address{}, miner)
	params := snapshot.ConsensusParams()
	require.NotNil(t, params)
	require.NotNil(t, params.Block)
	var gasLimit uint64
	if params.Block.MaxGas == -1 {
		gasLimit = math.MaxUint64
	} else {
		require.GreaterOrEqual(t, params.Block.MaxGas, int64(0))
		gasLimit = uint64(params.Block.MaxGas)
	}
	baseFee := s.app.EvmKeeper.GetNextBaseFeePerGas(s.ctxProvider(evmrpc.LatestCtxHeight)).TruncateInt().BigInt()
	wantHashes := []common.Hash{}
	var wantGas uint64
	var wantBloom ethtypes.Bloom
	check := func() {
		t.Helper()
		block, err := api.GetBlockByNumber(ctx, rpc.PendingBlockNumber, false)
		require.NoError(t, err)
		require.Len(t, block, 17)
		require.Equal(t, (*hexutil.Big)(big.NewInt(s.committed+1)), block["number"])
		require.Contains(t, block, "hash")
		require.Nil(t, block["hash"])
		require.Contains(t, block, "stateRoot")
		require.Nil(t, block["stateRoot"])
		require.Equal(t, hexutil.Uint64(snapshot.BlockTime().Unix()), block["timestamp"])
		require.Equal(t, wantHashes, block["transactions"])
		require.Equal(t, parentHash, block["parentHash"])
		require.Equal(t, miner, block["miner"])
		require.Equal(t, hexutil.Uint64(gasLimit), block["gasLimit"])
		require.Equal(t, (*hexutil.Big)(baseFee), block["baseFeePerGas"])
		require.Equal(t, hexutil.Uint64(wantGas), block["gasUsed"])
		require.Equal(t, wantBloom, block["logsBloom"])
		require.Equal(t, ethtypes.EmptyUncleHash, block["sha3Uncles"])
		require.Equal(t, ethtypes.BlockNonce{}, block["nonce"])
		require.Equal(t, (*hexutil.Big)(big.NewInt(0)), block["difficulty"])
		require.Equal(t, hexutil.Bytes{}, block["extraData"])
		require.Equal(t, common.Hash{}, block["mixHash"])
		require.Equal(t, []common.Hash{}, block["uncles"])
		encoded, err := json.Marshal(block)
		require.NoError(t, err)
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(encoded, &fields))
		require.JSONEq(t, "null", string(fields["hash"]))
		require.JSONEq(t, "null", string(fields["stateRoot"]))
		require.JSONEq(t, "[]", string(fields["uncles"]))
		for key, value := range map[string]string{
			"parentHash": parentHash.Hex(), "miner": hexutil.Encode(miner[:]),
			"gasLimit": hexutil.EncodeUint64(gasLimit), "gasUsed": hexutil.EncodeUint64(wantGas),
			"baseFeePerGas": hexutil.EncodeBig(baseFee), "logsBloom": hexutil.Encode(wantBloom[:]),
			"sha3Uncles": ethtypes.EmptyUncleHash.Hex(), "difficulty": "0x0",
			"extraData": "0x", "nonce": "0x0000000000000000", "mixHash": (common.Hash{}).Hex(),
		} {
			var got string
			require.NoError(t, json.Unmarshal(fields[key], &got), key)
			require.Equal(t, value, got, key)
		}
	}
	check()
	for nonce := uint64(0); nonce < 4; nonce++ {
		var tx []byte
		var hash common.Hash
		if nonce < 2 {
			tx = pendingTransfer(t, sender, nonce, recipient, big.NewInt(pendingOnePaxWei))
			hash = pendingBlockEthHash(t, s, tx)
		} else {
			topic := common.BigToHash(new(big.Int).SetUint64(nonce))
			code := append(append([]byte{0x7f}, topic.Bytes()...), 0x60, 0x00, 0x60, 0x00, 0xa1, 0x00)
			tx, hash = pendingFilterTx(t, sender, nonce, nil, big.NewInt(0), code, pendingFilterDeployGas)
		}
		accepted, err := s.exec.Accept(tx)
		require.NoError(t, err)
		require.Equal(t, abci.CodeTypeOK, accepted.Result.Code, accepted.Result.Log)
		snap, err := s.exec.Snapshot()
		require.NoError(t, err)
		receipt, err := s.app.EvmKeeper.GetTransientReceipt(snap, hash, uint64(accepted.TxIndex))
		require.NoError(t, err)
		require.Equal(t, uint32(ethtypes.ReceiptStatusSuccessful), receipt.Status, receipt.VmError)
		if nonce < 2 {
			require.Equal(t, uint64(pendingTransferGas), receipt.GasUsed)
			require.Empty(t, receipt.Logs)
		} else {
			require.Len(t, receipt.Logs, 1)
			require.Equal(t, crypto.CreateAddress(sender.EvmAddress, nonce).Hex(), receipt.Logs[0].Address)
			require.Equal(t, []string{common.BigToHash(new(big.Int).SetUint64(nonce)).Hex()}, receipt.Logs[0].Topics)
			require.NotEqual(t, ethtypes.Bloom{}, ethtypes.BytesToBloom(receipt.LogsBloom))
		}
		wantHashes = append(wantHashes, hash)
		wantGas += receipt.GasUsed
		require.Len(t, receipt.LogsBloom, ethtypes.BloomByteLength)
		for i, b := range receipt.LogsBloom {
			wantBloom[i] |= b
		}
		check()
	}
	_, err = api.GetBlockByNumber(ctx, rpc.PendingBlockNumber, true)
	require.EqualError(t, err, "pending block lists transaction hashes only")
}
