#[path = "support/attestor_cluster.rs"]
mod attestor_cluster;

use attestor_cluster::{Cluster, Result, SIGNERS};
use layerx_human_kms::attestor::{
    AttestorError, AttestorNodeConfig, AttestorSigner, AttestorSignerConfig, MIN_SIGNERS,
};
use sha3::{Digest, Keccak256};
use std::fs;
use std::os::unix::fs::PermissionsExt as _;
use std::path::PathBuf;

fn config(
    cluster: &Cluster,
    public_key: [u8; 32],
    address: [u8; 20],
) -> Result<AttestorSignerConfig> {
    cluster.pki.certificate_der("node-ca")?;
    cluster.pki.certificate_der("gateway")?;
    cluster.pki.key_der("gateway")?;
    fs::set_permissions(
        cluster.pki.path("gateway.pk8"),
        fs::Permissions::from_mode(0o600),
    )?;
    Ok(AttestorSignerConfig {
        nodes: cluster
            .nodes
            .iter()
            .map(|(id, address)| AttestorNodeConfig {
                id: id.clone(),
                address: *address,
            })
            .collect(),
        trust_anchors: vec![cluster.pki.path("node-ca.der")],
        client_certificate_chain: vec![cluster.pki.path("gateway.der")],
        client_private_key_pkcs8: cluster.pki.path("gateway.pk8"),
        key_id: "ed25519-owner".into(),
        public_key,
        evm_key_id: "evm-owner".into(),
        evm_address: address,
        signers: SIGNERS.iter().map(|id| (*id).to_owned()).collect(),
        network_id: 125,
        timeout_seconds: 60,
    })
}

fn unsigned_transaction() -> Vec<u8> {
    let mut raw = vec![0x02, 0xdf, 0x7d, 0x80, 0x01, 0x02, 0x82, 0x52, 0x08, 0x94];
    raw.extend_from_slice(&[0x44; 20]);
    raw.extend_from_slice(&[0x01, 0x80, 0xc0]);
    raw
}

fn recovered_address(signature: &[u8; 65], preimage: &[u8]) -> Result<[u8; 20]> {
    assert!(signature[64] <= 1);
    let compact = k256::ecdsa::Signature::from_slice(&signature[..64])?;
    assert!(compact.normalize_s().is_none());
    let recovery = k256::ecdsa::RecoveryId::from_byte(signature[64]).ok_or("invalid parity")?;
    let key = k256::ecdsa::VerifyingKey::recover_from_prehash(
        &Keccak256::digest(preimage),
        &compact,
        recovery,
    )?;
    let point = key.to_encoded_point(false);
    Ok(Keccak256::digest(&point.as_bytes()[1..])[12..].try_into()?)
}

#[test]
fn attestor_signs_an_eip1559_transaction_that_recovers_to_the_evm_address() -> Result<()> {
    let mut cluster = Cluster::start(&[])?;
    let owner = "evm-wallet-owner";
    let public = cluster.generate_key(
        "ed25519-owner",
        "ed25519",
        owner,
        "0x1111111111111111111111111111111111111111",
    )?;
    let evm_public = cluster.generate_key("evm-owner", "secp256k1", owner, "")?;
    let group = k256::ecdsa::VerifyingKey::from_sec1_bytes(&evm_public)?;
    let point = group.to_encoded_point(false);
    let address: [u8; 20] = Keccak256::digest(&point.as_bytes()[1..])[12..].try_into()?;
    let config = config(
        &cluster,
        public.try_into().map_err(|_| "Ed25519 key length")?,
        address,
    )?;
    let mut signer = AttestorSigner::from_config(&config)?;
    let unsigned = unsigned_transaction();
    assert!(matches!(
        signer.sign_evm_transaction(&unsigned, 125),
        Err(AttestorError::Configuration("EVM owner assertion"))
    ));
    signer.set_evm_assertion(&cluster.tokens.mint(owner)?)?;
    let before = cluster.audit(signer.client())?;
    assert!(signer.sign_evm_transaction(&unsigned, 1).is_err());
    assert!(signer.sign_evm_transaction(&[2, 0xc0], 125).is_err());
    let mut signed_input = unsigned.clone();
    signed_input[1] += 3;
    signed_input.extend_from_slice(&[0x80, 0x01, 0x01]);
    assert!(signer.sign_evm_transaction(&signed_input, 125).is_err());
    assert_eq!(before, cluster.audit(signer.client())?);

    let signature = signer.sign_evm_transaction(&unsigned, 125)?;
    assert_eq!(signature.len(), 65);
    assert_eq!(
        recovered_address(&signature, &unsigned)?,
        config.evm_address
    );
    let audit = signer.audit_sequences();
    assert!(audit.len() >= MIN_SIGNERS);
    assert_eq!(audit.len(), SIGNERS.len());
    for (node, previous) in SIGNERS.iter().zip(before) {
        assert!(audit[*node] > previous);
    }

    let mut wire = unsigned.clone();
    wire[1] += 3;
    wire.extend_from_slice(&[0x80, 0x80, 0x80]);
    let wire_signature = signer.sign_evm_transaction(&wire, 125)?;
    assert_eq!(recovered_address(&wire_signature, &unsigned)?, address);

    let mut wrong_config = config;
    wrong_config.evm_address[0] ^= 1;
    let mut wrong_signer = AttestorSigner::from_config(&wrong_config)?;
    wrong_signer.set_evm_assertion(&cluster.tokens.mint(owner)?)?;
    assert!(matches!(
        wrong_signer.sign_evm_transaction(&unsigned, 125),
        Err(AttestorError::SignatureInvalid { .. })
    ));
    Ok(())
}

#[test]
fn attestor_signer_config_without_evm_key_is_refused() -> Result<()> {
    let mut config = AttestorSignerConfig {
        nodes: Vec::new(),
        trust_anchors: Vec::new(),
        client_certificate_chain: Vec::new(),
        client_private_key_pkcs8: PathBuf::from("unused.pk8"),
        key_id: "ed25519-owner".into(),
        public_key: [1; 32],
        evm_key_id: String::new(),
        evm_address: [2; 20],
        signers: SIGNERS.iter().map(|id| (*id).to_owned()).collect(),
        network_id: 125,
        timeout_seconds: 60,
    };
    assert!(matches!(
        AttestorSigner::from_config(&config),
        Err(AttestorError::Configuration("EVM key id"))
    ));
    config.evm_key_id = " ".into();
    assert!(matches!(
        AttestorSigner::from_config(&config),
        Err(AttestorError::Configuration("EVM key id"))
    ));
    Ok(())
}
