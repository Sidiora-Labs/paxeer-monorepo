package cli

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
)

func TestRepair463_MintGasAdjustment(t *testing.T) {
	registry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	txConfig := authtx.NewTxConfig(cdc, authtx.DefaultSignModes)
	sender := sdk.AccAddress(bytes.Repeat([]byte{1}, 20)).String()
	denom, err := types.GetTokenDenom(sender, "repair463")
	require.NoError(t, err)
	for _, adjustment := range []string{"-1", "NaN", "+Inf", "-Inf", "1.25"} {
		t.Run(adjustment, func(t *testing.T) {
			var output bytes.Buffer
			clientCtx := client.Context{}.WithCodec(cdc).WithInterfaceRegistry(registry).
				WithTxConfig(txConfig).WithChainID("repair-463").WithOutput(&output)
			root := GetTxCmd()
			root.SilenceErrors = true
			root.SilenceUsage = true
			root.SetOut(&output)
			root.SetErr(&output)
			root.SetArgs([]string{"mint", "17" + denom, "--from=" + sender,
				"--generate-only", "--offline", "--keyring-backend=memory", "--node=",
				"--gas-adjustment=" + adjustment, "--gas=123456", "--note=valid factory"})
			err := root.ExecuteContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx))
			if adjustment != "1.25" {
				require.EqualError(t, err, fmt.Sprintf("invalid --gas-adjustment: gas adjustment must be finite and non-negative: %s", adjustment))
				require.Empty(t, output.Bytes())
				return
			}
			require.NoError(t, err)
			generated, err := txConfig.TxJSONDecoder()(output.Bytes())
			require.NoError(t, err)
			require.Equal(t, uint64(123456), generated.(authsigning.Tx).GetGas())
			require.Equal(t, "valid factory", generated.(authsigning.Tx).GetMemo())
			msgs := generated.GetMsgs()
			require.Len(t, msgs, 1)
			msg, ok := msgs[0].(*types.MsgMint)
			require.True(t, ok, "unexpected message type %T", msgs[0])
			require.Equal(t, sender, msg.Sender)
			require.Equal(t, sdk.NewInt64Coin(denom, 17), msg.Amount)
			require.NoError(t, msg.ValidateBasic())
		})
	}
}
