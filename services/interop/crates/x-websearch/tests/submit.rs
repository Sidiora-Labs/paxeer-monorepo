use std::error::Error;
use std::io::{Read as _, Write as _};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex, PoisonError};
use std::thread;

use k256::ecdsa::SigningKey;
use serde_json::{json, Value};
use x_websearch::attest::{signer_address, AttestorSet, Ready};
use x_websearch::submit::{
    attestor_set, fulfil_calldata, fulfil_gas_limit, fulfil_program_calldata,
    fulfil_program_gas_limit, request_status, selector, Fees, JournalState, Outcome, SubmitError,
    Submitter, Transaction, FULFIL_PROGRAM_SIGNATURE, FULFIL_SIGNATURE, STATUS_FULFILLED,
};
use x_websearch::watch::{
    hex0x, kernel_network_id, unhex0x, EvmError, EvmRpc, GET_KERNEL_NETWORK_ID_SIGNATURE,
    XWEB_PRECOMPILE,
};

type Checked<T = ()> = Result<T, Box<dyn Error>>;

const CHAIN_ID: u64 = 713_714;

/// The signatures of the test attestors three, one and two over the
/// `evm-fetch` digest: ascending by signer address.
const SIGNATURES: [&str; 3] = [
    "0x7ed23e8fa77c59172964767ddddf8c7d96de1f421479572a0a87e313962b52970ed8483730dabee36f7baf748532bff032f8d8b0a3f782244a93883a8a81f2c71c",
    "0x93854ca688e858b2a183944e5c6c9aa339bbbe46d792e3f46e042fd79aa641a25948de84fb0aea7c4fd5c05e14fa26659deb82928af88b3d7e9761f63a65152d1b",
    "0xd94dba30b84f2e593ffef9a4a943b93ad24556694ebb1f34b3bc2fa1cea096837d63a7f1445e3502162be14fbb93669ae53f549a86f4fd231528a3f98f4938d71c",
];

fn fail(message: impl Into<String>) -> Box<dyn Error> {
    message.into().into()
}

fn fixtures() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures/evm")
}

fn read_json(path: &Path) -> Checked<Value> {
    Ok(serde_json::from_str(&std::fs::read_to_string(path)?)?)
}

fn text<'a>(value: &'a Value, pointer: &str) -> Checked<&'a str> {
    value
        .pointer(pointer)
        .and_then(Value::as_str)
        .ok_or_else(|| fail(format!("missing {pointer}")))
}

fn bytes(text: &str) -> Checked<Vec<u8>> {
    unhex0x(text).ok_or_else(|| fail(format!("{text} is not hexadecimal")))
}

fn secp(first: u8, last: u8) -> Checked<SigningKey> {
    let mut secret = [0_u8; 32];
    secret[0] = first;
    secret[31] = last;
    Ok(SigningKey::from_slice(&secret)?)
}

fn submitter_key() -> Checked<SigningKey> {
    secp(0x5a, 1)
}

/// The fulfil for the `evm-fetch` vector's request with all three
/// signatures.
fn ready() -> Checked<Ready> {
    let signers = [3, 1, 2]
        .into_iter()
        .map(|index| secp(0x3c, index).map(|key| signer_address(&key)))
        .collect::<Checked<Vec<_>>>()?;
    let signatures = SIGNATURES
        .iter()
        .map(|signature| {
            bytes(signature)?
                .try_into()
                .map_err(|_| fail("signature is not 65 bytes"))
        })
        .collect::<Checked<Vec<[u8; 65]>>>()?;
    Ok(Ready {
        request_id: 7,
        response: b"Paxeer X Network".to_vec(),
        content_digest: [0x22; 32],
        full_length: 16,
        callback_gas: 200_000,
        digest: bytes("0x21e2a70f243f13f7e2b72d14cc842c5f6b1779b532144b01c1d61d47b886a0be")?
            .try_into()
            .map_err(|_| fail("digest"))?,
        signers,
        signatures,
    })
}

struct Scratch(PathBuf);

impl Scratch {
    fn new(name: &str) -> Checked<Self> {
        let path =
            std::env::temp_dir().join(format!("x-websearch-submit-{}-{name}", std::process::id()));
        let _ = std::fs::remove_dir_all(&path);
        std::fs::create_dir_all(&path)?;
        Ok(Self(path))
    }

    fn journal(&self) -> PathBuf {
        self.0.join("submit")
    }
}

