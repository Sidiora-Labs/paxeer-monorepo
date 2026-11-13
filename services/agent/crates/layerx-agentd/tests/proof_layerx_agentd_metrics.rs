//! Proves: `degraded::Controller::record_metric` and `tenant::normalize_error` -> `Metrics::observe_named` / `BoundedMetrics::scrape_errors` -> `Metrics::snapshot`.
//!
//! Repairs the split error vocabulary in src/obs/metrics.rs: the real tenant
//! `MetricLabel::BoundaryFailure` produces `node_failure`, which the shared registry
//! could not represent. Named decoding now rejects malformed or misplaced labels
//! before they can change a scrape. The declared degraded interface names remain
//! compatibility constants for the merge's node and chain labels.
//!
//! Outside touches, `tests/metrics_rungs.rs::degraded_state_labels_name_node_conditions`
//! requires Halted and Emergency to share `chain_halted`; this proof asserts that
//! current behavior rather than inventing a distinction in the metric transport.
//! There is no durable metrics anchor: each consumer reads fresh in-memory state,
//! twice, with an intervening producer update, and error scrapes rebuild the view.

use std::cell::Cell;

use layerx_agentd::degraded::{enter, Controller, Mode, Observation, OperationError, Reference};
use layerx_agentd::obs::metrics::{MetricKind, MetricLabel, MetricPoint, Metrics, MetricsError};
use layerx_agentd::store::TenantId;
use layerx_agentd::tenant::{
    normalize_error, BoundedMetricKey, BoundedMetrics, ErrorClass, InternalError,
    MetricKind as TenantMetricKind, MetricLabel as TenantMetricLabel, Surface,
};
use layerx_types::rung::Rung;

fn tenant(name: &str) -> TenantId {
    TenantId::new(name).unwrap_or_else(|error| panic!("tenant: {error}"))
}

#[test]
fn real_degraded_producers_cross_named_metrics_into_fresh_tenant_scrapes() {
    let alpha = tenant("alpha");
    let beta = tenant("beta");
    let reference = Reference {
        head_sequence: 20,
        checkpoint: [4; 32],
    };
    let mut controller = Controller::default();
    let cases = degraded_cases(reference);
    for (observation, mode, label, name) in cases {
        let status = enter(&mut controller, observation);
        assert_eq!(status.mode, mode);
        let mut metrics = Metrics::new(2).unwrap_or_else(|error| panic!("metrics: {error}"));
        metrics
            .register_tenant(alpha.clone())
            .unwrap_or_else(|error| panic!("register: {error}"));
        metrics
            .register_tenant(beta.clone())
            .unwrap_or_else(|error| panic!("register: {error}"));
        let mut previous = Vec::new();
        for count in 1_u64..=2 {
            controller
                .record_metric(&mut metrics, &alpha)
                .unwrap_or_else(|error| panic!("producer metric: {error}"));
            let observed = metrics.snapshot(&alpha);
            assert_ne!(observed, previous, "a scrape must see the producer update");
            assert_eq!(observed.len(), 1);
            let (key, point) = &observed[0];
            assert_eq!(key.tenant, alpha);
            assert_eq!(key.kind, MetricKind::DegradedState);
            assert_eq!(key.label, label);
            assert_eq!(key.label.name(), name);
            assert_eq!(key.label.to_string(), name);
            assert_eq!(
                *point,
                MetricPoint {
                    total: u128::from(count),
                    samples: count,
                    maximum: 1,
                }
            );
            assert_eq!(metrics.snapshot(&alpha), observed);
            assert!(metrics.snapshot(&beta).is_empty());
            previous = observed;
        }
        let ran = Cell::new(false);
        let result = controller.guard_preparation(|| ran.set(true));
        if mode == Mode::Healthy {
            assert_eq!(result, Ok(()));
            assert!(ran.get());
        } else {
            assert_eq!(result, Err(OperationError::LiveCoreRequired(mode)));
            assert!(!ran.get());
        }
        let before = metrics.snapshot(&alpha);
        assert_eq!(
            controller.record_metric(&mut metrics, &tenant("unregistered")),
            Err(MetricsError::UnknownTenant)
        );
        assert_eq!(metrics.snapshot(&alpha), before);
    }
}

