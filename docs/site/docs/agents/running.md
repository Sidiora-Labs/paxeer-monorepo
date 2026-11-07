# Running an agent

An agent runs against Paxeer X Network, one BFT chain with EVM chain ID
`125` (`0x7d`). The kernel is part of the network and participates in block
proposal and execution inside `paxd`. A transaction is final when its block
is committed. An agent daemon manages local authority, signing, budgets and
recovery; it does not run a second execution or settlement network.

This path connects an agent runtime to `layerx-agentd` through MCP. The CLI
registers a stdio process using a daemon-issued binding document. The daemon
retains the session, capability and durable invocation state. The CLI
forwards calls to that authority rather than opening an independent local
store or signing transactions itself.

See [Agentd](agentd.md) for daemon internals, [MCP](mcp.md) for the protocol,
[CLI](../platform/cli.md) for command reference, and the
[payments developer path](../overview/payments.md) for wallet and token work.
The source paths below are relative to the repository root.

## Point the CLI at the network

Keep the node RPC origin and the hosted gateway origin separate:

| Connection | Address and purpose |
| --- | --- |
| Public node JSON-RPC | HTTPS at the root of `api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network`, or `api-mainnet-beta.paxeer.network`; chain ID `125` |
| Hosted gateway | Deployment-specific origin for authenticated agent operations, tenant credentials, `px_register` and `px_requestFunds` |
| Agent program listener | Private loopback `127.0.0.1:<port>` endpoint protected by its program bearer |
| Human and local MCP listeners | Protected Unix sockets with explicit peer admission |

The public node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `px`,
`test` and `txpool`. The `px` namespace is served at the node root from node
state, including `px_getReceiptKeys`; only `px_register` and `px_requestFunds`
are hosted operations. The gateway's method contract is declared in
`services/platform/hosted/gateway/openrpc.json`. Use the operator-provided
hosted origin and credentials for registration and funding.
See [Public RPC endpoints](../reference/public-rpc.md).

For example, check the node identity before signing or broadcasting:

