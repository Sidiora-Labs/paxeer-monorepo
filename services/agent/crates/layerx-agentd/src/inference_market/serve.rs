use std::collections::BTreeMap;
use std::fs::{File, OpenOptions};
use std::io::{self, BufRead, BufReader, Read, Write};
use std::os::unix::fs::{MetadataExt as _, OpenOptionsExt as _};
use std::os::unix::net::UnixStream;
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex};
use std::time::Duration;

use layerx_agentd::boot::InferenceBootGate;
use layerx_agentd::degraded::Mode;
use layerx_agentd::node_rpc::{AskStreamState, NodeRpc};
use layerx_client::ai_market::market_program::{
    codec::Reader,
    laps::LapReceipt,
    rooms::{DealRecord, DealState, RoomBook, RoomRecord, RoomState},
    state,
    types::{Digest32, Signature64},
};
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};

pub const BODY_LIMIT: usize = 1_048_576;
const HEADER_LIMIT: usize = 16_384;
const EVENT_LIMIT: usize = 65_536;
const RUNTIME_TIMEOUT: Duration = Duration::from_secs(30);

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ServeError {
    pub status: u16,
    pub code: &'static str,
}
impl ServeError {
    fn new(status: u16, code: &'static str) -> Self {
        Self { status, code }
    }
    fn invalid() -> Self {
        Self::new(400, "invalid_request_error")
    }
    fn upstream() -> Self {
        Self::new(502, "invalid_completion_receipt")
    }
    fn unavailable() -> Self {
        Self::new(503, "completion_unavailable")
    }
}
impl std::fmt::Display for ServeError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}: {}", self.status, self.code)
    }
}
impl std::error::Error for ServeError {}
impl From<io::Error> for ServeError {
    fn from(_: io::Error) -> Self {
        Self::unavailable()
    }
}

pub struct CompletionConnection<'a, S> {
    prefix: io::Cursor<Vec<u8>>,
    stream: &'a mut S,
    completion: bool,
}
impl<'a, S: Read> CompletionConnection<'a, S> {
    pub fn read(stream: &'a mut S) -> Result<Self, ServeError> {
        let mut prefix = Vec::new();
        while !prefix.ends_with(b"\r\n") {
            if prefix.len() >= HEADER_LIMIT {
                return Err(ServeError::new(431, "headers_too_large"));
            }
            let mut byte = [0];
            stream
                .read_exact(&mut byte)
                .map_err(|_| ServeError::invalid())?;
            prefix.push(byte[0]);
        }
        let completion = prefix
            .split(|byte| *byte == b' ')
            .nth(1)
            .is_some_and(|path| matches!(path, b"/v1/chat/completions" | b"/v1/messages"));
        Ok(Self {
            prefix: io::Cursor::new(prefix),
            stream,
            completion,
        })
    }
    pub fn is_completion(&self) -> bool {
        self.completion
    }
}
impl<S: Read> Read for CompletionConnection<'_, S> {
    fn read(&mut self, bytes: &mut [u8]) -> io::Result<usize> {
        if self.prefix.position() < self.prefix.get_ref().len() as u64 {
            self.prefix.read(bytes)
        } else {
            self.stream.read(bytes)
        }
    }
}
impl<S: Write> Write for CompletionConnection<'_, S> {
    fn write(&mut self, bytes: &[u8]) -> io::Result<usize> {
        self.stream.write(bytes)
    }
    fn flush(&mut self) -> io::Result<()> {
        self.stream.flush()
    }
}

/// Admission uses money and the stream's integer rate, never an estimated token count.
#[derive(Clone, Debug)]
pub struct AskGate {
    stream: AskStreamState,
    token_budget: u64,
}
impl AskGate {
    pub fn read(node: &NodeRpc, ask: &[u8; 32]) -> Result<Self, ServeError> {
        Self::check(
            node.ask_stream(ask)
                .map_err(|_| ServeError::unavailable())?,
        )
    }

    pub fn check(stream: Option<AskStreamState>) -> Result<Self, ServeError> {
        let refused = || ServeError::new(402, "ask_stream_unfunded");
        let stream = stream.ok_or_else(refused)?;
        if stream.stream_id == [0; 32]
            || stream.closed
            || stream.paused
            || stream.underfunded
            || stream.total_cap == 0
            || stream.accrued_total >= stream.total_cap
            || stream.settled_total >= stream.total_cap
        {
            return Err(refused());
        }
        if stream.mode != 2
            || stream.rate == 0
            || stream.rate_unit == 0
            || stream.meter_authorities.len() != 1
            || stream.meter_authorities[0] == [0; 32]
            || stream.settled_total > stream.accrued_total
            || stream.remainder_carry >= u128::from(stream.rate_unit)
        {
            return Err(ServeError::upstream());
        }
        let credit = stream
            .total_cap
            .checked_sub(stream.accrued_total)
            .and_then(|value| value.checked_mul(u128::from(stream.rate_unit)))
            .and_then(|value| value.checked_sub(stream.remainder_carry))
            .ok_or_else(ServeError::upstream)?;
        let tokens = credit / stream.rate;
        let token_budget =
            u64::try_from(tokens.min(u128::from(u64::MAX))).map_err(|_| ServeError::upstream())?;
        if token_budget == 0 {
            return Err(refused());
        }
        Ok(Self {
            stream,
            token_budget,
        })
    }
}

