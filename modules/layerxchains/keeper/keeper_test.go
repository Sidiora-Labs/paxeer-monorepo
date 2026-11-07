package keeper_test

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strings"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	chains "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

const denom = "apax"

var operator = common.HexToAddress("0x1234")
var other = common.HexToAddress("0x5678")
var authority = common.HexToAddress("0x9999")

type transfer struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Amount sdk.Int `json:"amount"`
}
type bankStub struct {
	key                sdk.StoreKey
	failModuleTransfer bool
}

func (b *bankStub) GetBalance(ctx sdk.Context, address sdk.AccAddress, d string) sdk.Coin {
	bz := ctx.KVStore(b.key).Get(append([]byte{1}, address...))
	amount := sdk.ZeroInt()
	if bz != nil {
		if err := json.Unmarshal(bz, &amount); err != nil {
			panic(err)
		}
	}
	return sdk.NewCoin(d, amount)
}
func (b *bankStub) setBalance(ctx sdk.Context, address sdk.AccAddress, amount sdk.Int) {
	bz, err := json.Marshal(amount)
	if err != nil {
		panic(err)
	}
	ctx.KVStore(b.key).Set(append([]byte{1}, address...), bz)
}
func (b *bankStub) transfers(ctx sdk.Context) []transfer {
	bz := ctx.KVStore(b.key).Get([]byte{2})
	out := []transfer{}
	if bz != nil {
		if err := json.Unmarshal(bz, &out); err != nil {
			panic(err)
		}
	}
	return out
}
func (b *bankStub) send(ctx sdk.Context, from, to sdk.AccAddress, fromName, toName string, coins sdk.Coins) error {
	if !coins.IsValid() || len(coins) != 1 || coins[0].Denom != denom {
		return fmt.Errorf("invalid coins")
	}
	amount := coins[0].Amount
	source := b.GetBalance(ctx, from, denom).Amount
	if source.LT(amount) {
		return fmt.Errorf("insufficient balance")
	}
	b.setBalance(ctx, from, source.Sub(amount))
	b.setBalance(ctx, to, b.GetBalance(ctx, to, denom).Amount.Add(amount))
	ts := append(b.transfers(ctx), transfer{From: fromName, To: toName, Amount: amount})
	bz, err := json.Marshal(ts)
	if err != nil {
		panic(err)
	}
	ctx.KVStore(b.key).Set([]byte{2}, bz)
	return nil
}
func (b *bankStub) SendCoinsFromAccountToModule(ctx sdk.Context, from sdk.AccAddress, to string, coins sdk.Coins) error {
	return b.send(ctx, from, authtypes.NewModuleAddress(to), from.String(), to, coins)
}
func (b *bankStub) SendCoinsFromModuleToAccount(ctx sdk.Context, from string, to sdk.AccAddress, coins sdk.Coins) error {
	return b.send(ctx, authtypes.NewModuleAddress(from), to, from, to.String(), coins)
}
func (b *bankStub) SendCoinsFromModuleToModule(ctx sdk.Context, from, to string, coins sdk.Coins) error {
	if b.failModuleTransfer {
		return fmt.Errorf("module transfer rejected")
	}
	return b.send(ctx, authtypes.NewModuleAddress(from), authtypes.NewModuleAddress(to), from, to, coins)
}

