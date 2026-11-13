# Interop gas station contract

The [gas station](../../../services/interop/crates/layerx-gas-station/src/lib.rs) validates sponsorship requests, applies budget policy and tracks funded transactions through authenticated receipts.

## cfg-gas-gas-limit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-gas-limit</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>gas station service config gas_limit: u64 per sponsored submission</code> |

## cfg-gas-listen

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-listen</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>gas station service config listen: socket address</code> |

## cfg-gas-max-priority-fee-per-gas

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-max-priority-fee-per-gas</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>gas station service config max_priority_fee_per_gas: u128 wei</code> |

## cfg-gas-rate-balance-floor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-rate-balance-floor</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>gas station rate publisher config rate_balance_floor: u128 wei</code> |

## cfg-gas-rate-cadence-seconds

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-rate-cadence-seconds</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>gas station rate publisher config rate_cadence_seconds: u64</code> |

## cfg-gas-rate-confirmation-retry-seconds

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-rate-confirmation-retry-seconds</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>gas station rate publisher config rate_confirmation_retry_seconds: u64</code> |

## cfg-gas-rate-gas-budget-per-day

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-rate-gas-budget-per-day</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>gas station rate publisher config rate_gas_budget_per_day: u128 wei</code> |

## cfg-gas-rate-max-fee-per-gas

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-rate-max-fee-per-gas</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>gas station rate publisher config rate_max_fee_per_gas: u128 wei</code> |

## cfg-gas-rate-owner-key-env

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-rate-owner-key-env</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>gas station rate publisher config rate_owner_key_env: string env var name</code> |

## cfg-gas-registered-senders

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-registered-senders</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>gas station service config registered_senders: optional array of {address, budget}</code> |

## cfg-gas-sequencer-receipt-keys

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-sequencer-receipt-keys</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>gas station rate publisher config sequencer_receipt_keys: array of receipt key entries (key id, ed25519 key, height window)</code> |

## cfg-gas-station-balance-floor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-station-balance-floor</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>gas station config balance_floor: u128 wei (required; unknown fields refused)</code> |

## cfg-gas-station-chain-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-station-chain-id</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>gas station config chain_id: u64 EVM chain id (required; unknown fields refused)</code> |

## cfg-gas-station-decimals

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-station-decimals</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>gas station config decimals: u8 token decimals (required; unknown fields refused)</code> |

## cfg-gas-station-endpoints

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-station-endpoints</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>gas station config endpoints: array of RPC endpoint URLs (required; unknown fields refused)</code> |

## cfg-gas-station-interval-seconds

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-station-interval-seconds</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>gas station config interval_seconds: u64 (required; unknown fields refused)</code> |

## cfg-gas-station-margin-bps

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-station-margin-bps</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>gas station config margin_bps: u32 basis points (required; unknown fields refused)</code> |

## cfg-gas-station-max-rate-age

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-station-max-rate-age</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>gas station config max_rate_age: u64 seconds (required; unknown fields refused)</code> |

## cfg-gas-station-paymaster

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-station-paymaster</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>gas station config paymaster: 0x address of the paymaster (required; unknown fields refused)</code> |

## cfg-gas-station-per-account-limit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-station-per-account-limit</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>gas station config per_account_limit: u128 base units (required; unknown fields refused)</code> |

## cfg-gas-station-per-interval-limit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-station-per-interval-limit</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>gas station config per_interval_limit: u128 base units (required; unknown fields refused)</code> |

## cfg-gas-station-per-quote-limit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-station-per-quote-limit</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>gas station config per_quote_limit: u128 base units (required; unknown fields refused)</code> |

## cfg-gas-station-relayer-key-env

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-station-relayer-key-env</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>gas station config relayer_key_env: string env var name of the relayer key (required; unknown fields refused)</code> |

## cfg-gas-station-spread-bps

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-station-spread-bps</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>gas station config spread_bps: u32 basis points (required; unknown fields refused)</code> |

## cfg-gas-station-token

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-gas-station-token</code> | <code>config_key</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>gas station config token: 0x address of the fee token (required; unknown fields refused)</code> |

## cli-gas

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cli-gas</code> | <code>cli_command</code> | [services/interop/crates/layerx-gas-station/src/main.rs](../../../services/interop/crates/layerx-gas-station/src/main.rs) | <code>paxeer-gas-station --config PATH --journal PATH &#124; check --config PATH --rate-config PATH &#124; rate --config PATH --journal PATH --rate-file PATH</code> |

## env-gas-gas-station-browser-origins

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-gas-gas-station-browser-origins</code> | <code>env_var</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>GAS_STATION_BROWSER_ORIGINS</code> |

## env-gas-gas-station-did-gas-limit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-gas-gas-station-did-gas-limit</code> | <code>env_var</code> | [services/interop/crates/layerx-gas-station/src/station.rs](../../../services/interop/crates/layerx-gas-station/src/station.rs) | <code>GAS_STATION_DID_GAS_LIMIT</code> |

| Field | Rule |
|---|---|
| interface | The gas station funds DID budgets with EVM transactions and confirms them from pax_sequencerReceipt. |
| storage | Budget ledger in the station state directory. |
| errors | Station errors for exhausted budget, transport and unverified receipt. |
| events | none |
| gas | Each funding transaction pays the network gas price. |
| activation | Starts on service launch. |
| invariants | A budget is debited once per funded transaction. |
| pause | none |

## env-gas-gas-station-paymaster

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-gas-gas-station-paymaster</code> | <code>env_var</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>GAS_STATION_PAYMASTER</code> |

## env-gas-rate-owner-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-gas-rate-owner-key</code> | <code>env_var</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>&lt;rate_owner_key_env&gt;: rate publisher owner key read from the variable named by config rate_owner_key_env</code> |

## env-gas-relayer-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.env-gas-relayer-key</code> | <code>env_var</code> | [services/interop/crates/layerx-gas-station/src/signer.rs](../../../services/interop/crates/layerx-gas-station/src/signer.rs) | <code>&lt;relayer_key_env&gt;: relayer secp256k1 private key read from the variable named by config relayer_key_env</code> |

## err-gas-balance-floor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gas-balance-floor</code> | <code>error_code</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>ServiceError::BalanceFloor.code() = "balance_floor"</code> |

## err-gas-conflict

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gas-conflict</code> | <code>error_code</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>ServiceError::Conflict.code() = "conflict"</code> |

## err-gas-expired-quote

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gas-expired-quote</code> | <code>error_code</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>ServiceError::ExpiredQuote.code() = "expired_quote"</code> |

## err-gas-incompatible-delegation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gas-incompatible-delegation</code> | <code>error_code</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>ServiceError::IncompatibleDelegation.code() = "incompatible_delegation"</code> |

## err-gas-internal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gas-internal</code> | <code>error_code</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>ServiceError::Accept.code() = "internal"</code> |

## err-gas-length-required

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gas-length-required</code> | <code>error_code</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>ServiceError::LengthRequired.code() = "length_required"</code> |

## err-gas-malformed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gas-malformed</code> | <code>error_code</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>ServiceError::Malformed.code() = "malformed"</code> |

## err-gas-method-not-allowed

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gas-method-not-allowed</code> | <code>error_code</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>ServiceError::MethodNotAllowed.code() = "method_not_allowed"</code> |

