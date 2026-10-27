use layerx_committed_proof::{
    commit::{verify_commit, BlockIdFlag, Commit, CommitSig},
    header::{BlockId, PartSetHeader, Timestamp, Validator},
    ProofError,
};

const CHAIN_ID: &str = "paxeer-commit-reference";
const HEIGHT: u64 = 1_234_567_890_123;
const HEADER_HASH: [u8; 32] = [0x11; 32];

fn hex_bytes<const N: usize>(hex: &str) -> [u8; N] {
    assert_eq!(hex.len(), 2 * N);
    let mut bytes = [0; N];
    for (index, byte) in bytes.iter_mut().enumerate() {
        *byte = u8::from_str_radix(&hex[2 * index..2 * index + 2], 16)
            .unwrap_or_else(|error| panic!("invalid Go reference bytes: {error}"));
    }
    bytes
}

fn reference_set() -> [Validator; 4] {
    PUBLIC_KEYS.map(|key| Validator {
        pub_key: hex_bytes(key),
        voting_power: 10,
    })
}

fn reference_vote(index: usize, flag: BlockIdFlag) -> CommitSig {
    CommitSig {
        block_id_flag: flag,
        validator_address: hex_bytes::<20>(ADDRESSES[index]).to_vec(),
        timestamp: Timestamp {
            seconds: 1_700_000_123 + index as i64,
            nanos: 987_654_321 - index as u32,
        },
        signature: Some(hex_bytes(if flag == BlockIdFlag::Nil {
            NIL_SIGNATURES[index]
        } else {
            BLOCK_SIGNATURES[index]
        })),
    }
}

fn reference_commit(signers: usize) -> Commit {
    Commit {
        height: HEIGHT,
        round: 7,
        block_id: BlockId {
            hash: HEADER_HASH.to_vec(),
            part_set_header: PartSetHeader {
                total: 2,
                hash: vec![0x22; 32],
            },
        },
        signatures: (0..4)
            .map(|index| {
                if index < signers {
                    reference_vote(index, BlockIdFlag::Commit)
                } else {
                    CommitSig::absent()
                }
            })
            .collect(),
    }
}

#[test]
fn a_three_of_four_commit_verifies() {
    let set = reference_set();
    let commit = reference_commit(3);
    assert_eq!(
        verify_commit(CHAIN_ID, &set, HEADER_HASH, HEIGHT, &commit),
        Ok(30)
    );
    assert_eq!(
        verify_commit(CHAIN_ID, &set, HEADER_HASH, HEIGHT, &reference_commit(4)),
        Ok(40)
    );

    let mut with_nil = commit.clone();
    with_nil.signatures[3] = reference_vote(3, BlockIdFlag::Nil);
    assert_eq!(
        verify_commit(CHAIN_ID, &set, HEADER_HASH, HEIGHT, &with_nil),
        Ok(30)
    );
    with_nil.signatures[3]
        .signature
        .as_mut()
        .unwrap_or_else(|| panic!("missing reference signature"))[0] ^= 1;
    assert_eq!(
        verify_commit(CHAIN_ID, &set, HEADER_HASH, HEIGHT, &with_nil),
        Err(ProofError::InvalidCommitSignature)
    );

    let mut after_quorum = reference_commit(4);
    after_quorum.signatures[3].signature = Some([0; 64]);
    assert_eq!(
        verify_commit(CHAIN_ID, &set, HEADER_HASH, HEIGHT, &after_quorum),
        Err(ProofError::InvalidCommitSignature)
    );
    assert_eq!(
        verify_commit("another-chain", &set, HEADER_HASH, HEIGHT, &commit),
        Err(ProofError::InvalidCommitSignature)
    );
    assert_eq!(
        verify_commit(CHAIN_ID, &set, HEADER_HASH, HEIGHT + 1, &commit),
        Err(ProofError::InvalidCommit)
    );
    assert_eq!(
        verify_commit(CHAIN_ID, &set, [0x33; 32], HEIGHT, &commit),
        Err(ProofError::InvalidCommit)
    );

    for mutation in 0..6 {
        let mut changed = commit.clone();
        match mutation {
            0 => changed.round += 1,
            1 => changed.signatures[0].timestamp.nanos += 1,
            2 => changed.block_id.part_set_header.hash[0] ^= 1,
            3 => changed.block_id.part_set_header.total += 1,
            4 => changed.signatures.swap(0, 1),
            5 => changed.signatures[0].validator_address[0] ^= 1,
            _ => unreachable!(),
        }
        assert_eq!(
            verify_commit(CHAIN_ID, &set, HEADER_HASH, HEIGHT, &changed),
            Err(ProofError::InvalidCommitSignature),
            "mutation {mutation} must invalidate the signed vote"
        );
    }

    let mut zero_round = reference_commit(4);
    zero_round.round = 0;
    for (index, vote) in zero_round.signatures.iter_mut().enumerate() {
        vote.timestamp = Timestamp {
            seconds: index as i64 - 1,
            nanos: 0,
        };
        vote.signature = Some(hex_bytes(ZERO_ROUND_SIGNATURES[index]));
    }
    assert_eq!(
        verify_commit(CHAIN_ID, &set, HEADER_HASH, HEIGHT, &zero_round),
        Ok(40)
    );
}

