//! Read-only adapter from one node's consensus RPC to the AI runtime capture route.
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::time::{Duration, Instant};

use layerx_committed_proof::{
    commit::{BlockIdFlag, Commit, CommitSig},
    header::{BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp},
};
use layerx_programs_ai_market::{
    codec::{self, ChunkRequest, Reader},
    queries::read_state_chunk,
    types::{ChainDomain, Presence, ProgramId},
    MAX_STATE_BYTES, SHARED_STATE_KEY,
};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

use crate::ai_market::BlockCommitProof;
use crate::ai_runtime::encode_committed_read;
use crate::codec::{base64_decode, hex, hex0x, unhex, unhex_fixed};
use crate::transport::{Endpoint, Security};
use crate::IndexError;

const NODE_TIMEOUT: Duration = Duration::from_secs(1);
const IO_TIMEOUT: Duration = Duration::from_secs(2);
const REQUEST_MAX_BYTES: usize = 4096;
const ARTIFACT_MAX_BYTES: usize = 1_048_576;

/// Environment settings for a single program and a single consensus RPC endpoint.
pub struct SourceConfig {
    pub listen: SocketAddr,
    pub node: Endpoint,
    pub chain: ChainDomain,
    pub program: ProgramId,
}

impl SourceConfig {
    /// Load process settings without any signing credentials.
    /// # Errors
    /// Refuses missing settings, invalid identifiers, remote plaintext and bad trust anchors.
    pub fn from_env() -> Result<Self, IndexError> {
        Self::from_lookup(&|name| std::env::var(name).ok())
    }

    /// Load the same settings through a caller-owned lookup.
    /// # Errors
    /// Refuses incomplete settings or an endpoint without the required transport security.
    pub fn from_lookup(lookup: &impl Fn(&str) -> Option<String>) -> Result<Self, IndexError> {
        let required = |name| {
            lookup(name)
                .filter(|s| !s.is_empty())
                .ok_or_else(|| IndexError::Config(format!("missing {name}")))
        };
        let listen = required("LAYERX_AI_SOURCE_LISTEN")?
            .parse()
            .map_err(|_| IndexError::Config("invalid AI source listener".to_owned()))?;
        let security = match lookup("LAYERX_AI_SOURCE_NODE_CA_DER").filter(|s| !s.is_empty()) {
            Some(path) => {
                let bytes = std::fs::File::open(path)
                    .and_then(|file| {
                        let mut bytes = Vec::new();
                        file.take(65_537).read_to_end(&mut bytes)?;
                        Ok(bytes)
                    })
                    .map_err(|error| IndexError::Source(error.to_string()))?;
                if bytes.len() > 65_536 {
                    return Err(IndexError::Config(
                        "AI source trust anchor too large".to_owned(),
                    ));
                }
                Security::PinnedTls(bytes)
            }
            None => Security::Plaintext {
                allow_remote: false,
            },
        };
        let node = Endpoint::parse(
            &required("LAYERX_AI_SOURCE_NODE_URL")?,
            security,
            NODE_TIMEOUT,
        )?;
        let chain = ChainDomain::new(unhex_fixed(&required("LAYERX_AI_SOURCE_CHAIN")?)?)
            .map_err(|_| invalid())?;
        let program = ProgramId::new(unhex_fixed(&required("LAYERX_AI_SOURCE_PROGRAM")?)?)
            .map_err(|_| invalid())?;
        Ok(Self {
            listen,
            node,
            chain,
            program,
        })
    }
}

/// Run a bounded, serial HTTP listener: at most one accepted connection is held.
/// Deploy beside the indexer on a private interface; the indexer pins consensus authority.
/// # Errors
/// Returns listener failures. Individual malformed or unavailable reads get HTTP refusals.
pub fn serve(listener: &TcpListener, config: &SourceConfig) -> Result<(), IndexError> {
    for connection in listener.incoming() {
        let mut stream = connection.map_err(|error| IndexError::Source(error.to_string()))?;
        let response = match request_target(&mut stream) {
            Ok(target) => respond(config, &target),
            Err(_) => (400, Vec::new()),
        };
        // A disconnected client cannot stop the listener or occupy it indefinitely.
        let _ = write_response(&mut stream, response.0, &response.1);
    }
    Ok(())
}

