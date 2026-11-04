package ibctesting_test

import (
	"fmt"
	"testing"
	"time"

	tmrand "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/rand"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	kmultisig "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/multisig"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	servertypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/types"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	storev2rootmulti "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	clitestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/rest"
	txtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx"
	authcli "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/client/cli"
	authrest "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/client/rest"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/suite"
	dbm "github.com/tendermint/tm-db"

	ibcclientcli "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/core/02-client/client/cli"
	ibctesting "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing/simapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing/simapp/params"
	paxdbconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
)

/*
	This file contains tests from the SDK which had to deleted during the migration of
	the IBC module from the SDK into this repository. https://github.com/cosmos/cosmos-sdk/pull/8735

	They can be removed once the SDK deprecates amino.
*/

type IntegrationTestSuite struct {
	suite.Suite

	cfg     network.Config
	network *network.Network
}

func (s *IntegrationTestSuite) SetupSuite() {
	s.T().Log("setting up integration test suite")

	cfg := DefaultConfig()

	s.cfg = cfg
	s.network = network.New(s.T(), cfg)
	s.T().Cleanup(s.network.Cleanup)

	kb := s.network.Validators[0].ClientCtx.Keyring
	_, _, err := kb.NewMnemonic("newAccount", keyring.English, sdk.FullFundraiserPath, keyring.DefaultBIP39Passphrase, hd.Secp256k1)
	s.Require().NoError(err)

	account1, _, err := kb.NewMnemonic("newAccount1", keyring.English, sdk.FullFundraiserPath, keyring.DefaultBIP39Passphrase, hd.Secp256k1)
	s.Require().NoError(err)

	account2, _, err := kb.NewMnemonic("newAccount2", keyring.English, sdk.FullFundraiserPath, keyring.DefaultBIP39Passphrase, hd.Secp256k1)
	s.Require().NoError(err)

	multi := kmultisig.NewLegacyAminoPubKey(2, []cryptotypes.PubKey{account1.GetPubKey(), account2.GetPubKey()})
	_, err = kb.SaveMultisig("multi", multi)
	s.Require().NoError(err)

	_, err = s.network.WaitForHeight(1)
	s.Require().NoError(err)

	s.Require().NoError(s.network.WaitForNextBlock())
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

// NewAppConstructor returns a new simapp AppConstructor
func NewAppConstructor(encodingCfg params.EncodingConfig) network.AppConstructor {
	return func(val network.Validator) servertypes.Application {
		// Match the current simapp test store, with synchronous history for CLI
		// account queries that may race the next committed consensus block.
		scConfig := paxdbconfig.DefaultStateCommitConfig()
		scConfig.MemIAVLConfig.AsyncCommitBuffer = 0
		scConfig.MemIAVLConfig.SnapshotMinTimeInterval = 0
		ssConfig := paxdbconfig.DefaultStateStoreConfig()
		ssConfig.AsyncWriteBuffer = 0
		cms := storev2rootmulti.NewStore(val.Ctx.Config.RootDir, scConfig, ssConfig, nil)
		return simapp.NewSimApp(
			dbm.NewMemDB(), nil, true, make(map[int64]bool), val.Ctx.Config.RootDir, 0,
			val.Ctx.Config, encodingCfg,
			simapp.EmptyAppOptions{},
			func(app *baseapp.BaseApp) { app.SetCMS(cms) },
			baseapp.SetPruning(storetypes.NewPruningOptionsFromString(val.AppConfig.Pruning)),
			baseapp.SetMinGasPrices(val.AppConfig.MinGasPrices),
		)
	}
}

// DefaultConfig returns a sane default configuration suitable for nearly all
// testing requirements.
func DefaultConfig() network.Config {
	encCfg := simapp.MakeTestEncodingConfig()

	return network.Config{
		Codec:             encCfg.Marshaler,
		TxConfig:          encCfg.TxConfig,
		LegacyAmino:       encCfg.Amino,
		InterfaceRegistry: encCfg.InterfaceRegistry,
		AccountRetriever:  authtypes.AccountRetriever{},
		AppConstructor:    NewAppConstructor(encCfg),
		GenesisState:      simapp.ModuleBasics.DefaultGenesis(encCfg.Marshaler),
		TimeoutCommit:     2 * time.Second,
		ChainID:           "chain-" + tmrand.NewRand().Str(6),
		NumValidators:     1,
		BondDenom:         sdk.DefaultBondDenom,
		MinGasPrices:      fmt.Sprintf("0.000006%s", sdk.DefaultBondDenom),
		AccountTokens:     sdk.TokensFromConsensusPower(1000, sdk.DefaultPowerReduction),
		StakingTokens:     sdk.TokensFromConsensusPower(500, sdk.DefaultPowerReduction),
		BondedTokens:      sdk.TokensFromConsensusPower(100, sdk.DefaultPowerReduction),
		PruningStrategy:   storetypes.PruningOptionNothing,
		CleanupDir:        true,
		SigningAlgo:       string(hd.Secp256k1Type),
		KeyringOptions:    []keyring.Option{},
	}
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.T().Log("tearing down integration test suite")
	// The registered cleanup also runs when setup or an assertion fails.
}

// TestLegacyRestErrMessages creates two IBC txs, one that fails, one that
// succeeds, and make sure we cannot query any of them (with pretty error msg).
// Our intension is to test the error message of querying a message which is
// signed with proto, since IBC won't support legacy amino at all we are
// considering a message from IBC module.
func (s *IntegrationTestSuite) TestLegacyRestErrMessages() {
	val := s.network.Validators[0]

	// Produce the CLI fixtures with the same registered interfaces as the app.
	solo := ibctesting.NewSolomachine(s.T(), val.ClientCtx.Codec, "06-solomachine-0", "testing", 1)
	clientState := solo.ClientState()
	clientJSON, err := val.ClientCtx.Codec.MarshalInterfaceJSON(clientState)
	s.Require().NoError(err)
	clientStateJSON := testutil.WriteToNewTempFile(s.T(), string(clientJSON))

	badClientState := solo.ClientState()
	badClientState.ConsensusState.Diversifier = "DIFFERENT"
	badJSON, err := val.ClientCtx.Codec.MarshalInterfaceJSON(badClientState)
	s.Require().NoError(err)
	badClientStateJSON := testutil.WriteToNewTempFile(s.T(), string(badJSON))

	consensus, err := val.ClientCtx.Codec.MarshalInterfaceJSON(solo.ConsensusState())
	s.Require().NoError(err)
	consensusJSON := testutil.WriteToNewTempFile(s.T(), string(consensus))

	testCases := []struct {
		desc string
		cmd  *cobra.Command
		args []string
		code uint32
	}{
		{
			"Failing IBC message",
			ibcclientcli.NewCreateClientCmd(),
			[]string{
				badClientStateJSON.Name(), // path to client state json
				consensusJSON.Name(),      // path to consensus json,
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
				fmt.Sprintf("--gas=%d", flags.DefaultGasLimit),
				fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
				fmt.Sprintf("--%s=foobar", flags.FlagNote),
			},
			uint32(8),
		},
		{
			"Successful IBC message",
			ibcclientcli.NewCreateClientCmd(),
			[]string{
				clientStateJSON.Name(), // path to client state json
				consensusJSON.Name(),   // path to consensus json,
				fmt.Sprintf("--%s=true", flags.FlagSkipConfirmation),
				fmt.Sprintf("--%s=%s", flags.FlagBroadcastMode, flags.BroadcastBlock),
				fmt.Sprintf("--%s=%s", flags.FlagFees, sdk.NewCoins(sdk.NewCoin(s.cfg.BondDenom, sdk.NewInt(10))).String()),
				fmt.Sprintf("--gas=%d", flags.DefaultGasLimit),
				fmt.Sprintf("--%s=%s", flags.FlagFrom, val.Address.String()),
				fmt.Sprintf("--%s=foobar", flags.FlagNote),
			},
			uint32(0),
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.desc), func() {
			out, err := clitestutil.ExecTestCLICmd(val.ClientCtx, tc.cmd, tc.args)
			s.Require().NoError(err)
			var txRes sdk.TxResponse
			s.Require().NoError(val.ClientCtx.Codec.UnmarshalAsJSON(out.Bytes(), &txRes))
			s.Require().Equal(tc.code, txRes.Code, txRes.RawLog)
			s.Require().Positive(txRes.Height)
			s.Require().NotEmpty(txRes.TxHash)

			s.Require().NoError(s.network.WaitForNextBlock())

			s.testQueryIBCTx(txRes, tc.cmd, tc.args)
		})
	}
}

