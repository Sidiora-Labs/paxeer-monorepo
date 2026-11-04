package simulation_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/keeper/testutils"
	oraclesim "github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/simulation"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	simtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/simulation"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/ante"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramstypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types/proposal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repairM256Input(t *testing.T) testutils.TestInput {
	t.Helper()
	input := testutils.CreateTestInput(t)
	input.Ctx = input.Ctx.WithBlockHeight(11).WithBlockTime(time.Unix(1700000000, 0).UTC())
	t.Cleanup(func() { require.NoError(t, input.Ctx.MultiStore().(store.CommitMultiStore).Close()) })
	return input
}

func repairM256ParamsKeeper(t *testing.T, input testutils.TestInput) paramskeeper.Keeper {
	t.Helper()
	keys := make(map[string]sdk.StoreKey)
	for _, key := range input.Ctx.MultiStore().(store.CommitMultiStore).StoreKeys() {
		keys[key.Name()] = key
	}
	pk := paramskeeper.NewKeeper(input.OracleKeeper.GetCdc(), input.Cdc,
		keys[paramstypes.StoreKey], keys[paramstypes.TStoreKey])
	pk.Subspace(types.ModuleName).WithKeyTable(types.ParamKeyTable())
	return pk
}

func TestRepairM_256_DecoderPersistedSchemas(t *testing.T) {
	input := repairM256Input(t)
	ctx, k := input.Ctx, input.OracleKeeper
	cdc := testutils.MakeEncodingConfig(t).Marshaler
	decode := oraclesim.NewDecodeStore(cdc)
	val := testutils.ValAddrs[0]
	pair := func(key sdk.StoreKey, entry []byte) kv.Pair {
		value := ctx.KVStore(key).Get(entry)
		require.NotEmpty(t, value)
		return kv.Pair{Key: append([]byte(nil), entry...), Value: append([]byte(nil), value...)}
	}
	compare := func(a, b kv.Pair, wantA, wantB interface{}) {
		t.Helper()
		require.NotEqual(t, a.Value, b.Value)
		text := decode(a, b)
		require.Equal(t, fmt.Sprintf("%v\n%v", wantA, wantB), text)
		require.NotEqual(t, decode(a, a), text)
	}

	k.SetBaseExchangeRate(ctx, utils.MicroAtomDenom, sdk.NewDec(12))
	rateKey := types.GetExchangeRateKey(utils.MicroAtomDenom)
	a := pair(k.GetStoreKey(), rateKey)
	rateA := types.OracleExchangeRate{ExchangeRate: sdk.NewDec(12), LastUpdate: sdk.NewInt(11), LastUpdateTimestamp: ctx.BlockTime().UnixMilli()}
	heightCtx := ctx.WithBlockHeight(12)
	k.SetBaseExchangeRate(heightCtx, utils.MicroAtomDenom, sdk.NewDec(12))
	b := pair(k.GetStoreKey(), rateKey)
	rateB := rateA
	rateB.LastUpdate = sdk.NewInt(12)
	compare(a, b, rateA, rateB)
	k.SetBaseExchangeRate(heightCtx.WithBlockTime(ctx.BlockTime().Add(time.Second)), utils.MicroAtomDenom, sdk.NewDec(12))
	c := pair(k.GetStoreKey(), rateKey)
	rateC := rateB
	rateC.LastUpdateTimestamp += 1000
	compare(b, c, rateB, rateC)

	snapshotA := types.NewPriceSnapshot(types.PriceSnapshotItems{types.NewPriceSnapshotItem(utils.MicroAtomDenom, rateA)}, ctx.BlockTime().Unix())
	k.AddPriceSnapshot(ctx, snapshotA)
	snapshotKey := types.GetPriceSnapshotKey(uint64(snapshotA.SnapshotTimestamp))
	a = pair(k.GetStoreKey(), snapshotKey)
	snapshotB := types.NewPriceSnapshot(types.PriceSnapshotItems{types.NewPriceSnapshotItem(utils.MicroAtomDenom, rateC)}, snapshotA.SnapshotTimestamp)
	k.SetPriceSnapshot(ctx, snapshotB)
	compare(a, pair(k.GetStoreKey(), snapshotKey), snapshotA, snapshotB)

	k.SetFeederDelegation(ctx, val, testutils.Addrs[1])
	a = pair(k.GetStoreKey(), types.GetFeederDelegationKey(val))
	k.SetFeederDelegation(ctx, val, testutils.Addrs[2])
	compare(a, pair(k.GetStoreKey(), a.Key), testutils.Addrs[1], testutils.Addrs[2])

	k.SetVotePenaltyCounter(ctx, val, 1, 2, 3)
	a = pair(k.GetStoreKey(), types.GetVotePenaltyCounterKey(val))
	k.SetVotePenaltyCounter(ctx, val, 1, 2, 4)
	compare(a, pair(k.GetStoreKey(), a.Key), types.VotePenaltyCounter{MissCount: 1, AbstainCount: 2, SuccessCount: 3}, types.VotePenaltyCounter{MissCount: 1, AbstainCount: 2, SuccessCount: 4})

	voteA := types.NewAggregateExchangeRateVote(types.ExchangeRateTuples{types.NewExchangeRateTuple(utils.MicroAtomDenom, sdk.NewDec(12))}, val)
	k.SetAggregateExchangeRateVote(ctx, val, voteA)
	a = pair(k.GetStoreKey(), types.GetAggregateExchangeRateVoteKey(val))
	voteB := types.NewAggregateExchangeRateVote(types.ExchangeRateTuples{types.NewExchangeRateTuple(utils.MicroAtomDenom, sdk.NewDec(13))}, val)
	k.SetAggregateExchangeRateVote(ctx, val, voteB)
	compare(a, pair(k.GetStoreKey(), a.Key), voteA, voteB)

	k.SetVoteTarget(ctx, utils.MicroAtomDenom)
	k.SetVoteTarget(ctx, utils.MicroEthDenom)
	compare(pair(k.GetStoreKey(), types.GetVoteTargetKey(utils.MicroAtomDenom)), pair(k.GetStoreKey(), types.GetVoteTargetKey(utils.MicroEthDenom)), types.Denom{Name: utils.MicroAtomDenom}, types.Denom{Name: utils.MicroEthDenom})

	var memKey sdk.StoreKey
	for _, key := range ctx.MultiStore().(store.CommitMultiStore).StoreKeys() {
		if key.Name() == types.MemStoreKey {
			memKey = key
		}
	}
	require.NotNil(t, memKey)
	require.NoError(t, k.CheckAndSetSpamPreventionCounter(ctx, val))
	a = pair(memKey, types.GetSpamPreventionCounterKey(val))
	require.NoError(t, k.CheckAndSetSpamPreventionCounter(heightCtx, val))
	compare(a, pair(memKey, a.Key), uint64(11), uint64(12))
	require.Panics(t, func() { decode(kv.Pair{}, kv.Pair{}) })
	require.Panics(t, func() { decode(kv.Pair{Key: []byte{0xff}}, kv.Pair{}) })
}

