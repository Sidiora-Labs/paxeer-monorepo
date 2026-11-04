package client_test

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/merkle"
	tmcrypto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/crypto"
	rpcclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/testdata"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	grpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/grpc"
)

func repairM1461Node(t *testing.T, query func(context.Context, *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error)) client.Context {
	t.Helper()
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, map[string]*rpcserver.RPCFunc{"abci_query": rpcserver.NewRPCFunc(query)})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	node, err := client.NewClientFromNode(server.URL)
	require.NoError(t, err)
	return client.Context{}.WithClient(node)
}

func repairM1461Echo(ctx context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
	if req.Path != "/testdata.Query/Echo" {
		return nil, fmt.Errorf("unexpected path: %s", req.Path)
	}
	var request testdata.EchoRequest
	if err := proto.Unmarshal(req.Data, &request); err != nil {
		return nil, err
	}
	response, err := (testdata.QueryImpl{}).Echo(ctx, &request)
	if err != nil {
		return nil, err
	}
	value, err := proto.Marshal(response)
	if err != nil {
		return nil, err
	}
	height := int64(req.Height)
	if height == 0 {
		height = 9
	}
	return &coretypes.ResultABCIQuery{Response: abci.ResponseQuery{Value: value, Height: height}}, nil
}

func TestRepairM_1461_InvokeValidation(t *testing.T) {
	ctx := repairM1461Node(t, repairM1461Echo)
	for name, request := range map[string]interface{}{
		"nil interface": nil, "typed nil": (*testdata.EchoRequest)(nil),
		"integer": 42, "struct": struct{}{}, "map": map[string]int{}, "slice": []byte("invalid"),
	} {
		t.Run(name, func(t *testing.T) {
			err := ctx.Invoke(context.Background(), "/testdata.Query/Echo", request, &testdata.EchoResponse{})
			require.Error(t, err)
			require.True(t, errors.Is(err, sdkerrors.ErrInvalidRequest), "%v", err)
		})
	}
	var header metadata.MD
	response, err := testdata.NewQueryClient(ctx).Echo(context.Background(), &testdata.EchoRequest{Message: "real protobuf request"}, grpc.Header(&header))
	require.NoError(t, err)
	require.Equal(t, "real protobuf request", response.Message)
	require.Equal(t, []string{"9"}, header.Get(grpctypes.GRPCBlockHeightHeader))
}

func TestRepairM_1461_GRPCSuiteConstructor(t *testing.T) {
	// The norace source is outside the gate's build tags. Check its actual call
	// expression and exercise the retained query contract over the real transport.
	file, err := parser.ParseFile(token.NewFileSet(), "grpc_query_test.go", nil, 0)
	require.NoError(t, err)
	calls := 0
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "DefaultConfig" {
			return true
		}
		receiver, ok := selector.X.(*ast.Ident)
		if !ok || receiver.Name != "network" {
			return true
		}
		calls++
		require.Len(t, call.Args, 1)
		argument, ok := call.Args[0].(*ast.CallExpr)
		require.True(t, ok)
		method, ok := argument.Fun.(*ast.SelectorExpr)
		require.True(t, ok)
		require.Equal(t, "T", method.Sel.Name)
		suite, ok := method.X.(*ast.Ident)
		require.True(t, ok)
		require.Equal(t, "s", suite.Name)
		require.Empty(t, argument.Args)
		return true
	})
	require.Equal(t, 1, calls)
	ctx := repairM1461Node(t, repairM1461Echo).WithHeight(1)
	var header metadata.MD
	reply, err := testdata.NewQueryClient(ctx).Echo(context.Background(), &testdata.EchoRequest{Message: "hello"}, grpc.Header(&header))
	require.NoError(t, err)
	require.Equal(t, "hello", reply.Message)
	require.Equal(t, []string{"1"}, header.Get(grpctypes.GRPCBlockHeightHeader))
}

