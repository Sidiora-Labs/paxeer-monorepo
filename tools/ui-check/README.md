# UI check

Render a web surface with Chromium, save full-page screenshots, and measure
loading, layout and accessibility across device sizes. Screenshots and the JSON
report go outside the repository. A clean report supports a visual review of the
screenshots for alignment, spacing, overlap, token use and other design details.

Install and run from this directory:

```sh
npm ci
npx playwright install --with-deps chromium
npm test
```

The workflow at `.github/workflows/ui-check-test.yml` runs these same commands
from this directory for changes to the render check and its workflow.

Only `playwright`, `axe-core` and the PNG decoder `pngjs` are dependencies, at
the exact versions in `package.json`. The browser and its system packages are
installed separately.
Tests use Node's test runner, local static pages and real Chromium.

From the repository root:

```sh
node tools/ui-check/check.mjs --surface /path/to/surface.json --out /tmp/surface-renders
node tools/ui-check/check.mjs --surface /path/to/surface.json --out /tmp/surface-renders --view home
node tools/ui-check/check.mjs --surface /path/to/surface.json --write-geometry
node tools/ui-check/check.mjs --list
```

`--surface` selects a JSON file. `--out` selects an output directory outside the
repository, including when resolved through symlinks. `--view` selects one view
by its exact name. `--list` prints the device sizes without starting a surface.
`--jobs COUNT` sets the maximum number of concurrent renders and requires a
positive whole number. It defaults to the number of processors available to the
process, capped at four. `--jobs 1` renders sequentially. Each render has its own
browser context; render order, screenshot names, findings and view summaries
keep matrix order for every job count. Unknown options and unknown view names
are errors.

Each view, including its default state and every named state, gets 34 renders
when it uses all modes:

- Widths 360, 390, 430, 768, 834, 1024, 1280, 1440, 1920 and 2560 at height 900,
  plus landscape 844 by 390, in both light and dark schemes.
- At 390 and 1440, both schemes also get reduced-motion, doubled-text and
  keyboard renders. These modes run separately. Doubled text multiplies computed
  font sizes by two. Keyboard renders use Tab through the focus sequence and
  record each focused selector.

Every render has a numbered PNG and an entry in `result.json`, with the view,
state, width, height, scheme, mode, render status, `hasContent`, `cancelled`,
`retried`, `firstDeadlineStep` and findings. `retried` says whether the render
runs a second time, and `firstDeadlineStep` names the step where its first
render reaches the deadline, or is `null` when it runs only once. The report's
`retriedCount` counts renders that run a second time. Each view summary and the
Total line append that count as `N retried` after their existing counts.
`hasContent` is true only when the document has visible content and the viewport
screenshot contains more than one colour. Findings include that context, a
selector and measured values. A view summary and a total print to stdout, each
including the count of renders that showed nothing. The total also counts
cancelled requests across the run. The view summary also counts renders that did
not render. The report also lists the modes for each view and
named state. The total line counts views and named states using fewer than all
four modes, counting the initial state as its view. Exit status is zero only when
every requested render succeeds and every finding belongs to `reportedKinds`.
Without that list, any finding fails the command. Startup, navigation and
screenshot errors produce a nonzero exit.
A surface that cannot start has a surface-level load finding with null render
context. Invalid arguments or surface files are rejected before rendering.

When a render reaches its deadline, the check force-closes its browser, starts
a new browser and renders the same view, state, size, scheme and mode once more
with the same bound. Only the second render's findings, measurements and
screenshot count, using the same screenshot name and matrix position. Geometry
measurements and `--write-geometry` also use that render. If it succeeds without
findings, the retry adds no finding and does not fail the check. If it reaches
the deadline again, its deadline finding stands and there is no third render.
Other causes do not trigger a retry. The 300000-millisecond bound per render,
`--jobs` concurrency and finding kinds stay the same.

## Surface files

Paths for the directory and answer files are relative to the surface file.
The directory may also be absolute. A typical surface file has this form:

```json
{
  "name": "Example surface",
  "directory": "../../../apps/example",
  "build": "npm run build",
  "buildMs": 180000,
  "outputs": ["dist"],
  "start": "npm run start -- --port 4173",
  "address": "http://localhost:4173",
  "readyPath": "/",
  "views": [
    {
      "name": "home",
      "path": "/",
      "states": [
        {
          "name": "search",
          "steps": [
            { "action": "fill", "selector": "input[type=search]", "value": "receipt" },
            { "action": "press", "selector": "input[type=search]", "key": "Enter" },
            { "action": "click", "selector": "button[type=submit]" },
            { "action": "wait", "text": "Results" }
          ]
        }
      ]
    }
  ],
  "answers": [],
  "embeddedMessages": [
    { "script": "/vendor/chart/", "message": "Chart data is unavailable" }
  ],
  "typeScale": [12, 14, 16, 20, 24, { "min": 32, "max": 48 }]
}
```

