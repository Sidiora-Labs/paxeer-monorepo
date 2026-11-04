use serde::de::DeserializeOwned;
use serde_json::json;
use verification_common::verifier_alliance::{
    verify_creation_code, verify_runtime_code, CompilationArtifacts, CreationCodeArtifacts, Match,
    MatchTransformation, MatchValues, Offset, RuntimeCodeArtifacts,
};

const LIBRARY_ID: &str = "contracts/1_Storage.sol:Journal";
const IMMUTABLE_ID: &str = "7";

fn parse_code(code: &str) -> Vec<u8> {
    blockscout_display_bytes::decode_hex(code).expect("recorded compiler bytecode must decode")
}

fn parse_artifacts<T: DeserializeOwned>(value: serde_json::Value) -> T {
    serde_json::from_slice(&serde_json::to_vec(&value).unwrap()).unwrap()
}

fn transport<T: serde::Serialize + DeserializeOwned>(artifacts: &T) -> T {
    serde_json::from_slice(&serde_json::to_vec(artifacts).unwrap()).unwrap()
}

// Compiler outputs and deployed bytecode recorded in integration/verifier_alliance_matches.rs.
fn library_runtime_fixture() -> (Vec<u8>, Vec<u8>, RuntimeCodeArtifacts) {
    let on_chain_runtime_code = "0x608060405234801561001057600080fd5b50600436106100415760003560e01c80636057361d146100465780638381f58a14610062578063e2e2a85a14610080575b600080fd5b610060600480360381019061005b919061017d565b6100b0565b005b61006a610124565b60405161007791906101b9565b60405180910390f35b61009a6004803603810190610095919061017d565b61012a565b6040516100a791906101b9565b60405180910390f35b80600081905550737d53f102f4d4aa014db4e10d6deec2009b3cda6b632be59dd56001836040518363ffffffff1660e01b81526004016100f19291906101ea565b60006040518083038186803b15801561010957600080fd5b505af415801561011d573d6000803e3d6000fd5b5050505050565b60005481565b60016020528060005260406000206000915090505481565b600080fd5b6000819050919050565b61015a81610147565b811461016557600080fd5b50565b60008135905061017781610151565b92915050565b60006020828403121561019357610192610142565b5b60006101a184828501610168565b91505092915050565b6101b381610147565b82525050565b60006020820190506101ce60008301846101aa565b92915050565b8082525050565b6101e481610147565b82525050565b60006040820190506101ff60008301856101d4565b61020c60208301846101db565b939250505056fea26469706673582212205d40d1517b560d915e1b7c005887b93fcce9ec65c0a38a80ee147739551bdd7264736f6c63430008120033";
    let re_compiled_runtime_code = "0x608060405234801561001057600080fd5b50600436106100415760003560e01c80636057361d146100465780638381f58a14610062578063e2e2a85a14610080575b600080fd5b610060600480360381019061005b919061017d565b6100b0565b005b61006a610124565b60405161007791906101b9565b60405180910390f35b61009a6004803603810190610095919061017d565b61012a565b6040516100a791906101b9565b60405180910390f35b80600081905550730000000000000000000000000000000000000000632be59dd56001836040518363ffffffff1660e01b81526004016100f19291906101ea565b60006040518083038186803b15801561010957600080fd5b505af415801561011d573d6000803e3d6000fd5b5050505050565b60005481565b60016020528060005260406000206000915090505481565b600080fd5b6000819050919050565b61015a81610147565b811461016557600080fd5b50565b60008135905061017781610151565b92915050565b60006020828403121561019357610192610142565b5b60006101a184828501610168565b91505092915050565b6101b381610147565b82525050565b60006020820190506101ce60008301846101aa565b92915050565b8082525050565b6101e481610147565b82525050565b60006040820190506101ff60008301856101d4565b61020c60208301846101db565b939250505056fea26469706673582212205d40d1517b560d915e1b7c005887b93fcce9ec65c0a38a80ee147739551bdd7264736f6c63430008120033";
    let runtime_code_artifacts = json!({"immutableReferences":{},"linkReferences":{"contracts/1_Storage.sol":{"Journal":[{"length":20,"start":185}]}},"sourceMap":"292:289:0:-:0;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;477:102;;;;;;;;;;;;;:::i;:::-;;:::i;:::-;;315:21;;;:::i;:::-;;;;;;;:::i;:::-;;;;;;;;343:42;;;;;;;;;;;;;:::i;:::-;;:::i;:::-;;;;;;;:::i;:::-;;;;;;;;477:102;531:3;522:6;:12;;;;544:7;:14;559:7;568:3;544:28;;;;;;;;;;;;;;;;:::i;:::-;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;477:102;:::o;315:21::-;;;;:::o;343:42::-;;;;;;;;;;;;;;;;;:::o;88:117:1:-;197:1;194;187:12;334:77;371:7;400:5;389:16;;334:77;;;:::o;417:122::-;490:24;508:5;490:24;:::i;:::-;483:5;480:35;470:63;;529:1;526;519:12;470:63;417:122;:::o;545:139::-;591:5;629:6;616:20;607:29;;645:33;672:5;645:33;:::i;:::-;545:139;;;;:::o;690:329::-;749:6;798:2;786:9;777:7;773:23;769:32;766:119;;;804:79;;:::i;:::-;766:119;924:1;949:53;994:7;985:6;974:9;970:22;949:53;:::i;:::-;939:63;;895:117;690:329;;;;:::o;1025:118::-;1112:24;1130:5;1112:24;:::i;:::-;1107:3;1100:37;1025:118;;:::o;1149:222::-;1242:4;1280:2;1269:9;1265:18;1257:26;;1293:71;1361:1;1350:9;1346:17;1337:6;1293:71;:::i;:::-;1149:222;;;;:::o;1377:132::-;1497:5;1492:3;1485:18;1377:132;;:::o;1515:126::-;1610:24;1628:5;1610:24;:::i;:::-;1605:3;1598:37;1515:126;;:::o;1647:406::-;1801:4;1839:2;1828:9;1824:18;1816:26;;1852:104;1953:1;1942:9;1938:17;1929:6;1852:104;:::i;:::-;1966:80;2042:2;2031:9;2027:18;2018:6;1966:80;:::i;:::-;1647:406;;;;;:::o","cborAuxdata":{"1":{"offset":532,"value":"0xa26469706673582212205d40d1517b560d915e1b7c005887b93fcce9ec65c0a38a80ee147739551bdd7264736f6c63430008120033"}}});
    (
        parse_code(on_chain_runtime_code),
        parse_code(re_compiled_runtime_code),
        parse_artifacts(runtime_code_artifacts),
    )
}

