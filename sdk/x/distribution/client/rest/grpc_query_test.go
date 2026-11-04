package rest_test

import (
	"context"
	"fmt"
	"net"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	dbm "github.com/tendermint/tm-db"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp" // Register the production SDK gRPC wire codec.
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/api"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	grpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/grpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/rest"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
)

type IntegrationTestSuite struct {
	suite.Suite
	fixture *distributionGatewayFixture
}

type distributionGatewayFixture struct {
	APIAddress string
	ClientCtx  client.Context
	Address    sdk.AccAddress
	ValAddress sdk.ValAddress
	ctx        sdk.Context
	keeper     keeper.Keeper
	bank       bankkeeper.BaseKeeper
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.fixture = newDistributionGatewayFixture(s.T())
}

func newDistributionGatewayFixture(t *testing.T) *distributionGatewayFixture {
	t.Helper()
	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	stakingtypes.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	bankKey := sdk.NewKVStoreKey(banktypes.StoreKey)
	stakingKey := sdk.NewKVStoreKey(stakingtypes.StoreKey)
	distrKey := sdk.NewKVStoreKey(types.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	cms := store.NewCommitMultiStore(db)
	for _, key := range []*sdk.KVStoreKey{authKey, bankKey, stakingKey, distrKey, paramsKey} {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	cms.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, db)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 1, Time: time.Unix(100, 0).UTC()}, false)
	subspace := func(name string) paramtypes.Subspace {
		return paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, name)
	}
	ak := authkeeper.NewAccountKeeper(cdc, authKey, subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount,
		map[string][]string{
			types.ModuleName: {}, authtypes.FeeCollectorName: {},
			stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
			stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
		})
	ak.SetParams(ctx, authtypes.DefaultParams())
	bk := bankkeeper.NewBaseKeeper(cdc, bankKey, ak, subspace(banktypes.ModuleName), map[string]bool{})
	sk := stakingkeeper.NewKeeper(cdc, stakingKey, ak, bk, subspace(stakingtypes.ModuleName))
	sk.SetParams(ctx, stakingtypes.DefaultParams())
	dk := keeper.NewKeeper(cdc, distrKey, subspace(types.ModuleName), ak, bk, sk, authtypes.FeeCollectorName, map[string]bool{})
	sk.SetHooks(dk.Hooks())

	pk := ed25519.GenPrivKeyFromSecret([]byte("distribution gateway validator")).PubKey()
	f := &distributionGatewayFixture{
		ClientCtx: client.Context{}.WithCodec(cdc).WithInterfaceRegistry(registry),
		Address:   sdk.AccAddress(pk.Address()), ValAddress: sdk.ValAddress(pk.Address()),
		keeper: dk, bank: bk,
	}
	for _, name := range []string{types.ModuleName, authtypes.FeeCollectorName, stakingtypes.BondedPoolName, stakingtypes.NotBondedPoolName} {
		ak.GetModuleAccount(ctx, name)
	}
	ak.SetAccount(ctx, ak.NewAccountWithAddress(ctx, f.Address))
	bankGenesis := banktypes.DefaultGenesisState()
	bankGenesis.Balances = []banktypes.Balance{
		{Address: ak.GetModuleAddress(authtypes.FeeCollectorName).String(), Coins: sdk.NewCoins(sdk.NewInt64Coin("uhpx", 20))},
		{Address: ak.GetModuleAddress(stakingtypes.BondedPoolName).String(), Coins: sdk.NewCoins(sdk.NewInt64Coin("uhpx", 100))},
		{Address: f.Address.String(), Coins: sdk.NewCoins(sdk.NewInt64Coin("uhpx", 40))},
	}
	bk.InitGenesis(ctx, bankGenesis)
	genesis := types.DefaultGenesisState()
	require.NoError(t, types.ValidateGenesis(genesis))
	dk.InitGenesis(ctx, *genesis)
	validator, err := stakingtypes.NewValidator(f.ValAddress, pk, stakingtypes.Description{})
	require.NoError(t, err)
	validator.Status = stakingtypes.Bonded
	validator.Tokens = sdk.NewInt(100)
	validator.DelegatorShares = sdk.NewDec(100)
	validator.Commission = stakingtypes.NewCommission(sdk.NewDecWithPrec(5, 1), sdk.OneDec(), sdk.OneDec())
	sk.SetValidator(ctx, validator)
	require.NoError(t, sk.SetValidatorByConsAddr(ctx, validator))
	sk.AfterValidatorCreated(ctx, f.ValAddress)
	sk.BeforeDelegationCreated(ctx, f.Address, f.ValAddress)
	sk.SetDelegation(ctx, stakingtypes.NewDelegation(f.Address, f.ValAddress, sdk.NewDec(100)))
	sk.AfterDelegationModified(ctx, f.Address, f.ValAddress)

	// Allocate actual fee-collector balances at height 2. Zero-tax genesis and
	// 50% commission produce exactly the suite's 20/10/10 uhpx expectations.
	f.ctx = ctx.WithBlockHeight(2)
	dk.AllocateTokens(f.ctx, 1, 1, sdk.ConsAddress(pk.Address()), []abci.VoteInfo{
		{Validator: abci.Validator{Address: pk.Address(), Power: 1}, SignedLastBlock: true},
	})
	require.True(t, bk.GetAllBalances(f.ctx, ak.GetModuleAddress(authtypes.FeeCollectorName)).IsZero())
	require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 20)), bk.GetAllBalances(f.ctx, ak.GetModuleAddress(types.ModuleName)))

	// Each real gRPC query gets an isolated branch, as in BaseApp. Reward
	// queries increment periods and must not mutate the fixture's live state.
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(func(callCtx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		md, _ := metadata.FromIncomingContext(callCtx)
		if heights := md.Get(grpctypes.GRPCBlockHeightHeader); len(heights) > 0 {
			height, err := strconv.ParseInt(heights[0], 10, 64)
			if err != nil || (height != 0 && height != f.ctx.BlockHeight()) {
				return nil, status.Error(codes.InvalidArgument, "fixture only serves its current height")
			}
		}
		queryCtx, _ := f.ctx.WithContext(callCtx).CacheContext()
		if err := grpc.SetHeader(callCtx, metadata.Pairs(grpctypes.GRPCBlockHeightHeader, strconv.FormatInt(f.ctx.BlockHeight(), 10))); err != nil {
			return nil, err
		}
		return handler(sdk.WrapSDKContext(queryCtx), req)
	}))
	types.RegisterQueryServer(grpcServer, dk)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	serveErr := make(chan error, 1)
	go func() { serveErr <- grpcServer.Serve(listener) }()
	t.Cleanup(func() {
		grpcServer.Stop()
		require.NoError(t, <-serveErr)
	})
	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	apiServer := api.New(f.ClientCtx)
	require.NoError(t, types.RegisterQueryHandlerClient(context.Background(), apiServer.GRPCGatewayRouter, types.NewQueryClient(conn)))
	server := httptest.NewServer(apiServer.GRPCGatewayRouter)
	t.Cleanup(server.Close)
	f.APIAddress = server.URL
	return f
}