func TestRepairM_1461_QueryCancellation(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(fmt.Sprintf("deadline=%v", deadline), func(t *testing.T) {
			started := make(chan struct{})
			stopped := make(chan error, 1)
			ctx := repairM1461Node(t, func(callCtx context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
				if req.Path != "/testdata.Query/Echo" {
					return nil, fmt.Errorf("unexpected path %s", req.Path)
				}
				close(started)
				select {
				case <-callCtx.Done():
					stopped <- callCtx.Err()
					return nil, callCtx.Err()
				case <-time.After(5 * time.Second):
					return nil, fmt.Errorf("caller cancellation never reached the node")
				}
			})
			caller, cancel := context.WithCancel(context.Background())
			if deadline {
				cancel()
				caller, cancel = context.WithTimeout(context.Background(), 500*time.Millisecond)
			}
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := testdata.NewQueryClient(ctx).Echo(caller, &testdata.EchoRequest{Message: "cancel"})
				done <- err
			}()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("request did not reach node")
			}
			if !deadline {
				cancel()
			}
			expected := context.Canceled
			if deadline {
				expected = context.DeadlineExceeded
			}
			select {
			case err := <-done:
				require.ErrorIs(t, err, expected)
			case <-time.After(3 * time.Second):
				t.Fatal("Invoke ignored cancellation")
			}
			select {
			case err := <-stopped:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(3 * time.Second):
				t.Fatal("underlying HTTP request was not cancelled")
			}
		})
	}
}

func TestRepairM_1461_QueryHeight(t *testing.T) {
	ctx := repairM1461Node(t, repairM1461Echo)
	for _, tc := range []struct{ request, configured, expected int64 }{{3, 1, 3}, {0, 3, 3}, {0, 0, 9}} {
		wire, err := proto.Marshal(&testdata.EchoRequest{Message: "height"})
		require.NoError(t, err)
		response, err := ctx.WithHeight(tc.configured).QueryABCI(abci.RequestQuery{Path: "/testdata.Query/Echo", Data: wire, Height: tc.request})
		require.NoError(t, err)
		require.Equal(t, tc.expected, response.Height)
	}
	caller := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(grpctypes.GRPCBlockHeightHeader, "4"))
	var header metadata.MD
	reply, err := testdata.NewQueryClient(ctx.WithHeight(1)).Echo(caller, &testdata.EchoRequest{Message: "metadata"}, grpc.Header(&header))
	require.NoError(t, err)
	require.Equal(t, "metadata", reply.Message)
	require.Equal(t, []string{"4"}, header.Get(grpctypes.GRPCBlockHeightHeader))
}

func repairM1461Proof(t *testing.T) (abci.ResponseQuery, []byte, string) {
	t.Helper()
	// This committed transcript is produced by TestChainProofVectorV1 through
	// the production store, commit and query paths, including both proof layers.
	data, err := os.ReadFile("../store/rootmulti/testdata/chain_proof_v1.hex")
	require.NoError(t, err)
	fields := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		key, value, ok := strings.Cut(line, "=")
		require.True(t, ok)
		fields[key] = value
	}
	decode := func(key string) []byte {
		value, err := hex.DecodeString(fields[key])
		require.NoError(t, err)
		return value
	}
	height, err := strconv.ParseInt(fields["height"], 10, 64)
	require.NoError(t, err)
	proof := new(tmcrypto.ProofOps)
	require.NoError(t, proof.Unmarshal(decode("present_proof_ops")))
	response := abci.ResponseQuery{Key: decode("present_key"), Value: decode("present_value"), Height: height, ProofOps: proof}
	module := decode("module_key")
	path := merkle.KeyPath{}.AppendKey(module, merkle.KeyEncodingHex).AppendKey(response.Key, merkle.KeyEncodingHex).String()
	root := decode("app_hash")
	require.NoError(t, rootmulti.DefaultProofRuntime().VerifyValue(proof, root, path, response.Value))
	return response, root, path
}

func TestRepairM_1461_ProofRefusal(t *testing.T) {
	for _, name := range []string{"authentic", "missing", "empty", "forged value", "forged proof", "wrong height"} {
		t.Run(name, func(t *testing.T) {
			response, root, keypath := repairM1461Proof(t)
			switch name {
			case "missing":
				response.ProofOps = nil
			case "empty":
				response.ProofOps.Ops = nil
			case "forged value":
				response.Value = []byte("forged")
			case "forged proof":
				response.ProofOps.Ops[0].Data[0] ^= 1
			case "wrong height":
				response.Height++
			}
			requests := make(chan coretypes.RequestABCIQuery, 3)
			ctx := repairM1461Node(t, func(_ context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
				requests <- *req
				return &coretypes.ResultABCIQuery{Response: response}, nil
			})
			node, err := ctx.GetNode()
			require.NoError(t, err)
			received, err := node.ABCIQueryWithOptions(context.Background(), "/store/bank/key", response.Key, rpcclient.ABCIQueryOptions{Height: 1, Prove: true})
			require.NoError(t, err)
			if name == "authentic" {
				require.NoError(t, rootmulti.DefaultProofRuntime().VerifyValue(received.Response.ProofOps, root, keypath, received.Response.Value))
			} else if name != "wrong height" {
				require.Error(t, rootmulti.DefaultProofRuntime().VerifyValue(received.Response.ProofOps, root, keypath, received.Response.Value))
			} else {
				require.NotEqual(t, int64(1), received.Response.Height)
			}
			<-requests
			refused, err := ctx.QueryABCIWithContext(context.Background(), abci.RequestQuery{Path: "/store/bank/key", Data: response.Key, Height: 1, Prove: true})
			require.ErrorIs(t, err, client.ErrQueryProofVerificationUnavailable)
			require.Equal(t, codes.Unimplemented, status.Code(err))
			require.Equal(t, abci.ResponseQuery{}, refused)
			req := <-requests
			require.Equal(t, "/store/bank/key", req.Path)
			require.Equal(t, response.Key, []byte(req.Data))
			require.Equal(t, int64(1), int64(req.Height))
			require.True(t, req.Prove)
		})
	}
}

