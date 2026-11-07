//! Task record: src/repair_22_31_211/record.kvx names findings and wiring.
//! Native proof adapters are overlaid into their owning Go test packages;
//! their application, keepers, RPC registration, stores and transports are real.
use std::fs;
use std::io::Write as _;
use std::os::unix::fs::{OpenOptionsExt as _, PermissionsExt as _};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Mutex, OnceLock};
use std::time::{Duration, Instant};

use layerx_client::sequencer::{ExecutedReceipt, ReceiptKeyEntry, SequencerLog, SequencerReceipt};
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_ramp_toolkit::clients::{
    authenticate_kms_signature, decode_hex, decode_kms_signature_response, hex,
    verify_custody_recovery, verify_recovery_settlement, ComplianceClient, Endpoint, LayerxClient,
    LayerxConfig, MutualTlsClient, MutualTlsFiles, PaxeerCustodyClient, ProviderClient, SecretFile,
};
use layerx_ramp_toolkit::engine::{InventoryRebalancer, RampEngine};
use layerx_ramp_toolkit::evm::{decode_budget_spent, verify_customer_budget, PAY_ADDRESS};
use layerx_ramp_toolkit::journal::{Journal, PaxeerObservation, TransitionEvidence, WorkflowStage};
use layerx_ramp_toolkit::{
    compile_customer_payment, compile_operator_send, AuthenticatedPrincipal, CreateOrder,
    OperatorIdentity, QuoteTerms, RampDirection, RampError, RampOrder,
};
use layerx_types::{payload::ModuleRegistry, rung::Rung};
use serde_json::{json, Value};
use sha3::{Digest as _, Keccak256};

static NEXT: AtomicU64 = AtomicU64::new(0);
static NATIVE: Mutex<()> = Mutex::new(());
static NATIVE_LIBRARIES: OnceLock<()> = OnceLock::new();
const WORKER: &str = "repair-worker";

