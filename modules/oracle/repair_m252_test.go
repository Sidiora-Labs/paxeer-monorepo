package oracle_test

import (
	"encoding/binary"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle"
	oraclewasm "github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/client/wasm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/keeper/testutils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking"
	"github.com/stretchr/testify/require"
)

func repairM252Doc(t *testing.T, path string, claims ...string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	for _, claim := range claims {
		require.Contains(t, string(data), claim, path)
	}
	return string(data)
}

func repairM252Validators(t *testing.T, input *testutils.TestInput) {
	t.Helper()
	params := input.StakingKeeper.GetParams(input.Ctx)
	params.MinCommissionRate = sdk.ZeroDec()
	input.StakingKeeper.SetParams(input.Ctx, params)
	for i := 0; i < 3; i++ {
		_, err := staking.NewHandler(input.StakingKeeper)(input.Ctx,
			testutils.NewTestMsgCreateValidator(testutils.ValAddrs[i], testutils.ValPubKeys[i], stakingAmt))
		require.NoError(t, err)
	}
	staking.EndBlocker(input.Ctx, input.StakingKeeper)
	for i := 0; i < 3; i++ {
		require.True(t, input.StakingKeeper.Validator(input.Ctx, testutils.ValAddrs[i]).IsBonded())
	}
}

func repairM252Submit(t *testing.T, input *testutils.TestInput, index int, rates string) *sdk.Result {
	t.Helper()
	msg := types.NewMsgAggregateExchangeRateVote(rates,
		input.OracleKeeper.GetFeederDelegation(input.Ctx, testutils.ValAddrs[index]), testutils.ValAddrs[index])
	require.NoError(t, msg.ValidateBasic())
	cdc := testutils.MakeTestCodec(t)
	var transported types.MsgAggregateExchangeRateVote
	require.NoError(t, cdc.Unmarshal(cdc.MustMarshal(msg), &transported))
	result, err := oracle.NewHandler(input.OracleKeeper)(input.Ctx, &transported)
	require.NoError(t, err)
	return result
}

func repairM252Targets(t *testing.T, input *testutils.TestInput) []string {
	t.Helper()
	var targets []string
	input.OracleKeeper.IterateVoteTargets(input.Ctx, func(denom string, info types.Denom) bool {
		require.Equal(t, denom, info.Name)
		targets = append(targets, denom)
		return false
	})
	query, err := keeper.NewQuerier(input.OracleKeeper).VoteTargets(sdk.WrapSDKContext(input.Ctx), &types.QueryVoteTargetsRequest{})
	require.NoError(t, err)
	require.Equal(t, targets, query.VoteTargets)
	return targets
}

