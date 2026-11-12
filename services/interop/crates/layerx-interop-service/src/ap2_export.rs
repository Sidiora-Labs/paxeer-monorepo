use base64::{engine::general_purpose::URL_SAFE_NO_PAD, Engine as _};
use layerx_ap2::{
    Ap2Error, PortableLayerXEvidence, ReceiptSigner, SignedAp2Evidence, VerifiedMandates,
};
use layerx_proof::inclusion::SequencerAuthorization;
use p256::ecdsa::signature::{Signer as _, Verifier as _};
use p256::ecdsa::{Signature, SigningKey, VerifyingKey};
use serde::Deserialize;
use serde_json::{json, Value};
use zeroize::Zeroizing;

pub struct Ap2ReceiptSigners {
    payment: HostedReceiptSigner,
    checkout: HostedReceiptSigner,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct SignerFile {
    payment: SignerEntry,
    checkout: SignerEntry,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct SignerEntry {
    issuer: String,
    key_id: String,
    #[serde(deserialize_with = "private_key_hex")]
    private_key_hex: Zeroizing<String>,
}

fn private_key_hex<'de, D: serde::Deserializer<'de>>(
    deserializer: D,
) -> Result<Zeroizing<String>, D::Error> {
    String::deserialize(deserializer).map(Zeroizing::new)
}

struct HostedReceiptSigner {
    issuer: String,
    key_id: String,
    key: SigningKey,
}

impl ReceiptSigner for HostedReceiptSigner {
    fn issuer(&self) -> &str {
        &self.issuer
    }
    fn key_id(&self) -> &str {
        &self.key_id
    }
    fn verifying_key(&self) -> &VerifyingKey {
        self.key.verifying_key()
    }
    fn sign_es256(&mut self, signing_input: &[u8]) -> Result<[u8; 64], Ap2Error> {
        let signature: Signature = self.key.sign(signing_input);
        Ok(signature.to_bytes().into())
    }
}

impl Ap2ReceiptSigners {
    pub(crate) fn from_json(bytes: &[u8]) -> Result<Self, String> {
        fn signer(entry: SignerEntry) -> Result<HostedReceiptSigner, String> {
            if entry.issuer.is_empty()
                || entry.issuer.len() > 1024
                || entry.key_id.is_empty()
                || entry.key_id.len() > 512
                || entry
                    .issuer
                    .bytes()
                    .chain(entry.key_id.bytes())
                    .any(|byte| byte.is_ascii_control())
            {
                return Err("invalid AP2 receipt signer identity".to_owned());
            }
            let secret = Zeroizing::new(crate::config::parse_hex32(&entry.private_key_hex)?);
            let key = SigningKey::from_bytes((&*secret).into())
                .map_err(|_| "invalid AP2 receipt signing key".to_owned())?;
            Ok(HostedReceiptSigner {
                issuer: entry.issuer,
                key_id: entry.key_id,
                key,
            })
        }
        let file: SignerFile = serde_json::from_slice(bytes)
            .map_err(|_| "invalid AP2 receipt signer file".to_owned())?;
        Ok(Self {
            payment: signer(file.payment)?,
            checkout: signer(file.checkout)?,
        })
    }

    /// Hosted EVM execution has already passed the AP2 authorized-payment and
    /// settlement checks. Export through AP2's public signer/evidence types.
    pub(crate) fn export(
        &mut self,
        mandates: &VerifiedMandates,
        order_id: &str,
        receipt: &[u8],
        transaction: &[u8],
        receipt_keys: &[SequencerAuthorization],
        now_seconds: u64,
    ) -> Result<SignedAp2Evidence, Ap2Error> {
        let verified = layerx_proof::receipt::verify_sequencer_signature(receipt, receipt_keys)
            .map_err(|_| Ap2Error::EvidenceMismatch)?;
        let protocol = verified.protocol().ok_or(Ap2Error::EvidenceMismatch)?;
        if protocol.result_code() != 0 {
            return Err(Ap2Error::EvidenceMismatch);
        }
        let key = receipt_keys
            .iter()
            .find(|key| key.sequencer_id() == protocol.context_hash())
            .ok_or(Ap2Error::EvidenceMismatch)?;
        let height = u64::from_be_bytes(
            protocol.batch_id()[24..]
                .try_into()
                .map_err(|_| Ap2Error::EvidenceMismatch)?,
        );
        let layerx =
            PortableLayerXEvidence::export_with_transaction(receipt, key, height, transaction)
                .map_err(|_| Ap2Error::EvidenceMismatch)?;
        let receipt_digest = layerx
            .verify_v2(receipt_keys)
            .map_err(|_| Ap2Error::EvidenceMismatch)?
            .receipt_digest();
        let reference = format!("lxp:{}", URL_SAFE_NO_PAD.encode(receipt_digest));
        let payment = json!({"status":"Success", "iss":self.payment.issuer(), "iat":now_seconds,
            "reference":mandates.payment_receipt_reference(), "layerx_receipt_reference":reference, "layerx_evidence":layerx});
        let checkout = json!({"status":"Success", "iss":self.checkout.issuer(), "iat":now_seconds,
            "reference":mandates.checkout_receipt_reference(), "order_id":order_id,
            "layerx_receipt_reference":reference, "layerx_evidence":layerx});
        Ok(SignedAp2Evidence {
            payment_receipt_jwt: sign_receipt(&payment, &mut self.payment)?,
            checkout_receipt_jwt: sign_receipt(&checkout, &mut self.checkout)?,
            layerx,
            receipt_digest,
        })
    }
}

fn sign_receipt(payload: &Value, signer: &mut impl ReceiptSigner) -> Result<String, Ap2Error> {
    let header = json!({"alg":"ES256", "typ":"JWT", "kid":signer.key_id()});
    let input = format!(
        "{}.{}",
        URL_SAFE_NO_PAD.encode(serde_json::to_vec(&header).map_err(|_| Ap2Error::ReceiptSigning)?),
        URL_SAFE_NO_PAD.encode(serde_json::to_vec(payload).map_err(|_| Ap2Error::ReceiptSigning)?)
    );
    let signature = signer.sign_es256(input.as_bytes())?;
    let parsed = Signature::from_slice(&signature).map_err(|_| Ap2Error::ReceiptSigning)?;
    signer
        .verifying_key()
        .verify(input.as_bytes(), &parsed)
        .map_err(|_| Ap2Error::ReceiptSigning)?;
    Ok(format!("{input}.{}", URL_SAFE_NO_PAD.encode(signature)))
}
