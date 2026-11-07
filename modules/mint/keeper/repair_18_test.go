package keeper_test

import (
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	epochkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/epoch/keeper"
	epochtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/epoch/types"
	mintkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/keeper"
	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
)

type repair18MintNode struct {
	stores storetypes.CommitMultiStore
	ctx    sdk.Context
	mint   mintkeeper.Keeper
	epoch  *epochkeeper.Keeper
	bank   bankkeeper.BaseKeeper
	closed bool
}

func (n *repair18MintNode) close(t *testing.T) {
	t.Helper()
	if !n.closed {
		require.NoError(t, n.stores.Close())
		n.closed = true
	}
}

func openRepair18MintNode(t *testing.T, directory string) *repair18MintNode {
	t.Helper()
	db, err := tmdb.NewGoLevelDB("mint-repair-18", directory)
	require.NoError(t, err)
	n := &repair18MintNode{stores: store.NewCommitMultiStore(db)}
	t.Cleanup(func() { n.close(t) })
	keys := sdk.NewKVStoreKeys(minttypes.StoreKey, epochtypes.StoreKey,
		authtypes.StoreKey, banktypes.StoreKey, paramtypes.ModuleName)
	for _, key := range keys {
		n.stores.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	transientKey := sdk.NewTransientStoreKey("transient_params")
	memKey := storetypes.NewMemoryStoreKey(epochtypes.MemStoreKey)
	n.stores.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, nil)
	n.stores.MountStoreWithDB(memKey, sdk.StoreTypeMemory, nil)
	require.NoError(t, n.stores.LoadLatestVersion())
	n.ctx = sdk.NewContext(n.stores, tmproto.Header{Height: n.stores.LastCommitID().Version + 1}, false)
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	params := paramskeeper.NewKeeper(cdc, codec.NewLegacyAmino(), keys[paramtypes.ModuleName], transientKey)
	accounts := authkeeper.NewAccountKeeper(cdc, keys[authtypes.StoreKey], params.Subspace(authtypes.ModuleName),
		authtypes.ProtoBaseAccount, map[string][]string{
			minttypes.ModuleName:       {authtypes.Minter},
			authtypes.FeeCollectorName: nil,
		})
	n.bank = bankkeeper.NewBaseKeeper(cdc, keys[banktypes.StoreKey], accounts,
		params.Subspace(banktypes.ModuleName), map[string]bool{})
	n.epoch = epochkeeper.NewKeeper(cdc, keys[epochtypes.StoreKey], memKey, params.Subspace(epochtypes.ModuleName))
	n.mint = mintkeeper.NewKeeper(cdc, keys[minttypes.StoreKey], params.Subspace(minttypes.ModuleName),
		nil, accounts, n.bank, n.epoch, authtypes.FeeCollectorName)
	n.epoch.SetHooks(epochtypes.NewMultiEpochHooks(n.mint.Hooks()))
	if n.stores.LastCommitID().Version == 0 {
		accounts.SetParams(n.ctx, authtypes.DefaultParams())
		n.bank.SetParams(n.ctx, banktypes.DefaultParams())
		n.epoch.SetParams(n.ctx, epochtypes.DefaultParams())
		n.mint.SetMinter(n.ctx, minttypes.DefaultInitialMinter())
	}
	return n
}

func (n *repair18MintNode) nextEpoch(t *testing.T) {
	t.Helper()
	epoch := n.epoch.GetEpoch(n.ctx)
	n.ctx = n.ctx.WithBlockHeight(n.ctx.BlockHeight() + 1).
		WithBlockTime(epoch.CurrentEpochStartTime.Add(epoch.EpochDuration + time.Second)).
		WithEventManager(sdk.NewEventManager())
	n.epoch.BeginBlock(n.ctx)
	require.Equal(t, epoch.CurrentEpoch+1, n.epoch.GetEpoch(n.ctx).CurrentEpoch)
}

