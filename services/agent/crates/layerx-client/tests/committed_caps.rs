use layerx_client::budget::{
    budget_state_key, ProtocolBudgetRecord, ReconcileError, BUDGET_MODULE_ID,
};
use serde_json::Value;
use std::os::unix::fs::DirBuilderExt;
use std::path::Path;
use std::process::Command;
use std::sync::OnceLock;

fn vectors() -> Value {
    static VECTORS: OnceLock<Value> = OnceLock::new();
    VECTORS
        .get_or_init(|| {
            let root = match Path::new(env!("CARGO_MANIFEST_DIR"))
                .join("../../../..")
                .canonicalize()
            {
                Ok(required_value) => required_value,
                Err(error) => panic!("{}: {error:?}", "repository root"),
            };
            let directory =
                std::env::temp_dir().join(format!("layerx-caps-vectors-{}", std::process::id()));
            match std::fs::DirBuilder::new().mode(0o700).create(&directory) {
                Ok(required_value) => required_value,
                Err(error) => panic!("{}: {error:?}", "private native producer directory"),
            }
            let executable = directory.join("lxp_wallet_caps_vectors");
            let compiled = match Command::new("cc")
                .current_dir(&root)
                .args([
                    "-std=c17",
                    "-pedantic",
                    "-Werror",
                    "-Wall",
                    "-Wextra",
                    "-Wconversion",
                    "-Wshadow",
                    "-Wvla",
                    "-fno-strict-aliasing",
                    "-ffp-contract=off",
                    "-O2",
                    "-ffunction-sections",
                    "-fdata-sections",
                    "-Iinclude",
                    "tests/qualification/lxp_wallet_caps_vectors.c",
                    "src/modules/budget/lx_budget_codec.c",
                    "src/modules/budget/lx_budget_create.c",
                    "src/protocol/lxp_u128.c",
                    "src/crypto/lxp_ct.c",
                    "-Wl,--gc-sections",
                    "-o",
                ])
                .arg(&executable)
                .output()
            {
                Ok(required_value) => required_value,
                Err(error) => panic!("{}: {error:?}", "compile native vector producer"),
            };
            assert!(
                compiled.status.success(),
                "native vector producer compile failed: {}",
                String::from_utf8_lossy(&compiled.stderr)
            );
            let produced = match Command::new(&executable).output() {
                Ok(required_value) => required_value,
                Err(error) => panic!("{}: {error:?}", "run native vector producer"),
            };
            assert!(
                produced.status.success(),
                "native vector producer failed: {}",
                String::from_utf8_lossy(&produced.stderr)
            );
            let vectors = match serde_json::from_slice(&produced.stdout) {
                Ok(required_value) => required_value,
                Err(error) => panic!("{}: {error:?}", "native JSON"),
            };
            match std::fs::remove_file(executable) {
                Ok(required_value) => required_value,
                Err(error) => panic!("{}: {error:?}", "remove native vector producer"),
            }
            match std::fs::remove_dir(directory) {
                Ok(required_value) => required_value,
                Err(error) => panic!("{}: {error:?}", "remove native producer directory"),
            }
            vectors
        })
        .clone()
}

fn bytes(v: &Value, name: &str) -> Vec<u8> {
    let Some(s) = v[name].as_str() else {
        panic!("native field")
    };
    assert_eq!(s.len() % 2, 0);
    (0..s.len())
        .step_by(2)
        .map(|i| match u8::from_str_radix(&s[i..i + 2], 16) {
            Ok(required_value) => required_value,
            Err(error) => panic!("{}: {error:?}", "hex"),
        })
        .collect()
}

#[test]
fn native_budget_versions_fields_and_extremes() {
    let v = vectors();
    assert_eq!(BUDGET_MODULE_ID, 3);
    for name in ["budget_v1", "budget_v2"] {
        let r = match ProtocolBudgetRecord::decode(&bytes(&v, name)) {
            Ok(required_value) => required_value,
            Err(error) => panic!("{}: {error:?}", "C budget codec"),
        };
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
    let r = match ProtocolBudgetRecord::decode(&bytes(&v, "budget_extreme")) {
        Ok(required_value) => required_value,
        Err(error) => panic!("{}: {error:?}", "native max fields"),
    };
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
