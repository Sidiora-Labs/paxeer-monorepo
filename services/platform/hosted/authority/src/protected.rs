use crate::AuthorityError;
use std::fs::{self, File, Metadata};
use std::io::Read;
use std::os::unix::fs::MetadataExt;
use std::path::Path;

/// # Errors
/// Returns an error when the file is not private, changes during reading, or exceeds the limit.
pub fn read(path: &Path, maximum: u64) -> crate::AuthorityResult<Vec<u8>> {
    if !path.is_absolute() || fs::canonicalize(path).map_err(|_| AuthorityError)? != path {
        return Err(AuthorityError);
    }
    let uid = fs::metadata("/proc/self")
        .map_err(|_| AuthorityError)?
        .uid();
    let before = fs::symlink_metadata(path).map_err(|_| AuthorityError)?;
    if !before.is_file() || before.uid() != uid || before.mode() & 0o077 != 0 {
        return Err(AuthorityError);
    }
    let mut file = File::open(path).map_err(|_| AuthorityError)?;
    let opened = file.metadata().map_err(|_| AuthorityError)?;
    if !same(&before, &opened) {
        return Err(AuthorityError);
    }
    let mut bytes = Vec::new();
    file.by_ref()
        .take(maximum.checked_add(1).ok_or(AuthorityError)?)
        .read_to_end(&mut bytes)
        .map_err(|_| AuthorityError)?;
    let after = file.metadata().map_err(|_| AuthorityError)?;
    let named = fs::symlink_metadata(path).map_err(|_| AuthorityError)?;
    if bytes.len() as u64 > maximum
        || !same(&opened, &after)
        || !same(&after, &named)
        || fs::canonicalize(path).map_err(|_| AuthorityError)? != path
    {
        return Err(AuthorityError);
    }
    Ok(bytes)
}

fn same(a: &Metadata, b: &Metadata) -> bool {
    a.dev() == b.dev()
        && a.ino() == b.ino()
        && a.uid() == b.uid()
        && a.gid() == b.gid()
        && a.mode() == b.mode()
        && a.nlink() == b.nlink()
        && a.size() == b.size()
        && a.mtime() == b.mtime()
        && a.mtime_nsec() == b.mtime_nsec()
        && a.ctime() == b.ctime()
        && a.ctime_nsec() == b.ctime_nsec()
}
