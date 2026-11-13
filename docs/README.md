# Paxeer X Network documentation

Paxeer mainnet-beta is one BFT network with EVM chain ID `125` (`0x7d`).
The kernel runs inside `paxd` and participates in execution and block proposal.
A receipt is final when its block commits. Sub-chains are tenant domains in
the database.

Public node JSON-RPC is served at the root of each node endpoint listed in the
[public RPC reference](site/docs/reference/public-rpc.md). The node registers
`px` methods for account, inference, history and kernel reads;
`px_register` and `px_requestFunds` belong to the hosted gateway contract.

| Documentation | Contents |
| --- | --- |
| [Network overview](paxeer-network.md) | Chain identity, endpoints, tenant domains and build commands |
| [Public node JSON-RPC](site/docs/reference/public-rpc.md) | Node endpoints, namespaces and client guidance |
| [Hosted gateway OpenRPC](../services/platform/hosted/gateway/openrpc.json) | Hosted method, parameter, result and error contracts |
| [Documentation site](site/README.md) | MkDocs preview and build instructions |
| [Wiki](wiki/Home.md) | Module, program and agent reference pages |
| [Monorepo layout](MONOREPO.md) | Source directories, build targets and trust boundaries |
| [Qualification](QUALIFICATION.md) | Native, node, replay, fault and deployment coverage |
| [Agent compatibility](agent/COMPATIBILITY.md) | Daemon, contract and SDK compatibility |
| [Inference market](agent/inference-market/README.md) | Protocol, operator, client and threat guides |
| [EVM JSON-RPC limitations](evm_jsonrpc_unsupported.md) | Registered methods and their supported behavior |
| [CI secrets and runners](ci/secrets-and-runners.md) | Workflow configuration names and runner labels |
| [Platform](platform/) | Platform APIs and integration guides |
| [Wallet](wallet/) | Wallet application and service documentation |
| [Explorer](explorer/) | Explorer configuration and upstream documentation |
| [Launchpad](../deploy/launchpad/README.md) | Service images, configuration and operational tooling |
| [Perpetuals](perps/) | Perpetuals data API reference |
| [Interoperability](interop/) | Portability requirements and guidance |
| [Translations](readme/) | Translations of the root README |

Protocol specifications live under [spec/](../spec/).