fn library_creation_fixture() -> (
    Vec<u8>,
    Vec<u8>,
    CreationCodeArtifacts,
    CompilationArtifacts,
) {
    let on_chain_creation_code = "0x608060405234801561001057600080fd5b50610249806100206000396000f3fe608060405234801561001057600080fd5b50600436106100415760003560e01c80636057361d146100465780638381f58a14610062578063e2e2a85a14610080575b600080fd5b610060600480360381019061005b919061017d565b6100b0565b005b61006a610124565b60405161007791906101b9565b60405180910390f35b61009a6004803603810190610095919061017d565b61012a565b6040516100a791906101b9565b60405180910390f35b80600081905550737d53f102f4d4aa014db4e10d6deec2009b3cda6b632be59dd56001836040518363ffffffff1660e01b81526004016100f19291906101ea565b60006040518083038186803b15801561010957600080fd5b505af415801561011d573d6000803e3d6000fd5b5050505050565b60005481565b60016020528060005260406000206000915090505481565b600080fd5b6000819050919050565b61015a81610147565b811461016557600080fd5b50565b60008135905061017781610151565b92915050565b60006020828403121561019357610192610142565b5b60006101a184828501610168565b91505092915050565b6101b381610147565b82525050565b60006020820190506101ce60008301846101aa565b92915050565b8082525050565b6101e481610147565b82525050565b60006040820190506101ff60008301856101d4565b61020c60208301846101db565b939250505056fea26469706673582212205d40d1517b560d915e1b7c005887b93fcce9ec65c0a38a80ee147739551bdd7264736f6c63430008120033";
    let re_compiled_creation_code = "0x608060405234801561001057600080fd5b50610249806100206000396000f3fe608060405234801561001057600080fd5b50600436106100415760003560e01c80636057361d146100465780638381f58a14610062578063e2e2a85a14610080575b600080fd5b610060600480360381019061005b919061017d565b6100b0565b005b61006a610124565b60405161007791906101b9565b60405180910390f35b61009a6004803603810190610095919061017d565b61012a565b6040516100a791906101b9565b60405180910390f35b80600081905550730000000000000000000000000000000000000000632be59dd56001836040518363ffffffff1660e01b81526004016100f19291906101ea565b60006040518083038186803b15801561010957600080fd5b505af415801561011d573d6000803e3d6000fd5b5050505050565b60005481565b60016020528060005260406000206000915090505481565b600080fd5b6000819050919050565b61015a81610147565b811461016557600080fd5b50565b60008135905061017781610151565b92915050565b60006020828403121561019357610192610142565b5b60006101a184828501610168565b91505092915050565b6101b381610147565b82525050565b60006020820190506101ce60008301846101aa565b92915050565b8082525050565b6101e481610147565b82525050565b60006040820190506101ff60008301856101d4565b61020c60208301846101db565b939250505056fea26469706673582212205d40d1517b560d915e1b7c005887b93fcce9ec65c0a38a80ee147739551bdd7264736f6c63430008120033";
    let creation_code_artifacts = json!({"linkReferences":{"contracts/1_Storage.sol":{"Journal":[{"length":20,"start":217}]}},"sourceMap":"292:289:0:-:0;;;;;;;;;;;;;;;;;;;","cborAuxdata":{"1":{"offset":564,"value":"0xa26469706673582212205d40d1517b560d915e1b7c005887b93fcce9ec65c0a38a80ee147739551bdd7264736f6c63430008120033"}}});
    let compilation_artifacts = json!({"abi":[{"inputs":[{"internalType":"uint256","name":"","type":"uint256"}],"name":"journal","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"number","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"num","type":"uint256"}],"name":"store","outputs":[],"stateMutability":"nonpayable","type":"function"}],"devdoc":{"details":"Store & retrieve value in a variable","kind":"dev","methods":{"store(uint256)":{"details":"Store value in variable","params":{"num":"value to store"}}},"title":"Storage","version":1},"userdoc":{"kind":"user","methods":{},"version":1},"storageLayout":{"storage":[{"astId":22,"contract":"contracts/1_Storage.sol:Storage","label":"number","offset":0,"slot":"0","type":"t_uint256"},{"astId":26,"contract":"contracts/1_Storage.sol:Storage","label":"journal","offset":0,"slot":"1","type":"t_mapping(t_uint256,t_uint256)"}],"types":{"t_mapping(t_uint256,t_uint256)":{"encoding":"mapping","key":"t_uint256","label":"mapping(uint256 => uint256)","numberOfBytes":"32","value":"t_uint256"},"t_uint256":{"encoding":"inplace","label":"uint256","numberOfBytes":"32"}}},"sources":{"contracts/1_Storage.sol":{"id":0}}});
    (
        parse_code(on_chain_creation_code),
        parse_code(re_compiled_creation_code),
        parse_artifacts(creation_code_artifacts),
        parse_artifacts(compilation_artifacts),
    )
}

