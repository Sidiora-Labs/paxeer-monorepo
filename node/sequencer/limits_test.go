package sequencer_test

import (
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

func admissionParams(maxGas, maxWanted, maxBytes int64) func(sdk.Context, *app.App) {
	return func(ctx sdk.Context, a *app.App) {
		cp := *app.DefaultConsensusParams
		cp.Block = &tmproto.BlockParams{MaxGas: maxGas, MaxGasWanted: maxWanted, MaxBytes: maxBytes}
		a.StoreConsensusParams(ctx, &cp)
	}
}

func admissionTx0(t *testing.T) []byte {
	t.Helper()
	msg := signSealBatch(t, sequencer.SequenceBatchFields{
		Height: uint64(openHeight), FromSeq: 1, ToSeq: 1,
	}, testKeySeed, testKeyID)
	bz, err := app.EncodeSequenceBatch(msg)
	require.NoError(t, err)
	return bz
}

func TestAcceptRefusesPastMaxGasWanted(t *testing.T) {
	for _, tc := range []struct {
		name              string
		maxGas, maxWanted int64
		limit             string
	}{
		{"max gas", 2*transferGas - 1, 10 * transferGas, "Block.MaxGas"},
		{"max wanted", 10 * transferGas, 2*transferGas - 1, "Block.MaxGasWanted"},
		{"zero gas", 0, 10 * transferGas, "Block.MaxGas"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sender, recipient := utils.NewSigner(), utils.NewSigner()
			h := newSourceHarness(t, openHeight, admissionParams(tc.maxGas, tc.maxWanted, 200000), sender)
			h.open()
			_, first := transfer(t, sender, 0, recipient.EvmAddress, big.NewInt(1))
			var accepted [][]byte
			if tc.maxGas != 0 {
				acc, err := h.exec.Accept(first)
				require.NoError(t, err)
				require.Equal(t, int64(transferGas), acc.Result.GasWanted)
				accepted = append(accepted, first)
			}
			ctx := h.openCtx()
			before := h.weiBalance(ctx, sender.EvmAddress)
			_, next := transfer(t, sender, uint64(len(accepted)), recipient.EvmAddress, big.NewInt(1))
			acc, err := h.exec.Accept(next)
			require.Nil(t, acc)
			require.ErrorIs(t, err, sequencer.ErrBlockFull)
			require.ErrorContains(t, err, tc.limit)
			requireWei(t, before, h.weiBalance(h.openCtx(), sender.EvmAddress))
			require.Equal(t, uint64(len(accepted)), h.app.EvmKeeper.GetNonce(h.openCtx(), sender.EvmAddress))
			batch, err := h.exec.Seal(openHeight)
			require.NoError(t, err)
			require.Equal(t, accepted, batch.Txs)
			require.Equal(t, sourceLastSeq+uint64(len(accepted)), batch.LastSeq)
		})
	}
	t.Run("Cosmos gas wanted and failed delivery", func(t *testing.T) {
		payer, sender, recipient := utils.NewSigner(), utils.NewSigner(), utils.NewSigner()
		h := newSourceHarness(t, openHeight, admissionParams(200000+transferGas, 1000000, 200000), payer, sender)
		h.open()
		_, gap := transfer(t, sender, 1, recipient.EvmAddress, big.NewInt(1))
		_, err := h.exec.Accept(gap)
		requireRefused(t, err, sdkerrors.ErrWrongSequence)
		bz := signCosmos(t, h.openCtx(), h.harness, payer, 200000,
			banktypes.NewMsgSend(payer.AccountAddress, recipient.AccountAddress, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1))))
		acc, err := h.exec.Accept(bz)
		require.NoError(t, err)
		require.Less(t, acc.Result.GasUsed, acc.Result.GasWanted)
		_, first := transfer(t, sender, 0, recipient.EvmAddress, big.NewInt(1))
		acc, err = h.exec.Accept(first)
		require.NoError(t, err)
		require.Equal(t, sourceLastSeq+2, acc.Seq)
		_, err = h.exec.Accept(gap)
		require.ErrorIs(t, err, sequencer.ErrBlockFull)
	})
}

