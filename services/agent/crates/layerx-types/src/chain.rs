//! Chain trust pins and unverified block references for the merged network.
//!
//! These codecs validate framing and configuration, never signatures or inclusion.
//! A decoded reference cannot construct `verify::Verified` or establish a write rung.

use crate::json::{self, JsonValue};

pub const PAXEER_CHAIN_ID: u64 = 125;
pub const CHAIN_TRUST_SCHEMA: &str = "layerx/chain-trust/1";

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct InvalidChainEvidence;

/// An explicitly supplied trust pin for one Comet chain and validator-set height.
/// There is no default, file loader, or conversion from settlement configuration.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ValidatorSetTrust {
    comet_chain_id: String,
    height: u64,
    validator_set_hash: [u8; 32],
}

impl ValidatorSetTrust {
    pub fn parse(text: &str) -> Result<Self, InvalidChainEvidence> {
        let document = json::parse(text).map_err(|_| InvalidChainEvidence)?;
        let JsonValue::Object(fields) = &document else {
            return Err(InvalidChainEvidence);
        };
        let names = [
            "schema",
            "chain_id",
            "comet_chain_id",
            "height",
            "validator_set_hash",
        ];
        if fields.len() != names.len()
            || fields
                .iter()
                .any(|(name, _)| !names.contains(&name.as_str()))
        {
            return Err(InvalidChainEvidence);
        }
        if document
            .str_at("schema")
            .map_err(|_| InvalidChainEvidence)?
            != CHAIN_TRUST_SCHEMA
            || document
                .u64_at("chain_id")
                .map_err(|_| InvalidChainEvidence)?
                != PAXEER_CHAIN_ID
        {
            return Err(InvalidChainEvidence);
        }
        let comet_chain_id = document
            .str_at("comet_chain_id")
            .map_err(|_| InvalidChainEvidence)?;
        let height = document
            .u64_at("height")
            .map_err(|_| InvalidChainEvidence)?;
        let validator_set_hash = document
            .hex_array_at("validator_set_hash")
            .map_err(|_| InvalidChainEvidence)?;
        if comet_chain_id.is_empty()
            || comet_chain_id.len() > 50
            || !comet_chain_id
                .bytes()
                .all(|byte| byte.is_ascii_alphanumeric() || b"-_.".contains(&byte))
            || height == 0
            || height > i64::MAX as u64
            || validator_set_hash == [0; 32]
        {
            return Err(InvalidChainEvidence);
        }
        Ok(Self {
            comet_chain_id: comet_chain_id.to_owned(),
            height,
            validator_set_hash,
        })
    }

    #[must_use]
    pub fn comet_chain_id(&self) -> &str {
        &self.comet_chain_id
    }
    #[must_use]
    pub const fn chain_id(&self) -> u64 {
        PAXEER_CHAIN_ID
    }
    #[must_use]
    pub const fn height(&self) -> u64 {
        self.height
    }
    #[must_use]
    pub const fn validator_set_hash(&self) -> [u8; 32] {
        self.validator_set_hash
    }

    /// Checks the pin, without authenticating the referenced block or witness.
    pub fn check_reference(&self, reference: &BlockReference) -> Result<(), InvalidChainEvidence> {
        if self.height != reference.covering_height
            || self.validator_set_hash != reference.validator_set_hash
        {
            return Err(InvalidChainEvidence);
        }
        Ok(())
    }
}

/// Transport provenance awaiting committed-header, receipt coverage and storage
/// witness verification. Execution and covering heights are deliberately distinct.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct BlockReference {
    execution_height: u64,
    covering_height: u64,
    block_hash: [u8; 32],
    validator_set_hash: [u8; 32],
    state_root: [u8; 32],
}

impl BlockReference {
    pub fn new(
        execution_height: u64,
        covering_height: u64,
        block_hash: [u8; 32],
        validator_set_hash: [u8; 32],
        state_root: [u8; 32],
    ) -> Result<Self, InvalidChainEvidence> {
        if execution_height == 0
            || covering_height < execution_height
            || covering_height > i64::MAX as u64
            || block_hash == [0; 32]
            || validator_set_hash == [0; 32]
            || state_root == [0; 32]
        {
            return Err(InvalidChainEvidence);
        }
        Ok(Self {
            execution_height,
            covering_height,
            block_hash,
            validator_set_hash,
            state_root,
        })
    }

    /// Versioned provenance encoding; successful decoding is not proof verification.
    #[must_use]
    pub fn encode(&self) -> Vec<u8> {
        let mut bytes = Vec::with_capacity(125);
        bytes.extend_from_slice(b"LXBR\x01");
        for number in [PAXEER_CHAIN_ID, self.execution_height, self.covering_height] {
            bytes.extend_from_slice(&number.to_be_bytes());
        }
        for hash in [&self.block_hash, &self.validator_set_hash, &self.state_root] {
            bytes.extend_from_slice(hash);
        }
        bytes
    }

    pub fn decode(bytes: &[u8]) -> Result<Self, InvalidChainEvidence> {
        if bytes.len() != 125 || &bytes[..5] != b"LXBR\x01" {
            return Err(InvalidChainEvidence);
        }
        fn take<const N: usize>(bytes: &mut &[u8]) -> Result<[u8; N], InvalidChainEvidence> {
            let (value, rest) = bytes.split_at_checked(N).ok_or(InvalidChainEvidence)?;
            *bytes = rest;
            value.try_into().map_err(|_| InvalidChainEvidence)
        }
        let mut rest = &bytes[5..];
        if u64::from_be_bytes(take(&mut rest)?) != PAXEER_CHAIN_ID {
            return Err(InvalidChainEvidence);
        }
        Self::new(
            u64::from_be_bytes(take(&mut rest)?),
            u64::from_be_bytes(take(&mut rest)?),
            take(&mut rest)?,
            take(&mut rest)?,
            take(&mut rest)?,
        )
    }

    #[must_use]
    pub const fn execution_height(&self) -> u64 {
        self.execution_height
    }
    #[must_use]
    pub const fn covering_height(&self) -> u64 {
        self.covering_height
    }
    #[must_use]
    pub const fn block_hash(&self) -> [u8; 32] {
        self.block_hash
    }
    #[must_use]
    pub const fn validator_set_hash(&self) -> [u8; 32] {
        self.validator_set_hash
    }
    #[must_use]
    pub const fn state_root(&self) -> [u8; 32] {
        self.state_root
    }
}
