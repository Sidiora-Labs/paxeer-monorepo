# Running an agent

An agent uses a tenant-scoped session and capability held by `layerx-agentd`.
The operator provisions that session and its protected binding document, then
registers a daemon-bound MCP client with an agent runtime. The daemon owns the
signing, policy, budget, approval, and receipt-resolution paths. Installing a
client does not create protocol authority.

Paxeer mainnet-beta is one BFT network, with EVM chain ID `125` (`0x7d`).
The kernel is part of the network and proposes blocks. Execution and finality
belong to that network: a receipt is final once its block is committed.
Sub-chains are tenant domains in the database, not additional infrastructure
or new public chain endpoints.

This guide follows the current implementations in
[agentd](../../services/agent/crates/layerx-agentd/src/main.rs),
[the MCP binding](../../services/agent/crates/layerx-mcp/src/binding.rs), and
[the CLI MCP client](../../tools/platform/cli/src/mcp.rs). See also
[Agentd](Agentd.md), [MCP](Mcp.md), [CLI](Cli.md), and the
[payments developer path](PaymentsQuickstart.md).

## Point the CLI at the hosted profile

First distinguish the transports involved:

| Transport | Endpoint and purpose |
| --- | --- |
| Public node JSON-RPC | Root HTTPS URL on `api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network`, or `api-mainnet-beta.paxeer.network`; EVM reads and signed transaction submission on chain `125` |
| Hosted gateway | Operator-provided HTTPS origin; hosted methods and authenticated agent envelopes, as declared in [the gateway contract](../../services/platform/hosted/gateway/openrpc.json) |
| MCP | Local stdio, or the protected Unix listener declared by the binding; the client forwards tool calls to the authenticated daemon transport |
| Agent program reads | The binding's loopback `agent.endpoint` and protected bearer; this is not the public node URL |

The node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `test`, and
`txpool`. A hosted `px_` method is not automatically available on a public
node. Obtain the hosted origin and its credentials from the deployment
operator; a node hostname alone does not establish an agent gateway.

The unified router and its `/rpc`, `/rpc/ws`, and `/rpc/schema` public routes are retired.
`node.hyperpaxeer.com` is only the hpx library mirror, never an RPC endpoint.
There is no separate LayerX interface or separate sequencer service to deploy.

For a node-connected agent process, the node configuration is:

```bash
export LAYERX_AGENT_NODE_RPC_URL=https://api1.mainnet-beta.paxeer.network
export LAYERX_AGENT_NODE_CHAIN_ID=125
export LAYERX_AGENT_HUMAN_NETWORK_ID=125
```

These three values are only the node identity portion of startup, not a
complete agent environment. `NodeRpc::connect` checks `eth_chainId` against
the configured chain ID. `HumanNodeEnvironment::from_environment` also
requires the Human network ID to match. Use HTTPS outside loopback. The
[node transport](../../services/agent/crates/layerx-agentd/src/node_rpc.rs)
loads TLS roots and applies a finite deadline; a chain mismatch is a refusal.
See [public RPC endpoints](../site/docs/reference/public-rpc.md) for the node
list and root-path examples.

MCP installation does not select or read a CLI environment profile. For
other hosted CLI commands, an already configured profile can be selected:

```text
layerx environment use beta
```

Profile names are `emulator`, `beta`, and `production`. The current
[CLI parser](../../tools/platform/cli/src/main.rs) still uses the legacy
flag names `--sequencer-trust-anchor` and `--sequencer-trust-anchor-file`
when initially configuring a profile. Its
[input resolver](../../tools/platform/cli/src/emulator.rs) requires an
endpoint, network ID, and one of those pin inputs together, or none of them.
That compatibility setting is not a request to provision another network
service and does not replace committed-block verification. `environment.selected`
reports `name`, `endpoint`, `network_id`, and `sequencer_trust_anchor`;
`--json` uses the CLI envelope `{ok, kind, message, data}`.

## Tenant credential and scope set

### Hosted gateway key (CLI command)

Hosted A2A installation and daemon-bound MCP installation have different
credential provisioning steps. The hosted CLI can create an Ed25519 identity
key and store an identity session:

```text
layerx key create <name>
layerx auth set --environment beta
```

`auth set` reads the token from stdin. `key create` obtains a 32-byte seed
from operating-system randomness and stores it under keyring service
`dev.layerx.cli`; its output contains public metadata. See
[credential storage](../../tools/platform/cli/src/credential.rs).

