package common_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	common "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/oracle"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func upgradeContext(t *testing.T) (sdk.Context, *upgradekeeper.Keeper) {
	t.Helper()
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	key := sdk.NewKVStoreKey(upgradetypes.StoreKey)
	ms := store.NewCommitMultiStore(db)
	ms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	require.NoError(t, ms.LoadLatestVersion())
	ctx := sdk.NewContext(ms, tmproto.Header{Height: 99}, false)
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	upgrade := upgradekeeper.NewKeeper(nil, key, cdc, t.TempDir(), nil)
	return ctx, &upgrade
}

func activeContext(t *testing.T) (sdk.Context, *upgradekeeper.Keeper) {
	t.Helper()
	ctx, upgrade := upgradeContext(t)
	ctx = ctx.WithBlockHeight(100)
	upgrade.SetDone(ctx, "merge-plan")
	return ctx, upgrade
}

func evmContext(ctx sdk.Context) *vm.EVM {
	db := new(state.DBImpl)
	db.WithCtx(ctx)
	return &vm.EVM{StateDB: db}
}

func selector(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(value)
	require.NoError(t, err)
	return decoded
}

func revertText(t *testing.T, data []byte) string {
	t.Helper()
	require.GreaterOrEqual(t, len(data), 4)
	require.Equal(t, []byte{0x08, 0xc3, 0x79, 0xa0}, data[:4])
	text, err := abi.UnpackRevert(data)
	require.NoError(t, err)
	return text
}

func guardedABI(t *testing.T) abi.ABI {
	t.Helper()
	a, err := abi.JSON(strings.NewReader(`[
		{"type":"function","name":"schedule","stateMutability":"payable","inputs":[{"name":"target","type":"address"},{"name":"callData","type":"bytes"},{"name":"executeAtBlock","type":"uint64"},{"name":"gasLimit","type":"uint64"}],"outputs":[{"name":"jobId","type":"uint256"}]},
		{"type":"function","name":"settle","stateMutability":"nonpayable","inputs":[{"name":"streamId","type":"uint256"}],"outputs":[{"name":"paid","type":"uint256"}]},
		{"type":"function","name":"hashTypedData","stateMutability":"pure","inputs":[{"name":"domainSeparator","type":"bytes32"},{"name":"structHash","type":"bytes32"}],"outputs":[{"name":"digest","type":"bytes32"}]}
	]`))
	require.NoError(t, err)
	return a
}

func TestUnknownSelectorGasAndRevert(t *testing.T) {
	ctx, upgrade := activeContext(t)
	p := common.NewPrecompile(abi.ABI{}, nil, ethcommon.Address{}, "layerxagent", upgrade, "merge-plan")
	input := selector(t, "deadbeef")
	require.Equal(t, uint64(3000), common.MethodGas(input, len(input)))
	require.Equal(t, uint64(3000), p.RequiredGas(input))
	evm := evmContext(ctx)
	data, err := p.Run(evm, ethcommon.Address{}, ethcommon.Address{}, input, nil, false, false, nil)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Equal(t, "LXP_PRECOMPILE_UNKNOWN_METHOD: no method with id: 0xdeadbeef", revertText(t, data))
	require.ErrorIs(t, state.GetDBImpl(evm.StateDB).GetPrecompileError(), common.ErrUnknownMethod)
	for _, unknown := range [][]byte{
		nil, {0xde}, {0xde, 0xad, 0xbe}, {0xde, 0xad, 0xbe, 0xef, 0x00},
		selector(t, "b6a4c9ae"), selector(t, "c6f7591b"),
	} {
		for _, length := range []int{-1, 0, 4, 260, math.MaxInt} {
			require.Equal(t, pcommon.UnknownMethodCallGas, common.MethodGas(unknown, length))
		}
	}
}

