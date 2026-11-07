use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;

use base64::Engine as _;
use layerx_agent_api::identity::{AgentDid, AuthorityRef};
use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint};
use layerx_crypto::disclosure::call::{bind_call, ConstructMethod};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_human_service::journeys::*;
use layerx_human_service::server::schema::ApiSchema;
use layerx_human_service::server::HumanComponentServer;
use layerx_types::account::AccountId;
use layerx_types::amount::Amount;
use layerx_types::ids::{AssetId, IdempotencyKey};
use layerx_types::intent::{
    AuthorizationSignature, ContextHash, EvmAddress, NetworkId, ProtocolVersion, PublicKey,
    SendAuthorization, SendAuthorizationKind, Sequence, TimestampSeconds,
};
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const INTENT_SCHEMA: &str = include_str!("../../../schema/human-api/intent.kvx");
const JOURNEY_SCHEMA: &str = include_str!("../../../schema/human-api/journeys.kvx");

fn prose(schema: &str, section: &str, key: &str) -> String {
    let section = schema
        .split_once(&format!("[{section}]\n"))
        .expect("declared contract section")
        .1
        .split("\n[")
        .next()
        .expect("section contents");
    let value = section
        .lines()
        .find_map(|line| line.strip_prefix(&format!("{key} = ")))
        .expect("declared semantic property");
    serde_json::from_str(value).expect("quoted contract prose")
}

fn transcript() -> Value {
    serde_json::from_str(include_str!(
        "../src/journeys/repair_22_31_153_transcript.json"
    ))
    .expect("committed node and consensus producer transcript")
}

fn unhex(text: &str) -> Vec<u8> {
    let text = text.strip_prefix("0x").unwrap_or(text);
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).expect("hex text"), 16).expect("hex byte")
        })
        .collect()
}

fn sender() -> [u8; 20] {
    unhex(transcript()["sender"].as_str().expect("producer sender"))
        .try_into()
        .expect("address width")
}

fn asset() -> AssetId {
    AssetId::new(Sha256::digest(b"uhpx").into())
}

struct RpcPeer {
    endpoint: SequencerEndpoint,
    calls: Arc<Mutex<Vec<Value>>>,
    stop: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
}

impl RpcPeer {
    fn start() -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("loopback transport");
        listener
            .set_nonblocking(true)
            .expect("nonblocking listener");
        let endpoint = SequencerEndpoint::new(
            &format!("http://{}/", listener.local_addr().expect("address")),
            &[],
        )
        .expect("production client endpoint");
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = stop.clone();
        let calls = Arc::new(Mutex::new(Vec::new()));
        let observed = calls.clone();
        let document = transcript();
        let worker = thread::spawn(move || {
            while !stopping.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(3)))
                    .expect("read deadline");
                stream
                    .set_write_timeout(Some(Duration::from_secs(3)))
                    .expect("write deadline");
                let mut reader = BufReader::new(stream);
                let mut length = None;
                loop {
                    let mut line = String::new();
                    assert_ne!(reader.read_line(&mut line).expect("HTTP header"), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = Some(value.trim().parse::<usize>().expect("body length"));
                    }
                }
                let length = length.expect("content length");
                assert!(length <= 65_536);
                let mut body = vec![0; length];
                reader.read_exact(&mut body).expect("RPC body");
                let request: Value = serde_json::from_slice(&body).expect("RPC request");
                let record = &document["records"][0];
                let result = match request["method"].as_str().expect("RPC method") {
                    "eth_getTransactionCount" => {
                        assert_eq!(request["params"][1], "pending");
                        json!(format!(
                            "0x{:x}",
                            document["nonce"].as_u64().expect("recorded nonce")
                        ))
                    }
                    "pax_sequencerReceipt" => {
                        assert_eq!(request["params"][0], record["view"]["txHash"]);
                        record["view"].clone()
                    }
                    "eth_getTransactionReceipt" => {
                        assert_eq!(request["params"][0], record["view"]["txHash"]);
                        record["landing"].clone()
                    }
                    "px_getCommit" => record["material"].clone(),
                    "px_getValidators" => record["validators"].clone(),
                    "eth_getBlockByNumber" if request["params"][1] == false => {
                        json!({"number":request["params"][0], "stateRoot":record["appHash"]})
                    }
                    "eth_getBlockByNumber" => record["block"].clone(),
                    other => panic!("unrecorded producer method {other}"),
                };
                observed.lock().expect("calls").push(request.clone());
                let body =
                    json!({"jsonrpc":"2.0", "id":request["id"], "result":result}).to_string();
                let mut stream = reader.into_inner();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).expect("RPC response");
                stream.flush().expect("response flush");
            }
        });
        Self {
            endpoint,
            calls,
            stop,
            worker: Some(worker),
        }
    }
}