func TestRepair_18_ReleaseSuccessorAfterRestart(t *testing.T) {
	for _, tc := range []struct {
		name      string
		firstEnd  string
		nextStart string
		nextEnd   string
	}{
		{"one_day_then_next_day", "2026-01-01", "2026-01-02", "2026-01-02"},
		{"successor_shares_start_date", "2026-01-01", "2026-01-01", "2026-01-02"},
		{"successor_shares_end_boundary", "2026-01-02", "2026-01-02", "2026-01-03"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			n := openRepair18MintNode(t, directory)
			start := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
			params := minttypes.NewParams(sdk.DefaultBondDenom, []minttypes.ScheduledTokenRelease{
				{StartDate: "2026-01-01", EndDate: tc.firstEnd, TokenReleaseAmount: 100},
				{StartDate: tc.nextStart, EndDate: tc.nextEnd, TokenReleaseAmount: 200},
			})
			require.NoError(t, params.Validate())
			for _, release := range params.TokenReleaseSchedule {
				require.NoError(t, minttypes.ValidateMinter(minttypes.NewMinter(release.StartDate,
					release.EndDate, params.MintDenom, release.TokenReleaseAmount)))
			}
			n.mint.SetParams(n.ctx, params)
			n.epoch.SetEpoch(n.ctx, epochtypes.Epoch{GenesisTime: start, EpochDuration: 24 * time.Hour,
				CurrentEpochStartTime: start, CurrentEpochHeight: n.ctx.BlockHeight()})
			holder := sdk.AccAddress([]byte("repair18-holder-addr"))
			existing := sdk.NewCoins(sdk.NewInt64Coin(params.MintDenom, 37))
			require.NoError(t, n.bank.MintCoins(n.ctx, minttypes.ModuleName, existing))
			require.NoError(t, n.bank.SendCoinsFromModuleToAccount(n.ctx, minttypes.ModuleName, holder, existing))
			assertBalances := func(n *repair18MintNode, released int64) {
				t.Helper()
				require.Equal(t, sdk.NewInt64Coin(params.MintDenom, 37+released), n.bank.GetSupply(n.ctx, params.MintDenom))
				require.Equal(t, existing[0], n.bank.GetBalance(n.ctx, holder, params.MintDenom))
				require.Equal(t, sdk.NewInt64Coin(params.MintDenom, released),
					n.bank.GetBalance(n.ctx, authtypes.NewModuleAddress(authtypes.FeeCollectorName), params.MintDenom))
				require.True(t, n.bank.GetBalance(n.ctx, authtypes.NewModuleAddress(minttypes.ModuleName), params.MintDenom).IsZero())
			}

			n.nextEpoch(t)
			first := n.mint.GetMinter(n.ctx)
			require.Equal(t, "2026-01-01", first.StartDate)
			require.Equal(t, tc.firstEnd, first.EndDate)
			require.Zero(t, first.RemainingMintAmount)
			require.Equal(t, uint64(100), first.LastMintAmount)
			require.NoError(t, minttypes.ValidateMinter(first))
			assertBalances(n, 100)
			epoch := n.epoch.GetEpoch(n.ctx)
			commit := n.stores.Commit(true)
			n.close(t)
			n = openRepair18MintNode(t, directory)
			require.Equal(t, commit, n.stores.LastCommitID())
			require.Equal(t, first, n.mint.GetMinter(n.ctx))
			require.Equal(t, params, n.mint.GetParams(n.ctx))
			require.Equal(t, epoch, n.epoch.GetEpoch(n.ctx))
			assertBalances(n, 100)

			n.nextEpoch(t)
			second := n.mint.GetMinter(n.ctx)
			require.Equal(t, tc.nextStart, second.StartDate)
			require.Equal(t, tc.nextEnd, second.EndDate)
			require.Equal(t, uint64(200), second.TotalMintAmount)
			require.Equal(t, uint64(200), second.LastMintAmount)
			require.Equal(t, "2026-01-02", second.LastMintDate)
			require.Zero(t, second.RemainingMintAmount)
			require.NoError(t, minttypes.ValidateMinter(second))
			assertBalances(n, 300)
			var mintEvents sdk.Events
			for _, event := range n.ctx.EventManager().Events() {
				if event.Type == minttypes.EventTypeMint {
					mintEvents = append(mintEvents, event)
				}
			}
			require.Len(t, mintEvents, 1)
			require.Contains(t, mintEvents[0].Attributes, sdk.NewAttribute(sdk.AttributeKeyAmount, "200").ToKVPair())
			commit = n.stores.Commit(true)
			n.close(t)
			n = openRepair18MintNode(t, directory)
			require.Equal(t, commit, n.stores.LastCommitID())
			require.Equal(t, second, n.mint.GetMinter(n.ctx))
			assertBalances(n, 300)
			for i := 0; i < 3; i++ {
				n.nextEpoch(t)
				require.Equal(t, second, n.mint.GetMinter(n.ctx))
				assertBalances(n, 300)
				for _, event := range n.ctx.EventManager().Events() {
					require.NotEqual(t, minttypes.EventTypeMint, event.Type)
				}
			}
		})
	}
}
