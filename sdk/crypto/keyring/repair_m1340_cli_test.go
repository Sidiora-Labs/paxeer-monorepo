package keyring_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func TestRepairM_1340_SR25519Show(t *testing.T) {
	dir := t.TempDir()
	created, err := func() (keyring.Info, error) {
		kr, err := keyring.New("repair-m1340-cli", keyring.BackendTest, dir, nil)
		if err != nil {
			return nil, err
		}
		return kr.NewAccount("sr-wallet", "alcohol woman abuse must during monitor noble actual mixed trade anger aisle", "", sdk.FullFundraiserPath, hd.Sr25519)
	}()
	require.NoError(t, err)

	for _, keyref := range []string{created.GetName(), created.GetAddress().String()} {
		t.Run(keyref, func(t *testing.T) {
			reopened, err := keyring.New("repair-m1340-cli", keyring.BackendTest, dir, nil)
			require.NoError(t, err)
			restored, err := reopened.Key("sr-wallet")
			require.NoError(t, err)
			require.IsType(t, keyring.LocalInfo{}, restored)
			expected, err := keyring.MkAccKeyOutput(restored)
			require.NoError(t, err)
			require.NotEmpty(t, expected.Address)
			require.Contains(t, expected.PubKey, "sr25519")

			ctx := client.Context{}.WithKeyring(reopened).WithHomeDir(dir).WithKeyringDir(dir)
			cmd := keys.Commands(dir)
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{"show", keyref, "--output", "json"})
			err = cmd.ExecuteContext(context.WithValue(context.Background(), client.ClientContextKey, &ctx))
			require.NoError(t, err, stderr.String())
			var out keyring.KeyOutput
			require.NoError(t, json.Unmarshal(stdout.Bytes(), &out))
			require.Equal(t, expected, out)
			require.Equal(t, "local", out.Type)
			require.Empty(t, out.EvmAddress)
			info, err := reopened.Key("sr-wallet")
			require.NoError(t, err)
			require.True(t, created.GetPubKey().Equals(info.GetPubKey()))
			inventory, err := reopened.List()
			require.NoError(t, err)
			require.Len(t, inventory, 1)
		})
	}
}
