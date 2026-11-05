# Attestor policy file

[`attestor-policy.json`](attestor-policy.json) is the shipped version 1 policy.
Its schema and evaluation rules are defined in
[`internal/policy/policy.go`](../attestor/internal/policy/policy.go).
The parser rejects unknown fields, unsupported versions, malformed addresses,
invalid selectors and negative or non-integer cap amounts.

## Document fields

| Field | Meaning |
| --- | --- |
| `version` | Required integer schema version; must be `1`. |
| `defaults` | Required rules used for every signing account, also providing the base for account overrides. Must supply `chain_id`, `kinds` and `rate_per_minute`. |
| `accounts` | Optional object mapping EVM account addresses to rule overrides. Address matching is case insensitive. Accounts without an entry use `defaults`. |

## Rule fields

The same fields are accepted in `defaults` and each `accounts` entry.

| Field | Meaning |
| --- | --- |
| `chain_id` | Positive unsigned 64-bit integer identifying the signing network. Transaction, typed-data domain, batch and authorization chain IDs must match. The shipped value is `125`. |
| `kinds` | Array of permitted request-kind strings. The shipped list is `evm_tx`, `eip712`, `personal_message`, `sponsored_batch`, `eip7702_authorization`, `lx_send_authorization`, `lx_bind` and `lx_grant`. Kernel kinds require their registered inspectors; listing a kind does not install an inspector. An empty array permits no ordinary request kinds. |
| `caps` | Object mapping assets to ordinary spend caps. Asset keys are `native`, an EVM token address, or a lowercase 64-character hex asset ID without `0x`. Token addresses are normalized to lowercase. Each value has the cap fields described below. A positive ordinary spend without a configured bound is refused. |
| `fee_caps` | Asset-to-cap object for the sponsored-fee leg of a sponsored batch paid to a listed `fee_sponsors` address. Fee budgets are separate from ordinary spend budgets and use ledger keys prefixed with `fee:`. A token listed only here has no ordinary transfer or untrusted approval/permit cap. |
| `fee_sponsors` | Array of sponsor addresses whose batch quote fees may use `fee_caps`. A fee to another sponsor, or a fee without a matching fee-cap entry, is treated as ordinary token spend. The shipped array is empty. |
| `rate_per_minute` | Required unsigned 32-bit maximum number of requests in a rolling minute, per account. The shipped limit is `60`; zero refuses all ordinary requests. Requests that reach spend-cap evaluation have already been recorded for rate accounting, even if a later spend check refuses them. |
| `destinations_allow` | Optional array of allowed addresses. When absent there is no allow-list restriction; an empty array blocks every inspected destination. Applies to inspected call targets and counterparties such as recipients and permit spenders. |
| `destinations_deny` | Optional array of refused addresses. Deny entries take precedence over the allow list. An absent or empty array adds no denials. |
| `selectors` | Object mapping destination addresses to arrays of allowed four-byte method selectors, encoded as `0x` plus eight hex digits. Precompile call targets require an entry and an allowed selector. Other call targets are restricted only if listed. An empty array permits no methods on that target. Counterparties still undergo address checks; a listed counterparty does not need a call selector because it is not the call target. A precompile counterparty still requires a `selectors` entry, even if its selector array is empty. |
| `trusted_spenders` | Array of spender addresses allowed bounded ERC-20 approvals and Permit allowances under the token-scope rules below. The shipped entries are `0x0000000000000000000000000000000000001015` and `0x0000000000000000000000000000000000001017`. |
| `authorization_delegates` | Array of permitted EIP-7702 delegation targets, checked for standalone authorizations and authorizations embedded in transactions. An absent or empty array refuses all delegation targets. The shipped array is empty. |

Account overrides inherit omitted (or JSON `null`) fields. Supplied scalar
fields replace the defaults. Supplied arrays replace the entire default array,
including when empty. `caps`, `fee_caps` and `selectors` merge by asset or
destination key, case insensitively for addresses: an override replaces the
whole cap or selector list at that key. An empty map does not remove inherited
entries. A cap override does not inherit individual bounds from that asset's
default cap.

### Cap fields and shipped limits

| Field | Meaning |
| --- | --- |
| `per_transaction` | Optional non-negative decimal integer string limiting the sum spent in one request, including multiple calls in a batch. An equal amount is allowed. |
| `daily` | Optional non-negative decimal integer string limiting cumulative spend in the rolling preceding 24 hours, including this request. An equal total is allowed. |

Amounts use the asset's smallest units, with no decimal point or automatic
token-decimal conversion. An omitted or empty-string bound is unset; `"0"`
is an actual zero limit. An asset entry with neither bound refuses positive
spend with `no_cap`. Zero spends do not consume a cap. Successful capped spends
are returned in the decision and recorded in the spend ledger.

The shipped ordinary caps are:

| Asset | Per request | Rolling 24 hours |
| --- | --- | --- |
| `native` | `1000000000000000000` | `10000000000000000000` |
| `0x85fcd13735f4309833a503ee804ea32395851479` | `1000000000` | `10000000000` |

