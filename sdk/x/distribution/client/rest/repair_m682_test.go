package rest_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	grpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/grpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/types"
)

// distribution-gateway-disabled: execute every restored gateway assertion in the
// task's focused gate as well as the ordinary TestIntegrationTestSuite entry.
func TestRepairM_682_DistributionGatewaySuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func TestRepairM_682_KeeperUpdatesReachGateway(t *testing.T) {
	f := newDistributionGatewayFixture(t)
	getDistributionGatewayResponse(t, f, "/community_pool", &types.QueryCommunityPoolResponse{Pool: sdk.DecCoins{}})

	amount := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 7))
	require.NoError(t, f.keeper.FundCommunityPool(f.ctx, amount, f.Address))
	require.Equal(t, sdk.NewInt64Coin("uhpx", 33), f.bank.GetBalance(f.ctx, f.Address, "uhpx"))
	getDistributionGatewayResponse(t, f, "/community_pool", &types.QueryCommunityPoolResponse{Pool: sdk.NewDecCoinsFromCoins(amount...)})

	withdrawAddr := sdk.AccAddress(ed25519.GenPrivKeyFromSecret([]byte("distribution gateway withdraw address")).PubKey().Address())
	_, err := keeper.NewMsgServerImpl(f.keeper).SetWithdrawAddress(sdk.WrapSDKContext(f.ctx), types.NewMsgSetWithdrawAddress(f.Address, withdrawAddr))
	require.NoError(t, err)
	getDistributionGatewayResponse(t, f, "/delegators/"+f.Address.String()+"/withdraw_address", &types.QueryDelegatorWithdrawAddressResponse{WithdrawAddress: withdrawAddr.String()})

	params := types.DefaultParams()
	params.CommunityTax = sdk.NewDecWithPrec(3, 2)
	params.WithdrawAddrEnabled = false
	f.keeper.SetParams(f.ctx, params)
	getDistributionGatewayResponse(t, f, "/params", &types.QueryParamsResponse{Params: params})
}

func TestRepairM_682_RewardQueriesAreIsolated(t *testing.T) {
	f := newDistributionGatewayFixture(t)
	current := f.keeper.GetValidatorCurrentRewards(f.ctx, f.ValAddress)
	starting := f.keeper.GetDelegatorStartingInfo(f.ctx, f.ValAddress, f.Address)
	history := f.keeper.GetValidatorHistoricalRewards(f.ctx, f.ValAddress, starting.PreviousPeriod)
	rewards := sdk.NewDecCoins(sdk.NewInt64DecCoin("uhpx", 10))
	for i := 0; i < 2; i++ {
		getDistributionGatewayResponse(t, f, fmt.Sprintf("/delegators/%s/rewards/%s", f.Address, f.ValAddress), &types.QueryDelegationRewardsResponse{Rewards: rewards})
		getDistributionGatewayResponse(t, f, fmt.Sprintf("/delegators/%s/rewards", f.Address), &types.QueryDelegationTotalRewardsResponse{
			Rewards: []types.DelegationDelegatorReward{types.NewDelegationDelegatorReward(f.ValAddress, rewards)}, Total: rewards,
		})
		require.Equal(t, current, f.keeper.GetValidatorCurrentRewards(f.ctx, f.ValAddress))
		require.Equal(t, starting, f.keeper.GetDelegatorStartingInfo(f.ctx, f.ValAddress, f.Address))
		require.Equal(t, history, f.keeper.GetValidatorHistoricalRewards(f.ctx, f.ValAddress, starting.PreviousPeriod))
	}
}

func TestRepairM_682_BlockHeightHeaders(t *testing.T) {
	f := newDistributionGatewayFixture(t)
	for _, tc := range []struct {
		height string
		status int
	}{
		{"2", http.StatusOK},
		{"0", http.StatusOK},
		{"1", http.StatusBadRequest},
		{"-1", http.StatusBadRequest},
		{"not-a-height", http.StatusBadRequest},
	} {
		t.Run(tc.height, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, f.APIAddress+"/cosmos/distribution/v1beta1/params", nil)
			require.NoError(t, err)
			req.Header.Set(grpctypes.GRPCBlockHeightHeader, tc.height)
			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			require.Equal(t, tc.status, response.StatusCode, "%s", body)
			if tc.status == http.StatusOK {
				require.Equal(t, "2", response.Header.Get("Grpc-Metadata-"+grpctypes.GRPCBlockHeightHeader))
				var decoded types.QueryParamsResponse
				require.NoError(t, f.ClientCtx.Codec.UnmarshalAsJSON(body, &decoded))
				require.Equal(t, types.DefaultParams(), decoded.Params)
			} else {
				var decoded struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				}
				require.NoError(t, json.Unmarshal(body, &decoded))
				require.Equal(t, 3, decoded.Code)
				require.Equal(t, "fixture only serves its current height", decoded.Message)
			}
		})
	}
}

func getDistributionGatewayResponse(t *testing.T, f *distributionGatewayFixture, path string, expected proto.Message) {
	t.Helper()
	response, err := http.Get(f.APIAddress + "/cosmos/distribution/v1beta1" + path)
	require.NoError(t, err)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode, "%s", body)
	decoded := reflect.New(reflect.TypeOf(expected).Elem()).Interface().(proto.Message)
	require.NoError(t, f.ClientCtx.Codec.UnmarshalAsJSON(body, decoded))
	require.Equal(t, expected.String(), decoded.String())
}
