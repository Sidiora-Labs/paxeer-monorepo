package layerxpay

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type budgetTestTuple struct {
	Owner                 [32]byte
	BudgetAccount         [32]byte
	AssetId               [32]byte
	PurposeHash           [32]byte
	PerPeriodLimit        *big.Int
	ConfiguredPeriodLimit *big.Int
	CarryCap              *big.Int
	SpentThisPeriod       *big.Int
	Carried               *big.Int
	PeriodLengthMs        uint64
	PeriodStartMs         uint64
	ExpiryMs              uint64
	RevocationSequence    uint64
	RolloverPolicy        uint8
	Closed                bool
	Revoked               bool
	Delegates             [][32]byte
	SourceAccount         [32]byte
}

func budgetTestArgs(t *testing.T, f *escrowTestFixture, id [32]byte) []interface{} {
	t.Helper()
	account, err := codec.DeriveAccountID([]byte("agent:" + string(f.owner.did) + ":budget:" + hex.EncodeToString(id[:])))
	require.NoError(t, err)
	now := uint64(f.h.db.Ctx().BlockTime().UnixMilli())
	return []interface{}{id, account, f.asset, [32]byte{0xb2}, big.NewInt(300), big.NewInt(50), big.NewInt(200), uint64(60000), now, now + 120000, uint64(0), uint8(2)}
}

func budgetTestRecord(t *testing.T, h *payPrecompileHarness, id [32]byte) paytypes.BudgetRecord {
	t.Helper()
	record, found, err := h.keeper.GetBudget(h.db.Ctx(), id)
	require.NoError(t, err)
	require.True(t, found)
	return record
}

func budgetTestBalance(t *testing.T, h *payPrecompileHarness, account, asset [32]byte, want string) {
	t.Helper()
	balance, err := h.keeper.Host().ReadBalance(h.db.Ctx(), account, asset)
	require.NoError(t, err)
	require.Equal(t, want, balance.String())
}