/// Handle the exact GET target used by `AiRuntime::poll`.
/// Upstream failure is never converted into an idle response.
#[must_use]
pub fn respond(config: &SourceConfig, target: &str) -> (u16, Vec<u8>) {
    let (path, query) = target.split_once('?').unwrap_or((target, ""));
    if path
        != format!(
            "/v1/ai/programs/{}/committed-read",
            hex(config.program.as_bytes())
        )
    {
        return (404, Vec::new());
    }
    let Some(after) = query
        .strip_prefix("after=")
        .filter(|s| !s.is_empty() && s.len() <= 20 && s.bytes().all(|b| b.is_ascii_digit()))
        .and_then(|s| s.parse::<u64>().ok())
    else {
        return (400, Vec::new());
    };
    match committed_read(config) {
        Ok((sequence, body)) if sequence > after => (200, body),
        Ok(_) => (204, Vec::new()),
        Err(_) => (502, Vec::new()),
    }
}

fn invalid() -> IndexError {
    IndexError::Source("invalid AI source node material".to_owned())
}
fn text(value: &Value) -> Result<&str, IndexError> {
    value.as_str().ok_or_else(invalid)
}
fn number(value: &Value) -> Result<u64, IndexError> {
    value
        .as_u64()
        .or_else(|| value.as_str()?.parse().ok())
        .ok_or_else(invalid)
}
fn bytes(value: &Value) -> Result<Vec<u8>, IndexError> {
    let s = text(value)?;
    if s.len() > 2 * ARTIFACT_MAX_BYTES {
        return Err(invalid());
    }
    let bytes = base64_decode(s)?;
    if bytes.len() > ARTIFACT_MAX_BYTES {
        return Err(invalid());
    }
    Ok(bytes)
}

fn committed_read(config: &SourceConfig) -> Result<(u64, Vec<u8>), IndexError> {
    let status = config.node.rpc("status", &json!({}))?;
    let height = number(&status["sync_info"]["latest_block_height"])?;
    let sequence = height
        .checked_sub(1)
        .filter(|h| *h > 0)
        .ok_or_else(invalid)?;
    let material = config
        .node
        .rpc("commit", &json!({"height": height.to_string()}))?;
    if material["canonical"] != true {
        return Err(invalid());
    }
    let (header, commit) = decode_commit(&material["signed_header"])?;
    if u64::try_from(header.height).ok() != Some(height)
        || commit.height != height
        || commit.block_id.hash != header.hash()
    {
        return Err(invalid());
    }
    let mut key = b"\x01\x00\x09progstor".to_vec();
    key.extend(config.program.as_bytes());
    key.push(1);
    let (namespace, ops) = artifact(&config.node, &key, sequence, true)?;
    if namespace.len() != 38 || namespace[..2] != [0, 1] {
        return Err(invalid());
    }
    let blob_key = |digest: &[u8]| [&[5, 0, 9][..], digest].concat();
    let (manifest, _) = artifact(&config.node, &blob_key(&namespace[6..]), sequence, false)?;
    if Sha256::digest(&manifest).as_slice() != &namespace[6..] {
        return Err(invalid());
    }
    let (digest, length) = state_entry(&namespace, &manifest)?;
    let (state, _) = artifact(&config.node, &blob_key(&digest), sequence, false)?;
    if state.len() != length || Sha256::digest(&state).as_slice() != digest {
        return Err(invalid());
    }
    layerx_programs_ai_market::queries::read_header(&state, config.chain, config.program)
        .map_err(|_| invalid())?;
    let store_proof = serde_json::to_vec(&json!({
        "membership": { "proof": unwrap_existence(&ops[0])?, "multistore": ops[1] },
        "manifest": manifest
    }))
    .map_err(|_| invalid())?;
    let chunks = state_chunks(&state)?;
    let body = encode_committed_read(
        config.chain,
        config.program,
        sequence,
        &BlockCommitProof {
            header: &header,
            commit: &commit,
            store_proof: &store_proof,
        },
        &chunks,
    )
    .map_err(|_| invalid())?;
    Ok((sequence, body))
}

