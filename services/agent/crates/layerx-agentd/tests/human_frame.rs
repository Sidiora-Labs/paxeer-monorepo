use layerx_agentd::human::frame::{
    read_frame, write_frame, FrameViolation, TransportError, LENGTH_PREFIX_BYTES,
};
use layerx_agentd::human::{serve_one, AcceptedTransport, HumanPeer};
use layerx_agentd::human_runtime::{ProductionHumanOperations, RemoteHumanAuthority};
use layerx_agentd::store::Store;
use layerx_client::client::{ClientConfig, ReconnectPolicy};
use layerx_client::lni::handshake::HandshakeConfig;
use layerx_client::lni::schema::Version;
use layerx_client::lni::transport::Limits;
use layerx_client::runtime_clock::RuntimeClock;
use layerx_client::Client;
use serde_json::Value;
use std::collections::BTreeMap;
use std::fmt::Debug;
use std::fs;
use std::io::{Cursor, Read, Write};
use std::os::unix::fs::PermissionsExt;
use std::os::unix::net::UnixStream;
use std::path::Path;
use std::sync::{Arc, Mutex};
use std::time::Duration;

fn checked<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

#[test]
fn human_frame_round_trips_with_a_length_prefix() {
    const MAXIMUM: usize = 1_048_576;
    assert_eq!(LENGTH_PREFIX_BYTES, 4);
    for payload in [Vec::new(), b"LXHAGT01\x07".to_vec(), vec![0xa5; MAXIMUM]] {
        let mut wire = Vec::new();
        checked(write_frame(&mut wire, &payload, MAXIMUM));
        assert_eq!(
            &wire[..4],
            &checked(u32::try_from(payload.len())).to_be_bytes()
        );
        assert_eq!(&wire[4..], payload);
        assert_eq!(checked(read_frame(&mut wire.as_slice(), MAXIMUM)), payload);
    }

    let oversized = vec![0; MAXIMUM + 1];
    let error = TransportError::Frame(FrameViolation::Oversized {
        declared: checked(u32::try_from(MAXIMUM + 1)),
        maximum: MAXIMUM,
    });
    let mut wire = Vec::new();
    assert_eq!(write_frame(&mut wire, &oversized, MAXIMUM), Err(error));
    assert!(wire.is_empty());
    wire.extend_from_slice(&checked(u32::try_from(MAXIMUM + 1)).to_be_bytes());
    wire.extend_from_slice(&oversized);
    let mut reader = Cursor::new(wire);
    assert_eq!(read_frame(&mut reader, MAXIMUM), Err(error));
    assert_eq!(reader.position(), 4);
    assert_eq!(
        read_frame(&mut u32::MAX.to_be_bytes().as_slice(), MAXIMUM),
        Err(TransportError::Frame(FrameViolation::Oversized {
            declared: u32::MAX,
            maximum: MAXIMUM,
        }))
    );

    let mut wire = Vec::new();
    checked(write_frame(&mut wire, b"first", MAXIMUM));
    checked(write_frame(&mut wire, b"second", MAXIMUM));
    let mut reader = wire.as_slice();
    assert_eq!(checked(read_frame(&mut reader, MAXIMUM)), b"first");
    assert_eq!(checked(read_frame(&mut reader, MAXIMUM)), b"second");
    assert!(reader.is_empty());
    assert_eq!(
        read_frame(&mut reader, MAXIMUM),
        Err(TransportError::PeerShutdown)
    );
    assert_eq!(
        read_frame(&mut [0, 0, 0].as_slice(), MAXIMUM),
        Err(TransportError::Frame(FrameViolation::TruncatedPrefix))
    );
    assert_eq!(
        read_frame(&mut [0, 0, 0, 2, 1].as_slice(), MAXIMUM),
        Err(TransportError::Frame(FrameViolation::TruncatedBody))
    );
}

fn protected(path: &Path, maximum: u64) -> Vec<u8> {
    let metadata = checked(fs::symlink_metadata(path));
    assert!(metadata.is_file());
    assert!(metadata.len() > 0 && metadata.len() <= maximum);
    assert_eq!(metadata.permissions().mode() & 0o077, 0);
    checked(fs::read(path))
}

fn text<'a>(fixture: &'a Value, key: &str) -> &'a str {
    fixture[key]
        .as_str()
        .unwrap_or_else(|| panic!("genuine fixture field {key} is required"))
}

