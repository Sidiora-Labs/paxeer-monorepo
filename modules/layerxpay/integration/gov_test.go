package integration_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	layerxgovtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	pay "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay"
	paykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/keeper"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func TestPayParamsThroughLayerXProposal(t *testing.T) {
	payKey := sdk.NewKVStoreKey(paytypes.StoreKey)
	hostKey := sdk.NewKVStoreKey("lxphost")
	w := app.NewTestWrapper(t, time.Unix(1800000000, 0).UTC(), ed25519.GenPrivKey().PubKey(), false,
		func(base *baseapp.BaseApp) {
			base.MountStores(payKey, hostKey)
		})
	ctx := w.Ctx
	registry := w.App.InterfaceRegistry()
	paytypes.RegisterInterfaces(registry)
	host, err := lxphost.NewHost(lxphost.Config{
		StoreKey: hostKey, NetworkID: 7, ParameterVersion: 1,
		TimestampWindowMs: 60000, GasLimit: 1000000,
	})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	keeper, err := paykeeper.NewKeeper(w.App.AppCodec(), payKey, hostKey, host,
		&w.App.EvmKeeper, w.App.LayerXCustodyKeeper, layerxgovtypes.GovernanceAuthority())
	require.NoError(t, err)
	queryRouter := baseapp.NewQueryServerTestHelper(ctx, registry)
	pay.NewAppModule(keeper).RegisterServices(module.NewConfigurator(w.App.AppCodec(), w.App.MsgServiceRouter(), queryRouter))
	query := paytypes.NewQueryClient(queryRouter)
	assertParams := func(expected paytypes.Params) {
		t.Helper()
		queryRouter.Ctx = ctx
		response, err := query.Params(context.Background(), &paytypes.QueryParamsRequest{})
		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, expected, response.Params)
	}
	gateKey, err := paytypes.KernelModuleEnableKey(paytypes.EscrowModuleID)
	require.NoError(t, err)
	gateRecord := make([]byte, 32)
	gateRecord[31] = 1
	genesis := paytypes.DefaultGenesis()
	genesis.KernelRecords = []kv.Pair{{Key: gateKey, Value: gateRecord}}
	require.NoError(t, keeper.InitGenesis(ctx, *genesis))
	before := paytypes.DefaultParams()
	assertParams(before)

	validators := w.App.StakingKeeper.GetAllValidators(ctx)
	require.Len(t, validators, 1)
	validator := validators[0]
	voter := sdk.AccAddress(validator.GetOperator())
	bond := w.App.StakingKeeper.TokensFromConsensusPower(ctx, 10)
	w.FundAcc(voter, sdk.NewCoins(sdk.NewCoin(w.App.StakingKeeper.BondDenom(ctx), bond)))
	_, err = w.App.StakingKeeper.Delegate(ctx, voter, bond, stakingtypes.Unbonded, validator, true)
	require.NoError(t, err)
	_, err = w.App.StakingKeeper.ApplyAndReturnValidatorSetUpdates(ctx)
	require.NoError(t, err)
	validator, found := w.App.StakingKeeper.GetValidator(ctx, validator.GetOperator())
	require.True(t, found)
	require.True(t, validator.IsBonded())
	require.True(t, w.App.StakingKeeper.TotalBondedTokens(ctx).IsPositive())

	params := before
	params.Paused = false
	params.EscrowEnabled = true
	update := &paytypes.MsgUpdateParams{Authority: layerxgovtypes.GovernanceAuthority(), Params: params}
	proposal, err := layerxgovtypes.NewLayerXProposal("Enable pay escrow", "Unpause pay and enable escrow", update)
	require.NoError(t, err)
	require.Equal(t, layerxgovtypes.ProposalTypeLayerX, proposal.ProposalType())
	deposit := w.App.GovKeeper.GetDepositParams(ctx).MinDeposit
	w.FundAcc(voter, deposit)
	submit, err := govtypes.NewMsgSubmitProposal(proposal, deposit, voter)
	require.NoError(t, err)
	amino, err := govtypes.ModuleCdc.MarshalAsJSON(submit)
	require.NoError(t, err)
	require.Contains(t, string(amino), `"layerxpay/MsgUpdateParams"`)
	encoded, err := w.App.AppCodec().Marshal(submit)
	require.NoError(t, err)
	var decoded govtypes.MsgSubmitProposal
	require.NoError(t, w.App.AppCodec().Unmarshal(encoded, &decoded))
	require.NoError(t, decoded.ValidateBasic())
	proposalID, err := w.App.GovKeeper.GetProposalID(ctx)
	require.NoError(t, err)
	submitHandler := w.App.MsgServiceRouter().Handler(&decoded)
	require.NotNil(t, submitHandler)
	result, err := submitHandler(ctx, &decoded)
	require.NoError(t, err)
	require.NotNil(t, result)
	stored, found := w.App.GovKeeper.GetProposal(ctx, proposalID)
	require.True(t, found)
	require.Equal(t, govtypes.StatusVotingPeriod, stored.Status)
	require.Equal(t, layerxgovtypes.ProposalTypeLayerX, stored.ProposalType())
	assertParams(before)

	vote := govtypes.NewMsgVote(voter, proposalID, govtypes.OptionYes)
	voteHandler := w.App.MsgServiceRouter().Handler(vote)
	require.NotNil(t, voteHandler)
	_, err = voteHandler(ctx, vote)
	require.NoError(t, err)
	storedVote, found := w.App.GovKeeper.GetVote(ctx, proposalID, voter)
	require.True(t, found)
	require.Equal(t, govtypes.NewNonSplitVoteOption(govtypes.OptionYes), govtypes.WeightedVoteOptions(storedVote.Options))
	assertParams(before)
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1).WithBlockTime(stored.VotingEndTime.Add(time.Second))
	gov.EndBlocker(ctx, w.App.GovKeeper)
	stored, found = w.App.GovKeeper.GetProposal(ctx, proposalID)
	require.True(t, found)
	require.Equal(t, govtypes.StatusPassed, stored.Status)
	require.Equal(t, w.App.StakingKeeper.TotalBondedTokens(ctx), stored.FinalTallyResult.Yes)
	require.True(t, stored.FinalTallyResult.No.IsZero())
	require.True(t, stored.FinalTallyResult.NoWithVeto.IsZero())
	require.True(t, stored.FinalTallyResult.Abstain.IsZero())
	assertParams(params)

	outsider := sdk.AccAddress(bytes.Repeat([]byte{42}, 20))
	forged := &paytypes.MsgUpdateParams{Authority: outsider.String(), Params: before}
	require.NoError(t, forged.ValidateBasic())
	forgedProposal, err := layerxgovtypes.NewLayerXProposal("Unauthorized pay update", "Refuse an unknown authority", forged)
	require.NoError(t, err)
	require.ErrorIs(t, forgedProposal.ValidateBasic(), govtypes.ErrInvalidProposalContent)
	forgedSubmit, err := govtypes.NewMsgSubmitProposal(forgedProposal, deposit, voter)
	require.NoError(t, err)
	nextID, err := w.App.GovKeeper.GetProposalID(ctx)
	require.NoError(t, err)
	result, err = submitHandler(ctx, forgedSubmit)
	require.ErrorIs(t, err, govtypes.ErrInvalidProposalContent)
	require.Nil(t, result)
	afterID, err := w.App.GovKeeper.GetProposalID(ctx)
	require.NoError(t, err)
	require.Equal(t, nextID, afterID)
	_, found = w.App.GovKeeper.GetProposal(ctx, nextID)
	require.False(t, found)
	assertParams(params)
	updateHandler := w.App.MsgServiceRouter().Handler(forged)
	require.NotNil(t, updateHandler)
	result, err = updateHandler(ctx, forged)
	require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
	require.Nil(t, result)
	assertParams(params)
	require.Equal(t, gateRecord, ctx.KVStore(hostKey).Get(gateKey))
}
