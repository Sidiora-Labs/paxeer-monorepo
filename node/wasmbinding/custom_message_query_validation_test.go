package wasmbinding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	oraclewasm "github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/client/wasm"
	oraclebindings "github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/client/wasm/bindings"
	oraclekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/keeper"
	oracletypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/types"
	oracleutils "github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/utils"
	tokenfactorywasm "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/client/wasm"
	tokenfactorybindings "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/client/wasm/bindings"
	tokenfactorykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/keeper"
	tokenfactorytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/wasmbinding/bindings"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	wasmvmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm-runtime/types"
	wasmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/keeper"
	wasmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
)

type customBindingsChain struct {
	ctx       sdk.Context
	keys      map[string]*storetypes.KVStoreKey
	factory   tokenfactorykeeper.Keeper
	oracle    oraclekeeper.Keeper
	bank      bankkeeper.BaseKeeper
	messenger wasmkeeper.Messenger
	querier   func(sdk.Context, json.RawMessage) ([]byte, error)
}

func newCustomBindingsChain(t *testing.T) customBindingsChain {
	t.Helper()
	db := tmdb.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	keys := sdk.NewKVStoreKeys(tokenfactorytypes.StoreKey, oracletypes.StoreKey, authtypes.StoreKey, banktypes.StoreKey, paramtypes.StoreKey)
	for _, key := range keys {
		ms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	transient := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	ms.MountStoreWithDB(transient, sdk.StoreTypeTransient, nil)
	memory := storetypes.NewMemoryStoreKey(oracletypes.MemStoreKey)
	ms.MountStoreWithDB(memory, sdk.StoreTypeMemory, nil)
	require.NoError(t, ms.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, ms.Close()) })
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	tokenfactorytypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	subspace := func(name string) paramtypes.Subspace {
		return paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), keys[paramtypes.StoreKey], transient, name)
	}
	accounts := authkeeper.NewAccountKeeper(cdc, keys[authtypes.StoreKey], subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount,
		map[string][]string{tokenfactorytypes.ModuleName: {authtypes.Minter, authtypes.Burner}, oracletypes.ModuleName: {}})
	bank := bankkeeper.NewBaseKeeper(cdc, keys[banktypes.StoreKey], accounts, subspace(banktypes.ModuleName), map[string]bool{})
	factory := tokenfactorykeeper.NewKeeper(cdc, keys[tokenfactorytypes.StoreKey], subspace(tokenfactorytypes.ModuleName), accounts, bank)
	oracle := oraclekeeper.NewKeeper(cdc, keys[oracletypes.StoreKey], memory, subspace(oracletypes.ModuleName), accounts, bank, nil, nil, "")
	ctx := sdk.NewContext(ms, tmproto.Header{Height: 11, Time: time.Unix(3600, 0).UTC()}, false)
	accounts.SetParams(ctx, authtypes.DefaultParams())
	bank.SetParams(ctx, banktypes.DefaultParams())
	factory.InitGenesis(ctx, *tokenfactorytypes.DefaultGenesis())
	oracle.SetParams(ctx, oracletypes.DefaultParams())
	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(registry)
	tokenfactorytypes.RegisterMsgServer(router, tokenfactorykeeper.NewMsgServerImpl(factory))
	messenger := CustomMessageHandler(router, nil, nil, bank, nil, registry, nil)
	qp := &QueryPlugin{
		oracleHandler:       *oraclewasm.NewOracleWasmQueryHandler(&oracle),
		tokenfactoryHandler: *tokenfactorywasm.NewTokenFactoryWasmQueryHandler(&factory),
	}
	return customBindingsChain{ctx: ctx, keys: keys, factory: factory, oracle: oracle, bank: bank, messenger: messenger, querier: CustomQuerier(qp)}
}

func customBindingsJSON(t *testing.T, value interface{}) json.RawMessage {
	t.Helper()
	bz, err := json.Marshal(value)
	require.NoError(t, err)
	return bz
}

