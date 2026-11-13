# layerx-mcp

Tenant- and scope-bound Model Context Protocol tools for agents on Paxeer
mainnet-beta. Each server uses one daemon-owned session and capability; model
arguments cannot expand its authority. Tenant domains share the same network.
The kernel proposes blocks within that network, and receipts become final
when their blocks are committed.

Both the `layerx-mcp` binary and `layerx mcp serve` use
`Binding::open_daemon_client`. The transport retrieves `mcp.describe` and
invokes tools through the authenticated agent envelope. The owner enforces
policy, capability, preparation, signing and submission checks. The MCP
process reads session and gateway credentials, but no signing seed, and does
not connect directly to node storage or the consensus core.

| Surface | Location |
| --- | --- |
| Agent workspace and network boundary | [Workspace README](../../README.md) |
| Agent owner | [layerx-agentd README](../layerx-agentd/README.md) |
| MCP / A2A interop transports | [services/interop](../../../interop/README.md) |
| CLI install and serve commands | [tools/platform/cli](../../../../tools/platform/cli/README.md) |
| Contract versions and evidence | [SDK compatibility](../../sdk/COMPATIBILITY.md) |

## Tools in this crate

The definitions live in [`server.rs`](src/server.rs) and
[`catalogue.rs`](src/catalogue.rs). The served list comes from the owner's
authenticated `mcp.describe` response, checked against these definitions.
The tables below enumerate all 36 definitions, including session and web tools.
Session scopes, owner availability and deployment mode constrain the list;
a declaration is not a promise that every bound session can call it.
`read-only` mode includes only tools whose kind is `Read`.

| Tool | Kind | Required scope | Operation mapping |
| --- | --- | --- | --- |
| `tenant.readiness` | read | `read` | `TenantReadiness` |
| `balance.get` | read | `read:balance` | `ReadBalance` |
| `wallet.balance` | read | `read:wallet:balance` | `ReadBalance` |
| `wallet.accounts` | read | `read:wallet:accounts` | `ReadAccount` |
| `history.list` | read | `read:history` | `ReadHistory` |
| `receipt.get` | read | `read:receipt` | `ProgramReceipt` |
| `checkpoint.get` | read | `read:checkpoint` | `ReadCheckpoint` |
| `proof.get` | read | `read:proof` | `ReadProofBundle` |
| `availability.get` | read | `read:availability` | `AvailabilityFetch` |
| `activity.prepare` | write | `write:prepare` | `Prepare` |
| `activity.disclose` | write | `write:disclose` | `Prepare` |
| `activity.sign` | write | `write:sign` | `Sign` |
| `activity.submit` | write | `write:submit` | `Submit` |
| `wallet.send` | write | `write:wallet:send` | `Submit` |
| `token.create` | write | `write:token:create` | `Submit` |
| `token.mint` | write | `write:token:mint` | `Submit` |
| `token.transfer` | write | `write:token:transfer` | `Submit` |
| `grant.issue` | write | `write:grant:issue` | `Submit` |
| `grant.draw` | write | `write:grant:draw` | `Submit` |
| `activity.track` | write | `write:track` | `Track` |
| `activity.wait` | write | `write:activity:wait` | `Wait` |
| `faucet.request` | write | `write:faucet:claim` | Dedicated faucet route |

`checkpoint.get` is a retained catalogue name. Committed verification uses
block headers, validator sets and consensus commits, not an additional
checkpoint publication or settlement network. `faucet.request` has no
`tool_operation` mapping; it is handled by the dedicated faucet boundary.

Session tools operate on authenticated daemon-owned state. Approvals constrain a
held request and confer no protocol authority. Subscription acknowledgement takes
a cursor with `block_height`, `tx_index` and `log_index`; approval results retain
`tx_hash` and the observed evidence rung.

| Tool | Kind | Required scope | Operation mapping |
| --- | --- | --- | --- |
| `subscription.create` | write | `subscribe` | `SubscriptionCreate` |
| `subscription.list` | read | `subscribe` | `SubscriptionList` |
| `subscription.pause` | write | `subscribe` | `SubscriptionPause` |
| `subscription.resume` | write | `subscribe` | `SubscriptionResume` |
| `subscription.delete` | write | `subscribe` | `SubscriptionDelete` |
| `subscription.health` | read | `subscribe` | `SubscriptionHealth` |
| `subscription.acknowledge` | write | `subscribe` | `SubscriptionAcknowledge` |
| `approval.list` | read | `approve` | `ApprovalList` |
| `approval.get` | read | `approve` | `ApprovalGet` |
| `approval.approve` | write | `approve` | `ApprovalApprove` |
| `approval.reject` | write | `approve` | `ApprovalReject` |

Web tools pay the configured x-websearch sidecar through the attached payer and
approval boundary. Receipt keys authorize verification of payment evidence;
finality still requires the network's block commit. External results are marked
`layerx/output: untrusted` and must not be treated as instructions.

| Tool | Kind | Required scope | Operation mapping |
| --- | --- | --- | --- |
| `web.search` | write | `write:web:search` | `Submit` |
| `web.fetch` | write | `write:web:fetch` | `Submit` |
| `web.content` | write | `write:web:content` | `Submit` |

Writes preserve exact prepared bytes, disclosure and signatures. Native
wallet/token/grant aliases use the canonical owner path rather than inventing
a tool-specific mutation. Results retain `tx_hash` and the evidence rung
(`submitted`, `receipted`, `committed`) or an honest non-terminal state.
`InvocationOutcome` records `Completed`, `Refused`, `Unknown` or `Failed`;
completion of a tool invocation alone does not prove committed finality.
Argument and scope boundaries are in [`validate.rs`](src/validate.rs),
[`untrusted.rs`](src/untrusted.rs) and the catalogue's strict schemas.