struct Files(PathBuf);
impl Files {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "ramp-repair-2231211-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path).unwrap();
        fs::set_permissions(&path, fs::Permissions::from_mode(0o700)).unwrap();
        Self(path)
    }
    fn path(&self, name: &str) -> PathBuf {
        self.0.join(name)
    }
    fn write(&self, name: &str, data: &[u8]) {
        let mut file = fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(self.path(name))
            .unwrap();
        file.write_all(data).unwrap();
    }
    fn tls(&self) {
        for args in [
            vec![
                "req",
                "-x509",
                "-newkey",
                "rsa:2048",
                "-nodes",
                "-days",
                "1",
                "-subj",
                "/CN=localhost",
                "-addext",
                "subjectAltName=DNS:localhost",
                "-keyout",
                "key.pem",
                "-out",
                "ca.pem",
            ],
            vec!["x509", "-in", "ca.pem", "-outform", "DER", "-out", "ca.der"],
            vec![
                "pkcs12",
                "-export",
                "-inkey",
                "key.pem",
                "-in",
                "ca.pem",
                "-out",
                "identity.p12",
                "-passout",
                "pass:repair-identity",
            ],
        ] {
            let result = Command::new("openssl")
                .args(args)
                .current_dir(&self.0)
                .output()
                .unwrap();
            assert!(
                result.status.success(),
                "openssl: {}",
                String::from_utf8_lossy(&result.stderr)
            );
        }
        fs::set_permissions(self.path("identity.p12"), fs::Permissions::from_mode(0o600)).unwrap();
        self.write("password", b"repair-identity");
        self.write("token", b"repair-client-token");
    }
    fn http(&self) -> MutualTlsClient {
        MutualTlsClient::new(
            &MutualTlsFiles {
                ca_pem: self.path("ca.pem"),
                identity_pkcs12: SecretFile::new(self.path("identity.p12")).unwrap(),
                identity_password: SecretFile::new(self.path("password")).unwrap(),
            },
            Duration::from_secs(5),
        )
        .unwrap()
    }
    fn client(&self, endpoint: &str, address: [u8; 20], key: ReceiptKeyEntry) -> LayerxClient {
        let config: LayerxConfig = serde_json::from_value(json!({
            "rpc_endpoint": endpoint, "rpc_trust_anchor_der": self.path("ca.der"), "chain_id":125,
            "operator_address":format!("0x{}",hex(&address)),
            "receipt_keys":[{"keyId":format!("0x{}",hex(&key.key_id)), "publicKey":format!("0x{}",hex(&key.public_key)), "firstHeight":key.first_height,"lastHeight":key.last_height}],
            "max_fee_per_gas":100000000000_u128,"max_priority_fee_per_gas":1000000000_u128,"gas_limit_cap":2000000,
            "signer_endpoint":"https://localhost:1", "signer_token_file":self.path("token"),"actor_did":"did:layerx:operator"
        })).unwrap();
        config.build(self.http()).unwrap()
    }
}
impl Drop for Files {
    fn drop(&mut self) {
        if !std::thread::panicking() {
            fs::remove_dir_all(&self.0).unwrap();
        }
    }
}
fn root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .ancestors()
        .nth(4)
        .unwrap()
        .to_owned()
}
fn fixed<const N: usize>(value: &str) -> [u8; N] {
    decode_hex(value, N).unwrap().try_into().unwrap()
}
fn quantity(value: &Value) -> u64 {
    u64::from_str_radix(value.as_str().unwrap().trim_start_matches("0x"), 16).unwrap()
}
fn word(value: u64) -> [u8; 32] {
    let mut out = [0; 32];
    out[24..].copy_from_slice(&value.to_be_bytes());
    out
}
fn base64(bytes: &[u8]) -> String {
    const TABLE: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut out = String::new();
    for chunk in bytes.chunks(3) {
        let n = (u32::from(chunk[0]) << 16)
            | (u32::from(*chunk.get(1).unwrap_or(&0)) << 8)
            | u32::from(*chunk.get(2).unwrap_or(&0));
        for shift in [18, 12] {
            out.push(TABLE[((n >> shift) & 63) as usize] as char);
        }
        out.push(if chunk.len() > 1 {
            TABLE[((n >> 6) & 63) as usize] as char
        } else {
            '='
        });
        out.push(if chunk.len() > 2 {
            TABLE[(n & 63) as usize] as char
        } else {
            '='
        });
    }
    out
}
fn order(direction: RampDirection) -> RampOrder {
    RampOrder::bind(
        CreateOrder {
            order_id: "repair-order".into(),
            quote_id: "repair-quote".into(),
            payer_grant: (direction == RampDirection::OffRamp).then_some([7; 32]),
            payer_grant_purpose: (direction == RampDirection::OffRamp).then_some([6; 32]),
        },
        QuoteTerms {
            quote_id: "repair-quote".into(),
            direction,
            layerx_asset: [1; 32],
            layerx_amount: 3,
            external_currency: "EUR".into(),
            external_amount_minor: 3,
            rate_numerator: 1,
            rate_denominator: 1,
            fee_minor: 0,
            maximum_slippage_bps: 0,
            context: [8; 32],
            provider_token: "product".into(),
            payout_token: "beneficiary".into(),
            expires_at: 1800000100,
        },
        AuthenticatedPrincipal {
            principal_id: "customer".into(),
            account: "agent:did:layerx:customer:main".into(),
        },
        OperatorIdentity {
            principal_id: "operator".into(),
            account: "agent:did:layerx:operator:main".into(),
            signer_key_handle: "kms-operator".into(),
        },
        1800000000,
    )
    .unwrap()
}
fn payment_call(order: &RampOrder, nonce: u64) -> Eip1559Call {
    let payment = match order.direction() {
        RampDirection::OnRamp => compile_operator_send(order),
        RampDirection::OffRamp => compile_customer_payment(order),
    }
    .unwrap();
    Eip1559Call {
        chain_id: 125,
        nonce,
        max_fee_per_gas: 100000000000,
        max_priority_fee_per_gas: 1000000000,
        gas_limit: 500000,
        to: payment.to,
        data: payment.data,
    }
}
fn provider(files: &Files) -> ProviderClient {
    ProviderClient {
        http: files.http(),
        endpoint: Endpoint::parse("https://localhost:1").unwrap(),
        credential: "repair-token".into(),
        settlement_path: "/settle".into(),
        status_path: "/status".into(),
    }
}
fn custody(files: &Files) -> PaxeerCustodyClient {
    PaxeerCustodyClient {
        http: files.http(),
        endpoint: Endpoint::parse("https://localhost:1").unwrap(),
        credential: "repair-token".into(),
        broadcast_path: "/broadcast".into(),
        status_path: "/status".into(),
        operator_account: "operator".into(),
        wallet_address: "wallet".into(),
        vault_id: "vault".into(),
        signer_key_handle: "kms-operator".into(),
    }
}
fn compliance(files: &Files) -> ComplianceClient {
    ComplianceClient {
        http: files.http(),
        endpoint: Endpoint::parse("https://localhost:1").unwrap(),
        service_token: "repair-token".into(),
        verifying_key: ed25519_dalek::SigningKey::from_bytes(&[5; 32])
            .verifying_key()
            .to_bytes(),
    }
}
fn seed_payment(journal: &mut Journal, order: &RampOrder, raw: &[u8]) {
    let d = order.order_digest;
    journal.create_order(order.clone(), 100).unwrap();
    journal.acquire_lease(d, WORKER, 100, 1000).unwrap();
    journal
        .transition(
            d,
            WorkflowStage::CompliancePending,
            WorkflowStage::AwaitingLayerxPayment,
            TransitionEvidence::empty(),
            WORKER,
            101,
        )
        .unwrap();
    let mut evidence = TransitionEvidence::empty();
    evidence.tx_hash = Some(transaction_hash(raw));
    evidence.raw_transaction = Some(raw.to_vec());
    evidence.rung = Some(Rung::Submitted);
    journal
        .transition(
            d,
            WorkflowStage::AwaitingLayerxPayment,
            WorkflowStage::LayerxSubmissionPlanned,
            evidence,
            WORKER,
            102,
        )
        .unwrap();
}

