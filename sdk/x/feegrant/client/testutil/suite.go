package testutil

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tmcli "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/cli"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/suite"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	clitestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/feegrant"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/feegrant/client/cli"
	govtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/client/testutil"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
)

const (
	oneYear  = 365 * 24 * 60 * 60
	tenHours = 10 * 60 * 60
	oneHour  = 60 * 60
)

type IntegrationTestSuite struct {
	suite.Suite

	cfg          network.Config
	network      *network.Network
	addedGranter sdk.AccAddress
	addedGrantee sdk.AccAddress
	addedGrant   feegrant.Grant
}

func NewIntegrationTestSuite(cfg network.Config) *IntegrationTestSuite {
	return &IntegrationTestSuite{cfg: cfg}
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.T().Log("setting up integration test suite")

	if testing.Short() {
		s.T().Skip("skipping test in unit-tests mode.")
	}

	s.network = network.New(s.T(), s.cfg)

	_, err := s.network.WaitForHeight(1)
	s.Require().NoError(err)

	val := s.network.Validators[0]
	granter := val.Address
	grantee := s.network.Validators[1].Address

	s.createGrant(granter, grantee)

	grant, err := feegrant.NewGrant(granter, grantee, &feegrant.BasicAllowance{
		SpendLimit: sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(100))),
	})
	s.Require().NoError(err)

	s.addedGrant = grant
	s.addedGranter = granter
	s.addedGrantee = grantee
}

func (s *IntegrationTestSuite) fixtureAddress(name string) sdk.AccAddress {
	kb := s.network.Validators[0].ClientCtx.Keyring
	algorithms, _ := kb.SupportedAlgorithms()
	algo, err := keyring.NewSigningAlgoFromString(s.cfg.SigningAlgo, algorithms)
	s.Require().NoError(err)
	info, _, err := kb.NewMnemonic(s.T().Name()+"/"+name, keyring.English, sdk.FullFundraiserPath, keyring.DefaultBIP39Passphrase, algo)
	s.Require().NoError(err)
	return sdk.AccAddress(info.GetPubKey().Address())
}

// createGrant creates a new basic allowance fee grant from granter to grantee.
func (s *IntegrationTestSuite) createGrant(granter, grantee sdk.Address) {
	val := s.network.Validators[0]

	clientCtx := val.ClientCtx
	commonFlags := []string{
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
	}

	fee := sdk.NewCoin("uhpx", sdk.NewInt(100))

	args := append(
		[]string{
			granter.String(),
			grantee.String(),
			fmt.Sprintf("--%s=%s", cli.FlagSpendLimit, fee.String()),
			fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
			fmt.Sprintf("--%s=%s", cli.FlagExpiration, getFormattedExpiration(oneYear)),
		},
		commonFlags...,
	)

	cmd := cli.NewCmdFeeGrant()

	_, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)
	_, err = s.network.WaitForHeight(1)
	s.Require().NoError(err)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down integration test suite")
	s.network.Cleanup()
}

func (s *IntegrationTestSuite) TestCmdGetFeeGrant() {
	missingGranter := s.fixtureAddress("missingGranter")

	val := s.network.Validators[0]
	granter := val.Address
	grantee := s.addedGrantee
	clientCtx := val.ClientCtx

	testCases := []struct {
		name         string
		args         []string
		expectErrMsg string
		expectErr    bool
		respType     *feegrant.Grant
		resp         *feegrant.Grant
	}{
		{
			"wrong granter",
			[]string{
				"wrong_granter",
				grantee.String(),
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			"decoding bech32 failed",
			true, nil, nil,
		},
		{
			"wrong grantee",
			[]string{
				granter.String(),
				"wrong_grantee",
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			"decoding bech32 failed",
			true, nil, nil,
		},
		{
			"non existed grant",
			[]string{
				missingGranter.String(),
				grantee.String(),
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			"fee-grant not found",
			true, nil, nil,
		},
		{
			"valid req",
			[]string{
				granter.String(),
				grantee.String(),
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			"",
			false,
			&feegrant.Grant{},
			&s.addedGrant,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdQueryFeeGrant()
			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)

			if tc.expectErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.expectErrMsg)
			} else {
				s.Require().NoError(err)
				s.Require().NoError(clientCtx.Codec.UnmarshalAsJSON(out.Bytes(), tc.respType), out.String())
				s.Require().Equal(tc.resp.Grantee, tc.respType.Grantee)
				s.Require().Equal(tc.resp.Granter, tc.respType.Granter)
				grant, err := tc.respType.GetGrant()
				s.Require().NoError(err)
				grant1, err1 := tc.resp.GetGrant()
				s.Require().NoError(err1)
				s.Require().Equal(
					grant.(*feegrant.BasicAllowance).SpendLimit,
					grant1.(*feegrant.BasicAllowance).SpendLimit,
				)
			}
		})
	}
}

