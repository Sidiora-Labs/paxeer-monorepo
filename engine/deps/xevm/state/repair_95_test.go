package state_test

import (
	"math/big"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xbank/keeper"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/types"
	transferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	gethstate "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
)

type repair95Keeper struct {
	k      *evmkeeper.Keeper
	ctx    sdk.Context
	root   *rootmulti.Store
	closed bool
}

func openRepair95Keeper(t *testing.T, dir string) *repair95Keeper {
	t.Helper()
	backing, err := tmdb.NewDB("repair95", tmdb.GoLevelDBBackend, dir)
	require.NoError(t, err)
	root := rootmulti.NewStore(backing)
	evmKey := sdk.NewKVStoreKey(evmtypes.StoreKey)
	bankKey := sdk.NewKVStoreKey("bank")
	authKey := sdk.NewKVStoreKey("auth")
	paramsKey := sdk.NewKVStoreKey("params")
	transientKey := sdk.NewTransientStoreKey("transient_params")
	keys := map[string]storetypes.StoreKey{}
	for _, key := range []storetypes.StoreKey{evmKey, bankKey, authKey, paramsKey} {
		root.MountStoreWithDB(key, sdk.StoreTypeIAVL, backing)
		keys[key.Name()] = key
	}
	root.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, nil)
	keys[transientKey.Name()] = transientKey
	require.NoError(t, root.LoadLatestVersion())
	stores := map[storetypes.StoreKey]storetypes.CacheWrapper{}
	for _, key := range keys {
		stores[key] = root.GetCommitKVStore(key)
	}
	ms := cachemulti.NewFromKVStore(dbadapter.Store{DB: backing}, stores, nil, keys,
		[]storetypes.StoreKey{evmKey, bankKey}, nil, nil, 0)
	ctx := sdk.NewContext(ms, tmproto.Header{Height: 1, Time: time.Unix(1, 0)}, false)
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	subspace := func(name string) paramtypes.Subspace {
		return paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), paramsKey, transientKey, name)
	}
	ak := authkeeper.NewAccountKeeper(cdc, authKey, subspace("auth"), authtypes.ProtoBaseAccount,
		map[string][]string{authtypes.FeeCollectorName: nil, evmtypes.ModuleName: {authtypes.Minter}})
	bk := bankkeeper.NewBaseKeeper(cdc, bankKey, &ak, subspace("bank"), nil)
	k := evmkeeper.NewKeeper(evmKey, transientKey, subspace("evm"), nil, bk, &ak, nil, transferkeeper.Keeper{}, nil, nil, nil)
	bk.RegisterRecipientChecker(k.CanAddressReceive)
	k.SetAddressMapping(ctx, ak.GetModuleAddress(authtypes.FeeCollectorName), evmkeeper.GetCoinbaseAddress())
	h := &repair95Keeper{k: k, ctx: ctx, root: root}
	t.Cleanup(func() { h.close(t) })
	return h
}

func (h *repair95Keeper) close(t *testing.T) {
	t.Helper()
	if !h.closed {
		require.NoError(t, h.root.Close())
		h.closed = true
	}
}

func (h *repair95Keeper) commit(t *testing.T, db *state.DBImpl) {
	t.Helper()
	_, err := db.Finalize()
	require.NoError(t, err)
	h.ctx.MultiStore().(sdk.CacheMultiStore).Write()
	h.ctx.GigaMultiStore().WriteGiga()
	require.Positive(t, h.root.Commit(true).Version)
	db.Cleanup()
}

func newRepair95EVM(db vm.StateDB) *vm.EVM {
	return vm.NewEVM(vm.BlockContext{
		CanTransfer: core.CanTransfer, Transfer: core.Transfer,
		BlockNumber: big.NewInt(1), Time: 1, GasLimit: 1_000_000,
		Difficulty: new(big.Int), BaseFee: new(big.Int),
	}, db, params.AllEthashProtocolChanges, vm.Config{}, nil)
}

func repair95CodeHash(t *testing.T, db *state.DBImpl, target common.Address) common.Hash {
	t.Helper()
	probe := common.HexToAddress("0x9501")
	code := append([]byte{byte(vm.PUSH20)}, target.Bytes()...)
	code = append(code, byte(vm.EXTCODEHASH), byte(vm.PUSH1), 0, byte(vm.MSTORE),
		byte(vm.PUSH1), 32, byte(vm.PUSH1), 0, byte(vm.RETURN))
	db.SetCode(probe, code)
	result, _, err := newRepair95EVM(db).Call(common.HexToAddress("0x9502"), probe, nil, 100_000, new(uint256.Int))
	require.NoError(t, err)
	require.NoError(t, db.Err())
	require.Len(t, result, 32)
	return common.BytesToHash(result)
}

