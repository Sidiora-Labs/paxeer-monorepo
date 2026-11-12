use layerx_client::sequencer::SequencerEndpoint;
use serde_json::{json, Value};
use std::io::{Read as _, Write as _};
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};

pub struct Replay {
    pub endpoint: SequencerEndpoint,
    stop: Arc<AtomicBool>,
    worker: Option<std::thread::JoinHandle<usize>>,
}
impl Replay {
    pub fn new(answers: Value) -> Self {
        let listener = std::net::TcpListener::bind("127.0.0.1:0").or_fail("bind RPC");
        listener
            .set_nonblocking(true)
            .or_fail("nonblocking listener");
        let address = listener.local_addr().or_fail("RPC address");
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let worker = std::thread::spawn(move || {
            let mut calls = 0;
            while !stopped.load(Ordering::SeqCst) {
                let mut stream = match listener.accept() {
                    Ok((stream, _)) => stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(std::time::Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("RPC accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(std::time::Duration::from_secs(5)))
                    .or_fail("read deadline");
                let mut headers = Vec::new();
                while !headers.ends_with(b"\r\n\r\n") {
                    let mut byte = [0];
                    stream.read_exact(&mut byte).or_fail("HTTP header");
                    headers.push(byte[0]);
                }
                let headers = String::from_utf8(headers).or_fail("headers");
                let length: usize = headers
                    .lines()
                    .find_map(|line| {
                        let (name, value) = line.split_once(':')?;
                        name.eq_ignore_ascii_case("content-length")
                            .then(|| value.trim().parse().or_fail("length"))
                    })
                    .or_fail("content length");
                let mut body = vec![0; length];
                stream.read_exact(&mut body).or_fail("RPC body");
                let request: Value = serde_json::from_slice(&body).or_fail("RPC JSON");
                let answer = answers
                    .as_array()
                    .or_fail("answers")
                    .iter()
                    .find(|answer| {
                        answer["method"] == request["method"]
                            && answer["params"] == request["params"]
                    })
                    .unwrap_or_else(|| panic!("unrecorded RPC request: {request}"));
                if request["method"] == "eth_call" {
                    calls += 1;
                }
                let body = json!({"jsonrpc":"2.0", "id":request["id"], "result":answer["result"]})
                    .to_string();
                write!(stream,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",body.len(),body).or_fail("RPC reply");
            }
            calls
        });
        Self {
            endpoint: SequencerEndpoint::new(&format!("http://{address}"), &[]).or_fail("endpoint"),
            stop,
            worker: Some(worker),
        }
    }
    pub fn finish(mut self) -> usize {
        self.stop.store(true, Ordering::SeqCst);
        self.worker
            .take()
            .or_fail("worker")
            .join()
            .or_fail("RPC replay")
    }
}
impl Drop for Replay {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::SeqCst);
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}

use crate::require_value::RequireValue as _;
