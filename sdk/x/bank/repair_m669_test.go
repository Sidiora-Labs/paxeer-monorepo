package bank_test

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"strings"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	vesting "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/vesting/types"
	bank "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

const repairM669Module = "repair-m669"

type repairM669Fixture struct {
	bank     keeper.BaseKeeper
	auth     authkeeper.AccountKeeper
	ctx      sdk.Context
	store    *rootmulti.Store
	cache    storetypes.CacheMultiStore
	cdc      *codec.ProtoCodec
	deferred sdk.StoreKey
	blocked  map[string]bool
	closed   bool
}

func repairM669Open(t *testing.T, dir string) *repairM669Fixture {
	t.Helper()
	if _, exists := sdk.GetDenomUnit("uhpx"); !exists {
		require.NoError(t, sdk.RegisterDenom("uhpx", sdk.OneDec()))
	}
	require.Equal(t, "uhpx", sdk.MustGetBaseDenom())
	sc := config.DefaultStateCommitConfig()
	sc.MemIAVLConfig.AsyncCommitBuffer = 0
	ms := rootmulti.NewStore(dir, sc, config.StateStoreConfig{Enable: false}, nil)
	bankKey := sdk.NewKVStoreKey(types.StoreKey)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	deferredKey := sdk.NewMemoryStoreKeys(types.DeferredCacheStoreKey)[types.DeferredCacheStoreKey]
	for _, key := range []*sdk.KVStoreKey{bankKey, authKey, paramsKey} {
		ms.MountStoreWithDB(key, sdk.StoreTypeIAVL, nil)
	}
	ms.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, nil)
	ms.MountStoreWithDB(deferredKey, sdk.StoreTypeMemory, nil)
	require.NoError(t, ms.LoadLatestVersion())
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	vesting.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	ak := authkeeper.NewAccountKeeper(cdc, authKey,
		paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, authtypes.ModuleName),
		authtypes.ProtoBaseAccount,
		map[string][]string{repairM669Module: {authtypes.Minter, authtypes.Burner}})
	blocked := map[string]bool{}
	bk := keeper.NewBaseKeeperWithDeferredCache(cdc, bankKey, ak,
		paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, types.ModuleName), blocked, deferredKey)
	cache := ms.CacheMultiStore()
	f := &repairM669Fixture{bank: bk, auth: ak, store: ms, cache: cache, cdc: cdc,
		deferred: deferredKey, blocked: blocked,
		ctx: sdk.NewContext(cache, tmproto.Header{Height: ms.LastCommitID().Version + 1, Time: time.Unix(1, 0)}, false)}
	t.Cleanup(func() { f.close(t) })
	return f
}

func (f *repairM669Fixture) close(t *testing.T) {
	t.Helper()
	if !f.closed {
		require.NoError(t, f.store.Close())
		f.closed = true
	}
}

func (f *repairM669Fixture) commit() {
	f.cache.Write()
	f.store.Commit(true)
	f.cache = f.store.CacheMultiStore()
	f.ctx = f.ctx.WithMultiStore(f.cache).WithBlockHeight(f.store.LastCommitID().Version + 1)
}

func repairM669Address(n byte) sdk.AccAddress {
	return sdk.AccAddress(bytes.Repeat([]byte{n}, 20))
}

func repairM669Coins(denom string, amount int64) sdk.Coins {
	return sdk.NewCoins(sdk.NewInt64Coin(denom, amount))
}

func (f *repairM669Fixture) seed(t *testing.T, coins sdk.Coins) {
	t.Helper()
	f.auth.SetParams(f.ctx, authtypes.DefaultParams())
	f.bank.SetParams(f.ctx, types.DefaultParams())
	for n := byte(1); n <= 3; n++ {
		addr := repairM669Address(n)
		f.auth.SetAccount(f.ctx, f.auth.NewAccountWithAddress(f.ctx, addr))
	}
	require.NoError(t, f.bank.MintCoins(f.ctx, repairM669Module, coins))
	require.NoError(t, f.bank.SendCoinsFromModuleToAccount(f.ctx, repairM669Module, repairM669Address(1), coins))
	f.ctx = f.ctx.WithEventManager(sdk.NewEventManager())
}

func repairM669Doc(t *testing.T, path string, claims ...string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	for _, claim := range claims {
		require.Contains(t, string(b), claim, "documented contract in %s", path)
	}
	return string(b)
}

