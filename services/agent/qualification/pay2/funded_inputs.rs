use super::{funding, required::Required};
use layerx_crypto::account_derivation::{derive_from_seed, DerivedAccount};
use serde_json::Value;
use sha2::{Digest as _, Sha256};
use std::{
    collections::BTreeMap,
    fs,
    os::unix::fs::PermissionsExt,
    path::{Path, PathBuf},
};

pub const NETWORK_ID: u32 = 125;
pub const PROTOCOL_VERSION: u16 = layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION;

pub struct HttpAnswer {
    pub status: u16,
    pub headers: BTreeMap<String, String>,
    pub body: String,
}

pub struct Cluster {
    pub root: PathBuf,
    pub treasury_seed: [u8; 32],
    pub treasury_did: String,
    pub asset: [u8; 32],
    pub sequencer_id: [u8; 32],
    pub sequencer_key: [u8; 32],
    pub chain_id: u64,
    pub node_url: String,
}

pub struct ProvisionedInputs {
    pub cluster: Cluster,
    pub funding: funding::Funding,
    pub payer_account: DerivedAccount,
    pub recipient_account: DerivedAccount,
    pub gateway_url: String,
    pub ca_der: Vec<u8>,
    pub payer_key: Value,
    pub recipient_key: Value,
}

impl ProvisionedInputs {
    pub fn read() -> Self {
        let path = std::env::var_os("PAY2_FUNDED_INPUTS").required("PAY2 provisioned inputs");
        let metadata = fs::metadata(&path).required("PAY2 input metadata");
        assert_eq!(
            metadata.permissions().mode() & 0o077,
            0,
            "PAY2 inputs must be private"
        );
        let bytes = fs::read(path).required("PAY2 provisioned input file");
        let input: Value = serde_json::from_slice(&bytes).required("PAY2 provisioned JSON");
        let text = |key: &str| input[key].as_str().required("PAY2 input field").to_owned();
        let payer_account = derive_from_seed(&decode::<32>(&text("payer_derivation_seed")), 0)
            .required("PAY2 payer account derivation");
        let recipient_account =
            derive_from_seed(&decode::<32>(&text("recipient_derivation_seed")), 0)
                .required("PAY2 recipient account derivation");
        let payer_did = format!(
            "did:layerx:{}",
            hex_encode(&payer_account.layerx_public_key())
        );
        let recipient_did = format!(
            "did:layerx:{}",
            hex_encode(&recipient_account.layerx_public_key())
        );
        assert_eq!(text("payer_did"), payer_did);
        assert_eq!(text("recipient_did"), recipient_did);
        let chain_id = input["chain_id"].as_u64().required("PAY2 chain id");
        assert_eq!(chain_id, u64::from(NETWORK_ID));
        let root = PathBuf::from(text("root"));
        assert!(
            root.is_absolute() && root.is_dir(),
            "PAY2 provisioned evidence directory"
        );
        let cluster = Cluster {
            root,
            treasury_seed: *payer_account.layerx_seed(),
            treasury_did: payer_did,
            asset: decode(&text("asset")),
            sequencer_id: decode(&text("sequencer_id")),
            sequencer_key: decode(&text("sequencer_key")),
            chain_id,
            node_url: text("node_url"),
        };
        let funding = funding::Funding {
            recipient_seed: *recipient_account.layerx_seed(),
            recipient_did,
        };
        Self {
            cluster,
            funding,
            payer_account,
            recipient_account,
            gateway_url: text("gateway_url"),
            ca_der: fs::read(text("ca_der")).required("PAY2 gateway trust anchor"),
            payer_key: input["payer_key"].clone(),
            recipient_key: input["recipient_key"].clone(),
        }
    }
}

fn decode<const N: usize>(text: &str) -> [u8; N] {
    assert_eq!(text.len(), N * 2);
    assert!(text.is_ascii());
    std::array::from_fn(|i| {
        u8::from_str_radix(&text[2 * i..2 * i + 2], 16).required("PAY2 hex field")
    })
}

pub fn hex_encode(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

pub fn now_ms() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .required("PAY2 wall clock")
        .as_millis()
        .try_into()
        .required("PAY2 timestamp range")
}

pub fn random32() -> [u8; 32] {
    let mut bytes = [0; 32];
    getrandom::fill(&mut bytes).required("PAY2 entropy");
    bytes
}

pub fn sha256(parts: &[&[u8]]) -> [u8; 32] {
    let mut hash = Sha256::new();
    for part in parts {
        hash.update(part);
    }
    hash.finalize().into()
}

pub fn write(path: &Path, bytes: &[u8], mode: u32) {
    use std::{io::Write as _, os::unix::fs::OpenOptionsExt};
    let mut file = fs::OpenOptions::new()
        .write(true)
        .create_new(true)
        .mode(mode)
        .open(path)
        .required("PAY2 private evidence file");
    file.write_all(bytes).required("PAY2 evidence write");
    file.sync_all().required("PAY2 evidence sync");
}
