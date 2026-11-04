package keeper

import (
	"errors"
	"strings"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
)

// ShareDenomPrefix opens every vault share denom; the spot module account
// mints and burns no other denom.
const ShareDenomPrefix = "spot/share/"

// ShareTransferTopic is topic0 of the ERC20 Transfer(address,address,uint256)
// log the spot keeper writes at a share pointer on each share mint and burn.
var ShareTransferTopic = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

// ShareDenom is the bank denom of the vault shares of token,
// spot/share/<lowercase hex address>.
func (k *Keeper) ShareDenom(token common.Address) string {
	return ShareDenomPrefix + strings.ToLower(token.Hex())
}

// isShareDenom reports whether denom is the share denom of some token.
func (k *Keeper) isShareDenom(denom string) bool {
	hex, ok := strings.CutPrefix(denom, ShareDenomPrefix)
	return ok && common.IsHexAddress(hex) && k.ShareDenom(common.HexToAddress(hex)) == denom
}

// CreateShareToken writes the bank metadata of the share denom of token,
// named "Spot Vault Share <symbol>" with symbol "s<symbol>" and the token
// decimals, and creates its ERC20 native pointer, NativePaxTokensERC20, whose
// balanceOf and totalSupply read the bank module. It returns the pointer
// address. The deployment interface is precompiles/spot/abi/share.json:
// allowance, approve, balanceOf, decimals, name, symbol, totalSupply, transfer
// and transferFrom, plus the native pointer's metadata/bank getters. Mint,
// burn, owner, renounceOwnership and transferOwnership are unsupported and
// revert with empty data for every caller, including governance and the vault.
// MintShares and BurnShares retain exclusive keeper authority; there is no
// Solidity owner. The old Solidity ABI is share-solidity.json, for history only.
// A share-specific delegate boundary preserves native execution and returns
// ERC20InsufficientBalance with the bank balance when a transfer exceeds it.
// evm is the instance the pointer is deployed in, the one-off
// instance of the activation (RunWithOneOffEVMInstance).
func (k *Keeper) CreateShareToken(ctx sdk.Context, evm *vm.EVM, token common.Address, symbol string,
	decimals uint8) (common.Address, error) {
	if token == (common.Address{}) {
		return common.Address{}, types.ErrZeroAddress
	}
	if evm == nil {
		return common.Address{}, sdkerrors.ErrInvalidRequest.Wrap("spot: share pointer needs an EVM instance")
	}
	if strings.TrimSpace(symbol) == "" || decimals > types.MaxTokenDecimals {
		return common.Address{}, types.ErrUnsupportedToken
	}
	denom := k.ShareDenom(token)
	if _, _, exists := k.evmKeeper.GetERC20NativePointer(ctx, denom); exists {
		return common.Address{}, types.ErrTokenAlreadyRegistered
	}
	name := "Spot Vault Share " + symbol
	shareSymbol := "s" + symbol
	metadata := banktypes.Metadata{
		Description: name,
		DenomUnits:  []*banktypes.DenomUnit{{Denom: denom, Exponent: 0}},
		Base:        denom,
		Display:     denom,
		Name:        name,
		Symbol:      shareSymbol,
	}
	if decimals > 0 {
		metadata.DenomUnits = append(metadata.DenomUnits,
			&banktypes.DenomUnit{Denom: shareSymbol, Exponent: uint32(decimals)})
		metadata.Display = shareSymbol
	}
	if err := metadata.Validate(); err != nil {
		return common.Address{}, types.ErrUnsupportedToken
	}
	k.bankKeeper.SetDenomMetaData(ctx, metadata)
	pointer, err := k.evmKeeper.UpsertERCNativePointer(ctx, evm, denom,
		utils.ERCMetadata{Name: name, Symbol: shareSymbol, Decimals: decimals})
	if err != nil {
		return common.Address{}, err
	}
	if err := k.installShareBalanceBoundary(ctx, evm, pointer); err != nil {
		return common.Address{}, err
	}
	return pointer, nil
}

