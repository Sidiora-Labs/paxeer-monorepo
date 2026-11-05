package tx_test

import (
	"bytes"
	"context"
	"io"
	"math"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	clienttx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	txtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/legacy/legacytx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
)

type txTestEncodingConfig struct {
	InterfaceRegistry codectypes.InterfaceRegistry
	TxConfig          client.TxConfig
	Amino             *codec.LegacyAmino
}

func makeTxEncodingConfig() txTestEncodingConfig {
	registry := codectypes.NewInterfaceRegistry()
	amino := codec.NewLegacyAmino()
	std.RegisterInterfaces(registry)
	std.RegisterLegacyAminoCodec(amino)
	banktypes.RegisterInterfaces(registry)
	banktypes.RegisterLegacyAminoCodec(amino)
	legacytx.RegisterLegacyAminoCodec(amino)
	return txTestEncodingConfig{
		InterfaceRegistry: registry,
		TxConfig:          authtx.NewTxConfig(codec.NewProtoCodec(registry), authtx.DefaultSignModes),
		Amino:             amino,
	}
}

func repairM1462Command(clientCtx client.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use: "send", SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return clienttx.GenerateOrBroadcastTxCLI(cmd.Context(), clientCtx, cmd.Flags(), msg0)
		},
	}
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func TestRepairM_1462_FactoryInvalidFlags(t *testing.T) {
	cfg := makeTxEncodingConfig()
	for _, tc := range []struct {
		flag, value string
	}{
		{flags.FlagGas, "not-gas"},
		{flags.FlagGas, "-1"},
		{flags.FlagGas, "1.5"},
		{flags.FlagGas, "18446744073709551616"},
		{flags.FlagFees, "not-fees"},
		{flags.FlagFees, "-1bam"},
		{flags.FlagFees, "1?"},
		{flags.FlagFees, "1bam,2bam"},
		{flags.FlagGasPrices, "not-prices"},
		{flags.FlagGasPrices, "-0.5bam"},
		{flags.FlagGasPrices, "0.5?"},
		{flags.FlagGasPrices, "0.5bam,0.6bam"},
		{flags.FlagGasPrices, "0.1234567890123456789bam"},
		{flags.FlagGasAdjustment, "-1"},
		{flags.FlagGasAdjustment, "NaN"},
		{flags.FlagGasAdjustment, "+Inf"},
		{flags.FlagGasAdjustment, "-Inf"},
	} {
		t.Run(tc.flag+"/"+tc.value, func(t *testing.T) {
			var output bytes.Buffer
			clientCtx := client.Context{TxConfig: cfg.TxConfig, ChainID: "repair-1462", GenerateOnly: true, Output: &output}
			cmd := repairM1462Command(clientCtx)
			require.NoError(t, cmd.Flags().Set(tc.flag, tc.value))
			factory, err := clienttx.NewFactoryCLI(clientCtx, cmd.Flags())
			require.ErrorContains(t, err, "--"+tc.flag)
			require.Equal(t, clienttx.Factory{}, factory)
			cmd.SetArgs([]string{"--" + tc.flag + "=" + tc.value})
			err = cmd.Execute()
			require.ErrorContains(t, err, "--"+tc.flag)
			require.Empty(t, output.String(), "invalid flags must not produce a transaction")

			// The submission branch must refuse the same input before accessing accounts or keys.
			clientCtx.GenerateOnly = false
			err = clienttx.GenerateOrBroadcastTxCLI(context.Background(), clientCtx, cmd.Flags(), msg0)
			require.ErrorContains(t, err, "--"+tc.flag)
			require.Empty(t, output.String())
		})
	}
}

func TestRepairM_1462_FactoryValidFlags(t *testing.T) {
	cfg := makeTxEncodingConfig()
	for _, tc := range []struct {
		name string
		args []string
		gas  uint64
		fees sdk.Coins
	}{
		{"defaults", nil, flags.DefaultGasLimit, nil},
		{"fees", []string{"--gas=17", "--fees=7bam", "--note=repair", "--timeout-height=8"}, 17, sdk.NewCoins(sdk.NewInt64Coin("bam", 7))},
		{"prices", []string{"--gas=3", "--gas-prices=0.5bam"}, 3, sdk.NewCoins(sdk.NewInt64Coin("bam", 2))},
		{"zero", []string{"--gas=0", "--gas-adjustment=0"}, 0, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			clientCtx := client.Context{TxConfig: cfg.TxConfig, ChainID: "repair-1462", GenerateOnly: true, Output: &output}
			cmd := repairM1462Command(clientCtx)
			cmd.SetArgs(tc.args)
			require.NoError(t, cmd.Execute())
			decoded, err := cfg.TxConfig.TxJSONDecoder()(output.Bytes())
			require.NoError(t, err)
			consumer := decoded.(signing.Tx)
			require.Equal(t, tc.gas, consumer.GetGas())
			require.True(t, tc.fees.IsEqual(consumer.GetFee()))
			require.Equal(t, []sdk.Msg{msg0}, consumer.GetMsgs())
			wire, err := cfg.TxConfig.TxEncoder()(decoded)
			require.NoError(t, err)
			roundTrip, err := cfg.TxConfig.TxDecoder()(wire)
			require.NoError(t, err)
			require.Equal(t, tc.gas, roundTrip.(sdk.FeeTx).GetGas())
			if tc.name == "fees" {
				require.Equal(t, "repair", consumer.GetMemo())
				require.Equal(t, uint64(8), consumer.GetTimeoutHeight())
			}
		})
	}
	cmd := repairM1462Command(client.Context{})
	require.NoError(t, cmd.Flags().Set(flags.FlagGas, "auto"))
	factory, err := clienttx.NewFactoryCLI(client.Context{}, cmd.Flags())
	require.NoError(t, err)
	require.True(t, factory.SimulateAndExecute())
	require.Zero(t, factory.Gas())
	require.Equal(t, flags.DefaultGasAdjustment, factory.GasAdjustment())
}

