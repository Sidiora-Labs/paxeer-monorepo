# Agent metrics contract

The in-process metrics registry is defined in
[`obs/metrics.rs`](../../../services/agent/crates/layerx-agentd/src/obs/metrics.rs).
Metrics are keyed by tenant, kind and label. `Metrics::observe` records total,
sample count and maximum; `Metrics::snapshot` returns one tenant's points.

## degraded-state

`MetricKind::DegradedState` accepts `boundary_ready`, `node_behind`,
`node_unavailable`, `chain_halted` and `block_unavailable`. The degraded controller
reports node and chain availability separately from receipt progress.

## rung

`MetricKind::Rung` accepts exactly `submitted`, `receipted` and `committed`.
Transaction submission is pending; a verified signed receipt establishes
execution, and block commitment establishes network finality.
