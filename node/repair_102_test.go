package app_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/merkle"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/provider"
	lightprovider "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/provider/http"
	lightrpc "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/rpc"
	lightstore "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/store/db"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramstypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

var repair102Time = time.Unix(1_800_000_000, 0).UTC()

type repair102Keys struct {
	key       *sequencer.ReceiptKey
	public    []byte
	proposer  []byte
	pins      seqkeeper.ReceiptKeys
	validator *privval.FilePV
}

func repair102SigningKeys(t *testing.T) repair102Keys {
	t.Helper()
	dir := t.TempDir()
	consensus := filepath.Join(dir, "consensus.json")
	pv, err := privval.GenFilePV(consensus, filepath.Join(dir, "consensus-state.json"), "")
	require.NoError(t, err)
	require.NoError(t, pv.Save())
	pub, err := pv.GetPubKey(context.Background())
	require.NoError(t, err)
	file := filepath.Join(dir, "receipt-key")
	require.NoError(t, os.WriteFile(file, []byte(hex.EncodeToString(sequencerKeySeed)), 0600))
	pins := seqkeeper.ReceiptKeys{{KeyId: sequencerKeyID, PublicKey: sequencerPublicKey(), FirstHeight: 1, LastHeight: math.MaxUint64}}
	key, err := sequencer.LoadReceiptKey(file, consensus, []verify.SequencerAuthorization{{
		SequencerID: [32]byte(sequencerKeyID), PublicKey: [32]byte(sequencerPublicKey()), FirstBatchNumber: 1, LastBatchNumber: math.MaxUint64,
	}}, 2)
	require.NoError(t, err)
	return repair102Keys{key: key, public: pub.Bytes(), proposer: pub.Address(), pins: pins, validator: pv}
}

type repair102Chain struct {
	a              *app.App
	home, database string
	keys           repair102Keys
	close          func()
}

func (h *repair102Chain) open(t *testing.T) {
	t.Helper()
	db, err := dbm.NewGoLevelDB("application", h.database)
	require.NoError(t, err)
	h.a = app.New(db, nil, true, map[int64]bool{}, h.home, 1, true, nil,
		app.MakeEncodingConfig(), wasm.EnableAllProposals,
		app.TestAppOpts{UseSc: true, EnableGiga: true, EnableGigaOCC: true}, app.EmptyWasmOpts, nil,
		func(ba *baseapp.BaseApp) { ba.SetConcurrencyWorkers(1); ba.SetOccEnabled(true) })
	h.a.ChainID = chainsBlockChainID
	a := h.a
	closed := false
	h.close = func() {
		if !closed {
			if done := a.GetOptimisticProcessingInfo().Completion; done != nil {
				<-done
			}
			require.NoError(t, a.Close())
			closed = true
		}
	}
	t.Cleanup(h.close)
}

func repair102NewChain(t *testing.T, keys repair102Keys, maxWanted int64, sequenced bool) *repair102Chain {
	t.Helper()
	h := &repair102Chain{home: t.TempDir(), database: t.TempDir(), keys: keys}
	h.open(t)
	genesis := app.NewDefaultGenesisState(app.MakeEncodingConfig().Marshaler)
	if sequenced {
		var err error
		genesis[seqtypes.ModuleName], err = json.Marshal(map[string]any{
			"leaders": [][]byte{keys.public}, "receipt_keys": keys.pins, "activation_height": 2,
		})
		require.NoError(t, err)
	}
	encoded, err := json.Marshal(genesis)
	require.NoError(t, err)
	params := *app.DefaultConsensusParams
	block := *params.Block
	block.MaxGas, block.MaxGasWanted = 4_000_000, maxWanted
	params.Block = &block
	_, err = h.a.InitChain(context.Background(), &abci.RequestInitChain{
		Time: repair102Time, ChainId: chainsBlockChainID, InitialHeight: 1,
		ConsensusParams: &params, AppStateBytes: encoded,
	})
	require.NoError(t, err)
	_, pending := h.a.UpgradeKeeper.GetUpgradePlan(h.initialContext())
	require.False(t, pending, "fixture genesis must not inherit a pending upgrade")
	return h
}

func (h *repair102Chain) initialContext() sdk.Context {
	ctx := h.a.NewContext(false, tmproto.Header{ChainID: chainsBlockChainID, Height: 1, Time: repair102Time, ProposerAddress: h.keys.proposer})
	return ctx.WithConsensusParams(h.a.GetConsensusParams(ctx))
}

func repair102Fund(t *testing.T, a *app.App, ctx sdk.Context, sender utils.TestAcct) {
	t.Helper()
	a.EvmKeeper.SetAddressMapping(ctx, sender.AccountAddress, sender.EvmAddress)
	coins := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1_000_000_000_000_000))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", coins))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", sender.AccountAddress, coins))
}