func repairM669Healthy(t *testing.T, f *repairM669Fixture) {
	t.Helper()
	message, broken := keeper.TotalSupply(f.bank)(f.ctx)
	require.False(t, broken, message)
}

func TestRepairM_669_StateStoresRecovery(t *testing.T) {
	repairM669Doc(t, "spec/01_state.md", "10^12", "0x04 \\| address", "0x11", "CompleteBlock", "before commit", "explicitly stored empty allowlist")
	dir := t.TempDir()
	f := repairM669Open(t, dir)
	a, b := repairM669Address(1), repairM669Address(2)
	f.seed(t, repairM669Coins("uhpx", 10))
	denom := "factory/" + a.String() + "/restricted"
	list := types.AllowList{Addresses: []string{a.String(), b.String()}}
	f.bank.SetDenomAllowList(f.ctx, denom, list)
	require.NoError(t, f.bank.SendCoinsAndWei(f.ctx, a, b, sdk.ZeroInt(), sdk.NewInt(5)))
	require.NoError(t, f.bank.DeferredSendCoinsFromAccountToModule(f.ctx.WithTxIndex(0), a, repairM669Module, repairM669Coins("uhpx", 2)))
	require.NoError(t, f.bank.DeferredSendCoinsFromAccountToModule(f.ctx.WithTxIndex(1), a, repairM669Module, repairM669Coins("uhpx", 1)))
	module := f.auth.GetModuleAddress(repairM669Module)
	require.Equal(t, sdk.ZeroInt(), f.bank.GetBalance(f.ctx, module, "uhpx").Amount)
	var pending []sdk.Coin
	f.bank.IterateDeferredBalances(f.ctx, func(addr sdk.AccAddress, coin sdk.Coin) bool {
		require.Equal(t, module, addr)
		pending = append(pending, coin)
		return false
	})
	require.Equal(t, []sdk.Coin{sdk.NewInt64Coin("uhpx", 2), sdk.NewInt64Coin("uhpx", 1)}, pending)
	for index, amount := range []int64{2, 1} {
		key := append(types.CreateDeferredCacheModuleTxIndexedPrefix(module, uint64(index)), []byte("uhpx")...)
		var stored sdk.Coin
		f.cdc.MustUnmarshal(f.ctx.KVStore(f.deferred).Get(key), &stored)
		require.Equal(t, sdk.NewInt64Coin("uhpx", amount), stored)
	}
	var storedList types.AllowList
	listKey := append(types.DenomAllowListKey(denom), []byte(denom)...)
	f.cdc.MustUnmarshal(f.ctx.KVStore(f.bank.GetStoreKey()).Get(listKey), &storedList)
	require.Equal(t, list, storedList)
	var storedWei sdk.Int
	require.NoError(t, storedWei.Unmarshal(f.ctx.KVStore(f.bank.GetStoreKey()).Get(append(types.WeiBalancesPrefix, b...))))
	require.Equal(t, sdk.NewInt(5), storedWei)
	repairM669Healthy(t, f)
	events := f.bank.WriteDeferredBalances(f.ctx)
	require.Equal(t, sdk.Events{types.NewCoinReceivedEvent(module, repairM669Coins("uhpx", 3))}.ToABCIEvents(), events)
	count := 0
	f.bank.IterateDeferredBalances(f.ctx, func(sdk.AccAddress, sdk.Coin) bool { count++; return false })
	require.Zero(t, count)
	require.Equal(t, sdk.NewInt(6), f.bank.GetBalance(f.ctx, a, "uhpx").Amount)
	require.Equal(t, sdk.NewInt(3), f.bank.GetBalance(f.ctx, module, "uhpx").Amount)
	repairM669Healthy(t, f)
	f.commit()
	f.close(t)
	reopened := repairM669Open(t, dir)
	require.Equal(t, list, reopened.bank.GetDenomAllowList(reopened.ctx, denom))
	require.Equal(t, sdk.NewInt(6), reopened.bank.GetBalance(reopened.ctx, a, "uhpx").Amount)
	require.Equal(t, sdk.NewInt(3), reopened.bank.GetBalance(reopened.ctx, module, "uhpx").Amount)
	require.Equal(t, keeper.OneUhpxInWei.SubRaw(5), reopened.bank.GetWeiBalance(reopened.ctx, a))
	require.Equal(t, sdk.NewInt(5), reopened.bank.GetWeiBalance(reopened.ctx, b))
	require.Empty(t, reopened.bank.WriteDeferredBalances(reopened.ctx), "flushed credits cannot replay on restart")
	repairM669Healthy(t, reopened)
	am := bank.NewAppModule(reopened.cdc, reopened.bank, reopened.auth)
	require.Panics(t, func() { am.ExportGenesis(reopened.ctx, reopened.cdc) }, "normal module export must not lose policy")
	genesis := reopened.bank.ExportGenesisWithAllowLists(reopened.ctx)
	encoded, err := keeper.MarshalGenesisJSON(reopened.cdc, genesis)
	require.NoError(t, err)
	decoded, err := keeper.UnmarshalGenesisJSON(reopened.cdc, encoded)
	require.NoError(t, err)
	recovered := repairM669Open(t, t.TempDir())
	recovered.bank.InitGenesisWithAllowLists(recovered.ctx, decoded)
	require.Equal(t, *genesis, *recovered.bank.ExportGenesisWithAllowLists(recovered.ctx))
	repairM669Healthy(t, recovered)
	empty := repairM669Open(t, t.TempDir())
	empty.seed(t, repairM669Coins("uhpx", 1))
	empty.bank.SetDenomAllowList(empty.ctx, denom, types.AllowList{})
	require.True(t, empty.bank.IsInDenomAllowList(empty.ctx, b, repairM669Coins(denom, 1), make(map[string]keeper.AllowedAddresses)))
	require.Panics(t, func() { empty.bank.ExportGenesis(empty.ctx) })
}

