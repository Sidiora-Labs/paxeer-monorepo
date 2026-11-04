package giga_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/tests/harness"
	evmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/config"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

const repairM1180Root = "data/qualification"

func repairM1180Fixtures(t *testing.T) map[string]*harness.StateTestJSON {
	t.Helper()
	fixtures, err := harness.LoadStateTest(filepath.Join(repairM1180Root, "transfer", "paxeerTransfer.json"))
	require.NoError(t, err)
	require.Len(t, fixtures, 3)
	return fixtures
}

func repairM1180Execute(t *testing.T, fixture *harness.StateTestJSON, mode ExecutorMode) (*StateTestContext, []*abci.ExecTxResult, common.Hash) {
	t.Helper()
	post := fixture.Post["Cancun"][0]
	tx, sender, err := harness.BuildTransaction(fixture, post)
	require.NoError(t, err)
	require.Equal(t, evmconfig.DefaultChainID, tx.ChainId().Int64())
	encoded, err := harness.EncodeTxForApp(tx)
	require.NoError(t, err)
	ctx := NewStateTestContext(t, time.Unix(1_800_000_000, 0).UTC(), 1, mode)
	require.Equal(t, mode == ModeGigaSequential, ctx.TestApp.GigaExecutorEnabled)
	require.Equal(t, tx.ChainId(), ctx.TestApp.EvmKeeper.ChainID(ctx.Ctx))
	ctx.SetupPreState(t, fixture.Pre)
	ctx.SetupSender(sender)
	_, results, err := RunStateTestBlock(ctx, [][]byte{encoded})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NotNil(t, results[0])
	return ctx, results, tx.Hash()
}

func TestRepairM_1180_RequiredSelection(t *testing.T) {
	// This is the same suite entry point as the documented real command. It
	// loads committed fixtures, encodes transactions and drives both ProcessBlocks.
	t.Setenv("STATE_TEST_ROOT", repairM1180Root)
	t.Setenv("STATE_TEST_DIR", "transfer")
	t.Setenv("STATE_TEST_NAME", "transferValid")
	t.Setenv("STATE_TEST_FORK", "Cancun")
	t.Setenv("STATE_TEST_COUNT", "1")
	t.Setenv("IGNORE_SKIP_LIST", "true")
	t.Setenv("VERIFY_ETHEREUM_SPEC", "true")
	t.Setenv("USE_REGULAR_STORE", "false")
	TestGigaVsV2_StateTests(t)

	tests, err := harness.LoadStateTestsFromDir(filepath.Join(repairM1180Root, "transfer"))
	require.NoError(t, err)
	selection := requiredStateSelection{Directory: "transfer", Name: "transferValid", Fork: "Cancun", Count: 1}
	cases := []struct {
		name      string
		selection requiredStateSelection
		empty     bool
	}{
		{name: "absent", selection: requiredStateSelection{}},
		{name: "empty", selection: selection, empty: true},
		{name: "unmatched", selection: requiredStateSelection{Directory: "transfer", Name: "absent", Fork: "Cancun", Count: 1}},
		{name: "unsupported fork", selection: requiredStateSelection{Directory: "transfer", Name: "transferValid", Fork: "Prague", Count: 1}},
		{name: "wrong count", selection: requiredStateSelection{Directory: "transfer", Name: "transferValid", Fork: "Cancun", Count: 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inventory := tests
			if tc.empty {
				inventory = nil
			}
			_, err := selectRequiredStateCases(inventory, tc.selection)
			require.Error(t, err)
		})
	}
	// The oracle cannot be disabled even when calling the comparison directly.
	fixture := repairM1180Fixtures(t)["transferValid"]
	require.False(t, runStateTestComparison(t, fixture, fixture.Post["Cancun"][0], ComparisonConfig{GigaMode: ModeGigaSequential}).Passed)
}

