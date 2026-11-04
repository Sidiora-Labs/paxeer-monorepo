//! Task 22.33.1540; audit 22.30.95; acceptance 22998.1.
//! Finding 35: creation_onboarding_consent, creation_recovery_policy,
//! creation_recovery_policy_with_delay_bounds, creation_budget_v1,
//! creation_open_account, and creation_inventory_is_exact.
//! Finding 36: recovery_zero_did_is_refused, recovery_zero_root_is_refused,
//! recovery_zero_threshold_is_refused and their extended-policy counterparts.
//! Baseline was not run: the explicit implementation-first and hf-build/hf-done
//! order prohibits the contradictory unchanged-tree verification step. No
//! baseline failures are claimed. This is the task record because spec edits
//! are forbidden; the gate ledger records the candidate revision and evidence.
//! Continued baseline (attempt 2): 2 passed, 11 failed; onboarding consent
//! failed binding with MalformedPayload, and the recovery signing path failed
//! with FieldMismatch("source"). The current gate ledger retains exact evidence.
//! KMS call binding now retains the EIP-1559 source, binds the recorded consent
//! shape, and refuses zero recovery fields before nonce reservation/signing.
//! Wiring request (Agent/Crypto domain 9): services/agent/crates/layerx-crypto/src/disclosure/call.rs must expose native onboarding consent binding for consumers outside KMS.
//! Wiring request (Human domain 10): services/human/crates/layerx-human-service/src/custody/provider.rs must use the call-specific re-encoder and the KMS native consent binding when constructing signing frames.
//! First current qualification: c97e9c3d0e63cce2ab438715139ef3b907f0316a,
//! cg spec done 22.33.1540 -f layerx-merge --root /work/paxeer-monorepo,
//! exit 1, /out/logs/verify-1.log: twelve cases refused the seeded host's
//! literal owner assertions before reaching binding. The fixture now produces
//! signed, current owner JWTs and each peer checks the exact issued assertion.
//! The production server, durable store, wire codec and remote client run in
//! process. Committed loopback signing peers use independent fixture keys and
//! the real EIP-1559 signing producer; no external toolchain is invoked.

#![allow(dead_code)]

pub use layerx_human_kms::attestor;
#[path = "../src/config.rs"]
mod config;
#[path = "../src/evm.rs"]
mod evm;
#[path = "../src/evm_types.rs"]
mod evm_types;
#[path = "../src/export.rs"]
mod export;
#[path = "../src/send.rs"]
mod send;
#[path = "../src/store.rs"]
mod store;
#[path = "../src/wire.rs"]
mod wire;
mod server {
    include!(concat!(env!("CARGO_MANIFEST_DIR"), "/src/server.rs"));

    pub fn serve_connection(
        tcp: std::net::TcpStream,
        config: &crate::config::Config,
        store: &std::sync::Mutex<crate::store::Store>,
        clock: &dyn layerx_types::clock::Clock,
    ) -> crate::wire::Result<()> {
        connection(tcp, config, store, clock)
    }
}
#[path = "../../../../platform/hosted/runtime-clock/src/server.rs"]
mod clock_server;
#[path = "provider/repair_22_33_1540_host.rs"]
mod fixture;
#[path = "provider/native_setup_vectors.rs"]
mod vectors;

use fixture::{checked, Host, Result, SERIAL};
use layerx_crypto::disclosure::call::ConstructMethod;
use layerx_crypto::evm_transaction::{recover_sender, Eip1559Call};
use layerx_crypto::signer::AgentSignature;
use layerx_human_kms::call::bind_call;
use layerx_human_service::custody::KmsProvider;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};

fn registry() -> Result<ModuleRegistry> {
    let registrations = [
        (ModuleId::Asset, vec![4]),
        (ModuleId::Budget, vec![1]),
        (ModuleId::Governance, vec![1, 3]),
    ]
    .into_iter()
    .map(|(module, ordinals)| {
        let types = ordinals
            .into_iter()
            .map(|ordinal| checked(ActivityType::new(module, ordinal)))
            .collect::<Result<Vec<_>>>()?;
        checked(ModuleRegistration::new(module, &types))
    })
    .collect::<Result<Vec<_>>>()?;
    checked(ModuleRegistry::new(&registrations))
}

