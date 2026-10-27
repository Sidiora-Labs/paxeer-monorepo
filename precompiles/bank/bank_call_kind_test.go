package bank_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/bank"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestBankStaticcallReason(t *testing.T) {
	for _, method := range []string{bank.SendMethod, bank.SendNativeMethod} {
		t.Run(method, func(t *testing.T) {
			assertBankCallKindReason(t, method, true, false,
				"cannot call "+method+" from staticcall",
				"LXP_BANK_STATICCALL: cannot call <method> from staticcall; "+method)
		})
	}
}

func TestBankDelegatecallReason(t *testing.T) {
	assertBankCallKindReason(t, bank.SendNativeMethod, false, true,
		"cannot delegatecall sendNative",
		"LXP_BANK_DELEGATECALL: cannot delegatecall <method>; sendNative")
}

func assertBankCallKindReason(t *testing.T, methodName string, readOnly, delegatecall bool, headText, codedText string) {
	t.Helper()
	testApp := testkeeper.EVMTestApp
	ctx, _ := testApp.GetContextForDeliverTx(nil).CacheContext()
	k := &testApp.EvmKeeper
	const planHeight = int64(100)
	k.UpgradeKeeper().SetDone(ctx.WithBlockHeight(planHeight), pcommon.MergeUpgrade)
	p, err := bank.NewPrecompile(testApp.GetPrecompileKeepers())
	require.NoError(t, err)
	method := p.ABI.Methods[methodName]
	caller := common.HexToAddress("0x0000000000000000000000000000000000000633")
	receiver := common.HexToAddress("0x0000000000000000000000000000000000000634")
	args := []interface{}{sdk.AccAddress(receiver.Bytes()).String()}
	var value *big.Int
	if methodName == bank.SendMethod {
		args = []interface{}{caller, receiver, "uhpx", big.NewInt(1)}
	} else {
		value = big.NewInt(1)
	}
	input, err := p.ABI.Pack(methodName, args...)
	require.NoError(t, err)

	for _, height := range []int64{planHeight - 1, planHeight, planHeight + 1} {
		t.Run(fmt.Sprintf("height_%d", height), func(t *testing.T) {
			callCtx := ctx.WithBlockHeight(height).WithGasMeter(sdk.NewGasMeter(1_000_000, 1, 1))
			wantText := headText
			if height >= planHeight {
				wantText = codedText
			}
			result, _, runErr := p.GetExecutor().Execute(callCtx.WithEVMPrecompileCalledFromDelegateCall(delegatecall),
				&method, caller, caller, args, value, readOnly, nil, 1_000_000, nil)
			require.EqualError(t, runErr, wantText)
			require.Nil(t, result)

			stateDB := state.NewDBImpl(callCtx, k, false)
			result, _, runErr = p.RunAndCalculateGas(&vm.EVM{StateDB: stateDB}, caller, caller,
				input, 1_000_000, value, nil, readOnly, delegatecall)
			require.ErrorIs(t, runErr, vm.ErrExecutionReverted)
			if height < planHeight {
				require.Nil(t, result)
			} else {
				reason, decodeErr := abi.UnpackRevert(result)
				require.NoError(t, decodeErr)
				require.Equal(t, wantText, reason)
			}
			require.Empty(t, stateDB.GetAllLogs())
		})
	}
}
