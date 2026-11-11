package giga_test

import (
	"bytes"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/tests/harness"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/require"
)

// EvmKeeperInterface defines the EVM keeper methods used by state tests.
// Both evmkeeper.Keeper and GigaEvmKeeper implement these methods.
type EvmKeeperInterface interface {
	GetNonce(ctx sdk.Context, addr common.Address) uint64
	SetNonce(ctx sdk.Context, addr common.Address, nonce uint64)
	GetCode(ctx sdk.Context, addr common.Address) []byte
	SetCode(ctx sdk.Context, addr common.Address, code []byte)
	GetState(ctx sdk.Context, addr common.Address, key common.Hash) common.Hash
	SetState(ctx sdk.Context, addr common.Address, key, value common.Hash)
	SetAddressMapping(ctx sdk.Context, paxAddr sdk.AccAddress, ethAddr common.Address)
	GetPaxAddressOrDefault(ctx sdk.Context, addr common.Address) sdk.AccAddress
}

// BankKeeperInterface defines the bank keeper methods used by state tests.
// Both bankkeeper and gigabankkeeper implement these methods.
type BankKeeperInterface interface {
	MintCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error
	SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	AddWei(ctx sdk.Context, addr sdk.AccAddress, amt sdk.Int) error
	GetBalance(ctx sdk.Context, addr sdk.AccAddress, denom string) sdk.Coin
	GetWeiBalance(ctx sdk.Context, addr sdk.AccAddress) sdk.Int
}

// FailureType categorizes the type of test failure
type FailureType string

const (
	FailureTypeResultCode      FailureType = "result_code"
	FailureTypeGasMismatch     FailureType = "gas_mismatch"
	FailureTypeStateMismatch   FailureType = "state_mismatch"
	FailureTypeCodeMismatch    FailureType = "code_mismatch"
	FailureTypeNonceMismatch   FailureType = "nonce_mismatch"
	FailureTypeBalanceMismatch FailureType = "balance_mismatch"
	FailureTypeErrorMismatch   FailureType = "error_mismatch"
	FailureTypeV2Error         FailureType = "v2_error"
	FailureTypeGigaError       FailureType = "giga_error"
	FailureTypeUnknown         FailureType = "unknown"
)

// TestResult captures the outcome of a state test comparison
type TestResult struct {
	Passed      bool
	FailureType FailureType
	Message     string
	Details     []string
}

// StateTestContext wraps the test context for state test execution
type StateTestContext struct {
	Ctx     sdk.Context
	TestApp *app.App
	Mode    ExecutorMode
}

// NewStateTestContext creates a test context configured for state tests
func NewStateTestContext(t testing.TB, blockTime time.Time, workers int, mode ExecutorMode) *StateTestContext {
	occEnabled := mode == ModeV2withOCC || mode == ModeGigaOCC
	gigaEnabled := mode == ModeGigaSequential || mode == ModeGigaOCC || mode == ModeGigaWithRegularStore
	gigaOCCEnabled := mode == ModeGigaOCC
	gigaWithRegularStore := mode == ModeGigaWithRegularStore

	testAcct := utils.NewSigner()

	var wrapper *app.TestWrapper
	if gigaWithRegularStore {
		// Special mode: Giga executor with regular KVStore (for debugging)
		wrapper = app.NewGigaTestWrapperWithRegularStore(t.(*testing.T), blockTime, testAcct.PublicKey, true, false, func(ba *baseapp.BaseApp) {
			ba.SetOccEnabled(false)
			ba.SetConcurrencyWorkers(workers)
		})
	} else if !gigaEnabled {
		wrapper = app.NewTestWrapperWithSc(t.(*testing.T), blockTime, testAcct.PublicKey, true, func(ba *baseapp.BaseApp) {
			ba.SetOccEnabled(occEnabled)
			ba.SetConcurrencyWorkers(workers)
		})
	} else {
		wrapper = app.NewGigaTestWrapper(t.(*testing.T), blockTime, testAcct.PublicKey, true, gigaOCCEnabled, func(ba *baseapp.BaseApp) {
			ba.SetOccEnabled(occEnabled)
			ba.SetConcurrencyWorkers(workers)
		})
	}
	testApp := wrapper.App
	t.Cleanup(func() { require.NoError(t, testApp.Close()) })
	ctx := wrapper.Ctx
	ctx = ctx.WithBlockHeader(tmproto.Header{
		Height:  ctx.BlockHeader().Height,
		ChainID: ctx.BlockHeader().ChainID,
		Time:    blockTime,
	})

	// Set minimum fee to 0 for state tests
	params := testApp.EvmKeeper.GetParams(ctx)
	params.MinimumFeePerGas = sdk.NewDecFromInt(sdk.NewInt(0))
	testApp.EvmKeeper.SetParams(ctx, params)

	// Also set minimum fee to 0 in GigaEvmKeeper for Giga mode
	if gigaEnabled {
		gigaParams := testApp.GigaEvmKeeper.GetParams(ctx)
		gigaParams.MinimumFeePerGas = sdk.NewDecFromInt(sdk.NewInt(0))
		testApp.GigaEvmKeeper.SetParams(ctx, gigaParams)
	}

	return &StateTestContext{
		Ctx:     ctx,
		TestApp: testApp,
		Mode:    mode,
	}
}