fn state_entry(namespace: &[u8], manifest: &[u8]) -> Result<([u8; 32], usize), IndexError> {
    if manifest.get(..6) != namespace.get(..6) {
        return Err(invalid());
    }
    let mut reader = Reader::new(manifest);
    if reader.u16().map_err(|_| invalid())? != 1 {
        return Err(invalid());
    }
    let count = reader.u32().map_err(|_| invalid())?;
    if count == 0 || count > 65_536 {
        return Err(invalid());
    }
    let mut found = None;
    let mut previous = &[][..];
    for _ in 0..count {
        let length = usize::from(reader.u16().map_err(|_| invalid())?);
        let name = reader.take(length).map_err(|_| invalid())?;
        let digest = reader.fixed::<32>().map_err(|_| invalid())?;
        let length =
            usize::try_from(reader.u32().map_err(|_| invalid())?).map_err(|_| invalid())?;
        if name.is_empty() || name.len() > 1024 || name <= previous {
            return Err(invalid());
        }
        previous = name;
        if name == SHARED_STATE_KEY {
            if length == 0 || length > MAX_STATE_BYTES {
                return Err(invalid());
            }
            found = Some((digest, length));
        }
    }
    reader.finish().map_err(|_| invalid())?;
    found.ok_or_else(invalid)
}

type Artifact = (Vec<u8>, Vec<Vec<u8>>);
fn artifact(node: &Endpoint, key: &[u8], height: u64, prove: bool) -> Result<Artifact, IndexError> {
    let result = node.rpc(
        "abci_query",
        &json!({
            "path": "/store/lxphost/key", "data": hex0x(key),
            "height": height.to_string(), "prove": prove
        }),
    )?;
    let response = &result["response"];
    if response["code"].as_u64() != Some(0)
        || number(&response["height"])? != height
        || bytes(&response["key"])? != key
    {
        return Err(invalid());
    }
    let value = bytes(&response["value"])?;
    if value.is_empty() {
        return Err(invalid());
    }
    let mut ops = Vec::new();
    if prove {
        let rows = response["proofOps"]["ops"]
            .as_array()
            .filter(|r| r.len() == 2)
            .ok_or_else(invalid)?;
        for (row, (kind, expected)) in rows
            .iter()
            .zip([("ics23:iavl", key), ("ics23:simple", &b"lxphost"[..])])
        {
            if row["type"] != kind || bytes(&row["key"])? != expected {
                return Err(invalid());
            }
            ops.push(bytes(&row["data"])?);
        }
        // Both proofs must contain exactly one ICS23 existence message.
        for op in &ops {
            unwrap_existence(op)?;
        }
    }
    Ok((value, ops))
}

fn unwrap_existence(bytes: &[u8]) -> Result<&[u8], IndexError> {
    if bytes.first() != Some(&10) {
        return Err(invalid());
    }
    let mut length = 0usize;
    for (i, byte) in bytes[1..].iter().copied().take(5).enumerate() {
        length |= usize::from(byte & 127) << (7 * i);
        if byte & 128 == 0 {
            if (i > 0 && byte == 0) || length == 0 || length != bytes.len() - i - 2 {
                return Err(invalid());
            }
            return Ok(&bytes[i + 2..]);
        }
    }
    Err(invalid())
}

fn state_chunks(state: &[u8]) -> Result<Vec<Vec<u8>>, IndexError> {
    if state.len() > MAX_STATE_BYTES {
        return Err(invalid());
    }
    let mut offset = 0;
    let mut revision = 0;
    let mut digest = Presence::Absent;
    let mut chunks = Vec::new();
    while usize::try_from(offset).map_err(|_| invalid())? < state.len() {
        let mut payload = [0; 46];
        codec::encode_chunk_request(
            &ChunkRequest {
                revision,
                digest,
                offset,
                requested: 8192,
            },
            &mut payload,
        )
        .map_err(|_| invalid())?;
        let mut body = vec![0; 8244];
        let length = read_state_chunk(state, &payload, &mut body).map_err(|_| invalid())?;
        body.truncate(length);
        let chunk = codec::decode_chunk_response(&body).map_err(|_| invalid())?;
        let advance = u32::try_from(chunk.bytes.len()).map_err(|_| invalid())?;
        if advance == 0 {
            return Err(invalid());
        }
        offset = offset.checked_add(advance).ok_or_else(invalid)?;
        revision = chunk.revision;
        digest = Presence::Present(chunk.digest);
        chunks.push(body);
    }
    Ok(chunks)
}