`layerx install a2a` provisions a hosted key through `/v1/keys` or
`/v1/keys/{id}/rotate`. Its payment scopes are `activity:write` and
`receipt:read`; read-only installation uses `receipt:read`. The secret stays
in credential storage. This key is distinct from a daemon session token.
See [A2A installation](../../tools/platform/cli/src/install/a2a.rs) and
[shared installation code](../../tools/platform/cli/src/install/mod.rs).

`layerx install mcp` does not provision a gateway key, identity, or session.
It reads the operator-provided binding and authenticates to the daemon
before publishing the host registration. The generated host entry has
`env: {}`. The launched process nevertheless reads the session token and
gateway credential through protected file references in that binding;
claims that an MCP client never holds a gateway credential are incorrect.

### Daemon enrolment and the binding document

There is no `layerx` subcommand that starts the agent host or performs its
enrolment. The `layerx-agentd` binary and library are off-chain agent
software. The library entry point is `enrolment::enrol`; the binary's
`LAYERX_AGENT_MCP_BINDING_ROOT` enables boot enrolment in full agent mode.
Human-owner-only mode refuses that setting because it does not serve the
program endpoint the document names.

Boot enrolment resolves the configured agent DID through the Human authority,
registers it for the configured peer's tenant and principal, restores its
capability, and opens a session. The request binds the agent, authority,
permitted activity types, scopes, and expiry. A random session token is
published with the binding. If publication fails, enrolment attempts to
close the new session; `OrphanedSession` reports failure of that cleanup.
On restart, an existing session must agree with its published binding;
`BindingPublisher::validate_existing` refuses inconsistent state.

[The publisher](../../services/agent/crates/layerx-agentd/src/enrolment.rs)
creates a protected binding directory (`0700`) containing:

| File | Contents |
| --- | --- |
| `binding.json` | The declaration parsed by `Binding::open` |
| `session-token` | The hex-encoded minted session token |
| `daemon-bearer` | The loopback daemon transport bearer |

Secret files are created with mode `0600` and `create_new`; existing secret
files are not overwritten. Failed partial publication removes files created
by that attempt. Repeating an identical published document reports
`created: false`; a different document at its path is refused.

The current document has these required top-level fields:

```text
mode, tenant, store, audit_root, session_id, session_token_file,
session_generation, capability_id, core_height, deadline_ms, agent, limit
```

`agent` contains `endpoint`, `bearer_file`, `probe_program`, and optionally
`readiness`. `limit` contains `id`, `name`, `scope`, `scope_id`, `ceiling`,
and `consumed`; the last two are decimal strings. Optional top-level
`listener` and `web` declarations have closed schemas too. Unknown fields
are refused. The old `core_sequence` document field is retired; publish
`core_height` instead. See the shared
[binding shape](../../tools/platform/cli/src/install/binding_shape.rs).

A document usable by the CLI or standalone MCP binary must also declare
`agent.readiness`:

| Field | Meaning |
| --- | --- |
| `endpoint` | Authenticated gateway HTTPS origin |
| `gateway_key_file` | Absolute protected file containing `key_id:lxp_live_<64 lowercase hexadecimal characters>` |
| `trust_anchors` | Optional absolute TLS trust-anchor file |

The session credential supplies tenant, session ID, token, and generation.
`Binding::open_daemon_client` uses these and the gateway key to open an
`AgentEnvelopeTransport`, then requests the daemon's live catalogue with
`mcp.describe`. A gateway credential does not grant a session broader scopes.

Local boot publication alone is insufficient for this client path: the
binary's `mcp_boot` currently constructs `DaemonSurface::new` without
attaching a readiness declaration. An embedding deployment must supply
`DaemonSurface::with_readiness(ReadinessDeclaration::new(...))` when
constructing its publisher and provision the matching authenticated daemon
route. A local-only document is refused by `open_daemon_client` with
`agent.readiness must declare the authenticated unified gateway`. Treat that
as an incomplete deployment, not a reason to bypass authentication or insert
credentials in runtime host JSON.

## Start the MCP server and connect a runtime

Once the operator has supplied a usable binding and the daemon admits its
session, register it:

```text
layerx install mcp --host <runtime> --daemon-binding <absolute-path-to-binding.json>
```

`--host` is repeatable. Supported values and registration locations are:

