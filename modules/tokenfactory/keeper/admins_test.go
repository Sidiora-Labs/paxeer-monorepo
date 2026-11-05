package keeper_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tfcli "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	clienttx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	"github.com/stretchr/testify/require"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
)

func (suite *KeeperTestSuite) TestAdminMsgs() {
	addr0bal := int64(0)
	addr1bal := int64(0)

	suite.CreateDefaultDenom()
	// Make sure that the admin is set correctly
	queryRes, err := suite.queryClient.DenomAuthorityMetadata(suite.Ctx.Context(), &types.QueryDenomAuthorityMetadataRequest{
		Denom: suite.defaultDenom,
	})
	suite.Require().NoError(err)
	suite.Require().Equal(suite.TestAccs[0].String(), queryRes.AuthorityMetadata.Admin)

	// Test minting to admins own account
	_, err = suite.msgServer.Mint(sdk.WrapSDKContext(suite.Ctx), types.NewMsgMint(suite.TestAccs[0].String(), sdk.NewInt64Coin(suite.defaultDenom, 10)))

	addr0bal += 10
	suite.Require().NoError(err)
	suite.Require().True(suite.App.BankKeeper.GetBalance(suite.Ctx, suite.TestAccs[0], suite.defaultDenom).Amount.Int64() == addr0bal, suite.App.BankKeeper.GetBalance(suite.Ctx, suite.TestAccs[0], suite.defaultDenom))

	// Test burning from own account
	_, err = suite.msgServer.Burn(sdk.WrapSDKContext(suite.Ctx), types.NewMsgBurn(suite.TestAccs[0].String(), sdk.NewInt64Coin(suite.defaultDenom, 5)))

	addr0bal -= 5
	suite.Require().NoError(err)
	suite.Require().True(suite.App.BankKeeper.GetBalance(suite.Ctx, suite.TestAccs[1], suite.defaultDenom).Amount.Int64() == addr1bal)

	// Test Change Admin
	_, err = suite.msgServer.ChangeAdmin(sdk.WrapSDKContext(suite.Ctx), types.NewMsgChangeAdmin(suite.TestAccs[0].String(), suite.defaultDenom, suite.TestAccs[1].String()))
	suite.Require().NoError(err)
	queryRes, err = suite.queryClient.DenomAuthorityMetadata(suite.Ctx.Context(), &types.QueryDenomAuthorityMetadataRequest{
		Denom: suite.defaultDenom,
	})
	suite.Require().NoError(err)
	suite.Require().Equal(suite.TestAccs[1].String(), queryRes.AuthorityMetadata.Admin)

	// Make sure old admin can no longer do actions
	_, err = suite.msgServer.Burn(sdk.WrapSDKContext(suite.Ctx), types.NewMsgBurn(suite.TestAccs[0].String(), sdk.NewInt64Coin(suite.defaultDenom, 5)))

	suite.Require().Error(err)

	// Make sure the new admin works
	_, err = suite.msgServer.Mint(sdk.WrapSDKContext(suite.Ctx), types.NewMsgMint(suite.TestAccs[1].String(), sdk.NewInt64Coin(suite.defaultDenom, 5)))

	addr1bal += 5
	suite.Require().NoError(err)
	suite.Require().True(suite.App.BankKeeper.GetBalance(suite.Ctx, suite.TestAccs[1], suite.defaultDenom).Amount.Int64() == addr1bal)

	// Try setting admin to empty
	renounce := types.NewMsgChangeAdmin(suite.TestAccs[1].String(), suite.defaultDenom, "")
	suite.Require().NoError(renounce.ValidateBasic())
	_, err = suite.msgServer.ChangeAdmin(sdk.WrapSDKContext(suite.Ctx), renounce)

	suite.Require().NoError(err)
	queryRes, err = suite.queryClient.DenomAuthorityMetadata(suite.Ctx.Context(), &types.QueryDenomAuthorityMetadataRequest{
		Denom: suite.defaultDenom,
	})
	suite.Require().NoError(err)
	suite.Require().Equal("", queryRes.AuthorityMetadata.Admin)
}

