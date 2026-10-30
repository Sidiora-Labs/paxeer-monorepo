use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_human_kms::attestor::{decode_unsigned_evm_transaction, unsigned_evm_transaction};
use layerx_human_service::agents::ProtocolEvidence;
use layerx_types::payload::{ActivityType, ModuleId};
use layerx_types::rung::Rung;

#[test]
fn human_kms_and_service_name_no_intents_envelope() -> Result<(), Box<dyn std::error::Error>> {
    for (path, source) in [
        ("kms/attestor.rs", include_str!("../src/attestor.rs")),
        ("kms/server.rs", include_str!("../src/server.rs")),
        (
            "service/agents/create.rs",
            include_str!("../../layerx-human-service/src/agents/create.rs"),
        ),
        (
            "service/custody/gateway.rs",
            include_str!("../../layerx-human-service/src/custody/gateway.rs"),
        ),
    ] {
        let compact: String = source.chars().filter(|ch| !ch.is_whitespace()).collect();
        for retired in [
            "layerx_intents::canonical",
            "layerx_intents::owner_activity",
        ] {
            assert!(!compact.contains(retired), "{path} still names {retired}");
        }
    }

    let mut to = [0; 20];
    to[18..].copy_from_slice(&[0x10, 0x1a]);
    let call = Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to,
        data: [
            layerx_crypto::disclosure::call::ledger_selector("openAccount(bytes32)").to_vec(),
            vec![9; 32],
        ]
        .concat(),
    };
    let unsigned = unsigned_evm_transaction(&call);
    assert_eq!(decode_unsigned_evm_transaction(&unsigned)?, call);
    let signed = call.sign(&[1; 32])?;
    assert!(decode_unsigned_evm_transaction(&signed).is_err());
    assert!(decode_signed(&unsigned).is_err());
    let mut trailing = unsigned.clone();
    trailing.push(0);
    assert!(decode_unsigned_evm_transaction(&trailing).is_err());
    let mut wrong_type = unsigned;
    wrong_type[0] = 1;
    assert!(decode_unsigned_evm_transaction(&wrong_type).is_err());

    let (decoded, signature) = decode_signed(&signed)?;
    assert_eq!(decoded, call);
    let sender = recover_sender(&decoded, &signature)?;
    let mut owner = [0; 32];
    owner[12..].copy_from_slice(&sender);
    let hash = transaction_hash(&signed);
    let evidence = ProtocolEvidence {
        signed_activity: signed,
        actor: sender.to_vec(),
        owner_public_key: owner,
        network_id: 125,
        action_key: [0; 32],
        activity_id: hash,
        receipt_bytes: Vec::new(),
        tx_hash: hash,
        receipt_hash: [0; 32],
        block_height: 0,
        rung: Rung::Submitted,
    };
    let expected = ActivityType::new(ModuleId::Asset, 4).map_err(|e| format!("{e:?}"))?;
    let activity = evidence
        .bound_activity(expected)
        .map_err(|e| format!("{e:?}"))?;
    assert_eq!(activity.account_sequence(), call.nonce);
    assert_eq!(activity.network_id(), 125);
    assert_eq!(activity.actor_did(), sender);
    assert_eq!(activity.authority(), owner);
    assert_eq!(activity.payload(), call.data);
    assert_eq!(activity.fee_limit(), 200_000);
    assert!(evidence.verify_outcome(expected).is_err());
    let mut invalid = evidence.clone();
    invalid.rung = Rung::Committed;
    assert!(invalid.verify_outcome(expected).is_err());
    for field in 0..8 {
        let mut invalid = evidence.clone();
        match field {
            0 => invalid.actor[0] ^= 1,
            1 => invalid.owner_public_key[12] ^= 1,
            2 => invalid.network_id += 1,
            3 => invalid.action_key[0] ^= 1,
            4 => invalid.activity_id[0] ^= 1,
            5 => invalid.tx_hash[0] ^= 1,
            6 => invalid.signed_activity.push(0),
            7 => invalid.signed_activity[0] = 1,
            _ => unreachable!(),
        }
        assert!(
            invalid.bound_activity(expected).is_err(),
            "accepted mutation {field}"
        );
    }
    let wrong_operation = ActivityType::new(ModuleId::Asset, 10).map_err(|e| format!("{e:?}"))?;
    assert!(evidence.bound_activity(wrong_operation).is_err());
    let mut unsupported = call;
    unsupported.to = [0; 20];
    let mut invalid = evidence;
    invalid.signed_activity = unsupported.sign(&[1; 32])?;
    invalid.tx_hash = transaction_hash(&invalid.signed_activity);
    invalid.activity_id = invalid.tx_hash;
    assert!(invalid.bound_activity(expected).is_err());
    Ok(())
}
