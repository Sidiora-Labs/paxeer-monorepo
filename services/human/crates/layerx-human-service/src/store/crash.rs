//! Thread-local crash boundaries for exercising the production storage protocol.

use std::cell::RefCell;
use std::ffi::OsString;
use std::io;

/// A completed filesystem operation in the durable publication protocol.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum DurabilityStep {
    DirectoryCreated(OsString),
    ParentSynced(OsString),
    TemporarySynced,
    Published,
    DirectorySynced,
}

#[derive(Default)]
struct Recording {
    stop_after: Option<usize>,
    steps: Vec<DurabilityStep>,
}

thread_local! {
    static RECORDING: RefCell<Option<Recording>> = const { RefCell::new(None) };
}

/// Records real operations and optionally interrupts after a numbered boundary.
/// Dropping the harness disarms it; no filesystem operations run on drop.
pub struct StorageCrashHarness {
    thread: std::thread::ThreadId,
    _not_send: std::marker::PhantomData<std::rc::Rc<()>>,
}

impl StorageCrashHarness {
    /// Arms this thread. `stop_after` is one-based; `None` only records.
    ///
    /// # Panics
    /// Panics if a harness is already armed on this thread or the boundary is zero.
    #[must_use]
    pub fn arm(stop_after: Option<usize>) -> Self {
        assert_ne!(stop_after, Some(0));
        RECORDING.with(|recording| {
            let mut recording = recording.borrow_mut();
            assert!(recording.is_none(), "storage crash harness already armed");
            *recording = Some(Recording {
                stop_after,
                steps: Vec::new(),
            });
        });
        Self {
            thread: std::thread::current().id(),
            _not_send: std::marker::PhantomData,
        }
    }

    /// Returns completed operations on this thread.
    ///
    /// # Panics
    /// Panics if the harness is read on another thread.
    #[must_use]
    pub fn steps(&self) -> Vec<DurabilityStep> {
        assert_eq!(std::thread::current().id(), self.thread);
        RECORDING.with(|recording| {
            recording
                .borrow()
                .as_ref()
                .map_or_else(Vec::new, |value| value.steps.clone())
        })
    }
}

impl Drop for StorageCrashHarness {
    fn drop(&mut self) {
        RECORDING.with(|recording| *recording.borrow_mut() = None);
    }
}

pub(crate) fn checkpoint(step: DurabilityStep) -> io::Result<()> {
    RECORDING.with(|recording| {
        if let Some(recording) = recording.borrow_mut().as_mut() {
            recording.steps.push(step);
            if recording.stop_after == Some(recording.steps.len()) {
                return Err(io::Error::other("storage crash boundary reached"));
            }
        }
        Ok(())
    })
}
