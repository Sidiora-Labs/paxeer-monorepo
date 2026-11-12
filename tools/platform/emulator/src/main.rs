mod settlement;
mod surface;

use std::collections::{HashMap, HashSet, VecDeque};
use std::ffi::{c_char, c_int, c_uchar, c_uint, c_ulonglong, c_void, CStr};
use std::fmt::Write as _;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::ptr;
use std::slice;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    mpsc::{sync_channel, SyncSender, TrySendError},
    Arc, Mutex,
};
use std::thread;
use std::time::Duration;

use ed25519_dalek::SigningKey;
use layerx_types::ids::Did;
use serde::de::DeserializeOwned;
use serde::Deserialize;
use sha2::{Digest as _, Sha256};

const DEFAULT_PORT: u16 = 9402;
const DEFAULT_NETWORK_ID: u32 = 402;
const DEFAULT_TIME_MS: u64 = 1_700_000_000_000;
const MAX_HEADER_BYTES: usize = 32 * 1024;
const MAX_REQUEST_BYTES: usize = 64 * 1024 * 1024;
const MAX_JSON_BYTES: usize = 8 * 1024 * 1024;
const IO_TIMEOUT: Duration = Duration::from_secs(10);
const PARSER_WORKERS: usize = 8;
const ADMISSION_CAPACITY: usize = 32;
const TRANSITION_CAPACITY: usize = 32;
const MAX_RECEIPTS: usize = 4096;
const MAX_RECEIPT_BYTES: usize = 1024 * 1024;
const MAX_CORE_SNAPSHOT_BYTES: usize = 24 * 1024 * 1024;
const MAX_PROGRAM_OPERATIONS: usize = 4096;
const MAX_MOVE_QUOTES: usize = 4096;
const MAX_MOVE_OPERATIONS: usize = 4096;
const MAX_EMULATOR_ACCOUNTS: usize = 512;
const MAX_PROGRAM_RESPONSE_BYTES: usize = MAX_JSON_BYTES;
const MAX_RETAINED_SIGNED_ACTIVITY_BYTES: usize = 1024 * 1024;
const RECOVERY_SNAPSHOT_MAGIC: &[u8; 8] = b"LXEMR001";
const RECOVERY_SNAPSHOT_VERSION: u32 = 4;
const RECOVERY_SNAPSHOT_DIGEST_DOMAIN: &[u8] = b"layerx-emulator-recovery-snapshot-v1\0";
const MOVE_JOURNEY_DOMAIN: &[u8] = b"layerx-emulator-move-journey-v1\0";

#[repr(C)]
struct CoreState {
    canonical_state_root: [u8; 32],
    receipt_state_root: [u8; 32],
    next_sequence: c_ulonglong,
    batch_number: c_ulonglong,
    timestamp_ms: c_ulonglong,
    cell_count: usize,
    account_count: usize,
}

unsafe extern "C" {
    fn platform_emulator_create_for_protocol(
        network_id: c_uint,
        timestamp_ms: c_ulonglong,
        sequencer_seed: *const c_uchar,
        protocol_version: u16,
    ) -> *mut c_void;
    fn platform_emulator_destroy(emulator: *mut c_void);
    fn platform_emulator_error_name(result: c_int) -> *const c_char;
    fn platform_emulator_set_time(emulator: *mut c_void, timestamp_ms: c_ulonglong) -> c_int;
    fn platform_emulator_advance_time(emulator: *mut c_void, delta_ms: c_ulonglong) -> c_int;
    fn platform_emulator_inject_failure(
        emulator: *mut c_void,
        kind: c_uint,
        count: c_ulonglong,
    ) -> c_int;
    fn platform_emulator_prefund(
        emulator: *mut c_void,
        did: *const c_uchar,
        did_length: usize,
        public_key: *const c_uchar,
        amount_hi: c_ulonglong,
        amount_lo: c_ulonglong,
    ) -> c_int;
    fn platform_emulator_inspect(emulator: *const c_void, state: *mut CoreState) -> c_int;
    fn platform_emulator_cell(
        emulator: *const c_void,
        index: usize,
        key: *mut c_uchar,
        value_hi: *mut c_ulonglong,
        value_lo: *mut c_ulonglong,
    ) -> c_int;
    fn platform_emulator_account(
        emulator: *const c_void,
        index: usize,
        id: *mut c_uchar,
        name: *mut *const c_uchar,
        name_length: *mut usize,
        balance_hi: *mut c_ulonglong,
        balance_lo: *mut c_ulonglong,
        next_sequence: *mut c_ulonglong,
    ) -> c_int;
    fn platform_emulator_identity_sequence(
        emulator: *const c_void,
        did: *const c_uchar,
        did_length: usize,
        next_sequence: *mut c_ulonglong,
    ) -> c_int;
    fn platform_emulator_owner_account_count(emulator: *const c_void, count: *mut usize) -> c_int;
    fn platform_emulator_snapshot_export(
        emulator: *mut c_void,
        bytes: *mut *const c_uchar,
        length: *mut usize,
    ) -> c_int;
    fn platform_emulator_snapshot_import(
        emulator: *mut c_void,
        bytes: *const c_uchar,
        length: usize,
    ) -> c_int;
}

