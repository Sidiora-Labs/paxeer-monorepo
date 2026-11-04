package gov_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmnode "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/node"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storev2rootmulti "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	moduletestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/client/cli"
	govkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	storageconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	gateway "github.com/gogo/gateway"
	"github.com/gogo/protobuf/proto"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repairM679Encoding() moduletestutil.TestEncodingConfig {
	return moduletestutil.MakeTestEncodingConfig(auth.AppModuleBasic{}, bank.AppModuleBasic{}, staking.AppModuleBasic{}, gov.AppModuleBasic{})
}

func repairM679Doc(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("spec", name))
	require.NoError(t, err)
	return string(b)
}

func repairM679Example(t *testing.T, name, marker string) string {
	t.Helper()
	pattern := "(?s)<!-- repair-m679:" + regexp.QuoteMeta(marker) + " -->\\s*```json\\s*(.*?)\\s*```"
	match := regexp.MustCompile(pattern).FindStringSubmatch(repairM679Doc(t, name))
	require.Len(t, match, 2, "missing executable documentation example %s", marker)
	return match[1]
}

func repairM679State(t *testing.T) *repair305State {
	t.Helper()
	s := openRepair305State(t, t.TempDir(), time.Unix(1700000000, 0).UTC())
	t.Cleanup(func() { require.NoError(t, s.cms.Close()) })
	return s
}

func repairM679ConversionRestart(t *testing.T) {
	for _, tc := range []struct {
		name   string
		option types.VoteOption
		status types.ProposalStatus
		burn   bool
	}{
		{"minority_yes", types.OptionYes, types.StatusPassed, false},
		{"minority_no", types.OptionNo, types.StatusRejected, false},
		{"minority_veto", types.OptionNoWithVeto, types.StatusRejected, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			s := openRepair305State(t, dir, time.Unix(1700000000, 0).UTC())
			t.Cleanup(func() { require.NoError(t, s.cms.Close()) })
			addrs := s.electorate(t, true)
			p := s.submit(t, addrs[0])
			msg := types.NewMsgVote(addrs[1], p.ProposalId, tc.option)
			require.NoError(t, msg.ValidateBasic())
			_, err := govkeeper.NewMsgServerImpl(s.gov).Vote(sdk.WrapSDKContext(s.ctx), msg)
			require.NoError(t, err)
			before, found := s.gov.GetVote(s.ctx, p.ProposalId, addrs[1])
			require.True(t, found)
			deposits := s.gov.GetDeposits(s.ctx, p.ProposalId)
			escrow := s.bank.GetAllBalances(s.ctx, s.auth.GetModuleAddress(types.ModuleName))
			balance := s.bank.GetAllBalances(s.ctx, addrs[0])
			commit := s.commitAndClose(t)

			s = openRepair305State(t, dir, p.VotingEndTime)
			require.Equal(t, commit, s.cms.LastCommitID())
			vote, found := s.gov.GetVote(s.ctx, p.ProposalId, addrs[1])
			require.True(t, found)
			require.Equal(t, before, vote)
			gov.EndBlocker(s.ctx, s.gov)
			converted, found := s.gov.GetProposal(s.ctx, p.ProposalId)
			require.True(t, found)
			require.False(t, converted.IsExpedited)
			require.Equal(t, types.StatusVotingPeriod, converted.Status)
			require.Equal(t, p.VotingStartTime.Add(s.gov.GetVotingParams(s.ctx).VotingPeriod), converted.VotingEndTime)
			require.Equal(t, deposits, s.gov.GetDeposits(s.ctx, p.ProposalId))
			require.Equal(t, escrow, s.bank.GetAllBalances(s.ctx, s.auth.GetModuleAddress(types.ModuleName)))
			oldQueue := s.gov.ActiveProposalQueueIterator(s.ctx, p.VotingEndTime)
			require.False(t, oldQueue.Valid())
			require.NoError(t, oldQueue.Close())
			commit = s.commitAndClose(t)

			s = openRepair305State(t, dir, converted.VotingEndTime.Add(-time.Second))
			require.Equal(t, commit, s.cms.LastCommitID())
			queries := baseapp.NewQueryServerTestHelper(s.ctx, repairM679Encoding().InterfaceRegistry)
			types.RegisterQueryServer(queries, s.gov)
			response, err := types.NewQueryClient(queries).Vote(t.Context(), &types.QueryVoteRequest{ProposalId: p.ProposalId, Voter: addrs[1].String()})
			require.NoError(t, err)
			require.Equal(t, before, response.Vote)
			gov.EndBlocker(s.ctx, s.gov)
			require.Len(t, s.gov.GetVotes(s.ctx, p.ProposalId), 1)
			s.ctx = s.ctx.WithBlockTime(converted.VotingEndTime)
			// Do not cast or replace a vote after conversion: the recovered vote decides the result.
			gov.EndBlocker(s.ctx, s.gov)
			final, found := s.gov.GetProposal(s.ctx, p.ProposalId)
			require.True(t, found)
			require.Equal(t, tc.status, final.Status)
			power := sdk.TokensFromConsensusPower(6, sdk.DefaultPowerReduction)
			expected := types.EmptyTallyResult()
			switch tc.option {
			case types.OptionYes:
				expected.Yes = power
			case types.OptionNo:
				expected.No = power
			case types.OptionNoWithVeto:
				expected.NoWithVeto = power
			}
			require.Equal(t, expected, final.FinalTallyResult)
			require.Empty(t, s.gov.GetVotes(s.ctx, p.ProposalId))
			require.Empty(t, s.gov.GetDeposits(s.ctx, p.ProposalId))
			require.True(t, s.bank.GetAllBalances(s.ctx, s.auth.GetModuleAddress(types.ModuleName)).IsZero())
			expectedBalance := balance
			if !tc.burn {
				expectedBalance = balance.Add(p.TotalDeposit...)
			}
			require.Equal(t, expectedBalance, s.bank.GetAllBalances(s.ctx, addrs[0]))
			s.commitAndClose(t)
			s = openRepair305State(t, dir, converted.VotingEndTime.Add(time.Second))
			recovered, found := s.gov.GetProposal(s.ctx, p.ProposalId)
			require.True(t, found)
			require.Equal(t, final, recovered)
			require.Empty(t, s.gov.GetVotes(s.ctx, p.ProposalId))
		})
	}
}

