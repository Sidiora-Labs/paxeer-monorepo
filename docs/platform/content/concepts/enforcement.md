# Who enforces what

Paxeer X Network labels capabilities by the layer that enforces them. A protocol limit, daemon policy and service control bind different operations.

## The four layers

| Layer | Who enforces it | What survives |
|---|---|---|
| `protocol` | The LayerX state machine | Everything above it, including a hostile client, a hostile daemon and a hostile gateway |
| `agent-layer` | `layerx-agentd` while it is in the request path | Nothing that bypasses the daemon |
| `service` | A LayerX service process, or middleware you deploy | Callers of that service only |
| `hosted-surface` | The hosted gateway, faucet or dashboard | Traffic arriving through the hosted deployment |

The label identifies the enforcing boundary. Daemon policy applies to requests through the daemon; protocol restrictions apply at the state transition.

## How to read a label

Read each label as a scope of enforcement:

- `protocol`: the transition function enforces the rule. Budget spending is constrained by funds, period allowance and delegated authority.
- `agent-layer`: `layerx-agentd` enforces the rule on requests through it. Daemon capability attenuation belongs here.
- `service`: the service enforces the rule on its callers. Quote-then-commit and the receipt gate in seller middleware belong here.
- `hosted-surface`: the configured deployment enforces the rule on its traffic. Rate limits and API key policy belong here.

## Where this shows up

The "Enforced by" tables identify the boundary for each listed capability. The [enforcement reference](../reference/enforcement.md) groups capabilities by layer.

## Component readiness and payment evidence

The private status endpoint reports `testnet`, `gateway`, `core` and `paxeer` separately. Readiness describes service availability; a payment outcome is determined by its receipt and committed block evidence. A healthy component does not establish a payment outcome.

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Atomic settlement | `protocol` | A payment activity applies completely or not at all. |
| Capability attenuation | `agent-layer` | Child daemon capabilities cannot exceed their parent. |
| Receipt-gated resource release | `service` | Binds requests that reach your service through the middleware. |
| Hosted rate limits | `hosted-surface` | An operational control on the hosted deployment, not a property of the protocol. |
| Honest degradation reporting | `hosted-surface` | Each component's health is reported on its own, so an outage names whose it is. |