func (h *repair102Chain) commitInitial(t *testing.T, merged bool, plans ...upgradetypes.Plan) {
	t.Helper()
	if merged {
		h.a.UpgradeKeeper.SetDone(h.initialContext(), app.MergeUpgrade)
	}
	hash := sha256.Sum256([]byte("repair-102-canonical-block-one"))
	_, err := h.a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Hash: hash[:], Header: &tmproto.Header{ChainID: chainsBlockChainID, Height: 1, Time: repair102Time, ProposerAddress: h.keys.proposer},
	})
	require.NoError(t, err)
	// Governance schedules during block execution, after the upgrade module's
	// BeginBlock check. Persist the real plan at that point before committing.
	for _, plan := range plans {
		require.True(t, h.a.UpgradeKeeper.HasHandler(plan.Name))
		require.NoError(t, h.a.UpgradeKeeper.ScheduleUpgrade(h.initialContext(), plan))
	}
	h.a.SetDeliverStateToCommit()
	_, err = h.a.Commit(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 1, h.a.LastBlockHeight())
}

func (h *repair102Chain) executor(t *testing.T, height int64) (sequencer.Executor, *sequencer.Receipter, *sequencer.ConsensusSource) {
	t.Helper()
	last, err := h.a.SequencerKeeper.LastSeq(h.a.GetCheckCtx())
	require.NoError(t, err)
	hash, err := h.a.SequencerKeeper.LastSeqHash(h.a.GetCheckCtx())
	require.NoError(t, err)
	exec, err := sequencer.NewExecutor(h.a, sequencer.Options{
		ChainID: chainsBlockChainID, ProposerAddress: h.keys.proposer, LastSeq: last, LastSeqHash: hash,
	})
	require.NoError(t, err)
	require.NoError(t, exec.Open(height, repair102Time.Add(time.Duration(height)*time.Second).UnixMilli()))
	receipts, err := sequencer.NewReceipter(exec, h.a, &h.a.EvmKeeper, h.keys.key, app.EVMReceiptModuleID)
	require.NoError(t, err)
	source, err := sequencer.NewConsensusSource(h.a, exec, h.keys.key)
	require.NoError(t, err)
	h.a.SetSequencerSource(source)
	return exec, receipts, source
}

func repair102Tx(t *testing.T, from utils.TestAcct, chainID *big.Int, typed bool, nonce, gas uint64, to common.Address, data []byte) ([]byte, common.Hash) {
	t.Helper()
	var payload ethtypes.TxData = &ethtypes.LegacyTx{Nonce: nonce, GasPrice: big.NewInt(seqCtxGasPrice), Gas: gas, To: &to, Value: big.NewInt(0), Data: data}
	if typed {
		payload = &ethtypes.DynamicFeeTx{ChainID: chainID, Nonce: nonce, GasFeeCap: big.NewInt(seqCtxGasPrice), GasTipCap: big.NewInt(seqCtxGasPrice), Gas: gas, To: &to, Value: big.NewInt(0), Data: data}
	}
	signed, err := ethtypes.SignTx(ethtypes.NewTx(payload), ethtypes.LatestSignerForChainID(chainID), from.EvmPrivateKey)
	require.NoError(t, err)
	txData, err := ethtx.NewTxDataFromTx(signed)
	require.NoError(t, err)
	msg, err := evmtypes.NewMsgEVMTransaction(txData)
	require.NoError(t, err)
	config := app.MakeEncodingConfig().TxConfig
	builder := config.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msg))
	builder.SetGasLimit(gas)
	bz, err := config.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	return bz, signed.Hash()
}

func repair102Accept(t *testing.T, receipts *sequencer.Receipter, key *sequencer.ReceiptKey, tx []byte) *sequencer.Accepted {
	t.Helper()
	accepted, receipt, err := receipts.Accept(tx)
	require.NoError(t, err)
	require.Zero(t, accepted.Result.Code, accepted.Result.Log)
	signed, err := verify.ReceiptSignature(receipt, key.PublicKey())
	require.NoError(t, err)
	require.Equal(t, accepted.Seq, signed.Receipt.GlobalSequence)
	return accepted
}

