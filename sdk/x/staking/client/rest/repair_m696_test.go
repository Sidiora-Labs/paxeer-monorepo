package rest_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/api"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// The norace suite imports the native node. Check its constructor call without
// linking that node into this service-free gate; the HTTP proof below runs the
// production gateway, keeper, store and codec.
func TestRepairM_696_StakingRESTConfig(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(filepath.Dir(filename), "grpc_query_test.go"), nil, 0)
	require.NoError(t, err)
	var setup *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "SetupSuite" {
			setup = fn
		}
	}
	require.NotNil(t, setup)
	calls := 0
	ast.Inspect(setup.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "DefaultConfig" {
			return true
		}
		pkg, ok := selector.X.(*ast.Ident)
		require.True(t, ok)
		require.Equal(t, "network", pkg.Name)
		require.Len(t, call.Args, 1, "network.DefaultConfig must receive the suite's current testing context")
		arg, ok := call.Args[0].(*ast.CallExpr)
		require.True(t, ok)
		require.Empty(t, arg.Args)
		currentT, ok := arg.Fun.(*ast.SelectorExpr)
		require.True(t, ok)
		require.Equal(t, "T", currentT.Sel.Name)
		receiver, ok := currentT.X.(*ast.Ident)
		require.True(t, ok)
		require.Equal(t, setup.Recv.List[0].Names[0].Name, receiver.Name)
		calls++
		return true
	})
	require.Equal(t, 1, calls)
}

func TestRepairM_696_RedelegationFilterPages(t *testing.T) {
	testRedelegationFilterPages(t)
}

type redelegationRESTFixture struct {
	server     *httptest.Server
	cdc        *codec.ProtoCodec
	delegator  sdk.AccAddress
	validators []sdk.ValAddress
	records    []types.Redelegation
	responses  []types.RedelegationResponse
}

func newRedelegationRESTFixture(t *testing.T) redelegationRESTFixture {
	t.Helper()
	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	stakingKey := sdk.NewKVStoreKey(types.StoreKey)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	bankKey := sdk.NewKVStoreKey(banktypes.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	cms := store.NewCommitMultiStore(db)
	for _, key := range []*sdk.KVStoreKey{stakingKey, authKey, bankKey, paramsKey} {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	cms.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, db)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 7, Time: time.Unix(100, 0).UTC()}, false)
	subspace := func(name string) paramtypes.Subspace {
		return paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, name)
	}
	ak := authkeeper.NewAccountKeeper(cdc, authKey, subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount,
		map[string][]string{types.BondedPoolName: {authtypes.Burner, authtypes.Staking}, types.NotBondedPoolName: {authtypes.Burner, authtypes.Staking}})
	bk := bankkeeper.NewBaseKeeper(cdc, bankKey, ak, subspace(banktypes.ModuleName), map[string]bool{})
	k := keeper.NewKeeper(cdc, stakingKey, ak, bk, subspace(types.ModuleName))
	k.SetParams(ctx, types.DefaultParams())
	fixture := redelegationRESTFixture{cdc: cdc, delegator: sdk.AccAddress(ed25519.GenPrivKeyFromSecret([]byte("rest redelegator")).PubKey().Address())}
	for i := 0; i < 4; i++ {
		pk := ed25519.GenPrivKeyFromSecret([]byte(fmt.Sprintf("rest validator %d", i))).PubKey()
		address := sdk.ValAddress(pk.Address())
		validator, err := types.NewValidator(address, pk, types.Description{})
		require.NoError(t, err)
		validator.Tokens = sdk.NewInt(200)
		validator.DelegatorShares = sdk.NewDec(100)
		k.SetValidator(ctx, validator)
		fixture.validators = append(fixture.validators, address)
	}
	sort.Slice(fixture.validators, func(i, j int) bool { return bytes.Compare(fixture.validators[i], fixture.validators[j]) < 0 })
	// Both sources share destinations, with nonmatching records between and
	// after matches. Dropping either HTTP filter must change the exact result.
	for i, pair := range [][2]int{{0, 1}, {0, 2}, {0, 3}, {1, 0}, {1, 2}, {1, 3}} {
		src, dst := fixture.validators[pair[0]], fixture.validators[pair[1]]
		shares := sdk.NewDec(int64(i + 1))
		balance := sdk.NewInt(int64(i + 11))
		completion := ctx.BlockTime().Add(time.Hour)
		record := types.NewRedelegation(fixture.delegator, src, dst, ctx.BlockHeight(), completion, balance, shares)
		k.SetRedelegation(ctx, record)
		stored, found := k.GetRedelegation(ctx, fixture.delegator, src, dst)
		require.True(t, found)
		require.Equal(t, record, stored)
		fixture.records = append(fixture.records, record)
		fixture.responses = append(fixture.responses, types.NewRedelegationResponse(fixture.delegator, src, dst,
			[]types.RedelegationEntryResponse{types.NewRedelegationEntryResponse(ctx.BlockHeight(), completion, shares, balance, shares.MulInt64(2).TruncateInt())}))
	}
	other := sdk.AccAddress(ed25519.GenPrivKeyFromSecret([]byte("other rest redelegator")).PubKey().Address())
	k.SetRedelegation(ctx, types.NewRedelegation(other, fixture.validators[0], fixture.validators[2], 7,
		ctx.BlockTime().Add(time.Hour), sdk.NewInt(99), sdk.NewDec(99)))
	apiServer := api.New(client.Context{}.WithCodec(cdc).WithInterfaceRegistry(registry))
	require.NoError(t, types.RegisterQueryHandlerServer(sdk.WrapSDKContext(ctx), apiServer.GRPCGatewayRouter, keeper.Querier{Keeper: k}))
	apiServer.Router.PathPrefix("/").Handler(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		apiServer.GRPCGatewayRouter.ServeHTTP(w, req.WithContext(sdk.WrapSDKContext(ctx)))
	}))
	fixture.server = httptest.NewServer(apiServer.Router)
	t.Cleanup(fixture.server.Close)
	return fixture
}

