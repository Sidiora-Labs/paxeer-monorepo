//! Disposable HTTP peer pinned to the Sourcify v2 wire contract used by this crate.
//! The production Client, retry middleware and v2 decoder run unchanged against it.
use serde_json::{json, Value};
use std::{
    io::{Read, Write},
    net::{SocketAddr, TcpListener, TcpStream},
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc,
    },
    thread::{self, JoinHandle},
    time::{Duration, Instant},
};

pub const ADDRESS: &str = "0x831b003398106153ed89a758bec9734667d18aec";
pub const LIBRARY: &str = "0x9224ee604e9b62f8e0a0e5824fee2e0df2ca902f";
pub const PLACEHOLDER: &str = "__$5762d9689e001ee319dd424b89cc702f5c$__";
pub const SOURCE: &str = "pragma solidity ^0.8.20; library Lib { function f() external pure returns (uint) { return 1; } } contract A { function f() external view returns (uint) { return Lib.f(); } }";
pub const FIELDS: &str = "sources,metadata,deployment.transactionHash,compilation,stdJsonOutput,creationBytecode.transformationValues,runtimeBytecode.transformationValues,runtimeBytecode.immutableReferences";
pub const PREFIX: &str = "/sourcify/server";

pub fn submit_route() -> String {
    format!("v2/verify/etherscan/10/{ADDRESS}")
}

pub fn contract_route() -> String {
    format!("v2/contract/10/{ADDRESS}")
}

pub fn immutables() -> Value {
    json!({"2155":[{"length":32,"start":4157},{"length":32,"start":4712}],"2157":[{"length":32,"start":1172},{"length":32,"start":1221},{"length":32,"start":1289},{"length":32,"start":2077},{"length":32,"start":4218},{"length":32,"start":5837}],"2159":[{"length":32,"start":742},{"length":32,"start":4943}],"2161":[{"length":32,"start":402},{"length":32,"start":3247},{"length":32,"start":5564}]})
}

pub fn metadata() -> Value {
    json!({"compiler":{"version":"0.8.20+commit.a1b79de6"},"language":"Solidity","settings":{"compilationTarget":{"contracts/Example.sol":"A"}},"version":1})
}

pub fn contract() -> Value {
    json!({
        "creationMatch": "match", "runtimeMatch": "match", "chainId": "10", "address": ADDRESS,
        "sources": {"contracts/Example.sol": {"content": SOURCE}},
        "metadata": metadata(),
        "deployment": {"transactionHash": "0x1234"},
        "compilation": {"fullyQualifiedName": "contracts/Example.sol:A", "compilerVersion":"0.8.20+commit.a1b79de6"},
        "stdJsonOutput": {"contracts":{"contracts/Example.sol":{"A":{"evm":{
            "bytecode":{"object":format!("60{PLACEHOLDER}00"),"linkReferences":{"contracts/Example.sol":{"Lib":[{"start":1,"length":20}]}}}
        }}}}},
        "creationBytecode": {"transformationValues":{"libraries":{"contracts/Example.sol:Lib":LIBRARY},"constructorArguments":"0x1234"}},
        "runtimeBytecode": {"transformationValues":{"libraries":{"contracts/Example.sol:Lib":LIBRARY}},"immutableReferences":immutables()}
    })
}

pub fn unverified() -> Value {
    json!({"creationMatch":null,"runtimeMatch":null,"chainId":"10","address":ADDRESS})
}

pub fn api_error(code: &str, message: &str) -> Value {
    json!({"customCode": code, "message": message})
}

pub struct Exchange {
    method: &'static str,
    route: String,
    status: u16,
    body: Value,
    header_delay: Duration,
    body_delay: Duration,
}

impl Exchange {
    pub fn new(method: &'static str, route: impl Into<String>, status: u16, body: Value) -> Self {
        Self {
            method,
            route: route.into(),
            status,
            body,
            header_delay: Duration::ZERO,
            body_delay: Duration::ZERO,
        }
    }

    pub fn delayed(mut self, delay: Duration) -> Self {
        self.header_delay = delay;
        self
    }

    pub fn delayed_body(mut self, delay: Duration) -> Self {
        self.body_delay = delay;
        self
    }
}