func TestDelegatecallAndStaticRefused(t *testing.T) {
	ctx, upgrade := activeContext(t)
	for _, test := range []struct {
		name        string
		readOnly    bool
		isDelegate  bool
		ctxDelegate bool
		want        common.ErrorCode
		message     string
	}{
		{"delegate", false, true, false, common.ErrDelegatecall, "LXP_AGENT_DELEGATECALL: cannot delegatecall an agent function"},
		{"context delegate", false, false, true, common.ErrDelegatecall, "LXP_AGENT_DELEGATECALL: cannot delegatecall an agent function"},
		{"static", true, false, false, common.ErrReadOnly, "LXP_AGENT_READ_ONLY: write under staticcall"},
		{"static delegate", true, true, false, common.ErrDelegatecall, "LXP_AGENT_DELEGATECALL: cannot delegatecall an agent function"},
		{"static context delegate", true, false, true, common.ErrDelegatecall, "LXP_AGENT_DELEGATECALL: cannot delegatecall an agent function"},
	} {
		t.Run(test.name, func(t *testing.T) {
			callCtx := ctx.WithEVMPrecompileCalledFromDelegateCall(test.ctxDelegate)
			err := common.GuardWrite(callCtx, test.readOnly, test.isDelegate)
			require.ErrorIs(t, err, test.want)
			require.EqualError(t, err, test.message)
			require.Equal(t, test.message, revertText(t, pcommon.RevertReason(err)))
		})
	}
	require.NoError(t, common.GuardWrite(ctx, false, false))
	a := guardedABI(t)
	p := common.NewPrecompile(a, nil, ethcommon.Address{}, "layerxagent", upgrade, "merge-plan")
	schedule := a.Methods["schedule"]
	require.Equal(t, selector(t, "849ba0a3"), schedule.ID)
	deposit := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	data, err := p.Run(evmContext(ctx), ethcommon.Address{}, ethcommon.Address{}, schedule.ID, deposit, false, true, nil)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Equal(t, "LXP_AGENT_DELEGATECALL: cannot delegatecall an agent function", revertText(t, data))
	input, err := a.Pack("settle", big.NewInt(7))
	require.NoError(t, err)
	data, err = p.Run(evmContext(ctx), ethcommon.Address{}, ethcommon.Address{}, input, nil, true, false, nil)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Equal(t, "LXP_AGENT_READ_ONLY: write under staticcall", revertText(t, data))
}

func TestHeightGateBeforePlan(t *testing.T) {
	ctx, upgrade := upgradeContext(t)
	bound := common.WithHeightGate(ctx, upgrade, "merge-plan")
	require.NoError(t, upgrade.ScheduleUpgrade(ctx, upgradetypes.Plan{Name: "merge-plan", Height: 100}))
	err := common.HeightGate(bound)
	require.ErrorIs(t, err, common.ErrNotActive)
	require.EqualError(t, err, "LXP_AGENT_NOT_ACTIVE: merge-plan not active at height 99")
	require.ErrorIs(t, common.HeightGate(bound.WithBlockHeight(100)), common.ErrNotActive)
	upgrade.SetDone(ctx.WithBlockHeight(100), "merge-plan")
	for _, height := range []int64{-1, 0, 1, 99, 100, 101, math.MaxInt64} {
		meter := sdk.NewGasMeter(1, 1, 1)
		callCtx := bound.WithBlockHeight(height).WithGasMeter(meter)
		err := common.HeightGate(callCtx)
		if height < 100 {
			require.ErrorIs(t, err, common.ErrNotActive)
		} else {
			require.NoError(t, err)
		}
		require.Zero(t, meter.GasConsumed())
	}
	require.ErrorIs(t, common.HeightGate(common.WithHeightGate(ctx, upgrade, "other-plan")), common.ErrNotActive)
	a := guardedABI(t)
	input, err := a.Pack("hashTypedData", [32]byte{1}, [32]byte{2})
	require.NoError(t, err)
	p := common.NewPrecompile(a, nil, ethcommon.Address{}, "layerxagent", upgrade, "merge-plan")
	require.Equal(t, uint64(1204), p.RequiredGas(input))
	meter := sdk.NewGasMeter(1, 1, 1)
	data, err := p.Run(evmContext(ctx.WithGasMeter(meter)), ethcommon.Address{}, ethcommon.Address{}, input, nil, true, false, nil)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Equal(t, "LXP_AGENT_NOT_ACTIVE: merge-plan not active at height 99", revertText(t, data))
	require.Zero(t, meter.GasConsumed())
}

