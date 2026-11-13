// Task 22.31.168: ucp_currency_units -> ucp_currency_units_native_checkout and ucp_currency_units_checked_conversion.
// Wiring request (domain 11): services/interop/crates/layerx-interop-service/src/{config,server}.rs must load merchant currency/asset/exponents, wrap UcpServicePlane with config.bind_plane, and pass that configuration to read_order_with_config; migrate the old unconfigured UCP test callers.
// Baseline was not run: the task's gate order forbids verification before implementation.
// Inherited qualification: checked_conversion passed; native_checkout failed to link the producer's native libraries.
// The bounded runner records the verified revision and private evidence location.

use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::time::{Duration, Instant};

use ed25519_dalek::SigningKey;
use layerx_interop_gateway::adapter::{AdapterId, ConformanceSuite, PinnedSpec, SpecVersion};
use layerx_interop_gateway::evidence::{SequencerEvidence, SettlementRung};
use layerx_interop_gateway::gateway::TranslationStatus;
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::trace::TraceId;
use layerx_interop_gateway::GatewayCore;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::sequencer_logs::SequencerLog;
use layerx_ucp::{
    ucp_adapter_descriptor, verify_sequencer_receipt_with_config, CheckoutStatus,
    CheckoutSubmission, ExecutedUcpPayment, MerchantCurrencyBinding, MerchantProfile,
    NegotiatedCapabilities, OrderMetadata, PaymentHandler, PlatformProfile, UcpAdapter, UcpError,
    UcpIdempotencyKey, UcpPaymentIntent, UcpPaymentPlane, UcpPlaneResult, UcpServiceConfig,
    UCP_CHECKOUT_SPEC_SHA256,
};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

fn submission(asset: [u8; 32], recipient: [u8; 32]) -> CheckoutSubmission {
    let handler = PaymentHandler::new(
        "layerx-402",
        "2026-04-08",
        "https://layerx.dev/2026-04-08/specification/402",
        "https://layerx.dev/2026-04-08/schemas/402.json",
    )
    .or_panic("expected Ok value");
    let merchant = MerchantProfile::layerx("https://merchant.example/ucp", handler.clone())
        .or_panic("expected Ok value");
    let platform = PlatformProfile {
        profile_url: "https://buyer.example/profile".into(),
        capabilities: merchant.capabilities().to_vec(),
        payment_handlers: vec![handler.clone()],
    };
    CheckoutSubmission {
        checkout_id: "currency-units".into(),
        currency: *b"USD",
        total_minor: 1000,
        layerx_asset: asset,
        layerx_recipient: recipient,
        idempotency_key: UcpIdempotencyKey::parse("123e4567-e89b-12d3-a456-426614174000")
            .or_panic("expected Ok value"),
        negotiated: NegotiatedCapabilities::negotiate(&platform, &handler)
            .or_panic("expected Ok value"),
    }
}

fn config(asset: [u8; 32], minor: u8, atomic: u8) -> UcpServiceConfig {
    UcpServiceConfig::new(
        MerchantCurrencyBinding::new(*b"USD", asset, minor, atomic).or_panic("expected Ok value"),
    )
}

fn gateway(trace: &TraceId) -> GatewayCore {
    let mut gateway = GatewayCore::new();
    let spec = PinnedSpec::new(
        AdapterId::new("ucp").or_panic("expected Ok value"),
        SpecVersion::parse("20260408").or_panic("expected Ok value"),
        UCP_CHECKOUT_SPEC_SHA256,
    )
    .or_panic("expected Ok value");
    let suite = ConformanceSuite::new(
        AdapterId::new("ucp-currency-units").or_panic("expected Ok value"),
        3,
        [0x68; 32],
    )
    .or_panic("expected Ok value");
    gateway
        .register_adapter(
            ucp_adapter_descriptor(spec, suite).or_panic("expected Ok value"),
            trace,
            1,
        )
        .or_panic("expected Ok value");
    gateway
}