struct Emulator {
    core: *mut c_void,
    signing_key: SigningKey,
    network_id: u32,
    receipts: HashMap<String, String>,
    receipt_order: VecDeque<String>,
    program_operations: HashMap<String, ProgramOperation>,
    program_activity_operations: HashMap<String, String>,
    accounts: HashMap<String, EmulatorAccount>,
    move_quotes: HashMap<String, MoveQuoteRecord>,
    move_operations: HashMap<String, MoveOperation>,
    trace: u64,
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct ProgramOperation {
    activity_id: String,
    response: String,
    retained_signed_activity: Option<String>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct EmulatorAccount {
    id: [u8; 32],
    did: String,
    public_key: [u8; 32],
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct MoveQuoteRecord {
    quote_id: String,
    source: String,
    destination: String,
    source_id: [u8; 32],
    destination_id: [u8; 32],
    amount: u128,
    currency: String,
    created_at: u64,
    expires_at: u64,
    identity_sequence: u64,
    source_sequence: u64,
    committed_idempotency: Option<String>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct MoveOperation {
    quote_id: String,
    status: u16,
    body: String,
}

impl Drop for Emulator {
    fn drop(&mut self) {
        unsafe { platform_emulator_destroy(self.core) };
    }
}

struct Config {
    listen: SocketAddr,
    network_id: u32,
    protocol_version: u16,
    timestamp_ms: u64,
    prefunds: Vec<Prefund>,
    sequencer_seed: Option<[u8; 32]>,
}

struct Prefund {
    did: String,
    public_key: [u8; 32],
    amount_hi: u64,
    amount_lo: u64,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct PrefundBody {
    did: String,
    public_key: String,
    #[serde(default)]
    amount_hi: u64,
    amount_lo: u64,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct TimestampBody {
    timestamp_ms: u64,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct AdvanceBody {
    delta_ms: u64,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct FaultBody {
    kind: String,
    #[serde(default = "one")]
    count: u64,
}

const fn one() -> u64 {
    1
}

struct Request {
    method: String,
    path: String,
    content_type: String,
    body: Vec<u8>,
}

struct Response {
    status: u16,
    content_type: &'static str,
    body: Vec<u8>,
}

struct ParsedConnection {
    request: Result<Request, String>,
    response: SyncSender<Response>,
}

fn hex_encode(bytes: &[u8]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut encoded = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        encoded.push(char::from(DIGITS[usize::from(byte >> 4)]));
        encoded.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
    encoded
}

fn nibble(value: u8) -> Option<u8> {
    match value {
        b'0'..=b'9' => Some(value - b'0'),
        b'a'..=b'f' => Some(value - b'a' + 10),
        b'A'..=b'F' => Some(value - b'A' + 10),
        _ => None,
    }
}

fn hex_decode(encoded: &str) -> Result<Vec<u8>, String> {
    let bytes = encoded.as_bytes();
    if !bytes.len().is_multiple_of(2) {
        return Err("hex input has an odd length".into());
    }
    bytes
        .chunks_exact(2)
        .map(|pair| {
            let high = nibble(pair[0]).ok_or_else(|| "invalid hex input".to_string())?;
            let low = nibble(pair[1]).ok_or_else(|| "invalid hex input".to_string())?;
            Ok((high << 4) | low)
        })
        .collect()
}

fn hash_bytes(domain: &[u8], bytes: &[u8]) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(domain);
    digest.update(bytes);
    digest.finalize().into()
}

fn valid_human_idempotency(value: &str) -> bool {
    (16..=128).contains(&value.len())
        && value
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'-' | b'_'))
}

fn core_account(emulator: &Emulator, expected_name: &str) -> Result<Option<[u8; 32]>, i32> {
    let state = inspect_state(emulator)?;
    for index in 0..state.account_count {
        let mut id = [0_u8; 32];
        let mut name = ptr::null();
        let mut name_length = 0_usize;
        let mut balance_hi = 0_u64;
        let mut balance_lo = 0_u64;
        let mut next_sequence = 0_u64;
        let code = unsafe {
            platform_emulator_account(
                emulator.core,
                index,
                id.as_mut_ptr(),
                &raw mut name,
                &raw mut name_length,
                &raw mut balance_hi,
                &raw mut balance_lo,
                &raw mut next_sequence,
            )
        };
        if code != 0 {
            return Err(code);
        }
        if name.is_null() || name_length == 0 {
            return Err(-3);
        }
        let name = unsafe { slice::from_raw_parts(name, name_length) };
        if name == expected_name.as_bytes() {
            return Ok(Some(id));
        }
    }
    Ok(None)
}

fn core_identity_sequence(emulator: &Emulator, did: &str) -> Result<u64, i32> {
    let mut next_sequence = 0_u64;
    let code = unsafe {
        platform_emulator_identity_sequence(
            emulator.core,
            did.as_ptr(),
            did.len(),
            &raw mut next_sequence,
        )
    };
    if code == 0 {
        Ok(next_sequence)
    } else {
        Err(code)
    }
}

fn decode_json<T: DeserializeOwned>(request: &Request) -> Result<T, String> {
    let media_type = request
        .content_type
        .split(';')
        .next()
        .unwrap_or_default()
        .trim();
    if media_type != "application/json" || request.body.len() > MAX_JSON_BYTES {
        return Err("request must carry bounded application/json".into());
    }
    serde_json::from_slice(&request.body).map_err(|_| "request body is not valid JSON".into())
}

fn escape_json(value: &str) -> String {
    let mut escaped = String::with_capacity(value.len());
    for character in value.chars() {
        match character {
            '"' => escaped.push_str("\\\""),
            '\\' => escaped.push_str("\\\\"),
            '\n' => escaped.push_str("\\n"),
            '\r' => escaped.push_str("\\r"),
            '\t' => escaped.push_str("\\t"),
            value if value.is_control() => {
                let _ = write!(escaped, "\\u{:04x}", u32::from(value));
            }
            value => escaped.push(value),
        }
    }
    escaped
}

fn core_error(code: i32) -> String {
    unsafe {
        let pointer = platform_emulator_error_name(code);
        if pointer.is_null() {
            return "LXP_ERR_UNKNOWN".into();
        }
        CStr::from_ptr(pointer).to_string_lossy().into_owned()
    }
}

fn success(trace: u64, result: &str) -> Response {
    Response {
        status: 200,
        content_type: "application/json",
        body: format!("{{\"ok\":true,\"result\":{result},\"trace\":\"emu-{trace:016x}\"}}")
            .into_bytes(),
    }
}

fn refusal(trace: u64, status: u16, code: &str, detail: &str) -> Response {
    Response {
        status,
        content_type: "application/json",
        body: format!("{{\"ok\":false,\"error\":{{\"code\":\"{}\",\"retry\":\"never\",\"detail\":\"{}\"}},\"trace\":\"emu-{trace:016x}\"}}", escape_json(code), escape_json(detail)).into_bytes(),
    }
}

fn core_response(trace: u64, code: i32) -> Response {
    let name = core_error(code);
    let status = if code == -904 { 503 } else { 400 };
    refusal(trace, status, &name, "the LayerX core refused the request")
}

fn canonical_hex32_text(value: &str) -> bool {
    value.len() == 64
        && value
            .bytes()
            .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte))
}

fn parse_request(stream: &mut TcpStream) -> Result<Request, String> {
    let mut bytes = Vec::with_capacity(4096);
    let mut chunk = [0_u8; 4096];
    let header_end = loop {
        let read = stream.read(&mut chunk).map_err(|error| error.to_string())?;
        if read == 0 {
            return Err("connection closed before request headers".into());
        }
        bytes.extend_from_slice(&chunk[..read]);
        if let Some(position) = bytes.windows(4).position(|window| window == b"\r\n\r\n") {
            if position.saturating_add(4) > MAX_HEADER_BYTES {
                return Err("request headers exceed emulator limit".into());
            }
            break position + 4;
        }
        if bytes.len() > MAX_HEADER_BYTES {
            return Err("request headers exceed emulator limit".into());
        }
    };
    let (method, path, content_type, content_length) = parse_request_headers(&bytes, header_end)?;
    while bytes.len() - header_end < content_length {
        let read = stream.read(&mut chunk).map_err(|error| error.to_string())?;
        if read == 0 {
            return Err("connection closed before request body".into());
        }
        bytes.extend_from_slice(&chunk[..read]);
        if bytes.len() > header_end.saturating_add(content_length) {
            return Err("request carries bytes beyond its declared body".into());
        }
    }
    if bytes.len() != header_end + content_length {
        return Err("request carries bytes beyond its declared body".into());
    }
    Ok(Request {
        method,
        path,
        content_type,
        body: bytes[header_end..header_end + content_length].to_vec(),
    })
}

fn write_response(stream: &mut TcpStream, response: &Response) -> std::io::Result<()> {
    let reason = match response.status {
        200 => "OK",
        202 => "Accepted",
        400 => "Bad Request",
        404 => "Not Found",
        405 => "Method Not Allowed",
        409 => "Conflict",
        410 => "Gone",
        413 => "Content Too Large",
        415 => "Unsupported Media Type",
        503 => "Service Unavailable",
        _ => "Error",
    };
    write!(stream, "HTTP/1.1 {} {}\r\nContent-Type: {}\r\nContent-Length: {}\r\nCache-Control: no-store\r\nConnection: close\r\n\r\n", response.status, reason, response.content_type, response.body.len())?;
    stream.write_all(&response.body)
}

fn inspect_state(emulator: &Emulator) -> Result<CoreState, i32> {
    let mut state = CoreState {
        canonical_state_root: [0; 32],
        receipt_state_root: [0; 32],
        next_sequence: 0,
        batch_number: 0,
        timestamp_ms: 0,
        cell_count: 0,
        account_count: 0,
    };
    let code = unsafe { platform_emulator_inspect(emulator.core, &raw mut state) };
    if code == 0 {
        Ok(state)
    } else {
        Err(code)
    }
}

fn prefund_core(
    emulator: &mut Emulator,
    did: &str,
    public_key: [u8; 32],
    amount_hi: u64,
    amount_lo: u64,
) -> Result<(), i32> {
    if emulator.accounts.len() >= MAX_EMULATOR_ACCOUNTS {
        return Err(-5);
    }
    let account_name = format!("agent:{did}:main");
    if emulator.accounts.contains_key(&account_name) {
        return Err(-3);
    }
    let code = unsafe {
        platform_emulator_prefund(
            emulator.core,
            did.as_ptr(),
            did.len(),
            public_key.as_ptr(),
            amount_hi,
            amount_lo,
        )
    };
    if code != 0 {
        return Err(code);
    }
    let account = core_account(emulator, &account_name)?.ok_or(-3)?;
    emulator.accounts.insert(
        account_name,
        EmulatorAccount {
            id: account,
            did: did.to_owned(),
            public_key,
        },
    );
    Ok(())
}

fn inspect(emulator: &Emulator, trace: u64) -> Response {
    let mut state = CoreState {
        canonical_state_root: [0; 32],
        receipt_state_root: [0; 32],
        next_sequence: 0,
        batch_number: 0,
        timestamp_ms: 0,
        cell_count: 0,
        account_count: 0,
    };
    let code = unsafe { platform_emulator_inspect(emulator.core, &raw mut state) };
    if code != 0 {
        return core_response(trace, code);
    }
    let mut cells = String::new();
    for index in 0..state.cell_count {
        let mut key = [0_u8; 32];
        let mut hi = 0_u64;
        let mut lo = 0_u64;
        let code = unsafe {
            platform_emulator_cell(
                emulator.core,
                index,
                key.as_mut_ptr(),
                &raw mut hi,
                &raw mut lo,
            )
        };
        if code != 0 {
            return core_response(trace, code);
        }
        if !cells.is_empty() {
            cells.push(',');
        }
        let _ = write!(
            cells,
            "{{\"key\":\"{}\",\"value_hi\":{hi},\"value_lo\":{lo}}}",
            hex_encode(&key)
        );
    }
    let mut accounts = String::new();
    for index in 0..state.account_count {
        let mut id = [0_u8; 32];
        let mut name = ptr::null();
        let mut name_length = 0_usize;
        let mut hi = 0_u64;
        let mut lo = 0_u64;
        let mut next_sequence = 0_u64;
        let code = unsafe {
            platform_emulator_account(
                emulator.core,
                index,
                id.as_mut_ptr(),
                &raw mut name,
                &raw mut name_length,
                &raw mut hi,
                &raw mut lo,
                &raw mut next_sequence,
            )
        };
        if code != 0 {
            return core_response(trace, code);
        }
        if name.is_null() || name_length == 0 {
            return refusal(
                trace,
                503,
                "core_invalid_output",
                "the LayerX core returned an invalid account name buffer",
            );
        }
        let account_name = unsafe { slice::from_raw_parts(name, name_length) };
        let account_name = String::from_utf8_lossy(account_name);
        if !accounts.is_empty() {
            accounts.push(',');
        }
        let _ = write!(
            accounts,
            "{{\"id\":\"{}\",\"name\":\"{}\",\"balance_hi\":{hi},\"balance_lo\":{lo},\"next_sequence\":{next_sequence}}}",
            hex_encode(&id),
            escape_json(&account_name)
        );
    }
    success(trace, &format!("{{\"network_mode\":\"emulator\",\"batch_cadence\":\"instant\",\"state_root\":\"{}\",\"canonical_state_root\":\"{}\",\"receipt_state_root\":\"{}\",\"next_sequence\":{},\"batch_number\":{},\"timestamp_ms\":{},\"cells\":[{cells}],\"accounts\":[{accounts}]}}", hex_encode(&state.canonical_state_root), hex_encode(&state.canonical_state_root), hex_encode(&state.receipt_state_root), state.next_sequence, state.batch_number, state.timestamp_ms))
}

fn identity_sequence_read(emulator: &Emulator, path: &str, trace: u64) -> Response {
    let Some(did) = path
        .strip_prefix("/v1/dids/")
        .and_then(|tail| tail.strip_suffix("/sequence"))
    else {
        return refusal(trace, 404, "not_found", "route does not exist");
    };
    if did.contains('/') || Did::new(did.as_bytes()).is_err() {
        return refusal(trace, 400, "invalid_did", "did is not a canonical DID");
    }
    let state = match inspect_state(emulator) {
        Ok(state) => state,
        Err(code) => return core_response(trace, code),
    };
    let next_sequence = match core_identity_sequence(emulator, did) {
        Ok(value) => value,
        Err(code) => return core_response(trace, code),
    };
    success(
        trace,
        &format!(
            "{{\"did\":\"{}\",\"next_sequence\":\"{next_sequence}\",\"observed_head_sequence\":\"{}\",\"state_root\":\"{}\",\"verification\":\"emulator_core_snapshot\"}}",
            escape_json(did),
            state.next_sequence,
            hex_encode(&state.canonical_state_root)
        ),
    )
}

fn sequencer_identity(emulator: &Emulator, trace: u64) -> Response {
    success(
        trace,
        &sequencer_identity_body(
            emulator.network_id,
            &emulator.signing_key.verifying_key().to_bytes(),
        ),
    )
}

fn sequencer_identity_body(network_id: u32, sequencer_public_key: &[u8; 32]) -> String {
    serde_json::json!({
        "network_id": network_id,
        "sequencer_public_key": hex_encode(sequencer_public_key),
        "sequencer_id": hex_encode(&Sha256::digest(sequencer_public_key)),
        "first_batch": 1,
        "last_batch": u64::MAX,
    })
    .to_string()
}

fn current_head(trace: u64) -> Response {
    refusal(
        trace,
        503,
        "principal_state_proof_unavailable",
        "legacy batch head evidence is retired; use merged-node block-commit evidence",
    )
}

fn health(emulator: &Emulator, trace: u64) -> Response {
    let mut state = CoreState {
        canonical_state_root: [0; 32],
        receipt_state_root: [0; 32],
        next_sequence: 0,
        batch_number: 0,
        timestamp_ms: 0,
        cell_count: 0,
        account_count: 0,
    };
    let code = unsafe { platform_emulator_inspect(emulator.core, &raw mut state) };
    if code != 0 {
        return refusal(
            trace,
            503,
            "core_unavailable",
            "the LayerX core did not return a readable state",
        );
    }
    success(trace, "{\"status\":\"ready\",\"core\":\"layerx\",\"surface\":\"/v1/surface\",\"payments_supported\":false}")
}

fn prefund(emulator: &mut Emulator, request: &Request, trace: u64) -> Response {
    let body = match decode_json::<PrefundBody>(request) {
        Ok(body) => body,
        Err(error) => return refusal(trace, 400, "invalid_argument", &error),
    };
    if body.did.len() > 512 || !body.did.starts_with("did:") || body.did.contains('\0') {
        return refusal(trace, 400, "invalid_argument", "did is not a bounded DID");
    }
    let Ok(key) = hex_decode(&body.public_key) else {
        return refusal(trace, 400, "invalid_argument", "public_key must be hex");
    };
    if key.len() != 32 {
        return refusal(
            trace,
            400,
            "invalid_argument",
            "public_key must be 32 bytes",
        );
    }
    let public_key: [u8; 32] = match key.try_into() {
        Ok(value) => value,
        Err(_) => {
            return refusal(
                trace,
                400,
                "invalid_argument",
                "public_key must be 32 bytes",
            )
        }
    };
    match prefund_core(
        emulator,
        &body.did,
        public_key,
        body.amount_hi,
        body.amount_lo,
    ) {
        Ok(()) => success(trace, "{\"prefunded\":true}"),
        Err(code) => core_response(trace, code),
    }
}

fn update_time(emulator: &mut Emulator, request: &Request, trace: u64, advance: bool) -> Response {
    let value = if advance {
        decode_json::<AdvanceBody>(request).map(|body| body.delta_ms)
    } else {
        decode_json::<TimestampBody>(request).map(|body| body.timestamp_ms)
    };
    let value = match value {
        Ok(value) => value,
        Err(error) => return refusal(trace, 400, "invalid_argument", &error),
    };
    let code = unsafe {
        if advance {
            platform_emulator_advance_time(emulator.core, value)
        } else {
            platform_emulator_set_time(emulator.core, value)
        }
    };
    if code == 0 {
        success(trace, "{\"updated\":true}")
    } else {
        core_response(trace, code)
    }
}

fn inject_fault(emulator: &mut Emulator, request: &Request, trace: u64) -> Response {
    let body = match decode_json::<FaultBody>(request) {
        Ok(body) => body,
        Err(error) => return refusal(trace, 400, "invalid_argument", &error),
    };
    let kind = match body.kind.as_str() {
        "reject" => 1,
        "drop_receipt" => 2,
        "corrupt_receipt" => 3,
        _ => return refusal(trace, 400, "invalid_argument", "unknown fault kind"),
    };
    if body.count == 0 || body.count > 1_000_000 {
        return refusal(
            trace,
            400,
            "invalid_argument",
            "fault count is outside its bound",
        );
    }
    let code = unsafe { platform_emulator_inject_failure(emulator.core, kind, body.count) };
    if code == 0 {
        success(trace, "{\"configured\":true}")
    } else {
        core_response(trace, code)
    }
}

struct RecoverySnapshot {
    core_snapshot: Vec<u8>,
    receipts: HashMap<String, String>,
    receipt_order: VecDeque<String>,
    program_operations: HashMap<String, ProgramOperation>,
    program_activity_operations: HashMap<String, String>,
    accounts: HashMap<String, EmulatorAccount>,
    move_quotes: HashMap<String, MoveQuoteRecord>,
    move_operations: HashMap<String, MoveOperation>,
}

fn append_snapshot_bytes(encoded: &mut Vec<u8>, bytes: &[u8]) -> Result<(), String> {
    let next = encoded
        .len()
        .checked_add(bytes.len())
        .and_then(|length| length.checked_add(32))
        .ok_or_else(|| "emulator recovery snapshot length overflowed".to_owned())?;
    if next > MAX_REQUEST_BYTES {
        return Err("emulator recovery snapshot exceeds its bound".to_owned());
    }
    encoded.extend_from_slice(bytes);
    Ok(())
}

fn append_snapshot_u32(encoded: &mut Vec<u8>, value: usize) -> Result<(), String> {
    let value = u32::try_from(value)
        .map_err(|_| "emulator recovery snapshot field exceeds u32".to_owned())?;
    append_snapshot_bytes(encoded, &value.to_be_bytes())
}

fn append_snapshot_u16(encoded: &mut Vec<u8>, value: usize) -> Result<(), String> {
    let value = u16::try_from(value)
        .map_err(|_| "emulator recovery snapshot field exceeds u16".to_owned())?;
    append_snapshot_bytes(encoded, &value.to_be_bytes())
}

fn append_snapshot_text(encoded: &mut Vec<u8>, value: &str) -> Result<(), String> {
    append_snapshot_u16(encoded, value.len())?;
    append_snapshot_bytes(encoded, value.as_bytes())
}

fn canonical_program_response(response: &str, activity_id: &str, idempotency_key: &str) -> bool {
    if response.is_empty() || response.len() > MAX_PROGRAM_RESPONSE_BYTES {
        return false;
    }
    let Ok(document) = serde_json::from_str::<serde_json::Value>(response) else {
        return false;
    };
    let canonical = document.to_string();
    document.is_object()
        && canonical == response
        && document
            .get("activity_id")
            .and_then(serde_json::Value::as_str)
            == Some(activity_id)
        && document
            .get("idempotency_key")
            .and_then(serde_json::Value::as_str)
            == Some(idempotency_key)
}

fn encode_recovery_snapshot(input: RecoverySnapshotInput<'_>) -> Result<Vec<u8>, String> {
    let RecoverySnapshotInput {
        core_snapshot,
        receipts,
        receipt_order,
        program_operations,
        program_activity_operations,
        accounts,
        move_quotes,
        move_operations,
    } = input;
    if core_snapshot.is_empty() || core_snapshot.len() > MAX_CORE_SNAPSHOT_BYTES {
        return Err("emulator core snapshot is outside its bound".to_owned());
    }
    if receipts.len() > MAX_RECEIPTS || receipt_order.len() != receipts.len() {
        return Err("emulator receipt recovery index is outside its bound".to_owned());
    }
    if program_operations.len() > MAX_PROGRAM_OPERATIONS
        || program_activity_operations.len() != program_operations.len()
    {
        return Err("emulator program recovery index is outside its bound".to_owned());
    }
    if accounts.len() > MAX_EMULATOR_ACCOUNTS
        || move_quotes.len() > MAX_MOVE_QUOTES
        || move_operations.len() > MAX_MOVE_OPERATIONS
    {
        return Err("emulator move recovery index is outside its bound".to_owned());
    }

    let mut encoded = Vec::new();
    append_snapshot_bytes(&mut encoded, RECOVERY_SNAPSHOT_MAGIC)?;
    append_snapshot_bytes(&mut encoded, &RECOVERY_SNAPSHOT_VERSION.to_be_bytes())?;
    append_snapshot_u32(&mut encoded, core_snapshot.len())?;
    append_snapshot_u32(&mut encoded, receipts.len())?;
    append_snapshot_u32(&mut encoded, program_operations.len())?;
    append_snapshot_u32(&mut encoded, accounts.len())?;
    append_snapshot_u32(&mut encoded, move_quotes.len())?;
    append_snapshot_u32(&mut encoded, move_operations.len())?;
    append_snapshot_bytes(&mut encoded, core_snapshot)?;

    encode_snapshot_receipts(&mut encoded, receipts, receipt_order)?;
    encode_snapshot_programs(
        &mut encoded,
        program_operations,
        program_activity_operations,
    )?;
    encode_snapshot_accounts(&mut encoded, accounts)?;
    encode_snapshot_quotes(&mut encoded, move_quotes, accounts)?;
    encode_snapshot_moves(&mut encoded, move_operations, move_quotes)?;
    let mut digest = Sha256::new();
    digest.update(RECOVERY_SNAPSHOT_DIGEST_DOMAIN);
    digest.update(&encoded);
    let digest: [u8; 32] = digest.finalize().into();
    append_snapshot_bytes(&mut encoded, &digest)?;
    Ok(encoded)
}

fn snapshot_take<'a>(
    snapshot: &'a [u8],
    offset: &mut usize,
    length: usize,
) -> Result<&'a [u8], String> {
    let end = offset
        .checked_add(length)
        .ok_or_else(|| "emulator recovery snapshot offset overflowed".to_owned())?;
    let value = snapshot
        .get(*offset..end)
        .ok_or_else(|| "emulator recovery snapshot is truncated".to_owned())?;
    *offset = end;
    Ok(value)
}

fn snapshot_u32(snapshot: &[u8], offset: &mut usize) -> Result<usize, String> {
    let bytes: [u8; 4] = snapshot_take(snapshot, offset, 4)?
        .try_into()
        .map_err(|_| "emulator recovery snapshot integer is invalid".to_owned())?;
    usize::try_from(u32::from_be_bytes(bytes))
        .map_err(|_| "emulator recovery snapshot integer exceeds usize".to_owned())
}

fn snapshot_u16(snapshot: &[u8], offset: &mut usize) -> Result<usize, String> {
    let bytes: [u8; 2] = snapshot_take(snapshot, offset, 2)?
        .try_into()
        .map_err(|_| "emulator recovery snapshot integer is invalid".to_owned())?;
    Ok(usize::from(u16::from_be_bytes(bytes)))
}

fn snapshot_u64(snapshot: &[u8], offset: &mut usize) -> Result<u64, String> {
    let bytes: [u8; 8] = snapshot_take(snapshot, offset, 8)?
        .try_into()
        .map_err(|_| "emulator recovery snapshot integer is invalid".to_owned())?;
    Ok(u64::from_be_bytes(bytes))
}

fn snapshot_u128(snapshot: &[u8], offset: &mut usize) -> Result<u128, String> {
    let bytes: [u8; 16] = snapshot_take(snapshot, offset, 16)?
        .try_into()
        .map_err(|_| "emulator recovery snapshot integer is invalid".to_owned())?;
    Ok(u128::from_be_bytes(bytes))
}

fn snapshot_text(snapshot: &[u8], offset: &mut usize, maximum: usize) -> Result<String, String> {
    let length = snapshot_u16(snapshot, offset)?;
    if length == 0 || length > maximum {
        return Err("emulator recovery snapshot text exceeds its bound".to_owned());
    }
    std::str::from_utf8(snapshot_take(snapshot, offset, length)?)
        .map(str::to_owned)
        .map_err(|_| "emulator recovery snapshot text is not UTF-8".to_owned())
}

fn decode_recovery_snapshot(body: &[u8]) -> Result<RecoverySnapshot, String> {
    if body.len() > MAX_REQUEST_BYTES || body.len() < 8 + 4 + 7 * 4 + 32 {
        return Err("emulator recovery snapshot is outside its bound".to_owned());
    }
    let authenticated_length = body.len() - 32;
    let (authenticated, supplied_digest) = body.split_at(authenticated_length);
    let mut digest = Sha256::new();
    digest.update(RECOVERY_SNAPSHOT_DIGEST_DOMAIN);
    digest.update(authenticated);
    let expected_digest: [u8; 32] = digest.finalize().into();
    if expected_digest.as_slice() != supplied_digest {
        return Err("emulator recovery snapshot digest is invalid".to_owned());
    }

    let mut offset = 0;
    if snapshot_take(authenticated, &mut offset, 8)? != RECOVERY_SNAPSHOT_MAGIC {
        return Err("emulator recovery snapshot magic is invalid".to_owned());
    }
    let version = snapshot_u32(authenticated, &mut offset)?;
    if version != RECOVERY_SNAPSHOT_VERSION as usize {
        return Err("emulator recovery snapshot version is unsupported".to_owned());
    }
    let core_length = snapshot_u32(authenticated, &mut offset)?;
    let receipt_count = snapshot_u32(authenticated, &mut offset)?;
    let operation_count = snapshot_u32(authenticated, &mut offset)?;
    let account_count = snapshot_u32(authenticated, &mut offset)?;
    let move_quote_count = snapshot_u32(authenticated, &mut offset)?;
    let move_operation_count = snapshot_u32(authenticated, &mut offset)?;
    if core_length == 0
        || core_length > MAX_CORE_SNAPSHOT_BYTES
        || receipt_count > MAX_RECEIPTS
        || operation_count > MAX_PROGRAM_OPERATIONS
        || account_count > MAX_EMULATOR_ACCOUNTS
        || move_quote_count > MAX_MOVE_QUOTES
        || move_operation_count > MAX_MOVE_OPERATIONS
    {
        return Err("emulator recovery snapshot counts exceed their bounds".to_owned());
    }
    let core_snapshot = snapshot_take(authenticated, &mut offset, core_length)?.to_vec();

    let (receipts, receipt_order) =
        decode_snapshot_receipts(authenticated, &mut offset, receipt_count)?;
    let (program_operations, program_activity_operations) =
        decode_snapshot_programs(authenticated, &mut offset, operation_count)?;
    let accounts = decode_snapshot_accounts(authenticated, &mut offset, account_count)?;
    let move_quotes =
        decode_snapshot_quotes(authenticated, &mut offset, move_quote_count, &accounts)?;
    let move_operations = decode_snapshot_moves(
        authenticated,
        &mut offset,
        move_operation_count,
        &move_quotes,
    )?;
    if move_quotes.values().any(|quote| {
        quote
            .committed_idempotency
            .as_ref()
            .is_some_and(|key| !move_operations.contains_key(key))
    }) {
        return Err("emulator recovery committed quote is missing its operation".to_owned());
    }
    if offset != authenticated.len() {
        return Err("emulator recovery snapshot has trailing bytes".to_owned());
    }
    Ok(RecoverySnapshot {
        core_snapshot,
        receipts,
        receipt_order,
        program_operations,
        program_activity_operations,
        accounts,
        move_quotes,
        move_operations,
    })
}

fn export_snapshot(emulator: &mut Emulator, trace: u64) -> Response {
    let mut bytes = ptr::null();
    let mut length = 0_usize;
    let code = unsafe {
        platform_emulator_snapshot_export(emulator.core, &raw mut bytes, &raw mut length)
    };
    if code != 0 {
        return core_response(trace, code);
    }
    if bytes.is_null() || length == 0 || length > MAX_CORE_SNAPSHOT_BYTES {
        return refusal(
            trace,
            503,
            "core_invalid_output",
            "the LayerX core returned an invalid snapshot buffer",
        );
    }
    let core_snapshot = unsafe { slice::from_raw_parts(bytes, length) };
    match encode_recovery_snapshot(RecoverySnapshotInput {
        core_snapshot,
        receipts: &emulator.receipts,
        receipt_order: &emulator.receipt_order,
        program_operations: &emulator.program_operations,
        program_activity_operations: &emulator.program_activity_operations,
        accounts: &emulator.accounts,
        move_quotes: &emulator.move_quotes,
        move_operations: &emulator.move_operations,
    }) {
        Ok(body) => Response {
            status: 200,
            content_type: "application/vnd.layerx.emulator-snapshot",
            body,
        },
        Err(error) => refusal(trace, 503, "snapshot_encoding_failed", &error),
    }
}

fn import_snapshot(emulator: &mut Emulator, body: &[u8], trace: u64) -> Response {
    let recovered = match decode_recovery_snapshot(body) {
        Ok(value) => value,
        Err(error) => return refusal(trace, 400, "invalid_snapshot", &error),
    };
    let mut prior_bytes = ptr::null();
    let mut prior_length = 0_usize;
    let export_code = unsafe {
        platform_emulator_snapshot_export(
            emulator.core,
            &raw mut prior_bytes,
            &raw mut prior_length,
        )
    };
    if export_code != 0
        || prior_bytes.is_null()
        || prior_length == 0
        || prior_length > MAX_CORE_SNAPSHOT_BYTES
    {
        return refusal(
            trace,
            503,
            "snapshot_rollback_unavailable",
            "current canonical state could not be retained before import",
        );
    }
    let prior_core = unsafe { slice::from_raw_parts(prior_bytes, prior_length) }.to_vec();
    let code = unsafe {
        platform_emulator_snapshot_import(
            emulator.core,
            recovered.core_snapshot.as_ptr(),
            recovered.core_snapshot.len(),
        )
    };
    if code != 0 {
        let rollback = unsafe {
            platform_emulator_snapshot_import(emulator.core, prior_core.as_ptr(), prior_core.len())
        };
        if rollback != 0 {
            return refusal(
                trace,
                503,
                "snapshot_rollback_failed",
                "prior canonical state could not be restored",
            );
        }
        return core_response(trace, code);
    }
    let mut owner_account_count = 0_usize;
    let owner_account_status = unsafe {
        platform_emulator_owner_account_count(emulator.core, &raw mut owner_account_count)
    };
    let recovered_accounts_match = owner_account_status == 0
        && owner_account_count == recovered.accounts.len()
        && recovered.accounts.iter().all(|(name, expected)| {
            core_account(emulator, name)
                .ok()
                .flatten()
                .is_some_and(|account| account == expected.id)
        });
    if !recovered_accounts_match {
        let rollback = unsafe {
            platform_emulator_snapshot_import(emulator.core, prior_core.as_ptr(), prior_core.len())
        };
        if rollback != 0 {
            return refusal(
                trace,
                503,
                "snapshot_rollback_failed",
                "invalid move metadata was refused but prior canonical state could not be restored",
            );
        }
        return refusal(
            trace,
            400,
            "invalid_snapshot",
            "recovered account metadata does not match canonical state",
        );
    }
    emulator.receipts = recovered.receipts;
    emulator.receipt_order = recovered.receipt_order;
    emulator.program_operations = recovered.program_operations;
    emulator.program_activity_operations = recovered.program_activity_operations;
    emulator.accounts = recovered.accounts;
    emulator.move_quotes = recovered.move_quotes;
    emulator.move_operations = recovered.move_operations;
    success(trace, "{\"imported\":true}")
}

fn advance_trace(trace: &mut u64) -> Option<u64> {
    let next = trace.checked_add(1)?;
    *trace = next;
    Some(next)
}

fn receipt_read(trace: u64) -> Response {
    refusal(
        trace,
        410,
        "legacy_receipt_verification_retired",
        "historical receipts remain in snapshots; use pax_sequencerReceipt with pinned node keys for merged transaction evidence",
    )
}

fn evidence_read(evidence_id: &str, trace: u64) -> Response {
    if evidence_id
        .strip_prefix("evd_")
        .filter(|hex| hex.len() == 64)
        .and_then(|hex| hex_decode(hex).ok())
        .is_none()
    {
        return refusal(
            trace,
            400,
            "invalid_evidence_id",
            "evidence id must be evd_ followed by a 64-character hex digest",
        );
    }
    refusal(
        trace,
        410,
        "legacy_receipt_verification_retired",
        "legacy receipt digest lookup is retired; use pax_sequencerReceipt with pinned node keys for merged transaction evidence",
    )
}

fn journey_read(emulator: &Emulator, journey_id: &str, trace: u64) -> Response {
    let Some(digest) = journey_id
        .strip_prefix("jrn_")
        .filter(|hex| hex.len() == 64)
        .and_then(|hex| hex_decode(hex).ok())
    else {
        return refusal(
            trace,
            400,
            "invalid_journey_id",
            "journey id must be jrn_ followed by a 64-character hex digest",
        );
    };
    for (idempotency, operation) in &emulator.move_operations {
        if operation.status != 200
            || hash_bytes(MOVE_JOURNEY_DOMAIN, idempotency.as_bytes())[..] != digest[..]
        {
            continue;
        }
        let Some(journey) = serde_json::from_str::<serde_json::Value>(&operation.body)
            .ok()
            .and_then(|body| body.get("result").cloned())
        else {
            continue;
        };
        return success(trace, &journey.to_string());
    }
    refusal(
        trace,
        404,
        "not_found",
        "journey is not present in this emulator process",
    )
}

fn route(emulator: &mut Emulator, request: &Request) -> Response {
    let Some(trace) = advance_trace(&mut emulator.trace) else {
        let result = refusal(
            u64::MAX,
            503,
            "trace_exhausted",
            "the emulator trace space is exhausted",
        );
        return result;
    };
    match (request.method.as_str(), request.path.as_str()) {
        ("GET", "/healthz") => health(emulator, trace),
        ("GET", "/v1/surface") => success(trace, &surface::document()),
        ("GET", "/v1/sequencer") => sequencer_identity(emulator, trace),
        ("GET", "/v1/protocol/account-state/head") => current_head(trace),
        (_, "/v1/settle") => refusal(
            trace,
            410,
            settlement::RETIRED_CODE,
            settlement::RETIRED_DETAIL,
        ),
        ("GET", "/v1/state") => inspect(emulator, trace),
        ("GET", path) if path.starts_with("/v1/dids/") && path.ends_with("/sequence") => {
            identity_sequence_read(emulator, path, trace)
        }
        ("POST", "/__emulator/accounts/prefund") => prefund(emulator, request, trace),
        ("POST", "/__emulator/time/set") => update_time(emulator, request, trace, false),
        ("POST", "/__emulator/time/advance") => update_time(emulator, request, trace, true),
        ("POST", "/__emulator/faults") => inject_fault(emulator, request, trace),
        ("GET", "/__emulator/snapshot") => export_snapshot(emulator, trace),
        ("PUT", "/__emulator/snapshot") => import_snapshot(emulator, &request.body, trace),
        ("GET", path) if path.starts_with("/v1/evidence/") => evidence_read(&path[13..], trace),
        ("GET", path) if path.starts_with("/v1/journeys/") => {
            journey_read(emulator, &path[13..], trace)
        }
        ("GET", path) if path.starts_with("/v1/receipts/") => {
            let id = &path[13..];
            match emulator.receipts.get(id) {
                Some(_) => receipt_read(trace),
                None => refusal(
                    trace,
                    404,
                    "not_found",
                    "receipt is not present in this emulator process",
                ),
            }
        }
        (
            _,
            "/v1/surface"
            | "/v1/sequencer"
            | "/v1/protocol/account-state/head"
            | "/v1/state"
            | "/__emulator/accounts/prefund"
            | "/__emulator/time/set"
            | "/__emulator/time/advance"
            | "/__emulator/faults"
            | "/__emulator/snapshot",
        ) => refusal(
            trace,
            405,
            "method_not_allowed",
            "method is not supported for this route",
        ),
        _ => refusal(trace, 404, "not_found", "route does not exist"),
    }
}

fn parse_amount(value: &str) -> Result<(u64, u64), String> {
    if let Some((hi, lo)) = value.split_once(':') {
        Ok((
            hi.parse().map_err(|_| "invalid amount high word")?,
            lo.parse().map_err(|_| "invalid amount low word")?,
        ))
    } else {
        Ok((0, value.parse().map_err(|_| "invalid amount")?))
    }
}

fn parse_prefund(value: &str) -> Result<Prefund, String> {
    let mut fields = value.split(',');
    let did = fields.next().ok_or("prefund requires did")?.to_string();
    let public = hex_decode(fields.next().ok_or("prefund requires public key")?)?;
    let (amount_hi, amount_lo) = parse_amount(fields.next().ok_or("prefund requires amount")?)?;
    if fields.next().is_some() || public.len() != 32 || did.is_empty() {
        return Err("prefund format is did,64-hex-public-key,amount-hi:amount-lo".into());
    }
    let mut public_key = [0_u8; 32];
    public_key.copy_from_slice(&public);
    Ok(Prefund {
        did,
        public_key,
        amount_hi,
        amount_lo,
    })
}

fn parse_config(arguments: impl IntoIterator<Item = String>) -> Result<Config, String> {
    let mut arguments = arguments.into_iter();
    if arguments.next().as_deref() != Some("up") {
        return Err("usage: layerx emulator up [--listen ADDRESS] [--network-id ID] [--protocol-version 2|3] [--time-ms MS] [--sequencer-seed-file PATH] [--prefund DID,PUBLIC_KEY,AMOUNT]".into());
    }
    let mut config = Config {
        listen: SocketAddr::from(([127, 0, 0, 1], DEFAULT_PORT)),
        network_id: DEFAULT_NETWORK_ID,
        protocol_version: 2,
        timestamp_ms: DEFAULT_TIME_MS,
        prefunds: Vec::new(),
        sequencer_seed: None,
    };
    while let Some(argument) = arguments.next() {
        let value = arguments
            .next()
            .ok_or_else(|| format!("missing value for {argument}"))?;
        match argument.as_str() {
            "--listen" => {
                let listen = value
                    .parse::<SocketAddr>()
                    .map_err(|_| "listen address must be an IP socket address")?;
                if !listen.ip().is_loopback() {
                    return Err("emulator control routes may listen only on loopback".into());
                }
                config.listen = listen;
            }
            "--network-id" => {
                config.network_id = value.parse().map_err(|_| "network id must be an integer")?;
            }
            "--protocol-version" => {
                config.protocol_version = match value.as_str() {
                    "2" => 2,
                    "3" => 3,
                    _ => return Err("protocol version must be 2 or 3".into()),
                };
            }
            "--time-ms" => {
                config.timestamp_ms = value.parse().map_err(|_| "time must be an integer")?;
            }
            "--sequencer-seed-file" => {
                let bytes = std::fs::read(&value).map_err(|error| {
                    format!("could not read sequencer seed file {value}: {error}")
                })?;
                let seed = if bytes.len() == 32 {
                    bytes
                } else {
                    let text = std::str::from_utf8(&bytes).map_err(|_| {
                        "sequencer seed file must contain 32 raw bytes or 64 hexadecimal characters"
                    })?;
                    hex_decode(text.trim())?
                };
                config.sequencer_seed = Some(
                    seed.try_into()
                        .map_err(|_| "sequencer seed must be exactly 32 bytes")?,
                );
            }
            "--prefund" => config.prefunds.push(parse_prefund(&value)?),
            _ => return Err(format!("unknown argument: {argument}")),
        }
    }
    Ok(config)
}

/// Starts the local gateway adapter around the production `LayerX` transition.
fn platform_emulator(config: &Config, shutdown: Arc<AtomicBool>) -> Result<(), String> {
    layerx_programs_runtime::retain_host_ffi_exports();
    layerx_programs_sandbox::retain_host_ffi_exports();
    let seed = config.sequencer_seed.ok_or_else(|| {
        "--sequencer-seed-file is required; the emulator has no compiled-in signing authority"
            .to_owned()
    })?;
    if seed == [0; 32] {
        return Err("sequencer seed must not be zero".to_owned());
    }
    let core = unsafe {
        platform_emulator_create_for_protocol(
            config.network_id,
            config.timestamp_ms,
            seed.as_ptr(),
            config.protocol_version,
        )
    };
    if core.is_null() {
        return Err("could not initialize the LayerX core".into());
    }
    let mut emulator = Emulator {
        core,
        signing_key: SigningKey::from_bytes(&seed),
        network_id: config.network_id,
        receipts: HashMap::new(),
        receipt_order: VecDeque::new(),
        program_operations: HashMap::new(),
        program_activity_operations: HashMap::new(),
        accounts: HashMap::new(),
        move_quotes: HashMap::new(),
        move_operations: HashMap::new(),
        trace: 0,
    };
    for prefund in &config.prefunds {
        if let Err(status) = prefund_core(
            &mut emulator,
            &prefund.did,
            prefund.public_key,
            prefund.amount_hi,
            prefund.amount_lo,
        ) {
            return Err(format!(
                "prefund {} failed: {}",
                prefund.did,
                core_error(status)
            ));
        }
    }
    serve_emulator(config, emulator, shutdown)
}

/// Runs the emulator subcommand behind the unified `layerx` CLI dispatcher.
///
/// # Errors
///
/// Returns configuration, core-initialisation, or listener failures without
/// terminating the owning CLI process.
pub fn run(arguments: impl IntoIterator<Item = String>) -> Result<(), String> {
    run_until(arguments, Arc::new(AtomicBool::new(false)))
}

/// Runs the same gateway as [`run`] until the owner requests shutdown.
/// Accepted requests drain before the listener, workers and core are released.
///
/// # Errors
/// Returns configuration, core-initialisation, listener or worker failures.
pub fn run_until(
    arguments: impl IntoIterator<Item = String>,
    shutdown: Arc<AtomicBool>,
) -> Result<(), String> {
    parse_config(arguments).and_then(|config| platform_emulator(&config, shutdown))
}

#[cfg(test)]
mod boundary_tests {
    use super::{advance_trace, hex_encode, parse_config, sequencer_identity_body};
    use sha2::Digest as _;

