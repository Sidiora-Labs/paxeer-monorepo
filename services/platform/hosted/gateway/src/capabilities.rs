//! Which fork surfaces the Paxeer chain answers for.
//!
//! The exchange, bridge, launchpad, pay and programs precompiles exist from the Paxeer X
//! fork on. A native precompile carries no bytecode, so `eth_getCode` answers
//! `0x` at its address on either side of the fork; the gateway instead sends
//! each surface one cheap view from its ABI by `eth_call` at one node height.
//! An answer that decodes marks the surface live; an empty answer or an
//! execution error marks it absent. The gateway keeps the probe for a short
//! TTL, publishes it as `px_getCapabilities`, and refuses a raw transaction
//! addressed to an absent surface with the typed `surface_unavailable` error
//! instead of relaying it.

use super::{paxeer, Config};
use layerx_platform_gateway::evm;
use layerx_platform_gateway::http::Endpoint;
use serde_json::{json, Value};
use std::sync::Mutex;
use std::time::{Duration, Instant};

/// JSON-RPC code of the typed refusal, between `-32002` (insufficient scope)
/// and `-32004` (WebSocket required) in the gateway's scheme.
pub(super) const SURFACE_UNAVAILABLE: i32 = -32003;

/// Relayed EVM methods that submit a signed transaction to the node.
pub(super) const SUBMISSION_METHODS: [&str; 2] =
    ["eth_sendRawTransaction", "eth_sendRawTransactionSync"];

const DEFAULT_TTL_SECONDS: u64 = 15;
const MAX_TTL_SECONDS: u64 = 300;
const MAX_RAW_TRANSACTION_BYTES: usize = 4 * 1024 * 1024;

/// Chain JSON-RPC path under a router root configured without one.
const CHAIN_RPC_PATH: &str = "/rpc";

/// The probe view of one surface: a cheap view that the precompile
/// serving the surface's address declares in its served ABI file.
struct ProbeView {
    signature: &'static str,
    selector: [u8; 4],
    outputs: &'static [&'static str],
    words: usize,
    /// The served ABI file, relative to the repository root.
    abi: &'static str,
}

/// One fork surface and the precompile that carries it.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(super) enum Surface {
    Exchange,
    Bridge,
    Launchpad,
    Pay,
    Programs,
}

impl Surface {
    pub(super) const ALL: [Self; 5] = [
        Self::Exchange,
        Self::Bridge,
        Self::Launchpad,
        Self::Pay,
        Self::Programs,
    ];

    pub(super) fn name(self) -> &'static str {
        match self {
            Self::Exchange => "exchange",
            Self::Bridge => "bridge",
            Self::Launchpad => "launchpad",
            Self::Pay => "pay",
            Self::Programs => "programs",
        }
    }

    pub(super) fn address(self) -> [u8; 20] {
        match self {
            Self::Exchange => evm::EXCHANGE_PRECOMPILE,
            Self::Bridge => evm::BRIDGE_PRECOMPILE,
            Self::Launchpad => evm::LAUNCHPAD_PRECOMPILE,
            Self::Pay => evm::PAY_PRECOMPILE,
            Self::Programs => evm::PROGRAMS_PRECOMPILE,
        }
    }

    /// The probe table. 0x1015 serves the perpetuals precompile from the
    /// v6.12.0 activation on, so the exchange surface is probed with its view.
    const fn probe_view(self) -> ProbeView {
        match self {
            Self::Exchange => ProbeView {
                signature: "totalMarkets()",
                selector: [0x81, 0x62, 0x48, 0x6b],
                outputs: &["uint256"],
                words: 1,
                abi: "precompiles/perps/abi/abi.json",
            },
            Self::Bridge => ProbeView {
                signature: "isPaused()",
                selector: [0xb1, 0x87, 0xbd, 0x26],
                outputs: &["bool"],
                words: 1,
                abi: "precompiles/layerxbridge/abi.json",
            },
            Self::Launchpad => ProbeView {
                signature: "getPoolCount()",
                selector: [0x8e, 0xec, 0x5d, 0x70],
                outputs: &["uint256"],
                words: 1,
                abi: "precompiles/launchpad/abi.json",
            },
            Self::Pay => ProbeView {
                signature: "budgetRemaining(bytes32)",
                selector: [0xd7, 0x77, 0x87, 0x49],
                outputs: &["uint128"],
                words: 1,
                abi: "precompiles/layerxpay/abi.json",
            },
            Self::Programs => ProbeView {
                signature: "getCurrentFeeSchedule()",
                selector: [0xbb, 0xe2, 0xe4, 0x2d],
                outputs: &["uint32", "uint64[7]", "bytes32"],
                words: 9,
                abi: "precompiles/layerxprograms/abi.json",
            },
        }
    }

    /// Calldata of the cheap view the probe sends to this surface.
    fn probe_calldata(self) -> Vec<u8> {
        let selector = self.probe_view().selector;
        match self {
            Self::Pay => evm::calldata_word(selector, &[0; 32]),
            _ => evm::calldata_empty(selector),
        }
    }

    /// Whether `answer` decodes as this surface's probe view return value.
    fn decodes(self, answer: &[u8]) -> bool {
        if answer.len() != self.probe_view().words * 32 {
            return false;
        }
        let answer = evm::Answer::new(answer);
        match self {
            Self::Bridge => answer.bool(0).is_ok(),
            Self::Pay => answer.word(0).is_ok_and(|word| word[..16] == [0; 16]),
            Self::Programs => {
                answer.u32(0).is_ok()
                    && (1..8).all(|index| answer.u64(index).is_ok())
                    && answer.word(8).is_ok()
            }
            Self::Exchange | Self::Launchpad => answer.word(0).is_ok(),
        }
    }

    fn of(address: &[u8; 20]) -> Option<Self> {
        Self::ALL
            .into_iter()
            .find(|surface| surface.address() == *address)
    }
}

