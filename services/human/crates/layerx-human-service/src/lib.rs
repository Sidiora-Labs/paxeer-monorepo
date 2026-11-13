#![forbid(unsafe_code)]

pub mod activity;
pub mod agents;
pub mod approvals;
pub mod audit;
pub mod auth;
pub mod custody;
mod event_producer;
pub mod frame;
pub mod health;
pub mod hex;
pub mod journeys;
pub mod notify;
pub mod onboarding;
pub mod redaction;
pub mod security;
pub mod server;
pub mod store;
pub mod support;
pub(crate) use server::identity_dispatch::api_projection::api_time as time;
pub mod trace;

#[cfg(test)]
mod time_round_trip_tests {
    use super::time::{rfc3339, seconds_from_rfc3339};

    #[test]
    fn every_declared_timestamp_round_trips_exactly() {
        for seconds in [0_u64, 1, 86_399, 86_400, 1_755_500_000, 4_102_444_800] {
            let text = rfc3339(seconds);
            assert_eq!(seconds_from_rfc3339(&text), Some(seconds), "{text}");
        }
    }

    #[test]
    fn a_timestamp_the_contract_forbids_is_refused() {
        for refused in [
            "2026-08-18T09:35:00+00:00",
            "2026-08-18 09:35:00Z",
            "2026-08-18T09:35:00.5Z",
            "2026-02-30T00:00:00Z",
            "2026-13-01T00:00:00Z",
            "2026-08-18T24:00:00Z",
            "",
        ] {
            assert_eq!(seconds_from_rfc3339(refused), None, "{refused}");
        }
    }
}

/// The immutable workspace boundary consumed by the human control plane.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct HumanWorkspaceManifest {
    /// Human-plane crates in build order.
    pub crates: [&'static str; 2],
    /// The only contract through which the human plane reaches `LayerX`.
    pub core_boundary: &'static str,
}

/// Returns the build-time declaration of the human workspace boundary.
#[must_use]
pub const fn human_workspace_manifest() -> HumanWorkspaceManifest {
    HumanWorkspaceManifest {
        crates: ["layerx-intents", "layerx-human-service"],
        core_boundary: "layerx-agent-api",
    }
}

#[cfg(test)]
mod tests {
    use super::human_workspace_manifest;

    #[test]
    fn declares_agent_contract_as_the_only_core_boundary() {
        assert_eq!(human_workspace_manifest().core_boundary, "layerx-agent-api");
    }
}
