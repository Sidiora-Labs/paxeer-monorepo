package authz_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"regexp"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz/keeper"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

var repair691Time = time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

func repair691Address(n byte) sdk.AccAddress { return sdk.AccAddress(bytes.Repeat([]byte{n}, 20)) }

type repair691State struct {
	cms      storetypes.CommitMultiStore
	cache    storetypes.CacheMultiStore
	ctx      sdk.Context
	cdc      *codec.ProtoCodec
	registry codectypes.InterfaceRegistry
	key      *sdk.KVStoreKey
	keeper   keeper.Keeper
	auth     authkeeper.AccountKeeper
	bank     bankkeeper.BaseKeeper
	staking  stakingkeeper.Keeper
	router   *baseapp.MsgServiceRouter
	closed   bool
}

func repair691Open(t *testing.T, dir string) *repair691State {
	t.Helper()
	db, err := dbm.NewDB("authz", dbm.GoLevelDBBackend, dir)
	require.NoError(t, err)
	cms := store.NewCommitMultiStore(db)
	keys := sdk.NewKVStoreKeys(authz.ModuleName, authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey, paramtypes.StoreKey)
	for _, key := range keys {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	tkey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	cms.MountStoreWithDB(tkey, sdk.StoreTypeTransient, db)
	require.NoError(t, cms.LoadLatestVersion())
	cache := cms.CacheMultiStore()
	ctx := sdk.NewContext(cache, tmproto.Header{Height: cms.LastCommitID().Version + 1, Time: repair691Time}, false)
	registry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	authz.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	stakingtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	std.RegisterLegacyAminoCodec(amino)
	authtypes.RegisterLegacyAminoCodec(amino)
	banktypes.RegisterLegacyAminoCodec(amino)
	stakingtypes.RegisterLegacyAminoCodec(amino)
	pk := paramskeeper.NewKeeper(cdc, amino, keys[paramtypes.StoreKey], tkey)
	ak := authkeeper.NewAccountKeeper(cdc, keys[authtypes.StoreKey], pk.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, map[string][]string{
		"repair691":                    {authtypes.Minter},
		stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
		stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
	})
	bk := bankkeeper.NewBaseKeeper(cdc, keys[banktypes.StoreKey], ak, pk.Subspace(banktypes.ModuleName), nil)
	sk := stakingkeeper.NewKeeper(cdc, keys[stakingtypes.StoreKey], ak, bk, pk.Subspace(stakingtypes.ModuleName))
	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(registry)
	k := keeper.NewKeeper(keys[authz.ModuleName], cdc, router)
	authz.RegisterMsgServer(router, k)
	banktypes.RegisterMsgServer(router, bankkeeper.NewMsgServerImpl(bk))
	stakingtypes.RegisterMsgServer(router, stakingkeeper.NewMsgServerImpl(sk))
	s := &repair691State{cms: cms, cache: cache, ctx: ctx, cdc: cdc, registry: registry, key: keys[authz.ModuleName], keeper: k, auth: ak, bank: bk, staking: sk, router: router}
	if cms.LastCommitID().Version == 0 {
		ak.SetParams(ctx, authtypes.DefaultParams())
		bk.SetParams(ctx, banktypes.DefaultParams())
		sp := stakingtypes.DefaultParams()
		sp.MinCommissionRate = sdk.ZeroDec()
		sk.SetParams(ctx, sp)
	}
	t.Cleanup(func() {
		if !s.closed {
			require.NoError(t, cms.Close())
			s.closed = true
		}
	})
	return s
}

func (s *repair691State) stop(t *testing.T) storetypes.CommitID {
	t.Helper()
	s.cache.Write()
	id := s.cms.Commit(true)
	require.NoError(t, s.cms.Close())
	s.closed = true
	return id
}

func repair691Snapshot(ctx sdk.Context, key *sdk.KVStoreKey) map[string]string {
	entries := make(map[string]string)
	it := sdk.KVStorePrefixIterator(ctx.KVStore(key), nil)
	defer it.Close()
	for ; it.Valid(); it.Next() {
		entries[string(it.Key())] = string(it.Value())
	}
	return entries
}

func repair691SendAuthorization() *banktypes.SendAuthorization {
	return banktypes.NewSendAuthorization(sdk.NewCoins(sdk.NewInt64Coin("uhpx", 100)))
}

// The constructor produces the Any; the real codec transports it to the real server.
func repair691Grant(t *testing.T, s *repair691State, a authz.Authorization, expiration time.Time) (*authz.MsgGrantResponse, error) {
	t.Helper()
	msg, err := authz.NewMsgGrant(repair691Address(1), repair691Address(2), a, expiration)
	require.NoError(t, err)
	var decoded authz.MsgGrant
	require.NoError(t, s.cdc.Unmarshal(s.cdc.MustMarshal(msg), &decoded))
	require.NoError(t, decoded.ValidateBasic())
	return s.keeper.Grant(sdk.WrapSDKContext(s.ctx), &decoded)
}

func repair691AssertExpiredExecution(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	s := repair691Open(t, dir)
	for _, replacement := range []bool{false, true} {
		if replacement {
			_, err := repair691Grant(t, s, repair691SendAuthorization(), repair691Time.Add(time.Hour))
			require.NoError(t, err)
		}
		for _, expiration := range []time.Time{repair691Time.Add(-time.Nanosecond), repair691Time} {
			before := repair691Snapshot(s.ctx, s.key)
			events := append(sdk.Events{}, s.ctx.EventManager().Events()...)
			response, err := repair691Grant(t, s, repair691SendAuthorization(), expiration)
			require.Nil(t, response)
			require.ErrorIs(t, err, authz.ErrInvalidExpirationTime)
			codespace, code, _ := sdkerrors.ABCIInfo(err, false)
			require.Equal(t, "authz", codespace)
			require.Equal(t, uint32(3), code)
			require.Equal(t, before, repair691Snapshot(s.ctx, s.key))
			require.EqualValues(t, events, s.ctx.EventManager().Events())
			require.ErrorIs(t, s.keeper.SaveGrant(s.ctx, repair691Address(2), repair691Address(1), repair691SendAuthorization(), expiration), authz.ErrInvalidExpirationTime)
			require.Equal(t, before, repair691Snapshot(s.ctx, s.key))
			require.EqualValues(t, events, s.ctx.EventManager().Events())
		}
	}
	before := repair691Snapshot(s.ctx, s.key)
	s.stop(t)
	reopened := repair691Open(t, dir)
	require.Equal(t, before, repair691Snapshot(reopened.ctx, reopened.key))
	a, expiration := reopened.keeper.GetCleanAuthorization(reopened.ctx, repair691Address(2), repair691Address(1), sdk.MsgTypeURL(&banktypes.MsgSend{}))
	require.NotNil(t, a)
	require.Equal(t, repair691Time.Add(time.Hour), expiration)
}

func TestRepairM_691_ExpiredGrantAcceptance(t *testing.T) { repair691AssertExpiredExecution(t) }

func TestRepairM_691_ExpirationCases(t *testing.T) {
	for _, delta := range []time.Duration{-time.Nanosecond, 0, time.Nanosecond, time.Hour} {
		g, err := authz.NewGrant(repair691Time, repair691SendAuthorization(), repair691Time.Add(delta))
		if delta <= 0 {
			require.ErrorIs(t, err, authz.ErrInvalidExpirationTime)
		} else {
			require.NoError(t, err)
			require.Equal(t, repair691Time.Add(delta), g.Expiration)
		}
	}
	s := repair691Open(t, t.TempDir())
	_, err := repair691Grant(t, s, repair691SendAuthorization(), repair691Time.Add(time.Nanosecond))
	require.NoError(t, err)
	require.Len(t, s.keeper.ExportGenesis(s.ctx).Authorization, 1)
	require.Len(t, s.ctx.EventManager().Events(), 1)
	s.ctx = s.ctx.WithBlockTime(repair691Time.Add(time.Nanosecond))
	a, _ := s.keeper.GetCleanAuthorization(s.ctx, repair691Address(2), repair691Address(1), sdk.MsgTypeURL(&banktypes.MsgSend{}))
	require.Nil(t, a, "a grant expires exactly at its expiration")
	require.Empty(t, repair691Snapshot(s.ctx, s.key))
}

func TestRepairM_691_PastTimeStructuralValidation(t *testing.T) {
	msg, err := authz.NewMsgGrant(repair691Address(1), repair691Address(2), repair691SendAuthorization(), repair691Time.Add(-time.Hour))
	require.NoError(t, err)
	require.NoError(t, msg.ValidateBasic(), "structural validation has no chain clock")
	repair691AssertExpiredExecution(t)
}

func TestRepairM_691_GenesisValidationRestart(t *testing.T) {
	dir := t.TempDir()
	s := repair691Open(t, dir)
	_, err := repair691Grant(t, s, repair691SendAuthorization(), repair691Time.Add(time.Hour))
	require.NoError(t, err)
	_, err = repair691Grant(t, s, authz.NewGenericAuthorization(sdk.MsgTypeURL(&authz.MsgRevoke{})), repair691Time.Add(2*time.Hour))
	require.NoError(t, err)
	_, err = repair691Grant(t, s, authz.NewGenericAuthorization(sdk.MsgTypeURL(&authz.MsgExec{})), repair691Time.Add(time.Nanosecond))
	require.NoError(t, err)
	s.ctx = s.ctx.WithBlockTime(repair691Time.Add(time.Second))
	exported := s.keeper.ExportGenesis(s.ctx)
	require.NoError(t, authz.ValidateGenesis(*exported), "imports preserve expired records for faithful restoration")
	encoded := s.cdc.MustMarshalJSON(exported)
	var decoded authz.GenesisState
	require.NoError(t, s.cdc.UnmarshalAsJSON(encoded, &decoded))
	require.NoError(t, authz.ValidateGenesis(decoded))
	id := s.stop(t)
	s = repair691Open(t, dir)
	require.Equal(t, id, s.cms.LastCommitID())
	require.JSONEq(t, string(encoded), string(s.cdc.MustMarshalJSON(s.keeper.ExportGenesis(s.ctx))))

	importDir := t.TempDir()
	imported := repair691Open(t, importDir)
	imported.keeper.InitGenesis(imported.ctx, &decoded)
	require.Empty(t, imported.ctx.EventManager().Events())
	require.JSONEq(t, string(encoded), string(imported.cdc.MustMarshalJSON(imported.keeper.ExportGenesis(imported.ctx))))
	imported.stop(t)
	recovered := repair691Open(t, importDir)
	require.JSONEq(t, string(encoded), string(recovered.cdc.MustMarshalJSON(recovered.keeper.ExportGenesis(recovered.ctx))))
	recovered.ctx = recovered.ctx.WithBlockTime(repair691Time.Add(time.Second))
	a, _ := recovered.keeper.GetCleanAuthorization(recovered.ctx, repair691Address(2), repair691Address(1), sdk.MsgTypeURL(&authz.MsgExec{}))
	require.Nil(t, a)
	a, _ = recovered.keeper.GetCleanAuthorization(recovered.ctx, repair691Address(2), repair691Address(1), sdk.MsgTypeURL(&banktypes.MsgSend{}))
	require.IsType(t, &banktypes.SendAuthorization{}, a)

	valid := decoded.Authorization[0]
	invalidSend, err := codectypes.NewAnyWithValue(&banktypes.SendAuthorization{})
	require.NoError(t, err)
	wrongAny, err := codectypes.NewAnyWithValue(&banktypes.MsgSend{})
	require.NoError(t, err)
	invalidEnum, err := codectypes.NewAnyWithValue(&stakingtypes.StakeAuthorization{AuthorizationType: stakingtypes.AuthorizationType(99)})
	require.NoError(t, err)
	unknownMessage, err := codectypes.NewAnyWithValue(authz.NewGenericAuthorization("/unsupported.Msg"))
	require.NoError(t, err)
	cases := map[string]func(*authz.GrantAuthorization){
		"bad granter":    func(e *authz.GrantAuthorization) { e.Granter = "invalid" },
		"bad grantee":    func(e *authz.GrantAuthorization) { e.Grantee = "invalid" },
		"same addresses": func(e *authz.GrantAuthorization) { e.Grantee = e.Granter },
		"nil Any":        func(e *authz.GrantAuthorization) { e.Authorization = nil },
		"unknown Any": func(e *authz.GrantAuthorization) {
			e.Authorization = &codectypes.Any{TypeUrl: "/unsupported.Authorization", Value: []byte{1}}
		},
		"non authorization Any": func(e *authz.GrantAuthorization) { e.Authorization = wrongAny },
		"invalid spend limit":   func(e *authz.GrantAuthorization) { e.Authorization = invalidSend },
		"unsupported enum":      func(e *authz.GrantAuthorization) { e.Authorization = invalidEnum },
		"unsupported message":   func(e *authz.GrantAuthorization) { e.Authorization = unknownMessage },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			entry := valid
			change(&entry)
			bad := authz.NewGenesisState([]authz.GrantAuthorization{valid, entry})
			require.Error(t, authz.ValidateGenesis(*bad))
			before := repair691Snapshot(recovered.ctx, recovered.key)
			require.Panics(t, func() { recovered.keeper.InitGenesis(recovered.ctx, bad) })
			require.Equal(t, before, repair691Snapshot(recovered.ctx, recovered.key))
		})
	}
	duplicate := valid
	duplicate.Expiration = duplicate.Expiration.Add(time.Hour)
	require.ErrorContains(t, authz.ValidateGenesis(*authz.NewGenesisState([]authz.GrantAuthorization{valid, duplicate})), "duplicate storage tuple")
	fresh := repair691Open(t, t.TempDir())
	require.Panics(t, func() {
		fresh.keeper.InitGenesis(fresh.ctx, authz.NewGenesisState([]authz.GrantAuthorization{valid, duplicate}))
	})
	require.Empty(t, repair691Snapshot(fresh.ctx, fresh.key), "validate the whole import before the first write")
	require.NoError(t, authz.ValidateGenesis(*authz.DefaultGenesisState()))
}