func repair95CallGas(t *testing.T, db *state.DBImpl, target common.Address) uint64 {
	t.Helper()
	probe := common.HexToAddress("0x9503")
	// CALL with value 1 and no input/output, returning its success word.
	code := []byte{byte(vm.PUSH1), 0, byte(vm.PUSH1), 0, byte(vm.PUSH1), 0, byte(vm.PUSH1), 0, byte(vm.PUSH1), 1, byte(vm.PUSH20)}
	code = append(code, target.Bytes()...)
	code = append(code, byte(vm.PUSH2), 0xff, 0xff, byte(vm.CALL), byte(vm.PUSH1), 0, byte(vm.MSTORE),
		byte(vm.PUSH1), 32, byte(vm.PUSH1), 0, byte(vm.RETURN))
	db.SetCode(probe, code)
	db.AddBalance(probe, uint256.NewInt(10), tracing.BalanceChangeUnspecified)
	db.AddAddressToAccessList(target)
	const gas = uint64(100_000)
	result, left, err := newRepair95EVM(db).Call(common.HexToAddress("0x9502"), probe, nil, gas, new(uint256.Int))
	require.NoError(t, err)
	require.NoError(t, db.Err())
	require.Equal(t, common.BigToHash(big.NewInt(1)).Bytes(), result)
	return gas - left
}

func TestRepair_95_010_EmptyCodeAndGethReopen(t *testing.T) {
	dir := t.TempDir()
	h := openRepair95Keeper(t, dir)
	db := state.NewDBImpl(h.ctx, h.k, false)
	nilCode := common.HexToAddress("0x9510")
	cleared := common.HexToAddress("0x9511")
	absent := common.HexToAddress("0x9512")
	withBalance := common.HexToAddress("0x9513")
	withNonce := common.HexToAddress("0x9514")
	withCode := common.HexToAddress("0x9515")
	h.k.SetCode(db.Ctx(), nilCode, nil)
	db.SetCode(cleared, []byte{byte(vm.STOP)})
	require.False(t, db.Empty(cleared))
	require.Equal(t, []byte{byte(vm.STOP)}, db.SetCode(cleared, nil))
	h.k.SetCode(db.Ctx(), withBalance, []byte{})
	db.AddBalance(withBalance, uint256.NewInt(1), tracing.BalanceChangeUnspecified)
	h.k.SetCode(db.Ctx(), withNonce, nil)
	db.SetNonce(withNonce, 1, tracing.NonceChangeUnspecified)
	db.SetCode(withCode, []byte{byte(vm.STOP)})
	creator := common.HexToAddress("0x9516")
	code, deployed, _, err := newRepair95EVM(db).Create(creator,
		[]byte{byte(vm.PUSH1), 0, byte(vm.PUSH1), 0, byte(vm.RETURN)}, 100_000, new(uint256.Int))
	require.NoError(t, err)
	require.Empty(t, code)
	require.Equal(t, crypto.CreateAddress(creator, 0), deployed)
	require.Equal(t, uint64(1), db.GetNonce(deployed))
	require.False(t, db.Empty(deployed), "a zero-byte deployment still has nonce 1")
	require.Equal(t, gethtypes.EmptyCodeHash, db.GetCodeHash(deployed))
	h.commit(t, db)
	h.close(t)
	h = openRepair95Keeper(t, dir)
	db = state.NewDBImpl(h.ctx, h.k, false)
	for _, target := range []common.Address{nilCode, cleared, absent} {
		require.True(t, db.Empty(target), target.Hex())
		require.Zero(t, h.k.GetCodeSize(db.Ctx(), target))
		require.Empty(t, h.k.GetCode(db.Ctx(), target))
		if target != absent {
			require.Equal(t, gethtypes.EmptyCodeHash, h.k.GetCodeHash(db.Ctx(), target))
			require.True(t, db.Exist(target))
		}
		require.Equal(t, common.Hash{}, repair95CodeHash(t, db, target))
	}
	for _, target := range []common.Address{withBalance, withNonce, deployed} {
		require.False(t, db.Empty(target), target.Hex())
		require.Equal(t, gethtypes.EmptyCodeHash, repair95CodeHash(t, db, target))
	}
	require.False(t, db.Empty(withCode))
	require.Equal(t, crypto.Keccak256Hash([]byte{byte(vm.STOP)}), repair95CodeHash(t, db, withCode))
	for _, target := range []common.Address{nilCode, cleared, absent, deployed, withCode} {
		snapshot := db.Snapshot()
		used := repair95CallGas(t, db, target)
		// Seven pushes, warm CALL, value transfer, stipend refund, and return/memory.
		want := uint64(7*3 + params.WarmStorageReadCostEIP2929 + params.CallValueTransferGas - params.CallStipend + 15)
		if target == nilCode || target == cleared || target == absent {
			want += params.CallNewAccountGas
		}
		// The nonempty STOP contract executes a zero-cost opcode.
		require.Equal(t, want, used, target.Hex())
		require.Equal(t, big.NewInt(1), h.k.GetBalance(db.Ctx(), h.k.GetPaxAddressOrDefault(db.Ctx(), target)))
		db.RevertToSnapshot(snapshot)
	}
	require.True(t, db.Empty(nilCode))
	require.True(t, db.Empty(cleared))
}

