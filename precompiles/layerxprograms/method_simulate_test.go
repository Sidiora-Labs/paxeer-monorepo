package layerxprograms_test

import (
	"bytes"
	"math/big"
	"testing"

	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func simulationStoreDump(store sdk.KVStore) map[string][]byte {
	values := map[string][]byte{}
	iterator := store.Iterator(nil, nil)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		values[string(iterator.Key())] = bytes.Clone(iterator.Value())
	}
	return values
}

func TestProgramsSimulateCallMatchesCallOutput(t *testing.T) {
	h, args, payload := programsCallFixture(t)
	ctx := h.stateDB.Ctx()
	reference, _ := ctx.CacheContext()
	callMethod := lp.ProgramsABI.Methods["call"]
	var limit [16]byte
	args[8].(*big.Int).FillBytes(limit[:])
	receipt, err := lp.Submit(reference, h.executor, lp.NewCallContext(h.caller, &callMethod), 0x00090003, payload, limit)
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	require.NotNil(t, receipt.ProgramOutcome)
	outcome := receipt.ProgramOutcome
	require.Zero(t, outcome.Status)
	require.Equal(t, uint8(1), outcome.Kind)
	require.Positive(t, receipt.FeeCharged.Sign())
	expected, err := callMethod.Outputs.Pack(outcome.Status, outcome.Kind, outcome.Response)
	require.NoError(t, err)

	input, err := lp.ProgramsABI.Pack("simulateCall", args...)
	require.NoError(t, err)
	simulated, _, err := h.evm.StaticCall(h.caller, h.precompile.Address(), input, 20000000)
	require.NoError(t, err)
	require.Equal(t, expected, simulated)
	decoded, err := lp.ProgramsABI.Methods["simulateCall"].Outputs.Unpack(simulated)
	require.NoError(t, err)
	require.Equal(t, []interface{}{outcome.Status, outcome.Kind, outcome.Response}, decoded)
	require.Empty(t, h.stateDB.GetAllLogs())

	callInput, err := lp.ProgramsABI.Pack("call", args...)
	require.NoError(t, err)
	called, _, err := h.evm.Call(h.caller, h.precompile.Address(), callInput, 20000000, uint256.NewInt(0))
	require.NoError(t, err)
	require.Equal(t, simulated, called)
	require.Equal(t, h.dump(reference), h.dump(h.stateDB.Ctx()))
	require.Len(t, h.stateDB.GetAllLogs(), 2)
}

func TestProgramsSimulateCallLeavesStateUnchanged(t *testing.T) {
	h, args, _ := programsCallFixture(t)
	ctx := h.stateDB.Ctx()
	txHash := [32]byte(ctx.EVMTxHash())
	require.Zero(t, lp.NextOrdinal(ctx, txHash))
	probe, _ := ctx.CacheContext()
	ordinalBefore := lp.NextOrdinal(probe, txHash)
	require.Equal(t, uint32(1), ordinalBefore)

	before := h.dump(ctx)
	storesBefore := map[string]map[string][]byte{}
	for _, key := range ctx.MultiStore().StoreKeys() {
		switch key := key.(type) {
		case *sdk.KVStoreKey:
			storesBefore[key.Name()] = simulationStoreDump(ctx.KVStore(key))
		case *sdk.TransientStoreKey:
			storesBefore[key.Name()] = simulationStoreDump(ctx.TransientStore(key))
		}
	}
	_, did, err := lp.PrincipalOf(ctx, h.executor.TestEVMKeeper(), h.caller)
	require.NoError(t, err)
	sequenceBefore, err := h.host.ReadIdentitySequence(ctx, did)
	require.NoError(t, err)
	require.Positive(t, sequenceBefore)
	lastBefore, err := h.host.ReadLastSequence(ctx, did)
	require.NoError(t, err)
	nonceBefore := h.executor.TestEVMKeeper().GetLayerXBindNonce(ctx, h.caller)

	headInput, err := lp.ProgramsABI.Pack("getStateHead", args[0])
	require.NoError(t, err)
	headBefore, _, err := h.evm.StaticCall(h.caller, h.precompile.Address(), headInput, 2000000)
	require.NoError(t, err)
	headValues, err := lp.ProgramsABI.Methods["getStateHead"].Outputs.Unpack(headBefore)
	require.NoError(t, err)
	require.Len(t, headValues, 1)
	require.Len(t, headValues[0].([]byte), 38)
	input, err := lp.ProgramsABI.Pack("simulateCall", args...)
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		_, _, err := h.evm.StaticCall(h.caller, h.precompile.Address(), input, 20000000)
		require.NoError(t, err)
		after := h.stateDB.Ctx()
		require.Equal(t, before, h.dump(after))
		for _, key := range after.MultiStore().StoreKeys() {
			switch key := key.(type) {
			case *sdk.KVStoreKey:
				require.Equal(t, storesBefore[key.Name()], simulationStoreDump(after.KVStore(key)), key.Name())
			case *sdk.TransientStoreKey:
				require.Equal(t, storesBefore[key.Name()], simulationStoreDump(after.TransientStore(key)), key.Name())
			}
		}
		sequenceAfter, err := h.host.ReadIdentitySequence(after, did)
		require.NoError(t, err)
		require.Equal(t, sequenceBefore, sequenceAfter)
		lastAfter, err := h.host.ReadLastSequence(after, did)
		require.NoError(t, err)
		require.Equal(t, lastBefore, lastAfter)
		require.Equal(t, nonceBefore, h.executor.TestEVMKeeper().GetLayerXBindNonce(after, h.caller))
		probe, _ := after.CacheContext()
		require.Equal(t, ordinalBefore, lp.NextOrdinal(probe, txHash))
		headAfter, _, err := h.evm.StaticCall(h.caller, h.precompile.Address(), headInput, 2000000)
		require.NoError(t, err)
		require.Equal(t, headBefore, headAfter)
		require.Empty(t, h.stateDB.GetAllLogs())
		require.Empty(t, after.EventManager().Events())
	}
}
