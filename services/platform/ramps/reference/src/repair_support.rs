//! In-process access to the listener, journal, and reconciler.

use super::*;

pub struct Server {
    pub address: std::net::SocketAddr,
    state: Arc<State>,
    mode: Arc<Listener>,
    gate: Arc<ConnectionGate>,
    stop: mpsc::Sender<()>,
    listener: Option<thread::JoinHandle<()>>,
    reconciler: Option<(mpsc::Sender<()>, thread::JoinHandle<()>)>,
}

impl Server {
    /// # Panics
    /// Panics if the listener fixture cannot be constructed or accepted.
    #[must_use]
    pub fn start(config: serde_json::Value, maximum: usize, schedule: bool) -> Self {
        let config: Config = serde_json::from_value(config).required("repair configuration");
        validate_config(&config).required("valid configuration");
        let state = Arc::new(build_state(&config).required("production state"));
        let mode = Arc::new(listener_config(&config).required("production TLS listener"));
        let gate = Arc::new(ConnectionGate::new(maximum));
        let tcp = TcpListener::bind(&config.listen).required("loopback listener");
        let address = tcp.local_addr().required("listener address");
        tcp.set_nonblocking(true).required("nonblocking accept");
        let (stop, stopped) = mpsc::channel();
        let listener = {
            let state = Arc::clone(&state);
            let mode = Arc::clone(&mode);
            let gate = Arc::clone(&gate);
            thread::spawn(move || {
                let mut workers = Vec::new();
                while let Err(mpsc::RecvTimeoutError::Timeout) =
                    stopped.recv_timeout(Duration::from_millis(2))
                {
                    match tcp.accept() {
                        Ok((stream, _)) => {
                            let Some(permit) = gate.try_acquire() else {
                                continue;
                            };
                            let state = Arc::clone(&state);
                            let mode = Arc::clone(&mode);
                            workers.push(thread::spawn(move || {
                                let _permit = permit;
                                serve(stream, &mode, &state);
                            }));
                        }
                        Err(error) if error.kind() == io::ErrorKind::WouldBlock => {}
                        Err(error) => panic!("accept: {error}"),
                    }
                }
                for worker in workers {
                    worker.join().required("request worker");
                }
            })
        };
        let reconciler = schedule.then(|| {
            let state = Arc::clone(&state);
            let (stop, stopped) = mpsc::channel();
            let cadence = Duration::from_secs(config.reconcile_seconds);
            let custody = Duration::from_secs(config.paxeer.poll_cadence_seconds);
            let worker = thread::spawn(move || reconcile_loop(&state, cadence, custody, &stopped));
            (stop, worker)
        });
        Self {
            address,
            state,
            mode,
            gate,
            stop,
            listener: Some(listener),
            reconciler,
        }
    }

    #[must_use]
    pub fn active(&self) -> usize {
        self.gate.active.load(Ordering::Acquire)
    }

    #[must_use]
    pub fn order(&self, id: &str) -> RampOrder {
        self.state
            .journal
            .lock()
            .required("journal")
            .order_by_id(id)
            .required("created order")
            .order
            .clone()
    }

    #[must_use]
    pub fn pending_custody(&self, id: [u8; 32]) -> bool {
        let journal = self.state.journal.lock().required("journal");
        let pending = self.state.pending_rebalances.lock().required("pending");
        pending.contains(&id)
            && journal
                .paxeer(&id)
                .is_some_and(|snapshot| snapshot.operation_id.is_none())
    }

    // The production HTTP codec/writer needs a response larger than TCP's send
    // window to exercise backpressure; ordinary /livez responses fit in it.
    /// # Panics
    /// Panics if the response writer fixture cannot acquire its listener or permit.
    #[must_use]
    pub fn response_writer(&self) -> (std::net::SocketAddr, thread::JoinHandle<io::Result<()>>) {
        let tcp = TcpListener::bind("127.0.0.1:0").required("writer listener");
        let address = tcp.local_addr().required("writer address");
        let mode = Arc::clone(&self.mode);
        let gate = Arc::clone(&self.gate);
        let response = json_response(200, "x".repeat(32 * 1024 * 1024));
        let worker = thread::spawn(move || {
            let (stream, _) = tcp.accept()?;
            let _permit = gate.try_acquire().required("writer admission");
            let stream = DeadlineStream::new(stream)?;
            let Listener::Tls(acceptor) = mode.as_ref() else {
                panic!("TLS writer")
            };
            let mut tls = acceptor
                .accept(stream)
                .map_err(|_| io::Error::other("TLS handshake"))?;
            read_request(&mut tls).map_err(|_| io::Error::other("HTTP request"))?;
            write_response(&mut tls, &response)
        });
        (address, worker)
    }
}

impl Drop for Server {
    fn drop(&mut self) {
        let _ = self.stop.send(());
        if let Some((stop, worker)) = self.reconciler.take() {
            let _ = stop.send(());
            worker.join().required("reconciler shutdown");
        }
        if let Some(worker) = self.listener.take() {
            worker.join().required("listener shutdown");
        }
    }
}
