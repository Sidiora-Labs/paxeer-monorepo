package testutil

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	tmcli "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/cli"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	oracletypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/types"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/suite"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	clitestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
)

type IntegrationTestSuite struct {
	suite.Suite

	cfg       network.Config
	network   *network.Network
	genesis   types.GenesisState
	validFees sdk.Coins
}

func NewIntegrationTestSuite(cfg network.Config) *IntegrationTestSuite {
	return &IntegrationTestSuite{cfg: cfg}
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.T().Log("setting up integration test suite")

	genesisState := s.cfg.GenesisState
	var bankGenesis types.GenesisState
	s.Require().NoError(s.cfg.Codec.UnmarshalAsJSON(genesisState[types.ModuleName], &bankGenesis))

	bankGenesis.DenomMetadata = []types.Metadata{
		{
			Name:        "Cosmos Hub Atom",
			Symbol:      "ATOM",
			Description: "The native staking token of the Cosmos Hub.",
			DenomUnits: []*types.DenomUnit{
				{
					Denom:    "uatom",
					Exponent: 0,
					Aliases:  []string{"microatom"},
				},
				{
					Denom:    "atom",
					Exponent: 6,
					Aliases:  []string{"ATOM"},
				},
			},
			Base:    "uatom",
			Display: "atom",
		},
		{
			Name:        "Ethereum",
			Symbol:      "ETH",
			Description: "Ethereum mainnet token",
			DenomUnits: []*types.DenomUnit{
				{
					Denom:    "wei",
					Exponent: 0,
				},
				{
					Denom:    "eth",
					Exponent: 6,
					Aliases:  []string{"ETH"},
				},
			},
			Base:    "wei",
			Display: "eth",
		},
	}

	bankGenesisBz, err := s.cfg.Codec.MarshalAsJSON(&bankGenesis)
	s.Require().NoError(err)
	genesisState[types.ModuleName] = bankGenesisBz
	s.cfg.GenesisState = genesisState

	s.network = network.New(s.T(), s.cfg)

	genDoc, err := tmtypes.GenesisDocFromFile(s.network.Validators[0].Ctx.Config.GenesisFile())
	s.Require().NoError(err)
	var appState map[string]json.RawMessage
	s.Require().NoError(json.Unmarshal(genDoc.AppState, &appState))
	s.Require().NoError(s.cfg.Codec.UnmarshalAsJSON(appState[types.ModuleName], &s.genesis))
	supply := sdk.NewCoins()
	for _, balance := range s.genesis.Balances {
		supply = supply.Add(balance.Coins...)
	}
	wei := sdk.ZeroInt()
	for _, balance := range s.genesis.WeiBalances {
		wei = wei.Add(balance.Amount)
	}
	whole, remainder := types.SplitUhpxWeiAmount(wei)
	s.Require().True(remainder.IsZero())
	if !whole.IsZero() {
		baseDenom, err := sdk.GetBaseDenom()
		s.Require().NoError(err)
		supply = supply.Add(sdk.NewCoin(baseDenom, whole))
	}
	if !s.genesis.Supply.Empty() {
		s.Require().Equal(s.genesis.Supply, supply)
	}
	s.genesis.Supply = supply

	// Oracle InitGenesis registers metadata for whitelisted denoms missing from bank genesis.
	var oracleGenesis oracletypes.GenesisState
	s.Require().NoError(s.cfg.Codec.UnmarshalAsJSON(appState[oracletypes.ModuleName], &oracleGenesis))
	metadataByDenom := make(map[string]types.Metadata)
	for _, metadata := range s.genesis.DenomMetadata {
		metadataByDenom[metadata.Base] = metadata
	}
	for _, denom := range oracleGenesis.Params.Whitelist {
		if _, exists := metadataByDenom[denom.Name]; exists {
			continue
		}
		display := denom.Name[1:]
		metadata := types.Metadata{
			Description: display, Base: denom.Name, Display: display,
			Name: strings.ToUpper(display), Symbol: strings.ToUpper(display),
			DenomUnits: []*types.DenomUnit{
				{Denom: denom.Name, Exponent: 0, Aliases: []string{"micro" + display}},
				{Denom: "m" + display, Exponent: 3, Aliases: []string{"milli" + display}},
				{Denom: display, Exponent: 6},
			},
		}
		metadataByDenom[denom.Name] = metadata
		s.genesis.DenomMetadata = append(s.genesis.DenomMetadata, metadata)
	}
	sort.Slice(s.genesis.DenomMetadata, func(i, j int) bool {
		return s.genesis.DenomMetadata[i].Base < s.genesis.DenomMetadata[j].Base
	})
	// Match the CLI JSON codec's representation of empty repeated fields.
	metadataJSON, err := s.cfg.Codec.MarshalAsJSON(&types.QueryDenomsMetadataResponse{Metadatas: s.genesis.DenomMetadata})
	s.Require().NoError(err)
	var metadataResponse types.QueryDenomsMetadataResponse
	s.Require().NoError(s.cfg.Codec.UnmarshalAsJSON(metadataJSON, &metadataResponse))
	s.genesis.DenomMetadata = metadataResponse.Metadatas

	var paramsGenesis paramtypes.GenesisState
	s.Require().NoError(s.cfg.Codec.UnmarshalAsJSON(appState[paramtypes.ModuleName], &paramsGenesis))
	validatorPrices, err := sdk.ParseDecCoins(s.cfg.MinGasPrices)
	s.Require().NoError(err)
	prices := paramsGenesis.FeesParams.GlobalMinimumGasPrices.UnionMax(validatorPrices)
	for _, price := range prices {
		fee := price.Amount.MulInt64(int64(flags.DefaultGasLimit)).Ceil().RoundInt()
		s.validFees = s.validFees.Add(sdk.NewCoin(price.Denom, fee))
	}

	_, err = s.network.WaitForHeight(1)
	s.Require().NoError(err)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down integration test suite")
	s.network.Cleanup()
}

