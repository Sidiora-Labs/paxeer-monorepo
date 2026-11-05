package feegrant_test

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/address"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/feegrant"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/feegrant/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repair307Keeper(t *testing.T) (keeper.Keeper, sdk.Context, *sdk.KVStoreKey) {
	t.Helper()
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	cms := store.NewCommitMultiStore(db)
	feeKey := sdk.NewKVStoreKey(feegrant.StoreKey)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	for _, key := range []*sdk.KVStoreKey{feeKey, authKey, paramsKey} {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	cms.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, db)
	require.NoError(t, cms.LoadLatestVersion())
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	feegrant.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	params := paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), paramsKey, transientKey, authtypes.ModuleName)
	ak := authkeeper.NewAccountKeeper(cdc, authKey, params, authtypes.ProtoBaseAccount, nil)
	ctx := sdk.NewContext(cms, tmproto.Header{Time: time.Unix(1700000000, 0).UTC()}, false)
	return keeper.NewKeeper(cdc, feeKey, ak), ctx, feeKey
}

func TestRepair_307_FeegrantKey(t *testing.T) {
	k, ctx, storeKey := repair307Keeper(t)
	lengths := []int{20, 32}
	grantees := make([]sdk.AccAddress, len(lengths))
	granters := make([]sdk.AccAddress, len(lengths))
	for i, length := range lengths {
		grantees[i] = sdk.AccAddress(bytes.Repeat([]byte{byte(i + 1)}, length))
		granters[i] = sdk.AccAddress(bytes.Repeat([]byte{byte(i + 11)}, length))
	}
	for _, granter := range granters {
		for _, grantee := range grantees {
			allowance := &feegrant.BasicAllowance{SpendLimit: sdk.NewCoins(sdk.NewInt64Coin("atom", int64(len(grantee))))}
			require.NoError(t, k.GrantAllowance(ctx, granter, grantee, allowance))
			key := feegrant.FeeAllowanceKey(granter, grantee)
			require.True(t, ctx.KVStore(storeKey).Has(key))
			decodedGranter, decodedGrantee := feegrant.ParseAddressesFromFeeAllowanceKey(key)
			require.Equal(t, granter, decodedGranter)
			require.Equal(t, grantee, decodedGrantee)
		}
	}
	for _, granter := range granters {
		t.Run(fmt.Sprintf("granter_length_%d", len(granter)), func(t *testing.T) {
			resp, err := k.AllowancesByGranter(sdk.WrapSDKContext(ctx), &feegrant.QueryAllowancesByGranterRequest{
				Granter: granter.String(), Pagination: &query.PageRequest{CountTotal: true},
			})
			require.NoError(t, err)
			require.Len(t, resp.Allowances, len(grantees))
			require.EqualValues(t, len(grantees), resp.Pagination.Total)
			seen := make(map[string]bool)
			for _, grant := range resp.Allowances {
				require.Equal(t, granter.String(), grant.Granter)
				require.False(t, seen[grant.Grantee])
				seen[grant.Grantee] = true
				grantee, err := sdk.AccAddressFromBech32(grant.Grantee)
				require.NoError(t, err)
				allowance, err := grant.GetGrant()
				require.NoError(t, err)
				require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin("atom", int64(len(grantee)))), allowance.(*feegrant.BasicAllowance).SpendLimit)
			}
			for _, grantee := range grantees {
				require.True(t, seen[grantee.String()])
			}
		})
	}
}