func TestAcceptRefusesPastMaxBlockBytes(t *testing.T) {
	for _, short := range []int64{0, 1} {
		t.Run(big.NewInt(short).String()+" bytes short", func(t *testing.T) {
			sender, recipient := utils.NewSigner(), utils.NewSigner()
			_, first := transfer(t, sender, 0, recipient.EvmAddress, big.NewInt(1))
			_, second := transfer(t, sender, 1, recipient.EvmAddress, big.NewInt(1))
			reserve := int64(len(admissionTx0(t)))
			maxBytes := reserve + int64(len(first)+len(second)) - short
			h := newSourceHarness(t, openHeight, admissionParams(1000000, 1000000, maxBytes), sender)
			h.open()
			_, err := h.exec.Accept(first)
			require.NoError(t, err)
			want := [][]byte{first}
			if short == 0 {
				acc, err := h.exec.Accept(second)
				require.NoError(t, err)
				require.Equal(t, sourceLastSeq+2, acc.Seq)
				want = append(want, second)
			}
			before := h.weiBalance(h.openCtx(), sender.EvmAddress)
			_, next := transfer(t, sender, uint64(len(want)), recipient.EvmAddress, big.NewInt(1))
			acc, err := h.exec.Accept(next)
			require.Nil(t, acc)
			require.ErrorIs(t, err, sequencer.ErrBlockFull)
			require.ErrorContains(t, err, "Block.MaxBytes")
			requireWei(t, before, h.weiBalance(h.openCtx(), sender.EvmAddress))
			require.Equal(t, uint64(len(want)), h.app.EvmKeeper.GetNonce(h.openCtx(), sender.EvmAddress))
			batch, err := h.exec.Seal(openHeight)
			require.NoError(t, err)
			require.Equal(t, want, batch.Txs)
			require.Equal(t, sourceLastSeq+uint64(len(want)), batch.LastSeq)
			if short == 0 {
				require.Equal(t, maxBytes, reserve+int64(len(first)+len(second)))
			}
		})
	}
}

func TestSealedBlockPassesCheckTotalBlockGas(t *testing.T) {
	sender, recipient := utils.NewSigner(), utils.NewSigner()
	_, first := transfer(t, sender, 0, recipient.EvmAddress, big.NewInt(1))
	_, second := transfer(t, sender, 1, recipient.EvmAddress, big.NewInt(1))
	maxBytes := int64(len(admissionTx0(t)) + len(first) + len(second))
	h := newSourceHarness(t, openHeight, func(ctx sdk.Context, a *app.App) {
		admissionParams(2*transferGas, 2*transferGas, maxBytes)(ctx, a)
		require.NoError(t, a.SequencerKeeper.SetActivationHeight(ctx, openHeight))
	}, sender)
	ms := h.open()
	for _, bz := range [][]byte{first, second} {
		acc, err := h.exec.Accept(bz)
		require.NoError(t, err)
		require.Equal(t, uint32(0), acc.Result.Code, acc.Result.Log)
	}
	_, third := transfer(t, sender, 2, recipient.EvmAddress, big.NewInt(1))
	_, err := h.exec.Accept(third)
	require.ErrorIs(t, err, sequencer.ErrBlockFull)
	batch, err := h.exec.Seal(openHeight)
	require.NoError(t, err)
	require.Equal(t, int64(2*transferGas), batch.Results[0].GasWanted+batch.Results[1].GasWanted)
	msg := signSealBatch(t, sequencer.BatchFields(batch), testKeySeed, testKeyID)
	sealed, tx0, err := sequencer.SealBatch(h.app, batch, msg)
	require.NoError(t, err)
	require.Equal(t, maxBytes, int64(len(tx0)+len(first)+len(second)))
	require.Equal(t, abci.ResponseProcessProposal_ACCEPT,
		resolutionProposal(t, h.app, openHeight, time.UnixMilli(ms), sealed.Txs))
}
