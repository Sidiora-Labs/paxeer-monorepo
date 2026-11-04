package state_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	vestingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/vesting/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// The fixture uses the application's production keepers and codec, with their
// committed genesis stores copied into a private, reopenable Go LevelDB.
type repairM1246Fixture struct {
	k    *keeper.Keeper
	ctx  sdk.Context
	root sdk.CommitMultiStore
	keys []*sdk.KVStoreKey
	dir  string
}

func newRepairM1246Fixture(t *testing.T) *repairM1246Fixture {
	t.Helper()
	app := testkeeper.EVMTestApp
	source := app.GetContextForDeliverTx(nil)
	f := &repairM1246Fixture{
		k:    &app.EvmKeeper,
		ctx:  source.WithBlockTime(time.Unix(1000, 0)).WithTxIndex(1246).WithChainID("repair-m1246"),
		keys: []*sdk.KVStoreKey{app.GetKey(types.StoreKey), app.GetKey(authtypes.StoreKey), app.GetKey(banktypes.StoreKey)},
		dir:  t.TempDir(),
	}
	f.open(t)
	for _, key := range f.keys {
		iter := source.KVStore(key).Iterator(nil, nil)
		for ; iter.Valid(); iter.Next() {
			f.ctx.KVStore(key).Set(append([]byte(nil), iter.Key()...), append([]byte{}, iter.Value()...))
		}
		require.NoError(t, iter.Close())
	}
	t.Cleanup(func() {
		if f.root != nil {
			require.NoError(t, f.root.Close())
		}
	})
	return f
}

func (f *repairM1246Fixture) open(t *testing.T) {
	t.Helper()
	database, err := dbm.NewGoLevelDB("state", f.dir)
	require.NoError(t, err)
	f.root = store.NewCommitMultiStore(database)
	for _, key := range f.keys {
		f.root.MountStoreWithDB(key, sdk.StoreTypeDB, nil)
	}
	require.NoError(t, f.root.LoadLatestVersion())
	f.ctx = f.ctx.WithMultiStore(f.root).WithEventManager(sdk.NewEventManager()).WithEvmEventManager(sdk.NewEVMEventManager())
}

func (f *repairM1246Fixture) commitAndReopen(t *testing.T) {
	t.Helper()
	id := f.root.Commit(true)
	require.NoError(t, f.root.Close())
	f.root = nil
	f.open(t)
	require.Equal(t, id, f.root.LastCommitID())
	f.ctx = f.ctx.WithBlockHeight(id.Version)
}

func repairM1246Fund(t *testing.T, k *keeper.Keeper, ctx sdk.Context, addr sdk.AccAddress, coins int64, wei int64) {
	t.Helper()
	amount := sdk.NewCoins(sdk.NewInt64Coin(k.GetBaseDenom(ctx), coins+1))
	require.NoError(t, k.BankKeeper().MintCoins(ctx, types.ModuleName, amount))
	require.NoError(t, k.BankKeeper().SendCoinsAndWei(ctx, k.AccountKeeper().GetModuleAddress(types.ModuleName), addr, sdk.NewInt(coins), sdk.NewInt(wei)))
}

func repairM1246StoreImage(ctx sdk.Context, keys []*sdk.KVStoreKey) map[string]map[string]string {
	image := make(map[string]map[string]string)
	for _, key := range keys {
		values := make(map[string]string)
		iter := ctx.KVStore(key).Iterator(nil, nil)
		for ; iter.Valid(); iter.Next() {
			values[string(iter.Key())] = string(iter.Value())
		}
		if err := iter.Close(); err != nil {
			panic(err)
		}
		image[key.Name()] = values
	}
	return image
}

func TestRepairM_1246_LiveChainMintingBoundary(t *testing.T) {
	f := newRepairM1246Fixture(t)
	for chainID := range config.ChainIDMapping {
		t.Run(chainID, func(t *testing.T) {
			ctx := f.ctx.WithChainID(chainID)
			require.True(t, config.IsLiveChainID(ctx))
			require.Error(t, config.ValidateMockBalancesChainID(ctx))
			for _, path := range []string{"balance-read", "debit", "direct-mint", "copied-debit"} {
				t.Run(path, func(t *testing.T) {
					addr := common.HexToAddress("0x12460001")
					pax := sdk.AccAddress(addr[:])
					db := state.NewMockBalancesDBForTest(ctx, f.k)
					before := repairM1246StoreImage(db.Ctx(), f.keys)
					parent := repairM1246StoreImage(ctx, f.keys)
					events := len(db.Ctx().EventManager().Events())
					require.Panics(t, func() {
						switch path {
						case "balance-read":
							db.GetBalance(addr)
						case "debit":
							db.SubBalance(addr, uint256.NewInt(1), tracing.BalanceChangeTransfer)
						case "direct-mint":
							state.TopOffAccountForTest(db, pax, big.NewInt(1))
						case "copied-debit":
							db.Copy().SubBalance(addr, uint256.NewInt(1), tracing.BalanceChangeTransfer)
						}
					})
					require.Equal(t, before, repairM1246StoreImage(db.Ctx(), f.keys))
					require.Equal(t, parent, repairM1246StoreImage(ctx, f.keys))
					require.Len(t, db.Ctx().EventManager().Events(), events)
				})
			}
		})
	}
	// The ordinary build must still read and debit real funds on live chains.
	ctx := f.ctx.WithChainID("hyperpax_125-1")
	addr := common.HexToAddress("0x12460002")
	repairM1246Fund(t, f.k, ctx, sdk.AccAddress(addr[:]), 2, 7)
	db := state.NewDBImpl(ctx, f.k, false)
	require.Equal(t, uint256.NewInt(2_000_000_000_007), db.GetBalance(addr))
	db.SubBalance(addr, uint256.NewInt(1), tracing.BalanceChangeTransfer)
	require.NoError(t, db.Error())
	require.Equal(t, uint256.NewInt(2_000_000_000_006), db.GetBalance(addr))
}