func TestRepairM_679_ConversionCoverage(t *testing.T) {
	repairM679ConversionRestart(t)
}

// The registry is an in-memory peer; the module manager, registered invariant,
// governance deposit accounting and bank transfers are all production code.
type repairM679InvariantRegistry map[string]sdk.Invariant

func (r repairM679InvariantRegistry) RegisterRoute(moduleName, route string, invariant sdk.Invariant) {
	key := moduleName + "/" + route
	if _, exists := r[key]; exists {
		panic("duplicate invariant route " + key)
	}
	r[key] = invariant
}

func TestRepairM_679_InvariantRegistered(t *testing.T) {
	s := repairM679State(t)
	addrs := s.electorate(t, false)
	p := s.submit(t, addrs[0])
	registry := repairM679InvariantRegistry{}
	manager := module.NewManager(gov.NewAppModule(repairM679Encoding().Codec, s.gov, s.auth, s.bank))
	manager.RegisterInvariants(registry)
	require.Len(t, registry, 1)
	invariant := registry["gov/module-account"]
	require.NotNil(t, invariant)
	_, broken := invariant(s.ctx)
	require.False(t, broken)
	missing := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 1))
	require.NoError(t, s.bank.SendCoinsFromModuleToAccount(s.ctx, types.ModuleName, addrs[1], missing))
	report, broken := invariant(s.ctx)
	require.True(t, broken)
	require.Contains(t, report, "sum of deposit amounts")
	require.Contains(t, report, p.TotalDeposit.String())
	require.NoError(t, s.bank.SendCoinsFromAccountToModule(s.ctx, addrs[1], types.ModuleName, missing))
	_, broken = invariant(s.ctx)
	require.False(t, broken)
}

func TestRepairM_679_LifecycleDoc(t *testing.T) {
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	key := sdk.NewKVStoreKey(upgradetypes.StoreKey)
	cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	require.NoError(t, cms.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, cms.Close()) })
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 10, Time: time.Unix(1700000000, 0).UTC()}, false)
	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	upgradetypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	home := t.TempDir()
	k := upgradekeeper.NewKeeper(map[int64]bool{}, key, cdc, home, nil)
	handler := upgrade.NewSoftwareUpgradeProposalHandler(k)
	plan := upgradetypes.Plan{Name: "m679-upgrade", Height: 20, Info: "scheduled height"}
	content := upgradetypes.NewSoftwareUpgradeProposal("Height plan", "Apply the named plan at its height", plan)
	require.NoError(t, content.ValidateBasic())
	msg, err := types.NewMsgSubmitProposal(content, nil, sdk.AccAddress(bytes.Repeat([]byte{1}, 20)))
	require.NoError(t, err)
	wire := cdc.MustMarshal(msg)
	var decoded types.MsgSubmitProposal
	cdc.MustUnmarshal(wire, &decoded)
	require.NoError(t, handler(ctx, decoded.GetContent()))
	stored, found := k.GetUpgradePlan(ctx)
	require.True(t, found)
	require.Equal(t, plan, stored)
	require.False(t, plan.ShouldExecute(ctx))
	require.NotPanics(t, func() { upgrade.BeginBlocker(k, ctx) })
	require.PanicsWithValue(t, upgrade.BuildUpgradeNeededMsg(plan), func() {
		upgrade.BeginBlocker(k, ctx.WithBlockHeight(plan.Height))
	})
	info, err := k.ReadUpgradeInfoFromDisk()
	require.NoError(t, err)
	require.Equal(t, plan.Name, info.Name)
	require.Equal(t, plan.Height, info.Height)
	cancel := upgradetypes.NewCancelSoftwareUpgradeProposal("Cancel", "Clear the pending plan")
	require.NoError(t, handler(ctx, cancel))
	_, found = k.GetUpgradePlan(ctx)
	require.False(t, found)
	past := plan
	past.Height = 9
	require.Error(t, k.ScheduleUpgrade(ctx, past))
	timed := plan
	timed.Time = ctx.BlockTime()
	require.Error(t, k.ScheduleUpgrade(ctx, timed))
	doc := repairM679Doc(t, "01_concepts.md")
	require.Contains(t, doc, "ScheduleUpgrade")
	require.Contains(t, doc, "writes upgrade information to disk and halts")
	require.Contains(t, doc, "including failure from insufficient\nquorum or veto")
	require.Contains(t, doc, "accepts an empty initial coin list")
}