func (s *IntegrationTestSuite) TestCmdGetFeeGrantsByGrantee() {
	missingGrantee := s.fixtureAddress("missingGrantee")

	val := s.network.Validators[0]
	grantee := s.addedGrantee
	clientCtx := val.ClientCtx

	testCases := []struct {
		name         string
		args         []string
		expectErr    bool
		resp         *feegrant.QueryAllowancesResponse
		expectLength int
	}{
		{
			"wrong grantee",
			[]string{
				"wrong_grantee",
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			true, nil, 0,
		},
		{
			"non existent grantee",
			[]string{
				missingGrantee.String(),
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			false, &feegrant.QueryAllowancesResponse{}, 0,
		},
		{
			"valid req",
			[]string{
				grantee.String(),
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			false, &feegrant.QueryAllowancesResponse{}, 1,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdQueryFeeGrantsByGrantee()
			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)

			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
				s.Require().NoError(clientCtx.Codec.UnmarshalAsJSON(out.Bytes(), tc.resp), out.String())
				s.Require().Len(tc.resp.Allowances, tc.expectLength)
				for _, grant := range tc.resp.Allowances {
					s.Require().Equal(s.addedGranter.String(), grant.Granter)
					s.Require().Equal(grantee.String(), grant.Grantee)
				}
			}
		})
	}
}

func (s *IntegrationTestSuite) TestCmdGetFeeGrantsByGranter() {
	missingGranter := s.fixtureAddress("missingGranter")

	val := s.network.Validators[0]
	granter := s.addedGranter
	clientCtx := val.ClientCtx

	testCases := []struct {
		name         string
		args         []string
		expectErr    bool
		resp         *feegrant.QueryAllowancesByGranterResponse
		expectLength int
	}{
		{
			"wrong grantee",
			[]string{
				"wrong_grantee",
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			true, nil, 0,
		},
		{
			"non existent grantee",
			[]string{
				missingGranter.String(),
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			false, &feegrant.QueryAllowancesByGranterResponse{}, 0,
		},
		{
			"valid req",
			[]string{
				granter.String(),
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			false, &feegrant.QueryAllowancesByGranterResponse{}, 1,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdQueryFeeGrantsByGranter()
			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)

			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
				s.Require().NoError(clientCtx.Codec.UnmarshalAsJSON(out.Bytes(), tc.resp), out.String())
				s.Require().Len(tc.resp.Allowances, tc.expectLength)
				for _, grant := range tc.resp.Allowances {
					s.Require().Equal(granter.String(), grant.Granter)
					s.Require().Equal(s.addedGrantee.String(), grant.Grantee)
				}
			}
		})
	}
}

