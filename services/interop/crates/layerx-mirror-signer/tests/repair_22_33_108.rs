use std::fmt::Debug;
use std::fs;
use std::os::unix::fs::{symlink, PermissionsExt};
use std::path::{Path, PathBuf};
use std::sync::Barrier;
use std::thread;
use std::time::Duration;

use ed25519_dalek::SigningKey as Ed25519SigningKey;
use k256::ecdsa::SigningKey;
use layerx_mirror::signer::{
    ChainSignature, RemoteChainSigner, RemoteSignerConfig, SignerEndpoint, SignerError,
    SigningAlgorithm,
};
use layerx_mirror_signer::{
    BridgeOptions, Options, SignerListener, StartupError, BRIDGE_ATTEST_INBOUND_DOMAIN,
    BRIDGE_ATTEST_OUTBOUND_DOMAIN, BRIDGE_ETHEREUM_TRANSACTION_DOMAIN,
    BRIDGE_PAXEER_TRANSACTION_DOMAIN, ETHEREUM_POLICY_DOMAIN, SOLANA_POLICY_DOMAIN,
};
use serde_json::{json, Value};

fn must<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("fixture or production path failed: {error:?}"))
}

struct Fixture {
    directory: PathBuf,
    options: Options,
}

impl Fixture {
    fn new(name: &str) -> Self {
        let directory =
            std::env::temp_dir().join(format!("signer-22-33-108-{}-{name}", std::process::id()));
        must(fs::create_dir(&directory));
        must(fs::set_permissions(
            &directory,
            fs::Permissions::from_mode(0o700),
        ));
        let options = Options {
            socket: directory.join("signer.sock"),
            ethereum_key_file: directory.join("ethereum.key"),
            solana_keypair_file: directory.join("solana.json"),
            ..Options::default()
        };
        write_private(&options.ethereum_key_file, "11".repeat(32).as_bytes());
        let key = Ed25519SigningKey::from_bytes(&[7; 32]);
        write_private(
            &options.solana_keypair_file,
            &must(serde_json::to_vec(&key.to_keypair_bytes().as_slice())),
        );
        Self { directory, options }
    }

    fn manifest(&self, entries: Vec<Value>) -> BridgeOptions {
        let keys_file = self.directory.join("keys.json");
        write_private(
            &keys_file,
            &must(serde_json::to_vec(&json!({"keys": entries}))),
        );
        BridgeOptions {
            socket: self.options.socket.clone(),
            keys_file,
        }
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        must(fs::remove_dir_all(&self.directory));
    }
}

fn write_private(path: &Path, bytes: &[u8]) {
    must(fs::write(path, bytes));
    must(fs::set_permissions(path, fs::Permissions::from_mode(0o600)));
}

fn entry(handle: &str, key: &Path, domain: &[u8]) -> Value {
    json!({
        "handle": handle,
        "algorithm": "secp256k1",
        "key_file": key,
        "domains": [must(std::str::from_utf8(domain))],
    })
}

fn client(socket: &Path, handle: &str, scalar: u8) -> RemoteChainSigner {
    let key = must(SigningKey::from_slice(&[scalar; 32]));
    remote(
        socket,
        handle,
        SigningAlgorithm::Secp256k1Recoverable,
        key.verifying_key()
            .to_encoded_point(true)
            .as_bytes()
            .to_vec(),
    )
}

fn remote(
    socket: &Path,
    handle: &str,
    algorithm: SigningAlgorithm,
    public_key: Vec<u8>,
) -> RemoteChainSigner {
    must(RemoteChainSigner::new(RemoteSignerConfig {
        endpoint: SignerEndpoint::Uds {
            socket: socket.to_owned(),
        },
        algorithm,
        key_handle: handle.to_owned(),
        public_key,
        timeout: Duration::from_secs(5),
    }))
}

fn round_trip<T>(listener: &SignerListener, request: impl FnOnce() -> T) -> T {
    thread::scope(|scope| {
        let server = scope.spawn(|| listener.serve_once());
        let result = request();
        must(must(server.join()));
        result
    })
}

fn assert_signed(listener: &SignerListener, handle: &str, scalar: u8, domain: &[u8]) {
    let signer = client(listener.socket(), handle, scalar);
    let signature = must(round_trip(listener, || {
        signer.sign_digest(domain, [0x53; 32])
    }));
    assert!(matches!(signature, ChainSignature::Secp256k1(bytes) if bytes[64] <= 1));
}