func TestHeightGateRequiresBinding(t *testing.T) {
	ctx, upgrade := activeContext(t)
	require.ErrorIs(t, common.HeightGate(sdk.Context{}), common.ErrNoContext)
	require.ErrorIs(t, common.HeightGate(common.WithHeightGate(sdk.Context{}, upgrade, "merge-plan")), common.ErrNoContext)
	require.ErrorIs(t, common.HeightGate(ctx), common.ErrNotActive)
	require.ErrorIs(t, common.HeightGate(common.WithHeightGate(ctx, nil, "merge-plan")), common.ErrNotActive)
	require.ErrorIs(t, common.HeightGate(common.WithHeightGate(ctx, upgrade, "")), common.ErrNotActive)
	require.NoError(t, common.HeightGate(common.WithHeightGate(ctx, upgrade, "merge-plan")))
}

func TestGasTable(t *testing.T) {
	for _, row := range []struct {
		method   string
		selector string
		base     uint64
		perByte  uint64
		at260    uint64
	}{
		{"resolveOffset", "454ad3a1", 1000, 3, 1780},
		{"resolveOffsetBatch", "3df597e3", 1000, 3, 1780},
		{"toOffset", "3f779aef", 1000, 3, 1780},
		{"computeClearing", "8397e9f8", 6000, 3, 6780},
		{"aggregate", "81097a95", 1000, 3, 1780},
		{"submitPrice", "dc9c2875", 30000, 30, 37800},
		{"scoreFill", "e087a6ba", 1000, 3, 1780},
		{"scoreBatch", "1b1fba48", 1000, 3, 1780},
		{"updateRollingScore", "a8f32b0e", 1000, 3, 1780},
		{"schedule", "849ba0a3", 25000, 30, 32800},
		{"cancel", "40e58ee5", 20000, 30, 27800},
		{"reschedule", "08e6f770", 20000, 30, 27800},
		{"open", "93e5b08b", 50000, 30, 57800},
		{"settle", "8df82800", 60000, 30, 67800},
		{"close", "0aebeb4e", 80000, 30, 87800},
		{"updateRate", "405abb41", 60000, 30, 67800},
		{"verify", "5fa55ca7", 46000, 3, 46780},
		{"verifyAndExpect", "71ded4b3", 46000, 3, 46780},
		{"hashTypedData", "d61c3c34", 1000, 3, 1780},
		{"domainSeparator", "e6d341cd", 1000, 3, 1780},
		{"recoverTypedSigner", "ac580b5c", 4000, 3, 4780},
		{"getValidatorPrice", "99812615", 1000, 3, 1780},
		{"getJob", "bf22c457", 1000, 3, 1780},
		{"pending", "5eebea20", 1000, 3, 1780},
		{"accrued", "f52981f4", 1000, 3, 1780},
		{"getStream", "894e9a0d", 1000, 3, 1780},
		{"rootOf", "01ec24b8", 1000, 3, 1780},
		{"rootCount", "26f02502", 1000, 3, 1780},
	} {
		t.Run(row.method, func(t *testing.T) {
			id := selector(t, row.selector)
			orobSignatures := map[string]string{
				"resolveOffset":      "resolveOffset(int256,int16)",
				"resolveOffsetBatch": "resolveOffsetBatch(int256,int16[])",
				"toOffset":           "toOffset(int256,int256)",
			}
			if signature, ok := orobSignatures[row.method]; ok {
				require.Equal(t, crypto.Keccak256([]byte(signature))[:4], id)
			}
			require.Equal(t, row.base, common.MethodGas(id, 0))
			require.Equal(t, row.at260, common.MethodGas(id, 260))
			require.Equal(t, row.base+4*row.perByte, common.MethodGas(id, 4))
			require.Equal(t, uint64(math.MaxUint64), common.MethodGas(id, -1))
			if strconv.IntSize == 64 {
				require.Equal(t, uint64(math.MaxUint64), common.MethodGas(id, math.MaxInt))
				limit := (uint64(math.MaxUint64) - row.base) / row.perByte
				require.Equal(t, row.base+row.perByte*limit, common.MethodGas(id, int(limit)))
				require.Equal(t, uint64(math.MaxUint64), common.MethodGas(id, int(limit+1)))
			}
		})
	}
	require.Equal(t, uint64(21080), common.MethodGas(selector(t, "40e58ee5"), 36))
	// Bind the scheduler gas inventory to canonical ABI signatures, not only literal selectors.
	for signature, id := range map[string]string{
		"schedule(address,bytes,uint64,uint64)": "849ba0a3",
		"cancel(uint256)":                       "40e58ee5",
		"reschedule(uint256,uint64)":            "08e6f770",
		"getJob(uint256)":                       "bf22c457",
		"pending(address)":                      "5eebea20",
	} {
		require.Equal(t, selector(t, id), crypto.Keccak256([]byte(signature))[:4])
	}
}

