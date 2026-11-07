// Proves: keeper.Keeper.CreateMarketFor/RecordFee/TriggerAirdrop/ClaimAirdropEpoch -> paxd subprocess and Tendermint HTTP ABCI -> cli.GetQueryCmd/types.QueryClient and decoded query responses.
//
// The daemon is built from daemon/paxd, with its production root command. Real
// keeper and EVM operations produce the state; application Commit, Close and
// reopen precede ExportGenesis and the query node's InitGenesis. No query,
// RPC client, server or keeper is replaced. The subprocess output is consumed
// through the application's protobuf JSON codec.
//
// The merged paxd root registers these launchpad queries. Its generated help
// documents the same command that sends the subprocess ABCI requests below.
package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/testutil/network"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	servertypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/types"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdknetwork "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestProofModuleCli(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	require.NoError(t, err)
	binary := proofModuleCliBinary(t)
	// Building the shipping executable is part of this subprocess proof, not
	// a second package gate or a generated replacement command implementation.
	build := exec.CommandContext(t.Context(), "go", "build", "-o", binary, "./daemon/paxd")
	build.Dir = root
	output, err := build.CombinedOutput()
	require.NoError(t, err, "build daemon: %s", output)

	genesis, market, epoch := proofModuleCliState(t)
	cfg, net := proofModuleCliNetwork(t, genesis)
	height, err := net.WaitForHeight(1)
	require.NoError(t, err)
	require.Len(t, net.Validators, 1)
	node := net.Validators[0]
	home := t.TempDir()
	run := func(args ...string) (string, string, error) {
		t.Helper()
		argv := append([]string{"query", "launchpad"}, args...)
		argv = append(argv, "--home", home, "--node", node.RPCAddress,
			"--height", strconv.FormatInt(height, 10), "--output", "json")
		command := exec.CommandContext(t.Context(), binary, argv...)
		var stdout, stderr bytes.Buffer
		command.Stdout, command.Stderr = &stdout, &stderr
		err := command.Run()
		return stdout.String(), stderr.String(), err
	}
	decode := func(response codec.ProtoMarshaler, args ...string) {
		t.Helper()
		stdout, stderr, err := run(args...)
		require.NoError(t, err, "%v: %s", args, stderr)
		require.NoError(t, cfg.Codec.UnmarshalAsJSON([]byte(stdout), response), "%v: %s", args, stdout)
	}
	pool := market.PoolAddress
	claimedHolder := genesis.FeeAccounting.Claims[0].Holder
	info := types.MarketInfoOf(market)
	require.NotEmpty(t, genesis.FeeAccounting.Pools)
	require.NotEmpty(t, genesis.FeeAccounting.Epochs)
	require.NotEmpty(t, genesis.FeeAccounting.Claims)
	wantAccounting := types.FeeAccountingInfoOf(genesis.FeeAccounting)
	for pass := 0; pass < 2; pass++ {
		// Each pass creates fresh OS processes and freshly decodes their stdout.
		t.Run("read-"+strconv.Itoa(pass), func(t *testing.T) {
			var byDenom types.QueryMarketResponse
			decode(&byDenom, "market", market.Denom)
			require.Equal(t, info, byDenom.Market)
			for _, address := range []string{pool, market.TokenAddress} {
				var byAddress types.QueryMarketByAddressResponse
				decode(&byAddress, "market-by-address", address)
				require.Equal(t, info, byAddress.Market)
			}
			var markets types.QueryMarketsByCreatorResponse
			decode(&markets, "markets-by-creator", market.Creator, "--offset", "1", "--limit", "1")
			require.Equal(t, []types.MarketInfo{info}, markets.Markets)
			decode(&markets, "markets-by-creator", market.Creator, "--offset", "2", "--limit", "1")
			require.Empty(t, markets.Markets)
			var right types.QueryFeeRightResponse
			decode(&right, "fee-right", market.FeeRightCollection, strconv.FormatUint(market.FeeRightID, 10))
			require.True(t, right.Found)
			require.Equal(t, &types.FeeRightInfo{ID: market.FeeRightID, Owner: genesis.FeeRights.Tokens[1].Owner,
				Strategy: uint32(market.FeeStrategy), Pool: pool}, right.FeeRight)
			var role types.QueryHasRoleResponse
			decode(&role, "has-role", market.FeeAccumulator, common.Hash(keeper.FeesRouterRole).Hex(),
				keeper.Generation2Surfaces[types.RoleFeesRouter].Hex())
			require.True(t, role.HasRole)
			decode(&role, "has-role", market.FeeAccumulator, common.Hash(keeper.FeesRouterRole).Hex(), claimedHolder)
			require.False(t, role.HasRole)
			var accounting types.QueryFeeAccountingResponse
			decode(&accounting, "fee-accounting")
			require.Equal(t, wantAccounting, accounting.Accounting)
			var gotEpoch types.QueryAirdropEpochResponse
			decode(&gotEpoch, "airdrop-epoch", pool, "1")
			require.True(t, gotEpoch.Found)
			epochInfo := types.AirdropEpochInfoOf(epoch)
			require.Equal(t, &epochInfo, gotEpoch.AirdropEpoch)
			var claimed types.QueryAirdropClaimedResponse
			decode(&claimed, "airdrop-claimed", pool, claimedHolder, "1")
			require.True(t, claimed.Claimed)
			decode(&claimed, "airdrop-claimed", pool, claimedHolder, "2")
			require.False(t, claimed.Claimed)
		})
	}
	for _, tc := range []struct {
		name    string
		args    []string
		message string
	}{
		{"unknown-market", []string{"market", "launchpad/unknown"}, types.ErrUnknownMarket.Error()},
		{"malformed-address", []string{"market-by-address", "bad-address"}, "address"},
		{"malformed-epoch", []string{"airdrop-epoch", pool, "bad"}, "epoch"},
		{"malformed-claim-epoch", []string{"airdrop-claimed", pool, claimedHolder, "bad"}, "epoch"},
		{"malformed-id", []string{"fee-right", market.FeeRightCollection, "bad"}, "id"},
		{"malformed-role", []string{"has-role", market.FeeAccumulator, "0x01", claimedHolder}, "role"},
		{"extra-accounting-arg", []string{"fee-accounting", "extra"}, "accepts 0 arg(s)"},
		{"invalid-page", []string{"markets-by-creator", market.Creator, "--limit", "101"}, types.ErrInvalidRange.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, err := run(tc.args...)
			var exit *exec.ExitError
			require.ErrorAs(t, err, &exit)
			require.Equal(t, 1, exit.ExitCode())
			require.Empty(t, stdout)
			require.Contains(t, stderr, tc.message)
		})
	}
	for _, name := range []string{"market", "market-by-address", "markets-by-creator", "fee-right", "has-role",
		"fee-accounting", "airdrop-epoch", "airdrop-claimed"} {
		stdout, stderr, err := run(name, "--help")
		require.NoError(t, err, stderr)
		require.NotContains(t, stdout, "layerxd")
		require.Contains(t, stdout, "Examples:\npaxd query launchpad "+name)
		require.Contains(t, stdout, "paxd query launchpad "+name)
	}
}

