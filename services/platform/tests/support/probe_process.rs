use std::fmt;
use std::io::{self, Read};
use std::os::fd::OwnedFd;
use std::os::unix::net::UnixStream;
use std::process::{Child, Command, Output, Stdio};
use std::time::{Duration, Instant};

pub const PROBE_DEADLINE: Duration = Duration::from_secs(30);
pub const PROBE_OUTPUT_LIMIT: usize = 64 * 1024;

#[derive(Debug)]
pub struct ProbeError {
    pub kind: io::ErrorKind,
    pub process_id: Option<u32>,
    pub output: Option<Output>,
    message: String,
}

impl From<io::Error> for ProbeError {
    fn from(error: io::Error) -> Self {
        Self {
            kind: error.kind(),
            process_id: None,
            output: None,
            message: error.to_string(),
        }
    }
}

impl fmt::Display for ProbeError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            formatter,
            "{} ({:?}, pid {:?})",
            self.message, self.kind, self.process_id
        )?;
        if let Some(output) = &self.output {
            write!(
                formatter,
                ": {} {}",
                String::from_utf8_lossy(&output.stdout),
                String::from_utf8_lossy(&output.stderr)
            )?;
        }
        Ok(())
    }
}

impl std::error::Error for ProbeError {}

struct ProbeChild(Child);

impl Drop for ProbeChild {
    fn drop(&mut self) {
        let _ = self.0.kill();
        let _ = self.0.wait();
    }
}

fn capture(
    stream: &mut UnixStream,
    bytes: &mut Vec<u8>,
    captured: &mut usize,
    maximum: usize,
) -> io::Result<bool> {
    let remaining = maximum.saturating_sub(*captured);
    let mut buffer = [0; 4096];
    let count = buffer.len().min(remaining.saturating_add(1));
    match stream.read(&mut buffer[..count]) {
        Ok(0) => Ok(true),
        Ok(count) => {
            let retained = remaining.min(count);
            bytes.extend_from_slice(&buffer[..retained]);
            *captured += retained;
            if count > remaining {
                return Err(io::Error::other("TLS probe output limit exceeded"));
            }
            Ok(false)
        }
        Err(error)
            if matches!(
                error.kind(),
                io::ErrorKind::WouldBlock | io::ErrorKind::Interrupted
            ) =>
        {
            Ok(false)
        }
        Err(error) => Err(error),
    }
}

pub fn run_probe(
    command: &mut Command,
    timeout: Duration,
    maximum_output: usize,
) -> Result<Output, ProbeError> {
    let started = Instant::now();
    let (mut stdout, child_stdout) = UnixStream::pair()?;
    let (mut stderr, child_stderr) = UnixStream::pair()?;
    stdout.set_nonblocking(true)?;
    stderr.set_nonblocking(true)?;
    command
        .stdin(Stdio::null())
        .stdout(Stdio::from(OwnedFd::from(child_stdout)))
        .stderr(Stdio::from(OwnedFd::from(child_stderr)));
    // Command retains its configured descriptors, so release them after spawning.
    let spawned = command.spawn();
    command.stdout(Stdio::null()).stderr(Stdio::null());
    let mut child = ProbeChild(spawned?);
    let process_id = child.0.id();
    let mut out = Vec::new();
    let mut err = Vec::new();
    let mut captured = 0;
    let mut stdout_closed = false;
    let mut stderr_closed = false;
    let failure = loop {
        if started.elapsed() >= timeout {
            break io::Error::new(io::ErrorKind::TimedOut, "TLS probe deadline exceeded");
        }
        if !stdout_closed {
            match capture(&mut stdout, &mut out, &mut captured, maximum_output) {
                Ok(closed) => stdout_closed = closed,
                Err(error) => break error,
            }
        }
        if !stderr_closed {
            match capture(&mut stderr, &mut err, &mut captured, maximum_output) {
                Ok(closed) => stderr_closed = closed,
                Err(error) => break error,
            }
        }
        match child.0.try_wait() {
            Ok(Some(status)) if stdout_closed && stderr_closed => {
                return Ok(Output {
                    status,
                    stdout: out,
                    stderr: err,
                });
            }
            Ok(_) => {}
            Err(error) => break error,
        }
        // Read each stream once per iteration so neither output can starve the deadline.
        std::thread::sleep(Duration::from_millis(2).min(timeout.saturating_sub(started.elapsed())));
    };
    if child.0.try_wait()?.is_none() {
        child.0.kill()?;
    }
    let status = child.0.wait()?;
    Err(ProbeError {
        kind: failure.kind(),
        process_id: Some(process_id),
        output: Some(Output {
            status,
            stdout: out,
            stderr: err,
        }),
        message: failure.to_string(),
    })
}
