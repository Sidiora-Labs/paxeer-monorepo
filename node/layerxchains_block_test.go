package app_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

const chainsBlockChainID = "hyperpax_125-1"

type chainsBlockHarness struct {
	a         *app.App
	exec      sequencer.Executor
	source    *sequencer.ConsensusSource
	contract  abi.ABI
	sender    utils.TestAcct
	open      func() (*app.App, func())
	close     func()
	at        time.Time
	wantCodes map[int]uint32
}

func newChainsBlockHarness(t *testing.T) *chainsBlockHarness {
	return newChainsBlockHarnessWithSetup(t, nil)
}

func newChainsBlockHarnessWithSetup(t *testing.T, setup func(*testing.T, *chainsBlockHarness, *app.App, sdk.Context)) *chainsBlockHarness {
	t.Helper()
	home, directory := t.TempDir(), t.TempDir()
	h := &chainsBlockHarness{sender: utils.NewSigner(), at: time.Unix(1_800_000_000, 0).UTC()}
	h.open = func() (*app.App, func()) {
		database, err := dbm.NewGoLevelDB("application", directory)
		require.NoError(t, err)
		a := app.New(database, nil, true, map[int64]bool{}, home, 1, true, nil,
			app.MakeEncodingConfig(), wasm.EnableAllProposals, app.TestAppOpts{UseSc: true}, app.EmptyWasmOpts, nil)
		if a.LastBlockHeight() > 0 {
			queryCtx := a.NewContext(true, tmproto.Header{ChainID: chainsBlockChainID, Height: a.LastBlockHeight(),
				Time: h.at.Add(time.Duration(a.LastBlockHeight()) * time.Second)})
			a.RefreshPrecompiles(queryCtx.WithGasMeter(sdk.NewInfiniteGasMeter(1, 1)))
		}
		closed := false
		closeApp := func() {
			if !closed {
				require.NoError(t, a.Close())
				closed = true
			}
		}
		t.Cleanup(closeApp)
		return a, closeApp
	}
	h.a, h.close = h.open()
	data, err := os.ReadFile("../precompiles/layerxchains/abi.json")
	require.NoError(t, err)
	h.contract, err = abi.JSON(strings.NewReader(string(data)))
	require.NoError(t, err)
	genesis := app.NewDefaultGenesisState(app.MakeEncodingConfig().Marshaler)
	chainsGenesis := chainstypes.DefaultGenesis()
	chainsGenesis.Params.MinGasAllowance = 2 * seqCtxTransferGas
	genesis[chainstypes.ModuleName], err = json.Marshal(chainsGenesis)
	require.NoError(t, err)
	evmGenesis := evmtypes.DefaultGenesis()
	evmGenesis.Params.MinimumFeePerGas = sdk.NewDec(10)
	evmGenesis.Params.MaximumFeePerGas = sdk.NewDec(1000)
	if setup != nil {
		evmGenesis.Params.MinimumFeePerGas = sdk.NewDec(67_000_000)
		evmGenesis.Params.MaximumFeePerGas = sdk.NewDec(500_000_000_000)
	}
	evmGenesis.Params.TargetGasUsedPerBlock = seqCtxTransferGas
	evmGenesis.Params.MaxDynamicBaseFeeUpwardAdjustment = sdk.MustNewDecFromStr("0.5")
	evmGenesis.Params.MaxDynamicBaseFeeDownwardAdjustment = sdk.MustNewDecFromStr("0.5")
	genesis[evmtypes.ModuleName], err = json.Marshal(evmGenesis)
	require.NoError(t, err)
	var activation map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(activationGenesis(t), &activation))
	activation["activation_height"] = json.RawMessage(`2`)
	genesis[seqtypes.ModuleName], err = json.Marshal(activation)
	require.NoError(t, err)
	encoded, err := json.Marshal(genesis)
	require.NoError(t, err)
	params := *app.DefaultConsensusParams
	block := *params.Block
	block.MaxGas, block.MaxGasWanted = 1_000_000, 1_000_000
	if setup != nil {
		block.MaxGas, block.MaxGasWanted = 10_000_000, 10_000_000
	}
	params.Block = &block
	_, err = h.a.InitChain(context.Background(), &abci.RequestInitChain{
		Time: h.at, ChainId: chainsBlockChainID, InitialHeight: 1,
		ConsensusParams: &params, AppStateBytes: encoded,
	})
	require.NoError(t, err)
	_, err = h.a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: chainsBlockChainID, Height: 1, Time: h.at},
	})
	require.NoError(t, err)
	ctx := h.a.GetContextForDeliverTx(nil)
	h.a.UpgradeKeeper.SetDone(ctx, app.MergeUpgrade)
	h.a.EvmKeeper.SetAddressMapping(ctx, h.sender.AccountAddress, h.sender.EvmAddress)
	funds := sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewIntWithDecimal(1000, 18)))
	require.NoError(t, h.a.BankKeeper.MintCoins(ctx, minttypes.ModuleName, funds))
	require.NoError(t, h.a.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, h.sender.AccountAddress, funds))
	if setup != nil {
		setup(t, h, h.a, ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx)))
	}
	h.a.SetDeliverStateToCommit()
	_, err = h.a.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), h.a.LastBlockHeight())
	h.exec, err = sequencer.NewExecutor(h.a, sequencer.Options{ChainID: chainsBlockChainID, ProposerAddress: crypto.AddressHash(sequencerLeader[:])})
	require.NoError(t, err)
	keyDir := t.TempDir()
	consensusKey := filepath.Join(keyDir, "consensus.json")
	pv, err := privval.GenFilePV(consensusKey, filepath.Join(keyDir, "consensus-state.json"), "")
	require.NoError(t, err)
	require.NoError(t, pv.Save())
	receiptKey := filepath.Join(keyDir, "receipt-key")
	require.NoError(t, os.WriteFile(receiptKey, []byte(hex.EncodeToString(sequencerKeySeed)), 0o600))
	pins, err := h.a.SequencerKeeper.ReceiptKeyPins(h.a.GetCheckCtx())
	require.NoError(t, err)
	key, err := sequencer.LoadReceiptKey(receiptKey, consensusKey, pins, 2)
	require.NoError(t, err)
	h.source, err = sequencer.NewConsensusSource(h.a, h.exec, key)
	require.NoError(t, err)
	h.a.SetSequencerSource(h.source)
	return h
}

