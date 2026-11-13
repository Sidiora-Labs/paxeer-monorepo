//! Task 22.33.1525 repair record (scope forbids editing the spec task record).
//! offline-export-unwired -> `offline_export_producer_rpc_consumer_and_mismatched_fact_refusal`.
//! human-socket-restart -> `human_socket_stop_crash_restart_and_foreign_live_symlink_refusal`.
//! human-total-deadline -> `human_whole_frame_deadline_preserves_another_admitted_peer`.
//! human-limits-config-unapplied -> `human_configured_limits_refuse_and_release_capacity`.
//! retired-environment-registry -> `startup_environment_accepts_height_node_and_refuses_retired_controls`.
//! managed-evidence-lifetime-cap -> `managed_recent_evidence_is_bounded_and_complete_index_survives_reopen`.
//! managed-effect-binding -> `managed_limit_binds_owner_agent_amount_budget_and_committed_effect_after_reopen`.
//! Wiring request: Domain 17 owns spec/layerx-merge/interfaces.kvx and generated
//! contracts/agent-config.md; retire sequence/reconnect/export-set rows and publish
//! `MCP_CORE_HEIGHT`, `MCP_EXPIRY_HEIGHT`, `NODE_RPC_URL` and `NODE_CHAIN_ID` with their real readers.
//! Baseline: no gate ran on the unchanged tree; the target did not exist. The explicit
//! implementation-first and hf-done-only gate order precludes the contradictory baseline step.
//! No pre-change test failure or gate success is claimed.
fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

use ed25519_dalek::{Signer, SigningKey};
use layerx_agent_api::{error::RequestId, export::FactRef, read::ReadRequest};
use layerx_agentd::{
    agent_rpc, agent_rpc_adapters,
    human::{
        self, HumanFinalizationEvidence, HumanListenerConfig, HumanOperationError, HumanRequest,
        HumanUnixServer, HumanWireResponse,
    },
    identity::{self, ProtocolAuthority},
    managed_agent::{self, ManagedAgent},
    outbox::Outbox,
    read::{
        self, BlockCommit, BlockId, CheckpointHeader, CommitTimestamp, CommitValidator, CommitVote,
        NativeResultSource, ServedCheckpoint,
    },
    receipt,
    session::{self, OpenRequest, SessionId, SessionRegistry},
    store::{Store, TenantId},
};
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_types::{ids::Did, rung::Rung, verify::VerificationLevel};
use layerx_wire::{encode::Encoder, hash::receipt_digest};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::{
    collections::{BTreeMap, BTreeSet},
    io::{BufRead, BufReader, Read, Write},
    net::{TcpListener, TcpStream},
    os::unix::{
        fs::{MetadataExt, PermissionsExt},
        net::{UnixListener, UnixStream},
    },
    path::PathBuf,
    sync::{
        atomic::{AtomicBool, AtomicU64, Ordering},
        Arc, Mutex,
    },
    thread,
    time::{Duration, Instant},
};

const HEIGHT: u64 = 1000;
const NOW: u64 = 1_750_000_400_000;
static NEXT: AtomicU64 = AtomicU64::new(1);
fn must<T, E: std::fmt::Debug>(r: Result<T, E>) -> T {
    r.unwrap_or_else(|e| panic!("{e:?}"))
}
fn hash(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}
fn rpc_hex(bytes: &[u8]) -> String {
    format!("0x{}", hex::encode(bytes))
}
fn decode_hex(v: &Value) -> Vec<u8> {
    must(hex::decode(require_some(
        v.as_str()
            .unwrap_or_else(|| panic!("{}", "required test value"))
            .strip_prefix("0x"),
        "required test value",
    )))
}
struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        let p = std::env::temp_dir().join(format!(
            "repair1525-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::SeqCst)
        ));
        must(std::fs::create_dir(&p));
        must(std::fs::set_permissions(
            &p,
            std::fs::Permissions::from_mode(0o700),
        ));
        Self(p)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}
fn actor() -> Did {
    must(Did::new(
        format!("did:layerx:{}", hex::encode([3; 32])).as_bytes(),
    ))
}
fn owner_word() -> [u8; 32] {
    let call = Eip1559Call {
        chain_id: 125,
        nonce: 1,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to: [0; 20],
        data: vec![],
    };
    let (call, sig) = must(decode_signed(&must(call.sign(&[1; 32]))));
    let mut word = [0; 32];
    word[12..].copy_from_slice(&must(recover_sender(&call, &sig)));
    word
}
fn seed() -> human::HumanAgentLifecycleSeed {
    human::HumanAgentLifecycleSeed {
        agent_id: format!("agt_{}", hex::encode([8; 32])),
        name: "agent".into(),
        purpose: "repair".into(),
        currency: "PAX".into(),
        monthly_limit: 100,
        period_start: 1,
        period_end: NOW / 1000 + 10000,
        created_at: 1,
        updated_at: 1,
        verified_evidence: vec![[1; 32]],
        actor: require_ok(
            String::from_utf8(actor().as_bytes().to_vec()),
            "required test value",
        ),
        primary_authority: hex::encode(owner_word()),
        custody_key: "owner".into(),
        custody_public_key: [3; 32],
        owner_account: format!(
            "agent:{}:main",
            require_ok(
                std::str::from_utf8(actor().as_bytes()),
                "required test value"
            )
        ),
        budget_account: format!(
            "agent:{}:budget:managed",
            require_ok(
                std::str::from_utf8(actor().as_bytes()),
                "required test value"
            )
        ),
        budget_asset: [4; 32],
        purpose_hash: [5; 32],
        recovery_root: [6; 32],
        recovery_threshold: 1,
        capability_id: [7; 32],
        activity_types: vec![7],
        counterparties: vec![[4; 32]],
        assets: vec![[4; 32]],
        amount_ceiling: 100,
        rate_maximum_uses: 10,
        rate_window_sequences: 10,
        purposes: vec!["repair".into()],
        capability_expiry_sequence: HEIGHT + 10000,
        session_scopes: vec!["prepare".into()],
        session_expiry_unix_seconds: NOW / 1000 + 10000,
        protocol_grant_id: [7; 32],
        budget_period_seconds: 100,
        budget_expiry_seconds: NOW / 1000 + 10000,
        initial_funding: 100,
        network_id: 125,
        creation_receipt_roots: vec![[1; 32]],
    }
}
fn validators() -> (Vec<SigningKey>, Vec<CommitValidator>) {
    let keys = (1..=4)
        .map(|n| SigningKey::from_bytes(&[n; 32]))
        .collect::<Vec<_>>();
    let validators = keys
        .iter()
        .map(|key| {
            let public_key = key.verifying_key().to_bytes();
            CommitValidator {
                address: CommitValidator::address_for_key(&public_key),
                public_key,
                voting_power: 1,
            }
        })
        .collect();
    (keys, validators)
}
fn committed(
    height: u64,
    seconds: i64,
    parent: [u8; 32],
    result_root: [u8; 32],
    signers: usize,
) -> ServedCheckpoint {
    let (keys, validators) = validators();
    let header = CheckpointHeader {
        version_block: 11,
        version_app: 1,
        chain_id: "125".into(),
        height,
        time: CommitTimestamp { seconds, nanos: 0 },
        last_block_id: Some(BlockId {
            hash: parent,
            part_set_total: 1,
            part_set_hash: [2; 32],
        }),
        last_commit_hash: Some([3; 32]),
        data_hash: Some([4; 32]),
        validators_hash: read::validator_set_hash(&validators),
        next_validators_hash: read::validator_set_hash(&validators),
        consensus_hash: [5; 32],
        app_hash: [6; 32],
        last_results_hash: Some(result_root),
        evidence_hash: Some([7; 32]),
        proposer_address: validators[0].address,
    };
    let mut commit = BlockCommit {
        height,
        round: 0,
        block_id: BlockId {
            hash: header.block_hash(),
            part_set_total: 1,
            part_set_hash: [8; 32],
        },
        votes: Vec::new(),
    };
    for (key, v) in keys.iter().zip(&validators).take(signers) {
        let timestamp = header.time;
        commit.votes.push(CommitVote {
            validator_address: v.address,
            voting_power: v.voting_power,
            timestamp,
            signature: key
                .sign(&commit.vote_sign_bytes("125", timestamp))
                .to_bytes(),
        });
    }
    // The negative quorum fixture is intentionally carried to the production verifier.
    ServedCheckpoint {
        height,
        block_hash: header.block_hash(),
        app_hash: header.app_hash,
        commit,
        rung: Rung::Committed,
        header,
        validators,
    }
}
fn protobuf_varint(mut value: u64, bytes: &mut Vec<u8>) {
    while value >= 128 {
        bytes.push(value.to_le_bytes()[0] | 128);
        value >>= 7;
    }
    bytes.push(value.to_le_bytes()[0]);
}
fn protobuf_bytes(number: u64, value: &[u8], bytes: &mut Vec<u8>) {
    protobuf_varint(number << 3 | 2, bytes);
    protobuf_varint(value.len() as u64, bytes);
    bytes.extend_from_slice(value);
}
fn protobuf_integer(number: u64, value: u64, bytes: &mut Vec<u8>) {
    if value != 0 {
        protobuf_varint(number << 3, bytes);
        protobuf_varint(value, bytes);
    }
}