type harness struct {
	ctx  sdk.Context
	cms  sdk.CommitMultiStore
	cdc  codec.BinaryCodec
	key  sdk.StoreKey
	bank *bankStub
	k    *keeper.Keeper
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	key := sdk.NewKVStoreKey(types.StoreKey)
	bankKey := sdk.NewKVStoreKey("bank")
	cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	cms.MountStoreWithDB(bankKey, sdk.StoreTypeIAVL, db)
	cms.MountStoreWithDB(testkeeper.EVMTestApp.EvmKeeper.GetStoreKey(), sdk.StoreTypeIAVL, db)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{ChainID: "hyperpax_125-1", Height: 7}, false)
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	bank := &bankStub{key: bankKey}
	k, err := keeper.NewKeeperWithAddressResolver(cdc, key, bank, func(sdk.Context) string { return denom }, &testkeeper.EVMTestApp.EvmKeeper)
	require.NoError(t, err)
	h := &harness{ctx: ctx, cms: cms, cdc: cdc, key: key, bank: bank, k: k}
	for _, addr := range []common.Address{operator, other} {
		bank.setBalance(ctx, sdk.AccAddress(addr.Bytes()), sdk.NewIntWithDecimal(10000, 6))
	}
	p := types.DefaultParams()
	p.Authority = authority
	k.SetParams(ctx, p)
	return h
}
func (h *harness) create(t *testing.T, name string, gas uint64) types.Domain {
	t.Helper()
	p := h.k.GetParams(h.ctx)
	d, err := h.k.Create(h.ctx, operator, name, gas, 65536, 100, p.MinBond.Add(p.CreationFee), evmtypes.DefaultMinFeePerGas)
	require.NoError(t, err)
	return d
}
func (h *harness) balance(address sdk.AccAddress) sdk.Int {
	return h.bank.GetBalance(h.ctx, address, denom).Amount
}
func snapshot(ctx sdk.Context, key sdk.StoreKey) map[string]string {
	out := map[string]string{}
	it := ctx.KVStore(key).Iterator(nil, nil)
	defer it.Close()
	for ; it.Valid(); it.Next() {
		out[string(it.Key())] = string(it.Value())
	}
	return out
}

func TestBondFundAndRefundUseTheAssociatedAccount(t *testing.T) {
	application := testkeeper.EVMTestApp
	ctx, _ := application.GetContextForDeliverTx(nil).CacheContext()
	k := application.LayerXChainsKeeper
	p := k.GetParams(ctx)
	baseDenom := application.EvmKeeper.GetBaseDenom(ctx)
	account := sdk.AccAddress(common.HexToAddress("0xaabbccdd").Bytes())
	funder := sdk.AccAddress(common.HexToAddress("0x11223344").Bytes())
	application.EvmKeeper.SetAddressMapping(ctx, account, operator)
	application.EvmKeeper.SetAddressMapping(ctx, funder, other)
	require.NotEqual(t, sdk.AccAddress(operator.Bytes()), account)
	require.NotEqual(t, sdk.AccAddress(other.Bytes()), funder)
	value := p.MinBond.Add(p.CreationFee)
	extra := sdk.NewInt(123)
	initial := value.MulRaw(3)
	coins := sdk.NewCoins(sdk.NewCoin(baseDenom, initial))
	require.NoError(t, application.BankKeeper.MintCoins(ctx, evmtypes.ModuleName, coins.Add(coins...)))
	for _, address := range []sdk.AccAddress{account, funder} {
		require.NoError(t, application.BankKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, address, coins))
	}
	balance := func(address sdk.AccAddress) sdk.Int {
		return application.BankKeeper.GetBalance(ctx, address, baseDenom).Amount
	}
	module := authtypes.NewModuleAddress(types.ModuleName)
	collector := authtypes.NewModuleAddress(authtypes.FeeCollectorName)
	moduleBefore, feesBefore := balance(module), balance(collector)
	assertRawBalances := func() {
		t.Helper()
		require.True(t, balance(sdk.AccAddress(operator.Bytes())).IsZero())
		require.True(t, balance(sdk.AccAddress(other.Bytes())).IsZero())
	}
	assertRawBalances()
	domain, err := k.Create(ctx, operator, "associated", 500000, 65536, 100, value, application.EvmKeeper.GetMinimumFeePerGas(ctx))
	require.NoError(t, err)
	require.Equal(t, operator, domain.Operator)
	require.True(t, balance(account).Equal(initial.Sub(value)))
	require.True(t, balance(funder).Equal(initial))
	require.True(t, balance(module).Equal(moduleBefore.Add(p.MinBond)))
	require.True(t, balance(collector).Equal(feesBefore.Add(p.CreationFee)))
	assertRawBalances()
	for _, sender := range []common.Address{operator, other} {
		require.NoError(t, k.Fund(ctx, domain.Id, sender, extra))
		assertRawBalances()
	}
	require.True(t, balance(account).Equal(initial.Sub(value).Sub(extra)))
	require.True(t, balance(funder).Equal(initial.Sub(extra)))
	require.True(t, balance(module).Equal(moduleBefore.Add(p.MinBond).Add(extra.MulRaw(2))))
	stored, ok := k.GetDomain(ctx, domain.Id)
	require.True(t, ok)
	require.True(t, stored.Bond.Equal(p.MinBond.Add(extra.MulRaw(2))))
	require.NoError(t, k.Retire(ctx, operator, domain.Id))
	require.True(t, balance(account).Equal(initial.Sub(p.CreationFee).Add(extra)))
	require.True(t, balance(funder).Equal(initial.Sub(extra)))
	require.True(t, balance(module).Equal(moduleBefore))
	require.True(t, balance(collector).Equal(feesBefore.Add(p.CreationFee)))
	assertRawBalances()
	stored, ok = k.GetDomain(ctx, domain.Id)
	require.True(t, ok)
	require.Equal(t, types.StatusRetired, stored.Status)
	require.True(t, stored.Bond.IsZero())
}

