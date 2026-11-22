use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine as _;
use layerx_human_service::custody::CustodyError;
use layerx_types::clock::Clock;
use std::cell::Cell;
use std::sync::Arc;
use std::time::Duration;

pub struct NativeProfileDiagnostics {
    clock: Arc<dyn Clock>,
    members: Vec<String>,
    assertions: Vec<(&'static str, Option<(u64, u64)>)>,
    step: Cell<&'static str>,
    complete: Cell<bool>,
}

impl NativeProfileDiagnostics {
    pub fn new(
        clock: Arc<dyn Clock>,
        members: &[String],
        assertions: [(&'static str, &str); 3],
    ) -> Self {
        let assertions = assertions
            .into_iter()
            .map(|(name, assertion)| {
                let times = assertion
                    .split('.')
                    .nth(1)
                    .and_then(|payload| URL_SAFE_NO_PAD.decode(payload).ok())
                    .and_then(|bytes| serde_json::from_slice::<serde_json::Value>(&bytes).ok())
                    .and_then(|claims| {
                        Some((claims.get("iat")?.as_u64()?, claims.get("exp")?.as_u64()?))
                    });
                (name, times)
            })
            .collect();
        Self {
            clock,
            members: members.to_vec(),
            assertions,
            step: Cell::new("setup"),
            complete: Cell::new(false),
        }
    }

    pub fn at(&self, step: &'static str) {
        self.step.set(step);
    }

    pub fn report(&self, step: &'static str, code: &str) {
        self.at(step);
        let now = self
            .clock
            .sample(Duration::from_secs(1))
            .map(|reading| reading.unix_seconds());
        eprintln!(
            "native profile step={step} code={code} members={:?} clock_seconds={now:?}",
            self.members
        );
        for (name, times) in &self.assertions {
            if let (Ok(now), Some((issued, expires))) = (&now, times) {
                eprintln!("native profile assertion={name} issued_seconds={issued} expires_seconds={expires} age_seconds={} future_seconds={}", now.saturating_sub(*issued), issued.saturating_sub(*now));
            } else {
                eprintln!("native profile assertion={name} time_metadata=unavailable");
            }
        }
    }

    pub fn custody<T>(
        &self,
        step: &'static str,
        result: Result<T, CustodyError>,
    ) -> Result<T, CustodyError> {
        self.at(step);
        if let Err(error) = &result {
            self.report(step, error.refusal_code());
        }
        result
    }

    pub fn grant(&self, grant: &layerx_agent_api::identity::NativeLocalGrantConsentV1, now: u64) {
        match layerx_human_kms::attestor::validate_native_local_grant(grant) {
            Ok(coordinates) => eprintln!("native profile grant clock_seconds={now} custody_now_ms={} created_at_ms={} expires_at_ms={}", now.saturating_mul(1000), coordinates.created_at_ms, grant.expires_at_ms),
            Err(_) => self.report("grant coordinates", "invalid-evidence"),
        }
    }

    pub fn finish(&self) {
        self.complete.set(true);
        self.report("complete", "ok");
    }
}

impl Drop for NativeProfileDiagnostics {
    fn drop(&mut self) {
        if !self.complete.get() {
            self.report(self.step.get(), "incomplete");
        }
    }
}
