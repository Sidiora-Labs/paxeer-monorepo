package sequencer_test

import (
	"crypto/sha256"
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	bridgetypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	clienttx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	serverconfig "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/config"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

func newFeeTokenAcceptHarness(t *testing.T, payer utils.TestAcct, rate sdk.Dec) *harness {
	t.Helper()
	return newSourceHarness(t, openHeight, func(ctx sdk.Context, a *app.App) {
		resolutionBlockGas(ctx, a)
		require.NoError(t, a.SequencerKeeper.SetActivationHeight(ctx, openHeight))
		params := a.EvmKeeper.GetParams(ctx)
		params.FeeTokenEnabled = true
		params.FeeTokenDistribution = true
		params.MaxFeeTokenRateAge = evmtypes.DefaultMaxFeeTokenRateAge
		params.AllowedFeeDenoms = []evmtypes.AllowedFeeDenom{{
			Denom: bridgetypes.SidioraDenom(), Rate: rate, RateUpdateHeight: committedTip,
		}}
		a.EvmKeeper.SetParams(ctx, params)
		fees := a.ParamsKeeper.GetFeesParams(ctx)
		fees.AllowedFeeDenoms = append(fees.AllowedFeeDenoms, bridgetypes.SidioraDenom())
		a.ParamsKeeper.SetFeesParams(ctx, fees)
		coins := sdk.NewCoins(sdk.NewInt64Coin(bridgetypes.SidioraDenom(), sidFunding))
		require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", coins))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", payer.AccountAddress, coins))
		require.NoError(t, a.EvmKeeper.SetAccountFeeDenom(ctx, payer.EvmAddress, bridgetypes.SidioraDenom()))
	}, payer).harness
}

func signFeeTokenSend(t *testing.T, h *harness, ctx sdk.Context, payer, recipient utils.TestAcct, amount int64) []byte {
	t.Helper()
	account := h.app.AccountKeeper.GetAccount(ctx, payer.AccountAddress)
	require.NotNil(t, account)
	tc := h.app.GetTxConfig()
	builder := tc.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(banktypes.NewMsgSend(payer.AccountAddress, recipient.AccountAddress,
		sdk.NewCoins(sdk.NewInt64Coin("uhpx", 7)))))
	builder.SetGasLimit(floorGas)
	builder.SetFeeAmount(sdk.NewCoins(sdk.NewInt64Coin(bridgetypes.SidioraDenom(), amount)))
	mode := tc.SignModeHandler().DefaultMode()
	require.NoError(t, builder.SetSignatures(signing.SignatureV2{
		PubKey: payer.PrivateKey.PubKey(), Data: &signing.SingleSignatureData{SignMode: mode}, Sequence: account.GetSequence(),
	}))
	sig, err := clienttx.SignWithPrivKey(mode, authsigning.SignerData{
		ChainID: testChainID, AccountNumber: account.GetAccountNumber(), Sequence: account.GetSequence(),
	}, builder, payer.PrivateKey, tc, account.GetSequence())
	require.NoError(t, err)
	require.NoError(t, builder.SetSignatures(sig))
	bz, err := tc.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	return bz
}