#[derive(Clone, Debug)]
pub struct CompletionBinding {
    pub room: RoomRecord,
    pub deal: DealRecord,
}
impl CompletionBinding {
    #[allow(
        clippy::large_types_passed_by_value,
        reason = "large_types_passed_by_value: tests/inference_serve.rs passes RoomRecord and DealRecord by value to CompletionBinding::new"
    )]
    pub fn new(room: RoomRecord, deal: DealRecord) -> Result<Self, ServeError> {
        room.profile
            .validate()
            .map_err(|_| ServeError::upstream())?;
        if room.room_id != deal.room_id
            || room.state != RoomState::Open
            || deal.state != DealState::Open
            || deal.layers.iter().flatten().count() == 0
        {
            return Err(ServeError::unavailable());
        }
        Ok(Self { room, deal })
    }
}

impl super::DealEngine {
    pub fn completion_binding(&self, ask: &[u8; 32]) -> Result<CompletionBinding, ServeError> {
        let negotiation = self
            .session
            .live
            .iter()
            .find(|deal| deal.frame.ask_id == *ask)
            .ok_or_else(ServeError::unavailable)?;
        if !self
            .live(&negotiation.frame.deal_id)
            .map_err(|_| ServeError::unavailable())?
        {
            return Err(ServeError::unavailable());
        }
        let snapshot = self.snapshot.as_ref().ok_or_else(ServeError::unavailable)?;
        let shared = state::decode_shared_state(snapshot.state_bytes())
            .map_err(|_| ServeError::upstream())?;
        let book = RoomBook::decode(shared.room_market().map_err(|_| ServeError::upstream())?.1)
            .map_err(|_| ServeError::upstream())?;
        let room = book
            .room(Digest32::new(negotiation.frame.room_id).map_err(|_| ServeError::upstream())?)
            .map_err(|_| ServeError::upstream())?
            .ok_or_else(ServeError::unavailable)?;
        let deal = book
            .deal(Digest32::new(negotiation.frame.deal_id).map_err(|_| ServeError::upstream())?)
            .map_err(|_| ServeError::upstream())?
            .ok_or_else(ServeError::unavailable)?;
        CompletionBinding::new(room, deal)
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum Shape {
    OpenAi,
    Anthropic,
}

#[derive(Clone, Debug, Deserialize, Serialize, Eq, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ChatMessage {
    pub role: String,
    pub content: String,
}

#[derive(Clone, Debug)]
pub struct CompletionRequest {
    pub ask: [u8; 32],
    pub shape: Shape,
    pub streaming: bool,
    pub messages: Vec<ChatMessage>,
    pub requested_tokens: Option<u64>,
    pub parameters: Value,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub struct RuntimeRequest {
    pub request_id: [u8; 32],
    pub ask_id: [u8; 32],
    pub room_id: [u8; 32],
    pub deal_id: [u8; 32],
    pub model_digest: [u8; 32],
    pub messages: Vec<ChatMessage>,
    pub max_generated_tokens: u64,
    pub token_budget: u64,
    pub next_lap_index: u64,
    pub cumulative_tokens: u64,
    pub parameters: Value,
}

#[derive(Clone, Debug, Deserialize, Serialize, Eq, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct TokenPiece {
    pub token_id: u32,
    pub text: String,
}

#[derive(Clone, Copy, Debug, Deserialize, Serialize, Eq, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum FinishReason {
    Stop,
    Length,
}

#[derive(Debug, Deserialize, Serialize)]
#[serde(tag = "type", rename_all = "snake_case", deny_unknown_fields)]
enum RuntimeEvent {
    Lap {
        request_id: [u8; 32],
        receipt: Vec<u8>,
        tokens: Vec<TokenPiece>,
    },
    Done {
        request_id: [u8; 32],
        reason: FinishReason,
    },
}

fn write_frame(
    writer: &mut impl Write,
    value: &impl Serialize,
    limit: usize,
) -> Result<(), ServeError> {
    let bytes = serde_json::to_vec(value).map_err(|_| ServeError::upstream())?;
    if bytes.len() > limit {
        return Err(ServeError::upstream());
    }
    let length = u32::try_from(bytes.len()).map_err(|_| ServeError::upstream())?;
    writer.write_all(&length.to_be_bytes())?;
    writer.write_all(&bytes)?;
    writer.flush()?;
    Ok(())
}
fn read_frame<T: serde::de::DeserializeOwned>(
    reader: &mut impl Read,
    limit: usize,
) -> Result<T, ServeError> {
    let mut length = [0; 4];
    reader.read_exact(&mut length)?;
    let length = u32::from_be_bytes(length) as usize;
    if length == 0 || length > limit {
        return Err(ServeError::upstream());
    }
    let mut bytes = vec![0; length];
    reader.read_exact(&mut bytes)?;
    serde_json::from_slice(&bytes).map_err(|_| ServeError::upstream())
}

