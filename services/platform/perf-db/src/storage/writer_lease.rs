use std::fs::{File, OpenOptions};
use std::path::Path;

use crate::error::PerfDbError;

/// A kernel lock on a stable inode. Never unlink the lock file: a second inode
/// would allow another process to acquire a conflicting lease.
pub(crate) struct WriterLease {
    _file: File,
}

impl WriterLease {
    pub(crate) fn acquire(dir: &Path) -> Result<Self, PerfDbError> {
        let file = OpenOptions::new()
            .read(true)
            .write(true)
            .create(true)
            .truncate(false)
            .open(dir.join(".writer.lock"))
            .map_err(|e| super::wal::io_error("open database writer lease", e))?;
        #[cfg(unix)]
        {
            use std::os::fd::AsRawFd;
            unsafe extern "C" {
                fn flock(fd: std::ffi::c_int, operation: std::ffi::c_int) -> std::ffi::c_int;
            }
            // LOCK_EX | LOCK_NB. flock locks an open file description, so even
            // two separate opens in this process compete, as do other processes.
            if unsafe { flock(file.as_raw_fd(), 2 | 4) } != 0 {
                return Err(super::wal::io_error(
                    "database already has a writer (exclusive lease)",
                    std::io::Error::last_os_error(),
                ));
            }
        }
        #[cfg(not(unix))]
        return Err(PerfDbError::InvalidArgument(
            "database writer leases require a supported OS lock".into(),
        ));
        #[cfg(unix)]
        Ok(Self { _file: file })
    }
}
