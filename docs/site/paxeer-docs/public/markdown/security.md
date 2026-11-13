<!-- Source: https://docs.paxeer.app/security/ -->

# Security and trust boundaries

How transaction signatures, receipt keys, committed blocks, and scoped service credentials establish trust in Paxeer X.

Paxeer mainnet-beta is one BFT network. The kernel executes within the chain, and a transaction is final when its covering block is committed. A signed execution receipt establishes a narrower claim before commitment. Applications verify the evidence needed before releasing goods or updating balances.

## The trust boundaries

| Boundary | Verification responsibility | Remaining dependency |
| --- | --- | --- |
| Signer to node | Signed transaction, chain identifier, nonce, gas, and balance checks. | Key custody and correct transaction construction. |
| Node to client | Canonical receipt, authorized receipt key, raw transaction hash, and execution status. | Receipt-key trust and node availability. |
| Proposer to validators | Sequence-batch signature, sequence continuity, and transaction replay. | Consensus validator set and deterministic execution. |
| Block to application | Transaction inclusion and consensus commit under a trusted validator set. | Authenticated chain and validator-set trust. |
| Application to hosted service | Authenticated request, scoped authorization, and evidence for the resulting operation. | Service availability and credential custody. |

## Evidence levels are consequential

A submission response identifies a transaction. A signed receipt authenticates execution under a pinned receipt key. A covering block and verified consensus commit establish committed inclusion. Applications retain the signed transaction and check its hash, receipt outcome, and block evidence together.

Native balances and EVM execution belong to the same network. Receipt signatures authenticate the signer and payload; consensus commitment establishes finality. Neither service authentication nor a matching chain identifier proves a payment.

## Canonical execution and replay

The node validates the ordered sequence batch and re-executes proposed transactions. Sequence continuity, signatures, and committed roots bind the execution order. Query indexes expose derived views and do not replace consensus evidence.

Program monetary effects pass through protocol transfer sets and the asset module. Storage occupancy is accounted for during deterministic execution. Applications check the resulting execution status and transfer evidence before treating a program call as successful.

## Verify receipt authority

The node exposes signed execution receipts bound to the raw EVM transaction hash. Clients authenticate the signing key against independently trusted key identifiers, public keys, and valid height ranges, then check the canonical receipt and execution outputs.

### Trusted receipt keys

Receipt-key entries come from `px_getReceiptKeys`. Their key identifiers, public keys, and inclusive height ranges describe the governed receipt-signing authority. Establish trust in that key list independently of the receipt response.

1. Start with the exact signed transaction and its hash on the intended network.
2. Retrieve its canonical receipt and authenticate the signing key for the receipt height.
3. Check the signature, transaction binding, execution status, and expected payment or program effect.
4. Authenticate committed block inclusion before treating the transaction as final.

A signature over another transaction does not establish the requested payment. A root or signing key copied from an untrusted response does not authenticate execution or consensus membership.

### Refusals and retry behavior

Authentication failures, unknown transactions, execution refusals, and unavailable dependencies have different meanings. Confirm the identifier when a transaction is unknown, inspect the execution outcome when it is refused, and retain the original transaction when transport failure leaves its outcome uncertain.

A liveness or readiness response reports service availability. It does not certify a transaction, establish receipt-key authority, or prove a block commit.

## Keys, roles, and custody

Owner keys, identity sessions, scoped gateway keys, capabilities, and budgets carry distinct authority. Grant only the operations, assets, amounts, and lifetime the application needs. Check revocation and current authorization before signing.

Validator governance and receipt-key governance serve different verification roles. The `modules/sequencer` module stores the leader list, receipt keys, and committed sequence position. See [consensus and signing roles](https://docs.paxeer.app/guarantors) when selecting the trust inputs for receipt verification.

## Emergency response and upgrade control

Upgrade controls follow the specific chain or contract authority. Contract timelocks restrict permitted calls, role assignments, delays, and execution windows. Applications inspect those deployed parameters rather than inferring authority from a service response.

During an outage, preserve signed transactions, receipts, and block inclusion evidence. Resume lookup using the original transaction hash and establish its outcome before authorizing another economic action.

## Integration review

- Pin the chain, validator set, and receipt-key authority.

- Reject noncanonical encodings and evidence bound to another transaction or sequence.

- Keep retries idempotent and retain the original transaction hash.

- Distinguish submission, authenticated execution, and committed inclusion.

- Inspect current role assignments, thresholds, delays, and limits when evaluating contract authority.

Continue with [verification roles](https://docs.paxeer.app/guarantors), [Governance](https://docs.paxeer.app/governance), and [Finality](https://docs.paxeer.app/finality).