func repairM1462Simulation(t *testing.T, cfg txTestEncodingConfig, gasUsed uint64) (*grpc.ClientConn, <-chan sdk.Tx, *atomic.Uint32) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer()
	received := make(chan sdk.Tx, 1)
	calls := new(atomic.Uint32)
	simulate := func(wire []byte) (sdk.GasInfo, *sdk.Result, error) {
		calls.Add(1)
		decoded, err := cfg.TxConfig.TxDecoder()(wire)
		if err != nil {
			return sdk.GasInfo{}, nil, err
		}
		received <- decoded
		meter := sdk.NewInfiniteGasMeter(1, 1)
		meter.ConsumeGas(gasUsed, "repair-1462 simulation fixture workload")
		return sdk.GasInfo{GasWanted: decoded.(sdk.FeeTx).GetGas(), GasUsed: meter.GasConsumed()}, &sdk.Result{Data: []byte("repair-1462")}, nil
	}
	txtypes.RegisterServiceServer(server, authtx.NewTxServer(nil, cfg.TxConfig, simulate, cfg.InterfaceRegistry))
	serveResult := make(chan error, 1)
	go func() { serveResult <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		require.NoError(t, <-serveResult)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	return conn, received, calls
}

func TestRepairM_1462_GasAdjustmentBounds(t *testing.T) {
	cfg := makeTxEncodingConfig()
	for _, tc := range []struct {
		name   string
		gas    uint64
		factor float64
		want   uint64
		err    string
	}{
		{"fractional", 3, 1.5, 5, ""},
		{"sub-unit", 1, 0.01, 1, ""},
		{"integral", 10, 1.2, 12, ""},
		{"zero-gas", 0, 1.5, 0, ""},
		{"zero-factor", 17, 0, 0, ""},
		{"negative", 17, -1, 0, "gas adjustment"},
		{"nan", 17, math.NaN(), 0, "gas adjustment"},
		{"positive-infinity", 17, math.Inf(1), 0, "gas adjustment"},
		{"negative-infinity", 17, math.Inf(-1), 0, "gas adjustment"},
		{"nan-with-zero-gas", 0, math.NaN(), 0, "gas adjustment"},
		{"infinity-with-zero-gas", 0, math.Inf(1), 0, "gas adjustment"},
		{"product-overflow", 2, math.MaxFloat64, 0, "outside uint64 range"},
		{"uint64-boundary", uint64(1) << 63, 2, 0, "outside uint64 range"},
		{"finite-above-boundary", uint64(1) << 63, 3, 0, "outside uint64 range"},
		{"rounded-boundary", math.MaxUint64, 1, 0, "outside uint64 range"},
		{"largest-float-below-boundary", 1, math.Nextafter(math.Exp2(64), 0), math.MaxUint64 - 2047, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, received, calls := repairM1462Simulation(t, cfg, tc.gas)
			factory := clienttx.Factory{}.WithTxConfig(cfg.TxConfig).WithChainID("repair-1462").WithGas(19).
				WithMemo("simulation producer").WithGasAdjustment(tc.factor).WithSignMode(cfg.TxConfig.SignModeHandler().DefaultMode())
			response, adjusted, err := clienttx.CalculateGas(conn, factory, msg0)
			if tc.err != "" {
				require.ErrorContains(t, err, tc.err)
				require.Nil(t, response)
				require.Zero(t, adjusted)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, adjusted)
				require.Equal(t, tc.gas, response.GasInfo.GasUsed)
				require.Equal(t, uint64(19), response.GasInfo.GasWanted)
				require.Equal(t, []byte("repair-1462"), response.Result.Data)
				builder, err := factory.WithGas(adjusted).BuildUnsignedTx(msg0)
				require.NoError(t, err)
				wire, err := cfg.TxConfig.TxEncoder()(builder.GetTx())
				require.NoError(t, err)
				consumer, err := cfg.TxConfig.TxDecoder()(wire)
				require.NoError(t, err)
				require.Equal(t, tc.want, consumer.(sdk.FeeTx).GetGas())
			}
			if tc.err == "gas adjustment" {
				require.Zero(t, calls.Load(), "invalid factors must be rejected before simulation")
				return
			}
			require.Equal(t, uint32(1), calls.Load())
			select {
			case decoded := <-received:
				require.Equal(t, []sdk.Msg{msg0}, decoded.GetMsgs())
				require.Equal(t, "simulation producer", decoded.(sdk.TxWithMemo).GetMemo())
				sigs, err := decoded.(signing.SigVerifiableTx).GetSignaturesV2()
				require.NoError(t, err)
				require.Len(t, sigs, 1)
			default:
				t.Fatalf("simulation request was not decoded for %s", tc.name)
			}
		})
	}
}
