# Interop gas station contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## cfg-gas-gas-limit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-gas-limit</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:294</code> | <code>gas station service config gas_limit: u64 per sponsored submission</code> |

## cfg-gas-listen

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-listen</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:287</code> | <code>gas station service config listen: socket address</code> |

## cfg-gas-max-priority-fee-per-gas

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-max-priority-fee-per-gas</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:298</code> | <code>gas station service config max_priority_fee_per_gas: u128 wei</code> |

## cfg-gas-rate-balance-floor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-rate-balance-floor</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:201</code> | <code>gas station rate publisher config rate_balance_floor: u128 wei</code> |

## cfg-gas-rate-cadence-seconds

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-rate-cadence-seconds</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:199</code> | <code>gas station rate publisher config rate_cadence_seconds: u64</code> |

## cfg-gas-rate-confirmation-retry-seconds

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-rate-confirmation-retry-seconds</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:204</code> | <code>gas station rate publisher config rate_confirmation_retry_seconds: u64</code> |

## cfg-gas-rate-gas-budget-per-day

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-rate-gas-budget-per-day</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:200</code> | <code>gas station rate publisher config rate_gas_budget_per_day: u128 wei</code> |

## cfg-gas-rate-max-fee-per-gas

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-rate-max-fee-per-gas</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:203</code> | <code>gas station rate publisher config rate_max_fee_per_gas: u128 wei</code> |

## cfg-gas-rate-owner-key-env

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-rate-owner-key-env</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:198</code> | <code>gas station rate publisher config rate_owner_key_env: string env var name</code> |

## cfg-gas-registered-senders

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-registered-senders</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:299</code> | <code>gas station service config registered_senders: optional array of {address, budget}</code> |

## cfg-gas-sequencer-receipt-keys

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-sequencer-receipt-keys</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:205</code> | <code>gas station rate publisher config sequencer_receipt_keys: array of receipt key entries (key id, ed25519 key, height window)</code> |

## cfg-gas-station-balance-floor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-station-balance-floor</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:154</code> | <code>gas station config balance_floor: u128 wei (required; unknown fields refused)</code> |

## cfg-gas-station-chain-id

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-station-chain-id</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:142</code> | <code>gas station config chain_id: u64 EVM chain id (required; unknown fields refused)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-gas-station-chain-id</code> | notes | 53 reference lines in 20 files; callers lists the first reference in each file |

## cfg-gas-station-decimals

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-station-decimals</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:146</code> | <code>gas station config decimals: u8 token decimals (required; unknown fields refused)</code> |

## cfg-gas-station-endpoints

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-station-endpoints</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:143</code> | <code>gas station config endpoints: array of RPC endpoint URLs (required; unknown fields refused)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-gas-station-endpoints</code> | notes | 69 reference lines in 15 files; callers lists the first reference in each file |

## cfg-gas-station-interval-seconds

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-station-interval-seconds</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:153</code> | <code>gas station config interval_seconds: u64 (required; unknown fields refused)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-gas-station-interval-seconds</code> | notes | 26 reference lines in 11 files; callers lists the first reference in each file |

## cfg-gas-station-margin-bps

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-station-margin-bps</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:149</code> | <code>gas station config margin_bps: u32 basis points (required; unknown fields refused)</code> |

## cfg-gas-station-max-rate-age

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-station-max-rate-age</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:147</code> | <code>gas station config max_rate_age: u64 seconds (required; unknown fields refused)</code> |

## cfg-gas-station-paymaster

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-station-paymaster</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:144</code> | <code>gas station config paymaster: 0x address of the paymaster (required; unknown fields refused)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-gas-station-paymaster</code> | notes | 61 reference lines in 8 files; callers lists the first reference in each file |

## cfg-gas-station-per-account-limit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-station-per-account-limit</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:150</code> | <code>gas station config per_account_limit: u128 base units (required; unknown fields refused)</code> |

## cfg-gas-station-per-interval-limit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-station-per-interval-limit</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:151</code> | <code>gas station config per_interval_limit: u128 base units (required; unknown fields refused)</code> |

## cfg-gas-station-per-quote-limit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-station-per-quote-limit</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:152</code> | <code>gas station config per_quote_limit: u128 base units (required; unknown fields refused)</code> |

## cfg-gas-station-relayer-key-env

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-station-relayer-key-env</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:155</code> | <code>gas station config relayer_key_env: string env var name of the relayer key (required; unknown fields refused)</code> |

## cfg-gas-station-spread-bps

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-station-spread-bps</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:148</code> | <code>gas station config spread_bps: u32 basis points (required; unknown fields refused)</code> |

## cfg-gas-station-token

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-gas-station-token</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:145</code> | <code>gas station config token: 0x address of the fee token (required; unknown fields refused)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-gas-station-token</code> | notes | 145 reference lines in 28 files; callers lists the first reference in each file |

## cli-gas

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cli-gas</code> | <code>cli_command</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/main.rs:203</code> | <code>paxeer-gas-station --config PATH --journal PATH &#124; check --config PATH --rate-config PATH &#124; rate --config PATH --journal PATH --rate-file PATH</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cli-gas</code> | notes | station serves HTTP; check loads both configs and prints refusals with the flag naming the file (no key read, no network); rate runs the rate publisher; usage exits 2; rate file lines rate=, set_at=, not_after= (services/interop/crates/layerx-gas-station/src/rate.rs:92-94) |

## env-gas-gas-station-browser-origins

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-gas-gas-station-browser-origins</code> | <code>env_var</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:260</code> | <code>GAS_STATION_BROWSER_ORIGINS: comma-separated browser origins allowed by CORS; optional</code> |

## env-gas-gas-station-did-gas-limit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-gas-gas-station-did-gas-limit</code> | <code>env_var</code> | <code>change</code> | <code>services/interop/crates/layerx-gas-station/src/station.rs:209</code> | <code>GAS_STATION_DID_GAS_LIMIT: per-DID gas budget for sponsored submissions; optional</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-gas-gas-station-did-gas-limit</code> | notes | spec/layerx-merge/spec.kvx task 6.5.41 do_2 reads the 0.12 per-DID daily budget with interval_seconds 86400 |

Contract context <code>contract_context.11.gas-station</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-gas-gas-station-paymaster</code> | <code>env_var</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:170</code> | <code>GAS_STATION_PAYMASTER: paymaster contract address; required at load, must equal the config paymaster</code> |

## env-gas-rate-owner-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-gas-rate-owner-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:306</code> | <code>&lt;rate_owner_key_env&gt;: rate publisher owner key read from the variable named by config rate_owner_key_env</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-gas-rate-owner-key</code> | notes | services/interop/crates/layerx-gas-station/src/rate.rs:311-319 also checks the alternative key source variables |

## env-gas-relayer-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.env-gas-relayer-key</code> | <code>env_var</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/signer.rs:65</code> | <code>&lt;relayer_key_env&gt;: relayer secp256k1 private key read from the variable named by config relayer_key_env</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.env-gas-relayer-key</code> | notes | variable name is configuration; deploy files name the concrete variable |

## err-gas-balance-floor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gas-balance-floor</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:143</code> | <code>ServiceError::BalanceFloor.code() = "balance_floor"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gas-balance-floor</code> | notes | wire code returned by ServiceError::code (services/interop/crates/layerx-gas-station/src/service.rs:127) |

## err-gas-conflict

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gas-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:133</code> | <code>ServiceError::Conflict.code() = "conflict"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gas-conflict</code> | notes | wire code returned by ServiceError::code (services/interop/crates/layerx-gas-station/src/service.rs:127) |

## err-gas-expired-quote

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gas-expired-quote</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:138</code> | <code>ServiceError::ExpiredQuote.code() = "expired_quote"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gas-expired-quote</code> | notes | wire code returned by ServiceError::code (services/interop/crates/layerx-gas-station/src/service.rs:127) |

## err-gas-incompatible-delegation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gas-incompatible-delegation</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:139</code> | <code>ServiceError::IncompatibleDelegation.code() = "incompatible_delegation"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gas-incompatible-delegation</code> | notes | wire code returned by ServiceError::code (services/interop/crates/layerx-gas-station/src/service.rs:127) |

## err-gas-internal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gas-internal</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:141</code> | <code>ServiceError::Accept.code() = "internal"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gas-internal</code> | notes | wire code returned by ServiceError::code (services/interop/crates/layerx-gas-station/src/service.rs:127) |

## err-gas-length-required

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gas-length-required</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:134</code> | <code>ServiceError::LengthRequired.code() = "length_required"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gas-length-required</code> | notes | wire code returned by ServiceError::code (services/interop/crates/layerx-gas-station/src/service.rs:127) |

## err-gas-malformed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gas-malformed</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:129</code> | <code>ServiceError::Malformed.code() = "malformed"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gas-malformed</code> | notes | wire code returned by ServiceError::code (services/interop/crates/layerx-gas-station/src/service.rs:127) |

## err-gas-method-not-allowed

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gas-method-not-allowed</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:131</code> | <code>ServiceError::MethodNotAllowed.code() = "method_not_allowed"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gas-method-not-allowed</code> | notes | wire code returned by ServiceError::code (services/interop/crates/layerx-gas-station/src/service.rs:127); 45 reference lines in 22 files; callers lists the first reference in each file |

