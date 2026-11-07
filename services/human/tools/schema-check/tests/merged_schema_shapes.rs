use std::fs;
use std::path::{Path, PathBuf};

use layerx_human_schema_check::{
    baseline_manifest, human_api_schema, parse_json, Json, BASELINE_FILE,
};

#[allow(non_upper_case_globals)]
const WalletSignRequest: &str = "type.WalletSignRequest";
#[allow(non_upper_case_globals)]
const AccountBalance: &str = "type.AccountBalance";
#[allow(non_upper_case_globals)]
const rung: &str = "rung:Rung";
#[allow(non_upper_case_globals)]
const nonce: &str = "nonce:EvmNonce";

fn real_schema_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../../schema/human-api")
}

fn read_schema(file: &str) -> String {
    fs::read_to_string(real_schema_root().join(file))
        .unwrap_or_else(|error| panic!("read {file}: {error}"))
}

fn declaration(file: &str, section: &str, key: &str) -> Json {
    let source = read_schema(file);
    let header = format!("[{section}]");
    let mut lines = source.lines().skip_while(|line| *line != header);
    assert_eq!(
        lines.next(),
        Some(header.as_str()),
        "missing {file}:{header}"
    );
    let value = lines
        .take_while(|line| !line.starts_with('['))
        .filter_map(|line| line.split_once('='))
        .find_map(|(name, value)| (name.trim() == key).then(|| value.trim()))
        .unwrap_or_else(|| panic!("missing {file}:{section}.{key}"));
    parse_json(value).unwrap_or_else(|error| panic!("{file}:{section}.{key}: {error}"))
}

fn string(file: &str, section: &str, key: &str) -> String {
    let Json::String(value) = declaration(file, section, key) else {
        panic!("{file}:{section}.{key} must be a string");
    };
    value
}

fn string_list(file: &str, section: &str, key: &str) -> Vec<String> {
    let Json::Array(values) = declaration(file, section, key) else {
        panic!("{file}:{section}.{key} must be an array");
    };
    values
        .into_iter()
        .map(|value| match value {
            Json::String(value) => value,
            other => panic!("{file}:{section}.{key} must contain strings: {other:?}"),
        })
        .collect()
}

fn optional_field(file: &str, section: &str, field: &str) {
    let name = field
        .split_once(':')
        .unwrap_or_else(|| panic!("invalid field {field}"))
        .0;
    let optional = string_list(file, section, "optional");
    assert_eq!(
        optional.iter().filter(|value| *value == field).count(),
        1,
        "{file}:{section} must declare {field} exactly once"
    );
    assert!(
        !string_list(file, section, "required")
            .iter()
            .any(|value| value
                .split_once(':')
                .is_some_and(|(value, _)| value == name)),
        "{file}:{section}.{name} must remain optional"
    );
}

fn response(file: &str, operation: &str, expected: &str) {
    assert_eq!(
        string(file, &format!("operation.{operation}"), "response"),
        expected
    );
}

fn additive_journey(file: &str, section: &str, additions: &[&str]) {
    assert_eq!(
        string_list(file, section, "required"),
        string_list("journeys.kvx", "type.Journey", "required")
    );
    let mut expected = string_list("journeys.kvx", "type.Journey", "optional");
    expected.extend(additions.iter().map(|value| (*value).to_owned()));
    assert_eq!(string_list(file, section, "optional"), expected);
}