func TestRepair_102_GasWantedZero(t *testing.T) {
	for _, bound := range []int64{0, -1, 42_000} {
		t.Run(fmt.Sprint(bound), func(t *testing.T) {
			keys := repair102SigningKeys(t)
			h := repair102NewChain(t, keys, bound, true)
			sender := utils.NewSigner()
			repair102Fund(t, h.a, h.initialContext(), sender)
			params := h.a.GetConsensusParams(h.initialContext())
			params.Block.MaxGas = 42_000
			h.a.StoreConsensusParams(h.initialContext(), params)
			h.commitInitial(t, true)
			_, receipts, source := h.executor(t, 2)
			for nonce := uint64(0); nonce < 2; nonce++ {
				tx, _ := repair102Tx(t, sender, big.NewInt(125), false, nonce, 21_000, common.HexToAddress("0xbeef"), nil)
				repair102Accept(t, receipts, keys.key, tx)
			}
			sealed, err := source.Seal(context.Background(), 2)
			require.NoError(t, err)
			txs := append([][]byte{sealed.Batch}, sealed.Txs.ToSliceOfBytes()...)
			header := &tmproto.Header{ChainID: chainsBlockChainID, Height: 2, Time: sealed.OpenTime, ProposerAddress: keys.proposer}
			response, err := h.a.ProcessProposal(context.Background(), &abci.RequestProcessProposal{Header: header, Txs: txs})
			require.NoError(t, err)
			require.Equal(t, abci.ResponseProcessProposal_ACCEPT, response.Status)
			if done := h.a.GetOptimisticProcessingInfo().Completion; done != nil {
				<-done
			}
			h.a.ClearOptimisticProcessingInfo()
			// The same receipted block exceeds a separately configured positive validator bound.
			rejecting := repair102NewChain(t, keys, 21_000, true)
			repair102Fund(t, rejecting.a, rejecting.initialContext(), sender)
			rejecting.commitInitial(t, true)
			response, err = rejecting.a.ProcessProposal(context.Background(), &abci.RequestProcessProposal{Header: header, Txs: txs})
			require.NoError(t, err)
			require.Equal(t, abci.ResponseProcessProposal_REJECT, response.Status)
		})
	}
}

func TestRepair_102_CommittedParentRestart(t *testing.T) {
	keys := repair102SigningKeys(t)
	h := repair102NewChain(t, keys, 0, true)
	h.commitInitial(t, true)
	want := h.a.LastBlockID()
	require.Len(t, want.Hash, sha256.Size)
	before, _, _ := h.executor(t, 2)
	opened, err := before.Snapshot()
	require.NoError(t, err)
	header := opened.BlockHeader()
	require.Equal(t, want, header.LastBlockId)
	h.close()
	h.open(t)
	require.EqualValues(t, 1, h.a.LastBlockHeight())
	require.Equal(t, want, h.a.LastBlockID())
	after, _, source := h.executor(t, 2)
	reopened, err := after.Snapshot()
	require.NoError(t, err)
	require.Equal(t, header, reopened.BlockHeader())
	sealed, err := source.Seal(context.Background(), 2)
	require.NoError(t, err)
	require.Equal(t, want, source.Sealed().Context().BlockHeader().LastBlockId)
	require.Equal(t, header.Time, sealed.OpenTime)
}

func TestRepair_102_MergeInfoResume(t *testing.T) {
	for _, governed := range []bool{true, false} {
		t.Run(fmt.Sprint(governed), func(t *testing.T) {
			keys := repair102SigningKeys(t)
			h := repair102NewChain(t, keys, 0, false)
			info, err := json.Marshal(map[string]any{"leaders": [][]byte{keys.public}, "receipt_keys": keys.pins, "activation_height": 3})
			require.NoError(t, err)
			plan := upgradetypes.Plan{Name: app.MergeUpgrade, Height: 2, Info: string(info)}
			if governed {
				h.commitInitial(t, false, plan)
			} else {
				h.commitInitial(t, false)
			}
			require.NoError(t, h.a.UpgradeKeeper.DumpUpgradeInfoWithInfoToDisk(plan.Height, plan.Name, plan.Info))
			before := repair102Stores(t, h.a.GetCheckCtx())
			h.close()
			h.open(t)
			ctx := sdk.NewContext(h.a.CommitMultiStore().CacheMultiStore(), tmproto.Header{ChainID: chainsBlockChainID, Height: 2, Time: repair102Time.Add(time.Second), ProposerAddress: keys.proposer}, false)
			ctx = ctx.WithConsensusParams(h.a.GetConsensusParams(ctx))
			require.Equal(t, before, repair102Stores(t, ctx), "the restart loader must preserve already committed merge stores")
			if governed {
				recovered, found := h.a.UpgradeKeeper.GetUpgradePlan(ctx)
				require.True(t, found)
				require.Equal(t, plan, recovered)
			}
			require.NotPanics(t, func() { h.a.BeginBlock(ctx, 2, nil, nil, true) })
			require.EqualValues(t, 2, h.a.UpgradeKeeper.GetDoneHeight(ctx, app.MergeUpgrade))
			state, err := h.a.SequencerKeeper.ExportGenesis(ctx)
			require.NoError(t, err)
			require.Equal(t, [][32]byte{[32]byte(keys.public)}, state.Leaders)
			require.EqualValues(t, 3, state.ActivationHeight)
			require.Equal(t, [32]byte(keys.pins[0].PublicKey), state.ReceiptKeys[0].PublicKey)
			require.Equal(t, [32]byte(keys.pins[0].KeyId), state.ReceiptKeys[0].KeyId)
			require.Equal(t, keys.pins[0].FirstHeight, state.ReceiptKeys[0].FirstHeight)
			require.Equal(t, keys.pins[0].LastHeight, state.ReceiptKeys[0].LastHeight)
		})
	}
}

