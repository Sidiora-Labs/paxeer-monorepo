# SettingsModal Component Documentation

## Overview

[`SettingsModal.tsx`](SettingsModal.tsx) exports a controlled settings drawer
with theme previews and seven local toggle states. Theme changes use
`next-themes`. The other toggles currently update only this component's state;
they do not change trading, wallet, privacy, audio or performance behavior
elsewhere in the application.

For network and market-data configuration use the
[deployment guide](../../../deploy/README.md) and
[data API guide](../../../data_api.md). The drawer does not configure endpoints
or chain selection. For live trading integration see the
[component guide](../../components/paxeer/README.md).

## Architecture

`SettingsModal` accepts two required props:

| Prop | Type | Contract |
| --- | --- | --- |
| `isOpen` | `boolean` | Controls whether the backdrop and drawer are shown |
| `onClose` | `() => void` | Parent callback for the close button and backdrop click |

```tsx
import { useState } from "react";
import { SettingsModal } from "@/app/modals/SettingsModal";

export function SettingsLauncher() {
  const [isOpen, setIsOpen] = useState(false);
  return (
    <>
      <button onClick={() => setIsOpen(true)}>Settings</button>
      <SettingsModal isOpen={isOpen} onClose={() => setIsOpen(false)} />
    </>
  );
}
```

Keep the application theme provider above this component. The Pages Router
entry [`_app.tsx`](../../pages/_app.tsx) provides it with `attribute="class"`,
`defaultTheme="dark"`, system resolution enabled, and the themes `light`,
`dark`, `greenify`, `hotline`, `oled`. Existing callers include
[`Header`](../shell/Header.tsx),
[`MobileHeader`](../../widgets/mobile/MobileHeader.tsx), and the
[portfolio page module](../../page-modules/portfolio/index.tsx).

After the initial client mount, the component portals an `AnimatePresence`
container into `document.body`. Its children are the animated backdrop and a
flex-column drawer containing a fixed header, scrollable sections and footer.
It renders nothing before that first mount to avoid a hydration mismatch.

## Theme System

The drawer's actual choices are:

| Label | `next-themes` value | Preview |
| --- | --- | --- |
| Paxify | `greenify` | Blue gradient |
| Dark | `dark` | `/Dark.png` and gradient |
| Hotline | `hotline` | `/Hotline.png` and gradient |
| OLED | `oled` | `/OLED.png` and gradient |

Each option defines `label`, `icon`, `value`, `gradient`, `badgeColor`,
`badgeBg`, `border`, and optionally `bgImage`. Selection calls
`setTheme(option.value)`; the selected button has `aria-pressed`, an accent
ring and check mark. When the provider theme is `system`, the drawer compares
against `resolvedTheme`. Although the provider supports `light` and system
selection, this drawer offers neither as a button.

Theme persistence belongs to `next-themes` (its browser storage), independently
of the local switches below.

## Settings Architecture

### State Management

Each switch uses a component-local `useState` value:

| Section | Label | State | Initial value |
| --- | --- | --- | --- |
| Display & Privacy | Hide PnL | `hidePnL` | `false` |
| Display & Privacy | Hide Size/Margin | `hideSize` | `false` |
| Display & Privacy | Show All Warnings | `showAllWarnings` | `true` |
| Audio & Notifications | Sound for Fills | `soundForFills` | `true` |
| Performance | Low Power Mode | `lowPowerMode` | `false` |
| Trading | Persist Trading Connection | `persistConnection` | `true` |
| Trading | Skip Order Confirmations | `skipConfirmations` | `true` |

`checked` reads the local value and `onCheckedChange` calls its setter. There
is no shared settings store, storage write, API request or downstream consumer
for these switches. Closing and reopening preserves values while the same
`SettingsModal` instance remains mounted; unmounting or reloading resets them.
The footer text “Settings are automatically saved” overstates this behavior
for switches. Do not treat Hide PnL as privacy protection, Persist Trading
Connection as a wallet setting, or Skip Order Confirmations as execution policy.

The drawer has no push-notification permission control, auto-refresh switch,
analytics consent, advanced mode, slippage preset or chart-default editor.

### Adding New Settings

A functioning setting needs an explicit owner, persistence contract and
consumer in the affected feature. Adding a local switch alone only changes its
appearance. Keep labels tied to their switch IDs, define reload/unmount
behavior, and verify the intended consumer before documenting app-wide effects.

## Animation System

`useIsMobile` from [`hooks/ui/useMobile`](../../hooks/ui/useMobile.tsx) tracks
`(max-width: 767px)`. On mobile the drawer moves vertically from `y: "100%"` to
`y: 0`; on desktop it moves horizontally from `x: "100%"` to `x: 0`. Exit uses
the corresponding off-screen position. The spring uses damping 30 and stiffness
300; the backdrop fades and applies background blur.

## Scroll Management

Opening sets `document.body.style.overflow` to `hidden`. Effect cleanup resets
it to the empty string; it does not restore an arbitrary previous inline value.

The drawer is `h-[85vh]` on mobile and full-screen height with width `420px`
(maximum `90vw`) on desktop. The content uses `flex-1 overflow-y-scroll` and
`WebkitOverflowScrolling: "touch"`. Preserve that height/flex relationship when
changing content so the header stays visible and the content can scroll.

## Styling System

The drawer uses application utility classes and CSS variables including
`--main-accent`. Theme previews use the per-option gradient, image and border
values. The backdrop has z-index 9998 and the drawer 9999; both are portaled
outside the caller's stacking hierarchy. Update the theme provider and actual
styles as well as the option list when introducing a theme.

## Dependencies

Current module paths are:

```tsx
import { useTheme } from "next-themes";
import { motion, AnimatePresence } from "framer-motion";
import { createPortal } from "react-dom";
import { Switch } from "@/ui/atoms/switch";
import { Label } from "@/ui/atoms/label";
import { Button } from "@/ui/atoms/button";
import { cn } from "@/ui/utils/cn";
import { useIsMobile } from "@/hooks/ui/useMobile";
```

React supplies state/effects, and `lucide-react` supplies the icons. Icon
classes such as `w-4 h-4` are Tailwind spacing units, not four-pixel dimensions.

## Accessibility

Switch labels use `htmlFor` and matching IDs (`hide-pnl`, `hide-size`,
`show-warnings`, `sound-fills`, `low-power`, `persist-connection`,
`skip-confirmations`). Theme choices are buttons with `aria-pressed`.

The implementation does not provide a dialog role, focus trap, Escape-key
handler or explicit focus restoration. The icon-only close button has no
explicit accessible name. Do not assume that portaling or animation supplies
these behaviors; an accessible dialog integration requires them to be addressed
in the component.

## Performance Considerations

The drawer content is conditional on `isOpen`, but local state belongs to the
mounted component. `themeOptions` is recreated on render. The Low Power Mode
switch does not currently change animation behavior.

## Troubleshooting

If a toggle changes visually but nothing else changes, that matches its local
state contract. For a theme that does not apply, check the provider and theme
styles; a selected preview alone is not proof that every surface supports it.
For mobile scrolling, inspect the fixed drawer height, flex child and overflow
classes. For overlap issues, inspect portal stacking and competing overlays;
for keyboard issues, account for the dialog behaviors listed above.

## Customization Guide

Adjust drawer dimensions in the responsive class branch and animation motion
in the matching mobile/desktop branch. Keep preview values synchronized with
real themes. Avoid adding documentation promises for persistence or runtime
consumers until those paths exist.