func proofModuleCliBinary(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "paxd")
}

func proofModuleCliNetwork(t *testing.T, genesis *types.GenesisState) (network.Config, *network.Network) {
	t.Helper()
	cfg := network.DefaultConfig()
	cfg.SigningAlgo = string(hd.Secp256k1Type)
	data, err := json.Marshal(genesis)
	cfg.GenesisState[types.ModuleName] = data
	require.NoError(t, err)
	// Use the production app with the network's own storage options, as the
	// existing CLI network tests do; query requests cross the real HTTP RPC.
	cfg.AppConstructor = func(val sdknetwork.Validator) servertypes.Application {
		opts := viper.New()
		sc, ss := val.AppConfig.StateCommit, val.AppConfig.StateStore
		for name, value := range map[string]interface{}{
			"chain-id": cfg.ChainID, "evm.http_enabled": false, "evm.ws_enabled": false,
			app.FlagSCEnable:                    sc.Enable,
			app.FlagSCAsyncCommitBuffer:         sc.MemIAVLConfig.AsyncCommitBuffer,
			app.FlagSCSnapshotKeepRecent:        sc.MemIAVLConfig.SnapshotKeepRecent,
			app.FlagSCSnapshotInterval:          sc.MemIAVLConfig.SnapshotInterval,
			app.FlagSCSnapshotMinTimeInterval:   sc.MemIAVLConfig.SnapshotMinTimeInterval,
			app.FlagSCSnapshotWriterLimit:       sc.MemIAVLConfig.SnapshotWriterLimit,
			app.FlagSCSnapshotPrefetchThreshold: sc.MemIAVLConfig.SnapshotPrefetchThreshold,
			app.FlagSCSnapshotWriteRateMBps:     sc.MemIAVLConfig.SnapshotWriteRateMBps,
			app.FlagSSEnable:                    ss.Enable, app.FlagSSBackend: ss.Backend,
			app.FlagSSAsyncWriterBuffer: ss.AsyncWriteBuffer,
			app.FlagSSKeepRecent:        ss.KeepRecent, app.FlagSSPruneInterval: ss.PruneIntervalSeconds,
			app.FlagSSImportNumWorkers: ss.ImportNumWorkers,
		} {
			opts.Set(name, value)
		}
		return app.New(dbm.NewMemDB(), nil, true, map[int64]bool{}, val.Ctx.Config.RootDir,
			0, false, val.Ctx.Config, app.MakeEncodingConfig(), wasm.EnableAllProposals,
			opts, nil, app.EmptyAppOptions,
			baseapp.SetPruning(storetypes.NewPruningOptionsFromString(val.AppConfig.Pruning)),
			baseapp.SetMinGasPrices(val.AppConfig.MinGasPrices))
	}
	net := network.New(t, cfg)
	return cfg, net
}

