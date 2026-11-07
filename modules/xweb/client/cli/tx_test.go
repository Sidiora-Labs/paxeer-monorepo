package cli_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
)

func runSetParams(t *testing.T, args ...string) (string, client.TxConfig, error) {
	t.Helper()
	registry := cdctypes.NewInterfaceRegistry()
	std.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	txConfig := authtx.NewTxConfig(cdc, authtx.DefaultSignModes)
	var out bytes.Buffer
	clientCtx := client.Context{}.
		WithCodec(cdc).
		WithInterfaceRegistry(registry).
		WithTxConfig(txConfig).
		WithLegacyAmino(codec.NewLegacyAmino()).
		WithChainID("paxeer-x-test").
		WithOutput(&out)
	cmd := cli.NewSetParamsCmd()
	cmd.SetArgs(append([]string{"1000000", "8192", "500000", "3600",
		"--from", sdk.AccAddress(bytes.Repeat([]byte{0x0b}, 20)).String(),
		"--generate-only",
		"--keyring-backend", "test",
		"--keyring-dir", t.TempDir(),
	}, args...))
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.ExecuteContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx))
	return out.String(), txConfig, err
}

func TestSetParamsCmdCarriesTheProgramPair(t *testing.T) {
	for _, tc := range []struct {
		name    string
		network string
		wantID  uint32
		denoms  []string
	}{
		{"all mapped denoms", "7", 7, []string{"uhpx", "usid", "uusdc", "uusdl"}},
		{"uint32 network id", "4294967295", 4294967295, []string{"uusdl", "uhpx"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			printed, txConfig, err := runSetParams(t,
				"--kernel-network-id", tc.network,
				"--program-fee-denoms", strings.Join(tc.denoms, ","))
			require.NoError(t, err, printed)
			decoded, err := txConfig.TxJSONDecoder()([]byte(strings.TrimSpace(printed)))
			require.NoError(t, err)
			msgs := decoded.GetMsgs()
			require.Len(t, msgs, 1)
			msg, ok := msgs[0].(*types.MsgSetParams)
			require.True(t, ok, "the transaction carries a %T", msgs[0])
			require.Equal(t, sdk.AccAddress(bytes.Repeat([]byte{0x0b}, 20)).String(), msg.Authority)
			require.Equal(t, sdk.NewInt(1_000_000), msg.Fee)
			require.Equal(t, uint32(8192), msg.MaxPayloadBytes)
			require.Equal(t, uint64(500000), msg.MaxCallbackGas)
			require.Equal(t, uint64(3600), msg.TimeoutBlocks)
			require.Equal(t, tc.wantID, msg.KernelNetworkId)
			require.Equal(t, tc.denoms, msg.ProgramFeeDenoms)
			require.NoError(t, msg.ValidateBasic())
		})
	}
}

func TestSetParamsCmdRefusesAnUnmappedFeeDenom(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"unmapped denom", []string{"--kernel-network-id", "7", "--program-fee-denoms", "uhpx,uatom"}, `program fee denom "uatom" is not one of`},
		{"missing network id", []string{"--program-fee-denoms", "uhpx"}, `required flag(s) "kernel-network-id" not set`},
		{"missing fee denoms", []string{"--kernel-network-id", "7"}, `required flag(s) "program-fee-denoms" not set`},
		{"zero network id", []string{"--kernel-network-id", "0", "--program-fee-denoms", "uhpx"}, "kernel network id is zero"},
		{"overflow network id", []string{"--kernel-network-id", "4294967296", "--program-fee-denoms", "uhpx"}, "value out of range"},
		{"empty fee denoms", []string{"--kernel-network-id", "7", "--program-fee-denoms", ""}, "no program fee denom"},
		{"duplicate fee denom", []string{"--kernel-network-id", "7", "--program-fee-denoms", "uhpx,uhpx"}, "duplicate program fee denom uhpx"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			printed, _, err := runSetParams(t, tc.args...)
			require.ErrorContains(t, err, tc.want)
			require.Empty(t, printed, "an invalid program pair must not generate a transaction")
		})
	}
}
