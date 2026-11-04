#[path = "support/repair_22_33_1529.rs"]
mod repair_support;
mod support;

use std::fs;

use layerx_agentd::admin::{OperatorCommand, ProtectedMutation, Surface};
use layerx_agentd::audit::verify_chain;
use layerx_agentd::human::HumanOperationError;
use layerx_agentd::human_runtime::route_operator_command;
use layerx_agentd::outbox::{Outbox, SubmissionState};
use layerx_agentd::store::Store;

use support::{directory, signed_transaction, tenant};

const OPERATOR: &str = "operator:incident-response";

fn enqueue_unknown(store: &mut Store, outbox: &mut Outbox, id: u8) -> [u8; 32] {
    let hash = outbox
        .enqueue(store, tenant(), signed_transaction(id))
        .unwrap_or_else(|error| panic!("enqueue: {error:?}"));
    outbox
        .transition(
            store,
            hash,
            SubmissionState::Submitted,
            "real boundary accepted bytes",
            None,
        )
        .unwrap_or_else(|error| panic!("submitted: {error:?}"));
    outbox
        .transition(
            store,
            hash,
            SubmissionState::Unknown,
            "acknowledgement was not observed",
            None,
        )
        .unwrap_or_else(|error| panic!("unknown: {error:?}"));
    hash
}

#[test]
fn human_socket_operator_commands_reach_the_audited_admin_dispatch() {
    let root = directory("operator-route");
    let mut store = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    let mut outbox = Outbox::default();
    let hash = enqueue_unknown(&mut store, &mut outbox, 1);
    let activity_id = outbox
        .status(hash)
        .unwrap_or_else(|| panic!("unknown missing"))
        .activity_id;

    let inspected = route_operator_command(
        &root,
        &tenant(),
        &outbox,
        OPERATOR,
        [7; 32],
        OperatorCommand::InspectUnknown(hash),
    )
    .unwrap_or_else(|error| panic!("inspect unknown: {error:?}"));
    let bytes = inspected.bytes();
    assert_eq!(bytes.len(), 74);
    assert_eq!(bytes[0], 1);
    assert_eq!(&bytes[1..33], &activity_id);
    assert_eq!(&bytes[33..65], &hash);
    assert_eq!(bytes[65], 5);
    assert_eq!(&bytes[66..74], &1_u64.to_be_bytes());

    assert_eq!(
        route_operator_command(
            &root,
            &tenant(),
            &outbox,
            OPERATOR,
            [8; 32],
            OperatorCommand::InspectUnknown([2; 32]),
        )
        .err(),
        Some(HumanOperationError::Refused)
    );
    assert_eq!(
        route_operator_command(
            &root,
            &tenant(),
            &outbox,
            OPERATOR,
            [9; 32],
            OperatorCommand::AttemptProtectedMutation {
                target: hash,
                mutation: ProtectedMutation::MarkUnknownExecuted,
            },
        )
        .err(),
        Some(HumanOperationError::Refused)
    );
    assert_eq!(
        outbox
            .status(hash)
            .unwrap_or_else(|| panic!("unknown missing"))
            .state,
        SubmissionState::Unknown
    );

    let planned = route_operator_command(
        &root,
        &tenant(),
        &outbox,
        OPERATOR,
        [10; 32],
        OperatorCommand::ResolveUnknown(hash),
    )
    .unwrap_or_else(|error| panic!("resolve plan: {error:?}"));
    assert_eq!(
        planned.bytes(),
        [&[2_u8][..], &4_u64.to_be_bytes()].concat()
    );

    let surface =
        Surface::open(&root, &tenant()).unwrap_or_else(|error| panic!("surface: {error:?}"));
    assert_eq!(
        verify_chain(surface.audit_path())
            .unwrap_or_else(|error| panic!("verify audit: {error}"))
            .entries,
        4
    );
    let _ = fs::remove_dir_all(root);
}

