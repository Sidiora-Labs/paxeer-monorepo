/*
Package inspect provides a tool for investigating the state of a
failed Tendermint node.

This package provides the Inspector type. The Inspector type runs a subset of the Tendermint
RPC endpoints that are useful for debugging issues with Tendermint consensus.

When a node running the Tendermint consensus engine detects an inconsistent consensus state,
the entire node will crash. The Tendermint consensus engine cannot run in this
inconsistent state so the node will not be able to start up again.

The RPC endpoints provided by the Inspector type allow for a node operator to inspect
the block store and state store to better understand what may have caused the inconsistent state.

The Inspector type's lifecycle is controlled by a context.Context. NewFromConfig
requires the full node configuration, including the existing node's root directory,
database settings and genesis file, and returns a construction error:

	nodeConfig := config.DefaultConfig().SetRoot("/path/to/node")
	ins, err := inspect.NewFromConfig(nodeConfig)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run blocks while serving; preserve its returned startup error.
	runErr := make(chan error, 1)
	go func() { runErr <- ins.Run(ctx) }()

	// Inspect the node, then cancel and wait for shutdown.
	cancel()
	if err := <-runErr; err != nil {
		return err
	}

Inspector serves its RPC endpoints on the address in the node's RPC configuration.
For a foreground server, use a context canceled by the interrupt signal:

	nodeConfig := config.DefaultConfig().SetRoot("/path/to/node")
	nodeConfig.RPC.ListenAddress = "tcp://127.0.0.1:26657"
	ins, err := inspect.NewFromConfig(nodeConfig)
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := ins.Run(ctx); err != nil {
		return err
	}

These snippets are function bodies in a caller that returns an error, using the
standard context, os and os/signal packages and the consensus config and inspect
packages. Canceling the context stops the servers. Run returns listener startup
errors promptly, after stopping its services, without canceling the caller's
context. Run also closes the databases opened by NewFromConfig before returning.

The list of available RPC endpoints can then be viewed by navigating to
http://127.0.0.1:26657/ in the web browser.
*/
package inspect