func repairM669Types(t *testing.T, source string) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "contract.go", source, 0)
	require.NoError(t, err)
	result := map[string]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			if typ, ok := spec.(*ast.TypeSpec); ok {
				var b bytes.Buffer
				require.NoError(t, printer.Fprint(&b, fset, typ))
				result[typ.Name.Name] = strings.Join(strings.Fields(b.String()), " ")
			}
		}
	}
	return result
}

func TestRepairM_669_KeeperCapabilities(t *testing.T) {
	doc := repairM669Doc(t, "spec/02_keepers.md", "checkNeg = false", "registration order", "one keyed lookup", "scan an account's denominations")
	documented := map[string]string{}
	for _, block := range strings.Split(doc, "```go\n")[1:] {
		for name, decl := range repairM669Types(t, "package keeper\n"+strings.SplitN(block, "```", 2)[0]) {
			documented[name] = decl
		}
	}
	for name, path := range map[string]string{"Keeper": "keeper/keeper.go", "SendKeeper": "keeper/send.go", "ViewKeeper": "keeper/view.go"} {
		source, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, repairM669Types(t, string(source))[name], documented[name], name)
	}
	f := repairM669Open(t, t.TempDir())
	a, denied := repairM669Address(1), repairM669Address(3)
	f.seed(t, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 5), sdk.NewInt64Coin("ordinary", 2)))
	locked := vesting.NewDelayedVestingAccount(authtypes.NewBaseAccountWithAddress(a), repairM669Coins("uhpx", 4), 100, nil)
	f.auth.SetAccount(f.ctx, locked)
	require.Equal(t, repairM669Coins("uhpx", 4), f.bank.LockedCoins(f.ctx, a))
	require.ErrorIs(t, f.bank.SubUnlockedCoins(f.ctx, a, repairM669Coins("uhpx", 2), true), sdkerrors.ErrInsufficientFunds)
	require.Equal(t, sdk.NewInt(5), f.bank.GetBalance(f.ctx, a, "uhpx").Amount)
	var calls []string
	var before []sdk.Int
	copy := f.bank
	for _, name := range []string{"first", "second"} {
		name := name
		f.bank.RegisterBalanceChangeHook(name, func(_ sdk.Context, _ sdk.AccAddress, _ string, previous func() sdk.Int) error {
			calls = append(calls, name)
			before = append(before, previous())
			return nil
		})
	}
	require.NoError(t, copy.SubUnlockedCoins(f.ctx, a, repairM669Coins("ordinary", 5), false))
	require.Equal(t, sdk.NewInt(-3), f.bank.GetBalance(f.ctx, a, "ordinary").Amount)
	require.Equal(t, []string{"first", "second"}, calls)
	require.Equal(t, []sdk.Int{sdk.NewInt(2), sdk.NewInt(2)}, before)
	require.ErrorIs(t, copy.AddCoins(f.ctx, a, repairM669Coins("ordinary", 1), true), sdkerrors.ErrInvalidCoins)
	require.Equal(t, sdk.NewInt(-3), f.bank.GetBalance(f.ctx, a, "ordinary").Amount)
	require.NoError(t, copy.AddCoins(f.ctx, a, repairM669Coins("ordinary", 3), true))
	require.Equal(t, sdk.ZeroInt(), f.bank.GetBalance(f.ctx, a, "ordinary").Amount)
	require.Nil(t, f.ctx.KVStore(f.bank.GetStoreKey()).Get(types.CreatePrefixedAccountStoreKey(a, []byte("ordinary"))))
	require.ErrorIs(t, copy.SubUnlockedCoins(f.ctx, a, sdk.Coins{{Denom: "ordinary", Amount: sdk.NewInt(-1)}}, false), sdkerrors.ErrInvalidCoins)
	f.bank.RegisterRecipientChecker(func(_ sdk.Context, addr sdk.AccAddress) bool { return !addr.Equals(denied) })
	require.False(t, copy.CanSendTo(f.ctx, denied))
	require.ErrorIs(t, copy.AddCoins(f.ctx, denied, repairM669Coins("ordinary", 1), false), sdkerrors.ErrInvalidRecipient)
	require.ErrorIs(t, copy.AddWei(f.ctx, denied, sdk.OneInt()), sdkerrors.ErrInvalidRecipient)
	f.bank.RegisterSendEnabledChecker(func(_ sdk.Context, denom string) bool { return denom != "ordinary" })
	require.ErrorIs(t, copy.IsSendEnabledCoins(f.ctx, sdk.NewInt64Coin("ordinary", 1)), types.ErrSendDisabled)
	require.NoError(t, copy.IsSendEnabledCoins(f.ctx, sdk.NewInt64Coin("uhpx", 1)))
	hookErr := errors.New("balance observer refused write")
	f.bank.RegisterBalanceChangeHook("refuse", func(sdk.Context, sdk.AccAddress, string, func() sdk.Int) error { return hookErr })
	require.ErrorIs(t, copy.AddCoins(f.ctx, a, repairM669Coins("uhpx", 1), true), hookErr)
	require.Equal(t, sdk.NewInt(5), copy.GetBalance(f.ctx, a, "uhpx").Amount)
	countBeforeWei := len(calls)
	require.NoError(t, copy.AddWei(f.ctx, a, sdk.OneInt()))
	require.Len(t, calls, countBeforeWei, "fractional-only write bypasses ordinary balance hooks")
	require.ErrorIs(t, copy.AddWei(f.ctx, a, keeper.OneUhpxInWei), hookErr)
	require.Equal(t, sdk.OneInt(), copy.GetWeiBalance(f.ctx, a), "failed carry cannot update the fractional store")
	require.Panics(t, func() {
		copy.RegisterBalanceChangeHook("first", func(sdk.Context, sdk.AccAddress, string, func() sdk.Int) error { return nil })
	})
	require.Panics(t, func() { copy.RegisterBalanceChangeHook("", nil) })
	require.Panics(t, func() { copy.RegisterBalanceChangeHook("nil", nil) })
	var visited sdk.Coins
	copy.IterateAccountBalances(f.ctx, a, func(coin sdk.Coin) bool { visited = append(visited, coin); return false })
	require.Equal(t, repairM669Coins("uhpx", 5), visited)
	count := 0
	copy.IterateAllBalances(f.ctx, func(sdk.AccAddress, sdk.Coin) bool { count++; return true })
	require.Equal(t, 1, count)
	require.Equal(t, repairM669Coins("uhpx", 1), copy.SpendableCoins(f.ctx, a))
}