| Runtime | Host document | Section |
| --- | --- | --- |
| `layerx` | `mcp.json` beside the CLI configuration | `mcpServers` |
| `claude-code` | `~/.claude.json` | `mcpServers` |
| `claude-desktop` | `$XDG_CONFIG_HOME/Claude/claude_desktop_config.json`; macOS uses `~/Library/Application Support/Claude/claude_desktop_config.json` | `mcpServers` |
| `cursor` | `~/.cursor/mcp.json` | `mcpServers` |
| `vscode` | `$XDG_CONFIG_HOME/Code/User/mcp.json`; macOS uses `~/Library/Application Support/Code/User/mcp.json` | `servers` |

Without an explicit host the installer includes `layerx` and detects other
hosts from their marker directories or configuration paths. An unknown host
is refused before binding access. `LAYERX_INSTALL_ROOT` can replace the home
root used by installation. Without `--daemon-binding`, the default is
`mcp/binding.json` beside CLI configuration; explicit relative paths are
resolved against the current directory.

The [installer](../../tools/platform/cli/src/install/mcp.rs) validates the
protected binding, opens an authenticated daemon session, and obtains
`tools/list`. A full installation must include the admitted prepare,
disclose, sign, submit, and track payment journey. Missing daemon admission
or an invalid catalogue prevents registration. `--read-only` narrows the
client's served tools; it cannot widen the document's declared mode.

The installed launch line is:

```text
layerx mcp serve --daemon-binding <absolute-path-to-binding.json> [--read-only]
```

The host entry contains `command`, `args`, and an empty `env`; VS Code also
gets `type: "stdio"`. Installation reports kind `install.mcp`, with
`component: "mcp"`, `transport: "stdio"`, `authorization: "agent-daemon"`,
`deployment_mode`, `daemon_binding`, `server`, `tools`, `scopes`,
`registrations`, `changed`, and `idempotent: true` in `data`. Binding metadata
includes `core_height` and `readiness`. Descriptors contain each tool's name,
kind, required scope, mutation, evidence, description, and argument schema.
Registration reports identify the host path, section, name, action, change
status, and owner-only permissions.

The crate also ships a
[standalone binary](../../services/agent/crates/layerx-mcp/src/bin/layerx-mcp.rs):

```text
layerx-mcp <absolute-path-to-binding.json>
```

It opens the same authenticated daemon client. Without a `listener`
declaration it serves stdio; with one it binds the declared Unix socket and
admits only configured peers. A live socket is not replaced. The CLI
`layerx mcp serve` always serves stdio. In-process hosts can use
`Server::bind` / `ReadOnly::bind` and attach the required daemon boundaries;
those library APIs do not make the local binding an authenticated remote
client automatically.

## Grant a budget

There is no `layerx budget` command. Choose the enforcement boundary before
assigning a limit:

| Mechanism | Enforcement |
| --- | --- |
| `LocalLimit::new` / `BudgetLimiter` | Daemon only; bypassing `layerx-agentd` bypasses the limit |
| `create_protocol_budget` | Verified layerxpay budget object created through a signed EIP-1559 transaction |

Local `LimitConfig` scopes are tenant, agent, session, capability, and
counterparty. The binary installs a Human limit from
`LAYERX_AGENT_HUMAN_LIMIT_*`; MCP binding limits use
`LAYERX_AGENT_MCP_LIMIT_*`. Local accounting figures never manufacture
protocol authority.

[Protocol creation](../../services/agent/crates/layerx-agentd/src/budget/create.rs)
binds the request to the exact verified signed bytes and idempotency key,
checks the layerxpay `budgetCreate` or `budgetCreateNative` calldata, submits
through `BudgetPipeline`, and verifies a successful committed receipt and
the matching budget state at the receipt's height. The budget ID, account,
asset, period limits, expiry, rollover fields, revocation sequence, purpose,
and initial spend state must agree; a closed or revoked object is refused.

On success, `create_protocol_budget` returns `Ok(ProtocolBudget)` with the
object ID, receipt bytes, verified record, observed height, and
`protocol-enforced` label. Missing or mismatched state returns
`CreatedBudgetUnconfirmed`; invalid receipt evidence is not success. The
function's `_store` argument is unused: a successful return alone does not
persist the returned object in the caller's local store. Callers must retain
and recover their accounting state through the owning persistence path.

The Human `AGENT_LIMIT` operation and SDK budget examples are other API
surfaces, not CLI subcommands. Use the actual producer's request schema when
integrating them; a local limit and a protocol budget are different objects.

