# Agent Rust crypto and proof API contract

Owner: agent domain (9). This document is included in `layerx_crypto` crate
documentation and maintained with its implementation. Paths below are relative
to `services/agent/crates`. Public network transactions use the root Paxeer
JSON-RPC endpoint, chain id 125, on the single BFT chain.

## Crypto

`binding::BindMessage` is the immutable, exact Ed25519 consent
`LX:PAXEER-BIND:v1 || chain_id_uint256_be || evm_address20 || bind_nonce_uint64_be`.
It is signed **raw**, without a hash or kernel activity wrapper. The bind nonce
comes from `layerXBindNonce(address)` and is independent of the EVM transaction
nonce. `SigningRequest::bind` accepts this typed message. `LocalSigner` and
`KeystoreSigner` sign it; `KeystoreSigner::sign_bind_bytes` validates the entire
bind domain before agent I/O and refuses activity preimages. Each SSH response
is strictly verified against the selected public key and exact requested bytes.
Reopening a signer never broadens its accepted signing purposes.

`BindAuthorization::to_call` verifies DID consent and binds chain and sender
before encoding `bindLayerX(bytes32,bytes)` at addr precompile `0x1004`.
`BindAuthorization::from_call` checks canonical ABI and verifies consent using
the recovered transaction sender and bind nonce supplied from chain state.
The enclosing type-2 transaction supplies the EVM owner's separate signature.
It cannot authorize another sender, chain or bind nonce. The DID signature
alone is not proof that a registration transaction was submitted or committed.

`send::SendDebit::signing_request` accepts only its re-encoded native payer
authorization. This signs the scoped native digest, preserving canonical
payment payload compatibility. It does not authorize an activity envelope.
`SigningDisclosure` distinguishes bind consent, payer authorization and the
deprecated activity purpose. `SigningRequest::new` validates retained activity
disclosures for auditing; every key boundary refuses to sign that purpose before
I/O. Remote protocol v1 supports the scoped payer digest. `BindSigner::sign_bind`
uses the separate v2 bind frame, decoded by `remote::decode_bind_request`, for
raw bind consent; the generic v1 request path refuses bind consent.

`send::ledger_payment_call(payload, CallOptions)` embeds the authenticated
native payment payload in canonical precompile ABI. `disclosure::call::bind_call`
requires an explicit sender and discloses a supported `Eip1559Call`.
`evm_transaction::decode_unsigned` consumes canonical type-2 transaction bytes;
`Disclosure::reencode` and `call::reencode` reproduce those bytes.
`validate_canonical` rejects different bytes and `validate_call` rejects changed
semantics or sender. `signer::sign_disclosed_call` validates every disclosed
field, requires the selected EVM key to match the sender and verifies recovery
of the returned canonical low-s secp256k1 signature over the EIP-1559 hash.
The DID Ed25519 key does not sign EVM transactions. A DID-only keystore reports
`SignError::EvmKeyUnavailable` for transaction signing.

Key storage and derivation remain in `keystore` and `account_derivation`.
Payload codecs remain in `payments`, `authority_grant`, `session` and disclosure
helpers. They describe kernel operations carried inside EVM calls. The obsolete settlement-recipient authorization is removed because it has no
repository caller. Live consumers still require `disclosure::bind`,
`bind_budget_mutation`, `send::EnvelopeOptions`, `DisclosedEnvelope`,
`encode_send_envelope`, and the `onboarding` types/re-exports. These compatibility
APIs retain their validation and are explicitly deprecated; their presence does
not make activity ingress a supported merged-network transaction path. Migrate
them to canonical type-2 calls with an explicit sender. Identity registration
uses DID bind consent inside the EVM registration transaction. There is no
settlement checkpoint or guarantor authorization in this contract.

Failures are explicit: `DisclosureError::FieldMismatch` names a changed semantic
field, malformed/unsupported calldata is refused, and `SignError` distinguishes
invalid disclosure, unavailable/refusing keystore, malformed response and invalid
returned signature. Keys and secrets are not included in error text.

## Proof

Proof verification is owned by `layerx-proof`, especially `receipt`, `level`,
`evidence` and `state`. Crypto signing does not itself establish a finality rung.
The supported rungs are **submitted**, **receipted** and **committed**:

* Submitted identifies a signed EVM transaction by its transaction hash.
* Receipted requires a strictly valid receipt signature under a governed,
  independently pinned receipt public key and its authorized height interval.
  A key advertised by the same untrusted response is not a trust pin. Verify
  the receipt's transaction identity, network and exact immutable receipt bytes.
* Committed requires independently verified evidence for the covering committed
  BFT block and the relevant inclusion/state commitments. A receipt, RPC claim,
  receipt-key pin or local signing success alone cannot establish this rung.

Persist the transaction hash and original receipt bytes before restart; resolve
the same transaction identity after reopening. Never manufacture a new receipt
or silently promote Receipted to Committed. Current proof paths that do not
verify committed block evidence must stop at their proven rung and report the
missing evidence. Retired batch/guarantor and external settlement proofs are
not an alternate route to committed finality.