func TestFrameworkRefusalCodes(t *testing.T) {
	ctx, upgrade := activeContext(t)
	a := guardedABI(t)
	p := common.NewPrecompile(a, nil, ethcommon.Address{}, "layerxagent", upgrade, "merge-plan")
	for _, test := range []struct {
		name    string
		evm     *vm.EVM
		input   []byte
		value   *big.Int
		want    common.ErrorCode
		message string
	}{
		{"nil EVM", nil, nil, nil, common.ErrNoContext, "LXP_PRECOMPILE_NO_CONTEXT: cannot get context from EVM"},
		{"missing state", &vm.EVM{}, nil, nil, common.ErrNoContext, "LXP_PRECOMPILE_NO_CONTEXT: cannot get context from EVM"},
		{"missing context", evmContext(sdk.Context{}), nil, nil, common.ErrNoContext, "LXP_PRECOMPILE_NO_CONTEXT: cannot get context from EVM"},
		{"short input", evmContext(ctx), []byte{1, 2, 3}, nil, common.ErrInputTooShort, "LXP_PRECOMPILE_INPUT_TOO_SHORT: input too short to extract method ID"},
		{"decode", evmContext(ctx), a.Methods["hashTypedData"].ID, nil, common.ErrABIDecode, ""},
		{"nonpayable", evmContext(ctx), a.Methods["hashTypedData"].ID, big.NewInt(1), common.ErrNonPayable, "LXP_PRECOMPILE_NON_PAYABLE: sending funds to a non-payable function"},
	} {
		t.Run(test.name, func(t *testing.T) {
			data, err := p.Run(test.evm, ethcommon.Address{}, ethcommon.Address{}, test.input, test.value, false, false, nil)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			text := revertText(t, data)
			if test.message != "" {
				require.Equal(t, test.message, text)
			} else {
				require.Equal(t, "LXP_AGENT_ABI_DECODE: abi: attempting to unmarshal an empty string while arguments are expected", text)
			}
			if test.evm != nil {
				if db := state.GetDBImpl(test.evm.StateDB); db != nil {
					require.ErrorIs(t, db.GetPrecompileError(), test.want)
				}
			}
		})
	}
	var absent *common.Precompile
	require.Equal(t, pcommon.UnknownMethodCallGas, absent.RequiredGas(selector(t, "deadbeef")))
	data, err := absent.Run(nil, ethcommon.Address{}, ethcommon.Address{}, nil, nil, false, false, nil)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Equal(t, "LXP_PRECOMPILE_NO_CONTEXT: cannot get context from EVM", revertText(t, data))
	require.Equal(t, pcommon.UnknownMethodCallGas, p.RequiredGas(nil))
	input, err := a.Pack("hashTypedData", [32]byte{}, [32]byte{})
	require.NoError(t, err)
	data, err = p.Run(evmContext(ctx), ethcommon.Address{}, ethcommon.Address{}, input, nil, true, false, nil)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Equal(t, "LXP_PRECOMPILE_NO_CONTEXT: precompile executor is unavailable", revertText(t, data))
}

