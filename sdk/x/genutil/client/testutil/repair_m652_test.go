package testutil

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	clitestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil/client/cli"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
)

func repairM652ClientContext(t *testing.T) client.Context {
	t.Helper()
	enc := paxapp.MakeEncodingConfig()
	return client.Context{}.WithCodec(enc.Marshaler).
		WithLegacyAmino(enc.Amino).WithInterfaceRegistry(enc.InterfaceRegistry).
		WithTxConfig(enc.TxConfig).WithHomeDir(t.TempDir())
}

func TestRepairM_652_RegisteredTargets(t *testing.T) {
	TestGetMigrationCallback(t)
	out, err := clitestutil.ExecTestCLICmd(repairM652ClientContext(t), cli.MigrateGenesisCmd(), []string{"--help"})
	require.NoError(t, err)
	require.Contains(t, out.String(), "Supported target versions: none (this application has not registered any genesis migrations)")
}

func TestRepairM_652_WeightedVoteMigration(t *testing.T) {
	clientCtx := repairM652ClientContext(t)
	genesisFile := testutil.WriteToNewTempFile(t, v040Valid)
	// The committed legacy vote fixture is valid input, but this fork does not
	// register the upstream conversion to weighted votes.
	doc, err := tmtypes.GenesisDocFromFile(genesisFile.Name())
	require.NoError(t, err)
	require.Equal(t, "test", doc.ChainID)
	var appState map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(doc.AppState, &appState))
	require.Contains(t, appState, govtypes.ModuleName)
	var govState govtypes.GenesisState
	require.NoError(t, clientCtx.Codec.UnmarshalAsJSON(appState[govtypes.ModuleName], &govState))
	require.Len(t, govState.Votes, 1)
	vote := govState.Votes[0]
	require.Equal(t, uint64(5), vote.ProposalId)
	require.Equal(t, "cosmos1fl48vsnmsdzcv85q5d2q4z5ajdha8yu34mf0eh", vote.Voter)
	require.Equal(t, govtypes.OptionYes, vote.Option)
	require.Empty(t, vote.Options)

	out, err := clitestutil.ExecTestCLICmd(clientCtx, cli.MigrateGenesisCmd(), []string{"v0.43", genesisFile.Name()})
	require.EqualError(t, err, "unknown migration function for version: v0.43; supported target versions: none (this application has not registered any genesis migrations)")
	require.NotContains(t, out.String(), "\"app_state\"", "unregistered migrations must not emit a genesis document")
	require.NotContains(t, out.String(), "\"weight\":\"1.000000000000000000\"", "unregistered migrations must not claim a weighted-vote conversion")

	unchanged, err := os.ReadFile(genesisFile.Name())
	require.NoError(t, err)
	require.Equal(t, v040Valid, string(unchanged), "rejection must preserve the legacy vote fixture")
}

func TestRepairM_652_InvalidConsensusParams(t *testing.T) {
	genesisFile := testutil.WriteToNewTempFile(t, v037Exported)
	out, err := clitestutil.ExecTestCLICmd(repairM652ClientContext(t), cli.MigrateGenesisCmd(), []string{"v0.42", genesisFile.Name()})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Make sure that you have correctly migrated all Tendermint consensus params")
	require.NotContains(t, err.Error(), "unknown migration function", "consensus validation must reject the input before migration dispatch")
	require.NotContains(t, out.String(), "\"app_state\"", "rejected input must not be emitted as migrated genesis")
}