`name`, `directory`, `address`, `readyPath`, `views`, `answers` and `typeScale`
are required. View and state names must be unique within their lists; `default`
is reserved for the initial state. View and readiness paths start with a single
slash and stay on the surface's origin. Each named state starts from a fresh
page and performs its own steps in order. `click`, `fill` and `press` use
Playwright selectors; `press` takes a Playwright key name. `wait` waits for visible
text containing its `text` value. States and build/start commands are optional.

Views and named states may declare `modes`, a nonempty list of unique names from
`default`, `reduced-motion`, `double-text` and `keyboard`. Every list must include
`default`. A view without `modes` uses all four; a state without `modes` inherits
its view's list. An explicit state list replaces the view's list. Renders use
only the matrix configurations in that list, in matrix order. For example,
`"modes": ["default"]` gives 22 configurations per view or state. Invalid lists
are rejected with the view's name.

Commands run in `directory`. The optional `buildMs` is a whole number from 1000
to 1800000 milliseconds, inclusive. It bounds the build; without it the bound is
120000 milliseconds (120 seconds). Invalid values are rejected with the surface
name, and a build timeout message names the applied bound.

The optional `outputs` is a list of one to sixteen nonempty paths relative to
`directory`, and requires a `build` command. Absolute paths, paths outside the
directory, the directory itself, and paths containing a `node_modules` component
are rejected with the surface name. The same restrictions apply after resolving
symlinks, including links in parent directories of outputs that do not yet exist.
Before building, the check recursively removes only the named outputs; absent
outputs are allowed. A named symlink is removed without removing its target.
The order is remove outputs, build, start. Without `outputs`, nothing is removed.
The report records the applied `buildMs` and an `outputs` list of the named paths
that were present and removed, including when the build fails.

The start command stays running while readiness is polled for up to
15 seconds. Navigation and state steps have a 10-second bound. Managed processes
and their children stop when the check finishes. The surface file contains
local commands, so use a file whose commands you intend to run.

For static pages, omit `start`. The command serves `directory` itself at the
loopback HTTP `address`; port zero chooses an available port. `readyPath` must
name an existing file or directory index. Files outside that directory,
including symlink targets, are not served. An optional `build` still runs first.

A fixed answer entry has these fields:

```json
{
  "method": "GET",
  "pattern": "http://localhost:4174/api/*",
  "file": "answers/items.json",
  "status": 200,
  "headers": {
    "content-type": "application/json",
    "access-control-allow-origin": "*"
  }
}
```

`method`, `pattern` and `file` are required. Patterns match the entire HTTP(S)
address; `*` matches any sequence of characters, and all other characters are
literal. The file must be beside the surface file or in one of its subdirectories.
Status defaults to 200. Headers and status are optional. Supply the response's
content type and CORS headers when the page needs them. Match preflight OPTIONS
requests separately where applicable. Fixed answers replace requests to other
origins inside the browser. Unmatched requests are blocked and reported.
External WebSockets are blocked and reported; same-origin WebSockets are
connected. Service workers are blocked so requests remain observable.

Views and named states may also contain an `answers` list with the same fields
and file rules. During a named-state render, the first matching answer is chosen
from state answers, then view answers, then surface answers. The initial state
uses view answers before surface answers. Each view and state keeps only its own
list, so an answer does not carry into another view or state. The report gives
each view and state the count of entries in its own list. For example, a sign-in
view can refuse one session request while signed out and admit that same address
after a signed-in state supplies two answers:

```json
{
  "name": "sign-in",
  "path": "/sign-in.html",
  "answers": [],
  "states": [
    {
      "name": "signed-out",
      "steps": [],
      "answers": [
        { "method": "GET", "pattern": "https://api.example.test/session", "file": "answers/refused.json", "status": 401 }
      ]
    },
    {
      "name": "signed-in",
      "steps": [],
      "answers": [
        { "method": "GET", "pattern": "https://api.example.test/session", "file": "answers/session.json" }
      ]
    }
  ]
}
```

A view may declare `expects` for its initial state, and each named state may
declare its own `expects` list. Every entry requires an uppercase `method`, a
`pattern` using the same whole-address syntax as fixed answers, and an integer
`status` from 400 to 599. For example, a sign-in view may declare:

```json
"expects": [
  { "method": "GET", "pattern": "http://localhost:4173/api/session", "status": 401 }
]
```

