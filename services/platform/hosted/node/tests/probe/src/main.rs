//! Real-process probe for the beta node: reads the network head and account
//! balance through `layerx-client` RPC endpoints, and talks to the supervisor
//! socket. Every result is printed as one JSON line on stdout.

use std::collections::BTreeMap;
use std::fmt::Write as _;
use std::io::{BufRead, BufReader, Read, Write};
use std::os::unix::fs::OpenOptionsExt;
use std::os::unix::net::UnixStream;
use std::path::PathBuf;
use std::process::ExitCode;
use std::time::Duration;

use layerx_client::px::{PxEndpoint, PxNetworkHead};
use layerx_client::rpc::account::{self as rpc_account, ReadContext, Requested};
use layerx_client::rpc::history;
use layerx_client::sequencer::SequencerEndpoint;
use layerx_types::account::AccountId;
use layerx_types::ids::Did;
use layerx_types::rung::Rung;

fn usage() -> ExitCode {
    eprintln!(
        "usage: layerx-node-probe handshake --rpc URL --trust-anchor PATH --network-id N\n       layerx-node-probe balance --rpc URL --trust-anchor PATH --network-id N --account NAME --asset HEX64\n       layerx-node-probe did-accounts --rpc URL --trust-anchor PATH --network-id N --did DID\n       layerx-node-probe supervisor --socket PATH --request reset|status\n       layerx-node-probe write-send --rpc URL --trust-anchor PATH --network-id N --seed-file PATH --destination-did DID --asset HEX64 --output PATH\n       layerx-node-probe custody-asset-profile --rpc URL --trust-anchor PATH --network-id N --asset HEX64 [--source-did DID --destination-did DID --profile-output ABS]\n       layerx-node-probe write-asset-send --rpc URL --trust-anchor PATH --network-id N --seed-file PATH --destination-did DID --asset HEX64 --output PATH [--source-did DID --amount N]"
    );
    ExitCode::from(2)
}

fn options(arguments: &[String]) -> Result<BTreeMap<String, String>, String> {
    let mut parsed = BTreeMap::new();
    let mut index = 0;
    while index < arguments.len() {
        let key = &arguments[index];
        let Some(name) = key.strip_prefix("--") else {
            return Err(format!("unexpected argument {key}"));
        };
        let Some(value) = arguments.get(index + 1) else {
            return Err(format!("missing value for --{name}"));
        };
        parsed.insert(name.to_owned(), value.clone());
        index += 2;
    }
    Ok(parsed)
}

fn required<'a>(parsed: &'a BTreeMap<String, String>, name: &str) -> Result<&'a str, String> {
    parsed
        .get(name)
        .map(String::as_str)
        .ok_or_else(|| format!("--{name} is required"))
}

fn hex(bytes: &[u8]) -> String {
    let mut text = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        let _ = write!(text, "{byte:02x}");
    }
    text
}

fn hex32(text: &str) -> Result<[u8; 32], String> {
    if text.len() != 64 {
        return Err("expected 64 hex characters".to_owned());
    }
    let mut out = [0u8; 32];
    for (index, chunk) in text.as_bytes().chunks(2).enumerate() {
        let pair = std::str::from_utf8(chunk).map_err(|error| error.to_string())?;
        out[index] = u8::from_str_radix(pair, 16).map_err(|error| error.to_string())?;
    }
    Ok(out)
}

fn rpc_session(
    parsed: &BTreeMap<String, String>,
) -> Result<(SequencerEndpoint, PxEndpoint, PxNetworkHead), String> {
    let url = required(parsed, "rpc")?;
    let anchor = parsed
        .get("trust-anchor")
        .map(std::fs::read)
        .transpose()
        .map_err(|error| format!("trust anchor: {error}"))?;
    let anchors: Vec<&[u8]> = anchor.iter().map(Vec::as_slice).collect();
    let endpoint = SequencerEndpoint::new(url, &anchors)
        .map_err(|error| format!("sequencer endpoint: {error:?}"))?;
    let px = PxEndpoint::new(url, &anchors).map_err(|error| format!("px endpoint: {error:?}"))?;
    let network = px
        .network()
        .map_err(|error| format!("network: {error:?}"))?;
    let expected_chain_id = required(parsed, "network-id")?
        .parse::<u64>()
        .map_err(|error| format!("--network-id: {error}"))?;
    let chain_id = endpoint
        .chain_id()
        .map_err(|error| format!("chain id: {error:?}"))?;
    if chain_id != expected_chain_id || network.paxeer.chain_id != chain_id {
        return Err("network id mismatch".to_owned());
    }
    Ok((endpoint, px, network))
}