func TestRepair_95_011_PreviousBalanceHooksAndRollback(t *testing.T) {
	for _, hooked := range []bool{false, true} {
		name := "StateDB"
		if hooked {
			name = "HookedStateDB"
		}
		t.Run(name, func(t *testing.T) {
			h := openRepair95Keeper(t, t.TempDir())
			addr := common.HexToAddress("0x9520")
			paxAddr := sdk.AccAddress(addr.Bytes())
			h.k.SetAddressMapping(h.ctx, paxAddr, addr)
			require.NoError(t, h.k.BankKeeper().AddCoins(h.ctx, paxAddr, sdk.NewCoins(sdk.NewInt64Coin(h.k.GetBaseDenom(h.ctx), 2)), true))
			require.NoError(t, h.k.BankKeeper().AddWei(h.ctx, paxAddr, sdk.NewInt(25)))
			db := state.NewDBImpl(h.ctx, h.k, false)
			type change struct {
				old, next *big.Int
				reason    tracing.BalanceChangeReason
			}
			var changes []change
			hooks := &tracing.Hooks{OnBalanceChange: func(got common.Address, old, next *big.Int, reason tracing.BalanceChangeReason) {
				require.Equal(t, addr, got)
				changes = append(changes, change{new(big.Int).Set(old), new(big.Int).Set(next), reason})
			}}
			db.SetLogger(hooks)
			var consumer vm.StateDB = db
			if hooked {
				consumer = gethstate.NewHookedState(db, hooks)
			}
			const initial = uint64(2_000_000_000_025)
			const afterAdd = uint64(4_000_000_000_015)
			const afterSub = uint64(2_999_999_999_975)
			assertBalance := func(want uint64) {
				require.NoError(t, db.Err())
				require.Equal(t, uint256.NewInt(want), consumer.GetBalance(addr))
				require.Equal(t, new(big.Int).SetUint64(want), h.k.GetBalance(db.Ctx(), paxAddr))
			}
			assertBalance(initial)
			snapshot := consumer.Snapshot()
			eventCount := len(db.Ctx().EventManager().Events())
			prior := consumer.AddBalance(addr, uint256.NewInt(1_999_999_999_990), tracing.BalanceIncreaseRewardTransactionFee)
			require.Equal(t, *uint256.NewInt(initial), prior)
			assertBalance(afterAdd)
			require.Equal(t, *uint256.NewInt(afterAdd), consumer.AddBalance(addr, new(uint256.Int), tracing.BalanceChangeUnspecified))
			require.Equal(t, *uint256.NewInt(afterAdd), consumer.SubBalance(addr, new(uint256.Int), tracing.BalanceChangeUnspecified))
			require.Len(t, changes, 1)
			require.Equal(t, *uint256.NewInt(afterAdd), consumer.SubBalance(addr, uint256.NewInt(1_000_000_000_040), tracing.BalanceDecreaseGasBuy))
			assertBalance(afterSub)
			require.Equal(t, *uint256.NewInt(afterSub), consumer.AddBalance(addr, new(uint256.Int), tracing.BalanceChangeUnspecified))
			require.Equal(t, *uint256.NewInt(afterSub), consumer.SubBalance(addr, new(uint256.Int), tracing.BalanceChangeUnspecified))
			require.Equal(t, []change{
				{new(big.Int).SetUint64(initial), new(big.Int).SetUint64(afterAdd), tracing.BalanceIncreaseRewardTransactionFee},
				{new(big.Int).SetUint64(afterAdd), new(big.Int).SetUint64(afterSub), tracing.BalanceDecreaseGasBuy},
			}, changes)
			consumer.RevertToSnapshot(snapshot)
			assertBalance(initial)
			require.Len(t, db.Ctx().EventManager().Events(), eventCount)
			require.Equal(t, *uint256.NewInt(initial), prior, "returned balance is a value, not live state")
			require.Equal(t, *uint256.NewInt(initial), consumer.SubBalance(addr, new(uint256.Int), tracing.BalanceChangeUnspecified))
			require.Equal(t, *uint256.NewInt(initial), consumer.AddBalance(addr, new(uint256.Int), tracing.BalanceChangeUnspecified))
			require.Len(t, changes, 2)
			surplus, err := db.Finalize()
			require.NoError(t, err)
			require.True(t, surplus.IsZero())
			require.Equal(t, new(big.Int).SetUint64(initial), h.k.GetBalance(h.ctx, paxAddr))
		})
	}
}
