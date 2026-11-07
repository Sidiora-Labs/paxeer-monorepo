# Paxeer X Network documentation

This directory holds the documentation of Paxeer X Network: the MkDocs site, the
wiki tree the root [README.md](../README.md) links into, monorepo notes and the
Paxeer X chain RPC references. The hosted documentation is
[docs.paxeer.app](https://docs.paxeer.app/). Normative protocol text lives under
[`spec/`](../spec/), not here.

Paxeer mainnet-beta is one BFT network, with EVM chain ID `125` (`0x7d`).
The kernel runs inside `paxd` and participates in block proposal and execution;
a receipt is final when its block is committed. Sub-chains are tenant domains
in the database, rather than separately deployed networks.

Start integrations with the public node JSON-RPC at the root of
`https://api-mainnet-beta.paxeer.network` or the numbered nodes `api1` through
`api14.mainnet-beta.paxeer.network`. The node serves the `px` namespace at
the same root; `px_register` and `px_requestFunds` are hosted identity and
faucet operations defined in the gateway OpenRPC contract.
The former router and separate LayerX interface are retired.

The historical references below retain useful background, but their pre-merge
execution/settlement split and LNI write transport are retired. Use the public
node reference and checked-in gateway contract for current integrations.

| Path | Contents |
| --- | --- |
| [site/](site/) | MkDocs Material site (Overview, Concepts, Protocol, Programs, Agents, Human, Platform, Interop, Operators, Reference); preview and build instructions in [site/README.md](site/README.md) |
| [Public node JSON-RPC](site/docs/reference/public-rpc.md) | Root node endpoints, chain identity, namespaces, and client guidance |
| [Hosted gateway OpenRPC](../services/platform/hosted/gateway/openrpc.json) | Declared hosted JSON-RPC method, parameter, result, and error contracts; implementation in [gateway source](../services/platform/hosted/gateway/src/) |
| [site/build_wiki.py](site/build_wiki.py) | Renders `wiki/` to static HTML under `site/out/` and refuses broken relative links |
| [build/build_wiki.py](build/build_wiki.py) | Older wiki renderer writing to `docs/_site/`, with a `--check` flag |
| [wiki/README.md](wiki/README.md) | About the wiki tree |
| [wiki/Home.md](wiki/Home.md) | Wiki index |
| [wiki/Quickstart.md](wiki/Quickstart.md) | Quickstart |
| [wiki/Protocol.md](wiki/Protocol.md) | Historical activity-envelope and three-rule description; pre-merge topology and write transport are retired |
| [wiki/Modules.md](wiki/Modules.md) | Module index; one page per module under `wiki/` |
| [wiki/Fees.md](wiki/Fees.md) | Canonical fee schedule |
| [wiki/Programs.md](wiki/Programs.md) | Programs runtime, ABI, occupancy, LXT-20 |
| [wiki/LNI.md](wiki/LNI.md) | Historical LNI v1.7 reference; retired node transport |
| [wiki/AgentApi.md](wiki/AgentApi.md) | Agent client API reference; pre-merge LNI transport details are historical |
| [wiki/Mcp.md](wiki/Mcp.md) | MCP tool reference; current catalogue in [layerx-mcp source](../services/agent/crates/layerx-mcp/src/server.rs) |
| [wiki/Roadmap.md](wiki/Roadmap.md) | Beta surface expansion (in development, not shipped) |
| [MONOREPO.md](MONOREPO.md) | Monorepo layout and release notes; pre-merge component boundaries are historical |
| [QUALIFICATION.md](QUALIFICATION.md) | Evidence levels layered by risk |
| [Perps ABI change history](perps-abi-change-history.md) | Decision commit scope, the separate SDK implementation, and outstanding follow-up tasks |
| [readme/](readme/) | Translations of the root README |
| [runbooks/paxeer-x-fork.md](runbooks/paxeer-x-fork.md) | Validator runbook for the Paxeer X upgrade of the chain |
| [paxeer-network.md](paxeer-network.md) | Historical chain overview; the separate execution/settlement topology is retired |
| [paxeer-chain-docs.md](paxeer-chain-docs.md) | Chain documentation index: Swagger/OpenAPI and EVM JSON-RPC notes; pre-merge boundary descriptions are historical |
| [evm_jsonrpc_unsupported.md](evm_jsonrpc_unsupported.md) | EVM JSON-RPC methods that are registered but return a documented error |
| [swagger-ui/](swagger-ui/), [swagger/](swagger/) | Swagger UI assets, and the `statik.go` package that embeds them for `paxd` |
| [hyperfide/](hyperfide/) | Vendored Hyperfide project documentation: CLI, architecture, workflows and integrations |
| [platform/](platform/) | Platform developer documentation, API references, integration guides and documentation site tooling |
| [wallet/](wallet/) | Wallet [app](wallet/app/) development, architecture and operations, and [service](wallet/service/) security reviews and runbooks |
| [explorer/](explorer/) | Explorer build configuration, environment variables, contribution guides and upstream notes |
| [launchpad/](launchpad/) | Launchpad production readiness checklist |
| [perps/](perps/) | Perpetuals data API reference |
| [agent/](agent/) | Agent daemon compatibility matrix |
| [interop/](interop/) | Interoperability portability requirements and guidance |