func TestArgumentAndPaymentValidation(t *testing.T) {
	require.NoError(t, common.ValidateArgsLength(nil, 0))
	require.NoError(t, common.ValidateArgsLength([]interface{}{big.NewInt(1)}, 1))
	for _, test := range []struct {
		args    []interface{}
		length  int
		message string
	}{
		{nil, 1, "LXP_PRECOMPILE_ARG_COUNT: expected 1 arguments but got 0"},
		{[]interface{}{1}, 0, "LXP_PRECOMPILE_ARG_COUNT: expected 0 arguments but got 1"},
		{[]interface{}{1, 2}, 1, "LXP_PRECOMPILE_ARG_COUNT: expected 1 arguments but got 2"},
	} {
		err := common.ValidateArgsLength(test.args, test.length)
		require.ErrorIs(t, err, common.ErrArgCount)
		require.EqualError(t, err, test.message)
		require.Equal(t, test.message, revertText(t, pcommon.RevertReason(err)))
	}
	require.NoError(t, common.ValidateNonPayable(nil))
	require.NoError(t, common.ValidateNonPayable(big.NewInt(0)))
	for _, value := range []*big.Int{big.NewInt(1), big.NewInt(-1)} {
		err := common.ValidateNonPayable(value)
		require.ErrorIs(t, err, common.ErrNonPayable)
		require.EqualError(t, err, "LXP_PRECOMPILE_NON_PAYABLE: sending funds to a non-payable function")
	}
}

