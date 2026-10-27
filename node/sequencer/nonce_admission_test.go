package sequencer_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

type nonceAdmissionReceipt struct {
	sender common.Address
	nonce  uint64
	seq    uint64
}

func newNonceAdmission(t *testing.T, rh *receiptHarness, cfg sequencer.Config, lifetime time.Duration) (*sequencer.NonceAdmission, *[]nonceAdmissionReceipt) {
	t.Helper()
	rh.app.SetOpenBlock(rh.exec)
	var receipts []nonceAdmissionReceipt
	admission, err := sequencer.NewNonceAdmission(cfg, resolutionOpenNonces{a: rh.app},
		func(sender common.Address, nonce uint64, tx []byte) error {
			acc, signed, err := rh.r.Accept(tx)
			if err != nil {
				return err
			}
			verified, err := verify.ReceiptSignature(signed, rh.key.PublicKey())
			require.NoError(t, err)
			require.Equal(t, nonce, verified.Receipt.FromSequence)
			receipts = append(receipts, nonceAdmissionReceipt{sender, nonce, acc.Seq})
			return nil
		}, lifetime)
	require.NoError(t, err)
	return admission, &receipts
}

func TestNonceAdmissionReleasesHeldNonceInOrder(t *testing.T) {
	sender := utils.NewSigner()
	recipient := utils.NewSigner().EvmAddress
	rh := newReceiptHarness(t, 0, nil, sender)
	admission, receipts := newNonceAdmission(t, rh, sequencer.Config{
		NonceQueueMaxPerSender: 2,
		NonceQueueMaxTotal:     4,
	}, time.Minute)
	require.Empty(t, admission.Open(time.Now()))

	txs := make([][]byte, 3)
	for nonce := range txs {
		_, txs[nonce] = transfer(t, sender, uint64(nonce), recipient, big.NewInt(1))
	}
	for _, nonce := range []uint64{2, 1} {
		held, err := admission.Send(sender.EvmAddress, nonce, txs[nonce])
		require.NoError(t, err)
		require.True(t, held)
	}
	require.Empty(t, *receipts)
	require.Equal(t, uint64(0), rh.app.EvmKeeper.GetNonce(rh.openCtx(), sender.EvmAddress))

	_, wrongChain := signEVM(t, sender, config.DefaultChainID+1, 0, recipient, big.NewInt(1))
	held, err := admission.Send(sender.EvmAddress, 0, wrongChain)
	require.False(t, held)
	requireRefused(t, err, sdkerrors.ErrInvalidChainID)
	require.Empty(t, *receipts)

	held, err = admission.Send(sender.EvmAddress, 0, txs[0])
	require.NoError(t, err)
	require.False(t, held)
	require.Equal(t, []nonceAdmissionReceipt{
		{sender.EvmAddress, 0, 1},
		{sender.EvmAddress, 1, 2},
		{sender.EvmAddress, 2, 3},
	}, *receipts)
	require.Equal(t, uint64(3), rh.app.EvmKeeper.GetNonce(rh.openCtx(), sender.EvmAddress))
	requireWei(t, big.NewInt(3), rh.weiBalance(rh.openCtx(), recipient))

	_, err = admission.Send(sender.EvmAddress, 1, txs[1])
	require.ErrorIs(t, err, sequencer.ErrSameNonce)
	require.Len(t, *receipts, 3)
	batch, err := rh.exec.Seal(openHeight)
	require.NoError(t, err)
	require.Equal(t, txs, batch.Txs)
	require.Equal(t, uint64(3), batch.LastSeq)
	admission.Sealed()
}

func TestNonceAdmissionRefusesPastSenderBound(t *testing.T) {
	sender := utils.NewSigner()
	other := utils.NewSigner()
	recipient := utils.NewSigner().EvmAddress
	rh := newReceiptHarness(t, 0, nil, sender, other)
	admission, receipts := newNonceAdmission(t, rh, sequencer.Config{
		NonceQueueMaxPerSender: 2,
		NonceQueueMaxTotal:     3,
	}, time.Minute)
	require.Empty(t, admission.Open(time.Now()))

	txs := make([][]byte, 4)
	for nonce := range txs {
		_, txs[nonce] = transfer(t, sender, uint64(nonce), recipient, big.NewInt(1))
	}
	for _, nonce := range []uint64{1, 2} {
		held, err := admission.Send(sender.EvmAddress, nonce, txs[nonce])
		require.NoError(t, err)
		require.True(t, held)
	}
	held, err := admission.Send(sender.EvmAddress, 3, txs[3])
	require.False(t, held)
	require.ErrorIs(t, err, sequencer.ErrSenderQueueFull)
	require.Empty(t, *receipts)
	require.Equal(t, uint64(0), rh.app.EvmKeeper.GetNonce(rh.openCtx(), sender.EvmAddress))

	_, otherTx := transfer(t, other, 1, recipient, big.NewInt(1))
	held, err = admission.Send(other.EvmAddress, 1, otherTx)
	require.NoError(t, err)
	require.True(t, held)
	_, otherNext := transfer(t, other, 2, recipient, big.NewInt(1))
	held, err = admission.Send(other.EvmAddress, 2, otherNext)
	require.False(t, held)
	require.ErrorIs(t, err, sequencer.ErrQueueFull)

	held, err = admission.Send(sender.EvmAddress, 0, txs[0])
	require.NoError(t, err)
	require.False(t, held)
	require.Equal(t, []nonceAdmissionReceipt{
		{sender.EvmAddress, 0, 1},
		{sender.EvmAddress, 1, 2},
		{sender.EvmAddress, 2, 3},
	}, *receipts)
	held, err = admission.Send(sender.EvmAddress, 3, txs[3])
	require.NoError(t, err)
	require.False(t, held)
	require.Len(t, *receipts, 4)
	require.Equal(t, nonceAdmissionReceipt{sender.EvmAddress, 3, 4}, (*receipts)[3])

	dropped := admission.Open(time.Now().Add(2 * time.Minute))
	require.Len(t, dropped, 1)
	require.Equal(t, other.EvmAddress, dropped[0].Sender)
	require.Equal(t, uint64(1), dropped[0].Nonce)
	require.Equal(t, otherTx, dropped[0].Tx)
	require.Len(t, *receipts, 4)
	_, otherZero := transfer(t, other, 0, recipient, big.NewInt(1))
	held, err = admission.Send(other.EvmAddress, 0, otherZero)
	require.NoError(t, err)
	require.False(t, held)
	require.Len(t, *receipts, 5)
	require.Equal(t, uint64(1), rh.app.EvmKeeper.GetNonce(rh.openCtx(), other.EvmAddress))
	held, err = admission.Send(other.EvmAddress, 1, otherTx)
	require.NoError(t, err)
	require.False(t, held)
	require.Len(t, *receipts, 6)
	require.Equal(t, nonceAdmissionReceipt{other.EvmAddress, 1, 6}, (*receipts)[5])
}
