use super::Checked;
use serde_json::{json, Value};
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc, Mutex,
};
use std::thread::{self, JoinHandle};
use std::time::Duration;

pub fn transcript() -> Value {
    serde_json::from_str(include_str!("../fixtures/repair_1321/committed_chain.json")).checked()
}

pub struct NativePeer {
    pub url: String,
    pub requests: Arc<Mutex<Vec<Value>>>,
    stopped: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

fn serve(mut stream: TcpStream, fixture: &Value, requests: &Mutex<Vec<Value>>) {
    stream
        .set_read_timeout(Some(Duration::from_secs(5)))
        .checked();
    stream
        .set_write_timeout(Some(Duration::from_secs(5)))
        .checked();
    let mut reader = BufReader::new(stream.try_clone().checked());
    let mut line = String::new();
    reader.read_line(&mut line).checked();
    assert!(line.starts_with("POST / HTTP/1.1"), "{line}");
    let mut length = None;
    let mut chunked = false;
    loop {
        line.clear();
        assert!(reader.read_line(&mut line).checked() > 0);
        if line == "\r\n" {
            break;
        }
        let lower = line.to_ascii_lowercase();
        if let Some(value) = lower.strip_prefix("content-length:") {
            length = Some(value.trim().parse::<usize>().checked());
        }
        if lower.starts_with("transfer-encoding:") && lower.contains("chunked") {
            chunked = true;
        }
    }
    let mut body = Vec::new();
    if chunked {
        loop {
            line.clear();
            reader.read_line(&mut line).checked();
            let length = usize::from_str_radix(line.trim(), 16).checked();
            if length == 0 {
                break;
            }
            assert!(body.len() + length <= 16384);
            let start = body.len();
            body.resize(start + length, 0);
            reader.read_exact(&mut body[start..]).checked();
            let mut newline = [0; 2];
            reader.read_exact(&mut newline).checked();
            assert_eq!(&newline, b"\r\n");
        }
    } else {
        let length = length.unwrap_or_else(|| panic!("RPC has a content length"));
        assert!(length <= 16384);
        body.resize(length, 0);
        reader.read_exact(&mut body).checked();
    }
    let request: Value = serde_json::from_slice(&body).checked();
    requests.lock().checked().push(request.clone());
    let exchange = fixture["rpc_exchanges"]
        .as_array()
        .checked()
        .iter()
        .find(|exchange| {
            exchange["method"] == request["method"] && exchange["params"] == request["params"]
        });
    let reply = match exchange {
        Some(exchange) => json!({"jsonrpc":"2.0", "id":request["id"], "result":exchange["result"]}),
        None => {
            json!({"jsonrpc":"2.0", "id":request["id"], "error":{"code":-32602,"message":"request is outside the committed transcript"}})
        }
    };
    let raw = serde_json::to_vec(&reply).checked();
    write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", raw.len()).checked();
    stream.write_all(&raw).checked();
}

impl NativePeer {
    pub fn start(fixture: Value) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").checked();
        let url = format!("http://{}/", listener.local_addr().checked());
        listener.set_nonblocking(true).checked();
        let stopped = Arc::new(AtomicBool::new(false));
        let requests = Arc::new(Mutex::new(Vec::new()));
        let stop = stopped.clone();
        let received = requests.clone();
        let worker = thread::spawn(move || {
            while !stop.load(Ordering::SeqCst) {
                match listener.accept() {
                    Ok((stream, _)) => serve(stream, &fixture, &received),
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                    }
                    Err(error) => panic!("native peer listener: {error}"),
                }
            }
        });
        Self {
            url,
            requests,
            stopped,
            worker: Some(worker),
        }
    }
}

impl Drop for NativePeer {
    fn drop(&mut self) {
        self.stopped.store(true, Ordering::SeqCst);
        if let Some(worker) = self.worker.take() {
            worker.join().checked();
        }
    }
}