/// The producer half of the local completion transport. The lap engine supplies
/// canonical, signed program receipts alongside the tokenizer's individual tokens.
#[cfg(test)]
pub struct CompletionWriter<W> {
    writer: W,
    request: RuntimeRequest,
}
#[cfg(test)]
impl<W: Read + Write> CompletionWriter<W> {
    pub fn accept(mut writer: W) -> Result<Self, ServeError> {
        let request: RuntimeRequest = read_frame(&mut writer, 2 * BODY_LIMIT)?;
        if request.max_generated_tokens == 0 || request.token_budget == 0 {
            return Err(ServeError::invalid());
        }
        Ok(Self { writer, request })
    }
    pub fn request(&self) -> &RuntimeRequest {
        &self.request
    }
    pub fn lap(
        &mut self,
        receipt: &LapReceipt<'_>,
        tokens: Vec<TokenPiece>,
    ) -> Result<(), ServeError> {
        if receipt.deal_id.bytes() != self.request.deal_id
            || receipt.generated_tokens != tokens.len() as u64
        {
            return Err(ServeError::upstream());
        }
        let mut bytes = vec![0; 2048];
        let length = receipt
            .encode(&mut bytes)
            .map_err(|_| ServeError::upstream())?;
        bytes.truncate(length);
        write_frame(
            &mut self.writer,
            &RuntimeEvent::Lap {
                request_id: self.request.request_id,
                receipt: bytes,
                tokens,
            },
            EVENT_LIMIT,
        )
    }
    pub fn finish(mut self, reason: FinishReason) -> Result<(), ServeError> {
        write_frame(
            &mut self.writer,
            &RuntimeEvent::Done {
                request_id: self.request.request_id,
                reason,
            },
            EVENT_LIMIT,
        )
    }
}

struct OwnedReceipt {
    deal_id: Digest32,
    lap_index: u64,
    prompt_tokens: u64,
    generated_tokens: u64,
    cumulative_tokens: u64,
    ranges: Vec<Digest32>,
    signature: Signature64,
    countersigners: u32,
}
impl OwnedReceipt {
    fn decode(bytes: &[u8]) -> Result<Self, ServeError> {
        let decode = || -> Result<Self, layerx_client::ai_market::market_program::errors::ApplicationError> {
            let mut r = Reader::new(bytes);
            let deal_id = Digest32::new(r.fixed()?)?;
            let lap_index = r.u64()?;
            let prompt_tokens = r.u64()?;
            let generated_tokens = r.u64()?;
            let cumulative_tokens = r.u64()?;
            let count = usize::from(r.u8()?);
            if !(1..=32).contains(&count) {
                return Err(layerx_client::ai_market::market_program::errors::NON_CANONICAL);
            }
            let mut ranges = Vec::with_capacity(count);
            for _ in 0..count { ranges.push(Digest32::new(r.fixed()?)?); }
            let signature = Signature64(r.fixed()?);
            let countersigners = r.u32()?;
            r.finish()?;
            Ok(Self { deal_id, lap_index, prompt_tokens, generated_tokens,
                cumulative_tokens, ranges, signature, countersigners })
        };
        let value = decode().map_err(|_| ServeError::upstream())?;
        value
            .receipt()
            .digest()
            .map_err(|_| ServeError::upstream())?;
        Ok(value)
    }
    fn receipt(&self) -> LapReceipt<'_> {
        LapReceipt {
            deal_id: self.deal_id,
            lap_index: self.lap_index,
            prompt_tokens: self.prompt_tokens,
            generated_tokens: self.generated_tokens,
            cumulative_tokens: self.cumulative_tokens,
            layer_range_digests: &self.ranges,
            host_signature: self.signature,
            peer_countersignatures: self.countersigners,
        }
    }
    fn verify(&self, key: &[u8; 32]) -> Result<(), ServeError> {
        let digest = self
            .receipt()
            .signing_digest()
            .map_err(|_| ServeError::upstream())?;
        layerx_crypto::ed25519::verify_digest(key, &self.signature.0, &digest.bytes())
            .map_err(|_| ServeError::upstream())
    }
}

#[derive(Clone, Copy, Debug, Default, Eq, PartialEq)]
pub struct LapUsage {
    pub prompt_tokens: u64,
    pub generated_tokens: u64,
}
impl LapUsage {
    fn total(self) -> Result<u64, ServeError> {
        self.prompt_tokens
            .checked_add(self.generated_tokens)
            .ok_or_else(ServeError::upstream)
    }
    fn openai(self) -> Result<Value, ServeError> {
        Ok(
            json!({"prompt_tokens":self.prompt_tokens,"completion_tokens":self.generated_tokens,"total_tokens":self.total()?}),
        )
    }
}

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct RetainedLap {
    ask: [u8; 32],
    stream: [u8; 32],
    request: [u8; 32],
    key: [u8; 32],
    receipt: Vec<u8>,
}
#[derive(Default)]
struct Journal {
    file: Option<File>,
    cursors: BTreeMap<[u8; 32], Cursor>,
}
#[derive(Clone, Copy)]
struct Cursor {
    stream: [u8; 32],
    deal: [u8; 32],
    next_lap: u64,
    cumulative: u64,
}
impl Journal {
    fn open(path: Option<&Path>) -> Result<Self, ServeError> {
        let Some(path) = path else {
            return Ok(Self::default());
        };
        let file = OpenOptions::new()
            .read(true)
            .append(true)
            .create(true)
            .mode(0o600)
            .open(path)?;
        let metadata = file.metadata()?;
        if !metadata.is_file() || metadata.mode() & 0o077 != 0 {
            return Err(ServeError::unavailable());
        }
        let mut journal = Self {
            file: Some(file.try_clone()?),
            cursors: BTreeMap::new(),
        };
        let mut reader = BufReader::new(file);
        loop {
            let mut bytes = Vec::new();
            let count = reader
                .by_ref()
                .take((EVENT_LIMIT + 1) as u64)
                .read_until(b'\n', &mut bytes)?;
            if count == 0 {
                break;
            }
            if count > EVENT_LIMIT || !bytes.ends_with(b"\n") {
                return Err(ServeError::upstream());
            }
            let retained: RetainedLap =
                serde_json::from_slice(&bytes).map_err(|_| ServeError::upstream())?;
            let receipt = OwnedReceipt::decode(&retained.receipt)?;
            receipt.verify(&retained.key)?;
            journal.restore(&retained, &receipt)?;
        }
        Ok(journal)
    }
    fn restore(
        &mut self,
        retained: &RetainedLap,
        receipt: &OwnedReceipt,
    ) -> Result<(), ServeError> {
        if let Some(previous) = self.cursors.get(&retained.ask) {
            if previous.stream != retained.stream
                || receipt.cumulative_tokens
                    != previous
                        .cumulative
                        .checked_add(receipt.prompt_tokens)
                        .and_then(|value| value.checked_add(receipt.generated_tokens))
                        .ok_or_else(ServeError::upstream)?
                || previous.deal == receipt.deal_id.bytes()
                    && receipt.lap_index != previous.next_lap
            {
                return Err(ServeError::upstream());
            }
        }
        self.cursors.insert(
            retained.ask,
            Cursor {
                stream: retained.stream,
                deal: receipt.deal_id.bytes(),
                next_lap: receipt
                    .lap_index
                    .checked_add(1)
                    .ok_or_else(ServeError::upstream)?,
                cumulative: receipt.cumulative_tokens,
            },
        );
        Ok(())
    }
    fn retain(&mut self, retained: &RetainedLap, receipt: &OwnedReceipt) -> Result<(), ServeError> {
        let mut bytes = serde_json::to_vec(retained).map_err(|_| ServeError::upstream())?;
        bytes.push(b'\n');
        if bytes.len() > EVENT_LIMIT {
            return Err(ServeError::upstream());
        }
        let file = self.file.as_mut().ok_or_else(ServeError::unavailable)?;
        file.write_all(&bytes)?;
        file.sync_data()?;
        self.restore(retained, receipt)
    }
}