// api/evm/tx.proto and cosmos/base/abci/v1beta1/abci.proto, independently
// encoded by the peer rather than using NativeResultSource's result encoder.
fn abci_response(receipt: &Value) -> Vec<u8> {
    let q = |name: &str| {
        require_ok(
            u64::from_str_radix(
                require_some(receipt[name].as_str(), "required test value")
                    .trim_start_matches("0x"),
                16,
            ),
            "required test value",
        )
    };
    let mut response = Vec::new();
    protobuf_integer(1, q("gasUsed"), &mut response);
    if receipt["vmError"] != "" {
        protobuf_bytes(
            2,
            require_some(receipt["vmError"].as_str(), "required test value").as_bytes(),
            &mut response,
        );
    }
    protobuf_bytes(3, &decode_hex(&receipt["returnData"]), &mut response);
    protobuf_bytes(
        4,
        require_some(receipt["txHash"].as_str(), "required test value").as_bytes(),
        &mut response,
    );
    for log in require_some(receipt["logs"].as_array(), "required test value") {
        let mut bytes = Vec::new();
        protobuf_bytes(
            1,
            require_some(log["address"].as_str(), "required test value").as_bytes(),
            &mut bytes,
        );
        for topic in require_some(log["topics"].as_array(), "required test value") {
            protobuf_bytes(
                2,
                require_some(topic.as_str(), "required test value").as_bytes(),
                &mut bytes,
            );
        }
        protobuf_bytes(3, &decode_hex(&log["data"]), &mut bytes);
        protobuf_integer(
            4,
            require_ok(
                u64::from_str_radix(
                    require_some(log["logIndex"].as_str(), "required test value")
                        .trim_start_matches("0x"),
                    16,
                ),
                "required test value",
            ),
            &mut bytes,
        );
        protobuf_bytes(5, &bytes, &mut response);
    }
    let mut msg = Vec::new();
    protobuf_bytes(1, b"/paxprotocol.paxchain.evm.MsgEVMTransaction", &mut msg);
    protobuf_bytes(2, &response, &mut msg);
    let mut tx = Vec::new();
    protobuf_bytes(1, &msg, &mut tx);
    tx
}

// Committed peer fixtures reuse the repository's native portable-export producer transcript.
mod portable_fixture {
    use super::{require_ok, require_some};

