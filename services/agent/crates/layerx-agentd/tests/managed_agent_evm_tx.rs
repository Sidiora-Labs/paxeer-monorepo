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
    binding.verify().expect("real identity consent");
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
    call.sign(&secret).expect("real owner signature")
}

fn receipt(hash: [u8; 32], signature: Option<[u8; 64]>) -> Vec<u8> {
    let mut out = Encoder::new(4096);
    out.structure_header_version(0x5201, 2).expect("header");
    out.u16(2).expect("protocol");
    out.bytes(&hash, 32).expect("transaction hash");
    out.u64(8).expect("sequence");
    for root in [[2; 32], [3; 32], [4; 32]] {
        out.bytes(&root, 32).expect("root");
    }
    out.i32(0).expect("success");
    out.sequence_length(0, 512).expect("effects");
    out.u128(1).expect("fee");
    let mut height = [0; 32];
    height[31] = 5;
    out.bytes(&height, 32).expect("batch");
    out.u16(1).expect("module");
    out.u32(1).expect("layout");
    out.u32(1).expect("parameters");
    out.u8(1).expect("operation");
    out.bytes(&[6; 32], 32).expect("asset");
    out.u128(1).expect("amount");
    out.bytes(&[7; 32], 32).expect("debit account");
    out.u128(100).expect("debit before");
    out.u128(99).expect("debit after");
    out.u64(7).expect("nonce");
    out.bytes(&[8; 32], 32).expect("credit account");
    out.u128(0).expect("credit before");
    out.u128(1).expect("credit after");
    out.bytes(&[9; 32], 32).expect("transfers");
    out.bytes(&[10; 32], 32).expect("authorization");
    out.bytes(&[11; 32], 32).expect("key id");
    out.u64(1000).expect("timestamp");
    out.u8(u8::from(signature.is_some()))
        .expect("signature flag");
    if let Some(signature) = signature {
        out.bytes(&signature, 64).expect("signature");
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
        let tenant = TenantId::new("managed-owner").expect("tenant");
        let call = transaction(7);
        let raw = sign(&call);
        assert_eq!(raw[0], 2);
        let hash = transaction_hash(&raw);
        let mut store = Store::open(&root).expect("durable store");
        assert_eq!(
            Outbox::default()
                .enqueue(&mut store, tenant.clone(), raw.clone())
                .expect("queue signed transaction"),
            hash
        );
        let receipt_hash = if mismatched_hash {
            transaction_hash(&sign(&transaction(8)))
        } else {
            hash
        };
        let sequencer = SigningKey::from_bytes(&[0x31; 32]);
        let digest = receipt_digest(&receipt(receipt_hash, None)).expect("receipt digest");
        let signed = receipt(receipt_hash, Some(sequencer.sign(&digest).to_bytes()));
        let keys = [SequencerAuthorization::new(
            [11; 32],
            sequencer.verifying_key().to_bytes(),
            5,
            5,
        )];
        let batch = AuthorizedBatch::from_receipt_keys(&signed, &keys, [6; 32], [2; 32], [3; 32])
            .expect("pinned sequencer authority");
        let metadata = store_receipt(&mut store, tenant.clone(), hash, &signed, &batch)
            .expect("verify and persist managed-agent receipt");
        let saved = ServedReceipt {
            canonical_bytes: signed,
            metadata,
        };
        drop(store);
        let store = Store::open(&root).expect("reopen durable store");
        let served = serve(&store, tenant.clone(), ReceiptLookupKey::Idempotency(hash))
            .expect("serve durable receipt");
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
        std::fs::remove_dir_all(&self.root).expect("remove test store");
    }
}

#[test]
fn managed_agent_accepts_signed_eip1559() {
    let fixture = Fixture::new(false);
    let store = Store::open(&fixture.root).expect("restore durable store");
    let restored =
        restore_owner_transaction(&store, &fixture.tenant, &fixture.served, OWNER, NETWORK)
            .expect("managed agent accepts the owner's exact type-2 transaction");
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
    let store = Store::open(&fixture.root).expect("restore durable store");
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
    outbox
        .restore(
            &store,
            fixture.tenant.clone(),
            transaction_hash(&fixture.raw),
        )
        .expect("mismatch preserves the outbox");
    assert_eq!(
        outbox
            .exact_signed_bytes(transaction_hash(&fixture.raw))
            .expect("exact bytes"),
        fixture.raw
    );
}