func TestRepairM_1246_DisposableChainTopOff(t *testing.T) {
	f := newRepairM1246Fixture(t)
	require.NoError(t, config.ValidateMockBalancesChainID(f.ctx))
	addr := common.HexToAddress("0x12460003")
	db := state.NewMockBalancesDBForTest(f.ctx, f.k)
	require.Equal(t, state.TopOffAmount, db.GetBalance(addr).ToBig())
	amount := new(big.Int).Add(state.TopOffAmount, big.NewInt(123))
	debit, overflow := uint256.FromBig(amount)
	require.False(t, overflow)
	db.SubBalance(addr, debit, tracing.BalanceChangeTransfer)
	require.NoError(t, db.Error())
	require.Equal(t, state.TopOffAmount, db.GetBalance(addr).ToBig())
	require.Zero(t, f.k.GetBalance(f.ctx, sdk.AccAddress(addr[:])).Sign())
	_, err := db.Finalize()
	require.NoError(t, err)
	db.Cleanup()
	f.commitAndReopen(t)
	require.Equal(t, state.TopOffAmount, state.NewDBImpl(f.ctx, f.k, false).GetBalance(addr).ToBig())
}

func TestRepairM_1246_AssociatedDestruction(t *testing.T) {
	for _, historical := range []bool{false, true} {
		name := "delivery"
		if historical {
			name = "historical"
		}
		t.Run(name, func(t *testing.T) {
			f := newRepairM1246Fixture(t)
			pax, addr := testkeeper.MockAddressPair()
			cast := sdk.AccAddress(addr[:])
			require.False(t, pax.Equals(cast))
			f.k.SetAddressMapping(f.ctx, pax, addr)
			repairM1246Fund(t, f.k, f.ctx, pax, 10, 37)
			f.k.SetCode(f.ctx, addr, []byte{0x60, 0x00})
			key, value := common.HexToHash("0x1246"), common.HexToHash("0x29")
			f.k.SetState(f.ctx, addr, key, value)
			f.commitAndReopen(t)
			ctx := f.ctx.WithIsTracing(historical)
			db := state.NewDBImpl(ctx, f.k, false)
			db.CreateContract(addr)
			initial := uint256.NewInt(10_000_000_000_037)
			require.Equal(t, initial, db.GetBalance(addr))
			require.Equal(t, initial.ToBig(), f.k.GetBalance(db.Ctx(), pax))
			require.Zero(t, f.k.GetBalance(db.Ctx(), cast).Sign())
			amount, destroyed := db.SelfDestruct6780(addr)
			require.True(t, destroyed)
			require.Equal(t, *initial, amount)
			require.NoError(t, db.Error())
			require.True(t, db.HasSelfDestructed(addr))
			require.Zero(t, f.k.GetBalance(db.Ctx(), pax).Sign())
			require.Zero(t, f.k.GetBalance(db.Ctx(), cast).Sign())
			_, associated := f.k.GetPaxAddress(db.Ctx(), addr)
			require.False(t, associated)
			require.Equal(t, initial.ToBig(), f.k.GetBalance(ctx, pax), "parent is unchanged until finalize")
			require.Equal(t, value, db.GetState(addr, key))
			_, err := db.Finalize()
			require.NoError(t, err)
			db.Cleanup()
			f.commitAndReopen(t)
			reloaded := state.NewDBImpl(f.ctx.WithIsTracing(historical), f.k, false)
			require.Zero(t, f.k.GetBalance(reloaded.Ctx(), pax).Sign())
			require.Zero(t, f.k.GetBalance(reloaded.Ctx(), cast).Sign())
			require.True(t, reloaded.GetBalance(addr).IsZero())
			_, associated = f.k.GetPaxAddress(reloaded.Ctx(), addr)
			require.False(t, associated)
			require.Empty(t, reloaded.GetCode(addr))
			require.Equal(t, common.Hash{}, reloaded.GetState(addr, key))
		})
	}
}

func TestRepairM_1246_AssociatedDestructionAssertions(t *testing.T) {
	TestSelfDestructAssociated(t)
}