## Read state

Use `tools/list` from the live session to discover available reads. Protocol
read entries include `balance.get`, `wallet.balance`, `wallet.accounts`,
`history.list`, `receipt.get`, `checkpoint.get`, `proof.get`, and
`availability.get`. `tenant.readiness` reports daemon-local readiness; it is
not verified protocol state or a guarantee that tenant writes are enabled.

[Read helpers](../../services/agent/crates/layerx-mcp/src/tools/read.rs)
return `VerifiedToolResult` with `value`, optional `rung`, `freshness`, and
`page`. Receipt values include canonical bytes, evidence identifiers, and
height. They require a receipted or committed rung and non-empty evidence;
state freshness must agree with the value's height. Pagination is bounded
and carries an explicit continuation when incomplete. A truncated page is
not a complete history.

`layerx receipt get <id>` is a separate CLI fetch, using its configured
transport rather than the MCP session. Likewise, SDK receipt verification
functions are library calls. Keep the source of evidence and the receipt
key policy explicit for each surface.

## Submit one signed activity

MCP exposes the daemon's ordinary write journey: Prepare, Disclose, Policy,
Sign, Submit, Track. The catalogue distinguishes `activity.prepare`,
`activity.disclose`, `activity.sign`, `activity.submit`, `activity.track`,
and `activity.wait`. Wallet, token, and grant writes use the same daemon
authority boundary. Use the argument schema returned by the current
catalogue; the gateway-bound A2A `raw_transaction` shape is not the daemon
preparation schema.

Every MCP write needs a stable idempotency key. The
[stdio daemon client](../../services/agent/crates/layerx-mcp/src/stdio.rs)
accepts it from the tool's declared arguments or
`params._meta["layerx/idempotency_key"]`: 64 lowercase hex characters
encoding a non-zero 32-byte key. Conflicting argument and metadata keys are
refused. Keep the same key and original transaction hash when resolving an
uncertain outcome.

For a signed EVM transaction sent directly to a public node, submit at the
root URL, after confirming chain ID `125`:

```bash
curl --max-time 30 -sS https://api1.mainnet-beta.paxeer.network/ \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_sendRawTransaction","params":["<signed-EIP-1559-transaction-hex>"]}'
```

The placeholder must be replaced with real signed bytes. A returned hash is
submission evidence, not proof of execution or finality. Resolve the
original hash with `eth_getTransactionReceipt` and the appropriate committed
receipt evidence. A timeout does not prove that the transaction was unsent;
do not create a new payment to recover from an unknown response.

### Hosted A2A adapter

`layerx a2a serve` is a loopback agent-to-agent adapter with its own local
authorization file and hosted gateway credential. Its base toolset is
`receipt.get` (`receipt:read`), `activity.submit` (`activity:write`), and
`faucet.request` (`write:faucet:claim`); read-only mode keeps only the read.
See [A2A transport](../../tools/platform/cli/src/a2a.rs) and
[toolset](../../tools/platform/cli/src/toolset.rs).

The A2A submit adapter accepts only `raw_transaction`, validates the signed
EIP-1559 transaction and chain ID, and checks the returned hash against the
signed bytes. Its result is `{txHash, rung: "submitted"}`. The adapter still contains a pre-merge `/rpc` forwarding path, which is retired as a public node route.
Do not assume that selecting a public node origin makes this hosted adapter
usable; direct public submission uses the root-path example above. Hosted
adapter deployment must match its actual gateway contract.

## Approve

An approval hold is a daemon policy decision about a specific disclosure;
it does not grant new protocol authority. The current catalogue includes
`approval.list`, `approval.get`, `approval.approve`, and `approval.reject`
under scope `approve`. Their availability is determined by the bound
session, and their records are daemon-owned facts.

Paid web calls pass through `WebBoundary` and `WebApproval`. A spend over
the configured threshold can create a hold before the payer submits. The
server returns a held outcome identifying the hold and what it is awaiting.
The approval registry binds the decision to the request and disclosure,
checks expiry, and supports restoring durable holds and reservations.
Approval does not permit substituting different payment terms. See
[the web boundary](../../services/agent/crates/layerx-mcp/src/server.rs),
[web tools](../../services/agent/crates/layerx-mcp/src/tools/web.rs), and
[approval persistence](../../services/agent/crates/layerx-agentd/src/policy/approval.rs).

