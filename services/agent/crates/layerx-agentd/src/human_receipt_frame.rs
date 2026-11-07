//! Owner receipt lookup wire contract: presence 0 is Absent, 2 is Submitted
//! (transaction hash and nonce), and 1 is Found followed by ReceiptLookupFrame.
//! Verification uses the Human codes 1 SEQUENCER_SIGNED, 2 BATCH_INCLUDED,
//! 3 STATE_PROVEN, 6 COMMITTED; legacy codes 4 and 5 decode as COMMITTED.
//! The rung is Rung::wire_rank (0 Submitted, 1 Receipted, 2 Committed).
//! The complete field order and program.receipt JSON projection are maintained in
//! `contracts/agent-human-peer.md` and `contracts/agent-api.md`. Verification codes
//! 4 and 5 are historical migration metadata, never additional committed-chain authority.

use layerx_proof::receipt::AuthorizedBatch;
use layerx_types::{result::KnownResult, rung::Rung, verify::VerificationLevel};
use layerx_wire::{decode::Decoder, encode::Encoder, WireError};

const MAX_BYTES: usize = 1_048_576;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ReceiptLookupFrame {
    pub canonical_bytes: Vec<u8>,
    pub authorised_batch: AuthorizedBatch,
    pub verification_level: VerificationLevel,
    pub tx_hash: [u8; 32],
    pub receipt_hash: [u8; 32],
    pub block_height: u64,
    pub rung: Rung,
    pub signed_receipt: Vec<u8>,
    pub receipt_key_id: [u8; 32],
    pub public_key: [u8; 32],
    pub first_height: u64,
    pub last_height: u64,
    pub inclusion_height: u64,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ReceiptLookupAnswer {
    Absent,
    Submitted { tx_hash: [u8; 32], nonce: u64 },
    Found(ReceiptLookupFrame),
}

fn invalid(offset: usize) -> WireError {
    WireError {
        result: KnownResult::NonCanonical.into(),
        offset,
    }
}

fn fixed(decoder: &mut Decoder<'_>) -> Result<[u8; 32], WireError> {
    let offset = decoder.offset();
    decoder.fixed(32)?.try_into().map_err(|_| invalid(offset))
}

impl ReceiptLookupFrame {
    pub fn encode(&self) -> Result<Vec<u8>, WireError> {
        let mut encoder = Encoder::new(MAX_BYTES);
        self.write(&mut encoder)?;
        Ok(encoder.finish())
    }

    fn write(&self, encoder: &mut Encoder) -> Result<(), WireError> {
        if self.canonical_bytes.is_empty()
            || self.signed_receipt.is_empty()
            || self.verification_level == VerificationLevel::UNVERIFIED
        {
            return Err(invalid(encoder.as_bytes().len()));
        }
        encoder.bytes(&self.canonical_bytes, MAX_BYTES)?;
        for field in [
            self.authorised_batch.batch_id(),
            self.authorised_batch.asset(),
            self.authorised_batch.previous_state_root(),
            self.authorised_batch.resulting_state_root(),
            self.authorised_batch.sequencer_public_key(),
        ] {
            encoder.fixed(&field)?;
        }
        encoder.u8(self.verification_level.wire_rank())?;
        encoder.fixed(&self.tx_hash)?;
        encoder.fixed(&self.receipt_hash)?;
        encoder.u64(self.block_height)?;
        encoder.u8(self.rung.wire_rank())?;
        encoder.bytes(&self.signed_receipt, MAX_BYTES)?;
        encoder.fixed(&self.receipt_key_id)?;
        encoder.fixed(&self.public_key)?;
        encoder.u64(self.first_height)?;
        encoder.u64(self.last_height)?;
        encoder.u64(self.inclusion_height)
    }

    pub fn decode(bytes: &[u8]) -> Result<Self, WireError> {
        let mut decoder = Decoder::new(bytes, MAX_BYTES);
        let value = Self::decode_from(&mut decoder)?;
        decoder.finish()?;
        Ok(value)
    }

    pub fn decode_from(decoder: &mut Decoder<'_>) -> Result<Self, WireError> {
        let canonical_bytes = decoder.bytes_owned(MAX_BYTES)?;
        let authorised_batch = AuthorizedBatch::new(
            fixed(decoder)?,
            fixed(decoder)?,
            fixed(decoder)?,
            fixed(decoder)?,
            fixed(decoder)?,
        );
        let verification_level = match decoder.u8()? {
            1 => VerificationLevel::SEQUENCER_SIGNED,
            2 => VerificationLevel::BATCH_INCLUDED,
            3 => VerificationLevel::STATE_PROVEN,
            4 | 5 | 6 => VerificationLevel::COMMITTED,
            _ => return Err(invalid(decoder.offset())),
        };
        let tx_hash = fixed(decoder)?;
        let receipt_hash = fixed(decoder)?;
        let block_height = decoder.u64()?;
        let rung = match decoder.u8()? {
            0 => Rung::Submitted,
            1 => Rung::Receipted,
            2 => Rung::Committed,
            _ => return Err(invalid(decoder.offset())),
        };
        let signed_receipt = decoder.bytes_owned(MAX_BYTES)?;
        if canonical_bytes.is_empty() || signed_receipt.is_empty() {
            return Err(invalid(decoder.offset()));
        }
        Ok(Self {
            canonical_bytes,
            authorised_batch,
            verification_level,
            tx_hash,
            receipt_hash,
            block_height,
            rung,
            signed_receipt,
            receipt_key_id: fixed(decoder)?,
            public_key: fixed(decoder)?,
            first_height: decoder.u64()?,
            last_height: decoder.u64()?,
            inclusion_height: decoder.u64()?,
        })
    }
}

impl ReceiptLookupAnswer {
    pub fn encode(&self) -> Result<Vec<u8>, WireError> {
        let mut encoder = Encoder::new(MAX_BYTES);
        match self {
            Self::Absent => encoder.u8(0)?,
            Self::Submitted { tx_hash, nonce } => {
                encoder.u8(2)?;
                encoder.fixed(tx_hash)?;
                encoder.u64(*nonce)?;
            }
            Self::Found(frame) => {
                encoder.u8(1)?;
                frame.write(&mut encoder)?;
            }
        }
        Ok(encoder.finish())
    }

    pub fn decode(bytes: &[u8]) -> Result<Self, WireError> {
        let mut decoder = Decoder::new(bytes, MAX_BYTES);
        let value = match decoder.u8()? {
            0 => Self::Absent,
            2 => Self::Submitted {
                tx_hash: fixed(&mut decoder)?,
                nonce: decoder.u64()?,
            },
            1 => Self::Found(ReceiptLookupFrame::decode_from(&mut decoder)?),
            _ => return Err(invalid(decoder.offset())),
        };
        decoder.finish()?;
        Ok(value)
    }
}

/// Loopback Human-owner health contract; credentials are separate from outbound authority access.
pub mod owner_health {
    use std::io::{Read, Write};

    pub const HEALTH_BEARER_ENV: &str = "LAYERX_AGENT_PROGRAM_BEARER_TOKEN";
    pub const AUTHORITY_BEARER_ENV: &str = "LAYERX_AGENT_HUMAN_AUTHORITY_BEARER";
    const HEALTH_HEADER_LIMIT: usize = 16 * 1024;

    /// Enforces the Human owner's loopback address, credential length and separation.
    ///
    /// # Errors
    /// Refuses a non-loopback listener, short health bearer or reused authority bearer.
    pub fn validate_health_credentials(
        listen: &str,
        bearer: &str,
        authority_bearer: &str,
    ) -> Result<(), String> {
        if !listen.starts_with("127.0.0.1:") || bearer.len() < 32 {
            return Err("human health requires loopback and a bounded credential".to_owned());
        }
        if bearer == authority_bearer {
            return Err("human health and authority credentials must be distinct".to_owned());
        }
        Ok(())
    }

    // The same bounded GET parser previously used through serve_http::parse_request.
    // S is DeadlineStream in the daemon and a deadline-configured TCP socket in the proof.
    fn health_request<S: Read>(
        stream: &mut S,
        bearer: &str,
    ) -> Result<Option<String>, (u16, &'static str)> {
        let mut bytes = [0_u8; HEALTH_HEADER_LIMIT];
        let mut length = 0;
        while length < bytes.len() && !bytes[..length].windows(4).any(|v| v == b"\r\n\r\n") {
            match stream.read(&mut bytes[length..]) {
                Ok(0) => return Ok(None),
                Ok(count) => length += count,
                Err(error)
                    if matches!(
                        error.kind(),
                        std::io::ErrorKind::WouldBlock | std::io::ErrorKind::TimedOut
                    ) =>
                {
                    return Err((408, "{\"error\":\"request_timeout\"}"))
                }
                Err(error) if error.kind() == std::io::ErrorKind::Interrupted => {}
                Err(_) => return Ok(None),
            }
        }
        let Some(end) = bytes[..length].windows(4).position(|v| v == b"\r\n\r\n") else {
            return Err((431, "{\"error\":\"headers_too_large\"}"));
        };
        let request = std::str::from_utf8(&bytes[..end])
            .map_err(|_| (400, "{\"error\":\"invalid_request\"}"))?;
        let mut parts = request
            .lines()
            .next()
            .unwrap_or_default()
            .split_ascii_whitespace();
        let method = parts.next().unwrap_or_default();
        let path = parts.next().unwrap_or_default();
        if parts.next() != Some("HTTP/1.1") || parts.next().is_some() || method != "GET" {
            return Err((400, "{\"error\":\"invalid_request\"}"));
        }
        let invalid = (400, "{\"error\":\"invalid_request\"}");
        let mut body_length = None;
        for header in request.split("\r\n").skip(1) {
            let (name, value) = header.split_once(':').ok_or(invalid)?;
            if name.eq_ignore_ascii_case("transfer-encoding") {
                return Err(invalid);
            }
            if name.eq_ignore_ascii_case("content-length") {
                let value = value.trim();
                if body_length.is_some()
                    || value.is_empty()
                    || !value.bytes().all(|b| b.is_ascii_digit())
                {
                    return Err(invalid);
                }
                body_length = Some(value.parse::<usize>().map_err(|_| invalid)?);
            }
        }
        if body_length.unwrap_or(0) != 0 || length != end + 4 {
            return Err((413, "{\"error\":\"body_too_large\"}"));
        }
        let mut credentials = request.lines().filter_map(|header| {
            let (name, value) = header.split_once(':')?;
            name.eq_ignore_ascii_case("authorization")
                .then_some(value.trim())
        });
        if credentials.next().and_then(|v| v.strip_prefix("Bearer ")) != Some(bearer)
            || credentials.next().is_some()
        {
            return Err((401, "{\"error\":\"unauthorized\"}"));
        }
        Ok(Some(path.to_owned()))
    }

    /// Serves one bounded loopback health request. The caller supplies a stream with
    /// an absolute request deadline (the daemon uses DeadlineStream), and probes live
    /// dependencies only after the distinct inbound health credential is authenticated.
    ///
    /// # Errors
    /// Returns a transport error if the response cannot be written.
    pub fn serve_health<S: Read + Write>(
        stream: &mut S,
        bearer: &str,
        owner_ready: impl Fn() -> bool,
        dependencies_ready: impl Fn() -> Result<(), String>,
    ) -> Result<(), String> {
        let (status, body) = match health_request(stream, bearer) {
            Ok(None) => return Ok(()),
            Err(refusal) => refusal,
            Ok(Some(path)) if path != "/healthz" => (404, "{\"error\":\"not_found\"}"),
            Ok(Some(_)) => {
                if owner_ready() && dependencies_ready().is_ok() && owner_ready() {
                    (200, "{\"ready\":true}")
                } else {
                    (503, "{\"ready\":false}")
                }
            }
        };
        let reason = if status < 300 { "OK" } else { "Refused" };
        write!(stream, "HTTP/1.1 {status} {reason}\r\nContent-Type: application/json\r\nCache-Control: no-store\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len())
        .map_err(|error| format!("agent response failed: {error}"))
    }
}
