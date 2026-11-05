package main

import (
	"context"
	"flag"
	"fmt"
	stdlog "log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	"github.com/paxeer-network/paxlog"
)

var logger = paxlog.NewLogger("tendermint", "rpc", "jsonrpc", "test")

var routes = map[string]*rpcserver.RPCFunc{
	"hello_world": rpcserver.NewRPCFunc(HelloWorld),
}

type HelloWorldRequest struct {
	Name string `json:"name"`
	Num  int    `json:"num"`
}

func HelloWorld(ctx context.Context, req *HelloWorldRequest) (Result, error) {
	return Result{fmt.Sprintf("hi %s %d", req.Name, req.Num)}, nil
}

type Result struct {
	Result string
}

func serveExample(ctx context.Context, listener net.Listener, config *rpcserver.Config) error {
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, routes)
	return rpcserver.Serve(ctx, listener, mux, config)
}

func main() {
	listenAddress := flag.String("listen", "tcp://127.0.0.1:8008", "RPC listen address")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	config := rpcserver.DefaultConfig()
	listener, err := rpcserver.Listen(*listenAddress, config.MaxOpenConnections)
	if err != nil {
		stdlog.Fatalf("rpc listening: %v", err)
	}
	fmt.Println("RPC listening on http://" + listener.Addr().String())

	if err = serveExample(ctx, listener, config); err != nil {
		logger.Error("rpc serve", "err", err)
	}
}