// IsGigaMode returns true if the context is configured for Giga execution modes.
func (stc *StateTestContext) IsGigaMode() bool {
	return stc.Mode == ModeGigaSequential || stc.Mode == ModeGigaOCC || stc.Mode == ModeGigaWithRegularStore
}

// EvmKeeper returns the appropriate EVM keeper based on mode.
// For Giga modes, returns GigaEvmKeeper; for V2 modes, returns EvmKeeper.
func (stc *StateTestContext) EvmKeeper() EvmKeeperInterface {
	if stc.IsGigaMode() {
		return &stc.TestApp.GigaEvmKeeper
	}
	return &stc.TestApp.EvmKeeper
}

// BankKeeper returns the appropriate bank keeper based on mode.
// For Giga modes, returns GigaBankKeeper; for V2 modes, returns BankKeeper.
func (stc *StateTestContext) BankKeeper() BankKeeperInterface {
	if stc.IsGigaMode() {
		return stc.TestApp.GigaBankKeeper
	}
	return stc.TestApp.BankKeeper
}

// SetupPreState configures the state from the test's pre-state allocation
func (stc *StateTestContext) SetupPreState(t testing.TB, pre ethtypes.GenesisAlloc) {
	evmKeeper := stc.EvmKeeper()
	bankKeeper := stc.BankKeeper()

	for addr, account := range pre {
		// Fund the account with the specified balance
		uhpx, wei := state.SplitUhpxWeiAmount(account.Balance)
		paxAddr := evmKeeper.GetPaxAddressOrDefault(stc.Ctx, addr)

		if uhpx.GT(sdk.ZeroInt()) {
			err := bankKeeper.MintCoins(stc.Ctx, "mint", sdk.NewCoins(sdk.NewCoin("uhpx", uhpx)))
			require.NoError(t, err, "failed to mint coins for %s", addr.Hex())
			err = bankKeeper.SendCoinsFromModuleToAccount(stc.Ctx, "mint", paxAddr, sdk.NewCoins(sdk.NewCoin("uhpx", uhpx)))
			require.NoError(t, err, "failed to send coins to %s", addr.Hex())
		}
		if wei.GT(sdk.ZeroInt()) {
			err := bankKeeper.AddWei(stc.Ctx, paxAddr, wei)
			require.NoError(t, err, "failed to add wei to %s", addr.Hex())
		}

		// Set nonce
		evmKeeper.SetNonce(stc.Ctx, addr, account.Nonce)

		// Set code if present
		if len(account.Code) > 0 {
			evmKeeper.SetCode(stc.Ctx, addr, account.Code)
		}

		// Set storage
		for key, value := range account.Storage {
			evmKeeper.SetState(stc.Ctx, addr, key, value)
		}

		// Associate the addresses
		evmKeeper.SetAddressMapping(stc.Ctx, paxAddr, addr)
	}
}