#[test]
fn ucp_currency_units_checked_conversion() {
    let mut checkout = submission([1; 32], [2; 32]);
    assert_eq!(
        config([1; 32], 2, 6).atomic_amount(&checkout),
        Ok(10_000_000)
    );
    assert_eq!(config([1; 32], 2, 2).atomic_amount(&checkout), Ok(1000));
    assert_eq!(config([1; 32], 2, 0).atomic_amount(&checkout), Ok(10));
    checkout.total_minor = 1001;
    assert_eq!(
        config([1; 32], 2, 0).atomic_amount(&checkout),
        Err(UcpError::InexactAmount)
    );
    checkout.total_minor = 1;
    assert_eq!(
        config([1; 32], 0, 38).atomic_amount(&checkout),
        Ok(10_u128.pow(38))
    );
    checkout.total_minor = u128::MAX;
    assert_eq!(
        config([1; 32], 2, 6).atomic_amount(&checkout),
        Err(UcpError::AmountOverflow)
    );
    assert_eq!(
        config([1; 32], 2, 2).atomic_amount(&checkout),
        Ok(u128::MAX)
    );
    checkout.total_minor = 0;
    assert_eq!(
        config([1; 32], 2, 6).atomic_amount(&checkout),
        Err(UcpError::InvalidCheckout)
    );
    checkout.total_minor = 1000;
    assert_eq!(
        UcpServiceConfig::default().atomic_amount(&checkout),
        Err(UcpError::CurrencyBindingRequired)
    );
    checkout.currency = *b"EUR";
    assert_eq!(
        config([1; 32], 2, 6).atomic_amount(&checkout),
        Err(UcpError::CurrencyBindingMismatch)
    );
    checkout.currency = *b"USD";
    checkout.layerx_asset = [3; 32];
    assert_eq!(
        config([1; 32], 2, 6).atomic_amount(&checkout),
        Err(UcpError::CurrencyBindingMismatch)
    );
    for (currency, asset, minor, atomic) in [
        (*b"usd", [1; 32], 2, 6),
        (*b"USD", [0; 32], 2, 6),
        (*b"USD", [1; 32], 39, 6),
        (*b"USD", [1; 32], 2, 39),
    ] {
        assert_eq!(
            MerchantCurrencyBinding::new(currency, asset, minor, atomic),
            Err(UcpError::InvalidCurrencyBinding)
        );
    }
}

fn bytes(value: &Value) -> Vec<u8> {
    let encoded = value
        .as_str()
        .or_panic("expected Some value")
        .strip_prefix("0x")
        .or_panic("expected Some value");
    assert_eq!(encoded.len() % 2, 0);
    (0..encoded.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&encoded[i..i + 2], 16).or_panic("expected Ok value"))
        .collect()
}

fn array<const N: usize>(value: &Value) -> [u8; N] {
    bytes(value).try_into().or_panic("expected Ok value")
}

fn hex(value: &[u8]) -> String {
    use std::fmt::Write as _;
    let mut out = String::from("0x");
    for byte in value {
        write!(out, "{byte:02x}").or_panic("expected Ok value");
    }
    out
}

fn payment(value: &Value) -> ExecutedUcpPayment {
    ExecutedUcpPayment {
        metadata: OrderMetadata {
            order_id: "native-currency-order".into(),
            permalink_url: "https://merchant.example/orders/native-currency-order".into(),
        },
        evidence: SequencerEvidence {
            transaction_hash: array(&value["txHash"]),
            signed_receipt: bytes(&value["receipt"]),
            logs: value["logs"]
                .as_array()
                .or_panic("expected Some value")
                .iter()
                .enumerate()
                .map(|(i, log)| {
                    SequencerLog::new(
                        array(&log["address"]),
                        log["topics"]
                            .as_array()
                            .or_panic("expected Some value")
                            .iter()
                            .map(array)
                            .collect(),
                        bytes(&log["data"]),
                        i as u64,
                    )
                })
                .collect(),
            rung: SettlementRung::Receipted,
        },
    }
}

// This boundary drives the native application and Receipter, not a replacement payment engine.
struct NativeCheckoutPlane {
    child: Child,
    dir: PathBuf,
    underpaid_reference: [u8; 32],
    produced: Option<Value>,
}

impl Drop for NativeCheckoutPlane {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
        let _ = fs::remove_dir_all(&self.dir);
    }
}

