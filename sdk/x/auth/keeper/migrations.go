package keeper

import (
	"github.com/gogo/protobuf/grpc"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

// Migrator is a struct for handling in-place store migrations.
type Migrator struct {
	keeper      AccountKeeper
	queryServer grpc.Server
}

// NewMigrator returns a new Migrator.
func NewMigrator(keeper AccountKeeper, queryServer grpc.Server) Migrator {
	return Migrator{keeper: keeper, queryServer: queryServer}
}

// Migrate1to2 migrates from version 1 to 2.
func (m Migrator) Migrate1to2(ctx sdk.Context) error {
	var iterErr error

	m.keeper.IterateAccounts(ctx, func(account types.AccountI) (stop bool) {
		return false
	})

	return iterErr
}

func (m Migrator) Migrate2to3(ctx sdk.Context) error {
	params := types.Params{DisableSeqnoCheck: types.DefaultParams().DisableSeqnoCheck}
	subspace := m.keeper.paramSubspace
	subspace.Get(ctx, types.KeyMaxMemoCharacters, &params.MaxMemoCharacters)
	subspace.Get(ctx, types.KeyTxSigLimit, &params.TxSigLimit)
	subspace.Get(ctx, types.KeyTxSizeCostPerByte, &params.TxSizeCostPerByte)
	subspace.Get(ctx, types.KeySigVerifyCostED25519, &params.SigVerifyCostED25519)
	subspace.Get(ctx, types.KeySigVerifyCostSecp256k1, &params.SigVerifyCostSecp256k1)
	m.keeper.SetParams(ctx, params)
	return nil
}