func TestRepairM_1461_QueryProofPath(t *testing.T) {
	response, _, _ := repairM1461Proof(t)
	requests := make(chan coretypes.RequestABCIQuery, 1)
	ctx := repairM1461Node(t, func(_ context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
		requests <- *req
		return &coretypes.ResultABCIQuery{Response: response}, nil
	})
	value, height, err := ctx.QueryStore(response.Key, "bank")
	require.NoError(t, err)
	require.Equal(t, response.Value, value)
	require.Equal(t, response.Height, height)
	req := <-requests
	require.Equal(t, "/store/bank/key", req.Path)
	require.False(t, req.Prove)
	req.Prove = true
	refused, err := ctx.QueryABCI(abci.RequestQuery{Path: req.Path, Data: req.Data, Prove: req.Prove})
	require.ErrorIs(t, err, client.ErrQueryProofVerificationUnavailable)
	require.Empty(t, refused.Value)
	require.True(t, (<-requests).Prove)
}

func TestRepairM_1461_Pagination(t *testing.T) {
	for _, tc := range []struct {
		name                                     string
		total, page, limit, fallback, start, end int
	}{
		{"maximum count", math.MaxInt, 1, math.MaxInt, 0, 0, math.MaxInt},
		{"product overflow", math.MaxInt, math.MaxInt, math.MaxInt, 0, -1, -1},
		{"addition overflow", math.MaxInt, 2, math.MaxInt/2 + 1, 0, -1, -1},
		{"zero default", 10, 1, 0, 0, -1, -1},
		{"default limit", 23, 2, 0, 10, 10, 20},
		{"partial final page", 23, 3, 10, 10, 20, 23},
		{"last representable index", math.MaxInt, math.MaxInt, 1, 0, math.MaxInt - 1, math.MaxInt},
		{"past final page", 23, 4, 10, 10, -1, -1},
		{"zero page", 23, 0, 10, 10, -1, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start, end := client.Paginate(tc.total, tc.page, tc.limit, tc.fallback)
			require.Equal(t, tc.start, start)
			require.Equal(t, tc.end, end)
		})
	}
	for _, tc := range []struct {
		name, page, limit, offset string
		expected                  uint64
		fail                      bool
	}{
		{"maximum page", "18446744073709551615", "1", "0", math.MaxUint64 - 1, false},
		{"overflow product", "18446744073709551615", "2", "0", 0, true},
		{"maximum limit", "2", "18446744073709551615", "0", math.MaxUint64, false},
		{"overflow maximum limit", "3", "18446744073709551615", "0", 0, true},
		{"zero limit uses server default", "3", "0", "0", 200, false},
		{"default page", "1", "0", "0", 0, false},
		{"explicit maximum offset", "1", "1", "18446744073709551615", math.MaxUint64, false},
		{"conflicting offset", "2", "10", "1", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			flags.AddPaginationFlagsToCmd(cmd, "records")
			require.NoError(t, cmd.Flags().Parse([]string{"--page", tc.page, "--limit", tc.limit, "--offset", tc.offset}))
			request, err := client.ReadPageRequest(cmd.Flags())
			if tc.fail {
				require.Error(t, err)
				require.Nil(t, request)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, request.Offset)
			limit, err := strconv.ParseUint(tc.limit, 10, 64)
			require.NoError(t, err)
			require.Equal(t, limit, request.Limit)
		})
	}
}
