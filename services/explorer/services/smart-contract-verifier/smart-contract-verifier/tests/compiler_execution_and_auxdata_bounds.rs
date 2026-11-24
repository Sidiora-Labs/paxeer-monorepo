use serde_json::{json, Value};
use smart_contract_verifier::{
    solidity, vyper, DetailedVersion, Error, EvmCompilersPool, ListFetcher, OnChainCode,
    OnChainContract, SolcCompiler, SolcInput, VyperCompiler, VyperInput,
};
use std::{
    io::{Read, Write},
    path::{Path, PathBuf},
    sync::Arc,
    time::Duration,
};
use tokio::{
    io::{AsyncReadExt, AsyncWriteExt},
    net::TcpListener,
    sync::{Mutex, Semaphore},
    time::{sleep, timeout},
};

const SOLC_VERSION: &str = "v0.8.14+commit.80d49f37";
const LEGACY_VERSION: &str = "v0.4.8+commit.60cc1668";
const VYPER_VERSION: &str = "v0.4.1+commit.00000000";
const VERSIONS: [&str; 3] = [SOLC_VERSION, LEGACY_VERSION, VYPER_VERSION];
const TEST_BOUND: Duration = Duration::from_secs(10);
static TEST_LOCK: Mutex<()> = Mutex::const_new(());

// Recorded metadata from src/verify/cbor_auxdata.rs; only the length suffix and
// the metadata digest are perturbed to model malformed compiler output.
const SOLC_AUXDATA: &str = "a26469706673582212202e82fb6222f966f0e56dc49cd1fb8a6b5eac9bdf74f62b8a5e9d8812901095d664736f6c634300080e0033";
const VYPER_AUXDATA: &str = "8558203aae1eb6ffcb8f8c87fd598b19f959d0abcc45d96a586f04c3fd37de50a51448182a801820a1657679706572830004010035";

fn fixture_code(language: &str, length: &str, boundary: bool, modified: bool) -> Vec<u8> {
    let (encoded, digest_offset) = if language == "Vyper" {
        (VYPER_AUXDATA, 3)
    } else {
        (SOLC_AUXDATA, 10)
    };
    let mut valid = hex::decode(encoded).unwrap();
    if modified {
        valid[digest_offset] ^= 1;
    }
    let mut last = valid.clone();
    match length {
        "missing" => last.truncate(last.len() - 2),
        "one-byte" => last.truncate(last.len() - 1),
        "wrong" => {
            let end = last.len();
            last[end - 2..].copy_from_slice(&[0, 0]);
        }
        "valid" => {}
        _ => panic!("unknown length fixture"),
    }
    if boundary {
        valid.extend(last);
        valid
    } else {
        last
    }
}

// The gate's own executable supplies the committed subprocess peer. The Linux
// initializer dispatches compiler protocol arguments before libtest parses them;
// no shell, installed compiler, environment switch or extra build is required.
#[used]
#[cfg_attr(target_os = "linux", link_section = ".init_array")]
static COMPILER_FIXTURE: extern "C" fn() = compiler_fixture;

extern "C" fn compiler_fixture() {
    let args: Vec<_> = std::env::args().collect();
    let legacy = args.get(1).map(String::as_str) == Some("--combined-json");
    if !legacy && args.get(1).map(String::as_str) != Some("--standard-json") {
        return;
    }
    let (content, language) = if legacy {
        let content = std::fs::read_to_string(args.last().unwrap()).unwrap();
        (content, "Solidity".to_string())
    } else {
        let mut input = String::new();
        std::io::stdin().read_to_string(&mut input).unwrap();
        let input: Value = serde_json::from_str(&input).unwrap();
        (
            input["sources"]["fixture.sol"]["content"]
                .as_str()
                .unwrap()
                .to_string(),
            input["language"].as_str().unwrap().to_string(),
        )
    };
    let fixture: Value = serde_json::from_str(&content).unwrap();
    if let Some(pid_file) = fixture["pid_file"].as_str() {
        std::fs::write(pid_file, std::process::id().to_string()).unwrap();
    }
    match fixture["mode"].as_str().unwrap() {
        "hang" => loop {
            std::thread::sleep(Duration::from_secs(60));
        },
        "invalid-json" => {
            std::io::stdout().write_all(b"not compiler JSON").unwrap();
            std::process::exit(0);
        }
        "exit-error" => {
            std::io::stderr()
                .write_all(b"fixture compiler refused input")
                .unwrap();
            std::process::exit(7);
        }
        "cbor" | "success" => {}
        _ => panic!("unknown subprocess fixture"),
    }
    let code = if fixture["mode"] == "cbor" {
        fixture_code(
            &language,
            fixture["length"].as_str().unwrap(),
            fixture["boundary"].as_bool().unwrap(),
            content.ends_with(' '),
        )
    } else {
        vec![0]
    };
    let code = hex::encode(code);
    let output = if legacy {
        json!({"contracts":{"Fixture":{"abi":"[]", "bin":code, "bin-runtime":code}}})
    } else {
        json!({
            "contracts":{"fixture.sol":{"Fixture":{
                "abi":[],
                "evm":{"bytecode":{"object":code}, "deployedBytecode":{"object":code}}
            }}},
            "sources":{"fixture.sol":{"id":0}}
        })
    };
    serde_json::to_writer(std::io::stdout(), &output).unwrap();
    std::io::stdout().flush().unwrap();
    std::process::exit(0);
}