    #[test]
    fn sequencer_identity_advertises_the_public_key_and_never_the_seed(
    ) -> Result<(), Box<dyn std::error::Error>> {
        let seed = [0x42_u8; 32];
        let public_key = ed25519_dalek::SigningKey::from_bytes(&seed)
            .verifying_key()
            .to_bytes();
        let body = sequencer_identity_body(402, &public_key);
        let document: serde_json::Value = serde_json::from_str(&body)?;
        assert_eq!(document["network_id"], 402);
        assert_eq!(document["sequencer_public_key"], hex_encode(&public_key));
        assert_eq!(
            document["sequencer_id"],
            hex_encode(&sha2::Sha256::digest(public_key))
        );
        assert_eq!(document["first_batch"], 1);
        assert_eq!(document["last_batch"], u64::MAX);
        assert!(!body.contains(&hex_encode(&seed)));
        Ok(())
    }

    #[test]
    fn control_surface_refuses_non_loopback_listeners() {
        assert!(parse_config([
            "up".to_owned(),
            "--listen".to_owned(),
            "0.0.0.0:9402".to_owned()
        ])
        .is_err());
        assert!(parse_config([
            "up".to_owned(),
            "--listen".to_owned(),
            "192.0.2.1:9402".to_owned()
        ])
        .is_err());
    }