func (s *IntegrationTestSuite) TestNewCmdFeeGrant() {
	basicGrantee := s.fixtureAddress("basicGrantee")
	keyNameGrantee := s.fixtureAddress("keyNameGrantee")
	aminoGrantee := s.fixtureAddress("aminoGrantee")
	unlimitedGrantee := s.fixtureAddress("unlimitedGrantee")
	noExpirationGrantee := s.fixtureAddress("noExpirationGrantee")
	unlimitedNoExpirationGrantee := s.fixtureAddress("unlimitedNoExpirationGrantee")
	periodicGrantee := s.fixtureAddress("periodicGrantee")
	periodicUnlimitedGrantee := s.fixtureAddress("periodicUnlimitedGrantee")
	periodicNoExpirationGrantee := s.fixtureAddress("periodicNoExpirationGrantee")
	periodicUnlimitedNoExpirationGrantee := s.fixtureAddress("periodicUnlimitedNoExpirationGrantee")

	val := s.network.Validators[0]
	granter := val.Address
	alreadyExistedGrantee := s.addedGrantee
	clientCtx := val.ClientCtx

	fromAddr, fromName, _, err := client.GetFromFields(clientCtx, clientCtx.Keyring, granter.String())
	s.Require().Equal(fromAddr, granter)
	s.Require().NoError(err)

	commonFlags := []string{
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
	}

	testCases := []struct {
		name         string
		args         []string
		expectErr    bool
		expectedCode uint32
		respType     proto.Message
	}{
		{
			"wrong granter address",
			append(
				[]string{
					"wrong_granter",
					basicGrantee.String(),
					fmt.Sprintf("--%s=%s", cli.FlagSpendLimit, "100uhpx"),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
				},
				commonFlags...,
			),
			true, 0, nil,
		},
		{
			"wrong grantee address",
			append(
				[]string{
					granter.String(),
					"wrong_grantee",
					fmt.Sprintf("--%s=%s", cli.FlagSpendLimit, "100uhpx"),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
				},
				commonFlags...,
			),
			true, 0, nil,
		},
		{
			"wrong granter key name",
			append(
				[]string{
					"invalid_granter",
					keyNameGrantee.String(),
					fmt.Sprintf("--%s=%s", cli.FlagSpendLimit, "100uhpx"),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
				},
				commonFlags...,
			),
			true, 0, nil,
		},
		{
			"valid basic fee grant",
			append(
				[]string{
					granter.String(),
					basicGrantee.String(),
					fmt.Sprintf("--%s=%s", cli.FlagSpendLimit, "100uhpx"),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
				},
				commonFlags...,
			),
			false, 0, &sdk.TxResponse{},
		},
		{
			"valid basic fee grant with granter key name",
			append(
				[]string{
					fromName,
					keyNameGrantee.String(),
					fmt.Sprintf("--%s=%s", cli.FlagSpendLimit, "100uhpx"),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, fromName),
				},
				commonFlags...,
			),
			false, 0, &sdk.TxResponse{},
		},
		{
			"valid basic fee grant with amino",
			append(
				[]string{
					granter.String(),
					aminoGrantee.String(),
					fmt.Sprintf("--%s=%s", cli.FlagSpendLimit, "100uhpx"),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
					fmt.Sprintf("--%s=%s", flags.FlagSignMode, flags.SignModeLegacyAminoJSON),
				},
				commonFlags...,
			),
			false, 0, &sdk.TxResponse{},
		},
		{
			"valid basic fee grant without spend limit",
			append(
				[]string{
					granter.String(),
					unlimitedGrantee.String(),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
				},
				commonFlags...,
			),
			false, 0, &sdk.TxResponse{},
		},
		{
			"valid basic fee grant without expiration",
			append(
				[]string{
					granter.String(),
					noExpirationGrantee.String(),
					fmt.Sprintf("--%s=%s", cli.FlagSpendLimit, "100uhpx"),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
				},
				commonFlags...,
			),
			false, 0, &sdk.TxResponse{},
		},
		{
			"valid basic fee grant without spend-limit and expiration",
			append(
				[]string{
					granter.String(),
					unlimitedNoExpirationGrantee.String(),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
				},
				commonFlags...,
			),
			false, 0, &sdk.TxResponse{},
		},
		{
			"try to add existed grant",
			append(
				[]string{
					granter.String(),
					alreadyExistedGrantee.String(),
					fmt.Sprintf("--%s=%s", cli.FlagSpendLimit, "100uhpx"),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
				},
				commonFlags...,
			),
			false, 18, &sdk.TxResponse{},
		},
		{
			"invalid number of args(periodic fee grant)",
			append(
				[]string{
					granter.String(),
					basicGrantee.String(),
					fmt.Sprintf("--%s=%s", cli.FlagSpendLimit, "100uhpx"),
					fmt.Sprintf("--%s=%s", cli.FlagPeriodLimit, "10uhpx"),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
					fmt.Sprintf("--%s=%s", cli.FlagExpiration, getFormattedExpiration(tenHours)),
				},
				commonFlags...,
			),
			true, 0, nil,
		},
		{
			"period mentioned and period limit omitted, invalid periodic grant",
			append(
				[]string{
					granter.String(),
					basicGrantee.String(),
					fmt.Sprintf("--%s=%s", cli.FlagSpendLimit, "100uhpx"),
					fmt.Sprintf("--%s=%d", cli.FlagPeriod, tenHours),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
					fmt.Sprintf("--%s=%s", cli.FlagExpiration, getFormattedExpiration(oneHour)),
				},
				commonFlags...,
			),
			true, 0, nil,
		},
		{
			"period cannot be greater than the actual expiration(periodic fee grant)",
			append(
				[]string{
					granter.String(),
					basicGrantee.String(),
					fmt.Sprintf("--%s=%s", cli.FlagSpendLimit, "100uhpx"),
					fmt.Sprintf("--%s=%d", cli.FlagPeriod, tenHours),
					fmt.Sprintf("--%s=%s", cli.FlagPeriodLimit, "10uhpx"),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
					fmt.Sprintf("--%s=%s", cli.FlagExpiration, getFormattedExpiration(oneHour)),
				},
				commonFlags...,
			),
			true, 0, nil,
		},
		{
			"valid periodic fee grant",
			append(
				[]string{
					granter.String(),
					periodicGrantee.String(),
					fmt.Sprintf("--%s=%s", cli.FlagSpendLimit, "100uhpx"),
					fmt.Sprintf("--%s=%d", cli.FlagPeriod, oneHour),
					fmt.Sprintf("--%s=%s", cli.FlagPeriodLimit, "10uhpx"),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
					fmt.Sprintf("--%s=%s", cli.FlagExpiration, getFormattedExpiration(tenHours)),
				},
				commonFlags...,
			),
			false, 0, &sdk.TxResponse{},
		},
		{
			"valid periodic fee grant without spend-limit",
			append(
				[]string{
					granter.String(),
					periodicUnlimitedGrantee.String(),
					fmt.Sprintf("--%s=%d", cli.FlagPeriod, oneHour),
					fmt.Sprintf("--%s=%s", cli.FlagPeriodLimit, "10uhpx"),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
					fmt.Sprintf("--%s=%s", cli.FlagExpiration, getFormattedExpiration(tenHours)),
				},
				commonFlags...,
			),
			false, 0, &sdk.TxResponse{},
		},
		{
			"valid periodic fee grant without expiration",
			append(
				[]string{
					granter.String(),
					periodicNoExpirationGrantee.String(),
					fmt.Sprintf("--%s=%s", cli.FlagSpendLimit, "100uhpx"),
					fmt.Sprintf("--%s=%d", cli.FlagPeriod, oneHour),
					fmt.Sprintf("--%s=%s", cli.FlagPeriodLimit, "10uhpx"),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
				},
				commonFlags...,
			),
			false, 0, &sdk.TxResponse{},
		},
		{
			"valid periodic fee grant without spend-limit and expiration",
			append(
				[]string{
					granter.String(),
					periodicUnlimitedNoExpirationGrantee.String(),
					fmt.Sprintf("--%s=%d", cli.FlagPeriod, oneHour),
					fmt.Sprintf("--%s=%s", cli.FlagPeriodLimit, "10uhpx"),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
				},
				commonFlags...,
			),
			false, 0, &sdk.TxResponse{},
		},
		{
			"invalid expiration",
			append(
				[]string{
					granter.String(),
					periodicUnlimitedGrantee.String(),
					fmt.Sprintf("--%s=%d", cli.FlagPeriod, oneHour),
					fmt.Sprintf("--%s=%s", cli.FlagPeriodLimit, "10uhpx"),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
					fmt.Sprintf("--%s=%s", cli.FlagExpiration, "invalid"),
				},
				commonFlags...,
			),
			true, 0, nil,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.NewCmdFeeGrant()
			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)

			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
				s.Require().NoError(clientCtx.Codec.UnmarshalAsJSON(out.Bytes(), tc.respType), out.String())

				txResp := tc.respType.(*sdk.TxResponse)
				s.Require().Equal(tc.expectedCode, txResp.Code, out.String())
			}
		})
	}
}

