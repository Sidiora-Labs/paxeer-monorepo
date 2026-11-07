package v630_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/ante"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	node "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/staking"
	stakingv630 "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/staking/legacy/v630"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/teststaking"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

const repair1210Height int64 = 100

func repair1210Fixture(t *testing.T) (*node.App, sdk.Context, common.Address, sdk.AccAddress) {
	t.Helper()
	a := testkeeper.EVMTestApp
	ctx, _ := a.GetContextForDeliverTx(nil).WithBlockHeight(repair1210Height).
		WithBlockTime(time.Unix(1_800_000_000, 0).UTC()).
		WithGasMeter(sdk.NewInfiniteGasMeter(1, 1)).CacheContext()
	key, err := crypto.ToECDSA(bytes.Repeat([]byte{0x12}, 32))
	require.NoError(t, err)
	caller := crypto.PubkeyToAddress(key.PublicKey)
	payer := sdk.AccAddress(caller.Bytes())
	a.EvmKeeper.SetAddressMapping(ctx, payer, caller)
	return a, ctx, caller, payer
}

func repair1210Fund(t *testing.T, a *node.App, ctx sdk.Context, address sdk.AccAddress, amount int64) {
	t.Helper()
	coins := sdk.NewCoins(sdk.NewInt64Coin(a.StakingKeeper.BondDenom(ctx), amount))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, evmtypes.ModuleName, coins))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, address, coins))
}

func repair1210Input(t *testing.T, contractABI abi.ABI, key string) []byte {
	t.Helper()
	input, err := contractABI.Pack(staking.CreateValidatorMethod, key, "repair-1210", "0.1", "0.2", "0.05", big.NewInt(1))
	require.NoError(t, err)
	return input
}

func repair1210Call(t *testing.T, ctx sdk.Context, k *evmkeeper.Keeper, runner vm.PrecompiledContract, caller common.Address, input []byte) ([]byte, error, *state.DBImpl) {
	t.Helper()
	db := state.NewDBImpl(ctx, k, true)
	evm := &vm.EVM{StateDB: db}
	value := big.NewInt(1_000_000_000_000_000_000)
	if dynamic, ok := runner.(vm.DynamicGasPrecompiledContract); ok {
		ret, _, err := dynamic.RunAndCalculateGas(evm, caller, caller, input, 20_000_000, value, nil, false, false)
		return ret, err, db
	}
	ret, err := runner.Run(evm, caller, caller, input, value, false, false, nil)
	return ret, err, db
}

func TestRepairM_1210_InvalidKeyBeforePayment(t *testing.T) {
	for _, size := range []int{0, 1, 31, 33, 64} {
		t.Run(fmt.Sprintf("bytes_%d", size), func(t *testing.T) {
			a, ctx, caller, payer := repair1210Fixture(t)
			a.UpgradeKeeper.SetDone(ctx, pcommon.MergeUpgrade)
			p, err := staking.NewPrecompile(a.GetPrecompileKeepers())
			require.NoError(t, err)
			precompileAccount := a.EvmKeeper.GetPaxAddressOrDefault(ctx, p.Address())
			// Fund the actual payment source, so a missing guard reaches the
			// keeper's consensus-address construction rather than a bank error.
			repair1210Fund(t, a, ctx, precompileAccount, 1_000_000)
			balances := a.BankKeeper.GetAllBalances(ctx, payer)
			precompileBalances := a.BankKeeper.GetAllBalances(ctx, precompileAccount)
			validators := a.StakingKeeper.GetAllValidators(ctx)
			supply := a.BankKeeper.GetSupply(ctx, a.StakingKeeper.BondDenom(ctx))
			input := repair1210Input(t, p.GetABI(), hex.EncodeToString(bytes.Repeat([]byte{1}, size)))
			ret, err, db := repair1210Call(t, ctx, &a.EvmKeeper, p, caller, input)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			require.Empty(t, ret)
			require.EqualError(t, db.GetPrecompileError(), "invalid public key length: expected 32 bytes")
			ctx = db.Ctx()
			require.Equal(t, balances, a.BankKeeper.GetAllBalances(ctx, payer))
			require.Equal(t, precompileBalances, a.BankKeeper.GetAllBalances(ctx, precompileAccount))
			require.Equal(t, supply, a.BankKeeper.GetSupply(ctx, a.StakingKeeper.BondDenom(ctx)))
			require.Equal(t, validators, a.StakingKeeper.GetAllValidators(ctx))
			_, found := a.StakingKeeper.GetDelegation(ctx, payer, sdk.ValAddress(payer))
			require.False(t, found)
		})
	}
}