## err-gas-not-found

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gas-not-found</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:130</code> | <code>ServiceError::NotFound.code() = "not_found"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gas-not-found</code> | notes | wire code returned by ServiceError::code (services/interop/crates/layerx-gas-station/src/service.rs:127); 93 reference lines in 34 files; callers lists the first reference in each file |

## err-gas-origin-refused

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gas-origin-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:137</code> | <code>ServiceError::OriginRefused.code() = "origin_refused"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gas-origin-refused</code> | notes | wire code returned by ServiceError::code (services/interop/crates/layerx-gas-station/src/service.rs:127) |

## err-gas-refused

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gas-refused</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:136</code> | <code>ServiceError::Refused.code() = "refused"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gas-refused</code> | notes | wire code returned by ServiceError::code (services/interop/crates/layerx-gas-station/src/service.rs:127); 211 reference lines in 70 files; callers lists the first reference in each file |

## err-gas-replay-state-unavailable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gas-replay-state-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:140</code> | <code>ServiceError::ReplayStateUnavailable.code() = "replay_state_unavailable"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gas-replay-state-unavailable</code> | notes | wire code returned by ServiceError::code (services/interop/crates/layerx-gas-station/src/service.rs:127) |

## err-gas-signing-outcome-unknown

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gas-signing-outcome-unknown</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:145</code> | <code>ServiceError::SigningUnknown.code() = "signing_outcome_unknown"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gas-signing-outcome-unknown</code> | notes | wire code returned by ServiceError::code (services/interop/crates/layerx-gas-station/src/service.rs:127) |

## err-gas-stale-rate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gas-stale-rate</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:144</code> | <code>ServiceError::StaleRate.code() = "stale_rate"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gas-stale-rate</code> | notes | wire code returned by ServiceError::code (services/interop/crates/layerx-gas-station/src/service.rs:127) |

## err-gas-timeout

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gas-timeout</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:132</code> | <code>ServiceError::Timeout.code() = "timeout"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gas-timeout</code> | notes | wire code returned by ServiceError::code (services/interop/crates/layerx-gas-station/src/service.rs:127); 194 reference lines in 36 files; callers lists the first reference in each file |

## err-gas-too-large

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gas-too-large</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:135</code> | <code>ServiceError::TooLarge.code() = "too_large"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gas-too-large</code> | notes | wire code returned by ServiceError::code (services/interop/crates/layerx-gas-station/src/service.rs:127) |

## err-gas-unavailable

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-gas-unavailable</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:142</code> | <code>ServiceError::Unavailable.code() = "unavailable"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-gas-unavailable</code> | notes | wire code returned by ServiceError::code (services/interop/crates/layerx-gas-station/src/service.rs:127); 220 reference lines in 91 files; callers lists the first reference in each file |

## fmt-gas-admission

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gas-admission</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:164</code> | <code>Admission { identity: Word, request_digest: Word, batch_nonce: Word, interval: u64 }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gas-admission</code> | notes | gas station journal JSONL via serde Clone, Copy, Debug, Deserialize, Serialize, Eq, PartialEq, deny_unknown_fields |

## fmt-gas-completion

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gas-completion</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:121</code> | <code>Completion = consumed &#124; included &#124; reverted &#124; cancelled (tag outcome)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gas-completion</code> | notes | gas station journal JSONL via serde Clone, Copy, Debug, Deserialize, Serialize, Eq, PartialEq, deny_unknown_fields, rename_all snake_case; 32 reference lines in 5 files; callers lists the first reference in each file |

## fmt-gas-entry

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gas-entry</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:186</code> | <code>Entry = quote_reserved &#124; signing_intent &#124; quote_admitted &#124; liability_released &#124; quoted &#124; prepared &#124; completed &#124; refused &#124; receipt_observed &#124; rate_published &#124; rate_settled &#124; rate_prepared &#124; rate_broadcast &#124; rate_replaced &#124; rate_finalized (tag kind)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gas-entry</code> | notes | gas station journal JSONL via serde Clone, Debug, Deserialize, Serialize, Eq, PartialEq, deny_unknown_fields, rename_all snake_case; 46 reference lines in 6 files; callers lists the first reference in each file |

## fmt-gas-fees

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gas-fees</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/tx.rs:38</code> | <code>Fees { gas_limit: u64, max_fee_per_gas: u128, max_priority_fee_per_gas: u128 }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gas-fees</code> | notes | gas station journal JSONL via serde Clone, Copy, Debug, Deserialize, Serialize, Eq, PartialEq, deny_unknown_fields; 31 reference lines in 8 files; callers lists the first reference in each file |

## fmt-gas-http-reason-phrases

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gas-http-reason-phrases</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:1120</code> | <code>fn reason(status: u16) -&gt; &amp;'static str: OK &#124; Forbidden &#124; Conflict</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gas-http-reason-phrases</code> | notes | HTTP/1.1 status line reason phrases written by the hand-rolled HTTP server |

## fmt-gas-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gas-key</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:36</code> | <code>Key { sponsor: Address, quote_nonce: Word }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gas-key</code> | notes | gas station journal JSONL via serde Clone, Copy, Debug, Deserialize, Serialize, Eq, PartialEq, Ord, PartialOrd, deny_unknown_fields; 40 reference lines in 6 files; callers lists the first reference in each file |

## fmt-gas-liability-release

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gas-liability-release</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:139</code> | <code>LiabilityRelease = expired &#124; settled &#124; refused &#124; consumed (tag reason)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gas-liability-release</code> | notes | gas station journal JSONL via serde Clone, Debug, Deserialize, Serialize, Eq, PartialEq, deny_unknown_fields, rename_all snake_case |

## fmt-gas-publication

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gas-publication</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:269</code> | <code>Publication { owner: Address, nonce: u64, hash: Word, rate: Word, gas_limit: u64, max_fee_per_gas: u128, signed_at: u64 }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gas-publication</code> | notes | gas station journal JSONL via serde Clone, Copy, Debug, Deserialize, Serialize, Eq, PartialEq, deny_unknown_fields |

## fmt-gas-publication-transaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gas-publication-transaction</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:253</code> | <code>PublicationTransaction { version: u16, publication: Publication, chain_id: u64, paymaster: Address, max_priority_fee_per_gas: u128, raw: Vec&lt;u8&gt;, cancellation: bool }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gas-publication-transaction</code> | notes | gas station journal JSONL via serde Clone, Debug, Deserialize, Serialize, Eq, PartialEq, deny_unknown_fields |

## fmt-gas-quote-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gas-quote-record</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:43</code> | <code>QuoteRecord { key: Key, did: String, batch_nonce: Word, chain_id: u64, paymaster: Address, account: Address, token: Address, maximum: Word, amount: Word, deadline: u64, gas_cost: Word, issued_at: u64, signature: Vec&lt;u8&gt;, fees: Fees }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gas-quote-record</code> | notes | gas station journal JSONL via serde Clone, Debug, Deserialize, Serialize, Eq, PartialEq, deny_unknown_fields |

## fmt-gas-rate-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gas-rate-file</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:92</code> | <code>rate file: lines name=integer for rate, set_at, not_after; blank and # lines ignored; duplicates or other names refused (RateFileMalformed)</code> |

## fmt-gas-settlement

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gas-settlement</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:283</code> | <code>Settlement { block_number: u64, gas_used: u64, cost_wei: u128, succeeded: bool }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gas-settlement</code> | notes | gas station journal JSONL via serde Clone, Copy, Debug, Deserialize, Serialize, Eq, PartialEq, deny_unknown_fields |

## fmt-gas-submission

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-gas-submission</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:111</code> | <code>Submission { did: String, batch_nonce: Word, nonce: u64, hash: Word, raw: Vec&lt;u8&gt; }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-gas-submission</code> | notes | gas station journal JSONL via serde Clone, Debug, Deserialize, Serialize, Eq, PartialEq, deny_unknown_fields |

## gas-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-address</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/quote.rs:3</code> | <code>pub type Address = [u8; 20]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-address</code> | notes | type alias layerx_gas_station::quote::Address; 118 reference lines in 16 files; callers lists the first reference in each file |

## gas-address-word

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-address-word</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/quote.rs:29</code> | <code>pub fn address_word(address: Address) -&gt; Word</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-address-word</code> | notes | fn layerx_gas_station::quote::address_word |

## gas-admission

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-admission</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:164</code> | <code>pub struct Admission</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-admission</code> | notes | struct layerx_gas_station::journal::Admission |

## gas-admission-identity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-admission-identity</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:172</code> | <code>pub fn identity(account: Address, nonce: Word, interval: u64) -&gt; Word</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-admission-identity</code> | notes | method layerx_gas_station::journal::Admission::identity |

## gas-authorization

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-authorization</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/tx.rs:29</code> | <code>pub struct Authorization</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-authorization</code> | notes | struct layerx_gas_station::tx::Authorization |

## gas-authorization-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-authorization-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/tx.rs:156</code> | <code>pub fn authorization_digest(auth: &amp;Authorization) -&gt; Result&lt;Word, TxError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-authorization-digest</code> | notes | fn layerx_gas_station::tx::authorization_digest |