func (f redelegationRESTFixture) get(t *testing.T, delegator string, params url.Values) (int, []byte) {
	t.Helper()
	response, err := f.server.Client().Get(f.server.URL + "/cosmos/staking/v1beta1/delegators/" + delegator + "/redelegations?" + params.Encode())
	require.NoError(t, err)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return response.StatusCode, body
}

func testRedelegationFilterPages(t *testing.T) {
	t.Helper()
	f := newRedelegationRESTFixture(t)
	for _, tc := range []struct {
		name    string
		src     string
		dst     string
		indices []int
	}{
		{"unfiltered", "", "", []int{0, 1, 2, 3, 4, 5}},
		{"source", f.validators[0].String(), "", []int{0, 1, 2}},
		{"destination", "", f.validators[2].String(), []int{1, 4}},
		{"source_and_destination", f.validators[0].String(), f.validators[2].String(), []int{1}},
		{"other_source", f.validators[1].String(), "", []int{3, 4, 5}},
		{"other_destination", "", f.validators[3].String(), []int{2, 5}},
		{"other_pair", f.validators[1].String(), f.validators[3].String(), []int{5}},
		{"empty_source", f.validators[3].String(), "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, reverse := range []bool{false, true} {
				indices := append([]int(nil), tc.indices...)
				if reverse {
					for i, j := 0, len(indices)-1; i < j; i, j = i+1, j-1 {
						indices[i], indices[j] = indices[j], indices[i]
					}
				}
				for _, mode := range []string{"key", "offset"} {
					for _, limit := range []int{1, 2, 3} {
						t.Run(fmt.Sprintf("%s/limit_%d/reverse_%t", mode, limit, reverse), func(t *testing.T) {
							var key []byte
							for start := 0; ; start += limit {
								params := url.Values{"pagination.limit": {strconv.Itoa(limit)}, "pagination.count_total": {"true"}, "pagination.reverse": {strconv.FormatBool(reverse)}}
								if tc.src != "" {
									params.Set("src_validator_addr", tc.src)
								}
								if tc.dst != "" {
									params.Set("dst_validator_addr", tc.dst)
								}
								if mode == "offset" {
									params.Set("pagination.offset", strconv.Itoa(start))
								} else if len(key) > 0 {
									params.Set("pagination.key", base64.StdEncoding.EncodeToString(key))
								}
								code, body := f.get(t, f.delegator.String(), params)
								require.Equal(t, http.StatusOK, code, string(body))
								var got types.QueryRedelegationsResponse
								require.NoError(t, f.cdc.UnmarshalAsJSON(body, &got))
								end := start + limit
								if end > len(indices) {
									end = len(indices)
								}
								want := make([]types.RedelegationResponse, 0, end-start)
								for _, index := range indices[start:end] {
									want = append(want, f.responses[index])
								}
								require.Equal(t, want, got.RedelegationResponses, "every page must contain exactly the matching records and entry balances")
								require.NotNil(t, got.Pagination)
								total := uint64(len(indices))
								if mode == "key" && start > 0 {
									total = 0
								}
								require.Equal(t, total, got.Pagination.Total)
								if end == len(indices) {
									require.Empty(t, got.Pagination.NextKey, "exhausted filters must not expose nonmatching continuation pages")
									break
								}
								next := f.records[indices[end]]
								src, err := sdk.ValAddressFromBech32(next.ValidatorSrcAddress)
								require.NoError(t, err)
								dst, err := sdk.ValAddressFromBech32(next.ValidatorDstAddress)
								require.NoError(t, err)
								storeKey := types.GetREDKey(f.delegator, src, dst)
								wantKey := storeKey[len(types.GetREDsKey(f.delegator)):]
								require.Equal(t, wantKey, got.Pagination.NextKey, "continuation must identify the next matching record")
								key = got.Pagination.NextKey
							}
						})
					}
				}
			}
		})
	}
}

func TestRepairM_696_RedelegationInvalidFilters(t *testing.T) {
	f := newRedelegationRESTFixture(t)
	for _, tc := range []struct {
		name, delegator string
		params          url.Values
	}{
		{"invalid_delegator", "wrongValAddress", url.Values{}},
		{"missing_delegator", "", url.Values{}},
		{"invalid_source", f.delegator.String(), url.Values{"src_validator_addr": {"wrongValAddress"}}},
		{"invalid_destination", f.delegator.String(), url.Values{"dst_validator_addr": {"wrongValAddress"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, body := f.get(t, tc.delegator, tc.params)
			require.GreaterOrEqual(t, code, http.StatusBadRequest, string(body))
			var got types.QueryRedelegationsResponse
			require.Error(t, f.cdc.UnmarshalAsJSON(body, &got))
		})
	}
}
