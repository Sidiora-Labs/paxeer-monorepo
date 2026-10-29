use layerx_agent_api::error::RequestId;
use layerx_agentd::agent_rpc::dispatch_target;
use layerx_agentd::agent_rpc_dispatch::{agent_register, session_open};
use layerx_agentd::prepare::evm::{AbiArgument, PrecompileCall, PrecompileTarget};
use layerx_agentd::session::{SessionCredential, SessionId};
use layerx_agentd::session_keys::SessionKeyRegistry;
use layerx_agentd::store::TenantId;
use layerx_agentd::tenant::{
    require_session_credential, AuthorizationError, Operation, OperationClass,
};
use layerx_client::account_binding::{plan_bind, BindFees, BindPlan, ADDR_PRECOMPILE};
use layerx_crypto::account_derivation::derive_from_mnemonic;
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_crypto::session::{
    decode_session_key, issue_session_key, SessionKeyRequest, SessionPurpose,
};
use layerx_types::payload::{ActivityType, ModuleId};
use std::fs;
use std::os::unix::fs::{MetadataExt as _, PermissionsExt as _};
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

static NEXT: AtomicU64 = AtomicU64::new(1);

fn fees() -> BindFees {
    BindFees {
        evm_nonce: 4,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 10,
        gas_limit: 200_000,
    }
}

struct RegistryDirectory(PathBuf);

impl RegistryDirectory {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "layerx-bootstrap-bind-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path).expect("new session registry directory");
        fs::set_permissions(&path, fs::Permissions::from_mode(0o700))
            .expect("protected session registry directory");
        Self(path)
    }

    fn open(&self) -> SessionKeyRegistry {
        SessionKeyRegistry::open(
            self.0.clone(),
            vec![0x61; 32],
            77,
            fs::metadata(&self.0).expect("registry metadata").uid(),
        )
        .expect("real encrypted session registry")
    }
}

impl Drop for RegistryDirectory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).expect("remove temporary session registry");
    }
}

#[test]
fn agent_register_prepares_bind_layerx() {
    let account = derive_from_mnemonic(
        "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about",
        "",
        0,
    )
    .expect("real derived account pair");
    let prepared = agent_register(&account, 125, 0, fees(), RequestId(1))
        .expect("prepare real signed bindLayerX transaction");
    assert_eq!(prepared.transaction.to, ADDR_PRECOMPILE);
    assert_eq!(&prepared.transaction.data[..4], &[0xdd, 0x9a, 0xa6, 0x28]);
    assert_eq!(prepared.bound_address, account.evm_address());
    let (transaction, signature) =
        decode_signed(&prepared.signed_transaction).expect("decode prepared binding transaction");
    assert_eq!(transaction, prepared.transaction);
    assert_eq!(
        recover_sender(&transaction, &signature).expect("binding signer"),
        account.evm_address()
    );
    assert_eq!(
        prepared.tx_hash,
        transaction_hash(&prepared.signed_transaction)
    );
    let BindPlan::Bind(call) = plan_bind(&account, 125, None, 0).expect("binding preparation")
    else {
        panic!("an unbound address must prepare bindLayerX");
    };
    call.signed_binding()
        .verify()
        .expect("real binding consent");
    assert_eq!(
        call.signed_binding().binding().evm_address(),
        account.evm_address()
    );
    assert_eq!(&call.data()[..4], &[0xdd, 0x9a, 0xa6, 0x28]);
    assert_eq!(&call.data()[4..36], &account.layerx_public_key());
    assert_eq!(
        call.to(),
        ADDR_PRECOMPILE,
        "9031.1 requires a real bindLayerX preparation targeting the addr precompile at 0x1004"
    );
    assert_eq!(prepared.transaction.data, call.data());
    let credential = SessionCredential::new(
        TenantId::new("bootstrap-bind").expect("tenant"),
        SessionId([1; 32]),
        [3; 32],
        1,
    );
    assert_eq!(
        OperationClass::for_operation(Operation::AgentRegister),
        Some(OperationClass::Prepare)
    );
    assert_eq!(
        require_session_credential(Operation::AgentRegister, Some(&credential)),
        Ok(())
    );
    assert_eq!(
        require_session_credential(Operation::AgentRegister, None),
        Err(AuthorizationError::BearerOnlyCatalogueWrite)
    );
    assert_eq!(
        dispatch_target(Operation::AgentRegister, RequestId(1)),
        Ok(Operation::AgentRegister),
        "agent.register must reach its adapter"
    );
}