func TestRepair_102_TenantReplay(t *testing.T) {
	for _, occ := range []bool{false, true} {
		t.Run(fmt.Sprint(occ), func(t *testing.T) {
			keys := repair102SigningKeys(t)
			producer := repair102NewChain(t, keys, 0, true)
			consumer := repair102NewChain(t, keys, 0, true)
			sender := utils.NewSigner()
			var domain chainstypes.Domain
			contract := common.HexToAddress("0x123456")
			code := common.FromHex("0x466000554860015500")
			for _, h := range []*repair102Chain{producer, consumer} {
				ctx := h.initialContext()
				repair102Fund(t, h.a, ctx, sender)
				h.a.EvmKeeper.SetCode(ctx, contract, code)
				params := h.a.LayerXChainsKeeper.GetParams(ctx)
				var err error
				domain, err = h.a.LayerXChainsKeeper.Create(ctx, sender.EvmAddress, "repair-tenant", 500_000, 65_536, 100,
					params.MinBond.Add(params.CreationFee), h.a.EvmKeeper.GetMinimumFeePerGas(ctx))
				require.NoError(t, err)
				h.commitInitial(t, true)
				// Reload both peers before admission/replay, including the chain registry.
				h.close()
				h.open(t)
				h.a.GigaOCCEnabled = occ
			}
			_, receipts, source := producer.executor(t, 2)
			initialFee := consumer.a.LayerXChainsKeeper.NextBaseFee(consumer.a.GetCheckCtx(), domain.Id)
			var hashes []common.Hash
			for nonce, typed := range []bool{false, true} {
				tx, hash := repair102Tx(t, sender, chainstypes.ChainIDOf(domain.Id), typed, uint64(nonce), 100_000, contract, nil)
				repair102Accept(t, receipts, keys.key, tx)
				hashes = append(hashes, hash)
			}
			sealed, err := source.Seal(context.Background(), 2)
			require.NoError(t, err)
			block := append([][]byte{sealed.Batch}, sealed.Txs.ToSliceOfBytes()...)
			ctx := sdk.NewContext(consumer.a.CommitMultiStore().CacheMultiStore(), source.Sealed().Context().BlockHeader(), false)
			ctx = ctx.WithConsensusParams(consumer.a.GetConsensusParams(ctx))
			_, results, _, err := consumer.a.ProcessBlock(ctx, block, &app.BlockProcessRequest{Height: 2, Time: sealed.OpenTime}, abci.CommitInfo{}, false, nil)
			require.NoError(t, err)
			require.Len(t, results, len(block))
			got, err := abci.MarshalTxResults(results)
			require.NoError(t, err)
			want, err := abci.MarshalTxResults(source.Sealed().Results)
			require.NoError(t, err)
			require.Equal(t, want, got)
			for _, peer := range []struct {
				a   *app.App
				ctx sdk.Context
			}{{producer.a, source.Sealed().Context()}, {consumer.a, ctx}} {
				require.Equal(t, common.BigToHash(chainstypes.ChainIDOf(domain.Id)), peer.a.EvmKeeper.GetState(peer.ctx, contract, common.Hash{}))
				require.Equal(t, uint64(2), peer.a.EvmKeeper.GetNonce(peer.ctx, sender.EvmAddress))
				require.Equal(t, hashes, peer.a.LayerXChainsKeeper.DomainTxs(peer.ctx, domain.Id, 2))
				require.Positive(t, peer.a.LayerXChainsKeeper.DomainGasUsed(peer.ctx, domain.Id, 2))
				require.Equal(t, common.BigToHash(initialFee.TruncateInt().BigInt()), peer.a.EvmKeeper.GetState(peer.ctx, contract, common.HexToHash("0x1")))
			}
		})
	}
}

