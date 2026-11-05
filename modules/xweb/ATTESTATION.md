# xweb attestation digest

Web attestors sign one raw digest with secp256k1 ECDSA. The digest is
`keccak256` over the packed 188-byte preimage below: no EIP-191
`"\x19Ethereum Signed Message"` prefix and no EIP-712 domain separator or
struct hashing. Every integer is big-endian and fixed width; the domain is the
ASCII string with no length prefix and no terminator. This is Solidity
`abi.encodePacked` of the listed types.
[`types.Preimage` / `types.Digest`](types/attestation.go) build these bytes,
[`types/attestation_test.go`](types/attestation_test.go) recomputes the compatibility
corpus, and [`types/testdata/preimage-vectors.json`](types/testdata/preimage-vectors.json)
carries the shared signing vectors.

The same layout serves both origins on one network. The kernel runs inside
`paxd` and participates in block proposal. Both origins use the xweb precompile
at `0x0000000000000000000000000000000000001019` and the persisted xweb keeper:

- Origin 1: a contract calls `request`; `fulfil` delegates signature verification
  and result storage to [`Keeper.Fulfil`](keeper/fulfil.go).
- Origin 2: a program emits a `PAXEERX_WEB_REQUEST_V1` event;
  [`Keeper.RequestFromProgram`](keeper/program.go) records the request.
  [`fulfilProgram(bytes observation)`](../../precompiles/xweb/program.go) delegates
  to [`Keeper.FulfilProgram`](keeper/program_fulfil.go), which verifies the
  observation, stores the answer and pays the signers in chain state.

Both paths use the same governance-managed xweb attestor set. A receipt is final
when its block is committed.

For origin 2, the program event admits a pending request; answer enforcement is
[`keeper.Keeper.FulfilProgram`](keeper/program_fulfil.go), reached through the
xweb precompile's `fulfilProgram(bytes)` entry point. Attestors sign the digest
of the 188-byte preimage; submitters encode the answer and signatures with
[`types.EncodeObservation`](types/observation.go), and submit those observation
bytes to the precompile. Consumers read the stored answer through
`getProgramResult(bytes32,uint64)` on the same `0x1019` precompile.

## Preimage

Domain `PAXEERX_WEB_V1`. Preimage length 188 bytes.

| Offset | Size | Field          | Encoding                                                                 |
|-------:|-----:|----------------|--------------------------------------------------------------------------|
|      0 |   14 | domain         | ASCII `PAXEERX_WEB_V1`                                                   |
|     14 |    1 | origin         | uint8, 1 for the EVM precompile, 2 for a kernel program                  |
|     15 |   32 | networkId      | uint256, the EVM chain id for origin 1, the configured kernel network id for origin 2 |
|     47 |   32 | requester      | bytes32, the EVM address left-padded with zeros for origin 1, the 32-byte program id for origin 2 |
|     79 |    8 | requestId      | uint64, the module nonce for origin 1, the program-supplied request id for origin 2 |
|     87 |    1 | kind           | uint8, 1 fetch, 2 search, 3 api, 4 storage receipt; support depends on origin |
|     88 |   32 | payloadHash    | bytes32, keccak256 of the request payload bytes                          |
|    120 |   32 | contentDigest  | bytes32, keccak256 of the canonical content bytes                        |
|    152 |   32 | responseHash   | bytes32, keccak256 of the stored response bytes (at most 4096)           |
|    184 |    4 | fullLength     | uint32, byte length of the full text or answer before response truncation |

`digest = keccak256(preimage)`.

For origin 1 the keeper reads the EVM chain id from chain state; mainnet-beta
uses `125` (`0x7d`). For origin 2 it requires the observation's nonzero uint32
network id to equal `Params.KernelNetworkId` (`kernel_network_id`); the signing
preimage encodes that value in 32 bytes with 28 leading zero bytes. The
precompile's `getKernelNetworkId()` exposes this configured value. These are
origin-specific signing domains within the same network.

