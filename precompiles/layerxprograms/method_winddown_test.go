package layerxprograms_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type windDownFixture struct {
	h                                    *harness
	program, account, asset, destination [32]byte
	seed                                 []byte
}

func newWindDownFixture(t *testing.T) windDownFixture {
	t.Helper()
	h := newHarness(t)
	ctx := h.stateDB.Ctx()
	authority, _, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	program := sha256.Sum256([]byte(t.Name()))
	metering := binary.BigEndian.AppendUint32([]byte("LXMR1"), 1)
	for _, coefficient := range []uint64{1, 1, 1, 1, 1, 8, 8, 64, 8} {
		metering = binary.BigEndian.AppendUint64(metering, coefficient)
	}
	metering = binary.BigEndian.AppendUint64(metering, 1)
	metering = append(metering, 1)
	metering = append(metering, authority.Principal[:]...)
	store := ctx.KVStore(h.key)
	store.Set(append([]byte{1, 0, 9}, []byte("progmet/active/v1")...), metering)
	store.Set(append([]byte{1, 0, 9}, binary.BigEndian.AppendUint32([]byte("progmet/history/v1/"), 1)...), metering)

	wasm := []byte{
		0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127,
		3, 3, 2, 0, 1, 5, 3, 1, 0, 1, 7, 41, 3,
		11, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'c', 'a', 'l', 'l', 0, 0,
		14, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'r', 'e', 's', 'e', 'r', 'v', 'e', 0, 1,
		6, 'm', 'e', 'm', 'o', 'r', 'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11,
	}
	deploy, err := lp.EncodeDeployPayload(program, 2, 1, authority.Principal, sha256.Sum256(wasm), nil, wasm)
	require.NoError(t, err)
	method := lp.ProgramsABI.Methods["registryRead"]
	submit := func(activity uint32, payload []byte) lxphost.Receipt {
		t.Helper()
		receipt, err := lp.Submit(ctx, h.executor, lp.NewCallContext(h.caller, &method), activity, payload, [16]byte{})
		require.NoError(t, err)
		require.Zero(t, receipt.ResultCode)
		return receipt
	}
	submit(0x00090001, deploy)
	record, err := h.host.ReadProgram(ctx, program)
	require.NoError(t, err)
	require.Equal(t, authority.Principal, record.Authority)
	require.Equal(t, uint16(2), record.ABIVersion)

	salt := bytes.Repeat([]byte{0x72}, 32)
	asset := sha256.Sum256(append(append([]byte("LX:ASSET:v1"), authority.Actor[:]...), salt...))
	registration := append([]byte{0, 1}, asset[:]...)
	registration = append(registration, salt...)
	registration = append(registration, 1, 'T', 1, 'T', 6)
	registration = binary.BigEndian.AppendUint64(registration, 0)
	registration = binary.BigEndian.AppendUint64(registration, 1000)
	submit(0x00010001, append(registration, 1, 0))
	submit(0x00010004, append([]byte{0, 1}, asset[:]...))
	destination, err := lxphost.DeriveKernelAccount(h.publicKey, 14, asset)
	require.NoError(t, err)
	seed := []byte("wind-down\x00vault")
	payload, err := lp.EncodeAccountPayload(program, 2, asset, seed)
	require.NoError(t, err)
	submit(0x00090006, payload)
	account, err := h.host.ReadProgramAccount(ctx, program, asset, seed)
	require.NoError(t, err)
	require.Equal(t, uint8(2), account.RecordVersion)
	require.NotZero(t, account.RegisteredSequence)
	return windDownFixture{h: h, program: program, account: account.AccountID, asset: asset, destination: destination.ID, seed: seed}
}

