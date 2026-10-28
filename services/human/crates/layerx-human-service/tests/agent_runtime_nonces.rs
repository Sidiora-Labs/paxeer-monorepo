use std::fmt::Debug;
use std::os::unix::net::UnixListener;
use std::path::PathBuf;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_agent_api::identity::{AgentDid, AuthorityRef};
use layerx_human_service::frame::{read_frame, write_frame, Limits};
use layerx_human_service::journeys::AgentBoundaryError;
use layerx_human_service::server::agent_runtime::AgentRuntime;
use layerx_human_service::server::native_send::HumanOwnerNativeSessionV1;
use layerx_proof::state::{AccountAsset, CanonicalAccount};
use layerx_types::payload::ModuleRegistry;
use layerx_types::rung::Rung;
use sha2::{Digest as _, Sha256};

const MAX_FRAME: usize = 4096;
const OWNER: &str = "did:layerx:nonce-owner";

fn checked<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("agent runtime nonce peer: {error:?}"))
}

struct Peer {
    directory: PathBuf,
    server: Option<JoinHandle<()>>,
}

impl Peer {
    fn serve(exchanges: Vec<(Vec<u8>, Vec<u8>)>) -> (Self, AgentRuntime) {
        let mut entropy = [0; 16];
        checked(getrandom::fill(&mut entropy));
        let directory = std::env::temp_dir().join(format!(
            "human-nonces-{:032x}",
            u128::from_be_bytes(entropy)
        ));
        checked(std::fs::create_dir(&directory));
        let socket = directory.join("peer.sock");
        let listener = checked(UnixListener::bind(&socket));
        checked(listener.set_nonblocking(true));
        let server = thread::spawn(move || {
            for (expected, response) in exchanges {
                let deadline = Instant::now() + Duration::from_secs(5);
                let mut stream = loop {
                    match listener.accept() {
                        Ok((stream, _)) => break stream,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            assert!(Instant::now() < deadline, "peer request deadline");
                            thread::sleep(Duration::from_millis(1));
                        }
                        Err(error) => panic!("peer accept: {error}"),
                    }
                };
                checked(stream.set_read_timeout(Some(Duration::from_secs(5))));
                checked(stream.set_write_timeout(Some(Duration::from_secs(5))));
                assert_eq!(checked(read_frame(&mut stream, MAX_FRAME)), expected);
                checked(write_frame(&mut stream, &response, MAX_FRAME));
            }
        });
        let runtime = checked(AgentRuntime::new(
            &socket,
            Limits {
                maximum_frame_bytes: MAX_FRAME,
                maximum_connections: 1,
                maximum_streams: 1,
                maximum_queued_bytes: MAX_FRAME,
                deadline: Duration::from_secs(5),
            },
            checked(ModuleRegistry::new(&[])),
        ));
        (
            Self {
                directory,
                server: Some(server),
            },
            runtime,
        )
    }

    fn finish(mut self) {
        if let Some(server) = self.server.take() {
            checked(server.join());
        }
    }
}

impl Drop for Peer {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.directory);
    }
}

fn bytes(target: &mut Vec<u8>, value: &[u8]) {
    target.extend_from_slice(&checked(u32::try_from(value.len())).to_be_bytes());
    target.extend_from_slice(value);
}

fn frame(opcode: u8, body: &[u8]) -> Vec<u8> {
    let mut frame = b"LXHAGT01".to_vec();
    frame.push(opcode);
    frame.extend_from_slice(body);
    frame
}

fn account_value() -> (CanonicalAccount, Vec<u8>) {
    let name = format!("agent:{OWNER}:main").into_bytes();
    let mut identity = b"LX:ACCOUNT:v1".to_vec();
    bytes(&mut identity, &name);
    let account = CanonicalAccount {
        account_id: Sha256::digest(identity).into(),
        name,
        kind: 1,
        asset: Some(AccountAsset {
            asset_id: [0x33; 32],
            balance: 5_000,
        }),
        next_sequence: 19,
        created_at_sequence: 1,
        frozen: false,
        has_open_reference: false,
        authority_key: Some([0x44; 32]),
    };
    let mut value = checked(u16::try_from(account.name.len()))
        .to_be_bytes()
        .to_vec();
    value.extend_from_slice(&account.name);
    value.push(account.kind);
    value.extend_from_slice(&account.balance().to_be_bytes());
    value.extend_from_slice(&account.asset_id());
    value.push(1);
    value.extend_from_slice(&account.next_sequence.to_be_bytes());
    value.extend_from_slice(&account.created_at_sequence.to_be_bytes());
    value.extend_from_slice(&[0, 0]);
    value.extend_from_slice(&[0x44; 32]);
    value.push(1);
    (account, value)
}

fn account_answer(account_id: [u8; 32], rank: u8, value: &[u8], height: u64) -> Vec<u8> {
    let mut body = account_id.to_vec();
    body.push(rank);
    bytes(&mut body, value);
    bytes(&mut body, &[0x55; 32]);
    body.extend_from_slice(&height.to_be_bytes());
    frame(0, &body)
}

