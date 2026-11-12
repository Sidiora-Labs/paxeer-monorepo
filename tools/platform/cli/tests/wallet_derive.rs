mod common;

use std::os::unix::fs::PermissionsExt as _;

use common::{envelope, error_envelope, Cli};
use serde_json::Value;

const FIXTURE: &str =
    include_str!("../../../../sdk/platform/conformance/fixtures/account-derivation-v1.json");

fn fixture() -> Value {
    serde_json::from_str(FIXTURE)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "the shared derivation fixture is JSON"))
}

#[test]
fn derive_reads_the_phrase_from_stdin_and_matches_the_shared_fixture() {
    if !common::credential_environment() {
        return;
    }
    let cli = Cli::new();
    let fixture = fixture();
    for vector in fixture["mnemonic_vectors"]
        .as_array()
        .unwrap_or_else(|| panic!("{}", "vectors"))
    {
        if vector["passphrase"] != "" {
            continue;
        }
        for account in vector["accounts"]
            .as_array()
            .unwrap_or_else(|| panic!("{}", "accounts"))
        {
            let index = account["index"]
                .as_u64()
                .unwrap_or_else(|| panic!("{}", "index"))
                .to_string();
            let output = cli.run_with_stdin(
                &["--json", "wallet", "derive", "--index", &index],
                &format!(
                    "{}\n",
                    vector["mnemonic"]
                        .as_str()
                        .unwrap_or_else(|| panic!("{}", "mnemonic"))
                ),
            );
            assert!(
                output.status.success(),
                "{}",
                String::from_utf8_lossy(&output.stderr)
            );
            let result = envelope(&output);
            assert_eq!(result["kind"], "wallet.derived");
            assert_eq!(result["data"]["evm_address"], account["evm_address"]);
            assert_eq!(result["data"]["did"], account["did"]);
            assert_eq!(result["data"]["evm_path"], account["evm_path"]);
            assert_eq!(result["data"]["layerx_path"], account["layerx_path"]);
            assert!(result["data"].get("private_keys_file").is_none());
        }
    }
}

#[test]
fn derive_reads_files_and_exports_keys_only_to_an_owner_only_file() {
    if !common::credential_environment() {
        return;
    }
    let cli = Cli::new();
    let fixture = fixture();
    let vector = &fixture["mnemonic_vectors"][1];
    let account = &vector["accounts"][1];
    let directory =
        tempfile::tempdir().unwrap_or_else(|error| panic!("{}: {error:?}", "temporary directory"));
    let phrase = directory.path().join("phrase");
    let passphrase = directory.path().join("passphrase");
    let keys = directory.path().join("keys.json");
    std::fs::write(
        &phrase,
        vector["mnemonic"]
            .as_str()
            .unwrap_or_else(|| panic!("{}", "mnemonic")),
    )
    .unwrap_or_else(|error| panic!("{}: {error:?}", "phrase"));
    std::fs::write(
        &passphrase,
        format!(
            "{}\n",
            vector["passphrase"]
                .as_str()
                .unwrap_or_else(|| panic!("{}", "passphrase"))
        ),
    )
    .unwrap_or_else(|error| panic!("{}: {error:?}", "passphrase"));
    let arguments = [
        "--json",
        "wallet",
        "derive",
        "--index",
        "1",
        "--mnemonic-file",
        phrase.to_str().unwrap_or_else(|| panic!("{}", "path")),
        "--passphrase-file",
        passphrase.to_str().unwrap_or_else(|| panic!("{}", "path")),
        "--export-private-keys",
        keys.to_str().unwrap_or_else(|| panic!("{}", "path")),
    ];
    let output = cli.run(&arguments);
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    let result = envelope(&output);
    assert_eq!(result["data"]["evm_address"], account["evm_address"]);
    assert_eq!(result["data"]["did"], account["did"]);
    let mode = std::fs::metadata(&keys)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "key file"))
        .permissions()
        .mode();
    assert_eq!(mode & 0o777, 0o600);
    let exported: Value = serde_json::from_str(
        &std::fs::read_to_string(&keys).unwrap_or_else(|error| panic!("{}: {error:?}", "key file")),
    )
    .unwrap_or_else(|error| panic!("{}: {error:?}", "JSON"));
    assert_eq!(exported["did"], account["did"]);
    let seed = exported["layerx_seed"]
        .as_str()
        .unwrap_or_else(|| panic!("{}", "seed"));
    let evm = exported["evm_private_key"]
        .as_str()
        .unwrap_or_else(|| panic!("{}", "EVM key"));
    assert_eq!(seed.len(), 64);
    assert_eq!(evm.len(), 64);
    let printed = format!(
        "{}{}",
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );
    assert!(!printed.contains(seed));
    assert!(!printed.contains(evm));

    let again = cli.run(&arguments);
    assert!(!again.status.success());
    assert_eq!(
        error_envelope(&again)["error"]["code"],
        "private_key_export_refused"
    );
}

