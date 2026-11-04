package pricefeed_test

import (
	"encoding/hex"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	pricefeedtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

const moduleNowMs int64 = 1_700_000_010_000

// newModule builds the module on the test application's keepers with the
// pricefeed store layered into a context at height.
func newModule(t *testing.T, height int64) (*app.App, pricefeed.AppModule, keeper.Keeper, sdk.StoreKey, sdk.Context) {
	t.Helper()
	testApp := app.Setup(t, false, false, false)
	ctx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(height).
		WithBlockTime(time.UnixMilli(moduleNowMs).UTC())
	key := sdk.NewKVStoreKey(types.StoreKey)
	k := keeper.NewKeeper(key, testApp.StakingKeeper, testApp.AccountKeeper, &testApp.EvmKeeper)
	ctx = pricefeedtestutil.WithPriceFeedStore(ctx, key).WithEventManager(sdk.NewEventManager())
	return testApp, pricefeed.NewAppModule(k), k, key, ctx
}

func eventsOf(ctx sdk.Context, eventType string) []sdk.Event {
	var out []sdk.Event
	for _, event := range ctx.EventManager().Events() {
		if event.Type == eventType {
			out = append(out, event)
		}
	}
	return out
}

func attributes(event sdk.Event) map[string]string {
	out := make(map[string]string, len(event.Attributes))
	for _, attribute := range event.Attributes {
		out[string(attribute.Key)] = string(attribute.Value)
	}
	return out
}

func newQuorumModule(t *testing.T) (pricefeed.AppModule, keeper.Keeper, sdk.StoreKey, sdk.Context, []sdk.ValAddress) {
	t.Helper()
	powers := []int64{6, 2, 1}
	operators := make([]sdk.ValAddress, len(powers))
	validators := make([]*tmtypes.Validator, len(powers))
	for i := range powers {
		pubkey := ed25519.GenPrivKeyFromSecret([]byte{'q', 'u', 'o', 'r', 'u', 'm', byte(i)}).PubKey()
		operators[i] = sdk.ValAddress(pubkey.Address())
		tmPubkey, err := cryptocodec.ToTmPubKeyInterface(pubkey)
		require.NoError(t, err)
		validators[i] = tmtypes.NewValidator(tmPubkey, 1)
	}
	delegator := sdk.AccAddress(operators[0])
	testApp := app.SetupWithGenesisValSet(t, tmtypes.NewValidatorSet(validators),
		[]authtypes.GenesisAccount{authtypes.NewBaseAccount(delegator, nil, 0, 0)})
	ctx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(10).
		WithBlockTime(time.UnixMilli(moduleNowMs).UTC())
	key := sdk.NewKVStoreKey(types.StoreKey)
	k := keeper.NewKeeper(key, testApp.StakingKeeper, testApp.AccountKeeper, &testApp.EvmKeeper)
	ctx = pricefeedtestutil.WithPriceFeedStore(ctx, key)
	require.Len(t, testApp.StakingKeeper.GetBondedValidatorsByPower(ctx), len(powers))
	server := stakingkeeper.NewMsgServerImpl(testApp.StakingKeeper)
	for i, power := range powers {
		validator, found := testApp.StakingKeeper.GetValidator(ctx, operators[i])
		require.True(t, found)
		require.Equal(t, int64(1), validator.GetConsensusPower(sdk.DefaultPowerReduction))
		if power > 1 {
			bond := sdk.NewCoin(testApp.StakingKeeper.BondDenom(ctx), sdk.DefaultPowerReduction.MulRaw(power-1))
			require.NoError(t, testApp.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(bond)))
			require.NoError(t, testApp.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName,
				delegator, sdk.NewCoins(bond)))
			_, err := server.Delegate(sdk.WrapSDKContext(ctx), &stakingtypes.MsgDelegate{
				DelegatorAddress: delegator.String(), ValidatorAddress: operators[i].String(), Amount: bond,
			})
			require.NoError(t, err)
		}
	}
	_, err := testApp.StakingKeeper.ApplyAndReturnValidatorSetUpdates(ctx)
	require.NoError(t, err)
	bonded := testApp.StakingKeeper.GetBondedValidatorsByPower(ctx)
	require.Len(t, bonded, len(powers))
	var total int64
	for i, validator := range bonded {
		require.Equal(t, operators[i], validator.GetOperator())
		require.Equal(t, powers[i], validator.GetConsensusPower(sdk.DefaultPowerReduction))
		total += validator.GetConsensusPower(sdk.DefaultPowerReduction)
	}
	require.Equal(t, int64(9), total)
	return pricefeed.NewAppModule(k), k, key, ctx.WithEventManager(sdk.NewEventManager()), operators
}

