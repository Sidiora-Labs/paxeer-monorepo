#![deny(unsafe_code)]

use std::fmt::Write as _;
use std::fs;
use std::io::{self, Read};
use std::path::PathBuf;

use layerx_mirror::committed::ReceiptKeyConfig;
use layerx_mirror::ethereum::EthereumMirrorReadConfig;
use layerx_mirror::rpc::RpcQuorumConfig;
use layerx_mirror::solana::SolanaMirrorReadConfig;
use layerx_mirror::source::{
    MirrorLocator, MirrorReadPolicy, MirrorSource, MirrorSourceId, MirrorSources,
};
use layerx_mirror::{ArchiveCommitment, CommittedCoordinate, MirrorVerifier};
use layerx_proof::inclusion::SequencerAuthorization;
use serde::Deserialize;
use serde_json::{json, Value};

const MAX_REQUEST_BYTES: u64 = 40 * 1024 * 1024;

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Config {
    layerx_network_id: u32,
    receipt_keys: Vec<ReceiptKeyConfig>,
    sources: Vec<SourceConfig>,
}

#[derive(Deserialize)]
#[serde(tag = "kind", rename_all = "kebab-case", deny_unknown_fields)]
enum SourceConfig {
    Ethereum {
        id: String,
        rpc: RpcQuorumConfig,
        chain_id: u64,
        genesis_hash_hex: String,
        archive_contract_hex: String,
        archive_code_hash_hex: String,
        publisher_hex: String,
    },
    Solana {
        id: String,
        rpc: RpcQuorumConfig,
        genesis_hash_hex: String,
        archive_program_hex: String,
        upgradeable_loader_hex: String,
        program_data_account_hex: String,
        program_code_hash_hex: String,
        publisher_hex: String,
    },
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Request {
    batch_number: String,
    evidence: EvidenceRequest,
    policy: PolicyRequest,
}

#[derive(Deserialize)]
#[serde(tag = "kind", rename_all = "kebab-case", deny_unknown_fields)]
enum EvidenceRequest {
    Receipt { index: usize },
    Committed,
}

#[derive(Deserialize)]
#[serde(tag = "kind", rename_all = "kebab-case", deny_unknown_fields)]
enum PolicyRequest {
    Exact {
        candidate: Candidate,
    },
    OrderedPreference {
        candidates: Vec<Candidate>,
    },
    Agreement {
        candidates: Vec<Candidate>,
        minimum: usize,
    },
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Candidate {
    source: usize,
    commitment_hex: String,
}

fn main() {
    let result = execute();
    let response = match result {
        Ok(value) => json!({ "ok": true, "verification": value }),
        Err(code) => json!({ "ok": false, "error": code }),
    };
    println!("{response}");
}

fn execute() -> Result<Value, &'static str> {
    let path = std::env::args_os()
        .nth(1)
        .map(PathBuf::from)
        .ok_or("configuration")?;
    let metadata = fs::metadata(&path).map_err(|_| "configuration")?;
    if !metadata.is_file() || metadata.len() > 1024 * 1024 {
        return Err("configuration");
    }
    let config: Config = serde_json::from_slice(&fs::read(path).map_err(|_| "configuration")?)
        .map_err(|_| "configuration")?;
    let mut request_bytes = Vec::new();
    io::stdin()
        .take(MAX_REQUEST_BYTES + 1)
        .read_to_end(&mut request_bytes)
        .map_err(|_| "malformed")?;
    if request_bytes.len() as u64 > MAX_REQUEST_BYTES {
        return Err("bounds");
    }
    let request: Request = serde_json::from_slice(&request_bytes).map_err(|_| "malformed")?;
    if config.receipt_keys.is_empty() {
        return Err("configuration");
    }
    let keys = config
        .receipt_keys
        .iter()
        .map(|key| {
            if key.first_height > key.last_height {
                return Err("configuration");
            }
            Ok(SequencerAuthorization::new(
                fixed_hex(&key.sequencer_id_hex)?,
                fixed_hex(&key.public_key_hex)?,
                key.first_height,
                key.last_height,
            ))
        })
        .collect::<Result<Vec<_>, &'static str>>()?;
    let sources = configured_sources(config)?;
    let policy = policy(&request.policy)?;
    let batch_number = canonical_u64(&request.batch_number)?;
    let archive = sources
        .read(batch_number, &policy)
        .map_err(|error| match error {
            layerx_mirror::source::MirrorSourceError::Divergent => "divergent",
            layerx_mirror::source::MirrorSourceError::InsufficientAgreement => {
                "insufficient-agreement"
            }
            layerx_mirror::source::MirrorSourceError::Missing => "missing",
            layerx_mirror::source::MirrorSourceError::RateLimited { .. } => "rate-limited",
            layerx_mirror::source::MirrorSourceError::RpcDivergent => "rpc-divergent",
            _ => "source-unavailable",
        })?;
    let evidence_checker =
        MirrorVerifier::from_committed_source(archive).map_err(|_| "verification")?;
    let committed = evidence_checker.committed().map_err(|_| "verification")?;
    match request.evidence {
        EvidenceRequest::Receipt { index } => {
            let verified = evidence_checker
                .committed_receipt(index, &keys)
                .map_err(|_| "verification")?;
            let observation = verified.observation().ok_or("verification")?;
            let unsigned = layerx_wire::receipt::encode_unsigned(verified.value())
                .map_err(|_| "verification")?;
            let digest =
                layerx_wire::hash::receipt_digest(&unsigned).map_err(|_| "verification")?;
            Ok(report(observation, committed, Some(digest)))
        }
        EvidenceRequest::Committed => {
            let observation = evidence_checker.observation().ok_or("verification")?;
            Ok(report(observation, committed, None))
        }
    }
}

fn configured_sources(config: Config) -> Result<MirrorSources, &'static str> {
    let mut sources = Vec::with_capacity(config.sources.len());
    for source in config.sources {
        sources.push(match source {
            SourceConfig::Ethereum {
                id,
                rpc,
                chain_id,
                genesis_hash_hex,
                archive_contract_hex,
                archive_code_hash_hex,
                publisher_hex,
            } => MirrorSource::ethereum(
                MirrorSourceId::new(id).map_err(|_| "configuration")?,
                EthereumMirrorReadConfig {
                    rpc,
                    chain_id,
                    genesis_hash: fixed_hex(&genesis_hash_hex)?,
                    archive_contract: fixed_hex(&archive_contract_hex)?,
                    archive_code_hash: fixed_hex(&archive_code_hash_hex)?,
                    publisher: fixed_hex(&publisher_hex)?,
                },
            )
            .map_err(|_| "source-unavailable")?,
            SourceConfig::Solana {
                id,
                rpc,
                genesis_hash_hex,
                archive_program_hex,
                upgradeable_loader_hex,
                program_data_account_hex,
                program_code_hash_hex,
                publisher_hex,
            } => MirrorSource::solana(
                MirrorSourceId::new(id).map_err(|_| "configuration")?,
                SolanaMirrorReadConfig {
                    rpc,
                    genesis_hash: fixed_hex(&genesis_hash_hex)?,
                    archive_program: fixed_hex(&archive_program_hex)?,
                    upgradeable_loader: fixed_hex(&upgradeable_loader_hex)?,
                    program_data_account: fixed_hex(&program_data_account_hex)?,
                    program_code_hash: fixed_hex(&program_code_hash_hex)?,
                    publisher: fixed_hex(&publisher_hex)?,
                },
            )
            .map_err(|_| "source-unavailable")?,
        });
    }
    let sources =
        MirrorSources::new(config.layerx_network_id, sources).map_err(|_| "configuration")?;
    Ok(sources)
}

