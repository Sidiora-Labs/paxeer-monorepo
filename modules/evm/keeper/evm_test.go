package keeper_test

import (
	"encoding/base64"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts"
	evmwasm "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/client/wasm"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/wasmbinding/bindings"
	wasmvmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm-runtime/types"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts/native"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
)

func TestInternalCallCreateContract(t *testing.T) {
	bytecode := native.GetBin()
	abi, err := native.NativeMetaData.GetAbi()
	require.Nil(t, err)
	args, err := abi.Pack("", "test", "TST", "TST", uint8(6))
	require.Nil(t, err)
	contractData := append(bytecode, args...)

	k := testkeeper.EVMTestApp.EvmKeeper
	ctx := testkeeper.EVMTestApp.NewContext(false, tmtypes.Header{}).WithBlockHeight(2).WithTxSum([32]byte{1, 2, 3})
	testAddr, _ := testkeeper.MockAddressPair()
	amt := sdk.NewCoins(sdk.NewCoin(k.GetBaseDenom(ctx), sdk.NewInt(200000000)))
	require.Nil(t, k.BankKeeper().MintCoins(ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(k.GetBaseDenom(ctx), sdk.NewInt(200000000)))))
	require.Nil(t, k.BankKeeper().SendCoinsFromModuleToAccount(ctx, types.ModuleName, testAddr, amt))
	req := &types.MsgInternalEVMCall{
		Sender: testAddr.String(),
		Data:   contractData,
	}
	// circular interop call
	ctx = ctx.WithIsEVM(true)
	_, err = k.HandleInternalEVMCall(ctx, req)
	require.Equal(t, "pax does not support EVM->CW->EVM call pattern", err.Error())
	ctx = ctx.WithIsEVM(false)
	oldBaseFee := k.GetNextBaseFeePerGas(ctx)
	k.SetNextBaseFeePerGas(ctx, sdk.ZeroDec())
	_, err = k.HandleInternalEVMCall(ctx, req)
	require.Nil(t, err)
	receipt, err := k.GetTransientReceipt(ctx, [32]byte{1, 2, 3}, 0)
	require.Nil(t, err)
	require.NotNil(t, receipt)
	// reset base fee
	k.SetNextBaseFeePerGas(ctx, oldBaseFee)
}

func TestInternalCall(t *testing.T) {
	bytecode := native.GetBin()
	abi, err := native.NativeMetaData.GetAbi()
	require.Nil(t, err)
	args, err := abi.Pack("", "test", "TST", "TST", uint8(6))
	require.Nil(t, err)
	contractData := append(bytecode, args...)

	k := testkeeper.EVMTestApp.EvmKeeper
	ctx := testkeeper.EVMTestApp.NewContext(false, tmtypes.Header{}).WithBlockHeight(2)
	testAddr, senderEvmAddr := testkeeper.MockAddressPair()
	k.SetAddressMapping(ctx, testAddr, senderEvmAddr)
	amt := sdk.NewCoins(sdk.NewCoin(k.GetBaseDenom(ctx), sdk.NewInt(200000000)))
	require.Nil(t, k.BankKeeper().MintCoins(ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(k.GetBaseDenom(ctx), sdk.NewInt(200000000)))))
	require.Nil(t, k.BankKeeper().SendCoinsFromModuleToAccount(ctx, types.ModuleName, testAddr, amt))
	req := &types.MsgInternalEVMCall{
		Sender: testAddr.String(),
		Data:   contractData,
	}
	ctx = ctx.WithIsEVM(true)
	_, err = k.HandleInternalEVMCall(ctx, req)
	require.Equal(t, "pax does not support EVM->CW->EVM call pattern", err.Error())
	ctx = ctx.WithIsEVM(false)
	ret, err := k.HandleInternalEVMCall(ctx, req)
	require.Nil(t, err)
	contractAddr := crypto.CreateAddress(senderEvmAddr, 0)
	require.NotEmpty(t, k.GetCode(ctx, contractAddr))
	require.Equal(t, ret.Data, k.GetCode(ctx, contractAddr))
	k.SetERC20NativePointer(ctx, "test", contractAddr)

	receiverAddr, evmAddr := testkeeper.MockAddressPair()
	k.SetAddressMapping(ctx, receiverAddr, evmAddr)
	args, err = abi.Pack("transfer", evmAddr, big.NewInt(1000))
	require.Nil(t, err)
	require.Nil(t, k.BankKeeper().MintCoins(ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin("test", sdk.NewInt(200000000)))))
	require.Nil(t, k.BankKeeper().SendCoinsFromModuleToAccount(ctx, types.ModuleName, testAddr, sdk.NewCoins(sdk.NewCoin("test", sdk.NewInt(200000000)))))
	val := sdk.NewInt(0)
	oldNonce := k.GetNonce(ctx, k.GetEVMAddressOrDefault(ctx, testAddr))
	req = &types.MsgInternalEVMCall{
		Sender: testAddr.String(),
		To:     contractAddr.Hex(),
		Data:   args,
		Value:  &val,
	}
	_, err = k.HandleInternalEVMCall(ctx, req)
	require.Nil(t, err)
	require.Equal(t, int64(1000), testkeeper.EVMTestApp.BankKeeper.GetBalance(ctx, receiverAddr, "test").Amount.Int64())
	// nonce should not change
	require.Equal(t, oldNonce, k.GetNonce(ctx, k.GetEVMAddressOrDefault(ctx, testAddr)))
}