[`types.KnownKind` and `types.ProgramKind`](types/state.go) define the request
policy:

| Kind | Request | Origin 1 | Origin 2 |
|-----:|---------|----------|----------|
| 1 | fetch | majority | majority |
| 2 | search | majority | majority |
| 3 | api | majority or single, selected by payload | unsupported |
| 4 | storage receipt | majority | unsupported |

Origin-2 request admission and persisted pending records accept only fetch and
search. Although the [observation codec](types/observation.go) can encode kind 4,
that alone does not provide a program storage-receipt lifecycle.

## Signature rules

Enforced by `Keeper.Fulfil` for origin 1 and `Keeper.FulfilProgram` for origin 2,
using [`layerxbridge/types.RecoverSigner`](../layerxbridge/types/attestation.go)
for signature recovery:

- each signature is 65 bytes `r (32) || s (32) || v (1)` with `v` in {27, 28}
  and `s <= secp256k1n / 2`;
- signatures are ordered by strictly ascending recovered signer address, which
  also rejects a repeated signer;
- every recovered signer must belong to the current registered web attestor
  set; the majority level requires at least its `threshold`, which is a strict
  majority of the set; an empty set accepts nothing;
- the stored response is at most 4096 bytes and `fullLength` is never shorter
  than it;
- a request is fulfilled at most once; origin 1 also rejects refunded requests;
- origin 2 requires an active `v7.0` program-origin upgrade and an unpaused
  module, a canonical observation with 1 to 16 signatures, the configured
  network id, and a pending record matching the program id, request id, kind
  and payload hash. The observation's wire encoding is distinct from the
  188-byte signing preimage.

An api request (kind 3) names its attestation level inside its payload, so the
level is bound through `payloadHash` and the preimage keeps its 188 bytes.
Under the single level a fulfilment carries exactly one signature, which must
recover to the attestor the payload names; the threshold rule above applies to
the majority level. [`types/api.go`](types/api.go) specifies the payload and
[`types/testdata/api-vectors.json`](types/testdata/api-vectors.json) pins it.

A storage-receipt request (kind 4, origin 1) uses
[`types/storage_receipt.go`](types/storage_receipt.go): a nonzero 32-byte object
digest followed by the UTF-8 base URL bytes, with no length prefix. The base URL
must be 1 to 512 bytes, use HTTPS with a host, and contain no userinfo, query,
fragment or trailing slash. `Keeper.Request` validates this payload before
recording its `payloadHash`.

The [x-websearch attestor](../../services/interop/crates/x-websearch/src/attest.rs)
fetches `<baseURL>/manifests/<objectDigest>.json`, with the digest rendered as
64 lowercase hexadecimal digits, and checks SHA-256 of those bytes against the
payload's object digest. It builds receipt text
`pns-receipt/v1\n<hex digest>\n<manifest byte length>\n<baseURL>\n` and stores its
canonical encoding using `ContentKind::Fetch`, the original request payload
and media type `text/plain`, as defined in
[`canonical.rs`](../../services/interop/crates/x-websearch/src/canonical.rs).
The attestation's `contentDigest` is keccak256 of that
canonical encoding; it is distinct from the SHA-256 object digest. On chain,
the keeper verifies the signatures binding the stored request's payload hash,
the supplied content digest, response hash and full length; it does not fetch
the manifest itself.

## Vectors

Origin-1 vectors below are regenerated for Paxeer chain `125`; origin-2
network id `1` is a fixture value. Live signing uses chain state and governed
configuration. The generator is
[`generate_repair_476.py`](../evm/config/generate_repair_476.py).

Vector `evm-fetch` (origin 1):

- networkId `125`
- requester `0x00000000000000000000000000000000000000000000000000000000000a11ce`
- requestId `7`, kind `1`
- payload `https://paxeer.app/`, payloadHash
  `0x62da65e513a2dc07e8c56bb6e148a96d6d259e496ec215d58c89522fa9649ce1`
