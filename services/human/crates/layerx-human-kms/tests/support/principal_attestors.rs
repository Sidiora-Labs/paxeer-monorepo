use super::attestor_cluster::{Cluster, Result, SIGNERS};
use layerx_human_service::custody::{KeyClass, PrincipalKeyBinding};
use serde_json::json;
use sha3::{Digest, Keccak256};
use std::fs;
use std::os::unix::fs::PermissionsExt;
use std::path::Path;

// Each row provisions a distinct key through the real threshold producer. No
// provider seal, persisted seed or descriptor is used to discover an EVM sender.
pub fn provision(
    cluster: &mut Cluster,
    root: &Path,
    provider: &str,
    raw_bindings: &[u8],
) -> Result<()> {
    let mut owners: Vec<_> = raw_bindings
        .iter()
        .map(|value| ([*value; 32], "alice".to_owned()))
        .collect();
    for (principal, class) in [
        ("alice", KeyClass::HumanPrimary),
        ("bob", KeyClass::HumanPrimary),
        ("bob", KeyClass::AgentPrimary),
    ] {
        let identity = format!(
            "layerx-human-custody/v1\0{principal}\0primary\0{}",
            class.label()
        );
        let binding = PrincipalKeyBinding::new(identity.into_bytes(), 125, class, provider)?;
        owners.push((binding.digest(), principal.into()));
    }
    let native = cluster.generate_key(
        "provider-native",
        "ed25519",
        "provider-native-owner",
        "0x1111111111111111111111111111111111111111",
    )?;
    for name in ["node-ca", "gateway"] {
        cluster.pki.certificate_der(name)?;
    }
    cluster.pki.key_der("gateway")?;
    fs::set_permissions(
        cluster.pki.path("gateway.pk8"),
        fs::Permissions::from_mode(0o600),
    )?;
    let nodes: Vec<_> = cluster
        .nodes
        .iter()
        .map(|(id, address)| json!({"id":id,"address":address}))
        .collect();
    let mut rows = Vec::new();
    for (index, (binding, principal)) in owners.into_iter().enumerate() {
        let owner = format!("provider-owner-{index}");
        let key_id = format!("provider-evm-{index}");
        let public = cluster.generate_key(&key_id, "secp256k1", &owner, "")?;
        let point = k256::ecdsa::VerifyingKey::from_sec1_bytes(&public)?.to_encoded_point(false);
        let address: [u8; 20] = Keccak256::digest(&point.as_bytes()[1..])[12..].try_into()?;
        let assertion_file = root.join(format!("owner-{index}.jwt"));
        fs::write(&assertion_file, cluster.tokens.mint(&owner)?)?;
        fs::set_permissions(&assertion_file, fs::Permissions::from_mode(0o600))?;
        rows.push(
            json!({"binding":binding,"tenant":"tenant","principal":principal,
            "assertion_file":assertion_file,"config":{
                "nodes":nodes,"trust_anchors":[cluster.pki.path("node-ca.der")],
                "client_certificate_chain":[cluster.pki.path("gateway.der")],
                "client_private_key_pkcs8":cluster.pki.path("gateway.pk8"),
                "key_id":"provider-native","public_key":native,
                "evm_key_id":key_id,"evm_address":address,"signers":SIGNERS,
                "network_id":125,"timeout_seconds":60
            }}),
        );
    }
    let path = root.join("attestor.json");
    fs::write(&path, serde_json::to_vec(&json!({"bindings":rows}))?)?;
    fs::set_permissions(path, fs::Permissions::from_mode(0o600))?;
    Ok(())
}