## err-gas-not-found

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gas-not-found</code> | <code>error_code</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>ServiceError::NotFound.code() = "not_found"</code> |

## err-gas-origin-refused

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gas-origin-refused</code> | <code>error_code</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>ServiceError::OriginRefused.code() = "origin_refused"</code> |

## err-gas-refused

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gas-refused</code> | <code>error_code</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>ServiceError::Refused.code() = "refused"</code> |

## err-gas-replay-state-unavailable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gas-replay-state-unavailable</code> | <code>error_code</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>ServiceError::ReplayStateUnavailable.code() = "replay_state_unavailable"</code> |

## err-gas-signing-outcome-unknown

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gas-signing-outcome-unknown</code> | <code>error_code</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>ServiceError::SigningUnknown.code() = "signing_outcome_unknown"</code> |

## err-gas-stale-rate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gas-stale-rate</code> | <code>error_code</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>ServiceError::StaleRate.code() = "stale_rate"</code> |

## err-gas-timeout

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gas-timeout</code> | <code>error_code</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>ServiceError::Timeout.code() = "timeout"</code> |

## err-gas-too-large

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gas-too-large</code> | <code>error_code</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>ServiceError::TooLarge.code() = "too_large"</code> |

## err-gas-unavailable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-gas-unavailable</code> | <code>error_code</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>ServiceError::Unavailable.code() = "unavailable"</code> |

## fmt-gas-admission

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gas-admission</code> | <code>file_format</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>Rust model: #[serde(deny_unknown_fields)] pub struct Admission { pub identity: Word, pub request_digest: Word, pub batch_nonce: Word, pub interval: u64, }</code> |

## fmt-gas-completion

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gas-completion</code> | <code>file_format</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>Completion = consumed &#124; included &#124; reverted &#124; cancelled (tag outcome)</code> |

## fmt-gas-entry

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gas-entry</code> | <code>file_format</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>Entry = policy_bound &#124; quote_reserved &#124; signing_intent &#124; quote_admitted &#124; liability_released &#124; quoted &#124; prepared &#124; completed &#124; refused &#124; receipt_observed &#124; rate_published &#124; rate_settled &#124; rate_prepared &#124; rate_broadcast &#124; rate_replaced &#124; rate_finalized (tag kind)</code> |

## fmt-gas-fees

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gas-fees</code> | <code>file_format</code> | [services/interop/crates/layerx-gas-station/src/tx.rs](../../../services/interop/crates/layerx-gas-station/src/tx.rs) | <code>Rust model: #[serde(deny_unknown_fields)] pub struct Fees { pub gas_limit: u64, #[serde(with = "fee_amount")] pub max_fee_per_gas: u128, #[serde(with = "fee_amount")] pub max_priority_fee_per_gas: u128, }</code> |

## fmt-gas-http-reason-phrases

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gas-http-reason-phrases</code> | <code>file_format</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>fn reason(status: u16) -&gt; &amp;'static str: OK &#124; Forbidden &#124; Conflict</code> |

## fmt-gas-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gas-key</code> | <code>file_format</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>Rust model: #[serde(deny_unknown_fields)] pub struct Key { pub sponsor: Address, pub quote_nonce: Word, }</code> |

## fmt-gas-liability-release

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gas-liability-release</code> | <code>file_format</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>LiabilityRelease = expired &#124; settled &#124; refused &#124; consumed (tag reason)</code> |

## fmt-gas-publication

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gas-publication</code> | <code>file_format</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>Rust model: #[serde(deny_unknown_fields)] pub struct Publication { pub owner: Address, pub nonce: u64, pub hash: Word, pub rate: Word, pub gas_limit: u64, #[serde(with = "crate::tx::fee_amount")] pub max_fee_per_gas: u128, pub signed_at: u64, }</code> |

## fmt-gas-publication-transaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gas-publication-transaction</code> | <code>file_format</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>Rust model: #[serde(deny_unknown_fields)] pub struct PublicationTransaction { pub version: u16, pub publication: Publication, pub chain_id: u64, pub paymaster: Address, #[serde(with = "crate::tx::fee_amount")] pub max_priority_fee_per_gas: u128, pub raw: Vec&lt;u8&gt;, pub cancellation: bool, }</code> |

## fmt-gas-quote-record

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gas-quote-record</code> | <code>file_format</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>Rust model: #[serde(deny_unknown_fields)] pub struct QuoteRecord { pub key: Key, pub did: String, pub batch_nonce: Word, pub chain_id: u64, pub paymaster: Address, pub account: Address, pub token: Address, pub maximum: Word, pub amount: Word, pub deadline: u64, pub gas_cost: Word, pub issued_at: u64, pub signature: Vec&lt;u8&gt;, pub fees: Fees, }</code> |

## fmt-gas-rate-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gas-rate-file</code> | <code>file_format</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>rate file: lines name=integer for rate, set_at, not_after; blank and # lines ignored; duplicates or other names refused (RateFileMalformed)</code> |

## fmt-gas-settlement

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gas-settlement</code> | <code>file_format</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>Rust model: #[serde(deny_unknown_fields)] pub struct Settlement { pub block_number: u64, pub gas_used: u64, #[serde(with = "crate::tx::fee_amount")] pub cost_wei: u128, pub succeeded: bool, }</code> |

## fmt-gas-submission

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-gas-submission</code> | <code>file_format</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>Rust model: #[serde(deny_unknown_fields)] pub struct Submission { pub did: String, pub batch_nonce: Word, pub nonce: u64, pub hash: Word, pub raw: Vec&lt;u8&gt;, }</code> |

## gas-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-address</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/quote.rs](../../../services/interop/crates/layerx-gas-station/src/quote.rs) | <code>pub type Address = [u8; 20]</code> |

## gas-address-word

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-address-word</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/quote.rs](../../../services/interop/crates/layerx-gas-station/src/quote.rs) | <code>pub fn address_word(address: Address) -&gt; Word</code> |

## gas-admission

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-admission</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub struct Admission</code> |

## gas-admission-identity

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-admission-identity</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub fn identity(account: Address, nonce: Word, interval: u64) -&gt; Word</code> |

## gas-authorization

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-authorization</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/tx.rs](../../../services/interop/crates/layerx-gas-station/src/tx.rs) | <code>pub struct Authorization</code> |

## gas-authorization-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-authorization-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/tx.rs](../../../services/interop/crates/layerx-gas-station/src/tx.rs) | <code>pub fn authorization_digest(auth: &amp;Authorization) -&gt; Result&lt;Word, TxError&gt;</code> |

## gas-batch-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-batch-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/tx.rs](../../../services/interop/crates/layerx-gas-station/src/tx.rs) | <code>pub fn batch_digest(chain_id: u64, account: Address, batch_nonce: Word, calls: &amp;[Call], quote: &amp;Quote) -&gt; Result&lt;Word, TxError&gt;</code> |

## gas-budget-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-budget-key</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/policy.rs](../../../services/interop/crates/layerx-gas-station/src/policy.rs) | <code>pub struct BudgetKey(String)</code> |

## gas-budget-key-account

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-budget-key-account</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/policy.rs](../../../services/interop/crates/layerx-gas-station/src/policy.rs) | <code>pub fn account(account: Address) -&gt; Result&lt;Self, PolicyRefusal&gt;</code> |

