package simulation

// DONTCOVER

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	simtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/simulation"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types/proposal"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/simulation"
)

// ParamChanges defines the parameters that can be modified by param change proposals
// on the simulation. Timing changes require current state; use ParamChangeProposalContent
// for period/window updates because this legacy interface has no context.
func ParamChanges(_ *rand.Rand) []simtypes.ParamChange {
	return []simtypes.ParamChange{
		simulation.NewSimParamChange(types.ModuleName, string(types.KeyVoteThreshold),
			func(r *rand.Rand) string {
				return fmt.Sprintf("\"%s\"", GenVoteThreshold(r))
			},
		),
		simulation.NewSimParamChange(types.ModuleName, string(types.KeyRewardBand),
			func(r *rand.Rand) string {
				return fmt.Sprintf("\"%s\"", GenRewardBand(r))
			},
		),
		simulation.NewSimParamChange(types.ModuleName, string(types.KeySlashFraction),
			func(r *rand.Rand) string {
				return fmt.Sprintf("\"%s\"", GenSlashFraction(r))
			},
		),
	}
}

// ParamChangesForParams generates changes that remain valid in any subset and order.
// Every proposed period divides the old window, and every proposed window is a
// multiple of both old and new periods, so selecting just one timing field is safe.
func ParamChangesForParams(r *rand.Rand, current types.Params) []simtypes.ParamChange {
	if err := current.Validate(); err != nil {
		panic(fmt.Errorf("invalid current oracle simulation parameters: %w", err))
	}
	periods := make([]uint64, 0, 100)
	for period := uint64(1); period <= 100; period++ {
		if current.SlashWindow%period == 0 {
			periods = append(periods, period)
		}
	}
	period := periods[r.Intn(len(periods))]
	a, b := current.VotePeriod, period
	for b != 0 {
		a, b = b, a%b
	}
	// Both periods divide the current window, so their least common multiple
	// cannot overflow and accommodates either field being applied first.
	step := current.VotePeriod / a * period
	maxMultiple := uint64(100000)
	if limit := uint64(math.MaxUint64) / step; limit < maxMultiple {
		maxMultiple = limit
	}
	window := step * uint64(1+r.Intn(int(maxMultiple)))
	next := current
	next.VotePeriod, next.SlashWindow = period, window
	if err := next.Validate(); err != nil {
		panic(fmt.Errorf("invalid proposed oracle simulation parameters: %w", err))
	}
	return append(ParamChanges(r),
		simulation.NewSimParamChange(types.ModuleName, string(types.KeyVotePeriod),
			func(_ *rand.Rand) string { return fmt.Sprintf("\"%d\"", period) }),
		simulation.NewSimParamChange(types.ModuleName, string(types.KeySlashWindow),
			func(_ *rand.Rand) string { return fmt.Sprintf("\"%d\"", window) }),
	)
}

// ParamChangeProposalContent reads the keeper at proposal generation time rather
// than freezing the genesis parameters when the simulation pool is registered.
func ParamChangeProposalContent(k keeper.Keeper) simtypes.ContentSimulatorFn {
	return func(r *rand.Rand, ctx sdk.Context, _ []simtypes.Account) simtypes.Content {
		pool := ParamChangesForParams(r, k.GetParams(ctx))
		count := 1 + r.Intn(len(pool))
		changes := make([]proposal.ParamChange, 0, count)
		for _, index := range r.Perm(len(pool))[:count] {
			change := pool[index]
			changes = append(changes, proposal.NewParamChange(change.Subspace(), change.Key(), change.SimValue()(r)))
		}
		return proposal.NewParameterChangeProposal("Oracle simulation parameters",
			"Update oracle parameters using the current voting configuration", changes, false)
	}
}
