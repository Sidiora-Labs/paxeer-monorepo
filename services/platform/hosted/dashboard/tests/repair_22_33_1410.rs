use layerx_platform_dashboard::{
    gateway::{Store, AUDIT_PAGE_SIZE},
    DashboardError, ReceiptReader, RequestOutcome,
};
use layerx_platform_gateway::{gateway_audit_event, verify_activity_operation, AuthorityFacts};
use layerx_platform_webhooks::{
    encoding::{fixed_hex, hex_decode, hex_encode},
    error::WebhookError,
    events::{
        settled_payment, EventId, PaymentDraft, Principal, ProtocolEvent, SubjectId, Verification,
    },
};
use native_tls::{Certificate, Identity, TlsAcceptor};
use serde_json::Value;
use std::{
    collections::BTreeMap,
    io::{Read, Write},
    net::TcpListener,
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc, Mutex,
    },
    thread::{self, JoinHandle},
    time::Duration,
};
use zeroize::Zeroizing;

const CERTIFICATE: &[u8] =
    include_bytes!("../../../examples/buyer-agent/test/fixtures/localhost-cert.pem");
const KEY: &[u8] = include_bytes!("../../../examples/buyer-agent/test/fixtures/localhost-key.pem");
const RECEIPTS: &str = include_str!("../src/fixtures/repair_22_33_1410_receipts.json");

fn checked<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

fn principal(name: &str) -> Principal {
    checked(Principal::new(name))
}

fn digest(principal: &Principal) -> String {
    hex_encode(&principal.audit_digest())
}

fn audit_key(principal: &Principal) -> String {
    format!("gateway:principal:{}:audit", digest(principal))
}

fn activity_key(principal: &Principal) -> String {
    format!("webhooks:principal:{}:activities", digest(principal))
}

fn payment(principal: &Principal, source: usize, sequence: u64) -> ProtocolEvent {
    let sources: Value = checked(serde_json::from_str(RECEIPTS));
    let source = &sources["receipts"][source];
    let field = |name: &str| {
        source[name]
            .as_str()
            .unwrap_or_else(|| panic!("missing {name}"))
    };
    let public_key = checked(fixed_hex(field("sequencer_public_key")));
    let operation = checked(verify_activity_operation(
        &checked(hex_decode(field("receipt_hex"))),
        AuthorityFacts::new(
            checked(fixed_hex(field("batch_id"))),
            checked(fixed_hex(field("asset"))),
            checked(fixed_hex(field("previous_state_root"))),
            checked(fixed_hex(field("resulting_state_root"))),
            public_key,
        ),
        &public_key,
        Some(checked(fixed_hex(field("activity_id")))),
    ));
    checked(settled_payment(PaymentDraft {
        id: checked(EventId::new(format!("payment-{sequence}"))),
        principal: principal.clone(),
        subject: checked(SubjectId::new("payments")),
        subject_sequence: sequence,
        occurred_at: sequence,
        operation: &operation,
        amount: field("amount").to_owned(),
        asset: field("asset").to_owned(),
    }))
}

fn activity(event: &ProtocolEvent) -> String {
    event
        .facts()
        .iter()
        .find(|fact| fact.name() == "activity_id")
        .unwrap_or_else(|| panic!("payment must have an activity"))
        .value()
        .to_owned()
}

#[derive(Default)]
struct IndexData {
    versions: BTreeMap<String, String>,
    streams: BTreeMap<String, Vec<(u64, String, String)>>,
    hashes: BTreeMap<String, BTreeMap<String, Vec<u8>>>,
    events: BTreeMap<String, Vec<ProtocolEvent>>,
    commands: Vec<Vec<String>>,
    repeat_cursor: bool,
}

impl IndexData {
    fn initialize(&mut self, principal: &Principal) {
        for key in [audit_key(principal), activity_key(principal)] {
            self.versions
                .insert(format!("{key}:version"), "1".to_owned());
        }
    }