func TestCreateFundPauseRetireRoundTrip(t *testing.T) {
	h := newHarness(t)
	p := h.k.GetParams(h.ctx)
	account := sdk.AccAddress(operator.Bytes())
	original := h.balance(account)
	d := h.create(t, "demo", 500000)
	require.Equal(t, types.Domain{Id: 1, Name: "demo", Operator: operator, Status: types.StatusActive, GasAllowance: 500000, ByteAllowance: 65536, FeeShareBps: 100, Bond: p.MinBond, TargetGasPerBlock: 250000, CreatedHeight: 7}, d)
	stored, ok := h.k.GetDomain(h.ctx, d.Id)
	require.True(t, ok)
	require.Equal(t, d, stored)
	named, ok := h.k.GetDomainByName(h.ctx, "demo")
	require.True(t, ok)
	require.Equal(t, d, named)
	require.True(t, h.balance(authtypes.NewModuleAddress(types.ModuleName)).Equal(p.MinBond))
	require.True(t, h.balance(authtypes.NewModuleAddress(authtypes.FeeCollectorName)).Equal(p.CreationFee))
	require.True(t, h.balance(account).Equal(original.Sub(p.MinBond).Sub(p.CreationFee)))
	ts := h.bank.transfers(h.ctx)
	require.Len(t, ts, 2)
	require.Equal(t, types.ModuleName, ts[0].To)
	require.True(t, ts[0].Amount.Equal(p.MinBond.Add(p.CreationFee)))
	require.Equal(t, types.ModuleName, ts[1].From)
	require.Equal(t, authtypes.FeeCollectorName, ts[1].To)
	require.True(t, ts[1].Amount.Equal(p.CreationFee))
	require.Equal(t, []byte{2, 0, 0, 0, 1}, types.DomainKey(1))
	require.Equal(t, []byte{3, 'd', 'e', 'm', 'o'}, types.NameIndexKey("demo"))
	require.Equal(t, uint32(2), h.k.NextID(h.ctx))
	require.Equal(t, uint32(1), h.k.DomainCount(h.ctx))
	require.True(t, h.k.NextBaseFee(h.ctx, d.Id).Equal(evmtypes.DefaultMinFeePerGas))
	resolved, ok := h.k.ResolveChainID(h.ctx, types.ChainIDOf(d.Id))
	require.True(t, ok)
	require.Equal(t, d, resolved)
	extra := sdk.NewIntWithDecimal(12, 6)
	require.NoError(t, h.k.Fund(h.ctx, d.Id, other, extra))
	stored, _ = h.k.GetDomain(h.ctx, d.Id)
	require.True(t, stored.Bond.Equal(p.MinBond.Add(extra)))
	require.ErrorIs(t, h.k.Pause(h.ctx, other, d.Id), types.ErrNotOperator)
	require.NoError(t, h.k.Pause(h.ctx, operator, d.Id))
	require.Zero(t, h.k.GasAllowance(h.ctx, d.Id))
	require.NoError(t, h.k.Resume(h.ctx, operator, d.Id))
	require.Equal(t, uint64(500000), h.k.GasAllowance(h.ctx, d.Id))
	require.ErrorIs(t, h.k.SetOperator(h.ctx, other, d.Id, other), types.ErrNotOperator)
	require.NoError(t, h.k.SetOperator(h.ctx, operator, d.Id, other))
	require.ErrorIs(t, h.k.Retire(h.ctx, operator, d.Id), types.ErrNotOperator)
	require.NoError(t, h.k.SetOperator(h.ctx, other, d.Id, operator))
	before := h.balance(account)
	require.NoError(t, h.k.Retire(h.ctx, operator, d.Id))
	require.True(t, h.balance(account).Equal(before.Add(p.MinBond).Add(extra)))
	require.True(t, h.balance(authtypes.NewModuleAddress(types.ModuleName)).IsZero())
	stored, ok = h.k.GetDomainByName(h.ctx, "demo")
	require.True(t, ok)
	require.Equal(t, types.StatusRetired, stored.Status)
	require.True(t, stored.Bond.IsZero())
	require.Equal(t, uint32(1), h.k.DomainCount(h.ctx))
	require.Zero(t, h.k.GasAllowance(h.ctx, d.Id))
	require.ErrorIs(t, h.k.Fund(h.ctx, d.Id, operator, extra), types.ErrDomainRetired)
	require.ErrorIs(t, h.k.Resume(h.ctx, operator, d.Id), types.ErrDomainRetired)
	require.ErrorIs(t, h.k.Retire(h.ctx, operator, d.Id), types.ErrDomainRetired)
	_, err := h.k.Create(h.ctx, operator, "demo", 500000, 65536, 100, p.MinBond.Add(p.CreationFee), sdk.OneDec())
	require.ErrorIs(t, err, types.ErrNameTaken)
}

