//! Proves: production::{run_if_configured, run_required, run_recovery, run_duplicate, run_peer_refusal} -> std::process::Command(agent-boundary-conformance) -> Makefile::agent-test-boundary and main::check_report.
//!
//! Repairs main.rs::run rejecting the Makefile's NODE REPOSITORY invocation,
//! and Makefile::agent-test-boundary accepting an unchecked producer report.
//! Repairs production.rs::transfer selecting a pending nonce for a new
//! submission: after pax_sequencerReceipt resolves the preceding transfer,
//! read the sender's nonce at the latest height for the next submission.
//! The proof's LocalNode::start uses the funded deployer account so the
//! launcher's live price feeder cannot race its transaction nonce.
//! The proof starts the repository's disposable paxd sequencer using
//! tools/qualification/localnode/v6120-node.sh, which creates and funds its
//! own signer. No operator endpoint, key, or pre-existing chain is required.
//! No RPC server or report producer is substituted. Each invocation is a fresh
//! process; recovery reopens the exact signed bytes retained by admission.
//! rpc/watermark_manager.go::resolvePending must serve committed state while
//! no block is open (22.7.2); the suite's pending balance read must succeed
//! directly, without retrying around a pending-block refusal.
//! rpc/tx.go::pendingNonce must include the executed transfer after
//! pax_sequencerReceipt resolves it (18.1502.1), including the committed-height
//! fallback in rpc/watermark_manager.go::resolvePending. Assert that nonce
//! between crossings without retrying or bypassing the pending read.

use std::fs;
use std::io::Write;
use std::path::{Path, PathBuf};
use std::process::{Command, Output, Stdio};
use std::time::{SystemTime, UNIX_EPOCH};

use layerx_client::sequencer::{ExecutedReceipt, SequencerEndpoint, SequencerReceipt};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};

const MODES: [&str; 5] = [
    "",
    "--production-required",
    "--production-recovery",
    "--production-duplicate",
    "--production-peer-refusal",
];

fn repository() -> &'static Path {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .ancestors()
        .nth(4)
        .expect("boundary crate lives inside the repository")
}

fn make(target: &str, mode: &str, node: &str) -> Command {
    let mut command = Command::new("make");
    command
        .env_clear()
        .env("PATH", std::env::var_os("PATH").expect("tool search path"))
        .current_dir(repository())
        .args(["--no-print-directory", "--silent", target])
        // Select the already-built real producer, avoiding a nested Cargo build.
        .arg(format!(
            "AGENT_BOUNDARY_COMMAND='{}'",
            env!("CARGO_BIN_EXE_agent-boundary-conformance").replace('\'', "'\\''")
        ))
        .arg(format!("AGENT_BOUNDARY_MODE={mode}"))
        .arg(format!("AGENT_NODE_RPC_URL={node}"));
    command
}

fn decode(mode: &str, report: &[u8]) -> Output {
    let mut child = make("agent-boundary-check-report", mode, "")
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .expect("start the Makefile's actual report consumer");
    child
        .stdin
        .take()
        .expect("report input")
        .write_all(report)
        .expect("send the producer's bytes to the consumer");
    child.wait_with_output().expect("wait for report consumer")
}

fn success<'a>(output: &'a Output, mode: &str) -> &'a str {
    assert!(
        output.status.success(),
        "real boundary crossing failed: mode={mode:?} status={} stderr={}",
        output.status,
        String::from_utf8_lossy(&output.stderr)
    );
    std::str::from_utf8(&output.stdout).expect("consumer accepted a UTF-8 report")
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

fn configured_make(mode: &str, node: &LocalNode, transaction: &Path) -> Command {
    let mut command = make("agent-test-boundary", mode, &node.url);
    command
        .env("LAYERX_QUALIFY_EVM_PRIVATE_KEY", &node.key)
        .env("LAYERX_QUALIFY_MAX_PRIORITY_FEE_PER_GAS", "1000000000")
        .env("LAYERX_QUALIFY_MAX_FEE_PER_GAS", "1000000000000")
        .env("LAYERX_QUALIFY_TRANSACTION_FILE", transaction);
    command
}

fn executed(endpoint: &SequencerEndpoint, hash: &[u8; 32]) -> ExecutedReceipt {
    match endpoint
        .sequencer_receipt(hash)
        .expect("read real node receipt")
    {
        Some(SequencerReceipt::Receipted(receipt))
        | Some(SequencerReceipt::Committed { receipt, .. }) => receipt,
        other => panic!("successful producer did not leave an executed receipt: {other:?}"),
    }
}

