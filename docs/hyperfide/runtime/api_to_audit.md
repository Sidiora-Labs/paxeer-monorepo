# Experimental plugin APIs

This reference describes experimental plugin API members and runtime compatibility
interfaces. Names retain their SDK spelling. Source paths and workspace commands
are relative to `tools/hyperfide`. HTTP API paths below are relative to `/api/v1`
unless the prefix is shown.

The [plugin authoring guide](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/SKILL.md)
provides usage examples. See [provider plugin API](provider-plugin-api.md) and
[provider bridge protocol](provider-bridge-protocol.md) for provider contracts.

## Composer popups

`ComposerCustomization.experimental_popups` registers `{ id, label, component }`
popups; `PluginComposerApi.experimental_openPopup(id)` opens one in that
composer and `experimental_closePopup()` closes it, both returning false when
they cannot act.

Docs: [frontend-hooks-and-ui.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-hooks-and-ui.md).

## `app.slots.experimental_composer`, `experimental_useComposerRenderer`, `experimental_ComposerRoot`, `experimental_useComposerContributions`, the composer search hooks, the prompt mention pill functions and the composer host components (`@hyperfide/plugin-sdk/app`)

`experimental_composer({ id, title, description?, component })` draws hyperfide platform's
composer input box everywhere hyperfide platform shows a composer. It is exclusive, chosen by
the synced `composer.provider` UI preference (Settings → Appearance →
Composer), and with no composer plugin enabled no composer is shown. hyperfide platform's own
composer is the bundled `plugins/composer` plugin (`hyperfide--composer`), built on
this slot: core renders every composer through `ComposerBox`
(`apps/web/src/components/promptbox/ComposerBox.tsx`), which resolves the
slot and provides the hooks below.

The composer reads and writes the draft through `useComposer()`, the same
path other plugins use: inside the box it is bound to the box's host, and
`submit()` runs the box's own Enter, queue or steer handler (the host picks
it from `experimental_mode` exactly as the box's shortcuts do), so a plugin's
submit steers a running thread exactly as Enter does. The host runs other plugins' draft observers and assembles every
highlight rule into `decorationSources`.

`experimental_useComposerRenderer()` carries `input` (the surface, submission,
typeahead, attachments, history, voice, sizing and collapse state, and hyperfide platform's
execution controls, handoff banner and compact-before-send chip, named so the
composer chooses where they go), the composer `view`, and host services
(`threadLinks`, `clipboard`, `attachmentSrc`, app key dispatch, toasts).
`input.submission.enter` and `.modifier` are the box's two actions, worked
out by the host from the surface and the user's "Enter steers" setting: each
says what it does now (`send`, `queue` or `steer`), whether it needs a
draft, and its shortcut, and `run()` runs it. The box only gates and draws
them; which key queues and which steers, and that an empty steer sends the
next queued message, stay with the host. `input.resolveMentionLink` says what clicking a
pill does, opening attachment mentions and tiles in hyperfide platform's viewer; core's
timeline, the thread view's attachment opener and the composer share that
opener. `input.maxHeight` caps the box where the host has limited room (the
compact New thread home), so the host does not need to measure the composer's DOM. `experimental_ComposerRoot` takes the composer's
root element and its editor and wires the composer: it publishes the editor to `useComposer()`, routes hyperfide platform's and
other plugins' composer commands, and registers pane focus and thread drops.
`experimental_useComposerContributions()` returns other plugins' popups, `+`
and send menu items and inline actions with their host renderers, applying
the surface's suppression. Host components cover the voice button, recording
bar and failed recordings, shortcut hints, the image lightbox, mention and
plugin icons and thread-title text.

Docs: [frontend-renderer-slots.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-renderer-slots.md).

`experimental_useComposerMentionSearch()` and
`experimental_useComposerCommandSearch()` search hyperfide platform's @-mention sources and
the provider's commands for the composer's open menu, and both return ranked
groups under their headings, so relevance and grouping stay with hyperfide. The
composer reports the menu's query with `search(query)` from the event that
changed it. Each composer box runs its searches in its own memoized provider,
which the surface configures with what to search (project, environment,
provider, triggers): a keystroke in an open menu renders the provider and the
composer once, and plain typing never runs the search. A query passed during
render would have to reach the provider from an effect, a second render.
Both report `refreshing` while earlier results show. The command search has
no paging, since hyperfide platform returns every match, and no prefetch call:
`experimental_ComposerRoot` starts loading commands when focus enters the
composer. A character is a mention trigger or a command trigger, never both;
when a plugin mention provider claims `$`, `$` opens the mention menu.
Every type in the contract is public: mentions, attachments and history use
the `useComposer()` shapes, and suggestions carry the mention to insert plus
per-kind display fields. hyperfide platform's composer works in these types throughout. hyperfide platform
reaches a composer only through the editor it hands to
`experimental_ComposerRoot`.

`experimental_describePromptMention(mention)`,
`experimental_promptMentionClipboardAttributes(mention, text)` and
`experimental_readPromptMentionClipboard(element)` give a composer's pills
hyperfide platform's classes, tooltip and text and its copy format, so pills copied between
any composer and hyperfide platform's messages paste back as pills. The timeline's pills use
the same implementation. `threadLinks.parse`, `.cached` and `.resolve` turn
pasted hyperfide platform thread links into thread pills; hyperfide platform owns
its route shapes, id format and lookup batching, and core's initial-draft
mention resolution shares the batching.

The bundled composer is on the forkable list (`tools/workspace/forkable-plugins.json`):
it imports only `@hyperfide/plugin-sdk`, npm packages and registry UI, and
`pnpm check:plugin-forks plugins/composer` builds a copy outside this
repository.

collapse state (`compact`, `onCollapse`, `minHeight`,
`maxHeight`, `mentionMenuPlacement`) is host state, because the controls outside the box
collapse with it; the attachment, clipboard, toast and thread-link services
stay composer-scoped until another caller needs them; voice keeps its config
and host components; `+` menu items show on surfaces that hide other plugin
customizations; @-mentions of the draft's own
attachments are the composer's, since it holds the draft, so hyperfide platform's mention
search returns none.

`apps/web/src/components/promptbox/composer-contract.test.tsx` is the
contract table: hyperfide platform's composer and a minimal textarea composer
(`apps/web/src/test/fixtures/minimal-composer.tsx`, public SDK only) each run
on the thread composer, a running thread that steers on Enter, embedded
thread chat, the queued and sent message editors and the New thread
composer, mounted through the real surface components. It covers drafts,
Enter, pane focus, thread drops, voice transcripts, other plugins' insert,
submit and popups, and which surfaces stay out of `useComposers()`.
`experimental_ComposerRoot` publishes stable functions that call the latest
editor and republishes only when the editor's state changes, so a composer
can build its editor inline on every render.

Styling the composer needs lives in the plugin (`plugins/composer/src/composer.css`).
hyperfide platform keeps the pill surface its timeline shares and the highlight classes of
its own decorations. hyperfide platform's controls inside the box size their labels against
hyperfide platform's own `[data-composer-box]` wrapper, so a composer declares no CSS
container. `input.compact` says when to show the one-line form: `"always"`
when the host collapsed the box, `"narrow"` when hyperfide platform's CSS container signal
says the pane is short or the column narrow. The box reports the form it
shows in `editor.state.layout`, which becomes `view.layout`; the host asks
for the caret at the end with `editor.focusEnd()`, and measures the menu
`experimental_ComposerRoot` is given on surfaces that make room for it, so
the input carries no change counters or layout callbacks. hyperfide platform releases
local attachment previews when attachments leave the draft.

## Composer commands

`PluginAppComposer.experimental_registerCommand({ id, title, defaultShortcut?, run })`
registers a palette and keyboard command that runs against the owning
composer's `PluginComposerApi`, rebindable under
`plugin:<plugin-id>/<command-id>` and sharing the `app.commands` id namespace.

Docs: [frontend-hooks-and-ui.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-hooks-and-ui.md).

## `settingsSection.experimental_page`

`experimental_page: "mobile"` mounts a plugin settings section only on
Settings → Mobile, and only when that plugin owns the selected access
provider. Omitting it keeps the section on the plugin's configuration page.

## `app.commands.register`

Registers frontend commands
(`{ id, title, defaultShortcut?, isAvailable?, run }`) for the quick palette
and keyboard bindings; the deprecated `app.slots.commandPaletteAction` alias
shares its validation and per-plugin id namespace.

Docs: [frontend-renderer-slots.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-renderer-slots.md).

## Discoverable RPC

`hyperfide.rpc.register` accepts `experimental_discoverable` and
`experimental_description` options, and method definitions accept
`experimental_description`. A discoverable method publishes its wire schemas
(Standard JSON Schema) and description. Discovery does not change RPC
authorization or dispatch.
`hyperfide.sdk.plugins.experimental_discoverRpc({ pluginId?, method?, signal? })`
lists the published methods of loaded plugins. Methods disappear on unload, so
callers handle the race between discovery and invocation. The fake host
exposes `experimental_publishedRpcMethods`.

## Plugin safe mode

`hyperfide.sdk.plugins.experimental_getSafeMode()` returns `{ enabled }`.
`experimental_setSafeMode({ enabled })` returns `{ enabled, problems }`, where
`problems` names each plugin that did not start when safe mode ended. While
safe mode is on, only plugins included with hyperfide platform run: `builtin` provenance, or
an auto-installed bundled source. Every other plugin stays unloaded with
status `disabled`. Each plugin's own `enabled` flag is untouched. Enable keeps
an affected plugin unloaded, reload reports a failure, and install and update
are refused. The flag persists on the server and backs
`hyperfide platform plugin safe-mode [on|off]`.

## Queued message edit holds

`hyperfide.sdk.threads.queuedMessages.experimental_holdForEdit({ threadId, queuedMessageId })`
(`POST /threads/:id/queued-messages/:queuedMessageId/edit-hold`) holds a
queued row while someone edits it and returns `{ leaseMs }`. While the hold
is live, no automatic dispatch (idle drain, schedule, plugin recheck, host
reconnect, failed-retry) claims the row, and the idle drain also keeps the
drainable rows behind it queued; rows waiting on a schedule or plugin are not
held back, and explicit send-now still claims it. Calling again renews the
lease; an unrenewed hold lapses after `leaseMs` and the periodic sweep
dispatches the row. It answers 409 once a dispatch claimed the row and 404
for a missing row. Holding never changes `updatedAt`, so a pre-hold
`expectedUpdatedAt` stays valid; a successful `queuedMessages.update` clears
the hold. `experimental_releaseEditHold` (`DELETE`, same path) cancels
without saving and is a no-op for an unheld row; releasing or saving
re-attempts dispatch immediately. The app's inline editor holds on open,
renews every quarter lease, and releases on close. Any client may renew or
release a hold.

## Plugin cache pruning

`hyperfide.sdk.plugins.experimental_pruneCache({ dryRun? })`
(`POST /plugins/cache/prune`) deletes cached git and npm plugin versions
under `<data>/plugins/cache` that no installed plugin uses and returns
`{ dryRun, removed, bytes }`; each removed entry has `pluginId` (null for an
unrecorded directory), `version`, `path`, and `bytes`, counting a shared
monorepo checkout once. It removes every non-active version regardless of
age and keeps active versions, versions a pending or restoring rollback
needs, directories containing or inside an installed plugin's root, and
in-flight staging directories. It runs under the plugin registration lock.
Removing a plugin deletes its cached versions at once, and a successful update
deletes replaced versions once activation stabilizes (its state snapshot
stays for the 7-day retention window). It backs `hyperfide platform plugin prune [--dry-run]`
and the palette's Clean up plugin cache.

## RPC caller identity (`ExperimentalPluginRpcHandlerContext.experimental_caller`)

Every `hyperfide.rpc.register` handler receives a context whose `experimental_caller`
is `{ kind: "plugin", pluginId }` for a loaded plugin's
`hyperfide.sdk.plugins.callRpc` and `{ kind: "client" }` for everyone else. A
per-load token makes the identity unforgeable by clients.