// SetupSender associates the sender address with its Pax address
func (stc *StateTestContext) SetupSender(sender common.Address) {
	evmKeeper := stc.EvmKeeper()
	paxAddr := evmKeeper.GetPaxAddressOrDefault(stc.Ctx, sender)
	evmKeeper.SetAddressMapping(stc.Ctx, paxAddr, sender)
}

// RunStateTestBlock executes a state test transaction and returns results
func RunStateTestBlock(stc *StateTestContext, txs [][]byte) ([]abci.Event, []*abci.ExecTxResult, error) {
	app.EnableOCC = stc.Mode == ModeV2withOCC || stc.Mode == ModeGigaOCC

	header := stc.Ctx.BlockHeader()
	req := &abci.RequestFinalizeBlock{
		Txs:    txs,
		Header: &header,
	}

	events, results, _, err := stc.TestApp.ProcessBlock(stc.Ctx, txs, app.NewBlockProcessRequest(req), req.DecidedLastCommit, false, nil)
	return events, results, err
}

// runStateTestComparison runs a state test and returns detailed results.
// The config parameter controls which Giga mode to use and whether to verify against fixtures.
func runStateTestComparison(t *testing.T, st *harness.StateTestJSON, post harness.StateTestPost, config ComparisonConfig) TestResult {
	blockTime := time.Unix(1_800_000_000, 0).UTC()
	if !config.VerifyEthereumSpec {
		return TestResult{FailureType: FailureTypeUnknown, Message: "qualification requires the fixture oracle"}
	}

	// Build the transaction
	signedTx, sender, err := harness.BuildTransaction(st, post)
	if err != nil {
		return TestResult{
			Passed:      false,
			FailureType: FailureTypeUnknown,
			Message:     fmt.Sprintf("failed to build transaction: %v", err),
		}
	}

	txBytes, err := harness.EncodeTxForApp(signedTx)
	if err != nil {
		return TestResult{
			Passed:      false,
			FailureType: FailureTypeUnknown,
			Message:     fmt.Sprintf("failed to encode transaction: %v", err),
		}
	}

	// --- Run with V2 Sequential (baseline) ---
	v2Ctx := NewStateTestContext(t, blockTime, 1, ModeV2Sequential)
	v2Ctx.SetupPreState(t, st.Pre)
	v2Ctx.SetupSender(sender)

	_, v2Results, v2Err := RunStateTestBlock(v2Ctx, [][]byte{txBytes})

	// --- Run with Giga (mode from config) ---
	gigaCtx := NewStateTestContext(t, blockTime, 1, config.GigaMode)
	gigaCtx.SetupPreState(t, st.Pre)
	gigaCtx.SetupSender(sender)

	_, gigaResults, gigaErr := RunStateTestBlock(gigaCtx, [][]byte{txBytes})

	// Block processing errors are infrastructure failures, never fixture exceptions.
	if result := compareExecutionOutcomes(post.ExpectException, v2Results, v2Err, gigaResults, gigaErr); !result.Passed {
		return result
	}
	if post.ExpectException != "" {
		return TestResult{Passed: true}
	}

	// --- Compare results ---
	if len(v2Results) != len(gigaResults) {
		return TestResult{
			Passed:      false,
			FailureType: FailureTypeUnknown,
			Message:     fmt.Sprintf("result count mismatch: V2=%d, Giga=%d", len(v2Results), len(gigaResults)),
		}
	}

	for i := range v2Results {
		// Compare success/failure
		if v2Results[i].Code != gigaResults[i].Code {
			details := []string{
				fmt.Sprintf("V2: code=%d log=%q", v2Results[i].Code, v2Results[i].Log),
				fmt.Sprintf("Giga: code=%d log=%q", gigaResults[i].Code, gigaResults[i].Log),
			}
			t.Logf("tx[%d] result code mismatch:", i)
			for _, d := range details {
				t.Logf("  %s", d)
			}
			return TestResult{
				Passed:      false,
				FailureType: FailureTypeResultCode,
				Message:     fmt.Sprintf("tx[%d] V2 code=%d, Giga code=%d", i, v2Results[i].Code, gigaResults[i].Code),
				Details:     details,
			}
		}

		// Compare gas used
		if v2Results[i].GasUsed != gigaResults[i].GasUsed {
			return TestResult{
				Passed:      false,
				FailureType: FailureTypeGasMismatch,
				Message:     fmt.Sprintf("tx[%d] gas mismatch: V2=%d, Giga=%d", i, v2Results[i].GasUsed, gigaResults[i].GasUsed),
				Details: []string{
					fmt.Sprintf("V2 gas used: %d", v2Results[i].GasUsed),
					fmt.Sprintf("Giga gas used: %d", gigaResults[i].GasUsed),
				},
			}
		}
	}

	// --- Compare V2 vs Giga post-state ---
	stateDiffs := comparePostStates(t, v2Ctx, gigaCtx, st.Pre)
	if len(stateDiffs) > 0 {
		t.Logf("V2 vs Giga state differences:")
		for _, diff := range stateDiffs {
			t.Logf("  %s", diff)
		}
		return TestResult{
			Passed:      false,
			FailureType: stateDiffs[0].Type,
			Message:     stateDiffs[0].Summary,
			Details:     formatStateDiffs(stateDiffs),
		}
	}

	// --- Verify post-state against fixture (if configured and available) ---
	if len(post.State) == 0 {
		return TestResult{FailureType: FailureTypeUnknown, Message: "fixture oracle has no explicit post.state"}
	}
	{
		v2Diffs := verifyPostStateWithResult(t, v2Ctx.Ctx, v2Ctx.EvmKeeper(), v2Ctx.BankKeeper(), post.State, "V2")
		gigaDiffs := verifyPostStateWithResult(t, gigaCtx.Ctx, gigaCtx.EvmKeeper(), gigaCtx.BankKeeper(), post.State, "Giga")

		diffs := append(v2Diffs, gigaDiffs...)
		if len(diffs) > 0 {
			return TestResult{FailureType: diffs[0].Type, Message: diffs[0].Summary, Details: formatStateDiffs(diffs)}
		}
		for _, ctx := range []*StateTestContext{v2Ctx, gigaCtx} {
			receipt, err := (&GigaTestContext{Ctx: ctx.Ctx, TestApp: ctx.TestApp, Mode: ctx.Mode}).GetTransientReceipt(t, signedTx.Hash(), 0)
			if err != nil {
				return TestResult{FailureType: FailureTypeUnknown, Message: fmt.Sprintf("fixture receipt: %v", err)}
			}
			logs := make([]*ethtypes.Log, 0, len(receipt.Logs))
			for _, log := range receipt.Logs {
				topics := make([]common.Hash, len(log.Topics))
				for i, topic := range log.Topics {
					topics[i] = common.HexToHash(topic)
				}
				logs = append(logs, &ethtypes.Log{Address: common.HexToAddress(log.Address), Topics: topics, Data: log.Data})
			}
			encoded, err := rlp.EncodeToBytes(logs)
			if err != nil {
				return TestResult{FailureType: FailureTypeUnknown, Message: err.Error()}
			}
			if crypto.Keccak256Hash(encoded) != post.Logs {
				return TestResult{FailureType: FailureTypeStateMismatch, Message: fmt.Sprintf("%s fixture logs hash differs: got %s want %s", ctx.Mode, crypto.Keccak256Hash(encoded), post.Logs)}
			}
		}

	}

	return TestResult{Passed: true}
}

