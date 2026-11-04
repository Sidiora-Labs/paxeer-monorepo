package query

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/grpc/tmservice"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	pagination "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	txtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tools/tx-scanner/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// The peers serve a deterministic in-memory transcript through the production
// protobuf services and transport. The query functions under test are unchanged
// production entry points, including the legacy scanner calls.
type repairM882Blocks struct {
	tmservice.UnimplementedServiceServer
}

func (*repairM882Blocks) GetLatestBlock(context.Context, *tmservice.GetLatestBlockRequest) (*tmservice.GetLatestBlockResponse, error) {
	return &tmservice.GetLatestBlockResponse{Block: &tmproto.Block{Header: tmproto.Header{Height: 42}}}, nil
}

func (*repairM882Blocks) GetBlockByHeight(_ context.Context, req *tmservice.GetBlockByHeightRequest) (*tmservice.GetBlockByHeightResponse, error) {
	return &tmservice.GetBlockByHeightResponse{Block: &tmproto.Block{Header: tmproto.Header{Height: req.Height}}}, nil
}

type repairM882Txs struct {
	txtypes.UnimplementedServiceServer
	total   int
	mode    string
	mu      sync.Mutex
	offsets []uint64
}

func repairM882Entry(index int) (*txtypes.Tx, *sdk.TxResponse) {
	memo := fmt.Sprintf("transaction-%d", index)
	return &txtypes.Tx{Body: &txtypes.TxBody{Memo: memo}}, &sdk.TxResponse{
		Height: 42, TxHash: fmt.Sprintf("%X", sha256.Sum256([]byte(memo))),
	}
}

func (s *repairM882Txs) GetTx(_ context.Context, req *txtypes.GetTxRequest) (*txtypes.GetTxResponse, error) {
	for i := 0; i < s.total; i++ {
		tx, response := repairM882Entry(i)
		if response.TxHash == req.Hash {
			return &txtypes.GetTxResponse{Tx: tx, TxResponse: response}, nil
		}
	}
	return nil, status.Error(codes.NotFound, "transaction absent from transcript")
}

func (s *repairM882Txs) GetTxsEvent(_ context.Context, req *txtypes.GetTxsEventRequest) (*txtypes.GetTxsEventResponse, error) {
	if !reflect.DeepEqual(req.Events, []string{"tx.height=42"}) || req.OrderBy != txtypes.OrderBy_ORDER_BY_ASC || req.Pagination == nil || !req.Pagination.CountTotal || len(req.Pagination.Key) != 0 {
		return nil, status.Error(codes.InvalidArgument, "expected ordered, explicitly paginated height query")
	}
	page, limit, err := pagination.ParsePagination(req.Pagination)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.offsets = append(s.offsets, req.Pagination.Offset)
	s.mu.Unlock()
	start := (page - 1) * limit
	if start > 0 && s.mode == "rpc-error" {
		return nil, status.Error(codes.Unavailable, "index unavailable on second page")
	}
	result := &txtypes.GetTxsEventResponse{Pagination: &pagination.PageResponse{Total: uint64(s.total)}}
	for i := start; i < min(start+limit, s.total); i++ {
		tx, response := repairM882Entry(i)
		result.Txs = append(result.Txs, tx)
		result.TxResponses = append(result.TxResponses, response)
	}
	if start > 0 {
		switch s.mode {
		case "missing-pagination":
			result.Pagination = nil
		case "changed-total":
			result.Pagination.Total++
		case "empty-page":
			result.Txs, result.TxResponses = nil, nil
		case "short-page":
			result.Txs = result.Txs[:len(result.Txs)-1]
			result.TxResponses = result.TxResponses[:len(result.TxResponses)-1]
		case "mismatched-pairs":
			result.TxResponses = result.TxResponses[:len(result.TxResponses)-1]
		case "duplicate":
			result.Txs[0], result.TxResponses[0] = repairM882Entry(0)
		case "missing-hash":
			result.TxResponses[0].TxHash = ""
		}
	}
	return result, nil
}

func repairM882Serve(t *testing.T, txs *repairM882Txs, interceptor grpc.UnaryServerInterceptor) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var options []grpc.ServerOption
	if interceptor != nil {
		options = append(options, grpc.UnaryInterceptor(interceptor))
	}
	server := grpc.NewServer(options...)
	tmservice.RegisterServiceServer(server, &repairM882Blocks{})
	txtypes.RegisterServiceServer(server, txs)
	go server.Serve(listener)
	dialCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		server.Stop()
		t.Fatal(err)
	}
	previous := client.GrpcConn
	client.GrpcConn = conn
	t.Cleanup(func() { client.GrpcConn = previous; conn.Close(); server.Stop(); listener.Close() })
}