    fn audit(&mut self, principal: &Principal, at: u64, outcome: &str) -> String {
        let event = gateway_audit_event(&digest(principal), "request", "payment", outcome, at);
        let record = (at * 1_000, event.clone(), outcome.to_owned());
        self.streams
            .entry("gateway:audit".to_owned())
            .or_default()
            .push(record.clone());
        self.streams
            .entry(audit_key(principal))
            .or_default()
            .push(record);
        event
            .split_once(':')
            .unwrap_or_else(|| panic!("audit commitment"))
            .1
            .to_owned()
    }

    fn publish(&mut self, event: ProtocolEvent) {
        checked(event.validate());
        let key = activity_key(event.principal());
        let id = activity(&event);
        self.hashes
            .entry(key)
            .or_default()
            .insert(id, checked(serde_json::to_vec(&event)));
        self.events
            .entry(event.principal().as_str().to_owned())
            .or_default()
            .push(event);
    }
}

// Only the peer is an in-memory fixture. Requests, TLS, RESP, paging, decoding,
// evidence checks and receipt projection execute the production dashboard code.
struct IndexFixture {
    endpoint: String,
    data: Arc<Mutex<IndexData>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl IndexFixture {
    fn start(data: IndexData) -> Self {
        let listener = checked(TcpListener::bind("127.0.0.1:0"));
        checked(listener.set_nonblocking(true));
        let endpoint = format!(
            "rediss://localhost:{}",
            checked(listener.local_addr()).port()
        );
        let acceptor = checked(TlsAcceptor::new(checked(Identity::from_pkcs8(
            CERTIFICATE,
            KEY,
        ))));
        let data = Arc::new(Mutex::new(data));
        let shared = Arc::clone(&data);
        let stop = Arc::new(AtomicBool::new(false));
        let finished = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            while !finished.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((socket, _)) => {
                        checked(socket.set_read_timeout(Some(Duration::from_secs(5))));
                        checked(socket.set_write_timeout(Some(Duration::from_secs(5))));
                        let mut stream = checked(acceptor.accept(socket));
                        assert_eq!(
                            command(&mut stream),
                            ["AUTH", "dashboard-test", "fixture-password"]
                        );
                        checked(stream.write_all(b"+OK\r\n"));
                        let arguments = command(&mut stream);
                        let response = {
                            let mut data = checked(shared.lock());
                            reply(&mut data, &arguments)
                        };
                        checked(stream.write_all(&response));
                        checked(stream.flush());
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("fixture listener: {error}"),
                }
            }
        });
        Self {
            endpoint,
            data,
            stop,
            worker: Some(worker),
        }
    }

    fn store(&self) -> Store {
        checked(Store::new(
            &self.endpoint,
            checked(Certificate::from_pem(CERTIFICATE)),
            Zeroizing::new("dashboard-test".to_owned()),
            Zeroizing::new("fixture-password".to_owned()),
        ))
    }

    fn reader(&self) -> ReceiptReader {
        ReceiptReader::new(self.store())
    }
}

impl Drop for IndexFixture {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            checked(worker.join());
        }
    }
}

fn line(stream: &mut impl Read) -> String {
    let mut bytes = Vec::new();
    loop {
        let mut byte = [0];
        checked(stream.read_exact(&mut byte));
        bytes.push(byte[0]);
        assert!(bytes.len() < 8192);
        if bytes.ends_with(b"\r\n") {
            bytes.truncate(bytes.len() - 2);
            return checked(String::from_utf8(bytes));
        }
    }
}

fn command(stream: &mut impl Read) -> Vec<String> {
    let header = line(stream);
    let length: usize = checked(
        header
            .strip_prefix('*')
            .unwrap_or_else(|| panic!("RESP array"))
            .parse(),
    );
    assert!(length <= 8);
    (0..length)
        .map(|_| {
            let header = line(stream);
            let length: usize = checked(
                header
                    .strip_prefix('$')
                    .unwrap_or_else(|| panic!("RESP bulk"))
                    .parse(),
            );
            assert!(length <= 8192);
            let mut bytes = vec![0; length];
            checked(stream.read_exact(&mut bytes));
            let mut ending = [0; 2];
            checked(stream.read_exact(&mut ending));
            assert_eq!(ending, *b"\r\n");
            checked(String::from_utf8(bytes))
        })
        .collect()
}

