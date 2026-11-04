use layerx_crypto::authority_grant::{AuthorityGrant, NativeFeeBudget};
use layerx_crypto::disclosure::DisclosureError;
use layerx_crypto::evm_transaction::decode_unsigned;

mod support;
use layerx_crypto::{ed25519, SignatureMessage};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_wire::{activity, hash};
use sha2::{Digest as _, Sha256};

struct Fixture {
    grant: &'static [u8],
    id: &'static [u8; 32],
    activity: &'static [u8],
    activity_id: &'static [u8; 32],
}

macro_rules! fixture {
    ($name:literal) => {
        Fixture {
            grant: include_bytes!(concat!(
                "../../../../../tests/fixtures/authority/native-fee-grants/",
                $name,
                "/grant.bin"
            )),
            id: include_bytes!(concat!(
                "../../../../../tests/fixtures/authority/native-fee-grants/",
                $name,
                "/grant.id"
            )),
            activity: include_bytes!(concat!(
                "../../../../../tests/fixtures/authority/native-fee-grants/",
                $name,
                "/grant.activity"
            )),
            activity_id: include_bytes!(concat!(
                "../../../../../tests/fixtures/authority/native-fee-grants/",
                $name,
                "/grant.activity-id"
            )),
        }
    };
}

const FIXTURES: &[Fixture] = &[
    fixture!("capability"),
    fixture!("budget"),
    fixture!("asset-mismatch"),
    fixture!("total-bound"),
    fixture!("period-bound"),
];

fn registry() -> ModuleRegistry {
    let kind = ActivityType::new(ModuleId::Governance, 8)
        .unwrap_or_else(|error| panic!("activity: {error:?}"));
    let module = ModuleRegistration::new(ModuleId::Governance, &[kind])
        .unwrap_or_else(|error| panic!("module: {error:?}"));
    ModuleRegistry::new(&[module]).unwrap_or_else(|error| panic!("registry: {error:?}"))
}

#[test]
fn original_native_fee_grants_preserve_owner_signatures_and_native_ids() {
    for fixture in FIXTURES {
        let grant = AuthorityGrant::decode(fixture.grant)
            .unwrap_or_else(|error| panic!("grant: {error:?}"));
        assert_eq!(grant.encode(), Ok(fixture.grant.to_vec()));
        assert_eq!(fixture.grant[4], 2);
        let mut hasher = Sha256::new();
        hasher.update(hash::Domain::AuthorityHash.tag());
        hasher.update(fixture.grant);
        assert_eq!(hasher.finalize().as_slice(), fixture.id);
        let submitted = activity::decode_signed(fixture.activity, &registry())
            .unwrap_or_else(|error| panic!("activity: {error:?}"));
        assert_eq!(hash::activity_id(&submitted), Ok(*fixture.activity_id));
        assert_eq!(grant.payload(), Ok(submitted.payload().to_vec()));
        let unsigned = activity::encode_unsigned(&submitted)
            .unwrap_or_else(|error| panic!("unsigned: {error:?}"));
        let key: &[u8; 32] = submitted
            .authority()
            .try_into()
            .unwrap_or_else(|error| panic!("key: {error:?}"));
        let signature: &[u8; 64] = submitted
            .signature()
            .unwrap_or_else(|| panic!("missing signature"))
            .try_into()
            .unwrap_or_else(|error| panic!("signature: {error:?}"));
        let message = SignatureMessage::new(
            hash::Domain::SignaturePreimage,
            submitted.protocol_version(),
            submitted.network_id(),
            &unsigned,
        )
        .unwrap_or_else(|error| panic!("message: {error:?}"));
        assert_eq!(ed25519::verify(key, signature, message), Ok(()));
        let disclosed = AuthorityGrant::from_payload(submitted.payload())
            .unwrap_or_else(|error| panic!("kernel grant payload: {error:?}"));
        assert_eq!(disclosed, grant);
        assert_eq!(
            disclosed.payload().expect("canonical payload"),
            submitted.payload()
        );
        assert_eq!(
            activity::encode_unsigned(&submitted).expect("archive reencode"),
            unsigned
        );
        assert!(
            decode_unsigned(&unsigned).is_err(),
            "archive cannot enter the current transaction path"
        );
    }
}

