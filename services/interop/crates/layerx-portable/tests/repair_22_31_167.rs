//! Task 22.31.167: portable_json_budget is proved by portable_json_budget.
//! Baseline verification was not run: the bounded gate order requires complete
//! implementation before the first build and verification. No wiring is needed.
//! The previous attempt failed before producing a receipt because the native
//! sequencer link dependencies were absent; the producer now builds them.

use std::io::{BufRead as _, BufReader, Write as _};
use std::process::{Child, ChildStdin, ChildStdout, Command, Stdio};

use layerx_portable::{
    verify_portable_json, PortableReceiptError, PortableReceiptV2, PortableVerified,
    ReceiptKeyEntry,
};
use serde_json::{json, Value};

const JSON_LIMIT: usize = 1_500_000;

// Use the node's receipt builder and persisted-key signer, rather than encoding
// or re-signing a fixture in this test. Each requested raw transaction is built
// and signed by go-ethereum, then bound by the real sequencer producer.
const PRODUCER: &str = r#"
package main

import (
    "bytes"
    "crypto/ed25519"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "io"
    "math/big"
    "os"
    "path/filepath"

    "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
    "github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
    "github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
    evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
    seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
    "github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
    "github.com/ethereum/go-ethereum/common"
    ethtypes "github.com/ethereum/go-ethereum/core/types"
    "github.com/ethereum/go-ethereum/crypto"
)

func must(err error) { if err != nil { panic(err) } }
func hx(b []byte) string { return "0x" + hex.EncodeToString(b) }

func main() {
    dir, err := os.MkdirTemp("", "portable-budget-key-"); must(err)
    defer os.RemoveAll(dir)
    seed := sha256.Sum256([]byte("portable budget receipt signing test key"))
    private := ed25519.NewKeyFromSeed(seed[:])
    public := [32]byte(private.Public().(ed25519.PublicKey))
    keyID := sha256.Sum256([]byte("portable budget receipt test key id"))
    pin := verify.SequencerAuthorization{SequencerID: keyID, PublicKey: public, FirstBatchNumber: 1, LastBatchNumber: 100}
    seedPath, consensusPath := filepath.Join(dir, "receipt.key"), filepath.Join(dir, "consensus.json")
    must(os.WriteFile(seedPath, []byte(hex.EncodeToString(seed[:])), 0600))
    pv, err := privval.GenFilePV(consensusPath, filepath.Join(dir, "state.json"), ""); must(err)
    must(pv.Key.Save())
    key, err := sequencer.LoadReceiptKey(seedPath, consensusPath, []verify.SequencerAuthorization{pin}, 1); must(err)
    txKey, err := crypto.HexToECDSA("0101010101010101010101010101010101010101010101010101010101010101"); must(err)
    dec, enc := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
    for {
        var req struct { Logs int; Data int }
        err := dec.Decode(&req)
        if err == io.EOF { return }; must(err)
        to := common.HexToAddress("0x1111111111111111111111111111111111111111")
        tx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
            To: &to, Gas: 30_000_000, GasPrice: big.NewInt(1), Value: big.NewInt(0),
            Data: bytes.Repeat([]byte{0x42}, req.Data),
        }), ethtypes.NewEIP155Signer(big.NewInt(125)), txKey); must(err)
        raw, err := tx.MarshalBinary(); must(err)
        activity := [32]byte(tx.Hash())
        logs := make([]*evmtypes.Log, req.Logs)
        for i := range logs {
            logs[i] = &evmtypes.Log{Address: to.Hex(), Topics: []string{
                common.HexToHash("0x01").Hex(), common.HexToHash("0x02").Hex(),
                common.HexToHash("0x03").Hex(), common.HexToHash("0x04").Hex(),
            }, Data: []byte{0x42}}
        }
        in := &sequencer.ReceiptInput{
            Accepted: &sequencer.Accepted{Seq: 1, ActivityID: activity,
                SeqHash: seqtypes.SeqHash([32]byte{}, 1, activity), Height: 1, TxIndex: 1, TimestampMs: 1000},
            EVM: &evmtypes.Receipt{Status: 1, To: to.Hex(), Logs: logs},
            PaymentDenom: "apax", FeeDenom: "apax", ModuleID: 12,
            FeeCharged: big.NewInt(0), Amount: big.NewInt(0),
            FromBalanceBefore: big.NewInt(0), FromBalanceAfter: big.NewInt(0),
            ToBalanceBefore: big.NewInt(0), ToBalanceAfter: big.NewInt(0),
        }
        unsigned, err := sequencer.BuildReceipt(in, keyID); must(err)
        signed, err := key.SignReceipt(in); must(err)
        digest := codec.ReceiptDigest(unsigned)
        if !ed25519.Verify(public[:], digest[:], signed[len(signed)-64:]) { panic("producer signature") }
        must(enc.Encode(map[string]any{"receipt": hx(signed), "raw": hx(raw), "digest": hx(digest[:]),
            "key": map[string]any{"keyId": hx(keyID[:]), "publicKey": hx(public[:]), "firstHeight": "0x1", "lastHeight": "0x64"}}))
    }
}
"#;

