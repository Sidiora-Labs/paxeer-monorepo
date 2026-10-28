package layerxprograms_test

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

type sandboxFundFixture struct {
	lease, tenant, program, escrow, asset, source [32]byte
	expiry                                        uint64
	fees                                          [7]uint64
	request                                       []byte
	stateOffset                                   int
}

func newSandboxFundFixture(t *testing.T, h *harness) sandboxFundFixture {
	t.Helper()
	f := sandboxFundFixture{lease: [32]byte{1}, program: [32]byte{3},
		expiry: 200, fees: [7]uint64{1, 1, 1, 1, 1, 1, 1}}
	ctx := h.stateDB.Ctx()
	authority, did, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	f.tenant = authority.Principal
	salt := [32]byte{9}
	assetMaterial := append([]byte("LX:ASSET:v1"), authority.Actor[:]...)
	f.asset = sha256.Sum256(append(assetMaterial, salt[:]...))
	_, f.source, err = lxphost.AssetAccountID(h.publicKey, f.asset)
	require.NoError(t, err)
	setup := func(activityType uint32, payload []byte) {
		t.Helper()
		sequence, err := h.host.ReadIdentitySequence(ctx, did)
		require.NoError(t, err)
		last, err := h.host.ReadLastSequence(ctx, did)
		require.NoError(t, err)
		receipt, err := lxphost.ExecuteActivity(lxphost.WithSequence(ctx, last+1), h.host,
			authority, activityType, payload, sequence)
		require.NoError(t, err)
		require.Equal(t, lxphost.Code(0), receipt.ResultCode)
	}
	registration := append([]byte{0, 1}, f.asset[:]...)
	registration = append(registration, salt[:]...)
	registration = append(registration, 1, 'T', 1, 'T', 6)
	registration = binary.BigEndian.AppendUint64(registration, 0)
	registration = binary.BigEndian.AppendUint64(registration, 10)
	setup(0x00010001, append(registration, 1, 0))
	setup(0x00010004, append([]byte{0, 1}, f.asset[:]...))
	mint := append([]byte{0, 1}, f.asset[:]...)
	mint = append(mint, f.source[:]...)
	mint = binary.BigEndian.AppendUint64(mint, 0)
	mint = binary.BigEndian.AppendUint64(mint, 10)
	setup(0x0001000a, mint)
	sequence, err := h.host.ReadIdentitySequence(ctx, did)
	require.NoError(t, err)
	seed := append([]byte("sandbox-lease-escrow/v1\x00"), f.lease[:]...)
	material := append([]byte("LayerX/programs/program-account/v1\x00"), f.program[:]...)
	material = binary.BigEndian.AppendUint32(material, uint32(len(seed)))
	f.escrow = sha256.Sum256(append(material, seed...))

	seedAccount := func(id [32]byte, name string, kind byte, balance, next uint64, key [32]byte, hasKey byte) {
		field := func(out, value []byte) []byte {
			return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
		}
		row := field(nil, id[:])
		row = field(row, []byte(name))
		row = append(row, kind)
		row = binary.BigEndian.AppendUint64(row, 0)
		row = binary.BigEndian.AppendUint64(row, balance)
		row = field(row, f.asset[:])
		row = append(row, 1)
		row = binary.BigEndian.AppendUint64(row, next)
		row = binary.BigEndian.AppendUint64(row, 1)
		row = append(row, 0, 0)
		row = field(row, key[:])
		row = append(row, hasKey)
		ctx.KVStore(h.key).Set(append([]byte{6}, id[:]...), row)
	}
	mainName, mainID, err := lxphost.MainAccountID(h.publicKey)
	require.NoError(t, err)
	seedAccount(mainID, mainName, 1, 0, sequence, h.publicKey, 1)
	seedAccount(f.escrow, "module:programs:value:"+hex.EncodeToString(f.escrow[:]), 13, 0, 0, [32]byte{}, 0)

	namespaceMaterial := append([]byte("LayerX/programs/sandbox/namespace/v1\x00"), f.program[:]...)
	namespace := sha256.Sum256(append(namespaceMaterial, f.lease[:]...))
	feeDestination, err := codec.DeriveAccountID([]byte("system:fees"))
	require.NoError(t, err)
	fields := []byte{}
	for _, id := range [][32]byte{f.lease, f.tenant, f.program, {4}, namespace, f.asset, f.escrow, feeDestination} {
		fields = append(fields, id[:]...)
	}
	fields = binary.BigEndian.AppendUint64(fields, 0)
	fields = binary.BigEndian.AppendUint64(fields, 5)
	for i := 0; i < 8; i++ {
		fields = binary.BigEndian.AppendUint64(fields, 1)
	}
	fields = binary.BigEndian.AppendUint64(fields, 1)
	fields = binary.BigEndian.AppendUint64(fields, f.expiry)
	schedule := binary.BigEndian.AppendUint32(nil, 1)
	for _, rate := range f.fees {
		schedule = binary.BigEndian.AppendUint64(schedule, rate)
	}
	requestMaterial := append([]byte("LayerX/programs/sandbox/request/v3\x00"), fields...)
	requestDigest := sha256.Sum256(append(requestMaterial, schedule...))
	f.request = append([]byte("LayerX/programs/sandbox/lease-state/v3\x00"), fields...)
	f.request = append(f.request, make([]byte, 64)...)
	f.request = append(f.request, schedule...)
	f.request = append(f.request, make([]byte, 16)...)
	f.stateOffset = len(f.request)
	f.request = append(f.request, 0, 1, 0, 0, 0)
	requestActivity := [32]byte{8}
	f.request = append(f.request, requestActivity[:]...)
	f.request = append(f.request, requestDigest[:]...)
	requestReceipt := sha256.Sum256(f.request)
	f.request = append(f.request, requestReceipt[:]...)
	f.request = binary.BigEndian.AppendUint64(f.request, 1)
	f.request = append(f.request, 0, 0, 0)
	return f
}

