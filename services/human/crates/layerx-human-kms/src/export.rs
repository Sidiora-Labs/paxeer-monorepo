//! Recipient-encrypted, replayable primary-key handover.
use k256::elliptic_curve::sec1::ToEncodedPoint;
use ring::aead::{Aad, LessSafeKey, Nonce, UnboundKey, AES_256_GCM};
use ring::hkdf;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use zeroize::{Zeroize, Zeroizing};

#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ExportCeremony {
    pub id: [u8; 32],
    pub recipient: Vec<u8>,
}
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ExportEnvelope {
    pub ceremony: ExportCeremony,
    pub ephemeral: Vec<u8>,
    pub nonce: [u8; 12],
    pub ciphertext: Vec<u8>,
}
#[derive(Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ExportAcknowledgement {
    pub ceremony: ExportCeremony,
    pub proof: [u8; 32],
}
impl Drop for ExportAcknowledgement {
    fn drop(&mut self) {
        self.proof.zeroize();
    }
}
/// Keep this secret at the ceremony recipient until recovery is complete.
pub struct ExportRecipient(Zeroizing<[u8; 32]>);
impl ExportRecipient {
    ///
    /// # Errors
    /// Returns an error if the recipient secret is not a valid scalar.
    pub fn new(secret: [u8; 32]) -> Result<Self, String> {
        let secret = Zeroizing::new(secret);
        k256::SecretKey::from_slice(&*secret).map_err(|_| "export recipient scalar invalid")?;
        Ok(Self(secret))
    }
    ///
    /// # Errors
    /// Returns an error if the recipient key or ceremony is invalid.
    pub fn ceremony(&self, id: [u8; 32]) -> Result<ExportCeremony, String> {
        let key = k256::SecretKey::from_slice(&*self.0).map_err(|_| "recipient invalid")?;
        let ceremony = ExportCeremony {
            id,
            recipient: key.public_key().to_encoded_point(true).as_bytes().to_vec(),
        };
        ceremony.validate()?;
        Ok(ceremony)
    }
    ///
    /// # Errors
    /// Returns an error if the envelope does not authenticate for this ceremony and context.
    pub fn open(
        &self,
        provider: &str,
        binding: [u8; 32],
        reference: &[u8],
        envelope: &ExportEnvelope,
    ) -> Result<(Zeroizing<[u8; 32]>, ExportAcknowledgement), String> {
        if self.ceremony(envelope.ceremony.id)? != envelope.ceremony {
            return Err("export recipient mismatch".into());
        }
        let context = context(provider, binding, reference, &envelope.ceremony)?;
        let key = encryption_key(&self.0, &envelope.ephemeral, &context)?;
        let mut bytes = Zeroizing::new(envelope.ciphertext.clone());
        let plain = key
            .open_in_place(
                Nonce::assume_unique_for_key(envelope.nonce),
                Aad::from(&context),
                &mut bytes,
            )
            .map_err(|_| "export envelope authentication failed")?;
        if plain.len() != 64 {
            return Err("export envelope length invalid".into());
        }
        let seed = Zeroizing::new(plain[..32].try_into().map_err(|_| "export seed invalid")?);
        let proof = plain[32..].try_into().map_err(|_| "export proof invalid")?;
        Ok((
            seed,
            ExportAcknowledgement {
                ceremony: envelope.ceremony.clone(),
                proof,
            },
        ))
    }
}
impl ExportCeremony {
    ///
    /// # Errors
    /// Returns an error if the ceremony identifier or recipient key is invalid.
    pub fn validate(&self) -> Result<(), String> {
        if self.id == [0; 32] || self.recipient.len() != 33 {
            return Err("export ceremony invalid".into());
        }
        k256::PublicKey::from_sec1_bytes(&self.recipient)
            .map_err(|_| "export public key invalid")?;
        Ok(())
    }
}
fn context(
    provider: &str,
    binding: [u8; 32],
    reference: &[u8],
    ceremony: &ExportCeremony,
) -> Result<Vec<u8>, String> {
    ceremony.validate()?;
    serde_json::to_vec(&("LXKP/export/v8", provider, binding, reference, ceremony))
        .map_err(|_| "export context invalid".into())
}
fn encryption_key(secret: &[u8; 32], peer: &[u8], context: &[u8]) -> Result<LessSafeKey, String> {
    let secret = k256::SecretKey::from_slice(secret).map_err(|_| "export scalar invalid")?;
    let peer = k256::PublicKey::from_sec1_bytes(peer).map_err(|_| "export peer invalid")?;
    let mut shared_point =
        k256::ProjectivePoint::from(*peer.as_affine()) * secret.to_nonzero_scalar().as_ref();
    let shared = Zeroizing::new(
        shared_point
            .to_affine()
            .to_encoded_point(true)
            .as_bytes()
            .to_vec(),
    );
    shared_point.zeroize();
    let salt = hkdf::Salt::new(hkdf::HKDF_SHA256, context);
    let prk = salt.extract(&shared);
    let info = [b"LXKP/export/key".as_slice()];
    let okm = prk
        .expand(&info, &AES_256_GCM)
        .map_err(|_| "export derivation failed")?;
    let mut key = Zeroizing::new([0; 32]);
    okm.fill(&mut *key)
        .map_err(|_| "export derivation failed")?;
    Ok(LessSafeKey::new(
        UnboundKey::new(&AES_256_GCM, &*key).map_err(|_| "export key invalid")?,
    ))
}
pub(crate) fn seal(
    provider: &str,
    binding: [u8; 32],
    reference: &[u8],
    ceremony: ExportCeremony,
    seed: &[u8; 32],
) -> Result<(ExportEnvelope, [u8; 32]), String> {
    let context = context(provider, binding, reference, &ceremony)?;
    let secret = loop {
        let mut bytes = Zeroizing::new([0; 32]);
        getrandom::fill(&mut *bytes).map_err(|_| "export entropy unavailable")?;
        if k256::SecretKey::from_slice(&*bytes).is_ok() {
            break bytes;
        }
    };
    let ephemeral = k256::SecretKey::from_slice(&*secret)
        .map_err(|_| "export scalar invalid")?
        .public_key()
        .to_encoded_point(true)
        .as_bytes()
        .to_vec();
    let key = encryption_key(&secret, &ceremony.recipient, &context)?;
    let mut proof = Zeroizing::new([0; 32]);
    let mut nonce = [0; 12];
    getrandom::fill(&mut *proof).map_err(|_| "export entropy unavailable")?;
    getrandom::fill(&mut nonce).map_err(|_| "export entropy unavailable")?;
    let proof_hash = Sha256::digest(*proof).into();
    let mut ciphertext = Zeroizing::new(seed.to_vec());
    ciphertext.extend_from_slice(&*proof);
    key.seal_in_place_append_tag(
        Nonce::assume_unique_for_key(nonce),
        Aad::from(&context),
        &mut *ciphertext,
    )
    .map_err(|_| "export encryption failed")?;
    Ok((
        ExportEnvelope {
            ceremony,
            ephemeral,
            nonce,
            ciphertext: ciphertext.to_vec(),
        },
        proof_hash,
    ))
}
