package types_test

import (
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/launchpadtest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func repairM946Keeper(t *testing.T) (*keeper.Keeper, sdk.Context) {
	t.Helper()
	app := testkeeper.EVMTestApp
	ctx := app.GetContextForDeliverTx(nil).WithBlockHeight(1).WithBlockTime(time.Unix(1_800_000_000, 0))
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	k, ctx := launchpadtest.NewKeeper(app, ctx)
	gs := types.DefaultGenesis()
	for role, address := range keeper.Generation2Surfaces {
		gs.Surfaces = append(gs.Surfaces, types.Surface{Address: address.Hex(), Role: role})
	}
	gs.Treasury = []types.TreasuryBalance{{Token: types.DefaultQuoteToken, Ledger: sdk.ZeroInt(), Held: sdk.NewInt(10)}}
	require.NoError(t, gs.Validate())
	k.InitGenesis(ctx, *gs)
	return k, ctx
}

func TestRepairM_946_RegisteredProducerCodes(t *testing.T) {
	k, ctx := repairM946Keeper(t)
	proofLaunchpadProducerCodes(t, k, ctx)
}

func TestRepairM_946_OpticalHookCode(t *testing.T) {
	k, ctx := repairM946Keeper(t)
	proofLaunchpadHookCode(t, k, ctx)
}

func TestRepairM_946_NativeUpdateParamsRefusal(t *testing.T) {
	k, ctx := repairM946Keeper(t)
	proofLaunchpadUpdateParamsRefusal(t, k, ctx)
}

func TestRepairM_946_GenesisValidationCodes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		corrupt func(*types.GenesisState)
	}{
		{"AirdropEpochs", func(gs *types.GenesisState) { gs.AirdropEpochs = []types.AirdropEpochAmount{{}} }},
		{"AirdropClaims", func(gs *types.GenesisState) { gs.AirdropClaims = []types.AirdropClaim{{}} }},
		{"ProtocolFees", func(gs *types.GenesisState) { gs.ProtocolFees = []types.ProtocolFeeEntry{{}} }},
		{"AirdropBases", func(gs *types.GenesisState) { gs.AirdropBases = []types.AirdropEpochBasis{{}} }},
		{"HoldingHistories", func(gs *types.GenesisState) { gs.HoldingHistories = []types.HoldingHistory{{}} }},
		{"HoldingCheckpoints", func(gs *types.GenesisState) { gs.HoldingCheckpoints = []types.HoldingCheckpoint{{}} }},
		{"DuplicateMarket", func(gs *types.GenesisState) { gs.Markets = []types.Market{nativeMarket(), nativeMarket()} }},
		{"MarketIndexGap", func(gs *types.GenesisState) { m := nativeMarket(); m.Index = 2; gs.Markets = []types.Market{m} }},
		{"InvalidContract", func(gs *types.GenesisState) { gs.Surfaces = []types.Surface{{}} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gs := types.DefaultGenesis()
			require.NoError(t, gs.Validate())
			tc.corrupt(gs)
			proofLaunchpadRegisteredProducer(t, gs.Validate(), types.ErrInvalidGenesis, 24, "")
		})
	}
}