impl Drop for RpcPeer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.worker
            .take()
            .expect("worker")
            .join()
            .expect("RPC peer");
    }
}

#[test]
fn intent_schema_dual_domain_bank_call_and_pending_nonce() {
    let peer = RpcPeer::start();
    let nonce = peer
        .endpoint
        .pending_nonce(&sender())
        .expect("pending node nonce");
    let request = RouteRequest {
        source: Endpoint::PaxeerWallet,
        destination: Endpoint::PaxeerWallet,
        relationship: Relationship::BankTransfer {
            from: EvmAddress::new(sender()),
            to: EvmAddress::new([0x22; 20]),
            route: SendRoute {
                account_sequence: Sequence::from_u64(nonce),
                idempotency_key: IdempotencyKey::new([0x31; 32]),
                expires_at: TimestampSeconds::from_u64(u64::MAX),
                context_hash: ContextHash::new([0; 32]),
                authorization: SendAuthorization::new(
                    SendAuthorizationKind::Owner,
                    PublicKey::new([0x11; 32]),
                    AuthorizationSignature::new([0x22; 64]),
                ),
                network_id: NetworkId::new(125).expect("network"),
                protocol_version: ProtocolVersion::new(3).expect("protocol"),
            },
            pointer: EvmAddress::new(BANK_ADDRESS),
            denom: "uhpx".to_owned(),
            chain_id: 125,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 1_000_000_000_000,
            gas_limit: 1_000_000,
        },
        asset: asset(),
        amount: Amount::from_u128(90),
    };
    let decoded =
        RouteRequest::canonical_decode(&request.canonical_encode()).expect("real route codec");
    assert_eq!(decoded, request);
    let route = RouteResolver::resolve(&decoded).expect("production bank resolver");
    let [call] = route.bank_transfers() else {
        panic!("one bank call")
    };
    assert_eq!(call.to, BANK_ADDRESS);
    assert_eq!(call.nonce, nonce);
    assert_eq!(call.chain_id, 125);
    assert!(prose(INTENT_SCHEMA, "type.IntentLeg", "evm_call").contains("value zero"));
    assert_eq!(&call.data[..4], &unhex("5c05961b"));
    let mut amount = [0; 32];
    amount[31] = 90;
    assert_eq!(
        bind_call(call, sender())
            .expect("production disclosure consumer")
            .construct_method()
            .expect("bank ABI"),
        Some(ConstructMethod::BankSend {
            from: sender(),
            to: [0x22; 20],
            denom: "uhpx".to_owned(),
            amount,
        })
    );
    let secret: [u8; 32] = unhex(transcript()["secret"].as_str().expect("synthetic signer"))
        .try_into()
        .expect("secret width");
    let raw = call.sign(&secret).expect("production transaction encoder");
    let (transaction, signature) = decode_signed(&raw).expect("production transaction consumer");
    assert_eq!(transaction, *call);
    assert_eq!(
        recover_sender(&transaction, &signature).expect("sender binding"),
        sender()
    );
    assert_eq!(peer.calls.lock().expect("calls").len(), 1);
    assert!(prose(INTENT_SCHEMA, "module", "spine").contains("single merged Paxeer network"));
    assert!(
        prose(INTENT_SCHEMA, "type.IntentDomain", "meaning").contains("compatibility vocabulary")
    );
    assert!(prose(INTENT_SCHEMA, "type.IntentLeg", "evm_call").contains("pending EVM nonce"));
    assert!(prose(INTENT_SCHEMA, "type.IntentLeg", "send").contains("0x5c05961b"));
    for retired in [
        LegMechanism::PaxeerCustodyDeposit,
        LegMechanism::PaxeerWithdrawFinalise,
    ] {
        assert_eq!(ExecutableShape::of_mechanisms(&[retired]), None);
    }
}

