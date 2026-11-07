# Inference peer

Standalone ES module npm package for the Paxeer inference peer. It has its own
lockfile and no build step or npm dependencies. Use Node.js 22 or newer:

```sh
npm --prefix apps/inference-peer install --no-audit --no-fund --ignore-scripts
node --test apps/inference-peer/test/vendored_engine.test.mjs
```

The `engine/` directory vendors [Pooled](https://github.com/Nehanth/pooled) at
commit `45b5ee275a8754a37fce4a6ce174f31e4a216ef4` under the MIT licence.
[LICENSE-ENGINE](LICENSE-ENGINE) preserves the complete upstream copyright and
permission notice; [NOTICE](NOTICE) records the source pin and SHA-256 hashes.

The snapshot preserves upstream paths beneath `engine/`:

| Path | Vendored contents |
| --- | --- |
| `engine/engine/` | Complete WebGPU engine: dense and Qwen hybrid engines, safetensors and quantised GGUF loaders, tokenizer, quantisation, generation loop, worker and storage helpers |
| `engine/engine/wgsl/` | WGSL shaders exported by upstream JavaScript modules |
| `engine/room/`, `engine/harness/` | Transitive helpers used by the upstream generation loop, including sampling, conversation and layer planning |
| `engine/tests/unit/tokenizer_llamacpp.json` | Upstream reference corpus, vocabulary subsets and token IDs generated using llama.cpp |

Every vendored JavaScript file differs only by its first-line origin comment.
The fixture is byte-identical to upstream. Do not format or edit the snapshot;
put local runtime integration in `src/`. To update it, copy the dependency
closure from one upstream commit, refresh the origin comments, NOTICE hashes
and pinned test revision together, and retain its complete MIT notice.

The package manifest, regression tests and integration documentation are our
work, covered by the repository's Apache-2.0 licence. Room networking, chain
identity, browser UI and native CLI integration belong to subsequent tasks.
The copied upstream helpers retain Pooled's wire and model assumptions; the
Paxeer room adapter must live in `src/`.

The package entry point exports upstream's public API. Additional exports
include `@paxeer/inference-peer/dense`, `/qwen35`, `/gguf`, `/tokenizer`,
`/sampling` and `/generate`. For example, the generation-loop module is
`engine/engine/generate.js`, with its original relative imports intact.

Importing the core modules and shaders and running the tokenizer needs no GPU.
Executing inference needs a WebGPU device and compatible model weights, which
are downloaded separately and carry their own licences. Some experimental
Qwen4 modules also require WebGPU globals at import time. The declared Node
test imports the core engines, loaders, generation loop and shader modules,
checks the snapshot hashes, and verifies the tokenizer against upstream's
reference IDs and exact Unicode round trips. It does not qualify GPU inference.
