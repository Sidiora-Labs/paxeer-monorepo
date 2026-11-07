# Paxeer Network documentation

These pages document the Paxeer node and chain modules in this monorepo. Node
integration, chain modules, EVM JSON-RPC, and contracts live at the repository root
in `node/`, `modules/`, `rpc/`, and `contracts/src/`.

Paxeer mainnet-beta is one BFT network running `paxd`, with EVM chain ID `125`
(`0x7d`). The merged kernel is part of the network and participates in block
proposal and execution. A receipt is final when its block is committed. Native
balances and kernel execution use chain state; separate custody deposits and
checkpoint settlement are retired. Sub-chains are tenant domains in the database,
launchable without new infrastructure.

| Document | Contents |
| --- | --- |
| [`paxeer-network.md`](paxeer-network.md) | Node source layout and build entrypoints |
| [`evm_jsonrpc_unsupported.md`](evm_jsonrpc_unsupported.md) | EVM JSON-RPC methods that return documented errors |
| [Public RPC endpoints](site/docs/reference/public-rpc.md) | Root-path node endpoints and chain connection details |
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

Public EVM JSON-RPC uses HTTPS at the root of
`https://api-mainnet-beta.paxeer.network` or the numbered nodes
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`. The node namespaces are `debug`,
`echo`, `eth`, `net`, `pax`, `test`, and `txpool`; availability depends on node
configuration. Hosted gateway `px_` methods are defined separately in
[`services/platform/hosted/gateway/openrpc.json`](../services/platform/hosted/gateway/openrpc.json).
`node.hyperpaxeer.com` is only the hpx library mirror, never a JSON-RPC endpoint.

Commands below are run from the repository root unless noted.

## OpenAPI/Swagger docs generation

Regenerate the OpenAPI document and embedded UI when the APIs or protobuf
services change. [OpenAPI](https://github.com/OAI/OpenAPI-Specification/) is the
specification; Swagger UI renders it. This guide covers the Cosmos API server's
OpenAPI document, separately from EVM JSON-RPC and the hosted gateway contract.

The checked-in [generation script](../scripts/update-swagger-ui-statik.sh) invokes
`ignite generate openapi -y`, embeds `docs/swagger-ui/` with `statik`, and changes
the generated registration to the `swagger` namespace used by the node.

### Prerequisites

The existing generation workflow calls for Ignite CLI **v0.23.0**. Obtain that
version from its [release page](https://github.com/ignite/cli/releases/tag/v0.23.0)
and confirm it with `ignite version`. The monorepo does not track a root Ignite
project configuration; supply the generation configuration expected by that CLI
before regenerating, rather than treating this script as a complete project
bootstrap.

The script also requires `statik` on `PATH`. The version used in `go.mod` is
`github.com/rakyll/statik v0.1.7`:

```bash
go install github.com/rakyll/statik@v0.1.7
```

### Running the script

On a system with BSD `sed`, run:

```bash
./scripts/update-swagger-ui-statik.sh
```

The script's final `sed -i ''` command uses BSD syntax. On GNU/Linux, run the
corresponding steps with GNU `sed` instead, stopping if generation fails:

```bash
ignite generate openapi -y &&
  statik -src=./docs/swagger-ui -dest=./docs -f -p swagger &&
  sed -i 's/fs.Register(data)/fs.RegisterWithNamespace("swagger", data)/g' docs/swagger/statik.go
```

The OpenAPI document is `docs/swagger-ui/swagger.yml`; the UI initializer loads
it as `./swagger.yml`. `statik` embeds that document and the other files in
`docs/swagger-ui/` into `docs/swagger/statik.go`. Review the generated document
and embedded assets, then rebuild `paxd` using the repository's `make paxd-build`
target. Changing the YAML or generated Go file alone does not update an already
built binary.

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