func repairM252Recovery(t *testing.T, disk bool) {
	t.Helper()
	for _, format := range []string{"protobuf", "json"} {
		t.Run(format, func(t *testing.T) {
			var source, recovered *testutils.TestInput
			var sourceDir, recoveredDir string
			if disk {
				source, sourceDir = repair12Input(t)
				recovered, recoveredDir = repair12Input(t)
			} else {
				s, r := testutils.CreateTestInput(t), testutils.CreateTestInput(t)
				source, recovered = &s, &r
			}
			repairM252Validators(t, source)
			repairM252Validators(t, recovered)
			observed := time.UnixMilli(1700000000123).UTC()
			source.Ctx = source.Ctx.WithBlockHeight(11).WithBlockTime(observed)
			params := source.OracleKeeper.GetParams(source.Ctx)
			params.VotePeriod, params.SlashWindow = 2, 200
			params.VoteThreshold = sdk.NewDecWithPrec(5, 1)
			params.MinValidPerWindow, params.SlashFraction = sdk.NewDecWithPrec(75, 2), sdk.NewDecWithPrec(1, 2)
			require.NoError(t, params.Validate())
			source.OracleKeeper.SetParams(source.Ctx, params)
			source.OracleKeeper.SetBaseExchangeRate(source.Ctx, "uatom", sdk.NewDec(5))
			source.OracleKeeper.SetBaseExchangeRate(source.Ctx, "ueth", sdk.NewDec(7))
			delegate := types.NewMsgDelegateFeedConsent(testutils.ValAddrs[0], testutils.Addrs[3])
			require.NoError(t, delegate.ValidateBasic())
			_, err := oracle.NewHandler(source.OracleKeeper)(source.Ctx, delegate)
			require.NoError(t, err)
			repairM252Submit(t, source, 0, "120uatom,240ueth")
			repairM252Submit(t, source, 1, "120uatom")
			wantTargets := repairM252Targets(t, source)
			require.Equal(t, []string{"uatom", "ueth"}, wantTargets)
			if disk {
				repair12Restart(t, source, sourceDir)
			}
			exported := oracle.ExportGenesis(source.Ctx, source.OracleKeeper)
			imported := repair12Transport(t, format, exported)
			require.NoError(t, types.ValidateGenesis(imported))
			recovered.Ctx = recovered.Ctx.WithBlockHeight(100).WithBlockTime(observed.Add(48 * time.Hour))
			// The fixture normally seeds default targets. Remove them so recovery
			// cannot pass merely because the destination already has them.
			recovered.OracleKeeper.ClearVoteTargets(recovered.Ctx)
			recovered.OracleKeeper.SetVoteTarget(recovered.Ctx, "ustale")
			oracle.InitGenesis(recovered.Ctx, recovered.OracleKeeper, imported)
			require.Equal(t, wantTargets, repairM252Targets(t, recovered))
			if disk {
				repair12Restart(t, recovered, recoveredDir)
			}
			require.Equal(t, exported, oracle.ExportGenesis(recovered.Ctx, recovered.OracleKeeper))
			require.Equal(t, wantTargets, repairM252Targets(t, recovered))
			for _, denom := range wantTargets {
				want := repair12Rate(t, source, denom)
				require.Equal(t, sdk.NewInt(11), want.LastUpdate)
				require.Equal(t, observed.UnixMilli(), want.LastUpdateTimestamp)
				require.Equal(t, want, repair12Rate(t, recovered, denom))
			}
			for _, input := range []*testutils.TestInput{source, recovered} {
				input.Ctx = input.Ctx.WithBlockHeight(101).WithBlockTime(observed.Add(48 * time.Hour)).WithEventManager(sdk.NewEventManager())
				oracle.MidBlocker(input.Ctx, input.OracleKeeper)
				rate := repair12Rate(t, input, "uatom")
				require.Equal(t, sdk.NewDec(120), rate.ExchangeRate)
				require.Equal(t, sdk.NewInt(101), rate.LastUpdate)
				require.Equal(t, input.Ctx.BlockTime().UnixMilli(), rate.LastUpdateTimestamp)
				old := repair12Rate(t, input, "ueth")
				require.Equal(t, types.OracleExchangeRate{ExchangeRate: sdk.NewDec(7), LastUpdate: sdk.NewInt(11), LastUpdateTimestamp: observed.UnixMilli()}, old)
				wantCounters := []types.VotePenaltyCounter{{SuccessCount: 1}, {MissCount: 1}, {AbstainCount: 1}}
				for i, want := range wantCounters {
					require.Equal(t, want, input.OracleKeeper.GetVotePenaltyCounter(input.Ctx, testutils.ValAddrs[i]))
					_, err := input.OracleKeeper.GetAggregateExchangeRateVote(input.Ctx, testutils.ValAddrs[i])
					require.Error(t, err)
				}
				oracle.EndBlocker(input.Ctx, input.OracleKeeper)
				require.Equal(t, wantCounters[1], input.OracleKeeper.GetVotePenaltyCounter(input.Ctx, testutils.ValAddrs[1]))
				input.Ctx = input.Ctx.WithBlockHeight(199).WithEventManager(sdk.NewEventManager())
				oracle.EndBlocker(input.Ctx, input.OracleKeeper)
				for i := 0; i < 3; i++ {
					require.Equal(t, types.VotePenaltyCounter{}, input.OracleKeeper.GetVotePenaltyCounter(input.Ctx, testutils.ValAddrs[i]))
					require.Equal(t, i != 0, input.StakingKeeper.Validator(input.Ctx, testutils.ValAddrs[i]).IsJailed())
					require.Equal(t, source.StakingKeeper.Validator(source.Ctx, testutils.ValAddrs[i]).GetTokens(), input.StakingKeeper.Validator(input.Ctx, testutils.ValAddrs[i]).GetTokens())
				}
			}
			require.Equal(t, oracle.ExportGenesis(source.Ctx, source.OracleKeeper), oracle.ExportGenesis(recovered.Ctx, recovered.OracleKeeper))
		})
	}
}