func repairM679Key(t *testing.T, s *repair305State, name string) sdk.StoreKey {
	t.Helper()
	for _, key := range s.cms.StoreKeys() {
		if key.Name() == name {
			return key
		}
	}
	t.Fatalf("missing mounted store %s", name)
	return nil
}

func TestRepairM_679_StateDoc(t *testing.T) {
	s := repairM679State(t)
	addrs := s.electorate(t, false)
	p := s.submit(t, addrs[0])
	enc := repairM679Encoding()
	options := repair305Options("0.4", "0.2", "0.1", "0.3")
	_, err := govkeeper.NewMsgServerImpl(s.gov).VoteWeighted(sdk.WrapSDKContext(s.ctx), types.NewMsgVoteWeighted(addrs[0], p.ProposalId, options))
	require.NoError(t, err)
	kv := s.ctx.KVStore(repairM679Key(t, s, types.StoreKey))
	id := make([]byte, 8)
	binary.BigEndian.PutUint64(id, p.ProposalId)
	require.Equal(t, append([]byte{0x00}, id...), types.ProposalKey(p.ProposalId))
	var stored types.Proposal
	enc.Codec.MustUnmarshal(kv.Get(types.ProposalKey(p.ProposalId)), &stored)
	require.Equal(t, p, stored)
	require.Equal(t, "/cosmos.gov.v1beta1.TextProposal", stored.Content.TypeUrl)
	require.True(t, stored.IsExpedited)
	require.Equal(t, id, kv.Get(types.ActiveProposalQueueKey(p.ProposalId, p.VotingEndTime)))
	queueKey := append([]byte{0x01}, sdk.FormatTimeBytes(p.VotingEndTime)...)
	require.Equal(t, append(queueKey, id...), types.ActiveProposalQueueKey(p.ProposalId, p.VotingEndTime))
	require.Equal(t, "2023-11-15T22:13:20.000000000", string(sdk.FormatTimeBytes(p.VotingEndTime)))
	require.Nil(t, kv.Get(types.InactiveProposalQueueKey(p.ProposalId, p.DepositEndTime)))
	voteKey := append(append([]byte{0x20}, id...), byte(len(addrs[0])))
	require.Equal(t, append(voteKey, addrs[0]...), types.VoteKey(p.ProposalId, addrs[0]))
	var vote types.Vote
	enc.Codec.MustUnmarshal(kv.Get(types.VoteKey(p.ProposalId, addrs[0])), &vote)
	require.Equal(t, types.OptionEmpty, vote.Option)
	require.Equal(t, []types.WeightedVoteOption(options), vote.Options)
	var deposit types.Deposit
	enc.Codec.MustUnmarshal(kv.Get(types.DepositKey(p.ProposalId, addrs[0])), &deposit)
	require.Equal(t, p.TotalDeposit, deposit.Amount)
	doc := repairM679Doc(t, "02_state.md")
	for i, option := range []types.VoteOption{types.OptionEmpty, types.OptionYes, types.OptionAbstain, types.OptionNo, types.OptionNoWithVeto} {
		require.Equal(t, i, int(option))
		require.Regexp(t, regexp.QuoteMeta(option.String())+`\s*\|\s*`+fmt.Sprint(i)+`\s*\|`, doc)
	}
	require.Contains(t, doc, "2006-01-02T15:04:05.000000000")
	require.Contains(t, doc, "is_expedited")
	require.Contains(t, doc, "options")
}

func TestRepairM_679_MessageDoc(t *testing.T) {
	s := repairM679State(t)
	addrs := s.electorate(t, false)
	enc := repairM679Encoding()
	msg, err := types.NewMsgSubmitProposalWithExpedite(types.NewTextProposal("Empty deposit", "Contributions accumulate", false), nil, addrs[0], true)
	require.NoError(t, err)
	require.NoError(t, msg.ValidateBasic())
	var decoded types.MsgSubmitProposal
	enc.Codec.MustUnmarshal(enc.Codec.MustMarshal(msg), &decoded)
	require.Equal(t, msg.Content.TypeUrl, decoded.Content.TypeUrl)
	require.True(t, decoded.IsExpedited)
	require.False(t, decoded.GetContent().(*types.TextProposal).IsExpedited, "the message selects the lifecycle independently of content")
	server := govkeeper.NewMsgServerImpl(s.gov)
	response, err := server.SubmitProposal(sdk.WrapSDKContext(s.ctx), &decoded)
	require.NoError(t, err)
	id := response.ProposalId
	p, found := s.gov.GetProposal(s.ctx, id)
	require.True(t, found)
	require.True(t, p.IsExpedited)
	require.Equal(t, types.StatusDepositPeriod, p.Status)
	_, err = server.Deposit(sdk.WrapSDKContext(s.ctx), types.NewMsgDeposit(addrs[0], id, types.DefaultDepositParams().MinDeposit))
	require.NoError(t, err)
	p, found = s.gov.GetProposal(s.ctx, id)
	require.True(t, found)
	require.Equal(t, types.StatusDepositPeriod, p.Status, "regular minimum cannot activate an expedited proposal")
	_, err = server.Deposit(sdk.WrapSDKContext(s.ctx), types.NewMsgDeposit(addrs[1], id, types.DefaultDepositParams().MinDeposit))
	require.NoError(t, err)
	p, found = s.gov.GetProposal(s.ctx, id)
	require.True(t, found)
	require.Equal(t, types.StatusVotingPeriod, p.Status)
	require.Equal(t, types.DefaultDepositParams().MinExpeditedDeposit, p.TotalDeposit)
	require.Equal(t, s.ctx.BlockTime().Add(types.DefaultVotingParams().ExpeditedVotingPeriod), p.VotingEndTime)
	for _, addr := range addrs {
		deposit, found := s.gov.GetDeposit(s.ctx, id, addr)
		require.True(t, found)
		require.Equal(t, types.DefaultDepositParams().MinDeposit, deposit.Amount)
	}
	doc := repairM679Doc(t, "03_messages.md")
	require.Contains(t, doc, "possibly empty")
	require.Contains(t, doc, "message's `is_expedited`")
	require.Contains(t, doc, "cumulative")
}

