# Agent observability contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## boundarylatency

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.met-agentd-boundarylatency</code> | <code>metric</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/obs/metrics.rs:12</code> | <code>metric BoundaryLatency labels BoundaryReady&#124;BoundaryBackpressured&#124;BoundaryUnavailable</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.met-agentd-boundarylatency</code> | notes | In-process per-tenant counter (total, samples, maximum) behind Metrics::observe and Metrics::snapshot; no exporter or scrape endpoint exists at this revision. Boundary labels describe the LNI node boundary that stale task 11.5.16 re-targets. |

## budgetutilization

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.met-agentd-budgetutilization</code> | <code>metric</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/obs/metrics.rs:16</code> | <code>metric BudgetUtilization labels UtilizationLow&#124;UtilizationHigh</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.met-agentd-budgetutilization</code> | notes | In-process per-tenant counter (total, samples, maximum) behind Metrics::observe and Metrics::snapshot; no exporter or scrape endpoint exists at this revision. |

## capabilitydecision

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.met-agentd-capabilitydecision</code> | <code>metric</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/obs/metrics.rs:15</code> | <code>metric CapabilityDecision labels Allowed&#124;Denied</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.met-agentd-capabilitydecision</code> | notes | In-process per-tenant counter (total, samples, maximum) behind Metrics::observe and Metrics::snapshot; no exporter or scrape endpoint exists at this revision. |

## degradedstate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.met-agentd-degradedstate</code> | <code>metric</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/obs/metrics.rs:19</code> | <code>metric DegradedState labels BoundaryReady&#124;BoundaryUnavailable&#124;BoundaryBehind&#124;CoreHalted&#124;CoreEmergency&#124;DataUnavailable</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.met-agentd-degradedstate</code> | notes | In-process per-tenant counter (total, samples, maximum) behind Metrics::observe and Metrics::snapshot; no exporter or scrape endpoint exists at this revision. Boundary labels describe the LNI node boundary that stale task 11.5.16 re-targets. |

## errorclass

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.met-agentd-errorclass</code> | <code>metric</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/obs/metrics.rs:13</code> | <code>metric ErrorClass labels AccessDenied&#124;InvalidRequest&#124;StorageFailure&#124;BoundaryUnavailable&#124;InternalFailure</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.met-agentd-errorclass</code> | notes | In-process per-tenant counter (total, samples, maximum) behind Metrics::observe and Metrics::snapshot; no exporter or scrape endpoint exists at this revision. |

## policydecision

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.met-agentd-policydecision</code> | <code>metric</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/obs/metrics.rs:14</code> | <code>metric PolicyDecision labels Allowed&#124;Denied</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.met-agentd-policydecision</code> | notes | In-process per-tenant counter (total, samples, maximum) behind Metrics::observe and Metrics::snapshot; no exporter or scrape endpoint exists at this revision. |

## ratelimitrefusal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.met-agentd-ratelimitrefusal</code> | <code>metric</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/obs/metrics.rs:18</code> | <code>metric RateLimitRefusal labels RateExceeded</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.met-agentd-ratelimitrefusal</code> | notes | In-process per-tenant counter (total, samples, maximum) behind Metrics::observe and Metrics::snapshot; no exporter or scrape endpoint exists at this revision. |

## submissionoutcome

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.met-agentd-submissionoutcome</code> | <code>metric</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/obs/metrics.rs:8</code> | <code>metric SubmissionOutcome labels Executed&#124;Failed&#124;Unknown</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.met-agentd-submissionoutcome</code> | notes | In-process per-tenant counter (total, samples, maximum) behind Metrics::observe and Metrics::snapshot; no exporter or scrape endpoint exists at this revision. |

## subscriptionlag

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.met-agentd-subscriptionlag</code> | <code>metric</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/obs/metrics.rs:17</code> | <code>metric SubscriptionLag labels LagHealthy&#124;Lagging</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.met-agentd-subscriptionlag</code> | notes | In-process per-tenant counter (total, samples, maximum) behind Metrics::observe and Metrics::snapshot; no exporter or scrape endpoint exists at this revision. |

## unknownage

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.met-agentd-unknownage</code> | <code>metric</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/obs/metrics.rs:10</code> | <code>metric UnknownAge labels AgeUnderSecond&#124;AgeUnderMinute&#124;AgeAtLeastMinute</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.met-agentd-unknownage</code> | notes | In-process per-tenant counter (total, samples, maximum) behind Metrics::observe and Metrics::snapshot; no exporter or scrape endpoint exists at this revision. |

## unknownpopulation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.met-agentd-unknownpopulation</code> | <code>metric</code> | <code>stable</code> | <code>services/agent/crates/layerx-agentd/src/obs/metrics.rs:9</code> | <code>metric UnknownPopulation labels Unknown</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.met-agentd-unknownpopulation</code> | notes | In-process per-tenant counter (total, samples, maximum) behind Metrics::observe and Metrics::snapshot; no exporter or scrape endpoint exists at this revision. |

## verificationlevel

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.9.met-agentd-verificationlevel</code> | <code>metric</code> | <code>change</code> | <code>services/agent/crates/layerx-agentd/src/obs/metrics.rs:11</code> | <code>metric VerificationLevel labels SequencerSigned&#124;BatchIncluded&#124;StateProven&#124;CheckpointFinalised&#124;SettlementAnchored</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.9.met-agentd-verificationlevel</code> | notes | In-process per-tenant counter (total, samples, maximum) behind Metrics::observe and Metrics::snapshot; no exporter or scrape endpoint exists at this revision. Labels SequencerSigned..SettlementAnchored are the old verification ladder; stale tasks 0.58 and 6.5.13 move callers to the Rung ladder and 6.6.12 retires SettlementAnchored. |
