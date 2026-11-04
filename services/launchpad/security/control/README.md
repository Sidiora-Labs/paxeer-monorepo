# Private control primitives

This Go module implements durable operator actions, capability admission,
disclosed recommendations, funded bounties, settlement dispatch and incident
recovery. [providers.go](providers.go), [workers.go](workers.go) and
[http.go](http.go) provide runtime, provider and HTTP adapters. The connected
[gateway](../../gateway/internal/app/app.go) constructs the control runtime,
checks readiness, mounts its handler and runs its configured workers.
Implementation and fixture coverage do not establish production payout authority
or launch qualification.

## Authority and storage boundaries

`Identity.Resolve` validates an active credential and returns its canonical
principal, role and exact scopes. `Identity.Active` compares the current identity
snapshot, including credential and session identity. Client-asserted roles do
not establish authority. `NewRuntime` uses `NewPostgresIdentityV2`, reading the
narrow `authority_credentials_v2` and `identity_control_wallets_v2` views through
a separate identity pool. The legacy `NewPostgresIdentity` adapter also exists;
it is not the runtime constructor's identity path.

`Runtime.Ready` requires distinct `control_runtime` and `identity_runtime` roles
without superuser, role-creation, database-creation, RLS-bypass or schema-owner
authority. It checks that the identity views are readable and the control
credential base table is inaccessible to the identity role. `Install` is a
migration constructor, not a public HTTP route.

[store.go](store.go) serializes mutations with a schema-specific transaction
advisory lock. Each mutation has a five-second context bound, a two-second lock
timeout, a four-second statement timeout and a one-second rollback cleanup.
Identity is checked again after acquiring the lock and after verification
callbacks before a successful commit. Domain faults roll back a savepoint and
commit the denial audit; unexpected database or audit errors roll back the
transaction. Audit UPDATE, DELETE and TRUNCATE are rejected by triggers. Runtime
roles must not own the objects or be able to disable their triggers. The single
lock targets low-volume controls; it is not a throughput qualification.

Reasons are nonempty UTF-8, at most 1000 bytes, without CR, LF or NUL. Credentials
are not persisted in domain objects or audit. Audit records actor, role,
operation, resource scope, reason and result; callers must keep secrets out of
reasons. Inventory and caps require exact `platform` scope. Actions, incidents,
bounties and obligations retain their canonical resource scope. Automation also
requires the owner to equal the actor. Unknown roles, scopes and privileged
datasets deny by default. Infrastructure does not inherit finance, support or
moderator permissions; moderator inventory changes and support exports of
private messages or payment secrets are denied and audited.

## Durable action and caps semantics

[operator.go](operator.go) binds proposals to kind, scope, reason, proposer,
payload SHA256, expiry and idempotency key. Conflicting reuse fails. Transitions
check an expected uint64 revision and reject overflow. Approval requires a
distinct operator whose authority is revalidated at execution. Approved replay,
redrive or export execution creates one durable job in the same transaction;
retries return that job even after a terminal result.

`CompleteAction` requires a read-only `JobResultVerifier` proof before recording
success or failure. [workflows.go](workflows.go) implements `DurableJobResults`
over persisted executor results and validates replay proposals. A queued job is
not evidence that an indexer or export executor performed the effect; that owner
must supply its idempotent inbox, effect/checkpoint transaction and result proof.

Caps cover `pool_creation`, `quote`, `socket` and `paid_features`. Increasing a
quota, extending expiry, advancing effective time or reactivating expired
positive authority requires a matching, one-use, dual-approved `cap_change`.
A single operator can disable authority with zero quota or narrow both its quota
and active interval. Version, effective time, expiry and revision are persisted.
Admission rereads PostgreSQL and denies unavailable, unknown, future or expired
state. `Runtime.RequireCapability` exposes middleware; the gateway wires it for
socket admission. Consumers still own numeric enforcement and pre-wallet UI
warnings, and read-only market access must remain available. These primitives
alone do not prove a complete frontend journey or propagation within 60 seconds.