## gas-budget-key-as-str

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-budget-key-as-str</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/policy.rs](../../../services/interop/crates/layerx-gas-station/src/policy.rs) | <code>pub fn as_str(&amp;self) -&gt; &amp;str</code> |

## gas-budget-key-parse

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-budget-key-parse</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/policy.rs](../../../services/interop/crates/layerx-gas-station/src/policy.rs) | <code>pub fn parse(value: &amp;str) -&gt; Result&lt;Self, PolicyRefusal&gt;</code> |

## gas-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-bytes</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rpc.rs](../../../services/interop/crates/layerx-gas-station/src/rpc.rs) | <code>pub fn bytes(text: &amp;str) -&gt; Result&lt;Vec&lt;u8&gt;, RpcFault&gt;</code> |

## gas-call

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-call</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/tx.rs](../../../services/interop/crates/layerx-gas-station/src/tx.rs) | <code>pub struct Call</code> |

## gas-cancellation-gas

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-cancellation-gas</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/tx.rs](../../../services/interop/crates/layerx-gas-station/src/tx.rs) | <code>pub const CANCELLATION_GAS: u64</code> |

## gas-completion

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-completion</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub enum Completion</code> |

## gas-config-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-config-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>pub struct ConfigError</code> |

## gas-configured-rpc

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-configured-rpc</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rpc.rs](../../../services/interop/crates/layerx-gas-station/src/rpc.rs) | <code>pub struct ConfiguredRpc&lt;E&gt;</code> |

## gas-configured-rpc-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-configured-rpc-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rpc.rs](../../../services/interop/crates/layerx-gas-station/src/rpc.rs) | <code>pub fn new(config: &amp;StationConfig, exchange: E) -&gt; Result&lt;Self, RpcFault&gt;</code> |

## gas-day-seconds

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-day-seconds</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>pub const DAY_SECONDS: u64</code> |

## gas-deserialize

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-deserialize</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/tx.rs](../../../services/interop/crates/layerx-gas-station/src/tx.rs) | <code>pub fn deserialize&lt;'de, D: Deserializer&lt;'de&gt;&gt;(deserializer: D) -&gt; Result&lt;u128, D::Error&gt;</code> |

## gas-did-budget

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-did-budget</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/policy.rs](../../../services/interop/crates/layerx-gas-station/src/policy.rs) | <code>pub struct DidBudget</code> |

| Field | Rule |
|---|---|
| interface | The gas station funds DID budgets with EVM transactions and confirms them from pax_sequencerReceipt. |
| storage | Budget ledger in the station state directory. |
| errors | Station errors for exhausted budget, transport and unverified receipt. |
| events | none |
| gas | Each funding transaction pays the network gas price. |
| activation | Starts on service launch. |
| invariants | A budget is debited once per funded transaction. |
| pause | none |

## gas-did-budget-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-did-budget-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/policy.rs](../../../services/interop/crates/layerx-gas-station/src/policy.rs) | <code>pub fn new(gas_limit: u64) -&gt; Self</code> |

## gas-did-budget-used

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-did-budget-used</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/policy.rs](../../../services/interop/crates/layerx-gas-station/src/policy.rs) | <code>pub fn used(&amp;self, did: &amp;str, interval: u64) -&gt; u64</code> |

## gas-did-gas-limit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-did-gas-limit</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>pub struct DidGasLimit(pub u64)</code> |

## gas-drive

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-drive</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>pub fn drive&lt;S, R, P, C&gt;(listener: &amp;TcpListener, service: &amp;mut Service&lt;S, R, P, C&gt;, schedule: Schedule, log: &amp;mut impl Write) -&gt; Result&lt;Infallible, ServiceError&gt; where S: QuoteSigner, R: JsonRpc, P: PriceSource, C: FnMut() -&gt; Option&lt;u64&gt;,</code> |

## gas-encode-sponsored

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-encode-sponsored</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/tx.rs](../../../services/interop/crates/layerx-gas-station/src/tx.rs) | <code>pub fn encode_sponsored(calls: &amp;[Call], quote: &amp;SignedQuote, account_signature: &amp;[u8; 65]) -&gt; Result&lt;Vec&lt;u8&gt;, TxError&gt;</code> |

## gas-entry

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-entry</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub enum Entry</code> |

## gas-exchange

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-exchange</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rpc.rs](../../../services/interop/crates/layerx-gas-station/src/rpc.rs) | <code>pub trait Exchange</code> |

## gas-execution

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-execution</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rpc.rs](../../../services/interop/crates/layerx-gas-station/src/rpc.rs) | <code>pub struct Execution</code> |

## gas-fees

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-fees</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/tx.rs](../../../services/interop/crates/layerx-gas-station/src/tx.rs) | <code>pub struct Fees</code> |

## gas-fees-gas-cost

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-fees-gas-cost</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/tx.rs](../../../services/interop/crates/layerx-gas-station/src/tx.rs) | <code>pub fn gas_cost(self) -&gt; Result&lt;u128, TxError&gt;</code> |

## gas-fees-replacement

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-fees-replacement</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/tx.rs](../../../services/interop/crates/layerx-gas-station/src/tx.rs) | <code>pub fn replacement(self) -&gt; Result&lt;Self, TxError&gt;</code> |

## gas-fees-replaces

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-fees-replaces</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/tx.rs](../../../services/interop/crates/layerx-gas-station/src/tx.rs) | <code>pub fn replaces(self, original: Self) -&gt; Result&lt;bool, TxError&gt;</code> |

## gas-gas-station

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-gas-station</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/station.rs](../../../services/interop/crates/layerx-gas-station/src/station.rs) | <code>pub struct GasStation&lt;S, R, P&gt;</code> |

## gas-gas-station-admission

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-admission</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/station.rs](../../../services/interop/crates/layerx-gas-station/src/station.rs) | <code>pub fn admission(&amp;self, identity: Word, digest: Word, account: Address, interval: u64) -&gt; Result&lt;Option&lt;Key&gt;, StationError&gt;</code> |

## gas-gas-station-batch-nonce

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-batch-nonce</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/station.rs](../../../services/interop/crates/layerx-gas-station/src/station.rs) | <code>pub fn batch_nonce(&amp;self, account: Address) -&gt; Result&lt;Word, StationError&gt;</code> |

## gas-gas-station-chain-time

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-chain-time</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/station.rs](../../../services/interop/crates/layerx-gas-station/src/station.rs) | <code>pub fn chain_time(&amp;self) -&gt; Result&lt;u64, StationError&gt;</code> |

## gas-gas-station-journal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-journal</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/station.rs](../../../services/interop/crates/layerx-gas-station/src/station.rs) | <code>pub const fn journal(&amp;self) -&gt; &amp;Journal</code> |

## gas-gas-station-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/station.rs](../../../services/interop/crates/layerx-gas-station/src/station.rs) | <code>pub fn new(config: StationConfig, signer: S, rpc: R, prices: P, journal: Journal) -&gt; Result&lt;Self, StationError&gt;</code> |

## gas-gas-station-quote

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-quote</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/station.rs](../../../services/interop/crates/layerx-gas-station/src/station.rs) | <code>pub fn quote(&amp;mut self, request: &amp;QuoteRequest, fees: Fees, now: u64) -&gt; Result&lt;QuoteOutcome, StationError&gt;</code> |