## gas-batch-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-batch-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/tx.rs:211</code> | <code>pub fn batch_digest(chain_id: u64, account: Address, batch_nonce: Word, calls: &amp;[Call], quote: &amp;Quote) -&gt; Result&lt;Word, TxError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-batch-digest</code> | notes | fn layerx_gas_station::tx::batch_digest |

## gas-budget-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-budget-key</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/policy.rs:71</code> | <code>pub struct BudgetKey(String)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-budget-key</code> | notes | struct layerx_gas_station::policy::BudgetKey |

## gas-budget-key-account

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-budget-key-account</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/policy.rs:85</code> | <code>pub fn account(account: Address) -&gt; Result&lt;Self, PolicyRefusal&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-budget-key-account</code> | notes | method layerx_gas_station::policy::BudgetKey::account |

## gas-budget-key-as-str

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-budget-key-as-str</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/policy.rs:90</code> | <code>pub fn as_str(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-budget-key-as-str</code> | notes | method layerx_gas_station::policy::BudgetKey::as_str; no reference outside the defining file found by name match |

## gas-budget-key-parse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-budget-key-parse</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/policy.rs:75</code> | <code>pub fn parse(value: &amp;str) -&gt; Result&lt;Self, PolicyRefusal&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-budget-key-parse</code> | notes | method layerx_gas_station::policy::BudgetKey::parse |

## gas-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-bytes</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rpc.rs:119</code> | <code>pub fn bytes(text: &amp;str) -&gt; Result&lt;Vec&lt;u8&gt;, RpcFault&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-bytes</code> | notes | fn layerx_gas_station::rpc::bytes; 115 reference lines in 16 files; callers lists the first reference in each file |

## gas-call

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-call</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/tx.rs:22</code> | <code>pub struct Call</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-call</code> | notes | struct layerx_gas_station::tx::Call |

## gas-cancellation-gas

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-cancellation-gas</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/tx.rs:108</code> | <code>pub const CANCELLATION_GAS: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-cancellation-gas</code> | notes | const layerx_gas_station::tx::CANCELLATION_GAS |

## gas-completion

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-completion</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:121</code> | <code>pub enum Completion</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-completion</code> | notes | enum layerx_gas_station::journal::Completion |

## gas-config-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-config-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:9</code> | <code>pub struct ConfigError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-config-error</code> | notes | struct layerx_gas_station::config::ConfigError |

## gas-configured-rpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-configured-rpc</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rpc.rs:164</code> | <code>pub struct ConfiguredRpc&lt;E&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-configured-rpc</code> | notes | struct layerx_gas_station::rpc::ConfiguredRpc |

## gas-configured-rpc-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-configured-rpc-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rpc.rs:172</code> | <code>pub fn new(config: &amp;StationConfig, exchange: E) -&gt; Result&lt;Self, RpcFault&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-configured-rpc-new</code> | notes | method layerx_gas_station::rpc::ConfiguredRpc::new; chain calls: eth_sendRawTransaction at :227 via private fn send_raw_transaction |

## gas-day-seconds

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-day-seconds</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:25</code> | <code>pub const DAY_SECONDS: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-day-seconds</code> | notes | const layerx_gas_station::rate::DAY_SECONDS |

## gas-deserialize

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-deserialize</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/tx.rs:52</code> | <code>pub fn deserialize&lt;'de, D: Deserializer&lt;'de&gt;&gt;(deserializer: D) -&gt; Result&lt;u128, D::Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-deserialize</code> | notes | fn layerx_gas_station::tx::deserialize; no reference outside the defining file found by name match |

## gas-did-budget

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-did-budget</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-gas-station/src/policy.rs:22</code> | <code>pub struct DidBudget</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-did-budget</code> | notes | struct layerx_gas_station::policy::DidBudget; spec/layerx-merge/spec.kvx task 6.5.41 do_2 reads the 0.12 per-DID daily budget with interval_seconds 86400 |

Contract context <code>contract_context.11.gas-station</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-did-budget-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/policy.rs:29</code> | <code>pub fn new(gas_limit: u64) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-did-budget-new</code> | notes | method layerx_gas_station::policy::DidBudget::new |

## gas-did-budget-used

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-did-budget-used</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/policy.rs:38</code> | <code>pub fn used(&amp;self, did: &amp;str, interval: u64) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-did-budget-used</code> | notes | method layerx_gas_station::policy::DidBudget::used |

## gas-did-gas-limit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-did-gas-limit</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:39</code> | <code>pub struct DidGasLimit(pub u64)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-did-gas-limit</code> | notes | struct layerx_gas_station::config::DidGasLimit |

## gas-drive

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-drive</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:821</code> | <code>pub fn drive&lt;S, R, P, C&gt;(listener: &amp;TcpListener, service: &amp;mut Service&lt;S, R, P, C&gt;, schedule: Schedule, log: &amp;mut impl Write) -&gt; Result&lt;Infallible, ServiceError&gt; where S: QuoteSigner, R: JsonRpc, P: PriceSource, C: FnMut() -&gt; Option&lt;u64&gt;,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-drive</code> | notes | fn layerx_gas_station::service::drive |

## gas-encode-sponsored

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-encode-sponsored</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/tx.rs:236</code> | <code>pub fn encode_sponsored(calls: &amp;[Call], quote: &amp;SignedQuote, account_signature: &amp;[u8; 65]) -&gt; Result&lt;Vec&lt;u8&gt;, TxError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-encode-sponsored</code> | notes | fn layerx_gas_station::tx::encode_sponsored |

## gas-entry

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-entry</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:186</code> | <code>pub enum Entry</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-entry</code> | notes | enum layerx_gas_station::journal::Entry |

## gas-exchange

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-exchange</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rpc.rs:144</code> | <code>pub trait Exchange</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-exchange</code> | notes | trait layerx_gas_station::rpc::Exchange |

## gas-execution

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-execution</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rpc.rs:247</code> | <code>pub struct Execution</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-execution</code> | notes | struct layerx_gas_station::rpc::Execution |

## gas-fees

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-fees</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/tx.rs:38</code> | <code>pub struct Fees</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-fees</code> | notes | struct layerx_gas_station::tx::Fees |

## gas-fees-gas-cost

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-fees-gas-cost</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/tx.rs:67</code> | <code>pub fn gas_cost(self) -&gt; Result&lt;u128, TxError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-fees-gas-cost</code> | notes | method layerx_gas_station::tx::Fees::gas_cost |

## gas-fees-replacement

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-fees-replacement</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/tx.rs:82</code> | <code>pub fn replacement(self) -&gt; Result&lt;Self, TxError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-fees-replacement</code> | notes | method layerx_gas_station::tx::Fees::replacement; no reference outside the defining file found by name match |

## gas-fees-replaces

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-fees-replaces</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/tx.rs:99</code> | <code>pub fn replaces(self, original: Self) -&gt; Result&lt;bool, TxError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-fees-replaces</code> | notes | method layerx_gas_station::tx::Fees::replaces; no reference outside the defining file found by name match |

## gas-gas-station

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-gas-station</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/station.rs:190</code> | <code>pub struct GasStation&lt;S, R, P&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-gas-station</code> | notes | struct layerx_gas_station::station::GasStation; re-exported at services/interop/crates/layerx-gas-station/src/lib.rs:78 |

## gas-gas-station-admission

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-admission</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/station.rs:468</code> | <code>pub fn admission(&amp;self, identity: Word, digest: Word, account: Address, interval: u64) -&gt; Result&lt;Option&lt;Key&gt;, StationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-gas-station-admission</code> | notes | method layerx_gas_station::station::GasStation::admission; chain calls: eth_getBlockByNumber at :504 via private fn anchored_block, eth_getCode at :513 via private fn account_code, eth_getCode at :552 via private fn replay_observation, eth_call at :569 via private fn replay_observation |

## gas-gas-station-batch-nonce

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-batch-nonce</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/station.rs:586</code> | <code>pub fn batch_nonce(&amp;self, account: Address) -&gt; Result&lt;Word, StationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-gas-station-batch-nonce</code> | notes | method layerx_gas_station::station::GasStation::batch_nonce; chain calls: eth_getTransactionReceipt at :606 via private fn receipt, eth_getBlockByNumber at :613 via private fn receipt, eth_getBlockByNumber at :618 via private fn receipt, eth_call at :663 via private fn budget_key |

## gas-gas-station-chain-time

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-chain-time</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/station.rs:281</code> | <code>pub fn chain_time(&amp;self) -&gt; Result&lt;u64, StationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-gas-station-chain-time</code> | notes | method layerx_gas_station::station::GasStation::chain_time; chain calls: eth_getBlockByNumber at :284, eth_getBalance at :299 via private fn sponsor_floor, eth_getBlockByNumber at :349 via private fn release_consumed, eth_getBlockByNumber at :352 via private fn release_consumed, eth_getCode at :358 via private fn release_consumed, eth_getBlockByNumber at :373 via private fn release_consumed, eth_getBlockByNumber at :375 via private fn release_consumed, eth_getBlockByNumber at :438 via private fn reconcile_liabilities, eth_getBlockByNumber at :441 via private fn reconcile_liabilities |

## gas-gas-station-journal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-journal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/station.rs:276</code> | <code>pub const fn journal(&amp;self) -&gt; &amp;Journal</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-gas-station-journal</code> | notes | method layerx_gas_station::station::GasStation::journal; 54 reference lines in 4 files; callers lists the first reference in each file |

## gas-gas-station-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/station.rs:201</code> | <code>pub fn new(config: StationConfig, signer: S, rpc: R, prices: P, journal: Journal) -&gt; Result&lt;Self, StationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-gas-station-new</code> | notes | method layerx_gas_station::station::GasStation::new; chain calls: eth_chainId at :212 |

## gas-gas-station-quote

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-quote</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/station.rs:693</code> | <code>pub fn quote(&amp;mut self, request: &amp;QuoteRequest, fees: Fees, now: u64) -&gt; Result&lt;QuoteOutcome, StationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-gas-station-quote</code> | notes | method layerx_gas_station::station::GasStation::quote; no reference outside the defining file found by name match |

## gas-gas-station-quote-admitted

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-quote-admitted</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/station.rs:737</code> | <code>pub fn quote_admitted(&amp;mut self, request: &amp;QuoteRequest, fees: Fees, now: u64, admission: Option&lt;Admission&gt;) -&gt; Result&lt;QuoteOutcome, StationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-gas-station-quote-admitted</code> | notes | method layerx_gas_station::station::GasStation::quote_admitted; chain calls: eth_getBalance at :776 |

## gas-gas-station-recover

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-recover</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/station.rs:1066</code> | <code>pub fn recover(&amp;mut self, now: u64, budget: Duration) -&gt; Result&lt;Recovery, StationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-gas-station-recover</code> | notes | method layerx_gas_station::station::GasStation::recover |

## gas-gas-station-resume

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-resume</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/station.rs:977</code> | <code>pub fn resume(&amp;mut self, key: Key, now: u64) -&gt; Result&lt;Progress, StationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-gas-station-resume</code> | notes | method layerx_gas_station::station::GasStation::resume |

## gas-gas-station-retry

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-retry</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/station.rs:1189</code> | <code>pub fn retry(&amp;mut self, key: Key, account: Address, relayer_signature: &amp;[u8; 65], now: u64) -&gt; Result&lt;Progress, StationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-gas-station-retry</code> | notes | method layerx_gas_station::station::GasStation::retry; no reference outside the defining file found by name match |

## gas-gas-station-status

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-status</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/station.rs:1162</code> | <code>pub fn status(&amp;self, key: Key, account: Address, relayer_signature: &amp;[u8; 65]) -&gt; Result&lt;Status, StationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-gas-station-status</code> | notes | method layerx_gas_station::station::GasStation::status; no reference outside the defining file found by name match |

## gas-gas-station-submit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-submit</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/station.rs:840</code> | <code>pub fn submit(&amp;mut self, request: &amp;SubmitRequest, now: u64) -&gt; Result&lt;Progress, StationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-gas-station-submit</code> | notes | method layerx_gas_station::station::GasStation::submit; no reference outside the defining file found by name match |

## gas-gas-station-unresolved

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-gas-station-unresolved</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/station.rs:1025</code> | <code>pub fn unresolved(&amp;self) -&gt; Vec&lt;Key&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-gas-station-unresolved</code> | notes | method layerx_gas_station::station::GasStation::unresolved |

## gas-governed-rate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-governed-rate</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/price.rs:30</code> | <code>pub struct GovernedRate</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-governed-rate</code> | notes | struct layerx_gas_station::price::GovernedRate |

## gas-hex

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-hex</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rpc.rs:107</code> | <code>pub fn hex(bytes: &amp;[u8]) -&gt; String</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-hex</code> | notes | fn layerx_gas_station::rpc::hex; 217 reference lines in 12 files; callers lists the first reference in each file |

## gas-https-exchange

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-https-exchange</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rpc.rs:362</code> | <code>pub struct HttpsExchange</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-https-exchange</code> | notes | struct layerx_gas_station::rpc::HttpsExchange |

## gas-https-exchange-read-response

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-https-exchange-read-response</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rpc.rs:367</code> | <code>pub fn read_response(stream: &amp;mut impl Read, budget: Duration) -&gt; Result&lt;Value, RpcFault&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-https-exchange-read-response</code> | notes | method layerx_gas_station::rpc::HttpsExchange::read_response |

## gas-item

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-item</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:291</code> | <code>pub struct Item</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-item</code> | notes | struct layerx_gas_station::journal::Item |

## gas-journal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-journal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:1066</code> | <code>pub struct Journal</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-journal</code> | notes | struct layerx_gas_station::journal::Journal; 44 reference lines in 9 files; callers lists the first reference in each file |

## gas-journal-append

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-journal-append</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:1128</code> | <code>pub fn append(&amp;mut self, entry: &amp;Entry) -&gt; Result&lt;(), JournalError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-journal-append</code> | notes | method layerx_gas_station::journal::Journal::append |

## gas-journal-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-journal-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:15</code> | <code>pub enum JournalError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-journal-error</code> | notes | enum layerx_gas_station::journal::JournalError; 44 reference lines in 5 files; callers lists the first reference in each file |

## gas-journal-open

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-journal-open</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:1074</code> | <code>pub fn open(path: &amp;Path) -&gt; Result&lt;Self, JournalError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-journal-open</code> | notes | method layerx_gas_station::journal::Journal::open |

## gas-journal-state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-journal-state</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:1123</code> | <code>pub const fn state(&amp;self) -&gt; &amp;State</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-journal-state</code> | notes | method layerx_gas_station::journal::Journal::state; 164 reference lines in 8 files; callers lists the first reference in each file |

## gas-json-rpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-json-rpc</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rpc.rs:43</code> | <code>pub trait JsonRpc</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-json-rpc</code> | notes | trait layerx_gas_station::rpc::JsonRpc; chain calls: eth_getTransactionCount at :66 via private fn pending_nonce, eth_estimateGas at :75 via private fn pending_gas_estimate, pax_sequencerReceipt at :86 via private fn sequencer_receipt |

## gas-keccak

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-keccak</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/quote.rs:36</code> | <code>pub fn keccak(bytes: &amp;[u8]) -&gt; Word</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-keccak</code> | notes | fn layerx_gas_station::quote::keccak; 92 reference lines in 13 files; callers lists the first reference in each file |

## gas-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-key</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:36</code> | <code>pub struct Key</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-key</code> | notes | struct layerx_gas_station::journal::Key |

## gas-liability-release

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-liability-release</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:139</code> | <code>pub enum LiabilityRelease</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-liability-release</code> | notes | enum layerx_gas_station::journal::LiabilityRelease |

## gas-limits

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-limits</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:206</code> | <code>pub struct Limits</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-limits</code> | notes | struct layerx_gas_station::service::Limits |

## gas-local-signer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-local-signer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/signer.rs:43</code> | <code>pub struct LocalSigner</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-local-signer</code> | notes | struct layerx_gas_station::signer::LocalSigner |

## gas-local-signer-from-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-local-signer-from-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/signer.rs:55</code> | <code>pub fn from_config(config: &amp;StationConfig) -&gt; Result&lt;Self, SignerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-local-signer-from-config</code> | notes | method layerx_gas_station::signer::LocalSigner::from_config |

## gas-local-signer-from-env

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-local-signer-from-env</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/signer.rs:64</code> | <code>pub fn from_env(name: &amp;str) -&gt; Result&lt;Self, SignerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-local-signer-from-env</code> | notes | method layerx_gas_station::signer::LocalSigner::from_env |

## gas-local-signer-sign-authorization

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-local-signer-sign-authorization</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/signer.rs:80</code> | <code>pub fn sign_authorization(&amp;self, chain_id: u64, delegate: Address, nonce: u64) -&gt; Result&lt;SignedAuthorization, SignerError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-local-signer-sign-authorization</code> | notes | method layerx_gas_station::signer::LocalSigner::sign_authorization; no reference outside the defining file found by name match |

## gas-next-wait

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-next-wait</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:893</code> | <code>pub fn next_wait(cadence_seconds: u64, result: &amp;Result&lt;Publication, RateRefusal&gt;) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-next-wait</code> | notes | fn layerx_gas_station::rate::next_wait |

## gas-pax-base-units

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-pax-base-units</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/price.rs:7</code> | <code>pub const PAX_BASE_UNITS: u128</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-pax-base-units</code> | notes | const layerx_gas_station::price::PAX_BASE_UNITS |

## gas-paymaster-rate-source

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-paymaster-rate-source</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/price.rs:41</code> | <code>pub struct PaymasterRateSource&lt;R&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-paymaster-rate-source</code> | notes | struct layerx_gas_station::price::PaymasterRateSource |

## gas-paymaster-rate-source-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-paymaster-rate-source-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/price.rs:47</code> | <code>pub const fn new(rpc: R, paymaster: Address) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-paymaster-rate-source-new</code> | notes | method layerx_gas_station::price::PaymasterRateSource::new; chain calls: eth_call at :54 via private fn call |

## gas-paymaster-required

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-paymaster-required</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:21</code> | <code>pub struct PaymasterRequired(pub Address)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-paymaster-required</code> | notes | struct layerx_gas_station::config::PaymasterRequired |

## gas-policy-refusal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-policy-refusal</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-gas-station/src/policy.rs:6</code> | <code>pub enum PolicyRefusal</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-policy-refusal</code> | notes | enum layerx_gas_station::policy::PolicyRefusal; spec/layerx-merge/spec.kvx task 0.167 adds PolicyRefusal::DidUnavailable for a failed 0x1004 getLayerXDid read (today BalanceFloor) |

Contract context <code>contract_context.11.gas-station</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-prepare-submission</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/station.rs:107</code> | <code>pub fn prepare_submission(journal: &amp;mut Journal, mut request: TransactionRequest&lt;'_&gt;, did: &amp;str, signer: &amp;impl QuoteSigner) -&gt; Result&lt;Submission, StationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-prepare-submission</code> | notes | fn layerx_gas_station::station::prepare_submission |

## gas-price-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-price-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/price.rs:10</code> | <code>pub enum PriceError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-price-error</code> | notes | enum layerx_gas_station::price::PriceError |

## gas-price-source

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-price-source</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/price.rs:35</code> | <code>pub trait PriceSource</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-price-source</code> | notes | trait layerx_gas_station::price::PriceSource |

## gas-pricing

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-pricing</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/price.rs:102</code> | <code>pub struct Pricing</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-pricing</code> | notes | struct layerx_gas_station::price::Pricing |

## gas-pricing-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-pricing-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/price.rs:108</code> | <code>pub fn new(config: &amp;StationConfig) -&gt; Result&lt;Self, ConfigError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-pricing-new</code> | notes | method layerx_gas_station::price::Pricing::new |

## gas-pricing-quote

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-pricing-quote</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/price.rs:117</code> | <code>pub fn quote(&amp;self, rate: &amp;GovernedRate, gas_cost: u128, now: u64) -&gt; Result&lt;u128, PriceError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-pricing-quote</code> | notes | method layerx_gas_station::price::Pricing::quote; no reference outside the defining file found by name match |

## gas-pricing-validate-amount

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-pricing-validate-amount</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/price.rs:131</code> | <code>pub fn validate_amount(&amp;self, rate: &amp;GovernedRate, gas_cost: u128, amount: u128, now: u64) -&gt; Result&lt;(), PriceError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-pricing-validate-amount</code> | notes | method layerx_gas_station::price::Pricing::validate_amount; no reference outside the defining file found by name match |

## gas-progress

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-progress</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/station.rs:60</code> | <code>pub enum Progress</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-progress</code> | notes | enum layerx_gas_station::station::Progress |

## gas-publication

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-publication</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:269</code> | <code>pub struct Publication</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-publication</code> | notes | struct layerx_gas_station::journal::Publication |

## gas-publication-transaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-publication-transaction</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:253</code> | <code>pub struct PublicationTransaction</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-publication-transaction</code> | notes | struct layerx_gas_station::journal::PublicationTransaction |

## gas-publisher-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-publisher-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:182</code> | <code>pub struct PublisherConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-publisher-config</code> | notes | struct layerx_gas_station::rate::PublisherConfig |

## gas-publisher-config-daily-wei-ceiling

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-publisher-config-daily-wei-ceiling</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:332</code> | <code>pub fn daily_wei_ceiling(&amp;self) -&gt; Option&lt;u128&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-publisher-config-daily-wei-ceiling</code> | notes | method layerx_gas_station::rate::PublisherConfig::daily_wei_ceiling |

## gas-publisher-config-load

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-publisher-config-load</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:223</code> | <code>pub fn load(path: &amp;Path) -&gt; Result&lt;Self, ConfigError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-publisher-config-load</code> | notes | method layerx_gas_station::rate::PublisherConfig::load |

## gas-publisher-config-parse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-publisher-config-parse</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:196</code> | <code>pub fn parse(text: &amp;str) -&gt; Result&lt;Self, ConfigError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-publisher-config-parse</code> | notes | method layerx_gas_station::rate::PublisherConfig::parse |

## gas-publisher-config-validate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-publisher-config-validate</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:238</code> | <code>pub fn validate(&amp;self) -&gt; Result&lt;(), ConfigError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-publisher-config-validate</code> | notes | method layerx_gas_station::rate::PublisherConfig::validate; no reference outside the defining file found by name match |

## gas-quantity

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quantity</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rpc.rs:132</code> | <code>pub fn quantity(text: &amp;str) -&gt; Result&lt;u128, RpcFault&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quantity</code> | notes | fn layerx_gas_station::rpc::quantity |

## gas-quote

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quote</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/quote.rs:11</code> | <code>pub struct Quote</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quote</code> | notes | struct layerx_gas_station::quote::Quote |

## gas-quote-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quote-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/quote.rs:41</code> | <code>pub fn quote_digest(chain_id: Word, account: Address, quote: &amp;Quote) -&gt; Word</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quote-digest</code> | notes | fn layerx_gas_station::quote::quote_digest |

## gas-quote-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quote-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/lib.rs:93</code> | <code>pub enum QuoteError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quote-error</code> | notes | enum layerx_gas_station::QuoteError |

## gas-quote-outcome

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quote-outcome</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/station.rs:55</code> | <code>pub enum QuoteOutcome</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quote-outcome</code> | notes | enum layerx_gas_station::station::QuoteOutcome |

## gas-quote-policy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/policy.rs:116</code> | <code>pub struct QuotePolicy</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quote-policy</code> | notes | struct layerx_gas_station::policy::QuotePolicy |

## gas-quote-policy-did-budget

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy-did-budget</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/policy.rs:150</code> | <code>pub const fn did_budget(&amp;self) -&gt; &amp;DidBudget</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quote-policy-did-budget</code> | notes | method layerx_gas_station::policy::QuotePolicy::did_budget |

## gas-quote-policy-finalized-spend

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy-finalized-spend</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/policy.rs:222</code> | <code>pub const fn finalized_spend(&amp;self) -&gt; u128</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quote-policy-finalized-spend</code> | notes | method layerx_gas_station::policy::QuotePolicy::finalized_spend; no reference outside the defining file found by name match |

## gas-quote-policy-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/policy.rs:129</code> | <code>pub fn new(config: &amp;StationConfig) -&gt; Result&lt;Self, ConfigError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quote-policy-new</code> | notes | method layerx_gas_station::policy::QuotePolicy::new |

## gas-quote-policy-outstanding-pax

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy-outstanding-pax</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/policy.rs:217</code> | <code>pub const fn outstanding_pax(&amp;self) -&gt; u128</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quote-policy-outstanding-pax</code> | notes | method layerx_gas_station::policy::QuotePolicy::outstanding_pax |

## gas-quote-policy-quote-for-budget

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy-quote-for-budget</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-gas-station/src/policy.rs:312</code> | <code>pub fn quote_for_budget&lt;E&gt;(&amp;mut self, request: &amp;crate::QuoteRequest, rate: &amp;crate::price::GovernedRate, fees: crate::tx::Fees, signer: &amp;impl crate::signer::QuoteSigner, budget: Result&lt;&amp;BudgetKey, E&gt;, context: (u128, u64)) -&gt; Result&lt;crate::SignedQuote, crate::QuoteError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quote-policy-quote-for-budget</code> | notes | method layerx_gas_station::policy::QuotePolicy::quote_for_budget; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 0.167 returns DidUnavailable here instead of BalanceFloor |

Contract context <code>contract_context.11.gas-station</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy-quote-for-did</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/policy.rs:285</code> | <code>pub fn quote_for_did(&amp;mut self, request: &amp;crate::QuoteRequest, rate: &amp;crate::price::GovernedRate, fees: crate::tx::Fees, signer: &amp;impl crate::signer::QuoteSigner, did: &amp;str, context: (u128, u64)) -&gt; Result&lt;crate::SignedQuote, crate::QuoteError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quote-policy-quote-for-did</code> | notes | method layerx_gas_station::policy::QuotePolicy::quote_for_did |

## gas-quote-policy-reserve

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy-reserve</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/policy.rs:380</code> | <code>pub fn reserve(&amp;mut self, account: Address, amount: u128, gas_cost: u128, balance: u128, now: u64) -&gt; Result&lt;(), PolicyRefusal&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quote-policy-reserve</code> | notes | method layerx_gas_station::policy::QuotePolicy::reserve |

## gas-quote-policy-reserve-for-budget

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy-reserve-for-budget</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-gas-station/src/policy.rs:257</code> | <code>pub fn reserve_for_budget&lt;E&gt;(&amp;mut self, budget: Result&lt;&amp;BudgetKey, E&gt;, account: Address, amount: u128, fees: crate::tx::Fees, balance: u128, now: u64) -&gt; Result&lt;(), PolicyRefusal&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quote-policy-reserve-for-budget</code> | notes | method layerx_gas_station::policy::QuotePolicy::reserve_for_budget; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 0.167 returns DidUnavailable here instead of BalanceFloor |

Contract context <code>contract_context.11.gas-station</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy-reserve-for-did</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/policy.rs:229</code> | <code>pub fn reserve_for_did(&amp;mut self, did: &amp;str, account: Address, amount: u128, fees: crate::tx::Fees, balance: u128, now: u64) -&gt; Result&lt;(), PolicyRefusal&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quote-policy-reserve-for-did</code> | notes | method layerx_gas_station::policy::QuotePolicy::reserve_for_did |

## gas-quote-policy-used

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy-used</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/policy.rs:157</code> | <code>pub fn used(&amp;self, key: &amp;str, now: u64) -&gt; u128</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quote-policy-used</code> | notes | method layerx_gas_station::policy::QuotePolicy::used |

## gas-quote-policy-with-did-budget

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quote-policy-with-did-budget</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/policy.rs:144</code> | <code>pub fn with_did_budget(mut self, budget: DidBudget) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quote-policy-with-did-budget</code> | notes | method layerx_gas_station::policy::QuotePolicy::with_did_budget |

## gas-quote-record

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quote-record</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:43</code> | <code>pub struct QuoteRecord</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quote-record</code> | notes | struct layerx_gas_station::journal::QuoteRecord |

## gas-quote-record-budget-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quote-record-budget-key</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:65</code> | <code>pub fn budget_key(&amp;self) -&gt; Result&lt;crate::policy::BudgetKey, JournalError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quote-record-budget-key</code> | notes | method layerx_gas_station::journal::QuoteRecord::budget_key |

## gas-quote-record-signed-quote

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quote-record-signed-quote</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:77</code> | <code>pub fn signed_quote(&amp;self) -&gt; Result&lt;SignedQuote, JournalError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quote-record-signed-quote</code> | notes | method layerx_gas_station::journal::QuoteRecord::signed_quote |

## gas-quote-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quote-request</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/lib.rs:101</code> | <code>pub struct QuoteRequest</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quote-request</code> | notes | struct layerx_gas_station::QuoteRequest |

## gas-quote-signer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quote-signer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/signer.rs:36</code> | <code>pub trait QuoteSigner</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quote-signer</code> | notes | trait layerx_gas_station::signer::QuoteSigner |

## gas-quote-word

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-quote-word</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/quote.rs:22</code> | <code>pub fn word(value: u128) -&gt; Word</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-quote-word</code> | notes | fn layerx_gas_station::quote::word; 172 reference lines in 12 files; callers lists the first reference in each file |

## gas-rate-file

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-rate-file</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:72</code> | <code>pub struct RateFile</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-rate-file</code> | notes | struct layerx_gas_station::rate::RateFile |

## gas-rate-file-load

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-rate-file-load</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:123</code> | <code>pub fn load(path: &amp;Path) -&gt; Result&lt;Self, RateRefusal&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-rate-file-load</code> | notes | method layerx_gas_station::rate::RateFile::load; no reference outside the defining file found by name match |

## gas-rate-file-parse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-rate-file-parse</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:83</code> | <code>pub fn parse(text: &amp;str) -&gt; Result&lt;Self, RateRefusal&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-rate-file-parse</code> | notes | method layerx_gas_station::rate::RateFile::parse |

## gas-rate-gas-limit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-rate-gas-limit</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:24</code> | <code>pub const RATE_GAS_LIMIT: u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-rate-gas-limit</code> | notes | const layerx_gas_station::rate::RATE_GAS_LIMIT |

## gas-rate-publisher

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-rate-publisher</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:406</code> | <code>pub struct RatePublisher&lt;S, R&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-rate-publisher</code> | notes | struct layerx_gas_station::rate::RatePublisher |

## gas-rate-publisher-cadence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-rate-publisher-cadence</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:464</code> | <code>pub const fn cadence(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-rate-publisher-cadence</code> | notes | method layerx_gas_station::rate::RatePublisher::cadence; chain calls: eth_call at :471 via private fn paymaster_word, eth_getBalance at :517 via private fn spend_ceiling |

## gas-rate-publisher-confirm-by-receipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-rate-publisher-confirm-by-receipt</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:742</code> | <code>pub fn confirm_by_receipt(&amp;mut self, transaction: &amp;PublicationTransaction) -&gt; Result&lt;Publication, RateRefusal&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-rate-publisher-confirm-by-receipt</code> | notes | method layerx_gas_station::rate::RatePublisher::confirm_by_receipt; no reference outside the defining file found by name match; chain calls: pax_sequencerReceipt at :755, eth_getTransactionReceipt at :820 via private fn try_finalize, eth_getBlockByNumber at :848 via private fn record, eth_getBlockByNumber at :853 via private fn record, eth_getTransactionCount at :865 via private fn record, eth_getBlockByNumber at :872 via private fn record |

## gas-rate-publisher-journal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-rate-publisher-journal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:460</code> | <code>pub const fn journal(&amp;self) -&gt; &amp;Journal</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-rate-publisher-journal</code> | notes | method layerx_gas_station::rate::RatePublisher::journal |

## gas-rate-publisher-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-rate-publisher-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:416</code> | <code>pub fn new(config: PublisherConfig, signer: S, rpc: R, journal: Journal, poll: Duration) -&gt; Result&lt;Self, RateRefusal&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-rate-publisher-new</code> | notes | method layerx_gas_station::rate::RatePublisher::new; chain calls: eth_chainId at :429 |

## gas-rate-publisher-publish

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-rate-publisher-publish</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:544</code> | <code>pub fn publish(&amp;mut self, rate_file: &amp;Path) -&gt; Result&lt;Publication, RateRefusal&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-rate-publisher-publish</code> | notes | method layerx_gas_station::rate::RatePublisher::publish; chain calls: eth_getBlockByNumber at :552, eth_getTransactionCount at :576 |

## gas-rate-publisher-recover-pending

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-rate-publisher-recover-pending</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:633</code> | <code>pub fn recover_pending(&amp;mut self) -&gt; Result&lt;(), RateRefusal&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-rate-publisher-recover-pending</code> | notes | method layerx_gas_station::rate::RatePublisher::recover_pending; no reference outside the defining file found by name match; chain calls: eth_getTransactionCount at :676, pax_sequencerReceipt at :684, eth_sendRawTransaction at :698 via private fn broadcast, pax_sequencerReceipt at :717 via private fn broadcast |

## gas-rate-refusal

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-rate-refusal</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rate.rs:28</code> | <code>pub enum RateRefusal</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-rate-refusal</code> | notes | enum layerx_gas_station::rate::RateRefusal |

## gas-read

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-read</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rpc.rs:98</code> | <code>pub fn read&lt;T: DeserializeOwned&gt;(rpc: &amp;impl JsonRpc, method: &amp;str, params: Value) -&gt; Result&lt;T, RpcFault&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-read</code> | notes | fn layerx_gas_station::rpc::read; 42 reference lines in 10 files; callers lists the first reference in each file |

## gas-recover

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-recover</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/tx.rs:163</code> | <code>pub fn recover(digest: Word, signature: &amp;[u8; 65]) -&gt; Result&lt;Address, TxError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-recover</code> | notes | fn layerx_gas_station::tx::recover |

## gas-recovery

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-recovery</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/station.rs:1218</code> | <code>pub struct Recovery</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-recovery</code> | notes | struct layerx_gas_station::station::Recovery |

## gas-registered-sender

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-registered-sender</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:239</code> | <code>pub struct RegisteredSender</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-registered-sender</code> | notes | struct layerx_gas_station::config::RegisteredSender |

## gas-replacement-bump-percent

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-replacement-bump-percent</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/tx.rs:109</code> | <code>pub const REPLACEMENT_BUMP_PERCENT: u128</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-replacement-bump-percent</code> | notes | const layerx_gas_station::tx::REPLACEMENT_BUMP_PERCENT; no reference outside the defining file found by name match |

## gas-rpc-fault

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-rpc-fault</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-gas-station/src/rpc.rs:14</code> | <code>pub enum RpcFault</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-rpc-fault</code> | notes | enum layerx_gas_station::rpc::RpcFault; 195 reference lines in 9 files; callers lists the first reference in each file; spec/layerx-merge/spec.kvx task 6.5.41 do_1 extends the JSON-RPC error variant at services/interop/crates/layerx-gas-station/src/rpc.rs:20 from {code} to {code, message, data} |

Contract context <code>contract_context.11.gas-station</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-rung</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-gas-station/src/rpc.rs:238</code> | <code>pub enum Rung</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-rung</code> | notes | enum layerx_gas_station::rpc::Rung; spec/layerx-merge/spec.kvx task 6.5.41 do_1 adds a full pax_sequencerReceipt decoder (logs, canonical receipt bytes, keyId) |

Contract context <code>contract_context.11.gas-station</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-schedule</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:800</code> | <code>pub struct Schedule</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-schedule</code> | notes | struct layerx_gas_station::service::Schedule |

## gas-send-outcome

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-send-outcome</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rpc.rs:37</code> | <code>pub enum SendOutcome</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-send-outcome</code> | notes | enum layerx_gas_station::rpc::SendOutcome; no reference outside the defining file found by name match |

## gas-sequencer-endpoint

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-sequencer-endpoint</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rpc.rs:263</code> | <code>pub struct SequencerEndpoint</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-sequencer-endpoint</code> | notes | struct layerx_gas_station::rpc::SequencerEndpoint |

## gas-sequencer-endpoint-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-sequencer-endpoint-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/rpc.rs:268</code> | <code>pub fn new(rpc: impl JsonRpc + 'static) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-sequencer-endpoint-new</code> | notes | method layerx_gas_station::rpc::SequencerEndpoint::new |

## gas-sequencer-endpoint-receipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-sequencer-endpoint-receipt</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-gas-station/src/rpc.rs:277</code> | <code>pub fn receipt(&amp;self, hash: &amp;Word) -&gt; Result&lt;Option&lt;SequencerReceipt&gt;, RpcFault&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-sequencer-endpoint-receipt</code> | notes | method layerx_gas_station::rpc::SequencerEndpoint::receipt; spec/layerx-merge/spec.kvx task 6.5.41 do_1 adds a full pax_sequencerReceipt decoder (logs, canonical receipt bytes, keyId) |

Contract context <code>contract_context.11.gas-station</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-sequencer-receipt</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-gas-station/src/rpc.rs:255</code> | <code>pub struct SequencerReceipt</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-sequencer-receipt</code> | notes | struct layerx_gas_station::rpc::SequencerReceipt; spec/layerx-merge/spec.kvx task 6.5.41 do_1 adds a full pax_sequencerReceipt decoder (logs, canonical receipt bytes, keyId) |

Contract context <code>contract_context.11.gas-station</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-serialize</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/tx.rs:48</code> | <code>pub fn serialize&lt;S: Serializer&gt;(value: &amp;u128, serializer: S) -&gt; Result&lt;S::Ok, S::Error&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-serialize</code> | notes | fn layerx_gas_station::tx::serialize; no reference outside the defining file found by name match |

## gas-serve

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-serve</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:767</code> | <code>pub fn serve&lt;S, R, P, C&gt;(listener: &amp;TcpListener, service: &amp;mut Service&lt;S, R, P, C&gt;, log: &amp;mut impl Write) -&gt; Result&lt;Infallible, ServiceError&gt; where S: QuoteSigner, R: JsonRpc, P: PriceSource, C: FnMut() -&gt; Option&lt;u64&gt;,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-serve</code> | notes | fn layerx_gas_station::service::serve |

## gas-service

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-service</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:220</code> | <code>pub struct Service&lt;S, R, P, C&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-service</code> | notes | struct layerx_gas_station::service::Service |

## gas-service-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-service-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:249</code> | <code>pub struct ServiceConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-service-config</code> | notes | struct layerx_gas_station::config::ServiceConfig |

## gas-service-config-account-budget

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-service-config-account-budget</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:335</code> | <code>pub fn account_budget(&amp;self, account: &amp;Address) -&gt; u128</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-service-config-account-budget</code> | notes | method layerx_gas_station::config::ServiceConfig::account_budget |

## gas-service-config-load

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-service-config-load</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:344</code> | <code>pub fn load(path: &amp;Path) -&gt; Result&lt;Self, ConfigError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-service-config-load</code> | notes | method layerx_gas_station::config::ServiceConfig::load |

## gas-service-config-parse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-service-config-parse</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:285</code> | <code>pub fn parse(text: &amp;str) -&gt; Result&lt;Self, ConfigError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-service-config-parse</code> | notes | method layerx_gas_station::config::ServiceConfig::parse |

## gas-service-config-validate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-service-config-validate</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:315</code> | <code>pub fn validate(&amp;self) -&gt; Result&lt;(), ConfigError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-service-config-validate</code> | notes | method layerx_gas_station::config::ServiceConfig::validate; no reference outside the defining file found by name match |

## gas-service-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-service-error</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:86</code> | <code>pub enum ServiceError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-service-error</code> | notes | enum layerx_gas_station::service::ServiceError; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 0.167 maps DidUnavailable to ServiceError::Unavailable (503) |

Contract context <code>contract_context.11.gas-station</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-service-error-status</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:108</code> | <code>pub const fn status(self) -&gt; u16</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-service-error-status</code> | notes | method layerx_gas_station::service::ServiceError::status; no reference outside the defining file found by name match |

## gas-service-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-service-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:244</code> | <code>pub fn new(config: &amp;ServiceConfig, station: GasStation&lt;S, R, P&gt;, clock: C, limits: Limits) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-service-new</code> | notes | method layerx_gas_station::service::Service::new |

## gas-service-recover

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-service-recover</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:582</code> | <code>pub fn recover(&amp;mut self, budget: Duration) -&gt; Result&lt;Recovery, StationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-service-recover</code> | notes | method layerx_gas_station::service::Service::recover |

## gas-service-respond

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-service-respond</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:284</code> | <code>pub fn respond(&amp;mut self, route: &amp;str, body: &amp;[u8]) -&gt; Result&lt;Value, ServiceError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-service-respond</code> | notes | method layerx_gas_station::service::Service::respond; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 6.5.41 do_2/do_5: /submit returns the full receipt and /status adds a per-DID budget |

Contract context <code>contract_context.11.gas-station</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-service-station</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:277</code> | <code>pub const fn station(&amp;self) -&gt; &amp;GasStation&lt;S, R, P&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-service-station</code> | notes | method layerx_gas_station::service::Service::station; no reference outside the defining file found by name match |

## gas-service-with-sequencer

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-service-with-sequencer</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:271</code> | <code>pub fn with_sequencer(mut self, sequencer: SequencerEndpoint) -&gt; Self</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-service-with-sequencer</code> | notes | method layerx_gas_station::service::Service::with_sequencer |

## gas-settlement

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-settlement</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:283</code> | <code>pub struct Settlement</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-settlement</code> | notes | struct layerx_gas_station::journal::Settlement |

## gas-sidiora

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-sidiora</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/quote.rs:5</code> | <code>pub const SIDIORA: Address</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-sidiora</code> | notes | const layerx_gas_station::quote::SIDIORA |

## gas-sign

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-sign</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/tx.rs:281</code> | <code>pub fn sign(request: &amp;TransactionRequest&lt;'_&gt;, signer: &amp;impl QuoteSigner) -&gt; Result&lt;SignedTransaction, TxError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-sign</code> | notes | fn layerx_gas_station::tx::sign |

## gas-sign-call

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-sign-call</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/tx.rs:376</code> | <code>pub fn sign_call(chain_id: u64, nonce: u64, fees: Fees, to: Address, data: &amp;[u8], signer: &amp;impl QuoteSigner) -&gt; Result&lt;SignedTransaction, TxError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-sign-call</code> | notes | fn layerx_gas_station::tx::sign_call |

## gas-sign-cancellation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-sign-cancellation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/tx.rs:361</code> | <code>pub fn sign_cancellation(chain_id: u64, nonce: u64, fees: Fees, signer: &amp;impl QuoteSigner) -&gt; Result&lt;SignedTransaction, TxError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-sign-cancellation</code> | notes | fn layerx_gas_station::tx::sign_cancellation; no reference outside the defining file found by name match |

## gas-signed-quote

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-signed-quote</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/lib.rs:109</code> | <code>pub struct SignedQuote</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-signed-quote</code> | notes | struct layerx_gas_station::SignedQuote |

## gas-signed-transaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-signed-transaction</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/tx.rs:112</code> | <code>pub struct SignedTransaction</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-signed-transaction</code> | notes | struct layerx_gas_station::tx::SignedTransaction; no reference outside the defining file found by name match |

## gas-signer-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-signer-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/signer.rs:7</code> | <code>pub enum SignerError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-signer-error</code> | notes | enum layerx_gas_station::signer::SignerError |

## gas-state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-state</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:298</code> | <code>pub struct State</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-state</code> | notes | struct layerx_gas_station::journal::State; no reference outside the defining file found by name match |

## gas-state-active-pax

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-state-active-pax</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:468</code> | <code>pub fn active_pax(&amp;self) -&gt; Result&lt;u128, JournalError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-state-active-pax</code> | notes | method layerx_gas_station::journal::State::active_pax; no reference outside the defining file found by name match |

## gas-state-did-spend

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-state-did-spend</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:424</code> | <code>pub fn did_spend(&amp;self, did: &amp;str, interval_seconds: u64, interval: u64) -&gt; Result&lt;(u128, u64), JournalError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-state-did-spend</code> | notes | method layerx_gas_station::journal::State::did_spend; no reference outside the defining file found by name match |

## gas-state-finalized-spend

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-state-finalized-spend</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:477</code> | <code>pub fn finalized_spend(&amp;self) -&gt; Result&lt;u128, JournalError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-state-finalized-spend</code> | notes | method layerx_gas_station::journal::State::finalized_spend; no reference outside the defining file found by name match |

## gas-state-holds

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-state-holds</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:387</code> | <code>pub fn holds(&amp;self, sponsor: Address, nonce: u64) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-state-holds</code> | notes | method layerx_gas_station::journal::State::holds; no reference outside the defining file found by name match |

## gas-state-liability

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-state-liability</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:455</code> | <code>pub fn liability(&amp;self, key: Key) -&gt; Result&lt;u128, JournalError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-state-liability</code> | notes | method layerx_gas_station::journal::State::liability; no reference outside the defining file found by name match |

## gas-state-next-nonce

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-state-next-nonce</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:399</code> | <code>pub fn next_nonce(&amp;self, sponsor: Address, pending: u64) -&gt; Option&lt;u64&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-state-next-nonce</code> | notes | method layerx_gas_station::journal::State::next_nonce; no reference outside the defining file found by name match |

## gas-state-publication-gas

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-state-publication-gas</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:315</code> | <code>pub fn publication_gas(&amp;self, day: u64) -&gt; u128</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-state-publication-gas</code> | notes | method layerx_gas_station::journal::State::publication_gas; no reference outside the defining file found by name match |

## gas-state-publication-wei

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-state-publication-wei</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:338</code> | <code>pub fn publication_wei(&amp;self, day: u64) -&gt; u128</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-state-publication-wei</code> | notes | method layerx_gas_station::journal::State::publication_wei; no reference outside the defining file found by name match |

## gas-state-refused

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-state-refused</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:378</code> | <code>pub fn refused(&amp;self, key: &amp;Key) -&gt; bool</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-state-refused</code> | notes | method layerx_gas_station::journal::State::refused; no reference outside the defining file found by name match |

## gas-state-reserved-wei

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-state-reserved-wei</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:364</code> | <code>pub fn reserved_wei(&amp;self) -&gt; u128</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-state-reserved-wei</code> | notes | method layerx_gas_station::journal::State::reserved_wei; no reference outside the defining file found by name match |

## gas-state-unsettled

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-state-unsettled</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:348</code> | <code>pub fn unsettled(&amp;self) -&gt; Vec&lt;Publication&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-state-unsettled</code> | notes | method layerx_gas_station::journal::State::unsettled; no reference outside the defining file found by name match |

## gas-station

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-station</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/lib.rs:115</code> | <code>pub struct Station&lt;S&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-station</code> | notes | struct layerx_gas_station::Station |

## gas-station-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-station-config</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:58</code> | <code>pub struct StationConfig</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-station-config</code> | notes | struct layerx_gas_station::config::StationConfig |

## gas-station-config-load

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-station-config-load</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:168</code> | <code>pub fn load(path: &amp;Path) -&gt; Result&lt;Self, ConfigError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-station-config-load</code> | notes | method layerx_gas_station::config::StationConfig::load; no reference outside the defining file found by name match |

## gas-station-config-parse

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-station-config-parse</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:136</code> | <code>pub fn parse(text: &amp;str) -&gt; Result&lt;Self, ConfigError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-station-config-parse</code> | notes | method layerx_gas_station::config::StationConfig::parse; no reference outside the defining file found by name match |

## gas-station-config-validate

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-station-config-validate</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/config.rs:179</code> | <code>pub fn validate(&amp;self) -&gt; Result&lt;(), ConfigError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-station-config-validate</code> | notes | method layerx_gas_station::config::StationConfig::validate; no reference outside the defining file found by name match |

## gas-station-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-station-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/station.rs:20</code> | <code>pub enum StationError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-station-error</code> | notes | enum layerx_gas_station::station::StationError; 72 reference lines in 6 files; callers lists the first reference in each file |

## gas-station-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-station-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/lib.rs:124</code> | <code>pub fn new(config: StationConfig, signer: S) -&gt; Result&lt;Self, ConfigError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-station-new</code> | notes | method layerx_gas_station::Station::new |

## gas-station-quote

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-station-quote</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/lib.rs:142</code> | <code>pub fn quote(&amp;mut self, request: &amp;QuoteRequest, rate: &amp;GovernedRate, balance: u128, now: u64) -&gt; Result&lt;SignedQuote, QuoteError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-station-quote</code> | notes | method layerx_gas_station::Station::quote; no reference outside the defining file found by name match |

## gas-status

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-status</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/station.rs:1230</code> | <code>pub struct Status</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-status</code> | notes | struct layerx_gas_station::station::Status; no reference outside the defining file found by name match |

## gas-submission

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-submission</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/journal.rs:111</code> | <code>pub struct Submission</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-submission</code> | notes | struct layerx_gas_station::journal::Submission |

## gas-submit-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-submit-request</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/station.rs:74</code> | <code>pub struct SubmitRequest</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-submit-request</code> | notes | struct layerx_gas_station::station::SubmitRequest |

## gas-transaction-request

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-transaction-request</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/tx.rs:267</code> | <code>pub struct TransactionRequest&lt;'a&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-transaction-request</code> | notes | struct layerx_gas_station::tx::TransactionRequest |

## gas-tx-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-tx-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/tx.rs:9</code> | <code>pub enum TxError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-tx-error</code> | notes | enum layerx_gas_station::tx::TxError |

## gas-word

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-word</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/quote.rs:4</code> | <code>pub type Word = [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-word</code> | notes | type alias layerx_gas_station::quote::Word; 105 reference lines in 13 files; callers lists the first reference in each file |

## http-gas-healthz

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gas-healthz</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:695</code> | <code>GET /healthz</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gas-healthz</code> | notes | answers {"status":"ok"}; any other method 405 |

## http-gas-quote

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gas-quote</code> | <code>http_route</code> | <code>change</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:289</code> | <code>POST /quote {account, nonce, calls, maxTokenAmount, gasCost, ...}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gas-quote</code> | notes | gas station HTTP; POST with a strict JSON body (duplicate keys refused, exact field sets); CORS preflight only from browser_origins with content-type; errors {"error":code} with the ServiceError status (services/interop/crates/layerx-gas-station/src/service.rs:108); answers a signed sponsor quote; stale task 0.167 adds PolicyRefusal::DidUnavailable answered 503 unavailable for a failed 0x1004 getLayerXDid read (today BalanceFloor) |

Contract context <code>contract_context.11.gas-station</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gas-retry</code> | <code>http_route</code> | <code>stable</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:292</code> | <code>POST /retry {sponsor, quoteNonce, account, relayerSignature}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gas-retry</code> | notes | gas station HTTP; POST with a strict JSON body (duplicate keys refused, exact field sets); CORS preflight only from browser_origins with content-type; errors {"error":code} with the ServiceError status (services/interop/crates/layerx-gas-station/src/service.rs:108); re-sends a recorded submission and answers the /status report |

## http-gas-status

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gas-status</code> | <code>http_route</code> | <code>change</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:291</code> | <code>POST /status {sponsor, quoteNonce, account, relayerSignature}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gas-status</code> | notes | gas station HTTP; POST with a strict JSON body (duplicate keys refused, exact field sets); CORS preflight only from browser_origins with content-type; errors {"error":code} with the ServiceError status (services/interop/crates/layerx-gas-station/src/service.rs:108); identity fields at services/interop/crates/layerx-gas-station/src/service.rs:588-590; stale task 6.5.41 do_5 adds budget {did, dailyWei, usedWei, remainingWei} |

Contract context <code>contract_context.11.gas-station</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.http-gas-submit</code> | <code>http_route</code> | <code>change</code> | <code>services/interop/crates/layerx-gas-station/src/service.rs:290</code> | <code>POST /submit {call:{to,value,data}, authorization, batch:{chainId, account, nonce, calls, quote:{sponsor, token, maxTokenAmount, tokenAmount, deadline, quoteNonce, gasCost, decimals}}, accountSignature, relayerSignature}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.http-gas-submit</code> | notes | gas station HTTP; POST with a strict JSON body (duplicate keys refused, exact field sets); CORS preflight only from browser_origins with content-type; errors {"error":code} with the ServiceError status (services/interop/crates/layerx-gas-station/src/service.rs:108); fields at services/interop/crates/layerx-gas-station/src/service.rs:1065-1093; sends eth_sendRawTransaction and reads pax_sequencerReceipt; answers transactionHash, rung and seq; stale task 6.5.41 do_2/do_4 return the full receipt (receipt hex, keyId, height, logs) and pass a sequencer refusal through as {code, message, data}, and enforce the per-DID daily cap |

Contract context <code>contract_context.11.gas-station</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-policy-refusal-did-unavailable</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-gas-station/src/policy.rs</code> (planned) | <code>PolicyRefusal::DidUnavailable</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-policy-refusal-did-unavailable</code> | notes | stale task 0.167 do_1/do_2: failed 0x1004 getLayerXDid read, mapped to ServiceError::Unavailable (503) |

Contract context <code>contract_context.11.gas-station</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-gas-sequencer-receipt-decoder</code> | <code>rust_pub_item</code> | <code>new</code> | <code>services/interop/crates/layerx-gas-station/src/rpc.rs</code> (planned) | <code>public decoder for the full pax_sequencerReceipt (logs address/topics/data/logIndex, canonical receipt bytes, keyId)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-gas-sequencer-receipt-decoder</code> | notes | stale task 6.5.41 do_1 |

Contract context <code>contract_context.11.gas-station</code>:

| Field | Frozen rule |
|---|---|
| interface | The gas station funds DID budgets with EVM transactions and confirms them from pax_sequencerReceipt. |
| storage | Budget ledger in the station state directory. |
| errors | Station errors for exhausted budget, transport and unverified receipt. |
| events | none |
| gas | Each funding transaction pays the network gas price. |
| activation | Starts on service launch. |
| invariants | A budget is debited once per funded transaction. |
| pause | none |