#[test]
fn signer_wire_canonical_compact_signature_recovers_real_operator() {
    let order = order(RampDirection::OnRamp);
    let call = payment_call(&order, 0);
    let raw = call.sign(&[1; 32]).unwrap();
    let (_, signature) = decode_signed(&raw).unwrap();
    let operator = recover_sender(&call, &signature).unwrap();
    let compact = base64(&signature.as_bytes()[..64]);
    let recovery = signature.as_bytes()[64] - 27;
    let wire = serde_json::to_vec(&json!({"signature":compact,"recovery_id":recovery})).unwrap();
    let decoded = decode_kms_signature_response(&call, &wire, operator).unwrap();
    for invalid in [
        json!({"signature":compact}),
        json!({"signature":compact,"recovery_id":null}),
        json!({"signature":compact,"recovery_id":recovery,"extra":true}),
        json!({"signature":compact,"recovery_id":"0"}),
    ] {
        assert!(decode_kms_signature_response(
            &call,
            &serde_json::to_vec(&invalid).unwrap(),
            operator
        )
        .is_err());
    }
    assert_eq!(decoded.as_bytes(), signature.as_bytes());
    for invalid in [2, 3, 27, 28, 255] {
        assert!(authenticate_kms_signature(&call, &compact, invalid, operator).is_err());
    }
    for malformed in [
        base64(signature.as_bytes()),
        base64(&signature.as_bytes()[..63]),
        format!("{compact}\n"),
        compact.trim_end_matches('=').to_owned(),
    ] {
        assert!(authenticate_kms_signature(&call, &malformed, recovery, operator).is_err());
    }
    assert!(authenticate_kms_signature(&call, &compact, recovery, [2; 20]).is_err());
    let mut other = call.clone();
    other.nonce += 1;
    assert!(authenticate_kms_signature(&other, &compact, recovery, operator).is_err());
}

#[test]
fn payment_expiry_is_checked_unix_milliseconds_at_native_boundary() {
    let order = order(RampDirection::OnRamp);
    let call = compile_operator_send(&order).unwrap();
    assert_eq!(&call.data[132..164], &word(order.quote.expires_at * 1000));
    assert!(order.quote.validate(order.quote.expires_at - 1).is_ok());
    assert_eq!(
        order.quote.validate(order.quote.expires_at),
        Err(RampError::InvalidOrder)
    );
    let mut overflow = order;
    overflow.quote.expires_at = u64::MAX / 1000 + 1;
    overflow.order_digest = overflow.digest();
    assert_eq!(
        compile_operator_send(&overflow),
        Err(RampError::InvalidOrder)
    );
    assert_eq!(overflow.quote.validate(0), Err(RampError::InvalidOrder));
}

