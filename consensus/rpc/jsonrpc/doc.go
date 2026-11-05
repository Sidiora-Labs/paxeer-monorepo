// HTTP RPC server supporting URI parameters, JSON-RPC over HTTP, and JSON-RPC
// over WebSocket.
//
// # Client Requests
//
// The complete server below exposes HelloWorld(context.Context, *HelloWorldRequest)
// as hello_world. Its typed request has name (a string) and num (an integer).
// All three transports call the same handler on the same listener.
//
// GET (URI)
//
// Pass URL-encoded parameters to the method's path:
//
//	curl 'http://localhost:8008/hello_world?name=%22my_world%22&num=5'
//
// The percent-encoded quotes preserve name as a JSON string. The URI response is
// the result object itself, {"message":"hi my_world 5"}, without a JSON-RPC envelope.
// A GET request to / returns a list of available endpoints and their parameters.
//
// POST (JSON-RPC)
//
// Send the same fields in the params object:
//
//	{
//	  "jsonrpc": "2.0",
//	  "id": "anything",
//	  "method": "hello_world",
//	  "params": {
//	    "name": "my_world",
//	    "num": 5
//	  }
//	}
//
// Save that JSON as data.json and send it to the listener's root path:
//
//	curl -H 'Content-Type: application/json' --data @data.json http://localhost:8008/
//
// The response is a JSON-RPC 2.0 envelope with the same id and the result object
// {"message":"hi my_world 5"} in result.
//
// WebSocket (JSON-RPC)
//
// Connect to ws://localhost:8008/websocket and send the same JSON object shown for
// POST as a text message. The response uses the same JSON-RPC envelope as POST.
// The server explicitly registers /websocket; choose a different path in both the
// server registration and the client URL if needed.
//
// # Server Definition
//
// This complete program registers the typed handler for all three transports.
// Listen takes the maximum number of open connections; Serve takes the context,
// listener, handler and server configuration. Interrupt or SIGTERM cancels the
// context and shuts down the HTTP server. Close WebSocket connections from the
// client when finished; HTTP shutdown does not close upgraded connections.
//
//	package main
//
//	import (
//	    "context"
//	    "errors"
//	    "fmt"
//	    "log"
//	    "net"
//	    "net/http"
//	    "os"
//	    "os/signal"
//	    "syscall"
//
//	    rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
//	)
//
//	type HelloWorldRequest struct {
//	    Name string `json:"name"`
//	    Num  int    `json:"num"`
//	}
//
//	type HelloWorldResult struct {
//	    Message string `json:"message"`
//	}
//
//	func HelloWorld(ctx context.Context, req *HelloWorldRequest) (*HelloWorldResult, error) {
//	    if err := ctx.Err(); err != nil {
//	        return nil, err
//	    }
//	    return &HelloWorldResult{Message: fmt.Sprintf("hi %s %d", req.Name, req.Num)}, nil
//	}
//
//	func serveRPC(ctx context.Context, listener net.Listener, config *rpcserver.Config) error {
//	    routes := map[string]*rpcserver.RPCFunc{
//	        "hello_world": rpcserver.NewRPCFunc(HelloWorld),
//	    }
//	    mux := http.NewServeMux()
//	    rpcserver.RegisterRPCFuncs(mux, routes)
//	    wm := rpcserver.NewWebsocketManager(routes)
//	    mux.HandleFunc("/websocket", wm.WebsocketHandler)
//	    return rpcserver.Serve(ctx, listener, mux, config)
//	}
//
//	func main() {
//	    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
//	    defer cancel()
//
//	    config := rpcserver.DefaultConfig()
//	    config.MaxOpenConnections = 100
//	    listener, err := rpcserver.Listen("tcp://127.0.0.1:8008", config.MaxOpenConnections)
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    defer listener.Close()
//
//	    if err := serveRPC(ctx, listener, config); err != nil && !errors.Is(err, http.ErrServerClosed) {
//	        log.Print(err)
//	    }
//	}
//
// For a Unix socket, replace the Listen address with unix:///path/to/socket and
// configure the client to dial that socket. Listen requires the tcp:// or
// unix:// prefix. With the TCP example running, GET http://localhost:8008/ lists
// hello_world, and the requests above reach it over URI, HTTP and WebSocket.
package jsonrpc