func (h *chainsBlockHarness) transaction(t *testing.T, chainID int64, nonce, gas uint64, to common.Address, value *big.Int, data []byte) ([]byte, common.Hash) {
	return h.signedTransaction(t, h.sender, chainID, nonce, gas, to, value, data)
}

func (h *chainsBlockHarness) signedTransaction(t *testing.T, signer utils.TestAcct, chainID int64, nonce, gas uint64, to common.Address, value *big.Int, data []byte) ([]byte, common.Hash) {
	t.Helper()
	id := big.NewInt(chainID)
	tx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID: id, Nonce: nonce, GasTipCap: big.NewInt(seqCtxGasPrice), GasFeeCap: big.NewInt(seqCtxGasPrice),
		Gas: gas, To: &to, Value: value, Data: data,
	}), ethtypes.LatestSignerForChainID(id), signer.EvmPrivateKey)
	require.NoError(t, err)
	require.Equal(t, id, tx.ChainId())
	txData, err := ethtx.NewTxDataFromTx(tx)
	require.NoError(t, err)
	msg, err := evmtypes.NewMsgEVMTransaction(txData)
	require.NoError(t, err)
	config := h.a.GetTxConfig()
	builder := config.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msg))
	builder.SetGasLimit(gas)
	encoded, err := config.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	return encoded, tx.Hash()
}

func (h *chainsBlockHarness) accept(t *testing.T, tx []byte, hash common.Hash) *sequencer.Accepted {
	t.Helper()
	accepted, err := h.exec.Accept(tx)
	require.NoError(t, err, "sequencer refused %s: %v", hash, err)
	require.NotNil(t, accepted)
	var messages sdk.TxMsgData
	require.NoError(t, messages.Unmarshal(accepted.Result.Data))
	require.Len(t, messages.Data, 1)
	var execution evmtypes.MsgEVMTransactionResponse
	require.NoError(t, execution.Unmarshal(messages.Data[0].Data))
	if execution.VmError != "" {
		reason, err := abi.UnpackRevert(execution.ReturnData)
		require.NoError(t, err, "EVM failure: %s; return data: %x", execution.VmError, execution.ReturnData)
		t.Logf("EVM revert: %s", reason)
	}
	require.Zero(t, accepted.Result.Code, accepted.Result.Log)
	require.Empty(t, execution.VmError)
	require.Equal(t, [32]byte(hash), accepted.ActivityID)
	receipt, err := h.a.EvmKeeper.GetTransientReceipt(accepted.ReceiptContext, hash, uint64(accepted.TxIndex))
	require.NoError(t, err)
	require.Equal(t, uint32(ethtypes.ReceiptStatusSuccessful), receipt.Status, receipt.VmError)
	require.Empty(t, receipt.VmError)
	return accepted
}