func TestStaticCall(t *testing.T) {
	bytecode := native.GetBin()
	abi, err := native.NativeMetaData.GetAbi()
	require.Nil(t, err)
	args, err := abi.Pack("", "test", "TST", "TST", uint8(6))
	require.Nil(t, err)
	contractData := append(bytecode, args...)

	k := testkeeper.EVMTestApp.EvmKeeper
	ctx := testkeeper.EVMTestApp.NewContext(false, tmtypes.Header{}).WithBlockHeight(2)
	testAddr, senderEvmAddr := testkeeper.MockAddressPair()
	k.SetAddressMapping(ctx, testAddr, senderEvmAddr)
	amt := sdk.NewCoins(sdk.NewCoin(k.GetBaseDenom(ctx), sdk.NewInt(2000)))
	require.Nil(t, k.BankKeeper().MintCoins(ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(k.GetBaseDenom(ctx), sdk.NewInt(2000)))))
	require.Nil(t, k.BankKeeper().SendCoinsFromModuleToAccount(ctx, types.ModuleName, testAddr, amt))
	req := &types.MsgInternalEVMCall{
		Sender: testAddr.String(),
		Data:   contractData,
	}
	ret, err := k.HandleInternalEVMCall(ctx, req)
	require.Nil(t, err)
	contractAddr := crypto.CreateAddress(senderEvmAddr, 0)
	require.NotEmpty(t, k.GetCode(ctx, contractAddr))
	require.Equal(t, ret.Data, k.GetCode(ctx, contractAddr))
	require.Nil(t, k.BankKeeper().MintCoins(ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin("test", sdk.NewInt(2000)))))
	require.Nil(t, k.BankKeeper().SendCoinsFromModuleToAccount(ctx, types.ModuleName, testAddr, sdk.NewCoins(sdk.NewCoin("test", sdk.NewInt(2000)))))

	args, err = abi.Pack("balanceOf", senderEvmAddr)
	require.Nil(t, err)
	res, err := k.StaticCallEVM(ctx, testAddr, &contractAddr, args)
	require.Nil(t, err)
	decoded, err := abi.Unpack("balanceOf", res)
	require.Nil(t, err)
	require.Equal(t, 1, len(decoded))
	require.Equal(t, big.NewInt(int64(2000)), decoded[0].(*big.Int))
}

func TestNegativeTransfer(t *testing.T) {
	steal_amount := int64(1_000_000_000_000)

	k := testkeeper.EVMTestApp.EvmKeeper
	ctx := testkeeper.EVMTestApp.NewContext(false, tmtypes.Header{}).WithBlockHeight(2)
	attackerAddr, attackerEvmAddr := testkeeper.MockAddressPair()
	victimAddr, victimEvmAddr := testkeeper.MockAddressPair()

	// associate addrs
	k.SetAddressMapping(ctx, attackerAddr, attackerEvmAddr)
	k.SetAddressMapping(ctx, victimAddr, victimEvmAddr)

	// mint some funds to victim
	amt := sdk.NewCoins(sdk.NewCoin(k.GetBaseDenom(ctx), sdk.NewInt(steal_amount)))
	require.Nil(t, k.BankKeeper().MintCoins(ctx, types.ModuleName, sdk.NewCoins(sdk.NewCoin(k.GetBaseDenom(ctx), sdk.NewInt(steal_amount)))))
	require.Nil(t, k.BankKeeper().SendCoinsFromModuleToAccount(ctx, types.ModuleName, victimAddr, amt))

	// construct attack payload
	val := sdk.NewInt(steal_amount).Mul(sdk.NewInt(steal_amount * -1))
	req := &types.MsgInternalEVMCall{
		Sender: attackerAddr.String(),
		Data:   []byte{},
		Value:  &val,
		To:     victimEvmAddr.Hex(),
	}

	// pre verification
	preAttackerBal := testkeeper.EVMTestApp.BankKeeper.GetBalance(ctx, attackerAddr, k.GetBaseDenom(ctx)).Amount.Int64()
	preVictimBal := testkeeper.EVMTestApp.BankKeeper.GetBalance(ctx, victimAddr, k.GetBaseDenom(ctx)).Amount.Int64()
	require.Zero(t, preAttackerBal)
	require.Equal(t, steal_amount, preVictimBal)

	_, err := k.HandleInternalEVMCall(ctx, req)
	require.ErrorContains(t, err, "invalid coins")

	// post verification
	postAttackerBal := testkeeper.EVMTestApp.BankKeeper.GetBalance(ctx, attackerAddr, k.GetBaseDenom(ctx)).Amount.Int64()
	postVictimBal := testkeeper.EVMTestApp.BankKeeper.GetBalance(ctx, victimAddr, k.GetBaseDenom(ctx)).Amount.Int64()
	require.Zero(t, postAttackerBal)
	require.Equal(t, steal_amount, postVictimBal)

	zeroVal := sdk.NewInt(0)
	req2 := &types.MsgInternalEVMCall{
		Sender: attackerAddr.String(),
		Data:   make([]byte, params.MaxInitCodeSize+1),
		Value:  &zeroVal,
	}

	_, err = k.HandleInternalEVMCall(ctx, req2)
	require.ErrorContains(t, err, "max initcode size exceeded")
}

