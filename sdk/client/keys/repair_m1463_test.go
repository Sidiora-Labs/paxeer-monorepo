package keys

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"testing"

	store "github.com/99designs/keyring"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/legacy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
)

func TestRepairM_1463_PathFailure(t *testing.T) {
	kb := keyring.NewInMemory()
	pubKey := secp256k1.GenPrivKeyFromSecret([]byte(t.Name())).PubKey()
	_, err := kb.SavePubKey("offline", pubKey, hd.Secp256k1Type)
	require.NoError(t, err)
	info, err := kb.Key("offline")
	require.NoError(t, err)
	path, pathErr := info.GetPath()
	require.Nil(t, path)
	require.EqualError(t, pathErr, "BIP44 Paths are not available for this type")

	// Ledger metadata currently has an infallible GetPath. Exercise the shared
	// production device-display path with a real Info that cannot supply a path.
	err = showAddressOnDevice(info)
	require.EqualError(t, err, "failed to get BIP44 path from ledger info: "+pathErr.Error())
	require.EqualError(t, errors.Unwrap(err), pathErr.Error())
}

func repairM1463LedgerKeyring(t *testing.T) (keyring.Keyring, keyring.Info) {
	t.Helper()
	// Public metadata from Test_writeReadLedgerInfo in sdk/crypto/keyring,
	// decoded as the registered production ledgerInfo rather than a test Info.
	const metadata = `{"type":"crypto/keys/ledgerInfo","value":{"name":"some_name","pubkey":{"type":"tendermint/PubKeySecp256k1","value":"A1rWgQpH8HNVP/MNL8x+DTscC3S2GhqqJYI0QDcVHhQ6"},"path":{"purpose":44,"coinType":118,"account":5,"change":false,"addressIndex":1},"algo":"secp256k1"}}`
	var info keyring.Info
	require.NoError(t, legacy.Cdc.UnmarshalAsJSON([]byte(metadata), &info))
	require.Equal(t, keyring.TypeLedger, info.GetType())
	path, err := info.GetPath()
	require.NoError(t, err)
	require.Equal(t, "m/44'/118'/5'/0/1", path.String())

	data, err := legacy.Cdc.MarshalLengthPrefixed(info)
	require.NoError(t, err)
	backing := store.NewArrayKeyring(nil)
	key := info.GetName() + ".info"
	require.NoError(t, backing.Set(store.Item{Key: key, Data: data}))
	require.NoError(t, backing.Set(store.Item{
		Key:  hex.EncodeToString(info.GetAddress().Bytes()) + ".address",
		Data: []byte(key),
	}))
	kb := keyring.NewInMemoryWithKeyring(backing)
	restored, err := kb.Key(info.GetName())
	require.NoError(t, err)
	require.Equal(t, info.GetPubKey(), restored.GetPubKey())
	require.Equal(t, keyring.TypeLedger, restored.GetType())
	return kb, restored
}

func TestRepairM_1463_LedgerDeviceFailure(t *testing.T) {
	kb, info := repairM1463LedgerKeyring(t)
	const deviceErr = "ledger nano S: support for ledger devices is not available in this executable"
	for _, keyref := range []string{info.GetName(), info.GetAddress().String()} {
		for _, output := range []struct {
			name  string
			flags []string
		}{
			{name: "text"},
			{name: "address", flags: []string{"--address"}},
			{name: "json", flags: []string{"--output=json"}},
		} {
			t.Run(keyref+"/"+output.name, func(t *testing.T) {
				for _, device := range []bool{false, true} {
					cmd := ShowKeysCmd()
					cmd.Flags().AddFlagSet(Commands("").PersistentFlags())
					cmd.SilenceUsage = true
					var stdout, stderr bytes.Buffer
					cmd.SetOut(&stdout)
					cmd.SetErr(&stderr)
					args := append([]string{keyref}, output.flags...)
					if device {
						args = append(args, "--device")
					}
					cmd.SetArgs(args)
					clientCtx := client.Context{}.WithKeyring(kb)
					err := cmd.ExecuteContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx))
					require.Contains(t, stdout.String(), info.GetAddress().String())
					if device {
						// Host output is insufficient to report device confirmation.
						require.EqualError(t, err, deviceErr)
						require.Contains(t, stderr.String(), "Error: "+deviceErr)
					} else {
						require.NoError(t, err)
						require.Empty(t, stderr.String())
					}
				}
			})
		}
	}
}
