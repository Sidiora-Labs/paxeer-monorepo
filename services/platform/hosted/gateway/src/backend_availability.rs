#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum KernelBackend {
    Component,
    PublicCore,
    /// Optional hosted compatibility backend; native receipt keys are governed on chain.
    Authority,
    Identity,
    Registry,
}

impl KernelBackend {
    pub(crate) const ALL: [Self; 5] = [
        Self::Component,
        Self::PublicCore,
        Self::Authority,
        Self::Identity,
        Self::Registry,
    ];

    pub(crate) const fn name(self) -> &'static str {
        match self {
            Self::Component => "core_agent_boundary",
            Self::PublicCore => "public_core",
            Self::Authority => "independent_receipt_authority",
            Self::Identity => "identity",
            Self::Registry => "program_registry",
        }
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) struct BackendAvailability {
    pub(crate) backend: &'static str,
    pub(crate) ready: bool,
    pub(crate) configured: bool,
    pub(crate) reason: &'static str,
}

impl BackendAvailability {
    pub(crate) const fn probed(backend: &'static str, ready: bool) -> Self {
        Self {
            backend,
            ready,
            configured: true,
            reason: if ready { "ready" } else { "unreachable" },
        }
    }

    pub(crate) const fn not_configured(backend: &'static str) -> Self {
        Self {
            backend,
            ready: false,
            configured: false,
            reason: "not_configured",
        }
    }

    pub(crate) fn document(self) -> serde_json::Value {
        serde_json::json!({
            "state": if self.ready { "ready" } else { "unavailable" },
            "reason": self.reason
        })
    }
}
