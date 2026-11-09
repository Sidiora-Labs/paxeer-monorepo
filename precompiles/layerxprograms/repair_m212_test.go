package layerxprograms_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"os"
	"regexp"
	"testing"

	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func repairM212Merge(t *testing.T, h *harness) *app.PrecompileKeepers {
	t.Helper()
	application := testkeeper.EVMTestApp
	previousHost := application.LayerXHost
	defer func() { application.LayerXHost = previousHost }()
	application.LayerXHost = h.host
	keepers := app.NewPrecompileKeepers(application)
	var contract lp.HostKeepers = keepers
	require.Same(t, h.host, contract.LayerXHost())
	require.Same(t, &application.XWebKeeper, contract.XWebK())
	merged, err := lp.NewMergePrecompile(keepers, precompiles.MergeUpgrade)
	require.NoError(t, err)
	require.Equal(t, common.HexToAddress("0x000000000000000000000000000000000000101d"), merged.Address())
	h.precompile = merged.(*pcommon.Precompile)
	h.executor = h.precompile.GetExecutor().(*lp.PrecompileExecutor)
	h.evm.SetPrecompiles(vm.PrecompiledContracts{lp.Address: h.precompile})
	return keepers
}

func repairM212SetFrozen(t *testing.T, h *harness, account [32]byte, frozen bool) {
	t.Helper()
	key := append([]byte{6}, account[:]...)
	row := bytes.Clone(h.stateDB.Ctx().KVStore(h.key).Get(key))
	require.GreaterOrEqual(t, len(row), 40)
	offset := 4 + 32 + 4 + int(binary.BigEndian.Uint32(row[36:40])) + 1 + 16 + 4 + 32 + 1 + 8 + 8
	require.Greater(t, len(row), offset)
	row[offset] = 0
	if frozen {
		row[offset] = 1
	}
	h.stateDB.Ctx().KVStore(h.key).Set(key, row)
}

