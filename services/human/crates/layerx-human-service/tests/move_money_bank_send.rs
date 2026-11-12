use layerx_client::runtime_clock::RuntimeClock;
use layerx_types::clock::{Clock, Deadline};
use std::collections::BTreeMap;
use std::fs::{self, OpenOptions};
use std::os::unix::fs::{OpenOptionsExt as _, PermissionsExt as _};
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::sync::Arc;
use std::thread;
use std::time::Duration;

use base64::Engine as _;
use layerx_agent_api::identity::{AgentDid, AuthorityRef};
use layerx_client::rpc::receipt::{
    lookup_authenticated, AuthenticatedReceipt, Lookup, LookupContext,
};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint, SequencerError};
use layerx_crypto::disclosure::call::ledger_selector;
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_human_service::custody::{KeyId, Operation};
use layerx_human_service::journeys::{
    Endpoint, MoveJourneyError, MoveLegExecution, MovePlan, Relationship, RouteRequest, SendRoute,
};
use layerx_human_service::notify::JourneyId;
use layerx_types::amount::Amount;
use layerx_types::ids::{AssetId, IdempotencyKey};
use layerx_types::intent::{
    AuthorizationSignature, ContextHash, EvmAddress, NetworkId, ProtocolVersion, PublicKey,
    SendAuthorization, SendAuthorizationKind, Sequence, TimestampSeconds,
};
use layerx_types::rung::Rung;
use serde_json::Value;

fn execution(key: u8, nonce: u64, ceiling: u128) -> MoveLegExecution {
    MoveLegExecution::new(
        [key; 32],
        AgentDid::new("did:layerx:alice").or_fail("actor"),
        AuthorityRef::new("custody-human-primary").or_fail("authority"),
        nonce,
        0,
        u64::MAX,
        ceiling,
    )
    .or_fail("execution")
}

fn request(nonce: u64, to: u8, amount: u128) -> RouteRequest {
    RouteRequest {
        source: Endpoint::PaxeerWallet,
        destination: Endpoint::PaxeerWallet,
        relationship: Relationship::BankTransfer {
            from: EvmAddress::new([0x11; 20]),
            to: EvmAddress::new([to; 20]),
            route: SendRoute {
                account_sequence: Sequence::from_u64(nonce),
                idempotency_key: IdempotencyKey::new([to; 32]),
                expires_at: TimestampSeconds::from_u64(2_000),
                context_hash: ContextHash::new([0; 32]),
                authorization: SendAuthorization::new(
                    SendAuthorizationKind::Owner,
                    PublicKey::new([0x11; 32]),
                    AuthorizationSignature::new([0x22; 64]),
                ),
                network_id: NetworkId::new(125).or_fail("network"),
                protocol_version: ProtocolVersion::new(2).or_fail("protocol"),
            },
            pointer: EvmAddress::new([0x88; 20]),
            denom: "uhpx".to_owned(),
            chain_id: 125,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 10,
            gas_limit: 100_000,
        },
        asset: AssetId::new([0x33; 32]),
        amount: Amount::from_u128(amount),
    }
}

fn plan(
    requests: Vec<RouteRequest>,
    executions: Vec<MoveLegExecution>,
) -> Result<MovePlan, MoveJourneyError> {
    MovePlan::new_bank_transfers(
        JourneyId::new("jrn_movebanksend").or_fail("journey"),
        [0x44; 32],
        KeyId::new("human-primary").or_fail("key"),
        Operation::ProtocolMutation,
        requests,
        executions,
        100,
        "HPX",
        "a few seconds",
    )
}

