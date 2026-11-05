package pricefeed_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestAppModuleValidateGenesisRunsKeeperChecks(t *testing.T) {
	basic := pricefeed.AppModuleBasic{}
	genesis := types.DefaultGenesis()
	genesis.Observations = []types.GenesisObservation{
		{
			MarketID: genesis.Params.Markets[0].ID,
			Operator: sdk.ValAddress([]byte("pricefeed-operator-a")),
			Observation: types.ObservationRecord{
				Sequence:          1,
				IndexPrice:        sdk.NewIntWithDecimal(100, 18),
				MarkPrice:         sdk.NewIntWithDecimal(101, 18),
				SourceTimestampMs: 1_700_000_000_000,
				Height:            1,
			},
		},
	}

	encoded, err := json.Marshal(genesis)
	require.NoError(t, err)
	require.NoError(t, keeper.ValidateGenesis(*genesis))
	require.NoError(t, basic.ValidateGenesis(nil, nil, encoded))

	unlistedMarket := uint64(len(genesis.Params.Markets))
	genesis.Observations[0].MarketID = unlistedMarket
	encoded, err = json.Marshal(genesis)
	require.NoError(t, err)
	keeperErr := keeper.ValidateGenesis(*genesis)
	require.ErrorIs(t, keeperErr, types.ErrInvalidGenesis)
	err = basic.ValidateGenesis(nil, nil, encoded)
	require.ErrorIs(t, err, types.ErrInvalidGenesis)
	require.ErrorContains(t, err, fmt.Sprintf("observation of unlisted market %d", unlistedMarket))
	require.EqualError(t, err, keeperErr.Error())

	genesis.Observations[0].MarketID = genesis.Params.Markets[0].ID
	genesis.Aggregates = []types.GenesisAggregate{
		{
			MarketID: 40,
			Aggregate: types.Aggregate{
				IndexPrice: sdk.NewIntWithDecimal(100, 18),
				MarkPrice:  sdk.NewIntWithDecimal(101, 18),
				Basis:      sdk.NewIntWithDecimal(1, 18),
				Sources:    1,
			},
		},
	}
	encoded, err = json.Marshal(genesis)
	require.NoError(t, err)
	err = basic.ValidateGenesis(nil, nil, encoded)
	require.ErrorIs(t, err, types.ErrInvalidGenesis)
	require.ErrorContains(t, err, "aggregate of unlisted market 40")

	require.ErrorIs(t, basic.ValidateGenesis(nil, nil, json.RawMessage(`{"params":`)), types.ErrInvalidGenesis)
	require.Equal(t, uint64(1), (pricefeed.AppModule{}).ConsensusVersion())
}