impl NativeCheckoutPlane {
    fn start() -> (Self, Value) {
        let root = Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("../../../..")
            .canonicalize()
            .or_panic("expected Ok value");
        let dir = std::env::temp_dir().join(format!("ucp-currency-units-{}", std::process::id()));
        fs::create_dir(&dir).or_panic("expected Ok value");
        let source = dir.join("producer_test.go");
        fs::write(&source, NATIVE_PRODUCER).or_panic("expected Ok value");
        let overlay = dir.join("overlay.json");
        fs::write(
            &overlay,
            serde_json::to_vec(&json!({"Replace": {
                (root.join("node/sequencer/repair_22_31_168_test.go").to_str().or_panic("expected Some value")): source
            }}))
            .or_panic("expected Ok value"),
        )
        .or_panic("expected Ok value");
        let log = fs::File::create(dir.join("producer.log")).or_panic("expected Ok value");
        let kernel_dir = dir.join("kernel");
        let programs_dir = dir.join("programs");
        let mut kernel = Command::new("make");
        kernel
            .arg("-s")
            .arg(format!("BUILD_DIR={}", kernel_dir.display()))
            .arg(kernel_dir.join("liblayerx.a"));
        let mut programs = Command::new("cargo");
        programs
            .args([
                "build",
                "--locked",
                "--manifest-path",
                "programs/Cargo.toml",
                "--package",
                "layerx-programs-sandbox",
                "--features",
                "layerx-programs-sandbox/host-ffi",
                "--target-dir",
            ])
            .arg(&programs_dir);
        for mut dependency in [kernel, programs] {
            let status = dependency
                .current_dir(&root)
                .stdin(Stdio::null())
                .stdout(log.try_clone().or_panic("expected Ok value"))
                .stderr(log.try_clone().or_panic("expected Ok value"))
                .status()
                .or_panic("expected Ok value");
            assert!(
                status.success(),
                "native producer dependency failed: {dependency:?}: {status}\n{}",
                fs::read_to_string(dir.join("producer.log")).or_panic("expected Ok value")
            );
        }
        let native_link_flags = format!(
            "{} -L{} -L{}",
            std::env::var("CGO_LDFLAGS").unwrap_or_default(),
            kernel_dir.display(),
            programs_dir.join("debug").display()
        );
        let child = Command::new("go")
            .current_dir(&root)
            .args(["test", "-count=1", "-timeout=3m", "-overlay"])
            .arg(&overlay)
            .args([
                "./node/sequencer",
                "-run",
                "^TestRepairUcpCurrencyProducer$",
            ])
            .env("CGO_ENABLED", "1")
            .env("CGO_LDFLAGS", native_link_flags)
            .env("UCP_REPAIR_DIR", &dir)
            .stdin(Stdio::null())
            .stdout(log.try_clone().or_panic("expected Ok value"))
            .stderr(log)
            .spawn()
            .or_panic("expected Ok value");
        let mut plane = Self {
            child,
            dir,
            underpaid_reference: [0; 32],
            produced: None,
        };
        let ready = plane.wait_json("ready.json", Duration::from_secs(1200));
        (plane, ready)
    }

    fn wait_json(&mut self, name: &str, limit: Duration) -> Value {
        let started = Instant::now();
        loop {
            if let Ok(data) = fs::read(self.dir.join(name)) {
                return serde_json::from_slice(&data).or_panic("expected Ok value");
            }
            let exited = self.child.try_wait().or_panic("expected Ok value");
            assert!(
                exited.is_none() && started.elapsed() < limit,
                "native producer did not emit {name}: {exited:?}\n{}",
                fs::read_to_string(self.dir.join("producer.log")).or_panic("expected Ok value")
            );
            std::thread::sleep(Duration::from_millis(50));
        }
    }
}

impl UcpPaymentPlane for NativeCheckoutPlane {
    fn execute(
        &mut self,
        intent: &UcpPaymentIntent,
        _trace: &TraceId,
    ) -> Result<UcpPlaneResult, UcpError> {
        assert_eq!(intent.currency, *b"USD");
        assert_eq!(
            intent.amount, 10_000_000,
            "intent must contain asset atomic units"
        );
        fs::write(
            self.dir.join("intent.tmp"),
            serde_json::to_vec(&json!({
                "asset": hex(&intent.asset), "recipient": hex(&intent.recipient),
                "reference": hex(&intent.idempotency_key), "amount": intent.amount.to_string(),
                "underpaidReference": hex(&self.underpaid_reference)
            }))
            .or_panic("expected Ok value"),
        )
        .or_panic("expected Ok value");
        fs::rename(self.dir.join("intent.tmp"), self.dir.join("intent.json"))
            .or_panic("expected Ok value");
        let produced = self.wait_json("result.json", Duration::from_secs(120));
        let status = self.child.wait().or_panic("expected Ok value");
        assert!(
            status.success(),
            "native producer failed: {}",
            fs::read_to_string(self.dir.join("producer.log")).or_panic("expected Ok value")
        );
        assert_eq!(produced["recipientBalance"], "10001000");
        assert_eq!(produced["payerBalance"], "29999000");
        let executed = payment(&produced["correct"]);
        self.produced = Some(produced);
        Ok(UcpPlaneResult::Executed(Box::new(executed)))
    }
}