func TestRepairM_1210_ValidKeyCreatesValidator(t *testing.T) {
	a, ctx, caller, payer := repair1210Fixture(t)
	a.UpgradeKeeper.SetDone(ctx, pcommon.MergeUpgrade)
	p, err := staking.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	precompileAccount := a.EvmKeeper.GetPaxAddressOrDefault(ctx, p.Address())
	repair1210Fund(t, a, ctx, precompileAccount, 1_000_000)
	key := ed25519.GenPrivKeyFromSecret([]byte("repair-1210-consensus-key")).PubKey()
	ret, err, db := repair1210Call(t, ctx, &a.EvmKeeper, p, caller, repair1210Input(t, p.GetABI(), hex.EncodeToString(key.Bytes())))
	require.NoError(t, err)
	ctx = db.Ctx()
	decoded, err := p.GetABI().Methods[staking.CreateValidatorMethod].Outputs.Unpack(ret)
	require.NoError(t, err)
	require.Equal(t, []interface{}{true}, decoded)
	validator, found := a.StakingKeeper.GetValidator(ctx, sdk.ValAddress(payer))
	require.True(t, found)
	storedKey, err := validator.ConsPubKey()
	require.NoError(t, err)
	require.Equal(t, key.Bytes(), storedKey.Bytes())
	require.Equal(t, "repair-1210", validator.Description.Moniker)
	require.Equal(t, sdk.NewInt(1_000_000), validator.Tokens)
	delegation, found := a.StakingKeeper.GetDelegation(ctx, payer, sdk.ValAddress(payer))
	require.True(t, found)
	require.Equal(t, sdk.NewDec(1_000_000), delegation.Shares)
	require.True(t, a.BankKeeper.GetAllBalances(ctx, precompileAccount).IsZero())
}

func TestRepairM_1210_KeyUpgradeReplay(t *testing.T) {
	a, base, caller, _ := repair1210Fixture(t)
	k := a.EvmKeeper
	address := common.HexToAddress(staking.StakingAddress)
	versions := staking.GetVersioned(pcommon.MergeUpgrade, a.GetPrecompileKeepers())
	k.SetCustomPrecompiles(map[common.Address]utils.VersionedPrecompiles{address: versions}, pcommon.MergeUpgrade)
	a.UpgradeKeeper.SetDone(base.WithBlockHeight(1), "v6.3.0")
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(base, pcommon.MergeUpgrade))
	archived, err := stakingv630.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	precompileAccount := k.GetPaxAddressOrDefault(base, address)
	repair1210Fund(t, a, base, precompileAccount, 1_000_000)
	input := repair1210Input(t, archived.GetABI(), hex.EncodeToString(bytes.Repeat([]byte{1}, 31)))
	for _, phase := range []string{"unscheduled", "scheduled", "applied"} {
		switch phase {
		case "scheduled":
			require.NoError(t, a.UpgradeKeeper.ScheduleUpgrade(base, upgradetypes.Plan{Name: pcommon.MergeUpgrade, Height: repair1210Height}))
		case "applied":
			a.UpgradeKeeper.SetDone(base.WithBlockHeight(repair1210Height), pcommon.MergeUpgrade)
		}
		for _, height := range []int64{1, repair1210Height - 1, repair1210Height, repair1210Height + 1} {
			for _, tracing := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/height_%d/tracing_%t", phase, height, tracing), func(t *testing.T) {
					ctx, _ := base.WithBlockHeight(height).WithIsTracing(tracing).CacheContext()
					selected := k.CustomPrecompiles(ctx)[address]
					require.NotNil(t, selected)
					active := phase == "applied" && height >= repair1210Height
					wantVersion := "v6.3.0"
					if active {
						wantVersion = pcommon.MergeUpgrade
					}
					require.Equal(t, wantVersion, k.GetCustomPrecompilesVersions(ctx)[address])
					if active {
						_, err, db := repair1210Call(t, ctx, &k, selected, caller, input)
						require.ErrorIs(t, err, vm.ErrExecutionReverted)
						require.EqualError(t, db.GetPrecompileError(), "invalid public key length: expected 32 bytes")
					} else {
						require.PanicsWithValue(t, "pubkey is incorrect size", func() {
							repair1210Call(t, ctx, &k, selected, caller, input)
						})
					}
					// The archived snapshot retains the old behavior at every height.
					legacyCtx, _ := base.WithBlockHeight(height).CacheContext()
					require.PanicsWithValue(t, "pubkey is incorrect size", func() {
						repair1210Call(t, legacyCtx, &k, archived, caller, input)
					})
				})
			}
		}
	}
}