// StateDiff represents a single state difference
type StateDiff struct {
	Type     FailureType
	Address  common.Address
	Summary  string
	Expected string
	Actual   string
}

// ComparisonConfig configures how state test comparison runs
type ComparisonConfig struct {
	GigaMode           ExecutorMode // ModeGigaSequential or ModeGigaWithRegularStore
	VerifyEthereumSpec bool         // Whether to verify against Ethereum test spec expected post-state
}

// formatStateDiffs converts state diffs to string slice for logging
func formatStateDiffs(diffs []StateDiff) []string {
	result := make([]string, 0, len(diffs))
	for _, diff := range diffs {
		result = append(result, fmt.Sprintf("%s at %s: expected=%s, actual=%s",
			diff.Type, diff.Address.Hex(), diff.Expected, diff.Actual))
	}
	return result
}

// comparePostStates compares state between V2 and Giga contexts
func comparePostStates(_ *testing.T, v2Ctx, gigaCtx *StateTestContext, preState ethtypes.GenesisAlloc) []StateDiff {
	var diffs []StateDiff
	v2Keeper := v2Ctx.EvmKeeper()
	gigaKeeper := gigaCtx.EvmKeeper()
	v2Bank := v2Ctx.BankKeeper()
	gigaBank := gigaCtx.BankKeeper()

	// Compare state for all addresses in pre-state
	for addr := range preState {
		// Get Pax address from EVM address
		paxAddr := v2Keeper.GetPaxAddressOrDefault(v2Ctx.Ctx, addr)

		// Compare balance (uhpx)
		v2Balance := stateBalanceWei(v2Ctx.Ctx, v2Bank, paxAddr)
		gigaBalance := stateBalanceWei(gigaCtx.Ctx, gigaBank, paxAddr)
		if v2Balance.Cmp(gigaBalance) != 0 {
			diffs = append(diffs, StateDiff{
				Type:     FailureTypeBalanceMismatch,
				Address:  addr,
				Summary:  fmt.Sprintf("balance V2=%s, Giga=%s", v2Balance.String(), gigaBalance.String()),
				Expected: v2Balance.String(),
				Actual:   gigaBalance.String(),
			})
		}

		// Compare nonce
		v2Nonce := v2Keeper.GetNonce(v2Ctx.Ctx, addr)
		gigaNonce := gigaKeeper.GetNonce(gigaCtx.Ctx, addr)
		if v2Nonce != gigaNonce {
			diffs = append(diffs, StateDiff{
				Type:     FailureTypeNonceMismatch,
				Address:  addr,
				Summary:  fmt.Sprintf("nonce V2=%d, Giga=%d", v2Nonce, gigaNonce),
				Expected: fmt.Sprintf("%d", v2Nonce),
				Actual:   fmt.Sprintf("%d", gigaNonce),
			})
		}

		// Compare code
		v2Code := v2Keeper.GetCode(v2Ctx.Ctx, addr)
		gigaCode := gigaKeeper.GetCode(gigaCtx.Ctx, addr)
		if !bytes.Equal(v2Code, gigaCode) {
			diffs = append(diffs, StateDiff{
				Type:     FailureTypeCodeMismatch,
				Address:  addr,
				Summary:  fmt.Sprintf("code len V2=%d, Giga=%d", len(v2Code), len(gigaCode)),
				Expected: fmt.Sprintf("%d bytes", len(v2Code)),
				Actual:   fmt.Sprintf("%d bytes", len(gigaCode)),
			})
		}

		// Compare storage for keys we know about
		for key := range preState[addr].Storage {
			v2Value := v2Keeper.GetState(v2Ctx.Ctx, addr, key)
			gigaValue := gigaKeeper.GetState(gigaCtx.Ctx, addr, key)
			if !bytes.Equal(v2Value.Bytes(), gigaValue.Bytes()) {
				diffs = append(diffs, StateDiff{
					Type:     FailureTypeStateMismatch,
					Address:  addr,
					Summary:  fmt.Sprintf("storage[%s] V2=%s, Giga=%s", truncateHex(key.Hex()), truncateHex(v2Value.Hex()), truncateHex(gigaValue.Hex())),
					Expected: v2Value.Hex(),
					Actual:   gigaValue.Hex(),
				})
			}
		}
	}

	return diffs
}