func TestRepairM_252_RestartTargetsAndFirstPeriod(t *testing.T) {
	repairM252Recovery(t, true)
}

func TestRepairM_252_GenesisRoundTrip(t *testing.T) {
	TestExportInitGenesis(t)
}

func TestRepairM_252_RetainedQuerySurfaces(t *testing.T) {
	input, _ := setup(t)
	input.Ctx = input.Ctx.WithBlockHeight(11).WithBlockTime(time.UnixMilli(1700000000123).UTC())
	for i := 0; i < 3; i++ {
		repairM252Submit(t, &input, i, "12uatom,24ueth")
	}
	oracle.MidBlocker(input.Ctx, input.OracleKeeper)
	response, err := keeper.NewQuerier(input.OracleKeeper).ExchangeRates(sdk.WrapSDKContext(input.Ctx), &types.QueryExchangeRatesRequest{})
	require.NoError(t, err)
	require.Len(t, response.DenomOracleExchangeRatePairs, 2)
	cdc := testutils.MakeTestCodec(t)
	var transported types.QueryExchangeRatesResponse
	require.NoError(t, cdc.Unmarshal(cdc.MustMarshal(response), &transported))
	wasm, err := oraclewasm.NewOracleWasmQueryHandler(&input.OracleKeeper).GetExchangeRates(input.Ctx)
	require.NoError(t, err)
	require.Equal(t, &transported, wasm)
	_, err = oraclewasm.NewOracleWasmQueryHandler(&input.OracleKeeper).GetOracleTwaps(input.Ctx, &types.QueryTwapsRequest{LookbackSeconds: 60})
	require.ErrorIs(t, err, types.ErrNoTwapData)
	repairM252Doc(t, "README.md", "ErrOraclePrecompileRetired", "0x0000000000000000000000000000000000001008", "CLI, gRPC and REST", "wasm", "../../precompiles/pricefeed/PriceFeed.sol", "0x0000000000000000000000000000000000001020")
}