#[test]
fn a_two_of_four_commit_is_refused() {
    let set = reference_set();
    let commit = reference_commit(2);
    assert_eq!(
        verify_commit(CHAIN_ID, &set, HEADER_HASH, HEIGHT, &commit),
        Err(ProofError::InsufficientVotingPower)
    );

    let mut exactly_two_thirds = commit.clone();
    exactly_two_thirds.signatures.truncate(3);
    assert_eq!(
        verify_commit(
            CHAIN_ID,
            &set[..3],
            HEADER_HASH,
            HEIGHT,
            &exactly_two_thirds
        ),
        Err(ProofError::InsufficientVotingPower)
    );

    let mut nil_is_not_power = commit.clone();
    nil_is_not_power.signatures[2] = reference_vote(2, BlockIdFlag::Nil);
    nil_is_not_power.signatures[3] = reference_vote(3, BlockIdFlag::Nil);
    assert_eq!(
        verify_commit(CHAIN_ID, &set, HEADER_HASH, HEIGHT, &nil_is_not_power),
        Err(ProofError::InsufficientVotingPower)
    );

    let mut weighted = set;
    weighted[0].voting_power = 40;
    weighted[1].voting_power = 40;
    assert_eq!(
        verify_commit(CHAIN_ID, &weighted, HEADER_HASH, HEIGHT, &commit),
        Ok(80)
    );
    let mut weak_majority = set;
    weak_majority[3].voting_power = 100;
    assert_eq!(
        verify_commit(
            CHAIN_ID,
            &weak_majority,
            HEADER_HASH,
            HEIGHT,
            &reference_commit(3)
        ),
        Err(ProofError::InsufficientVotingPower)
    );

    for mutation in 0..4 {
        let mut invalid_set = set;
        match mutation {
            0 => invalid_set[3].voting_power = -1,
            1 => invalid_set[3].voting_power = 0,
            2 => invalid_set[3].voting_power = i64::MAX,
            3 => invalid_set[3].pub_key = invalid_set[0].pub_key,
            _ => unreachable!(),
        }
        assert_eq!(
            verify_commit(CHAIN_ID, &invalid_set, HEADER_HASH, HEIGHT, &commit),
            Err(ProofError::InvalidValidatorSet)
        );
    }

    let mut missing_slot = reference_commit(3);
    missing_slot.signatures.pop();
    assert_eq!(
        verify_commit(CHAIN_ID, &set, HEADER_HASH, HEIGHT, &missing_slot),
        Err(ProofError::InvalidCommit)
    );
    let mut missing_signature = reference_commit(3);
    missing_signature.signatures[0].signature = None;
    assert_eq!(
        verify_commit(CHAIN_ID, &set, HEADER_HASH, HEIGHT, &missing_signature),
        Err(ProofError::InvalidCommit)
    );
    let mut invalid_absence = reference_commit(3);
    invalid_absence.signatures[3].signature = Some([0; 64]);
    assert_eq!(
        verify_commit(CHAIN_ID, &set, HEADER_HASH, HEIGHT, &invalid_absence),
        Err(ProofError::InvalidCommit)
    );
    assert_eq!(
        verify_commit(
            CHAIN_ID,
            &[],
            HEADER_HASH,
            HEIGHT,
            &Commit {
                signatures: Vec::new(),
                ..commit
            }
        ),
        Err(ProofError::InvalidCommit)
    );
}

