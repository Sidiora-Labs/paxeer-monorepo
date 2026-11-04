package rest_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/bech32/legacybech32"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	sdkrest "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/rest"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	slashingrest "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/slashing/client/rest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/slashing/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/slashing/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type repair285Query struct {
	request  abci.RequestQuery
	response abci.ResponseQuery
}

type repair285Fixture struct {
	amino   *codec.LegacyAmino
	client  client.Context
	server  *httptest.Server
	queries chan repair285Query
	pubkey  string
	info    types.ValidatorSigningInfo
}

func newRepair285Fixture(t *testing.T) *repair285Fixture {
	t.Helper()
	amino := codec.NewLegacyAmino()
	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	sc := config.DefaultStateCommitConfig()
	sc.MemIAVLConfig.AsyncCommitBuffer = 0
	sc.MemIAVLConfig.SnapshotInterval = 1
	sc.MemIAVLConfig.SnapshotMinTimeInterval = 0
	store := rootmulti.NewStore(t.TempDir(), sc, config.StateStoreConfig{}, nil)
	app := baseapp.NewBaseApp(t.Name(), dbm.NewMemDB(), nil, nil, &testutil.TestAppOpts{})
	app.SetCMS(store)
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	key := sdk.NewKVStoreKey(types.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	app.MountStores(key, paramsKey, transientKey)
	params := paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, types.ModuleName)
	k := keeper.NewKeeper(cdc, key, nil, params)
	app.QueryRouter().AddRoute(types.QuerierRoute, keeper.NewQuerier(k, amino))
	app.GRPCQueryRouter().SetInterfaceRegistry(registry)
	types.RegisterQueryServer(app.GRPCQueryRouter(), k)
	require.NoError(t, app.LoadLatestVersion())
	header := tmproto.Header{ChainID: app.ChainID, Height: 1, Time: time.Unix(100, 0).UTC()}
	ms := app.CommitMultiStore().CacheMultiStore()
	ctx := sdk.NewContext(ms, header, false)
	pk := ed25519.GenPrivKeyFromSecret([]byte("repair 285 persisted validator")).PubKey()
	addr := sdk.ConsAddress(pk.Address())
	info := types.NewValidatorSigningInfo(addr, 17, 29, time.Unix(500, 0).UTC(), true, 7)
	k.SetValidatorSigningInfo(ctx, addr, info)
	otherPK := ed25519.GenPrivKeyFromSecret([]byte("repair 285 other validator")).PubKey()
	otherAddr := sdk.ConsAddress(otherPK.Address())
	k.SetValidatorSigningInfo(ctx, otherAddr, types.NewValidatorSigningInfo(otherAddr, 3, 5, time.Unix(0, 0).UTC(), false, 2))
	staged, found := k.GetValidatorSigningInfo(ctx, addr)
	require.True(t, found, "the signing record must exist before committing its block")
	require.Equal(t, info, staged)
	require.NoError(t, app.SetSealedStateToCommit(ctx, ms))
	_, err := app.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), app.LastBlockHeight())
	require.Equal(t, cdc.MustMarshal(&info), app.CommitMultiStore().GetKVStore(key).Get(types.ValidatorSigningInfoKey(addr)))
	queryCtx, err := app.CreateQueryContext(1, false)
	require.NoError(t, err)
	stored, found := k.GetValidatorSigningInfo(queryCtx, addr)
	require.True(t, found)
	require.Equal(t, info, stored)

	f := &repair285Fixture{amino: amino, queries: make(chan repair285Query, 8), pubkey: legacybech32.MustMarshalPubKey(legacybech32.ConsPK, pk), info: info}
	rpcMux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(rpcMux, map[string]*rpcserver.RPCFunc{
		"abci_query": rpcserver.NewRPCFunc(func(ctx context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
			query := abci.RequestQuery{Path: req.Path, Data: req.Data, Height: int64(req.Height), Prove: req.Prove}
			response, err := app.Query(ctx, &query)
			if err != nil {
				return nil, err
			}
			f.queries <- repair285Query{request: query, response: *response}
			return &coretypes.ResultABCIQuery{Response: *response}, nil
		}),
	})
	rpcServer := httptest.NewServer(rpcMux)
	t.Cleanup(rpcServer.Close)
	rpcClient, err := rpchttp.NewWithClient(rpcServer.URL, rpcServer.Client())
	require.NoError(t, err)
	f.client = client.Context{}.WithLegacyAmino(amino).WithCodec(cdc).WithInterfaceRegistry(registry).WithClient(rpcClient)
	router := mux.NewRouter()
	slashingrest.RegisterHandlers(f.client, router)
	f.server = httptest.NewServer(router)
	t.Cleanup(f.server.Close)
	return f
}

