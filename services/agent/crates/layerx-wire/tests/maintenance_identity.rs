use layerx_wire::hash::{
    committed_last_sequence, execution_batch_id, program_execution_batch_id,
    receipt_execution_batch_id, receipt_execution_batch_id_for_evidence,
    receipt_execution_batch_id_maintenance,
};
use layerx_wire::maintenance::decode_occupancy_maintenance;
#[path = "support/maintenance.rs"]
mod maintenance;
use layerx_wire::receipt::{decode, decode_batch_header, Receipt};
use std::fmt::Debug;

fn must<T, E: Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("{error:?}"))
}

fn fixture_field(document: &str, field: &str) -> Vec<u8> {
    let marker = format!("\"{field}\": \"");
    let value = document
        .split(&marker)
        .nth(1)
        .unwrap_or_else(|| panic!("field"))
        .split('"')
        .next()
        .unwrap_or_else(|| panic!("value"));
    value
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            let pair = must(std::str::from_utf8(pair));
            must(u8::from_str_radix(pair, 16))
        })
        .collect()
}

fn scalar_identity_fixtures() -> Vec<(Receipt, Vec<u8>)> {
    let receipt = must(decode(include_bytes!(
        "../../../../../tests/fixtures/asset/supply-pause.receipt"
    )));
    let protocol = receipt.protocol().unwrap_or_else(|| panic!("protocol"));
    assert_eq!(protocol.module_id(), 1);
    assert_eq!(protocol.activity_root(), [0; 32]);
    let checkpoint = must(layerx_types::json::parse(include_str!(
        "../../../../../tests/vectors/checkpoint/fresh.json"
    )));
    let mut bytes = must(checkpoint.hex_at("header.bytes"));
    bytes[32..40].copy_from_slice(&protocol.global_sequence().to_be_bytes());
    bytes[41..49].copy_from_slice(&protocol.global_sequence().to_be_bytes());
    let header = must(decode_batch_header(&bytes));
    assert_eq!(header.protocol_version(), protocol.protocol_version());
    let document = include_str!(
        "../../../../../services/platform/hosted/authority/tests/fixtures/real-program-deploy-receipt.json"
    );
    let archived = must(decode(&fixture_field(document, "receipt_hex")));
    let fields = archived.protocol().unwrap_or_else(|| panic!("protocol"));
    assert_eq!(fields.module_id(), 9);
    assert_eq!(fields.operation(), 0);
    assert_eq!(fields.activity_root(), [0; 32]);
    vec![
        (archived, fixture_field(document, "header_hex")),
        (receipt, bytes),
    ]
}

#[test]
fn handover_sequencer_identity_matches_native_signed_header() {
    let document = include_str!(
        "../../../../../services/platform/hosted/authority/tests/fixtures/real-program-deploy-receipt.json"
    );
    let public_key = must(fixture_field(document, "sequencer_public_key_hex").try_into());
    let header = must(decode_batch_header(&fixture_field(document, "header_hex")));
    assert_eq!(
        must(layerx_wire::handover::sequencer_id(&public_key)),
        header.sequencer_id()
    );
}

#[test]
fn historical_and_maintained_identity_select_distinct_committed_ranges() {
    for (receipt, mut header_bytes) in scalar_identity_fixtures() {
        let protocol = receipt.protocol().unwrap_or_else(|| panic!("protocol"));
        let legacy = must(decode_batch_header(&header_bytes));
        assert_eq!(
            receipt_execution_batch_id(protocol, &legacy),
            execution_batch_id(
                legacy.previous_state_root(),
                protocol.activity_id(),
                protocol.global_sequence(),
                legacy.batch_number()
            )
        );
        header_bytes[41..49].copy_from_slice(&(legacy.last_sequence() + 1).to_be_bytes());
        let header = must(decode_batch_header(&header_bytes));
        let maintenance = maintenance::maintenance_bytes(&header);
        let mut record = must(decode_occupancy_maintenance(&maintenance));
        let expected = program_execution_batch_id(
            header.previous_state_root(),
            header.activity_merkle_root(),
            header.first_sequence(),
            header.last_sequence() - 1,
            header.batch_number(),
        );
        assert_eq!(
            receipt_execution_batch_id_maintenance(protocol, &header, &record, 1),
            expected
        );
        assert_ne!(receipt_execution_batch_id(protocol, &header), expected);
        for count in [0, 2, u32::MAX] {
            assert!(
                receipt_execution_batch_id_maintenance(protocol, &header, &record, count).is_err()
            );
        }
        record.global_sequence += 1;
        assert!(receipt_execution_batch_id_maintenance(protocol, &header, &record, 1).is_err());
        record.global_sequence -= 1;
        header_bytes[41..49].copy_from_slice(&(header.last_sequence() + 1).to_be_bytes());
        let changed = must(decode_batch_header(&header_bytes));
        assert!(receipt_execution_batch_id_maintenance(protocol, &changed, &record, 1).is_err());
    }
}