func TestRepair_102_ExecutionFailureAtomic(t *testing.T) {
	for _, occ := range []bool{false, true} {
		t.Run(fmt.Sprint(occ), func(t *testing.T) {
			keys := repair102SigningKeys(t)
			h := repair102NewChain(t, keys, 0, false)
			sender := utils.NewSigner()
			repair102Fund(t, h.a, h.initialContext(), sender)
			h.commitInitial(t, false)
			h.close()
			h.open(t)
			h.a.GigaOCCEnabled = occ
			blockTime := repair102Time.Add(time.Second)
			ctx := sdk.NewContext(h.a.CommitMultiStore().CacheMultiStore(), tmproto.Header{ChainID: chainsBlockChainID, Height: 2, Time: blockTime, ProposerAddress: keys.proposer}, false)
			ctx = ctx.WithConsensusParams(h.a.GetConsensusParams(ctx))
			// Reopening clears the keeper's process-local coinbase cache. Remove
			// the persisted mapping its real VM block-context producer requires.
			collector := h.a.AccountKeeper.GetModuleAddress(authtypes.FeeCollectorName)
			address, found := h.a.EvmKeeper.GetEVMAddress(ctx, collector)
			require.True(t, found)
			h.a.EvmKeeper.DeleteAddressMapping(ctx, collector, address)
			tx, _ := repair102Tx(t, sender, big.NewInt(125), false, 0, 21_000, common.HexToAddress("0xbeef"), nil)
			before := repair102Stores(t, ctx)
			_, err := h.a.GigaEvmKeeper.GetVMBlockContext(ctx, h.a.GigaEvmKeeper.GetGasPool())
			require.ErrorContains(t, err, "fee collector's EVM address not found")
			_, results, _, err := h.a.ProcessBlock(ctx, [][]byte{tx}, &app.BlockProcessRequest{Height: 2, Time: blockTime}, abci.CommitInfo{}, false, nil)
			require.ErrorContains(t, err, "giga")
			require.ErrorContains(t, err, "fee collector's EVM address not found")
			require.Nil(t, results)
			require.Equal(t, before, repair102Stores(t, ctx), "BeginBlock and every transaction branch must be discarded")
			require.Zero(t, h.a.EvmKeeper.GetNonce(ctx, sender.EvmAddress))
			for _, incomplete := range [][]*abci.ExecTxResult{nil, {nil}} {
				_, _, err = h.a.CompleteBlock(ctx, 2, incomplete, []*evmtypes.MsgEVMTransaction{{}})
				require.Error(t, err)
				require.Equal(t, before, repair102Stores(t, ctx))
			}
		})
	}
}

func TestRepair_102_SchedulerFailureAtomic(t *testing.T) {
	keys := repair102SigningKeys(t)
	h := repair102NewChain(t, keys, 0, false)
	sender := utils.NewSigner()
	repair102Fund(t, h.a, h.initialContext(), sender)
	h.commitInitial(t, false)
	ctx := sdk.NewContext(h.a.CommitMultiStore().CacheMultiStore(), tmproto.Header{
		ChainID: chainsBlockChainID, Height: 2, Time: repair102Time.Add(time.Second), ProposerAddress: keys.proposer,
	}, false)
	ctx = ctx.WithConsensusParams(h.a.GetConsensusParams(ctx))
	// Corrupt a real store value consumed only by Cosmos ante. The EVM scheduler
	// successfully executes first, then Cosmos delivery recovers the codec panic.
	ctx.KVStore(h.a.GetKey(paramstypes.StoreKey)).Set([]byte(paramstypes.ModuleName+"/"+string(paramstypes.ParamStoreKeyCosmosGasParams)), []byte("invalid JSON"))
	evm, _ := repair102Tx(t, sender, big.NewInt(125), false, 0, 21_000, common.HexToAddress("0xbeef"), nil)
	builder := h.a.GetTxConfig().NewTxBuilder()
	require.NoError(t, builder.SetMsgs(&banktypes.MsgSend{FromAddress: sender.AccountAddress.String(), ToAddress: sender.AccountAddress.String(), Amount: sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1))}))
	builder.SetGasLimit(100_000)
	cosmos, err := h.a.GetTxConfig().TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	txs := [][]byte{evm, cosmos}
	typed := make([]sdk.Tx, len(txs))
	for i, tx := range txs {
		typed[i], err = h.a.GetTxConfig().TxDecoder()(tx)
		require.NoError(t, err)
	}
	before := repair102Stores(t, ctx)
	// Establish the actual producer failure, with the real V2 handler and codec.
	failed, _ := ctx.CacheContext()
	v2 := h.a.DeliverTxWithResult(failed, cosmos, typed[1])
	require.Contains(t, v2.Log, "marshalling cosmosGasParams")
	results, _, err := h.a.ExecuteTxsConcurrently(ctx, txs, typed, 0)
	require.ErrorContains(t, err, "infrastructure failure")
	require.Nil(t, results)
	require.Equal(t, before, repair102Stores(t, ctx))
	require.Zero(t, h.a.EvmKeeper.GetNonce(ctx, sender.EvmAddress))
	// The enclosing block must discard BeginBlock as well as the completed EVM half.
	_, results, _, err = h.a.ProcessBlock(ctx, txs, &app.BlockProcessRequest{Height: 2, Time: ctx.BlockTime()}, abci.CommitInfo{}, false, nil)
	require.ErrorContains(t, err, "infrastructure failure")
	require.Nil(t, results)
	require.Equal(t, before, repair102Stores(t, ctx))
}

