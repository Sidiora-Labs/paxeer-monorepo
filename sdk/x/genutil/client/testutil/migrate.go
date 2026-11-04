package testutil

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	clitestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil/client/cli"
)

func TestGetMigrationCallback(t *testing.T) {
	versions := cli.GetMigrationVersions()
	require.Equal(t, []string{}, versions, "this fork has no application-registered genesis migrations")
	require.Equal(t, versions, cli.MigrateGenesisCmd().ValidArgs)
	require.Nil(t, cli.GetMigrationCallback("v0.43"), "upstream SDK targets are not registered by this fork")
}

func (s *IntegrationTestSuite) TestMigrateGenesis() {
	val0 := s.network.Validators[0]

	testCases := []struct {
		name      string
		genesis   string
		target    string
		expErrMsg string
	}{
		{
			"migrate 0.37 to 0.42",
			v037Exported,
			"v0.42",
			"Make sure that you have correctly migrated all Tendermint consensus params",
		},
		{
			"unregistered upstream weighted-vote migration",
			v040Valid,
			"v0.43",
			"unknown migration function for version: v0.43; supported target versions: none",
		},
	}

	for _, tc := range testCases {
		tc := tc
		s.Run(tc.name, func() {
			genesisFile := testutil.WriteToNewTempFile(s.T(), tc.genesis)
			jsonOutput, err := clitestutil.ExecTestCLICmd(val0.ClientCtx, cli.MigrateGenesisCmd(), []string{tc.target, genesisFile.Name()})
			s.Require().Error(err)
			s.Require().Contains(err.Error(), tc.expErrMsg)
			s.Require().NotContains(jsonOutput.String(), "\"app_state\"", "rejected input must not be emitted as migrated genesis")
		})
	}
}
