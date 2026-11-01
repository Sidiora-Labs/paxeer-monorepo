package keeper_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	xwebtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func newProgramGateSuite(t *testing.T) *programSuite {
	t.Helper()
	testApp := app.Setup(t, false, false, false)
	ctx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(8)
	key := sdk.NewKVStoreKey(types.StoreKey)
	k := keeper.NewKeeper(key, testApp.BankKeeper, &testApp.EvmKeeper)
	ctx = xwebtestutil.WithXWebStore(ctx, key)
	genesis := types.DefaultGenesis()
	genesis.Params.KernelNetworkId = 1
	genesis.Params.ProgramFeeDenoms = []string{types.ProgramFeeDenomPAX}
	k.InitGenesis(ctx, *genesis)
	require.False(t, k.ProgramOriginActive(ctx))
	return &programSuite{suite: &suite{t: t, app: testApp, ctx: ctx, k: k}, key: key}
}

func programGatePendingState(t *testing.T, s *programSuite) map[string][]byte {
	t.Helper()
	store := s.ctx.KVStore(s.key)
	state := map[string][]byte{"web/count/pending": bytes.Clone(store.Get([]byte("web/count/pending")))}
	iterator := sdk.KVStorePrefixIterator(store, []byte("web/pending"))
	defer func() { require.NoError(t, iterator.Close()) }()
	for ; iterator.Valid(); iterator.Next() {
		state[string(iterator.Key())] = bytes.Clone(iterator.Value())
	}
	return state
}

func TestRequestFromProgramDisabledBeforeTheMergePlan(t *testing.T) {
	s := newProgramGateSuite(t)
	before := programGatePendingState(t, s)
	count := s.k.ProgramPendingCount(s.ctx)
	for _, planDone := range []bool{false, true} {
		if planDone {
			s.app.UpgradeKeeper.SetDone(s.ctx.WithBlockHeight(100), keeper.ProgramOriginPlan)
		}
		for _, height := range []int64{1, 8, 99} {
			for _, paused := range []bool{false, true} {
				for _, malformed := range []bool{false, true} {
					t.Run(fmt.Sprintf("done_%t/height_%d/paused_%t/malformed_%t", planDone, height, paused, malformed), func(t *testing.T) {
						ctx := s.ctx.WithBlockHeight(height)
						if paused {
							require.NoError(t, s.k.Pause(ctx, types.MsgPause{Authority: types.DefaultAuthority()}))
						} else {
							require.NoError(t, s.k.Unpause(ctx, types.MsgUnpause{Authority: types.DefaultAuthority()}))
						}
						require.False(t, s.k.ProgramOriginActive(ctx))
						call := newCall(feeLeg(programPrincipal, paxAsset, sdk.NewInt(1)))
						data := requestData(11, types.KindSearch, programPayload)
						if malformed {
							data = []byte{1}
						}
						err := s.k.RequestFromProgram(ctx, call, []byte(keeper.ProgramRequestTopic), data)
						require.EqualError(t, err, "LXP_MODULE_DISABLED")
						require.ErrorIs(t, err, keeper.ErrProgramModuleDisabled)
						require.Equal(t, before, programGatePendingState(t, s))
						require.Equal(t, count, s.k.ProgramPendingCount(ctx))
					})
				}
			}
		}
	}
}

func TestRequestFromProgramOtherTopicBeforeTheMergePlanRecordsNothing(t *testing.T) {
	s := newProgramGateSuite(t)
	before := programGatePendingState(t, s)
	count := s.k.ProgramPendingCount(s.ctx)
	for _, planDone := range []bool{false, true} {
		if planDone {
			s.app.UpgradeKeeper.SetDone(s.ctx.WithBlockHeight(100), keeper.ProgramOriginPlan)
		}
		for _, height := range []int64{1, 8, 99, 100, 101, 1000} {
			for _, paused := range []bool{false, true} {
				t.Run(fmt.Sprintf("done_%t/height_%d/paused_%t", planDone, height, paused), func(t *testing.T) {
					ctx := s.ctx.WithBlockHeight(height)
					if paused {
						require.NoError(t, s.k.Pause(ctx, types.MsgPause{Authority: types.DefaultAuthority()}))
					} else {
						require.NoError(t, s.k.Unpause(ctx, types.MsgUnpause{Authority: types.DefaultAuthority()}))
					}
					require.Equal(t, planDone && height >= 100, s.k.ProgramOriginActive(ctx))
					for _, topic := range [][]byte{nil, []byte("PROGRAM_EVENT"), []byte(keeper.ProgramRequestTopic + "_OTHER")} {
						require.NoError(t, s.k.RequestFromProgram(ctx, nil, topic, []byte{1}))
						call := newCall(feeLeg(programPrincipal, paxAsset, sdk.NewInt(1)))
						require.NoError(t, s.k.RequestFromProgram(ctx, call, topic, requestData(11, types.KindSearch, programPayload)))
						require.Equal(t, before, programGatePendingState(t, s))
						require.Equal(t, count, s.k.ProgramPendingCount(ctx))
					}
				})
			}
		}
	}
}
