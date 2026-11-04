package keeper_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/teststaking"
)

func repair306Slashes(t *testing.T) (types.QueryClient, string, []types.ValidatorSlashEvent) {
	t.Helper()
	suite := new(KeeperTestSuite)
	suite.SetT(t)
	suite.SetupTest()
	app, ctx := suite.app, suite.ctx
	helper := teststaking.NewHelper(t, ctx, app.StakingKeeper)
	helper.CreateValidator(suite.valAddrs[0], valConsPk1, sdk.NewInt(100), true)
	helper.CreateValidator(suite.valAddrs[1], valConsPk2, sdk.NewInt(100), true)

	valAddr := suite.valAddrs[0]
	heights := []uint64{99, 100, 100, 150, 200, 201}
	events := make([]types.ValidatorSlashEvent, 0, len(heights))
	for i, height := range heights {
		ctx = ctx.WithBlockHeight(int64(height))
		period := app.DistrKeeper.GetValidatorCurrentRewards(ctx, valAddr).Period
		require.Less(t, period, uint64(99), "reward periods must differ from query heights")
		fraction := sdk.NewDecWithPrec(int64(i+1), 2)
		app.DistrKeeper.Hooks().BeforeValidatorSlashed(ctx, valAddr, fraction)
		event, found := app.DistrKeeper.GetValidatorSlashEvent(ctx, valAddr, height, period)
		require.True(t, found, "slash hook must persist the height and period in the key")
		require.Equal(t, period, event.ValidatorPeriod)
		require.Equal(t, fraction, event.Fraction)
		events = append(events, event)
	}
	app.DistrKeeper.Hooks().BeforeValidatorSlashed(ctx.WithBlockHeight(125), suite.valAddrs[1], sdk.NewDecWithPrec(9, 2))
	return suite.queryClient, valAddr.String(), events
}

func TestRepair_306_SlashHeightRange(t *testing.T) {
	client, validator, events := repair306Slashes(t)
	cases := []struct {
		name       string
		start, end uint64
		want       []types.ValidatorSlashEvent
	}{
		{"inclusive boundaries", 100, 200, events[1:5]},
		{"multiple slashes at starting height", 100, 100, events[1:3]},
		{"single ending height", 200, 200, events[4:5]},
		{"no event between heights", 101, 149, nil},
		{"reward periods are not heights", events[0].ValidatorPeriod, events[5].ValidatorPeriod, nil},
		{"zero height", 0, 0, nil},
		{"full uint64 range", 0, ^uint64(0), events},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := client.ValidatorSlashes(context.Background(), &types.QueryValidatorSlashesRequest{
				ValidatorAddress: validator,
				StartingHeight:   tc.start,
				EndingHeight:     tc.end,
				Pagination:       &query.PageRequest{CountTotal: true},
			})
			require.NoError(t, err)
			require.NotNil(t, res)
			require.Len(t, res.Slashes, len(tc.want))
			for i, event := range tc.want {
				require.Equal(t, event, res.Slashes[i])
			}
			require.NotNil(t, res.Pagination)
			require.Equal(t, uint64(len(tc.want)), res.Pagination.Total)
			require.Empty(t, res.Pagination.NextKey)
		})
	}
}

func TestRepair_306_SlashHeightOffsetPagination(t *testing.T) {
	client, validator, events := repair306Slashes(t)
	for _, reverse := range []bool{false, true} {
		name := "forward"
		want := []types.ValidatorSlashEvent{events[2], events[3]}
		if reverse {
			name = "reverse"
			want = []types.ValidatorSlashEvent{events[3], events[2]}
		}
		t.Run(name, func(t *testing.T) {
			res, err := client.ValidatorSlashes(context.Background(), &types.QueryValidatorSlashesRequest{
				ValidatorAddress: validator,
				StartingHeight:   100,
				EndingHeight:     200,
				Pagination:       &query.PageRequest{Offset: 1, Limit: 2, CountTotal: true, Reverse: reverse},
			})
			require.NoError(t, err)
			require.Equal(t, want, res.Slashes)
			require.Equal(t, uint64(4), res.Pagination.Total, "only matching heights count toward pagination")
			require.NotEmpty(t, res.Pagination.NextKey)
		})
	}
}

func TestRepair_306_SlashHeightKeyPagination(t *testing.T) {
	client, validator, events := repair306Slashes(t)
	for _, reverse := range []bool{false, true} {
		name := "forward"
		want := []types.ValidatorSlashEvent{events[1], events[2], events[3], events[4]}
		if reverse {
			name = "reverse"
			want = []types.ValidatorSlashEvent{events[4], events[3], events[2], events[1]}
		}
		t.Run(name, func(t *testing.T) {
			var key []byte
			var got []types.ValidatorSlashEvent
			for page := 0; ; page++ {
				require.Less(t, page, len(events)+1, "cursor traversal must terminate")
				res, err := client.ValidatorSlashes(context.Background(), &types.QueryValidatorSlashesRequest{
					ValidatorAddress: validator,
					StartingHeight:   100,
					EndingHeight:     200,
					Pagination:       &query.PageRequest{Key: key, Limit: 2, CountTotal: true, Reverse: reverse},
				})
				require.NoError(t, err)
				require.LessOrEqual(t, len(res.Slashes), 2)
				if page == 0 {
					require.Equal(t, want[:2], res.Slashes)
					require.Equal(t, uint64(4), res.Pagination.Total)
					require.NotEmpty(t, res.Pagination.NextKey)
				}
				got = append(got, res.Slashes...)
				next := res.Pagination.NextKey
				if len(next) == 0 {
					break
				}
				require.NotEqual(t, key, next, "cursor must advance")
				key = next
			}
			require.Equal(t, want, got, "height filtering must precede pagination in both directions")
		})
	}
}
