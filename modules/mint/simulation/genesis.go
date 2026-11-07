package simulation

// DONTCOVER

import (
	"encoding/json"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
)

// RandomizedGenState generates a random GenesisState for mint.
func RandomizedGenState(simState *module.SimulationState) {
	mintDenom := sdk.DefaultBondDenom
	randomProvision := uint64(simState.Rand.Int63n(1000000)) //nolint:gosec
	startDate := simState.GenTimestamp.UTC().AddDate(1, 0, 0)
	tokenReleaseSchedule := []types.ScheduledTokenRelease{}

	for i := 1; i <= 10; i++ {
		endDate := startDate.AddDate(2, 0, 0)
		scheduledTokenRelease := types.ScheduledTokenRelease{
			StartDate:          startDate.Format(types.TokenReleaseDateFormat),
			EndDate:            endDate.Format(types.TokenReleaseDateFormat),
			TokenReleaseAmount: randomProvision / uint64(i), //nolint:gosec
		}
		tokenReleaseSchedule = append(tokenReleaseSchedule, scheduledTokenRelease)
		startDate = endDate.AddDate(0, 0, 1)
	}

	params := types.NewParams(mintDenom, tokenReleaseSchedule)

	mintGenesis := types.NewGenesisState(types.InitialMinter(), params)
	if err := types.ValidateGenesis(*mintGenesis); err != nil {
		panic(fmt.Errorf("invalid mint simulation genesis: %w", err))
	}

	bz, err := json.MarshalIndent(&mintGenesis, "", " ")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Selected deterministically generated minting parameters:\n%s\n", bz)
	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(mintGenesis)
}