## Disclosed automation

[automation.go](automation.go) implements recommendation mode
`disclosed_assistant`. Policies begin in draft and require explicit enable;
they bind owner, disclosure, asset, instruments, quote age, expiry and request
and spend ceilings. Reservations carry amount zero, suggestion state and
disclosure. Request budgets are reserved atomically per owner and asset across
policies; updating budgets preserves usage. New policies cannot reset an owner's
budget. Quote age, allowlist, state and policy/owner expiry are checked under
lock. Stopping or revoking a policy blocks new reservations and preserves history;
reusing a reservation key returns its original result.

Recommendations reserve no money and do not authorize signing. Delegated
execution returns `authority_unavailable`; no server-held user signing keys are
supported. `PublicCampaign` keeps recommendation opt-in distinct from execution:
an internally enabled recommendation policy has public state `blocked`, retains
its `recommendationState` and reports `delegated_execution_unavailable`.
Discovery consumers must preserve the disclosure and exclude sponsored activity
from organic ranking signals.

## Funded bounties and external inventory

Amounts are canonical nonnegative uint256 decimal strings, with no cross-asset
sums. `Asset.Valid` accepts native PAX on chain 125 with 18 decimals, or USDL at
`0x85fcd13735f4309833a503ee804ea32395851479` with 6 decimals. The settlement
capability response reports a creation protocol fee of `1000000` raw USDL and
`gasIncluded=false`; it does not change the factory fee or its deployment.

[rewards.go](rewards.go) persists bounty rules, reward asset/amount, custody
funding source, deadline and published refund disposition. Funding requires
independently observed inventory with matching custody, fresh within 60 seconds,
and sufficient atomically unreserved balance. Reserving the full liability
precedes active publication: two 80-unit reservations against 100 units admit
one. Expired drafts cannot reserve. Cancellation and expiry release applicable
liability and retain reversal history; they do not manufacture refund receipts.
Award requires a trusted `AwardAuthority` binding the bounty, evidence and exact
beneficiary. `StoredAwardAuthority` checks the stored submission and verified
wallet ownership. Award creates an obligation and durable settlement outbox.

`SettlementWorker.Process` does submit payments through `PayoutProvider`.
It claims the durable obligation, checks its authority reference and looks up
the same idempotency key before dispatch. Only a provider lookup reporting
`absent` allows a PUT to `/v1/settlements/{key}`; GET uses the same path and both
carry `Idempotency-Key`. Dispatch intent is persisted before submission.
Pending or confirmed responses record provider acceptance separately from intent.
Timeouts and ambiguous responses preserve an `unknown` obligation; redelivery
reconciles the same key. A provider's `confirmed` response is not independent
chain settlement proof. Safe retries require that provider's durable key and
lookup semantics, not an assumption that a timed-out payment failed.

[chain.go](chain.go) supplies the read-only `ChainVerifier`. Each read checks
current manifest/custody authority and chain ID 125. Inventory reads verify the
asset and committed-block balance; settlement checks a successful receipt,
amount, recipient, custody source and canonical block identity. On Paxeer the
node-reported block hash is authoritative; recomputing an Ethereum header hash
does not identify the CometBFT block. Native settlement verifies the transaction's
chain and sender; USDL requires the exact custody-to-beneficiary Transfer log.
A unique receipt can settle only one obligation.

Paxeer is one BFT network whose kernel proposes blocks as part of the network.
Receipts are final once their blocks commit; there is no additional execution
or periodic settlement chain. Public node JSON-RPC is at the root path, such as
`https://api1.mainnet-beta.paxeer.network/`, with chain ID 125 (`0x7d`); see the
[public RPC reference](../../../../docs/site/docs/reference/public-rpc.md).
Private Anvil fixtures prove adapter behavior, not Paxeer consensus or current
mainnet custody authority.

