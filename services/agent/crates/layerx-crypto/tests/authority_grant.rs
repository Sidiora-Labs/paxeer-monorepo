use crate::support;
use ed25519_dalek::SigningKey;
use layerx_crypto::authority_grant::{authority_grant_call, AuthorityGrant, GrantKind};
use layerx_crypto::disclosure::call::{bind_call, reencode, validate_call};
use layerx_crypto::disclosure::DisclosureError;
use layerx_types::ids::Did;
use layerx_wire::hash;

const CAPABILITY: &[u8] = include_bytes!("fixtures/authority-grant-capability.bin");
const BUDGET: &[u8] = include_bytes!("fixtures/authority-grant-budget.bin");

fn native_grant(bytes: &[u8]) -> AuthorityGrant {
    AuthorityGrant::decode(bytes).unwrap_or_else(|error| panic!("native grant: {error:?}"))
}

fn owner() -> (Did, [u8; 32]) {
    let key = SigningKey::from_bytes(&[0x11; 32])
        .verifying_key()
        .to_bytes();
    let mut did = String::from("did:layerx:");
    for byte in key {
        use std::fmt::Write as _;
        assert!(write!(&mut did, "{byte:02x}").is_ok());
    }
    (
        Did::new(did.as_bytes()).unwrap_or_else(|error| panic!("did: {error:?}")),
        key,
    )
}

#[test]
fn real_native_builder_matches_both_grant_kinds() {
    let (did, _) = owner();
    for (bytes, kind) in [
        (CAPABILITY, GrantKind::DelegatedCapability),
        (BUDGET, GrantKind::BudgetAllowance),
    ] {
        let grant = native_grant(bytes);
        assert_eq!(grant.kind, kind);
        assert_eq!(
            grant.grantor,
            hash::did_id_for_protocol(&did, 3).unwrap_or_else(|error| panic!("hash: {error:?}"))
        );
        assert_eq!(grant.grantee, grant.grantor);
        assert_eq!(
            grant.delegate_key,
            SigningKey::from_bytes(&[0x33; 32])
                .verifying_key()
                .to_bytes()
        );
        assert_eq!(grant.scope.module_mask, 1 << 7);
        assert_eq!((grant.scope.ordinal_min, grant.scope.ordinal_max), (1, 8));
        assert_eq!(grant.scope.maximum_per_activity, 10);
        assert_eq!(grant.scope.maximum_total, 30);
        assert_eq!(grant.not_before, 1000);
        assert_eq!(grant.not_after, 3_601_000);
        assert_eq!(grant.revocation_sequence, 7);
        assert_eq!(grant.encode(), Ok(bytes.to_vec()));
        let payload = grant
            .payload()
            .unwrap_or_else(|error| panic!("payload: {error:?}"));
        assert_eq!(&payload[..4], &[0x71, 8, 1, 1]);
        assert_eq!(AuthorityGrant::from_payload(&payload), Ok(grant));
        for length in 0..payload.len() {
            assert!(AuthorityGrant::from_payload(&payload[..length]).is_err());
        }
        let mut trailing = payload.clone();
        trailing.push(0);
        assert!(AuthorityGrant::from_payload(&trailing).is_err());
        for index in [0, 1, 2, 3] {
            let mut unknown = payload.clone();
            unknown[index] ^= 0x80;
            assert!(AuthorityGrant::from_payload(&unknown).is_err());
        }
    }
}