The library `layerx_mcp::approval::require` also supports disclosure holds
for an embedding host. Do not infer from that helper that every ordinary
write automatically waits for a human: the configured policy and served
boundary determine whether a hold is required. There is no `layerx approval`
CLI command. Human approval operations are another daemon API surface.

## Observe the receipt

The migrated result vocabulary is `rung`:

| Rung | What it establishes |
| --- | --- |
| `submitted` | Transaction submission; no verified execution receipt yet |
| `receipted` | Receipt verified against a pinned receipt-key entry; committed-block finality has not yet been established by this result |
| `committed` | Verified Comet commit at the receipt height; final on the BFT network |

These meanings are shared in [the rung type](../../services/agent/crates/layerx-types/src/rung.rs).

[Typed write outcomes](../../services/agent/crates/layerx-mcp/src/tools/write.rs)
are `Executed`, `Pending`, or `Unknown`, each carrying a rung and submission
reference. `Executed` additionally requires a verified receipt with canonical
bytes and evidence IDs. `SuccessWithoutVerifiedReceipt` refuses a transcript
that claims execution without that evidence. Pending, held, and unknown
outcomes must not be presented as a completed payment.

A committed transaction can still have a failure result code. Check execution
status as well as the evidence rung. Finality does not convert a rejected
operation into a successful transfer.

For offline canonical receipt verification the current CLI still exposes
`layerx receipt verify` with `--receipt`, `--batch-id`, `--asset`,
`--previous-state-root`, `--resulting-state-root`, and
`--sequencer-public-key`. These are compatibility names in
[the verifier](../../tools/platform/cli/src/receipt.rs): the caller must
supply independently trusted facts. It emits `verified: true` only after
`verify_outcome` succeeds and still calls its evidence rank
`verification_level`. That legacy field is not the migrated `rung`, and
signature verification alone does not prove BFT commitment.

## Observe revocation

Closing a session persists `open: false` and advances its generation. An
old session token and generation must not authorize another write. Restore
session and capability records from the daemon store after restart rather
than opening replacement authority to resume unresolved work.

[Revocation processing](../../services/agent/crates/layerx-agentd/src/session_revocation.rs)
invalidates matching open sessions and reports invalidated sessions and
generations, cancelled preparations, unresolved work, and executed work left
untouched. Prepared and signed work is cancelled; queued, unknown, and
nonce-consumed work continues receipt resolution. On on-chain grant
revocation, queued/unknown work is also marked cancelled while resolution
continues. Executed and failed work remains unchanged. Revoking a session
cannot undo a transaction already committed by the network.

Hosted gateway key revocation and daemon session revocation are separate
operations. A gateway key response with `state: "revoked"` is not itself a
session invalidation report. There is no CLI subcommand for the daemon's
session close operation.

Local event delivery uses protected Unix frames `LXOW` / `LXOA`. Consumers
must deduplicate using `deduplication_id`; the transport is not an HTTP
webhook registration. See [event delivery](../../services/agent/crates/layerx-agentd/src/events/deliver.rs).

## MCP tools

The current base definitions are in
[server.rs](../../services/agent/crates/layerx-mcp/src/server.rs). The actual
served set is filtered by the live daemon's session, capability, scopes,
mode, and attached boundaries; use `tools/list` rather than a fixed count.

| Tool | Required scope |
| --- | --- |
| `tenant.readiness` | `read` |
| `balance.get` | `read:balance` |
| `history.list` | `read:history` |
| `receipt.get` | `read:receipt` |
| `checkpoint.get` | `read:checkpoint` |
| `proof.get` | `read:proof` |
| `availability.get` | `read:availability` |
| `wallet.accounts` | `read:wallet:accounts` |
| `wallet.balance` | `read:wallet:balance` |
| `activity.prepare` | `write:prepare` |
| `activity.disclose` | `write:disclose` |
| `activity.sign` | `write:sign` |
| `activity.submit` | `write:submit` |
| `activity.track` | `write:track` |
| `wallet.send` | `write:wallet:send` |
| `token.create` | `write:token:create` |
| `token.mint` | `write:token:mint` |
| `token.transfer` | `write:token:transfer` |
| `grant.issue` | `write:grant:issue` |
| `grant.draw` | `write:grant:draw` |
| `activity.wait` | `write:activity:wait` |
| `faucet.request` | `write:faucet:claim` |

