package processblock

import (
	"encoding/json"
	"os"
	"os/exec"
	"testing"

	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestNewTestAppWithGenesisOverridesModuleSection(t *testing.T) {
	genesis := minttypes.DefaultGenesisState()
	genesis.Params.TokenReleaseSchedule = []minttypes.ScheduledTokenRelease{{
		StartDate:          "2099-01-01",
		EndDate:            "2099-01-31",
		TokenReleaseAmount: 12345,
	}}
	require.NoError(t, minttypes.ValidateGenesis(*genesis))
	encodingConfig := app.MakeEncodingConfig()
	section := encodingConfig.Marshaler.MustMarshalJSON(genesis)
	original := append(json.RawMessage(nil), section...)
	override := map[string]json.RawMessage{minttypes.ModuleName: section}

	a := NewTestAppWithGenesis(t, override)
	require.Equal(t, genesis.Params, a.MintKeeper.GetParams(a.Ctx()))
	require.True(t, a.BankKeeper.GetSupply(a.Ctx(), sdk.DefaultBondDenom).Amount.IsZero())
	require.Len(t, override, 1)
	require.Equal(t, original, override[minttypes.ModuleName])

	defaultApp := NewTestApp(t)
	defaults := app.NewDefaultGenesisState(encodingConfig.Marshaler)
	var defaultMint minttypes.GenesisState
	encodingConfig.Marshaler.MustUnmarshalJSON(defaults[minttypes.ModuleName], &defaultMint)
	for _, pair := range defaultMint.Params.ParamSetPairs() {
		stored := encodingConfig.Amino.MustMarshalJSON(pair.Value)
		encodingConfig.Amino.MustUnmarshalJSON(stored, pair.Value)
	}
	require.Equal(t, defaultMint.Params, defaultApp.MintKeeper.GetParams(defaultApp.Ctx()))
	require.Equal(t, defaultApp.BankKeeper.GetParams(defaultApp.Ctx()), a.BankKeeper.GetParams(a.Ctx()))
}

type genesisFailureRecorder struct {
	testing.TB
	failed bool
}

func (r *genesisFailureRecorder) recordFailure() {
	r.failed = r.Failed()
	r.Logf("recorded genesis failure: %t", r.failed)
}

func TestNewTestAppWithGenesisRejectsUnknownModule(t *testing.T) {
	const subprocessEnv = "PROCESSBLOCK_TEST_UNKNOWN_GENESIS_MODULE"
	if os.Getenv(subprocessEnv) == "1" {
		t.Run("unknown_module", func(t *testing.T) {
			recorder := &genesisFailureRecorder{TB: t}
			defer recorder.recordFailure()
			NewTestAppWithGenesis(t, map[string]json.RawMessage{
				"nosuchmodule": json.RawMessage(`{}`),
			})
			t.Fatal("constructor returned with an unknown genesis module")
		})
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestNewTestAppWithGenesisRejectsUnknownModule$", "-test.v")
	cmd.Env = append(os.Environ(), subprocessEnv+"=1")
	output, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr, "%s", output)
	require.Equal(t, 1, exitErr.ExitCode(), "%s", output)
	require.Contains(t, string(output), `unknown genesis module "nosuchmodule"`)
	require.Contains(t, string(output), "recorded genesis failure: true")
	require.Contains(t, string(output), "--- FAIL: TestNewTestAppWithGenesisRejectsUnknownModule/unknown_module")
	require.NotContains(t, string(output), "constructor returned with an unknown genesis module")
}