func TestCreateRefusesBadNamesAllowancesAndBond(t *testing.T) {
	h := newHarness(t)
	p := h.k.GetParams(h.ctx)
	value := p.MinBond.Add(p.CreationFee)
	for _, name := range []string{"ab", "-abc", "abc-", "ABC", strings.Repeat("a", 33), "a_b", "aéz"} {
		t.Run(name, func(t *testing.T) {
			_, err := h.k.Create(h.ctx, operator, name, 500000, 65536, 100, value, sdk.OneDec())
			require.ErrorIs(t, err, types.ErrInvalidName)
		})
	}
	for _, tc := range []struct {
		name       string
		gas, bytes uint64
		share      uint16
		value      sdk.Int
		want       error
	}{
		{"lowgas", p.MinGasAllowance - 1, 65536, 100, value, types.ErrAllowanceOutOfRange},
		{"highgas", p.MaxGasAllowance + 1, 65536, 100, value, types.ErrAllowanceOutOfRange},
		{"lowbytes", 500000, p.MinByteAllowance - 1, 100, value, types.ErrAllowanceOutOfRange},
		{"highbytes", 500000, p.MaxByteAllowance + 1, 100, value, types.ErrAllowanceOutOfRange},
		{"highshare", 500000, 65536, p.MaxFeeShareBps + 1, value, types.ErrAllowanceOutOfRange},
		{"lowbond", 500000, 65536, 100, value.SubRaw(1), types.ErrBondTooLow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := h.k.Create(h.ctx, operator, tc.name, tc.gas, tc.bytes, tc.share, tc.value, sdk.OneDec())
			require.ErrorIs(t, err, tc.want)
		})
	}
	require.Empty(t, h.bank.transfers(h.ctx))
	require.Equal(t, uint32(1), h.k.NextID(h.ctx))
	h.bank.failModuleTransfer = true
	before := snapshot(h.ctx, h.key)
	bankBefore := snapshot(h.ctx, h.bank.key)
	_, err := h.k.Create(h.ctx, operator, "rollback", 500000, 65536, 100, value, sdk.OneDec())
	require.EqualError(t, err, "module transfer rejected")
	require.Equal(t, before, snapshot(h.ctx, h.key))
	require.Equal(t, bankBefore, snapshot(h.ctx, h.bank.key))
	h.bank.failModuleTransfer = false
	first := h.create(t, "first", 4000000)
	_, err = h.k.Create(h.ctx, operator, "first", 500000, 65536, 100, value, sdk.OneDec())
	require.ErrorIs(t, err, types.ErrNameTaken)
	_, err = h.k.Create(h.ctx, operator, "overflow", 2250001, 65536, 100, value, sdk.OneDec())
	require.ErrorIs(t, err, types.ErrTotalAllowanceExceeded)
	require.NoError(t, h.k.Pause(h.ctx, operator, first.Id))
	second := h.create(t, "second", 4000000)
	require.ErrorIs(t, h.k.Resume(h.ctx, operator, first.Id), types.ErrTotalAllowanceExceeded)
	require.NoError(t, h.k.Retire(h.ctx, operator, second.Id))
	require.NoError(t, h.k.Resume(h.ctx, operator, first.Id))
	limited := newHarness(t)
	lp := limited.k.GetParams(limited.ctx)
	lp.MaxChains = 1
	limited.k.SetParams(limited.ctx, lp)
	only := limited.create(t, "only", 500000)
	require.NoError(t, limited.k.Retire(limited.ctx, operator, only.Id))
	_, err = limited.k.Create(limited.ctx, operator, "another", 500000, 65536, 100, value, sdk.OneDec())
	require.ErrorIs(t, err, types.ErrTooManyDomains)
	var nilBank *bankStub
	_, err = keeper.NewKeeperWithAddressResolver(h.cdc, h.key, nilBank, func(sdk.Context) string { return denom }, &testkeeper.EVMTestApp.EvmKeeper)
	require.Error(t, err)
	_, err = keeper.NewKeeperWithAddressResolver(h.cdc, nil, h.bank, func(sdk.Context) string { return denom }, &testkeeper.EVMTestApp.EvmKeeper)
	require.Error(t, err)
	_, err = keeper.NewKeeperWithAddressResolver(h.cdc, h.key, nil, func(sdk.Context) string { return denom }, &testkeeper.EVMTestApp.EvmKeeper)
	require.Error(t, err)
	_, err = keeper.NewKeeperWithAddressResolver(h.cdc, h.key, h.bank, func(sdk.Context) string { return denom }, nil)
	require.ErrorContains(t, err, "missing address resolver")
	var nilResolver *evmkeeper.Keeper
	_, err = keeper.NewKeeperWithAddressResolver(h.cdc, h.key, h.bank, func(sdk.Context) string { return denom }, nilResolver)
	require.ErrorContains(t, err, "missing address resolver")
	readOnly, err := keeper.NewKeeper(h.cdc, h.key, h.bank, func(sdk.Context) string { return denom })
	require.NoError(t, err)
	_, err = readOnly.Create(h.ctx, operator, "unresolved", 500000, 65536, 100, value, sdk.OneDec())
	require.ErrorContains(t, err, "address resolver is required for bank transfers")
	require.ErrorContains(t, readOnly.Fund(h.ctx, first.Id, operator, sdk.OneInt()), "address resolver is required for bank transfers")
	require.ErrorContains(t, readOnly.Retire(h.ctx, operator, first.Id), "address resolver is required for bank transfers")
}