// verifyPostStateWithResult verifies state and returns detailed diffs
func verifyPostStateWithResult(t *testing.T, ctx sdk.Context, keeper EvmKeeperInterface, bankKeeper BankKeeperInterface, expectedState ethtypes.GenesisAlloc, executorName string) []StateDiff {
	var diffs []StateDiff

	for addr, expectedAccount := range expectedState {
		// Verify balance
		paxAddr := keeper.GetPaxAddressOrDefault(ctx, addr)
		actualBalance := stateBalanceWei(ctx, bankKeeper, paxAddr)

		expectedBalance := expectedAccount.Balance
		if expectedBalance == nil {
			expectedBalance = big.NewInt(0)
		}

		if actualBalance.Cmp(expectedBalance) != 0 {
			diffs = append(diffs, StateDiff{
				Type:     FailureTypeBalanceMismatch,
				Address:  addr,
				Summary:  fmt.Sprintf("balance expected=%s, actual=%s", expectedBalance.String(), actualBalance.String()),
				Expected: expectedBalance.String(),
				Actual:   actualBalance.String(),
			})
			t.Logf("%s: balance mismatch for %s: expected %s, got %s",
				executorName, addr.Hex(), expectedBalance.String(), actualBalance.String())
		}

		// Verify storage
		for key, expectedValue := range expectedAccount.Storage {
			actualValue := keeper.GetState(ctx, addr, key)
			if !bytes.Equal(expectedValue.Bytes(), actualValue.Bytes()) {
				diffs = append(diffs, StateDiff{
					Type:     FailureTypeStateMismatch,
					Address:  addr,
					Summary:  fmt.Sprintf("storage[%s] differs", truncateHex(key.Hex())),
					Expected: expectedValue.Hex(),
					Actual:   actualValue.Hex(),
				})
				t.Logf("%s: storage mismatch for %s key %s:", executorName, addr.Hex(), key.Hex())
				t.Logf("  expected: %s", expectedValue.Hex())
				t.Logf("  actual:   %s", actualValue.Hex())
			}
		}

		// Verify nonce
		actualNonce := keeper.GetNonce(ctx, addr)
		if expectedAccount.Nonce != actualNonce {
			diffs = append(diffs, StateDiff{
				Type:     FailureTypeNonceMismatch,
				Address:  addr,
				Summary:  fmt.Sprintf("nonce expected=%d, actual=%d", expectedAccount.Nonce, actualNonce),
				Expected: fmt.Sprintf("%d", expectedAccount.Nonce),
				Actual:   fmt.Sprintf("%d", actualNonce),
			})
			t.Logf("%s: nonce mismatch for %s: expected %d, got %d",
				executorName, addr.Hex(), expectedAccount.Nonce, actualNonce)
		}

		// Verify code
		actualCode := keeper.GetCode(ctx, addr)
		if !bytes.Equal(expectedAccount.Code, actualCode) {
			diffs = append(diffs, StateDiff{
				Type:     FailureTypeCodeMismatch,
				Address:  addr,
				Summary:  fmt.Sprintf("code len expected=%d, actual=%d", len(expectedAccount.Code), len(actualCode)),
				Expected: fmt.Sprintf("%d bytes", len(expectedAccount.Code)),
				Actual:   fmt.Sprintf("%d bytes", len(actualCode)),
			})
			t.Logf("%s: code mismatch for %s: expected %d bytes, got %d bytes",
				executorName, addr.Hex(), len(expectedAccount.Code), len(actualCode))
		}
	}

	return diffs
}

