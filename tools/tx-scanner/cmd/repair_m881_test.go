package cmd

import (
	"context"
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	ctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/grpc/tmservice"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	pagination "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	txtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tools/tx-scanner/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestRepairM_881_F034InvalidThroughput(t *testing.T) {
	previous := client.GrpcConn
	client.GrpcConn = nil
	t.Cleanup(func() { client.GrpcConn = previous })
	for _, tc := range []struct {
		name string
		args []string
		flag string
	}{
		{"zero_batch", []string{"--batch-size=0"}, "batch-size"},
		{"negative_batch", []string{"--batch-size=-1"}, "batch-size"},
		{"zero_rate", []string{"--bps-limit=0"}, "bps-limit"},
		{"negative_rate", []string{"--bps-limit=-1"}, "bps-limit"},
		{"negative_rate_and_larger_batch", []string{"--batch-size=1000", "--bps-limit=-10"}, "bps-limit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := ScanCmd()
			if cmd.RunE == nil {
				t.Fatal("scan command must return validation errors to its caller")
			}
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tc.flag+" must be positive") {
				t.Fatalf("Execute() error = %v; want a positive %s validation error", err, tc.flag)
			}
			if client.GrpcConn != nil {
				t.Fatal("invalid throughput initialized the gRPC connection")
			}
		})
	}
}

type repairM881BlockFixture struct {
	tmservice.UnimplementedServiceServer
	block *tmproto.Block
}

func (s *repairM881BlockFixture) GetBlockByHeight(_ context.Context, req *tmservice.GetBlockByHeightRequest) (*tmservice.GetBlockByHeightResponse, error) {
	if req.Height != s.block.Header.Height {
		return nil, status.Error(codes.NotFound, "block not in fixture")
	}
	return &tmservice.GetBlockByHeightResponse{Block: s.block}, nil
}

type repairM881TxFixture struct {
	txtypes.UnimplementedServiceServer
	height     int64
	txs        []*txtypes.Tx
	responses  []*sdk.TxResponse
	byHash     map[string]*txtypes.GetTxResponse
	lowercase  bool
	failOffset *uint64
	mu         sync.Mutex
	offsets    []uint64
	lookups    []string
}

func (s *repairM881TxFixture) GetTxsEvent(_ context.Context, req *txtypes.GetTxsEventRequest) (*txtypes.GetTxsEventResponse, error) {
	if !reflect.DeepEqual(req.Events, []string{fmt.Sprintf("tx.height=%d", s.height)}) {
		return nil, status.Error(codes.InvalidArgument, "incorrect height event")
	}
	page, limit, err := pagination.ParsePagination(req.Pagination)
	if err != nil {
		return nil, err
	}
	offset := uint64((page - 1) * limit)
	s.mu.Lock()
	s.offsets = append(s.offsets, offset)
	s.mu.Unlock()
	if s.failOffset != nil && offset == *s.failOffset {
		return nil, status.Error(codes.Unavailable, "fixture index page unavailable")
	}
	start := int(offset)
	if start > len(s.txs) {
		start = len(s.txs)
	}
	end := start + limit
	if end > len(s.txs) {
		end = len(s.txs)
	}
	responses := make([]*sdk.TxResponse, 0, end-start)
	for _, response := range s.responses[start:end] {
		copy := *response
		if s.lowercase {
			copy.TxHash = strings.ToLower(copy.TxHash)
		}
		responses = append(responses, &copy)
	}
	return &txtypes.GetTxsEventResponse{
		Txs: s.txs[start:end], TxResponses: responses,
		Pagination: &pagination.PageResponse{Total: uint64(len(s.txs))},
	}, nil
}

func (s *repairM881TxFixture) GetTx(_ context.Context, req *txtypes.GetTxRequest) (*txtypes.GetTxResponse, error) {
	s.mu.Lock()
	s.lookups = append(s.lookups, req.Hash)
	s.mu.Unlock()
	response, ok := s.byHash[strings.ToUpper(req.Hash)]
	if !ok {
		return nil, status.Error(codes.NotFound, "transaction not in fixture")
	}
	return response, nil
}

func repairM881Fixtures(t *testing.T, count int) (*repairM881BlockFixture, *repairM881TxFixture) {
	t.Helper()
	const height = 37
	block := &repairM881BlockFixture{block: &tmproto.Block{Header: tmproto.Header{Height: height}}}
	index := &repairM881TxFixture{height: height, byHash: make(map[string]*txtypes.GetTxResponse)}
	for i := 0; i < count; i++ {
		tx := &txtypes.Tx{Body: &txtypes.TxBody{Memo: fmt.Sprintf("repair-m881-%d", i)}, AuthInfo: &txtypes.AuthInfo{}}
		raw, err := tx.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		block.block.Data.Txs = append(block.block.Data.Txs, raw)
		response := sdk.NewResponseResultTx(&ctypes.ResultTx{
			Hash: tmtypes.Tx(raw).Hash().Bytes(), Height: height, Index: uint32(i), Tx: raw,
		}, nil, "")
		index.txs = append(index.txs, tx)
		index.responses = append(index.responses, response)
		index.byHash[response.TxHash] = &txtypes.GetTxResponse{Tx: tx, TxResponse: response}
	}
	return block, index
}