fn handshake(parsed: &BTreeMap<String, String>) -> Result<String, String> {
    let (_, _, network) = rpc_session(parsed)?;
    Ok(format!(
        "{{\"network_id\":{},\"chain_id\":{},\"latest_committed_height\":{}}}",
        public_json_string(&network.network_id),
        network.paxeer.chain_id,
        network.latest_committed_height,
    ))
}

fn balance(parsed: &BTreeMap<String, String>) -> Result<String, String> {
    let account = AccountId::parse(required(parsed, "account")?)
        .map_err(|error| format!("--account: {error:?}"))?;
    let asset = hex32(required(parsed, "asset")?)?;
    let (endpoint, _, network) = rpc_session(parsed)?;
    let account_id = layerx_wire::hash::account_id_for_protocol(
        &account,
        layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION,
    )
    .map_err(|error| format!("account id: {error:?}"))?;
    let requested =
        Requested::new(Rung::Committed).map_err(|error| format!("balance rung: {error:?}"))?;
    let amount = rpc_account::balance(
        &endpoint,
        account_id,
        asset,
        ReadContext {
            committed_height: network.latest_committed_height,
            requested,
        },
    )
    .map_err(|error| format!("balance read failed: {error:?}"))?;
    Ok(format!(
        "{{\"account\":{},\"account_id\":\"{}\",\"asset\":\"{}\",\"balance\":\"{}\",\"achieved\":\"{}\",\"global_sequence\":{}}}",
        public_json_string(account.canonical()),
        hex(&account_id),
        hex(&asset),
        amount.value(),
        requested.level().name(),
        network.latest_committed_height,
    ))
}

/// List native bound accounts as JSON, following `did-accounts.schema.json`.
/// Empty and unbound listings have `count: 0` and `accounts: []`; unavailable
/// reads remain errors rather than being reported as empty listings.
pub fn did_accounts(parsed: &BTreeMap<String, String>) -> Result<String, String> {
    let did = required(parsed, "did")?;
    Did::new(did.as_bytes()).map_err(|error| format!("--did: {error:?}"))?;
    let (_, px, _) = rpc_session(parsed)?;
    let values = history::did_accounts(&px, did)
        .map_err(|error| format!("did account listing failed: {error:?}"))?;
    let accounts: Vec<String> = values.iter().map(ToString::to_string).collect();
    Ok(format!(
        "{{\"did\":{},\"count\":{},\"accounts\":[{}]}}",
        public_json_string(did),
        values.len(),
        accounts.join(","),
    ))
}

fn supervisor(parsed: &BTreeMap<String, String>) -> Result<String, String> {
    let socket = required(parsed, "socket")?;
    let request = required(parsed, "request")?;
    if request != "reset" && request != "status" {
        return Err("--request must be reset or status".to_owned());
    }
    let mut stream = UnixStream::connect(socket)
        .map_err(|error| format!("supervisor connect failed: {error}"))?;
    stream
        .set_read_timeout(Some(Duration::from_secs(330)))
        .map_err(|error| error.to_string())?;
    stream
        .write_all(format!("{request}\n").as_bytes())
        .map_err(|error| format!("supervisor write failed: {error}"))?;
    let mut reply = String::new();
    BufReader::new(stream)
        .read_line(&mut reply)
        .map_err(|error| format!("supervisor read failed: {error}"))?;
    let reply = reply.trim_end().to_owned();
    if reply.is_empty() {
        return Err("supervisor closed without a reply".to_owned());
    }
    Ok(reply)
}

