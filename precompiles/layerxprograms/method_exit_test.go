package layerxprograms_test

import (
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type exitValueFixture struct {
	h           *harness
	program     [32]byte
	account     [32]byte
	asset       [32]byte
	destination [32]byte
}

func syncExitValuePaymentSequence(t *testing.T, h *harness, asset [32]byte) {
	t.Helper()
	ctx := h.stateDB.Ctx()
	_, did, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	sequence, err := h.host.ReadIdentitySequence(ctx, did)
	require.NoError(t, err)
	name, id, err := lxphost.MainAccountID(h.publicKey)
	require.NoError(t, err)
	store := ctx.KVStore(h.key)
	accountKey := append([]byte{0x06}, id[:]...)
	row := append([]byte(nil), store.Get(accountKey)...)
	sequenceOffset := 4 + 32 + 4 + len(name) + 1 + 16 + 4 + 32 + 1
	require.GreaterOrEqual(t, len(row), sequenceOffset+8)
	binary.BigEndian.PutUint64(row[sequenceOffset:sequenceOffset+8], sequence)
	store.Set(accountKey, row)
	account, err := h.host.ReadAccount(ctx, name, id)
	require.NoError(t, err)
	require.Equal(t, sequence, account.NextSequence)
	require.Equal(t, asset, account.AssetID)
}

func newExitValueFixture(t *testing.T, balance uint64) exitValueFixture {
	t.Helper()
	wind := newWindDownFixture(t)
	h := wind.h
	f := exitValueFixture{h: h, program: wind.program, account: wind.account, asset: wind.asset, destination: wind.destination}
	method := lp.ProgramsABI.Methods["registryRead"]
	mint := append([]byte{0, 1}, f.asset[:]...)
	mint = append(mint, f.destination[:]...)
	mint = binary.BigEndian.AppendUint64(mint, 0)
	mint = binary.BigEndian.AppendUint64(mint, 1000)
	receipt, err := lp.Submit(h.stateDB.Ctx(), h.executor, lp.NewCallContext(h.caller, &method), 0x0001000a, mint, [16]byte{})
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	programsTransferAccount(t, h, h.publicKey, h.occupancyAsset)
	syncExitValuePaymentSequence(t, h, h.occupancyAsset)
	fund, err := lp.EncodeTransferPayload(f.program, []lp.TransferLeg{{From: f.destination, Asset: f.asset, To: f.account, Amount: new(big.Int).SetUint64(balance)}})
	require.NoError(t, err)
	receipt, err = lp.Submit(h.stateDB.Ctx(), h.executor, lp.NewCallContext(h.caller, &method), 0x00090005, fund, [16]byte{})
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	for _, operation := range []struct {
		name string
		args []interface{}
	}{
		{"setExitRoute", []interface{}{f.program, f.account, f.asset, f.destination, wind.seed}},
		{"deprecate", []interface{}{f.program, f.program, uint64(1000)}},
		{"tombstone", []interface{}{f.program}},
	} {
		input, err := lp.ProgramsABI.Pack(operation.name, operation.args...)
		require.NoError(t, err)
		output, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
		require.NoError(t, err, "%s returned %x", operation.name, output)
		require.Empty(t, output)
	}
	status, err := h.host.ReadWindDown(h.stateDB.Ctx(), f.program)
	require.NoError(t, err)
	require.Equal(t, uint8(3), status.Status)
	require.Equal(t, uint16(1), status.LiveValueAccountCount)
	record, err := h.host.ReadProgram(h.stateDB.Ctx(), f.program)
	require.NoError(t, err)
	require.False(t, record.Active)
	syncExitValuePaymentSequence(t, h, h.occupancyAsset)
	f.assertBalances(t, int64(balance), int64(1000-balance))
	return f
}

func (f exitValueFixture) assertBalances(t *testing.T, source, destination int64) {
	t.Helper()
	balance, err := f.h.host.ReadBalance(f.h.stateDB.Ctx(), f.account, f.asset)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(source), balance)
	valueAccount, err := f.h.host.ReadValueAccount(f.h.stateDB.Ctx(), f.program, f.account)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(source), valueAccount.Balance)
	target, err := f.h.host.ReadBalance(f.h.stateDB.Ctx(), f.destination, f.asset)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(destination), target)
}

func TestProgramsExitValueDrainsToRoute(t *testing.T) {
	for _, balance := range []uint64{7, 250} {
		t.Run(big.NewInt(int64(balance)).String(), func(t *testing.T) {
			f := newExitValueFixture(t, balance)
			logsBefore := len(f.h.stateDB.GetAllLogs())
			input, err := lp.ProgramsABI.Pack("exitValue", f.program, f.account)
			require.NoError(t, err)
			out, _, err := f.h.evm.Call(f.h.caller, f.h.precompile.Address(), input, 2000000, uint256.NewInt(0))
			if err != nil {
				reason, decodeErr := abi.UnpackRevert(out)
				require.NoError(t, decodeErr, "exitValue result: %x", out)
				require.NoError(t, err, "exitValue reverted: %s", reason)
			}
			require.Empty(t, out)
			f.assertBalances(t, 0, 1000)
			logs := f.h.stateDB.GetAllLogs()[logsBefore:]
			require.Len(t, logs, 1)
			definition := lp.ProgramsABI.Events["ProgramValueExited"]
			require.Equal(t, lp.Address, logs[0].Address)
			require.Equal(t, []common.Hash{definition.ID, common.Hash(f.program)}, logs[0].Topics)
			values, err := definition.Inputs.NonIndexed().Unpack(logs[0].Data)
			require.NoError(t, err)
			require.Len(t, values, 2)
			require.Equal(t, f.account, values[0])
			require.NotEqual(t, [32]byte{}, values[1])
			status, err := f.h.host.ReadWindDown(f.h.stateDB.Ctx(), f.program)
			require.NoError(t, err)
			require.Equal(t, uint16(0), status.LiveValueAccountCount)
		})
	}
}

func TestProgramsExitValueBoundedRefusesBelowBalance(t *testing.T) {
	for _, tc := range []struct {
		balance uint64
		maximum int64
	}{
		{7, 3},
		{250, 249},
	} {
		t.Run(big.NewInt(int64(tc.balance)).String(), func(t *testing.T) {
			f := newExitValueFixture(t, tc.balance)
			before := f.h.dump(f.h.stateDB.Ctx())
			logsBefore := f.h.stateDB.GetAllLogs()
			input, err := lp.ProgramsABI.Pack("exitValueBounded", f.program, f.account, big.NewInt(tc.maximum))
			require.NoError(t, err)
			out, _, err := f.h.evm.Call(f.h.caller, f.h.precompile.Address(), input, 2000000, uint256.NewInt(0))
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			reason, err := abi.UnpackRevert(out)
			require.NoError(t, err)
			require.Contains(t, reason, "LXP_PROGRAM_REFUSED:")
			require.Equal(t, lp.KernelError(lxphost.Code(-736), "exitValueBounded").Error(), reason)
			require.ErrorIs(t, f.h.stateDB.GetPrecompileError(), lxphost.Code(-736))
			f.assertBalances(t, int64(tc.balance), int64(1000-tc.balance))
			require.Equal(t, before, f.h.dump(f.h.stateDB.Ctx()))
			require.Equal(t, logsBefore, f.h.stateDB.GetAllLogs())
		})
	}
}