## gas-gas-station-quote-admitted

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-quote-admitted</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/station.rs](../../../services/interop/crates/layerx-gas-station/src/station.rs) | <code>pub fn quote_admitted(&amp;mut self, request: &amp;QuoteRequest, fees: Fees, now: u64, admission: Option&lt;Admission&gt;) -&gt; Result&lt;QuoteOutcome, StationError&gt;</code> |

## gas-gas-station-recover

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-recover</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/station.rs](../../../services/interop/crates/layerx-gas-station/src/station.rs) | <code>pub fn recover(&amp;mut self, now: u64, budget: Duration) -&gt; Result&lt;Recovery, StationError&gt;</code> |

## gas-gas-station-resume

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-resume</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/station.rs](../../../services/interop/crates/layerx-gas-station/src/station.rs) | <code>pub fn resume(&amp;mut self, key: Key, now: u64) -&gt; Result&lt;Progress, StationError&gt;</code> |

## gas-gas-station-retry

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-retry</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/station.rs](../../../services/interop/crates/layerx-gas-station/src/station.rs) | <code>pub fn retry(&amp;mut self, key: Key, account: Address, relayer_signature: &amp;[u8; 65], now: u64) -&gt; Result&lt;Progress, StationError&gt;</code> |

## gas-gas-station-status

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-status</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/station.rs](../../../services/interop/crates/layerx-gas-station/src/station.rs) | <code>pub fn status(&amp;self, key: Key, account: Address, relayer_signature: &amp;[u8; 65]) -&gt; Result&lt;Status, StationError&gt;</code> |

## gas-gas-station-submit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-submit</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/station.rs](../../../services/interop/crates/layerx-gas-station/src/station.rs) | <code>pub fn submit(&amp;mut self, request: &amp;SubmitRequest, now: u64) -&gt; Result&lt;Progress, StationError&gt;</code> |

## gas-gas-station-unresolved

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-unresolved</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/station.rs](../../../services/interop/crates/layerx-gas-station/src/station.rs) | <code>pub fn unresolved(&amp;self) -&gt; Vec&lt;Key&gt;</code> |

## gas-governed-rate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-governed-rate</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/price.rs](../../../services/interop/crates/layerx-gas-station/src/price.rs) | <code>pub struct GovernedRate</code> |

## gas-hex

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-hex</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rpc.rs](../../../services/interop/crates/layerx-gas-station/src/rpc.rs) | <code>pub fn hex(bytes: &amp;[u8]) -&gt; String</code> |

## gas-https-exchange

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-https-exchange</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rpc.rs](../../../services/interop/crates/layerx-gas-station/src/rpc.rs) | <code>pub struct HttpsExchange</code> |

## gas-https-exchange-read-response

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-https-exchange-read-response</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rpc.rs](../../../services/interop/crates/layerx-gas-station/src/rpc.rs) | <code>pub fn read_response(stream: &amp;mut impl Read, budget: Duration) -&gt; Result&lt;Value, RpcFault&gt;</code> |

## gas-item

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-item</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub struct Item</code> |

## gas-journal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-journal</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub struct Journal</code> |

## gas-journal-append

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-journal-append</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub fn append(&amp;mut self, entry: &amp;Entry) -&gt; Result&lt;(), JournalError&gt;</code> |

## gas-journal-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-journal-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub enum JournalError</code> |

## gas-journal-open

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-journal-open</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub fn open(path: &amp;Path) -&gt; Result&lt;Self, JournalError&gt;</code> |

## gas-journal-state

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-journal-state</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub const fn state(&amp;self) -&gt; &amp;State</code> |

## gas-json-rpc

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-json-rpc</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rpc.rs](../../../services/interop/crates/layerx-gas-station/src/rpc.rs) | <code>pub trait JsonRpc</code> |

## gas-keccak

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-keccak</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/quote.rs](../../../services/interop/crates/layerx-gas-station/src/quote.rs) | <code>pub fn keccak(bytes: &amp;[u8]) -&gt; Word</code> |

## gas-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-key</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub struct Key</code> |

## gas-liability-release

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-liability-release</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub enum LiabilityRelease</code> |

## gas-limits

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-limits</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>pub struct Limits</code> |

## gas-local-signer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-local-signer</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/signer.rs](../../../services/interop/crates/layerx-gas-station/src/signer.rs) | <code>pub struct LocalSigner</code> |

## gas-local-signer-from-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-local-signer-from-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/signer.rs](../../../services/interop/crates/layerx-gas-station/src/signer.rs) | <code>pub fn from_config(config: &amp;StationConfig) -&gt; Result&lt;Self, SignerError&gt;</code> |

## gas-local-signer-from-env

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-local-signer-from-env</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/signer.rs](../../../services/interop/crates/layerx-gas-station/src/signer.rs) | <code>pub fn from_env(name: &amp;str) -&gt; Result&lt;Self, SignerError&gt;</code> |

## gas-local-signer-sign-authorization

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-local-signer-sign-authorization</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/signer.rs](../../../services/interop/crates/layerx-gas-station/src/signer.rs) | <code>pub fn sign_authorization(&amp;self, chain_id: u64, delegate: Address, nonce: u64) -&gt; Result&lt;SignedAuthorization, SignerError&gt;</code> |

## gas-next-wait

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-next-wait</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>pub fn next_wait(cadence_seconds: u64, result: &amp;Result&lt;Publication, RateRefusal&gt;) -&gt; u64</code> |

## gas-pax-base-units

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-pax-base-units</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/price.rs](../../../services/interop/crates/layerx-gas-station/src/price.rs) | <code>pub const PAX_BASE_UNITS: u128</code> |

## gas-paymaster-rate-source

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-paymaster-rate-source</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/price.rs](../../../services/interop/crates/layerx-gas-station/src/price.rs) | <code>pub struct PaymasterRateSource&lt;R&gt;</code> |

## gas-paymaster-rate-source-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-paymaster-rate-source-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/price.rs](../../../services/interop/crates/layerx-gas-station/src/price.rs) | <code>pub const fn new(rpc: R, paymaster: Address) -&gt; Self</code> |

## gas-paymaster-required

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-paymaster-required</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>pub struct PaymasterRequired(pub Address)</code> |

## gas-policy-refusal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-policy-refusal</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/policy.rs](../../../services/interop/crates/layerx-gas-station/src/policy.rs) | <code>pub enum PolicyRefusal</code> |

| Field | Rule |
|---|---|
| interface | The gas station funds DID budgets with EVM transactions and confirms them from pax_sequencerReceipt. |
| storage | Budget ledger in the station state directory. |
| errors | Station errors for exhausted budget, transport and unverified receipt. |
| events | none |
| gas | Each funding transaction pays the network gas price. |
| activation | Starts on service launch. |
| invariants | A budget is debited once per funded transaction. |
| pause | none |

## gas-prepare-submission

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-prepare-submission</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/station.rs](../../../services/interop/crates/layerx-gas-station/src/station.rs) | <code>pub fn prepare_submission(journal: &amp;mut Journal, mut request: TransactionRequest&lt;'_&gt;, did: &amp;str, signer: &amp;impl QuoteSigner) -&gt; Result&lt;Submission, StationError&gt;</code> |

## gas-price-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-price-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/price.rs](../../../services/interop/crates/layerx-gas-station/src/price.rs) | <code>pub enum PriceError</code> |

## gas-price-source

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-price-source</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/price.rs](../../../services/interop/crates/layerx-gas-station/src/price.rs) | <code>pub trait PriceSource</code> |

