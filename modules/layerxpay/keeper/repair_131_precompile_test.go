package keeper_test

import (
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/keeper"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxpay"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestRepair_131_Budget0x101B(t *testing.T) {
	for _, spent := range []int64{0, 20} {
		t.Run(fmt.Sprint(spent), func(t *testing.T) {
			world := keeper.OpenRepair131BudgetWorld(t)
			application := testkeeper.EVMTestApp
			appContext, _ := application.GetContextForDeliverTx(nil).CacheContext()
			stores := map[storetypes.StoreKey]storetypes.CacheWrapper{}
			keys := map[string]storetypes.StoreKey{}
			for _, source := range []sdk.MultiStore{appContext.MultiStore(), world.Context.MultiStore()} {
				for _, key := range source.StoreKeys() {
					var kv storetypes.KVStore
					func() { defer func() { _ = recover() }(); kv = source.GetKVStore(key) }()
					if kv != nil {
						stores[key] = kv
						keys[key.Name()] = key
					}
				}
			}
			composite := cachemulti.NewStore(dbm.NewMemDB(), stores, keys, nil, nil, nil, 0)
			ctx := world.Context.WithMultiStore(composite).WithEVMTxHash(common.HexToHash("0x131101b")).WithEventManager(sdk.NewEventManager())
			p, err := lp.NewServedPrecompile(world.Keeper, &application.EvmKeeper)
			require.NoError(t, err)
			db := state.NewDBImpl(ctx, &application.EvmKeeper, false)
			block, err := application.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
			require.NoError(t, err)
			chain := evmtypes.DefaultChainConfig().EthereumConfig(application.EvmKeeper.ChainID(ctx))
			machine := vm.NewEVM(*block, db, chain, vm.Config{}, map[common.Address]vm.PrecompiledContract{p.Address(): p})
			contract := p.GetABI()
			call := func(caller common.Address, method string, args ...interface{}) ([]byte, error) {
				input, err := contract.Pack(method, args...)
				require.NoError(t, err)
				output, _, err := machine.Call(caller, p.Address(), input, 2000000, uint256.NewInt(0))
				return output, err
			}
			output, err := call(world.Caller, "budgetCreate", world.ID, world.Account, world.Asset, [32]byte{1},
				big.NewInt(100), big.NewInt(0), big.NewInt(100), uint64(60000), uint64(1700000000000), uint64(1700001000000), uint64(7), uint8(1))
			require.NoError(t, err, "%x", output)
			if spent > 0 {
				output, err = call(world.Caller, "budgetSpend", world.ID, world.Recipient, big.NewInt(spent))
				require.NoError(t, err, "%x", output)
			}
			before := world.Keeper.ExportGenesis(db.Ctx())
			logCount := len(db.GetAllLogs())
			output, err = call(world.Stranger, "budgetRevoke", world.ID, uint64(8))
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			reason, err := abi.UnpackRevert(output)
			require.NoError(t, err)
			require.Contains(t, reason, "LXP_UNAUTHORIZED_DEBIT")
			require.Equal(t, before, world.Keeper.ExportGenesis(db.Ctx()))
			require.Len(t, db.GetAllLogs(), logCount)
			output, err = call(world.Caller, "budgetRevoke", world.ID, uint64(7))
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			reason, err = abi.UnpackRevert(output)
			require.NoError(t, err)
			require.Contains(t, reason, "LXP_STALE_REVOCATION")
			require.Equal(t, before, world.Keeper.ExportGenesis(db.Ctx()))
			output, err = call(world.Caller, "budgetRevoke", world.ID, uint64(8))
			require.NoError(t, err, "%x", output)
			logs := db.GetAllLogs()
			require.Len(t, logs, logCount+1)
			log := logs[len(logs)-1]
			require.Equal(t, []common.Hash{contract.Events[lp.BudgetRevoked].ID, common.Hash(world.ID)}, log.Topics)
			expected, err := contract.Events[lp.BudgetRevoked].Inputs.NonIndexed().Pack(big.NewInt(100-spent), uint64(8))
			require.NoError(t, err)
			require.Equal(t, expected, log.Data)
			record, found, err := world.Keeper.GetBudget(db.Ctx(), world.ID)
			require.NoError(t, err)
			require.True(t, found)
			require.True(t, record.Revoked)
			remaining, err := world.Keeper.BudgetRemaining(db.Ctx(), world.ID)
			require.NoError(t, err)
			require.True(t, remaining.IsZero())
			checkBalances := func(ctx sdk.Context) {
				for account, amount := range map[[32]byte]int64{world.Owner: 200 - spent, world.Account: 0, world.Recipient: spent} {
					balance, err := world.Keeper.Host().ReadBalance(ctx, account, world.Asset)
					require.NoError(t, err)
					require.Equal(t, fmt.Sprint(amount), balance.String())
				}
			}
			checkBalances(db.Ctx())
			before = world.Keeper.ExportGenesis(db.Ctx())
			output, err = call(world.Caller, "budgetRevoke", world.ID, uint64(8))
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			reason, err = abi.UnpackRevert(output)
			require.NoError(t, err)
			require.Contains(t, reason, "LXP_BUDGET_REVOKED")
			require.Equal(t, before, world.Keeper.ExportGenesis(db.Ctx()))
			require.Len(t, db.GetAllLogs(), len(logs))
			_, err = db.Finalize()
			require.NoError(t, err)
			composite.Write()
			world.Reopen(t)
			got, found, err := world.Keeper.GetBudget(world.Context, world.ID)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, record, got)
			checkBalances(world.Context)
			remaining, err = world.Keeper.BudgetRemaining(world.Context, world.ID)
			require.NoError(t, err)
			require.True(t, remaining.IsZero())
		})
	}
}
