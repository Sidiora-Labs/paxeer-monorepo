package tokenfactory_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory"
	tfcli "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/client/cli"
	tfwasm "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/client/wasm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/std"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
)

type repairM1226Chain struct {
	store    storetypes.CommitMultiStore
	ctx      sdk.Context
	cdc      *codec.ProtoCodec
	registry codectypes.InterfaceRegistry
	accounts authkeeper.AccountKeeper
	bank     bankkeeper.BaseKeeper
	factory  keeper.Keeper
	module   tokenfactory.AppModule
	closed   bool
}

func openRepairM1226Chain(t *testing.T, dir string) *repairM1226Chain {
	t.Helper()
	db, err := tmdb.NewGoLevelDB("repair", dir)
	require.NoError(t, err)
	ms := store.NewCommitMultiStore(db)
	keys := sdk.NewKVStoreKeys(types.StoreKey, authtypes.StoreKey, banktypes.StoreKey, paramtypes.StoreKey)
	for _, key := range keys {
		ms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	transient := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	ms.MountStoreWithDB(transient, sdk.StoreTypeTransient, nil)
	require.NoError(t, ms.LoadLatestVersion())
	registry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	subspace := func(name string) paramtypes.Subspace {
		return paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), keys[paramtypes.StoreKey], transient, name)
	}
	accounts := authkeeper.NewAccountKeeper(cdc, keys[authtypes.StoreKey], subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount,
		map[string][]string{types.ModuleName: {authtypes.Minter, authtypes.Burner}, "repair-receiver": {}})
	bank := bankkeeper.NewBaseKeeper(cdc, keys[banktypes.StoreKey], accounts, subspace(banktypes.ModuleName), map[string]bool{})
	factory := keeper.NewKeeper(cdc, keys[types.StoreKey], subspace(types.ModuleName), accounts, bank)
	bank.RegisterSendEnabledChecker(factory.SendEnabledChecker)
	s := &repairM1226Chain{store: ms, ctx: sdk.NewContext(ms, tmproto.Header{Height: 1}, false), cdc: cdc, registry: registry,
		accounts: accounts, bank: bank, factory: factory, module: tokenfactory.NewAppModule(factory, accounts, bank)}
	t.Cleanup(func() { s.close(t) })
	return s
}

func (s *repairM1226Chain) close(t *testing.T) {
	t.Helper()
	if !s.closed {
		require.NoError(t, s.store.Close())
		s.closed = true
	}
}

func (s *repairM1226Chain) init() {
	s.accounts.SetParams(s.ctx, authtypes.DefaultParams())
	s.bank.SetParams(s.ctx, banktypes.DefaultParams())
	s.factory.InitGenesis(s.ctx, *types.DefaultGenesis())
}

func (s *repairM1226Chain) create(t *testing.T, admin sdk.AccAddress, subdenom string) string {
	t.Helper()
	msg := types.NewMsgCreateDenom(admin.String(), subdenom)
	require.NoError(t, msg.ValidateBasic())
	response, err := keeper.NewMsgServerImpl(s.factory).CreateDenom(sdk.WrapSDKContext(s.ctx), msg)
	require.NoError(t, err)
	return response.NewTokenDenom
}

func drainRepairM1226Stream(t *testing.T, stream <-chan json.RawMessage) json.RawMessage {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	var snapshot json.RawMessage
	select {
	case payload, ok := <-stream:
		require.True(t, ok, "stream closed without a snapshot")
		snapshot = payload
	case <-timer.C:
		t.Fatal("snapshot was not available after producer closure")
	}
	select {
	case _, ok := <-stream:
		require.False(t, ok, "stream emitted more than one snapshot")
	case <-timer.C:
		t.Fatal("stream did not close after its snapshot")
	}
	return snapshot
}