#[test]
fn move_plan_is_one_bank_transfer_per_leg() {
    let requests = vec![request(7, 0x22, 90), request(8, 0x55, 40)];
    let executions = vec![execution(1, 7, 1_000_000), execution(2, 8, 1_000_000)];
    let movement = plan(requests.clone(), executions.clone()).or_fail("two-leg bank move");
    assert_eq!(movement.bank_transfers().len(), 2);
    assert!(movement.route().legs().is_empty());
    for (index, call) in movement.bank_transfers().iter().enumerate() {
        assert_eq!(call.to, [0x88; 20]);
        assert_eq!(call.nonce, 7 + index as u64);
        assert_eq!(movement.executions()[index].pending_nonce(), call.nonce);
        assert_eq!(call.chain_id, 125);
        assert_eq!(
            &call.data[..4],
            &ledger_selector("transfer(address,uint256)")
        );
        assert_eq!(call.data.len(), 68);
        assert_eq!(&call.data[4..16], &[0; 12]);
        assert_eq!(
            &call.data[16..36],
            &[if index == 0 { 0x22 } else { 0x55 }; 20]
        );
        let amount = if index == 0 { 90_u128 } else { 40_u128 };
        assert_eq!(&call.data[36..52], &[0; 16]);
        assert_eq!(&call.data[52..68], &amount.to_be_bytes());
        let raw = call.sign(&[0x66; 32]).or_fail("real EIP-1559 signing");
        assert_eq!(
            decode_signed(&raw).or_fail("decode signed bank call").0,
            *call
        );
    }
    let single = MovePlan::new(
        JourneyId::new("jrn_movebanksingle").or_fail("single journey"),
        [0x44; 32],
        KeyId::new("human-primary").or_fail("key"),
        Operation::ProtocolMutation,
        requests[0].clone(),
        vec![executions[0].clone()],
        100,
        "HPX",
        "a few seconds",
    )
    .unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "existing constructor plans a bank transfer"
        )
    });
    assert_eq!(single.bank_transfers(), &movement.bank_transfers()[..1]);
    assert_eq!(single.quote().amount(), 90);
    let quote = movement.quote();
    assert_eq!(quote.amount(), 130);
    assert_eq!(quote.asset_label(), "HPX");
    assert_eq!(quote.fee_estimate(), 100);
    assert_eq!(quote.fee_ceiling(), 2_000_000);
    assert_eq!(quote.arrival_expectation(), "a few seconds");
    assert!(quote.irreversibility().contains("cannot be cancelled"));
    assert!(quote.plain_language().contains("Move 130 HPX"));
    assert!(!movement.complete());
    assert!(!movement.finalised());

    let root = layerx_human_test_support::directory("move-bank-plan");
    std::fs::create_dir_all(&root).or_fail("plan directory");
    let path = root.join("plan");
    std::fs::write(&path, movement.canonical_encode()).or_fail("persist producer's plan");
    drop(movement);
    let loaded = MovePlan::canonical_decode(&std::fs::read(&path).or_fail("read plan"))
        .or_fail("recover complete bank plan");
    assert_eq!(
        loaded,
        plan(requests.clone(), executions.clone()).or_fail("same plan")
    );
    std::fs::remove_dir_all(root).or_fail("remove plan directory");

    assert!(plan(
        requests.clone(),
        vec![execution(1, 7, 1_000_000), execution(2, 9, 1_000_000)]
    )
    .is_err());
    assert!(plan(
        vec![request(u64::MAX, 0x22, 90), request(0, 0x55, 40)],
        vec![
            execution(1, u64::MAX, 1_000_000),
            execution(2, 0, 1_000_000)
        ]
    )
    .is_err());
    assert!(plan(
        requests.clone(),
        vec![execution(1, 7, 999_999), execution(2, 8, 1_000_000)]
    )
    .is_err());
    assert!(plan(
        requests,
        vec![execution(1, 7, 1_000_000), execution(1, 8, 1_000_000)]
    )
    .is_err());
}

fn unhex(text: &str) -> Vec<u8> {
    text.strip_prefix("0x")
        .or_fail("hex prefix")
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).or_fail("hex text"), 16)
                .or_fail("hex byte")
        })
        .collect()
}

struct BankNode {
    clock: Arc<RuntimeClock>,
    script: PathBuf,
    home: PathBuf,
    endpoint: SequencerEndpoint,
    secret: [u8; 32],
}