    #[test]
    fn control_surface_accepts_ipv4_and_ipv6_loopback() {
        assert!(parse_config([
            "up".to_owned(),
            "--listen".to_owned(),
            "127.0.0.1:0".to_owned()
        ])
        .is_ok());
        assert!(
            parse_config(["up".to_owned(), "--listen".to_owned(), "[::1]:0".to_owned()]).is_ok()
        );
    }

    #[test]
    fn trace_exhaustion_never_wraps() {
        let mut trace = u64::MAX - 1;
        assert_eq!(advance_trace(&mut trace), Some(u64::MAX));
        assert_eq!(advance_trace(&mut trace), None);
        assert_eq!(trace, u64::MAX);
    }

    #[test]
    fn emulator_protocol_profiles_are_explicit_and_closed() -> Result<(), String> {
        assert_eq!(parse_config(["up".to_owned()])?.protocol_version, 2);
        for protocol in ["2", "3"] {
            let config = parse_config([
                "up".to_owned(),
                "--protocol-version".to_owned(),
                protocol.to_owned(),
            ])?;
            assert_eq!(config.protocol_version.to_string(), protocol);
        }
        for invalid in ["0", "1", "4", "03", "-1"] {
            assert!(parse_config([
                "up".to_owned(),
                "--protocol-version".to_owned(),
                invalid.to_owned()
            ])
            .is_err());
        }
        Ok(())
    }
}

#[cfg(test)]
mod recovery_snapshot_tests {
    use super::*;