struct Fixture {
    directory: tempfile::TempDir,
    fetcher: Arc<ListFetcher<DetailedVersion>>,
    permits: Arc<Semaphore>,
}

impl Fixture {
    async fn new() -> Self {
        let directory = tempfile::tempdir().unwrap();
        for version in VERSIONS {
            let path = directory.path().join(version);
            std::fs::create_dir(&path).unwrap();
            std::os::unix::fs::symlink(std::env::current_exe().unwrap(), path.join("solc"))
                .unwrap();
        }
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let address = listener.local_addr().unwrap();
        let server = tokio::spawn(async move {
            let (mut stream, _) = listener.accept().await.unwrap();
            let mut request = Vec::new();
            let mut buffer = [0; 1024];
            while !request.windows(4).any(|bytes| bytes == b"\r\n\r\n") {
                let count = stream.read(&mut buffer).await.unwrap();
                assert_ne!(count, 0);
                request.extend_from_slice(&buffer[..count]);
            }
            let builds: Vec<_> = VERSIONS
                .into_iter()
                .map(|version| {
                    json!({
                        "path":"solc", "longVersion":version, "sha256":"00".repeat(32)
                    })
                })
                .collect();
            let body = json!({"builds":builds}).to_string();
            let response = format!(
                "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                body.len()
            );
            stream.write_all(response.as_bytes()).await.unwrap();
        });
        let fetcher = ListFetcher::new(
            format!("http://{address}/list.json").parse().unwrap(),
            directory.path().to_path_buf(),
            None,
            None,
        )
        .await
        .unwrap();
        server.await.unwrap();
        Self {
            directory,
            fetcher: Arc::new(fetcher),
            permits: Arc::new(Semaphore::new(1)),
        }
    }

    fn compiler(&self, version: &str) -> PathBuf {
        self.directory.path().join(version).join("solc")
    }

    fn pid_file(&self, case: &str) -> PathBuf {
        self.directory.path().join(format!("{case}.pid"))
    }

    fn solc(
        &self,
        max: usize,
        queue: Duration,
        execution: Duration,
    ) -> EvmCompilersPool<SolcCompiler> {
        EvmCompilersPool::new_with_limits(
            self.fetcher.clone(),
            self.permits.clone(),
            max,
            queue,
            execution,
        )
        .unwrap()
    }

    fn vyper(
        &self,
        max: usize,
        queue: Duration,
        execution: Duration,
    ) -> EvmCompilersPool<VyperCompiler> {
        EvmCompilersPool::new_with_limits(
            self.fetcher.clone(),
            self.permits.clone(),
            max,
            queue,
            execution,
        )
        .unwrap()
    }
}

fn standard_input(language: &str, fixture: Value) -> Value {
    json!({"language":language, "sources":{"fixture.sol":{"content":fixture.to_string()}}, "settings":{}})
}

fn solc_input(fixture: Value) -> SolcInput {
    serde_json::from_value(standard_input("Solidity", fixture)).unwrap()
}

fn assert_compilation_error(error: Error, message: &str) {
    match error {
        Error::Compilation(messages) => {
            assert!(messages.iter().any(|s| s.contains(message)), "{messages:?}")
        }
        other => panic!("expected explicit compilation error, got {other}"),
    }
}

async fn wait_for_pid(path: &Path) -> u32 {
    timeout(TEST_BOUND, async {
        loop {
            if let Ok(pid) = tokio::fs::read_to_string(path).await {
                if let Ok(pid) = pid.parse() {
                    return pid;
                }
            }
            sleep(Duration::from_millis(5)).await;
        }
    })
    .await
    .expect("child must signal readiness")
}

