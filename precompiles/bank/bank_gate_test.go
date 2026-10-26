package bank_test

import (
	"math/big"
	"testing"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/bank"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	wasmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestBankPlanGatesNilUpgradeKeeper(t *testing.T) {
	testApp := testkeeper.EVMTestApp
	ctx, _ := testApp.GetContextForDeliverTx(nil).CacheContext()
	ctx = ctx.WithBlockHeight(100).WithGasMeter(sdk.NewGasMeter(1_000_000, 1, 1))
	k := evmkeeper.NewKeeper(
		testApp.GetKey(evmtypes.StoreKey), testApp.GetTKey(evmtypes.TransientStoreKey),
		testApp.GetSubspace(evmtypes.ModuleName), nil, testApp.BankKeeper,
		&testApp.AccountKeeper, &testApp.StakingKeeper, testApp.TransferKeeper,
		wasmkeeper.NewDefaultPermissionKeeper(testApp.WasmKeeper), &testApp.WasmKeeper, nil,
	)
	require.Nil(t, k.UpgradeKeeper())
	keepers := app.NewPrecompileKeepers(testApp)
	keepers.EVMKeeper = k
	p, err := bank.NewPrecompile(keepers)
	require.NoError(t, err)
	executor := p.GetExecutor().(*bank.PrecompileExecutor)
	method := p.ABI.Methods[bank.SendMethod]

	sender, senderEVM := testkeeper.MockAddressPair()
	receiver, receiverEVM := testkeeper.MockAddressPair()
	pointer := common.HexToAddress("0x0000000000000000000000000000000000000631")
	const denom = "ubankgate"
	k.SetAddressMapping(ctx, sender, senderEVM)
	k.SetAddressMapping(ctx, receiver, receiverEVM)
	require.NoError(t, k.SetERC20NativePointer(ctx, denom, pointer))
	funds := sdk.NewCoins(sdk.NewInt64Coin(denom, 25))
	require.NoError(t, testApp.BankKeeper.MintCoins(ctx, evmtypes.ModuleName, funds))
	require.NoError(t, testApp.BankKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, sender, funds))

	t.Run("send", func(t *testing.T) {
		args := []interface{}{senderEVM, receiverEVM, denom, big.NewInt(10)}
		var result []byte
		require.NotPanics(t, func() {
			result, _, err = executor.Execute(ctx, &method, pointer, pointer, args, nil, false, nil, 1_000_000, nil)
		})
		require.NoError(t, err)
		out, unpackErr := method.Outputs.Unpack(result)
		require.NoError(t, unpackErr)
		require.Equal(t, []interface{}{true}, out)
		require.Equal(t, sdk.NewInt(15), testApp.BankKeeper.GetBalance(ctx, sender, denom).Amount)
		require.Equal(t, sdk.NewInt(10), testApp.BankKeeper.GetBalance(ctx, receiver, denom).Amount)
	})

	t.Run("insufficient funds keeps below-plan error", func(t *testing.T) {
		args := []interface{}{senderEVM, receiverEVM, denom, big.NewInt(26)}
		require.NotPanics(t, func() {
			_, _, err = executor.Execute(ctx, &method, pointer, pointer, args, nil, false, nil, 1_000_000, nil)
		})
		require.ErrorIs(t, err, sdkerrors.ErrInsufficientFunds)
		require.Equal(t, sdk.NewInt(15), testApp.BankKeeper.GetBalance(ctx, sender, denom).Amount)
		require.Equal(t, sdk.NewInt(10), testApp.BankKeeper.GetBalance(ctx, receiver, denom).Amount)
	})

	t.Run("refusal keeps below-plan text", func(t *testing.T) {
		args := []interface{}{senderEVM, receiverEVM, denom, big.NewInt(1)}
		require.NotPanics(t, func() {
			_, _, err = executor.Execute(ctx, &method, pointer, pointer, args, big.NewInt(1), false, nil, 1_000_000, nil)
		})
		require.EqualError(t, err, "sending funds to a non-payable function")
	})
}
