# Inference peer variables

The Paxeer inference peer in `apps/inference-peer` provides the browser peer page, native room participant, completion proxy and vendored inference engine. The tables cover environment variables, entry-point settings, model manifest configuration and query settings accepted by the vendored helpers; all source paths are relative to `apps/inference-peer`.

Required settings are conditional on the indicated operation. The browser server and native completion proxy are separate commands. Export the environment variables before starting the relevant command; the application does not automatically load `.env.example`. The model manifest file is selected by `join --model-manifest-url`; its rows identify the JSON section and key. Signed deal and enrolment documents, participant manifests, identity files, tensor metadata and saved engine state follow their respective data formats.

## Required

| Name | What it sets | Source default | Secret | Source reader |
| --- | --- | --- | --- | --- |
| `PAXAI_ASK_BEARER` | Host ask credential for native `serve`, required unless `--ask-bearer` is supplied; the flag takes precedence | None | Yes | `src/cli.mjs` (`serveCommand`) |
| `browser / host` | Host signalling URL for joining a room | None | No | `src/page.mjs` (`PeerPage.join`) |
| `browser / manifest-v1` | Canonical participant manifest used for device measurement | None | No | `src/page.mjs` (`PeerPage.measure`) |
| `browser / region` | Two-letter region code for device measurement | None | No | `src/page.mjs` (`PeerPage.measure`) |
| `browser / room` | Room identifier for joining | None | No | `src/page.mjs` (`PeerPage.join`) |
| `join / --deal` | File containing the chain-checked accepted deal, required for execution | None | No | `src/cli.mjs` (`joinCommand`); `src/deal.mjs` (`dealDigest`) |
| `join / --host-key` | Host peer public key, required for execution | None | No | `src/cli.mjs` (`joinCommand`) |
| `join / --manifest` | Native v2 participant manifest file, required to join | None | No | `src/cli.mjs` (`parseCLI`, `joinCommand`) |
| `join / --model-manifest-url` | Model manifest file URL, required for execution | None | No | `src/cli.mjs` (`joinCommand`); `src/deal.mjs` (`loadLayerRange`) |
| `join / host` | Positional host URL | None | No | `src/cli.mjs` (`parseCLI`, `joinCommand`) |
| `join / room` | Positional room identifier | None | No | `src/cli.mjs` (`parseCLI`, `joinCommand`) |
| `model manifest / config / hidden_size` | Dense model width; must match root `hidden_size` | None | No | `src/deal.mjs`; `src/cli.mjs`; `engine/engine/dense.js` |
| `model manifest / config / intermediate_size` | Dense feed-forward width | None | No | `src/cli.mjs`; `engine/engine/dense.js` |
| `model manifest / config / num_attention_heads` | Dense attention head count | None | No | `src/cli.mjs`; `engine/engine/dense.js` |
| `model manifest / config / num_hidden_layers` | Dense layer count; must match root `layer_count` | None | No | `src/deal.mjs`; `src/cli.mjs`; `engine/engine/dense.js` |
| `model manifest / config / num_key_value_heads` | Dense key/value head count | None | No | `src/cli.mjs`; `engine/engine/dense.js` |
| `model manifest / config / rms_norm_eps` | Dense RMS normalization epsilon | None | No | `src/cli.mjs`; `engine/engine/dense.js` |
| `model manifest / config / rope_theta` | Dense rotary embedding base | None | No | `src/cli.mjs`; `engine/engine/dense.js` |
| `model manifest / root / config` | Dense model configuration object | None | No | `src/deal.mjs`; `src/cli.mjs` |
| `model manifest / root / format` | Runtime model format; dense runtimes accept `safetensors-dense-v1` | None | No | `src/deal.mjs`; `src/cli.mjs` |
| `model manifest / root / hidden_size` | Activation width | None | No | `src/deal.mjs` (`loadLayerRange`) |
| `model manifest / root / layer_count` | Total number of layers covered by shards | None | No | `src/deal.mjs` (`loadLayerRange`) |
| `model manifest / root / shards` | Ordered shard descriptors covering every layer exactly once | None | No | `src/deal.mjs` (`loadLayerRange`) |
| `model manifest / root / version` | Manifest schema version; must be `1` | None | No | `src/deal.mjs` (`loadLayerRange`) |
| `model manifest / shards[] / end_layer` | Exclusive end of a shard's layer range | None | No | `src/deal.mjs` (`loadLayerRange`) |
| `model manifest / shards[] / first_layer` | Inclusive start of a shard's layer range | None | No | `src/deal.mjs` (`loadLayerRange`) |
| `model manifest / shards[] / path` | Shard path resolved relative to the manifest, on the same origin | None | No | `src/deal.mjs` (`loadLayerRange`) |
| `model manifest / shards[] / sha256` | Expected shard content digest | None | No | `src/deal.mjs` (`loadLayerRange`) |
| `model manifest / shards[] / size` | Exact shard size in bytes | None | No | `src/deal.mjs` (`loadLayerRange`) |
| `serve / --host` | Host URL receiving forwarded completion requests | None | No | `src/cli.mjs` (`parseCLI`, `serveCommand`) |