The [catalogue](../../services/agent/crates/layerx-mcp/src/catalogue.rs) also
defines `subscription.create`, `subscription.list`, `subscription.pause`,
`subscription.resume`, `subscription.delete`, `subscription.health`, and
`subscription.acknowledge` under `subscribe`, and the four `approval.*` tools
under `approve`. Paid `web.search`, `web.fetch`, and `web.content` require
`write:web:search`, `write:web:fetch`, and `write:web:content`, respectively,
plus an attached payer and sidecar. All three are writes because they may
spend funds; their external content is marked untrusted output.

Read-only mode filters out write tools, including paid web calls. Even a
read entry remains subject to the session's scope and authority checks.
The `faucet.request` definition is an adapter for `px_requestFunds`; it does
not restore the retired `faucet.claim` agent operation. Older clients can
receive `unavailable_capability`, and a catalogue definition does not promise
that a public mainnet node provides hosted funding. See
[the funding adapter](../../services/agent/crates/layerx-mcp/src/tools/faucet.rs).

## Agentd configuration keys

### Library startup configuration

`config::load` reads a protected UTF-8 `key=value` file; exact environment
keys override file values. Unknown settings, duplicates, blank overrides,
and incomplete tenant maps are refused. The required settings in
[config.rs](../../services/agent/crates/layerx-agentd/src/config.rs) are:

| File key | Environment key | Meaning |
| --- | --- | --- |
| `network_id` | `LAYERX_NETWORK_ID` | Non-zero network ID; mainnet-beta is `125` |
| `node_endpoint` | `LAYERX_NODE_ENDPOINT` | HTTP(S) JSON-RPC URL, not a local socket path; use the public node root for EVM calls |
| `expected_chain_id` | `LAYERX_EXPECTED_CHAIN_ID` | Expected non-zero chain ID; `125` for mainnet-beta |
| `tenants` | `LAYERX_TENANTS` | Non-empty, unique tenant IDs |
| `policy_sources` | `LAYERX_POLICY_SOURCES` | Exactly one absolute policy path per tenant |
| `signer_configurations` | `LAYERX_SIGNER_CONFIGURATIONS` | Exactly one absolute signer configuration path per tenant |
| `verification_defaults` | `LAYERX_VERIFICATION_DEFAULTS` | Per-tenant compatibility verifier policy, not a network finality ladder |
| `receipt_key_source` | `LAYERX_RECEIPT_KEY_SOURCE` | Protected receipt-key policy file, or the parser token `px_getReceiptKeys` |

Maps use comma-separated `tenant:value` entries. The parser still accepts
legacy verifier labels such as `state-proven`; these are compatibility
inputs, not successive execution, checkpoint, and settlement services.
`expected_protocol_version` and `sequencer_authority_source`, including
their corresponding environment names, are retired and explicitly refused.

There is an important library boundary: `Gate::new` loads the protected
receipt-key policy through `ProtocolEvidenceVerifier::load`. Although the
configuration parser accepts `receipt_key_source=px_getReceiptKeys`, this
loader currently returns `AuthoritySourceUnavailable` for that variant;
it does not bootstrap trusted policy from an untrusted RPC response.
Use an operator-provisioned pinned policy for this library gate. The
[policy loader](../../services/agent/crates/layerx-agentd/src/protocol_evidence.rs)
still parses the versioned compatibility file format; consult that parser
when provisioning it rather than inventing a new key-file schema.

[Readiness](../../services/agent/crates/layerx-agentd/src/boot.rs) begins
`NotReady`. `NodeRpc::connect` validates chain identity, and
`Gate::readiness_gate` fetches current receipt keys and checks their validity
intervals against the protected policy before opening writes. Empty keys,
a key-policy mismatch, or a disconnected node leave the gate closed.
A reconnect requires another readiness check; `guard_write` refuses
`NotReady` before invoking its write closure.

### Binary environment and MCP enrolment

The binary uses its own environment-driven startup rather than loading the
library file above. The following groups identify the settings relevant to
this path; the complete conditional startup logic is in
[main.rs](../../services/agent/crates/layerx-agentd/src/main.rs) and
[node_rpc.rs](../../services/agent/crates/layerx-agentd/src/node_rpc.rs).
These groups are not a minimal environment sufficient to boot a deployment.