func TestRepairM_256_RandomizedGenesis(t *testing.T) {
	input := repairM256Input(t)
	cdc := testutils.MakeEncodingConfig(t).Marshaler
	for seed := int64(0); seed < 64; seed++ {
		state := module.SimulationState{Cdc: cdc, Rand: rand.New(rand.NewSource(seed)), AppParams: simtypes.AppParams{}, GenState: map[string]json.RawMessage{}}
		oraclesim.RandomizedGenState(&state)
		var genesis types.GenesisState
		cdc.MustUnmarshalJSON(state.GenState[types.ModuleName], &genesis)
		require.NoError(t, types.ValidateGenesis(&genesis), "seed %d", seed)
		require.Positive(t, genesis.Params.LookbackDuration)
		require.Equal(t, types.DefaultParams().LookbackDuration, genesis.Params.LookbackDuration)
		require.Zero(t, genesis.Params.SlashWindow%genesis.Params.VotePeriod)
		oracle.InitGenesis(input.Ctx, input.OracleKeeper, &genesis)
		require.Equal(t, genesis.Params, input.OracleKeeper.GetParams(input.Ctx))
	}
	for _, overrides := range []simtypes.AppParams{
		{"vote_period": json.RawMessage(`97`)},
		{"vote_period": json.RawMessage(`7`), "slash_window": json.RawMessage(`700`)},
	} {
		state := module.SimulationState{Cdc: cdc, Rand: rand.New(rand.NewSource(17)), AppParams: overrides, GenState: map[string]json.RawMessage{}}
		oraclesim.RandomizedGenState(&state)
		var genesis types.GenesisState
		cdc.MustUnmarshalJSON(state.GenState[types.ModuleName], &genesis)
		require.NoError(t, types.ValidateGenesis(&genesis))
		require.Positive(t, genesis.Params.LookbackDuration)
	}
	for _, overrides := range []simtypes.AppParams{
		{"vote_period": json.RawMessage(`0`)},
		{"vote_period": json.RawMessage(`7`), "slash_window": json.RawMessage(`100`)},
		{"vote_period": json.RawMessage(`18446744073709551615`)},
	} {
		previous := json.RawMessage(`{"preserved":true}`)
		state := module.SimulationState{Cdc: cdc, Rand: rand.New(rand.NewSource(17)), AppParams: overrides, GenState: map[string]json.RawMessage{types.ModuleName: previous}}
		require.Panics(t, func() { oraclesim.RandomizedGenState(&state) })
		require.Equal(t, previous, state.GenState[types.ModuleName])
	}
}