- contentDigest `0x2222222222222222222222222222222222222222222222222222222222222222`
- response `Paxeer X Network`, responseHash
  `0xe75d956cdf14f4a1e94d2ca208572632a4b4a0cc5323a2de37e86f08c9f0acfd`
- fullLength `16`
- digest `0x215a1830bba1f455bdfbf11d33ea5344e5ea85925d4f3aa2187ef6128aaf3ba0`
- signer `0x9c4ca69112e7f6ee6b77a8c39dc6039556e1ec37`, signature
  `0x3bfc74e9ca2a7f4f3b669d567449538d369ae2a05c34c42b6c46e99ce7395fda4e5e78d7b68843fb7c46dd174b027610d5283c276b99700c76264106611cab801c`

Vector `program-search` (origin 2):

- networkId `1`
- requester `0x5555555555555555555555555555555555555555555555555555555555555555`
- requestId `42`, kind `2`
- payload `paxeer x network`, payloadHash
  `0x637c790bf5fe77888f9696cd67ea9c582f9be57c0b16c9ee11d1cf62ee25c2f8`
- contentDigest `0x3333333333333333333333333333333333333333333333333333333333333333`
- response `[{"url":"https://paxeer.app/","title":"Paxeer","snippet":"Paxeer X Network"}]`,
  responseHash `0x09b4dc1f0fb623c5ffb240fef88176c4706aad87d7a63e622777c029d552fb3a`
- fullLength `5000`
- digest `0xc634942bc32e91ff577e48527c40c951d7f00b0808764b080540b57292343379`
- signer `0x767eb87feba9e1851bd2f5d6fc931d2476b1db90`, signature
  `0x7893009709dc168f1378cc7296bca594594b522e937a17b1948efb399726e78c4532a5ce1715617c5ce302722588935b996f8b8d115093c7475932a081717aa91b`

Vector `evm-api-single` (origin 1):

- networkId `125`
- requester `0x00000000000000000000000000000000000000000000000000000000000a11ce`
- requestId `9`, kind `3`
- payload: the `post-single-credential` api vector, payloadHash
  `0xd6c5bc752f9769fa30ebf29d2e9b8fc6ae47892c6bdffb659612f490d89b7ca5`
- contentDigest `0x4444444444444444444444444444444444444444444444444444444444444444`
- response `["3.114"]`, responseHash
  `0x2c85cb58c805de4b2edb12efa1506788e69c5d32749f03c9e9d402b09013e9ec`
- fullLength `9`
- digest `0x7a1dbe443684ec50ef70a3ad2dbb5b2bc531fe845588667a5fbcdc285f72e24b`
- signer `0xe35182c595c7db5baf6237c02cf6f8177831f384`, signature
  `0x628dbe17c026484d3232ed68434e5b3b5a69aad2468822fb3cb59920f469b0a167f0f8376c7921a19e7a0644b7244c1f206f5fcade8cd095e65f5f32e9da9ff31b`

The shared
[`types/testdata/preimage-vectors.json`](types/testdata/preimage-vectors.json)
still contains the historical origin-1 compatibility corpus. Its legacy
digests and signatures below identify those archived bytes; they are not
chain-125 signing examples. Updating that corpus and its cross-language
consumers requires a separate scope.

- `evm-fetch`: digest `0x21e2a70f243f13f7e2b72d14cc842c5f6b1779b532144b01c1d61d47b886a0be`, signature `0x1e95bb99402e2ae4b06e98eb3ce29a00973ce759afeac27c3d0253ebba3211a7555a8954e3d0d46bce57952e7648f08acbb3f84f2a1741056974a3e96c7961611b`.
- `evm-api-single`: digest `0x18ef556323e81c5d30bff202abbf20910cc82809cf280e08279e658441ac865e`, signature `0x9144452b00ca7e3d2ff7909a87bc504d45409126175bee22aacf06eb64e8470e0ad0adf0f3c1b1c2ebb9887ae5fb5b293bdb0cbea5462f9bc54500064f7bbd9f1c`.
