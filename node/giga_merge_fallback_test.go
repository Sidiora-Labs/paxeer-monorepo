package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func gigaV6120FailFast(t *testing.T, w *app.TestWrapper, txs ...[]byte) []*abci.ExecTxResult {
	t.Helper()
	require.True(t, w.App.GigaExecutorEnabled)
	require.True(t, w.App.GigaOCCEnabled)
	require.Equal(t, int64(26_300_000), w.Ctx.BlockHeight())
	return runMergeFallbackBlock(t, w, txs...)
}

func runMergeFallbackBlock(t *testing.T, w *app.TestWrapper, txs ...[]byte) []*abci.ExecTxResult {
	t.Helper()
	_, results, _, err := w.App.ProcessBlock(w.Ctx, txs, &app.BlockProcessRequest{
		Height: w.Ctx.BlockHeight(),
		Time:   w.Ctx.BlockTime(),
	}, abci.CommitInfo{}, false, nil)
	require.NoError(t, err)
	require.Len(t, results, len(txs))
	for _, result := range results {
		require.NotNil(t, result)
		require.Zero(t, result.Code, result.Log)
	}
	return results
}

func mergeFallbackCounter(t *testing.T) int64 {
	t.Helper()
	registry, ok := prometheus.DefaultGatherer.(prometheus.Collector)
	require.True(t, ok)
	metrics := make(chan prometheus.Metric)
	go func() {
		registry.Collect(metrics)
		close(metrics)
	}()
	var matches []prometheus.Metric
	for measurement := range metrics {
		if strings.Contains(measurement.Desc().String(), `fqName: "pax_chain_app_giga_fallback_to_v2_total"`) {
			matches = append(matches, measurement)
		}
	}
	require.LessOrEqual(t, len(matches), 1)
	var count int64
	for _, measurement := range matches {
		var sample dto.Metric
		require.NoError(t, measurement.Write(&sample))
		require.NotNil(t, sample.Counter)
		var scope string
		for _, label := range sample.Label {
			if label.GetName() == "otel_scope_name" {
				scope = label.GetValue()
			}
		}
		require.Equal(t, "app", scope)
		value := sample.Counter.GetValue()
		require.Equal(t, float64(int64(value)), value)
		count += int64(value)
	}
	return count
}

func TestGigaMergeAddressFallsBackToOrdinaryPath(t *testing.T) {
	for _, height := range []int64{1, 26_299_999, 26_300_000, 26_300_001} {
		t.Run(fmt.Sprint(height), func(t *testing.T) { runGigaMergeAddressFallback(t, height) })
	}
}