func TestRepairM_252_StateEncodingAndUnits(t *testing.T) {
	input, _ := setup(t)
	input.Ctx = input.Ctx.WithBlockHeight(11).WithBlockTime(time.UnixMilli(1700000000123).UTC())
	delegate := types.NewMsgDelegateFeedConsent(testutils.ValAddrs[0], testutils.Addrs[3])
	_, err := oracle.NewHandler(input.OracleKeeper)(input.Ctx, delegate)
	require.NoError(t, err)
	repairM252Submit(t, &input, 0, "12uatom,24ueth")
	input.OracleKeeper.SetBaseExchangeRate(input.Ctx, "uatom", sdk.NewDec(12))
	input.OracleKeeper.SetVotePenaltyCounter(input.Ctx, testutils.ValAddrs[0], 2, 3, 4)
	store := input.Ctx.KVStore(input.OracleKeeper.GetStoreKey())
	cdc := input.OracleKeeper.GetCdc()
	var rate types.OracleExchangeRate
	cdc.MustUnmarshal(store.Get(types.GetExchangeRateKey("uatom")), &rate)
	require.Equal(t, byte(1), types.GetExchangeRateKey("uatom")[0])
	require.Equal(t, "uatom", string(types.GetExchangeRateKey("uatom")[1:]))
	require.Equal(t, sdk.NewInt(11), rate.LastUpdate)
	require.Equal(t, input.Ctx.BlockTime().UnixMilli(), rate.LastUpdateTimestamp)
	for _, key := range [][]byte{types.GetFeederDelegationKey(testutils.ValAddrs[0]), types.GetVotePenaltyCounterKey(testutils.ValAddrs[0]), types.GetAggregateExchangeRateVoteKey(testutils.ValAddrs[0])} {
		require.Equal(t, byte(len(testutils.ValAddrs[0])), key[1])
		require.Equal(t, []byte(testutils.ValAddrs[0]), key[2:])
	}
	require.Equal(t, []byte(testutils.Addrs[3]), store.Get(types.GetFeederDelegationKey(testutils.ValAddrs[0])))
	var counter types.VotePenaltyCounter
	cdc.MustUnmarshal(store.Get(types.GetVotePenaltyCounterKey(testutils.ValAddrs[0])), &counter)
	require.Equal(t, types.VotePenaltyCounter{MissCount: 2, AbstainCount: 3, SuccessCount: 4}, counter)
	var vote types.AggregateExchangeRateVote
	cdc.MustUnmarshal(store.Get(types.GetAggregateExchangeRateVoteKey(testutils.ValAddrs[0])), &vote)
	require.Equal(t, testutils.ValAddrs[0].String(), vote.Voter)
	require.Len(t, vote.ExchangeRateTuples, 2)
	var target types.Denom
	cdc.MustUnmarshal(store.Get(types.GetVoteTargetKey("uatom")), &target)
	require.Equal(t, types.Denom{Name: "uatom"}, target)
	snapshot := types.NewPriceSnapshot(types.PriceSnapshotItems{{Denom: "uatom", OracleExchangeRate: rate}}, input.Ctx.BlockTime().Unix()-60)
	input.OracleKeeper.AddPriceSnapshot(input.Ctx, snapshot)
	key := types.GetPriceSnapshotKey(uint64(snapshot.SnapshotTimestamp))
	require.Equal(t, byte(7), key[0])
	require.Equal(t, uint64(snapshot.SnapshotTimestamp), binary.BigEndian.Uint64(key[1:]))
	var stored types.PriceSnapshot
	cdc.MustUnmarshal(store.Get(key), &stored)
	require.Equal(t, snapshot, stored)
	twaps, err := input.OracleKeeper.CalculateTwaps(input.Ctx, 60)
	require.NoError(t, err)
	require.Equal(t, types.OracleTwaps{{Denom: "uatom", Twap: sdk.NewDec(12), LookbackSeconds: 60}}, twaps)
	require.NoError(t, input.OracleKeeper.CheckAndSetSpamPreventionCounter(input.Ctx, testutils.ValAddrs[0]))
	require.ErrorIs(t, input.OracleKeeper.CheckAndSetSpamPreventionCounter(input.Ctx, testutils.ValAddrs[0]), sdkerrors.ErrAlreadyExists)
	repairM252Doc(t, "spec/02_state.md", "not Amino", "Unix milliseconds", "Unix **seconds**", "0x06 + denom", "0x07 + uint64", "reconstructs targets", "2→3", "3→4", "4→5", "5→6")
}

func TestRepairM_252_MidBlockQuorumAndRetention(t *testing.T) {
	for _, quorum := range []bool{false, true} {
		t.Run(map[bool]string{false: "below-quorum", true: "cross-rate-reference"}[quorum], func(t *testing.T) {
			input, _ := setup(t)
			params := input.OracleKeeper.GetParams(input.Ctx)
			params.VotePeriod = 2
			input.OracleKeeper.SetParams(input.Ctx, params)
			observed := time.UnixMilli(1700000000123).UTC()
			input.Ctx = input.Ctx.WithBlockHeight(7).WithBlockTime(observed)
			input.OracleKeeper.SetBaseExchangeRate(input.Ctx, "uatom", sdk.NewDec(5))
			old := repair12Rate(t, &input, "uatom")
			input.Ctx = input.Ctx.WithBlockHeight(10).WithBlockTime(observed.Add(time.Hour)).WithEventManager(sdk.NewEventManager())
			count := 1
			if quorum {
				count = 3
			}
			for i := 0; i < count; i++ {
				repairM252Submit(t, &input, i, "12uatom,24ueth")
			}
			oracle.MidBlocker(input.Ctx, input.OracleKeeper)
			require.Equal(t, old, repair12Rate(t, &input, "uatom"))
			_, err := input.OracleKeeper.GetAggregateExchangeRateVote(input.Ctx, testutils.ValAddrs[0])
			require.NoError(t, err)
			require.Empty(t, input.Ctx.EventManager().Events())
			input.Ctx = input.Ctx.WithBlockHeight(11)
			oracle.MidBlocker(input.Ctx, input.OracleKeeper)
			if quorum {
				require.Equal(t, sdk.NewDec(12), repair12Rate(t, &input, "uatom").ExchangeRate)
				require.Equal(t, sdk.NewDec(24), repair12Rate(t, &input, "ueth").ExchangeRate)
				require.Len(t, input.Ctx.EventManager().Events(), 2)
			} else {
				require.Equal(t, old, repair12Rate(t, &input, "uatom"))
				require.Empty(t, input.Ctx.EventManager().Events())
				_, _, _, err := input.OracleKeeper.GetBaseExchangeRate(input.Ctx, "ueth")
				require.ErrorIs(t, err, types.ErrUnknownDenom)
			}
			for i := 0; i < 3; i++ {
				want := types.VotePenaltyCounter{SuccessCount: 1}
				if !quorum && i != 0 {
					want = types.VotePenaltyCounter{AbstainCount: 1}
				}
				require.Equal(t, want, input.OracleKeeper.GetVotePenaltyCounter(input.Ctx, testutils.ValAddrs[i]))
				_, err := input.OracleKeeper.GetAggregateExchangeRateVote(input.Ctx, testutils.ValAddrs[i])
				require.Error(t, err)
			}
		})
	}
	repairM252Doc(t, "spec/03_end_block.md", "MidBlocker", "EndBlocker", "source height and Unix millisecond timestamp", "ascending denom name", "ClearBallots", "no ballot-winner token distribution")
}