impl BankNode {
    fn start() -> Self {
        let root = Path::new(env!("CARGO_MANIFEST_DIR"))
            .ancestors()
            .nth(4)
            .or_fail("repository root");
        let script = root.join("tools/qualification/localnode/v6120-node.sh");
        let environment = std::env::temp_dir()
            .join("lx-localnode")
            .join(format!("move-bank-{}.env", std::process::id()));
        let evidence = layerx_human_test_support::directory("move-bank-node");
        fs::create_dir_all(&evidence).or_fail("node evidence directory");
        fs::set_permissions(&evidence, fs::Permissions::from_mode(0o700))
            .or_fail("private evidence directory");
        let log_path = evidence.join("startup.log");
        let log = OpenOptions::new()
            .create_new(true)
            .write(true)
            .mode(0o600)
            .open(&log_path)
            .or_fail("private node log");
        let status = Command::new("bash")
            .arg(&script)
            .arg("--sequencer")
            .arg("--env-out")
            .arg(&environment)
            .stdout(Stdio::from(log.try_clone().unwrap_or_else(|error| {
                panic!("{}: {error:?}", "node log clone")
            })))
            .stderr(Stdio::from(log))
            .status()
            .or_fail("start genuine disposable sequencer");
        assert!(
            status.success(),
            "node startup failed; evidence: {}",
            log_path.display()
        );
        let text = fs::read_to_string(&environment).or_fail("disposable node environment");
        let fields: BTreeMap<_, _> = text
            .lines()
            .map(|line| line.split_once('=').or_fail("node environment entry"))
            .collect();
        let node = Self {
            clock: RuntimeClock::from_environment().or_fail("bank node clock"),
            script,
            home: PathBuf::from(fields["LOCAL_NODE_HOME"]),
            endpoint: SequencerEndpoint::new(fields["LOCAL_NODE_SEQUENCER_RPC"], &[])
                .or_fail("genuine sequencer endpoint"),
            secret: unhex(fields["LOCAL_NODE_KEY"])
                .try_into()
                .or_fail("EVM secret width"),
        };
        fs::remove_file(environment).or_fail("remove copied private environment");
        node
    }

    fn receipt_keys(&self) -> Vec<ReceiptKeyEntry> {
        let genesis: Value = serde_json::from_slice(
            &fs::read(self.home.join("config/genesis.json")).or_fail("governed genesis"),
        )
        .or_fail("genesis JSON");
        genesis["app_state"]["sequencer"]["receipt_keys"]
            .as_array()
            .or_fail("governed receipt key list")
            .iter()
            .map(|key| ReceiptKeyEntry {
                key_id: base64::engine::general_purpose::STANDARD
                    .decode(key["key_id"].as_str().or_fail("governed key ID"))
                    .or_fail("base64 key ID")
                    .try_into()
                    .or_fail("key ID width"),
                public_key: base64::engine::general_purpose::STANDARD
                    .decode(key["public_key"].as_str().or_fail("governed public key"))
                    .or_fail("base64 public key")
                    .try_into()
                    .or_fail("public key width"),
                first_height: key["first_height"].as_u64().or_fail("first key height"),
                last_height: key["last_height"].as_u64().or_fail("last key height"),
            })
            .collect()
    }

    fn receipt(
        &self,
        hash: [u8; 32],
        keys: &[ReceiptKeyEntry],
        rung: Rung,
    ) -> AuthenticatedReceipt {
        let mut deadline = Deadline::start(self.clock.as_ref(), Duration::from_secs(60))
            .or_fail("receipt deadline");
        loop {
            match lookup_authenticated(&self.endpoint, hash, LookupContext { receipt_keys: keys })
                .unwrap_or_else(|error| {
                    panic!("{}: {error:?}", "authenticate real sequencer receipt")
                }) {
                Lookup::Verified(receipt) if receipt.rung() >= rung => return receipt,
                Lookup::Absent | Lookup::Submitted { .. } | Lookup::Verified(_) => {}
            }
            assert!(
                !deadline
                    .remaining(self.clock.as_ref())
                    .or_fail("receipt clock")
                    .is_zero(),
                "transaction did not reach {rung:?}; evidence: {}",
                self.home.display()
            );
            thread::sleep(Duration::from_millis(10));
        }
    }
}

impl Drop for BankNode {
    fn drop(&mut self) {
        let status = Command::new("bash")
            .arg(&self.script)
            .arg("--stop")
            .arg(&self.home)
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .status()
            .or_fail("stop disposable sequencer");
        assert!(status.success(), "disposable sequencer did not stop");
    }
}