const PINNED_PREVIOUS_STATE_ROOT: [u8; 32] = [0x11; 32];
const PINNED_ACTIVITY_ID: [u8; 32] = [0x22; 32];
const PINNED_ACTIVITY_MERKLE_ROOT: [u8; 32] = [0x33; 32];
const PINNED_GLOBAL_SEQUENCE: u64 = 7;
const PINNED_BATCH_NUMBER: u64 = 5;
const PINNED_FIRST_SEQUENCE: u64 = 7;
const PINNED_COMMITTED_LAST_SEQUENCE: u64 = 9;
const PINNED_ACTIVITY_IDENTITY: [u8; 32] = [
    0x7e, 0xa6, 0x30, 0x2b, 0x68, 0x1e, 0x3b, 0xac, 0x3a, 0x66, 0x6c, 0xa0, 0xa6, 0xba, 0xe2, 0x43,
    0xe5, 0xbe, 0x6b, 0x2d, 0xe6, 0x3e, 0xaf, 0x8a, 0xbb, 0xb0, 0x72, 0xa3, 0x9e, 0xb5, 0x2d, 0xac,
];
const PINNED_COMMITTED_IDENTITY: [u8; 32] = [
    0x1a, 0x4e, 0xff, 0x4b, 0x89, 0xf4, 0xcb, 0x83, 0x0e, 0x37, 0xa5, 0xa4, 0x15, 0x0b, 0x4a, 0x84,
    0x29, 0xdb, 0x11, 0x60, 0x41, 0x45, 0xa8, 0x40, 0x28, 0xdc, 0xa1, 0xdd, 0xee, 0xf8, 0x49, 0xf3,
];

#[test]
fn both_preimage_forms_reproduce_the_identifiers_the_native_source_pins() {
    assert_eq!(
        must(execution_batch_id(
            PINNED_PREVIOUS_STATE_ROOT,
            PINNED_ACTIVITY_ID,
            PINNED_GLOBAL_SEQUENCE,
            PINNED_BATCH_NUMBER
        )),
        PINNED_ACTIVITY_IDENTITY
    );
    assert_eq!(
        must(program_execution_batch_id(
            PINNED_PREVIOUS_STATE_ROOT,
            PINNED_ACTIVITY_MERKLE_ROOT,
            PINNED_FIRST_SEQUENCE,
            PINNED_COMMITTED_LAST_SEQUENCE,
            PINNED_BATCH_NUMBER
        )),
        PINNED_COMMITTED_IDENTITY
    );
    assert_ne!(PINNED_ACTIVITY_IDENTITY, PINNED_COMMITTED_IDENTITY);
}

#[test]
fn the_committed_range_excludes_only_a_published_maintenance_sequence() {
    assert_eq!(must(committed_last_sequence(4, 9, false)), 9);
    assert_eq!(must(committed_last_sequence(4, 9, true)), 8);
    assert_eq!(must(committed_last_sequence(4, 4, false)), 4);
    assert!(committed_last_sequence(4, 4, true).is_err());
    assert!(committed_last_sequence(0, 4, false).is_err());
    assert!(committed_last_sequence(5, 4, false).is_err());
}

