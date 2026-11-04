package keeper_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"

	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/types"
	commitmenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/23-commitment/types"
	ibctmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/light-clients/07-tendermint/types"
)

type repair337Fixture struct {
	client  types.QueryClient
	heights []types.Height
	states  map[types.Height]*ibctmtypes.ConsensusState
}

func newRepair337Fixture(t *testing.T) repair337Fixture {
	t.Helper()
	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	ibctmtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	key := sdk.NewKVStoreKey("repair337")
	tkey := sdk.NewTransientStoreKey("repair337_transient")
	ctx := testutil.DefaultContext(key, tkey).
		WithChainID("repair-337").WithBlockHeight(20).
		WithBlockTime(time.Unix(1700000000, 0).UTC())
	subspace := paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), key, tkey, "client")
	k := keeper.NewKeeper(cdc, key, subspace, nil, nil)
	f := repair337Fixture{
		heights: []types.Height{types.NewHeight(0, 1), types.NewHeight(0, 10), types.NewHeight(0, 2), types.NewHeight(1, 1), types.NewHeight(1, 2)},
		states:  make(map[types.Height]*ibctmtypes.ConsensusState),
	}
	// Store order is lexicographic, including across revision boundaries.
	for i := len(f.heights) - 1; i >= 0; i-- {
		h := f.heights[i]
		state := ibctmtypes.NewConsensusState(ctx.BlockTime().Add(time.Duration(i)*time.Second), commitmenttypes.NewMerkleRoot([]byte{byte(i + 1)}), make([]byte, 32))
		k.SetClientConsensusState(ctx, testClientID, h, state)
		clientStore := k.ClientStore(ctx, testClientID)
		ibctmtypes.SetProcessedTime(clientStore, h, uint64(ctx.BlockTime().UnixNano()))
		ibctmtypes.SetProcessedHeight(clientStore, h, types.GetSelfHeight(ctx))
		ibctmtypes.SetIterationKey(clientStore, h)
		stored, found := k.GetClientConsensusState(ctx, testClientID, h)
		require.True(t, found)
		require.Equal(t, state, stored)
		f.states[h] = state
	}
	k.SetClientConsensusState(ctx, testClientID2, types.NewHeight(9, 9), f.states[f.heights[0]])
	router := baseapp.NewQueryServerTestHelper(ctx, registry)
	types.RegisterQueryServer(router, k)
	f.client = types.NewQueryClient(router)
	return f
}

type repair337Query func(*testing.T, *query.PageRequest) ([]types.Height, *query.PageResponse)

func TestRepair_337_ConsensusStatesPagination(t *testing.T) {
	f := newRepair337Fixture(t)
	repair337CheckPages(t, f.heights, func(t *testing.T, page *query.PageRequest) ([]types.Height, *query.PageResponse) {
		res, err := f.client.ConsensusStates(context.Background(), &types.QueryConsensusStatesRequest{ClientId: testClientID, Pagination: page})
		require.NoError(t, err)
		heights := make([]types.Height, 0, len(res.ConsensusStates))
		for _, entry := range res.ConsensusStates {
			state, err := types.UnpackConsensusState(entry.ConsensusState)
			require.NoError(t, err)
			require.Equal(t, f.states[entry.Height], state, "stored payload at %s", entry.Height)
			heights = append(heights, entry.Height)
		}
		return heights, res.Pagination
	})
}

func TestRepair_337_ConsensusHeightsPagination(t *testing.T) {
	f := newRepair337Fixture(t)
	t.Run("registered_query", func(t *testing.T) {
		repair337CheckPages(t, f.heights, func(t *testing.T, page *query.PageRequest) ([]types.Height, *query.PageResponse) {
			res, err := f.client.ConsensusStateHeights(context.Background(), &types.QueryConsensusStateHeightsRequest{ClientId: testClientID, Pagination: page})
			require.NoError(t, err)
			return res.ConsensusStateHeights, res.Pagination
		})
	})
	t.Run("gateway", func(t *testing.T) {
		mux := runtime.NewServeMux()
		require.NoError(t, types.RegisterQueryHandlerClient(context.Background(), mux, f.client))
		repair337CheckPages(t, f.heights, func(t *testing.T, page *query.PageRequest) ([]types.Height, *query.PageResponse) {
			params := url.Values{}
			if page != nil {
				params.Set("pagination.offset", strconv.FormatUint(page.Offset, 10))
				params.Set("pagination.limit", strconv.FormatUint(page.Limit, 10))
				params.Set("pagination.count_total", strconv.FormatBool(page.CountTotal))
				params.Set("pagination.reverse", strconv.FormatBool(page.Reverse))
				if len(page.Key) != 0 {
					params.Set("pagination.key", base64.StdEncoding.EncodeToString(page.Key))
				}
			}
			req := httptest.NewRequest(http.MethodGet, "/ibc/core/client/v1/consensus_states/"+testClientID+"/heights?"+params.Encode(), nil)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, req)
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			var res types.QueryConsensusStateHeightsResponse
			require.NoError(t, (&runtime.JSONPb{}).Unmarshal(response.Body.Bytes(), &res))
			return res.ConsensusStateHeights, res.Pagination
		})
	})
}