fn immutable_runtime_fixture() -> (Vec<u8>, Vec<u8>, RuntimeCodeArtifacts) {
    let on_chain_runtime_code = "0x608060405234801561001057600080fd5b50600436106100415760003560e01c80636057361d146100465780638381f58a146100625780639fe44c4a14610080575b600080fd5b610060600480360381019061005b919061010d565b61009e565b005b61006a6100a8565b6040516100779190610149565b60405180910390f35b6100886100ae565b6040516100959190610149565b60405180910390f35b8060008190555050565b60005481565b7f000000000000000000000000000000000000000000000000000000000000006481565b600080fd5b6000819050919050565b6100ea816100d7565b81146100f557600080fd5b50565b600081359050610107816100e1565b92915050565b600060208284031215610123576101226100d2565b5b6000610131848285016100f8565b91505092915050565b610143816100d7565b82525050565b600060208201905061015e600083018461013a565b9291505056fea26469706673582212205fff17b2676425e48225435ac15579ccae1af038ff8ffb334fc372526b94722664736f6c63430008120033";
    let re_compiled_runtime_code = "0x608060405234801561001057600080fd5b50600436106100415760003560e01c80636057361d146100465780638381f58a146100625780639fe44c4a14610080575b600080fd5b610060600480360381019061005b919061010d565b61009e565b005b61006a6100a8565b6040516100779190610149565b60405180910390f35b6100886100ae565b6040516100959190610149565b60405180910390f35b8060008190555050565b60005481565b7f000000000000000000000000000000000000000000000000000000000000000081565b600080fd5b6000819050919050565b6100ea816100d7565b81146100f557600080fd5b50565b600081359050610107816100e1565b92915050565b600060208284031215610123576101226100d2565b5b6000610131848285016100f8565b91505092915050565b610143816100d7565b82525050565b600060208201905061015e600083018461013a565b9291505056fea26469706673582212205fff17b2676425e48225435ac15579ccae1af038ff8ffb334fc372526b94722664736f6c63430008120033";
    let runtime_code_artifacts = json!({"immutableReferences":{"7":[{"length":32,"start":176}]},"linkReferences":{},"sourceMap":"141:250:0:-:0;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;;325:64;;;;;;;;;;;;;:::i;:::-;;:::i;:::-;;164:21;;;:::i;:::-;;;;;;;:::i;:::-;;;;;;;;192:41;;;:::i;:::-;;;;;;;:::i;:::-;;;;;;;;325:64;379:3;370:6;:12;;;;325:64;:::o;164:21::-;;;;:::o;192:41::-;;;:::o;88:117:1:-;197:1;194;187:12;334:77;371:7;400:5;389:16;;334:77;;;:::o;417:122::-;490:24;508:5;490:24;:::i;:::-;483:5;480:35;470:63;;529:1;526;519:12;470:63;417:122;:::o;545:139::-;591:5;629:6;616:20;607:29;;645:33;672:5;645:33;:::i;:::-;545:139;;;;:::o;690:329::-;749:6;798:2;786:9;777:7;773:23;769:32;766:119;;;804:79;;:::i;:::-;766:119;924:1;949:53;994:7;985:6;974:9;970:22;949:53;:::i;:::-;939:63;;895:117;690:329;;;;:::o;1025:118::-;1112:24;1130:5;1112:24;:::i;:::-;1107:3;1100:37;1025:118;;:::o;1149:222::-;1242:4;1280:2;1269:9;1265:18;1257:26;;1293:71;1361:1;1350:9;1346:17;1337:6;1293:71;:::i;:::-;1149:222;;;;:::o","cborAuxdata":{"1":{"offset":357,"value":"0xa26469706673582212205fff17b2676425e48225435ac15579ccae1af038ff8ffb334fc372526b94722664736f6c63430008120033"}}});
    (
        parse_code(on_chain_runtime_code),
        parse_code(re_compiled_runtime_code),
        parse_artifacts(runtime_code_artifacts),
    )
}

