use layerx_client::budget::{
    budget_state_key, ProtocolBudgetRecord, ReconcileError, BUDGET_MODULE_ID,
};
use serde_json::Value;

fn vectors() -> Value {
    let path =
        std::env::var("LAYERX_CAPS_VECTORS").expect("native vector producer output required");
    serde_json::from_slice(&std::fs::read(path).expect("native vectors readable"))
        .expect("native JSON")
}

fn bytes(v: &Value, name: &str) -> Vec<u8> {
    let s = v[name].as_str().expect("native field");
    assert_eq!(s.len() % 2, 0);
    (0..s.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&s[i..i + 2], 16).expect("hex"))
        .collect()
}

#[test]
fn native_budget_versions_fields_and_extremes() {
    let v = vectors();
    assert_eq!(BUDGET_MODULE_ID, 3);
    for name in ["budget_v1", "budget_v2"] {
        let r = ProtocolBudgetRecord::decode(&bytes(&v, name)).expect("C budget codec");
        assert_eq!(budget_state_key(r.budget_id), bytes(&v, "budget_key"));
        assert_eq!(
            (r.owner, r.budget_account, r.asset_id, r.purpose_hash),
            ([2; 32], [3; 32], [4; 32], [5; 32])
        );
        assert_eq!(
            (
                r.per_period_limit,
                r.configured_period_limit,
                r.spent_this_period,
                r.carry_cap,
                r.carried
            ),
            (100, 100, 23, 0, 0)
        );
        assert_eq!(
            (
                r.period_length,
                r.period_start,
                r.expiry,
                r.revocation_sequence
            ),
            (1000, 1000, 9000, 31)
        );
        assert_eq!((r.remaining(), r.window_end_sequence()), (77, 2000));
        assert_eq!(r.delegates, vec![[6; 32], [7; 32]]);
        assert_eq!(
            r.source_account,
            if name == "budget_v2" {
                Some([8; 32])
            } else {
                None
            }
        );
        assert!(!r.closed && !r.revoked);
    }
    let r = ProtocolBudgetRecord::decode(&bytes(&v, "budget_extreme")).expect("native max fields");
    assert_eq!(r.delegates.len(), 16);
    assert_eq!(
        (
            r.per_period_limit,
            r.configured_period_limit,
            r.carry_cap,
            r.spent_this_period,
            r.carried
        ),
        (u128::MAX, u128::MAX, u128::MAX, u128::MAX, u128::MAX)
    );
    assert_eq!(
        (r.period_length, r.expiry, r.revocation_sequence),
        (u64::MAX, u64::MAX, u64::MAX)
    );
    assert_eq!((r.remaining(), r.window_end_sequence()), (0, u64::MAX));
    assert!(r.closed && r.revoked);
}

#[test]
fn budget_refuses_malformed_and_arithmetic_bounds() {
    let v = vectors();
    for name in ["budget_v1", "budget_v2", "budget_extreme"] {
        let b = bytes(&v, name);
        let key = bytes(&v, "budget_key");
        assert!(ProtocolBudgetRecord::decode_state(&key, &b).is_ok());
        let mut wrong = key.clone();
        wrong[7] ^= 1;
        assert!(ProtocolBudgetRecord::decode_state(&wrong, &b).is_err());
        assert!(ProtocolBudgetRecord::decode_state(&key[..38], &b).is_err());
        for length in 0..b.len() {
            assert!(
                ProtocolBudgetRecord::decode(&b[..length]).is_err(),
                "{name} truncated at {length}"
            );
        }
        let mut x = b.clone();
        x.push(0);
        assert!(ProtocolBudgetRecord::decode(&x).is_err());
        for (offset, value) in [(0, 1), (1, 0), (1, 3), (275, 2), (276, 2), (277, 17)] {
            let mut x = b.clone();
            x[offset] = value;
            assert!(ProtocolBudgetRecord::decode(&x).is_err(), "offset {offset}");
        }
        let mut x = b.clone();
        x[2..34].fill(0);
        assert!(ProtocolBudgetRecord::decode(&x).is_err());
        let mut x = b.clone();
        x[242..250].fill(0);
        assert!(ProtocolBudgetRecord::decode(&x).is_err());
        let mut x = b.clone();
        x[250..258].fill(255);
        assert!(ProtocolBudgetRecord::decode(&x).is_err());
        let mut x = b.clone();
        x[162..178].fill(0);
        assert!(ProtocolBudgetRecord::decode(&x).is_err());
        let mut x = b.clone();
        x[310..342].copy_from_slice(&b[278..310]);
        assert!(ProtocolBudgetRecord::decode(&x).is_err());
        let mut x = b.clone();
        x[278..310].copy_from_slice(&b[310..342]);
        x[310..342].copy_from_slice(&b[278..310]);
        assert!(ProtocolBudgetRecord::decode(&x).is_err());
    }
    assert_eq!(
        ProtocolBudgetRecord::decode(b"bad"),
        Err(ReconcileError::ProtocolStateSchemaUnavailable)
    );
}