func TestRepairM_1226_StreamSnapshotBeforeMutationAndClose(t *testing.T) {
	s := openRepairM1226Chain(t, t.TempDir())
	s.init()
	admin := sdk.AccAddress(bytes.Repeat([]byte{0x41}, 20))
	denom := s.create(t, admin, "snapshot")
	issuer := "did:layerx:" + strings.Repeat("ab", 32)
	require.NoError(t, s.factory.SetIssuerDid(s.ctx, denom, issuer))
	require.NoError(t, s.factory.SetDenomPausedByIssuer(s.ctx, denom, issuer, true))
	s.factory.SetParams(s.ctx, types.Params{DenomAllowlistMaxSize: 17})
	expected := s.module.ExportGenesis(s.ctx, s.cdc)
	stream := s.module.ExportGenesisStream(s.ctx, s.cdc)
	require.Equal(t, 1, cap(stream))
	require.Equal(t, 1, len(stream), "snapshot must be queued before return")

	// Change every exported state category before consuming the first stream.
	s.factory.SetParams(s.ctx, types.Params{DenomAllowlistMaxSize: 29})
	require.NoError(t, s.factory.SetDenomPausedByIssuer(s.ctx, denom, issuer, false))
	_, err := keeper.NewMsgServerImpl(s.factory).ChangeAdmin(sdk.WrapSDKContext(s.ctx), types.NewMsgChangeAdmin(admin.String(), denom, ""))
	require.NoError(t, err)
	s.create(t, admin, "later")
	changed := s.module.ExportGenesis(s.ctx, s.cdc)
	require.NotEqual(t, expected, changed)
	second := s.module.ExportGenesisStream(s.ctx, s.cdc)
	s.store.Commit(true)
	s.close(t)
	payload := drainRepairM1226Stream(t, stream)
	require.Equal(t, expected, payload, "snapshot bytes changed after producer mutation/closure")
	require.Equal(t, changed, drainRepairM1226Stream(t, second))

	consumer := openRepairM1226Chain(t, t.TempDir())
	consumer.init()
	require.NoError(t, consumer.module.ValidateGenesis(consumer.cdc, nil, payload))
	require.Empty(t, consumer.module.InitGenesis(consumer.ctx, consumer.cdc, payload))
	require.Equal(t, expected, consumer.module.ExportGenesis(consumer.ctx, consumer.cdc))
	authority, err := consumer.factory.GetAuthorityMetadata(consumer.ctx, denom)
	require.NoError(t, err)
	require.Equal(t, types.DenomAuthorityMetadata{Admin: admin.String(), IssuerDid: issuer}, authority)
	require.True(t, consumer.factory.IsDenomPaused(consumer.ctx, denom))
	require.Equal(t, uint32(17), consumer.factory.GetDenomAllowListMaxSize(consumer.ctx))
}

func TestRepairM_1226_StreamEmptyProducerClosed(t *testing.T) {
	s := openRepairM1226Chain(t, t.TempDir())
	s.init()
	expected := s.module.ExportGenesis(s.ctx, s.cdc)
	stream := s.module.ExportGenesisStream(s.ctx, s.cdc)
	require.Equal(t, 1, len(stream))
	s.close(t)
	payload := drainRepairM1226Stream(t, stream)
	require.Equal(t, expected, payload)
	require.NoError(t, s.module.ValidateGenesis(s.cdc, nil, payload))
	consumer := openRepairM1226Chain(t, t.TempDir())
	consumer.init()
	consumer.module.InitGenesis(consumer.ctx, consumer.cdc, payload)
	require.Equal(t, expected, consumer.module.ExportGenesis(consumer.ctx, consumer.cdc))
}

