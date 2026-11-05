use std::io::{BufRead as _, BufReader, Read as _, Write as _};
use std::net::TcpListener;
use std::process::{Child, Command, Output, Stdio};
use std::time::{Duration, Instant};

use serde_json::{json, Value};

type TestResult = Result<(), Box<dyn std::error::Error>>;

struct FaucetCli {
    directory: tempfile::TempDir,
}

impl FaucetCli {
    fn new(endpoint: &str) -> Result<Self, Box<dyn std::error::Error>> {
        let cli = Self {
            directory: tempfile::tempdir()?,
        };
        let key = cli.run(
            &["key", "import", "alice", "--did", "did:layerx:alice"],
            Some(&"42".repeat(32)),
        )?;
        assert!(
            key.status.success(),
            "{}",
            String::from_utf8_lossy(&key.stderr)
        );
        let public_key = ed25519_dalek::SigningKey::from_bytes(&[0x42; 32])
            .verifying_key()
            .to_bytes();
        let trust_anchor = public_key
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>();
        let configured = cli.run(
            &[
                "environment",
                "use",
                "beta",
                "--endpoint",
                endpoint,
                "--network-id",
                "125",
                "--sequencer-trust-anchor",
                &trust_anchor,
            ],
            None,
        )?;
        assert!(
            configured.status.success(),
            "{}",
            String::from_utf8_lossy(&configured.stderr)
        );
        Ok(cli)
    }

    fn command(&self, arguments: &[&str]) -> Command {
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx"));
        command.env_clear();
        for name in ["PATH", "LD_LIBRARY_PATH"] {
            if let Some(value) = std::env::var_os(name) {
                command.env(name, value);
            }
        }
        command
            .args(["--json"])
            .args(arguments)
            .env("LAYERX_CONFIG", self.directory.path().join("config.json"))
            .env("LAYERX_CREDENTIAL_STORE", "file")
            .env(
                "LAYERX_CREDENTIAL_PASSPHRASE",
                "faucet route test passphrase",
            )
            .stdin(Stdio::null())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped());
        command
    }

    fn run(
        &self,
        arguments: &[&str],
        input: Option<&str>,
    ) -> Result<Output, Box<dyn std::error::Error>> {
        let mut command = self.command(arguments);
        if input.is_some() {
            command.stdin(Stdio::piped());
        }
        let mut child = command.spawn()?;
        if let Some(input) = input {
            child
                .stdin
                .take()
                .ok_or("missing stdin")?
                .write_all(input.as_bytes())?;
        }
        Ok(child.wait_with_output()?)
    }
}

struct Invocation(Child);

impl Drop for Invocation {
    fn drop(&mut self) {
        let _ = self.0.kill();
        let _ = self.0.wait();
    }
}

#[test]
fn faucet_posts_service_claim_route() -> TestResult {
    let listener = TcpListener::bind("127.0.0.1:0")?;
    listener.set_nonblocking(true)?;
    let cli = FaucetCli::new(&format!("http://{}", listener.local_addr()?))?;
    let session = "faucet-route-identity-session";
    let stored = cli.run(&["auth", "set"], Some(session))?;
    assert!(
        stored.status.success(),
        "{}",
        String::from_utf8_lossy(&stored.stderr)
    );
    let mut invocation = Invocation(cli.command(&["faucet", "--key", "alice"]).spawn()?);
    let deadline = Instant::now() + Duration::from_secs(10);
    let stream = loop {
        match listener.accept() {
            Ok((stream, _)) => break stream,
            Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                assert!(Instant::now() < deadline, "faucet sent no request");
                assert!(
                    invocation.0.try_wait()?.is_none(),
                    "faucet exited before sending"
                );
                std::thread::sleep(Duration::from_millis(10));
            }
            Err(error) => return Err(error.into()),
        }
    };
    stream.set_read_timeout(Some(Duration::from_secs(5)))?;
    let mut reader = BufReader::new(stream);
    let mut request_line = String::new();
    reader.read_line(&mut request_line)?;
    assert_eq!(request_line, "POST /rpc HTTP/1.1\r\n");
    let mut content_length = None;
    let mut authorization = None;
    let mut header_bytes = request_line.len();
    loop {
        let mut line = String::new();
        assert_ne!(
            reader.read_line(&mut line)?,
            0,
            "incomplete request headers"
        );
        header_bytes += line.len();
        assert!(header_bytes <= 16 * 1024, "request headers exceeded bound");
        if line == "\r\n" {
            break;
        }
        let (name, value) = line.split_once(':').ok_or("invalid request header")?;
        if name.eq_ignore_ascii_case("content-length") {
            content_length = Some(value.trim().parse::<usize>()?);
        }
        if name.eq_ignore_ascii_case("authorization") {
            authorization = Some(value.trim().to_owned());
        }
    }
    assert_eq!(
        authorization.as_deref(),
        Some(format!("Bearer {session}").as_str())
    );
    let length = content_length.ok_or("missing content length")?;
    assert!(length <= 4096, "faucet request exceeded bound");
    let mut body = vec![0; length];
    reader.read_exact(&mut body)?;
    let request: Value = serde_json::from_slice(&body)?;
    let configuration: Value =
        serde_json::from_slice(&std::fs::read(cli.directory.path().join("config.json"))?)?;
    assert_eq!(
        request,
        json!({
            "jsonrpc": "2.0",
            "id": "faucet",
            "method": "px_requestFunds",
            "params": ["did:layerx:alice", configuration["keys"]["alice"]["public_key"]],
        })
    );
    assert!(!String::from_utf8_lossy(&body).contains("lx_requestFunds"));
    drop(reader);
    let deadline = Instant::now() + Duration::from_secs(10);
    while invocation.0.try_wait()?.is_none() {
        assert!(
            Instant::now() < deadline,
            "faucet did not report closed transport"
        );
        std::thread::sleep(Duration::from_millis(10));
    }
    let mut stderr = String::new();
    invocation
        .0
        .stderr
        .take()
        .ok_or("missing stderr")?
        .read_to_string(&mut stderr)?;
    assert!(!invocation.0.wait()?.success());
    assert!(stderr.contains("POST /rpc failed"), "{stderr}");
    assert!(
        matches!(listener.accept(), Err(error) if error.kind() == std::io::ErrorKind::WouldBlock)
    );
    Ok(())
}

#[test]
fn faucet_requires_identity_session() -> TestResult {
    let listener = TcpListener::bind("127.0.0.1:0")?;
    listener.set_nonblocking(true)?;
    let cli = FaucetCli::new(&format!("http://{}", listener.local_addr()?))?;
    for arguments in [vec!["faucet"], vec!["faucet", "--key", "alice"]] {
        let output = cli.run(&arguments, None)?;
        assert!(!output.status.success());
        let refusal: Value = serde_json::from_slice(&output.stderr)?;
        assert_eq!(refusal["ok"], false);
        assert_eq!(refusal["error"]["code"], "identity_session_required");
        assert!(refusal["error"]["detail"]
            .as_str()
            .ok_or("missing detail")?
            .contains("layerx auth set"));
        assert!(output.stdout.is_empty());
        assert!(
            matches!(listener.accept(), Err(error) if error.kind() == std::io::ErrorKind::WouldBlock)
        );
    }
    Ok(())
}
