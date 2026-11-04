package rest_test

import (
	"fmt"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/rest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz/client/cli"
	authztestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/authz/client/testutil"
	banktestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/client/testutil"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
)

type IntegrationTestSuite struct {
	suite.Suite
	cfg     network.Config
	network *network.Network
	grantee sdk.AccAddress
}

var typeMsgSend = banktypes.SendAuthorization{}.MsgTypeURL()
var typeMsgVote = sdk.MsgTypeURL(&govtypes.MsgVote{})

func (s *IntegrationTestSuite) defaultConfig() network.Config {
	return network.DefaultConfig(s.T())
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.T().Log("setting up integration test suite")

	cfg := s.defaultConfig()

	cfg.NumValidators = 1
	s.cfg = cfg
	s.network = network.New(s.T(), cfg)

	val := s.network.Validators[0]
	// Create new account in the keyring.
	info, _, err := val.ClientCtx.Keyring.NewMnemonic("grantee", keyring.English, sdk.FullFundraiserPath, keyring.DefaultBIP39Passphrase, hd.Secp256k1)
	s.Require().NoError(err)
	newAddr := sdk.AccAddress(info.GetPubKey().Address())

	// Send some funds to the new account.
	out, err := banktestutil.MsgSendExec(
		val.ClientCtx,
		val.Address,
		newAddr,
		sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(200))), fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
	)
	s.Require().NoError(err)
	s.Require().Contains(out.String(), `"code":0`)

	// grant authorization
	out, err = authztestutil.ExecGrant(val, []string{
		newAddr.String(),
		"send",
		fmt.Sprintf("--%s=100steak", cli.FlagSpendLimit),
		fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
		fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(10))).String()),
		fmt.Sprintf("--%s=%d", cli.FlagExpiration, time.Now().Add(time.Minute*time.Duration(120)).Unix()),
	})
	s.Require().NoError(err)
	s.Require().Contains(out.String(), `"code":0`)

	s.grantee = newAddr
	_, err = s.network.WaitForHeight(1)
	s.Require().NoError(err)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down integration test suite")
	s.network.Cleanup()
}

func (s *IntegrationTestSuite) TestQueryGrantGRPC() {
	val := s.network.Validators[0]
	grantsURL := val.APIAddress + "/cosmos/authz/v1beta1/grants?granter=%s&grantee=%s&msg_type_url=%s"
	testCases := []struct {
		name      string
		url       string
		expectErr bool
		errorMsg  string
	}{
		{
			"fail invalid granter address",
			fmt.Sprintf(grantsURL, "invalid_granter", s.grantee.String(), typeMsgSend),
			true,
			"decoding bech32 failed: invalid separator index -1: invalid request",
		},
		{
			"fail invalid grantee address",
			fmt.Sprintf(grantsURL, val.Address.String(), "invalid_grantee", typeMsgSend),
			true,
			"decoding bech32 failed: invalid separator index -1: invalid request",
		},
		{
			"fail with empty granter",
			fmt.Sprintf(grantsURL, "", s.grantee.String(), typeMsgSend),
			true,
			"empty address string is not allowed: invalid request",
		},
		{
			"fail with empty grantee",
			fmt.Sprintf(grantsURL, val.Address.String(), "", typeMsgSend),
			true,
			"empty address string is not allowed: invalid request",
		},
		{
			"fail invalid msg-type",
			fmt.Sprintf(grantsURL, val.Address.String(), s.grantee.String(), "invalidMsg"),
			true,
			"rpc error: code = NotFound desc = no authorization found for invalidMsg type: key not found",
		},
		{
			"valid query",
			fmt.Sprintf(grantsURL, val.Address.String(), s.grantee.String(), typeMsgSend),
			false,
			"",
		},
	}
	for _, tc := range testCases {
		tc := tc
		s.Run(tc.name, func() {
			resp, _ := rest.GetRequest(tc.url)
			require := s.Require()
			if tc.expectErr {
				require.Contains(string(resp), tc.errorMsg)
			} else {
				var g authz.QueryGrantsResponse
				err := val.ClientCtx.Codec.UnmarshalAsJSON(resp, &g)
				require.NoError(err)
				require.Len(g.Grants, 1)
				require.NoError(g.Grants[0].UnpackInterfaces(val.ClientCtx.InterfaceRegistry))
				auth := g.Grants[0].GetAuthorization()
				require.Equal(auth.MsgTypeURL(), banktypes.SendAuthorization{}.MsgTypeURL())
			}
		})
	}
}