func repairM679BaseApp(t *testing.T, enc moduletestutil.TestEncodingConfig) *baseapp.BaseApp {
	t.Helper()
	opts := viper.New()
	opts.Set("chain-id", "repair-m679")
	app := baseapp.NewBaseApp("repair-m679", dbm.NewMemDB(), enc.TxConfig.TxDecoder(), tmconfig.TestConfig(), opts)
	app.SetInterfaceRegistry(enc.InterfaceRegistry)
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	return app
}

func repairM679EventValues(events []abci.Event, kind, key string) []string {
	var values []string
	for _, event := range events {
		if event.Type == kind {
			for _, attr := range event.Attributes {
				if string(attr.Key) == key {
					values = append(values, string(attr.Value))
				}
			}
		}
	}
	return values
}

func TestRepairM_679_EventsDoc(t *testing.T) {
	for _, protobuf := range []bool{false, true} {
		t.Run(fmt.Sprintf("protobuf=%t", protobuf), func(t *testing.T) {
			s := repairM679State(t)
			addrs := s.electorate(t, false)
			app := repairM679BaseApp(t, repairM679Encoding())
			if protobuf {
				types.RegisterMsgServer(app.MsgServiceRouter(), govkeeper.NewMsgServerImpl(s.gov))
			} else {
				app.Router().AddRoute(sdk.NewRoute(types.RouterKey, gov.NewHandler(s.gov)))
			}
			msg, err := types.NewMsgSubmitProposalWithExpedite(types.NewTextProposal("Events", "Index the actual emitted events", true), types.DefaultDepositParams().MinExpeditedDeposit, addrs[0], true)
			require.NoError(t, err)
			result, err := app.RunMsgs(s.ctx, []sdk.Msg{msg})
			require.NoError(t, err)
			var data sdk.TxMsgData
			require.NoError(t, proto.Unmarshal(result.Data, &data))
			require.Len(t, data.Data, 1)
			var submitted types.MsgSubmitProposalResponse
			require.NoError(t, proto.Unmarshal(data.Data[0].Data, &submitted))
			id := fmt.Sprint(submitted.ProposalId)
			require.Equal(t, []string{id}, repairM679EventValues(result.Events, types.EventTypeSubmitProposal, types.AttributeKeyProposalID))
			require.Equal(t, []string{types.ProposalTypeText}, repairM679EventValues(result.Events, types.EventTypeSubmitProposal, types.AttributeKeyProposalType))
			require.Equal(t, []string{id}, repairM679EventValues(result.Events, types.EventTypeSubmitProposal, types.AttributeKeyVotingPeriodStart))
			// The bank transfer emits its own message.sender as well as the governance event.
			// Assert both senders and the exact single governance-scoped sender.
			require.Equal(t, []string{addrs[0].String(), addrs[0].String()}, repairM679EventValues(result.Events, sdk.EventTypeMessage, sdk.AttributeKeySender))
			var governanceEvents []abci.Event
			for _, event := range result.Events {
				if event.Type != sdk.EventTypeMessage {
					continue
				}
				for _, attr := range event.Attributes {
					if string(attr.Key) == sdk.AttributeKeyModule && string(attr.Value) == types.AttributeValueCategory {
						governanceEvents = append(governanceEvents, event)
					}
				}
			}
			require.Len(t, governanceEvents, 1)
			require.Equal(t, []string{addrs[0].String()}, repairM679EventValues(governanceEvents, sdk.EventTypeMessage, sdk.AttributeKeySender))
			require.Equal(t, []string{types.DefaultDepositParams().MinExpeditedDeposit.String()}, repairM679EventValues(result.Events, types.EventTypeProposalDeposit, sdk.AttributeKeyAmount))
			weighted := types.NewMsgVoteWeighted(addrs[1], submitted.ProposalId, repair305Options("0.6", "0", "0.4", "0"))
			require.NoError(t, weighted.ValidateBasic())
			result, err = app.RunMsgs(s.ctx, []sdk.Msg{weighted})
			require.NoError(t, err)
			action := weighted.Type()
			if protobuf {
				action = sdk.MsgTypeURL(weighted)
			}
			require.Equal(t, []string{action}, repairM679EventValues(result.Events, sdk.EventTypeMessage, sdk.AttributeKeyAction))
			require.Equal(t, []string{addrs[1].String()}, repairM679EventValues(result.Events, sdk.EventTypeMessage, sdk.AttributeKeySender))
			require.Equal(t, []string{types.AttributeValueCategory}, repairM679EventValues(result.Events, sdk.EventTypeMessage, sdk.AttributeKeyModule))
			require.Equal(t, []string{types.WeightedVoteOptions(weighted.Options).String()}, repairM679EventValues(result.Events, types.EventTypeProposalVote, types.AttributeKeyOption))
			vote, found := s.gov.GetVote(s.ctx, submitted.ProposalId, addrs[1])
			require.True(t, found)
			require.Equal(t, weighted.Options, vote.Options)
			require.Contains(t, repairM679Doc(t, "04_events.md"), action)
		})
	}
}

