package apptesting

import (
	"testing"

	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1074_BuildTxRoundTrip(t *testing.T) {
	priv := secp256k1.GenPrivKeyFromSecret([]byte("repair-m1074-sender"))
	sender := sdk.AccAddress(priv.PubKey().Address())
	recipient := sdk.AccAddress(secp256k1.GenPrivKeyFromSecret([]byte("repair-m1074-recipient")).PubKey().Address())
	validator := sdk.ValAddress(recipient)
	send := banktypes.NewMsgSend(sender, recipient, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 17)))
	delegate := stakingtypes.NewMsgDelegate(sender, validator, sdk.NewInt64Coin(sdk.DefaultBondDenom, 29))
	last := banktypes.NewMsgSend(sender, recipient, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 41)))

	for _, tc := range []struct {
		name string
		msgs []sdk.Msg
	}{
		{name: "single", msgs: []sdk.Msg{send}},
		{name: "multiple", msgs: []sdk.Msg{send, delegate, last, send}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := app.MakeEncodingConfig().TxConfig
			builder := cfg.NewTxBuilder()
			helper := &KeeperTestHelper{}
			helper.SetT(t)
			fee := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 3))
			const memo = "repair m1074 complete message sequence"
			const gasLimit uint64 = 250000
			sig := signing.SignatureV2{
				PubKey: priv.PubKey(),
				Data: &signing.SingleSignatureData{
					SignMode: signing.SignMode_SIGN_MODE_DIRECT,
				},
				Sequence: 7,
			}
			tx := helper.BuildTx(builder, tc.msgs, sig, memo, fee, gasLimit)
			signerData := authsigning.SignerData{ChainID: "pax-test", AccountNumber: 11, Sequence: sig.Sequence}
			signBytes, err := cfg.SignModeHandler().GetSignBytes(signing.SignMode_SIGN_MODE_DIRECT, signerData, tx)
			require.NoError(t, err)
			signature, err := priv.Sign(signBytes)
			require.NoError(t, err)
			sig.Data = &signing.SingleSignatureData{SignMode: signing.SignMode_SIGN_MODE_DIRECT, Signature: signature}
			require.NoError(t, builder.SetSignatures(sig))

			encoded, err := cfg.TxEncoder()(tx)
			require.NoError(t, err)
			require.NotEmpty(t, encoded)
			decoded, err := cfg.TxDecoder()(encoded)
			require.NoError(t, err)
			require.NoError(t, decoded.ValidateBasic())
			msgs := decoded.GetMsgs()
			require.Len(t, msgs, len(tc.msgs))
			for i, want := range tc.msgs {
				require.IsType(t, want, msgs[i])
				require.Equal(t, want, msgs[i], "message %d changed during encoding", i)
				require.NoError(t, msgs[i].ValidateBasic())
			}
			signed, ok := decoded.(authsigning.Tx)
			require.True(t, ok)
			require.Equal(t, memo, signed.GetMemo())
			require.Equal(t, fee, signed.GetFee())
			require.Equal(t, gasLimit, signed.GetGas())
			require.Equal(t, []sdk.AccAddress{sender}, signed.GetSigners())
			signatures, err := signed.GetSignaturesV2()
			require.NoError(t, err)
			require.Equal(t, []signing.SignatureV2{sig}, signatures)
			decodedSignBytes, err := cfg.SignModeHandler().GetSignBytes(signing.SignMode_SIGN_MODE_DIRECT, signerData, signed)
			require.NoError(t, err)
			require.Equal(t, signBytes, decodedSignBytes)
			require.True(t, signatures[0].PubKey.VerifySignature(decodedSignBytes, signature))
		})
	}
}

func TestRepairM_1074_RejectsEmptyMessages(t *testing.T) {
	for _, tc := range []struct {
		name string
		msgs []sdk.Msg
	}{
		{name: "nil", msgs: nil},
		{name: "empty", msgs: []sdk.Msg{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := app.MakeEncodingConfig().TxConfig
			builder := cfg.NewTxBuilder()
			helper := &KeeperTestHelper{}
			helper.SetT(t)
			priv := secp256k1.GenPrivKeyFromSecret([]byte("repair-m1074-empty"))
			sender := sdk.AccAddress(priv.PubKey().Address())
			msg := banktypes.NewMsgSend(sender, sender, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 1)))
			sig := signing.SignatureV2{
				PubKey: priv.PubKey(),
				Data: &signing.SingleSignatureData{
					SignMode: signing.SignMode_SIGN_MODE_DIRECT,
				},
				Sequence: 9,
			}
			tx := helper.BuildTx(builder, []sdk.Msg{msg}, sig, "keep existing transaction", sdk.NewCoins(), 12345)
			before, err := cfg.TxEncoder()(tx)
			require.NoError(t, err)
			require.PanicsWithValue(t, "BuildTx requires at least one message", func() {
				helper.BuildTx(builder, tc.msgs, signing.SignatureV2{}, "replace", sdk.NewCoins(), 0)
			})
			after, err := cfg.TxEncoder()(builder.GetTx())
			require.NoError(t, err)
			require.Equal(t, before, after, "empty input must not mutate an existing transaction")
			decoded, err := cfg.TxDecoder()(after)
			require.NoError(t, err)
			require.Len(t, decoded.GetMsgs(), 1)
			require.Equal(t, msg, decoded.GetMsgs()[0])
		})
	}
}