// testQueryIBCTx is a helper function to test querying txs which:
// - show an error message on legacy REST endpoints
// - succeed using gRPC
// In practice, we call this function on IBC txs.
func (s *IntegrationTestSuite) testQueryIBCTx(txRes sdk.TxResponse, cmd *cobra.Command, args []string) {
	val := s.network.Validators[0]

	errMsg := "this transaction cannot be displayed via legacy REST endpoints, because it does not support" +
		" Amino serialization. Please either use CLI, gRPC, gRPC-gateway, or directly query the Tendermint RPC" +
		" endpoint to query this transaction. The new REST endpoint (via gRPC-gateway) is "

	// Test that legacy endpoint return the above error message on IBC txs.
	testCases := []struct {
		desc string
		url  string
	}{
		{
			"Query by hash",
			fmt.Sprintf("%s/txs/%s", val.APIAddress, txRes.TxHash),
		},
		{
			"Query by height",
			fmt.Sprintf("%s/txs?tx.height=%d", val.APIAddress, txRes.Height),
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.desc), func() {
			txJSON, err := rest.GetRequest(tc.url)
			s.Require().NoError(err)

			var errResp rest.ErrorResponse
			s.Require().NoError(val.ClientCtx.LegacyAmino.UnmarshalAsJSON(txJSON, &errResp))

			s.Require().Contains(errResp.Error, errMsg)
		})
	}

	// try fetching the txn using gRPC req, it will fetch info since it has proto codec.
	grpcJSON, err := rest.GetRequest(fmt.Sprintf("%s/cosmos/tx/v1beta1/txs/%s", val.APIAddress, txRes.TxHash))
	s.Require().NoError(err)

	var getTxRes txtypes.GetTxResponse
	s.Require().NoError(val.ClientCtx.Codec.UnmarshalAsJSON(grpcJSON, &getTxRes))
	s.Require().Equal(getTxRes.Tx.Body.Memo, "foobar")
	s.Require().Equal(txRes.TxHash, getTxRes.TxResponse.TxHash)
	s.Require().Equal(txRes.Height, getTxRes.TxResponse.Height)
	s.Require().Equal(txRes.Code, getTxRes.TxResponse.Code)
	s.Require().Len(getTxRes.Tx.Signatures, 1)
	s.Require().NotEmpty(getTxRes.Tx.Signatures[0])

	// generate broadcast only txn.
	args = append(args, fmt.Sprintf("--%s=true", flags.FlagGenerateOnly))
	out, err := clitestutil.ExecTestCLICmd(val.ClientCtx, cmd, args)
	s.Require().NoError(err)

	txFile := testutil.WriteToNewTempFile(s.T(), string(out.Bytes()))
	txFileName := txFile.Name()

	// encode the generated txn.
	out, err = clitestutil.ExecTestCLICmd(val.ClientCtx, authcli.GetEncodeCommand(), []string{txFileName})
	s.Require().NoError(err)

	bz, err := val.ClientCtx.LegacyAmino.MarshalAsJSON(authrest.DecodeReq{Tx: string(out.Bytes())})
	s.Require().NoError(err)

	// try to decode the txn using legacy rest, it fails.
	res, err := rest.PostRequest(fmt.Sprintf("%s/txs/decode", val.APIAddress), "application/json", bz)
	s.Require().NoError(err)

	var errResp rest.ErrorResponse
	s.Require().NoError(val.ClientCtx.LegacyAmino.UnmarshalAsJSON(res, &errResp))
	s.Require().Contains(errResp.Error, errMsg)
}
