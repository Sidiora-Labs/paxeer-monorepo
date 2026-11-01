use ed25519_dalek::{Signer, SigningKey};
use layerx_programs::chain_proof::{
    decode::{CommitmentProof, ProofOp},
    header::{ChainHeader, HeaderBlockId, HeaderTime, HeaderVersion},
    ics23::{ExistenceProof, InnerOp, LeafOp},
    validators::{TrustedValidator, ValidatorSet},
    vote::{vote_sign_bytes, BlockId, BlockIdFlag, CommitSig, PartSetHeader, Timestamp},
    ChainProofError, ChainProofVerifier, Commit,
};
use sha2::{Digest, Sha256};

pub const STORE_NAME: &str = "layerx";
pub const STATE_HEIGHT: i64 = 37;

pub struct ChainFixture {
    pub verifier: ChainProofVerifier,
    pub header: ChainHeader,
    pub commit: Commit,
    store_name: String,
    tree: Tree,
    store_proof: ProofOp,
}

impl ChainFixture {
    pub fn new(
        chain_id: &str,
        validator_seeds: &[[u8; 32]],
        pairs: Vec<(Vec<u8>, Vec<u8>)>,
    ) -> Result<Self, ChainProofError> {
        Self::new_in_store(STORE_NAME, chain_id, validator_seeds, pairs)
    }

    pub fn new_in_store(
        store_name: &str,
        chain_id: &str,
        validator_seeds: &[[u8; 32]],
        mut pairs: Vec<(Vec<u8>, Vec<u8>)>,
    ) -> Result<Self, ChainProofError> {
        pairs.sort_by(|left, right| left.0.cmp(&right.0));
        if validator_seeds.is_empty()
            || pairs.is_empty()
            || pairs
                .iter()
                .any(|(key, value)| key.is_empty() || value.is_empty())
            || pairs.windows(2).any(|pair| pair[0].0 == pair[1].0)
        {
            return Err(ChainProofError::NonCanonical);
        }
        let tree = Tree::new(&pairs, TreeKind::Iavl);
        let mut stores = ["auth", "bank", store_name, "staking"]
            .into_iter()
            .map(|name| {
                let root = if name == store_name {
                    tree.root
                } else {
                    Tree::new(
                        &[(b"fixture".to_vec(), name.as_bytes().to_vec())],
                        TreeKind::Iavl,
                    )
                    .root
                };
                (name.as_bytes().to_vec(), root.to_vec())
            })
            .collect::<Vec<_>>();
        stores.sort_by(|left, right| left.0.cmp(&right.0));
        let multistore = Tree::new(&stores, TreeKind::Simple);
        let store = multistore
            .leaves
            .iter()
            .find(|proof| proof.key == store_name.as_bytes())
            .ok_or(ChainProofError::Absent)?;
        let store_proof = ProofOp {
            op_type: "ics23:simple".to_owned(),
            key: store_name.as_bytes().to_vec(),
            data: message(1, &encode_existence(store)),
        };

        let mut keys = validator_seeds
            .iter()
            .map(SigningKey::from_bytes)
            .collect::<Vec<_>>();
        keys.sort_by_key(|key| TrustedValidator::new(key.verifying_key().to_bytes(), 1).address);
        let validators = ValidatorSet::new(
            keys.iter()
                .map(|key| TrustedValidator::new(key.verifying_key().to_bytes(), 1))
                .collect(),
        )?;
        let header = ChainHeader {
            version: HeaderVersion { block: 11, app: 1 },
            chain_id: chain_id.to_owned(),
            height: STATE_HEIGHT + 1,
            time: HeaderTime {
                seconds: 1_700_000_000,
                nanos: 123_456_789,
            },
            last_block_id: HeaderBlockId {
                hash: vec![1; 32],
                part_set_total: 1,
                part_set_hash: vec![2; 32],
            },
            last_commit_hash: vec![3; 32],
            data_hash: vec![4; 32],
            validators_hash: validators.hash().to_vec(),
            next_validators_hash: validators.hash().to_vec(),
            consensus_hash: vec![5; 32],
            app_hash: multistore.root.to_vec(),
            last_results_hash: vec![6; 32],
            evidence_hash: vec![7; 32],
            proposer_address: validators.validators()[0].address.to_vec(),
        };
        let mut commit = Commit {
            height: header.height,
            round: 0,
            block_id: BlockId {
                hash: header.hash()?.to_vec(),
                part_set_header: PartSetHeader {
                    total: 1,
                    hash: vec![8; 32],
                },
            },
            signatures: Vec::new(),
        };
        let timestamp = Timestamp {
            seconds: header.time.seconds,
            nanos: header.time.nanos,
        };
        let bytes = vote_sign_bytes(
            chain_id,
            commit.height,
            commit.round,
            Some(&commit.block_id),
            timestamp,
        )?;
        commit.signatures = keys
            .iter()
            .zip(validators.validators())
            .map(|(key, validator)| CommitSig {
                flag: BlockIdFlag::Commit,
                validator_address: validator.address.to_vec(),
                timestamp,
                signature: key.sign(&bytes).to_bytes().to_vec(),
            })
            .collect();
        Ok(Self {
            verifier: ChainProofVerifier {
                chain_id: chain_id.to_owned(),
                validators,
            },
            header,
            commit,
            store_name: store_name.to_owned(),
            tree,
            store_proof,
        })
    }