func TestRepairM_252_MessageWireAndAuthorization(t *testing.T) {
	input, _ := setup(t)
	cdc := testutils.MakeTestCodec(t)
	delegate := types.NewMsgDelegateFeedConsent(testutils.ValAddrs[0], testutils.Addrs[3])
	require.NoError(t, delegate.ValidateBasic())
	require.Equal(t, []sdk.AccAddress{testutils.Addrs[0]}, delegate.GetSigners())
	var wireDelegate types.MsgDelegateFeedConsent
	require.NoError(t, cdc.Unmarshal(cdc.MustMarshal(delegate), &wireDelegate))
	_, err := oracle.NewHandler(input.OracleKeeper)(input.Ctx, &wireDelegate)
	require.NoError(t, err)
	msg := types.NewMsgAggregateExchangeRateVote("12uatom,24ueth", testutils.Addrs[3], testutils.ValAddrs[0])
	require.NoError(t, msg.ValidateBasic())
	require.Equal(t, []sdk.AccAddress{testutils.Addrs[3]}, msg.GetSigners())
	var transported types.MsgAggregateExchangeRateVote
	require.NoError(t, cdc.Unmarshal(cdc.MustMarshal(msg), &transported))
	require.Equal(t, *msg, transported)
	_, err = oracle.NewHandler(input.OracleKeeper)(input.Ctx, &transported)
	require.NoError(t, err)
	vote, err := input.OracleKeeper.GetAggregateExchangeRateVote(input.Ctx, testutils.ValAddrs[0])
	require.NoError(t, err)
	require.Len(t, vote.ExchangeRateTuples, 2)
	// Operator permission survives delegation and replaces the same vote key.
	operator := types.NewMsgAggregateExchangeRateVote("13uatom", testutils.Addrs[0], testutils.ValAddrs[0])
	_, err = oracle.NewHandler(input.OracleKeeper)(input.Ctx, operator)
	require.NoError(t, err)
	vote, err = input.OracleKeeper.GetAggregateExchangeRateVote(input.Ctx, testutils.ValAddrs[0])
	require.NoError(t, err)
	require.Equal(t, types.ExchangeRateTuples{{Denom: "uatom", ExchangeRate: sdk.NewDec(13)}}, vote.ExchangeRateTuples)
	for _, tc := range []struct {
		name, rates string
		feeder      sdk.AccAddress
		want        error
	}{
		{"unauthorized", "12uatom", testutils.Addrs[4], types.ErrNoVotingPermission},
		{"non-target", "12uunknown", testutils.Addrs[3], types.ErrUnknownDenom},
		{"duplicate", "12uatom,13uatom", testutils.Addrs[3], sdkerrors.ErrInvalidCoins},
		{"malformed", "invalid", testutils.Addrs[3], sdkerrors.ErrInvalidCoins},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := types.NewMsgAggregateExchangeRateVote(tc.rates, tc.feeder, testutils.ValAddrs[0])
			_, err := oracle.NewHandler(input.OracleKeeper)(input.Ctx, bad)
			require.ErrorIs(t, err, tc.want)
		})
	}
	for _, rates := range []string{"", strings.Repeat("1", 4097), "12uatom,13uatom", "invalid"} {
		require.Error(t, types.NewMsgAggregateExchangeRateVote(rates, testutils.Addrs[0], testutils.ValAddrs[0]).ValidateBasic())
	}
	for _, rates := range []string{"0uatom"} {
		require.NoError(t, types.NewMsgAggregateExchangeRateVote(rates, testutils.Addrs[0], testutils.ValAddrs[0]).ValidateBasic())
	}
	require.Error(t, types.NewMsgAggregateExchangeRateVote("-1uatom", testutils.Addrs[0], testutils.ValAddrs[0]).ValidateBasic())
	account, err := sdk.AccAddressFromBech32("pax1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq0va854")
	require.NoError(t, err)
	validator, err := sdk.ValAddressFromBech32("paxvaloper1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqyp4n3k")
	require.NoError(t, err)
	require.Equal(t, []byte(account), []byte(validator))
	repairM252Doc(t, "spec/04_messages.md", "`exchange_rates` (2)", "`feeder` (3)", "`validator` (4)", "reserves field 1", "4096 bytes", account.String(), validator.String())
}

