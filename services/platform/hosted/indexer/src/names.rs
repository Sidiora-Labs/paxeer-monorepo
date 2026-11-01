use serde_json::{json, Map, Value};

use crate::abi::{keccak, AbiRegistry, DecodedEvent, LoadedAbi};
use crate::codec::{be_decimal, hex0x, unhex_fixed};
use crate::store::NameEventRow;
use crate::IndexError;

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct NamesConfig {
    pub registry: [u8; 20],
    pub registrar: [u8; 20],
    pub controller: [u8; 20],
    pub resolver: [u8; 20],
    pub reverse: [u8; 20],
}

impl NamesConfig {
    pub fn from_env() -> Result<Option<Self>, IndexError> {
        Self::from_lookup(&|key| std::env::var(key).ok())
    }

    pub fn from_lookup(
        lookup: &impl Fn(&str) -> Option<String>,
    ) -> Result<Option<Self>, IndexError> {
        let keys = ["REGISTRY", "REGISTRAR", "CONTROLLER", "RESOLVER", "REVERSE"];
        let values = keys.map(|key| lookup(&format!("LAYERX_INDEXER_NAMES_{key}")));
        if values.iter().all(Option::is_none) {
            return Ok(None);
        }
        let mut addresses = [[0; 20]; 5];
        for (index, value) in values.iter().enumerate() {
            let key = format!("LAYERX_INDEXER_NAMES_{}", keys[index]);
            let value = value.as_deref().ok_or_else(|| {
                IndexError::Config(format!(
                    "{key} is required when names indexing is configured"
                ))
            })?;
            addresses[index] = unhex_fixed(value)
                .map_err(|_| IndexError::Config(format!("{key} must be a 20-byte address")))?;
        }
        Ok(Some(Self {
            registry: addresses[0],
            registrar: addresses[1],
            controller: addresses[2],
            resolver: addresses[3],
            reverse: addresses[4],
        }))
    }
}

pub fn names_abi(config: &NamesConfig) -> Result<Vec<LoadedAbi>, IndexError> {
    let document: Value = serde_json::from_str(include_str!("../abis/names.json"))
        .map_err(|error| IndexError::Config(format!("names ABI: {error}")))?;
    [
        ("names_registry", config.registry),
        ("names_registrar", config.registrar),
        ("names_controller", config.controller),
        ("names_resolver", config.resolver),
    ]
    .into_iter()
    .map(|(contract, address)| {
        Ok(LoadedAbi {
            contract: contract.to_owned(),
            address: Some(address),
            events: AbiRegistry::parse_events(contract, &document[contract])?,
        })
    })
    .collect()
}

pub fn pax_node() -> [u8; 32] {
    child_node([0; 32], keccak(b"pax"))
}

pub fn child_node(parent: [u8; 32], label: [u8; 32]) -> [u8; 32] {
    let mut bytes = [0; 64];
    bytes[..32].copy_from_slice(&parent);
    bytes[32..].copy_from_slice(&label);
    keccak(&bytes)
}

pub fn reverse_node(sender: &str) -> Option<[u8; 32]> {
    let address = unhex_fixed::<20>(sender).ok()?;
    let reverse = child_node([0; 32], keccak(b"reverse"));
    let addr_reverse = child_node(reverse, keccak(b"addr"));
    Some(child_node(
        addr_reverse,
        keccak(crate::codec::hex(&address).as_bytes()),
    ))
}

fn decimal_word(text: &str) -> Option<[u8; 32]> {
    if text.is_empty() {
        return None;
    }
    let mut word = [0_u8; 32];
    for digit in text.bytes() {
        if !digit.is_ascii_digit() {
            return None;
        }
        let mut carry = u16::from(digit - b'0');
        for byte in word.iter_mut().rev() {
            let next = u16::from(*byte) * 10 + carry;
            *byte = u8::try_from(next & 255).ok()?;
            carry = next >> 8;
        }
        if carry != 0 {
            return None;
        }
    }
    Some(word)
}

pub fn decode(event: &DecodedEvent, sender: &str, timestamp: u64) -> Option<NameEventRow> {
    let text = |key: &str| event.arg(key)?.as_str();
    let bytes32 = |key: &str| unhex_fixed::<32>(text(key)?).ok();
    let mut changes = Map::new();
    let mut put = |field: &str, key: &str| -> Option<()> {
        changes.insert(field.to_owned(), Value::String(text(key)?.to_owned()));
        Some(())
    };
    let (node, action) = match (event.contract.as_str(), event.name.as_str()) {
        ("names_registry", "NewOwner") => {
            put("owner", "owner")?;
            (child_node(bytes32("node")?, bytes32("label")?), "transfer")
        }
        ("names_registry", "Transfer") => {
            put("owner", "owner")?;
            (bytes32("node")?, "transfer")
        }
        ("names_registry", "NewResolver") => {
            put("resolver", "resolver")?;
            (bytes32("node")?, "set_resolver")
        }
        ("names_registrar", "NameRegistered") => {
            put("owner", "owner")?;
            put("registrant", "owner")?;
            put("expires_at", "expires")?;
            put("token_id", "id")?;
            changes.insert("registered_at".to_owned(), json!(timestamp));
            (
                child_node(pax_node(), decimal_word(text("id")?)?),
                "register",
            )
        }
        ("names_registrar", "Transfer") => {
            put("owner", "to")?;
            put("registrant", "to")?;
            put("token_id", "tokenId")?;
            (
                child_node(pax_node(), decimal_word(text("tokenId")?)?),
                "transfer",
            )
        }
        ("names_controller", "NameRegistered" | "NameRenewed") => {
            put("label", "label")?;
            put("expires_at", "expires")?;
            let label = bytes32("labelhash")?;
            changes.insert("name".to_owned(), json!(format!("{}.pax", text("label")?)));
            changes.insert("token_id".to_owned(), json!(be_decimal(&label)));
            let action = if event.name == "NameRegistered" {
                changes.insert("owner".to_owned(), json!(text("owner")?));
                changes.insert("registrant".to_owned(), json!(text("owner")?));
                changes.insert("registered_at".to_owned(), json!(timestamp));
                "register"
            } else {
                "renew"
            };
            (child_node(pax_node(), label), action)
        }
        ("names_resolver", "AddrChanged") => {
            put("resolved_address", "a")?;
            (bytes32("node")?, "set_addr")
        }
        ("names_resolver", "ContenthashChanged") => {
            put("contenthash", "hash")?;
            (bytes32("node")?, "set_contenthash")
        }
        ("names_resolver", "TextChanged") => (bytes32("node")?, "set_text"),
        ("names_resolver", "NameChanged") => {
            put("name", "name")?;
            let node = bytes32("node")?;
            let action = if node == reverse_node(sender)? {
                "reverse_name_set"
            } else {
                "name_changed"
            };
            (node, action)
        }
        _ => return None,
    };
    Some(NameEventRow {
        height: 0,
        tx_hash: String::new(),
        log_index: 0,
        node: hex0x(&node),
        action: action.to_owned(),
        from_address: sender.to_owned(),
        timestamp,
        data: json!({"contract": event.contract, "event": event.name, "args": event.args, "changes": changes}),
    })
}