func (c customBindingsChain) dispatch(t *testing.T, sender sdk.AccAddress, payload interface{}) ([]sdk.Event, [][]byte, error) {
	t.Helper()
	msg := wasmvmtypes.CosmosMsg{Custom: customBindingsJSON(t, payload)}
	return c.messenger.DispatchMsg(c.ctx, sender, "", msg, wasmvmtypes.MessageInfo{Sender: sender.String()}, wasmtypes.CodeInfo{})
}

func (c customBindingsChain) query(t *testing.T, route string, payload interface{}) ([]byte, error) {
	t.Helper()
	return c.querier(c.ctx, customBindingsJSON(t, PaxQueryWrapper{Route: route, QueryData: customBindingsJSON(t, payload)}))
}

func (c customBindingsChain) state(t *testing.T) map[string]map[string][]byte {
	t.Helper()
	result := make(map[string]map[string][]byte)
	for name, key := range c.keys {
		values := make(map[string][]byte)
		iter := c.ctx.KVStore(key).Iterator(nil, nil)
		for ; iter.Valid(); iter.Next() {
			values[string(iter.Key())] = bytes.Clone(iter.Value())
		}
		require.NoError(t, iter.Close())
		result[name] = values
	}
	return result
}

func TestCustomBindingsMessageAmbiguousVariants(t *testing.T) {
	c := newCustomBindingsChain(t)
	sender := sdk.AccAddress(bytes.Repeat([]byte{0x31}, 20))
	denom := "factory/" + sender.String() + "/ambiguous"
	zero := sdk.ZeroInt()
	variants := []struct {
		name  string
		value interface{}
	}{
		{"create_denom", bindings.CreateDenom{Subdenom: "ambiguous"}},
		{"mint_tokens", bindings.MintTokens{Amount: sdk.NewInt64Coin(denom, 100)}},
		{"burn_tokens", bindings.BurnTokens{Amount: sdk.NewInt64Coin(denom, 10)}},
		{"change_admin", bindings.ChangeAdmin{Denom: denom, NewAdminAddress: sender.String()}},
		{"set_metadata", bindings.SetMetadata{Metadata: banktypes.Metadata{Base: denom}}},
		{"set_denom_paused", tokenfactorybindings.SetDenomPaused{Denom: denom, Paused: true}},
		{"call_evm", bindings.CallEVM{To: "0x0000000000000000000000000000000000000001", Value: &zero}},
		{"delegate_call_evm", bindings.DelegateCallEVM{To: "0x0000000000000000000000000000000000000001"}},
	}
	before := c.state(t)
	beforeEvents := append(sdk.Events{}, c.ctx.EventManager().Events()...)
	for i, first := range variants {
		for _, second := range variants[i+1:] {
			t.Run(first.name+"_and_"+second.name, func(t *testing.T) {
				events, data, err := c.dispatch(t, sender, map[string]interface{}{first.name: first.value, second.name: second.value})
				require.ErrorIs(t, err, bindings.ErrParsingPaxWasmMsg)
				require.Nil(t, events)
				require.Nil(t, data)
				require.Equal(t, before, c.state(t))
				require.Equal(t, beforeEvents, c.ctx.EventManager().Events())
			})
		}
	}
	t.Run("reject_before_decoding_module_payload", func(t *testing.T) {
		events, data, err := c.dispatch(t, sender, map[string]interface{}{
			"create_denom": bindings.CreateDenom{Subdenom: "ambiguous"}, "mint_tokens": "invalid module payload",
		})
		require.ErrorIs(t, err, bindings.ErrParsingPaxWasmMsg)
		require.Nil(t, events)
		require.Nil(t, data)
		require.Equal(t, before, c.state(t))
	})
	for _, raw := range []json.RawMessage{json.RawMessage(`{}`), json.RawMessage(`null`), json.RawMessage(`{"create_denom":null}`), json.RawMessage(`{"unknown":{}}`)} {
		_, _, err := c.dispatch(t, sender, raw)
		require.ErrorAs(t, err, new(wasmvmtypes.UnsupportedRequest))
		require.Equal(t, before, c.state(t))
	}
}

