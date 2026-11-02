// Proves: coretypes.ErrConsensusStateNotAvailable / coretypes.ErrEvidenceNotAccepted -> core.Environment / core.NewRoutesMap / JSON-RPC HTTP -> http.HTTP.ConsensusState / http.HTTP.BroadcastEvidence / types.RPCResponse.UnmarshalJSON
//
// A real, initialized Autobahn router activates the producers; the production
// routes and clients carry their results across a loopback HTTP connection.
// These refusal sentinels have no persistent state, so every observation is
// repeated and each request is counted at the server to exclude cached results.
// A valid response from the same server is then corrupted only at the error
// code field and refused by the production consumer's response decoder.
//
// The current wire limitation lives outside this task's scope:
// consensus/rpc/jsonrpc/types/types.go, RPCRequest.MakeError and RPCError,
// preserve the sentinel message as Data with CodeInternalError, but do not
// preserve errors.Is identity. The proof pins that behavior; callers must
// inspect the decoded RPCError rather than expect Go identity across JSON.
package coretypes_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"golang.org/x/time/rate"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	atypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/autobahn/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	aconsensus "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/consensus"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/autobahn/producer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/p2p/conn"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/rpc/core"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	tmjson "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/json"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/tcp"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/client"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestProofRpcAutobahn(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	const chainID = "proof-rpc-autobahn"
	rng := utils.TestRng()
	app := kvstore.NewApplication()
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	state := sm.NewStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, state.Close()) })
	peerDB := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, peerDB.Close()) })
	_, keys := atypes.GenCommittee(rng, 1)
	nodeKey := p2p.NodeSecretKey(ed25519.TestSecretKey([]byte(chainID)))
	addr := tcp.TestReserveAddr()
	endpoint := p2p.Endpoint{AddrPort: addr}
	genesis := &types.GenesisDoc{ChainID: chainID, InitialHeight: 1, AppState: json.RawMessage(`{}`)}
	require.NoError(t, genesis.ValidateAndComplete())
	nodeInfo := types.NodeInfo{
		NodeID: nodeKey.Public().NodeID(), ListenAddr: addr.String(),
		Network: chainID, Moniker: chainID, Channels: []byte{},
	}
	router, err := p2p.NewRouter(p2p.NopMetrics(), nodeKey,
		func() *types.NodeInfo { return &nodeInfo }, peerDB, &p2p.RouterOptions{
			SelfAddress: utils.Some(endpoint.NodeAddress(nodeKey.Public().NodeID())),
			Endpoint:    endpoint, Connection: conn.DefaultMConnConfig(),
			IncomingConnectionWindow: utils.Some(time.Duration(0)),
			MaxAcceptRate:            utils.Some(rate.Inf), MaxDialRate: utils.Some(rate.Limit(30)),
			Giga: utils.Some(&p2p.GigaRouterConfig{
				DialInterval: time.Second,
				ValidatorAddrs: map[atypes.PublicKey]p2p.GigaNodeAddr{
					keys[0].Public(): {Key: nodeKey.Public(), HostPort: tcp.HostPort{
						Hostname: addr.Addr().String(), Port: addr.Port(),
					}},
				},
				Consensus: &aconsensus.Config{
					Key: keys[0], ViewTimeout: func(atypes.View) time.Duration { return time.Hour },
					PersistentStateDir: utils.None[string](),
				},
				Producer: &producer.Config{
					App: proxy.New(app, proxy.NopMetrics()), MaxTxsPerBlock: 1,
					MaxGasWantedPerBlock: 1_000, MaxGasEstimatedPerBlock: 1_000,
					MaxTxsPerSecond: utils.None[uint64](), BlockInterval: time.Hour,
				},
				GenDoc: genesis, StateStore: state,
			}),
		})
	require.NoError(t, err)
	giga, ok := router.Giga().Get()
	require.True(t, ok)
	require.Equal(t, genesis.InitialHeight, giga.EarliestBlockNumber())
	env := &core.Environment{Router: router}

	// Evidence is created and signed through the real types, not a wire fixture.
	key := ed25519.TestSecretKey([]byte(chainID + "-evidence"))
	validator := types.NewValidator(key.Public(), 10)
	timestamp := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	voteA := &types.Vote{
		Type: tmproto.PrevoteType, Height: 1, Timestamp: timestamp,
		ValidatorAddress: validator.Address, ValidatorIndex: 0,
		BlockID: types.BlockID{
			Hash:          crypto.Checksum([]byte("block-a")).Bytes(),
			PartSetHeader: types.PartSetHeader{Total: 1, Hash: crypto.Checksum([]byte("parts")).Bytes()},
		},
	}
	voteB := voteA.Copy()
	voteB.BlockID.Hash = crypto.Checksum([]byte("block-b")).Bytes()
	for _, vote := range []*types.Vote{voteA, voteB} {
		vote.Signature = utils.Some(key.Sign(types.VoteSignBytes(chainID, vote.ToProto())))
		require.NoError(t, vote.Verify(chainID, key.Public()))
	}
	evidence, err := types.NewDuplicateVoteEvidence(voteA, voteB, timestamp, types.NewValidatorSet([]*types.Validator{validator}))
	require.NoError(t, err)
	require.NoError(t, evidence.ValidateBasic())

	// Assert the exported Go error identities before crossing JSON-RPC.
	for range 2 {
		dump, err := env.DumpConsensusState(ctx)
		require.Nil(t, dump)
		require.ErrorIs(t, err, coretypes.ErrConsensusStateNotAvailable)
		round, err := env.GetConsensusState(ctx)
		require.Nil(t, round)
		require.ErrorIs(t, err, coretypes.ErrConsensusStateNotAvailable)
		result, err := env.BroadcastEvidence(ctx, &coretypes.RequestBroadcastEvidence{Evidence: evidence})
		require.Nil(t, result)
		require.ErrorIs(t, err, coretypes.ErrEvidenceNotAccepted)
	}
	require.NotErrorIs(t, coretypes.ErrConsensusStateNotAvailable, coretypes.ErrEvidenceNotAccepted)

	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, core.NewRoutesMap(env, nil))
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	client, err := rpchttp.NewWithClient(server.URL, server.Client())
	require.NoError(t, err)

	for range 2 {
		before := requests.Load()
		health, err := client.Health(ctx)
		require.NoError(t, err)
		require.NotNil(t, health)
		require.Equal(t, before+1, requests.Load())
	}
	for _, tc := range []struct {
		name string
		want error
		call func() error
	}{
		{"dump_consensus_state", coretypes.ErrConsensusStateNotAvailable, func() error {
			result, err := client.DumpConsensusState(ctx)
			require.Nil(t, result)
			return err
		}},
		{"consensus_state", coretypes.ErrConsensusStateNotAvailable, func() error {
			result, err := client.ConsensusState(ctx)
			require.Nil(t, result)
			return err
		}},
		{"broadcast_evidence", coretypes.ErrEvidenceNotAccepted, func() error {
			result, err := client.BroadcastEvidence(ctx, evidence)
			require.Nil(t, result)
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for range 2 {
				before := requests.Load()
				err := tc.call()
				var rpcErr *rpctypes.RPCError
				require.ErrorAs(t, err, &rpcErr)
				require.Equal(t, int(rpctypes.CodeInternalError), rpcErr.Code)
				require.Equal(t, rpctypes.CodeInternalError.String(), rpcErr.Message)
				require.Equal(t, tc.want.Error(), rpcErr.Data)
				require.NotErrorIs(t, err, tc.want, "JSON does not transmit Go error identity")
				require.NotContains(t, rpcErr.Data, coretypes.ErrHeightNotAvailable.Error())
				require.NotContains(t, rpcErr.Data, coretypes.ErrHeightExceedsChainHead.Error())
				require.Equal(t, before+1, requests.Load(), "each call must reach the real server")
			}
		})
	}

	// Unknown identifiers are refused by the actual routing and client layers.
	rawClient, err := rpcclient.NewWithHTTPClient(server.URL, server.Client())
	require.NoError(t, err)
	var result coretypes.ResultHealth
	err = rawClient.Call(ctx, "unknown_autobahn_method", nil, &result)
	var rpcErr *rpctypes.RPCError
	require.ErrorAs(t, err, &rpcErr)
	require.Equal(t, int(rpctypes.CodeMethodNotFound), rpcErr.Code)

	// Obtain a fresh producer response, then damage its code without replacing
	// the server, HTTP transport, envelope encoder or consumer decoder.
	req := rpctypes.NewRequest(42)
	require.NoError(t, req.SetMethodAndParams("consensus_state", nil))
	payload, err := json.Marshal(req)
	require.NoError(t, err)
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, bytes.NewReader(payload))
	require.NoError(t, err)
	hreq.Header.Set("Content-Type", "application/json")
	response, err := server.Client().Do(hreq)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	wire, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	var decoded rpctypes.RPCResponse
	require.NoError(t, tmjson.Unmarshal(wire, &decoded))
	require.Equal(t, req.ID(), decoded.ID())
	require.NotNil(t, decoded.Error)
	require.Equal(t, coretypes.ErrConsensusStateNotAvailable.Error(), decoded.Error.Data)
	var envelope map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(wire, &envelope))
	var errorFields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(envelope["error"], &errorFields))
	errorFields["code"] = json.RawMessage(`"malformed"`)
	envelope["error"], err = json.Marshal(errorFields)
	require.NoError(t, err)
	malformed, err := json.Marshal(envelope)
	require.NoError(t, err)
	var refused rpctypes.RPCResponse
	err = tmjson.Unmarshal(malformed, &refused)
	var typeErr *json.UnmarshalTypeError
	require.True(t, errors.As(err, &typeErr), "malformed response must be refused by the consumer decoder: %v", err)
	require.Equal(t, reflect.TypeOf(int(0)), typeErr.Type)
	require.Equal(t, "string", typeErr.Value)
	require.Contains(t, typeErr.Field, "code")
	require.Nil(t, refused.Error)
}
