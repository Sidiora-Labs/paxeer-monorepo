# UCI: Unified Continuous Integration

**UCI (Unified Continuous Integration)** is Sei’s **standard engineering guardrail** for CI. It delivers **consistency** across repositories and **maintainability** through shared, versioned workflows and sensible defaults.

* **Consistency:** One pattern for lint/test/build so teams don’t reinvent CI per repo.
* **Maintainability:** Centralized modules reduce duplication and drift, making updates safer and faster.

## Status

🚧 **Active development.**
The current focus is **Go**; additional languages may be added later.

## Principles

* **Consistency first:** Shared templates & defaults are the norm; overrides are opt-in and minimal.
* **Maintainability at scale:** Central updates propagate safely; repos stay aligned.
* **Guardrails, not handcuffs:** Opinionated by default, extensible where it counts.