| Keys | Purpose |
| --- | --- |
| `LAYERX_AGENT_NODE_RPC_URL`, `LAYERX_AGENT_NODE_CHAIN_ID` | Human node JSON-RPC connection and expected chain ID; decimal or canonical hex chain quantity |
| `LAYERX_AGENT_HUMAN_NETWORK_ID`, `LAYERX_AGENT_HUMAN_PROTOCOL_VERSION` | Matching network ID and current state-commitment wire version |
| `LAYERX_AGENT_NODE_ENDPOINT`, `LAYERX_EXPECTED_CHAIN_ID` | Program/native node connection and expected decimal chain ID |
| `LAYERX_AGENT_PROGRAM_LISTEN`, `LAYERX_AGENT_PROGRAM_BEARER_TOKEN` | Loopback `127.0.0.1:<port>` program listener and bearer of at least 32 bytes |
| `LAYERX_AGENT_NODE_BEARER_TOKEN`, `LAYERX_AGENT_AUTHORITY_BEARER_TOKEN` | Upstream bearers, each at least 32 bytes; program bearer must differ from both |
| `LAYERX_AGENT_AUTHORITY_ENDPOINT`, `LAYERX_AGENT_AUTHORITY_CA_DER`, `LAYERX_AGENT_AUTHORITY_REPLICA_ID` | Authority boundary, TLS CA file, and 32-byte replica identity |
| `LAYERX_AGENT_CHAIN_TRUST_ROOT`, `LAYERX_AGENT_DEPLOYMENT_JOURNAL`, `LAYERX_AGENT_PROGRAM_PROBE_ID` | Protected chain trust root, verified deployment journal, and probe program |
| `LAYERX_AGENT_PROGRAM_MAX_STALENESS_MS` | Non-zero program read staleness bound |
| `LAYERX_POLICY_SOURCES`, `LAYERX_NATIVE_POLICY_SOURCES` | Policy maps covering the configured Human tenants |
| `LAYERX_AGENT_HUMAN_STORE`, `LAYERX_AGENT_HUMAN_SOCKET`, `LAYERX_AGENT_HUMAN_SESSION_KEY_ROOT`, `LAYERX_AGENT_HUMAN_SESSION_OPERATOR_SECRET_FILE` | Absolute local store, owner socket, session-key directory, and protected operator secret paths |
| `LAYERX_AGENT_HUMAN_PEERS` | Comma-separated `uid=<u32>;tenant=<tenant>;principal=<did>` entries; duplicate UIDs are refused |
| `LAYERX_AGENT_HUMAN_LIMIT_ID`, `LAYERX_AGENT_HUMAN_LIMIT_NAME`, `LAYERX_AGENT_HUMAN_LIMIT_SCOPE`, `LAYERX_AGENT_HUMAN_LIMIT_SCOPE_ID`, `LAYERX_AGENT_HUMAN_LIMIT_CEILING`, `LAYERX_AGENT_HUMAN_LIMIT_CONSUMED` | Local Human limit; ID is 16 bytes and scope identity is 32 bytes, both hex encoded |

When `LAYERX_AGENT_MCP_BINDING_ROOT` is set, enrolment also requires:

| Keys | Purpose |
| --- | --- |
| `LAYERX_AGENT_MCP_BINDING_ROOT`, `LAYERX_AGENT_MCP_AUDIT_ROOT` | Absolute binding and audit directories |
| `LAYERX_AGENT_MCP_PEER_UID`, `LAYERX_AGENT_MCP_AGENT_DID` | Configured Human peer and authority-verified agent DID |
| `LAYERX_AGENT_MCP_SESSION_ID`, `LAYERX_AGENT_MCP_CAPABILITY_ID` | 32-byte hexadecimal session and capability IDs |
| `LAYERX_AGENT_MCP_ACTIVITY_TYPES`, `LAYERX_AGENT_MCP_SCOPES` | Permitted activity types and scopes; duplicate entries are refused |
| `LAYERX_AGENT_MCP_CORE_HEIGHT`, `LAYERX_AGENT_MCP_EXPIRY_HEIGHT` | Current and expiry block heights; the binding field is `core_height` |
| `LAYERX_AGENT_MCP_OPENING_CLIENT`, `LAYERX_AGENT_MCP_POLICY_VERSION` | Session audit context |
| `LAYERX_AGENT_MCP_MODE`, `LAYERX_AGENT_MCP_DEADLINE_MS` | `full` or `read-only` mode and transport deadline |
| `LAYERX_AGENT_MCP_LIMIT_ID`, `LAYERX_AGENT_MCP_LIMIT_NAME`, `LAYERX_AGENT_MCP_LIMIT_SCOPE`, `LAYERX_AGENT_MCP_LIMIT_SCOPE_ID`, `LAYERX_AGENT_MCP_LIMIT_CEILING`, `LAYERX_AGENT_MCP_LIMIT_CONSUMED` | Binding limit with the same scope vocabulary as the Human limit |
| `LAYERX_AGENT_MCP_LISTENER_SOCKET`, `LAYERX_AGENT_MCP_LISTENER_OWNER_UID`, `LAYERX_AGENT_MCP_LISTENER_OWNER_GID`, `LAYERX_AGENT_MCP_LISTENER_MODE`, `LAYERX_AGENT_MCP_LISTENER_ADMITTED_UIDS` | Protected listener declaration; mode is octal and admitted UIDs must include the enrolment peer |