    use ed25519_dalek::{Signer as _, SigningKey};
    use layerx_agent_api::export::{FactRef, OfflineExport};
    use layerx_agent_api::prepare::CanonicalBytes;
    use layerx_agent_api::proof::ProofBundle;
    use layerx_agentd::export::{
        encode_commit, encode_header, encode_storage_witnesses, encode_validators,
        ExportTrustSource,
    };
    use layerx_agentd::read::{
        validator_set_hash, BlockCommit, BlockId, CheckpointHeader, CommitTimestamp,
        CommitValidator, CommitVote,
    };
    use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint};
    use layerx_crypto::evm_transaction::{transaction_hash, Eip1559Call};
    use layerx_types::rung::Rung;
    use layerx_wire::encode::Encoder;
    use layerx_wire::hash::receipt_digest;
    use layerx_wire::limits::{MAX_MESSAGE_BYTES, MAX_SEQUENCER_EFFECTS, PROTOCOL_VERSION};
    use sha2::{Digest as _, Sha256};

    pub fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
        value.unwrap_or_else(|error| panic!("export fixture: {error:?}"))
    }

    pub fn canonical(bytes: Vec<u8>) -> CanonicalBytes {
        must(CanonicalBytes::new(bytes))
    }

    pub fn rlp(payload: &[u8], list: bool) -> Vec<u8> {
        if !list && payload.len() == 1 && payload[0] < 128 {
            return payload.to_vec();
        }
        let prefix = if list { 0xc0 } else { 0x80 };
        let mut out = if payload.len() < 56 {
            vec![prefix + require_ok(u8::try_from(payload.len()), "test value fits u8")]
        } else {
            let length = payload.len().to_be_bytes();
            let length = &length[length.iter().position(|byte| *byte != 0).unwrap_or(0)..];
            [
                &[prefix + 55 + require_ok(u8::try_from(length.len()), "test value fits u8")][..],
                length,
            ]
            .concat()
        };
        out.extend_from_slice(payload);
        out
    }

    pub struct Fixture {
        pub artifact: OfflineExport,
        pub trust: ExportTrustSource,
        pub header: CheckpointHeader,
        pub account_id: [u8; 32],
        pub account_proof: Vec<u8>,
    }

    fn signed_receipt(activity: [u8; 32], height: u64, status: bool, key: &SigningKey) -> Vec<u8> {
        let encode = |signature: Option<[u8; 64]>| {
            let mut summary = Vec::new();
            summary.extend_from_slice(&21_000_u64.to_be_bytes());
            summary.extend_from_slice(&21_000_u64.to_be_bytes());
            summary.extend_from_slice(&1_u64.to_be_bytes());
            summary.extend_from_slice(&0_u32.to_be_bytes());
            summary.extend_from_slice(&[0; 40]);
            summary.extend_from_slice(&Sha256::digest(b""));
            summary.extend_from_slice(&0_u32.to_be_bytes());
            summary.extend_from_slice(&0_u32.to_be_bytes());
            summary.push(4);
            summary.extend_from_slice(b"upax");
            let sequence_hash: [u8; 32] = Sha256::digest(
                [
                    b"PAXEER/v1/sequence\0".as_slice(),
                    &[4; 32],
                    &7_u64.to_be_bytes(),
                    &activity,
                ]
                .concat(),
            )
            .into();
            let mut block = [0; 32];
            block[24..].copy_from_slice(&height.to_be_bytes());
            let mut from = [0; 32];
            from[12..].copy_from_slice(&[0xaa; 20]);
            let mut to = [0; 32];
            to[12..].copy_from_slice(&[0xbb; 20]);
            let mut encoder = Encoder::new(MAX_MESSAGE_BYTES);
            must(encoder.structure_header_version(0x5201, PROTOCOL_VERSION));
            must(encoder.u16(PROTOCOL_VERSION));
            must(encoder.bytes(&activity, 32));
            must(encoder.u64(7));
            must(encoder.bytes(&[4; 32], 32));
            must(encoder.bytes(&sequence_hash, 32));
            must(encoder.bytes(&Sha256::digest(b""), 32));
            must(encoder.i32(if status { 0 } else { -1 }));
            must(encoder.sequence_length(1, MAX_SEQUENCER_EFFECTS));
            must(encoder.u16(12));
            must(encoder.u16(0));
            must(encoder.u16(0));
            must(encoder.tag(1, 3));
            must(encoder.u8(0));
            must(encoder.bytes(&[0; 32], 32));
            must(encoder.bytes(&summary, 256));
            must(encoder.u128(21_000));
            must(encoder.bytes(&block, 32));
            must(encoder.u16(12));
            must(encoder.u32(1));
            must(encoder.u32(0));
            must(encoder.u8(2));
            must(encoder.bytes(&Sha256::digest(b"upax"), 32));
            must(encoder.u128(1));
            must(encoder.bytes(&from, 32));
            must(encoder.u128(100));
            must(encoder.u128(99));
            must(encoder.u64(1));
            must(encoder.bytes(&to, 32));
            must(encoder.u128(10));
            must(encoder.u128(11));
            must(encoder.bytes(&[0; 32], 32));
            must(encoder.bytes(&[0; 32], 32));
            must(encoder.bytes(&[0x33; 32], 32));
            must(encoder.u64(1_750_000_000_250));
            must(encoder.u8(u8::from(signature.is_some())));
            if let Some(signature) = signature {
                must(encoder.bytes(&signature, 64));
            }
            encoder.finish()
        };
        let digest = must(receipt_digest(&encode(None)));
        encode(Some(key.sign(&digest).to_bytes()))
    }

    impl Fixture {
        pub fn new(height: u64, status: bool) -> Self {
            let raw = must(
                Eip1559Call {
                    chain_id: 10,
                    nonce: height,
                    max_priority_fee_per_gas: 1,
                    max_fee_per_gas: 2,
                    gas_limit: 21_000,
                    to: [0x22; 20],
                    data: Vec::new(),
                }
                .sign(&[1; 32]),
            );
            let activity = transaction_hash(&raw);
            let receipt_signing_key = SigningKey::from_bytes(&[7; 32]);
            let receipt_key = ReceiptKeyEntry {
                key_id: [0x33; 32],
                public_key: receipt_signing_key.verifying_key().to_bytes(),
                first_height: 1,
                last_height: 100,
            };
            let signed_receipt = signed_receipt(activity, height, status, &receipt_signing_key);
            let decoded = must(layerx_wire::receipt::decode(&signed_receipt));
            let digest = must(receipt_digest(&must(
                layerx_wire::receipt::encode_unsigned(&decoded),
            )));
            let evm_receipt = [
                &[2][..],
                &rlp(
                    &[
                        rlp(if status { &[1] } else { &[] }, false),
                        rlp(&[0x52, 0x08], false),
                        rlp(&[0; 256], false),
                        rlp(&[], true),
                    ]
                    .concat(),
                    true,
                ),
            ]
            .concat();
            finish_new_3_repair_22_33_1525(FinishNew3Repair22331525Context {
                height,
                raw,
                activity,
                receipt_key,
                signed_receipt,
                digest,
                evm_receipt,
            })
        }

        pub fn state_artifact(&self) -> OfflineExport {
            let mut artifact = self.artifact.clone();
            artifact.facts[0] = must(FactRef::new(format!(
                "state:{}:{}",
                hex::encode(transaction_hash(
                    artifact.bundles[0].raw_transaction.as_bytes()
                )),
                hex::encode(self.account_id)
            )));
            artifact.bundles[0].account_proof = Some(canonical(self.account_proof.clone()));
            artifact
        }
    }

    struct FinishNew3Repair22331525Context {
        height: u64,
        raw: Vec<u8>,
        activity: [u8; 32],
        receipt_key: ReceiptKeyEntry,
        signed_receipt: Vec<u8>,
        digest: [u8; 32],
        evm_receipt: Vec<u8>,
    }

    fn finish_new_3_repair_22_33_1525(context: FinishNew3Repair22331525Context) -> Fixture {
        let FinishNew3Repair22331525Context {
            height,
            raw,
            activity,
            receipt_key,
            signed_receipt,
            digest,
            evm_receipt,
        } = context;

        let leaf = rlp(
            &[rlp(&[0x30], false), rlp(&evm_receipt, false)].concat(),
            true,
        );
        let leaf_hash = transaction_hash(&leaf);
        let mut children = Vec::new();
        for index in 0..17 {
            children.extend(rlp(if index == 8 { &leaf_hash } else { &[] }, false));
        }
        let branch = rlp(&children, true);
        let receipts_root = transaction_hash(&branch);
        let receipt_path = [branch, leaf];
        let inclusion = rlp(
            &[
                rlp(&evm_receipt, false),
                rlp(
                    &receipt_path
                        .iter()
                        .flat_map(|node| rlp(node, false))
                        .collect::<Vec<_>>(),
                    true,
                ),
            ]
            .concat(),
            true,
        );
        let signing_keys: Vec<_> = (1..=3)
            .map(|value| SigningKey::from_bytes(&[value; 32]))
            .collect();
        let validators: Vec<_> = signing_keys
            .iter()
            .map(|key| {
                let public_key = key.verifying_key().to_bytes();
                CommitValidator {
                    address: CommitValidator::address_for_key(&public_key),
                    public_key,
                    voting_power: 3,
                }
            })
            .collect();
        let vectors: serde_json::Value = must(serde_json::from_str(include_str!(
            "../../../../../programs/tests/vectors/ics23-evm-storage-app-hash.json"
        )));
        let vector = &vectors["entries"][0];
        let account: [u8; 20] = must(
            must(hex::decode(require_some(
                vector["address"].as_str(),
                "storage address",
            )))
            .try_into(),
        );
        let slot: [u8; 32] = must(
            must(hex::decode(require_some(
                vector["slot"].as_str(),
                "storage slot",
            )))
            .try_into(),
        );
        let state_root: [u8; 32] = must(
            must(hex::decode(require_some(
                vector["app_hash"].as_str(),
                "storage app hash",
            )))
            .try_into(),
        );
        finish_new_2_repair_22_33_1525(FinishNew2Repair22331525Context {
            height,
            raw,
            activity,
            receipt_key,
            signed_receipt,
            digest,

            receipts_root,

            inclusion,
            signing_keys,
            validators,
            vector,
            account,
            slot,
            state_root,
        })
    }

    struct FinishNew2Repair22331525Context<'a> {
        height: u64,
        raw: Vec<u8>,
        activity: [u8; 32],
        receipt_key: ReceiptKeyEntry,
        signed_receipt: Vec<u8>,
        digest: [u8; 32],

        receipts_root: [u8; 32],

        inclusion: Vec<u8>,
        signing_keys: Vec<SigningKey>,
        validators: Vec<CommitValidator>,
        vector: &'a serde_json::Value,
        account: [u8; 20],
        slot: [u8; 32],
        state_root: [u8; 32],
    }

    fn finish_new_2_repair_22_33_1525(context: FinishNew2Repair22331525Context<'_>) -> Fixture {
        let FinishNew2Repair22331525Context {
            height,
            raw,
            activity,
            receipt_key,
            signed_receipt,
            digest,

            receipts_root,

            inclusion,
            signing_keys,
            validators,
            vector,
            account,
            slot,
            state_root,
        } = context;

        let mut account_id = [0; 32];
        account_id[12..].copy_from_slice(&account);
        let listener = must(std::net::TcpListener::bind("127.0.0.1:0"));
        let endpoint = must(SequencerEndpoint::new(
            &format!("http://{}/rpc", must(listener.local_addr())),
            &[],
        ));
        let response = vector["response"].clone();
        let worker = std::thread::spawn(move || {
            use std::io::{BufReader, Write};
            let (stream, _) = must(listener.accept());
            must(stream.set_read_timeout(Some(std::time::Duration::from_secs(5))));
            let mut reader = BufReader::new(stream);
            let request: serde_json::Value =
                must(serde_json::from_slice(&super::read_http(&mut reader)));
            assert_eq!(request["method"], "eth_getProof");
            assert_eq!(
                request["params"],
                serde_json::json!([
                    format!("0x{}", hex::encode(account)),
                    [format!("0x{}", hex::encode(slot))],
                    format!("{height:#x}")
                ])
            );
            let body = serde_json::json!({
                "jsonrpc":"2.0", "id":request["id"], "result":response
            })
            .to_string();
            let mut stream = reader.into_inner();
            must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()));
            must(stream.flush());
        });
        let witnesses = must(endpoint.storage_witness(&account, &[slot], height));
        must(worker.join());
        for witness in &witnesses {
            must(layerx_client::storage_witness::verify_storage_witness(
                witness, state_root,
            ));
        }
        let account_proof = must(encode_storage_witnesses(&witnesses))
            .as_bytes()
            .to_vec();
        let header = CheckpointHeader {
            version_block: 11,
            version_app: 1,
            chain_id: "paxeer-test".to_owned(),
            height,
            time: CommitTimestamp {
                seconds: 1_700_000_000,
                nanos: 0,
            },
            last_block_id: None,
            last_commit_hash: None,
            data_hash: Some([1; 32]),
            validators_hash: validator_set_hash(&validators),
            next_validators_hash: validator_set_hash(&validators),
            consensus_hash: [2; 32],
            app_hash: state_root,
            last_results_hash: Some(receipts_root),
            evidence_hash: None,
            proposer_address: validators[0].address,
        };
        finish_new_1_repair_22_33_1525(FinishNew1Repair22331525Context {
            height,
            raw,
            activity,
            receipt_key,
            signed_receipt,
            digest,

            inclusion,
            signing_keys,
            validators,
            account_id,
            account_proof,
            header,
        })
    }

    struct FinishNew1Repair22331525Context {
        height: u64,
        raw: Vec<u8>,
        activity: [u8; 32],
        receipt_key: ReceiptKeyEntry,
        signed_receipt: Vec<u8>,
        digest: [u8; 32],

        inclusion: Vec<u8>,
        signing_keys: Vec<SigningKey>,
        validators: Vec<CommitValidator>,
        account_id: [u8; 32],
        account_proof: Vec<u8>,
        header: CheckpointHeader,
    }

    fn finish_new_1_repair_22_33_1525(context: FinishNew1Repair22331525Context) -> Fixture {
        let FinishNew1Repair22331525Context {
            height,
            raw,
            activity,
            receipt_key,
            signed_receipt,
            digest,

            inclusion,
            signing_keys,
            validators,
            account_id,
            account_proof,
            header,
        } = context;

        let mut commit = BlockCommit {
            height,
            round: 0,
            block_id: BlockId {
                hash: header.block_hash(),
                part_set_total: 1,
                part_set_hash: [4; 32],
            },
            votes: Vec::new(),
        };
        let timestamp = CommitTimestamp {
            seconds: 1_700_000_001,
            nanos: 0,
        };
        for (key, validator) in signing_keys.iter().zip(&validators) {
            commit.votes.push(CommitVote {
                validator_address: validator.address,
                voting_power: validator.voting_power,
                timestamp,
                signature: key
                    .sign(&commit.vote_sign_bytes(&header.chain_id, timestamp))
                    .to_bytes(),
            });
        }
        let trust = must(ExportTrustSource::new(
            vec![receipt_key],
            validators.clone(),
        ));
        let artifact = OfflineExport {
            facts: vec![must(FactRef::new(format!(
                "receipt:{}",
                hex::encode(activity)
            )))],
            height,
            bundles: vec![ProofBundle {
                format: "layerx-receipt-proof-v2".to_owned(),
                verification_level: Rung::Committed,
                raw_transaction: canonical(raw),
                signed_receipt: canonical(signed_receipt),
                receipt_digest: digest,
                receipt_key,
                block_header: Some(canonical(encode_header(&header))),
                receipt_inclusion: Some(canonical(inclusion)),
                commit: Some(canonical(encode_commit(&commit, &validators))),
                validator_set: Some(canonical(encode_validators(&validators))),
                account_proof: None,
            }],
        };
        Fixture {
            artifact,
            trust,
            header,

            account_id,
            account_proof,
        }
    }
}

