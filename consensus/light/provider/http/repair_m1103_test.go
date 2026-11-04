package http_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/provider"
	lighthttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/provider/http"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/version"
)

func TestRepairM_1103_MalformedHTTPReplies(t *testing.T) {
	for _, height := range []int64{42, 0} {
		for _, tc := range []struct {
			name   string
			result string
		}{
			{"null_result", `null`},
			{"missing_signed_header", `{}`},
			{"null_signed_header", `{"signed_header":null}`},
			{"empty_signed_header", `{"signed_header":{}}`},
			{"missing_header", `{"signed_header":{"commit":null}}`},
			{"null_header", `{"signed_header":{"header":null,"commit":null}}`},
			{"commit_without_header", `{"signed_header":{"commit":{"height":"42","round":0,"block_id":{"hash":"","parts":{"total":0,"hash":""}},"signatures":[]}}}`},
		} {
			t.Run(fmt.Sprintf("height_%d/%s", height, tc.name), func(t *testing.T) {
				requests := make(chan rpctypes.RPCRequest, 4)
				server := repairM1103Server(t, requests, func(req rpctypes.RPCRequest) any {
					return json.RawMessage(tc.result)
				})
				p, err := lighthttp.NewWithOptions("repair-m1103", server.URL, lighthttp.Options{Timeout: time.Second})
				require.NoError(t, err)
				require.NotPanics(t, func() {
					lb, err := p.LightBlock(t.Context(), height)
					require.Nil(t, lb)
					var bad provider.ErrBadLightBlock
					require.ErrorAs(t, err, &bad)
					require.Error(t, bad.Reason)
				})
				require.Len(t, requests, 1, "malformed headers must be rejected before fetching validators or retrying")
				req := <-requests
				require.Equal(t, "commit", req.Method)
				repairM1103CheckHeight(t, req, height)
			})
		}
	}
}

func TestRepairM_1103_ValidHTTPReplies(t *testing.T) {
	const chainID = "repair-m1103"
	key := ed25519.TestSecretKey([]byte("repair-m1103-signed-http-fixture"))
	vals := types.NewValidatorSet([]*types.Validator{types.NewValidator(key.Public(), 10)})
	header := &types.Header{
		Version:            version.Consensus{Block: version.BlockProtocol},
		ChainID:            chainID,
		Height:             42,
		Time:               time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ValidatorsHash:     vals.Hash(),
		NextValidatorsHash: vals.Hash(),
		ProposerAddress:    key.Public().Address(),
	}
	blockID := types.BlockID{
		Hash:          header.Hash(),
		PartSetHeader: types.PartSetHeader{Total: 1, Hash: crypto.Checksum([]byte("repair-m1103-parts")).Bytes()},
	}
	vote := &types.Vote{
		Type:             tmproto.PrecommitType,
		Height:           header.Height,
		BlockID:          blockID,
		Timestamp:        header.Time,
		ValidatorAddress: key.Public().Address(),
	}
	vote.Signature = utils.Some(key.Sign(types.VoteSignBytes(chainID, vote.ToProto())))
	commit := &types.Commit{Height: header.Height, BlockID: blockID, Signatures: []types.CommitSig{vote.CommitSig()}}
	require.NoError(t, vals.VerifyCommit(chainID, blockID, header.Height, commit))
	result := coretypes.NewResultCommit(header, commit, true)
	require.NoError(t, result.SignedHeader.ValidateBasic(chainID))
	for _, height := range []int64{42, 0, 41} {
		t.Run(fmt.Sprintf("height_%d", height), func(t *testing.T) {
			requests := make(chan rpctypes.RPCRequest, 4)
			server := repairM1103Server(t, requests, func(req rpctypes.RPCRequest) any {
				if req.Method == "validators" {
					return &coretypes.ResultValidators{BlockHeight: header.Height, Validators: vals.Validators, Count: 1, Total: 1}
				}
				return result
			})
			p, err := lighthttp.NewWithOptions(chainID, server.URL, lighthttp.Options{Timeout: time.Second})
			require.NoError(t, err)
			lb, err := p.LightBlock(t.Context(), height)
			if height == 41 {
				require.Nil(t, lb)
				var bad provider.ErrBadLightBlock
				require.ErrorAs(t, err, &bad)
				require.Len(t, requests, 1)
			} else {
				require.NoError(t, err)
				require.NotNil(t, lb)
				require.NoError(t, lb.ValidateBasic(chainID))
				require.Equal(t, header.Hash(), lb.Hash())
				require.Equal(t, header.Height, lb.Height)
				require.NoError(t, lb.ValidatorSet.VerifyCommit(chainID, blockID, lb.Height, lb.Commit))
				require.Len(t, requests, 2)
			}
			req := <-requests
			require.Equal(t, "commit", req.Method)
			repairM1103CheckHeight(t, req, height)
			if height != 41 {
				req = <-requests
				require.Equal(t, "validators", req.Method)
				repairM1103CheckHeight(t, req, header.Height)
			}
		})
	}
}

func repairM1103Server(t *testing.T, requests chan<- rpctypes.RPCRequest, result func(rpctypes.RPCRequest) any) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpctypes.RPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			http.Error(w, "invalid JSON-RPC request", http.StatusBadRequest)
			return
		}
		requests <- req
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(req.MakeResponse(result(req))); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func repairM1103CheckHeight(t *testing.T, req rpctypes.RPCRequest, height int64) {
	t.Helper()
	var params coretypes.RequestBlockInfo
	require.NoError(t, json.Unmarshal(req.Params, &params))
	if height == 0 {
		require.Nil(t, params.Height)
	} else {
		require.NotNil(t, params.Height)
		require.Equal(t, height, int64(*params.Height))
	}
}
