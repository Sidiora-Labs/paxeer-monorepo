# Social service runtime

The connected runtime may run with voice off. In that form the runtime
document omits `voice`; voice endpoints remain mounted and answer the service's
unavailable response until a qualified voice binding is supplied.

Connected readiness reports the stable checks `social_runtime`,
`social_voice` (when voice is configured), `social_notification`,
`social_broker`, `social_durable`, and `social_secrets`. A check that is not
configured is not registered.

The `ranking-visibility` authority route returns a complete snapshot for an
empty market set: `pools` is `{}`, `observedAt` records the read time, and
`revision` is the 64-character SHA-256 digest of the empty set.