func TestAcceptChargesFeeTokenInFeeDenom(t *testing.T) {
	for _, kind := range []string{"EVM", "Cosmos"} {
		t.Run(kind, func(t *testing.T) {
			payer, recipient := utils.NewSigner(), utils.NewSigner()
			h := newFeeTokenAcceptHarness(t, payer, sdk.NewDec(evmtypes.InitialSidioraBaseUnitsPerPax))
			floor := newFloor(t, h, serverconfig.DefaultMinGasPrices)
			ms := h.genesis.Add(time.Second).UnixMilli()
			require.NoError(t, floor.Open(openHeight, ms))
			denom := bridgetypes.SidioraDenom()
			collector := h.app.AccountKeeper.GetModuleAddress(authtypes.FeeCollectorName)
			require.NotNil(t, collector)
			before := h.openCtx()
			payerBefore := h.app.BankKeeper.GetBalance(before, payer.AccountAddress, denom).Amount
			collectorBefore := h.app.BankKeeper.GetBalance(before, collector, denom).Amount
			nativeBefore := h.uhpx(before, payer.AccountAddress)
			var tx []byte
			var activity [32]byte
			fee, sent := sdk.NewInt(67), sdk.NewInt(7)
			if kind == "EVM" {
				signed, bz := transfer(t, payer, 0, recipient.EvmAddress, big.NewInt(onePaxInWei))
				tx, activity = bz, signed.Hash()
				fee, sent = sdk.NewInt(32_697), sdk.NewInt(1_000_000)
			} else {
				tx = signFeeTokenSend(t, h, before, payer, recipient, fee.Int64())
				activity = sha256.Sum256(tx)
			}
			accepted, err := floor.Accept(tx)
			require.NoError(t, err)
			require.NotNil(t, accepted)
			require.Equal(t, abci.CodeTypeOK, accepted.Result.Code, accepted.Result.Log)
			require.Equal(t, sourceLastSeq+1, accepted.Seq)
			require.Equal(t, 1, accepted.TxIndex)
			require.Equal(t, activity, accepted.ActivityID)
			after := h.openCtx()
			require.Equal(t, payerBefore.Sub(fee), h.app.BankKeeper.GetBalance(after, payer.AccountAddress, denom).Amount)
			require.Equal(t, nativeBefore.Sub(sent), h.uhpx(after, payer.AccountAddress))
			if kind == "EVM" {
				require.Equal(t, fee, h.app.BankKeeper.GetBalance(after, state.GetCoinbaseAddress(accepted.TxIndex), denom).Amount)
				receipt, err := h.app.EvmKeeper.GetTransientReceipt(after, activity, uint64(accepted.TxIndex))
				require.NoError(t, err)
				require.Equal(t, uint64(transferGas), receipt.GasUsed)
				require.Equal(t, uint64(gasPriceWei), receipt.EffectiveGasPrice)
			}
			batch, err := floor.Seal(openHeight)
			require.NoError(t, err)
			require.Equal(t, [][]byte{tx}, batch.Txs)
			sealed, _, err := sequencer.SealBatch(h.app, batch, signSealBatch(t, sequencer.BatchFields(batch), testKeySeed, testKeyID))
			require.NoError(t, err)
			replay := h.processSequencedBlock(t, ms, sealed.Txs)
			require.Equal(t, abci.CodeTypeOK, replay.results[1].Code, replay.results[1].Log)
			for _, ctx := range []sdk.Context{sealed.Context(), replay.ctx} {
				require.Equal(t, payerBefore.Sub(fee), h.app.BankKeeper.GetBalance(ctx, payer.AccountAddress, denom).Amount)
				require.Equal(t, collectorBefore.Add(fee), h.app.BankKeeper.GetBalance(ctx, collector, denom).Amount)
				require.Equal(t, nativeBefore.Sub(sent), h.uhpx(ctx, payer.AccountAddress))
				if kind == "EVM" {
					requireWei(t, big.NewInt(onePaxInWei), h.weiBalance(ctx, recipient.EvmAddress))
				} else {
					require.Equal(t, sent, h.uhpx(ctx, recipient.AccountAddress))
				}
			}
			require.Equal(t, sealed.Results[1], replay.results[1])
		})
	}
}

