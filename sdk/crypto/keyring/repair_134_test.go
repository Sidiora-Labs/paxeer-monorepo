//go:build ledger && test_ledger_mock
// +build ledger,test_ledger_mock

package keyring_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/legacy/legacytx"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
)

func TestRepair_134_LedgerSupportedAlgorithms(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []keyring.Option
		want string
	}{
		{name: "default", want: "secp256k1"},
		{
			name: "configured_sr25519",
			opts: []keyring.Option{func(options *keyring.Options) {
				options.SupportedAlgosLedger = keyring.SigningAlgoList{hd.Sr25519, hd.Secp256k1}
			}},
			want: "secp256k1",
		},
		{
			name: "sr25519_only",
			opts: []keyring.Option{func(options *keyring.Options) {
				options.SupportedAlgosLedger = keyring.SigningAlgoList{hd.Sr25519}
			}},
		},
		{
			name: "disabled",
			opts: []keyring.Option{func(options *keyring.Options) {
				options.SupportedAlgosLedger = nil
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kr := keyring.NewInMemory(tc.opts...)
			software, hardware := kr.SupportedAlgorithms()
			require.True(t, software.Contains(hd.Sr25519))
			require.True(t, software.Contains(hd.Secp256k1))
			require.Equal(t, tc.want, hardware.String())
			require.False(t, hardware.Contains(hd.Sr25519))
			_, err := keyring.NewSigningAlgoFromString(string(hd.Sr25519Type), hardware)
			require.Error(t, err)
			if tc.want != "" {
				algo, err := keyring.NewSigningAlgoFromString(string(hd.Secp256k1Type), hardware)
				require.NoError(t, err)
				require.Equal(t, hd.Secp256k1Type, algo.Name())
			}
		})
	}
}

func TestRepair_134_LedgerRejectsSr25519BeforeDevice(t *testing.T) {
	for _, configured := range []bool{false, true} {
		name := "default"
		var opts []keyring.Option
		if configured {
			name = "configured_sr25519"
			opts = append(opts, func(options *keyring.Options) {
				options.SupportedAlgosLedger = keyring.SigningAlgoList{hd.Sr25519, hd.Secp256k1}
			})
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			kr, err := keyring.New(t.Name(), keyring.BackendTest, dir, nil, opts...)
			require.NoError(t, err)
			info, err := kr.SaveLedgerKey("unsupported", hd.Sr25519, "cosmos", 118, 0, 0)
			require.ErrorIs(t, err, keyring.ErrUnsupportedSigningAlgo)
			require.Nil(t, info)
			require.NotContains(t, err.Error(), "failed to generate ledger key")
			keys, err := kr.List()
			require.NoError(t, err)
			require.Empty(t, keys)

			reopened, err := keyring.New(t.Name(), keyring.BackendTest, dir, nil)
			require.NoError(t, err)
			keys, err = reopened.List()
			require.NoError(t, err)
			require.Empty(t, keys)
			_, err = reopened.Key("unsupported")
			require.Error(t, err)
		})
	}
}

func TestRepair_134_LedgerRejectsUnsupportedDespiteAdvertisedListMutation(t *testing.T) {
	kr := keyring.NewInMemory()
	_, hardware := kr.SupportedAlgorithms()
	require.Len(t, hardware, 1)
	hardware[0] = hd.Sr25519
	info, err := kr.SaveLedgerKey("unsupported", hd.Sr25519, "cosmos", 118, 0, 0)
	require.ErrorIs(t, err, keyring.ErrUnsupportedSigningAlgo)
	require.Nil(t, info)
	keys, err := kr.List()
	require.NoError(t, err)
	require.Empty(t, keys)
}

func TestRepair_134_LedgerRespectsDisabledSecp256k1(t *testing.T) {
	kr := keyring.NewInMemory(func(options *keyring.Options) {
		options.SupportedAlgosLedger = nil
	})
	info, err := kr.SaveLedgerKey("disabled", hd.Secp256k1, "cosmos", 118, 0, 0)
	require.ErrorIs(t, err, keyring.ErrUnsupportedSigningAlgo)
	require.Nil(t, info)
}

func TestRepair_134_LedgerCreateReopenSign(t *testing.T) {
	dir := t.TempDir()
	created := func() keyring.Info {
		kr, err := keyring.New(t.Name(), keyring.BackendTest, dir, nil)
		require.NoError(t, err)
		_, hardware := kr.SupportedAlgorithms()
		algo, err := keyring.NewSigningAlgoFromString(string(hd.Secp256k1Type), hardware)
		require.NoError(t, err)
		info, err := kr.SaveLedgerKey("supported", algo, sdk.GetConfig().GetBech32AccountAddrPrefix(), sdk.GetConfig().GetCoinType(), 0, 0)
		require.NoError(t, err, "create the Ledger key through the package's tagged Ledger transport")
		require.Equal(t, keyring.TypeLedger, info.GetType())
		require.Equal(t, algo.Name(), info.GetAlgo())
		require.IsType(t, &secp256k1.PubKey{}, info.GetPubKey())
		require.Equal(t, string(info.GetAlgo()), info.GetPubKey().Type())
		return info
	}()

	reopened, err := keyring.New(t.Name(), keyring.BackendTest, dir, nil)
	require.NoError(t, err)
	restored, err := reopened.Key("supported")
	require.NoError(t, err)
	require.Equal(t, keyring.TypeLedger, restored.GetType())
	require.Equal(t, created.GetName(), restored.GetName())
	require.Equal(t, created.GetAlgo(), restored.GetAlgo())
	require.Equal(t, hd.Secp256k1Type, restored.GetAlgo())
	require.IsType(t, &secp256k1.PubKey{}, restored.GetPubKey())
	require.Equal(t, string(restored.GetAlgo()), restored.GetPubKey().Type())
	require.True(t, created.GetPubKey().Equals(restored.GetPubKey()))
	require.Equal(t, created.GetAddress(), restored.GetAddress())
	createdPath, err := created.GetPath()
	require.NoError(t, err)
	restoredPath, err := restored.GetPath()
	require.NoError(t, err)
	require.Equal(t, createdPath, restoredPath)
	require.Equal(t, hd.NewFundraiserParams(0, sdk.GetConfig().GetCoinType(), 0), restoredPath)

	msg := banktypes.NewMsgSend(restored.GetAddress(), restored.GetAddress(), sdk.NewCoins(sdk.NewInt64Coin("stake", 1)))
	require.NoError(t, msg.ValidateBasic())
	fee := legacytx.NewStdFee(200000, sdk.NewCoins(sdk.NewInt64Coin("stake", 1)))
	signBytes := legacytx.StdSignBytes("repair-134", 0, 0, 0, fee, []sdk.Msg{msg}, "ledger-algorithm")
	sig, pub, err := reopened.SignByAddress(restored.GetAddress(), signBytes)
	require.NoError(t, err)
	require.IsType(t, &secp256k1.PubKey{}, pub)
	require.Equal(t, string(restored.GetAlgo()), pub.Type())
	require.True(t, restored.GetPubKey().Equals(pub))
	require.Len(t, sig, 64)
	require.True(t, restored.GetPubKey().VerifySignature(signBytes, sig))
	require.True(t, pub.VerifySignature(signBytes, sig))
	changedBytes := legacytx.StdSignBytes("repair-134", 0, 1, 0, fee, []sdk.Msg{msg}, "ledger-algorithm")
	require.False(t, restored.GetPubKey().VerifySignature(changedBytes, sig))
}