func repair102Stores(t *testing.T, ctx sdk.Context) map[string]map[string][]byte {
	t.Helper()
	stores := map[string]map[string][]byte{}
	for _, key := range ctx.MultiStore().StoreKeys() {
		if _, ok := key.(*sdk.KVStoreKey); !ok {
			continue
		}
		pairs := map[string][]byte{}
		iter := ctx.KVStore(key).Iterator(nil, nil)
		for ; iter.Valid(); iter.Next() {
			pairs[string(iter.Key())] = bytes.Clone(iter.Value())
		}
		require.NoError(t, iter.Close())
		stores[key.Name()] = pairs
	}
	return stores
}

func TestRepair_102_VerifyRetirement(t *testing.T) {
	keys := repair102SigningKeys(t)
	h := repair102NewChain(t, keys, 0, false)
	ctx := h.initialContext()
	require.Zero(t, h.a.UpgradeKeeper.GetDoneHeight(ctx, app.ActivationUpgrade))
	require.NotContains(t, h.a.UpgradeKeeper.GetModuleVersionMap(ctx), "layerxanchor")
	address := common.HexToAddress("0x1012")
	precompile := h.a.EvmKeeper.CustomPrecompiles(ctx)[address]
	require.NotNil(t, precompile)
	contract, ok := precompile.(*pcommon.Precompile)
	require.True(t, ok)
	for _, height := range []int64{1, 2, 3} {
		at := ctx.WithBlockHeight(height)
		if height == 2 {
			info, err := json.Marshal(map[string]any{"leaders": [][]byte{keys.public}, "receipt_keys": keys.pins, "activation_height": 3})
			require.NoError(t, err)
			h.a.UpgradeKeeper.ApplyUpgrade(at, upgradetypes.Plan{Name: app.MergeUpgrade, Height: height, Info: string(info)})
			require.EqualValues(t, height, h.a.UpgradeKeeper.GetDoneHeight(at, app.MergeUpgrade))
		}
		require.Zero(t, h.a.UpgradeKeeper.GetDoneHeight(at, app.ActivationUpgrade))
		require.Contains(t, h.a.EvmKeeper.CustomPrecompiles(at), address)
		for name, method := range contract.GetABI().Methods {
			t.Run(fmt.Sprintf("%d/%s", height, name), func(t *testing.T) {
				args := make([]any, len(method.Inputs))
				for i, arg := range method.Inputs {
					switch arg.Type.String() {
					case "bytes":
						args[i] = []byte{}
					case "bytes32":
						args[i] = [32]byte{}
					case "uint8":
						args[i] = uint8(0)
					case "uint64":
						args[i] = uint64(0)
					default:
						t.Fatalf("unhandled ABI type %s", arg.Type.String())
					}
				}
				if name == "verifyEd25519" {
					private := ed25519.NewKeyFromSeed(sequencerKeySeed)
					message := []byte("historical raw signature")
					args = []any{[32]byte(private.Public().(ed25519.PublicKey)), uint8(255), message, ed25519.Sign(private, message)}
				}
				input, err := contract.GetABI().Pack(name, args...)
				require.NoError(t, err)
				cost := precompile.RequiredGas(input)
				require.Positive(t, cost)
				db := state.NewDBImpl(at, &h.a.EvmKeeper, true)
				defer db.Cleanup()
				block, err := h.a.EvmKeeper.GetVMBlockContext(at, h.a.EvmKeeper.GetGasPool())
				require.NoError(t, err)
				evm := vm.NewEVM(*block, db, evmtypes.DefaultChainConfig().EthereumConfig(big.NewInt(125)), vm.Config{}, h.a.EvmKeeper.CustomPrecompiles(at))
				const supplied = uint64(1_000_000)
				output, left, err := evm.StaticCall(common.Address{1}, address, input, supplied)
				require.Equal(t, supplied-cost, left)
				if height < 2 && name == "verifyEd25519" {
					require.NoError(t, err)
					values, unpackErr := contract.GetABI().Unpack(name, output)
					require.NoError(t, unpackErr)
					require.Equal(t, []any{true}, values)
				} else {
					require.ErrorIs(t, err, vm.ErrExecutionReverted)
				}
				if height >= 2 {
					reason, decodeErr := abi.UnpackRevert(output)
					require.NoError(t, decodeErr)
					require.Contains(t, reason, "LXP_MODULE_DISABLED")
				} else if name == "verifyEd25519" {
					require.NotContains(t, string(output), "LXP_MODULE_DISABLED")
				}
			})
		}
	}
}