func budgetTestWrite(t *testing.T, h *payPrecompileHarness, caller common.Address, name, event string, want []interface{}, args ...interface{}) paytypes.BudgetRecord {
	t.Helper()
	a := h.precompile.GetABI()
	input, err := a.Pack(name, args...)
	require.NoError(t, err)
	before := h.keeper.ExportGenesis(h.db.Ctx())
	logCount := len(h.db.GetAllLogs())
	ordinalKey := crypto.Keccak256Hash([]byte("layerxpay/call-ordinal/v1"))
	ordinal := h.db.GetTransientState(h.precompile.Address(), ordinalKey)
	output, _, err := h.evm.StaticCall(caller, h.precompile.Address(), input, 2000000)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(output)
	require.NoError(t, err)
	require.Contains(t, reason, "LXP_UNAUTHORIZED_DEBIT")
	require.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
	require.Len(t, h.db.GetAllLogs(), logCount)
	require.Equal(t, ordinal, h.db.GetTransientState(h.precompile.Address(), ordinalKey))
	if name == "budgetDelegateSpend" || name == "budgetPull" {
		var selector [4]byte
		copy(selector[:], a.Methods[name].ID)
		key := paytypes.DeriveIdempotencyKey([32]byte(h.db.Ctx().EVMTxHash()), uint32(ordinal.Big().Uint64()), selector)
		require.NotEqual(t, [32]byte{}, key)
		var payload []byte
		if name == "budgetPull" {
			payload, err = paytypes.EncodeBudgetPull(args[0].([32]byte), args[1].([32]byte), payU128(args[2]), key, args[3].([]byte))
		} else {
			payload, err = paytypes.EncodeBudgetDelegateSpend(args[0].([32]byte), args[1].([32]byte), payU128(args[2]), key)
		}
		require.NoError(t, err)
		require.Equal(t, key[:], payload[82:114])
		branch, _ := h.db.Ctx().CacheContext()
		receipt, err := h.keeper.Submit(branch, caller, name, payload, key)
		require.NoError(t, err)
		require.Zero(t, receipt.ResultCode)
		expected := h.keeper.ExportGenesis(branch)
		output, _, err = h.evm.Call(caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
		require.NoError(t, err, "%x", output)
		require.Equal(t, expected, h.keeper.ExportGenesis(h.db.Ctx()))
	} else {
		output, _, err = h.evm.Call(caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
		require.NoError(t, err, "%x", output)
	}
	require.Empty(t, output)
	require.Equal(t, ordinal.Big().Uint64()+1, h.db.GetTransientState(h.precompile.Address(), ordinalKey).Big().Uint64())
	logs := h.db.GetAllLogs()
	require.Len(t, logs, logCount+1)
	log := logs[logCount]
	require.Equal(t, h.precompile.Address(), log.Address)
	require.Equal(t, []common.Hash{a.Events[event].ID, common.Hash(args[0].([32]byte))}, log.Topics)
	encoded, err := a.Events[event].Inputs.NonIndexed().Pack(want...)
	require.NoError(t, err)
	require.Equal(t, encoded, log.Data)
	return budgetTestRecord(t, h, args[0].([32]byte))
}

func budgetTestView(t *testing.T, h *payPrecompileHarness, name string, id [32]byte) []interface{} {
	t.Helper()
	snapshot := h.db.Snapshot()
	defer h.db.RevertToSnapshot(snapshot)
	a := h.precompile.GetABI()
	input, err := a.Pack(name, id)
	require.NoError(t, err)
	before := h.keeper.ExportGenesis(h.db.Ctx())
	logs := len(h.db.GetAllLogs())
	output, _, err := h.evm.StaticCall(h.caller, h.precompile.Address(), input, 2000000)
	require.NoError(t, err, "%x", output)
	require.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
	require.Len(t, h.db.GetAllLogs(), logs)
	values, err := a.Unpack(name, output)
	require.NoError(t, err)
	return values
}

func budgetTestRevert(t *testing.T, h *payPrecompileHarness, caller common.Address, name string, args ...interface{}) {
	t.Helper()
	a := h.precompile.GetABI()
	input, err := a.Pack(name, args...)
	require.NoError(t, err)
	before := h.keeper.ExportGenesis(h.db.Ctx())
	logs := len(h.db.GetAllLogs())
	ordinalKey := crypto.Keccak256Hash([]byte("layerxpay/call-ordinal/v1"))
	ordinal := h.db.GetTransientState(h.precompile.Address(), ordinalKey)
	_, _, err = h.evm.Call(caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
	require.Len(t, h.db.GetAllLogs(), logs)
	require.Equal(t, ordinal, h.db.GetTransientState(h.precompile.Address(), ordinalKey))
}

func TestPrecompileBudgetLifecycle(t *testing.T) {
	f := newEscrowTestFixture(t)
	h := f.h
	h.keeper.SetParams(h.db.Ctx(), paytypes.Params{BudgetEnabled: true})
	id := [32]byte{0xb1}
	args := budgetTestArgs(t, f, id)
	r := budgetTestWrite(t, h, f.owner.caller, "budgetCreate", "BudgetCreated", []interface{}{f.asset, big.NewInt(200)}, args...)
	require.Equal(t, f.owner.account, r.Owner)
	require.Equal(t, args[1], r.BudgetAccount)
	values := budgetTestView(t, h, "getBudget", id)
	got := abi.ConvertType(values[0], new(budgetTestTuple)).(*budgetTestTuple)
	wantTuple := &budgetTestTuple{
		Owner: f.owner.account, BudgetAccount: args[1].([32]byte), AssetId: f.asset, PurposeHash: args[3].([32]byte),
		PerPeriodLimit: big.NewInt(300), ConfiguredPeriodLimit: big.NewInt(300), CarryCap: big.NewInt(50),
		SpentThisPeriod: big.NewInt(0), Carried: big.NewInt(0), PeriodLengthMs: 60000, PeriodStartMs: args[8].(uint64),
		ExpiryMs: args[9].(uint64), RolloverPolicy: 2, Delegates: [][32]byte{},
	}
	contract := h.precompile.GetABI()
	wantABI, err := contract.Methods["getBudget"].Outputs.Pack(wantTuple)
	require.NoError(t, err)
	gotABI, err := contract.Methods["getBudget"].Outputs.Pack(got)
	require.NoError(t, err)
	require.Equal(t, wantABI, gotABI)
	require.Equal(t, big.NewInt(200), budgetTestView(t, h, "budgetRemaining", id)[0])
	budgetTestWrite(t, h, f.owner.caller, "budgetFund", "BudgetFunded", []interface{}{big.NewInt(50)}, id, big.NewInt(50))
	expiry := args[9].(uint64) + 60000
	r = budgetTestWrite(t, h, f.owner.caller, "budgetAmend", "BudgetAmended", []interface{}{big.NewInt(280), expiry}, id, big.NewInt(280), big.NewInt(30), expiry, uint8(2))
	require.Equal(t, codec.U128{Lo: 280}, r.ConfiguredPeriodLimit)
	require.Equal(t, codec.U128{Lo: 30}, r.CarryCap)
	r = budgetTestWrite(t, h, f.owner.caller, "budgetDelegateAdd", "BudgetDelegateAdded", []interface{}{f.beneficiary.actor}, id, f.beneficiary.actor)
	require.Equal(t, [][32]byte{f.beneficiary.actor}, r.Delegates)
	got = abi.ConvertType(budgetTestView(t, h, "getBudget", id)[0], new(budgetTestTuple)).(*budgetTestTuple)
	require.Equal(t, r.Delegates, got.Delegates)
	budgetTestRevert(t, h, f.arbiter.caller, "budgetSpend", id, f.beneficiary.account, big.NewInt(20))
	r = budgetTestWrite(t, h, f.owner.caller, "budgetSpend", "BudgetSpent", []interface{}{f.beneficiary.account, big.NewInt(20)}, id, f.beneficiary.account, big.NewInt(20))
	require.Equal(t, codec.U128{Lo: 20}, r.SpentThisPeriod)
	r = budgetTestWrite(t, h, f.owner.caller, "budgetDelegateRemove", "BudgetDelegateRemoved", []interface{}{f.beneficiary.actor}, id, f.beneficiary.actor)
	require.Empty(t, r.Delegates)
	r = budgetTestWrite(t, h, f.owner.caller, "budgetDefund", "BudgetDefunded", []interface{}{big.NewInt(30), big.NewInt(220)}, id, big.NewInt(30))
	require.Equal(t, codec.U128{Lo: 220}, r.PerPeriodLimit)
	require.Equal(t, big.NewInt(200), budgetTestView(t, h, "budgetRemaining", id)[0])
	r = budgetTestWrite(t, h, f.owner.caller, "budgetClose", "BudgetClosed", []interface{}{big.NewInt(200), uint64(1)}, id, uint64(1))
	require.True(t, r.Closed)
	require.False(t, r.Revoked)
	require.Equal(t, uint64(1), r.RevocationSequence)
	require.Zero(t, budgetTestView(t, h, "budgetRemaining", id)[0].(*big.Int).Sign())
	budgetTestBalance(t, h, f.owner.account, f.asset, "980")
	budgetTestBalance(t, h, f.beneficiary.account, f.asset, "20")
	budgetTestBalance(t, h, r.BudgetAccount, f.asset, "0")
	budgetTestRevert(t, h, f.owner.caller, "budgetSpend", id, f.beneficiary.account, big.NewInt(1))
	id = [32]byte{0xb3}
	args = budgetTestArgs(t, f, id)
	budgetTestWrite(t, h, f.owner.caller, "budgetCreate", "BudgetCreated", []interface{}{f.asset, big.NewInt(200)}, args...)
	budgetTestWrite(t, h, f.owner.caller, "budgetSpend", "BudgetSpent", []interface{}{f.beneficiary.account, big.NewInt(10)}, id, f.beneficiary.account, big.NewInt(10))
	r = budgetTestWrite(t, h, f.owner.caller, "budgetRevoke", "BudgetRevoked", []interface{}{big.NewInt(190), uint64(3)}, id, uint64(3))
	require.True(t, r.Revoked)
	require.False(t, r.Closed)
	require.Equal(t, uint64(3), r.RevocationSequence)
	got = abi.ConvertType(budgetTestView(t, h, "getBudget", id)[0], new(budgetTestTuple)).(*budgetTestTuple)
	require.True(t, got.Revoked)
	require.Equal(t, uint64(3), got.RevocationSequence)
	budgetTestBalance(t, h, f.owner.account, f.asset, "970")
	budgetTestBalance(t, h, f.beneficiary.account, f.asset, "30")
	budgetTestBalance(t, h, r.BudgetAccount, f.asset, "0")
	budgetTestRevert(t, h, f.owner.caller, "budgetSpend", id, f.beneficiary.account, big.NewInt(1))
	require.Zero(t, budgetTestView(t, h, "budgetRemaining", [32]byte{0xff})[0].(*big.Int).Sign())
	budgetTestRevert(t, h, f.owner.caller, "getBudget", [32]byte{0xff})
}

func budgetTestGrant(t *testing.T, f *escrowTestFixture, purpose [32]byte) ([]byte, [32]byte) {
	t.Helper()
	body := append([]byte(nil), f.owner.account[:]...)
	body = append(body, f.beneficiary.account[:]...)
	body = append(body, f.asset[:]...)
	perDraw, allowance := (codec.U128{Lo: 40}).Bytes(), (codec.U128{Lo: 100}).Bytes()
	body = append(body, perDraw[:]...)
	body = append(body, allowance[:]...)
	body = append(body, 1)
	body = binary.BigEndian.AppendUint64(body, 60000)
	body = binary.BigEndian.AppendUint64(body, uint64(f.h.db.Ctx().BlockTime().UnixMilli())+120000)
	body = append(body, purpose[:]...)
	body = append(body, 0)
	body = append(body, make([]byte, 32)...)
	body = binary.BigEndian.AppendUint64(body, 0)
	body = append(body, f.owner.public[:]...)
	id, err := codec.DomainHash(codec.DomainAuthorityHash, append([]byte("LXP:GRANT:v1"), body...))
	require.NoError(t, err)
	grant := append(append([]byte(nil), id[:]...), body...)
	grant = append(grant, ed25519.Sign(f.owner.private, id[:])...)
	require.Len(t, grant, paytypes.PayerGrantBytes)
	return grant, id
}

func TestPrecompileBudgetNativeAndPull(t *testing.T) {
	f := newEscrowTestFixture(t)
	h := f.h
	h.keeper.SetParams(h.db.Ctx(), paytypes.Params{BudgetEnabled: true})
	fundedOwner := f.owner
	ownerAsset := f.asset
	f.owner = escrowTestRegister(t, h, [32]byte{0xc6}, common.HexToAddress("0xa2284"), true)
	f.beneficiary = escrowTestRegister(t, h, [32]byte{0xc7}, common.HexToAddress("0xa2285"), true)
	salt := [32]byte{0xc0}
	f.asset = sha256.Sum256(append(append([]byte("LX:ASSET:v1"), f.owner.actor[:]...), salt[:]...))
	f.owner.account = escrowTestMain(t, h, f.owner, ownerAsset)
	f.beneficiary.account = escrowTestMain(t, h, f.beneficiary, f.asset)
	registration := append(append([]byte{0, 1}, f.asset[:]...), salt[:]...)
	registration = append(registration, 1, 'N', 1, 'N', 6)
	registration = binary.BigEndian.AppendUint64(registration, 0)
	registration = binary.BigEndian.AppendUint64(registration, 1000)
	registration = append(registration, 1, 0)
	escrowTestExecute(t, h, f.owner, 0x00010001, registration, [32]byte{0xc4})
	escrowTestExecute(t, h, f.owner, 0x00010004, append([]byte{0, 1}, f.asset[:]...), [32]byte{0xc5})
	sourceName, source, err := lxphost.AssetAccountID(f.owner.public, f.asset)
	require.NoError(t, err)
	mint := append(append([]byte{0, 1}, f.asset[:]...), source[:]...)
	mint = binary.BigEndian.AppendUint64(mint, 0)
	mint = binary.BigEndian.AppendUint64(mint, 400)
	escrowTestExecute(t, h, f.owner, 0x0001000a, mint, [32]byte{0xc1})
	sourceBefore, err := h.keeper.Host().ReadAccount(h.db.Ctx(), sourceName, source)
	require.NoError(t, err)
	id := [32]byte{0xc2}
	args := budgetTestArgs(t, f, id)
	args = append(args, source, sourceBefore.NextSequence)
	r := budgetTestWrite(t, h, f.owner.caller, "budgetCreateNative", "BudgetCreated", []interface{}{f.asset, big.NewInt(200)}, args...)
	require.True(t, r.NativeSource)
	require.Equal(t, source, r.SourceAccount)
	budgetTestWrite(t, h, f.owner.caller, "budgetFundNative", "BudgetFunded", []interface{}{big.NewInt(50)}, id, big.NewInt(50), sourceBefore.NextSequence+1)
	sourceAfter, err := h.keeper.Host().ReadAccount(h.db.Ctx(), sourceName, source)
	require.NoError(t, err)
	require.Equal(t, sourceBefore.NextSequence+2, sourceAfter.NextSequence)
	budgetTestBalance(t, h, source, f.asset, "150")
	budgetTestBalance(t, h, fundedOwner.account, ownerAsset, "1000")
	budgetTestBalance(t, h, f.owner.account, ownerAsset, "0")
	got := abi.ConvertType(budgetTestView(t, h, "getBudget", id)[0], new(budgetTestTuple)).(*budgetTestTuple)
	require.Equal(t, source, got.SourceAccount)
	require.Equal(t, big.NewInt(250), budgetTestView(t, h, "budgetRemaining", id)[0])
	budgetTestRevert(t, h, f.owner.caller, "budgetFundNative", id, big.NewInt(10), sourceBefore.NextSequence)
	f = newEscrowTestFixture(t)
	h = f.h
	h.keeper.SetParams(h.db.Ctx(), paytypes.Params{BudgetEnabled: true})
	args = budgetTestArgs(t, f, id)
	r = budgetTestWrite(t, h, f.owner.caller, "budgetCreate", "BudgetCreated", []interface{}{f.asset, big.NewInt(200)}, args...)
	budgetTestWrite(t, h, f.owner.caller, "budgetFund", "BudgetFunded", []interface{}{big.NewInt(50)}, id, big.NewInt(50))
	budgetTestWrite(t, h, f.owner.caller, "budgetDelegateAdd", "BudgetDelegateAdded", []interface{}{f.beneficiary.actor}, id, f.beneficiary.actor)
	capability := make([]byte, 195)
	capability[1] = 1
	copy(capability[2:34], f.beneficiary.actor[:])
	copy(capability[34:66], f.asset[:])
	copy(capability[66:98], f.beneficiary.account[:])
	copy(capability[98:130], r.PurposeHash[:])
	maximumSpend, maximumTotal := (codec.U128{Lo: 25}).Bytes(), (codec.U128{Lo: 100}).Bytes()
	copy(capability[130:146], maximumSpend[:])
	copy(capability[146:162], maximumTotal[:])
	binary.BigEndian.PutUint64(capability[178:186], args[9].(uint64))
	capabilityKey := paytypes.KernelKVKey(paytypes.BudgetModuleID, append(append([]byte("budget-delegate:"), id[:]...), f.beneficiary.actor[:]...))
	h.db.Ctx().KVStore(escrowTestHostKey(t, h)).Set(capabilityKey, capability)
	for i := 0; i < 2; i++ {
		r = budgetTestWrite(t, h, f.beneficiary.caller, "budgetDelegateSpend", "BudgetSpent", []interface{}{f.beneficiary.account, big.NewInt(10)}, id, f.beneficiary.account, big.NewInt(10))
		require.Equal(t, codec.U128{Lo: uint64((i + 1) * 10)}, r.SpentThisPeriod)
	}
	consumed := h.db.Ctx().KVStore(escrowTestHostKey(t, h)).Get(capabilityKey)
	require.Equal(t, (codec.U128{Lo: 20}).Bytes(), [16]byte(consumed[162:178]))
	budgetTestRevert(t, h, f.beneficiary.caller, "budgetDelegateSpend", id, f.beneficiary.account, big.NewInt(26))
	grant, grantID := budgetTestGrant(t, f, r.PurposeHash)
	escrowTestExecute(t, h, f.owner, 0x00010007, grant, [32]byte{0xc3})
	for i := 0; i < 2; i++ {
		r = budgetTestWrite(t, h, f.beneficiary.caller, "budgetPull", "BudgetSpent", []interface{}{f.beneficiary.account, big.NewInt(30)}, id, f.beneficiary.account, big.NewInt(30), grant)
		require.Equal(t, codec.U128{Lo: uint64(50 + i*30)}, r.SpentThisPeriod)
	}
	state, err := h.keeper.Host().ReadPayerGrant(h.db.Ctx(), grantID)
	require.NoError(t, err)
	require.Equal(t, "60", state.DrawnTotal.String())
	require.Equal(t, "60", state.DrawnThisPeriod.String())
	require.Equal(t, big.NewInt(170), budgetTestView(t, h, "budgetRemaining", id)[0])
	budgetTestBalance(t, h, r.BudgetAccount, f.asset, "170")
	budgetTestBalance(t, h, f.beneficiary.account, f.asset, "80")
	budgetTestRevert(t, h, f.beneficiary.caller, "budgetPull", id, f.beneficiary.account, big.NewInt(30), grant[:len(grant)-1])
	ctx := h.db.Ctx()
	h.db.WithCtx(ctx.WithEVMTxHash(common.Hash{}))
	budgetTestRevert(t, h, f.beneficiary.caller, "budgetDelegateSpend", id, f.beneficiary.account, big.NewInt(10))
	budgetTestRevert(t, h, f.beneficiary.caller, "budgetPull", id, f.beneficiary.account, big.NewInt(10), grant)
	h.db.WithCtx(ctx)
	snapshot := h.db.Snapshot()
	input, err := h.precompile.GetABI().Pack("budgetDelegateSpend", id, f.beneficiary.account, big.NewInt(10))
	require.NoError(t, err)
	before := h.keeper.ExportGenesis(h.db.Ctx())
	logCount := len(h.db.GetAllLogs())
	_, _, err = h.evm.Call(f.beneficiary.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
	require.NoError(t, err)
	h.db.RevertToSnapshot(snapshot)
	require.Equal(t, before, h.keeper.ExportGenesis(h.db.Ctx()))
	require.Len(t, h.db.GetAllLogs(), logCount)
}