fn assert_reaped(pid: u32) {
    assert!(
        !Path::new(&format!("/proc/{pid}")).exists(),
        "child {pid} is still alive or unreaped"
    );
}

fn gauge(name: &str) -> f64 {
    prometheus::gather()
        .iter()
        .find(|family| family.get_name() == name)
        .unwrap_or_else(|| panic!("missing gauge {name}"))
        .get_metric()[0]
        .get_gauge()
        .get_value()
}

fn assert_idle(permits: &Semaphore) {
    assert_eq!(permits.available_permits(), 1);
    assert_eq!(gauge("smart_contract_verifier_compiles_in_queue"), 0.0);
    assert_eq!(gauge("smart_contract_verifier_compiles_in_flight"), 0.0);
}

#[tokio::test]
async fn compiler_execution_deadline_reaps_all_adapters_and_recovers_permits() {
    let _serial = TEST_LOCK.lock().await;
    let fixture = Fixture::new().await;
    for version in [SOLC_VERSION, LEGACY_VERSION] {
        let pool = fixture.solc(2, Duration::from_secs(2), Duration::from_secs(1));
        let pid_file = fixture.pid_file(version);
        let input = solc_input(json!({"mode":"hang", "pid_file":pid_file}));
        let error = timeout(
            TEST_BOUND,
            pool.compile(
                &fixture.compiler(version),
                &version.parse().unwrap(),
                &input,
            ),
        )
        .await
        .unwrap()
        .unwrap_err();
        assert_compilation_error(error, "execution deadline exceeded");
        assert_reaped(wait_for_pid(&pid_file).await);
        assert_idle(&fixture.permits);
        let output = pool
            .compile(
                &fixture.compiler(version),
                &version.parse().unwrap(),
                &solc_input(json!({"mode":"success"})),
            )
            .await
            .unwrap();
        assert!(output.raw["contracts"]["fixture.sol"]["Fixture"].is_object());
        assert_idle(&fixture.permits);
    }
    let pool = fixture.vyper(2, Duration::from_secs(2), Duration::from_secs(1));
    let pid_file = fixture.pid_file("vyper-timeout");
    let input: VyperInput = serde_json::from_value(standard_input(
        "Vyper",
        json!({"mode":"hang", "pid_file":pid_file}),
    ))
    .unwrap();
    let error = timeout(
        TEST_BOUND,
        pool.compile(
            &fixture.compiler(VYPER_VERSION),
            &VYPER_VERSION.parse().unwrap(),
            &input,
        ),
    )
    .await
    .unwrap()
    .unwrap_err();
    assert_compilation_error(error, "execution deadline exceeded");
    assert_reaped(wait_for_pid(&pid_file).await);
    assert_idle(&fixture.permits);
    let input: VyperInput =
        serde_json::from_value(standard_input("Vyper", json!({"mode":"success"}))).unwrap();
    let output = pool
        .compile(
            &fixture.compiler(VYPER_VERSION),
            &VYPER_VERSION.parse().unwrap(),
            &input,
        )
        .await
        .unwrap();
    assert!(output.raw["contracts"]["fixture.sol"]["Fixture"].is_object());
    assert_idle(&fixture.permits);
}

#[tokio::test]
async fn compiler_cancellation_reaps_before_capacity_is_reused() {
    let _serial = TEST_LOCK.lock().await;
    let fixture = Fixture::new().await;
    for version in VERSIONS {
        let pid_file = fixture.pid_file(version);
        if version == VYPER_VERSION {
            let pool = Arc::new(fixture.vyper(1, TEST_BOUND, TEST_BOUND));
            let task_pool = pool.clone();
            let compiler = fixture.compiler(version);
            let input: VyperInput = serde_json::from_value(standard_input(
                "Vyper",
                json!({"mode":"hang", "pid_file":pid_file}),
            ))
            .unwrap();
            let task = tokio::spawn(async move {
                task_pool
                    .compile(&compiler, &version.parse().unwrap(), &input)
                    .await
            });
            let pid = wait_for_pid(&pid_file).await;
            task.abort();
            assert!(task.await.unwrap_err().is_cancelled());
            let permit = timeout(TEST_BOUND, fixture.permits.acquire())
                .await
                .unwrap()
                .unwrap();
            assert_reaped(pid);
            drop(permit);
            let input: VyperInput =
                serde_json::from_value(standard_input("Vyper", json!({"mode":"success"}))).unwrap();
            pool.compile(
                &fixture.compiler(version),
                &version.parse().unwrap(),
                &input,
            )
            .await
            .unwrap();
        } else {
            let pool = Arc::new(fixture.solc(1, TEST_BOUND, TEST_BOUND));
            let task_pool = pool.clone();
            let compiler = fixture.compiler(version);
            let input = solc_input(json!({"mode":"hang", "pid_file":pid_file}));
            let task = tokio::spawn(async move {
                task_pool
                    .compile(&compiler, &version.parse().unwrap(), &input)
                    .await
            });
            let pid = wait_for_pid(&pid_file).await;
            task.abort();
            assert!(task.await.unwrap_err().is_cancelled());
            let permit = timeout(TEST_BOUND, fixture.permits.acquire())
                .await
                .unwrap()
                .unwrap();
            assert_reaped(pid);
            drop(permit);
            pool.compile(
                &fixture.compiler(version),
                &version.parse().unwrap(),
                &solc_input(json!({"mode":"success"})),
            )
            .await
            .unwrap();
        }
        assert_idle(&fixture.permits);
    }
}

