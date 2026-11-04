use super::RecordedSubmit;
use sha2::{Digest, Sha256};
const POSITIVE: &str =
    include_str!("../../../../daemon/committed-fixture/vectors/vector-v1-receipted-positive.json");
pub(super) fn positive() -> Vec<RecordedSubmit> {
    assert_eq!(
        super::hex(&Sha256::digest(POSITIVE.as_bytes())),
        "d74c1e130b5f9e3687655a4f8a1c048e0ee77a479931a87d790510f4419a22e2",
        "producer vector changed: regenerate receipt spans"
    );
    vec![
RecordedSubmit {
name: r#"vector-v1-receipted-positive"#.to_owned(),
expected: r#"receipted"#.to_owned(),
raw: r#"0x02f873830ae3f201808403fe56c083030d4080809d6001600052601160206000a16002600052602260206000a160006000f3c080a08ae3225c0849a6983307fe8e5ada22f9c23adbbbeebad7cb26ef8743d374fcd7a0680793c6962ac2074b20913538845d0931d1729d872410e5142ee39c2499f880"#.to_owned(),
hash: r#"0x3e13485af55ff4f49c494aa634ca8b23b55a3bab09436d42fcc338cbaa8797ac"#.to_owned(),
keys: r#"{"keys":[{"keyId":"0x08dec1015669bc470709bdf2136fe3adb1552ca3c871b58e827cbaf092a2b09e","publicKey":"0x85a0b7f2c67621d3def86305a632affdf31fa6a051961deb869ae05e623a2e78","firstHeight":1,"lastHeight":9223372036854775807}]}"#.to_owned(),
receipt: POSITIVE[35948..39560].to_owned(),
},
]
}
const REVERT: &str =
    include_str!("../../../../daemon/committed-fixture/vectors/vector-v2-receipted-revert.json");
pub(super) fn revert() -> Vec<RecordedSubmit> {
    assert_eq!(
        super::hex(&Sha256::digest(REVERT.as_bytes())),
        "64fa4664780380bd6da0e36cd2444fa41e751e9fb0919514a4cb2a6f73feea0a",
        "producer vector changed: regenerate receipt spans"
    );
    vec![
RecordedSubmit {
name: r#"vector-v2-receipted-revert"#.to_owned(),
expected: r#"receipted"#.to_owned(),
raw: r#"0x02f901307d05843b9aca0084b2d05e0083015f9094000000000000000000000000000000000000101a80b8c478ea11db1fb0082aa4aa4ce1eb67568a7d8f67798f3b008c254e0929dfe5eabb1dc0d769112b929c0d6cb01c57eaad307e6d0a094c6c6e58f2e0ce3c62856cf815c0fba44e366c1f82bbcb0cfc80b6168346024ca11db454d4b5e93dc7cac57bc5292f280000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000000000000000000001977457ca8035e88ee49bb79cbb71380ed443a54c661f071a0809eca9dbaada5de031db59f1c080a00cf1abc283fa9ff61853934fba0892a53bacd9c2e91159c55738d874df8037a9a036c4e9b916d3b94d9538e6d196c1ae7ab56750a105e26eb1a1997837f8990b43"#.to_owned(),
hash: r#"0xd9ea67d1ca9968723dbc84262f2fcf28bd44512b5035a92acb9f417f1c8e1b8e"#.to_owned(),
keys: r#"{"keys":[{"keyId":"0x6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b","publicKey":"0x8146640f02493af4fbc54fe33388e75dc2c937ae0b7727cc2b2afb1b75199a3e","firstHeight":1000,"lastHeight":1999}]}"#.to_owned(),
receipt: REVERT[1338..4093].to_owned(),
},
]
}
const REFUSALS: &str =
    include_str!("../../../../daemon/committed-fixture/vectors/vector-v3-receipt-refusals.json");
