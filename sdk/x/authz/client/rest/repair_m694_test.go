package rest_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/gogo/gateway"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type repair694Grant struct {
	granter       sdk.AccAddress
	grantee       sdk.AccAddress
	authorization authz.Authorization
}

type repair694Fixture struct {
	cdc        codec.Codec
	server     *httptest.Server
	addresses  []sdk.AccAddress
	grants     []repair694Grant
	expiration time.Time
}

func repair694NewFixture(t *testing.T) *repair694Fixture {
	t.Helper()
	s := new(IntegrationTestSuite)
	s.SetT(t)
	cfg := s.defaultConfig()
	require.NotNil(t, cfg.Codec)
	require.NotNil(t, cfg.InterfaceRegistry)
	require.NotNil(t, cfg.AppConstructor)
	require.NotNil(t, cfg.TxConfig)
	require.NotEmpty(t, cfg.GenesisState[authz.ModuleName])

	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	key := sdk.NewKVStoreKey(authz.ModuleName)
	cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	require.NoError(t, cms.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, cms.Close()) })
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 1, Time: now}, false)
	k := keeper.NewKeeper(key, cfg.Codec, baseapp.NewMsgServiceRouter())
	f := &repair694Fixture{cdc: cfg.Codec, expiration: now.Add(time.Hour)}
	keys := keyring.NewInMemory()
	for i := 0; i < 4; i++ {
		info, _, err := keys.NewMnemonic(fmt.Sprintf("repair694-%d", i), keyring.English, sdk.FullFundraiserPath, keyring.DefaultBIP39Passphrase, hd.Secp256k1)
		require.NoError(t, err)
		address := info.GetAddress()
		decoded, err := sdk.AccAddressFromBech32(address.String())
		require.NoError(t, err)
		require.Equal(t, address, decoded)
		f.addresses = append(f.addresses, address)
	}
	a, b, c := f.addresses[0], f.addresses[1], f.addresses[2]
	send := func(amount int64) authz.Authorization {
		return banktypes.NewSendAuthorization(sdk.NewCoins(sdk.NewInt64Coin(cfg.BondDenom, amount)))
	}
	f.grants = []repair694Grant{
		{a, b, send(111)},
		{a, c, authz.NewGenericAuthorization(typeMsgVote)},
		{c, b, send(222)},
		{b, c, send(333)},
	}
	for _, grant := range f.grants {
		require.NoError(t, k.SaveGrant(ctx, grant.grantee, grant.granter, grant.authorization, f.expiration))
		stored, expiration := k.GetCleanAuthorization(ctx, grant.grantee, grant.granter, grant.authorization.MsgTypeURL())
		require.Equal(t, grant.authorization, stored)
		require.Equal(t, f.expiration, expiration)
	}
	// The committed SDK query helper performs the production request/response
	// protobuf encoding and invokes the registered keeper through its real router.
	queries := baseapp.NewQueryServerTestHelper(ctx, cfg.InterfaceRegistry)
	authz.RegisterQueryServer(queries, k)
	mux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &gateway.JSONPb{
			EmitDefaults: true, OrigName: true, AnyResolver: cfg.InterfaceRegistry,
		}),
		runtime.WithProtoErrorHandler(runtime.DefaultHTTPProtoErrorHandler),
	)
	require.NoError(t, authz.RegisterQueryHandlerClient(context.Background(), mux, authz.NewQueryClient(queries)))
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func (f *repair694Fixture) get(t *testing.T, path string, status int) []byte {
	t.Helper()
	resp, err := f.server.Client().Get(f.server.URL + path)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, status, resp.StatusCode, "%s: %s", path, body)
	return body
}

func (f *repair694Fixture) assertGrants(t *testing.T, actual []*authz.GrantAuthorization, indices ...int) {
	t.Helper()
	require.Len(t, actual, len(indices))
	want := make([]string, 0, len(indices))
	for _, i := range indices {
		grant := f.grants[i]
		encoded, err := f.cdc.MarshalInterfaceJSON(grant.authorization)
		require.NoError(t, err)
		want = append(want, grant.granter.String()+"/"+grant.grantee.String()+"/"+string(encoded))
	}
	got := make([]string, 0, len(actual))
	for _, grant := range actual {
		require.Equal(t, f.expiration, grant.Expiration)
		var authorization authz.Authorization
		require.NoError(t, f.cdc.UnpackAny(grant.Authorization, &authorization))
		encoded, err := f.cdc.MarshalInterfaceJSON(authorization)
		require.NoError(t, err)
		got = append(got, grant.Granter+"/"+grant.Grantee+"/"+string(encoded))
	}
	require.ElementsMatch(t, want, got)
}

