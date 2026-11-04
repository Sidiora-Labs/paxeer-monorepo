package types_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdktestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestRepairM_364_GenesisPositiveFixtureShape(t *testing.T) {
	t.Run("matched_fill_export_import_reopen", TestGenesisValidateAccepts)
	t.Run("independent_corruption_before_import", TestGenesisValidateRejectsProducedState)
	t.Run("retained_malformed_references", TestGenesisValidateRejects)
	t.Run("record_and_json_codecs", TestGenesisValidateRecordsRoundTrip)
}

func TestRepairM_364_ErrorProofPinsLoss(t *testing.T) {
	t.Run("updated_proof", TestProofPerpsErrorsOutsideScope)
	key := sdk.NewKVStoreKey(types.StoreKey)
	ctx := sdktestutil.DefaultContext(key, sdk.NewTransientStoreKey("repair_m364_errors"))
	k := keeper.NewKeeper(key, nil, nil)
	_, unknownMarket := k.GetMarket(ctx, 987)
	genesis := types.DefaultGenesis()
	genesis.Params.CollateralToken = common.Address{}
	invalidGenesis := types.ValidateGenesis(*genesis)
	for _, tc := range []struct {
		name     string
		produced error
		root     *sdkerrors.Error
		code     uint32
	}{
		{"unknown_market", unknownMarket, types.ErrUnknownMarket, 3},
		{"invalid_genesis", invalidGenesis, types.ErrInvalidGenesis, 39},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.code, tc.root.ABCICode())
			proofPerpsTransport(t, tc.produced, tc.root)
		})
	}
}