#[tokio::test]
async fn compiler_admission_queue_deadline_and_queue_cancellation_release_capacity() {
    let _serial = TEST_LOCK.lock().await;
    let fixture = Fixture::new().await;
    let pool = Arc::new(fixture.solc(1, Duration::from_millis(100), TEST_BOUND));
    let held = fixture.permits.acquire().await.unwrap();
    let compiler = fixture.compiler(SOLC_VERSION);
    let version = SOLC_VERSION.parse().unwrap();
    let input = solc_input(json!({"mode":"success"}));
    {
        let waiting = pool.compile(&compiler, &version, &input);
        tokio::pin!(waiting);
        tokio::select! {
            result = &mut waiting => panic!("request unexpectedly finished: {result:?}"),
            _ = sleep(Duration::from_millis(10)) => {}
        }
        let error = pool.compile(&compiler, &version, &input).await.unwrap_err();
        assert_compilation_error(error, "admission capacity exhausted");
        let error = timeout(TEST_BOUND, &mut waiting)
            .await
            .unwrap()
            .unwrap_err();
        assert_compilation_error(error, "queue deadline exceeded");
        assert_eq!(gauge("smart_contract_verifier_compiles_in_queue"), 0.0);
        assert_eq!(fixture.permits.available_permits(), 0);
    }
    drop(held);
    pool.compile(&compiler, &version, &input).await.unwrap();
    assert_idle(&fixture.permits);

    let held = fixture.permits.acquire().await.unwrap();
    let task_pool = pool.clone();
    let task = tokio::spawn(async move { task_pool.compile(&compiler, &version, &input).await });
    timeout(TEST_BOUND, async {
        while gauge("smart_contract_verifier_compiles_in_queue") != 1.0 {
            tokio::task::yield_now().await;
        }
    })
    .await
    .unwrap();
    task.abort();
    assert!(task.await.unwrap_err().is_cancelled());
    timeout(TEST_BOUND, async {
        while gauge("smart_contract_verifier_compiles_in_queue") != 0.0 {
            tokio::task::yield_now().await;
        }
    })
    .await
    .unwrap();
    drop(held);
    pool.compile(
        &fixture.compiler(SOLC_VERSION),
        &SOLC_VERSION.parse().unwrap(),
        &solc_input(json!({"mode":"success"})),
    )
    .await
    .unwrap();
    assert_idle(&fixture.permits);
}

#[tokio::test]
async fn compiler_spawn_and_output_errors_release_capacity() {
    let _serial = TEST_LOCK.lock().await;
    let fixture = Fixture::new().await;
    let pool = fixture.solc(1, TEST_BOUND, TEST_BOUND);
    for (compiler, mode) in [
        (fixture.directory.path().join("missing-compiler"), "success"),
        (fixture.compiler(SOLC_VERSION), "invalid-json"),
        (fixture.compiler(SOLC_VERSION), "exit-error"),
    ] {
        let error = pool
            .compile(
                &compiler,
                &SOLC_VERSION.parse().unwrap(),
                &solc_input(json!({"mode":mode})),
            )
            .await
            .unwrap_err();
        let expected = match mode {
            "invalid-json" => "deserializing compiler output",
            "exit-error" => "fixture compiler refused input",
            _ => "spawning compiler process",
        };
        assert!(error.to_string().contains(expected), "{error}");
        assert_idle(&fixture.permits);
    }
    pool.compile(
        &fixture.compiler(SOLC_VERSION),
        &SOLC_VERSION.parse().unwrap(),
        &solc_input(json!({"mode":"success"})),
    )
    .await
    .unwrap();
    assert_idle(&fixture.permits);
    assert!(EvmCompilersPool::<SolcCompiler>::new_with_limits(
        fixture.fetcher.clone(),
        fixture.permits.clone(),
        0,
        TEST_BOUND,
        TEST_BOUND
    )
    .is_err());
    assert!(EvmCompilersPool::<SolcCompiler>::new_with_limits(
        fixture.fetcher.clone(),
        fixture.permits.clone(),
        1,
        Duration::ZERO,
        TEST_BOUND
    )
    .is_err());
    assert!(EvmCompilersPool::<SolcCompiler>::new_with_limits(
        fixture.fetcher.clone(),
        fixture.permits.clone(),
        1,
        TEST_BOUND,
        Duration::ZERO
    )
    .is_err());
}

