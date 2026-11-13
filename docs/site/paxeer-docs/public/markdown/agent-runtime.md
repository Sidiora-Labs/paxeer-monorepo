<!-- Source: https://docs.paxeer.app/agent-runtime/ -->

# Agent runtime

Run the agent daemon, enrol an identity, connect MCP runtimes, and preserve policy and receipt continuity across restarts.

The Paxeer X agent runtime connects autonomous software to LayerX activity execution through a controlled daemon boundary. `layerx-agentd` owns sessions, capabilities, approval holds, local limits, and durable evidence. MCP exposes those controls as tools. Actions reach the node as signed EIP-1559 transactions that call native precompiles; the hosted agent boundary serves verified execution receipts.

## Runtime components

| Component | Responsibility | Trust boundary |
| --- | --- | --- |
| `layerx-agentd` | Identity enrolment, session authentication, capability checks, local policy, and evidence verification. | Protected tenant store, human owner, and daemon credentials. |
| `layerx-mcp` | Daemon-bound tool catalogue for reads and actions. | One session and capability; no separate MCP write authority. |
| `layerx mcp serve` | Serve the bound catalogue over standard input and output. | Protected daemon binding document. |
| `layerx-agent-boundary` | TLS receipt lookup and signed deployment evidence. | Separate gateway, registry, and webhook credential planes. |

## 1. Configure the network profile

Only the profile name is positional. To configure a profile, supply the endpoint, network identifier, and receipt trust anchor together. Omitting all three reuses a configured profile. Supply either a literal trust anchor or a trust-anchor file. The CLI retains the implementation name `layerx`.

```
layerx environment use beta \
  --endpoint <https-endpoint> \
  --network-id <network-id> \
  --sequencer-trust-anchor <hex>

# The trust anchor may instead be read from a file:
#   --sequencer-trust-anchor-file <path>
```

The profile names are `emulator`, `beta`, and `production`. Non-loopback endpoints use HTTPS. A profile controls CLI network access. The daemon configuration uses `node_endpoint` or `LAYERX_NODE_ENDPOINT` for the node root HTTP(S) URL.

## 2. Enrol the daemon identity

Configure the agent DID, tenant, authority, capability, session identity, and binding output under the daemon's `LAYERX_AGENT_MCP_*` settings. Setting `LAYERX_AGENT_MCP_BINDING_ROOT` enables binding publication. The daemon resolves the DID through the human authority, registers the identity, restores the named capability, opens a session, and publishes the runtime binding before accepting requests. These are daemon lifecycle operations, not a `layerx` session-opening command. A publication failure closes the newly opened session; restart restores the named session and its matching document instead of creating duplicate credentials.

| Binding artifact | Contents | Protection |
| --- | --- | --- |
| `binding.json` | Mode, tenant, store, audit root, session and capability references, daemon endpoint, credential file paths, local limit context, the protected Unix listener socket and admitted peer UIDs, and an optional web-search sidecar reference. | Owner-only binding directory and file access. |
| `session-token` | OS-random session Bearer token. | Separate protected file; excluded from host configuration. |
| `daemon-bearer` | Daemon transport authentication. | Separate protected file; excluded from the binding document. |

Binding directories use mode `0700` and files use `0600`. Existing secret files are refused instead of overwritten. A different binding at an existing publication path is also refused.

## 3. Install the runtime connection

Install MCP after daemon enrolment publishes its binding. Supported hosts are `layerx`, `claude-code`, `claude-desktop`, `cursor`, and `vscode`. Repeat `--host` to install into multiple runtimes.

```
layerx install mcp --host claude-code \
  --daemon-binding /absolute/path/to/binding.json

# Restrict the installed tool surface to reads.
layerx install mcp --host cursor --read-only \
  --daemon-binding /absolute/path/to/binding.json

# Run the same stdio server directly.
layerx mcp serve --daemon-binding /absolute/path/to/binding.json
```

