# Human control plane

The human control plane is implemented in
`services/human/crates/layerx-human-service`. It is custodial by default.
Passkey assertions are verified in
`services/human/crates/layerx-human-identity-provider/src/assertion.rs`.
Primary keys remain in the protected keystore implemented by
`services/human/crates/layerx-human-kms`.

| Topic | Page |
| --- | --- |
| Web application | [Web app](web-app.md) |
| Authenticated custody credit | [Custody](custody.md) |
| Journeys and Done | [Journeys](journeys.md) |
| Owner registration | [Owner registration](owner-registration.md) |
| Guardian enrolment details | [Owner registration details](owner-registration-details.md) |
| Hosted Human material | [Hosted Human](hosted.md) |
