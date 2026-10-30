#[path = "../../../../platform/hosted/runtime-clock/src/server.rs"]
mod runtime_clock_server;

use std::error::Error;
use std::io::Write as _;
use std::net::{Ipv4Addr, SocketAddrV4};
use std::os::unix::fs::OpenOptionsExt as _;
use std::path::PathBuf;
use std::process::{Command, Output, Stdio};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::thread;
use std::time::{Duration, Instant};

type NodeRpcConfig = layerx_agentd::node_rpc::NodeRpcConfig;

struct ClockAuthority {
    socket: PathBuf,
    shutdown: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<std::io::Result<()>>>,
}

impl ClockAuthority {
    fn start() -> Result<Self, Box<dyn Error>> {
        let mut server = runtime_clock_server::Server::bind(&std::env::temp_dir())?;
        let socket = server.socket().to_owned();
        let shutdown = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&shutdown);
        let worker = thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                server.poll()?;
                thread::sleep(Duration::from_millis(1));
            }
            Ok(())
        });
        Ok(Self {
            socket,
            shutdown,
            worker: Some(worker),
        })
    }
}

impl Drop for ClockAuthority {
    fn drop(&mut self) {
        self.shutdown.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}

fn node_integration(config: &NodeRpcConfig) -> Command {
    let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-agentd"));
    command
        .env_clear()
        .env("LAYERX_AGENT_MODE", "human-owner")
        .env("LAYERX_AGENT_PROGRAM_LISTEN", "127.0.0.1:0")
        .env("LAYERX_AGENT_PROGRAM_BEARER_TOKEN", "h".repeat(32))
        .env("LAYERX_AGENT_HUMAN_AUTHORITY_BEARER", "a".repeat(32))
        .env("LAYERX_AGENT_HUMAN_DEADLINE_MS", "5000")
        .env(
            "LAYERX_AGENT_HUMAN_PROTOCOL_VERSION",
            layerx_wire::limits::PROTOCOL_VERSION.to_string(),
        )
        .env("LAYERX_AGENT_HUMAN_NETWORK_ID", config.chain_id.to_string())
        .env("LAYERX_AGENT_HUMAN_RECONNECT_ATTEMPTS", "3")
        .env("LAYERX_AGENT_HUMAN_RECONNECT_BASE_MS", "100")
        .env("LAYERX_AGENT_HUMAN_RECONNECT_MAX_MS", "1000")
        .env("LAYERX_AGENT_HUMAN_RECONNECT_JITTER_PERCENT", "10")
        .env("LAYERX_AGENT_HUMAN_MAX_FRAME_BYTES", "16777216")
        .env("LAYERX_AGENT_HUMAN_MAX_CONNECTIONS", "4")
        .env("LAYERX_AGENT_HUMAN_MAX_STREAMS", "16")
        .env("LAYERX_AGENT_HUMAN_MAX_QUEUED_BYTES", "16777216")
        .env(
            "LAYERX_AGENT_HUMAN_PEERS",
            "uid=4020;tenant=beta;principal=did:layerx:beta:alice",
        )
        .env("LAYERX_AGENT_NODE_RPC_URL", &config.url)
        .env("LAYERX_AGENT_NODE_CHAIN_ID", config.chain_id.to_string())
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());
    command
}

fn startup_output(command: &mut Command) -> Result<Output, Box<dyn Error>> {
    let clock = ClockAuthority::start()?;
    let root = clock.socket.parent().ok_or("clock directory is missing")?;
    let policy = root.join("native-policy.json");
    let secret = root.join("operator-secret");
    struct Inputs([PathBuf; 2]);
    impl Drop for Inputs {
        fn drop(&mut self) {
            for path in &self.0 {
                let _ = std::fs::remove_file(path);
            }
        }
    }
    let _inputs = Inputs([policy.clone(), secret.clone()]);
    let mut secret_bytes = zeroize::Zeroizing::new([0_u8; 32]);
    getrandom::fill(secret_bytes.as_mut())
        .map_err(|error| std::io::Error::other(error.to_string()))?;
    for (path, bytes) in [
        (
            &policy,
            br#"{"schema":"layerx.native-program-policy.v1","version":"startup-refusal","network_id":125,"step_limit":1000,"rules":[{"id":"deny-operation","effect":"Deny","activity":{"module":9,"ordinal":1},"scope":{"kind":"Operation"},"required_approval":false,"purpose_commitment":null}]}"#.as_slice(),
        ),
        (&secret, secret_bytes.as_slice()),
    ] {
        std::fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(path)?
            .write_all(bytes)?;
    }
    command
        .env(
            "LAYERX_NATIVE_POLICY_SOURCES",
            format!("beta:{}", policy.display()),
        )
        .env("LAYERX_AGENT_HUMAN_STORE", root.join("store"))
        .env("LAYERX_AGENT_HUMAN_SOCKET", root.join("human.sock"))
        .env("LAYERX_AGENT_HUMAN_SESSION_KEY_ROOT", root.join("keys"))
        .env("LAYERX_AGENT_HUMAN_SESSION_OPERATOR_SECRET_FILE", &secret);
    let mut child = command
        .env("LAYERX_RUNTIME_CLOCK_SOCKET", &clock.socket)
        .env("LAYERX_RUNTIME_CLOCK_PID", std::process::id().to_string())
        .env(
            "LAYERX_RUNTIME_CLOCK_UID",
            rustix::process::geteuid().as_raw().to_string(),
        )
        .spawn()?;
    let expires = Instant::now() + Duration::from_secs(40);
    loop {
        if child.try_wait()?.is_some() {
            return Ok(child.wait_with_output()?);
        }
        if Instant::now() >= expires {
            child.kill()?;
            let output = child.wait_with_output()?;
            return Err(format!(
                "agentd did not refuse startup within 40 seconds: {}",
                String::from_utf8_lossy(&output.stderr)
            )
            .into());
        }
        thread::sleep(Duration::from_millis(10));
    }
}