#[test]
fn normalized_boundary_failure_crosses_the_shared_scrape_without_foreign_context() {
    let alpha = tenant("alpha");
    let beta = tenant("beta");
    let mut producer = BoundedMetrics::default();
    let boundary = InternalError::boundary("private node diagnostic");
    normalize_error(&boundary, &beta, Surface::Export, &mut producer);
    for round in 1_u64..=2 {
        for surface in Surface::ALL {
            let result = normalize_error(&boundary, &alpha, surface, &mut producer);
            assert_eq!(result.status, 503);
            assert_eq!(result.code, "boundary_unavailable");
            assert_eq!(result.class, ErrorClass::Boundary);
            assert!(result.retryable);
        }
        let observed = producer
            .scrape_errors(&alpha)
            .unwrap_or_else(|error| panic!("shared scrape: {error}"));
        assert_eq!(observed.len(), 1);
        let (key, point) = &observed[0];
        assert_eq!(key.tenant, alpha);
        assert_eq!(key.kind, MetricKind::ErrorClass);
        assert_eq!(key.label, MetricLabel::BoundaryFailure);
        assert_eq!(key.label.name(), TenantMetricLabel::BoundaryFailure.name());
        assert_eq!(key.label.to_string(), "node_failure");
        assert_eq!(point.total, u128::from(round) * Surface::ALL.len() as u128);
        assert_eq!(point.samples, Surface::ALL.len() as u64);
        assert_eq!(point.maximum, round);
        assert_eq!(producer.scrape_errors(&alpha), Ok(observed.clone()));
        assert!(!format!("{observed:?}").contains("private node diagnostic"));
        let foreign = producer
            .scrape_errors(&beta)
            .unwrap_or_else(|error| panic!("beta scrape: {error}"));
        assert_eq!(foreign.len(), 1);
        assert_eq!(foreign[0].0.tenant, beta);
        assert_eq!(foreign[0].1.total, 1);
    }
}

#[test]
fn malformed_producer_labels_are_refused_at_the_real_consumer_decoder() {
    let alpha = tenant("alpha");
    let mut producer = BoundedMetrics::default();
    normalize_error(
        &InternalError::boundary("private diagnostic"),
        &alpha,
        Surface::RustSdk,
        &mut producer,
    );
    let observed = producer
        .scrape_errors(&alpha)
        .unwrap_or_else(|error| panic!("valid scrape: {error}"));
    let emitted_name = observed[0].0.label.to_string();
    let mut consumer = Metrics::new(1).unwrap_or_else(|error| panic!("metrics: {error}"));
    consumer
        .register_tenant(alpha.clone())
        .unwrap_or_else(|error| panic!("register: {error}"));
    consumer
        .observe_named(&alpha, MetricKind::ErrorClass, &emitted_name, 1)
        .unwrap_or_else(|error| panic!("decode emitted label: {error}"));
    let before = consumer.snapshot(&alpha);
    for malformed in [
        format!("{emitted_name}\n"),
        format!("{emitted_name}\0"),
        String::new(),
    ] {
        assert_eq!(
            consumer.observe_named(&alpha, MetricKind::ErrorClass, &malformed, 1),
            Err(MetricsError::InvalidLabel)
        );
        assert_eq!(consumer.snapshot(&alpha), before);
    }
    assert_eq!(
        consumer.observe_named(&alpha, MetricKind::DegradedState, &emitted_name, 1),
        Err(MetricsError::InvalidLabel)
    );
    assert_eq!(consumer.snapshot(&alpha), before);

    // The public bounded producer can carry a typed but misplaced label. Its
    // consumer must reject the entire scrape, rather than silently omit it.
    producer.record(
        BoundedMetricKey {
            tenant: alpha.clone(),
            surface: Surface::RustSdk,
            kind: TenantMetricKind::ErrorClass,
            label: TenantMetricLabel::Executed,
        },
        1,
    );
    assert_eq!(
        producer.scrape_errors(&alpha),
        Err(MetricsError::InvalidLabel)
    );
    assert_eq!(
        producer.scrape_errors(&alpha),
        Err(MetricsError::InvalidLabel)
    );
}

fn degraded_cases(reference: Reference) -> [(Observation, Mode, MetricLabel, &'static str); 6] {
    [
        (
            Observation::Unreachable,
            Mode::Unreachable,
            MetricLabel::NodeUnavailable,
            "node_unavailable",
        ),
        (
            Observation::Ready {
                reference,
                maximum_rung: Rung::Committed,
            },
            Mode::Healthy,
            MetricLabel::BoundaryReady,
            "boundary_ready",
        ),
        (
            Observation::Ready {
                reference,
                maximum_rung: Rung::Receipted,
            },
            Mode::Behind,
            MetricLabel::BoundaryBehind,
            "node_behind",
        ),
        (
            Observation::Halted {
                reference,
                maximum_rung: Rung::Receipted,
            },
            Mode::Halted,
            MetricLabel::CoreHalted,
            "chain_halted",
        ),
        (
            Observation::Emergency {
                reference,
                maximum_rung: Rung::Submitted,
            },
            Mode::Emergency,
            MetricLabel::CoreEmergency,
            "chain_halted",
        ),
        (
            Observation::DataUnavailable {
                reference,
                maximum_rung: Rung::Submitted,
            },
            Mode::DataUnavailable,
            MetricLabel::DataUnavailable,
            "block_unavailable",
        ),
    ]
}
