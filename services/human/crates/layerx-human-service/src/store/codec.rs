use std::ffi::{OsStr, OsString};
use std::fs::File;
use std::io::{self, Read as _, Write as _};
use std::os::unix::ffi::OsStringExt as _;
use std::os::unix::fs::MetadataExt as _;
use std::path::{Component, Path};
use std::sync::Arc;

use rustix::fs::{mkdirat, openat, renameat, Dir, Mode, OFlags};

use super::crash::{checkpoint, DurabilityStep};

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum CodecError {
    Truncated,
    Overflow,
}

pub(crate) fn push_length(output: &mut Vec<u8>, value: usize) -> Result<(), CodecError> {
    let value = u32::try_from(value).map_err(|_| CodecError::Overflow)?;
    output.extend_from_slice(&value.to_be_bytes());
    Ok(())
}

pub(crate) fn push_bytes(output: &mut Vec<u8>, bytes: &[u8]) -> Result<(), CodecError> {
    push_length(output, bytes.len())?;
    output.extend_from_slice(bytes);
    Ok(())
}

pub(crate) struct Decoder<'a> {
    bytes: &'a [u8],
    offset: usize,
}

impl<'a> Decoder<'a> {
    pub(crate) const fn new(bytes: &'a [u8]) -> Self {
        Self { bytes, offset: 0 }
    }

    pub(crate) fn take(&mut self, length: usize) -> Result<&'a [u8], CodecError> {
        let end = self
            .offset
            .checked_add(length)
            .ok_or(CodecError::Overflow)?;
        let value = self
            .bytes
            .get(self.offset..end)
            .ok_or(CodecError::Truncated)?;
        self.offset = end;
        Ok(value)
    }

    pub(crate) fn byte(&mut self) -> Result<u8, CodecError> {
        self.take(1)?.first().copied().ok_or(CodecError::Truncated)
    }

    pub(crate) fn u32(&mut self) -> Result<u32, CodecError> {
        let mut encoded = [0_u8; 4];
        encoded.copy_from_slice(self.take(4)?);
        Ok(u32::from_be_bytes(encoded))
    }

    pub(crate) fn u64(&mut self) -> Result<u64, CodecError> {
        let mut encoded = [0_u8; 8];
        encoded.copy_from_slice(self.take(8)?);
        Ok(u64::from_be_bytes(encoded))
    }

    pub(crate) fn length(&mut self) -> Result<usize, CodecError> {
        usize::try_from(self.u32()?).map_err(|_| CodecError::Overflow)
    }

    pub(crate) fn bytes(&mut self) -> Result<&'a [u8], CodecError> {
        let length = self.length()?;
        self.take(length)
    }

    pub(crate) const fn is_empty(&self) -> bool {
        self.offset == self.bytes.len()
    }
}

#[derive(Clone, Debug)]
pub(crate) struct Directory(Arc<File>);

impl Directory {
    pub(crate) fn open(path: &Path) -> io::Result<Self> {
        Self::open_path(path, true)
    }

    pub(crate) fn open_existing(path: &Path) -> io::Result<Self> {
        Self::open_path(path, false)
    }

    fn open_path(path: &Path, create: bool) -> io::Result<Self> {
        let start = if path.is_absolute() { "/" } else { "." };
        let mut directory = Self(Arc::new(File::from(openat(
            rustix::fs::CWD,
            start,
            OFlags::RDONLY | OFlags::DIRECTORY | OFlags::NOFOLLOW | OFlags::CLOEXEC,
            Mode::empty(),
        )?)));
        for component in path.components() {
            match component {
                Component::RootDir | Component::CurDir => (),
                Component::Normal(name) => {
                    directory = directory.child_inner(name, create, false)?
                }
                _ => return Err(invalid("store path contains a parent traversal")),
            }
        }
        directory.validate()?;
        Ok(directory)
    }

    fn validate(&self) -> io::Result<()> {
        let metadata = self.0.metadata()?;
        if !metadata.is_dir()
            || metadata.uid() != rustix::process::geteuid().as_raw()
            || metadata.nlink() < 2
            || metadata.mode() & 0o022 != 0
        {
            return Err(invalid("unprotected store directory"));
        }
        Ok(())
    }