    pub fn prove(&self, key: &[u8]) -> Result<Vec<ProofOp>, ChainProofError> {
        let index = self
            .tree
            .leaves
            .binary_search_by(|proof| proof.key.as_slice().cmp(key))
            .map_err(|_| ChainProofError::Absent)?;
        Ok(self.ops(key, 1, &encode_existence(&self.tree.leaves[index])))
    }

    pub fn prove_absent(&self, key: &[u8]) -> Result<Vec<ProofOp>, ChainProofError> {
        let index = match self
            .tree
            .leaves
            .binary_search_by(|proof| proof.key.as_slice().cmp(key))
        {
            Ok(_) => return Err(ChainProofError::Mismatch),
            Err(index) => index,
        };
        let mut proof = message(1, key);
        if index > 0 {
            bytes_field(
                2,
                &encode_existence(&self.tree.leaves[index - 1]),
                &mut proof,
            );
        }
        if let Some(right) = self.tree.leaves.get(index) {
            bytes_field(3, &encode_existence(right), &mut proof);
        }
        Ok(self.ops(key, 2, &proof))
    }

    pub fn encode(&self) -> Result<Vec<u8>, ChainProofError> {
        self.encode_key(&self.tree.leaves[0].key)
    }

    pub fn encode_key(&self, key: &[u8]) -> Result<Vec<u8>, ChainProofError> {
        let (value, proof_ops) = match self
            .tree
            .leaves
            .binary_search_by(|proof| proof.key.as_slice().cmp(key))
        {
            Ok(index) => (
                Some(self.tree.leaves[index].value.clone()),
                self.prove(key)?,
            ),
            Err(_) => (None, self.prove_absent(key)?),
        };
        CommitmentProof {
            signed_header: encode_header(&self.header),
            commit: encode_commit(&self.commit),
            validator_set: encode_validators(&self.verifier.validators),
            store_name: self.store_name.clone(),
            key: key.to_vec(),
            value,
            proof_ops,
        }
        .encode()
    }

    fn ops(&self, key: &[u8], variant: u32, proof: &[u8]) -> Vec<ProofOp> {
        vec![
            ProofOp {
                op_type: "ics23:iavl".to_owned(),
                key: key.to_vec(),
                data: message(variant, proof),
            },
            self.store_proof.clone(),
        ]
    }
}

#[derive(Clone, Copy)]
enum TreeKind {
    Iavl,
    Simple,
}

struct Tree {
    root: [u8; 32],
    height: u64,
    leaves: Vec<ExistenceProof>,
}