func (s *IntegrationTestSuite) TestNewCmdRevokeFeegrant() {
	missingGrantee := s.fixtureAddress("missingGrantee")

	val := s.network.Validators[0]
	granter := s.addedGranter
	grantee := s.addedGrantee
	clientCtx := val.ClientCtx

	commonFlags := []string{
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
	}

	// Create new fee grant specifically to test amino.
	aminoGrantee := s.fixtureAddress("amino-grantee")
	s.createGrant(granter, aminoGrantee)

	testCases := []struct {
		name         string
		args         []string
		expectErr    bool
		expectedCode uint32
		respType     proto.Message
	}{
		{
			"invalid grantee",
			append(
				[]string{
					"wrong_granter",
					grantee.String(),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
				},
				commonFlags...,
			),
			true, 0, nil,
		},
		{
			"invalid grantee",
			append(
				[]string{
					granter.String(),
					"wrong_grantee",
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
				},
				commonFlags...,
			),
			true, 0, nil,
		},
		{
			"Non existed grant",
			append(
				[]string{
					granter.String(),
					missingGrantee.String(),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
				},
				commonFlags...,
			),
			false, 4, &sdk.TxResponse{},
		},
		{
			"Valid revoke",
			append(
				[]string{
					granter.String(),
					grantee.String(),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
				},
				commonFlags...,
			),
			false, 0, &sdk.TxResponse{},
		},
		{
			"Valid revoke with amino",
			append(
				[]string{
					granter.String(),
					aminoGrantee.String(),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
					fmt.Sprintf("--%s=%s", flags.FlagSignMode, flags.SignModeLegacyAminoJSON),
				},
				commonFlags...,
			),
			false, 0, &sdk.TxResponse{},
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.NewCmdRevokeFeegrant()
			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)

			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
				s.Require().NoError(clientCtx.Codec.UnmarshalAsJSON(out.Bytes(), tc.respType), out.String())

				txResp := tc.respType.(*sdk.TxResponse)
				s.Require().Equal(tc.expectedCode, txResp.Code, out.String())
			}
		})
	}
}