struct Producer {
    child: Child,
    input: ChildStdin,
    output: BufReader<ChildStdout>,
    directory: std::path::PathBuf,
}

impl Producer {
    fn new() -> Self {
        let directory =
            std::env::temp_dir().join(format!("portable-budget-{}", std::process::id()));
        std::fs::create_dir(&directory).expect("producer directory");
        let source = directory.join("main.go");
        std::fs::write(&source, PRODUCER).expect("producer driver");
        let root = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../..");
        let native = Command::new("make")
            .args([
                "-s",
                "-j2",
                "build/liblayerx.a",
                "programs/target/release/liblayerx_programs_sandbox.a",
            ])
            .current_dir(&root)
            .status()
            .expect("build real sequencer native dependencies");
        assert!(native.success(), "native sequencer dependencies: {native}");
        let mut child = Command::new("go")
            .arg("run")
            .arg(source)
            .env("CGO_ENABLED", "1")
            .current_dir(root)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::inherit())
            .spawn()
            .expect("start real sequencer producer");
        let input = child.stdin.take().expect("producer stdin");
        let output = BufReader::new(child.stdout.take().expect("producer stdout"));
        Self {
            child,
            input,
            output,
            directory,
        }
    }

    fn receipt(&mut self, logs: usize, data: usize) -> Value {
        writeln!(self.input, "{}", json!({"Logs": logs, "Data": data})).expect("producer request");
        self.input.flush().expect("flush producer request");
        let mut line = String::new();
        assert_ne!(
            self.output.read_line(&mut line).expect("producer response"),
            0,
            "real sequencer producer stopped: {:?}",
            self.child.try_wait()
        );
        serde_json::from_str(&line).expect("producer output JSON")
    }
}

impl Drop for Producer {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
        std::fs::remove_dir_all(&self.directory).expect("remove producer driver");
    }
}

fn bytes(value: &Value) -> Vec<u8> {
    let hex = value
        .as_str()
        .expect("hex string")
        .strip_prefix("0x")
        .expect("hex prefix");
    (0..hex.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&hex[i..i + 2], 16).expect("hex byte"))
        .collect()
}

fn key(case: &Value) -> ReceiptKeyEntry {
    ReceiptKeyEntry::new(
        bytes(&case["key"]["keyId"]).try_into().expect("key id"),
        bytes(&case["key"]["publicKey"])
            .try_into()
            .expect("public key"),
        1,
        100,
    )
}

fn document(case: &Value, height: u64, raw: bool) -> Value {
    let mut proof = json!({"format": "layerx-receipt-proof-v2", "verificationLevel": "receipted",
        "signedReceipt": case["receipt"], "receiptDigest": case["digest"],
        "receiptKey": case["key"], "inclusionHeight": format!("{height:#x}")});
    if raw {
        proof["rawTransaction"] = case["raw"].clone();
    }
    proof
}

