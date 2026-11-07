package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"gopkg.in/yaml.v3"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpc "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

type repairM1113Document struct {
	Info struct {
		Title       string `yaml:"title"`
		Description string `yaml:"description"`
	} `yaml:"info"`
	ExternalDocs struct {
		URL string `yaml:"url"`
	} `yaml:"externalDocs"`
	Servers []struct {
		URL         string `yaml:"url"`
		Description string `yaml:"description"`
	} `yaml:"servers"`
	Paths map[string]struct {
		Get struct {
			Description string `yaml:"description"`
			Responses   map[string]struct {
				Content map[string]struct {
					Examples map[string]struct {
						Value struct {
							Code    int    `yaml:"code"`
							Message string `yaml:"message"`
							Data    string `yaml:"data"`
						} `yaml:"value"`
					} `yaml:"examples"`
				} `yaml:"content"`
			} `yaml:"responses"`
		} `yaml:"get"`
	} `yaml:"paths"`
}

func repairM1113LoadDocument(t *testing.T) repairM1113Document {
	t.Helper()
	data, err := os.ReadFile("../../../rpc/openapi/openapi.yaml")
	require.NoError(t, err)
	var doc repairM1113Document
	require.NoError(t, yaml.Unmarshal(data, &doc))
	return doc
}

func repairM1113Server(t *testing.T, env *Environment) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	rpc.RegisterRPCFuncs(mux, NewRoutesMap(env, nil))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func TestRepairM_1113_DiagnosticBoundary(t *testing.T) {
	doc := repairM1113LoadDocument(t)
	defaults := config.DefaultRPCConfig()
	require.Contains(t, doc.Info.Title, "consensus diagnostic")
	require.Equal(t, "tcp://127.0.0.1:26657", defaults.ListenAddress)
	require.Contains(t, doc.Info.Description, "`"+defaults.ListenAddress+"`")
	require.Empty(t, defaults.CORSAllowedOrigins)
	require.Contains(t, doc.Info.Description, "CORS allowed origins are empty by default")
	require.Len(t, doc.Servers, 1)
	require.Equal(t, strings.Replace(defaults.ListenAddress, "tcp://", "http://", 1), doc.Servers[0].URL)
	require.Contains(t, doc.Servers[0].Description, "Internal consensus diagnostics")
	for _, text := range []string{"chain 125", "https://api-mainnet-beta.paxeer.network", "public-rpc.md#method-contracts", "gateway/openrpc.json", "does not describe eth_ or px_ methods"} {
		require.Contains(t, doc.Info.Description, text)
	}
	for _, target := range []string{"../../../docs/site/docs/reference/public-rpc.md", "../../../services/platform/hosted/gateway/openrpc.json"} {
		_, err := os.Stat(filepath.Join("../../../rpc/openapi", target))
		require.NoError(t, err, "public contract link must resolve")
	}
	public, err := os.ReadFile("../../../../docs/site/docs/reference/public-rpc.md")
	require.NoError(t, err)
	require.Contains(t, string(public), "https://api-mainnet-beta.paxeer.network")
	require.Contains(t, string(public), "## Method contracts")

	// The production diagnostic route table serves health but owns neither
	// public node namespaces nor hosted gateway methods.
	env := &Environment{Config: *defaults}
	server := repairM1113Server(t, env)
	client, err := rpchttp.New(server.URL)
	require.NoError(t, err)
	_, err = client.Health(t.Context())
	require.NoError(t, err)
	for _, method := range []string{"eth_chainId", "px_getCapabilities"} {
		require.NotContains(t, NewRoutesMap(env, nil), method)
		request, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 125, "method": method})
		require.NoError(t, err)
		status, body, media := repair91Request(t, t.Context(), server, http.MethodPost, "/", string(request))
		require.Equal(t, http.StatusOK, status)
		require.Equal(t, "application/json", media)
		var response rpctypes.RPCResponse
		require.NoError(t, json.Unmarshal(body, &response))
		require.NotNil(t, response.Error)
		require.Equal(t, int(rpctypes.CodeMethodNotFound), response.Error.Code)
		require.Empty(t, response.Result)
	}
}

