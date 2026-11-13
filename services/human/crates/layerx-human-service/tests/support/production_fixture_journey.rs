use crate::production_fixture::{Fixture, IdentityTenancy, Result};
use base64::{engine::general_purpose::URL_SAFE_NO_PAD, Engine as _};
use layerx_types::clock::Clock;
use serde_json::Value;
use std::sync::Arc;
use std::time::Duration;

pub(crate) fn journey_record(fixture: &mut Fixture, journey_id: &str) -> Result<Value> {
    edit_journey_record(fixture, journey_id, |_| {})
}

pub(crate) fn edit_journey_record(
    fixture: &mut Fixture,
    journey_id: &str,
    edit: impl FnOnce(&mut Value),
) -> Result<Value> {
    use layerx_human_service::store::{PrincipalId, PrincipalStore, RowKey, Table, TenancyDigest};

    fixture.close_router()?;
    let binding = layerx_identity_binding::Client::new(
        layerx_identity_binding::Config {
            socket: fixture.root.join("identity-binding.sock"),
            tenant: "tenant-a".to_owned(),
            peer_uid: rustix::process::getuid().as_raw(),
            peer_gid: rustix::process::getgid().as_raw(),
            deadline: Duration::from_secs(5),
        },
        fixture.clock.clone(),
    )?;
    let digest: [u8; 32] = URL_SAFE_NO_PAD
        .decode(std::env::var("LAYERX_HUMAN_TENANCY_DIGEST")?)?
        .try_into()
        .map_err(|_| "tenancy digest")?;
    let mut store = PrincipalStore::open_with_authority(
        fixture.root.join("store"),
        layerx_human_test_support::retention_uniform(86400),
        TenancyDigest::new(digest),
        Arc::new(IdentityTenancy(binding)),
    )?;
    let principal = PrincipalId::new(fixture.document["principal"].as_str().ok_or("principal")?)?;
    let mut scope = store.principal(&principal)?;
    let key = RowKey::new(format!("intent-evm-{journey_id}"))?;
    let mut record: Value = serde_json::from_slice(
        scope
            .get(Table::Journeys, &key)
            .ok_or("journey record")?
            .bytes(),
    )?;
    let original = record.clone();
    edit(&mut record);
    if record != original {
        let now = fixture.clock.sample(Duration::from_secs(1))?.unix_seconds();
        scope.put(Table::Journeys, key, now, serde_json::to_vec(&record)?)?;
    }
    drop(scope);
    drop(store);
    fixture.reopen()?;
    Ok(record)
}
