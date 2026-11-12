use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine as _;
use layerx_ap2::{
    Ap2Error, Ap2MandatePair, KeyResolver, KeyUse, ProtectedHeader, VerificationContext,
};
use layerx_interop_gateway::adapter::{AdapterId, ConformanceSuite};
use p256::ecdsa::signature::Signer as _;
use p256::ecdsa::{Signature, SigningKey, VerifyingKey};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

pub const NOW: u64 = 1_700_000_000;
const DIRECT_VECTOR: &[u8] =
    include_bytes!("../../../../specs/conformance/ap2/direct/001-minimal-valid.json");

// Committed conformance peers: real ES256 signatures and purpose-scoped key pins.
// The component under test remains Ap2ExternalMandateVerifier and its JOSE verifier.
pub struct MandateKeys {
    pub pins: Vec<(KeyUse, &'static str, VerifyingKey)>,
}

impl MandateKeys {
    pub fn pinned() -> Self {
        Self {
            pins: vec![
                (
                    KeyUse::CheckoutMandateIssuer,
                    "issuer-001",
                    *key(7).verifying_key(),
                ),
                (
                    KeyUse::PaymentMandateIssuer,
                    "issuer-001",
                    *key(7).verifying_key(),
                ),
                (
                    KeyUse::MerchantCheckout,
                    "merchant-001",
                    *key(11).verifying_key(),
                ),
            ],
        }
    }
}

impl KeyResolver for MandateKeys {
    fn resolve(&self, usage: KeyUse, header: &ProtectedHeader) -> Result<VerifyingKey, Ap2Error> {
        if header.certificate_chain().is_some() {
            return Err(Ap2Error::KeyResolution);
        }
        self.pins
            .iter()
            .find(|(purpose, kid, _)| *purpose == usage && Some(*kid) == header.key_id())
            .map(|(_, _, key)| *key)
            .ok_or(Ap2Error::KeyResolution)
    }
}

fn key(scalar: u8) -> SigningKey {
    SigningKey::from_bytes((&[scalar; 32]).into())
        .unwrap_or_else(|error| panic!("conformance scalar: {error:?}"))
}

fn sign(payload: &Value, kid: &str, token_type: &str, scalar: u8) -> String {
    let header = URL_SAFE_NO_PAD.encode(
        serde_json::to_vec(&json!({"alg":"ES256", "typ":token_type, "kid":kid}))
            .unwrap_or_else(|error| panic!("JWS header: {error}")),
    );
    let body = URL_SAFE_NO_PAD
        .encode(serde_json::to_vec(payload).unwrap_or_else(|error| panic!("JWS payload: {error}")));
    let input = format!("{header}.{body}");
    let signature: Signature = key(scalar).sign(input.as_bytes());
    format!("{input}.{}", URL_SAFE_NO_PAD.encode(signature.to_bytes()))
}

fn root(mandate: &Value) -> String {
    format!(
        "{}~",
        sign(
            &json!({"iat":NOW-60, "exp":NOW+3600, "delegate_payload":[mandate]}),
            "issuer-001",
            "dc+sd-jwt",
            7,
        )
    )
}

pub struct MandateFixture {
    pub checkout: String,
    pub payment: String,
    pub transaction_id: String,
    pub expected: Value,
}

impl MandateFixture {
    pub fn direct(tamper_merchant: bool) -> Self {
        let vector: Value = serde_json::from_slice(DIRECT_VECTOR)
            .unwrap_or_else(|error| panic!("committed AP2 vector: {error}"));
        assert_eq!(vector["generator"], "mandates::direct_pair");
        assert_eq!(vector["issuer_key_scalar"], 7);
        assert_eq!(vector["merchant_key_scalar"], 11);
        let expected = vector["expected_values"].clone();
        let amount = expected["amount_minor_units"]
            .as_u64()
            .unwrap_or_else(|| panic!("conformance amount"));
        let mut merchant = sign(
            &json!({
                "id":expected["checkout_id"],
                "merchant":{"id":expected["payee_id"], "name":expected["payee_name"]},
                "line_items":[{"id":"line-001", "item":{"id":"sku-001", "title":"LayerX node credit", "price":amount}, "quantity":1, "totals":[{"type":"total", "amount":amount}]}],
                "status":"completed", "currency":expected["currency"],
                "totals":[{"type":"total", "amount":amount}], "links":[]
            }),
            "merchant-001",
            "JWT",
            11,
        );
        if tamper_merchant {
            merchant = corrupt_signature(&merchant);
        }
        let transaction_id = URL_SAFE_NO_PAD.encode(Sha256::digest(merchant.as_bytes()));
        let checkout = root(&json!({
            "vct":"mandate.checkout.1", "checkout_jwt":merchant,
            "checkout_hash":transaction_id, "iat":NOW-30, "exp":NOW+1800
        }));
        let payment = root(&json!({
            "vct":"mandate.payment.1", "transaction_id":transaction_id,
            "payee":{"id":expected["payee_id"], "name":expected["payee_name"]},
            "payment_amount":{"amount":amount, "currency":expected["currency"]},
            "payment_instrument":{"id":"card-001", "type":"card"},
            "iat":NOW-30, "exp":NOW+1800
        }));
        Self {
            checkout,
            payment,
            transaction_id,
            expected,
        }
    }

    pub fn payload(&self) -> Vec<u8> {
        Ap2MandatePair::new(&self.checkout, &self.payment)
            .to_json()
            .unwrap_or_else(|error| panic!("production AP2 envelope: {error:?}"))
    }
}

pub fn corrupt_signature(token: &str) -> String {
    let (input, signature) = token
        .rsplit_once('.')
        .unwrap_or_else(|| panic!("signed JWS"));
    let (signature, suffix) = signature
        .strip_suffix('~')
        .map_or((signature, ""), |signature| (signature, "~"));
    let mut bytes = URL_SAFE_NO_PAD
        .decode(signature)
        .unwrap_or_else(|error| panic!("signature encoding: {error}"));
    assert_eq!(bytes.len(), 64);
    bytes[31] ^= 1;
    format!("{input}.{}{suffix}", URL_SAFE_NO_PAD.encode(bytes))
}

pub fn context() -> VerificationContext<'static> {
    VerificationContext {
        now: NOW,
        clock_skew_seconds: 300,
        expected_audience: "test-merchant-api",
        expected_nonce: "nonce-abc123xyz",
        currency_minor_exponent: 2,
        usage: None,
    }
}

pub fn suite() -> ConformanceSuite {
    ConformanceSuite::new(
        AdapterId::new("ap2-portable-direct-vector")
            .unwrap_or_else(|error| panic!("suite id: {error:?}")),
        1,
        Sha256::digest(DIRECT_VECTOR).into(),
    )
    .unwrap_or_else(|error| panic!("committed conformance suite: {error:?}"))
}