func TestRepairM_694_Config(t *testing.T) {
	f := repair694NewFixture(t)
	grant := f.grants[0]
	query := url.Values{"granter": {grant.granter.String()}, "grantee": {grant.grantee.String()}, "msg_type_url": {grant.authorization.MsgTypeURL()}}
	body := f.get(t, "/cosmos/authz/v1beta1/grants?"+query.Encode(), http.StatusOK)
	var response authz.QueryGrantsResponse
	require.NoError(t, f.cdc.UnmarshalAsJSON(body, &response))
	require.Len(t, response.Grants, 1)
	require.NoError(t, response.Grants[0].UnpackInterfaces(f.cdc))
	require.Equal(t, grant.authorization, response.Grants[0].GetAuthorization())
	require.Equal(t, f.expiration, response.Grants[0].Expiration)
}

func TestRepairM_694_AddressFixture(t *testing.T) {
	f := repair694NewFixture(t)
	for _, tc := range []struct {
		name    string
		route   string
		account int
		indices []int
	}{
		{"granter A", "granter", 0, []int{0, 1}},
		{"granter B", "granter", 1, []int{3}},
		{"granter C", "granter", 2, []int{2}},
		{"granter without grants", "granter", 3, nil},
		{"grantee A", "grantee", 0, nil},
		{"grantee B", "grantee", 1, []int{0, 2}},
		{"grantee C", "grantee", 2, []int{1, 3}},
		{"grantee without grants", "grantee", 3, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := "/cosmos/authz/v1beta1/grants/" + tc.route + "/" + f.addresses[tc.account].String()
			body := f.get(t, path, http.StatusOK)
			var grants []*authz.GrantAuthorization
			if tc.route == "granter" {
				var response authz.QueryGranterGrantsResponse
				require.NoError(t, f.cdc.UnmarshalAsJSON(body, &response))
				grants = response.Grants
			} else {
				var response authz.QueryGranteeGrantsResponse
				require.NoError(t, f.cdc.UnmarshalAsJSON(body, &response))
				grants = response.Grants
			}
			f.assertGrants(t, grants, tc.indices...)
		})
	}
	for _, route := range []string{"granter", "grantee"} {
		t.Run(route+" pagination", func(t *testing.T) {
			account := 0
			indices := []int{0, 1}
			if route == "grantee" {
				account = 1
				indices = []int{0, 2}
			}
			path := "/cosmos/authz/v1beta1/grants/" + route + "/" + f.addresses[account].String()
			var all []*authz.GrantAuthorization
			next := ""
			for page := 0; page < len(f.grants)+1; page++ {
				query := url.Values{"pagination.limit": {"1"}, "pagination.key": {next}}
				body := f.get(t, path+"?"+query.Encode(), http.StatusOK)
				var grants []*authz.GrantAuthorization
				if route == "granter" {
					var response authz.QueryGranterGrantsResponse
					require.NoError(t, f.cdc.UnmarshalAsJSON(body, &response))
					require.NotNil(t, response.Pagination)
					grants = response.Grants
					next = base64.StdEncoding.EncodeToString(response.Pagination.NextKey)
				} else {
					var response authz.QueryGranteeGrantsResponse
					require.NoError(t, f.cdc.UnmarshalAsJSON(body, &response))
					require.NotNil(t, response.Pagination)
					grants = response.Grants
					next = base64.StdEncoding.EncodeToString(response.Pagination.NextKey)
				}
				if len(all) < len(indices) {
					require.Len(t, grants, 1)
				} else {
					require.Empty(t, grants)
				}
				all = append(all, grants...)
				if page == 0 {
					require.NotEmpty(t, next)
				}
				if next == "" {
					break
				}
			}
			require.Empty(t, next, "pagination must terminate within the persisted grant count")
			f.assertGrants(t, all, indices...)
		})
		for _, invalid := range []string{"invalid address", f.addresses[1].String()[1:2], f.addresses[1].String()[:8]} {
			t.Run(route+" invalid "+invalid, func(t *testing.T) {
				resp, err := f.server.Client().Get(f.server.URL + "/cosmos/authz/v1beta1/grants/" + route + "/" + url.PathEscape(invalid))
				require.NoError(t, err)
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.GreaterOrEqual(t, resp.StatusCode, 400)
				require.Contains(t, string(body), "decoding bech32 failed")
			})
		}
	}
}