Docs: [backend-events.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-events.md), [backend-sdk.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-sdk.md).

## `hyperfide.http.experimental_websocket`

Registers an exact-path WebSocket upgrade in the plugin's
`/api/v1/plugins/<id>/http/` namespace. It shares HTTP route auth modes.
Sockets from a replaced or disabled plugin generation close with code 1012.
The test harness drives it with `experimental_openWebSocket`.

Docs: [backend-events.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-events.md), [testing.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/testing.md).

## `hyperfide.providers.experimental_contributeEnvHealth`

Registers one host-scoped readiness resolver beside a provider environment
contribution. When the bridge reports `unauthenticated` or `expired`, a
non-null answer presents the provider as ready with its label and status
message.

Docs: [providers.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/providers.md).

## `hyperfide.providers.experimental_contributeEnv`

Registers one resolver per provider per plugin that returns up to 32
environment entries for each session and turn, and for model catalog listing
with a `null` thread and project. Each entry is a literal or a
server-relative path the host expands. Conflicts resolve in plugin load
order, and the values override the shell environment.

Docs: [providers.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/providers.md).

## Compatibility windows

Registration rejects renamed declaration, tool and navigation-panel fields with
messages naming their replacements.

Presentation-less `toolCall` rows pass through `upgradeLegacyToolItem` in
`@hyperfide/domain`. It renders Read/Grep/Glob and read/grep/find/ls as
`fileRead`/`search` rows and suppresses Task/Todo/ToolSearch bookkeeping calls.

## Bridge compatibility exports

Re-exported from `@hyperfide/domain`, where each still has core consumers:

- `acpNativeReasoningSchema`
- `acpPermissionCliSchema`
- `acpReasoningCliSchema`
- `extensionKindSchema`
- `interactionRequestPayloadSchema`
- `isExtensionKind`
- `isUserQuestionPendingInteractionPayload`
- `isUserQuestionPendingInteractionResolution`
- `providerRecoveryKindValues`
- `threadEventItemPresentationSchema`
- `threadEventSearchModeSchema`

Aliases of moved definitions. The ACP pair aliases the ACP kit's
`experimental_acpLaunchSpecSchema` / `AcpLaunchSpec` and normalizer
(`@hyperfide/plugin-sdk/provider-bridge/acp`). The task-tool pair has no
replacement.

- `hostDaemonAcpLaunchSpecSchema`
- `normalizeHostDaemonAcpLaunchSpec`
- `claudeTaskToolNameSchema`
- `claudeTaskToolOutputSchema`
- `HostDaemonAcpLaunchSpec`
- `ClaudeTaskToolOutput`

Additional compatibility members:

- On `@hyperfide/plugin-sdk/provider-bridge/acp`, `AcpAgentProfile`, a deprecated
  alias of `AcpLaunchSpec`.
- On `@hyperfide/plugin-sdk/provider-bridge/testing`,
  `CONFORMANCE_ASSEMBLED_EVENT_METHOD`. The conformance kit assembles
  `thread/delta` itself and reads nothing under that method.
- The `disallowedTools` field of `threadStartParamsSchema`,
  `threadResumeParamsSchema` and `threadForkParamsSchema`. The runtime never
  populates it.

`experimental_useSidebarThreadActions` and `PluginSidebarThreadActions`
(`@hyperfide/plugin-sdk/app`), with the testing harness's `SidebarActionCall` /
`inspection.sidebarActionCalls`, are superseded by the thread action
registry, `useSdk().threads`, and `useHyperfideNavigate().toThread` / `toCompose`;
`experimental_archiveEnvironmentThreads` moved to
`experimental_useArchiveEnvironmentThreads`. Like `useComposerView` they are
`@internal`: published declarations drop them, while the runtime keeps
implementing them for installed plugins.

## Settings schemas and server writes

`PluginSettingDescriptor.experimental_schema` is a synchronous,
non-transforming Standard Schema validator. The server runs it on every
proposed value. `PluginSettingsHandle.experimental_set` validates and persists
the plugin's own fields, like a settings route or `hyperfide platform plugin config` write.

Docs: [backend-foundation.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-foundation.md).

## `PluginSettingDescriptor` type `"number"`

A numeric setting descriptor stores a finite `number`, renders a number
input, converts CLI values, and reads legacy numeric strings as numbers.

Docs: [backend-foundation.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-foundation.md).

## `hyperfide.experimental_hooks` (`on`, `recheck`)

`on("message.dispatch", handler)` answers core's admission checkpoint before
every message reaches a provider. A handler returns `proceed`, `wait` (queue
the message with a reason and optional `sendAt`), or `reject` (409).
Handlers run as a fail-closed chain in install order under one server-wide
lock. `recheck("message.dispatch")` schedules a re-attempt of every
plugin-queued row and resolves when the walk is scheduled.

Docs: [backend-events.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-events.md).

## `interaction.pending` (`hyperfide.events.on`)

Fires after core commits a pending interaction. It carries the public thread
and pending interaction DTOs and is observe-only.

Docs: [backend-events.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-events.md).

## `message.queued` / `message.dispatched` / `message.cancelled` / `turn.failed` (`hyperfide.events.on`)

These are observe-only announcements. `message.queued` (on first queue and on
every rewritten wait) and `message.dispatched` carry the `ThreadQueuedMessage`
DTO. `message.cancelled` fires when a queued row is deleted before dispatch.
`turn.failed` carries ids and failure facts after a thread lands in `error`.

Docs: [backend-events.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-events.md).

## `hyperfide.experimental_environments` (`register`, `recheck`)

Registers an environment provider: a named place threads run. Ids are flat
and first-wins, and the persisted owner plugin is the only one that can
recover or remove its resources. It declares `requires` eligibility facts,
Standard Schema `inputs`, optional `validate`, and durable `create`/`remove`
resource operations that core drives. `recheck` schedules another pending
launch attempt. The built-in checkout, worktree and personal workspace
behaviors are first-party plugins.

Docs: [backend-events.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-events.md).

## `@hyperfide/plugin-sdk/environment-provider`

Exports the resource-operation contract for
`hyperfide.experimental_environments.register`. It also exports optional
`availability(context)` and `restore(context)`. `availability` is cached per
project and machine, re-checked at thread creation, and reported to pickers
as `machineAvailability`. `restore` is the only way core rebuilds a destroyed
environment.
`PluginEnvironmentProviderCreateContext.experimental_claimPath(path)` durably
reserves a host path on the launch row before mutation, and resolves false for
competing claims or stale attempts. Values keep their prefix, and named types
are unprefixed.

Docs: [backend-events.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-events.md).

## `app.slots.experimental_environmentProviderInputs`, `experimental_BranchPicker`, `experimental_useBranches` and `experimental_useCheckoutState` (`@hyperfide/plugin-sdk/app`)

The slot registers `{ environmentProviderId, component }`, the control the
New Thread environment picker renders beside that provider. Its props are
`{ projectId, target, value, onChange }`. `target` is
`{ kind: "existing-host", hostId }` or `{ kind: "new-host" }`. The control
reports `{ status: "ready", value }` or `{ status: "blocked", reason }`.
Inputs are persisted and readable by every plugin, so they must hold no
secrets. The branch picker, branch hook and checkout-state hook are the host's
branch and checkout primitives for such controls.

Docs: [frontend-components.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-components.md) (environment row, branch and checkout
primitives). Only the API index names the slot registration itself.

## `app.slots.experimental_machineProviderInputs` (`@hyperfide/plugin-sdk/app`)

Registers `{ machineProviderId, component }`, a compact control for one
machine provider's inputs. It renders in the New Thread toolbar when a
selected composition declares machine inputs. The component receives
`{ value, onChange }` and reports ready JSON or a blocked reason. The value
must contain no secrets.

Docs: [frontend-components.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-components.md).

## `hyperfide.experimental_machines` (`register`)

Registers a project-independent machine provider: required id, displayName,
description, icon, `create`, `reconcileCleanup` and `remove`; optional
`ephemeral`, Standard Schema inputs, availability, validate, and paired
suspend/resume. Core owns enrollment, durable launches, cleanup retries and
lifecycle transitions. Plugins own allocation, filesystem preservation and
idle policy. The lifecycle context's `checkpoint(resource)` persists a
bounded (16 KiB), credential-free recovery record.

Docs: [backend-machines.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-machines.md).

## `@hyperfide/plugin-sdk/machine-provider`

Exports the machine provider's definition, input, availability/validation,
create and lifecycle context, progress, and resource/removal types. These
supporting declarations belong to the experimental machine namespace, so
their unprefixed names do not mean they are stable.

Docs: [backend-machines.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-machines.md).

## `hyperfide.branding.experimental_icons` (manifest) and namespaced presentation glyphs

A manifest map from names to plugin SVG files. Timeline presentation glyphs
and provider `icon` may reference an entry as `"<pluginId>/<name>"`. The map
is validated at build and load. Ingest rejects a glyph that is not the
emitting plugin's own declared icon. `hyperfide.branding.icon` refuses the namespaced
form.

Docs: [quickstart.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/quickstart.md) (manifest and SVG rules), [providers.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/providers.md) (provider
icons), [backend-cli-agents.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-cli-agents.md) (tool presentation).

## `experimental_buildBridgeToolCallContent`

Converts a decoded hyperfide platform tool-call response into the ordered text and
inline-image content blocks that MCP and Pi tool results accept. It takes
ordered `contentBlocks` or the legacy aggregate `{ content, images }`.

## The ACP bridge kit (`@hyperfide/plugin-sdk/provider-bridge/acp`)

`experimental_acpProviderBridge` is the generic Agent Client Protocol bridge a
plugin re-exports, backed by `core/provider-bridge-acp` (the official
`@agentclientprotocol/sdk` transport, turns for agent-started work, slash
commands and session options as hyperfide platform thread state, elicitation forms as
question cards, sign-in guidance from advertised auth methods). It launches
the agent named by `providerOptions.acpLaunchSpec` with the dialect named by
`acpDialect` in bridge options. `experimental_probeAcpAgent` asks an installed
agent for its `agentCapabilities`, and `experimental_acpAgentProbeSchema`
validates the answer across a host RPC boundary.
`experimental_acpLaunchSpecSchema` / `AcpLaunchSpec` is the launch spec the
bridge parses.

`experimental_registerAcpDialect` adds a dialect. A plugin calls it at module
load in its `hyperfide.host` artifact with an `AcpDialect` and names the id as
`acpDialect` in its registration. Empty and built-in ids (`acp`, `cursor`,
`grok`, `omp`, `opencode`) throw, re-registering an id replaces it, and an
unregistered id resolves to the generic dialect. hyperfide platform's own dialects resolve
through the same registry (`resolveAcpDialect`).

`@hyperfide/plugin-sdk/provider-bridge/acp-next` is a deprecated re-export of
this kit. Each subpath has its own dialect registry, so a plugin imports all
members from the same subpath.

Docs: [providers.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/providers.md) (ACP agents), [provider-bridge-api-index.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/provider-bridge-api-index.md).

## `PluginProviderDeclaration.experimental_nativeSkillRoots`

Names the directories a provider's agent reads skills from: `user` relative
to the host home, and `project` relative to the workspace. hyperfide platform lists those
skills beside its own.

Docs: [providers.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/providers.md), [provider-plugin-api.md](provider-plugin-api.md) §1.

There is no `absolute` side; host-specific roots go
through `experimental_resolvesNativeRoots`.

## `PluginProviderDeclaration.experimental_nativeCommandRoots`

The same two-sided declaration, with the same per-root options, for flat
directories of `*.md` slash-command prompt files.

Docs: [providers.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/providers.md), [provider-plugin-api.md](provider-plugin-api.md) §1.

## `PluginProviderDeclaration.experimental_resolvesNativeRoots`