func proofModuleCliState(t *testing.T) (*types.GenesisState, types.Market, types.PoolAirdropEpoch) {
	t.Helper()
	home, dir := t.TempDir(), t.TempDir()
	open := func() *app.App {
		database, err := dbm.NewGoLevelDB("application", dir)
		require.NoError(t, err)
		return app.New(database, nil, true, map[int64]bool{}, home, 1, true, nil, app.MakeEncodingConfig(),
			wasm.EnableAllProposals, app.TestAppOpts{}, app.EmptyWasmOpts, nil)
	}
	a := open()
	t.Cleanup(func() {
		if a != nil {
			require.NoError(t, a.Close())
		}
	})
	start := time.Unix(1_800_000_000, 0).UTC()
	genesis, err := json.Marshal(app.NewDefaultGenesisState(app.MakeEncodingConfig().Marshaler))
	require.NoError(t, err)
	_, err = a.InitChain(t.Context(), &abci.RequestInitChain{Time: start, ConsensusParams: app.DefaultConsensusParams,
		ChainId: "pax-test", AppStateBytes: genesis})
	require.NoError(t, err)
	block := func(body func(sdk.Context)) {
		height := a.LastBlockHeight() + 1
		_, err := a.FinalizeBlock(t.Context(), &abci.RequestFinalizeBlock{
			Header: &tmproto.Header{ChainID: "pax-test", Height: height}})
		require.NoError(t, err)
		ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(height).WithBlockTime(start.Add(time.Duration(height) * time.Minute))
		body(ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx)).WithEventManager(sdk.NewEventManager()))
		a.SetDeliverStateToCommit()
		_, err = a.Commit(context.Background())
		require.NoError(t, err)
	}
	creator := common.HexToAddress("0x0000000000000000000000000000000000001234")
	holder := common.HexToAddress("0x0000000000000000000000000000000000005678")
	other := common.HexToAddress("0x0000000000000000000000000000000000009abc")
	send := func(ctx sdk.Context, token, from common.Address, signature string, words ...[]byte) {
		input := append([]byte(nil), crypto.Keccak256([]byte(signature))[:4]...)
		for _, word := range words {
			input = append(input, word...)
		}
		result, err := a.EvmKeeper.CallEVM(ctx, from, &token, nil, input)
		require.NoError(t, err)
		require.Equal(t, common.LeftPadBytes([]byte{1}, 32), result)
	}
	word := func(value int64) []byte { return common.LeftPadBytes(big.NewInt(value).Bytes(), 32) }
	var market, quote types.Market
	block(func(ctx sdk.Context) {
		a.UpgradeKeeper.SetDone(ctx, app.V6120Upgrade)
		a.RefreshPrecompiles(ctx)
		k := a.LaunchpadKeeper
		var err error
		previewCtx, _ := ctx.CacheContext()
		_, preview, err := k.CreateMarketFor(previewCtx, k.ModuleCaller(), nil, creator, "CLI Quote", "CLIQ", types.FeeStrategyClaim, common.Address{})
		require.NoError(t, err)
		require.Zero(t, k.GetMarketCount(ctx))
		require.Zero(t, k.FactoryNonce(ctx, creator))
		require.Empty(t, a.EvmKeeper.GetCode(ctx, common.HexToAddress(preview.TokenAddress)))
		params := k.GetParams(ctx)
		params.QuoteToken = preview.TokenAddress
		require.NoError(t, k.UpdateParams(ctx, k.Authority(), params))
		ctx, quote, err = k.CreateMarketFor(ctx, k.ModuleCaller(), nil, creator, "CLI Quote", "CLIQ", types.FeeStrategyClaim, common.Address{})
		require.NoError(t, err)
		require.Equal(t, params.QuoteToken, quote.TokenAddress)
		ctx, market, err = k.CreateMarketFor(ctx, k.ModuleCaller(), nil, creator, "CLI Airdrop", "CLIA", types.FeeStrategyAirdrop, common.Address{})
		require.NoError(t, err)
		pool, token := common.HexToAddress(market.PoolAddress), common.HexToAddress(market.TokenAddress)
		send(ctx, token, pool, "transfer(address,uint256)", common.LeftPadBytes(holder.Bytes(), 32), word(200))
		send(ctx, token, pool, "transfer(address,uint256)", common.LeftPadBytes(other.Bytes(), 32), word(100))
		roles := k.Generation2()
		granted, err := k.GovernanceGrant(ctx, k.Authority(), roles.FeeAccumulator, keeper.FeesRouterRole, roles.FeesRouter)
		require.NoError(t, err)
		require.True(t, granted)
		quoteToken, quotePool := common.HexToAddress(quote.TokenAddress), common.HexToAddress(quote.PoolAddress)
		send(ctx, quoteToken, quotePool, "approve(address,uint256)", common.LeftPadBytes(common.HexToAddress(types.LaunchpadAddress).Bytes(), 32), word(100_000))
		ctx, amount, err := k.PullQuote(ctx, k.ModuleCaller(), quotePool, keeper.FeeHeldLedger(roles.FeeAccumulator), sdk.NewInt(100_000))
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(100_000), amount)
		_, protocol, poolCut, err := k.RecordFee(ctx, k.ModuleCaller(), market.Denom, amount)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(10_000), protocol)
		require.Equal(t, sdk.NewInt(90_000), poolCut)
	})
	var epoch types.PoolAirdropEpoch
	var before []byte
	block(func(ctx sdk.Context) {
		k := a.LaunchpadKeeper
		pool := common.HexToAddress(market.PoolAddress)
		amount, err := k.TriggerAirdrop(ctx, k.ModuleCaller(), k.Generation2().FeesRouter, pool)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(90_000), amount)
		ctx, paid, err := k.ClaimAirdropEpoch(ctx, k.ModuleCaller(), pool, holder, 1)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(60_000), paid)
		var found bool
		market, found = k.GetMarket(ctx, market.Denom)
		require.True(t, found)
		epoch, found, err = k.GetPoolAirdropEpoch(ctx, pool, 1)
		require.NoError(t, err)
		require.True(t, found)
		before, err = json.Marshal(k.ExportGenesis(ctx))
		require.NoError(t, err)
	})
	height := a.LastBlockHeight()
	require.NoError(t, a.Close())
	a = nil
	a = open()
	require.Equal(t, height, a.LastBlockHeight())
	var exported *types.GenesisState
	block(func(ctx sdk.Context) {
		k := a.LaunchpadKeeper
		exported = k.ExportGenesis(ctx)
		after, err := json.Marshal(exported)
		require.NoError(t, err)
		require.JSONEq(t, string(before), string(after), "reopen must preserve all produced state")
		pool := common.HexToAddress(market.PoolAddress)
		_, _, err = k.ClaimAirdropEpoch(ctx, k.ModuleCaller(), pool, holder, 1)
		require.ErrorIs(t, err, types.ErrAlreadyClaimed)
		_, err = k.MintFeeRight(ctx, k.Generation2().SidioraNFT, creator, pool, market.FeeStrategy)
		require.ErrorIs(t, err, types.ErrDuplicatePool)
		granted, err := k.GovernanceGrant(ctx, k.Authority(), k.Generation2().FeeAccumulator, keeper.FeesRouterRole, k.Generation2().FeesRouter)
		require.NoError(t, err)
		require.False(t, granted, "repeated grant is idempotent")
		after, err = json.Marshal(k.ExportGenesis(ctx))
		require.NoError(t, err)
		require.JSONEq(t, string(before), string(after), "duplicate producers must leave state unchanged")
		require.NoError(t, exported.Validate())
	})
	require.NoError(t, a.Close())
	a = nil
	return exported, market, epoch
}
