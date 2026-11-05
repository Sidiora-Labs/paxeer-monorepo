#[path = "repair_22_33_44_clock.rs"]
mod runtime_clock;

use std::net::TcpListener;
use std::os::unix::fs::PermissionsExt;
use std::sync::Arc;
use std::time::{Duration, Instant};

use layerx_human_service::custody::{
    GatewayChainSnapshot, GatewayLimits, GatewayPolicy, GatewayTls, LxkpGateway, RemoteKmsProvider,
    GATEWAY_FRAME_LIMIT,
};
use layerx_human_service::frame::{
    read_frame, write_frame, ConnectionGate, FrameTransport, Limits, MutualTlsConfig, Tls,
};
use layerx_types::payload::ModuleRegistry;
use rustls::pki_types::{CertificateDer, PrivateKeyDer, ServerName};
use rustls::{RootCertStore, ServerConfig, ServerConnection, StreamOwned};
use zeroize::Zeroizing;

const CA: &[u8] =
    include_bytes!("../../../../../../tools/bringup/tests/fixtures/repair_m794/human-kms/ca.der");
const SERVER: &[u8] =
    include_bytes!("../../../../../../tools/bringup/tests/fixtures/repair_m794/human-kms/cert.der");
const SERVER_KEY: &[u8] =
    include_bytes!("../../../../../../tools/bringup/tests/fixtures/repair_m794/human-kms/key.der");
const CLIENT: &[u8] = include_bytes!(
    "../../../../../../tools/bringup/tests/fixtures/repair_m794/human-kms-client/cert.der"
);
const CLIENT_KEY: &[u8] = include_bytes!(
    "../../../../../../tools/bringup/tests/fixtures/repair_m794/human-kms-client/key.der"
);
const PROVIDER: &str = "gateway-frame-kms";

fn client_tls() -> MutualTlsConfig {
    let mut roots = RootCertStore::empty();
    roots
        .add(CertificateDer::from(CA.to_vec()))
        .expect("committed CA");
    MutualTlsConfig::new(
        roots,
        vec![CertificateDer::from(CLIENT.to_vec())],
        PrivateKeyDer::try_from(CLIENT_KEY.to_vec()).expect("fixture client key"),
    )
    .expect("mutual TLS client")
}
fn limits() -> Limits {
    Limits {
        maximum_frame_bytes: GATEWAY_FRAME_LIMIT,
        maximum_connections: 2,
        maximum_streams: 1,
        maximum_queued_bytes: GATEWAY_FRAME_LIMIT,
        deadline: Duration::from_secs(2),
    }
}
fn probe() -> Vec<u8> {
    let mut request = b"LXKP\0\x01\0".to_vec();
    request.extend_from_slice(&(PROVIDER.len() as u32).to_be_bytes());
    request.extend_from_slice(PROVIDER.as_bytes());
    request
}

