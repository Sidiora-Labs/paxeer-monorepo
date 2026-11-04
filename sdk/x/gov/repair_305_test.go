package gov_test

import (
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov"
	govkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/keeper"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type repair305State struct {
	cms     storetypes.CommitMultiStore
	cache   storetypes.CacheMultiStore
	ctx     sdk.Context
	gov     govkeeper.Keeper
	auth    authkeeper.AccountKeeper
	bank    bankkeeper.BaseKeeper
	staking stakingkeeper.Keeper
}

func openRepair305State(t *testing.T, dir string, at time.Time) *repair305State {
	t.Helper()
	db, err := dbm.NewDB("gov", dbm.GoLevelDBBackend, dir)
	require.NoError(t, err)
	cms := store.NewCommitMultiStore(db)
	keys := sdk.NewKVStoreKeys(govtypes.StoreKey, authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey, paramtypes.StoreKey)
	for _, key := range keys {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	tkey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	cms.MountStoreWithDB(tkey, sdk.StoreTypeTransient, db)
	require.NoError(t, cms.LoadLatestVersion())
	cache := cms.CacheMultiStore()
	ctx := sdk.NewContext(cache, tmproto.Header{Height: cms.LastCommitID().Version + 1, Time: at}, false)
	registry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	govtypes.RegisterInterfaces(registry)
	stakingtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	std.RegisterLegacyAminoCodec(amino)
	authtypes.RegisterLegacyAminoCodec(amino)
	banktypes.RegisterLegacyAminoCodec(amino)
	govtypes.RegisterLegacyAminoCodec(amino)
	stakingtypes.RegisterLegacyAminoCodec(amino)
	pk := paramskeeper.NewKeeper(cdc, amino, keys[paramtypes.StoreKey], tkey)
	ak := authkeeper.NewAccountKeeper(cdc, keys[authtypes.StoreKey], pk.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, map[string][]string{
		govtypes.ModuleName:            {authtypes.Burner},
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
		"repair305":                    {authtypes.Minter},
	})
	bk := bankkeeper.NewBaseKeeper(cdc, keys[banktypes.StoreKey], ak, pk.Subspace(banktypes.ModuleName), nil)
	sk := stakingkeeper.NewKeeper(cdc, keys[stakingtypes.StoreKey], ak, bk, pk.Subspace(stakingtypes.ModuleName))
	gk := govkeeper.NewKeeper(cdc, keys[govtypes.StoreKey], pk.Subspace(govtypes.ModuleName).WithKeyTable(govtypes.ParamKeyTable()), ak, bk, sk, pk,
		govtypes.NewRouter().AddRoute(govtypes.RouterKey, govtypes.ProposalHandler))
	s := &repair305State{cms: cms, cache: cache, ctx: ctx, gov: gk, auth: ak, bank: bk, staking: sk}
	if cms.LastCommitID().Version == 0 {
		ak.SetParams(ctx, authtypes.DefaultParams())
		bk.SetParams(ctx, banktypes.DefaultParams())
		sp := stakingtypes.DefaultParams()
		sp.MinCommissionRate = sdk.ZeroDec()
		sk.SetParams(ctx, sp)
		gov.InitGenesis(ctx, ak, bk, gk, govtypes.DefaultGenesisState())
	}
	return s
}

func (s *repair305State) commitAndClose(t *testing.T) storetypes.CommitID {
	t.Helper()
	s.cache.Write()
	id := s.cms.Commit(true)
	require.NoError(t, s.cms.Close())
	return id
}

func (s *repair305State) electorate(t *testing.T, bonded bool) []sdk.AccAddress {
	t.Helper()
	addrs := []sdk.AccAddress{
		sdk.AccAddress(ed25519.GenPrivKey().PubKey().Address()),
		sdk.AccAddress(ed25519.GenPrivKey().PubKey().Address()),
	}
	for _, addr := range addrs {
		s.auth.SetAccount(s.ctx, s.auth.NewAccountWithAddress(s.ctx, addr))
		coins := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.TokensFromConsensusPower(100, sdk.DefaultPowerReduction)))
		require.NoError(t, s.bank.MintCoins(s.ctx, "repair305", coins))
		require.NoError(t, s.bank.SendCoinsFromModuleToAccount(s.ctx, "repair305", addr, coins))
	}
	if bonded {
		msg, err := stakingtypes.NewMsgCreateValidator(sdk.ValAddress(addrs[0]), ed25519.GenPrivKey().PubKey(),
			sdk.NewCoin(sdk.DefaultBondDenom, sdk.TokensFromConsensusPower(10, sdk.DefaultPowerReduction)),
			stakingtypes.Description{}, stakingtypes.NewCommissionRates(sdk.ZeroDec(), sdk.ZeroDec(), sdk.ZeroDec()), sdk.OneInt())
		require.NoError(t, err)
		_, err = stakingkeeper.NewMsgServerImpl(s.staking).CreateValidator(sdk.WrapSDKContext(s.ctx), msg)
		require.NoError(t, err)
		_, err = stakingkeeper.NewMsgServerImpl(s.staking).Delegate(sdk.WrapSDKContext(s.ctx), stakingtypes.NewMsgDelegate(
			addrs[1], sdk.ValAddress(addrs[0]), sdk.NewCoin(sdk.DefaultBondDenom, sdk.TokensFromConsensusPower(6, sdk.DefaultPowerReduction))))
		require.NoError(t, err)
		_, err = s.staking.ApplyAndReturnValidatorSetUpdates(s.ctx)
		require.NoError(t, err)
		require.True(t, s.staking.TotalBondedTokens(s.ctx).Equal(sdk.TokensFromConsensusPower(16, sdk.DefaultPowerReduction)))
	}
	return addrs
}

