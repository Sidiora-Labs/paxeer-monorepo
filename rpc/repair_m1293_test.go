package evmrpc_test

// Task 22.33.1293 repair record (spec/ is outside this task's scope):
// legacy-batch-limit -> TestRepairM_1293_LegacyBatchLimit
// routing -> TestRepairM_1293_Routing
// trace-baker-server-lifecycle -> TestRepairM_1293_TraceBakerServerLifecycle
// subscription-server-lifecycle -> TestRepairM_1293_SubscriptionServerLifecycle, TestRepairM_1293_LegacySubscriptionLifecycle
// tenant-error-metadata -> TestRepairM_1293_TenantErrorMetadata
// tenant-txpool -> TestRepairM_1293_TenantTxPool
// Wiring request, domain 7/RPC worker owner: rpc/worker_pool.go must serialize Submit and SubmitWithMetrics admission with Close, then prove concurrent admission, refusal and draining; participant remains open because this path is outside SCOPE.
// Baseline verify was not run: the explicit bounded gate order forbids a
// preliminary verification before implementation and hf-build.
// Attempt 1 passed all six other cases; only TraceBakerServerLifecycle failed.

import (
	"encoding/json"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmnode "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/node"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	rpclocal "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/ante"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	rpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	receiptstore "github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/receipt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1293_LegacyBatchLimit(t *testing.T) { evmrpc.RunRepairM1293LegacyBatchLimit(t) }

func TestRepairM_1293_Routing(t *testing.T) {
	TestServedMethodsIncludesPromotedMethods(t)
	TestServedMethodsMatchServerDispatch(t)
}

func repair1293Config() rpcconfig.Config {
	config := rpcconfig.DefaultConfig
	config.HTTPAddress = TestAddr
	config.HTTPPort, config.WSPort, config.IPRateLimitRPS = 0, 0, 0
	config.FilterTimeout = time.Minute
	config.EnabledLegacyPaxApis = evmrpc.PaxLegacyAllGatedMethodNames()
	return config
}

func repair1293Consensus(t *testing.T) *rpclocal.Local {
	t.Helper()
	config, err := tmconfig.ResetTestRoot(t.TempDir(), "rpc-repair-1293")
	require.NoError(t, err)
	config.Mode, config.DBBackend = tmconfig.ModeFull, "memdb"
	config.RPC.ListenAddress, config.RPC.PprofListenAddress = "", ""
	config.P2P.ListenAddress = "tcp://127.0.0.1:0"
	config.P2P.PersistentPeers, config.P2P.BootstrapPeers, config.P2P.BlockSyncPeers = "", "", ""
	application := kvstore.NewApplication()
	node, err := tmnode.New(t.Context(), config, func() {}, application, nil, nil,
		tmnode.NoOpMetricsProvider(), tmtypes.DefaultConsensusPolicy())
	require.NoError(t, err)
	// Construction wires the real mempool, proxy, event bus and stores. No
	// consensus is started, so the submitted inventory cannot be committed away.
	t.Cleanup(func() {
		node.(interface{ OnStop() }).OnStop()
		require.NoError(t, application.Close())
	})
	consensus, err := rpclocal.New(node)
	require.NoError(t, err)
	return consensus
}

func TestRepairM_1293_TenantTxPool(t *testing.T) {
	h := newDomainScopeHarness(t)
	t.Cleanup(func() { require.NoError(t, h.app.Close()) })
	require.Equal(t, uint32(2), h.create(t, "pool-second").Id)
	consensus := repair1293Consensus(t)
	var hashes []common.Hash
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	for i, chainID := range []*big.Int{big.NewInt(125), chainstypes.ChainIDOf(1), chainstypes.ChainIDOf(1), chainstypes.ChainIDOf(2)} {
		tx, err := ethtypes.SignNewTx(key, ethtypes.LatestSignerForChainID(chainID), &ethtypes.DynamicFeeTx{
			ChainID: chainID, Nonce: uint64(i), Gas: 21000, GasFeeCap: big.NewInt(1000000000000), GasTipCap: big.NewInt(0), To: &h.operator,
		})
		require.NoError(t, err)
		data, err := ethtx.NewDynamicFeeTx(tx)
		require.NoError(t, err)
		msg, err := evmtypes.NewMsgEVMTransaction(data)
		require.NoError(t, err)
		builder := TxConfig.NewTxBuilder()
		require.NoError(t, builder.SetMsgs(msg))
		encoded, err := Encoder(builder.GetTx())
		require.NoError(t, err)
		accepted, err := consensus.BroadcastTx(t.Context(), encoded)
		require.NoError(t, err)
		require.Equal(t, abci.CodeTypeOK, accepted.Code, accepted.Log)
		hashes = append(hashes, tx.Hash())
	}
	// Non-EVM and undecodable entries must not affect either pool method.
	bankSend := &banktypes.MsgSend{
		FromAddress: sdk.AccAddress(h.operator.Bytes()).String(),
		ToAddress:   sdk.AccAddress(h.operator.Bytes()).String(),
		Amount:      sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1)),
	}
	for _, encoded := range []tmtypes.Tx{mustRepair1293Encode(t, bankSend), []byte("undecodable")} {
		accepted, err := consensus.BroadcastTx(t.Context(), encoded)
		require.NoError(t, err)
		require.Equal(t, abci.CodeTypeOK, accepted.Code)
	}
	// The RPC reads the snapshot last published by proposal reaping. Exercise
	// that real producer without removing any of the submitted inventory.
	mempool, ok := consensus.Mempool.Get()
	require.True(t, ok)
	require.Len(t, repair1293ReapAll(mempool.ReapTxs), 6)
	require.Eventually(t, func() bool {
		inventory, err := consensus.UnconfirmedTxs(t.Context(), nil, nil)
		return err == nil && len(inventory.Txs) == 6
	}, 5*time.Second, time.Millisecond)
	provider := func(height int64) sdk.Context {
		if height == evmrpc.LatestCtxHeight {
			return h.ctx
		}
		return h.ctx.WithBlockHeight(height)
	}
	txProvider := func(int64) client.TxConfig { return TxConfig }
	config := repair1293Config()
	httpService, err := evmrpc.NewEVMHTTPServer(config, consensus, &h.app.EvmKeeper, h.app.BeginBlockKeepers,
		h.app.BaseApp, h.app.TracerAnteHandler, provider, txProvider, "", nil)
	require.NoError(t, err)
	t.Cleanup(httpService.Stop)
	require.NoError(t, httpService.Start())
	wsService, err := evmrpc.NewEVMWebSocketServer(config, consensus, &h.app.EvmKeeper, h.app.BeginBlockKeepers,
		h.app.BaseApp, h.app.TracerAnteHandler, provider, txProvider, "", nil, evmrpc.NewBlockHeaderNotifier(4))
	require.NoError(t, err)
	t.Cleanup(wsService.Stop)
	require.NoError(t, wsService.Start())
	for _, view := range repair339Views(t, httpService.(*evmrpc.HTTPServer), wsService.(*evmrpc.HTTPServer)) {
		for _, consumer := range []*rpc.Client{view.http, view.ws} {
			var content map[string]map[string]map[string]struct {
				Hash  common.Hash
				From  common.Address
				Nonce hexutil.Uint64
			}
			require.NoError(t, consumer.CallContext(t.Context(), &content, "txpool_content"))
			require.Empty(t, content["queued"])
			require.Len(t, content["pending"], 1, view.path)
			var visible []common.Hash
			for sender, transactions := range content["pending"] {
				for nonce, tx := range transactions {
					require.Equal(t, crypto.PubkeyToAddress(key.PublicKey), tx.From)
					require.Equal(t, strings.ToLower(sender), strings.ToLower(tx.From.Hex()))
					require.Equal(t, strconv.FormatUint(uint64(tx.Nonce), 10), nonce)
					visible = append(visible, tx.Hash)
				}
			}
			var want []common.Hash
			for _, i := range view.indices {
				want = append(want, hashes[i])
			}
			require.ElementsMatch(t, want, visible, view.path)
			var status map[string]hexutil.Uint
			require.NoError(t, consumer.CallContext(t.Context(), &status, "txpool_status"))
			require.Equal(t, map[string]hexutil.Uint{"pending": hexutil.Uint(len(visible)), "queued": 0}, status)
		}
	}
}

