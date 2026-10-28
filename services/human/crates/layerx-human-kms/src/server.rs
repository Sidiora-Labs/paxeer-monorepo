use crate::config::Config;
use crate::store::Store;
use crate::wire::{self, Error, Operation, Request, Result};
use rustls::{ServerConnection, StreamOwned};
use sha2::{Digest, Sha256};
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use zeroize::Zeroizing;

struct Permit(Arc<AtomicUsize>);
impl Drop for Permit {
    fn drop(&mut self) {
        self.0.fetch_sub(1, Ordering::AcqRel);
    }
}
pub(crate) fn run(
    config: Config,
    clock: &Arc<dyn layerx_types::clock::Clock>,
) -> std::result::Result<(), String> {
    let store = Arc::new(Mutex::new(Store::open(&config)?));
    let listener = TcpListener::bind(config.listen).map_err(|_| "KMS listener unavailable")?;
    let config = Arc::new(config);
    let active = Arc::new(AtomicUsize::new(0));
    for stream in listener.incoming() {
        let Ok(stream) = stream else {
            return Err("KMS accept failed".into());
        };
        if active
            .fetch_update(Ordering::AcqRel, Ordering::Acquire, |count| {
                (count < 16).then_some(count + 1)
            })
            .is_err()
        {
            continue;
        }
        let permit = Permit(Arc::clone(&active));
        let config = Arc::clone(&config);
        let store = Arc::clone(&store);
        let clock = Arc::clone(clock);
        thread::Builder::new()
            .name("human-kms".into())
            .spawn(move || {
                let _permit = permit;
                let _ = connection(stream, &config, &store, clock.as_ref());
            })
            .map_err(|_| "KMS worker unavailable")?;
    }
    Err("KMS listener stopped".into())
}
fn connection(
    mut tcp: TcpStream,
    config: &Config,
    store: &Mutex<Store>,
    clock: &dyn layerx_types::clock::Clock,
) -> Result<()> {
    tcp.set_read_timeout(Some(config.deadline))
        .map_err(|_| Error::Unavailable)?;
    tcp.set_write_timeout(Some(config.deadline))
        .map_err(|_| Error::Unavailable)?;
    let mut connection =
        ServerConnection::new(Arc::clone(&config.tls)).map_err(|_| Error::Unavailable)?;
    while connection.is_handshaking() {
        connection
            .complete_io(&mut tcp)
            .map_err(|_| Error::Refused)?;
    }
    let leaf = connection
        .peer_certificates()
        .and_then(|chain| chain.first())
        .ok_or(Error::Refused)?;
    let observed: [u8; 32] = Sha256::digest(leaf.as_ref()).into();
    let service = layerx_crypto::ct::eq_fixed(&observed, &config.client_pin);
    let executor = config
        .attestor_evm_client_certificate
        .as_ref()
        .is_some_and(|certificate| {
            let pin: [u8; 32] = Sha256::digest(certificate.as_ref()).into();
            layerx_crypto::ct::eq_fixed(&observed, &pin)
        });
    if !service && !executor {
        return Err(Error::Refused);
    }
    let mut tls = StreamOwned::new(connection, tcp);
    let frame = read_request(&mut tls)?;
    let request = Request::decode(&frame)?;
    let now = clock
        .sample(std::time::Duration::from_secs(1))
        .map_err(|_| Error::Unavailable)?
        .unix_seconds();
    let admission = if request.kind == Operation::ExecutorProbe {
        if executor {
            Ok(None)
        } else {
            Err(Error::Refused)
        }
    } else if !service && !matches!(request.operation, 6 | 8..=10 | 12) {
        Err(Error::Refused)
    } else {
        validate_sign(&request, config, now)
    };
    let answer = admission.and_then(|digest| {
        store
            .lock()
            .map_err(|_| Error::Unavailable)?
            .dispatch(&request, digest, now)
    });
    write_response(
        &mut tls,
        &wire::response(request.version, request.operation, answer),
    )
}
fn read_request(reader: &mut impl Read) -> Result<Zeroizing<Vec<u8>>> {
    let mut prefix = [0_u8; 4];
    reader.read_exact(&mut prefix).map_err(|_| Error::Refused)?;
    let length = usize::try_from(u32::from_be_bytes(prefix)).map_err(|_| Error::Refused)?;
    if length == 0 || length > wire::MAX_FRAME {
        return Err(Error::Refused);
    }
    let mut payload = Zeroizing::new(vec![0_u8; length]);
    reader
        .read_exact(&mut payload)
        .map_err(|_| Error::Refused)?;
    Ok(payload)
}
fn write_response(writer: &mut impl Write, payload: &[u8]) -> Result<()> {
    if payload.len() > wire::MAX_FRAME {
        return Err(Error::Unavailable);
    }
    let length = u32::try_from(payload.len()).map_err(|_| Error::Unavailable)?;
    writer
        .write_all(&length.to_be_bytes())
        .and_then(|()| writer.write_all(payload))
        .and_then(|()| writer.flush())
        .map_err(|_| Error::Unavailable)
}
fn validate_sign(request: &Request<'_>, config: &Config, now: u64) -> Result<Option<[u8; 32]>> {
    let network = u32::try_from(config.chain.chain_id).map_err(|_| Error::Refused)?;
    if request.operation == 13 {
        return if request.network == network && request.class == 1 {
            Ok(None)
        } else {
            Err(Error::Refused)
        };
    }
    if request.operation == 11 {
        let value = serde_json::from_slice(request.evm).map_err(|_| Error::Refused)?;
        return crate::send::digest(&value, request.binding, network, 3, now).map(Some);
    }
    if request.operation != 5 {
        return Ok(None);
    }
    let call = decode_unsigned_call(request.canonical)?;
    if call.chain_id != config.chain.chain_id || request.network != network {
        return Err(Error::Refused);
    }
    let actor_length = request.disclosure.get(5..9).ok_or(Error::Refused)?;
    if actor_length != 20_u32.to_be_bytes() {
        return Err(Error::Refused);
    }
    let sender = request
        .disclosure
        .get(9..29)
        .ok_or(Error::Refused)?
        .try_into()
        .map_err(|_| Error::Refused)?;
    let disclosure =
        layerx_crypto::disclosure::call::bind_call(&call, sender).map_err(|_| Error::Refused)?;
    if wire::disclosure(&disclosure)? != request.disclosure {
        return Err(Error::Refused);
    }
    let digest = call.signing_hash();
    if !layerx_crypto::ct::eq_fixed(&digest, &request.digest) {
        return Err(Error::Integrity);
    }
    Ok(Some(digest))
}