func TestGenesisExportImportRoundTrip(t *testing.T) {
	t.Run("accounting-restart-recovery", TestRepairM_1424_GenesisAccountingRestartRecovery)
	h := newHarness(t)
	first := h.create(t, "first", 500000)
	second := h.create(t, "second", 750000)
	require.NoError(t, h.k.Fund(h.ctx, first.Id, other, sdk.NewInt(99)))
	require.NoError(t, h.k.SetOperator(h.ctx, operator, first.Id, other))
	require.NoError(t, h.k.Pause(h.ctx, other, first.Id))
	require.NoError(t, h.k.Retire(h.ctx, operator, second.Id))
	h.k.SetNextBaseFee(h.ctx, first.Id, sdk.MustNewDecFromStr("123.456"))
	h.k.SetNextBaseFee(h.ctx, second.Id, sdk.MustNewDecFromStr("987.654"))
	h.cms.Commit(true)
	reopened, err := keeper.NewKeeperWithAddressResolver(h.cdc, h.key, h.bank, func(sdk.Context) string { return denom }, &testkeeper.EVMTestApp.EvmKeeper)
	require.NoError(t, err)
	gs := reopened.ExportGenesis(h.ctx)
	require.NoError(t, gs.Validate())
	require.Equal(t, sdk.NewInt(100_000_000), gs.Params.MinBond)
	require.Equal(t, sdk.NewInt(10_000_000), gs.Params.CreationFee)
	require.Len(t, gs.Domains, 2)
	require.Equal(t, uint32(3), gs.NextID)
	raw, err := json.Marshal(gs)
	require.NoError(t, err)
	var decoded types.GenesisState
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Equal(t, sdk.NewInt(100_000_000), decoded.Params.MinBond)
	require.Equal(t, sdk.NewInt(10_000_000), decoded.Params.CreationFee)
	fresh := newHarness(t)
	require.NoError(t, fresh.k.InitGenesis(fresh.ctx, decoded))
	require.Equal(t, gs, fresh.k.ExportGenesis(fresh.ctx))
	for _, want := range gs.Domains {
		got, ok := fresh.k.GetDomain(fresh.ctx, want.Id)
		require.True(t, ok)
		require.Equal(t, want, got)
		byName, ok := fresh.k.GetDomainByName(fresh.ctx, want.Name)
		require.True(t, ok)
		require.Equal(t, want, byName)
		require.True(t, gs.NextBaseFees[want.Id].Equal(fresh.k.NextBaseFee(fresh.ctx, want.Id)))
	}
	require.Equal(t, snapshot(h.ctx, h.key), snapshot(fresh.ctx, fresh.key))
	third := fresh.create(t, "third", 500000)
	require.Equal(t, uint32(3), third.Id)
	_, err = fresh.k.Create(fresh.ctx, operator, "second", 500000, 65536, 100, gs.Params.MinBond.Add(gs.Params.CreationFee), sdk.OneDec())
	require.ErrorIs(t, err, types.ErrNameTaken)
	basic := chains.NewAppModuleBasic()
	require.NoError(t, basic.ValidateGenesis(nil, nil, basic.DefaultGenesis(nil)))
	require.Error(t, basic.ValidateGenesis(nil, nil, json.RawMessage(`{`)))
	moduleFresh := newHarness(t)
	app := chains.NewAppModule(moduleFresh.k)
	require.Empty(t, app.InitGenesis(moduleFresh.ctx, nil, raw))
	require.Equal(t, uint64(1), app.ConsensusVersion())
	var moduleExport types.GenesisState
	require.NoError(t, json.Unmarshal(app.ExportGenesis(moduleFresh.ctx, nil), &moduleExport))
	require.Equal(t, *gs, moduleExport)
	for _, mutate := range []func(*types.GenesisState){
		func(g *types.GenesisState) { g.Domains[1].Id = g.Domains[0].Id },
		func(g *types.GenesisState) { g.Domains[1].Name = g.Domains[0].Name },
		func(g *types.GenesisState) { g.NextID = 2 },
		func(g *types.GenesisState) { g.Domains[0].Status = 9 },
		func(g *types.GenesisState) { g.NextBaseFees[99] = sdk.OneDec() },
		func(g *types.GenesisState) { delete(g.NextBaseFees, 1) },
		func(g *types.GenesisState) {
			g.Domains[0].GasAllowance = 4000000
			g.Domains[0].TargetGasPerBlock = 2000000
			g.Domains[0].Status = types.StatusActive
			g.Domains[1].GasAllowance = 4000000
			g.Domains[1].TargetGasPerBlock = 2000000
			g.Domains[1].Status = types.StatusActive
		},
	} {
		var invalid types.GenesisState
		require.NoError(t, json.Unmarshal(raw, &invalid))
		mutate(&invalid)
		require.ErrorIs(t, invalid.Validate(), types.ErrInvalidGenesis)
	}
}