func (h *chainsBlockHarness) commit(t *testing.T, height int64) {
	t.Helper()
	_, err := h.source.Seal(context.Background(), height)
	require.NoError(t, err)
	sealed := h.source.Sealed()
	require.NotNil(t, sealed)
	header := &tmproto.Header{ChainID: chainsBlockChainID, Height: height,
		Time: h.at.Add(time.Duration(height) * time.Second), ProposerAddress: crypto.AddressHash(sequencerLeader[:])}
	proposal, err := h.a.ProcessProposal(context.Background(), &abci.RequestProcessProposal{
		Hash: []byte(t.Name()), Header: header, Txs: sealed.Txs,
	})
	require.NoError(t, err)
	require.Equal(t, abci.ResponseProcessProposal_ACCEPT, proposal.Status)
	response, err := h.a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Hash: []byte(t.Name()), Txs: sealed.Txs,
		Header: &tmproto.Header{ChainID: chainsBlockChainID, Height: height,
			Time: h.at.Add(time.Duration(height) * time.Second), ProposerAddress: crypto.AddressHash(sequencerLeader[:])},
	})
	require.NoError(t, err)
	require.Len(t, response.TxResults, len(sealed.Txs))
	for i, result := range response.TxResults {
		if code, ok := h.wantCodes[i]; ok {
			require.Equal(t, code, result.Code, result.Log)
		} else {
			require.Zero(t, result.Code, result.Log)
		}
		if i > 0 {
			require.Same(t, sealed.Results[i], result)
		}
	}
	h.wantCodes = nil
	_, err = h.a.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, height, h.a.LastBlockHeight())
	require.Equal(t, response.AppHash, h.a.LastCommitID().Hash)
}

func (h *chainsBlockHarness) mixedBlock(t *testing.T) ([]common.Hash, sdk.Dec) {
	t.Helper()
	address := common.HexToAddress("0x000000000000000000000000000000000000101E")
	input, err := h.contract.Pack("createChain", "block-chain", uint64(2*seqCtxTransferGas), uint64(65536), uint16(100))
	require.NoError(t, err)
	params := chainstypes.DefaultParams()
	bond := new(big.Int).Mul(params.MinBond.Add(params.CreationFee).BigInt(), state.UhpxToSweiMultiplier)
	tx, hash := h.transaction(t, 125, 0, 800_000, address, bond, input)
	require.NoError(t, h.exec.Open(2, h.at.Add(2*time.Second).UnixMilli()))
	h.accept(t, tx, hash)
	h.commit(t, 2)
	rootBefore := h.a.EvmKeeper.GetNextBaseFeePerGas(h.a.GetCheckCtx())
	require.NoError(t, h.exec.Open(3, h.at.Add(3*time.Second).UnixMilli()))
	recipient := utils.NewSigner().EvmAddress
	root, rootHash := h.transaction(t, 125, 1, seqCtxTransferGas, recipient, big.NewInt(1), nil)
	first, firstHash := h.transaction(t, 125000001, 2, seqCtxTransferGas, recipient, big.NewInt(2), nil)
	second, secondHash := h.transaction(t, 125000001, 3, seqCtxTransferGas, recipient, big.NewInt(3), nil)
	for _, send := range []struct {
		tx   []byte
		hash common.Hash
	}{{root, rootHash}, {first, firstHash}, {second, secondHash}} {
		accepted := h.accept(t, send.tx, send.hash)
		require.Equal(t, int64(seqCtxTransferGas), accepted.Result.GasUsed, "transaction %s used %d gas", send.hash, accepted.Result.GasUsed)
	}
	third, thirdHash := h.transaction(t, 125000001, 4, seqCtxTransferGas, recipient, big.NewInt(4), nil)
	refused, err := h.exec.Accept(third)
	require.ErrorIs(t, err, sequencer.ErrBlockFull)
	require.EqualError(t, err, sequencer.ErrBlockFull.Error()+": chain 1 gas wanted 63000 exceeds its allowance 42000")
	require.Nil(t, refused)
	_, pending := h.exec.PendingTx([32]byte(thirdHash))
	require.False(t, pending)
	h.commit(t, 3)
	require.Len(t, h.source.Sealed().Txs, 4)
	ctx := h.a.GetCheckCtx()
	require.Equal(t, uint64(4), h.a.EvmKeeper.GetNonce(ctx, h.sender.EvmAddress))
	requireSeqCtxWei(t, big.NewInt(6), seqCtxWei(h.a, ctx, recipient))
	domain, found := h.a.LayerXChainsKeeper.GetDomain(ctx, 1)
	require.True(t, found)
	require.Equal(t, chainstypes.Domain{Id: 1, Name: "block-chain", Operator: h.sender.EvmAddress,
		Status: chainstypes.StatusActive, GasAllowance: 42000, ByteAllowance: 65536, FeeShareBps: 100,
		Bond: params.MinBond, TargetGasPerBlock: 21000, CreatedHeight: 2}, domain)
	fee := h.a.LayerXChainsKeeper.NextBaseFee(ctx, 1)
	require.True(t, fee.Equal(sdk.NewDec(15)), "chain fee: %s", fee)
	require.True(t, fee.GT(sdk.NewDec(10)))
	require.Equal(t, uint64(42000), h.a.LayerXChainsKeeper.DomainGasUsed(ctx, 1, 3))
	hashes := []common.Hash{firstHash, secondHash}
	require.Equal(t, hashes, h.a.LayerXChainsKeeper.DomainTxs(ctx, 1, 3))
	require.Empty(t, h.a.LayerXChainsKeeper.DomainTxs(ctx, 1, 2))
	return hashes, rootBefore
}

