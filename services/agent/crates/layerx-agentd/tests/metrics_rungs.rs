use std::collections::BTreeSet;

use layerx_agentd::degraded::{enter, Controller, Mode, Observation, Reference};
use layerx_agentd::obs::metrics::{MetricKind, MetricLabel, Metrics, MetricsError};
use layerx_agentd::store::TenantId;
use layerx_agentd::tenant::{
    normalize_error, BoundedMetrics, ErrorClass, InternalError, MetricLabel as TenantMetricLabel,
    Surface,
};
use layerx_types::rung::Rung;

#[test]
fn rung_metric_labels_are_exactly_the_three_rungs() {
    let tenant = TenantId::new("rung-metrics").expect("tenant");
    let mut metrics = Metrics::new(1).expect("metrics");
    metrics.register_tenant(tenant.clone()).expect("register");
    let expected = [
        (Rung::Submitted, MetricLabel::Submitted),
        (Rung::Receipted, MetricLabel::Receipted),
        (Rung::Committed, MetricLabel::Committed),
    ];
    assert_eq!(
        MetricKind::ALL
            .iter()
            .filter(|&&kind| kind == MetricKind::Rung)
            .count(),
        1
    );
    for label in MetricLabel::ALL {
        let result = metrics.observe(&tenant, MetricKind::Rung, label, 7);
        if expected.iter().any(|&(_, expected)| label == expected) {
            assert_eq!(result, Ok(()));
        } else {
            assert_eq!(result, Err(MetricsError::InvalidLabel));
        }
    }
    let snapshot = metrics.snapshot(&tenant);
    assert_eq!(snapshot.len(), 3);
    assert_eq!(
        snapshot
            .iter()
            .map(|(key, _)| key.label.to_string())
            .collect::<Vec<_>>(),
        ["submitted", "receipted", "committed"]
    );
    for ((key, point), (rung, label)) in snapshot.iter().zip(expected) {
        assert_eq!(key.tenant, tenant);
        assert_eq!(key.kind, MetricKind::Rung);
        assert_eq!(key.label, label);
        assert_eq!(key.label.name(), rung.name());
        assert_eq!(point.samples, 1);
        assert_eq!(point.total, 7);
        assert_eq!(point.maximum, 7);
    }
}

#[test]
fn degraded_state_labels_name_node_conditions() {
    let tenant = TenantId::new("degraded-metrics").expect("tenant");
    let reference = Reference {
        head_sequence: 20,
        checkpoint: [4; 32],
    };
    let mut controller = Controller::default();
    assert_eq!(controller.status().mode, Mode::Unreachable);
    let cases = [
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
            MetricLabel::NodeBehind,
            "node_behind",
        ),
        (
            Observation::Halted {
                reference,
                maximum_rung: Rung::Receipted,
            },
            Mode::Halted,
            MetricLabel::ChainHalted,
            "chain_halted",
        ),
        (
            Observation::Emergency {
                reference,
                maximum_rung: Rung::Submitted,
            },
            Mode::Emergency,
            MetricLabel::ChainHalted,
            "chain_halted",
        ),
        (
            Observation::DataUnavailable {
                reference,
                maximum_rung: Rung::Submitted,
            },
            Mode::DataUnavailable,
            MetricLabel::BlockUnavailable,
            "block_unavailable",
        ),
    ];
    let mut degraded_labels = BTreeSet::new();
    for (observation, mode, label, name) in cases {
        let status = enter(&mut controller, observation);
        assert_eq!(status.mode, mode);
        match observation {
            Observation::Ready { maximum_rung, .. }
            | Observation::Halted { maximum_rung, .. }
            | Observation::Emergency { maximum_rung, .. }
            | Observation::DataUnavailable { maximum_rung, .. } => {
                assert_eq!(status.maximum_rung, maximum_rung);
            }
            Observation::Unreachable => assert!(status.reference.is_none()),
        }
        assert_eq!(status.readiness.preparation, mode == Mode::Healthy);
        assert_eq!(status.readiness.live_stream, mode == Mode::Healthy);
        let mut metrics = Metrics::new(1).expect("metrics");
        metrics.register_tenant(tenant.clone()).expect("register");
        controller
            .record_metric(&mut metrics, &tenant)
            .expect("degraded metric");
        let snapshot = metrics.snapshot(&tenant);
        assert_eq!(snapshot.len(), 1);
        let (key, point) = &snapshot[0];
        assert_eq!(key.kind, MetricKind::DegradedState);
        assert_eq!(key.label, label);
        assert_eq!(key.label.name(), name);
        assert_eq!(key.label.to_string(), name);
        assert_eq!(point.samples, 1);
        assert_eq!(point.total, 1);
        if mode != Mode::Healthy {
            degraded_labels.insert(key.label.name());
        }
    }
    assert_eq!(
        degraded_labels,
        BTreeSet::from([
            "node_behind",
            "node_unavailable",
            "chain_halted",
            "block_unavailable",
        ])
    );
    let mut metrics = Metrics::new(1).expect("metrics");
    metrics.register_tenant(tenant.clone()).expect("register");
    for label in MetricLabel::ALL {
        assert_eq!(
            metrics.observe(&tenant, MetricKind::DegradedState, label, 1),
            if label == MetricLabel::BoundaryReady || degraded_labels.contains(label.name()) {
                Ok(())
            } else {
                Err(MetricsError::InvalidLabel)
            }
        );
    }
    let mut tenant_metrics = BoundedMetrics::default();
    let error = normalize_error(
        &InternalError::boundary("private node diagnostic"),
        &tenant,
        Surface::RustSdk,
        &mut tenant_metrics,
    );
    assert_eq!(error.class, ErrorClass::Boundary);
    assert_eq!(error.status, 503);
    assert!(error.retryable);
    assert_eq!(tenant_metrics.counters().len(), 1);
    let (key, count) = tenant_metrics
        .counters()
        .first_key_value()
        .expect("node failure");
    assert_eq!(key.label, TenantMetricLabel::BoundaryFailure);
    assert_eq!(key.label.name(), "node_failure");
    assert_eq!(key.label.to_string(), "node_failure");
    assert_eq!(*count, 1);
}