impl Drop for Scratch {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

/// One recorded EVM answer.
struct Rule {
    method: String,
    params: Value,
    answer: Value,
    once: bool,
    used: bool,
}

fn load_rules(names: &[&str]) -> Checked<Vec<Rule>> {
    let mut rules = Vec::new();
    for name in names {
        let recording = read_json(&fixtures().join(name))?;
        let list = recording
            .pointer("/endpoints/evm/*")
            .and_then(Value::as_array)
            .ok_or_else(|| fail(format!("{name} has no evm rules")))?;
        for rule in list {
            let answer = rule
                .get("result")
                .map(|result| json!({ "result": result }))
                .or_else(|| rule.get("error").map(|error| json!({ "error": error })))
                .ok_or_else(|| fail("rule without an answer"))?;
            rules.push(Rule {
                method: text(rule, "/method")?.to_owned(),
                params: rule.get("params").cloned().unwrap_or(Value::Null),
                answer,
                once: rule.get("once").and_then(Value::as_bool).unwrap_or(false),
                used: false,
            });
        }
    }
    Ok(rules)
}

fn read_http(stream: &mut TcpStream) -> Checked<Vec<u8>> {
    let mut bytes = Vec::new();
    let mut chunk = [0; 4_096];
    loop {
        if let Some(end) = bytes.windows(4).position(|window| window == b"\r\n\r\n") {
            let head = String::from_utf8(bytes[..end].to_vec())?;
            let length = head
                .split("\r\n")
                .find_map(|line| {
                    let (name, value) = line.split_once(':')?;
                    name.eq_ignore_ascii_case("content-length")
                        .then(|| value.trim().parse::<usize>().ok())?
                })
                .unwrap_or(0);
            if bytes.len() >= end + 4 + length {
                return Ok(bytes[end + 4..end + 4 + length].to_vec());
            }
        }
        let read = stream.read(&mut chunk)?;
        if read == 0 {
            return Err(fail("connection closed early"));
        }
        bytes.extend_from_slice(&chunk[..read]);
    }
}

type Calls = Arc<Mutex<Vec<(String, Value)>>>;

/// Replays a recorded EVM exchange on the loopback interface. A request no
/// recorded rule matches is answered 503. At every broadcast it reads the
/// journal file of request 7 as it is on disk at that moment.
struct Node {
    address: SocketAddr,
    calls: Calls,
    journalled: Arc<Mutex<Vec<Option<String>>>>,
}

struct Replay {
    rules: Mutex<Vec<Rule>>,
    calls: Calls,
    journal: PathBuf,
    journalled: Arc<Mutex<Vec<Option<String>>>>,
}

impl Replay {
    fn reply(&self, request: &Value) -> Option<Value> {
        let method = request.get("method")?.as_str()?;
        let params = request.get("params").cloned().unwrap_or(Value::Null);
        if method == "eth_sendRawTransaction" {
            self.journalled
                .lock()
                .unwrap_or_else(PoisonError::into_inner)
                .push(std::fs::read_to_string(&self.journal).ok());
        }
        let mut rules = self.rules.lock().unwrap_or_else(PoisonError::into_inner);
        let rule = rules
            .iter_mut()
            .find(|rule| !rule.used && rule.method == method && rule.params == params)?;
        rule.used = rule.once;
        let mut reply =
            json!({ "jsonrpc": "2.0", "id": request.get("id").cloned().unwrap_or(json!(1)) });
        if let (Some(reply), Some(answer)) = (reply.as_object_mut(), rule.answer.as_object()) {
            reply.extend(answer.clone());
        }
        Some(reply)
    }

    fn serve(&self, stream: &mut TcpStream) -> Checked {
        let request: Value = serde_json::from_slice(&read_http(stream)?)?;
        self.calls
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
            .push((
                text(&request, "/method")?.to_owned(),
                request.get("params").cloned().unwrap_or(Value::Null),
            ));
        let response = self.reply(&request).map_or_else(
            || "HTTP/1.1 503 Service Unavailable\r\nContent-Length: 0\r\nConnection: close\r\n\r\n".to_owned(),
            |reply| {
                let body = reply.to_string();
                format!(
                    "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                    body.len()
                )
            },
        );
        stream.write_all(response.as_bytes())?;
        Ok(())
    }
}

impl Node {
    fn start(names: &[&str], journal: &Path) -> Checked<Self> {
        Self::start_rules(load_rules(names)?, journal)
    }

    fn start_rules(rules: Vec<Rule>, journal: &Path) -> Checked<Self> {
        Self::start_rules_at(rules, &journal.join("7.json"))
    }

    fn start_rules_at(rules: Vec<Rule>, journal: &Path) -> Checked<Self> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let calls: Calls = Arc::new(Mutex::new(Vec::new()));
        let journalled = Arc::new(Mutex::new(Vec::new()));
        let replay = Replay {
            rules: Mutex::new(rules),
            calls: Arc::clone(&calls),
            journal: journal.to_path_buf(),
            journalled: Arc::clone(&journalled),
        };
        thread::spawn(move || {
            for mut stream in listener.incoming().flatten() {
                let _ = replay.serve(&mut stream);
            }
        });
        Ok(Self {
            address,
            calls,
            journalled,
        })
    }

    fn rpc(&self) -> Checked<EvmRpc> {
        Ok(EvmRpc::new(&format!("http://{}/", self.address))?)
    }

    fn params(&self, method: &str) -> Vec<Value> {
        self.calls
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
            .iter()
            .filter(|(name, _)| name == method)
            .map(|(_, params)| params.clone())
            .collect()
    }

    fn calls(&self, method: &str) -> usize {
        self.params(method).len()
    }

    fn journalled(&self) -> Vec<Option<String>> {
        self.journalled
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
            .clone()
    }
}

fn recorded() -> Checked<Value> {
    read_json(&fixtures().join("fulfil.json"))
}

fn recorded_hash() -> Checked<[u8; 32]> {
    bytes(text(&recorded()?, "/hash")?)?
        .try_into()
        .map_err(|_| fail("hash is not 32 bytes"))
}