func (s *IntegrationTestSuite) TestTxWithFeeGrant() {
	val := s.network.Validators[0]
	clientCtx := val.ClientCtx
	granter := val.Address

	// creating an account manually (This account won't be exist in state)
	info, _, err := val.ClientCtx.Keyring.NewMnemonic("grantee", keyring.English, sdk.FullFundraiserPath, keyring.DefaultBIP39Passphrase, hd.Secp256k1)
	s.Require().NoError(err)
	grantee := sdk.AccAddress(info.GetPubKey().Address())

	commonFlags := []string{
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
	}

	fee := sdk.NewCoin("uhpx", sdk.NewInt(100))

	args := append(
		[]string{
			granter.String(),
			grantee.String(),
			fmt.Sprintf("--%s=%s", cli.FlagSpendLimit, fee.String()),
			fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
			fmt.Sprintf("--%s=%s", cli.FlagExpiration, getFormattedExpiration(oneYear)),
		},
		commonFlags...,
	)

	cmd := cli.NewCmdFeeGrant()

	_, err = clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)
	_, err = s.network.WaitForHeight(1)
	s.Require().NoError(err)

	// granted fee allowance for an account which is not in state and creating
	// any tx with it by using --fee-account shouldn't fail
	out, err := govtestutil.MsgSubmitProposal(val.ClientCtx, grantee.String(),
		"Text Proposal", "No desc", govtypes.ProposalTypeText,
		fmt.Sprintf("--%s=%s", flags.FlagFeeAccount, granter.String()),
	)

	s.Require().NoError(err)
	var resp sdk.TxResponse
	s.Require().NoError(clientCtx.Codec.UnmarshalAsJSON(out.Bytes(), &resp), out.String())
	s.Require().Equal(uint32(0), resp.Code)
}

