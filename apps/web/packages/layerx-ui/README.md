# @layerx/ui

A responsive React component library for **Next.js**, used by the human web application in `apps/web`. One component API renders the right
interaction per platform — bottom sheets on mobile, centered modals on desktop;
grouped money lists on mobile, sortable tables on desktop; a bottom tab bar
on mobile, a sidebar on desktop.

## Install

The [web app](../../package.json) depends on it as `"@layerx/ui": "file:packages/layerx-ui"`. Its `npm run build` runs `npm run build:copy`, `npm run build:ui`, then `next build --webpack`. The [package manifest](package.json) declares peer dependencies of `react` >= 18.2.0, `react-dom` >= 18.2.0 and `tailwindcss` >= 4.0.0.

## Setup (Tailwind v4)

In your app CSS, import the tokens after Tailwind:

```css
@import "tailwindcss";
@import "@layerx/ui/styles.css";
```

That registers every token (`bg-primary`, `text-muted-foreground`, `rounded-sheet`,
`shadow-card`, `animate-sheet-up`, …) as Tailwind utilities, backed by CSS
variables you can override in `:root` to re-theme (e.g. swap `--accent` to a new
brand color).

## The platform contract

Components with a `platform` prop use its explicit value when supplied, otherwise
the nearest `PlatformProvider` value. Both accept `"mobile"`, `"desktop"`, or
`"auto"`; the default is `"auto"`.

`"auto"` follows the viewport query `(max-width: 767px)`, with matching viewports
rendering the mobile variant. Server rendering and the first client render use
the desktop variant until the media query is read. An explicit `platform="auto"`
follows the viewport even inside a provider set to `"mobile"` or `"desktop"`.

| Pattern             | Mobile                                   | Desktop                                                 |
| ------------------- | ---------------------------------------- | ------------------------------------------------------- |
| Navigation          | Bottom tab bar + center action (FAB)     | Left sidebar: approval badge, Explorer, settings footer |
| Primary action      | Full-width button in a sticky footer     | Button with a minimum width in the footer, or a header button |
| Confirmation        | Bottom sheet with consequence copy       | Centered modal, max 440px, Esc + overlay, focus-trapped |
| Detail / education  | Bottom sheet or pushed screen            | Right drawer or inline expanding section                |
| Filters             | Sheet with Clear + Apply                 | Popovers on filter chips; calendar range picker         |
| Money lists         | Stacked rows under labeled groups with subtotals | Sortable table, hover, sticky group rows, CSV export |
| Multi-step journeys | Full-screen wizard, one decision/screen  | Split pane: form + live "what will happen" summary      |
| Search              | Pushed search screen                     | Cmd+K command bar with type-ahead                       |
| Code entry          | Segmented display + on-screen keypad     | One input with a segmented display and full-code paste |
| Notifications       | Pushed screen with recency segments      | Bell popover + full archive page                        |

[`MoneyList`](src/components/money-list.tsx) renders the supplied groups, labels
and subtotals as [`MoneyGroup`](src/lib/types.ts) values. The caller constructs
these groups, including any grouping by month. Desktop sorting orders items within each
group; CSV export uses the supplied group and item order.

Notifications use separate `NotificationsScreen`, `BellPopover` and
`NotificationsArchive` components from [`notifications.tsx`](src/components/notifications.tsx).
The caller chooses which to render; these components have no `platform` prop.

## Quick start

```tsx
"use client";
import { AppShell } from "@layerx/ui";

const nav = [
  { id: "home", label: "Home" },
  { id: "agents", label: "Agents" },
  { id: "activity", label: "Activity", badge: 3 }, // approval badge
  { id: "more", label: "More" },
];

export default function Page() {
  return (
    <AppShell nav={nav} activeNav="home" title="Home">
      {/* your screen */}
    </AppShell>
  );
}
```

## Package layout

- `src/lib` — `cn`, money formatting, platform provider, shared types
- `src/components` — primitives, overlays, patterns (all client components)
- `src/styles/tokens.css` — the design tokens (`@layerx/ui/styles.css`)

## Build

From `apps/web/packages/layerx-ui`:

```bash
npm run build   # tsup → dist/ (ESM + CJS + d.ts)
```

Consumers import the built `dist/` output, so rebuild after changing a component (`npm run build:ui` from `apps/web`).
The [build configuration](tsup.config.ts) adds a `"use client"` banner to the
component and main entry points; the standalone `cn` entry is built separately.
