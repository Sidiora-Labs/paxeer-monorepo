use layerx_committed_proof::{
    header::{
        validators_hash, BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp, Validator,
    },
    verify_header, ProofError,
};

// Recorded by a Go one-off against revision 88bf9744288ab91ec6123cf5861a0a364feb72d3:
// consensus/types/block.go:513 Header.Hash and validator_set.go:364 ValidatorSet.Hash,
// whose leaves are validator.go:153 Validator.Bytes. The set is constructed in the
// order below with types.NewValidator, without sorting. Public keys come from
// crypto/ed25519.NewKeyFromSeed(bytes.Repeat([]byte{i}, 32)).Public(), i=1..4,
// converted with consensus/crypto/ed25519.PublicKeyFromBytes. The populated Go
// Header uses the fields in reference_header(), with time.Unix(1700000123,
// 987654321).UTC(). Sparse vectors keep only ValidatorsHash and Time, using
// time.Unix(-1, 0), then time.Unix(0, 0), then Height=-1. The reordered set swaps
// validators 0 and 1; the empty vector calls (&types.ValidatorSet{}).Hash().
const HEADER_HASH: &str = "6ea46ec595bac2dc17d1a567a4bff602c059522020a3b5afafa4e8607ea4db99";
const VALIDATORS_HASH: &str = "0321b92eefbfb6129dcbfb8c9066c149bb1091c312f388e2142ef4bb4ba86fb0";
const SPARSE_HASH: &str = "1b24feafa580b99217ebc9e6c2c901dccd4c811ab7f63317cf5c7a42ecf5a727";
const EPOCH_HASH: &str = "06d746f12e090549ecbd69129f5624d81c25ad3c5c69c1b64a869a34b800d166";
const NEGATIVE_HEIGHT_HASH: &str =
    "2e2d84ec16e71731ff2ee894e039785986f36fe02f68d0e24d657e2885d3669c";
const REORDERED_HASH: &str = "76e35940da2115bc19577fd58ab45f4ed7e34e714ddddacccf14c7303e58d2cf";
const EMPTY_HASH: &str = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855";
const VALIDATORS: [(&str, i64); 4] = [
    (
        "8a88e3dd7409f195fd52db2d3cba5d72ca6709bf1d94121bf3748801b40f6f5c",
        40,
    ),
    (
        "8139770ea87d175f56a35466c34c7ecccb8d8a91b4ee37a25df60f5b8fc9b394",
        30,
    ),
    (
        "ed4928c628d1c2c6eae90338905995612959273a5c63f93636c14614ac8737d1",
        20,
    ),
    (
        "ca93ac1705187071d67b83c7ff0efe8108e8ec4530575d7726879333dbdabe7c",
        10,
    ),
];

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
    VALIDATORS.map(|(key, power)| Validator {
        pub_key: hex_bytes(key),
        voting_power: power,
    })
}

fn reference_header() -> Header {
    Header {
        version: ConsensusVersion { block: 11, app: 2 },
        chain_id: "paxeer-committed-π".to_owned(),
        height: 1_234_567_890_123,
        time: Timestamp {
            seconds: 1_700_000_123,
            nanos: 987_654_321,
        },
        last_block_id: BlockId {
            hash: vec![0x11; 32],
            part_set_header: PartSetHeader {
                total: 257,
                hash: vec![0x22; 32],
            },
        },
        last_commit_hash: vec![0x33; 32],
        data_hash: vec![0x44; 32],
        validators_hash: hex_bytes(VALIDATORS_HASH),
        next_validators_hash: vec![0x55; 32],
        consensus_hash: vec![0x66; 32],
        app_hash: vec![0, 1, 2, 0xff, 0x80],
        last_results_hash: vec![0x77; 32],
        evidence_hash: vec![0x88; 32],
        proposer_address: hex_bytes::<20>("34750f98bd59fcfc946da45aaabe933be154a4b5").to_vec(),
    }
}

#[test]
fn header_hash_matches_the_reference_header() {
    let header = reference_header();
    let mut set = reference_set();
    assert_eq!(header.hash(), hex_bytes(HEADER_HASH));
    assert_eq!(verify_header(&header, &set), Ok(hex_bytes(HEADER_HASH)));

    set[0].voting_power += 1;
    assert_eq!(
        verify_header(&header, &set),
        Err(ProofError::ValidatorsHashMismatch)
    );
    let mut wrong_commitment = header.clone();
    wrong_commitment.validators_hash[0] ^= 1;
    assert_eq!(
        verify_header(&wrong_commitment, &reference_set()),
        Err(ProofError::ValidatorsHashMismatch)
    );

    let mut sparse = Header {
        version: ConsensusVersion::default(),
        chain_id: String::new(),
        height: 0,
        time: Timestamp {
            seconds: -1,
            nanos: 0,
        },
        last_block_id: BlockId::default(),
        last_commit_hash: Vec::new(),
        data_hash: Vec::new(),
        validators_hash: hex_bytes(VALIDATORS_HASH),
        next_validators_hash: Vec::new(),
        consensus_hash: Vec::new(),
        app_hash: Vec::new(),
        last_results_hash: Vec::new(),
        evidence_hash: Vec::new(),
        proposer_address: Vec::new(),
    };
    assert_eq!(sparse.hash(), hex_bytes(SPARSE_HASH));
    sparse.time.seconds = 0;
    assert_eq!(sparse.hash(), hex_bytes(EPOCH_HASH));
    sparse.height = -1;
    assert_eq!(sparse.hash(), hex_bytes(NEGATIVE_HEIGHT_HASH));
}

#[test]
fn validators_hash_matches_the_reference_set() {
    let mut set = reference_set();
    assert_eq!(validators_hash(&set), hex_bytes(VALIDATORS_HASH));
    set.swap(0, 1);
    assert_eq!(validators_hash(&set), hex_bytes(REORDERED_HASH));
    assert_ne!(validators_hash(&set), hex_bytes(VALIDATORS_HASH));
    assert_eq!(
        verify_header(&reference_header(), &set),
        Err(ProofError::ValidatorsHashMismatch)
    );

    let mut changed_key = reference_set();
    changed_key[0].pub_key[0] ^= 1;
    assert_ne!(validators_hash(&changed_key), hex_bytes(VALIDATORS_HASH));
    assert_eq!(validators_hash(&[]), hex_bytes(EMPTY_HASH));
}