fn hex_bytes(value: &Value, length: usize, empty: bool) -> Result<Vec<u8>, IndexError> {
    let s = text(value)?;
    if s.len() != length * 2 && !(empty && s.is_empty()) {
        return Err(invalid());
    }
    unhex(s)
}
fn block_id(value: &Value) -> Result<BlockId, IndexError> {
    Ok(BlockId {
        hash: hex_bytes(&value["hash"], 32, true)?,
        part_set_header: PartSetHeader {
            total: u32::try_from(number(&value["parts"]["total"])?).map_err(|_| invalid())?,
            hash: hex_bytes(&value["parts"]["hash"], 32, true)?,
        },
    })
}

fn decode_commit(signed: &Value) -> Result<(Header, Commit), IndexError> {
    let h = &signed["header"];
    if text(&h["chain_id"])?.is_empty() || text(&h["chain_id"])?.len() > 50 {
        return Err(invalid());
    }
    let header = Header {
        version: ConsensusVersion {
            block: number(&h["version"]["block"])?,
            app: number(&h["version"]["app"])?,
        },
        chain_id: text(&h["chain_id"])?.to_owned(),
        height: i64::try_from(number(&h["height"])?).map_err(|_| invalid())?,
        time: timestamp(&h["time"])?,
        last_block_id: block_id(&h["last_block_id"])?,
        last_commit_hash: hex_bytes(&h["last_commit_hash"], 32, true)?,
        data_hash: hex_bytes(&h["data_hash"], 32, true)?,
        validators_hash: unhex_fixed(text(&h["validators_hash"])?)?,
        next_validators_hash: hex_bytes(&h["next_validators_hash"], 32, false)?,
        consensus_hash: hex_bytes(&h["consensus_hash"], 32, false)?,
        app_hash: hex_bytes(&h["app_hash"], 32, false)?,
        last_results_hash: hex_bytes(&h["last_results_hash"], 32, true)?,
        evidence_hash: hex_bytes(&h["evidence_hash"], 32, true)?,
        proposer_address: hex_bytes(&h["proposer_address"], 20, false)?,
    };
    let c = &signed["commit"];
    let rows = c["signatures"]
        .as_array()
        .filter(|r| !r.is_empty() && r.len() <= 1024)
        .ok_or_else(invalid)?;
    let signatures = rows
        .iter()
        .map(|row| {
            let block_id_flag = match number(&row["block_id_flag"])? {
                1 => BlockIdFlag::Absent,
                2 => BlockIdFlag::Commit,
                3 => BlockIdFlag::Nil,
                _ => return Err(invalid()),
            };
            let signature = if block_id_flag == BlockIdFlag::Absent {
                if !row["signature"].is_null() || row["validator_address"] != "" {
                    return Err(invalid());
                }
                None
            } else {
                Some(
                    bytes(&row["signature"])?
                        .try_into()
                        .map_err(|_| invalid())?,
                )
            };
            Ok(CommitSig {
                block_id_flag,
                validator_address: hex_bytes(
                    &row["validator_address"],
                    20,
                    block_id_flag == BlockIdFlag::Absent,
                )?,
                timestamp: timestamp(&row["timestamp"])?,
                signature,
            })
        })
        .collect::<Result<Vec<_>, IndexError>>()?;
    Ok((
        header,
        Commit {
            height: number(&c["height"])?,
            round: i32::try_from(number(&c["round"])?).map_err(|_| invalid())?,
            block_id: block_id(&c["block_id"])?,
            signatures,
        },
    ))
}

