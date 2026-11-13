# Paxeer Network documentation

These pages document the Paxeer node and chain modules in this monorepo. Node
integration, chain modules, EVM JSON-RPC, and contracts live at the repository root
in `node/`, `modules/`, `rpc/`, and `contracts/src/`.

Paxeer mainnet-beta is one BFT network running `paxd`, with EVM chain ID `125`
(`0x7d`). The merged kernel is part of the network and participates in block
proposal and execution. A receipt is final when its block is committed. Native
balances and kernel execution use chain state. Sub-chains are tenant domains in
the database, launchable without new infrastructure.

| Document | Contents |
| --- | --- |
| [`paxeer-network.md`](paxeer-network.md) | Node source layout and build entrypoints |
| [`evm_jsonrpc_unsupported.md`](evm_jsonrpc_unsupported.md) | EVM JSON-RPC methods that return documented errors |
| [Public RPC endpoints](site/paxeer-docs/src/app/public-rpc/page.tsx) | Root-path node endpoints and chain connection details |
| This page (below) | Regenerating and serving the node's OpenAPI / Swagger UI |
| [`../docker/README.md`](../docker/README.md) | Local Docker node and cluster |
| [`../contracts/README.md`](../contracts/README.md) | Foundry / Hardhat for chain contracts |
| [`../modules/README.md`](../modules/README.md) | Native chain modules and their integration |
| [`../tools/hpx/README.md`](../tools/hpx/README.md) | `paxd` distribution and peer registry tooling |

The kernel host is [`modules/lxphost/`](../modules/lxphost/), wired into the node
by [`node/app.go`](../node/app.go). Native funds use the
[bank module](../sdk/x/bank/) and [ledger](../modules/layerxledger/).
Transfers to external chains use [`layerxbridge`](../modules/layerxbridge/).
These are components of the merged network, rather than a second settlement tree.

Public EVM JSON-RPC uses HTTPS at the selected node's root path; see the
[public RPC guide](site/paxeer-docs/src/app/public-rpc/page.tsx) for connection details.
The HTTP server registers `debug`, `echo`, `eth`, `net`, `pax`, `pax2`, `px`,
`txpool` and `web3`; method availability depends on node configuration. The `test`
namespace additionally requires `EnableTestAPI` and a non-live chain ID. The `px` namespace answers from node state; `px_register` and
`px_requestFunds` are operations of the hosted identity and faucet
services declared in [the gateway OpenRPC contract](../services/platform/hosted/gateway/openrpc.json).

Commands below are run from the repository root unless noted.

## OpenAPI/Swagger docs generation

Regenerate the OpenAPI document and embedded UI when the APIs or protobuf
services change. [OpenAPI](https://github.com/OAI/OpenAPI-Specification/) is the
specification; Swagger UI renders it. This guide covers the Cosmos API server's
OpenAPI document, separately from EVM JSON-RPC and the hosted gateway contract.

The checked-in [generation script](../scripts/update-swagger-ui-statik.sh) invokes
`ignite generate openapi -y`, stages `docs/swagger-ui/`, and embeds the staged
assets with `statik`. Its [asset helper](../scripts/swagger_assets.py) validates
the archive and registers it in both the default and `swagger` namespaces.

### Prerequisites

Schema generation requires an Ignite installation supporting
`ignite generate openapi -y` and its project configuration. To embed the
checked-in schema without invoking Ignite, use `--assets-only` below.

The script requires Python 3 and `statik` on `PATH`. `STATIK` can select another
statik executable. The version used in `go.mod` is
`github.com/rakyll/statik v0.1.7`:

```bash
go install github.com/rakyll/statik@v0.1.7
```

### Running the script

To regenerate the schema and embedded assets, run:

```bash
./scripts/update-swagger-ui-statik.sh
```

To embed the existing assets and schema, run:

```bash
./scripts/update-swagger-ui-statik.sh --assets-only
```

The OpenAPI document is `docs/swagger-ui/swagger.yml`; the UI initializer loads
it as `./swagger.yml`. Staging adds an identical `swagger.yaml` alias to the
archive. The script runs `statik` with `-f -m -p swagger`, validates the schema
aliases and required UI assets, and atomically replaces `docs/swagger/statik.go`.
It removes its staging directory on exit. Rebuild `paxd` using the repository's
`make paxd-build` target to serve the embedded assets from a new binary.

## Serving the OpenAPI docs on the node

Enable the Cosmos API server and Swagger in the node home's `config/app.toml`:

```toml
[api]
enable = true
swagger = true
address = "tcp://127.0.0.1:1317"
```

This example binds the API server locally. With a rebuilt node restarted using
that configuration, open `http://127.0.0.1:1317/swagger/`. For another binding,
use the host and port from `[api].address`; retain the trailing slash so the UI
can resolve its relative asset paths.

[`App.RegisterAPIRoutes`](../node/app.go) registers Swagger only when
`apiConfig.Swagger` is true. `RegisterSwaggerAPI` loads the embedded `swagger`
namespace and serves it under `/swagger/`. This is the Cosmos API listener,
not the public EVM JSON-RPC listener. Enabling Swagger locally does not publish
it on the public node endpoints.