func (s *IntegrationTestSuite) TestQueryParamsGRPC() {
	val := s.fixture
	baseURL := val.APIAddress

	testCases := []struct {
		name     string
		url      string
		respType proto.Message
		expected proto.Message
	}{
		{
			"gRPC request params",
			fmt.Sprintf("%s/cosmos/distribution/v1beta1/params", baseURL),
			&types.QueryParamsResponse{},
			&types.QueryParamsResponse{
				Params: types.DefaultParams(),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		resp, err := rest.GetRequest(tc.url)
		s.Run(tc.name, func() {
			s.Require().NoError(err)
			s.Require().NoError(val.ClientCtx.Codec.UnmarshalAsJSON(resp, tc.respType))
			s.Require().Equal(tc.expected, tc.respType)
		})
	}
}

func (s *IntegrationTestSuite) TestQueryOutstandingRewardsGRPC() {
	val := s.fixture
	baseURL := val.APIAddress

	rewards, err := sdk.ParseDecCoins("20uhpx")
	s.Require().NoError(err)

	testCases := []struct {
		name     string
		url      string
		headers  map[string]string
		expErr   bool
		respType proto.Message
		expected proto.Message
	}{
		{
			"gRPC request params with wrong validator address",
			fmt.Sprintf("%s/cosmos/distribution/v1beta1/validators/%s/outstanding_rewards", baseURL, "wrongAddress"),
			map[string]string{},
			true,
			&types.QueryValidatorOutstandingRewardsResponse{},
			&types.QueryValidatorOutstandingRewardsResponse{},
		},
		{
			"gRPC request params valid address",
			fmt.Sprintf("%s/cosmos/distribution/v1beta1/validators/%s/outstanding_rewards", baseURL, val.ValAddress.String()),
			map[string]string{
				grpctypes.GRPCBlockHeightHeader: "2",
			},
			false,
			&types.QueryValidatorOutstandingRewardsResponse{},
			&types.QueryValidatorOutstandingRewardsResponse{
				Rewards: types.ValidatorOutstandingRewards{
					Rewards: rewards,
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		resp, err := testutil.GetRequestWithHeaders(tc.url, tc.headers)
		s.Run(tc.name, func() {
			if tc.expErr {
				s.Require().NoError(err)
				s.Require().Error(val.ClientCtx.Codec.UnmarshalAsJSON(resp, tc.respType))
			} else {
				s.Require().NoError(err)
				s.Require().NoError(val.ClientCtx.Codec.UnmarshalAsJSON(resp, tc.respType))
				s.Require().Equal(tc.expected.String(), tc.respType.String())
			}
		})
	}
}

func (s *IntegrationTestSuite) TestQueryValidatorCommissionGRPC() {
	val := s.fixture
	baseURL := val.APIAddress

	commission, err := sdk.ParseDecCoins("10uhpx")
	s.Require().NoError(err)

	testCases := []struct {
		name     string
		url      string
		headers  map[string]string
		expErr   bool
		respType proto.Message
		expected proto.Message
	}{
		{
			"gRPC request params with wrong validator address",
			fmt.Sprintf("%s/cosmos/distribution/v1beta1/validators/%s/commission", baseURL, "wrongAddress"),
			map[string]string{},
			true,
			&types.QueryValidatorCommissionResponse{},
			&types.QueryValidatorCommissionResponse{},
		},
		{
			"gRPC request params valid address",
			fmt.Sprintf("%s/cosmos/distribution/v1beta1/validators/%s/commission", baseURL, val.ValAddress.String()),
			map[string]string{
				grpctypes.GRPCBlockHeightHeader: "2",
			},
			false,
			&types.QueryValidatorCommissionResponse{},
			&types.QueryValidatorCommissionResponse{
				Commission: types.ValidatorAccumulatedCommission{
					Commission: commission,
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		resp, err := testutil.GetRequestWithHeaders(tc.url, tc.headers)
		s.Run(tc.name, func() {
			if tc.expErr {
				s.Require().NoError(err)
				s.Require().Error(val.ClientCtx.Codec.UnmarshalAsJSON(resp, tc.respType))
			} else {
				s.Require().NoError(err)
				s.Require().NoError(val.ClientCtx.Codec.UnmarshalAsJSON(resp, tc.respType))
				s.Require().Equal(tc.expected.String(), tc.respType.String())
			}
		})
	}
}

func (s *IntegrationTestSuite) TestQuerySlashesGRPC() {
	val := s.fixture
	baseURL := val.APIAddress

	testCases := []struct {
		name     string
		url      string
		expErr   bool
		respType proto.Message
		expected proto.Message
	}{
		{
			"invalid validator address",
			fmt.Sprintf("%s/cosmos/distribution/v1beta1/validators/%s/slashes", baseURL, ""),
			true,
			&types.QueryValidatorSlashesResponse{},
			nil,
		},
		{
			"invalid start height",
			fmt.Sprintf("%s/cosmos/distribution/v1beta1/validators/%s/slashes?starting_height=%s&ending_height=%s", baseURL, val.ValAddress.String(), "-1", "3"),
			true,
			&types.QueryValidatorSlashesResponse{},
			nil,
		},
		{
			"invalid start height",
			fmt.Sprintf("%s/cosmos/distribution/v1beta1/validators/%s/slashes?starting_height=%s&ending_height=%s", baseURL, val.ValAddress.String(), "1", "-3"),
			true,
			&types.QueryValidatorSlashesResponse{},
			nil,
		},
		{
			"valid request get slashes",
			fmt.Sprintf("%s/cosmos/distribution/v1beta1/validators/%s/slashes?starting_height=%s&ending_height=%s", baseURL, val.ValAddress.String(), "1", "3"),
			false,
			&types.QueryValidatorSlashesResponse{},
			&types.QueryValidatorSlashesResponse{
				Pagination: &query.PageResponse{},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		resp, err := rest.GetRequest(tc.url)

		s.Run(tc.name, func() {
			if tc.expErr {
				s.Require().NoError(err)
				s.Require().Error(val.ClientCtx.Codec.UnmarshalAsJSON(resp, tc.respType))
			} else {
				s.Require().NoError(err)
				s.Require().NoError(val.ClientCtx.Codec.UnmarshalAsJSON(resp, tc.respType))
				s.Require().Equal(tc.expected.String(), tc.respType.String())
			}
		})
	}
}

func (s *IntegrationTestSuite) TestQueryDelegatorRewardsGRPC() {
	val := s.fixture
	baseUrl := val.APIAddress

	rewards, err := sdk.ParseDecCoins("10.0uhpx")
	s.Require().NoError(err)

	testCases := []struct {
		name     string
		url      string
		headers  map[string]string
		expErr   bool
		respType proto.Message
		expected proto.Message
	}{
		{
			"wrong delegator address",
			fmt.Sprintf("%s/cosmos/distribution/v1beta1/delegators/%s/rewards", baseUrl, "wrongDelegatorAddress"),
			map[string]string{},
			true,
			&types.QueryDelegationTotalRewardsResponse{},
			nil,
		},
		{
			"valid request",
			fmt.Sprintf("%s/cosmos/distribution/v1beta1/delegators/%s/rewards", baseUrl, val.Address.String()),
			map[string]string{
				grpctypes.GRPCBlockHeightHeader: "2",
			},
			false,
			&types.QueryDelegationTotalRewardsResponse{},
			&types.QueryDelegationTotalRewardsResponse{
				Rewards: []types.DelegationDelegatorReward{
					types.NewDelegationDelegatorReward(val.ValAddress, rewards),
				},
				Total: rewards,
			},
		},
		{
			"wrong validator address(specific validator rewards)",
			fmt.Sprintf("%s/cosmos/distribution/v1beta1/delegators/%s/rewards/%s", baseUrl, val.Address.String(), "wrongValAddress"),
			map[string]string{},
			true,
			&types.QueryDelegationTotalRewardsResponse{},
			nil,
		},
		{
			"valid request(specific validator rewards)",
			fmt.Sprintf("%s/cosmos/distribution/v1beta1/delegators/%s/rewards/%s", baseUrl, val.Address.String(), val.ValAddress.String()),
			map[string]string{
				grpctypes.GRPCBlockHeightHeader: "2",
			},
			false,
			&types.QueryDelegationRewardsResponse{},
			&types.QueryDelegationRewardsResponse{
				Rewards: rewards,
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		resp, err := testutil.GetRequestWithHeaders(tc.url, tc.headers)

		s.Run(tc.name, func() {
			if tc.expErr {
				s.Require().NoError(err)
				s.Require().Error(val.ClientCtx.Codec.UnmarshalAsJSON(resp, tc.respType))
			} else {
				s.Require().NoError(err)
				s.Require().NoError(val.ClientCtx.Codec.UnmarshalAsJSON(resp, tc.respType))
				s.Require().Equal(tc.expected.String(), tc.respType.String())
			}
		})
	}
}

func (s *IntegrationTestSuite) TestQueryDelegatorValidatorsGRPC() {
	val := s.fixture
	baseUrl := val.APIAddress

	testCases := []struct {
		name     string
		url      string
		expErr   bool
		respType proto.Message
		expected proto.Message
	}{
		{
			"empty delegator address",
			fmt.Sprintf("%s/cosmos/distribution/v1beta1/delegators/%s/validators", baseUrl, ""),
			true,
			&types.QueryDelegatorValidatorsResponse{},
			nil,
		},
		{
			"wrong delegator address",
			fmt.Sprintf("%s/cosmos/distribution/v1beta1/delegators/%s/validators", baseUrl, "wrongDelegatorAddress"),
			true,
			&types.QueryDelegatorValidatorsResponse{},
			nil,
		},
		{
			"valid request",
			fmt.Sprintf("%s/cosmos/distribution/v1beta1/delegators/%s/validators", baseUrl, val.Address.String()),
			false,
			&types.QueryDelegatorValidatorsResponse{},
			&types.QueryDelegatorValidatorsResponse{
				Validators: []string{val.ValAddress.String()},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		resp, err := rest.GetRequest(tc.url)

		s.Run(tc.name, func() {
			if tc.expErr {
				s.Require().NoError(err)
				s.Require().Error(val.ClientCtx.Codec.UnmarshalAsJSON(resp, tc.respType))
			} else {
				s.Require().NoError(err)
				s.Require().NoError(val.ClientCtx.Codec.UnmarshalAsJSON(resp, tc.respType))
				s.Require().Equal(tc.expected.String(), tc.respType.String())
			}
		})
	}
}

func (s *IntegrationTestSuite) TestQueryWithdrawAddressGRPC() {
	val := s.fixture
	baseUrl := val.APIAddress

	testCases := []struct {
		name     string
		url      string
		expErr   bool
		respType proto.Message
		expected proto.Message
	}{
		{
			"empty delegator address",
			fmt.Sprintf("%s/cosmos/distribution/v1beta1/delegators/%s/withdraw_address", baseUrl, ""),
			true,
			&types.QueryDelegatorWithdrawAddressResponse{},
			nil,
		},
		{
			"wrong delegator address",
			fmt.Sprintf("%s/cosmos/distribution/v1beta1/delegators/%s/withdraw_address", baseUrl, "wrongDelegatorAddress"),
			true,
			&types.QueryDelegatorWithdrawAddressResponse{},
			nil,
		},
		{
			"valid request",
			fmt.Sprintf("%s/cosmos/distribution/v1beta1/delegators/%s/withdraw_address", baseUrl, val.Address.String()),
			false,
			&types.QueryDelegatorWithdrawAddressResponse{},
			&types.QueryDelegatorWithdrawAddressResponse{
				WithdrawAddress: val.Address.String(),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		resp, err := rest.GetRequest(tc.url)

		s.Run(tc.name, func() {
			if tc.expErr {
				s.Require().NoError(err)
				s.Require().Error(val.ClientCtx.Codec.UnmarshalAsJSON(resp, tc.respType))
			} else {
				s.Require().NoError(err)
				s.Require().NoError(val.ClientCtx.Codec.UnmarshalAsJSON(resp, tc.respType))
				s.Require().Equal(tc.expected.String(), tc.respType.String())
			}
		})
	}
}

func (s *IntegrationTestSuite) TestQueryValidatorCommunityPoolGRPC() {
	val := s.fixture
	baseURL := val.APIAddress

	communityPool, err := sdk.ParseDecCoins("0uhpx")
	s.Require().NoError(err)

	testCases := []struct {
		name     string
		url      string
		headers  map[string]string
		expErr   bool
		respType proto.Message
		expected proto.Message
	}{
		{
			"gRPC request params with wrong validator address",
			fmt.Sprintf("%s/cosmos/distribution/v1beta1/community_pool", baseURL),
			map[string]string{
				grpctypes.GRPCBlockHeightHeader: "2",
			},
			false,
			&types.QueryCommunityPoolResponse{},
			&types.QueryCommunityPoolResponse{
				Pool: communityPool,
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		resp, err := testutil.GetRequestWithHeaders(tc.url, tc.headers)

		s.Run(tc.name, func() {
			if tc.expErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
				s.Require().NoError(val.ClientCtx.Codec.UnmarshalAsJSON(resp, tc.respType))
				s.Require().Equal(tc.expected.String(), tc.respType.String())
			}
		})
	}
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
