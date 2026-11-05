use std::fs;
use std::io::Write;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::time::{SystemTime, UNIX_EPOCH};

use layerx_client::sequencer::{SequencerEndpoint, SequencerReceipt};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};

#[path = "../boundary.rs"]
mod boundary;

fn repository() -> &'static Path {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .ancestors()
        .nth(4)
        .expect("qualification crate lives inside the repository")
}

struct RetainedDirectory(PathBuf);

impl RetainedDirectory {
    fn new() -> Self {
        let stamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("time after epoch")
            .as_nanos();
        let path = std::env::temp_dir().join(format!(
            "proof-agent-boundary-{}-{stamp}",
            std::process::id()
        ));
        fs::create_dir(&path).expect("create private transaction directory");
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            fs::set_permissions(&path, fs::Permissions::from_mode(0o700))
                .expect("restrict retained transaction directory");
        }
        Self(path)
    }
}

impl Drop for RetainedDirectory {
    fn drop(&mut self) {
        // Preserve private node/build evidence if qualification fails.
        if !std::thread::panicking() {
            fs::remove_dir_all(&self.0).expect("remove this proof's retained transactions");
        }
    }
}

struct LocalNode {
    directory: RetainedDirectory,
    url: String,
    key: String,
}

impl LocalNode {
    fn launcher(&self) -> Command {
        let mut command = Command::new("bash");
        command
            .env_clear()
            .env("PATH", std::env::var_os("PATH").expect("tool search path"))
            .env("HOME", std::env::var_os("HOME").expect("Go toolchain home"))
            .env("GOTOOLCHAIN", "auto")
            .env("TMPDIR", &self.directory.0)
            .env("GOBIN", self.directory.0.join("bin"))
            .env("PAXD", self.directory.0.join("bin/paxd"))
            .current_dir(repository())
            .arg(repository().join("tools/qualification/localnode/v6120-node.sh"));
        command
    }

    fn start() -> Self {
        let mut node = Self {
            directory: RetainedDirectory::new(),
            url: String::new(),
            key: String::new(),
        };
        fs::create_dir(node.directory.0.join("bin")).expect("create private node binary directory");
        let log = node.directory.0.join("node-start.log");
        let stdout = fs::File::create(&log).expect("create private node startup log");
        let stderr = stdout.try_clone().expect("share node startup log");
        // This existing launcher installs the real paxd when absent, creates
        // isolated genesis state, and waits for the actual sequencer RPC.
        // It does not run any qualification suite or replace an RPC method.
        let status = node
            .launcher()
            .arg("--sequencer")
            .stdout(stdout)
            .stderr(stderr)
            .status()
            .expect("start the repository's real disposable node launcher");
        assert!(
            status.success(),
            "real node setup failed: {status}; private evidence: {}",
            log.display()
        );

        // Read only the launcher's own output. A shell decodes its %q quoting;
        // credentials stay in memory and are never printed in test diagnostics.
        let environment = node.directory.0.join("lx-localnode/localnode.env");
        let output = Command::new("bash")
            .args(["-c", "set -eu; . \"$1\"; printf '%s\\n' \"$LOCAL_NODE_SEQUENCER_RPC\" \"$LOCAL_NODE_HOME\""])
            .arg("boundary-node-environment")
            .arg(environment)
            .output()
            .expect("read the real node launcher's environment");
        assert!(
            output.status.success(),
            "node environment could not be decoded"
        );
        let values = std::str::from_utf8(&output.stdout).expect("UTF-8 node environment");
        let mut values = values.lines();
        node.url = values.next().expect("local sequencer URL").to_owned();
        let home = values.next().expect("this launcher's disposable node home");
        assert!(
            values.next().is_none(),
            "unexpected node environment fields"
        );
        assert!(node.url.starts_with("http://127.0.0.1:"));
        assert!(Path::new(home).starts_with(node.directory.0.join("lx-localnode")));

        // The launcher feeds prices with its `test` account. Export its other
        // funded signer so background Cosmos transactions cannot consume the
        // proof's EVM nonce between nonce_at and send_raw_transaction.
        let mut signer = Command::new(node.directory.0.join("bin/paxd"))
            .env_clear()
            // node/app.go initializes DefaultNodeHome before parsing --home.
            // Give the isolated CLI its own HOME even with an explicit keyring.
            .env("HOME", &node.directory.0)
            .args(["keys", "export", "deployer", "--unsafe", "--unarmored-hex"])
            .arg("--home")
            .arg(home)
            .args(["--keyring-backend", "test"])
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
            .expect("export this node's separately funded deployer key");
        signer
            .stdin
            .take()
            .expect("key export confirmation input")
            .write_all(b"y\n")
            .expect("confirm this disposable key export");
        let signer = signer
            .wait_with_output()
            .expect("wait for local key export");
        let export_log = node.directory.0.join("deployer-export.log");
        fs::write(&export_log, &signer.stderr).expect("retain private deployer export diagnostics");
        assert!(
            signer.status.success(),
            "disposable deployer export failed: {}; private evidence: {}",
            signer.status,
            export_log.display()
        );
        let key = std::str::from_utf8(&signer.stdout)
            .expect("UTF-8 local deployer key")
            .trim();
        assert_eq!(key.len(), 64, "deployer key must contain 32 bytes");
        assert!(key.bytes().all(|byte| byte.is_ascii_hexdigit()));
        node.key = format!("0x{key}");
        assert_eq!(node.key.len(), 66, "funded key must contain 32 bytes");
        node
    }
}

