use layerx_human_service::store::{PrincipalStore, RetentionPolicy, TenancyMap};
use std::error::Error;
use std::path::Path;
use std::sync::{Arc, Mutex};

pub fn open(
    root: &Path,
    tenancy: &TenancyMap,
    retention: RetentionPolicy,
) -> Result<Arc<Mutex<PrincipalStore>>, Box<dyn Error>> {
    let digest = tenancy.install(root)?;
    Ok(Arc::new(Mutex::new(PrincipalStore::open(
        root, retention, digest,
    )?)))
}
