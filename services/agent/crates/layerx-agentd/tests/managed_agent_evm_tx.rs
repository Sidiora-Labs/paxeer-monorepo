use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::human::HumanOperationError;
use layerx_agentd::managed_agent::restore_owner_transaction;
use layerx_agentd::outbox::Outbox;
use layerx_agentd::receipt::{serve, store as store_receipt, ReceiptLookupKey, ServedReceipt};
use layerx_agentd::store::{Store, TenantId};
use layerx_client::account_binding::{ADDR_PRECOMPILE, SELECTOR_BIND_LAYERX};
use layerx_client::paxeer_binding::Binding;
use layerx_crypto::evm_transaction::{transaction_hash, Eip1559Call};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::AuthorizedBatch;
use layerx_wire::encode::Encoder;
use layerx_wire::hash::receipt_digest;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

static NEXT: AtomicU64 = AtomicU64::new(1);
const OWNER: [u8; 20] = [
    0x7e, 0x5f, 0x45, 0x52, 0x09, 0x1a, 0x69, 0x12, 0x5d, 0x5d, 0xfc, 0xb7, 0xb8, 0xc2, 0x65, 0x90,
    0x29, 0x39, 0x5b, 0xdf,
];
const NETWORK: u32 = 125;

fn transaction(nonce: u64) -> Eip1559Call {
    let binding = Binding::new(u64::from(NETWORK), OWNER, 0).sign(&[0x42; 32]);
    lint_success(binding.verify(), "real identity consent");
    let mut word = [0; 32];
    word[31] = 64;
    let mut data = SELECTOR_BIND_LAYERX.to_vec();
    data.extend_from_slice(&binding.public_key());
    data.extend_from_slice(&word);
    data.extend_from_slice(&word);
    data.extend_from_slice(&binding.signature());
    Eip1559Call {
        chain_id: u64::from(NETWORK),
        nonce,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 10,
        gas_limit: 100_000,
        to: ADDR_PRECOMPILE,
        data,
    }
}

fn sign(call: &Eip1559Call) -> Vec<u8> {
    let mut secret = [0; 32];
    secret[31] = 1;
    lint_success(call.sign(&secret), "real owner signature")
}

fn receipt(hash: [u8; 32], signature: Option<[u8; 64]>) -> Vec<u8> {
    let mut out = Encoder::new(4096);
    lint_success(out.structure_header_version(0x5201, 2), "header");
    lint_success(out.u16(2), "protocol");
    lint_success(out.bytes(&hash, 32), "transaction hash");
    lint_success(out.u64(8), "sequence");
    for root in [[2; 32], [3; 32], [4; 32]] {
        lint_success(out.bytes(&root, 32), "root");
    }
    lint_success(out.i32(0), "success");
    lint_success(out.sequence_length(0, 512), "effects");
    lint_success(out.u128(1), "fee");
    let mut height = [0; 32];
    height[31] = 5;
    lint_success(out.bytes(&height, 32), "batch");
    lint_success(out.u16(1), "module");
    lint_success(out.u32(1), "layout");
    lint_success(out.u32(1), "parameters");
    lint_success(out.u8(1), "operation");
    lint_success(out.bytes(&[6; 32], 32), "asset");
    lint_success(out.u128(1), "amount");
    lint_success(out.bytes(&[7; 32], 32), "debit account");
    lint_success(out.u128(100), "debit before");
    lint_success(out.u128(99), "debit after");
    lint_success(out.u64(7), "nonce");
    lint_success(out.bytes(&[8; 32], 32), "credit account");
    lint_success(out.u128(0), "credit before");
    lint_success(out.u128(1), "credit after");
    lint_success(out.bytes(&[9; 32], 32), "transfers");
    lint_success(out.bytes(&[10; 32], 32), "authorization");
    lint_success(out.bytes(&[11; 32], 32), "key id");
    lint_success(out.u64(1000), "timestamp");
    lint_success(out.u8(u8::from(signature.is_some())), "signature flag");
    if let Some(signature) = signature {
        lint_success(out.bytes(&signature, 64), "signature");
    }
    out.finish()
}

struct Fixture {
    root: PathBuf,
    tenant: TenantId,
    call: Eip1559Call,
    raw: Vec<u8>,
    served: ServedReceipt,
}