func submitModulePrices(t *testing.T, k keeper.Keeper, ctx sdk.Context, operators []sdk.ValAddress) {
	t.Helper()
	server := keeper.NewMsgServerImpl(k)
	for _, operator := range operators {
		pubkey := secp256k1.GenPrivKeyFromSecret(append([]byte("module-feed-"), operator...)).PubKey()
		feeder, _, err := k.RegisterFeedKey(ctx, operator, pubkey.Bytes())
		require.NoError(t, err)
		response, err := server.SubmitPrices(sdk.WrapSDKContext(ctx), &types.MsgSubmitPrices{
			Feeder: feeder.String(),
			Observations: []types.Observation{{
				MarketId: 0, Sequence: 1, IndexPrice: "64250000000000000000000",
				MarkPrice: "64254000000000000000000", SourceTimestampMs: uint64(moduleNowMs - 1_000),
			}},
		})
		require.NoError(t, err)
		require.Equal(t, &types.MsgSubmitPricesResponse{Accepted: 1}, response)
		record, found := k.GetObservation(ctx, 0, operator)
		require.True(t, found)
		require.Equal(t, types.ObservationRecord{
			Sequence: 1, IndexPrice: sdk.NewIntWithDecimal(64250, 18),
			MarkPrice: sdk.NewIntWithDecimal(64254, 18), SourceTimestampMs: uint64(moduleNowMs - 1_000),
			Height: ctx.BlockHeight(),
		}, record)
	}
	require.Len(t, eventsOf(ctx, types.EventTypeObservation), len(operators))
	require.Empty(t, eventsOf(ctx, types.EventTypeObservationRefused))
}

func TestAppModule(t *testing.T) {
	am, k, key, ctx, operators := newQuorumModule(t)
	require.Equal(t, "pricefeed", am.Name())
	require.Equal(t, uint64(1), am.ConsensusVersion())

	params := types.DefaultParams(types.DefaultAuthority())
	require.NoError(t, k.SetParams(ctx, params))

	const sourceMs = uint64(moduleNowMs - 1_000)
	submitModulePrices(t, k, ctx, operators)

	am.MidBlock(ctx, 10)

	var aggregate types.Aggregate
	encoded := ctx.KVStore(key).Get(types.AggregateKey(0))
	require.NotNil(t, encoded)
	require.NoError(t, json.Unmarshal(encoded, &aggregate))
	require.Equal(t, "64250000000000000000000", aggregate.IndexPrice.String())
	require.Equal(t, "64254000000000000000000", aggregate.MarkPrice.String())
	require.Equal(t, "4000000000000000000", aggregate.Basis.String())
	require.Equal(t, sourceMs, aggregate.TimestampMs)
	require.Equal(t, int64(10), aggregate.Height)
	require.Equal(t, uint32(len(operators)), aggregate.Sources)
	for _, market := range params.Markets[1:] {
		require.Nil(t, ctx.KVStore(key).Get(types.AggregateKey(market.ID)))
	}

	sources := strconv.Itoa(len(operators))
	required := strconv.FormatUint(uint64(k.GetParams(ctx).MinSources), 10)
	written := eventsOf(ctx, types.EventTypeAggregate)
	require.Len(t, written, 1)
	require.Equal(t, map[string]string{
		types.AttributeMarketID:    "0",
		types.AttributeIndexPrice:  "64250000000000000000000",
		types.AttributeMarkPrice:   "64254000000000000000000",
		types.AttributeBasis:       "4000000000000000000",
		types.AttributeTimestampMs: strconv.FormatUint(sourceMs, 10),
		types.AttributeSources:     sources,
		types.AttributeRequired:    required,
		types.AttributeHeight:      "10",
	}, attributes(written[0]))

	skippedEvents := eventsOf(ctx, types.EventTypeAggregateSkipped)
	require.Len(t, skippedEvents, len(params.Markets)-1)
	for i, event := range skippedEvents {
		require.Equal(t, map[string]string{
			types.AttributeMarketID: strconv.FormatUint(params.Markets[i+1].ID, 10),
			types.AttributeSources:  "0",
			types.AttributeRequired: required,
		}, attributes(event))
	}
}

func TestAppModuleNoParams(t *testing.T) {
	_, am, _, key, ctx := newModule(t, 5)

	am.MidBlock(ctx, 5)

	iterator := ctx.KVStore(key).Iterator(nil, nil)
	defer func() { _ = iterator.Close() }()
	require.False(t, iterator.Valid())
	require.Empty(t, ctx.EventManager().Events())
}