impl Tree {
    fn new(pairs: &[(Vec<u8>, Vec<u8>)], kind: TreeKind) -> Self {
        if let [(key, value)] = pairs {
            let prefix = match kind {
                TreeKind::Iavl => vec![0, 2, 2],
                TreeKind::Simple => vec![0],
            };
            let mut bytes = prefix.clone();
            varint(key.len() as u64, &mut bytes);
            bytes.extend_from_slice(key);
            varint(32, &mut bytes);
            bytes.extend_from_slice(&Sha256::digest(value));
            return Self {
                root: Sha256::digest(bytes).into(),
                height: 0,
                leaves: vec![ExistenceProof {
                    key: key.clone(),
                    value: value.clone(),
                    leaf: LeafOp {
                        hash: 1,
                        prehash_key: 0,
                        prehash_value: 1,
                        length: 1,
                        prefix,
                    },
                    path: Vec::new(),
                }],
            };
        }
        let split = match kind {
            TreeKind::Iavl => pairs.len() / 2,
            TreeKind::Simple => 1 << (usize::BITS - (pairs.len() - 1).leading_zeros() - 1),
        };
        let mut left = Self::new(&pairs[..split], kind);
        let mut right = Self::new(&pairs[split..], kind);
        let height = left.height.max(right.height) + 1;
        let (prefix, mut suffix, right_prefix) = match kind {
            TreeKind::Iavl => {
                let mut metadata = Vec::new();
                varint(height * 2, &mut metadata);
                varint(pairs.len() as u64 * 2, &mut metadata);
                varint(2, &mut metadata);
                metadata.push(32);
                let mut right_prefix = metadata.clone();
                right_prefix.extend_from_slice(&left.root);
                right_prefix.push(32);
                (metadata, vec![32], right_prefix)
            }
            TreeKind::Simple => {
                let mut right_prefix = vec![1];
                right_prefix.extend_from_slice(&left.root);
                (vec![1], Vec::new(), right_prefix)
            }
        };
        suffix.extend_from_slice(&right.root);
        let mut bytes = prefix.clone();
        bytes.extend_from_slice(&left.root);
        bytes.extend_from_slice(&suffix);
        for proof in &mut left.leaves {
            proof.path.push(InnerOp {
                hash: 1,
                prefix: prefix.clone(),
                suffix: suffix.clone(),
            });
        }
        for proof in &mut right.leaves {
            proof.path.push(InnerOp {
                hash: 1,
                prefix: right_prefix.clone(),
                suffix: Vec::new(),
            });
        }
        left.leaves.extend(right.leaves);
        Self {
            root: Sha256::digest(bytes).into(),
            height,
            leaves: left.leaves,
        }
    }
}

fn varint(mut value: u64, bytes: &mut Vec<u8>) {
    while value >= 128 {
        bytes.push((value.to_le_bytes()[0] & 127) | 128);
        value >>= 7;
    }
    bytes.push(value.to_le_bytes()[0]);
}

fn scalar_field(number: u32, value: u64, bytes: &mut Vec<u8>) {
    if value != 0 {
        varint(u64::from(number) << 3, bytes);
        varint(value, bytes);
    }
}

fn bytes_field(number: u32, value: &[u8], bytes: &mut Vec<u8>) {
    varint(u64::from(number) << 3 | 2, bytes);
    varint(value.len() as u64, bytes);
    bytes.extend_from_slice(value);
}

fn message(number: u32, value: &[u8]) -> Vec<u8> {
    let mut bytes = Vec::new();
    bytes_field(number, value, &mut bytes);
    bytes
}

fn encode_existence(proof: &ExistenceProof) -> Vec<u8> {
    let mut bytes = message(1, &proof.key);
    bytes_field(2, &proof.value, &mut bytes);
    let mut leaf = Vec::new();
    scalar_field(1, u64::from(proof.leaf.hash), &mut leaf);
    scalar_field(2, u64::from(proof.leaf.prehash_key), &mut leaf);
    scalar_field(3, u64::from(proof.leaf.prehash_value), &mut leaf);
    scalar_field(4, u64::from(proof.leaf.length), &mut leaf);
    bytes_field(5, &proof.leaf.prefix, &mut leaf);
    bytes_field(3, &leaf, &mut bytes);
    for op in &proof.path {
        let mut inner = Vec::new();
        scalar_field(1, u64::from(op.hash), &mut inner);
        bytes_field(2, &op.prefix, &mut inner);
        if !op.suffix.is_empty() {
            bytes_field(3, &op.suffix, &mut inner);
        }
        bytes_field(4, &inner, &mut bytes);
    }
    bytes
}

