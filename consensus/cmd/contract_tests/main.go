package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"

	jsonclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/client"
)

func main() {
	// This must be compiled beforehand and given to dredd as parameter, in the meantime the server should be running.
	port := flag.Int("port", 0, "The port that the hooks server will run on")
	remote := flag.String("rpc", "", "RPC endpoint of the application described by the fixtures")
	fixturesPath := flag.String("fixtures", "", "JSON fixtures containing funded, signed transactions and query/evidence inputs")
	flag.Parse()
	if *port == 0 || *remote == "" || *fixturesPath == "" {
		log.Fatal("-port, -rpc and -fixtures are required")
	}
	data, err := os.ReadFile(*fixturesPath)
	if err != nil {
		log.Fatal(err)
	}
	var fixtures Fixtures
	if err := json.Unmarshal(data, &fixtures); err != nil {
		log.Fatal(err)
	}
	client, err := jsonclient.New(*remote)
	if err != nil {
		log.Fatal(err)
	}
	runner, err := newHooksRunner(client, fixtures)
	if err != nil {
		log.Fatal(err)
	}
	mux, err := hookHandler(runner)
	if err != nil {
		log.Fatal(err)
	}
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	if err := http.Serve(listener, mux); err != nil {
		log.Fatal(err)
	}
}

func hookHandler(runner *HooksRunner) (http.Handler, error) {
	server := rpc.NewServer()
	if err := server.RegisterName("HooksRunner", runner); err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle("/", server)
	return mux, nil
}
