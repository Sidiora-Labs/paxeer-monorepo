package evmrpc_test

import (
	"context"
	"math/big"
	"sync/atomic"
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
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	receiptstore "github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

// TestEthclientOnTheChainPathSendsReceiptsAndFilters sends through a real HTTP
// server and consensus node. As in TestProofClientParity, the application's ante
// handler and EVM message server execute the envelopes actually committed by
// consensus; only their emitted receipts and blooms are published for RPC reads.
func TestEthclientOnTheChainPathSendsReceiptsAndFilters(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	application := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, application.Close()) })
	k := &application.EvmKeeper
	sdkctx := application.GetContextForDeliverTx(nil).WithBlockHeight(1).WithChainID("hyperpax_125-1").
		WithBlockTime(time.Now().UTC()).WithClosestUpgradeName(LatestCtxUpgradeName).
		WithConsensusParams(&tmproto.ConsensusParams{Block: &tmproto.BlockParams{MaxGas: 10000000}})
	fund := func(address sdk.AccAddress, amount sdk.Int) {
		coins := sdk.NewCoins(sdk.NewCoin(k.GetBaseDenom(sdkctx), amount))
		require.NoError(t, application.BankKeeper.MintCoins(sdkctx, "evm", coins))
		require.NoError(t, application.BankKeeper.SendCoinsFromModuleToAccount(sdkctx, "evm", address, coins))
	}
	operator := common.HexToAddress("0x2043")
	params := application.LayerXChainsKeeper.GetParams(sdkctx)
	fund(sdk.AccAddress(operator.Bytes()), params.MinBond.Add(params.CreationFee))
	domain, err := application.LayerXChainsKeeper.Create(sdkctx, operator, "ethclient-chain", params.MinGasAllowance,
		params.MinByteAllowance, 0, params.MinBond.Add(params.CreationFee), k.GetNextBaseFeePerGas(sdkctx))
	require.NoError(t, err)
	require.Equal(t, uint32(1), domain.Id)
	sender := utils.NewSigner()
	k.SetAddressMapping(sdkctx, sender.AccountAddress, sender.EvmAddress)
	fund(sender.AccountAddress, sdk.NewInt(1000000000))
	recipient := common.HexToAddress("0x204301")
	emitter := common.HexToAddress("0x204302")
	k.SetCode(sdkctx, emitter, common.FromHex("0x602a600052600160206000a100"))
	k.SetNonce(sdkctx, emitter, 1)

	conf, err := rpctest.CreateConfig(t, t.Name())
	require.NoError(t, err)
	genesis, err := tmtypes.GenesisDocFromFile(conf.GenesisFile())
	require.NoError(t, err)
	consensusApp := kvstore.NewApplication()
	consensusApp.SetValidators(genesis.ValidatorUpdates())
	nodeCtx, stopNode := context.WithCancel(ctx)
	node, err := tmnode.New(nodeCtx, conf, func() {}, consensusApp, nil, nil, tmnode.NoOpMetricsProvider(), tmtypes.DefaultConsensusPolicy())
	require.NoError(t, err)
	state, ok := node.RPCEnvironment().ConsensusState.Get()
	require.True(t, ok)
	signing, ok := state.(interface {
		SetPrivValidator(context.Context, tmutils.Option[tmtypes.PrivValidator])
	})
	require.True(t, ok)
	validator, err := privval.LoadFilePV(conf.PrivValidator.KeyFile(), conf.PrivValidator.StateFile())
	require.NoError(t, err)
	signing.SetPrivValidator(ctx, tmutils.Some[tmtypes.PrivValidator](validator))
	require.NoError(t, node.Start(nodeCtx))
	t.Cleanup(func() {
		stopNode()
		node.Wait()
		require.NoError(t, consensusApp.Close())
	})
	require.NoError(t, rpctest.WaitForRPC(ctx, conf))
	tm, err := rpclocal.New(node)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		status, err := tm.Status(ctx)
		require.NoError(t, err)
		return status.SyncInfo.LatestBlockHeight >= 1
	}, 20*time.Second, 20*time.Millisecond)
	initialStatus, err := tm.Status(ctx)
	require.NoError(t, err)
	initialHeight := initialStatus.SyncInfo.LatestBlockHeight
	var height atomic.Int64
	height.Store(initialHeight)
	provider := func(h int64) sdk.Context {
		if h == evmrpc.LatestCtxHeight {
			h = height.Load()
		}
		return sdkctx.WithBlockHeight(h)
	}
	txConfig := application.GetTxConfig()
	config := evmrpcconfig.DefaultConfig
	config.HTTPAddress, config.HTTPPort = TestAddr, 0
	config.IPRateLimitRPS = 0
	config.FilterTimeout = time.Minute
	config.TraceBakeEnabled = false
	serverService, err := evmrpc.NewEVMHTTPServer(config, tm, k, application.BeginBlockKeepers,
		application.BaseApp, application.TracerAnteHandler, provider, func(int64) client.TxConfig { return txConfig }, "", nil)
	require.NoError(t, err)
	t.Cleanup(serverService.Stop)
	require.NoError(t, serverService.Start())
	endpoint := "http://" + serverService.(*evmrpc.HTTPServer).ListenAddr()
	chain, err := ethclient.DialContext(ctx, endpoint+"/chain/1")
	require.NoError(t, err)
	t.Cleanup(chain.Close)
	root, err := ethclient.DialContext(ctx, endpoint+"/")
	require.NoError(t, err)
	t.Cleanup(root.Close)
	chainID, err := chain.ChainID(ctx)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(125000001), chainID)
	networkID, err := chain.NetworkID(ctx)
	require.NoError(t, err)
	require.Equal(t, chainID, networkID)
	rootID, err := root.ChainID(ctx)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(125), rootID)

	sign := func(id *big.Int, nonce uint64, to common.Address, value int64) *ethtypes.Transaction {
		t.Helper()
		tx, err := ethtypes.SignNewTx(sender.EvmPrivateKey, ethtypes.LatestSignerForChainID(id), &ethtypes.DynamicFeeTx{
			ChainID: id, Nonce: nonce, Gas: 100000, GasFeeCap: big.NewInt(1000000000000),
			GasTipCap: big.NewInt(0), To: &to, Value: big.NewInt(value),
		})
		require.NoError(t, err)
		return tx
	}
	transfer := sign(chainID, 0, recipient, 123)
	chainLog := sign(chainID, 1, emitter, 0)
	rootLog := sign(rootID, 2, emitter, 0)
	want := []*ethtypes.Transaction{transfer, chainLog, rootLog}
	wantHashes := make([]common.Hash, 0, len(want))
	for _, tx := range want {
		wantHashes = append(wantHashes, tx.Hash())
		require.NoError(t, chain.SendTransaction(ctx, tx))
	}

	// Discover the actual committed envelopes, including empty blocks between
	// sends. Submission does not promise a particular proposal or batch boundary.
	next := initialHeight + 1
	processed := 0
	committedHashes := make([]common.Hash, 0, len(want))
	blockHashes := make(map[int64]common.Hash)
	blockTxs := make(map[int64][]common.Hash)
	chainTxs := make(map[int64][]common.Hash)
	for processed < len(want) {
		require.Eventually(t, func() bool {
			status, err := tm.Status(ctx)
			require.NoError(t, err)
			return status.SyncInfo.LatestBlockHeight >= next
		}, 20*time.Second, 20*time.Millisecond)
		block, err := tm.Block(ctx, &next)
		require.NoError(t, err)
		next++
		if len(block.Block.Txs) == 0 {
			continue
		}
		blockHashes[block.Block.Height] = common.BytesToHash(block.BlockID.Hash)
		chainTxs[block.Block.Height] = []common.Hash{}
		sdkctx = sdkctx.WithBlockHeight(block.Block.Height).WithBlockTime(block.Block.Time)
		records := make([]receiptstore.ReceiptRecord, 0, len(want))
		blooms := make([]ethtypes.Bloom, 0, len(want))
		for i, envelope := range block.Block.Txs {
			decoded, err := txConfig.TxDecoder()(envelope)
			require.NoError(t, err)
			message := evmtypes.GetEVMTransactionMessage(decoded)
			require.NotNil(t, message)
			tx, _ := message.AsTransaction()
			require.NotNil(t, tx)
			require.Less(t, processed, len(want))
			// kvstore CheckTx does not supply EVM nonce metadata. Its mempool
			// orders equal-priority envelopes from a map, not by send order.
			require.Contains(t, wantHashes, tx.Hash())
			require.NotContains(t, committedHashes, tx.Hash())
			committedHashes = append(committedHashes, tx.Hash())
			processed++
			blockTxs[block.Block.Height] = append(blockTxs[block.Block.Height], tx.Hash())
			executionCtx := sdkctx.WithBlockHeight(block.Block.Height).WithBlockTime(block.Block.Time).
				WithTxIndex(i).WithIsEVM(true).WithEVMTxHash(tx.Hash())
			executionCtx, err = application.AnteHandler(executionCtx, decoded, false)
			require.NoError(t, err)
			id, scoped := chainstypes.DomainFrom(executionCtx)
			if tx.ChainId().Cmp(rootID) == 0 {
				require.False(t, scoped, "a root-signed transaction sent to the chain path must retain root routing")
			} else {
				require.True(t, scoped)
				require.Equal(t, uint32(1), id)
				chainTxs[block.Block.Height] = append(chainTxs[block.Block.Height], tx.Hash())
			}
			response, err := evmkeeper.NewMsgServerImpl(k).EVMTransaction(sdk.WrapSDKContext(executionCtx), message)
			require.NoError(t, err)
			require.Empty(t, response.VmError)
			receipt, err := k.GetTransientReceipt(executionCtx, tx.Hash(), uint64(i))
			require.NoError(t, err)
			require.Equal(t, uint32(ethtypes.ReceiptStatusSuccessful), receipt.Status)
			records = append(records, receiptstore.ReceiptRecord{TxHash: tx.Hash(), Receipt: receipt})
			blooms = append(blooms, ethtypes.BytesToBloom(receipt.LogsBloom))
		}
		require.NoError(t, k.ReceiptStore().SetReceipts(sdkctx, records))
		require.Eventually(t, func() bool {
			for _, record := range records {
				if _, err := k.GetReceipt(sdkctx, record.TxHash); err != nil {
					return false
				}
			}
			return true
		}, 5*time.Second, time.Millisecond)
		require.NoError(t, k.ReceiptStore().SetEarliestVersion(1))
		require.NoError(t, k.ReceiptStore().SetLatestVersion(block.Block.Height))
		k.SetBlockBloom(sdkctx, blooms)
		k.SetEvmOnlyBlockBloom(sdkctx, blooms)
		height.Store(block.Block.Height)

	}
	require.ElementsMatch(t, wantHashes, committedHashes)

	// Wait for every receipt through the standard client, including the
	// root-signed transaction submitted through /chain/1.
	receipts := make([]*ethtypes.Receipt, len(want))
	for i, tx := range want {
		reader := chain
		if tx.ChainId().Cmp(rootID) == 0 {
			reader = root
		}
		require.Eventually(t, func() bool {
			receipts[i], err = reader.TransactionReceipt(ctx, tx.Hash())
			if err == ethereum.NotFound {
				return false
			}
			require.NoError(t, err)
			return receipts[i] != nil
		}, 5*time.Second, time.Millisecond)
		receipt := receipts[i]
		require.Equal(t, tx.Hash(), receipt.TxHash)
		require.Equal(t, uint64(ethtypes.ReceiptStatusSuccessful), receipt.Status)
		require.Equal(t, blockHashes[receipt.BlockNumber.Int64()], receipt.BlockHash)
		returned, pending, err := reader.TransactionByHash(ctx, tx.Hash())
		require.NoError(t, err)
		require.False(t, pending)
		require.Equal(t, tx.Hash(), returned.Hash())
		encoded, err := tx.MarshalBinary()
		require.NoError(t, err)
		returnedBytes, err := returned.MarshalBinary()
		require.NoError(t, err)
		require.Equal(t, encoded, returnedBytes)
	}
	require.Empty(t, receipts[0].Logs)
	require.Len(t, receipts[1].Logs, 1)
	require.Len(t, receipts[2].Logs, 1)

	for number, hashes := range blockTxs {
		blockNumber := big.NewInt(number)
		chainBlock, err := chain.BlockByNumber(ctx, blockNumber)
		require.NoError(t, err)
		rootBlock, err := root.BlockByNumber(ctx, blockNumber)
		require.NoError(t, err)
		require.Equal(t, rootBlock.Hash(), chainBlock.Hash())
		require.Equal(t, blockNumber, chainBlock.Number())
		require.Equal(t, rootBlock.Number(), chainBlock.Number())
		require.Equal(t, rootBlock.ParentHash(), chainBlock.ParentHash())
		require.Len(t, rootBlock.Transactions(), len(hashes))
		require.Len(t, chainBlock.Transactions(), len(chainTxs[number]))
		for i, tx := range rootBlock.Transactions() {
			require.Equal(t, hashes[i], tx.Hash())
		}
		for i, tx := range chainBlock.Transactions() {
			require.Equal(t, chainTxs[number][i], tx.Hash())
		}
		if number == receipts[2].BlockNumber.Int64() {
			require.Greater(t, len(rootBlock.Transactions()), len(chainBlock.Transactions()))
		}

		// rpc/block.go:EncodeTmBlock publishes the consensus hash. The unmodified
		// ethclient Block.Hash recomputes an Ethereum header hash, so check the
		// wire identity against the receipts as well as both decoded headers.
		for _, reader := range []*ethclient.Client{chain, root} {
			var wire struct {
				Hash       common.Hash
				Number     *hexutil.Big
				ParentHash common.Hash
			}
			require.NoError(t, reader.Client().CallContext(ctx, &wire, "eth_getBlockByNumber", hexutil.EncodeBig(blockNumber), false))
			require.Equal(t, blockHashes[number], wire.Hash)
			require.Equal(t, blockNumber, (*big.Int)(wire.Number))
			require.Equal(t, rootBlock.ParentHash(), wire.ParentHash)
		}
		var count hexutil.Uint64
		require.NoError(t, chain.Client().CallContext(ctx, &count, "eth_getBlockTransactionCountByNumber", hexutil.EncodeBig(blockNumber)))
		require.Equal(t, uint64(len(chainTxs[number])), uint64(count))
		require.Equal(t, chainTxs[number], application.LayerXChainsKeeper.DomainTxs(sdkctx, 1, uint64(number)))
	}

	_, err = chain.TransactionReceipt(ctx, rootLog.Hash())
	require.ErrorIs(t, err, ethereum.NotFound)
	_, _, err = chain.TransactionByHash(ctx, rootLog.Hash())
	require.ErrorIs(t, err, ethereum.NotFound)

	query := ethereum.FilterQuery{FromBlock: receipts[0].BlockNumber, ToBlock: receipts[2].BlockNumber}
	rootLogs, err := root.FilterLogs(ctx, query)
	require.NoError(t, err)
	require.Len(t, rootLogs, 2)
	require.Equal(t, chainLog.Hash(), rootLogs[0].TxHash)
	require.Equal(t, rootLog.Hash(), rootLogs[1].TxHash)
	for _, query := range []ethereum.FilterQuery{query, {BlockHash: &receipts[1].BlockHash}} {
		logs, err := chain.FilterLogs(ctx, query)
		require.NoError(t, err)
		require.Len(t, logs, 1)
		require.Equal(t, rootLogs[0], logs[0])
		require.Equal(t, *receipts[1].Logs[0], logs[0])
		require.Equal(t, chainLog.Hash(), logs[0].TxHash)
		require.Equal(t, emitter, logs[0].Address)
		require.Equal(t, []common.Hash{common.BigToHash(big.NewInt(1))}, logs[0].Topics)
		require.Equal(t, common.LeftPadBytes([]byte{42}, 32), logs[0].Data)
		require.Equal(t, receipts[1].BlockHash, logs[0].BlockHash)
	}
	rootOnlyLogs, err := chain.FilterLogs(ctx, ethereum.FilterQuery{BlockHash: &receipts[2].BlockHash, Addresses: []common.Address{emitter}})
	require.NoError(t, err)
	if receipts[1].BlockHash == receipts[2].BlockHash {
		require.Equal(t, []ethtypes.Log{rootLogs[0]}, rootOnlyLogs)
	} else {
		require.Empty(t, rootOnlyLogs)
	}
	require.Equal(t, big.NewInt(123), k.GetBalance(sdkctx, k.GetPaxAddressOrDefault(sdkctx, recipient)))
}
