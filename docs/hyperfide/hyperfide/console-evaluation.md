# Hyperfide console surfaces

The platform app contains the task, Ask, fleet and leader surfaces. It uses the platform plugins and authenticated authority interfaces described in the [platform guide](platform.md) and [operations guide](operations.md).

The [VS Code extension](editors.md) provides language navigation, KVX editing, index status and commands to open the current task or file in the platform app. Its package is `hyperfide-language-client` under `tools/hyperfide/extensions/vscode`.

## Control read models

The control plane's console component exposes `console.needs`, `console.issues`, `console.sandboxes` and `console.leaders`. Reads use the enrolled caller's authority. The corresponding CLI views are available through `hyperfide console <needs|issues|sandboxes|leaders>` with JSON and watch options.

See the [console component](../../../tools/hyperfide/core/control-plane/src/observability/console.ts) and [platform plugins](../../../tools/hyperfide/plugins).
