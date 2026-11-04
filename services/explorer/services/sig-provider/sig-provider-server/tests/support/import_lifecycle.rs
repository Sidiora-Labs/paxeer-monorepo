use actix_web::{web, App, HttpResponse, HttpServer};
use ethabi::Contract;
use serde_json::{json, Value};
use sig_provider_proto::blockscout::sig_provider::v1::{
    abi_service_server::AbiService, signature_service_server::SignatureService,
    CreateSignaturesRequest, GetFunctionAbiRequest,
};
use sig_provider_server::{new_service, SourcesSettings};
use std::{
    collections::BTreeMap,
    net::TcpListener,
    sync::{Arc, Mutex},
    thread,
    time::Duration,
};
use tokio::sync::mpsc;

const DEADLINE: Duration = Duration::from_secs(5);

#[derive(Clone)]
struct Registry {
    refuse: bool,
    functions: Arc<Mutex<BTreeMap<String, String>>>,
    receipts: mpsc::UnboundedSender<Value>,
}

async fn import(registry: web::Data<Registry>, request: web::Json<Value>) -> HttpResponse {
    let body = request.into_inner();
    let abi = if let Some(abi) = body.get("contract_abi") {
        abi.as_str()
            .expect("fourbyte ABI must be a string")
            .to_owned()
    } else {
        assert_eq!(body["type"], "abi");
        assert_eq!(body["data"].as_array().unwrap().len(), 1);
        body["data"][0].to_string()
    };
    let contract = Contract::load(abi.as_bytes()).expect("production client must send a valid ABI");
    assert_eq!(contract.functions().count(), 1);
    if !registry.refuse {
        for function in contract.functions() {
            registry.functions.lock().unwrap().insert(
                hex::encode(function.short_signature()),
                function.signature(),
            );
        }
    }
    registry.receipts.send(body.clone()).unwrap();
    if registry.refuse {
        HttpResponse::BadRequest().json(json!({"ok": false, "error": "import refused"}))
    } else if body.get("contract_abi").is_some() {
        HttpResponse::Created().json(json!({
            "num_processed": 1, "num_imported": 1, "num_duplicates": 0, "num_ignored": 0
        }))
    } else {
        let function = contract.functions().next().unwrap();
        HttpResponse::Ok().json(json!({"ok": true, "result": {
            "function": {
                "imported": {(function.signature()): format!("0x{}", hex::encode(function.short_signature()))},
                "duplicated": {}, "invalid": null
            },
            "event": {"imported": {}, "duplicated": {}, "invalid": null}
        }}))
    }
}

async fn lookup_fourbyte(
    registry: web::Data<Registry>,
    query: web::Query<BTreeMap<String, String>>,
) -> HttpResponse {
    let functions = registry.functions.lock().unwrap();
    let results: Vec<_> = functions
        .get(query.get("hex_signature").unwrap())
        .map(|name| json!({"text_signature": name}))
        .into_iter()
        .collect();
    HttpResponse::Ok().json(json!({"next": null, "results": results}))
}

async fn lookup_sigeth(
    registry: web::Data<Registry>,
    query: web::Query<BTreeMap<String, String>>,
) -> HttpResponse {
    let hash = query.get("function").unwrap();
    let functions = registry.functions.lock().unwrap();
    let signatures: Vec<_> = functions
        .get(hash.strip_prefix("0x").unwrap())
        .map(|name| json!({"name": name, "filtered": false}))
        .into_iter()
        .collect();
    HttpResponse::Ok().json(json!({
        "ok": true, "result": {"function": {(hash): signatures}, "event": {}}
    }))
}

struct Upstream {
    url: url::Url,
    registry: Registry,
    receipts: mpsc::UnboundedReceiver<Value>,
    server: actix_web::dev::ServerHandle,
    thread: Option<thread::JoinHandle<()>>,
}

impl Upstream {
    fn start(refuse: bool) -> Self {
        let listener = TcpListener::bind(("127.0.0.1", 0)).unwrap();
        let url = format!("http://{}/", listener.local_addr().unwrap())
            .parse()
            .unwrap();
        let (sender, receipts) = mpsc::unbounded_channel();
        let registry = Registry {
            refuse,
            functions: Arc::new(Mutex::new(BTreeMap::new())),
            receipts: sender,
        };
        let state = registry.clone();
        let (started, ready) = std::sync::mpsc::channel();
        let thread = thread::spawn(move || {
            actix_web::rt::System::new().block_on(async move {
                let server = HttpServer::new(move || {
                    App::new()
                        .app_data(web::Data::new(state.clone()))
                        .route("/api/v1/import-solidity/", web::post().to(import))
                        .route("/api/v1/import", web::post().to(import))
                        .route("/api/v1/signatures/", web::get().to(lookup_fourbyte))
                        .route("/api/v1/signatures", web::get().to(lookup_sigeth))
                })
                .workers(1)
                .disable_signals()
                .listen(listener)
                .unwrap()
                .run();
                started.send(server.handle()).unwrap();
                server.await.unwrap();
            });
        });
        Self {
            url,
            registry,
            receipts,
            server: ready.recv_timeout(DEADLINE).unwrap(),
            thread: Some(thread),
        }
    }

