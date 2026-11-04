package query_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/apptesting"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/prefix"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
)

func TestRepairM_649_Paginate(t *testing.T) {
	repairM649StorePages(t, func(store storetypes.KVStore, request *query.PageRequest, onResult func([]byte, []byte) error) (*query.PageResponse, error) {
		return query.Paginate(store, request, onResult)
	})
}

func TestRepairM_649_FilteredPaginate(t *testing.T) {
	repairM649StorePages(t, func(store storetypes.KVStore, request *query.PageRequest, onResult func([]byte, []byte) error) (*query.PageResponse, error) {
		return query.FilteredPaginate(store, request, func(key, value []byte, accumulate bool) (bool, error) {
			if !accumulate {
				return true, nil
			}
			return true, onResult(key, value)
		})
	})
}

func repairM649StorePages(t *testing.T, paginate func(storetypes.KVStore, *query.PageRequest, func([]byte, []byte) error) (*query.PageResponse, error)) {
	t.Helper()
	all := []string{"\xff\xff", "\xff", "e", "c", "a\x01", "a\x00", "a"}
	cases := []struct {
		name string
		key  []byte
		want []string
	}{
		{"largest", []byte("\xff\xff"), all},
		{"exact", []byte("c"), all[3:]},
		{"missing_between", []byte("d"), all[3:]},
		{"missing_below_largest", []byte("\xff\xfe"), all[1:]},
		{"beyond_range", []byte("\xff\xff\x00"), all},
		{"below_range", []byte("0"), []string{}},
		{"key_with_zero_suffix", []byte("a\x00"), all[5:]},
		{"key_before_zero_suffix", []byte("a"), all[6:]},
		{"nil_key", nil, all},
		{"empty_key", []byte{}, all},
	}
	for _, wrapped := range []bool{false, true} {
		for _, empty := range []bool{false, true} {
			for _, tc := range cases {
				name := tc.name
				if wrapped {
					name += "/prefix"
				}
				if empty {
					name += "/empty_store"
				}
				t.Run(name, func(t *testing.T) {
					db := dbm.NewMemDB()
					t.Cleanup(func() { require.NoError(t, db.Close()) })
					parent := dbadapter.Store{DB: db}
					var store storetypes.KVStore = parent
					if wrapped {
						store = prefix.NewStore(parent, []byte{0x42})
						parent.Set([]byte{0x41}, []byte("outside prefix"))
						parent.Set([]byte{0x43}, []byte("outside prefix"))
					}
					if !empty {
						for _, key := range all {
							store.Set([]byte(key), []byte("value:"+key))
						}
					}
					want := tc.want
					if empty {
						want = []string{}
					}
					// Spare capacity must remain untouched when constructing the bound.
					var buffer, key []byte
					if tc.key != nil {
						buffer = bytes.Repeat([]byte{0x7b}, len(tc.key)+3)
						copy(buffer, tc.key)
						key = buffer[:len(tc.key)]
					}
					original := bytes.Clone(buffer)
					request := &query.PageRequest{Key: key, Reverse: true, Limit: 1}
					got := []string{}
					for page := 0; ; page++ {
						require.LessOrEqual(t, page, len(want), "continuations must make progress")
						before := len(got)
						response, err := paginate(store, request, func(key, value []byte) error {
							require.Equal(t, "value:"+string(key), string(value))
							got = append(got, string(key))
							return nil
						})
						require.NoError(t, err)
						require.NotNil(t, response)
						require.Equal(t, original, buffer, "the caller's key and spare capacity must not change")
						require.LessOrEqual(t, len(got)-before, 1)
						require.Zero(t, response.Total)
						if len(got) < len(want) {
							require.Equal(t, []byte(want[len(got)]), response.NextKey)
						} else {
							require.Empty(t, response.NextKey)
						}
						if len(response.NextKey) == 0 {
							break
						}
						request = &query.PageRequest{Key: response.NextKey, Reverse: true, Limit: 1}
					}
					require.Equal(t, want, got, "resume keys are inclusive and missing keys cannot admit greater records")
				})
			}
		}
	}
}

func TestRepairM_649_BankQuery(t *testing.T) {
	app, ctx, _ := setupTest(t)
	addr := sdk.AccAddress(bytes.Repeat([]byte{0x64}, 20))
	emptyAddr := sdk.AccAddress(bytes.Repeat([]byte{0x65}, 20))
	account := app.AccountKeeper.NewAccountWithAddress(ctx, addr)
	app.AccountKeeper.SetAccount(ctx, account)
	balances := sdk.NewCoins(
		sdk.NewInt64Coin("aaa", 11),
		sdk.NewInt64Coin("aaa0", 22),
		sdk.NewInt64Coin("ccc", 33),
		sdk.NewInt64Coin("eee", 44),
	)
	require.NoError(t, apptesting.FundAccount(app.BankKeeper, ctx, addr, balances))
	router := baseapp.NewQueryServerTestHelper(ctx, app.InterfaceRegistry())
	banktypes.RegisterQueryServer(router, app.BankKeeper)
	client := banktypes.NewQueryClient(router)
	all := []string{"eee", "ccc", "aaa0", "aaa"}
	cases := []struct {
		name string
		key  []byte
		want []string
	}{
		{"largest", []byte("eee"), all},
		{"exact", []byte("ccc"), all[1:]},
		{"missing_between", []byte("bbb"), all[2:]},
		{"missing_below_largest", []byte("ddd"), all[1:]},
		{"beyond_range", []byte("zzz"), all},
		{"below_range", []byte("000"), []string{}},
		{"before_longer_key", []byte("aaa"), all[3:]},
		{"nil_key", nil, all},
	}
	for _, empty := range []bool{false, true} {
		for _, tc := range cases {
			name := tc.name
			address, want := addr, tc.want
			if empty {
				name += "/empty_store"
				address, want = emptyAddr, []string{}
			}
			t.Run(name, func(t *testing.T) {
				request := &query.PageRequest{Key: tc.key, Reverse: true, Limit: 1}
				got := []string{}
				for page := 0; ; page++ {
					require.LessOrEqual(t, page, len(want), "bank query continuations must make progress")
					response, err := client.AllBalances(context.Background(), banktypes.NewQueryAllBalancesRequest(address, request))
					require.NoError(t, err)
					require.NotNil(t, response.Pagination)
					require.LessOrEqual(t, len(response.Balances), 1)
					for _, coin := range response.Balances {
						require.Equal(t, balances.AmountOf(coin.Denom), coin.Amount)
						got = append(got, coin.Denom)
					}
					require.Zero(t, response.Pagination.Total)
					if len(got) < len(want) {
						require.Equal(t, []byte(want[len(got)]), response.Pagination.NextKey)
					} else {
						require.Empty(t, response.Pagination.NextKey)
					}
					if len(response.Pagination.NextKey) == 0 {
						break
					}
					request = &query.PageRequest{Key: response.Pagination.NextKey, Reverse: true, Limit: 1}
				}
				require.Equal(t, want, got, "bank balances survive production encoding, query routing and response decoding in reverse order")
			})
		}
	}
}