func TestRepairM_691_ConceptsDenyOnly(t *testing.T) {
	dir := t.TempDir()
	s := repair691Open(t, dir)
	granter, grantee := repair691Address(1), repair691Address(2)
	validator, denied := sdk.ValAddress(repair691Address(3)), sdk.ValAddress(repair691Address(4))
	_, err := stakingtypes.NewStakeAuthorization([]sdk.ValAddress{validator}, []sdk.ValAddress{denied}, stakingtypes.AuthorizationType_AUTHORIZATION_TYPE_DELEGATE, nil)
	require.Error(t, err)
	authorization, err := stakingtypes.NewStakeAuthorization(nil, []sdk.ValAddress{denied}, stakingtypes.AuthorizationType_AUTHORIZATION_TYPE_DELEGATE, nil)
	require.NoError(t, err)
	_, err = repair691Grant(t, s, authorization, repair691Time.Add(time.Hour))
	require.NoError(t, err)
	s.stop(t)
	s = repair691Open(t, dir)
	stored, _ := s.keeper.GetCleanAuthorization(s.ctx, grantee, granter, sdk.MsgTypeURL(&stakingtypes.MsgDelegate{}))
	require.Equal(t, []string{denied.String()}, stored.(*stakingtypes.StakeAuthorization).GetDenyList().Address)
	msg := stakingtypes.NewMsgDelegate(granter, validator, sdk.NewInt64Coin("uhpx", 10))
	meter := sdk.NewGasMeter(100, 1, 1)
	accept, err := stored.Accept(s.ctx.WithGasMeter(meter), msg)
	require.NoError(t, err)
	require.True(t, accept.Accept)
	require.Equal(t, uint64(10), meter.GasConsumed(), "list traversal is charged during acceptance")
	meter = sdk.NewGasMeter(100, 1, 1)
	_, err = stored.Accept(s.ctx.WithGasMeter(meter), stakingtypes.NewMsgDelegate(granter, denied, sdk.NewInt64Coin("uhpx", 10)))
	require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
	require.Equal(t, uint64(10), meter.GasConsumed())

	for _, addr := range []sdk.AccAddress{granter, sdk.AccAddress(validator)} {
		s.auth.SetAccount(s.ctx, s.auth.NewAccountWithAddress(s.ctx, addr))
		coins := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 100000000))
		require.NoError(t, s.bank.MintCoins(s.ctx, "repair691", coins))
		require.NoError(t, s.bank.SendCoinsFromModuleToAccount(s.ctx, "repair691", addr, coins))
	}
	create, err := stakingtypes.NewMsgCreateValidator(validator, ed25519.GenPrivKeyFromSecret([]byte("repair691-validator")).PubKey(),
		sdk.NewInt64Coin("uhpx", 1000000), stakingtypes.Description{}, stakingtypes.NewCommissionRates(sdk.ZeroDec(), sdk.ZeroDec(), sdk.ZeroDec()), sdk.OneInt())
	require.NoError(t, err)
	_, err = stakingkeeper.NewMsgServerImpl(s.staking).CreateValidator(sdk.WrapSDKContext(s.ctx), create)
	require.NoError(t, err)
	exec := authz.NewMsgExec(grantee, []sdk.Msg{msg})
	var decoded authz.MsgExec
	require.NoError(t, s.cdc.Unmarshal(s.cdc.MustMarshal(&exec), &decoded))
	result, err := s.router.Handler(&decoded)(s.ctx, &decoded)
	require.NoError(t, err)
	require.NotNil(t, result)
	delegation, found := s.staking.GetDelegation(s.ctx, granter, validator)
	require.True(t, found)
	require.True(t, delegation.Shares.IsPositive())
	stored, _ = s.keeper.GetCleanAuthorization(s.ctx, grantee, granter, sdk.MsgTypeURL(msg))
	require.Equal(t, []string{denied.String()}, stored.(*stakingtypes.StakeAuthorization).GetDenyList().Address, "MsgExec updates preserve the deny-only policy")
	doc, err := os.ReadFile("spec/01_concepts.md")
	require.NoError(t, err)
	require.Contains(t, string(doc), "persisted deny-only")
	require.Contains(t, string(doc), "10 gas")
}