// Used by the committed signing peers to derive their independent wallet key.
fn call(nonce: u64) -> Eip1559Call {
    let mut to = [0; 20];
    to[18..].copy_from_slice(&[0x10, 0x1a]);
    let mut data =
        layerx_crypto::disclosure::call::ledger_selector("openAccount(bytes32)").to_vec();
    data.extend([3; 32]);
    Eip1559Call {
        chain_id: 125,
        nonce,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to,
        data,
    }
}

#[test]
fn creation_inventory_is_exact() -> Result<()> {
    let cases = vectors::creation_cases([3; 32])?;
    assert_eq!(
        cases.iter().map(|(kind, _)| *kind).collect::<Vec<_>>(),
        [
            "onboarding-consent",
            "recovery-policy",
            "recovery-policy-with-delay-bounds",
            "budget-create-v1",
            "open-account",
        ]
    );
    assert_eq!(cases.len(), vectors::CREATION_KINDS.len());
    Ok(())
}

fn creation_before_and_after_restart(kind: &str) -> Result<()> {
    let _serial = SERIAL.lock().map_err(|_| "fixture lock poisoned")?;
    let mut host = Host::new()?;
    let remote = host.remote(false)?;
    let binding = host.bindings[0].clone();
    let description = remote.create_key(&binding)?;
    let reference = description.reference();
    let wallet = remote.wallet_descriptor(&binding, reference)?;
    let cases = vectors::creation_cases(description.public_key())?;
    assert_eq!(
        cases.iter().map(|(kind, _)| *kind).collect::<Vec<_>>(),
        vectors::CREATION_KINDS
    );
    let selected = cases
        .iter()
        .filter(|(candidate, _)| *candidate == kind)
        .collect::<Vec<_>>();
    assert_eq!(selected.len(), 1, "exact coverage of {kind}");
    let payload = &selected[0].1;
    let tx = vectors::setup_call(payload.clone(), vectors::creation_limits(payload))?;
    let mut first = None;
    let mut passes = Vec::new();
    for pass in 0..2 {
        if pass == 1 {
            host.stop()?;
            host.start()?;
        }
        let remote = host.remote(false)?;
        assert_eq!(remote.describe_key(&binding, reference)?, description);
        assert_eq!(remote.wallet_descriptor(&binding, reference)?, wallet);
        let disclosure = bind_call(&tx, wallet.evm_secp256k1).map_err(|error| {
            format!("native creation {kind}, pass {pass}: bind_call: {error:?}")
        })?;
        match checked(disclosure.construct_method())? {
            Some(ConstructMethod::AuthLifecycle {
                payload: actual,
                idempotency_key,
            }) => {
                assert_eq!(
                    actual,
                    payload.as_bytes(),
                    "{kind}: retained lifecycle body"
                );
                assert_eq!(idempotency_key, [4; 32], "{kind}: retained action key");
            }
            Some(ConstructMethod::PayBudgetCreate { payload: actual }) => {
                assert_eq!(actual, payload.as_bytes(), "{kind}: retained budget body");
            }
            Some(ConstructMethod::LedgerOpenAccount { asset }) => {
                assert_eq!(asset, payload.as_bytes()[2..34], "{kind}: retained asset");
            }
            other => {
                return Err(format!("native creation {kind}: unexpected binding {other:?}").into())
            }
        }
        let canonical = attestor::unsigned_evm_transaction(&tx);
        assert_eq!(attestor::decode_unsigned_evm_transaction(&canonical)?, tx);
        assert_eq!(checked(disclosure.reencode())?, canonical);
        let frame = host.sign_frame(
            &binding,
            reference,
            wallet.evm_secp256k1,
            &canonical,
            &checked(disclosure.disclosure_bytes())?,
        )?;
        let response = host.exchange(&frame, false)?;
        assert_eq!(
            &response[..8],
            b"LXKP\0\x07\x05\0",
            "native creation {kind}, pass {pass}: signing refused"
        );
        assert_eq!(
            response.len(),
            73,
            "native creation {kind}, pass {pass}: signature length"
        );
        let signature: [u8; 65] = response[8..].try_into()?;
        let encoded = checked(disclosure.disclosure_bytes())?;
        for index in [0, 1, 9, 33, encoded.len() - 1] {
            let mut changed = encoded.clone();
            changed[index] ^= 1;
            let frame = host.sign_frame(
                &binding,
                reference,
                wallet.evm_secp256k1,
                &canonical,
                &changed,
            )?;
            assert_eq!(
                host.exchange(&frame, false)?,
                wire::response(7, 5, Err(wire::Error::Refused)),
                "{kind}, pass {pass}: disclosure corruption {index}"
            );
        }
        assert_eq!(
            recover_sender(&tx, &checked(AgentSignature::new(signature))?)?,
            wallet.evm_secp256k1,
            "native creation {kind}, pass {pass}"
        );
        if let Some(expected) = first {
            assert_eq!(
                signature, expected,
                "{kind} signature after reopening store"
            );
        } else {
            first = Some(signature);
        }
        passes.push(pass);
    }
    assert_eq!(passes, [0, 1], "both restart passes for {kind}");
    Ok(())
}

