package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xbank/types"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

const gigaGenesisUnsupported = "Giga bank genesis is unsupported; use the canonical bank keeper for initialization and recovery"

// InitGenesis refuses initialization: canonical bank genesis owns the shared state.
func (k BaseKeeper) InitGenesis(ctx sdk.Context, genState *types.GenesisState) {
	panic(gigaGenesisUnsupported)
}

// ExportGenesis refuses export: canonical bank genesis owns the shared state.
func (k BaseKeeper) ExportGenesis(ctx sdk.Context) *types.GenesisState {
	panic(gigaGenesisUnsupported)
}
