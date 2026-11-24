package simulation_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
)

func newMintSimulationState(seed int64, timestamp time.Time) *module.SimulationState {
	registry := cdctypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	return &module.SimulationState{
		Cdc:          codec.NewProtoCodec(registry),
		Rand:         rand.New(rand.NewSource(seed)),
		GenState:     make(map[string]json.RawMessage),
		GenTimestamp: timestamp,
	}
}

func generateMintSimulationGenesis(t *testing.T, state *module.SimulationState) types.GenesisState {
	t.Helper()
	appModule := mint.AppModule{}
	appModule.GenerateGenesisState(state)
	raw, ok := state.GenState[types.ModuleName]
	require.True(t, ok, "the registered generator must publish mint genesis")
	require.NoError(t, appModule.ValidateGenesis(state.Cdc, nil, raw))
	var genesis types.GenesisState
	require.NoError(t, state.Cdc.UnmarshalAsJSON(raw, &genesis))
	require.NoError(t, types.ValidateGenesis(genesis))
	return genesis
}

func TestNonOverlappingGenesis(t *testing.T) {
	timestamps := []time.Time{
		{},
		time.Date(2000, time.February, 29, 23, 59, 59, 0, time.UTC),
		time.Date(2030, time.January, 1, 0, 30, 0, 0, time.FixedZone("east", 14*60*60)),
	}
	for _, seed := range []int64{0, 1, 573, 1<<63 - 1} {
		for index, timestamp := range timestamps {
			t.Run(fmt.Sprintf("seed_%d/time_%d", seed, index), func(t *testing.T) {
				state := newMintSimulationState(seed, timestamp)
				genesis := generateMintSimulationGenesis(t, state)
				schedule := genesis.Params.TokenReleaseSchedule
				require.Len(t, schedule, 10)
				require.Equal(t, types.InitialMinter(), genesis.Minter)
				firstStart, err := time.Parse(types.TokenReleaseDateFormat, schedule[0].StartDate)
				require.NoError(t, err)
				require.Equal(t, timestamp.UTC().AddDate(1, 0, 0).Format(types.TokenReleaseDateFormat), schedule[0].StartDate)
				require.False(t, firstStart.Before(timestamp.UTC()), "releases must begin after genesis")
				previousEnd := time.Time{}
				for _, release := range schedule {
					start, err := time.Parse(types.TokenReleaseDateFormat, release.StartDate)
					require.NoError(t, err)
					end, err := time.Parse(types.TokenReleaseDateFormat, release.EndDate)
					require.NoError(t, err)
					require.True(t, end.After(start), "each period must have a positive duration")
					require.True(t, start.After(previousEnd), "successive periods must not share a day")
					previousEnd = end
				}
			})
		}
	}
}

func TestReproducibleSeedAndGenesisTime(t *testing.T) {
	timestamp := time.Date(2040, time.January, 15, 3, 4, 5, 0, time.UTC)
	first := newMintSimulationState(573, timestamp)
	firstGenesis := generateMintSimulationGenesis(t, first)
	second := newMintSimulationState(573, timestamp)
	generateMintSimulationGenesis(t, second)
	require.Equal(t, first.GenState[types.ModuleName], second.GenState[types.ModuleName], "identical simulation inputs must produce identical JSON")

	reference := rand.New(rand.NewSource(573))
	provision := uint64(reference.Int63n(1000000))
	require.Equal(t, provision, firstGenesis.Params.TokenReleaseSchedule[0].TokenReleaseAmount)
	require.Equal(t, reference.Int63(), first.Rand.Int63(), "generation must consume the simulation's random source")

	differentSeed := generateMintSimulationGenesis(t, newMintSimulationState(574, timestamp))
	require.NotEqual(t, firstGenesis.Params.TokenReleaseSchedule[0].TokenReleaseAmount, differentSeed.Params.TokenReleaseSchedule[0].TokenReleaseAmount)
	for index, release := range firstGenesis.Params.TokenReleaseSchedule {
		require.Equal(t, release.StartDate, differentSeed.Params.TokenReleaseSchedule[index].StartDate)
		require.Equal(t, release.EndDate, differentSeed.Params.TokenReleaseSchedule[index].EndDate)
	}

	shifted := generateMintSimulationGenesis(t, newMintSimulationState(573, timestamp.AddDate(0, 0, 7)))
	for index, release := range firstGenesis.Params.TokenReleaseSchedule {
		start, err := time.Parse(types.TokenReleaseDateFormat, release.StartDate)
		require.NoError(t, err)
		end, err := time.Parse(types.TokenReleaseDateFormat, release.EndDate)
		require.NoError(t, err)
		require.Equal(t, start.AddDate(0, 0, 7).Format(types.TokenReleaseDateFormat), shifted.Params.TokenReleaseSchedule[index].StartDate)
		require.Equal(t, end.AddDate(0, 0, 7).Format(types.TokenReleaseDateFormat), shifted.Params.TokenReleaseSchedule[index].EndDate)
		require.Equal(t, release.TokenReleaseAmount, shifted.Params.TokenReleaseSchedule[index].TokenReleaseAmount)
	}

	sameInstant := newMintSimulationState(573, timestamp.In(time.FixedZone("west", -12*60*60)))
	generateMintSimulationGenesis(t, sameInstant)
	require.Equal(t, first.GenState[types.ModuleName], sameInstant.GenState[types.ModuleName])
}

func TestGenesisValidationBeforeSerialization(t *testing.T) {
	state := newMintSimulationState(573, time.Date(9999, time.January, 1, 0, 0, 0, 0, time.UTC))
	previous := json.RawMessage(`{"previous":"genesis"}`)
	state.GenState[types.ModuleName] = previous
	defer func() {
		err, ok := recover().(error)
		require.True(t, ok, "invalid generated dates must panic with a validation error")
		require.ErrorContains(t, err, "invalid mint simulation genesis")
		require.ErrorContains(t, err, "invalid start date format")
		require.Equal(t, previous, state.GenState[types.ModuleName], "validation failure must not publish invalid JSON")
	}()
	mint.AppModule{}.GenerateGenesisState(state)
}