func TestRepairM_256_AggregateVoteDelivery(t *testing.T) {
	for _, delegated := range []bool{false, true} {
		t.Run(fmt.Sprintf("delegated_%t", delegated), func(t *testing.T) {
			input := repairM256Input(t)
			// The shared keeper fixture predates deferred fees. Mount the same
			// dedicated memory cache and bank implementation used by the node.
			ms := input.Ctx.MultiStore().(store.CommitMultiStore)
			ms.Commit(true)
			cacheKey := sdk.NewMemoryStoreKeys(banktypes.DeferredCacheStoreKey)[banktypes.DeferredCacheStoreKey]
			ms.MountStoreWithDB(cacheKey, sdk.StoreTypeMemory, nil)
			require.NoError(t, ms.LoadLatestVersion())
			var bankKey sdk.StoreKey
			for _, key := range ms.StoreKeys() {
				if key.Name() == banktypes.StoreKey {
					bankKey = key
				}
			}
			require.NotNil(t, bankKey)
			pk := repairM256ParamsKeeper(t, input)
			input.BankKeeper = bankkeeper.NewBaseKeeperWithDeferredCache(input.OracleKeeper.GetCdc(), bankKey,
				input.AccountKeeper, pk.Subspace(banktypes.ModuleName), nil, cacheKey)
			r := rand.New(rand.NewSource(256))
			accounts := simtypes.RandomAccounts(r, 2)
			for _, account := range accounts {
				input.AccountKeeper.SetAccount(input.Ctx, input.AccountKeeper.NewAccountWithAddress(input.Ctx, account.Address))
				require.NoError(t, testutils.FundAccount(input, account.Address, testutils.InitCoins))
			}
			validator := sdk.ValAddress(accounts[0].Address)
			_, err := staking.NewHandler(input.StakingKeeper)(input.Ctx,
				testutils.NewTestMsgCreateValidator(validator, accounts[0].ConsKey.PubKey(), sdk.TokensFromConsensusPower(100, sdk.DefaultPowerReduction)))
			require.NoError(t, err)
			staking.EndBlocker(input.Ctx, input.StakingKeeper)
			require.True(t, input.StakingKeeper.Validator(input.Ctx, validator).IsBonded())
			feeder := accounts[0].Address
			if delegated {
				feeder = accounts[1].Address
				_, err = keeper.NewMsgServerImpl(input.OracleKeeper).DelegateFeedConsent(sdk.WrapSDKContext(input.Ctx), types.NewMsgDelegateFeedConsent(validator, feeder))
				require.NoError(t, err)
			}
			input.AccountKeeper.SetParams(input.Ctx, authtypes.DefaultParams())
			encoding := testutils.MakeEncodingConfig(t)
			app := baseapp.NewBaseApp(t.Name(), dbm.NewMemDB(), encoding.TxConfig.TxDecoder(), nil, &testutil.TestAppOpts{})
			app.SetCMS(input.Ctx.MultiStore().(store.CommitMultiStore))
			app.SetInterfaceRegistry(encoding.InterfaceRegistry)
			types.RegisterMsgServer(app.MsgServiceRouter(), keeper.NewMsgServerImpl(input.OracleKeeper))
			anteHandler, err := ante.NewAnteHandler(ante.HandlerOptions{AccountKeeper: input.AccountKeeper, BankKeeper: input.BankKeeper, ParamsKeeper: pk, SignModeHandler: encoding.TxConfig.SignModeHandler()})
			require.NoError(t, err)
			app.SetAnteHandler(anteHandler)
			_, err = app.InitChain(context.Background(), &abci.RequestInitChain{ChainId: "repair-m256", Time: input.Ctx.BlockTime()})
			require.NoError(t, err)
			weighted := oraclesim.WeightedOperations(simtypes.AppParams{}, encoding.Marshaler, input.AccountKeeper, input.BankKeeper, input.OracleKeeper)
			require.Len(t, weighted, 2)
			operation := weighted[0].Op()
			for round := 0; round < 4; round++ {
				ctx := app.DeliverContext().WithBlockHeight(int64(11 + 2*round)).WithBlockTime(input.Ctx.BlockTime().Add(time.Duration(round) * time.Second))
				*app.DeliverContext() = ctx
				// Include a target added after the operation was constructed.
				if round == 1 {
					p := input.OracleKeeper.GetParams(ctx)
					p.Whitelist = append(p.Whitelist, types.Denom{Name: "unew"})
					input.OracleKeeper.SetParams(ctx, p)
					input.OracleKeeper.SetVoteTarget(ctx, "unew")
				}
				success := false
				for attempt := 0; attempt < 32; attempt++ {
					msg, future, err := operation(r, app, ctx, accounts, "repair-m256")
					require.NoError(t, err, "%+v", msg)
					require.Empty(t, future)
					if msg.OK {
						success = true
						break
					}
				}
				require.True(t, success, "seeded simulation must deliver a vote each round")
				vote, err := input.OracleKeeper.GetAggregateExchangeRateVote(ctx, validator)
				require.NoError(t, err)
				require.Equal(t, validator.String(), vote.Voter)
				targets := input.OracleKeeper.GetVoteTargets(ctx)
				require.Len(t, vote.ExchangeRateTuples, len(targets))
				for _, tuple := range vote.ExchangeRateTuples {
					require.Contains(t, targets, tuple.Denom)
					require.True(t, tuple.ExchangeRate.IsPositive())
				}
				require.Equal(t, uint64(round+1), input.AccountKeeper.GetAccount(ctx, feeder).GetSequence(), "real ante handler must verify and consume the feeder signature")
				// With one validator, the lexicographically first target is the
				// reference. Other prices undergo the fixed-precision cross-rate
				// round trip before being persisted.
				reference := vote.ExchangeRateTuples[0]
				for _, tuple := range vote.ExchangeRateTuples {
					if tuple.Denom < reference.Denom {
						reference = tuple
					}
				}
				oracle.MidBlocker(ctx, input.OracleKeeper)
				for _, tuple := range vote.ExchangeRateTuples {
					rate, height, timestamp, err := input.OracleKeeper.GetBaseExchangeRate(ctx, tuple.Denom)
					require.NoError(t, err)
					expected := tuple.ExchangeRate
					if tuple.Denom != reference.Denom {
						expected = reference.ExchangeRate.Quo(reference.ExchangeRate.Quo(tuple.ExchangeRate))
					}
					require.Equal(t, expected, rate)
					require.Equal(t, ctx.BlockHeight(), height.Int64())
					require.Equal(t, ctx.BlockTime().UnixMilli(), timestamp)
				}
				require.Equal(t, uint64(round+1), input.OracleKeeper.GetSuccessCount(ctx, validator))
				input.BankKeeper.WriteDeferredBalances(ctx)
			}
			ctx := *app.DeliverContext()
			input.OracleKeeper.ClearVoteTargets(ctx)
			msg, _, err := operation(r, app, ctx, accounts[:1], "repair-m256")
			require.NoError(t, err)
			require.False(t, msg.OK)
			require.Equal(t, "no vote targets", msg.Comment)
			if delegated {
				input.OracleKeeper.SetVoteTarget(ctx, utils.MicroAtomDenom)
				msg, _, err = operation(r, app, ctx, accounts[:1], "repair-m256")
				require.NoError(t, err)
				require.False(t, msg.OK)
				require.Equal(t, "authorized feeder signing account unavailable", msg.Comment)
			}
		})
	}
}