pub struct ServeRoutes {
    room_id: [u8; 32],
    model_digest: [u8; 32],
    runtime: Option<PathBuf>,
    health: Arc<Mutex<InferenceBootGate>>,
    journal: Mutex<Journal>,
}
impl ServeRoutes {
    #[allow(
        clippy::needless_pass_by_value,
        reason = "needless_pass_by_value: src/main.rs and tests/inference_serve.rs pass owned receipt paths to ServeRoutes::new"
    )]
    pub fn new(
        room_id: [u8; 32],
        model_digest: [u8; 32],
        runtime: Option<PathBuf>,
        receipts: Option<PathBuf>,
        health: Arc<Mutex<InferenceBootGate>>,
    ) -> Result<Self, ServeError> {
        if room_id == [0; 32]
            || model_digest == [0; 32]
            || runtime.is_some() != receipts.is_some()
            || runtime.as_ref().is_some_and(|path| !path.is_absolute())
            || receipts.as_ref().is_some_and(|path| !path.is_absolute())
        {
            return Err(ServeError::invalid());
        }
        Ok(Self {
            room_id,
            model_digest,
            runtime,
            health,
            journal: Mutex::new(Journal::open(receipts.as_deref())?),
        })
    }
    pub fn model_alias(&self) -> String {
        layerx_programs::hex::encode(&self.model_digest)
    }

    pub fn serve(
        &self,
        stream: &mut (impl Read + Write),
        node: &NodeRpc,
        binding: impl FnOnce(&[u8; 32]) -> Result<CompletionBinding, ServeError>,
    ) -> Result<(), ServeError> {
        let request = match self.read_request(stream) {
            Ok(Some(request)) => request,
            Ok(None) => return Ok(()),
            Err(error) => return write_error(stream, &error),
        };
        let admitted = AskGate::read(node, &request.ask);
        let gate = match admitted {
            Ok(gate) => gate,
            Err(error) => return write_error(stream, &error),
        };
        let binding = match binding(&request.ask) {
            Ok(binding) => binding,
            Err(error) => return write_error(stream, &error),
        };
        self.respond(stream, request, gate, binding)
    }

    pub fn read_request(
        &self,
        stream: &mut (impl Read + Write),
    ) -> Result<Option<CompletionRequest>, ServeError> {
        let mut headers = Vec::new();
        while !headers.ends_with(b"\r\n\r\n") {
            if headers.len() >= HEADER_LIMIT {
                return Err(ServeError::new(431, "headers_too_large"));
            }
            let mut byte = [0];
            stream
                .read_exact(&mut byte)
                .map_err(|_| ServeError::invalid())?;
            headers.push(byte[0]);
        }
        let text = std::str::from_utf8(&headers).map_err(|_| ServeError::invalid())?;
        let mut lines = text.split("\r\n");
        let first = lines.next().ok_or_else(ServeError::invalid)?;
        let fields: Vec<_> = first.split(' ').collect();
        if fields.len() != 3 || fields[2] != "HTTP/1.1" {
            return Err(ServeError::invalid());
        }
        let mut values = BTreeMap::new();
        for line in lines.take_while(|line| !line.is_empty()) {
            let (name, value) = line.split_once(':').ok_or_else(ServeError::invalid)?;
            if name.is_empty()
                || !name.bytes().all(|b| b.is_ascii_alphanumeric() || b == b'-')
                || value.bytes().any(|b| b < 32 && b != b'\t' || b == 127)
                || values
                    .insert(name.to_ascii_lowercase(), value.trim())
                    .is_some()
            {
                return Err(ServeError::invalid());
            }
        }
        let ready = self
            .health
            .lock()
            .map_err(|_| ServeError::unavailable())?
            .status()
            .mode
            == Mode::Healthy;
        if fields[0] == "GET" && fields[1] == "/healthz" {
            write_json(
                stream,
                if ready { 200 } else { 503 },
                &json!({"ready":ready}),
            )?;
            return Ok(None);
        }
        let shape = match fields[1] {
            "/v1/chat/completions" => Shape::OpenAi,
            "/v1/messages" => Shape::Anthropic,
            _ => return Err(ServeError::new(404, "not_found_error")),
        };
        if fields[0] != "POST" {
            return Err(ServeError::new(405, "method_not_allowed"));
        }
        self.read_completion(stream, &values, shape, ready)
    }

    fn read_completion(
        &self,
        stream: &mut impl Read,
        values: &BTreeMap<String, &str>,
        shape: Shape,
        ready: bool,
    ) -> Result<Option<CompletionRequest>, ServeError> {
        if values.contains_key("transfer-encoding") {
            return Err(ServeError::invalid());
        }
        let length = values
            .get("content-length")
            .ok_or_else(|| ServeError::new(411, "length_required"))?;
        if length.is_empty() || !length.bytes().all(|b| b.is_ascii_digit()) {
            return Err(ServeError::invalid());
        }
        let length: usize = length
            .parse()
            .map_err(|_| ServeError::new(413, "request_too_large"))?;
        if length > BODY_LIMIT {
            return Err(ServeError::new(413, "request_too_large"));
        }
        if values
            .get("content-type")
            .and_then(|v| v.split(';').next())
            .is_none_or(|v| !v.trim().eq_ignore_ascii_case("application/json"))
        {
            return Err(ServeError::new(415, "unsupported_media_type"));
        }
        let auth = values
            .get("authorization")
            .ok_or_else(|| ServeError::new(401, "authentication_error"))?;
        let (scheme, token) = auth
            .split_once(' ')
            .ok_or_else(|| ServeError::new(401, "authentication_error"))?;
        let token = token.strip_prefix("0x").unwrap_or(token);
        if !scheme.eq_ignore_ascii_case("Bearer")
            || token.len() != 64
            || !token
                .bytes()
                .all(|b| b.is_ascii_digit() || (b'a'..=b'f').contains(&b))
        {
            return Err(ServeError::new(401, "authentication_error"));
        }
        let ask = layerx_programs::hex::decode_digest(token)
            .map_err(|_| ServeError::new(401, "authentication_error"))?;
        if ask == [0; 32] {
            return Err(ServeError::new(401, "authentication_error"));
        }
        let mut bytes = vec![0; length];
        stream
            .read_exact(&mut bytes)
            .map_err(|_| ServeError::invalid())?;
        let body: Value = serde_json::from_slice(&bytes).map_err(|_| ServeError::invalid())?;
        self.parse_completion(ask, shape, ready, &body)
    }

    fn parse_completion(
        &self,
        ask: [u8; 32],
        shape: Shape,
        ready: bool,
        body: &Value,
    ) -> Result<Option<CompletionRequest>, ServeError> {
        let object = body.as_object().ok_or_else(ServeError::invalid)?;
        let model = object
            .get("model")
            .and_then(Value::as_str)
            .ok_or_else(ServeError::invalid)?;
        if model != self.model_alias() {
            return Err(ServeError::new(404, "model_not_found"));
        }
        if !ready {
            return Err(ServeError::unavailable());
        }
        if object.keys().any(|key| {
            !matches!(
                key.as_str(),
                "model"
                    | "messages"
                    | "system"
                    | "max_tokens"
                    | "max_completion_tokens"
                    | "stream"
                    | "stream_options"
                    | "temperature"
                    | "top_p"
                    | "stop"
            )
        }) {
            return Err(ServeError::invalid());
        }
        let streaming = match body.get("stream") {
            None => false,
            Some(Value::Bool(value)) => *value,
            _ => return Err(ServeError::invalid()),
        };
        if body.get("max_tokens").is_some() && body.get("max_completion_tokens").is_some() {
            return Err(ServeError::invalid());
        }
        let maximum = body
            .get("max_tokens")
            .or_else(|| body.get("max_completion_tokens"));
        let requested_tokens = maximum
            .map(|value| {
                value
                    .as_u64()
                    .filter(|n| *n > 0)
                    .ok_or_else(ServeError::invalid)
            })
            .transpose()?;
        if shape == Shape::Anthropic
            && (requested_tokens.is_none() || body.get("max_completion_tokens").is_some())
        {
            return Err(ServeError::invalid());
        }
        let mut messages = Vec::new();
        if let Some(system) = body.get("system") {
            if shape != Shape::Anthropic {
                return Err(ServeError::invalid());
            }
            messages.push(ChatMessage {
                role: "system".to_owned(),
                content: content_text(system)?,
            });
        }
        let input = body["messages"]
            .as_array()
            .filter(|values| !values.is_empty())
            .ok_or_else(ServeError::invalid)?;
        for message in input {
            let role = message["role"].as_str().ok_or_else(ServeError::invalid)?;
            if !matches!(role, "user" | "assistant" | "system")
                || shape == Shape::Anthropic && role == "system"
            {
                return Err(ServeError::invalid());
            }
            messages.push(ChatMessage {
                role: role.to_owned(),
                content: content_text(&message["content"])?,
            });
        }
        let mut parameters = serde_json::Map::new();
        for key in ["temperature", "top_p", "stop"] {
            if let Some(value) = body.get(key) {
                parameters.insert(key.to_owned(), value.clone());
            }
        }
        Ok(Some(CompletionRequest {
            ask,
            shape,
            streaming,
            messages,
            requested_tokens,
            parameters: Value::Object(parameters),
        }))
    }

    #[allow(
        clippy::needless_pass_by_value,
        reason = "needless_pass_by_value: tests/inference_serve.rs passes owned CompletionRequest, AskGate and CompletionBinding to respond"
    )]
    pub fn respond(
        &self,
        stream: &mut (impl Read + Write),
        request: CompletionRequest,
        gate: AskGate,
        binding: CompletionBinding,
    ) -> Result<(), ServeError> {
        let mut started = false;
        let result = match request.shape {
            Shape::OpenAi => self.chat_completions(stream, &request, &gate, &binding, &mut started),
            Shape::Anthropic => self.messages(stream, &request, &gate, &binding, &mut started),
        };
        match result {
            Ok(()) => Ok(()),
            Err(error) if !started => write_error(stream, &error),
            Err(error) => {
                sse(
                    stream,
                    Some("error"),
                    &json!({"type":"error","error":{"type":error.code,"message":error.code}}),
                )?;
                Err(error)
            }
        }
    }
    pub fn chat_completions(
        &self,
        stream: &mut (impl Read + Write),
        request: &CompletionRequest,
        gate: &AskGate,
        binding: &CompletionBinding,
        started: &mut bool,
    ) -> Result<(), ServeError> {
        self.complete(stream, request, gate, binding, started)
    }
    pub fn messages(
        &self,
        stream: &mut (impl Read + Write),
        request: &CompletionRequest,
        gate: &AskGate,
        binding: &CompletionBinding,
        started: &mut bool,
    ) -> Result<(), ServeError> {
        self.complete(stream, request, gate, binding, started)
    }

    fn prepare_runtime_request(
        &self,
        request: &CompletionRequest,
        gate: &AskGate,
        binding: &CompletionBinding,
        journal: &Journal,
        max_tokens: u64,
    ) -> Result<RuntimeRequest, ServeError> {
        let previous = journal.cursors.get(&request.ask).copied();
        if previous.is_some_and(|cursor| cursor.stream != gate.stream.stream_id) {
            return Err(ServeError::upstream());
        }
        let cumulative = previous.map_or(gate.stream.cumulative_meter, |cursor| {
            cursor.cumulative.max(gate.stream.cumulative_meter)
        });
        let outstanding = cumulative
            .checked_sub(gate.stream.cumulative_meter)
            .ok_or_else(ServeError::upstream)?;
        let token_budget = gate
            .token_budget
            .checked_sub(outstanding)
            .filter(|n| *n > 0)
            .ok_or_else(|| ServeError::new(402, "ask_stream_unfunded"))?;
        let next_lap = previous
            .filter(|cursor| cursor.deal == binding.deal.deal_id.bytes())
            .map_or(0, |cursor| cursor.next_lap);
        let mut request_id = [0; 32];
        getrandom::fill(&mut request_id).map_err(|_| ServeError::unavailable())?;
        Ok(RuntimeRequest {
            request_id,
            ask_id: request.ask,
            room_id: self.room_id,
            deal_id: binding.deal.deal_id.bytes(),
            model_digest: self.model_digest,
            messages: request.messages.clone(),
            max_generated_tokens: max_tokens,
            token_budget,
            next_lap_index: next_lap,
            cumulative_tokens: cumulative,
            parameters: request.parameters.clone(),
        })
    }

    fn complete(
        &self,
        stream: &mut (impl Read + Write),
        request: &CompletionRequest,
        gate: &AskGate,
        binding: &CompletionBinding,
        started: &mut bool,
    ) -> Result<(), ServeError> {
        if binding.room.room_id.bytes() != self.room_id
            || binding.room.model_digest.bytes() != self.model_digest
            || binding.deal.ask_id.bytes() != request.ask
            || binding.deal.room_id != binding.room.room_id
            || binding.room.state != RoomState::Open
            || binding.deal.state != DealState::Open
            || binding.room.profile.settlement_account.bytes() != gate.stream.recipient
        {
            return Err(ServeError::unavailable());
        }
        binding
            .room
            .profile
            .validate()
            .map_err(|_| ServeError::upstream())?;
        let profile = binding.room.profile;
        let max_tokens = request
            .requested_tokens
            .unwrap_or(profile.max_generated_tokens)
            .min(profile.max_generated_tokens);
        let socket = self.runtime.as_ref().ok_or_else(ServeError::unavailable)?;
        // Serialize admission and retain debits before exposing tokens. An unmetered
        // receipt remains charged across the next request and a daemon restart.
        let mut journal = self.journal.lock().map_err(|_| ServeError::unavailable())?;
        let runtime_request =
            self.prepare_runtime_request(request, gate, binding, &journal, max_tokens)?;
        let mut runtime = UnixStream::connect(socket)?;
        runtime.set_read_timeout(Some(RUNTIME_TIMEOUT))?;
        runtime.set_write_timeout(Some(RUNTIME_TIMEOUT))?;
        write_frame(&mut runtime, &runtime_request, 2 * BODY_LIMIT)?;
        let mut session = CompletionSession {
            request,
            gate,
            binding,
            expected_lap: runtime_request.next_lap_index,
            expected_cumulative: runtime_request.cumulative_tokens,
            id: format!(
                "{}{}",
                if request.shape == Shape::OpenAi {
                    "chatcmpl-"
                } else {
                    "msg_"
                },
                layerx_programs::hex::encode(&runtime_request.request_id)
            ),
            runtime_request,
            usage: LapUsage::default(),
            text: String::new(),
            lap_count: 0,
        };
        let reason = session.receive(self, stream, &mut runtime, &mut journal, started)?;
        let CompletionSession {
            usage, text, id, ..
        } = session;
        if request.streaming {
            finish_events(
                stream,
                request.shape,
                &id,
                &self.model_alias(),
                usage,
                reason,
            )
        } else {
            let body = match request.shape {
                Shape::OpenAi => {
                    json!({"id":id,"object":"chat.completion","created":0,"model":self.model_alias(),
                    "choices":[{"index":0,"message":{"role":"assistant","content":text},"finish_reason":openai_reason(reason)}],"usage":usage.openai()?})
                }
                Shape::Anthropic => {
                    json!({"id":id,"type":"message","role":"assistant","model":self.model_alias(),
                    "content":[{"type":"text","text":text}],"stop_reason":anthropic_reason(reason),"stop_sequence":null,
                    "usage":{"input_tokens":usage.prompt_tokens,"output_tokens":usage.generated_tokens}})
                }
            };
            write_json(stream, 200, &body)
        }
    }
}

