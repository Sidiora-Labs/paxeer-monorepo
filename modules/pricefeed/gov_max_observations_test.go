package pricefeed_test

import (
	"testing"

	layerxgov "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov"
	layerxgovtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func CheckSubmission(ctx sdk.Context, tx sdk.Tx, k keeper.Keeper) error {
	return pricefeed.CheckSubmission(ctx, tx, k)
}

func TestGovernedMaxObservationsRefusedAtAnte(t *testing.T) {
	k, ctx := anteKeeper(t)
	before := k.GetParams(ctx)
	require.Equal(t, layerxgovtypes.GovernanceAuthority(), before.Authority)
	require.Equal(t, uint32(32), before.MaxObservationsPerSubmission)
	operator, feeder := registerFeeder(t, k, ctx, "governed-max-observations", stakingtypes.Bonded)
	msg := submission(feeder, 5)
	require.Len(t, msg.Observations, 5)
	require.NoError(t, msg.ValidateBasic())
	tx, bz := buildTx(t, msg)
	require.LessOrEqual(t, uint64(len(bz)), uint64(before.MaxSubmissionBytes))
	for _, mode := range []sdk.Context{checkAt(ctx, 49), deliverAt(ctx, 50)} {
		require.NoError(t, CheckSubmission(mode.WithTxBytes(bz), tx, k))
	}

	am := pricefeed.NewAppModule(k)
	registry := cdctypes.NewInterfaceRegistry()
	am.RegisterInterfaces(registry)
	layerxgovtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(registry)
	am.RegisterServices(module.NewConfigurator(cdc, router, baseapp.NewGRPCQueryRouter()))
	require.NotNil(t, router.HandlerByTypeURL(sdk.MsgTypeURL(&types.MsgSetParams{})))

	want := before
	want.MaxObservationsPerSubmission = 4
	proposal, err := layerxgovtypes.NewLayerXProposal(
		"Lower the pricefeed observation bound", "Limit each price submission to four observations",
		&types.MsgSetParams{
			Authority:                    want.Authority,
			Markets:                      want.Markets,
			StalenessWindowMs:            want.StalenessWindowMs,
			MaxClockSkewMs:               want.MaxClockSkewMs,
			MinSources:                   want.MinSources,
			MaxDivergenceBps:             want.MaxDivergenceBps,
			MaxDeviationBps:              want.MaxDeviationBps,
			MaxObservationsPerSubmission: want.MaxObservationsPerSubmission,
			MaxSubmissionBytes:           want.MaxSubmissionBytes,
			MarkEmaTauMs:                 want.MarkEmaTauMs,
			MaxMarkBasisBps:              want.MaxMarkBasisBps,
			MaxPriceAgeMs:                want.MaxPriceAgeMs,
		})
	require.NoError(t, err)
	require.NoError(t, layerxgov.NewProposalHandler(router)(ctx, proposal))
	require.Equal(t, want, k.GetParams(ctx))
	require.Equal(t, uint32(4), k.GetParams(ctx).MaxObservationsPerSubmission)

	boundaryTx, boundaryBz := buildTx(t, submission(feeder, 4))
	for _, mode := range []sdk.Context{checkAt(ctx, 49), deliverAt(ctx, 50)} {
		err := CheckSubmission(mode.WithTxBytes(bz), tx, k)
		requireRefusal(t, err, types.ErrTooManyObservations, "LXP_PRICEFEED_TOO_MANY_OBSERVATIONS")
		require.ErrorContains(t, err, "5 observations, at most 4")
		require.NoError(t, CheckSubmission(mode.WithTxBytes(boundaryBz), boundaryTx, k))
		_, marked := k.GetLastSubmissionHeight(mode, operator)
		require.False(t, marked, "CheckSubmission wrote a submission height")
	}
}