func (s *IntegrationTestSuite) TestQueryGrantsGRPC() {
	val := s.network.Validators[0]
	grantsURL := val.APIAddress + "/cosmos/authz/v1beta1/grants?granter=%s&grantee=%s"
	testCases := []struct {
		name      string
		url       string
		expectErr bool
		errMsg    string
		preRun    func()
		postRun   func(*authz.QueryGrantsResponse)
	}{
		{
			"valid query: expect single grant",
			fmt.Sprintf(grantsURL, val.Address.String(), s.grantee.String()),
			false,
			"",
			func() {},
			func(g *authz.QueryGrantsResponse) {
				s.Require().Len(g.Grants, 1)
			},
		},
		{
			"valid query: expect two grants",
			fmt.Sprintf(grantsURL, val.Address.String(), s.grantee.String()),
			false,
			"",
			func() {
				_, err := authztestutil.ExecGrant(val, []string{
					s.grantee.String(),
					"generic",
					fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
					fmt.Sprintf("--%s=%s", cli.FlagMsgType, typeMsgVote),
					fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
					fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
					fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdk.NewInt(10))).String()),
					fmt.Sprintf("--%s=%d", cli.FlagExpiration, time.Now().Add(time.Minute*time.Duration(120)).Unix()),
				})
				s.Require().NoError(err)
			},
			func(g *authz.QueryGrantsResponse) {
				s.Require().Len(g.Grants, 2)
			},
		},
		{
			"valid query: expect single grant with pagination",
			fmt.Sprintf(grantsURL+"&pagination.limit=1", val.Address.String(), s.grantee.String()),
			false,
			"",
			func() {},
			func(g *authz.QueryGrantsResponse) {
				s.Require().Len(g.Grants, 1)
			},
		},
		{
			"valid query: expect two grants with pagination",
			fmt.Sprintf(grantsURL+"&pagination.limit=2", val.Address.String(), s.grantee.String()),
			false,
			"",
			func() {},
			func(g *authz.QueryGrantsResponse) {
				s.Require().Len(g.Grants, 2)
			},
		},
	}
	for _, tc := range testCases {
		tc := tc
		s.Run(tc.name, func() {
			tc.preRun()
			resp, err := rest.GetRequest(tc.url)
			s.Require().NoError(err)

			if tc.expectErr {
				s.Require().Contains(string(resp), tc.errMsg)
			} else {
				var authorizations authz.QueryGrantsResponse
				err := val.ClientCtx.Codec.UnmarshalAsJSON(resp, &authorizations)
				s.Require().NoError(err)
				tc.postRun(&authorizations)
			}

		})
	}
}

// grantListFixture creates an isolated, asymmetric graph through the real CLI:
// seven grants from one granter, plus an eighth from another granter to recipient 0.
func (s *IntegrationTestSuite) grantListFixture() (sdk.AccAddress, []sdk.AccAddress, sdk.AccAddress, sdk.AccAddress) {
	val := s.network.Validators[0]
	newAccount := func(label string) sdk.AccAddress {
		info, _, err := val.ClientCtx.Keyring.NewMnemonic(s.T().Name()+label, keyring.English, sdk.FullFundraiserPath, keyring.DefaultBIP39Passphrase, hd.Secp256k1)
		s.Require().NoError(err)
		return info.GetAddress()
	}
	granter := newAccount("granter")
	other := newAccount("other-granter")
	empty := newAccount("empty")
	for _, addr := range []sdk.AccAddress{granter, other} {
		out, err := banktestutil.MsgSendExec(val.ClientCtx, val.Address, addr,
			sdk.NewCoins(sdk.NewInt64Coin(s.cfg.BondDenom, 1000)),
			fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
			fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
			fmt.Sprintf("--%s=10%s", flags.FlagFees, s.cfg.BondDenom))
		s.Require().NoError(err)
		s.Require().Contains(out.String(), `"code":0`)
	}
	grant := func(from, to sdk.AccAddress) {
		out, err := authztestutil.ExecGrant(val, []string{
			to.String(), "generic",
			fmt.Sprintf("--%s=%s", flags.FlagFrom, from.String()),
			fmt.Sprintf("--%s=%s", cli.FlagMsgType, typeMsgVote),
			fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
			fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
			fmt.Sprintf("--%s=10%s", flags.FlagFees, s.cfg.BondDenom),
			fmt.Sprintf("--%s=%d", cli.FlagExpiration, time.Now().Add(2*time.Hour).Unix()),
		})
		s.Require().NoError(err)
		s.Require().Contains(out.String(), `"code":0`)
	}
	recipients := make([]sdk.AccAddress, 7)
	for i := range recipients {
		recipients[i] = newAccount(fmt.Sprintf("recipient-%d", i))
		grant(granter, recipients[i])
	}
	grant(other, recipients[0])
	return granter, recipients, empty, other
}