Each entry must match a response during that render. A matching response and
the browser's failed-resource console line for it do not produce load findings.
An entry without a matching response is a load finding that names its method,
pattern and status. Undeclared failed responses and application console errors
remain findings. View expectations apply only to the initial state; named
states declare their own expected refusals. Expectations do not supply answers.

The optional surface-level `embeddedMessages` list declares exact console messages
from scripts embedded unchanged from another party. Each entry contains only
`script`, an origin-relative path with at least one directory (such as
`"/vendor/chart/"` or `"/vendor/chart/bundle.js"`), and `message`, a nonempty string.
Invalid lists and entries are rejected before rendering, with the entry index.
A console message of type `error` matches only when its location has the surface's
origin, its URL pathname begins with `script`, and its text equals `message`.
Each render records matches in `embeddedMessages` as `{ "script", "message",
"count" }`, and the view summary appends `N declared messages` when the count is
nonzero. Unmatched declarations add no finding. The same text outside that path
or origin, other text under that path, and uncaught errors or rejections remain
load findings.

Each browser context grants clipboard writing to the surface's origin so copy
controls can complete their normal interaction. Clipboard reading is ungranted.

`typeScale` contains positive CSS pixel sizes or inclusive `{ "min", "max" }`
fluid ranges. Doubled-text renders compare half the computed size with the
scale. A 0.05-pixel comparison tolerance accounts for browser size rounding.
Unknown fields are rejected at every structured level, including fields for
limits or exemptions. Surface files cannot alter a finding's limits or omit its
measurements.

## Stored geometry

A surface may declare `geometry` with a `file` path relative to the surface file
and optional `widths`. Widths are unique positive whole CSS pixel numbers; they
default to 390, 1280, 1440, 1920 and 2560. Each view may declare `parts`, a list
of unique names and Playwright selectors:

```json
"geometry": { "file": "example/geometry.json", "widths": [390, 1280, 1440, 1920, 2560] },
"views": [
  {
    "name": "home",
    "path": "/",
    "parts": [
      { "name": "header", "selector": "header" },
      { "name": "summary", "selector": "#summary" }
    ]
  }
]
```

`--write-geometry` renders every view with parts in its initial state, in default
mode and dark scheme, at the geometry widths and height 900. Each selector must
match exactly one visible element. The command stores the element's bounding
box (`left`, `top`, `width`, `height`), computed `fontSize` and computed
`lineHeight` in CSS pixels. A computed line height of `normal` keeps that keyword.
Edges are relative to the viewport after settling and returning to the top.
The file contains version 1 and entries sorted by view name, numeric width and
part name, without rounding the measurements:

```json
{
  "version": 1,
  "entries": [
    {
      "view": "home",
      "width": 390,
      "part": "header",
      "selector": "header",
      "values": { "left": 0, "top": 0, "width": 390, "height": 64, "fontSize": 16, "lineHeight": 24 }
    }
  ]
}
```

Writing requires `geometry` and at least one selected view with parts. It saves
only the geometry file: no PNGs, result report or output directory. `--out` is
optional and produces no files in this mode. Named states and views without
parts do not render. `--view NAME` replaces only that view's entries and preserves
the entries of other views. Loading errors and missing or ambiguous parts fail
the write and leave the stored file intact. Presentation findings do not prevent
capturing the geometry.

Without `--write-geometry`, a surface with `geometry` reads the existing file and
measures parts on every run in the same initial state, mode, scheme and sizes.
Widths outside the regular device list add renders; matching regular renders
provide the measurements without duplication. Other modes, schemes and named
states keep their regular checks. A difference greater than one CSS pixel in
any measurement produces one `geometry` finding per part and width. A change
between `normal` and a numeric line height also produces a finding. Exactly one
pixel is allowed. A listed part without a stored entry and a stored part absent
from the page each produce a finding. Stored selectors keep parts measurable
even when their declaration is removed. The report gives the view, width, part,
selector, stored values and measured values; missing values are `null`. Normal
runs leave the geometry file unchanged. Invalid geometry files are rejected
before rendering.

## Reported kinds

The optional surface-level `reportedKinds` list contains unique names from
`sideways`, `cut`, `contrast`, `touch`, `focus`, `shift`, `motion` and `scale`:

```json
"reportedKinds": ["contrast", "touch", "scale"]
```

These findings retain their complete context in the render and report findings
and contribute to the finding count in every summary. With this list, each view
summary and the total line also give separate `reported` and `failing` counts.
The JSON report contains `reportedKinds`, `reportedCount` and `failingCount`.
The exit status is zero when every requested render succeeds and the failing
count is zero. `geometry` and `load` cannot appear in the list: geometry changes,
surface startup failures, render failures and all other loading findings remain
fatal. Unknown, duplicate or forbidden kinds are rejected before any surface
command or browser render. Omitting both `geometry` and `reportedKinds` preserves
the regular device matrix, report and exit behavior.