func TestRepairM_1210_EVMTransactionRefusal(t *testing.T) {
	for _, size := range []int{0, 31, 33} {
		t.Run(fmt.Sprintf("bytes_%d", size), func(t *testing.T) {
			a, ctx, caller, payer := repair1210Fixture(t)
			a.UpgradeKeeper.SetDone(ctx, pcommon.MergeUpgrade)
			k := a.EvmKeeper
			address := common.HexToAddress(staking.StakingAddress)
			versions := staking.GetVersioned(pcommon.MergeUpgrade, a.GetPrecompileKeepers())
			k.SetCustomPrecompiles(map[common.Address]utils.VersionedPrecompiles{address: versions}, pcommon.MergeUpgrade)
			p := versions[pcommon.MergeUpgrade].(*pcommon.DynamicGasPrecompile)
			repair1210Fund(t, a, ctx, payer, 2_000_000_000_000)
			balances := a.BankKeeper.GetAllBalances(ctx, payer)
			precompileAccount := k.GetPaxAddressOrDefault(ctx, address)
			precompileBalances := a.BankKeeper.GetAllBalances(ctx, precompileAccount)
			validators := a.StakingKeeper.GetAllValidators(ctx)
			key, err := crypto.ToECDSA(bytes.Repeat([]byte{0x12}, 32))
			require.NoError(t, err)
			signer := ethtypes.MakeSigner(evmtypes.DefaultChainConfig().EthereumConfig(k.ChainID(ctx)), big.NewInt(ctx.BlockHeight()), uint64(ctx.BlockTime().Unix()))
			tx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
				GasPrice: big.NewInt(1_000_000_000_000), Gas: 2_000_000, To: &address,
				Value: big.NewInt(1_000_000_000_000_000_000),
				Data:  repair1210Input(t, p.GetABI(), hex.EncodeToString(bytes.Repeat([]byte{1}, size))),
			}), signer, key)
			require.NoError(t, err)
			require.Equal(t, caller, crypto.PubkeyToAddress(key.PublicKey))
			wrapped, err := ethtx.NewLegacyTx(tx)
			require.NoError(t, err)
			request, err := evmtypes.NewMsgEVMTransaction(wrapped)
			require.NoError(t, err)
			require.NoError(t, ante.Preprocess(ctx, request, k.ChainID(ctx), false))
			response, err := evmkeeper.NewMsgServerImpl(&k).EVMTransaction(sdk.WrapSDKContext(ctx), request)
			require.NoError(t, err)
			require.Equal(t, vm.ErrExecutionReverted.Error(), response.VmError)
			require.Empty(t, response.ReturnData)
			require.Empty(t, response.Logs)
			require.Equal(t, balances, a.BankKeeper.GetAllBalances(ctx, payer))
			require.Equal(t, precompileBalances, a.BankKeeper.GetAllBalances(ctx, precompileAccount))
			require.Equal(t, validators, a.StakingKeeper.GetAllValidators(ctx))
			_, found := a.StakingKeeper.GetValidator(ctx, sdk.ValAddress(payer))
			require.False(t, found)
		})
	}
}