fn contract(code: Vec<u8>) -> OnChainContract {
    OnChainContract {
        code: OnChainCode::complete(code.clone(), code),
        chain_id: None,
        address: None,
    }
}

#[tokio::test]
async fn solidity_cbor_backward_bound() {
    let _serial = TEST_LOCK.lock().await;
    let fixture = Fixture::new().await;
    let pool = fixture.solc(1, TEST_BOUND, TEST_BOUND);
    pool.load_from_dir(&fixture.directory.path().to_path_buf())
        .await;
    for boundary in [false, true] {
        for length in ["missing", "one-byte", "wrong", "valid"] {
            let request = solidity::standard_json::VerificationRequest {
                contract: contract(fixture_code("Solidity", length, boundary, false)),
                compiler_version: SOLC_VERSION.parse().unwrap(),
                content: solc_input(json!({"mode":"cbor", "length":length, "boundary":boundary})),
            };
            let result = timeout(TEST_BOUND, solidity::standard_json::verify(&pool, request))
                .await
                .unwrap();
            if length == "valid" {
                let contracts = result.unwrap();
                assert_eq!(contracts.len(), 1);
                let expected = if boundary { 2 } else { 1 };
                let creation = contracts[0]
                    .creation_code_artifacts
                    .cbor_auxdata
                    .as_ref()
                    .unwrap();
                let runtime = contracts[0]
                    .runtime_code_artifacts
                    .cbor_auxdata
                    .as_ref()
                    .unwrap();
                assert_eq!(creation.len(), expected);
                assert_eq!(runtime.len(), expected);
                if boundary {
                    assert_eq!(
                        creation["2"].offset as usize,
                        hex::decode(SOLC_AUXDATA).unwrap().len()
                    );
                }
            } else {
                let error = result.unwrap_err();
                assert!(
                    error
                        .to_string()
                        .contains("failed to parse next cbor auxdata value"),
                    "{length} boundary={boundary}: {error}"
                );
            }
        }
    }
}

#[tokio::test]
async fn vyper_cbor_backward_bound() {
    let _serial = TEST_LOCK.lock().await;
    let fixture = Fixture::new().await;
    let pool = fixture.vyper(1, TEST_BOUND, TEST_BOUND);
    pool.load_from_dir(&fixture.directory.path().to_path_buf())
        .await;
    for boundary in [false, true] {
        for length in ["missing", "one-byte", "wrong", "valid"] {
            let request = vyper::standard_json::VerificationRequest {
                contract: contract(fixture_code("Vyper", length, boundary, false)),
                compiler_version: VYPER_VERSION.parse().unwrap(),
                content: serde_json::from_value(standard_input(
                    "Vyper",
                    json!({"mode":"cbor", "length":length, "boundary":boundary}),
                ))
                .unwrap(),
            };
            let result = timeout(TEST_BOUND, vyper::standard_json::verify(&pool, request))
                .await
                .unwrap();
            if length == "valid" {
                let contracts = result.unwrap();
                assert_eq!(contracts.len(), 1);
                let expected = if boundary { 2 } else { 1 };
                let creation = contracts[0]
                    .creation_code_artifacts
                    .cbor_auxdata
                    .as_ref()
                    .unwrap();
                let runtime = contracts[0]
                    .runtime_code_artifacts
                    .cbor_auxdata
                    .as_ref()
                    .unwrap();
                assert_eq!(creation.len(), expected);
                assert_eq!(runtime.len(), expected);
                if boundary {
                    assert_eq!(
                        creation["2"].offset as usize,
                        hex::decode(VYPER_AUXDATA).unwrap().len()
                    );
                }
            } else {
                let error = result.unwrap_err();
                assert!(
                    error
                        .to_string()
                        .contains("failed to parse next cbor auxdata value"),
                    "{length} boundary={boundary}: {error}"
                );
            }
        }
    }
}