    async fn received(&mut self) -> Value {
        tokio::time::timeout(DEADLINE, self.receipts.recv())
            .await
            .expect("production service never reached the upstream HTTP boundary")
            .expect("upstream stopped before receiving the import")
    }
}

impl Drop for Upstream {
    fn drop(&mut self) {
        futures::executor::block_on(self.server.stop(false));
        self.thread.take().unwrap().join().unwrap();
    }
}

fn settings(fourbyte: &Upstream, sigeth: &Upstream) -> SourcesSettings {
    let mut settings = SourcesSettings::default();
    settings.precompiles.enabled = false;
    settings.eth_bytecode_db.enabled = false;
    settings.fourbyte = fourbyte.url.clone();
    settings.sigeth = sigeth.url.clone();
    settings
}

fn producer_abi() -> (String, String) {
    let function = ethabi::Function {
        name: "repairImportLifecycle".to_owned(),
        inputs: vec![],
        outputs: vec![],
        constant: None,
        state_mutability: ethabi::StateMutability::NonPayable,
    };
    let input = format!("0x{}", hex::encode(function.short_signature()));
    let mut entry = serde_json::to_value(function).unwrap();
    entry["type"] = json!("function");
    (json!([entry]).to_string(), input)
}

fn runtime() -> tokio::runtime::Runtime {
    tokio::runtime::Builder::new_current_thread()
        .enable_all()
        .build()
        .unwrap()
}

pub fn acknowledged_import_survives_restart() {
    let fourbyte = Upstream::start(false);
    let sigeth = Upstream::start(false);
    let settings = settings(&fourbyte, &sigeth);
    let (abi, tx_input) = producer_abi();
    let before_restart = runtime();
    before_restart.block_on(async {
        let service = new_service(settings.clone());
        tokio::time::timeout(
            DEADLINE,
            service.create_signatures(tonic::Request::new(CreateSignaturesRequest { abi })),
        )
        .await
        .expect("import acknowledgement timed out")
        .expect("import must be accepted");
        assert_eq!(fourbyte.registry.functions.lock().unwrap().len(), 1);
        assert_eq!(sigeth.registry.functions.lock().unwrap().len(), 1);
    });
    // Dropping the whole runtime kills detached work, as a process restart does.
    drop(before_restart);

    let recovered = runtime().block_on(async {
        let reopened = new_service(settings);
        tokio::time::timeout(
            DEADLINE,
            reopened.get_function_abi(tonic::Request::new(GetFunctionAbiRequest { tx_input })),
        )
        .await
        .expect("recovered lookup timed out")
        .expect("recovered lookup failed")
        .into_inner()
    });
    assert_eq!(
        recovered.abi.len(),
        1,
        "an acknowledged import must remain deliverable after the provider runtime restarts"
    );
    assert_eq!(recovered.abi[0].name, "repairImportLifecycle");
    assert!(recovered.abi[0].inputs.is_empty());
    assert_eq!(fourbyte.registry.functions.lock().unwrap().len(), 1);
    assert_eq!(sigeth.registry.functions.lock().unwrap().len(), 1);
}

pub async fn upstream_refusal_is_observable() {
    refused_import(true, true).await;
}

pub async fn single_upstream_refusal_is_observable() {
    refused_import(true, false).await;
    refused_import(false, true).await;
}

async fn refused_import(refuse_fourbyte: bool, refuse_sigeth: bool) {
    let mut fourbyte = Upstream::start(refuse_fourbyte);
    let mut sigeth = Upstream::start(refuse_sigeth);
    let service = new_service(settings(&fourbyte, &sigeth));
    let (abi, _) = producer_abi();
    let response = tokio::time::timeout(
        DEADLINE,
        service.create_signatures(tonic::Request::new(CreateSignaturesRequest {
            abi: abi.clone(),
        })),
    )
    .await
    .expect("refused import must return an observable result");
    let (fourbyte_request, sigeth_request) = tokio::join!(fourbyte.received(), sigeth.received());
    assert_eq!(fourbyte_request["contract_abi"], abi);
    assert_eq!(
        sigeth_request["data"][0],
        serde_json::from_str::<Value>(&abi).unwrap()
    );
    assert_eq!(
        fourbyte.registry.functions.lock().unwrap().is_empty(),
        refuse_fourbyte
    );
    assert_eq!(
        sigeth.registry.functions.lock().unwrap().is_empty(),
        refuse_sigeth
    );
    assert!(
        response.is_err(),
        "an upstream refused the real HTTP import, but the service acknowledged success: {response:?}"
    );
    let error = response.unwrap_err();
    assert_eq!(error.code(), tonic::Code::Unavailable);
    assert!(error.message().contains("HTTP 400"), "{error}");
    let refused_url = if refuse_sigeth {
        &sigeth.url
    } else {
        &fourbyte.url
    };
    assert!(error.message().contains(refused_url.as_str()), "{error}");
}
