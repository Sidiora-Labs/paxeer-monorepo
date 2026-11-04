package distribution_test

import (
	"bytes"
	"encoding/binary"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types/proposal"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// All collaborators are production keepers over isolated in-memory stores.
// BaseApp supplies the real protobuf Msg-service routing and message.action event.
type repairM685Fixture struct {
	app          *baseapp.BaseApp
	ctx          sdk.Context
	cdc          *codec.ProtoCodec
	ak           authkeeper.AccountKeeper
	bk           bankkeeper.BaseKeeper
	sk           stakingkeeper.Keeper
	dk           keeper.Keeper
	key          *sdk.KVStoreKey
	paramHandler govtypes.Handler
}

func newRepairM685Fixture(t *testing.T) *repairM685Fixture {
	t.Helper()
	db := dbm.NewMemDB()
	app := baseapp.NewBaseApp(t.Name(), db, nil, nil, testutil.TestAppOpts{})
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	bankKey := sdk.NewKVStoreKey(banktypes.StoreKey)
	deferredKey := sdk.NewKVStoreKey(banktypes.DeferredCacheStoreKey)
	stakingKey := sdk.NewKVStoreKey(stakingtypes.StoreKey)
	distKey := sdk.NewKVStoreKey(types.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	app.MountStores(authKey, bankKey, deferredKey, stakingKey, distKey, paramsKey, transientKey)
	registry := codectypes.NewInterfaceRegistry()
	sdk.RegisterInterfaces(registry)
	cryptocodec.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	stakingtypes.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	proposal.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	sdk.RegisterLegacyAminoCodec(amino)
	cryptocodec.RegisterCrypto(amino)
	authtypes.RegisterLegacyAminoCodec(amino)
	app.SetInterfaceRegistry(registry)
	pk := paramskeeper.NewKeeper(cdc, amino, paramsKey, transientKey)
	permissions := map[string][]string{
		authtypes.FeeCollectorName: nil, types.ModuleName: nil,
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
		"repair_m685_mint":             {authtypes.Minter},
	}
	ak := authkeeper.NewAccountKeeper(cdc, authKey, pk.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, permissions)
	bk := bankkeeper.NewBaseKeeperWithDeferredCache(cdc, bankKey, ak, pk.Subspace(banktypes.ModuleName), nil, deferredKey)
	sk := stakingkeeper.NewKeeper(cdc, stakingKey, ak, bk, pk.Subspace(stakingtypes.ModuleName))
	dk := keeper.NewKeeper(cdc, distKey, pk.Subspace(types.ModuleName), ak, bk, sk, authtypes.FeeCollectorName, nil)
	sk.SetHooks(dk.Hooks())
	types.RegisterMsgServer(app.MsgServiceRouter(), keeper.NewMsgServerImpl(dk))
	require.NoError(t, app.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	ctx := sdk.NewContext(app.CommitMultiStore(), tmproto.Header{Height: 1, ChainID: "repair-m685", Time: time.Unix(1700000000, 0).UTC()}, false)
	ak.SetParams(ctx, authtypes.DefaultParams())
	for name := range permissions {
		ak.SetModuleAccount(ctx, authtypes.NewEmptyModuleAccount(name, permissions[name]...))
	}
	bk.SetParams(ctx, banktypes.DefaultParams())
	stakingParams := stakingtypes.DefaultParams()
	stakingParams.BondDenom = "uhpx"
	sk.SetParams(ctx, stakingParams)
	dk.InitGenesis(ctx, *types.DefaultGenesisState())
	return &repairM685Fixture{app: app, ctx: ctx, cdc: cdc, ak: ak, bk: bk, sk: sk, dk: dk, key: distKey, paramHandler: dk.ParamChangeProposalHandler(params.NewParamChangeProposalHandler(pk))}
}

func repairM685Coins(n int64) sdk.Coins  { return sdk.NewCoins(sdk.NewInt64Coin("uhpx", n)) }
func repairM685Dec(n int64) sdk.DecCoins { return sdk.NewDecCoinsFromCoins(repairM685Coins(n)...) }

func (f *repairM685Fixture) fundModule(t *testing.T, name string, n int64) {
	t.Helper()
	require.NoError(t, f.bk.MintCoins(f.ctx, "repair_m685_mint", repairM685Coins(n)))
	require.NoError(t, f.bk.SendCoinsFromModuleToModule(f.ctx, "repair_m685_mint", name, repairM685Coins(n)))
}

func (f *repairM685Fixture) validator(t *testing.T, id byte, tokens int64, commission sdk.Dec) (stakingtypes.Validator, sdk.ConsAddress) {
	t.Helper()
	pub := ed25519.GenPrivKeyFromSecret([]byte{id}).PubKey()
	val, err := stakingtypes.NewValidator(sdk.ValAddress(bytes.Repeat([]byte{id}, 20)), pub, stakingtypes.Description{})
	require.NoError(t, err)
	val.Tokens = sdk.NewInt(tokens)
	val.DelegatorShares = sdk.NewDec(tokens)
	val.Status = stakingtypes.Bonded
	val.Commission = stakingtypes.NewCommission(commission, sdk.OneDec(), sdk.OneDec())
	f.sk.SetValidator(f.ctx, val)
	require.NoError(t, f.sk.SetValidatorByConsAddr(f.ctx, val))
	f.sk.SetValidatorByPowerIndex(f.ctx, val)
	f.dk.Hooks().AfterValidatorCreated(f.ctx, val.GetOperator())
	return val, sdk.ConsAddress(pub.Address())
}

func repairM685Doc(t *testing.T, name string, claims ...string) string {
	t.Helper()
	data, err := os.ReadFile("spec/" + name + ".md")
	require.NoError(t, err)
	for _, claim := range claims {
		require.Contains(t, string(data), claim, name)
	}
	return string(data)
}

func TestRepairM_685_State(t *testing.T) {
	repairM685Doc(t, "02_state", "one-byte address length", "eight-byte little-endian", "eight-byte big-endian", "Genesis import and export", "0x08", "0x0102030405060708")
	f := newRepairM685Fixture(t)
	v := sdk.ValAddress(bytes.Repeat([]byte{0x11}, 32))
	d := sdk.AccAddress(bytes.Repeat([]byte{0x22}, 20))
	w := sdk.AccAddress(bytes.Repeat([]byte{0x33}, 20))
	proposer := sdk.ConsAddress(bytes.Repeat([]byte{0x44}, 20))
	const period uint64 = 0x0102030405060708
	const height uint64 = 0x1112131415161718
	pool := types.FeePool{CommunityPool: repairM685Dec(7)}
	outstanding := types.ValidatorOutstandingRewards{Rewards: repairM685Dec(13)}
	current := types.NewValidatorCurrentRewards(repairM685Dec(9), period+1)
	historical := types.NewValidatorHistoricalRewards(repairM685Dec(2), 2)
	commission := types.ValidatorAccumulatedCommission{Commission: repairM685Dec(4)}
	starting := types.NewDelegatorStartingInfo(period, sdk.NewDec(100), height-1)
	slash := types.NewValidatorSlashEvent(period, sdk.NewDecWithPrec(2, 1))
	f.dk.SetFeePool(f.ctx, pool)
	f.dk.SetPreviousProposerConsAddr(f.ctx, proposer)
	f.dk.SetValidatorOutstandingRewards(f.ctx, v, outstanding)
	f.dk.SetDelegatorWithdrawAddr(f.ctx, d, w)
	f.dk.SetDelegatorStartingInfo(f.ctx, v, d, starting)
	f.dk.SetValidatorHistoricalRewards(f.ctx, v, period, historical)
	f.dk.SetValidatorCurrentRewards(f.ctx, v, current)
	f.dk.SetValidatorAccumulatedCommission(f.ctx, v, commission)
	f.dk.SetValidatorSlashEvent(f.ctx, v, height, period, slash)
	// These expected keys are written from the documentation, independently of key constructors.
	prefix := func(b byte) []byte { return append([]byte{b, 32}, v...) }
	le := make([]byte, 8)
	binary.LittleEndian.PutUint64(le, period)
	be := make([]byte, 16)
	binary.BigEndian.PutUint64(be, height)
	binary.BigEndian.PutUint64(be[8:], period)
	kv := f.ctx.KVStore(f.key)
	require.Equal(t, f.cdc.MustMarshal(&pool), kv.Get([]byte{0}))
	require.Equal(t, f.cdc.MustMarshal(&gogotypes.BytesValue{Value: proposer}), kv.Get([]byte{1}))
	require.Equal(t, f.cdc.MustMarshal(&outstanding), kv.Get(prefix(2)))
	require.Equal(t, w.Bytes(), kv.Get(append([]byte{3, 20}, d...)))
	require.Equal(t, f.cdc.MustMarshal(&starting), kv.Get(append(append(prefix(4), 20), d...)))
	require.Equal(t, f.cdc.MustMarshal(&historical), kv.Get(append(prefix(5), le...)))
	require.Equal(t, f.cdc.MustMarshal(&current), kv.Get(prefix(6)))
	require.Equal(t, f.cdc.MustMarshal(&commission), kv.Get(prefix(7)))
	require.Equal(t, f.cdc.MustMarshal(&slash), kv.Get(append(prefix(8), be...)))
	f.fundModule(t, types.ModuleName, 20)
	exported := f.dk.ExportGenesis(f.ctx)
	require.Len(t, exported.ValidatorSlashEvents, 1)
	require.Equal(t, period, exported.ValidatorSlashEvents[0].Period)
	require.Equal(t, height, exported.ValidatorSlashEvents[0].Height)
	wire := f.cdc.MustMarshal(exported)
	var imported types.GenesisState
	require.NoError(t, f.cdc.Unmarshal(wire, &imported))
	g := newRepairM685Fixture(t)
	g.fundModule(t, types.ModuleName, 20)
	g.dk.InitGenesis(g.ctx, imported)
	require.Equal(t, wire, g.cdc.MustMarshal(g.dk.ExportGenesis(g.ctx)))
	require.Equal(t, starting, g.dk.GetDelegatorStartingInfo(g.ctx, v, d))
	require.Equal(t, w, g.dk.GetDelegatorWithdrawAddr(g.ctx, d))
	require.Equal(t, proposer, g.dk.GetPreviousProposerConsAddr(g.ctx))
	var seen []uint64
	g.dk.IterateValidatorSlashEventsBetween(g.ctx, v, height, height, func(h uint64, e types.ValidatorSlashEvent) bool {
		require.Equal(t, slash, e)
		seen = append(seen, h)
		return false
	})
	require.Equal(t, []uint64{height}, seen)
}

func TestRepairM_685_Proposer(t *testing.T) {
	repairM685Doc(t, "03_begin_block", "totalPreviousPower == 0", "100uhpx", "60uhpx", "40uhpx", "54uhpx", "empty `DecidedLastCommit`", "zero-amount")
	// The internal consensus package cannot be imported here. Inspect its actual
	// request literal, then exercise the header codec and the in-process ABCI consumer.
	source, err := parser.ParseFile(token.NewFileSet(), "../../../consensus/internal/p2p/giga_router.go", nil, 0)
	require.NoError(t, err)
	emptyCommit := false
	ast.Inspect(source, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "executeBlock" {
			return true
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			kv, ok := n.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "DecidedLastCommit" {
				return true
			}
			literal, ok := kv.Value.(*ast.CompositeLit)
			require.True(t, ok)
			typ, ok := literal.Type.(*ast.SelectorExpr)
			require.True(t, ok)
			require.Equal(t, "CommitInfo", typ.Sel.Name)
			require.Empty(t, literal.Elts)
			emptyCommit = true
			return false
		})
		return false
	})
	require.True(t, emptyCommit, "Autobahn must still supply the documented empty commit")
	f := newRepairM685Fixture(t)
	v, cons := f.validator(t, 1, 100, sdk.NewDecWithPrec(1, 1))
	f.ctx = f.ctx.WithBlockHeader(tmproto.Header{Height: 1, ProposerAddress: cons})
	f.fundModule(t, authtypes.FeeCollectorName, 100)
	distribution.BeginBlocker(f.ctx, nil, f.dk)
	require.Equal(t, repairM685Coins(100), f.bk.GetAllBalances(f.ctx, f.ak.GetModuleAddress(authtypes.FeeCollectorName)))
	// FinalizeBlock requests are plain Go values passed in process. Only their
	// header is protobuf; preserve that round trip before consuming the request.
	header := &tmproto.Header{Height: 2, ProposerAddress: cons}
	wire, err := header.Marshal()
	require.NoError(t, err)
	var decodedHeader tmproto.Header
	require.NoError(t, decodedHeader.Unmarshal(wire))
	require.Equal(t, *header, decodedHeader)
	received := &abci.RequestFinalizeBlock{Header: &decodedHeader, DecidedLastCommit: abci.CommitInfo{}}
	require.Empty(t, received.DecidedLastCommit.Votes)
	f.ctx = f.ctx.WithBlockHeader(*received.Header).WithEventManager(sdk.NewEventManager())
	distribution.BeginBlocker(f.ctx, received.DecidedLastCommit.Votes, f.dk)
	require.Equal(t, repairM685Dec(100), f.dk.GetFeePool(f.ctx).CommunityPool)
	require.True(t, f.dk.GetValidatorOutstandingRewards(f.ctx, v.GetOperator()).Rewards.IsZero())
	require.Empty(t, repairM685AllocationEvents(f.ctx.EventManager().ABCIEvents()))
	transfers := repairM685EventsOfType(f.ctx.EventManager().ABCIEvents(), banktypes.EventTypeTransfer)
	require.Len(t, transfers, 1)
	require.Equal(t, map[string]string{"recipient": f.ak.GetModuleAddress(types.ModuleName).String(), "sender": f.ak.GetModuleAddress(authtypes.FeeCollectorName).String(), "amount": "100uhpx"}, repairM685Attributes(transfers[0]))
	require.True(t, f.bk.GetAllBalances(f.ctx, f.ak.GetModuleAddress(authtypes.FeeCollectorName)).IsZero())
	require.Equal(t, repairM685Coins(100), f.bk.GetAllBalances(f.ctx, f.ak.GetModuleAddress(types.ModuleName)))
	require.Equal(t, cons, f.dk.GetPreviousProposerConsAddr(f.ctx))
	// Contrast the retained nonzero-power calculation at exactly the same defaults.
	g := newRepairM685Fixture(t)
	a, ca := g.validator(t, 1, 60, sdk.NewDecWithPrec(1, 1))
	b, cb := g.validator(t, 2, 40, sdk.ZeroDec())
	g.dk.SetPreviousProposerConsAddr(g.ctx, ca)
	g.fundModule(t, authtypes.FeeCollectorName, 100)
	votes := []abci.VoteInfo{{Validator: abci.Validator{Address: ca, Power: 60}, SignedLastBlock: true}, {Validator: abci.Validator{Address: cb, Power: 40}, SignedLastBlock: false}}
	g.ctx = g.ctx.WithBlockHeight(2).WithEventManager(sdk.NewEventManager())
	distribution.BeginBlocker(g.ctx, votes, g.dk)
	defaultEvents := repairM685AllocationEvents(g.ctx.EventManager().ABCIEvents())
	require.Len(t, defaultEvents, 7)
	require.Equal(t, "proposer_reward", defaultEvents[0].Type)
	require.Equal(t, map[string]string{"amount": "", "validator": a.GetOperator().String()}, repairM685Attributes(defaultEvents[0]))
	require.Equal(t, repairM685Dec(60), g.dk.GetValidatorOutstandingRewards(g.ctx, a.GetOperator()).Rewards)
	require.Equal(t, repairM685Dec(40), g.dk.GetValidatorOutstandingRewards(g.ctx, b.GetOperator()).Rewards)
	require.Equal(t, repairM685Dec(6), g.dk.GetValidatorAccumulatedCommission(g.ctx, a.GetOperator()).Commission)
	require.Equal(t, repairM685Dec(54), g.dk.GetValidatorCurrentRewards(g.ctx, a.GetOperator()).Rewards)
	require.True(t, g.dk.GetFeePool(g.ctx).CommunityPool.IsZero())
}

func TestRepairM_685_Slash(t *testing.T) {
	repairM685Doc(t, "04_messages", "1 - event.Fraction", "rewards +=", "100000000", "80000000", "40000000", "38uhpx")
	f := newRepairM685Fixture(t)
	v, cons := f.validator(t, 1, 100000000, sdk.ZeroDec())
	d := sdk.AccAddress(v.GetOperator())
	f.dk.Hooks().BeforeDelegationCreated(f.ctx, d, v.GetOperator())
	del := stakingtypes.NewDelegation(d, v.GetOperator(), sdk.NewDec(100000000))
	f.sk.SetDelegation(f.ctx, del)
	f.dk.Hooks().AfterDelegationModified(f.ctx, d, v.GetOperator())
	require.Equal(t, uint64(1), f.dk.GetDelegatorStartingInfo(f.ctx, v.GetOperator(), d).PreviousPeriod)
	f.fundModule(t, stakingtypes.BondedPoolName, 100000000)
	f.fundModule(t, types.ModuleName, 38)
	f.ctx = f.ctx.WithBlockHeight(3)
	f.dk.AllocateTokensToValidator(f.ctx, v, repairM685Dec(10))
	f.sk.Slash(f.ctx, cons, 3, 100, sdk.NewDecWithPrec(2, 1))
	v, found := f.sk.GetValidator(f.ctx, v.GetOperator())
	require.True(t, found)
	require.Equal(t, sdk.NewInt(80000000), v.Tokens)
	first, found := f.dk.GetValidatorSlashEvent(f.ctx, v.GetOperator(), 3, 2)
	require.True(t, found)
	require.Equal(t, sdk.NewDecWithPrec(2, 1), first.Fraction)
	require.Equal(t, sdk.MustNewDecFromStr("0.0000001"), f.dk.GetValidatorHistoricalRewards(f.ctx, v.GetOperator(), 2).CumulativeRewardRatio.AmountOf("uhpx"))
	f.ctx = f.ctx.WithBlockHeight(5)
	f.dk.AllocateTokensToValidator(f.ctx, v, repairM685Dec(16))
	f.sk.Slash(f.ctx, cons, 5, 80, sdk.NewDecWithPrec(5, 1))
	v, found = f.sk.GetValidator(f.ctx, v.GetOperator())
	require.True(t, found)
	require.Equal(t, sdk.NewInt(40000000), v.Tokens)
	second, found := f.dk.GetValidatorSlashEvent(f.ctx, v.GetOperator(), 5, 3)
	require.True(t, found)
	require.Equal(t, sdk.NewDecWithPrec(5, 1), second.Fraction)
	require.Equal(t, sdk.MustNewDecFromStr("0.0000003"), f.dk.GetValidatorHistoricalRewards(f.ctx, v.GetOperator(), 3).CumulativeRewardRatio.AmountOf("uhpx"))
	f.ctx = f.ctx.WithBlockHeight(7)
	f.dk.AllocateTokensToValidator(f.ctx, v, repairM685Dec(12))
	got, err := f.dk.WithdrawDelegationRewards(f.ctx, d, v.GetOperator())
	require.NoError(t, err)
	require.Equal(t, repairM685Coins(38), got)
	require.Equal(t, repairM685Coins(38), f.bk.GetAllBalances(f.ctx, d))
	require.True(t, f.dk.GetValidatorOutstandingRewards(f.ctx, v.GetOperator()).Rewards.IsZero())
	start := f.dk.GetDelegatorStartingInfo(f.ctx, v.GetOperator(), d)
	require.Equal(t, sdk.NewDec(40000000), start.Stake)
	require.Equal(t, uint64(4), start.PreviousPeriod)
	require.Equal(t, uint64(7), start.Height)
	require.Equal(t, sdk.MustNewDecFromStr("0.0000006"), f.dk.GetValidatorHistoricalRewards(f.ctx, v.GetOperator(), 4).CumulativeRewardRatio.AmountOf("uhpx"))
	require.True(t, f.dk.CalculateDelegationRewards(f.ctx, v, del, 4).IsZero(), "same-block reinitialization has no new rewards")
}

func repairM685Attributes(event abci.Event) map[string]string {
	attrs := map[string]string{}
	for _, a := range event.Attributes {
		attrs[string(a.Key)] = string(a.Value)
	}
	return attrs
}

func repairM685EventsOfType(events []abci.Event, kind string) []abci.Event {
	var selected []abci.Event
	for _, e := range events {
		if e.Type == kind {
			selected = append(selected, e)
		}
	}
	return selected
}

func repairM685AllocationEvents(events []abci.Event) []abci.Event {
	var selected []abci.Event
	for _, e := range events {
		switch e.Type {
		case types.EventTypeProposerReward, types.EventTypeCommission, types.EventTypeRewards:
			selected = append(selected, e)
		}
	}
	return selected
}

func TestRepairM_685_Events(t *testing.T) {
	doc := repairM685Doc(t, "06_events", "| proposer_reward | amount |", "proposer_reward.amount = 5.000000000000000000uhpx", "Non-default", "message.action")
	f := newRepairM685Fixture(t)
	v, cons := f.validator(t, 1, 100, sdk.NewDecWithPrec(1, 1))
	p := types.DefaultParams()
	p.CommunityTax = sdk.NewDecWithPrec(2, 2)
	p.BaseProposerReward = sdk.NewDecWithPrec(1, 2)
	p.BonusProposerReward = sdk.NewDecWithPrec(4, 2)
	f.dk.SetParams(f.ctx, p)
	f.fundModule(t, authtypes.FeeCollectorName, 100)
	f.ctx = f.ctx.WithEventManager(sdk.NewEventManager())
	f.dk.AllocateTokens(f.ctx, 100, 100, cons, []abci.VoteInfo{{Validator: abci.Validator{Address: cons, Power: 100}, SignedLastBlock: true}})
	events := repairM685AllocationEvents(f.ctx.EventManager().ABCIEvents())
	require.Len(t, events, 5)
	for i, want := range []struct{ kind, amount string }{{"proposer_reward", "5.000000000000000000uhpx"}, {"commission", "0.500000000000000000uhpx"}, {"rewards", "5.000000000000000000uhpx"}, {"commission", "9.300000000000000000uhpx"}, {"rewards", "93.000000000000000000uhpx"}} {
		require.Equal(t, want.kind, events[i].Type)
		require.Equal(t, map[string]string{"amount": want.amount, "validator": v.GetOperator().String()}, repairM685Attributes(events[i]))
		require.Contains(t, doc, want.kind+".amount = "+want.amount)
	}
	require.Equal(t, repairM685Dec(2), f.dk.GetFeePool(f.ctx).CommunityPool)
	require.Equal(t, sdk.MustNewDecFromStr("9.8"), f.dk.GetValidatorAccumulatedCommission(f.ctx, v.GetOperator()).Commission.AmountOf("uhpx"))
	require.Equal(t, sdk.MustNewDecFromStr("88.2"), f.dk.GetValidatorCurrentRewards(f.ctx, v.GetOperator()).Rewards.AmountOf("uhpx"))
	require.Equal(t, repairM685Dec(98), f.dk.GetValidatorOutstandingRewards(f.ctx, v.GetOperator()).Rewards)
	d := sdk.AccAddress(v.GetOperator())
	w := sdk.AccAddress(bytes.Repeat([]byte{3}, 20))
	f.dk.Hooks().BeforeDelegationCreated(f.ctx, d, v.GetOperator())
	f.sk.SetDelegation(f.ctx, stakingtypes.NewDelegation(d, v.GetOperator(), sdk.NewDec(100)))
	f.dk.Hooks().AfterDelegationModified(f.ctx, d, v.GetOperator())
	messages := []sdk.Msg{
		&types.MsgSetWithdrawAddress{DelegatorAddress: d.String(), WithdrawAddress: w.String()},
		&types.MsgWithdrawDelegatorReward{DelegatorAddress: d.String(), ValidatorAddress: v.GetOperator().String()},
		&types.MsgWithdrawValidatorCommission{ValidatorAddress: v.GetOperator().String()},
		&types.MsgFundCommunityPool{Depositor: w.String(), Amount: repairM685Coins(1)},
	}
	for _, msg := range messages {
		// The message codec and BaseApp are the real producer and consumer of action events.
		wire, err := f.cdc.MarshalInterface(msg)
		require.NoError(t, err)
		var decoded sdk.Msg
		require.NoError(t, f.cdc.UnmarshalInterface(wire, &decoded))
		require.Equal(t, msg, decoded)
		f.ctx = f.ctx.WithEventManager(sdk.NewEventManager())
		result, err := f.app.RunMsgs(f.ctx, []sdk.Msg{decoded})
		require.NoError(t, err)
		require.NotEmpty(t, result.Events)
		require.Equal(t, "message", result.Events[0].Type)
		require.Equal(t, map[string]string{"action": sdk.MsgTypeURL(msg)}, repairM685Attributes(result.Events[0]))
		require.Contains(t, doc, sdk.MsgTypeURL(msg))
		moduleMessage := false
		for _, event := range result.Events[1:] {
			if event.Type == "message" && repairM685Attributes(event)["module"] == "distribution" {
				sender := d.String()
				if _, ok := msg.(*types.MsgWithdrawValidatorCommission); ok {
					sender = v.GetOperator().String()
				}
				if _, ok := msg.(*types.MsgFundCommunityPool); ok {
					sender = w.String()
				}
				require.Equal(t, map[string]string{"module": "distribution", "sender": sender}, repairM685Attributes(event))
				moduleMessage = true
			}
		}
		require.True(t, moduleMessage)
		switch msg.(type) {
		case *types.MsgSetWithdrawAddress:
			specific := repairM685EventsOfType(result.Events, "set_withdraw_address")
			require.Len(t, specific, 1)
			require.Equal(t, map[string]string{"withdraw_address": w.String()}, repairM685Attributes(specific[0]))
		case *types.MsgWithdrawDelegatorReward:
			specific := repairM685EventsOfType(result.Events, "withdraw_rewards")
			require.Len(t, specific, 1)
			denom, _ := sdk.GetBaseDenom()
			if denom == "" {
				denom = sdk.DefaultBondDenom
			}
			require.Equal(t, map[string]string{"amount": "0" + denom, "validator": v.GetOperator().String()}, repairM685Attributes(specific[0]))
		case *types.MsgWithdrawValidatorCommission:
			specific := repairM685EventsOfType(result.Events, "withdraw_commission")
			require.Len(t, specific, 1)
			require.Equal(t, map[string]string{"amount": "9uhpx"}, repairM685Attributes(specific[0]))
		case *types.MsgFundCommunityPool:
			transfers := repairM685EventsOfType(result.Events, banktypes.EventTypeTransfer)
			require.Len(t, transfers, 1)
			require.Equal(t, map[string]string{"amount": "1uhpx", "sender": w.String(), "recipient": f.ak.GetModuleAddress(types.ModuleName).String()}, repairM685Attributes(transfers[0]))
		}
	}
	require.Equal(t, w, f.dk.GetDelegatorWithdrawAddr(f.ctx, d))
	require.Equal(t, repairM685Coins(8), f.bk.GetAllBalances(f.ctx, w))
	require.Equal(t, repairM685Dec(3), f.dk.GetFeePool(f.ctx).CommunityPool)
}

func TestRepairM_685_Params(t *testing.T) {
	repairM685Doc(t, "07_params", "[0, 1]", "Zero is valid", "Non-default", "ParamChangeProposalHandler", "SetParams", "0.02", "0.01", "0.04")
	f := newRepairM685Fixture(t)
	defaults := types.DefaultParams()
	require.True(t, defaults.CommunityTax.IsZero())
	require.True(t, defaults.BaseProposerReward.IsZero())
	require.True(t, defaults.BonusProposerReward.IsZero())
	require.True(t, defaults.WithdrawAddrEnabled)
	require.Equal(t, defaults, f.dk.GetParams(f.ctx))
	require.NoError(t, defaults.ValidateBasic())
	for _, pair := range defaults.ParamSetPairs()[:3] {
		require.NoError(t, pair.ValidatorFn(sdk.ZeroDec()))
		require.NoError(t, pair.ValidatorFn(sdk.OneDec()))
		require.Error(t, pair.ValidatorFn(sdk.SmallestDec().Neg()))
		require.Error(t, pair.ValidatorFn(sdk.OneDec().Add(sdk.SmallestDec())))
		require.Error(t, pair.ValidatorFn(sdk.Dec{}))
		require.Error(t, pair.ValidatorFn("0"))
	}
	withdrawPair := defaults.ParamSetPairs()[3]
	require.NoError(t, withdrawPair.ValidatorFn(true))
	require.NoError(t, withdrawPair.ValidatorFn(false))
	require.Error(t, withdrawPair.ValidatorFn("true"))
	for i := 0; i < 3; i++ {
		p := types.DefaultParams()
		rates := []*sdk.Dec{&p.CommunityTax, &p.BaseProposerReward, &p.BonusProposerReward}
		*rates[i] = sdk.OneDec()
		require.NoError(t, p.ValidateBasic())
		f.dk.SetParams(f.ctx, p)
		require.Equal(t, p, f.dk.GetParams(f.ctx))
		*rates[(i+1)%3] = sdk.SmallestDec()
		require.Error(t, p.ValidateBasic())
		before := f.dk.GetParams(f.ctx)
		require.Panics(t, func() { f.dk.SetParams(f.ctx, p) })
		require.Equal(t, before, f.dk.GetParams(f.ctx), "invalid combined bound cannot publish partial writes")
	}
	example := types.DefaultParams()
	example.CommunityTax = sdk.MustNewDecFromStr("0.02")
	example.BaseProposerReward = sdk.MustNewDecFromStr("0.01")
	example.BonusProposerReward = sdk.MustNewDecFromStr("0.04")
	require.NoError(t, example.ValidateBasic())
	f.dk.SetParams(f.ctx, example)
	require.Equal(t, example, f.dk.GetParams(f.ctx))
	// Governance uses the same complete-set bound after decoding its proposal.
	change := proposal.NewParameterChangeProposal("Distribution bounds", "Reject an over-budget final parameter set", []proposal.ParamChange{
		proposal.NewParamChange(types.ModuleName, string(types.ParamStoreKeyBaseProposerReward), `"0.8"`),
		proposal.NewParamChange(types.ModuleName, string(types.ParamStoreKeyBonusProposerReward), `"0.8"`),
	}, false)
	require.NoError(t, change.ValidateBasic())
	wire, err := f.cdc.MarshalInterface(change)
	require.NoError(t, err)
	var content govtypes.Content
	require.NoError(t, f.cdc.UnmarshalInterface(wire, &content))
	require.Error(t, f.paramHandler(f.ctx, content))
	require.Equal(t, example, f.dk.GetParams(f.ctx))
}