The installer opens and validates the same document used at serve time. Relative explicit paths become absolute. Without an explicit binding path, installation resolves `mcp/binding.json` beside the CLI configuration. Host launch configuration holds the executable, arguments, and an empty process environment; secret material stays in protected daemon files.

MCP installation uses the binding's session and capability. It does not provision a gateway credential or ask for a source account, asset, signing seed, or network profile. The read-only flag narrows the declared binding mode. Tool discovery exposes only scopes admitted by the bound session.

## 4. Admit an action

Each action passes session authentication, capability evaluation, policy, rate and quota limits, deadlines, and budget reservation before submission. Capability checks evaluate expiry, activity type, counterparty, asset, amount, rate, and purpose. An approval hold retains the exact canonical preparation and its reservation until an authorized decision or expiry.

Use the [agent action lifecycle](https://docs.paxeer.app/agents) to prepare, review, sign, submit, and track. Requests carry stable idempotency keys. Repeating an identical mutation returns its original result; changing the body under the same key produces an idempotency conflict.

## Submission and receipt recovery

Submit the signed EIP-1559 transaction to the node with `eth_sendRawTransaction`. Retain its transaction hash and the daemon preparation and submission references. The hosted boundary retrieves signed receipts by transaction hash and verifies them against its configured receipt keys.

| Interface | Use |
| --- | --- |
| `eth_sendRawTransaction` | Submit a signed EVM transaction to the node. |
| `GET /v1/receipts/{id}` | Retrieve the signed receipt associated with a transaction hash. |
| `pax_sequencerReceipt` | Read the node receipt and its reported evidence rung. |
| `eth_getTransactionReceipt` | Read the EVM execution status, logs, and block reference. |
| `eth_call` | Read or simulate a precompile or contract call without committing it. |
| `eth_getProof` | Retrieve account and storage proof material at a block. |
| `eth_getBlockByNumber` | Retrieve a block for a selected height or tag. |
| `eth_chainId` | Check the EVM chain identifier, `0x7d` for chain 125. |

The receipt route is served by the hosted agent boundary; the JSON-RPC methods are served by the node. The boundary's Bearer planes are separate from daemon sessions and from CLI gateway credentials. Keep transport-specific credentials with their intended boundary.

## Evidence and write readiness

Receipt trust keys are configured independently of incoming proof data. The hosted boundary verifies the receipt signature, its authorized key interval, transaction hash, and signing identity. The Agent API distinguishes execution evidence from committed-block evidence; a receipt is final when its block is committed by the BFT network.

`tenant.readiness` reports `transport_ready`, `verified_reads_ready`, `writes_admitted`, and an optional recovery reason. Writes are admitted exactly when no recovery reason is present, which requires transport and a genuine state-proven account read for the authenticated tenant. Readiness is tenant-scoped; an HTTP connection alone does not establish recovered spending authority.

After a process failure, restore durable holds, verify receipts, and resolve uncertain spends before opening writes. An unresolved reservation remains held. Local limits enforce daemon policy; bypassing the daemon bypasses those limits. Protocol-enforced authority and cryptographic receipt evidence remain separate from local accounting.

## Operate and revoke

Monitor preparation age, open approvals, held budget, unresolved submissions, stream gaps, and verification failures. The hosted boundary's `/livez` reports process liveness; `/readyz` additionally probes node receipt lookup. Loss of the node can leave liveness healthy while readiness fails.

Revoke a capability or close a session to withdraw agent access. Revoking a capability revokes its whole parent subtree and fails its unsubmitted preparations. Preserve audit and receipt records so a revoked agent's historical actions remain traceable.

**Reference:** `services/agent/crates/layerx-agentd/src/enrolment.rs`, `session.rs`; `services/agent/schema/agent-api/identity.kvx`; `tools/platform/cli/src/install/mcp.rs`; `services/platform/hosted/agent-boundary/src/main.rs`.