fn write_send(parsed: &BTreeMap<String, String>) -> Result<String, String> {
    let network_id = required(parsed, "network-id")?
        .parse::<u32>()
        .map_err(|error| error.to_string())?;
    let seed_file = std::fs::OpenOptions::new()
        .read(true)
        .custom_flags(libc::O_NONBLOCK | libc::O_NOFOLLOW)
        .open(required(parsed, "seed-file")?)
        .map_err(|error| error.to_string())?;
    let metadata = seed_file.metadata().map_err(|error| error.to_string())?;
    if !metadata.is_file() || metadata.len() != 32 {
        return Err("test seed must be a 32-byte regular file".to_owned());
    }
    let mut seed_bytes = Vec::new();
    seed_file
        .take(33)
        .read_to_end(&mut seed_bytes)
        .map_err(|error| error.to_string())?;
    let seed: [u8; 32] = seed_bytes
        .try_into()
        .map_err(|_| "test seed must be 32 bytes".to_owned())?;
    let source_did = layerx_platform_core::treasury_did(&seed);
    let (endpoint, px, network) = rpc_session(parsed)?;
    let address = layerx_platform_core::custody_source_address(&px, &source_did)?;
    let nonce = endpoint
        .nonce_at(&address, network.latest_committed_height)
        .map_err(|error| format!("preparation account: {error:?}"))?;
    let timestamp = endpoint
        .block_by_number(network.latest_committed_height)
        .map_err(|error| format!("preparation block: {error:?}"))?
        .ok_or("preparation committed block unavailable")?
        .timestamp
        .checked_mul(1000)
        .ok_or("timestamp exhausted")?;
    let signed = layerx_platform_core::build_send(
        &seed,
        &layerx_platform_core::SendRequest {
            network_id,
            source_did,
            destination_did: required(parsed, "destination-did")?.to_owned(),
            asset: hex32(required(parsed, "asset")?)?,
            amount: 1,
            account_sequence: nonce.checked_add(1).ok_or("sequence exhausted")?,
            idempotency_key: [0x61; 32],
            not_before_ms: timestamp,
            expires_at_ms: timestamp
                .checked_add(300_000)
                .ok_or("timestamp exhausted")?,
            fee_limit: 1000,
        },
    )?;
    let mut output = std::fs::OpenOptions::new()
        .write(true)
        .create_new(true)
        .mode(0o600)
        .open(required(parsed, "output")?)
        .map_err(|error| error.to_string())?;
    output
        .write_all(&signed.canonical)
        .map_err(|error| error.to_string())?;
    output.sync_all().map_err(|error| error.to_string())?;
    Ok(hex(&signed.activity_id))
}

struct CustodySession {
    sequencer: SequencerEndpoint,
    network: PxEndpoint,
    network_id: u32,
    committed_height: u64,
}

fn custody_session(parsed: &BTreeMap<String, String>) -> Result<CustodySession, String> {
    let network_id = required(parsed, "network-id")?
        .parse::<u32>()
        .map_err(|error| error.to_string())?;
    let url = required(parsed, "rpc")?;
    let mut der = Vec::new();
    std::fs::File::open(required(parsed, "trust-anchor")?)
        .and_then(|file| file.take(1_048_577).read_to_end(&mut der))
        .map_err(|error| format!("custody trust anchor: {error}"))?;
    if der.is_empty() || der.len() > 1_048_576 {
        return Err("invalid custody trust anchor DER".to_owned());
    }
    let sequencer = SequencerEndpoint::new(url, &[&der])
        .map_err(|error| format!("custody sequencer endpoint: {error:?}"))?;
    let network = PxEndpoint::new(url, &[&der])
        .map_err(|error| format!("custody network endpoint: {error:?}"))?;
    let head = network
        .network()
        .map_err(|error| format!("custody committed height: {error:?}"))?;
    let chain_id = sequencer
        .chain_id()
        .map_err(|error| format!("custody chain id: {error:?}"))?;
    if chain_id != u64::from(network_id) || head.paxeer.chain_id != chain_id {
        return Err("network id mismatch".to_owned());
    }
    let committed_height = head.latest_committed_height;
    Ok(CustodySession {
        sequencer,
        network,
        network_id,
        committed_height,
    })
}

#[derive(Eq, PartialEq)]
struct CustodyPreparation {
    height: u64,
    nonce: u64,
    address: [u8; 20],
    block: layerx_client::sequencer::SequencerBlock,
    state_root: [u8; 32],
}