fn timestamp(value: &Value) -> Result<Timestamp, IndexError> {
    let s = text(value)?;
    let digits = |start, end| -> Result<i64, IndexError> {
        let part = s
            .get(start..end)
            .filter(|p| p.bytes().all(|b| b.is_ascii_digit()))
            .ok_or_else(invalid)?;
        part.parse().map_err(|_| invalid())
    };
    if s.len() < 20
        || s.get(4..5) != Some("-")
        || s.get(7..8) != Some("-")
        || s.get(10..11) != Some("T")
        || s.get(13..14) != Some(":")
        || s.get(16..17) != Some(":")
        || !s.ends_with('Z')
    {
        return Err(invalid());
    }
    let (mut year, month, day) = (digits(0, 4)?, digits(5, 7)?, digits(8, 10)?);
    let leap = year % 4 == 0 && (year % 100 != 0 || year % 400 == 0);
    let days = [
        31,
        if leap { 29 } else { 28 },
        31,
        30,
        31,
        30,
        31,
        31,
        30,
        31,
        30,
        31,
    ];
    if !(1..=12).contains(&month)
        || day < 1
        || day > days[usize::try_from(month - 1).map_err(|_| invalid())?]
    {
        return Err(invalid());
    }
    let (hour, minute, second) = (digits(11, 13)?, digits(14, 16)?, digits(17, 19)?);
    if hour > 23 || minute > 59 || second > 59 {
        return Err(invalid());
    }
    let nanos = if s.len() == 20 {
        0
    } else {
        let fraction = s
            .get(20..s.len() - 1)
            .filter(|p| !p.is_empty() && p.len() <= 9 && p.bytes().all(|b| b.is_ascii_digit()))
            .ok_or_else(invalid)?;
        if s.get(19..20) != Some(".") {
            return Err(invalid());
        }
        fraction.parse::<u32>().map_err(|_| invalid())?
            * 10u32.pow(9 - u32::try_from(fraction.len()).map_err(|_| invalid())?)
    };
    year -= i64::from(month <= 2);
    let era = year.div_euclid(400);
    let yoe = year - era * 400;
    let mp = month + if month > 2 { -3 } else { 9 };
    let doy = (153 * mp + 2) / 5 + day - 1;
    let days = era * 146_097 + yoe * 365 + yoe / 4 - yoe / 100 + doy - 719_468;
    let seconds = days * 86_400 + hour * 3_600 + minute * 60 + second;
    if !(-62_135_596_800..=253_402_300_799).contains(&seconds) {
        return Err(invalid());
    }
    Ok(Timestamp { seconds, nanos })
}

fn remaining(deadline: Instant) -> Result<Duration, IndexError> {
    deadline
        .checked_duration_since(Instant::now())
        .filter(|d| !d.is_zero())
        .ok_or_else(invalid)
}
fn request_target(stream: &mut TcpStream) -> Result<String, IndexError> {
    let deadline = Instant::now() + IO_TIMEOUT;
    let mut head = Vec::new();
    while !head.ends_with(b"\r\n\r\n") {
        if head.len() >= REQUEST_MAX_BYTES {
            return Err(invalid());
        }
        stream
            .set_read_timeout(Some(remaining(deadline)?))
            .map_err(|error| IndexError::Source(error.to_string()))?;
        let mut byte = [0];
        stream
            .read_exact(&mut byte)
            .map_err(|error| IndexError::Source(error.to_string()))?;
        head.push(byte[0]);
    }
    let head = std::str::from_utf8(&head).map_err(|_| invalid())?;
    let mut lines = head.split("\r\n");
    let mut request = lines.next().ok_or_else(invalid)?.split(' ');
    if request.next() != Some("GET") {
        return Err(invalid());
    }
    let target = request.next().ok_or_else(invalid)?;
    if request.next() != Some("HTTP/1.1")
        || request.next().is_some()
        || target
            .bytes()
            .any(|b| b.is_ascii_control() || b.is_ascii_whitespace())
    {
        return Err(invalid());
    }
    let mut length_seen = false;
    for line in lines.filter(|l| !l.is_empty()) {
        let (name, value) = line.split_once(':').ok_or_else(invalid)?;
        if name.eq_ignore_ascii_case("transfer-encoding") {
            return Err(invalid());
        }
        if name.eq_ignore_ascii_case("content-length") {
            if length_seen || value.trim() != "0" {
                return Err(invalid());
            }
            length_seen = true;
        }
    }
    Ok(target.to_owned())
}
fn write_response(stream: &mut TcpStream, status: u16, body: &[u8]) -> Result<(), IndexError> {
    let deadline = Instant::now() + IO_TIMEOUT;
    let reason = match status {
        200 => "OK",
        204 => "No Content",
        400 => "Bad Request",
        404 => "Not Found",
        _ => "Bad Gateway",
    };
    let head = format!("HTTP/1.1 {status} {reason}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len());
    for part in [head.as_bytes(), body] {
        for mut chunk in part.chunks(16 * 1024) {
            while !chunk.is_empty() {
                stream
                    .set_write_timeout(Some(remaining(deadline)?))
                    .map_err(|error| IndexError::Source(error.to_string()))?;
                let count = stream
                    .write(chunk)
                    .map_err(|error| IndexError::Source(error.to_string()))?;
                if count == 0 {
                    return Err(invalid());
                }
                chunk = &chunk[count..];
            }
        }
    }
    Ok(())
}