func (f sandboxFundFixture) input(t *testing.T, request []byte) []byte {
	t.Helper()
	input, err := lp.ProgramsABI.Pack("sandboxFund", f.lease, f.tenant, f.program, f.escrow,
		f.asset, big.NewInt(5), f.expiry, uint32(1), f.fees, request, f.program,
		[]lp.TransferLeg{{From: f.source, Asset: f.asset, To: f.escrow, Amount: big.NewInt(5)}})
	require.NoError(t, err)
	return input
}

func assertSandboxFundBalances(t *testing.T, h *harness, f sandboxFundFixture, source, escrow int64) {
	t.Helper()
	for _, account := range []struct {
		id   [32]byte
		want int64
	}{{f.source, source}, {f.escrow, escrow}} {
		balance, err := h.host.ReadBalance(h.stateDB.Ctx(), account.id, f.asset)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(account.want), balance)
	}
}

func TestProgramsSandboxFundEscrowsLegs(t *testing.T) {
	h := newHarness(t)
	f := newSandboxFundFixture(t, h)
	assertSandboxFundBalances(t, h, f, 10, 0)
	method := lp.ProgramsABI.Methods["sandboxFund"]
	require.Equal(t, pcommon.LXPRevert("LXP_PROGRAM_REFUSED", "write under staticcall"),
		pcommon.RevertReason(lp.CheckCall(&method, big.NewInt(0), true)))
	before := h.dump(h.stateDB.Ctx())
	data, _, err := h.evm.StaticCall(h.caller, h.precompile.Address(), f.input(t, f.request), 2000000)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Equal(t, pcommon.LXPRevert("LXP_PROGRAM_REFUSED", "write under staticcall"), data)
	reason, err := abi.UnpackRevert(data)
	require.NoError(t, err)
	require.Equal(t, lp.ErrReadOnlyWrite.Error(), reason)
	require.Equal(t, before, h.dump(h.stateDB.Ctx()))
	data, _, err = h.evm.Call(h.caller, h.precompile.Address(), f.input(t, f.request), 2000000, uint256.NewInt(0))
	require.NoError(t, err, "sandboxFund reverted: %x", data)
	require.Empty(t, data)
	assertSandboxFundBalances(t, h, f, 5, 5)

	input, err := lp.ProgramsABI.Pack("getSandbox", f.lease, uint8(0))
	require.NoError(t, err)
	before = h.dump(h.stateDB.Ctx())
	data, _, err = h.evm.StaticCall(h.caller, h.precompile.Address(), input, 2000000)
	require.NoError(t, err)
	require.Equal(t, before, h.dump(h.stateDB.Ctx()))
	values, err := lp.ProgramsABI.Unpack("getSandbox", data)
	require.NoError(t, err)
	require.Len(t, values, 1)
	state := values[0].([]byte)
	require.Greater(t, len(state), f.stateOffset+1)
	require.Equal(t, f.request[:f.stateOffset], state[:f.stateOffset])
	require.Equal(t, byte(1), state[f.stateOffset], "lease must be funded")
	require.Equal(t, byte(2), state[f.stateOffset+1], "request and fund transitions must be recorded")
	hostState, err := h.host.ReadSandbox(h.stateDB.Ctx(), f.lease, 0)
	require.NoError(t, err)
	require.Equal(t, hostState, state)
}

func TestProgramsSandboxFundEmptyLeaseRequestReverts(t *testing.T) {
	h := newHarness(t)
	f := newSandboxFundFixture(t, h)
	assertSandboxFundBalances(t, h, f, 10, 0)
	_, did, err := lp.PrincipalOf(h.stateDB.Ctx(), h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	sequence, err := h.host.ReadIdentitySequence(h.stateDB.Ctx(), did)
	require.NoError(t, err)
	before := h.dump(h.stateDB.Ctx())
	data, _, err := h.evm.Call(h.caller, h.precompile.Address(), f.input(t, nil), 2000000, uint256.NewInt(0))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(data)
	require.NoError(t, err)
	require.Equal(t, "LXP_NON_CANONICAL", reason)
	assertSandboxFundBalances(t, h, f, 10, 0)
	require.Equal(t, before, h.dump(h.stateDB.Ctx()))
	afterSequence, err := h.host.ReadIdentitySequence(h.stateDB.Ctx(), did)
	require.NoError(t, err)
	require.Equal(t, sequence, afterSequence)
	state, err := h.host.ReadSandbox(h.stateDB.Ctx(), f.lease, 0)
	require.ErrorIs(t, err, lxphost.Code(-7))
	require.Nil(t, state)
}
