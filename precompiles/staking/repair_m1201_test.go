package staking_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/ante"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/staking"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func repairM1201TransactionSetup(t *testing.T, height int64) *createValidatorTestSetup {
	t.Helper()
	s := setupCreateValidatorTest(t)
	s.ctx = s.ctx.WithBlockHeight(height).WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	k := *s.k
	s.k = &k
	keepers := app.NewPrecompileKeepers(s.testApp)
	keepers.EVMKeeper = s.k
	s.k.SetCustomPrecompiles(map[common.Address]utils.VersionedPrecompiles{
		s.addr: staking.GetVersioned(pcommon.MergeUpgrade, keepers),
	}, pcommon.MergeUpgrade)
	s.msgServer = evmkeeper.NewMsgServerImpl(s.k)
	return s
}

func repairM1201Send(t *testing.T, s *createValidatorTestSetup, pubkey string) (*evmtypes.MsgEVMTransactionResponse, *evmtypes.Receipt) {
	t.Helper()
	input, err := s.abi.Pack(staking.CreateValidatorMethod, pubkey, "RepairM1201", "0.05", "0.20", "0.01", big.NewInt(1_000_000))
	require.NoError(t, err)
	tx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
		GasPrice: big.NewInt(1_000_000_000_000), Gas: 20_000_000,
		To: &s.addr, Value: big.NewInt(1_000_000_000_000_000_000), Data: input,
	}), s.signer, s.key)
	require.NoError(t, err)
	wrapped, err := ethtx.NewLegacyTx(tx)
	require.NoError(t, err)
	msg, err := evmtypes.NewMsgEVMTransaction(wrapped)
	require.NoError(t, err)
	require.NoError(t, ante.Preprocess(s.ctx, msg, s.k.ChainID(s.ctx), false))
	response, err := s.msgServer.EVMTransaction(sdk.WrapSDKContext(s.ctx), msg)
	require.NoError(t, err)
	receipt, err := s.k.GetTransientReceipt(s.ctx, tx.Hash(), uint64(s.ctx.TxIndex()))
	require.NoError(t, err)
	return response, receipt
}

func TestRepairM_1201_ValidatorKeyLength(t *testing.T) {
	for _, height := range []int64{repair126PlanHeight, repair126PlanHeight + 1} {
		for _, size := range []int{0, 31, 32, 33} {
			t.Run(fmt.Sprintf("height_%d/bytes_%d", height, size), func(t *testing.T) {
				s := repairM1201TransactionSetup(t, height)
				s.testApp.UpgradeKeeper.SetDone(s.ctx.WithBlockHeight(repair126PlanHeight), pcommon.MergeUpgrade)
				pax, _ := testkeeper.PrivateKeyToAddresses(s.testPrivKey)
				validatorsBefore := s.testApp.StakingKeeper.GetAllValidators(s.ctx)
				callerBefore := s.testApp.BankKeeper.GetAllBalances(s.ctx, pax)
				precompileAddress := s.k.GetPaxAddressOrDefault(s.ctx, s.addr)
				precompileBefore := s.testApp.BankKeeper.GetAllBalances(s.ctx, precompileAddress)
				poolBefore := s.testApp.StakingKeeper.GetNotBondedPool(s.ctx).GetAddress()
				poolBalanceBefore := s.testApp.BankKeeper.GetAllBalances(s.ctx, poolBefore)
				key := hex.EncodeToString(bytes.Repeat([]byte{0x42}, size))
				response, receipt := repairM1201Send(t, s, key)
				if size == ed25519.PubKeySize {
					require.Empty(t, response.VmError)
					require.Equal(t, uint32(ethtypes.ReceiptStatusSuccessful), receipt.Status)
					decoded, err := s.abi.Methods[staking.CreateValidatorMethod].Outputs.Unpack(response.ReturnData)
					require.NoError(t, err)
					require.Equal(t, []interface{}{true}, decoded)
					validator, found := s.testApp.StakingKeeper.GetValidator(s.ctx, sdk.ValAddress(pax))
					require.True(t, found)
					pubkey, err := validator.ConsPubKey()
					require.NoError(t, err)
					require.Equal(t, key, hex.EncodeToString(pubkey.Bytes()))
					return
				}
				reason := fmt.Sprintf("invalid public key length: expected 32 bytes, got %d", size)
				require.Equal(t, vm.ErrExecutionReverted.Error(), response.VmError)
				require.Nil(t, response.ReturnData)
				require.Equal(t, uint32(ethtypes.ReceiptStatusFailed), receipt.Status)
				require.Contains(t, receipt.VmError, reason)
				require.Equal(t, validatorsBefore, s.testApp.StakingKeeper.GetAllValidators(s.ctx))
				_, found := s.testApp.StakingKeeper.GetValidator(s.ctx, sdk.ValAddress(pax))
				require.False(t, found)
				require.Equal(t, callerBefore, s.testApp.BankKeeper.GetAllBalances(s.ctx, pax))
				require.Equal(t, precompileBefore, s.testApp.BankKeeper.GetAllBalances(s.ctx, precompileAddress))
				require.Equal(t, poolBalanceBefore, s.testApp.BankKeeper.GetAllBalances(s.ctx, poolBefore))
			})
		}
	}
}

