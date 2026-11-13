//! Repair record: `mcp_readiness_test_rewrite` is proven by
//! `mcp_readiness_test_rewrite_publishes_and_recovers_exact_output` and
//! `mcp_readiness_test_rewrite_refuses_unprotected_publication_credentials`.
//! Wiring request: `tools/platform/cli/tests/mcp_daemon_bound.rs` (CLI domain)
//! must prepare Gateway before enrolment, pass `Gateway::readiness` to `DaemonSurface`,
//! then serve the unmodified publication; its call sites are outside this task's scope.
//! Baseline: the repair target did not exist on the unchanged tree; no baseline
//! verify was run because the task's ordered gates require implementation first.

use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::os::unix::fs::PermissionsExt as _;
use std::path::Path;
use std::time::Duration;

use layerx_agentd::budget::{LimitConfig, LimitId, LimitScope};
use layerx_agentd::capability::timed::{self, NativeTimedCapabilityV1, TimedCapability};
use layerx_agentd::capability::{Capability, CapabilityDimensions, CapabilityId, RateCeiling};
use layerx_agentd::enrolment::{
    self, BindingMode, BindingPublisher, DaemonSurface, EnrolmentError, EnrolmentRequest,
    PublishedBinding, ReadinessDeclaration,
};
use layerx_agentd::identity::{
    register, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
};
use layerx_agentd::session::{SessionId, SessionRegistry};
use layerx_agentd::store::{Store, TenantId};
use layerx_mcp::binding::Binding;
use layerx_mcp::server::catalogue;
use layerx_mcp::stdio::DaemonClientSession;
use layerx_sdk::agent_envelope::{AgentEnvelopeTransport, EnvelopeCredential};
use layerx_types::ids::Did;
use layerx_types::payload::{ActivityType, ModuleId};
use layerx_types::verify::VerificationLevel;
use serde_json::{json, Value};

const OBSERVED_SEQUENCE: u64 = 120;

#[path = "common/daemon_gateway.rs"]
mod daemon_gateway;

struct BoundaryIdentity(CoreIdentity);

impl IdentityResolver for BoundaryIdentity {
    fn resolve(&mut self, _did: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        Ok(Some(self.0.clone()))
    }
}

fn enrol(
    root: &Path,
    readiness: ReadinessDeclaration,
) -> Result<
    (
        PublishedBinding,
        BindingPublisher,
        layerx_agentd::identity::IdentityRecord,
        EnrolmentRequest,
    ),
    EnrolmentError,
> {
    let mut store =
        Store::open(root.join("store")).unwrap_or_else(|error| panic!("store: {error}"));
    let tenant = TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error}"));
    let capability = Capability::new(
        CapabilityId([9; 32]),
        tenant.clone(),
        CapabilityDimensions {
            activity_types: BTreeSet::from([7]),
            counterparties: BTreeSet::from([[2; 32]]),
            assets: BTreeSet::from([[3; 32]]),
            amount_ceiling: 100,
            rate_ceiling: RateCeiling {
                maximum_uses: 2,
                window_sequences: 10,
            },
            purposes: BTreeSet::from(["service-payment".to_owned()]),
            expiry_sequence: 400,
        },
    )
    .unwrap_or_else(|error| panic!("capability: {error:?}"));
    capability
        .persist(&mut store)
        .unwrap_or_else(|error| panic!("capability persist: {error:?}"));
    let mut boundary = BoundaryIdentity(CoreIdentity {
        canonical_bytes: b"model-identity".to_vec(),
        head_sequence: 10,
        revocation_sequence: 1,
        verification_level: VerificationLevel::STATE_PROVEN,
        frozen: false,
        authorities: vec![ProtocolAuthority::CapabilityGrant(capability.id.0)],
    });
    let identity = register(
        &mut store,
        tenant,
        Did::new(b"did:layerx:model").unwrap_or_else(|error| panic!("DID: {error:?}")),
        &mut boundary,
    )
    .unwrap_or_else(|error| panic!("identity: {error:?}"));
    let binding_publisher_handle = binding_publisher(root, readiness);
    let mut sessions = SessionRegistry::default();
    let request = EnrolmentRequest {
        session_id: SessionId([0x0c; 32]),
        capability_id: capability.id,
        permitted_activity_types: BTreeSet::from([7]),
        scopes: catalogue()
            .iter()
            .map(|tool| tool.required_scope.to_owned())
            .collect(),
        expiry_sequence: 300,
        opening_client: "mcp".to_owned(),
        policy_version: "policy-v1".to_owned(),
        core_sequence: 50,
    };
    let published = enrolment::enrol(
        &mut store,
        &mut sessions,
        &identity,
        request.clone(),
        &binding_publisher_handle,
    )?;
    drop(store);
    persist_native_grant(root);
    Ok((published, binding_publisher_handle, identity, request))
}