func (h *chainsBlockHarness) getChain(t *testing.T, a *app.App) []interface{} {
	t.Helper()
	ctx, _ := a.NewContext(true, tmproto.Header{ChainID: chainsBlockChainID, Height: a.LastBlockHeight(),
		Time: h.at.Add(time.Duration(a.LastBlockHeight()) * time.Second)}).CacheContext()
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	a.RefreshPrecompiles(ctx)
	require.Equal(t, big.NewInt(125), a.EvmKeeper.ChainID(ctx))
	require.Contains(t, a.EvmKeeper.CustomPrecompiles(ctx), common.HexToAddress("0x101E"))
	block, err := a.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(2_000_000))
	require.NoError(t, err)
	db := state.NewDBImpl(ctx, &a.EvmKeeper, false)
	evm := vm.NewEVM(*block, db, evmtypes.DefaultChainConfig().EthereumConfig(a.EvmKeeper.ChainID(ctx)), vm.Config{}, a.EvmKeeper.CustomPrecompiles(ctx))
	input, err := h.contract.Pack("getChain", uint32(1))
	require.NoError(t, err)
	output, _, err := evm.StaticCall(h.sender.EvmAddress, common.HexToAddress("0x101E"), input, 2_000_000)
	require.NoError(t, err)
	values, err := h.contract.Unpack("getChain", output)
	require.NoError(t, err)
	return values
}

func TestOneBlockCarriesRootAndChainTransactions(t *testing.T) {
	h := newChainsBlockHarness(t)
	_, rootBefore := h.mixedBlock(t)
	values := h.getChain(t, h.a)
	require.Equal(t, []interface{}{uint32(1), "block-chain", h.sender.EvmAddress, chainstypes.StatusActive,
		uint64(125000001), uint64(42000), uint64(65536), uint16(100),
		new(big.Int).Mul(chainstypes.DefaultParams().MinBond.BigInt(), state.UhpxToSweiMultiplier), big.NewInt(15), uint64(2)}, values)
	rootAfter := h.a.EvmKeeper.GetNextBaseFeePerGas(h.a.GetCheckCtx())
	require.True(t, rootBefore.Equal(rootAfter), "root traffic exactly meets its 21000 gas target: before %s, after %s", rootBefore, rootAfter)
}

func TestChainStateSurvivesRestart(t *testing.T) {
	h := newChainsBlockHarness(t)
	hashes, _ := h.mixedBlock(t)
	before := h.getChain(t, h.a)
	fee := h.a.LayerXChainsKeeper.NextBaseFee(h.a.GetCheckCtx(), 1)
	commit := h.a.LastCommitID()
	h.close()
	restarted, _ := h.open()
	require.NotSame(t, h.a, restarted)
	require.Equal(t, int64(3), restarted.LastBlockHeight())
	require.Equal(t, commit, restarted.LastCommitID())
	require.Equal(t, before, h.getChain(t, restarted))
	ctx := restarted.GetCheckCtx()
	require.True(t, fee.Equal(restarted.LayerXChainsKeeper.NextBaseFee(ctx, 1)))
	require.Equal(t, hashes, restarted.LayerXChainsKeeper.DomainTxs(ctx, 1, 3))
	require.Equal(t, uint64(42000), restarted.LayerXChainsKeeper.DomainGasUsed(ctx, 1, 3))
	require.Empty(t, restarted.LayerXChainsKeeper.DomainTxs(ctx, 1, 2))
}