// Produce the entry through BeginRedelegation, after a real slash has changed
// the destination's exchange rate. No query response or ABI output is supplied
// by the fixture: both are produced by the real keepers and precompile.
func repair1210RedelegationState(t *testing.T) (*node.App, sdk.Context, *stakingtypes.QueryRedelegationsRequest) {
	t.Helper()
	a := testkeeper.EVMTestApp
	ctx, _ := a.GetContextForDeliverTx(nil).WithBlockHeight(20).
		WithBlockTime(time.Unix(1_800_000_000, 0).UTC()).
		WithGasMeter(sdk.NewInfiniteGasMeter(1, 1)).CacheContext()
	k := a.StakingKeeper
	validators := make([]sdk.ValAddress, 2)
	for i := range validators {
		pubkey := ed25519.GenPrivKeyFromSecret([]byte(fmt.Sprintf("repair-1210-validator-%d", i))).PubKey()
		address := sdk.ValAddress(pubkey.Address())
		stake := k.TokensFromConsensusPower(ctx, 3)
		funds := sdk.NewCoins(sdk.NewCoin(k.BondDenom(ctx), stake))
		require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", funds))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", sdk.AccAddress(address), funds))
		h := teststaking.NewHelper(t, ctx, k)
		h.Denom = k.BondDenom(ctx)
		h.Handle(h.CreateValidatorMsg(address, pubkey, stake), true)
		validator, found := k.GetValidator(ctx, address)
		require.True(t, found)
		// Unbonding validators and newly created validators both use the
		// not-bonded pool; the future completion keeps the redelegation live.
		validator = validator.UpdateStatus(stakingtypes.Unbonding)
		validator.UnbondingTime = ctx.BlockTime().Add(24 * time.Hour)
		validator.UnbondingHeight = ctx.BlockHeight()
		k.SetValidator(ctx, validator)
		validators[i] = address
	}
	dst, found := k.GetValidator(ctx, validators[1])
	require.True(t, found)
	consensusAddress, err := dst.GetConsAddr()
	require.NoError(t, err)
	k.Slash(ctx, consensusAddress, ctx.BlockHeight(), 3, sdk.MustNewDecFromStr("0.4"))
	dst, found = k.GetValidator(ctx, validators[1])
	require.True(t, found)
	require.Equal(t, "0.600000000000000000", dst.TokensFromShares(sdk.OneDec()).String())

	delegator := sdk.AccAddress(validators[0])
	_, err = k.BeginRedelegation(ctx, delegator, validators[0], validators[1], sdk.NewDec(7))
	require.NoError(t, err)
	return a, ctx, &stakingtypes.QueryRedelegationsRequest{
		DelegatorAddr: delegator.String(), SrcValidatorAddr: validators[0].String(),
		DstValidatorAddr: validators[1].String(), Pagination: &query.PageRequest{},
	}
}

func repair1210Query(t *testing.T, a *node.App, ctx sdk.Context, req *stakingtypes.QueryRedelegationsRequest) *stakingtypes.QueryRedelegationsResponse {
	t.Helper()
	response, err := (stakingkeeper.Querier{Keeper: a.StakingKeeper}).Redelegations(sdk.WrapSDKContext(ctx), req)
	require.NoError(t, err)
	require.Len(t, response.RedelegationResponses, 1)
	require.Len(t, response.RedelegationResponses[0].Entries, 1)
	return response
}