func repair337CheckPages(t *testing.T, ordered []types.Height, fetch repair337Query) {
	t.Helper()
	t.Run("default_page", func(t *testing.T) {
		for _, page := range []*query.PageRequest{nil, {}} {
			got, pagination := fetch(t, page)
			require.Equal(t, ordered, got)
			require.NotNil(t, pagination)
			require.Empty(t, pagination.NextKey)
			require.Zero(t, pagination.Total)
		}
	})
	for _, reverse := range []bool{false, true} {
		t.Run("reverse_"+strconv.FormatBool(reverse), func(t *testing.T) {
			want := append([]types.Height(nil), ordered...)
			if reverse {
				for i, j := 0, len(want)-1; i < j; i, j = i+1, j-1 {
					want[i], want[j] = want[j], want[i]
				}
			}
			for _, countTotal := range []bool{false, true} {
				t.Run("count_total_"+strconv.FormatBool(countTotal), func(t *testing.T) {
					for _, offset := range []uint64{0, 1, 3, 5, 7} {
						t.Run("offset_"+strconv.FormatUint(offset, 10), func(t *testing.T) {
							page := &query.PageRequest{Offset: offset, Limit: 2, Reverse: reverse, CountTotal: countTotal}
							got, pagination := fetch(t, page)
							start := min(int(offset), len(want))
							end := min(start+2, len(want))
							require.Equal(t, want[start:end], append([]types.Height{}, got...))
							require.LessOrEqual(t, uint64(len(got)), page.Limit)
							require.NotNil(t, pagination)
							if countTotal {
								require.Equal(t, uint64(len(want)), pagination.Total)
							} else {
								require.Zero(t, pagination.Total)
							}
							if end < len(want) {
								require.Equal(t, []byte(want[end].String()), pagination.NextKey)
								next, _ := fetch(t, &query.PageRequest{Key: pagination.NextKey, Limit: 2, Reverse: reverse})
								require.Equal(t, want[end:min(end+2, len(want))], next)
							} else {
								require.Empty(t, pagination.NextKey)
							}
						})
					}
				})
			}
			t.Run("key_continuation", func(t *testing.T) {
				var key []byte
				seen := make(map[types.Height]bool)
				var all []types.Height
				for pageIndex := 0; pageIndex <= len(want); pageIndex++ {
					got, pagination := fetch(t, &query.PageRequest{Key: key, Limit: 2, Reverse: reverse, CountTotal: true})
					start := len(all)
					require.Equal(t, want[start:min(start+2, len(want))], got)
					require.LessOrEqual(t, len(got), 2)
					for _, h := range got {
						require.False(t, seen[h], "duplicate height %s", h)
						seen[h] = true
					}
					all = append(all, got...)
					require.NotNil(t, pagination)
					if len(key) == 0 {
						require.Equal(t, uint64(len(want)), pagination.Total)
					} else {
						require.Zero(t, pagination.Total)
					}
					if len(all) < len(want) {
						require.Equal(t, []byte(want[len(all)].String()), pagination.NextKey)
					} else {
						require.Empty(t, pagination.NextKey)
					}
					if len(pagination.NextKey) == 0 {
						require.Equal(t, want, all)
						return
					}
					require.NotEqual(t, key, pagination.NextKey)
					key = append([]byte(nil), pagination.NextKey...)
				}
				t.Fatal("pagination did not terminate")
			})
		})
	}
}
