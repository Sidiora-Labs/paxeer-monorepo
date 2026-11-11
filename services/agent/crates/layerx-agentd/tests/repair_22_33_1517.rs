fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

use layerx_agentd::store::TenantId;
use layerx_agentd::tenant::{
    normalize_error, BoundedMetricKey, BoundedMetrics, ErrorClass, InternalError, MetricKind,
    MetricLabel, NormalizedError, Surface,
};

#[test]
fn error_trace_growth_is_bounded_under_sustained_equivalent_errors() {
    let tenant = require_ok(TenantId::new("caller"), "valid tenant");
    let errors = [
        InternalError::missing(b"missing-secret-object".to_vec()),
        InternalError::not_authorized("foreign-secret-tenant", b"existing-secret-object".to_vec()),
    ];
    let expected = NormalizedError {
        status: 404,
        code: "not_authorized",
        message: "the requested resource is unavailable",
        class: ErrorClass::AccessDenied,
        retryable: false,
    };

    for surface in Surface::ALL {
        let mut metrics = BoundedMetrics::default();
        assert!(metrics.traces().is_empty());
        assert!(metrics.counters().is_empty());
        for index in 0..BoundedMetrics::TRACE_CAPACITY {
            assert_eq!(
                normalize_error(
                    &errors[index % errors.len()],
                    &tenant,
                    surface,
                    &mut metrics
                ),
                expected
            );
            assert_eq!(metrics.traces().len(), index + 1);
        }
        assert_eq!(metrics.counters().len(), 1);
        let retained = metrics.traces().to_vec();
        let allocation = metrics.traces().as_ptr();
        let error_key = BoundedMetricKey {
            tenant: tenant.clone(),
            surface,
            kind: MetricKind::ErrorClass,
            label: MetricLabel::AccessDenied,
        };
        let dropped_key = BoundedMetricKey {
            kind: MetricKind::DroppedTrace,
            ..error_key.clone()
        };
        assert!(!metrics.counters().contains_key(&dropped_key));

        for batch in 1..=8 {
            for index in 0..BoundedMetrics::TRACE_CAPACITY {
                assert_eq!(
                    normalize_error(
                        &errors[index % errors.len()],
                        &tenant,
                        surface,
                        &mut metrics
                    ),
                    expected
                );
                assert_eq!(metrics.traces().len(), BoundedMetrics::TRACE_CAPACITY);
                assert_eq!(metrics.traces().as_ptr(), allocation);
            }
            let dropped = (batch * BoundedMetrics::TRACE_CAPACITY) as u64;
            assert_eq!(metrics.counters().len(), 2);
            assert_eq!(metrics.counters()[&dropped_key], dropped);
            assert_eq!(
                metrics.counters()[&error_key],
                dropped + BoundedMetrics::TRACE_CAPACITY as u64
            );
            assert_eq!(metrics.traces(), retained);
            let scrape = require_ok(metrics.scrape_errors(&tenant), "shared error scrape");
            assert_eq!(scrape.len(), 1);
            let (key, point) = &scrape[0];
            assert_eq!(key.tenant, tenant);
            assert_eq!(
                key.kind,
                layerx_agentd::obs::metrics::MetricKind::ErrorClass
            );
            assert_eq!(
                key.label,
                layerx_agentd::obs::metrics::MetricLabel::AccessDenied
            );
            assert_eq!(point.total, u128::from(metrics.counters()[&error_key]));
        }
        for trace in metrics.traces() {
            assert_eq!(trace.tenant, tenant);
            assert_eq!(trace.surface, surface);
            assert_eq!(trace.error_class, ErrorClass::AccessDenied);
            assert_eq!(trace.normalization_work_units, 64);
            assert_eq!(trace.normalization_tag, retained[0].normalization_tag);
        }
        let rendered = format!(
            "{:?} {:?} {expected:?}",
            metrics.traces(),
            metrics.counters()
        );
        for secret in [
            "missing-secret-object",
            "foreign-secret-tenant",
            "existing-secret-object",
        ] {
            assert!(!rendered.contains(secret));
        }
    }
}

