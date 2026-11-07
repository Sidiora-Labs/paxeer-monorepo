use crate::committed::{base64, verify_membership, CommittedHeader};
use crate::hex;
use serde_json::json;
use serde_json::Value;
use std::collections::BTreeMap;
use std::io::{Read, Write};
use std::net::TcpStream;
use std::time::{Duration, Instant};

pub(super) struct Evidence {
    pub value: Vec<u8>,
    pub document: Value,
}

pub(super) fn read(url: &str, head: &CommittedHeader, key: &[u8]) -> Result<Evidence, ()> {
    let reply = query(url, key, head.state_height)?;
    if reply["code"].as_u64() != Some(0)
        || reply["height"].as_str().and_then(|s| s.parse::<u64>().ok()) != Some(head.state_height)
        || base64(reply["key"].as_str().ok_or(())?)? != key
    {
        return Err(());
    }
    let value = base64(reply["value"].as_str().ok_or(())?)?;
    let ops = reply["proofOps"]["ops"].as_array().ok_or(())?;
    if value.is_empty()
        || ops.len() != 2
        || ops[0]["type"] != "ics23:iavl"
        || ops[1]["type"] != "ics23:simple"
        || base64(ops[0]["key"].as_str().ok_or(())?)? != key
        || base64(ops[1]["key"].as_str().ok_or(())?)? != b"lxphost"
    {
        return Err(());
    }
    verify_membership(
        key,
        &value,
        &base64(ops[0]["data"].as_str().ok_or(())?)?,
        &base64(ops[1]["data"].as_str().ok_or(())?)?,
        head.app_hash,
    )?;
    Ok(Evidence {
        value,
        document: reply,
    })
}

pub(super) fn account(
    id: [u8; 32],
    bytes: &[u8],
) -> Result<layerx_proof::state::CanonicalAccount, ()> {
    use layerx_wire::decode::Decoder;
    let mut reader = Decoder::new(bytes, 2048);
    if reader.bytes(32).map_err(|_| ())? != id {
        return Err(());
    }
    let name = reader.bytes(512).map_err(|_| ())?;
    let mut canonical = u16::try_from(name.len())
        .map_err(|_| ())?
        .to_be_bytes()
        .to_vec();
    canonical.extend_from_slice(name);
    canonical.push(reader.u8().map_err(|_| ())?);
    canonical.extend_from_slice(&reader.u128().map_err(|_| ())?.to_be_bytes());
    let asset = reader.bytes(32).map_err(|_| ())?;
    if asset.len() != 32 {
        return Err(());
    }
    canonical.extend_from_slice(asset);
    canonical.push(reader.u8().map_err(|_| ())?);
    canonical.extend_from_slice(&reader.u64().map_err(|_| ())?.to_be_bytes());
    canonical.extend_from_slice(&reader.u64().map_err(|_| ())?.to_be_bytes());
    canonical.push(reader.u8().map_err(|_| ())?);
    canonical.push(reader.u8().map_err(|_| ())?);
    let authority = reader.bytes(32).map_err(|_| ())?;
    if authority.len() != 32 {
        return Err(());
    }
    canonical.extend_from_slice(authority);
    canonical.push(reader.u8().map_err(|_| ())?);
    reader.finish().map_err(|_| ())?;
    layerx_proof::state::decode_account_value(id, &canonical).map_err(|_| ())
}

fn query(url: &str, key: &[u8], height: u64) -> Result<Value, ()> {
    let base = url
        .trim_end_matches('/')
        .strip_suffix("/evm")
        .unwrap_or(url.trim_end_matches('/'));
    let (host, address) = crate::loopback_http(base).ok_or(())?;
    let body = json!({"jsonrpc":"2.0", "id":1, "method":"abci_query", "params":{
        "path":"/store/lxphost/key", "data":format!("0x{}",hex::encode(key)),
        "height":height.to_string(), "prove":true}})
    .to_string();
    let deadline = Instant::now() + Duration::from_secs(8);
    let remaining = || {
        deadline
            .checked_duration_since(Instant::now())
            .filter(|d| !d.is_zero())
            .ok_or(())
    };
    let mut stream = TcpStream::connect_timeout(&address, remaining()?).map_err(|_| ())?;
    stream
        .set_write_timeout(Some(remaining()?))
        .map_err(|_| ())?;
    write!(stream, "POST /comet HTTP/1.1\r\nHost: {host}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).map_err(|_| ())?;
    let mut raw = Vec::new();
    let mut chunk = [0; 4096];
    loop {
        stream
            .set_read_timeout(Some(remaining()?))
            .map_err(|_| ())?;
        let count = stream.read(&mut chunk).map_err(|_| ())?;
        if count == 0 {
            break;
        }
        raw.extend_from_slice(&chunk[..count]);
        if raw.len() > 2 * 1024 * 1024 {
            return Err(());
        }
    }
    let end = raw
        .windows(4)
        .position(|s| s == b"\r\n\r\n")
        .filter(|n| *n <= 32768)
        .ok_or(())?;
    let mut lines = std::str::from_utf8(&raw[..end])
        .map_err(|_| ())?
        .split("\r\n");
    if lines.next().and_then(|s| s.split_whitespace().nth(1)) != Some("200") {
        return Err(());
    }
    let mut headers = BTreeMap::new();
    for line in lines {
        let (name, value) = line.split_once(':').ok_or(())?;
        if headers
            .insert(name.to_ascii_lowercase(), value.trim())
            .is_some()
        {
            return Err(());
        }
    }
    let body = &raw[end + 4..];
    if headers.contains_key("transfer-encoding")
        || headers
            .get("content-length")
            .and_then(|s| s.parse::<usize>().ok())
            != Some(body.len())
    {
        return Err(());
    }
    let document: Value = serde_json::from_slice(body).map_err(|_| ())?;
    if document["jsonrpc"] != "2.0" || document["id"] != 1 || document.get("error").is_some() {
        return Err(());
    }
    document["result"].get("response").cloned().ok_or(())
}