#[test]
fn the_evidence_class_picks_the_selector_the_daemon_used() {
    for (receipt, mut bytes) in scalar_identity_fixtures() {
        let protocol = receipt.protocol().unwrap_or_else(|| panic!("protocol"));
        let historical = must(decode_batch_header(&bytes));
        assert_eq!(
            receipt_execution_batch_id_for_evidence(protocol, &historical, None),
            receipt_execution_batch_id(protocol, &historical)
        );
        bytes[41..49].copy_from_slice(&(historical.last_sequence() + 1).to_be_bytes());
        let maintained = must(decode_batch_header(&bytes));
        let maintenance_bytes = maintenance::maintenance_bytes(&maintained);
        let record = must(decode_occupancy_maintenance(&maintenance_bytes));
        assert_eq!(
            receipt_execution_batch_id_for_evidence(protocol, &maintained, Some(&record)),
            receipt_execution_batch_id_maintenance(protocol, &maintained, &record, 1)
        );
        assert_ne!(
            receipt_execution_batch_id_for_evidence(protocol, &maintained, Some(&record)),
            receipt_execution_batch_id_for_evidence(protocol, &maintained, None)
        );
    }
}

#[test]
fn programs_identity_requires_the_committed_activity_root() {
    let receipt = must(decode(include_bytes!(
        "../../../../../tests/fixtures/programs/module-maintained-call/receipt"
    )));
    let protocol = receipt.protocol().unwrap_or_else(|| panic!("protocol"));
    assert_eq!(protocol.module_id(), 9);
    assert_eq!(protocol.operation(), 3);
    assert_ne!(protocol.activity_root(), [0; 32]);
    let mut bytes =
        include_bytes!("../../../../../tests/fixtures/programs/module-maintained-call/header")
            .to_vec();
    let header = must(decode_batch_header(&bytes));
    assert_eq!(protocol.activity_root(), header.activity_merkle_root());
    let maintenance = must(layerx_wire::batch_maintenance::decode_maintenance(
        include_bytes!(
            "../../../../../tests/fixtures/programs/module-maintained-call/maintenance.receipt"
        ),
    ));
    must(maintenance.verify_header(&header));
    let count = must(u32::try_from(
        header.last_sequence() - header.first_sequence(),
    ));
    let record = maintenance.occupancy();
    let maintained_id = must(program_execution_batch_id(
        header.previous_state_root(),
        header.activity_merkle_root(),
        header.first_sequence(),
        header.last_sequence() - 1,
        header.batch_number(),
    ));
    assert_eq!(
        receipt_execution_batch_id_maintenance(protocol, &header, record, count),
        Ok(maintained_id)
    );
    assert_eq!(
        receipt_execution_batch_id_for_evidence(protocol, &header, Some(record)),
        Ok(maintained_id)
    );
    let full_range_id = must(program_execution_batch_id(
        header.previous_state_root(),
        header.activity_merkle_root(),
        header.first_sequence(),
        header.last_sequence(),
        header.batch_number(),
    ));
    assert_eq!(
        receipt_execution_batch_id(protocol, &header),
        Ok(full_range_id)
    );
    assert_eq!(
        receipt_execution_batch_id_for_evidence(protocol, &header, None),
        Ok(full_range_id)
    );
    let expected = Err(layerx_wire::WireError {
        result: layerx_types::result::KnownResult::NonCanonical.into(),
        offset: 0,
    });
    for root in [[0; 32], [0xa5; 32]] {
        assert_ne!(root, protocol.activity_root());
        bytes[128..160].copy_from_slice(&root);
        let changed = must(decode_batch_header(&bytes));
        assert_eq!(changed.activity_merkle_root(), root);
        assert_eq!(receipt_execution_batch_id(protocol, &changed), expected);
        assert_eq!(
            receipt_execution_batch_id_for_evidence(protocol, &changed, None),
            expected
        );
        assert_eq!(
            receipt_execution_batch_id_maintenance(protocol, &changed, record, count),
            expected
        );
        assert_eq!(
            receipt_execution_batch_id_for_evidence(protocol, &changed, Some(record)),
            expected
        );
    }
}