/// One probe of the surfaces at one node height.
#[derive(Clone, Debug, Eq, PartialEq)]
pub(super) struct Capabilities {
    pub(super) exchange: bool,
    pub(super) bridge: bool,
    pub(super) launchpad: bool,
    pub(super) pay: bool,
    pub(super) programs: bool,
    pub(super) probed_at: u64,
    pub(super) rpc_height: u64,
}

impl Capabilities {
    pub(super) fn live(&self, surface: Surface) -> bool {
        match surface {
            Surface::Exchange => self.exchange,
            Surface::Bridge => self.bridge,
            Surface::Launchpad => self.launchpad,
            Surface::Pay => self.pay,
            Surface::Programs => self.programs,
        }
    }

    pub(super) fn document(&self) -> Value {
        let addresses: serde_json::Map<String, Value> = Surface::ALL
            .into_iter()
            .map(|surface| {
                (
                    surface.name().to_owned(),
                    json!(evm::address_hex(&surface.address())),
                )
            })
            .collect();
        json!({
            "exchange": self.exchange,
            "bridge": self.bridge,
            "launchpad": self.launchpad,
            "pay": self.pay,
            "programs": self.programs,
            "addresses": addresses,
            "probed_at": self.probed_at,
            "rpc_height": self.rpc_height.to_string()
        })
    }
}

/// The last probe and when it was taken.
pub(super) struct Cache {
    ttl: Duration,
    entry: Mutex<Option<(Instant, Capabilities)>>,
}

impl Cache {
    pub(super) fn new(ttl: Duration) -> Self {
        Self {
            ttl,
            entry: Mutex::new(None),
        }
    }

    /// The cached answer while it is younger than the TTL, otherwise a fresh
    /// probe. A failed probe is returned and never cached.
    pub(super) fn current(
        &self,
        now: Instant,
        probe: impl FnOnce() -> Result<Capabilities, &'static str>,
    ) -> Result<Capabilities, &'static str> {
        let mut entry = self
            .entry
            .lock()
            .map_err(|_| "capabilities_cache_unavailable")?;
        if let Some((taken, capabilities)) = entry.as_ref() {
            if now.saturating_duration_since(*taken) < self.ttl {
                return Ok(capabilities.clone());
            }
        }
        let fresh = probe()?;
        *entry = Some((now, fresh.clone()));
        Ok(fresh)
    }
}

pub(super) fn configured() -> Result<Cache, String> {
    let seconds = std::env::var("LAYERX_GATEWAY_CAPABILITIES_TTL_SECONDS")
        .unwrap_or_else(|_| DEFAULT_TTL_SECONDS.to_string())
        .parse::<u64>()
        .map_err(|_| "gateway capabilities TTL is invalid".to_owned())?;
    if !(1..=MAX_TTL_SECONDS).contains(&seconds) {
        return Err("gateway capabilities TTL is outside its bound".to_owned());
    }
    Ok(Cache::new(Duration::from_secs(seconds)))
}

/// The chain JSON-RPC endpoint for one configured Paxeer name: a bare router
/// root answers chain JSON-RPC under `/rpc`, and an explicit path is kept.
pub(super) fn chain_rpc(mut endpoint: Endpoint) -> Endpoint {
    if endpoint.base_path.is_empty() {
        CHAIN_RPC_PATH.clone_into(&mut endpoint.base_path);
    }
    endpoint
}

/// Whether one probe `eth_call` answer shows the surface live: a result that
/// decodes as the view's return value is live; an execution error, an empty
/// result or one that does not decode is absent.
pub(super) fn answers(surface: Surface, answer: &Value) -> bool {
    answer
        .get("result")
        .and_then(Value::as_str)
        .and_then(|text| text.strip_prefix("0x"))
        .and_then(|digits| super::decode_hex(digits, surface.probe_view().words * 32).ok())
        .is_some_and(|bytes| surface.decodes(&bytes))
}

fn height(answer: &Value) -> Result<u64, &'static str> {
    answer
        .get("result")
        .and_then(Value::as_str)
        .and_then(|text| text.strip_prefix("0x"))
        .filter(|digits| !digits.is_empty() && digits.len() <= 16)
        .and_then(|digits| u64::from_str_radix(digits, 16).ok())
        .ok_or("invalid_paxeer_response")
}

/// Reads the node head, then calls every surface's probe view at that head.
pub(super) fn probe(
    mut node: impl FnMut(&Value) -> Result<Value, &'static str>,
    probed_at: u64,
) -> Result<Capabilities, &'static str> {
    let head = node(&json!({
        "jsonrpc": "2.0",
        "id": "px-capabilities",
        "method": "eth_blockNumber",
        "params": []
    }))?;
    let rpc_height = height(&head)?;
    let tag = format!("0x{rpc_height:x}");
    let mut live = [false; Surface::ALL.len()];
    for (slot, surface) in live.iter_mut().zip(Surface::ALL) {
        let answer = node(&json!({
            "jsonrpc": "2.0",
            "id": "px-capabilities",
            "method": "eth_call",
            "params": [
                {
                    "to": evm::address_hex(&surface.address()),
                    "data": format!("0x{}", super::hex(&surface.probe_calldata()))
                },
                tag
            ]
        }))?;
        *slot = answers(surface, &answer);
    }
    let [exchange, bridge, launchpad, pay, programs] = live;
    Ok(Capabilities {
        exchange,
        bridge,
        launchpad,
        pay,
        programs,
        probed_at,
        rpc_height,
    })
}

fn current(config: &Config) -> Result<Capabilities, &'static str> {
    config.capabilities.current(Instant::now(), || {
        let probed_at = super::now().map_err(|_| "clock_unavailable")?;
        probe(|request| paxeer::node(config, request), probed_at)
    })
}