func mustRepair1293Encode(t *testing.T, message sdk.Msg) []byte {
	t.Helper()
	builder := TxConfig.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(message))
	encoded, err := Encoder(builder.GetTx())
	require.NoError(t, err)
	return encoded
}

// Infer the consensus pool's limits type without importing its internal
// package. Zero limits impose no proposal bounds; false retains the inventory.
func repair1293ReapAll[T any](reap func(T, bool) (tmtypes.Txs, int64)) tmtypes.Txs {
	var limits T
	txs, _ := reap(limits, false)
	return txs
}

func TestRepairM_1293_TenantErrorMetadata(t *testing.T) {
	h := newDomainWSHarness(t)
	h.ws.Stop()
	k := &h.app.EvmKeeper
	contract := common.HexToAddress("0x123456")
	k.SetCode(h.ctx, contract, common.FromHex("0x60006000fd"))
	previous := []sdk.Tx{MultiTxBlockTx1, MultiTxBlockTx2, MultiTxBlockTx3, Block2SyntheticTx}
	t.Cleanup(func() {
		MultiTxBlockTx1, MultiTxBlockTx2, MultiTxBlockTx3, Block2SyntheticTx = previous[0], previous[1], previous[2], previous[3]
	})
	var transactions []sdk.Tx
	var records []receiptstore.ReceiptRecord
	h.hashes = nil
	for i, chainID := range []*big.Int{big.NewInt(125), chainstypes.ChainIDOf(1), chainstypes.ChainIDOf(1), chainstypes.ChainIDOf(2)} {
		key, err := crypto.GenerateKey()
		require.NoError(t, err)
		from := crypto.PubkeyToAddress(key.PublicKey)
		coins := sdk.NewCoins(sdk.NewCoin(k.GetBaseDenom(h.ctx), sdk.NewInt(10000000)))
		require.NoError(t, h.app.BankKeeper.MintCoins(h.ctx, "evm", coins))
		require.NoError(t, h.app.BankKeeper.SendCoinsFromModuleToAccount(h.ctx, "evm", sdk.AccAddress(from.Bytes()), coins))
		tx, err := ethtypes.SignNewTx(key, ethtypes.LatestSignerForChainID(chainID), &ethtypes.DynamicFeeTx{
			ChainID: chainID, Gas: 100000, GasFeeCap: big.NewInt(1000000000000), GasTipCap: big.NewInt(0), To: &contract,
		})
		require.NoError(t, err)
		data, err := ethtx.NewDynamicFeeTx(tx)
		require.NoError(t, err)
		msg, err := evmtypes.NewMsgEVMTransaction(data)
		require.NoError(t, err)
		builder := TxConfig.NewTxBuilder()
		require.NoError(t, builder.SetMsgs(msg))
		position := []int{1, 3, 4, 5}[i]
		execution := h.ctx.WithTxIndex(position).WithIsEVM(true).WithEVMNonce(0).WithEVMSenderAddress(from).WithEVMTxHash(tx.Hash())
		require.NoError(t, ante.Preprocess(execution, msg, chainID, false))
		execution, err = ante.NewEVMFeeCheckDecorator(k, &h.app.UpgradeKeeper).AnteHandle(execution, builder.GetTx(), false,
			func(next sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) { return next, nil })
		require.NoError(t, err)
		response, err := evmkeeper.NewMsgServerImpl(k).EVMTransaction(sdk.WrapSDKContext(execution), msg)
		require.NoError(t, err)
		require.Equal(t, "execution reverted", response.VmError)
		receipt, err := k.GetTransientReceipt(execution, tx.Hash(), uint64(position))
		require.NoError(t, err)
		require.Equal(t, response.VmError, receipt.VmError)
		records = append(records, receiptstore.ReceiptRecord{TxHash: tx.Hash(), Receipt: receipt})
		transactions = append(transactions, builder.GetTx())
		h.hashes = append(h.hashes, tx.Hash())
	}
	MultiTxBlockTx1, MultiTxBlockTx2, MultiTxBlockTx3, Block2SyntheticTx = transactions[0], transactions[1], transactions[2], transactions[3]
	require.NoError(t, k.ReceiptStore().SetReceipts(h.ctx, records))
	require.Eventually(t, func() bool {
		for _, hash := range h.hashes {
			if _, err := k.GetReceipt(h.ctx, hash); err != nil {
				return false
			}
		}
		return true
	}, 5*time.Second, time.Millisecond)
	provider := func(height int64) sdk.Context {
		if height == evmrpc.LatestCtxHeight {
			return h.ctx
		}
		return h.ctx.WithBlockHeight(height)
	}
	service, err := evmrpc.NewEVMHTTPServer(repair1293Config(), &MockClient{}, k, h.app.BeginBlockKeepers,
		h.app.BaseApp, h.app.TracerAnteHandler, provider, func(int64) client.TxConfig { return TxConfig }, "", nil)
	require.NoError(t, err)
	t.Cleanup(service.Stop)
	server := service.(*evmrpc.HTTPServer)
	evmrpc.RepairM1293EnableHTTPAPIsWS(t, server)
	require.NoError(t, server.Start())
	for _, view := range repair339Views(t, server, server) {
		for _, consumer := range []*rpc.Client{view.http, view.ws} {
			for _, namespace := range []string{"eth", "pax"} {
				var missing string
				missingErr := consumer.CallContext(t.Context(), &missing, namespace+"_getVMError", common.HexToHash("0xdead"))
				require.Error(t, missingErr)
				for i, hash := range h.hashes {
					var receipt map[string]json.RawMessage
					require.NoError(t, consumer.CallContext(t.Context(), &receipt, namespace+"_getTransactionReceipt", hash))
					if view.allows(i) {
						require.NotNil(t, receipt)
					} else {
						require.Nil(t, receipt)
					}
					var vmError, transactionError string
					err := consumer.CallContext(t.Context(), &vmError, namespace+"_getVMError", hash)
					require.NoError(t, consumer.CallContext(t.Context(), &transactionError, namespace+"_getTransactionErrorByHash", hash))
					if view.allows(i) {
						require.NoError(t, err)
						require.Equal(t, "execution reverted", vmError)
						require.Equal(t, vmError, transactionError)
					} else {
						require.EqualError(t, err, missingErr.Error())
						require.Empty(t, vmError)
						require.Empty(t, transactionError)
					}
				}
				require.NoError(t, consumer.CallContext(t.Context(), &missing, namespace+"_getTransactionErrorByHash", common.HexToHash("0xdead")))
				require.Empty(t, missing)
			}
		}
	}
}