func TestRepairM_1226_DocumentedAuthorityLifecycle(t *testing.T) {
	guide, err := os.ReadFile("README.md")
	require.NoError(t, err)
	for _, surface := range []string{"issuer_did", "renounces admin", "ErrIssuerMismatch", "ErrPauseUnchanged", "module-transfer", "gRPC", "REST", "CosmWasm", "paused_denoms", `change-admin "$DENOM" ""`} {
		require.Contains(t, string(guide), surface)
	}
	dir := t.TempDir()
	s := openRepairM1226Chain(t, dir)
	s.init()
	admin := sdk.AccAddress(bytes.Repeat([]byte{0x42}, 20))
	recipient := sdk.AccAddress(bytes.Repeat([]byte{0x43}, 20))
	denom := s.create(t, admin, "lifecycle")
	server := keeper.NewMsgServerImpl(s.factory)
	ctx := sdk.WrapSDKContext(s.ctx)
	_, err = server.Mint(ctx, types.NewMsgMint(admin.String(), sdk.NewInt64Coin(denom, 10)))
	require.NoError(t, err)
	_, err = server.SetDenomPaused(ctx, types.NewMsgSetDenomPaused(admin.String(), denom, true))
	require.NoError(t, err)
	_, err = server.SetDenomPaused(ctx, types.NewMsgSetDenomPaused(admin.String(), denom, true))
	require.ErrorIs(t, err, types.ErrPauseUnchanged)
	_, err = server.Mint(ctx, types.NewMsgMint(admin.String(), sdk.NewInt64Coin(denom, 1)))
	require.ErrorIs(t, err, types.ErrDenomPaused)
	_, err = server.Burn(ctx, types.NewMsgBurn(admin.String(), sdk.NewInt64Coin(denom, 1)))
	require.ErrorIs(t, err, types.ErrDenomPaused)
	issuer := "did:layerx:" + strings.Repeat("cd", 32)
	require.NoError(t, s.factory.SetIssuerDid(s.ctx, denom, issuer))
	require.NoError(t, s.factory.SetIssuerDid(s.ctx, denom, issuer))
	require.ErrorIs(t, s.factory.SetIssuerDid(s.ctx, denom, "did:layerx:"+strings.Repeat("ef", 32)), types.ErrInvalidAuthorityMetadata)
	_, err = server.SetDenomPaused(ctx, types.NewMsgSetDenomPaused(admin.String(), denom, false))
	require.ErrorIs(t, err, types.ErrIssuerMismatch)
	require.ErrorIs(t, s.factory.SetDenomPausedByIssuer(s.ctx, denom, "did:layerx:"+strings.Repeat("ef", 32), false), types.ErrIssuerMismatch)
	require.ErrorIs(t, s.factory.MintByIssuer(s.ctx, issuer, sdk.NewInt64Coin(denom, 1), admin.String()), types.ErrDenomPaused)
	require.ErrorIs(t, s.factory.BurnByIssuer(s.ctx, issuer, sdk.NewInt64Coin(denom, 1), admin.String()), types.ErrDenomPaused)

	coins := sdk.NewCoins(sdk.NewInt64Coin(denom, 3))
	_, err = bankkeeper.NewMsgServerImpl(s.bank).Send(ctx, banktypes.NewMsgSend(admin, recipient, coins))
	require.ErrorIs(t, err, banktypes.ErrSendDisabled)
	// Module flows retain their documented exemption even while user sends fail.
	s.accounts.SetModuleAccount(s.ctx, authtypes.NewEmptyModuleAccount("repair-receiver"))
	require.NoError(t, s.bank.SendCoinsFromAccountToModule(s.ctx, admin, types.ModuleName, coins))
	require.NoError(t, s.bank.SendCoinsFromModuleToModule(s.ctx, types.ModuleName, "repair-receiver", coins))
	require.NoError(t, s.bank.SendCoinsFromModuleToAccount(s.ctx, "repair-receiver", recipient, coins))
	require.Equal(t, sdk.NewInt(7), s.bank.GetBalance(s.ctx, admin, denom).Amount)
	require.Equal(t, sdk.NewInt(3), s.bank.GetBalance(s.ctx, recipient, denom).Amount)

	// Run the documented CLI command through real transaction codecs and routes.
	txConfig := authtx.NewTxConfig(s.cdc, authtx.DefaultSignModes)
	var output bytes.Buffer
	clientCtx := client.Context{}.WithCodec(s.cdc).WithInterfaceRegistry(s.registry).WithTxConfig(txConfig).
		WithKeyring(keyring.NewInMemory()).WithChainID("repair-m1226").WithFrom(admin.String()).WithFromAddress(admin).
		WithGenerateOnly(true).WithOffline(true).WithOutput(&output)
	cmd := tfcli.NewChangeAdminCmd()
	cmd.SetContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx))
	cmd.SetOut(&output)
	cmd.SetArgs([]string{denom, ""})
	require.NoError(t, cmd.Execute())
	decoded, err := txConfig.TxJSONDecoder()(output.Bytes())
	require.NoError(t, err)
	msgs := decoded.GetMsgs()
	require.Len(t, msgs, 1)
	require.NoError(t, msgs[0].ValidateBasic())
	require.Equal(t, types.NewMsgChangeAdmin(admin.String(), denom, ""), msgs[0])
	routes := baseapp.NewMsgServiceRouter()
	routes.SetInterfaceRegistry(s.registry)
	queries := baseapp.NewQueryServerTestHelper(s.ctx, s.registry)
	s.module.RegisterServices(module.NewConfigurator(s.cdc, routes, queries))
	handler := routes.Handler(msgs[0])
	require.NotNil(t, handler)
	_, err = handler(s.ctx, msgs[0])
	require.NoError(t, err)
	emptySender := types.NewMsgChangeAdmin("", denom, admin.String())
	_, err = routes.Handler(emptySender)(s.ctx, emptySender)
	require.Error(t, err, "an empty sender cannot reclaim renounced authority")
	_, err = server.ChangeAdmin(ctx, types.NewMsgChangeAdmin(admin.String(), denom, admin.String()))
	require.ErrorIs(t, err, types.ErrUnauthorized)
	_, err = server.Mint(ctx, types.NewMsgMint(admin.String(), sdk.NewInt64Coin(denom, 1)))
	require.ErrorIs(t, err, types.ErrUnauthorized)

	query := types.NewQueryClient(queries)
	metadata, err := query.DenomAuthorityMetadata(context.Background(), &types.QueryDenomAuthorityMetadataRequest{Denom: denom})
	require.NoError(t, err)
	require.Equal(t, types.DenomAuthorityMetadata{IssuerDid: issuer}, metadata.AuthorityMetadata)
	expectedPause := &types.QueryDenomPausedResponse{Paused: true, IssuerDid: issuer}
	paused, err := query.DenomPaused(context.Background(), &types.QueryDenomPausedRequest{Denom: denom})
	require.NoError(t, err)
	require.Equal(t, expectedPause, paused)
	gateway := runtime.NewServeMux()
	require.NoError(t, types.RegisterQueryHandlerClient(context.Background(), gateway, query))
	recorder := httptest.NewRecorder()
	gateway.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/pax-protocol/paxchain/tokenfactory/denom_paused?denom="+url.QueryEscape(denom), nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var rest types.QueryDenomPausedResponse
	require.NoError(t, (&runtime.JSONPb{}).Unmarshal(recorder.Body.Bytes(), &rest))
	require.Equal(t, expectedPause, &rest)
	wasmPause, err := tfwasm.NewTokenFactoryWasmQueryHandler(&s.factory).GetDenomPaused(s.ctx, &types.QueryDenomPausedRequest{Denom: denom})
	require.NoError(t, err)
	require.Equal(t, expectedPause, wasmPause)

	genesis := s.module.ExportGenesis(s.ctx, s.cdc)
	commit := s.store.Commit(true)
	s.close(t)
	reopened := openRepairM1226Chain(t, dir)
	require.Equal(t, commit, reopened.store.LastCommitID())
	require.Equal(t, genesis, reopened.module.ExportGenesis(reopened.ctx, reopened.cdc))
	consumer := openRepairM1226Chain(t, t.TempDir())
	consumer.init()
	require.NoError(t, consumer.module.ValidateGenesis(consumer.cdc, nil, genesis))
	consumer.module.InitGenesis(consumer.ctx, consumer.cdc, genesis)
	require.Equal(t, genesis, consumer.module.ExportGenesis(consumer.ctx, consumer.cdc))
	require.Equal(t, []string{denom}, consumer.factory.GetAllDenomsFromCreator(consumer.ctx, admin.String()))
	require.True(t, consumer.bank.GetBalance(consumer.ctx, recipient, denom).IsZero(), "tokenfactory genesis does not contain bank balances")
	require.NoError(t, consumer.factory.SetDenomPausedByIssuer(consumer.ctx, denom, issuer, false))
	require.NoError(t, consumer.factory.MintByIssuer(consumer.ctx, issuer, sdk.NewInt64Coin(denom, 5), recipient.String()))
	require.Equal(t, sdk.NewInt(5), consumer.bank.GetBalance(consumer.ctx, recipient, denom).Amount)
	_, err = keeper.NewMsgServerImpl(consumer.factory).ChangeAdmin(sdk.WrapSDKContext(consumer.ctx), types.NewMsgChangeAdmin(admin.String(), denom, admin.String()))
	require.ErrorIs(t, err, types.ErrUnauthorized)
}
