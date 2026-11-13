# Paxeer SDK

`sdk/` contains the Cosmos SDK fork embedded by `paxd` and the network's client SDKs. The Go fork supplies application execution, stores, modules, encoding and client context; the client packages provide transactions, receipt verification and service integrations. Paxeer mainnet-beta is one BFT network with EVM chain ID `125` (`0x7d`). The LayerX kernel runs inside `paxd`, participating in block proposal and execution, and a receipt is final once its covering block is committed.

## Layout

| Path | What it holds |
| --- | --- |
| [baseapp/](baseapp/) | ABCI application base: transaction execution, commit, halt handling and state sync snapshots |
| [chainflow/](chainflow/) | Chainflow data gateway clients for Go, Python, Rust and TypeScript |
| [client/](client/) | CLI client context, transaction signing, broadcasting and client configuration |
| [codec/](codec/) | Amino and Protobuf encoding and interface registration |
| [contrib/](contrib/) | Development tools, hooks, container definitions and Rosetta helpers |
| [cosmovisor/](cosmovisor/) | Process manager that selects node binaries at upgrade heights |
| [crypto/](crypto/) | Keys, keyring, signatures, HD derivation and Ledger integration |
| [ics23/](ics23/) | ICS-23 commitment proof library with its own Go module |
| [integrations/](integrations/) | Agent, Android, Express, FastAPI, iOS, Next.js and Spring integrations |
| [internal/](internal/) | Internal conversion helpers |
| [js/](js/) | `@pax-js/cosmos` package manifest and tests for generated types, REST clients and encoding exports |
| [launchpad/](launchpad/) | Launchpad client packages, protocol bindings and shared service libraries |
| [names/](names/) | Name hashing, manifests, content hashes and storage proof verification |
| [platform/](platform/) | Go, JVM, Swift and .NET clients, generators and conformance fixtures |
| [proto/](proto/) | Cosmos SDK Protobuf definitions |
| [scripts/](scripts/) | Module test, Protobuf, Swagger asset and changelog scripts |
| [server/](server/) | Node commands including start, export, rollback and tendermint, plus application configuration |
| [snapshots/](snapshots/) | State sync snapshot manager and on-disk snapshot store |
| [std/](std/) | Standard inter-module codec registration and time helpers |
| [store/](store/) | KV store wrappers and the original `rootmulti` multistore |
| [storev2/](storev2/) | The `rootmulti` multistore mounted by `paxd`, backed by [storage/](../storage/) |
| [tasks/](tasks/) | Concurrent transaction scheduler used by the node for parallel transaction delivery |
| [telemetry/](telemetry/) | Metrics and tracing helpers |
| [tests/](tests/) | Shared fixtures and mocks |
| [testutil/](testutil/) | Account, CLI, network, I/O and telemetry test helpers |
| [third_party/](third_party/) | Vendored Protobuf dependencies |
| [types/](types/) | Core SDK types, coins, addresses, transactions and execution context |
| [utils/](utils/) | Random, slice and tracing helpers |
| [version/](version/) | Build metadata and version command |
| [wallet/](wallet/) | Embedded wallet client, EIP-1193 provider, receipt verification and React hooks |
| [x/](x/) | Auth, authz, bank, capability, distribution, evidence, feegrant, genutil, gov, params, simulation, slashing, staking and upgrade modules |
| [Makefile](Makefile) | SDK package build, installation, test and code generation targets |
| [README.md](README.md) | SDK directory guide |

The chain application in [node/app.go](../node/app.go) wires the modules together. Chain-specific modules, including EVM, mint, oracle, tokenfactory and the kernel modules, live in [modules/](../modules/). IBC lives in [interchain/](../interchain/), and the consensus engine is in [consensus/](../consensus/).

## Build and test

The Cosmos SDK fork belongs to the root Go module declared in [go.mod](../go.mod), `github.com/Sidiora-Labs/Paxeer-X-Network`. Packages import it through `github.com/Sidiora-Labs/Paxeer-X-Network/sdk/...`. Go clients and libraries with their own manifests are separate modules: [cosmovisor](cosmovisor/go.mod), [ics23](ics23/go.mod), [Chainflow](chainflow/go/go.mod), [platform](platform/go/go.mod), [launchpad protocol](launchpad/protocol/go.mod) and [launchpad shared](launchpad/shared/go.mod). The root module's ICS-23 replacement resolves to the dependency named in its `replace` directive rather than to `sdk/ics23`.

Run these commands from the repository root. [chain.mk](../chain.mk) builds the native prerequisites and the node:

```sh
make -f chain.mk build
make -f chain.mk install
```