func TestRepairM_1293_SubscriptionServerLifecycle(t *testing.T) {
	h := newRepair185Harness(t, 100)
	notifier := evmrpc.NewBlockHeaderNotifier(4)
	for round := 0; round < 3; round++ {
		service, err := evmrpc.NewEVMWebSocketServer(repair1293Config(), h.tm, &h.s.app.EvmKeeper, h.s.app.BeginBlockKeepers,
			h.s.app.BaseApp, h.s.app.TracerAnteHandler, h.s.ctxProvider, func(int64) client.TxConfig { return h.s.app.GetTxConfig() }, "", nil, notifier)
		require.NoError(t, err)
		t.Cleanup(service.Stop)
		require.NoError(t, service.Start())
		server := service.(*evmrpc.HTTPServer)
		assertStopped := evmrpc.RepairM1293SubscriptionStopped(t, server)
		consumer := domainWSClient(t, server, "/")
		heads := make(chan map[string]json.RawMessage, 4)
		subscription, err := consumer.EthSubscribe(t.Context(), heads, "newHeads")
		require.NoError(t, err)
		var logSubscription *rpc.ClientSubscription
		for n := 0; n < 3; n++ {
			repair1293Commit(t, h)
			height := h.s.committed
			block := h.blocks[height].Block
			notifier.Stash(&abci.RequestFinalizeBlock{Hash: h.blocks[height].BlockID.Hash, Header: block.Header.ToProto()}, &abci.ResponseFinalizeBlock{AppHash: block.AppHash})
			require.True(t, notifier.PublishStashed())
			select {
			case head := <-heads:
				require.JSONEq(t, `"`+hexutil.EncodeUint64(uint64(height))+`"`, string(head["number"]))
				require.JSONEq(t, `"`+common.BytesToHash(h.blocks[height].BlockID.Hash).Hex()+`"`, string(head["hash"]))
			case err := <-subscription.Err():
				t.Fatalf("active subscription stopped: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("replacement server lost committed head")
			}
			if n == 0 {
				logs := make(chan json.RawMessage, 4)
				logSubscription, err = consumer.EthSubscribe(t.Context(), logs, "logs", map[string]any{"fromBlock": hexutil.EncodeUint64(uint64(height))})
				require.NoError(t, err)
				select {
				case log := <-logs:
					require.Contains(t, string(log), h.hashes[height][0].Hex())
				case err := <-logSubscription.Err():
					t.Fatalf("log consumer ended: %v", err)
				case <-time.After(5 * time.Second):
					t.Fatal("log consumer did not receive committed receipt")
				}
			}
		}
		service.Stop()
		assertStopped()
		select {
		case <-logSubscription.Err():
		case <-time.After(time.Second):
			t.Fatal("old log consumer stayed alive")
		}
		logSubscription.Unsubscribe()
		select {
		case <-subscription.Err():
		case <-time.After(time.Second):
			t.Fatal("old websocket consumer stayed alive")
		}
		subscription.Unsubscribe()
		consumer.Close()
	}
}

func TestRepairM_1293_LegacySubscriptionLifecycle(t *testing.T) {
	consensus := repair1293Consensus(t)
	// EventBus.Start is sufficient; consensus and network services are unused.
	require.NoError(t, consensus.EventBus.Start(t.Context()))
	h := newDomainScopeHarness(t)
	t.Cleanup(func() { require.NoError(t, h.app.Close()) })
	for round := 0; round < 3; round++ {
		service, err := evmrpc.NewEVMWebSocketServer(repair1293Config(), consensus, &h.app.EvmKeeper, h.app.BeginBlockKeepers,
			h.app.BaseApp, h.app.TracerAnteHandler, func(int64) sdk.Context { return h.ctx }, func(int64) client.TxConfig { return TxConfig }, "", nil, nil)
		require.NoError(t, err)
		t.Cleanup(service.Stop)
		require.NoError(t, service.Start())
		assertStopped := evmrpc.RepairM1293SubscriptionStopped(t, service.(*evmrpc.HTTPServer))
		service.Stop()
		assertStopped()
		require.Zero(t, consensus.EventBus.NumClients(), "old server retained a consensus subscription")
	}
}

func TestRepairM_1293_TraceBakerServerLifecycle(t *testing.T) {
	h := newRepair185Harness(t, 100)
	validators, err := h.tm.StateStore.LoadValidators(1)
	require.NoError(t, err)
	require.Len(t, validators.Validators, 1)
	// The two fixture applications have independent genesis keys. Commit the
	// real consensus signer to staking before replay needs its distribution
	// and slashing records; a validator set alone is insufficient for replay.
	repair1293Commit(t, h, func(ctx sdk.Context) {
		pubkey, err := cryptocodec.FromTmPubKeyInterface(validators.Validators[0].PubKey)
		require.NoError(t, err)
		operator := sdk.ValAddress(pubkey.Address())
		bond := sdk.NewCoin(h.s.app.StakingKeeper.BondDenom(ctx), sdk.DefaultPowerReduction)
		require.NoError(t, h.s.app.BankKeeper.SendCoins(ctx, h.sender.AccountAddress, sdk.AccAddress(operator), sdk.NewCoins(bond)))
		rate := h.s.app.StakingKeeper.MinCommissionRate(ctx)
		msg, err := stakingtypes.NewMsgCreateValidator(operator, pubkey, bond,
			stakingtypes.NewDescription("trace-baker-validator", "", "", "", ""),
			stakingtypes.NewCommissionRates(rate, rate, sdk.ZeroDec()), sdk.OneInt())
		require.NoError(t, err)
		_, err = stakingkeeper.NewMsgServerImpl(h.s.app.StakingKeeper).CreateValidator(sdk.WrapSDKContext(ctx), msg)
		require.NoError(t, err)
		_, err = h.s.app.StakingKeeper.ApplyAndReturnValidatorSetUpdates(ctx)
		require.NoError(t, err)
		_, found := h.s.app.SlashingKeeper.GetValidatorSigningInfo(ctx, sdk.ConsAddress(validators.Validators[0].Address))
		require.True(t, found)
	})
	repair1293Commit(t, h)
	// Replaying block 3 requires block 2's committed results and validator set.
	require.NoError(t, h.tm.StateStore.SaveValidatorSets(1, 3, validators))
	collect := evmrpc.RepairM1293Metrics(t)
	directory := t.TempDir()
	cache, err := evmkeeper.NewTraceDB(directory)
	require.NoError(t, err)
	h.s.app.EvmKeeper.SetTraceDB(cache)
	t.Cleanup(func() { h.s.app.EvmKeeper.SetTraceDB(nil); require.NoError(t, cache.Close()) })
	config := repair1293Config()
	config.TraceBakeEnabled, config.TraceBakeWorkers, config.TraceBakeQueueSize, config.TraceBakeWindowBlocks = true, 1, 8, 0
	provider := func(height int64) sdk.Context {
		if height == evmrpc.LatestCtxHeight {
			return h.s.ctxProvider(height)
		}
		store, err := h.s.app.CommitMultiStore().CacheMultiStoreWithVersion(height)
		if err != nil {
			panic(err)
		}
		ctx := sdk.NewContext(store, tmproto.Header{ChainID: pendingChainID, Height: height}, false).WithIsTracing(true)
		return ctx.WithConsensusParams(h.s.app.GetConsensusParams(ctx))
	}
	start := func() *evmrpc.HTTPServer {
		service, err := evmrpc.NewEVMHTTPServer(config, h.tm, &h.s.app.EvmKeeper, h.s.app.BeginBlockKeepers,
			h.s.app.BaseApp, h.s.app.TracerAnteHandler, provider, func(int64) client.TxConfig { return h.s.app.GetTxConfig() }, "", nil)
		require.NoError(t, err)
		t.Cleanup(service.Stop)
		require.NoError(t, service.Start())
		return service.(*evmrpc.HTTPServer)
	}
	old := start()
	oldServerStopped := evmrpc.RepairM1293ServerStopped(t, old)
	baker, stopped := evmrpc.RepairM1293Baker(t, cache)
	var live json.RawMessage
	require.NoError(t, domainHTTPClient(t, old, "/").CallContext(t.Context(), &live,
		"debug_traceBlockByNumber", "0x3", map[string]any{"tracer": "callTracer"}))
	cache.Enqueue(3)
	require.Eventually(t, func() bool { return baker.BakedCount() >= 1 }, 5*time.Second, time.Millisecond)
	payload, found, err := cache.GetBlock(3, "callTracer")
	require.NoError(t, err)
	require.True(t, found, "real baker must persist the traced transaction")
	var traces []struct {
		TxHash common.Hash
		Result json.RawMessage
	}
	require.NoError(t, json.Unmarshal(payload, &traces))
	require.Len(t, traces, 1)
	require.Equal(t, h.hashes[3][0], traces[0].TxHash)
	require.NotEmpty(t, traces[0].Result)
	require.Equal(t, int64(1), collect()["evmrpc_trace_bake_baked_total"])
	// Start a replacement while the old owner exists, then stop the old owner.
	replacement := start()
	replacementServerStopped := evmrpc.RepairM1293ServerStopped(t, replacement)
	newBaker, newStopped := evmrpc.RepairM1293Baker(t, cache)
	require.NotSame(t, baker, newBaker)
	old.Stop()
	oldServerStopped()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("old server did not stop its baker")
	}
	cache.Enqueue(3)
	require.Eventually(t, func() bool { return newBaker.BakedCount() >= 1 }, 5*time.Second, time.Millisecond)
	require.Equal(t, int64(1), collect()["evmrpc_trace_bake_baked_total"], "old callback detach removed replacement metrics")
	consumer := domainHTTPClient(t, replacement, "/")
	var served json.RawMessage
	require.NoError(t, consumer.CallContext(t.Context(), &served, "debug_traceBlockByNumber", "0x3", map[string]any{"tracer": "callTracer"}))
	require.JSONEq(t, string(payload), string(served))
	replacement.Stop()
	replacementServerStopped()
	select {
	case <-newStopped:
	case <-time.After(time.Second):
		t.Fatal("replacement baker stayed alive")
	}
	require.NotContains(t, collect(), "evmrpc_trace_bake_baked_total")
	require.NoError(t, cache.Close())
	cache, err = evmkeeper.NewTraceDB(directory)
	require.NoError(t, err)
	h.s.app.EvmKeeper.SetTraceDB(cache)
	recovered, found, err := cache.GetBlock(3, "callTracer")
	require.NoError(t, err)
	require.True(t, found)
	require.JSONEq(t, string(payload), string(recovered))
	restarted := start()
	restartedServerStopped := evmrpc.RepairM1293ServerStopped(t, restarted)
	restartedBaker, restartedStopped := evmrpc.RepairM1293Baker(t, cache)
	cache.Enqueue(3)
	require.Eventually(t, func() bool { return restartedBaker.BakedCount() >= 1 }, 5*time.Second, time.Millisecond)
	require.Equal(t, uint64(1), baker.BakedCount(), "stopped baker consumed replacement work")
	require.Equal(t, int64(1), collect()["evmrpc_trace_bake_baked_total"])
	restarted.Stop()
	restartedServerStopped()
	select {
	case <-restartedStopped:
	case <-time.After(time.Second):
		t.Fatal("restarted baker stayed alive")
	}
}

