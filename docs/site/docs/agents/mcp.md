# MCP

MCP exposes scoped tools backed by an authenticated `layerx-agentd` session.
The daemon owns policy, preparation, signing, submission and recovery; an
MCP server does not acquire protocol authority by translating a tool call.
Paxeer is one BFT network. Chain results carry the evidence actually checked,
and transaction finality follows the committed block.

Tools can also return daemon-local state. `tenant.readiness`, subscriptions
and approval lifecycle facts are authenticated local information, not proof
of chain execution. Preparation, disclosure and signing likewise do not
claim a completed transaction. Submission and tracking report a transaction
hash and evidence rung, a verified receipt, or an honest non-terminal state.

## Surfaces

| Entry point | Role |
| --- | --- |
| `services/agent/crates/layerx-mcp` | Tenant- and scope-bound MCP server and standalone `layerx-mcp` binary |
| Developer CLI | `layerx install mcp` installs a host entry; `layerx mcp serve --daemon-binding <path>` opens the daemon session described by the binding |

Every binding fixes a tenant, session, generation and capability. Tools are
filtered by the bound scope set and deployment mode; read-only deployment
omits write tools. Calls still pass through daemon authorization, so a listed
tool is not a permission to bypass policy or use another tenant's objects.

Sources: `services/agent/crates/layerx-mcp/src/{server,binding,stdio}.rs`.
See [Agentd](agentd.md) for local enforcement and
[Node access](lni.md) for the chain connection that replaced LNI.

## Tools in `layerx-mcp`

`TOOL_CATALOGUE` in `src/server.rs` declares these **22 base tools**.
This is the base inventory, not a promise that a particular session lists
all of them. `tools/list` is filtered by scopes and deployment mode and also
includes the session tools below where authorized.

| Tool | Kind | Required scope |
| --- | --- | --- |
| `tenant.readiness` | read | `read` |
| `balance.get` | read | `read:balance` |
| `wallet.balance` | read | `read:wallet:balance` |
| `wallet.accounts` | read | `read:wallet:accounts` |
| `history.list` | read | `read:history` |
| `receipt.get` | read | `read:receipt` |
| `checkpoint.get` | read | `read:checkpoint` |
| `proof.get` | read | `read:proof` |
| `availability.get` | read | `read:availability` |
| `activity.prepare` | write | `write:prepare` |
| `activity.disclose` | write | `write:disclose` |
| `activity.sign` | write | `write:sign` |
| `activity.submit` | write | `write:submit` |
| `wallet.send` | write | `write:wallet:send` |
| `token.create` | write | `write:token:create` |
| `token.mint` | write | `write:token:mint` |
| `token.transfer` | write | `write:token:transfer` |
| `grant.issue` | write | `write:grant:issue` |
| `grant.draw` | write | `write:grant:draw` |
| `activity.track` | write | `write:track` |
| `activity.wait` | write | `write:activity:wait` |
| `faucet.request` | write | `write:faucet:claim` |

`tenant.readiness` reads daemon recovery and transport readiness through the
authenticated gateway. Its evidence is explicitly **unverified protocol
state**. It is useful for deciding whether an agent can proceed, not for
claiming a balance, receipt or committed block.

`checkpoint.get` is a retained tool name for block-commit evidence; it is not
a separate settlement checkpoint. Read results retain their verification and
freshness metadata. `faucet.request` is a testnet funding request through
`px_requestFunds`, returning a funding transaction hash or an explicit
refusal; that hash alone is not a verified committed receipt.

### Session and optional web tools

`SESSION_TOOLS` in `src/catalogue.rs` adds these 11 tools. Their mutations
change daemon state. An approval can expose a tracked `tx_hash` and rung,
but approving an exact held request does not execute it on chain.

| Tool | Kind | Required scope |
| --- | --- | --- |
| `subscription.create` | write | `subscribe` |
| `subscription.list` | read | `subscribe` |
| `subscription.pause` | write | `subscribe` |
| `subscription.resume` | write | `subscribe` |
| `subscription.delete` | write | `subscribe` |
| `subscription.health` | read | `subscribe` |
| `subscription.acknowledge` | write | `subscribe` |
| `approval.list` | read | `approve` |
| `approval.get` | read | `approve` |
| `approval.approve` | write | `approve` |
| `approval.reject` | write | `approve` |

When the web boundary is configured, `WEB_TOOLS` declares `web.search`,
`web.fetch` and `web.content`, with scopes `write:web:search`,
`write:web:fetch` and `write:web:content`. Web operations can spend funds,
require their own payer/approval binding, and return untrusted external
content. They are not implied by a read-only binding.

Use the served `tools/list` descriptors for argument schemas and available
tools. Other CLI operations do not automatically become MCP tools.

## Serving

Both entry points take an operator-owned binding document:

```sh
layerx-mcp /absolute/path/to/binding.json
layerx mcp serve --daemon-binding /absolute/path/to/binding.json
```

The standalone binary in `src/bin/layerx-mcp.rs` opens the daemon client
session first. **Without a `listener` declaration it serves standard input
and output. With `listener` it binds a Unix socket**, admits peers by their
credentials and serves that same session. The CLI serve path uses standard
input and output. The model-facing process uses daemon credentials rather
than keeping a signing seed.

`Binding::open` requires an absolute, canonical, owner-only regular file,
bounded to 65,536 bytes. The JSON object is closed: unknown fields are refused.
Its required fields are:

- `mode` (`full` or `read-only`), `tenant`, `store`, `audit_root`;
- `session_id`, `session_token_file`, `session_generation`, `capability_id`;
- `core_height`, positive `deadline_ms`, `agent` and `limit`.

`agent` declares `endpoint`, `bearer_file` and `probe_program`, with optional
`readiness` containing `endpoint`, `gateway_key_file` and optional
`trust_anchors`. `limit` declares `id`, `name`, `scope`, `scope_id`, `ceiling`
and `consumed`. These are bindings to daemon-owned records, not independent
permissions minted by the document. The old `core_sequence` binding key is
retired; the current key is `core_height`.

The optional `listener` object requires `socket`, `owner_uid`, `owner_gid`,
`mode` and `admitted_uids`. Its mode is an octal string granting owner
read/write and no permissions beyond owner/group read/write (for example,
`"0600"` or `"0660"`). The normalized absolute socket path must be inside a
dedicated directory. `admitted_uids` contains 1 to 64 distinct unsigned user
ids. Listener validation additionally checks directory ownership, permissions
and peer credentials. A live socket is refused rather than replaced.

Source of truth for binding validation: `src/binding.rs` and
`src/listener.rs`. For daemon publication, full mode reads
`LAYERX_AGENT_MCP_BINDING_ROOT` and its explicit enrolment, session, scope and
listener settings in `services/agent/crates/layerx-agentd/src/main.rs`.
Human-owner-only mode refuses that publication because it does not serve
the program endpoint and probe program the binding requires.

Host values for `layerx install mcp` are `layerx`, `claude-code`,
`claude-desktop`, `cursor` and `vscode`; see [CLI](../platform/cli.md).

## Interop ingress

MCP is also an ingress label on the interoperability gateway alongside HTTP
and A2A. That translation boundary is distinct from this daemon-bound
catalogue. A2A support lives in the interop gateway, x402 and the developer
CLI; there is no standalone `layerx-a2a` crate. Do not infer MCP tools or
receipt verification from an ingress label alone.

See [Running an agent](running.md), [x402](../interop/x402.md),
[Agentd](agentd.md) and [Node access](lni.md).
