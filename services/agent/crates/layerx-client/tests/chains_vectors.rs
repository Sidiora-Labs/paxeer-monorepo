use std::path::Path;

use layerx_client::chains::{
    chain_rpc_url, decode_call, decode_event, decode_get_chain, decode_params, encode_call,
    virtual_chain_id, ChainCall, ChainEvent, ChainRecord, ChainsParams, CHAINS_PRECOMPILE,
    EVENT_SIGNATURES, METHOD_SIGNATURES,
};
use serde_json::Value;

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

fn text(value: &Value) -> &str {
    value
        .as_str()
        .unwrap_or_else(|| panic!("expected string: {value}"))
}

fn integer<T: TryFrom<u64>>(value: &Value) -> T
where
    T::Error: std::fmt::Debug,
{
    must(T::try_from(
        value
            .as_u64()
            .unwrap_or_else(|| panic!("expected integer: {value}")),
    ))
}

fn unhex(value: &str) -> Vec<u8> {
    let hex = value
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("missing hex prefix"));
    assert_eq!(hex.len() % 2, 0);
    hex.as_bytes()
        .chunks_exact(2)
        .map(|pair| must(u8::from_str_radix(must(std::str::from_utf8(pair)), 16)))
        .collect()
}

fn fixed<const N: usize>(value: &str) -> [u8; N] {
    must(unhex(value).try_into())
}

fn decimal_word(value: &str) -> [u8; 32] {
    assert!(!value.is_empty());
    let mut result = [0_u8; 32];
    for digit in value.bytes() {
        assert!(digit.is_ascii_digit());
        let mut carry = u16::from(digit - b'0');
        for byte in result.iter_mut().rev() {
            carry += u16::from(*byte) * 10;
            *byte = must(u8::try_from(carry & 255));
            carry >>= 8;
        }
        assert_eq!(carry, 0, "uint256 overflow");
    }
    result
}

fn vector_call(name: &str, args: &[Value]) -> ChainCall {
    match name {
        "createChain" => ChainCall::CreateChain {
            name: text(&args[0]).to_owned(),
            gas_allowance: integer(&args[1]),
            byte_allowance: integer(&args[2]),
            fee_share_bps: integer(&args[3]),
        },
        "fundChain" => ChainCall::FundChain(integer(&args[0])),
        "pauseChain" => ChainCall::PauseChain(integer(&args[0])),
        "resumeChain" => ChainCall::ResumeChain(integer(&args[0])),
        "retireChain" => ChainCall::RetireChain(integer(&args[0])),
        "governancePause" => ChainCall::GovernancePause {
            id: integer(&args[0]),
            paused: args[1].as_bool().unwrap_or_else(|| panic!("expected bool")),
        },
        "setOperator" => ChainCall::SetOperator {
            id: integer(&args[0]),
            operator: fixed(text(&args[1])),
        },
        "getChain" => ChainCall::GetChain(integer(&args[0])),
        "chainByName" => ChainCall::ChainByName(text(&args[0]).to_owned()),
        "chainCount" => ChainCall::ChainCount,
        "chainIdOf" => ChainCall::ChainIdOf(integer(&args[0])),
        "params" => ChainCall::Params,
        other => panic!("unknown vector method {other}"),
    }
}

