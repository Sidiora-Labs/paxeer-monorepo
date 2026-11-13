# Production operations

For the architecture and fee model, see the [launchpad overview](../README.md); for central event
payloads and authorization, see [IEventEmitter](../docs/interfaces/IEventEmitter.md).

This runbook describes deployment, incident controls, oracle publication and read-only inspection of
the launchpad contracts.

## Launchpad mainnet deployment

Core deployment defaults to `GOVERNANCE_MODE=admin`: the configured administrator queues changes
through Timelock. This mode does not require or read `SID_ADDRESS`. `GOVERNANCE_ADMIN_ADDRESS`
selects that administrator or multisig and defaults to the deployment wallet. The administrator
retains `ADMIN_ROLE` on GovernanceModule; Timelock holds `DEFAULT_ADMIN_ROLE` on the proxies and
remains the only path for delayed protocol configuration and upgrades. The Timelock proposer is
GovernanceModule.

Supply `PRIVATE_KEY` and the existing six-decimal mainnet `USDL_ADDRESS` in the deployment
environment. Set `GUARDIAN_ADDRESS` to the intended Timelock cancellation guardian, or it defaults
to the deployment wallet. `TIMELOCK_DELAY` defaults to 172800 seconds and must be an integer
from 3600 through 2592000 seconds. Leave `ADMIN_ADDRESS` unset:
proxy administration is assigned to Timelock. Shell-exported variables override `.env`, so replace
any previously exported local token address before running on mainnet.

Run all commands in this runbook from the launchpad project, `contracts/launchpad/`, which contains
its own `package.json`, `hardhat.config.js` and `scripts/`. From the repository root, enter it with:

```sh
cd contracts/launchpad
```

The configured mainnet network in [hardhat.config.js](../hardhat.config.js) is `paxeer-x-network`
(chain ID 125 / `0x7d`). It uses node JSON-RPC at the configured HTTPS origin’s root path. These
contracts run on the single Paxeer BFT network; its kernel proposes blocks, and receipts are final
when their blocks commit.

With the deployment environment loaded, run:

```sh
NETWORK_TYPE=paxeer-x-network GOVERNANCE_MODE=admin \
node node_modules/hardhat/internal/cli/cli.js run scripts/deploy.js --network paxeer-x-network
```

This deploys the launchpad core, with a fresh pool creation fee of one USDL plus network gas. It
does not deploy the optional PECOR/oracle layer. The local token helper is unnecessary for mainnet.

Deployment records the selected mode and administrator in
`deployments/paxeer-x-network-addresses.json`. Complete the queued beacon `acceptOwnership`
operation after `_governance.beaconAcceptance.eta` using the recorded target/value/data/eta tuple
with `Timelock.executeTransaction`; then rerun the identical core command with
`RESUME_DEPLOYMENT=true` to reconcile final ownership status. Changing the mode, administrator,
signer or token addresses is a new deployment configuration, not a compatible resume. Preserve
partial manifests and their operations journal before planning a replacement; do not edit their
identity hashes or delete records to bypass recovery checks.

`GovernanceModule.initializeAdmin` sets `adminOnlyMode` and keeps `adminModeActive`. `propose` and
`deactivateAdminMode` reject administrator-only mode so it cannot accidentally strand its control
path. `adminExecute` requires `ADMIN_ROLE`; its queued transaction still waits for the configured
Timelock delay. Administrator rotation uses delayed `grantRole` and `revokeRole` calls through
Timelock. The alternate `GOVERNANCE_MODE=token` uses the existing `initialize` path and requires a
SID token with historical voting checkpoints; it disables bootstrap administrator decisions at
handoff.

## Verify the launchpad on Paxscan

Use the same `paxeer-x-network` name as deployment. From `contracts/launchpad/`:

```sh
NETWORK_TYPE=paxeer-x-network \
node node_modules/hardhat/internal/cli/cli.js run --no-compile scripts/verify.js --network paxeer-x-network
```

The `verify:paxeer` package command uses this network too. Preserve the deployment artifacts and
their linked build-info files: `--no-compile` prevents verification from replacing the build used to
deploy. The verifier's `main` reads confirmed creation records from
`deployments/paxeer-x-network-addresses.json`, checks each artifact and encoded constructor against
its recorded creation hash, and compares mainnet runtime hashes before submitting any source. A
mismatch stops the run; recover the matching artifacts rather than editing journal hashes.
`VERIFY_DRY_RUN=true` performs these checks without publishing.

