use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::{
    atomic::{AtomicBool, AtomicUsize, Ordering},
    Arc, Condvar, Mutex,
};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use layerx_oracle_feeder::feed_agent;

const REQUESTS_PER_ROUND: usize = 18;
const ROUNDS: usize = 5;

struct Feed {
    address: String,
    accepts: Arc<AtomicUsize>,
    stop: Arc<AtomicBool>,
    listener_thread: Option<JoinHandle<()>>,
    connections: Arc<Mutex<Vec<JoinHandle<()>>>>,
}

struct RoundGate {
    state: Mutex<(usize, usize)>,
    released: Condvar,
}

impl RoundGate {
    fn wait(&self, stop: &AtomicBool) -> bool {
        let mut state = self.state.lock().expect("round gate lock");
        let target = (state.0 / REQUESTS_PER_ROUND + 1) * REQUESTS_PER_ROUND;
        state.0 += 1;
        if state.0 == target {
            state.1 = target;
            self.released.notify_all();
        } else {
            while state.1 < target && !stop.load(Ordering::Acquire) {
                state = self
                    .released
                    .wait_timeout(state, Duration::from_millis(10))
                    .expect("round gate wait")
                    .0;
            }
        }
        !stop.load(Ordering::Acquire)
    }
}

impl Feed {
    fn start() -> Self {
        let listener = TcpListener::bind(("127.0.0.1", 0)).expect("bind stand-in feed");
        listener
            .set_nonblocking(true)
            .expect("set listener nonblocking");
        let address = format!(
            "http://{}",
            listener.local_addr().expect("listener address")
        );
        let accepts = Arc::new(AtomicUsize::new(0));
        let stop = Arc::new(AtomicBool::new(false));
        let connections = Arc::new(Mutex::new(Vec::new()));
        let gate = Arc::new(RoundGate {
            state: Mutex::new((0, 0)),
            released: Condvar::new(),
        });
        let accepts_for_thread = Arc::clone(&accepts);
        let stop_for_thread = Arc::clone(&stop);
        let connections_for_thread = Arc::clone(&connections);
        let gate_for_thread = Arc::clone(&gate);
        let listener_thread = thread::spawn(move || {
            while !stop_for_thread.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((stream, _)) => {
                        accepts_for_thread.fetch_add(1, Ordering::AcqRel);
                        let stop_for_connection = Arc::clone(&stop_for_thread);
                        let gate_for_connection = Arc::clone(&gate_for_thread);
                        let connection = thread::spawn(move || {
                            serve_connection(stream, stop_for_connection, gate_for_connection)
                        });
                        connections_for_thread
                            .lock()
                            .expect("connection list lock")
                            .push(connection);
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("accept stand-in feed connection: {error}"),
                }
            }
        });
        Self {
            address,
            accepts,
            stop,
            listener_thread: Some(listener_thread),
            connections,
        }
    }

    fn url(&self) -> String {
        format!("{}/ticker", self.address)
    }

    fn accepts(&self) -> usize {
        self.accepts.load(Ordering::Acquire)
    }
}

impl Drop for Feed {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        let _ = TcpStream::connect(self.address.trim_start_matches("http://"));
        if let Some(listener_thread) = self.listener_thread.take() {
            listener_thread.join().expect("join feed listener");
        }
        let connections =
            std::mem::take(&mut *self.connections.lock().expect("connection list lock"));
        for connection in connections {
            connection.join().expect("join feed connection");
        }
    }
}

fn serve_connection(mut stream: TcpStream, stop: Arc<AtomicBool>, gate: Arc<RoundGate>) {
    stream
        .set_read_timeout(Some(Duration::from_millis(100)))
        .expect("set connection read timeout");
    let response = b"HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: keep-alive\r\n\r\n{}";
    let mut request = [0_u8; 4096];
    let mut buffered = Vec::new();
    while !stop.load(Ordering::Acquire) {
        match stream.read(&mut request) {
            Ok(0) => break,
            Ok(read) => {
                buffered.extend_from_slice(&request[..read]);
                while let Some(end) = buffered.windows(4).position(|window| window == b"\r\n\r\n") {
                    buffered.drain(..end + 4);
                    if !gate.wait(&stop) {
                        return;
                    }
                    if stream.write_all(response).is_err() {
                        return;
                    }
                    if stream.flush().is_err() {
                        return;
                    }
                }
            }
            Err(error)
                if matches!(
                    error.kind(),
                    std::io::ErrorKind::WouldBlock | std::io::ErrorKind::TimedOut
                ) => {}
            Err(_) => break,
        }
    }
}

fn send_round(agent: &ureq::Agent, url: &str) {
    thread::scope(|scope| {
        let requests = (0..REQUESTS_PER_ROUND)
            .map(|_| {
                let agent = agent.clone();
                scope.spawn(move || {
                    let mut response = agent.get(url).call().expect("feed request");
                    assert_eq!(response.status(), 200);
                    assert_eq!(
                        response.body_mut().read_to_string().expect("feed body"),
                        "{}"
                    );
                })
            })
            .collect::<Vec<_>>();
        for request in requests {
            request.join().expect("join feed request");
        }
    });
}

fn run_rounds(agent: &ureq::Agent, url: &str) {
    for _ in 0..ROUNDS {
        send_round(agent, url);
    }
}

#[test]
fn feed_connection_reuse_configured_pool_stays_within_round_size() {
    let feed = Feed::start();
    let url = feed.url();
    let agent = feed_agent(None);
    assert_eq!(agent.config().max_idle_connections(), 64);
    assert_eq!(agent.config().max_idle_connections_per_host(), 64);
    run_rounds(&agent, &url);
    assert!(
        feed.accepts() <= REQUESTS_PER_ROUND,
        "accepted {} connections",
        feed.accepts()
    );
}

#[test]
fn feed_connection_reuse_default_pool_is_smaller() {
    let feed = Feed::start();
    let url = feed.url();
    let agent: ureq::Agent = ureq::Agent::config_builder().build().into();
    run_rounds(&agent, &url);
    assert!(
        feed.accepts() > REQUESTS_PER_ROUND,
        "accepted {} connections",
        feed.accepts()
    );
}