/// `px_getCapabilities`.
pub(super) fn get(config: &Config, id: &Value, params: Option<&Value>) -> Value {
    if config.paxeer.is_none() {
        return paxeer::unconfigured(id);
    }
    if !paxeer::no_params(params) {
        return super::rpc::error(id, -32602, "Invalid params");
    }
    match current(config) {
        Ok(capabilities) => json!({"jsonrpc":"2.0","id":id,"result":capabilities.document()}),
        Err(code) => paxeer::unavailable(id, code),
    }
}

/// The typed refusal for a write to a surface whose probe view did not answer.
pub(super) fn refusal(id: &Value, surface: Surface) -> Value {
    let mut refusal = super::rpc::error(id, SURFACE_UNAVAILABLE, "Surface unavailable");
    refusal["error"]["data"] = json!({
        "code": "surface_unavailable",
        "surface": surface.name(),
        "address": evm::address_hex(&surface.address())
    });
    refusal
}

/// The fork surface a submission writes to, or `None` when the method is not
/// a submission or its transaction is not addressed to a surface precompile.
pub(super) fn submission_surface(method: &str, params: Option<&Value>) -> Option<Surface> {
    if !SUBMISSION_METHODS.contains(&method) {
        return None;
    }
    let Some(Value::Array(args)) = params else {
        return None;
    };
    let digits = args.first()?.as_str()?.strip_prefix("0x")?;
    let raw = super::decode_hex(digits, MAX_RAW_TRANSACTION_BYTES).ok()?;
    Surface::of(&transaction_target(&raw)?)
}

/// Consults the capabilities for a surface write and answers the refusal, or
/// `None` when the request may be relayed.
pub(super) fn gate_with(
    cache: &Cache,
    now: Instant,
    probe: impl FnOnce() -> Result<Capabilities, &'static str>,
    id: &Value,
    method: &str,
    params: Option<&Value>,
) -> Option<Value> {
    let surface = submission_surface(method, params)?;
    match cache.current(now, probe) {
        Ok(capabilities) if capabilities.live(surface) => None,
        Ok(_) => Some(refusal(id, surface)),
        Err(code) => Some(paxeer::unavailable(id, code)),
    }
}

pub(super) fn gate(
    config: &Config,
    id: &Value,
    method: &str,
    params: Option<&Value>,
) -> Option<Value> {
    config.paxeer.as_ref()?;
    gate_with(
        &config.capabilities,
        Instant::now(),
        || {
            let probed_at = super::now().map_err(|_| "clock_unavailable")?;
            probe(|request| paxeer::node(config, request), probed_at)
        },
        id,
        method,
        params,
    )
}

fn rlp_length(bytes: &[u8]) -> Option<usize> {
    if bytes.is_empty() || bytes.len() > 8 || bytes.first() == Some(&0) {
        return None;
    }
    bytes.iter().try_fold(0_usize, |length, byte| {
        length.checked_mul(256)?.checked_add(usize::from(*byte))
    })
}

/// One RLP item at the start of `bytes`: payload offset, payload length and
/// whether it is a list.
fn rlp_item(bytes: &[u8]) -> Option<(usize, usize, bool)> {
    let first = *bytes.first()?;
    let (offset, length, list) = match first {
        0x00..=0x7f => (0, 1, false),
        0x80..=0xb7 => (1, usize::from(first - 0x80), false),
        0xb8..=0xbf => {
            let width = usize::from(first - 0xb7);
            (1 + width, rlp_length(bytes.get(1..=width)?)?, false)
        }
        0xc0..=0xf7 => (1, usize::from(first - 0xc0), true),
        0xf8..=0xff => {
            let width = usize::from(first - 0xf7);
            (1 + width, rlp_length(bytes.get(1..=width)?)?, true)
        }
    };
    (offset.checked_add(length)? <= bytes.len()).then_some((offset, length, list))
}