Inventory drift freezes new funded/awarded obligations while retaining visible
liabilities. Native payout gas can itself cause unexplained drift. Production
use still needs approved gas, deposit and reversal accounting, independently
trusted custody and reviewer policy, an admitted payout provider and operational
reconciliation/alerting. There is no manual bypass or unfreeze API.

## Incidents

[incident.go](incident.go) records evidence, quarantines the service identity and
writes a durable revocation/cache-invalidation request atomically. Further
privileged mutations for that identity are denied at the store. Public status
uses fixed nonsensitive text.

`IncidentWorker` drains contained incidents, revokes credentials in the control
store and persists progress. A configured `RevocationProvider` sends a bounded
HTTPS PUT to `/v1/identity-revocations/{incident}` and requires matching incident
and identity, rotation/integrity references, and both secret-revocation and
cache-invalidation acknowledgements before completing the outbox. Missing or
unavailable external authority leaves completion pending; local quarantine is
not evidence of external containment.

Recovery needs current dual approval bound to the exact incident, rotation and
integrity references, plus a trusted `RecoveryVerifier`. `IncidentRecovery`
checks the durable acknowledgement. An old recovery cannot clear a newer
quarantine. Production credential-compromise drills and a measured external
containment deadline remain separate qualification requirements.

## Frozen interface integration

[views.go](views.go) implements public campaign, bounty, asset, cap-warning and
error adapters; internal objects must not be serialized as frozen public DTOs.
`PublicBounty` maps funded to `funding`, active to `open`, and submitted,
adjudicating or awarded to `reviewing`. Only an awarded bounty with a matching
settled obligation and receipt reference becomes `paid`. Its public schema
requires an ERC20 address, so native rewards return unavailable rather than
masquerading as USDL. `wireAsset` represents native assets with null address,
chain ID string `"125"` and 18 decimals; USDL uses its canonical ERC20 address.

HTTP handlers expose caps, campaign/recommendation reads, settlement capability
and `POST /v2/control/{operation}`. Successful responses use `WorkflowEnvelope`;
errors use `WorkflowError`. `ErrorResponse` maps invalid input to HTTP 400
`CURSOR_INVALID`, denial to 403 `SCOPE_DENIED`, missing records to 404 `NOT_FOUND`,
conflicts to 409 `IDEMPOTENCY_CONFLICT`, and funding/budget/dependency failures
to 503 `DEPENDENCY_UNAVAILABLE`. Consult the handlers for authentication-specific
responses. Gateway authentication and owning read models supply their own
wrappers and identities.

## Runtime and qualification boundary

`NewRuntime` accepts separate control/identity pools, schema bindings, an
observer and optional verifier, payout/revocation adapters, credentials and
assets. It rejects incomplete authority combinations. `Run` checks readiness,
then runs configured reconciliation, incident and settlement workers;
reconciliation uses a 30-second interval. The current gateway configuration
wires identity/control pools and optional incident/revocation authority, but
supplies no payout adapter, evidence verifier or reconciliation assets.
Its settlement capability therefore remains unavailable. Implemented optional
adapters must not be advertised as a configured production treasury.

The private checker is invoked from the repository root with
`python3 services/launchpad/security/control/check.py`. It reconstructs an
isolated Go module from the shared manifests and uses readonly module resolution
and the installed dependency cache. It starts an owned chain-125 Anvil, reserves
its port and uses its private IPC socket for mutation. The Go fixtures start
an isolated PostgreSQL instance using local server binaries; root execution
requires the unprivileged `postgres` account. The checker also requires its
configured Anvil binary at `/root/.foundry/bin/anvil`. Fixture credentials and
local authorities are test data and never production attestation.

This documentation rewrite does not run that checker or certify a deployment.
Image and release authority are described in the
[supply-chain notes](../supplychain/README.md); the media service's qualification
boundary is recorded in [livestream progress](../../livestream/PROGRESS.md).