struct IdentityTranscript;
impl identity::IdentityResolver for IdentityTranscript {
    fn resolve(
        &mut self,
        did: &Did,
    ) -> Result<Option<identity::CoreIdentity>, identity::IdentityError> {
        assert_eq!(did, &actor());
        Ok(Some(identity::CoreIdentity {
            canonical_bytes: b"committed-owner-key-3".to_vec(),
            head_sequence: HEIGHT,
            revocation_sequence: 1,
            verification_level: VerificationLevel::COMMITTED,
            frozen: false,
            authorities: vec![ProtocolAuthority::PrimaryKey([3; 32])],
        }))
    }
}
fn install(
    store: &mut Store,
    tenant: &TenantId,
    mut seed: human::HumanAgentLifecycleSeed,
    n: u8,
) -> ManagedAgent {
    let identity = must(identity::register(
        store,
        tenant.clone(),
        actor(),
        &mut IdentityTranscript,
    ));
    let mut sessions = SessionRegistry::default();
    must(session::open(
        store,
        &mut sessions,
        &identity,
        OpenRequest {
            session_id: SessionId([n; 32]),
            token_id: [n + 1; 32],
            tenant: tenant.clone(),
            agent: actor(),
            authority: ProtocolAuthority::PrimaryKey([3; 32]),
            permitted_activity_types: BTreeSet::from([7]),
            scopes: BTreeSet::from(["prepare".into(), "read".into(), "export".into()]),
            expiry_sequence: HEIGHT + 10000,
            expiry_seconds: Some(NOW / 1000 + 10000),
            opening_client: "repair1525".into(),
            policy_version: "v1".into(),
        },
        HEIGHT,
    ));
    seed.protocol_grant_id = [n + 2; 32];
    let agent = must(ManagedAgent::from_creation(
        &seed,
        &seed.actor,
        [n; 32],
        [n + 1; 32],
        1,
    ));
    must(managed_agent::publish_creation(store, tenant, &agent));
    agent
}
fn owner() -> (Directory, TenantId, ManagedAgent) {
    let directory = Directory::new();
    let tenant = must(TenantId::new("repair1525"));
    let mut store = must(Store::open(directory.0.join("store")));
    let agent = install(&mut store, &tenant, seed(), 10);
    (directory, tenant, agent)
}
fn word(n: u128) -> [u8; 32] {
    let mut w = [0; 32];
    w[16..].copy_from_slice(&n.to_be_bytes());
    w
}
fn replacement(current: [u8; 32], action: [u8; 32]) -> [u8; 32] {
    Sha256::new()
        .chain_update(b"layerx-human/agent-limit-budget/v1")
        .chain_update(current)
        .chain_update(action)
        .finalize()
        .into()
}
fn budget_call(agent: &ManagedAgent, id: [u8; 32], amount: u128, nonce: u64) -> Eip1559Call {
    use layerx_agentd::prepare::evm::{AbiArgument, PrecompileCall, PrecompileTarget};
    let account = must(layerx_types::account::AccountId::parse(
        &agent.context.budget_account,
    ));
    let budget_account = must(layerx_wire::hash::account_id_for_protocol(&account, 3));
    let args = vec![
        id,
        budget_account,
        agent.context.budget_asset,
        agent.context.purpose_hash,
        word(amount),
        word(0),
        word(0),
        word(u128::from(agent.context.budget_period_seconds) * 1000),
        word(u128::from(NOW)),
        word(u128::from(NOW + 100_000_000)),
        word(1),
        word(1),
    ];
    let create=PrecompileCall { target:PrecompileTarget::Pay,
        selector:layerx_crypto::disclosure::call::ledger_selector("budgetCreate(bytes32,bytes32,bytes32,bytes32,uint128,uint128,uint128,uint64,uint64,uint64,uint64,uint8)"),
        arguments:args.into_iter().map(AbiArgument::Word).collect() };
    Eip1559Call {
        chain_id: 125,
        nonce,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to: create.target.address(),
        data: must(create.encode()),
    }
}
fn event(
    address: [u8; 20],
    name: &str,
    id: [u8; 32],
    data: Vec<u8>,
) -> layerx_proof::sequencer_logs::SequencerLog {
    layerx_proof::sequencer_logs::SequencerLog::new(
        address,
        vec![transaction_hash(name.as_bytes()), id],
        data,
        0,
    )
}
fn protocol_receipt(
    raw: &[u8],
    logs: &[layerx_proof::sequencer_logs::SequencerLog],
    signature: Option<[u8; 64]>,
) -> Vec<u8> {
    let tx = transaction_hash(raw);
    let height = HEIGHT + must(decode_signed(raw)).0.nonce;
    let (call, sig) = must(decode_signed(raw));
    let sender = must(recover_sender(&call, &sig));
    let mut from = [0; 32];
    from[12..].copy_from_slice(&sender);
    let mut to = [0; 32];
    to[12..].copy_from_slice(&call.to);
    let mut summary = Encoder::new(256);
    must(summary.u64(1));
    must(summary.u64(1));
    must(summary.u64(1));
    must(summary.u32(0));
    must(summary.fixed(&[0; 20]));
    must(summary.fixed(&[0; 20]));
    must(summary.fixed(&hash(b"")));
    must(summary.u32(0));
    must(summary.u32(require_ok(u32::try_from(logs.len()), "test value fits u32")));
    must(summary.u8(4));
    must(summary.fixed(b"upax"));
    let mut e = Encoder::new(16384);
    must(e.structure_header_version(0x5201, 2));
    must(e.u16(2));
    must(e.bytes(&tx, 32));
    must(e.u64(height));
    must(e.bytes(&[2; 32], 32));
    let seq = hash(
        &[
            b"PAXEER/v1/sequence\0".as_slice(),
            &[2; 32],
            &height.to_be_bytes(),
            &tx,
        ]
        .concat(),
    );
    must(e.bytes(&seq, 32));
    must(e.bytes(&hash(b""), 32));
    must(e.i32(0));
    must(e.sequence_length(1 + logs.len(), 512));
    for (index, body) in std::iter::once(summary.finish())
        .chain(logs.iter().map(|log| {
            let mut body = log.address().to_vec();
            body.push(require_ok(
                u8::try_from(log.topics().len()),
                "test value fits u8",
            ));
            for topic in log.topics() {
                body.extend_from_slice(topic);
            }
            body.extend_from_slice(&hash(log.data()));
            body
        }))
        .enumerate()
    {
        must(e.u16(12));
        must(e.u16(require_ok(u16::try_from(index), "test value fits u16")));
        must(e.u16(0));
        must(e.tag(if index == 0 { 1 } else { 3 }, 3));
        must(e.u8(0));
        must(e.bytes(&[0; 32], 32));
        must(e.bytes(&body, 256));
    }
    must(e.u128(1));
    let mut block = [0; 32];
    block[24..].copy_from_slice(&height.to_be_bytes());
    must(e.bytes(&block, 32));
    must(e.u16(12));
    must(e.u32(1));
    must(e.u32(0));
    must(e.u8(2));
    must(e.bytes(&[0; 32], 32));
    must(e.u128(0));
    must(e.bytes(&from, 32));
    must(e.u128(1000));
    must(e.u128(999));
    must(e.u64(call.nonce));
    must(e.bytes(&to, 32));
    must(e.u128(0));
    must(e.u128(0));
    must(e.bytes(&[0; 32], 32));
    must(e.bytes(&[0; 32], 32));
    must(e.bytes(&[11; 32], 32));
    must(e.u64(NOW));
    must(e.u8(u8::from(signature.is_some())));
    if let Some(signature) = signature {
        must(e.bytes(&signature, 64));
    }
    e.finish()
}
fn retain_receipt(
    store: &mut Store,
    tenant: &TenantId,
    call: &Eip1559Call,
    action: [u8; 32],
    logs: &[layerx_proof::sequencer_logs::SequencerLog],
) -> HumanFinalizationEvidence {
    let height = HEIGHT + call.nonce;
    let raw = must(call.sign(&[1; 32]));
    let tx = must(Outbox::default().enqueue(store, tenant.clone(), raw.clone()));
    let signer = SigningKey::from_bytes(&[31; 32]);
    let digest = must(receipt_digest(&protocol_receipt(&raw, logs, None)));
    let canonical = protocol_receipt(&raw, logs, Some(signer.sign(&digest).to_bytes()));
    // Both signed log effects and their committed ABCI result are emitted by the peer transcript.
    let peer = json!({"gasUsed":"0x1","vmError":"","returnData":"0x","txHash":rpc_hex(&tx),"logs":logs.iter().map(|log|
        json!({"address":rpc_hex(&log.address()),"topics":log.topics().iter().map(|t|rpc_hex(t)).collect::<Vec<_>>(),
            "data":rpc_hex(log.data()),"logIndex":format!("{:#x}",log.log_index())})).collect::<Vec<_>>()});
    let results = json!([{"data":openssl::base64::encode_block(&abci_response(&peer)),"gas_wanted":"1","gas_used":"1"}]);
    let inclusion = must(NativeResultSource::from_results(&results, 0));
    let root = layerx_programs::chain_proof::merkle::hash_from_byte_slices(&[&inclusion.result]);
    let checkpoint = committed(
        height + 1,
        require_ok(i64::try_from(NOW), "test value fits i64") / 1000,
        [1; 32],
        root,
        4,
    );
    let key = ReceiptKeyEntry {
        key_id: [11; 32],
        public_key: signer.verifying_key().to_bytes(),
        first_height: height,
        last_height: height,
    };
    must(receipt::persist_committed_protocol_receipt(
        store,
        tenant.clone(),
        &canonical,
        &[key],
        height,
        0,
        &inclusion,
        &checkpoint,
        "125",
        &checkpoint.validators,
    ));
    HumanFinalizationEvidence {
        action_key: action,
        tx_hash: tx,
        receipt_hash: hash(&canonical),
        observed_height: height,
        rung: Rung::Committed.wire_rank(),
        finalized_at: NOW / 1000 + call.nonce,
    }
}
fn limit_request(
    store: &mut Store,
    tenant: &TenantId,
    agent: &ManagedAgent,
    nonce: u64,
    amount: u128,
) -> HumanRequest {
    let action = hash(&nonce.to_be_bytes());
    let budget = replacement(agent.active_budget_id, action);
    let call = budget_call(agent, budget, amount, nonce);
    let data = [agent.context.budget_asset.to_vec(), word(0).to_vec()].concat();
    let log = event(
        call.to,
        "BudgetCreated(bytes32,bytes32,uint128)",
        budget,
        data,
    );
    let evidence = retain_receipt(store, tenant, &call, action, &[log]);
    HumanRequest::AgentLimit {
        agent_id: agent.agent_id.clone(),
        monthly_limit: amount,
        currency: agent.currency.clone(),
        replacement_budget_id: budget,
        evidence,
    }
}
fn response_bytes(response: HumanWireResponse) -> Vec<u8> {
    match response {
        HumanWireResponse::Success(value) => value.bytes().to_vec(),
        other => panic!("expected success, got {other:?}"),
    }
}
fn limit_values(bytes: &[u8]) -> (u8, u128, usize) {
    let mut offset = 0;
    for _ in 0..3 {
        let n = u32::from_be_bytes(require_ok(
            bytes[offset..offset + 4].try_into(),
            "required test value",
        )) as usize;
        offset += 4 + n;
    }
    let state = bytes[offset];
    offset += 1;
    let amount = u128::from_be_bytes(require_ok(
        bytes[offset..offset + 16].try_into(),
        "required test value",
    ));
    offset += 16;
    let n = u32::from_be_bytes(require_ok(
        bytes[offset..offset + 4].try_into(),
        "required test value",
    )) as usize;
    offset += 4 + n;
    offset += 1; // threshold state
    for _ in 0..2 {
        let n = u32::from_be_bytes(require_ok(
            bytes[offset..offset + 4].try_into(),
            "required test value",
        )) as usize;
        offset += 4 + n;
    }
    offset += 16 + 16 + 1; // spent, remaining, authority
    for _ in 0..2 {
        let n = u32::from_be_bytes(require_ok(
            bytes[offset..offset + 4].try_into(),
            "required test value",
        )) as usize;
        offset += 4 + n;
    }
    (state, amount, bytes[offset] as usize)
}