/// The `to` field of a signed legacy, EIP-2930, EIP-1559, EIP-4844 or
/// EIP-7702 transaction, or `None` for a contract creation or bytes that do
/// not decode.
pub(super) fn transaction_target(raw: &[u8]) -> Option<[u8; 20]> {
    let (body, field) = match *raw.first()? {
        0x01 => (raw.get(1..)?, 4),
        0x02..=0x04 => (raw.get(1..)?, 5),
        0xc0..=0xff => (raw, 3),
        _ => return None,
    };
    let (offset, length, list) = rlp_item(body)?;
    if !list || offset + length != body.len() {
        return None;
    }
    let mut fields = body.get(offset..offset + length)?;
    if raw.first() == Some(&0x03) {
        let (inner, inner_length, inner_list) = rlp_item(fields)?;
        if inner_list {
            fields = fields.get(inner..inner + inner_length)?;
        }
    }
    for _ in 0..field {
        let (skip, skipped, _) = rlp_item(fields)?;
        fields = fields.get(skip + skipped..)?;
    }
    let (offset, length, list) = rlp_item(fields)?;
    if list || offset != 1 || length != 20 {
        return None;
    }
    fields.get(1..21)?.try_into().ok()
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::cell::Cell;

    fn rlp_bytes(bytes: &[u8]) -> Vec<u8> {
        match bytes {
            [single] if *single < 0x80 => vec![*single],
            _ => {
                let mut out = rlp_header(0x80, bytes.len());
                out.extend_from_slice(bytes);
                out
            }
        }
    }

    fn rlp_list(items: &[Vec<u8>]) -> Vec<u8> {
        let payload: Vec<u8> = items.concat();
        let mut out = rlp_header(0xc0, payload.len());
        out.extend(payload);
        out
    }

    fn rlp_header(base: u8, length: usize) -> Vec<u8> {
        if length <= 55 {
            return vec![base + u8::try_from(length).unwrap_or_else(|e| panic!("{e}"))];
        }
        let width: Vec<u8> = length
            .to_be_bytes()
            .into_iter()
            .skip_while(|byte| *byte == 0)
            .collect();
        let mut out = vec![base + 55 + u8::try_from(width.len()).unwrap_or_else(|e| panic!("{e}"))];
        out.extend(width);
        out
    }

    fn eip1559(to: &[u8]) -> Vec<u8> {
        let mut raw = vec![0x02];
        raw.extend(rlp_list(&[
            rlp_bytes(&[0x01, 0x2d]),
            rlp_bytes(&[0x07]),
            rlp_bytes(&[0x3b, 0x9a, 0xca, 0x00]),
            rlp_bytes(&[0x3b, 0x9a, 0xca, 0x00]),
            rlp_bytes(&[0x01, 0x86, 0xa0]),
            rlp_bytes(to),
            rlp_bytes(&[]),
            rlp_bytes(&[0xab; 100]),
            rlp_list(&[]),
            rlp_bytes(&[0x01]),
            rlp_bytes(&[0x11; 32]),
            rlp_bytes(&[0x22; 32]),
        ]));
        raw
    }

    fn legacy(to: &[u8]) -> Vec<u8> {
        rlp_list(&[
            rlp_bytes(&[0x07]),
            rlp_bytes(&[0x3b, 0x9a, 0xca, 0x00]),
            rlp_bytes(&[0x52, 0x08]),
            rlp_bytes(to),
            rlp_bytes(&[0x01]),
            rlp_bytes(&[]),
            rlp_bytes(&[0x02, 0x7e]),
            rlp_bytes(&[0x11; 32]),
            rlp_bytes(&[0x22; 32]),
        ])
    }

    fn submission(raw: &[u8]) -> Value {
        json!([format!("0x{}", crate::hex(raw))])
    }

    fn capabilities(exchange: bool, bridge: bool, launchpad: bool) -> Capabilities {
        Capabilities {
            exchange,
            bridge,
            launchpad,
            pay: false,
            programs: false,
            probed_at: 1_790_000_000,
            rpc_height: 23_860_000,
        }
    }

    fn word(value: u64) -> String {
        format!("0x{value:064x}")
    }

    /// Paxeer answers to the probe: pre-fork every view answers `0x` (a call
    /// to an address without a precompile), post-fork each answers its ABI
    /// return word, and `eth_getCode` stays `0x` either way.
    fn recorded_node(
        forked: bool,
        calls: &Cell<usize>,
    ) -> impl FnMut(&Value) -> Result<Value, &'static str> + '_ {
        move |request: &Value| {
            calls.set(calls.get() + 1);
            let result = match request["method"].as_str() {
                Some("eth_blockNumber") => json!("0x16c1320"),
                Some("eth_call") => {
                    assert_eq!(request["params"][1], json!("0x16c1320"));
                    let to = request["params"][0]["to"].as_str().unwrap_or_default();
                    let surface = Surface::ALL
                        .into_iter()
                        .find(|surface| evm::address_hex(&surface.address()) == to)
                        .unwrap_or_else(|| panic!("probe called {to}"));
                    assert_eq!(
                        request["params"][0]["data"],
                        json!(format!("0x{}", crate::hex(&surface.probe_calldata())))
                    );
                    if forked {
                        json!(match surface {
                            Surface::Exchange => word(7),
                            Surface::Bridge => word(0),
                            Surface::Launchpad | Surface::Pay => word(0),
                            Surface::Programs => format!("0x{}", "00".repeat(9 * 32)),
                        })
                    } else {
                        json!("0x")
                    }
                }
                other => panic!("unexpected probe method {other:?}"),
            };
            Ok(json!({"jsonrpc": "2.0", "id": request["id"], "result": result}))
        }
    }

    #[test]
    fn capabilities_probe_calls_each_surface_view_and_reads_a_decoded_answer_as_live() {
        assert_eq!(
            Surface::Exchange.probe_calldata(),
            vec![0x81, 0x62, 0x48, 0x6b]
        );
        assert_eq!(
            Surface::Bridge.probe_calldata(),
            vec![0xb1, 0x87, 0xbd, 0x26]
        );
        assert_eq!(
            Surface::Launchpad.probe_calldata(),
            vec![0x8e, 0xec, 0x5d, 0x70]
        );

        for surface in Surface::ALL {
            assert!(
                answers(
                    surface,
                    &json!({"result": format!("0x{}", "00".repeat(surface.probe_view().words * 32))})
                ),
                "{}",
                surface.name()
            );
            assert!(
                answers(
                    surface,
                    &json!({"result": format!("{}{}", word(1), "00".repeat((surface.probe_view().words - 1) * 32))})
                ),
                "{}",
                surface.name()
            );
            for absent in [
                json!({"error": {"code": -32000, "message": "execution reverted"}}),
                json!({"error": {"code": -32601, "message": "precompile not active"}}),
                json!({"result": "0x"}),
                json!({"result": "0x01"}),
                json!({"result": format!("{}00", word(1))}),
                json!({"result": word(1).trim_start_matches("0x")}),
                json!({"result": "0xzz"}),
            ] {
                assert!(!answers(surface, &absent), "{} {absent}", surface.name());
            }
        }
        assert!(!answers(Surface::Bridge, &json!({"result": word(2)})));
        for surface in [Surface::Exchange, Surface::Launchpad] {
            assert!(answers(
                surface,
                &json!({"result": format!("0x01{}", "00".repeat(31))})
            ));
        }

        let calls = Cell::new(0);
        let before = probe(recorded_node(false, &calls), 1_790_000_000)
            .unwrap_or_else(|code| panic!("{code}"));
        assert_eq!(calls.get(), 6);
        assert_eq!(before, {
            let mut expected = capabilities(false, false, false);
            expected.rpc_height = 0x16c_1320;
            expected
        });
        assert_eq!(
            before.document(),
            json!({
                "exchange": false,
                "bridge": false,
                "launchpad": false,
                "pay": false,
                "programs": false,
                "addresses": {
                    "exchange": "0x0000000000000000000000000000000000001015",
                    "bridge": "0x0000000000000000000000000000000000001016",
                    "launchpad": "0x0000000000000000000000000000000000001017",
                    "pay": "0x000000000000000000000000000000000000101b",
                    "programs": "0x000000000000000000000000000000000000101d"
                },
                "probed_at": 1_790_000_000_u64,
                "rpc_height": "23860000"
            })
        );
        let after = probe(recorded_node(true, &calls), 1_790_000_015)
            .unwrap_or_else(|code| panic!("{code}"));
        assert_eq!(calls.get(), 12);
        assert!(Surface::ALL.into_iter().all(|surface| after.live(surface)));

        let mut bridge_refuses = recorded_node(true, &calls);
        let partial = probe(
            |request: &Value| {
                if request["params"][0]["to"] == json!(evm::address_hex(&evm::BRIDGE_PRECOMPILE)) {
                    return Ok(json!({
                        "jsonrpc": "2.0",
                        "id": request["id"],
                        "error": {"code": -32000, "message": "execution reverted"}
                    }));
                }
                bridge_refuses(request)
            },
            1_790_000_030,
        )
        .unwrap_or_else(|code| panic!("{code}"));
        assert!(partial.exchange && !partial.bridge && partial.launchpad);

        assert_eq!(
            probe(
                |_: &Value| Ok(json!({"jsonrpc": "2.0", "id": "px-capabilities", "result": "0x"})),
                0
            ),
            Err("invalid_paxeer_response")
        );
        assert_eq!(
            probe(|_: &Value| Err("paxeer_unreachable"), 0),
            Err("paxeer_unreachable")
        );
    }

    #[test]
    fn launchpad_probe_uses_get_pool_count() {
        assert_eq!(
            Surface::Launchpad.probe_view().selector,
            [0x8e, 0xec, 0x5d, 0x70]
        );
        assert_eq!(Surface::Launchpad.address(), evm::LAUNCHPAD_PRECOMPILE);
        assert_eq!(
            evm::address_hex(&evm::LAUNCHPAD_PRECOMPILE),
            "0x0000000000000000000000000000000000001017"
        );
        assert!(Surface::Launchpad.decodes(&[0; 32]));
        assert!(Surface::Launchpad.decodes(&[0xff; 32]));
        for length in [0, 1, 31, 33, 64] {
            assert!(!Surface::Launchpad.decodes(&vec![0; length]));
        }

        let launchpad_calls = Cell::new(0);
        let pools = format!("0x{}", "00".repeat(31) + "03");
        let node = |answer: &'static str| {
            let launchpad_calls = &launchpad_calls;
            let pools = pools.clone();
            move |request: &Value| -> Result<Value, &'static str> {
                let result = match request["method"].as_str() {
                    Some("eth_blockNumber") => json!("0x16c1321"),
                    Some("eth_call") => {
                        assert_eq!(request["params"][1], json!("0x16c1321"));
                        if request["params"][0]["to"]
                            == json!("0x0000000000000000000000000000000000001017")
                        {
                            launchpad_calls.set(launchpad_calls.get() + 1);
                            assert_eq!(request["params"][0]["data"], json!("0x8eec5d70"));
                            match answer {
                                "pools" => json!(pools),
                                other => json!(other),
                            }
                        } else {
                            json!(word(0))
                        }
                    }
                    other => panic!("unexpected probe method {other:?}"),
                };
                Ok(json!({"jsonrpc": "2.0", "id": request["id"], "result": result}))
            }
        };

        let live = probe(node("pools"), 1_790_000_045).unwrap_or_else(|code| panic!("{code}"));
        assert_eq!(launchpad_calls.get(), 1);
        assert!(live.launchpad);
        assert_eq!(live.document()["launchpad"], json!(true));
        assert_eq!(live.rpc_height, 0x16c_1321);

        for absent in ["0x", "0x03"] {
            let probed = probe(node(absent), 1_790_000_060).unwrap_or_else(|code| panic!("{code}"));
            assert!(!probed.launchpad, "{absent}");
        }
        assert_eq!(launchpad_calls.get(), 3);
    }

    #[test]
    fn pay_and_programs_surfaces_probe_their_served_views() {
        let pay_address = "0x000000000000000000000000000000000000101b";
        let programs_address = "0x000000000000000000000000000000000000101d";
        assert_eq!(Surface::Pay.name(), "pay");
        assert_eq!(Surface::Programs.name(), "programs");
        assert_eq!(evm::address_hex(&Surface::Pay.address()), pay_address);
        assert_eq!(
            evm::address_hex(&Surface::Programs.address()),
            programs_address
        );
        assert_eq!(Surface::ALL.len(), 5);
        assert!(Surface::ALL.contains(&Surface::Pay));
        assert!(Surface::ALL.contains(&Surface::Programs));

        let mut pay = vec![0; 32];
        pay[16..].fill(0xff);
        let mut programs = vec![0; 9 * 32];
        programs[28..32].fill(0xff);
        for index in 1..8 {
            programs[index * 32 + 24..(index + 1) * 32].fill(0xff);
        }
        programs[8 * 32..].fill(0xff);
        let probe_answers = |pay_answer: Value, programs_answer: Value| {
            let mut visited = Vec::new();
            let result = probe(
                |request| {
                    if request["method"] == "eth_blockNumber" {
                        return Ok(json!({"result": "0x123"}));
                    }
                    assert_eq!(request["method"], "eth_call");
                    assert_eq!(request["params"][1], "0x123");
                    let address = request["params"][0]["to"].as_str().expect("probe address");
                    visited.push(address.to_owned());
                    match address {
                        to if to == pay_address => {
                            assert_eq!(
                                request["params"][0]["data"],
                                format!("0xd7778749{}", "00".repeat(32))
                            );
                            Ok(pay_answer.clone())
                        }
                        to if to == programs_address => {
                            assert_eq!(request["params"][0]["data"], "0xbbe2e42d");
                            Ok(programs_answer.clone())
                        }
                        _ => Ok(json!({"result": word(0)})),
                    }
                },
                1_790_000_100,
            )
            .expect("capabilities probe");
            assert_eq!(visited.len(), 5);
            for surface in Surface::ALL {
                let address = evm::address_hex(&surface.address());
                assert_eq!(visited.iter().filter(|to| **to == address).count(), 1);
                assert_eq!(result.document()["addresses"][surface.name()], address);
                assert_eq!(result.document()[surface.name()], result.live(surface));
            }
            assert!(result.exchange && result.bridge && result.launchpad);
            assert_eq!(result.rpc_height, 0x123);
            assert_eq!(result.probed_at, 1_790_000_100);
            result
        };
        let pay_answer = json!({"result": format!("0x{}", crate::hex(&pay))});
        let programs_answer = json!({"result": format!("0x{}", crate::hex(&programs))});
        let live = probe_answers(pay_answer.clone(), programs_answer.clone());
        assert!(live.pay && live.programs);
        assert!(
            probe_answers(
                json!({"result": word(0)}),
                json!({"result": format!("0x{}", "00".repeat(9 * 32))})
            )
            .pay
        );
        assert!(Surface::Programs.decodes(&[0; 9 * 32]));

        for (surface, valid, lengths) in [
            (Surface::Pay, pay, vec![0, 1, 31, 33, 64, 9 * 32]),
            (
                Surface::Programs,
                programs,
                vec![0, 1, 32, 8 * 32, 9 * 32 - 1, 9 * 32 + 1, 10 * 32],
            ),
        ] {
            let mut invalid = lengths
                .into_iter()
                .map(|length| json!({"result": format!("0x{}", "00".repeat(length))}))
                .collect::<Vec<_>>();
            let overflowing = match surface {
                Surface::Pay => vec![(0, 1), (15, 1)],
                Surface::Programs => std::iter::once((27, 1))
                    .chain((1..8).map(|index| (index * 32 + 23, 1)))
                    .collect(),
                _ => unreachable!(),
            };
            for (index, value) in overflowing {
                let mut bytes = valid.clone();
                bytes[index] = value;
                invalid.push(json!({"result": format!("0x{}", crate::hex(&bytes))}));
            }
            invalid.extend([
                json!({"error": {"code": -32000, "message": "execution reverted"}}),
                json!({"result": "0xzz"}),
                json!({"result": crate::hex(&valid)}),
            ]);
            for answer in invalid {
                let result = match surface {
                    Surface::Pay => probe_answers(answer.clone(), programs_answer.clone()),
                    Surface::Programs => probe_answers(pay_answer.clone(), answer.clone()),
                    _ => unreachable!(),
                };
                assert!(
                    !result.live(surface),
                    "{} accepted {answer}",
                    surface.name()
                );
                assert_eq!(result.pay, surface != Surface::Pay);
                assert_eq!(result.programs, surface != Surface::Programs);
            }
        }
    }

    #[test]
    fn capabilities_probe_views_exist_in_the_served_abi_of_each_surface() {
        let root = concat!(env!("CARGO_MANIFEST_DIR"), "/../../../../");
        for surface in Surface::ALL {
            let view = surface.probe_view();
            let path = format!("{root}{}", view.abi);
            let abi: Value = serde_json::from_slice(
                &std::fs::read(&path).unwrap_or_else(|e| panic!("{path}: {e}")),
            )
            .unwrap_or_else(|e| panic!("{path}: {e}"));
            let entries = abi
                .as_array()
                .unwrap_or_else(|| panic!("{path} is not an ABI"));
            let found = entries
                .iter()
                .filter(|entry| entry["type"] == "function")
                .find(|entry| {
                    let inputs = entry["inputs"]
                        .as_array()
                        .unwrap_or_else(|| panic!("{path}: inputs missing"))
                        .iter()
                        .map(|input| input["type"].as_str().expect("input type"))
                        .collect::<Vec<_>>()
                        .join(",");
                    entry["type"] == "function"
                        && entry["name"]
                            .as_str()
                            .map(|name| format!("{name}({inputs})"))
                            == Some(view.signature.to_owned())
                });
            let entry = found.unwrap_or_else(|| {
                panic!(
                    "{} probe {} missing from {path}",
                    surface.name(),
                    view.signature
                )
            });
            assert_eq!(entry["stateMutability"], "view", "{}", view.signature);
            assert_eq!(
                entry["outputs"].as_array().map(|outputs| outputs
                    .iter()
                    .map(|output| output["type"].clone())
                    .collect::<Vec<_>>()),
                Some(
                    view.outputs
                        .iter()
                        .map(|output| json!(output))
                        .collect::<Vec<_>>()
                ),
                "{}",
                view.signature
            );
            let hash = layerx_indexer::abi::keccak(view.signature.as_bytes());
            assert_eq!(view.selector, hash[..4], "{}", view.signature);
            let mut expected = hash[..4].to_vec();
            if surface == Surface::Pay {
                expected.extend_from_slice(&[0; 32]);
            }
            assert_eq!(surface.probe_calldata(), expected, "{}", view.signature);
            let words = view
                .outputs
                .iter()
                .map(|output| match *output {
                    "uint256" | "bool" | "uint128" | "uint32" | "bytes32" => 1,
                    "uint64[7]" => 7,
                    other => panic!("unexpected static output {other}"),
                })
                .sum::<usize>();
            assert_eq!(view.words, words, "{}", view.signature);
        }
    }

    #[test]
    fn capabilities_reverting_probe_marks_only_that_surface_absent() {
        for reverting in Surface::ALL {
            let calls = Cell::new(0);
            let mut node = recorded_node(true, &calls);
            let probed = probe(
                |request: &Value| {
                    if request["params"][0]["to"] == json!(evm::address_hex(&reverting.address())) {
                        return Ok(json!({
                            "jsonrpc": "2.0",
                            "id": request["id"],
                            "error": {"code": 3, "message": "execution reverted", "data": "0x"}
                        }));
                    }
                    node(request)
                },
                1_790_000_090,
            )
            .unwrap_or_else(|code| panic!("{code}"));
            for surface in Surface::ALL {
                assert_eq!(
                    probed.live(surface),
                    surface != reverting,
                    "{}",
                    surface.name()
                );
            }
            let params = submission(&eip1559(&reverting.address()));
            let cache = Cache::new(Duration::from_secs(15));
            let refused = gate_with(
                &cache,
                Instant::now(),
                || Ok(probed.clone()),
                &json!(1),
                "eth_sendRawTransaction",
                Some(&params),
            )
            .unwrap_or_else(|| panic!("{} write was relayed", reverting.name()));
            assert_eq!(refused["error"]["data"]["surface"], reverting.name());
            for surface in Surface::ALL
                .into_iter()
                .filter(|surface| *surface != reverting)
            {
                assert_eq!(
                    gate_with(
                        &cache,
                        Instant::now(),
                        || -> Result<Capabilities, &'static str> {
                            panic!("the cached probe was not reused")
                        },
                        &json!(1),
                        "eth_sendRawTransaction",
                        Some(&submission(&eip1559(&surface.address()))),
                    ),
                    None,
                    "{}",
                    surface.name()
                );
            }
        }
    }

    #[test]
    fn capabilities_chain_rpc_defaults_a_router_root_to_rpc_exactly_once() {
        for (configured, path) in [
            ("https://router.example", "/rpc"),
            ("https://router.example/", "/rpc"),
            ("https://router.example/rpc", "/rpc"),
            ("https://router.example/rpc/", "/rpc"),
            ("https://router.example:8545/evm", "/evm"),
        ] {
            let endpoint = Endpoint::parse(configured).unwrap_or_else(|e| panic!("{e}"));
            let port = endpoint.port;
            let once = chain_rpc(endpoint);
            assert_eq!(once.base_path, path, "{configured}");
            assert_eq!(once.host, "router.example");
            assert_eq!(once.port, port);
            assert_eq!(chain_rpc(once).base_path, path, "{configured} twice");
        }
    }

    #[test]
    fn capabilities_cache_answers_within_the_ttl_and_reprobes_after_it() {
        let cache = Cache::new(Duration::from_secs(15));
        let start = Instant::now();
        let probes = Cell::new(0);
        let probe_as = |answer: Capabilities| {
            let probes = &probes;
            move || {
                probes.set(probes.get() + 1);
                Ok(answer)
            }
        };
        assert_eq!(
            cache.current(start, probe_as(capabilities(false, false, false))),
            Ok(capabilities(false, false, false))
        );
        assert_eq!(
            cache.current(
                start + Duration::from_secs(14),
                probe_as(capabilities(true, true, true))
            ),
            Ok(capabilities(false, false, false))
        );
        assert_eq!(probes.get(), 1);
        assert_eq!(
            cache.current(
                start + Duration::from_secs(15),
                probe_as(capabilities(true, true, true))
            ),
            Ok(capabilities(true, true, true))
        );
        assert_eq!(probes.get(), 2);
        assert_eq!(
            cache.current(start + Duration::from_secs(40), || Err(
                "paxeer_unreachable"
            )),
            Err("paxeer_unreachable")
        );
        assert_eq!(
            cache.current(
                start + Duration::from_secs(41),
                probe_as(capabilities(false, true, false))
            ),
            Ok(capabilities(false, true, false))
        );
        assert_eq!(probes.get(), 3);
    }

    #[test]
    fn capabilities_refuse_surface_writes_with_the_typed_error_until_code_exists() {
        for (surface, address) in [
            (
                Surface::Exchange,
                "0x0000000000000000000000000000000000001015",
            ),
            (
                Surface::Bridge,
                "0x0000000000000000000000000000000000001016",
            ),
            (
                Surface::Launchpad,
                "0x0000000000000000000000000000000000001017",
            ),
        ] {
            assert_eq!(evm::address_hex(&surface.address()), address);
        }
        let id = json!(9);
        for surface in Surface::ALL {
            for raw in [eip1559(&surface.address()), legacy(&surface.address())] {
                assert_eq!(transaction_target(&raw), Some(surface.address()));
                let params = submission(&raw);
                let cache = Cache::new(Duration::from_secs(15));
                let refused = gate_with(
                    &cache,
                    Instant::now(),
                    || Ok(capabilities(false, false, false)),
                    &id,
                    "eth_sendRawTransaction",
                    Some(&params),
                )
                .unwrap_or_else(|| panic!("{} write was relayed", surface.name()));
                assert_eq!(refused["id"], id);
                assert_eq!(refused["error"]["code"], json!(SURFACE_UNAVAILABLE));
                assert_eq!(refused["error"]["message"], "Surface unavailable");
                assert_eq!(
                    refused["error"]["data"],
                    json!({
                        "code": "surface_unavailable",
                        "surface": surface.name(),
                        "address": evm::address_hex(&surface.address())
                    })
                );
                assert!(refused.get("result").is_none());
                let mut live = capabilities(false, false, false);
                match surface {
                    Surface::Exchange => live.exchange = true,
                    Surface::Bridge => live.bridge = true,
                    Surface::Launchpad => live.launchpad = true,
                    Surface::Pay => live.pay = true,
                    Surface::Programs => live.programs = true,
                }
                let fresh = Cache::new(Duration::from_secs(15));
                assert_eq!(
                    gate_with(
                        &fresh,
                        Instant::now(),
                        || Ok(live),
                        &id,
                        "eth_sendRawTransactionSync",
                        Some(&params)
                    ),
                    None
                );
            }
        }
        let unavailable = gate_with(
            &Cache::new(Duration::from_secs(15)),
            Instant::now(),
            || Err("paxeer_unreachable"),
            &id,
            "eth_sendRawTransaction",
            Some(&submission(&eip1559(&evm::BRIDGE_PRECOMPILE))),
        )
        .unwrap_or_else(|| panic!("an unprobed bridge write was relayed"));
        assert_eq!(unavailable["error"]["code"], json!(-32001));
        assert_eq!(unavailable["error"]["data"]["code"], "paxeer_unreachable");

        let untouched = || -> Result<Capabilities, &'static str> {
            panic!("a request outside the fork surfaces consulted the capabilities")
        };
        let cache = Cache::new(Duration::from_secs(15));
        let other = [0x42_u8; 20];
        assert_eq!(transaction_target(&eip1559(&other)), Some(other));
        assert_eq!(transaction_target(&eip1559(&[])), None);
        let calldata =
            json!([{"to": evm::address_hex(&evm::EXCHANGE_PRECOMPILE), "data": "0x"}, "latest"]);
        for (method, params) in [
            ("eth_sendRawTransaction", submission(&eip1559(&other))),
            ("eth_sendRawTransaction", submission(&legacy(&[]))),
            ("eth_sendRawTransaction", json!(["0x02zz"])),
            ("eth_sendRawTransaction", json!([])),
            ("eth_call", calldata.clone()),
            ("eth_estimateGas", calldata),
            (
                "eth_getCode",
                json!([evm::address_hex(&evm::LAUNCHPAD_PRECOMPILE), "latest"]),
            ),
            (
                "px_getHistory",
                json!(["0x0000000000000000000000000000000000001015"]),
            ),
        ] {
            assert_eq!(
                gate_with(
                    &cache,
                    Instant::now(),
                    untouched,
                    &id,
                    method,
                    Some(&params)
                ),
                None,
                "{method} {params}"
            );
        }
    }

    #[test]
    fn capabilities_schema_names_each_probe_view() {
        let schema: Value = serde_json::from_slice(include_bytes!("../openrpc.json"))
            .unwrap_or_else(|e| panic!("{e}"));
        let entry = schema["methods"]
            .as_array()
            .expect("methods missing")
            .iter()
            .find(|method| method["name"] == "px_getCapabilities")
            .expect("px_getCapabilities missing");
        let description = entry["description"].as_str().expect("description missing");
        assert!(description.contains("eth_call"));
        assert!(description.contains("revert"));
        assert!(description.contains("malformed"));
        for surface in Surface::ALL {
            assert!(
                description.contains(surface.probe_view().signature),
                "{} probe view missing",
                surface.name()
            );
            assert!(
                description.contains(&evm::address_hex(&surface.address())),
                "{} probe address missing",
                surface.name()
            );
        }
        let mut pending = vec![entry];
        while let Some(value) = pending.pop() {
            match value {
                Value::Object(fields) => {
                    if let Some(description) = fields.get("description") {
                        let description = description.as_str().expect("description is not text");
                        assert!(!description.contains("eth_getCode"), "{description}");
                    }
                    pending.extend(fields.values());
                }
                Value::Array(values) => pending.extend(values),
                _ => {}
            }
        }
        for surface in [Surface::Exchange, Surface::Bridge, Surface::Launchpad] {
            let property = entry["result"]["schema"]["properties"][surface.name()]["description"]
                .as_str()
                .expect("surface description missing");
            assert!(
                property.contains(surface.probe_view().signature),
                "{property}"
            );
            assert!(property.contains("eth_call"), "{property}");
            assert!(property.contains("revert"), "{property}");
            assert!(property.contains("malformed"), "{property}");
        }
    }

    #[test]
    fn capabilities_method_is_published_in_openrpc() {
        let schema: Value = serde_json::from_slice(include_bytes!("../openrpc.json"))
            .unwrap_or_else(|e| panic!("{e}"));
        let methods = schema["methods"]
            .as_array()
            .unwrap_or_else(|| panic!("methods missing"));
        let entry = methods
            .iter()
            .find(|method| method["name"] == "px_getCapabilities")
            .unwrap_or_else(|| panic!("px_getCapabilities missing"));
        assert_eq!(entry["paramStructure"], "by-position");
        assert_eq!(entry["params"], json!([]));
        let result = &entry["result"]["schema"];
        assert_eq!(result["type"], "object");
        for field in ["exchange", "bridge", "launchpad"] {
            assert_eq!(result["properties"][field]["type"], "boolean", "{field}");
        }
        assert_eq!(result["properties"]["probed_at"]["type"], "integer");
        assert_eq!(result["properties"]["rpc_height"]["type"], "string");
        assert_eq!(
            result["required"],
            json!(["exchange", "bridge", "launchpad", "probed_at", "rpc_height"])
        );
        assert!(entry["errors"]
            .as_array()
            .unwrap_or_else(|| panic!("errors missing"))
            .iter()
            .any(|error| error["code"] == json!(-32001)));
        let description = entry["description"].as_str().unwrap_or_default();
        assert!(description.contains("surface_unavailable"));
        assert!(description.contains(&SURFACE_UNAVAILABLE.to_string()));
        for surface in Surface::ALL {
            assert!(description.contains(&evm::address_hex(&surface.address())));
        }
    }
}