func TestDomainBaseFeeTracksItsOwnUtilisation(t *testing.T) {
	h := newHarness(t)
	d := h.create(t, "demo", 500000)
	second := h.create(t, "other", 1000000)
	minimum, maximum := sdk.NewDec(10), sdk.NewDec(200)
	upward, downward := sdk.MustNewDecFromStr("0.1"), sdk.MustNewDecFromStr("0.2")
	h.k.SetNextBaseFee(h.ctx, d.Id, sdk.NewDec(100))
	otherFee := h.k.NextBaseFee(h.ctx, second.Id)
	fee := h.k.AdjustDomainBaseFee(h.ctx, d.Id, 500000, minimum, maximum, upward, downward)
	require.True(t, fee.Equal(sdk.NewDec(110)))
	fee = h.k.AdjustDomainBaseFee(h.ctx, d.Id, 0, minimum, maximum, upward, downward)
	require.True(t, fee.Equal(sdk.NewDec(88)))
	unchanged := h.k.AdjustDomainBaseFee(h.ctx, d.Id, 250000, minimum, maximum, upward, downward)
	require.True(t, fee.Equal(unchanged))
	h.k.SetNextBaseFee(h.ctx, d.Id, minimum)
	require.True(t, h.k.AdjustDomainBaseFee(h.ctx, d.Id, 0, minimum, maximum, upward, downward).Equal(minimum))
	h.k.SetNextBaseFee(h.ctx, d.Id, maximum)
	require.True(t, h.k.AdjustDomainBaseFee(h.ctx, d.Id, math.MaxUint64, minimum, maximum, upward, downward).Equal(maximum))
	h.k.SetNextBaseFee(h.ctx, d.Id, sdk.NewDec(100))
	require.True(t, h.k.AdjustDomainBaseFee(h.ctx, d.Id, math.MaxUint64, minimum, maximum, upward, downward).Equal(sdk.NewDec(110)))
	require.True(t, h.k.NextBaseFee(h.ctx, second.Id).Equal(otherFee))
	a, b, c := common.HexToHash("0x01"), common.HexToHash("0x02"), common.HexToHash("0x03")
	h.k.RecordDomainTx(h.ctx, d.Id, 7, a, 111)
	h.k.RecordDomainTx(h.ctx, d.Id, 7, b, 222)
	h.k.RecordDomainTx(h.ctx, d.Id, 8, c, 333)
	h.k.RecordDomainTx(h.ctx, second.Id, 7, c, 444)
	require.Equal(t, []common.Hash{a, b}, h.k.DomainTxs(h.ctx, d.Id, 7))
	require.Equal(t, uint64(333), h.k.DomainGasUsed(h.ctx, d.Id, 7))
	require.Equal(t, []common.Hash{c}, h.k.DomainTxs(h.ctx, d.Id, 8))
	require.Equal(t, uint64(333), h.k.DomainGasUsed(h.ctx, d.Id, 8))
	require.Equal(t, []common.Hash{c}, h.k.DomainTxs(h.ctx, second.Id, 7))
	require.Equal(t, uint64(444), h.k.DomainGasUsed(h.ctx, second.Id, 7))
	require.Empty(t, h.k.DomainTxs(h.ctx, d.Id, 9))
	require.Zero(t, h.k.DomainGasUsed(h.ctx, d.Id, 9))
	key := types.HeightKey(types.DomainTxPrefix, d.Id, 7)
	require.Len(t, key, 13)
	require.Equal(t, byte(6), key[0])
	require.Equal(t, d.Id, binary.BigEndian.Uint32(key[1:5]))
	require.Equal(t, uint64(7), binary.BigEndian.Uint64(key[5:]))
}

