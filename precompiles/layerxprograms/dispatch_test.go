package layerxprograms_test

import (
	"testing"

	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestProgramsDispatchRefusesDuplicateRegistration(t *testing.T) {
	table := lp.HandlerTable{}
	invocations := 0
	h := func(_ sdk.Context, _ *lp.PrecompileExecutor, call lp.CallContext, args []interface{}) ([]byte, error) {
		invocations++
		_, err := lp.DecodeArgs(call.Method(), args)
		return []byte{1}, err
	}
	lp.RegisterInto(table, "getProgram", h)
	require.Panics(t, func() { lp.RegisterInto(table, "getProgram", h) })

	executor := lp.NewTestExecutorWithHandlers(nil, nil, table)
	method := lp.ProgramsABI.Methods["transfer"]
	out, err := executor.Execute(sdk.Context{}, &method, common.Address{}, common.Address{}, nil, nil, false, nil, nil)
	require.Nil(t, out)
	require.EqualError(t, err, "LXP_PRECOMPILE_UNKNOWN_METHOD: transfer")
	require.Zero(t, invocations)
}

func TestProgramsDispatchServesViewsUnderStaticCall(t *testing.T) {
	table := lp.HandlerTable{}
	var recordedCaller common.Address
	var recordedName string
	var recordedEVM *vm.EVM
	var recordedArgs []interface{}
	var recordedExecutor *lp.PrecompileExecutor
	var recordedHeight int64
	invocations := map[string]int{}
	h := func(ctx sdk.Context, p *lp.PrecompileExecutor, call lp.CallContext, args []interface{}) ([]byte, error) {
		invocations[call.Method().Name]++
		recordedCaller, recordedName, recordedEVM = call.Caller(), call.Method().Name, call.EVM()
		recordedExecutor, recordedHeight = p, ctx.BlockHeight()
		var err error
		recordedArgs, err = lp.DecodeArgs(call.Method(), args)
		if err != nil {
			return nil, err
		}
		return []byte{0x42}, nil
	}
	lp.RegisterInto(table, "getProgram", h)
	lp.RegisterInto(table, "registryRead", h)
	executor := lp.NewTestExecutorWithHandlers(nil, nil, table)
	ctx := sdk.Context{}.WithBlockHeight(12)
	caller, contract := common.HexToAddress("0x1234"), common.HexToAddress("0x5678")
	evm := &vm.EVM{}
	method := lp.ProgramsABI.Methods["getProgram"]
	program := [32]byte{0xa1}
	packed, err := method.Inputs.Pack(program)
	require.NoError(t, err)
	args, err := method.Inputs.Unpack(packed)
	require.NoError(t, err)
	out, err := executor.Execute(ctx, &method, caller, contract, args, nil, true, evm, nil)
	require.NoError(t, err)
	require.Equal(t, []byte{0x42}, out)
	require.Equal(t, 1, invocations["getProgram"])
	require.Equal(t, caller, recordedCaller)
	require.Equal(t, "getProgram", recordedName)
	require.Same(t, evm, recordedEVM)
	require.Same(t, executor, recordedExecutor)
	require.Equal(t, ctx.BlockHeight(), recordedHeight)
	require.Equal(t, []interface{}{program}, recordedArgs)

	writeMethod := lp.ProgramsABI.Methods["registryRead"]
	writeArgs := []interface{}{program, []byte("manifest")}
	out, err = executor.Execute(ctx, &writeMethod, caller, contract, writeArgs, nil, true, evm, nil)
	require.Nil(t, out)
	require.ErrorIs(t, err, lp.ErrReadOnlyWrite)
	require.Zero(t, invocations["registryRead"])
	require.Equal(t, 1, invocations["getProgram"])
	require.Equal(t, "getProgram", recordedName)
}
