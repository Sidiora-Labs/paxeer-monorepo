#[path = "repair_22_33_44_clock.rs"]
mod runtime_clock;

use std::net::TcpListener;
use std::os::unix::fs::PermissionsExt;
use std::sync::Arc;
use std::time::Duration;

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
        .or_fail("committed CA");
    MutualTlsConfig::new(
        roots,
        vec![CertificateDer::from(CLIENT.to_vec())],
        PrivateKeyDer::try_from(CLIENT_KEY.to_vec()).or_fail("fixture client key"),
    )
    .or_fail("mutual TLS client")
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
    request.extend_from_slice(
        &u32::try_from(PROVIDER.len())
            .unwrap_or_else(|_| panic!("provider length fits u32"))
            .to_be_bytes(),
    );
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
        let refusal = GatewayChainSnapshot::decode(document)
            .err()
            .unwrap_or_else(|| panic!("exact startup snapshot refusal"));
        assert!(
            refusal.contains("LAYERX_HUMAN_KMS_GATEWAY_REGISTRY_FILE"),
            "{refusal}"
        );
    }
    assert_eq!(
        GatewayChainSnapshot::decode(br#"{"chain_id":125}"#)
            .or_fail("valid snapshot")
            .chain_id,
        125
    );
}

fn serve_probe_provider(listener: TcpListener) -> std::thread::JoinHandle<()> {
    std::thread::spawn(move || {
        let mut roots = RootCertStore::empty();
        roots.add(CertificateDer::from(CA.to_vec())).or_fail("CA");
        let verifier = rustls::server::WebPkiClientVerifier::builder(Arc::new(roots))
            .build()
            .or_fail("client verification");
        let config = Arc::new(
            ServerConfig::builder()
                .with_client_cert_verifier(verifier)
                .with_single_cert(
                    vec![CertificateDer::from(SERVER.to_vec())],
                    PrivateKeyDer::try_from(SERVER_KEY.to_vec()).or_fail("server key"),
                )
                .or_fail("provider TLS"),
        );
        for _ in 0..2 {
            let mut accept_steps = 0..5_000;
            let stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(accept_steps.next().is_some(), "provider request deadline");
                        std::thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("provider accept: {error}"),
                }
            };
            stream
                .set_read_timeout(Some(Duration::from_secs(2)))
                .or_fail("read deadline");
            stream
                .set_write_timeout(Some(Duration::from_secs(2)))
                .or_fail("write deadline");
            let mut tls = StreamOwned::new(
                ServerConnection::new(config.clone()).or_fail("server"),
                stream,
            );
            assert_eq!(
                read_frame(&mut tls, GATEWAY_FRAME_LIMIT).unwrap_or_else(|error| panic!(
                    "{}: {error:?}",
                    "real remote provider request"
                )),
                probe()
            );
            write_frame(&mut tls, b"LXKP\0\x01\0\0", GATEWAY_FRAME_LIMIT)
                .or_fail("recorded probe response");
            tls.conn.send_close_notify();
            std::io::Write::flush(&mut tls).or_fail("flush provider response");
        }
    })
}

#[test]
fn kms_snapshot_serves_real_gateway_frames_with_authenticated_clock() {
    let _ = rustls::crypto::ring::default_provider().install_default();
    let root = layerx_human_test_support::directory("kms-clock-frame");
    std::fs::create_dir_all(&root).or_fail("root");
    std::fs::set_permissions(&root, std::fs::Permissions::from_mode(0o700))
        .or_fail("private clock root");
    let authority = runtime_clock::ClockAuthority::start(&root).or_fail("real clock authority");
    let clock = authority.connect();
    let listener = TcpListener::bind("127.0.0.1:0").or_fail("recorded provider listener");
    let endpoint = listener.local_addr().or_fail("provider address");
    listener.set_nonblocking(true).or_fail("bounded fixture");
    // The provider peer replays the committed LXKP probe transcript from the
    // existing frame test. Gateway, provider client, TLS and clock stay real.
    let provider_worker = serve_probe_provider(listener);
    let provider = RemoteKmsProvider::new(
        PROVIDER,
        endpoint,
        "localhost",
        client_tls(),
        limits(),
        clock.clone(),
    )
    .or_fail("real provider client");
    let snapshot =
        GatewayChainSnapshot::decode(br#"{"chain_id":125}"#).or_fail("production startup decoder");
    let gateway = LxkpGateway::new(
        Arc::new(provider),
        GatewayTls::new(CA, SERVER, &Zeroizing::new(SERVER_KEY.to_vec()), CLIENT)
            .or_fail("real gateway TLS"),
        GatewayPolicy {
            network_id: snapshot.chain_id,
            protocol_version: 3,
            registry: ModuleRegistry::new(&[]).or_fail("registry"),
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
    .or_fail("clock-bound gateway startup");
    let listener = TcpListener::bind("127.0.0.1:0").or_fail("gateway listener");
    let address = listener.local_addr().or_fail("gateway address");
    let worker = std::thread::spawn(move || {
        let (stream, _) = listener.accept().or_fail("gateway accept");
        gateway
            .serve_connection(stream)
            .or_fail("production gateway frame dispatch");
    });
    let gate = ConnectionGate::new(1);
    let mut transport = Tls::connect(
        address,
        ServerName::try_from("localhost").or_fail("name"),
        &client_tls(),
        &gate,
        limits(),
    )
    .or_fail("client mutual TLS");
    transport.send(&probe()).or_fail("real frame request");
    assert_eq!(
        transport.receive().or_fail("real gateway response"),
        b"LXKP\0\x01\0\0"
    );
    drop(transport);
    worker.join().or_fail("gateway worker");
    provider_worker.join().or_fail("provider peer");
    drop(clock);
    drop(authority);
    std::fs::remove_dir_all(root).or_fail("cleanup");
}

use crate::require_value::RequireValue as _;
