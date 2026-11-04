use super::{
    fs, io, verify_principals_tree, AgentTenantId, BTreeSet, Path, PrincipalId, PrincipalStore,
    RetentionPolicy, StoreError, TenancyDigest, TenancyError, PRINCIPALS_DIR,
};
use std::os::unix::fs::MetadataExt as _;
use std::sync::Arc;

pub trait PrincipalTenancyAuthority: std::fmt::Debug + Send + Sync {
    /// # Errors
    /// Refuses unavailable or unauthenticated provider bindings.
    fn tenant_for(&self, principal: &PrincipalId) -> Result<AgentTenantId, StoreError>;
}

impl PrincipalStore {
    /// # Errors
    /// Preserves pinned static tenancy checks and refuses changed provider bindings.
    pub fn open_with_authority(
        root: impl AsRef<Path>,
        retention: RetentionPolicy,
        tenancy_digest: TenancyDigest,
        authority: Arc<dyn PrincipalTenancyAuthority>,
    ) -> Result<Self, StoreError> {
        let root = root.as_ref();
        if !root.is_absolute() || fs::canonicalize(root)? != root {
            return Err(StoreError::Tenancy(TenancyError::DigestMismatch));
        }
        let mut store = Self::open(root, retention, tenancy_digest)?;
        store.provider = Some(authority);
        for principal in store.known_principals()? {
            store.resolve_tenant(&principal)?;
        }
        Ok(store)
    }

    /// # Errors
    /// Refuses invalid durable principal directories or unavailable provider state.
    pub fn known_principals(&self) -> Result<Vec<PrincipalId>, StoreError> {
        self.verify_root()?;
        let mut principals: BTreeSet<_> = self.tenancy.principals().into_iter().collect();
        if self.provider.is_some() {
            verify_principals_tree(&self.root)?;
            match self.root.child(PRINCIPALS_DIR, false) {
                Ok(directory) => {
                    for name in directory.entries()? {
                        let name = name.to_str().ok_or(StoreError::InvalidPrincipal)?;
                        principals.insert(PrincipalId::new(name)?);
                    }
                }
                Err(error) if error.kind() == io::ErrorKind::NotFound => (),
                Err(error) => return Err(error.into()),
            }
        }
        Ok(principals.into_iter().collect())
    }

    pub(super) fn resolve_tenant(
        &self,
        principal: &PrincipalId,
    ) -> Result<AgentTenantId, StoreError> {
        self.verify_root()?;
        let directory = self.principal_directory(principal)?;
        self.resolve_tenant_in(principal, &directory)
    }

    pub(super) fn resolve_tenant_in(
        &self,
        principal: &PrincipalId,
        directory: &super::codec::Directory,
    ) -> Result<AgentTenantId, StoreError> {
        let configured = self.tenancy.tenant_for(principal);
        let Some(provider) = &self.provider else {
            return configured.cloned().map_err(StoreError::from);
        };
        let tenant = provider.tenant_for(principal)?;
        if configured.is_ok_and(|value| value != &tenant) {
            return Err(StoreError::Tenancy(TenancyError::DigestMismatch));
        }
        let bytes = tenant.as_str().as_bytes();
        if let Some(file) = binding_file(directory, "provider-binding")? {
            verify_binding_file(&file, bytes)?;
        } else {
            if let Some(file) = binding_file(directory, "provider-binding.pending")? {
                verify_binding_file(&file, bytes)?;
            }
            super::codec::write_atomic(
                directory,
                "provider-binding",
                "provider-binding.pending",
                bytes,
            )?;
        }
        Ok(tenant)
    }
}

fn binding_file(
    directory: &super::codec::Directory,
    name: &str,
) -> Result<Option<fs::File>, StoreError> {
    match directory.open_file(name, rustix::fs::OFlags::RDONLY) {
        Ok(file) => Ok(Some(file)),
        Err(error) if error.kind() == io::ErrorKind::NotFound => Ok(None),
        Err(error) => Err(error.into()),
    }
}

fn verify_binding_file(file: &fs::File, expected: &[u8]) -> Result<(), StoreError> {
    let metadata = file.metadata()?;
    if metadata.len() > 255 || metadata.mode() & 0o077 != 0 {
        return Err(StoreError::Tenancy(TenancyError::DigestMismatch));
    }
    let mut bytes = Vec::new();
    std::io::Read::read_to_end(&mut std::io::Read::take(file, 256), &mut bytes)?;
    if bytes != expected {
        return Err(StoreError::Tenancy(TenancyError::DigestMismatch));
    }
    Ok(())
}

pub(super) fn ownership_lock(root: &super::codec::Directory) -> Result<fs::File, StoreError> {
    let lock = root.open_file(
        "provider.lock",
        rustix::fs::OFlags::RDWR | rustix::fs::OFlags::CREATE,
    )?;
    if lock.metadata()?.mode() & 0o077 != 0 {
        return Err(StoreError::Tenancy(TenancyError::DigestMismatch));
    }
    rustix::fs::flock(&lock, rustix::fs::FlockOperation::NonBlockingLockExclusive)
        .map_err(io::Error::from)?;
    Ok(lock)
}