    #[test]
    fn recovery_snapshot_is_deterministic_and_preserves_program_indexes(
    ) -> Result<(), Box<dyn std::error::Error>> {
        let receipt_activity = "a".repeat(64);
        let program_activity = "b".repeat(64);
        let idempotency = "c".repeat(64);
        let receipts = HashMap::from([
            (receipt_activity.clone(), "deadbeef".to_owned()),
            (program_activity.clone(), "01020304".to_owned()),
        ]);
        let receipt_order = [receipt_activity, program_activity.clone()]
            .into_iter()
            .collect::<VecDeque<_>>();
        let response = serde_json::json!({
            "state":"executed",
            "activity_id":program_activity.as_str(),
            "idempotency_key":idempotency.as_str(),
        })
        .to_string();
        let program_operations = HashMap::from([(
            idempotency.clone(),
            ProgramOperation {
                activity_id: program_activity.clone(),
                response: response.clone(),
                retained_signed_activity: Some("00ff".to_owned()),
            },
        )]);
        let program_activity_operations =
            HashMap::from([(program_activity.clone(), idempotency.clone())]);
        let source_name = "agent:did:layerx:source:main".to_owned();
        let destination_name = "agent:did:layerx:destination:main".to_owned();
        let accounts = HashMap::from([
            (
                source_name.clone(),
                EmulatorAccount {
                    id: [1; 32],
                    did: "did:layerx:source".to_owned(),
                    public_key: [2; 32],
                },
            ),
            (
                destination_name.clone(),
                EmulatorAccount {
                    id: [3; 32],
                    did: "did:layerx:destination".to_owned(),
                    public_key: [4; 32],
                },
            ),
        ]);
        let move_idempotency = "move-operation-key-0001".to_owned();
        let quote_id = format!("qte_{}", "d".repeat(64));
        let move_quotes = HashMap::from([(
            quote_id.clone(),
            MoveQuoteRecord {
                quote_id: quote_id.clone(),
                source: source_name,
                destination: destination_name,
                source_id: [1; 32],
                destination_id: [3; 32],
                amount: 9,
                currency: "LXP".to_owned(),
                created_at: 1_700_000_000_000,
                expires_at: 1_700_000_300_000,
                identity_sequence: 7,
                source_sequence: 8,
                committed_idempotency: Some(move_idempotency.clone()),
            },
        )]);
        let move_operations = HashMap::from([(
            move_idempotency,
            MoveOperation {
                quote_id,
                status: 200,
                body: serde_json::json!({"ok":true,"result":{"state":"done"}}).to_string(),
            },
        )]);

        let input = super::RecoverySnapshotInput {
            core_snapshot: b"bounded-core-snapshot",
            receipts: &receipts,
            receipt_order: &receipt_order,
            program_operations: &program_operations,
            program_activity_operations: &program_activity_operations,
            accounts: &accounts,
            move_quotes: &move_quotes,
            move_operations: &move_operations,
        };
        let first = encode_recovery_snapshot(input)?;
        let second = encode_recovery_snapshot(input)?;
        assert_eq!(first, second);

        let recovered = decode_recovery_snapshot(&first)?;
        assert_eq!(recovered.core_snapshot.as_slice(), b"bounded-core-snapshot");
        assert_eq!(recovered.receipts, receipts);
        assert_eq!(recovered.receipt_order, receipt_order);
        assert_eq!(recovered.program_operations, program_operations);
        assert_eq!(
            recovered.program_activity_operations,
            program_activity_operations
        );
        assert_eq!(recovered.accounts, accounts);
        assert_eq!(recovered.move_quotes, move_quotes);
        assert_eq!(recovered.move_operations, move_operations);
        Ok(())
    }

