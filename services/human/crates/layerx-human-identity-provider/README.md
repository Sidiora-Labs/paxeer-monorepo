# layerx-human-identity-provider

Linux LXIP identity directory for the Unix client in the
[Human service](../layerx-human-service/README.md), implemented in
`src/server/identity_dispatch.rs` there. This local directory is distinct from
the hosted HTTP identity service. It uses the service's `PrincipalId`, `Device`,
`AccountIdentity`, `Did` and recovery policy types. Its records support
principal-scoped onboarding on the merged chain, whose EVM chain id is 125;
local directory provisioning alone is not proof of an on-chain account binding.

## Run

Build from the repository root:

```sh
cargo build --locked --manifest-path services/human/Cargo.toml -p layerx-human-identity-provider
```

With the default Cargo target directory, the binary is
`services/human/target/debug/layerx-human-identity-provider` (release builds use
`services/human/target/release/layerx-human-identity-provider`). A configured
`CARGO_TARGET_DIR` changes those output paths. It takes at most one
command:

| Command | Effect |
| --- | --- |
| none, `serve` | Starts the listener |
| `probe` | Sends operation 0 to the running listener |
| `bind-device` | Imports one trusted device enrollment (below) |
| `provision-owner` | Runs operation 1 against the exclusively held state from at most 16384 bytes of JSON on stdin and prints the provisioned fields as JSON |
| `provision-account` | Derives the main account id from an exported DID using the retained protocol-3 canonical account domain; does not register an account or derive the asset account used for owner limits |
| `validate-account-head` | Reads authenticated committed-chain account observations and emits `layerx.human.account-head.v1` only for a bound owner asset account with no debit history |

Required environment for `serve`:

| Variable | Meaning |
| --- | --- |
| `LAYERX_HUMAN_IDENTITY_PROVIDER_STATE_ROOT` | Absolute canonical dedicated state directory; created as 0700 if absent, existing directory must be owner-only. Parent must already exist. |
| `LAYERX_HUMAN_IDENTITY_PROVIDER_SOCKET` | Absolute socket path, e.g. `/run/layerx/human/identity.sock`; parent must already exist, be owned by the effective UID, and not be group/world writable. |
| `LAYERX_HUMAN_IDENTITY_PROVIDER_ALLOWED_UID` | Decimal effective UID of the human-service process permitted by Linux SO_PEERCRED. Required; no wildcard. |
| `LAYERX_HUMAN_IDENTITY_PROVIDER_RECOVERY_POLICY_FILE` | Absolute path to a 0600, single-link, regular JSON file owned by the provider UID. Required even on restart. Its parent must be protected. |
| `LAYERX_HUMAN_IDENTITY_PROVIDER_DEADLINE_SECONDS` | Optional total connection I/O budget, 1–60 seconds; default 5. |
| `LAYERX_RUNTIME_CLOCK_SOCKET` | Required absolute Unix socket of the authenticated clock supervisor. Socket and parent directory must be private to the configured clock UID. |
| `LAYERX_RUNTIME_CLOCK_PID` | Required nonzero decimal supervisor PID, checked against SO_PEERCRED. |
| `LAYERX_RUNTIME_CLOCK_UID` | Required decimal supervisor UID, checked against socket ownership and SO_PEERCRED. |

Launch in this order:

1. Create the protected state/socket parent directories and recovery policy file
   as the provider UID, then set the provider variables above.
2. Start the runtime-clock supervisor as that same UID. It creates a private
   0700 generation directory and a 0600 clock socket, and injects all three
   `LAYERX_RUNTIME_CLOCK_*` bindings into its child. The PID identifies the
   supervisor, not the provider. The client checks directory/socket ownership
   and permissions and the supervisor's Linux SO_PEERCRED PID and UID on every
   connection; the supervisor also refuses clients of another UID.
3. Let the supervisor launch `serve`. The listener connects to the clock before
   opening and replaying its state and publishing the identity socket. Missing
   bindings, an unavailable clock or a mismatched peer refuses startup. Restart
   the provider with fresh bindings when replacing the supervisor.
4. Run `probe` against the published socket before starting dependent clients.
   It sends LXIP operation 0 and exits zero only on the exact empty success
   response; absent, stopped, refusing or unresponsive listeners fail within
   `LAYERX_HUMAN_IDENTITY_PROVIDER_DEADLINE_SECONDS` (default 5, range 1–60).
   Run it as the socket owner, which must also be the configured
   `LAYERX_HUMAN_IDENTITY_PROVIDER_ALLOWED_UID`: the probe checks socket owner
   and world permissions, and the listener enforces its allowed UID. A deployment
   using a different client UID must account for this built-in probe constraint.

For example, after setting the provider variables, the repository wrapper
starts the supervisor and its provider child:

