package v575_test

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	transferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/addr"
	legacy "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/addr/legacy/v575"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common/legacy/v575"
	putils "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// Only the three constructor dependencies are provided. Every accessor used by
// the precompiles returns a production keeper; unused interfaces stay unbound.
type repair1264Keepers struct {
	putils.Keepers
	EvmKeeper     *evmkeeper.Keeper
	BankKeeper    bankkeeper.BaseKeeper
	AccountKeeper authkeeper.AccountKeeper
}

func (k repair1264Keepers) EVMK() putils.EVMKeeper         { return k.EvmKeeper }
func (k repair1264Keepers) BankK() putils.BankKeeper       { return k.BankKeeper }
func (k repair1264Keepers) AccountK() putils.AccountKeeper { return k.AccountKeeper }

func repair1264NewKeepers(t *testing.T) (repair1264Keepers, sdk.Context) {
	t.Helper()
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	bankKey := sdk.NewKVStoreKey(banktypes.StoreKey)
	evmKey := sdk.NewKVStoreKey(evmtypes.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	upgradeKey := sdk.NewKVStoreKey(upgradetypes.StoreKey)
	paramsTransient := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	evmTransient := sdk.NewTransientStoreKey(evmtypes.TransientStoreKey)
	for _, key := range []*sdk.KVStoreKey{authKey, bankKey, evmKey, paramsKey, upgradeKey} {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	cms.MountStoreWithDB(paramsTransient, sdk.StoreTypeTransient, nil)
	cms.MountStoreWithDB(evmTransient, sdk.StoreTypeTransient, nil)
	require.NoError(t, cms.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, cms.Close()) })
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	cryptocodec.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	auth := authkeeper.NewAccountKeeper(cdc, authKey,
		paramtypes.NewSubspace(cdc, amino, paramsKey, paramsTransient, authtypes.ModuleName),
		authtypes.ProtoBaseAccount, map[string][]string{"evm": {authtypes.Minter}, authtypes.FeeCollectorName: {}})
	bank := bankkeeper.NewBaseKeeper(cdc, bankKey, auth,
		paramtypes.NewSubspace(cdc, amino, paramsKey, paramsTransient, banktypes.ModuleName), map[string]bool{})
	upgrades := upgradekeeper.NewKeeper(map[int64]bool{}, upgradeKey, cdc, t.TempDir(), nil)
	evm := evmkeeper.NewKeeper(evmKey, evmTransient,
		paramtypes.NewSubspace(cdc, amino, paramsKey, paramsTransient, evmtypes.ModuleName),
		nil, bank, &auth, nil, transferkeeper.Keeper{}, nil, nil, &upgrades)
	ctx := sdk.NewContext(cms, tmtypes.Header{Height: 2, Time: time.Unix(1700000000, 0)}, false)
	auth.SetParams(ctx, authtypes.DefaultParams())
	bank.SetParams(ctx, banktypes.DefaultParams())
	evm.SetParams(ctx, evmtypes.DefaultParams())
	return repair1264Keepers{EvmKeeper: evm, BankKeeper: bank, AccountKeeper: auth}, ctx
}

// This signed, non-payable association is shared by replay and current calls.
// The expected state transition is 100 uhpx, 23 replaytoken and 19 wei moving
// from the EVM-cast account to its recovered Pax account, with both mappings set.
type repair1264Fixture struct {
	keepers repair1264Keepers
	db      *state.DBImpl
	evm     *vm.EVM
	target  common.Address
	pax     sdk.AccAddress
	cast    sdk.AccAddress
	pub     *secp256k1.PubKey
	input   []byte
	coins   sdk.Coins
}