#[test]
fn creation_onboarding_consent() -> Result<()> {
    creation_before_and_after_restart("onboarding-consent")
}
#[test]
fn creation_recovery_policy() -> Result<()> {
    creation_before_and_after_restart("recovery-policy")
}
#[test]
fn creation_recovery_policy_with_delay_bounds() -> Result<()> {
    creation_before_and_after_restart("recovery-policy-with-delay-bounds")
}
#[test]
fn creation_budget_v1() -> Result<()> {
    creation_before_and_after_restart("budget-create-v1")
}
#[test]
fn creation_open_account() -> Result<()> {
    creation_before_and_after_restart("open-account")
}

fn zeroed_recovery_is_refused(
    field: &str,
    start: usize,
    length: usize,
    extended: bool,
) -> Result<()> {
    let responses = {
        let _serial = SERIAL.lock().map_err(|_| "fixture lock poisoned")?;
        let mut host = Host::new()?;
        let remote = host.remote(false)?;
        let binding = host.bindings[0].clone();
        let description = remote.create_key(&binding)?;
        let reference = description.reference();
        let wallet = remote.wallet_descriptor(&binding, reference)?;
        let valid = vectors::recovery(extended)?;
        let valid_tx =
            vectors::setup_call(vectors::payload(ModuleId::Governance, 3, &valid)?, (7, 1))?;
        let mut responses = Vec::new();
        for pass in 0..2 {
            if pass == 1 {
                host.stop()?;
                host.start()?;
            }
            let remote = host.remote(false)?;
            assert_eq!(remote.describe_key(&binding, reference)?, description);
            let valid_disclosure = checked(bind_call(&valid_tx, wallet.evm_secp256k1))?;
            let valid_frame = host.sign_frame(
                &binding,
                reference,
                wallet.evm_secp256k1,
                &checked(valid_disclosure.reencode())?,
                &checked(valid_disclosure.disclosure_bytes())?,
            )?;
            let response = host.exchange(&valid_frame, false)?;
            assert_eq!(
                &response[..8],
                b"LXKP\0\x07\x05\0",
                "valid recovery control"
            );
            assert_eq!(response.len(), 73);
            let signature: [u8; 65] = response[8..].try_into()?;
            assert_eq!(
                recover_sender(&valid_tx, &checked(AgentSignature::new(signature))?)?,
                wallet.evm_secp256k1
            );
            let mut bytes = valid.clone();
            assert!(bytes[start..start + length].iter().any(|byte| *byte != 0));
            bytes[start..start + length].fill(0);
            let tx =
                vectors::setup_call(vectors::payload(ModuleId::Governance, 3, &bytes)?, (8, 1))?;
            let disclosure = checked(bind_call(&tx, wallet.evm_secp256k1))?;
            let canonical = attestor::unsigned_evm_transaction(&tx);
            assert_eq!(attestor::decode_unsigned_evm_transaction(&canonical)?, tx);
            let frame = host.sign_frame(
                &binding,
                reference,
                wallet.evm_secp256k1,
                &canonical,
                &checked(disclosure.disclosure_bytes())?,
            )?;
            responses.push(host.exchange(&frame, false)?);
            let mut next_valid = valid_tx.clone();
            next_valid.nonce = tx.nonce;
            let next_disclosure = checked(bind_call(&next_valid, wallet.evm_secp256k1))?;
            let next_frame = host.sign_frame(
                &binding,
                reference,
                wallet.evm_secp256k1,
                &checked(next_disclosure.reencode())?,
                &checked(next_disclosure.disclosure_bytes())?,
            )?;
            let accepted = host.exchange(&next_frame, false)?;
            assert_eq!(
                &accepted[..8],
                b"LXKP\0\x07\x05\0",
                "refused recovery must not reserve its nonce"
            );
            assert_eq!(accepted.len(), 73);
            let signature: [u8; 65] = accepted[8..].try_into()?;
            assert_eq!(
                recover_sender(&next_valid, &checked(AgentSignature::new(signature))?)?,
                wallet.evm_secp256k1
            );
        }
        responses
    };
    // Assert after releasing the fixture lock so every malformed field executes
    // even when a preceding case detects a production validation defect.
    assert_eq!(responses.len(), 2);
    for (pass, response) in responses.into_iter().enumerate() {
        assert_eq!(
            response,
            wire::response(7, 5, Err(wire::Error::Refused)),
            "zeroed recovery {field}, extended={extended}, pass={pass} must be typed Refused with no signature"
        );
    }
    Ok(())
}