func (s *IntegrationTestSuite) TestGetBalancesCmd() {
	val := s.network.Validators[0]

	testCases := []struct {
		name      string
		args      []string
		expectErr bool
		respType  proto.Message
		expected  proto.Message
	}{
		{"no address provided", []string{}, true, nil, nil},
		{
			"total account balance",
			[]string{
				val.Address.String(),
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
				fmt.Sprintf("--%s=1", flags.FlagHeight),
			},
			false,
			&types.QueryAllBalancesResponse{},
			&types.QueryAllBalancesResponse{
				Balances: sdk.NewCoins(
					sdk.NewCoin(fmt.Sprintf("%stoken", val.Moniker), s.cfg.AccountTokens),
					sdk.NewCoin(s.cfg.BondDenom, s.cfg.StakingTokens.Sub(s.cfg.BondedTokens)),
				),
				Pagination: &query.PageResponse{},
			},
		},
		{
			"total account balance of a specific denom",
			[]string{
				val.Address.String(),
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
				fmt.Sprintf("--%s=%s", cli.FlagDenom, s.cfg.BondDenom),
				fmt.Sprintf("--%s=1", flags.FlagHeight),
			},
			false,
			&sdk.Coin{},
			NewCoin(s.cfg.BondDenom, s.cfg.StakingTokens.Sub(s.cfg.BondedTokens)),
		},
		{
			"total account balance of a bogus denom",
			[]string{
				val.Address.String(),
				fmt.Sprintf("--%s=foobar", cli.FlagDenom),
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			false,
			&sdk.Coin{},
			NewCoin("foobar", sdk.ZeroInt()),
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetBalancesCmd()
			out, err := clitestutil.ExecTestCLICmd(val.ClientCtx, cmd, tc.args)

			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
				s.Require().NoError(val.ClientCtx.Codec.UnmarshalAsJSON(out.Bytes(), tc.respType))
				s.Require().Equal(tc.expected.String(), tc.respType.String())
			}
		})
	}
}

