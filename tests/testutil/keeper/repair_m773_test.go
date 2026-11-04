package keeper

import (
	"fmt"
	"math/big"
	"sync"
	"testing"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/bank"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestRepairM_773_SequentialFixtures(t *testing.T) {
	addr := common.HexToAddress("0x773")
	slot := common.HexToHash("0x773")
	value := common.HexToHash("0x1234")
	paxAddr := sdk.AccAddress([]byte("paxAddr"))

	first, firstCtx := MockEVMKeeperWithPrecompiles()
	first.SetNonce(firstCtx, addr, 77)
	first.SetState(firstCtx, addr, slot, value)
	first.SetCode(firstCtx, addr, []byte{0x60, 0x00})
	coins := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 7))
	require.NoError(t, first.BankKeeper().MintCoins(firstCtx, "evm", coins))
	require.NoError(t, first.BankKeeper().SendCoinsFromModuleToAccount(firstCtx, "evm", paxAddr, coins))

	second, secondCtx := MockEVMKeeperWithPrecompiles()
	require.NotSame(t, first, second)
	require.NotSame(t, firstCtx.MultiStore().GetKVStore(first.GetStoreKey()), secondCtx.MultiStore().GetKVStore(second.GetStoreKey()))
	require.NotSame(t, first, &EVMTestApp.EvmKeeper)
	require.Equal(t, int64(8), secondCtx.BlockHeight())
	require.Zero(t, second.GetNonce(secondCtx, addr))
	require.Equal(t, common.Hash{}, second.GetState(secondCtx, addr, slot))
	require.Empty(t, second.GetCode(secondCtx, addr))
	require.Equal(t, sdk.NewInt(10), second.BankKeeper().GetBalance(secondCtx, paxAddr, "uhpx").Amount)

	second.SetNonce(secondCtx, addr, 88)
	second.SetState(secondCtx, addr, slot, common.HexToHash("0x5678"))
	second.SetCode(secondCtx, addr, []byte{0x60, 0x01})
	require.Equal(t, uint64(77), first.GetNonce(firstCtx, addr))
	require.Equal(t, value, first.GetState(firstCtx, addr, slot))
	require.Equal(t, []byte{0x60, 0x00}, first.GetCode(firstCtx, addr))

	for i, fixture := range []struct {
		keeper *evmkeeper.Keeper
		ctx    sdk.Context
		amount int64
	}{{first, firstCtx, 17}, {second, secondCtx, 10}} {
		t.Run(fmt.Sprintf("bank_precompile_%d", i), func(t *testing.T) {
			fixture.keeper.SetAddressMapping(fixture.ctx, paxAddr, common.BytesToAddress(paxAddr))
			contract := fixture.keeper.CustomPrecompiles(fixture.ctx)[common.HexToAddress(bank.BankAddress)]
			require.NotNil(t, contract)
			precompile, ok := contract.(*pcommon.DynamicGasPrecompile)
			require.True(t, ok)
			abi := bank.GetABI()
			input, err := abi.Pack(bank.BalanceMethod, common.BytesToAddress(paxAddr), "uhpx")
			require.NoError(t, err)
			evm := &vm.EVM{StateDB: state.NewDBImpl(fixture.ctx, fixture.keeper, false)}
			output, _, err := precompile.RunAndCalculateGas(evm, addr, addr, input, 1_000_000, nil, nil, true, false)
			require.NoError(t, err)
			decoded, err := abi.Unpack(bank.BalanceMethod, output)
			require.NoError(t, err)
			require.Equal(t, []interface{}{big.NewInt(fixture.amount)}, decoded)
		})
	}
}

func TestRepairM_773_ConcurrentFixtures(t *testing.T) {
	const callers = 3
	type fixture struct {
		keeper *evmkeeper.Keeper
		ctx    sdk.Context
	}
	fixtures := make([]fixture, callers)
	start := make(chan struct{})
	var ready sync.WaitGroup
	ready.Add(callers)
	for i := range fixtures {
		go func(i int) {
			defer ready.Done()
			<-start
			fixtures[i].keeper, fixtures[i].ctx = MockEVMKeeperWithPrecompiles()
		}(i)
	}
	close(start)
	ready.Wait()
	for i := range fixtures {
		for j := 0; j < i; j++ {
			require.NotSame(t, fixtures[i].keeper, fixtures[j].keeper)
			require.NotSame(t, fixtures[i].ctx.MultiStore().GetKVStore(fixtures[i].keeper.GetStoreKey()), fixtures[j].ctx.MultiStore().GetKVStore(fixtures[j].keeper.GetStoreKey()))
			require.NotSame(t, fixtures[i].ctx.EventManager(), fixtures[j].ctx.EventManager())
		}
	}

	addr := common.HexToAddress("0x773")
	slot := common.HexToHash("0x773")
	var written sync.WaitGroup
	written.Add(callers)
	for i, f := range fixtures {
		go func(i int, f fixture) {
			defer written.Done()
			f.keeper.SetNonce(f.ctx, addr, uint64(i+1))
			f.keeper.SetState(f.ctx, addr, slot, common.BigToHash(big.NewInt(int64(i+1))))
			f.keeper.SetCode(f.ctx, addr, []byte{0x60, byte(i)})
			f.ctx.EventManager().EmitEvent(sdk.NewEvent(fmt.Sprintf("fixture_%d", i)))
		}(i, f)
	}
	written.Wait()
	for i, f := range fixtures {
		require.Equal(t, uint64(i+1), f.keeper.GetNonce(f.ctx, addr))
		require.Equal(t, common.BigToHash(big.NewInt(int64(i+1))), f.keeper.GetState(f.ctx, addr, slot))
		require.Equal(t, []byte{0x60, byte(i)}, f.keeper.GetCode(f.ctx, addr))
		for j := range fixtures {
			found := false
			for _, event := range f.ctx.EventManager().Events() {
				found = found || event.Type == fmt.Sprintf("fixture_%d", j)
			}
			require.Equal(t, i == j, found)
		}
	}
	fresh, ctx := MockEVMKeeperWithPrecompiles()
	require.Zero(t, fresh.GetNonce(ctx, addr))
	require.Equal(t, common.Hash{}, fresh.GetState(ctx, addr, slot))
	require.Empty(t, fresh.GetCode(ctx, addr))
}
