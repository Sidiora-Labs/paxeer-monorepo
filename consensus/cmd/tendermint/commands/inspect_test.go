package commands

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
)

func TestInspectRefusesUnderAutobahn(t *testing.T) {
	cfg := config.TestConfig().SetRoot(t.TempDir())
	cfg.DBBackend = "goleveldb"
	cfg.AutobahnConfigFile = "autobahn.json"
	cmd := MakeInspectCommand(cfg)
	cmd.SetContext(t.Context())
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.ErrorIs(t, err, ErrAutobahnOfflineStore)
	require.NoDirExists(t, cfg.DBDir())
}