#[test]
fn the_kernel_network_id_is_read_from_the_xweb_precompile() -> Checked {
    let scratch = Scratch::new("kernel-network")?;
    let node = Node::start(&["kernel-network.json"], &scratch.journal())?;
    assert_eq!(GET_KERNEL_NETWORK_ID_SIGNATURE, "getKernelNetworkId()");
    assert_eq!(
        selector(GET_KERNEL_NETWORK_ID_SIGNATURE),
        [0xfc, 0x0d, 0x26, 0xf4]
    );
    let params = json!([{
        "to": "0x0000000000000000000000000000000000001019",
        "data": "0xfc0d26f4"
    }, "latest"]);
    assert_eq!(kernel_network_id(&node.rpc()?)?, 1);
    assert_eq!(node.params("eth_call"), vec![params.clone()]);

    let mut cases = Vec::new();
    for network in [0_u32, 1, 7, 0x01020304, u32::MAX] {
        let mut word = vec![0; 32];
        word[28..].copy_from_slice(&network.to_be_bytes());
        cases.push((word, Ok(network)));
    }
    for length in [0, 4, 31, 33, 64] {
        cases.push((vec![0; length], Err(EvmError::Malformed)));
    }
    for index in [0, 23, 27] {
        let mut word = vec![0; 32];
        word[index] = 1;
        cases.push((word, Err(EvmError::Malformed)));
    }
    for (word, expected) in cases {
        let mut rules = load_rules(&["kernel-network.json"])?;
        let rule = rules
            .iter_mut()
            .find(|rule| rule.method == "eth_call" && rule.params == params)
            .ok_or_else(|| fail("kernel network recording is missing"))?;
        rule.answer = json!({ "result": hex0x(&word) });
        let node = Node::start_rules(rules, &scratch.journal())?;
        assert_eq!(
            kernel_network_id(&node.rpc()?),
            expected,
            "word {}",
            hex0x(&word)
        );
        assert_eq!(node.params("eth_call"), vec![params.clone()]);
    }
    Ok(())
}

#[test]
fn fulfil_calldata_and_the_signed_transaction_match_an_independent_encoder() -> Checked {
    let recorded = recorded()?;
    let ready = ready()?;
    let calldata = fulfil_calldata(&ready);
    assert_eq!(selector(FULFIL_SIGNATURE), [0xae, 0xb8, 0xa7, 0xec]);
    assert_eq!(hex0x(&calldata), text(&recorded, "/calldata")?);
    let gas_limit = fulfil_gas_limit(&calldata, ready.signatures.len(), ready.callback_gas);
    assert_eq!(
        Some(gas_limit),
        recorded.get("gas_limit").and_then(Value::as_u64)
    );
    let transaction = Transaction {
        chain_id: CHAIN_ID,
        nonce: 3,
        fees: Fees {
            max_fee_per_gas: text(&recorded, "/max_fee_per_gas")?.parse()?,
            max_priority_fee_per_gas: text(&recorded, "/max_priority_fee_per_gas")?.parse()?,
        },
        gas_limit,
        to: XWEB_PRECOMPILE,
        data: calldata,
    };
    let signed = transaction.sign(&submitter_key()?)?;
    assert_eq!(hex0x(&signed.raw), text(&recorded, "/raw")?);
    assert_eq!(signed.hash, recorded_hash()?);

    let inverted = Transaction {
        fees: Fees {
            max_fee_per_gas: 1,
            max_priority_fee_per_gas: 2,
        },
        ..transaction.clone()
    };
    assert_eq!(inverted.sign(&submitter_key()?), Err(SubmitError::Encode));
    let unchained = Transaction {
        chain_id: 0,
        ..transaction
    };
    assert_eq!(unchained.sign(&submitter_key()?), Err(SubmitError::Encode));
    Ok(())
}

#[test]
fn the_attestor_set_and_request_status_are_read_from_the_precompile() -> Checked {
    let scratch = Scratch::new("views")?;
    let node = Node::start(
        &["already-fulfilled.json", "chain.json"],
        &scratch.journal(),
    )?;
    let set = attestor_set(&node.rpc()?)?;
    let expected = [1, 2, 3]
        .into_iter()
        .map(|index| secp(0x3c, index).map(|key| signer_address(&key)))
        .collect::<Checked<Vec<_>>>()?;
    assert_eq!(
        set,
        AttestorSet {
            signers: expected,
            threshold: 2
        }
    );
    assert_eq!(request_status(&node.rpc()?, 7)?, STATUS_FULFILLED);
    Ok(())
}

#[test]
fn a_fulfil_is_journalled_before_broadcast_and_confirmed_from_its_receipt() -> Checked {
    let scratch = Scratch::new("submit")?;
    let node = Node::start(&["submit.json", "chain.json"], &scratch.journal())?;
    let submitter = Submitter::open(node.rpc()?, submitter_key()?, CHAIN_ID, &scratch.journal())?;
    assert_eq!(
        hex0x(&submitter.address()),
        "0xa3e570d1d3ad527d6c518c53be6c6293cddeee01"
    );
    let hash = recorded_hash()?;
    assert_eq!(submitter.submit(&ready()?)?, Outcome::Sent { hash });

    let raw = text(&recorded()?, "/raw")?.to_owned();
    assert_eq!(node.params("eth_sendRawTransaction"), vec![json!([&raw])]);
    let journalled = node.journalled();
    assert_eq!(journalled.len(), 1);
    let on_disk: Value = serde_json::from_str(
        journalled[0]
            .as_deref()
            .ok_or_else(|| fail("no journal entry on disk at broadcast"))?,
    )?;
    assert_eq!(text(&on_disk, "/state")?, "signed");
    assert_eq!(text(&on_disk, "/raw")?, raw);
    assert_eq!(on_disk.get("nonce"), Some(&json!(3)));

    let entry = submitter
        .journal()
        .load(7)?
        .ok_or_else(|| fail("no journal entry"))?;
    assert_eq!(entry.state, JournalState::Signed);

    assert_eq!(submitter.confirm(7)?, Some(Outcome::Fulfilled));
    let entry = submitter
        .journal()
        .load(7)?
        .ok_or_else(|| fail("no journal entry"))?;
    assert_eq!(entry.state, JournalState::Fulfilled);
    assert!(entry.state.completed());

    // A settled request is answered from the journal alone.
    assert_eq!(submitter.submit(&ready()?)?, Outcome::Fulfilled);
    assert_eq!(node.calls("eth_sendRawTransaction"), 1);
    assert_eq!(node.calls("eth_getTransactionCount"), 1);
    assert_eq!(submitter.confirm(8)?, None);
    Ok(())
}