func repair1210SharesCall(t *testing.T, ctx sdk.Context, k *evmkeeper.Keeper, runner vm.DynamicGasPrecompiledContract, contractABI abi.ABI, req *stakingtypes.QueryRedelegationsRequest) ([]byte, staking.RedelegationsResponse) {
	t.Helper()
	input, err := contractABI.Pack(staking.RedelegationsMethod, req.DelegatorAddr, req.SrcValidatorAddr, req.DstValidatorAddr, req.Pagination.Key)
	require.NoError(t, err)
	require.Equal(t, "7ac3ee49", common.Bytes2Hex(input[:4]), "frozen redelegations selector")
	evm := &vm.EVM{StateDB: state.NewDBImpl(ctx, k, true)}
	ret, _, err := runner.RunAndCalculateGas(evm, common.Address{}, common.Address{}, input, 20_000_000, nil, nil, true, false)
	require.NoError(t, err, "redelegations reverted: %x", ret)
	values, err := contractABI.Methods[staking.RedelegationsMethod].Outputs.Unpack(ret)
	require.NoError(t, err)
	require.Len(t, values, 1)
	decoded := abi.ConvertType(values[0], new(staking.RedelegationsResponse)).(*staking.RedelegationsResponse)
	require.Len(t, decoded.Redelegations, 1)
	require.Len(t, decoded.Redelegations[0].Entries, 1)
	return ret, *decoded
}

func TestRepairM_1210_RedelegationShares(t *testing.T) {
	a, base, request := repair1210RedelegationState(t)
	a.UpgradeKeeper.SetDone(base.WithBlockHeight(repair1210Height), pcommon.MergeUpgrade)
	precompile, err := staking.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	for _, sourceIndex := range []bool{false, true} {
		t.Run(fmt.Sprintf("source_index_%t", sourceIndex), func(t *testing.T) {
			req := *request
			if sourceIndex {
				req.DelegatorAddr, req.DstValidatorAddr = "", ""
			}
			ctx, _ := base.WithBlockHeight(repair1210Height).CacheContext()
			producer := repair1210Query(t, a, ctx, &req)
			entry := producer.RedelegationResponses[0].Entries[0]
			// SharesFromTokens uses Dec.QuoInt, which truncates to 18 places.
			delegator, err := sdk.AccAddressFromBech32(request.DelegatorAddr)
			require.NoError(t, err)
			src, err := sdk.ValAddressFromBech32(request.SrcValidatorAddr)
			require.NoError(t, err)
			dst, err := sdk.ValAddressFromBech32(request.DstValidatorAddr)
			require.NoError(t, err)
			stored, found := a.StakingKeeper.GetRedelegation(ctx, delegator, src, dst)
			require.True(t, found)
			require.Len(t, stored.Entries, 1)
			require.Equal(t, stored.Entries[0], entry.RedelegationEntry)
			require.Equal(t, "11.666666666666666666", entry.RedelegationEntry.SharesDst.String())
			require.NotEqual(t, entry.Balance.ToDec(), entry.RedelegationEntry.SharesDst)
			_, consumer := repair1210SharesCall(t, ctx, &a.EvmKeeper, precompile, precompile.GetABI(), &req)
			redel := consumer.Redelegations[0]
			require.Equal(t, request.DelegatorAddr, redel.DelegatorAddress)
			require.Equal(t, request.SrcValidatorAddr, redel.ValidatorSrcAddress)
			require.Equal(t, request.DstValidatorAddr, redel.ValidatorDstAddress)
			require.Equal(t, entry.RedelegationEntry.SharesDst.String(), redel.Entries[0].SharesDst)
			require.NotEqual(t, entry.Balance.String(), redel.Entries[0].SharesDst)
			require.Equal(t, entry.RedelegationEntry.InitialBalance.String(), redel.Entries[0].InitialBalance)
			require.Equal(t, entry.RedelegationEntry.CreationHeight, redel.Entries[0].CreationHeight)
			require.Equal(t, entry.RedelegationEntry.CompletionTime.Unix(), redel.Entries[0].CompletionTime)
			require.True(t, bytes.Equal(producer.Pagination.NextKey, consumer.NextKey), "pagination key must survive ABI encoding")
		})
	}
}