fn custody_preparation(
    session: &CustodySession,
    actor: &str,
) -> Result<CustodyPreparation, String> {
    let height = session
        .network
        .network()
        .map_err(|error| format!("custody committed height: {error:?}"))?
        .latest_committed_height;
    if height != session.committed_height {
        return Err("custody committed height changed; refresh required".to_owned());
    }
    let address = layerx_platform_core::custody_source_address(&session.network, actor)?;
    let nonce = session
        .sequencer
        .pending_nonce(&address)
        .map_err(|error| format!("custody pending nonce: {error:?}"))?;
    let (block, state_root) =
        layerx_platform_core::native_committed_state(&session.sequencer, height)?;
    Ok(CustodyPreparation {
        height,
        nonce,
        address,
        block,
        state_root,
    })
}

fn public_json_string(value: &str) -> String {
    let mut output = String::from("\"");
    for character in value.chars() {
        match character {
            '\\' => output.push_str("\\\\"),
            '"' => output.push_str("\\\""),
            control if control <= '\u{001f}' => {
                let _ = write!(output, "\\u{:04x}", u32::from(control));
            }
            other => output.push(other),
        }
    }
    output.push('"');
    output
}

pub fn custody_asset_profile(parsed: &BTreeMap<String, String>) -> Result<String, String> {
    let session = custody_session(parsed)?;
    let asset = hex32(required(parsed, "asset")?)?;
    let actors: Vec<_> = ["source-did", "destination-did"]
        .iter()
        .filter_map(|role| parsed.get(*role))
        .collect();
    let before = actors
        .iter()
        .map(|did| custody_preparation(&session, did))
        .collect::<Result<Vec<_>, _>>()?;
    let snapshot_before =
        layerx_platform_core::native_committed_state(&session.sequencer, session.committed_height)?;
    let selected = layerx_platform_core::read_custody_profile(
        &session.sequencer,
        session.network_id,
        session.committed_height,
        asset,
    )?;
    let after = actors
        .iter()
        .map(|did| custody_preparation(&session, did))
        .collect::<Result<Vec<_>, _>>()?;
    let height_after = session
        .network
        .network()
        .map_err(|error| format!("custody committed height: {error:?}"))?
        .latest_committed_height;
    if before != after || height_after != session.committed_height {
        return Err(
            "custody preparation changed during profile reads; refresh required".to_owned(),
        );
    }
    let snapshot_after =
        layerx_platform_core::native_committed_state(&session.sequencer, height_after)?;
    if snapshot_before != snapshot_after || selected.trusted_height() != session.committed_height {
        return Err(
            "custody preparation changed during profile reads; refresh required".to_owned(),
        );
    }
    let mut accounts = String::new();
    for role in ["source", "destination"] {
        if let Some(did) = parsed.get(&format!("{role}-did")) {
            let name = selected.account_name(did)?;
            let account = AccountId::parse(&name)
                .map_err(|error| format!("custody {role} account: {error:?}"))?;
            let id = layerx_wire::hash::account_id_for_protocol(&account, 3)
                .map_err(|error| format!("custody {role} account id: {error:?}"))?;
            let _ = write!(
                accounts,
                ",\"{role}_account\":{},\"{role}_account_id\":\"{}\"",
                public_json_string(&name),
                hex(&id)
            );
        }
    }
    if let Some(profile_output) = parsed.get("profile-output") {
        let path = PathBuf::from(profile_output);
        if !path.is_absolute() {
            return Err("--profile-output must be absolute".to_owned());
        }
        let mut output = std::fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .custom_flags(libc::O_NOFOLLOW | libc::O_NONBLOCK)
            .open(&path)
            .map_err(|error| format!("custody profile output: {error}"))?;
        output
            .write_all(selected.profile())
            .map_err(|error| error.to_string())?;
        output.sync_all().map_err(|error| error.to_string())?;
        let _ = write!(
            accounts,
            ",\"profile_path\":{}",
            public_json_string(profile_output)
        );
    }
    Ok(format!(
        "{{\"asset\":\"{}\",\"profile_sha256\":\"{}\",\"profile_hex\":\"{}\",\"trusted_height\":{},\"trust_source\":\"authenticated_native_bank_pointer\",\"head_sequence\":{},\"global_sequence\":{},\"committed_height\":{},\"block_hash\":\"{}\",\"app_hash\":\"{}\",\"achieved\":\"COMMITTED\"{accounts}}}",
        hex(&selected.asset()), hex(&selected.profile_hash()),
        hex(selected.profile()), selected.trusted_height(),
        session.committed_height, session.committed_height, selected.trusted_height(),
        hex(&snapshot_after.0.hash), hex(&snapshot_after.1),
    ))
}

