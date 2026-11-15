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

Only `playwright` and `axe-core` are dependencies, at the exact versions in
`package.json`. The browser and its system packages are installed separately.
Tests use Node's test runner, local static pages and real Chromium.

From the repository root:

```sh
node tools/ui-check/check.mjs --surface /path/to/surface.json --out /tmp/surface-renders
node tools/ui-check/check.mjs --surface /path/to/surface.json --out /tmp/surface-renders --view home
node tools/ui-check/check.mjs --list
```

`--surface` selects a JSON file. `--out` selects an output directory outside the
repository, including when resolved through symlinks. `--view` selects one view
by its exact name. `--list` prints the device sizes without starting a surface.
Unknown options and unknown view names are errors.

Each view, including its default state and every named state, gets 34 renders:

- Widths 360, 390, 430, 768, 834, 1024, 1280, 1440, 1920 and 2560 at height 900,
  plus landscape 844 by 390, in both light and dark schemes.
- At 390 and 1440, both schemes also get reduced-motion, doubled-text and
  keyboard renders. These modes run separately. Doubled text multiplies computed
  font sizes by two. Keyboard renders use Tab through the focus sequence and
  record each focused selector.

Every render has a numbered PNG and an entry in `result.json`, with the view,
state, width, height, scheme, mode, render status and findings. Findings include
that context, a selector and measured values. A view summary and a total print
to stdout. Exit status is zero only when every requested render succeeds with
no findings. Startup, navigation and screenshot errors produce a nonzero exit.
A surface that cannot start has a surface-level load finding with null render
context. Invalid arguments or surface files are rejected before rendering.

## Surface files

Paths for the directory and answer files are relative to the surface file.
The directory may also be absolute. A typical surface file has this form:

```json
{
  "name": "Example surface",
  "directory": "../../../apps/example",
  "build": "npm run build",
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

Commands run in `directory`. The build must finish successfully within 120
seconds. The start command stays running while readiness is polled for up to
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
literal. The first matching method and pattern wins. Status defaults to 200.
The file must be beside the surface file or in one of its subdirectories.
Headers and status are optional. Supply the response's content type and CORS
headers when the page needs them. Match preflight OPTIONS requests separately
where applicable. Fixed answers replace requests to other origins inside the
browser. Unmatched requests are blocked and reported. External WebSockets are
blocked and reported; same-origin WebSockets are connected. Service workers are
blocked so requests remain observable.

`typeScale` contains positive CSS pixel sizes or inclusive `{ "min", "max" }`
fluid ranges. Doubled-text renders compare half the computed size with the
scale. A 0.05-pixel comparison tolerance accounts for browser size rounding.
Unknown fields are rejected at every structured level, including fields for
limits or exemptions. Surface files cannot alter a finding's limits or omit a
finding kind.

## Measurements

| Kind | Finding |
| --- | --- |
| `load` | Failed HTTP response or request, console error, uncaught error or rejection, unmatched external request, startup or render error. |
| `sideways` | Document wider than the viewport, or visible element extending past either side or wider than the viewport; element geometry has a 0.5-pixel rounding tolerance. |
| `cut` | Text with hidden or clipped overflow exceeds its box by more than 1 pixel, without declared ellipsis. Scrollable text is allowed. |
| `contrast` | Axe measures text below 4.5:1, or large text below 3:1 (24 pixels, or bold at approximately 18.67 pixels). A custom axe check measures visible control borders, identifying fills and SVG fills/strokes below 3:1. Disabled controls are excluded. |
| `touch` | Enabled touch control below 44 CSS pixels in either direction at widths up to 834. An inline link within a line of text is exempt. |
| `focus` | In keyboard mode, Tab focus produces no visible change (comparing computed appearance and viewport pixels) on the element, its descendants or ancestors, leaves any part of its box outside the viewport, or fails to reach a sequentially focusable element. |
| `shift` | Sum of layout-shift entries without recent input, from initial painting through settling, exceeds 0.05. |
| `motion` | Under reduced motion, a running animation or transition lasts over 100 milliseconds, including iterations. Start events preserve measurements of motion that finishes before settling. |
| `scale` | Visible text's computed font size is outside every declared step or fluid range. |

The command waits for fonts, visits the full document to load lazy content,
returns to the top and observes a further 600 milliseconds before measuring.
State steps and the doubled-text mode discard the layout movement caused by
those intentional changes before that observation interval. Findings concern
the render and observation interval. Visual review of the screenshots covers
qualities that these measurements do not establish.

## Adding a surface

Add one JSON file under `tools/ui-check/surfaces` with its answer files alongside
it. Declare the surface's actual build and start commands, readiness path, views,
states and type scale. Static surfaces declare their served directory instead.
Include every route and meaningful state needed for the surface review. Add
fixed answers for its external requests. Run the command with an external output
directory, address each finding in the surface, and inspect all screenshots.