func TestHandleInternalEVMDelegateCall_AssociationError(t *testing.T) {
	proveRegisteredDelegateCall(t)
}

func proveRegisteredDelegateCall(t *testing.T) {
	t.Helper()
	a := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	k := &a.EvmKeeper
	ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(1)
	sender, senderEVM := testkeeper.MockAddressPair()
	pointee := common.HexToAddress("0x124200")
	// Executable EVM fixture: return a single uint256 word containing 42.
	k.SetCode(ctx, pointee, common.FromHex("0x602a60005260206000f3"))
	register := func(target common.Address) sdk.AccAddress {
		res, err := evmkeeper.NewMsgServerImpl(k).RegisterPointer(sdk.WrapSDKContext(ctx), &types.MsgRegisterPointer{
			Sender: sender.String(), PointerType: types.PointerType_ERC20, ErcAddress: target.Hex(),
		})
		require.NoError(t, err)
		pointer, err := sdk.AccAddressFromBech32(res.PointerAddress)
		require.NoError(t, err)
		return pointer
	}
	pointer := register(pointee)
	codeInfo := a.WasmKeeper.GetCodeInfo(ctx, k.GetStoredPointerCodeID(ctx, types.PointerType_ERC20))
	require.NotNil(t, codeInfo)
	parsedABI := artifacts.GetParsedABI("native")
	calldata, err := parsedABI.Pack("totalSupply")
	require.NoError(t, err)
	payload, err := json.Marshal(bindings.DelegateCallEVM{To: pointee.Hex(), Data: base64.StdEncoding.EncodeToString(calldata)})
	require.NoError(t, err)
	encode := func(from sdk.AccAddress) *types.MsgInternalEVMDelegateCall {
		msgs, err := evmwasm.EncodeDelegateCallEVM(payload, from, wasmvmtypes.MessageInfo{Sender: sender.String()}, *codeInfo)
		require.NoError(t, err)
		require.Len(t, msgs, 1)
		// Cross the protobuf transport rather than handing the encoder's object directly to the handler.
		wire, err := msgs[0].(*types.MsgInternalEVMDelegateCall).Marshal()
		require.NoError(t, err)
		req := new(types.MsgInternalEVMDelegateCall)
		require.NoError(t, req.Unmarshal(wire))
		require.Equal(t, from.String(), req.FromContract)
		require.Equal(t, sender.String(), req.Sender)
		require.Equal(t, codeInfo.CodeHash, req.CodeHash)
		require.Equal(t, calldata, req.Data)
		return req
	}
	req := encode(pointer)
	_, err = k.HandleInternalEVMDelegateCall(ctx, req)
	require.EqualError(t, err, types.NewAssociationMissingErr(sender.String()).Error())
	k.SetAddressMapping(ctx, sender, senderEVM)
	res, err := k.HandleInternalEVMDelegateCall(ctx, req)
	require.NoError(t, err)
	decoded, err := parsedABI.Unpack("totalSupply", res.Data)
	require.NoError(t, err)
	require.Len(t, decoded, 1)
	require.Equal(t, big.NewInt(42), decoded[0])
	otherPointer := register(common.HexToAddress("0x124201"))
	_, err = k.HandleInternalEVMDelegateCall(ctx, encode(otherPointer))
	require.EqualError(t, err, "only pointer contract can make delegatecalls")
}