Declares that the plugin's `hyperfide.host` entry serves
`resolveNativeRoots({ providerId, cwd })`. hyperfide platform scans the answer beside the
declared roots (cached ten seconds). A failed answer yields no resolved roots
and never fails the listing.

Docs: [providers.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/providers.md), [provider-plugin-api.md](provider-plugin-api.md) §1.

## `experimental_nativeRootsHostContract` (`@hyperfide/plugin-sdk/host`)

The one-method host RPC contract (`resolveNativeRoots`). Input is
`{ providerId, cwd: string | null }`. Output is `{ skills, commands }`, each a
list of host-absolute roots with `origin`, `recursive`, `ancestors`,
`namePrefix`, and a `shape` that tells the daemon how to read the root.

Docs: [providers.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/providers.md).

## `experimental_filterResolvedNativeRoots` (`@hyperfide/plugin-sdk/host`)

Checks a `resolveNativeRoots` answer root by root against the contract. It
drops each failing root with a warning, and truncates a side past 256 roots,
so one bad root cannot void the whole listing.

Docs: [providers.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/providers.md).

## Vendor plugin roots (`experimental_resolveClaudePluginRoots` and `experimental_resolveVendorPluginRoots`, `@hyperfide/plugin-sdk/host`)

`experimental_resolveVendorPluginRoots({ plugins, layout })` walks vendor
plugin directories into native roots with `namePrefix: "<plugin>:"`.
`layout: "claude"` reads each plugin's root `SKILL.md`, `skills/`, `commands/`,
then its manifest `skills` and `commands` entries. `layout: "grok"` reads only
the manifest entries, each recursive.
`experimental_resolveClaudePluginRoots({ cwd, homeDir, env })` reads Claude
Code's installed-plugin registry and enablement settings and runs the walk.
Its answer also carries `claudeDir`. Two shared rules apply. User-origin
plugins follow skill symlinks, project-origin plugins do not, and command
components never follow links. Within an answer each path appears once per
side, and the first root to claim it wins.

## `PluginSettingDescriptor.experimental_multiline`

A `type: "string"` descriptor flag that renders a multi-line monospace editor.
The stored string is unchanged. A descriptor that combines it with
`secret: true` is refused.

Docs: [backend-foundation.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-foundation.md).

## `hyperfide.sdk.threads.experimental_listPluginMetadata`

`POST /threads/plugin-metadata` with `{ pluginId, threadIds }` (1–200 ids)
returns `{ threads: { threadId, metadata }[] }` for each thread holding that
plugin's namespace, archived and deleted threads included, in one indexed
query; threads without a namespace and corrupt namespaces are omitted. The
plugin-bound SDKs default `pluginId` to the caller.

Docs: [backend-sdk.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-sdk.md).

## `hyperfide.sdk.threads.experimental_listAncestors`

`POST /threads/ancestors` with `{ threadIds }` (1–200 ids) returns
`{ threads: { threadId, ancestorIds }[] }`, parent first up to the root,
for every existing thread including archived and deleted ones, in one
recursive primary-key query.

Docs: [backend-sdk.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-sdk.md).

## `hyperfide.sdk.threads.experimental_listDescendants`

