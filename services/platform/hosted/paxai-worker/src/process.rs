//! Bounded process calls with cancellable, nonblocking input and output.
use std::io::{self, Read, Write};
use std::net::Shutdown;
use std::os::fd::OwnedFd;
use std::os::unix::{net::UnixStream, process::CommandExt};
use std::process::{Child, Command, ExitStatus, Stdio};
use std::time::{Duration, Instant};

#[derive(Debug)]
pub enum CallError {
    NotStarted,
    Ambiguous,
}

struct Invocation(Child);
impl Drop for Invocation {
    fn drop(&mut self) {
        let _ = self.0.kill();
        let _ = self.0.wait();
    }
}

/// One deadline covers spawn, input delivery, process exit and output EOF.
/// No I/O threads or pipe handles survive the call.
pub fn invoke(
    command: &mut Command,
    input: &[u8],
    limit: usize,
    timeout: Duration,
) -> Result<(ExitStatus, Vec<u8>), CallError> {
    let deadline = Instant::now()
        .checked_add(timeout)
        .ok_or(CallError::NotStarted)?;
    let (mut writer, child_input) = UnixStream::pair().map_err(|_| CallError::NotStarted)?;
    let (mut reader, child_output) = UnixStream::pair().map_err(|_| CallError::NotStarted)?;
    writer
        .set_nonblocking(true)
        .map_err(|_| CallError::NotStarted)?;
    reader
        .set_nonblocking(true)
        .map_err(|_| CallError::NotStarted)?;
    command
        .process_group(0)
        .stdin(Stdio::from(OwnedFd::from(child_input)))
        .stdout(Stdio::from(OwnedFd::from(child_output)))
        .stderr(Stdio::null());
    let mut child = Invocation(command.spawn().map_err(|_| CallError::NotStarted)?);
    // Command owns its configured descriptors too; release those copies after spawn.
    command.stdin(Stdio::null()).stdout(Stdio::null());
    let mut sent = 0;
    let mut input_closed = false;
    let mut eof = false;
    let mut status = None;
    let mut bytes = Vec::new();
    loop {
        if Instant::now() >= deadline {
            return Err(CallError::Ambiguous);
        }
        if !input_closed {
            match writer.write(&input[sent..]) {
                Ok(count) => sent += count,
                Err(error)
                    if matches!(
                        error.kind(),
                        io::ErrorKind::WouldBlock | io::ErrorKind::Interrupted
                    ) => {}
                Err(_) => return Err(CallError::Ambiguous),
            }
            if sent == input.len() {
                writer
                    .shutdown(Shutdown::Write)
                    .map_err(|_| CallError::Ambiguous)?;
                input_closed = true;
            }
        }
        if !eof {
            let mut chunk = [0; 8192];
            match reader.read(&mut chunk) {
                Ok(0) => eof = true,
                Ok(count) => {
                    if count > limit.saturating_sub(bytes.len()) {
                        return Err(CallError::Ambiguous);
                    }
                    bytes.extend_from_slice(&chunk[..count]);
                }
                Err(error)
                    if matches!(
                        error.kind(),
                        io::ErrorKind::WouldBlock | io::ErrorKind::Interrupted
                    ) => {}
                Err(_) => return Err(CallError::Ambiguous),
            }
        }
        if status.is_none() {
            status = child.0.try_wait().map_err(|_| CallError::Ambiguous)?;
        }
        if let Some(status) = status {
            if eof && input_closed {
                return Ok((status, bytes));
            }
        }
        std::thread::sleep(
            Duration::from_millis(2).min(deadline.saturating_duration_since(Instant::now())),
        );
    }
}