// truncateHex truncates a hex string for display
func truncateHex(hex string) string {
	if len(hex) > 18 {
		return hex[:10] + "..." + hex[len(hex)-4:]
	}
	return hex
}

// stateBalanceWei compares the complete EVM balance, including the fractional bank store.
func stateBalanceWei(ctx sdk.Context, bank BankKeeperInterface, addr sdk.AccAddress) *big.Int {
	return new(big.Int).Add(new(big.Int).Mul(bank.GetBalance(ctx, addr, "uhpx").Amount.BigInt(), state.UhpxToSweiMultiplier), bank.GetWeiBalance(ctx, addr).BigInt())
}

// exceptionKind classifies a failed transaction by the canonical ABCI code and EVM cause.
// Unknown codes and infrastructure errors never satisfy an expected fixture exception.
func exceptionKind(result *abci.ExecTxResult) string {
	if result == nil || result.Code == 0 {
		return ""
	}
	if result.Codespace == sdkerrors.RootCodespace {
		switch result.Code {
		case sdkerrors.ErrInsufficientFunds.ABCICode():
			return "TransactionException.INSUFFICIENT_ACCOUNT_FUNDS"
		case sdkerrors.ErrInvalidSequence.ABCICode(), sdkerrors.ErrWrongSequence.ABCICode():
			return "TransactionException.NONCE_MISMATCH"
		}
	}
	causes := []struct{ text, kind string }{
		{core.ErrIntrinsicGas.Error(), "TransactionException.INTRINSIC_GAS_TOO_LOW"},
		{core.ErrInsufficientFunds.Error(), "TransactionException.INSUFFICIENT_ACCOUNT_FUNDS"},
		{core.ErrNonceTooHigh.Error(), "TransactionException.NONCE_MISMATCH"},
		{core.ErrNonceTooLow.Error(), "TransactionException.NONCE_MISMATCH"},
		{core.ErrFeeCapTooLow.Error(), "TransactionException.INSUFFICIENT_MAX_FEE_PER_GAS"},
		{core.ErrTipAboveFeeCap.Error(), "TransactionException.PRIORITY_GREATER_THAN_MAX_FEE_PER_GAS"},
		{core.ErrSenderNoEOA.Error(), "TransactionException.SENDER_NOT_EOA"},
	}
	if result.Codespace == "giga-infrastructure" {
		return ""
	}
	for _, cause := range causes {
		if strings.Contains(result.Log, cause.text) {
			return cause.kind
		}
	}
	return ""
}

