//go:build norace

package rest_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	grpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/grpc"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/suite"

	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
)

type IntegrationTestSuite struct {
	suite.Suite
	cfg     network.Config
	network *network.Network
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.T().Log("setting up integration test suite")

	cfg := network.DefaultConfig(s.T())

	genesisState := cfg.GenesisState
	cfg.NumValidators = 1

	var mintData minttypes.GenesisState
	s.Require().NoError(cfg.Codec.UnmarshalAsJSON(genesisState[minttypes.ModuleName], &mintData))

	now := time.Now().UTC()
	release := minttypes.ScheduledTokenRelease{
		StartDate:          now.AddDate(1, 0, 0).Format(minttypes.TokenReleaseDateFormat),
		EndDate:            now.AddDate(2, 0, 0).Format(minttypes.TokenReleaseDateFormat),
		TokenReleaseAmount: 500000000,
	}
	mintData.Params = minttypes.NewParams(sdk.DefaultBondDenom, []minttypes.ScheduledTokenRelease{release})
	mintData.Minter = minttypes.NewMinter(release.StartDate, release.EndDate, mintData.Params.MintDenom, release.TokenReleaseAmount)
	s.Require().NoError(minttypes.ValidateGenesis(mintData))

	mintDataBz, err := cfg.Codec.MarshalAsJSON(&mintData)
	s.Require().NoError(err)
	genesisState[minttypes.ModuleName] = mintDataBz
	cfg.GenesisState = genesisState

	s.cfg = cfg
	s.network = network.New(s.T(), cfg)

	_, err = s.network.WaitForHeight(1)
	s.Require().NoError(err)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down integration test suite")
	s.network.Cleanup()
}

func (s *IntegrationTestSuite) TestQueryGRPC() {
	val := s.network.Validators[0]
	baseURL := val.APIAddress
	var mintData minttypes.GenesisState
	s.Require().NoError(s.cfg.Codec.UnmarshalAsJSON(s.cfg.GenesisState[minttypes.ModuleName], &mintData))

	testCases := []struct {
		name     string
		url      string
		headers  map[string]string
		respType proto.Message
		expected proto.Message
	}{
		{
			"gRPC request params",
			fmt.Sprintf("%s/paxchain/mint/v1beta1/params", baseURL),
			map[string]string{},
			&minttypes.QueryParamsResponse{},
			&minttypes.QueryParamsResponse{
				Params: mintData.Params,
			},
		},
		{
			"gRPC request minter",
			fmt.Sprintf("%s/paxchain/mint/v1beta1/minter", baseURL),
			map[string]string{
				grpctypes.GRPCBlockHeightHeader: "1",
			},
			&minttypes.QueryMinterResponse{},
			&minttypes.QueryMinterResponse{
				StartDate:           mintData.Minter.StartDate,
				EndDate:             mintData.Minter.EndDate,
				Denom:               sdk.DefaultBondDenom,
				TotalMintAmount:     500000000,
				RemainingMintAmount: 500000000,
				LastMintAmount:      0,
				LastMintDate:        "0001-01-01",
				LastMintHeight:      0,
			},
		},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			resp, err := testutil.GetRequestWithHeaders(tc.url, tc.headers)
			s.Require().NoError(err)
			s.Require().NoError(val.ClientCtx.Codec.UnmarshalAsJSON(resp, tc.respType))
			s.Require().Equal(tc.expected, tc.respType)
		})
	}
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