#[test]
fn intent_schema_dual_domain_strict_replan_and_unknown_refusal() {
    let owner = AccountId::parse("agent:did:layerx:alice:main").expect("owner");
    let home = Endpoint::human(owner.clone()).expect("home");
    let destination =
        Endpoint::agent(AccountId::parse("agent:did:layerx:bob:main").expect("agent"))
            .expect("destination");
    let snapshot = |known: bool| {
        ObservedState::new(
            TimestampSeconds::from_u64(100),
            owner.clone(),
            None,
            if known {
                vec![BalanceEntry::new(
                    home.clone(),
                    asset(),
                    Amount::from_u128(500),
                )]
            } else {
                vec![]
            },
            vec![],
            vec![],
            FeeSchedule::new(vec![(LegMechanism::Protocol(Mechanism::Send), 2)]).expect("fees"),
        )
        .expect("real observations")
    };
    let intent = |amount| {
        UnifiedIntent::new(
            home.clone(),
            destination.clone(),
            asset(),
            Amount::from_u128(amount),
            Constraints::new(TimestampSeconds::from_u64(200), 100, false),
        )
        .expect("intent")
    };
    let planned = plan(&intent(25), &snapshot(true)).expect("production planner");
    assert_eq!(planned.legs()[0].domain().as_str(), "layerx");
    assert_eq!(
        planned,
        plan(&intent(25), &snapshot(true)).expect("deterministic replan")
    );
    assert_eq!(
        plan(&intent(25), &snapshot(false)),
        Err(Refusal::BalanceUnknown {
            endpoint: home.clone(),
            asset: asset()
        })
    );
    let replanned = plan(&intent(26), &snapshot(true)).expect("changed intent");
    assert_ne!(planned.digest(), replanned.digest());
    assert_ne!(
        planned.action_key(0).expect("action"),
        replanned.action_key(0).expect("action")
    );
    assert_ne!(
        planned.signing_context(0).expect("context"),
        replanned.signing_context(0).expect("context")
    );
    let expectation = BindingExpectation {
        actor: AgentDid::new("did:layerx:alice").expect("actor"),
        authority: AuthorityRef::new("custody-human-primary").expect("authority"),
        account_sequence: 7,
        currency: "HPX".to_owned(),
        now: 100,
    };
    let action: String = planned
        .action_key(0)
        .expect("action")
        .iter()
        .map(|b| format!("{b:02x}"))
        .collect();
    let binding = IntentLegBinding::from_json(&json!({
        "leg_index":0, "action_key":action, "actor":expectation.actor.as_str(),
        "authority":authority_label(planned.legs()[0].authority()), "relationship":"self",
        "account_sequence":7, "not_before":90, "not_after":150,
        "fee_limit":{"amount":"2", "currency":"HPX"}
    }))
    .expect("production binding decoder");
    let mut request = SubmitPlanRequest {
        plan_digest: planned.digest(),
        signed_digest: planned.digest(),
        bindings: vec![binding],
    };
    assert_eq!(verify_bindings(&planned, &request, &expectation), Ok(()));
    assert_eq!(
        verify_bindings(&replanned, &request, &expectation),
        Err(SubmitRefusal::PlanDigestMismatch)
    );
    request.bindings[0].account_sequence = 8;
    assert_eq!(
        verify_bindings(&planned, &request, &expectation),
        Err(SubmitRefusal::StaleSequence { index: 0 })
    );
    assert!(prose(INTENT_SCHEMA, "module", "never_partial").contains("unobserved balance as zero"));
    assert!(
        prose(INTENT_SCHEMA, "type.SubmitPlanRequest", "replan").contains("no journey is created")
    );
}