func TestCustomBindingsMessageKeeperMutationAndAuthorization(t *testing.T) {
	c := newCustomBindingsChain(t)
	sender := sdk.AccAddress(bytes.Repeat([]byte{0x32}, 20))
	other := sdk.AccAddress(bytes.Repeat([]byte{0x33}, 20))
	denom := "factory/" + sender.String() + "/valid"
	create := PaxWasmMessage{CreateDenom: customBindingsJSON(t, bindings.CreateDenom{Subdenom: "valid"}), MintTokens: json.RawMessage(`null`)}
	events, data, err := c.dispatch(t, sender, create)
	require.NoError(t, err)
	require.NotEmpty(t, events)
	require.Len(t, data, 1)
	require.NotEmpty(t, data[0])
	metadata, found := c.bank.GetDenomMetaData(c.ctx, denom)
	require.True(t, found)
	require.Equal(t, denom, metadata.Base)
	authority, err := c.factory.GetAuthorityMetadata(c.ctx, denom)
	require.NoError(t, err)
	require.Equal(t, sender.String(), authority.Admin)

	_, _, err = c.dispatch(t, sender, create)
	require.ErrorIs(t, err, tokenfactorytypes.ErrDenomExists)
	_, _, err = c.dispatch(t, sender, PaxWasmMessage{MintTokens: customBindingsJSON(t, bindings.MintTokens{Amount: sdk.NewInt64Coin(denom, 100)})})
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(100), c.bank.GetBalance(c.ctx, sender, denom).Amount)
	_, _, err = c.dispatch(t, sender, PaxWasmMessage{BurnTokens: customBindingsJSON(t, bindings.BurnTokens{Amount: sdk.NewInt64Coin(denom, 40)})})
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(60), c.bank.GetBalance(c.ctx, sender, denom).Amount)
	require.Equal(t, sdk.NewInt(60), c.bank.GetSupply(c.ctx, denom).Amount)

	for _, paused := range []bool{true, false} {
		_, _, err = c.dispatch(t, sender, PaxWasmMessage{SetDenomPaused: customBindingsJSON(t, tokenfactorybindings.SetDenomPaused{Denom: denom, Paused: paused})})
		require.NoError(t, err)
		require.Equal(t, paused, c.factory.IsDenomPaused(c.ctx, denom))
		if paused {
			before := c.state(t)
			_, _, err = c.dispatch(t, sender, PaxWasmMessage{MintTokens: customBindingsJSON(t, bindings.MintTokens{Amount: sdk.NewInt64Coin(denom, 1)})})
			require.ErrorIs(t, err, tokenfactorytypes.ErrDenomPaused)
			require.Equal(t, before, c.state(t))
		}
	}
	metadata.Name = "Validated token"
	_, _, err = c.dispatch(t, sender, PaxWasmMessage{SetMetadata: customBindingsJSON(t, bindings.SetMetadata{Metadata: metadata})})
	require.NoError(t, err)
	updated, found := c.bank.GetDenomMetaData(c.ctx, denom)
	require.True(t, found)
	require.Equal(t, metadata, updated)
	_, _, err = c.dispatch(t, sender, PaxWasmMessage{ChangeAdmin: customBindingsJSON(t, bindings.ChangeAdmin{Denom: denom, NewAdminAddress: other.String()})})
	require.NoError(t, err)
	authority, err = c.factory.GetAuthorityMetadata(c.ctx, denom)
	require.NoError(t, err)
	require.Equal(t, other.String(), authority.Admin)

	forgedMint := map[string]interface{}{"mint_tokens": map[string]interface{}{"amount": sdk.NewInt64Coin(denom, 7), "sender": other.String()}}
	before := c.state(t)
	events, data, err = c.dispatch(t, sender, forgedMint)
	require.ErrorIs(t, err, tokenfactorytypes.ErrUnauthorized)
	require.Nil(t, events)
	require.Nil(t, data)
	require.Equal(t, before, c.state(t))
	_, _, err = c.dispatch(t, other, forgedMint)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(7), c.bank.GetBalance(c.ctx, other, denom).Amount)
	_, _, err = c.dispatch(t, other, PaxWasmMessage{MintTokens: json.RawMessage(`{"amount":"invalid"}`)})
	require.ErrorIs(t, err, tokenfactorytypes.ErrEncodeTokenFactoryMint)
}

