# Observability primitives and qualification

This directory contains launchpad telemetry contracts and measurement tooling.
It does not start network validators or deploy exporters. For the application
deployment layout, see the [launchpad overview](../../README.md); service
restoration is covered in the [recovery overview](../recovery/README.md).

## Dependencies and v1 contracts

Install the Python dependencies from the repository root with `python3 -m pip install -r deploy/launchpad/production/observability/requirements.txt` before collecting or running the observability tests. This includes the pinned `jsonschema` dependency used by the phase4 plan validator.

`config.json` is strict revision 1 configuration, not an activated deployment. `contracts.py` validates it and implements bounded observation/privacy/SLI/state contracts. Unknown fields, unowned alerts, excessive limits and unproven capacity claims reject. Runtime activation intentionally fails closed until serial wiring and exporter privacy proof exist.

`prometheus-rules.json` is native Prometheus-compatible JSON/YAML: sixteen windowed burn/denominator recording rules and fifteen owned alerts. Read 99.95% and mutation 99.9% budgets use simultaneous 14.4x five-minute/one-hour critical and 6x thirty-minute/six-hour warning gates, with >=100 requests in each applicable window. Insufficient traffic is explicit. Expected synthetic journey inventory must originate independently of probe results. The dashboard includes normal denominators, availability, latency and storage horizon, as well as alert expressions.

`otel-collector.yaml` is a staged contrib 0.123.0 contract. No native OTTL validation/export proof is claimed. Positive attribute-key allowlists do not prove arbitrary allowed values, scope/link/exemplar/name privacy; source instrumentation and native marker checks are mandatory before activation. A 512 MiB memory limiter is advisory and needs a real runtime/cgroup ceiling. Failures may lose telemetry under pressure; no business commit may wait for export. OTLP endpoint and mTLS CA/cert/key references are mandatory runtime variables, not credentials checked into source.

## Run the primitive checks

From the repository root, with a reviewed native binary and private evidence directory:

```sh
python3 deploy/launchpad/production/observability/check.py \
  --promtool /reviewed/path/to/promtool \
  --output /private/evidence/checks.json
```

Native promtool 3.2.1 required; download tool independently from the official release and verify its binary digest. Python standard library checks use no service fixtures. Prometheus rule fixtures execute actual evaluation but contain synthetic counters, not deployed telemetry. Million-request and million-address loops test arithmetic and cardinality, not traffic capacity or independently reconstructed access logs.

`capacity_evidence` is exact typed ingestion shape only; it does not verify artifact files, independent measurements, workload sufficiency, prices or release eligibility. Zero workload is representable and never implies qualified capacity. Actual capacity release qualification belongs to the serial/load gate.

All twelve D14_019 through D14_030 end-state requirement rehearsals remain pending deployed runtime evidence. This lane supplies primitive declarations and local invariants only. The [activation runbook](phase2/runbooks/activation.md) and
[capacity runbook](phase2/runbooks/capacity.md) define the runtime evidence and
provider bindings. The [alert runbooks](runbooks/) cover availability, integrity,
storage, synthetic journeys and telemetry.

## Adversarial evidence and activation

`phase2/qualify.py --phase 3` evaluates adversarial evidence version 3.0 without accepting version 2.0 as recovery proof. It retains the twelve scenario checks and requires an isolated provider inventory, timestamped metric samples, independently captured authoritative state, denied control actions, zone-loss fencing and interrupted activation reconciliation. Missing captures yield failed requirement results. Source implementation does not establish a completed rehearsal.

`phase2/control.py --phase 3 --operator-authority-file /private/operator-authority.json` requires an OS-owned private authority mount. Authority binds the runtime binding digest, an expiry within 24 hours, and unique OS UID/actor/action assignments. An actor argument alone cannot authorize phase 3 or production changes. A secured journal cannot be reopened with a lower control phase or without its authority mount. Activation atomically publishes an immutable bundle and commits its digest in the journal; an orphan bundle is verified before retry and never implies activation.

Phase 3 tests consume `OBSERVABILITY_PHASE3_SCENARIO_MANIFEST` alongside the existing runtime binding, phase 2 manifest and verifier-key mounts. They exercise the real SQLite journal, process interruption, competing writes, private authority mounts and immutable bundle recovery; native evidence tests reject tampered copies of independently captured artifacts. No external failure-injection API is inferred from an endpoint. Read the [activation runbook](phase2/runbooks/activation.md) for the authority
and capture contract; the phase options above are not complete invocations.

## Measurement and shared capture

`phase2/phase4.py collect` runs actual bounded service-metric scrapes and all five route probes on a 30-second cadence. It writes private raw metric files, probe responses reduced to safe outcome metadata, a durable progress manifest and a final measurement manifest. The approved raw-capture byte budget is enforced before each capture write; progress and final metadata each have a 64 MiB bound reserved from actual disk allocations. A missing provider, failed probe, counter reset, schedule gap or exhausted capture budget remains a failed observation. Native telemetry is never synthesized from workload targets.

`phase2/phase4.py evaluate` consumes the collection hash, current attested phase 3 scenarios, an actual D16 `kindle-shared-capture-v1` manifest and source-bound allocation/cost inputs. It reconstructs origin numerators and histogram bounds, checks all twelve requirement scenarios and recovery records, compares measured instance headroom with allocations, and emits twelve requirement files plus a machine-readable result. An isolated component measurement can pass the lane checks; capacity, monthly availability and production qualification remain false. Final shared 24-hour soak and all burst/restore acceptance belong to gate 9.1.

Run the collector alongside the same admitted D16 run, with `--plan`, `--binding`, `--config`, `--root` and a fresh private `--output`. After captures close, invoke evaluate with those plan/binding/config inputs and `--measurement`, `--measurement-sha256`, `--shared-capture`, `--shared-capture-sha256`, `--scenario-evidence`, `--scenario-evidence-sha256`, `--attestation-key-file` and a separate output directory. The source does not provision infrastructure or infer provider credentials. See the [capacity runbook](phase2/runbooks/capacity.md) for the plan and
allocation contracts. Recovery measurements
for the same shared capture are described in the
[recovery release runbook](../recovery/phase4/runbook.md).

From the repository root, collection uses:

```sh
python3 deploy/launchpad/production/observability/phase2/phase4.py collect \
  --plan <private-plan> --binding <runtime-binding> --config <runtime-config> \
  --root <repository-root> --output <fresh-private-collection-directory>
```

Evaluation uses a separate output directory and the admitted capture hashes:

```sh
python3 deploy/launchpad/production/observability/phase2/phase4.py evaluate \
  --plan <private-plan> --binding <runtime-binding> --config <runtime-config> \
  --root <repository-root> --output <fresh-private-evaluation-directory> \
  --measurement <measurement-manifest> --measurement-sha256 <measurement-digest> \
  --shared-capture <shared-capture> --shared-capture-sha256 <capture-digest> \
  --scenario-evidence <phase3-evidence> --scenario-evidence-sha256 <evidence-digest> \
  --attestation-key-file <private-verifier-key>
```