func (s *repair305State) submit(t *testing.T, proposer sdk.AccAddress) govtypes.Proposal {
	t.Helper()
	msg, err := govtypes.NewMsgSubmitProposalWithExpedite(govtypes.NewTextProposal("Preserve votes", "Keep the electorate through conversion", true),
		s.gov.GetDepositParams(s.ctx).MinExpeditedDeposit, proposer, true)
	require.NoError(t, err)
	res, err := govkeeper.NewMsgServerImpl(s.gov).SubmitProposal(sdk.WrapSDKContext(s.ctx), msg)
	require.NoError(t, err)
	p, found := s.gov.GetProposal(s.ctx, res.ProposalId)
	require.True(t, found)
	require.Equal(t, govtypes.StatusVotingPeriod, p.Status)
	require.True(t, p.IsExpedited)
	return p
}

func repair305Options(yes, abstain, no, veto string) govtypes.WeightedVoteOptions {
	options := govtypes.WeightedVoteOptions{}
	for i, weight := range []string{yes, abstain, no, veto} {
		w := sdk.MustNewDecFromStr(weight)
		if w.IsPositive() {
			options = append(options, govtypes.WeightedVoteOption{Option: govtypes.VoteOption(i + 1), Weight: w})
		}
	}
	return options
}

func TestRepair_305_ConversionRestartRegularTally(t *testing.T) {
	for _, tc := range []struct {
		name          string
		options       govtypes.WeightedVoteOptions
		delegatorOnly bool
		noStake       bool
		status        govtypes.ProposalStatus
		burn          bool
	}{
		{"threshold", repair305Options("0.6", "0", "0.4", "0"), false, false, govtypes.StatusPassed, false},
		{"quorum", repair305Options("1", "0", "0", "0"), true, false, govtypes.StatusPassed, false},
		{"abstain", repair305Options("0", "1", "0", "0"), false, false, govtypes.StatusRejected, false},
		{"veto", repair305Options("0", "0", "0", "1"), false, false, govtypes.StatusRejected, true},
		{"no", repair305Options("0", "0", "1", "0"), false, false, govtypes.StatusRejected, false},
		{"zero_bonded", repair305Options("1", "0", "0", "0"), false, true, govtypes.StatusRejected, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			s := openRepair305State(t, dir, time.Unix(1700000000, 0).UTC())
			addrs := s.electorate(t, !tc.noStake)
			proposal := s.submit(t, addrs[0])
			voters := addrs
			if tc.delegatorOnly {
				voters = addrs[1:]
			}
			for _, voter := range voters {
				msg := govtypes.NewMsgVoteWeighted(voter, proposal.ProposalId, tc.options)
				require.NoError(t, msg.ValidateBasic())
				_, err := govkeeper.NewMsgServerImpl(s.gov).VoteWeighted(sdk.WrapSDKContext(s.ctx), msg)
				require.NoError(t, err)
			}
			votes := s.gov.GetVotes(s.ctx, proposal.ProposalId)
			require.Len(t, votes, len(voters))
			deposits := s.gov.GetDeposits(s.ctx, proposal.ProposalId)
			balance := s.bank.GetAllBalances(s.ctx, addrs[0])
			s.ctx = s.ctx.WithBlockTime(proposal.VotingEndTime)
			gov.EndBlocker(s.ctx, s.gov)
			converted, found := s.gov.GetProposal(s.ctx, proposal.ProposalId)
			require.True(t, found)
			require.False(t, converted.IsExpedited)
			require.Equal(t, govtypes.StatusVotingPeriod, converted.Status)
			require.Equal(t, proposal.VotingStartTime.Add(s.gov.GetVotingParams(s.ctx).VotingPeriod), converted.VotingEndTime)
			require.Equal(t, votes, s.gov.GetVotes(s.ctx, proposal.ProposalId))
			require.Equal(t, deposits, s.gov.GetDeposits(s.ctx, proposal.ProposalId))
			require.Equal(t, balance, s.bank.GetAllBalances(s.ctx, addrs[0]))
			iter := s.gov.ActiveProposalQueueIterator(s.ctx, proposal.VotingEndTime)
			require.False(t, iter.Valid())
			require.NoError(t, iter.Close())
			commit := s.commitAndClose(t)

			s = openRepair305State(t, dir, proposal.VotingEndTime.Add(time.Second))
			require.Equal(t, commit, s.cms.LastCommitID())
			require.Equal(t, votes, s.gov.GetVotes(s.ctx, proposal.ProposalId))
			recovered, found := s.gov.GetProposal(s.ctx, proposal.ProposalId)
			require.True(t, found)
			require.Equal(t, converted, recovered)
			for _, voter := range voters {
				res, err := s.gov.Vote(sdk.WrapSDKContext(s.ctx), &govtypes.QueryVoteRequest{ProposalId: proposal.ProposalId, Voter: voter.String()})
				require.NoError(t, err)
				require.Equal(t, []govtypes.WeightedVoteOption(tc.options), res.Vote.Options)
			}
			s.ctx = s.ctx.WithBlockTime(converted.VotingEndTime.Add(-time.Second))
			gov.EndBlocker(s.ctx, s.gov)
			require.Equal(t, votes, s.gov.GetVotes(s.ctx, proposal.ProposalId))
			s.ctx = s.ctx.WithBlockTime(converted.VotingEndTime)
			gov.EndBlocker(s.ctx, s.gov)
			final, found := s.gov.GetProposal(s.ctx, proposal.ProposalId)
			require.True(t, found)
			require.Equal(t, tc.status, final.Status)
			require.Equal(t, converted.FinalTallyResult, final.FinalTallyResult)
			if !tc.noStake {
				power := sdk.TokensFromConsensusPower(16, sdk.DefaultPowerReduction).ToDec()
				if tc.delegatorOnly {
					power = sdk.TokensFromConsensusPower(6, sdk.DefaultPowerReduction).ToDec()
				}
				counts := map[govtypes.VoteOption]sdk.Int{
					govtypes.OptionYes: final.FinalTallyResult.Yes, govtypes.OptionAbstain: final.FinalTallyResult.Abstain,
					govtypes.OptionNo: final.FinalTallyResult.No, govtypes.OptionNoWithVeto: final.FinalTallyResult.NoWithVeto,
				}
				for _, option := range tc.options {
					require.True(t, power.Mul(option.Weight).TruncateInt().Equal(counts[option.Option]))
				}
			}
			require.Empty(t, s.gov.GetVotes(s.ctx, proposal.ProposalId))
			require.Empty(t, s.gov.GetDeposits(s.ctx, proposal.ProposalId))
			expectedBalance := balance
			if !tc.burn {
				expectedBalance = balance.Add(proposal.TotalDeposit...)
			}
			require.Equal(t, expectedBalance, s.bank.GetAllBalances(s.ctx, addrs[0]))
			iter = s.gov.ActiveProposalQueueIterator(s.ctx, converted.VotingEndTime)
			require.False(t, iter.Valid())
			require.NoError(t, iter.Close())
			s.commitAndClose(t)
			s = openRepair305State(t, dir, converted.VotingEndTime.Add(time.Second))
			require.Empty(t, s.gov.GetVotes(s.ctx, proposal.ProposalId))
			persisted, found := s.gov.GetProposal(s.ctx, proposal.ProposalId)
			require.True(t, found)
			require.Equal(t, final, persisted)
			require.NoError(t, s.cms.Close())
		})
	}
}