func TestCustomBindingsOracleAmbiguousQuery(t *testing.T) {
	c := newCustomBindingsChain(t)
	before := c.state(t)
	for _, lookback := range []uint64{0, 200} {
		res, err := c.query(t, OracleRoute, oraclebindings.PaxOracleQuery{
			ExchangeRates: &oracletypes.QueryExchangeRatesRequest{},
			OracleTwaps:   &oracletypes.QueryTwapsRequest{LookbackSeconds: lookback},
		})
		require.ErrorIs(t, err, oracletypes.ErrParsingOracleQuery)
		require.Nil(t, res)
		require.Equal(t, before, c.state(t))
	}
	for _, query := range []json.RawMessage{json.RawMessage(`{}`), json.RawMessage(`null`), json.RawMessage(`{"exchange_rates":null,"oracle_twaps":null}`)} {
		res, err := c.query(t, OracleRoute, query)
		require.ErrorIs(t, err, oracletypes.ErrUnknownPaxOracleQuery)
		require.Nil(t, res)
	}
	res, err := c.query(t, OracleRoute, json.RawMessage(`{"exchange_rates":[],"oracle_twaps":{}}`))
	require.ErrorIs(t, err, oracletypes.ErrParsingOracleQuery)
	require.Nil(t, res)
}

