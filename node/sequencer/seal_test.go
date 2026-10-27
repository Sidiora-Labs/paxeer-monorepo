package sequencer_test

import (
	"math/big"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	bridgetypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

const (
	sealTransferGas       = uint64(21_000)
	sealFeeTokenConverted = int64(32_697)
	sealFeeTokenFunded    = int64(1_000_000)
)

type processedBlock struct {
	ctx     sdk.Context
	events  []abci.Event
	results []*abci.ExecTxResult
	end     abci.ResponseEndBlock
}

func processSealBatchBlock(t *testing.T, a *app.App, batch *sequencer.Batch, tx0 []byte) processedBlock {
	t.Helper()
	header := batch.Ctx.BlockHeader()
	ctx := sdk.NewContext(a.CommitMultiStore().CacheMultiStore(), header, false)
	ctx = ctx.WithConsensusParams(a.GetConsensusParams(ctx))
	txs := append([][]byte{tx0}, batch.Txs...)
	events, results, end, err := a.ProcessBlock(ctx, txs, &app.BlockProcessRequest{
		Height: header.Height,
		Time:   header.Time,
	}, abci.CommitInfo{}, false, nil)
	require.NoError(t, err)
	require.Len(t, results, len(txs))
	require.Equal(t, abci.ExecTxResult{}, *results[0])
	return processedBlock{ctx: ctx, events: events, results: results, end: end}
}

// requireSameState compares every mounted module store of the sealed layer with the ProcessBlock run.
func requireSameState(t *testing.T, a *app.App, want sdk.Context, got sdk.Context) {
	compared := 0
	for _, name := range keys.MemIAVLStoreKeys {
		key := a.GetKey(name)
		if key == nil {
			continue
		}
		utils.CompareStores(t, key, want.KVStore(key), got.KVStore(key), name)
		compared++
	}
	require.NotZero(t, compared)
}

// requireSameBlock compares results, events and the end-block response with the ProcessBlock run.
func requireSameBlock(t *testing.T, a *app.App, want processedBlock, got *sequencer.SealedLayer) {
	require.Equal(t, want.results, got.Results)
	require.Equal(t, want.events, got.Events)
	require.Equal(t, want.end, got.EndBlock)
	requireSameState(t, a, want.ctx, got.Context())
}

func TestSealBatchAppliesDeferredBalances(t *testing.T) {
	sender, recipient := utils.NewSigner(), utils.NewSigner()
	h := newSourceHarness(t, openHeight, func(ctx sdk.Context, a *app.App) {
		resolutionBlockGas(ctx, a)
		require.NoError(t, a.SequencerKeeper.SetActivationHeight(ctx, openHeight))
	}, sender).harness
	ctx := h.committedCtx()
	collector := h.app.AccountKeeper.GetModuleAddress(authtypes.FeeCollectorName)
	collectorBefore := h.app.BankKeeper.GetBalance(ctx, collector, "uhpx").Amount
	senderBefore := h.app.BankKeeper.GetBalance(ctx, sender.AccountAddress, "uhpx").Amount
	recipientBefore := h.app.BankKeeper.GetBalance(ctx, recipient.AccountAddress, "uhpx").Amount

	h.open()
	for i := int64(1); i <= 20; i++ {
		tx := signCosmos(t, h.openCtx(), h, sender, 200_000,
			banktypes.NewMsgSend(sender.AccountAddress, recipient.AccountAddress, utils.Funds(i)))
		acc, err := h.exec.Accept(tx)
		require.NoError(t, err)
		require.Equal(t, uint32(0), acc.Result.Code, acc.Result.Log)
	}
	batch, err := h.exec.Seal(openHeight)
	require.NoError(t, err)
	pending := 0
	h.app.BankKeeper.IterateDeferredBalances(batch.Ctx, func(sdk.AccAddress, sdk.Coin) bool {
		pending++
		return false
	})
	require.Positive(t, pending)

	msg := signSealBatch(t, sequencer.BatchFields(batch), testKeySeed, testKeyID)
	tx0, err := app.EncodeSequenceBatch(msg)
	require.NoError(t, err)
	reference := processSealBatchBlock(t, h.app, batch, tx0)
	sealed, gotTx0, err := sequencer.SealBatch(h.app, batch, msg)
	require.NoError(t, err)
	require.Equal(t, tx0, gotTx0)
	requireSameBlock(t, h.app, reference, sealed)

	got := sealed.Context()
	fees := sdk.NewInt(20 * cosmosFee)
	sent := sdk.NewInt(210)
	require.Equal(t, collectorBefore.Add(fees), h.app.BankKeeper.GetBalance(got, collector, "uhpx").Amount)
	require.Equal(t, senderBefore.Sub(fees).Sub(sent), h.app.BankKeeper.GetBalance(got, sender.AccountAddress, "uhpx").Amount)
	require.Equal(t, recipientBefore.Add(sent), h.app.BankKeeper.GetBalance(got, recipient.AccountAddress, "uhpx").Amount)
	pending = 0
	h.app.BankKeeper.IterateDeferredBalances(got, func(sdk.AccAddress, sdk.Coin) bool {
		pending++
		return false
	})
	require.Zero(t, pending)
}

func TestSealBatchRoutesCollectedFeeTokens(t *testing.T) {
	denom := bridgetypes.SidioraDenom()
	payer := utils.NewSigner()
	h := newSourceHarness(t, openHeight, func(ctx sdk.Context, a *app.App) {
		resolutionBlockGas(ctx, a)
		require.NoError(t, a.SequencerKeeper.SetActivationHeight(ctx, openHeight))
		keeper := &a.EvmKeeper
		params := keeper.GetParams(ctx)
		params.FeeTokenEnabled = true
		params.MaxFeeTokenRateAge = evmtypes.DefaultMaxFeeTokenRateAge
		params.AllowedFeeDenoms = []evmtypes.AllowedFeeDenom{{
			Denom:            denom,
			Rate:             sdk.NewDec(evmtypes.InitialSidioraBaseUnitsPerPax),
			RateUpdateHeight: committedTip,
		}}
		keeper.SetParams(ctx, params)
		funding := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(sealFeeTokenFunded)))
		require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", funding))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", payer.AccountAddress, funding))
		require.NoError(t, keeper.SetAccountFeeDenom(ctx, payer.EvmAddress, denom))
	}, payer)
	ctx := h.committedCtx()
	collector := h.app.AccountKeeper.GetModuleAddress(authtypes.FeeCollectorName)
	holding := h.app.AccountKeeper.GetModuleAddress(evmtypes.FeeTokenHoldingAccount)
	require.NotNil(t, holding)
	holdingBefore := h.app.BankKeeper.GetBalance(ctx, holding, denom).Amount

	h.open()
	recipient := common.HexToAddress("0x00000000000000000000000000000000000000de")
	for nonce := range uint64(3) {
		_, tx := transfer(t, payer, nonce, recipient, big.NewInt(0))
		acc, err := h.exec.Accept(tx)
		require.NoError(t, err)
		require.Equal(t, uint32(0), acc.Result.Code, acc.Result.Log)
	}
	batch, err := h.exec.Seal(openHeight)
	require.NoError(t, err)
	charged := sdk.NewInt(3 * sealFeeTokenConverted)
	require.Equal(t, holdingBefore, h.app.BankKeeper.GetBalance(batch.Ctx, holding, denom).Amount)
	collected := sdk.ZeroInt()
	for txIndex := range batch.Txs {
		collected = collected.Add(h.app.BankKeeper.GetBalance(batch.Ctx, state.GetCoinbaseAddress(txIndex+1), denom).Amount)
	}
	require.Equal(t, charged, collected)

	msg := signSealBatch(t, sequencer.BatchFields(batch), testKeySeed, testKeyID)
	tx0, err := app.EncodeSequenceBatch(msg)
	require.NoError(t, err)
	reference := processSealBatchBlock(t, h.app, batch, tx0)
	sealed, gotTx0, err := sequencer.SealBatch(h.app, batch, msg)
	require.NoError(t, err)
	require.Equal(t, tx0, gotTx0)
	requireSameBlock(t, h.app, reference, sealed)

	got := sealed.Context()
	require.Equal(t, holdingBefore.Add(charged), h.app.BankKeeper.GetBalance(got, holding, denom).Amount)
	require.True(t, h.app.BankKeeper.GetBalance(got, collector, denom).Amount.IsZero())
	require.Equal(t, sdk.NewInt(sealFeeTokenFunded).Sub(charged), h.app.BankKeeper.GetBalance(got, payer.AccountAddress, denom).Amount)
}
