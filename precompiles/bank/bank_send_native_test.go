package bank_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/bank"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestSendNativeZeroValueReason(t *testing.T) {
	receiver := sdk.AccAddress(common.HexToAddress("0x0636").Bytes()).String()
	for _, tc := range []struct {
		name  string
		value *big.Int
	}{
		{name: "nil"},
		{name: "zero", value: big.NewInt(0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertSendNativeRefusal(t, receiver, tc.value,
				"set `value` field to non-zero to send",
				"LXP_BANK_ZERO_VALUE: set `value` field to non-zero to send")
		})
	}
}

func TestSendNativeBadReceiverReason(t *testing.T) {
	valid := sdk.AccAddress(common.HexToAddress("0x0636").Bytes()).String()
	checksum := valid[:len(valid)-1] + "q"
	if checksum == valid {
		checksum = valid[:len(valid)-1] + "p"
	}
	for _, tc := range []struct {
		name     string
		receiver string
	}{
		{name: "malformed", receiver: "not-a-bech32-address"},
		{name: "checksum", receiver: checksum},
		{name: "empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			headText := "invalid addr"
			if tc.receiver != "" {
				_, err := sdk.AccAddressFromBech32(tc.receiver)
				require.Error(t, err)
				headText = err.Error()
			}
			assertSendNativeRefusal(t, tc.receiver, big.NewInt(1), headText,
				"LXP_PRECOMPILE_INVALID_ADDR: invalid addr")
		})
	}
}

func assertSendNativeRefusal(t *testing.T, receiver string, value *big.Int, headText, codedText string) {
	t.Helper()
	testApp := testkeeper.EVMTestApp
	ctx, _ := testApp.GetContextForDeliverTx(nil).CacheContext()
	k := &testApp.EvmKeeper
	const planHeight = int64(100)
	k.UpgradeKeeper().SetDone(ctx.WithBlockHeight(planHeight), pcommon.MergeUpgrade)
	caller := common.HexToAddress("0x0635")
	sender := sdk.AccAddress(caller.Bytes())
	k.SetAddressMapping(ctx, sender, caller)
	funds := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 25))
	require.NoError(t, testApp.BankKeeper.MintCoins(ctx, evmtypes.ModuleName, funds))
	require.NoError(t, testApp.BankKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, sender, funds))
	p, err := bank.NewPrecompile(testApp.GetPrecompileKeepers())
	require.NoError(t, err)
	method := p.ABI.Methods[bank.SendNativeMethod]
	args := []interface{}{receiver}
	input, err := p.ABI.Pack(bank.SendNativeMethod, args...)
	require.NoError(t, err)

	for _, height := range []int64{planHeight - 1, planHeight, planHeight + 1} {
		t.Run(fmt.Sprintf("height_%d", height), func(t *testing.T) {
			callCtx := ctx.WithBlockHeight(height).WithGasMeter(sdk.NewGasMeter(1_000_000, 1, 1))
			wantText := headText
			if height >= planHeight {
				wantText = codedText
			}
			result, _, runErr := p.GetExecutor().Execute(callCtx, &method, caller, caller,
				args, value, false, nil, 1_000_000, nil)
			require.EqualError(t, runErr, wantText)
			require.Nil(t, result)
			require.Equal(t, sdk.NewInt(25), testApp.BankKeeper.GetBalance(callCtx, sender, "uhpx").Amount)

			stateDB := state.NewDBImpl(callCtx, k, false)
			result, _, runErr = p.RunAndCalculateGas(&vm.EVM{StateDB: stateDB}, caller, caller,
				input, 1_000_000, value, nil, false, false)
			require.ErrorIs(t, runErr, vm.ErrExecutionReverted)
			if height < planHeight {
				require.Nil(t, result)
			} else {
				reason, decodeErr := abi.UnpackRevert(result)
				require.NoError(t, decodeErr)
				require.Equal(t, wantText, reason)
			}
			require.Equal(t, sdk.NewInt(25), testApp.BankKeeper.GetBalance(stateDB.Ctx(), sender, "uhpx").Amount)
			require.Empty(t, stateDB.GetAllLogs())
		})
	}
}
