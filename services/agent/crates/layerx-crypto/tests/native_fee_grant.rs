use crate::support;
use layerx_crypto::authority_grant::{authority_grant_call, AuthorityGrant, NativeFeeBudget};
use layerx_crypto::disclosure::call::{bind_call, reencode};
use layerx_crypto::disclosure::DisclosureError;
use layerx_crypto::evm_transaction::{decode_signed, recover_sender};
use layerx_crypto::signer::Signer;
use layerx_wire::hash;
use sha2::{Digest as _, Sha256};

struct Fixture {
    grant: &'static [u8],
    id: &'static [u8; 32],
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

#[test]
fn original_native_fee_grants_preserve_owner_signatures_and_native_ids(
) -> Result<(), Box<dyn std::error::Error>> {
    for fixture in FIXTURES {
        let grant = AuthorityGrant::decode(fixture.grant)
            .unwrap_or_else(|error| panic!("grant: {error:?}"));
        assert_eq!(grant.encode(), Ok(fixture.grant.to_vec()));
        assert_eq!(fixture.grant[4], 2);
        let mut hasher = Sha256::new();
        hasher.update(hash::Domain::AuthorityHash.tag());
        hasher.update(fixture.grant);
        assert_eq!(hasher.finalize().as_slice(), fixture.id);
        let options = support::call_options(7);
        let produced = authority_grant_call(&grant, &options)
            .map_err(|error| format!("merged grant producer: {error:?}"))?;
        let account = layerx_crypto::account_derivation::derive_from_seed(&[7; 64], 0)
            .map_err(|error| format!("transaction account: {error:?}"))?;
        let raw = produced
            .call
            .sign(account.evm_secret().ok_or("EVM key")?)
            .map_err(|error| format!("signed submission: {error:?}"))?;
        let (received, signature) =
            decode_signed(&raw).map_err(|error| format!("transaction transport: {error:?}"))?;
        assert_eq!(recover_sender(&received, &signature), Ok(options.from));
        assert_eq!(support::signer().evm_address(), Some(options.from));
        let disclosed = bind_call(&received, options.from)
            .map_err(|error| format!("grant consumer: {error:?}"))?;
        assert_eq!(disclosed.authority_grant, Some(grant));
        assert_eq!(disclosed.authority, options.from);
        assert_eq!(reencode(&disclosed), Ok(received.encode_unsigned()));
        // The precompile derives the returned grantId from the same raw body.
        let mut received_id = Sha256::new();
        received_id.update(hash::Domain::AuthorityHash.tag());
        received_id.update(
            disclosed
                .authority_grant
                .ok_or("bound grant")?
                .encode()
                .map_err(|error| format!("body: {error:?}"))?,
        );
        assert_eq!(received_id.finalize().as_slice(), fixture.id);
    }

    Ok(())
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
fn every_fee_field_is_bound_into_the_signing_disclosure() -> Result<(), Box<dyn std::error::Error>>
{
    let fixture = &FIXTURES[4];
    let original = AuthorityGrant::decode(fixture.grant)
        .map_err(|error| format!("native grant vector: {error:?}"))?;
    let grant = original;
    let options = support::call_options(7);
    let produced = authority_grant_call(&grant, &options)
        .map_err(|error| format!("merged grant producer: {error:?}"))?;
    let disclosure = bind_call(&produced.call, options.from)
        .map_err(|error| format!("grant consumer: {error:?}"))?;
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
            let account = layerx_crypto::account_derivation::derive_from_seed(&[7; 64], 0)
                .map_err(|error| format!("owner transaction key: {error:?}"))?;
            let raw = produced
                .call
                .sign(account.evm_secret().ok_or("EVM key")?)
                .map_err(|error| format!("original grant transaction signature: {error:?}"))?;
            let (_, signature) =
                decode_signed(&raw).map_err(|error| format!("original transport: {error:?}"))?;
            let altered = authority_grant_call(&grant, &options)
                .map_err(|error| format!("altered grant call: {error:?}"))?;
            assert_ne!(altered.call.signing_hash(), produced.call.signing_hash());
            assert_ne!(recover_sender(&altered.call, &signature), Ok(options.from));
            assert!(layerx_crypto::disclosure::call::validate_call(
                &altered.call,
                &disclosure,
                options.from
            )
            .is_err());
        }
        changed.authority_grant = Some(grant);
        assert_eq!(
            reencode(&changed),
            Err(DisclosureError::FieldMismatch("authority_grant"))
        );
        assert!(changed.audit_digest().is_err());
    }
    let mut removed = disclosure;
    let mut grant = original;
    grant.fee_budget = None;
    assert_ne!(
        grant
            .encode()
            .map_err(|error| format!("no fee budget: {error:?}"))?,
        fixture.grant
    );
    removed.authority_grant = Some(grant);
    assert_eq!(
        reencode(&removed),
        Err(DisclosureError::FieldMismatch("authority_grant"))
    );

    Ok(())
}
