use layerx_human_service::onboarding::gas_station::{
    Call, GasStationClient, GasStationError, QuoteRequest,
};
use layerx_platform_internal::secret::hex as lowercase_hex;
use serde_json::{json, Value};
use sha3::{Digest, Keccak256};

const MAX_WORD: &str =
    "115792089237316195423570985008687907853269984665640564039457584007913129639935";

fn hex(bytes: &[u8]) -> String {
    format!("0x{}", lowercase_hex(bytes))
}

#[test]
fn gas_station_quote_body_uses_the_station_field_names() -> Result<(), GasStationError> {
    let selector = Keccak256::digest(b"bindLayerX(bytes32,bytes)");
    let mut data = selector[..4].to_vec();
    data.extend_from_slice(&[0x42; 32]);
    let mut offset = [0; 32];
    offset[31] = 64;
    data.extend_from_slice(&offset);
    let mut length = [0; 32];
    length[31] = 65;
    data.extend_from_slice(&length);
    data.extend_from_slice(&[0x33; 65]);
    data.resize(4 + 32 * 6, 0);
    let mut binding = [0; 20];
    binding[18..].copy_from_slice(&[0x10, 0x04]);
    let request = QuoteRequest {
        account: [0x11; 20],
        nonce: [0xff; 32],
        calls: vec![Call {
            to: binding,
            value: [0; 32],
            data: data.clone(),
        }],
        max_token_amount: 2_100_000,
        gas_cost: 1_000_000_000_000_000_000,
        token: [0x22; 20],
        decimals: 6,
    };
    let body = GasStationClient::quote_body(&request)?;
    let mut keys: Vec<_> = body
        .as_object()
        .ok_or(GasStationError::Malformed)?
        .keys()
        .map(String::as_str)
        .collect();
    keys.sort_unstable();
    assert_eq!(
        keys,
        [
            "account",
            "calls",
            "chainId",
            "decimals",
            "gasCost",
            "maxTokenAmount",
            "nonce",
            "token",
        ]
    );
    assert_eq!(
        body,
        json!({
            "account": "0x1111111111111111111111111111111111111111",
            "nonce": MAX_WORD,
            "calls": [{
                "to": "0x0000000000000000000000000000000000001004",
                "value": "0",
                "data": hex(&data),
            }],
            "maxTokenAmount": "2100000",
            "gasCost": "1000000000000000000",
            "chainId": "125",
            "token": "0x2222222222222222222222222222222222222222",
            "decimals": 6,
        })
    );
    Ok(())
}

fn quote_answer() -> Value {
    json!({
        "quote": {
            "sponsor": hex(&[0x33; 20]),
            "token": hex(&[0x22; 20]),
            "maxTokenAmount": MAX_WORD,
            "tokenAmount": "2000000",
            "deadline": "1000",
            "quoteNonce": MAX_WORD,
            "gasCost": "1000000000000000000",
            "decimals": 6,
        },
        "relayerSignature": hex(&[0x44; 65]),
    })
}

#[test]
fn gas_station_quote_for_another_chain_is_refused() -> Result<(), GasStationError> {
    let answer = quote_answer();
    let signed = GasStationClient::decode_quote(&answer)?;
    assert_eq!(signed.quote.sponsor, [0x33; 20]);
    assert_eq!(signed.quote.token, [0x22; 20]);
    assert_eq!(signed.quote.max_token_amount, [0xff; 32]);
    assert_eq!(signed.quote.nonce, [0xff; 32]);
    let mut amount = [0; 32];
    amount[16..].copy_from_slice(&2_000_000_u128.to_be_bytes());
    assert_eq!(signed.quote.token_amount, amount);
    let mut deadline = [0; 32];
    deadline[24..].copy_from_slice(&1000_u64.to_be_bytes());
    assert_eq!(signed.quote.deadline, deadline);
    let mut gas_cost = [0; 32];
    gas_cost[16..].copy_from_slice(&1_000_000_000_000_000_000_u128.to_be_bytes());
    assert_eq!(signed.quote.gas_cost, gas_cost);
    assert_eq!(signed.signature, [0x44; 65]);
    assert_eq!(signed.decimals, 6);

    for chain in [json!("125"), json!(125)] {
        let mut same_chain = answer.clone();
        same_chain["chainId"] = chain.clone();
        same_chain["quote"]["chainId"] = chain;
        assert_eq!(GasStationClient::decode_quote(&same_chain)?, signed);
    }
    for chain in [json!("1"), json!(1), json!(null), json!("0125")] {
        let mut foreign = answer.clone();
        foreign["chainId"] = chain.clone();
        assert_eq!(
            GasStationClient::decode_quote(&foreign),
            Err(GasStationError::ChainMismatch)
        );
        let mut nested_foreign = answer.clone();
        nested_foreign["chainId"] = json!("125");
        nested_foreign["quote"]["chainId"] = chain;
        assert_eq!(
            GasStationClient::decode_quote(&nested_foreign),
            Err(GasStationError::ChainMismatch)
        );
    }
    for size in [0, 64, 66] {
        let mut malformed = answer.clone();
        malformed["relayerSignature"] = json!(hex(&vec![0x44; size]));
        assert_eq!(
            GasStationClient::decode_quote(&malformed),
            Err(GasStationError::Malformed)
        );
    }
    for amount in [
        "115792089237316195423570985008687907853269984665640564039457584007913129639936",
        "01",
        "-1",
        "",
    ] {
        let mut malformed = answer.clone();
        malformed["quote"]["quoteNonce"] = json!(amount);
        assert_eq!(
            GasStationClient::decode_quote(&malformed),
            Err(GasStationError::Malformed)
        );
    }
    Ok(())
}