func TestCustomBindingsOracleDeclaredResponses(t *testing.T) {
	c := newCustomBindingsChain(t)
	twapQuery := oraclebindings.PaxOracleQuery{OracleTwaps: &oracletypes.QueryTwapsRequest{LookbackSeconds: 200}}
	res, err := c.query(t, OracleRoute, twapQuery)
	require.ErrorIs(t, err, oracletypes.ErrNoTwapData)
	require.Nil(t, res)
	c.oracle.SetBaseExchangeRate(c.ctx, oracleutils.MicroAtomDenom, sdk.NewDec(12))
	c.oracle.AddPriceSnapshot(c.ctx, oracletypes.PriceSnapshot{SnapshotTimestamp: 3600, PriceSnapshotItems: oracletypes.PriceSnapshotItems{
		oracletypes.NewPriceSnapshotItem(oracleutils.MicroAtomDenom, oracletypes.OracleExchangeRate{ExchangeRate: sdk.NewDec(20), LastUpdate: sdk.NewInt(10)}),
	}})
	c.oracle.SetVoteTarget(c.ctx, oracleutils.MicroAtomDenom)
	c.ctx = c.ctx.WithBlockHeight(14).WithBlockTime(time.Unix(3700, 0).UTC())

	res, err = c.query(t, OracleRoute, oraclebindings.PaxOracleQuery{ExchangeRates: &oracletypes.QueryExchangeRatesRequest{}})
	require.NoError(t, err)
	require.JSONEq(t, string(customBindingsJSON(t, oracletypes.QueryExchangeRatesResponse{DenomOracleExchangeRatePairs: oracletypes.DenomOracleExchangeRatePairs{
		oracletypes.NewDenomOracleExchangeRatePair(oracleutils.MicroAtomDenom, sdk.NewDec(12), sdk.NewInt(11), 3600000),
	}})), string(res))
	var rates oracletypes.QueryExchangeRatesResponse
	require.NoError(t, json.Unmarshal(res, &rates))
	require.Len(t, rates.DenomOracleExchangeRatePairs, 1)

	res, err = c.query(t, OracleRoute, twapQuery)
	require.NoError(t, err)
	expectedTwaps := oracletypes.QueryTwapsResponse{OracleTwaps: oracletypes.OracleTwaps{
		{Denom: oracleutils.MicroAtomDenom, Twap: sdk.NewDec(20), LookbackSeconds: 100},
	}}
	require.JSONEq(t, string(customBindingsJSON(t, expectedTwaps)), string(res))
	var twaps oracletypes.QueryTwapsResponse
	require.NoError(t, json.Unmarshal(res, &twaps))
	require.Equal(t, expectedTwaps, twaps)

	res, err = c.query(t, OracleRoute, oraclebindings.PaxOracleQuery{ExchangeRates: &oracletypes.QueryExchangeRatesRequest{}, OracleTwaps: twapQuery.OracleTwaps})
	require.ErrorIs(t, err, oracletypes.ErrParsingOracleQuery)
	require.Nil(t, res)
	res, err = c.query(t, OracleRoute, oraclebindings.PaxOracleQuery{OracleTwaps: &oracletypes.QueryTwapsRequest{LookbackSeconds: 3601}})
	require.ErrorIs(t, err, oracletypes.ErrInvalidTwapLookback)
	require.Nil(t, res)
	res, err = c.query(t, OracleRoute, map[string]interface{}{"exchange_rates": nil, "oracle_twaps": twapQuery.OracleTwaps})
	require.NoError(t, err)
	require.JSONEq(t, string(customBindingsJSON(t, expectedTwaps)), string(res))
}

func TestCustomBindingsTokenFactoryQueryValidationRetained(t *testing.T) {
	c := newCustomBindingsChain(t)
	sender := sdk.AccAddress(bytes.Repeat([]byte{0x34}, 20))
	_, _, err := c.dispatch(t, sender, PaxWasmMessage{CreateDenom: customBindingsJSON(t, bindings.CreateDenom{Subdenom: "query"})})
	require.NoError(t, err)
	denom := "factory/" + sender.String() + "/query"
	queries := []struct {
		name     string
		request  interface{}
		response interface{}
	}{
		{"denom_authority_metadata", &tokenfactorytypes.QueryDenomAuthorityMetadataRequest{Denom: denom}, tokenfactorytypes.QueryDenomAuthorityMetadataResponse{AuthorityMetadata: tokenfactorytypes.DenomAuthorityMetadata{Admin: sender.String()}}},
		{"denoms_from_creator", &tokenfactorytypes.QueryDenomsFromCreatorRequest{Creator: sender.String()}, tokenfactorytypes.QueryDenomsFromCreatorResponse{Denoms: []string{denom}}},
		{"denom_paused", &tokenfactorytypes.QueryDenomPausedRequest{Denom: denom}, tokenfactorytypes.QueryDenomPausedResponse{Paused: false}},
	}
	for i, first := range queries {
		t.Run(first.name, func(t *testing.T) {
			res, err := c.query(t, TokenFactoryRoute, map[string]interface{}{first.name: first.request})
			require.NoError(t, err)
			require.JSONEq(t, string(customBindingsJSON(t, first.response)), string(res))
		})
		for _, second := range queries[i+1:] {
			t.Run(fmt.Sprintf("reject_%s_and_%s", first.name, second.name), func(t *testing.T) {
				res, err := c.query(t, TokenFactoryRoute, map[string]interface{}{first.name: first.request, second.name: second.request})
				require.ErrorIs(t, err, tokenfactorytypes.ErrParsingPaxTokenFactoryQuery)
				require.Nil(t, res)
			})
		}
	}
}
