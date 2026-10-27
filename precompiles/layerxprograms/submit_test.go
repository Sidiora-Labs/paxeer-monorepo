package layerxprograms

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestProgramsSubmitCarriesFeeLimitAndKey(t *testing.T) {
	h := newHarness(t)
	ctx := h.stateDB.Ctx()
	method := programsABI.Methods["registryRead"]
	call := callContext{caller: h.caller, method: &method}
	program := [32]byte{0xa1}
	payload, err := EncodeRegistryReadPayload(program, []byte("manifest"))
	require.NoError(t, err)
	authority, did, err := principalOf(ctx, h.executor.evmKeeper, h.caller)
	require.NoError(t, err)
	require.Equal(t, []byte(evmtypes.LayerXDid(h.publicKey)), did)
	actorID, err := lxphost.DIDID(did)
	require.NoError(t, err)
	require.Equal(t, lxphost.KernelAuthority{Bound: true, Actor: actorID, Principal: actorID, VerifiedKey: h.publicKey}, authority)

	for _, limit := range [][16]byte{{15: 9}, {0: 1}, {7: 1}, {8: 1}} {
		branch, _ := ctx.CacheContext()
		before := h.dump(branch)
		receipt, err := submit(branch, h.executor, call, 0x00090004, payload, limit)
		require.ErrorIs(t, err, lxphost.Code(-602))
		require.Equal(t, lxphost.Receipt{}, receipt)
		require.Equal(t, before, h.dump(branch))
	}

	for ordinal := uint32(0); ordinal < 3; ordinal++ {
		if ordinal == 2 {
			method = programsABI.Methods["transfer"]
		}
		var selector [4]byte
		copy(selector[:], method.ID)
		key := IdempotencyKey([32]byte(ctx.EVMTxHash()), ordinal, selector)
		reference, _ := ctx.CacheContext()
		timestamp := uint64(ctx.BlockTime().UnixMilli())
		expected, err := h.host.Execute(lxphost.WithSequence(reference, uint64(ordinal)+1), lxphost.Activity{
			ProtocolVersion: 3, Type: 0x00090004, ActorDID: did, AuthorityKey: h.publicKey,
			AccountSequence: uint64(ordinal), NotBefore: timestamp, NotAfter: timestamp,
			IdempotencyKey: key, Payload: payload,
		}, authority)
		require.NoError(t, err)
		require.Zero(t, expected.ResultCode)
		receipt, err := submit(ctx, h.executor, call, 0x00090004, payload, [16]byte{})
		require.NoError(t, err)
		require.Equal(t, expected, receipt)
		require.Zero(t, receipt.ResultCode)
		require.Equal(t, uint64(ordinal)+1, receipt.GlobalSequence)
		require.Equal(t, uint64(ctx.BlockTime().UnixMilli()), receipt.Timestamp)
		require.Equal(t, []lxphost.ReceiptEvent{{Code: 4, Body: program[:]}}, receipt.Events)
		decoded, err := codec.DecodeReceipt(receipt.Encoded)
		require.NoError(t, err)
		require.Equal(t, uint16(3), decoded.ProtocolVersion)
		require.Equal(t, receipt.Encoded, decoded.UnsignedBytes())
		sequence, err := h.host.ReadIdentitySequence(ctx, did)
		require.NoError(t, err)
		require.Equal(t, uint64(ordinal)+1, sequence)
	}
	ctx = ctx.WithEVMTxHash(common.HexToHash("0xfedcba"))
	var selector [4]byte
	copy(selector[:], method.ID)
	key := IdempotencyKey([32]byte(ctx.EVMTxHash()), 0, selector)
	reference, _ := ctx.CacheContext()
	timestamp := uint64(ctx.BlockTime().UnixMilli())
	expected, err := h.host.Execute(lxphost.WithSequence(reference, 4), lxphost.Activity{
		ProtocolVersion: 3, Type: 0x00090004, ActorDID: did, AuthorityKey: h.publicKey,
		AccountSequence: 3, NotBefore: timestamp, NotAfter: timestamp,
		IdempotencyKey: key, Payload: payload,
	}, authority)
	require.NoError(t, err)
	require.Zero(t, expected.ResultCode)
	receipt, err := submit(ctx, h.executor, call, 0x00090004, payload, [16]byte{})
	require.NoError(t, err)
	require.Equal(t, expected, receipt)
	require.Equal(t, uint64(4), receipt.GlobalSequence)
	sequence, err := h.host.ReadIdentitySequence(ctx, did)
	require.NoError(t, err)
	require.Equal(t, uint64(4), sequence)
}