fn bulk(bytes: &[u8]) -> Vec<u8> {
    let mut output = format!("${}\r\n", bytes.len()).into_bytes();
    output.extend_from_slice(bytes);
    output.extend_from_slice(b"\r\n");
    output
}

fn array(values: &[Vec<u8>]) -> Vec<u8> {
    let mut output = format!("*{}\r\n", values.len()).into_bytes();
    for value in values {
        output.extend_from_slice(value);
    }
    output
}

fn reply(data: &mut IndexData, arguments: &[String]) -> Vec<u8> {
    data.commands.push(arguments.to_vec());
    match arguments[0].as_str() {
        "GET" => {
            assert_eq!(arguments.len(), 2);
            data.versions
                .get(&arguments[1])
                .map_or_else(|| b"$-1\r\n".to_vec(), |value| bulk(value.as_bytes()))
        }
        "HGET" => {
            assert_eq!(arguments.len(), 3);
            data.hashes
                .get(&arguments[1])
                .and_then(|hash| hash.get(&arguments[2]))
                .map_or_else(|| b"$-1\r\n".to_vec(), |value| bulk(value))
        }
        "XREVRANGE" => {
            assert_eq!(arguments.len(), 6);
            assert_eq!(arguments[3], "-");
            assert_eq!(arguments[4], "COUNT");
            let count: usize = checked(arguments[5].parse());
            assert!((1..=AUDIT_PAGE_SIZE).contains(&count));
            let before = if arguments[2] == "+" {
                None
            } else {
                let cursor = arguments[2]
                    .strip_prefix('(')
                    .unwrap_or_else(|| panic!("exclusive cursor"));
                Some(checked(
                    cursor
                        .split_once('-')
                        .unwrap_or_else(|| panic!("stream ID"))
                        .0
                        .parse::<u64>(),
                ))
            };
            let entries = data
                .streams
                .get(&arguments[1])
                .into_iter()
                .flatten()
                .rev()
                .filter(|(at, _, _)| before.is_none_or(|before| *at < before || data.repeat_cursor))
                .take(count)
                .map(|(at, event, outcome)| {
                    array(&[
                        bulk(format!("{at}-0").as_bytes()),
                        array(&[
                            bulk(b"event"),
                            bulk(event.as_bytes()),
                            bulk(b"outcome"),
                            bulk(outcome.as_bytes()),
                        ]),
                    ])
                })
                .collect::<Vec<_>>();
            array(&entries)
        }
        other => panic!("unexpected Redis command {other}"),
    }
}

#[test]
fn finding_027_busy_principal_cannot_hide_retained_audit_and_pages_are_bounded() {
    let tenant = principal("tenant:quiet");
    let busy = principal("tenant:busy");
    let mut data = IndexData::default();
    data.initialize(&tenant);
    data.initialize(&busy);
    let mut expected = (1..=140)
        .map(|at| data.audit(&tenant, at, "completed"))
        .collect::<Vec<_>>();
    expected.reverse();
    for at in 141..=2140 {
        data.audit(&busy, at, "pending");
    }
    assert!(data.streams["gateway:audit"]
        .iter()
        .rev()
        .take(1600)
        .all(|(_, event, _)| !event.starts_with(&format!("{}:", digest(&tenant)))));
    let fixture = IndexFixture::start(data);
    let records = checked(fixture.store().requests(&tenant, 140));
    assert_eq!(records.len(), 140);
    assert_eq!(
        records
            .iter()
            .map(|record| record.operation_digest.clone())
            .collect::<Vec<_>>(),
        expected
    );
    assert!(records
        .iter()
        .all(|record| record.outcome == RequestOutcome::Completed
            && record.verification == Verification::Unverified));
    assert_eq!(records.first().map(|record| record.at), Some(140));
    assert_eq!(records.last().map(|record| record.at), Some(1));
    let data = checked(fixture.data.lock());
    let pages = data
        .commands
        .iter()
        .filter(|command| command[0] == "XREVRANGE")
        .collect::<Vec<_>>();
    assert_eq!(pages.len(), 3);
    assert_eq!(pages[0][2], "+");
    assert_eq!(pages[1][2], "(77000-0");
    assert_eq!(pages[2][2], "(13000-0");
    assert_eq!(
        pages
            .iter()
            .map(|command| command[5].as_str())
            .collect::<Vec<_>>(),
        ["64", "64", "12"]
    );
    assert!(pages.iter().all(|command| command[1] == audit_key(&tenant)));
}

