# L48 workload primitives

Run commands from the repository root. Install the pinned Python dependencies
from the hashed [requirements.lock](requirements.lock) in the qualification
environment; frozen schema validation uses `jsonschema==4.25.1`:

```sh
python3 -m pip install --require-hashes -r tools/launchpad/production/load/requirements.lock
```

## Local workload tests

Select the local tests explicitly:

```sh
python3 -m unittest discover \
  -s tools/launchpad/production/load -p 'test_load.py' -v
```

Only this selection has the loopback-only transport boundary: it opens ports
24800/24801, failing on occupied ports rather than reusing peers. It collects
actual monotonic sender/receiver timestamps, retains every scheduled arrival,
and exercises a real two-second receiver pause and RFC6455 masked subscription.
The receiver is a calibration echo, not an application gateway or chain fixture.

Broad `test_*.py` discovery also includes live phase-2 through phase-4 provider
gates. Those require explicit runtime admission and source-bound configuration;
they fail when inputs are absent and must not be described as loopback-only.

## Accounting and calibration

`workload.ledger()` returns exact fraction/decimal-byte accounting for the shared
500,000-DAU model. Quotes and writes are included in total HTTP RPS. Arithmetic
does not establish measured capacity. `scheduler.run` schedules independently of
completions, retains timeout/missed/error arrivals, and bounds memory and
concurrent tasks. Latency starts at scheduled arrival; never subtract monotonic
clocks from different hosts. Missing arrivals remain explicit failures rather
than fabricated latency samples.

`calibration.evaluate` checks raw sample digest and counters, resource deltas,
expiry, four distinct host labels and candidate/topology/generator bindings. It
validates metrics; host identity and clock attestation require independently
authenticated host/clock/NIC telemetry. It always returns `capacity_qualified`
false. CPU readings are host aggregates, so shared hardware requires dedicated
generator/cgroup sampling. Measure NIC speed from actual hardware; loopback
cannot establish it. The local suite does not certify those prerequisites.

## Environment-bound workload phases

Provision the actual owned application, receiver, generator, browser, account and
chain resources required by the phase configuration before running these commands.
Each configuration file and SHA-256 must be reviewed; generator source identity
must match `source_digest()` in `phase2_types.py`. The binding set includes the
candidate, interface, topology and generator. Outputs belong in private owned
capture directories, not in the source tree as invented execution evidence.

| Phase | Required variables | Command |
| --- | --- | --- |
| 2 | `LOAD_PHASE2_CONFIG_FILE`, `LOAD_PHASE2_CONFIG_SHA256`, `LOAD_PHASE2_OUTPUT_DIR` | `python3 -m unittest discover -s tools/launchpad/production/load -p 'test_phase2_journeys.py' -v` |
| 3 | `LOAD_PHASE3_CONFIG_FILE`, `LOAD_PHASE3_CONFIG_SHA256`, `LOAD_PHASE3_OUTPUT_DIR` | `python3 -m unittest discover -s tools/launchpad/production/load -p 'test_phase3_faults.py' -v` |
| 4 | `LOAD_PHASE4_CONFIG_FILE`, `LOAD_PHASE4_CONFIG_SHA256`, `LOAD_PHASE4_OUTPUT_DIR` | `python3 -m unittest discover -s tools/launchpad/production/load -p 'test_phase4_capacity.py' -v` |

Every phase 2–4 configuration requires `contract_authority_path` and
`contract_authority_sha256`. The reviewed JSON document contains:

```json
{
  "bindings": {"candidate": "<sha256>", "interface": "<sha256>", "topology": "<sha256>", "generator": "<sha256>"},
  "schemas": {"path": "<reviewed-v2-schema-definitions.json>", "sha256": "<sha256>"},
  "openapi": {"path": "<reviewed-openapi-3.1.json>", "sha256": "<sha256>"}
}
```

All four bindings must equal the admitted run's bindings. Document hashes bind
exact bytes, duplicate JSON keys and remote references are refused, and request,
response and stream validation use that same schema authority. Paths may be
absolute private authority paths or relative to the repository root. The current
frozen v2 schema bundle is
`sdk/launchpad/packages/production-contracts-v2/src/schemas.gen.json`; the API
owner supplies the reviewed complete OpenAPI authority for the exact workload.
The local v1 suite uses the explicitly pinned
`sdk/launchpad/packages/production-contracts/src/schemas.gen.json` bundle.

Final phase-4 readout additionally requires `coordinator_authority_path` and
`coordinator_authority_sha256`. That document has the same `bindings`, a `spec`
entry with the reviewed coordinator specification's `path` and exact `sha256`,
plus the root-approved `freeze_task` and `final_task`. The native runtime authority must
repeat the coordinator authority digest and final task and bind
`coordinator_source_sha256` to the candidate. Missing authority fails admission.

Phase 2 runs an actual connected ten-minute host workload via `phase2_runner.py`;
its per-host assertions require 25,000 connections, 125,000 subscriptions and
1,500,000 scheduled HTTP arrivals. A host result alone does not qualify the
four-host scenario. Phase 3 adds hot-pool skew, isolated PostgreSQL/Redis/NATS
faults and recovery through `phase3_runner.py`, with exact owned fault authority.
Phase 4 exercises the calibrated isolated socket/fanout/event/HTTP adapter via
`phase4_runner.py` and the native `eventdriver`; it requires actual capture and
runtime authority. It is distinct from the final shared capacity scenarios.

Paxeer is one network on chain ID 125, where a receipt is final after block commit.
Application replay and local EVM fixtures do not prove Paxeer consensus throughput.
Use the node root URL for public JSON-RPC, for example
`https://api1.mainnet-beta.paxeer.network/`; provider-backed qualification must
still use its explicitly admitted targets and accounts.

## Qualification limits

Consult the phase scenario manifests/evidence policies and
[final-integrated-gate-readiness.json](final-integrated-gate-readiness.json) for
required evidence. Implemented runners, configured targets and isolated component
proofs do not establish full managed-provider playback, four-host calibration,
100,000/200,000 sockets, a 24-hour soak, reconciliation/lag or durability.
Final shared execution requires a root-reviewed coordinator authority bound to
the candidate's coordinator specification. Its `freeze_task` must be done, its
`final_task` must be in progress and depend on that freeze, and all 49 other runtime lane bindings
must match the candidate, interface and topology. The old `9.1` scenario labels
identify frozen workload definitions only. Capacity and release qualification
remain false without actual final evidence. The root programme owns the reviewed
freeze/final task mapping.

The [workload accounting, calibration and loopback transport tests](test_load.py)
check scheduled-arrival accounting, capacity refusal, receiver pauses and masked
subscriptions. The [contract and final-admission tests](repair_m965_cases.py),
`test_v1_contract`, `test_v2_contract`, `test_final_admission` and
`test_phase4_qualification`, check schema authority and coordinator admission.
These local checks do not supply provider or final shared-capacity evidence;
the phase commands above retain their runtime assertions and admission requirements.

See the [integration phases](../integration/README.md),
[bootstrap baseline](../bootstrap/README.md),
[security evidence limits](../bootstrap/security/GAPS.md) and
[dispatcher](../dispatcher/README.md) for the adjacent qualification workflows.