```bash
curl --fail-with-body --max-time 30 \
  https://api1.mainnet-beta.paxeer.network/ \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

The expected result is `"0x7d"`. Keep bounded timeouts and more than one
node configured; compare `eth_blockNumber` when selecting a node near the
head. `node.hyperpaxeer.com` is only the hpx library mirror, never an RPC endpoint.
The pre-merge router and its `/rpc`, `/rpc/ws` and `/rpc/schema` paths are retired.

### CLI profile compatibility

MCP installation and serving use the binding document, not a CLI environment
profile. A separately configured hosted CLI workflow can select its existing
profile with:

```text
layerx environment use beta
```

The CLI accepts profile names `emulator`, `beta`, and `production`. In the
current implementation, creating a profile still requires `--endpoint`,
`--network-id` and either `--sequencer-trust-anchor` or
`--sequencer-trust-anchor-file` together. These are compatibility fields of
`EnvironmentCommand::Use` and `emulator::resolve_inputs`, not a requirement
to operate a separate network service. For non-emulator profiles, selection
does not verify the supplied anchor against the endpoint. Do not treat
profile selection as receipt verification or finality, and do not fabricate
an anchor to make a profile usable. Obtain any required receipt key from the
operator's authenticated key policy.

Non-loopback CLI HTTP endpoints require HTTPS. `environment.selected`
returns `name`, `endpoint`, `network_id` and the compatibility
`sequencer_trust_anchor` field; `--json` uses the usual
`{ok, kind, message, data}` envelope. Sources:
`tools/platform/cli/src/main.rs`, `config.rs`, `emulator.rs`, `http.rs`
and `output.rs`.

## Tenant credential and scope set

Sub-chains are tenant domains in the database, launchable without new
infrastructure. Carry the tenant through identity resolution, capability
lookup, session authentication, budgets, audit records and subscriptions.
A session for one tenant cannot borrow authority from another tenant.

### Hosted gateway key (CLI command)

For hosted CLI operations, create a key in operating-system credential
storage and provide an identity session through stdin:

```text
layerx key create <name>
printf '%s\n' '<identity-session>' | layerx auth set --environment beta
```

`key create` generates a 32-byte Ed25519 seed from operating-system randomness
under keyring service `dev.layerx.cli`; its public output includes `name`,
`did` and `public_key`. The identity session authorizes hosted key
provisioning. Keep session tokens, gateway keys, daemon bearers and signing
seeds out of runtime host JSON and logs.

`layerx install a2a` provisions a hosted key through `/v1/keys` or the key
rotation route. Its payment scopes are `activity:write` and `receipt:read`;
read-only uses `receipt:read`. These hosted scopes differ from MCP daemon
scopes such as `write:submit`. The A2A transport has compatibility limitations
described under [Submit one signed activity](#submit-one-signed-activity).

`layerx install mcp` does not provision a gateway key or read an identity
session from the CLI profile. It writes an empty process `env` into the host
registration. The authenticated transport's credential files are referenced
by the binding instead. Sources: `tools/platform/cli/src/credential.rs`,
`install/mod.rs`, `install/a2a.rs`, and `install/mcp.rs`.

### Daemon enrolment and the binding document

There is no `layerx` subcommand that starts the daemon, enrols a daemon
session or writes its binding. `layerx-agentd` is a library and a binary;
an operator launches the binary with its deployment configuration. Full
agent mode can enrol a configured session at boot. Human-owner-only mode
refuses `LAYERX_AGENT_MCP_BINDING_ROOT` because it does not serve the program
endpoint the binding names.

With enrolment enabled, the daemon resolves the agent DID through the Human
authority, checks the configured peer's tenant and principal, registers the
identity, restores the capability and opens a session. `enrolment::enrol`
mints a 32-byte random session token and publishes through `BindingPublisher`.
If publication fails it attempts to close the session; `OrphanedSession`
reports failure of that cleanup too. A restored open session must match its
published binding. Current boot code also supports republication with
verified capability expiry when that evidence is available.

The publisher creates a mode `0700` binding directory containing:

| File | Contents |
| --- | --- |
| `binding.json` | Session coordinates, mode, transport references and local limit |
| `session-token` | Hex-encoded session token, mode `0600` |
| `daemon-bearer` | Program transport bearer, mode `0600` |

Secret files use exclusive creation. A conflicting publication is refused
rather than silently overwritten; identical publication is idempotent.
Protect the directory, store, audit root and referenced credential files.

The current document uses `core_height`, not the retired `core_sequence`
field. Its required fields are `mode`, `tenant`, `store`, `audit_root`,
`session_id`, `session_token_file`, `session_generation`, `capability_id`,
`core_height`, `deadline_ms`, `agent`, and `limit`. `agent` names `endpoint`,
`bearer_file` and `probe_program`, with optional `readiness`; `limit` names
`id`, `name`, `scope`, `scope_id`, `ceiling` and `consumed`. Optional top-level
`listener` and `web` declarations describe local peer admission and paid web
access. Unknown fields are refused.

For the current CLI, `agent.readiness` is required in practice:
`Binding::open_daemon_client` refuses a document without the authenticated
gateway declaration. It contains `endpoint`, `gateway_key_file` and optional
`trust_anchors`. The CLI uses this transport to reach the live daemon.
The full-mode binary's basic `publish_mcp_binding` path attaches listener
and optional web declarations but does not supply this readiness declaration.
Consequently, basic boot publication alone is insufficient for CLI MCP
installation; a deployment must supply a binding through an enrolment path
that configures `DaemonSurface::with_readiness`, and an authenticated route
to the same daemon owner. An installed JSON entry is not evidence that this
connection has been admitted.

Sources: `services/agent/crates/layerx-agentd/src/enrolment.rs`, `main.rs`,
`human_owner_mode.rs`, `session.rs`, and
`services/agent/crates/layerx-mcp/src/binding.rs`.

## Start the MCP server and connect a runtime

Once the daemon and its authenticated gateway route are running and the
binding is usable:

```text
layerx install mcp --host <runtime> --daemon-binding <absolute-binding-path>
```

Supported hosts are `layerx`, `claude-code`, `claude-desktop`, `cursor` and
`vscode`. `--host` is repeatable; omitting it selects the LayerX host and
other detected runtimes. `--read-only` narrows the binding's mode. Without
`--daemon-binding`, the installer looks for `mcp/binding.json` beside the CLI
configuration file. Explicit relative paths are made absolute for launch.

The installer parses the document, authenticates to the daemon and fetches
its actual `tools/list`. It refuses tools outside its installation catalogue
and, in full mode, refuses a catalogue missing any of `activity.prepare`,
`activity.disclose`, `activity.sign`, `activity.submit` or `activity.track`.
A missing or malformed document, unavailable authenticated route or refused
session prevents installation. There are no environment, key, account or
asset selection flags on this command.

The generated registration names the current CLI executable and arguments:

```text
mcp serve --daemon-binding <absolute-binding-path> [--read-only]
```

You can start the same stdio transport directly:

```text
layerx mcp serve --daemon-binding <absolute-binding-path> --read-only
```

Host registration locations are:

| Host | Document | Section |
| --- | --- | --- |
| `layerx` | `mcp.json` beside the CLI configuration | `mcpServers` |
| `claude-code` | `~/.claude.json` | `mcpServers` |
| `claude-desktop` | `$XDG_CONFIG_HOME/Claude/claude_desktop_config.json` | `mcpServers` |
| `cursor` | `~/.cursor/mcp.json` | `mcpServers` |
| `vscode` | `$XDG_CONFIG_HOME/Code/User/mcp.json` | `servers` |

On macOS the desktop paths use `~/Library/Application Support/Claude/` and
`~/Library/Application Support/Code/User/`. `LAYERX_INSTALL_ROOT` replaces
`HOME` for installation roots. VS Code registrations include `type: "stdio"`.
The process environment is empty in the registration; credentials remain
in protected referenced files.

Successful installation returns kind `install.mcp`, the effective
`deployment_mode`, `daemon_binding` coordinates including `core_height`,
`server`, `tools`, `scopes`, registration reports, `changed` and
`idempotent: true`. Read `tools/list` from the served session to determine
what this authority can actually invoke. In-process consumers use
`Server::bind` or `ReadOnly::bind` with shared `SessionControl`; the
`layerx-mcp` crate itself has no binary.

Sources: `tools/platform/cli/src/mcp.rs`, `install/mcp.rs`, `install/mod.rs`,
`install/binding_shape.rs`, and
`services/agent/crates/layerx-mcp/src/server.rs`.

## Grant a budget

There is no `layerx budget` CLI command. Use the authenticated agent operation
or the daemon library with the correct tenant, signer, capability and policy.

A local `LocalLimit` is labelled `daemon-enforced`: bypassing agentd bypasses
that limit. `BudgetLimiter` supports tenant, agent, session, capability and
counterparty scopes. It reserves locally before effects and reconciles the
result; a local reservation is not an on-chain budget.

Protocol budget creation is implemented by `create_protocol_budget` in
`services/agent/crates/layerx-agentd/src/budget/create.rs`. It submits an
exact signed EIP-1559 call to `layerxpay` (`0x000000000000000000000000000000000000101b`),
checks the committed receipt and creation log, and compares the proven budget
state at the same height against the requested asset, ceiling, expiry and
other creation fields. It returns a `ProtocolBudget` with
`enforcement: "protocol-enforced"` only after those checks. Missing or
mismatched resulting state returns `CreatedBudgetUnconfirmed`. A successful
broadcast alone does not create a confirmed budget.

## Read state

Read tools include balance, wallet accounts, history, receipts, checkpoints,
proofs and availability. The bound daemon authorizes each invocation;
untrusted arguments cannot replace the bound tenant, authority or scope.
`tenant.readiness` reports daemon-local readiness and is explicitly unverified
protocol state. Subscription and approval list/get results likewise describe
authenticated daemon state, not a proof of chain finality.

Read helpers return `VerifiedToolResult` with `value`, optional `rung`,
`freshness` and `page` information. Preserve these fields when displaying
results. `ReceiptValue` includes canonical receipt bytes and evidence IDs.
Read-only deployments omit write tools; session scopes can narrow the set
further. The CLI enforces read-only filtering for both listing and calls.

For independent native reads, use the root node JSON-RPC with the expected
chain ID and a deliberate block tag. Pending reads describe speculative
state; committed reads must be checked against authenticated block and state
evidence. The daemon implementation is in
`services/agent/crates/layerx-agentd/src/read/` and `node_rpc.rs`;
MCP result types are in `services/agent/crates/layerx-mcp/src/tools/read.rs`.

## Submit one signed activity

The MCP payment journey is prepare, disclose, policy, sign, submit and track.
Use the argument schemas returned by the bound daemon. Current native
operations also depend on the daemon's admitted native policy and signing
profiles; a catalogue name alone does not grant spending authority.
`activity.submit` over MCP reaches the authenticated daemon and its durable
invocation handling. It is not the hosted A2A tool's argument contract.

For a transaction already signed for chain ID `125`, node submission uses
`eth_sendRawTransaction` at the root origin. This JSON is a request shape;
replace the placeholder with the exact signed transaction:

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "eth_sendRawTransaction",
  "params": ["<0x02-prefixed-signed-transaction>"]
}
```