impl Drop for LocalNode {
    fn drop(&mut self) {
        let base = self.directory.0.join("lx-localnode");
        if let Ok(entries) = fs::read_dir(base) {
            for entry in entries {
                let path = entry.expect("read this proof's node home").path();
                if path.join(".localnode").is_file() {
                    let output = self
                        .launcher()
                        .arg("--stop")
                        .arg(&path)
                        .output()
                        .expect("stop this proof's real node");
                    assert!(
                        output.status.success(),
                        "could not stop disposable node: {}",
                        path.display()
                    );
                }
            }
        }
    }
}

#[test]
fn boundary_report_mismatch_real_producer_transport_consumer() {
    let node = LocalNode::start();
    let directory = RetainedDirectory::new();
    let transaction = directory.0.join("admission.rlp");
    let harness = Path::new(env!("CARGO_BIN_EXE_agent-boundary-conformance"));
    let output = Command::new(env!("CARGO_BIN_EXE_agent-qualification-gates"))
        .env_clear()
        .args(["boundary"])
        .arg(repository())
        .arg(&node.url)
        .arg(harness)
        .env("LAYERX_QUALIFY_EVM_PRIVATE_KEY", &node.key)
        .env("LAYERX_QUALIFY_MAX_PRIORITY_FEE_PER_GAS", "1000000000")
        .env("LAYERX_QUALIFY_MAX_FEE_PER_GAS", "1000000000000")
        .env("LAYERX_QUALIFY_TRANSACTION_FILE", &transaction)
        .output()
        .expect("run the real qualification CLI with the real boundary harness");
    assert!(
        output.status.success(),
        "qualification failed: status={} stderr={}",
        output.status,
        String::from_utf8_lossy(&output.stderr)
    );
    let stdout = String::from_utf8(output.stdout).expect("UTF-8 qualification report");
    let lines: Vec<_> = stdout.lines().filter(|line| !line.is_empty()).collect();
    assert_eq!(lines.len(), 7, "every production mode must report");
    assert_eq!(lines[0], "agent_qualify_boundary_gate passed node_json_rpc=true modes=admission,recovery,duplicate,peer-refusal");
    assert!(lines[1].starts_with(
        "agent boundary conformance suite passed: node JSON-RPC endpoint, committed block "
    ));
    assert!(lines[1].ends_with(", absent receipt, malformed submit"));
    assert!(lines[2].ends_with(
        " pending reads exercised: eth_getBalance eth_getTransactionCount eth_estimateGas eth_call"
    ));
    assert_eq!(lines[3], "RPC_CASE signed-transfer-admission-and-receipt");
    assert_eq!(lines[4], "RPC_CASE retained-transaction-receipt-recovery");
    assert_eq!(
        lines[5],
        "RPC_CASE identical-raw-transaction-preserves-hash-and-receipt"
    );
    assert_eq!(lines[6], "RPC_CASE wrong-chain-id-refusal");

    let endpoint = SequencerEndpoint::new(&node.url, &[]).expect("real node endpoint");
    let raw = fs::read(&transaction).expect("real harness retained the signed transaction");
    let (call, signature) = decode_signed(&raw).expect("real retained EIP-1559 transaction");
    assert_eq!(call.chain_id, endpoint.chain_id().expect("actual chain id"));
    let sender = recover_sender(&call, &signature).expect("actual transaction signer");
    assert_eq!(call.to, sender);
    let hash = transaction_hash(&raw);
    let receipt = match endpoint.sequencer_receipt(&hash).expect("actual receipt") {
        Some(SequencerReceipt::Receipted(receipt))
        | Some(SequencerReceipt::Committed { receipt, .. }) => receipt,
        other => panic!("real transfer has no executed receipt: {other:?}"),
    };
    assert_eq!(receipt.tx_hash, hash);
    assert_eq!(receipt.from, sender);
    assert_eq!(receipt.nonce, call.nonce);
    assert!(receipt.success);
    assert!(receipt.vm_error.is_empty());

    let admission = lines[1..4].join("\n") + "\n";
    boundary::check_boundary_report(harness, "--production-required", admission.as_bytes())
        .expect("the real producer report crosses the shared decoder");
    for invalid in [
        admission.replace("node JSON-RPC endpoint,", "node JSON-RPC,"),
        admission.replace("committed block ", "committed block -"),
        admission.replace(", absent receipt, malformed submit", ""),
        admission.replace(" eth_getBalance", ""),
        admission.replace(" eth_getTransactionCount", ""),
        admission.replace(" eth_estimateGas", ""),
        admission.replace(" eth_call", ""),
        admission.replace("chain_id=", "chain_id=0"),
        admission.replace("RPC_CASE signed-transfer-admission-and-receipt", ""),
        admission.lines().skip(1).collect::<Vec<_>>().join("\n"),
        format!("{admission}unexpected line\n"),
    ] {
        assert!(
            boundary::check_boundary_report(harness, "--production-required", invalid.as_bytes())
                .is_err(),
            "accepted damaged real producer report: {invalid:?}"
        );
    }
    let mut invalid_utf8 = admission.as_bytes().to_vec();
    invalid_utf8[0] = 0xff;
    assert!(
        boundary::check_boundary_report(harness, "--production-required", &invalid_utf8).is_err()
    );
    let oversized = admission.repeat(65_537 / admission.len() + 1);
    assert!(boundary::check_boundary_report(
        harness,
        "--production-required",
        oversized.as_bytes()
    )
    .is_err());
    for (mode, line) in [
        ("--production-recovery", lines[4]),
        ("--production-duplicate", lines[5]),
        ("--production-peer-refusal", lines[6]),
    ] {
        boundary::check_boundary_report(harness, mode, line.as_bytes())
            .expect("real followup report");
        assert!(
            boundary::check_boundary_report(harness, mode, &line.as_bytes()[..line.len() / 2])
                .is_err()
        );
        assert!(
            boundary::check_boundary_report(harness, "--production-required", line.as_bytes())
                .is_err()
        );
        assert!(boundary::check_boundary_report(harness, mode, admission.as_bytes()).is_err());
    }
    assert!(boundary::agent_qualify_boundary_gate(
        repository(),
        Path::new("file:///invalid"),
        harness
    )
    .is_err());
}