#[test]
fn journal_receipted_progress_cannot_settle_or_pay_out_after_restart() {
    let files = Files::new();
    let path = files.path("journal");
    let order = order(RampDirection::OffRamp);
    let d = order.order_digest;
    let raw = payment_call(&order, 0).sign(&[1; 32]).unwrap();
    let mut journal = Journal::open(&path).unwrap();
    seed_payment(&mut journal, &order, &raw);
    let mut evidence = journal.order(&d).unwrap().evidence.clone();
    evidence.receipt_digest = Some(Keccak256::digest(b"receipt-identity-for-journal-gate").into());
    evidence.receipt_height = Some(42);
    evidence.rung = Some(Rung::Receipted);
    journal
        .transition(
            d,
            WorkflowStage::LayerxSubmissionPlanned,
            WorkflowStage::LayerxPending,
            evidence.clone(),
            WORKER,
            103,
        )
        .unwrap();
    assert_eq!(
        journal.transition(
            d,
            WorkflowStage::LayerxPending,
            WorkflowStage::LayerxVerified,
            evidence.clone(),
            WORKER,
            104
        ),
        Err(RampError::IllegalTransition)
    );
    drop(journal);
    let mut journal = Journal::open(&path).unwrap();
    assert!(!journal.health().ready);
    journal
        .recover_verified(|projection| {
            let snapshot = projection.order(&d).unwrap();
            assert_eq!(snapshot.stage, WorkflowStage::LayerxPending);
            assert_eq!(snapshot.evidence.rung, Some(Rung::Receipted));
            assert!(snapshot.evidence.receipt_digest.is_some());
            assert_eq!(
                snapshot.evidence.raw_transaction.as_deref(),
                Some(raw.as_slice())
            );
            Ok(())
        })
        .unwrap();
    assert_eq!(
        journal.transition(
            d,
            WorkflowStage::LayerxPending,
            WorkflowStage::LayerxVerified,
            evidence.clone(),
            WORKER,
            104
        ),
        Err(RampError::IllegalTransition)
    );
    files.tls();
    let vector = producer_vector();
    let client = files.client("http://127.0.0.1:1", [1; 20], pin(&vector["pinnedKey"]));
    let provider = provider(&files);
    let compliance = compliance(&files);
    let registry = ModuleRegistry::new(&[]).unwrap();
    {
        let mut engine = RampEngine {
            journal: &mut journal,
            compliance: &compliance,
            provider: &provider,
            layerx: &client,
            registry: &registry,
            worker_id: WORKER,
            lease_seconds: 30,
        };
        engine.finish_if_complete(d, 104).unwrap();
        assert_eq!(
            engine.submit_provider(d, 104),
            Err(RampError::IllegalTransition)
        );
    }
    assert_eq!(
        journal.order(&d).unwrap().stage,
        WorkflowStage::LayerxPending
    );
    evidence.rung = Some(Rung::Committed);
    journal
        .transition(
            d,
            WorkflowStage::LayerxPending,
            WorkflowStage::LayerxVerified,
            evidence,
            WORKER,
            105,
        )
        .unwrap();
    let mut paid = TransitionEvidence::empty();
    paid.provider_operation_id = Some("payout".into());
    paid.provider_evidence_digest = Some([3; 32]);
    journal
        .transition(
            d,
            WorkflowStage::LayerxVerified,
            WorkflowStage::ProviderSettled,
            paid,
            WORKER,
            106,
        )
        .unwrap();
    journal
        .transition(
            d,
            WorkflowStage::ProviderSettled,
            WorkflowStage::Done,
            TransitionEvidence::empty(),
            WORKER,
            107,
        )
        .unwrap();
    drop(journal);
    let mut journal = Journal::open(&path).unwrap();
    journal
        .recover_verified(|projection| {
            let snapshot = projection.order(&d).unwrap();
            assert_eq!(snapshot.stage, WorkflowStage::Done);
            assert_eq!(snapshot.evidence.tx_hash, Some(transaction_hash(&raw)));
            assert!(snapshot.evidence.activity_id.is_none());
            assert_eq!(snapshot.evidence.rung, Some(Rung::Committed));
            Ok(())
        })
        .unwrap();
    assert!(journal.health().ready);
}

#[test]
fn custody_committed_rung_height_and_hash_survive_restart() {
    let files = Files::new();
    let path = files.path("journal");
    let mut journal = Journal::open(&path).unwrap();
    let key = [1; 32];
    journal.plan_paxeer(key, [2; 32], 3, 100).unwrap();
    let committed = PaxeerObservation {
        operation_id: "custody-operation",
        transaction_hash: [3; 32],
        rung: Rung::Committed,
        block_hash: Some([4; 32]),
        block_height: Some(42),
    };
    journal.observe_paxeer(key, committed, 101).unwrap();
    drop(journal);
    let mut journal = Journal::open(&path).unwrap();
    for changed in [
        PaxeerObservation {
            rung: Rung::Submitted,
            block_hash: None,
            block_height: None,
            ..committed
        },
        PaxeerObservation {
            rung: Rung::Receipted,
            block_hash: None,
            block_height: None,
            ..committed
        },
        PaxeerObservation {
            block_hash: Some([5; 32]),
            ..committed
        },
        PaxeerObservation {
            block_height: Some(43),
            ..committed
        },
    ] {
        assert_eq!(
            journal.recover_verified(|projection| verify_custody_recovery(
                projection.paxeer(&key).unwrap(),
                &changed
            )),
            Err(RampError::Paxeer)
        );
        assert!(!journal.health().ready);
    }
    journal
        .recover_verified(|projection| {
            verify_custody_recovery(projection.paxeer(&key).unwrap(), &committed)
        })
        .unwrap();
    assert!(journal.health().ready);
}