#[test]
fn session_open_prepares_grant_session_for_the_session_address() {
    let directory = RegistryDirectory::new();
    let registry = directory.open();
    let prepared = registry
        .prepare_seed(b"bootstrap-bind/session", [7; 32])
        .expect("durable session seed preparation");
    let address = prepared.evm_address();
    let mut session_public_key = [0; 32];
    session_public_key[12..].copy_from_slice(&address);
    let issued = issue_session_key(&SessionKeyRequest {
        grantor: [0x31; 32],
        session_public_key,
        not_before: 100,
        expires_at: Some(1000),
        permitted_activity_types: vec![
            ActivityType::new(ModuleId::Asset, 5).expect("asset send activity")
        ],
        revocation_sequence: Some(3),
        fee_budget: None,
        purpose: SessionPurpose::Activity,
    })
    .expect("canonical protocol session grant");
    registry
        .provision(issued.grant_id, prepared.secret(), &issued)
        .expect("provision real secp256k1 session key");
    let loaded = registry
        .load(issued.grant_id, issued.clone())
        .expect("load provisioned session key");
    assert_eq!(loaded.evm_address(), address);

    let owner = derive_from_mnemonic(
        "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about",
        "",
        0,
    ).expect("real grant transaction owner");
    let transaction = session_open(
        &owner,
        &registry,
        &issued,
        125,
        100,
        [7; 32],
        fees(),
        RequestId(2),
    )
    .expect("prepare grantSession with a provisioned signer");
    assert_eq!(transaction.transaction.to, PrecompileTarget::Auth.address());
    assert_eq!(transaction.bound_address, address);
    assert_eq!(
        &transaction.transaction.data[..4],
        &[0x11, 0x07, 0xb4, 0x9e]
    );
    let prepared_grant = decode_session_key(
        &transaction.transaction.data[132..132 + issued.registration_payload.len()],
    )
    .expect("canonical grant in prepared transaction");
    assert_eq!(&prepared_grant.session_public_key[12..], &address);
    assert_eq!(prepared_grant.grant_id, issued.grant_id);
    let (decoded, signature) =
        decode_signed(&transaction.signed_transaction).expect("decode prepared grant transaction");
    assert_eq!(decoded, transaction.transaction);
    assert_eq!(
        recover_sender(&decoded, &signature).expect("grant signer"),
        owner.evm_address()
    );
    assert_eq!(
        transaction.tx_hash,
        transaction_hash(&transaction.signed_transaction)
    );

    let mut expiry_sequence = [0; 32];
    expiry_sequence[24..].copy_from_slice(&100_u64.to_be_bytes());
    let call = PrecompileCall {
        target: PrecompileTarget::Auth,
        selector: [0x11, 0x07, 0xb4, 0x9e],
        arguments: vec![
            AbiArgument::Bytes(issued.registration_payload.clone()),
            AbiArgument::Word(expiry_sequence),
            AbiArgument::Word([7; 32]),
        ],
    };
    let calldata = call.encode().expect("real grantSession ABI encoder");
    assert_eq!(call.target.address()[18..], [0x10, 0x1c]);
    assert_eq!(&calldata[..4], &[0x11, 0x07, 0xb4, 0x9e]);
    let grant = decode_session_key(&calldata[132..132 + issued.registration_payload.len()])
        .expect("grantSession carries the canonical session grant");
    assert_eq!(&grant.session_public_key[12..], &loaded.evm_address());
    assert_eq!(grant.grant_id, issued.grant_id);

    let credential = SessionCredential::new(
        TenantId::new("bootstrap-bind").expect("tenant"),
        SessionId([2; 32]),
        [3; 32],
        1,
    );
    assert_eq!(
        require_session_credential(Operation::SessionOpen, Some(&credential)),
        Ok(()),
        "9031.2 requires the authenticated session.open request to reach preparation"
    );
    assert_eq!(
        OperationClass::for_operation(Operation::SessionOpen),
        Some(OperationClass::Prepare)
    );
    assert_eq!(
        require_session_credential(Operation::SessionOpen, None),
        Err(AuthorizationError::BearerOnlyCatalogueWrite)
    );
    assert_eq!(OperationClass::for_operation(Operation::McpInvoke), None);
    assert_eq!(
        dispatch_target(Operation::SessionOpen, RequestId(2)),
        Ok(Operation::SessionOpen),
        "session.open must reach its adapter"
    );
}