#[test]
fn chains_vectors_match_and_round_trip() {
    let path = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../..")
        .join("tests/vectors/chains/abi-vectors.json");
    let vectors: Value = must(serde_json::from_slice(&must(std::fs::read(path))));
    assert_eq!(CHAINS_PRECOMPILE, fixed::<20>(text(&vectors["address"])));
    let methods = vectors["methods"]
        .as_array()
        .unwrap_or_else(|| panic!("missing methods"));
    assert_eq!(methods.len(), 12);
    for (vector, signature) in methods.iter().zip(METHOD_SIGNATURES) {
        let call = vector_call(
            text(&vector["name"]),
            vector["args"]
                .as_array()
                .unwrap_or_else(|| panic!("missing args")),
        );
        assert_eq!(call.signature(), signature);
        assert_eq!(call.signature(), text(&vector["signature"]));
        assert_eq!(call.selector(), fixed::<4>(text(&vector["selector"])));
        let encoded = encode_call(&call);
        let expected = unhex(text(&vector["calldata"]));
        assert_eq!(encoded, expected, "{} calldata", call.signature());
        assert_eq!(must(decode_call(&expected)), call);
        assert_eq!(must(decode_call(&encoded)), call);
        let mut trailing = encoded.clone();
        trailing.push(0);
        assert!(decode_call(&trailing).is_err());
        assert!(decode_call(&encoded[..encoded.len() - 1]).is_err());
    }

    let value = &vectors["getChain"]["decoded"];
    let expected_chain = ChainRecord {
        id: integer(&value["id"]),
        name: text(&value["name"]).to_owned(),
        operator: fixed(text(&value["operator"])),
        status: integer(&value["status"]),
        chain_id: integer(&value["chainId"]),
        gas_allowance: integer(&value["gasAllowance"]),
        byte_allowance: integer(&value["byteAllowance"]),
        fee_share_bps: integer(&value["feeShareBps"]),
        bond: decimal_word(text(&value["bond"])),
        base_fee_per_gas: decimal_word(text(&value["baseFeePerGas"])),
        created_height: integer(&value["createdHeight"]),
    };
    let chain_bytes = unhex(text(&vectors["getChain"]["returndata"]));
    assert_eq!(must(decode_get_chain(&chain_bytes)), expected_chain);
    assert_eq!(expected_chain.encode(), chain_bytes);
    assert_eq!(
        must(decode_get_chain(&expected_chain.encode())),
        expected_chain
    );

    let value = &vectors["params"]["decoded"];
    let expected_params = ChainsParams {
        authority: fixed(text(&value["authority"])),
        min_bond: decimal_word(text(&value["minBond"])),
        creation_fee: decimal_word(text(&value["creationFee"])),
        min_gas_allowance: integer(&value["minGasAllowance"]),
        max_gas_allowance: integer(&value["maxGasAllowance"]),
        total_gas_allowance_left: integer(&value["totalGasAllowanceLeft"]),
        max_chains: integer(&value["maxChains"]),
    };
    let params_bytes = unhex(text(&vectors["params"]["returndata"]));
    assert_eq!(must(decode_params(&params_bytes)), expected_params);
    assert_eq!(expected_params.encode(), params_bytes);
    assert_eq!(
        must(decode_params(&expected_params.encode())),
        expected_params
    );

    let events = [
        ChainEvent::Created {
            id: expected_chain.id,
            name: expected_chain.name.clone(),
            operator: expected_chain.operator,
            chain_id: expected_chain.chain_id,
        },
        ChainEvent::StatusChanged {
            id: expected_chain.id,
            status: expected_chain.status,
        },
        ChainEvent::Funded {
            id: expected_chain.id,
            from: expected_chain.operator,
            amount: expected_chain.bond,
        },
        ChainEvent::OperatorChanged {
            id: expected_chain.id,
            operator: expected_chain.operator,
        },
    ];
    let event_vectors = vectors["events"]
        .as_array()
        .unwrap_or_else(|| panic!("missing events"));
    assert_eq!(event_vectors.len(), 4);
    for ((event, vector), signature) in events.iter().zip(event_vectors).zip(EVENT_SIGNATURES) {
        assert_eq!(event.signature(), signature);
        assert_eq!(event.signature(), text(&vector["signature"]));
        assert_eq!(event.topic0(), fixed::<32>(text(&vector["topic0"])));
        let (topics, data) = event.encode();
        assert_eq!(topics[0], fixed::<32>(text(&vector["topic0"])));
        assert_eq!(topics[1], decimal_word("1"));
        if topics.len() == 3 {
            assert_eq!(&topics[2][..12], &[0; 12]);
            assert_eq!(&topics[2][12..], &expected_chain.operator);
        }
        match event {
            ChainEvent::Created { .. } => {
                let mut expected = [
                    decimal_word("64"),
                    decimal_word("125000001"),
                    decimal_word("4"),
                ]
                .concat();
                expected.extend_from_slice(b"demo");
                expected.resize(128, 0);
                assert_eq!(data, expected);
            }
            ChainEvent::StatusChanged { .. } => assert_eq!(data, decimal_word("1")),
            ChainEvent::Funded { .. } => assert_eq!(data, expected_chain.bond),
            ChainEvent::OperatorChanged { .. } => assert!(data.is_empty()),
        }
        assert_eq!(must(decode_event(&topics, &data)), *event);
        assert!(decode_event(&topics[..topics.len() - 1], &data).is_err());
        let mut excess = topics.clone();
        excess.push([0; 32]);
        assert!(decode_event(&excess, &data).is_err());
        let mut bad_id = topics.clone();
        bad_id[1][27] = 1;
        assert!(decode_event(&bad_id, &data).is_err());
        let mut trailing = data;
        trailing.push(0);
        assert!(decode_event(&topics, &trailing).is_err());
    }
    assert!(decode_event(&[], &[]).is_err());
    assert!(decode_event(&[[0; 32]], &[]).is_err());

    assert_eq!(virtual_chain_id(1), 125_000_001);
    assert_eq!(virtual_chain_id(0), 125_000_000);
    assert_eq!(virtual_chain_id(u32::MAX), 4_419_967_295);
    assert_eq!(
        chain_rpc_url("https://rpc.example", 1),
        "https://rpc.example/chain/1"
    );
    assert_eq!(
        chain_rpc_url("https://rpc.example/evm///", u32::MAX),
        "https://rpc.example/evm/chain/4294967295"
    );
    assert_eq!(
        chain_rpc_url("wss://rpc.example/", 1),
        "wss://rpc.example/chain/1"
    );

    for name in [
        String::new(),
        "x".repeat(32),
        "x".repeat(33),
        "链🚀".repeat(20),
    ] {
        for call in [
            ChainCall::ChainByName(name.clone()),
            ChainCall::CreateChain {
                name: name.clone(),
                gas_allowance: u64::MAX,
                byte_allowance: u64::MAX,
                fee_share_bps: u16::MAX,
            },
        ] {
            assert_eq!(must(decode_call(&encode_call(&call))), call);
        }
        let record = ChainRecord {
            name,
            id: u32::MAX,
            status: u8::MAX,
            chain_id: u64::MAX,
            gas_allowance: u64::MAX,
            byte_allowance: u64::MAX,
            fee_share_bps: u16::MAX,
            bond: [255; 32],
            base_fee_per_gas: [255; 32],
            created_height: u64::MAX,
            ..expected_chain.clone()
        };
        assert_eq!(must(decode_get_chain(&record.encode())), record);
    }
    for paused in [false, true] {
        let call = ChainCall::GovernancePause {
            id: u32::MAX,
            paused,
        };
        assert_eq!(must(decode_call(&encode_call(&call))), call);
    }
    let full_params = ChainsParams {
        min_bond: [255; 32],
        creation_fee: [255; 32],
        min_gas_allowance: u64::MAX,
        max_gas_allowance: u64::MAX,
        total_gas_allowance_left: u64::MAX,
        max_chains: u32::MAX,
        ..expected_params
    };
    assert_eq!(must(decode_params(&full_params.encode())), full_params);
    let full_event = ChainEvent::Funded {
        id: u32::MAX,
        from: [255; 20],
        amount: [255; 32],
    };
    let (topics, data) = full_event.encode();
    assert_eq!(must(decode_event(&topics, &data)), full_event);

    for length in [0, 31, 32, 351, chain_bytes.len() - 1] {
        assert!(decode_get_chain(&chain_bytes[..length]).is_err());
    }
    for position in [
        0,
        32 + 31,
        64,
        96 + 30,
        128,
        160,
        192,
        224 + 29,
        320,
        352,
        chain_bytes.len() - 1,
    ] {
        let mut invalid = chain_bytes.clone();
        invalid[position] = 255;
        assert!(
            decode_get_chain(&invalid).is_err(),
            "invalid chain byte {position}"
        );
    }
    let mut invalid_utf8 = chain_bytes.clone();
    invalid_utf8[384] = 255;
    assert!(decode_get_chain(&invalid_utf8).is_err());
    for position in [0, 96, 128, 160, 192] {
        let mut invalid = params_bytes.clone();
        invalid[position] = 255;
        assert!(decode_params(&invalid).is_err());
    }
    assert!(decode_params(&params_bytes[..params_bytes.len() - 1]).is_err());
    let mut invalid_bool = encode_call(&ChainCall::GovernancePause {
        id: 1,
        paused: true,
    });
    invalid_bool[67] = 2;
    assert!(decode_call(&invalid_bool).is_err());
    let mut invalid_id = encode_call(&ChainCall::GetChain(1));
    invalid_id[4 + 27] = 1;
    assert!(decode_call(&invalid_id).is_err());
    assert!(decode_call(&[0; 4]).is_err());
}
