//! Gateway ABI tests copied from `services/platform/hosted/gateway/src/evm.rs`.
//! Public codecs are exercised through the gateway dev-dependency.

fn word_u64(value: u64) -> [u8; 32] {
    let mut word = [0_u8; 32];
    word[24..].copy_from_slice(&value.to_be_bytes());
    word
}

#[cfg(test)]
mod tests {
    use super::word_u64;
    use layerx_platform_gateway::evm::*;
    use serde_json::Value;

    fn vectors() -> Value {
        serde_json::from_slice(include_bytes!("../fixtures/paxeer-abi-vectors.json"))
            .unwrap_or_else(|error| panic!("{error}"))
    }

    fn bytes(text: &str) -> Vec<u8> {
        (0..text.len() / 2)
            .map(|index| {
                u8::from_str_radix(&text[index * 2..index * 2 + 2], 16)
                    .unwrap_or_else(|error| panic!("{error}"))
            })
            .collect()
    }

    fn case(name: &str) -> (Vec<u8>, Vec<u8>, Value) {
        let document = vectors();
        let cases = document["cases"]
            .as_array()
            .unwrap_or_else(|| panic!("cases missing"))
            .clone();
        let found = cases
            .into_iter()
            .find(|case| case["name"] == name)
            .unwrap_or_else(|| panic!("{name} missing"));
        (
            bytes(found["calldata"].as_str().unwrap_or_default()),
            bytes(found["result"].as_str().unwrap_or_default()),
            found["out"].clone(),
        )
    }

    fn word32(text: &str) -> [u8; 32] {
        let mut word = [0_u8; 32];
        word.copy_from_slice(&bytes(text));
        word
    }

    #[test]
    fn merged_construct_addresses_are_exact() {
        for (address, expected) in [
            (
                LEDGER_PRECOMPILE,
                "0x000000000000000000000000000000000000101a",
            ),
            (PAY_PRECOMPILE, "0x000000000000000000000000000000000000101b"),
            (
                AUTH_PRECOMPILE,
                "0x000000000000000000000000000000000000101c",
            ),
            (
                PROGRAMS_PRECOMPILE,
                "0x000000000000000000000000000000000000101d",
            ),
        ] {
            assert_eq!(address_hex(&address), expected);
        }
    }

    #[test]
    fn all_balances_answer_decodes_exactly() {
        let (calldata, result, out) = case("bank.all_balances");
        let account = parse_address("0x102132435465768798a9bacbdcedfe0f1e2d3c4b")
            .unwrap_or_else(|| panic!("address"));
        assert_eq!(
            calldata_address(SELECTOR_BANK_ALL_BALANCES, &account),
            calldata
        );
        let coins = decode_coins(&result).unwrap_or_else(|error| panic!("{error:?}"));
        let expected = out["response"]
            .as_array()
            .unwrap_or_else(|| panic!("coins"));
        assert_eq!(expected.len(), 2);
        assert_eq!(coins.len(), expected.len());
        for ((denom, amount), expected) in coins.iter().zip(expected) {
            assert_eq!(Value::String(denom.clone()), expected["denom"]);
            assert_eq!(Value::String(uint256_decimal(amount)), expected["amount"]);
        }
        assert_eq!(coins[0].1, word_u64(123_456_789));
        let mut large = [0_u8; 32];
        large[6] = 1;
        assert_eq!(coins[1].1, large);
        for length in 0..result.len() {
            assert!(decode_coins(&result[..length]).is_err(), "length {length}");
        }
        let mut too_many = result.clone();
        too_many[32..64].copy_from_slice(&word_u64(257));
        assert_eq!(decode_coins(&too_many), Err(AbiError::Width));
        let mut bad_offset = result.clone();
        bad_offset[..32].copy_from_slice(&word_u64(33));
        assert_eq!(decode_coins(&bad_offset), Err(AbiError::Offset));
        bad_offset[..32].copy_from_slice(&word_u64(0));
        assert_eq!(decode_coins(&bad_offset), Err(AbiError::Offset));
        let mut bad_tuple = result.clone();
        bad_tuple[64..96].copy_from_slice(&word_u64(0));
        assert_eq!(decode_coins(&bad_tuple), Err(AbiError::Offset));
        let mut oversized_denom = result.clone();
        oversized_denom[192..224].copy_from_slice(&word_u64(4097));
        assert_eq!(decode_coins(&oversized_denom), Err(AbiError::Text));
        let mut invalid_text = result.clone();
        invalid_text[224] = 0xff;
        assert_eq!(decode_coins(&invalid_text), Err(AbiError::Text));
        let mut empty = word_u64(32).to_vec();
        empty.extend_from_slice(&word_u64(0));
        assert_eq!(decode_coins(&empty), Ok(Vec::new()));
    }