fn listener_config(path: PathBuf, tenant: &str) -> HumanListenerConfig {
    let metadata = must(std::fs::metadata(require_some(
        path.parent(),
        "required test value",
    )));
    HumanListenerConfig {
        endpoint: path,
        owner_uid: metadata.uid(),
        owner_gid: metadata.gid(),
        mode: 0o600,
        maximum_frame_bytes: 8192,
        deadline: Duration::from_millis(700),
        maximum_connections: 4,
        maximum_streams: 4,
        maximum_queued_bytes: 32_768,
        peers: BTreeMap::from([(metadata.uid(), ("recorded-peer".into(), tenant.into()))]),
    }
}
struct Server {
    path: PathBuf,
    stop: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
}
impl Server {
    fn start(directory: &Directory, config: HumanListenerConfig) -> Self {
        let path = config.endpoint.clone();
        let store = must(Store::open(directory.0.join("store")));
        let server = must(HumanUnixServer::bind(config, store));
        let stop = Arc::new(AtomicBool::new(false));
        let running = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            must(server.serve_with(&running, |store, peer, request| {
                let tenant =
                    TenantId::new(peer.tenant.clone()).map_err(|_| HumanOperationError::Refused)?;
                managed_agent::apply_lifecycle_request(store, &tenant, request)
            }));
        });
        Self {
            path,
            stop,
            worker: Some(worker),
        }
    }
    fn exchange(&self, bytes: &[u8]) -> Result<HumanWireResponse, human::frame::TransportError> {
        let mut stream = must(UnixStream::connect(&self.path));
        must(stream.set_read_timeout(Some(Duration::from_secs(2))));
        human::frame::write_frame(&mut stream, bytes, 8192)?;
        let bytes = human::frame::read_frame(&mut stream, 8192)?;
        Ok(must(human::decode_response(bytes)))
    }
    fn stop(mut self) {
        self.finish();
    }
    fn finish(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            must(worker.join());
        }
    }
}
impl Drop for Server {
    fn drop(&mut self) {
        self.finish();
    }
}

fn ready_request(
    directory: &Directory,
    tenant: &TenantId,
    agent: &ManagedAgent,
) -> (HumanRequest, Vec<u8>) {
    let mut store = must(Store::open(directory.0.join("store")));
    let request = limit_request(&mut store, tenant, agent, 1, 200);
    let bytes = must(human::encode_lifecycle_request(&request));
    (request, bytes)
}
fn partial_peer(server: &Server, frame: &[u8]) -> UnixStream {
    let mut peer = must(UnixStream::connect(&server.path));
    must(
        peer.write_all(
            &(require_ok(u32::try_from(frame.len()), "test value fits u32")).to_be_bytes(),
        ),
    );
    must(peer.write_all(&frame[..1]));
    thread::sleep(Duration::from_millis(40));
    peer
}
fn assert_capacity_refused(server: &Server, frame: &[u8]) {
    let before = Instant::now();
    assert!(
        server.exchange(frame).is_err(),
        "excess boundary occupancy was admitted"
    );
    assert!(
        before.elapsed() < Duration::from_millis(500),
        "excess work queued instead of being refused"
    );
}

#[test]
fn human_socket_stop_crash_restart_and_foreign_live_symlink_refusal() {
    let (directory, tenant, agent) = owner();
    let (_, frame) = ready_request(&directory, &tenant, &agent);
    let path = directory.0.join("human.sock");
    let server = Server::start(&directory, listener_config(path.clone(), tenant.as_str()));
    let expected = response_bytes(must(server.exchange(&frame)));
    let live_identity = must(std::fs::symlink_metadata(&path)).ino();
    assert!(HumanUnixServer::bind(listener_config(path.clone(), tenant.as_str()), ()).is_err());
    assert_eq!(must(std::fs::symlink_metadata(&path)).ino(), live_identity);
    assert_eq!(response_bytes(must(server.exchange(&frame))), expected);
    server.stop();
    assert!(!path.exists(), "normal shutdown left its owned socket");
    let server = Server::start(&directory, listener_config(path.clone(), tenant.as_str()));
    assert_eq!(response_bytes(must(server.exchange(&frame))), expected);
    server.stop();

    // A crashed process leaves this exact kernel socket inode without a listening owner.
    let orphan = must(UnixListener::bind(&path));
    must(std::fs::set_permissions(
        &path,
        std::fs::Permissions::from_mode(0o600),
    ));
    drop(orphan);
    assert!(path.exists());
    let server = Server::start(&directory, listener_config(path.clone(), tenant.as_str()));
    assert_eq!(response_bytes(must(server.exchange(&frame))), expected);
    server.stop();
    assert!(!path.exists());

    must(std::os::unix::fs::symlink(
        directory.0.join("missing-target"),
        &path,
    ));
    assert!(HumanUnixServer::bind(listener_config(path.clone(), tenant.as_str()), ()).is_err());
    assert!(must(std::fs::symlink_metadata(&path))
        .file_type()
        .is_symlink());
    must(std::fs::remove_file(&path));
    // Neither an arbitrary file nor a socket outside the configured ownership contract is reclaimed.
    must(std::fs::write(&path, b"foreign-file"));
    assert!(HumanUnixServer::bind(listener_config(path.clone(), tenant.as_str()), ()).is_err());
    assert_eq!(must(std::fs::read(&path)), b"foreign-file");
    must(std::fs::remove_file(&path));
    let foreign = must(UnixListener::bind(&path));
    drop(foreign);
    must(std::fs::set_permissions(
        &path,
        std::fs::Permissions::from_mode(0o600),
    ));
    let mut config = listener_config(path.clone(), tenant.as_str());
    config.owner_uid = require_some(config.owner_uid.checked_add(1), "required test value");
    assert!(HumanUnixServer::bind(config, ()).is_err());
    assert!(path.exists());
    must(std::fs::remove_file(&path));

    let unprotected = directory.0.join("unprotected");
    must(std::fs::create_dir(&unprotected));
    must(std::fs::set_permissions(
        &unprotected,
        std::fs::Permissions::from_mode(0o770),
    ));
    assert!(HumanUnixServer::bind(
        listener_config(unprotected.join("human.sock"), tenant.as_str()),
        ()
    )
    .is_err());

    // Cleanup cannot unlink a replacement pathname owned by a later component.
    let server = must(HumanUnixServer::bind(
        listener_config(path.clone(), tenant.as_str()),
        (),
    ));
    must(std::fs::remove_file(&path));
    must(std::fs::write(&path, b"replacement"));
    drop(server);
    assert_eq!(must(std::fs::read(&path)), b"replacement");
}

