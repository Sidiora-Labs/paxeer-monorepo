# seidroid

`seidroid[bot]` is Sei's AI reviewer and assistant for GitHub. Each subfolder here is a
**seidroid feature** — its base prompts, its driver, and/or the reusable workflow it
publishes. The reusable *workflows* that run these features live in
[`../workflows/`](../workflows/)
(`ai-review.yml`, `ai-assistant.yml`); a caller repo wires them with `uses:` and pins
`uci-ref` to the same ref. See each feature's own README for how to enable it.

| Feature | What it is | Trigger |
|---|---|---|
| [`ai-review/`](ai-review/) | The workflow-driven seidroid[bot] helpers and their base prompts: an automatic three-pass PR review (OpenAI Codex ∥ Cursor → Claude synthesis, posting one PR review + an `AI Review` check) and the conversational `@seidroid` assistant. Prompts: `scout.md`, `review.md`, `assistant.md`. | `pull_request` (review); `@seidroid` mention (assistant) |

`ai-review` checks out the pull request's merge ref in each scout job and supplies
the diff context to the models. Its optional `prebuild-script` runs before the
Codex scout.
