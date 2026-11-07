use layerx_types::chain::{BlockReference, ValidatorSetTrust};
use layerx_types::historical::settlement::DeclaredCheckpointSettlement;
use layerx_types::historical::verify::HistoricalVerificationLevel;
use layerx_types::intent::ProgramId;
use layerx_types::limits::MAX_PAYLOAD_BYTES;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry, Payload};
use layerx_types::program_lifecycle::{
    NativeProgramDeploy, NativeProgramUpgrade, ProgramUpgradePolicy,
};
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;

const TRUST: &str = r#"{"schema":"layerx/chain-trust/1","chain_id":125,"comet_chain_id":"hyperpax_125-1","height":19,"validator_set_hash":"0x2222222222222222222222222222222222222222222222222222222222222222"}"#;
const ARCHIVE: &str = include_str!("../../../../../contracts/config/checkpoint-settlement.json");

// Emit valid WASM with one custom section, including its real LEB128 framing.
fn wasm(size: usize) -> Vec<u8> {
    let mut bytes = b"\0asm\x01\0\0\0".to_vec();
    for width in 1..=5 {
        let content = size - 9 - width;
        let mut length = content;
        let mut leb = Vec::new();
        loop {
            let byte = (length & 0x7f) as u8;
            length >>= 7;
            leb.push(byte | if length == 0 { 0 } else { 0x80 });
            if length == 0 {
                break;
            }
        }
        if leb.len() == width {
            bytes.push(0);
            bytes.extend_from_slice(&leb);
            // Empty custom-section name followed by its uninterpreted contents.
            bytes.resize(size, 0);
            return bytes;
        }
    }
    panic!("WASM section length is not representable");
}

fn registry() -> ModuleRegistry {
    let activities = [
        ActivityType::new(ModuleId::Programs, 1).unwrap(),
        ActivityType::new(ModuleId::Programs, 2).unwrap(),
    ];
    ModuleRegistry::new(&[ModuleRegistration::new(ModuleId::Programs, &activities).unwrap()])
        .unwrap()
}

#[test]
fn repair_f12_deploy_payload_matches_native_admission() {
    let registry = registry();
    for interface in [None, Some(&b"interface"[..]), Some(&[7; 952][..])] {
        let overhead = 104 + interface.map_or(0, |value| 4 + value.len());
        let code = wasm(MAX_PAYLOAD_BYTES - overhead);
        let deploy = NativeProgramDeploy {
            program_id: ProgramId::new([1; 32]),
            guest_abi: 1,
            policy: ProgramUpgradePolicy::Immutable,
            new_hash: [3; 32],
            interface,
            wasm: &code,
        };
        let encoded = deploy.encode().unwrap();
        assert_eq!(encoded.len(), MAX_PAYLOAD_BYTES);
        let admitted = Payload::new(
            &registry,
            ActivityType::new(ModuleId::Programs, 1).unwrap(),
            &encoded,
        )
        .unwrap();
        let Payload::Programs(_, transported) = admitted else {
            panic!("wrong native module");
        };
        assert_eq!(NativeProgramDeploy::decode(&transported).unwrap(), deploy);
        assert_eq!(
            NativeProgramDeploy::decode(&transported)
                .unwrap()
                .encode()
                .unwrap(),
            encoded
        );
        let over_code = wasm(code.len() + 1);
        assert!(NativeProgramDeploy {
            wasm: &over_code,
            ..deploy
        }
        .encode()
        .is_err());
        let mut over = encoded;
        let length = u32::try_from(over_code.len()).unwrap();
        over[100..104].copy_from_slice(&length.to_be_bytes());
        over.truncate(overhead);
        over.extend_from_slice(&over_code);
        assert_eq!(over.len(), MAX_PAYLOAD_BYTES + 1);
        assert!(Payload::new(
            &registry,
            ActivityType::new(ModuleId::Programs, 1).unwrap(),
            &over
        )
        .is_err());
        assert!(NativeProgramDeploy::decode(&over).is_err());
    }
}