func TestRepair_305_ExpeditedPassDeletesVotes(t *testing.T) {
	s := openRepair305State(t, t.TempDir(), time.Unix(1700000000, 0).UTC())
	defer func() { require.NoError(t, s.cms.Close()) }()
	addrs := s.electorate(t, true)
	proposal := s.submit(t, addrs[0])
	for _, voter := range addrs {
		_, err := govkeeper.NewMsgServerImpl(s.gov).Vote(sdk.WrapSDKContext(s.ctx), govtypes.NewMsgVote(voter, proposal.ProposalId, govtypes.OptionYes))
		require.NoError(t, err)
	}
	s.ctx = s.ctx.WithBlockTime(proposal.VotingEndTime)
	gov.EndBlocker(s.ctx, s.gov)
	final, found := s.gov.GetProposal(s.ctx, proposal.ProposalId)
	require.True(t, found)
	require.Equal(t, govtypes.StatusPassed, final.Status)
	require.True(t, final.IsExpedited)
	require.True(t, sdk.TokensFromConsensusPower(16, sdk.DefaultPowerReduction).Equal(final.FinalTallyResult.Yes))
	require.Empty(t, s.gov.GetVotes(s.ctx, proposal.ProposalId))
	require.Empty(t, s.gov.GetDeposits(s.ctx, proposal.ProposalId))
}