    #[test]
    fn recovery_snapshot_rejects_tampering_and_inconsistent_bindings(
    ) -> Result<(), Box<dyn std::error::Error>> {
        let activity = "a".repeat(64);
        let idempotency = "b".repeat(64);
        let receipts = HashMap::from([(activity.clone(), "00".to_owned())]);
        let receipt_order = [activity.clone()].into_iter().collect::<VecDeque<_>>();
        let response = serde_json::json!({
            "state":"executed",
            "activity_id":activity.as_str(),
            "idempotency_key":idempotency.as_str(),
        })
        .to_string();
        let operations = HashMap::from([(
            idempotency,
            ProgramOperation {
                activity_id: activity.clone(),
                response,
                retained_signed_activity: None,
            },
        )]);
        let bindings = HashMap::from([(activity, "c".repeat(64))]);
        let accounts = HashMap::new();
        let move_quotes = HashMap::new();
        let move_operations = HashMap::new();
        assert!(encode_recovery_snapshot(super::RecoverySnapshotInput {
            core_snapshot: b"bounded-core-snapshot",
            receipts: &receipts,
            receipt_order: &receipt_order,
            program_operations: &operations,
            program_activity_operations: &bindings,
            accounts: &accounts,
            move_quotes: &move_quotes,
            move_operations: &move_operations,
        })
        .is_err());

        let empty_operations = HashMap::new();
        let empty_bindings = HashMap::new();
        let mut encoded = encode_recovery_snapshot(super::RecoverySnapshotInput {
            core_snapshot: b"bounded-core-snapshot",
            receipts: &receipts,
            receipt_order: &receipt_order,
            program_operations: &empty_operations,
            program_activity_operations: &empty_bindings,
            accounts: &accounts,
            move_quotes: &move_quotes,
            move_operations: &move_operations,
        })?;
        let last = encoded.len() - 1;
        encoded[last] ^= 1;
        assert!(decode_recovery_snapshot(&encoded).is_err());
        Ok(())
    }
}

fn parse_request_headers(bytes: &[u8], header_end: usize) -> Result<RequestHeaders, String> {
    let headers = std::str::from_utf8(&bytes[..header_end]).map_err(|_| "headers are not UTF-8")?;
    let mut lines = headers.split("\r\n");
    let (method, target) = parse_request_line(lines.next().ok_or("missing request line")?)?;
    let path = target.to_string();
    let mut content_length = 0_usize;
    let mut has_content_length = false;
    let mut content_type = String::new();
    let mut has_content_type = false;
    let mut has_host = false;
    let mut idempotency_key = None;
    for line in lines.filter(|line| !line.is_empty()) {
        let (name, value) = line
            .split_once(':')
            .ok_or_else(|| "invalid request header".to_string())?;
        if name.is_empty()
            || !name
                .bytes()
                .all(|byte| byte.is_ascii_alphanumeric() || byte == b'-')
        {
            return Err("invalid request header name".into());
        }
        let value = value.trim_matches([' ', '\t']);
        if value
            .bytes()
            .any(|byte| byte.is_ascii_control() && byte != b'\t')
        {
            return Err("invalid request header value".into());
        }
        if name.eq_ignore_ascii_case("content-length") {
            if has_content_length
                || value.is_empty()
                || !value.bytes().all(|byte| byte.is_ascii_digit())
            {
                return Err("duplicate content length".into());
            }
            content_length = value.parse().map_err(|_| "invalid content length")?;
            has_content_length = true;
        } else if name.eq_ignore_ascii_case("content-type") {
            if has_content_type {
                return Err("duplicate content type".into());
            }
            content_type = value.to_ascii_lowercase();
            has_content_type = true;
        } else if name.eq_ignore_ascii_case("host") {
            if has_host || value.is_empty() {
                return Err("invalid host header".into());
            }
            has_host = true;
        } else if name.eq_ignore_ascii_case("idempotency-key") {
            if idempotency_key.is_some()
                || value.len() != 64
                || !value.bytes().all(|byte| byte.is_ascii_hexdigit())
            {
                return Err("invalid idempotency key".into());
            }
            idempotency_key = Some(value.to_ascii_lowercase());
        } else if name.eq_ignore_ascii_case("transfer-encoding") {
            return Err("transfer encoding is not supported".into());
        }
    }
    if !has_host {
        return Err("missing host header".into());
    }
    if matches!(method, "POST" | "PUT") && !has_content_length {
        return Err("write request is missing content length".into());
    }
    if !matches!(method, "POST" | "PUT") && content_length != 0 {
        return Err("read request may not carry a body".into());
    }
    if content_length > MAX_REQUEST_BYTES {
        return Err("request body exceeds emulator limit".into());
    }
    let method = method.to_owned();
    Ok((method, path, content_type, content_length))
}

type RequestHeaders = (String, String, String, usize);

fn parse_request_line(request_line: &str) -> Result<(&str, &str), String> {
    let mut request_fields = request_line.split(' ');
    let method = request_fields.next().unwrap_or_default();
    let target = request_fields.next().unwrap_or_default();
    if method.is_empty()
        || !method.bytes().all(|byte| byte.is_ascii_uppercase())
        || request_fields.next() != Some("HTTP/1.1")
        || request_fields.next().is_some()
        || !target.starts_with('/')
        || target.starts_with("//")
        || target.contains(['?', '#', '\\', '\0'])
        || target
            .split('/')
            .any(|segment| matches!(segment, "." | ".."))
    {
        return Err("invalid request line".into());
    }
    Ok((method, target))
}

fn spawn_admission(
    listener: TcpListener,
    admission_sender: SyncSender<TcpStream>,
    admission_shutdown: Arc<AtomicBool>,
) -> Result<thread::JoinHandle<()>, String> {
    thread::Builder::new()
        .name("layerx-emulator-admission".to_owned())
        .spawn(move || {
            while !admission_shutdown.load(Ordering::Acquire) {
                match listener.accept().map(|(stream, _)| stream) {
                    Ok(stream) => match admission_sender.try_send(stream) {
                        Ok(()) => {}
                        Err(TrySendError::Full(mut stream)) => {
                            let _ = stream.set_write_timeout(Some(Duration::from_secs(2)));
                            let response = refusal(
                                0,
                                503,
                                "overloaded",
                                "the emulator HTTP admission queue is full",
                            );
                            let _ = write_response(&mut stream, &response);
                        }
                        Err(TrySendError::Disconnected(_)) => return,
                    },
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(10));
                    }
                    Err(error) => {
                        eprintln!("emulator connection error: {error}");
                        return;
                    }
                }
            }
        })
        .map_err(|error| format!("could not start emulator admission worker: {error}"))
}