#[test]
fn finding_027_cursor_pages_do_not_repeat_and_refuse_foreign_or_nonprogressing_entries() {
    let tenant = principal("tenant:pages");
    let mut data = IndexData::default();
    data.initialize(&tenant);
    for at in 1..=5 {
        data.audit(&tenant, at, "rate_limited");
    }
    let fixture = IndexFixture::start(data);
    let store = fixture.store();
    let first = checked(store.requests_page(&tenant, None, 2));
    assert_eq!(first.next_cursor.as_deref(), Some("4000-0"));
    let second = checked(store.requests_page(&tenant, first.next_cursor.as_deref(), 2));
    assert_eq!(
        second
            .records
            .iter()
            .map(|record| record.at)
            .collect::<Vec<_>>(),
        [3, 2]
    );
    let third = checked(store.requests_page(&tenant, second.next_cursor.as_deref(), 2));
    assert_eq!(third.records.len(), 1);
    assert_eq!(third.records[0].at, 1);
    assert_eq!(third.next_cursor, None);
    assert!(first
        .records
        .iter()
        .all(|record| record.outcome == RequestOutcome::RateLimited));
    checked(fixture.data.lock()).repeat_cursor = true;
    assert!(matches!(
        store.requests_page(&tenant, Some("4000-0"), 2),
        Err(DashboardError::CorruptStore)
    ));
    {
        let mut data = checked(fixture.data.lock());
        data.repeat_cursor = false;
        data.streams
            .get_mut(&audit_key(&tenant))
            .unwrap_or_else(|| panic!("stream"))[4]
            .1 = gateway_audit_event(
            &digest(&principal("tenant:foreign")),
            "request",
            "payment",
            "completed",
            5,
        );
    }
    assert!(matches!(
        store.requests_page(&tenant, None, 2),
        Err(DashboardError::CorruptStore)
    ));
    for cursor in ["-", "+", "01-0", "3--2", "18446744073709551616-0"] {
        assert!(matches!(
            store.requests_page(&tenant, Some(cursor), 2),
            Err(DashboardError::InvalidRequest)
        ));
    }
    for limit in [0, AUDIT_PAGE_SIZE + 1] {
        assert!(matches!(
            store.requests_page(&tenant, None, limit),
            Err(DashboardError::InvalidRequest)
        ));
    }
}

#[test]
fn finding_026_receipt_lookup_survives_more_than_200_later_events() {
    let tenant = principal("tenant:receipts");
    let other = principal("tenant:other");
    let retained = payment(&tenant, 0, 1);
    let id = activity(&retained);
    let mut data = IndexData::default();
    data.initialize(&tenant);
    data.initialize(&other);
    data.publish(retained.clone());
    for sequence in 2..=251 {
        data.publish(payment(&tenant, 1, sequence));
    }
    let history = &data.events[tenant.as_str()];
    assert_eq!(history.len(), 251);
    assert!(history
        .iter()
        .rev()
        .take(200)
        .all(|event| activity(event) != id));
    let fixture = IndexFixture::start(data);
    let reader = fixture.reader();
    let receipt = checked(reader.receipt(&tenant, &id.to_ascii_uppercase()));
    assert_eq!(receipt.activity_id, id);
    assert_eq!(receipt.event, retained.id().as_str());
    assert_eq!(receipt.verification, Verification::ReceiptVerified);
    assert!(receipt.settled);
    assert_eq!(
        receipt.receipt_digest,
        retained.facts()[0]
            .receipt_digest()
            .unwrap_or_else(|| panic!("settlement receipt"))
    );
    assert!(matches!(
        reader.receipt(&other, &id),
        Err(DashboardError::UnknownReceipt)
    ));
    assert!(matches!(
        reader.receipt(&tenant, &"ff".repeat(32)),
        Err(DashboardError::UnknownReceipt)
    ));
    let data = checked(fixture.data.lock());
    assert!(data
        .commands
        .iter()
        .all(|command| matches!(command[0].as_str(), "GET" | "HGET")));
    assert_eq!(
        data.commands
            .iter()
            .filter(|command| command[0] == "HGET")
            .count(),
        3
    );
}