#[test]
fn human_whole_frame_deadline_preserves_another_admitted_peer() {
    let (directory, tenant, agent) = owner();
    let (_, frame) = ready_request(&directory, &tenant, &agent);
    let mut config = listener_config(directory.0.join("human.sock"), tenant.as_str());
    config.deadline = Duration::from_millis(300);
    let server = Server::start(&directory, config);
    let began = Instant::now();
    let mut slow = partial_peer(&server, &frame);
    let mut writer = must(slow.try_clone());
    let payload = frame.clone();
    let trickle = thread::spawn(move || {
        for byte in payload.iter().skip(1) {
            thread::sleep(Duration::from_millis(40));
            if writer.write_all(&[*byte]).is_err() {
                return;
            }
        }
    });
    let service = Instant::now();
    assert!(!response_bytes(must(server.exchange(&frame))).is_empty());
    assert!(
        service.elapsed() < Duration::from_millis(200),
        "one partial peer monopolized admission"
    );
    must(slow.set_read_timeout(Some(Duration::from_secs(1))));
    let mut byte = [0; 1];
    assert!(matches!(slow.read(&mut byte), Ok(0) | Err(_)));
    assert!(
        began.elapsed() <= Duration::from_millis(450),
        "progress reset the total request deadline"
    );
    must(trickle.join());
    assert!(!response_bytes(must(server.exchange(&frame))).is_empty());
}

#[test]
fn human_configured_limits_refuse_and_release_capacity() {
    for limit in ["connections", "streams", "queued-bytes"] {
        let (directory, tenant, agent) = owner();
        let (_, frame) = ready_request(&directory, &tenant, &agent);
        let mut config = listener_config(directory.0.join("human.sock"), tenant.as_str());
        let limits = must(human::human_frame_limits(config.deadline, |name| {
            Ok(match name {
                "LAYERX_AGENT_HUMAN_MAX_FRAME_BYTES" => "8192".to_owned(),
                "LAYERX_AGENT_HUMAN_MAX_CONNECTIONS" => {
                    if limit == "connections" { "1" } else { "4" }.to_owned()
                }
                "LAYERX_AGENT_HUMAN_MAX_STREAMS" => {
                    if limit == "streams" { "1" } else { "4" }.to_owned()
                }
                "LAYERX_AGENT_HUMAN_MAX_QUEUED_BYTES" => {
                    if limit == "queued-bytes" {
                        frame.len().to_string()
                    } else {
                        "32768".to_owned()
                    }
                }
                _ => panic!("unknown mandatory limit {name}"),
            })
        }));
        config.maximum_connections = limits.maximum_connections;
        config.maximum_streams = limits.maximum_streams;
        config.maximum_queued_bytes = limits.maximum_queued_bytes;
        let server = Server::start(&directory, config);
        let slow = partial_peer(&server, &frame);
        assert_capacity_refused(&server, &frame);
        drop(slow);
        thread::sleep(Duration::from_millis(40));
        let expected = response_bytes(must(server.exchange(&frame)));
        // Completion releases every slot and the request's byte reservation.
        thread::sleep(Duration::from_millis(10));
        assert_eq!(response_bytes(must(server.exchange(&frame))), expected);
    }
    let (directory, tenant, agent) = owner();
    let (_, frame) = ready_request(&directory, &tenant, &agent);
    let server = Server::start(
        &directory,
        listener_config(directory.0.join("human.sock"), "foreign-tenant"),
    );
    assert!(
        matches!(must(server.exchange(&frame)), HumanWireResponse::Refused(_)),
        "a kernel-authenticated peer crossed its tenant boundary"
    );
}

#[test]
fn managed_limit_binds_owner_agent_amount_budget_and_committed_effect_after_reopen() {
    let (directory, tenant, agent) = owner();
    let mut store = must(Store::open(directory.0.join("store")));
    let mut other_seed = seed();
    other_seed.agent_id = format!("agt_{}", hex::encode([9; 32]));
    other_seed.budget_account = format!(
        "agent:{}:budget:other",
        require_ok(
            std::str::from_utf8(actor().as_bytes()),
            "required test value"
        )
    );
    let other = install(&mut store, &tenant, other_seed, 20);
    let request = limit_request(&mut store, &tenant, &agent, 1, 200);
    let HumanRequest::AgentLimit {
        evidence,
        replacement_budget_id,
        ..
    } = copy_request(&request)
    else {
        panic!("limit");
    };
    // All attacks use a genuine successful same-module committed receipt.
    let attacks = vec![
        HumanRequest::AgentLimit {
            agent_id: other.agent_id.clone(),
            monthly_limit: 200,
            currency: "PAX".into(),
            replacement_budget_id,
            evidence,
        },
        HumanRequest::AgentLimit {
            agent_id: agent.agent_id.clone(),
            monthly_limit: 201,
            currency: "PAX".into(),
            replacement_budget_id,
            evidence,
        },
        HumanRequest::AgentLimit {
            agent_id: agent.agent_id.clone(),
            monthly_limit: 200,
            currency: "PAX".into(),
            replacement_budget_id: [90; 32],
            evidence,
        },
    ];
    for attack in &attacks {
        assert!(
            managed_agent::apply_lifecycle_request(&mut store, &tenant, copy_request(attack))
                .is_err()
        );
    }
    assert_eq!(
        limit_values(must(managed_agent::get(&store, &tenant, &agent.agent_id)).bytes()).1,
        100
    );
    drop(store);
    let mut store = must(Store::open(directory.0.join("store")));
    for attack in &attacks {
        assert!(
            managed_agent::apply_lifecycle_request(&mut store, &tenant, copy_request(attack))
                .is_err()
        );
    }
    let expected = must(managed_agent::apply_lifecycle_request(
        &mut store,
        &tenant,
        copy_request(&request),
    ))
    .bytes()
    .to_vec();
    finish_managed_limit_binds_owner_agent_amount_budget_and_committed_effect_after_reopen_1_repair_22_33_1525(FinishManagedLimitBindsOwnerAgentAmountBudgetAndCommittedEffectAfterReopen1Repair22331525Context { directory, tenant, agent, other, request, evidence, replacement_budget_id, attacks, store, expected });
}

struct FinishManagedLimitBindsOwnerAgentAmountBudgetAndCommittedEffectAfterReopen1Repair22331525Context
{
    directory: Directory,
    tenant: TenantId,
    agent: ManagedAgent,
    other: ManagedAgent,
    request: HumanRequest,
    evidence: HumanFinalizationEvidence,
    replacement_budget_id: [u8; 32],
    attacks: Vec<HumanRequest>,
    store: Store,
    expected: Vec<u8>,
}

fn finish_managed_limit_binds_owner_agent_amount_budget_and_committed_effect_after_reopen_1_repair_22_33_1525(
    context: FinishManagedLimitBindsOwnerAgentAmountBudgetAndCommittedEffectAfterReopen1Repair22331525Context,
) {
    let FinishManagedLimitBindsOwnerAgentAmountBudgetAndCommittedEffectAfterReopen1Repair22331525Context { directory, tenant, agent, other, request, evidence, replacement_budget_id, attacks, mut store, expected } = context;

    assert_eq!(limit_values(&expected).1, 200);
    for attack in &attacks {
        assert!(
            managed_agent::apply_lifecycle_request(&mut store, &tenant, copy_request(attack))
                .is_err()
        );
    }
    drop(store);
    let mut store = must(Store::open(directory.0.join("store")));
    assert_eq!(
        must(managed_agent::apply_lifecycle_request(
            &mut store,
            &tenant,
            copy_request(&request)
        ))
        .bytes(),
        expected
    );
    for attack in &attacks {
        assert!(
            managed_agent::apply_lifecycle_request(&mut store, &tenant, copy_request(attack))
                .is_err()
        );
    }
    assert_eq!(
        must(managed_agent::finalized_evidence(
            &store,
            &tenant,
            &agent.agent_id,
            evidence.receipt_hash
        ))
        .metadata
        .transaction_hash,
        evidence.tx_hash
    );
    assert!(managed_agent::finalized_evidence(
        &store,
        &tenant,
        &other.agent_id,
        evidence.receipt_hash
    )
    .is_err());
    assert_eq!(
        limit_values(must(managed_agent::get(&store, &tenant, &other.agent_id)).bytes()).1,
        100
    );

    // An owner-signed transaction and committed success without its exact event cannot project a limit.
    let action = hash(b"missing-budget-created-event");
    let budget = replacement(replacement_budget_id, action);
    let mut current = agent.clone();
    current.active_budget_id = replacement_budget_id;
    let call = budget_call(&current, budget, 300, 2);
    let unbound = retain_receipt(&mut store, &tenant, &call, action, &[]);
    assert!(managed_agent::finalize_limit(
        &mut store,
        &tenant,
        &agent.agent_id,
        300,
        "PAX",
        budget,
        unbound.into()
    )
    .is_err());
}