func TestProgramsSetExitRouteThenDeprecate(t *testing.T) {
	f := newWindDownFixture(t)
	h := f.h
	ctx := h.stateDB.Ctx()
	const deadline uint64 = 1000
	for _, operation := range []struct {
		method, event string
		args          []interface{}
		payload       []byte
		code          uint16
	}{
		{method: "setExitRoute", event: "ExitRouteSet", args: []interface{}{f.program, f.account, f.asset, f.destination, f.seed}, code: 9},
		{method: "deprecate", event: "ProgramDeprecated", args: []interface{}{f.program, f.program, deadline}, code: 10},
	} {
		var err error
		if operation.method == "setExitRoute" {
			operation.payload, err = lp.EncodeSetExitRoutePayload(f.program, f.account, f.asset, f.destination, f.seed)
			require.NoError(t, err)
			require.Equal(t, byte(1), operation.payload[32])
		} else {
			operation.payload, err = lp.EncodeDeprecatePayload(f.program, f.program, deadline)
			require.NoError(t, err)
			require.Equal(t, byte(2), operation.payload[32])
		}
		reference, _ := h.stateDB.Ctx().CacheContext()
		method := lp.ProgramsABI.Methods[operation.method]
		receipt, err := lp.Submit(reference, h.executor, lp.NewCallContext(h.caller, &method), 0x00090007, operation.payload, [16]byte{})
		require.NoError(t, err)
		require.Len(t, receipt.Events, 1)
		require.Equal(t, operation.code, receipt.Events[0].Code)
		decoded, err := lp.DecodeProgramsReceiptEvent(0x00090007, receipt.Events[0])
		require.NoError(t, err)
		require.Equal(t, operation.event, decoded.Name())
		input, err := lp.ProgramsABI.Pack(operation.method, operation.args...)
		require.NoError(t, err)
		data, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
		require.NoError(t, err, "%s returned %x", operation.method, data)
		require.Empty(t, data)
		require.Equal(t, h.dump(reference), h.dump(h.stateDB.Ctx()))
		logs := h.stateDB.GetAllLogs()
		require.NotEmpty(t, logs)
		log := logs[len(logs)-1]
		definition := lp.ProgramsABI.Events[operation.event]
		require.Equal(t, h.precompile.Address(), log.Address)
		require.Equal(t, []common.Hash{definition.ID, common.Hash(f.program)}, log.Topics)
		values, err := definition.Inputs.NonIndexed().Unpack(log.Data)
		require.NoError(t, err)
		require.Equal(t, decoded.Values(), values)
		if operation.method == "setExitRoute" {
			route, err := h.host.ReadExitRoute(h.stateDB.Ctx(), f.program, f.account)
			require.NoError(t, err)
			require.Equal(t, lxphost.ExitRoute{ProgramID: f.program, AccountID: f.account, AssetID: f.asset, Destination: f.destination, Seed: f.seed}, route)
			require.Equal(t, []interface{}{f.account, f.asset, f.destination}, values)
		} else {
			wind, err := h.host.ReadWindDown(h.stateDB.Ctx(), f.program)
			require.NoError(t, err)
			require.Equal(t, uint8(2), wind.Status)
			require.Equal(t, f.program, wind.ExitProgram)
			require.Equal(t, deadline, wind.Deadline)
			require.Equal(t, receipt.GlobalSequence, wind.EffectiveSequence)
			require.Equal(t, uint16(1), wind.ValueAccountCount)
			require.Zero(t, wind.LiveValueAccountCount)
			require.Equal(t, uint8(1), values[0])
			require.Equal(t, uint8(2), values[1])
		}
	}
	require.Len(t, h.stateDB.GetAllLogs(), 2)
	_, did, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	sequence, err := h.host.ReadIdentitySequence(h.stateDB.Ctx(), did)
	require.NoError(t, err)
	require.Equal(t, uint64(6), sequence)
}

func TestProgramsDeprecateByNonOwnerReverts(t *testing.T) {
	f := newWindDownFixture(t)
	h := f.h
	ctx := h.stateDB.Ctx()
	caller := common.HexToAddress("0xb222")
	bindProgramsPrincipal(t, ctx, h.executor.TestEVMKeeper(), caller, 0x42)
	owner, _, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	other, did, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), caller)
	require.NoError(t, err)
	require.NotEqual(t, owner.Principal, other.Principal)
	const deadline uint64 = 1000
	payload, err := lp.EncodeDeprecatePayload(f.program, f.program, deadline)
	require.NoError(t, err)
	method := lp.ProgramsABI.Methods["deprecate"]
	reference, _ := ctx.CacheContext()
	receipt, err := lp.Submit(reference, h.executor, lp.NewCallContext(caller, &method), 0x00090007, payload, [16]byte{})
	require.ErrorIs(t, err, lxphost.Code(-204))
	require.Equal(t, lxphost.Code(-204), receipt.ResultCode)
	require.Empty(t, receipt.Events)
	before := h.dump(ctx)
	input, err := lp.ProgramsABI.Pack("deprecate", f.program, f.program, deadline)
	require.NoError(t, err)
	data, _, err := h.evm.Call(caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(data)
	require.NoError(t, err)
	require.Equal(t, lp.KernelError(lxphost.Code(-204), "deprecate").Error(), reason)
	require.ErrorIs(t, h.stateDB.GetPrecompileError(), lxphost.Code(-204))
	require.Equal(t, before, h.dump(h.stateDB.Ctx()))
	require.Empty(t, h.stateDB.GetAllLogs())
	_, err = h.host.ReadWindDown(h.stateDB.Ctx(), f.program)
	require.ErrorIs(t, err, lxphost.Code(-7))
	sequence, err := h.host.ReadIdentitySequence(h.stateDB.Ctx(), did)
	require.NoError(t, err)
	require.Zero(t, sequence)
}
