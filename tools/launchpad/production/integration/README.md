# Independent integration assurance primitives

Run commands from the repository root. The local phase-1 tests use the Python
standard library and do not import product reducers:

```sh
python3 -m unittest discover \
  -s tools/launchpad/production/integration -p 'test_integration.py' -v
```

This selects the local primitives only. Broad `test_*.py` discovery also executes
phase-2, phase-3 and phase-4 runtime gates, which require explicitly admitted
services, configurations and private output directories. Missing prerequisites
fail those gates; they are not skipped or replaced with synthetic providers.
Arithmetic inputs and hashes in local tests are synthetic, never chain receipt
evidence. HTTP tests use a bounded loopback server; process cleanup uses a
directly owned process group. Local Docker boundary tests do not fault a running
container.

## Independent models

`oracle.inventory` requires complete contiguous block observations, including
empty blocks, anchored to an independently supplied start height and parent hash.
It checks chain-125 event/header identity, global log ordering, complete-block
agreement and transport duplicate equality. Obtain and authenticate raw node
receipts, deployed ABI/code provenance and complete log ranges independently;
normalized models cannot authenticate their own inputs or establish activation.
Paxeer is one network: its kernel participates in block proposal and block commit
establishes finality. Disagreement quarantines the stream; ordinary committed
blocks are not rolled back. Public JSON-RPC uses node root URLs such as
`https://api1.mainnet-beta.paxeer.network/` with chain ID 125 (`0x7d`).

`Ledger` uses remaining weighted cost, integer-floor partial allocation and the
full residual cost on complete disposal. Quantities and prices are raw token and
raw USDL units. Buy inputs must be actual net tokens received and total USDL
debited; sell inputs must be actual inventory removed and net USDL credited.
Pool execution volume and fee-inclusive wallet cashflow are distinct facts.
The buy recipient owns new inventory. A sell needs an explicit `inventory_owner`,
independent of the proceeds recipient; never infer it from the transaction sender.
Realized proceeds belong to the disposed position for economic accounting even
when another beneficiary receives the cash. Unknown transfer basis blocks
disposal attribution until authoritative lot selection is supplied.
`unrealized` accepts an exact `Fraction` of raw USDL per raw token; convert human
prices by multiplying by `10^6 / 10^tokenDecimals`. This is an assurance model,
not a replacement PnL service.

`candles` consumes independently normalized execution amounts, sorts canonical
height/transaction/global-log positions and uses exact `Fraction` human USDL/token
ratios across seven UTC intervals, with weeks starting Monday. Empty buckets
are omitted. Invoke inventory before either oracle: events alone cannot prove
completeness. These models do not prove database commit, revisions, outbox
publication or runtime coverage; integrated outputs and durable fault tests do.

## Local adapters and recovery authority

`LocalHTTP` requires a numeric loopback HTTP origin on an explicitly approved
port, bounds request/response sizes, and neither follows redirects nor resolves
hostnames. `OwnedProcess` controls only the child and process group it starts,
terminating then killing the group if needed. Its executable must be an absolute
path. Use reviewed arguments, require service startup to fail on occupied ports,
and check child liveness before fixture mutation; the adapter does not reserve
ports.

`FaultPermit`/`DockerFaults` permit only exact 64-character container IDs with
coordinator-established namespace, attempt, role and isolation labels. They deny
privileged containers, host network/PID namespaces and mounted containers. Actions
are allowlisted per target: `pause`, `unpause`, `stop`, `start`. There is no shell,
exec, deletion, pruning or shared Compose reset API.

The implemented `RecoveryPermit`/`RecoveryFaults` extension supports persistent
fault tests on exact owned named volumes. Each permit binds container ID,
namespace, attempt, role, failure domain, volume names and actions. Inspection
requires matching `kindle.assurance.*` labels on both container and volumes,
exact mount inventory, local volume drivers and no volume options. Bind mounts,
foreign volumes, host namespaces, privileged containers, devices and added
capabilities are rejected. Container and volume ownership is rechecked before
and after an action, and the resulting running/paused state must match it.
The coordinator supplies an absolute Docker executable. This authority never
extends to existing shared bootstrap resources merely because their names match.

## Environment-bound phases

The executable tests below run their phase's boundaries and actual runtime journey.
For each phase N, set `PHASEN_CONFIG_FILE` to the reviewed configuration file,
`PHASEN_CONFIG_SHA256` to its SHA-256 and `PHASEN_OUTPUT_DIR` to a private owned
output location. The actual variable names are listed explicitly here:

| Phase | Required configuration variables | Command |
| --- | --- | --- |
| 2 | `PHASE2_CONFIG_FILE`, `PHASE2_CONFIG_SHA256`, `PHASE2_OUTPUT_DIR` | `python3 -m unittest discover -s tools/launchpad/production/integration -p 'test_phase2.py' -v` |
| 3 | `PHASE3_CONFIG_FILE`, `PHASE3_CONFIG_SHA256`, `PHASE3_OUTPUT_DIR` | `python3 -m unittest discover -s tools/launchpad/production/integration -p 'test_phase3.py' -v` |
| 4 | `PHASE4_CONFIG_FILE`, `PHASE4_CONFIG_SHA256`, `PHASE4_OUTPUT_DIR` | `python3 -m unittest discover -s tools/launchpad/production/integration -p 'test_phase4.py' -v` |

All three also require `QUALIFICATION_CANDIDATE_SHA256`. The configuration loader
must verify the configuration digest and current candidate binding. Use the
phase-specific scenario manifests and evidence policies beside this page to
review inputs; file presence alone is not a runtime admission or pass.

- Phase 2 calls `Journey` in `journeys.py` for current services, browser actions,
  real receipts and persisted effects. It requires admitted runtime authority,
  frozen interfaces and actual database/transport resources.
- Phase 3 calls `Phase3Journey` for current-candidate faults and security cases,
  with prior phase admission, exact runtime/topology identity and isolated fault
  permits, including the named-volume extension where persistence is involved.
- Phase 4 calls `Phase4Recovery` in `replay_restore.py` for replay, point-in-time
  restore and regressions. It needs prior qualification, actual PITR/broker and
  generator authority, recovery targets and timestamp/checksum evidence. Its
  isolated proof does not establish an actual elapsed 72-hour replay or the final
  integrated gate.

## Qualification limits

[scenario-inventory.json](scenario-inventory.json) records production scenarios
as blocked. Implemented harnesses and phase-specific results do not promote
that inventory to production qualification. Full proof still needs integrated
services/browser, real isolated receipts bound to deployed ABIs, transactional
PostgreSQL/outbox/inbox writers, dedicated NATS R3 and failover resources,
cross-region restore targets, scanner/object resources, actual elapsed replay,
mutation copies and coverage profiles. Capacity, 500,000 DAU, 10,000 events/sec,
AZ/region recovery and full journeys require their own declared evidence.
No core oracle, graduation, ordinary reorg or embedded wallet assumptions follow
from these primitives.

See the [bootstrap baseline](../bootstrap/README.md),
[security prerequisite map](../bootstrap/security/GAPS.md),
[dispatcher](../dispatcher/README.md) and [load phases](../load/README.md).