fn rlp_item<'a>(input: &mut &'a [u8], list: bool) -> Result<&'a [u8]> {
    let prefix = *input.first().ok_or(Error::Refused)?;
    if prefix < 0x80 {
        if list {
            return Err(Error::Refused);
        }
        let (payload, rest) = input.split_at(1);
        *input = rest;
        return Ok(payload);
    }
    if (prefix >= 0xc0) != list {
        return Err(Error::Refused);
    }
    let short = if list { 0xc0 } else { 0x80 };
    let length_prefix = prefix - short;
    let (offset, length) = if length_prefix < 56 {
        (1, usize::from(length_prefix))
    } else {
        let width = usize::from(length_prefix - 55);
        let encoded = input.get(1..1 + width).ok_or(Error::Refused)?;
        if encoded.first() == Some(&0) {
            return Err(Error::Refused);
        }
        let length = encoded
            .iter()
            .try_fold(0_usize, |value, byte| {
                value.checked_mul(256)?.checked_add(usize::from(*byte))
            })
            .ok_or(Error::Refused)?;
        if length < 56 {
            return Err(Error::Refused);
        }
        (1 + width, length)
    };
    let end = offset.checked_add(length).ok_or(Error::Refused)?;
    let payload = input.get(offset..end).ok_or(Error::Refused)?;
    if !list && length == 1 && payload[0] < 0x80 {
        return Err(Error::Refused);
    }
    *input = &input[end..];
    Ok(payload)
}

fn rlp_uint<const N: usize>(input: &mut &[u8]) -> Result<[u8; N]> {
    let bytes = rlp_item(input, false)?;
    if bytes.first() == Some(&0) {
        return Err(Error::Refused);
    }
    let offset = N.checked_sub(bytes.len()).ok_or(Error::Refused)?;
    let mut value = [0; N];
    value[offset..].copy_from_slice(bytes);
    Ok(value)
}

fn decode_unsigned_call(raw: &[u8]) -> Result<layerx_crypto::evm_transaction::Eip1559Call> {
    let Some((&2, mut envelope)) = raw.split_first() else {
        return Err(Error::Refused);
    };
    let mut fields = rlp_item(&mut envelope, true)?;
    if !envelope.is_empty() {
        return Err(Error::Refused);
    }
    let chain_id = u64::from_be_bytes(rlp_uint(&mut fields)?);
    let nonce = u64::from_be_bytes(rlp_uint(&mut fields)?);
    let max_priority_fee_per_gas = u128::from_be_bytes(rlp_uint(&mut fields)?);
    let max_fee_per_gas = u128::from_be_bytes(rlp_uint(&mut fields)?);
    let gas_limit = u64::from_be_bytes(rlp_uint(&mut fields)?);
    let to = rlp_item(&mut fields, false)?
        .try_into()
        .map_err(|_| Error::Refused)?;
    if rlp_uint::<32>(&mut fields)? != [0; 32] {
        return Err(Error::Refused);
    }
    let data = rlp_item(&mut fields, false)?.to_vec();
    if !rlp_item(&mut fields, true)?.is_empty() || !fields.is_empty() {
        return Err(Error::Refused);
    }
    Ok(layerx_crypto::evm_transaction::Eip1559Call {
        chain_id,
        nonce,
        max_priority_fee_per_gas,
        max_fee_per_gas,
        gas_limit,
        to,
        data,
    })
}