fn report(
    observation: &layerx_mirror::source::MirrorObservation,
    committed: CommittedCoordinate,
    evidence: Option<[u8; 32]>,
) -> Value {
    json!({
        "level": "Committed",
        "evidenceDigest": evidence.map(|digest| encode_hex(&digest)),
        "sourceId": observation.source.as_str(),
        "target": target(&observation.target),
        "canonicalPosition": position(observation.position),
        "provenance": format!("{:?}", observation.provenance),
        "latestBatch": observation.freshness.latest_batch.map(|value| value.to_string()),
        "batchLag": format!("{:?}", observation.freshness.batch_lag),
        "failoverCount": observation.failover_count,
        "agreeingSources": observation.agreeing_sources,
        "committedHeight": committed.height.to_string(),
        "committedBlockHash": encode_hex(&committed.block_hash)
    })
}

fn policy(value: &PolicyRequest) -> Result<MirrorReadPolicy, &'static str> {
    match value {
        PolicyRequest::Exact { candidate } => Ok(MirrorReadPolicy::Exact(locator(candidate)?)),
        PolicyRequest::OrderedPreference { candidates } => Ok(MirrorReadPolicy::OrderedPreference(
            candidates.iter().map(locator).collect::<Result<_, _>>()?,
        )),
        PolicyRequest::Agreement {
            candidates,
            minimum,
        } => Ok(MirrorReadPolicy::Agreement {
            candidates: candidates.iter().map(locator).collect::<Result<_, _>>()?,
            minimum: *minimum,
        }),
    }
}