The build writes `build/paxd`; installation uses Go's `GOBIN` and `GOPATH` settings. The [SDK Makefile](Makefile) builds and installs packages using the root Go manifests and fork linker packages, and its unit test target supplies the SDK test build tags:

```sh
make -C sdk build
make -C sdk install
make -C sdk test-unit
```

These package targets cover the SDK packages in the root Go module. Client packages use their own manifests. The following commands come from the [names manifest](names/package.json), [wallet manifest](wallet/package.json), [JavaScript manifest](js/package.json) and [Chainflow TypeScript manifest](chainflow/typescript/package.json):

```sh
pnpm --dir sdk/names build
pnpm --dir sdk/names test
pnpm --dir sdk/wallet build
pnpm --dir sdk/wallet test
npm --prefix sdk/js test
npm --prefix sdk/chainflow/typescript run build
npm --prefix sdk/chainflow/typescript run typecheck
```

The wallet build and test scripts also install and build the agent TypeScript SDK. The JavaScript manifest provides a test script; its tests consume the distribution files declared by the package exports. See the [Chainflow Go](chainflow/go/README.md), [Python](chainflow/python/README.md) and [Rust](chainflow/rust/README.md) guides for their package commands.

## Configuration

The [server configuration loader](server/util.go) reads `config.toml` and `app.toml` under the node home's `config` directory. It binds environment variables using the executable name as a prefix and replaces dots and hyphens in configuration keys with underscores. Logging uses `PAX_LOG_LEVEL`, `PAX_LOG_FORMAT` and `PAX_LOG_OUTPUT`. The [client configuration loader](client/config/config.go) reads `client.toml`; its keys include `chain-id`, `keyring-backend`, `output`, `node` and `broadcast-mode`.

The [SDK Makefile](Makefile) accepts `LEDGER_ENABLED`, `COSMOS_BUILD_OPTIONS`, `BUILD_TAGS`, `LDFLAGS`, `BUILDDIR`, `BINDIR` and `GOPATH` for build configuration. Client packages hold language configuration in their manifests and TypeScript configuration files, including [names/tsconfig.json](names/tsconfig.json) and [wallet/tsconfig.json](wallet/tsconfig.json).

[Cosmovisor](cosmovisor/args.go) reads `DAEMON_HOME`, `DAEMON_NAME`, `DAEMON_ALLOW_DOWNLOAD_BINARIES`, `DAEMON_RESTART_AFTER_UPGRADE` and `DAEMON_LOG_BUFFER_SIZE`. The Chainflow clients read `REST_API_KEY`, `WS_MINT_KEY`, `WS_TOKEN` and `SIDIORALABS_BASE_URL`; the Go and Python clients also read `SIDIORALABS_CUSTOM_HEADERS`. TypeScript and Python logging use `SIDIORALABS_LOG`, and Python model initialization reads `DEFER_PYDANTIC_BUILD`.

Application clients receive node and service connection settings through their constructors. Connect to node JSON-RPC at the root HTTPS or WebSocket endpoint selected from the [public RPC reference](../docs/site/paxeer-docs/src/app/public-rpc/page.tsx), and check `eth_chainId` before signing. Hosted gateway methods have a separate [contract](../services/platform/hosted/gateway/openrpc.json).

Construct and sign an EIP-1559 transaction, then submit its exact bytes with `eth_sendRawTransaction`. Native precompiles dispatch calls to the modules and embedded kernel; the [Go Programs client](platform/go/programs.go) implements this path. Keep the signed bytes and returned transaction hash, and query `pax_sequencerReceipt` with that hash. Evidence progresses through `submitted`, `receipted` and `committed`. Verify receipt signatures against independently trusted receipt keys, bind the receipt to the transaction and verify committed block evidence before claiming finality. Inspect execution status as well: a committed failure remains a failure.

## Documentation

- [Repository layout and build ownership](../docs/MONOREPO.md)
- [SDK surfaces and exported symbols](../docs/system/sdk.md)
- [Documentation site: Cosmos SDK fork](../docs/site/paxeer-docs/src/app/sdk/page.tsx)
- [Documentation site: SDK quickstarts](../docs/site/paxeer-docs/src/app/sdk-quickstarts/page.tsx)
- [SDK site Markdown](../docs/site/paxeer-docs/public/markdown/sdk.md) and [quickstart Markdown](../docs/site/paxeer-docs/public/markdown/sdk-quickstarts.md)
- [Module specifications](x/README.md), with each module's `spec/` directory describing its behavior
- [Stores](store/README.md), [snapshots](snapshots/README.md), [server](server/README.md), [Cosmovisor](cosmovisor/README.md) and [scripts](scripts/README.md)