Send it to, for example, `https://api1.mainnet-beta.paxeer.network/`.
The returned hash acknowledges submission. It is not proof that execution
succeeded or that a block committed. Retain the signed bytes, transaction
hash, request identity and idempotency key. An interrupted send has an
unknown outcome; resolve the original transaction before authorizing another
economic action. Daemon invocation recovery reconciles a retained submission
reference instead of blindly replaying an uncertain effect.

### A2A compatibility limitation

`layerx a2a serve` exposes `receipt.get` (`receipt:read`),
`activity.submit` (`activity:write`) and `faucet.request`
(`write:faucet:claim`). A2A `activity.submit` accepts only `raw_transaction`
and returns `txHash` with `rung: "submitted"`. However,
`tools/platform/cli/src/toolset.rs` still posts these operations to the retired `/rpc` path.
The current A2A implementation therefore does not provide a working public
root-node submission path. Changing its profile URL does not remove the
hard-coded path. Use a root-aware node client for native broadcasting; do
not describe the compatibility A2A route as a public network endpoint.
Hosted funding is a separate authenticated service governed by its declared
`px_requestFunds` contract, not a public-node faucet method.

## Approve

There is no `layerx approval` command. An admitted session may expose
`approval.list`, `approval.get`, `approval.approve` and `approval.reject`
under scope `approve`. The Human socket also has approval operations;
application integrations can use `ApprovalService` and its submission queue.

