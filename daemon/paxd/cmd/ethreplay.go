package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	//nolint:gosec
	_ "net/http/pprof"
	"strconv"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	wasmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/keeper"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"

	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
)

//nolint:gosec
func ReplayCmd(defaultNodeHome string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ethreplay",
		Short: "replay EVM transactions",
		Long:  "replay EVM transactions",
		RunE: func(cmd *cobra.Command, _ []string) error {

			serverCtx := server.GetServerContextFromCmd(cmd)
			if err := serverCtx.Viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			profiler, err := startReplayProfiler(cmd.Context(), 6060)
			if err != nil {
				return err
			}
			defer profiler.Close()

			home := serverCtx.Viper.GetString(flags.FlagHome)

			cache := store.NewCommitKVStoreCacheManager()
			wasmGasRegisterConfig := wasmkeeper.DefaultGasRegisterConfig()
			wasmGasRegisterConfig.GasMultiplier = 21_000_000
			a := app.New(
				nil,
				nil,
				true,
				map[int64]bool{},
				home,
				0,
				true,
				nil,
				app.MakeEncodingConfig(),
				wasm.EnableAllProposals,
				serverCtx.Viper,
				[]wasm.Option{
					wasmkeeper.WithGasRegister(
						wasmkeeper.NewWasmGasRegister(
							wasmGasRegisterConfig,
						),
					),
				},
				app.EmptyAppOptions,
				baseapp.SetPruning(storetypes.PruneEverything),
				baseapp.SetMinGasPrices(cast.ToString(serverCtx.Viper.Get(server.FlagMinGasPrices))),
				baseapp.SetMinRetainBlocks(cast.ToUint64(serverCtx.Viper.Get(server.FlagMinRetainBlocks))),
				baseapp.SetInterBlockCache(cache),
			)
			app.Replay(a)
			return nil
		},
	}

	cmd.Flags().String(flags.FlagHome, defaultNodeHome, "The database home directory")
	cmd.Flags().String(flags.FlagChainID, "pax-chain", "chain ID")

	return cmd
}

func startReplayProfiler(ctx context.Context, port uint16) (*http.Server, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port))))
	if err != nil {
		return nil, fmt.Errorf("start replay profiling: %w", err)
	}
	profiler := &http.Server{
		Addr:              listener.Addr().String(),
		Handler:           http.DefaultServeMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	stop := context.AfterFunc(ctx, func() { _ = profiler.Close() })
	go func() {
		defer stop()
		if err := profiler.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Error from profiling server", "error", err)
		}
	}()
	logger.Info("Listening for profiling", "address", "http://"+profiler.Addr+"/debug/pprof/")
	return profiler, nil
}