func compareExecutionOutcomes(expected string, v2 []*abci.ExecTxResult, v2Err error, giga []*abci.ExecTxResult, gigaErr error) TestResult {
	fail := func(message string) TestResult {
		return TestResult{FailureType: FailureTypeErrorMismatch, Message: message}
	}
	if v2Err != nil || gigaErr != nil {
		return fail(fmt.Sprintf("unexpected executor error: V2=%v Giga=%v", v2Err, gigaErr))
	}
	if len(v2) != 1 || len(giga) != 1 || v2[0] == nil || giga[0] == nil {
		return fail("each executor must return exactly one nonnil result")
	}
	if expected == "" {
		if v2[0].Code != 0 || giga[0].Code != 0 {
			return fail(fmt.Sprintf("unexpected transaction failure: V2=%s Giga=%s", v2[0], giga[0]))
		}
		return TestResult{Passed: true}
	}
	left, right := exceptionKind(v2[0]), exceptionKind(giga[0])
	if left == "" || right == "" || left != right {
		return fail(fmt.Sprintf("expected %s: V2=%q Giga=%q", expected, left, right))
	}
	for _, allowed := range strings.Split(expected, "|") {
		if left == allowed {
			return TestResult{Passed: true}
		}
	}
	return fail(fmt.Sprintf("expected %s: both executors returned %s", expected, left))
}

type requiredStateSelection struct {
	Directory string
	Name      string
	Fork      string
	Count     int
}

type selectedStateCase struct {
	Name string
	Test *harness.StateTestJSON
	Post harness.StateTestPost
}

