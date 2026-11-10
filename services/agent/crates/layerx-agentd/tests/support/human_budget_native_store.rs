use super::{hex, must, RecordedAuthority, Value};
use serde_json::json;
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

fn hash(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}

fn varint(mut value: u64) -> Vec<u8> {
    let mut bytes = Vec::new();
    while value >= 128 {
        bytes.push(assert_ok(u8::try_from(value & 127), "seven-bit varint byte") | 128);
        value >>= 7;
    }
    bytes.push(assert_ok(u8::try_from(value), "terminal varint byte"));
    bytes
}

fn field(number: u8, value: &[u8]) -> Vec<u8> {
    [
        vec![(number << 3) | 2],
        varint(assert_ok(u64::try_from(value.len()), "field length")),
        value.to_vec(),
    ]
    .concat()
}

fn leaf_hash(prefix: &[u8], key: &[u8], value: &[u8]) -> [u8; 32] {
    hash(
        &[
            prefix.to_vec(),
            varint(assert_ok(u64::try_from(key.len()), "key length")),
            key.to_vec(),
            vec![32],
            hash(value).to_vec(),
        ]
        .concat(),
    )
}

fn leaf_proof(key: &[u8], value: &[u8], prefix: &[u8], path: &[(Vec<u8>, Vec<u8>)]) -> Vec<u8> {
    let leaf = [vec![8, 1, 24, 1, 32, 1], field(5, prefix)].concat();
    let mut existence = [field(1, key), field(2, value), field(3, &leaf)].concat();
    for (prefix, suffix) in path {
        let inner = [vec![8, 1], field(2, prefix), field(3, suffix)].concat();
        existence.extend(field(4, &inner));
    }
    field(1, &existence)
}

type MembershipPath = Vec<(Vec<u8>, Vec<u8>)>;
type StoreRow = (Vec<u8>, Vec<u8>, MembershipPath);

struct StoreNode {
    root: [u8; 32],
    height: u64,
    size: u64,
    rows: Vec<StoreRow>,
}

impl StoreNode {
    fn build(rows: &[(Vec<u8>, Vec<u8>)]) -> Self {
        if rows.len() == 1 {
            let (key, value) = &rows[0];
            return Self {
                root: leaf_hash(&[0, 2, 2], key, value),
                height: 0,
                size: 1,
                rows: vec![(key.clone(), value.clone(), Vec::new())],
            };
        }
        let (left, right) = rows.split_at(rows.len() / 2);
        let mut left = Self::build(left);
        let mut right = Self::build(right);
        let height = 1 + left.height.max(right.height);
        let size = left.size + right.size;
        let prefix = [varint(height * 2), varint(size * 2), vec![2, 32]].concat();
        let suffix = [vec![32], right.root.to_vec()].concat();
        let root = hash(&[prefix.clone(), left.root.to_vec(), suffix.clone()].concat());
        for (_, _, path) in &mut left.rows {
            path.push((prefix.clone(), suffix.clone()));
        }
        let right_prefix = [prefix, left.root.to_vec(), vec![32]].concat();
        for (_, _, path) in &mut right.rows {
            path.push((right_prefix.clone(), vec![]));
        }
        left.rows.extend(right.rows);
        Self {
            root,
            height,
            size,
            rows: left.rows,
        }
    }
}

pub(super) struct NativeStore {
    pub app_hash: [u8; 32],
    pub queries: BTreeMap<String, Value>,
}

impl NativeStore {
    fn commit(mut rows: Vec<(Vec<u8>, Vec<u8>)>) -> Self {
        rows.sort_by(|a, b| a.0.cmp(&b.0));
        assert!(rows.windows(2).all(|pair| pair[0].0 != pair[1].0));
        let tree = StoreNode::build(&rows);
        let outer = leaf_proof(b"lxphost", &tree.root, &[0], &[]);
        let app_hash = leaf_hash(&[0], b"lxphost", &tree.root);
        let queries = tree
            .rows
            .into_iter()
            .map(|(key, value, path)| {
                let proof = leaf_proof(&key, &value, &[0, 2, 2], &path);
                // The in-memory peer commits real ICS23 membership, verified by the production codec.
                must(layerx_platform_authority::committed::verify_membership(
                    &key, &value, &proof, &outer, app_hash,
                ));
                (
                    hex::encode(&key),
                    json!({"code":0,"key":encode_base64(&key),"value":encode_base64(&value),
                "proofOps":{"ops":[{"type":"ics23:iavl","key":encode_base64(&key),"data":encode_base64(&proof)},
                {"type":"ics23:simple","key":encode_base64(b"lxphost"),"data":encode_base64(&outer)}]}}),
                )
            })
            .collect();
        Self { app_hash, queries }
    }
}