func TestRepair_102_MergeFallbackActivation(t *testing.T) {
	for _, activation := range []int64{1, 42} {
		for _, height := range []int64{activation - 1, activation, activation + 1} {
			if height < 1 {
				continue
			}
			t.Run(fmt.Sprintf("%d/%d", activation, height), func(t *testing.T) {
				keys := repair102SigningKeys(t)
				h := repair102NewChain(t, keys, 0, false)
				ctx := h.initialContext().WithBlockHeight(height)
				h.a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(activation), app.MergeUpgrade)
				h.a.RefreshPrecompiles(ctx)
				payer := utils.NewSigner()
				repair102Fund(t, h.a, ctx, payer)
				for _, address := range []string{"0x101A", "0x101B", "0x101C", "0x101D", "0x101E", "0x0901", "0x0902", "0x0903", "0x0904", "0x0905", "0x0906", "0x0907", "0x0908"} {
					to := common.HexToAddress(address)
					if height >= activation {
						require.Contains(t, h.a.EvmKeeper.CustomPrecompiles(ctx), to)
					} else {
						require.NotContains(t, h.a.EvmKeeper.CustomPrecompiles(ctx), to)
					}
					tx, _ := repair102Tx(t, payer, big.NewInt(125), true, 0, 100_000, to, []byte{0xff, 0xff, 0xff, 0xff})
					typed, err := h.a.GetTxConfig().TxDecoder()(tx)
					require.NoError(t, err)
					ordinary, _ := ctx.CacheContext()
					want := h.a.DeliverTxWithResult(ordinary, tx, typed)
					for _, occ := range []bool{false, true} {
						giga, _ := ctx.CacheContext()
						h.a.GigaOCCEnabled = occ
						results, _, err := h.a.ExecuteTxsConcurrently(giga, [][]byte{tx}, []sdk.Tx{typed}, 0)
						require.NoError(t, err, "%s, OCC %t", address, occ)
						require.Len(t, results, 1)
						expected, err := abci.MarshalTxResults([]*abci.ExecTxResult{want})
						require.NoError(t, err)
						actual, err := abci.MarshalTxResults(results)
						require.NoError(t, err)
						require.Equal(t, expected, actual, "%s, OCC %t", address, occ)
					}
				}
			})
		}
	}
}

func TestRepair_102_StrictResultCommitment(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			keys := repair102SigningKeys(t)
			h := repair102NewChain(t, keys, 0, true)
			h.a.GigaExecutorEnabled = enabled
			require.False(t, tmtypes.SkipLastResultsHashValidation.Load())
			payer := utils.NewSigner()
			repair102Fund(t, h.a, h.initialContext(), payer)
			// Use a non-unit priority normalizer to prove the consensus gas conversion.
			params := h.a.EvmKeeper.GetParams(h.initialContext())
			params.PriorityNormalizer = sdk.NewDec(2)
			h.a.EvmKeeper.SetParams(h.initialContext(), params)
			h.commitInitial(t, true)
			_, receipts, source := h.executor(t, 2)
			tx, _ := repair102Tx(t, payer, big.NewInt(125), true, 0, 50_000, common.HexToAddress("0xbeef"), nil)
			repair102Accept(t, receipts, keys.key, tx)
			sealed, err := source.Seal(context.Background(), 2)
			require.NoError(t, err)
			header := source.Sealed().Context().BlockHeader()
			replay := sdk.NewContext(h.a.CommitMultiStore().CacheMultiStore(), header, false)
			replay = replay.WithConsensusParams(h.a.GetConsensusParams(replay))
			_, results, _, err := h.a.ProcessBlock(replay, append([][]byte{sealed.Batch}, sealed.Txs.ToSliceOfBytes()...), &app.BlockProcessRequest{Height: 2, Time: sealed.OpenTime}, abci.CommitInfo{}, false, nil)
			require.NoError(t, err)
			want, err := abci.MarshalTxResults(source.Sealed().Results)
			require.NoError(t, err)
			got, err := abci.MarshalTxResults(results)
			require.NoError(t, err)
			require.Equal(t, want, got)
			root, proofs := merkle.ProofsFromByteSlices(got)
			require.NoError(t, proofs[1].Verify(root, got[1]))
			tampered := *results[1]
			tampered.GasUsed++
			bad, err := abci.MarshalTxResults([]*abci.ExecTxResult{&tampered})
			require.NoError(t, err)
			require.Error(t, proofs[1].Verify(root, bad[0]))
			trusted := &tmtypes.Header{LastResultsHash: root}
			conflicting := *trusted
			conflicting.LastResultsHash = bytes.Repeat([]byte{0x42}, 32)
			evidence := &tmtypes.LightClientAttackEvidence{ConflictingBlock: &tmtypes.LightBlock{SignedHeader: &tmtypes.SignedHeader{Header: &conflicting}}}
			require.True(t, evidence.ConflictingHeaderIsInvalid(trusted))
			repair102LightResults(t, keys, results, root)
		})
	}
}