#[test]
fn wallet_sign_request_carries_eip1559_fields() {
    assert_eq!(
        string_list("movement.kvx", WalletSignRequest, "required"),
        [
            "stage_id:StageId",
            "copy_key:CopyKey",
            "from_address:WalletAddress",
            "to_sign_base64:string"
        ]
    );
    assert_eq!(
        string_list("movement.kvx", WalletSignRequest, "optional"),
        [
            "settlement_domain:SettlementDomain",
            "chain_id:EvmChainId",
            nonce,
            "max_fee_per_gas:Amount",
            "max_priority_fee_per_gas:Amount"
        ]
    );
    for field in [
        "chain_id:EvmChainId",
        nonce,
        "max_fee_per_gas:Amount",
        "max_priority_fee_per_gas:Amount",
        "settlement_domain:SettlementDomain",
    ] {
        optional_field("movement.kvx", WalletSignRequest, field);
    }
    assert_eq!(
        string(
            "movement.kvx",
            WalletSignRequest,
            "settlement_domain.retired"
        ),
        "true"
    );
    assert_eq!(
        string("movement.kvx", WalletSignRequest, "settlement_domain"),
        "paxeer"
    );
    assert!(string("movement.kvx", "module", "bank_sends").contains("bank 0x1001 sends"));
    for scalar in ["scalar.EvmChainId", "scalar.EvmNonce"] {
        for (key, expected) in [
            ("json", "string"),
            ("format", "decimal"),
            ("rust", "u64"),
            ("typescript", "bigint"),
        ] {
            assert_eq!(string("movement.kvx", scalar, key), expected);
        }
    }
    assert!(
        string_list("intent.kvx", "type.IntentLegBinding", "required")
            .contains(&"account_sequence:integer".to_owned())
    );
    let binding = string("intent.kvx", "type.IntentLegBinding", "account_sequence");
    assert!(
        binding.contains("decision.1003")
            && binding.contains("pending tag")
            && binding.contains("nonce")
    );
    for (operation, shape) in [
        ("intent.plan", "IntentPlan"),
        ("intent.submit", "IntentSubmission"),
    ] {
        response("intent.kvx", operation, shape);
        optional_field("intent.kvx", &format!("type.{shape}"), "tx_hash:WalletTxId");
    }
    for (operation, request) in [
        ("native.send.access.begin", "NativeSendAccessRequest"),
        ("native.send.access.confirm", "NativeSendAccessConfirmation"),
    ] {
        assert_eq!(
            string("v1.kvx", &format!("operation.{operation}"), "request"),
            request
        );
        optional_field("v1.kvx", &format!("type.{request}"), nonce);
    }
    for operation in ["onboarding.status", "onboarding.resume"] {
        response("identity.kvx", operation, "OnboardingJourney");
    }
    additive_journey(
        "identity.kvx",
        "type.OnboardingJourney",
        &["sponsor:WalletAddress", "tx_hash:WalletTxId"],
    );
    for field in ["sponsor:WalletAddress", "tx_hash:WalletTxId"] {
        optional_field("identity.kvx", "type.OnboardingJourney", field);
    }
    let sponsorship = string("identity.kvx", "module", "sponsorship");
    assert!(
        sponsorship.contains("gas station sponsorship")
            && sponsorship.contains("bindLayerX")
            && sponsorship.contains("LayerXBound")
    );
    for (operation, request, answer) in [
        ("move.quote", "MoveQuoteRequest", "MoveQuote"),
        ("move.commit", "MoveCommitRequest", "Journey"),
    ] {
        assert_eq!(
            string("movement.kvx", &format!("operation.{operation}"), "request"),
            request
        );
        response("movement.kvx", operation, answer);
    }
    assert_eq!(
        string_list("movement.kvx", "type.MoveQuoteRequest", "required"),
        ["source:string", "destination:string", "money:Money"]
    );
    assert_eq!(
        string_list("movement.kvx", "type.MoveQuote", "required"),
        [
            "quote_id:QuoteId",
            "description_copy_key:CopyKey",
            "mechanism:MoveMechanism",
            "money:Money",
            "fee_estimate:Money",
            "fee_ceiling:Money",
            "arrival_estimate:Timestamp",
            "expires_at:Timestamp"
        ]
    );
    assert_eq!(
        string_list("movement.kvx", "type.MoveQuote", "optional"),
        [
            "irreversibility_copy_key:CopyKey",
            "native_send_access:NativeSendAccessRequest"
        ]
    );
    assert_eq!(
        string_list("movement.kvx", "type.MoveCommitRequest", "required"),
        ["quote_id:QuoteId"]
    );
    assert_eq!(
        string_list("movement.kvx", "type.MoveCommitRequest", "optional"),
        ["native_access_id:string"]
    );
}

