<!-- id: beta_contract -->
<!-- readiness_claim: false -->

# Paxeer X Network mainnet-beta contract

This is the canonical beta contract of Paxeer X Network. The beta is
mainnet-beta: it runs on the Paxeer network, chain 125. Public node JSON-RPC
is served at the selected node's root path; see the
[public RPC guide](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/docs/site/docs/reference/public-rpc.md). Hosted services have their own operator-provided origins. LayerX
is the kernel domain inside
Paxeer X Network; it is not a separate network and has no separate beta.

The tables below identify the network and application components. Hosted service availability depends on the selected deployment.

## Identity

| Key | Value |
| --- | --- |
| id | beta_contract |
| product | Paxeer X Network |
| profile | mainnet-beta |
| readiness_claim | false |
| chain_id | 125 |
| rpc_path | node root path |
| go_module | github.com/Sidiora-Labs/Paxeer-X-Network |

## Network and endpoints

| Key | Value |
| --- | --- |
| chain | Paxeer network, EVM chain id 125 |
| rpc | configured node origin, JSON-RPC at the root path |
| emulator_endpoint | http://127.0.0.1:9402, local only |
| explorer | the Paxeer explorer fork |

Accounts are funded through chain transfers on chain 125. There is no public
faucet or control surface on the mainnet-beta.

## Surfaces

| Surface | Scope | Source |
| --- | --- | --- |
| paxeer-chain | paxd on chain 125 with the merged precompiles | daemon/paxd, node/, modules/, precompiles/ |
| gateway | hosted gateway JSON-RPC at its root path | services/platform/hosted/gateway |
| kernel | LayerX kernel domain services: core, receipt authority and agent boundaries | services/platform/hosted/node, services/platform/hosted/core, services/platform/hosted/authority, services/platform/hosted/agent-boundary |
| identity | principals, sessions, introspection and service tokens | services/platform/hosted/identity |
| registry | hosted program registry | services/platform/hosted/registry |
| webhooks | signed webhook deliveries | services/platform/hosted/webhooks |
| dashboard | developer dashboard API and web | services/platform/hosted/dashboard |
| indexer | decoded history of the kernel domain and Paxeer blocks | services/platform/hosted/indexer |
| human | Human API, intents, approvals and the wallet | services/human/, services/platform/hosted/human |
| interop | x402, AP2, UCP, Visa TAP, bridge relayer, gas station and web search | services/interop/crates |
| agent | layerx-agentd, MCP surface | services/agent/crates |
| sdks | TypeScript, Python, Rust, Go, JVM, Swift and .NET SDKs | services/agent/sdk, services/agent/crates/layerx-sdk, sdk/platform |
| platform-cli | CLI and local emulator | tools/platform/cli, tools/platform/emulator |
| programs | Programs runtime: deploy, paid call, restart | programs/crates |
| docs-site | documentation site and executable samples | docs/platform |

## Unknown-state behaviour

- An outcome a client did not observe stays unknown. Keep the idempotency key
  and transaction reference and reconcile the original submission through its
  journey or tracking API before issuing another payment.
- Paxeer degradation is never presented as kernel finality.
- Check the configured service readiness before sending application requests.

## Enforced by

| Capability | Layer | What that means here |
| --- | --- | --- |
| Atomic settlement | `protocol` | Each payment applies completely or not at all. |
| Honest degradation reporting | `hosted-surface` | Component readiness is reported separately. |
| Unknown is a real outcome | `agent-layer` | Unresolved submissions remain unknown until evidence resolves them. |
