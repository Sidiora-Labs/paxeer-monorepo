#[path = "support/chain.rs"]
mod chain;

use chain::{ChainFixture, STATE_HEIGHT};
use layerx_programs::{
    chain_evidence::programs_state_key, AccountStateError, CanonicalAccountLeaf,
    VerifiedAccountSnapshot,
};

#[test]
fn chain_account_snapshot_verifies_primary_leaf() {
    let account_id = [0x31; 32];
    let leaf = CanonicalAccountLeaf {
        account_id,
        name: format!("module:programs:value:{}", "31".repeat(32)).into_bytes(),
        kind: 13,
        balance: (1_u128 << 96) + 42,
        asset_id: [0x51; 32],
        has_asset: true.into(),
        next_sequence: 7,
        created_at_sequence: 3,
        frozen: true.into(),
        has_open_reference: true.into(),
        authority_key: [0; 32],
        has_authority_key: false.into(),
    };
    let mut primary_key = vec![4];
    primary_key.extend_from_slice(&account_id);
    assert_eq!(leaf.primary_key().as_slice(), primary_key);
    let mut key = vec![0x01, 0x00, 0x09];
    key.extend_from_slice(&primary_key);
    assert_eq!(programs_state_key(&leaf.primary_key()), key);

    let mut value = 86_u16.to_be_bytes().to_vec();
    value.extend_from_slice(&leaf.name);
    value.push(13);
    value.extend_from_slice(&leaf.balance.to_be_bytes());
    value.extend_from_slice(&[0x51; 32]);
    value.push(1);
    value.extend_from_slice(&7_u64.to_be_bytes());
    value.extend_from_slice(&3_u64.to_be_bytes());
    value.extend_from_slice(&[1, 1]);
    value.extend_from_slice(&[0; 32]);
    value.push(0);
    assert_eq!(leaf.primary_value(), Ok(value.clone()));

    let seeds = [[1; 32], [2; 32], [3; 32], [4; 32]];
    let fixture = ChainFixture::new(
        "paxeer-account-snapshot",
        &seeds,
        vec![
            (key.clone(), value.clone()),
            (programs_state_key(b"other-record"), vec![1]),
        ],
    )
    .unwrap_or_else(|error| panic!("chain fixture: {error:?}"));
    let head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap_or_else(|error| panic!("verified chain head: {error:?}"));
    let ops = fixture
        .prove(&key)
        .unwrap_or_else(|error| panic!("account proof: {error:?}"));
    let snapshot = VerifiedAccountSnapshot::verify_chain(&head, &leaf, &ops)
        .unwrap_or_else(|error| panic!("verified account: {error}"));
    assert_eq!(snapshot.account(), &leaf);
    assert_eq!(snapshot.account().balance, (1_u128 << 96) + 42);
    assert_eq!(snapshot.chain_head(), &head);
    assert_eq!(snapshot.height(), STATE_HEIGHT);
    assert_eq!(snapshot.height(), head.height() - 1);

    let mut changed = leaf.clone();
    changed.balance += 1;
    assert_eq!(
        VerifiedAccountSnapshot::verify_chain(&head, &changed, &ops),
        Err(AccountStateError::InvalidAccountLeaf)
    );

    let mut malformed = leaf.clone();
    malformed.name.push(b'x');
    assert_eq!(
        VerifiedAccountSnapshot::verify_chain(&head, &malformed, &ops),
        Err(AccountStateError::InvalidAccountLeaf)
    );
    assert_eq!(
        VerifiedAccountSnapshot::verify_chain(&head, &leaf, &[]),
        Err(AccountStateError::InvalidAccountLeaf)
    );
    let mut wrong_module = ops.clone();
    wrong_module[0].key[2] = 8;
    assert_eq!(
        VerifiedAccountSnapshot::verify_chain(&head, &leaf, &wrong_module),
        Err(AccountStateError::InvalidAccountLeaf)
    );
    let mut corrupt = ops.clone();
    corrupt[1].data.pop();
    assert_eq!(
        VerifiedAccountSnapshot::verify_chain(&head, &leaf, &corrupt),
        Err(AccountStateError::InvalidAccountLeaf)
    );

    let mut changed_value = value;
    changed_value[3] ^= 1;
    let other = ChainFixture::new(
        "paxeer-account-snapshot",
        &seeds,
        vec![(key, changed_value)],
    )
    .unwrap_or_else(|error| panic!("other chain fixture: {error:?}"));
    let other_head = other
        .verifier
        .verify_head(&other.header, &other.commit)
        .unwrap_or_else(|error| panic!("other verified head: {error:?}"));
    assert_ne!(other_head.app_hash(), head.app_hash());
    assert_eq!(
        VerifiedAccountSnapshot::verify_chain(&other_head, &leaf, &ops),
        Err(AccountStateError::InvalidAccountLeaf)
    );
}