#[test]
fn overflow_counts_the_producing_tenant_surface_and_error_class() {
    let first = require_ok(TenantId::new("first"), "valid tenant");
    let second = require_ok(TenantId::new("second"), "valid tenant");
    let mut metrics = BoundedMetrics::default();
    for _ in 0..BoundedMetrics::TRACE_CAPACITY {
        normalize_error(
            &InternalError::missing(Vec::new()),
            &first,
            Surface::Contract,
            &mut metrics,
        );
    }
    let retained = metrics.traces().to_vec();
    let cases = [
        (
            InternalError::missing(Vec::new()),
            ErrorClass::AccessDenied,
            MetricLabel::AccessDenied,
            404,
            "not_authorized",
            "the requested resource is unavailable",
            false,
        ),
        (
            InternalError::invalid_request(),
            ErrorClass::InvalidRequest,
            MetricLabel::InvalidRequest,
            400,
            "invalid_request",
            "the request is invalid",
            false,
        ),
        (
            InternalError::storage("secret-storage-path"),
            ErrorClass::Storage,
            MetricLabel::StorageFailure,
            503,
            "internal_unavailable",
            "the service is temporarily unavailable",
            true,
        ),
        (
            InternalError::boundary("secret-peer"),
            ErrorClass::Boundary,
            MetricLabel::BoundaryFailure,
            503,
            "boundary_unavailable",
            "the service is temporarily unavailable",
            true,
        ),
        (
            InternalError::internal("secret-panic"),
            ErrorClass::Internal,
            MetricLabel::InternalFailure,
            500,
            "internal_error",
            "the request could not be completed",
            false,
        ),
    ];
    finish_overflow_counts_the_producing_tenant_surface_and_error_class_1_repair_22_33_1517(
        &first, &second, metrics, &retained, &cases,
    );
}

fn finish_overflow_counts_the_producing_tenant_surface_and_error_class_1_repair_22_33_1517(
    first: &TenantId,
    second: &TenantId,
    mut metrics: BoundedMetrics,
    retained: &Vec<layerx_agentd::tenant::SanitizedTrace>,
    cases: &[(
        InternalError,
        ErrorClass,
        MetricLabel,
        u16,
        &'static str,
        &'static str,
        bool,
    ); 5],
) {
    for tenant in [first, second] {
        for surface in Surface::ALL {
            for (error, class, label, status, code, message, retryable) in cases {
                for _ in 0..3 {
                    assert_eq!(
                        normalize_error(error, tenant, surface, &mut metrics),
                        NormalizedError {
                            status: *status,
                            code,
                            message,
                            class: *class,
                            retryable: *retryable,
                        }
                    );
                }
                let key = BoundedMetricKey {
                    tenant: tenant.clone(),
                    surface,
                    kind: MetricKind::DroppedTrace,
                    label: *label,
                };
                assert_eq!(metrics.counters()[&key], 3);
                let error_key = BoundedMetricKey {
                    kind: MetricKind::ErrorClass,
                    ..key
                };
                let initial = if tenant == first
                    && surface == Surface::Contract
                    && *class == ErrorClass::AccessDenied
                {
                    BoundedMetrics::TRACE_CAPACITY as u64
                } else {
                    0
                };
                assert_eq!(metrics.counters()[&error_key], initial + 3);
            }
        }
    }
    assert_eq!(metrics.traces(), retained);
    assert_eq!(
        metrics.counters().len(),
        2 * 2 * Surface::ALL.len() * cases.len()
    );
    let total_dropped: u64 = metrics
        .counters()
        .iter()
        .filter(|(key, _)| key.kind == MetricKind::DroppedTrace)
        .map(|(_, value)| value)
        .sum();
    assert_eq!(
        total_dropped,
        (2 * Surface::ALL.len() * cases.len() * 3) as u64
    );
}
