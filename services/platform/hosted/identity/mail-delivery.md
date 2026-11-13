# Identity verification delivery

The identity service runs its built-in outbox consumer every five seconds.
The single replica owns delivery and durable acknowledgement. `pending: true`
acknowledges a queued signup, not receipt of mail. `/readyz` checks the identity store, not SMTP delivery. Delivery is an
explicitly operated consumer rather than a dependency of the readiness probe.

The [deployment](deployment.yaml) requires these objects in `layerx-testnet`:

| Object | Key | Contract |
| --- | --- | --- |
| Secret `layerx-identity-smtp` | `url` | `smtps://<encoded-user>:<encoded-password>@<relay-host>:465` |
| ConfigMap `layerx-identity-mail` | `from` | A relay-authorized sender mailbox in `local@domain` form; the domain must contain a dot. |

Percent-encode credential delimiters in the URL. Never commit the resolved URL,
credentials, verification codes or recipient data. Missing objects or keys
prevent container startup; malformed configured SMTP settings fail startup.
Secret and ConfigMap environment changes require restarting the identity pod.

The Service `layerx-identity-smtp` selects only relay pods labeled
`app: layerx-identity-smtp` in `layerx-testnet`. Relay pods require an
authenticated implicit-TLS listener on TCP 465, a certificate matching the
configured relay hostname and trusted by the identity image's native trust store,
and upstream delivery for the configured sender. The identity policy admits
TCP 465 only to those pods, plus DNS on TCP/UDP 53. It grants no direct external SMTP access. Admission to the relay,
its ingress policy, credentials, certificate renewal, upstream egress and
recipient delivery belong to the relay operator. Do not substitute STARTTLS on
587: the production mailer uses implicit TLS for `smtps://`.

The producer and consumer share
`/var/lib/layerx/identity/signup-outbox` on the identity state PVC. Startup
reconciles pending durable delivery records before accepting requests. Each
consumer pass reconciles that state, then sends JSON verification messages;
successful SMTP acceptance is durably recorded before the file is removed.
Errors leave deliveries for retry at the next 5-second pass. Expired deliveries
are retired by reconciliation. Preserve the PVC and store key during recovery;
do not manually remove queue files. An interrupted SMTP acknowledgement can
produce duplicate messages, so SMTP acceptance is not an exactly-once promise.

Monitor `layerx-identity mailer failed:` in
identity logs and queue count/oldest age on the state volume. Page when the
oldest unexpired queued message exceeds 60 seconds or delivery errors persist
for 60 seconds. Inspect metadata only; queue contents contain verification
tokens and email addresses. During a sustained incident, pause new signup at
the provisioning caller, repair relay connectivity/TLS/authentication or sender
authorization, and let the built-in consumer retry. Confirm that the queue
drains; request a fresh signup if the original code expired. Do not restart the
service repeatedly to mask the error or treat a green `/readyz` as delivery proof.

Before enabling signup after deployment or rotation, the operator must send
one signup through the authenticated provisioning caller to `/v1/signup` using
an operator-controlled mailbox and a unique subject. Confirm `pending: true`,
receipt of the verification message from the configured sender, and successful
`/v1/signup/verify` with the delivered code. Confirm that the corresponding
outbox entry disappears after relay acceptance. In a controlled staging check,
temporarily refuse relay acceptance, confirm the error and retained queue entry,
restore the relay, and confirm retry delivery. Retain only redacted outcomes in
private deployment evidence.