func selectRequiredStateCases(tests map[string]*harness.StateTestJSON, selection requiredStateSelection) ([]selectedStateCase, error) {
	if selection.Directory == "" || selection.Name == "" || selection.Fork != "Cancun" || selection.Count <= 0 {
		return nil, fmt.Errorf("required selection needs directory, name, Cancun fork and positive expected count")
	}
	lastSlash := strings.LastIndex(selection.Name, "/")
	indexed := lastSlash >= 0 && lastSlash+1 < len(selection.Name) && strings.Trim(selection.Name[lastSlash+1:], "0123456789") == ""
	names := make([]string, 0, len(tests))
	for name := range tests {
		names = append(names, name)
	}
	sort.Strings(names)
	var selected []selectedStateCase
	for _, name := range names {
		posts := tests[name].Post[selection.Fork]
		if len(posts) == 0 && strings.Contains(name, selection.Name) {
			return nil, fmt.Errorf("selected fixture %s has no %s posts", name, selection.Fork)
		}
		for index, post := range posts {
			caseName := name
			if len(posts) > 1 {
				caseName = fmt.Sprintf("%s/%d", name, index)
			}
			matches := strings.Contains(caseName, selection.Name)
			if indexed {
				matches = caseName == selection.Name
			}
			if !matches {
				continue
			}
			selected = append(selected, selectedStateCase{Name: caseName, Test: tests[name], Post: post})
		}
	}
	if len(selected) != selection.Count {
		return nil, fmt.Errorf("selected=%d required=%d", len(selected), selection.Count)
	}
	return selected, nil
}

func runRequiredStateSelection(t *testing.T, root string, selection requiredStateSelection, config ComparisonConfig, summaryName string) {
	t.Helper()
	tests, err := harness.LoadStateTestsFromDir(filepath.Join(root, selection.Directory))
	require.NoError(t, err)
	selected, err := selectRequiredStateCases(tests, selection)
	require.NoError(t, err)
	summary := NewTestSummary()
	executed := 0
	for _, fixture := range selected {
		fixture := fixture
		t.Logf("Nonexcluded fixture: %s/%s fork=%s", selection.Directory, fixture.Name, selection.Fork)
		t.Run(fixture.Name, func(t *testing.T) {
			executed++
			result := runStateTestComparison(t, fixture.Test, fixture.Post, config)
			if result.Passed {
				summary.RecordPass(selection.Directory, fixture.Name)
			} else {
				summary.RecordFailure(selection.Directory, fixture.Name, result.FailureType, result.Message)
				t.Errorf("State comparison failed: %s - %s: %v", result.FailureType, result.Message, result.Details)
			}
		})
	}
	t.Logf("%s: selected=%d executed=%d required=%d skipped=%d", summaryName, len(selected), executed, selection.Count, summary.TotalSkipped)
	summary.PrintSummary(t)
	require.Equal(t, selection.Count, executed, "required fixtures were filtered out by the Go test runner")
	require.Zero(t, summary.TotalSkipped)
}

func runStateTestSuite(t *testing.T, config ComparisonConfig, summaryName string) {
	count, err := strconv.Atoi(os.Getenv("STATE_TEST_COUNT"))
	require.NoError(t, err, "STATE_TEST_COUNT must declare the required executed count")
	selection := requiredStateSelection{Directory: os.Getenv("STATE_TEST_DIR"), Name: os.Getenv("STATE_TEST_NAME"), Fork: os.Getenv("STATE_TEST_FORK"), Count: count}
	// Validate configuration before touching the archive. Exclusions never qualify a repair.
	require.NotEmpty(t, selection.Directory)
	require.NotEmpty(t, selection.Name)
	require.Equal(t, "Cancun", selection.Fork)
	require.Positive(t, selection.Count)
	require.Equal(t, "true", os.Getenv("IGNORE_SKIP_LIST"), "qualification must activate every selected fixture")
	require.Equal(t, "true", os.Getenv("VERIFY_ETHEREUM_SPEC"), "qualification must enable the independent oracle")
	root := os.Getenv("STATE_TEST_ROOT")
	if root == "" {
		root, err = harness.GetStateTestsPath()
		require.NoError(t, err)
	}
	runRequiredStateSelection(t, root, selection, config, summaryName)
}