func repairM669Dispatch(t *testing.T, f *repairM669Fixture, multi bool, inputs []types.Input, outputs []types.Output) error {
	t.Helper()
	server := keeper.NewMsgServerImpl(f.bank)
	if multi {
		producer := types.NewMsgMultiSend(inputs, outputs)
		var consumer types.MsgMultiSend
		f.cdc.MustUnmarshal(f.cdc.MustMarshal(producer), &consumer)
		_, err := server.MultiSend(sdk.WrapSDKContext(f.ctx), &consumer)
		return err
	}
	producer := &types.MsgSend{FromAddress: inputs[0].Address, ToAddress: outputs[0].Address, Amount: inputs[0].Coins}
	var consumer types.MsgSend
	f.cdc.MustUnmarshal(f.cdc.MustMarshal(producer), &consumer)
	_, err := server.Send(sdk.WrapSDKContext(f.ctx), &consumer)
	return err
}

func TestRepairM_669_MessageAllowlist(t *testing.T) {
	repairM669Doc(t, "spec/03_messages.md", "missing or empty list allows all addresses", "Carol sending", "Alice sending it to Carol", "every\ninput", "every output")
	a, b, carol := repairM669Address(1), repairM669Address(2), repairM669Address(3)
	denom := "factory/" + a.String() + "/restricted"
	for _, multi := range []bool{false, true} {
		for _, test := range []struct {
			name                       string
			list                       []string
			present, blocked, disabled bool
			from, to                   sdk.AccAddress
			amount                     int64
			want                       error
			message                    string
		}{
			{name: "missing", from: a, to: carol, amount: 1},
			{name: "empty", present: true, from: a, to: carol, amount: 1},
			{name: "allowed", present: true, list: []string{a.String(), b.String()}, from: a, to: b, amount: 1},
			{name: "sender", present: true, list: []string{b.String()}, from: a, to: b, amount: 1, want: sdkerrors.ErrUnauthorized, message: a.String() + " is not allowed to send funds"},
			{name: "recipient", present: true, list: []string{a.String(), b.String()}, from: a, to: carol, amount: 1, want: sdkerrors.ErrUnauthorized, message: carol.String() + " is not allowed to receive funds"},
			{name: "blocked", blocked: true, from: a, to: b, amount: 1, want: sdkerrors.ErrUnauthorized, message: b.String() + " is not allowed to receive funds"},
			{name: "disabled", disabled: true, from: a, to: b, amount: 1, want: types.ErrSendDisabled},
			{name: "insufficient", from: a, to: b, amount: 101, want: sdkerrors.ErrInsufficientFunds},
		} {
			prefix := "Send/"
			if multi {
				prefix = "MultiSend/"
			}
			t.Run(prefix+test.name, func(t *testing.T) {
				f := repairM669Open(t, t.TempDir())
				f.seed(t, repairM669Coins(denom, 100))
				if test.present {
					f.bank.SetDenomAllowList(f.ctx, denom, types.AllowList{Addresses: test.list})
				}
				f.blocked[b.String()] = test.blocked
				if test.disabled {
					params := types.DefaultParams()
					params.DefaultSendEnabled = false
					f.bank.SetParams(f.ctx, params)
				}
				amt := repairM669Coins(denom, test.amount)
				err := repairM669Dispatch(t, f, multi, []types.Input{types.NewInput(test.from, amt)}, []types.Output{types.NewOutput(test.to, amt)})
				if test.want != nil {
					require.ErrorIs(t, err, test.want)
					if test.message != "" {
						require.Contains(t, err.Error(), test.message)
					}
					require.Equal(t, sdk.NewInt(100), f.bank.GetBalance(f.ctx, a, denom).Amount)
					require.True(t, f.bank.GetBalance(f.ctx, test.to, denom).IsZero())
					require.Empty(t, f.ctx.EventManager().Events())
				} else {
					require.NoError(t, err)
					require.Equal(t, sdk.NewInt(100-test.amount), f.bank.GetBalance(f.ctx, a, denom).Amount)
					require.Equal(t, sdk.NewInt(test.amount), f.bank.GetBalance(f.ctx, test.to, denom).Amount)
					repairM669Healthy(t, f)
				}
			})
		}
	}
	for _, lateInput := range []bool{true, false} {
		f := repairM669Open(t, t.TempDir())
		f.seed(t, repairM669Coins(denom, 100))
		f.bank.SetDenomAllowList(f.ctx, denom, types.AllowList{Addresses: []string{a.String(), b.String()}})
		one, two := repairM669Coins(denom, 1), repairM669Coins(denom, 2)
		inputs, outputs := []types.Input{types.NewInput(a, two)}, []types.Output{types.NewOutput(b, one), types.NewOutput(carol, one)}
		message := carol.String() + " is not allowed to receive funds"
		if lateInput {
			inputs = []types.Input{types.NewInput(a, one), types.NewInput(carol, one)}
			outputs = []types.Output{types.NewOutput(b, two)}
			message = carol.String() + " is not allowed to send funds"
		}
		err := repairM669Dispatch(t, f, true, inputs, outputs)
		require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
		require.Contains(t, err.Error(), message)
		require.Equal(t, sdk.NewInt(100), f.bank.GetBalance(f.ctx, a, denom).Amount)
		require.Empty(t, f.ctx.EventManager().Events())
	}
}

