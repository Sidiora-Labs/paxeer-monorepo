use std::sync::atomic::{AtomicU64, Ordering};
use std::time::Instant;

pub struct Metrics {
    requests: Box<[AtomicU64]>,
    started: Instant,
}

impl Default for Metrics {
    fn default() -> Self {
        Self {
            requests: (0..=u16::MAX).map(|_| AtomicU64::new(0)).collect(),
            started: Instant::now(),
        }
    }
}

impl Metrics {
    pub fn record(&self, status: u16) {
        self.requests[usize::from(status)].fetch_add(1, Ordering::Relaxed);
    }

    #[must_use]
    pub fn render(&self) -> String {
        let mut text = format!(
            "# TYPE paxai_worker_up gauge\npaxai_worker_up 1\n\
             # TYPE paxai_worker_uptime_seconds gauge\npaxai_worker_uptime_seconds {}\n\
             # TYPE paxai_worker_requests_total counter\n",
            self.started.elapsed().as_secs_f64()
        );
        for (status, counter) in self.requests.iter().enumerate() {
            let count = counter.load(Ordering::Relaxed);
            if count > 0 || status == 200 {
                text.push_str("paxai_worker_requests_total{status=\"");
                text.push_str(&status.to_string());
                text.push_str("\"} ");
                text.push_str(&count.to_string());
                text.push('\n');
            }
        }
        text
    }
}