func repair1264NewFixture(t *testing.T, historical bool) repair1264Fixture {
	t.Helper()
	a, ctx := repair1264NewKeepers(t)
	ctx = ctx.WithMultiStore(ctx.MultiStore().CacheMultiStore()).WithEventManager(sdk.NewEventManager())
	if historical {
		ctx = ctx.WithIsTracing(true).WithClosestUpgradeName("v5.7.5")
	}
	key, err := crypto.HexToECDSA("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	require.NoError(t, err)
	pub := &secp256k1.PubKey{Key: crypto.CompressPubkey(&key.PublicKey)}
	target := crypto.PubkeyToAddress(key.PublicKey)
	pax := sdk.AccAddress(pub.Address())
	cast := sdk.AccAddress(target.Bytes())
	require.NotEqual(t, pax, cast)
	message := "\x19Ethereum Signed Message:\n21repair historical 575"
	sig, err := crypto.Sign(crypto.Keccak256([]byte(message)), key)
	require.NoError(t, err)
	p, err := legacy.NewPrecompile(a)
	require.NoError(t, err)
	input, err := p.ABI.Pack(legacy.Associate, fmt.Sprintf("0x%x", sig[64:]),
		fmt.Sprintf("0x%x", sig[:32]), fmt.Sprintf("0x%x", sig[32:64]), message)
	require.NoError(t, err)
	_, found := a.EvmKeeper.GetEVMAddress(ctx, pax)
	require.False(t, found)
	_, found = a.EvmKeeper.GetPaxAddress(ctx, target)
	require.False(t, found)
	require.Nil(t, a.AccountKeeper.GetAccount(ctx, pax))
	a.AccountKeeper.SetAccount(ctx, authtypes.NewBaseAccount(cast, nil, 42, 7))
	coins := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 100), sdk.NewInt64Coin("replaytoken", 23))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, "evm", coins))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "evm", cast, coins))
	require.NoError(t, a.BankKeeper.AddWei(ctx, cast, sdk.NewInt(19)))
	db := state.NewDBImpl(ctx, a.EvmKeeper, true)
	return repair1264Fixture{keepers: a, db: db, evm: &vm.EVM{StateDB: db}, target: target,
		pax: pax, cast: cast, pub: pub, input: input, coins: coins}
}

func repair1264Snapshot(t *testing.T, ctx sdk.Context) map[string]map[string][]byte {
	t.Helper()
	out := make(map[string]map[string][]byte)
	for _, key := range ctx.MultiStore().StoreKeys() {
		values := make(map[string][]byte)
		it := ctx.KVStore(key).Iterator(nil, nil)
		for ; it.Valid(); it.Next() {
			values[string(it.Key())] = bytes.Clone(it.Value())
		}
		require.NoError(t, it.Close())
		out[key.Name()] = values
	}
	return out
}

func repair1264AssertAssociated(t *testing.T, f repair1264Fixture, ret []byte, method abi.Method) {
	t.Helper()
	a := f.keepers
	ctx := f.db.Ctx()
	expected, err := method.Outputs.Pack(f.pax.String(), f.target)
	require.NoError(t, err)
	require.Equal(t, expected, ret)
	pax, found := a.EvmKeeper.GetPaxAddress(ctx, f.target)
	require.True(t, found)
	require.Equal(t, f.pax, pax)
	evm, found := a.EvmKeeper.GetEVMAddress(ctx, f.pax)
	require.True(t, found)
	require.Equal(t, f.target, evm)
	require.Equal(t, f.coins, a.BankKeeper.GetAllBalances(ctx, f.pax))
	require.Equal(t, sdk.NewInt(19), a.BankKeeper.GetWeiBalance(ctx, f.pax))
	require.True(t, a.BankKeeper.GetAllBalances(ctx, f.cast).IsZero())
	require.True(t, a.BankKeeper.GetWeiBalance(ctx, f.cast).IsZero())
	require.Nil(t, a.AccountKeeper.GetAccount(ctx, f.cast))
	account := a.AccountKeeper.GetAccount(ctx, f.pax)
	require.NotNil(t, account)
	require.NotNil(t, account.GetPubKey())
	require.Equal(t, f.pub.Bytes(), account.GetPubKey().Bytes())
}