func repairM669Event(kind string, attributes ...string) sdk.Event {
	e := sdk.NewEvent(kind)
	for i := 0; i < len(attributes); i += 2 {
		e = e.AppendAttributes(sdk.NewAttribute(attributes[i], attributes[i+1]))
	}
	return e
}

func TestRepairM_669_WeiEvents(t *testing.T) {
	repairM669Doc(t, "spec/04_events.md", "999999999995", "2000000000005", "wei_spent", "wei_received", "wei_transfer", "does not emit an additional")
	f := repairM669Open(t, t.TempDir())
	a, b := repairM669Address(1), repairM669Address(2)
	f.seed(t, repairM669Coins("uhpx", 5))
	require.NoError(t, f.bank.SendCoinsAndWei(f.ctx, a, b, sdk.NewInt(2), sdk.NewInt(5)))
	require.Equal(t, sdk.Events{
		repairM669Event("wei_spent", "spender", a.String(), "amount", "5"),
		repairM669Event("wei_received", "receiver", b.String(), "amount", "5"),
		repairM669Event("wei_transfer", "recipient", b.String(), "sender", a.String(), "amount", "5"),
		repairM669Event("coin_spent", "spender", a.String(), "amount", "2uhpx"),
		repairM669Event("coin_received", "receiver", b.String(), "amount", "2uhpx"),
		repairM669Event("transfer", "recipient", b.String(), "sender", a.String(), "amount", "2uhpx"),
		repairM669Event("message", "sender", a.String()),
	}, f.ctx.EventManager().Events())
	require.Equal(t, sdk.NewInt(2), f.bank.GetBalance(f.ctx, a, "uhpx").Amount)
	require.Equal(t, keeper.OneUhpxInWei.SubRaw(5), f.bank.GetWeiBalance(f.ctx, a))
	require.Equal(t, sdk.NewInt(2), f.bank.GetBalance(f.ctx, b, "uhpx").Amount)
	require.Equal(t, sdk.NewInt(5), f.bank.GetWeiBalance(f.ctx, b))
	repairM669Healthy(t, f)
	f.ctx = f.ctx.WithEventManager(sdk.NewEventManager())
	require.ErrorIs(t, f.bank.SendCoinsAndWei(f.ctx, a, b, sdk.NewInt(100), sdk.OneInt()), sdkerrors.ErrInsufficientFunds)
	require.Empty(t, f.ctx.EventManager().Events())
	require.Equal(t, keeper.OneUhpxInWei.SubRaw(5), f.bank.GetWeiBalance(f.ctx, a))
	require.Equal(t, sdk.NewInt(5), f.bank.GetWeiBalance(f.ctx, b))
	require.Equal(t, sdk.NewInt(2), f.bank.GetBalance(f.ctx, a, "uhpx").Amount)
	require.Equal(t, sdk.NewInt(2), f.bank.GetBalance(f.ctx, b, "uhpx").Amount)
	require.NoError(t, f.bank.SendCoinsAndWei(f.ctx, a, b, sdk.ZeroInt(), sdk.ZeroInt()))
	require.Empty(t, f.ctx.EventManager().Events())
	require.NoError(t, f.bank.SendCoinsAndWei(f.ctx, a, b, sdk.OneInt(), sdk.ZeroInt()))
	require.Equal(t, sdk.Events{
		repairM669Event("wei_transfer", "recipient", b.String(), "sender", a.String(), "amount", "0"),
		repairM669Event("coin_spent", "spender", a.String(), "amount", "1uhpx"),
		repairM669Event("coin_received", "receiver", b.String(), "amount", "1uhpx"),
		repairM669Event("transfer", "recipient", b.String(), "sender", a.String(), "amount", "1uhpx"),
		repairM669Event("message", "sender", a.String()),
	}, f.ctx.EventManager().Events())
	carry := repairM669Open(t, t.TempDir())
	carry.seed(t, repairM669Coins("uhpx", 2))
	require.NoError(t, carry.bank.SubWei(carry.ctx, a, sdk.NewInt(5)))
	require.Equal(t, sdk.OneInt(), carry.bank.GetBalance(carry.ctx, a, "uhpx").Amount)
	require.Equal(t, keeper.OneUhpxInWei.SubRaw(5), carry.bank.GetWeiBalance(carry.ctx, a))
	require.NoError(t, carry.bank.AddWei(carry.ctx, a, sdk.NewInt(5)))
	require.Equal(t, sdk.NewInt(2), carry.bank.GetBalance(carry.ctx, a, "uhpx").Amount)
	require.Equal(t, sdk.ZeroInt(), carry.bank.GetWeiBalance(carry.ctx, a))
	require.Nil(t, carry.ctx.KVStore(carry.bank.GetStoreKey()).Get(append(types.WeiBalancesPrefix, a...)))
	require.Equal(t, sdk.Events{repairM669Event("wei_spent", "spender", a.String(), "amount", "5"), repairM669Event("wei_received", "receiver", a.String(), "amount", "5")}, carry.ctx.EventManager().Events())
	repairM669Healthy(t, carry)
}

