package commands

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRollbackRefusesUnderAutobahn(t *testing.T) {
	cfg := setupAutobahnOfflineStoreConfig(t)

	height, hash, err := RollbackStateToTargetHeight(cfg, true, 1)
	require.ErrorIs(t, err, ErrAutobahnOfflineStore)
	require.EqualError(t, err, "rollback: "+ErrAutobahnOfflineStore.Error())
	require.Equal(t, int64(-1), height)
	require.Nil(t, hash)
}

func TestLoadTendermintStateRefusesUnderAutobahn(t *testing.T) {
	cfg := setupAutobahnOfflineStoreConfig(t)

	tmState, err := LoadTendermintState(cfg)
	require.ErrorIs(t, err, ErrAutobahnOfflineStore)
	require.EqualError(t, err, "load tendermint state: "+ErrAutobahnOfflineStore.Error())
	require.True(t, tmState.IsEmpty())
}