#[test]
fn ucp_currency_units_native_checkout() {
    let (mut native, ready) = NativeCheckoutPlane::start();
    let asset = array(&ready["asset"]);
    let checkout = submission(asset, array(&ready["recipient"]));
    let mut underpaid = checkout.clone();
    underpaid.checkout_id = "underpaid-currency-units".into();
    underpaid.idempotency_key = UcpIdempotencyKey::parse("123e4567-e89b-12d3-a456-426614174001")
        .or_panic("expected Ok value");
    native.underpaid_reference = underpaid.idempotency_key.gateway_key();
    let config = config(asset, 2, 6);
    let keys = receipt_keys();
    let trace = TraceId::mint([0x68; 16]);
    let principal = PrincipalId::new("merchant-currency-units").or_panic("expected Ok value");
    let mut gateway = gateway(&trace);
    let outcome = UcpAdapter::complete_checkout(
        &mut gateway,
        &principal,
        &checkout,
        &mut config.bind_plane(&mut native),
        &keys,
        &trace,
        2,
    )
    .or_panic("expected Ok value");
    let (order, stored) =
        assert_completed_order(outcome, &keys, &config, &gateway, &principal, &checkout);
    let produced = native.produced.as_ref().or_panic("expected Some value");
    let mut correct = payment(&produced["correct"]);
    let mut insufficient = payment(&produced["underpaid"]);
    assert_underpaid_receipt_refused(&underpaid, &insufficient, &keys, &config);
    let refusal = UcpAdapter::complete_checkout(
        &mut gateway,
        &principal,
        &underpaid,
        &mut config.bind_plane(&mut insufficient),
        &keys,
        &trace,
        3,
    )
    .err()
    .or_panic("expected Err value")
    .into_error();
    assert_eq!(refusal, UcpError::ReceiptMismatch);
    assert_ne!(
        gateway.translation(&principal, underpaid.idempotency_key.gateway_key()),
        Some(TranslationStatus::ReceiptVerified {
            receipt_digest: order.receipt_digest
        })
    );
    let replay = UcpAdapter::complete_checkout(
        &mut gateway,
        &principal,
        &checkout,
        &mut config.bind_plane(&mut correct),
        &keys,
        &trace,
        4,
    )
    .or_panic("expected Ok value");
    assert_eq!(replay.order, Some(order));
    let changed = self::config(asset, 2, 5);
    assert!(UcpAdapter::complete_checkout(
        &mut gateway,
        &principal,
        &checkout,
        &mut changed.bind_plane(&mut correct),
        &keys,
        &trace,
        5
    )
    .is_err());
    assert_eq!(
        UcpAdapter::read_order_with_config(&stored, &keys, &changed),
        Err(UcpError::ReceiptMismatch)
    );
    assert_eq!(
        UcpAdapter::complete_checkout(
            &mut gateway,
            &principal,
            &checkout,
            &mut correct,
            &keys,
            &trace,
            6
        )
        .err()
        .or_panic("expected Err value")
        .into_error(),
        UcpError::CurrencyBindingRequired
    );
    let mut corrupted = stored;
    corrupted.signed_receipt[0] ^= 1;
    assert_eq!(
        UcpAdapter::read_order_with_config(&corrupted, &keys, &config),
        Err(UcpError::ReceiptMismatch)
    );
}

fn assert_underpaid_receipt_refused(
    underpaid: &CheckoutSubmission,
    insufficient: &ExecutedUcpPayment,
    keys: &[SequencerAuthorization],
    config: &UcpServiceConfig,
) {
    assert_eq!(
        verify_sequencer_receipt_with_config(
            underpaid,
            &insufficient.evidence.signed_receipt,
            keys,
            config
        ),
        Err(UcpError::ReceiptMismatch)
    );
}

