#[path = "../src/token_call.rs"]
mod token_call;

use layerx_program_sdk::{
    lxt20::{Request, REFERENCE_ASSET},
    payments::{PaymentGrant, PreparedProgramAccount},
    Amount, AssetId, CallInput, Capability, GrantedCapabilities, Principal, ProgramId,
};
use token_call::{NestedTokenCall, TokenCall};

const GUIDE: &str = include_str!("../README.md");

fn guide_text() -> String {
    GUIDE.split_whitespace().collect::<Vec<_>>().join(" ")
}

#[test]
fn finding_036_documented_wire_and_authority_match_production() {
    let pool = ProgramId::new([0x55; 32]).unwrap();
    let other_pool = ProgramId::new([0x56; 32]).unwrap();
    let user = Principal::new([0x71; 32]).unwrap();
    let amount = Amount::from_u128(4_000);

    for (token_byte, backing_byte) in [(0x5a, 0x6a), (0x5b, 0x6b)] {
        let token = ProgramId::new([token_byte; 32]).unwrap();
        let backing = AssetId::new([backing_byte; 32]).unwrap();
        let pool_seed = pool.bytes();
        let user_seed = user.bytes();
        let pool_account = PreparedProgramAccount::new(token, &pool_seed, backing).unwrap();
        let user_account = PreparedProgramAccount::new(token, &user_seed, backing).unwrap();

        for pull in [true, false] {
            let call = if pull {
                TokenCall::pull(token, backing, pool, user, amount)
            } else {
                TokenCall::push(token, backing, pool, user, amount)
            }
            .unwrap();
            assert_eq!(call.program(), token);
            let transported = CallInput::new(call.input()).unwrap();
            let request = Request::decode(transported.bytes()).unwrap();
            let (source, recipient, ordinal, length, operation, debit, spender, recipient_seed) =
                if pull {
                    assert_eq!(
                        request,
                        Request::TransferFrom {
                            owner: user,
                            to: pool_account.account(),
                            amount,
                        }
                    );
                    (
                        user_account,
                        pool_account.account(),
                        3,
                        90,
                        "Pull",
                        "user id32",
                        "immediate pool id32",
                        "pool id32",
                    )
                } else {
                    assert_eq!(
                        request,
                        Request::Transfer {
                            to: user_account.account(),
                            amount,
                        }
                    );
                    (
                        pool_account,
                        user_account.account(),
                        1,
                        58,
                        "Push",
                        "immediate pool id32",
                        "none",
                        "user id32",
                    )
                };
            assert_eq!(transported.bytes().len(), length);
            assert_eq!(&transported.bytes()[..4], &[b'L', b'X', 20, ordinal]);
            assert_eq!(request.encode().unwrap().as_slice(), transported.bytes());
            let row = format!(
                "| {operation} | `{}` | {ordinal} | {length} | {debit} | {spender} | {recipient_seed} |",
                request.method(),
            );
            assert!(
                GUIDE.lines().any(|line| line == row),
                "missing contract row: {row}"
            );

            let received = NestedTokenCall::decode(transported.bytes(), Some(pool)).unwrap();
            assert_eq!(received.to, recipient);
            assert_eq!(received.amount, amount);
            if pull {
                assert_eq!(received.owner, user);
                assert_eq!(received.spender.unwrap().bytes(), pool.bytes());
                assert_ne!(received.spender, Some(user));
            } else {
                assert_eq!(received.owner.bytes(), pool.bytes());
                assert_eq!(received.spender, None);
            }
            assert!(NestedTokenCall::decode(transported.bytes(), None).is_err());
            let different_caller =
                NestedTokenCall::decode(transported.bytes(), Some(other_pool)).unwrap();
            if pull {
                assert_eq!(different_caller.owner, user);
                assert_eq!(
                    different_caller.spender.unwrap().bytes(),
                    other_pool.bytes()
                );
            } else {
                assert_eq!(different_caller.owner.bytes(), other_pool.bytes());
                assert_eq!(different_caller.spender, None);
            }

            let grants = call.capabilities().unwrap();
            assert_eq!(grants.len(), 3);
            for grant in [
                PaymentGrant::Basic(Capability::SharedStorageRead),
                PaymentGrant::Basic(Capability::SharedStorageWrite),
                PaymentGrant::ProgramSpend {
                    account: source,
                    to: recipient,
                    maximum: amount,
                },
            ] {
                assert!(grants.grants().contains(&grant));
            }
            let mut encoded = [0; 256];
            let written = grants.encode_into(&mut encoded).unwrap();
            let transport = GrantedCapabilities::new(&encoded[..written]).unwrap();
            let mut expected = source
                .spend_grant(recipient, amount)
                .unwrap()
                .as_slice()
                .to_vec();
            expected[..2].copy_from_slice(&3_u16.to_be_bytes());
            expected.extend_from_slice(&[7, 8]);
            assert_eq!(transport.bytes(), expected);
        }
        assert_ne!(backing.bytes(), REFERENCE_ASSET);
    }
}

#[test]
fn finding_036_guide_preserves_token_blockers_and_proof_boundary() {
    // These are source-drift checks, not a native execution or rollback proof.
    let token_source = include_str!("../../token-lxt20/src/lib.rs");
    assert!(token_source.contains(
        "fn refuse(_: &[u8]) -> Result<CallResult, ProgramError> {\n    Err(denied())\n}"
    ));
    assert!(token_source.contains("layerx_program_sdk::entrypoint!(refuse);"));
    assert!(token_source
        .contains("if Context::immediate_caller()?.is_some() {\n        return Err(denied());"));
    for method in ["transfer", "transfer_from"] {
        assert!(token_source.contains(&format!(
            "fn {method}_impl(input: &[u8]) -> Result<CallResult, ProgramError> {{\n    let caller = direct()?;"
        )));
    }
    let registry = include_str!("../../../../../crates/layerx-programs-registry/src/lxt20.rs");
    assert!(registry.contains("(\"transfer\", 1, spend(10, 42))"));
    assert!(registry.contains("(\"transfer_from\", 3, spend(42, 74))"));
    assert!(!registry.contains("\"layerx_call\""));
    assert_eq!(REFERENCE_ASSET, [0x44; 32]);

    let guide = guide_text();
    for claim in [
        "The supplied token is **incompatible with swap settlement**.",
        "The supplied token's `layerx_call` unconditionally refuses",
        "Its registry interface binds named direct-call entries, not the required nested route.",
        "immediate caller supplied by the runtime, never a caller identity in calldata.",
        "These tests do not execute a native swap or prove rollback or restart recovery.",
        "No such native rollback/restart proof is supplied here.",
        "refuse an output leg after a successful input leg.",
        "Close and reopen the persistent store",
    ] {
        assert!(guide.contains(claim), "missing qualification: {claim}");
    }
    let record = include_str!("../src/repair_22_33_250.kvx");
    assert!(record.contains("finding = \"22.30.129/036\""));
    for test in [
        "finding_036_documented_wire_and_authority_match_production",
        "finding_036_guide_preserves_token_blockers_and_proof_boundary",
    ] {
        assert!(record.contains(test));
    }
    for path in [
        "programs/sdk/rust/examples/token-lxt20/src/lib.rs",
        "programs/crates/layerx-programs-registry/src/lxt20.rs",
        "programs/crates/layerx-programs-runtime/tests",
    ] {
        assert!(record.lines().any(|line| line.starts_with("wiring_")
            && line.contains("Domain 13 (fm-programs)")
            && line.contains(path)));
    }
}