func TestRepairM_1113_AutobahnRefusals(t *testing.T) {
	doc := repairM1113LoadDocument(t)
	app := kvstore.NewApplication()
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	states := sm.NewStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, states.Close()) })
	router, _ := newAutobahnTestRouter(t, utils.TestRng(), "repair-m1113-refusals", proxy.New(app, proxy.NopMetrics()), states, 1, 1000)
	// CometBFT components are deliberately absent: Autobahn mode must refuse
	// before accessing any of them. The router and handlers are production code.
	env := &Environment{Router: router}
	server := repairM1113Server(t, env)
	client, err := rpchttp.New(server.URL)
	require.NoError(t, err)
	ctx := t.Context()
	for _, route := range []struct {
		name     string
		symbol   string
		sentinel error
		producer func() (any, error)
		consumer func() (any, error)
	}{
		{"commit", "ErrCommitNotAvailable", coretypes.ErrCommitNotAvailable,
			func() (any, error) { return env.Commit(ctx, &coretypes.RequestBlockInfo{}) },
			func() (any, error) { return client.Commit(ctx, nil) }},
		{"consensus_state", "ErrConsensusStateNotAvailable", coretypes.ErrConsensusStateNotAvailable,
			func() (any, error) { return env.GetConsensusState(ctx) },
			func() (any, error) { return client.ConsensusState(ctx) }},
		{"dump_consensus_state", "ErrConsensusStateNotAvailable", coretypes.ErrConsensusStateNotAvailable,
			func() (any, error) { return env.DumpConsensusState(ctx) },
			func() (any, error) { return client.DumpConsensusState(ctx) }},
		{"broadcast_evidence", "ErrEvidenceNotAccepted", coretypes.ErrEvidenceNotAccepted,
			func() (any, error) { return env.BroadcastEvidence(ctx, &coretypes.RequestBroadcastEvidence{}) },
			func() (any, error) { return client.BroadcastEvidence(ctx, nil) }},
	} {
		t.Run(route.name, func(t *testing.T) {
			operation, ok := doc.Paths["/"+route.name]
			require.True(t, ok)
			for _, text := range []string{"Autobahn", route.symbol, route.sentinel.Error(), "-32603", "HTTP 200", "legacy"} {
				require.Contains(t, operation.Get.Description, text)
			}
			example := operation.Get.Responses["200"].Content["application/json"].Examples["autobahn_refusal"].Value
			require.Equal(t, -32603, example.Code)
			require.Equal(t, "Internal error", example.Message)
			require.Equal(t, route.sentinel.Error(), example.Data)
			result, err := route.producer()
			require.Nil(t, result)
			require.ErrorIs(t, err, route.sentinel)
			for _, method := range []string{http.MethodGet, http.MethodPost} {
				path, request := "/"+route.name, ""
				if method == http.MethodPost {
					path = "/"
					var params any
					switch route.name {
					case "commit":
						params = &coretypes.RequestBlockInfo{}
					case "broadcast_evidence":
						params = &coretypes.RequestBroadcastEvidence{}
					}
					body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": "repair-m1113", "method": route.name, "params": params})
					require.NoError(t, err)
					request = string(body)
				}
				status, body, media := repair91Request(t, ctx, server, method, path, request)
				require.Equal(t, http.StatusOK, status)
				require.Equal(t, "application/json", media)
				var fields map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(body, &fields))
				require.NotContains(t, fields, "result")
				var wireError rpctypes.RPCError
				if method == http.MethodPost {
					require.Len(t, fields, 3)
					require.JSONEq(t, `"2.0"`, string(fields["jsonrpc"]))
					require.JSONEq(t, `"repair-m1113"`, string(fields["id"]))
					require.NoError(t, json.Unmarshal(fields["error"], &wireError))
				} else {
					require.Len(t, fields, 3)
					require.NoError(t, json.Unmarshal(body, &wireError))
				}
				require.Equal(t, example.Code, wireError.Code)
				require.Equal(t, example.Message, wireError.Message)
				require.Equal(t, example.Data, wireError.Data)
			}
			if route.name == "commit" || route.name == "broadcast_evidence" {
				// Omitting the required parameter object is rejected before the
				// production handler can issue its consensus-mode refusal.
				request, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": "invalid-params", "method": route.name})
				require.NoError(t, err)
				status, body, media := repair91Request(t, ctx, server, http.MethodPost, "/", string(request))
				require.Equal(t, http.StatusOK, status)
				require.Equal(t, "application/json", media)
				var response rpctypes.RPCResponse
				require.NoError(t, json.Unmarshal(body, &response))
				require.NotNil(t, response.Error)
				require.Equal(t, int(rpctypes.CodeInvalidParams), response.Error.Code)
				require.Equal(t, "Invalid params", response.Error.Message)
				require.Contains(t, response.Error.Data, "parameters must be an object or an array")
				require.Empty(t, response.Result)
			}
			result, err = route.consumer()
			require.Nil(t, result)
			var wireError *rpctypes.RPCError
			require.ErrorAs(t, err, &wireError)
			require.Equal(t, example.Code, wireError.Code)
			require.Equal(t, example.Message, wireError.Message)
			require.Equal(t, example.Data, wireError.Data)
			require.NotErrorIs(t, err, route.sentinel, "wire errors preserve fields, not Go sentinel identity")
		})
	}
}

