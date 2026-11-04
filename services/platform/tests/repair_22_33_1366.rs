use std::io::{self, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::process::Command;
use std::time::{Duration, Instant};

#[path = "support/probe_process.rs"]
mod probe_process;

const CHILD_CASE: &str = "LAYERX_TLS_PROBE_REPAIR_CASE";

fn is_child(name: &str) -> bool {
    std::env::var(CHILD_CASE).as_deref() == Ok(name)
}

fn probe_command(name: &str) -> Command {
    let mut command =
        Command::new(std::env::current_exe().expect("running integration executable"));
    command
        .args(["--exact", name, "--nocapture", "--test-threads=1"])
        .env(CHILD_CASE, name);
    command
}

fn accept(listener: &TcpListener) -> TcpStream {
    listener
        .set_nonblocking(true)
        .expect("nonblocking listener");
    let deadline = Instant::now() + Duration::from_secs(5);
    loop {
        match listener.accept() {
            Ok((stream, _)) => {
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .expect("server read deadline");
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .expect("server write deadline");
                return stream;
            }
            Err(error) if error.kind() == io::ErrorKind::WouldBlock => {
                assert!(Instant::now() < deadline, "probe never connected");
                std::thread::sleep(Duration::from_millis(2));
            }
            Err(error) => panic!("loopback accept: {error}"),
        }
    }
}

fn assert_reaped(error: &probe_process::ProbeError) {
    let output = error.output.as_ref().expect("cleanup returns wait status");
    assert!(!output.status.success(), "running probe must be killed");
    let process_id = error.process_id.expect("spawned probe PID");
    #[cfg(target_os = "linux")]
    assert!(
        !std::path::Path::new(&format!("/proc/{process_id}")).exists(),
        "killed probe must be reaped, not left as a zombie"
    );
}

#[test]
fn tls_probe_deadline_kills_and_reaps_stalled_client() {
    let name = "tls_probe_deadline_kills_and_reaps_stalled_client";
    if is_child(name) {
        let mut stream =
            TcpStream::connect(std::env::var("LAYERX_TLS_PROBE_REPAIR_PEER").expect("peer"))
                .expect("connect to real loopback peer");
        println!("probe connected");
        eprintln!("waiting for peer response");
        stream.write_all(b"request").expect("send real request");
        let mut reply = [0];
        stream
            .read_exact(&mut reply)
            .expect("peer stalls this read");
        panic!("stalled probe unexpectedly returned");
    }
    let listener = TcpListener::bind("127.0.0.1:0").expect("loopback fixture");
    let mut command = probe_command(name);
    command.env(
        "LAYERX_TLS_PROBE_REPAIR_PEER",
        listener.local_addr().expect("bound address").to_string(),
    );
    let server = std::thread::spawn(move || {
        let mut stream = accept(&listener);
        let mut request = [0; 7];
        stream
            .read_exact(&mut request)
            .expect("receive probe request");
        assert_eq!(&request, b"request");
        assert_eq!(
            stream
                .read(&mut [0])
                .expect("killed client closes transport"),
            0
        );
    });
    let started = Instant::now();
    let error = probe_process::run_probe(
        &mut command,
        Duration::from_secs(1),
        probe_process::PROBE_OUTPUT_LIMIT,
    )
    .expect_err("stalled client must fail the deadline");
    assert_eq!(error.kind, io::ErrorKind::TimedOut);
    assert!(started.elapsed() < Duration::from_secs(5));
    assert_reaped(&error);
    let output = error.output.as_ref().expect("bounded diagnostic output");
    assert!(String::from_utf8_lossy(&output.stdout).contains("probe connected"));
    assert!(String::from_utf8_lossy(&output.stderr).contains("waiting for peer response"));
    assert!(error.to_string().contains("deadline exceeded"));
    server.join().expect("real peer observed transport cleanup");
}

fn output_limit_case(name: &str, stderr: bool) {
    if is_child(name) {
        let block = [b'x'; 4096];
        let mut writer: Box<dyn Write> = if stderr {
            Box::new(io::stderr().lock())
        } else {
            Box::new(io::stdout().lock())
        };
        loop {
            writer.write_all(&block).expect("write probe output");
            writer.flush().expect("flush probe output");
        }
    }
    let mut command = probe_command(name);
    let started = Instant::now();
    let error = probe_process::run_probe(
        &mut command,
        probe_process::PROBE_DEADLINE,
        probe_process::PROBE_OUTPUT_LIMIT,
    )
    .expect_err("unbounded output must fail");
    assert_eq!(error.kind, io::ErrorKind::Other);
    assert!(error.to_string().contains("output limit exceeded"));
    assert!(started.elapsed() < Duration::from_secs(5));
    assert_reaped(&error);
    let output = error.output.as_ref().expect("bounded captured bytes");
    assert_eq!(
        output.stdout.len() + output.stderr.len(),
        probe_process::PROBE_OUTPUT_LIMIT
    );
    let noisy = if stderr {
        &output.stderr
    } else {
        &output.stdout
    };
    assert!(noisy.windows(4096).any(|window| window == [b'x'; 4096]));
}

#[test]
fn tls_probe_stdout_limit_kills_and_reaps_client() {
    output_limit_case("tls_probe_stdout_limit_kills_and_reaps_client", false);
}

#[test]
fn tls_probe_stderr_limit_kills_and_reaps_client() {
    output_limit_case("tls_probe_stderr_limit_kills_and_reaps_client", true);
}

#[test]
fn tls_probe_captures_real_client_response_and_both_outputs() {
    let name = "tls_probe_captures_real_client_response_and_both_outputs";
    if is_child(name) {
        let mut stream =
            TcpStream::connect(std::env::var("LAYERX_TLS_PROBE_REPAIR_PEER").expect("peer"))
                .expect("real loopback connection");
        stream.write_all(b"GET /livez\r\n").expect("request");
        let mut response = String::new();
        stream.read_to_string(&mut response).expect("response");
        assert_eq!(response, "real peer response");
        println!("TLS client probe completed: {response}");
        eprintln!("probe diagnostic");
        return;
    }
    let listener = TcpListener::bind("127.0.0.1:0").expect("loopback fixture");
    let mut command = probe_command(name);
    command.env(
        "LAYERX_TLS_PROBE_REPAIR_PEER",
        listener.local_addr().expect("bound address").to_string(),
    );
    let server = std::thread::spawn(move || {
        let mut stream = accept(&listener);
        let mut request = [0; 12];
        stream.read_exact(&mut request).expect("request");
        assert_eq!(&request, b"GET /livez\r\n");
        stream.write_all(b"real peer response").expect("response");
    });
    let output = probe_process::run_probe(
        &mut command,
        Duration::from_secs(5),
        probe_process::PROBE_OUTPUT_LIMIT,
    )
    .expect("successful real child probe");
    assert!(output.status.success());
    assert!(String::from_utf8_lossy(&output.stdout)
        .contains("TLS client probe completed: real peer response"));
    assert!(String::from_utf8_lossy(&output.stderr).contains("probe diagnostic"));
    server.join().expect("real peer completed");
}

#[test]
fn tls_probe_preserves_failed_client_status_and_diagnostics() {
    let name = "tls_probe_preserves_failed_client_status_and_diagnostics";
    if is_child(name) {
        io::stdout()
            .write_all(b"client assertion failed\n")
            .expect("stdout");
        io::stderr()
            .write_all(b"certificate rejected\n")
            .expect("stderr");
        std::process::exit(23);
    }
    let output = probe_process::run_probe(
        &mut probe_command(name),
        Duration::from_secs(5),
        probe_process::PROBE_OUTPUT_LIMIT,
    )
    .expect("completed process preserves its failed status");
    assert_eq!(output.status.code(), Some(23));
    assert!(String::from_utf8_lossy(&output.stdout).contains("client assertion failed"));
    assert!(String::from_utf8_lossy(&output.stderr).contains("certificate rejected"));
}

#[test]
fn tls_probe_spawn_failure_is_reported() {
    let mut command = Command::new("/dev/null/not-a-probe-executable");
    let error = probe_process::run_probe(
        &mut command,
        Duration::from_secs(5),
        probe_process::PROBE_OUTPUT_LIMIT,
    )
    .expect_err("invalid executable must fail, never silently fall back");
    assert!(error.process_id.is_none());
    assert!(error.output.is_none());
    assert!(!error.to_string().is_empty());
}
