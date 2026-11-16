# Launchpad private channel material

`meshpki` creates an ECDSA P-256 private authority, dual-use TLS service
certificates, and Ed25519 request-assertion keys using the Go standard library.
Run the command from `sdk/launchpad/shared` with `go run ./meshpki/cmd/meshpki`.

Every action requires `--dir` naming an operator-owned directory outside Git
working copies. Linked directories are resolved and linked worktrees are also
refused. Newly created directories use mode `0700`; private files use `0600`,
public files use `0644`. Existing files, including links, are never replaced.
A collision leaves the existing set intact. Generation actions print nothing;
`describe` prints one public JSON object. Errors go to standard error.

## Actions and files

| Action | Required arguments | Output inside `--dir` |
| --- | --- | --- |
| `authority` | `--lifetime` | `ca.cert.pem`, `ca.key.pem` |
| `issue` | `--service`, `--names`, `--lifetime` | `<service>.cert.pem`, `<service>.key.pem` |
| `assertion` | `--service` | `<service>.assertion.public` and the five private forms below |
| `describe` | `--service` | Public JSON on standard output; no files |

`--service` is a lowercase token of 1–63 letters, digits or hyphens, beginning
with a letter; `ca` is reserved. `--names` is a comma-separated list of 1–64
unique DNS names or IP addresses. DNS names use ordinary labels without
wildcards. The certificate subject identifies the service; the supplied names
become SANs. Include every name used in HTTPS endpoint URLs and every name
checked by private ingress or service-authority policies. A common name alone
does not satisfy TLS name verification. No loopback names are added implicitly.

Certificate lifetimes are explicit Go durations, such as `8760h`. Both have a
minimum of one hour. An authority has a maximum of ten 365-day years
(`87600h`); a service certificate has a maximum of one 365-day year (`8760h`).
Validity begins at issuance, with no backdating. A leaf must fit entirely
within the authority's remaining validity; an excessive lifetime is refused.
The authority cannot issue intermediate authorities. Leaf certificates support
both TLS server and TLS client authentication. TLS private keys are PKCS#8 PEM
and certificates are PEM, loadable with `tls.LoadX509KeyPair`.

Create the authority first, issue the gateway and each of user, metadata,
social and livestream, then create each service's assertion key. API and other
identity peers can be issued in the same directory. `describe` requires a
certificate, its authority certificate and its assertion public key. It reads
no private files. Output filenames are relative to `--dir`; use their mounted
absolute paths in deployment values. Copy certificates without changing bytes
when using the reported file digests.

## Assertion key forms

Every form belongs to the same generated Ed25519 key. Files contain only the
encoded bytes, without a newline. Import private file contents into the
operator's secret provider; a credential reference names a secret, not a file.

| File suffix after `<service>.assertion.` | Bytes and encoding | Reader / destination |
| --- | --- | --- |
| `public` | 32-byte public key, unpadded base64url | Gateway `identityPeers[].publicKey` |
| `rawurl` | 64-byte private key, unpadded base64url | Gateway assertion credential; social bound runtime `identitySigningKeyReference` |
| `std` | 64-byte private key, padded standard base64 | Livestream `IDENTITY_ASSERTION_SIGNING_KEY`; social platform `IDENTITY_ASSERTION_SIGNING_KEY` also admits this form |
| `rawstd` | 64-byte private key, unpadded standard base64 | API `IDENTITY_SIGNING_KEY` |
| `hex` | 64-byte private key, lowercase hex | Metadata `METADATA_ASSERTION_PRIVATE_KEY_HEX` |
| `seed.std` | 32-byte seed, padded standard base64 | User `ACCOUNT_SERVICE_SIGNING_SEED` |

The public JSON also supplies `publicKeyStdBase64` for user
`ACCOUNT_GATEWAY_PUBLIC_KEY`, and `publicKeyHex` for metadata
`GATEWAY_ASSERTION_PUBLIC_KEY_HEX` or `USER_ASSERTION_PUBLIC_KEY_HEX`.
These are encodings of the public key, never of a private key or seed.
Symmetric credentials, endpoint-proof keys, broker credentials and identity
session-token keys are separate from these service request-assertion keys.

## Public values and composer destinations

All digests are 64 lowercase hexadecimal characters without a prefix.
`caSha256` hashes the exact authority PEM file bytes. `certSha256` hashes the
exact service PEM file bytes. `peerCertSha256` hashes the leaf certificate DER,
as presented on a TLS connection. A file digest cannot be substituted for a
peer pin. `names`, `notBefore`, `notAfter` and `service` describe the leaf.

The readers defining these formats are gateway `gatewayBoundBytes`,
`newGatewayConnectedRuntime` and `identityServiceVerifiers`; social
`socialBoundBytes`, its bound-runtime ingress and `platformPrivate`; user
`materialTLS`, `pinnedAccountClient`, `DecodeAccountKey` and account runtime;
metadata private ingress and `metadataPinnedClient`; livestream
`platformPrivate` and `Config.IdentityAuthority`; API `runtimeIdentity`; and
shared auth `DecodeIdentitySigningKey` and the mTLS identity transport.

