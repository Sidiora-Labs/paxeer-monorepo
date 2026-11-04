package keeper_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestRepairM_657_RegisteredMigrationPreservesGovernance(t *testing.T) {
	testMigrationM657Registered(t)
}

func TestRepairM_657_MigrationRecovery(t *testing.T) {
	db := dbm.NewMemDB()
	fixture := newMigrationM657Fixture(t, db)
	want := seedMigrationM657LegacyParams(t, fixture)
	migrator := keeper.NewMigrator(fixture.keeper, fixture.config.QueryServer())
	require.NoError(t, migrator.Migrate2to3(fixture.ctx))
	assertMigrationM657Params(t, fixture, want)
	commit := fixture.store.Commit(true)
	require.Equal(t, int64(1), commit.Version)
	require.NoError(t, fixture.store.Close())

	reopened := newMigrationM657Fixture(t, db)
	t.Cleanup(func() { require.NoError(t, reopened.store.Close()) })
	require.Equal(t, commit, reopened.store.LastCommitID())
	assertMigrationM657Params(t, reopened, want)
	migrator = keeper.NewMigrator(reopened.keeper, reopened.config.QueryServer())
	require.NoError(t, migrator.Migrate2to3(reopened.ctx))
	assertMigrationM657Params(t, reopened, want)
	require.Equal(t, int64(2), reopened.store.Commit(true).Version)
}