func TestRepairM_1201_ForwardVersionPolicy(t *testing.T) {
	s := repairM1201TransactionSetup(t, repair126PlanHeight-1)
	require.Zero(t, s.testApp.UpgradeKeeper.GetDoneHeight(s.ctx, pcommon.MergeUpgrade))
	s.testApp.UpgradeKeeper.SetDone(s.ctx.WithBlockHeight(1), "v6.4.0")
	for _, phase := range []string{"unscheduled", "scheduled", "applied"} {
		switch phase {
		case "scheduled":
			require.NoError(t, s.testApp.UpgradeKeeper.ScheduleUpgrade(s.ctx, upgradetypes.Plan{Name: pcommon.MergeUpgrade, Height: repair126PlanHeight}))
		case "applied":
			s.testApp.UpgradeKeeper.SetDone(s.ctx.WithBlockHeight(repair126PlanHeight), pcommon.MergeUpgrade)
		}
		for _, height := range []int64{repair126PlanHeight - 1, repair126PlanHeight, repair126PlanHeight + 1} {
			for _, tracing := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/height_%d/tracing_%t", phase, height, tracing), func(t *testing.T) {
					ctx, _ := s.ctx.WithBlockHeight(height).WithIsTracing(tracing).CacheContext()
					p, found := s.k.CustomPrecompiles(ctx)[s.addr]
					require.True(t, found)
					runner, ok := p.(vm.DynamicGasPrecompiledContract)
					require.True(t, ok)
					input, err := s.abi.Pack(staking.CreateValidatorMethod, "", "RepairM1201", "0.05", "0.20", "0.01", big.NewInt(1_000_000))
					require.NoError(t, err)
					_, caller := testkeeper.PrivateKeyToAddresses(s.testPrivKey)
					db := state.NewDBImpl(ctx, s.k, true)
					_, _, err = runner.RunAndCalculateGas(&vm.EVM{StateDB: db}, caller, caller, input, 20_000_000, nil, nil, false, false)
					require.ErrorIs(t, err, vm.ErrExecutionReverted)
					reason := "set `value` field to non-zero to send delegate fund"
					if phase == "applied" && height >= repair126PlanHeight {
						reason = "invalid public key length: expected 32 bytes, got 0"
					}
					require.EqualError(t, db.GetPrecompileError(), reason)
				})
			}
		}
	}
}

func TestRepairM_1201_StakingErrorAssertion(t *testing.T) {
	t.Run("createValidator refusal reasons", TestCreateValidator)
	for _, height := range []int64{repair126PlanHeight - 1, repair126PlanHeight} {
		t.Run(fmt.Sprintf("validators/height_%d", height), func(t *testing.T) {
			s := repairM1201TransactionSetup(t, height)
			s.testApp.UpgradeKeeper.SetDone(s.ctx.WithBlockHeight(1), "v6.4.0")
			s.testApp.UpgradeKeeper.SetDone(s.ctx.WithBlockHeight(repair126PlanHeight), pcommon.MergeUpgrade)
			p, found := s.k.CustomPrecompiles(s.ctx)[s.addr]
			require.True(t, found)
			runner, ok := p.(vm.DynamicGasPrecompiledContract)
			require.True(t, ok)
			input, err := s.abi.Pack(staking.ValidatorsMethod, "BOND_STATUS_BONDED", []byte{})
			require.NoError(t, err)
			db := state.NewDBImpl(s.ctx, s.k, true)
			ret, _, err := runner.RunAndCalculateGas(&vm.EVM{StateDB: db}, common.Address{}, common.Address{}, input, 20_000_000, big.NewInt(1), nil, true, false)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			require.Nil(t, ret)
			require.Error(t, db.GetPrecompileError())
			require.Contains(t, db.GetPrecompileError().Error(), "sending funds to a non-payable function")
		})
	}
}

func repairM1201RedelegationShares(t *testing.T) {
	t.Helper()
	a, ctx, req := repair126RedelegationState(t)
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(repair126PlanHeight), pcommon.MergeUpgrade)
	ctx = ctx.WithBlockHeight(repair126PlanHeight)
	del := sdk.MustAccAddressFromBech32(req.DelegatorAddr)
	src, err := sdk.ValAddressFromBech32(req.SrcValidatorAddr)
	require.NoError(t, err)
	dst, err := sdk.ValAddressFromBech32(req.DstValidatorAddr)
	require.NoError(t, err)
	persisted, found := a.StakingKeeper.GetRedelegation(ctx, del, src, dst)
	require.True(t, found)
	require.Len(t, persisted.Entries, 1)
	entry := persisted.Entries[0]
	require.Equal(t, "11.666666666666666666", entry.SharesDst.String())
	producer := repair126Query(t, a, ctx, req)
	require.NotEqual(t, producer.RedelegationResponses[0].Entries[0].Balance.ToDec(), entry.SharesDst)
	p, err := staking.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	_, decoded := repair126Call(t, ctx, &a.EvmKeeper, p, p.GetABI(), req)
	require.Equal(t, entry.SharesDst.String(), decoded.Redelegations[0].Entries[0].SharesDst)
	require.Equal(t, entry.InitialBalance.String(), decoded.Redelegations[0].Entries[0].InitialBalance)
}