fn pending<T>(clock: &dyn Clock, mut operation: impl FnMut() -> Result<T, SequencerError>) -> T {
    let mut deadline = Deadline::start(clock, Duration::from_secs(20)).or_fail("pending deadline");
    loop {
        match operation() {
            Ok(value) => return value,
            Err(SequencerError::Rpc(refusal))
                if refusal.message == "pending block unavailable: sequencer: no block is open"
                    || refusal.message == "sequencer: no block is open" =>
            {
                assert!(
                    !deadline.remaining(clock).or_fail("pending clock").is_zero(),
                    "sequencer did not open a block"
                );
                thread::sleep(Duration::from_millis(10));
            }
            Err(error) => panic!("real sequencer refused bank operation: {error:?}"),
        }
    }
}

fn live_request(
    nonce: u64,
    to: u8,
    amount: u128,
    sender: [u8; 20],
    native_pointer: [u8; 20],
) -> RouteRequest {
    let mut request = request(nonce, to, amount);
    let Relationship::BankTransfer {
        from,
        route,
        pointer,
        max_fee_per_gas,
        gas_limit,
        ..
    } = &mut request.relationship
    else {
        panic!("bank request");
    };
    *from = EvmAddress::new(sender);
    *pointer = EvmAddress::new(native_pointer);
    route.expires_at = TimestampSeconds::from_u64(u64::MAX);
    *max_fee_per_gas = 1_000_000_000_000;
    *gas_limit = 1_000_000;
    request
}

fn native_pointer(node: &BankNode, sender: [u8; 20], keys: &[ReceiptKeyEntry]) -> [u8; 20] {
    let mut view = [0; 20];
    view[18..].copy_from_slice(&0x100a_u16.to_be_bytes());
    let mut data = ledger_selector("getNativePointer(string)").to_vec();
    let mut offset = [0; 32];
    offset[31] = 32;
    data.extend(offset);
    let mut length = [0; 32];
    length[31] = 4;
    data.extend(length);
    let mut denom = [0; 32];
    denom[..4].copy_from_slice(b"uhpx");
    data.extend(denom);
    let mut result = pending(node.clock.as_ref(), || {
        node.endpoint.call_pending(&view, &data)
    });
    assert_eq!(result.len(), 96, "native pointer view ABI");
    if result[64..] == [0; 32] {
        let mut registry = [0; 20];
        registry[18..].copy_from_slice(&0x100b_u16.to_be_bytes());
        let mut registration_data = data.clone();
        registration_data[..4].copy_from_slice(&ledger_selector("addNativePointer(string)"));
        let call = Eip1559Call {
            chain_id: 125,
            nonce: pending(node.clock.as_ref(), || node.endpoint.pending_nonce(&sender)),
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 1_000_000_000_000,
            gas_limit: 5_000_000,
            to: registry,
            data: registration_data,
        };
        let raw = call
            .sign(&node.secret)
            .or_fail("sign native pointer registration");
        let hash = transaction_hash(&raw);
        assert_eq!(
            pending(node.clock.as_ref(), || node
                .endpoint
                .send_raw_transaction(&raw)),
            hash
        );
        let receipt = node.receipt(hash, keys, Rung::Receipted);
        assert!(
            receipt.execution().success,
            "native pointer registration reverted: {:?}; node evidence: {}",
            String::from_utf8_lossy(&receipt.execution().return_data),
            node.home.display()
        );
        result = pending(node.clock.as_ref(), || {
            node.endpoint.call_pending(&view, &data)
        });
    }
    assert_eq!(result.len(), 96, "registered native pointer view ABI");
    assert_eq!(&result[..12], &[0; 12]);
    assert_eq!(&result[64..95], &[0; 31]);
    assert_eq!(result[95], 1, "native pointer must exist");
    let pointer = result[12..32].try_into().or_fail("native pointer address");
    assert_ne!(pointer, [0; 20]);
    pointer
}

#[derive(Clone, Copy)]
struct BankTransferContext {
    nonce: u64,
    ceiling: u128,
    sender: [u8; 20],
    pointer: [u8; 20],
}

