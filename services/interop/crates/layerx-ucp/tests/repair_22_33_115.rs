#[path = "support/producer_checkout.rs"]
mod producer_checkout;

#[path = "sequenced_vectors.rs"]
mod sequenced_vectors;

#[test]
fn ucp_conformance_producer() {
    producer_checkout::producer_checkout("ucp-conformance-producer");
}

#[test]
fn ucp_e2e_durability() {
    producer_checkout::producer_checkout("ucp-e2e-durability");
}

#[test]
fn ucp_merchant_producer() {
    producer_checkout::producer_checkout("ucp-merchant-producer");
}

#[test]
fn ucp_checkout_producer() {
    producer_checkout::producer_checkout("ucp-checkout-producer");
}

#[test]
fn ucp_native_payment_encoding_carries_checkout_key() {
    use layerx_ucp::{
        CheckoutSubmission, MerchantCurrencyBinding, MerchantProfile, NegotiatedCapabilities,
        PaymentHandler, PlatformProfile, UcpError, UcpIdempotencyKey, UcpServiceConfig,
    };

    let handler = PaymentHandler::new(
        "layerx-402",
        "2026-04-08",
        "https://layerx.dev/2026-04-08/specification/402",
        "https://layerx.dev/2026-04-08/schemas/402.json",
    )
    .unwrap();
    let merchant =
        MerchantProfile::layerx("https://merchant.example/ucp", handler.clone()).unwrap();
    let platform = PlatformProfile {
        profile_url: "https://buyer.example/profile".into(),
        capabilities: merchant.capabilities().to_vec(),
        payment_handlers: vec![handler.clone()],
    };
    let mut checkout = CheckoutSubmission {
        checkout_id: "encoded-native-checkout".into(),
        currency: *b"USD",
        total_minor: 100,
        layerx_asset: [0x31; 32],
        layerx_recipient: [0x32; 32],
        idempotency_key: UcpIdempotencyKey::parse("123e4567-e89b-12d3-a456-426614174000").unwrap(),
        negotiated: NegotiatedCapabilities::negotiate(&platform, &handler).unwrap(),
    };
    let config = UcpServiceConfig::new(
        MerchantCurrencyBinding::new(*b"USD", checkout.layerx_asset, 2, 6).unwrap(),
    );
    let intent = checkout.payment_intent(&config).unwrap();
    assert_eq!(intent.amount, 1_000_000);
    assert_eq!(
        intent.idempotency_key,
        checkout.idempotency_key.gateway_key()
    );
    let payer = [0x33; 32];
    let expires_at = 1_750_000_000_123_u64;
    let calldata = intent.encode_pay_exact(payer, expires_at).unwrap();
    assert_eq!(calldata.len(), 196);
    assert_eq!(&calldata[..4], &[0x78, 0xea, 0x11, 0xdb]);
    assert_eq!(&calldata[4..36], &payer);
    assert_eq!(&calldata[36..68], &checkout.layerx_recipient);
    assert_eq!(&calldata[68..100], &checkout.layerx_asset);
    assert_eq!(&calldata[100..116], &[0; 16]);
    assert_eq!(&calldata[116..132], &1_000_000_u128.to_be_bytes());
    assert_eq!(&calldata[132..156], &[0; 24]);
    assert_eq!(&calldata[156..164], &expires_at.to_be_bytes());
    assert_eq!(&calldata[164..], &checkout.idempotency_key.gateway_key());
    assert_eq!(
        intent.encode_pay_exact([0; 32], expires_at),
        Err(UcpError::InvalidCheckout)
    );
    assert_eq!(
        intent.encode_pay_exact(checkout.layerx_recipient, expires_at),
        Err(UcpError::InvalidCheckout)
    );
    assert_eq!(
        intent.encode_pay_exact(payer, 0),
        Err(UcpError::InvalidCheckout)
    );
    checkout.idempotency_key =
        UcpIdempotencyKey::parse("123e4567-e89b-12d3-a456-426614174001").unwrap();
    let changed = checkout
        .payment_intent(&config)
        .unwrap()
        .encode_pay_exact(payer, expires_at)
        .unwrap();
    assert_eq!(&changed[..164], &calldata[..164]);
    assert_ne!(&changed[164..], &calldata[164..]);
    assert_eq!(
        checkout.payment_intent(&UcpServiceConfig::default()),
        Err(UcpError::CurrencyBindingRequired)
    );
}

#[test]
fn ucp_pending_transcript_survives_reopen_without_resubmission() {
    producer_checkout::durable_submission_recovery();
}

#[test]
fn ucp_native_codec_matches_unchanged_producer() {
    producer_checkout::native_codec_matches_producer();
}

#[test]
fn ucp_unrelated_producer_reference_is_refused() {
    producer_checkout::unrelated_producer_reference_is_refused();
}