#[test]
fn human_peer_still_answers_lxhagt01_frames() {
    let fixture_path = std::env::var_os("PAXEER_X_NATIVE_PROGRAM_HUMAN_PEER_FIXTURE")
        .unwrap_or_else(|| panic!("genuine native_program_human_peer fixture is required"));
    let fixture: Value = checked(serde_json::from_slice(&protected(
        Path::new(&fixture_path),
        65_536,
    )));
    assert_eq!(fixture["schema"], "paxeer-x.native-program-human-peer.v1");
    let deadline = Duration::from_secs(8);
    let maximum = 1_048_576;
    let node = checked(Client::connect(ClientConfig {
        endpoint: Path::new(text(&fixture, "lni_socket")).to_path_buf(),
        handshake: HandshakeConfig {
            built_interface_version: Version::V1_8,
            expected_protocol_version: 3,
            expected_network_id: checked(u32::try_from(
                fixture["network_id"]
                    .as_u64()
                    .unwrap_or_else(|| panic!("native network is required")),
            )),
        },
        limits: Limits {
            maximum_frame_bytes: 1_212_416,
            maximum_connections: 1,
            maximum_streams: 1,
            maximum_queued_bytes: 1_212_416,
            deadline,
        },
        reconnect: ReconnectPolicy {
            maximum_attempts: 1,
            base_delay: Duration::from_millis(1),
            maximum_delay: Duration::from_millis(1),
            jitter_percent: 0,
        },
    }));
    let head = node.head();
    let mut expected = b"LXHAGT01\x00".to_vec();
    expected.extend_from_slice(&head.chain_sequence.to_be_bytes());
    expected.extend_from_slice(&head.sealed_batch.to_be_bytes());
    expected.extend_from_slice(&head.finalised_checkpoint);
    let bearer = checked(String::from_utf8(protected(
        Path::new(text(&fixture, "bearer_file")),
        4096,
    )));
    let authority = checked(RemoteHumanAuthority::connect(
        text(&fixture, "authority_endpoint"),
        bearer.trim().to_owned(),
        deadline,
        maximum,
        &protected(Path::new(text(&fixture, "ca_der_file")), 65_536),
    ));
    let clock = checked(RuntimeClock::from_environment());
    let mut random = [0; 16];
    checked(getrandom::fill(&mut random));
    let root = std::env::temp_dir().join(format!(
        "human-frame-{}-{}",
        std::process::id(),
        hex::encode(random)
    ));
    let store = Arc::new(Mutex::new(checked(Store::open(&root))));
    let peer = HumanPeer {
        uid: checked(u32::try_from(
            fixture["uid"]
                .as_u64()
                .unwrap_or_else(|| panic!("native uid is required")),
        )),
        principal: text(&fixture, "principal").to_owned(),
        tenant: text(&fixture, "tenant").to_owned(),
        subject: None,
    };
    let peers = BTreeMap::from([(peer.uid, (peer.principal.clone(), peer.tenant.clone()))]);
    let mut operations = checked(ProductionHumanOperations::new(
        authority, node, store, &peers, maximum, 30, clock,
    ));
    let (mut client, server) = checked(UnixStream::pair());
    checked(client.set_read_timeout(Some(deadline)));
    checked(client.set_write_timeout(Some(deadline)));
    let mut transport = checked(AcceptedTransport::new(server, maximum, deadline));

    for trace in [None, Some("trc_0123456789abcdef0123456789abcdef")] {
        let request = b"LXHAGT01\x07";
        let (request, response) = if let Some(trace) = trace {
            let envelope = |payload: &[u8]| {
                let mut out = b"LXHAGT02".to_vec();
                out.extend_from_slice(trace.as_bytes());
                out.extend_from_slice(&checked(u32::try_from(payload.len())).to_be_bytes());
                out.extend_from_slice(payload);
                out
            };
            (envelope(request), envelope(&expected))
        } else {
            (request.to_vec(), expected.clone())
        };
        checked(client.write_all(&checked(u32::try_from(request.len())).to_be_bytes()));
        checked(client.write_all(&request));
        checked(serve_one(&mut transport, &peer, &mut operations));
        let mut prefix = [0; 4];
        checked(client.read_exact(&mut prefix));
        assert_eq!(
            u32::from_be_bytes(prefix),
            checked(u32::try_from(response.len()))
        );
        let mut payload = vec![0; response.len()];
        checked(client.read_exact(&mut payload));
        assert_eq!(payload, response);
    }
    drop(operations);
    checked(fs::remove_dir_all(root));
}
