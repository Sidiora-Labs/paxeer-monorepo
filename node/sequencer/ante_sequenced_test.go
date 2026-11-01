package sequencer_test

import (
	"crypto/sha256"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	oracletypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/antedecorators"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	clienttx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/teststaking"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

func Accept(t *testing.T, h *harness, tx []byte) (*sequencer.Accepted, error) {
	t.Helper()
	require.Equal(t, sequencer.ModeSequential, h.exec.Mode())
	require.True(t, h.app.Sequenced(h.openCtx(), openHeight))
	return h.exec.Accept(tx)
}

func replaySequencedAnteBlock(t *testing.T, h *harness) (*sequencer.SealedLayer, processedBlock) {
	t.Helper()
	batch, err := h.exec.Seal(openHeight)
	require.NoError(t, err)
	msg := signSealBatch(t, sequencer.BatchFields(batch), testKeySeed, testKeyID)
	sealed, tx0, err := sequencer.SealBatch(h.app, batch, msg)
	require.NoError(t, err)
	reference := processSealBatchBlock(t, h.app, batch, tx0)
	requireSameBlock(t, h.app, reference, sealed)
	return sealed, reference
}

func TestSequencedGaslessOracleVoteChargesNoFee(t *testing.T) {
	feeder := utils.NewSigner()
	var validator sdk.ValAddress
	h := newSourceHarness(t, openHeight, func(ctx sdk.Context, a *app.App) {
		resolutionBlockGas(ctx, a)
		require.NoError(t, a.SequencerKeeper.SetActivationHeight(ctx, openHeight))
		validators := a.StakingKeeper.GetAllValidators(ctx)
		require.Len(t, validators, 1)
		validatorPub := ed25519.GenPrivKey().PubKey()
		validator = sdk.ValAddress(validatorPub.Address())
		staking := teststaking.NewHelper(t, ctx, a.StakingKeeper)
		staking.Denom = a.StakingKeeper.GetParams(ctx).BondDenom
		stake := a.StakingKeeper.TokensFromConsensusPower(ctx, 100)
		funding := sdk.NewCoins(sdk.NewCoin(staking.Denom, stake))
		require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", funding))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", sdk.AccAddress(validator), funding))
		staking.CreateValidator(validator, validatorPub, stake, true)
		a.OracleKeeper.SetFeederDelegation(ctx, validator, feeder.AccountAddress)
		a.OracleKeeper.SetVoteTarget(ctx, "uatom")
	}, feeder).harness
	h.open()
	before := h.openCtx()
	require.NoError(t, h.app.OracleKeeper.ValidateFeeder(before, feeder.AccountAddress, validator))
	require.Equal(t, int64(100), h.app.StakingKeeper.GetLastValidatorPower(before, validator))
	_, err := h.app.OracleKeeper.GetAggregateExchangeRateVote(before, validator)
	require.ErrorIs(t, err, oracletypes.ErrNoAggregateVote)
	collector := h.app.AccountKeeper.GetModuleAddress(authtypes.FeeCollectorName)
	payerBalance := h.app.BankKeeper.GetAllBalances(before, feeder.AccountAddress)
	collectorBalance := h.app.BankKeeper.GetAllBalances(before, collector)
	account := h.app.AccountKeeper.GetAccount(before, feeder.AccountAddress)
	require.NotNil(t, account)
	sequence := account.GetSequence()
	msg := oracletypes.NewMsgAggregateExchangeRateVote("12.5uatom", feeder.AccountAddress, validator)
	tc := h.app.GetTxConfig()
	builder := tc.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msg))
	builder.SetGasLimit(0)
	builder.SetFeeAmount(sdk.NewCoins())
	mode := tc.SignModeHandler().DefaultMode()
	require.NoError(t, builder.SetSignatures(signing.SignatureV2{
		PubKey: feeder.PrivateKey.PubKey(), Data: &signing.SingleSignatureData{SignMode: mode}, Sequence: sequence,
	}))
	signature, err := clienttx.SignWithPrivKey(mode, authsigning.SignerData{
		ChainID: testChainID, AccountNumber: account.GetAccountNumber(), Sequence: sequence,
	}, builder, feeder.PrivateKey, tc, sequence)
	require.NoError(t, err)
	require.NoError(t, builder.SetSignatures(signature))
	tx, err := tc.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	decoded, err := tc.TxDecoder()(tx)
	require.NoError(t, err)
	feeTx, ok := decoded.(sdk.FeeTx)
	require.True(t, ok)
	require.True(t, feeTx.GetFee().IsZero())
	require.Zero(t, feeTx.GetGas())
	gasless, err := antedecorators.IsTxGasless(decoded, before, h.app.OracleKeeper, &h.app.EvmKeeper, h.app.PricefeedKeeper)
	require.NoError(t, err)
	require.True(t, gasless)

	accepted, err := Accept(t, h, tx)
	require.NoError(t, err)
	require.NotNil(t, accepted)
	require.Equal(t, abci.CodeTypeOK, accepted.Result.Code, accepted.Result.Log)
	require.Zero(t, accepted.Result.GasUsed)
	require.Equal(t, sourceLastSeq+1, accepted.Seq)
	require.Equal(t, sha256.Sum256(tx), accepted.ActivityID)
	after := h.openCtx()
	vote, err := h.app.OracleKeeper.GetAggregateExchangeRateVote(after, validator)
	require.NoError(t, err)
	tuples, err := oracletypes.ParseExchangeRateTuples(msg.ExchangeRates)
	require.NoError(t, err)
	require.Equal(t, oracletypes.NewAggregateExchangeRateVote(tuples, validator), vote)
	assertNoFee := func(ctx sdk.Context) {
		t.Helper()
		require.Equal(t, payerBalance, h.app.BankKeeper.GetAllBalances(ctx, feeder.AccountAddress))
		require.Equal(t, collectorBalance, h.app.BankKeeper.GetAllBalances(ctx, collector))
		require.Equal(t, sequence+1, h.app.AccountKeeper.GetAccount(ctx, feeder.AccountAddress).GetSequence())
	}
	assertNoFee(after)
	sealed, replay := replaySequencedAnteBlock(t, h)
	require.Equal(t, [][]byte{sealed.Txs[0], tx}, sealed.Txs)
	require.Equal(t, accepted.Result, replay.results[1])
	assertNoFee(sealed.Context())
	assertNoFee(replay.ctx)
	require.Equal(t, sequence, h.app.AccountKeeper.GetAccount(h.committedCtx(), feeder.AccountAddress).GetSequence())
}