func (s *IntegrationTestSuite) TestFilteredFeeAllowance() {
	invalidGranterPeer := s.fixtureAddress("invalidGranterPeer")
	unauthorizedGrantee := s.fixtureAddress("unauthorizedGrantee")

	val := s.network.Validators[0]

	granter := val.Address
	info, _, err := val.ClientCtx.Keyring.NewMnemonic("grantee1", keyring.English, sdk.FullFundraiserPath, keyring.DefaultBIP39Passphrase, hd.Secp256k1)
	s.Require().NoError(err)
	grantee := sdk.AccAddress(info.GetPubKey().Address())

	clientCtx := val.ClientCtx

	commonFlags := []string{
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
	}
	spendLimit := sdk.NewCoin("uhpx", sdk.NewInt(1000))

	allowMsgs := strings.Join([]string{sdk.MsgTypeURL(&govtypes.MsgSubmitProposal{}), sdk.MsgTypeURL(&govtypes.MsgVoteWeighted{})}, ",")

	testCases := []struct {
		name         string
		args         []string
		expectErr    bool
		respType     proto.Message
		expectedCode uint32
	}{
		{
			"invalid granter address",
			append(
				[]string{
					"not an address",
					invalidGranterPeer.String(),
					fmt.Sprintf("--%s=%s", cli.FlagAllowedMsgs, allowMsgs),
					fmt.Sprintf("--%s=%s", cli.FlagSpendLimit, spendLimit.String()),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
				},
				commonFlags...,
			),
			true, &sdk.TxResponse{}, 0,
		},
		{
			"invalid grantee address",
			append(
				[]string{
					granter.String(),
					"not an address",
					fmt.Sprintf("--%s=%s", cli.FlagAllowedMsgs, allowMsgs),
					fmt.Sprintf("--%s=%s", cli.FlagSpendLimit, spendLimit.String()),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
				},
				commonFlags...,
			),
			true, &sdk.TxResponse{}, 0,
		},
		{
			"valid filter fee grant",
			append(
				[]string{
					granter.String(),
					grantee.String(),
					fmt.Sprintf("--%s=%s", cli.FlagAllowedMsgs, allowMsgs),
					fmt.Sprintf("--%s=%s", cli.FlagSpendLimit, spendLimit.String()),
					fmt.Sprintf("--%s=%s", flags.FlagFrom, granter),
				},
				commonFlags...,
			),
			false, &sdk.TxResponse{}, 0,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.NewCmdFeeGrant()
			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)

			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
				s.Require().NoError(clientCtx.Codec.UnmarshalAsJSON(out.Bytes(), tc.respType), out.String())

				txResp := tc.respType.(*sdk.TxResponse)
				s.Require().Equal(tc.expectedCode, txResp.Code, out.String())
			}
		})
	}

	args := []string{
		granter.String(),
		grantee.String(),
		fmt.Sprintf("--%s=json", tmcli.OutputFlag),
	}

	// get filtered fee allowance and check info
	cmd := cli.GetCmdQueryFeeGrant()
	out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
	s.Require().NoError(err)

	resp := &feegrant.Grant{}

	s.Require().NoError(clientCtx.Codec.UnmarshalAsJSON(out.Bytes(), resp), out.String())
	s.Require().Equal(grantee.String(), resp.Grantee)
	s.Require().Equal(granter.String(), resp.Granter)

	grant, err := resp.GetGrant()
	s.Require().NoError(err)

	filteredFeeGrant, err := grant.(*feegrant.AllowedMsgAllowance).GetAllowance()
	s.Require().NoError(err)

	s.Require().Equal(
		filteredFeeGrant.(*feegrant.BasicAllowance).SpendLimit.String(),
		spendLimit.String(),
	)

	// exec filtered fee allowance
	cases := []struct {
		name         string
		malleate     func() (testutil.BufferWriter, error)
		respType     proto.Message
		expectedCode uint32
	}{
		{
			"valid proposal tx",
			func() (testutil.BufferWriter, error) {
				return govtestutil.MsgSubmitProposal(val.ClientCtx, grantee.String(),
					"Text Proposal", "No desc", govtypes.ProposalTypeText,
					fmt.Sprintf("--%s=%s", flags.FlagFeeAccount, granter.String()),
				)
			},
			&sdk.TxResponse{},
			0,
		},
		{
			"valid weighted_vote tx",
			func() (testutil.BufferWriter, error) {
				return govtestutil.MsgVote(val.ClientCtx, grantee.String(), "0", "yes",
					fmt.Sprintf("--%s=%s", flags.FlagFeeAccount, granter.String()),
				)
			},
			&sdk.TxResponse{},
			2,
		},
		{
			"should fail with unauthorized msgs",
			func() (testutil.BufferWriter, error) {
				args := append(
					[]string{
						grantee.String(),
						unauthorizedGrantee.String(),
						fmt.Sprintf("--%s=%s", cli.FlagSpendLimit, "100uhpx"),
						fmt.Sprintf("--%s=%s", flags.FlagFeeAccount, granter),
					},
					commonFlags...,
				)
				cmd := cli.NewCmdFeeGrant()
				return clitestutil.ExecTestCLICmd(clientCtx, cmd, args)
			},
			&sdk.TxResponse{},
			7,
		},
	}

	for _, tc := range cases {
		tc := tc

		s.Run(tc.name, func() {
			out, err := tc.malleate()
			s.Require().NoError(err)
			s.Require().NoError(clientCtx.Codec.UnmarshalAsJSON(out.Bytes(), tc.respType), out.String())
			txResp := tc.respType.(*sdk.TxResponse)
			s.Require().Equal(tc.expectedCode, txResp.Code, out.String())
		})
	}
}

func getFormattedExpiration(duration int64) string {
	return time.Now().Add(time.Duration(duration) * time.Second).Format(time.RFC3339)
}
