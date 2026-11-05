//! Canonical Paxeer DID-to-EVM binding consent and its registration transaction.

use crate::ed25519;
use crate::evm_transaction::Eip1559Call;
use crate::send::CallOptions;
use crate::signer::SignError;

pub const BIND_DOMAIN: &[u8; 17] = b"LX:PAXEER-BIND:v1";
pub const BIND_MESSAGE_BYTES: usize = 77;
pub const BIND_SELECTOR: [u8; 4] = [0xdd, 0x9a, 0xa6, 0x28];
pub const ADDR_PRECOMPILE: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x04,
];

/// Immutable raw Ed25519 consent: domain, uint256 chain, address, bind nonce.
/// The bind nonce comes from `layerXBindNonce(address)`, independently of the
/// EVM transaction nonce.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct BindMessage([u8; BIND_MESSAGE_BYTES]);

impl BindMessage {
    #[must_use]
    pub fn new(chain_id: [u8; 32], address: [u8; 20], nonce: u64) -> Self {
        let mut bytes = [0; BIND_MESSAGE_BYTES];
        bytes[..17].copy_from_slice(BIND_DOMAIN);
        bytes[17..49].copy_from_slice(&chain_id);
        bytes[49..69].copy_from_slice(&address);
        bytes[69..].copy_from_slice(&nonce.to_be_bytes());
        Self(bytes)
    }

    /// Parses only the complete current bind domain, never an activity preimage.
    ///
    /// # Errors
    /// Refuses a different domain, truncation or trailing bytes.
    pub fn decode(bytes: &[u8]) -> Result<Self, SignError> {
        if bytes.len() != BIND_MESSAGE_BYTES || !bytes.starts_with(BIND_DOMAIN) {
            return Err(SignError::InvalidDisclosure);
        }
        let mut canonical = [0; BIND_MESSAGE_BYTES];
        canonical.copy_from_slice(bytes);
        Ok(Self(canonical))
    }

    #[must_use]
    pub const fn as_bytes(&self) -> &[u8; BIND_MESSAGE_BYTES] {
        &self.0
    }

    /// # Errors
    /// Refuses a weak/noncanonical key or a signature over any other consent.
    pub fn verify(&self, public_key: &[u8; 32], signature: &[u8; 64]) -> Result<(), SignError> {
        ed25519::verify_message(public_key, signature, self.as_bytes())
            .map_err(|_| SignError::ReturnedSignatureInvalid)
    }
}

/// Both consents travel in one transaction: the DID signature is calldata and
/// the EVM signature recovers the address named by the DID consent.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BindAuthorization {
    pub message: BindMessage,
    pub public_key: [u8; 32],
    pub signature: [u8; 64],
}

impl BindAuthorization {
    /// Produces the actual addr precompile registration call.
    ///
    /// # Errors
    /// Refuses invalid DID consent or options for a different chain/sender.
    pub fn to_call(&self, options: &CallOptions) -> Result<Eip1559Call, SignError> {
        self.message.verify(&self.public_key, &self.signature)?;
        let mut chain = [0; 32];
        chain[24..].copy_from_slice(&options.chain_id.to_be_bytes());
        if self.message.0[17..49] != chain || self.message.0[49..69] != options.from {
            return Err(SignError::DisclosureMismatch("bind_scope"));
        }
        let mut data = BIND_SELECTOR.to_vec();
        data.extend_from_slice(&self.public_key);
        let mut word = [0; 32];
        word[31] = 64;
        data.extend_from_slice(&word);
        data.extend_from_slice(&word);
        data.extend_from_slice(&self.signature);
        Ok(Eip1559Call {
            chain_id: options.chain_id,
            nonce: options.nonce,
            max_priority_fee_per_gas: options.max_priority_fee_per_gas,
            max_fee_per_gas: options.max_fee_per_gas,
            gas_limit: options.gas_limit,
            to: ADDR_PRECOMPILE,
            data,
        })
    }

    /// Consumes canonical registration calldata with the recovered EVM sender
    /// and the bind nonce read from chain state.
    ///
    /// # Errors
    /// Refuses a different target/selector, noncanonical ABI or invalid consent.
    pub fn from_call(
        call: &Eip1559Call,
        recovered_sender: [u8; 20],
        bind_nonce: u64,
    ) -> Result<Self, SignError> {
        let mut word = [0; 32];
        word[31] = 64;
        if call.to != ADDR_PRECOMPILE
            || call.data.len() != 164
            || call.data[..4] != BIND_SELECTOR
            || call.data[36..68] != word
            || call.data[68..100] != word
        {
            return Err(SignError::InvalidDisclosure);
        }
        let mut chain = [0; 32];
        chain[24..].copy_from_slice(&call.chain_id.to_be_bytes());
        let mut public_key = [0; 32];
        public_key.copy_from_slice(&call.data[4..36]);
        let mut signature = [0; 64];
        signature.copy_from_slice(&call.data[100..]);
        let authorization = Self {
            message: BindMessage::new(chain, recovered_sender, bind_nonce),
            public_key,
            signature,
        };
        authorization.message.verify(&public_key, &signature)?;
        Ok(authorization)
    }
}