func TestSequencedAnteRefusesBadSignature(t *testing.T) {
	sender, recipient := utils.NewSigner(), utils.NewSigner()
	source := newSourceHarness(t, openHeight, func(ctx sdk.Context, a *app.App) {
		resolutionBlockGas(ctx, a)
		require.NoError(t, a.SequencerKeeper.SetActivationHeight(ctx, openHeight))
	}, sender)
	h := source.harness
	h.open()
	before := h.openCtx()
	senderBalance := h.app.BankKeeper.GetBalance(before, sender.AccountAddress, "uhpx")
	recipientBalance := h.app.BankKeeper.GetBalance(before, recipient.AccountAddress, "uhpx")
	collector := h.app.AccountKeeper.GetModuleAddress(authtypes.FeeCollectorName)
	collectorBalance := h.app.BankKeeper.GetBalance(before, collector, "uhpx")
	sequence := h.app.AccountKeeper.GetAccount(before, sender.AccountAddress).GetSequence()
	const amount = int64(123)
	valid := signCosmos(t, before, h, sender, 200_000,
		banktypes.NewMsgSend(sender.AccountAddress, recipient.AccountAddress, utils.Funds(amount)))
	tc := h.app.GetTxConfig()
	decoded, err := tc.TxDecoder()(valid)
	require.NoError(t, err)
	signedTx, ok := decoded.(authsigning.SigVerifiableTx)
	require.True(t, ok)
	signatures, err := signedTx.GetSignaturesV2()
	require.NoError(t, err)
	require.Len(t, signatures, 1)
	data, ok := signatures[0].Data.(*signing.SingleSignatureData)
	require.True(t, ok)
	require.NotEmpty(t, data.Signature)
	corrupted := append([]byte(nil), data.Signature...)
	corrupted[0] ^= 1
	signatures[0].Data = &signing.SingleSignatureData{SignMode: data.SignMode, Signature: corrupted}
	builder, err := tc.WrapTxBuilder(decoded)
	require.NoError(t, err)
	require.NoError(t, builder.SetSignatures(signatures...))
	bad, err := tc.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	require.NotEqual(t, valid, bad)

	refused, err := Accept(t, h, bad)
	require.Nil(t, refused)
	requireRefused(t, err, sdkerrors.ErrUnauthorized)
	require.ErrorContains(t, err, "signature verification failed")
	refused, receipt, err := source.r.Accept(bad)
	require.Nil(t, refused)
	require.Empty(t, receipt)
	requireRefused(t, err, sdkerrors.ErrUnauthorized)
	blockResult, _ := h.deliverOnBranch(bad, 1)
	require.Equal(t, sdkerrors.ErrUnauthorized.ABCICode(), blockResult.Code)
	require.Equal(t, sdkerrors.ErrUnauthorized.Codespace(), blockResult.Codespace)
	require.Contains(t, blockResult.Log, "signature verification failed")
	afterRefusal := h.openCtx()
	requireSameState(t, h.app, before, afterRefusal)
	require.Equal(t, senderBalance, h.app.BankKeeper.GetBalance(afterRefusal, sender.AccountAddress, "uhpx"))
	require.Equal(t, recipientBalance, h.app.BankKeeper.GetBalance(afterRefusal, recipient.AccountAddress, "uhpx"))
	require.Equal(t, collectorBalance, h.app.BankKeeper.GetBalance(afterRefusal, collector, "uhpx"))
	require.Equal(t, sequence, h.app.AccountKeeper.GetAccount(afterRefusal, sender.AccountAddress).GetSequence())

	accepted, err := Accept(t, h, valid)
	require.NoError(t, err)
	require.NotNil(t, accepted)
	require.Equal(t, abci.CodeTypeOK, accepted.Result.Code, accepted.Result.Log)
	require.Equal(t, sourceLastSeq+1, accepted.Seq)
	require.Equal(t, sourcePrevSeqHash, accepted.PrevSeqHash)
	require.Equal(t, 1, accepted.TxIndex)
	sealed, replay := replaySequencedAnteBlock(t, h)
	require.Equal(t, [][]byte{sealed.Txs[0], valid}, sealed.Txs)
	require.Len(t, sealed.Results, 2)
	require.Equal(t, accepted.Result, replay.results[1])
	require.Equal(t, accepted.Seq, sealed.Batch.ToSeq)
	for _, ctx := range []sdk.Context{sealed.Context(), replay.ctx} {
		require.Equal(t, sequence+1, h.app.AccountKeeper.GetAccount(ctx, sender.AccountAddress).GetSequence())
		require.Equal(t, senderBalance.Amount.SubRaw(amount+cosmosFee), h.app.BankKeeper.GetBalance(ctx, sender.AccountAddress, "uhpx").Amount)
		require.Equal(t, recipientBalance.Amount.AddRaw(amount), h.app.BankKeeper.GetBalance(ctx, recipient.AccountAddress, "uhpx").Amount)
		require.Equal(t, collectorBalance.Amount.AddRaw(cosmosFee), h.app.BankKeeper.GetBalance(ctx, collector, "uhpx").Amount)
	}
}