func repair102LightResults(t *testing.T, keys repair102Keys, results []*abci.ExecTxResult, root []byte) {
	t.Helper()
	pub, err := keys.validator.GetPubKey(context.Background())
	require.NoError(t, err)
	validators := tmtypes.NewValidatorSet([]*tmtypes.Validator{tmtypes.NewValidator(pub, 1)})
	block := tmtypes.MakeBlock(3, nil, &tmtypes.Commit{}, nil)
	block.ChainID = chainsBlockChainID
	block.Time = repair102Time.Add(3 * time.Second)
	block.ProposerAddress = pub.Address()
	block.ValidatorsHash = validators.Hash()
	block.NextValidatorsHash = validators.Hash()
	block.LastResultsHash = root
	parts, err := block.MakePartSet(tmtypes.BlockPartSizeBytes)
	require.NoError(t, err)
	id := tmtypes.BlockID{Hash: block.Hash(), PartSetHeader: parts.Header()}
	votes := tmtypes.NewVoteSet(chainsBlockChainID, 3, 0, tmproto.PrecommitType, validators)
	commit, err := tmtypes.MakeCommit(context.Background(), id, 3, 0, votes, []tmtypes.PrivValidator{keys.validator}, block.Time)
	require.NoError(t, err)
	trusted := &tmtypes.LightBlock{SignedHeader: &tmtypes.SignedHeader{Header: &block.Header, Commit: commit}, ValidatorSet: validators}
	require.NoError(t, trusted.ValidateBasic(chainsBlockChainID))
	require.NoError(t, validators.VerifyCommit(chainsBlockChainID, id, 3, commit))
	database := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	store := lightstore.New(database)
	require.NoError(t, store.SaveLightBlock(trusted))

	var response atomic.Pointer[coretypes.ResultBlockResults]
	response.Store(&coretypes.ResultBlockResults{Height: 2, TxsResults: results})
	forged := block.Header
	forged.LastResultsHash = bytes.Repeat([]byte{0x42}, 32)
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, map[string]*rpcserver.RPCFunc{
		"block_results": rpcserver.NewRPCFunc(func(context.Context, *struct {
			Height *int64 `json:"height"`
		}) (*coretypes.ResultBlockResults, error) {
			return response.Load(), nil
		}),
		"header_by_hash": rpcserver.NewRPCFunc(func(context.Context, *struct {
			Hash []byte `json:"hash"`
		}) (*coretypes.ResultHeader, error) {
			return &coretypes.ResultHeader{Header: &forged}, nil
		}),
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	next, err := rpchttp.New(server.URL)
	require.NoError(t, err)
	primary, err := lightprovider.New(chainsBlockChainID, server.URL)
	require.NoError(t, err)
	witness, err := lightprovider.New(chainsBlockChainID, server.URL)
	require.NoError(t, err)
	lc, err := light.NewClientFromTrustedStore(chainsBlockChainID, time.Hour, primary, []provider.Provider{witness}, store, time.Hour)
	require.NoError(t, err)
	client := lightrpc.NewClient(next, lc)
	height := int64(2)
	_, err = client.BlockResults(context.Background(), &height)
	require.NoError(t, err)
	tampered := append([]*abci.ExecTxResult(nil), results...)
	changed := *tampered[1]
	changed.GasUsed++
	tampered[1] = &changed
	response.Store(&coretypes.ResultBlockResults{Height: 2, TxsResults: tampered})
	_, err = client.BlockResults(context.Background(), &height)
	require.ErrorContains(t, err, "does not match with trusted last results")
	_, err = client.HeaderByHash(context.Background(), forged.Hash())
	require.ErrorContains(t, err, "primary header hash does not match trusted header hash")
}
