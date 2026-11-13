# Agent observability contract

[`Metrics`](../../../services/agent/crates/layerx-agentd/src/obs/metrics.rs) is a
bounded, in-process registry. Each point is keyed by tenant, `MetricKind` and
`MetricLabel`; it records `total`, `samples` and `maximum`. A tenant must be
registered before observation. Unknown tenants, exhausted tenant capacity and
labels outside the selected family are refused.

`observe_named` decodes a label through `MetricLabel::ALL` before recording it.
`snapshot` returns only the selected tenant's points. The labels below are the
strings emitted by `MetricLabel::name`.

## boundarylatency

`BoundaryLatency`: `boundary_ready`, `boundary_backpressured`, `boundary_unavailable`.

## budgetutilization

`BudgetUtilization`: `utilization_low`, `utilization_high`.

## capabilitydecision

`CapabilityDecision`: `allowed`, `denied`.

## degradedstate

`DegradedState`: `boundary_ready`, `node_behind`, `node_unavailable`,
`chain_halted`, `block_unavailable`.

## errorclass

`ErrorClass`: `access_denied`, `invalid_request`, `storage_failure`,
`node_failure`, `boundary_unavailable`, `internal_failure`.

## policydecision

`PolicyDecision`: `allowed`, `denied`.

## ratelimitrefusal

`RateLimitRefusal`: `rate_exceeded`.

## submissionoutcome

`SubmissionOutcome`: `executed`, `failed`, `unknown`.

## subscriptionlag

`SubscriptionLag`: `lag_healthy`, `lagging`.

## unknownage

`UnknownAge`: `age_under_second`, `age_under_minute`, `age_at_least_minute`.

## unknownpopulation

`UnknownPopulation`: `unknown`.

## verificationlevel

Receipt progress is recorded by `MetricKind::Rung`, with labels `submitted`,
`receipted` and `committed`. See [agent-metrics.md](agent-metrics.md#rung).

Observation output uses the tenant redaction rules in
[`obs/mod.rs`](../../../services/agent/crates/layerx-agentd/src/obs/mod.rs).
