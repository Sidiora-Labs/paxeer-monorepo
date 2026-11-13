use std::fs::{self, DirBuilder, File, Metadata, OpenOptions};
use std::io::{self, Read, Write};
use std::os::fd::AsRawFd;
use std::os::unix::fs::{DirBuilderExt, MetadataExt, OpenOptionsExt, PermissionsExt};
use std::path::{Component, Path, PathBuf};

use super::STORE_FILE;

// Linux open flags, also used by the daemon's protected read-only snapshot path.
const NO_FOLLOW: i32 = 0o400_000;
const DIRECTORY: i32 = 0o200_000;
const NONBLOCK: i32 = 0o4000;

#[derive(Debug)]
pub(super) struct ProtectedRoot {
    path: PathBuf,
    directory: File,
    uid: u32,
}

fn refused(reason: &'static str) -> io::Error {
    io::Error::new(io::ErrorKind::PermissionDenied, reason)
}

fn same_file(left: &Metadata, right: &Metadata) -> bool {
    left.dev() == right.dev() && left.ino() == right.ino()
}

impl ProtectedRoot {
    pub(super) fn open(root: &Path, writable: bool) -> io::Result<Self> {
        let path = if root.is_absolute() {
            root.to_path_buf()
        } else {
            std::env::current_dir()?.join(root)
        };
        let mut ancestor = PathBuf::new();
        for component in path.components() {
            if matches!(component, Component::ParentDir) {
                return Err(refused("store root is not canonical"));
            }
            ancestor.push(component);
            match fs::symlink_metadata(&ancestor) {
                Ok(metadata) if !metadata.is_dir() => {
                    return Err(refused("store root contains a non-directory or symlink"));
                }
                Ok(_) => {}
                Err(error) if error.kind() == io::ErrorKind::NotFound && writable => {
                    DirBuilder::new().mode(0o700).create(&ancestor)?;
                }
                Err(error) => return Err(error),
            }
        }
        let directory = OpenOptions::new()
            .read(true)
            .custom_flags(NO_FOLLOW | DIRECTORY)
            .open(&path)?;
        let root = Self {
            path,
            directory,
            uid: rustix::process::geteuid().as_raw(),
        };
        root.validate()?;
        if writable {
            root.directory.try_lock().map_err(|error| match error {
                std::fs::TryLockError::WouldBlock => {
                    io::Error::new(io::ErrorKind::WouldBlock, "store root already has a writer")
                }
                std::fs::TryLockError::Error(error) => error,
            })?;
            root.validate()?;
        }
        Ok(root)
    }

    pub(super) fn path(&self) -> &Path {
        &self.path
    }

    fn anchored(&self, name: &str) -> PathBuf {
        PathBuf::from(format!("/proc/self/fd/{}", self.directory.as_raw_fd())).join(name)
    }

    fn validate(&self) -> io::Result<()> {
        let held = self.directory.metadata()?;
        let current = fs::symlink_metadata(&self.path)?;
        if !current.is_dir()
            || !same_file(&held, &current)
            || current.uid() != self.uid
            || current.mode() & 0o7777 != 0o700
            || fs::canonicalize(&self.path)? != self.path
        {
            return Err(refused(
                "store root is not a canonical owned private directory",
            ));
        }
        Ok(())
    }

    fn validate_file(&self, metadata: &Metadata) -> io::Result<()> {
        if !metadata.is_file()
            || metadata.uid() != self.uid
            || metadata.nlink() != 1
            || metadata.mode() & 0o7177 != 0
        {
            return Err(refused(
                "store snapshot is not an owned private regular file",
            ));
        }
        Ok(())
    }

    fn snapshot(&self) -> io::Result<Option<File>> {
        self.validate()?;
        let path = self.anchored(STORE_FILE);
        match fs::symlink_metadata(&path) {
            Ok(metadata) => self.validate_file(&metadata)?,
            Err(error) if error.kind() == io::ErrorKind::NotFound => return Ok(None),
            Err(error) => return Err(error),
        }
        let file = OpenOptions::new()
            .read(true)
            .custom_flags(NO_FOLLOW | NONBLOCK)
            .open(&path)?;
        let metadata = file.metadata()?;
        self.validate_file(&metadata)?;
        if !same_file(&metadata, &fs::symlink_metadata(path)?) {
            return Err(refused("store snapshot was substituted"));
        }
        Ok(Some(file))
    }

    pub(super) fn validate_snapshot(&self) -> io::Result<()> {
        self.snapshot().map(|_| ())
    }

    pub(super) fn read(&self) -> io::Result<Option<Vec<u8>>> {
        let Some(mut file) = self.snapshot()? else {
            return Ok(None);
        };
        let mut bytes = Vec::new();
        file.read_to_end(&mut bytes)?;
        Ok(Some(bytes))
    }

    pub(super) fn write(&self, bytes: &[u8]) -> io::Result<()> {
        self.validate()?;
        // Predictable paths used by older versions are never followed or reused.
        for name in ["store.bin.tmp", "store.bin.migrating"] {
            match fs::symlink_metadata(self.anchored(name)) {
                Ok(_) => return Err(refused("unexpected legacy store temporary path")),
                Err(error) if error.kind() == io::ErrorKind::NotFound => {}
                Err(error) => return Err(error),
            }
        }
        let _previous = self.snapshot()?;
        let mut random = [0_u8; 16];
        getrandom::fill(&mut random).map_err(|error| {
            io::Error::other(format!("store temporary randomness failed: {error}"))
        })?;
        let suffix = u128::from_ne_bytes(random);
        let path = self.anchored(&format!(".store.bin.tmp-{suffix:032x}"));
        let mut file = OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .custom_flags(NO_FOLLOW | NONBLOCK)
            .open(&path)?;
        let temporary = Temporary { path };
        file.set_permissions(fs::Permissions::from_mode(0o600))?;
        self.validate_file(&file.metadata()?)?;
        file.write_all(bytes)?;
        file.sync_all()?;
        self.validate()?;
        let metadata = fs::symlink_metadata(&temporary.path)?;
        self.validate_file(&metadata)?;
        if !same_file(&file.metadata()?, &metadata) {
            return Err(refused("store temporary file was substituted"));
        }
        let _previous = self.snapshot()?;
        fs::rename(&temporary.path, self.anchored(STORE_FILE))?;
        self.directory.sync_all()?;
        Ok(())
    }
}

struct Temporary {
    path: PathBuf,
}

impl Drop for Temporary {
    fn drop(&mut self) {
        let _ = fs::remove_file(&self.path);
    }
}
