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

func repair204BuildTx(t *testing.T, cfg client.TxConfig, estimate uint64, granter sdk.AccAddress) client.TxBuilder {
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

func repair204RoundTrip(t *testing.T, cfg client.TxConfig, source signing.Tx) signing.Tx {
	t.Helper()
	wire, err := cfg.TxEncoder()(source)
	require.NoError(t, err)
	decoded, err := cfg.TxDecoder()(wire)
	require.NoError(t, err)
	tx, ok := decoded.(signing.Tx)
	require.True(t, ok)
	return tx
}

func repair204AssertFields(t *testing.T, source, target signing.Tx) {
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

func TestRepair_204_ProtobufFeeFields(t *testing.T) {
	cfg := makeTxEncodingConfig().TxConfig
	for _, estimate := range []uint64{0, gasEstimate, ^uint64(0)} {
		t.Run(fmt.Sprintf("estimate_%d", estimate), func(t *testing.T) {
			producer := repair204BuildTx(t, cfg, estimate, addr2)
			source := repair204RoundTrip(t, cfg, producer.GetTx())
			require.Equal(t, addr2, source.FeeGranter())
			require.Equal(t, estimate, source.GetGasEstimate())
			target := repair204BuildTx(t, cfg, 1, addr1)
			// Populate the target's encoding cache before copying into it.
			repair204RoundTrip(t, cfg, target.GetTx())
			require.NoError(t, clienttx.CopyTx(source, target, false))
			consumer := repair204RoundTrip(t, cfg, target.GetTx())
			repair204AssertFields(t, source, consumer)
		})
	}
}

func TestRepair_204_UnsupportedFeeGranter(t *testing.T) {
	enc := makeTxEncodingConfig()
	protoCfg := enc.TxConfig
	aminoCfg := legacytx.StdTxConfig{Cdc: enc.Amino}
	producer := repair204BuildTx(t, protoCfg, gasEstimate, addr2)
	source := repair204RoundTrip(t, protoCfg, producer.GetTx())
	for _, ignoreSignatureError := range []bool{false, true} {
		t.Run(fmt.Sprintf("ignore_signature_error_%t", ignoreSignatureError), func(t *testing.T) {
			target := repair204BuildTx(t, aminoCfg, 1, nil)
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

func TestRepair_204_GasEstimateAcrossFormats(t *testing.T) {
	enc := makeTxEncodingConfig()
	protoCfg := enc.TxConfig
	aminoCfg := legacytx.StdTxConfig{Cdc: enc.Amino}
	producer := repair204BuildTx(t, protoCfg, gasEstimate, nil)
	source := repair204RoundTrip(t, protoCfg, producer.GetTx())
	legacy, err := clienttx.ConvertTxToStdTx(enc.Amino, source)
	require.NoError(t, err)
	legacyConsumer := repair204RoundTrip(t, aminoCfg, legacy)
	repair204AssertFields(t, source, legacyConsumer)
	target := protoCfg.NewTxBuilder()
	require.NoError(t, clienttx.CopyTx(legacyConsumer, target, false))
	consumer := repair204RoundTrip(t, protoCfg, target.GetTx())
	repair204AssertFields(t, source, consumer)
}

func TestRepair_204_EmptyFeeFieldsReplaceTarget(t *testing.T) {
	cfg := makeTxEncodingConfig().TxConfig
	producer := repair204BuildTx(t, cfg, 0, nil)
	source := repair204RoundTrip(t, cfg, producer.GetTx())
	target := repair204BuildTx(t, cfg, gasEstimate, addr2)
	repair204RoundTrip(t, cfg, target.GetTx())
	require.NoError(t, clienttx.CopyTx(source, target, false))
	consumer := repair204RoundTrip(t, cfg, target.GetTx())
	repair204AssertFields(t, source, consumer)
	require.Empty(t, consumer.FeeGranter())
	require.Zero(t, consumer.GetGasEstimate())
}