func TestAppModuleInitGenesis(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		_, am, k, _, ctx := newModule(t, 1)
		genesis := types.DefaultGenesis()
		encoded, err := json.Marshal(genesis)
		require.NoError(t, err)
		require.Empty(t, am.InitGenesis(ctx, nil, encoded))
		require.Equal(t, genesis.Params, k.GetParams(ctx))
	})

	t.Run("duplicate feed account", func(t *testing.T) {
		_, am, _, key, ctx := newModule(t, 1)
		pubKey := secp256k1.GenPrivKeyFromSecret([]byte("pricefeed-module-genesis")).PubKey()
		feedKey := types.FeedKey{
			PubKey:      hex.EncodeToString(pubKey.Bytes()),
			FeedAccount: sdk.AccAddress(pubKey.Address()).String(),
		}
		genesis := types.DefaultGenesis()
		genesis.FeedKeys = []types.GenesisFeedKey{
			{Operator: sdk.ValAddress([]byte("pricefeed-operator-a")), FeedKey: feedKey},
			{Operator: sdk.ValAddress([]byte("pricefeed-operator-b")), FeedKey: feedKey},
		}
		encoded, err := json.Marshal(genesis)
		require.NoError(t, err)
		genesisErr := keeper.ValidateGenesis(*genesis)
		require.ErrorIs(t, genesisErr, types.ErrInvalidGenesis)
		require.Contains(t, genesisErr.Error(), "is mapped to validators")
		require.PanicsWithError(t, genesisErr.Error(), func() {
			am.InitGenesis(ctx, nil, encoded)
		})
		iterator := ctx.KVStore(key).Iterator(nil, nil)
		defer func() { _ = iterator.Close() }()
		require.False(t, iterator.Valid())
		require.Empty(t, ctx.EventManager().Events())
	})
}

func TestAppModuleValidateGenesis(t *testing.T) {
	basic := pricefeed.AppModuleBasic{}
	require.NoError(t, basic.ValidateGenesis(nil, nil, basic.DefaultGenesis(nil)))

	require.ErrorIs(t, basic.ValidateGenesis(nil, nil, json.RawMessage(`{"params":`)), types.ErrInvalidGenesis)

	invalid := types.DefaultGenesis()
	invalid.Params.StalenessWindowMs = 0
	encoded, err := json.Marshal(invalid)
	require.NoError(t, err)
	require.ErrorIs(t, basic.ValidateGenesis(nil, nil, encoded), types.ErrInvalidParams)

	feedAccount := sdk.AccAddress([]byte("pricefeed-feed-acct-")).String()
	duplicateAccount := types.DefaultGenesis()
	duplicateAccount.FeedKeys = []types.GenesisFeedKey{
		{Operator: sdk.ValAddress([]byte("pricefeed-operator-a")), FeedKey: types.FeedKey{FeedAccount: feedAccount}},
		{Operator: sdk.ValAddress([]byte("pricefeed-operator-b")), FeedKey: types.FeedKey{FeedAccount: feedAccount}},
	}
	encoded, err = json.Marshal(duplicateAccount)
	require.NoError(t, err)
	require.ErrorIs(t, basic.ValidateGenesis(nil, nil, encoded), types.ErrInvalidGenesis)

	duplicateOperator := types.DefaultGenesis()
	operator := sdk.ValAddress([]byte("pricefeed-operator-a"))
	duplicateOperator.FeedKeys = []types.GenesisFeedKey{
		{Operator: operator, FeedKey: types.FeedKey{FeedAccount: feedAccount}},
		{Operator: operator, FeedKey: types.FeedKey{FeedAccount: sdk.AccAddress([]byte("pricefeed-feed-acct2")).String()}},
	}
	encoded, err = json.Marshal(duplicateOperator)
	require.NoError(t, err)
	require.ErrorIs(t, basic.ValidateGenesis(nil, nil, encoded), types.ErrInvalidGenesis)
}

func TestProofPricefeedErrorsMalformedGenesisPanic(t *testing.T) {
	_, am, _, key, ctx := newModule(t, 1)
	for i := 0; i < 2; i++ {
		func() {
			defer func() {
				err, ok := recover().(error)
				require.True(t, ok, "InitGenesis must panic with a registered error")
				require.ErrorIs(t, err, types.ErrInvalidGenesis)
				name, found := types.LXPName(err)
				require.True(t, found)
				require.Equal(t, "LXP_PRICEFEED_INVALID_GENESIS", name)
			}()
			am.InitGenesis(ctx, nil, json.RawMessage(`{"params":`))
		}()
	}
	require.Nil(t, ctx.KVStore(key).Get(types.ParamsKey))
}