#[test]
fn repair_f12_upgrade_budgets_hook_interface_and_clear_marker() {
    let registry = registry();
    for hook in [&b""[..], &b"migrate"[..], &[9; 65_535][..]] {
        for (interface, clear_interface) in [
            (None, false),
            (Some(&b"interface"[..]), false),
            (Some(&b""[..]), true),
        ] {
            let overhead = 106 + hook.len() + interface.map_or(0, |value| 4 + value.len());
            let code = wasm(MAX_PAYLOAD_BYTES - overhead);
            let upgrade = NativeProgramUpgrade {
                program_id: ProgramId::new([1; 32]),
                guest_abi: 1,
                old_hash: [2; 32],
                new_hash: [3; 32],
                migration_hook: hook,
                clear_interface,
                interface,
                wasm: &code,
            };
            let encoded = upgrade.encode().unwrap();
            assert_eq!(encoded.len(), MAX_PAYLOAD_BYTES);
            let admitted = Payload::new(
                &registry,
                ActivityType::new(ModuleId::Programs, 2).unwrap(),
                &encoded,
            )
            .unwrap();
            let Payload::Programs(_, transported) = admitted else {
                panic!("wrong native module");
            };
            assert_eq!(NativeProgramUpgrade::decode(&transported).unwrap(), upgrade);
            let over_code = wasm(code.len() + 1);
            assert!(NativeProgramUpgrade {
                wasm: &over_code,
                ..upgrade
            }
            .encode()
            .is_err());
            let mut over = encoded;
            over[102..106].copy_from_slice(&u32::try_from(over_code.len()).unwrap().to_be_bytes());
            over.truncate(overhead);
            over.extend_from_slice(&over_code);
            assert_eq!(over.len(), MAX_PAYLOAD_BYTES + 1);
            assert!(Payload::new(
                &registry,
                ActivityType::new(ModuleId::Programs, 2).unwrap(),
                &over
            )
            .is_err());
            assert!(NativeProgramUpgrade::decode(&over).is_err());
        }
    }
}

#[test]
fn repair_f30_block_validator_and_state_provenance_roundtrip() {
    let trust = ValidatorSetTrust::parse(TRUST).unwrap();
    let producer = BlockReference::new(17, 19, [1; 32], [0x22; 32], [3; 32]).unwrap();
    let transported = producer.encode();
    let consumer = BlockReference::decode(&transported).unwrap();
    trust.check_reference(&consumer).unwrap();
    assert_eq!(consumer, producer);
    assert_eq!(consumer.block_hash(), [1; 32]);
    assert_eq!(consumer.validator_set_hash(), trust.validator_set_hash());
    assert_eq!(consumer.state_root(), [3; 32]);
    for length in 0..transported.len() {
        assert!(BlockReference::decode(&transported[..length]).is_err());
    }
    let mut trailing = transported.clone();
    trailing.push(0);
    assert!(BlockReference::decode(&trailing).is_err());
    for (offset, value) in [(4, 2), (12, 126)] {
        let mut invalid = transported.clone();
        invalid[offset] = value;
        assert!(BlockReference::decode(&invalid).is_err());
    }
    let wrong_set = BlockReference::new(17, 19, [1; 32], [4; 32], [3; 32]).unwrap();
    assert!(trust
        .check_reference(&BlockReference::decode(&wrong_set.encode()).unwrap())
        .is_err());
    let wrong_height = BlockReference::new(17, 20, [1; 32], [0x22; 32], [3; 32]).unwrap();
    assert!(trust.check_reference(&wrong_height).is_err());
}

