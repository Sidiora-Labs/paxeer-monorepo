use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::time::Duration;

use ed25519_dalek::SigningKey;
use layerx_agentd::read::{NativeReadError, NativeReadRoute};
use layerx_client::availability::{
    AvailabilitySelector, FetchContext, FetchOutcome, RetrievalLimits,
};
use layerx_client::client::{ClientConfig, ReconnectPolicy};
use layerx_client::evidence::FinalityEvidenceCandidate;
use layerx_client::lni::handshake::HandshakeConfig;
use layerx_client::lni::schema::Version;
use layerx_client::lni::transport::Limits;
use layerx_client::Client;
use layerx_programs::hex;
use layerx_proof::availability::RootCommitments;
use layerx_types::ids::Did;

fn connect(socket: &Path) -> Client {
    Client::connect(ClientConfig {
        endpoint: socket.to_path_buf(),
        handshake: HandshakeConfig {
            built_interface_version: Version::V1_5,
            expected_protocol_version: 3,
            expected_network_id: 77,
        },
        limits: Limits {
            maximum_frame_bytes: 1_212_416,
            maximum_connections: 1,
            maximum_streams: 1,
            maximum_queued_bytes: 1_212_416,
            deadline: Duration::from_secs(10),
        },
        reconnect: ReconnectPolicy {
            maximum_attempts: 1,
            base_delay: Duration::ZERO,
            maximum_delay: Duration::ZERO,
            jitter_percent: 0,
        },
    })
    .unwrap_or_else(|error| panic!("real daemon connection: {error:?}"))
}

fn prepare_stage(client: &mut Client, stage: &str, work: &Path) {
    if stage == "retained" {
        let signed = client
            .batch_header(1, 1)
            .unwrap_or_else(|error| panic!("signed header: {error:?}"));
        let header = &signed.header;
        let outcome = client
            .fetch_availability(
                AvailabilitySelector::SealedCandidate(1),
                FetchContext {
                    interface_version: Version::V1_5,
                    correlation_id: 2,
                    expected_batch_number: 1,
                    data_availability_root: header.data_availability_root(),
                    record_roots: RootCommitments {
                        activity: header.activity_merkle_root(),
                        receipt: header.receipt_merkle_root(),
                        event: header.event_merkle_root(),
                        oracle: header.oracle_root(),
                    },
                    limits: RetrievalLimits {
                        maximum_bytes: 96 * 1024,
                        maximum_chunks: 256,
                        deadline: Duration::from_secs(10),
                    },
                },
                |_| {},
            )
            .unwrap_or_else(|error| panic!("candidate availability: {error:?}"));
        assert!(
            matches!(outcome, FetchOutcome::Complete(_)),
            "candidate incomplete: {outcome:?}"
        );
        assert!(!work.join("availability-output/checkpoint.bin").exists());
        let pending = work.join("availability-output/header.pending");
        fs::write(&pending, signed.canonical_bytes())
            .unwrap_or_else(|error| panic!("signed header: {error}"));
        fs::rename(
            pending,
            work.join("availability-output/available-header.bin"),
        )
        .unwrap_or_else(|error| panic!("header publication: {error}"));
    } else if stage == "finalized" {
        let checkpoint = fs::read(work.join("availability-output/checkpoint.bin"))
            .unwrap_or_else(|error| panic!("checkpoint: {error}"));
        let finality = fs::read(work.join("availability-output/finality.bin"))
            .unwrap_or_else(|error| panic!("finality: {error}"));
        let candidate = FinalityEvidenceCandidate::from_exact_bytes(checkpoint, finality, 3, 77)
            .unwrap_or_else(|error| panic!("settlement verification: {error:?}"));
        assert_eq!(
            client
                .register_finality_evidence(&candidate, 3)
                .unwrap_or_else(|error| panic!("finality registration: {error:?}"))
                .batch_number,
            1
        );
    }
}