fn binding_publisher(root: &Path, readiness: ReadinessDeclaration) -> BindingPublisher {
    let surface = DaemonSurface::new("127.0.0.1:9", "d".repeat(48), [0xcc; 32])
        .unwrap_or_else(|error| panic!("surface: {error}"))
        .with_readiness(readiness);
    BindingPublisher::new(
        root.join("mcp"),
        root.join("store"),
        root.join("audit"),
        surface,
        LimitConfig {
            id: LimitId([0x0a; 16]),
            name: "mcp".to_owned(),
            scope: LimitScope::Tenant([1; 32]),
            ceiling: 1_000,
            consumed: 0,
        },
        Duration::from_millis(5_000),
        BindingMode::Full,
    )
    .unwrap_or_else(|error| panic!("publisher: {error}"))
}

fn persist_native_grant(root: &Path) {
    let mut store = Store::open(root.join("store"))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "native capability store"));
    let tenant =
        TenantId::new("tenant-a").unwrap_or_else(|error| panic!("{}: {error:?}", "native tenant"));
    let native = NativeTimedCapabilityV1 {
        record: TimedCapability {
            id: [9; 32],
            parent: None,
            tenant: tenant.clone(),
            agent: "did:layerx:model".into(),
            authority: ProtocolAuthority::CapabilityGrant([9; 32]),
            activity_types: BTreeSet::new(),
            counterparties: BTreeSet::from([[2; 32]]),
            assets: BTreeSet::from([[3; 32]]),
            amount_ceilings: BTreeMap::from([([3; 32], 100)]),
            rate_ceilings: BTreeMap::from([(10, 2)]),
            purposes: BTreeSet::new(),
            expiry_seconds: 400,
            grant_not_after_ms: 400_000,
            created_at_ms: 1,
            created_at_sequence: 1,
            revoked: None,
        },
        activities: [1, 5, 6, 7, 10]
            .into_iter()
            .map(|ordinal| {
                ActivityType::new(ModuleId::Asset, ordinal)
                    .unwrap_or_else(|error| panic!("{}: {error:?}", "native activity"))
                    .into()
            })
            .collect(),
        purpose_commitments: BTreeSet::new(),
        spend_ceilings: BTreeMap::new(),
    };
    store
        .put_local(
            timed::native_record_key(&tenant, &[9; 32])
                .unwrap_or_else(|error| panic!("{}: {error:?}", "native key")),
            native
                .encode()
                .unwrap_or_else(|error| panic!("{}: {error:?}", "native capability")),
        )
        .unwrap_or_else(|error| panic!("{}: {error:?}", "persist native grant"));
    drop(store);
}