For `deploy/launchpad/documents/values/gateway.json`:

| Public output | Values field |
| --- | --- |
| `caFile`, `caSha256` | `runtime.caFile`, `runtime.caSha256` |
| Gateway `certFile`, `certSha256` | Both `runtime.clientCertFile` / `runtime.clientCertSha256` and `runtime.serverCertFile` / `runtime.serverCertSha256` |
| Identity server `peerCertSha256` | `runtime.identityServerPeerCertSha256` |
| Each identity peer's service token, `peerCertSha256`, `publicKey` | `runtime.identityPeers[].issuer`, `.peerCertSha256`, `.publicKey` |

`runtime.clientKeyCredential` and `runtime.serverKeyCredential` select
`credentials` entries whose references resolve to the gateway PKCS#8 PEM key.
`runtime.assertionKeyCredential` selects the entry resolving to its `rawurl`
file contents. The composer translates the values' `Sha256` spelling into the
runtime document's `SHA256` spelling. The private identity listener normally
uses the gateway certificate as its server certificate, so identity clients
pin that gateway leaf. The same dual-use leaf can serve both gateway roles;
separate leaves can be issued under distinct service tokens when required.

For `deploy/launchpad/documents/values/social.json`, the fields are top-level:
`caFile` / `caSHA256` take the authority filename and PEM file digest;
`serverCertFile` / `serverCertSHA256` and `clientCertFile` / `clientCertSHA256`
take the social filename and PEM file digest. `serverKeyReference` and
`clientKeyReference` resolve to the social PEM key; `identitySigningKeyReference`
resolves to its `rawurl` assertion key. `identityServerPeerCertSHA256` pins the
identity listener's DER. `gatewayCertSHA256` pins the gateway client's DER and
`gatewayDNS` must be a SAN on that client certificate. Each `authorityPeers`
entry maps a name present in the authority client's SANs to that client's
`peerCertSha256`. The platform `SOCIAL_AUTHORITY_PEERS` variable expresses the
same entries as comma-separated `name=pin` pairs.

User, metadata and API composer modules have no certificate fields in their
service-specific values objects. Livestream's values describe deployment and
providers; its private channel is configured through service variables. Supply
the following variables alongside composed documents, using one file source
or its corresponding PEM source where the configuration admits both:

| Service | Certificate material | Peer pins |
| --- | --- | --- |
| User | `ACCOUNT_TLS_CA_FILE`, `ACCOUNT_TLS_CERT_FILE`, `ACCOUNT_TLS_KEY_FILE`; corresponding `_PEM` forms; broker material uses `ACCOUNT_NATS_CA_FILE`, `ACCOUNT_NATS_CERT_FILE`, `ACCOUNT_NATS_KEY_FILE` or `_PEM` forms | `ACCOUNT_GATEWAY_CERT_SHA256`, `IDENTITY_CERT_SHA256`, `METADATA_CERT_SHA256`, `NOTIFICATION_CERT_SHA256` |
| Metadata | `METADATA_TLS_CLIENT_CA_FILE`, `METADATA_TLS_CERT_FILE`, `METADATA_TLS_KEY_FILE` | `GATEWAY_PEER_CERT_SHA256`, `IDENTITY_PEER_CERT_SHA256`, `USER_PEER_CERT_SHA256`, `MODERATION_PEER_CERT_SHA256` |
| Social platform | `IDENTITY_MTLS_CA_FILE`, `IDENTITY_MTLS_CLIENT_CERT_FILE`, `IDENTITY_MTLS_CLIENT_KEY_FILE`; `SOCIAL_TLS_CLIENT_CA_FILE`, `SOCIAL_TLS_CERT_FILE`, `SOCIAL_TLS_KEY_FILE`; corresponding `_PEM` forms | `IDENTITY_MTLS_PEER_CERT_SHA256`, `GATEWAY_PEER_CERT_SHA256` |
| Livestream | `IDENTITY_MTLS_CA_FILE`, `IDENTITY_MTLS_CLIENT_CERT_FILE`, `IDENTITY_MTLS_CLIENT_KEY_FILE`; `LIVESTREAM_TLS_CLIENT_CA_FILE`, `LIVESTREAM_TLS_CERT_FILE`, `LIVESTREAM_TLS_KEY_FILE` or the latter three's `_PEM` forms | `IDENTITY_MTLS_PEER_CERT_SHA256`, `GATEWAY_PEER_CERT_SHA256` |
| API | `IDENTITY_CA_FILE`, `IDENTITY_CLIENT_CERT_FILE`, `IDENTITY_CLIENT_KEY_FILE` or corresponding `_PEM` forms | `IDENTITY_PEER_CERT_SHA256` |

Each CA variable takes `ca.cert.pem`; each service certificate/key variable
takes that service's PEM pair. Every peer-pin variable takes the peer's
`peerCertSha256`. Public key variables take the peer's public key, while
signing variables take the service's own private form from the assertion table.
The private files remain under operator control; keep the authority key offline
from service deployments and deliver only each service's own private material.
