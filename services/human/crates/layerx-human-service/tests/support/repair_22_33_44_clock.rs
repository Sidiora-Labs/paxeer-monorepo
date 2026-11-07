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
    pub fn connect(&self) -> Arc<RuntimeClock> {
        RuntimeClock::connect(self.binding.clone()).expect("authenticated runtime clock")
    }
    pub fn bind_command(&self, command: &mut std::process::Command) {
        command
            .env("LAYERX_RUNTIME_CLOCK_SOCKET", &self.binding.socket)
            .env("LAYERX_RUNTIME_CLOCK_PID", self.binding.process.to_string())
            .env("LAYERX_RUNTIME_CLOCK_UID", self.binding.user.to_string());
    }
}
impl Drop for ClockAuthority {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.worker
            .take()
            .expect("clock worker")
            .join()
            .expect("clock worker panicked")
            .expect("clock authority IO");
    }
}