func TestRepairM_679_ParamsDoc(t *testing.T) {
	s := repairM679State(t)
	enc := repairM679Encoding()
	pk := paramskeeper.NewKeeper(enc.Codec, enc.Amino,
		repairM679Key(t, s, paramtypes.StoreKey), repairM679Key(t, s, paramtypes.TStoreKey))
	space := pk.Subspace(types.ModuleName).WithKeyTable(types.ParamKeyTable())
	defaults := types.DefaultParams()
	for _, item := range []struct {
		key   []byte
		value interface{}
	}{
		{types.ParamStoreKeyDepositParams, defaults.DepositParams},
		{types.ParamStoreKeyVotingParams, defaults.VotingParams},
		{types.ParamStoreKeyTallyParams, defaults.TallyParams},
	} {
		require.NoError(t, space.Validate(s.ctx, item.key, item.value))
	}
	// Updates exercise the same merge/validation/store path used by parameter-change proposals.
	for _, tc := range []struct {
		key  []byte
		body string
	}{
		{types.ParamStoreKeyDepositParams, `{"max_deposit_period":"0"}`},
		{types.ParamStoreKeyDepositParams, `{"min_deposit":[{"denom":"uhpx","amount":"-1"}]}`},
		{types.ParamStoreKeyDepositParams, `{"min_expedited_deposit":[{"denom":"uhpx","amount":"10000000"}]}`},
		{types.ParamStoreKeyVotingParams, `{"voting_period":"0"}`},
		{types.ParamStoreKeyVotingParams, `{"expedited_voting_period":"0"}`},
		{types.ParamStoreKeyVotingParams, `{"expedited_voting_period":"172800000000000"}`},
		{types.ParamStoreKeyTallyParams, `{"quorum":"-0.1"}`},
		{types.ParamStoreKeyTallyParams, `{"quorum":"1.1"}`},
		{types.ParamStoreKeyTallyParams, `{"expedited_quorum":"0.334"}`},
		{types.ParamStoreKeyTallyParams, `{"expedited_quorum":"1.1"}`},
		{types.ParamStoreKeyTallyParams, `{"threshold":"0"}`},
		{types.ParamStoreKeyTallyParams, `{"threshold":"1.1"}`},
		{types.ParamStoreKeyTallyParams, `{"expedited_threshold":"0.5"}`},
		{types.ParamStoreKeyTallyParams, `{"expedited_threshold":"1.1"}`},
		{types.ParamStoreKeyTallyParams, `{"veto_threshold":"0"}`},
		{types.ParamStoreKeyTallyParams, `{"veto_threshold":"1.1"}`},
	} {
		t.Run(string(tc.key)+tc.body, func(t *testing.T) {
			require.Error(t, space.Update(s.ctx, tc.key, []byte(tc.body)))
			require.Equal(t, defaults, types.NewParams(s.gov.GetVotingParams(s.ctx), s.gov.GetTallyParams(s.ctx), s.gov.GetDepositParams(s.ctx)))
		})
	}
	require.NoError(t, space.Update(s.ctx, types.ParamStoreKeyVotingParams, []byte(`{"expedited_voting_period":"43200000000000"}`)))
	require.Equal(t, 12*time.Hour, s.gov.GetVotingParams(s.ctx).ExpeditedVotingPeriod)
	require.Equal(t, defaults.VotingParams.VotingPeriod, s.gov.GetVotingParams(s.ctx).VotingPeriod)
	doc := repairM679Doc(t, "06_params.md")
	for field, value := range map[string]string{
		"min_deposit": "10000000uhpx", "min_expedited_deposit": "20000000uhpx", "max_deposit_period": "48 hours",
		"voting_period": "48 hours", "expedited_voting_period": "24 hours", "quorum": "0.334000000000000000",
		"expedited_quorum": "0.667000000000000000", "threshold": "0.500000000000000000", "expedited_threshold": "0.667000000000000000", "veto_threshold": "0.334000000000000000",
	} {
		require.Regexp(t, "(?m)^\\|[^\\n]*`"+field+"`[^\\n]*"+regexp.QuoteMeta(value), doc)
	}
	require.Equal(t, "10000000uhpx", defaults.DepositParams.MinDeposit.String())
	require.Equal(t, "20000000uhpx", defaults.DepositParams.MinExpeditedDeposit.String())
	require.Equal(t, 48*time.Hour, defaults.VotingParams.VotingPeriod)
	require.Equal(t, 24*time.Hour, defaults.VotingParams.ExpeditedVotingPeriod)
	require.Equal(t, defaults.VotingParams.VotingPeriod, defaults.DepositParams.MaxDepositPeriod)
	require.Equal(t, "0.334000000000000000", defaults.TallyParams.Quorum.String())
	require.Equal(t, "0.667000000000000000", defaults.TallyParams.ExpeditedQuorum.String())
	require.Equal(t, "0.500000000000000000", defaults.TallyParams.Threshold.String())
	require.Equal(t, "0.667000000000000000", defaults.TallyParams.ExpeditedThreshold.String())
	require.Equal(t, "0.334000000000000000", defaults.TallyParams.VetoThreshold.String())
}

