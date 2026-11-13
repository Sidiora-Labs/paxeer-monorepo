use super::gateway::{must, Server};
use layerx_client::ai_market::{AiClientError, OperationJournal, OperationRecord, OperationState};
use layerx_client::rpc::receipt::{LookupContext, ReceiptError};
use layerx_client::sequencer::decode_receipt_keys;
use layerx_types::payload::ModuleRegistry;
use serde_json::{json, Value};
use std::sync::atomic::{AtomicU64, Ordering};
static NEXT: AtomicU64 = AtomicU64::new(0);
struct Directory(std::path::PathBuf);
impl Drop for Directory {
    fn drop(&mut self) {
        must(std::fs::remove_dir_all(&self.0));
    }
}
fn unhex(value: &Value) -> Vec<u8> {
    (match value.as_str() {
        Some(required_value) => required_value,
        None => panic!("hex"),
    })
    .trim_start_matches("0x")
    .as_bytes()
    .chunks_exact(2)
    .map(|p| must(u8::from_str_radix(must(std::str::from_utf8(p)), 16)))
    .collect()
}
pub fn producer_receipts_survive_reopen_and_refuse_missing_payment_denom() {
    let fixture: Value = must(serde_json::from_str(include_str!(
        "../../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )));
    assert_eq!(
        fixture["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    let registry = must(ModuleRegistry::new(&[]));
    // Value-bearing transfers remain outside the client's zero-value program-call type.
    assert_eq!(
        OperationRecord::from_signed_transaction(
            unhex(&fixture["cases"][0]["rawTransaction"]),
            [0x55; 32],
            10_000
        ),
        Err(AiClientError::Transaction(
            layerx_crypto::evm_transaction::EvmTransactionError::UnsupportedField
        ))
    );
    // Both successful and reverted producer executions use the exact recorded bytes and view.
    for index in [1, 5] {
        check_receipt_case(&fixture["cases"][index], index, &registry);
    }
}

fn check_receipt_case(case: &Value, index: usize, registry: &ModuleRegistry) {
    let view = case["receiptJson"].clone();
    assert_eq!(view["paymentDenom"], "uhpx");
    let pins = must(decode_receipt_keys(&json!({"keys":[case["pinnedKey"]]})));
    let raw = unhex(&case["rawTransaction"]);
    let id = layerx_crypto::evm_transaction::transaction_hash(&raw);
    assert_eq!(id.as_slice(), unhex(&view["txHash"]));
    let directory = Directory(std::env::temp_dir().join(format!(
        "repair-1546-receipt-{}-{}",
        std::process::id(),
        NEXT.fetch_add(1, Ordering::Relaxed)
    )));
    let journal = must(OperationJournal::open(&directory.0));
    let mut record = must(OperationRecord::from_signed_transaction(
        raw.clone(),
        [0x55; 32],
        10_000,
    ));
    must(journal.record_signed(&record));
    let response_body = view.clone();
    let signed = raw.clone();
    let server = Server::start(move |request| {
        match match request["method"].as_str() {
            Some(required_value) => required_value,
            None => panic!("method"),
        } {
            "eth_sendRawTransaction" => {
                assert_eq!(unhex(&request["params"][0]), signed);
                json!({"result":response_body["txHash"]})
            }
            "pax_sequencerReceipt" => {
                assert_eq!(request["params"], json!([response_body["txHash"]]));
                json!({"result":response_body})
            }
            "eth_getTransactionReceipt" => json!({"result":null}),
            other => panic!("unexpected {other}"),
        }
    });
    assert_eq!(
        must(journal.submit(&mut record, &server.sequencer())),
        OperationState::Pending
    );
    refuse_changed_payment_denoms(&journal, &mut record, &view, &pins, &id, registry);
    let expected = if index == 1 {
        OperationState::Executed
    } else {
        OperationState::Failed
    };
    assert_eq!(
        must(journal.resolve(
            &mut record,
            &server.sequencer(),
            LookupContext {
                receipt_keys: &pins
            }
        )),
        expected
    );
    assert_eq!(
        record.global_sequence(),
        Some(if index == 1 { 2 } else { 6 })
    );
    assert_eq!(
        record.execution_height(),
        Some(if index == 1 { 1000 } else { 1002 })
    );
    assert_eq!(record.result_code(), Some(i32::from(index != 1)));
    assert_eq!(record.rung(), Some(layerx_types::rung::Rung::Receipted));
    let encoded = must(record.encode());
    drop(record);
    drop(journal);
    drop(server);
    let journal = must(OperationJournal::open(&directory.0));
    let restored = must(journal.load(&id, registry));
    assert_eq!(restored.state(), expected);
    assert_eq!(restored.signed_bytes(), raw);
    assert_eq!(must(restored.encode()), encoded);
    let mut damaged = encoded.clone();
    damaged.pop();
    must(std::fs::write(journal.record_path(&id), damaged));
    assert!(journal.load(&id, registry).is_err());
}

fn refuse_changed_payment_denoms(
    journal: &OperationJournal,
    record: &mut OperationRecord,
    view: &Value,
    pins: &[layerx_client::sequencer::ReceiptKeyEntry],
    id: &[u8; 32],
    registry: &ModuleRegistry,
) {
    let mut missing = view.clone();
    (match missing.as_object_mut() {
        Some(required_value) => required_value,
        None => panic!("view"),
    })
    .remove("paymentDenom");
    let missing_server = Server::start(move |request| {
        assert_eq!(request["method"], "pax_sequencerReceipt");
        assert_eq!(request["params"], json!([missing["txHash"]]));
        json!({"result":missing})
    });
    assert_eq!(
        journal.resolve(
            record,
            &missing_server.sequencer(),
            LookupContext { receipt_keys: pins }
        ),
        Err(AiClientError::Receipt(ReceiptError::Sequencer(
            layerx_client::sequencer::SequencerError::Malformed
        )))
    );
    assert_eq!(record.state(), OperationState::Pending);
    assert_eq!(must(journal.load(id, registry)), *record);
    let mut changed = view.clone();
    changed["paymentDenom"] = json!("another-denom");
    let changed_server = Server::start(move |_| json!({"result":changed}));
    assert!(journal
        .resolve(
            record,
            &changed_server.sequencer(),
            LookupContext { receipt_keys: pins }
        )
        .is_err());
    assert_eq!(record.state(), OperationState::Pending);
}