func TestRepairM_1210_SharesUpgradeReplay(t *testing.T) {
	a, base, req := repair1210RedelegationState(t)
	k := a.EvmKeeper
	address := common.HexToAddress(staking.StakingAddress)
	versions := staking.GetVersioned(pcommon.MergeUpgrade, a.GetPrecompileKeepers())
	k.SetCustomPrecompiles(map[common.Address]utils.VersionedPrecompiles{address: versions}, pcommon.MergeUpgrade)
	a.UpgradeKeeper.SetDone(base.WithBlockHeight(1), "v6.3.0")
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(base, pcommon.MergeUpgrade))
	legacy, err := stakingv630.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	current := versions[pcommon.MergeUpgrade].(*pcommon.DynamicGasPrecompile)
	contractABI := current.GetABI()
	producer := repair1210Query(t, a, base, req)
	entry := producer.RedelegationResponses[0].Entries[0]

	for _, phase := range []string{"unscheduled", "scheduled", "applied"} {
		switch phase {
		case "scheduled":
			require.NoError(t, a.UpgradeKeeper.ScheduleUpgrade(base, upgradetypes.Plan{
				Name: pcommon.MergeUpgrade, Height: repair1210Height,
			}))
		case "applied":
			a.UpgradeKeeper.SetDone(base.WithBlockHeight(repair1210Height), pcommon.MergeUpgrade)
		}
		for _, height := range []int64{1, repair1210Height - 1, repair1210Height, repair1210Height + 1} {
			for _, tracing := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/height_%d/tracing_%t", phase, height, tracing), func(t *testing.T) {
					ctx, _ := base.WithBlockHeight(height).WithIsTracing(tracing).CacheContext()
					served, found := k.CustomPrecompiles(ctx)[address]
					require.True(t, found)
					runner, ok := served.(vm.DynamicGasPrecompiledContract)
					require.True(t, ok)
					wantVersion := "v6.3.0"
					if phase == "applied" && height >= repair1210Height {
						wantVersion = pcommon.MergeUpgrade
					}
					require.Equal(t, wantVersion, k.GetCustomPrecompilesVersions(ctx)[address])
					actual, decoded := repair1210SharesCall(t, ctx, &k, runner, contractABI, req)
					legacyCtx, _ := ctx.CacheContext()
					historical, historicalDecoded := repair1210SharesCall(t, legacyCtx, &k, legacy, contractABI, req)
					require.Equal(t, entry.Balance.String(), historicalDecoded.Redelegations[0].Entries[0].SharesDst,
						"archived implementation must preserve historical bytes even after activation")
					if phase == "applied" && height >= repair1210Height {
						require.Equal(t, entry.RedelegationEntry.SharesDst.String(), decoded.Redelegations[0].Entries[0].SharesDst)
						expected := historicalDecoded
						expected.Redelegations[0].Entries[0].SharesDst = entry.RedelegationEntry.SharesDst.String()
						expectedBytes, err := contractABI.Methods[staking.RedelegationsMethod].Outputs.Pack(expected)
						require.NoError(t, err)
						require.Equal(t, expectedBytes, actual, "only the sharesDst projection changes")
					} else {
						require.Equal(t, entry.Balance.String(), decoded.Redelegations[0].Entries[0].SharesDst)
						require.Equal(t, historical, actual, "pre-activation replay must be byte identical")
					}
				})
			}
		}
	}
}

func TestRepairM_1210_ABICompatibility(t *testing.T) {
	a, _, _, _ := repair1210Fixture(t)
	current, err := staking.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	historical, err := stakingv630.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	require.Equal(t, historical.Address(), current.Address())
	currentABI, historicalABI := current.GetABI(), historical.GetABI()
	require.Len(t, currentABI.Methods, len(historicalABI.Methods))
	for name, method := range historicalABI.Methods {
		active, found := currentABI.Methods[name]
		require.True(t, found, name)
		require.Equal(t, method.Sig, active.Sig)
		require.Equal(t, method.ID, active.ID)
	}
	for name, event := range historicalABI.Events {
		active, found := currentABI.Events[name]
		require.True(t, found, name)
		require.Equal(t, event.Sig, active.Sig)
		require.Equal(t, event.ID, active.ID)
		require.Equal(t, event.Inputs, active.Inputs)
	}
	// Consumers keep the same tuple schema on both sides of the semantic change.
	require.Equal(t, historicalABI.Methods[staking.RedelegationsMethod].Outputs,
		currentABI.Methods[staking.RedelegationsMethod].Outputs)
}
