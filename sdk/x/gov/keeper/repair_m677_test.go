package keeper_test

import (
	"context"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	paramproposal "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types/proposal"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repairM677FailedProposal(t *testing.T, expedited bool) (sdk.Context, keeper.Keeper, types.QueryClient, *codec.LegacyAmino, types.Proposal) {
	t.Helper()
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	cms := store.NewCommitMultiStore(db)
	keys := sdk.NewKVStoreKeys(types.StoreKey, authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey, paramtypes.StoreKey)
	for _, key := range keys {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	tkey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	cms.MountStoreWithDB(tkey, sdk.StoreTypeTransient, db)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 1, Time: time.Unix(1700000000, 0).UTC()}, false)

	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	paramproposal.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	types.RegisterLegacyAminoCodec(amino)
	paramproposal.RegisterLegacyAminoCodec(amino)
	pk := paramskeeper.NewKeeper(cdc, amino, keys[paramtypes.StoreKey], tkey)
	ak := authkeeper.NewAccountKeeper(cdc, keys[authtypes.StoreKey], pk.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, map[string][]string{
		types.ModuleName:               {},
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
	})
	bk := bankkeeper.NewBaseKeeper(cdc, keys[banktypes.StoreKey], ak, pk.Subspace(banktypes.ModuleName), map[string]bool{})
	sk := stakingkeeper.NewKeeper(cdc, keys[stakingtypes.StoreKey], ak, bk, pk.Subspace(stakingtypes.ModuleName))
	sk.SetParams(ctx, stakingtypes.DefaultParams())
	tokens := sdk.NewInt(10000000)
	bankGenesis := banktypes.DefaultGenesisState()
	bankGenesis.Balances = []banktypes.Balance{{
		Address: ak.GetModuleAddress(stakingtypes.BondedPoolName).String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(sk.BondDenom(ctx), tokens)),
	}}
	bk.InitGenesis(ctx, bankGenesis)
	pubkey := ed25519.GenPrivKeyFromSecret([]byte("repair-m677-validator")).PubKey()
	voter := sdk.AccAddress(pubkey.Address())
	validator, err := stakingtypes.NewValidator(sdk.ValAddress(voter), pubkey, stakingtypes.Description{})
	require.NoError(t, err)
	validator.Status = stakingtypes.Bonded
	validator.Tokens = tokens
	validator.DelegatorShares = tokens.ToDec()
	sk.SetValidator(ctx, validator)
	sk.SetValidatorByPowerIndex(ctx, validator)

	router := types.NewRouter().AddRoute(paramproposal.RouterKey, params.NewParamChangeProposalHandler(pk))
	k := keeper.NewKeeper(cdc, keys[types.StoreKey], pk.Subspace(types.ModuleName).WithKeyTable(types.ParamKeyTable()), ak, bk, sk, pk, router)
	k.SetProposalID(ctx, 1)
	k.SetDepositParams(ctx, types.DefaultDepositParams())
	k.SetVotingParams(ctx, types.DefaultVotingParams())
	k.SetTallyParams(ctx, types.DefaultTallyParams())
	content := paramproposal.NewParameterChangeProposal("Invalid validator limit", "Execution must reject a zero validator limit", []paramproposal.ParamChange{
		paramproposal.NewParamChange(stakingtypes.ModuleName, string(stakingtypes.KeyMaxValidators), "0"),
	}, expedited)
	require.NoError(t, content.ValidateBasic())
	proposal, err := k.SubmitProposalWithExpedite(ctx, content, expedited)
	require.NoError(t, err)
	k.ActivateVotingPeriod(ctx, proposal)
	require.NoError(t, k.AddVote(ctx, proposal.ProposalId, voter, types.WeightedVoteOptions{
		{Option: types.OptionYes, Weight: sdk.NewDecWithPrec(7, 1)},
		{Option: types.OptionAbstain, Weight: sdk.NewDecWithPrec(1, 1)},
		{Option: types.OptionNo, Weight: sdk.NewDecWithPrec(1, 1)},
		{Option: types.OptionNoWithVeto, Weight: sdk.NewDecWithPrec(1, 1)},
	}))
	require.Len(t, k.GetVotes(ctx, proposal.ProposalId), 1)
	proposal, found := k.GetProposal(ctx, proposal.ProposalId)
	require.True(t, found)
	ctx = ctx.WithBlockTime(proposal.VotingEndTime)
	gov.EndBlocker(ctx, k)
	proposal, found = k.GetProposal(ctx, proposal.ProposalId)
	require.True(t, found)
	require.Equal(t, types.StatusFailed, proposal.Status)
	require.Equal(t, types.NewTallyResult(sdk.NewInt(7000000), sdk.NewInt(1000000), sdk.NewInt(1000000), sdk.NewInt(1000000)), proposal.FinalTallyResult)
	require.Empty(t, k.GetVotes(ctx, proposal.ProposalId))
	require.Equal(t, stakingtypes.DefaultParams().MaxValidators, sk.MaxValidators(ctx))
	queryHelper := baseapp.NewQueryServerTestHelper(ctx, registry)
	types.RegisterQueryServer(queryHelper, k)
	return ctx, k, types.NewQueryClient(queryHelper), amino, proposal
}

func TestRepairM_677_FailedProposalTally(t *testing.T) {
	for _, expedited := range []bool{false, true} {
		name := "regular"
		if expedited {
			name = "expedited"
		}
		t.Run(name, func(t *testing.T) {
			ctx, k, client, _, proposal := repairM677FailedProposal(t, expedited)
			for i := 0; i < 2; i++ {
				response, err := client.TallyResult(context.Background(), &types.QueryTallyResultRequest{ProposalId: proposal.ProposalId})
				require.NoError(t, err)
				require.Equal(t, proposal.FinalTallyResult, response.Tally)
			}
			stored, found := k.GetProposal(ctx, proposal.ProposalId)
			require.True(t, found)
			require.Equal(t, proposal, stored)
		})
	}
}

func TestRepairM_677_FailedProposalLegacyTally(t *testing.T) {
	for _, expedited := range []bool{false, true} {
		name := "regular"
		if expedited {
			name = "expedited"
		}
		t.Run(name, func(t *testing.T) {
			ctx, k, client, amino, proposal := repairM677FailedProposal(t, expedited)
			querier := keeper.NewQuerier(k, amino)
			request := abci.RequestQuery{
				Path: "custom/gov/tally",
				Data: amino.MustMarshalJSON(types.NewQueryProposalParams(proposal.ProposalId)),
			}
			for i := 0; i < 2; i++ {
				bz, err := querier(ctx, []string{types.QueryTally}, request)
				require.NoError(t, err)
				var tally types.TallyResult
				require.NoError(t, amino.UnmarshalAsJSON(bz, &tally))
				require.Equal(t, proposal.FinalTallyResult, tally)
				response, err := client.TallyResult(context.Background(), &types.QueryTallyResultRequest{ProposalId: proposal.ProposalId})
				require.NoError(t, err)
				require.Equal(t, response.Tally, tally)
			}
			stored, found := k.GetProposal(ctx, proposal.ProposalId)
			require.True(t, found)
			require.Equal(t, proposal, stored)
		})
	}
}
