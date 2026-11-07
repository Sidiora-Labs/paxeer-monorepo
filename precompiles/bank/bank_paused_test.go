package bank_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/bank"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestSendPausedDenomReason(t *testing.T) {
	for _, height := range []int64{100, 101} {
		for _, v6120Height := range []int64{1, 200} {
			t.Run(fmt.Sprintf("height_%d/v6120_%d", height, v6120Height), func(t *testing.T) {
				assertSendPausedDenom(t, height, v6120Height)
			})
		}
	}
}

func TestSendPausedDenomBelowPlan(t *testing.T) {
	for _, v6120Height := range []int64{1, 200} {
		t.Run(fmt.Sprintf("v6120_%d", v6120Height), func(t *testing.T) {
			assertSendPausedDenom(t, 99, v6120Height)
		})
	}
}

func assertSendPausedDenom(t *testing.T, height, v6120Height int64) {
	t.Helper()
	testApp := testkeeper.EVMTestApp
	ctx, _ := testApp.GetContextForDeliverTx(nil).CacheContext()
	ctx = ctx.WithBlockHeight(height).WithGasMeter(sdk.NewGasMeter(1_000_000, 1, 1))
	k := &testApp.EvmKeeper
	const planHeight = int64(100)
	k.UpgradeKeeper().SetDone(ctx.WithBlockHeight(planHeight), pcommon.MergeUpgrade)
	k.UpgradeKeeper().SetDone(ctx.WithBlockHeight(v6120Height), bank.V6120Upgrade)

	sender, senderEVM := testkeeper.MockAddressPair()
	receiver, receiverEVM := testkeeper.MockAddressPair()
	pointer := common.HexToAddress("0x0000000000000000000000000000000000000632")
	const denom = "upausedbank"
	k.SetAddressMapping(ctx, sender, senderEVM)
	k.SetAddressMapping(ctx, receiver, receiverEVM)
	require.NoError(t, k.SetERC20NativePointer(ctx, denom, pointer))
	funds := sdk.NewCoins(sdk.NewInt64Coin(denom, 25))
	require.NoError(t, testApp.BankKeeper.MintCoins(ctx, types.ModuleName, funds))
	require.NoError(t, testApp.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sender, funds))
	params := testApp.BankKeeper.GetParams(ctx).SetSendEnabledParam(denom, false)
	testApp.BankKeeper.SetParams(ctx, params)
	require.False(t, testApp.BankKeeper.IsSendEnabledCoin(ctx, funds[0]))

	p, err := bank.NewPrecompile(testApp.GetPrecompileKeepers())
	require.NoError(t, err)
	method := p.ABI.Methods[bank.SendMethod]
	args := []interface{}{senderEVM, receiverEVM, denom, big.NewInt(10)}
	const headText = denom + " transfers are currently disabled: send transactions are disabled"
	wantText := headText
	if height >= planHeight {
		wantText = "LXP_ASSET_PAUSED: asset paused; " + headText
	}
	result, _, err := p.GetExecutor().Execute(ctx, &method, pointer, pointer, args, nil, false, nil, 1_000_000, nil)
	require.EqualError(t, err, wantText)
	require.Nil(t, result)
	if height < planHeight {
		require.ErrorIs(t, err, banktypes.ErrSendDisabled)
	}

	input, err := p.ABI.Pack(bank.SendMethod, args...)
	require.NoError(t, err)
	stateDB := state.NewDBImpl(ctx, k, false)
	result, _, err = p.RunAndCalculateGas(&vm.EVM{StateDB: stateDB}, pointer, pointer, input, 1_000_000, nil, nil, false, false)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	if height >= planHeight {
		reason, decodeErr := abi.UnpackRevert(result)
		require.NoError(t, decodeErr)
		require.Equal(t, wantText, reason)
	} else {
		require.Nil(t, result)
	}
	require.Equal(t, sdk.NewInt(25), testApp.BankKeeper.GetBalance(stateDB.Ctx(), sender, denom).Amount)
	require.Equal(t, sdk.ZeroInt(), testApp.BankKeeper.GetBalance(stateDB.Ctx(), receiver, denom).Amount)
	require.Empty(t, stateDB.GetAllLogs())
}