#[test]
fn signer_role_alias_rejects_shared_identities_across_handles() {
    let fixture = Fixture::new("role-alias");
    let copied = fixture.directory.join("copied.key");
    // A different path and textual encoding still name the same public identity.
    write_private(&copied, format!("  0x{}\n", "11".repeat(32)).as_bytes());
    let linked = fixture.directory.join("hard-linked.key");
    must(fs::hard_link(&fixture.options.ethereum_key_file, &linked));
    for key in [&fixture.options.ethereum_key_file, &copied, &linked] {
        for attestation in [BRIDGE_ATTEST_INBOUND_DOMAIN, BRIDGE_ATTEST_OUTBOUND_DOMAIN] {
            for transaction in [
                BRIDGE_PAXEER_TRANSACTION_DOMAIN,
                BRIDGE_ETHEREUM_TRANSACTION_DOMAIN,
            ] {
                for reverse in [false, true] {
                    let mut entries = vec![
                        entry("attestor", &fixture.options.ethereum_key_file, attestation),
                        entry("fee-payer", key, transaction),
                    ];
                    if reverse {
                        entries.reverse();
                    }
                    let options = fixture.manifest(entries);
                    match SignerListener::bind_bridge(&options) {
                        Err(StartupError::Manifest { reason, .. }) => {
                            assert!(reason.contains("shares a public identity"), "{reason}");
                        }
                        Err(error) => panic!("wrong refusal: {error}"),
                        Ok(_) => panic!("one public identity must never attest and pay fees"),
                    }
                    assert!(
                        !options.socket.exists(),
                        "refused keys must not publish a socket"
                    );
                }
            }
        }
    }
}

#[test]
fn signer_role_alias_separated_keys_sign_through_the_real_client() {
    let fixture = Fixture::new("separated-roles");
    let submitter = fixture.directory.join("submitter.key");
    write_private(&submitter, "22".repeat(32).as_bytes());
    let options = fixture.manifest(vec![
        entry(
            "attestor-in",
            &fixture.options.ethereum_key_file,
            BRIDGE_ATTEST_INBOUND_DOMAIN,
        ),
        entry("fee-payer", &submitter, BRIDGE_PAXEER_TRANSACTION_DOMAIN),
        entry(
            "attestor-out",
            &fixture.options.ethereum_key_file,
            BRIDGE_ATTEST_OUTBOUND_DOMAIN,
        ),
        entry(
            "ethereum-fees",
            &submitter,
            BRIDGE_ETHEREUM_TRANSACTION_DOMAIN,
        ),
    ]);
    let listener = must(SignerListener::bind_bridge(&options));
    for (handle, scalar, domain) in [
        ("attestor-in", 0x11, BRIDGE_ATTEST_INBOUND_DOMAIN),
        ("attestor-out", 0x11, BRIDGE_ATTEST_OUTBOUND_DOMAIN),
        ("fee-payer", 0x22, BRIDGE_PAXEER_TRANSACTION_DOMAIN),
        ("ethereum-fees", 0x22, BRIDGE_ETHEREUM_TRANSACTION_DOMAIN),
    ] {
        assert_signed(&listener, handle, scalar, domain);
    }
    for (handle, scalar, domain) in [
        ("attestor-in", 0x11, BRIDGE_PAXEER_TRANSACTION_DOMAIN),
        ("fee-payer", 0x22, BRIDGE_ATTEST_INBOUND_DOMAIN),
    ] {
        let signer = client(listener.socket(), handle, scalar);
        assert_eq!(
            round_trip(&listener, || signer.sign_digest(domain, [0x53; 32])),
            Err(SignerError::Refused)
        );
    }
}

#[test]
fn signer_secret_readability_requires_owner_only_files_for_both_algorithms() {
    let fixture = Fixture::new("permissions");
    let bridge = fixture.manifest(vec![entry(
        "attestor",
        &fixture.options.ethereum_key_file,
        BRIDGE_ATTEST_INBOUND_DOMAIN,
    )]);
    for path in [
        &fixture.options.ethereum_key_file,
        &fixture.options.solana_keypair_file,
    ] {
        for mode in [0o644, 0o604, 0o640, 0o440, 0o620, 0o602, 0o601] {
            must(fs::set_permissions(path, fs::Permissions::from_mode(mode)));
            match SignerListener::bind(&fixture.options) {
                Err(StartupError::KeyMaterial {
                    path: refused,
                    reason,
                }) => {
                    assert_eq!(&refused, path);
                    assert!(reason.contains("owner-only"), "{reason}");
                }
                Err(error) => panic!("wrong refusal for mode {mode:o}: {error}"),
                Ok(_) => panic!("mode {mode:o} exposes private material"),
            }
            if path == &fixture.options.ethereum_key_file {
                assert!(matches!(
                    SignerListener::bind_bridge(&bridge),
                    Err(StartupError::KeyMaterial { .. })
                ));
            }
            assert!(!fixture.options.socket.exists());
        }
        must(fs::set_permissions(path, fs::Permissions::from_mode(0o600)));
    }
    for mode in [0o400, 0o600] {
        for path in [
            &fixture.options.ethereum_key_file,
            &fixture.options.solana_keypair_file,
        ] {
            must(fs::set_permissions(path, fs::Permissions::from_mode(mode)));
        }
        let listener = must(SignerListener::bind(&fixture.options));
        assert_signed(
            &listener,
            &fixture.options.ethereum_key_handle,
            0x11,
            ETHEREUM_POLICY_DOMAIN,
        );
        let solana = remote(
            listener.socket(),
            &fixture.options.solana_key_handle,
            SigningAlgorithm::Ed25519,
            Ed25519SigningKey::from_bytes(&[7; 32])
                .verifying_key()
                .to_bytes()
                .to_vec(),
        );
        let signature = must(round_trip(&listener, || {
            solana.sign_message(SOLANA_POLICY_DOMAIN, b"archive publication")
        }));
        assert!(matches!(signature, ChainSignature::Ed25519(bytes) if bytes != [0; 64]));
    }
}