Approvals are daemon policy. They do not mint protocol authority or add a
finality stage. A decision must bind the exact held disclosure and digest,
the approver and an idempotency key. `ApprovalError::DisclosureChanged`
refuses a decision for changed material. The ordinary write helper is not a
universal automatic approval hold: integrate the daemon's policy boundary
for the operation being served. The paid web route explicitly uses an
approval registry and configured amount threshold before spending.

Sources: `services/agent/crates/layerx-mcp/src/approval.rs`, `server.rs`,
`tools/web.rs`, and `services/agent/crates/layerx-agentd/src/approval/`.

## Observe the receipt

The transaction lifecycle is:

| State | Meaning |
| --- | --- |
| `submitted` | The node knows the transaction hash; signed execution evidence is not yet available |
| `receipted` | A signed execution receipt exists; its covering block is not yet committed |
| `committed` | The receipt is covered by the sequence batch in a committed block |

`pax_sequencerReceipt` is the node lookup by transaction hash. It may return
`null` for an unknown hash; preserve pending or unknown status rather than
inventing a receipt. `rpc/sequencer_receipt.go` checks committed sequence
coverage and supplies the committed block number and hash. Finality comes
from that BFT commit, not from an external settlement chain.

Authenticate receipt keys independently of the receipt being checked. The
hosted gateway publishes `px_getReceiptKeys` as an object containing `keys`,
with `keyId`, `publicKey`, `firstHeight` and `lastHeight`; an unauthenticated
response from the same server as the receipt is not an independent pin.
For committed results, verify the covering block and commit against the
trusted validator set and bind the receipt to the exact signed transaction.
The SDK verifiers are `verifySequencerReceipt` in the TypeScript
`services/agent/sdk/typescript/src/verifier.ts` and `verify_sequencer_receipt`
in Python `services/agent/sdk/python/layerx_sdk/verifier.py`. Signature
verification alone does not establish a BFT commit.