#[test]
fn finding_026_activity_index_preserves_principal_and_receipt_evidence_checks() {
    let tenant = principal("tenant:checks");
    let retained = payment(&tenant, 0, 1);
    let id = activity(&retained);
    let mut data = IndexData::default();
    data.initialize(&tenant);
    data.publish(retained.clone());
    let fixture = IndexFixture::start(data);
    let reader = fixture.reader();
    let original: Value = checked(serde_json::to_value(&retained));
    for case in 0..7 {
        let mut event = original.clone();
        match case {
            0 => event["principal"] = Value::from("tenant:foreign"),
            1 => event["kind"] = Value::from("journey"),
            2 => event["facts"][3]["value"] = Value::from("ff".repeat(32)),
            3 => {
                event["facts"][3]["verification"] = Value::from("unverified");
                event["facts"][3]["receipt_digest"] = Value::Null;
            }
            4 => event["facts"][3]["receipt_digest"] = Value::from("ff".repeat(32)),
            5 => {
                event["facts"][0]["verification"] = Value::from("unverified");
                event["facts"][0]["receipt_digest"] = Value::Null;
            }
            6 => event["verification"] = Value::from("block-committed"),
            _ => unreachable!(),
        }
        checked(fixture.data.lock())
            .hashes
            .get_mut(&activity_key(&tenant))
            .unwrap_or_else(|| panic!("activity index"))
            .insert(id.clone(), checked(serde_json::to_vec(&event)));
        let result = reader.receipt(&tenant, &id);
        if matches!(case, 0 | 6) {
            assert!(
                matches!(
                    result,
                    Err(DashboardError::Webhooks(WebhookError::CorruptStore))
                ),
                "case {case}: {result:?}"
            );
        } else {
            assert!(
                matches!(result, Err(DashboardError::UnknownReceipt)),
                "case {case}: {result:?}"
            );
        }
    }
    checked(fixture.data.lock())
        .hashes
        .get_mut(&activity_key(&tenant))
        .unwrap_or_else(|| panic!("activity index"))
        .insert(id.clone(), b"not JSON".to_vec());
    assert!(matches!(
        reader.receipt(&tenant, &id),
        Err(DashboardError::Webhooks(WebhookError::CorruptStore))
    ));
    for invalid in ["", "aa", &"z".repeat(64)] {
        assert!(matches!(
            reader.receipt(&tenant, invalid),
            Err(DashboardError::InvalidRequest)
        ));
    }
}

#[test]
fn missing_producer_wiring_is_an_error_instead_of_empty_history() {
    let tenant = principal("tenant:uninitialized");
    let fixture = IndexFixture::start(IndexData::default());
    assert!(matches!(
        fixture.store().requests(&tenant, 1),
        Err(DashboardError::CorruptStore)
    ));
    assert!(matches!(
        fixture.reader().receipt(&tenant, &"ab".repeat(32)),
        Err(DashboardError::Webhooks(WebhookError::Unavailable))
    ));
    let mut data = checked(fixture.data.lock());
    data.initialize(&tenant);
    drop(data);
    assert!(checked(fixture.store().requests(&tenant, 1)).is_empty());
    assert!(matches!(
        fixture.reader().receipt(&tenant, &"ab".repeat(32)),
        Err(DashboardError::UnknownReceipt)
    ));
}