func TestRepairM_1201_SharesTestFixture(t *testing.T) {
	repairM1201RedelegationShares(t)
}

func repairM1201StaticDelegation(t *testing.T) {
	t.Helper()
	a, ctx, req := repair126RedelegationState(t)
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(repair126PlanHeight), pcommon.MergeUpgrade)
	ctx = ctx.WithBlockHeight(repair126PlanHeight)
	del := sdk.MustAccAddressFromBech32(req.DelegatorAddr)
	dst, err := sdk.ValAddressFromBech32(req.DstValidatorAddr)
	require.NoError(t, err)
	persisted, found := a.StakingKeeper.GetDelegation(ctx, del, dst)
	require.True(t, found)
	_, caller := testkeeper.MockAddressPair()
	k := a.EvmKeeper
	k.SetAddressMapping(ctx, del, caller)
	keepers := app.NewPrecompileKeepers(a)
	keepers.EVMKeeper = &k
	k.SetCustomPrecompiles(map[common.Address]utils.VersionedPrecompiles{
		common.HexToAddress(staking.StakingAddress): staking.GetVersioned(pcommon.MergeUpgrade, keepers),
	}, pcommon.MergeUpgrade)
	registered, found := k.CustomPrecompiles(ctx)[common.HexToAddress(staking.StakingAddress)]
	require.True(t, found)
	runner, ok := registered.(vm.DynamicGasPrecompiledContract)
	require.True(t, ok)
	contractABI := pcommon.MustGetABI(f, "abi.json")
	producer, err := (stakingkeeper.Querier{Keeper: a.StakingKeeper}).Delegation(sdk.WrapSDKContext(ctx), &stakingtypes.QueryDelegationRequest{
		DelegatorAddr: del.String(), ValidatorAddr: dst.String(),
	})
	require.NoError(t, err)
	input, err := contractABI.Pack(staking.DelegationMethod, caller, dst.String())
	require.NoError(t, err)
	db := state.NewDBImpl(ctx, &k, true)
	ret, _, err := runner.RunAndCalculateGas(&vm.EVM{StateDB: db}, caller, caller, input, 20_000_000, nil, nil, true, false)
	require.NoError(t, err)
	values, err := contractABI.Methods[staking.DelegationMethod].Outputs.Unpack(ret)
	require.NoError(t, err)
	require.Len(t, values, 1)
	decoded := abi.ConvertType(values[0], new(staking.Delegation)).(*staking.Delegation)
	require.Equal(t, producer.DelegationResponse.Balance.Amount.BigInt(), decoded.Balance.Amount)
	require.Equal(t, producer.DelegationResponse.Balance.Denom, decoded.Balance.Denom)
	require.Equal(t, persisted.Shares.BigInt(), decoded.Delegation.Shares)
	require.Equal(t, big.NewInt(sdk.Precision), decoded.Delegation.Decimals)
	require.Equal(t, persisted.DelegatorAddress, decoded.Delegation.DelegatorAddress)
	require.Equal(t, persisted.ValidatorAddress, decoded.Delegation.ValidatorAddress)
	require.NotEqual(t, producer.DelegationResponse.Balance.Amount.ToDec(), persisted.Shares)
	for _, method := range []struct {
		name string
		args []interface{}
	}{
		{staking.DelegateMethod, []interface{}{dst.String()}},
		{staking.RedelegateMethod, []interface{}{req.SrcValidatorAddr, dst.String(), big.NewInt(1)}},
		{staking.UndelegateMethod, []interface{}{dst.String(), big.NewInt(1)}},
		{staking.CreateValidatorMethod, []interface{}{hex.EncodeToString(ed25519.GenPrivKey().PubKey().Bytes()), "RepairM1201", "0.05", "0.20", "0.01", big.NewInt(1)}},
		{staking.EditValidatorMethod, []interface{}{"RepairM1201", "0.05", big.NewInt(1)}},
	} {
		t.Run(method.name+" static refusal", func(t *testing.T) {
			input, err := contractABI.Pack(method.name, method.args...)
			require.NoError(t, err)
			db := state.NewDBImpl(ctx, &k, true)
			_, _, err = runner.RunAndCalculateGas(&vm.EVM{StateDB: db}, caller, caller, input, 20_000_000, nil, nil, true, false)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			require.EqualError(t, db.GetPrecompileError(), "cannot call staking precompile from staticcall")
		})
	}
	after, found := a.StakingKeeper.GetDelegation(ctx, del, dst)
	require.True(t, found)
	require.Equal(t, persisted, after)
}

func TestRepairM_1201_StaticQueryNoAssertion(t *testing.T) {
	repairM1201StaticDelegation(t)
}