func TestErrorCodeRegistry(t *testing.T) {
	for _, row := range []struct {
		code common.ErrorCode
		text string
	}{
		{common.ErrInputTooShort, "LXP_PRECOMPILE_INPUT_TOO_SHORT"},
		{common.ErrUnknownMethod, "LXP_PRECOMPILE_UNKNOWN_METHOD"},
		{common.ErrArgCount, "LXP_PRECOMPILE_ARG_COUNT"},
		{common.ErrNonPayable, "LXP_PRECOMPILE_NON_PAYABLE"},
		{common.ErrNoContext, "LXP_PRECOMPILE_NO_CONTEXT"},
		{common.ErrABIDecode, "LXP_AGENT_ABI_DECODE"},
		{common.ErrReadOnly, "LXP_AGENT_READ_ONLY"},
		{common.ErrDelegatecall, "LXP_AGENT_DELEGATECALL"},
		{common.ErrOverflow, "LXP_AGENT_OVERFLOW"},
		{common.ErrBatchTooLarge, "LXP_AGENT_BATCH_TOO_LARGE"},
		{common.ErrOrobOraclePrice, "LXP_AGENT_OROB_ORACLE_PRICE"},
		{common.ErrOrobOffsetRange, "LXP_AGENT_OROB_OFFSET_RANGE"},
		{common.ErrClearingLengthMismatch, "LXP_AGENT_CLEARING_LENGTH_MISMATCH"},
		{common.ErrOracleNoFeeds, "LXP_AGENT_ORACLE_NO_FEEDS"},
		{common.ErrOracleNoConfidentFeeds, "LXP_AGENT_ORACLE_NO_CONFIDENT_FEEDS"},
		{common.ErrOracleNotSigner, "LXP_AGENT_ORACLE_NOT_SIGNER"},
		{common.ErrOraclePrice, "LXP_AGENT_ORACLE_PRICE"},
		{common.ErrOracleConfidence, "LXP_AGENT_ORACLE_CONFIDENCE"},
		{common.ErrPofqLengthMismatch, "LXP_AGENT_POFQ_LENGTH_MISMATCH"},
		{common.ErrPofqDecayRange, "LXP_AGENT_POFQ_DECAY_RANGE"},
		{common.ErrSchedulerJobNotFound, "LXP_AGENT_SCHEDULER_JOB_NOT_FOUND"},
		{common.ErrSchedulerUnauthorized, "LXP_AGENT_SCHEDULER_UNAUTHORIZED"},
		{common.ErrSchedulerHorizonExceeded, "LXP_AGENT_SCHEDULER_HORIZON_EXCEEDED"},
		{common.ErrSchedulerPastBlock, "LXP_AGENT_SCHEDULER_PAST_BLOCK"},
		{common.ErrSchedulerMaxJobs, "LXP_AGENT_SCHEDULER_MAX_JOBS"},
		{common.ErrSchedulerDepositTooLow, "LXP_AGENT_SCHEDULER_DEPOSIT_TOO_LOW"},
		{common.ErrSchedulerInvalidGasLimit, "LXP_AGENT_SCHEDULER_INVALID_GAS_LIMIT"},
		{common.ErrSchedulerInvalidTarget, "LXP_AGENT_SCHEDULER_INVALID_TARGET"},
		{common.ErrSchedulerJobInactive, "LXP_AGENT_SCHEDULER_JOB_INACTIVE"},
		{common.ErrSchedulerEscrow, "LXP_AGENT_SCHEDULER_ESCROW"},
		{common.ErrStreamsInvalidPayee, "LXP_AGENT_STREAMS_INVALID_PAYEE"},
		{common.ErrStreamsSelfPayment, "LXP_AGENT_STREAMS_SELF_PAYMENT"},
		{common.ErrStreamsUnknownToken, "LXP_AGENT_STREAMS_UNKNOWN_TOKEN"},
		{common.ErrTEEUnknownFamily, "LXP_AGENT_TEE_UNKNOWN_FAMILY"},
		{common.ErrTEENoRoots, "LXP_AGENT_TEE_NO_ROOTS"},
		{common.ErrTEEInvalidQuote, "LXP_AGENT_TEE_INVALID_QUOTE"},
		{common.ErrTEEUntrustedChain, "LXP_AGENT_TEE_UNTRUSTED_CHAIN"},
		{common.ErrTEEStale, "LXP_AGENT_TEE_STALE"},
		{common.ErrTEEDebugRefused, "LXP_AGENT_TEE_DEBUG_REFUSED"},
		{common.ErrTEEReportDataMismatch, "LXP_AGENT_TEE_REPORT_DATA_MISMATCH"},
		{common.ErrTEEUnauthorizedUpdate, "LXP_AGENT_TEE_UNAUTHORIZED_UPDATE"},
		{common.ErrNotActive, "LXP_AGENT_NOT_ACTIVE"},
	} {
		t.Run(row.text, func(t *testing.T) {
			require.Equal(t, row.text, row.code.Error())
			require.True(t, errors.Is(row.code, row.code))
			data := pcommon.RevertReason(row.code)
			require.Equal(t, row.text, revertText(t, data))
			require.True(t, bytes.Equal(data[:4], []byte{0x08, 0xc3, 0x79, 0xa0}))
		})
	}
}

func TestStreamsGasCanonicalCallSizes(t *testing.T) {
	for _, row := range []struct {
		signature string
		words     int
		want      uint64
	}{
		{"open(address,address,uint256,uint64,uint64,uint256)", 6, 55880},
		{"settle(uint256)", 1, 61080},
		{"close(uint256)", 1, 81080},
		{"updateRate(uint256,uint256)", 2, 62040},
		{"accrued(uint256)", 1, 1108},
		{"getStream(uint256)", 1, 1108},
	} {
		t.Run(row.signature, func(t *testing.T) {
			id := crypto.Keccak256([]byte(row.signature))[:4]
			require.Equal(t, row.want, common.MethodGas(id, 4+32*row.words))
		})
	}
}