#[test]
fn recovery_zero_did_is_refused() -> Result<()> {
    zeroed_recovery_is_refused("did_id", 4, 32, false)
}
#[test]
fn recovery_zero_root_is_refused() -> Result<()> {
    zeroed_recovery_is_refused("recovery_root", 36, 32, false)
}
#[test]
fn recovery_zero_threshold_is_refused() -> Result<()> {
    zeroed_recovery_is_refused("threshold", 68, 2, false)
}
#[test]
fn extended_recovery_zero_did_is_refused() -> Result<()> {
    zeroed_recovery_is_refused("did_id", 4, 32, true)
}
#[test]
fn extended_recovery_zero_root_is_refused() -> Result<()> {
    zeroed_recovery_is_refused("recovery_root", 36, 32, true)
}
#[test]
fn extended_recovery_zero_threshold_is_refused() -> Result<()> {
    zeroed_recovery_is_refused("threshold", 68, 2, true)
}

#[test]
fn onboarding_rejects_malformed_body_limits_and_abi_through_real_server() -> Result<()> {
    let _serial = SERIAL.lock().map_err(|_| "fixture lock poisoned")?;
    let host = Host::new()?;
    let remote = host.remote(false)?;
    let binding = &host.bindings[0];
    let description = remote.create_key(binding)?;
    let reference = description.reference();
    let wallet = remote.wallet_descriptor(binding, reference)?;
    let cases = vectors::creation_cases(description.public_key())?;
    let consent = &cases[0].1;
    assert_eq!(cases[0].0, "onboarding-consent");
    let valid = vectors::setup_call(consent.clone(), (0, 0))?;
    let disclosed = checked(bind_call(&valid, wallet.evm_secp256k1))?;
    let encoded = checked(disclosed.disclosure_bytes())?;
    let mut corruptions = Vec::new();
    for (field, start, length) in [
        ("sponsor", 4, 32),
        ("native_asset", 36, 32),
        ("target_account", 68, 32),
        ("action_key", 100, 32),
        ("expires_at", 132, 8),
    ] {
        let mut tx = valid.clone();
        tx.data[100 + start..100 + start + length].fill(0);
        corruptions.push((field, tx));
    }
    for field in [
        "nonce",
        "fee",
        "priority_fee",
        "offset",
        "padding",
        "length",
        "action_mismatch",
        "selector",
        "target",
        "trailing",
    ] {
        let mut tx = valid.clone();
        match field {
            "nonce" => tx.nonce = 1,
            "fee" => tx.max_fee_per_gas = 1,
            "priority_fee" => tx.max_priority_fee_per_gas = 1,
            "offset" => tx.data[35] = 96,
            "padding" => *tx.data.last_mut().ok_or("missing consent padding")? = 1,
            "length" => tx.data[99] -= 1,
            "action_mismatch" => tx.data[36] ^= 1,
            "selector" => tx.data[0] ^= 1,
            "target" => tx.to[19] = 0x1e,
            "trailing" => tx.data.push(0),
            _ => unreachable!(),
        }
        corruptions.push((field, tx));
    }
    for (field, tx) in corruptions {
        assert!(
            bind_call(&tx, wallet.evm_secp256k1).is_err(),
            "consent {field} binds"
        );
        let frame = host.sign_frame(
            binding,
            reference,
            wallet.evm_secp256k1,
            &attestor::unsigned_evm_transaction(&tx),
            &encoded,
        )?;
        assert_eq!(
            host.exchange(&frame, false)?,
            wire::response(7, 5, Err(wire::Error::Refused)),
            "consent {field} must be typed Refused without a signature"
        );
    }
    Ok(())
}