func TestChainIDMappingIsBijective(t *testing.T) {
	require.Equal(t, uint64(125000000), types.VirtualChainIDBase)
	id, ok := types.DomainOfChainID(big.NewInt(125))
	require.True(t, ok)
	require.Zero(t, id)
	for _, n := range []uint32{1, 2, 1024, math.MaxUint32} {
		chain := types.ChainIDOf(n)
		require.Equal(t, uint64(125000000)+uint64(n), chain.Uint64())
		got, ok := types.DomainOfChainID(chain)
		require.True(t, ok)
		require.Equal(t, n, got)
	}
	for _, chain := range []*big.Int{nil, big.NewInt(-1), big.NewInt(0), big.NewInt(1), big.NewInt(126), big.NewInt(124999999), big.NewInt(125000000), new(big.Int).SetUint64(types.VirtualChainIDBase + uint64(math.MaxUint32) + 1), new(big.Int).Lsh(big.NewInt(1), 100)} {
		_, ok := types.DomainOfChainID(chain)
		require.False(t, ok)
	}
	h := newHarness(t)
	d := h.create(t, "demo", 500000)
	got, ok := h.k.ResolveChainID(h.ctx, types.ChainIDOf(d.Id))
	require.True(t, ok)
	require.Equal(t, d, got)
	_, ok = h.k.ResolveChainID(h.ctx, types.ChainIDOf(999))
	require.False(t, ok)
	_, ok = h.k.ResolveChainID(h.ctx, big.NewInt(126))
	require.False(t, ok)
}