fn producer_vector() -> Value {
    let value: Value = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .unwrap();
    assert_eq!(
        value["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    value["cases"][2].clone()
}
fn executed(value: &Value) -> ExecutedReceipt {
    let q = |name: &str| quantity(&value[name]);
    let s = |name: &str| value[name].as_str().unwrap().to_owned();
    let mut fee = [0; 32];
    let bytes = decode_hex(
        &format!("{:0>64}", s("feeAmount").trim_start_matches("0x")),
        32,
    )
    .unwrap();
    fee.copy_from_slice(&bytes);
    ExecutedReceipt {
        tx_hash: fixed(&s("txHash")),
        nonce: q("nonce"),
        seq: q("seq"),
        height: q("height"),
        index: q("index"),
        timestamp: q("timestamp"),
        success: q("status") == 1,
        return_data: decode_hex(&s("returnData"), 512 * 1024).unwrap(),
        logs: value["logs"]
            .as_array()
            .unwrap()
            .iter()
            .map(|log| SequencerLog {
                address: fixed(log["address"].as_str().unwrap()),
                topics: log["topics"]
                    .as_array()
                    .unwrap()
                    .iter()
                    .map(|topic| fixed(topic.as_str().unwrap()))
                    .collect(),
                data: decode_hex(log["data"].as_str().unwrap(), 512 * 1024).unwrap(),
                log_index: quantity(&log["logIndex"]),
            })
            .collect(),
        gas_used: q("gasUsed"),
        cumulative_gas_used: q("cumulativeGasUsed"),
        effective_gas_price: q("effectiveGasPrice"),
        fee_denom: s("feeDenom"),
        payment_denom: s("paymentDenom"),
        fee_amount: fee,
        sponsor: fixed(&s("sponsor")),
        from: fixed(&s("from")),
        to: fixed(&s("to")),
        contract_address: fixed(&s("contractAddress")),
        vm_error: s("vmError"),
        key_id: fixed(&s("keyId")),
        receipt: decode_hex(&s("receipt"), 512 * 1024).unwrap(),
        signing_key: fixed(&s("signingKey")),
    }
}
fn pin(value: &Value) -> ReceiptKeyEntry {
    ReceiptKeyEntry {
        key_id: fixed(value["keyId"].as_str().unwrap()),
        public_key: fixed(value["publicKey"].as_str().unwrap()),
        first_height: value["firstHeight"].as_u64().unwrap(),
        last_height: value["lastHeight"].as_u64().unwrap(),
    }
}

#[test]
fn custody_pinned_receipt_validator_rejects_forged_execution_and_logs() {
    let files = Files::new();
    files.tls();
    let vector = producer_vector();
    let receipt = executed(&vector["receiptJson"]);
    let client = files.client(
        "http://127.0.0.1:1",
        receipt.from,
        pin(&vector["pinnedKey"]),
    );
    let hash = receipt.tx_hash;
    assert!(client
        .authenticate_settlement(hash, &SequencerReceipt::Receipted(receipt.clone()))
        .is_ok());
    for change in 0..8 {
        let mut altered = receipt.clone();
        match change {
            0 => altered.success = false,
            1 => *altered.receipt.last_mut().unwrap() ^= 1,
            2 => altered.key_id = [0; 32],
            3 => altered.nonce += 1,
            4 => altered.from = [0; 20],
            5 => altered.gas_used += 1,
            6 => altered.return_data.push(1),
            _ => altered.logs[0].data[0] ^= 1,
        }
        assert!(client
            .authenticate_settlement(hash, &SequencerReceipt::Receipted(altered))
            .is_err());
    }
    assert!(client
        .authenticate_settlement([9; 32], &SequencerReceipt::Receipted(receipt))
        .is_err());
}

struct NativeProcess {
    child: Child,
    log: PathBuf,
}
impl NativeProcess {
    fn start(files: &Files, package: &str, test: &str, calldata: Option<&[u8]>) -> Self {
        let root = root();
        NATIVE_LIBRARIES.get_or_init(|| {
            let log = files.path("native-build.log");
            let output = fs::OpenOptions::new()
                .write(true)
                .create_new(true)
                .mode(0o600)
                .open(&log)
                .unwrap();
            // These are the real libraries linked by modules/lxphost. Build
            // only its prerequisites, without running another test suite.
            let mut kernel = Command::new("make");
            kernel.args(["-j2", "build/liblayerx.a"]);
            let mut programs = Command::new("cargo");
            programs
                .args([
                    "build",
                    "--locked",
                    "--manifest-path",
                    "programs/Cargo.toml",
                    "-p",
                    "layerx-programs-sandbox",
                    "--features",
                    "host-ffi",
                ])
                .env("CARGO_TARGET_DIR", root.join("programs/target"));
            for mut command in [kernel, programs] {
                let status = command
                    .current_dir(&root)
                    .stdout(Stdio::from(output.try_clone().unwrap()))
                    .stderr(Stdio::from(output.try_clone().unwrap()))
                    .status()
                    .expect("native library build toolchain");
                assert!(
                    status.success(),
                    "native library build {status}; evidence {}\n{}",
                    log.display(),
                    fs::read_to_string(&log).unwrap()
                );
            }
        });
        let source = root.join("services/platform/ramps/toolkit/src/repair_22_31_211");
        // The general RPC fixture deliberately disables custom precompiles.
        // Enable the production constructors and activation gates for this
        // native payment proof, rather than calling an empty EVM account.
        let rpc_fixture = root.join("node/sequencer/rpc_e2e_test.go");
        let fixture_source = fs::read_to_string(&rpc_fixture).unwrap();
        let disabled =
            "a := app.New(dbm.NewMemDB(), nil, true, map[int64]bool{}, t.TempDir(), 1, false,";
        let enabled =
            "a := app.New(dbm.NewMemDB(), nil, true, map[int64]bool{}, t.TempDir(), 1, true,";
        assert_eq!(fixture_source.matches(disabled).count(), 1);
        files.write(
            "rpc_e2e_test.go",
            fixture_source.replacen(disabled, enabled, 1).as_bytes(),
        );
        let replacements = std::collections::BTreeMap::from([
            (rpc_fixture, files.path("rpc_e2e_test.go")),
            (
                root.join("node/sequencer/repair_22_31_211_test.go"),
                source.join("node_test.go"),
            ),
            (
                root.join("modules/layerxledger/keeper/repair_22_31_211_test.go"),
                source.join("expiry_test.go"),
            ),
            (
                root.join("precompiles/layerxpay/repair_22_31_211_test.go"),
                source.join("payment_test.go"),
            ),
        ]);
        let overlay = json!({"Replace":replacements});
        files.write("overlay.json", &serde_json::to_vec(&overlay).unwrap());
        let log = files.path("native.log");
        let output = fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&log)
            .unwrap();
        let mut command = Command::new("go");
        command
            .args(["test", "-overlay"])
            .arg(files.path("overlay.json"))
            .args(["-count=1", "-run", test, "-v", package])
            .current_dir(&root)
            .env("GOMAXPROCS", "2")
            .env("RAMP_REPAIR_EXCHANGE", &files.0)
            .stdout(Stdio::from(output.try_clone().unwrap()))
            .stderr(Stdio::from(output));
        if let Some(data) = calldata {
            command.env("RAMP_REPAIR_CALLDATA", hex(data));
        }
        let child = command.spawn().expect("native producer toolchain");
        Self { child, log }
    }
    fn wait_file(&mut self, path: &Path) -> Value {
        let deadline = Instant::now() + Duration::from_secs(900);
        loop {
            if let Ok(bytes) = fs::read(path) {
                if let Ok(value) = serde_json::from_slice(&bytes) {
                    return value;
                }
            }
            if let Some(status) = self.child.try_wait().unwrap() {
                panic!(
                    "native producer exited {status}; evidence {}\n{}",
                    self.log.display(),
                    fs::read_to_string(&self.log).unwrap()
                );
            }
            assert!(
                Instant::now() < deadline,
                "native producer deadline; evidence {}",
                self.log.display()
            );
            std::thread::sleep(Duration::from_millis(25));
        }
    }
    fn finish(&mut self) {
        let status = self.child.wait().unwrap();
        assert!(
            status.success(),
            "native producer {status}; evidence {}\n{}",
            self.log.display(),
            fs::read_to_string(&self.log).unwrap()
        );
    }
}
impl Drop for NativeProcess {
    fn drop(&mut self) {
        if self.child.try_wait().unwrap().is_none() {
            let _ = self.child.kill();
            let _ = self.child.wait();
        }
    }
}

#[test]
fn native_keeper_expiry_consumes_ramp_calldata_before_and_at_expiry() {
    let _serial = NATIVE.lock().unwrap_or_else(|poison| poison.into_inner());
    let files = Files::new();
    let data = compile_operator_send(&order(RampDirection::OnRamp))
        .unwrap()
        .data;
    let mut process = NativeProcess::start(
        &files,
        "./modules/layerxledger/keeper",
        "^TestRepairRampExpiry2231211$",
        Some(&data),
    );
    process.finish();
}

#[test]
fn native_customer_budget_debits_through_maintained_authority_and_decodes_payment() {
    let _serial = NATIVE.lock().unwrap_or_else(|poison| poison.into_inner());
    let files = Files::new();
    let mut process = NativeProcess::start(
        &files,
        "./precompiles/layerxpay",
        "^TestRepairRampCustomerPayment2231211$",
        None,
    );
    let bindings = process.wait_file(&files.path("budget.json"));
    let mut order = order(RampDirection::OffRamp);
    order.customer.account = bindings["customer"].as_str().unwrap().into();
    order.operator.account = bindings["operator"].as_str().unwrap().into();
    order.quote.layerx_asset = fixed(bindings["asset"].as_str().unwrap());
    order.payer_grant = Some(fixed(bindings["budget"].as_str().unwrap()));
    order.payer_grant_purpose = Some(fixed(bindings["purpose"].as_str().unwrap()));
    order.quote.expires_at = bindings["expiry_ms"].as_u64().unwrap() / 1000;
    order.order_digest = order.digest();
    let compiled = compile_customer_payment(&order).unwrap();
    assert_eq!(compiled.to, PAY_ADDRESS);
    assert_eq!(
        &compiled.data[36..68],
        fixed::<32>(bindings["operator_account"].as_str().unwrap()).as_slice()
    );
    let actual_budget = decode_hex(bindings["budget_abi"].as_str().unwrap(), 4096).unwrap();
    verify_customer_budget(&order, &actual_budget, true).unwrap();
    for index in [32, 96, 128, 12 * 32 + 31, 15 * 32 + 31, 16 * 32 + 31] {
        let mut changed = actual_budget.clone();
        changed[index] ^= 1;
        assert!(verify_customer_budget(&order, &changed, true).is_err());
    }
    let signed = payment_call(&order, 0).sign(&[9; 32]).unwrap();
    let (decoded, signature) = decode_signed(&signed).unwrap();
    assert_eq!(decoded.to, PAY_ADDRESS);
    assert_eq!(decoded.data, compiled.data);
    assert_ne!(recover_sender(&decoded, &signature).unwrap(), [0; 20]);
    files.write("calldata", hex(&compiled.data).as_bytes());
    let produced = process.wait_file(&files.path("payment.json"));
    process.finish();
    let log = SequencerLog {
        address: fixed(produced["address"].as_str().unwrap()),
        topics: produced["topics"]
            .as_array()
            .unwrap()
            .iter()
            .map(|topic| fixed(topic.as_str().unwrap()))
            .collect(),
        data: decode_hex(produced["data"].as_str().unwrap(), 1024).unwrap(),
        log_index: quantity(&produced["logIndex"]),
    };
    let account = layerx_wire::hash::account_id_for_protocol(
        &layerx_types::account::AccountId::parse(&order.operator.account).unwrap(),
        layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION,
    )
    .unwrap();
    assert_eq!(
        decode_budget_spent(&log),
        Some((
            order.payer_grant.unwrap(),
            account,
            order.quote.layerx_amount
        ))
    );
    let mut retired = log;
    retired.address[19] = 0x1a;
    assert!(decode_budget_spent(&retired).is_none());
}

#[test]
fn recovery_node_durable_transaction_and_failed_native_custody_use_real_root_rpc() {
    let _serial = NATIVE.lock().unwrap_or_else(|poison| poison.into_inner());
    let files = Files::new();
    files.tls();
    let mut process = NativeProcess::start(
        &files,
        "./node/sequencer",
        "^TestRepairRampNode2231211$",
        None,
    );
    let bindings = process.wait_file(&files.path("node.json"));
    let pin = ReceiptKeyEntry {
        key_id: fixed(bindings["key_id"].as_str().unwrap()),
        public_key: fixed(bindings["public_key"].as_str().unwrap()),
        first_height: 1,
        last_height: 10000,
    };
    let endpoint = bindings["endpoint"].as_str().unwrap();
    let address = fixed(bindings["operator"].as_str().unwrap());
    let secret = fixed(bindings["secret"].as_str().unwrap());
    let order = order(RampDirection::OnRamp);
    let raw = payment_call(&order, 0).sign(&secret).unwrap();
    let transaction = transaction_hash(&raw);
    let path = files.path("journal");
    let mut journal = Journal::open(&path).unwrap();
    journal.create_order(order.clone(), 100).unwrap();
    journal
        .acquire_lease(order.order_digest, WORKER, 100, 1000)
        .unwrap();
    journal
        .transition(
            order.order_digest,
            WorkflowStage::CompliancePending,
            WorkflowStage::AwaitingExternalCredit,
            TransitionEvidence::empty(),
            WORKER,
            101,
        )
        .unwrap();
    // A planned payment after external funding: the provider state is not
    // claimed here; the durable transaction reconciliation runs independently.
    drop(journal);
    let client = files.client(endpoint, address, pin.clone());
    let provider = provider(&files);
    let custody = custody(&files);
    let mut journal = Journal::open(&path).unwrap();
    journal
        .recover_verified(|projection| {
            verify_recovery_settlement(projection, &order.operator, &provider, &client, &custody)
        })
        .unwrap();
    assert!(journal.health().ready);
    let (unknown, _, committed) = client
        .resolve_transaction(&order, transaction, Some(&raw), false)
        .unwrap();
    assert_eq!(unknown.tx_hash(), transaction);
    assert_eq!(unknown.raw_transaction(), Some(raw.as_slice()));
    assert!(!committed);
    drop(client);
    let client = files.client(endpoint, address, pin);
    let mut changed = raw.clone();
    *changed.last_mut().unwrap() ^= 1;
    assert!(client
        .resolve_transaction(&order, transaction, Some(&changed), false)
        .is_err());
    let (sent, _, _) = client
        .resolve_transaction(&order, transaction, Some(&raw), true)
        .unwrap();
    assert_eq!(sent.tx_hash(), transaction);
    assert_eq!(sent.raw_transaction(), Some(raw.as_slice()));
    let receipt = client
        .sequencer
        .sequencer_receipt(&transaction)
        .unwrap()
        .expect("real sequencer executed the native call");
    let SequencerReceipt::Receipted(execution) = &receipt else {
        panic!("uncommitted native execution must be receipted")
    };
    assert!(
        !execution.success,
        "unprovisioned native authority must refuse"
    );
    let key = [10; 32];
    journal.plan_paxeer(key, [1; 32], 3, 110).unwrap();
    journal
        .observe_paxeer(
            key,
            PaxeerObservation {
                operation_id: "native-custody",
                transaction_hash: transaction,
                rung: Rung::Submitted,
                block_hash: None,
                block_height: None,
            },
            111,
        )
        .unwrap();
    let mut rebalancer = InventoryRebalancer {
        journal: &mut journal,
        custody: &custody,
        layerx: &client,
    };
    assert!(rebalancer
        .poll(key, "native-custody", transaction, 112)
        .is_err());
    assert_eq!(journal.paxeer(&key).unwrap().rung, Some(Rung::Submitted));
    files.write("stop", b"stop");
    process.finish();
}

#[test]
fn one_customer_transaction_cannot_authorize_two_payout_orders() {
    let files = Files::new();
    let mut journal = Journal::open(files.path("journal")).unwrap();
    let first = order(RampDirection::OffRamp);
    let raw = payment_call(&first, 0).sign(&[1; 32]).unwrap();
    seed_payment(&mut journal, &first, &raw);
    let mut second = first.clone();
    second.order_id = "another-payout".into();
    second.order_digest = second.digest();
    journal.create_order(second.clone(), 103).unwrap();
    journal
        .acquire_lease(second.order_digest, WORKER, 103, 30)
        .unwrap();
    journal
        .transition(
            second.order_digest,
            WorkflowStage::CompliancePending,
            WorkflowStage::AwaitingLayerxPayment,
            TransitionEvidence::empty(),
            WORKER,
            104,
        )
        .unwrap();
    let mut evidence = TransitionEvidence::empty();
    evidence.tx_hash = Some(transaction_hash(&raw));
    evidence.raw_transaction = Some(raw);
    evidence.rung = Some(Rung::Submitted);
    assert_eq!(
        journal.transition(
            second.order_digest,
            WorkflowStage::AwaitingLayerxPayment,
            WorkflowStage::LayerxSubmissionPlanned,
            evidence,
            WORKER,
            105
        ),
        Err(RampError::Conflict)
    );
}