func TestRepairM_882_CompletePagination(t *testing.T) {
	for _, count := range []int{0, 100, 101, 200, 205} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			peer := &repairM882Txs{total: count}
			repairM882Serve(t, peer, nil)
			result, err := GetTxsEvent(42)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Txs) != count || len(result.TxResponses) != count || result.Pagination.Total != uint64(count) || len(result.Pagination.NextKey) != 0 {
				t.Fatalf("incomplete aggregate: txs=%d responses=%d pagination=%v", len(result.Txs), len(result.TxResponses), result.Pagination)
			}
			var expectedOffsets []uint64
			for offset := 0; offset < max(count, 1); offset += 100 {
				expectedOffsets = append(expectedOffsets, uint64(offset))
			}
			peer.mu.Lock()
			offsets := append([]uint64(nil), peer.offsets...)
			peer.mu.Unlock()
			if !reflect.DeepEqual(offsets, expectedOffsets) {
				t.Fatalf("offsets=%v want %v", offsets, expectedOffsets)
			}
			for i, response := range result.TxResponses {
				expectedTx, expectedResponse := repairM882Entry(i)
				if response.TxHash != expectedResponse.TxHash || result.Txs[i].Body.Memo != expectedTx.Body.Memo {
					t.Fatalf("entry %d was lost or reordered", i)
				}
				tx, err := GetTxByHash(response.TxHash)
				if err != nil || tx.TxResponse.TxHash != response.TxHash || tx.Tx.Body.Memo != expectedTx.Body.Memo {
					t.Fatalf("hash consumer %d: %v, %v", i, tx, err)
				}
			}
		})
	}
}

func TestRepairM_882_IncompleteSetReturnsError(t *testing.T) {
	for _, mode := range []string{"rpc-error", "missing-pagination", "changed-total", "empty-page", "short-page", "mismatched-pairs", "duplicate", "missing-hash"} {
		t.Run(mode, func(t *testing.T) {
			repairM882Serve(t, &repairM882Txs{total: 205, mode: mode}, nil)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			result, err := GetTxsEventContext(ctx, 42)
			want := codes.DataLoss
			if mode == "rpc-error" {
				want = codes.Unavailable
			}
			if result != nil || status.Code(err) != want {
				t.Fatalf("partial scan escaped: result=%v error=%v, want %v", result, err, want)
			}
		})
	}
}

type repairM882Call struct {
	name   string
	call   func(context.Context) error
	legacy func() error
}

func repairM882Calls() []repairM882Call {
	return []repairM882Call{
		{"latest-block", func(ctx context.Context) error { _, err := GetLatestBlockContext(ctx); return err }, func() error { _, err := GetLatestBlock(); return err }},
		{"block-height", func(ctx context.Context) error { _, err := GetBlockByHeightContext(ctx, 42); return err }, func() error { _, err := GetBlockByHeight(42); return err }},
		{"tx-events", func(ctx context.Context) error { _, err := GetTxsEventContext(ctx, 42); return err }, func() error { _, err := GetTxsEvent(42); return err }},
		{"tx-hash", func(ctx context.Context) error {
			_, response := repairM882Entry(0)
			_, err := GetTxByHashContext(ctx, response.TxHash)
			return err
		}, func() error { _, response := repairM882Entry(0); _, err := GetTxByHash(response.TxHash); return err }},
	}
}

func TestRepairM_882_CallerDeadlineAndRetry(t *testing.T) {
	var stall atomic.Bool
	canceled := make(chan struct{}, 4)
	repairM882Serve(t, &repairM882Txs{total: 1}, func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if stall.Load() {
			<-ctx.Done()
			canceled <- struct{}{}
			return nil, status.FromContextError(ctx.Err()).Err()
		}
		return handler(ctx, req)
	})
	for _, operation := range repairM882Calls() {
		t.Run(operation.name, func(t *testing.T) {
			stall.Store(true)
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			start := time.Now()
			err := operation.call(ctx)
			if status.Code(err) != codes.DeadlineExceeded || time.Since(start) > time.Second {
				t.Fatalf("caller deadline not propagated: %v", err)
			}
			select {
			case <-canceled:
			case <-time.After(time.Second):
				t.Fatal("server did not receive cancellation")
			}
			stall.Store(false)
			if err := operation.call(context.Background()); err != nil {
				t.Fatalf("batch retry failed: %v", err)
			}
		})
	}
}