func TestRepairM_1180_ExceptionAgreement(t *testing.T) {
	fixtures := repairM1180Fixtures(t)
	outcomes := make(map[ExecutorMode]map[string][]*abci.ExecTxResult)
	for _, mode := range []ExecutorMode{ModeV2Sequential, ModeGigaSequential} {
		outcomes[mode] = make(map[string][]*abci.ExecTxResult)
		for _, name := range []string{"transferValid", "nonceRejected", "fundsRejected"} {
			t.Run(mode.String()+"/"+name, func(t *testing.T) {
				_, result, _ := repairM1180Execute(t, fixtures[name], mode)
				outcomes[mode][name] = result
				if name == "transferValid" {
					require.Zero(t, result[0].Code, result[0].Log)
				} else {
					require.NotZero(t, result[0].Code, result[0].Log)
					require.Equal(t, fixtures[name].Post["Cancun"][0].ExpectException, exceptionKind(result[0]), result[0].String())
				}
			})
		}
	}
	left, right := outcomes[ModeV2Sequential], outcomes[ModeGigaSequential]
	expected := fixtures["nonceRejected"].Post["Cancun"][0].ExpectException
	require.True(t, compareExecutionOutcomes(expected, left["nonceRejected"], nil, right["nonceRejected"], nil).Passed)
	require.True(t, compareExecutionOutcomes(fixtures["fundsRejected"].Post["Cancun"][0].ExpectException, left["fundsRejected"], nil, right["fundsRejected"], nil).Passed)
	for _, tc := range []struct{ name, expected, v2, giga string }{
		{"V2 only", expected, "nonceRejected", "transferValid"},
		{"Giga only", expected, "transferValid", "nonceRejected"},
		{"different errors", expected, "nonceRejected", "fundsRejected"},
		{"unexpected common failure", "", "nonceRejected", "nonceRejected"},
		{"wrong expected kind", "TransactionException.INTRINSIC_GAS_TOO_LOW", "nonceRejected", "nonceRejected"},
		{"unsupported expected kind", "unknown", "nonceRejected", "nonceRejected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := compareExecutionOutcomes(tc.expected, left[tc.v2], nil, right[tc.giga], nil)
			require.False(t, result.Passed)
			require.Equal(t, FailureTypeErrorMismatch, result.FailureType)
		})
	}
	// Decoder failures come from the real block processor, not a manufactured error.
	var executorErrors []error
	var malformedResults [][]*abci.ExecTxResult
	for _, mode := range []ExecutorMode{ModeV2Sequential, ModeGigaSequential} {
		ctx := NewStateTestContext(t, time.Unix(1_800_000_000, 0).UTC(), 1, mode)
		_, results, err := RunStateTestBlock(ctx, [][]byte{{0xff}})
		require.False(t, compareExecutionOutcomes("", results, err, right["transferValid"], nil).Passed)
		executorErrors = append(executorErrors, err)
		malformedResults = append(malformedResults, results)
	}
	require.False(t, compareExecutionOutcomes(expected, malformedResults[0], executorErrors[0], malformedResults[1], executorErrors[1]).Passed)

}

func TestRepairM_1180_IndependentOracle(t *testing.T) {
	fixture := repairM1180Fixtures(t)["transferValid"]
	expected := fixture.Post["Cancun"][0].State
	contexts := make([]*StateTestContext, 0, 2)
	for _, mode := range []ExecutorMode{ModeV2Sequential, ModeGigaSequential} {
		ctx, results, _ := repairM1180Execute(t, fixture, mode)
		require.Zero(t, results[0].Code, results[0].Log)
		require.Empty(t, verifyPostStateWithResult(t, ctx.Ctx, ctx.EvmKeeper(), ctx.BankKeeper(), expected, mode.String()))
		contexts = append(contexts, ctx)
	}
	// Both real stores agree on the same wrong nonce, so differential equality
	// alone succeeds. The unchanged independent fixture rejects each executor.
	sender := fixture.Transaction.Sender
	for _, ctx := range contexts {
		ctx.EvmKeeper().SetNonce(ctx.Ctx, sender, 99)
	}
	require.Empty(t, comparePostStates(t, contexts[0], contexts[1], fixture.Pre))
	for _, ctx := range contexts {
		diffs := verifyPostStateWithResult(t, ctx.Ctx, ctx.EvmKeeper(), ctx.BankKeeper(), expected, ctx.Mode.String())
		require.NotEmpty(t, diffs)
		require.Equal(t, FailureTypeNonceMismatch, diffs[0].Type)
	}
}