func TestRepairM_1246_ResidualSettlementAtomic(t *testing.T) {
	for _, failure := range []string{"locked-debit", "collector-credit"} {
		t.Run(failure, func(t *testing.T) {
			f := newRepairM1246Fixture(t)
			pax, addr := testkeeper.MockAddressPair()
			f.k.SetAddressMapping(f.ctx, pax, addr)
			f.k.SetCode(f.ctx, addr, []byte{0x60, 0x01})
			key, value := common.HexToHash("0x1246"), common.HexToHash("0x31")
			f.k.SetState(f.ctx, addr, key, value)
			f.commitAndReopen(t)
			db := state.NewDBImpl(f.ctx, f.k, false)
			destroyedAmount := db.SelfDestruct(addr)
			require.True(t, destroyedAmount.IsZero())
			require.NoError(t, db.Error())
			require.True(t, db.HasSelfDestructed(addr))
			cast := sdk.AccAddress(addr[:])
			repairM1246Fund(t, f.k, db.Ctx(), cast, 3, 19)
			wantErr := sdkerrors.ErrInvalidRecipient
			if failure == "locked-debit" {
				base := f.k.AccountKeeper().GetAccount(db.Ctx(), cast)
				if base == nil {
					base = f.k.AccountKeeper().NewAccountWithAddress(db.Ctx(), cast)
				}
				locked := vestingtypes.NewDelayedVestingAccount(
					authtypes.NewBaseAccount(cast, nil, base.GetAccountNumber(), base.GetSequence()),
					sdk.NewCoins(sdk.NewInt64Coin(f.k.GetBaseDenom(db.Ctx()), 3)),
					f.ctx.BlockTime().Add(time.Hour).Unix(), cast,
				)
				f.k.AccountKeeper().SetAccount(db.Ctx(), locked)
				require.True(t, f.k.BankKeeper().SpendableCoins(db.Ctx(), cast).AmountOf(f.k.GetBaseDenom(db.Ctx())).IsZero())
				// Fee-token buying power makes the real balance view include
				// the locked network amount; the bank debit must still reject it.
				feeCoins := sdk.NewCoins(sdk.NewInt64Coin("usid", 3))
				require.NoError(t, f.k.BankKeeper().MintCoins(db.Ctx(), types.ModuleName, feeCoins))
				require.NoError(t, f.k.BankKeeper().SendCoinsFromModuleToAccount(db.Ctx(), types.ModuleName, cast, feeCoins))
				db.SetFeeTokenCharge(&state.FeeTokenCharge{Payer: addr, Denom: "usid", Rate: sdk.NewDec(1_000_000)}, false)
				wantErr = sdkerrors.ErrInsufficientFunds
			} else {
				// The real EVM recipient checker rejects a cast address once it
				// is mapped elsewhere, including the transaction's collector.
				collector := state.GetCoinbaseAddress(db.TxIndex())
				other, _ := testkeeper.MockAddressPair()
				f.k.SetAddressMapping(db.Ctx(), other, common.BytesToAddress(collector))
				require.False(t, f.k.BankKeeper().CanSendTo(db.Ctx(), collector))
			}
			require.Equal(t, uint256.NewInt(3_000_000_000_019), db.GetBalance(addr))
			require.NoError(t, db.Error())
			db.Snapshot()
			db.SetTransientState(addr, key, value)
			before := repairM1246StoreImage(db.Ctx(), f.keys)
			parent := repairM1246StoreImage(f.ctx, f.keys)
			events := len(db.Ctx().EventManager().Events())
			parentEvents := len(f.ctx.EventManager().Events())
			_, err := db.Finalize()
			require.ErrorIs(t, err, wantErr)
			require.ErrorIs(t, db.Error(), wantErr)
			require.Equal(t, before, repairM1246StoreImage(db.Ctx(), f.keys), "failed settlement rolls back both balances")
			require.Equal(t, parent, repairM1246StoreImage(f.ctx, f.keys), "failed finalization never publishes execution")
			require.Len(t, db.Ctx().EventManager().Events(), events)
			require.Len(t, f.ctx.EventManager().Events(), parentEvents)
			require.Equal(t, value, db.GetState(addr, key), "account state is not cleared on error")
			require.NotEmpty(t, db.GetCode(addr))
			require.Equal(t, value, db.GetTransientState(addr, key))
			require.Equal(t, sdk.NewInt(3), f.k.BankKeeper().GetBalance(db.Ctx(), cast, f.k.GetBaseDenom(db.Ctx())).Amount)
			require.Equal(t, sdk.NewInt(19), f.k.BankKeeper().GetWeiBalance(db.Ctx(), cast))
			require.Zero(t, f.k.GetBalance(db.Ctx(), state.GetCoinbaseAddress(db.TxIndex())).Sign())
			db.Cleanup()
			f.commitAndReopen(t)
			require.Equal(t, parent, repairM1246StoreImage(f.ctx, f.keys))
			reloaded := state.NewDBImpl(f.ctx, f.k, false)
			mapped, exists := f.k.GetPaxAddress(reloaded.Ctx(), addr)
			require.True(t, exists)
			require.True(t, pax.Equals(mapped))
			require.Equal(t, value, reloaded.GetState(addr, key))
		})
	}
}