#[test]
fn kms_snapshot_parser_retains_exact_registry_refusals() {
    for document in [
        br#"{"network_id":125,"protocol_version":3,"modules":[]}"#.as_slice(),
        br#"{"chain_id":125,"modules":[]}"#.as_slice(),
        br#"{"chain_id":125,"modules":[{"module_id":1,"activity_types":[65537]}]}"#.as_slice(),
        br#"{"chain_id":125,"modules":null}"#.as_slice(),
        br#"{"chain_id":124}"#.as_slice(),
        br#"{"chain_id":126}"#.as_slice(),
    ] {
        let refusal =
            GatewayChainSnapshot::decode(document).expect_err("exact startup snapshot refusal");
        assert!(
            refusal.contains("LAYERX_HUMAN_KMS_GATEWAY_REGISTRY_FILE"),
            "{refusal}"
        );
    }
    assert_eq!(
        GatewayChainSnapshot::decode(br#"{"chain_id":125}"#)
            .expect("valid snapshot")
            .chain_id,
        125
    );
}

#[test]
fn kms_snapshot_serves_real_gateway_frames_with_authenticated_clock() {
    let _ = rustls::crypto::ring::default_provider().install_default();
    let root = layerx_human_test_support::directory("kms-clock-frame");
    std::fs::create_dir_all(&root).expect("root");
    std::fs::set_permissions(&root, std::fs::Permissions::from_mode(0o700))
        .expect("private clock root");
    let authority = runtime_clock::ClockAuthority::start(&root).expect("real clock authority");
    let clock = authority.connect();
    let listener = TcpListener::bind("127.0.0.1:0").expect("recorded provider listener");
    let endpoint = listener.local_addr().expect("provider address");
    listener.set_nonblocking(true).expect("bounded fixture");
    // The provider peer replays the committed LXKP probe transcript from the
    // existing frame test. Gateway, provider client, TLS and clock stay real.
    let provider_worker = std::thread::spawn(move || {
        let mut roots = RootCertStore::empty();
        roots.add(CertificateDer::from(CA.to_vec())).expect("CA");
        let verifier = rustls::server::WebPkiClientVerifier::builder(Arc::new(roots))
            .build()
            .expect("client verification");
        let config = Arc::new(
            ServerConfig::builder()
                .with_client_cert_verifier(verifier)
                .with_single_cert(
                    vec![CertificateDer::from(SERVER.to_vec())],
                    PrivateKeyDer::try_from(SERVER_KEY.to_vec()).expect("server key"),
                )
                .expect("provider TLS"),
        );
        for _ in 0..2 {
            let expires = Instant::now() + Duration::from_secs(5);
            let stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < expires, "provider request deadline");
                        std::thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("provider accept: {error}"),
                }
            };
            stream
                .set_read_timeout(Some(Duration::from_secs(2)))
                .expect("read deadline");
            stream
                .set_write_timeout(Some(Duration::from_secs(2)))
                .expect("write deadline");
            let mut tls = StreamOwned::new(
                ServerConnection::new(config.clone()).expect("server"),
                stream,
            );
            assert_eq!(
                read_frame(&mut tls, GATEWAY_FRAME_LIMIT).expect("real remote provider request"),
                probe()
            );
            write_frame(&mut tls, b"LXKP\0\x01\0\0", GATEWAY_FRAME_LIMIT)
                .expect("recorded probe response");
            tls.conn.send_close_notify();
            std::io::Write::flush(&mut tls).expect("flush provider response");
        }
    });
    let provider = RemoteKmsProvider::new(
        PROVIDER,
        endpoint,
        "localhost",
        client_tls(),
        limits(),
        clock.clone(),
    )
    .expect("real provider client");
    let snapshot =
        GatewayChainSnapshot::decode(br#"{"chain_id":125}"#).expect("production startup decoder");
    let gateway = LxkpGateway::new(
        Arc::new(provider),
        GatewayTls::new(CA, SERVER, &Zeroizing::new(SERVER_KEY.to_vec()), CLIENT)
            .expect("real gateway TLS"),
        GatewayPolicy {
            network_id: snapshot.chain_id,
            protocol_version: 3,
            registry: ModuleRegistry::new(&[]).expect("registry"),
            limits: GatewayLimits {
                maximum_connections: 2,
                maximum_frame_bytes: GATEWAY_FRAME_LIMIT,
                deadline: Duration::from_secs(2),
                maximum_operations: 100,
                operation_window: Duration::from_secs(60),
            },
        },
        clock.clone(),
    )
    .expect("clock-bound gateway startup");
    let listener = TcpListener::bind("127.0.0.1:0").expect("gateway listener");
    let address = listener.local_addr().expect("gateway address");
    let worker = std::thread::spawn(move || {
        let (stream, _) = listener.accept().expect("gateway accept");
        gateway
            .serve_connection(stream)
            .expect("production gateway frame dispatch");
    });
    let gate = ConnectionGate::new(1);
    let mut transport = Tls::connect(
        address,
        ServerName::try_from("localhost").expect("name"),
        &client_tls(),
        &gate,
        limits(),
    )
    .expect("client mutual TLS");
    transport.send(&probe()).expect("real frame request");
    assert_eq!(
        transport.receive().expect("real gateway response"),
        b"LXKP\0\x01\0\0"
    );
    drop(transport);
    worker.join().expect("gateway worker");
    provider_worker.join().expect("provider peer");
    drop(clock);
    drop(authority);
    std::fs::remove_dir_all(root).expect("cleanup");
}