## gas-pricing

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-pricing</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/price.rs](../../../services/interop/crates/layerx-gas-station/src/price.rs) | <code>pub struct Pricing</code> |

## gas-pricing-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-pricing-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/price.rs](../../../services/interop/crates/layerx-gas-station/src/price.rs) | <code>pub fn new(config: &amp;StationConfig) -&gt; Result&lt;Self, ConfigError&gt;</code> |

## gas-pricing-quote

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-pricing-quote</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/price.rs](../../../services/interop/crates/layerx-gas-station/src/price.rs) | <code>pub fn quote(&amp;self, rate: &amp;GovernedRate, gas_cost: u128, now: u64) -&gt; Result&lt;u128, PriceError&gt;</code> |

## gas-pricing-validate-amount

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-pricing-validate-amount</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/price.rs](../../../services/interop/crates/layerx-gas-station/src/price.rs) | <code>pub fn validate_amount(&amp;self, rate: &amp;GovernedRate, gas_cost: u128, amount: u128, now: u64) -&gt; Result&lt;(), PriceError&gt;</code> |

## gas-progress

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-progress</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/station.rs](../../../services/interop/crates/layerx-gas-station/src/station.rs) | <code>pub enum Progress</code> |

## gas-publication

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-publication</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub struct Publication</code> |

## gas-publication-transaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-publication-transaction</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub struct PublicationTransaction</code> |

## gas-publisher-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-publisher-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>pub struct PublisherConfig</code> |

## gas-publisher-config-daily-wei-ceiling

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-publisher-config-daily-wei-ceiling</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>pub fn daily_wei_ceiling(&amp;self) -&gt; Option&lt;u128&gt;</code> |

## gas-publisher-config-load

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-publisher-config-load</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>pub fn load(path: &amp;Path) -&gt; Result&lt;Self, ConfigError&gt;</code> |

## gas-publisher-config-parse

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-publisher-config-parse</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>pub fn parse(text: &amp;str) -&gt; Result&lt;Self, ConfigError&gt;</code> |

## gas-publisher-config-validate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-publisher-config-validate</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>pub fn validate(&amp;self) -&gt; Result&lt;(), ConfigError&gt;</code> |

## gas-quantity

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quantity</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rpc.rs](../../../services/interop/crates/layerx-gas-station/src/rpc.rs) | <code>pub fn quantity(text: &amp;str) -&gt; Result&lt;u128, RpcFault&gt;</code> |

## gas-quote

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quote</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/quote.rs](../../../services/interop/crates/layerx-gas-station/src/quote.rs) | <code>pub struct Quote</code> |

## gas-quote-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quote-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/quote.rs](../../../services/interop/crates/layerx-gas-station/src/quote.rs) | <code>pub fn quote_digest(chain_id: Word, account: Address, quote: &amp;Quote) -&gt; Word</code> |

## gas-quote-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quote-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/lib.rs](../../../services/interop/crates/layerx-gas-station/src/lib.rs) | <code>pub enum QuoteError</code> |

## gas-quote-outcome

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quote-outcome</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/station.rs](../../../services/interop/crates/layerx-gas-station/src/station.rs) | <code>pub enum QuoteOutcome</code> |

## gas-quote-policy

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/policy.rs](../../../services/interop/crates/layerx-gas-station/src/policy.rs) | <code>pub struct QuotePolicy</code> |

## gas-quote-policy-did-budget

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy-did-budget</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/policy.rs](../../../services/interop/crates/layerx-gas-station/src/policy.rs) | <code>pub const fn did_budget(&amp;self) -&gt; &amp;DidBudget</code> |

## gas-quote-policy-finalized-spend

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy-finalized-spend</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/policy.rs](../../../services/interop/crates/layerx-gas-station/src/policy.rs) | <code>pub const fn finalized_spend(&amp;self) -&gt; u128</code> |

## gas-quote-policy-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/policy.rs](../../../services/interop/crates/layerx-gas-station/src/policy.rs) | <code>pub fn new(config: &amp;StationConfig) -&gt; Result&lt;Self, ConfigError&gt;</code> |

## gas-quote-policy-outstanding-pax

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy-outstanding-pax</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/policy.rs](../../../services/interop/crates/layerx-gas-station/src/policy.rs) | <code>pub const fn outstanding_pax(&amp;self) -&gt; u128</code> |

## gas-quote-policy-quote-for-budget

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy-quote-for-budget</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/policy.rs](../../../services/interop/crates/layerx-gas-station/src/policy.rs) | <code>pub fn quote_for_budget&lt;E&gt;(&amp;mut self, request: &amp;crate::QuoteRequest, rate: &amp;crate::price::GovernedRate, fees: crate::tx::Fees, signer: &amp;impl crate::signer::QuoteSigner, budget: Result&lt;&amp;BudgetKey, E&gt;, context: (u128, u64)) -&gt; Result&lt;crate::SignedQuote, crate::QuoteError&gt;</code> |

| Field | Rule |
|---|---|
| interface | The gas station funds DID budgets with EVM transactions and confirms them from pax_sequencerReceipt. |
| storage | Budget ledger in the station state directory. |
| errors | Station errors for exhausted budget, transport and unverified receipt. |
| events | none |
| gas | Each funding transaction pays the network gas price. |
| activation | Starts on service launch. |
| invariants | A budget is debited once per funded transaction. |
| pause | none |

## gas-quote-policy-quote-for-did

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy-quote-for-did</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/policy.rs](../../../services/interop/crates/layerx-gas-station/src/policy.rs) | <code>pub fn quote_for_did(&amp;mut self, request: &amp;crate::QuoteRequest, rate: &amp;crate::price::GovernedRate, fees: crate::tx::Fees, signer: &amp;impl crate::signer::QuoteSigner, did: &amp;str, context: (u128, u64)) -&gt; Result&lt;crate::SignedQuote, crate::QuoteError&gt;</code> |

## gas-quote-policy-reserve

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy-reserve</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/policy.rs](../../../services/interop/crates/layerx-gas-station/src/policy.rs) | <code>pub fn reserve(&amp;mut self, account: Address, amount: u128, gas_cost: u128, balance: u128, now: u64) -&gt; Result&lt;(), PolicyRefusal&gt;</code> |

## gas-quote-policy-reserve-for-budget

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy-reserve-for-budget</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/policy.rs](../../../services/interop/crates/layerx-gas-station/src/policy.rs) | <code>pub fn reserve_for_budget&lt;E&gt;(&amp;mut self, budget: Result&lt;&amp;BudgetKey, E&gt;, account: Address, amount: u128, fees: crate::tx::Fees, balance: u128, now: u64) -&gt; Result&lt;(), PolicyRefusal&gt;</code> |

| Field | Rule |
|---|---|
| interface | The gas station funds DID budgets with EVM transactions and confirms them from pax_sequencerReceipt. |
| storage | Budget ledger in the station state directory. |
| errors | Station errors for exhausted budget, transport and unverified receipt. |
| events | none |
| gas | Each funding transaction pays the network gas price. |
| activation | Starts on service launch. |
| invariants | A budget is debited once per funded transaction. |
| pause | none |

