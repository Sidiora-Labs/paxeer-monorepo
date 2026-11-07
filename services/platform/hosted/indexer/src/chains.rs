use serde::{Deserialize, Serialize};

use crate::abi::{keccak, AbiType, DecodedEvent, EventDef, EventParam, LoadedAbi};
use crate::codec::unhex_fixed;
use crate::IndexError;

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct ChainsConfig {
    pub precompile: String,
}

impl Default for ChainsConfig {
    fn default() -> Self {
        Self {
            precompile: "0x000000000000000000000000000000000000101e".to_owned(),
        }
    }
}

impl ChainsConfig {
    pub fn from_env() -> Result<Self, IndexError> {
        Self::from_lookup(&|key| std::env::var(key).ok())
    }

    pub fn from_lookup(lookup: &impl Fn(&str) -> Option<String>) -> Result<Self, IndexError> {
        let config = Self {
            precompile: lookup("LAYERX_INDEXER_CHAINS_PRECOMPILE")
                .unwrap_or_else(|| Self::default().precompile),
        };
        unhex_fixed::<20>(&config.precompile).map_err(|_| {
            IndexError::Config(
                "LAYERX_INDEXER_CHAINS_PRECOMPILE must be a 20-byte address".to_owned(),
            )
        })?;
        Ok(config)
    }
}

pub fn chains_abi(config: &ChainsConfig) -> Vec<LoadedAbi> {
    use AbiType::{Address, String as Text, Uint};
    let definitions = [
        (
            "ChainCreated",
            "ChainCreated(uint32,string,address,uint64)",
            vec![
                ("id", Uint(32), true),
                ("name", Text, false),
                ("operator", Address, true),
                ("chainId", Uint(64), false),
            ],
        ),
        (
            "ChainStatusChanged",
            "ChainStatusChanged(uint32,uint8)",
            vec![("id", Uint(32), true), ("status", Uint(8), false)],
        ),
        (
            "ChainFunded",
            "ChainFunded(uint32,address,uint256)",
            vec![
                ("id", Uint(32), true),
                ("from", Address, true),
                ("amount", Uint(256), false),
            ],
        ),
        (
            "ChainOperatorChanged",
            "ChainOperatorChanged(uint32,address)",
            vec![("id", Uint(32), true), ("operator", Address, true)],
        ),
    ];
    vec![LoadedAbi {
        contract: "chains_registry".to_owned(),
        address: Some(
            unhex_fixed(&config.precompile)
                .unwrap_or_else(|error| panic!("invalid chains precompile: {error}")),
        ),
        events: definitions
            .into_iter()
            .map(|(name, signature, params)| EventDef {
                contract: "chains_registry".to_owned(),
                name: name.to_owned(),
                signature: signature.to_owned(),
                topic: keccak(signature.as_bytes()),
                params: params
                    .into_iter()
                    .map(|(name, kind, indexed)| EventParam {
                        name: name.to_owned(),
                        kind,
                        indexed,
                    })
                    .collect(),
            })
            .collect(),
    }]
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct ChainRecord {
    pub id: u32,
    pub name: String,
    pub operator: String,
    pub status: String,
    pub chain_id: u64,
    pub created_height: u64,
    pub bond: String,
    pub gas_allowance: u64,
    pub byte_allowance: u64,
    pub fee_share_bps: u32,
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub enum ChainRow {
    Created(ChainRecord),
    Status { id: u32, status: String },
    Funded { id: u32, amount: String },
    Operator { id: u32, operator: String },
}

pub fn decode(event: &DecodedEvent, height: u64, _tx_hash: &str) -> Option<ChainRow> {
    if event.contract != "chains_registry" {
        return None;
    }
    let text = |key: &str| event.arg(key)?.as_str();
    let id = text("id")?.parse::<u32>().ok()?;
    Some(match event.name.as_str() {
        "ChainCreated" => {
            let chain_id = 125_000_000 + u64::from(id);
            if text("chainId")?.parse::<u64>().ok()? != chain_id {
                return None;
            }
            ChainRow::Created(ChainRecord {
                id,
                name: text("name")?.to_owned(),
                operator: text("operator")?.to_owned(),
                status: "active".to_owned(),
                chain_id,
                created_height: height,
                bond: "0".to_owned(),
                gas_allowance: 500_000,
                byte_allowance: 65_536,
                fee_share_bps: 100,
            })
        }
        "ChainStatusChanged" => ChainRow::Status {
            id,
            status: match text("status")? {
                "1" => "active",
                "2" => "paused",
                "3" => "governance_paused",
                "4" => "retired",
                _ => return None,
            }
            .to_owned(),
        },
        "ChainFunded" => ChainRow::Funded {
            id,
            amount: text("amount")?.to_owned(),
        },
        "ChainOperatorChanged" => ChainRow::Operator {
            id,
            operator: text("operator")?.to_owned(),
        },
        _ => return None,
    })
}

#[derive(Clone, Debug, PartialEq)]
pub struct ChainTransaction {
    pub tx_id: String,
    pub chain: String,
    pub gas_used: u64,
}