fn library_offsets(artifacts: &mut RuntimeCodeArtifacts) -> &mut Vec<Offset> {
    artifacts
        .link_references
        .as_mut()
        .unwrap()
        .get_mut("contracts/1_Storage.sol")
        .unwrap()
        .get_mut("Journal")
        .unwrap()
}

fn creation_library_offsets(artifacts: &mut CreationCodeArtifacts) -> &mut Vec<Offset> {
    artifacts
        .link_references
        .as_mut()
        .unwrap()
        .get_mut("contracts/1_Storage.sol")
        .unwrap()
        .get_mut("Journal")
        .unwrap()
}

fn immutable_offsets(artifacts: &mut RuntimeCodeArtifacts) -> &mut Vec<Offset> {
    artifacts
        .immutable_references
        .as_mut()
        .unwrap()
        .get_mut(IMMUTABLE_ID)
        .unwrap()
}

fn invalid_offsets(code_len: usize) -> Vec<Offset> {
    let code_len = u32::try_from(code_len).unwrap();
    vec![
        Offset {
            start: code_len + 1,
            length: 0,
        },
        Offset {
            start: code_len,
            length: 1,
        },
        Offset {
            start: code_len - 1,
            length: 2,
        },
        Offset {
            start: 0,
            length: u32::MAX,
        },
        Offset {
            start: u32::MAX,
            length: 0,
        },
        Offset {
            start: u32::MAX,
            length: 1,
        },
        Offset {
            start: u32::MAX,
            length: u32::MAX,
        },
        Offset {
            start: 1,
            length: u32::MAX,
        },
    ]
}

fn assert_range_error(error: anyhow::Error, reason: &str, id: &str, offset: &Offset) {
    let message = error.to_string();
    for context in [
        format!("reason={reason}"),
        format!("id={id}"),
        format!("start={}", offset.start),
        format!("length={}", offset.length),
    ] {
        assert!(message.contains(&context), "missing {context}: {message}");
    }
    assert!(
        message.contains("out of range") || message.contains("overflow"),
        "{message}"
    );
}