The shipped fee cap for SID,
`0x21f7b20a555199fa73a238b1a91fd0f549068fee`, is `5000000` per request and
`50000000` per rolling 24 hours. It applies only to eligible sponsored fees;
the shipped empty `fee_sponsors` list enables no sponsor to use it.

### Trusted spender token scope

A `trusted_spenders` approval or Permit can pass for **any token contract**
that names the trusted spender. Trust is attached to the spender address, not
to a particular token or the tokens listed in `caps` or `fee_caps`. The allowance
must be below `2^256-1` and satisfy the other applicable policy checks and any
ordinary token cap.

The current implementation imposes a stricter bound: it refuses every trusted
allowance at or above `2^255` with `unlimited_approval`, including near-maximum
values below `2^256-1`. Exact amounts below `2^255` pass these allowance checks.
The permitted trusted-allowance range is therefore `0 <= amount < 2^255`,
subject to the other checks described above.

When the token has no entry in `caps`, the trusted allowance is allowed without
capped spend: it is absent from the decision's spends and does not consume a
spend budget. For example, under the shipped policy a SID Permit to
`0x0000000000000000000000000000000000001017` for `100000000` is allowed with
no capped spend, although an ordinary SID transfer or Permit to an untrusted
spender is refused with `no_cap`.

An allowance of exactly `2^256-1` (`maxUint256`) to a trusted spender is refused
with `unlimited_approval`, even when the token has no ordinary cap. A Permit
using the boolean `allowed` form maps `true` to that unlimited amount and
`false` to zero. When an ordinary token cap exists, trusted allowances count
against it, except for spender `0x0000000000000000000000000000000000001015`:
its allowance is not counted again because its token pull is counted at use.
Trust does not exempt a later transfer or pull from the applicable spend checks.

## Decision and refusal codes

Successful decisions use `allowed`. Refusals set `Allowed` to false and include
a machine-readable code and a human-readable reason. Policy loading errors are
returned by the loader before evaluation; equivalent invalid in-memory policy
states can produce the policy-related codes below.

| Code | Refusal reason |
| --- | --- |
| `no_policy` | No policy document is loaded. |
| `unknown_version` | The policy version is unsupported. |
| `invalid_policy` | The effective rules are invalid. |
| `missing_field` | A required policy, account, request, view or spend field is missing or invalid. |
| `unknown_kind` | No inspector is registered for the request kind. |
| `kind_not_allowed` | The account's effective `kinds` list does not permit the request. |
| `decode_error` | The request cannot be decoded or contains malformed values. |
| `chain_mismatch` | A transaction, typed-data domain, batch or authorization names another chain. |
| `digest_mismatch` | A claimed signing digest does not match the decoded request. |
| `contract_creation` | An EVM transaction creates a contract rather than calling a destination. |
| `destination_denied` | An inspected destination is on `destinations_deny`. |
| `destination_not_allowed` | An inspected destination is absent from the configured allow list, or a transfer recipient is a precompile. |
| `selector_not_allowed` | A target's method selector is not allowed, or a precompile target has no selector policy. |
| `rate_limited` | The account has reached its rolling one-minute request limit. |
| `no_cap` | A positive spend has no applicable cap with at least one bound. |
| `value_cap` | The request's summed asset amount exceeds its per-transaction bound. |
| `daily_cap` | The request would exceed a rolling 24-hour asset bound. |
| `unlimited_approval` | A trusted spender approval or Permit grants at least `2^255` (including `2^256-1`), or a token-bearing LayerX authority grant has no total allowance bound. |
| `delegate_not_allowed` | An EIP-7702 target is not in `authorization_delegates`. |
| `ledger_error` | The ledger is unavailable or a clock, request-count or spend operation fails. |
| `verification_view` | The isolated operator verification view or its bound verification message is invalid. |
| `verification_isolated` | An operator verification message is carried under another request kind. |

`operator_verification` is a separate built-in ceremony path: it validates only
the imported-key verification message and bypasses ordinary kind, rate and spend
checks. It is not enabled by adding arbitrary message content to `kinds`.

Registered kernel inspectors can also return the following refusal codes. Their
additional kernel policy is separate from this file's `Document` and `Rules`.

| Code | Refusal reason |
| --- | --- |
| `unknown_module` | The activity names an unknown kernel module. |
| `unknown_operation` | The activity names an operation the attestor cannot decode. |
| `module_not_allowed` | The account's kernel policy does not allow the module. |
| `operation_not_allowed` | The account's kernel policy does not allow the operation. |
| `authority_mismatch` | The activity or grant does not name the signing identity or authority. |
| `disclosure_mismatch` | The approved disclosure differs from the decoded activity. |
| `disclosure_missing` | The request lacks an approved disclosure or approval binding. |
| `approval_mismatch` | The approval is not bound to this request's identity, key, session, network, protocol or digest. |
| `approval_expired` | The approval has expired. |
| `outside_validity` | The activity or grant is outside its validity window. |
| `account_not_owned` | A debited account is not held by the signing key. |
| `bind_address_mismatch` | A binding names an address other than the key's own address. |
| `stale_bind_nonce` | A binding's nonce differs from the chain's current nonce. |
| `chain_unavailable` | The chain lookup needed to validate a binding fails. |
| `grant_cap` | A grant or receive preimage exceeds its per-request grant cap. |