func runGigaMergeAddressFallback(t *testing.T, height int64) {
	previousOCC := app.EnableOCC
	previousSkipResultsHash := tmtypes.SkipLastResultsHashValidation.Load()
	app.EnableOCC = true
	t.Cleanup(func() {
		app.EnableOCC = previousOCC
		tmtypes.SkipLastResultsHashValidation.Store(previousSkipResultsHash)
	})

	const gas uint64 = 100_000
	timestamp := time.Unix(1_800_000_000, 0).UTC()
	validator := utils.NewTestAccounts(1)[0]
	payer := utils.NewSigner()
	address := common.HexToAddress(layerxledger.LedgerAddress)
	recipient := utils.NewSigner()
	owner, asset := [32]byte{1}, [32]byte{2}
	ledgerABI := layerxledger.GetABI()
	input, err := ledgerABI.Pack("accountId", owner, asset)
	require.NoError(t, err)
	expectedAccount, err := ledgerkeeper.AccountID(owner, asset)
	require.NoError(t, err)
	newBlock := func(giga bool) *app.TestWrapper {
		a := app.SetupWithSc(t, true, true, app.TestAppOpts{
			UseSc: true, EnableGiga: true, EnableGigaOCC: true,
		}, func(ba *baseapp.BaseApp) {
			ba.SetOccEnabled(true)
			ba.SetConcurrencyWorkers(1)
		})
		t.Cleanup(func() { require.NoError(t, a.Close()) })
		genesis, err := json.Marshal(app.NewDefaultGenesisState(app.MakeEncodingConfig().Marshaler))
		require.NoError(t, err)
		_, err = a.InitChain(context.Background(), &abci.RequestInitChain{
			ChainId:         chainsBlockChainID,
			Time:            timestamp,
			InitialHeight:   height,
			ConsensusParams: app.DefaultConsensusParams,
			AppStateBytes:   genesis,
		})
		require.NoError(t, err)
		a.GigaExecutorEnabled = giga
		ctx := a.NewContext(false, tmproto.Header{
			Height: height, ChainID: chainsBlockChainID, Time: timestamp,
			ProposerAddress: validator.PublicKey.Address(),
		})
		ctx = ctx.WithConsensusParams(a.GetConsensusParams(ctx))
		w := &app.TestWrapper{App: a, Ctx: ctx}
		w.App.DistrKeeper.SetPreviousProposerConsAddr(w.Ctx, sdk.ConsAddress(validator.PublicKey.Address()))
		w.App.UpgradeKeeper.SetDone(w.Ctx, app.MergeUpgrade)
		w.App.RefreshPrecompiles(w.Ctx)
		w.App.EvmKeeper.SetAddressMapping(w.Ctx, payer.AccountAddress, payer.EvmAddress)
		w.App.EvmKeeper.SetAddressMapping(w.Ctx, recipient.AccountAddress, recipient.EvmAddress)
		require.Contains(t, w.App.EvmKeeper.CustomPrecompiles(w.Ctx), address)
		funding := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1_000_000_000))
		require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", funding))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", payer.AccountAddress, funding))
		require.Equal(t, uint64(0), w.App.EvmKeeper.GetNonce(w.Ctx, payer.EvmAddress))
		return w
	}
	ordinary := newBlock(false)
	giga := newBlock(true)

	encode := func(nonce uint64, to common.Address, value *big.Int, data []byte) ([]byte, *ethtypes.Transaction) {
		t.Helper()
		chainID := ordinary.App.EvmKeeper.ChainID(ordinary.Ctx)
		signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
			Nonce: nonce, GasPrice: big.NewInt(500_000_000_000), Gas: gas,
			To: &to, Value: value, Data: data,
		}), ethtypes.LatestSignerForChainID(chainID), payer.EvmPrivateKey)
		require.NoError(t, err)
		txData, err := ethtx.NewTxDataFromTx(signed)
		require.NoError(t, err)
		msg, err := evmtypes.NewMsgEVMTransaction(txData)
		require.NoError(t, err)
		txConfig := app.MakeEncodingConfig().TxConfig
		builder := txConfig.NewTxBuilder()
		require.NoError(t, builder.SetMsgs(msg))
		builder.SetGasLimit(gas)
		tx, err := txConfig.TxEncoder()(builder.GetTx())
		require.NoError(t, err)
		return tx, signed
	}
	call, signed := encode(0, address, big.NewInt(0), input)
	transfer, _ := encode(1, recipient.EvmAddress, big.NewInt(1), nil)
	txs := [][]byte{call, transfer}

	before := mergeFallbackCounter(t)
	want := runMergeFallbackBlock(t, ordinary, txs...)
	require.Equal(t, before, mergeFallbackCounter(t))
	require.True(t, giga.App.GigaExecutorEnabled)
	require.True(t, giga.App.GigaOCCEnabled)
	var got []*abci.ExecTxResult
	if height == 26_300_000 {
		got = gigaV6120FailFast(t, giga, txs...)
	} else {
		got = runMergeFallbackBlock(t, giga, txs...)
	}
	require.Equal(t, before+1, mergeFallbackCounter(t))
	require.Equal(t, want, got)

	var msgData sdk.TxMsgData
	require.NoError(t, msgData.Unmarshal(got[0].Data))
	require.Len(t, msgData.Data, 1)
	var response evmtypes.MsgEVMTransactionResponse
	require.NoError(t, response.Unmarshal(msgData.Data[0].Data))
	require.Empty(t, response.VmError)
	require.Equal(t, signed.Hash().Hex(), response.Hash)
	require.Positive(t, response.GasUsed)
	values, err := ledgerABI.Unpack("accountId", response.ReturnData)
	require.NoError(t, err)
	require.Equal(t, []interface{}{expectedAccount}, values)
	for _, w := range []*app.TestWrapper{ordinary, giga} {
		require.Equal(t, uint64(2), w.App.EvmKeeper.GetNonce(w.Ctx, payer.EvmAddress))
		require.Equal(t, big.NewInt(0), w.App.EvmKeeper.GetBalance(w.Ctx, address[:]))
		require.Equal(t, big.NewInt(1), w.App.EvmKeeper.GetBalance(w.Ctx, recipient.AccountAddress))
	}
	require.Equal(t,
		ordinary.App.EvmKeeper.GetBalance(ordinary.Ctx, payer.AccountAddress),
		giga.App.EvmKeeper.GetBalance(giga.Ctx, payer.AccountAddress))
}
