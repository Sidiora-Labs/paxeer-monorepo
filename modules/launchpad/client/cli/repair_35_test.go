package cli_test

import (
	"encoding/json"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	rpcclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client"
	evm "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	servertypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/types"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	clitestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func repair35State(t *testing.T) (*types.GenesisState, *evmtypes.GenesisState, types.Market) {
	t.Helper()
	a := app.New(tmdb.NewMemDB(), nil, true, map[int64]bool{}, t.TempDir(), 1, true, nil, app.MakeEncodingConfig(),
		wasm.EnableAllProposals, app.TestAppOpts{}, app.EmptyWasmOpts, nil)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	genesis, err := json.Marshal(app.NewDefaultGenesisState(app.MakeEncodingConfig().Marshaler))
	require.NoError(t, err)
	_, err = a.InitChain(t.Context(), &abci.RequestInitChain{ChainId: "pax-test", Time: time.Now().UTC(), AppStateBytes: genesis, ConsensusParams: app.DefaultConsensusParams})
	require.NoError(t, err)
	ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(1)
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	a.UpgradeKeeper.SetDone(ctx, app.V6120Upgrade)
	a.RefreshPrecompiles(ctx)
	k := a.LaunchpadKeeper
	params := k.GetParams(ctx)
	reserve, ok := sdk.NewIntFromString("1000000000000000000000000")
	require.True(t, ok)
	params.VirtualQuoteDefault, params.VirtualTokenDefault = reserve, reserve
	require.NoError(t, k.UpdateParams(ctx, k.Authority(), params))
	creator := common.HexToAddress("0x0000000000000000000000000000000000001234")
	previewCtx, _ := ctx.CacheContext()
	_, preview, err := k.CreateMarketFor(previewCtx, k.ModuleCaller(), nil, creator, "Repair Quote", "RQT", types.FeeStrategyClaim, common.Address{})
	require.NoError(t, err)
	require.Zero(t, k.GetMarketCount(ctx))
	require.Zero(t, k.FactoryNonce(ctx, creator))
	require.Empty(t, a.EvmKeeper.GetCode(ctx, common.HexToAddress(preview.TokenAddress)))
	params.QuoteToken = preview.TokenAddress
	require.NoError(t, k.UpdateParams(ctx, k.Authority(), params))
	ctx, quote, err := k.CreateMarketFor(ctx, k.ModuleCaller(), nil, creator, "Repair Quote", "RQT", types.FeeStrategyClaim, common.Address{})
	require.NoError(t, err)
	require.Equal(t, params.QuoteToken, quote.TokenAddress)
	ctx, market, err := k.CreateMarketFor(ctx, k.ModuleCaller(), nil, creator, "Repair Market", "RMT", types.FeeStrategyClaim, common.Address{})
	require.NoError(t, err)
	pool := common.HexToAddress(market.PoolAddress)
	quoteToken := common.HexToAddress(quote.TokenAddress)
	funding := reserve.QuoRaw(100)
	input := append([]byte(nil), crypto.Keccak256([]byte("transfer(address,uint256)"))[:4]...)
	input = append(input, common.LeftPadBytes(pool.Bytes(), 32)...)
	input = append(input, common.LeftPadBytes(funding.BigInt().Bytes(), 32)...)
	result, err := a.EvmKeeper.CallEVM(ctx, common.HexToAddress(quote.PoolAddress), &quoteToken, nil, input)
	require.NoError(t, err)
	require.Equal(t, common.LeftPadBytes([]byte{1}, 32), result)
	ctx, bought, err := k.Swap(ctx, k.ModuleCaller(), state.NewDBImpl(ctx, &a.EvmKeeper, false), market.Denom, true,
		funding.BigInt(), big.NewInt(0), creator, creator, ^uint64(0))
	require.NoError(t, err)
	require.Positive(t, bought.AmountOut.Sign())
	var found bool
	market, found = k.GetMarket(ctx, market.Denom)
	require.True(t, found)
	require.True(t, market.RealQuoteBalance.IsPositive())
	launchpadGenesis := k.ExportGenesis(ctx)
	require.NoError(t, launchpadGenesis.Validate())
	return launchpadGenesis, evm.ExportGenesis(ctx, &a.EvmKeeper), market
}

func TestRepair_35_QuoteUint256(t *testing.T) {
	cfg := network.DefaultConfig(t)
	launchpadGenesis, evmGenesis, market := repair35State(t)
	var err error
	cfg.GenesisState[types.ModuleName], err = json.Marshal(launchpadGenesis)
	require.NoError(t, err)
	cfg.GenesisState[evmtypes.ModuleName], err = cfg.Codec.MarshalAsJSON(evmGenesis)
	require.NoError(t, err)
	cfg.AppConstructor = func(val network.Validator) servertypes.Application {
		opts := viper.New()
		sc, ss := val.AppConfig.StateCommit, val.AppConfig.StateStore
		for name, value := range map[string]interface{}{
			"chain-id":                          cfg.ChainID,
			"evm.http_enabled":                  false,
			"evm.ws_enabled":                    false,
			app.FlagSCEnable:                    sc.Enable,
			app.FlagSCAsyncCommitBuffer:         sc.MemIAVLConfig.AsyncCommitBuffer,
			app.FlagSCSnapshotKeepRecent:        sc.MemIAVLConfig.SnapshotKeepRecent,
			app.FlagSCSnapshotInterval:          sc.MemIAVLConfig.SnapshotInterval,
			app.FlagSCSnapshotMinTimeInterval:   sc.MemIAVLConfig.SnapshotMinTimeInterval,
			app.FlagSCSnapshotWriterLimit:       sc.MemIAVLConfig.SnapshotWriterLimit,
			app.FlagSCSnapshotPrefetchThreshold: sc.MemIAVLConfig.SnapshotPrefetchThreshold,
			app.FlagSCSnapshotWriteRateMBps:     sc.MemIAVLConfig.SnapshotWriteRateMBps,
			app.FlagSSEnable:                    ss.Enable,
			app.FlagSSBackend:                   ss.Backend,
			app.FlagSSAsyncWriterBuffer:         ss.AsyncWriteBuffer,
			app.FlagSSKeepRecent:                ss.KeepRecent,
			app.FlagSSPruneInterval:             ss.PruneIntervalSeconds,
			app.FlagSSImportNumWorkers:          ss.ImportNumWorkers,
		} {
			opts.Set(name, value)
		}
		encoding := app.MakeEncodingConfig()
		application := app.New(tmdb.NewMemDB(), nil, true, map[int64]bool{}, val.Ctx.Config.RootDir,
			0, false, val.Ctx.Config, encoding, wasm.EnableAllProposals, opts, nil, app.EmptyAppOptions,
			baseapp.SetPruning(storetypes.NewPruningOptionsFromString(val.AppConfig.Pruning)),
			baseapp.SetMinGasPrices(val.AppConfig.MinGasPrices))
		return application
	}
	net := network.New(t, cfg)
	t.Cleanup(net.Cleanup)
	height, err := net.WaitForHeight(1)
	require.NoError(t, err)
	require.NotEmpty(t, market.Denom)
	validator := net.Validators[0]
	clientCtx := validator.ClientCtx.WithHeight(height)
	service := types.NewQueryClient(clientCtx)
	before, err := service.Market(t.Context(), &types.QueryMarketRequest{Denom: market.Denom})
	require.NoError(t, err)
	require.Equal(t, types.MarketInfoOf(market), before.Market)
	held, err := service.QuoteLedger(t.Context(), &types.QueryQuoteLedgerRequest{Kind: "held", Address: market.PoolAddress})
	require.NoError(t, err)
	require.Equal(t, market.RealQuoteBalance.String(), held.Amount)
	for _, side := range []string{"buy", "sell"} {
		for _, amount := range []string{"18446744073709551616", "00018446744073709551616", strings.Repeat("0", 301) + "18446744073709551616"} {
			t.Run(side+"/digits-"+strconv.Itoa(len(amount)), func(t *testing.T) {
				want, err := service.Quote(t.Context(), &types.QueryQuoteRequest{Denom: market.Denom, IsBuy: side == "buy", AmountIn: amount})
				require.NoError(t, err)
				output, err := clitestutil.ExecTestCLICmd(clientCtx, cli.CmdQueryQuote(), []string{market.Denom, side, amount, "--output=json", "--height=" + strconv.FormatInt(height, 10)})
				require.NoError(t, err)
				var got types.QueryQuoteResponse
				require.NoError(t, cfg.Codec.UnmarshalAsJSON(output.Bytes(), &got))
				require.Equal(t, want, &got)
				require.Equal(t, "18446744073709551616", got.Quote.AmountIn.String())
				require.True(t, got.Quote.AmountOut.IsPositive())
				var jsonQuote struct {
					Quote struct {
						AmountIn string `json:"amount_in"`
					} `json:"quote"`
				}
				require.NoError(t, json.Unmarshal(output.Bytes(), &jsonQuote))
				require.Equal(t, "18446744073709551616", jsonQuote.Quote.AmountIn)
			})
		}
		t.Run(side+"/max-uint256-validation", func(t *testing.T) {
			amount := types.MaxUint256().String()
			_, serviceErr := service.Quote(t.Context(), &types.QueryQuoteRequest{Denom: "unknown-market", IsBuy: side == "buy", AmountIn: amount})
			_, commandErr := clitestutil.ExecTestCLICmd(clientCtx, cli.CmdQueryQuote(), []string{"unknown-market", side, amount, "--output=json"})
			require.ErrorContains(t, serviceErr, types.ErrUnknownMarket.Error())
			require.EqualError(t, commandErr, serviceErr.Error())
		})
		for _, amount := range []string{"", "0", "000", "abc", "-1", "+1", "1.5", "1e3", " 1", "1 ", "0x10", "0b10", "0o10", "1_000", "１２", new(big.Int).Add(types.MaxUint256(), big.NewInt(1)).String(), strings.Repeat("9", 301)} {
			t.Run(side+"/refuse-"+amount, func(t *testing.T) {
				_, serviceErr := service.Quote(t.Context(), &types.QueryQuoteRequest{Denom: market.Denom, IsBuy: side == "buy", AmountIn: amount})
				require.Equal(t, codes.InvalidArgument, status.Code(serviceErr))
				require.Equal(t, "invalid request", status.Convert(serviceErr).Message())
				for _, ctx := range []client.Context{clientCtx, {}} {
					output, err := clitestutil.ExecTestCLICmd(ctx, cli.CmdQueryQuote(), []string{"--output=json", "--", market.Denom, side, amount})
					require.ErrorContains(t, err, "amount-in must be a positive base-10 uint256 integer")
					var numberErr *strconv.NumError
					require.ErrorAs(t, err, &numberErr)
					require.Equal(t, amount, numberErr.Num)
					require.NotContains(t, output.String(), `"quote"`)
				}
				request, err := cfg.Codec.Marshal(&types.QueryQuoteRequest{Denom: market.Denom, IsBuy: side == "buy", AmountIn: amount})
				require.NoError(t, err)
				response, err := validator.RPCClient.ABCIQueryWithOptions(t.Context(), "/paxprotocol.paxchain.launchpad.Query/Quote", request, rpcclient.ABCIQueryOptions{Height: height})
				require.NoError(t, err)
				require.Equal(t, uint32(18), response.Response.Code)
				require.Equal(t, "sdk", response.Response.Codespace)
				require.Equal(t, "invalid request", response.Response.Log)
				require.Equal(t, response.Response.Log, status.Convert(serviceErr).Message())
			})
		}
	}
	after, err := service.Market(t.Context(), &types.QueryMarketRequest{Denom: market.Denom})
	require.NoError(t, err)
	require.Equal(t, before, after)
}