MCP receipt results retain `canonical_receipt` and non-empty `evidence_ids`.
The write helpers return `Executed`, `Pending` or `Unknown`; an `Executed`
outcome without verified receipt evidence is refused as
`SuccessWithoutVerifiedReceipt`. Preserve the receipt evidence and
transaction rung instead of presenting every tool success as final.

The existing `layerx receipt verify` command still consumes an authorized
batch, asset, state roots and a signing key through the older proof format
in `tools/platform/cli/src/receipt.rs`. It is not a verifier for the current
node's committed-block envelope. Do not use that command or the obsolete
SDK `verify_receipt` example as instructions for current network finality.

## Observe revocation

`session::close` advances generation and closes the local session.
`session_revocation.rs` also validates receipt-backed `layerxauth` grant and
revocation events at `0x000000000000000000000000000000000000101c`. Restored
sessions must still satisfy current generation, tenant, capability, scope
and expiry checks before a tool effect is authorized.

Local revocation events invalidate matching sessions and generations and
cancel prepared or signed work. Uncertain submitted work requires
resolution; an on-chain grant revocation also marks matching queued or
unknown activities cancelled. Completed effects are not undone. Revoking a hosted gateway key
is a separate operation from closing a daemon session or revoking a protocol
grant; integrations must track the relevant authorities explicitly.

Daemon local event delivery uses Unix frames `LXOW` / `LXOA`. Consumers must
deduplicate by `deduplication_id`; it is not an HTTP webhook. Sources:
`services/agent/crates/layerx-agentd/src/session.rs`, `session_revocation.rs`,
`session_control.rs`, and `events/`.

## MCP tools

The live daemon's `tools/list` is authoritative for the session. The current
source catalogue contains 22 base definitions, 11 subscription/approval
definitions, and three paid web definitions; mode, scope, native authority
and runtime route availability determine the served subset. There is no
fixed promise of 21 tools.

| Base tool | Required scope | Kind |
| --- | --- | --- |
| `tenant.readiness` | `read` | read |
| `balance.get` | `read:balance` | read |
| `history.list` | `read:history` | read |
| `receipt.get` | `read:receipt` | read |
| `checkpoint.get` | `read:checkpoint` | read |
| `proof.get` | `read:proof` | read |
| `availability.get` | `read:availability` | read |
| `wallet.accounts` | `read:wallet:accounts` | read |
| `wallet.balance` | `read:wallet:balance` | read |
| `activity.prepare` | `write:prepare` | write |
| `activity.disclose` | `write:disclose` | write |
| `activity.sign` | `write:sign` | write |
| `activity.submit` | `write:submit` | write |
| `activity.track` | `write:track` | write |
| `wallet.send` | `write:wallet:send` | write |
| `token.create` | `write:token:create` | write |
| `token.mint` | `write:token:mint` | write |
| `token.transfer` | `write:token:transfer` | write |
| `grant.issue` | `write:grant:issue` | write |
| `grant.draw` | `write:grant:draw` | write |
| `activity.wait` | `write:activity:wait` | write |
| `faucet.request` | `write:faucet:claim` | write |

`subscription.create`, `.list`, `.pause`, `.resume`, `.delete`, `.health`
and `.acknowledge` use scope `subscribe`; list and health are read tools.
`approval.list`, `.get`, `.approve` and `.reject` use `approve`; list and get
are read tools. `web.search`, `web.fetch` and `web.content` use the respective
`write:web:search`, `write:web:fetch` and `write:web:content` scopes and are
write tools because they can spend. Treat returned web content as untrusted.