func repairM212OutputSemantics(t *testing.T) {
	t.Helper()
	h, program, asset := registerAccountFixture(t)
	repairM212Merge(t, h)
	seed := []byte("repair-m212-value")
	conservationCall(t, h, "registerAccount", program, uint8(2), asset, seed)
	registered, err := h.host.ReadProgramAccount(h.stateDB.Ctx(), program, asset, seed)
	require.NoError(t, err)
	authority, _, err := lp.PrincipalOf(h.stateDB.Ctx(), h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	require.NotEqual(t, [32]byte{}, authority.Principal)
	for _, frozen := range []bool{false, true, false} {
		repairM212SetFrozen(t, h, registered.AccountID, frozen)
		value, err := h.host.ReadValueAccount(h.stateDB.Ctx(), program, registered.AccountID)
		require.NoError(t, err)
		require.Equal(t, frozen, value.Frozen)
		values := repair251View(t, h, "getValueAccount", program, registered.AccountID)
		require.Zero(t, value.Balance.BigInt().Cmp(values[0].(*big.Int)))
		require.Equal(t, value.Frozen, values[1])
	}
	budget := [7]uint64{19, 23, 29, 31, 37, 41, 43}
	for revision := uint32(1); revision <= 2; revision++ {
		raw, err := h.host.ReadFeeSchedule(h.stateDB.Ctx())
		require.NoError(t, err)
		raw = bytes.Clone(raw)
		binary.BigEndian.PutUint32(raw[5:9], revision)
		raw[65] = byte(0x51 + revision)
		h.stateDB.Ctx().KVStore(h.key).Set(append([]byte{1, 0, 9}, []byte("progfee/active/v1")...), raw)
		schedule, err := lp.DecodeFeeScheduleRecord(raw)
		require.NoError(t, err)
		current := repair251View(t, h, "getCurrentFeeSchedule")
		quote := repair251View(t, h, "quoteCallFee", budget)
		require.Equal(t, schedule.Version, quote[0])
		require.Equal(t, schedule.Asset, quote[2])
		require.Equal(t, current[2], quote[2])
		require.NotEqual(t, sha256.Sum256(raw), quote[2])
		ceiling := new(big.Int)
		for i, price := range schedule.Prices {
			ceiling.Add(ceiling, new(big.Int).Mul(new(big.Int).SetUint64(price), new(big.Int).SetUint64(budget[i])))
		}
		require.Zero(t, ceiling.Cmp(quote[1].(*big.Int)))
	}
}

func TestRepairM_212_SolidityOutputSemantics(t *testing.T) {
	source, err := os.ReadFile("LayerXPrograms.sol")
	require.NoError(t, err)
	require.Regexp(t, regexp.MustCompile(`(?s)function getValueAccount\(bytes32 programId, bytes32 accountId\)\s*external view returns \(uint128 balance, bool frozen\)`), string(source))
	require.Regexp(t, regexp.MustCompile(`(?s)function quoteCallFee\(uint64\[7\] calldata budget\)\s*external\s*view\s*returns \(uint32 version, uint128 executionCeiling, bytes32 feeAsset\)`), string(source))
	repairM212OutputSemantics(t)
}

func TestRepairM_212_ABIOutputSemantics(t *testing.T) {
	source, err := os.ReadFile("abi.json")
	require.NoError(t, err)
	contract, err := abi.JSON(bytes.NewReader(source))
	require.NoError(t, err)
	require.Equal(t, "frozen", contract.Methods["getValueAccount"].Outputs[1].Name)
	require.Equal(t, "bool", contract.Methods["getValueAccount"].Outputs[1].Type.String())
	require.Equal(t, "feeAsset", contract.Methods["quoteCallFee"].Outputs[2].Name)
	require.Equal(t, "bytes32", contract.Methods["quoteCallFee"].Outputs[2].Type.String())
	require.Equal(t, contract.Methods["getValueAccount"], lp.ProgramsABI.Methods["getValueAccount"])
	require.Equal(t, contract.Methods["quoteCallFee"], lp.ProgramsABI.Methods["quoteCallFee"])
	repairM212OutputSemantics(t)
}

func TestRepairM_212_BankParityFrozen(t *testing.T) {
	TestProgramsValueAccountMatchesBankBalance(t)
}

func TestRepairM_212_RegistryAddressType(t *testing.T) {
	var address common.Address = lp.Address
	require.Equal(t, common.HexToAddress("0x000000000000000000000000000000000000101d"), address)
	log, program := transfer(t)
	require.Equal(t, address, log.Address)
	event, err := lp.DecodeProgramTransferred(log)
	require.NoError(t, err)
	require.Equal(t, common.Hash(program), event.ProgramID)
	wrong := *log
	wrong.Address = common.HexToAddress("0x101e")
	_, err = lp.DecodeProgramTransferred(&wrong)
	require.Error(t, err)
}

func TestRepairM_212_RegistryHostKeepers(t *testing.T) {
	h := newHarness(t)
	keepers := repairM212Merge(t, h)
	require.NotNil(t, keepers.XWebK())
	require.Equal(t, lp.PrecompileName, h.precompile.GetName())
	require.Equal(t, lp.ProgramsABI, h.precompile.GetABI())
	record, err := h.host.ReadFeeSchedule(h.stateDB.Ctx())
	require.NoError(t, err)
	quote := repair251View(t, h, "quoteCallFee", [7]uint64{1, 1, 1, 1, 1, 1, 1})
	require.Equal(t, [32]byte(record[65:97]), quote[2])
}

func TestRepairM_212_RetiredCustodyWiring(t *testing.T) {
	t.Run("governed_fee_denom", lp.TestProgramsCallRecordsFirstWebRequestFeeDenomFromAssetMapping)
	t.Run("ungoverned_fee_denom_refused", lp.TestProgramsCallRefusesFirstWebRequestWithoutAssetMapping)
	h, args, payload, guest := repair251Responder(t)
	keepers := repairM212Merge(t, h)
	// This adapter has no custody keeper at all; the constructor and transfer
	// resolve the real registered host asset to its native bank mirror denom.
	application := testkeeper.EVMTestApp
	previousHost, previousCustody := application.LayerXHost, application.LayerXCustodyKeeper
	defer func() {
		application.LayerXHost = previousHost
		application.LayerXCustodyKeeper = previousCustody
	}()
	application.LayerXHost = h.host
	application.LayerXCustodyKeeper = nil
	keepers = app.NewPrecompileKeepers(application)
	merged, err := lp.NewMergePrecompile(keepers, precompiles.MergeUpgrade)
	require.NoError(t, err)
	h.precompile = merged.(*pcommon.Precompile)
	h.executor = h.precompile.GetExecutor().(*lp.PrecompileExecutor)
	h.evm.SetPrecompiles(vm.PrecompiledContracts{lp.Address: h.precompile})
	require.Nil(t, keepers.LayerXCustodyK())
	receipt, reference := repair251Receipt(t, h, args, payload)
	leg := receipt.Transfers[0]
	registered, err := h.host.ReadAsset(h.stateDB.Ctx(), leg.Asset)
	require.NoError(t, err)
	require.Equal(t, leg.Asset, registered.AssetID)
	denom := ledgertypes.MirrorDenom(registered.AssetID)
	parsed, err := ledgertypes.ParseMirrorDenom(denom)
	require.NoError(t, err)
	require.Equal(t, registered.AssetID, parsed)
	bank := h.executor.TestEVMKeeper().BankKeeper()
	from, to := lxphost.BankAddress(leg.From), lxphost.BankAddress(leg.To)
	require.NoError(t, bank.AddCoins(h.stateDB.Ctx(), from, sdk.NewCoins(sdk.NewInt64Coin(denom, 100)), true))
	beforeFrom := bank.GetBalance(h.stateDB.Ctx(), from, denom).Amount
	beforeTo := bank.GetBalance(h.stateDB.Ctx(), to, denom).Amount
	input, err := lp.ProgramsABI.Pack("call", args...)
	require.NoError(t, err)
	output, _, err := h.evm.Call(h.caller, lp.Address, input, 20000000, uint256.NewInt(0))
	reason, _ := abi.UnpackRevert(output)
	require.NoError(t, err, "%s", reason)
	require.Equal(t, beforeFrom.Sub(leg.Amount), bank.GetBalance(h.stateDB.Ctx(), from, denom).Amount)
	require.Equal(t, beforeTo.Add(leg.Amount), bank.GetBalance(h.stateDB.Ctx(), to, denom).Amount)
	require.Equal(t, h.dump(reference), h.dump(h.stateDB.Ctx()))
	decoded, err := lp.DecodeProgramExecutionResult(output, 64, repair251TransferRoot(t, receipt))
	require.NoError(t, err)
	require.Equal(t, guest, decoded.GuestResponse)
}

func TestRepairM_212_CallTestResponseOracle(t *testing.T) {
	assertProgramsKnownBoundedResponse(t)
	t.Run("fee_refusals", TestProgramsCallFeeLimitBelowFeeReverts)
}

func TestRepairM_212_QuoteTestDigest(t *testing.T) {
	TestRepair_251_QuoteAsset(t)
	lp.TestProgramsQuoteCallFeeMatchesKernelCeiling(t)
}
