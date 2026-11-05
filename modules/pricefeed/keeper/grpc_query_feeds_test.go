package keeper_test

import (
	"encoding/hex"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestQueryServerFeedKey(t *testing.T) {
	k, ctx := registryKeeper(t)
	operator := setValidator(t, ctx, "query-key", stakingtypes.Bonded)
	other := setValidator(t, ctx, "query-key-other", stakingtypes.Bonded)
	pubKey := feedPubKey("query-key")
	account, _, err := k.RegisterFeedKey(ctx, operator, pubKey)
	require.NoError(t, err)
	_, _, err = k.RegisterFeedKey(ctx, other, feedPubKey("query-key-other"))
	require.NoError(t, err)
	server := keeper.NewQueryServerImpl(k)
	ctx = ctx.WithEventManager(sdk.NewEventManager())
	before := queryStoreSnapshot(t, k, ctx)

	for _, op := range []sdk.ValAddress{operator, other} {
		stored, found := k.GetFeedKey(ctx, op)
		require.True(t, found)
		response, err := server.FeedKey(sdk.WrapSDKContext(ctx), &types.QueryFeedKeyRequest{Validator: op.String()})
		require.NoError(t, err)
		require.Equal(t, &types.QueryFeedKeyResponse{PubKey: stored.PubKey, FeedAccount: stored.FeedAccount}, response)
		if op.Equals(operator) {
			require.Equal(t, hex.EncodeToString(pubKey), response.PubKey)
			require.Equal(t, account.String(), response.FeedAccount)
		}
	}

	withoutKey := setValidator(t, ctx, "query-no-key", stakingtypes.Bonded)
	response, err := server.FeedKey(sdk.WrapSDKContext(ctx), &types.QueryFeedKeyRequest{Validator: withoutKey.String()})
	require.Nil(t, response)
	requireRefusal(t, err, types.ErrNoKey, "LXP_PRICEFEED_NO_KEY")

	for _, req := range []*types.QueryFeedKeyRequest{nil, {}, {Validator: "invalid"}, {Validator: account.String()}} {
		response, err := server.FeedKey(sdk.WrapSDKContext(ctx), req)
		require.Nil(t, response)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	require.Equal(t, before, queryStoreSnapshot(t, k, ctx))
	require.Empty(t, ctx.EventManager().Events())

	replacement := feedPubKey("query-key-replacement")
	newAccount, _, err := k.RegisterFeedKey(ctx, operator, replacement)
	require.NoError(t, err)
	response, err = server.FeedKey(sdk.WrapSDKContext(ctx), &types.QueryFeedKeyRequest{Validator: operator.String()})
	require.NoError(t, err)
	require.Equal(t, &types.QueryFeedKeyResponse{PubKey: hex.EncodeToString(replacement), FeedAccount: newAccount.String()}, response)
	_, err = k.RevokeFeedKey(ctx, operator)
	require.NoError(t, err)
	response, err = server.FeedKey(sdk.WrapSDKContext(ctx), &types.QueryFeedKeyRequest{Validator: operator.String()})
	require.Nil(t, response)
	require.ErrorIs(t, err, types.ErrNoKey)
}

func TestQueryServerValidatorByFeedAccount(t *testing.T) {
	k, ctx := registryKeeper(t)
	server := keeper.NewQueryServerImpl(k)
	operators := []sdk.ValAddress{
		setValidator(t, ctx, "query-account-a", stakingtypes.Bonded),
		setValidator(t, ctx, "query-account-b", stakingtypes.Bonded),
	}
	accounts := make([]sdk.AccAddress, len(operators))
	for i, operator := range operators {
		account, _, err := k.RegisterFeedKey(ctx, operator, feedPubKey(operator.String()))
		require.NoError(t, err)
		accounts[i] = account
	}
	ctx = ctx.WithEventManager(sdk.NewEventManager())
	before := queryStoreSnapshot(t, k, ctx)
	for i, account := range accounts {
		stored, found := k.GetValidatorByFeedAccount(ctx, account)
		require.True(t, found)
		require.Equal(t, operators[i], stored)
		response, err := server.ValidatorByFeedAccount(sdk.WrapSDKContext(ctx), &types.QueryValidatorByFeedAccountRequest{FeedAccount: account.String()})
		require.NoError(t, err)
		require.Equal(t, &types.QueryValidatorByFeedAccountResponse{Validator: stored.String()}, response)
	}

	unknown := feedAccount(feedPubKey("query-unregistered"))
	response, err := server.ValidatorByFeedAccount(sdk.WrapSDKContext(ctx), &types.QueryValidatorByFeedAccountRequest{FeedAccount: unknown.String()})
	require.Nil(t, response)
	requireRefusal(t, err, types.ErrUnregisteredFeeder, "LXP_PRICEFEED_UNREGISTERED_FEEDER")
	for _, req := range []*types.QueryValidatorByFeedAccountRequest{nil, {}, {FeedAccount: "invalid"}, {FeedAccount: operators[0].String()}} {
		response, err := server.ValidatorByFeedAccount(sdk.WrapSDKContext(ctx), req)
		require.Nil(t, response)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	require.Equal(t, before, queryStoreSnapshot(t, k, ctx))
	require.Empty(t, ctx.EventManager().Events())

	replacement, _, err := k.RegisterFeedKey(ctx, operators[0], feedPubKey("query-account-replacement"))
	require.NoError(t, err)
	response, err = server.ValidatorByFeedAccount(sdk.WrapSDKContext(ctx), &types.QueryValidatorByFeedAccountRequest{FeedAccount: accounts[0].String()})
	require.Nil(t, response)
	require.ErrorIs(t, err, types.ErrUnregisteredFeeder)
	response, err = server.ValidatorByFeedAccount(sdk.WrapSDKContext(ctx), &types.QueryValidatorByFeedAccountRequest{FeedAccount: replacement.String()})
	require.NoError(t, err)
	require.Equal(t, operators[0].String(), response.Validator)
	_, err = k.RevokeFeedKey(ctx, operators[0])
	require.NoError(t, err)
	response, err = server.ValidatorByFeedAccount(sdk.WrapSDKContext(ctx), &types.QueryValidatorByFeedAccountRequest{FeedAccount: replacement.String()})
	require.Nil(t, response)
	require.ErrorIs(t, err, types.ErrUnregisteredFeeder)
}

func TestQueryServerObservation(t *testing.T) {
	e := newSubmitEnv(t, nil)
	operator, feeder := e.feeder(t, "query-observation")
	other, otherFeeder := e.feeder(t, "query-observation-other")
	ctx := e.at(123)
	first := valid(0, 7)
	market := e.k.GetParams(ctx).Markets[1]
	second := observation(1, 11, market.MinPrice.String(), market.MinPrice.AddRaw(1).String(), uint64(submitBlockMs-100))
	response, err := e.submit(ctx, feeder, first, second)
	requireCounts(t, response, err, 2, 0)
	otherObservation := valid(0, 3)
	response, err = e.submit(ctx, otherFeeder, otherObservation)
	requireCounts(t, response, err, 1, 0)
	server := keeper.NewQueryServerImpl(e.k)
	ctx = ctx.WithEventManager(sdk.NewEventManager())
	before := queryStoreSnapshot(t, e.k, ctx)

	check := func(operator sdk.ValAddress, observation types.Observation, height int64) {
		t.Helper()
		stored, found := e.k.GetObservation(ctx, observation.MarketId, operator)
		require.True(t, found)
		require.Equal(t, types.ObservationRecord{
			Sequence: observation.Sequence, IndexPrice: intOf(t, observation.IndexPrice),
			MarkPrice: intOf(t, observation.MarkPrice), SourceTimestampMs: observation.SourceTimestampMs, Height: height,
		}, stored)
		got, err := server.Observation(sdk.WrapSDKContext(ctx), &types.QueryObservationRequest{MarketId: observation.MarketId, Validator: operator.String()})
		require.NoError(t, err)
		require.Equal(t, &types.QueryObservationResponse{
			Sequence: stored.Sequence, IndexPrice: stored.IndexPrice.String(), MarkPrice: stored.MarkPrice.String(),
			SourceTimestampMs: stored.SourceTimestampMs, Height: stored.Height,
		}, got)
	}
	check(operator, first, 123)
	check(operator, second, 123)
	check(other, otherObservation, 123)

	for _, req := range []*types.QueryObservationRequest{
		{MarketId: 2, Validator: operator.String()},
		{MarketId: 1, Validator: other.String()},
	} {
		got, err := server.Observation(sdk.WrapSDKContext(ctx), req)
		require.Nil(t, got)
		require.Equal(t, codes.NotFound, status.Code(err))
	}
	for _, req := range []*types.QueryObservationRequest{nil, {}, {Validator: "invalid"}, {Validator: feeder}} {
		got, err := server.Observation(sdk.WrapSDKContext(ctx), req)
		require.Nil(t, got)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	require.Equal(t, before, queryStoreSnapshot(t, e.k, ctx))
	require.Empty(t, ctx.EventManager().Events())

	ctx = e.at(124)
	latest := valid(0, 8)
	latest.IndexPrice = "65000500000000000000001"
	latest.MarkPrice = "65010000000000000000002"
	latest.SourceTimestampMs++
	response, err = e.submit(ctx, feeder, latest)
	requireCounts(t, response, err, 1, 0)
	ctx = ctx.WithEventManager(sdk.NewEventManager())
	before = queryStoreSnapshot(t, e.k, ctx)
	check(operator, latest, 124)
	check(operator, second, 123)
	check(other, otherObservation, 123)
	require.Equal(t, before, queryStoreSnapshot(t, e.k, ctx))
	require.Empty(t, ctx.EventManager().Events())
}