#[test]
fn money_types_carry_an_optional_rung() {
    assert_eq!(
        string_list("home.kvx", "type.Rung", "variants"),
        ["submitted", "receipted", "committed"]
    );
    assert_eq!(
        string_list("journeys.kvx", "type.VerificationLevel", "rung_mapping"),
        [
            "submitted:unverified",
            "receipted:receipt-verified",
            "committed:checkpoint-finalised"
        ]
    );
    for (file, section) in [
        ("home.kvx", AccountBalance),
        ("agents.kvx", "type.AgentSpend"),
        ("agents.kvx", "type.VerifiedMoney"),
        ("program-approvals.kvx", "type.ProgramApprovalBudget"),
    ] {
        optional_field(file, section, rung);
        assert!(
            string_list(file, section, "required")
                .contains(&"verification:VerificationLevel".to_owned()),
            "{section} retains its verification field"
        );
    }
    for (file, operation, answer) in [
        ("home.kvx", "account.balance", "AccountBalance"),
        ("home.kvx", "home.summary", "HomeSummary"),
        ("agents.kvx", "agent.create", "AgentCreationJourney"),
        (
            "program-approvals.kvx",
            "approval.program.approve",
            "ProgramApprovalDecision",
        ),
        ("activity.kvx", "activity.export.evidence", "ExportArtefact"),
    ] {
        response(file, operation, answer);
        optional_field(file, &format!("type.{answer}"), rung);
    }
    additive_journey("agents.kvx", "type.AgentCreationJourney", &[rung]);
    assert!(string_list("home.kvx", "type.HomeSummary", "required")
        .contains(&"balance:AccountBalance".to_owned()));
}

fn copy_tree(from: &Path, to: &Path) {
    fs::create_dir(to).unwrap_or_else(|error| panic!("create {}: {error}", to.display()));
    for entry in
        fs::read_dir(from).unwrap_or_else(|error| panic!("read {}: {error}", from.display()))
    {
        let entry = entry.unwrap_or_else(|error| panic!("read directory entry: {error}"));
        let source = entry.path();
        let target = to.join(entry.file_name());
        if source.is_dir() {
            copy_tree(&source, &target);
        } else {
            fs::copy(&source, &target)
                .unwrap_or_else(|error| panic!("copy {}: {error}", source.display()));
        }
    }
}

struct SchemaCopy(PathBuf);

impl SchemaCopy {
    fn new() -> Self {
        let root = std::env::temp_dir().join(format!(
            "layerx-merged-schema-shapes-{}",
            std::process::id()
        ));
        copy_tree(&real_schema_root(), &root);
        Self(root)
    }

    fn write(&self, file: &str, contents: &str) {
        fs::write(self.0.join(file), contents)
            .unwrap_or_else(|error| panic!("write {file}: {error}"));
    }
}

impl Drop for SchemaCopy {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).unwrap_or_else(|error| panic!("remove schema copy: {error}"));
    }
}

#[test]
fn merged_schema_shapes_are_additive_over_the_baseline() {
    let root = real_schema_root();
    let report = human_api_schema(&root)
        .unwrap_or_else(|violations| panic!("real schema conformance: {violations:#?}"));
    assert_eq!(report.additions_beyond_baseline, 0);
    assert_eq!(
        read_schema(BASELINE_FILE),
        baseline_manifest(&root)
            .unwrap_or_else(|violations| panic!("render baseline: {violations:#?}")),
        "baseline must be refreshed from the real schema"
    );

    let copy = SchemaCopy::new();
    let home = read_schema("home.kvx");
    copy.write(
        "home.kvx",
        &home.replacen("rung:Rung", "rung:Rung\",\"note:string", 1),
    );
    let additive = human_api_schema(&copy.0).unwrap_or_else(|violations| {
        panic!("optional additive declaration refused: {violations:#?}")
    });
    assert_eq!(additive.additions_beyond_baseline, 0);

    copy.write(
        "home.kvx",
        &home.replacen("optional = [\"rung:Rung\"]\n", "", 1),
    );
    let removed = human_api_schema(&copy.0).unwrap_err();
    assert!(
        removed
            .iter()
            .any(|violation| violation.rule == "breaking-declaration-removal"
                && violation
                    .detail
                    .contains("home.kvx:type.AccountBalance.optional")),
        "removing an optional field must fail the baseline: {removed:#?}"
    );

    copy.write("home.kvx", &home.replacen("rung:Rung", "rung:string", 1));
    let changed = human_api_schema(&copy.0).unwrap_err();
    assert!(
        changed
            .iter()
            .any(|violation| violation.rule == "breaking-declaration-change"
                && violation
                    .detail
                    .contains("home.kvx:type.AccountBalance.optional")),
        "widening an optional field must fail the baseline: {changed:#?}"
    );
}