func (suite *KeeperTestSuite) TestChangeAdminDenom() {
	for _, tc := range []struct {
		desc                    string
		msgChangeAdmin          func(denom string) *types.MsgChangeAdmin
		expectedChangeAdminPass bool
		expectedAdminIndex      int
		msgMint                 func(denom string) *types.MsgMint
		expectedMintPass        bool
	}{
		{
			desc: "creator admin can't mint after setting to '' ",
			msgChangeAdmin: func(denom string) *types.MsgChangeAdmin {
				return types.NewMsgChangeAdmin(suite.TestAccs[0].String(), denom, "")
			},
			expectedChangeAdminPass: true,
			expectedAdminIndex:      -1,
			msgMint: func(denom string) *types.MsgMint {
				return types.NewMsgMint(suite.TestAccs[0].String(), sdk.NewInt64Coin(denom, 5))
			},
			expectedMintPass: false,
		},
		{
			desc: "non-admins can't change the existing admin",
			msgChangeAdmin: func(denom string) *types.MsgChangeAdmin {
				return types.NewMsgChangeAdmin(suite.TestAccs[1].String(), denom, suite.TestAccs[2].String())
			},
			expectedChangeAdminPass: false,
			expectedAdminIndex:      0,
		},
		{
			desc: "change to same admin should fail",
			msgChangeAdmin: func(denom string) *types.MsgChangeAdmin {
				return types.NewMsgChangeAdmin(suite.TestAccs[0].String(), denom, suite.TestAccs[0].String())
			},
			expectedChangeAdminPass: false,
			expectedAdminIndex:      0,
		},
		{
			desc: "success change admin",
			msgChangeAdmin: func(denom string) *types.MsgChangeAdmin {
				return types.NewMsgChangeAdmin(suite.TestAccs[0].String(), denom, suite.TestAccs[1].String())
			},
			expectedAdminIndex:      1,
			expectedChangeAdminPass: true,
			msgMint: func(denom string) *types.MsgMint {
				return types.NewMsgMint(suite.TestAccs[1].String(), sdk.NewInt64Coin(denom, 5))
			},
			expectedMintPass: true,
		},
	} {
		suite.Run(fmt.Sprintf("Case %s", tc.desc), func() {
			// setup test
			suite.SetupTest()

			// Create a denom and mint
			res, err := suite.msgServer.CreateDenom(sdk.WrapSDKContext(suite.Ctx), types.NewMsgCreateDenom(suite.TestAccs[0].String(), "bitcoin"))
			suite.Require().NoError(err)

			testDenom := res.GetNewTokenDenom()

			_, err = suite.msgServer.Mint(sdk.WrapSDKContext(suite.Ctx), types.NewMsgMint(suite.TestAccs[0].String(), sdk.NewInt64Coin(testDenom, 10)))

			suite.Require().NoError(err)

			_, err = suite.msgServer.ChangeAdmin(sdk.WrapSDKContext(suite.Ctx), tc.msgChangeAdmin(testDenom))

			if tc.expectedChangeAdminPass {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
			}

			queryRes, err := suite.queryClient.DenomAuthorityMetadata(suite.Ctx.Context(), &types.QueryDenomAuthorityMetadataRequest{
				Denom: testDenom,
			})
			suite.Require().NoError(err)

			// expectedAdminIndex with negative value is assumed as admin with value of ""
			const emptyStringAdminIndexFlag = -1
			if tc.expectedAdminIndex == emptyStringAdminIndexFlag {
				suite.Require().Equal("", queryRes.AuthorityMetadata.Admin)
			} else {
				suite.Require().Equal(suite.TestAccs[tc.expectedAdminIndex].String(), queryRes.AuthorityMetadata.Admin)
			}

			// we test mint to test if admin authority is performed properly after admin change.
			if tc.msgMint != nil {
				_, err := suite.msgServer.Mint(sdk.WrapSDKContext(suite.Ctx), tc.msgMint(testDenom))

				if tc.expectedMintPass {
					suite.Require().NoError(err)
				} else {
					suite.Require().Error(err)
				}
			}
		})
	}
}