#[test]
fn account_state_is_verified_only_at_committed() {
    let (account, value) = account_value();
    let request = frame(42, &account.account_id);
    let mut exchanges: Vec<_> = (0..=u8::MAX)
        .map(|rank| {
            (
                request.clone(),
                account_answer(account.account_id, rank, &value, 88),
            )
        })
        .collect();
    let committed = Rung::Committed.wire_rank();
    let mut truncated = account_answer(account.account_id, committed, &value, 88);
    truncated.pop();
    let mut trailing = account_answer(account.account_id, committed, &value, 88);
    trailing.push(0);
    let mut corrupt_value = value.clone();
    corrupt_value.push(0);
    let invalid_answers = [
        account_answer(account.account_id, committed, &value, 0),
        account_answer([0x66; 32], committed, &value, 88),
        account_answer(account.account_id, committed, &corrupt_value, 88),
        truncated,
        trailing,
    ];
    exchanges.extend(
        invalid_answers
            .iter()
            .map(|answer| (request.clone(), answer.clone())),
    );
    let (peer, mut runtime) = Peer::serve(exchanges);
    for rank in 0..=u8::MAX {
        let result = runtime.account_state(account.account_id);
        if rank == committed {
            assert_eq!(checked(result), account);
        } else {
            assert_eq!(
                result,
                Err(AgentBoundaryError::CorruptResponse),
                "rank {rank}"
            );
        }
    }
    for _ in invalid_answers {
        assert_eq!(
            runtime.account_state(account.account_id),
            Err(AgentBoundaryError::CorruptResponse)
        );
    }
    peer.finish();
}

#[test]
fn native_send_owner_context_carries_the_pending_nonce() {
    let session: HumanOwnerNativeSessionV1 = checked(serde_json::from_value(serde_json::json!({
        "version": 1,
        "tenant": "nonce-tenant",
        "principal": "nonce-principal",
        "owner": OWNER,
        "owner_public_key": vec![0x11; 32],
        "session_id": vec![0x22; 32],
        "generation": 3,
        "grant_id": vec![0x33; 32],
        "grant_receipt_digest": vec![0x44; 32],
        "grant_activity_id": vec![0x55; 32],
        "grant_sequence": 40,
        "expires_at_ms": 200_000,
        "token": vec![0x66; 32]
    })));
    let authority_text = format!("session:{}", "33".repeat(32));
    let actor = checked(AgentDid::new(OWNER));
    let authority = checked(AuthorityRef::new(&authority_text));
    let mut nonce_request = Vec::new();
    bytes(&mut nonce_request, OWNER.as_bytes());
    bytes(&mut nonce_request, authority_text.as_bytes());
    let nonce_request = frame(13, &nonce_request);
    let mut owner_request = Vec::new();
    bytes(&mut owner_request, session.tenant.as_bytes());
    owner_request.extend_from_slice(&session.session_id);
    owner_request.extend_from_slice(&[0x66; 32]);
    owner_request.extend_from_slice(&session.generation.to_be_bytes());
    owner_request.extend_from_slice(&session.owner_public_key);
    owner_request.extend_from_slice(&9_u64.to_be_bytes());
    let owner_request = frame(64, &owner_request);
    let nonces = [0_u64, 1, 7, u64::MAX];
    let mut exchanges = Vec::new();
    for pending_nonce in nonces {
        let mut nonce_answer = pending_nonce.to_be_bytes().to_vec();
        let rung = match pending_nonce {
            0 => Rung::Submitted,
            1 => Rung::Committed,
            _ => Rung::Receipted,
        };
        nonce_answer.push(rung.wire_rank());
        exchanges.push((nonce_request.clone(), frame(0, &nonce_answer)));
        let mut body = Vec::new();
        bytes(&mut body, OWNER.as_bytes());
        body.push(2);
        body.extend_from_slice(&session.grant_id);
        for coordinate in [session.generation, 88, 100_000, pending_nonce, 120, 200_000] {
            body.extend_from_slice(&coordinate.to_be_bytes());
        }
        body.extend_from_slice(&session.owner_public_key);
        body.extend_from_slice(&80_u64.to_be_bytes());
        body.extend_from_slice(&[0x77; 32]);
        exchanges.push((owner_request.clone(), frame(0, &body)));
    }
    let mut malformed = vec![7_u64.to_be_bytes().to_vec()];
    let mut truncated = 7_u64.to_be_bytes().to_vec();
    truncated.pop();
    malformed.push(truncated);
    for rank in 3..=u8::MAX {
        let mut body = 7_u64.to_be_bytes().to_vec();
        body.push(rank);
        malformed.push(body);
    }
    let mut trailing = 7_u64.to_be_bytes().to_vec();
    trailing.extend_from_slice(&[Rung::Committed.wire_rank(), 0]);
    malformed.push(trailing);
    exchanges.extend(
        malformed
            .iter()
            .map(|body| (nonce_request.clone(), frame(0, body))),
    );
    let (peer, mut runtime) = Peer::serve(exchanges);
    for pending_nonce in nonces {
        assert_eq!(
            checked(runtime.account_sequence(&actor, &authority)),
            pending_nonce
        );
        let coordinates = checked(runtime.native_send_owner_context(&session, 9));
        assert_eq!(coordinates.account_sequence, pending_nonce);
        assert_eq!(coordinates.actor, OWNER);
        assert_eq!(coordinates.authority, authority_text);
        assert_eq!(coordinates.generation, session.generation);
        assert_eq!(coordinates.head_sequence, 88);
        assert_eq!(coordinates.protocol_time_ms, 100_000);
        assert_eq!(coordinates.expiry_sequence, 120);
        assert_eq!(coordinates.expiry_ms, 200_000);
        assert_eq!(coordinates.owner_public_key, session.owner_public_key);
        assert_eq!(coordinates.revocation_sequence, 80);
        assert_eq!(coordinates.native_fee_asset, [0x77; 32]);
    }
    for _ in malformed {
        assert_eq!(
            runtime.pending_nonce(&actor, &authority),
            Err(AgentBoundaryError::CorruptResponse)
        );
    }
    peer.finish();
}
