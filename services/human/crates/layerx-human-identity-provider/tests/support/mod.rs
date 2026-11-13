#[path = "../../../../../platform/hosted/runtime-clock/src/server.rs"]
#[allow(dead_code)]
mod clock_server;

use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use std::cell::RefCell;
use std::io;
use std::process::Command;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::thread::JoinHandle;
use std::time::Duration;

struct ClockAuthority {
    binding: ClockBinding,
    shutdown: Arc<AtomicBool>,
    worker: Option<JoinHandle<io::Result<()>>>,
    _directory: tempfile::TempDir,
}

impl ClockAuthority {
    fn start() -> io::Result<Self> {
        let directory = tempfile::tempdir()?;
        let mut server = clock_server::Server::bind(directory.path())?;
        let binding = ClockBinding {
            socket: server.socket().to_owned(),
            process: std::process::id(),
            user: rustix::process::geteuid().as_raw(),
        };
        let shutdown = Arc::new(AtomicBool::new(false));
        let flag = Arc::clone(&shutdown);
        let worker = std::thread::spawn(move || {
            while !flag.load(Ordering::Acquire) {
                server.poll()?;
                std::thread::sleep(Duration::from_millis(1));
            }
            Ok(())
        });
        Ok(Self {
            binding,
            shutdown,
            worker: Some(worker),
            _directory: directory,
        })
    }
}

impl Drop for ClockAuthority {
    fn drop(&mut self) {
        self.shutdown.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let result = worker
                .join()
                .unwrap_or_else(|_| panic!("runtime clock authority panicked"));
            assert!(result.is_ok(), "runtime clock authority failed: {result:?}");
        }
    }
}

thread_local! {
    static AUTHORITY: RefCell<Option<ClockAuthority>> = const { RefCell::new(None) };
}

fn binding() -> io::Result<ClockBinding> {
    AUTHORITY.with(|authority| {
        let mut authority = authority.borrow_mut();
        if authority.is_none() {
            *authority = Some(ClockAuthority::start()?);
        }
        Ok(authority
            .as_ref()
            .unwrap_or_else(|| panic!("clock authority initialized"))
            .binding
            .clone())
    })
}

pub fn clock() -> io::Result<Arc<RuntimeClock>> {
    RuntimeClock::connect(binding()?).map_err(io::Error::other)
}

pub fn command() -> io::Result<Command> {
    let binding = binding()?;
    let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-human-identity-provider"));
    command
        .env("LAYERX_RUNTIME_CLOCK_SOCKET", binding.socket)
        .env("LAYERX_RUNTIME_CLOCK_PID", binding.process.to_string())
        .env("LAYERX_RUNTIME_CLOCK_UID", binding.user.to_string());
    Ok(command)
}