fn receipt_keys() -> [SequencerAuthorization; 1] {
    let seed: [u8; 32] = Sha256::digest(b"sequencer receipt test key").into();
    [SequencerAuthorization::new(
        Sha256::digest(b"sequencer receipt test key id").into(),
        SigningKey::from_bytes(&seed).verifying_key().to_bytes(),
        1,
        1000,
    )]
}

fn assert_completed_order(
    outcome: layerx_ucp::CheckoutOutcome,
    keys: &[SequencerAuthorization],
    config: &UcpServiceConfig,
    gateway: &GatewayCore,
    principal: &PrincipalId,
    checkout: &CheckoutSubmission,
) -> (layerx_ucp::UcpOrder, layerx_ucp::StoredOrder) {
    assert_eq!(outcome.status, CheckoutStatus::Completed);
    let order = outcome.order.or_panic("expected Some value");
    assert_eq!(order.currency, *b"USD");
    assert_eq!(order.total_minor, 1000);
    let stored = outcome.stored_order.or_panic("expected Some value");
    assert_eq!(
        UcpAdapter::read_order_with_config(&stored, keys, config),
        Ok(order.clone())
    );
    assert_eq!(
        UcpAdapter::read_order(&stored, keys),
        Err(UcpError::CurrencyBindingRequired)
    );
    assert_eq!(
        gateway.translation(principal, checkout.idempotency_key.gateway_key()),
        Some(TranslationStatus::ReceiptVerified {
            receipt_digest: order.receipt_digest
        })
    );
    (order, stored)
}

const NATIVE_PRODUCER: &str = r#"
package sequencer_test

import (
    "crypto/ed25519"
    "crypto/sha256"
    "encoding/json"
    "math/big"
    "os"
    "path/filepath"
    "testing"
    "time"

    evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
    ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
    ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
    app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
    "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
    sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
    "github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/common/hexutil"
    "github.com/stretchr/testify/require"
)