// TestEip712CalldataGas checks the exported schedule against real ABI calldata.
func TestEip712CalldataGas(t *testing.T) {
	a, err := abi.JSON(strings.NewReader(`[
    {"type":"function","name":"hashTypedData","inputs":[{"type":"bytes32"},{"type":"bytes32"}]},
    {"type":"function","name":"domainSeparator","inputs":[{"type":"string"},{"type":"string"},{"type":"uint256"},{"type":"address"}]},
    {"type":"function","name":"recoverTypedSigner","inputs":[{"type":"bytes32"},{"type":"bytes32"},{"type":"bytes"}]}
  ]`))
	require.NoError(t, err)
	for _, row := range []struct {
		name string
		args []interface{}
		base uint64
	}{
		{"hashTypedData", []interface{}{[32]byte{1}, [32]byte{2}}, 1000},
		{"domainSeparator", []interface{}{"Paxeer", "1", big.NewInt(125), ethcommon.Address{}}, 1000},
		{"recoverTypedSigner", []interface{}{[32]byte{1}, [32]byte{2}, make([]byte, 65)}, 4000},
	} {
		input, err := a.Pack(row.name, row.args...)
		require.NoError(t, err)
		require.Equal(t, crypto.Keccak256([]byte(a.Methods[row.name].Sig))[:4], input[:4])
		require.Equal(t, row.base+3*uint64(len(input)), common.MethodGas(input[:4], len(input)))
	}
}

func TestOracleGasCanonicalCallSizes(t *testing.T) {
	for _, row := range []struct {
		signature string
		length    int
		want      uint64
	}{
		{"aggregate((int256,uint256,uint256)[])", 4 + 64 + 96, 1492},
		{"aggregate((int256,uint256,uint256)[])", 4 + 64 + 32*96, 10420},
		{"submitPrice(bytes32,int256,uint256)", 4 + 96, 33000},
		{"getValidatorPrice(bytes32)", 4 + 32, 1108},
	} {
		t.Run(row.signature+strconv.Itoa(row.length), func(t *testing.T) {
			id := crypto.Keccak256([]byte(row.signature))[:4]
			require.Equal(t, row.want, common.MethodGas(id, row.length))
		})
	}
	require.Equal(t, "0x0000000000000000000000000000000000000903", oracle.OracleAddress)
}

func TestTeeGasCanonicalCallSizes(t *testing.T) {
	// Full calldata includes selector, tuple heads, quote length and byte padding.
	for _, row := range []struct {
		signature         string
		quoteBytes, words int
		base              uint64
	}{
		{"rootCount(uint8)", 0, 1, 1000},
		{"rootOf(uint8,uint256)", 0, 2, 1000},
		{"verify(uint8,bytes)", 291, 3, 46000},
		{"verifyAndExpect(uint8,bytes,bytes32)", 291, 4, 46000},
		{"verify(uint8,bytes)", 421, 3, 46000},
		{"verifyAndExpect(uint8,bytes,bytes32)", 421, 4, 46000},
	} {
		t.Run(row.signature+strconv.Itoa(row.quoteBytes), func(t *testing.T) {
			id := crypto.Keccak256([]byte(row.signature))[:4]
			size := 4 + 32*row.words + 32*((row.quoteBytes+31)/32)
			require.Equal(t, row.base+3*uint64(size), common.MethodGas(id, size))
			require.Equal(t, uint64(math.MaxUint64), common.MethodGas(id, -1))
		})
	}
}

func TestPofqGasCanonicalCallSizes(t *testing.T) {
	for _, row := range []struct {
		signature string
		inputLen  int
		want      uint64
	}{
		{"scoreFill(int256,int256)", 68, 1204},
		{"scoreBatch((int256,int256,uint128)[])", 68, 1204},
		{"scoreBatch((int256,int256,uint128)[])", 260, 1780},
		{"updateRollingScore(uint256,uint256,uint256,uint256,uint16)", 164, 1492},
	} {
		t.Run(row.signature+"/"+strconv.Itoa(row.inputLen), func(t *testing.T) {
			id := crypto.Keccak256([]byte(row.signature))[:4]
			require.Equal(t, row.want, common.MethodGas(id, row.inputLen))
			require.Equal(t, row.want-12, common.MethodGas(id, row.inputLen-4))
		})
	}
}