func repairM256TransportProposal(t *testing.T, input testutils.TestInput, p *proposal.ParameterChangeProposal) *proposal.ParameterChangeProposal {
	t.Helper()
	var decoded proposal.ParameterChangeProposal
	input.Cdc.MustUnmarshalJSON(input.Cdc.MustMarshalJSON(p), &decoded)
	require.NoError(t, decoded.ValidateBasic())
	return &decoded
}

func TestRepairM_256_ParameterProgression(t *testing.T) {
	input := repairM256Input(t)
	pk := repairM256ParamsKeeper(t, input)
	handler := params.NewParamChangeProposalHandler(pk)
	producer := oraclesim.ParamChangeProposalContent(input.OracleKeeper)
	r := rand.New(rand.NewSource(256))
	seen := map[string]bool{}
	for round := 0; round < 32; round++ {
		before := input.OracleKeeper.GetParams(input.Ctx)
		p := producer(r, input.Ctx, nil).(*proposal.ParameterChangeProposal)
		for _, change := range p.Changes {
			seen[change.Key] = true
		}
		decoded := repairM256TransportProposal(t, input, p)
		require.NoError(t, handler(input.Ctx, decoded))
		after := input.OracleKeeper.GetParams(input.Ctx)
		require.NoError(t, after.Validate())
		require.Positive(t, after.LookbackDuration)
		if round == 0 {
			require.NotEqual(t, before, after)
		}
	}
	for _, key := range [][]byte{types.KeyVotePeriod, types.KeySlashWindow, types.KeyVoteThreshold, types.KeyRewardBand, types.KeySlashFraction} {
		require.True(t, seen[string(key)], "generator must exercise %s", key)
	}
	// Exercise every subset in both timing orders through the real handler.
	for _, start := range []struct{ period, window uint64 }{{7, 700}, {97, 970}, {101, 101}, {1, math.MaxUint64}} {
		current := types.DefaultParams()
		current.VotePeriod, current.SlashWindow = start.period, start.window
		for mask := 1; mask < 32; mask++ {
			for _, reverse := range []bool{false, true} {
				input.OracleKeeper.SetParams(input.Ctx, current)
				pool := oraclesim.ParamChangesForParams(r, input.OracleKeeper.GetParams(input.Ctx))
				require.Len(t, pool, 5)
				var changes []proposal.ParamChange
				for index, change := range pool {
					if mask&(1<<index) != 0 {
						changes = append(changes, proposal.NewParamChange(change.Subspace(), change.Key(), change.SimValue()(r)))
					}
				}
				if reverse {
					for left, right := 0, len(changes)-1; left < right; left, right = left+1, right-1 {
						changes[left], changes[right] = changes[right], changes[left]
					}
				}
				p := proposal.NewParameterChangeProposal("Valid subset", "Apply generated oracle changes", changes, false)
				require.NoError(t, handler(input.Ctx, repairM256TransportProposal(t, input, p)))
				require.NoError(t, input.OracleKeeper.GetParams(input.Ctx).Validate())
			}
		}
	}
	// The legacy context-free pool must remain safe against nondefault state.
	for _, change := range oraclesim.ParamChanges(r) {
		p := proposal.NewParameterChangeProposal("Legacy scalar", "Apply independent scalar parameter", []proposal.ParamChange{proposal.NewParamChange(change.Subspace(), change.Key(), change.SimValue()(r))}, false)
		require.NoError(t, handler(input.Ctx, repairM256TransportProposal(t, input, p)))
		require.NoError(t, input.OracleKeeper.GetParams(input.Ctx).Validate())
	}
}

func TestRepairM_256_IntentionalParameterRejections(t *testing.T) {
	input := repairM256Input(t)
	handler := params.NewParamChangeProposalHandler(repairM256ParamsKeeper(t, input))
	for _, key := range [][]byte{types.KeyVotePeriod, types.KeySlashWindow} {
		before := input.OracleKeeper.GetParams(input.Ctx)
		p := proposal.NewParameterChangeProposal("Invalid zero", "Intentionally reject a zero timing parameter", []proposal.ParamChange{proposal.NewParamChange(types.ModuleName, string(key), `"0"`)}, false)
		cache, _ := input.Ctx.CacheContext()
		require.Error(t, handler(cache, repairM256TransportProposal(t, input, p)))
		require.Equal(t, before, input.OracleKeeper.GetParams(input.Ctx))
	}
	invalid := types.DefaultParams()
	invalid.VotePeriod, invalid.SlashWindow = 7, 100
	require.Error(t, invalid.Validate())
	require.Panics(t, func() { oraclesim.ParamChangesForParams(rand.New(rand.NewSource(256)), invalid) })
}