`POST /threads/descendants` with `{ threadIds, includeArchived?,
includeHidden? }` (1–200 ids) returns `{ threads: { threadId, descendantIds
}[] }` in breadth-first order for every existing, non-deleted thread.
Archived and hidden descendants are omitted unless flagged, matching
`threads.count`, but the walk passes through them; anything reachable only
through a deleted thread is omitted. One recursive query over
`threads_parent_idx`. unarchived`.

Docs: [backend-sdk.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-sdk.md).

## `PluginSettingDescriptor.experimental_optionLabels`

A `type: "select"` descriptor record from option value to display label,
shown in the settings form. Stored values, defaults, `settings.get()` and
`hyperfide platform plugin config` keep the option values, and labels for values outside
`options` are refused at define time.

Docs: [backend-foundation.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-foundation.md).

## `hyperfide.server.experimental_dataDir`

The server's data directory. The Account Pool plugin keeps secret files under `<dataDir>/plugins/<pluginId>/secrets/accounts`.

Docs: [backend-foundation.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-foundation.md).

## `hyperfide.server.experimental_appUrl`

The operator-configured public app URL from `HYPERFIDE_APP_URL`, or `null`.

Docs: [backend-foundation.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-foundation.md).

## Bridge record mode (`experimental_recordProviderChildIo` and `experimental_isProviderBridgeRecording`)

`experimental_recordProviderChildIo` tees a provider child's stdio into record
mode and is a no-op when record mode is off.
`experimental_isProviderBridgeRecording` reports whether record mode is on.

Docs: [provider-bridge-protocol.md](provider-bridge-protocol.md) "Record
mode", [providers.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/providers.md).

## Portable provider spawn (`experimental_spawnPortableProcess`, `experimental_killPortableProcess`) (`@hyperfide/plugin-sdk/provider-bridge`)

`experimental_spawnPortableProcess({ command, args, cwd?, env?, stdio?, detached? })`
returns a `ChildProcess` launched the way the daemon launches one. On Windows
it resolves PATH/PATHEXT, runs npm `.cmd` shims, and hides the console;
elsewhere it behaves like `spawn`. `experimental_killPortableProcess(child, signal)`
terminates the whole process tree on Windows and sends `signal` to the child
elsewhere.

## `experimental_BridgeRecoveryError`

A request handler throws it to reject the request with a JSON-RPC `code` and
`error.data.recovery { kind, message, retryable }`.

Docs: [provider-bridge-protocol.md](provider-bridge-protocol.md) "Recovery
hints".

## Provider maintenance toolkit (`experimental_resolveExecutablePath`, `experimental_readCliVersion`, `experimental_commandOutput`, `experimental_versionFrom`, `experimental_compareVersions`, `experimental_formatCommand`, `experimental_npmCommand`, `experimental_npmGlobalInstallCommand`, `experimental_npmLatestVersion`, `experimental_probeNpmGlobalPackage`, `experimental_npmGlobalInstallSource`, `experimental_installationVerification`, `experimental_downloadedInstallerCommand`, `experimental_clampPercent`) (`@hyperfide/plugin-sdk/provider-bridge`)

These are the host-local probes and install commands behind a bridge's
`provider/health`, `provider/usage` and `provider/installation/*` answers for
a user-installed CLI.

- Probes: `resolveExecutablePath` returns an absolute path or null, and on
  Windows prefers a PATHEXT match such as an npm `.cmd` launcher.
  `readCliVersion` runs `--version` with stdin closed and returns the first
  valid SemVer token or null. `commandOutput` returns trimmed stdout+stderr
  or null. The others are `versionFrom`, `npmLatestVersion` and
  `probeNpmGlobalPackage`.
- Decisions: `compareVersions` uses SemVer precedence and throws `TypeError`
  on invalid input. `npmGlobalInstallSource` returns `npmGlobal`, `external`
  or `notInstalled`. `installationVerification` passes an install on
  existence, and an update on reaching the latest version (or any change
  when the registry was unreachable).
- Actions: `npmGlobalInstallCommand`; `downloadedInstallerCommand`, which runs
  a vendor script from a temp file and, given `powershellUrl` on Windows,
  returns `irm <url> | iex`; `npmCommand`; `formatCommand`; and
  `clampPercent`.

## Presentation builders (`experimental_presentationTitle`, `experimental_presentationDetail`, `experimental_withTitle`, `experimental_presentationFileName`, `experimental_COMPACTION_PRESENTATION`, `experimental_REASONING_PRESENTATION`, `experimental_fileReadPresentation`, `experimental_searchPresentation`, `experimental_webSearchPresentation`, `experimental_webFetchPresentation`, `experimental_planStepsPresentation`, `experimental_toolPresentation`) (`@hyperfide/plugin-sdk/provider-bridge`)

These build the grammar-v3 `presentation` a bridge stamps on every item
([provider-plugin-api.md](provider-plugin-api.md) §3 owns the vocabulary).
`presentationTitle` takes the first non-empty line capped at 160 characters,
or undefined. `presentationDetail` caps a detail at the persisted schema's 280
characters. `withTitle` stamps a title only when there is one, and
`presentationFileName` returns the last path segment. The two constants carry
hyperfide platform's fixed wording for compaction and reasoning rows. The five builders shape
file read, search, web search, web fetch and plan-steps rows, and
`toolPresentation` is the generic "Running/Ran <tool>" row.

## `experimental_readBoundedLines` (`@hyperfide/plugin-sdk/provider-bridge`)

A newline-delimited line reader. It frames on LF only (never `readline`, which
also splits on U+2028/U+2029) and strips a trailing CR. A line over
`maxLineBytes` (default 64 MiB) is discarded up to its terminator and reported
through the required `onOverflow` with its byte count. The optional `onClose`
fires after a final unterminated line is emitted.

## Live-file navigation (`experimental_FileLink`, `HyperfideNavigate.experimental_openFilePreview`, and `HyperfideNavigate.experimental_openFileExternally`)

`experimental_FileLink` and `experimental_openFilePreview` open explicit live
workspace, host and thread-storage file targets through the current surface's
shared file-tab controller. `experimental_openFileExternally` opens a target
in the client's preferred external app. The boolean results report host
acceptance, and malformed targets stay inert.

**Core callers.** `usePanelFiles` owns file opening on the thread view, New
thread screen, and plugin page: core's workspace, host, and storage opens and
`openFilePreview` build the same tab requests under one scope rule, which
`usePanelFiles.test.tsx` checks against every surface. Links route through
`usePanelBrowser` on every surface.

Docs: [frontend-components.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-components.md), [frontend-hooks-and-ui.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-hooks-and-ui.md).

## Terminal navigation (`HyperfideNavigate.experimental_openTerminal`)

Shows a terminal the plugin created with `useSdk().terminals.create` in the
current surface's terminal panel. A thread surface accepts only that thread's
terminals, the New thread screen only its current terminal scope, and a
plugin page any terminal. It resolves false for unknown or exited terminals
and surfaces without a terminal panel, and closing the tab closes the
terminal.

**Core callers.** This API calls `usePanelTerminals`'s `open`; core's Start
terminal row, the `terminal.open` shortcut, and terminal tab selection use
its `select` on every surface. `usePanelTerminals.test.tsx` runs one contract
table against the thread view, New thread screen, and plugin page.

Docs: [frontend-hooks-and-ui.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-hooks-and-ui.md).

## Host plugin foundation (`hyperfide.hosts.experimental_client`, `ExperimentalHostClient.experimental_onWorkerExit`, `ExperimentalHostClient.experimental_onSignal`, `ExperimentalHostRpcContext.experimental_retainWorker`, `experimental_defineHostEntry`, `experimental_killProcessesWithCwdUnder`, `experimental_readProcessIdentity`, and `experimental_createHostEntryHarness`)

A plugin declares one `hyperfide.host` Node entry with
`experimental_defineHostEntry`, shares a Standard Schema contract with its
server, and calls methods on an explicit enrolled host through
`hyperfide.hosts.experimental_client`. The client can observe unexpected worker exits
and ephemeral host signals. Handlers can hold retention leases, and idle
workers are evicted after five minutes. `experimental_killProcessesWithCwdUnder`
reaps processes working under any of the given directories before they are
deleted (the SDK also accepts a single `{ directory }` at runtime), and `experimental_readProcessIdentity(pid)` returns
`{ command, startedAt }` so a caller can confirm a recorded PID before
signalling it.

Docs: [backend-foundation.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-foundation.md), [testing.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/testing.md).

## Fixed-tab targets (`experimental_target`, `experimental_useAppPanel`, and `experimental_useFixedTabTarget`)

A fixed tab's `experimental_target` validator owns a JSON-safe target type.
`experimental_useAppPanel()` selects one of the calling plugin's eligible tabs
on the current surface and can submit a target.
`experimental_useFixedTabTarget()` returns the validated current-session
target with a sequence and `clear()`. Targets are memory-only: they survive
remounts but not a refresh.

Docs: [frontend-core-slots.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-core-slots.md), [frontend-components.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-components.md).

## `PluginNavPanelRegistration.experimental_useBadge`

A nav panel hook returning `ExperimentalBadge`
(`{ count?, text?, tone?, label }`) or `null`. `count` and `text` are
exclusive; `text` is at most 8 characters; `tone` is `neutral` (the default),
`warning`, or `critical`, and hyperfide platform owns the colors. The sidebar navigation model
runs the hook once per registered panel in the plugin's slot scope and crash
boundary and hands the value to every navigation provider as `item.badge`.
The bundled Navigation plugin's Rail draws a count (capped at `9+`) or a dot
on the icon, and does not draw `text`; its Top list and More menu draw the count
(capped at `99+`), the text, or a dot. Both add `label` to the accessible name
and put a dot on More, in the strongest hidden tone, while a badged panel is
hidden there.

A plugin that passes `experimental_sidebarAccessory` keeps loading; the
field is dropped with a warning.

Docs: [frontend-core-slots.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-core-slots.md).

## `app.slots.experimental_sidebarNavigation` (`@hyperfide/plugin-sdk/app`)

**What it does.** Draws the sidebar navigation: New thread, Search threads,
Plugins, Skills, Automations, plugin panels, and More, plus optional controls
in the sidebar header. Exclusive.
`sidebar.navigationProvider` picks the provider by plugin key: the bundled
Navigation plugin's `navigation/rail` or `navigation/top-list`, another
plugin's key, or `__automatic__` (the default: the first registered provider
from another plugin, else `navigation/rail`). Legacy `__builtin__` and
`navigation/navigation` read as `__automatic__`.

The registration declares `placement`, `rail` or `list` (the default). hyperfide platform lays
the sidebar out from the placement before the provider renders, and keeps the
layout around it: for `rail`, a fixed-width column that stays when the sidebar
collapses and Settings at the bottom of the column; for `list`, the area
above the thread list, a Settings footer action,
Back to app rows on the Settings, Plugins, and Skills lists, and a collapse
that hides the whole sidebar. Both placements use the same framed macOS
window chrome on wide viewports, so changing placement does not move the
window controls, history controls, or the vertical position of Settings search.
The component receives `isCompactViewport`,
`placement`, and the deprecated `experimental_Original`, which renders the
Navigation plugin's provider for the same placement, or nothing when that
plugin is disabled.

The optional `headerComponent` renders in the sidebar header row while the
navigation is active. On wide macOS desktop windows, this row sits below the
window title bar; elsewhere it sits between the sidebar toggle and hyperfide platform's back
and forward buttons. It receives `width`, `controlSize`,
and `isCompactViewport`. The host clips it to the row, keeps the macOS drag
region for non-interactive space, aligns it with the macOS window controls,
and on a crash draws the header of the Navigation plugin's provider for the
same placement, with one toast. It replaces the separate
`experimental_sidebarHeader` slot and its `sidebar.headerProvider` setting,
which remains as an untyped runtime alias: a plugin's header joins each of its
navigations that has no `headerComponent`.

Before plugin frontends settle, a pinned provider, or the provider Automatic
last chose, reserves the placement it last had and draws nothing until it
registers; Automatic with nothing remembered reserves the rail. Once
frontends settle, a missing pinned provider falls back like Automatic, and
with no provider at all the sidebar uses the list placement and says no
navigation plugin is enabled. A crashing provider is replaced by the
Navigation plugin's provider for its placement, with one toast.

**Core callers on the same path.** hyperfide platform's Rail and Top list are the bundled,
default-enabled Navigation plugin (`plugins/navigation`), which is on the
forkable list. It registers both through this slot with no host-only API:
the host owns only the placement layout, Settings, the customize editor, and
the navigation model. Rail's header is its New thread button, registered as
`headerComponent`; it does not read the size props.

## `experimental_useSidebarNavigation`, `experimental_useSidebarNavigationSplit`, `experimental_SidebarNavigationIcon` (`@hyperfide/plugin-sdk/app`)

**What it does.** `experimental_useSidebarNavigation()` returns
`{ items, activeItemId, isHomeActive, isShortcutModifierHeld, actions }` from
one model mounted above the sidebar. Items arrive in the user's saved order
with `isVisible`, `isLoading`, `pluginId`, shortcut metadata, and `badge` (the
panel's `experimental_useBadge` value). The runtime ignores `experimental_Accessory`; counts come from `badge`. Item ids are the keys stored in
`sidebar.pluginPanelOrder` and `sidebar.visiblePluginPanels`, and items keep
their identity while unchanged. `actions` covers
`activate(id, { target })`, `goHome({ target })`, `setVisible`,
`setOrder` (unknown
ids are dropped and omitted ids keep their relative order at the end),
`openCustomize()` (hyperfide platform's editor in a popover beside the navigation, after open
menus close, returning focus to the opener), `openDetails`, and
`disablePlugin`. `target` is `current`, `split`, or `tab` and the host handles
the navigation: `split` opens or focuses a pane, replacing the focused pane
at the pane limit. It opens a browser tab instead when the destination cannot
be a pane, splitting is disabled, the viewport is compact, or no split layout
exists. `tab` always opens a browser tab. Both require a synchronous user
gesture. Home accepts `current` or `tab`, with `tab` opening the new thread
screen. `goHome()` without options navigates in place. The bundled Navigation
plugin requests `split` on Cmd/Ctrl-click; the host owns the fallback.
Search remains a command at every target. Native shells
retain their window-open policy.
Existing plugin bundles using `activate(id, { openInSplit })` map `true`
to `target: "split"` and `false` to `target: "current"`, using the same
fallback rules. The SDK types only accept `{ target }`, so rebuilt plugins
must migrate. An explicit `target` takes precedence.

Calls after the caller unmounts do nothing; outside the
sidebar the hook returns no items and inert actions.
`experimental_useSidebarNavigationSplit(id, options?)` mirrors
`experimental_useSidebarThreadSplit` and accepts `activation: "distance"` and
`onDragStart` for rows in menus. `experimental_SidebarNavigationIcon` renders
hyperfide platform's glyphs and panel icons with plugin branding as fallback.

**Core callers on the same path.** The bundled Navigation plugin's Rail, Top
list, and header New thread button use these hooks; the app's Settings →
Appearance → Navigation picker reads the same registrations.

## `PluginContentScriptContext.experimental_setThreadRowStatus`

Lets a plugin-lifetime content script set or clear one of its own status
indicators on an explicit thread row. The status survives route changes and
clears when that frontend generation deactivates.

Docs: [frontend-core-slots.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-core-slots.md).

## `hyperfide.providers.register` (`experimental_bridgeOptions`, `experimental_visibility`, and the `experimental_providerBridge` artifact export)

`experimental_bridgeOptions` is bounded JSON, opaque to core, delivered to
every bridge launch as provider-scoped static options.
`experimental_visibility: "installed"` withholds a provider from unscoped
listings until its `provider/health` is not `not_installed`. The bridge is the
`experimental_providerBridge` export of the plugin's `hyperfide.host` artifact.

Docs: [providers.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/providers.md), [provider-plugin-api.md](provider-plugin-api.md) §1–2.

## `@hyperfide/plugin-sdk/provider-bridge` (the provider-bridge authoring surface)

The published module a provider bridge compiles against. It holds
`experimental_defineProviderBridge`, the Provider Bridge Protocol's methods,
the `thread/delta` grammar and param schemas, the bridge kit's helpers, and
the `@hyperfide/domain` command-plane vocabulary those params reference. It is
curated by hand with named exports only, and bundled into the plugin's
artifact rather than stubbed at runtime.

Docs: [providers.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/providers.md), [provider-plugin-api.md](provider-plugin-api.md) §2,
[provider-bridge-protocol.md](provider-bridge-protocol.md).

bridges emit the protocol's own `thread/delta` grammar,
and the `@hyperfide/domain` command-plane, interaction and enum types they reference
stay re-exported through the facade permanently.

## `@hyperfide/plugin-sdk/provider-bridge/testing` (the provider-bridge testing kit)

The published kit a bridge author proves a bridge with, with no private
`@hyperfide/*` package in reach. It contains the conformance runner
(`experimental_runBridgeConformance`), the real delta assembler, the
delta→event collector, the in-process JSON-RPC harness, the calibration
normalizer, and the recorded-replay harness and readers. It is
framework-agnostic and exports named members only.

Docs: [providers.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/providers.md).

## `experimental_scanPublicSdkOnly` (`@hyperfide/plugin-sdk/testing`)

Scans a plugin package for imports outside the public SDK and an allowlist.
It returns the files read and each offending specifier: a private `@hyperfide/*`
package, `outside-package`, `dynamic-specifier`, and `@hyperfide/*` package.json
dependencies. The suite asserts on the result.

Docs: [testing.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/testing.md).

## `app.experimental_useProviders` (`@hyperfide/plugin-sdk/app`)

Returns `{ status, providers }`, the host's `ProviderInfo[]` roster in picker
order with shared cache and realtime invalidation.

Docs: [frontend-registration.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-registration.md),
[provider-plugin-api.md](provider-plugin-api.md) §5.

## `app.experimental_useCodeTheme` (`@hyperfide/plugin-sdk/app`)

Returns `{ mode, name, theme }`: the active light/dark mode, the code theme
name hyperfide platform renders it with, and the resolved VS Code theme document, for plugins
that render code with their own engine.

Docs: [frontend-hooks-and-ui.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-hooks-and-ui.md).

## `app.experimental_copyToClipboard` (`@hyperfide/plugin-sdk/app`)

A plain function, `({ text, html? }) => Promise<boolean>`, over the writer
hyperfide platform's own copy actions use: hyperfide platform Desktop's native clipboard, then the Clipboard
API, then `execCommand("copy")`. It resolves true only once a path wrote the
content and never rejects. The test harness records writes in the most
recently rendered slot's `inspection.experimental_clipboardWrites` and
resolves with that slot's `experimental_copyToClipboard` render option
(default true).

**Core callers.** The export is `copyToClipboard` from
`apps/web/src/lib/clipboard.ts`. Core copies call it directly or through
`copyToClipboardWithToast` and `useClipboardCopy`: `CopyButton`, the message
action bar, file preview, terminal, and the Copy thread link thread action.

Docs: [frontend-hooks-and-ui.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-hooks-and-ui.md).

## `app.experimental_saveFile` (`@hyperfide/plugin-sdk/app`)

`experimental_saveFile({ fileName, url } | { fileName, blob })` starts a
download through the same function hyperfide platform's own downloads use. It clicks a
temporary download anchor, using an object URL for a blob. The hyperfide platform iOS shell
downloads through `WKDownload`, keeping the WebView's request context, then
presents the share sheet; file bytes and cookies do not cross the JavaScript
bridge. The promise resolves `started` once the link is clicked or `failed`,
with an error toast, if it could not start. Completion and share-sheet
cancellation are not reported. The test harness resolves `started` and records
nothing.

**Core callers.** The export is `saveFile` from `apps/web/src/lib/save-file.ts`,
which file preview downloads, chat attachment links in the native shell
(`ConversationAttachments`), and voice-input recording downloads call. The
Tasks plugin's attachment cards call the SDK export.

Docs: [frontend-hooks-and-ui.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-hooks-and-ui.md).

## `app.experimental_usePluginId` (`@hyperfide/plugin-sdk/app`)

Returns the id of the plugin that owns the calling component, the same value
as `hyperfide.pluginId`.

Docs: [frontend-hooks-and-ui.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-hooks-and-ui.md).

## `app.experimental_useQuestionFormHost` (`@hyperfide/plugin-sdk/app`)

Returns the remappable answer shortcuts hyperfide platform binds while a pending interaction is
open, keyed by option index, and `registerChoiceHandler` for a chosen index.
Outside a pending interaction the map is empty and handlers never run.

Docs: [frontend-hooks-and-ui.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-hooks-and-ui.md).

## `app.slots.experimental_providerIcon` (`@hyperfide/plugin-sdk/app`)

Registers `{ providerKind, providerId, icon }`, a React component that draws
one agent, environment, or machine provider's icon in place of its served
`logoUrl`. Exact-kind registrations beat all-kind ones.

Docs: [frontend-renderer-slots.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-renderer-slots.md), [frontend-components.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-components.md).

## `experimental_ProviderModelPicker` (`@hyperfide/plugin-sdk/app`)

hyperfide platform's execution picker as a controlled
`{ providerId, model, reasoningLevel, serviceTier? }` component. It has
optional host/environment `routing`, `allowProviderChange`, `align` and
`disabled`, and the composer's catalog and normalization policy.

Docs: [frontend-components.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-components.md).

## `experimental_PermissionModePicker` (`@hyperfide/plugin-sdk/app`)

hyperfide platform's permission picker as a controlled
`{ providerId, value, onChange, routing?, align?, disabled?, className? }`
component. It applies the composer's supported modes, fallback order and the
routed machine's permission ceiling.

Docs: [frontend-components.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-components.md).

## `app.slots.experimental_timelineRenderer` (`@hyperfide/plugin-sdk/app`)

Registers `{ kind, component }` to render the expanded body of the plugin's
own extension rows (`"<pluginId>/<name>"`), of `"tool"` rows from its
providers, or of a `requestInput` form's history (`"<pluginId>/<rendererId>"`).
The row header stays host-rendered, and a crash falls back to the declarative
base.

Docs: [frontend-renderer-slots.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-renderer-slots.md), [backend-cli-agents.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-cli-agents.md) (form rows),
[provider-plugin-api.md](provider-plugin-api.md) §5.

## `experimental_NewThreadComposer` (`@hyperfide/plugin-sdk/app`)

The host-owned new-thread compose surface. It calls `onSubmit` with a
`NewThreadRequest` carrying every resolved selection (including
`sessionOptions` when the user changed a provider's agent option), and the plugin then
creates the thread with `hyperfide.sdk.threads.spawn`, which owns filing and
attribution.

**Core callers.** The New thread screen (`RootComposeView`) renders the same
`apps/web/src/components/promptbox/NewThreadComposer.tsx`;
`PluginNewThreadComposer` adapts it for the SDK, and its round-trip tests
(`PluginNewThreadComposer.test.tsx`, `new-thread-environment-seed.test.ts`)
guard the seed mapping against the root composer's environment resolution.

Docs: [frontend-components.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-components.md).

## `app.slots.experimental_appOverlay` (`@hyperfide/plugin-sdk/app`)

Mounts a no-props plugin component once per app window, outside route-owned
layout, with app-level SDK hook context. A crash hides only that overlay.

Docs: [frontend-core-slots.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-core-slots.md).

## `app.slots.experimental_newThreadPanelAction` (`@hyperfide/plugin-sdk/app`)

Adds a row to the root New thread screen's right-panel Actions list. The row
can open a closable tab with `{ projectId: string | null, params: JsonValue | null }`.
It is separate from `threadPanelAction`, which requires a `threadId`.

Docs: [frontend-core-slots.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-core-slots.md).

## `app.experimental_sidebarFooter` (`@hyperfide/plugin-sdk/app`)

Registers host-rendered icon items in the sidebar footer: an `action` with
`onActivate`, or a `disclosure` whose component is revealed above the row and
receives `dismiss()`. A disclosure registration returns an
`open`/`close`/`toggle` controller, and only one disclosure is active across
all plugins.

Docs: [frontend-core-slots.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-core-slots.md).

## `app.slots.experimental_threadList` (`@hyperfide/plugin-sdk/app`)

An exclusive slot that replaces the sidebar's scrolling thread list. hyperfide platform's own
list is the bundled `thread-list` plugin. Automatic selection prefers another
registered list, and the user can pin one with `sidebar.threadListProvider`.

Docs: [frontend-registration.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-registration.md).

## AI services (`hyperfide.experimental_aiServices.register`)

Registers `{ id, displayName, complete?, transcribe?, status? }`. `complete`
serves thread titles and commit messages, `transcribe` serves voice input,
and `status` feeds pickers and Automatic. The user selects per task
(`automatic`, `off`, or `{ pluginId, serviceId }`), and an explicit pick
never falls through to another service.

Docs: [backend-cli-agents.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-cli-agents.md), [provider-plugin-api.md](provider-plugin-api.md) §7.

## `PluginFileOpenerSource.experimental_hostId` (`@hyperfide/plugin-sdk/app`)

Names the explicit host for a project-backed workspace file that a file opener
cannot route through a thread or environment. It is omitted for the other
source kinds and for the primary host.

Docs: [frontend-core-slots.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-core-slots.md).

## `experimental_VoiceInputTextarea` (`@hyperfide/plugin-sdk/app`)

**What it does.** A host-owned controlled textarea with hyperfide platform's voice input. It
takes `value`, `onValueChange`, and an optional `onVoiceInputActiveChange`;
every other textarea attribute, including `ref` and `className`, reaches the
underlying `<textarea>`. The caller styles the textarea; the host wraps it in
a relative container and, when the browser supports voice input, adds bottom
padding and the microphone, waveform, cancel, and stop controls. Finished
transcripts are appended to `value` through `onValueChange` and stay editable;
nothing is submitted. `onVoiceInputActiveChange` is true from the start of
recording until transcription finishes or is cancelled, and false on unmount,
so a form can hold navigation and submission. Unmounting discards late
transcripts. It uses the same microphone preference, transcription service,
and error handling as the prompt box. Without voice support it renders the
plain textarea; the test harness renders that plain textarea too.

The registry's `voice-input-textarea` item re-exports it, and the registry's
`question-form` renders its free-text answer with it. Inside hyperfide platform, the built-in
Ask User Question and pi plugins and hyperfide platform's own question form reach the same
component through the `@hyperfide/shared-ui/voice-input-textarea` module the build
shims.

## `experimental_SourceCode` / `experimental_Diff` (`@hyperfide/plugin-sdk/app`)

Host-owned renderers for supplied code. `experimental_SourceCode` takes text
and a path. `experimental_Diff` takes a single-file patch, a path and
optional `experimental_fullFileContents` (`{ old, new }`). They are the same
components hyperfide platform's own surfaces use, so an active renderer replacement covers
both.

Docs: [frontend-components.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-components.md).

context expansion takes caller-resolved contents, not a
loader callback.

## `app.slots.experimental_sourceCodeRenderer` / `app.slots.experimental_diffRenderer` (`@hyperfide/plugin-sdk/app`)

Exclusive slots that replace hyperfide platform's source or diff renderer everywhere it draws
supplied content, including other plugins' calls to the public components.
Automatic selection prefers a registration from a plugin the user installed
over one bundled with hyperfide platform (such as Monaco), then the first in slot order, and
the user can pin hyperfide platform's renderer or a provider per client. A crash falls back to hyperfide platform's
renderer.

Docs: [frontend-renderer-slots.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-renderer-slots.md).

the pin stays per client; a crash swaps back to hyperfide platform's
renderer silently and disables the slot for the session; the replacement is
global, other plugins' surfaces included; replacements receive
`experimental_fullFileContents` like the public component.

## `app.slots.experimental_codeFenceRenderer` (`@hyperfide/plugin-sdk/app`)

Renders fenced code blocks of the claimed `languages` in assistant and
nested-agent message Markdown, where `messageDirective` activates. The
component receives `{ code, language, status, index, message, Original }`,
mounts once per fence, and survives streaming: the host owns the instance and
portals it to a placeholder keyed by the fence's source offset, keeping
iframes alive where `Element.moveBefore` exists. Contested languages, a
missing plugin, and a throwing fence fall back to `Original`. The harness
exposes `loadPluginApp(...).codeFenceRenderers`.

**Core callers.** `MarkdownPreview`'s code renderer
(`markdown-code-fence-renderers.tsx`), the pipeline that also resolves
`messageDirective`, hosts every claimed fence; the built-in `inline-vis`
plugin renders through it. HYPERFIDE's `mermaid` diagram stays a core special case
inside `Original`, because it renders on every Markdown surface and this slot
is scoped to messages.

Docs: [frontend-renderer-slots.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-renderer-slots.md), [testing.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/testing.md).

## `experimental_useSidebarThreads` / `experimental_useSidebarThreadActions` (`@hyperfide/plugin-sdk/app`)

The read hook returns the sidebar's live thread view (`PluginSidebarThread`
DTOs with a host-resolved `indicator`, plus `sections`) from the host's own
query and cache. The action hook is `@internal` and scheduled for removal
(see Scheduled removals). Neither
doc covers three September 2026 additions:

- `experimental_lifecycles` (active, archived, or both; active by default).
  With it, `PluginSidebarThreadsState.experimental_archived` exposes archive
  loading/error state, pagination flags and `fetchNextPage`; it is null for
  active-only reads.
- `PluginSidebarThreadsState.experimental_hosts`, the current machines in host
  query order, including those with no threads. Older hosts may omit it.
- `openNewThread({ hostId })`, unprefixed by request, preselects a machine for
  a new environment. `environmentId` takes priority.

Docs: [frontend-registration.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-registration.md).

## `app.slots.experimental_threadHeaderAction` (`@hyperfide/plugin-sdk/app`)

Renders a plugin component in the thread header's action row, once per pane
with that pane's `threadId`, for controls that draw live state. A crash
removes only that pane's control.

Docs: [frontend-registration.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-registration.md).

## `app.slots.experimental_threadInfoRow` / `app.slots.experimental_threadInfoSection` / `experimental_THREAD_INFO_ROW_GROUPS` / `experimental_THREAD_INFO_SECTION_GROUPS` / `experimental_InfoSection` / `experimental_DetailRow` / `experimental_InfoList` / `experimental_InfoListRow` (`@hyperfide/plugin-sdk/app`)

A registration is `{ id, group, order?, component }`. The component receives
`{ threadId, projectId }` and mounts once per visible thread's Info tab. Rows
render in the details block, sections below it, each sorted the way thread
actions are: by `group` (string compare), then `order` within the group (unset
last), then registration order with core first on a tie. hyperfide platform's groups are the
two exported tables; core uses orders 10, 20, 30 within each. Groups draw no
separator. Each plugin
entry mounts in its own crash boundary keyed by thread, so a crash drops only
that row or section. The four components are the Info tab's own building
blocks: `experimental_InfoSection` (heading, count pill, collapse),
`experimental_DetailRow` (label/value row with an optional icon),
`experimental_InfoList` (the "N more" list), and `experimental_InfoListRow`.
Inside a section slot, `experimental_InfoSection` collapses, and the state is
stored in the `infoPanel.collapsedSections` preference under the core id or
`<pluginId>/<id>`.

**Core callers.** `ThreadMetadataContent` renders the Info tab only through
`ThreadInfoRows` and `ThreadInfoSections`
(`apps/web/src/components/secondary-panel/info/ThreadInfoSlots.tsx`), which
merge `CORE_THREAD_INFO_ROWS` and `CORE_THREAD_INFO_SECTIONS` with plugin
registrations. `experimental_DetailRow` is hyperfide platform's only label/value row
(`DetailRow` in `apps/web/src/components/ui/detail-card.tsx`): all eleven core
rows and the commit dialog draw with it, and core's five sections draw with
`InfoSection` and `InfoList`. The
Automations plugin registers the Automations section. Contract tests:
`ThreadInfoSlots.test.tsx`.

Docs: [frontend-registration.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-registration.md), [frontend-components.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-components.md).

## `app.slots.experimental_threadAction` / `experimental_useThreadActions` / `experimental_useThreadActionRegistrations` / `experimental_ThreadActionsMenu` / `experimental_ThreadActionsContextMenu` / `experimental_THREAD_ACTION_GROUPS` (`@hyperfide/plugin-sdk/app`)

A registration is `{ id, title, icon, group, order?, useData?, item }`.
`useData` runs once per registration for the whole app and receives
`{ threadIds }`, every thread a visible surface evaluates actions for,
debounced (32 ms quiet, 100 ms max). `item` is a pure function of
`{ thread, data, sdk, navigate }` returning a `PluginThreadAction` or null;
throws drop the registration or that thread's item, and `run` errors are
contained. Registrations sort by `group`, `order`, then registration order
for every menu and the quick-action picker. Thread-list registers Move to
section, and push-notifications registers Notifications. The SDK test fake
throws when a menu `trigger` drops its props and ref.

**Core callers.** hyperfide platform's own actions are `CORE_THREAD_ACTIONS`
(`apps/web/src/lib/thread-actions/core-thread-actions.ts`), registered under
the reserved `hyperfide--core` owner. The header menu (`ThreadDetailView` →
`ThreadDetailHeader`), `RootComposeMobileRecents`, and
`ThreadArchiveCommandHandler` read the same registry, and the thread list's
row menus and quick actions use the SDK exports. The contract table in
`ThreadActionsMenu.test.tsx` runs every menu surface against the same
registrations.

Docs: [frontend-registration.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-registration.md).

## `experimental_useArchiveEnvironmentThreads` (`@hyperfide/plugin-sdk/app`)

Returns `(environmentId) => Promise<void>`, core's environment-group archive:
the optimistic `environments.archiveThreads`, split-pane cleanup, route
repair, and one ten-second Undo toast that unarchives the returned ids
(lifecycle owners first) and returns to the route. Failures show a host error
toast, and archive failures reject. It replaces
`PluginSidebarThreadActions.experimental_archiveEnvironmentThreads`. The test
harness records calls in `inspection.experimental_environmentArchiveCalls`.

**Core callers.** The hook returns `ThreadActionsProvider`'s
`archiveEnvironmentThreads`, hyperfide platform's only environment-group archive, which the
deprecated sidebar-actions shim also calls. Its one caller is the built-in
thread list's environment group menu.

Docs: [frontend-registration.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-registration.md).

## `app.slots.experimental_browserToolbarAction` (`@hyperfide/plugin-sdk/app`)

Renders a plugin component beside the address bar in each Browser tab. It
receives `threadId`, `tabId`, `url` and a compact-viewport hint, with a crash
boundary per tab mount.

Docs: [frontend-registration.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-registration.md).

## `ExperimentalPluginBrowserToolbarActionProps.experimental_page` (`@hyperfide/plugin-sdk/app`)

Gives a Browser toolbar action script access to its tab's top-level document
without a CDP lease. `evaluate(expression, { world })` runs in the main world
or hyperfide platform's isolated world, where `hyperfide.postMessage` reaches the calling plugin's
`onMessage` listeners. The value is `null` outside the desktop app.

Docs: [frontend-registration.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-registration.md).

## `PluginMentionProviderRegistration.resolve().experimental_images` (`@hyperfide/plugin-sdk`)

Lets a mention provider resolve a picked mention to agent-only image inputs,
each optionally preceded by a short agent-only text input. The images are
validated like ordinary prompt images before dispatch.

Docs: [backend-ui-lifecycle.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-ui-lifecycle.md).

## Composer handles

`useComposer()` returns a stable handle per composer with reactive draft and
selection fields. `insert`, `replace`, `onSubmitted`, `submit` and `setSelection`
are public methods. `ComposerCustomization.sendMenu`,
`PluginMessageActionContext.composer` and `useComposers()` use these handles.
Runtime compatibility methods marked `@internal` are omitted from published
declarations.

Docs: [frontend-hooks-and-ui.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-hooks-and-ui.md).

## Successful submission subscriptions

`PluginComposerApi.onSubmitted(listener)` observes successful local
thread-send, queue-create and new-thread-create mutations in the composer's
scope. It is a local UI notification, not a server event.

Docs: [frontend-hooks-and-ui.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-hooks-and-ui.md).

## `useComposer().submit` and dispatch `experimental_submission`

`submit({ experimental_mode?, sendAt?, experimental_data? })` submits the
draft through the on-screen composer's own queue or steer handler and rejects
with a user-presentable reason when it cannot. It requires a draft: where the
shortcut would steer the next queued message from an empty draft, `submit`
rejects instead. Without a mode it runs whichever one
Enter runs for this user; `"queue-if-active"` and `"steer-if-active"`, the
names `hyperfide.sdk.threads.send` uses, pick one explicitly, and
`"steer-if-active"` starts a turn where the composer has nothing to steer. The
options combine: `sendAt` and the plugin data travel into the same send,
queue or steer request the shortcuts build, so a scheduled steer steers the
turn running at its time. `experimental_data` reaches every `message.dispatch`
hook on the initial attempt as an `experimental_submission` envelope carrying
the submitting plugin's id. Core validates the data as JSON but neither
persists nor interprets it.

There is one implementation. `PromptBoxInternal` publishes `submit` on the
composer editor bridge; it picks Enter's handler, the queue handler or the
steer handler and gates it exactly as the matching shortcut is gated, then
calls the same `onSubmit` or `onModifierSubmit` the shortcuts and the send
button call, passing `sendAt` and the plugin submission. Only hosts that set
`sendsOnEnter` (the thread, embedded thread chat and New thread composers)
accept it. Off-screen voice sends run
the same Enter action. Every form rejects when the send fails; the Enter
action still shows its toast, and Enter and the send button ignore the
rejection.

Docs: [frontend-hooks-and-ui.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-hooks-and-ui.md), [backend-events.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-events.md).

## `useComposer().setSelection`

`setSelection(selection)` sets a composer's pickers through the same handlers
the pickers call, so a plugin choice is indistinguishable from a hand-made
one. Fields the composer has no picker for are ignored. It resolves with the
composer's selection once values commit and the model catalog settles, or
after 15 s. `PluginComposerApi.selection` is the reactive read side. It is a
stable snapshot that re-renders consumers on picker changes, and it is `null`
for composers without pickers. Both use unprefixed SDK names.

## Desktop browser control

`hyperfide.sdk.experimental_desktopBrowsers` (`ExperimentalDesktopBrowsersArea` and
its `ExperimentalDesktopBrowser*` types) drives the desktop app's browser. It
provides explicit host/window/thread discovery, tab creation in the single hyperfide platform
browser profile, and expiring control leases with scoped CDP connections. It
also provides capture, reveal, close and release, plus `subscribe`, which
polls tab state every two seconds and is not a lossless event log. Cookie
import from an installed browser uses `listImportSources` and `importCookies`.
Connection credentials stay private to workers on the browser host. The core
CLI is `hyperfide platform browser`.

## Machine paths and environment cleanup

`hyperfide.sdk.hosts.get({ hostId })` includes nullable `threadStorageRootPath` from
the machine's latest daemon session, without waking the machine (CLI:
`hyperfide platform machine show`). `hyperfide.sdk.environments.experimental_cleanup({ environmentId })`
requests removal of an unused provider-managed environment, overriding
retention and bypassing cleanup backoff. It rejects live threads and unmanaged
environments and succeeds for already-removed ones. Acceptance is not
completion: read the environment lifecycle fields for progress (CLI:
`hyperfide platform environment cleanup`). After five failed attempts, automatic removal stops
with a "Needs recovery" message in the existing lifecycle fields.
The additive `recover: true` option acknowledges interrupted hooks and retries
failed cleanup after an explicit inspection. Core's cleanup route and the CLI
use this same path. Linux checks cwd and mapped paths for processes owned by
the daemon UID and refuses unreadable processes; other users require separate
inspection. Unsupported platforms refuse workspace recovery. Provider resources
without an owned workspace can restart removal without acknowledging any hooks.
The inspection does not kill processes.

## Workspace diff Viewed state (`hyperfide.sdk.environments.experimental_diffViewed`, `experimental_markDiffViewed`, `experimental_clearDiffViewed`)

`GET /environments/:id/diff/viewed` (diff target in the query) lists
`{ files: { path, revision }[] }` for one environment and diff target.
`PUT` with `{ target, path, revision }` marks a file Viewed at a revision, and
`DELETE` with `{ target, path? }` clears one file or the whole target; both
return `{ ok: true }` and emit the `diff-viewed-changed` environment change.
`POST` accepts the diff query fields and `paths` (up to 500) for a bounded
lookup; an empty list returns no records. The SDK selects this path when
`experimental_diffViewed` receives `paths`. Core uses the loaded diff paths,
independent of display filters, so marking Viewed cannot hide its own query.
Records live in `diff_viewed_files`, keyed by environment, target
(`uncommitted`, `commit:<sha>`, `branch_committed:<branch>`, `all:<branch>`)
and path. Revisions come from the `revision` field that `diffFiles` now
returns for every file: `<oldMode>:<oldBlob>:<newMode>:<newBlob>` from
`git diff --raw`, with the worktree side hashed by `git hash-object`. A file
counts as Viewed only while the stored revision equals the current one.
`diffFiles` and `diffPatch` also accept `ignoreWhitespace` (git `-w`).

The workspace diff panel uses these methods for its Viewed checkbox.
(`DiffFilesPanel` through `useSetDiffFileViewed` and
`useEnvironmentDiffViewed`) and `hyperfide platform environment diff-viewed list|mark|unmark|clear`.

## Host process primitives (`@hyperfide/plugin-sdk/host`)

`experimental_spawnPortableOutputProcess` spawns an output-only child process,
and `experimental_sanitizeInheritedChildProcessEnv`
(`ExperimentalSanitizeInheritedChildProcessEnvArgs`) filters the inherited
environment, for host-local plugin commands.

## `TimelineOutputPreview.experimental_fullOutputAvailability` (`@hyperfide/plugin-sdk`)

Says why a timeline row holds a preview instead of the full output, as
returned by `hyperfide.sdk.threads.timeline`. `available` means a detail read can
still hydrate it. `detail-limit` means that read exceeded its byte budget.
`retention-expired` means the full value no longer exists.

## Document Markdown (`MarkdownProps.experimental_document`)

The Markdown component accepts explicit `{ target, rootPath, threadId }`
document context. `target` is a workspace or thread-storage live-file
identity, and `rootPath` is its resolved root. Relative links and images
resolve from the document's directory within that root. Thread-storage targets
must name the same thread. Malformed context never falls back to the ambient
workspace. Omitting it keeps message routing.

## `PluginFileOpenerProps.experimental_lineRange` (`@hyperfide/plugin-sdk/app`)

Passes the owning file tab's latest one-based, inclusive
`{ startLineNumber, endLineNumber }` range to its opener (`null` for no
requested navigation). Each targeted open supplies a new object, so openers
observe its identity.

Docs: [frontend-core-slots.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-core-slots.md).

## `hyperfide.experimental_serverAccess.recheck`

Sends a payload-less system `config-changed` notification to connected
clients, so configuration re-reads every registered provider's availability.

Docs: [backend-machines.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-machines.md).

## `hyperfide.experimental_serverAccess.register`

Registers a server access provider: `availability` (optionally with a public
`serverUrl`), `acquire({ key, hostId, signal })` returning a grant
`{ id, serverUrl, headers? }` or `{ status: "failed", message }`, and
`release({ key, hostId, grantId })`. Machines attach the grant's headers to
enrollment and runtime traffic.

Docs: [backend-machines.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-machines.md).

## Machine enrollment and bootstrap

`hyperfide.experimental_machines.bootstrap` prepares enrollment through the
provider's `MachineExecutor` and waits for the daemon connection. Each call
issues a fresh single-use credential for the same durable host identity. The
supporting `MachineExecutorRequest`, `MachineExecutor`,
`MachineBootstrapRequest` and `MachineBootstrapApi` belong to the experimental
`PluginMachines`, so their unprefixed names do not mean they are stable.

Docs: [backend-machines.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-machines.md).

## Machine provider `reconcileCleanup`

A required callback on `PluginMachineProviderDefinition`, used only when no
resource checkpoint exists. It discovers and removes uncertain allocations by
the durable launch key and must never allocate or bootstrap. Experimental
through `hyperfide.experimental_machines`.

Docs: [backend-machines.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-machines.md).

## Machine provider `ephemeral`

An optional boolean on `PluginMachineProviderDefinition`, false by default,
for disposable compute. After the last live thread and its machine launch are
gone, core requests removal regardless of environment retirement policy.

Docs: [backend-machines.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-machines.md).

## Project checkout ownership

The provider context's `projectCheckout.experimental_ownsPath` is a required
boolean whenever a checkout is present. It is true only for a checkout core
materialised at that exact path, derived from persisted source ownership and
never from caller inputs. Core's environment hook policy uses it to apply to
fresh machine clones and leave user-maintained attachments alone.

## Coordinated machine maintenance

`hyperfide.sdk.hosts.experimental_suspend({ hostId })` drains active work (bounded at
five minutes) and invokes the provider's suspend callback. It rejects with
`machine_busy` while a live launch, provisioning environment or checkout
preparation ties work to the host. Plugins own idle timing and preservation.

Docs: [backend-machines.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-machines.md), [backend-sdk.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-sdk.md).

## Thread-sequence and terminal-input notifications

`experimental_thread.events` delivers `{ thread, sequence }`, coalesced to one
notification per thread per second. `experimental_terminal.input` delivers
`{ terminal }` after real user input is forwarded. Neither exposes contents.

Docs: [backend-events.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-events.md).

## Thread parent-change notifications

`experimental_thread.parentChanged` delivers `{ thread,
previousParentThreadId }` after a `threads.update` moves or releases a
thread, or after core releases an archived thread's unarchived children (one
event each, after the archive commits). Threads below the moved one get no
event. Push-notifications uses it to republish the moved subtree's levels.

Docs: [backend-sdk.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-sdk.md).

## Host deletion notifications

`experimental_host.deleted` delivers the removed host's public DTO once,
fire-and-forget. A plugin not loaded at removal never sees it, so it must
also reconcile against a 404 from `hyperfide.sdk.hosts.get`.

Docs: [backend-events.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-events.md).

## Environment removal notification

`experimental_environment.removed` delivers `{ removal }` (environment id,
pre-removal host and path, provider ownership, removal time) after a
provider's removal is committed. Delivery is ephemeral, and provider success
does not guarantee the path was deleted.

Docs: [backend-events.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-events.md).

## `hyperfide.experimental_machines.getResource`

`getResource(hostId)` returns core's persisted host resource JSON, or null when
the host or resource is absent. Any plugin can read it, following the host
access model. It makes no vendor observation, and callers parse the
provider-specific data.

## Environment compositions

`hyperfide.experimental_environments.register({ id, displayName, description, icon, machineProviderId, environmentProviderId })`
declares a new-machine environment option backed by one concrete environment
provider. It supplies no lifecycle callbacks or inputs.

Docs: [backend-machines.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-machines.md), [frontend-components.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-components.md).

## Experimental host SDK machine operations

`hyperfide.sdk.hosts.experimental_create`, `experimental_getEnrollmentCommand`,
`experimental_listProviders`, `experimental_reconnect`, `experimental_suspend`,
`experimental_resume` and `experimental_retryCleanup` have no unprefixed
aliases. `hosts.delete({ hostId, force: true })` is the path core's machine
removal dialog and `hyperfide platform machine remove --force` use to finish a persistent
machine's removal whose environment cleanup failed while the machine is
disconnected; it skips on-machine environment cleanup, emits
`experimental_environment.removed` for each environment, and still runs the
provider's `remove`.
`experimental_getEnrollmentCommand` serves the in-memory manual setup command
and expiry only while that host is creating. Completion, removal, expiry and
server restart drop the command, which never enters persisted progress or
transcripts. `experimental_reconnect({ hostId })` returns
`{ command, windowsCommand, expiresAt, hostId }`, expiring commands that
re-enroll an existing machine.

Docs: [backend-machines.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-machines.md), [backend-sdk.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/backend-sdk.md).

## Moving the server (`hyperfide.sdk.experimental_server`, `hosts.experimental_deleteOldServerCopy`)

`experimental_server.checkMove({ targetHostId, serverUrl })` returns the
pre-move checklist (`ServerMoveCheckResponse`).
`startMove({ targetHostId, serverUrl, stopRunningWork: true, archiveExistingTargetServerData })`
freezes this server, copies its data to the target, starts the new server
there, switches machines over, and retires this process. `moveStatus()`
reports the active and last move. A move whose activation was never confirmed
reports `recovery_required` and keeps this server up and frozen. In direct
mode the status carries `destinationStatusUrl`. `cancelMove()` works until the
switch starts, and in `recovery_required` it rolls the switch back.
`export({ signal })` streams an unencrypted gzip archive with its `sha256`.
`hosts.experimental_deleteOldServerCopy({ hostId })` deletes the locked old
copy. All of them refuse machine credentials. Everything except status and
cancel requires the default-off `serverMove` experiment (403
`server_move_experiment_disabled`). CLI:
`hyperfide platform server move|export|import|unlock|delete-old-copy|install-machine-service`.

## `app.experimental_icons.register` and `experimental_Icon`

`app.experimental_icons.register({ name, component })` adds inline React
artwork to the shared app icon registry. Explicit registrations override
built-ins, and the first plugin id in lexical order wins collisions.
`experimental_Icon` renders a name with a `fallback` (default `Zap`).
Resolved explicit icon names win over plugin branding for per-item icons.

Docs: [frontend-components.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-components.md).

## `experimental_ProviderIcon`

The shared frontend renderer for agent, machine and environment provider
artwork. It takes `providerKind`, `provider` and an optional `fallback`
(default `Code`), and resolves slot overrides, then the logo mask, the glyph,
and the fallback, without fetching.

Docs: [frontend-components.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-components.md).

## `experimental_ThreadStatusGlyph` (`@hyperfide/plugin-sdk/app`)

Renders hyperfide platform's thread status glyph from `PluginThreadStatusGlyphProps`
(`indicator`, `archived?`, `rowStatus`, `hideIdleDraftLabel`, `size`); the
host owns icons, colors, and labels, and the caller resolves the indicator.
The SDK test fake exposes the props as `data-*` attributes.

**Core callers.** `ThreadStatusGlyph`
(`apps/web/src/components/thread/ThreadStatusGlyph.tsx`) is the SDK export
and the glyph in the thread search palette, the related threads info section,
and mobile recents. The thread-list plugin draws its rows and collapsed-group
rollups with it.

Docs: [frontend-registration.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-registration.md).

## `HostsArea.experimental_reconcile`

Explicitly reconciles a provider-managed machine with core's recorded state.
For a suspended machine it starts the provider's suspension operation and
returns HTTP 202; callers poll host status for completion. Active and
transitional states are left unchanged. Provider suspend and resume must be
idempotent. CLI: `hyperfide platform machine reconcile`.

## Lifecycle ownership on thread creation

`hyperfide.sdk.threads.spawn` and `hyperfide.sdk.threads.fork` accept
`lifecycleOwnerThreadId`, and thread responses expose its nullable value.
These are data fields on existing SDK methods, so they carry no prefix.

## Environment provider existing-path selection

`PluginEnvironmentProviderDefinition.experimental_existingPath(inputs)` returns
an absolute path or null from parsed inputs, without mutating anything. On an
existing machine, a usable environment of that project at the path is reused
with its provider, ownership, resource and cleanup identity intact. An
unusable one is refused, and a missing one goes through normal creation.
Other projects' managed paths remain forbidden.

## Composer editing: `insert` and `replace`

Docs: [frontend-hooks-and-ui.md](../../../tools/hyperfide/plugins/hyperfide-guide/skills/hyperfide-plugin-authoring/references/frontend-hooks-and-ui.md).

## Thread creation placement

`PluginSidebarThreadActions.openNewThread` accepts `experimental_placement`
(`{ sectionId: string | null, pinned: boolean }`), which overrides the legacy
section option. Omitting it clears prior composer placement and uses the
legacy section or the general list, unpinned. The composer sends this
placement with normal and scheduled creation.

## Provider discovery metadata

`package.json` → `hyperfide.experimental_providers` statically declares
`{ kind, id, displayName }` provider identities before a plugin runs. Only
`kind: "agent"` is accepted. It keeps a disabled plugin's agents discoverable
in Settings → Providers, and grants no runtime capability. Add `"environment"`
or `"machine"` only together with a consumer.

## Global prompt history (`hyperfide.sdk.experimental_promptHistory`)

`list({ cursor?, limit?, signal? })` returns `{ entries, nextCursor }`, every
accepted user prompt newest first, each with `id`, `createdAt`, `input`,
`projectId`, `threadId`, and `scope` (`"project"` for a prompt that started
a top-level thread, `"thread"` for a follow-up). `limit` is a digit string (default 100, max 1000).
A page can be short while `nextCursor` is set, because unparseable rows are
skipped. The same route backs `hyperfide platform prompt-history list`.

## `ThreadChatMessageReference.experimental_messageSeq`

The message reference handed to `messageAction` runs, `ThreadChat` consumer
message actions, and prose selections carries the event sequence that recorded
the message. It is the `msg` value of a message link and the seq accepted by
`sdk.threads.message` and `hyperfide platform thread log --message`, so a plugin can build or
resolve a message link without guessing. It equals `sourceSeqEnd` except for a
steer, which is recorded by its request and shown at its acceptance.

## `PluginHyperfideSdk.plugins.experimental_startUpdate` and `experimental_updateJobs`

Starts a server-owned plugin update and returns its job at once;
`experimental_updateJobs.list/get` report queued/running phases and the
terminal update, rollback, or failure. Jobs survive client disconnects, stay
ten minutes after finishing, and live in memory only. `applyUpdate` keeps its
result contract by polling the job; raw callers without
`Prefer: respond-async` keep the synchronous response. Running updates cannot
be cancelled during activation or rollback.

## `PluginHyperfideSdk.system.experimental_cliCommand`, `experimental_installCliCommand`, and `experimental_removeCliCommand`

`experimental_cliCommand()` reports whether `hyperfide-platform` is on PATH on the server's
computer: `supported` (false without an installed hyperfide-app launcher),
`installedPath`, `onPath`, and `shadowedBy` (`source` `npm`, `machine`, or
`other`). Install links `<dataDir>/bin/hyperfide-platform` into a PATH folder (on Windows,
adds `<dataDir>\bin` to the user PATH) and refuses to replace another `hyperfide-platform`
(`409 cli_command_blocked`); install and remove return the new status, fail
with `409 cli_command_unsupported` when unsupported, and 403 for machine
credentials. Core callers: Settings → General → Command line
(`CliCommandSettingsSection`) and `hyperfide platform settings cli-command`.

## `PluginHyperfideSdk.hosts.experimental_discoverRepos`

`hosts.experimental_discoverRepos({ hostId })` lists up to 10 git
repositories under the machine's home directory with local activity in the
last 30 days, newest first, each with `path`, `name`, `lastActivityAt`,
`originUrl`, and the bound `projectId` or null; `truncated` reports an
exhausted three-second walk budget. The walk skips dot, build, and scratch
directories, linked worktrees, and submodules. Core callers: the setup guide
(`OnboardingFlow`) and `hyperfide platform project discover`; it rides the
`host.discover_repos` daemon command.

## Shared VCS process admission (`@hyperfide/plugin-sdk/host`)

`experimental_runVcsProcess(command, args, operation, options)` admits a `git`
or `gh` operation to the runtime's shared queue before invoking it. `args` are
the command's arguments; they classify git network subcommands (`fetch`,
`pull`, `push`, `clone`, `ls-remote`, `submodule update`, `remote update`).
Arguments without a network subcommand, including an empty list for pipelines,
admit to the local pool. The call holds the permit until the returned promise
settles; callers must await child close and termination cleanup. Calls nested
inside an admitted operation run under the outer permit without queueing again.
`ExperimentalVcsProcessOptions` accepts `priority` (`interactive`, `status`,
`background`), an optional cancellation signal, and `slots` for pipelines
that start multiple git children. Queue cancellation prevents spawning; callers
own cancellation of running children. Execution timeouts begin after admission.

The runtime keeps two disjoint pools. The local pool admits
`clamp(os.availableParallelism() - 2, 4, 6)` local git children, computed once
per runtime: background work can fill two fewer slots and status work one
fewer, so eight or more cores give six, five, and four. The
network pool admits at most four network git or `gh` children, with at most two
`gh`: background work can fill two slots and status work three. Waiters gain one
priority level every 15 seconds. FIFO applies within effective priority in each
pool: when the first waiter that is not held back by the GitHub lane needs more
slots than its pool has free, later waiters in that pool wait behind it, so
pipelines are not starved by single-slot work. A blocked GitHub waiter does not
block eligible network git work, and neither pool blocks the other. The budget
is shared across module copies in one Node runtime, including bundled plugins.
Separate host daemons and server processes own separate budgets; arbitrary
commands run by agents or terminals are outside this queue.

Core buffered git/gh reads, streaming status/diff reads, blob reads, shell
pipelines, watcher setup, environment provisioning, and the GitHub plugin use
this path. The CLI and SDK's workspace reads and mutations reach the same host
handlers. Host commands give explicit user mutations and diff panels interactive
priority, status reads status priority, and PR polling/watcher setup/provisioning
background priority.

## Navigation-panel route sidebar

`PluginNavPanelRegistration.experimental_sidebar` is an optional component
receiving `PluginNavPanelProps`. `AppLayout` resolves one route sidebar per
location with `resolveRouteSidebar`: Settings, Plugins and Skills are core
entries, and a live navigation panel with `experimental_sidebar` is a plugin
entry. `AppLayoutSidebar` mounts whichever entry resolves in the alternate
sidebar body, holds the outgoing entry while a compact drawer closes, and
retains the native rail and mounted thread-list state. Every entry renders in
`SectionSidebar`, which owns the top chrome and history controls, the "Back to
app" row and title in the top-list layout, compact-drawer dismissal for SDK and
delegated-link navigation, and the resize handle. Panel sidebar routes are
section routes in `useAppSettingsRouteMemory`, so Back to app and the `app.back`
command return to the last core app route, as from Plugins and Skills. A crash
shows the plugin crash notice inside the section until the subPath or plugin
generation changes. Registration and route switching change no setting, thread,
draft or execution state.

Docs: Plugin Guide Full-page panels surface. The API is generic; no plugin-id
special case exists in core. CLI navigation uses the existing panel URL; this
field introduces no separate backend or headless side effect.

## Plugin contract live reads and writes

- `experimental_useRpcQuery`, `experimental_useRpcKeyedQuery`, `experimental_useRpcMutation`, and `experimental_useRpcRefresh` use the core `@hyperfide/query-tags` cache mechanism per client. Query state keeps previous data during refresh errors; consumers show an inline Retry using `refetch`. Successful reads clear the error. Keyed calls accept `{ keys }` and return a record containing every requested key; consumers share one observer per plugin/method and requests batch up to 200 keys. A pure `select` projection retains unchanged JSON subobjects across key and fetch-state changes, so default rows do not republish unchanged notification levels.
- `ExperimentalRpcMutationState.pendingInputs` contains every in-flight input for that hook instance until the server acknowledges its write. Background rereads are excluded. `isPending` remains true while any input is pending. Core `useTaggedMutation` exposes the same state from TanStack's mutation cache.
- `ExperimentalRpcMutationOptions.optimistic(input, cache)` predicts loaded cached values before the write. `cache.update(method, input, updater)` targets a plain read, and `cache.updateKey(method, key, updater)` targets one keyed row. `cache` is an `ExperimentalRpcOptimisticCache`. Pure updaters receive the currently visible value and return its prediction. An absent plain read is skipped; a keyed row that loads while the write is pending shows the prediction. Predictions are shared by every reader on that client. The core route mutation accepts the same core optimistic descriptors and uses the same `@hyperfide/query-tags` implementation. Ui-preferences already updates its shared Jotai atoms optimistically, which remains unchanged.
- Predictions remain over incoming reads until confirmed matched reads arrive, independently of mutation pending state. A rejected write rolls back its layer and surfaces the mutation error. An accepted write whose refresh fails keeps its prediction with the read error and Retry; a successful refresh replaces it. Later predictions and confirmed values are protected from older mutation settles and rollbacks. Inverted completion retains the later confirmed value until a subsequent fresh read can replace it. Keyed rows live in the `@hyperfide/query-tags` keyed store: an accepted row prediction becomes the row's value, and only a read of that row that started after acceptance replaces it.
- `defineRpcContract` accepts the core `ContractTagDeclarations`: `tags(input, output)`, `keyTags(key, value)`, and `invalidates(input, output)` receive validated values. Tags are in the RPC envelope; successful writes broadcast their invalidated tags and return `invalidated`. Tagged mutations resolve at write acknowledgement while matching active reads refresh in the background.
- `hyperfide.rpc.experimental_invalidate(tags)` requires nonempty tags; `experimental_invalidateAll()` refreshes all reads of that plugin, including untagged reads. Load, reload, disable and reconnect refresh plugin reads through the same cache path. Plugin changed messages are scoped to `{ kind: "plugin", pluginId }` subscribers; reconnect batches subscriptions and unknown targets are ignored.
- Core uses the same tagged reads, invalidation and mutation implementation for UI preferences. Core keeps each thread once, in the same `@hyperfide/query-tags` keyed store as plugin keyed rows, and derives thread detail, every thread list, search and sidebar navigation from it as keyed views. Every core thread write (pin and unpin through one batcher, read state, rename, section and parent moves, archive, unarchive, delete, stop, send status, environment rename) predicts on the entity with `optimisticKeyedUpdate` through `beginOptimisticMutation`, the path `cache.updateKey` uses, and confirms at the server's acknowledgement. One contract table (`@hyperfide/query-tags/testing`) runs against core thread entities and plugin keyed rows; the pin hooks add `useTaggedMutation` with `meta.errorMessage` for core's error toast. Push-notifications thread levels are the first plugin caller; delivery retains `useRealtime`. Core's composer banner, Info row and the retired sidebar pull request member call `experimental_useRpcKeyedQuery` and `experimental_useRpcMutation` with an explicit `pluginId: "hyperfide--github-pr-status"` outside any plugin slot, the calls another plugin makes, so that UI can move into the plugin unchanged; the hooks require `pluginId` only when no plugin slot provides one. Cache seeding remains outside this API.

## Page titles

`PluginNavPanelRegistration.experimental_usePageTitle(props)` returns
`ExperimentalPageTitle | null`: either `title` or
`breadcrumbs: { label, to? }[]`, with an optional `windowTitle` override.
The host calls this React hook in each pane's plugin scope with its own
`subPath`. Content appears after the panel icon/name. Breadcrumb links must
be absolute app routes starting with a single slash. The focused pane supplies
the window title, defaulting to the leaf/title combined with the panel name.
Null restores defaults. A title hook failure clears the title without crashing
the body; navigation and plugin reload retry it. The host isolates hook
lifecycles by pane, route and plugin generation.

Core Tools, Skills and Plugins use the same hook host and title provider;
Automations registers its own hook and shares its detail query with the body.
The Plugin Guide full-page panels card owns the API documentation.
`renderSlot` evaluates the registered hook in its slot scope and exposes its
current result as `RenderedSlotInspectionState.experimental_pageTitle`.

## Plugin watched reads

The GitHub PR Status plugin's keyed `pullRequests` read is a producer: it re-checks a watched environment every 30 seconds while checks are pending, auto-merge is on, the PR is queued or mergeability is unknown, and after a thread on the environment goes idle or the environment's refs, status or metadata change. It invalidates the row's key tag only when the row changed.

`hyperfide.rpc.experimental_whileWatched(method, handler)` starts one producer for each distinct subscribed `{input}` or `{key}` read, and aborts it when its last client leaves or the plugin unloads. An app tab hidden for five seconds stops counting as a client until it is visible again. Register after the RPC method. Read identity uses canonical JSON and the encoded read is capped at 16 KiB. The server drops an oversized target from a subscribe message and keeps the rest, and the app doesn't send one. Every live app query hook holds a `plugin-read` subscription for its read, whether or not the method has a producer. Producers start and stop through the hub's watched-target registry, the path core uses for daemon git and thread-storage watching, thread timeline pushes and environment pull request checks.

## Sidebar pull request compatibility

The runtime retains `experimental_useSidebarThreadPullRequest(threadId)` for
installed plugins, while published `@hyperfide/plugin-sdk/app` declarations omit
it. It reads the GitHub PR Status plugin's `pullRequests` row for the thread's
environment and returns `{ isLoading: false, pullRequest: null }` when the plugin
is disabled or the read fails. Plugin builds warn with the replacement call.

## Unified tabs (`experimental_tab`)

- **Surface:** `app.slots.experimental_tab`, `ExperimentalTabRegistration`, `ExperimentalTabContext`, `ExperimentalTabReference`, `ExperimentalTabTargetReference`, `ExperimentalThreadActionTabProps`, `ExperimentalThreadTabActionContext`, and `ExperimentalAppPanel.experimental_openTab`. Context is thread (`threadId`) or an owned navigation panel (`panelId`, component `subPath`). Availability is fixed or action. Only actions accept `run`, default `params` and per-open persisted params/title; navigation-page fixed tabs retain validated session targets. `tooltip` defaults to title and layout is padded or flush.
- **Behavior:** one registry and destination dispatcher serve thread fixed, thread action, navigation-page fixed and navigation-page action tabs. `useVisible` runs inside plugin scope/crash isolation for the current context. It hides a destination or launcher without removing saved fixed identity/order or already-open action instances. Action params deduplicate instances; fixed tabs have one identity and cannot close. Existing `openFixedTab`, action `openPanel`, message-action openers and `openThreadPanel` delegate through the shared dispatcher. New-thread-screen actions retain their project-context API and use the same action-opening implementation.
- **Compatibility:** `threadPanelAction`, `PluginThreadPanelActionRegistration`, `PluginThreadPanelProps`, `PluginThreadPanelActionContext`, `navPanel.fixedTabs`, `PluginFixedTabRegistration` and `PluginFixedTabDeclaration` are removed from public source typings. Runtime adapters in the registration collector remain for published bundles and Git-source install/update builds. Legacy optional icons, unknown fields and older fixed-tab owner shapes are accepted; malformed entries warn/skip rather than crashing the plugin. `hyperfide-plugin-build` emits migration warnings with replacement examples and the authoring migration link. Server install/rebuild paths copy warnings into the plugin log; warnings never block install or update. The runtime export manifest continues to expose existing SDK functions.
- **Persistence and automation:** thread fixed entries use `{id, kind: 'plugin-fixed', pluginId, tabId}` through SDK `threads.tabs.get/update` and CLI `hyperfide platform thread tabs show/set`, with revision checks. Existing `plugin-page-fixed` and `plugin-panel` action entries/params keep their IDs and storage keys. Selection is client-local. Legacy core Info/Diff kinds, active selections and split IDs convert without reordering; server entries rewrite through the revision-checked queue. Split restoration waits for registration/visibility readiness.
- **Core callers:** Info and Diff register as thread + fixed and use the shared content host, visibility, descriptor and open paths. Diff retains shortcut, Git setting, file/commit navigation and review state. Core-only Info metadata and Diff controllers remain pane-owned contexts; they are not public plugin API. Docs, GitHub, Side Chat, Tasks, Workflows and the ThreadChat demo use the unified registration.
