package keeper

import sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"

type Migrator struct {
	keeper *Keeper
}

func NewMigrator(k *Keeper) Migrator {
	return Migrator{keeper: k}
}

func (m Migrator) Migrate1to2(ctx sdk.Context) error {
	m.keeper.syncExitCheckpoint(ctx)
	return nil
}