func (s *IntegrationTestSuite) TestGetCmdQueryTotalSupply() {
	val := s.network.Validators[0]

	testCases := []struct {
		name      string
		args      []string
		expectErr bool
		respType  proto.Message
		expected  proto.Message
	}{
		{
			name: "total supply",
			args: []string{
				fmt.Sprintf("--%s=1", flags.FlagHeight),
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			respType: &types.QueryTotalSupplyResponse{},
			expected: &types.QueryTotalSupplyResponse{
				Supply:     s.genesis.Supply,
				Pagination: &query.PageResponse{Total: 0},
			},
		},
		{
			name: "total supply of a specific denomination",
			args: []string{
				fmt.Sprintf("--%s=1", flags.FlagHeight),
				fmt.Sprintf("--%s=%s", cli.FlagDenom, s.cfg.BondDenom),
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			respType: &sdk.Coin{},
			expected: &sdk.Coin{
				Denom:  s.cfg.BondDenom,
				Amount: s.genesis.Supply.AmountOf(s.cfg.BondDenom),
			},
		},
		{
			name: "total supply of a bogus denom",
			args: []string{
				fmt.Sprintf("--%s=1", flags.FlagHeight),
				fmt.Sprintf("--%s=foobar", cli.FlagDenom),
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			respType: &sdk.Coin{},
			expected: &sdk.Coin{
				Denom:  "foobar",
				Amount: sdk.ZeroInt(),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdQueryTotalSupply()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
				s.Require().NoError(clientCtx.Codec.UnmarshalAsJSON(out.Bytes(), tc.respType))
				s.Require().Equal(tc.expected, tc.respType)
			}
		})
	}
}

func (s *IntegrationTestSuite) TestGetCmdQueryDenomsMetadata() {
	val := s.network.Validators[0]
	var atomMetadata types.Metadata
	for _, metadata := range s.genesis.DenomMetadata {
		if metadata.Base == "uatom" {
			atomMetadata = metadata
			break
		}
	}
	s.Require().Equal("uatom", atomMetadata.Base)

	s.Run("all denoms metadata with total count", func() {
		queryClient := types.NewQueryClient(val.ClientCtx.WithHeight(1))
		metadata, err := queryClient.DenomsMetadata(context.Background(), &types.QueryDenomsMetadataRequest{
			Pagination: &query.PageRequest{CountTotal: true},
		})
		s.Require().NoError(err)
		metadataJSON, err := val.ClientCtx.Codec.MarshalAsJSON(metadata)
		s.Require().NoError(err)
		var response types.QueryDenomsMetadataResponse
		s.Require().NoError(val.ClientCtx.Codec.UnmarshalAsJSON(metadataJSON, &response))
		s.Require().Equal(&types.QueryDenomsMetadataResponse{
			Metadatas:  s.genesis.DenomMetadata,
			Pagination: &query.PageResponse{Total: uint64(len(s.genesis.DenomMetadata))},
		}, &response)
	})

	testCases := []struct {
		name      string
		args      []string
		expectErr bool
		respType  proto.Message
		expected  proto.Message
	}{
		{
			name: "all denoms client metadata",
			args: []string{
				fmt.Sprintf("--%s=1", flags.FlagHeight),
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			respType: &types.QueryDenomsMetadataResponse{},
			expected: &types.QueryDenomsMetadataResponse{
				Metadatas:  s.genesis.DenomMetadata,
				Pagination: &query.PageResponse{},
			},
		},
		{
			name: "all denoms client metadata with default pagination",
			args: []string{
				fmt.Sprintf("--%s=1", flags.FlagHeight),
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			respType: &types.QueryDenomsMetadataResponse{},
			expected: &types.QueryDenomsMetadataResponse{
				Metadatas:  s.genesis.DenomMetadata,
				Pagination: &query.PageResponse{},
			},
		},
		{
			name: "client metadata of a specific denomination",
			args: []string{
				fmt.Sprintf("--%s=1", flags.FlagHeight),
				fmt.Sprintf("--%s=%s", cli.FlagDenom, "uatom"),
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			respType: &types.QueryDenomMetadataResponse{},
			expected: &types.QueryDenomMetadataResponse{
				Metadata: atomMetadata,
			},
		},
		{
			name: "client metadata of a bogus denom",
			args: []string{
				fmt.Sprintf("--%s=1", flags.FlagHeight),
				fmt.Sprintf("--%s=foobar", cli.FlagDenom),
				fmt.Sprintf("--%s=json", tmcli.OutputFlag),
			},
			expectErr: true,
			respType:  &types.QueryDenomMetadataResponse{},
			expected: &types.QueryDenomMetadataResponse{
				Metadata: types.Metadata{
					DenomUnits: []*types.DenomUnit{},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetCmdDenomsMetadata()
			clientCtx := val.ClientCtx

			out, err := clitestutil.ExecTestCLICmd(clientCtx, cmd, tc.args)
			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
				s.Require().NoError(clientCtx.Codec.UnmarshalAsJSON(out.Bytes(), tc.respType))
				s.Require().Equal(tc.expected, tc.respType)
			}
		})
	}
}

func (s *IntegrationTestSuite) TestNewSendTxCmdGenOnly() {
	val := s.network.Validators[0]

	clientCtx := val.ClientCtx

	from := val.Address
	to := val.Address
	amount := sdk.NewCoins(
		sdk.NewCoin(fmt.Sprintf("%stoken", val.Moniker), sdk.NewInt(10)),
		sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10)),
	)
	args := []string{
		fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
		fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
		fmt.Sprintf("--%s=%s", flags.FlagFees, s.validFees.String()),
		fmt.Sprintf("--%s=true", flags.FlagGenerateOnly),
	}

	bz, err := MsgSendExec(clientCtx, from, to, amount, args...)
	s.Require().NoError(err)
	tx, err := s.cfg.TxConfig.TxJSONDecoder()(bz.Bytes())
	s.Require().NoError(err)
	s.Require().Equal([]sdk.Msg{types.NewMsgSend(from, to, amount)}, tx.GetMsgs())
}

func (s *IntegrationTestSuite) TestNewSendTxCmd() {
	val := s.network.Validators[0]

	testCases := []struct {
		name         string
		from, to     sdk.AccAddress
		amount       sdk.Coins
		args         []string
		expectErr    bool
		expectedCode uint32
		respType     proto.Message
	}{
		{
			"valid transaction",
			val.Address,
			val.Address,
			sdk.NewCoins(
				sdk.NewCoin(fmt.Sprintf("%stoken", val.Moniker), sdk.NewInt(10)),
				sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10)),
			),
			[]string{
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
				fmt.Sprintf("--%s=%s", flags.FlagFees, s.validFees.String()),
			},
			false, 0, &sdk.TxResponse{},
		},
		{
			"not enough fees",
			val.Address,
			val.Address,
			sdk.NewCoins(
				sdk.NewCoin(fmt.Sprintf("%stoken", val.Moniker), sdk.NewInt(10)),
				sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10)),
			),
			[]string{
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(1))).String()),
			},
			false,
			sdkerrors.ErrInsufficientFee.ABCICode(),
			&sdk.TxResponse{},
		},
		{
			"not enough gas",
			val.Address,
			val.Address,
			sdk.NewCoins(
				sdk.NewCoin(fmt.Sprintf("%stoken", val.Moniker), sdk.NewInt(10)),
				sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10)),
			),
			[]string{
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
				fmt.Sprintf("--%s=%s", flags.FlagFees, s.validFees.String()),
				"--gas=10",
			},
			false,
			sdkerrors.ErrOutOfGas.ABCICode(),
			&sdk.TxResponse{},
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			clientCtx := val.ClientCtx

			bz, err := MsgSendExec(clientCtx, tc.from, tc.to, tc.amount, tc.args...)
			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)

				s.Require().NoError(clientCtx.Codec.UnmarshalAsJSON(bz.Bytes(), tc.respType), bz.String())
				txResp := tc.respType.(*sdk.TxResponse)
				s.Require().Equal(tc.expectedCode, txResp.Code)
			}
		})
	}
}

func NewCoin(denom string, amount sdk.Int) *sdk.Coin {
	coin := sdk.NewCoin(denom, amount)
	return &coin
}
