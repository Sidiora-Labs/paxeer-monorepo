package tx_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	clienttx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/legacy/legacytx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
)

func buildFeeFieldsTx(t *testing.T, cfg client.TxConfig, estimate uint64, granter sdk.AccAddress) client.TxBuilder {
	t.Helper()
	factory := clienttx.Factory{}.
		WithTxConfig(cfg).
		WithChainID("repair-204").
		WithGas(gas).
		WithFees(fee.String()).
		WithMemo(memo).
		WithTimeoutHeight(timeoutHeight)
	builder, err := factory.BuildUnsignedTx(msg0, msg1)
	require.NoError(t, err)
	builder.SetGasEstimate(estimate)
	builder.SetFeeGranter(granter)
	return builder
}

func roundTripFeeFieldsTx(t *testing.T, cfg client.TxConfig, source signing.Tx) signing.Tx {
	t.Helper()
	wire, err := cfg.TxEncoder()(source)
	require.NoError(t, err)
	decoded, err := cfg.TxDecoder()(wire)
	require.NoError(t, err)
	tx, ok := decoded.(signing.Tx)
	require.True(t, ok)
	return tx
}

func assertCopiedTxFields(t *testing.T, source, target signing.Tx) {
	t.Helper()
	require.Equal(t, source.FeeGranter(), target.FeeGranter())
	require.Equal(t, source.GetGasEstimate(), target.GetGasEstimate())
	require.Equal(t, source.GetGas(), target.GetGas())
	require.Equal(t, source.GetFee(), target.GetFee())
	require.Equal(t, source.GetMemo(), target.GetMemo())
	require.Equal(t, source.GetTimeoutHeight(), target.GetTimeoutHeight())
	require.Equal(t, source.GetMsgs(), target.GetMsgs())
	require.Equal(t, source.GetSigners(), target.GetSigners())
}

func TestCopyTxProtobufFeeFields(t *testing.T) {
	cfg := makeTxEncodingConfig().TxConfig
	for _, estimate := range []uint64{0, gasEstimate, ^uint64(0)} {
		t.Run(fmt.Sprintf("estimate_%d", estimate), func(t *testing.T) {
			producer := buildFeeFieldsTx(t, cfg, estimate, addr2)
			source := roundTripFeeFieldsTx(t, cfg, producer.GetTx())
			require.Equal(t, addr2, source.FeeGranter())
			require.Equal(t, estimate, source.GetGasEstimate())
			target := buildFeeFieldsTx(t, cfg, 1, addr1)
			// Populate the target's encoding cache before copying into it.
			roundTripFeeFieldsTx(t, cfg, target.GetTx())
			require.NoError(t, clienttx.CopyTx(source, target, false))
			consumer := roundTripFeeFieldsTx(t, cfg, target.GetTx())
			assertCopiedTxFields(t, source, consumer)
		})
	}
}

func TestCopyTxUnsupportedFeeGranter(t *testing.T) {
	enc := makeTxEncodingConfig()
	protoCfg := enc.TxConfig
	aminoCfg := legacytx.StdTxConfig{Cdc: enc.Amino}
	producer := buildFeeFieldsTx(t, protoCfg, gasEstimate, addr2)
	source := roundTripFeeFieldsTx(t, protoCfg, producer.GetTx())
	for _, ignoreSignatureError := range []bool{false, true} {
		t.Run(fmt.Sprintf("ignore_signature_error_%t", ignoreSignatureError), func(t *testing.T) {
			target := buildFeeFieldsTx(t, aminoCfg, 1, nil)
			target.SetMemo("preserve existing transaction on refusal")
			before, err := aminoCfg.TxEncoder()(target.GetTx())
			require.NoError(t, err)
			err = clienttx.CopyTx(source, target, ignoreSignatureError)
			require.EqualError(t, err, "cannot copy fee granter to legacy StdTx")
			after, err := aminoCfg.TxEncoder()(target.GetTx())
			require.NoError(t, err)
			require.Equal(t, before, after)
		})
	}
	converted, err := clienttx.ConvertTxToStdTx(enc.Amino, source)
	require.EqualError(t, err, "cannot copy fee granter to legacy StdTx")
	require.Equal(t, legacytx.StdTx{}, converted)
}

func TestCopyTxGasEstimateAcrossFormats(t *testing.T) {
	enc := makeTxEncodingConfig()
	protoCfg := enc.TxConfig
	aminoCfg := legacytx.StdTxConfig{Cdc: enc.Amino}
	producer := buildFeeFieldsTx(t, protoCfg, gasEstimate, nil)
	source := roundTripFeeFieldsTx(t, protoCfg, producer.GetTx())
	legacy, err := clienttx.ConvertTxToStdTx(enc.Amino, source)
	require.NoError(t, err)
	legacyConsumer := roundTripFeeFieldsTx(t, aminoCfg, legacy)
	assertCopiedTxFields(t, source, legacyConsumer)
	target := protoCfg.NewTxBuilder()
	require.NoError(t, clienttx.CopyTx(legacyConsumer, target, false))
	consumer := roundTripFeeFieldsTx(t, protoCfg, target.GetTx())
	assertCopiedTxFields(t, source, consumer)
}

func TestCopyTxEmptyFeeFieldsReplaceTarget(t *testing.T) {
	cfg := makeTxEncodingConfig().TxConfig
	producer := buildFeeFieldsTx(t, cfg, 0, nil)
	source := roundTripFeeFieldsTx(t, cfg, producer.GetTx())
	target := buildFeeFieldsTx(t, cfg, gasEstimate, addr2)
	roundTripFeeFieldsTx(t, cfg, target.GetTx())
	require.NoError(t, clienttx.CopyTx(source, target, false))
	consumer := roundTripFeeFieldsTx(t, cfg, target.GetTx())
	assertCopiedTxFields(t, source, consumer)
	require.Empty(t, consumer.FeeGranter())
	require.Zero(t, consumer.GetGasEstimate())
}