fn replace_link(link: &Path, target: &Path) {
    let staging = link.with_extension("next");
    must(symlink(target, &staging));
    must(fs::rename(staging, link));
}

#[test]
fn signer_secret_readability_rechecks_symlink_targets_after_replacement() {
    let fixture = Fixture::new("replacement");
    let original = fixture.directory.join("original.key");
    must(fs::rename(&fixture.options.ethereum_key_file, &original));
    must(symlink(&original, &fixture.options.ethereum_key_file));
    let replacement = fixture.directory.join("replacement.key");
    write_private(&replacement, "22".repeat(32).as_bytes());
    must(fs::set_permissions(
        &replacement,
        fs::Permissions::from_mode(0o644),
    ));
    let listener = must(SignerListener::bind(&fixture.options));
    replace_link(&fixture.options.ethereum_key_file, &replacement);
    assert_signed(
        &listener,
        &fixture.options.ethereum_key_handle,
        0x11,
        ETHEREUM_POLICY_DOMAIN,
    );
    drop(listener);
    assert!(matches!(
        SignerListener::bind(&fixture.options),
        Err(StartupError::KeyMaterial { .. })
    ));
    must(fs::set_permissions(
        &replacement,
        fs::Permissions::from_mode(0o600),
    ));
    let reopened = must(SignerListener::bind(&fixture.options));
    assert_signed(
        &reopened,
        &fixture.options.ethereum_key_handle,
        0x22,
        ETHEREUM_POLICY_DOMAIN,
    );
    drop(reopened);
    replace_link(&fixture.options.ethereum_key_file, &fixture.directory);
    assert!(matches!(
        SignerListener::bind(&fixture.options),
        Err(StartupError::KeyMaterial { .. })
    ));
    let oversized = fixture.directory.join("oversized.key");
    write_private(&oversized, &[b'1'; 4097]);
    replace_link(&fixture.options.ethereum_key_file, &oversized);
    assert!(matches!(
        SignerListener::bind(&fixture.options),
        Err(StartupError::KeyMaterial { .. })
    ));
}

#[test]
fn signer_secret_readability_checks_the_descriptor_during_atomic_replacement() {
    let fixture = Fixture::new("atomic-replacement");
    let secure = fixture.directory.join("secure.key");
    must(fs::rename(&fixture.options.ethereum_key_file, &secure));
    must(symlink(&secure, &fixture.options.ethereum_key_file));
    let exposed = fixture.directory.join("exposed.key");
    write_private(&exposed, "22".repeat(32).as_bytes());
    must(fs::set_permissions(
        &exposed,
        fs::Permissions::from_mode(0o644),
    ));
    // Guarantee the accepted branch is exercised, independently of race scheduling.
    let listener = must(SignerListener::bind(&fixture.options));
    assert_signed(
        &listener,
        &fixture.options.ethereum_key_handle,
        0x11,
        ETHEREUM_POLICY_DOMAIN,
    );
    drop(listener);
    let barrier = Barrier::new(2);
    thread::scope(|scope| {
        let writer = scope.spawn(|| {
            barrier.wait();
            for _ in 0..2048 {
                replace_link(&fixture.options.ethereum_key_file, &exposed);
                replace_link(&fixture.options.ethereum_key_file, &secure);
            }
        });
        barrier.wait();
        for _ in 0..128 {
            match SignerListener::bind(&fixture.options) {
                Ok(listener) => assert_signed(
                    &listener,
                    &fixture.options.ethereum_key_handle,
                    0x11,
                    ETHEREUM_POLICY_DOMAIN,
                ),
                Err(StartupError::KeyMaterial { reason, .. }) => {
                    assert!(reason.contains("owner-only"), "{reason}")
                }
                Err(error) => panic!("unexpected replacement error: {error}"),
            }
        }
        must(writer.join());
    });
}
