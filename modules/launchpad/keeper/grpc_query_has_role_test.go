package keeper_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestQueryHasRoleServes(t *testing.T) {
	e := newEnv(t)
	creator := e.account(10_000_000)
	market := e.create(creator, types.FeeStrategyClaim)
	contract := e.k.Generation2().FeeAccumulator
	pool := common.HexToAddress(market.PoolAddress)
	require.True(t, e.k.HasRole(e.ctx, contract, keeper.PoolRole, pool))
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, keeper.NewQuerier(e.k))
	client := types.NewQueryClient(router)

	cases := []struct {
		name     string
		contract common.Address
		role     common.Hash
		account  common.Address
		want     bool
	}{
		{"created pool", contract, keeper.PoolRole, pool, true},
		{"creator has no pool role", contract, keeper.PoolRole, e.evmAddress(creator), false},
		{"other contract", e.k.Generation2().SidioraNFT, keeper.PoolRole, pool, false},
		{"other role", contract, common.HexToHash("0x01"), pool, false},
		{"default admin role", contract, common.Hash(keeper.DefaultAdminRole), pool, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response, err := client.HasRole(context.Background(), &types.QueryHasRoleRequest{
				Contract: tc.contract.Hex(), Role: tc.role.Hex(), Account: tc.account.Hex(),
			})
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Equal(t, tc.want, response.HasRole)
			require.Equal(t, e.k.HasRole(e.ctx, tc.contract, tc.role, tc.account), response.HasRole)
		})
	}

	request := &types.QueryHasRoleRequest{
		Contract: contract.Hex(), Role: "0x" + strings.ToUpper(keeper.PoolRole.Hex()[2:]), Account: pool.Hex(),
	}
	response, err := client.HasRole(context.Background(), request)
	require.NoError(t, err)
	require.True(t, response.HasRole)
	revoked, err := e.k.GovernanceRevoke(e.ctx, e.k.Authority(), contract, keeper.PoolRole, pool)
	require.NoError(t, err)
	require.True(t, revoked)
	response, err = client.HasRole(context.Background(), request)
	require.NoError(t, err)
	require.False(t, response.HasRole)
}

func TestQueryHasRoleRefusals(t *testing.T) {
	e := newEnv(t)
	server := keeper.NewQuerier(e.k)
	for _, ctx := range []context.Context{context.Background(), sdk.WrapSDKContext(e.ctx)} {
		response, err := server.HasRole(ctx, nil)
		require.Nil(t, response)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
		require.Equal(t, "empty request", status.Convert(err).Message())
	}
	router := baseapp.NewQueryServerTestHelper(e.ctx, testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, server)
	client := types.NewQueryClient(router)
	valid := types.QueryHasRoleRequest{
		Contract: e.k.Generation2().FeeAccumulator.Hex(), Role: keeper.PoolRole.Hex(), Account: address("query role account").Hex(),
	}
	assertRefusal := func(t *testing.T, request types.QueryHasRoleRequest, want *sdkerrors.Error) {
		t.Helper()
		response, err := client.HasRole(context.Background(), &request)
		require.Nil(t, response)
		require.ErrorIs(t, err, want)
		codespace, code, _ := sdkerrors.ABCIInfo(err, false)
		require.Equal(t, want.Codespace(), codespace)
		require.Equal(t, want.ABCICode(), code)
	}
	roles := []struct {
		name  string
		value string
	}{
		{"empty", ""},
		{"empty hex", "0x"},
		{"no prefix", keeper.PoolRole.Hex()[2:]},
		{"uppercase prefix", "0X" + keeper.PoolRole.Hex()[2:]},
		{"31 bytes", "0x" + strings.Repeat("ab", 31)},
		{"33 bytes", "0x" + strings.Repeat("ab", 33)},
		{"63 digits", "0x" + strings.Repeat("a", 63)},
		{"65 digits", "0x" + strings.Repeat("a", 65)},
		{"non hex", "0x" + strings.Repeat("z", 64)},
		{"leading space", " " + keeper.PoolRole.Hex()},
		{"trailing space", keeper.PoolRole.Hex() + " "},
	}
	for _, tc := range roles {
		t.Run("role/"+tc.name, func(t *testing.T) {
			request := valid
			request.Role = tc.value
			assertRefusal(t, request, sdkerrors.ErrInvalidRequest)
		})
	}
	addresses := []struct {
		name  string
		value string
	}{
		{"empty", ""},
		{"short", "0x1234"},
		{"no prefix", strings.Repeat("a", 40)},
		{"uppercase prefix", "0X" + strings.Repeat("a", 40)},
		{"39 digits", "0x" + strings.Repeat("a", 39)},
		{"41 digits", "0x" + strings.Repeat("a", 41)},
		{"non hex", "0x" + strings.Repeat("z", 40)},
		{"bech32", e.k.Authority()},
	}
	for _, field := range []string{"contract", "account"} {
		for _, tc := range addresses {
			t.Run(field+"/"+tc.name, func(t *testing.T) {
				request := valid
				if field == "contract" {
					request.Contract = tc.value
				} else {
					request.Account = tc.value
				}
				assertRefusal(t, request, sdkerrors.ErrInvalidAddress)
			})
		}
		t.Run(field+"/zero", func(t *testing.T) {
			request := valid
			if field == "contract" {
				request.Contract = (common.Address{}).Hex()
			} else {
				request.Account = (common.Address{}).Hex()
			}
			assertRefusal(t, request, types.ErrZeroAddress)
		})
	}
}