#[test]
fn a_restart_rebroadcasts_the_journalled_bytes_without_signing_again() -> Checked {
    let scratch = Scratch::new("restart")?;
    let node = Node::start(&["submit.json", "chain.json"], &scratch.journal())?;
    let hash = recorded_hash()?;
    {
        let submitter =
            Submitter::open(node.rpc()?, submitter_key()?, CHAIN_ID, &scratch.journal())?;
        assert_eq!(submitter.submit(&ready()?)?, Outcome::Sent { hash });
    }
    let restarted = Submitter::open(node.rpc()?, submitter_key()?, CHAIN_ID, &scratch.journal())?;
    let resumed = restarted.resume()?;
    assert_eq!(resumed, vec![(7, Ok(Outcome::Sent { hash }))]);
    let broadcasts = node.params("eth_sendRawTransaction");
    assert_eq!(broadcasts.len(), 2);
    assert_eq!(broadcasts[0], broadcasts[1]);
    assert_eq!(node.calls("eth_getTransactionCount"), 1);
    assert_eq!(node.calls("eth_maxPriorityFeePerGas"), 1);

    // A submit after the restart rebroadcasts too, and the receipt settles it.
    assert_eq!(restarted.submit(&ready()?)?, Outcome::Sent { hash });
    assert_eq!(node.calls("eth_sendRawTransaction"), 3);
    assert_eq!(restarted.confirm(7)?, Some(Outcome::Fulfilled));
    assert!(restarted.resume()?.is_empty());
    assert_eq!(node.calls("eth_getTransactionCount"), 1);
    Ok(())
}

#[test]
fn an_already_fulfilled_request_is_recorded_as_completed_without_a_transaction() -> Checked {
    let scratch = Scratch::new("already")?;
    let node = Node::start(
        &["already-fulfilled.json", "chain.json"],
        &scratch.journal(),
    )?;
    let submitter = Submitter::open(node.rpc()?, submitter_key()?, CHAIN_ID, &scratch.journal())?;
    let outcome = submitter.submit(&ready()?)?;
    assert_eq!(outcome, Outcome::AlreadyFulfilled);
    assert!(outcome.completed());
    let on_disk = read_json(&scratch.journal().join("7.json"))?;
    assert_eq!(
        on_disk,
        json!({ "request_id": 7, "state": "already_fulfilled" })
    );
    assert_eq!(node.calls("eth_getTransactionCount"), 0);
    assert_eq!(node.calls("eth_sendRawTransaction"), 0);
    assert_eq!(submitter.confirm(7)?, Some(Outcome::AlreadyFulfilled));
    assert_eq!(submitter.submit(&ready()?)?, Outcome::AlreadyFulfilled);
    assert_eq!(node.calls("eth_call"), 1);
    Ok(())
}

#[test]
fn a_refused_broadcast_for_a_request_fulfilled_meanwhile_is_completed() -> Checked {
    let scratch = Scratch::new("lost")?;
    let node = Node::start(&["lost-race.json", "chain.json"], &scratch.journal())?;
    let submitter = Submitter::open(node.rpc()?, submitter_key()?, CHAIN_ID, &scratch.journal())?;
    assert_eq!(submitter.submit(&ready()?)?, Outcome::AlreadyFulfilled);
    assert_eq!(node.calls("eth_sendRawTransaction"), 1);
    let entry = submitter
        .journal()
        .load(7)?
        .ok_or_else(|| fail("no journal entry"))?;
    assert_eq!(entry.state, JournalState::AlreadyFulfilled);
    assert_eq!(entry.transaction, None);
    Ok(())
}

#[test]
fn signatures_out_of_ascending_signer_order_are_refused_before_any_call() -> Checked {
    let scratch = Scratch::new("order")?;
    let node = Node::start(&["submit.json", "chain.json"], &scratch.journal())?;
    let submitter = Submitter::open(node.rpc()?, submitter_key()?, CHAIN_ID, &scratch.journal())?;
    let mut unordered = ready()?;
    unordered.signers.swap(0, 1);
    unordered.signatures.swap(0, 1);
    assert_eq!(submitter.submit(&unordered), Err(SubmitError::Encode));
    let mut repeated = ready()?;
    repeated.signers[1] = repeated.signers[0];
    assert_eq!(submitter.submit(&repeated), Err(SubmitError::Encode));
    assert!(node.params("eth_call").is_empty());
    assert!(!scratch.journal().join("7.json").exists());
    Ok(())
}