func repairM881Serve(t *testing.T, block *repairM881BlockFixture, index *repairM881TxFixture) {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	tmservice.RegisterServiceServer(server, block)
	txtypes.RegisterServiceServer(server, index)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "passthrough:///repair-m881",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	previous := client.GrpcConn
	client.GrpcConn = conn
	t.Cleanup(func() { client.GrpcConn = previous; _ = conn.Close() })
}

func TestRepairM_881_F033CanonicalIndexComparison(t *testing.T) {
	for _, tc := range []struct {
		name        string
		count       int
		change      func(*repairM881BlockFixture, *repairM881TxFixture)
		wantBad     bool
		wantPages   int
		wantLookups int
	}{
		{name: "empty_block", count: 0, wantPages: 1},
		{name: "complete_single_page", count: 1, wantPages: 1, wantLookups: 1},
		{name: "exact_page_boundary", count: 100, wantPages: 1, wantLookups: 100},
		{name: "complete_three_pages", count: 205, wantPages: 3, wantLookups: 205},
		{name: "lowercase_hashes", count: 101, wantPages: 2, wantLookups: 101,
			change: func(_ *repairM881BlockFixture, index *repairM881TxFixture) { index.lowercase = true }},
		{name: "one_of_one_hundred_indexed", count: 100, wantBad: true, wantPages: 1,
			change: func(_ *repairM881BlockFixture, index *repairM881TxFixture) {
				index.txs = index.txs[:1]
				index.responses = index.responses[:1]
			}},
		{name: "none_indexed", count: 2, wantBad: true, wantPages: 1,
			change: func(_ *repairM881BlockFixture, index *repairM881TxFixture) {
				index.txs = nil
				index.responses = nil
			}},
		{name: "missing_on_last_page", count: 205, wantBad: true, wantPages: 3,
			change: func(_ *repairM881BlockFixture, index *repairM881TxFixture) {
				index.txs = index.txs[:204]
				index.responses = index.responses[:204]
			}},
		{name: "same_count_wrong_hash", count: 2, wantBad: true, wantPages: 1,
			change: func(block *repairM881BlockFixture, _ *repairM881TxFixture) {
				block.block.Data.Txs[1] = block.block.Data.Txs[1][:len(block.block.Data.Txs[1])-1]
			}},
		{name: "duplicate_hides_omission", count: 101, wantBad: true, wantPages: 2,
			change: func(_ *repairM881BlockFixture, index *repairM881TxFixture) {
				index.txs[100] = index.txs[0]
				index.responses[100] = index.responses[0]
			}},
		{name: "extra_index_entry", count: 1, wantBad: true, wantPages: 1,
			change: func(_ *repairM881BlockFixture, index *repairM881TxFixture) {
				index.txs = append(index.txs, index.txs[0])
				index.responses = append(index.responses, index.responses[0])
			}},
		{name: "hash_lookup_missing", count: 1, wantBad: true, wantPages: 1, wantLookups: 1,
			change: func(_ *repairM881BlockFixture, index *repairM881TxFixture) {
				delete(index.byHash, index.responses[0].TxHash)
			}},
		{name: "hash_lookup_mismatched", count: 2, wantBad: true, wantPages: 1,
			change: func(_ *repairM881BlockFixture, index *repairM881TxFixture) {
				first, second := index.responses[0].TxHash, index.responses[1].TxHash
				index.byHash[first], index.byHash[second] = index.byHash[second], index.byHash[first]
			}, wantLookups: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			block, index := repairM881Fixtures(t, tc.count)
			if tc.change != nil {
				tc.change(block, index)
			}
			repairM881Serve(t, block, index)
			bad, err := processBlock(block.block.Header.Height)
			if err != nil || bad != tc.wantBad {
				t.Fatalf("processBlock() = (%v, %v), want (%v, nil)", bad, err, tc.wantBad)
			}
			index.mu.Lock()
			defer index.mu.Unlock()
			if len(index.offsets) != tc.wantPages {
				t.Fatalf("requested offsets %v; want %d pages", index.offsets, tc.wantPages)
			}
			for i, offset := range index.offsets {
				if offset != uint64(i*pagination.DefaultLimit) {
					t.Fatalf("page %d offset = %d", i, offset)
				}
			}
			if len(index.lookups) != tc.wantLookups {
				t.Fatalf("looked up %d hashes; want %d", len(index.lookups), tc.wantLookups)
			}
		})
	}
}

func TestRepairM_881_F033LaterPageError(t *testing.T) {
	block, index := repairM881Fixtures(t, 101)
	offset := uint64(pagination.DefaultLimit)
	index.failOffset = &offset
	repairM881Serve(t, block, index)
	bad, err := processBlock(block.block.Header.Height)
	if bad || status.Code(err) != codes.Unavailable {
		t.Fatalf("processBlock() = (%v, %v); want the second-page query error", bad, err)
	}
	index.mu.Lock()
	defer index.mu.Unlock()
	if !reflect.DeepEqual(index.offsets, []uint64{0, offset}) || len(index.lookups) != 0 {
		t.Fatalf("offsets = %v, lookups = %v; want both pages and no premature success", index.offsets, index.lookups)
	}
}
