use layerx_paxai_worker::metrics::Metrics;

#[test]
fn metrics_render_counts_requests_by_status() {
    let metrics = Metrics::default();
    let initial = metrics.render();
    assert!(initial.lines().any(|line| line == "paxai_worker_up 1"));
    assert!(initial
        .lines()
        .any(|line| line == "paxai_worker_requests_total{status=\"200\"} 0"));

    metrics.record(200);
    metrics.record(404);
    metrics.record(404);

    let rendered = metrics.render();
    assert!(rendered.lines().any(|line| line == "paxai_worker_up 1"));
    assert!(rendered
        .lines()
        .any(|line| line == "paxai_worker_requests_total{status=\"200\"} 1"));
    assert!(rendered
        .lines()
        .any(|line| line == "paxai_worker_requests_total{status=\"404\"} 2"));
    let uptime = rendered
        .lines()
        .find_map(|line| line.strip_prefix("paxai_worker_uptime_seconds "))
        .expect("uptime metric")
        .parse::<f64>()
        .expect("numeric uptime");
    assert!(uptime.is_finite() && uptime >= 0.0);
}