fn serve_emulator(
    config: &Config,
    mut emulator: Emulator,
    shutdown: Arc<AtomicBool>,
) -> Result<(), String> {
    let listener = TcpListener::bind(config.listen)
        .map_err(|error| format!("cannot listen on {}: {error}", config.listen))?;
    listener
        .set_nonblocking(true)
        .map_err(|error| error.to_string())?;
    eprintln!(
        "LayerX emulator ready on http://{} (network {}, deterministic time {})",
        config.listen, config.network_id, config.timestamp_ms
    );
    let (admission_sender, admission_receiver) = sync_channel::<TcpStream>(ADMISSION_CAPACITY);
    let admission_receiver = Arc::new(Mutex::new(admission_receiver));
    let (transition_sender, transition_receiver) =
        sync_channel::<ParsedConnection>(TRANSITION_CAPACITY);
    let mut workers = Vec::with_capacity(PARSER_WORKERS);
    for index in 0..PARSER_WORKERS {
        let admission_receiver = Arc::clone(&admission_receiver);
        let transition_sender = transition_sender.clone();
        workers.push(
            thread::Builder::new()
                .name(format!("layerx-emulator-http-{index}"))
                .spawn(move || loop {
                    let stream = match admission_receiver.lock() {
                        Ok(receiver) => receiver.recv(),
                        Err(_) => return,
                    };
                    let Ok(mut stream) = stream else {
                        return;
                    };
                    if stream
                        .set_read_timeout(Some(IO_TIMEOUT))
                        .and_then(|()| stream.set_write_timeout(Some(IO_TIMEOUT)))
                        .is_err()
                    {
                        continue;
                    }
                    let request = parse_request(&mut stream);
                    let (response_sender, response_receiver) = sync_channel(1);
                    if transition_sender
                        .send(ParsedConnection {
                            request,
                            response: response_sender,
                        })
                        .is_err()
                    {
                        return;
                    }
                    let Ok(response) = response_receiver.recv() else {
                        return;
                    };
                    let _ = write_response(&mut stream, &response);
                })
                .map_err(|error| {
                    format!("could not start emulator HTTP worker {index}: {error}")
                })?,
        );
    }
    drop(transition_sender);
    let final_shutdown = Arc::clone(&shutdown);
    let admission = spawn_admission(listener, admission_sender, shutdown)?;
    for work in transition_receiver {
        let response = match work.request {
            Ok(request) => route(&mut emulator, &request),
            Err(error) => {
                if let Some(trace) = advance_trace(&mut emulator.trace) {
                    refusal(trace, 400, "invalid_request", &error)
                } else {
                    refusal(
                        u64::MAX,
                        503,
                        "trace_exhausted",
                        "the emulator trace space is exhausted",
                    )
                }
            }
        };
        let _ = work.response.send(response);
    }
    admission
        .join()
        .map_err(|_| "emulator admission worker panicked")?;
    for worker in workers {
        worker.join().map_err(|_| "emulator HTTP worker panicked")?;
    }
    if final_shutdown.load(Ordering::Acquire) {
        Ok(())
    } else {
        Err("all emulator HTTP workers stopped".into())
    }
}

#[derive(Clone, Copy)]
struct RecoverySnapshotInput<'a> {
    core_snapshot: &'a [u8],
    receipts: &'a HashMap<String, String>,
    receipt_order: &'a VecDeque<String>,
    program_operations: &'a HashMap<String, ProgramOperation>,
    program_activity_operations: &'a HashMap<String, String>,
    accounts: &'a HashMap<String, EmulatorAccount>,
    move_quotes: &'a HashMap<String, MoveQuoteRecord>,
    move_operations: &'a HashMap<String, MoveOperation>,
}

fn encode_snapshot_receipts(
    encoded: &mut Vec<u8>,
    receipts: &HashMap<String, String>,
    receipt_order: &VecDeque<String>,
) -> Result<(), String> {
    let mut seen_receipts = HashSet::new();
    for activity_id in receipt_order {
        if !canonical_hex32_text(activity_id) || !seen_receipts.insert(activity_id) {
            return Err("emulator receipt recovery order is invalid".to_owned());
        }
        let activity = hex_decode(activity_id)
            .map_err(|_| "emulator receipt activity id is invalid".to_owned())?;
        let receipt = receipts
            .get(activity_id)
            .ok_or_else(|| "emulator receipt recovery order is incomplete".to_owned())?;
        let receipt = hex_decode(receipt)
            .map_err(|_| "emulator receipt recovery bytes are invalid".to_owned())?;
        if receipt.is_empty() || receipt.len() > MAX_RECEIPT_BYTES {
            return Err("emulator receipt recovery bytes exceed their bound".to_owned());
        }
        append_snapshot_bytes(encoded, &activity)?;
        append_snapshot_u32(encoded, receipt.len())?;
        append_snapshot_bytes(encoded, &receipt)?;
    }

    Ok(())
}

fn encode_snapshot_programs(
    encoded: &mut Vec<u8>,
    program_operations: &HashMap<String, ProgramOperation>,
    program_activity_operations: &HashMap<String, String>,
) -> Result<(), String> {
    let mut operations: Vec<_> = program_operations.iter().collect();
    operations.sort_by(|(left, _), (right, _)| left.cmp(right));
    for (idempotency_key, operation) in operations {
        if !canonical_hex32_text(idempotency_key)
            || !canonical_hex32_text(&operation.activity_id)
            || program_activity_operations.get(&operation.activity_id) != Some(idempotency_key)
            || !canonical_program_response(
                &operation.response,
                &operation.activity_id,
                idempotency_key,
            )
        {
            return Err("emulator program recovery binding is invalid".to_owned());
        }
        let idempotency = hex_decode(idempotency_key)
            .map_err(|_| "emulator program idempotency key is invalid".to_owned())?;
        let activity = hex_decode(&operation.activity_id)
            .map_err(|_| "emulator program activity id is invalid".to_owned())?;
        let retained = match operation.retained_signed_activity.as_deref() {
            Some(value) => {
                let bytes = hex_decode(value).map_err(|_| {
                    "emulator retained signed program activity is invalid".to_owned()
                })?;
                if bytes.is_empty() || bytes.len() > MAX_RETAINED_SIGNED_ACTIVITY_BYTES {
                    return Err(
                        "emulator retained signed program activity exceeds its bound".to_owned(),
                    );
                }
                bytes
            }
            None => Vec::new(),
        };
        append_snapshot_bytes(encoded, &idempotency)?;
        append_snapshot_bytes(encoded, &activity)?;
        append_snapshot_u32(encoded, operation.response.len())?;
        append_snapshot_bytes(encoded, operation.response.as_bytes())?;
        append_snapshot_u32(encoded, retained.len())?;
        append_snapshot_bytes(encoded, &retained)?;
    }

    Ok(())
}

fn encode_snapshot_accounts(
    encoded: &mut Vec<u8>,
    accounts: &HashMap<String, EmulatorAccount>,
) -> Result<(), String> {
    let mut account_entries: Vec<_> = accounts.iter().collect();
    account_entries.sort_by(|(left, _), (right, _)| left.cmp(right));
    for (name, account) in account_entries {
        if name != &format!("agent:{}:main", account.did)
            || account.did.is_empty()
            || account.did.len() > 512
            || !account.did.starts_with("did:")
            || account.did.contains('\0')
            || account.public_key == [0; 32]
            || account.id == [0; 32]
        {
            return Err("emulator account recovery binding is invalid".to_owned());
        }
        append_snapshot_text(encoded, name)?;
        append_snapshot_text(encoded, &account.did)?;
        append_snapshot_bytes(encoded, &account.id)?;
        append_snapshot_bytes(encoded, &account.public_key)?;
    }

    Ok(())
}

fn encode_snapshot_quotes(
    encoded: &mut Vec<u8>,
    move_quotes: &HashMap<String, MoveQuoteRecord>,
    accounts: &HashMap<String, EmulatorAccount>,
) -> Result<(), String> {
    let mut quote_entries: Vec<_> = move_quotes.iter().collect();
    quote_entries.sort_by(|(left, _), (right, _)| left.cmp(right));
    for (quote_id, quote) in quote_entries {
        let quote_hex = quote_id.strip_prefix("qte_");
        if quote_id != &quote.quote_id
            || quote_hex.is_none_or(|value| !canonical_hex32_text(value))
            || quote.source == quote.destination
            || quote.amount == 0
            || quote.currency != "LXP"
            || quote.created_at == 0
            || quote.expires_at <= quote.created_at
            || accounts
                .get(&quote.source)
                .is_none_or(|account| account.id != quote.source_id)
            || accounts
                .get(&quote.destination)
                .is_none_or(|account| account.id != quote.destination_id)
            || quote
                .committed_idempotency
                .as_deref()
                .is_some_and(|value| !valid_human_idempotency(value))
        {
            return Err("emulator move quote recovery binding is invalid".to_owned());
        }
        append_snapshot_text(encoded, quote_id)?;
        append_snapshot_text(encoded, &quote.source)?;
        append_snapshot_text(encoded, &quote.destination)?;
        append_snapshot_bytes(encoded, &quote.source_id)?;
        append_snapshot_bytes(encoded, &quote.destination_id)?;
        append_snapshot_bytes(encoded, &quote.amount.to_be_bytes())?;
        append_snapshot_text(encoded, &quote.currency)?;
        append_snapshot_bytes(encoded, &quote.created_at.to_be_bytes())?;
        append_snapshot_bytes(encoded, &quote.expires_at.to_be_bytes())?;
        append_snapshot_bytes(encoded, &quote.identity_sequence.to_be_bytes())?;
        append_snapshot_bytes(encoded, &quote.source_sequence.to_be_bytes())?;
        match quote.committed_idempotency.as_deref() {
            Some(value) => {
                append_snapshot_bytes(encoded, &[1])?;
                append_snapshot_text(encoded, value)?;
            }
            None => append_snapshot_bytes(encoded, &[0])?,
        }
    }

    Ok(())
}

