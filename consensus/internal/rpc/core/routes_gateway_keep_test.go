package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpc "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestRoutesMapKeepsGatewayConsensusMethods(t *testing.T) {
	key := ed25519.GenerateSecretKey()
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	state, err := sm.MakeGenesisState(&types.GenesisDoc{
		ChainID:     "gateway-consensus-routes",
		GenesisTime: now,
		Validators: []types.GenesisValidator{{
			Address: key.Public().Address(),
			PubKey:  key.Public(),
			Power:   10,
		}},
	})
	require.NoError(t, err)
	stateStore := sm.NewStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, stateStore.Close()) })
	require.NoError(t, stateStore.Save(state))
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, blockStore.Close()) })
	block := state.MakeBlockAt(1, []types.Tx{types.Tx("gateway=ready")}, &types.Commit{Signatures: []types.CommitSig{}}, nil, key.Public().Address(), now)
	parts, err := block.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(t, err)
	blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: parts.Header()}
	dir := t.TempDir()
	validator := privval.NewFilePV(key, filepath.Join(dir, "key.json"), filepath.Join(dir, "state.json"))
	votes := types.NewVoteSet(state.ChainID, 1, 0, tmproto.PrecommitType, state.Validators)
	commit, err := types.MakeCommit(t.Context(), blockID, 1, 0, votes, []types.PrivValidator{validator}, now)
	require.NoError(t, err)
	blockStore.SaveBlock(block, parts, commit)
	env := &Environment{StateStore: stateStore, BlockStore: blockStore}

	for _, options := range []struct {
		name string
		opts *RouteOptions
	}{
		{name: "nil"},
		{name: "defaults", opts: &RouteOptions{}},
	} {
		t.Run(options.name, func(t *testing.T) {
			routes := NewRoutesMap(env, options.opts)
			require.NotContains(t, routes, "unsafe_flush_mempool")
			mux := http.NewServeMux()
			rpc.RegisterRPCFuncs(mux, routes)
			for _, method := range []struct {
				name string
				want any
			}{
				{name: "block", want: &coretypes.ResultBlock{BlockID: blockID, Block: block}},
				{name: "commit", want: coretypes.NewResultCommit(&block.Header, commit, false)},
				{name: "validators", want: &coretypes.ResultValidators{
					BlockHeight: 1,
					Validators:  state.Validators.Validators,
					Count:       1,
					Total:       1,
				}},
			} {
				t.Run(method.name, func(t *testing.T) {
					require.Contains(t, routes, method.name)
					require.NotNil(t, routes[method.name])
					result, err := routes[method.name].Call(t.Context(), json.RawMessage(`{"height":"1"}`))
					require.NoError(t, err)
					require.IsType(t, method.want, result)
					wantJSON, err := json.Marshal(method.want)
					require.NoError(t, err)
					resultJSON, err := json.Marshal(result)
					require.NoError(t, err)
					require.JSONEq(t, string(wantJSON), string(resultJSON))

					body := `{"jsonrpc":"2.0","id":1,"method":"` + method.name + `","params":{"height":"1"}}`
					request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body))
					request.Header.Set("Content-Type", "application/json")
					response := httptest.NewRecorder()
					mux.ServeHTTP(response, request)
					require.Equal(t, http.StatusOK, response.Code)
					var envelope struct {
						ID     int             `json:"id"`
						Result json.RawMessage `json:"result"`
						Error  json.RawMessage `json:"error"`
					}
					require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
					require.Equal(t, 1, envelope.ID)
					require.Empty(t, envelope.Error)
					require.JSONEq(t, string(wantJSON), string(envelope.Result))
				})
			}
		})
	}
}