func repairM252Event(t *testing.T, events sdk.Events, name string, attrs map[string]string) {
	t.Helper()
	found := 0
	for _, event := range events {
		require.NotEqual(t, "vote", event.Type)
		if event.Type != name {
			continue
		}
		got := make(map[string]string)
		for _, attr := range event.Attributes {
			got[string(attr.Key)] = string(attr.Value)
		}
		require.Equal(t, attrs, got)
		found++
	}
	require.Equal(t, 1, found, name)
}

func TestRepairM_252_EventsByPhase(t *testing.T) {
	input, _ := setup(t)
	params := input.OracleKeeper.GetParams(input.Ctx)
	params.Whitelist = types.DenomList{{Name: "uatom"}}
	params.VotePeriod, params.SlashWindow = 2, 4
	input.OracleKeeper.SetParams(input.Ctx, params)
	input.OracleKeeper.ClearVoteTargets(input.Ctx)
	input.OracleKeeper.SetVoteTarget(input.Ctx, "uatom")
	input.Ctx = input.Ctx.WithBlockHeight(3).WithBlockTime(time.UnixMilli(1700000000123).UTC()).WithEventManager(sdk.NewEventManager())
	server := keeper.NewMsgServerImpl(input.OracleKeeper)
	delegate := types.NewMsgDelegateFeedConsent(testutils.ValAddrs[0], testutils.Addrs[3])
	_, err := server.DelegateFeedConsent(sdk.WrapSDKContext(input.Ctx), delegate)
	require.NoError(t, err)
	repairM252Event(t, input.Ctx.EventManager().Events(), types.EventTypeFeedDelegate, map[string]string{"feeder": delegate.Delegate})
	repairM252Event(t, input.Ctx.EventManager().Events(), sdk.EventTypeMessage, map[string]string{"module": "oracle", "sender": delegate.Operator})
	for i := 0; i < 3; i++ {
		input.Ctx = input.Ctx.WithEventManager(sdk.NewEventManager())
		msg := types.NewMsgAggregateExchangeRateVote("12uatom", input.OracleKeeper.GetFeederDelegation(input.Ctx, testutils.ValAddrs[i]), testutils.ValAddrs[i])
		_, err := server.AggregateExchangeRateVote(sdk.WrapSDKContext(input.Ctx), msg)
		require.NoError(t, err)
		repairM252Event(t, input.Ctx.EventManager().Events(), types.EventTypeAggregateVote, map[string]string{"voter": msg.Validator, "exchange_rates": msg.ExchangeRates})
		repairM252Event(t, input.Ctx.EventManager().Events(), sdk.EventTypeMessage, map[string]string{"module": "oracle", "sender": msg.Feeder})
	}
	input.Ctx = input.Ctx.WithEventManager(sdk.NewEventManager())
	oracle.MidBlocker(input.Ctx, input.OracleKeeper)
	repairM252Event(t, input.Ctx.EventManager().Events(), types.EventTypeExchangeRateUpdate, map[string]string{"denom": "uatom", "exchange_rate": sdk.NewDec(12).String()})
	require.Len(t, input.Ctx.EventManager().Events(), 1)
	input.Ctx = input.Ctx.WithEventManager(sdk.NewEventManager())
	oracle.EndBlocker(input.Ctx, input.OracleKeeper)
	require.Len(t, input.Ctx.EventManager().Events(), 3)
	for _, event := range input.Ctx.EventManager().Events() {
		require.Equal(t, types.EventTypeEndSlashWindow, event.Type)
		attrs := make(map[string]string)
		for _, attr := range event.Attributes {
			attrs[string(attr.Key)] = string(attr.Value)
		}
		operator, err := sdk.ValAddressFromBech32(attrs["operator"])
		require.NoError(t, err)
		repairM252Event(t, sdk.Events{event}, types.EventTypeEndSlashWindow, map[string]string{"operator": operator.String(), "miss_count": "0", "abstain_count": "0", "success_count": "1"})
		require.Equal(t, types.VotePenaltyCounter{}, input.OracleKeeper.GetVotePenaltyCounter(input.Ctx, operator))
	}
	repairM252Doc(t, "spec/05_events.md", "## MidBlocker", "## EndBlocker", "retired scalar `vote` event is absent", "`end_slash_window`", "`miss_count`", "`abstain_count`", "`success_count`")
}

