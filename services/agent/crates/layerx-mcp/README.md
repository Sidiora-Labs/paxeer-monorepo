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

The `SESSION_TOOLS` definitions add `subscription.create`,
`subscription.list`, `subscription.pause`, `subscription.resume`,
`subscription.delete`, `subscription.health` and `subscription.acknowledge`
under `subscribe`; and `approval.list`, `approval.get`, `approval.approve`
and `approval.reject` under `approve`. The list/health/get tools are reads;
the others are writes. These operate on authenticated daemon-owned state.
Approvals constrain a held request and confer no protocol authority.

`WEB_TOOLS` adds `web.search`, `web.fetch` and `web.content`, with scopes
`write:web:search`, `write:web:fetch` and `write:web:content`. They pay the
configured x-websearch sidecar through the attached payer and approval
boundary. Receipt keys authorize verification of payment evidence; finality
still requires the network's block commit. External results are marked
`layerx/output: untrusted` and must not be treated as instructions.

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

The document is a closed JSON object. This template shows all required fields
and the readiness section required by the served daemon-client path. Replace
every angle-bracket value with an operator-provisioned value; it does not
create a session or capability:

```json
{
  "mode": "full",
  "tenant": "beta",
  "store": "/var/lib/layerx/agentd/store",
  "audit_root": "/var/lib/layerx/agentd/audit",
  "session_id": "<64 hex characters>",
  "session_token_file": "/etc/layerx/mcp-session-token",
  "session_generation": 1,
  "capability_id": "<64 hex characters>",
  "core_height": 120,
  "deadline_ms": 10000,
  "agent": {
    "endpoint": "127.0.0.1:8081",
    "bearer_file": "/etc/layerx/agentd-bearer",
    "probe_program": "<64 hex characters>",
    "readiness": {
      "endpoint": "https://gateway.example.invalid",
      "gateway_key_file": "/etc/layerx/mcp-gateway-key"
    }
  },
  "limit": {
    "id": "<32 hex characters>",
    "name": "mcp",
    "scope": "tenant",
    "scope_id": "<64 hex characters>",
    "ceiling": "1000",
    "consumed": "0"
  }
}
```

`mode` is `full` or `read-only`. `agent.endpoint` names the local IPv4
loopback Programs surface, while `agent.readiness.endpoint` names the HTTPS
gateway base URL used by `AgentEnvelopeTransport`. Optional
`agent.readiness.trust_anchors` names a PEM trust file; absent that field, the
transport uses system roots. This gateway is an agent API transport, distinct
from public node JSON-RPC. Readiness reports `transport_ready`,
`verified_reads_ready`, `writes_admitted` and `recovery_reason`; it is local
readiness information, not verified protocol state.

The binding and secret files must be canonical absolute regular files owned
by the process with owner-only access. The document is bounded to 65,536
bytes. `session_token_file` contains the session's 32-byte token encoded as
64 hex characters. The gateway credential file contains
`key_id:lxp_live_<64 lowercase hex characters>`. Gateway admission and the
tenant session credential are separate authorities; neither file holds a
signing seed. Keep credential contents out of the binding document.

For a socket deployment, add this optional top-level member with the actual
owner and admitted peer UIDs:

```json
{
  "listener": {
    "socket": "/run/layerx/mcp.sock",
    "owner_uid": 0,
    "owner_gid": 0,
    "mode": "660",
    "admitted_uids": [0]
  }
}
```

### Paid web configuration

The owner's full session needs a `web` section and an attached payer and
approval registry to serve web scopes. The daemon host loads payer
configuration through `LAYERX_AGENTD_MCP_PAYERS`; the client transport does not
load signing material. Add the following top-level member, substituting the
actual payment addresses and independently obtained receipt key index:

```json
{
  "web": {
    "endpoint": "http://127.0.0.1:8090",
    "payer_address": "<0x followed by 40 hex characters>",
    "receiver_address": "<0x followed by 40 hex characters>",
    "network": "eip155:125",
    "receipt_key_index": [
      {
        "keyId": "<64 hex characters identifying the receipt key>",
        "publicKey": "<64 hex characters encoding its public key>",
        "firstHeight": 1,
        "lastHeight": 1000
      }
    ],
    "sequencer_id": "<same hex value as keyId above>",
    "first_batch_number": 120,
    "last_batch_number": 120,
    "timeout_ms": 10000,
    "pending_attempts": 3,
    "approval_threshold": "1000"
  }
}
```

`sequencer_id`, `first_batch_number` and `last_batch_number` are retained wire
field names for the selected receipt key and height interval. The key index
must be nonempty, contain unique nonzero key IDs and nonzero public keys, and
cover the selected interval. Each entry has exactly `keyId`, `publicKey`,
`firstHeight` and `lastHeight`. `timeout_ms` is 1 through 60,000 and
`pending_attempts` is 1 through 10. See [`binding.rs`](src/binding.rs) for the
closed field sets and bounds, and [`tools/web.rs`](src/tools/web.rs) for the
payment and receipt checks.
