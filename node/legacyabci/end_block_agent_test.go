package legacyabci_test

import (
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	layerxagentkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/keeper"
	agenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/legacyabci"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
)

type endBlockAgentHarness struct {
	app     *app.App
	ctx     sdk.Context
	agent   layerxagentkeeper.Keeper
	keepers legacyabci.EndBlockKeepers
	jobID   uint64
	creator common.Address
	target  common.Address
}

func newEndBlockAgentHarness(t *testing.T) *endBlockAgentHarness {
	t.Helper()
	a := app.Setup(t, false, true, false)
	ctx, _ := a.NewContext(false, tmtypes.Header{
		ChainID: "pax-test", Height: 1000, Time: time.Unix(1_800_000_000, 0).UTC(),
	}).CacheContext()
	key := sdk.NewKVStoreKey(layerxagentkeeper.StoreKey)
	stores := map[storetypes.StoreKey]storetypes.CacheWrapper{}
	keys := map[string]storetypes.StoreKey{}
	ctx.MultiStore().SetKVStores(func(key storetypes.StoreKey, store storetypes.KVStore) storetypes.CacheWrap {
		stores[key] = store
		keys[key.Name()] = key
		return store.(storetypes.CacheWrap)
	})
	stores[key] = dbadapter.Store{DB: tmdb.NewMemDB()}
	keys[key.Name()] = key
	ctx = ctx.WithMultiStore(cachemulti.NewStore(tmdb.NewMemDB(), stores, keys, nil, nil, nil, 0))
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(1), app.V6120Upgrade)
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(900), app.MergeUpgrade)
	a.EvmKeeper.SetNextBaseFeePerGas(ctx, sdk.NewDec(10))
	agent := layerxagentkeeper.NewKeeper(key, &a.EvmKeeper, app.MergeUpgrade)
	require.NoError(t, agent.Active(ctx))
	creator := common.HexToAddress("0x1234")
	creatorAcc := sdk.AccAddress(creator.Bytes())
	a.EvmKeeper.SetAddressMapping(ctx, creatorAcc, creator)
	coins := sdk.NewCoins(sdk.NewCoin(sdk.MustGetBaseDenom(), sdk.NewInt(10_000)))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, "evm", coins))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "evm", creatorAcc, coins))
	target := common.HexToAddress("0x5678")
	a.EvmKeeper.SetCode(ctx, target, []byte{0x60, 0x00, 0x54, 0x60, 0x01, 0x01, 0x60, 0x00, 0x55, 0x00})
	jobID, err := agent.Schedule(ctx, creatorAcc, creator, target, nil, 1001, 100_000, big.NewInt(2_000_000))
	require.NoError(t, err)
	keepers := a.EndBlockKeepers
	keepers.V6120Live = nil
	keepers.AgentKeeper = &agent
	return &endBlockAgentHarness{
		app: a, ctx: ctx.WithBlockHeight(1001).WithEventManager(sdk.NewEventManager()),
		agent: agent, keepers: keepers, jobID: jobID, creator: creator, target: target,
	}
}

func TestEndBlockRunsAgentSchedulerWhenLive(t *testing.T) {
	h := newEndBlockAgentHarness(t)
	h.keepers.MergeLive = func() bool { return true }
	legacyabci.EndBlock(h.ctx, h.ctx.BlockHeight(), 0, h.keepers)
	_, found := h.agent.GetJob(h.ctx, h.jobID)
	require.False(t, found)
	require.Empty(t, h.agent.Pending(h.ctx, h.creator))
	require.Equal(t, common.HexToHash("0x1"), h.app.EvmKeeper.GetState(h.ctx, h.target, common.Hash{}))
	var executed []*agenttypes.EventJobExecuted
	for _, event := range h.ctx.EventManager().Events() {
		require.NotEqual(t, proto.MessageName(&agenttypes.EventJobFailed{}), event.Type)
		if event.Type == proto.MessageName(&agenttypes.EventJobExecuted{}) {
			parsed, err := sdk.ParseTypedEvent(abci.Event(event))
			require.NoError(t, err)
			executed = append(executed, parsed.(*agenttypes.EventJobExecuted))
		}
	}
	require.Len(t, executed, 1)
	require.Equal(t, h.jobID, executed[0].JobId)
	require.Equal(t, h.creator.Hex(), executed[0].Creator)
	require.Positive(t, executed[0].GasUsed)
	require.Zero(t, h.app.EvmKeeper.GetBalance(h.ctx, h.agent.ModuleAddress()).Sign())
}

func TestEndBlockSkipsAgentSchedulerBeforeMerge(t *testing.T) {
	for _, name := range []string{"before_merge", "nil_merge_live", "nil_agent_keeper"} {
		t.Run(name, func(t *testing.T) {
			h := newEndBlockAgentHarness(t)
			switch name {
			case "before_merge":
				h.keepers.MergeLive = func() bool { return false }
			case "nil_merge_live":
				h.keepers.MergeLive = nil
			case "nil_agent_keeper":
				h.keepers.AgentKeeper = nil
				h.keepers.MergeLive = func() bool { return true }
			}
			before, found := h.agent.GetJob(h.ctx, h.jobID)
			require.True(t, found)
			legacyabci.EndBlock(h.ctx, h.ctx.BlockHeight(), 0, h.keepers)
			after, found := h.agent.GetJob(h.ctx, h.jobID)
			require.True(t, found)
			require.Equal(t, before, after)
			require.Equal(t, []uint64{h.jobID}, h.agent.Pending(h.ctx, h.creator))
			require.Equal(t, common.Hash{}, h.app.EvmKeeper.GetState(h.ctx, h.target, common.Hash{}))
			require.Equal(t, big.NewInt(2_000_000), h.app.EvmKeeper.GetBalance(h.ctx, h.agent.ModuleAddress()))
			for _, event := range h.ctx.EventManager().Events() {
				require.NotEqual(t, proto.MessageName(&agenttypes.EventJobExecuted{}), event.Type)
				require.NotEqual(t, proto.MessageName(&agenttypes.EventJobFailed{}), event.Type)
			}
		})
	}
}