func TestAcceptRefusesStaleFeeTokenRate(t *testing.T) {
	payer, recipient := utils.NewSigner(), utils.NewSigner()
	currentRate := sdk.NewDec(3_150_000)
	h := newFeeTokenAcceptHarness(t, payer, currentRate)
	floor := newFloor(t, h, "100000000uhpx")
	require.NoError(t, floor.Open(openHeight, h.genesis.Add(time.Second).UnixMilli()))
	ctx := h.openCtx()
	denom := bridgetypes.SidioraDenom()
	before := h.app.BankKeeper.GetBalance(ctx, payer.AccountAddress, denom).Amount
	sequence := h.app.AccountKeeper.GetAccount(ctx, payer.AccountAddress).GetSequence()
	stale := signFeeTokenSend(t, h, ctx, payer, recipient, 60)
	decoded, err := h.app.GetTxConfig().TxDecoder()(stale)
	require.NoError(t, err)
	oldValue, err := h.app.EvmKeeper.ConvertFeeFromDenom(sdk.NewInt(60), sdk.NewDec(3_000_000), false)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(20_000_000_000_000), oldValue)
	currentValue, err := h.app.EvmKeeper.ConvertFeeFromDenom(sdk.NewInt(60), currentRate, false)
	require.NoError(t, err)
	require.True(t, currentValue.LT(oldValue))
	requireRefused(t, floor.CheckNodeFloor(ctx, decoded), sdkerrors.ErrInsufficientFee)
	accepted, err := floor.Accept(stale)
	require.Nil(t, accepted)
	requireRefused(t, err, sdkerrors.ErrInsufficientFee)
	require.ErrorContains(t, err, "required: 20000000000000uhpx")
	ctx = h.openCtx()
	require.Equal(t, before, h.app.BankKeeper.GetBalance(ctx, payer.AccountAddress, denom).Amount)
	require.Equal(t, sequence, h.app.AccountKeeper.GetAccount(ctx, payer.AccountAddress).GetSequence())
	require.True(t, h.uhpx(ctx, recipient.AccountAddress).IsZero())
	current := signFeeTokenSend(t, h, ctx, payer, recipient, 63)
	accepted, err = floor.Accept(current)
	require.NoError(t, err)
	require.Equal(t, abci.CodeTypeOK, accepted.Result.Code, accepted.Result.Log)
	require.Equal(t, sourceLastSeq+1, accepted.Seq)
	require.Equal(t, 1, accepted.TxIndex)

	refreshed := h.openCtx()
	params := h.app.EvmKeeper.GetParams(refreshed)
	params.AllowedFeeDenoms[0].Rate = sdk.NewDec(3_307_500)
	params.AllowedFeeDenoms[0].RateUpdateHeight = openHeight
	require.NoError(t, h.app.EvmKeeper.ValidateFeeTokenRateUpdate(refreshed, params.AllowedFeeDenoms))
	h.app.EvmKeeper.SetParams(refreshed, params)
	rate, err := h.app.EvmKeeper.GetFeeTokenRate(refreshed, denom)
	require.NoError(t, err)
	require.Equal(t, sdk.NewDec(3_307_500), rate)
	decoded, err = h.app.GetTxConfig().TxDecoder()(current)
	require.NoError(t, err)
	requireRefused(t, floor.CheckNodeFloor(refreshed, decoded), sdkerrors.ErrInsufficientFee)
	fresh := signFeeTokenSend(t, h, refreshed, payer, recipient, 67)
	decoded, err = h.app.GetTxConfig().TxDecoder()(fresh)
	require.NoError(t, err)
	require.NoError(t, floor.CheckNodeFloor(refreshed, decoded))
	expired := refreshed.WithBlockHeight(openHeight + params.MaxFeeTokenRateAge + 1)
	require.ErrorIs(t, floor.CheckNodeFloor(expired, decoded), evmkeeper.ErrFeeTokenRateStale)
	batch, err := floor.Seal(openHeight)
	require.NoError(t, err)
	require.Equal(t, [][]byte{current}, batch.Txs)
	require.Equal(t, sourceLastSeq+1, batch.LastSeq)
	require.Equal(t, before.SubRaw(63), h.app.BankKeeper.GetBalance(batch.Ctx, payer.AccountAddress, denom).Amount)
	rate, err = h.app.EvmKeeper.GetFeeTokenRate(batch.Ctx, denom)
	require.NoError(t, err)
	require.Equal(t, currentRate, rate)
}