pub fn write_asset_send(parsed: &BTreeMap<String, String>) -> Result<String, String> {
    let seed_file = std::fs::OpenOptions::new()
        .read(true)
        .custom_flags(libc::O_NONBLOCK | libc::O_NOFOLLOW)
        .open(required(parsed, "seed-file")?)
        .map_err(|error| error.to_string())?;
    let metadata = seed_file.metadata().map_err(|error| error.to_string())?;
    if !metadata.is_file() || metadata.len() != 32 {
        return Err("test seed must be a 32-byte regular file".to_owned());
    }
    let mut seed_bytes = Vec::new();
    seed_file
        .take(33)
        .read_to_end(&mut seed_bytes)
        .map_err(|error| error.to_string())?;
    let seed: [u8; 32] = seed_bytes
        .try_into()
        .map_err(|_| "test seed must be 32 bytes".to_owned())?;
    let source_did = layerx_platform_core::treasury_did(&seed);
    if parsed
        .get("source-did")
        .is_some_and(|supplied| supplied != &source_did)
    {
        return Err("source DID does not match the seed owner".to_owned());
    }
    let session = custody_session(parsed)?;
    let before = custody_preparation(&session, &source_did)?;
    let asset = hex32(required(parsed, "asset")?)?;
    let selected = layerx_platform_core::read_custody_asset(
        &session.sequencer,
        &session.network,
        session.network_id,
        session.committed_height,
        asset,
        &source_did,
    )?;
    let after = custody_preparation(&session, &source_did)?;
    if before != after
        || selected.account_sequence() != before.nonce
        || selected.trusted_height() != before.height
        || selected.state_root() != before.state_root
    {
        return Err(
            "custody preparation changed during asset proof reads; refresh required".to_owned(),
        );
    }
    let amount = parsed
        .get("amount")
        .map_or(Ok(1), |value| value.parse::<u128>())
        .map_err(|error| format!("--amount: {error}"))?;
    let timestamp = before
        .block
        .timestamp
        .checked_mul(1000)
        .ok_or("timestamp exhausted")?;
    let signed = layerx_platform_core::build_asset_send_with_identity_sequence(
        &seed,
        before.nonce,
        &layerx_platform_core::SendRequest {
            network_id: session.network_id,
            source_did,
            destination_did: required(parsed, "destination-did")?.to_owned(),
            asset,
            amount,
            account_sequence: selected.account_sequence(),
            idempotency_key: [0x62; 32],
            not_before_ms: timestamp,
            expires_at_ms: timestamp
                .checked_add(300_000)
                .ok_or("timestamp exhausted")?,
            fee_limit: 1000,
        },
        &selected,
    )?;
    let mut output = std::fs::OpenOptions::new()
        .write(true)
        .create_new(true)
        .mode(0o600)
        .custom_flags(libc::O_NOFOLLOW | libc::O_NONBLOCK)
        .open(required(parsed, "output")?)
        .map_err(|error| error.to_string())?;
    output
        .write_all(&signed.canonical)
        .map_err(|error| error.to_string())?;
    output.sync_all().map_err(|error| error.to_string())?;
    Ok(hex(&signed.activity_id))
}

fn main() -> ExitCode {
    let arguments: Vec<String> = std::env::args().skip(1).collect();
    let Some(command) = arguments.first() else {
        return usage();
    };
    let parsed = match options(&arguments[1..]) {
        Ok(parsed) => parsed,
        Err(error) => {
            eprintln!("layerx-node-probe: {error}");
            return usage();
        }
    };
    let outcome = match command.as_str() {
        "handshake" => handshake(&parsed),
        "balance" => balance(&parsed),
        "did-accounts" => did_accounts(&parsed),
        "supervisor" => supervisor(&parsed),
        "write-send" => write_send(&parsed),
        "custody-asset-profile" => custody_asset_profile(&parsed),
        "write-asset-send" => write_asset_send(&parsed),
        _ => return usage(),
    };
    match outcome {
        Ok(line) => {
            println!("{line}");
            ExitCode::SUCCESS
        }
        Err(error) => {
            eprintln!("layerx-node-probe: {error}");
            ExitCode::FAILURE
        }
    }
}