fn encode_base64(bytes: &[u8]) -> String {
    const ALPHABET: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut encoded = String::new();
    for chunk in bytes.chunks(3) {
        let word = (u32::from(chunk[0]) << 16)
            | (u32::from(*chunk.get(1).unwrap_or(&0)) << 8)
            | u32::from(*chunk.get(2).unwrap_or(&0));
        encoded.push(char::from(
            ALPHABET[assert_ok(usize::try_from(word >> 18), "six-bit base64 index")],
        ));
        encoded.push(char::from(
            ALPHABET[assert_ok(usize::try_from((word >> 12) & 63), "six-bit base64 index")],
        ));
        encoded.push(if chunk.len() > 1 {
            char::from(
                ALPHABET[assert_ok(usize::try_from((word >> 6) & 63), "six-bit base64 index")],
            )
        } else {
            '='
        });
        encoded.push(if chunk.len() > 2 {
            char::from(ALPHABET[assert_ok(usize::try_from(word & 63), "six-bit base64 index")])
        } else {
            '='
        });
    }
    encoded
}

pub(super) fn prepare_authority(fixture: &mut RecordedAuthority) {
    let owner_name = format!("agent:{}:main", fixture.agent_did);
    let budget_name = format!(
        "agent:{}:budget:{}",
        fixture.agent_did,
        hex::encode(fixture.budget_id)
    );
    for (index, name) in [(1, owner_name), (2, budget_name)] {
        let account = must(layerx_types::account::AccountId::parse(&name));
        let id = must(layerx_wire::hash::account_id_for_protocol(&account, 3));
        fixture.budget_call[index * 32..(index + 1) * 32].copy_from_slice(&id);
    }
    fixture.balance_context["account_id"] = json!(hex::encode(&fixture.budget_call[32..64]));
}

impl NativeStore {
    pub fn for_authority(fixture: &RecordedAuthority) -> Self {
        use layerx_wire::encode::Encoder;
        let sequence = assert_ok(
            u64::from_str_radix(
                assert_some(fixture.block["number"].as_str(), "expected Some value")
                    .trim_start_matches("0x"),
                16,
            ),
            "unexpected error",
        );
        let mut rows = vec![(vec![2], sequence.to_be_bytes().to_vec())];
        let word = |index: usize| -> [u8; 32] {
            assert_ok(
                fixture.budget_call[index * 32..(index + 1) * 32].try_into(),
                "unexpected error",
            )
        };
        let owner = word(1);
        let budget = word(2);
        let asset = word(3);
        let source = word(18);
        let mut accounts = vec![owner, budget];
        if source != [0; 32] && !accounts.contains(&source) {
            accounts.push(source);
        }
        for id in accounts {
            let name = if id == owner {
                format!("agent:{}:main", fixture.agent_did)
            } else {
                format!(
                    "agent:{}:budget:{}",
                    fixture.agent_did,
                    hex::encode(fixture.budget_id)
                )
            };
            let mut writer = Encoder::new(2048);
            must(writer.bytes(&id, 32));
            must(writer.bytes(name.as_bytes(), 512));
            must(writer.u8(if id == owner { 1 } else { 2 }));
            must(writer.u128(77));
            must(writer.bytes(&asset, 32));
            must(writer.u8(1));
            must(writer.u64(2));
            must(writer.u64(1));
            must(writer.u8(0));
            must(writer.u8(0));
            must(writer.bytes(&[0x61; 32], 32));
            must(writer.u8(1));
            let mut key = vec![6];
            key.extend_from_slice(&id);
            rows.push((key, writer.finish()));
        }
        Self::commit(rows)
    }
}

#[track_caller]
fn assert_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{message}: {error:?}"),
    }
}

#[track_caller]
fn assert_some<T>(value: Option<T>, message: &str) -> T {
    match value {
        Some(value) => value,
        None => panic!("{message}"),
    }
}