fn locator(value: &Candidate) -> Result<MirrorLocator, &'static str> {
    Ok(MirrorLocator {
        source_index: value.source,
        commitment: ArchiveCommitment::from_bytes(fixed_hex(&value.commitment_hex)?),
    })
}

fn fixed_hex<const N: usize>(value: &str) -> Result<[u8; N], &'static str> {
    hex(value)?.try_into().map_err(|_| "configuration")
}

fn hex(value: &str) -> Result<Vec<u8>, &'static str> {
    let value = value.strip_prefix("0x").unwrap_or(value);
    if !value.len().is_multiple_of(2) {
        return Err("malformed");
    }
    (0..value.len())
        .step_by(2)
        .map(|index| u8::from_str_radix(&value[index..index + 2], 16).map_err(|_| "malformed"))
        .collect()
}

fn encode_hex(value: &[u8]) -> String {
    value.iter().fold(String::new(), |mut output, byte| {
        write!(&mut output, "{byte:02x}").unwrap_or_else(|error| panic!("hex encoding: {error}"));
        output
    })
}

fn target(value: &layerx_mirror::source::MirrorTargetIdentity) -> String {
    match value {
        layerx_mirror::source::MirrorTargetIdentity::Ethereum {
            chain_id,
            genesis_hash,
            contract,
            code_hash,
            publisher,
        } => format!(
            "ethereum:chain={chain_id};genesis={};contract={};code={};publisher={}",
            encode_hex(genesis_hash),
            encode_hex(contract),
            encode_hex(code_hash),
            encode_hex(publisher)
        ),
        layerx_mirror::source::MirrorTargetIdentity::Solana {
            genesis_hash,
            program,
            program_data,
            code_hash,
            publisher,
        } => format!(
            "solana:genesis={};program={};program_data={};code={};publisher={}",
            encode_hex(genesis_hash),
            encode_hex(program),
            encode_hex(program_data),
            encode_hex(code_hash),
            encode_hex(publisher)
        ),
    }
}

fn position(value: layerx_mirror::source::MirrorCanonicalPosition) -> String {
    match value {
        layerx_mirror::source::MirrorCanonicalPosition::Ethereum {
            block_number,
            block_hash,
            reference_head_number,
            reference_head_hash,
        } => format!(
            "ethereum:block={block_number};hash={};reference={reference_head_number};reference_hash={}",
            encode_hex(&block_hash),
            encode_hex(&reference_head_hash)
        ),
        layerx_mirror::source::MirrorCanonicalPosition::Solana {
            rooted_slot,
            rooted_blockhash,
        } => format!(
            "solana:slot={rooted_slot};hash={}",
            encode_hex(&rooted_blockhash)
        ),
    }
}

fn canonical_u64(value: &str) -> Result<u64, &'static str> {
    if value.is_empty()
        || value == "0"
        || value.starts_with('0')
        || !value.bytes().all(|byte| byte.is_ascii_digit())
    {
        return Err("malformed");
    }
    value.parse().map_err(|_| "malformed")
}