#[test]
fn derive_refuses_bad_phrases_argv_phrases_and_bind_without_an_endpoint() {
    if !common::credential_environment() {
        return;
    }
    let cli = Cli::new();
    let bad = cli.run_with_stdin(&["--json", "wallet", "derive"], "abandon abandon abandon\n");
    assert!(!bad.status.success());
    assert_eq!(error_envelope(&bad)["error"]["code"], "invalid_mnemonic");

    let argv = cli.run_with_stdin(
        &["--json", "wallet", "derive", "abandon abandon abandon"],
        "",
    );
    assert!(!argv.status.success());

    let unbound = cli.run_with_stdin(
        &["--json", "wallet", "derive", "--bind"],
        "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about\n",
    );
    assert!(!unbound.status.success());
    assert_eq!(
        error_envelope(&unbound)["error"]["code"],
        "bind_requires_rpc"
    );
}

#[test]
fn derive_binding_vectors_preserve_pair_refusals_and_recovered_nonce(
) -> Result<(), Box<dyn std::error::Error>> {
    use layerx_client::account_binding::{
        plan_bind, sign_bind_transaction, BindError, BindFees, BindPlan,
    };
    use layerx_crypto::account_derivation::derive_from_mnemonic;
    use layerx_crypto::evm_transaction::{decode_signed, recover_sender};
    let corpus = fixture();
    let vector = &corpus["mnemonic_vectors"][0];
    for expected in vector["accounts"]
        .as_array()
        .unwrap_or_else(|| panic!("{}", "derived accounts"))
    {
        let account = derive_from_mnemonic(
            vector["mnemonic"]
                .as_str()
                .unwrap_or_else(|| panic!("{}", "mnemonic")),
            "",
            u32::try_from(
                expected["index"]
                    .as_u64()
                    .unwrap_or_else(|| panic!("{}", "index")),
            )
            .unwrap_or_else(|error| panic!("{error:?}")),
        )?;
        assert_eq!(
            account.evm_address(),
            layerx_platform_cli::encoding::fixed_hex::<20>(
                "fixture EVM address",
                expected["evm_address"]
                    .as_str()
                    .unwrap_or_else(|| panic!("{}", "EVM address"))
                    .strip_prefix("0x")
                    .unwrap_or_else(|| panic!("{}", "address prefix")),
            )?
        );
        assert_eq!(account.did(), expected["did"]);
        let BindPlan::Bind(call) = plan_bind(&account, 125, None, 0)? else {
            panic!("unbound pair")
        };
        call.signed_binding()
            .verify()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "real binding consent"));
        let raw = sign_bind_transaction(
            &account,
            125,
            &call,
            BindFees {
                evm_nonce: 0,
                max_priority_fee_per_gas: 1_000_000_000,
                max_fee_per_gas: 2_000_000_000,
                gas_limit: 200_000,
            },
        )?;
        assert_eq!(
            layerx_platform_cli::encoding::hex_encode(&raw),
            expected["bind"]["transaction"]["raw"]
        );
        let (transaction, signature) = decode_signed(&raw)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "production EIP-1559 codec"));
        assert_eq!(
            recover_sender(&transaction, &signature)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "real sender")),
            account.evm_address()
        );
        assert!(matches!(
            plan_bind(&account, 125, Some(account.layerx_public_key()), 1)?,
            BindPlan::AlreadyBound
        ));
        assert_eq!(
            plan_bind(&account, 125, Some([0x61; 32]), 1)
                .err()
                .unwrap_or_else(|| panic!("{}", "other identity")),
            BindError::BoundToDifferentDid { bound: [0x61; 32] }
        );
        let BindPlan::Bind(recovered) = plan_bind(&account, 125, None, 7)? else {
            panic!("recovered nonce")
        };
        assert_eq!(recovered.signed_binding().binding().nonce(), 7);
        assert!(recovered
            .signed_binding()
            .binding()
            .verify(
                &call.signed_binding().public_key(),
                &call.signed_binding().signature()
            )
            .is_err());
        recovered
            .signed_binding()
            .verify()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "recovered consent"));
    }
    Ok(())
}

#[test]
fn derive_bind_accepts_the_node_root_transport() -> Result<(), String> {
    let listener = std::net::TcpListener::bind("127.0.0.1:0").map_err(|error| error.to_string())?;
    let root = format!(
        "http://{}",
        listener.local_addr().map_err(|error| error.to_string())?
    );
    let _client = layerx_platform_cli::rpc::RpcClient::new(&root, None)?;
    Ok(())
}