## gas-quote-policy-reserve-for-did

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy-reserve-for-did</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/policy.rs](../../../services/interop/crates/layerx-gas-station/src/policy.rs) | <code>pub fn reserve_for_did(&amp;mut self, did: &amp;str, account: Address, amount: u128, fees: crate::tx::Fees, balance: u128, now: u64) -&gt; Result&lt;(), PolicyRefusal&gt;</code> |

## gas-quote-policy-used

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy-used</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/policy.rs](../../../services/interop/crates/layerx-gas-station/src/policy.rs) | <code>pub fn used(&amp;self, key: &amp;str, now: u64) -&gt; u128</code> |

## gas-quote-policy-with-did-budget

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy-with-did-budget</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/policy.rs](../../../services/interop/crates/layerx-gas-station/src/policy.rs) | <code>pub fn with_did_budget(mut self, budget: DidBudget) -&gt; Self</code> |

## gas-quote-record

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quote-record</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub struct QuoteRecord</code> |

## gas-quote-record-budget-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quote-record-budget-key</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub fn budget_key(&amp;self) -&gt; Result&lt;crate::policy::BudgetKey, JournalError&gt;</code> |

## gas-quote-record-signed-quote

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quote-record-signed-quote</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub fn signed_quote(&amp;self) -&gt; Result&lt;SignedQuote, JournalError&gt;</code> |

## gas-quote-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quote-request</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/lib.rs](../../../services/interop/crates/layerx-gas-station/src/lib.rs) | <code>pub struct QuoteRequest</code> |

## gas-quote-signer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quote-signer</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/signer.rs](../../../services/interop/crates/layerx-gas-station/src/signer.rs) | <code>pub trait QuoteSigner</code> |

## gas-quote-word

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-quote-word</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/quote.rs](../../../services/interop/crates/layerx-gas-station/src/quote.rs) | <code>pub fn word(value: u128) -&gt; Word</code> |

## gas-rate-file

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-rate-file</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>pub struct RateFile</code> |

## gas-rate-file-load

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-rate-file-load</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>pub fn load(path: &amp;Path) -&gt; Result&lt;Self, RateRefusal&gt;</code> |

## gas-rate-file-parse

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-rate-file-parse</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>pub fn parse(text: &amp;str) -&gt; Result&lt;Self, RateRefusal&gt;</code> |

## gas-rate-gas-limit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-rate-gas-limit</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>pub const RATE_GAS_LIMIT: u64</code> |

## gas-rate-publisher

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-rate-publisher</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>pub struct RatePublisher&lt;S, R&gt;</code> |

## gas-rate-publisher-cadence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-rate-publisher-cadence</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>pub const fn cadence(&amp;self) -&gt; u64</code> |

## gas-rate-publisher-confirm-by-receipt

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-rate-publisher-confirm-by-receipt</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>pub fn confirm_by_receipt(&amp;mut self, transaction: &amp;PublicationTransaction) -&gt; Result&lt;Publication, RateRefusal&gt;</code> |

## gas-rate-publisher-journal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-rate-publisher-journal</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>pub const fn journal(&amp;self) -&gt; &amp;Journal</code> |

## gas-rate-publisher-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-rate-publisher-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>pub fn new(config: PublisherConfig, signer: S, rpc: R, journal: Journal, poll: Duration) -&gt; Result&lt;Self, RateRefusal&gt;</code> |

## gas-rate-publisher-publish

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-rate-publisher-publish</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>pub fn publish(&amp;mut self, rate_file: &amp;Path) -&gt; Result&lt;Publication, RateRefusal&gt;</code> |

## gas-rate-publisher-recover-pending

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-rate-publisher-recover-pending</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>pub fn recover_pending(&amp;mut self) -&gt; Result&lt;(), RateRefusal&gt;</code> |

## gas-rate-refusal

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-rate-refusal</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rate.rs](../../../services/interop/crates/layerx-gas-station/src/rate.rs) | <code>pub enum RateRefusal</code> |

## gas-read

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-read</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rpc.rs](../../../services/interop/crates/layerx-gas-station/src/rpc.rs) | <code>pub fn read&lt;T: DeserializeOwned&gt;(rpc: &amp;impl JsonRpc, method: &amp;str, params: Value) -&gt; Result&lt;T, RpcFault&gt;</code> |

## gas-recover

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-recover</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/tx.rs](../../../services/interop/crates/layerx-gas-station/src/tx.rs) | <code>pub fn recover(digest: Word, signature: &amp;[u8; 65]) -&gt; Result&lt;Address, TxError&gt;</code> |

## gas-recovery

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-recovery</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/station.rs](../../../services/interop/crates/layerx-gas-station/src/station.rs) | <code>pub struct Recovery</code> |

## gas-registered-sender

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-registered-sender</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>pub struct RegisteredSender</code> |

## gas-replacement-bump-percent

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-replacement-bump-percent</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/tx.rs](../../../services/interop/crates/layerx-gas-station/src/tx.rs) | <code>pub const REPLACEMENT_BUMP_PERCENT: u128</code> |

## gas-rpc-fault

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-rpc-fault</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rpc.rs](../../../services/interop/crates/layerx-gas-station/src/rpc.rs) | <code>pub enum RpcFault</code> |

| Field | Rule |
|---|---|
| interface | The gas station funds DID budgets with EVM transactions and confirms them from pax_sequencerReceipt. |
| storage | Budget ledger in the station state directory. |
| errors | Station errors for exhausted budget, transport and unverified receipt. |
| events | none |
| gas | Each funding transaction pays the network gas price. |
| activation | Starts on service launch. |
| invariants | A budget is debited once per funded transaction. |
| pause | none |

## gas-rung

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-rung</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rpc.rs](../../../services/interop/crates/layerx-gas-station/src/rpc.rs) | <code>pub enum Rung</code> |

| Field | Rule |
|---|---|
| interface | The gas station funds DID budgets with EVM transactions and confirms them from pax_sequencerReceipt. |
| storage | Budget ledger in the station state directory. |
| errors | Station errors for exhausted budget, transport and unverified receipt. |
| events | none |
| gas | Each funding transaction pays the network gas price. |
| activation | Starts on service launch. |
| invariants | A budget is debited once per funded transaction. |
| pause | none |

## gas-schedule

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-schedule</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>pub struct Schedule</code> |

## gas-send-outcome

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-send-outcome</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rpc.rs](../../../services/interop/crates/layerx-gas-station/src/rpc.rs) | <code>pub enum SendOutcome</code> |

## gas-sequencer-endpoint

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-sequencer-endpoint</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rpc.rs](../../../services/interop/crates/layerx-gas-station/src/rpc.rs) | <code>pub struct SequencerEndpoint</code> |

## gas-sequencer-endpoint-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-sequencer-endpoint-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rpc.rs](../../../services/interop/crates/layerx-gas-station/src/rpc.rs) | <code>pub fn new(rpc: impl JsonRpc + 'static) -&gt; Self</code> |

## gas-sequencer-endpoint-receipt

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-sequencer-endpoint-receipt</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rpc.rs](../../../services/interop/crates/layerx-gas-station/src/rpc.rs) | <code>pub fn receipt(&amp;self, hash: &amp;Word) -&gt; Result&lt;Option&lt;SequencerReceipt&gt;, RpcFault&gt;</code> |