fn refused(output: Output, diagnostic: &str) -> Result<(), Box<dyn Error>> {
    let stderr = String::from_utf8(output.stderr)?;
    assert_eq!(output.status.code(), Some(2), "{stderr}");
    assert!(stderr.contains(diagnostic), "{stderr}");
    assert!(output.stdout.is_empty(), "unexpected startup stdout");
    Ok(())
}

fn closed_endpoint() -> Result<(std::os::fd::OwnedFd, NodeRpcConfig), Box<dyn Error>> {
    let socket = rustix::net::socket(
        rustix::net::AddressFamily::INET,
        rustix::net::SocketType::STREAM,
        None,
    )?;
    rustix::net::bind(&socket, &SocketAddrV4::new(Ipv4Addr::LOCALHOST, 0))?;
    let address = SocketAddrV4::try_from(rustix::net::getsockname(&socket)?)?;
    let config = NodeRpcConfig {
        url: format!("http://{address}/"),
        chain_id: 125,
        deadline: Duration::from_secs(5),
    };
    Ok((socket, config))
}

#[test]
fn startup_checks_node_json_rpc_before_the_human_lni_variable() -> Result<(), Box<dyn Error>> {
    let (_socket, config) = closed_endpoint()?;
    let unserved_socket = std::env::temp_dir().join(format!(
        "layerx-agentd-unserved-node-{}.sock",
        std::process::id()
    ));
    assert!(unserved_socket.is_absolute());
    assert!(!unserved_socket.exists());
    let output = startup_output(
        node_integration(&config).env("LAYERX_AGENT_HUMAN_NODE_LNI", &unserved_socket),
    )?;
    let stderr = String::from_utf8(output.stderr.clone())?;
    refused(output, "node JSON-RPC is unavailable")?;
    assert!(
        !stderr.contains(&*unserved_socket.to_string_lossy()),
        "{stderr}"
    );
    assert!(!stderr.contains("LNI"), "{stderr}");
    Ok(())
}

#[test]
fn startup_reports_node_json_rpc_unavailable_for_a_closed_endpoint() -> Result<(), Box<dyn Error>> {
    let (_socket, config) = closed_endpoint()?;
    let output = startup_output(&mut node_integration(&config))?;
    let stderr = String::from_utf8(output.stderr.clone())?;
    refused(output, "node JSON-RPC is unavailable")?;
    assert!(!stderr.contains("LNI"), "{stderr}");
    Ok(())
}

#[test]
fn startup_refuses_a_chain_id_mismatch() -> Result<(), Box<dyn Error>> {
    let chain_id: u64 = std::env::var("LAYERX_TEST_PAXEER_CHAIN_ID")
        .map_err(|_| "LAYERX_TEST_PAXEER_CHAIN_ID is required for the in-tree local node")?
        .parse()?;
    let port: u16 = std::env::var("LAYERX_TEST_PAXEER_RPC_PORT")
        .map_err(|_| "LAYERX_TEST_PAXEER_RPC_PORT is required for the in-tree local node")?
        .parse()?;
    let mut config = NodeRpcConfig {
        url: format!("http://127.0.0.1:{port}"),
        chain_id,
        deadline: Duration::from_secs(5),
    };
    let node = layerx_agentd::node_rpc::NodeRpc::connect(&config)?;
    assert_eq!(node.chain_id(), chain_id);
    config.chain_id = chain_id ^ 1;
    assert_ne!(config.chain_id, chain_id);
    let output = startup_output(&mut node_integration(&config))?;
    let stderr = String::from_utf8(output.stderr.clone())?;
    refused(output, "node chain id mismatch")?;
    assert!(!stderr.contains("LNI"), "{stderr}");
    assert!(!stderr.contains("node JSON-RPC is unavailable"), "{stderr}");
    Ok(())
}
