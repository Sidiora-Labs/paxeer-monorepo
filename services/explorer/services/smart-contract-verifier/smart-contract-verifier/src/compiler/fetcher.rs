use async_trait::async_trait;
use bytes::Bytes;
use mismatch::Mismatch;
use primitive_types::H256;
use sha2::{Digest, Sha256};
use std::{
    fmt::{Debug, Display},
    fs::{DirBuilder, File, Permissions},
    hash::Hash,
    io::{Read, Seek, SeekFrom, Write},
    os::unix::fs::{DirBuilderExt, MetadataExt, PermissionsExt},
    path::{Path, PathBuf},
    str::FromStr,
};
use thiserror::Error;
use tracing::instrument;

#[derive(Error, Debug)]
pub enum FetchError {
    #[error("version {0} not found")]
    NotFound(String),
    #[error("couldn't fetch the file: {0}")]
    Fetch(anyhow::Error),
    #[error("compiler HTTP request timed out")]
    Timeout,
    #[error("compiler HTTP response exceeds {limit} bytes")]
    BodyTooLarge { limit: usize },
    #[error("compiler HTTP request returned status {status}")]
    HttpStatus { status: u16 },
    #[error("hashsum of fetched file mismatch: {0}")]
    HashMismatch(#[from] Mismatch<H256>),
    #[error("can't parse hashsum: {0}")]
    HashParse(anyhow::Error),
    #[error("couldn't create file: {0}")]
    File(#[from] std::io::Error),
    #[error("tokio sheduling error: {0}")]
    Schedule(#[from] tokio::task::JoinError),
    #[error("validation failed: {0}")]
    Validation(anyhow::Error),
}

impl FetchError {
    pub fn is_timeout(&self) -> bool {
        matches!(self, Self::Timeout)
    }

    pub fn is_body_too_large(&self) -> bool {
        matches!(self, Self::BodyTooLarge { .. })
    }

    pub fn http_status(&self) -> Option<u16> {
        match self {
            Self::HttpStatus { status } => Some(*status),
            _ => None,
        }
    }
}

#[async_trait]
pub trait FileValidator<Ver>: Send + Sync {
    async fn validate(&self, ver: &Ver, path: &Path) -> Result<(), anyhow::Error>;
}

#[async_trait]
pub trait Fetcher: Send + Sync {
    type Version: Sync;
    async fn fetch(&self, ver: &Self::Version) -> Result<PathBuf, FetchError>;
    fn all_versions(&self) -> Vec<Self::Version>;

    /// Recovered files require trusted metadata from the distribution, never a local sidecar.
    /// Fetchers without that metadata reject reuse and download through their normal path.
    async fn validate_cached(&self, _ver: &Self::Version, _path: &Path) -> Result<(), FetchError> {
        Err(FetchError::Validation(anyhow::anyhow!(
            "fetcher cannot authenticate a recovered compiler"
        )))
    }
}

pub trait Version:
    Clone + Debug + Display + FromStr + PartialEq + Eq + Hash + Send + Sync + 'static
{
    fn to_semver(&self) -> &semver::Version;
}

impl Version for semver::Version {
    fn to_semver(&self) -> &semver::Version {
        self
    }
}

fn unsafe_cache(message: &str) -> std::io::Error {
    std::io::Error::new(std::io::ErrorKind::PermissionDenied, message)
}

/// The configured cache and its parents must belong to the service account. Cache directories
/// are private (0700); a directory writable by peers or a symlink is rejected, never repaired.
fn private_directory(path: &Path) -> std::io::Result<()> {
    DirBuilder::new().recursive(true).mode(0o700).create(path)?;
    let metadata = std::fs::symlink_metadata(path)?;
    if !metadata.is_dir() || metadata.permissions().mode() & 0o022 != 0 {
        return Err(unsafe_cache(
            "compiler cache must be a directory not writable by peers",
        ));
    }
    // A create-new file gives the effective service owner without platform-specific UID calls.
    let probe = tempfile::Builder::new()
        .prefix(".owner-")
        .tempfile_in(path)?;
    if metadata.uid() != probe.as_file().metadata()?.uid() {
        return Err(unsafe_cache(
            "compiler cache must be owned by the service account",
        ));
    }
    std::fs::set_permissions(path, Permissions::from_mode(0o700))?;
    Ok(())
}

fn check_file(path: &Path, file: &File) -> std::io::Result<()> {
    let named = std::fs::symlink_metadata(path)?;
    let opened = file.metadata()?;
    if !named.is_file()
        || named.dev() != opened.dev()
        || named.ino() != opened.ino()
        || named.nlink() != 1
        || named.permissions().mode() & 0o077 != 0
        || named.permissions().mode() & 0o100 == 0
    {
        return Err(unsafe_cache(
            "compiler must be a private regular executable",
        ));
    }
    Ok(())
}

fn checksum_file(file: &mut File, expected: H256, limit: usize) -> Result<(), FetchError> {
    file.seek(SeekFrom::Start(0))?;
    let mut hasher = Sha256::new();
    let mut count = 0usize;
    let mut buffer = [0; 16 * 1024];
    loop {
        let read = file.read(&mut buffer)?;
        if read == 0 {
            break;
        }
        if read > limit.saturating_sub(count) {
            return Err(FetchError::BodyTooLarge { limit });
        }
        count += read;
        hasher.update(&buffer[..read]);
    }
    let found = H256::from_slice(&hasher.finalize());
    if found != expected {
        return Err(Mismatch::new(expected, found).into());
    }
    Ok(())
}

pub(super) async fn validate_cached_file<Ver: Version>(
    ver: &Ver,
    path: &Path,
    root: &Path,
    expected: H256,
    limit: usize,
    validator: Option<&dyn FileValidator<Ver>>,
) -> Result<(), FetchError> {
    let expected_path = root.join(ver.to_string()).join("solc");
    if path != expected_path {
        return Err(FetchError::Validation(anyhow::anyhow!(
            "unexpected compiler cache path"
        )));
    }
    let path_owned = path.to_path_buf();
    let root = root.to_path_buf();
    let mut file = tokio::task::spawn_blocking(move || {
        let root_meta = std::fs::symlink_metadata(&root)?;
        let folder_meta = std::fs::symlink_metadata(path_owned.parent().unwrap())?;
        for metadata in [&root_meta, &folder_meta] {
            if !metadata.is_dir() || metadata.permissions().mode() & 0o077 != 0 {
                return Err(FetchError::File(unsafe_cache(
                    "recovered cache directory is not private",
                )));
            }
        }
        let metadata = std::fs::symlink_metadata(&path_owned)?;
        // Check the type before open: a FIFO/device must never block recovery.
        if !metadata.is_file()
            || metadata.uid() != root_meta.uid()
            || folder_meta.uid() != root_meta.uid()
        {
            return Err(FetchError::File(unsafe_cache(
                "unsafe recovered compiler type or owner",
            )));
        }
        let mut file = File::open(&path_owned)?;
        check_file(&path_owned, &file)?;
        checksum_file(&mut file, expected, limit)?;
        Ok::<_, FetchError>(file)
    })
    .await??;
    if let Some(validator) = validator {
        validator
            .validate(ver, path)
            .await
            .map_err(FetchError::Validation)?;
    }
    let path = path.to_path_buf();
    tokio::task::spawn_blocking(move || {
        check_file(&path, &file)?;
        checksum_file(&mut file, expected, limit)
    })
    .await?
}

#[instrument(skip(bytes), level = "debug")]
pub fn validate_checksum(bytes: &Bytes, expected: H256) -> Result<(), Mismatch<H256>> {
    let found = Sha256::digest(bytes);
    let found = H256::from_slice(&found);
    if expected != found {
        Err(Mismatch::new(expected, found))
    } else {
        Ok(())
    }
}

pub async fn write_executable<Ver: Version>(
    data: Bytes,
    sha: H256,
    path: &Path,
    ver: &Ver,
    validator: Option<&dyn FileValidator<Ver>>,
) -> Result<PathBuf, FetchError> {
    validate_checksum(&data, sha)?;
    let version = ver.to_string();
    if Path::new(&version).components().count() != 1
        || !matches!(
            Path::new(&version).components().next(),
            Some(std::path::Component::Normal(_))
        )
    {
        return Err(FetchError::Validation(anyhow::anyhow!(
            "unsafe compiler version path"
        )));
    }
    let root = path.to_path_buf();
    let folder = root.join(version);
    let file = folder.join("solc");
    let limit = data.len();
    let mut temporary = tokio::task::spawn_blocking(move || {
        private_directory(&root)?;
        private_directory(&folder)?;
        let mut temporary = tempfile::Builder::new()
            .prefix(".solc-")
            .tempfile_in(&folder)?;
        temporary
            .as_file()
            .set_permissions(Permissions::from_mode(0o700))?;
        temporary.write_all(&data)?;
        temporary.as_file().sync_all()?;
        check_file(temporary.path(), temporary.as_file())?;
        checksum_file(temporary.as_file_mut(), sha, limit)?;
        Ok::<_, FetchError>(temporary)
    })
    .await??;

    if let Some(validator) = validator {
        validator
            .validate(ver, temporary.path())
            .await
            .map_err(FetchError::Validation)?;
    }

    let result = file.clone();
    tokio::task::spawn_blocking(move || {
        check_file(temporary.path(), temporary.as_file())?;
        checksum_file(temporary.as_file_mut(), sha, limit)?;
        temporary
            .persist(&file)
            .map_err(|error| FetchError::File(error.error))?;
        File::open(file.parent().unwrap())?.sync_all()?;
        Ok::<_, FetchError>(())
    })
    .await??;
    Ok(result)
}

#[cfg(test)]
mod tests {
    use super::{super::version_detailed as evm_version, *};
    use std::str::FromStr;

    #[tokio::test]
    async fn write_text_executable() {
        let tmp_dir = tempfile::tempdir().unwrap();

        let data = "this is a compiler binary";
        let bytes = Bytes::from_static(data.as_bytes());
        let sha = Sha256::digest(data.as_bytes());

        let version = evm_version::DetailedVersion::from_str("v0.4.10+commit.f0d539ae").unwrap();
        let file = write_executable(
            bytes,
            H256::from_slice(&sha),
            tmp_dir.path(),
            &version,
            None,
        )
        .await
        .unwrap();

        let content = tokio::fs::read_to_string(file).await.unwrap();
        assert_eq!(data, content);
    }

    #[tokio::test]
    async fn wrong_file_checksum() {
        let tmp_dir = tempfile::tempdir().unwrap();

        let data = "this is a compiler binary";
        let bytes = Bytes::from_static(data.as_bytes());
        let sha = H256::default();
        let version = evm_version::DetailedVersion::from_str("v0.4.10+commit.f0d539ae").unwrap();

        let err = write_executable(bytes, sha, tmp_dir.path(), &version, None)
            .await
            .expect_err("expected to fail with wrong checksum");
        assert!(matches!(err, FetchError::HashMismatch(_)));

        let dir = tmp_dir.path().join(version.to_string());
        let file = dir.join("solc");
        let mut tmp_file = file.clone();
        tmp_file.set_extension("tmp");
        assert!(!file.exists());
        assert!(
            !tmp_file.exists(),
            "failed downloads must leave no temporary executable"
        );
        assert_eq!(std::fs::read_dir(tmp_dir.path()).unwrap().count(), 0);
    }
}