```sh
sh tools/runtime/run-with-clock.sh services/human/target/debug/layerx-human-identity-provider serve
```

From another shell with the same provider socket and deadline settings and UID:

```sh
services/human/target/debug/layerx-human-identity-provider probe
```

`probe`, `bind-device`, `provision-owner` and `provision-account` do not attach
to the clock themselves. Both `serve` and `validate-account-head` require the
authenticated clock bindings. Readiness requires the serving provider's clock
to answer. Direct production launches must inherit all three bindings from
the live supervisor.
The [workspace guide](../../README.md) also describes the wrapper.

The recovery file has `root` (array of 32 byte integers), `threshold` (nonzero
u16), and `delay_seconds` (nonzero u64). The operator supplies an established,
exercisable recovery authority's commitment and policy. This server does not
invent recovery commitments or claim to implement that authority. Existing
accounts retain their original policy across configuration changes.

The socket mode is 0660, owned by the provider's effective UID and inherited
socket GID. Configure the client's `LAYERX_HUMAN_IDENTITY_SOCKET` to this socket,
`LAYERX_HUMAN_IDENTITY_PEER_UID` and `LAYERX_HUMAN_IDENTITY_PEER_GID` to its owner
and group, and grant the human service group access to the socket and traversal
of its parent. The client's `LAYERX_HUMAN_IDENTITY_MAX_FRAME_BYTES` should be
1048576 and `LAYERX_HUMAN_IDENTITY_DEADLINE_SECONDS` should accommodate queueing
and the server's I/O budget. The client independently checks socket ownership,
world permissions and server SO_PEERCRED UID/GID before sending requests.

The listener handles one connection at a time with a listen backlog of 16,
allocating at most one 1 MiB request and at most four decoded fields. Each
connection carries one request and one response, then closes. The deadline is
absolute across reads and writes, including partial reads, so a slow sender
cannot extend it by sending occasional bytes. SIGTERM/SIGINT stops acceptance,
finishes the active bounded transaction, and removes only this listener's
socket inode. Filesystem fsync duration is not forcibly bounded. A crashed
listener's socket is removed on startup only if it is owned, protected, and
connection-refused; a live or untrusted endpoint is refused.

## Provisioning helpers and chain evidence

`provision-owner` needs `STATE_ROOT` and `RECOVERY_POLICY_FILE`, takes the same
exclusive state lock as the listener, and accepts `email`, `display_name`,
`idempotency_key` and `now`. Its JSON response contains `principal`, `did`,
`recovery_root`, `recovery_threshold` and `recovery_delay_seconds`. Stop the
listener before using this command.

Example `owner-request.json` for local identity creation:

```json
{"email":"owner@example.com","display_name":"Owner","idempotency_key":"owner-install-1","now":1700000000}
```

Run the administrative command as the state owner while the listener is stopped:

```sh
services/human/target/debug/layerx-human-identity-provider provision-owner < owner-request.json > owner-result.json
```

This creates a local principal and directory DID. Merged registration binds that
principal to the exported signing authority DID; do not pass the local directory
DID as an exported chain authority. `provision-account` accepts at most 16384
bytes of JSON with only `did`, whose `did:layerx:` suffix must be exactly 64
lowercase hex characters and nonzero. It returns `{"account":"<hex>"}` for
`agent:<did>:main`. Owner limits instead use the distinct asset account derived
from `agent:<did>:asset:<asset_id>`. Neither derivation registers an account.

The committed-chain producer is the configured hosted gateway's `px_getAccount`
join ([gateway account producer](../../../platform/hosted/gateway/src/paxeer.rs)),
together with its merged-node `eth_*` transport. Public node RPC alone need not
serve `px_getAccount`. In
[hosted provisioning](../../../platform/hosted/human/provision.sh), owner
identity creation precedes `--produce-owner-registration`, then
`--validate-owner-registration`, then `human_account_head_observe`. That last
step invokes the current [validator](src/provision_head.rs), after registration,
before publishing limits or an owner configuration.

Example `account-observation-request.json` (replace the endpoint, CA and identity
fields with the protected deployment inputs; the identity values below are
illustrative):

```json
{
  "rpc_url": "<configured HTTPS gateway origin>",
  "rpc_ca_der_hex": ["<lowercase DER CA certificate hex>"],
  "chain_id": 125,
  "owner_did": "did:layerx:6161616161616161616161616161616161616161616161616161616161616161",
  "owner_account": "d3513012c0fe14fbbbfe2fd1e506a56b90d58ae19c70f6e7acebde2c3047706c",
  "asset_id": "3333333333333333333333333333333333333333333333333333333333333333",
  "maximum_age_seconds": 60
}
```

```sh
sh tools/runtime/run-with-clock.sh services/human/target/debug/layerx-human-identity-provider validate-account-head < account-observation-request.json > account-observation.json
```