func (f *repair285Fixture) get(t *testing.T, publicKey, suffix string) (int, []byte) {
	t.Helper()
	resp, err := f.server.Client().Get(f.server.URL + "/slashing/validators/" + url.PathEscape(publicKey) + "/signing_info" + suffix)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, body
}

func (f *repair285Fixture) nextQuery(t *testing.T) repair285Query {
	t.Helper()
	select {
	case query := <-f.queries:
		return query
	case <-time.After(5 * time.Second):
		t.Fatal("registered REST route did not reach ABCI")
		return repair285Query{}
	}
}

func TestRepair_285_SigningInfoHTTPABCI(t *testing.T) {
	f := newRepair285Fixture(t)
	for _, suffix := range []string{"", "?height=0", "?height=1"} {
		t.Run(suffix, func(t *testing.T) {
			code, body := f.get(t, f.pubkey, suffix)
			query := f.nextQuery(t)
			require.Equal(t, "custom/"+types.QuerierRoute+"/"+types.QuerySigningInfo, query.request.Path)
			require.Equal(t, int64(1), query.request.Height)
			require.False(t, query.request.Prove)
			var request types.QuerySigningInfoRequest
			require.NoError(t, f.amino.UnmarshalAsJSON(query.request.Data, &request))
			require.Equal(t, f.info.Address, request.ConsAddress)
			decoded, err := sdk.ConsAddressFromBech32(request.ConsAddress)
			require.NoError(t, err)
			pk, err := legacybech32.UnmarshalPubKey(legacybech32.ConsPK, f.pubkey)
			require.NoError(t, err)
			require.Equal(t, []byte(pk.Address()), []byte(decoded))
			grpcResult, err := types.NewQueryClient(f.client).SigningInfo(context.Background(), &request)
			require.NoError(t, err)
			require.Equal(t, f.info, grpcResult.ValSigningInfo)
			grpcQuery := f.nextQuery(t)
			require.Equal(t, "/cosmos.slashing.v1beta1.Query/SigningInfo", grpcQuery.request.Path)
			require.Zero(t, grpcQuery.response.Code, grpcQuery.response.Log)
			require.Equal(t, uint32(0), query.response.Code, query.response.Log)
			require.Equal(t, int64(1), query.response.Height)
			require.Equal(t, http.StatusOK, code, string(body))
			var wrapped sdkrest.ResponseWithHeight
			require.NoError(t, f.amino.UnmarshalAsJSON(body, &wrapped))
			require.Equal(t, int64(1), wrapped.Height)
			var result types.ValidatorSigningInfo
			require.NoError(t, f.amino.UnmarshalAsJSON(wrapped.Result, &result))
			require.Equal(t, f.info, result)
			require.Equal(t, result, grpcResult.ValSigningInfo)
		})
	}
}

func TestRepair_285_UnknownValidator(t *testing.T) {
	f := newRepair285Fixture(t)
	pk := ed25519.GenPrivKeyFromSecret([]byte("repair 285 absent validator")).PubKey()
	addr := sdk.ConsAddress(pk.Address()).String()
	code, body := f.get(t, legacybech32.MustMarshalPubKey(legacybech32.ConsPK, pk), "?height=1")
	query := f.nextQuery(t)
	var request types.QuerySigningInfoRequest
	require.NoError(t, f.amino.UnmarshalAsJSON(query.request.Data, &request))
	require.Equal(t, addr, request.ConsAddress)
	require.Equal(t, types.ErrNoSigningInfoFound.Codespace(), query.response.Codespace)
	require.Equal(t, types.ErrNoSigningInfoFound.ABCICode(), query.response.Code)
	wantLog := sdkerrors.Wrap(types.ErrNoSigningInfoFound, addr).Error()
	require.Equal(t, wantLog, query.response.Log)
	require.Empty(t, query.response.Value)
	require.Equal(t, http.StatusInternalServerError, code)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result))
	require.Equal(t, map[string]interface{}{"error": status.Error(codes.Unknown, wantLog).Error()}, result)
}

func TestRepair_285_InvalidRequests(t *testing.T) {
	f := newRepair285Fixture(t)
	_, keyErr := legacybech32.UnmarshalPubKey(legacybech32.ConsPK, "invalid-public-key")
	require.Error(t, keyErr)
	for _, tc := range []struct{ name, key, suffix, message string }{
		{"invalid public key", "invalid-public-key", "", keyErr.Error()},
		{"negative height", f.pubkey, "?height=-1", "height must be equal or greater than zero"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, body := f.get(t, tc.key, tc.suffix)
			require.Equal(t, http.StatusBadRequest, code)
			var result map[string]interface{}
			require.NoError(t, json.Unmarshal(body, &result))
			require.Equal(t, map[string]interface{}{"error": tc.message}, result)
			require.Empty(t, f.queries, "invalid HTTP input must be rejected before ABCI")
		})
	}
}