A configured web sidecar additionally uses `LAYERX_AGENT_MCP_WEB_ENDPOINT`,
`LAYERX_AGENT_MCP_WEB_NETWORK`, `LAYERX_AGENT_MCP_WEB_RECEIPT_KEY_INDEX`,
`LAYERX_AGENT_MCP_WEB_TIMEOUT_MS`, `LAYERX_AGENT_MCP_WEB_PENDING_ATTEMPTS`,
and `LAYERX_AGENT_MCP_WEB_APPROVAL_THRESHOLD`. The index selects a receipt
key from `px_getReceiptKeys` after checking the node chain ID. It does not
imply that every public node exposes that hosted method.

`LAYERX_AGENT_MCP_WEB_SEQUENCER_PUBLIC_KEY`,
`LAYERX_AGENT_MCP_CORE_SEQUENCE`, and `LAYERX_AGENT_MCP_EXPIRY_SEQUENCE`
are retired and refused even when set to empty values. Use receipt-key
selection and the two height settings instead.

## Typed refusals the agent sees

The installed client speaks one JSON-RPC message per stdio line.
`initialize` reports protocol version `2025-06-18`, daemon binding metadata,
and the effective tool counts. `tools/list` supplies descriptions, argument
schemas, scope and evidence metadata; it is refreshed from the daemon.
Treat the returned schema as authoritative for that session.

| Refusal | Meaning |
| --- | --- |
| `the daemon binding document at {path} could not be used: ...` | Installation could not read or validate the protected declaration |
| `mcp.binding_retired_key` | A binding still contains the retired `core_sequence` field |
| `the authenticated agent daemon did not admit this installation binding: ...` | The document parsed but the live authenticated session was refused |
| `agent.readiness must declare the authenticated unified gateway` | The local binding lacks the transport required by `open_daemon_client` |
| `the authenticated daemon has not admitted the complete payment journey` | Full installation lacks one or more required activity tools |
| JSON-RPC `-32700` | Invalid JSON |
| JSON-RPC `-32600` | Message did not name a method |
| JSON-RPC `-32601` | Method is not implemented |
| JSON-RPC `-32602` | Missing tool name, absent tool, or a write attempted through the CLI read-only client |
| `mcp.idempotency_key_required` / `mcp.idempotency_key_conflict` | Missing, malformed, zero, or conflicting invocation key |
| `mcp.description_invalid` | The daemon's catalogue response failed strict validation |
| `gateway.authentication` | Gateway credential rejected |
| `outcome.unknown` | The invocation may have reached the daemon; resolve it without inventing a new operation |

Tool argument validation returns `isError: true` with `stage: "arguments"`.
Daemon failures return `stage: "daemon"` with the structured refusal,
including `state: "refused"` or `"unknown"` where available. A tool response
can contain `result` while still having `isError: true` when its state is
refused or unknown; inspect both fields. Successful tool content uses
`structuredContent` with `{tool, result}`.

Library callers also receive typed errors such as
`ServerError::MissingSession`, `MissingCapability`, `ClosedSession`,
`RevokedSession`, `TenantMismatch`, `CapabilityMismatch`, `ExpiredAuthority`,
`NoScope`, `ToolAbsent`, and `InvocationUnknown`. Read helpers can refuse
`Unverified`, `HeightMismatch`, invalid bounds, cursor mismatches, oversized
results, or missing receipt evidence. Write helpers report the failing stage
and refuse success without a verified receipt. Preserve these distinctions
when presenting an error to an agent or deciding whether to resume tracking.

[Home](Home.md)