    #[test]
    fn precompile_addresses_and_selectors_match_the_generated_vectors() {
        let document = vectors();
        let precompiles = document["precompiles"]
            .as_object()
            .unwrap_or_else(|| panic!("precompiles missing"));
        for retired in ["custody", "anchor"] {
            assert!(!precompiles.contains_key(retired), "{retired}");
        }
        let addresses = [
            ("bank", BANK_PRECOMPILE),
            ("addr", ADDR_PRECOMPILE),
            ("exchange", EXCHANGE_PRECOMPILE),
            ("bridge", BRIDGE_PRECOMPILE),
            ("launchpad", LAUNCHPAD_PRECOMPILE),
            ("ledger", LEDGER_PRECOMPILE),
            ("pay", PAY_PRECOMPILE),
            ("auth", AUTH_PRECOMPILE),
            ("programs", PROGRAMS_PRECOMPILE),
        ];
        assert_eq!(precompiles.len(), addresses.len());
        for (name, address) in addresses {
            assert_eq!(precompiles[name], address_hex(&address), "{name}");
        }
        let cases = document["cases"]
            .as_array()
            .unwrap_or_else(|| panic!("cases missing"));
        assert_eq!(cases.len(), 6);
        for vector in cases {
            let contract = vector["contract"]
                .as_str()
                .unwrap_or_else(|| panic!("contract missing"));
            let name = vector["name"]
                .as_str()
                .unwrap_or_else(|| panic!("name missing"));
            assert!(matches!(contract, "addr" | "bank"), "{contract}");
            assert!(name.starts_with(&format!("{contract}.")), "{name}");
        }
        for (name, selector) in [
            ("addr.getUnifiedAccount", SELECTOR_GET_UNIFIED_ACCOUNT),
            ("addr.getLayerXDid", SELECTOR_GET_LAYERX_DID),
            ("addr.getEvmAddrByLayerX", SELECTOR_GET_EVM_ADDR_BY_LAYERX),
            ("addr.layerXBindNonce", SELECTOR_LAYERX_BIND_NONCE),
            ("bank.balance", SELECTOR_BANK_BALANCE),
            ("bank.all_balances", SELECTOR_BANK_ALL_BALANCES),
        ] {
            let (calldata, _, _) = case(name);
            assert_eq!(calldata.get(..4), Some(selector.as_slice()), "{name}");
            let vector = cases
                .iter()
                .find(|vector| vector["name"] == name)
                .unwrap_or_else(|| panic!("{name} missing"));
            assert_eq!(vector["selector"], hex_lower(&selector), "{name}");
        }
        let selectors = document["selectors"]
            .as_object()
            .unwrap_or_else(|| panic!("selectors missing"));
        for name in selectors.keys() {
            assert!(
                !name.starts_with("custody.") && !name.starts_with("anchor."),
                "{name}"
            );
        }
        assert_eq!(selectors.len(), 5);
        for (name, selector) in [
            ("bank.name", SELECTOR_BANK_NAME),
            ("bank.symbol", SELECTOR_BANK_SYMBOL),
            ("bank.decimals", SELECTOR_BANK_DECIMALS),
            ("bank.supply", SELECTOR_BANK_SUPPLY),
            ("ledger.getAsset", SELECTOR_GET_ASSET),
        ] {
            assert_eq!(selectors[name], hex_lower(&selector), "{name}");
        }
    }

    #[test]
    fn calldata_matches_the_go_ethereum_encoding() {
        let account = parse_address("0x102132435465768798a9bacbdcedfe0f1e2d3c4b")
            .unwrap_or_else(|| panic!("address"));
        let did = [0x61_u8; 32];
        for (name, calldata) in [
            (
                "addr.getUnifiedAccount",
                calldata_address(SELECTOR_GET_UNIFIED_ACCOUNT, &account),
            ),
            (
                "addr.getLayerXDid",
                calldata_address(SELECTOR_GET_LAYERX_DID, &account),
            ),
            (
                "addr.getEvmAddrByLayerX",
                calldata_word(SELECTOR_GET_EVM_ADDR_BY_LAYERX, &did),
            ),
            (
                "addr.layerXBindNonce",
                calldata_address(SELECTOR_LAYERX_BIND_NONCE, &account),
            ),
            (
                "bank.balance",
                calldata_address_string(SELECTOR_BANK_BALANCE, &account, "ulxp"),
            ),
            (
                "bank.all_balances",
                calldata_address(SELECTOR_BANK_ALL_BALANCES, &account),
            ),
        ] {
            let (expected, _, _) = case(name);
            assert_eq!(calldata, expected, "{name}");
        }
    }

    #[test]
    fn account_answers_decode_exactly() {
        let (_, result, out) = case("addr.getUnifiedAccount");
        let account = decode_unified_account(&result).unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(address_hex(&account.evm), out["evm"]);
        assert_eq!(Value::String(account.pax_address), out["paxAddr"]);
        assert_eq!(
            Value::String(hex_lower(&account.did_public_key)),
            out["didPublicKey"]
        );
        assert_eq!(
            Value::String(hex_lower(&account.layerx_main_account_id)),
            out["layerxMainAccountId"]
        );
        let (_, result, out) = case("addr.getLayerXDid");
        let (key, did) = decode_layerx_did(&result).unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(Value::String(hex_lower(&key)), out["didPublicKey"]);
        assert_eq!(Value::String(did), out["did"]);
        let (_, result, out) = case("addr.getEvmAddrByLayerX");
        let address = decode_address(&result).unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(address_hex(&address), out["evmAddr"]);
        let (_, result, out) = case("addr.layerXBindNonce");
        let nonce = decode_u64(&result).unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(Value::String(nonce.to_string()), out["nonce"]);
    }

