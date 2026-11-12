// The included runtime-clock/src/server.rs is shared with another test module.
#[allow(clippy::duplicate_mod)]
#[path = "../../../../../platform/hosted/runtime-clock/src/server.rs"]
mod authority;

use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use std::path::Path;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::thread::JoinHandle;
use std::time::Duration;

pub struct ClockAuthority {
    pub binding: ClockBinding,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<std::io::Result<()>>>,
}
impl ClockAuthority {
    /// Starts a runtime clock for the private socket directory.
    ///
    /// # Errors
    /// Returns an error if the clock socket cannot be opened.
    pub fn start(root: &Path) -> std::io::Result<Self> {
        let mut server = authority::Server::bind(root)?;
        let binding = ClockBinding {
            socket: server.socket().to_owned(),
            process: std::process::id(),
            user: rustix::process::geteuid().as_raw(),
        };
        let stop = Arc::new(AtomicBool::new(false));
        let flag = stop.clone();
        let worker = std::thread::spawn(move || {
            while !flag.load(Ordering::Acquire) {
                server.poll()?;
                std::thread::sleep(Duration::from_millis(1));
            }
            Ok(())
        });
        Ok(Self {
            binding,
            stop,
            worker: Some(worker),
        })
    }
    /// Connects a clock client to the running authority.
    ///
    /// # Panics
    /// Panics if the local clock client cannot connect.
    #[must_use]
    pub fn connect(&self) -> Arc<RuntimeClock> {
        RuntimeClock::connect(self.binding.clone()).or_fail("authenticated runtime clock")
    }
}
impl Drop for ClockAuthority {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.worker
            .take()
            .or_fail("clock worker")
            .join()
            .or_fail("clock worker panicked")
            .or_fail("clock authority IO");
    }
}

use crate::require_value::RequireValue as _;