func (suite *KeeperTestSuite) TestSetDenomMetaData() {

	// setup test
	suite.SetupTest()
	suite.CreateDefaultDenom()

	for _, tc := range []struct {
		desc                string
		msgSetDenomMetadata types.MsgSetDenomMetadata
		expectedPass        bool
	}{
		{
			desc: "successful set denom metadata",
			msgSetDenomMetadata: *types.NewMsgSetDenomMetadata(suite.TestAccs[0].String(), banktypes.Metadata{
				Description: "test1",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    suite.defaultDenom,
						Exponent: 0,
					},
					{
						Denom:    "uhpx",
						Exponent: 6,
					},
				},
				Base:    suite.defaultDenom,
				Display: "uhpx",
				Name:    "PAX",
				Symbol:  "PAX",
			}),
			expectedPass: true,
		},
		{
			desc: "non existent factory denom name",
			msgSetDenomMetadata: *types.NewMsgSetDenomMetadata(suite.TestAccs[0].String(), banktypes.Metadata{
				Description: "test2",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    fmt.Sprintf("factory/%s/litecoin", suite.TestAccs[0].String()),
						Exponent: 0,
					},
					{
						Denom:    "uhpx",
						Exponent: 6,
					},
				},
				Base:    fmt.Sprintf("factory/%s/litecoin", suite.TestAccs[0].String()),
				Display: "uhpx",
				Name:    "PAX",
				Symbol:  "PAX",
			}),
			expectedPass: false,
		},
		{
			desc: "non-factory denom",
			msgSetDenomMetadata: *types.NewMsgSetDenomMetadata(suite.TestAccs[0].String(), banktypes.Metadata{
				Description: "test3",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    "uhpx",
						Exponent: 0,
					},
					{
						Denom:    "uhpx",
						Exponent: 6,
					},
				},
				Base:    "uhpx",
				Display: "uhpx",
				Name:    "PAX",
				Symbol:  "PAX",
			}),
			expectedPass: false,
		},
		{
			desc: "wrong admin",
			msgSetDenomMetadata: *types.NewMsgSetDenomMetadata(suite.TestAccs[1].String(), banktypes.Metadata{
				Description: "test4",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    suite.defaultDenom,
						Exponent: 0,
					},
					{
						Denom:    "uhpx",
						Exponent: 6,
					},
				},
				Base:    suite.defaultDenom,
				Display: "uhpx",
				Name:    "PAX",
				Symbol:  "PAX",
			}),
			expectedPass: false,
		},
		{
			desc: "invalid metadata (missing display denom unit)",
			msgSetDenomMetadata: *types.NewMsgSetDenomMetadata(suite.TestAccs[0].String(), banktypes.Metadata{
				Description: "test5",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    suite.defaultDenom,
						Exponent: 0,
					},
				},
				Base:    suite.defaultDenom,
				Display: "uhpx",
				Name:    "PAX",
				Symbol:  "PAX",
			}),
			expectedPass: false,
		},
	} {
		suite.Run(fmt.Sprintf("Case %s", tc.desc), func() {
			res, err := suite.msgServer.SetDenomMetadata(sdk.WrapSDKContext(suite.Ctx), &tc.msgSetDenomMetadata)
			if tc.expectedPass {
				suite.Require().NoError(err)
				suite.Require().NotNil(res)

				md, found := suite.App.BankKeeper.GetDenomMetaData(suite.Ctx, suite.defaultDenom)
				suite.Require().True(found)
				suite.Require().Equal(tc.msgSetDenomMetadata.Metadata.Name, md.Name)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func adminsIssuerSuite(t *testing.T) *KeeperTestSuite {
	s := new(KeeperTestSuite)
	s.SetT(t)
	s.SetupTest()
	s.CreateDefaultDenom()
	return s
}

func adminsIssuerDid(fill byte) string {
	var key [32]byte
	for i := range key {
		key[i] = fill
	}
	return evmtypes.LayerXDid(key)
}

// adminsRequireIssuerDid observes the exported keeper and the same routed query
// used by the admin consumers, issuing a fresh request on each invocation.
func adminsRequireIssuerDid(s *KeeperTestSuite, issuer types.IssuerKeeper, admin, did string) {
	s.T().Helper()
	stored, err := issuer.IssuerDid(s.Ctx, s.defaultDenom)
	s.Require().NoError(err)
	s.Require().Equal(did, stored)
	queried, err := s.queryClient.DenomAuthorityMetadata(s.Ctx.Context(),
		&types.QueryDenomAuthorityMetadataRequest{Denom: s.defaultDenom})
	s.Require().NoError(err)
	s.Require().Equal(types.DenomAuthorityMetadata{Admin: admin, IssuerDid: did}, queried.AuthorityMetadata)
}

func TestIssuerDidValidate(t *testing.T) {
	s := adminsIssuerSuite(t)
	k := s.App.TokenFactoryKeeper
	admin := s.TestAccs[0].String()
	issuer := adminsIssuerDid(0xab)
	s.Require().Equal(evmtypes.LayerXDidPrefix+strings.Repeat("ab", 32), issuer)

	existing, err := k.GetAuthorityMetadata(s.Ctx, s.defaultDenom)
	s.Require().NoError(err)
	s.Require().Equal(types.DenomAuthorityMetadata{Admin: admin}, existing)
	s.Require().NoError(existing.Validate())
	did, err := k.IssuerDid(s.Ctx, s.defaultDenom)
	s.Require().NoError(err)
	s.Require().Equal("", did)

	for _, refused := range []string{
		issuer[:len(issuer)-1],
		issuer + "a",
		evmtypes.LayerXDidPrefix + strings.Repeat("AB", 32),
		evmtypes.LayerXDidPrefix + strings.Repeat("g", 64),
		"did:other:" + strings.Repeat("ab", 32),
		strings.Repeat("ab", 32),
	} {
		metadata := types.DenomAuthorityMetadata{Admin: admin, IssuerDid: refused}
		s.Require().ErrorIs(metadata.Validate(), types.ErrInvalidAuthorityMetadata, refused)
		s.Require().ErrorIs(k.SetIssuerDid(s.Ctx, s.defaultDenom, refused), types.ErrInvalidAuthorityMetadata, refused)
		did, err = k.IssuerDid(s.Ctx, s.defaultDenom)
		s.Require().NoError(err)
		s.Require().Equal("", did)
	}

	s.Require().NoError(types.DenomAuthorityMetadata{Admin: admin, IssuerDid: issuer}.Validate())
	s.Require().NoError(k.SetIssuerDid(s.Ctx, s.defaultDenom, issuer))
	stored, err := k.GetAuthorityMetadata(s.Ctx, s.defaultDenom)
	s.Require().NoError(err)
	s.Require().Equal(types.DenomAuthorityMetadata{Admin: admin, IssuerDid: issuer}, stored)
	adminsRequireIssuerDid(s, k, admin, issuer)

	s.Require().NoError(k.SetIssuerDid(s.Ctx, s.defaultDenom, issuer))
	s.Require().ErrorIs(k.SetIssuerDid(s.Ctx, s.defaultDenom, adminsIssuerDid(0xcd)), types.ErrInvalidAuthorityMetadata)
	s.Require().ErrorIs(k.SetIssuerDid(s.Ctx, s.defaultDenom, ""), types.ErrInvalidAuthorityMetadata)
	did, err = k.IssuerDid(s.Ctx, s.defaultDenom)
	s.Require().NoError(err)
	s.Require().Equal(issuer, did)

	missing := fmt.Sprintf("factory/%s/missing", admin)
	s.Require().ErrorIs(k.SetIssuerDid(s.Ctx, missing, issuer), types.ErrDenomDoesNotExist)
	did, err = k.IssuerDid(s.Ctx, missing)
	s.Require().NoError(err)
	s.Require().Equal("", did)
}

func TestIssuerDidSurvivesChangeAdmin(t *testing.T) {
	s := adminsIssuerSuite(t)
	k := s.App.TokenFactoryKeeper
	issuer := adminsIssuerDid(0x5a)
	s.Require().NoError(k.SetIssuerDid(s.Ctx, s.defaultDenom, issuer))

	for _, step := range []struct {
		sender   string
		newAdmin string
	}{
		{s.TestAccs[0].String(), s.TestAccs[1].String()},
		{s.TestAccs[1].String(), ""},
	} {
		msg := types.NewMsgChangeAdmin(step.sender, s.defaultDenom, step.newAdmin)
		s.Require().NoError(msg.ValidateBasic())
		_, err := s.msgServer.ChangeAdmin(sdk.WrapSDKContext(s.Ctx), msg)
		s.Require().NoError(err)

		stored, err := k.GetAuthorityMetadata(s.Ctx, s.defaultDenom)
		s.Require().NoError(err)
		s.Require().Equal(types.DenomAuthorityMetadata{Admin: step.newAdmin, IssuerDid: issuer}, stored)

		queried, err := s.queryClient.DenomAuthorityMetadata(s.Ctx.Context(), &types.QueryDenomAuthorityMetadataRequest{Denom: s.defaultDenom})
		s.Require().NoError(err)
		s.Require().Equal(issuer, queried.AuthorityMetadata.IssuerDid)
		s.Require().Equal(step.newAdmin, queried.AuthorityMetadata.Admin)

		did, err := k.IssuerDid(s.Ctx, s.defaultDenom)
		s.Require().NoError(err)
		s.Require().Equal(issuer, did)
		s.Require().ErrorIs(k.SetIssuerDid(s.Ctx, s.defaultDenom, adminsIssuerDid(0x01)), types.ErrInvalidAuthorityMetadata)
	}
}

func TestAdminSignedTransactionRenunciation(t *testing.T) {
	s := new(KeeperTestSuite)
	s.SetT(t)
	s.SetupTest()
	t.Cleanup(func() { require.NoError(t, s.App.Close()) })
	first := &secp256k1.PrivKey{Key: bytes.Repeat([]byte{1}, 32)}
	second := &secp256k1.PrivKey{Key: bytes.Repeat([]byte{2}, 32)}
	address := func(key *secp256k1.PrivKey) sdk.AccAddress { return sdk.AccAddress(key.PubKey().Address()) }
	s.TestAccs[0], s.TestAccs[1] = address(first), address(second)
	for _, addr := range s.TestAccs[:2] {
		s.FundAcc(addr, sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 1_000_000_000)))
	}
	s.CreateDefaultDenom()
	did := adminsIssuerDid(0x5a)
	issuer := s.App.TokenFactoryKeeper
	require.NoError(t, issuer.SetIssuerDid(s.Ctx, s.defaultDenom, did))
	require.NoError(t, issuer.MintByIssuer(s.Ctx, did, sdk.NewInt64Coin(s.defaultDenom, 37), address(first).String()))
	supply := s.App.BankKeeper.GetSupply(s.Ctx, s.defaultDenom)
	firstBalance := s.App.BankKeeper.GetBalance(s.Ctx, address(first), s.defaultDenom)
	secondBalance := s.App.BankKeeper.GetBalance(s.Ctx, address(second), s.defaultDenom)
	observe := func(admin string) {
		t.Helper()
		adminsRequireIssuerDid(s, issuer, admin, did)
		require.Equal(t, supply, s.App.BankKeeper.GetSupply(s.Ctx, s.defaultDenom))
		require.Equal(t, firstBalance, s.App.BankKeeper.GetBalance(s.Ctx, address(first), s.defaultDenom))
		require.Equal(t, secondBalance, s.App.BankKeeper.GetBalance(s.Ctx, address(second), s.defaultDenom))
	}
	deliver := func(key *secp256k1.PrivKey, newAdmin string) abci.ResponseDeliverTx {
		t.Helper()
		// Let the production CLI construct the transaction, including an empty admin.
		config := s.App.GetTxConfig()
		var output bytes.Buffer
		clientCtx := client.Context{}.WithCodec(s.App.AppCodec()).WithInterfaceRegistry(s.App.InterfaceRegistry()).
			WithTxConfig(config).WithKeyring(keyring.NewInMemory()).WithChainID(s.Ctx.ChainID()).
			WithFrom(address(key).String()).WithFromAddress(address(key)).WithGenerateOnly(true).WithOffline(true).WithOutput(&output)
		cmd := tfcli.NewChangeAdminCmd()
		cmd.SilenceUsage, cmd.SilenceErrors = true, true
		cmd.SetOut(&output)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx))
		cmd.SetArgs([]string{s.defaultDenom, newAdmin})
		require.NoError(t, cmd.Execute())
		produced, err := config.TxJSONDecoder()(output.Bytes())
		require.NoError(t, err)
		builder, err := config.WrapTxBuilder(produced)
		require.NoError(t, err)
		require.Len(t, produced.GetMsgs(), 1)
		require.Equal(t, types.NewMsgChangeAdmin(address(key).String(), s.defaultDenom, newAdmin), produced.GetMsgs()[0])
		account := s.App.AccountKeeper.GetAccount(s.Ctx, address(key))
		require.NotNil(t, account)
		data := authsigning.SignerData{ChainID: s.Ctx.ChainID(), AccountNumber: account.GetAccountNumber(), Sequence: account.GetSequence()}
		mode := config.SignModeHandler().DefaultMode()
		builder.SetGasLimit(500_000)
		builder.SetFeeAmount(sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 50_000)))
		require.NoError(t, builder.SetSignatures(signing.SignatureV2{
			PubKey: key.PubKey(), Data: &signing.SingleSignatureData{SignMode: mode}, Sequence: data.Sequence,
		}))
		signature, err := clienttx.SignWithPrivKey(mode, data, builder, key, config, data.Sequence)
		require.NoError(t, err)
		require.NoError(t, builder.SetSignatures(signature))
		wire, err := config.TxEncoder()(builder.GetTx())
		require.NoError(t, err)
		received, err := config.TxDecoder()(wire)
		require.NoError(t, err)
		// DeliverTx performs stateless validation, verifies the actual signature and
		// routes the decoded message through the application's registered handler.
		result := s.App.DeliverTx(s.Ctx, abci.RequestDeliverTxV2{Tx: wire}, received, sha256.Sum256(wire))
		after := s.App.AccountKeeper.GetAccount(s.Ctx, address(key))
		require.Equal(t, data.Sequence+1, after.GetSequence(), "transaction must pass signature verification and ante handling: %s", result.Log)
		return result
	}
	observe(address(first).String())
	unauthorized := deliver(second, "")
	require.Equal(t, types.ErrUnauthorized.ABCICode(), unauthorized.Code, unauthorized.Log)
	require.Equal(t, types.ErrUnauthorized.Codespace(), unauthorized.Codespace)
	observe(address(first).String())
	for _, step := range []struct {
		key   *secp256k1.PrivKey
		admin string
	}{{first, address(second).String()}, {second, ""}} {
		result := deliver(step.key, step.admin)
		require.Zero(t, result.Code, result.Log)
		var changes []abci.Event
		for _, event := range result.Events {
			if event.Type == types.TypeMsgChangeAdmin {
				changes = append(changes, event)
			}
		}
		require.Len(t, changes, 1)
		attributes := map[string]string{}
		for _, attr := range changes[0].Attributes {
			attributes[string(attr.Key)] = string(attr.Value)
		}
		require.Equal(t, s.defaultDenom, attributes[types.AttributeDenom])
		require.Equal(t, step.admin, attributes[types.AttributeNewAdmin])
		observe(step.admin)
	}
	for _, key := range []*secp256k1.PrivKey{first, second} {
		result := deliver(key, address(key).String())
		require.Equal(t, types.ErrUnauthorized.ABCICode(), result.Code, result.Log)
		require.Equal(t, types.ErrUnauthorized.Codespace(), result.Codespace)
		for _, event := range result.Events {
			require.NotEqual(t, types.TypeMsgChangeAdmin, event.Type)
		}
		observe("")
	}
}