| Field | Rule |
|---|---|
| interface | The gas station funds DID budgets with EVM transactions and confirms them from pax_sequencerReceipt. |
| storage | Budget ledger in the station state directory. |
| errors | Station errors for exhausted budget, transport and unverified receipt. |
| events | none |
| gas | Each funding transaction pays the network gas price. |
| activation | Starts on service launch. |
| invariants | A budget is debited once per funded transaction. |
| pause | none |

## gas-sequencer-receipt

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-sequencer-receipt</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rpc.rs](../../../services/interop/crates/layerx-gas-station/src/rpc.rs) | <code>pub struct SequencerReceipt</code> |

| Field | Rule |
|---|---|
| interface | The gas station funds DID budgets with EVM transactions and confirms them from pax_sequencerReceipt. |
| storage | Budget ledger in the station state directory. |
| errors | Station errors for exhausted budget, transport and unverified receipt. |
| events | none |
| gas | Each funding transaction pays the network gas price. |
| activation | Starts on service launch. |
| invariants | A budget is debited once per funded transaction. |
| pause | none |

## gas-serialize

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-serialize</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/tx.rs](../../../services/interop/crates/layerx-gas-station/src/tx.rs) | <code>pub fn serialize&lt;S: Serializer&gt;(value: &amp;u128, serializer: S) -&gt; Result&lt;S::Ok, S::Error&gt;</code> |

## gas-serve

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-serve</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>pub fn serve&lt;S, R, P, C&gt;(listener: &amp;TcpListener, service: &amp;mut Service&lt;S, R, P, C&gt;, log: &amp;mut impl Write) -&gt; Result&lt;Infallible, ServiceError&gt; where S: QuoteSigner, R: JsonRpc, P: PriceSource, C: FnMut() -&gt; Option&lt;u64&gt;,</code> |

## gas-service

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-service</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>pub struct Service&lt;S, R, P, C&gt;</code> |

## gas-service-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-service-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>pub struct ServiceConfig</code> |

## gas-service-config-account-budget

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-service-config-account-budget</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>pub fn account_budget(&amp;self, account: &amp;Address) -&gt; u128</code> |

## gas-service-config-load

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-service-config-load</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>pub fn load(path: &amp;Path) -&gt; Result&lt;Self, ConfigError&gt;</code> |

## gas-service-config-parse

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-service-config-parse</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>pub fn parse(text: &amp;str) -&gt; Result&lt;Self, ConfigError&gt;</code> |

## gas-service-config-validate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-service-config-validate</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>pub fn validate(&amp;self) -&gt; Result&lt;(), ConfigError&gt;</code> |

## gas-service-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-service-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>pub enum ServiceError</code> |

| Field | Rule |
|---|---|
| interface | The gas station funds DID budgets with EVM transactions and confirms them from pax_sequencerReceipt. |
| storage | Budget ledger in the station state directory. |
| errors | Station errors for exhausted budget, transport and unverified receipt. |
| events | none |
| gas | Each funding transaction pays the network gas price. |
| activation | Starts on service launch. |
| invariants | A budget is debited once per funded transaction. |
| pause | none |

## gas-service-error-status

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-service-error-status</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>pub const fn status(self) -&gt; u16</code> |

## gas-service-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-service-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>pub fn new( config: &amp;ServiceConfig, mut station: GasStation&lt;S, R, P&gt;, clock: C, limits: Limits, ) -&gt; Self</code> |

## gas-service-recover

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-service-recover</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>pub fn recover(&amp;mut self, budget: Duration) -&gt; Result&lt;Recovery, StationError&gt;</code> |

## gas-service-respond

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-service-respond</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>pub fn respond(&amp;mut self, route: &amp;str, body: &amp;[u8]) -&gt; Result&lt;Value, ServiceError&gt;</code> |

| Field | Rule |
|---|---|
| interface | The gas station funds DID budgets with EVM transactions and confirms them from pax_sequencerReceipt. |
| storage | Budget ledger in the station state directory. |
| errors | Station errors for exhausted budget, transport and unverified receipt. |
| events | none |
| gas | Each funding transaction pays the network gas price. |
| activation | Starts on service launch. |
| invariants | A budget is debited once per funded transaction. |
| pause | none |

## gas-service-station

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-service-station</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>pub const fn station(&amp;self) -&gt; &amp;GasStation&lt;S, R, P&gt;</code> |

## gas-service-with-sequencer

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-service-with-sequencer</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>pub fn with_sequencer(mut self, sequencer: SequencerEndpoint) -&gt; Self</code> |

## gas-settlement

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-settlement</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub struct Settlement</code> |

## gas-sidiora

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-sidiora</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/quote.rs](../../../services/interop/crates/layerx-gas-station/src/quote.rs) | <code>pub const SIDIORA: Address</code> |

## gas-sign

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-sign</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/tx.rs](../../../services/interop/crates/layerx-gas-station/src/tx.rs) | <code>pub fn sign(request: &amp;TransactionRequest&lt;'_&gt;, signer: &amp;impl QuoteSigner) -&gt; Result&lt;SignedTransaction, TxError&gt;</code> |

## gas-sign-call

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-sign-call</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/tx.rs](../../../services/interop/crates/layerx-gas-station/src/tx.rs) | <code>pub fn sign_call(chain_id: u64, nonce: u64, fees: Fees, to: Address, data: &amp;[u8], signer: &amp;impl QuoteSigner) -&gt; Result&lt;SignedTransaction, TxError&gt;</code> |

## gas-sign-cancellation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-sign-cancellation</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/tx.rs](../../../services/interop/crates/layerx-gas-station/src/tx.rs) | <code>pub fn sign_cancellation(chain_id: u64, nonce: u64, fees: Fees, signer: &amp;impl QuoteSigner) -&gt; Result&lt;SignedTransaction, TxError&gt;</code> |

## gas-signed-quote

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-signed-quote</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/lib.rs](../../../services/interop/crates/layerx-gas-station/src/lib.rs) | <code>pub struct SignedQuote</code> |

## gas-signed-transaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-signed-transaction</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/tx.rs](../../../services/interop/crates/layerx-gas-station/src/tx.rs) | <code>pub struct SignedTransaction</code> |

## gas-signer-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-signer-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/signer.rs](../../../services/interop/crates/layerx-gas-station/src/signer.rs) | <code>pub enum SignerError</code> |

## gas-state

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-state</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub struct State</code> |

## gas-state-active-pax

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-state-active-pax</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub fn active_pax(&amp;self) -&gt; Result&lt;u128, JournalError&gt;</code> |

## gas-state-did-spend

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-state-did-spend</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub fn did_spend(&amp;self, did: &amp;str, interval_seconds: u64, interval: u64) -&gt; Result&lt;(u128, u64), JournalError&gt;</code> |

## gas-state-finalized-spend

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-state-finalized-spend</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub fn finalized_spend(&amp;self) -&gt; Result&lt;u128, JournalError&gt;</code> |

## gas-state-holds

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-state-holds</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub fn holds(&amp;self, sponsor: Address, nonce: u64) -&gt; bool</code> |

## gas-state-liability

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-state-liability</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub fn liability(&amp;self, key: Key) -&gt; Result&lt;u128, JournalError&gt;</code> |

