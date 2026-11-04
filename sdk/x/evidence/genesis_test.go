package evidence_test

import (
	"fmt"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/stretchr/testify/suite"

	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/exported"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/types"
)

type GenesisTestSuite struct {
	suite.Suite

	cdc    codec.Codec
	ctx    sdk.Context
	keeper keeper.Keeper
}

func (suite *GenesisTestSuite) SetupTest() {
	checkTx := false
	app := paxapp.Setup(suite.T(), checkTx, false, false)

	suite.cdc = app.AppCodec()
	suite.ctx = app.BaseApp.NewContext(checkTx, tmproto.Header{Height: 1})
	suite.keeper = app.EvidenceKeeper
}

func (suite *GenesisTestSuite) TestInitGenesis() {
	var (
		genesisState *types.GenesisState
		testEvidence []exported.Evidence
		pk           = ed25519.GenPrivKey()
	)

	testCases := []struct {
		msg       string
		malleate  func()
		expPass   bool
		posttests func()
	}{
		{
			"valid",
			func() {
				testEvidence = make([]exported.Evidence, 100)
				for i := 0; i < 100; i++ {
					testEvidence[i] = types.FromABCIEvidence(abci.Evidence{
						Height: int64(i + 1), Time: time.Now().UTC(),
						Validator: abci.Validator{Address: pk.PubKey().Address(), Power: 100},
					})
				}
				genesisState = types.NewGenesisState(testEvidence)
			},
			true,
			func() {
				for _, e := range testEvidence {
					stored, ok := suite.keeper.GetEvidence(suite.ctx, e.Hash())
					suite.Require().True(ok)
					suite.Equal(pk.PubKey().Address().Bytes(), stored.(*types.Equivocation).GetConsensusAddress().Bytes())
					suite.Equal(e, stored)
				}
			},
		},
		{
			"invalid",
			func() {
				testEvidence = make([]exported.Evidence, 100)
				for i := 0; i < 100; i++ {
					testEvidence[i] = types.FromABCIEvidence(abci.Evidence{
						Time:      time.Now().UTC(),
						Validator: abci.Validator{Address: pk.PubKey().Address(), Power: 100},
					})
				}
				genesisState = types.NewGenesisState(testEvidence)
			},
			false,
			func() {
				suite.Empty(suite.keeper.GetAllEvidence(suite.ctx))
			},
		},
		{
			"malformed consensus address",
			func() {
				e := types.FromABCIEvidence(abci.Evidence{
					Height: 1, Time: time.Now().UTC(),
					Validator: abci.Validator{Address: pk.PubKey().Address(), Power: 100},
				}).(*types.Equivocation)
				e.ConsensusAddress = pk.PubKey().Address().String()
				suite.Require().ErrorIs(e.ValidateBasic(), types.ErrInvalidEvidence)
				genesisState = types.NewGenesisState([]exported.Evidence{e})
			},
			false,
			func() { suite.Empty(suite.keeper.GetAllEvidence(suite.ctx)) },
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			suite.SetupTest()

			tc.malleate()
			var decoded types.GenesisState
			suite.Require().NoError(suite.cdc.UnmarshalAsJSON(suite.cdc.MustMarshalJSON(genesisState), &decoded))
			genesisState = &decoded

			if tc.expPass {
				suite.NotPanics(func() {
					evidence.InitGenesis(suite.ctx, suite.keeper, genesisState)
				})
			} else {
				suite.Panics(func() {
					evidence.InitGenesis(suite.ctx, suite.keeper, genesisState)
				})
			}

			tc.posttests()
		})
	}
}

func (suite *GenesisTestSuite) TestExportGenesis() {
	pk := ed25519.GenPrivKey()

	testCases := []struct {
		msg       string
		malleate  func()
		expPass   bool
		posttests func()
	}{
		{
			"success",
			func() {
				suite.keeper.SetEvidence(suite.ctx, types.FromABCIEvidence(abci.Evidence{
					Height: 1, Time: time.Now().UTC(),
					Validator: abci.Validator{Address: pk.PubKey().Address(), Power: 100},
				}))
			},
			true,
			func() {},
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			suite.SetupTest()

			tc.malleate()

			if tc.expPass {
				suite.NotPanics(func() {
					exportedState := evidence.ExportGenesis(suite.ctx, suite.keeper)
					var decoded types.GenesisState
					suite.Require().NoError(suite.cdc.UnmarshalAsJSON(suite.cdc.MustMarshalJSON(exportedState), &decoded))
					suite.Require().NoError(decoded.Validate())
					suite.Require().Len(decoded.Evidence, 1)
					stored := decoded.Evidence[0].GetCachedValue().(*types.Equivocation)
					suite.Equal(pk.PubKey().Address().Bytes(), stored.GetConsensusAddress().Bytes())
				})
			} else {
				suite.Panics(func() {
					evidence.ExportGenesis(suite.ctx, suite.keeper)
				})
			}

			tc.posttests()
		})
	}
}

func TestGenesisTestSuite(t *testing.T) {
	suite.Run(t, new(GenesisTestSuite))
}