#[test]
fn artifact_offset_valid_library_creation_and_runtime() {
    let address = parse_code("0x7d53f102f4d4aa014db4e10d6deec2009b3cda6b");
    let mut values = MatchValues::default();
    values.add_library(LIBRARY_ID, address);

    let (deployed, compiled, artifacts) = library_runtime_fixture();
    let runtime_match = verify_runtime_code(&deployed, compiled, &transport(&artifacts)).unwrap();
    assert_eq!(
        runtime_match,
        Some(Match {
            metadata_match: true,
            transformations: vec![MatchTransformation::library(185, LIBRARY_ID)],
            values: values.clone(),
        })
    );

    let (deployed, compiled, artifacts, compilation) = library_creation_fixture();
    let creation_match = verify_creation_code(
        &deployed,
        compiled,
        &transport(&artifacts),
        &transport(&compilation),
    )
    .unwrap();
    assert_eq!(
        creation_match,
        Some(Match {
            metadata_match: true,
            transformations: vec![MatchTransformation::library(217, LIBRARY_ID)],
            values,
        })
    );
}

#[test]
fn artifact_offset_valid_immutable_runtime() {
    let (deployed, compiled, artifacts) = immutable_runtime_fixture();
    let mut values = MatchValues::default();
    values.add_immutable(
        IMMUTABLE_ID,
        parse_code("0x0000000000000000000000000000000000000000000000000000000000000064"),
    );
    let result = verify_runtime_code(&deployed, compiled, &transport(&artifacts)).unwrap();
    assert_eq!(
        result,
        Some(Match {
            metadata_match: true,
            transformations: vec![MatchTransformation::immutable(176, IMMUTABLE_ID)],
            values,
        })
    );
}

#[test]
fn artifact_offset_invalid_library_ranges_return_contextual_errors() {
    let (deployed, compiled, artifacts) = library_runtime_fixture();
    let original_deployed = deployed.clone();
    for offset in invalid_offsets(deployed.len()) {
        let mut invalid = artifacts.clone();
        library_offsets(&mut invalid).push(offset.clone());
        let error = verify_runtime_code(&deployed, compiled.clone(), &transport(&invalid))
            .expect_err("malformed library reference must return an error");
        assert_range_error(error, "link_reference", LIBRARY_ID, &offset);
        assert_eq!(deployed, original_deployed);
    }

    let (deployed, compiled, artifacts, compilation) = library_creation_fixture();
    let original_deployed = deployed.clone();
    for offset in invalid_offsets(deployed.len()) {
        let mut invalid = artifacts.clone();
        creation_library_offsets(&mut invalid).push(offset.clone());
        let error = verify_creation_code(
            &deployed,
            compiled.clone(),
            &transport(&invalid),
            &transport(&compilation),
        )
        .expect_err("malformed creation library reference must return an error");
        assert_range_error(error, "link_reference", LIBRARY_ID, &offset);
        assert_eq!(deployed, original_deployed);
    }
}

#[test]
fn artifact_offset_invalid_immutable_ranges_return_contextual_errors() {
    let (deployed, compiled, artifacts) = immutable_runtime_fixture();
    let original_deployed = deployed.clone();
    for offset in invalid_offsets(deployed.len()) {
        let mut invalid = artifacts.clone();
        immutable_offsets(&mut invalid).push(offset.clone());
        let error = verify_runtime_code(&deployed, compiled.clone(), &transport(&invalid))
            .expect_err("malformed immutable reference must return an error");
        assert_range_error(error, "immutable_reference", IMMUTABLE_ID, &offset);
        assert_eq!(deployed, original_deployed);
    }
}

#[test]
fn artifact_offset_library_range_must_fit_compiled_code() {
    let (deployed, mut compiled, mut artifacts) = library_runtime_fixture();
    let offset = library_offsets(&mut artifacts)[0].clone();
    compiled.truncate((offset.start + offset.length - 1) as usize);
    artifacts.cbor_auxdata = None;
    assert!((offset.start + offset.length) as usize <= deployed.len());
    let error = verify_runtime_code(&deployed, compiled, &transport(&artifacts))
        .expect_err("a range fitting deployed code must also fit compiled code");
    assert_range_error(error, "link_reference", LIBRARY_ID, &offset);

    let (deployed, mut compiled, mut artifacts, compilation) = library_creation_fixture();
    let offset = creation_library_offsets(&mut artifacts)[0].clone();
    compiled.truncate((offset.start + offset.length - 1) as usize);
    artifacts.cbor_auxdata = None;
    assert!((offset.start + offset.length) as usize <= deployed.len());
    let error = verify_creation_code(
        &deployed,
        compiled,
        &transport(&artifacts),
        &transport(&compilation),
    )
    .expect_err("a creation range fitting deployed code must also fit compiled code");
    assert_range_error(error, "link_reference", LIBRARY_ID, &offset);
}

