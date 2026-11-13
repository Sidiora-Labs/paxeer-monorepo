<!-- Source: https://docs.paxeer.app/oracle-risk/ -->

# Oracles & Market Risk

Understand how Paxeer X accepts signed price observations, rejects stale or anomalous inputs, and applies market-specific risk limits.

## Prices enter the ledger as signed observations

Paxeer X carries external market data into deterministic execution through signed `ORACLE_PUSH` activities. Execution consumes accepted observations; it does not make an external network request to obtain a price. This gives replay the same ordered market input that original execution used.

An observation identifies a market, observation sequence, unsigned integer price, observation timestamp, and source identifier. The signed transport appends a signature to the observation (see the format below); the signer key itself is checked against the market's permitted set rather than carried in the transport. A market defines the permitted keys, freshness window, minimum and maximum prices, and maximum deviation from its preceding accepted observation.

## Price acceptance pipeline

| Check | Acceptance rule | Refusal |
| --- | --- | --- |
| Canonical encoding | The payload has the exact expected length and re-encodes to identical bytes. | `LXP_ERR_NON_CANONICAL` |
| Authorized signer | The public key belongs to the market permission set and verifies the domain-separated observation signature. | `LXP_ERR_UNAUTHORIZED_ORACLE` |
| Freshness | Observation time is no later than sealed batch time, and age does not exceed maximum staleness. | `LXP_ERR_TIMESTAMP_REGRESSION` or `LXP_ERR_ORACLE_STALE` |
| Price range | The integer price lies between configured minimum and maximum, inclusive. | `LXP_ERR_ORACLE_BOUNDS` |
| Observation ordering | The incoming sequence is strictly greater than the preceding accepted sequence for this market. | `LXP_ERR_ORACLE_SEQUENCE` |
| Deviation | Absolute change does not exceed the prior accepted price multiplied by the deviation limit in basis points. | `LXP_ERR_ORACLE_DEVIATION` |

The deviation allowance is rounded down in integer price units. The first accepted observation has no preceding observation for a relative-deviation comparison, but it must still satisfy authorization, freshness, encoding, and configured bounds.

### Concrete acceptance example

Suppose the last accepted price is 100,000 integer units and the permitted deviation is 200 basis points. The next observation may differ by at most 2,000 units. A price of 101,500 satisfies that deviation check; 103,000 does not. If maximum staleness is 5,000 milliseconds, an observation aged exactly 5,000 milliseconds satisfies the freshness boundary, while 5,001 milliseconds fails it. These are example parameters, not network-wide defaults.

## Canonical observation format

The unsigned canonical observation is 72 bytes (`LX_ORACLE_OBSERVATION_BYTES`). The signed transport is 137 bytes (`LX_ORACLE_TRANSPORT_BYTES`): a 1-byte version (`LX_ORACLE_TRANSPORT_VERSION` = 1), the 72-byte observation, and a 64-byte signature, decoded by `lx_perps_oracle_transport_decode`. The transport carries no public key; activity authority supplies the signer identity, which must belong to the market's permitted set. Integer fields use big-endian encoding; price uses unsigned 128-bit units and timestamps use milliseconds. Verification binds the observation to the `LXP:ORACLE:OBSERVATION:v1` tag under the signature-preimage domain. These formats are defined in `include/layerx/lx_oracle.h` and `src/modules/perps/lx_perps_command.c`.

```
market_id:             32 bytes
observation_sequence:   8 bytes
price:                 16 bytes
observed_at:            8 bytes
source_identifier:      8 bytes
```

Keep the market price scale with every decoded price. A source identifier describes the submitted observation; it does not replace signer authorization or make an unsigned price acceptable.

## Risk limits are market-specific

Initial margin, maintenance margin, liquidation fees, funding caps, permitted oracle keys, and price acceptance bounds are explicit market parameters. A parameter version identifies the configuration revision. Market creation validates nonzero units and intervals, distinct system accounts, sorted unique permitted keys, and coherent margin and price bounds.

Market halt state is an additional control. Risk-taking operations and funding execution enforce halt checks. Treat the halt flag as an operation-specific refusal boundary: an integration should inspect the receipt of each action rather than assume that all account or recovery operations have identical halt behavior.

### What accepted data proves

An accepted observation proves that its canonical payload passed the configured signature and market checks at its place in execution. It does not prove the external source was economically correct, guarantee future prices, or remove liquidity and insurance constraints. Application risk policies can impose stricter freshness or exposure limits than the protocol minimums.

## Integration workflow

1. Load the market identifier, price scale, parameter version, permitted signer set, and risk thresholds.
2. Read the latest accepted observation with its sequence, timestamp, source identifier, and price.
3. Display observation age and market status beside price-sensitive actions.
4. Submit actions using canonical payloads and inspect execution receipts for price, margin, authorization, and halt refusals.
5. On a stale, out-of-range, or deviation refusal, obtain a new accepted observation or reassess the request. Repeatedly submitting the same invalid observation cannot make it valid.

Registered oracle activities persist the latest accepted observation in the committed perps key-value state. The observation retains its sequence, timestamp, source identifier, and signer. Oracle ingestion does not transfer balances; authorized value movements use explicit ledger transfer sets. Keep observation acceptance and financial settlement separate in application accounting.

## Related documentation

- [Perpetual positions, funding, and liquidation](https://docs.paxeer.app/perpetuals)

- [Native Oracle module interfaces](https://docs.paxeer.app/modules/oracle)

- [Module architecture](https://docs.paxeer.app/modules)
