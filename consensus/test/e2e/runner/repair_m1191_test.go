package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/code"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/mempool"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/rpc/core"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/app"
	e2e "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/pkg"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

type repair1191DockerResource struct {
	ID     string `json:"Id"`
	Name   string
	Labels map[string]string
}

// The Engine peer implements Docker's label conjunction and resource lifecycle
// over HTTP; cleanup and its HTTP request path are production code.
func repair1191Engine(t *testing.T) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	networks := map[string]repair1191DockerResource{}
	containers := map[string]repair1191DockerResource{
		"selected":   {ID: "selected", Labels: map[string]string{"e2e": "true", "com.docker.compose.project": "alphatest_net"}},
		"other":      {ID: "other", Labels: map[string]string{"e2e": "true", "com.docker.compose.project": "beta"}},
		"unlabelled": {ID: "unlabelled", Labels: map[string]string{"com.docker.compose.project": "alphatest_net"}},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		var filters map[string][]string
		if raw := r.URL.Query().Get("filters"); raw != "" {
			if err := json.Unmarshal([]byte(raw), &filters); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		matches := func(labels map[string]string) bool {
			for _, label := range filters["label"] {
				key, value, hasValue := strings.Cut(label, "=")
				actual, present := labels[key]
				if !present || hasValue && actual != value {
					return false
				}
			}
			return true
		}
		switch {
		case r.Method == http.MethodPost && path == "/networks/create":
			var request repair1191DockerResource
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			networks[request.Name] = repair1191DockerResource{ID: request.Name, Name: request.Name, Labels: request.Labels}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(repair1191DockerResource{ID: request.Name})
		case r.Method == http.MethodGet && path == "/networks":
			result := []repair1191DockerResource{}
			for _, resource := range networks {
				if matches(resource.Labels) {
					result = append(result, resource)
				}
			}
			_ = json.NewEncoder(w).Encode(result)
		case r.Method == http.MethodGet && path == "/containers/json":
			if r.URL.Query().Get("all") != "1" {
				t.Error("cleanup must include stopped containers")
			}
			result := []repair1191DockerResource{}
			for _, resource := range containers {
				if matches(resource.Labels) {
					result = append(result, resource)
				}
			}
			_ = json.NewEncoder(w).Encode(result)
		case r.Method == http.MethodDelete && strings.HasPrefix(path, "/containers/"):
			if r.URL.Query().Get("force") != "1" {
				t.Error("cleanup must remove running containers")
			}
			delete(containers, strings.TrimPrefix(path, "/containers/"))
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete && strings.HasPrefix(path, "/networks/"):
			id := strings.TrimPrefix(path, "/networks/")
			for _, resource := range containers {
				if resource.Labels["e2e"] != "" && resource.Labels["com.docker.compose.project"] == networks[id].Labels["com.docker.compose.project"] {
					http.Error(w, `{"message":"network has active endpoints"}`, http.StatusConflict)
					return
				}
			}
			delete(networks, id)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected Engine request: %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestRepairM_1191_CleanupNetworkIsolation(t *testing.T) {
	server := repair1191Engine(t)
	docker, endpoint := server.Client(), server.URL
	ctx := t.Context()
	for _, project := range []string{"alphatest_net", "beta"} {
		request := repair1191DockerResource{
			Name: project + "_testnet", Labels: map[string]string{"e2e": "true", "com.docker.compose.project": project},
		}
		var created repair1191DockerResource
		require.NoError(t, cleanupDockerRequest(ctx, docker, endpoint, http.MethodPost, "/networks/create", request, &created))
		require.Equal(t, request.Name, created.ID)
	}
	var before []repair1191DockerResource
	require.NoError(t, cleanupDockerRequest(ctx, docker, endpoint, http.MethodGet, "/networks", nil, &before))
	require.Len(t, before, 2)

	testnet := &e2e.Testnet{Dir: filepath.Join(t.TempDir(), "Alpha.Test_Net")}
	require.NoError(t, cleanupDocker(testnet, docker, endpoint))
	var after []repair1191DockerResource
	require.NoError(t, cleanupDockerRequest(ctx, docker, endpoint, http.MethodGet, "/networks", nil, &after))
	require.Len(t, after, 1)
	require.Equal(t, "beta_testnet", after[0].ID)
	var remaining []repair1191DockerResource
	require.NoError(t, cleanupDockerRequest(ctx, docker, endpoint, http.MethodGet, "/containers/json?all=1", nil, &remaining))
	ids := make([]string, 0, len(remaining))
	for _, resource := range remaining {
		ids = append(ids, resource.ID)
	}
	require.ElementsMatch(t, []string{"other", "unlabelled"}, ids)
	require.NoError(t, cleanupDocker(testnet, docker, endpoint)) // an empty selection is safe
	require.Error(t, cleanupDocker(&e2e.Testnet{}, docker, endpoint))
	require.Error(t, cleanupDocker(&e2e.Testnet{Dir: "/---..."}, docker, endpoint))
	require.NoError(t, cleanupDockerRequest(ctx, docker, endpoint, http.MethodGet, "/networks", nil, &after))
	require.Len(t, after, 1)
	require.Equal(t, "beta_testnet", after[0].ID)
}

func repair1191RPCNode(t *testing.T, routes map[string]*rpcserver.RPCFunc) *e2e.Node {
	t.Helper()
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, routes)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	require.NoError(t, err)
	number, err := strconv.ParseUint(port, 10, 32)
	require.NoError(t, err)
	return &e2e.Node{Name: "validator01", Mode: e2e.ModeValidator, HasStarted: true, ProxyPort: uint32(number)}
}

func TestRepairM_1191_LoadRejectionCount(t *testing.T) {
	for _, tc := range []struct {
		name     string
		txs      []types.Tx
		accepted int
	}{
		{name: "all_rejected", txs: []types.Tx{[]byte("malformed-one"), []byte("malformed-two")}},
		{name: "mixed", txs: []types.Tx{[]byte("malformed-one"), []byte("load-one=value"), []byte("malformed-two"), []byte("load-two=value")}, accepted: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			application, err := app.NewApplication(app.DefaultConfig(t.TempDir()))
			require.NoError(t, err)
			pool := mempool.NewTxMempool(mempool.DefaultConfig(), proxy.New(application, proxy.NopMetrics()), mempool.NopMetrics(), mempool.NopTxConstraintsFetcher)
			env := &core.Environment{Mempool: utils.Some(pool)}
			type observation struct {
				tx     types.Tx
				result *coretypes.ResultBroadcastTx
			}
			observed := make(chan observation, len(tc.txs)+1)
			node := repair1191RPCNode(t, map[string]*rpcserver.RPCFunc{
				"status": rpcserver.NewRPCFunc(func(context.Context) (*coretypes.ResultStatus, error) {
					return &coretypes.ResultStatus{NodeInfo: types.NodeInfo{Network: "repair-m1191"}}, nil
				}),
				"broadcast_tx_sync": rpcserver.NewRPCFunc(func(ctx context.Context, request *coretypes.RequestBroadcastTx) (*coretypes.ResultBroadcastTx, error) {
					result, err := env.BroadcastTxSync(ctx, request)
					if err == nil {
						observed <- observation{tx: request.Tx, result: result}
					}
					return result, err
				}),
			})
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			transactions := make(chan types.Tx)
			successes := make(chan int, len(tc.txs)+1)
			err = scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
				s.SpawnBg(func() error {
					loadProcess(ctx, &e2e.Testnet{Nodes: []*e2e.Node{node}}, transactions, successes)
					return nil
				})
				// Receiving the final rejected sentinel proves the worker completed
				// every preceding response before scope cancellation joins it.
				for _, tx := range append(append([]types.Tx{}, tc.txs...), types.Tx("sentinel-malformed")) {
					select {
					case transactions <- tx:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
				return nil
			})
			require.NoError(t, err)
			count := 0
			for len(successes) > 0 {
				count += <-successes
			}
			require.Equal(t, tc.accepted, count)
			require.Equal(t, tc.accepted, pool.Size())
			for _, tx := range tc.txs {
				select {
				case response := <-observed:
					require.Equal(t, tx, response.tx)
					require.Equal(t, []byte(tx.Hash().Bytes()), []byte(response.result.Hash))
					expected := code.CodeTypeEncodingError
					if strings.Contains(string(tx), "=") {
						expected = code.CodeTypeOK
					}
					require.Equal(t, expected, response.result.Code)
					require.Empty(t, response.result.Codespace)
					require.Empty(t, response.result.Log)
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
			}
		})
	}
}

func TestRepairM_1191_HeightBlockRetry(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(fmt.Sprintf("empty_block_%t", empty), func(t *testing.T) {
			var statuses, blocks atomic.Int32
			block := types.MakeBlock(7, []types.Tx{[]byte("load=committed")}, &types.Commit{}, nil)
			block.ChainID = "repair-m1191"
			block.ValidatorsHash = types.Tx("validators").Hash().Bytes()
			parts, err := block.MakePartSet(65536)
			require.NoError(t, err)
			blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: parts.Header()}
			node := repair1191RPCNode(t, map[string]*rpcserver.RPCFunc{
				"status": rpcserver.NewRPCFunc(func(context.Context) (*coretypes.ResultStatus, error) {
					statuses.Add(1)
					return &coretypes.ResultStatus{NodeInfo: types.NodeInfo{Network: "repair-m1191"}, SyncInfo: coretypes.SyncInfo{LatestBlockHeight: 9}}, nil
				}),
				"block": rpcserver.NewRPCFunc(func(_ context.Context, request *coretypes.RequestBlockInfo) (*coretypes.ResultBlock, error) {
					if request.Height == nil || int64(*request.Height) != 7 {
						return nil, errors.New("incorrect target height")
					}
					if blocks.Add(1) == 1 {
						if empty {
							return &coretypes.ResultBlock{}, nil
						}
						return nil, errors.New("transient block read failure")
					}
					return &coretypes.ResultBlock{Block: block, BlockID: blockID}, nil
				}),
			})
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			result, id, err := waitForHeight(ctx, &e2e.Testnet{Nodes: []*e2e.Node{node}, InitialHeight: 7}, 0)
			require.NoError(t, err)
			require.Equal(t, int64(7), result.Height)
			require.Equal(t, block.Data.Txs, result.Data.Txs)
			require.Equal(t, blockID, *id)
			require.Equal(t, int32(2), blocks.Load())
			require.Equal(t, int32(1), statuses.Load())
		})
	}
}

func TestRepairM_1191_HeightBlockCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var blocks atomic.Int32
	node := repair1191RPCNode(t, map[string]*rpcserver.RPCFunc{
		"status": rpcserver.NewRPCFunc(func(context.Context) (*coretypes.ResultStatus, error) {
			return &coretypes.ResultStatus{NodeInfo: types.NodeInfo{Network: "repair-m1191"}, SyncInfo: coretypes.SyncInfo{LatestBlockHeight: 7}}, nil
		}),
		"block": rpcserver.NewRPCFunc(func(context.Context, *coretypes.RequestBlockInfo) (*coretypes.ResultBlock, error) {
			if blocks.Add(1) == 2 {
				cancel()
			}
			return nil, errors.New("target block remains unavailable")
		}),
	})
	deadline, stop := context.WithTimeout(ctx, 5*time.Second)
	defer stop()
	block, id, err := waitForHeight(deadline, &e2e.Testnet{Nodes: []*e2e.Node{node}}, 7)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, block)
	require.Nil(t, id)
	require.Equal(t, int32(2), blocks.Load())
}