#[test]
fn unbounded_or_inconsistent_grants_cannot_be_encoded() {
    let mutations: &[fn(&mut AuthorityGrant)] = &[
        |g| g.grantor = [0; 32],
        |g| g.grantee[0] ^= 1,
        |g| g.delegate_key = [0; 32],
        |g| g.delegate_key = [0xff; 32],
        |g| g.scope.asset = [0; 32],
        |g| g.scope.purpose = [0; 32],
        |g| g.scope.maximum_per_activity = 0,
        |g| g.scope.maximum_total = 0,
        |g| g.scope.maximum_per_activity = 31,
        |g| g.scope.module_mask = 0,
        |g| g.scope.module_mask = 1 << 63,
        |g| g.scope.ordinal_min = 0,
        |g| g.scope.ordinal_min = 9,
        |g| g.not_after = g.not_before,
        |g| g.not_after = u64::MAX,
        |g| g.revocation_sequence = 0,
        |g| g.scope.period_start = 1,
        |g| g.scope.maximum_per_period = 20,
    ];
    for mutate in mutations {
        let mut grant = native_grant(CAPABILITY);
        mutate(&mut grant);
        assert!(grant.encode().is_err(), "accepted invalid grant {grant:?}");
    }
    let mut recurring = native_grant(BUDGET);
    recurring.scope.period_start += 1;
    assert!(recurring.encode().is_err());
    recurring = native_grant(BUDGET);
    recurring.scope.maximum_per_period = 9;
    assert!(recurring.encode().is_err());
}

#[test]
fn signing_disclosure_binds_every_immutable_grant_field() -> Result<(), Box<dyn std::error::Error>>
{
    let grant = native_grant(BUDGET);
    let payload = grant
        .payload()
        .map_err(|error| format!("native grant payload: {error:?}"))?;
    let options = support::call_options(10);
    let produced = authority_grant_call(&grant, &options)
        .map_err(|error| format!("merged grant constructor: {error:?}"))?;
    let disclosure = bind_call(&produced.call, options.from)
        .map_err(|error| format!("grant call binding: {error:?}"))?;
    assert_eq!(disclosure.authority_grant, Some(grant));
    assert_eq!(disclosure.asset, grant.scope.asset);
    assert_eq!(disclosure.authority, options.from);
    assert_eq!(disclosure.expiry.payload_expires_at, grant.not_after);
    assert_eq!(reencode(&disclosure), Ok(produced.call.encode_unsigned()));
    let mutations: &[fn(&mut AuthorityGrant)] = &[
        |g| g.grantor[0] ^= 1,
        |g| g.grantee[0] ^= 1,
        |g| g.delegate_key[0] ^= 1,
        |g| g.kind = GrantKind::DelegatedCapability,
        |g| g.scope.module_mask ^= 2,
        |g| g.scope.ordinal_min += 1,
        |g| g.scope.ordinal_max -= 1,
        |g| g.scope.asset[0] ^= 1,
        |g| g.scope.maximum_per_activity += 1,
        |g| g.scope.maximum_total += 1,
        |g| g.scope.period_length += 1,
        |g| g.scope.maximum_per_period += 1,
        |g| g.scope.period_start += 1,
        |g| g.scope.purpose[0] ^= 1,
        |g| g.not_before += 1,
        |g| g.not_after += 1,
        |g| g.revocation_sequence += 1,
    ];
    for mutate in mutations {
        let mut changed = disclosure.clone();
        let mut semantics = grant;
        mutate(&mut semantics);
        // Every immutable change either fails the retained codec or produces
        // different canonical kernel bytes. Neither can be injected into a call.
        if let Ok(changed_payload) = semantics.payload() {
            assert_ne!(changed_payload, payload);
            assert_eq!(
                AuthorityGrant::from_payload(&changed_payload)
                    .map_err(|error| format!("changed codec: {error:?}"))?,
                semantics
            );
        }
        changed.authority_grant = Some(semantics);
        assert_eq!(
            reencode(&changed),
            Err(DisclosureError::FieldMismatch("authority_grant"))
        );
        assert!(changed.audit_digest().is_err());
    }
    let mut different = options.from;
    different[0] ^= 1;
    assert!(validate_call(&produced.call, &disclosure, different).is_err());
    let mut substituted = produced.call;
    substituted.data[0] ^= 1;
    assert!(bind_call(&substituted, options.from).is_err());

    Ok(())
}