    pub(crate) fn child(&self, name: impl AsRef<OsStr>, create: bool) -> io::Result<Self> {
        self.validate()?;
        self.child_inner(name.as_ref(), create, true)
    }

    fn child_inner(&self, name: &OsStr, create: bool, validate: bool) -> io::Result<Self> {
        let mut created = false;
        if create {
            match mkdirat(self.0.as_ref(), name, Mode::from_raw_mode(0o700)) {
                Ok(()) => {
                    created = true;
                    checkpoint(DurabilityStep::DirectoryCreated(name.to_os_string()))?;
                }
                Err(rustix::io::Errno::EXIST) => (),
                Err(error) => return Err(error.into()),
            }
        }
        let child = Self(Arc::new(File::from(openat(
            self.0.as_ref(),
            name,
            OFlags::RDONLY | OFlags::DIRECTORY | OFlags::NOFOLLOW | OFlags::CLOEXEC,
            Mode::empty(),
        )?)));
        if validate {
            child.validate()?;
        }
        // Also sync on retry: a prior attempt may have stopped after mkdir.
        if create {
            self.0.sync_all()?;
            if created {
                checkpoint(DurabilityStep::ParentSynced(name.to_os_string()))?;
            }
        }
        Ok(child)
    }

    pub(crate) fn entries(&self) -> io::Result<Vec<OsString>> {
        self.validate()?;
        let mut entries = Vec::new();
        for entry in Dir::read_from(self.0.as_ref())? {
            let entry = entry?;
            let name = entry.file_name().to_bytes();
            if name != b"." && name != b".." {
                entries.push(OsString::from_vec(name.to_vec()));
            }
        }
        Ok(entries)
    }

    pub(crate) fn open_file(&self, name: &str, flags: OFlags) -> io::Result<File> {
        self.validate()?;
        let file = File::from(openat(
            self.0.as_ref(),
            name,
            flags | OFlags::NOFOLLOW | OFlags::CLOEXEC | OFlags::NONBLOCK,
            Mode::from_raw_mode(0o600),
        )?);
        validate_file(&file)?;
        Ok(file)
    }

    pub(crate) fn read(&self, name: &str) -> io::Result<Option<Vec<u8>>> {
        let mut file = match self.open_file(name, OFlags::RDONLY) {
            Ok(file) => file,
            Err(error) if error.kind() == io::ErrorKind::NotFound => return Ok(None),
            Err(error) => return Err(error),
        };
        let mut bytes = Vec::new();
        file.read_to_end(&mut bytes)?;
        validate_file(&file)?;
        Ok(Some(bytes))
    }

    pub(crate) fn same_directory(&self, other: &Self) -> io::Result<bool> {
        let left = self.0.metadata()?;
        let right = other.0.metadata()?;
        Ok(left.dev() == right.dev() && left.ino() == right.ino())
    }
}

fn invalid(message: &'static str) -> io::Error {
    io::Error::new(io::ErrorKind::PermissionDenied, message)
}

fn validate_file(file: &File) -> io::Result<()> {
    let metadata = file.metadata()?;
    if !metadata.is_file()
        || metadata.uid() != rustix::process::geteuid().as_raw()
        || metadata.nlink() != 1
        || metadata.mode() & 0o022 != 0
    {
        return Err(invalid("unprotected store file"));
    }
    Ok(())
}

pub(crate) fn write_atomic(
    directory: &Directory,
    final_name: &str,
    temp_name: &str,
    bytes: &[u8],
) -> io::Result<()> {
    // Refuse aliases at both names, including a leftover temporary file.
    match directory.open_file(final_name, OFlags::RDONLY) {
        Ok(_) => (),
        Err(error) if error.kind() == io::ErrorKind::NotFound => (),
        Err(error) => return Err(error),
    }
    let mut file = directory.open_file(temp_name, OFlags::CREATE | OFlags::WRONLY)?;
    file.set_len(0)?;
    file.write_all(bytes)?;
    file.sync_all()?;
    checkpoint(DurabilityStep::TemporarySynced)?;
    renameat(
        directory.0.as_ref(),
        temp_name,
        directory.0.as_ref(),
        final_name,
    )?;
    checkpoint(DurabilityStep::Published)?;
    directory.0.sync_all()?;
    checkpoint(DurabilityStep::DirectorySynced)?;
    Ok(())
}