// Recorded with eth-abi 6.0.0 and eth-account 0.14.0: encode(["bytes"], [observation])
// and Account.sign_transaction(type=2, chainId=713714, to=0x1019).
const PROGRAM_RECORDINGS: &str = r#"[
  {
    "program_id": "0x7171717171717171717171717171717171717171717171717171717171717171",
    "request_id": 7,
    "nonce": 3,
    "observation": "0x020000000000000000000000000000000000000000000000000000000000000001717171717171717171717171717171717171717171717171717171717171717100000000000000070178e2b53bf00e401996720cd3200858d74dcea9e2e85fd8902c2390f16888dfa5222222222222222222222222222222222222222222222222222222222222222200000010000000105061786565722058204e6574776f726b02fffa7b9297830882d729170c9ecbb377be79200a4c890700530a13c0fe2ae74a1e69846fd3edc1110f2ef722f1d2293d11916ebccb6a4fc3a39517f2a9dd42731bcc3b1d9feadce303b137ff92e429650cec6ab1867c0179f6ba9e33237dc0880b52bca335be388fde7add0ca9e1c725e3e69332a9d8ce0ad5fa5a14d57ccc40d41b",
    "calldata": "0x2d4bf7ad00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000125020000000000000000000000000000000000000000000000000000000000000001717171717171717171717171717171717171717171717171717171717171717100000000000000070178e2b53bf00e401996720cd3200858d74dcea9e2e85fd8902c2390f16888dfa5222222222222222222222222222222222222222222222222222222222222222200000010000000105061786565722058204e6574776f726b02fffa7b9297830882d729170c9ecbb377be79200a4c890700530a13c0fe2ae74a1e69846fd3edc1110f2ef722f1d2293d11916ebccb6a4fc3a39517f2a9dd42731bcc3b1d9feadce303b137ff92e429650cec6ab1867c0179f6ba9e33237dc0880b52bca335be388fde7add0ca9e1c725e3e69332a9d8ce0ad5fa5a14d57ccc40d41b000000000000000000000000000000000000000000000000000000",
    "gas_limit": 79352,
    "raw": "0x02f901f5830ae3f203843b9aca008504e3b29200830135f894000000000000000000000000000000000000101980b901842d4bf7ad00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000125020000000000000000000000000000000000000000000000000000000000000001717171717171717171717171717171717171717171717171717171717171717100000000000000070178e2b53bf00e401996720cd3200858d74dcea9e2e85fd8902c2390f16888dfa5222222222222222222222222222222222222222222222222222222222222222200000010000000105061786565722058204e6574776f726b02fffa7b9297830882d729170c9ecbb377be79200a4c890700530a13c0fe2ae74a1e69846fd3edc1110f2ef722f1d2293d11916ebccb6a4fc3a39517f2a9dd42731bcc3b1d9feadce303b137ff92e429650cec6ab1867c0179f6ba9e33237dc0880b52bca335be388fde7add0ca9e1c725e3e69332a9d8ce0ad5fa5a14d57ccc40d41b000000000000000000000000000000000000000000000000000000c080a05365d87b67f84aaa27a38c3585f1f54e116bd8dae726a59a9ed71c0ef471bac4a051672b7348ef97a3951d0360be932cd316efbea69290a00db8b9853b4216e0d7",
    "hash": "0x7bce3798270291f199a0403bf1539b3bc16101b48fa57d73fba654bc3196b04e"
  },
  {
    "program_id": "0x7272727272727272727272727272727272727272727272727272727272727272",
    "request_id": 7,
    "nonce": 4,
    "observation": "0x020000000000000000000000000000000000000000000000000000000000000001727272727272727272727272727272727272727272727272727272727272727200000000000000070178e2b53bf00e401996720cd3200858d74dcea9e2e85fd8902c2390f16888dfa5222222222222222222222222222222222222222222222222222222222222222200000010000000105061786565722058204e6574776f726b0235b4b7f4fa5e903a1c7571f067aa7e1f99b2d4c954d65dd4f364a0320e8360993e309b8578fded1ce622482f44df4e6fe88899e5eacd879e39c0076355b9875d1c2b965fdf561ebb49fcaa6f37579df3554fdd20f514df42ead94d73ff49a8f0c0215795625ce0518d83f84d3deccc40116759e85fc6d06c6ed4a6c026c6ca31621b",
    "calldata": "0x2d4bf7ad00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000125020000000000000000000000000000000000000000000000000000000000000001727272727272727272727272727272727272727272727272727272727272727200000000000000070178e2b53bf00e401996720cd3200858d74dcea9e2e85fd8902c2390f16888dfa5222222222222222222222222222222222222222222222222222222222222222200000010000000105061786565722058204e6574776f726b0235b4b7f4fa5e903a1c7571f067aa7e1f99b2d4c954d65dd4f364a0320e8360993e309b8578fded1ce622482f44df4e6fe88899e5eacd879e39c0076355b9875d1c2b965fdf561ebb49fcaa6f37579df3554fdd20f514df42ead94d73ff49a8f0c0215795625ce0518d83f84d3deccc40116759e85fc6d06c6ed4a6c026c6ca31621b000000000000000000000000000000000000000000000000000000",
    "gas_limit": 79352,
    "raw": "0x02f901f5830ae3f204843b9aca008504e3b29200830135f894000000000000000000000000000000000000101980b901842d4bf7ad00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000125020000000000000000000000000000000000000000000000000000000000000001727272727272727272727272727272727272727272727272727272727272727200000000000000070178e2b53bf00e401996720cd3200858d74dcea9e2e85fd8902c2390f16888dfa5222222222222222222222222222222222222222222222222222222222222222200000010000000105061786565722058204e6574776f726b0235b4b7f4fa5e903a1c7571f067aa7e1f99b2d4c954d65dd4f364a0320e8360993e309b8578fded1ce622482f44df4e6fe88899e5eacd879e39c0076355b9875d1c2b965fdf561ebb49fcaa6f37579df3554fdd20f514df42ead94d73ff49a8f0c0215795625ce0518d83f84d3deccc40116759e85fc6d06c6ed4a6c026c6ca31621b000000000000000000000000000000000000000000000000000000c080a090390cd456e360d97a754755d4ead820d4660d71b2c05c9c27aa5bbb611511ffa00f476bb9e5ee77205dcba0d2b165d28dfa010a157c7a4958d25e5f8b0ffb400b",
    "hash": "0x6d6a6a7fae98a74900f43037e0fe7e6f350c184e783caa0d7e2a3a2b0dfdd2aa"
  },
  {
    "program_id": "0x7171717171717171717171717171717171717171717171717171717171717171",
    "request_id": 8,
    "nonce": 5,
    "observation": "0x020000000000000000000000000000000000000000000000000000000000000001717171717171717171717171717171717171717171717171717171717171717100000000000000080178e2b53bf00e401996720cd3200858d74dcea9e2e85fd8902c2390f16888dfa5222222222222222222222222222222222222222222222222222222222222222200000010000000105061786565722058204e6574776f726b025ae961f9d1a9496d31d1f99d5c726626eec36c9b235002408dcc581ba2cf67ac169e9d23e43c67bb3df7d86112d1c369a4adba2b049f9444bf0211904162f80e1b795034981efd1985fda48634b75dbe7d40fb09cb4499872ed95e28671a34a7107de66076f8f1d74710b12ae10a9dcaed198c7550a20c11f2bb63917875ecdd2b1b",
    "calldata": "0x2d4bf7ad00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000125020000000000000000000000000000000000000000000000000000000000000001717171717171717171717171717171717171717171717171717171717171717100000000000000080178e2b53bf00e401996720cd3200858d74dcea9e2e85fd8902c2390f16888dfa5222222222222222222222222222222222222222222222222222222222222222200000010000000105061786565722058204e6574776f726b025ae961f9d1a9496d31d1f99d5c726626eec36c9b235002408dcc581ba2cf67ac169e9d23e43c67bb3df7d86112d1c369a4adba2b049f9444bf0211904162f80e1b795034981efd1985fda48634b75dbe7d40fb09cb4499872ed95e28671a34a7107de66076f8f1d74710b12ae10a9dcaed198c7550a20c11f2bb63917875ecdd2b1b000000000000000000000000000000000000000000000000000000",
    "gas_limit": 79352,
    "raw": "0x02f901f5830ae3f205843b9aca008504e3b29200830135f894000000000000000000000000000000000000101980b901842d4bf7ad00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000125020000000000000000000000000000000000000000000000000000000000000001717171717171717171717171717171717171717171717171717171717171717100000000000000080178e2b53bf00e401996720cd3200858d74dcea9e2e85fd8902c2390f16888dfa5222222222222222222222222222222222222222222222222222222222222222200000010000000105061786565722058204e6574776f726b025ae961f9d1a9496d31d1f99d5c726626eec36c9b235002408dcc581ba2cf67ac169e9d23e43c67bb3df7d86112d1c369a4adba2b049f9444bf0211904162f80e1b795034981efd1985fda48634b75dbe7d40fb09cb4499872ed95e28671a34a7107de66076f8f1d74710b12ae10a9dcaed198c7550a20c11f2bb63917875ecdd2b1b000000000000000000000000000000000000000000000000000000c001a0bfb031b06559dff2735e249f4b4e69aca03a7600de8a124f3ce7f551bd881ea3a074f92ddfe27da79f105c235ff2b83736bfb922b8cb291b757d30419a216843c8",
    "hash": "0xd72652f50a6554f21212419a05d7b0d0d448719ca80fd8dc8e3bc8920c71ff31"
  }
]"#;