func TestRepairUcpCurrencyProducer(t *testing.T) {
    dir := os.Getenv("UCP_REPAIR_DIR")
    require.NotEmpty(t, dir)
    publish := func(name string, value any) {
        encoded, err := json.Marshal(value)
        require.NoError(t, err)
        require.NoError(t, os.WriteFile(filepath.Join(dir, name+".tmp"), encoded, 0600))
        require.NoError(t, os.Rename(filepath.Join(dir, name+".tmp"), filepath.Join(dir, name)))
    }
    sender := utils.NewSigner()
    recipientCaller := common.HexToAddress("0x2320000000000000000000000000000000000002")
    var asset, from, to [32]byte
    rh := newPrecompileHarness(t, func(ctx sdk.Context, a *app.App) {
        a.UpgradeKeeper.SetDone(ctx, app.MergeUpgrade)
        bind := func(caller common.Address, tag byte) ledgerkeeper.Actor {
            var seed [32]byte
            seed[0] = tag
            private := ed25519.NewKeyFromSeed(seed[:])
            var key [32]byte
            copy(key[:], private.Public().(ed25519.PublicKey))
            message, err := evmtypes.LayerXBindMessage(a.EvmKeeper.ChainID(ctx), caller, 0)
            require.NoError(t, err)
            var signature [64]byte
            copy(signature[:], ed25519.Sign(private, message))
            _, err = a.EvmKeeper.BindLayerX(ctx, caller, key, signature)
            require.NoError(t, err)
            actor, err := a.LayerXLedgerKeeper.Actor(ctx, caller)
            require.NoError(t, err)
            return actor
        }
        payer := bind(sender.EvmAddress, 1)
        payee := bind(recipientCaller, 2)
        salt := [32]byte{0x41}
        material := append(append([]byte("LX:ASSET:v1"), payer.Authority.Actor[:]...), salt[:]...)
        asset = sha256.Sum256(material)
        _, err := a.LayerXLedgerKeeper.RegisterAsset(ctx, payer, ledgertypes.RegisterArgs{
            AssetID: asset, Salt: salt, Symbol: "USD", Name: "Merchant dollar", Decimals: 6,
            SupplyCap: sdk.NewInt(100000000), IssuerKind: ledgertypes.AssetIssuerNative,
        }, [32]byte{1})
        require.NoError(t, err)
        from, err = ledgerkeeper.AccountID(payer.DIDKey, asset)
        require.NoError(t, err)
        to, err = ledgerkeeper.AccountID(payee.DIDKey, asset)
        require.NoError(t, err)
        _, err = a.LayerXLedgerKeeper.OpenAccount(ctx, payer, asset, [32]byte{2})
        require.NoError(t, err)
        _, err = a.LayerXLedgerKeeper.OpenAccount(ctx, payee, asset, [32]byte{3})
        require.NoError(t, err)
        _, err = a.LayerXLedgerKeeper.Mint(ctx, payer, asset, from, sdk.NewInt(40000000), [32]byte{4})
        require.NoError(t, err)
        a.RefreshPrecompiles(ctx)
    }, sender)
    defer rh.app.Close()
    require.Contains(t, rh.app.EvmKeeper.CustomPrecompiles(rh.openCtx()), common.HexToAddress(layerxledger.LedgerAddress))
    publish("ready.json", map[string]any{"asset": common.Hash(asset).Hex(), "recipient": common.Hash(to).Hex()})
    var input struct { Asset, Recipient, Reference, Amount, UnderpaidReference string }
    deadline := time.Now().Add(time.Minute)
    for {
        data, err := os.ReadFile(filepath.Join(dir, "intent.json"))
        if err == nil {
            require.NoError(t, json.Unmarshal(data, &input))
            break
        }
        require.True(t, time.Now().Before(deadline), "checkout did not send its intent")
        time.Sleep(10*time.Millisecond)
    }
    require.Equal(t, common.Hash(asset).Hex(), input.Asset)
    require.Equal(t, common.Hash(to).Hex(), input.Recipient)
    require.Equal(t, "10000000", input.Amount)
    amount, ok := new(big.Int).SetString(input.Amount, 10)
    require.True(t, ok)
    require.True(t, amount.IsInt64())
    ledger := layerxledger.GetABI()
    address := common.HexToAddress(layerxledger.LedgerAddress)
    execute := func(nonce uint64, amount int64, reference string) map[string]any {
        data, err := ledger.Pack("payExact", from, to, asset, big.NewInt(amount), uint64(rh.openMs)+60000, [32]byte(common.HexToHash(reference)))
        require.NoError(t, err)
        tx, wire := evmTx(t, sender, dynamicTx(nonce, &address, big.NewInt(0), 1000000, data))
        accepted, receipt, signed := rh.accept(t, wire)
        require.Zero(t, receipt.ResultCode)
        transient := rh.transient(t, tx, accepted)
        require.Equal(t, uint32(1), transient.Status)
        require.NotEmpty(t, transient.Logs)
        logs := make([]map[string]any, len(transient.Logs))
        for i, log := range transient.Logs {
            logs[i] = map[string]any{"address": log.Address, "topics": log.Topics, "data": hexutil.Encode(log.Data)}
        }
        return map[string]any{"txHash": tx.Hash().Hex(), "receipt": hexutil.Encode(signed), "logs": logs}
    }
    correct := execute(0, amount.Int64(), input.Reference)
    underpaid := execute(1, 1000, input.UnderpaidReference)
    recipientBalance, err := rh.app.LayerXLedgerKeeper.BalanceOf(rh.openCtx(), to, asset)
    require.NoError(t, err)
    payerBalance, err := rh.app.LayerXLedgerKeeper.BalanceOf(rh.openCtx(), from, asset)
    require.NoError(t, err)
    require.Equal(t, "10001000", recipientBalance.String())
    require.Equal(t, "29999000", payerBalance.String())
    publish("result.json", map[string]any{"correct": correct, "underpaid": underpaid,
        "recipientBalance": recipientBalance.String(), "payerBalance": payerBalance.String()})
}
"#;

trait OrPanic {
    type Output;

    fn or_panic(self, context: &str) -> Self::Output;
}

impl<T, E: std::fmt::Debug> OrPanic for std::result::Result<T, E> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|error| panic!("{context}: {error:?}"))
    }
}

impl<T> OrPanic for std::option::Option<T> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|| panic!("{context}"))
    }
}