func TestRepairM_252_ParameterContract(t *testing.T) {
	input := testutils.CreateTestInput(t)
	defaults := types.DefaultParams()
	require.NoError(t, defaults.Validate())
	require.Equal(t, uint64(2), defaults.VotePeriod)
	require.Equal(t, sdk.MustNewDecFromStr("0.667"), defaults.VoteThreshold)
	require.Equal(t, sdk.MustNewDecFromStr("0.02"), defaults.RewardBand)
	require.Equal(t, types.DenomList{{Name: "uatom"}, {Name: "ueth"}}, defaults.Whitelist)
	require.Equal(t, uint64(216000), defaults.SlashWindow)
	require.Equal(t, sdk.ZeroDec(), defaults.MinValidPerWindow)
	require.Equal(t, sdk.ZeroDec(), defaults.SlashFraction)
	require.Equal(t, uint64(3600), defaults.LookbackDuration)
	response, err := keeper.NewQuerier(input.OracleKeeper).Params(sdk.WrapSDKContext(input.Ctx), &types.QueryParamsRequest{})
	require.NoError(t, err)
	cdc := testutils.MakeTestCodec(t)
	var transported types.QueryParamsResponse
	require.NoError(t, cdc.Unmarshal(cdc.MustMarshal(response), &transported))
	require.Equal(t, defaults, transported.Params)
	example := defaults
	example.VotePeriod, example.SlashWindow = 4, 200
	example.MinValidPerWindow, example.SlashFraction = sdk.NewDecWithPrec(75, 2), sdk.NewDecWithPrec(1, 2)
	example.LookbackDuration = 600
	require.NoError(t, example.Validate())
	input.OracleKeeper.SetParams(input.Ctx, example)
	require.Equal(t, example, input.OracleKeeper.GetParams(input.Ctx))
	invalid := example
	invalid.SlashWindow = 201
	require.Error(t, invalid.Validate())
	invalid = example
	invalid.VotePeriod = 0
	require.Error(t, invalid.Validate())
	require.NoError(t, input.OracleKeeper.ValidateLookbackSeconds(input.Ctx, 1))
	require.NoError(t, input.OracleKeeper.ValidateLookbackSeconds(input.Ctx, 600))
	for _, lookback := range []uint64{0, 601} {
		require.ErrorIs(t, input.OracleKeeper.ValidateLookbackSeconds(input.Ctx, lookback), types.ErrInvalidTwapLookback)
	}
	repairM252Doc(t, "spec/06_params.md", "0.667000000000000000", "216000", "3600", "8 is reserved", "`SlashWindow % VotePeriod == 0`", "0 < lookback_seconds <= LookbackDuration", "For illustration only")
	repairM252Doc(t, "../../api/oracle/oracle.proto", "Removed reward_distribution_window", "reserved 8;", "maximum positive TWAP query lookback, in seconds")
}
