# SettingsModal

`SettingsModal` is a controlled, accessible drawer. Its props are `isOpen: boolean`
and `onClose: () => void`. Keep the application `next-themes` provider above it.
The supported controls are the four appearance choices and **Reduce dialog motion**.

## Appearance

| Label   | Theme value |
| ------- | ----------- |
| Paxify  | `greenify`  |
| Dark    | `dark`      |
| Hotline | `hotline`   |
| OLED    | `oled`      |

The theme buttons call `next-themes`' `setTheme` and expose selection through
`aria-pressed`, a border, ring and check mark. System themes resolve through
`resolvedTheme`. The drawer does not offer light or system buttons, but still
reflects those values if another application surface selects them.

The shared consumer is the theme provider in `src/pages/_app.tsx`, which applies
classes to the document root. Portfolio, trading and wallet surfaces consume
those classes through the application styles. `next-themes` owns theme storage
under its `theme` key, separately from dialog motion. The drawer does not claim
that the theme provider's best-effort storage operation has succeeded.

## Dialog motion and persistence

**Reduce dialog motion** removes the settings drawer's horizontal/vertical
movement and backdrop transition, and the wallet panel's entrance animation.
It does not change charts, polling, transaction processing or audio.

[`dialogPreferences.ts`](dialogPreferences.ts) supplies a shared external store
and `useDialogPreferences`. `SettingsModal` and `WalletControlPanel` subscribe
to the same value. The versioned browser-storage key is
`hyperpax:dialog-preferences`; its current record is
`{ "version": 1, "reduceMotion": true }`. Missing data uses `false`.
The decoder rejects incompatible versions and non-boolean values.

Consumers wait for restoration before rendering their animated surfaces.
Updates persist the record before publishing the new value to subscribers.
The store survives drawer dismissal, and a fresh runtime reads the stored record
before applying motion behavior. Storage events synchronize other tabs,
including deletion or clearing of the key.

The status text distinguishes loading, an unsaved default, a successful write
or restoration, and a storage error. A failed write preserves the last applied
value and reports that the preference was not saved. A failed read or invalid
record reports an alert in settings; choosing a preference retries persistence.
Persistence is local to this browser and origin, without account synchronization.

## Supported behavior and future consumers

There are no switches for hiding PnL or size/margin, risk warnings, fill audio,
wallet reconnection or skipping confirmations. Those controls previously had
no runtime consumers. The scoped motion setting replaces the broader low-power
promise. Adding any of these features requires a shared contract and tests at
the owning feature before exposing a control. The domain/path wiring requests
are recorded in [`repair-22-33-11.kvx`](repair-22-33-11.kvx).

## Accessible operation and responsive motion

The drawer uses `@radix-ui/react-dialog`: a modal root, portal, overlay, content,
associated title and description, and a close control labelled “Close settings”.
Radix supplies initial focus, focus containment, Tab looping, Escape dismissal,
outside dismissal and scroll management. The custom body-overflow effect is
removed so Radix coordinates scroll locking with other dialogs.

Launchers use controlled props rather than `Dialog.Trigger`.
[`useDialogFocus.ts`](useDialogFocus.ts) captures the active launcher when Radix
opens the dialog and restores focus on dismissal if that element is still
connected. Each opening captures its own launcher. `DepositWithdrawModal` uses
the same lifecycle, labelled title/description and “Close collateral dialog”
control; dismissal does not clear its transaction recovery state.

`useIsMobile` from `hooks/ui/useMobile` selects a bottom drawer at widths up to
767px and a right drawer above that. Existing `framer-motion` spring parameters
are damping 30 and stiffness 300. Reduced motion sets duration to zero and skips
the entrance movement. Animated exits complete before content unmounts.
The fixed header and scrollable body remain within an 85vh mobile drawer or a
420px desktop drawer (maximum 90vw).

Current UI dependencies are `ui/atoms/button`, `ui/atoms/label`,
`ui/atoms/switch`, and `ui/utils/cn`. The switch has an associated label and
an accessible description of the exact affected surfaces.

## Verification

`tests/repair-22-33-11.test.ts` drives the production preference switch, shared
hook and motion consumers, checks actual Radix server-rendered semantics and
controlled dismissal, exercises the focus-restoration controller, and restarts
the component and preference modules while retaining browser storage. It also
covers write/read failures and the wallet signer's identity through real wagmi
and embedded SDK paths with loopback services. The Node-only gate does not run
a browser keyboard walkthrough; focus trapping and Escape are delegated to the
existing Radix primitive without overriding their handlers.