## Measurements

| Kind | Finding |
| --- | --- |
| `load` | Undeclared failed HTTP response, expected refusal without a matching response, failed request other than a page-cancelled request, console error, uncaught error or rejection, unmatched external request, startup or render error. A document without visible text of an element's own, media with an area (image, SVG drawing, canvas, video or embedded frame) or controls, or a viewport screenshot containing a single colour, also produces a finding. Text clipped to nothing does not count as content. The finding names the document or screenshot measurement. A render has a 300000-millisecond deadline across navigation, settling, measurement, contrast, keyboard traversal and screenshots. On the first deadline, its browser is force-closed and the same render runs once more in a new browser with the same bound; only the second outcome counts. A second deadline produces a finding with `source: "deadline"` and the last started `step`; the render is marked not rendered and its browser is force-closed. The per-render bound, `--jobs` concurrency and finding kinds stay the same. A matching declared refusal and its browser failed-resource console line are allowed. Page-cancelled requests add to the render's `cancelled` count without a finding; a cancelled document navigation remains a finding. Requests ended by the check produce only their original finding. |
| `sideways` | Document wider than the viewport, or visible element extending past either side or wider than the viewport. Text clipped to no area is excluded. Descendants of horizontal scroll containers are measured within those containers, whose boxes are measured against the viewport. Element geometry has a 0.5-pixel rounding tolerance. |
| `cut` | Text with hidden or clipped overflow exceeds its box by more than 1 pixel, without declared ellipsis. Scrollable text is allowed. |
| `contrast` | Axe measures text below 4.5:1, or large text below 3:1 (24 pixels, or bold at approximately 18.67 pixels). A custom axe check measures visible control borders, identifying fills and SVG fills/strokes below 3:1. Disabled controls are excluded. |
| `touch` | Enabled touch control below 44 CSS pixels in either direction at widths up to 834. An inline link within a line of text is exempt. |
| `focus` | In keyboard mode, Tab focus produces no visible change (comparing computed appearance and viewport pixels) on the element, its descendants or ancestors, or fails to reach a sequentially focusable element. While a modal dialog is open, expected elements are limited to the topmost modal, including controls through shadow roots; nonmodal dialogs leave the expected elements unchanged. A frame is recorded as one stop without judging its focus mark or controls inside its document. Tab continues to the following top-document control, using at most 32 additional presses and 2000 milliseconds per frame. If focus does not leave the frame within either bound, one focus finding names the frame and traversal ends there. Appearances are compared with an unfocused baseline. In each direction, a box that fits must stay entirely in the viewport; a larger box must keep its starting edge in the viewport after focus. |
| `shift` | Sum of layout-shift entries without recent input, from initial painting through settling, exceeds 0.05. |
| `motion` | Under reduced motion, a running animation or transition lasts over 100 milliseconds, including iterations. Start events preserve measurements of motion that finishes before settling. |
| `scale` | Visible text's computed font size is outside every declared step or fluid range. |
| `geometry` | A named part differs from its stored box or text metrics by more than one CSS pixel, has no stored entry, or is absent or ambiguous on the page. |

The command waits for fonts, visits the full document to load lazy content,
returns to the top and observes a further 600 milliseconds before measuring.
State steps and the doubled-text mode discard the layout movement caused by
those intentional changes before that observation interval. Findings concern
the render and observation interval. Visual review of the screenshots covers
qualities that these measurements do not establish.

Requests may still be pending when their render finishes, reaches its deadline
or closes its browser context. Rejections from routing replies and WebSocket
answers after that point are discarded, and the check finishes with its report
and the exit status of its findings. While a render is active, requests the page
cancels (`net::ERR_ABORTED`) add no finding and increment that render's `cancelled`
count, which is present even when zero. A cancelled document navigation remains a
`load` finding with source `requestfailed`. The check remembers requests it ends:
unmatched external requests (`blockedbyclient`) have one `unanswered-request`
finding, and requests to the surface that cannot be fetched (`failed`) have one
`requestfailed` finding. Their subsequent browser failures and corresponding
failed-resource console lines add no second finding or cancellation count. Every
other failed request remains a `load` finding with source `requestfailed`. The
total summary line reports the sum of `cancelled` across all renders.

## Adding a surface

Add one JSON file under `tools/ui-check/surfaces` with its answer files alongside
it. Declare the surface's actual build and start commands, readiness path, views,
states and type scale. Static surfaces declare their served directory instead.
Include every route and meaningful state needed for the surface review. Add
fixed answers for its external requests. Run the command with an external output
directory, address each finding in the surface, and inspect all screenshots.