#[test]
fn journey_schema_finality_copy_authenticates_committed_receipt() {
    let document = transcript();
    let record = &document["records"][0];
    let key = &document["key"];
    let keys = vec![ReceiptKeyEntry {
        key_id: unhex(key["keyId"].as_str().expect("key id"))
            .try_into()
            .expect("id width"),
        public_key: unhex(key["publicKey"].as_str().expect("public key"))
            .try_into()
            .expect("key width"),
        first_height: key["firstHeight"].as_u64().expect("first height"),
        last_height: key["lastHeight"].as_u64().expect("last height"),
    }];
    let raw = unhex(record["raw"].as_str().expect("producer transaction"));
    let hash = transaction_hash(&raw);
    let peer = RpcPeer::start();
    let Lookup::Verified(receipt) = lookup_authenticated(
        &peer.endpoint,
        hash,
        LookupContext {
            receipt_keys: &keys,
        },
    )
    .expect("authenticated real producer receipt over HTTP") else {
        panic!("verified receipt")
    };
    assert_eq!(receipt.activity_id(), hash);
    assert_eq!(receipt.rung(), Rung::Receipted);
    let batch = unhex(record["batch"].as_str().expect("signed sequence batch"));
    let proof = &record["proof"];
    let mut leaf = b"LXAR\x01".to_vec();
    for name in ["total", "index"] {
        leaf.extend(
            proof[name]
                .as_str()
                .expect("proof number")
                .parse::<i64>()
                .expect("integer")
                .to_be_bytes(),
        );
    }
    leaf.extend(Sha256::digest(&batch));
    let mut fields = vec![leaf];
    fields.extend(
        proof["aunts"]
            .as_array()
            .expect("proof aunts")
            .iter()
            .map(|aunt| {
                base64::engine::general_purpose::STANDARD
                    .decode(aunt.as_str().expect("aunt"))
                    .expect("base64 proof")
            }),
    );
    let evidence = BankCommitEvidence {
        chain_id: record["material"]["signedHeader"]["header"]["chain_id"]
            .as_str()
            .expect("chain")
            .to_owned(),
        trusted_validators: record["validators"].clone(),
        committed_header: record["material"]["signedHeader"].clone(),
        covering_height: record["height"].as_u64().expect("height"),
        batch_transaction: batch,
        transaction_proof: fields,
        activity_ids: vec![hash],
    };
    let committed = evidence
        .authenticate(&peer.endpoint, hash, &keys)
        .expect("production quorum and inclusion verifier");
    assert_eq!(committed.rung(), Rung::Committed);
    assert_eq!(
        committed.receipt().canonical_bytes(),
        receipt.canonical_bytes()
    );
    let mut damaged = evidence.clone();
    damaged.batch_transaction[0] ^= 1;
    assert!(damaged.authenticate(&peer.endpoint, hash, &keys).is_err());
    let mut forged = keys.clone();
    forged[0].public_key[0] ^= 1;
    assert!(lookup_authenticated(
        &peer.endpoint,
        hash,
        LookupContext {
            receipt_keys: &forged
        }
    )
    .is_err());
    for rung in [Rung::Submitted, receipt.rung(), committed.rung()] {
        let mapping = format!(
            "{}:{}",
            rung.name(),
            HumanComponentServer::verification_level_name(rung)
        );
        assert!(JOURNEY_SCHEMA.contains(&format!("\"{mapping}\"")));
    }
    assert_eq!(
        HumanComponentServer::verification_level_name(committed.rung()),
        "committed"
    );
    assert!(prose(JOURNEY_SCHEMA, "module", "network").contains("committed is final"));
    assert!(prose(
        JOURNEY_SCHEMA,
        "type.JourneyState",
        "done-finalised.semantics"
    )
    .contains("rather than an additional proof"));
    assert!(prose(JOURNEY_SCHEMA, "type.EvidenceClass", "meaning")
        .contains("Legacy evidence classes remain decodable"));
    assert!(
        prose(JOURNEY_SCHEMA, "type.JourneyState", "closed.semantics").contains("after reopening")
    );
    let schema = ApiSchema::v1().expect("production schema consumer accepts the revised contract");
    assert!(schema.operation("journey.get").is_some());
    assert!(schema.operation("evidence.get").is_some());
    let methods: Vec<_> = peer
        .calls
        .lock()
        .expect("calls")
        .iter()
        .map(|request| request["method"].as_str().expect("method").to_owned())
        .collect();
    for required in [
        "pax_sequencerReceipt",
        "px_getCommit",
        "px_getValidators",
        "eth_getBlockByNumber",
    ] {
        assert!(methods.iter().any(|method| method == required));
    }
}