`grant.issue` and `grant.draw` remain catalogue definitions, but native
aliases are filtered using the active authority chain and the relevant
native effect or send profile. Use only an advertised and admitted tool
with its returned schema. Catalogue presence does not guarantee that a
particular asset, capability or deployment supports an operation.
Sources: `services/agent/crates/layerx-mcp/src/server.rs` and `catalogue.rs`.

## Agentd configuration keys

### Library startup configuration

`config::load` reads a protected UTF-8 `key=value` file. Exact `LAYERX_*`
environment values override file entries; duplicates, blank values, unknown
settings and incomplete tenant maps are refused. It is a library API,
not the binary's environment loader.

| File key | Environment key | Current interpretation |
| --- | --- | --- |
| `network_id` | `LAYERX_NETWORK_ID` | Non-zero `u32` network identity |
| `node_endpoint` | `LAYERX_NODE_ENDPOINT` | HTTP or HTTPS JSON-RPC URL, not a filesystem path |
| `expected_chain_id` | `LAYERX_EXPECTED_CHAIN_ID` | Non-zero decimal `u64`; `125` for mainnet-beta |
| `tenants` | `LAYERX_TENANTS` | Unique tenant IDs |
| `policy_sources` | `LAYERX_POLICY_SOURCES` | `tenant:/absolute/path` map covering every tenant |
| `signer_configurations` | `LAYERX_SIGNER_CONFIGURATIONS` | Same complete per-tenant path map |
| `verification_defaults` | `LAYERX_VERIFICATION_DEFAULTS` | Complete per-tenant compatibility verification policy map; see limitation below |
| `receipt_key_source` | `LAYERX_RECEIPT_KEY_SOURCE` | Protected pinned-policy path; discovery selector has a boot limitation below |

`expected_protocol_version` and `sequencer_authority_source`, including their
environment forms, are retired and explicitly refused. Tenant parsing here
accepts lowercase ASCII letters, digits, `-`, `_` and `.`. Protected sources
must be canonical absolute regular files owned by the process user, without
group or other permission bits.

The parser still accepts the old verification labels from `sequencer-signed`
through `settlement-anchored`. Those are compatibility enum values, not a
set of separate network finality stages. Do not configure a supposed external
settlement dependency from these labels. Integrations must require verified
committed-block evidence for finality.

Although `receipt_key_source=px_getReceiptKeys` parses, the current
`ProtocolEvidenceVerifier::load` returns `AuthoritySourceUnavailable` for
that selector. The pinned-file loader still reads its versioned legacy
policy format, so changing a filename does not convert it to committed-block
trust. Separately, `Gate::readiness_gate` calls `NodeRpc::receipt_keys`, which
uses the hosted `px_getReceiptKeys` method through the configured node client.
A public root node alone cannot satisfy that join. A deployment must supply
an explicitly authenticated hosted integration and appropriate pinned trust;
do not disable the gate or attribute the hosted method to the public node.
Sources: `config.rs`, `boot.rs`, `node_rpc.rs` and `protocol_evidence.rs` under
`services/agent/crates/layerx-agentd/src/`.

### Binary environment and MCP enrolment

The binary reads deployment environment variables directly. The following
are the principal groups; this is not a minimal standalone launch file.
It also needs the runtime clock and the operation-specific policies, trust
and bounds read by `main.rs` and `human_owner_mode.rs`.