func repairM679CLI(t *testing.T, ctx client.Context, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	var output bytes.Buffer
	ctx = ctx.WithOutput(&output)
	cmd.SetOut(&output)
	cmd.SetErr(io.Discard)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.WithValue(t.Context(), client.ClientContextKey, &ctx))
	return output.String(), err
}

func TestRepairM_679_ClientDoc(t *testing.T) {
	enc := repairM679Encoding()
	app := repairM679BaseApp(t, enc)
	cfg := tmconfig.TestConfig().SetRoot(t.TempDir())
	tmconfig.EnsureRoot(cfg.RootDir)
	cfg.Mode = tmconfig.ModeFull
	cfg.DBBackend = "memdb"
	cfg.TxIndex.Indexer = []string{"kv"}
	app.TmConfig = cfg
	sc := storageconfig.DefaultStateCommitConfig()
	sc.MemIAVLConfig.AsyncCommitBuffer = 0
	sc.MemIAVLConfig.SnapshotInterval = 1
	sc.MemIAVLConfig.SnapshotMinTimeInterval = 0
	app.SetCMS(storev2rootmulti.NewStore(t.TempDir(), sc, storageconfig.StateStoreConfig{}, nil))
	keys := sdk.NewKVStoreKeys(types.StoreKey, authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey, paramtypes.StoreKey)
	tkeys := sdk.NewTransientStoreKeys(paramtypes.TStoreKey)
	app.MountKVStores(keys)
	app.MountTransientStores(tkeys)
	pk := paramskeeper.NewKeeper(enc.Codec, enc.Amino, keys[paramtypes.StoreKey], tkeys[paramtypes.TStoreKey])
	ak := authkeeper.NewAccountKeeper(enc.Codec, keys[authtypes.StoreKey], pk.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount,
		map[string][]string{
			types.ModuleName:               {authtypes.Burner},
			stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
			stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
			"repair305":                    {authtypes.Minter},
		})
	bk := bankkeeper.NewBaseKeeper(enc.Codec, keys[banktypes.StoreKey], ak, pk.Subspace(banktypes.ModuleName), nil)
	sk := stakingkeeper.NewKeeper(enc.Codec, keys[stakingtypes.StoreKey], ak, bk, pk.Subspace(stakingtypes.ModuleName))
	k := govkeeper.NewKeeper(enc.Codec, keys[types.StoreKey], pk.Subspace(types.ModuleName).WithKeyTable(types.ParamKeyTable()),
		ak, bk, sk, pk, types.NewRouter().AddRoute(types.RouterKey, types.ProposalHandler))
	types.RegisterQueryServer(app.GRPCQueryRouter(), k)
	types.RegisterMsgServer(app.MsgServiceRouter(), govkeeper.NewMsgServerImpl(k))
	require.NoError(t, app.LoadLatestVersion())
	_, err := app.InitChain(t.Context(), &abci.RequestInitChain{ChainId: "repair-m679"})
	require.NoError(t, err)
	ctx := app.NewContext(false, tmproto.Header{Height: 1, ChainID: "repair-m679", Time: time.Unix(1700000000, 0).UTC()})
	ak.SetParams(ctx, authtypes.DefaultParams())
	bk.SetParams(ctx, banktypes.DefaultParams())
	sk.SetParams(ctx, stakingtypes.DefaultParams())
	gov.InitGenesis(ctx, ak, bk, k, types.DefaultGenesisState())
	s := &repair305State{cms: app.CommitMultiStore(), ctx: ctx, gov: k, auth: ak, bank: bk, staking: sk}
	addrs := s.electorate(t, false)
	s.ctx.MultiStore().(sdk.CacheMultiStore).Write()
	require.Equal(t, int64(1), s.cms.Commit(true).Version)
	s.ctx = s.ctx.WithMultiStore(s.cms.CacheMultiStore()).WithBlockHeight(2)
	node, err := tmnode.New(t.Context(), cfg, func() {}, app,
		&tmtypes.GenesisDoc{ChainID: "repair-m679", GenesisTime: s.ctx.BlockTime()}, nil,
		tmnode.NoOpMetricsProvider(), tmtypes.ConsensusPolicy{})
	require.NoError(t, err)
	// Production local RPC, stores, block store and transaction index; no node service is started.
	t.Cleanup(node.(interface{ OnStop() }).OnStop)
	rpc, err := local.New(node)
	require.NoError(t, err)
	clientCtx := client.Context{}.WithClient(rpc).WithCodec(enc.Codec).WithLegacyAmino(enc.Amino).
		WithInterfaceRegistry(enc.InterfaceRegistry).WithTxConfig(enc.TxConfig).
		WithChainID("repair-m679").WithOutputFormat("json").WithKeyring(keyring.NewInMemory())
	proposalFile := filepath.Join(t.TempDir(), "proposal.json")
	require.NoError(t, os.WriteFile(proposalFile, []byte(repairM679Example(t, "07_client.md", "expedited-file")), 0600))
	generated, err := repairM679CLI(t, clientCtx.WithFrom(addrs[0].String()).WithFromAddress(addrs[0]),
		cli.NewCmdSubmitProposal(), "--proposal="+proposalFile, "--"+flags.FlagGenerateOnly, "--"+flags.FlagOffline)
	require.NoError(t, err)
	tx, err := enc.TxConfig.TxJSONDecoder()([]byte(generated))
	require.NoError(t, err)
	require.Len(t, tx.GetMsgs(), 1)
	submission, ok := tx.GetMsgs()[0].(*types.MsgSubmitProposal)
	require.True(t, ok)
	require.True(t, submission.IsExpedited)
	require.True(t, submission.GetContent().(*types.TextProposal).IsExpedited)
	require.Equal(t, "20000000uhpx", submission.InitialDeposit.String())
	var txs []tmtypes.Tx
	execute := func(msg sdk.Msg) {
		require.NoError(t, msg.ValidateBasic())
		builder := enc.TxConfig.NewTxBuilder()
		require.NoError(t, builder.SetMsgs(msg))
		wire, err := enc.TxConfig.TxEncoder()(builder.GetTx())
		require.NoError(t, err)
		decoded, err := enc.TxConfig.TxDecoder()(wire)
		require.NoError(t, err)
		result := app.DeliverTx(s.ctx, abci.RequestDeliverTxV2{Tx: wire}, decoded, sha256.Sum256(wire))
		require.Zero(t, result.Code, result.Log)
		indexed := &abci.TxResultV2{Height: 2, Index: uint32(len(txs)), Tx: wire,
			Result: abci.ExecTxResult{Code: result.Code, Codespace: result.Codespace, Data: result.Data, Log: result.Log,
				GasWanted: result.GasWanted, GasUsed: result.GasUsed, Events: result.Events}}
		for _, sink := range node.RPCEnvironment().EventSinks {
			require.NoError(t, sink.IndexTxEvents([]*abci.TxResultV2{indexed}))
		}
		txs = append(txs, tmtypes.Tx(wire))
	}
	initialProposerBalance := s.bank.GetAllBalances(s.ctx, addrs[0])
	initialDepositorBalance := s.bank.GetAllBalances(s.ctx, addrs[1])
	execute(submission)
	p, found := s.gov.GetProposal(s.ctx, 1)
	require.True(t, found)
	require.Equal(t, types.StatusVotingPeriod, p.Status)
	options := repair305Options("0.6", "0", "0.4", "0")
	execute(types.NewMsgVoteWeighted(addrs[0], p.ProposalId, options))
	execute(types.NewMsgDeposit(addrs[1], p.ProposalId, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 1))))
	// A real indexed block supplies the header consumed by historical tx queries.
	block := tmtypes.MakeBlock(2, txs, &tmtypes.Commit{}, nil)
	block.ChainID = "repair-m679"
	block.Time = s.ctx.BlockTime()
	pubKey, err := cryptocodec.ToTmPubKeyInterface(ed25519.GenPrivKeyFromSecret([]byte("repair-m679-validator")).PubKey())
	require.NoError(t, err)
	validators := tmtypes.NewValidatorSet([]*tmtypes.Validator{tmtypes.NewValidator(pubKey, 1)})
	block.ProposerAddress = pubKey.Address()
	block.ValidatorsHash = validators.Hash()
	block.NextValidatorsHash = validators.Hash()
	require.NoError(t, block.ValidateBasic(tmtypes.ConsensusPolicy{}))
	parts, err := block.MakePartSet(tmtypes.BlockPartSizeBytes)
	require.NoError(t, err)
	node.RPCEnvironment().BlockStore.SaveBlock(block, parts, &tmtypes.Commit{Height: 2})
	s.ctx.MultiStore().(sdk.CacheMultiStore).Write()
	require.Equal(t, int64(2), s.cms.Commit(true).Version)
	s.ctx = s.ctx.WithMultiStore(s.cms.CacheMultiStore()).WithBlockHeight(3)

	paramsJSON, err := repairM679CLI(t, clientCtx, cli.GetCmdQueryParams(), "--height=2", "--output=json")
	require.NoError(t, err)
	require.JSONEq(t, repairM679Example(t, "07_client.md", "cli-params"), paramsJSON)
	t.Logf("fixture height 2 CLI params: %s", paramsJSON)
	liveVote, err := repairM679CLI(t, clientCtx, cli.GetCmdQueryVote(), "1", addrs[0].String(), "--height=2", "--output=json")
	require.NoError(t, err)
	var vote types.Vote
	require.NoError(t, enc.Codec.UnmarshalAsJSON([]byte(liveVote), &vote))
	require.Equal(t, []types.WeightedVoteOption(options), vote.Options)

	queries := baseapp.NewQueryServerTestHelper(s.ctx.WithBlockHeight(2), enc.InterfaceRegistry)
	types.RegisterQueryServer(queries, s.gov)
	mux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &gateway.JSONPb{OrigName: true, EmitDefaults: true, AnyResolver: enc.InterfaceRegistry}),
		runtime.WithProtoErrorHandler(runtime.DefaultHTTPProtoErrorHandler),
	)
	require.NoError(t, types.RegisterQueryHandlerClient(t.Context(), mux, types.NewQueryClient(queries)))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	get := func(path string, status int) []byte {
		response, err := server.Client().Get(server.URL + "/cosmos/gov/v1beta1/" + path)
		require.NoError(t, err)
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, status, response.StatusCode, "%s: %s", path, body)
		return body
	}
	body := get("params/voting", http.StatusOK)
	var groups map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &groups))
	require.JSONEq(t, repairM679Example(t, "07_client.md", "gateway-voting"), string(groups["voting_params"]))
	t.Logf("fixture height 2 gateway voting_params: %s", groups["voting_params"])
	var voteResponse types.QueryVoteResponse
	require.NoError(t, enc.Codec.UnmarshalAsJSON(get("proposals/1/votes/"+addrs[0].String(), http.StatusOK), &voteResponse))
	require.Equal(t, vote, voteResponse.Vote)
	var deposits types.QueryDepositsResponse
	require.NoError(t, enc.Codec.UnmarshalAsJSON(get("proposals/1/deposits", http.StatusOK), &deposits))
	require.Len(t, deposits.Deposits, 2)

	s.ctx = s.ctx.WithBlockTime(p.VotingEndTime)
	gov.EndBlocker(s.ctx, s.gov)
	converted, found := s.gov.GetProposal(s.ctx, 1)
	require.True(t, found)
	require.False(t, converted.IsExpedited)
	s.ctx = s.ctx.WithBlockTime(converted.VotingEndTime)
	gov.EndBlocker(s.ctx, s.gov)
	final, found := s.gov.GetProposal(s.ctx, 1)
	require.True(t, found)
	require.Equal(t, types.StatusRejected, final.Status)
	require.Equal(t, types.EmptyTallyResult(), final.FinalTallyResult)
	require.Equal(t, initialProposerBalance, s.bank.GetAllBalances(s.ctx, addrs[0]))
	require.Equal(t, initialDepositorBalance, s.bank.GetAllBalances(s.ctx, addrs[1]))
	require.True(t, s.bank.GetAllBalances(s.ctx, s.auth.GetModuleAddress(types.ModuleName)).IsZero())
	require.Empty(t, s.gov.GetVotes(s.ctx, 1))
	require.Empty(t, s.gov.GetDeposits(s.ctx, 1))
	s.ctx.MultiStore().(sdk.CacheMultiStore).Write()
	require.Equal(t, int64(3), s.cms.Commit(true).Version)
	s.ctx = s.ctx.WithMultiStore(s.cms.CacheMultiStore())
	queries.Ctx = s.ctx
	get("proposals/1/votes/"+addrs[0].String(), http.StatusBadRequest)
	get("proposals/1/deposits/"+addrs[1].String(), http.StatusBadRequest)
	require.NoError(t, enc.Codec.UnmarshalAsJSON(get("proposals/1/deposits", http.StatusOK), &deposits))
	require.Empty(t, deposits.Deposits)
	var votes types.QueryVotesResponse
	require.NoError(t, enc.Codec.UnmarshalAsJSON(get("proposals/1/votes", http.StatusOK), &votes))
	require.Empty(t, votes.Votes)
	historical, err := repairM679CLI(t, clientCtx, cli.GetCmdQueryVote(), "1", addrs[0].String(), "--height=3", "--output=json")
	require.NoError(t, err)
	require.JSONEq(t, liveVote, historical)
	historicalDeposit, err := repairM679CLI(t, clientCtx, cli.GetCmdQueryDeposit(), "1", addrs[1].String(), "--height=3", "--output=json")
	require.NoError(t, err)
	var deposit types.Deposit
	require.NoError(t, enc.Codec.UnmarshalAsJSON([]byte(historicalDeposit), &deposit))
	require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 1)), deposit.Amount)
	require.Equal(t, addrs[1].String(), deposit.Depositor)
	doc := repairM679Doc(t, "07_client.md")
	require.Contains(t, doc, "height 2")
	require.Contains(t, doc, "height 3")
	require.Contains(t, doc, "individual `vote`")
	require.Contains(t, doc, "transaction index")
	require.NotContains(t, doc, "simd")
	require.NotContains(t, doc, "uatom")
	require.NotContains(t, doc, "cosmos1")
	require.True(t, strings.HasPrefix(addrs[0].String(), "pax1"))
}
