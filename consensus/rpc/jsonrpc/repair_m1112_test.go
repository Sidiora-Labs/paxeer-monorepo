package jsonrpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/doc/comment"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
)

type HelloWorldRequest struct {
	Name string `json:"name"`
	Num  int    `json:"num"`
}

type HelloWorldResult struct {
	Message string `json:"message"`
}

func HelloWorld(ctx context.Context, req *HelloWorldRequest) (*HelloWorldResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &HelloWorldResult{Message: fmt.Sprintf("hi %s %d", req.Name, req.Num)}, nil
}

func serveRPC(ctx context.Context, listener net.Listener, config *rpcserver.Config) error {
	routes := map[string]*rpcserver.RPCFunc{
		"hello_world": rpcserver.NewRPCFunc(HelloWorld),
	}
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, routes)
	wm := rpcserver.NewWebsocketManager(routes)
	mux.HandleFunc("/websocket", wm.WebsocketHandler)
	return rpcserver.Serve(ctx, listener, mux, config)
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	config := rpcserver.DefaultConfig()
	config.MaxOpenConnections = 100
	listener, err := rpcserver.Listen("tcp://127.0.0.1:8008", config.MaxOpenConnections)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	if err := serveRPC(ctx, listener, config); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Print(err)
	}
}

// repairM1112Blocks reads the published examples rather than maintaining separate
// request fixtures that could silently drift away from the documentation.
func repairM1112Blocks(t *testing.T) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "doc.go", nil, parser.ParseComments)
	require.NoError(t, err)
	require.NotNil(t, file.Doc)
	var p comment.Parser
	var blocks []string
	for _, block := range p.Parse(file.Doc.Text()).Content {
		if code, ok := block.(*comment.Code); ok {
			blocks = append(blocks, code.Text)
		}
	}
	return blocks
}

func TestRepairM_1112_DocumentedProgram(t *testing.T) {
	var program string
	for _, block := range repairM1112Blocks(t) {
		if strings.HasPrefix(block, "package main\n") {
			require.Empty(t, program, "exactly one complete server program")
			program = block
		}
	}
	require.NotEmpty(t, program, "publish a complete runnable server example")
	documented, err := parser.ParseFile(token.NewFileSet(), "example.go", program, 0)
	require.NoError(t, err)
	compiled, err := parser.ParseFile(token.NewFileSet(), "repair_m1112_test.go", nil, 0)
	require.NoError(t, err)

	render := func(node ast.Node) string {
		var out bytes.Buffer
		require.NoError(t, printer.Fprint(&out, token.NewFileSet(), node))
		return out.String()
	}
	// Each declaration in the published program must be the exact declaration
	// compiled here, including main's connection limit and lifecycle calls.
	var checked []string
	for _, decl := range documented.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.IMPORT {
			for _, spec := range gen.Specs {
				want := render(spec)
				var found bool
				for _, got := range compiled.Imports {
					found = found || render(got) == want
				}
				require.True(t, found, "documented import must compile: %s", want)
			}
			continue
		}
		want := render(decl)
		var found bool
		for _, got := range compiled.Decls {
			found = found || render(got) == want
		}
		require.True(t, found, "documented declaration must match compiled code:\n%s", want)
		checked = append(checked, want)
	}
	require.Len(t, checked, 5, "request, result, handler, serving function and main")
}

func repairM1112Server(t *testing.T) (string, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	config := rpcserver.DefaultConfig()
	config.MaxOpenConnections = 100
	listener, err := rpcserver.Listen("tcp://127.0.0.1:0", config.MaxOpenConnections)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	exited := make(chan error, 1)
	go func() { exited <- serveRPC(ctx, listener, config) }()
	stop := sync.OnceFunc(func() {
		cancel()
		select {
		case err := <-exited:
			require.ErrorIs(t, err, http.ErrServerClosed)
		case <-time.After(5 * time.Second):
			t.Fatal("Serve did not stop after context cancellation")
		}
	})
	t.Cleanup(stop)
	return listener.Addr().String(), stop
}

func TestRepairM_1112_DocumentedTransports(t *testing.T) {
	address, stop := repairM1112Server(t)
	httpClient := &http.Client{Timeout: 5 * time.Second}
	t.Cleanup(httpClient.CloseIdleConnections)
	var uri string
	var body []byte
	for _, block := range repairM1112Blocks(t) {
		if strings.HasPrefix(block, "curl 'http://localhost:8008/hello_world?") {
			uri = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(block), "curl '"), "'")
		}
		if strings.HasPrefix(block, "{\n") {
			body = []byte(block)
		}
	}
	require.NotEmpty(t, uri, "URI example must be published")
	require.NotEmpty(t, body, "JSON-RPC example must be published")
	var request rpctypes.RPCRequest
	require.NoError(t, json.Unmarshal(body, &request))
	require.Equal(t, "hello_world", request.Method)
	require.Equal(t, `"anything"`, request.ID())

	checkResult := func(t *testing.T, bits []byte) {
		t.Helper()
		var got HelloWorldResult
		require.NoError(t, json.Unmarshal(bits, &got))
		require.Equal(t, "hi my_world 5", got.Message)
	}
	checkEnvelope := func(t *testing.T, bits []byte) {
		t.Helper()
		var got rpctypes.RPCResponse
		require.NoError(t, json.Unmarshal(bits, &got))
		require.Nil(t, got.Error)
		require.Equal(t, request.ID(), got.ID())
		var envelope struct {
			Version string `json:"jsonrpc"`
		}
		require.NoError(t, json.Unmarshal(bits, &envelope))
		require.Equal(t, "2.0", envelope.Version)
		checkResult(t, got.Result)
	}
	readHTTP := func(t *testing.T, response *http.Response) []byte {
		t.Helper()
		defer response.Body.Close()
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Equal(t, "application/json", response.Header.Get("Content-Type"))
		bits, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		return bits
	}

	t.Run("URI", func(t *testing.T) {
		response, err := httpClient.Get(strings.Replace(uri, "localhost:8008", address, 1))
		require.NoError(t, err)
		bits := readHTTP(t, response)
		checkResult(t, bits)
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(bits, &fields))
		require.Len(t, fields, 1, "URI returns only the result, without an RPC envelope")
		require.Contains(t, fields, "message")
	})
	t.Run("HTTP", func(t *testing.T) {
		response, err := httpClient.Post("http://"+address+"/", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		checkEnvelope(t, readHTTP(t, response))
	})
	t.Run("WebSocket", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn, response, err := websocket.DefaultDialer.DialContext(ctx, "ws://"+address+"/websocket", nil)
		require.NoError(t, err)
		defer conn.Close()
		defer response.Body.Close()
		require.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
		require.NoError(t, conn.SetWriteDeadline(time.Now().Add(5*time.Second)))
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, body))
		kind, bits, err := conn.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.TextMessage, kind)
		checkEnvelope(t, bits)
	})

	// The real lifecycle must close the listener when the context is cancelled.
	stop()
	_, err := httpClient.Get("http://" + address + "/hello_world?name=%22my_world%22&num=5")
	require.Error(t, err, "cancelled server must refuse new connections")
}