| Group | Configuration |
| --- | --- |
| Mode | `LAYERX_AGENT_MODE=full` (default) or `human-owner`; MCP boot enrolment requires full mode |
| Human node | `LAYERX_AGENT_NODE_RPC_URL` is the root JSON-RPC URL; `LAYERX_AGENT_NODE_CHAIN_ID` accepts decimal or canonical hex (`125` or `0x7d`) |
| Human identity | `LAYERX_AGENT_HUMAN_NETWORK_ID` must equal the node chain ID; `LAYERX_AGENT_HUMAN_PROTOCOL_VERSION` must be the compiled current state-commitment protocol (`3`) |
| Human persistence | Absolute `LAYERX_AGENT_HUMAN_STORE`, `LAYERX_AGENT_HUMAN_SOCKET`, `LAYERX_AGENT_HUMAN_SESSION_KEY_ROOT`, and protected `LAYERX_AGENT_HUMAN_SESSION_OPERATOR_SECRET_FILE` |
| Peer admission | `LAYERX_AGENT_HUMAN_PEERS` contains comma-separated `uid=<u32>;tenant=<tenant>;principal=<did>` entries; duplicate UIDs and malformed entries are refused |
| Human authority | `LAYERX_AGENT_HUMAN_AUTHORITY_ENDPOINT`, `LAYERX_AGENT_HUMAN_AUTHORITY_BEARER`, `LAYERX_AGENT_HUMAN_AUTHORITY_CA_DER` and `LAYERX_AGENT_HUMAN_AUTHORITY_MAX_BYTES` |
| Human transport bounds | `LAYERX_AGENT_HUMAN_DEADLINE_MS`, `LAYERX_AGENT_HUMAN_MAX_FRAME_BYTES`, `LAYERX_AGENT_HUMAN_MAX_CONNECTIONS`, `LAYERX_AGENT_HUMAN_MAX_STREAMS`, `LAYERX_AGENT_HUMAN_MAX_QUEUED_BYTES`, `LAYERX_AGENT_HUMAN_MAX_PAYLOAD_BYTES`, `LAYERX_AGENT_HUMAN_TIMESTAMP_SPAN` (maximum width of an activity timestamp bound, in milliseconds of protocol time; the hosted material writes `60000`) |
| Human socket ownership | `LAYERX_AGENT_HUMAN_SOCKET_UID`, `LAYERX_AGENT_HUMAN_SOCKET_GID`, `LAYERX_AGENT_HUMAN_SOCKET_MODE` |
| Local limit | `LAYERX_AGENT_HUMAN_LIMIT_ID`, `LIMIT_SCOPE`, `LIMIT_SCOPE_ID`, `LIMIT_NAME`, `LIMIT_CEILING`, `LIMIT_CONSUMED` with the same prefix; ID is 16 bytes of hex and scope identity is 32 bytes of hex |
| Full-mode program listener | `LAYERX_AGENT_PROGRAM_LISTEN`, `LAYERX_AGENT_PROGRAM_BEARER_TOKEN`, `LAYERX_AGENT_PROGRAM_PROBE_ID`, `LAYERX_AGENT_PROGRAM_MAX_STALENESS_MS` and `LAYERX_AGENT_DEPLOYMENT_JOURNAL` |
| Full-mode program trust | `LAYERX_AGENT_NODE_ENDPOINT`, `LAYERX_EXPECTED_CHAIN_ID`, `LAYERX_AGENT_NODE_BEARER_TOKEN`, `LAYERX_AGENT_AUTHORITY_ENDPOINT`, `LAYERX_AGENT_AUTHORITY_BEARER_TOKEN`, `LAYERX_AGENT_AUTHORITY_CA_DER`, `LAYERX_AGENT_AUTHORITY_REPLICA_ID`, `LAYERX_AGENT_CHAIN_TRUST_ROOT` |
| Tenant policies | `LAYERX_POLICY_SOURCES` and `LAYERX_NATIVE_POLICY_SOURCES`; optional `LAYERX_NATIVE_EFFECT_POLICY_SOURCES` and `LAYERX_PROGRAM_BUDGET_DENOMINATION_SOURCES` |

The program listener requires loopback and a bearer of at least 32 bytes;
its bearer, node bearer and authority bearer must be distinct. The program
node endpoint is a separate configuration field from the Human node RPC
URL and may need hosted read/key functionality. Do not silently substitute
one origin for the other.

Setting `LAYERX_AGENT_MCP_BINDING_ROOT` enables boot enrolment. Configure:

| MCP key suffix (prefix `LAYERX_AGENT_MCP_`) | Meaning |
| --- | --- |
| `BINDING_ROOT`, `AUDIT_ROOT` | Absolute publication and audit directories |
| `PEER_UID`, `AGENT_DID` | Configured Human peer and authority-resolved agent |
| `SESSION_ID`, `CAPABILITY_ID` | 32-byte hex identifiers |
| `ACTIVITY_TYPES`, `SCOPES` | Comma-separated unique activity types and scopes |
| `CORE_HEIGHT`, `EXPIRY_HEIGHT` | Unsigned block heights; internal session APIs retain some sequence-named fields |
| `OPENING_CLIENT`, `POLICY_VERSION` | Recorded session metadata |
| `MODE` | `full` or `read-only` |
| `DEADLINE_MS` | Non-zero transport deadline |
| `LIMIT_ID`, `LIMIT_NAME`, `LIMIT_SCOPE`, `LIMIT_SCOPE_ID`, `LIMIT_CEILING`, `LIMIT_CONSUMED` | Bound local limit |
| `LISTENER_SOCKET`, `LISTENER_OWNER_UID`, `LISTENER_OWNER_GID`, `LISTENER_MODE`, `LISTENER_ADMITTED_UIDS` | Protected Unix listener; mode is octal, admitted UID list must include `PEER_UID` |

Optional `LAYERX_AGENT_MCP_WEB_ENDPOINT` enables web configuration. Then
provide the same prefix's `WEB_NETWORK`, `WEB_RECEIPT_KEY_INDEX`,
`WEB_TIMEOUT_MS`, `WEB_PENDING_ATTEMPTS` and `WEB_APPROVAL_THRESHOLD`.
The key index selects authenticated receipt-key material; current boot code
performs that lookup through `NativeReadRoute::signed_authority` on the
program node endpoint. This requires the hosted key-service join described
above, not a new public node namespace.

`LAYERX_AGENT_MCP_WEB_SEQUENCER_PUBLIC_KEY`,
`LAYERX_AGENT_MCP_CORE_SEQUENCE` and `LAYERX_AGENT_MCP_EXPIRY_SEQUENCE`
are retired and refused even when empty. Use governed key selection and the
height keys, preserving receipt-key validity intervals and chain identity.

## Typed refusals the agent sees

Handle refusals at the boundary that produced them; preserve pending and
unknown outcomes for reconciliation.

| Boundary | Examples and response |
| --- | --- |
| MCP install | Unreadable/malformed binding, missing authenticated gateway declaration, refused daemon authentication, or incomplete payment journey prevents registration |
| Daemon boot | Missing required settings and retired environment keys print a redacted `layerx-agentd:` diagnostic and exit `2` |
| Node connection | `ChainIdMismatch`, malformed RPC answers, transport unavailability or unavailable receipt keys leaves readiness closed |
| MCP protocol | Invalid JSON `-32700`, missing method `-32600`, unknown method `-32601`, invalid tool call `-32602` |
| Read-only CLI | A write call returns `-32602`, `the tool is not served by this read-only deployment` |
| Session authority | Missing, closed, revoked, expired or cross-tenant sessions and capabilities are refused before effects |
| Invocation recovery | `InvocationUnknown` retains uncertainty; reconcile the recorded submission instead of replaying it |
| Read evidence | `Unverified`, invalid bounds/cursor, oversized results or missing receipt evidence are errors |
| Write evidence | Named stage failures and `SuccessWithoutVerifiedReceipt` must not be displayed as execution success |
| Budget / approval | `LimitRefusal::Exceeded`, `CreatedBudgetUnconfirmed` and `DisclosureChanged` require an explicit operator or application decision |

MCP tool results use `structuredContent`; `isError: true` and a refusal
object are errors even when the outer JSON-RPC transport succeeds. Inspect
the tool, stage and retained transaction state. Sources:
`services/agent/crates/layerx-mcp/src/stdio.rs`, `server.rs`, `tools/read.rs`,
`tools/write.rs`, and `tools/platform/cli/src/mcp.rs`.

[Home](../index.md)