fn encode_snapshot_moves(
    encoded: &mut Vec<u8>,
    move_operations: &HashMap<String, MoveOperation>,
    move_quotes: &HashMap<String, MoveQuoteRecord>,
) -> Result<(), String> {
    let mut move_entries: Vec<_> = move_operations.iter().collect();
    move_entries.sort_by(|(left, _), (right, _)| left.cmp(right));
    for (idempotency, operation) in move_entries {
        if !valid_human_idempotency(idempotency)
            || operation.body.is_empty()
            || operation.body.len() > MAX_PROGRAM_RESPONSE_BYTES
            || !matches!(operation.status, 200 | 400 | 404 | 409 | 503)
            || serde_json::from_str::<serde_json::Value>(&operation.body).is_err()
            || move_quotes
                .get(&operation.quote_id)
                .and_then(|quote| quote.committed_idempotency.as_deref())
                != Some(idempotency)
        {
            return Err("emulator move operation recovery binding is invalid".to_owned());
        }
        append_snapshot_text(encoded, idempotency)?;
        append_snapshot_text(encoded, &operation.quote_id)?;
        append_snapshot_bytes(encoded, &operation.status.to_be_bytes())?;
        append_snapshot_u32(encoded, operation.body.len())?;
        append_snapshot_bytes(encoded, operation.body.as_bytes())?;
    }

    Ok(())
}

type ReceiptIndex = (HashMap<String, String>, VecDeque<String>);
type ProgramIndex = (HashMap<String, ProgramOperation>, HashMap<String, String>);

fn decode_snapshot_receipts(
    authenticated: &[u8],
    offset: &mut usize,
    receipt_count: usize,
) -> Result<ReceiptIndex, String> {
    let mut receipts = HashMap::with_capacity(receipt_count);
    let mut receipt_order = VecDeque::with_capacity(receipt_count);
    for _ in 0..receipt_count {
        let activity_id = hex_encode(snapshot_take(authenticated, offset, 32)?);
        let receipt_length = snapshot_u32(authenticated, offset)?;
        if receipt_length == 0 || receipt_length > MAX_RECEIPT_BYTES {
            return Err("emulator recovery receipt exceeds its bound".to_owned());
        }
        let receipt = hex_encode(snapshot_take(authenticated, offset, receipt_length)?);
        if receipts.insert(activity_id.clone(), receipt).is_some() {
            return Err("emulator recovery receipt activity is duplicated".to_owned());
        }
        receipt_order.push_back(activity_id);
    }

    Ok((receipts, receipt_order))
}

fn decode_snapshot_programs(
    authenticated: &[u8],
    offset: &mut usize,
    operation_count: usize,
) -> Result<ProgramIndex, String> {
    let mut program_operations = HashMap::with_capacity(operation_count);
    let mut program_activity_operations = HashMap::with_capacity(operation_count);
    for _ in 0..operation_count {
        let idempotency_key = hex_encode(snapshot_take(authenticated, offset, 32)?);
        let activity_id = hex_encode(snapshot_take(authenticated, offset, 32)?);
        let response_length = snapshot_u32(authenticated, offset)?;
        if response_length == 0 || response_length > MAX_PROGRAM_RESPONSE_BYTES {
            return Err("emulator recovery program response exceeds its bound".to_owned());
        }
        let response = std::str::from_utf8(snapshot_take(authenticated, offset, response_length)?)
            .map_err(|_| "emulator recovery program response is not UTF-8".to_owned())?
            .to_owned();
        if !canonical_program_response(&response, &activity_id, &idempotency_key) {
            return Err("emulator recovery program response is invalid".to_owned());
        }
        let retained_length = snapshot_u32(authenticated, offset)?;
        if retained_length > MAX_RETAINED_SIGNED_ACTIVITY_BYTES {
            return Err("emulator retained recovery activity exceeds its bound".to_owned());
        }
        let retained_signed_activity = if retained_length == 0 {
            None
        } else {
            Some(hex_encode(snapshot_take(
                authenticated,
                offset,
                retained_length,
            )?))
        };
        if program_activity_operations
            .insert(activity_id.clone(), idempotency_key.clone())
            .is_some()
            || program_operations
                .insert(
                    idempotency_key,
                    ProgramOperation {
                        activity_id,
                        response,
                        retained_signed_activity,
                    },
                )
                .is_some()
        {
            return Err("emulator recovery program binding is duplicated".to_owned());
        }
    }

    Ok((program_operations, program_activity_operations))
}

fn decode_snapshot_accounts(
    authenticated: &[u8],
    offset: &mut usize,
    account_count: usize,
) -> Result<HashMap<String, EmulatorAccount>, String> {
    let mut accounts = HashMap::with_capacity(account_count);
    for _ in 0..account_count {
        let name = snapshot_text(authenticated, offset, 1_024)?;
        let did = snapshot_text(authenticated, offset, 512)?;
        let id: [u8; 32] = snapshot_take(authenticated, offset, 32)?
            .try_into()
            .map_err(|_| "emulator recovery account id is invalid".to_owned())?;
        let public_key: [u8; 32] = snapshot_take(authenticated, offset, 32)?
            .try_into()
            .map_err(|_| "emulator recovery public key is invalid".to_owned())?;
        if name != format!("agent:{did}:main")
            || !did.starts_with("did:")
            || did.contains('\0')
            || id == [0; 32]
            || public_key == [0; 32]
            || accounts
                .insert(
                    name,
                    EmulatorAccount {
                        id,
                        did,
                        public_key,
                    },
                )
                .is_some()
        {
            return Err("emulator recovery account binding is invalid".to_owned());
        }
    }

    Ok(accounts)
}

fn decode_snapshot_quotes(
    authenticated: &[u8],
    offset: &mut usize,
    move_quote_count: usize,
    accounts: &HashMap<String, EmulatorAccount>,
) -> Result<HashMap<String, MoveQuoteRecord>, String> {
    let mut move_quotes = HashMap::with_capacity(move_quote_count);
    for _ in 0..move_quote_count {
        let quote_id = snapshot_text(authenticated, offset, 80)?;
        let source = snapshot_text(authenticated, offset, 1_024)?;
        let destination = snapshot_text(authenticated, offset, 1_024)?;
        let source_id: [u8; 32] = snapshot_take(authenticated, offset, 32)?
            .try_into()
            .map_err(|_| "emulator recovery source id is invalid".to_owned())?;
        let destination_id: [u8; 32] = snapshot_take(authenticated, offset, 32)?
            .try_into()
            .map_err(|_| "emulator recovery destination id is invalid".to_owned())?;
        let amount = snapshot_u128(authenticated, offset)?;
        let currency = snapshot_text(authenticated, offset, 16)?;
        let created_at = snapshot_u64(authenticated, offset)?;
        let expires_at = snapshot_u64(authenticated, offset)?;
        let identity_sequence = snapshot_u64(authenticated, offset)?;
        let source_sequence = snapshot_u64(authenticated, offset)?;
        let committed_idempotency = match snapshot_take(authenticated, offset, 1)?[0] {
            0 => None,
            1 => Some(snapshot_text(authenticated, offset, 128)?),
            _ => return Err("emulator recovery move quote state is invalid".to_owned()),
        };
        let quote = MoveQuoteRecord {
            quote_id: quote_id.clone(),
            source,
            destination,
            source_id,
            destination_id,
            amount,
            currency,
            created_at,
            expires_at,
            identity_sequence,
            source_sequence,
            committed_idempotency,
        };
        if quote_id
            .strip_prefix("qte_")
            .is_none_or(|value| !canonical_hex32_text(value))
            || quote.source == quote.destination
            || quote.amount == 0
            || quote.currency != "LXP"
            || quote.created_at == 0
            || quote.expires_at <= quote.created_at
            || accounts
                .get(&quote.source)
                .is_none_or(|account| account.id != quote.source_id)
            || accounts
                .get(&quote.destination)
                .is_none_or(|account| account.id != quote.destination_id)
            || quote
                .committed_idempotency
                .as_deref()
                .is_some_and(|value| !valid_human_idempotency(value))
            || move_quotes.insert(quote_id, quote).is_some()
        {
            return Err("emulator recovery move quote binding is invalid".to_owned());
        }
    }

    Ok(move_quotes)
}

fn decode_snapshot_moves(
    authenticated: &[u8],
    offset: &mut usize,
    move_operation_count: usize,
    move_quotes: &HashMap<String, MoveQuoteRecord>,
) -> Result<HashMap<String, MoveOperation>, String> {
    let mut move_operations = HashMap::with_capacity(move_operation_count);
    for _ in 0..move_operation_count {
        let idempotency = snapshot_text(authenticated, offset, 128)?;
        let quote_id = snapshot_text(authenticated, offset, 80)?;
        let status_bytes: [u8; 2] = snapshot_take(authenticated, offset, 2)?
            .try_into()
            .map_err(|_| "emulator recovery move status is invalid".to_owned())?;
        let status = u16::from_be_bytes(status_bytes);
        let body_length = snapshot_u32(authenticated, offset)?;
        if body_length == 0 || body_length > MAX_PROGRAM_RESPONSE_BYTES {
            return Err("emulator recovery move response exceeds its bound".to_owned());
        }
        let body = std::str::from_utf8(snapshot_take(authenticated, offset, body_length)?)
            .map_err(|_| "emulator recovery move response is not UTF-8".to_owned())?
            .to_owned();
        let operation = MoveOperation {
            quote_id: quote_id.clone(),
            status,
            body,
        };
        if !valid_human_idempotency(&idempotency)
            || !matches!(status, 200 | 400 | 404 | 409 | 503)
            || serde_json::from_str::<serde_json::Value>(&operation.body).is_err()
            || move_quotes
                .get(&quote_id)
                .and_then(|quote| quote.committed_idempotency.as_deref())
                != Some(idempotency.as_str())
            || move_operations.insert(idempotency, operation).is_some()
        {
            return Err("emulator recovery move operation binding is invalid".to_owned());
        }
    }
    Ok(move_operations)
}