// installShareBalanceBoundary keeps native storage and bank authorization at
// pointer. Its immutable delegate contains the original native runtime, while
// the pointer translates only the native transfer-failed revert when the real
// bank balance is insufficient. Native allowance/address checks run first.
func (k *Keeper) installShareBalanceBoundary(ctx sdk.Context, evm *vm.EVM, pointer common.Address) error {
	gasKeeper, ok := k.evmKeeper.(interface {
		GetEVMGasLimitFromCtx(sdk.Context) uint64
		GetCosmosGasLimitFromEVMGas(sdk.Context, uint64) uint64
	})
	if !ok {
		return sdkerrors.ErrInvalidRequest.Wrap("spot: share deployment requires EVM gas accounting")
	}
	runtime := evm.StateDB.GetCode(pointer)
	if len(runtime) == 0 || len(runtime) > 65535 {
		return sdkerrors.ErrInvalidRequest.Wrap("spot: invalid native share runtime")
	}
	// Init code returns the existing runtime verbatim; no second constructor or
	// storage initialization runs. All delegate calls use the pointer's storage.
	length := []byte{byte(len(runtime) >> 8), byte(len(runtime))}
	init := []byte{byte(vm.PUSH2), length[0], length[1], byte(vm.PUSH2), 0, 15,
		byte(vm.PUSH1), 0, byte(vm.CODECOPY), byte(vm.PUSH2), length[0], length[1],
		byte(vm.PUSH1), 0, byte(vm.RETURN)}
	init = append(init, runtime...)
	gas := gasKeeper.GetEVMGasLimitFromCtx(ctx)
	if run := runOf(ctx); run != nil {
		gas = run.Gas
	}
	_, implementation, left, err := evm.Create(pointer, init, gas, uint256.NewInt(0))
	if run := runOf(ctx); run != nil {
		run.Gas = left
	} else {
		ctx.GasMeter().ConsumeGas(gasKeeper.GetCosmosGasLimitFromEVMGas(ctx, gas-left), "share balance boundary deployment")
	}
	if err != nil {
		return err
	}
	evm.StateDB.SetCode(pointer, nativeShareRuntime(implementation))
	return nil
}

// nativeShareRuntime assembles the immutable share boundary directly from EVM
// instructions. It needs no Solidity compiler or generated deployment artifact.
// Successful calls and all other refusals retain the native return bytes.
func nativeShareRuntime(implementation common.Address) []byte {
	var code []byte
	labels := make(map[string]int)
	type reference struct {
		offset int
		label  string
	}
	var references []reference
	op := func(ops ...vm.OpCode) {
		for _, instruction := range ops {
			code = append(code, byte(instruction))
		}
	}
	pushBytes := func(value []byte) {
		code = append(code, byte(vm.PUSH1)+byte(len(value))-1)
		code = append(code, value...)
	}
	push := func(value uint16) {
		code = append(code, byte(vm.PUSH2), byte(value>>8), byte(value))
	}
	jump := func(label string, conditional bool) {
		op(vm.PUSH2)
		references = append(references, reference{len(code), label})
		code = append(code, 0, 0)
		if conditional {
			op(vm.JUMPI)
		} else {
			op(vm.JUMP)
		}
	}
	label := func(name string) {
		labels[name] = len(code)
		op(vm.JUMPDEST)
	}

	// Delegate calldata unchanged. Save returndata above the scratch area.
	op(vm.CALLDATASIZE)
	push(0)
	push(0)
	op(vm.CALLDATACOPY)
	push(0)
	push(0)
	op(vm.CALLDATASIZE)
	push(0)
	pushBytes(implementation.Bytes())
	op(vm.GAS, vm.DELEGATECALL, vm.RETURNDATASIZE)
	push(0)
	push(0x100)
	op(vm.RETURNDATACOPY)
	jump("return", true)
	op(vm.RETURNDATASIZE)
	push(0x80)
	op(vm.MSTORE)
	// Match the complete native Error(string), preserving other errors exactly.
	op(vm.RETURNDATASIZE)
	push(0x100)
	op(vm.KECCAK256)
	pushBytes(crypto.Keccak256(pcommon.RevertReason(errors.New("NativePaxTokensERC20: transfer failed"))))
	op(vm.EQ, vm.ISZERO)
	jump("revert", true)
	push(0)
	op(vm.CALLDATALOAD)
	push(224)
	op(vm.SHR, vm.DUP1)
	pushBytes(erc20Transfer)
	op(vm.EQ)
	jump("transfer", true)
	pushBytes(erc20TransferFrom)
	op(vm.EQ, vm.ISZERO)
	jump("revert", true)
	push(4)
	op(vm.CALLDATALOAD)
	push(0x40)
	op(vm.MSTORE)
	push(68)
	op(vm.CALLDATALOAD)
	jump("balance", false)
	label("transfer")
	op(vm.POP, vm.CALLER)
	push(0x40)
	op(vm.MSTORE)
	push(36)
	op(vm.CALLDATALOAD)
	label("balance")
	push(0x60)
	op(vm.MSTORE)
	// Static-call the pointer's native balanceOf to read the real bank balance.
	pushBytes(erc20BalanceOf)
	push(224)
	op(vm.SHL)
	push(0)
	op(vm.MSTORE)
	push(0x40)
	op(vm.MLOAD)
	push(4)
	op(vm.MSTORE)
	push(32)
	push(0xa0)
	push(36)
	push(0)
	op(vm.ADDRESS, vm.GAS, vm.STATICCALL, vm.ISZERO)
	jump("revert", true)
	op(vm.RETURNDATASIZE)
	push(32)
	op(vm.EQ, vm.ISZERO)
	jump("revert", true)
	push(0x60)
	op(vm.MLOAD)
	push(0xa0)
	op(vm.MLOAD, vm.LT, vm.ISZERO)
	jump("revert", true)
	// ERC20InsufficientBalance(address sender,uint256 balance,uint256 needed).
	pushBytes(types.ERC20InsufficientBalance(common.Address{}, uint256.NewInt(0), uint256.NewInt(0)).Data()[:4])
	push(224)
	op(vm.SHL)
	push(0)
	op(vm.MSTORE)
	push(0x40)
	op(vm.MLOAD)
	push(4)
	op(vm.MSTORE)
	push(0xa0)
	op(vm.MLOAD)
	push(36)
	op(vm.MSTORE)
	push(0x60)
	op(vm.MLOAD)
	push(68)
	op(vm.MSTORE)
	push(100)
	push(0)
	op(vm.REVERT)
	label("revert")
	push(0x80)
	op(vm.MLOAD)
	push(0x100)
	op(vm.REVERT)
	label("return")
	op(vm.RETURNDATASIZE)
	push(0x100)
	op(vm.RETURN)
	for _, ref := range references {
		destination := labels[ref.label]
		code[ref.offset], code[ref.offset+1] = byte(destination>>8), byte(destination)
	}
	return code
}

