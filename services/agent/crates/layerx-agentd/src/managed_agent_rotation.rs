use crate::human::HumanResponse;
use crate::identity::CoreIdentity;
use crate::managed_agent::FinalizationEvidence;
use crate::session::{RevocationEvent, SessionRegistry};
use crate::store::{Store, TenantId, TenantKey};
use layerx_proof::receipt::AuthorizedBatch;
use layerx_types::payload::ModuleRegistry;

pub(crate) struct Projection<'a> {
    pub custody_key: &'a str,
    pub signed_activity: &'a [u8],
    pub evidence: FinalizationEvidence,
    pub identity: &'a CoreIdentity,
    pub authority: &'a AuthorizedBatch,
    pub registry: &'a ModuleRegistry,
}

pub(crate) enum Prepared {
    Replay(HumanResponse),
    Commit {
        event: RevocationEvent,
        updates: Vec<(TenantKey, Vec<u8>)>,
        response: HumanResponse,
    },
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum OwnerRotation {
    Unsupported,
}

impl OwnerRotation {
    #[must_use]
    pub(crate) const fn reason(self) -> &'static str {
        match self {
            Self::Unsupported => "rotation_unsupported",
        }
    }
}

impl std::fmt::Display for OwnerRotation {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter.write_str(self.reason())
    }
}

impl std::error::Error for OwnerRotation {}

pub(crate) fn prepare(
    _store: &Store,
    _sessions: &SessionRegistry,
    _tenant: &TenantId,
    _agent_id: &str,
    _request: &Projection<'_>,
) -> Result<Prepared, OwnerRotation> {
    Err(OwnerRotation::Unsupported)
}
