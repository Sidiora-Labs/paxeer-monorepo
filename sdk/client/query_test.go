//go:build norace
// +build norace

package client_test

import (
	"context"
	"fmt"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	rpcclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
)

func (s *IntegrationTestSuite) TestQueryABCIHeight() {

	testCases := []struct {
		name      string
		reqHeight int64
		ctxHeight int64
		expHeight int64
	}{
		{
			name:      "non zero request height",
			reqHeight: 3,
			ctxHeight: 1, // query at height 1 or 2 would cause an error
			expHeight: 3,
		},
		{
			name:      "empty request height - use context height",
			reqHeight: 0,
			ctxHeight: 3,
			expHeight: 3,
		},
		{
			name:      "empty request height and context height - use latest height",
			reqHeight: 0,
			ctxHeight: 0,
			expHeight: 4,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.network.WaitForHeight(tc.expHeight)

			val := s.network.Validators[0]

			clientCtx := val.ClientCtx
			clientCtx = clientCtx.WithHeight(tc.ctxHeight)

			req := abci.RequestQuery{
				Path:   fmt.Sprintf("/store/%s/key", banktypes.StoreKey),
				Height: tc.reqHeight,
				Data:   banktypes.CreateAccountBalancesPrefix(val.Address),
				Prove:  false,
			}

			res, err := clientCtx.QueryABCI(req)
			s.Require().NoError(err)

			s.Require().Equal(tc.expHeight, res.Height)

			req.Prove = true
			refused, err := clientCtx.QueryABCI(req)
			s.Require().ErrorIs(err, client.ErrQueryProofVerificationUnavailable)
			s.Require().Empty(refused.Value)

			node, err := clientCtx.GetNode()
			s.Require().NoError(err)
			proof, err := node.ABCIQueryWithOptions(context.Background(), req.Path, req.Data, rpcclient.ABCIQueryOptions{Height: tc.expHeight, Prove: true})
			s.Require().NoError(err)
			s.Require().Zero(proof.Response.Code)
			s.Require().Equal(tc.expHeight, proof.Response.Height)
			s.Require().NotNil(proof.Response.ProofOps)
			s.Require().NotEmpty(proof.Response.ProofOps.Ops)
		})
	}
}