func TestRepairM_1113_MaintainedContractRecovery(t *testing.T) {
	doc := repairM1113LoadDocument(t)
	require.Equal(t, "../../internal/rpc/core/consensus_contract.md", doc.ExternalDocs.URL)
	contractPath := filepath.Join("../../../rpc/openapi", doc.ExternalDocs.URL)
	contract, err := os.ReadFile(contractPath)
	require.NoError(t, err)
	for _, section := range []string{"## RPC boundary", "## Names and encodings", "## Autobahn refusals", "## Commitment sources", "## State recovery"} {
		require.Contains(t, string(contract), section)
	}
	for _, text := range []string{"PersistentStateDir", "globalblocks", "fullcommitqcs", "StateStore", "RetainHeight", "no replacement", "errors.Is"} {
		require.Contains(t, string(contract), text)
	}

	// Every published local link must resolve, including the producer types,
	// public namespace contract and recovery implementation.
	links := regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`).FindAllStringSubmatch(string(contract), -1)
	require.NotEmpty(t, links)
	for _, link := range links {
		target := strings.SplitN(link[1], "#", 2)[0]
		_, err := os.Stat(filepath.Join(filepath.Dir(contractPath), target))
		require.NoError(t, err, "maintained contract link %s", link[1])
	}

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	application := &statusAutobahnObservedApplication{Application: kvstore.NewApplication(), committed: utils.NewAtomicSend[int64](0)}
	t.Cleanup(func() { require.NoError(t, application.Close()) })
	app := proxy.New(application, proxy.NopMetrics())
	dir := t.TempDir()
	db, err := dbm.NewGoLevelDB("execution-results", dir)
	require.NoError(t, err)
	states := sm.NewStore(db)
	t.Cleanup(func() { require.NoError(t, states.Close()) })
	router, giga := newAutobahnTestRouter(t, utils.TestRng(), "repair-m1113-recovery", app, states, 1, 1000)
	env := &Environment{App: app, Router: router, StateStore: states}
	var before *coretypes.ResultBlockResults
	var finalized *coretypes.ResultBlock
	require.NoError(t, scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.SpawnBgNamed("router", func() error { return utils.IgnoreCancel(router.Run(ctx)) })
		for _, tx := range []types.Tx{types.Tx("recovery=retained"), types.Tx("seal=next")} {
			response, err := giga.InsertTx(ctx, tx)
			if err != nil {
				return err
			}
			require.True(t, response.IsOK())
		}
		if _, err := application.committed.Subscribe().Wait(ctx, func(height int64) bool { return height >= 1 }); err != nil {
			return err
		}
		server := repairM1113Server(t, env)
		client, err := rpchttp.New(server.URL)
		if err != nil {
			return err
		}
		height := int64(1)
		finalized, err = client.Block(ctx, &height)
		if err != nil {
			return err
		}
		require.Equal(t, types.Txs{types.Tx("recovery=retained")}, finalized.Block.Txs)
		require.NotEmpty(t, finalized.BlockID.Hash)
		require.NotNil(t, finalized.Block.LastCommit)
		require.Empty(t, finalized.Block.LastCommit.Signatures, "Autobahn metadata has no Tendermint precommit signatures")
		require.Zero(t, finalized.Block.LastCommit.Height)
		require.Empty(t, finalized.Block.LastCommit.BlockID.Hash)
		before, err = client.BlockResults(ctx, &height)
		if err != nil {
			return err
		}
		require.Equal(t, int64(1), before.Height)
		require.Len(t, before.TxsResults, 1)
		require.Zero(t, before.TxsResults[0].Code)
		server.Close()
		return nil
	}))
	// The producer is stopped before closing and reopening its actual result
	// database. No response is copied into the reopened store.
	require.NoError(t, states.Close())
	db, err = dbm.NewGoLevelDB("execution-results", dir)
	require.NoError(t, err)
	states = sm.NewStore(db)
	router, _ = newAutobahnTestRouter(t, utils.TestRng(), "repair-m1113-recovery", app, states, 1, 1000)
	env = &Environment{App: app, Router: router, StateStore: states}
	server := repairM1113Server(t, env)
	client, err := rpchttp.New(server.URL)
	require.NoError(t, err)
	height := int64(1)
	after, err := client.BlockResults(ctx, &height)
	require.NoError(t, err)
	require.Equal(t, before, after)
	// Disabled data persistence has no recovered hash history. The durable
	// execution result is not evidence that all node state has recovered.
	missing, err := client.BlockByHash(ctx, finalized.BlockID.Hash)
	require.NoError(t, err)
	require.Nil(t, missing.Block)
	_, err = client.Commit(ctx, &height)
	var refusal *rpctypes.RPCError
	require.ErrorAs(t, err, &refusal)
	require.Equal(t, coretypes.ErrCommitNotAvailable.Error(), refusal.Data)
}

func TestRepairM_1113_ValidEvidenceRefused(t *testing.T) {
	const chainID = "repair-m1113-evidence"
	key := ed25519.TestSecretKey([]byte(chainID))
	validator := types.NewValidator(key.Public(), 10)
	validators := types.NewValidatorSet([]*types.Validator{validator})
	timestamp := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	votes := make([]*types.Vote, 2)
	for i, label := range []string{"first", "conflicting"} {
		vote := &types.Vote{
			Type: tmproto.PrevoteType, Height: 1, Timestamp: timestamp,
			ValidatorAddress: validator.Address, ValidatorIndex: 0,
			BlockID: types.BlockID{
				Hash:          crypto.Checksum([]byte(label)).Bytes(),
				PartSetHeader: types.PartSetHeader{Total: 1, Hash: crypto.Checksum([]byte("parts")).Bytes()},
			},
		}
		vote.Signature = utils.Some(key.Sign(types.VoteSignBytes(chainID, vote.ToProto())))
		require.NoError(t, vote.Verify(chainID, key.Public()))
		votes[i] = vote
	}
	evidence, err := types.NewDuplicateVoteEvidence(votes[0], votes[1], timestamp, validators)
	require.NoError(t, err)
	require.NoError(t, evidence.ValidateBasic())
	app := kvstore.NewApplication()
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	states := sm.NewStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, states.Close()) })
	router, _ := newAutobahnTestRouter(t, utils.TestRng(), chainID, proxy.New(app, proxy.NopMetrics()), states, 1, 1000)
	env := &Environment{Router: router}
	result, err := env.BroadcastEvidence(t.Context(), &coretypes.RequestBroadcastEvidence{Evidence: evidence})
	require.Nil(t, result)
	require.ErrorIs(t, err, coretypes.ErrEvidenceNotAccepted)
	server := repairM1113Server(t, env)
	client, err := rpchttp.New(server.URL)
	require.NoError(t, err)
	result, err = client.BroadcastEvidence(t.Context(), evidence)
	require.Nil(t, result)
	var refusal *rpctypes.RPCError
	require.ErrorAs(t, err, &refusal)
	require.Equal(t, int(rpctypes.CodeInternalError), refusal.Code)
	require.Equal(t, "Internal error", refusal.Message)
	require.Equal(t, coretypes.ErrEvidenceNotAccepted.Error(), refusal.Data)
}