func (s *IntegrationTestSuite) TestQueryGranterGrantsGRPC() {
	val := s.network.Validators[0]
	granter, recipients, empty, _ := s.grantListFixture()
	testCases := []struct {
		name      string
		address   string
		expectErr bool
		numItems  int
	}{
		{"invalid account address", "invalid address", true, 0},
		{"no authorizations found", empty.String(), false, 0},
		{"valid query", granter.String(), false, 7},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			require := s.Require()
			resp, err := rest.GetRequest(fmt.Sprintf("%s/cosmos/authz/v1beta1/grants/granter/%s", val.APIAddress, tc.address))
			require.NoError(err)
			if tc.expectErr {
				require.Contains(string(resp), "decoding bech32 failed")
			} else {
				var authorizations authz.QueryGranterGrantsResponse
				require.NoError(val.ClientCtx.Codec.UnmarshalAsJSON(resp, &authorizations))
				require.Len(authorizations.Grants, tc.numItems)
				got := make([]string, 0, len(authorizations.Grants))
				for _, grant := range authorizations.Grants {
					require.Equal(granter.String(), grant.Granter)
					got = append(got, grant.Grantee)
					require.NoError(grant.UnpackInterfaces(val.ClientCtx.InterfaceRegistry))
					require.Equal(typeMsgVote, grant.Authorization.GetCachedValue().(authz.Authorization).MsgTypeURL())
				}
				if tc.numItems > 0 {
					want := make([]string, len(recipients))
					for i, addr := range recipients {
						want[i] = addr.String()
					}
					require.ElementsMatch(want, got)
				}
			}
		})
	}
}

func (s *IntegrationTestSuite) TestQueryGranteeGrantsGRPC() {
	val := s.network.Validators[0]
	granter, recipients, empty, other := s.grantListFixture()
	testCases := []struct {
		name      string
		address   string
		expectErr bool
		numItems  int
	}{
		{"invalid account address", "invalid address", true, 0},
		{"no authorizations found", empty.String(), false, 0},
		{"valid query", recipients[1].String(), false, 1},
		{"multiple granters", recipients[0].String(), false, 2},
		{"granter is not a grantee", granter.String(), false, 0},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			require := s.Require()
			resp, err := rest.GetRequest(fmt.Sprintf("%s/cosmos/authz/v1beta1/grants/grantee/%s", val.APIAddress, tc.address))
			require.NoError(err)
			if tc.expectErr {
				require.Contains(string(resp), "decoding bech32 failed")
			} else {
				var authorizations authz.QueryGranteeGrantsResponse
				require.NoError(val.ClientCtx.Codec.UnmarshalAsJSON(resp, &authorizations))
				require.Len(authorizations.Grants, tc.numItems)
				granters := make(map[string]bool)
				for _, grant := range authorizations.Grants {
					require.Equal(tc.address, grant.Grantee)
					require.False(granters[grant.Granter])
					granters[grant.Granter] = true
					require.NoError(grant.UnpackInterfaces(val.ClientCtx.InterfaceRegistry))
					require.Equal(typeMsgVote, grant.Authorization.GetCachedValue().(authz.Authorization).MsgTypeURL())
				}
				if tc.numItems > 0 {
					require.True(granters[granter.String()])
				}
				if tc.numItems == 2 {
					require.True(granters[other.String()])
				}
			}
		})
	}
}