## gas-state-next-nonce

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-state-next-nonce</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub fn next_nonce(&amp;self, sponsor: Address, pending: u64) -&gt; Option&lt;u64&gt;</code> |

## gas-state-publication-gas

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-state-publication-gas</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub fn publication_gas(&amp;self, day: u64) -&gt; u128</code> |

## gas-state-publication-wei

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-state-publication-wei</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub fn publication_wei(&amp;self, day: u64) -&gt; u128</code> |

## gas-state-refused

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-state-refused</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub fn refused(&amp;self, key: &amp;Key) -&gt; bool</code> |

## gas-state-reserved-wei

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-state-reserved-wei</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub fn reserved_wei(&amp;self) -&gt; u128</code> |

## gas-state-unsettled

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-state-unsettled</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub fn unsettled(&amp;self) -&gt; Vec&lt;Publication&gt;</code> |

## gas-station

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-station</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/lib.rs](../../../services/interop/crates/layerx-gas-station/src/lib.rs) | <code>pub struct Station&lt;S&gt;</code> |

## gas-station-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-station-config</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>pub struct StationConfig</code> |

## gas-station-config-load

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-station-config-load</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>pub fn load(path: &amp;Path) -&gt; Result&lt;Self, ConfigError&gt;</code> |

## gas-station-config-parse

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-station-config-parse</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>pub fn parse(text: &amp;str) -&gt; Result&lt;Self, ConfigError&gt;</code> |

## gas-station-config-validate

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-station-config-validate</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/config.rs](../../../services/interop/crates/layerx-gas-station/src/config.rs) | <code>pub fn validate(&amp;self) -&gt; Result&lt;(), ConfigError&gt;</code> |

## gas-station-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-station-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/station.rs](../../../services/interop/crates/layerx-gas-station/src/station.rs) | <code>pub enum StationError</code> |

## gas-station-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-station-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/lib.rs](../../../services/interop/crates/layerx-gas-station/src/lib.rs) | <code>pub fn new(config: StationConfig, signer: S) -&gt; Result&lt;Self, ConfigError&gt;</code> |

## gas-station-quote

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-station-quote</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/lib.rs](../../../services/interop/crates/layerx-gas-station/src/lib.rs) | <code>pub fn quote(&amp;mut self, request: &amp;QuoteRequest, rate: &amp;GovernedRate, balance: u128, now: u64) -&gt; Result&lt;SignedQuote, QuoteError&gt;</code> |

## gas-status

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-status</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/station.rs](../../../services/interop/crates/layerx-gas-station/src/station.rs) | <code>pub struct Status</code> |

## gas-submission

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-submission</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/journal.rs](../../../services/interop/crates/layerx-gas-station/src/journal.rs) | <code>pub struct Submission</code> |

## gas-submit-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-submit-request</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/station.rs](../../../services/interop/crates/layerx-gas-station/src/station.rs) | <code>pub struct SubmitRequest</code> |

## gas-transaction-request

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-transaction-request</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/tx.rs](../../../services/interop/crates/layerx-gas-station/src/tx.rs) | <code>pub struct TransactionRequest&lt;'a&gt;</code> |

## gas-tx-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-tx-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/tx.rs](../../../services/interop/crates/layerx-gas-station/src/tx.rs) | <code>pub enum TxError</code> |

## gas-word

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-word</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/quote.rs](../../../services/interop/crates/layerx-gas-station/src/quote.rs) | <code>pub type Word = [u8; 32]</code> |

## http-gas-healthz

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gas-healthz</code> | <code>http_route</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>GET /healthz</code> |

## http-gas-quote

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gas-quote</code> | <code>http_route</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>POST /quote {account, nonce, calls, maxTokenAmount, gasCost, ...}</code> |

| Field | Rule |
|---|---|
| interface | The gas station funds DID budgets with EVM transactions and confirms them from pax_sequencerReceipt. |
| storage | Budget ledger in the station state directory. |
| errors | Station errors for exhausted budget, transport and unverified receipt. |
| events | none |
| gas | Each funding transaction pays the network gas price. |
| activation | Starts on service launch. |
| invariants | A budget is debited once per funded transaction. |
| pause | none |

## http-gas-retry

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gas-retry</code> | <code>http_route</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>POST /retry {sponsor, quoteNonce, account, relayerSignature}</code> |

## http-gas-status

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gas-status</code> | <code>http_route</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>POST /status {sponsor, quoteNonce, account, relayerSignature}</code> |

| Field | Rule |
|---|---|
| interface | The gas station funds DID budgets with EVM transactions and confirms them from pax_sequencerReceipt. |
| storage | Budget ledger in the station state directory. |
| errors | Station errors for exhausted budget, transport and unverified receipt. |
| events | none |
| gas | Each funding transaction pays the network gas price. |
| activation | Starts on service launch. |
| invariants | A budget is debited once per funded transaction. |
| pause | none |

## http-gas-submit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.http-gas-submit</code> | <code>http_route</code> | [services/interop/crates/layerx-gas-station/src/service.rs](../../../services/interop/crates/layerx-gas-station/src/service.rs) | <code>POST /submit {call:{to,value,data}, authorization, batch:{chainId, account, nonce, calls, quote:{sponsor, token, maxTokenAmount, tokenAmount, deadline, quoteNonce, gasCost, decimals}}, accountSignature, relayerSignature}</code> |

| Field | Rule |
|---|---|
| interface | The gas station funds DID budgets with EVM transactions and confirms them from pax_sequencerReceipt. |
| storage | Budget ledger in the station state directory. |
| errors | Station errors for exhausted budget, transport and unverified receipt. |
| events | none |
| gas | Each funding transaction pays the network gas price. |
| activation | Starts on service launch. |
| invariants | A budget is debited once per funded transaction. |
| pause | none |

## rs-gas-policy-refusal-did-unavailable

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-policy-refusal-did-unavailable</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/policy.rs](../../../services/interop/crates/layerx-gas-station/src/policy.rs) | <code>PolicyRefusal::DidUnavailable</code> |

| Field | Rule |
|---|---|
| interface | The gas station funds DID budgets with EVM transactions and confirms them from pax_sequencerReceipt. |
| storage | Budget ledger in the station state directory. |
| errors | Station errors for exhausted budget, transport and unverified receipt. |
| events | none |
| gas | Each funding transaction pays the network gas price. |
| activation | Starts on service launch. |
| invariants | A budget is debited once per funded transaction. |
| pause | none |

## rs-gas-sequencer-receipt-decoder

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-gas-sequencer-receipt-decoder</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-gas-station/src/rpc.rs](../../../services/interop/crates/layerx-gas-station/src/rpc.rs) | <code>public decoder for the full pax_sequencerReceipt (logs address/topics/data/logIndex, canonical receipt bytes, keyId)</code> |

| Field | Rule |
|---|---|
| interface | The gas station funds DID budgets with EVM transactions and confirms them from pax_sequencerReceipt. |
| storage | Budget ledger in the station state directory. |
| errors | Station errors for exhausted budget, transport and unverified receipt. |
| events | none |
| gas | Each funding transaction pays the network gas price. |
| activation | Starts on service launch. |
| invariants | A budget is debited once per funded transaction. |
| pause | none |