func TestRepairM_691_MessageExpirationDoc(t *testing.T) {
	doc, err := os.ReadFile("spec/03_messages.md")
	require.NoError(t, err)
	for _, text := range []string{"expiration.After(ctx.BlockTime())", "ErrInvalidExpirationTime", "codespace `authz`", "code `3`", "no grant storage", "no `EventGrant`"} {
		require.Contains(t, string(doc), text)
	}
	repair691AssertExpiredExecution(t)
}

func TestRepairM_691_ClientNetworkDoc(t *testing.T) {
	s := repair691Open(t, t.TempDir())
	_, err := repair691Grant(t, s, repair691SendAuthorization(), repair691Time.Add(time.Hour))
	require.NoError(t, err)
	helper := baseapp.NewQueryServerTestHelper(s.ctx, s.registry)
	authz.RegisterQueryServer(helper, s.keeper)
	client := authz.NewQueryClient(helper)
	pair, err := client.Grants(context.Background(), &authz.QueryGrantsRequest{Granter: repair691Address(1).String(), Grantee: repair691Address(2).String(), MsgTypeUrl: sdk.MsgTypeURL(&banktypes.MsgSend{})})
	require.NoError(t, err)
	byGranter, err := client.GranterGrants(context.Background(), &authz.QueryGranterGrantsRequest{Granter: repair691Address(1).String()})
	require.NoError(t, err)
	byGrantee, err := client.GranteeGrants(context.Background(), &authz.QueryGranteeGrantsRequest{Grantee: repair691Address(2).String()})
	require.NoError(t, err)
	require.Len(t, pair.Grants, 1)
	require.Len(t, byGranter.Grants, 1)
	require.Len(t, byGrantee.Grants, 1)
	doc, err := os.ReadFile("spec/05_client.md")
	require.NoError(t, err)
	for _, addr := range []sdk.AccAddress{repair691Address(1), repair691Address(2)} {
		require.Contains(t, string(doc), addr.String())
		decoded, err := sdk.AccAddressFromBech32(addr.String())
		require.NoError(t, err)
		require.Equal(t, addr, decoded)
	}
	for _, command := range cli.GetQueryCmd().Commands() {
		require.Contains(t, string(doc), "paxd query authz "+command.Name())
	}
	require.NotContains(t, string(doc), "simd")
	require.NotContains(t, string(doc), "cosmos1")
	require.Contains(t, string(doc), "100uhpx")
	blocks := regexp.MustCompile("(?s)<!-- repair-m691:(pair|granter|grantee) -->\\s*```json\\s*(.*?)\\s*```").FindAllSubmatch(doc, -1)
	require.Len(t, blocks, 3)
	responses := map[string]proto.Message{"pair": pair, "granter": byGranter, "grantee": byGrantee}
	for _, block := range blocks {
		actual := s.cdc.MustMarshalJSON(responses[string(block[1])])
		require.JSONEq(t, string(block[2]), string(actual))
		var pretty bytes.Buffer
		require.NoError(t, json.Indent(&pretty, actual, "", "  "))
		t.Logf("real Query/%s response:\n%s", block[1], pretty.String())
	}
}