pub fn verification() -> Vec<Exchange> {
    vec![
        Exchange::new(
            "POST",
            submit_route(),
            202,
            json!({"verificationId":"job-521"}),
        ),
        Exchange::new(
            "GET",
            "v2/verify/job-521",
            200,
            json!({"isJobCompleted":true,"error":null}),
        ),
        Exchange::new("GET", contract_route(), 200, contract()),
    ]
}

pub struct Service {
    address: SocketAddr,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<usize>>,
    expected: usize,
}

impl Service {
    pub fn start(exchanges: Vec<Exchange>) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        listener.set_nonblocking(true).unwrap();
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = stop.clone();
        let expected = exchanges.len();
        let worker = thread::spawn(move || {
            let mut seen = 0;
            while !stopped.load(Ordering::SeqCst) {
                let (mut stream, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("accept: {error}"),
                };
                let exchange = exchanges.get(seen).expect("unexpected extra HTTP request");
                assert_request(&mut stream, exchange);
                seen += 1;
                if !delay(exchange.header_delay, &stopped) {
                    break;
                }
                let body = exchange.body.to_string();
                let header = format!("HTTP/1.1 {} Response\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", exchange.status, body.len());
                if write_response(&mut stream, header.as_bytes())
                    && delay(exchange.body_delay, &stopped)
                {
                    write_response(&mut stream, body.as_bytes());
                }
            }
            seen
        });
        Self {
            address,
            stop,
            worker: Some(worker),
            expected,
        }
    }

    pub fn base_url(&self) -> String {
        format!("http://{}{PREFIX}", self.address)
    }

    pub fn assert_finished(mut self) {
        self.stop.store(true, Ordering::SeqCst);
        let seen = self
            .worker
            .take()
            .unwrap()
            .join()
            .expect("HTTP contract assertion");
        assert_eq!(seen, self.expected, "not all exchanges were exercised");
    }
}

impl Drop for Service {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::SeqCst);
        if let Some(worker) = self.worker.take() {
            let result = worker.join();
            if !thread::panicking() {
                result.expect("HTTP contract assertion");
            }
        }
    }
}

fn delay(duration: Duration, stopped: &AtomicBool) -> bool {
    let until = Instant::now() + duration;
    while Instant::now() < until {
        if stopped.load(Ordering::SeqCst) {
            return false;
        }
        thread::sleep(Duration::from_millis(1));
    }
    !stopped.load(Ordering::SeqCst)
}

fn write_response(stream: &mut TcpStream, bytes: &[u8]) -> bool {
    match stream.write_all(bytes) {
        Ok(()) => true,
        Err(error)
            if matches!(
                error.kind(),
                std::io::ErrorKind::BrokenPipe | std::io::ErrorKind::ConnectionReset
            ) =>
        {
            false
        }
        Err(error) => panic!("write HTTP response: {error}"),
    }
}

fn assert_request(stream: &mut TcpStream, exchange: &Exchange) {
    stream
        .set_read_timeout(Some(Duration::from_secs(3)))
        .unwrap();
    let mut header = Vec::new();
    while !header.ends_with(b"\r\n\r\n") {
        let mut byte = [0];
        stream.read_exact(&mut byte).expect("request header");
        header.push(byte[0]);
        assert!(header.len() < 16_384, "oversized request header");
    }
    let header = String::from_utf8(header).unwrap();
    let mut lines = header.lines();
    let mut request_line = lines.next().unwrap().split_whitespace();
    assert_eq!(request_line.next(), Some(exchange.method));
    let target = request_line.next().unwrap();
    let url = url::Url::parse(&format!("http://localhost{target}")).unwrap();
    assert_eq!(url.path(), format!("{PREFIX}/{}", exchange.route));
    let query: Vec<_> = url.query_pairs().collect();
    if exchange.route.starts_with("v2/contract/") {
        assert_eq!(query, vec![("fields".into(), FIELDS.into())]);
    } else {
        assert!(query.is_empty(), "unexpected query: {query:?}");
    }
    let content_length = lines
        .filter_map(|line| line.split_once(':'))
        .find_map(|(name, value)| {
            name.eq_ignore_ascii_case("content-length")
                .then(|| value.trim().parse::<usize>().unwrap())
        })
        .unwrap_or(0);
    let mut body = vec![0; content_length];
    stream.read_exact(&mut body).unwrap();
    if exchange.method == "POST" {
        assert_eq!(serde_json::from_slice::<Value>(&body).unwrap(), json!({}));
    } else {
        assert!(body.is_empty());
    }
}
