use std::fs;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_client::sequencer::{ExecutedReceipt, SequencerReceipt};
use layerx_ramp_toolkit::journal::{Journal, PaxeerObservation};
use layerx_ramp_toolkit::RampError;
use layerx_types::rung::Rung;

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(0);
const IDEMPOTENCY_KEY: [u8; 32] = [5; 32];
const TRANSACTION_HASH: [u8; 32] = [6; 32];
const BLOCK_HASH: [u8; 32] = [7; 32];
const OPERATION: &str = "custody-operation-1";

struct JournalDirectory(PathBuf);

impl JournalDirectory {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "ramp-custody-rungs-{}-{}",
            std::process::id(),
            NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path).expect("create journal directory");
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt as _;
            fs::set_permissions(&path, fs::Permissions::from_mode(0o700))
                .expect("protect journal directory");
        }
        Self(path)
    }

    fn path(&self) -> PathBuf {
        self.0.join("journal.jsonl")
    }
}

impl Drop for JournalDirectory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).expect("remove journal directory");
    }
}

fn executed_receipt() -> ExecutedReceipt {
    ExecutedReceipt {
        tx_hash: TRANSACTION_HASH,
        nonce: 9,
        seq: 12,
        height: 42,
        index: 0,
        timestamp: 1_000,
        success: true,
        return_data: Vec::new(),
        logs: Vec::new(),
        gas_used: 21_000,
        cumulative_gas_used: 21_000,
        effective_gas_price: 1,
        fee_denom: "upax".to_owned(),
        fee_amount: {
            let mut amount = [0; 32];
            amount[30..].copy_from_slice(&21_000_u16.to_be_bytes());
            amount
        },
        sponsor: [0; 20],
        from: [1; 20],
        to: [2; 20],
        contract_address: [0; 20],
        vm_error: String::new(),
        key_id: [3; 32],
        receipt: Vec::new(),
    }
}

#[test]
fn custody_observation_records_committed_rung() {
    let directory = JournalDirectory::new();
    let path = directory.path();
    let mut journal = Journal::open(&path).expect("open journal");
    journal
        .plan_paxeer(IDEMPOTENCY_KEY, [1; 32], 10, 1)
        .expect("plan custody transfer");
    let receipts = [
        SequencerReceipt::Submitted {
            tx_hash: TRANSACTION_HASH,
            nonce: 9,
        },
        SequencerReceipt::Receipted(executed_receipt()),
        SequencerReceipt::Committed {
            receipt: executed_receipt(),
            block_number: 42,
            block_hash: BLOCK_HASH,
        },
    ];
    for (receipt, expected) in
        receipts
            .iter()
            .zip([Rung::Submitted, Rung::Receipted, Rung::Committed])
    {
        let observation = PaxeerObservation::from_sequencer_receipt(OPERATION, receipt);
        assert_eq!(observation.rung, expected);
        journal
            .observe_paxeer(IDEMPOTENCY_KEY, observation, 2)
            .expect("record sequencer receipt");
        let snapshot = journal.paxeer(&IDEMPOTENCY_KEY).expect("custody snapshot");
        assert_eq!(snapshot.rung, Some(expected));
        assert_eq!(snapshot.transaction_hash, Some(TRANSACTION_HASH));
        assert_eq!(snapshot.operation_id.as_deref(), Some(OPERATION));
        assert_eq!(snapshot.stage, expected.name());
    }
    let committed = journal
        .paxeer(&IDEMPOTENCY_KEY)
        .expect("committed snapshot")
        .clone();
    assert_eq!(committed.rung, Some(Rung::Committed));
    assert_eq!(committed.block_hash, Some(BLOCK_HASH));
    assert_eq!(committed.block_height, Some(42));
    assert_eq!(journal.record_count(), 4);
    drop(journal);
    let reopened = Journal::open(&path).expect("replay custody journal");
    assert_eq!(reopened.paxeer(&IDEMPOTENCY_KEY), Some(&committed));
}

#[test]
fn custody_observation_never_regresses_rung() {
    let directory = JournalDirectory::new();
    let path = directory.path();
    let mut journal = Journal::open(&path).expect("open journal");
    journal
        .plan_paxeer(IDEMPOTENCY_KEY, [1; 32], 10, 1)
        .expect("plan custody transfer");
    let committed = SequencerReceipt::Committed {
        receipt: executed_receipt(),
        block_number: 42,
        block_hash: BLOCK_HASH,
    };
    journal
        .observe_paxeer(
            IDEMPOTENCY_KEY,
            PaxeerObservation::from_sequencer_receipt(OPERATION, &committed),
            2,
        )
        .expect("record committed custody transfer");
    let snapshot = journal
        .paxeer(&IDEMPOTENCY_KEY)
        .expect("committed snapshot")
        .clone();
    let projection = journal.projection().clone();
    let head = journal.head();
    let count = journal.record_count();
    let bytes = fs::read(&path).expect("read committed journal");
    let receipted = SequencerReceipt::Receipted(executed_receipt());
    assert_eq!(
        journal.observe_paxeer(
            IDEMPOTENCY_KEY,
            PaxeerObservation::from_sequencer_receipt(OPERATION, &receipted),
            3
        ),
        Err(RampError::Conflict)
    );
    assert_eq!(journal.paxeer(&IDEMPOTENCY_KEY), Some(&snapshot));
    assert_eq!(
        journal
            .paxeer(&IDEMPOTENCY_KEY)
            .expect("retained snapshot")
            .rung,
        Some(Rung::Committed)
    );
    assert_eq!(journal.projection(), &projection);
    assert_eq!(journal.head(), head);
    assert_eq!(journal.record_count(), count);
    assert_eq!(fs::read(&path).expect("read journal after refusal"), bytes);
    drop(journal);
    let reopened = Journal::open(&path).expect("replay unchanged committed journal");
    assert_eq!(reopened.paxeer(&IDEMPOTENCY_KEY), Some(&snapshot));
}