#[test]
fn artifact_offset_immutable_range_must_fit_compiled_code() {
    let (deployed, mut compiled, mut artifacts) = immutable_runtime_fixture();
    let offset = immutable_offsets(&mut artifacts)[0].clone();
    compiled.truncate((offset.start + offset.length - 1) as usize);
    artifacts.cbor_auxdata = None;
    assert!((offset.start + offset.length) as usize <= deployed.len());
    let error = verify_runtime_code(&deployed, compiled, &transport(&artifacts))
        .expect_err("an immutable range fitting deployed code must also fit compiled code");
    assert_range_error(error, "immutable_reference", IMMUTABLE_ID, &offset);
}

#[test]
fn artifact_offset_exact_end_and_empty_boundary_ranges_are_valid() {
    for (mut deployed, mut compiled, mut artifacts, immutable) in [
        {
            let (d, c, a) = library_runtime_fixture();
            (d, c, a, false)
        },
        {
            let (d, c, a) = immutable_runtime_fixture();
            (d, c, a, true)
        },
    ] {
        let offset = if immutable {
            immutable_offsets(&mut artifacts)
        } else {
            library_offsets(&mut artifacts)
        }[0]
        .clone();
        let end = (offset.start + offset.length) as usize;
        deployed.truncate(end);
        compiled.truncate(end);
        artifacts.cbor_auxdata = None;
        let matched = verify_runtime_code(&deployed, compiled.clone(), &transport(&artifacts))
            .unwrap()
            .expect("a reference ending exactly at both buffer lengths is valid");
        assert_eq!(
            matched.transformations,
            vec![if immutable {
                MatchTransformation::immutable(offset.start as usize, IMMUTABLE_ID)
            } else {
                MatchTransformation::library(offset.start as usize, LIBRARY_ID)
            }]
        );

        let offsets = if immutable {
            immutable_offsets(&mut artifacts)
        } else {
            library_offsets(&mut artifacts)
        };
        *offsets = vec![Offset {
            start: u32::try_from(end).unwrap(),
            length: 0,
        }];
        let matched = verify_runtime_code(&deployed, deployed.clone(), &transport(&artifacts))
            .unwrap()
            .expect("an empty reference at the boundary is valid");
        assert_eq!(
            matched.transformations,
            vec![if immutable {
                MatchTransformation::immutable(end, IMMUTABLE_ID)
            } else {
                MatchTransformation::library(end, LIBRARY_ID)
            }]
        );
    }
}

#[test]
fn artifact_offset_repeated_reference_consistency_is_preserved() {
    for (deployed, compiled, mut artifacts, immutable) in [
        {
            let (d, c, a) = library_runtime_fixture();
            (d, c, a, false)
        },
        {
            let (d, c, a) = immutable_runtime_fixture();
            (d, c, a, true)
        },
    ] {
        let offsets = if immutable {
            immutable_offsets(&mut artifacts)
        } else {
            library_offsets(&mut artifacts)
        };
        let first = offsets[0].clone();
        offsets.push(first.clone());
        let matched = verify_runtime_code(&deployed, compiled.clone(), &transport(&artifacts))
            .unwrap()
            .expect("repeated references to the same value are valid");
        assert_eq!(matched.transformations.len(), 2);
        assert_eq!(matched.transformations[0], matched.transformations[1]);

        let offsets = if immutable {
            immutable_offsets(&mut artifacts)
        } else {
            library_offsets(&mut artifacts)
        };
        offsets[1] = Offset {
            start: 0,
            length: first.length,
        };
        let error = verify_runtime_code(&deployed, compiled, &transport(&artifacts))
            .expect_err("different values for one reference must still be rejected");
        let id = if immutable { IMMUTABLE_ID } else { LIBRARY_ID };
        let reason = if immutable {
            "immutable_reference"
        } else {
            "link_reference"
        };
        assert_eq!(
            error.to_string(),
            format!("(reason={reason}; id={id}) offset values are not consistent")
        );
    }
}

#[test]
fn artifact_offset_unrelated_bytecode_mismatch_is_preserved() {
    for (mut deployed, compiled, artifacts) in
        [library_runtime_fixture(), immutable_runtime_fixture()]
    {
        deployed[0] ^= 0xff;
        let original = deployed.clone();
        assert_eq!(
            verify_runtime_code(&deployed, compiled, &transport(&artifacts)).unwrap(),
            None
        );
        assert_eq!(deployed, original);
    }
}