fn complete_receipted_leg(
    node: &BankNode,
    keys: &[ReceiptKeyEntry],
    movement: &mut MovePlan,
    index: usize,
    context: BankTransferContext,
    root: &Path,
) -> (Vec<u8>, AuthenticatedReceipt) {
    let BankTransferContext {
        nonce,
        ceiling,
        sender,
        pointer,
    } = context;
    let raw = movement.bank_transfers()[index]
        .sign(&node.secret)
        .or_fail("sign planned bank transfer");
    let hash = transaction_hash(&raw);
    assert_eq!(
        pending(node.clock.as_ref(), || node
            .endpoint
            .send_raw_transaction(&raw)),
        hash
    );
    let receipted = node.receipt(hash, keys, Rung::Receipted);
    assert_eq!(receipted.rung(), Rung::Receipted);
    assert!(
        receipted.execution().success,
        "planned bank transfer reverted: {:?}; node evidence: {}",
        String::from_utf8_lossy(&receipted.execution().return_data),
        node.home.display()
    );
    let index_byte = u8::try_from(index).unwrap_or_else(|_| panic!("leg index fits u8"));
    let mut leg = execution(10 + index_byte, nonce + index as u64, ceiling);
    assert!(!leg.complete());
    assert!(!leg.finalised());
    assert!(leg.complete_from_receipt([0x99; 32], &receipted).is_err());
    assert!(execution(20, nonce + index as u64 + 1, ceiling)
        .complete_from_receipt(hash, &receipted)
        .is_err());
    assert!(execution(20, nonce + index as u64, 0)
        .complete_from_receipt(hash, &receipted)
        .is_err());
    leg.complete_from_receipt(hash, &receipted)
        .or_fail("receipt completes leg");
    assert!(leg.complete());
    assert_eq!(leg.rung(), Rung::Receipted);
    assert!(!leg.finalised());
    let mut unrelated = plan(
        vec![live_request(
            nonce + index as u64,
            0x77,
            90,
            sender,
            pointer,
        )],
        vec![execution(3, nonce + index as u64, ceiling)],
    )
    .or_fail("unrelated bank plan with same nonce");
    assert!(unrelated.complete_bank_leg(0, &raw, &receipted).is_err());
    assert!(!unrelated.executions()[0].complete());
    assert!(!unrelated.finalised());
    assert!(movement
        .complete_bank_leg(1 - index, &raw, &receipted)
        .is_err());
    movement
        .complete_bank_leg(index, &raw, &receipted)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "matching receipt completes planned leg"));
    assert!(movement.executions()[index].complete());
    assert_eq!(movement.complete(), index == 1);
    assert!(!movement.finalised());
    let (key, actor, authority, nonce, before, after, fee) = leg.to_wire_parts();
    let recovered = MoveLegExecution::from_wire_parts(
        key,
        actor.clone(),
        authority.clone(),
        nonce,
        before,
        after,
        fee,
    )
    .unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "recreate receipt consumer from immutable context"
        )
    });
    drop(leg);
    assert!(!recovered.complete());
    fs::write(root.join(format!("transaction-{index}")), &raw)
        .or_fail("persist signed transaction");
    (raw, receipted)
}

fn assert_reverted_transfer(
    node: &BankNode,
    keys: &[ReceiptKeyEntry],
    context: BankTransferContext,
) {
    let BankTransferContext {
        nonce,
        ceiling,
        sender,
        pointer,
    } = context;
    let reverted_plan = plan(
        vec![live_request(nonce + 2, 0x77, u128::MAX, sender, pointer)],
        vec![execution(4, nonce + 2, ceiling)],
    )
    .or_fail("insufficient balance transfer plan");
    let mut invalid_call = reverted_plan.bank_transfers()[0].clone();
    invalid_call.data[..4].copy_from_slice(&[0xff; 4]);
    let raw = invalid_call
        .sign(&node.secret)
        .or_fail("sign reverting bank call");
    let hash = transaction_hash(&raw);
    assert_eq!(
        pending(node.clock.as_ref(), || node
            .endpoint
            .send_raw_transaction(&raw)),
        hash
    );
    let reverted = node.receipt(hash, keys, Rung::Receipted);
    assert!(!reverted.execution().success);
    assert!(execution(4, reverted.execution().nonce, ceiling)
        .complete_from_receipt(reverted.activity_id(), &reverted)
        .is_err());
}