struct CompletionSession<'a> {
    request: &'a CompletionRequest,
    gate: &'a AskGate,
    binding: &'a CompletionBinding,
    runtime_request: RuntimeRequest,
    expected_lap: u64,
    expected_cumulative: u64,
    lap_count: i32,
    usage: LapUsage,
    text: String,
    id: String,
}
impl CompletionSession<'_> {
    fn receive(
        &mut self,
        routes: &ServeRoutes,
        stream: &mut impl Write,
        runtime: &mut UnixStream,
        journal: &mut Journal,
        started: &mut bool,
    ) -> Result<FinishReason, ServeError> {
        loop {
            if routes
                .health
                .lock()
                .map_err(|_| ServeError::unavailable())?
                .status()
                .mode
                != Mode::Healthy
            {
                return Err(ServeError::unavailable());
            }
            match read_frame::<RuntimeEvent>(runtime, EVENT_LIMIT)? {
                RuntimeEvent::Done {
                    request_id: actual,
                    reason,
                } => {
                    if actual != self.runtime_request.request_id || self.lap_count == 0 {
                        return Err(ServeError::upstream());
                    }
                    return Ok(
                        if self.usage.generated_tokens == self.runtime_request.max_generated_tokens
                        {
                            FinishReason::Length
                        } else {
                            reason
                        },
                    );
                }
                RuntimeEvent::Lap {
                    request_id: actual,
                    receipt: bytes,
                    tokens,
                } => {
                    self.accept_lap(actual, bytes, &tokens, journal)?;
                    if self.request.streaming && !*started {
                        write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\nCache-Control: no-store\r\nConnection: close\r\nX-Accel-Buffering: no\r\n\r\n")?;
                        *started = true;
                        start_events(
                            stream,
                            self.request.shape,
                            &self.id,
                            &routes.model_alias(),
                            self.usage.prompt_tokens,
                        )?;
                    }
                    for token in tokens {
                        self.text.push_str(&token.text);
                        if self.request.streaming {
                            token_event(
                                stream,
                                self.request.shape,
                                &self.id,
                                &routes.model_alias(),
                                &token.text,
                            )?;
                        }
                    }
                }
            }
        }
    }

    fn accept_lap(
        &mut self,
        actual: [u8; 32],
        bytes: Vec<u8>,
        tokens: &[TokenPiece],
        journal: &mut Journal,
    ) -> Result<(), ServeError> {
        if actual != self.runtime_request.request_id {
            return Err(ServeError::upstream());
        }
        let receipt = OwnedReceipt::decode(&bytes)?;
        receipt.verify(&self.gate.stream.meter_authorities[0])?;
        let peers = self.binding.deal.layers.iter().flatten().count();
        let bitmap = if peers == 32 {
            u32::MAX
        } else {
            (1_u32 << peers) - 1
        };
        let added = receipt
            .prompt_tokens
            .checked_add(receipt.generated_tokens)
            .ok_or_else(ServeError::upstream)?;
        let prompt = self
            .usage
            .prompt_tokens
            .checked_add(receipt.prompt_tokens)
            .ok_or_else(ServeError::upstream)?;
        let generated = self
            .usage
            .generated_tokens
            .checked_add(receipt.generated_tokens)
            .ok_or_else(ServeError::upstream)?;
        self.expected_cumulative = self
            .expected_cumulative
            .checked_add(added)
            .ok_or_else(ServeError::upstream)?;
        if receipt.deal_id.bytes() != self.runtime_request.deal_id
            || receipt.lap_index != self.expected_lap
            || receipt.cumulative_tokens != self.expected_cumulative
            || receipt.generated_tokens != tokens.len() as u64
            || receipt.generated_tokens > self.binding.room.profile.lap_size_tokens
            || generated > self.runtime_request.max_generated_tokens
            || prompt
                .checked_add(generated)
                .ok_or_else(ServeError::upstream)?
                > self.runtime_request.token_budget
            || receipt.ranges.len() != peers
            || receipt.countersigners != bitmap
            || self.lap_count > 0 && receipt.prompt_tokens != 0
            || self.lap_count > 0 && added == 0
        {
            return Err(ServeError::upstream());
        }
        let added_bytes = tokens
            .iter()
            .try_fold(0_usize, |n, piece| n.checked_add(piece.text.len()))
            .ok_or_else(ServeError::upstream)?;
        if self
            .text
            .len()
            .checked_add(added_bytes)
            .is_none_or(|size| size > BODY_LIMIT)
        {
            return Err(ServeError::upstream());
        }
        journal.retain(
            &RetainedLap {
                ask: self.request.ask,
                stream: self.gate.stream.stream_id,
                request: self.runtime_request.request_id,
                key: self.gate.stream.meter_authorities[0],
                receipt: bytes,
            },
            &receipt,
        )?;
        self.expected_lap = self
            .expected_lap
            .checked_add(1)
            .ok_or_else(ServeError::upstream)?;
        self.lap_count += 1;
        self.usage = LapUsage {
            prompt_tokens: prompt,
            generated_tokens: generated,
        };
        Ok(())
    }
}

