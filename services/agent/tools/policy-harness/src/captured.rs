use layerx_agentd::read::{
    BlockCommit, BlockId, CheckpointHeader, CommitTimestamp, CommitValidator, CommitVote,
    ServedCheckpoint,
};

// Captured by the budget owner: tests/vectors/budget-proof-v2/node-answers.json.
pub const HEIGHT: u64 = 8;
pub const BUDGET_ID: [u8; 32] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 7,
];
pub const CALL_DATA: [u8; 36] = [
    84, 104, 135, 163, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
    0, 0, 0, 0, 0, 0, 7,
];
pub fn budget_result() -> Result<Vec<u8>, String> {
    let transcript = layerx_types::json::parse(include_str!(
        "../../../../../tests/vectors/budget-proof-v2/node-answers.json"
    ))
    .map_err(|error| error.to_string())?;
    transcript
        .hex_at("4.result")
        .map_err(|error| error.to_string())
}

pub fn checkpoint() -> Result<ServedCheckpoint, String> {
    let validator = CommitValidator {
        address: [
            52, 117, 15, 152, 189, 89, 252, 252, 148, 109, 164, 90, 170, 190, 147, 59, 225, 84,
            164, 181,
        ],
        public_key: [
            138, 136, 227, 221, 116, 9, 241, 149, 253, 82, 219, 45, 60, 186, 93, 114, 202, 103, 9,
            191, 29, 148, 18, 27, 243, 116, 136, 1, 180, 15, 111, 92,
        ],
        voting_power: 10,
    };
    let timestamp = CommitTimestamp {
        seconds: 1_700_000_000,
        nanos: 0,
    };
    let header = CheckpointHeader {
        version_block: 11,
        version_app: 1,
        chain_id: "125".to_owned(),
        height: HEIGHT,
        time: timestamp,
        last_block_id: Some(BlockId {
            hash: [1; 32],
            part_set_total: 1,
            part_set_hash: [2; 32],
        }),
        last_commit_hash: Some([3; 32]),
        data_hash: Some([4; 32]),
        validators_hash: [
            168, 218, 91, 205, 147, 122, 117, 56, 181, 163, 233, 166, 123, 167, 52, 101, 87, 5,
            171, 42, 128, 94, 59, 151, 37, 94, 144, 100, 71, 71, 180, 151,
        ],
        next_validators_hash: [
            168, 218, 91, 205, 147, 122, 117, 56, 181, 163, 233, 166, 123, 167, 52, 101, 87, 5,
            171, 42, 128, 94, 59, 151, 37, 94, 144, 100, 71, 71, 180, 151,
        ],
        consensus_hash: [5; 32],
        app_hash: [6; 32],
        last_results_hash: Some([7; 32]),
        evidence_hash: Some([8; 32]),
        proposer_address: validator.address,
    };
    let commit = BlockCommit {
        height: HEIGHT,
        round: 1,
        block_id: BlockId {
            hash: [
                124, 110, 3, 185, 185, 72, 73, 85, 109, 238, 245, 11, 124, 114, 241, 232, 41, 16,
                102, 66, 217, 43, 151, 87, 68, 72, 131, 126, 188, 209, 238, 220,
            ],
            part_set_total: 1,
            part_set_hash: [9; 32],
        },
        votes: vec![CommitVote {
            validator_address: validator.address,
            voting_power: 10,
            timestamp,
            signature: [
                119, 14, 89, 18, 146, 76, 121, 226, 162, 19, 229, 1, 80, 111, 149, 32, 173, 103,
                115, 60, 216, 6, 139, 240, 176, 141, 42, 42, 141, 143, 10, 250, 224, 62, 126, 42,
                109, 174, 68, 208, 93, 64, 239, 220, 2, 210, 253, 251, 99, 201, 127, 57, 125, 105,
                162, 53, 81, 97, 38, 10, 202, 3, 181, 12,
            ],
        }],
    };
    layerx_agentd::read::checkpoint(HEIGHT, "125", &header, &commit, &[validator])
        .map_err(|error| format!("captured checkpoint: {error:?}"))
}
