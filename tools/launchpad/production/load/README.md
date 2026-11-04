# L48 workload primitives

Run commands from the repository root. Install the pinned Python dependencies in
[requirements.txt](requirements.txt) in the qualification environment; frozen
schema validation uses `jsonschema==4.25.1`.

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
The final shared scenarios belong to task `9.1` of the launchpad production-system
specification; capacity and release qualification remain false without that gate's
actual evidence. Dependencies and the root qualifier/plugin registry remain
integrator-owned.

See the [integration phases](../integration/README.md),
[bootstrap baseline](../bootstrap/README.md),
[security evidence limits](../bootstrap/security/GAPS.md) and
[dispatcher](../dispatcher/README.md) for the adjacent qualification workflows.