func TestRepairM_669_InvariantWiring(t *testing.T) {
	repairM669Doc(t, "spec/README.md", "no registered all-denomination bank invariant", "light_invariance.supply_enabled", "defaults to `true`", "MockBalancesEnabled", "Malformed changed keys")
	require.False(t, app.MockBalancesEnabled)
	opts := viper.New()
	defaultCfg, err := app.ReadLightInvarianceConfig(opts)
	require.NoError(t, err)
	require.True(t, defaultCfg.SupplyEnabled)
	opts.Set("light_invariance.supply_enabled", false)
	disabledCfg, err := app.ReadLightInvarianceConfig(opts)
	require.NoError(t, err)
	require.False(t, disabledCfg.SupplyEnabled)
	opts.Set("light_invariance.supply_enabled", "invalid-bool")
	_, err = app.ReadLightInvarianceConfig(opts)
	require.Error(t, err)
	for _, scenario := range []string{"other-denomination", "native", "wei-remainder"} {
		t.Run(scenario, func(t *testing.T) {
			f := repairM669Open(t, t.TempDir())
			f.seed(t, repairM669Coins("uhpx", 5))
			f.commit()
			node := &app.App{BankKeeper: f.bank}
			bank.NewAppModule(f.cdc, f.bank, f.auth).RegisterInvariants(nil)
			repairM669Healthy(t, f)
			if scenario == "wei-remainder" {
				require.NoError(t, f.bank.AddWei(f.ctx, repairM669Address(1), sdk.OneInt()))
			} else {
				denom := "uhpx"
				if scenario == "other-denomination" {
					denom = "ordinary"
				}
				require.NoError(t, f.bank.AddCoins(f.ctx, repairM669Address(1), repairM669Coins(denom, 1), true))
			}
			message, broken := keeper.TotalSupply(f.bank)(f.ctx)
			require.True(t, broken, message)
			f.cache.Write()
			require.NotPanics(t, func() { node.LightInvarianceChecks(f.ctx.Context(), f.store, disabledCfg) })
			check := func() { node.LightInvarianceChecks(f.ctx.Context(), f.store, defaultCfg) }
			if scenario == "other-denomination" {
				require.NotPanics(t, check)
			} else if scenario == "wei-remainder" {
				require.PanicsWithValue(t, "non-zero wei diff found! Pre-block wei total 0, post-block wei total 1", check)
			} else {
				require.PanicsWithValue(t, "unexpected uhpx balance total found! Pre-block uhpx total 5 wei total 0 total supply 5, post-block uhpx total 6 wei total 0 total supply 5", check)
			}
		})
	}
	fset := token.NewFileSet()
	source, err := parser.ParseFile(fset, "module.go", nil, 0)
	require.NoError(t, err)
	found := false
	for _, declaration := range source.Decls {
		if fn, ok := declaration.(*ast.FuncDecl); ok && fn.Name.Name == "RegisterInvariants" {
			found = true
			require.Empty(t, fn.Body.List, "the documented absence of invariant registration must stay explicit")
		}
	}
	require.True(t, found)
}