## Optional

| Name | What it sets | Source default | Secret | Source reader |
| --- | --- | --- | --- | --- |
| `PORT` | Browser static server port; integer from 1 through 65535 | `4173` | No | `scripts/serve.mjs` |
| `PREFILL_MATH` | Qwen prefill operand precision when imported under Deno with environment permission; accepts `f32`, `f16`, `sgmatrix`; explicit `prefillMath` takes precedence | `f32`; unsupported modes use `f32` | No | `engine/engine/qwen35.js` (`envPrefillMath`, `Qwen35Engine._init`) |
| `browser / probe-url` | Device bandwidth probe URL | Current page origin, path `/probe` | No | `src/page.mjs` (`PeerPage.start`, `PeerPage.measure`) |
| `join / --attestation` | PXVE attestation envelope file checked before enrolment or signalling | Unset; no local attestation check | No | `src/cli.mjs` (`joinCommand`, `checkAttestation`) |
| `join / --enrolment` | Signed enrolment document file relayed before joining | Unset; no enrolment request | No | `src/cli.mjs` (`joinCommand`); `src/identity.mjs` (`PeerIdentity.enrolPeer`) |
| `join / --identity` | Persistent native identity file path; file contents are secret | `~/.paxeer/inference-peer/identity.pkcs8`; created if absent | No (path only) | `src/cli.mjs` (`joinCommand`); `src/identity.mjs` (`nativeKeys`) |
| `join / --model-digest` | Expected model digest when checking an attestation | Accepted deal's `model_digest`; required explicitly if an attestation has no deal | No | `src/cli.mjs` (`joinCommand`) |
| `join / --runtime-digest` | Expected runtime digest when checking an attestation | Accepted deal's `runtime_digest`; required explicitly if an attestation has no deal | No | `src/cli.mjs` (`joinCommand`) |
| `join / --status-port` | Native join status listener port | `8788` | No | `src/cli.mjs` (`parseCLI`, `joinCommand`) |
| `model manifest / config / head_dim` | Dense attention head width | `hidden_size / num_attention_heads` when omitted | No | `src/cli.mjs`; `engine/engine/dense.js` |
| `model manifest / config / hidden_act` | CPU dense activation declaration; accepts `silu` when present | Omitted; CPU uses SiLU | No | `src/cli.mjs` (`createCPURuntime`) |
| `model manifest / config / rope_scaling` | CPU dense rotary scaling declaration; truthy values are rejected | Omitted; no scaling | No | `src/cli.mjs` (`createCPURuntime`) |
| `model manifest / config / vocab_size` | Vocabulary dimension read by the dense GPU engine; the peer runs without an embedding or output head | None; unused by CPU | No | `engine/engine/dense.js` (`DenseEngine._init`) |
| `serve / --ask-bearer` | Host ask credential overriding the environment variable | `PAXAI_ASK_BEARER` | Yes | `src/cli.mjs` (`serveCommand`) |
| `serve / --port` | Native completion proxy listener port | `8787` | No | `src/cli.mjs` (`parseCLI`, `serveCommand`) |
| `status / --port` | Port of the running join status listener | `8788` | No | `src/cli.mjs` (`parseCLI`, `statusCommand`) |
| `vendored query / argmaxwide` | Wide draft argmax in `roomQwen35Options` | Follows `gpusample` | No | `engine/engine/preset.js` |
| `vendored query / attndecode` | Attention decode implementation in `roomQwen35Options` | Engine default `v2` | No | `engine/engine/preset.js`; `engine/engine/qwen35.js` |
| `vendored query / cdnresolve` | CDN URL resolution in `modelLoadOptions`; `0` disables | Enabled | No | `engine/room/models.js` |
| `vendored query / code` | Room code parsed by `codeFromLocation` | Empty; a valid path code takes precedence, then query, then fragment | No | `engine/room/plan.js` |
| `vendored query / diskbatch` | Batched disk expert prompts in `modelLoadOptions`; `0` disables | Enabled | No | `engine/room/models.js` |
| `vendored query / draftchain` | Chained speculative steps in `roomQwen35Options`; `0` disables | Enabled | No | `engine/engine/preset.js` |
| `vendored query / draftvocab` | Draft vocabulary row limit in `roomQwen35Options`; `0` uses full vocabulary | `65536` | No | `engine/engine/preset.js` |
| `vendored query / dvauto` | Automatic full-vocabulary drafting in `roomQwen35Options`; `0` disables | Enabled | No | `engine/engine/preset.js` |
| `vendored query / fuse` | Attention glue, DeltaNet fusion and batched attention in `roomQwen35Options`; `0` disables | Enabled | No | `engine/engine/preset.js` |
| `vendored query / gpusample` | GPU sampling in `roomQwen35Options`; `0` disables | Enabled | No | `engine/engine/preset.js` |
| `vendored query / hostfuse` | Host trunk fusion in `roomQwen35Options`; `0` disables | Enabled | No | `engine/engine/preset.js` |
| `vendored query / kv` | Quantized key/value cache in `roomQwen35Options`; `q8` enables | Unquantized | No | `engine/engine/preset.js` |
| `vendored query / layered` | Layer-wise batched disk prompts in `modelLoadOptions`; `0` disables | Enabled | No | `engine/room/models.js` |
| `vendored query / layerfuse` | Decode layer fusion in `roomQwen35Options`; accepts `0`, `1` or engine flags | Per-adapter engine policy | No | `engine/engine/preset.js`; `engine/engine/qwen35.js` |
| `vendored query / moednrows` | MoE down-projection rows in `roomQwen35Options` | `1` | No | `engine/engine/preset.js` |
| `vendored query / moefuse` | MoE feed-forward fusion in `roomQwen35Options`; `0` disables | Enabled | No | `engine/engine/preset.js` |
| `vendored query / mtpbatch` | Batched draft-cache fill in `roomEngineFlags`; `0` disables | Enabled | No | `engine/engine/preset.js` |
| `vendored query / mtprefill` | Batched draft-cache refill in `roomEngineFlags`; `0` disables | Enabled | No | `engine/engine/preset.js` |
| `vendored query / ngramworker` | N-gram table copy worker in `modelLoadOptions`; `0` disables | Enabled | No | `engine/room/models.js` |
| `vendored query / parload` | Parallel model part loading in `modelLoadOptions`; `0` disables | Enabled | No | `engine/room/models.js` |
| `vendored query / predraft` | Predrafting after verification in `roomEngineFlags`; `0` disables | Enabled | No | `engine/engine/preset.js` |
| `vendored query / prefetch` | Model lookahead in `modelLoadOptions`; supplied values are clamped to nonnegative integers | `null`; live device policy | No | `engine/room/models.js` |
| `vendored query / saveahead` | Reads in flight per expert-copy worker in `modelLoadOptions`; clamped to 1–16 | `3` | No | `engine/room/models.js` |
| `vendored query / savechunk` | Experts per worker read in `modelLoadOptions`; clamped to 1–64 | `8` | No | `engine/room/models.js` |
| `vendored query / saveearly` | Early expert saving in `modelLoadOptions`; `0` waits for the GPU step | Enabled | No | `engine/room/models.js` |
| `vendored query / saveworkers` | Expert-copy worker count in `modelLoadOptions`; clamped to 0–64 | `4` | No | `engine/room/models.js` |
| `vendored query / specfuse` | Fused speculative verification in `roomQwen35Options`; `0` disables | Enabled | No | `engine/engine/preset.js` |
| `vendored query / wcache` | Converted weight cache in `modelLoadOptions`; `0` disables | Enabled | No | `engine/room/models.js` |
| `vendored query / wcacheverify` | Converted weight verification in `modelLoadOptions`; `1` enables | Disabled | No | `engine/room/models.js` |

The vendored query helpers parse strings supplied by their callers. The peer browser page uses the form settings listed above. `PREFILL_MATH` is read by engine source under Deno; Node and browser entry points use engine options instead. `.env.example` lists all three environment names, with the credential empty and non-secret defaults supplied.
