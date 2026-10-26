package app_test

import (
	"context"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	evmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/config"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func gigaV6120FailFast(t *testing.T, w *app.TestWrapper, tx []byte) *abci.ExecTxResult {
	t.Helper()
	require.True(t, w.App.GigaExecutorEnabled)
	require.True(t, w.App.GigaOCCEnabled)
	require.Equal(t, int64(26_300_000), w.Ctx.BlockHeight())
	return runMergeFallbackBlock(t, w, tx)
}

func runMergeFallbackBlock(t *testing.T, w *app.TestWrapper, tx []byte) *abci.ExecTxResult {
	t.Helper()
	_, results, _, err := w.App.ProcessBlock(w.Ctx, [][]byte{tx}, &app.BlockProcessRequest{
		Height: w.Ctx.BlockHeight(),
		Time:   w.Ctx.BlockTime(),
	}, abci.CommitInfo{}, false, nil)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NotNil(t, results[0])
	require.Zero(t, results[0].Code, results[0].Log)
	return results[0]
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
	previousOCC := app.EnableOCC
	previousSkipResultsHash := tmtypes.SkipLastResultsHashValidation.Load()
	app.EnableOCC = true
	t.Cleanup(func() {
		app.EnableOCC = previousOCC
		tmtypes.SkipLastResultsHashValidation.Store(previousSkipResultsHash)
	})

	const height int64 = 26_300_000
	const gas uint64 = 100_000
	timestamp := time.Unix(1_800_000_000, 0).UTC()
	validator := utils.NewTestAccounts(1)[0]
	payer := utils.NewSigner()
	address := common.HexToAddress("0x101A")
	newBlock := func(giga bool) *app.TestWrapper {
		a := app.SetupWithSc(t, true, true, app.TestAppOpts{
			UseSc: true, EnableGiga: true, EnableGigaOCC: true,
		}, func(ba *baseapp.BaseApp) {
			ba.SetOccEnabled(true)
			ba.SetConcurrencyWorkers(1)
		})
		genesis, err := json.Marshal(app.NewDefaultGenesisState(app.MakeEncodingConfig().Marshaler))
		require.NoError(t, err)
		_, err = a.InitChain(context.Background(), &abci.RequestInitChain{
			ChainId:         "pax-test",
			Time:            timestamp,
			InitialHeight:   height,
			ConsensusParams: app.DefaultConsensusParams,
			AppStateBytes:   genesis,
		})
		require.NoError(t, err)
		a.GigaExecutorEnabled = giga
		ctx := a.NewContext(false, tmproto.Header{
			Height: height, ChainID: "pax-test", Time: timestamp,
			ProposerAddress: validator.PublicKey.Address(),
		})
		w := &app.TestWrapper{App: a, Ctx: ctx}
		w.App.DistrKeeper.SetPreviousProposerConsAddr(w.Ctx, sdk.ConsAddress(validator.PublicKey.Address()))
		w.App.UpgradeKeeper.SetDone(w.Ctx, app.MergeUpgrade)
		w.App.RefreshPrecompiles(w.Ctx)
		w.App.EvmKeeper.SetAddressMapping(w.Ctx, payer.AccountAddress, payer.EvmAddress)
		funding := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1_000_000_000))
		require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", funding))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", payer.AccountAddress, funding))
		require.Equal(t, uint64(0), w.App.EvmKeeper.GetNonce(w.Ctx, payer.EvmAddress))
		return w
	}
	ordinary := newBlock(false)
	giga := newBlock(true)

	signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    0,
		GasPrice: big.NewInt(500_000_000_000),
		Gas:      gas,
		To:       &address,
		Value:    big.NewInt(1),
	}), ethtypes.LatestSignerForChainID(big.NewInt(evmconfig.DefaultChainID)), payer.EvmPrivateKey)
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

	before := mergeFallbackCounter(t)
	want := runMergeFallbackBlock(t, ordinary, tx)
	require.Equal(t, before, mergeFallbackCounter(t))
	got := gigaV6120FailFast(t, giga, tx)
	require.Equal(t, before+1, mergeFallbackCounter(t))
	require.Equal(t, want, got)

	var msgData sdk.TxMsgData
	require.NoError(t, msgData.Unmarshal(got.Data))
	require.Len(t, msgData.Data, 1)
	var response evmtypes.MsgEVMTransactionResponse
	require.NoError(t, response.Unmarshal(msgData.Data[0].Data))
	require.Empty(t, response.VmError)
	require.Equal(t, signed.Hash().Hex(), response.Hash)
	require.Positive(t, response.GasUsed)
	for _, w := range []*app.TestWrapper{ordinary, giga} {
		require.Equal(t, uint64(1), w.App.EvmKeeper.GetNonce(w.Ctx, payer.EvmAddress))
		require.Equal(t, big.NewInt(1), w.App.EvmKeeper.GetBalance(w.Ctx, address[:]))
	}
	require.Equal(t,
		ordinary.App.EvmKeeper.GetBalance(ordinary.Ctx, payer.AccountAddress),
		giga.App.EvmKeeper.GetBalance(giga.Ctx, payer.AccountAddress))
}