fn program_recordings() -> Checked<Vec<Value>> {
    Ok(serde_json::from_str(PROGRAM_RECORDINGS)?)
}

fn program_key(record: &Value) -> Checked<x_websearch::program::ProgramKey> {
    Ok((
        bytes(text(record, "/program_id")?)?
            .try_into()
            .map_err(|_| fail("program id is not 32 bytes"))?,
        record["request_id"]
            .as_u64()
            .ok_or_else(|| fail("request id"))?,
    ))
}

fn rule(method: &str, params: Value, result: Value, once: bool) -> Rule {
    Rule {
        method: method.to_owned(),
        params,
        answer: json!({ "result": result }),
        once,
        used: false,
    }
}

#[test]
fn fulfil_program_calldata_matches_an_independent_encoder() -> Checked {
    assert_eq!(FULFIL_PROGRAM_SIGNATURE, "fulfilProgram(bytes)");
    assert_eq!(selector(FULFIL_PROGRAM_SIGNATURE), [0x2d, 0x4b, 0xf7, 0xad]);
    let abi: Value =
        serde_json::from_str(include_str!("../../../../../precompiles/xweb/abi.json"))?;
    let method = abi
        .as_array()
        .and_then(|methods| {
            methods
                .iter()
                .find(|method| method["name"] == "fulfilProgram")
        })
        .ok_or_else(|| fail("fulfilProgram missing from ABI"))?;
    assert_eq!(method["inputs"].as_array().map(Vec::len), Some(1));
    assert_eq!(method["inputs"][0]["type"], "bytes");
    for recorded in program_recordings()? {
        let observation = bytes(text(&recorded, "/observation")?)?;
        let calldata = fulfil_program_calldata(&observation);
        assert_eq!(hex0x(&calldata), text(&recorded, "/calldata")?);
        let gas_limit = fulfil_program_gas_limit(&calldata, 2);
        assert_eq!(Some(gas_limit), recorded["gas_limit"].as_u64());
        assert_eq!(
            gas_limit,
            21_000
                + 16 * calldata.len() as u64
                + 30_000
                + 16 * (calldata.len() as u64 - 4)
                + 2 * 8_000
        );
        assert_eq!(fulfil_gas_limit(&calldata, 2, 200_000), gas_limit + 210_000);
        let transaction = Transaction {
            chain_id: CHAIN_ID,
            nonce: recorded["nonce"].as_u64().ok_or_else(|| fail("nonce"))?,
            fees: Fees {
                max_fee_per_gas: 21_000_000_000,
                max_priority_fee_per_gas: 1_000_000_000,
            },
            gas_limit,
            to: XWEB_PRECOMPILE,
            data: calldata,
        };
        assert_eq!(
            hex0x(&transaction.to),
            "0x0000000000000000000000000000000000001019"
        );
        let signed = transaction.sign(&submitter_key()?)?;
        assert_eq!(signed.raw[0], 2);
        assert_eq!(hex0x(&signed.raw), text(&recorded, "/raw")?);
        assert_eq!(hex0x(&signed.hash), text(&recorded, "/hash")?);
    }
    for length in [0_usize, 1, 31, 32, 33, 64, 65, 4096] {
        let observation = vec![0xa5; length];
        let calldata = fulfil_program_calldata(&observation);
        assert_eq!(calldata.len(), 4 + 64 + length.div_ceil(32) * 32);
        assert_eq!(&calldata[4..35], &[0; 31]);
        assert_eq!(calldata[35], 32);
        assert_eq!(&calldata[36..60], &[0; 24]);
        assert_eq!(&calldata[60..68], &(length as u64).to_be_bytes());
        assert_eq!(&calldata[68..68 + length], observation);
        assert!(calldata[68 + length..].iter().all(|byte| *byte == 0));
    }
    assert_eq!(fulfil_program_gas_limit(&[], usize::MAX), u64::MAX);
    Ok(())
}