func TestRepairM_1264_HistoricalStaticCallStateDiff(t *testing.T) {
	var writable map[string]map[string][]byte
	var writableEvents sdk.Events
	for _, readOnly := range []bool{false, true} {
		f := repair1264NewFixture(t, true)
		p, err := legacy.NewPrecompile(f.keepers)
		require.NoError(t, err)
		before := repair1264Snapshot(t, f.db.Ctx())
		ret, err := p.Run(f.evm, f.target, f.target, f.input, big.NewInt(0), readOnly, false, nil)
		require.NoError(t, err)
		repair1264AssertAssociated(t, f, ret, p.ABI.Methods[legacy.Associate])
		after := repair1264Snapshot(t, f.db.Ctx())
		require.NotEqual(t, before, after)
		require.NotEqual(t, before["evm"], after["evm"])
		require.NotEqual(t, before["bank"], after["bank"])
		require.NotEqual(t, before["acc"], after["acc"])
		if !readOnly {
			writable = after
			writableEvents = f.db.Ctx().EventManager().Events()
		} else {
			// Replay must retain the complete historical CALL state diff under STATICCALL.
			require.Equal(t, writable, after)
			require.Equal(t, writableEvents, f.db.Ctx().EventManager().Events())
		}
	}
}

func TestRepairM_1264_CurrentAssociationRejectsStaticCall(t *testing.T) {
	f := repair1264NewFixture(t, false)
	a := f.keepers
	p, err := addr.NewPrecompile(a)
	require.NoError(t, err)
	before := repair1264Snapshot(t, f.db.Ctx())
	events := f.db.Ctx().EventManager().Events()
	_, _, err = p.RunAndCalculateGas(f.evm, f.target, f.target, f.input, 1_000_000,
		big.NewInt(0), nil, true, false)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.ErrorContains(t, f.db.GetPrecompileError(), "cannot call associate precompile from staticcall")
	// Inspect before any rollback: refusal itself must leave every store intact.
	require.Equal(t, before, repair1264Snapshot(t, f.db.Ctx()))
	require.Equal(t, events, f.db.Ctx().EventManager().Events())
	_, found := a.EvmKeeper.GetPaxAddress(f.db.Ctx(), f.target)
	require.False(t, found)
	_, found = a.EvmKeeper.GetEVMAddress(f.db.Ctx(), f.pax)
	require.False(t, found)
	require.Equal(t, f.coins, a.BankKeeper.GetAllBalances(f.db.Ctx(), f.cast))
	require.Equal(t, sdk.NewInt(19), a.BankKeeper.GetWeiBalance(f.db.Ctx(), f.cast))
	require.True(t, a.BankKeeper.GetAllBalances(f.db.Ctx(), f.pax).IsZero())
	require.True(t, a.BankKeeper.GetWeiBalance(f.db.Ctx(), f.pax).IsZero())
	// The identical payload must succeed as a normal call, excluding invalid input
	// or missing funds as the reason the static call was refused.
	f.db.SetPrecompileError(nil)
	ret, _, err := p.RunAndCalculateGas(f.evm, f.target, f.target, f.input, 1_000_000,
		big.NewInt(0), nil, false, false)
	require.NoError(t, err)
	repair1264AssertAssociated(t, f, ret, p.ABI.Methods[addr.Associate])
}

func TestRepairM_1264_HistoricalConstructorVersionIsolation(t *testing.T) {
	keepers, _ := repair1264NewKeepers(t)
	versions := addr.GetVersioned("v99.0.0", keepers)
	for version, precompile := range versions {
		p, historical := precompile.(*pcommon.Precompile)
		if version == "v5.7.5" {
			require.True(t, historical)
			require.IsType(t, &legacy.PrecompileExecutor{}, p.GetExecutor())
		} else if historical {
			require.NotSame(t, versions["v5.7.5"], p)
			t.Fatalf("v575 constructor reused for %s", version)
		}
	}
	require.Contains(t, versions, "v5.7.5")
	require.Contains(t, versions, "v99.0.0")
}