fn gateway_authentication_required(root: &Path, endpoint: &str, binding_path: &Path) {
    let binding = Binding::open(binding_path)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "unchanged binding"));
    let session = binding
        .daemon_credential()
        .unwrap_or_else(|error| panic!("{}: {error:?}", "real enrolled credential"));
    let credential = EnvelopeCredential::new(
        session.tenant().as_str(),
        session.session_id().0,
        session.token_id(),
        session.generation(),
    )
    .unwrap_or_else(|error| panic!("{}: {error:?}", "session envelope credential"));
    let transport = AgentEnvelopeTransport::connect(endpoint, None, Some(&root.join("ca.pem")))
        .unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "TLS transport without a gateway credential"
            )
        });
    let Err(refusal) = DaemonClientSession::connect(transport, credential, binding.mode()) else {
        panic!("a session token must not replace gateway authentication");
    };
    assert_eq!(refusal["reason"], "gateway.authentication");
    assert_eq!(refusal["state"], "refused");
}

fn consume_published_binding(binding_path: &Path) -> Value {
    let binding = Binding::open(binding_path)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "exact producer document"));
    let mut client = binding
        .open_daemon_client()
        .unwrap_or_else(|error| panic!("authenticated published transport: {}", error.detail()));
    let initialization = client
        .handle(
            &json!({
                "jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}
            })
            .to_string(),
        )
        .unwrap_or_else(|| panic!("{}", "MCP initialization"));
    assert_eq!(
        initialization["result"]["_meta"]["layerx/binding"],
        "agent-daemon"
    );
    assert_eq!(
        initialization["result"]["_meta"]["layerx/deployment_mode"],
        "full"
    );
    let response = client
        .handle(
            &json!({
                "jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}
            })
            .to_string(),
        )
        .unwrap_or_else(|| panic!("{}", "MCP catalogue"));
    assert!(response.get("error").is_none(), "{response}");
    let tools = response["result"]["tools"]
        .as_array()
        .unwrap_or_else(|| panic!("{}", "daemon-produced tools"));
    let names = tools
        .iter()
        .map(|tool| {
            tool["name"]
                .as_str()
                .unwrap_or_else(|| panic!("{}", "tool name"))
        })
        .collect::<BTreeSet<_>>();
    assert_eq!(names, catalogue().iter().map(|tool| tool.name).collect());
    assert_eq!(tools.len(), catalogue().len());
    response
}

fn assert_private_paths(paths: &[&std::path::Path]) {
    for path in paths {
        assert_eq!(
            fs::metadata(path)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "protected file"))
                .permissions()
                .mode()
                & 0o777,
            0o600
        );
    }
}