// sharePointer checks a share mint or burn of amount of denom in evm and
// returns the share pointer and the coins it moves.
func (k *Keeper) sharePointer(ctx sdk.Context, evm *vm.EVM, denom string, amount sdk.Int) (common.Address, sdk.Coins, error) {
	if !k.isShareDenom(denom) {
		return common.Address{}, nil, types.ErrUnsupportedToken
	}
	if evm == nil {
		return common.Address{}, nil, sdkerrors.ErrInvalidRequest.Wrap("spot: shares are minted and burned only inside a transaction")
	}
	if amount.IsNil() || !amount.IsPositive() {
		return common.Address{}, nil, types.ErrZeroAmount
	}
	if amount.BigInt().BitLen() > 256 {
		return common.Address{}, nil, sdkerrors.ErrInvalidRequest.Wrap("spot: share amount exceeds uint256")
	}
	pointer, _, exists := k.evmKeeper.GetERC20NativePointer(ctx, denom)
	if !exists {
		return common.Address{}, nil, types.ErrTokenNotRegistered
	}
	return pointer, sdk.NewCoins(sdk.NewCoin(denom, amount)), nil
}

// emitShareTransfer writes Transfer(from, to, amount) at the share pointer,
// the log the pointer's _update would write, which a bank mint or burn never
// runs.
func emitShareTransfer(evm *vm.EVM, pointer, from, to common.Address, amount sdk.Int) error {
	return pcommon.EmitEVMLog(evm, pointer,
		[]common.Hash{ShareTransferTopic, common.BytesToHash(from.Bytes()), common.BytesToHash(to.Bytes())},
		common.LeftPadBytes(amount.BigInt().Bytes(), 32))
}

// MintShares mints amount of the share denom to to through the bank keeper
// with the spot module account and writes Transfer(address(0), to, amount)
// at the share pointer. It refuses any denom outside spot/share/* and runs
// only inside a transaction, evm being the instance of that transaction.
func (k *Keeper) MintShares(ctx sdk.Context, evm *vm.EVM, denom string, to common.Address, amount sdk.Int) error {
	pointer, coins, err := k.sharePointer(ctx, evm, denom, amount)
	if err != nil {
		return err
	}
	if to == (common.Address{}) {
		return types.ERC20InvalidReceiver(to)
	}
	if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, coins); err != nil {
		return err
	}
	if recipient := k.evmKeeper.GetPaxAddressOrDefault(ctx, to); !recipient.Equals(CustodyAddress()) {
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, recipient, coins); err != nil {
			return err
		}
	}
	return emitShareTransfer(evm, pointer, common.Address{}, to, amount)
}

// BurnShares burns amount of the share denom held by from through the bank
// keeper with the spot module account and writes Transfer(from, address(0),
// amount) at the share pointer. It refuses any denom outside spot/share/*,
// returns ERC20InsufficientBalance when from holds less than amount, and runs
// only inside a transaction, evm being the instance of that transaction.
func (k *Keeper) BurnShares(ctx sdk.Context, evm *vm.EVM, denom string, from common.Address, amount sdk.Int) error {
	pointer, coins, err := k.sharePointer(ctx, evm, denom, amount)
	if err != nil {
		return err
	}
	if from == (common.Address{}) {
		return types.ERC20InvalidSender(from)
	}
	holder := k.evmKeeper.GetPaxAddressOrDefault(ctx, from)
	if balance := k.bankKeeper.GetBalance(ctx, holder, denom).Amount; balance.LT(amount) {
		held, _ := uint256.FromBig(balance.BigInt())
		needed, _ := uint256.FromBig(amount.BigInt())
		return types.ERC20InsufficientBalance(from, held, needed)
	}
	if !holder.Equals(CustodyAddress()) {
		if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, holder, types.ModuleName, coins); err != nil {
			return err
		}
	}
	if err := k.bankKeeper.BurnCoins(ctx, types.ModuleName, coins); err != nil {
		return err
	}
	return emitShareTransfer(evm, pointer, from, common.Address{}, amount)
}