#[test]
fn fee_scope_wire_requires_zero_initial_counters_and_canonical_bounds() {
    for fixture in FIXTURES {
        for length in 0..fixture.grant.len() {
            assert!(AuthorityGrant::decode(&fixture.grant[..length]).is_err());
        }
        let mut extra = fixture.grant.to_vec();
        extra.push(0);
        assert!(AuthorityGrant::decode(&extra).is_err());
        for version in 0..=u8::MAX {
            if version == 2 {
                continue;
            }
            let mut changed = fixture.grant.to_vec();
            changed[4] = version;
            assert!(AuthorityGrant::decode(&changed).is_err());
        }
        let base = fixture.grant.len() - 132;
        for offset in (68..84).chain(108..124) {
            let mut changed = fixture.grant.to_vec();
            changed[base + offset] = 1;
            assert!(AuthorityGrant::decode(&changed).is_err());
        }
    }
    let mutations: &[fn(&mut NativeFeeBudget)] = &[
        |fee| fee.asset = [0; 32],
        |fee| fee.maximum_per_activity = 0,
        |fee| fee.maximum_total = 0,
        |fee| fee.maximum_per_activity = fee.maximum_total + 1,
        |fee| fee.period_start += 1,
        |fee| fee.maximum_per_period = 1,
        |fee| fee.period_length = 0,
    ];
    for mutate in mutations {
        let mut grant = AuthorityGrant::decode(FIXTURES[4].grant)
            .unwrap_or_else(|error| panic!("grant: {error:?}"));
        let Some(fee) = grant.fee_budget.as_mut() else {
            panic!("missing fee")
        };
        mutate(fee);
        assert!(grant.encode().is_err());
    }
}

#[test]
fn every_fee_field_is_bound_into_the_signing_disclosure() {
    let fixture = &FIXTURES[4];
    let submitted = activity::decode_signed(fixture.activity, &registry())
        .unwrap_or_else(|error| panic!("activity: {error:?}"));
    let unsigned =
        activity::encode_unsigned(&submitted).unwrap_or_else(|error| panic!("unsigned: {error:?}"));
    let original = AuthorityGrant::from_payload(submitted.payload()).expect("kernel grant");
    assert_eq!(original.encode().expect("original grant"), fixture.grant);
    assert!(decode_unsigned(&unsigned).is_err());
    let disclosure = support::send_call(25, 7).disclosure;
    let mutations: &[fn(&mut NativeFeeBudget)] = &[
        |fee| fee.asset[0] ^= 1,
        |fee| fee.maximum_per_activity += 1,
        |fee| fee.maximum_total += 1,
        |fee| fee.period_length += 1,
        |fee| fee.maximum_per_period += 1,
        |fee| fee.period_start += 1,
    ];
    for mutate in mutations {
        let mut changed = disclosure.clone();
        let mut grant = original;
        let Some(fee) = grant.fee_budget.as_mut() else {
            panic!("missing fee")
        };
        mutate(fee);
        if let Ok(encoded) = grant.encode() {
            assert_ne!(encoded, fixture.grant);
            let mut hash = Sha256::new();
            hash.update(hash::Domain::AuthorityHash.tag());
            hash.update(&encoded);
            assert_ne!(hash.finalize().as_slice(), fixture.id);
            let altered_payload = grant.payload().expect("altered payload");
            let mut altered = unsigned.clone();
            let payload_start = altered.len() - submitted.payload().len();
            assert_eq!(&altered[payload_start..], submitted.payload());
            assert_eq!(altered_payload.len(), submitted.payload().len());
            altered[payload_start..].copy_from_slice(&altered_payload);
            let original_hash = submitted.payload_hash();
            let hash_start = altered[..payload_start]
                .windows(32)
                .position(|bytes| bytes == original_hash)
                .expect("recorded payload commitment");
            let mut payload_hash = Sha256::new();
            payload_hash.update(hash::Domain::PayloadHash.tag());
            payload_hash.update(&altered_payload);
            altered[hash_start..hash_start + 32].copy_from_slice(&payload_hash.finalize());
            let decoded =
                activity::decode_unsigned(&altered, &registry()).expect("changed archive framing");
            assert_eq!(
                hash::payload_hash(&decoded).expect("changed payload hash"),
                decoded.payload_hash()
            );
            let key: &[u8; 32] = submitted.authority().try_into().expect("owner key");
            let signature: &[u8; 64] = submitted
                .signature()
                .expect("owner signature")
                .try_into()
                .expect("signature width");
            let message = SignatureMessage::new(
                hash::Domain::SignaturePreimage,
                submitted.protocol_version(),
                submitted.network_id(),
                &altered,
            )
            .expect("altered message");
            assert!(ed25519::verify(key, signature, message).is_err());
        }
        changed.authority_grant = Some(grant);
        assert_eq!(
            changed.reencode(),
            Err(DisclosureError::FieldMismatch("authority_grant"))
        );
        assert!(changed.audit_digest().is_err());
    }
    let mut removed = disclosure;
    let mut grant = original;
    grant.fee_budget = None;
    assert_ne!(grant.encode().expect("no fee budget"), fixture.grant);
    removed.authority_grant = Some(grant);
    assert_eq!(
        removed.reencode(),
        Err(DisclosureError::FieldMismatch("authority_grant"))
    );
}