fn copy_request(request: &HumanRequest) -> HumanRequest {
    must(human::decode_request(must(
        human::encode_lifecycle_request(request),
    )))
}

#[test]
fn managed_recent_evidence_is_bounded_and_complete_index_survives_reopen() {
    let (directory, tenant, mut agent) = owner();
    let mut store = must(Store::open(directory.0.join("store")));
    let mut history = Vec::new();
    for nonce in 1..=70 {
        let request = limit_request(&mut store, &tenant, &agent, nonce, 200 + u128::from(nonce));
        let HumanRequest::AgentLimit {
            replacement_budget_id,
            evidence,
            ..
        } = copy_request(&request)
        else {
            panic!("limit");
        };
        let result = must(managed_agent::apply_lifecycle_request(
            &mut store,
            &tenant,
            copy_request(&request),
        ));
        let (_, amount, recent) = limit_values(result.bytes());
        assert_eq!(amount, 200 + u128::from(nonce));
        assert!(recent <= 64);
        agent.active_budget_id = replacement_budget_id;
        history.push((request, result.bytes().to_vec(), evidence));
    }
    finish_managed_recent_evidence_is_bounded_and_complete_index_survives_reopen_2_repair_22_33_1525(directory, tenant, agent, store, history);
}

fn finish_managed_recent_evidence_is_bounded_and_complete_index_survives_reopen_2_repair_22_33_1525(
    directory: Directory,
    tenant: TenantId,
    agent: ManagedAgent,
    store: Store,
    history: Vec<(HumanRequest, Vec<u8>, HumanFinalizationEvidence)>,
) {
    assert_eq!(
        limit_values(must(managed_agent::get(&store, &tenant, &agent.agent_id)).bytes()),
        (1, 270, 64)
    );
    drop(store);
    let mut store = must(Store::open(directory.0.join("store")));
    let final_projection = must(managed_agent::get(&store, &tenant, &agent.agent_id))
        .bytes()
        .to_vec();
    for (request, response, evidence) in &history {
        let exported = must(managed_agent::finalized_evidence(
            &store,
            &tenant,
            &agent.agent_id,
            evidence.receipt_hash,
        ));
        assert_eq!(exported.metadata.transaction_hash, evidence.tx_hash);
        assert_eq!(hash(&exported.canonical_receipt), evidence.receipt_hash);
        assert_eq!(
            must(managed_agent::evidence_export(
                &store,
                &tenant,
                &agent.agent_id,
                evidence.receipt_hash
            ))
            .canonical_bytes,
            exported.canonical_receipt
        );
        assert_eq!(
            must(managed_agent::apply_lifecycle_request(
                &mut store,
                &tenant,
                copy_request(request)
            ))
            .bytes(),
            response
        );
        assert_eq!(
            must(managed_agent::get(&store, &tenant, &agent.agent_id)).bytes(),
            final_projection,
            "historical replay applied an old effect again"
        );
    }
    // Subsequent control still uses the real restored session and committed grant revocation.
    let mut sessions = SessionRegistry::default();
    must(sessions.restore_tenant(&store, &tenant));
    let action = hash(b"pause-after-seventy-limit-changes");
    let observation = must(managed_agent::suspend_session(
        &mut store,
        &mut sessions,
        &tenant,
        &agent.agent_id,
        action,
    ));
    let observation = must(managed_agent::SessionObservationV1::decode(
        observation.bytes(),
    ));

    finish_managed_recent_evidence_is_bounded_and_complete_index_survives_reopen_1_repair_22_33_1525(FinishManagedRecentEvidenceIsBoundedAndCompleteIndexSurvivesReopen1Repair22331525Context { directory, tenant, agent, history, store, action, observation });
}

struct FinishManagedRecentEvidenceIsBoundedAndCompleteIndexSurvivesReopen1Repair22331525Context {
    directory: Directory,
    tenant: TenantId,
    agent: ManagedAgent,
    history: Vec<(HumanRequest, Vec<u8>, HumanFinalizationEvidence)>,
    store: Store,
    action: [u8; 32],
    observation: managed_agent::SessionObservationV1,
}

fn finish_managed_recent_evidence_is_bounded_and_complete_index_survives_reopen_1_repair_22_33_1525(
    context: FinishManagedRecentEvidenceIsBoundedAndCompleteIndexSurvivesReopen1Repair22331525Context,
) {
    use layerx_agentd::prepare::evm::{AbiArgument, PrecompileCall, PrecompileTarget};

    let FinishManagedRecentEvidenceIsBoundedAndCompleteIndexSurvivesReopen1Repair22331525Context {
        directory,
        tenant,
        agent,
        history,
        mut store,
        action,
        observation,
    } = context;

    let sequence = HEIGHT + 71;
    let revoke = PrecompileCall {
        target: PrecompileTarget::Auth,
        selector: layerx_crypto::disclosure::call::ledger_selector(
            "revokeGrant(bytes32,uint8,uint64)",
        ),
        arguments: vec![
            AbiArgument::Word(agent.protocol_grant_id),
            AbiArgument::Word(word(1)),
            AbiArgument::Word(word(u128::from(sequence))),
        ],
    };
    let call = Eip1559Call {
        chain_id: 125,
        nonce: 71,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to: revoke.target.address(),
        data: must(revoke.encode()),
    };
    let record = [
        agent.protocol_grant_id.to_vec(),
        vec![1],
        sequence.to_be_bytes().to_vec(),
    ]
    .concat();
    let data = must(
        PrecompileCall {
            target: PrecompileTarget::Auth,
            selector: [0; 4],
            arguments: vec![AbiArgument::Bytes(record)],
        }
        .encode(),
    )[4..]
        .to_vec();
    let log = event(
        call.to,
        "GrantRevoked(bytes32,bytes)",
        agent.protocol_grant_id,
        data,
    );
    let evidence = retain_receipt(&mut store, &tenant, &call, action, &[log]);
    drop(store);
    let mut store = must(Store::open(directory.0.join("store")));
    let response = must(managed_agent::finalize_control(
        &mut store,
        &tenant,
        &agent.agent_id,
        false,
        observation.digest,
        evidence.into(),
    ));
    assert_eq!(limit_values(response.bytes()), (2, 270, 64));
    assert_eq!(
        must(managed_agent::finalized_evidence(
            &store,
            &tenant,
            &agent.agent_id,
            history[0].2.receipt_hash
        ))
        .metadata
        .transaction_hash,
        history[0].2.tx_hash
    );
}

fn read_http(reader: &mut BufReader<TcpStream>) -> Vec<u8> {
    let mut first = String::new();
    must(reader.read_line(&mut first));
    assert!(first.starts_with("POST /rpc ") || first.starts_with("HTTP/1.1 "));
    let mut length = None;
    loop {
        let mut line = String::new();
        assert_ne!(must(reader.read_line(&mut line)), 0);
        if line == "\r\n" {
            break;
        }
        if let Some((name, value)) = line.split_once(':') {
            if name.eq_ignore_ascii_case("content-length") {
                length = Some(require_ok(
                    value.trim().parse::<usize>(),
                    "required test value",
                ));
            }
        }
    }
    let length = require_some(length, "required test value");
    assert!(length <= 1_048_576);
    let mut body = vec![0; length];
    must(reader.read_exact(&mut body));
    body
}