fn encode_time(seconds: i64, nanos: i32) -> Vec<u8> {
    let mut bytes = Vec::new();
    scalar_field(1, u64::from_le_bytes(seconds.to_le_bytes()), &mut bytes);
    scalar_field(2, u64::from(nanos.unsigned_abs()), &mut bytes);
    bytes
}

fn encode_block_id(hash: &[u8], total: u32, parts_hash: &[u8]) -> Vec<u8> {
    let mut parts = Vec::new();
    scalar_field(1, u64::from(total), &mut parts);
    bytes_field(2, parts_hash, &mut parts);
    let mut bytes = message(1, hash);
    bytes_field(2, &parts, &mut bytes);
    bytes
}

fn encode_header(header: &ChainHeader) -> Vec<u8> {
    let mut version = Vec::new();
    scalar_field(1, header.version.block, &mut version);
    scalar_field(2, header.version.app, &mut version);
    let mut bytes = message(1, &version);
    bytes_field(2, header.chain_id.as_bytes(), &mut bytes);
    scalar_field(3, header.height.unsigned_abs(), &mut bytes);
    bytes_field(
        4,
        &encode_time(header.time.seconds, header.time.nanos),
        &mut bytes,
    );
    bytes_field(
        5,
        &encode_block_id(
            &header.last_block_id.hash,
            header.last_block_id.part_set_total,
            &header.last_block_id.part_set_hash,
        ),
        &mut bytes,
    );
    for (number, hash) in (6..=14).zip([
        &header.last_commit_hash,
        &header.data_hash,
        &header.validators_hash,
        &header.next_validators_hash,
        &header.consensus_hash,
        &header.app_hash,
        &header.last_results_hash,
        &header.evidence_hash,
        &header.proposer_address,
    ]) {
        bytes_field(number, hash, &mut bytes);
    }
    bytes
}

fn encode_commit(commit: &Commit) -> Vec<u8> {
    let mut bytes = Vec::new();
    scalar_field(1, commit.height.unsigned_abs(), &mut bytes);
    scalar_field(2, u64::from(commit.round.unsigned_abs()), &mut bytes);
    bytes_field(
        3,
        &encode_block_id(
            &commit.block_id.hash,
            commit.block_id.part_set_header.total,
            &commit.block_id.part_set_header.hash,
        ),
        &mut bytes,
    );
    for signature in &commit.signatures {
        let mut sig = Vec::new();
        scalar_field(1, u64::from(signature.flag as u8), &mut sig);
        bytes_field(2, &signature.validator_address, &mut sig);
        bytes_field(
            3,
            &encode_time(signature.timestamp.seconds, signature.timestamp.nanos),
            &mut sig,
        );
        bytes_field(4, &signature.signature, &mut sig);
        bytes_field(4, &sig, &mut bytes);
    }
    bytes
}

fn encode_validator(validator: &TrustedValidator) -> Vec<u8> {
    let mut bytes = message(1, &validator.address);
    bytes_field(2, &message(1, &validator.public_key), &mut bytes);
    scalar_field(3, validator.voting_power, &mut bytes);
    bytes
}

fn encode_validators(validators: &ValidatorSet) -> Vec<u8> {
    let mut bytes = Vec::new();
    for validator in validators.validators() {
        bytes_field(1, &encode_validator(validator), &mut bytes);
    }
    bytes_field(
        2,
        &encode_validator(&validators.validators()[0]),
        &mut bytes,
    );
    scalar_field(3, validators.total_power(), &mut bytes);
    bytes
}