// Recorded by a Go one-off calling consensus/types/block.go Commit.VoteSignBytes
// and signing the returned bytes with crypto/ed25519. Keys use seeds [i; 32],
// i=1..4. The commit uses CHAIN_ID, HEIGHT, round 7, hash [0x11; 32], and
// parts {total: 2, hash: [0x22; 32]}. Each timestamp is
// time.Unix(1700000123+i, 987654321-i).UTC(), i=0..3. Nil vectors change
// only BlockIDFlag; zero-round vectors use round 0 and time.Unix(i-1, 0).
const PUBLIC_KEYS: [&str; 4] = [
    "8a88e3dd7409f195fd52db2d3cba5d72ca6709bf1d94121bf3748801b40f6f5c",
    "8139770ea87d175f56a35466c34c7ecccb8d8a91b4ee37a25df60f5b8fc9b394",
    "ed4928c628d1c2c6eae90338905995612959273a5c63f93636c14614ac8737d1",
    "ca93ac1705187071d67b83c7ff0efe8108e8ec4530575d7726879333dbdabe7c",
];
const ADDRESSES: [&str; 4] = [
    "34750f98bd59fcfc946da45aaabe933be154a4b5",
    "6a3803d5f059902a1c6dafbc9ba4729212f7caac",
    "b62e867fa2f33afe62d5d6b1642e1621d5433078",
    "c5b940ed3f65c391965de8295fc5d25f474fa57b",
];
const BLOCK_SIGNATURES: [&str; 4] = [
    "5b0029c887146fb4be46a5fabcacaf811b9159c73d50a2f326e85edf91c57c4839af327a00ef0afb74114f08cd1645efaff7216d2c244dfa026b9eab90540d0e",
    "136eb42f2f1f5c2616dc93b40126fc8afd7e9d20ef1253efaa0fc11d87dfaea4249c292466e111028b37877b4225c7b639db279db43e8a8f8337c091eeb5a704",
    "396ffba4c83e45ad59ae3583afd8163aede64eb4994eed0ada66422b650cc7b623acb60bad2cfda6230f2112ad7e634265d2069c92b9e1730dd52b54aeb34702",
    "61a55f664f1dd70dec9c87240215fb124e46cc4e8edb25b51617c7c54dd0f02e8ea1b5e52ea85fd65e59449035251bcd1b5af61a8a8ee034a47708ee565ade09",
];
const NIL_SIGNATURES: [&str; 4] = [
    "ebe22040e1430eed4f18c3b5c10f31f5b0531f8ebd99d5ad33090553f681eac0b1613ab7daf8cd4430ac69a42cb7ae3a8709c47a6273fd6200ef5c3e8bd6e804",
    "a1d19441ab6ccf4bc9073f60e30e589960ff57c4418437a51c59cd7a358e419e205fc0723bd56523c5c1f2693dbe26dbc40e36c699cdf19c995a3849a97e080c",
    "a17eb6fdfdc0b3328c6439e2d681b3a1b87d8e1f4a5cea4bc7e22f18c4f30bceea651f3af81278bc4bf2611359c318e9cd96471d3634e1f5094127a0bc805b02",
    "c2c7bd3b59b939750d8a7eace26db666ad86133c9a5238d1db5f528064119d72a7637527b748febd174dd2ff6771bce41f118bd404e075815a45b6358630410b",
];
const ZERO_ROUND_SIGNATURES: [&str; 4] = [
    "99874ca26d48b7bdd803c9d01714fbb68ef44beca1f87bdc2a191652919be26013f216d612d0039c1b22b10cde8c0acff4efcebfd3ef24599a8f9dd6bd00250e",
    "aa994b3164f3db55541fcf30a94a029097aea7424e4ce6301483810bd102899d7ce49c8169bad11ef08b0befeab878682302ec6490995156c2d888a5e2b1200e",
    "ef5f4ee96d64a79578b5be0a95a87a3cee05039ea2a6ef580ca00b9823141bba03aaf904ca2ce0db3c37c59aabe2978dc0b49550781ce52c0142f465c7104d09",
    "18f6a34500a02e077a1e28074e62cdc6cdc45f33997a0b6f689fbcd8c1b8f0324579c2d9396d0b3a01f1b07f885a41d78f551c4f4f4c049cfe212f7212b1d506",
];