fn content_text(value: &Value) -> Result<String, ServeError> {
    if let Some(text) = value.as_str() {
        return Ok(text.to_owned());
    }
    let blocks = value.as_array().ok_or_else(ServeError::invalid)?;
    let mut text = String::new();
    for block in blocks {
        if block["type"] != "text" {
            return Err(ServeError::invalid());
        }
        text.push_str(block["text"].as_str().ok_or_else(ServeError::invalid)?);
    }
    Ok(text)
}
fn write_json(stream: &mut impl Write, status: u16, body: &Value) -> Result<(), ServeError> {
    let bytes = serde_json::to_vec(body).map_err(|_| ServeError::upstream())?;
    write!(stream, "HTTP/1.1 {status} Response\r\nContent-Type: application/json\r\nContent-Length: {}\r\nCache-Control: no-store\r\nConnection: close\r\n{}\r\n", bytes.len(), if status == 503 { "Retry-After: 1\r\n" } else { "" })?;
    stream.write_all(&bytes)?;
    stream.flush()?;
    Ok(())
}
pub fn write_error(stream: &mut impl Write, error: &ServeError) -> Result<(), ServeError> {
    write_json(
        stream,
        error.status,
        &json!({"type":"error","error":{"type":error.code,"message":error.code}}),
    )
}
fn sse(stream: &mut impl Write, event: Option<&str>, value: &Value) -> Result<(), ServeError> {
    if let Some(event) = event {
        writeln!(stream, "event: {event}")?;
    }
    writeln!(stream, "data: {value}\n")?;
    stream.flush()?;
    Ok(())
}
fn chunk(id: &str, model: &str, delta: &Value, finish: &Value) -> Value {
    json!({"id":id,"object":"chat.completion.chunk","created":0,"model":model,
        "choices":[{"index":0,"delta":delta,"finish_reason":finish}]})
}
fn start_events(
    stream: &mut impl Write,
    shape: Shape,
    id: &str,
    model: &str,
    prompt: u64,
) -> Result<(), ServeError> {
    match shape {
        Shape::OpenAi => sse(
            stream,
            None,
            &chunk(
                id,
                model,
                &json!({"role":"assistant","content":""}),
                &Value::Null,
            ),
        ),
        Shape::Anthropic => {
            sse(
                stream,
                Some("message_start"),
                &json!({"type":"message_start","message":{"id":id,"type":"message","role":"assistant","content":[],"model":model,"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":prompt,"output_tokens":0}}}),
            )?;
            sse(
                stream,
                Some("content_block_start"),
                &json!({"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}),
            )
        }
    }
}
fn token_event(
    stream: &mut impl Write,
    shape: Shape,
    id: &str,
    model: &str,
    text: &str,
) -> Result<(), ServeError> {
    match shape {
        Shape::OpenAi => sse(
            stream,
            None,
            &chunk(id, model, &json!({"content":text}), &Value::Null),
        ),
        Shape::Anthropic => sse(
            stream,
            Some("content_block_delta"),
            &json!({"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":text}}),
        ),
    }
}
fn openai_reason(reason: FinishReason) -> &'static str {
    match reason {
        FinishReason::Stop => "stop",
        FinishReason::Length => "length",
    }
}
fn anthropic_reason(reason: FinishReason) -> &'static str {
    match reason {
        FinishReason::Stop => "end_turn",
        FinishReason::Length => "max_tokens",
    }
}
fn finish_events(
    stream: &mut impl Write,
    shape: Shape,
    id: &str,
    model: &str,
    usage: LapUsage,
    reason: FinishReason,
) -> Result<(), ServeError> {
    match shape {
        Shape::OpenAi => {
            sse(
                stream,
                None,
                &chunk(id, model, &json!({}), &json!(openai_reason(reason))),
            )?;
            sse(
                stream,
                None,
                &json!({"id":id,"object":"chat.completion.chunk","created":0,"model":model,"choices":[],"usage":usage.openai()?}),
            )?;
            stream.write_all(b"data: [DONE]\n\n")?;
            stream.flush()?;
            Ok(())
        }
        Shape::Anthropic => {
            sse(
                stream,
                Some("content_block_stop"),
                &json!({"type":"content_block_stop","index":0}),
            )?;
            sse(
                stream,
                Some("message_delta"),
                &json!({"type":"message_delta","delta":{"stop_reason":anthropic_reason(reason),"stop_sequence":null},"usage":{"output_tokens":usage.generated_tokens}}),
            )?;
            sse(
                stream,
                Some("message_stop"),
                &json!({"type":"message_stop"}),
            )
        }
    }
}
