# Source reference: VS Code extension

This part of the [source reference](../SOURCE-REFERENCE.md) lists 36 selected declarations in 4 files of the [Hyperfide workspace](../../../tools/hyperfide). Paths are relative to that workspace. Line numbers and quoted declarations refer to the linked source files. The declaration column shows the first source line, shortened to 200 characters where necessary. The tables include implementation helpers as well as public declarations.

The extension provides language navigation, KVX support, index status, and commands to open the current task or file in the platform app. Its configuration and command names are declared in [package.json](../../../tools/hyperfide/extensions/vscode/package.json).

## extensions/vscode/client.js

[Open source](../../../tools/hyperfide/extensions/vscode/client.js)

| Indexed name     | Kind     | Line | Source declaration                                                         |
| ---------------- | -------- | ---: | -------------------------------------------------------------------------- |
| `LspClient`      | class    |   14 | <code>class LspClient {</code>                                             |
| `constructor`    | method   |   15 | <code>constructor(bin, cwd, log) {</code>                                  |
| `start`          | method   |   29 | <code>async start() {</code>                                               |
| `dispose`        | method   |   72 | <code>dispose() {</code>                                                   |
| `onNotification` | method   |   82 | <code>onNotification(method, fn) { this.handlers.set(method, fn); }</code> |
| `request`        | method   |   84 | <code>request(method, params, timeoutMs = 20000) {</code>                  |
| `notify`         | method   |  101 | <code>notify(method, params) {</code>                                      |
| `tryRequest`     | method   |  107 | <code>async tryRequest(method, params, fallback) {</code>                  |
| `_send`          | method   |  118 | <code>_send(msg) {</code>                                                  |
| `_consume`       | method   |  130 | <code>_consume(chunk) {</code>                                             |
| `_dispatch`      | method   |  149 | <code>_dispatch(msg) {</code>                                              |
| `uriOf`          | function |  169 | <code>function uriOf(fsPath) {</code>                                      |

## extensions/vscode/extension.js

[Open source](../../../tools/hyperfide/extensions/vscode/extension.js)

| Indexed name  | Kind     | Line | Source declaration                                 |
| ------------- | -------- | ---: | -------------------------------------------------- |
| `platformUrl` | function |    8 | <code>function platformUrl(base, context) {</code> |
| `activate`    | function |   17 | <code>async function activate(ctx) {</code>        |
| `open`        | function |   33 | <code>async function open(kind) {</code>           |
| `updateIndex` | function |   55 | <code>async function updateIndex() {</code>        |

## extensions/vscode/kvx.js

[Open source](../../../tools/hyperfide/extensions/vscode/kvx.js)

| Indexed name             | Kind     | Line | Source declaration                                            |
| ------------------------ | -------- | ---: | ------------------------------------------------------------- |
| `KVX`                    | object   |   11 | <code>const KVX = { scheme: 'file', language: 'kvx' };</code> |
| `TASK_KEYS`              | object   |   14 | <code>const TASK_KEYS = {</code>                              |
| `META_KEYS`              | object   |   30 | <code>const META_KEYS = {</code>                              |
| `lineInfo`               | function |   38 | <code>function lineInfo(doc, line) {</code>                   |
| `findSection`            | function |   47 | <code>function findSection(doc, name) {</code>                |
| `register`               | function |   57 | <code>function register(ctx, cgJson, workspaceRoot) {</code>  |
| `provideDefinition`      | method   |   62 | <code>provideDefinition(doc, pos) {</code>                    |
| `provideCompletionItems` | method   |   88 | <code>async provideCompletionItems(doc, pos) {</code>         |
| `provideHover`           | method   |  136 | <code>provideHover(doc, pos) {</code>                         |
| `provideDocumentSymbols` | method   |  152 | <code>provideDocumentSymbols(doc) {</code>                    |

## extensions/vscode/language.js

[Open source](../../../tools/hyperfide/extensions/vscode/language.js)

| Indexed name              | Kind     | Line | Source declaration                                         |
| ------------------------- | -------- | ---: | ---------------------------------------------------------- |
| `toRange`                 | function |   19 | <code>function toRange(r) {</code>                         |
| `toLocation`              | function |   24 | <code>function toLocation(l) {</code>                      |
| `docParams`               | function |   28 | <code>function docParams(doc, pos) {</code>                |
| `register`                | function |   34 | <code>function register(ctx, client, diagnostics) {</code> |
| `provideDefinition`       | method   |   39 | <code>async provideDefinition(doc, pos) {</code>           |
| `provideReferences`       | method   |   46 | <code>async provideReferences(doc, pos) {</code>           |
| `provideHover`            | method   |   53 | <code>async provideHover(doc, pos) {</code>                |
| `provideDocumentSymbols`  | method   |   67 | <code>async provideDocumentSymbols(doc) {</code>           |
| `provideWorkspaceSymbols` | method   |   78 | <code>async provideWorkspaceSymbols(query) {</code>        |
| `provideCodeLenses`       | method   |   88 | <code>async provideCodeLenses(doc) {</code>                |