#[test]
fn a_program_fulfil_is_journalled_by_program_and_request() -> Checked {
    let scratch = Scratch::new("program-submit")?;
    let records = program_recordings()?;
    let journal = x_websearch::submit::Journal::open(&scratch.journal())?;
    let key = program_key(&records[0])?;
    let path = journal.path(key);
    assert!(path.starts_with(scratch.journal()));
    assert_eq!(journal.path(7), scratch.journal().join("7.json"));
    assert_ne!(path, journal.path(7));
    assert_ne!(path, journal.path(program_key(&records[1])?));
    assert_ne!(path, journal.path(program_key(&records[2])?));
    let web = x_websearch::submit::JournalEntry {
        request_id: 7,
        state: JournalState::AlreadyFulfilled,
        transaction: None,
    };
    journal.store(&web)?;
    let web_on_disk = std::fs::read(journal.path(7))?;
    let mut rules = load_rules(&["chain.json"])?;
    let block_hash = format!("0x{}", "ab".repeat(32));
    for (index, record) in records.iter().enumerate() {
        rules.insert(
            index,
            rule(
                "eth_getTransactionCount",
                json!(["0xa3e570d1d3ad527d6c518c53be6c6293cddeee01", "pending"]),
                json!(format!(
                    "0x{:x}",
                    record["nonce"].as_u64().ok_or_else(|| fail("nonce"))?
                )),
                true,
            ),
        );
        rules.push(rule(
            "eth_sendRawTransaction",
            json!([record["raw"]]),
            record["hash"].clone(),
            false,
        ));
    }
    let receipt_params = json!([records[0]["hash"]]);
    let receipt = json!({"transactionHash": records[0]["hash"], "status": "0x1",
        "blockNumber": "0x21", "blockHash": block_hash});
    rules.push(rule(
        "eth_getTransactionReceipt",
        receipt_params.clone(),
        Value::Null,
        true,
    ));
    rules.push(rule(
        "eth_getTransactionReceipt",
        receipt_params,
        receipt,
        false,
    ));
    rules.push(rule("eth_blockNumber", json!([]), json!("0x22"), true));
    rules.push(rule("eth_blockNumber", json!([]), json!("0x23"), false));
    rules.push(rule(
        "eth_getBlockByNumber",
        json!(["0x21", false]),
        json!({"hash": format!("0x{}", "cd".repeat(32))}),
        true,
    ));
    rules.push(rule(
        "eth_getBlockByNumber",
        json!(["0x21", false]),
        json!({"hash": block_hash}),
        false,
    ));
    rules.push(rule("eth_getTransactionReceipt", json!([records[1]["hash"]]),
        json!({"transactionHash": records[1]["hash"], "status": "0x0", "blockNumber": "0x21", "blockHash": block_hash}), false));
    let node = Node::start_rules_at(rules, &path)?;
    let hash: [u8; 32] = bytes(text(&records[0], "/hash")?)?
        .try_into()
        .map_err(|_| fail("hash"))?;
    let observation = bytes(text(&records[0], "/observation")?)?;
    {
        let mut submitter =
            Submitter::open(node.rpc()?, submitter_key()?, CHAIN_ID, &scratch.journal())?
                .with_confirmations(2);
        assert_eq!(submitter.resume_program(key)?, None);
        let mut mismatched = observation.clone();
        mismatched[33] ^= 1;
        assert_eq!(
            submitter.submit_program(key, &mismatched, 2),
            Err(SubmitError::Encode)
        );
        assert_eq!(
            submitter.submit_program(key, &observation, 1),
            Err(SubmitError::Encode)
        );
        assert_eq!(
            submitter.submit_program(key, &observation[..145], 2),
            Err(SubmitError::Encode)
        );
        assert_eq!(node.calls("eth_getTransactionCount"), 0);
        assert_eq!(
            submitter.submit_program(key, &observation, 2)?,
            Outcome::Sent { hash }
        );
    }
    let on_disk: Value = serde_json::from_str(
        node.journalled()[0]
            .as_deref()
            .ok_or_else(|| fail("not journalled before broadcast"))?,
    )?;
    assert_eq!(on_disk["state"], "signed");
    assert_eq!(on_disk["raw"], records[0]["raw"]);
    assert_eq!(on_disk["hash"], records[0]["hash"]);
    assert_eq!(on_disk["nonce"], records[0]["nonce"]);
    // A different key on restart proves that the stored envelope is reused.
    let mut restarted = Submitter::open(node.rpc()?, secp(0x5a, 2)?, CHAIN_ID, &scratch.journal())?
        .with_confirmations(2);
    assert_eq!(restarted.resume_program(key)?, Some(Outcome::Sent { hash }));
    assert_eq!(
        node.params("eth_sendRawTransaction"),
        vec![json!([records[0]["raw"]]), json!([records[0]["raw"]])]
    );
    assert_eq!(node.journalled()[0], node.journalled()[1]);
    assert_eq!(node.calls("eth_getTransactionCount"), 1);
    assert_eq!(node.calls("eth_maxPriorityFeePerGas"), 1);
    assert_eq!(restarted.resume_program(key)?, Some(Outcome::Sent { hash }));
    assert_eq!(
        journal.load(key)?.ok_or_else(|| fail("journal"))?.state,
        JournalState::Signed
    );
    assert_eq!(restarted.resume_program(key)?, Some(Outcome::Sent { hash }));
    assert_eq!(
        journal.load(key)?.ok_or_else(|| fail("journal"))?.state,
        JournalState::Signed
    );
    assert_eq!(restarted.resume_program(key)?, Some(Outcome::Fulfilled));
    let completed = journal
        .load(key)?
        .ok_or_else(|| fail("completed journal"))?;
    assert_eq!(completed.state, JournalState::Fulfilled);
    assert_eq!(
        hex0x(
            &completed
                .transaction
                .ok_or_else(|| fail("signed transaction"))?
                .1
                .raw
        ),
        records[0]["raw"]
    );
    assert_eq!(
        restarted.submit_program(key, &observation, 2)?,
        Outcome::Fulfilled
    );
    assert_eq!(node.calls("eth_getTransactionReceipt"), 4);
    assert_eq!(node.calls("eth_sendRawTransaction"), 2);
    assert_eq!(std::fs::read(journal.path(7))?, web_on_disk);
    assert_eq!(journal.load(7)?, Some(web));
    assert_eq!(journal.requests()?, vec![7]);
    assert!(restarted.resume()?.is_empty());
    // Program and request components each select a separate entry.
    let mut submitter =
        Submitter::open(node.rpc()?, submitter_key()?, CHAIN_ID, &scratch.journal())?
            .with_confirmations(2);
    for record in &records[1..] {
        let other = program_key(record)?;
        let expected_hash: [u8; 32] = bytes(text(record, "/hash")?)?
            .try_into()
            .map_err(|_| fail("hash"))?;
        assert_eq!(
            submitter.submit_program(other, &bytes(text(record, "/observation")?)?, 2)?,
            Outcome::Sent {
                hash: expected_hash
            }
        );
        let entry = journal
            .load(other)?
            .ok_or_else(|| fail("other program journal"))?;
        assert_eq!(entry.state, JournalState::Signed);
        assert_eq!(
            hex0x(
                &entry
                    .transaction
                    .ok_or_else(|| fail("other transaction"))?
                    .1
                    .raw
            ),
            record["raw"]
        );
    }
    assert_eq!(node.calls("eth_getTransactionCount"), 3);
    assert_eq!(
        submitter.resume_program(program_key(&records[1])?)?,
        Some(Outcome::Reverted)
    );
    assert_eq!(journal.load(program_key(&records[1])?)?, None);
    assert!(journal.load(program_key(&records[2])?)?.is_some());
    assert_eq!(
        journal
            .load(key)?
            .ok_or_else(|| fail("first journal"))?
            .state,
        JournalState::Fulfilled
    );
    assert_eq!(std::fs::read(journal.path(7))?, web_on_disk);
    Ok(())
}
