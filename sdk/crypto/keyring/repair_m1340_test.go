package keyring

import (
	"testing"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/multisig"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

const repairM1340Mnemonic = "alcohol woman abuse must during monitor noble actual mixed trade anger aisle"

func repairM1340Store(t *testing.T, dir string) Keyring {
	t.Helper()
	kr, err := New("repair-m1340", BackendTest, dir, nil)
	require.NoError(t, err)
	return kr
}

func TestRepairM_1340_WrongPassword(t *testing.T) {
	for _, algo := range []SignatureAlgo{hd.Secp256k1, hd.Sr25519} {
		t.Run(string(algo.Name()), func(t *testing.T) {
			source := repairM1340Store(t, t.TempDir())
			created, err := source.NewAccount("source", repairM1340Mnemonic, "", sdk.FullFundraiserPath, algo)
			require.NoError(t, err)
			armor, err := source.ExportPrivKeyArmorByAddress(created.GetAddress(), "correct-password")
			require.NoError(t, err)

			dir := t.TempDir()
			target := repairM1340Store(t, dir)
			_, _, err = target.NewMnemonic("existing", English, sdk.FullFundraiserPath, "", hd.Secp256k1)
			require.NoError(t, err)
			before, err := target.List()
			require.NoError(t, err)
			raw, _, err := crypto.UnarmorDecryptPrivKey(armor, "wrong-password")
			require.ErrorIs(t, err, sdkerrors.ErrWrongPassword)
			require.Empty(t, raw)
			space, code, _ := sdkerrors.ABCIInfo(err, false)
			require.Equal(t, sdkerrors.RootCodespace, space)
			require.Equal(t, uint32(23), code)

			err = target.ImportPrivKey("imported", armor, "wrong-password")
			require.ErrorIs(t, err, sdkerrors.ErrWrongPassword)
			space, code, _ = sdkerrors.ABCIInfo(err, false)
			require.Equal(t, sdkerrors.RootCodespace, space)
			require.Equal(t, uint32(23), code)
			after, err := target.List()
			require.NoError(t, err)
			require.Equal(t, before, after)
			target = repairM1340Store(t, dir)
			after, err = target.List()
			require.NoError(t, err)
			require.Equal(t, before, after)
			_, err = target.Key("imported")
			require.ErrorIs(t, err, sdkerrors.ErrKeyNotFound)
			_, err = target.KeyByAddress(created.GetAddress())
			require.ErrorIs(t, err, sdkerrors.ErrKeyNotFound)

			require.NoError(t, target.ImportPrivKey("imported", armor, "correct-password"))
			target = repairM1340Store(t, dir)
			restored, err := target.Key("imported")
			require.NoError(t, err)
			require.True(t, created.GetPubKey().Equals(restored.GetPubKey()))
			require.Equal(t, algo.Name(), restored.GetAlgo())
			msg := []byte("restored armored identity")
			sig, pub, err := target.Sign("imported", msg)
			require.NoError(t, err)
			require.True(t, pub.VerifySignature(msg, sig))
			after, err = target.List()
			require.NoError(t, err)
			require.Len(t, after, len(before)+1)
		})
	}

	t.Run("existing-export-import", TestKeyringKeybaseExportImportPrivKey)
	t.Run("existing-name-export", TestAltKeyring_ImportExportPrivKey)
	t.Run("existing-address-export", TestAltKeyring_ImportExportPrivKey_ByAddress)
}

func TestRepairM_1340_OfflineShape(t *testing.T) {
	// Execute the updated legacy test as part of the bounded repair gate too.
	t.Run("existing-save-public-key", TestAltKeyring_SavePubKey)
	for _, pub := range []cryptotypes.PubKey{ed25519.GenPrivKey().PubKey(), secp256k1.GenPrivKey().PubKey()} {
		t.Run(pub.Type(), func(t *testing.T) {
			dir := t.TempDir()
			kr := repairM1340Store(t, dir)
			wrong := hd.Secp256k1Type
			if pub.Type() == string(wrong) {
				wrong = hd.Ed25519Type
			}
			_, err := kr.SavePubKey("bad-label", pub, wrong)
			require.ErrorIs(t, err, ErrUnsupportedSigningAlgo)
			kr = repairM1340Store(t, dir)
			list, err := kr.List()
			require.NoError(t, err)
			require.Empty(t, list)
			_, err = kr.Key("bad-label")
			require.ErrorIs(t, err, sdkerrors.ErrKeyNotFound)
			_, err = kr.KeyByAddress(sdk.AccAddress(pub.Address()))
			require.ErrorIs(t, err, sdkerrors.ErrKeyNotFound)

			saved, err := kr.SavePubKey("watch", pub, hd.PubKeyType(pub.Type()))
			require.NoError(t, err)
			kr = repairM1340Store(t, dir)
			restored, err := kr.KeyByAddress(saved.GetAddress())
			require.NoError(t, err)
			require.True(t, pub.Equals(restored.GetPubKey()))
			require.Equal(t, hd.PubKeyType(pub.Type()), restored.GetAlgo())
			require.Equal(t, TypeOffline, restored.GetType())
			list, err = kr.List()
			require.NoError(t, err)
			out, err := MkAccKeysOutput(list)
			require.NoError(t, err)
			require.Len(t, out, 1)
			require.Equal(t, "watch", out[0].Name)
			require.Equal(t, "offline", out[0].Type)
			require.Equal(t, saved.GetAddress().String(), out[0].Address)
			expected, err := MkAccKeyOutput(saved)
			require.NoError(t, err)
			require.Equal(t, expected.PubKey, out[0].PubKey)
			if pub.Type() == string(hd.Ed25519Type) {
				require.Empty(t, out[0].EvmAddress)
			} else {
				require.NotEmpty(t, out[0].EvmAddress)
			}
		})
	}
}

func TestRepairM_1340_PublicAddress(t *testing.T) {
	dir := t.TempDir()
	kr := repairM1340Store(t, dir)
	created, err := kr.NewAccount("local", repairM1340Mnemonic, "", sdk.FullFundraiserPath, hd.Secp256k1)
	require.NoError(t, err)
	require.IsType(t, &LocalInfo{}, created)
	secretHex, err := NewUnsafe(kr).UnsafeExportPrivKeyHex("local")
	require.NoError(t, err)
	secret, err := ethcrypto.HexToECDSA(secretHex)
	require.NoError(t, err)
	expectedAddress := ethcrypto.PubkeyToAddress(secret.PublicKey).Hex()
	require.NotEmpty(t, expectedAddress)

	reopened, err := repairM1340Store(t, dir).Key("local")
	require.NoError(t, err)
	require.IsType(t, LocalInfo{}, reopened)
	watchDir := t.TempDir()
	watch, err := repairM1340Store(t, watchDir).SavePubKey("watch", created.GetPubKey(), hd.Secp256k1Type)
	require.NoError(t, err)
	reopenedWatch, err := repairM1340Store(t, watchDir).Key("watch")
	require.NoError(t, err)

	// Exercise the production persisted Ledger-reference producer; display does
	// not discover or sign with a device and needs only the cached public key.
	ledgerDir := t.TempDir()
	ledgerStore := repairM1340Store(t, ledgerDir)
	path := *hd.NewFundraiserParams(0, sdk.CoinType, 0)
	hardware, err := ledgerStore.(keystore).writeLedgerKey("hardware", created.GetPubKey(), path, hd.Secp256k1Type)
	require.NoError(t, err)
	reopenedHardware, err := repairM1340Store(t, ledgerDir).Key("hardware")
	require.NoError(t, err)
	require.Equal(t, TypeLedger, reopenedHardware.GetType())
	restoredPath, err := reopenedHardware.GetPath()
	require.NoError(t, err)
	require.Equal(t, path, *restoredPath)

	for name, info := range map[string]Info{
		"created": created, "reopened": reopened,
		"watch": watch, "reopened-watch": reopenedWatch,
		"hardware": hardware, "reopened-hardware": reopenedHardware,
	} {
		t.Run(name, func(t *testing.T) {
			base, err := MkAccKeyOutput(info)
			require.NoError(t, err)
			out, err := PopulateEvmAddrIfApplicable(info, base)
			require.NoError(t, err)
			require.Equal(t, expectedAddress, out.EvmAddress)
			out.EvmAddress = ""
			require.Equal(t, base, out)
			listOut, err := MkAccKeysOutput([]Info{info})
			require.NoError(t, err)
			require.Len(t, listOut, 1)
			require.Equal(t, expectedAddress, listOut[0].EvmAddress)
		})
	}
	// A public-only LocalInfo still displays, with either Go representation.
	publicOnly := *created.(*LocalInfo)
	publicOnly.PrivKeyArmor = ""
	for _, info := range []Info{publicOnly, &publicOnly} {
		base, err := MkAccKeyOutput(info)
		require.NoError(t, err)
		out, err := PopulateEvmAddrIfApplicable(info, base)
		require.NoError(t, err)
		require.Equal(t, expectedAddress, out.EvmAddress)
	}
}

func TestRepairM_1340_SR25519Output(t *testing.T) {
	dir := t.TempDir()
	created, err := repairM1340Store(t, dir).NewAccount("sr-wallet", repairM1340Mnemonic, "", sdk.FullFundraiserPath, hd.Sr25519)
	require.NoError(t, err)
	restored, err := repairM1340Store(t, dir).Key("sr-wallet")
	require.NoError(t, err)
	require.IsType(t, LocalInfo{}, restored)
	multi, err := NewMultiInfo("multi", multisig.NewLegacyAminoPubKey(1, []cryptotypes.PubKey{created.GetPubKey()}))
	require.NoError(t, err)
	for _, info := range []Info{created, restored, newOfflineInfo("ed", ed25519.GenPrivKey().PubKey(), hd.Ed25519Type), multi} {
		base, err := MkAccKeyOutput(info)
		require.NoError(t, err)
		out, err := PopulateEvmAddrIfApplicable(info, base)
		require.NoError(t, err)
		require.Equal(t, base, out)
		require.Empty(t, out.EvmAddress)
		listOut, err := MkAccKeysOutput([]Info{info})
		require.NoError(t, err)
		require.Equal(t, []KeyOutput{base}, listOut)
	}
}