fn assert_round_trip(case: &Value, height: u64, raw: bool) {
    let receipt = bytes(&case["receipt"]);
    let transaction = bytes(&case["raw"]);
    let pin = key(case);
    let portable = if raw {
        PortableReceiptV2::export_with_transaction(&receipt, &pin, height, &transaction)
    } else {
        PortableReceiptV2::export(&receipt, &pin, height)
    }
    .expect("bounded producer output exports");
    let wire = portable.to_json().expect("bounded serialization");
    assert!(wire.len() <= JSON_LIMIT);
    assert_eq!(
        PortableReceiptV2::from_json(&wire).expect("import emitted bytes"),
        portable
    );
    let PortableVerified::V2(verified) =
        verify_portable_json(&wire, &[pin]).expect("consumer verifies");
    assert_eq!(verified.signed_receipt(), receipt);
    assert_eq!(verified.receipt_digest().as_slice(), bytes(&case["digest"]));
    assert_eq!(
        verified.raw_transaction(),
        if raw {
            Some(transaction.as_slice())
        } else {
            None
        }
    );
    assert_eq!(verified.inclusion_height(), height);

    let mut python = Command::new(std::env::var_os("LAYERX_TEST_PYTHON").unwrap_or_else(|| "python3".into()))
        .args(["-c", "import json,sys; sys.path.insert(0,sys.argv[1]); from python_portable_verifier import verify; v=json.load(sys.stdin); print(json.dumps(verify(v['proof'], v['pins'])))"])
        .arg(concat!(env!("CARGO_MANIFEST_DIR"), "/tests"))
        .env("PYTHONDONTWRITEBYTECODE", "1")
        .stdin(Stdio::piped()).stdout(Stdio::piped()).stderr(Stdio::piped())
        .spawn().expect("independent verifier");
    let proof: Value = serde_json::from_slice(&wire).expect("emitted document");
    let input_result = python.stdin.take().expect("verifier stdin").write_all(
        &serde_json::to_vec(&json!({"proof": proof, "pins": [case["key"]]}))
            .expect("verifier input"),
    );
    let output = python.wait_with_output().expect("independent verification");
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    input_result.expect("write independent verifier input");
    let result: Value = serde_json::from_slice(&output.stdout).expect("independent digest");
    assert_eq!(result[0], case["digest"]);
    assert_eq!(bytes(&result[1]).as_slice(), verified.activity_id());
}

fn assert_oversize(case: &Value, height: u64, raw: bool) {
    let proof = document(case, height, raw);
    let wire = serde_json::to_vec(&proof).expect("independent size observation");
    assert!(wire.len() > JSON_LIMIT);
    assert!(bytes(&case["receipt"]).len() <= 1_048_576);
    assert!(bytes(&case["raw"]).len() <= 1_048_576);
    let bypass: PortableReceiptV2 = serde_json::from_value(proof).expect("valid individual fields");
    assert!(
        bypass.verify_v2(&[key(case)]).is_ok(),
        "valid signed proof before wire budget"
    );
    assert_eq!(bypass.to_json(), Err(PortableReceiptError::JsonBounds));
    assert_eq!(
        PortableReceiptV2::from_json(&wire),
        Err(PortableReceiptError::JsonBounds)
    );
    let receipt = bytes(&case["receipt"]);
    let exported = if raw {
        PortableReceiptV2::export_with_transaction(
            &receipt,
            &key(case),
            height,
            &bytes(&case["raw"]),
        )
    } else {
        PortableReceiptV2::export(&receipt, &key(case), height)
    };
    assert_eq!(exported, Err(PortableReceiptError::JsonBounds));
}

#[test]
fn portable_json_budget() {
    let mut producer = Producer::new();
    let small = producer.receipt(0, 0);
    let next = producer.receipt(1, 0);
    let base = serde_json::to_vec(&document(&small, 15, false))
        .expect("base size")
        .len();
    let effect_bytes = bytes(&next["receipt"]).len() - bytes(&small["receipt"]).len();
    assert_eq!(effect_bytes, 229, "real four-topic log effect size");
    let logs = (JSON_LIMIT - base) / (2 * effect_bytes);
    assert!(logs > 3000, "exercise expanded sequencer effect layout");
    let below = producer.receipt(logs, 0);
    assert_round_trip(&below, 15, false);
    let above = producer.receipt(logs + 1, 0);
    assert_oversize(&above, 15, false);

    let mut data = 100_000;
    let mut combined = producer.receipt(2000, data);
    // Hex payloads advance in two-byte wire steps. The inclusion quantity's
    // extra digit selects the parity needed to reach the exact importer limit.
    let size = serde_json::to_vec(&document(&combined, 15, true))
        .expect("combined size")
        .len();
    assert!(size < JSON_LIMIT);
    data += (JSON_LIMIT - size) / 2;
    combined = producer.receipt(2000, data);
    let size = serde_json::to_vec(&document(&combined, 15, true))
        .expect("boundary size")
        .len();
    assert!(size == JSON_LIMIT || size == JSON_LIMIT - 1);
    let height = if size == JSON_LIMIT { 15 } else { 16 };
    assert_eq!(
        serde_json::to_vec(&document(&combined, height, true))
            .expect("exact boundary")
            .len(),
        JSON_LIMIT
    );
    assert_round_trip(&combined, height, true);
    let before = producer.receipt(2000, data - 1);
    assert_round_trip(&before, height, true);
    let after = producer.receipt(2000, data + 1);
    assert_oversize(&after, height, true);
}