#[test]
fn offline_export_producer_rpc_consumer_and_mismatched_fact_refusal() {
    use layerx_agentd::export::{self};
    use layerx_sdk::agent_envelope::{encode_envelope, EnvelopeCredential};
    let fixture = portable_fixture::Fixture::new(8, true);
    let directory = Directory::new();
    let tenant = must(TenantId::new("rpc-export"));
    let mut store = must(Store::open(directory.0.join("store")));
    must(export::retain_bundle(
        &mut store,
        tenant.clone(),
        fixture.artifact.clone(),
        &fixture.trust,
    ));
    let state = fixture.state_artifact();
    must(export::retain_bundle(
        &mut store,
        tenant.clone(),
        state.clone(),
        &fixture.trust,
    ));
    // Deliberately request the state fact first; transport and producer must preserve order.
    let facts = vec![state.facts[0].clone(), fixture.artifact.facts[0].clone()];
    let request = ReadRequest {
        selector: facts.clone(),
        requested_rung: Rung::Receipted,
    };
    let credential = must(EnvelopeCredential::new(
        tenant.as_str(),
        [10; 32],
        [11; 32],
        1,
    ));
    let body=must(encode_envelope(layerx_sdk::Operation::ExportOffline,RequestId(7),&json!({
        "fact_set":facts.iter().map(FactRef::as_str).collect::<Vec<_>>(),"requested_rung":"receipted"
    }),Some(&credential),None)).to_string().into_bytes();
    let pinned = fixture.trust.clone();
    let bound = fixture.account_id;
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let address = must(listener.local_addr());
    let worker = thread::spawn(move || {
        let store = Mutex::new(store);
        let (stream, _) = must(listener.accept());
        must(stream.set_read_timeout(Some(Duration::from_secs(3))));
        let mut reader = BufReader::new(stream);
        let body = read_http(&mut reader);
        let envelope = must(agent_rpc::decode(&body));
        assert_eq!(
            envelope.operation,
            layerx_agentd::tenant::Operation::ExportOffline
        );
        assert_eq!(
            require_some(envelope.credential.as_ref(), "required test value").tenant(),
            &tenant
        );
        let dispatched = must(agent_rpc_adapters::export_offline_with_owner(
            envelope.request_id,
            &envelope.request,
            |typed| {
                assert_eq!(typed.requested_rung, Rung::Receipted);
                assert_eq!(typed.selector, facts);
                let produced = export::produce_retained(
                    &store,
                    &tenant,
                    Some(bound),
                    &pinned,
                    &typed.selector,
                )
                .map_err(|_| HumanOperationError::Refused)?;
                Ok(produced.response)
            },
        ));
        let response = must(agent_rpc::success_body(
            envelope.request_id,
            &dispatched,
            Some(Some(Rung::Receipted)),
        ));
        let socket = reader.get_mut();
        must(write!(socket,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",response.body.len()));
        must(socket.write_all(&response.body));
    });
    finish_offline_export_producer_rpc_consumer_and_mismatched_fact_refusal_1_repair_22_33_1525(
        &fixture, &request, &body, address, worker,
    );
}

fn finish_offline_export_producer_rpc_consumer_and_mismatched_fact_refusal_1_repair_22_33_1525(
    fixture: &portable_fixture::Fixture,
    request: &ReadRequest<Vec<FactRef>>,
    body: &[u8],
    address: std::net::SocketAddr,
    worker: thread::JoinHandle<()>,
) {
    use layerx_agent_api::verify::VerificationStatus;
    use layerx_agentd::export::{self, ExportTrustSource};
    use layerx_sdk::agent_envelope::decode_response;

    let mut client = must(TcpStream::connect(address));
    must(client.set_read_timeout(Some(Duration::from_secs(3))));
    must(write!(client,"POST /rpc HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",body.len()));
    must(client.write_all(body));
    let response = read_http(&mut BufReader::new(client));
    must(worker.join());
    let document: Value = must(serde_json::from_slice(&response));
    let success = must(require_some(
        decode_response(200, &document),
        "required test value",
    ));
    assert_eq!(
        success.verification_status,
        VerificationStatus::Achieved(Rung::Committed)
    );
    let offline = must(layerx_sdk::rpc_export::decode_offline_export(
        &success.value["value"],
        layerx_sdk::Operation::ExportOffline,
    ));
    assert_eq!(offline.facts, request.selector);
    let independent = must(ExportTrustSource::new(
        fixture.trust.receipt_keys().to_vec(),
        fixture.trust.validator_set().to_vec(),
    ));
    let report = must(export::verify(&offline, &independent));
    assert_eq!(report.verified_receipts, 2);
    assert_eq!(report.verified_inclusions, 2);
    assert_eq!(report.height, 8);

    let produced = layerx_agent_api::read::VerifiedRead::new(
        offline.clone(),
        Rung::Committed,
        layerx_agent_api::read::Freshness {
            chain_head: 8,
            value_height: 8,
            relative_to: layerx_agent_api::read::RelativeTo::Block {
                height: 8,
                hash: fixture.header.block_hash(),
            },
        },
    );
    let mut mismatch = request.clone();
    mismatch.selector.reverse();
    assert!(
        agent_rpc_adapters::offline_export_operation(RequestId(8), &mismatch, &produced).is_err()
    );
    let mut mismatch = produced.clone();
    mismatch.value.bundles.pop();
    assert!(
        agent_rpc_adapters::offline_export_operation(RequestId(8), request, &mismatch).is_err()
    );
    let mut mismatch = produced.clone();
    mismatch.value.height = 9;
    assert!(
        agent_rpc_adapters::offline_export_operation(RequestId(8), request, &mismatch).is_err()
    );
}

#[test]
fn startup_environment_accepts_height_node_and_refuses_retired_controls() {
    let supported = BTreeMap::from([
        ("LAYERX_AGENT_MCP_CORE_HEIGHT", "8"),
        ("LAYERX_AGENT_MCP_EXPIRY_HEIGHT", "100"),
        ("LAYERX_AGENT_NODE_RPC_URL", "http://127.0.0.1:8545"),
        ("LAYERX_AGENT_NODE_CHAIN_ID", "125"),
        ("LAYERX_AGENT_HUMAN_MAX_FRAME_BYTES", "8192"),
        ("LAYERX_AGENT_HUMAN_MAX_CONNECTIONS", "2"),
        ("LAYERX_AGENT_HUMAN_MAX_STREAMS", "1"),
        ("LAYERX_AGENT_HUMAN_MAX_QUEUED_BYTES", "8192"),
    ]);
    must(human::refuse_retired_human_inputs(
        supported.keys().copied(),
    ));
    let limits = must(human::human_frame_limits(
        Duration::from_millis(300),
        |name| {
            supported
                .get(name)
                .map(std::string::ToString::to_string)
                .ok_or_else(|| format!("{name} is required"))
        },
    ));
    assert_eq!(limits.maximum_frame_bytes, 8192);
    assert_eq!(limits.maximum_connections, 2);
    assert_eq!(limits.maximum_streams, 1);
    assert_eq!(limits.maximum_queued_bytes, 8192);
    for name in [
        "LAYERX_AGENT_MCP_CORE_SEQUENCE",
        "LAYERX_AGENT_MCP_EXPIRY_SEQUENCE",
        "LAYERX_AGENT_HUMAN_RECONNECT_ATTEMPTS",
        "LAYERX_AGENT_HUMAN_RECONNECT_BASE_MS",
        "LAYERX_AGENT_HUMAN_RECONNECT_MAX_MS",
        "LAYERX_AGENT_HUMAN_RECONNECT_JITTER_PERCENT",
        "LAYERX_AGENT_HUMAN_EXPORT_SET_VERSION",
    ] {
        assert_eq!(
            human::refuse_retired_human_inputs([name]),
            Err(format!("{name} is retired"))
        );
    }
    for setting in [
        "LAYERX_AGENT_HUMAN_MAX_CONNECTIONS",
        "LAYERX_AGENT_HUMAN_MAX_STREAMS",
        "LAYERX_AGENT_HUMAN_MAX_QUEUED_BYTES",
    ] {
        assert!(
            human::human_frame_limits(Duration::from_millis(300), |name| Ok(if name == setting {
                "0".to_owned()
            } else {
                supported[name].to_owned()
            }))
            .is_err()
        );
    }
    // Exercise the shared production environment readers without relying on external settings.
    let keys = [
        "LAYERX_AGENT_MCP_CORE_HEIGHT",
        "LAYERX_AGENT_MCP_EXPIRY_HEIGHT",
        "LAYERX_AGENT_NODE_RPC_URL",
        "LAYERX_AGENT_NODE_CHAIN_ID",
        "LAYERX_AGENT_HUMAN_PROTOCOL_VERSION",
        "LAYERX_AGENT_HUMAN_NETWORK_ID",
    ];
    let old = keys.map(|name| (name, std::env::var_os(name)));
    for name in keys.iter().take(4) {
        std::env::set_var(name, supported[name]);
    }
    std::env::set_var(
        "LAYERX_AGENT_HUMAN_PROTOCOL_VERSION",
        layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION.to_string(),
    );
    std::env::set_var("LAYERX_AGENT_HUMAN_NETWORK_ID", "125");
    let heights = must(layerx_agentd::node_rpc::environment::McpHeights::from_environment());
    assert_eq!((heights.core_height, heights.expiry_height), (8, 100));
    let node = must(layerx_agentd::node_rpc::environment::HumanNodeEnvironment::from_environment());
    assert_eq!(node.rpc.url, supported["LAYERX_AGENT_NODE_RPC_URL"]);
    assert_eq!(node.rpc.chain_id, 125);
    std::env::set_var("LAYERX_AGENT_NODE_CHAIN_ID", "126");
    assert!(
        layerx_agentd::node_rpc::environment::HumanNodeEnvironment::from_environment().is_err()
    );
    for (name, value) in old {
        match value {
            Some(value) => std::env::set_var(name, value),
            None => std::env::remove_var(name),
        }
    }
}