#[test]
fn move_leg_completes_from_its_receipt() {
    let node = BankNode::start();
    assert_eq!(node.endpoint.chain_id().or_fail("chain ID"), 125);
    let probe = crate::request(0, 0x22, 90);
    let probe_plan =
        plan(vec![probe], vec![execution(1, 0, 1_000_000)]).or_fail("bank signing probe");
    let probe_raw = probe_plan.bank_transfers()[0]
        .sign(&node.secret)
        .or_fail("sign probe");
    let (probe_call, probe_signature) = decode_signed(&probe_raw).or_fail("decode probe");
    let sender = recover_sender(&probe_call, &probe_signature).or_fail("real sender");
    let keys = node.receipt_keys();
    assert!(!keys.is_empty());
    let pointer = native_pointer(&node, sender, &keys);
    let nonce = pending(node.clock.as_ref(), || node.endpoint.pending_nonce(&sender));
    let ceiling = 1_000_000_000_000_000_000;
    let requests = vec![
        live_request(nonce, 0x22, 90, sender, pointer),
        live_request(nonce + 1, 0x55, 40, sender, pointer),
    ];
    let mut movement = plan(
        requests.clone(),
        vec![
            execution(1, nonce, ceiling),
            execution(2, nonce + 1, ceiling),
        ],
    )
    .or_fail("two-leg live bank plan");
    let root = layerx_human_test_support::directory("move-bank-recovery");
    fs::create_dir_all(&root).or_fail("recovery directory");
    let plan_path = root.join("plan");
    fs::write(&plan_path, movement.canonical_encode()).or_fail("persist real producer plan");
    let mut raw_transactions = Vec::new();
    let mut receipts = Vec::new();
    let mut before = Vec::new();
    for recipient in [[0x22; 20], [0x55; 20]] {
        before.push(pending(node.clock.as_ref(), || {
            node.endpoint.balance(&recipient)
        }));
    }
    let context = BankTransferContext {
        nonce,
        ceiling,
        sender,
        pointer,
    };
    for index in 0..2 {
        let (raw, receipted) =
            complete_receipted_leg(&node, &keys, &mut movement, index, context, &root);
        raw_transactions.push(raw);
        receipts.push(receipted);
    }
    assert_eq!(
        fs::read(&plan_path).or_fail("immutable plan"),
        movement.canonical_encode()
    );
    drop(movement);
    let mut recovered =
        MovePlan::canonical_decode(&fs::read(&plan_path).or_fail("read producer plan"))
            .or_fail("restart consumer from durable plan");
    for index in 0..2 {
        let raw = fs::read(root.join(format!("transaction-{index}")))
            .or_fail("recover original signed transaction");
        assert_eq!(raw, raw_transactions[index]);
        let hash = transaction_hash(&raw);
        let committed = node.receipt(hash, &keys, Rung::Committed);
        assert_eq!(committed.rung(), Rung::Committed);
        assert_eq!(
            committed.canonical_bytes(),
            receipts[index].canonical_bytes()
        );
        recovered
            .complete_bank_leg(index, &raw, &committed)
            .or_fail("receipt finalises leg after restart");
        assert!(recovered.executions()[index].complete());
        assert!(recovered.executions()[index].finalised());
        assert_eq!(recovered.executions()[index].rung(), Rung::Committed);
        assert!(recovered
            .complete_bank_leg(index, &raw, &receipts[index])
            .is_err());
        assert!(recovered.executions()[index].finalised());
        assert_eq!(recovered.finalised(), index == 1);
        let recipient = [if index == 0 { 0x22 } else { 0x55 }; 20];
        let after = pending(node.clock.as_ref(), || node.endpoint.balance(&recipient));
        let before =
            u128::from_be_bytes(before[index][16..].try_into().or_fail("recipient balance"));
        assert_eq!(
            u128::from_be_bytes(after[16..].try_into().or_fail("recipient balance")) - before,
            (if index == 0 { 90 } else { 40 }) * 1_000_000_000_000_u128
        );
    }
    assert!(recovered.complete());
    assert!(recovered.finalised());
    assert_reverted_transfer(&node, &keys, context);
    fs::remove_dir_all(root).or_fail("remove recovery directory");
}

use crate::require_value::RequireValue as _;

pub mod require_value {
    include!("support/require_value.rs");
}