#[test]
fn proof_agent_boundary_conformance_real_producers_reopen_and_reach_make_twice() {
    let node = LocalNode::start();
    let endpoint = SequencerEndpoint::new(&node.url, &[]).expect("real disposable node endpoint");
    let directory = RetainedDirectory::new();
    let mut previous_nonce = None;
    let mut previous_hash = None;
    for round in 0..2 {
        for mode in ["", "--production-required"] {
            let transaction = directory.0.join(format!("{round}-{mode}.rlp"));
            assert!(!transaction.exists());
            let output = configured_make(mode, &node, &transaction)
                .output()
                .expect("run real admission through Makefile");
            assert!(
                success(&output, mode).contains("RPC_CASE signed-transfer-admission-and-receipt")
            );
            success(&decode(mode, &output.stdout), mode);
            assert!(
                !decode(mode, &output.stdout[..output.stdout.len() / 2])
                    .status
                    .success(),
                "accepted truncated admission output"
            );
            let mut invalid_utf8 = output.stdout.clone();
            invalid_utf8[0] = 0xff;
            assert!(
                !decode(mode, &invalid_utf8).status.success(),
                "accepted non-UTF-8 admission output"
            );

            // The producer has stopped. Reopen its persisted signed bytes and
            // bind the consumer's report to the real node's observable state.
            let raw = fs::read(&transaction).expect("reopen producer-retained transaction");
            let (call, signature) = decode_signed(&raw).expect("decode real signed transfer");
            let sender = recover_sender(&call, &signature).expect("recover real signer");
            let hash = transaction_hash(&raw);
            assert_eq!(call.chain_id, endpoint.chain_id().expect("node chain id"));
            assert_eq!(call.to, sender);
            assert!(call.data.is_empty());
            if let Some(nonce) = previous_nonce {
                assert_eq!(call.nonce, nonce + 1, "admission must execute afresh");
            }
            assert_ne!(previous_hash, Some(hash), "no cached admission report");
            previous_nonce = Some(call.nonce);
            previous_hash = Some(hash);
            let receipt = executed(&endpoint, &hash);
            assert_eq!(receipt.tx_hash, hash);
            assert_eq!(receipt.nonce, call.nonce);
            assert_eq!(receipt.from, sender);
            assert_eq!(receipt.to, sender);
            assert!(receipt.success);
            assert!(receipt.vm_error.is_empty());
            let latest = endpoint
                .block_number()
                .expect("latest node height after receipt");
            assert_eq!(
                endpoint.nonce_at(&sender, latest),
                Ok(call.nonce + 1),
                "production.rs::transfer must select the next sequence from latest state after pax_sequencerReceipt resolves the preceding transfer"
            );
            assert_eq!(
                endpoint.pending_nonce(&sender),
                Ok(call.nonce + 1),
                "eth_getTransactionCount pending must advance after pax_sequencerReceipt resolves the transfer, before the next crossing (rpc/tx.go::pendingNonce)"
            );

            for followup in &MODES[2..] {
                let output = configured_make(followup, &node, &transaction)
                    .output()
                    .expect("reopen transaction in a fresh real producer process");
                let marker = match *followup {
                    "--production-recovery" => "RPC_CASE retained-transaction-receipt-recovery",
                    "--production-duplicate" => {
                        "RPC_CASE identical-raw-transaction-preserves-hash-and-receipt"
                    }
                    "--production-peer-refusal" => "RPC_CASE wrong-chain-id-refusal",
                    _ => unreachable!("declared followup mode"),
                };
                assert_eq!(success(&output, followup).trim_end(), marker);
                success(&decode(followup, &output.stdout), followup);
                assert_eq!(fs::read(&transaction).expect("retained bytes survive"), raw);
                assert_eq!(executed(&endpoint, &hash), receipt);
                assert_eq!(endpoint.pending_nonce(&sender), Ok(call.nonce + 1));

                // Corrupt bytes emitted by the real producer, then cross the
                // very same consumer decoder used by agent-test-boundary.
                let truncated = decode(followup, &output.stdout[..output.stdout.len() / 2]);
                assert!(
                    !truncated.status.success(),
                    "accepted truncated producer output"
                );
                let mut invalid_utf8 = output.stdout.clone();
                invalid_utf8[0] = 0xff;
                assert!(!decode(followup, &invalid_utf8).status.success());
                assert!(!decode("--production-required", &output.stdout)
                    .status
                    .success());
            }
            assert_eq!(endpoint.sequencer_receipt(&[0xff; 32]), Ok(None));
        }
    }
}

#[test]
fn proof_agent_boundary_conformance_make_propagates_all_five_real_refusals_twice() {
    for _ in 0..2 {
        for mode in MODES {
            let output = make("agent-test-boundary", mode, "http://127.0.0.1:1")
                .env("LAYERX_QUALIFY_LNI_SOCKET", "retired")
                .output()
                .expect("run the real producer through the Makefile consumer");
            assert!(
                !output.status.success(),
                "Makefile swallowed the producer refusal"
            );
            assert!(output.stdout.is_empty(), "refusal emitted success evidence");
            let diagnostic = std::str::from_utf8(&output.stderr).expect("UTF-8 refusal");
            assert!(diagnostic.contains("LAYERX_QUALIFY_LNI_SOCKET is retired"));
            let malformed_result = decode(mode, &output.stderr);
            assert!(
                !malformed_result.status.success(),
                "accepted refusal as a success report"
            );
            assert!(String::from_utf8_lossy(&malformed_result.stderr).contains("boundary result"));
        }
    }
}

#[test]
fn proof_agent_boundary_conformance_positional_input_is_checked_before_rpc() {
    let missing_repository = repository().join("missing-boundary-proof-repository");
    assert!(!missing_repository.exists());
    let output = Command::new(env!("CARGO_BIN_EXE_agent-boundary-conformance"))
        .env_clear()
        .arg("http://127.0.0.1:1")
        .arg(missing_repository)
        .output()
        .expect("run actual positional CLI");
    assert!(!output.status.success());
    assert!(String::from_utf8_lossy(&output.stderr)
        .contains("REPOSITORY must contain the agent boundary crate"));

    for mode in MODES {
        let output = make("agent-test-boundary", mode, "file:///retired-lni")
            .output()
            .expect("run malformed NODE through the real consumer");
        assert!(!output.status.success());
        assert!(String::from_utf8_lossy(&output.stderr)
            .contains("LAYERX_QUALIFY_NODE_RPC_URL endpoint refused"));
    }
}