func TestRepairM_882_ScanCancellation(t *testing.T) {
	entered := make(chan struct{}, 1)
	repairM882Serve(t, &repairM882Txs{total: 205}, func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		entered <- struct{}{}
		<-ctx.Done()
		return nil, status.FromContextError(ctx.Err()).Err()
	})
	for _, operation := range repairM882Calls() {
		t.Run(operation.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			finished := make(chan error, 1)
			go func() { finished <- operation.call(ctx) }()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("request did not reach peer")
			}
			cancel()
			select {
			case err := <-finished:
				if status.Code(err) != codes.Canceled {
					t.Fatalf("scan cancellation lost: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("scan cancellation left query blocked")
			}
		})
	}
}

func TestRepairM_882_DefaultRequestDeadlines(t *testing.T) {
	deadlines := make(chan time.Duration, 4)
	repairM882Serve(t, &repairM882Txs{total: 1}, func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			deadlines <- 0
		} else {
			deadlines <- time.Until(deadline)
		}
		<-ctx.Done()
		return nil, status.FromContextError(ctx.Err()).Err()
	})
	var wg sync.WaitGroup
	for _, operation := range repairM882Calls() {
		wg.Add(1)
		go func(operation repairM882Call) {
			defer wg.Done()
			start := time.Now()
			if err := operation.legacy(); status.Code(err) != codes.DeadlineExceeded {
				t.Errorf("%s: expected deadline, got %v", operation.name, err)
			}
			if elapsed := time.Since(start); elapsed > requestTimeout+2*time.Second {
				t.Errorf("%s exceeded request bound: %v", operation.name, elapsed)
			}
		}(operation)
	}
	wg.Wait()
	for range repairM882Calls() {
		select {
		case remaining := <-deadlines:
			if remaining <= 0 || remaining > requestTimeout {
				t.Errorf("unbounded request deadline: %v", remaining)
			}
		case <-time.After(time.Second):
			t.Fatal("RPC did not reach deadline-checking peer")
		}
	}
}

func TestRepairM_882_LaterPageDeadline(t *testing.T) {
	var calls atomic.Int32
	repairM882Serve(t, &repairM882Txs{total: 205}, func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if calls.Add(1) == 2 {
			<-ctx.Done()
			return nil, status.FromContextError(ctx.Err()).Err()
		}
		return handler(ctx, req)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	result, err := GetTxsEventContext(ctx, 42)
	if result != nil || status.Code(err) != codes.DeadlineExceeded || calls.Load() != 2 {
		t.Fatalf("later page timeout escaped: result=%v error=%v calls=%d", result, err, calls.Load())
	}
	retry, err := GetTxsEventContext(context.Background(), 42)
	if err != nil || len(retry.Txs) != 205 {
		t.Fatalf("complete retry failed: %v, %v", retry, err)
	}
}

func TestRepairM_882_BlockResponses(t *testing.T) {
	repairM882Serve(t, &repairM882Txs{total: 1}, nil)
	latest, err := GetLatestBlock()
	if err != nil || latest.GetBlock().Header.Height != 42 {
		t.Fatalf("latest block response: %v, %v", latest, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	block, err := GetBlockByHeightContext(ctx, 17)
	if err != nil || block.GetBlock().Header.Height != 17 {
		t.Fatalf("requested height response: %v, %v", block, err)
	}
}

func TestRepairM_882_PerPageDeadlines(t *testing.T) {
	var mu sync.Mutex
	var deadlines []time.Time
	repairM882Serve(t, &repairM882Txs{total: 205}, func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > requestTimeout {
			return nil, status.Error(codes.Internal, "page has no bounded deadline")
		}
		mu.Lock()
		deadlines = append(deadlines, deadline)
		mu.Unlock()
		// Distinct request start times make deadline renewal observable.
		time.Sleep(20 * time.Millisecond)
		return handler(ctx, req)
	})
	result, err := GetTxsEventContext(context.Background(), 42)
	if err != nil || len(result.Txs) != 205 {
		t.Fatalf("paginated query: %v, %v", result, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(deadlines) != 3 {
		t.Fatalf("page deadlines=%v", deadlines)
	}
	for i := 1; i < len(deadlines); i++ {
		if !deadlines[i].After(deadlines[i-1]) {
			t.Fatalf("page %d did not receive its own deadline: %v", i, deadlines)
		}
	}
}