#[test]
fn repair_f31_explicit_trust_refuses_retired_or_invalid_configuration() {
    let trust = ValidatorSetTrust::parse(TRUST).unwrap();
    assert_eq!(trust.chain_id(), 125);
    assert_eq!(trust.comet_chain_id(), "hyperpax_125-1");
    assert_eq!(trust.height(), 19);
    assert!(ValidatorSetTrust::parse(ARCHIVE).is_err());
    for invalid in [
        TRUST.replace("chain-trust/1", "chain-trust/2"),
        TRUST.replace("125,", "126,"),
        TRUST.replace("\"height\":19", "\"height\":0"),
        TRUST.replace("\"height\":19", "\"height\":9223372036854775808"),
        TRUST.replace("hyperpax_125-1", ""),
        TRUST.replace("hyperpax_125-1", "wrong chain"),
        TRUST.replace(&"22".repeat(32), &"00".repeat(32)),
        TRUST.replace("\"height\":19,", ""),
        TRUST.replace("\"height\":19", "\"height\":19,\"height\":20"),
        TRUST.replace(
            "\"height\":19",
            "\"height\":19,\"settlement_contract\":\"0x01\"",
        ),
    ] {
        assert!(ValidatorSetTrust::parse(&invalid).is_err(), "{invalid}");
    }
}

#[test]
#[allow(deprecated)]
fn repair_f32_retired_labels_cannot_satisfy_current_policy() {
    let levels = [
        VerificationLevel::UNVERIFIED,
        VerificationLevel::SEQUENCER_SIGNED,
        VerificationLevel::BATCH_INCLUDED,
        VerificationLevel::STATE_PROVEN,
        VerificationLevel::COMMITTED,
    ];
    for (index, achieved) in levels.iter().copied().enumerate() {
        let observed = VerificationLevel::from_wire_rank(achieved.wire_rank()).unwrap();
        assert_eq!(observed, achieved);
        assert!(observed.is_current());
        for (requested_index, requested) in levels.iter().copied().enumerate() {
            assert_eq!(observed.compare(requested), index.cmp(&requested_index));
        }
    }
    for retired in [
        VerificationLevel::CHECKPOINT_FINALISED,
        VerificationLevel::SETTLEMENT_ANCHORED,
    ] {
        assert!(!retired.is_current());
        assert!(VerificationLevel::from_wire_rank(retired.wire_rank()).is_err());
        assert_eq!(retired.rung(), None);
        for current in levels {
            assert!(retired.compare(current).is_lt());
            assert!(current.compare(retired).is_lt());
            assert!(retired < current);
        }
    }
    assert_eq!(VerificationLevel::COMMITTED.rung(), Some(Rung::Committed));
    assert_ne!(VerificationLevel::COMMITTED.wire_rank(), 4);
}

#[test]
fn repair_f33_archive_readers_preserve_bytes_without_current_privileges() {
    let archive = DeclaredCheckpointSettlement::parse(ARCHIVE).unwrap();
    assert!(!archive.domains().is_empty());
    assert!(archive.domain("undeclared-domain").is_err());
    assert!(DeclaredCheckpointSettlement::parse(
        &ARCHIVE.replace("checkpoint-settlement/1", "checkpoint-settlement/2")
    )
    .is_err());
    for rank in 0..=5 {
        let observation = HistoricalVerificationLevel::from_wire_rank(rank).unwrap();
        assert_eq!(observation.wire_rank(), rank);
        assert_eq!(observation.rung(), None);
    }
    for rank in 6..=u8::MAX {
        assert!(HistoricalVerificationLevel::from_wire_rank(rank).is_err());
    }
    for rank in [4, 5, 7, u8::MAX] {
        assert!(VerificationLevel::from_wire_rank(rank).is_err());
    }
}

#[test]
fn repair_f48_covering_block_preserves_distinct_execution_height() {
    for covering_height in [17, 19] {
        let producer = BlockReference::new(17, covering_height, [1; 32], [2; 32], [3; 32]).unwrap();
        let receipt_provenance = BlockReference::decode(&producer.encode()).unwrap();
        assert_eq!(receipt_provenance.execution_height(), 17);
        assert_eq!(receipt_provenance.covering_height(), covering_height);
    }
    assert!(BlockReference::new(17, 16, [1; 32], [2; 32], [3; 32]).is_err());
    assert!(BlockReference::new(0, 19, [1; 32], [2; 32], [3; 32]).is_err());
    // Decoding provenance remains separate from achieving a finality rung.
    assert_eq!(Rung::from_name("checkpoint-finalised"), None);
}