    #[test]
    fn bank_balance_answer_decodes_exactly() {
        let (_, result, out) = case("bank.balance");
        let balance = decode_word(&result).unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(Value::String(uint256_decimal(&balance)), out["amount"]);
    }

    fn hex_lower(bytes: &[u8]) -> String {
        let mut text = String::with_capacity(bytes.len() * 2);
        for byte in bytes {
            text.push(char::from_digit(u32::from(byte >> 4), 16).unwrap_or('0'));
            text.push(char::from_digit(u32::from(byte & 0x0f), 16).unwrap_or('0'));
        }
        text
    }

    #[test]
    fn quantities_and_addresses_render_without_loss() {
        assert_eq!(uint256_decimal(&[0_u8; 32]), "0");
        assert_eq!(uint256_quantity(&[0_u8; 32]), "0x0");
        assert_eq!(uint256_decimal(&word32(&"ff".repeat(32))), u256_max());
        assert_eq!(
            uint256_quantity(&word32(&"ff".repeat(32))),
            format!("0x{}", "ff".repeat(32))
        );
        let mut one = [0_u8; 32];
        one[31] = 1;
        assert_eq!(uint256_quantity(&one), "0x1");
        assert_eq!(uint256_decimal(&one), "1");
        let mut sixteen = [0_u8; 32];
        sixteen[31] = 16;
        assert_eq!(uint256_quantity(&sixteen), "0x10");
        assert_eq!(
            parse_address("0x00000000000000000000000000000000000000a1"),
            Some([0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xa1])
        );
        for invalid in ["0x", "0xzz", "00", &"ab".repeat(21)] {
            assert_eq!(parse_address(invalid), None);
        }
    }

    fn u256_max() -> String {
        "115792089237316195423570985008687907853269984665640564039457584007913129639935".to_owned()
    }

    #[test]
    fn sequencer_receipt_is_the_only_relayed_pax_name() {
        assert!(relayable_method("pax_sequencerReceipt"));
        for method in [
            "pax_other",
            "pax_",
            "pax_sequencerReceipt_other",
            "pax_sequencerreceipt",
            "eth_sign",
        ] {
            assert!(!relayable_method(method), "{method}");
        }
    }

    #[test]
    fn relayed_methods_exclude_node_signing_and_subscriptions() {
        for method in [
            "eth_blockNumber",
            "eth_call",
            "eth_sendRawTransaction",
            "eth_getBalance",
            "net_version",
            "web3_clientVersion",
        ] {
            assert!(relayable_method(method), "{method}");
        }
        for method in [
            "eth_accounts",
            "eth_coinbase",
            "eth_sendTransaction",
            "eth_sign",
            "eth_signTransaction",
            "eth_signTypedData",
            "eth_signTypedData_v4",
            "eth_mining",
            "eth_subscribe",
            "eth_unsubscribe",
            "lx_getNodeInfo",
            "px_getNetwork",
            "eth_call ",
            "eth_",
        ] {
            assert_eq!(relayable_method(method), method == "eth_", "{method}");
        }
        assert!(!relayable_method(&format!("eth_{}", "a".repeat(64))));
        assert!(is_evm_namespace("eth_subscribe"));
        assert!(!is_evm_namespace("px_getAccount"));
    }

    #[test]
    fn malformed_answers_are_refused() {
        assert_eq!(decode_u64(&[0_u8; 31]), Err(AbiError::Truncated));
        assert_eq!(decode_u64(&[0xff_u8; 32]), Err(AbiError::Width));
        assert_eq!(decode_address(&[0xff_u8; 32]), Err(AbiError::Width));
        assert_eq!(
            decode_unified_account(&[0_u8; 32]),
            Err(AbiError::Truncated)
        );
        let mut outside = [0_u8; 32];
        outside[31] = 64;
        assert_eq!(Answer::new(&outside).string(0, 0), Err(AbiError::Offset));
        let mut unaligned = [0_u8; 64];
        unaligned[31] = 8;
        assert_eq!(Answer::new(&unaligned).string(0, 0), Err(AbiError::Offset));
        let mut non_boolean = [0_u8; 64];
        non_boolean[63] = 2;
        assert_eq!(Answer::new(&non_boolean).bool(1), Err(AbiError::Width));
        let (_, result, _) = case("addr.getUnifiedAccount");
        assert_eq!(
            decode_unified_account(&result[..result.len() - 32]),
            Err(AbiError::Truncated)
        );
    }
}
