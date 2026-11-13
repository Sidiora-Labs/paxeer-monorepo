# Who enforces what

Paxeer X Network labels capabilities by the layer that enforces them. A protocol limit, daemon policy and service control bind different operations.

## The four layers

| Layer | Who enforces it | What survives |
|---|---|---|
| `protocol` | The LayerX state machine | Everything above it, including a hostile client, a hostile daemon and a hostile gateway |
| `agent-layer` | `layerx-agentd` while it is in the request path | Nothing that bypasses the daemon |
| `service` | A LayerX service process, or middleware you deploy | Callers of that service only |
| `hosted-surface` | The hosted gateway, faucet or dashboard | Traffic arriving through the hosted deployment |

**A lower layer never implies a higher one.** A restriction the daemon enforces is not a protocol guarantee, and describing it as one is the failure mode this labelling exists to prevent.

## How to read a label

Ask what an attacker would have to compromise to defeat the capability.

- A `protocol` capability requires breaking the transition function itself. Spending through a protocol budget is constrained by its funded authority.
- An `agent-layer` capability requires getting to the protocol without going through `layerx-agentd`. Capability attenuation is here.
- A `service` capability requires reaching around the service. Quote-then-commit is here; so is the receipt gate in your own seller middleware, which binds requests arriving through your middleware and nothing else.
- A `hosted-surface` capability requires nothing but a different deployment. Rate limits and API key policy are here.

None of these are worthless. They are just worth different amounts, and you should know which you are buying.

## Where this shows up

Every page on this site ends with an "Enforced by" table. The [enforcement reference](../reference/enforcement.md) collects every capability the documentation states, grouped by layer, and is generated from the same registry the build validates.


## The same honesty when something breaks

Layer labelling is only half of it. When a component is degraded, saying so vaguely destroys the same information the labels exist to preserve: a single red indicator tells you something is wrong and hides whose problem it is.

The private status endpoint reports `testnet`, `gateway`, `core` and `paxeer` separately. Readiness describes service availability; a payment outcome is determined by its receipt and committed block evidence. A healthy component does not establish a payment outcome.

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Atomic settlement | `protocol` | The canonical example: no component above the protocol can produce a half-applied payment. |
| Capability attenuation | `agent-layer` | Real, useful, and defeated by anything that reaches the protocol without the daemon. |
| Receipt-gated resource release | `service` | Binds requests that reach your service through the middleware. |
| Hosted rate limits | `hosted-surface` | An operational control on the hosted deployment, not a property of the protocol. |
| Honest degradation reporting | `hosted-surface` | Each component's health is reported on its own, so an outage names whose it is. |