Verification publishes compiler standard JSON to the explorer; it does not send blockchain
transactions or modify deployment journals. `deployments/paxeer-x-network-verification.json` records
per-contract results. An accepted request remains pending until the explorer reports
`is_verified=true`; pending and failed results produce exit code 1. Rerun after pending jobs
complete; already verified addresses skip submission. Beacon ownership acceptance remains a separate
delayed operation.

The native v2 API does not expose a job failure through the address flag: an unconfirmed result can
also mean the worker rejected verification. For persistent pending results, inspect Paxscan's
address-channel `verification_result` event or its verification worker logs. An explorer error does
not call for redeploying the contracts. Keep the matching source artifacts and deployment records
while diagnosing the service.

## Incident containment and recovery

`EmergencyPauseController` gives `INCIDENT_GUARDIAN_ROLE` holders one capability: call `pause()` on
allowlisted targets. It accepts at most 32 targets per call, bounds external-call gas and return
data, checks the resulting pause state, and emits `IncidentTargetResult` for every requested target.
Target and guardian administration belong to Timelock. The controller has no withdrawal, upgrade or
unpause entry point.

Layered deployment creates the controller and grants it `PAUSER_ROLE` on `PECOR`, `PECOROrders`,
`MetaAGRouter`, `PriceOracle` and `OracleHub` before removing bootstrap administration. Set
`INCIDENT_GUARDIAN_ADDRESS` to the intended incident responder. Without an override, deployment
validates the inherited core guardian against the actual Timelock; where no guardian is recorded it
uses Timelock itself. That fallback provides no independent immediate response. Guardian identity is
part of the deployment resume configuration.

For a trading incident, submit `pauseTargets(targets, incidentId)` with all five configured endpoint
addresses from the reviewed deployment manifest. A five-target call needs approximately 1.5 million
gas as a transaction limit; the contract requires `250000 * targetCount + 50000` gas remaining at
its body, plus transaction and authorization overhead. Measure suitability against the actual
chain's block and transaction limits.

Inspect every result: enum values are `Paused=0`, `AlreadyPaused=1`, `NotAllowed=2`,
`InvalidStatus=3`, `CallFailed=4`, `PostconditionFailed=5`. A successful batch transaction does not
mean every endpoint was paused. Reconcile failures and verify current endpoint state before
declaring containment. `emergencyPaused()` exposes each endpoint's local flag.

PriceOracle pause stops publications while fresh accepted prices remain readable. Pausing only that
source does not immediately stop consumers. Include the execution endpoints in the incident
transaction. OracleHub pause stops its reads; other independent oracle consumers are outside this
controller's scope.

Order owners can still cancel and recover escrow while execution is paused. Expired orders can still
be refunded. Vault custody has no global pause: quarantine blocks new exposure for an asset while
reservation release and supported refund transfers remain possible. Taxed or reverting outgoing
tokens may still prevent refunds; pause does not remove token-level behavior or custody deficits.
Core AMM pools retain their existing creator/guardian model and are not automatically controlled by
this controller.

Recovery requires governed `unpause()` calls. Incident responders cannot unpause, change fees,
rotate administrators or upgrade contracts by virtue of their pause role. Review the cause, custody
backing, source freshness and all target results before proposing recovery.

## Oracle publication and readiness

`setMaxPriceJumpBps(token, limitBps)` is admin-only. Zero disables the additional guard; values up
to 10000 constrain each accepted update relative to the last accepted price. Existing
`deviationThresholdBps` remains publication-trigger metadata. An enabled jump guard is enforced in
addition to absolute bounds, before price/history changes, and a rejected batch rolls back every
update. The first valid publication is allowed. A stale previous price remains the anchor: an
operator must obtain a governed limit change for recovery rather than wait out the control.

This is a per-update limit, not a cumulative drift or independent fair-price guarantee. A trusted
relayer can still move prices gradually within configured limits. Select parameters from the asset's
reviewed operating policy; the deployment default does not silently impose a new threshold.

`getPriceStatus(token, period)` reports registration, publication pause, current
price/round/timestamp, freshness, heartbeat delay, retained observation count and earliest
timestamp, and requested-window temporal readiness. `twapReady` indicates fresh retained coverage,
not an independent price-quality guarantee. `registeredTokenCount()` and
`getRegisteredTokensPage(cursor, limit)` support discovery; pages cap at 100. Existing whole-array
and executable price getters remain available.

## Optical review identity

Optical registration records the observed direct runtime hash and flags. Initial registration has a
zero external evidence digest. `updateMetadata` changes descriptive fields without renewing
executable approval. To renew review, an administrator calls:

```text
renewReview(optical, expectedCodeHash, expectedFlags, evidenceDigest)
```

Use the reviewed runtime identity and a nonzero digest of the retained review evidence. Renewal
validates and records one flags observation. Approval queries return false if runtime identity or
flags change, or if a flags probe fails or returns invalid data.

The registry is optional trust signaling. Deregistration does not disable attached swap, fee or
transfer hooks. Do not skip vesting restrictions in an attempt to bypass a revoked approval. Direct
runtime identity cannot identify a proxy's changing implementation or arbitrary mutable behavior.
Review known proxy storage and actual implementation bytecode separately and invalidate/review when
that identity changes. A self-reported `implementation()` value is insufficient for an arbitrary
proxy.

## Read-only health inspection

Run from `contracts/launchpad/`, with a deployment manifest and `RPC_URL` set to a root-path
JSON-RPC provider URL:

```sh
node scripts/production-health.js deployments/paxeer-x-network-addresses.json policy.json
```

The optional second file selects policy assertions. The command does not load Hardhat, require a
private key, or submit transactions. It emits JSON and exits 1 for critical or incomplete
inspection; warnings exit 0 and must still be interpreted. RPC exception details and endpoint
credentials are suppressed.

Reads use one block number and recheck its hash at the end. The report includes the chain, block
hash and timestamp, pinned deployment runtime identities, supported EIP-1967 implementation
pointers, beacon identity, vault inventory, raw custody/book/reservations and actual decimals,
migration readiness, oracle freshness and pause flags. Donations are distinguished from deficits.
Required RPC failures, malformed responses and incomplete inventories are critical. An unsupported
proxy type cannot receive implementation qualification.

By default, all six execution/custody/oracle endpoints are required: Vault, Orders, PriceOracle,
PECOR, MetaAGRouter and OracleHub. An explicit `requiredContracts` list creates a labeled partial
inspection. Set `requiredAssets` to the inventory that must exist, `maxTokens` to a bounded
inventory limit, and `pausedIsCritical` according to incident policy. Add `expectedRoles` assertions
with `contract` (manifest key), `role` (bytes32), `account`, and `present` (boolean) to check both
required authority and revoked bootstrap authority. Missing role policy remains an explicit warning.
The tool does not enumerate all historical role holders or establish that a responder address is a
correctly operated multisig.

The `scripts/meta-ag/inspect-live-state.js` entry point delegates to this same inspector. Retain
JSON snapshots for monitoring and investigate changes; a healthy snapshot is not an execution
simulation, oracle economic analysis or production migration certificate.

## Release qualification

From `contracts/launchpad/`, use the package’s pinned pnpm version and supported Node version from
[package.json](../package.json):

```sh
pnpm install --frozen-lockfile
node scripts/release-qualify.js
```

The gate forces compilation, runs JavaScript lint, verifies artifact bytecode/ABI and transitive
compiler source inputs, checks the 24,576-byte deployed runtime limit, compares 22 frozen production
storage layouts, and runs the complete local Hardhat suite. It invokes Hardhat directly to avoid
package-manager command dispatch. Source and compiled-artifact hashes must remain unchanged during
qualification; the build report and logs are hashed. Gas snapshot updates and documentation
generation are disabled during the run.

Each execution writes a distinct run directory under `.release/`; `.release/qualification.json`
identifies its exact evidence and begins with a `running` state. A failed run cannot inherit an
earlier passing result. Use `--output DIRECTORY` to archive elsewhere. The output directory must be
outside contract, test, script and storage-baseline inputs.

PECOR stress observations are retained in the current run directory and hashed in its qualification
record. Standalone test runs write temporary observations by default; `PECOR_COVERAGE_OUTPUT`
explicitly selects an output file. Each qualification record binds its own observations.

`node scripts/release-check.js --report FILE` performs only the compiled artifact/layout checks; it
does not compile, install dependencies or execute tests. Storage layouts are retained under
`storage-layout/core`, `storage-layout/meta-ag` and `storage-layout/production`. The production
manifest binds its compiler-derived layout files.

## Design references

- [OpenZeppelin access control](https://docs.openzeppelin.com/contracts/5.x/access-control):
  separate limited operational roles from administration and delayed recovery.
- [ERC-1967](https://eips.ethereum.org/EIPS/eip-1967): standard implementation and beacon storage
  identity for supported proxies.
- [EIP-170](https://eips.ethereum.org/EIPS/eip-170): deployed contract runtime size limit.