pub(super) fn refusals() -> Vec<RecordedSubmit> {
    assert_eq!(
        super::hex(&Sha256::digest(REFUSALS.as_bytes())),
        "480ed0e242f16cd5064ba8a7a613ef24e96e9119d9327da3ffd67563ca12df5e",
        "producer vector changed: regenerate receipt spans"
    );
    vec![
RecordedSubmit {
name: r#"key-id-not-pinned"#.to_owned(),
expected: r#"sequencer: receipt key not pinned at height"#.to_owned(),
raw: r#"0x02f8af7d01843b9aca0084b2d05e0082ea60945fbdb2315678afecb367f032d93f642f64180aa380b844095ea7b300000000000000000000000070997970c51812dc3a010c7d01b50e0d17dc79c800000000000000000000000000000000000000000000000d8d726b7177a80000c080a0283470ae2480d3ae1039166110076d51b29f5116f2b0a5fb55d921298de2a390a01d54f5763b853430566827fc806ded4e96beb930b060bfeae1183dcaeabf5a83"#.to_owned(),
hash: r#"0xf36621d835bfc801ee639486d202b26733e6d138606dd0143cb07a498e2508db"#.to_owned(),
keys: r#"{"keys":[{"keyId":"0x812ef3644dd1518f0112c38fade6c0b504baf7c3caff5179d52b249dc0ae750a","publicKey":"0x9596a19cb083a777b2fc083ef2fc8c8df7076c7d13de25dbca52a57f36297b2f","firstHeight":1000,"lastHeight":1999}]}"#.to_owned(),
receipt: REFUSALS[4540..8083].to_owned(),
},
RecordedSubmit {
name: r#"height-outside-range"#.to_owned(),
expected: r#"sequencer: receipt key not pinned at height"#.to_owned(),
raw: r#"0x02f8af7d01843b9aca0084b2d05e0082ea60945fbdb2315678afecb367f032d93f642f64180aa380b844095ea7b300000000000000000000000070997970c51812dc3a010c7d01b50e0d17dc79c800000000000000000000000000000000000000000000000d8d726b7177a80000c080a0283470ae2480d3ae1039166110076d51b29f5116f2b0a5fb55d921298de2a390a01d54f5763b853430566827fc806ded4e96beb930b060bfeae1183dcaeabf5a83"#.to_owned(),
hash: r#"0xf36621d835bfc801ee639486d202b26733e6d138606dd0143cb07a498e2508db"#.to_owned(),
keys: r#"{"keys":[{"keyId":"0x812ef3644dd1518f0112c38fade6c0b504baf7c3caff5179d52b249dc0ae750a","publicKey":"0x9596a19cb083a777b2fc083ef2fc8c8df7076c7d13de25dbca52a57f36297b2f","firstHeight":1000,"lastHeight":1999}]}"#.to_owned(),
receipt: REFUSALS[8221..11764].to_owned(),
},
RecordedSubmit {
name: r#"activity-id-not-raw-hash"#.to_owned(),
expected: r#"activity_id is not keccak256 of the raw transaction"#.to_owned(),
raw: r#"0x02f8af7d01843b9aca0084b2d05e0082ea60945fbdb2315678afecb367f032d93f642f64180aa380b844095ea7b300000000000000000000000070997970c51812dc3a010c7d01b50e0d17dc79c800000000000000000000000000000000000000000000000d8d726b7177a80000c080a0283470ae2480d3ae1039166110076d51b29f5116f2b0a5fb55d921298de2a390a01d54f5763b853430566827fc806ded4e96beb930b060bfeae1183dcaeabf5a83"#.to_owned(),
hash: r#"0xf36621d835bfc801ee639486d202b26733e6d138606dd0143cb07a498e2508db"#.to_owned(),
keys: r#"{"keys":[{"keyId":"0x812ef3644dd1518f0112c38fade6c0b504baf7c3caff5179d52b249dc0ae750a","publicKey":"0x9596a19cb083a777b2fc083ef2fc8c8df7076c7d13de25dbca52a57f36297b2f","firstHeight":1000,"lastHeight":1999}]}"#.to_owned(),
receipt: REFUSALS[11914..15457].to_owned(),
},
RecordedSubmit {
name: r#"sequence-hash-mismatch"#.to_owned(),
expected: r#"sequence hash mismatch"#.to_owned(),
raw: r#"0x02f8af7d01843b9aca0084b2d05e0082ea60945fbdb2315678afecb367f032d93f642f64180aa380b844095ea7b300000000000000000000000070997970c51812dc3a010c7d01b50e0d17dc79c800000000000000000000000000000000000000000000000d8d726b7177a80000c080a0283470ae2480d3ae1039166110076d51b29f5116f2b0a5fb55d921298de2a390a01d54f5763b853430566827fc806ded4e96beb930b060bfeae1183dcaeabf5a83"#.to_owned(),
hash: r#"0xf36621d835bfc801ee639486d202b26733e6d138606dd0143cb07a498e2508db"#.to_owned(),
keys: r#"{"keys":[{"keyId":"0x812ef3644dd1518f0112c38fade6c0b504baf7c3caff5179d52b249dc0ae750a","publicKey":"0x9596a19cb083a777b2fc083ef2fc8c8df7076c7d13de25dbca52a57f36297b2f","firstHeight":1000,"lastHeight":1999}]}"#.to_owned(),
receipt: REFUSALS[15576..19119].to_owned(),
},
RecordedSubmit {
name: r#"tampered-log"#.to_owned(),
expected: r#"sequencer receipt: JSON view differs from receipt bytes"#.to_owned(),
raw: r#"0x02f8af7d01843b9aca0084b2d05e0082ea60945fbdb2315678afecb367f032d93f642f64180aa380b844095ea7b300000000000000000000000070997970c51812dc3a010c7d01b50e0d17dc79c800000000000000000000000000000000000000000000000d8d726b7177a80000c080a0283470ae2480d3ae1039166110076d51b29f5116f2b0a5fb55d921298de2a390a01d54f5763b853430566827fc806ded4e96beb930b060bfeae1183dcaeabf5a83"#.to_owned(),
hash: r#"0xf36621d835bfc801ee639486d202b26733e6d138606dd0143cb07a498e2508db"#.to_owned(),
keys: r#"{"keys":[{"keyId":"0x812ef3644dd1518f0112c38fade6c0b504baf7c3caff5179d52b249dc0ae750a","publicKey":"0x9596a19cb083a777b2fc083ef2fc8c8df7076c7d13de25dbca52a57f36297b2f","firstHeight":1000,"lastHeight":1999}]}"#.to_owned(),
receipt: REFUSALS[19261..22804].to_owned(),
},
RecordedSubmit {
name: r#"non-canonical-receipt-bytes"#.to_owned(),
expected: r#"layerx codec: non-canonical"#.to_owned(),
raw: r#"0x02f8af7d01843b9aca0084b2d05e0082ea60945fbdb2315678afecb367f032d93f642f64180aa380b844095ea7b300000000000000000000000070997970c51812dc3a010c7d01b50e0d17dc79c800000000000000000000000000000000000000000000000d8d726b7177a80000c080a0283470ae2480d3ae1039166110076d51b29f5116f2b0a5fb55d921298de2a390a01d54f5763b853430566827fc806ded4e96beb930b060bfeae1183dcaeabf5a83"#.to_owned(),
hash: r#"0xf36621d835bfc801ee639486d202b26733e6d138606dd0143cb07a498e2508db"#.to_owned(),
keys: r#"{"keys":[{"keyId":"0x812ef3644dd1518f0112c38fade6c0b504baf7c3caff5179d52b249dc0ae750a","publicKey":"0x9596a19cb083a777b2fc083ef2fc8c8df7076c7d13de25dbca52a57f36297b2f","firstHeight":1000,"lastHeight":1999}]}"#.to_owned(),
receipt: REFUSALS[22933..26476].to_owned(),
},
]
}
pub(super) fn refusal_control() -> Vec<RecordedSubmit> {
    assert_eq!(
        super::hex(&Sha256::digest(REFUSALS.as_bytes())),
        "480ed0e242f16cd5064ba8a7a613ef24e96e9119d9327da3ffd67563ca12df5e",
        "producer vector changed: regenerate receipt spans"
    );
    vec![
RecordedSubmit {
name: r#"v3-unmodified-control"#.to_owned(),
expected: r#"receipted"#.to_owned(),
raw: r#"0x02f8af7d01843b9aca0084b2d05e0082ea60945fbdb2315678afecb367f032d93f642f64180aa380b844095ea7b300000000000000000000000070997970c51812dc3a010c7d01b50e0d17dc79c800000000000000000000000000000000000000000000000d8d726b7177a80000c080a0283470ae2480d3ae1039166110076d51b29f5116f2b0a5fb55d921298de2a390a01d54f5763b853430566827fc806ded4e96beb930b060bfeae1183dcaeabf5a83"#.to_owned(),
hash: r#"0xf36621d835bfc801ee639486d202b26733e6d138606dd0143cb07a498e2508db"#.to_owned(),
keys: r#"{"keys":[{"keyId":"0x812ef3644dd1518f0112c38fade6c0b504baf7c3caff5179d52b249dc0ae750a","publicKey":"0x9596a19cb083a777b2fc083ef2fc8c8df7076c7d13de25dbca52a57f36297b2f","firstHeight":1000,"lastHeight":1999}]}"#.to_owned(),
receipt: REFUSALS[999..4398].to_owned(),
},
]
}