impl Fixture {
    fn new(mismatched_hash: bool) -> Self {
        let root = std::env::temp_dir().join(format!(
            "layerx-managed-agent-evm-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        let tenant = lint_success(TenantId::new("managed-owner"), "tenant");
        let call = transaction(7);
        let raw = sign(&call);
        assert_eq!(raw[0], 2);
        let hash = transaction_hash(&raw);
        let mut store = lint_success(Store::open(&root), "durable store");
        assert_eq!(
            lint_success(
                Outbox::default().enqueue(&mut store, tenant.clone(), raw.clone()),
                "queue signed transaction"
            ),
            hash
        );
        let receipt_hash = if mismatched_hash {
            transaction_hash(&sign(&transaction(8)))
        } else {
            hash
        };
        let sequencer = SigningKey::from_bytes(&[0x31; 32]);
        let digest = lint_success(
            receipt_digest(&receipt(receipt_hash, None)),
            "receipt digest",
        );
        let signed = receipt(receipt_hash, Some(sequencer.sign(&digest).to_bytes()));
        let keys = [SequencerAuthorization::new(
            [11; 32],
            sequencer.verifying_key().to_bytes(),
            5,
            5,
        )];
        let batch = lint_success(
            AuthorizedBatch::from_receipt_keys(&signed, &keys, [6; 32], [2; 32], [3; 32]),
            "pinned sequencer authority",
        );
        let metadata = lint_success(
            store_receipt(&mut store, tenant.clone(), hash, &signed, &batch),
            "verify and persist managed-agent receipt",
        );
        let saved = ServedReceipt {
            canonical_bytes: signed,
            metadata,
        };
        drop(store);
        let store = lint_success(Store::open(&root), "reopen durable store");
        let served = lint_success(
            serve(&store, tenant.clone(), ReceiptLookupKey::Idempotency(hash)),
            "serve durable receipt",
        );
        assert_eq!(saved, served);
        Self {
            root,
            tenant,
            call,
            raw,
            served,
        }
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        lint_success(std::fs::remove_dir_all(&self.root), "remove test store");
    }
}

#[test]
fn managed_agent_accepts_signed_eip1559() {
    let fixture = Fixture::new(false);
    let store = lint_success(Store::open(&fixture.root), "restore durable store");
    let restored = lint_success(
        restore_owner_transaction(&store, &fixture.tenant, &fixture.served, OWNER, NETWORK),
        "managed agent accepts the owner's exact type-2 transaction",
    );
    assert_eq!(restored, fixture.call);
    assert_eq!(sign(&restored), fixture.raw);
    assert_eq!(
        transaction_hash(&fixture.raw),
        fixture.served.metadata.activity_id
    );
    for (owner, network) in [([0x22; 20], NETWORK), (OWNER, NETWORK + 1), (OWNER, 0)] {
        assert!(matches!(
            restore_owner_transaction(&store, &fixture.tenant, &fixture.served, owner, network),
            Err(HumanOperationError::Refused)
        ));
    }
}

#[test]
fn managed_agent_refuses_hash_mismatch() {
    let fixture = Fixture::new(true);
    let store = lint_success(Store::open(&fixture.root), "restore durable store");
    assert_ne!(
        transaction_hash(&fixture.raw),
        fixture.served.metadata.activity_id
    );
    assert_eq!(
        transaction_hash(&fixture.raw),
        fixture.served.metadata.idempotency_key
    );
    assert!(matches!(
        restore_owner_transaction(&store, &fixture.tenant, &fixture.served, OWNER, NETWORK),
        Err(HumanOperationError::Refused)
    ));
    let mut outbox = Outbox::default();
    lint_success(
        outbox.restore(
            &store,
            fixture.tenant.clone(),
            transaction_hash(&fixture.raw),
        ),
        "mismatch preserves the outbox",
    );
    assert_eq!(
        lint_success(
            outbox.exact_signed_bytes(transaction_hash(&fixture.raw)),
            "exact bytes"
        ),
        fixture.raw
    );
}

#[track_caller]
fn lint_success<T, E: std::fmt::Debug>(result: Result<T, E>, context: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{context}: {error:?}"),
    }
}
