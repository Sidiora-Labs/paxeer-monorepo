//! Authenticated reads over persisted journey and activity evidence.

use layerx_client::sequencer::ReceiptKeyEntry;

use super::{
    ApiFailure, AuthorizedSession, BackendResponse, ComponentOperationRequest, IdentityServices,
    PrivilegedHumanServices, Readiness,
};
use crate::auth::Passkeys;
use crate::journeys::JourneyEngine;
use crate::notify::{ActivityEntryId, JourneyId};
use crate::store::{PrincipalScope, PrincipalStore};

/// A read composition with explicitly installed chain authorities and asset metadata.
/// Authentication and principal scoping remain owned by `PrivilegedHumanComponents`.
pub struct PersistedReadServices {
    identity: IdentityServices,
    receipt_keys: Vec<ReceiptKeyEntry>,
    assets: layerx_network_gateway::AssetTable,
}

impl PersistedReadServices {
    /// # Errors
    /// Refuses missing, invalid or ambiguous receipt authorities.
    pub fn new(
        identity: IdentityServices,
        receipt_keys: Vec<ReceiptKeyEntry>,
        assets: layerx_network_gateway::AssetTable,
    ) -> Result<Self, ApiFailure> {
        if receipt_keys.is_empty()
            || receipt_keys.iter().enumerate().any(|(index, key)| {
                key.public_key == [0; 32]
                    || key.first_height > key.last_height
                    || receipt_keys[..index]
                        .iter()
                        .any(|prior| prior.key_id == key.key_id)
            })
        {
            return Err(ApiFailure::upstream_degraded());
        }
        Ok(Self {
            identity,
            receipt_keys,
            assets,
        })
    }
}

impl PrivilegedHumanServices for PersistedReadServices {
    fn execute_public(
        &mut self,
        store: &mut PrincipalStore,
        passkeys: &Passkeys,
        request: ComponentOperationRequest<'_>,
        now: u64,
    ) -> Result<BackendResponse, ApiFailure> {
        self.identity.execute_public(store, passkeys, request, now)
    }

    fn execute_authorized(
        &mut self,
        scope: &mut PrincipalScope<'_>,
        passkeys: &Passkeys,
        session: AuthorizedSession<'_>,
        request: ComponentOperationRequest<'_>,
        now: u64,
    ) -> Result<BackendResponse, ApiFailure> {
        let result = match request.operation.name.as_str() {
            "journey.get" => {
                let id = JourneyId::new(
                    request
                        .path_parameters
                        .get("journey_id")
                        .cloned()
                        .ok_or_else(|| ApiFailure::invalid_request(Some("journey_id")))?,
                )
                .map_err(|_| ApiFailure::invalid_request(Some("journey_id")))?;
                let journey = JourneyEngine::load(scope, &id)
                    .map_err(|_| ApiFailure::upstream_degraded())?
                    .ok_or_else(ApiFailure::not_found)?;
                super::production_reads::native_journey_json_with(scope, &journey, || {
                    Ok(self.receipt_keys.clone())
                })?
            }
            "activity.entry" => {
                let id = ActivityEntryId::new(
                    request
                        .path_parameters
                        .get("entry_id")
                        .cloned()
                        .ok_or_else(|| ApiFailure::invalid_request(Some("entry_id")))?,
                )
                .map_err(|_| ApiFailure::invalid_request(Some("entry_id")))?;
                super::production_reads::activity_entry_json_with(
                    scope,
                    &id,
                    || Ok(self.receipt_keys.clone()),
                    || Ok(self.assets.clone()),
                )?
            }
            _ => {
                return self
                    .identity
                    .execute_authorized(scope, passkeys, session, request, now)
            }
        };
        Ok(BackendResponse {
            result,
            session: None,
        })
    }

    fn readiness(&mut self, store: &mut PrincipalStore, now: u64) -> Result<Readiness, ApiFailure> {
        self.identity.readiness(store, now)
    }
}