The input is bounded to 1 MiB and rejects unknown fields. Production transport
requires HTTPS with explicit DER CA trust anchors; HTTP is restricted to a local
loopback node. The configured endpoint authenticates the observations: this is
not an offline consensus proof verifier. `chain_id` must match the deployment
(125 on the merged network); `maximum_age_seconds` is 1–300. The CLI samples Unix
milliseconds from the authenticated runtime-clock supervisor. Callers cannot override this time with a request timestamp.

Before publishing an owner configuration, require all of these observations:

- A nonzero committed height, block hash and state root; the block timestamp
  must be nonzero, no later than the trusted current time, and within the age
  limit. The endpoint must still report the same head, block and root when
  validation finishes.
- A bound `px_getAccount` join with the exact exported owner DID, canonical main
  account id and a nonzero EVM address matching the Paxeer account. The join
  must be unchanged on reread.
- At that committed height, address precompile `getEvmAddrByLayerX` must match
  the join, and ledger precompile `accountId` and `accountOwner` must match the
  requested asset account and canonical owner DID id. `balanceOf` must fit u128,
  and `nextSequence` must be zero.

Success emits the exact fields `schema` (`layerx.human.account-head.v1`),
`chain_id`, `owner_did`, `account`, `asset_id`, `committed_height`, `block_hash`,
`state_root`, `observed_at` (the committed block timestamp in Unix milliseconds),
`balance` (decimal string) and
`consumed` (zero). The hosted consumer retains the full observation, binds it
to the registration/deployment inputs and projects the verified `consumed`
value into its existing assembler input. Retain the full evidence when
publishing configuration; local provisioning or a submitted registration
transaction alone is insufficient.

Later committed heights are supported, including after restart. A nonzero debit
sequence is unsupported history and refuses without output: a later height or
current balance cannot establish unused limits. The retired protocol-3 signed
first-batch receipt envelope (`head`, `network_id`, `sequencer_id`, `public_key`)
is rejected by this command. Protocol-3 canonical account/DID domains remain in
use; historical receipt compatibility elsewhere cannot provision a current owner.

## LXIP v1 wire contract

All integers are unsigned, big endian. Both directions start with a u32 body
length, excluding that prefix. The body is:

```text
"LXIP" | version:u8=1 | operation-or-status:u8 | field-count:u32
       | (field-length:u32 | field-bytes)*
```

No padding, trailing body bytes, or alternate versions are accepted. Server
body bounds are 10–1048576 bytes, checked before allocation; field counts are
checked against the operation before allocating fields. The client permits a
configured maximum in 64–1048576 and checks both request and response against
that maximum. A too-small client maximum can reject an otherwise valid reply.
Text responses must be nonempty UTF-8 and at most 4096 bytes, then satisfy the
real typed constructors. Request text uses the same 4096-byte bound, refuses
control characters, and imposes the narrower account rules below.

| Operation | Request fields, in order | Success response fields, in order |
| --- | --- | --- |
| 0 `probe` | none | none |
| 1 `provision` | email UTF-8; display name UTF-8; idempotency key UTF-8; timestamp u64 (8 bytes) | principal UTF-8; DID bytes; recovery root (32 bytes); approval threshold u16 (2 bytes); challenge delay u64 (8 bytes) |
| 2 `resolve_email` | email UTF-8 | principal UTF-8 |
| 3 `device_for_assertion` | principal UTF-8; assertion ID UTF-8 | device ID UTF-8; label UTF-8; platform UTF-8 |
| 4 `assertion` | identity token UTF-8; optional producer-signed wallet binding (JWS) | principal UTF-8; DID UTF-8 when a wallet binding is recorded |

Status 0 means success; status 1 is refusal with zero fields. Operation 4
adds typed refusals with zero fields: 2 for a refused token, 3 for a wallet DID
bound to another account, 4 for a disabled principal or unavailable key set.
The client maps status 4 to `ProviderUnavailable` and every other nonzero
status to `ProviderRefused`. Bad framing/inputs, missing bindings, idempotency conflicts, unknown
operations and capacity limits receive status 1 if the connection remains
writable within its deadline. Wrong-UID peers are disconnected before reading
a frame. I/O failure or timeout appears to the client as
`ProviderUnavailable`; malformed success evidence appears as
`ProviderEvidence`; peer mismatch appears as `ProviderAuthentication`.
Persistence/entropy failures stop the server, withdrawing readiness; no success
is sent for an uncommitted mutation.

