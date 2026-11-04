use layerx_agentd::identity::{CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority};
use layerx_agentd::read::{checkpoint, CommitValidator, CommittedReads};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::signer::{SignError, Signer};
use layerx_sdk::rpc::{RpcClient, RpcError};
use layerx_types::{ids::Did, verify::VerificationLevel};
use serde_json::Value;

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn digest<const N: usize>(text: &str) -> Result<[u8; N], IdentityError> {
    if text.len() != N * 2 || !text.is_ascii() {
        return Err(IdentityError::Unverified);
    }
    let mut bytes = [0; N];
    for (index, byte) in bytes.iter_mut().enumerate() {
        *byte = u8::from_str_radix(&text[index * 2..index * 2 + 2], 16)
            .map_err(|_| IdentityError::Unverified)?;
    }
    Ok(bytes)
}

pub(crate) fn pay2_signer_nonce(rpc: &RpcClient, signer: &dyn Signer) -> Result<u64, RpcError> {
    let sender = signer
        .evm_address()
        .ok_or(RpcError::Signing(SignError::EvmKeyUnavailable))?;
    rpc.get_identity_sequence(&format!("0x{}", hex(&sender)))
}

pub(crate) struct Pay2CommittedAuthority {
    pub(crate) node: SequencerEndpoint,
    pub(crate) chain_id: String,
    pub(crate) validators: Vec<CommitValidator>,
}

impl Pay2CommittedAuthority {
    pub(crate) fn from_environment() -> Result<Self, IdentityError> {
        let path = std::env::var_os("PAY2_COMMITTED_AUTHORITY")
            .ok_or(IdentityError::BoundaryUnavailable)?;
        let bytes = std::fs::read(path).map_err(|_| IdentityError::BoundaryUnavailable)?;
        let config: Value =
            serde_json::from_slice(&bytes).map_err(|_| IdentityError::Unverified)?;
        let text = |key: &str| config[key].as_str().ok_or(IdentityError::Unverified);
        let chain_id = text("chain_id")?.to_owned();
        if chain_id.parse::<u64>().ok().filter(|id| *id > 0).is_none() {
            return Err(IdentityError::Unverified);
        }
        let ca = std::fs::read(text("ca_der")?).map_err(|_| IdentityError::BoundaryUnavailable)?;
        let node = SequencerEndpoint::new(text("rpc_url")?, &[&ca])
            .map_err(|_| IdentityError::BoundaryUnavailable)?;
        if node
            .chain_id()
            .map_err(|_| IdentityError::BoundaryUnavailable)?
            .to_string()
            != chain_id
        {
            return Err(IdentityError::Unverified);
        }
        let validators = config["validators"]
            .as_array()
            .ok_or(IdentityError::Unverified)?
            .iter()
            .map(|entry| {
                let public_key = digest(
                    entry["public_key"]
                        .as_str()
                        .ok_or(IdentityError::Unverified)?,
                )?;
                Ok(CommitValidator {
                    address: CommitValidator::address_for_key(&public_key),
                    public_key,
                    voting_power: entry["voting_power"]
                        .as_u64()
                        .filter(|power| *power > 0)
                        .ok_or(IdentityError::Unverified)?,
                })
            })
            .collect::<Result<Vec<_>, IdentityError>>()?;
        if validators.is_empty() {
            return Err(IdentityError::Unverified);
        }
        Ok(Self {
            node,
            chain_id,
            validators,
        })
    }
}

pub(crate) struct Pay2GatewayIdentity<'a> {
    rpc: &'a RpcClient,
    authority: Pay2CommittedAuthority,
    capability: [u8; 32],
}

impl<'a> Pay2GatewayIdentity<'a> {
    pub(crate) fn new(
        rpc: &'a RpcClient,
        authority: Pay2CommittedAuthority,
        capability: [u8; 32],
    ) -> Self {
        Self {
            rpc,
            authority,
            capability,
        }
    }

    pub(crate) fn from_environment(
        rpc: &'a RpcClient,
        capability: [u8; 32],
    ) -> Result<Self, IdentityError> {
        Ok(Self {
            rpc,
            authority: Pay2CommittedAuthority::from_environment()?,
            capability,
        })
    }
}

impl IdentityResolver for Pay2GatewayIdentity<'_> {
    fn resolve(&mut self, did: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        let text = std::str::from_utf8(did.as_bytes()).map_err(|_| IdentityError::Unverified)?;
        let public_key = digest(
            text.strip_prefix("did:layerx:")
                .ok_or(IdentityError::Unverified)?,
        )?;
        let Some(sender) = self
            .authority
            .node
            .bound_address(&public_key)
            .map_err(|_| IdentityError::BoundaryUnavailable)?
        else {
            return Ok(None);
        };
        let nonce: u64 = self
            .rpc
            .get_identity_sequence(&format!("0x{}", hex(&sender)))
            .map_err(|_| IdentityError::BoundaryUnavailable)?;
        let height = self
            .authority
            .node
            .block_number()
            .map_err(|_| IdentityError::BoundaryUnavailable)?;
        let reads = CommittedReads::open(&self.authority.node, height)
            .map_err(|_| IdentityError::BoundaryUnavailable)?;
        let observed = reads.checkpoint();
        let authenticated = checkpoint(
            height,
            &self.authority.chain_id,
            &observed.header,
            &observed.commit,
            &self.authority.validators,
        )
        .map_err(|_| IdentityError::Unverified)?;
        if authenticated != *observed || authenticated.app_hash == [0; 32] {
            return Err(IdentityError::Unverified);
        }
        let mut canonical = authenticated.app_hash.to_vec();
        canonical.extend_from_slice(&authenticated.height.to_be_bytes());
        canonical.extend_from_slice(&nonce.to_be_bytes());
        canonical.extend_from_slice(&sender);
        canonical.extend_from_slice(&authenticated.block_hash);
        canonical.extend_from_slice(&public_key);
        Ok(Some(CoreIdentity {
            canonical_bytes: canonical,
            head_sequence: authenticated.height,
            revocation_sequence: 1,
            verification_level: VerificationLevel::CHECKPOINT_FINALISED,
            frozen: false,
            authorities: vec![ProtocolAuthority::CapabilityGrant(self.capability)],
        }))
    }
}