func TestGovernancePauseIsAuthorityOnly(t *testing.T) {
	h := newHarness(t)
	d := h.create(t, "demo", 4000000)
	require.ErrorIs(t, h.k.GovernancePause(h.ctx, operator, d.Id, true), types.ErrNotAuthority)
	stored, _ := h.k.GetDomain(h.ctx, d.Id)
	require.Equal(t, types.StatusActive, stored.Status)
	require.NoError(t, h.k.GovernancePause(h.ctx, authority, d.Id, true))
	require.Zero(t, h.k.GasAllowance(h.ctx, d.Id))
	stored, _ = h.k.GetDomain(h.ctx, d.Id)
	require.Equal(t, types.StatusGovernancePaused, stored.Status)
	require.ErrorIs(t, h.k.Resume(h.ctx, operator, d.Id), types.ErrNotAuthority)
	require.ErrorIs(t, h.k.Pause(h.ctx, operator, d.Id), types.ErrDomainNotActive)
	require.ErrorIs(t, h.k.GovernancePause(h.ctx, other, d.Id, false), types.ErrNotAuthority)
	second := h.create(t, "second", 4000000)
	require.ErrorIs(t, h.k.GovernancePause(h.ctx, authority, d.Id, false), types.ErrTotalAllowanceExceeded)
	require.NoError(t, h.k.Pause(h.ctx, operator, second.Id))
	require.NoError(t, h.k.GovernancePause(h.ctx, authority, d.Id, false))
	require.Equal(t, uint64(4000000), h.k.GasAllowance(h.ctx, d.Id))
	stored, _ = h.k.GetDomain(h.ctx, d.Id)
	require.Equal(t, types.StatusActive, stored.Status)
	require.NoError(t, h.k.Retire(h.ctx, operator, d.Id))
	require.ErrorIs(t, h.k.GovernancePause(h.ctx, authority, d.Id, true), types.ErrDomainRetired)
	require.ErrorIs(t, h.k.GovernancePause(h.ctx, authority, 999, true), types.ErrUnknownDomain)
}