Provisioning requires canonical lowercase ASCII email, maximum 256 bytes,
nonempty local/domain parts and a dotted domain, with no whitespace or controls.
Display names are bounded to 256 bytes and validated by `AccountIdentity`.
A new principal uses 256 bits from the operating system RNG, rendered as
`act_` plus lowercase hex, and validated by `PrincipalId`. The DID is
`did:layerx:` plus that principal, validated by `Did`. Email and idempotency
bindings are unique. An exact retry returns the same account and original
policy; timestamp may change on retry, but email/display name may not. A new
key cannot take over an existing email. The client constructs its onboarding
idempotency digest with SHA-256 of the original key.

## Identity assertions

Operation 4 is served only when the assertion section is configured. The
section is optional but all-or-nothing:

| Variable | Meaning |
| --- | --- |
| `LAYERX_HUMAN_IDENTITY_PROVIDER_ASSERTION_JWKS_URL` | Key set of the token issuer |
| `LAYERX_HUMAN_IDENTITY_PROVIDER_ASSERTION_ISSUER` | Required token issuer |
| `LAYERX_HUMAN_IDENTITY_PROVIDER_ASSERTION_AUDIENCE` | Required token audience |
| `LAYERX_HUMAN_IDENTITY_PROVIDER_ASSERTION_CLOCK_SKEW_SECONDS` | Allowed clock skew; default 60, at most 300 |
| `LAYERX_HUMAN_IDENTITY_PROVIDER_ASSERTION_REFRESH_INTERVAL_SECONDS` | Key set refresh interval; default 300, at most 3600 |
| `LAYERX_HUMAN_IDENTITY_PROVIDER_ASSERTION_BINDING_PRODUCER_KEY` | ES256 (P-256) public key, SEC1 hex, that signs wallet bindings |

A verified token opens or resumes the principal for its issuer and subject. The
second field, when present, is a wallet binding signed by the producer key,
never caller-selected DID text.

`LAYERX_HUMAN_IDENTITY_PROVIDER_BINDING_SOCKET`,
`LAYERX_HUMAN_IDENTITY_PROVIDER_BINDING_TENANT` and
`LAYERX_HUMAN_IDENTITY_PROVIDER_BINDING_ALLOWED_UIDS` (comma-separated, at most
16) together open an optional read-only socket that answers whether a principal
of that tenant has a recorded wallet DID. Setting any of them requires all three.

## Trusted device enrollment

LXIP has no device enrollment operation or device metadata fields. Operation 3
is strictly a persisted principal/assertion lookup, never an attestation of
arbitrary user text. Unknown assertions and cross-principal lookups are refused.

Stop the server and run the binary as its state owner with `bind-device`, using
the same state/policy environment. Send at most 16384 bytes of JSON on stdin:
`principal`, `assertion_id`, and `device` containing `id`, `label`, `platform`.
These values must come from the trusted assertion enrollment authority. The
operator must verify that binding before import. Device fields are validated
with the real `Device` type; an existing assertion cannot change principal or
device. Exact repeated enrollment is idempotent. The library exposes the same
operation as `State::bind_device`. Exclusive state locking prevents concurrent
imports while serving. No enrollment endpoint is exposed to LXIP callers.

The provider's integration suites cover the service client, wire validation,
protected storage and listener lifecycle. [Shared test support](../layerx-human-test-support/README.md)
distinguishes local admission evidence from committed chain evidence.

## Durability and limits

`writer.lock` holds an exclusive OS file lock for the full state lifetime.
A durable 0600 `initialized` marker prevents a missing committed snapshot from
being mistaken for a new directory after identities have been acknowledged.
`state.json` is a versioned, SHA-256 checked snapshot containing account/email/key
bindings, original policies and assertion/device bindings. State files are
0600, regular, owned by the effective UID, single-link, and opened with
O_NOFOLLOW/O_NONBLOCK. Symlink directory components and unsafe ancestor owners
or writable non-sticky ancestors are refused. The dedicated root is 0700;
state is access-protected, not encrypted at rest.

Each mutation writes a new 0600 `state.pending` with create-new, fsyncs it,
atomically renames it to `state.json`, and fsyncs the directory before updating
the in-memory snapshot or replying. Startup validates the committed checksum,
version, all typed values, uniqueness, and assertion references before serving.
It then discards an owner-protected uncommitted pending file. A crash before
rename replays the previous snapshot; a crash after rename replays the complete
new snapshot. An interrupted response is safely retried by idempotency key.
A malformed committed file is never reset to empty. Readiness is published only
after this replay and any initial empty-state commit succeed. Each request
rechecks the committed file digest and held directory/lock identities; detected
corruption or replacement withdraws the listener. Failed writes poison that
state instance until it is reopened and replayed.

The snapshot is capped at 16 MiB, 10000 accounts and 20000 assertion bindings.
Capacity exhaustion refuses further mutations without evicting identities.
Device bindings do not expire automatically; their lifecycle is the enrollment
authority's responsibility. Backups must preserve ownership and modes.
