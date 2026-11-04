use serde::{Deserialize, Serialize};

/// Empty object emitted by HealthController in standalone media worker mode.
#[derive(Clone, Default, Debug, PartialEq, Serialize, Deserialize, derive_new::new)]
#[serde(deny_unknown_fields)]
pub struct HealthCheckResponseData {}
