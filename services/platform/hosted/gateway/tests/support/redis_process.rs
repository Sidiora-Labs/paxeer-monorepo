use layerx_platform_gateway::store::{RedisEndpoint, RedisStore};
use native_tls::Certificate;
use std::fs;
use std::net::{TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicU64, Ordering};
use std::thread;
use std::time::{Duration, SystemTime, UNIX_EPOCH};
use zeroize::Zeroizing;

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(0);

pub struct RedisProcess {
    child: Child,
    pub directory: PathBuf,
    endpoint: RedisEndpoint,
    pub certificate: Certificate,
    pub url: String,
    port: u16,
}

impl RedisProcess {
    pub fn start() -> Self {
        let directory = loop {
            let counter = NEXT_DIRECTORY
                .fetch_update(Ordering::Relaxed, Ordering::Relaxed, |value| {
                    value.checked_add(1)
                })
                .expect("test Redis directory counter must not overflow");
            let unique = format!(
                "layerx-principal-audit-redis-{}-{counter}-{}",
                std::process::id(),
                SystemTime::now()
                    .duration_since(UNIX_EPOCH)
                    .unwrap_or_default()
                    .as_nanos()
            );
            let directory = std::env::temp_dir().join(unique);
            match fs::create_dir(&directory) {
                Ok(()) => break directory,
                Err(error) if error.kind() == std::io::ErrorKind::AlreadyExists => continue,
                Err(error) => panic!("test Redis directory must be created: {error}"),
            }
        };
        let certificate_pem = directory.join("server.pem");
        let certificate_der = directory.join("server.der");
        let private_key = directory.join("server.key");
        command(
            "openssl",
            &[
                "req",
                "-x509",
                "-newkey",
                "rsa:2048",
                "-nodes",
                "-keyout",
                path(&private_key),
                "-out",
                path(&certificate_pem),
                "-days",
                "1",
                "-subj",
                "/CN=localhost",
                "-addext",
                "subjectAltName=DNS:localhost",
            ],
        );
        command(
            "openssl",
            &[
                "x509",
                "-in",
                path(&certificate_pem),
                "-outform",
                "DER",
                "-out",
                path(&certificate_der),
            ],
        );
        let listener = TcpListener::bind("127.0.0.1:0")
            .unwrap_or_else(|error| panic!("test port must be allocated: {error}"));
        let port = listener
            .local_addr()
            .unwrap_or_else(|error| panic!("test port must resolve: {error}"))
            .port();
        drop(listener);
        let acl = directory.join("users.acl");
        fs::write(
            &acl,
            "user default on nopass ~* &* +@all\nuser tap on >tap-secret ~* &* +@all\n",
        )
        .unwrap_or_else(|error| panic!("test Redis ACL must be written: {error}"));
        let config = directory.join("redis.conf");
        fs::write(
            &config,
            format!(
                "bind 127.0.0.1\nport 0\ntls-port {port}\ntls-cert-file {}\ntls-key-file {}\ntls-ca-cert-file {}\ntls-auth-clients no\naclfile {}\nappendonly yes\nappendfsync always\ndir {}\nprotected-mode yes\n",
                path(&certificate_pem),
                path(&private_key),
                path(&certificate_pem),
                path(&acl),
                path(&directory),
            ),
        )
        .unwrap_or_else(|error| panic!("test Redis config must be written: {error}"));
        let endpoint = RedisEndpoint::parse(&format!("rediss://localhost:{port}"))
            .unwrap_or_else(|error| panic!("test Redis endpoint must parse: {error}"));
        let certificate = Certificate::from_der(
            &fs::read(&certificate_der)
                .unwrap_or_else(|error| panic!("test certificate must be read: {error}")),
        )
        .unwrap_or_else(|error| panic!("test certificate must parse: {error}"));
        let child = Command::new("redis-server")
            .arg(&config)
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .spawn()
            .unwrap_or_else(|error| panic!("real Redis server must start: {error}"));
        let process = Self {
            child,
            directory,
            endpoint,
            certificate,
            url: format!("rediss://localhost:{port}"),
            port,
        };
        for _ in 0..100 {
            if TcpStream::connect(("127.0.0.1", port)).is_ok() {
                return process;
            }
            thread::sleep(Duration::from_millis(20));
        }
        redis_unreachable(process)
    }

    pub fn process_id(&self) -> u32 {
        self.child.id()
    }

    pub fn restart(&mut self) {
        self.child
            .kill()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "stop Redis before recovery"));
        self.child
            .wait()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "reap stopped Redis"));
        self.child = Command::new("redis-server")
            .arg(self.directory.join("redis.conf"))
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .spawn()
            .unwrap_or_else(|error| {
                panic!(
                    "{}: {error:?}",
                    "reopen Redis on the original AOF directory"
                )
            });
        for _ in 0..100 {
            assert!(
                self.child
                    .try_wait()
                    .unwrap_or_else(|error| panic!("{}: {error:?}", "Redis process status"))
                    .is_none(),
                "Redis refused recovery"
            );
            if self.store().ready() {
                return;
            }
            thread::sleep(Duration::from_millis(20));
        }
        panic!("reopened Redis did not recover its persisted state");
    }

    pub fn command(&self, arguments: &[&str]) -> serde_json::Value {
        let output = Command::new("redis-cli")
            .args([
                "-2",
                "--tls",
                "--cacert",
                path(&self.directory.join("server.pem")),
                "--user",
                "tap",
                "--pass",
                "tap-secret",
                "--no-auth-warning",
                "-h",
                "localhost",
                "-p",
                &self.port.to_string(),
                "--json",
            ])
            .args(arguments)
            .output()
            .expect("Redis CLI must run");
        assert!(
            output.status.success(),
            "Redis command failed: {}",
            String::from_utf8_lossy(&output.stderr)
        );
        serde_json::from_slice(&output.stdout).expect("Redis JSON response")
    }

    pub fn store(&self) -> RedisStore {
        RedisStore::new(
            self.endpoint.clone(),
            self.certificate.clone(),
            Zeroizing::new("tap".to_owned()),
            Zeroizing::new("tap-secret".to_owned()),
        )
    }
}

impl Drop for RedisProcess {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
        let _ = fs::remove_dir_all(&self.directory);
    }
}

fn command(program: &str, arguments: &[&str]) {
    let output = Command::new(program)
        .args(arguments)
        .output()
        .unwrap_or_else(|error| panic!("{program} must run: {error}"));
    assert!(
        output.status.success(),
        "{program} failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );
}

fn path(value: &Path) -> &str {
    value
        .to_str()
        .unwrap_or_else(|| panic!("test path must be UTF-8"))
}

fn redis_unreachable(mut process: RedisProcess) -> ! {
    let _ = process.child.kill();
    process.child.wait().expect("Redis process must be reaped");
    panic!("real Redis server did not become reachable")
}