#[test]
fn mcp_readiness_test_rewrite_publishes_and_recovers_exact_output() {
    let directory = tempfile::Builder::new()
        .prefix("mcp-readiness-")
        .tempdir()
        .unwrap_or_else(|error| panic!("{}: {error:?}", "private directory"));
    let root = fs::canonicalize(directory.path())
        .unwrap_or_else(|error| panic!("{}: {error:?}", "canonical root"));
    fs::set_permissions(&root, fs::Permissions::from_mode(0o700))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "private root"));
    let mut gateway = daemon_gateway::Gateway::prepare(&root);
    let endpoint = gateway.endpoint();
    let (published, binding_publisher_handle, identity, request) =
        enrol(&root, gateway.readiness(&root)).unwrap_or_else(|error| {
            panic!("{}: {error:?}", "publish through real enrolment inputs")
        });
    let original = fs::read(&published.binding)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "published bytes"));
    let token = fs::read(&published.session_token_file)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "published session token"));
    let bearer = fs::read(&published.daemon_bearer_file)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "published daemon bearer"));
    let key = fs::read(root.join("gateway-key"))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "protected gateway credential"));
    let document: Value = serde_json::from_slice(&original)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "published JSON"));
    assert_eq!(
        document["agent"]["readiness"],
        json!({
            "endpoint": endpoint,
            "gateway_key_file": root.join("gateway-key"),
            "trust_anchors": root.join("ca.pem"),
        })
    );
    assert_eq!(document["core_height"], request.core_sequence);
    assert!(document.get("core_sequence").is_none());
    assert!(!original.windows(key.len()).any(|bytes| bytes == key));
    assert_private_paths(&[
        &published.binding,
        &published.session_token_file,
        &published.daemon_bearer_file,
        &root.join("gateway-key"),
    ]);
    gateway.serve(&root, &published.binding);
    assert_eq!(
        fs::read(&published.binding)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "served publication")),
        original
    );
    gateway_authentication_required(&root, &endpoint, &published.binding);
    let first = consume_published_binding(&published.binding);
    let readiness = gateway.readiness(&root);
    drop(gateway);
    drop(binding_publisher_handle);

    let store = Store::open(root.join("store"))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "reopened production store"));
    let mut sessions = SessionRegistry::default();
    sessions
        .restore_tenant(&store, identity.tenant())
        .unwrap_or_else(|error| panic!("{}: {error:?}", "recover durable session"));
    let record = sessions
        .get(identity.tenant(), published.session_id)
        .unwrap_or_else(|| panic!("{}", "recovered session"));
    assert!(record.open);
    assert_eq!(record.generation, published.session_generation);
    let binding_publisher_handle = binding_publisher(&root, readiness);
    binding_publisher_handle
        .validate_existing(&sessions, &identity, &request)
        .unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "recover exact publication against real durable authority"
            )
        });
    drop(store);
    let gateway = daemon_gateway::Gateway::start(&root, &published.binding);
    assert_eq!(gateway.endpoint(), endpoint);
    gateway_authentication_required(&root, &endpoint, &published.binding);
    assert_eq!(consume_published_binding(&published.binding), first);
    assert_eq!(
        fs::read(&published.binding)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "recovered document")),
        original
    );
    assert_eq!(
        fs::read(&published.session_token_file)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "recovered token")),
        token
    );
    assert_eq!(
        fs::read(&published.daemon_bearer_file)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "recovered bearer")),
        bearer
    );
    assert_eq!(
        fs::read(root.join("gateway-key"))
            .unwrap_or_else(|error| panic!("{}: {error:?}", "recovered key")),
        key
    );
    drop(gateway);
}

#[test]
fn mcp_readiness_test_rewrite_refuses_unprotected_publication_credentials() {
    let directory = tempfile::Builder::new()
        .prefix("mcp-readiness-refusal-")
        .tempdir()
        .unwrap_or_else(|error| panic!("{}: {error:?}", "private directory"));
    let root = fs::canonicalize(directory.path())
        .unwrap_or_else(|error| panic!("{}: {error:?}", "canonical root"));
    fs::set_permissions(&root, fs::Permissions::from_mode(0o700))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "private root"));
    let gateway = daemon_gateway::Gateway::prepare(&root);
    fs::set_permissions(root.join("gateway-key"), fs::Permissions::from_mode(0o644))
        .unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "expose credential permissions for refusal case"
            )
        });
    let Err(error) = enrol(&root, gateway.readiness(&root)) else {
        panic!("publication must refuse an unprotected gateway credential");
    };
    assert!(
        matches!(
            error,
            EnrolmentError::InvalidSurface("the readiness gateway credential is not protected")
        ),
        "{error}"
    );
    for name in [
        enrolment::BINDING_FILE,
        enrolment::SESSION_TOKEN_FILE,
        enrolment::DAEMON_BEARER_FILE,
    ] {
        assert!(!root.join("mcp").join(name).exists());
    }
    let store = Store::open(root.join("store"))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "refused publication store"));
    let mut sessions = SessionRegistry::default();
    let tenant =
        TenantId::new("tenant-a").unwrap_or_else(|error| panic!("{}: {error:?}", "tenant"));
    sessions
        .restore_tenant(&store, &tenant)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "recover refused publication"));
    assert!(
        !sessions
            .get(&tenant, SessionId([0x0c; 32]))
            .unwrap_or_else(|| panic!("{}", "rolled-back session"))
            .open
    );
}
