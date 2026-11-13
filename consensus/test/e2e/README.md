# End-to-End Tests

This harness configures local consensus-engine testnets from TOML manifests and uses Docker Compose to run them with the test application in [`app/`](app/). Paxeer embeds this engine in `paxd`; the harness exercises the engine and test application rather than the full mainnet application. See the [consensus overview](../../README.md) and the [native fuzz targets](../fuzz/README.md) for related material.

Run the commands on this page from `consensus/test/e2e/`. The Go module is rooted at the repository root; its [`go.mod`](../../../go.mod) declares Go 1.25.8. The runner invokes the `docker` and `docker-compose` executables.

Build the `tendermint/e2e-node` image using the repository root as its context, as required by [`docker/Dockerfile`](docker/Dockerfile):

```sh
docker build --tag tendermint/e2e-node -f docker/Dockerfile ../../..
```

The image uses Go 1.25.8 on Debian Bookworm and builds the embedded node and test application as `/usr/bin/app`, with LevelDB and RocksDB development libraries installed. Use the explicit command above for the repository-root context.

The individual Go tool targets are:

```sh
make generator runner tests node
```

The `node` target enables the `badgerdb,boltdb,cleveldb,rocksdb` build tags and requires the corresponding native database dependencies. These targets are defined in the local [`Makefile`](Makefile).

After building the image, select the CI manifest with:

```sh
./build/runner -f networks/ci.toml
```

This uses a testnet named `ci` under `networks/ci/`. The default run cleans up existing testnet state first, runs the stages below, then cleans up after success. Pass `--preserve` to retain a successful run; failed runs retain their state for debugging.

## Conceptual Overview

End-to-end testnets exercise consensus functionality through nodes with different configurations and check that the nodes and network behave correctly. The upstream background is outlined in [ADR-066](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-066-e2e-testing.md).

The suite asks:

1. Does a valid testnet configuration produce blocks, with all nodes eventually catching up?
2. Does each node satisfy the invariants specified by the Go E2E tests?

These are the intended invariants for valid network configurations. Handwritten manifests test specific configurations; randomly generated manifests explore permutations, using the runtime protocol described below.

The default run sets up the testnet, starts a background transaction load, starts the nodes, and waits for blocks and node catch-up. It applies configured perturbations, such as killing or disconnecting nodes, followed by evidence injection when requested. Finally, it stops the load, waits for the network to settle, and runs the Go tests. The orchestration is in [`runner/main.go`](runner/main.go).

The tests use the consensus RPC client to check externally visible behavior, such as blocks being identical across nodes. In [`tests/e2e_test.go`](tests/e2e_test.go), `testNode()` runs a subtest for each stateful node, currently sequentially, and `fetchBlockChain()` retrieves history from the freshest archive node.

Tests must account for the node configuration and for a live network that continues producing blocks. For example, validator tests must select validators and respect block retention and state sync when querying history.

## Testnet Manifests

Testnets are specified as TOML manifests. For an example see [`networks/ci.toml`](networks/ci.toml), and for field documentation see [`pkg/manifest.go`](pkg/manifest.go). Loading and validation live in [`pkg/testnet.go`](pkg/testnet.go).

Use `abci_protocol = "builtin"` for every node. The runtime embeds the test application and accepts this protocol only. Select this value in generated manifests before running them.

## Random Testnet Generation

Generate random testnet manifests with `generator`:

```sh
./build/generator -d networks/generated/

# Split networks into 8 groups (by filename)
./build/generator -g 8 -d networks/generated/
```

[`generator/main.go`](generator/main.go) uses the fixed seed `4827085738`; the CLI has no seed flag. Save the generated TOML manifest and pass that saved file to `runner -f` to reuse a configuration. Map iteration in the generator can affect choices across processes even with the same seed; a saved manifest preserves the configuration inputs.

With a working Docker image and supported manifests, [`run-multiple.sh`](run-multiple.sh) runs testnets sequentially:

```sh
./run-multiple.sh networks/generated/gen-group03-*.toml
```

Group numbers are zero-based and padded to two digits. The script dumps logs and cleans up a failed testnet before continuing, then exits nonzero if any run failed. Capture its output to retain those diagnostics.

## Test Stages

Execute a stage with `./build/runner -f <manifest> <stage>`:

- `setup`: generates the testnet directory, configuration, genesis and Compose files.
- `start`: sets up the directory if absent, then starts containers and waits for nodes.
- `load`: generates transaction load until canceled.
- `evidence [amount]`: generates and broadcasts evidence to a random node; defaults to one item.
- `pause` / `resume`: pauses or resumes the Docker testnet.
- `perturb`: applies requested perturbations, such as node restarts or network disconnects.
- `wait`: waits for five more blocks and node catch-up.
- `test`: runs the compiled tests against the running testnet.
- `stop`: stops and removes containers with Compose `down`.
- `cleanup`: removes testnet files and Docker containers/networks.

Auxiliary commands:

- `logs [node]`: outputs all node logs, or those of the selected node.
- `tail [node]`: follows logs until canceled.

## Tests

Test cases are normal Go tests in [`tests/`](tests/). The runner executes the compiled `build/tests` binary with `-test.count=1 -test.v`, setting `E2E_MANIFEST` to the selected manifest.

### Running Manual Tests

After building the Docker image, start the testnet and set `E2E_MANIFEST` to an absolute path so it also resolves from the Go test package's working directory:

```sh
./build/runner -f networks/ci.toml start
E2E_MANIFEST="$PWD/networks/ci.toml" go test -count=1 -v ./tests/...
```

Optionally, `E2E_NODE` selects a single node for tests using `testNode()`. Set these variables in your editor's test environment when running from an IDE. Without `E2E_MANIFEST`, the live-network tests skip; that is not a successful testnet run.

### Debugging Failures

If a command or test fails, the runner exits with an error and nonzero status. A failed default run preserves its testnet state. Inspect it with `docker ps`, `docker logs`, or `./build/runner -f <manifest> logs` or `tail`. To shut down and remove the testnet, use `./build/runner -f <manifest> cleanup`.

Set `log_level` in the manifest for more detailed node logging.

The generated node configuration sets a [pprof](https://pkg.go.dev/net/http/pprof) listener on port 6060, which the Compose file publishes. For a running node that serves it, find the mapped port with `docker port <NODENAME> 6060`. For an IPv4 loopback mapping, these queries inspect the available profiles:

```sh
PORT=$(docker port full01 6060/tcp | awk -F: '/^0\.0\.0\.0:/ {print $NF; exit}')
go tool pprof "http://localhost:$PORT/debug/pprof/goroutine"
go tool pprof "http://localhost:$PORT/debug/pprof/heap"
go tool pprof "http://localhost:$PORT/debug/pprof/threadcreate"
go tool pprof "http://localhost:$PORT/debug/pprof/block"
go tool pprof "http://localhost:$PORT/debug/pprof/mutex"
```

See this [pprof introduction](https://jvns.ca/blog/2017/09/24/profiling-go-with-pprof/) for background.

## Enabling IPv6

The manifest's `ipv6` field selects IPv6 addressing in the generated Compose network. Docker does not enable IPv6 by default. Configure the Docker daemon, for example in `daemon.json` or Docker Desktop's Docker Engine settings:

```json
{
  "ipv6": true,
  "fixed-cidr-v6": "2001:db8:1::/64"
}
```

## Benchmarking Testnets

After building the Docker image, `./build/runner -f <manifest> benchmark` sets up a testnet, generates load, starts nodes, waits for catch-up, and samples 100 blocks. It reports the mean, standard deviation, minimum and maximum block intervals, along with transaction and sampling statistics. It does not apply perturbations and cleans up on exit. See [`runner/benchmark.go`](runner/benchmark.go).

## Running Individual Nodes

The harness also builds a node executable that embeds the test application directly. Given an already initialized engine home with compatible configuration, genesis and validator files:

```sh
make node
TMHOME=$HOME/.tendermint ./build/node ./node/built-in.toml
```

The node loads its engine configuration from `$TMHOME/config/config.toml`; the command does not initialize that home. The runner's `setup` stage writes per-node homes under the testnet directory and application settings in each node's `config/app.toml`.

[`node/built-in.toml`](node/built-in.toml) is an example of the test application's settings. [`node/config.go`](node/config.go) aliases the definitions and validation in [`node/runtime/config.go`](node/runtime/config.go); startup and home loading are in [`node/runtime/node.go`](node/runtime/node.go). Only the `builtin` application protocol is supported.