## Test

From the repository root, focused targets include:

```sh
make agent-test-mcp-scope
make agent-test-mcp-read
make agent-test-mcp-write
make agent-test-mcp-approval
make agent-test-mcp-injection
make agent-test-mcp-readonly
```

These wrap Cargo with the runtime clock helper. Cargo discovers the crate's
`tests/` targets; `injection` is explicitly registered from
`services/agent/tests/mcp/injection.rs`. The full `make agent-test` also needs
the native and TLS prerequisites in the [workspace guide](../../README.md#tests).

## Serving

Use an absolute path to an operator-protected binding document:

```sh
layerx-mcp /etc/layerx/mcp-binding.json
layerx mcp serve --daemon-binding /etc/layerx/mcp-binding.json
```

The binary serves a peer-credential-admitted Unix socket when `listener` is
present, and standard input/output otherwise. The CLI serves standard
input/output and may further narrow tools with `--read-only`. Both paths
authenticate the session and retrieve the live catalogue before serving it.

The binding is a closed JSON object. Its required top-level fields are:

| Field | Value |
| --- | --- |
| `mode` | `full` or `read-only` |
| `tenant` | Session tenant name |
| `store`, `audit_root` | Absolute paths to the owner's store and audit directory |
| `session_id`, `capability_id` | Session and capability identifiers, each 32 bytes encoded as 64 hex characters |
| `session_token_file` | Absolute path to the protected session token file |
| `session_generation` | Unsigned session generation |
| `core_height` | Unsigned height used by local owner operations |
| `deadline_ms` | Positive unsigned deadline in milliseconds |
| `agent` | Programs and gateway connection fields described below |
| `limit` | Local spending limit fields described below |

The required `agent` fields are `endpoint`, `bearer_file` and `probe_program`.
They identify the local IPv4 loopback Programs surface, its protected bearer
file and its probe program for local owner operations. The daemon-client
transport uses `agent.readiness`, with required `endpoint` and
`gateway_key_file` fields. `endpoint` is the HTTPS gateway base URL used by
`AgentEnvelopeTransport`. Optional `trust_anchors` names an absolute PEM trust
file; absent that field, the transport uses system roots. `Binding::parse`
accepts an absent readiness section for local owner use, but
`Binding::open_daemon_client` requires it.

The required `limit` fields are `id` (16 hex-encoded bytes), `name`, `scope`,
`scope_id` (32 hex-encoded bytes), `ceiling` and `consumed` (decimal strings
representing unsigned 128-bit amounts). The scope is `tenant`, `agent`,
`session`, `capability` or `counterparty`.

Both gateway API-key admission and the tenant session credential are required
for the `POST /v1/agent/rpc` envelope. Readiness reports `transport_ready`,
`verified_reads_ready`, `writes_admitted` and `recovery_reason` as local
readiness information.

The binding and secret files must be canonical absolute regular files owned
by the process with owner-only access. The document is bounded to 65,536
bytes. `session_token_file` contains the session's 32-byte token encoded as
64 hex characters. The gateway credential file contains a key identifier,
a colon, the prefix `lxp_live_` and 64 lowercase hex characters. Credential
contents stay in their protected files.

For a socket deployment, the optional top-level `listener` object has exactly
these fields:

| Field | Value |
| --- | --- |
| `socket` | Absolute normalized Unix socket path inside a dedicated directory |
| `owner_uid`, `owner_gid` | Unsigned 32-bit owner identifiers |
| `mode` | Octal string granting owner read/write and optional group read/write, bounded by `600` and `660` |
| `admitted_uids` | Array of 1 through 64 distinct unsigned 32-bit peer UIDs |

The canonical parent directory has the declared ownership and permits no
group-write or world access.

### Paid web configuration

The owner's full session needs a `web` section and an attached payer and
approval registry to serve web scopes. The daemon host loads payer
configuration through `LAYERX_AGENTD_MCP_PAYERS`. The `web` object requires:

| Field | Value |
| --- | --- |
| `endpoint` | Configured x-websearch sidecar endpoint |
| `payer_address`, `receiver_address` | EVM payment addresses encoded as `0x` plus 40 hex characters |
| `network` | EIP-155 network identifier, `eip155:125` for mainnet-beta |
| `receipt_key_index` | Independently obtained receipt key authorizations |
| `sequencer_id` | Selected nonzero receipt key identifier, encoded as 64 hex characters |
| `first_batch_number`, `last_batch_number` | Selected unsigned receipt height interval, with first no greater than last |
| `timeout_ms` | Unsigned timeout from 1 through 60,000 milliseconds |
| `pending_attempts` | Unsigned attempt count from 1 through 10 |
| `approval_threshold` | Decimal string representing an unsigned 128-bit amount |

The key index contains 1 through 64 entries with unique nonzero key IDs and
nonzero public keys. Each entry has exactly `keyId`, `publicKey`, `firstHeight`
and `lastHeight`: the first two encode 32 bytes as hex, and the last two are
unsigned heights with first no greater than last. One authorization for the
selected key must cover the entire selected interval. See
[`binding.rs`](src/binding.rs) for field sets and bounds, and
[`tools/web.rs`](src/tools/web.rs) for payment and receipt checks.