#[test]
fn authenticated_human_listener_frames_operator_commands_and_reopens_audit() {
    use layerx_agentd::human::frame::{read_frame, write_frame};
    use layerx_agentd::human::{self, serve_one, AcceptedTransport, HumanPeer};
    use layerx_agentd::human_runtime::ProductionHumanOperations;
    use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
    use layerx_client::sequencer::SequencerEndpoint;
    use std::{
        collections::BTreeMap,
        os::unix::{
            fs::{MetadataExt as _, PermissionsExt as _},
            net::{UnixListener, UnixStream},
        },
        sync::{Arc, Mutex},
        time::{Duration, Instant},
    };
    let root = directory("operator-real-listener");
    let mut store = Store::open(&root).expect("operator store");
    fs::set_permissions(&root, fs::Permissions::from_mode(0o700))
        .expect("protected listener parent");
    let mut outbox = Outbox::default();
    let hash = enqueue_unknown(&mut store, &mut outbox, 2);
    let activity_id = outbox.status(hash).expect("unknown").activity_id;
    drop(outbox);
    let shared = Arc::new(Mutex::new(store));
    let identity = fs::metadata(&root).expect("owner metadata");
    let peers = BTreeMap::from([(
        identity.uid(),
        (OPERATOR.to_owned(), tenant().as_str().to_owned()),
    )]);
    let (authority, authority_server) = repair_support::authority_peer(vec![]);
    authority_server
        .join()
        .expect("no authority reads for operator facts");
    let (url, node_server) =
        repair_support::rpc_peer(vec![("eth_chainId", serde_json::json!("0x7d"))]);
    let node = NodeRpc::connect(&NodeRpcConfig {
        url: url.clone(),
        chain_id: 125,
        deadline: Duration::from_secs(3),
    })
    .expect("node startup");
    node_server.join().expect("node startup transcript");
    let endpoint = SequencerEndpoint::new(&url, &[]).expect("endpoint");
    let (clock, clock_server) = repair_support::clock_peer(&root);
    let mut operations = ProductionHumanOperations::new(
        authority,
        endpoint,
        Box::new(node),
        shared.clone(),
        &peers,
        1_024,
        30,
        clock,
    )
    .expect("production operator dispatcher");
    let socket = root.join("human.sock");
    // Own a bounded real listener so its production transport and operation
    // owner can both be dropped before the durable Store is reopened.
    let listener = UnixListener::bind(&socket).expect("real Human listener");
    fs::set_permissions(&socket, fs::Permissions::from_mode(0o600))
        .expect("protected Human socket");
    listener.set_nonblocking(true).expect("bounded accept");
    let worker = std::thread::spawn(move || {
        for _ in 0..2 {
            let deadline = Instant::now() + Duration::from_secs(5);
            let stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < deadline, "missing Human request");
                        std::thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("Human accept: {error}"),
                }
            };
            let credentials = rustix::net::sockopt::socket_peercred(&stream)
                .expect("kernel-authenticated Human peer");
            assert_eq!(credentials.uid.as_raw(), identity.uid());
            let (principal, tenant) = peers
                .get(&credentials.uid.as_raw())
                .expect("configured peer principal and tenant");
            let peer = HumanPeer {
                uid: credentials.uid.as_raw(),
                principal: principal.clone(),
                tenant: tenant.clone(),
                subject: None,
            };
            let mut transport = AcceptedTransport::new(stream, 4096, Duration::from_secs(3))
                .expect("production accepted Human transport");
            serve_one(&mut transport, &peer, &mut operations)
                .expect("production framed Human dispatch");
        }
    });
    let exchange = |code: u8, request_id: u8| {
        let mut request = b"LXHAGT01".to_vec();
        request.push(human::OPERATOR);
        request.extend_from_slice(&(OPERATOR.len() as u32).to_be_bytes());
        request.extend_from_slice(OPERATOR.as_bytes());
        request.extend_from_slice(&[request_id; 32]);
        request.push(code);
        request.extend_from_slice(&hash);
        let mut stream = UnixStream::connect(&socket).expect("Human socket");
        stream
            .set_read_timeout(Some(Duration::from_secs(3)))
            .expect("read bound");
        stream
            .set_write_timeout(Some(Duration::from_secs(3)))
            .expect("write bound");
        write_frame(&mut stream, &request, 4096).expect("real frame encoder");
        read_frame(&mut stream, 4096).expect("real frame decoder")
    };
    let inspected = exchange(1, 21);
    assert_eq!(&inspected[..9], b"LXHAGT01\x00");
    let expected = [
        vec![1],
        activity_id.to_vec(),
        hash.to_vec(),
        vec![5],
        1u64.to_be_bytes().to_vec(),
    ]
    .concat();
    assert_eq!(&inspected[9..], expected);
    let refused = exchange(33, 22);
    assert_eq!(
        refused,
        human::encode_response(Err(HumanOperationError::Refused))
    );
    worker
        .join()
        .expect("drop listener and production operation owner");
    assert_eq!(Arc::strong_count(&shared), 1);
    drop(shared);
    let reopened = Store::open(&root).expect("reopen operator store");
    let mut recovered = Outbox::default();
    recovered
        .restore(&reopened, tenant(), hash)
        .expect("reload unknown");
    assert_eq!(
        recovered.status(hash).expect("unknown remains").state,
        SubmissionState::Unknown
    );
    assert_eq!(
        recovered.exact_signed_bytes(hash).expect("retained bytes"),
        signed_transaction(2)
    );
    let surface = Surface::open(&root, &tenant()).expect("reopened operator audit");
    assert_eq!(
        verify_chain(surface.audit_path())
            .expect("durable audit chain")
            .entries,
        2
    );
    drop(surface);
    drop(recovered);
    drop(reopened);
    drop(clock_server);
    fs::remove_dir_all(root).expect("remove operator fixture");
}