func TestRepairM_1180_OwnedInventory(t *testing.T) {
	skip, err := harness.LoadSkipList()
	require.NoError(t, err)
	var ledger struct {
		Repairs []struct {
			Case                string  `json:"case"`
			Reason              string  `json:"reason"`
			OwnerDomain         int     `json:"owner_domain"`
			OwnerPath           string  `json:"owner_path"`
			Status              string  `json:"status"`
			OriginalCommand     *string `json:"original_command"`
			OriginalCommandNote string  `json:"original_command_note"`
			ReplayCommand       *string `json:"replay_command"`
			VerifiedRevision    *string `json:"verified_revision"`
		} `json:"repairs"`
	}
	data, err := os.ReadFile("data/state_repairs.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &ledger))
	require.Len(t, ledger.Repairs, len(skip.Tests)+len(skip.Categories))
	seen := make(map[string]bool)
	for _, repair := range ledger.Repairs {
		require.False(t, seen[repair.Case], repair.Case)
		seen[repair.Case] = true
		require.Equal(t, 5, repair.OwnerDomain)
		require.Equal(t, "node/app.go", repair.OwnerPath)
		require.Equal(t, "unresolved", repair.Status)
		require.Nil(t, repair.OriginalCommand)
		require.NotEmpty(t, repair.OriginalCommandNote)
		require.Nil(t, repair.VerifiedRevision)
		if reason, exists := skip.Tests[repair.Case]; exists {
			require.Equal(t, reason, repair.Reason)
			require.NotNil(t, repair.ReplayCommand)
			require.Contains(t, *repair.ReplayCommand, "IGNORE_SKIP_LIST=true VERIFY_ETHEREUM_SPEC=true")
		} else {
			category := strings.TrimSuffix(repair.Case, "/*")
			require.True(t, skip.IsCategorySkipped(category), repair.Case)
		}
	}
	// The production loader and selector expose an exact active inventory. No
	// skip-list decision is involved in selecting qualification fixtures.
	fixtures, err := harness.LoadStateTestsFromDir(filepath.Join(repairM1180Root, "transfer"))
	require.NoError(t, err)
	selected, err := selectRequiredStateCases(fixtures, requiredStateSelection{Directory: "transfer", Name: "paxeerTransfer.json/", Fork: "Cancun", Count: 3})
	require.NoError(t, err)
	require.Len(t, selected, 3)
	for _, fixture := range selected {
		require.NotEmpty(t, fixture.Post.State)
	}
	t.Logf("historical unresolved cases=%d categories=%d; no exclusions removed", len(skip.Tests), len(skip.Categories))
}

func TestRepairM_1180_Runbook(t *testing.T) {
	contents, err := os.ReadFile("AGENTS.md")
	require.NoError(t, err)
	text := string(contents)
	for _, required := range []string{"STATE_TEST_ROOT=data/qualification", "STATE_TEST_DIR=transfer", "STATE_TEST_NAME=transferValid", "STATE_TEST_FORK=Cancun", "STATE_TEST_COUNT=1", "VERIFY_ETHEREUM_SPEC=true", "IGNORE_SKIP_LIST=true", "original failure command", "executed count", "verified revision", "Never add an exclusion"} {
		require.Contains(t, text, required)
	}
	fixtures, err := harness.LoadStateTestsFromDir(filepath.Join(repairM1180Root, "transfer"))
	require.NoError(t, err)
	selected, err := selectRequiredStateCases(fixtures, requiredStateSelection{Directory: "transfer", Name: "transferValid", Fork: "Cancun", Count: 1})
	require.NoError(t, err)
	require.Equal(t, "paxeerTransfer.json/transferValid", selected[0].Name)
}