func TestRepair_307_FeegrantKeyLayout(t *testing.T) {
	for _, granterLen := range []int{20, 32, address.MaxAddrLen} {
		for _, granteeLen := range []int{20, 32, address.MaxAddrLen} {
			t.Run(fmt.Sprintf("round_trip_%d_%d", granterLen, granteeLen), func(t *testing.T) {
				granter := sdk.AccAddress(bytes.Repeat([]byte{1}, granterLen))
				grantee := sdk.AccAddress(bytes.Repeat([]byte{2}, granteeLen))
				key := feegrant.FeeAllowanceKey(granter, grantee)
				require.Len(t, key, 3+granterLen+granteeLen)
				decodedGranter, decodedGrantee := feegrant.ParseAddressesFromFeeAllowanceKey(key)
				require.Equal(t, granter, decodedGranter)
				require.Equal(t, grantee, decodedGrantee)
			})
		}
	}
	granter := sdk.AccAddress(bytes.Repeat([]byte{1}, 32))
	grantee := sdk.AccAddress(bytes.Repeat([]byte{2}, 20))
	key := feegrant.FeeAllowanceKey(granter, grantee)
	for length := 0; length < len(key); length++ {
		t.Run(fmt.Sprintf("truncated_%d", length), func(t *testing.T) {
			require.Panics(t, func() { feegrant.ParseAddressesFromFeeAllowanceKey(key[:length]) })
		})
	}
	wrongPrefix := append([]byte(nil), key...)
	wrongPrefix[0] = 1
	for name, malformed := range map[string][]byte{
		"wrong_prefix":   wrongPrefix,
		"trailing_bytes": append(append([]byte(nil), key...), 1),
		"empty_grantee":  {0, 0, 1, 1},
		"empty_granter":  {0, 1, 1, 0},
	} {
		t.Run(name, func(t *testing.T) {
			require.Panics(t, func() { feegrant.ParseAddressesFromFeeAllowanceKey(malformed) })
		})
	}
}

func TestRepair_307_FilteredRemove(t *testing.T) {
	for _, wrapped := range []bool{false, true} {
		for _, unlimited := range []bool{false, true} {
			t.Run(fmt.Sprintf("wrapped_%t_unlimited_%t", wrapped, unlimited), func(t *testing.T) {
				k, ctx, storeKey := repair307Keeper(t)
				granter := sdk.AccAddress(bytes.Repeat([]byte{1}, 20))
				grantee := sdk.AccAddress(bytes.Repeat([]byte{2}, 20))
				expiration := ctx.BlockTime().Add(time.Hour)
				basic := &feegrant.BasicAllowance{Expiration: &expiration}
				if !unlimited {
					basic.SpendLimit = sdk.NewCoins(sdk.NewInt64Coin("atom", 10))
				}
				var allowance feegrant.FeeAllowanceI = basic
				msg := &banktypes.MsgSend{FromAddress: granter.String(), ToAddress: grantee.String(), Amount: sdk.NewCoins(sdk.NewInt64Coin("atom", 1))}
				msgs := []sdk.Msg{msg}
				if wrapped {
					var err error
					allowance, err = feegrant.NewAllowedMsgAllowance(basic, []string{sdk.MsgTypeURL(msg)})
					require.NoError(t, err)
				}
				require.NoError(t, allowance.ValidateBasic())
				require.NoError(t, k.GrantAllowance(ctx, granter, grantee, allowance))
				fee := sdk.NewCoins(sdk.NewInt64Coin("atom", 1))
				key := feegrant.FeeAllowanceKey(granter, grantee)
				for _, blockTime := range []time.Time{ctx.BlockTime(), expiration} {
					ctx = ctx.WithBlockTime(blockTime)
					require.NoError(t, k.UseGrantedFees(ctx, granter, grantee, fee, msgs))
					require.True(t, ctx.KVStore(storeKey).Has(key))
				}
				stored, err := k.GetAllowance(ctx, granter, grantee)
				require.NoError(t, err)
				underlying := stored
				if wrapped {
					underlying, err = stored.(*feegrant.AllowedMsgAllowance).GetAllowance()
					require.NoError(t, err)
				}
				if !unlimited {
					require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin("atom", 8)), underlying.(*feegrant.BasicAllowance).SpendLimit)
				}
				ctx = ctx.WithBlockTime(expiration.Add(time.Nanosecond))
				remove, err := stored.Accept(ctx, fee, msgs)
				require.ErrorIs(t, err, feegrant.ErrFeeLimitExpired)
				require.True(t, remove)
				err = k.UseGrantedFees(ctx, granter, grantee, fee, msgs)
				require.ErrorIs(t, err, feegrant.ErrFeeLimitExpired)
				require.False(t, ctx.KVStore(storeKey).Has(key))
				stored, err = k.GetAllowance(ctx, granter, grantee)
				require.Nil(t, stored)
				require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
				err = k.UseGrantedFees(ctx, granter, grantee, fee, msgs)
				require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
				events := ctx.EventManager().Events()
				require.Equal(t, feegrant.EventTypeRevokeFeeGrant, events[len(events)-1].Type)
			})
		}
	}
}