fn probe(socket: &Path, stage: &str, work: &Path) {
    let mut client = connect(socket);
    prepare_stage(&mut client, stage, work);
    let key = SigningKey::from_bytes(&[0x11; 32]);
    let did =
        Did::new(format!("did:layerx:{}", hex::encode(key.verifying_key().as_bytes())).as_bytes())
            .unwrap_or_else(|error| panic!("actor DID: {error:?}"));
    let mut route = NativeReadRoute::new(
        client,
        did,
        "test-native-read-cursor-key-32-bytes".to_owned(),
        layerx_client::runtime_clock::RuntimeClock::from_environment()
            .unwrap_or_else(|error| panic!("native read clock: {error}")),
    )
    .unwrap_or_else(|error| panic!("native route: {error:?}"));
    if stage != "finalized" {
        assert!(route.read("/v1/reads/checkpoint/1").is_err());
        let availability = route.read("/v1/reads/availability/1");
        assert!(availability
            .as_ref()
            .map_or(true, |value| value["complete"] != true));
        return;
    }
    finalized_reads(&mut route, work);
}

fn finalized_reads(route: &mut NativeReadRoute, work: &Path) {
    let activity = fs::read_to_string(work.join("availability-activity-id"))
        .unwrap_or_else(|error| panic!("activity: {error}"));
    for (path, operation) in [
        (
            format!("/v1/reads/receipt/{}", activity.trim()),
            "read.receipt",
        ),
        (
            format!("/v1/reads/proof/{}", activity.trim()),
            "read.proof_bundle",
        ),
        ("/v1/reads/checkpoint/1".to_owned(), "read.checkpoint"),
        ("/v1/reads/availability/1".to_owned(), "read.availability"),
    ] {
        let Err(NativeReadError::UnavailableCapability(error)) = route.read(&path) else {
            panic!("height-addressed operation must refuse");
        };
        assert_eq!(
            error.class,
            layerx_agent_api::error::ErrorClass::UnavailableCapability
        );
        assert_eq!(
            error.reason.as_str(),
            format!("unavailable_capability.{operation}")
        );
    }
    history_reads(route, "account");
}

fn history_reads(route: &mut NativeReadRoute, account: &str) {
    let Err(NativeReadError::UnavailableCapability(error)) =
        route.read(&format!("/v1/reads/history/{account}?limit=2"))
    else {
        panic!("height-addressed history must refuse");
    };
    assert_eq!(
        error.class,
        layerx_agent_api::error::ErrorClass::UnavailableCapability
    );
    assert_eq!(error.reason.as_str(), "unavailable_capability.read.history");
}

#[test]
fn real_daemon_availability_refusals() {
    if let Some(socket) = std::env::var_os("LAYERX_TEST_AVAILABILITY_SOCKET") {
        let stage = std::env::var("LAYERX_TEST_AVAILABILITY_STAGE")
            .unwrap_or_else(|error| panic!("stage: {error}"));
        assert!(matches!(
            stage.as_str(),
            "retained" | "finalized" | "corrupt"
        ));
        let work = PathBuf::from(
            std::env::var_os("LAYERX_TEST_AVAILABILITY_WORK")
                .unwrap_or_else(|| panic!("work directory")),
        );
        probe(Path::new(&socket), &stage, &work);
        return;
    }
    let repository = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../../..");
    let binaries = std::env::var_os("LAYERX_TEST_NATIVE_BIN_DIR")
        .map_or_else(|| repository.join("build/bin"), PathBuf::from);
    assert!(binaries.join("layerxd").is_file());
    assert!(binaries.join("layerx-genesis-build").is_file());
    let clock = PathBuf::from(
        std::env::var_os("LAYERX_TEST_RUNTIME_CLOCK_BIN")
            .unwrap_or_else(|| panic!("actual runtime clock binary is required")),
    );
    assert!(clock.is_file());
    let status = Command::new("bash")
        .arg(repository.join("tests/daemon/program-admission.sh"))
        .args(["build", "--availability-batches"])
        .arg(std::env::current_exe().unwrap_or_else(|error| panic!("test executable: {error}")))
        .env("LAYERX_TEST_NATIVE_BIN_DIR", binaries)
        .env("LAYERX_TEST_RUNTIME_CLOCK_BIN", clock)
        .env("CARGO_BUILD_JOBS", "4")
        .env("MAKEFLAGS", "-j4")
        .current_dir(&repository)
        .status()
        .unwrap_or_else(|error| panic!("daemon harness: {error}"));
    assert!(status.success(), "real native reads harness: {status}");
}
