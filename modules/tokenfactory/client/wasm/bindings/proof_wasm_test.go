// Proves: PaxTokenFactoryQuery and SetDenomPaused -> PaxQueryWrapper/CustomQuerier and PaxWasmMessage/CustomEncoder -> TokenFactoryWasmQueryHandler and keeper.MsgServer.SetDenomPaused.
//
// The proof uses real bank and tokenfactory keepers, commits and reopens their
// database, and decodes responses as node/wasmbinding/test/query_test.go does.
// It repairs SetDenomPaused.UnmarshalJSON in msgs.go accepting absent/null
// paused as false, and QueryPlugin.HandleTokenFactoryQuery in
// node/wasmbinding/queries.go silently choosing one of multiple query variants.
// The response alias in queries.go keeps the exported wasm and keeper types equal.
// Outside this task's scope, modules/tokenfactory/keeper/grpc_query.go:
// Keeper.DenomPaused maps corrupt authority metadata to NotFound; the proof
// preserves that current refusal rather than changing the keeper's error code.
package bindings_test

import (
	"encoding/json"
	"strings"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	epochwasm "github.com/Sidiora-Labs/Paxeer-X-Network/modules/epoch/client/wasm"
	evmwasm "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/client/wasm"
	oraclewasm "github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/client/wasm"
	tfwasm "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/client/wasm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/client/wasm/bindings"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/wasmbinding"
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
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	wasmvmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm-runtime/types"
	wasmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestProofWasmProducerTransportConsumer(t *testing.T) {
	dir := t.TempDir()
	var cms storetypes.CommitMultiStore
	var ctx sdk.Context
	var k keeper.Keeper
	var query func(sdk.Context, json.RawMessage) ([]byte, error)

	// Reconstruct every keeper and the real routing layer on each reopen.
	open := func() {
		t.Helper()
		db, err := dbm.NewDB("proof", dbm.GoLevelDBBackend, dir)
		require.NoError(t, err)
		cms = store.NewCommitMultiStore(db)
		tfKey := sdk.NewKVStoreKey(types.StoreKey)
		bankKey := sdk.NewKVStoreKey(banktypes.StoreKey)
		authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
		paramsKey := sdk.NewKVStoreKey("params")
		transientKey := sdk.NewTransientStoreKey("transient_params")
		for _, key := range []*sdk.KVStoreKey{tfKey, bankKey, authKey, paramsKey} {
			cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, nil)
		}
		cms.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, nil)
		require.NoError(t, cms.LoadLatestVersion())
		ctx = sdk.NewContext(cms, tmproto.Header{}, false)
		cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
		subspace := func(name string) paramtypes.Subspace {
			return paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), paramsKey, transientKey, name)
		}
		ak := authkeeper.NewAccountKeeper(cdc, authKey, subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, nil)
		bk := bankkeeper.NewBaseKeeper(cdc, bankKey, ak, subspace(banktypes.ModuleName), nil)
		k = keeper.NewKeeper(cdc, tfKey, subspace(types.ModuleName), ak, bk)
		qp := wasmbinding.NewQueryPlugin(
			&oraclewasm.OracleWasmQueryHandler{}, &epochwasm.EpochWasmQueryHandler{},
			tfwasm.NewTokenFactoryWasmQueryHandler(&k), &evmwasm.EVMQueryHandler{}, stakingkeeper.Keeper{},
		)
		query = wasmbinding.CustomQuerier(qp)
	}
	reopen := func() {
		t.Helper()
		cms.Commit(true)
		require.NoError(t, cms.Close())
		open()
	}
	open()
	t.Cleanup(func() { require.NoError(t, cms.Close()) })
	sender := sdk.AccAddress([]byte("wasm-proof-creator01"))
	other := sdk.AccAddress([]byte("wasm-proof-stranger1"))
	denom, err := k.CreateDenom(ctx, sender.String(), "proof")
	require.NoError(t, err)
	issuerDenom, err := k.CreateDenom(ctx, sender.String(), "issuer")
	require.NoError(t, err)
	issuer := "did:layerx:" + strings.Repeat("ab", 32)
	require.NoError(t, k.SetIssuerDid(ctx, issuerDenom, issuer))
	require.NoError(t, k.SetDenomPausedByIssuer(ctx, issuerDenom, issuer, true))
	reopen()

	marshal := func(value interface{}) []byte {
		t.Helper()
		raw, err := json.Marshal(value)
		require.NoError(t, err)
		return raw
	}
	wrap := func(request bindings.PaxTokenFactoryQuery) json.RawMessage {
		return marshal(wasmbinding.PaxQueryWrapper{
			Route: wasmbinding.TokenFactoryRoute, QueryData: marshal(request),
		})
	}
	read := func(request bindings.PaxTokenFactoryQuery, response interface{}) {
		t.Helper()
		raw, err := query(ctx, wrap(request))
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(raw, response))
	}
	assertPaused := func(target string, paused bool, did string) {
		t.Helper()
		for i := 0; i < 2; i++ {
			var response bindings.QueryDenomPausedResponse
			read(bindings.PaxTokenFactoryQuery{DenomPaused: &types.QueryDenomPausedRequest{Denom: target}}, &response)
			require.Equal(t, bindings.QueryDenomPausedResponse{Paused: paused, IssuerDid: did}, response)
		}
	}
	encode := func(signer sdk.AccAddress, paused bool) *types.MsgSetDenomPaused {
		t.Helper()
		payload := marshal(bindings.SetDenomPaused{Denom: denom, Paused: paused})
		raw := marshal(wasmbinding.PaxWasmMessage{SetDenomPaused: payload})
		msgs, err := wasmbinding.CustomEncoder(signer, raw, wasmvmtypes.MessageInfo{}, wasmtypes.CodeInfo{})
		require.NoError(t, err)
		require.Len(t, msgs, 1)
		msg, ok := msgs[0].(*types.MsgSetDenomPaused)
		require.True(t, ok)
		require.Equal(t, types.NewMsgSetDenomPaused(signer.String(), denom, paused), msg)
		require.NoError(t, msg.ValidateBasic())
		return msg
	}

	// All query producers are consumed through the same exported JSON router.
	for i := 0; i < 2; i++ {
		var authority types.QueryDenomAuthorityMetadataResponse
		read(bindings.PaxTokenFactoryQuery{DenomAuthorityMetadata: &types.QueryDenomAuthorityMetadataRequest{Denom: denom}}, &authority)
		require.Equal(t, types.DenomAuthorityMetadata{Admin: sender.String()}, authority.AuthorityMetadata)
		var denoms types.QueryDenomsFromCreatorResponse
		read(bindings.PaxTokenFactoryQuery{DenomsFromCreator: &types.QueryDenomsFromCreatorRequest{Creator: sender.String()}}, &denoms)
		require.Equal(t, []string{issuerDenom, denom}, denoms.Denoms)
	}
	assertPaused(denom, false, "")
	assertPaused(issuerDenom, true, issuer)
	for _, paused := range []bool{true, false} {
		msg := encode(sender, paused)
		_, err := keeper.NewMsgServerImpl(k).SetDenomPaused(sdk.WrapSDKContext(ctx), msg)
		require.NoError(t, err)
		// Reusing the query closure before reopen must also see live changes.
		assertPaused(denom, paused, "")
		reopen()
		assertPaused(denom, paused, "")
		assertPaused(issuerDenom, true, issuer)
	}

	t.Run("wrong signer cannot pause", func(t *testing.T) {
		response, err := keeper.NewMsgServerImpl(k).SetDenomPaused(sdk.WrapSDKContext(ctx), encode(other, true))
		require.ErrorIs(t, err, types.ErrUnauthorized)
		require.Nil(t, response)
		assertPaused(denom, false, "")
	})
	t.Run("malformed producer output refused at decoder", func(t *testing.T) {
		for _, payload := range []json.RawMessage{
			json.RawMessage(`{"denom":"` + denom + `","paused":"true"}`),
			json.RawMessage(`{"denom":"` + denom + `","paused":null}`),
			json.RawMessage(`{"denom":"` + denom + `"}`),
			json.RawMessage(`null`),
		} {
			raw := marshal(wasmbinding.PaxWasmMessage{SetDenomPaused: payload})
			msgs, err := wasmbinding.CustomEncoder(sender, raw, wasmvmtypes.MessageInfo{}, wasmtypes.CodeInfo{})
			require.ErrorIs(t, err, types.ErrEncodeTokenFactorySetDenomPaused)
			require.Empty(t, msgs)
		}
		assertPaused(denom, false, "")
	})
	t.Run("query refusals", func(t *testing.T) {
		for _, request := range []bindings.PaxTokenFactoryQuery{
			{DenomAuthorityMetadata: &types.QueryDenomAuthorityMetadataRequest{Denom: denom}, DenomPaused: &types.QueryDenomPausedRequest{Denom: denom}},
			{DenomsFromCreator: &types.QueryDenomsFromCreatorRequest{Creator: sender.String()}, DenomPaused: &types.QueryDenomPausedRequest{Denom: denom}},
			{DenomAuthorityMetadata: &types.QueryDenomAuthorityMetadataRequest{Denom: denom}, DenomsFromCreator: &types.QueryDenomsFromCreatorRequest{Creator: sender.String()}},
		} {
			res, err := query(ctx, wrap(request))
			require.ErrorIs(t, err, types.ErrParsingPaxTokenFactoryQuery)
			require.Nil(t, res)
		}
		res, err := query(ctx, wrap(bindings.PaxTokenFactoryQuery{}))
		require.ErrorIs(t, err, types.ErrUnknownPaxTokenFactoryQuery)
		require.Nil(t, res)
		for _, target := range []string{"", denom + "missing"} {
			res, err := query(ctx, wrap(bindings.PaxTokenFactoryQuery{DenomPaused: &types.QueryDenomPausedRequest{Denom: target}}))
			expected := codes.NotFound
			if target == "" {
				expected = codes.InvalidArgument
			}
			require.Equal(t, expected, status.Code(err))
			require.Nil(t, res)
		}
		malformed := marshal(wasmbinding.PaxQueryWrapper{Route: wasmbinding.TokenFactoryRoute, QueryData: json.RawMessage(`{"denom_paused":{"denom":17}}`)})
		res, err = query(ctx, malformed)
		require.ErrorIs(t, err, types.ErrParsingPaxTokenFactoryQuery)
		require.Nil(t, res)
	})
	t.Run("corrupt metadata refusal survives transport", func(t *testing.T) {
		k.GetDenomPrefixStore(ctx, denom).Set([]byte(types.DenomAuthorityMetadataKey), []byte{0xff})
		reopen()
		res, err := query(ctx, wrap(bindings.PaxTokenFactoryQuery{DenomPaused: &types.QueryDenomPausedRequest{Denom: denom}}))
		require.Equal(t, codes.NotFound, status.Code(err))
		require.Nil(t, res)
	})
}