func repair1293Commit(t *testing.T, h *repair185Harness, beforeCommit ...func(sdk.Context)) {
	t.Helper()
	height := h.s.committed + 1
	encoded, hash := pendingFilterDeploy(t, h.sender, h.nonce)
	h.nonce++
	accepted, err := h.s.exec.Accept(encoded)
	require.NoError(t, err)
	require.Equal(t, abci.CodeTypeOK, accepted.Result.Code, accepted.Result.Log)
	batch, err := h.s.exec.Seal(height)
	require.NoError(t, err)
	for _, prepare := range beforeCommit {
		prepare(batch.Ctx)
	}
	require.NoError(t, h.s.app.EvmKeeper.FlushTransientReceipts(batch.Ctx))
	batch.Store.Write()
	require.Equal(t, height, h.s.app.CommitMultiStore().Commit(true).Version)
	h.s.committed = height
	require.Eventually(t, func() bool {
		receipt, err := h.s.app.EvmKeeper.GetReceipt(h.s.ctxProvider(height), hash)
		return err == nil && receipt.BlockNumber == uint64(height)
	}, 5*time.Second, time.Millisecond)
	require.NoError(t, h.s.app.EvmKeeper.ReceiptStore().SetLatestVersion(height))
	h.saveBlock(height, batch.Txs, batch.Ctx.BlockTime())
	h.hashes[height] = []common.Hash{hash}
	require.NoError(t, h.tm.StateStore.SaveFinalizeBlockResponses(height, &abci.ResponseFinalizeBlock{
		TxResults: batch.Results, AppHash: h.s.app.LastCommitID().Hash, ConsensusParamUpdates: batch.Ctx.ConsensusParams(),
	}))
	require.NoError(t, h.s.exec.Open(height+1, batch.TimestampMs+1000))
}