func TestProgramsSubmitKernelRefusalReverts(t *testing.T) {
	h := newHarness(t)
	ctx := h.stateDB.Ctx()
	before := h.dump(ctx)
	method := programsABI.Methods["registryRead"]
	call := callContext{caller: h.caller, method: &method}
	branch, _ := ctx.CacheContext()
	receipt, err := submit(branch, h.executor, call, 0x00090004, make([]byte, 32), [16]byte{})
	require.ErrorIs(t, err, lxphost.Code(-7))
	require.Equal(t, lxphost.Code(-7), receipt.ResultCode)
	require.EqualError(t, err, kernelError(receipt.ResultCode, method.Name).Error())
	require.NotEqual(t, before, h.dump(branch))
	require.Equal(t, before, h.dump(ctx))

	h.executor.host = h.newHost(1)
	payload, err := EncodeRegistryReadPayload([32]byte{0xa1}, []byte("manifest"))
	require.NoError(t, err)
	branch, _ = ctx.CacheContext()
	receipt, err = submit(branch, h.executor, call, 0x00090004, payload, [16]byte{})
	require.Equal(t, lxphost.Code(-601), receipt.ResultCode)
	require.ErrorIs(t, err, receipt.ResultCode)
	require.NotEqual(t, before, h.dump(branch))
	input, err := programsABI.Pack("registryRead", [32]byte{0xa1}, []byte("manifest"))
	require.NoError(t, err)
	data, _, err := h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(data)
	require.NoError(t, err)
	require.Equal(t, kernelError(lxphost.Code(-601), "registryRead").Error(), reason)
	require.ErrorIs(t, h.stateDB.GetPrecompileError(), lxphost.Code(-601))
	require.Equal(t, before, h.dump(h.stateDB.Ctx()))
	ordinalBranch, _ := h.stateDB.Ctx().CacheContext()
	require.Zero(t, nextOrdinal(ordinalBranch, [32]byte(ctx.EVMTxHash())))
	h.executor.host = h.host
	var selector [4]byte
	copy(selector[:], method.ID)
	authority, did, err := principalOf(h.stateDB.Ctx(), h.executor.evmKeeper, h.caller)
	require.NoError(t, err)
	reference, _ := h.stateDB.Ctx().CacheContext()
	timestamp := uint64(ctx.BlockTime().UnixMilli())
	expected, err := h.host.Execute(lxphost.WithSequence(reference, 1), lxphost.Activity{
		ProtocolVersion: 3, Type: 0x00090004, ActorDID: did, AuthorityKey: h.publicKey,
		NotBefore: timestamp, NotAfter: timestamp,
		IdempotencyKey: IdempotencyKey([32]byte(ctx.EVMTxHash()), 0, selector), Payload: payload,
	}, authority)
	require.NoError(t, err)
	require.Zero(t, expected.ResultCode)
	require.Equal(t, uint64(1), expected.GlobalSequence)
	_, _, err = h.evm.Call(h.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
	require.NoError(t, err)
	require.NotEqual(t, before, h.dump(h.stateDB.Ctx()))
	require.Equal(t, h.dump(reference), h.dump(h.stateDB.Ctx()))
	sequence, err := h.host.ReadIdentitySequence(h.stateDB.Ctx(), did)
	require.NoError(t, err)
	require.Equal(t, uint64(1), sequence)
}
